package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// The Story 2.3 store surface: the reads of the assurance, binding and
// duplicate views, the identifier key of the HMAC tokens and the four
// transactional commands. Everything is append-only: the activity of a
// binding is the anti-join with the reversals, the candidate state is the
// existence of a resolution, the assurance is the read-time derivation.
// ---------------------------------------------------------------------------

// WithIDs wires the id generator of the in-transaction inserts (the
// candidate rows of the record_fact tokenization are born inside the
// transaction — their count is the lock-visible match count, unknown to the
// app before the locks).
func (s *Postgres) WithIDs(ids casesAPI.IDGenerator) *Postgres {
	s.ids = ids
	return s
}

// ReadConversationOfCase reads one conversation of the case with the
// modality of its pre-case session (the method eligibility dictionary
// input).
func (s *Postgres) ReadConversationOfCase(ctx context.Context, scope tenancy.Context, caseID, conversationID id.UUID) (casesDomain.Conversation, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesDomain.Conversation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var conversation casesDomain.Conversation
	err = tx.QueryRow(ctx, `
		select c.conversation_id, c.case_id, c.conversation_no, c.employee_id, c.connection_id, c.provider, c.provider_session_id, c.response_owner_id, c.response_owner_epoch, c.writable_surface, c.state, c.created_at, coalesce(p.modality, '')
		from cases.conversations c
		left join lateral (
			select modality from cases.pre_case_sessions p
			where p.tenant_id = c.tenant_id and p.connection_id = c.connection_id
			  and p.provider = c.provider and p.provider_session_id = c.provider_session_id
			limit 1
		) p on true
		where c.tenant_id=$1 and c.case_id=$2 and c.conversation_id=$3`,
		scope.TenantID(), caseID, conversationID).
		Scan(&conversation.ConversationID, &conversation.CaseID, &conversation.ConversationNo,
			&conversation.EmployeeID, &conversation.ConnectionID, &conversation.Provider, &conversation.ProviderSessionID,
			&conversation.ResponseOwnerID, &conversation.ResponseOwnerEpoch, &conversation.WritableSurface,
			&conversation.State, &conversation.CreatedAt, &conversation.Modality)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.Conversation{}, casesDomain.ErrNotFound
	}
	if err != nil {
		return casesDomain.Conversation{}, err
	}
	conversation.TenantID = scope.TenantID()
	conversation.CreatedAt = conversation.CreatedAt.UTC()
	return conversation, tx.Commit(ctx)
}

// ReadVerificationRecords reads the verification records of one case
// ordered by the issue time (the assurance derivation input).
func (s *Postgres) ReadVerificationRecords(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.VerificationRecord, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	records, err := readVerificationRecordsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return records, tx.Commit(ctx)
}

func readVerificationRecordsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.VerificationRecord, error) {
	rows, err := tx.Query(ctx, `
		select verification_id, case_id, conversation_id, kind, method, outcome, assurance_level, subject_handle, issuer_id, evidence_event_id, note, authority_scope, issued_at, expires_at, created_at
		from cases.verification_records
		where tenant_id=$1 and case_id=$2 order by issued_at, verification_id`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []casesDomain.VerificationRecord{}
	for rows.Next() {
		var record casesDomain.VerificationRecord
		var evidenceEventID *id.UUID
		var note, authorityScope *string
		if err = rows.Scan(&record.VerificationID, &record.CaseID, &record.ConversationID, &record.Kind, &record.Method,
			&record.Outcome, &record.AssuranceLevel, &record.SubjectHandle, &record.IssuerID, &evidenceEventID, &note,
			&authorityScope, &record.IssuedAt, &record.ExpiresAt, &record.CreatedAt); err != nil {
			return nil, err
		}
		record.TenantID = tenantID
		if evidenceEventID != nil {
			record.EvidenceEventID = *evidenceEventID
		}
		if note != nil {
			record.Note = *note
		}
		if authorityScope != nil {
			record.AuthorityScope = *authorityScope
		}
		record.IssuedAt = record.IssuedAt.UTC()
		record.ExpiresAt = record.ExpiresAt.UTC()
		record.CreatedAt = record.CreatedAt.UTC()
		records = append(records, record)
	}
	return records, rows.Err()
}

// ReadBindingsTouchingCase reads the bindings that target the case or bind
// one of its conversations, with the reversals (the activity anti-join
// input of the views and the gate re-validation).
func (s *Postgres) ReadBindingsTouchingCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ChannelBinding, []casesDomain.BindingReversal, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bindings, reversals, err := readBindingsTouchingCaseTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, nil, err
	}
	return bindings, reversals, tx.Commit(ctx)
}

func readBindingsTouchingCaseTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ChannelBinding, []casesDomain.BindingReversal, error) {
	rows, err := tx.Query(ctx, `
		select binding_id, case_id, conversation_id, bound_case_id, bound_conversation_id, method, purpose, on_behalf, subject_handle, evidence_event_id, note, actor_id, created_at
		from cases.channel_bindings
		where tenant_id=$1 and (case_id=$2 or bound_case_id=$2) order by created_at, binding_id`, tenantID, caseID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	bindings := []casesDomain.ChannelBinding{}
	for rows.Next() {
		var binding casesDomain.ChannelBinding
		var purpose, note *string
		var evidenceEventID *id.UUID
		if err = rows.Scan(&binding.BindingID, &binding.CaseID, &binding.ConversationID, &binding.BoundCaseID,
			&binding.BoundConversationID, &binding.Method, &purpose, &binding.OnBehalf, &binding.SubjectHandle,
			&evidenceEventID, &note, &binding.ActorID, &binding.CreatedAt); err != nil {
			return nil, nil, err
		}
		binding.TenantID = tenantID
		if purpose != nil {
			binding.Purpose = *purpose
		}
		if note != nil {
			binding.Note = *note
		}
		if evidenceEventID != nil {
			binding.EvidenceEventID = *evidenceEventID
		}
		binding.CreatedAt = binding.CreatedAt.UTC()
		bindings = append(bindings, binding)
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	reversals := []casesDomain.BindingReversal{}
	if len(bindings) > 0 {
		rows, err = tx.Query(ctx, `
			select reversal_id, binding_id, reason, actor_id, created_at
			from cases.binding_reversals where tenant_id=$1 order by created_at, reversal_id`, tenantID)
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var reversal casesDomain.BindingReversal
			if err = rows.Scan(&reversal.ReversalID, &reversal.BindingID, &reversal.Reason, &reversal.ActorID, &reversal.CreatedAt); err != nil {
				return nil, nil, err
			}
			reversal.TenantID = tenantID
			reversal.CreatedAt = reversal.CreatedAt.UTC()
			reversals = append(reversals, reversal)
		}
		if err = rows.Err(); err != nil {
			return nil, nil, err
		}
	}
	return bindings, reversals, nil
}

// ReadDuplicatesOfCase reads the candidate cards touching the case (the new
// or the existing side): the minimal review material — the candidate, both
// case numbers, the MASKED identifier value and the resolution. The
// protected content (payloads, fact values) never enters the card: the mask
// derives from the fact of the NEW case inside this read only.
func (s *Postgres) ReadDuplicatesOfCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.DuplicateCard, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cards, err := readDuplicatesOfCaseTx(ctx, tx, scope, caseID)
	if err != nil {
		return nil, err
	}
	return cards, tx.Commit(ctx)
}

func readDuplicatesOfCaseTx(ctx context.Context, tx pgx.Tx, scope tenancy.Context, caseID id.UUID) ([]casesPort.DuplicateCard, error) {
	tenantID := scope.TenantID()
	rows, err := tx.Query(ctx, `
		select d.candidate_id, d.case_id_new, d.case_id_existing, d.token, d.material_key, d.match_reason, d.rule_confidence, d.created_at,
		       nc.case_no, ec.case_no
		from cases.duplicate_candidates d
		join cases.cases nc on nc.tenant_id = d.tenant_id and nc.case_id = d.case_id_new
		join cases.cases ec on ec.tenant_id = d.tenant_id and ec.case_id = d.case_id_existing
		where d.tenant_id=$1 and (d.case_id_new=$2 or d.case_id_existing=$2)
		order by d.created_at, d.candidate_id`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cards := []casesPort.DuplicateCard{}
	type candidateRow struct {
		card casesPort.DuplicateCard
	}
	var pending []candidateRow
	for rows.Next() {
		var row candidateRow
		if err = rows.Scan(&row.card.Candidate.CandidateID, &row.card.Candidate.CaseIDNew, &row.card.Candidate.CaseIDExisting,
			&row.card.Candidate.Token, &row.card.Candidate.MaterialKey, &row.card.Candidate.MatchReason,
			&row.card.Candidate.RuleConfidence, &row.card.Candidate.CreatedAt, &row.card.NewCaseNo, &row.card.ExistingCaseNo); err != nil {
			return nil, err
		}
		row.card.Candidate.TenantID = tenantID
		row.card.Candidate.CreatedAt = row.card.Candidate.CreatedAt.UTC()
		pending = append(pending, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	// The masked value: the LAST TWO runes of the normalized identifier of
	// THE VERSION WHOSE TOKEN MATCHED the candidate (the value that created
	// the match — a later rewrite of the fact does not shift the mask). The
	// value itself stays in the fact (the review right of the ordinary
	// surfaces); the card carries the mask only. The channel of the card is
	// the channel of the first accepted event of the new case (the honest
	// "which channel" of the review material).
	// The identifier key is read ONCE before the version cursors (pgx
	// refuses the nested queries of an open rows cursor on one connection).
	// The read path is READ-ONLY: the key exists by the tokenization time —
	// an absent key degrades the mask honestly (never an insert in a read).
	identifierKey, keyErr := readIdentifierKeyReadOnly(ctx, tx, tenantID)
	if keyErr != nil {
		return nil, keyErr
	}
	for i := range pending {
		rows, versionsErr := tx.Query(ctx, `
			select fv.value from cases.facts f
			join lateral (
				select value from cases.fact_versions
				where tenant_id = f.tenant_id and fact_id = f.fact_id
				order by version_no
			) fv on true
			where f.tenant_id=$1 and f.case_id=$2 and f.material_key=$3`,
			tenantID, pending[i].card.Candidate.CaseIDNew, pending[i].card.Candidate.MaterialKey)
		if versionsErr != nil {
			return nil, versionsErr
		}
		for rows.Next() {
			var value []byte
			if err = rows.Scan(&value); err != nil {
				rows.Close()
				return nil, err
			}
			if identifierKey != nil {
				if raw, ok := casesDomain.IdentifierStringValue(value); ok {
					if normalized, normalizeErr := casesDomain.NormalizeIdentifier(pending[i].card.Candidate.MaterialKey, raw); normalizeErr == nil {
						if casesDomain.IdentifierToken(identifierKey, pending[i].card.Candidate.MaterialKey, normalized) == pending[i].card.Candidate.Token {
							pending[i].card.MaskedValue = casesDomain.MaskIdentifier(normalized)
						}
					}
				}
			}
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		var channel *string
		if err = tx.QueryRow(ctx, `
			select channel from cases.channel_events e
			where e.tenant_id=$1 and e.case_id=$2 and e.state='accepted'
			order by e.received_at, e.event_id limit 1`,
			tenantID, pending[i].card.Candidate.CaseIDNew).Scan(&channel); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if channel != nil {
			pending[i].card.Channel = *channel
		}

		var resolution casesDomain.DuplicateResolution
		var note string
		err = tx.QueryRow(ctx, `
			select resolution_id, candidate_id, decision, note, actor_id, created_at
			from cases.duplicate_resolutions where tenant_id=$1 and candidate_id=$2`,
			tenantID, pending[i].card.Candidate.CandidateID).
			Scan(&resolution.ResolutionID, &resolution.CandidateID, &resolution.Decision, &note, &resolution.ActorID, &resolution.CreatedAt)
		if err == nil {
			resolution.TenantID = tenantID
			resolution.Note = note
			resolution.CreatedAt = resolution.CreatedAt.UTC()
			pending[i].card.Resolution = &resolution
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		pending[i].card.IsNewSide = pending[i].card.Candidate.CaseIDNew == caseID
		cards = append(cards, pending[i].card)
	}
	return cards, nil
}

// ReadIdentifierKey reads (or creates on the first use) the per-tenant HMAC
// key of the identifier tokens. The creation race resolves by ON CONFLICT
// DO NOTHING + re-read (INSERT-only, no update). The now is the caller's
// command time (the explicit clock discipline of the module).
func (s *Postgres) ReadIdentifierKey(ctx context.Context, scope tenancy.Context, now time.Time) ([]byte, error) {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	key, err := readIdentifierKeyTx(ctx, tx, scope, now.UTC().Truncate(time.Microsecond))
	if err != nil {
		return nil, err
	}
	return key, tx.Commit(ctx)
}

// readIdentifierKeyReadOnly reads the per-tenant identifier key without
// creation (the read-only transactions of the views).
func readIdentifierKeyReadOnly(ctx context.Context, tx pgx.Tx, tenantID id.UUID) ([]byte, error) {
	var stored []byte
	err := tx.QueryRow(ctx, `select key from cases.identifier_hmac_keys where tenant_id=$1`, tenantID).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // the honest degrade: no key, no mask
	}
	return stored, err
}

func readIdentifierKeyTx(ctx context.Context, tx pgx.Tx, scope tenancy.Context, now time.Time) ([]byte, error) {
	key, err := casesDomain.NewIdentifierKey()
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `
		insert into cases.identifier_hmac_keys (tenant_id, key, created_at)
		values ($1,$2,$3) on conflict (tenant_id) do nothing`,
		scope.TenantID(), key, now); err != nil {
		return nil, err
	}
	var stored []byte
	if err = tx.QueryRow(ctx, `
		select key from cases.identifier_hmac_keys where tenant_id=$1`, scope.TenantID()).Scan(&stored); err != nil {
		return nil, err
	}
	return stored, nil
}

func mapVerificationInsertError(command string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch {
		case pgErr.ConstraintName == "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case pgErr.ConstraintName == "binding_reversals_tenant_id_binding_id_key":
			return casesDomain.ErrCommandKeyConflict
		case pgErr.ConstraintName == "duplicate_resolutions_tenant_id_candidate_id_key":
			return casesDomain.ErrCommandKeyConflict
		}
	}
	return err
}

// execStartVerification writes the append-only verification record under
// the case lock: the step-up window re-validates (a raced crossing refuses
// honestly), the record inserts with the canonical outcome and the computed
// expiry, and the record that CROSSES the threshold writes the
// identity_step_up exception in the same transaction (no identifier values,
// no secrets — the summary is the fixed text).
func (s *Postgres) execStartVerification(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	// The conversation containment: the verification context belongs to
	// the case.
	var conversationCase id.UUID
	err := tx.QueryRow(ctx, `
		select case_id from cases.conversations where tenant_id=$1 and case_id=$2 and conversation_id=$3`,
		tenantID, p.CaseID, p.ConversationID).Scan(&conversationCase)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	// The step-up re-validation under the lock: the raced crossing of a
	// concurrent writer refuses honestly (the operator retries after the
	// window).
	records, err := readVerificationRecordsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	if casesDomain.DeriveAssuranceState(records, now).StepUp {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.StepUpText}
	}
	// The evidence validation: the event evidence must be an accepted event
	// of this case.
	if p.EvidenceEventID.Valid() {
		var eventState string
		err := tx.QueryRow(ctx, `
			select state from cases.channel_events where tenant_id=$1 and case_id=$2 and event_id=$3`,
			tenantID, p.CaseID, p.EvidenceEventID).Scan(&eventState)
		if errors.Is(err, pgx.ErrNoRows) {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: "событие-evidence не найдено среди событий этого обращения"}
		}
		if err != nil {
			return err
		}
		if eventState != casesDomain.EventStateAccepted {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: "событие-evidence не принято"}
		}
	}
	if _, err = tx.Exec(ctx, `
		insert into cases.verification_records (tenant_id, case_id, conversation_id, verification_id, kind, method, outcome, assurance_level, subject_handle, issuer_id, evidence_event_id, note, authority_scope, issued_at, expires_at, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		tenantID, p.CaseID, p.ConversationID, p.VerificationID, p.Kind, p.Method, p.OutcomeName, p.AssuranceLevel,
		p.SubjectHandle, p.Actor, nilID(p.EvidenceEventID), nilText(p.Note), nilText(p.AuthorityScope),
		now, p.ExpiresAt, now); err != nil {
		return mapVerificationInsertError(p.Command, err)
	}
	// The threshold crossing: the projected window reaches the limit with
	// this record — the exception lands in the same transaction.
	projected := append(append([]casesDomain.VerificationRecord(nil), records...), casesDomain.VerificationRecord{
		Kind: p.Kind, Outcome: p.OutcomeName, IssuedAt: now,
	})
	if casesDomain.DeriveAssuranceState(projected, now).StepUp {
		summary := "Порог неудачных проверок обращения достигнут: шаги проверки и связывания приостановлены на окно порога (значения идентификаторов не раскрываются)."
		if _, err = tx.Exec(ctx, `
			insert into cases.operational_exceptions (tenant_id, exception_id, kind, summary, correlation_id, event_id, original_event_id, case_id, conversation_id, created_at)
			values ($1,$2,$3,$4,$5,NULL,NULL,$6,$7,$8)`,
			tenantID, p.ExceptionID, casesDomain.ExceptionIdentityStepUp, summary,
			p.ExceptionID, p.CaseID, p.ConversationID, now); err != nil {
			return err
		}
	}
	return nil
}

// execBindChannel writes the append-only binding under the conversation
// locks: the target and the bound conversations re-validate in their
// cases, the no-active-binding check runs under the lock, and the consent
// handoff gate re-derives the assurance of the target case under the case
// lock (a raced expiring verification refuses honestly).
func (s *Postgres) execBindChannel(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := validateBindingPair(ctx, tx, tenantID, p); err != nil {
		return err
	}
	if err := assertNoActiveBinding(ctx, tx, tenantID, p.BoundConversationID); err != nil {
		return err
	}
	// The step-up re-validation covers EVERY binding method (the threshold
	// window suspends the linking steps as a class — AC-10); the identity
	// gate stays the consent-handoff material (the duplicate review carries
	// the reviewer's decision as its own check).
	if records, recordsErr := readVerificationRecordsTx(ctx, tx, tenantID, p.CaseID); recordsErr != nil {
		return recordsErr
	} else if state := casesDomain.DeriveAssuranceState(records, now); state.StepUp {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.StepUpText}
	} else if p.Method == casesDomain.BindingMethodConsentHandoff && !state.BindingAllowed(p.OnBehalf) {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.BindingGateText(p.OnBehalf)}
	}
	if p.EvidenceEventID.Valid() {
		var eventState string
		err := tx.QueryRow(ctx, `
			select state from cases.channel_events where tenant_id=$1 and case_id=$2 and event_id=$3`,
			tenantID, p.CaseID, p.EvidenceEventID).Scan(&eventState)
		if errors.Is(err, pgx.ErrNoRows) {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: "событие-evidence не найдено среди событий этого обращения"}
		}
		if err != nil {
			return err
		}
		if eventState != casesDomain.EventStateAccepted {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: "событие-evidence не принято"}
		}
	}
	return insertBinding(ctx, tx, tenantID, p, now)
}

// insertBinding writes the binding row (the shared path of the consent
// handoff and the duplicate resolution).
func insertBinding(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	_, err := tx.Exec(ctx, `
		insert into cases.channel_bindings (tenant_id, binding_id, case_id, conversation_id, bound_case_id, bound_conversation_id, method, purpose, on_behalf, subject_handle, evidence_event_id, note, actor_id, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		tenantID, p.BindingID, p.CaseID, p.ConversationID, p.BoundCaseID, p.BoundConversationID,
		p.Method, nilText(p.Purpose), p.OnBehalf, p.SubjectHandle, nilID(p.EvidenceEventID), nilText(p.Note),
		p.Actor, now)
	return mapVerificationInsertError(p.Command, err)
}

// validateBindingPair re-validates the target and the bound conversations
// under the locks: both exist, both belong to their declared cases, and the
// two cases differ.
func validateBindingPair(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams) error {
	var targetCase, boundCase id.UUID
	err := tx.QueryRow(ctx, `
		select case_id from cases.conversations where tenant_id=$1 and conversation_id=$2`,
		tenantID, p.ConversationID).Scan(&targetCase)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `
		select case_id from cases.conversations where tenant_id=$1 and conversation_id=$2`,
		tenantID, p.BoundConversationID).Scan(&boundCase)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if targetCase != p.CaseID || boundCase != p.BoundCaseID {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "разговоры не принадлежат заявленным обращениям"}
	}
	if targetCase == boundCase {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "связывание объединяет обращения разных кейсов"}
	}
	return nil
}

// assertNoActiveBinding refuses a second active link of one conversation:
// a binding without a reversal.
func assertNoActiveBinding(ctx context.Context, tx pgx.Tx, tenantID id.UUID, conversationID id.UUID) error {
	var count int
	err := tx.QueryRow(ctx, `
		select count(*) from cases.channel_bindings b
		where b.tenant_id=$1 and b.bound_conversation_id=$2
		  and not exists (select 1 from cases.binding_reversals r
		                  where r.tenant_id = b.tenant_id and r.binding_id = b.binding_id)`,
		tenantID, conversationID).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return casesDomain.ErrActiveBinding
	}
	return nil
}

// execUnlinkChannel writes the append-only reversal: the binding must exist
// targeting this case and carry no reversal yet (the UNIQUE is the raced
// backstop).
func (s *Postgres) execUnlinkChannel(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	var boundConversation id.UUID
	err := tx.QueryRow(ctx, `
		select bound_conversation_id from cases.channel_bindings
		where tenant_id=$1 and binding_id=$2 and case_id=$3`,
		tenantID, p.BindingID, p.CaseID).Scan(&boundConversation)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	var reversalCount int
	err = tx.QueryRow(ctx, `
		select count(*) from cases.binding_reversals where tenant_id=$1 and binding_id=$2`,
		tenantID, p.BindingID).Scan(&reversalCount)
	if err != nil {
		return err
	}
	if reversalCount > 0 {
		return casesDomain.ErrCommandKeyConflict
	}
	_, err = tx.Exec(ctx, `
		insert into cases.binding_reversals (tenant_id, reversal_id, binding_id, reason, actor_id, created_at)
		values ($1,$2,$3,$4,$5,$6)`,
		tenantID, p.ReversalID, p.BindingID, p.Reason, p.Actor, now)
	return mapVerificationInsertError(p.Command, err)
}

// execResolveDuplicate decides the candidate: the resolution row plus (the
// same_client decision) the duplicate-review binding — one transaction, the
// atomicity of "decision + link". The separate decision writes the
// resolution only; the cases stay apart.
func (s *Postgres) execResolveDuplicate(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	// The candidate: the new case of the command.
	var caseNew, caseExisting id.UUID
	err := tx.QueryRow(ctx, `
		select case_id_new, case_id_existing from cases.duplicate_candidates
		where tenant_id=$1 and candidate_id=$2 and case_id_new=$3`,
		tenantID, p.CandidateID, p.CaseID).Scan(&caseNew, &caseExisting)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	var resolutionCount int
	err = tx.QueryRow(ctx, `
		select count(*) from cases.duplicate_resolutions where tenant_id=$1 and candidate_id=$2`,
		tenantID, p.CandidateID).Scan(&resolutionCount)
	if err != nil {
		return err
	}
	if resolutionCount > 0 {
		return casesDomain.ErrCommandKeyConflict
	}
	if _, err = tx.Exec(ctx, `
		insert into cases.duplicate_resolutions (tenant_id, resolution_id, candidate_id, decision, note, actor_id, created_at)
		values ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, p.ResolutionID, p.CandidateID, p.Decision, p.Note, p.Actor, now); err != nil {
		return mapVerificationInsertError(p.Command, err)
	}
	if p.Decision != casesDomain.DuplicateDecisionSameClient {
		return nil
	}
	// The duplicate-review binding: the target conversation of the
	// existing case and the bound conversation of the new case; the
	// reviewer is the actor, the resolution note is the basis.
	binding := casesPort.WorkCommandParams{
		Command: p.Command, CaseID: caseExisting, ConversationID: p.ConversationID,
		BoundCaseID: caseNew, BoundConversationID: p.BoundConversationID,
		Method: casesDomain.BindingMethodDuplicateReview, SubjectHandle: p.SubjectHandle,
		Actor: p.Actor, BindingID: p.BindingID,
	}
	if err := validateBindingPair(ctx, tx, tenantID, binding); err != nil {
		return err
	}
	if err := assertNoActiveBinding(ctx, tx, tenantID, p.BoundConversationID); err != nil {
		return err
	}
	return insertBinding(ctx, tx, tenantID, binding, now)
}

// execIdentifierToken inserts the identifier token of the fact and creates
// the duplicate candidates for every EXACT token match in another OPEN case
// (under the token lock — the match set is the lock-visible truth). The
// candidate rows never carry values: the token is the HMAC; the review card
// masks the value at the read.
func execIdentifierToken(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time, s *Postgres) error {
	if _, err := tx.Exec(ctx, `
		insert into cases.identifier_tokens (tenant_id, token, case_id, material_key, created_at)
		values ($1,$2,$3,$4,$5) on conflict (tenant_id, token, case_id) do nothing`,
		tenantID, p.IdentifierToken, p.CaseID, p.MaterialKey, now); err != nil {
		return mapVerificationInsertError(p.Command, err)
	}
	rows, err := tx.Query(ctx, `
		select t.case_id from cases.identifier_tokens t
		join cases.cases c on c.tenant_id = t.tenant_id and c.case_id = t.case_id
		where t.tenant_id=$1 and t.token=$2 and t.case_id<>$3 and c.state='open'
		order by t.created_at, t.case_id`, tenantID, p.IdentifierToken, p.CaseID)
	if err != nil {
		return err
	}
	defer rows.Close()
	matches := []id.UUID{}
	for rows.Next() {
		var caseID id.UUID
		if err = rows.Scan(&caseID); err != nil {
			return err
		}
		matches = append(matches, caseID)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if s.ids == nil {
		return fmt.Errorf("генератор идентификаторов не сконфигурирован: токенизация идентификаторов честно отказывается")
	}
	for _, existing := range matches {
		candidateID, err := s.ids.New(now)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			insert into cases.duplicate_candidates (tenant_id, candidate_id, case_id_new, case_id_existing, token, material_key, match_reason, rule_confidence, created_at)
			values ($1,$2,$3,$4,$5,$6,$7,$8,$9) on conflict (tenant_id, case_id_new, case_id_existing, token) do nothing`,
			tenantID, candidateID, p.CaseID, existing, p.IdentifierToken, p.MaterialKey,
			casesDomain.MatchReasonExactIdentifier, casesDomain.RuleConfidenceHigh, now); err != nil {
			return mapVerificationInsertError(p.Command, err)
		}
	}
	return nil
}
