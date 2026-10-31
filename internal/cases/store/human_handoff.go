package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 2.13: the human handoff lifecycle writes, the composed ownership
// transitions and the case reads
// ---------------------------------------------------------------------------

// TerminalCaseHandoffText is the honest refusal of every lifecycle write
// on a terminal case: the service is over — the ask, the assignment and
// the return have nothing to record (the v1 backstop word of the
// degradation precedent).
const TerminalCaseHandoffText = "Терминальное обращение не принимает записей запроса человека: обслуживание завершено — жизненный цикл передачи не пишется."

// HandoffAbsentText is the honest refusal of every lifecycle command
// without a request: there is nothing to assign, acknowledge, decline,
// cancel or return — the lifecycle begins with the client's ask. The
// word lives in the domain dictionary (the app pre-transaction guards
// and the store re-validation share the honest sentence).
const HandoffAbsentText = casesDomain.HandoffAbsentText

// HandoffConversationForeignText is the honest refusal of the
// conversation guard: the mode write of the ask anchors to a conversation
// of THIS case (the 2.5 discipline — the ladder of the case's own
// dialog).
const HandoffConversationForeignText = "Команда запроса человека работает на собственном диалоге обращения: выберите диалог этого обращения."

// mapHandoffInsertError maps the raced insert conflicts of the lifecycle
// surfaces onto the honest domain errors: the command record UNIQUE is
// the material idempotency conflict; the request and event number
// UNIQUEs are the raced derivations (the loser of the advisory-lock race
// sees the winner's committed number — the honest stale-retry signal,
// the route_degradations precedent).
func mapHandoffInsertError(err error, command string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "human_handoffs_pkey":
			// The raced request number: the loser re-derives under the locks
			// and retries (the honest stale-retry signal).
			return casesDomain.ErrStaleVersion
		case "human_handoff_events_pkey":
			// The raced event number of the same request (the two-operator
			// acknowledge race backstop — the locks serialize the normal
			// path; the loser re-derives the closed state honestly).
			return casesDomain.ErrStaleVersion
		case "ownership_terms_tenant_id_case_id_epoch_key":
			// The raced epoch of the composed transition (the CAS backstop).
			return casesDomain.ErrStaleVersion
		case "transfer_resolutions_tenant_id_request_id_key":
			return casesDomain.ErrCommandKeyConflict
		}
	}
	return err
}

// scanHandoffRequest reads one append-only request row.
func scanHandoffRequest(scan func(dest ...any) error, tenantID id.UUID) (casesDomain.HandoffRequest, error) {
	var request casesDomain.HandoffRequest
	var roles []byte
	err := scan(&request.CaseID, &request.RequestNo, &request.Reason,
		&request.OwnerKind, &request.OwnerRef, &request.OwnerEpoch,
		&roles, &request.AckDeadline, &request.AckDeadlineAt,
		&request.TurnRef, &request.ActorID, &request.CreatedAt)
	if err != nil {
		return request, err
	}
	request.TenantID = tenantID
	request.AckDeadlineAt = request.AckDeadlineAt.UTC()
	request.CreatedAt = request.CreatedAt.UTC()
	if err := json.Unmarshal(roles, &request.TargetRoles); err != nil {
		return request, err
	}
	return request, nil
}

// scanHandoffEvent reads one append-only lifecycle event row (the
// per-kind material is nullable — the pointer scan of the mode terms
// precedent).
func scanHandoffEvent(scan func(dest ...any) error, tenantID id.UUID) (casesDomain.HandoffEvent, error) {
	var event casesDomain.HandoffEvent
	var assignee, backupOwner, turnRef *id.UUID
	var ackDeadline, result *string
	var deadlineAt *time.Time
	var resultingEpoch *int64
	err := scan(&event.CaseID, &event.RequestNo, &event.EventNo, &event.Kind,
		&assignee, &backupOwner, &ackDeadline, &deadlineAt,
		&result, &turnRef, &resultingEpoch, &event.ActorID, &event.CreatedAt)
	if err != nil {
		return event, err
	}
	event.TenantID = tenantID
	if assignee != nil {
		event.Assignee = *assignee
	}
	if backupOwner != nil {
		event.BackupOwner = *backupOwner
	}
	if ackDeadline != nil {
		event.AckDeadline = *ackDeadline
	}
	if deadlineAt != nil {
		event.DeadlineAt = deadlineAt.UTC()
	}
	if result != nil {
		event.Result = *result
	}
	if turnRef != nil {
		event.TurnRef = *turnRef
	}
	if resultingEpoch != nil {
		event.ResultingEpoch = *resultingEpoch
	}
	event.CreatedAt = event.CreatedAt.UTC()
	return event, nil
}

// readHandoffsTx reads every request row and every lifecycle event row of
// one case ordered by the monotonic numbers (the derivation input of the
// panel, the client status and the freeze gate — the states are NEVER
// stored). The read is the lock-visible truth under the SAME advisory
// locks of the commands and the commitment freeze gate (the
// readDegradationsTx mirror of 2.12 — the freeze gate never reads a raced
// snapshot).
func readHandoffsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.HandoffRequest, []casesDomain.HandoffEvent, error) {
	requestRows, err := tx.Query(ctx, `
                select case_id, request_no, reason, owner_kind, owner_ref, owner_epoch,
                       target_roles, ack_deadline, ack_deadline_at, turn_ref, actor_id, created_at
                from cases.human_handoffs
                where tenant_id=$1 and case_id=$2 order by request_no`, tenantID, caseID)
	if err != nil {
		return nil, nil, err
	}
	defer requestRows.Close()
	requests := []casesDomain.HandoffRequest{}
	for requestRows.Next() {
		request, err := scanHandoffRequest(requestRows.Scan, tenantID)
		if err != nil {
			return nil, nil, err
		}
		requests = append(requests, request)
	}
	if err := requestRows.Err(); err != nil {
		return nil, nil, err
	}
	eventRows, err := tx.Query(ctx, `
                select case_id, request_no, event_no, kind, assignee, backup_owner,
                       ack_deadline, deadline_at, result, turn_ref, resulting_epoch, actor_id, created_at
                from cases.human_handoff_events
                where tenant_id=$1 and case_id=$2 order by request_no, event_no`, tenantID, caseID)
	if err != nil {
		return nil, nil, err
	}
	defer eventRows.Close()
	events := []casesDomain.HandoffEvent{}
	for eventRows.Next() {
		event, err := scanHandoffEvent(eventRows.Scan, tenantID)
		if err != nil {
			return nil, nil, err
		}
		events = append(events, event)
	}
	return requests, events, eventRows.Err()
}

// ReadHandoffMaterial reads the whole append-only lifecycle material of
// one case (the own read transaction of the panel, the transfer view and
// the command pre-checks — the lifecycle states are the read-time domain
// derivation, never a stored status).
func (s *Postgres) ReadHandoffMaterial(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesPort.HandoffMaterial, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesPort.HandoffMaterial{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	requests, events, err := readHandoffsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return casesPort.HandoffMaterial{}, err
	}
	material := casesPort.HandoffMaterial{Requests: requests, Events: events}
	return material, tx.Commit(ctx)
}

// handoffOpenView derives the OPEN lifecycle view of the case: the
// one-competing-task invariant keeps at most one open request (the
// request_human 409), so the open view IS the live ask; the defensive
// scan of every request keeps the answer honest (a closed request never
// hides a newer open one — the last request decides, the earlier rows
// are the readable history).
func handoffOpenView(requests []casesDomain.HandoffRequest, events []casesDomain.HandoffEvent, now time.Time) *casesDomain.HandoffStateView {
	for i := len(requests) - 1; i >= 0; i-- {
		view := casesDomain.DeriveHandoffState(requests[i], events, now)
		if view.Active() {
			return &view
		}
	}
	return nil
}

// handoffLastView derives the view of the LAST request (the maximal
// request number — the append-only history is never rewritten): the
// return answers the lifecycle the last ask closed.
func handoffLastView(requests []casesDomain.HandoffRequest, events []casesDomain.HandoffEvent, now time.Time) *casesDomain.HandoffStateView {
	if len(requests) == 0 {
		return nil
	}
	view := casesDomain.DeriveHandoffState(requests[len(requests)-1], events, now)
	return &view
}

// handoffCaseOpen re-validates the case existence and the open state
// under the locks (the honest 404 of the non-participant path and the
// terminal backstop of every lifecycle write).
func handoffCaseOpen(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, command string, now time.Time) error {
	var caseState string
	err := tx.QueryRow(ctx, `
                select state from cases.cases where tenant_id=$1 and case_id=$2`,
		tenantID, caseID).Scan(&caseState)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if caseState != casesDomain.CaseStateOpen {
		return casesDomain.WorkRuleError{Rule: command, Reason: TerminalCaseHandoffText}
	}
	return nil
}

// handoffTurnRefOfTx re-validates the turn anchor under the locks: an
// accepted event of THIS case (the 2.5 turn signal discipline — the
// anchor of the unconditional ask and of the client's cancellation; the
// modeTurnRefOfTx mirror with the lifecycle rule word).
func handoffTurnRefOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, eventID id.UUID, command string) error {
	var found int
	err := tx.QueryRow(ctx, `
                select 1 from cases.channel_events
                where tenant_id=$1 and event_id=$2 and case_id=$3 and state='accepted'`,
		tenantID, eventID, caseID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.WorkRuleError{Rule: command, Reason: casesDomain.TurnRefNotFoundText}
	}
	return err
}

// handoffConversationOfTx re-validates the conversation anchor of the
// mode write under the locks: the conversation must belong to THIS case
// (the mode ladder of the case's own dialog — the modeConversationOfTx
// mirror with the lifecycle rule word).
func handoffConversationOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, conversationID id.UUID, command string) error {
	var owner id.UUID
	err := tx.QueryRow(ctx, `
                select response_owner_id from cases.conversations
                where tenant_id=$1 and conversation_id=$2 and case_id=$3`,
		tenantID, conversationID, caseID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.WorkRuleError{Rule: command, Reason: HandoffConversationForeignText}
	}
	return err
}

// handoffEventNo derives the NEXT monotonic event number of one request
// under the locks (the request never carries it; the UNIQUE backstop
// catches the two-operator race the locks already serialize).
func handoffEventNo(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, requestNo int64) (int64, error) {
	var eventNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(event_no),0)+1 from cases.human_handoff_events
                where tenant_id=$1 and case_id=$2 and request_no=$3`,
		tenantID, caseID, requestNo).Scan(&eventNo); err != nil {
		return 0, err
	}
	return eventNo, nil
}

// insertHandoffEvent writes one append-only lifecycle event row under
// the locks. The material is per-kind (the table CHECK is the backstop);
// the numbers are the server truth; the owner NEVER changes on this row
// — only the composed transitions move the ownership.
func insertHandoffEvent(ctx context.Context, tx pgx.Tx, tenantID id.UUID, command string, event casesDomain.HandoffEvent) error {
	if _, err := tx.Exec(ctx, `
                insert into cases.human_handoff_events (tenant_id, case_id, request_no, event_no, kind,
                        assignee, backup_owner, ack_deadline, deadline_at, result, turn_ref, resulting_epoch, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		tenantID, event.CaseID, event.RequestNo, event.EventNo, event.Kind,
		nilID(event.Assignee), nilID(event.BackupOwner), nilText(event.AckDeadline),
		nilTime(event.DeadlineAt), nilText(event.Result), nilID(event.TurnRef),
		nilEpoch(event.ResultingEpoch), event.ActorID, event.CreatedAt); err != nil {
		return mapHandoffInsertError(err, command)
	}
	return nil
}

// nilEpoch renders the zero epoch as the SQL NULL (the per-kind material
// of the lifecycle events).
func nilEpoch(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

// insertHandoffOutbox writes one lifecycle outbox event: the envelope
// carries the identifiers and the dictionary words only (the request and
// event numbers, the kind, the actors, the deadline and result words,
// the anchor turns) — the system reason text of the mode and any free
// material NEVER enter (the «кандидат не покидает строку» precedent).
func insertHandoffOutbox(ctx context.Context, tx pgx.Tx, tenantID id.UUID, eventType string, schemaVersion int,
	outboxEventID id.UUID, p casesPort.WorkCommandParams, payload casesDomain.HandoffEventPayload, dedupKey string) error {
	body, err := casesDomain.HandoffEventPayloadJSON(payload)
	if err != nil {
		return err
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, payload.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, outboxEventID, eventType, schemaVersion,
		payload.CaseID, sourceConnection, p.CorrelationID, payload.OccurredAt, body, dedupKey, payload.OccurredAt); err != nil {
		return mapHandoffOutboxInsertError(err)
	}
	return nil
}

// mapHandoffOutboxInsertError maps the raced envelope conflicts onto the
// honest domain errors (the degradation precedent).
func mapHandoffOutboxInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "outbox_events_pkey":
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// handoffOutboxEvent resolves the safe payload of one lifecycle write:
// the identifiers (the tenant, the case, the request and event numbers,
// the actors), the dictionary words (the kind, the reason, the target
// roles, the deadline, the result) and the server truths (the resulting
// epoch, the deadline time, the anchor turns). The request write carries
// the zero event material — the envelope type already says requested.
func handoffOutboxEvent(tenantID id.UUID, request casesDomain.HandoffRequest, event casesDomain.HandoffEvent, now time.Time) casesDomain.HandoffEventPayload {
	deadlineAt := event.DeadlineAt
	if deadlineAt.IsZero() {
		deadlineAt = request.AckDeadlineAt
	}
	return casesDomain.HandoffEventPayload{
		TenantID: tenantID, CaseID: request.CaseID, RequestNo: request.RequestNo, EventNo: event.EventNo,
		Kind: event.Kind, Reason: request.Reason, TargetRoles: request.TargetRoles,
		AckDeadline: request.AckDeadline, DeadlineAt: deadlineAt,
		Assignee: event.Assignee, BackupOwner: event.BackupOwner, Result: event.Result,
		ResultingEpoch: event.ResultingEpoch, TurnRef: event.TurnRef,
		ActorID: event.ActorID, OccurredAt: now,
	}
}

// execRequestHuman writes the unconditional ask under the case +
// conversation locks: the case must exist under the tenant scope and stay
// open, the conversation must be the case's own (the mode write anchor),
// the turn anchor must be an accepted event of the case (the store
// guard), the domain guard re-validates the material under the locks, the
// mode CAS must match (the raced twin gets the honest conflict with the
// current mode words), NO open lifecycle may exist (the
// one-competing-task 409 with the current state words), and the
// conditional mode term of the ratchet raises through the EXISTING 2.5
// machinery (insertModeTerm — the mode_changed envelope with the system
// reason constant) whenever the raise is honest — a stricter ladder
// leaves the mode untouched (the ask is still recorded, AC-1). The
// request number is the NEXT monotonic number of the case derived under
// the locks; the owner snapshot is the server truth of the moment; the
// human_requested envelope carries the identifiers and the dictionary
// words only.
func (s *Postgres) execRequestHuman(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := handoffCaseOpen(ctx, tx, tenantID, p.CaseID, p.Command, now); err != nil {
		return err
	}
	if err := handoffConversationOfTx(ctx, tx, tenantID, p.CaseID, p.ConversationID, p.Command); err != nil {
		return err
	}
	if err := handoffTurnRefOfTx(ctx, tx, tenantID, p.CaseID, p.TurnRef, p.Command); err != nil {
		return err
	}
	if err := casesDomain.GuardRequestHuman(p.TurnRef, p.ExpectedModeVersion, p.HandoffTargetRoles, p.HandoffAckDeadline); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The Story 4.3 reason re-validation under the locks (the honest
	// 400 of the out-of-dictionary word BEFORE SQL; the table CHECK of
	// 00019 is the material backstop).
	if err := casesDomain.GuardHandoffRequestReason(p.HandoffReason); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The mode CAS under the locks: ANY raced mode change refuses with the
	// honest conflict (the loser re-reads the ladder); the conditional
	// write below then decides the ratchet direction.
	terms, err := readModeMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	active := casesDomain.ActiveConversationMode(terms)
	if current := casesDomain.ModeTermMaxVersion(terms); current != p.ExpectedModeVersion {
		return modeConflictOf(active, current)
	}
	// The one-competing-task gate: an open lifecycle refuses the second
	// ask with the honest words of what already lives (AC-4).
	requests, events, err := readHandoffsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	if open := handoffOpenView(requests, events, now); open != nil {
		return casesDomain.HandoffActiveError{
			State:        open.State,
			ClientStatus: casesDomain.DeriveHandoffClientStatus(open.State),
		}
	}
	// The owner snapshot of the moment (the server truth under the locks).
	ownershipTerms, _, _, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(ownershipTerms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	// The conditional ratchet raise through the EXISTING 2.5 machinery: a
	// stricter or equal active mode leaves the ladder untouched (the ask
	// is unconditional — AC-1); the honest raise writes the term with the
	// system reason constant and the turn signal source.
	if term, ok := casesDomain.HandoffModeTerm(active, tenantID, p.CaseID, p.ConversationID, p.TurnRef, p.Actor, now); ok {
		term.TermID = p.ModeTermID
		if err := insertModeTerm(ctx, tx, tenantID, p, *term, now); err != nil {
			return err
		}
	}
	// The request row: the NEXT monotonic number under the locks (the
	// request never carries it; the UNIQUE backstop catches the race the
	// locks already serialize).
	var requestNo int64
	if err = tx.QueryRow(ctx, `
                select coalesce(max(request_no),0)+1 from cases.human_handoffs
                where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&requestNo); err != nil {
		return err
	}
	// The reason word of the request row: the EMPTY command word stays
	// the unconditional client ask of 2.13 (the system truth — the old
	// path byte-identical); the Story 4.3 boundary word of the operator
	// (validated above) is the honest reason of the boundary handoff
	// (the SQL CHECK of 00019 admits the four dictionary words).
	requestReason := p.HandoffReason
	if requestReason == "" {
		requestReason = casesDomain.HandoffReasonClientRequest
	}
	request := casesDomain.HandoffRequest{
		TenantID: tenantID, CaseID: p.CaseID, RequestNo: requestNo,
		Reason:    requestReason,
		OwnerKind: current.OwnerKind, OwnerRef: current.OwnerRef, OwnerEpoch: current.Epoch,
		TargetRoles: p.HandoffTargetRoles, AckDeadline: p.HandoffAckDeadline,
		AckDeadlineAt: now.Add(casesDomain.HandoffAckDeadlineWindow(p.HandoffAckDeadline)),
		TurnRef:       p.TurnRef, ActorID: p.Actor, CreatedAt: now,
	}
	roles, err := json.Marshal(request.TargetRoles)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.human_handoffs (tenant_id, case_id, request_no, reason, owner_kind, owner_ref, owner_epoch,
                        target_roles, ack_deadline, ack_deadline_at, turn_ref, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		tenantID, request.CaseID, request.RequestNo, request.Reason,
		request.OwnerKind, request.OwnerRef, request.OwnerEpoch,
		roles, request.AckDeadline, request.AckDeadlineAt, request.TurnRef, request.ActorID, request.CreatedAt); err != nil {
		return mapHandoffInsertError(err, p.Command)
	}
	// The lifecycle envelope: the identifiers and the dictionary words
	// only — the system reason text of the mode NEVER enters. The request
	// envelope carries the anchor turn and the actor through the event
	// carrier (the zero event number and kind — the envelope type already
	// says requested).
	requestEnvelopeEvent := casesDomain.HandoffEvent{TurnRef: request.TurnRef, ActorID: request.ActorID}
	payload := handoffOutboxEvent(tenantID, request, requestEnvelopeEvent, now)
	return insertHandoffOutbox(ctx, tx, tenantID, casesDomain.OutboxEventHumanRequested, casesDomain.OutboxEventHumanRequestedV1,
		p.HandoffOutboxEventID, p, payload, casesDomain.DedupKeyHandoffEvent(p.CaseID, requestNo, 0))
}

// execAssignHumanHandoff writes one assignment round event under the
// case + conversation locks: the open lifecycle must exist, the domain
// guard re-validates the round material under the locks (the state, the
// assignee, the role word against the request's target roles, the backup
// and the deadline word), the event number is the NEXT monotonic number
// of the request derived under the locks, and the assignment NEVER
// changes the owner (AC-3: назначение ≠ принятие). The re-assignment is
// a NEW event with a NEW deadline — the human re-decision.
func (s *Postgres) execAssignHumanHandoff(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := handoffCaseOpen(ctx, tx, tenantID, p.CaseID, p.Command, now); err != nil {
		return err
	}
	requests, events, err := readHandoffsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	// The target of the assignment: the OPEN lifecycle, or the LAST closed
	// one (the guard then refuses with the honest state word — the raced
	// twin of a closed lifecycle learns what already happened).
	target := handoffOpenView(requests, events, now)
	if target == nil {
		target = handoffLastView(requests, events, now)
	}
	if target == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: HandoffAbsentText}
	}
	if err := casesDomain.GuardAssignHandoff(target.State, p.HandoffAssignee, p.HandoffAssigneeRole,
		target.Request.TargetRoles, p.HandoffBackup, p.HandoffAckDeadline); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	open := target
	eventNo, err := handoffEventNo(ctx, tx, tenantID, p.CaseID, open.Request.RequestNo)
	if err != nil {
		return err
	}
	event := casesDomain.HandoffEvent{
		TenantID: tenantID, CaseID: p.CaseID, RequestNo: open.Request.RequestNo, EventNo: eventNo,
		Kind: casesDomain.HandoffEventAssigned, Assignee: p.HandoffAssignee, BackupOwner: p.HandoffBackup,
		AckDeadline: p.HandoffAckDeadline, DeadlineAt: now.Add(casesDomain.HandoffAckDeadlineWindow(p.HandoffAckDeadline)),
		ActorID: p.Actor, CreatedAt: now,
	}
	if err := insertHandoffEvent(ctx, tx, tenantID, p.Command, event); err != nil {
		return err
	}
	payload := handoffOutboxEvent(tenantID, open.Request, event, now)
	return insertHandoffOutbox(ctx, tx, tenantID, casesDomain.OutboxEventHumanAssigned, casesDomain.OutboxEventHumanAssignedV1,
		p.OutboxEventID, p, payload, casesDomain.DedupKeyHandoffEvent(p.CaseID, open.Request.RequestNo, eventNo))
}

// execHandoffAcknowledge writes the confirmed acceptance of the assigned
// round as ONE transaction with the 2.4 transfer machinery (the CALL of
// the existing mechanics, never a copy): the derived state must be the
// open assigned round, the actor IS the assignee (the server truth — the
// assignment or the notification is never an acknowledgement, AC-3/5),
// the ownership CAS must match, no pending 2.4 transfer may live, and
// the composed write lands atomically — the acknowledged event with the
// resulting epoch, the TransferRequest of the handoff dictionary reason
// constant (from the prior owner to the assignee, the inbox surface),
// the acknowledged Resolution and the new ownership Term (the
// transfer_acknowledge origin, the epoch+1, the previous owner snapshot)
// with its ownership_changed envelope, plus the human_acknowledged
// lifecycle envelope of the identifiers and the dictionary words. The
// raced twin of the two-operator race loses on the locks first (the
// re-derived closed state refuses honestly) and on the UNIQUE + CAS
// backstops second (ErrStaleVersion).
func (s *Postgres) execHandoffAcknowledge(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := handoffCaseOpen(ctx, tx, tenantID, p.CaseID, p.Command, now); err != nil {
		return err
	}
	requests, events, err := readHandoffsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	// The target of the acceptance: the OPEN lifecycle, or the LAST closed
	// one (the guard then refuses with the honest state word — the raced
	// twin of the two-operator race learns the lifecycle is already
	// acknowledged).
	target := handoffOpenView(requests, events, now)
	if target == nil {
		target = handoffLastView(requests, events, now)
	}
	if target == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: HandoffAbsentText}
	}
	open := target
	if err := casesDomain.GuardHandoffAcknowledge(open.State, p.Actor, open.Assignee, p.ExpectedVersion); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	ownershipTerms, transferRequests, resolutions, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(ownershipTerms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	if current.Epoch != p.ExpectedVersion {
		return casesDomain.ErrStaleVersion
	}
	if casesDomain.ActiveTransferRequest(ownershipTerms, transferRequests, resolutions, now) != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.PendingTransferText}
	}
	// The acknowledged lifecycle event: the resulting epoch is the term of
	// this same transaction (the ONE transition that moves the ownership).
	eventNo, err := handoffEventNo(ctx, tx, tenantID, p.CaseID, open.Request.RequestNo)
	if err != nil {
		return err
	}
	acknowledgedAt := now
	event := casesDomain.HandoffEvent{
		TenantID: tenantID, CaseID: p.CaseID, RequestNo: open.Request.RequestNo, EventNo: eventNo,
		Kind: casesDomain.HandoffEventAcknowledged, ResultingEpoch: current.Epoch + 1,
		ActorID: p.Actor, CreatedAt: acknowledgedAt,
	}
	if err := insertHandoffEvent(ctx, tx, tenantID, p.Command, event); err != nil {
		return err
	}
	// The composed 2.4 machinery: the TransferRequest of the honest
	// dictionary reason constant (the operator never types it), the
	// acknowledged Resolution, the ownership Term of the epoch+1.
	requestedAt := open.Request.CreatedAt
	if open.LastEvent != nil {
		requestedAt = open.LastEvent.CreatedAt
	}
	if err := insertHandoffTransfer(ctx, tx, tenantID, p, casesDomain.TransferRequest{
		TenantID: tenantID, CaseID: p.CaseID, RequestID: p.TransferRequestID,
		FromOwnerKind: current.OwnerKind, FromOwnerRef: current.OwnerRef,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: open.Assignee,
		WritableSurface: casesDomain.WritableSurfaceInboxV2,
		Reason:          casesDomain.HandoffReasonText(casesDomain.HandoffReasonClientRequest),
		RequestedBy:     p.Actor, DeadlineAt: now.Add(casesDomain.HandoffAckDeadlineWindow(open.Request.AckDeadline)),
		CreatedAt: acknowledgedAt,
	}); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.transfer_resolutions (tenant_id, resolution_id, request_id, kind, actor_id, resulting_epoch, reason, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8)`,
		tenantID, p.TransferResolutionID, p.TransferRequestID, casesDomain.TransferResolutionAcknowledged,
		p.Actor, current.Epoch+1, nilText(p.Reason), acknowledgedAt); err != nil {
		return mapHandoffInsertError(err, p.Command)
	}
	if err := insertOwnershipTerm(ctx, tx, tenantID, p, casesDomain.OwnershipTerm{
		TenantID: tenantID, CaseID: p.CaseID, TermID: p.VerificationID,
		Epoch: current.Epoch + 1, OwnerKind: casesDomain.OwnerKindHuman, OwnerRef: open.Assignee,
		WritableSurface: casesDomain.WritableSurfaceInboxV2, Origin: casesDomain.OwnershipOriginTransferAcknowledge,
		PreviousOwnerKind: current.OwnerKind, PreviousOwnerRef: current.OwnerRef,
		ActorID: p.Actor, TransferRequestID: p.TransferRequestID,
		RequestedAt: requestedAt, AcknowledgedAt: acknowledgedAt, CreatedAt: acknowledgedAt,
	}, acknowledgedAt); err != nil {
		return err
	}
	payload := handoffOutboxEvent(tenantID, open.Request, event, acknowledgedAt)
	return insertHandoffOutbox(ctx, tx, tenantID, casesDomain.OutboxEventHumanAcknowledged, casesDomain.OutboxEventHumanAcknowledgedV1,
		p.HandoffOutboxEventID, p, payload, casesDomain.DedupKeyHandoffEvent(p.CaseID, open.Request.RequestNo, eventNo))
}

// execHandoffDecline writes the honest refusal of the assigned round
// under the case + conversation locks: the derived state must be the
// open assigned round and the actor IS the assignee; the decline NEVER
// changes the owner (AC-6: the prior safe owner stays) and the request
// stays open for the human re-assignment.
func (s *Postgres) execHandoffDecline(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := handoffCaseOpen(ctx, tx, tenantID, p.CaseID, p.Command, now); err != nil {
		return err
	}
	requests, events, err := readHandoffsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	// The target of the refusal: the OPEN lifecycle, or the LAST closed
	// one (the guard then refuses with the honest state word).
	target := handoffOpenView(requests, events, now)
	if target == nil {
		target = handoffLastView(requests, events, now)
	}
	if target == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: HandoffAbsentText}
	}
	open := target
	if err := casesDomain.GuardHandoffDecline(open.State, p.Actor, open.Assignee); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	eventNo, err := handoffEventNo(ctx, tx, tenantID, p.CaseID, open.Request.RequestNo)
	if err != nil {
		return err
	}
	event := casesDomain.HandoffEvent{
		TenantID: tenantID, CaseID: p.CaseID, RequestNo: open.Request.RequestNo, EventNo: eventNo,
		Kind: casesDomain.HandoffEventDeclined, ActorID: p.Actor, CreatedAt: now,
	}
	if err := insertHandoffEvent(ctx, tx, tenantID, p.Command, event); err != nil {
		return err
	}
	payload := handoffOutboxEvent(tenantID, open.Request, event, now)
	return insertHandoffOutbox(ctx, tx, tenantID, casesDomain.OutboxEventHumanDeclined, casesDomain.OutboxEventHumanDeclinedV1,
		p.OutboxEventID, p, payload, casesDomain.DedupKeyHandoffEvent(p.CaseID, open.Request.RequestNo, eventNo))
}

// execCancelHumanRequest writes the client's cancellation under the case
// + conversation locks: the lifecycle must be open (the closed
// lifecycles refuse — the acknowledged case returns through
// return_to_staffy), the cancelling turn anchor must be an accepted event
// of the case, and the cancellation NEVER changes the owner and NEVER
// deletes the history (AC-10); the mode stays (the 2.5 ratchet — the
// exit is the authoritative exit_mode).
func (s *Postgres) execCancelHumanRequest(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := handoffCaseOpen(ctx, tx, tenantID, p.CaseID, p.Command, now); err != nil {
		return err
	}
	requests, events, err := readHandoffsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	// The target of the cancellation: the OPEN lifecycle, or the LAST closed
	// one (the guard then refuses with the honest state word — the
	// acknowledged case returns through return_to_staffy).
	target := handoffOpenView(requests, events, now)
	if target == nil {
		target = handoffLastView(requests, events, now)
	}
	if target == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: HandoffAbsentText}
	}
	open := target
	if err := casesDomain.GuardCancelHumanRequest(open.State, p.TurnRef); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	if err := handoffTurnRefOfTx(ctx, tx, tenantID, p.CaseID, p.TurnRef, p.Command); err != nil {
		return err
	}
	eventNo, err := handoffEventNo(ctx, tx, tenantID, p.CaseID, open.Request.RequestNo)
	if err != nil {
		return err
	}
	event := casesDomain.HandoffEvent{
		TenantID: tenantID, CaseID: p.CaseID, RequestNo: open.Request.RequestNo, EventNo: eventNo,
		Kind: casesDomain.HandoffEventCancelled, TurnRef: p.TurnRef, ActorID: p.Actor, CreatedAt: now,
	}
	if err := insertHandoffEvent(ctx, tx, tenantID, p.Command, event); err != nil {
		return err
	}
	payload := handoffOutboxEvent(tenantID, open.Request, event, now)
	return insertHandoffOutbox(ctx, tx, tenantID, casesDomain.OutboxEventHumanCancelled, casesDomain.OutboxEventHumanCancelledV1,
		p.OutboxEventID, p, payload, casesDomain.DedupKeyHandoffEvent(p.CaseID, open.Request.RequestNo, eventNo))
}

// execReturnToStaffy writes the explicit return of the acknowledged
// lifecycle as ONE transaction with the 2.4 transfer machinery (the same
// composed CALL): the derived state of the LAST request must be the
// acknowledged acceptance, the result word belongs to the fixed
// dictionary, the ownership CAS must match, and the composed write lands
// atomically — the returned event with the result word, the
// TransferRequest back to the digital employee of the case (the
// external_inbox surface — the 2.4 word of the human Inbox contour), the
// acknowledged Resolution and the new ownership Term (the
// transfer_acknowledge origin, the owner=staffy, the epoch+1) with its
// ownership_changed envelope, plus the human_returned lifecycle envelope.
// The mode is NEVER touched by the return (AC-9: the ladder keeps its
// honest word — Staffy does not resume the sales path under the stricter
// modes).
func (s *Postgres) execReturnToStaffy(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := handoffCaseOpen(ctx, tx, tenantID, p.CaseID, p.Command, now); err != nil {
		return err
	}
	requests, events, err := readHandoffsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	last := handoffLastView(requests, events, now)
	if last == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: HandoffAbsentText}
	}
	if err := casesDomain.GuardReturnToStaffy(last.State, p.HandoffResult); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	ownershipTerms, transferRequests, resolutions, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(ownershipTerms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	if current.Epoch != p.ExpectedVersion {
		return casesDomain.ErrStaleVersion
	}
	if current.OwnerKind != casesDomain.OwnerKindHuman {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.StaffyOwnerText}
	}
	if casesDomain.ActiveTransferRequest(ownershipTerms, transferRequests, resolutions, now) != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.PendingTransferText}
	}
	// The digital employee of the case is the staffy owner truth (the
	// owner_ref resolution of the 2.4 dictionary: a member or the case
	// employee — the return composes to the case's own employee).
	var employeeID id.UUID
	if err := tx.QueryRow(ctx, `
                select employee_id from cases.cases where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&employeeID); err != nil {
		return err
	}
	// The returned lifecycle event: the result word of the fixed
	// dictionary (the free material never exists).
	eventNo, err := handoffEventNo(ctx, tx, tenantID, p.CaseID, last.Request.RequestNo)
	if err != nil {
		return err
	}
	returnedAt := now
	event := casesDomain.HandoffEvent{
		TenantID: tenantID, CaseID: p.CaseID, RequestNo: last.Request.RequestNo, EventNo: eventNo,
		Kind: casesDomain.HandoffEventReturned, Result: p.HandoffResult,
		ActorID: p.Actor, CreatedAt: returnedAt,
	}
	if err := insertHandoffEvent(ctx, tx, tenantID, p.Command, event); err != nil {
		return err
	}
	// The composed 2.4 machinery: the TransferRequest back to the digital
	// employee with the honest dictionary result constant, the acknowledged
	// Resolution, the ownership Term of the epoch+1 (owner=staffy, the
	// external_inbox surface).
	if err := insertHandoffTransfer(ctx, tx, tenantID, p, casesDomain.TransferRequest{
		TenantID: tenantID, CaseID: p.CaseID, RequestID: p.TransferRequestID,
		FromOwnerKind: current.OwnerKind, FromOwnerRef: current.OwnerRef,
		ToOwnerKind: casesDomain.OwnerKindStaffy, ToOwnerRef: employeeID,
		WritableSurface: casesDomain.WritableSurfaceExternalInbox,
		Reason:          casesDomain.HandoffResultText(p.HandoffResult),
		RequestedBy:     p.Actor, DeadlineAt: now.Add(casesDomain.HandoffAckDeadlineWindow(last.Request.AckDeadline)),
		CreatedAt: returnedAt,
	}); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.transfer_resolutions (tenant_id, resolution_id, request_id, kind, actor_id, resulting_epoch, reason, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8)`,
		tenantID, p.TransferResolutionID, p.TransferRequestID, casesDomain.TransferResolutionAcknowledged,
		p.Actor, current.Epoch+1, nilText(p.Reason), returnedAt); err != nil {
		return mapHandoffInsertError(err, p.Command)
	}
	if err := insertOwnershipTerm(ctx, tx, tenantID, p, casesDomain.OwnershipTerm{
		TenantID: tenantID, CaseID: p.CaseID, TermID: p.VerificationID,
		Epoch: current.Epoch + 1, OwnerKind: casesDomain.OwnerKindStaffy, OwnerRef: employeeID,
		WritableSurface: casesDomain.WritableSurfaceExternalInbox, Origin: casesDomain.OwnershipOriginTransferAcknowledge,
		PreviousOwnerKind: current.OwnerKind, PreviousOwnerRef: current.OwnerRef,
		ActorID: p.Actor, TransferRequestID: p.TransferRequestID,
		RequestedAt: returnedAt, AcknowledgedAt: returnedAt, CreatedAt: returnedAt,
	}, returnedAt); err != nil {
		return err
	}
	payload := handoffOutboxEvent(tenantID, last.Request, event, returnedAt)
	return insertHandoffOutbox(ctx, tx, tenantID, casesDomain.OutboxEventHumanReturned, casesDomain.OutboxEventHumanReturnedV1,
		p.HandoffOutboxEventID, p, payload, casesDomain.DedupKeyHandoffEvent(p.CaseID, last.Request.RequestNo, eventNo))
}

// insertHandoffTransfer writes the composed TransferRequest row of the
// handoff transitions (the execRequestTransfer material under the same
// locks — the request number is the NEXT monotonic number of the case):
// the dictionaries of 2.4 are never extended; the reason is the honest
// dictionary constant of the handoff surface (the operator never types
// it).
func insertHandoffTransfer(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, request casesDomain.TransferRequest) error {
	var requestNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(request_no),0)+1 from cases.transfer_requests
                where tenant_id=$1 and case_id=$2`,
		tenantID, request.CaseID).Scan(&requestNo); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
                insert into cases.transfer_requests (tenant_id, case_id, request_id, request_no, from_owner_kind, from_owner_ref,
                        to_owner_kind, to_owner_ref, writable_surface, reason, requested_by, deadline_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		tenantID, request.CaseID, request.RequestID, requestNo, request.FromOwnerKind, request.FromOwnerRef,
		request.ToOwnerKind, request.ToOwnerRef, request.WritableSurface, request.Reason,
		request.RequestedBy, request.DeadlineAt, request.CreatedAt); err != nil {
		return mapHandoffInsertError(err, p.Command)
	}
	return nil
}

// commitmentHandoffGateOfTx re-validates the Story 2.13 freeze gate of
// the new commitments under the advisory locks (the store half of the
// app gate — the live derivation of the 00013 rows the app could not
// read before the migration): the effective mode of the Work Item (the
// stricter of the conversation ladder and the item's own context) EQUALS
// human_requested, OR the case carries an ACTIVE human handoff — the
// creation refuses with the honest mode word. The control branch stays
// byte-identical: without the mode and without the handoff rows the gate
// returns nil and the 2.9 path never differs.
func commitmentHandoffGateOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID id.UUID, command string, now time.Time) error {
	terms, err := readModeMaterialTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return err
	}
	requests, events, err := readHandoffsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return err
	}
	handoffActive := false
	for i := range requests {
		view := casesDomain.DeriveHandoffState(requests[i], events, now)
		if view.Active() {
			handoffActive = true
			break
		}
	}
	effective := casesDomain.EffectiveWorkMode(
		casesDomain.ActiveConversationMode(terms),
		casesDomain.WorkItemModeContext(terms, workItemID))
	if err := casesDomain.GuardCommitmentFreeze(effective, handoffActive); err != nil {
		return casesDomain.WorkRuleError{Rule: command, Reason: err.Error()}
	}
	return nil
}
