package store

import (
	"context"
	"errors"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 2.4: the ownership reads, the five command executions and the
// reply drain transaction
// ---------------------------------------------------------------------------

// readOwnershipMaterialTx reads the terms, the requests and the
// resolutions of one case (the derivation input, ordered by the epoch and
// the creation).
func readOwnershipMaterialTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.OwnershipTerm, []casesDomain.TransferRequest, []casesDomain.TransferResolution, error) {
	rows, err := tx.Query(ctx, `
                select term_id, case_id, epoch, owner_kind, owner_ref, writable_surface, origin,
                       previous_owner_kind, previous_owner_ref, actor_id, reason, transfer_request_id,
                       requested_at, acknowledged_at, created_at
                from cases.ownership_terms
                where tenant_id=$1 and case_id=$2 order by epoch`, tenantID, caseID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	terms := []casesDomain.OwnershipTerm{}
	for rows.Next() {
		var term casesDomain.OwnershipTerm
		var previousKind, reason *string
		var previousRef, transferRequestID *id.UUID
		var requestedAt, acknowledgedAt *time.Time
		if err = rows.Scan(&term.TermID, &term.CaseID, &term.Epoch, &term.OwnerKind, &term.OwnerRef,
			&term.WritableSurface, &term.Origin, &previousKind, &previousRef, &term.ActorID, &reason,
			&transferRequestID, &requestedAt, &acknowledgedAt, &term.CreatedAt); err != nil {
			return nil, nil, nil, err
		}
		term.TenantID = tenantID
		if previousKind != nil {
			term.PreviousOwnerKind = *previousKind
		}
		if previousRef != nil {
			term.PreviousOwnerRef = *previousRef
		}
		if reason != nil {
			term.Reason = *reason
		}
		if transferRequestID != nil {
			term.TransferRequestID = *transferRequestID
		}
		if requestedAt != nil {
			term.RequestedAt = (*requestedAt).UTC()
		}
		if acknowledgedAt != nil {
			term.AcknowledgedAt = (*acknowledgedAt).UTC()
		}
		term.CreatedAt = term.CreatedAt.UTC()
		terms = append(terms, term)
	}
	if err = rows.Err(); err != nil {
		return nil, nil, nil, err
	}

	rows, err = tx.Query(ctx, `
                select request_id, case_id, request_no, from_owner_kind, from_owner_ref, to_owner_kind, to_owner_ref,
                       writable_surface, reason, requested_by, deadline_at, created_at
                from cases.transfer_requests
                where tenant_id=$1 and case_id=$2 order by created_at, request_id`, tenantID, caseID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	requests := []casesDomain.TransferRequest{}
	for rows.Next() {
		var request casesDomain.TransferRequest
		if err = rows.Scan(&request.RequestID, &request.CaseID, &request.RequestNo,
			&request.FromOwnerKind, &request.FromOwnerRef, &request.ToOwnerKind, &request.ToOwnerRef,
			&request.WritableSurface, &request.Reason, &request.RequestedBy, &request.DeadlineAt, &request.CreatedAt); err != nil {
			return nil, nil, nil, err
		}
		request.TenantID = tenantID
		request.DeadlineAt = request.DeadlineAt.UTC()
		request.CreatedAt = request.CreatedAt.UTC()
		requests = append(requests, request)
	}
	if err = rows.Err(); err != nil {
		return nil, nil, nil, err
	}

	resolutions := []casesDomain.TransferResolution{}
	if len(requests) > 0 {
		rows, err = tx.Query(ctx, `
                        select resolution_id, request_id, kind, actor_id, resulting_epoch, reason, created_at
                        from cases.transfer_resolutions
                        where tenant_id=$1 order by created_at, resolution_id`, tenantID)
		if err != nil {
			return nil, nil, nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var resolution casesDomain.TransferResolution
			var reason *string
			var resultingEpoch *int64
			if err = rows.Scan(&resolution.ResolutionID, &resolution.RequestID, &resolution.Kind,
				&resolution.ActorID, &resultingEpoch, &reason, &resolution.CreatedAt); err != nil {
				return nil, nil, nil, err
			}
			resolution.TenantID = tenantID
			if resultingEpoch != nil {
				resolution.ResultingEpoch = *resultingEpoch
			}
			if reason != nil {
				resolution.Reason = *reason
			}
			resolution.CreatedAt = resolution.CreatedAt.UTC()
			resolutions = append(resolutions, resolution)
		}
		if err = rows.Err(); err != nil {
			return nil, nil, nil, err
		}
	}
	return terms, requests, resolutions, nil
}

// ReadOwnershipMaterial reads the ownership material of one case.
func (s *Postgres) ReadOwnershipMaterial(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.OwnershipTerm, []casesDomain.TransferRequest, []casesDomain.TransferResolution, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	terms, requests, resolutions, err := readOwnershipMaterialTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, nil, nil, err
	}
	return terms, requests, resolutions, tx.Commit(ctx)
}

// ReadConversationByID reads one conversation by its identifier (the
// governing-case resolution input of the reply gate).
func (s *Postgres) ReadConversationByID(ctx context.Context, scope tenancy.Context, conversationID id.UUID) (casesDomain.Conversation, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesDomain.Conversation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	conversation, err := scanConversation(tx.QueryRow(ctx, `
                select tenant_id, case_id, conversation_id, conversation_no, employee_id, connection_id,
                       provider, provider_session_id, response_owner_id, response_owner_epoch, writable_surface, state, created_at
                from cases.conversations where tenant_id=$1 and conversation_id=$2`,
		scope.TenantID(), conversationID))
	if err != nil {
		return casesDomain.Conversation{}, err
	}
	return conversation, tx.Commit(ctx)
}

// scanConversation reads one conversation row (the shared reader of the
// ownership surface).
func scanConversation(r row) (casesDomain.Conversation, error) {
	var c casesDomain.Conversation
	err := r.Scan(&c.TenantID, &c.CaseID, &c.ConversationID, &c.ConversationNo, &c.EmployeeID,
		&c.ConnectionID, &c.Provider, &c.ProviderSessionID, &c.ResponseOwnerID, &c.ResponseOwnerEpoch,
		&c.WritableSurface, &c.State, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.Conversation{}, casesDomain.ErrNotFound
	}
	if err != nil {
		return casesDomain.Conversation{}, err
	}
	c.CreatedAt = c.CreatedAt.UTC()
	return c, nil
}

// ReadClientReplies reads the outgoing replies of the case with their
// derived outcome states.
func (s *Postgres) ReadClientReplies(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.ReplyCard, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select r.reply_id, r.case_id, r.conversation_id, r.owner_kind, r.owner_ref, r.epoch, r.writable_surface,
                       r.connection_id, r.provider, r.channel, r.direction, r.employee_id, r.decision_no, r.profile_version,
                       r.content, r.accuracy_qualifier, r.update_promise, r.actor_id, r.created_at, o.kind, o.created_at
                from cases.client_replies r
                left join cases.client_reply_outcomes o on o.tenant_id = r.tenant_id and o.reply_id = r.reply_id
                where r.tenant_id=$1 and r.case_id=$2 order by r.created_at, r.reply_id`,
		scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cards := []casesPort.ReplyCard{}
	for rows.Next() {
		var reply casesDomain.ClientReply
		var qualifier *string
		var promise *string
		var outcomeKind *string
		var outcomeAt *time.Time
		if err = rows.Scan(&reply.ReplyID, &reply.CaseID, &reply.ConversationID, &reply.OwnerKind, &reply.OwnerRef,
			&reply.Epoch, &reply.WritableSurface, &reply.ConnectionID, &reply.Provider, &reply.Channel,
			&reply.Direction, &reply.EmployeeID, &reply.DecisionNo, &reply.ProfileVersion, &reply.Content,
			&qualifier, &promise, &reply.ActorID, &reply.CreatedAt, &outcomeKind, &outcomeAt); err != nil {
			return nil, err
		}
		if qualifier != nil {
			reply.AccuracyQualifier = *qualifier
		}
		if promise != nil {
			reply.UpdatePromise = *promise
		}
		reply.TenantID = scope.TenantID()
		reply.CreatedAt = reply.CreatedAt.UTC()
		card := casesPort.ReplyCard{Reply: reply}
		if outcomeKind != nil {
			card.OutcomeKind = *outcomeKind
			card.OutcomeAt = (*outcomeAt).UTC()
		}
		cards = append(cards, card)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return cards, tx.Commit(ctx)
}

// PendingClientReplies returns the prepared replies (no outcome row) of
// the tenant bounded by the limit — the drain input.
func (s *Postgres) PendingClientReplies(ctx context.Context, scope tenancy.Context, limit int) ([]casesDomain.ClientReply, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select reply_id, case_id, conversation_id, owner_kind, owner_ref, epoch, writable_surface,
                       connection_id, provider, channel, direction, employee_id, decision_no, profile_version,
                       actor_id, created_at
                from cases.client_replies r
                where r.tenant_id=$1 and not exists (
                        select 1 from cases.client_reply_outcomes o
                        where o.tenant_id = r.tenant_id and o.reply_id = r.reply_id)
                order by r.created_at, r.reply_id limit $2`, scope.TenantID(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	replies := []casesDomain.ClientReply{}
	for rows.Next() {
		var reply casesDomain.ClientReply
		if err = rows.Scan(&reply.ReplyID, &reply.CaseID, &reply.ConversationID, &reply.OwnerKind, &reply.OwnerRef,
			&reply.Epoch, &reply.WritableSurface, &reply.ConnectionID, &reply.Provider, &reply.Channel,
			&reply.Direction, &reply.EmployeeID, &reply.DecisionNo, &reply.ProfileVersion, &reply.ActorID,
			&reply.CreatedAt); err != nil {
			return nil, err
		}
		reply.TenantID = scope.TenantID()
		reply.CreatedAt = reply.CreatedAt.UTC()
		replies = append(replies, reply)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return replies, tx.Commit(ctx)
}

// ResolveReplyDrain re-validates one prepared reply under the case
// advisory lock and writes the append-only outcome: fresh (the current
// term still matches the reply snapshot, the case is open, the app-side
// admission words hold) — dispatched with the outbox record; stale —
// stale_needs_review (the payload is never relayed). The observed
// attention subjects (the expired pending request, the unavailable
// owner) raise the ownership_attention exception ONCE (the partial
// UNIQUE voids the repeats). The material UNIQUE (tenant, reply) is the
// drain idempotency: a decided reply writes nothing.
func (s *Postgres) ResolveReplyDrain(ctx context.Context, p casesPort.ReplyDrainParams) (string, bool, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	tenantID := p.Scope.TenantID()
	if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", caseLockKey(tenantID, p.Reply.CaseID)); err != nil {
		return "", false, err
	}
	// The in-transaction re-validation: the ownership state of the
	// governing case under the lock.
	terms, requests, resolutions, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.Reply.CaseID)
	if err != nil {
		return "", false, err
	}
	current := casesDomain.CurrentOwnership(terms)
	var caseState string
	if err = tx.QueryRow(ctx, "select state from cases.cases where tenant_id=$1 and case_id=$2",
		tenantID, p.Reply.CaseID).Scan(&caseState); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, casesDomain.ErrNotFound
		}
		return "", false, err
	}
	state := casesDomain.DeriveOwnershipState(terms, requests, resolutions, p.OwnerActive, now)
	kind := casesDomain.ReplyOutcomeStaleNeedsReview
	fresh := current != nil && caseState == casesDomain.CaseStateOpen && p.RouteAdmitted && p.OwnerActive &&
		current.OwnerKind == p.Reply.OwnerKind && current.OwnerRef == p.Reply.OwnerRef &&
		current.Epoch == p.Reply.Epoch
	if fresh {
		kind = casesDomain.ReplyOutcomeDispatched
	}
	// The append-only outcome: the UNIQUE (tenant, reply) is the material
	// idempotency — a raced or repeated drain writes nothing new.
	outcome, err := tx.Exec(ctx, `
                insert into cases.client_reply_outcomes (tenant_id, outcome_id, reply_id, kind, created_at)
                values ($1,$2,$3,$4,$5) on conflict (tenant_id, reply_id) do nothing`,
		tenantID, p.OutcomeID, p.Reply.ReplyID, kind, now)
	if err != nil {
		return "", false, mapOwnershipInsertError(err)
	}
	if outcome.RowsAffected() == 0 {
		return "", false, tx.Commit(ctx)
	}
	// The dispatched outbox record: the safe snapshot only (the content
	// never enters the envelope).
	if kind == casesDomain.ReplyOutcomeDispatched {
		payload, err := casesDomain.ClientReplyDispatchedPayloadJSON(casesDomain.ClientReplyDispatchedPayload{
			TenantID: tenantID, CaseID: p.Reply.CaseID, ReplyID: p.Reply.ReplyID, Kind: kind, OccurredAt: now,
		})
		if err != nil {
			return "", false, err
		}
		if _, err = tx.Exec(ctx, `
                        insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                        values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			tenantID, p.OutboxEventID, casesDomain.OutboxEventClientReplyDispatched, casesDomain.OutboxEventClientReplyDispatchedV1,
			p.Reply.CaseID, p.Reply.ConnectionID, p.ExceptionID, now, payload,
			casesDomain.DedupKeyClientReplyDispatched(p.Reply.ReplyID), now); err != nil {
			return "", false, mapOwnershipInsertError(err)
		}
	}
	// The attention subjects: the expired pending request (the request id
	// is the correlation subject) and the unavailable owner (the current
	// term id). The partial UNIQUE voids the repeats — one exception per
	// subject, forever.
	if state.Pending != nil && state.PendingTimedOut {
		if _, err = tx.Exec(ctx, `
                        insert into cases.operational_exceptions (tenant_id, exception_id, kind, summary, correlation_id, event_id, original_event_id, case_id, conversation_id, created_at)
                        values ($1,$2,$3,$4,$5,NULL,NULL,$6,$7,$8) on conflict (tenant_id, correlation_id) where kind = 'ownership_attention' do nothing`,
			tenantID, p.ExceptionID, casesDomain.ExceptionOwnershipAttention, casesDomain.OwnershipAttentionSummary,
			state.Pending.RequestID, p.Reply.CaseID, p.Reply.ConversationID, now); err != nil {
			return "", false, mapOwnershipInsertError(err)
		}
	}
	if state.Health == casesDomain.OwnershipHealthReassignment && current != nil {
		if _, err = tx.Exec(ctx, `
                        insert into cases.operational_exceptions (tenant_id, exception_id, kind, summary, correlation_id, event_id, original_event_id, case_id, conversation_id, created_at)
                        values ($1,$2,$3,$4,$5,NULL,NULL,$6,$7,$8) on conflict (tenant_id, correlation_id) where kind = 'ownership_attention' do nothing`,
			tenantID, p.ExceptionID, casesDomain.ExceptionOwnershipAttention, casesDomain.OwnershipAttentionSummary,
			current.TermID, p.Reply.CaseID, p.Reply.ConversationID, now); err != nil {
			return "", false, mapOwnershipInsertError(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return "", false, err
	}
	return kind, true, nil
}

// mapOwnershipInsertError maps the ownership surface insert conflicts onto
// the honest error classes: the term epoch UNIQUE is the CAS race backstop
// (the stale version), the request resolution UNIQUE is the raced decision,
// the command record pkey is the idempotency conflict.
func mapOwnershipInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "ownership_terms_tenant_id_case_id_epoch_key":
			return casesDomain.ErrStaleVersion
		case "transfer_resolutions_tenant_id_request_id_key":
			return casesDomain.ErrCommandKeyConflict
		case "client_reply_outcomes_tenant_id_reply_id_key":
			return casesDomain.ErrCommandKeyConflict
		}
	}
	return err
}

// ---------------------------------------------------------------------------
// The five ownership command executions (the ExecuteWorkCommand material)
// ---------------------------------------------------------------------------

// execTakeOwnership writes the take term under the case lock: the current
// epoch must match the expectation (the CAS discipline), the state must be
// takeable (the expired request or the unavailable owner — re-derived
// under the lock), and the term of the new epoch inserts with the previous
// owner snapshot; the UNIQUE backstop maps the raced twin onto the stale
// version conflict.
func (s *Postgres) execTakeOwnership(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	// The owner activity word: the app-side member snapshot (the honest
	// word of the member facts read before the transaction).
	terms, requests, resolutions, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(terms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	state := casesDomain.DeriveOwnershipState(terms, requests, resolutions, p.OwnerActiveWord, now)
	if current.Epoch != p.ExpectedVersion {
		return casesDomain.ErrStaleVersion
	}
	if !state.Takeable() {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.HealthyTakeText}
	}
	// The take voids the unresolved ask with the cancel resolution of the
	// SAME transaction (the reassign precedent): after the transition the
	// ownership is healthy — the zombie expired request never keeps the
	// case takeable and never misrepresents the health word.
	if pending := casesDomain.PendingTransfer(terms, requests, resolutions, now); pending != nil {
		if _, err := tx.Exec(ctx, `
                        insert into cases.transfer_resolutions (tenant_id, resolution_id, request_id, kind, actor_id, resulting_epoch, reason, created_at)
                        values ($1,$2,$3,$4,$5,NULL,$6,$7)`,
			tenantID, p.TransferResolutionID, pending.RequestID, casesDomain.TransferResolutionCancelled, p.Actor,
			"погашено взятием владения (дедлайн истёк или владелец недоступен)", now); err != nil {
			return mapOwnershipInsertError(err)
		}
	}
	return insertOwnershipTerm(ctx, tx, tenantID, p, casesDomain.OwnershipTerm{
		TenantID: tenantID, CaseID: p.CaseID, TermID: p.VerificationID,
		Epoch: current.Epoch + 1, OwnerKind: p.OwnerKind, OwnerRef: p.OwnerRef,
		WritableSurface: p.WritableSurface, Origin: casesDomain.OwnershipOriginTake,
		PreviousOwnerKind: current.OwnerKind, PreviousOwnerRef: current.OwnerRef,
		ActorID: p.Actor, Reason: p.Reason, CreatedAt: now,
	}, now)
}

// execRequestTransfer writes the assigned handoff ask under the case lock:
// the current epoch must match, no pending request may exist (the honest
// 409 of the second simultaneous ask), and the request number is the
// per-case monotonic sequence under the lock.
func (s *Postgres) execRequestTransfer(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	terms, requests, resolutions, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(terms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	if current.Epoch != p.ExpectedVersion {
		return casesDomain.ErrStaleVersion
	}
	if casesDomain.ActiveTransferRequest(terms, requests, resolutions, now) != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.PendingTransferText}
	}
	var requestNo int64
	if err = tx.QueryRow(ctx, `
                select coalesce(max(request_no),0)+1 from cases.transfer_requests where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&requestNo); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.transfer_requests (tenant_id, case_id, request_id, request_no, from_owner_kind, from_owner_ref, to_owner_kind, to_owner_ref, writable_surface, reason, requested_by, deadline_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		tenantID, p.CaseID, p.TransferRequestID, requestNo, current.OwnerKind, current.OwnerRef,
		p.OwnerKind, p.OwnerRef, p.WritableSurface, p.Reason, p.Actor, p.DeadlineAt, now); err != nil {
		return mapOwnershipInsertError(err)
	}
	return nil
}

// execResolveTransfer decides one request under the case lock: the
// acknowledge is the atomic transition (the resolution with the resulting
// epoch, the term of the new epoch — the target owner, the request
// surface, the previous owner snapshot — and the outbox event); the
// decline and the cancel write the resolution only. The late acknowledge
// past the deadline refuses honestly — the derivation voided the ask.
func (s *Postgres) execResolveTransfer(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	terms, requests, resolutions, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	var request *casesDomain.TransferRequest
	for i := range requests {
		if requests[i].RequestID == p.TransferRequestID {
			request = &requests[i]
			break
		}
	}
	if request == nil || request.CaseID != p.CaseID {
		return casesDomain.ErrNotFound
	}
	for i := range resolutions {
		if resolutions[i].RequestID == request.RequestID {
			return casesDomain.ErrCommandKeyConflict
		}
	}
	if p.Decision == casesDomain.TransferResolutionAcknowledged {
		current := casesDomain.CurrentOwnership(terms)
		if current == nil {
			return casesDomain.ErrNotFound
		}
		if !now.Before(request.DeadlineAt) {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.RequestExpiredText}
		}
		if _, err = tx.Exec(ctx, `
                        insert into cases.transfer_resolutions (tenant_id, resolution_id, request_id, kind, actor_id, resulting_epoch, reason, created_at)
                        values ($1,$2,$3,$4,$5,$6,$7,$8)`,
			tenantID, p.TransferResolutionID, request.RequestID, p.Decision, p.Actor, current.Epoch+1, nilText(p.Reason), now); err != nil {
			return mapOwnershipInsertError(err)
		}
		return insertOwnershipTerm(ctx, tx, tenantID, p, casesDomain.OwnershipTerm{
			TenantID: tenantID, CaseID: p.CaseID, TermID: p.VerificationID,
			Epoch: current.Epoch + 1, OwnerKind: request.ToOwnerKind, OwnerRef: request.ToOwnerRef,
			WritableSurface: request.WritableSurface, Origin: casesDomain.OwnershipOriginTransferAcknowledge,
			PreviousOwnerKind: current.OwnerKind, PreviousOwnerRef: current.OwnerRef,
			ActorID: p.Actor, TransferRequestID: request.RequestID,
			RequestedAt: request.CreatedAt, AcknowledgedAt: now, CreatedAt: now,
		}, now)
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.transfer_resolutions (tenant_id, resolution_id, request_id, kind, actor_id, resulting_epoch, reason, created_at)
                values ($1,$2,$3,$4,$5,NULL,$6,$7)`,
		tenantID, p.TransferResolutionID, request.RequestID, p.Decision, p.Actor, nilText(p.Reason), now); err != nil {
		return mapOwnershipInsertError(err)
	}
	return nil
}

// execReassignOwnership writes the supervision term under the case lock:
// the direct transition of the new epoch with the honest reason; the
// pending request (if any) is voided by the cancel resolution of the SAME
// transaction.
func (s *Postgres) execReassignOwnership(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	terms, requests, resolutions, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(terms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	if current.Epoch != p.ExpectedVersion {
		return casesDomain.ErrStaleVersion
	}
	// The pending request void: the cancel resolution of the same
	// transaction (the supervision path decides). Earlier unresolved
	// asks are void by the term watermark of the same transaction.
	if pending := casesDomain.PendingTransfer(terms, requests, resolutions, now); pending != nil {
		if _, err = tx.Exec(ctx, `
                        insert into cases.transfer_resolutions (tenant_id, resolution_id, request_id, kind, actor_id, resulting_epoch, reason, created_at)
                        values ($1,$2,$3,$4,$5,NULL,$6,$7)`,
			tenantID, p.TransferResolutionID, pending.RequestID, casesDomain.TransferResolutionCancelled, p.Actor,
			"отменено переназначением надзора", now); err != nil {
			return mapOwnershipInsertError(err)
		}
	}
	return insertOwnershipTerm(ctx, tx, tenantID, p, casesDomain.OwnershipTerm{
		TenantID: tenantID, CaseID: p.CaseID, TermID: p.VerificationID,
		Epoch: current.Epoch + 1, OwnerKind: p.OwnerKind, OwnerRef: p.OwnerRef,
		WritableSurface: p.WritableSurface, Origin: casesDomain.OwnershipOriginReassign,
		PreviousOwnerKind: current.OwnerKind, PreviousOwnerRef: current.OwnerRef,
		ActorID: p.Actor, Reason: p.Reason, CreatedAt: now,
	}, now)
}

// execSendClientReply writes the reply record under the case +
// conversation locks: the gate re-validates (the current owner word, the
// epoch, the health, the governing case of the target conversation), the
// route snapshot of the conversation re-reads in the transaction, and the
// prepared outbox event lands with the safe payload (the content never
// enters the envelope).
func (s *Postgres) execSendClientReply(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	// The governing case of the target conversation: the case it is
	// actively bound INTO (the 2-3 derivation — the binding target), or
	// its own case when no active binding exists (re-read under the lock).
	conversation, err := scanConversation(tx.QueryRow(ctx, `
                select tenant_id, case_id, conversation_id, conversation_no, employee_id, connection_id,
                       provider, provider_session_id, response_owner_id, response_owner_epoch, writable_surface, state, created_at
                from cases.conversations where tenant_id=$1 and conversation_id=$2`,
		tenantID, p.ConversationID))
	if err != nil {
		return err
	}
	governing := conversation.CaseID
	bindings, reversals, err := readBindingsTouchingCaseTx(ctx, tx, tenantID, conversation.CaseID)
	if err != nil {
		return err
	}
	for _, binding := range casesDomain.ActiveBindings(bindings, reversals) {
		if binding.BoundConversationID == p.ConversationID {
			governing = binding.CaseID
			break
		}
	}
	if governing != p.CaseID {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.GoverningCaseText}
	}
	// The gate re-validation: the current owner word, the epoch, the
	// health derivation under the lock.
	terms, requests, resolutions, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(terms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	state := casesDomain.DeriveOwnershipState(terms, requests, resolutions, p.OwnerActiveWord, now)
	if current.OwnerKind == casesDomain.OwnerKindStaffy {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.StaffyOwnerText}
	}
	if state.ReplyBlocked() {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.ReassignmentText}
	}
	if current.OwnerRef != p.Actor || current.OwnerKind != p.OwnerKind {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.NotOwnerText}
	}
	if current.Epoch != p.ExpectedVersion {
		return casesDomain.ErrStaleVersion
	}
	// The mode gate (Story 2.5, AFTER the ownership gate — the same «gate
	// before CAS» order): the effective mode of the Work Item the reply
	// belongs to decides the sales class; the neutral class passes by the
	// explicit v1 rule.
	if err := replyModeGateOfTx(ctx, tx, tenantID, p.CaseID, p); err != nil {
		return err
	}
	// The disclosure gate (the sales and the protected classes): the
	// acting disclosure of the live channel word of the reply.
	if p.ReplyClass == casesDomain.ReplyClassSales || p.ReplyClass == casesDomain.ReplyClassProtectedRequest {
		if err := replyDisclosureGateOfTx(ctx, tx, tenantID, p.CaseID, p.ConversationID, p); err != nil {
			return err
		}
	}
	// The policy gate (Story 2.6, AFTER the disclosure gate — the frozen
	// order: ownership 2-4 → mode 2-5 → disclosure 2-5 → policy 2-6 → CAS):
	// the sales reply needs the Work Item anchor and the CURRENT (the
	// maximal number) outcome of that item to be commercial_answer; the
	// refusal names the current outcome words and the permitted path.
	// The passing gate resolves the Policy Outcome id the reply record
	// references (the request never carries it — the server-side truth).
	policyOutcomeID, err := replyPolicyGateOfTx(ctx, tx, tenantID, p.CaseID, p)
	if err != nil {
		return err
	}
	// The objection gate (Story 2.8, AFTER the policy gate — the frozen
	// ladder: ownership 2-4 → mode 2-5 → disclosure 2-5 → policy 2-6 →
	// objection 2-8 → write; the SALES class only — the neutral and the
	// protected paths stay byte-identical): the stopped persuasion refuses
	// with the allowed endings; a CURRENT objection of the Work Item makes
	// the playbook step mandatory (the first linked answer — always
	// acknowledge_concern); no objection — the step must be empty (the old
	// path). The passing gate resolves the objection id the reply record
	// references (the request never carries it — the server-side truth).
	objectionID, err := replyObjectionGateOfTx(ctx, tx, tenantID, p.CaseID, p)
	if err != nil {
		return err
	}
	// The accuracy gate re-validation (Story 2.11, AFTER the objection gate
	// — the frozen ladder: ... → objection 2.8 → accuracy 2.11 → write;
	// the SALES class only — the neutral and the protected paths stay
	// byte-identical): the CURRENT outcome of the Work Item decides the
	// honest words — active alternatives REQUIRE the qualifier of the
	// active set; no alternatives demand the empty word (the app validated
	// pre-transaction; the store re-validates the server-side truth under
	// the locks).
	if err := replyAccuracyGateOfTx(ctx, tx, tenantID, p.CaseID, p); err != nil {
		return err
	}
	// The commitment link re-validation (Story 2.9, AFTER the objection
	// gate — the frozen ladder: ... → objection 2-8 → commitment link 2-9 →
	// write): the nullable pair is both or neither; ONLY the sales rows
	// carry the commercial material of the EXACT shown version; the linked
	// version must exist, belong to this Work Item of this case and be
	// offered — the draft never presents (the app validated pre-transaction;
	// the store re-validates the server-side truth under the locks).
	if p.CommitmentID != id.UUID("") || p.VersionNo != 0 {
		if p.ReplyClass != casesDomain.ReplyClassSales {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Связь обязательства несёт только коммерческий ответ (sales): нейтральные и защищённые сообщения не предъявляют материальные условия."}
		}
		version, versionErr := commitmentVersionOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.CommitmentID, p.VersionNo)
		if versionErr != nil {
			return versionErr
		}
		if version.Status != casesDomain.CommitmentStatusOffered {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Черновик не считается предъявляемым обязательством: свяжите offered-версию (draft-not-presentable)."}
		}
	}
	// The continuation link binding re-validation (Story 2.10, AFTER the
	// commitment gate — the frozen ladder: ... → commitment link 2-9 →
	// continuation link 2-10 → write): the nullable LinkID is carried ONLY
	// by the sales rows; the linked row must exist in THIS case (the
	// server-side truth under the locks — the app validated
	// pre-transaction). The state never matters: the binding is the honest
	// delivery record; the panel derives the link state read-time.
	if p.LinkID != id.UUID("") {
		if p.ReplyClass != casesDomain.ReplyClassSales {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Связь ссылки продолжения несёт только коммерческий ответ (sales): нейтральные и защищённые сообщения не вплетают ссылку продолжения."}
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
                select exists(select 1 from cases.continuation_links
                where tenant_id=$1 and case_id=$2 and link_id=$3)`,
			tenantID, p.CaseID, p.LinkID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Ссылка продолжения не найдена: связь возможна только со ссылкой этого обращения."}
		}
	}
	// The route snapshot of the conversation: the channel and the
	// direction of its latest accepted event (the honest live route).
	channel, direction := p.RouteChannel, p.RouteDirection
	if err = tx.QueryRow(ctx, `
                select channel, direction from cases.channel_events
                where tenant_id=$1 and conversation_id=$2 and state='accepted'
                order by received_at desc limit 1`, tenantID, p.ConversationID).Scan(&channel, &direction); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// No accepted event: the route snapshot degrades to the command
		// word (the pre-case session of the conversation carries the
		// modality channel).
		channel, direction = p.RouteChannel, p.RouteDirection
	}
	// The honest timing gate re-validation (Story 2.12, AFTER the
	// commitment link gates and the live route resolution — the frozen
	// ladder: ... → commitment link 2-9 → honest timing 2-12 → CAS;
	// CLASSLESS): the reply route (the conversation connection and the
	// resolved live channel) is degraded → the promise word of the
	// approved dictionary is REQUIRED; healthy → the word must stay
	// empty (the app validated pre-transaction; the store re-validates
	// the server-side truth under the locks).
	if err := replyUpdatePromiseGateOfTx(ctx, tx, tenantID, p, conversation.ConnectionID, channel); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch, writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version, content, actor_id, reply_class, work_item_id, policy_outcome_id, objection_id, playbook_step, commitment_id, commitment_version, link_id, accuracy_qualifier, update_promise, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28)`,
		tenantID, p.CaseID, p.ConversationID, p.ReplyID, current.OwnerKind, current.OwnerRef,
		current.Epoch, current.WritableSurface, conversation.ConnectionID, conversation.Provider,
		channel, "outbound", p.LineageEmployeeID, p.LineageDecisionNo, p.LineageProfileVersion,
		p.ReplyContent, p.Actor, nilText(p.ReplyClass), nilID(p.WorkItemID), nilID(policyOutcomeID), nilID(objectionID), nilText(p.PlaybookStep), nilID(p.CommitmentID), nilInt64(p.VersionNo), nilID(p.LinkID), nilText(p.AccuracyQualifier), nilText(p.UpdatePromise), now); err != nil {
		return mapOwnershipInsertError(err)
	}
	// The prepared outbox event: the safe snapshot only.
	payload, err := casesDomain.ClientReplyPreparedPayloadJSON(casesDomain.ClientReplyPreparedPayload{
		TenantID: tenantID, CaseID: p.CaseID, ConversationID: p.ConversationID, ReplyID: p.ReplyID,
		OwnerKind: current.OwnerKind, OwnerRef: current.OwnerRef, Epoch: current.Epoch,
		WritableSurface: current.WritableSurface, ConnectionID: conversation.ConnectionID,
		Provider: conversation.Provider, Channel: channel, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventClientReplyPrepared, casesDomain.OutboxEventClientReplyPreparedV1,
		p.CaseID, conversation.ConnectionID, p.ReplyID, now, payload,
		casesDomain.DedupKeyClientReplyPrepared(p.ReplyID), now); err != nil {
		return mapOwnershipInsertError(err)
	}
	return nil
}

// insertOwnershipTerm writes one transition term and its ownership_changed
// outbox event — the single atomic write of every ownership transition.
// The source connection of the outbox record is the connection of the
// case's original conversation (the acting decision that admitted the
// case — the honest source word of the ownership events).
func insertOwnershipTerm(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, term casesDomain.OwnershipTerm, now time.Time) error {
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, term.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	var requestedAt, acknowledgedAt *time.Time
	if term.RequestedAt != (time.Time{}) {
		requestedAt = &term.RequestedAt
	}
	if term.AcknowledgedAt != (time.Time{}) {
		acknowledgedAt = &term.AcknowledgedAt
	}
	if _, err := tx.Exec(ctx, `
                insert into cases.ownership_terms (tenant_id, case_id, term_id, epoch, owner_kind, owner_ref, writable_surface, origin, previous_owner_kind, previous_owner_ref, actor_id, reason, transfer_request_id, requested_at, acknowledged_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		tenantID, term.CaseID, term.TermID, term.Epoch, term.OwnerKind, term.OwnerRef,
		term.WritableSurface, term.Origin, nilText(term.PreviousOwnerKind), nilID(term.PreviousOwnerRef),
		term.ActorID, nilText(term.Reason), nilID(term.TransferRequestID), requestedAt, acknowledgedAt, now); err != nil {
		return mapOwnershipInsertError(err)
	}
	payload, err := casesDomain.OwnershipChangedPayloadJSON(casesDomain.OwnershipChangedPayload{
		TenantID: tenantID, CaseID: term.CaseID, TermID: term.TermID, Epoch: term.Epoch,
		OwnerKind: term.OwnerKind, OwnerRef: term.OwnerRef, WritableSurface: term.WritableSurface,
		Origin: term.Origin, ActorID: term.ActorID, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventOwnershipChanged, casesDomain.OutboxEventOwnershipChangedV1,
		term.CaseID, sourceConnection, term.TermID, now, payload,
		casesDomain.DedupKeyOwnershipChanged(term.TermID), now); err != nil {
		return mapOwnershipInsertError(err)
	}
	return nil
}
