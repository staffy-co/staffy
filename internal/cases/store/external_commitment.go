package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 3.3: the external commitment dispatch writes and reads
// ---------------------------------------------------------------------------
//
// The envelope versions live under the case + work item advisory locks
// (the commitments 00009 precedent): the per-envelope monotonic version
// number is re-derived under the locks — the raced ladder is the honest
// stale-version refusal, never a silent merge. The enqueue transaction is
// the persist-before-emit core: its signature carries NO adapter, the ONE
// commit writes the outbox record with the safe payload only (the content
// never enters the envelope, AC-10), and a database failure leaves the
// external system untouched (the probe of the integration matrix). The
// dispatch history is append-only: the gasp, the attempt outcome and the
// relay receipt appends are CONDITIONAL under the locks — the raced twin
// writes nothing, the second Staffy intent never exists (the events are
// the first dedup line, the outbox dedup key the second).

// mapExternalInsertError maps the raced insert conflicts of the external
// commitment surface onto the honest domain errors: the version PK is the
// monotonic ladder backstop, the approval UNIQUE is the silent idempotency
// of the repeated approver, the event_no UNIQUE is the raced append twin,
// the outbox dedup key is the second enqueue line.
func mapExternalInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "external_commitments_pkey":
			return casesDomain.ErrStaleVersion
		case "external_dispatch_events_tenant_id_ext_commitment_id_version_no_event_no_key":
			return casesDomain.ErrEventRaced
		case "outbox_events_tenant_id_dedup_key_key":
			return casesDomain.WorkRuleError{Rule: "enqueue_external_commitment", Reason: "Версия уже стоит в очереди: повторный enqueue той же версии не создаёт второго намерения — дождитесь дренера или создайте новую версию."}
		}
	}
	return err
}

// readExternalCommitmentsTx reads every envelope version of one case
// ordered by the envelope and the monotonic number (the derivation input
// of the CURRENT version, the readiness and the enqueue gate).
func readExternalCommitmentsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ExternalCommitment, error) {
	rows, err := tx.Query(ctx, `
                select tenant_id, case_id, work_item_id, ext_commitment_id, version_no, purpose, recipient_kind, channel,
                       content_text, valid_until, expected_next_step, ack_requirement,
                       coalesce(pin_commitment_id, '00000000-0000-0000-0000-000000000000'), coalesce(pin_version_no, 0), coalesce(pin_content_token, ''),
                       owner_kind, owner_ref, owner_epoch, decision_no, profile_version, policy_outcome_no, content_token, actor_id, created_at
                from cases.external_commitments
                where tenant_id=$1 and case_id=$2 order by ext_commitment_id, version_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := []casesDomain.ExternalCommitment{}
	for rows.Next() {
		version, err := scanExternalCommitment(tenantID, rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

// readEnvelopeVersionsTx reads every version of ONE envelope identifier of
// the tenant ordered by the monotonic number (the pending-drain and the
// fence material).
func readEnvelopeVersionsTx(ctx context.Context, tx pgx.Tx, tenantID, extID id.UUID) ([]casesDomain.ExternalCommitment, error) {
	rows, err := tx.Query(ctx, `
                select tenant_id, case_id, work_item_id, ext_commitment_id, version_no, purpose, recipient_kind, channel,
                       content_text, valid_until, expected_next_step, ack_requirement,
                       coalesce(pin_commitment_id, '00000000-0000-0000-0000-000000000000'), coalesce(pin_version_no, 0), coalesce(pin_content_token, ''),
                       owner_kind, owner_ref, owner_epoch, decision_no, profile_version, policy_outcome_no, content_token, actor_id, created_at
                from cases.external_commitments
                where tenant_id=$1 and ext_commitment_id=$2 order by version_no`, tenantID, extID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := []casesDomain.ExternalCommitment{}
	for rows.Next() {
		version, err := scanExternalCommitment(tenantID, rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

// scanExternalCommitment decodes one envelope version row (the nullable
// pin triple reads as the zero triple — the empty commitment id is the
// honest «pin absent» word, the 2.9 nullable-pair precedent).
func scanExternalCommitment(tenantID id.UUID, r row) (casesDomain.ExternalCommitment, error) {
	var version casesDomain.ExternalCommitment
	if err := r.Scan(&version.TenantID, &version.CaseID, &version.WorkItemID, &version.ExtCommitmentID,
		&version.VersionNo, &version.Purpose, &version.RecipientKind, &version.Channel,
		&version.ContentText, &version.ValidUntil, &version.ExpectedNextStep, &version.AckRequirement,
		&version.Pin.CommitmentID, &version.Pin.VersionNo, &version.Pin.ContentToken,
		&version.OwnerKind, &version.OwnerRef, &version.OwnerEpoch, &version.DecisionNo,
		&version.ProfileVersion, &version.PolicyOutcomeNo, &version.ContentToken,
		&version.ActorID, &version.CreatedAt); err != nil {
		return version, err
	}
	if version.Pin.CommitmentID == id.UUID("00000000-0000-0000-0000-000000000000") {
		version.Pin = casesDomain.ExternalCommitmentPin{}
	}
	version.TenantID = tenantID
	version.ValidUntil = version.ValidUntil.UTC()
	version.CreatedAt = version.CreatedAt.UTC()
	return version, nil
}

// externalVersionOfTx reads one EXACT version of one envelope under the
// locks (the approval, the enqueue and the relay anchor re-validation:
// the version must exist and belong to this work item of this case).
func externalVersionOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID, extID id.UUID, versionNo int64) (casesDomain.ExternalCommitment, error) {
	var version casesDomain.ExternalCommitment
	err := tx.QueryRow(ctx, `
                select tenant_id, case_id, work_item_id, ext_commitment_id, version_no, purpose, recipient_kind, channel,
                       content_text, valid_until, expected_next_step, ack_requirement,
                       coalesce(pin_commitment_id, '00000000-0000-0000-0000-000000000000'), coalesce(pin_version_no, 0), coalesce(pin_content_token, ''),
                       owner_kind, owner_ref, owner_epoch, decision_no, profile_version, policy_outcome_no, content_token, actor_id, created_at
                from cases.external_commitments
                where tenant_id=$1 and case_id=$2 and work_item_id=$3 and ext_commitment_id=$4 and version_no=$5`,
		tenantID, caseID, workItemID, extID, versionNo).Scan(
		&version.TenantID, &version.CaseID, &version.WorkItemID, &version.ExtCommitmentID,
		&version.VersionNo, &version.Purpose, &version.RecipientKind, &version.Channel,
		&version.ContentText, &version.ValidUntil, &version.ExpectedNextStep, &version.AckRequirement,
		&version.Pin.CommitmentID, &version.Pin.VersionNo, &version.Pin.ContentToken,
		&version.OwnerKind, &version.OwnerRef, &version.OwnerEpoch, &version.DecisionNo,
		&version.ProfileVersion, &version.PolicyOutcomeNo, &version.ContentToken,
		&version.ActorID, &version.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return version, casesDomain.ErrNotFound
	}
	if err != nil {
		return version, err
	}
	if version.Pin.CommitmentID == id.UUID("00000000-0000-0000-0000-000000000000") {
		version.Pin = casesDomain.ExternalCommitmentPin{}
	}
	version.ValidUntil = version.ValidUntil.UTC()
	version.CreatedAt = version.CreatedAt.UTC()
	return version, nil
}

// readExternalDispatchEventsOfTx reads the append-only history of one
// version ordered by the monotonic event number (the conditional append
// and the derivation input).
func readExternalDispatchEventsOfTx(ctx context.Context, tx pgx.Tx, tenantID, extID id.UUID, versionNo int64) ([]casesDomain.ExternalDispatchEvent, error) {
	rows, err := tx.Query(ctx, `
                select tenant_id, case_id, ext_commitment_id, version_no, event_id, event_no, kind, coalesce(reason, ''), attempt_no,
                       coalesce(external_correlation_id, ''), idempotent_guarantee, details, actor_id, created_at
                from cases.external_dispatch_events
                where tenant_id=$1 and ext_commitment_id=$2 and version_no=$3 order by event_no`,
		tenantID, extID, versionNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []casesDomain.ExternalDispatchEvent{}
	for rows.Next() {
		var event casesDomain.ExternalDispatchEvent
		if err := rows.Scan(&event.TenantID, &event.CaseID, &event.ExtCommitmentID, &event.VersionNo,
			&event.EventID, &event.EventNo, &event.Kind, &event.Reason, &event.AttemptNo,
			&event.ExternalCorrelationID, &event.IdempotentGuarantee, &event.Details,
			&event.ActorID, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.CreatedAt = event.CreatedAt.UTC()
		events = append(events, event)
	}
	return events, rows.Err()
}

// lockExternalAnchors takes the case + work item advisory locks in the
// deterministic order (the 2.2 discipline: sorted keys, every writer the
// same order).
func lockExternalAnchors(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID id.UUID) error {
	keys := []string{caseLockKey(tenantID, caseID), workLockKey(tenantID, workItemID)}
	for _, key := range keys {
		if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", key); err != nil {
			return err
		}
	}
	return nil
}

// ReadExternalCommitments reads every envelope version of one case (the
// own read transaction of the views and the gate pre-checks).
func (s *Postgres) ReadExternalCommitments(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ExternalCommitment, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	versions, err := readExternalCommitmentsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return versions, tx.Commit(ctx)
}

// ReadExternalApprovals reads every approval row of one case ordered by
// the creation time (the enqueue gate input).
func (s *Postgres) ReadExternalApprovals(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ExternalApproval, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select tenant_id, case_id, work_item_id, ext_commitment_id, version_no, approval_id, approver_id, step_up_verified, created_at
                from cases.external_commitment_approvals
                where tenant_id=$1 and case_id=$2 order by created_at`, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	approvals := []casesDomain.ExternalApproval{}
	for rows.Next() {
		var approval casesDomain.ExternalApproval
		if err := rows.Scan(&approval.TenantID, &approval.CaseID, &approval.WorkItemID,
			&approval.ExtCommitmentID, &approval.VersionNo, &approval.ApprovalID,
			&approval.ApproverID, &approval.StepUpVerified, &approval.CreatedAt); err != nil {
			return nil, err
		}
		approval.CreatedAt = approval.CreatedAt.UTC()
		approvals = append(approvals, approval)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return approvals, tx.Commit(ctx)
}

// ReadExternalDispatchEvents reads every dispatch event of one case
// ordered by the envelope, the version and the monotonic event number
// (the seven-state derivation input).
func (s *Postgres) ReadExternalDispatchEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ExternalDispatchEvent, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select tenant_id, case_id, ext_commitment_id, version_no, event_id, event_no, kind, coalesce(reason, ''), attempt_no,
                       coalesce(external_correlation_id, ''), idempotent_guarantee, details, actor_id, created_at
                from cases.external_dispatch_events
                where tenant_id=$1 and case_id=$2 order by ext_commitment_id, version_no, event_no`,
		scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []casesDomain.ExternalDispatchEvent{}
	for rows.Next() {
		var event casesDomain.ExternalDispatchEvent
		if err := rows.Scan(&event.TenantID, &event.CaseID, &event.ExtCommitmentID, &event.VersionNo,
			&event.EventID, &event.EventNo, &event.Kind, &event.Reason, &event.AttemptNo,
			&event.ExternalCorrelationID, &event.IdempotentGuarantee, &event.Details,
			&event.ActorID, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.CreatedAt = event.CreatedAt.UTC()
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, tx.Commit(ctx)
}

// ReadCommitmentVersionsByID reads every version of ONE 2.9 commitment
// identifier tenant-wide (the live pin gate input: the pinned version
// with its STORED token — the live match against the pin triple — and the
// maximal number of the readiness derivation).
func (s *Postgres) ReadCommitmentVersionsByID(ctx context.Context, scope tenancy.Context, commitmentID id.UUID) ([]casesDomain.CommitmentVersion, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	versions, err := readCommitmentVersionsByIDTx(ctx, tx, scope.TenantID(), commitmentID)
	if err != nil {
		return nil, err
	}
	return versions, tx.Commit(ctx)
}

// ReadExternalQueuedVersions reads the queue evidence of one case: the
// version pins that carry the external_commitment.queued outbox record
// (the aggregate of the record is the case — the enqueue wrote it so).
// The record IS the queued proof: its presence marks the version
// enqueued whatever the dispatch events said afterwards — the read feeds
// the Enqueued flag of the view derivation (the events refine the state,
// the queue evidence never disappears).
func (s *Postgres) ReadExternalQueuedVersions(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.ExternalQueuedVersion, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select (payload->>'ext_commitment_id')::uuid, (payload->>'version_no')::bigint
                from cases.outbox_events
                where tenant_id=$1 and aggregate_id=$2 and event_type='external_commitment.queued'
                order by occurred_at, event_id`, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	queued := []casesPort.ExternalQueuedVersion{}
	for rows.Next() {
		var pin casesPort.ExternalQueuedVersion
		if err := rows.Scan(&pin.ExtCommitmentID, &pin.VersionNo); err != nil {
			return nil, err
		}
		queued = append(queued, pin)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return queued, tx.Commit(ctx)
}

// readCommitmentVersionsByIDTx reads every version of one commitment
// identifier inside a caller transaction (the enqueue pin gate under the
// locks).
func readCommitmentVersionsByIDTx(ctx context.Context, tx pgx.Tx, tenantID, commitmentID id.UUID) ([]casesDomain.CommitmentVersion, error) {
	rows, err := tx.Query(ctx, `
                select commitment_id, case_id, work_item_id, version_no, status, conditions, action_set,
                       valid_until, consumption_limit, material_unknowns, owner_kind, owner_ref, owner_epoch,
                       lineage_word, policy_outcome_no, pinned_price_observation_no, content_token, actor_id, created_at
                from cases.commitments
                where tenant_id=$1 and commitment_id=$2 order by version_no`, tenantID, commitmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := []casesDomain.CommitmentVersion{}
	for rows.Next() {
		var version casesDomain.CommitmentVersion
		var conditions, actionSet, unknowns []byte
		if err := rows.Scan(&version.CommitmentID, &version.CaseID, &version.WorkItemID, &version.VersionNo,
			&version.Status, &conditions, &actionSet, &version.ValidUntil, &version.ConsumptionLimit,
			&unknowns, &version.OwnerKind, &version.OwnerRef, &version.OwnerEpoch, &version.LineageWord,
			&version.PolicyOutcomeNo, &version.PinnedPriceObservationNo, &version.ContentToken,
			&version.ActorID, &version.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(actionSet, &version.ActionSet); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(unknowns, &version.MaterialUnknowns); err != nil {
			return nil, err
		}
		version.TenantID = tenantID
		version.Conditions = conditions
		version.ValidUntil = version.ValidUntil.UTC()
		version.CreatedAt = version.CreatedAt.UTC()
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

// CreateExternalCommitment writes one envelope version under the case +
// work item advisory locks: the work item must be anchored and ALIVE (the
// terminal states refuse honestly), the expected monotonic number is
// re-derived under the locks (the raced ladder is the honest stale-version
// refusal), the material passes the domain guard again (the CHECK words
// backstop the dictionaries), the content token is re-computed from the
// material under the locks and the row inserts append-only. No outbox
// record: the enqueue is a separate command — the approval gate sits
// between the creation and the queue (AC-9).
func (s *Postgres) CreateExternalCommitment(ctx context.Context, p casesPort.ExternalCommitmentCreateParams) error {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if err := lockExternalAnchors(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	currents, err := readWorkItemCurrentsTx(ctx, tx, tenantID, p.CaseID, []id.UUID{p.WorkItemID})
	if err != nil {
		return err
	}
	current, ok := currents[p.WorkItemID]
	if !ok {
		return casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: "create_external_commitment", Reason: casesDomain.TerminalWorkItemCommitmentText}
	}
	// The domain guard again under the locks (the honest 400 backstopped
	// by the table CHECK words).
	version := casesDomain.ExternalCommitment{
		TenantID: tenantID, CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		ExtCommitmentID: p.ExtCommitmentID, VersionNo: p.VersionNo,
		Purpose: p.Purpose, RecipientKind: p.RecipientKind, Channel: p.Channel,
		ContentText: p.ContentText, ValidUntil: p.ValidUntil, ExpectedNextStep: p.ExpectedNextStep,
		AckRequirement: p.AckRequirement, Pin: p.Pin,
		OwnerKind: p.OwnerKind, OwnerRef: p.OwnerRef, OwnerEpoch: p.OwnerEpoch,
		DecisionNo: p.DecisionNo, ProfileVersion: p.ProfileVersion, PolicyOutcomeNo: p.PolicyOutcomeNo,
	}
	if err := casesDomain.GuardCreateExternalCommitment(casesDomain.CreateExternalCommitmentCommand{
		TenantID: tenantID, CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		ExtCommitmentID: p.ExtCommitmentID, VersionNo: p.VersionNo,
		Purpose: p.Purpose, RecipientKind: p.RecipientKind, Channel: p.Channel,
		ContentText: p.ContentText, ValidUntil: p.ValidUntil,
		ExpectedNextStep: p.ExpectedNextStep, AckRequirement: p.AckRequirement, Pin: p.Pin,
		OwnerKind: p.OwnerKind, OwnerRef: p.OwnerRef, OwnerEpoch: p.OwnerEpoch,
		DecisionNo: p.DecisionNo, ProfileVersion: p.ProfileVersion, PolicyOutcomeNo: p.PolicyOutcomeNo,
	}, p.Now); err != nil {
		return casesDomain.WorkRuleError{Rule: "create_external_commitment", Reason: err.Error()}
	}
	// The per-envelope monotonic ladder under the lock: the app resolved
	// the expected number from its read; a raced create moved the ladder
	// — the honest stale-version refusal (the token never binds the
	// wrong version bytes).
	envelope, err := readEnvelopeVersionsTx(ctx, tx, tenantID, p.ExtCommitmentID)
	if err != nil {
		return err
	}
	versionNo := int64(0)
	for _, existing := range envelope {
		if existing.VersionNo > versionNo {
			versionNo = existing.VersionNo
		}
	}
	versionNo++
	if versionNo != p.VersionNo {
		return casesDomain.ErrStaleVersion
	}
	// The content binding token re-computed from the material under the
	// locks with the per-tenant commitment key (the 2.9 reuse; the
	// ensure pattern — INSERT-only, the rotation honestly deferred).
	key, err := readCommitmentKeyTx(ctx, tx, p.Scope, p.Now)
	if err != nil {
		return err
	}
	if token := casesDomain.ExternalCommitmentContentToken(key, version); token != p.ContentToken {
		return casesDomain.ErrStaleVersion
	}
	var pinCommitmentID any
	var pinVersionNo any
	var pinContentToken any
	if p.Pin.Present() {
		pinCommitmentID = p.Pin.CommitmentID
		pinVersionNo = p.Pin.VersionNo
		pinContentToken = p.Pin.ContentToken
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.external_commitments (tenant_id, case_id, work_item_id, ext_commitment_id, version_no,
                        purpose, recipient_kind, channel, content_text, valid_until, expected_next_step, ack_requirement,
                        pin_commitment_id, pin_version_no, pin_content_token,
                        owner_kind, owner_ref, owner_epoch, decision_no, profile_version, policy_outcome_no,
                        content_token, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`,
		tenantID, p.CaseID, p.WorkItemID, p.ExtCommitmentID, p.VersionNo,
		p.Purpose, p.RecipientKind, p.Channel, p.ContentText,
		p.ValidUntil.UTC().Truncate(time.Microsecond), p.ExpectedNextStep, p.AckRequirement,
		pinCommitmentID, pinVersionNo, pinContentToken,
		p.OwnerKind, p.OwnerRef, p.OwnerEpoch, p.DecisionNo, p.ProfileVersion, p.PolicyOutcomeNo,
		p.ContentToken, p.ActorID, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return mapExternalInsertError(err)
	}
	return tx.Commit(ctx)
}

// ApproveExternalCommitment writes one approval row of the EXACT version
// by the CURRENT human owner under the locks: the version must exist, the
// live ownership must be a HUMAN owner equal to the approver and the live
// epoch must equal the version snapshot (a moved owner demands a NEW
// version — the old approval structurally never authorises the changed
// text, AC-9). The UNIQUE (tenant, envelope, version, approver) makes the
// repeated approval of the same approver the silent idempotency (false).
// No outbox record (AC-9).
func (s *Postgres) ApproveExternalCommitment(ctx context.Context, p casesPort.ExternalApprovalParams) (bool, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if err := lockExternalAnchors(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return false, err
	}
	version, err := externalVersionOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExtCommitmentID, p.VersionNo)
	if err != nil {
		return false, err
	}
	terms, _, _, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return false, err
	}
	current := casesDomain.CurrentOwnership(terms)
	if current == nil {
		return false, casesDomain.ErrNotFound
	}
	if current.OwnerKind != casesDomain.OwnerKindHuman || current.OwnerRef != p.ApproverID {
		return false, casesDomain.WorkRuleError{Rule: "approve_external_commitment", Reason: "Утверждать версию конверта может только текущий человек-владелец кейса (response owner): подтвердите передачу владения или создайте версию под текущим владельцем."}
	}
	if current.Epoch != version.OwnerEpoch {
		return false, casesDomain.WorkRuleError{Rule: "approve_external_commitment", Reason: "Владелец сменился после создания версии: утверждение требует новой версии конверта под текущим владельцем (старый approval не разрешает изменённый текст)."}
	}
	approval, err := tx.Exec(ctx, `
                insert into cases.external_commitment_approvals (tenant_id, case_id, work_item_id, ext_commitment_id, version_no, approval_id, approver_id, step_up_verified, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
                on conflict (tenant_id, ext_commitment_id, version_no, approver_id) do nothing`,
		tenantID, p.CaseID, p.WorkItemID, p.ExtCommitmentID, p.VersionNo,
		p.ApprovalID, p.ApproverID, p.StepUpVerified, p.Now.UTC().Truncate(time.Microsecond))
	if err != nil {
		return false, mapExternalInsertError(err)
	}
	if approval.RowsAffected() == 0 {
		return false, tx.Commit(ctx)
	}
	return true, tx.Commit(ctx)
}

// EnqueueExternalCommitment is the persist-before-emit transaction: ONE
// write transaction re-validates the approval of the EXACT version by the
// CURRENT human owner (the live ownership under the locks) and the
// readiness (the live material: the envelope ladder, the window, the live
// epoch and the offer pin readiness of the LIVE 2.9 row), then inserts
// the outbox record 'external_commitment.queued' with the safe payload
// (ids, the version pin, the dictionary words, the idempotency key and
// the content digest — the content NEVER enters the envelope, AC-10).
// The signature carries NO adapter: the network call is physically
// impossible before the commit (AC-3); a database failure leaves the
// external system untouched. The dedup key of the version pin is the
// UNIQUE second line — the repeated enqueue of the same version is the
// honest refusal.
func (s *Postgres) EnqueueExternalCommitment(ctx context.Context, p casesPort.ExternalEnqueueParams) error {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if err := lockExternalAnchors(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	version, err := externalVersionOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExtCommitmentID, p.VersionNo)
	if err != nil {
		return err
	}
	// The approval gate under the locks: the LIVE owner must be the human
	// approver of the EXACT version (AC-9) — an absent approval, a
	// departed approver, a staffy owner or a moved epoch all refuse with
	// the same honest words, and NOTHING is written.
	terms, _, _, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(terms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	approved := false
	if current.OwnerKind == casesDomain.OwnerKindHuman && current.Epoch == version.OwnerEpoch {
		var found int
		err = tx.QueryRow(ctx, `
                        select 1 from cases.external_commitment_approvals
                        where tenant_id=$1 and ext_commitment_id=$2 and version_no=$3 and approver_id=$4`,
			tenantID, p.ExtCommitmentID, p.VersionNo, current.OwnerRef).Scan(&found)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		approved = err == nil
	}
	if !approved {
		return casesDomain.WorkRuleError{Rule: "enqueue_external_commitment", Reason: casesDomain.ExternalApprovalRequiredText(p.VersionNo)}
	}
	// The readiness under the locks: the live ladder, the window, the
	// live epoch and — for the offer — the LIVE readiness of the pinned
	// 2.9 version (the stored token match against the pin triple: the
	// stored word is the truth, never a re-computation over the jsonb
	// bytes).
	pinReadiness := casesDomain.CommitmentReadinessConfirmable
	if version.Purpose == casesDomain.ExternalPurposeOffer {
		pinReadiness, err = externalPinReadinessOfTx(ctx, tx, tenantID, version.Pin)
		if err != nil {
			return err
		}
	}
	envelope, err := readEnvelopeVersionsTx(ctx, tx, tenantID, p.ExtCommitmentID)
	if err != nil {
		return err
	}
	maxVersionNo := int64(0)
	for _, existing := range envelope {
		if existing.VersionNo > maxVersionNo {
			maxVersionNo = existing.VersionNo
		}
	}
	readiness := casesDomain.DeriveExternalReadiness(version, casesDomain.ExternalReadinessInput{
		Now: p.Now, MaxVersionNo: maxVersionNo, OwnerEpochCurrent: current.Epoch, PinReadiness: pinReadiness,
	})
	if err := casesDomain.GuardEnqueueExternalCommitment(readiness); err != nil {
		return casesDomain.WorkRuleError{Rule: "enqueue_external_commitment", Reason: err.Error()}
	}
	// The safe payload (the content NEVER enters — AC-10): the
	// identifiers, the version pin, the dictionary words, the
	// idempotency key derivative of the version pin and the content
	// digest.
	payload, err := casesDomain.ExternalCommitmentQueuedPayloadJSON(casesDomain.ExternalCommitmentQueuedPayload{
		TenantID: tenantID, CaseID: p.CaseID, ExtCommitmentID: p.ExtCommitmentID,
		VersionNo: p.VersionNo, Purpose: version.Purpose, RecipientKind: version.RecipientKind,
		Channel:        version.Channel,
		IdempotencyKey: casesDomain.ExternalCommitmentIdempotencyKey(tenantID, p.ExtCommitmentID, p.VersionNo),
		ContentDigest:  casesDomain.ExternalContentDigest(version.ContentText),
	})
	if err != nil {
		return err
	}
	var sourceConnection id.UUID
	if err = tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventExternalCommitmentQueued, casesDomain.OutboxEventExternalCommitmentQueuedV1,
		p.CaseID, sourceConnection, p.CorrelationID, p.Now.UTC().Truncate(time.Microsecond), payload,
		casesDomain.DedupKeyExternalCommitmentQueued(p.ExtCommitmentID, p.VersionNo),
		p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return mapExternalInsertError(err)
	}
	return tx.Commit(ctx)
}

// externalPinReadinessOfTx derives the LIVE readiness word of the pinned
// 2.9 version under the locks: the pinned version must exist tenant-wide,
// its STORED content token must equal the pin token (the live match — the
// stored word is the truth) and the DeriveCommitmentReadiness of the
// exact version resolves the word (offered_confirmable is the only
// passing readiness of the offer pin). A missing or mismatched pin is the
// honest dictionary refusal — never a silent default.
func externalPinReadinessOfTx(ctx context.Context, tx pgx.Tx, tenantID id.UUID, pin casesDomain.ExternalCommitmentPin) (string, error) {
	versions, err := readCommitmentVersionsByIDTx(ctx, tx, tenantID, pin.CommitmentID)
	if err != nil {
		return "", err
	}
	var pinned *casesDomain.CommitmentVersion
	maxVersionNo := int64(0)
	for i := range versions {
		if versions[i].VersionNo == pin.VersionNo {
			pinned = &versions[i]
		}
		if versions[i].VersionNo > maxVersionNo {
			maxVersionNo = versions[i].VersionNo
		}
	}
	if pinned == nil || pinned.ContentToken != pin.ContentToken {
		return "", casesDomain.WorkRuleError{Rule: "enqueue_external_commitment", Reason: "Пин обязательства не совпал с живой строкой 2.9 (нет версии с таким номером либо токен иной): сверьте пин точной версии и повторите."}
	}
	return casesDomain.DeriveCommitmentReadiness(*pinned, maxVersionNo), nil
}

// PendingExternalCommitments returns the drain input: the queued outbox
// records of the tenant whose history allows an action — no
// attempt_submitted (the words already left), no cancelled_stale
// (gassed), the attempt bound not exhausted and the LAST event not a
// guarantee-free unknown (the blind retry is structurally unselectable,
// AC-5). Every candidate carries the consistent material of the fences:
// the immutable version row, the append-only history, the LIVE owner
// epoch of the case, the maximal version number of the envelope and the
// per-tenant commitment key (the token re-check basis) — ONE read
// transaction (the honest snapshot).
func (s *Postgres) PendingExternalCommitments(ctx context.Context, scope tenancy.Context, limit int) ([]casesPort.ExternalDispatchCandidate, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := scope.TenantID()
	rows, err := tx.Query(ctx, `
                select o.tenant_id, o.event_id, o.event_type, o.schema_version, o.aggregate_id, o.correlation_id, o.occurred_at, o.payload, o.dedup_key
                from cases.outbox_events o
                where o.tenant_id=$1 and o.event_type='external_commitment.queued'
                  and not exists (
                        select 1 from cases.external_dispatch_events e
                        where e.tenant_id = o.tenant_id
                          and e.ext_commitment_id = (o.payload->>'ext_commitment_id')::uuid
                          and e.version_no = (o.payload->>'version_no')::bigint
                          and (e.kind in ('attempt_submitted','cancelled_stale')
                               or e.attempt_no >= $3
                               or (e.kind = 'attempt_unknown' and e.idempotent_guarantee = false and e.event_no = (
                                        select max(e2.event_no) from cases.external_dispatch_events e2
                                        where e2.tenant_id = e.tenant_id
                                          and e2.ext_commitment_id = e.ext_commitment_id
                                          and e2.version_no = e.version_no))))
                order by o.occurred_at, o.event_id
                limit $2`, tenantID, limit, casesDomain.MaxDispatchAttempts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []casesDomain.OutboxEvent{}
	for rows.Next() {
		var record casesDomain.OutboxEvent
		if err := rows.Scan(&record.TenantID, &record.EventID, &record.EventType, &record.SchemaVersion,
			&record.AggregateID, &record.CorrelationID, &record.OccurredAt, &record.Payload, &record.DedupKey); err != nil {
			return nil, err
		}
		record.OccurredAt = record.OccurredAt.UTC()
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	candidates := []casesPort.ExternalDispatchCandidate{}
	for _, record := range records {
		var queued casesDomain.ExternalCommitmentQueuedPayload
		if err := json.Unmarshal(record.Payload, &queued); err != nil {
			return nil, err
		}
		envelope, err := readEnvelopeVersionsTx(ctx, tx, tenantID, queued.ExtCommitmentID)
		if err != nil {
			return nil, err
		}
		var version *casesDomain.ExternalCommitment
		maxVersionNo := int64(0)
		for i := range envelope {
			if envelope[i].VersionNo == queued.VersionNo {
				version = &envelope[i]
			}
			if envelope[i].VersionNo > maxVersionNo {
				maxVersionNo = envelope[i].VersionNo
			}
		}
		if version == nil {
			continue // the version row is gone (impossible through the guards) — never a crash of the batch
		}
		events, err := readExternalDispatchEventsOfTx(ctx, tx, tenantID, queued.ExtCommitmentID, queued.VersionNo)
		if err != nil {
			return nil, err
		}
		terms, _, _, err := readOwnershipMaterialTx(ctx, tx, tenantID, version.CaseID)
		if err != nil {
			return nil, err
		}
		var ownerEpoch int64
		if current := casesDomain.CurrentOwnership(terms); current != nil {
			ownerEpoch = current.Epoch
		}
		var key []byte
		if err := tx.QueryRow(ctx, `
                        select key from cases.commitment_hmac_keys where tenant_id=$1`, tenantID).Scan(&key); err != nil {
			return nil, err
		}
		candidates = append(candidates, casesPort.ExternalDispatchCandidate{
			Record: record, Version: *version, Events: events,
			OwnerEpochCurrent: ownerEpoch, MaxVersionNo: maxVersionNo, HMACKey: key,
		})
	}
	return candidates, tx.Commit(ctx)
}

// GasExternalCommitmentQueue appends ONE cancelled_stale event under the
// locks — ONLY when the words provably never left (the history is empty
// or carries attempt_failed only: the pre-send refusals). A raced
// decision (the words left, a receipt arrived, an unknown landed or the
// gasp already happened) writes nothing (false) — the append-only
// history never claims «слова не ушли» after the words left (AC-6).
func (s *Postgres) GasExternalCommitmentQueue(ctx context.Context, p casesPort.ExternalGasParams) (bool, error) {
	if err := casesDomain.GuardExternalStaleReason(p.Reason); err != nil {
		return false, casesDomain.WorkRuleError{Rule: "drain_external_commitments", Reason: err.Error()}
	}
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if err := lockExternalAnchors(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return false, err
	}
	if _, err := externalVersionOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExtCommitmentID, p.VersionNo); err != nil {
		return false, err
	}
	events, err := readExternalDispatchEventsOfTx(ctx, tx, tenantID, p.ExtCommitmentID, p.VersionNo)
	if err != nil {
		return false, err
	}
	for _, event := range events {
		if event.Kind != casesDomain.ExternalDispatchAttemptFailed {
			return false, tx.Commit(ctx) // the words left, were observed or the gasp already landed — nothing to write
		}
	}
	reasonWord := p.Reason
	if err := insertExternalDispatchEvent(ctx, tx, tenantID, p.CaseID, p.ExtCommitmentID, p.VersionNo,
		p.EventID, int64(len(events)+1), casesDomain.ExternalDispatchCancelledStale, &reasonWord, 0, "", false,
		casesDomain.ExternalStaleReasonText(p.Reason), p.ActorID, p.Now); err != nil {
		return false, mapExternalInsertError(err)
	}
	return true, tx.Commit(ctx)
}

// RecordExternalDispatchOutcome appends the attempt outcome events of ONE
// dispatch call under the locks: a raced attempt_submitted (the words
// already left), a landed gasp (the words never left claim stays) or an
// exhausted bound writes nothing (false — the second Staffy intent never
// exists; the events are the first dedup line, the outbox key the
// second, AC-6/AC-8). The attempt number is re-computed under the locks
// — the app's numbers are advisory, the history is the truth.
func (s *Postgres) RecordExternalDispatchOutcome(ctx context.Context, p casesPort.ExternalDispatchOutcomeParams) (bool, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if err := lockExternalAnchors(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return false, err
	}
	if _, err := externalVersionOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExtCommitmentID, p.VersionNo); err != nil {
		return false, err
	}
	events, err := readExternalDispatchEventsOfTx(ctx, tx, tenantID, p.ExtCommitmentID, p.VersionNo)
	if err != nil {
		return false, err
	}
	attemptUsed := int64(0)
	for _, event := range events {
		switch event.Kind {
		case casesDomain.ExternalDispatchAttemptSubmitted, casesDomain.ExternalDispatchCancelledStale:
			return false, tx.Commit(ctx) // the raced twin decided first — nothing new
		case casesDomain.ExternalDispatchAttemptFailed, casesDomain.ExternalDispatchAttemptUnknown:
			if event.AttemptNo > attemptUsed {
				attemptUsed = event.AttemptNo
			}
		}
	}
	attemptNo := attemptUsed + 1
	if attemptNo > casesDomain.MaxDispatchAttempts {
		return false, tx.Commit(ctx) // the honest bound backstop — the words never leave
	}
	eventNo := int64(len(events))
	for _, outcome := range p.Events {
		eventNo++
		attemptOfEvent := int64(0)
		switch outcome.Kind {
		case casesDomain.ExternalDispatchAttemptSubmitted, casesDomain.ExternalDispatchAttemptFailed, casesDomain.ExternalDispatchAttemptUnknown:
			attemptOfEvent = attemptNo
		}
		if err := insertExternalDispatchEvent(ctx, tx, tenantID, p.CaseID, p.ExtCommitmentID, p.VersionNo,
			outcome.EventID, eventNo, outcome.Kind, nil, attemptOfEvent, outcome.ExternalCorrelationID,
			outcome.IdempotentGuarantee, outcome.Details, p.ActorID, p.Now); err != nil {
			return false, mapExternalInsertError(err)
		}
	}
	return true, tx.Commit(ctx)
}

// RelayExternalReceipt appends ONE receipt event of the operator relay
// observation (the 2.12 ConfirmerOperatorRelay precedent): the version
// must exist and BE ENQUEUED (the outbox record with the version pin is
// the queue evidence); the append-only history accepts the observation
// whatever came before — the read-time derivation takes the LAST event.
func (s *Postgres) RelayExternalReceipt(ctx context.Context, p casesPort.ExternalRelayReceiptParams) (bool, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if err := lockExternalAnchors(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return false, err
	}
	if _, err := externalVersionOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExtCommitmentID, p.VersionNo); err != nil {
		return false, err
	}
	var queued int
	err = tx.QueryRow(ctx, `
                select 1 from cases.outbox_events
                where tenant_id=$1 and event_type='external_commitment.queued'
                  and dedup_key=$2`,
		tenantID, casesDomain.DedupKeyExternalCommitmentQueued(p.ExtCommitmentID, p.VersionNo)).Scan(&queued)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, casesDomain.WorkRuleError{Rule: "relay_external_receipt", Reason: "Версия конверта не состоит в очереди на отправку: наблюдение канала касается отправляемой версии — поставьте версию в очередь."}
	}
	if err != nil {
		return false, err
	}
	events, err := readExternalDispatchEventsOfTx(ctx, tx, tenantID, p.ExtCommitmentID, p.VersionNo)
	if err != nil {
		return false, err
	}
	if err := insertExternalDispatchEvent(ctx, tx, tenantID, p.CaseID, p.ExtCommitmentID, p.VersionNo,
		p.EventID, int64(len(events)+1), p.Kind, nil, 0, "", false,
		"наблюдение оператора канала", p.ActorID, p.Now); err != nil {
		return false, mapExternalInsertError(err)
	}
	return true, tx.Commit(ctx)
}

// insertExternalDispatchEvent appends one dispatch history row: the
// monotonic event number is computed under the locks (the raced twin hits
// the UNIQUE backstop — the second intent never exists). The event
// identifiers are minted by the APP before the transaction (the module
// convention — the store stays a pure persistence boundary).
func insertExternalDispatchEvent(ctx context.Context, tx pgx.Tx, tenantID, caseID, extID id.UUID, versionNo int64,
	eventID id.UUID, eventNo int64, kind string, reason *string, attemptNo int64,
	correlationID string, guarantee bool, details string, actorID id.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `
                insert into cases.external_dispatch_events (tenant_id, case_id, ext_commitment_id, version_no,
                        event_id, event_no, kind, reason, attempt_no, external_correlation_id, idempotent_guarantee, details, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		tenantID, caseID, extID, versionNo, eventID, eventNo, kind, reason, attemptNo,
		correlationID, guarantee, casesDomain.TruncateExternalDetails(details), actorID,
		now.UTC().Truncate(time.Microsecond))
	return err
}
