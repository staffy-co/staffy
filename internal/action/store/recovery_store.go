package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 3.8 (step-02): the recovery-case persistence — the recorded fact
// read, the case reads and the ONE creation transaction
// ---------------------------------------------------------------------------
//
// The recovery repository owns NO provider adapters: the fact read and
// the case reads are pure tenant-scoped SELECTs (read-time, no locks, no
// writes — the eligibility ladder NEVER writes on any gas), and the ONE
// creation transaction materializes the case row, the created event and
// the outbox recovery_case.created record in a SINGLE commit under the
// recovery advisory lock (the creation race resolves by the idempotent
// return — the UNIQUE (tenant_id, operation_id) is the structural twin).
// A network call is impossible BY TYPE inside the store — the signature
// itself is the persist-before-emit discipline (the
// AcquireDispatchLease/ReserveOperation precedents).

// The structural pin: the store satisfies the consumer-owned repository
// contract of the recovery contour (the PreviewRepository precedent).
var _ actionPort.RecoveryRepository = (*Postgres)(nil)

// recoveryLockKey is the advisory lock key of one operation's recovery
// creation — its OWN key space (the "recovery:" prefix never mixes with
// the dispatchLockKey/reconcileLockKey ones): the creation race of two
// commands of the SAME operation resolves by the lock, the loser sees
// the winner's rows and answers the idempotent/evidence decision. The
// lock guards ONLY the creation race (the contour holds NO leases: the
// creation is idempotent and never competes with the execution).
func recoveryLockKey(tenantID id.UUID, operationID id.UUID) string {
	return "recovery:" + tenantID.String() + ":" + operationID.String()
}

// mapRecoveryInsertError maps the raced insert conflicts and the CHECK
// backstops onto the honest consumer-owned signals: a unique violation
// of the case tables (a raced twin won under the lock — structurally
// impossible through the advisory discipline, the loyal backstop
// nonetheless) and a refused CHECK word both read the honest not-found
// refusal — the store backstop never masks a lost race as a write.
func mapRecoveryInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		return actionPort.ErrRecoveryNotFound
	}
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23514" {
		return actionPort.ErrRecoveryNotFound
	}
	return err
}

// recoveryCaseColumns is the canonical column list of the case row (the
// SELECT and the INSERT share it — the scan target list mirrors it).
const recoveryCaseColumns = `tenant_id, recovery_case_id, recovery_case_no, recovery_key, operation_id, trigger_word,
 case_id, work_item_id, definition_id, definition_version,
 preview_id, preview_version_no, confirmation_id, confirmation_version_no,
 auth_owner_kind, auth_owner_ref, auth_owner_epoch, release_decision_no, profile_version,
 business_intent_key, idempotency_key, material_digest,
 dispatch_attempts, reconcile_attempts,
 receipt_versions_count, latest_receipt_version_no, latest_receipt_outcome,
 observations, target_kind, target_ref, owning_system,
 external_object_id, external_object_version, external_object_time,
 assign_owner_kind, assign_owner_ref, assign_owner_epoch, backup_owner_id, deadline_at,
 bundle_digest, created_by, created_at`

// scanRecoveryCase scans one case row (the shared target list of every
// case read) into the port row shape: the bundle re-hydrates from the
// SAFE columns, the observations pins unmarshal from the bounded JSON,
// the assignment re-assembles from its columns and the events ride a
// SEPARATE read (the append-only history of the case).
func scanRecoveryCase(row pgx.Row, tenantID id.UUID) (actionPort.RecoveryCaseRow, error) {
	var c actionPort.RecoveryCaseRow
	var rowTenantID id.UUID
	var observations []byte
	var externalTime *time.Time
	err := row.Scan(&rowTenantID, &c.RecoveryCaseID, &c.RecoveryCaseNo, &c.RecoveryKey, &c.OperationID, &c.Trigger,
		&c.Bundle.CaseID, &c.Bundle.WorkItemID, &c.Bundle.DefinitionID, &c.Bundle.DefinitionVer,
		&c.Bundle.PreviewID, &c.Bundle.PreviewVersionNo, &c.Bundle.ConfirmationID, &c.Bundle.ConfirmationVersionNo,
		&c.Bundle.OwnerKind, &c.Bundle.OwnerRef, &c.Bundle.OwnerEpoch, &c.Bundle.ReleaseDecisionNo, &c.Bundle.ProfileVersion,
		&c.Bundle.BusinessIntentKey, &c.Bundle.IdempotencyKey, &c.Bundle.MaterialDigest,
		&c.Bundle.DispatchAttempts, &c.Bundle.ReconcileAttempts,
		&c.Bundle.ReceiptVersionsCount, &c.Bundle.LatestReceiptVersion, &c.Bundle.LatestReceiptOutcome,
		&observations, &c.Bundle.TargetKind, &c.Bundle.TargetRef, &c.Bundle.OwningSystem,
		&c.Bundle.ExternalObjectID, &c.Bundle.ExternalObjectVersion, &externalTime,
		&c.Assignment.OwnerKind, &c.Assignment.OwnerRef, &c.Assignment.OwnerEpoch, &c.Assignment.BackupOwnerID, &c.DeadlineAt,
		&c.BundleDigest, &c.CreatedBy, &c.CreatedAt)
	_ = tenantID
	_ = rowTenantID
	if err != nil {
		return actionPort.RecoveryCaseRow{}, err
	}
	if externalTime != nil {
		c.Bundle.ExternalObjectTime = externalTime.UTC().Format(time.RFC3339)
	}
	if err := json.Unmarshal(observations, &c.Bundle.Observations); err != nil {
		return actionPort.RecoveryCaseRow{}, err
	}
	if c.Bundle.Observations == nil {
		c.Bundle.Observations = []actionDomain.VersionPin{}
	}
	c.Assignment.AckDeadlineMinutes = 0 // the materialized deadline_at is the row's truth
	c.DeadlineAt = c.DeadlineAt.UTC().Truncate(time.Microsecond)
	c.CreatedAt = c.CreatedAt.UTC().Truncate(time.Microsecond)
	c.Events = []actionDomain.RecoveryEventRow{}
	return c, nil
}

// readRecoveryEvents reads the append-only event history of one case
// ordered by the monotonic event number (the shared read of every case
// view — the history is never rewritten).
func readRecoveryEvents(ctx context.Context, tx pgx.Tx, tenantID id.UUID, recoveryCaseID id.UUID) ([]actionDomain.RecoveryEventRow, error) {
	rows, err := tx.Query(ctx, `
                select event_no, kind, details, actor_id, created_at
                from action.recovery_events
                where tenant_id=$1 and recovery_case_id=$2
                order by event_no`, tenantID, recoveryCaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []actionDomain.RecoveryEventRow{}
	for rows.Next() {
		var e actionDomain.RecoveryEventRow
		if err := rows.Scan(&e.EventNo, &e.Kind, &e.Details, &e.ActorID, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.CreatedAt = e.CreatedAt.UTC().Truncate(time.Microsecond)
		events = append(events, e)
	}
	return events, rows.Err()
}

// ReadRecoveryOperationFacts reads the RECORDED facts of one operation —
// the eligibility ladder and the bundle build input: the immutable
// Operation row, the append-only ledger (the state derives), the attempt
// counters of both contours, the recorded guarantee of the last dispatch
// attempt, the no_effect unlock fact (the LAST reconciliation verdict),
// the receipt-chain pins with the external identity triple of the latest
// receipt and the supporting-observations pins of the EXACT preview
// version. Read-time: NO writes, NO locks, NO adapters (the RLS
// blindness of a foreign tenant answers ErrRecoveryNotFound — the
// membership 404 BEFORE any material).
func (s *Postgres) ReadRecoveryOperationFacts(ctx context.Context, scope tenancy.Context, operationID id.UUID) (actionPort.RecoveryOperationFacts, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return actionPort.RecoveryOperationFacts{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := scope.TenantID()
	facts := actionPort.RecoveryOperationFacts{}
	var effects, ledger, observations []byte
	var lastVerdict *string
	var latestVersion *int64
	var latestOutcome, externalID, externalVersion *string
	var externalTime *time.Time
	err = tx.QueryRow(ctx, `
                select o.operation_id, o.definition_id, o.definition_version,
                       o.preview_id, o.preview_version_no, o.confirmation_id, o.confirmation_version_no,
                       o.case_id, o.work_item_id, o.target_kind, o.target_ref,
                       o.owner_kind, o.owner_ref, o.owner_epoch, o.release_decision_no, o.profile_version,
                       o.partition, o.effects, o.business_intent_key, o.idempotency_key, o.material_digest, o.created_at,
                       lg.ledger,
                       (select count(*) from action.dispatch_leases dl where dl.tenant_id=o.tenant_id and dl.operation_id=o.operation_id),
                       (select coalesce((select guarantee_recorded from action.dispatch_outcomes uo
                                         where uo.tenant_id=o.tenant_id and uo.operation_id=o.operation_id
                                         order by uo.attempt_no desc limit 1), false)),
                       (select count(*) from action.reconciliation_leases rc where rc.tenant_id=o.tenant_id and rc.operation_id=o.operation_id),
                       (select verdict from action.reconciliation_outcomes ro
                         where ro.tenant_id=o.tenant_id and ro.operation_id=o.operation_id
                         order by ro.attempt_no desc limit 1),
                       (select count(*) from action.action_receipts ar where ar.tenant_id=o.tenant_id and ar.operation_id=o.operation_id),
                       (select receipt_version_no from action.action_receipts ar
                         where ar.tenant_id=o.tenant_id and ar.operation_id=o.operation_id
                         order by ar.receipt_version_no desc limit 1),
                       (select outcome from action.action_receipts ar
                         where ar.tenant_id=o.tenant_id and ar.operation_id=o.operation_id
                         order by ar.receipt_version_no desc limit 1),
                       (select receipt_references->>'external_object_id' from action.action_receipts ar
                         where ar.tenant_id=o.tenant_id and ar.operation_id=o.operation_id
                         order by ar.receipt_version_no desc limit 1),
                       (select receipt_references->>'external_object_version' from action.action_receipts ar
                         where ar.tenant_id=o.tenant_id and ar.operation_id=o.operation_id
                         order by ar.receipt_version_no desc limit 1),
                       (select external_object_time from action.dispatch_outcomes uo
                         where uo.tenant_id=o.tenant_id and uo.operation_id=o.operation_id and uo.external_object_time is not null
                         order by uo.attempt_no desc limit 1),
                       pv.supporting_observations
                from action.operations o
                left join lateral (
                        select json_agg(x order by x.entry_no) as ledger
                        from (select entry_no, kind, details, created_at from action.operation_ledger le
                              where le.tenant_id=o.tenant_id and le.operation_id=o.operation_id) x
                ) lg on true
                left join lateral (
                        select supporting_observations from action.action_previews ap
                        where ap.tenant_id=o.tenant_id and ap.preview_id=o.preview_id and ap.version_no=o.preview_version_no
                ) pv on true
                where o.tenant_id=$1 and o.operation_id=$2`,
		tenantID, operationID).Scan(
		&facts.Operation.OperationID, &facts.Operation.DefinitionID, &facts.Operation.DefinitionVersion,
		&facts.Operation.PreviewID, &facts.Operation.PreviewVersionNo, &facts.Operation.ConfirmationID, &facts.Operation.ConfirmationVersionNo,
		&facts.Operation.CaseID, &facts.Operation.WorkItemID, &facts.Operation.Target.Kind, &facts.Operation.Target.Ref,
		&facts.Operation.OwnerKind, &facts.Operation.OwnerRef, &facts.Operation.OwnerEpoch, &facts.Operation.ReleaseDecisionNo, &facts.Operation.ProfileVersion,
		&facts.Operation.Partition, &effects, &facts.Operation.BusinessIntentKey, &facts.Operation.IdempotencyKey, &facts.Operation.MaterialDigest, &facts.Operation.CreatedAt,
		&ledger, &facts.DispatchAttemptCount, &facts.LastOutcomeGuarantee, &facts.ReconcileAttemptCount, &lastVerdict,
		&facts.ReceiptVersionsCount, &latestVersion, &latestOutcome, &externalID, &externalVersion, &externalTime,
		&observations)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actionPort.RecoveryOperationFacts{}, actionPort.ErrRecoveryNotFound
		}
		return actionPort.RecoveryOperationFacts{}, err
	}
	facts.Operation.TenantID = tenantID
	if err := json.Unmarshal(effects, &facts.Operation.Effects); err != nil {
		return actionPort.RecoveryOperationFacts{}, err
	}
	facts.Operation.CreatedAt = facts.Operation.CreatedAt.UTC()
	if ledger != nil {
		if err := json.Unmarshal(ledger, &facts.Ledger); err != nil {
			return actionPort.RecoveryOperationFacts{}, err
		}
	} else {
		facts.Ledger = []actionDomain.OperationLedgerEntry{}
	}
	state, err := actionDomain.DeriveOperationState(facts.Ledger)
	if err != nil {
		return actionPort.RecoveryOperationFacts{}, err
	}
	facts.StateWord = state
	if lastVerdict != nil && *lastVerdict == actionDomain.ReconciliationVerdictNoEffect {
		facts.NoEffectProven = true
	}
	if latestVersion != nil {
		facts.LatestReceiptVersionNo = *latestVersion
	}
	if latestOutcome != nil {
		facts.LatestReceiptOutcome = *latestOutcome
	}
	if externalID != nil {
		facts.LatestExternalObjectID = *externalID
	}
	if externalVersion != nil {
		facts.LatestExternalObjectVersion = *externalVersion
	}
	if externalTime != nil {
		facts.LatestExternalObjectTime = externalTime.UTC().Truncate(time.Microsecond)
	}
	if observations != nil {
		if err := json.Unmarshal(observations, &facts.Observations); err != nil {
			return actionPort.RecoveryOperationFacts{}, err
		}
	}
	if facts.Observations == nil {
		facts.Observations = []actionDomain.VersionPin{}
	}
	return facts, tx.Commit(ctx)
}

// ReadRecoveryCaseByOperation reads the case of one operation with its
// event history (the idempotent pre-check of the command — the creation
// transaction re-reads UNDER the lock, the belt over braces).
func (s *Postgres) ReadRecoveryCaseByOperation(ctx context.Context, scope tenancy.Context, operationID id.UUID) (actionPort.RecoveryCaseRow, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return actionPort.RecoveryCaseRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := scope.TenantID()
	row := tx.QueryRow(ctx, `select `+recoveryCaseColumns+`
                from action.recovery_cases where tenant_id=$1 and operation_id=$2`, tenantID, operationID)
	c, err := scanRecoveryCase(row, tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actionPort.RecoveryCaseRow{}, actionPort.ErrRecoveryNotFound
		}
		return actionPort.RecoveryCaseRow{}, err
	}
	if c.Events, err = readRecoveryEvents(ctx, tx, tenantID, c.RecoveryCaseID); err != nil {
		return actionPort.RecoveryCaseRow{}, err
	}
	return c, tx.Commit(ctx)
}

// ReadRecoveryCase reads one case by its identity with its event history
// (the card view material).
func (s *Postgres) ReadRecoveryCase(ctx context.Context, scope tenancy.Context, recoveryCaseID id.UUID) (actionPort.RecoveryCaseRow, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return actionPort.RecoveryCaseRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := scope.TenantID()
	row := tx.QueryRow(ctx, `select `+recoveryCaseColumns+`
                from action.recovery_cases where tenant_id=$1 and recovery_case_id=$2`, tenantID, recoveryCaseID)
	c, err := scanRecoveryCase(row, tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actionPort.RecoveryCaseRow{}, actionPort.ErrRecoveryNotFound
		}
		return actionPort.RecoveryCaseRow{}, err
	}
	if c.Events, err = readRecoveryEvents(ctx, tx, tenantID, c.RecoveryCaseID); err != nil {
		return actionPort.RecoveryCaseRow{}, err
	}
	return c, tx.Commit(ctx)
}

// ReadRecoveryCases reads the cases of the tenant ordered by the creation
// moment descending, bounded by the limit (the panel list — the bounded
// read discipline of the drain limits).
func (s *Postgres) ReadRecoveryCases(ctx context.Context, scope tenancy.Context, limit int) ([]actionPort.RecoveryCaseRow, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if limit < 1 {
		limit = actionPort.RecoveryPanelLimit
	}
	tenantID := scope.TenantID()
	rows, err := tx.Query(ctx, `select `+recoveryCaseColumns+`
                from action.recovery_cases where tenant_id=$1
                order by created_at desc, recovery_case_no desc limit $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cases := []actionPort.RecoveryCaseRow{}
	for rows.Next() {
		c, err := scanRecoveryCase(rows, tenantID)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range cases {
		if cases[i].Events, err = readRecoveryEvents(ctx, tx, tenantID, cases[i].RecoveryCaseID); err != nil {
			return nil, err
		}
	}
	return cases, tx.Commit(ctx)
}

// insertRecoveryCase is the row INSERT of the creation path: the SAFE
// bundle columns, the assignment columns, the keyed recovery key and the
// frozen bundle digest — ONE row of the frozen creation moment.
func insertRecoveryCase(ctx context.Context, tx pgx.Tx, p actionPort.CreateRecoveryCaseParams, caseNo int64) error {
	var externalTime any
	if p.Bundle.ExternalObjectTime != "" {
		if parsed, err := time.Parse(time.RFC3339, p.Bundle.ExternalObjectTime); err == nil {
			externalTime = parsed.UTC().Truncate(time.Microsecond)
		}
	}
	observations, err := marshalJSON(p.Bundle.Observations)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
                insert into action.recovery_cases (tenant_id, recovery_case_id, recovery_case_no, recovery_key, operation_id, trigger_word,
                        case_id, work_item_id, definition_id, definition_version,
                        preview_id, preview_version_no, confirmation_id, confirmation_version_no,
                        auth_owner_kind, auth_owner_ref, auth_owner_epoch, release_decision_no, profile_version,
                        business_intent_key, idempotency_key, material_digest,
                        dispatch_attempts, reconcile_attempts,
                        receipt_versions_count, latest_receipt_version_no, latest_receipt_outcome,
                        observations, target_kind, target_ref, owning_system,
                        external_object_id, external_object_version, external_object_time,
                        assign_owner_kind, assign_owner_ref, assign_owner_epoch, backup_owner_id, deadline_at,
                        bundle_digest, created_by, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37,$38,$39,$40,$41,$42)`,
		p.Scope.TenantID(), p.RecoveryCaseID, caseNo, p.RecoveryKey, p.Operation.OperationID, p.Trigger,
		p.Bundle.CaseID, p.Bundle.WorkItemID, p.Bundle.DefinitionID, p.Bundle.DefinitionVer,
		p.Bundle.PreviewID, p.Bundle.PreviewVersionNo, p.Bundle.ConfirmationID, p.Bundle.ConfirmationVersionNo,
		p.Bundle.OwnerKind, p.Bundle.OwnerRef, p.Bundle.OwnerEpoch, p.Bundle.ReleaseDecisionNo, p.Bundle.ProfileVersion,
		p.Bundle.BusinessIntentKey, p.Bundle.IdempotencyKey, p.Bundle.MaterialDigest,
		p.Bundle.DispatchAttempts, p.Bundle.ReconcileAttempts,
		p.Bundle.ReceiptVersionsCount, p.Bundle.LatestReceiptVersion, p.Bundle.LatestReceiptOutcome,
		observations, p.Bundle.TargetKind, p.Bundle.TargetRef, p.Bundle.OwningSystem,
		p.Bundle.ExternalObjectID, p.Bundle.ExternalObjectVersion, externalTime,
		p.Assignment.OwnerKind, p.Assignment.OwnerRef, p.Assignment.OwnerEpoch, p.Assignment.BackupOwnerID,
		p.DeadlineAt.UTC().Truncate(time.Microsecond),
		p.BundleDigest, p.ActorID, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return mapRecoveryInsertError(err)
	}
	return nil
}

// CreateRecoveryCase is the ONE creation transaction of the contour
// under the recovery advisory lock (tenant, operation_id): the existing
// case re-read decides the path — the UNCHANGED digest answers the
// IDEMPOTENT return (NO second row, NO event — the noise-free repeat),
// a CHANGED digest appends ONE evidence_appended row with the SAFE delta
// (the original bundle and the original receipts are NEVER rewritten),
// no case CREATES the row (the monotonic recovery_case_no
// coalesce(max)+1 UNDER the lock — the caseNo precedent of the cases
// store), the created event and the outbox recovery_case.created record.
// ANY failure rolls back everything (ONE commit, AD-6
// persist-before-emit; the signature carries ZERO adapters BY TYPE).
func (s *Postgres) CreateRecoveryCase(ctx context.Context, p actionPort.CreateRecoveryCaseParams) (actionPort.CreateRecoveryCaseResult, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return actionPort.CreateRecoveryCaseResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		recoveryLockKey(tenantID, p.Operation.OperationID)); err != nil {
		return actionPort.CreateRecoveryCaseResult{}, err
	}
	// The existing case re-read UNDER the lock (the creation race of two
	// commands resolves here: the loser sees the winner's rows).
	row := tx.QueryRow(ctx, `select `+recoveryCaseColumns+`
                from action.recovery_cases where tenant_id=$1 and operation_id=$2`,
		tenantID, p.Operation.OperationID)
	existing, err := scanRecoveryCase(row, tenantID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return actionPort.CreateRecoveryCaseResult{}, err
	}
	if err == nil {
		// The repeat comparator UNDER the lock (the 3-6/3-7 discipline:
		// every path with a recorded trace compares BEFORE any write).
		decision, cmpErr := actionDomain.DeriveRecoveryRepeatDecision(existing.BundleDigest, p.BundleDigest)
		if cmpErr != nil {
			return actionPort.CreateRecoveryCaseResult{}, cmpErr
		}
		if decision == actionDomain.RecoveryRepeatIdempotent {
			if existing.Events, err = readRecoveryEvents(ctx, tx, tenantID, existing.RecoveryCaseID); err != nil {
				return actionPort.CreateRecoveryCaseResult{}, err
			}
			return actionPort.CreateRecoveryCaseResult{Case: existing, Decision: actionPort.RecoveryDecisionIdempotent}, tx.Commit(ctx)
		}
		// The evidence append: ONE row with the SAFE delta, the monotonic
		// event number UNDER the case history (the original bundle row is
		// never rewritten — the append-only reference is the whole update).
		var eventNo int64
		if err := tx.QueryRow(ctx, `
                        select coalesce(max(event_no), 0) + 1 from action.recovery_events
                        where tenant_id=$1 and recovery_case_id=$2`,
			tenantID, existing.RecoveryCaseID).Scan(&eventNo); err != nil {
			return actionPort.CreateRecoveryCaseResult{}, err
		}
		delta := actionDomain.DeriveRecoveryEvidenceDelta(existing.Bundle, p.Bundle)
		if _, err := tx.Exec(ctx, `
                        insert into action.recovery_events (tenant_id, recovery_case_id, event_no, kind, details, actor_id, created_at)
                        values ($1,$2,$3,$4,$5,$6,$7)`,
			tenantID, existing.RecoveryCaseID, eventNo, actionDomain.RecoveryEventEvidenceAppended,
			delta, p.ActorID, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
			return actionPort.CreateRecoveryCaseResult{}, mapRecoveryInsertError(err)
		}
		if existing.Events, err = readRecoveryEvents(ctx, tx, tenantID, existing.RecoveryCaseID); err != nil {
			return actionPort.CreateRecoveryCaseResult{}, err
		}
		return actionPort.CreateRecoveryCaseResult{Case: existing, Decision: actionPort.RecoveryDecisionEvidence}, tx.Commit(ctx)
	}
	// The creation path: the monotonic case number UNDER the lock (the
	// caseNo precedent — coalesce(max)+1 per tenant), the case row, the
	// created event and the outbox recovery_case.created record — ONE
	// commit.
	var caseNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(recovery_case_no), 0) + 1 from action.recovery_cases
                where tenant_id=$1`, tenantID).Scan(&caseNo); err != nil {
		return actionPort.CreateRecoveryCaseResult{}, err
	}
	if err := insertRecoveryCase(ctx, tx, p, caseNo); err != nil {
		return actionPort.CreateRecoveryCaseResult{}, err
	}
	if _, err := tx.Exec(ctx, `
                insert into action.recovery_events (tenant_id, recovery_case_id, event_no, kind, details, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, p.RecoveryCaseID, int64(1), actionDomain.RecoveryEventCreated,
		fmt.Sprintf("Recovery Case создан: триггер %s.", p.Trigger),
		p.ActorID, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return actionPort.CreateRecoveryCaseResult{}, mapRecoveryInsertError(err)
	}
	payload, err := marshalJSON(actionPort.RecoveryCaseCreatedPayload{
		EventType:      actionDomain.OutboxEventRecoveryCaseCreated,
		TenantID:       tenantID,
		RecoveryCaseID: p.RecoveryCaseID,
		RecoveryCaseNo: caseNo,
		OperationID:    p.Operation.OperationID,
		CaseID:         p.Bundle.CaseID,
		WorkItemID:     p.Bundle.WorkItemID,
		DefinitionID:   p.Bundle.DefinitionID,
		Trigger:        p.Trigger,
		OwnerKind:      p.Assignment.OwnerKind,
		OwnerRef:       p.Assignment.OwnerRef,
		BackupOwnerID:  p.Assignment.BackupOwnerID,
		DeadlineAt:     p.DeadlineAt.UTC().Truncate(time.Microsecond),
		CreatedAt:      p.Now.UTC().Truncate(time.Microsecond),
	})
	if err != nil {
		return actionPort.CreateRecoveryCaseResult{}, err
	}
	if _, err := tx.Exec(ctx, `
                insert into action.outbox_events (tenant_id, event_id, event_type, case_id, operation_id, payload, created_at)
                values ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, p.OutboxEventID, actionDomain.OutboxEventRecoveryCaseCreated,
		p.Bundle.CaseID, p.Operation.OperationID, payload, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return actionPort.CreateRecoveryCaseResult{}, mapRecoveryInsertError(err)
	}
	created := actionPort.RecoveryCaseRow{
		RecoveryCaseID: p.RecoveryCaseID,
		RecoveryCaseNo: caseNo,
		RecoveryKey:    p.RecoveryKey,
		OperationID:    p.Operation.OperationID,
		Trigger:        p.Trigger,
		Bundle:         p.Bundle,
		BundleDigest:   p.BundleDigest,
		Assignment:     p.Assignment,
		DeadlineAt:     p.DeadlineAt.UTC().Truncate(time.Microsecond),
		CreatedBy:      p.ActorID,
		CreatedAt:      p.Now.UTC().Truncate(time.Microsecond),
		Events: []actionDomain.RecoveryEventRow{{
			EventNo:   1,
			Kind:      actionDomain.RecoveryEventCreated,
			Details:   fmt.Sprintf("Recovery Case создан: триггер %s.", p.Trigger),
			ActorID:   p.ActorID,
			CreatedAt: p.Now.UTC().Truncate(time.Microsecond),
		}},
	}
	return actionPort.CreateRecoveryCaseResult{Case: created, Decision: actionPort.RecoveryDecisionCreated}, tx.Commit(ctx)
}
