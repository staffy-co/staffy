package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 3.7 (step-02): the reconciliation persistence — the write-ahead
// query lease, the fenced verdict and the pending classification
// ---------------------------------------------------------------------------
//
// The TWO transactions of the reconciliation contour, each with ZERO
// adapters in the signature (a network call is impossible BY TYPE — the
// persist-before-emit discipline of the AcquireDispatchLease precedent):
// AcquireReconciliationLease materializes the RIGHT and the MOMENT of the
// query BEFORE the owning-system call (the lease row plus the ledger
// reconciling entry — NOT state-bearing — in ONE commit under the
// reconciliation advisory lock); RecordReconciliationVerdict writes the
// normalized verdict fenced by the (attempt_no, lease_token) pair — a
// stale holder NEVER writes history — with the terminal writes of the
// verdict (the ledger word of committed/no_effect, the receipt v2+ and
// the outbox receipt.created record) guarded by the receipt TERMINAL LOCK
// inside the SAME commit. The pending classification is the ONE
// reconciler of decisions: every pending target is considered, not every
// target is queried.

// The structural pin: the store satisfies the consumer-owned repository
// contract of the reconciliation contour (the PreviewRepository precedent).
var _ actionPort.ReconciliationRepository = (*Postgres)(nil)

// reconcileLockKey is the advisory lock key of one operation's
// reconciliation contour — its OWN key space (the "reconcile:" prefix
// never mixes with the dispatchLockKey one): every transaction of the
// contour takes it, the raced twin waits, then sees the winner's rows and
// answers the honest signal.
func reconcileLockKey(tenantID id.UUID, operationID id.UUID) string {
	return "reconcile:" + tenantID.String() + ":" + operationID.String()
}

// boundReconcileDetails bounds the safe details to the store CHECK limit
// (the loyal backstop of the domain truncation — the app bounds too, the
// store never trusts the caller alone).
func boundReconcileDetails(text string) string {
	runes := []rune(text)
	if len(runes) > 500 {
		return string(runes[:500])
	}
	return text
}

// mapReconcileInsertError maps the raced insert conflicts and the CHECK
// backstops onto the honest consumer-owned signals of the reconciliation
// fencing dictionary: a unique violation of the contour tables (a raced
// twin won the attempt, the receipt version or the ledger entry) and a
// refused CHECK word both read the honest stale refusal — the store
// backstop never masks a lost race as a write.
func mapReconcileInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "reconciliation_leases_pkey":
			return actionPort.ErrReconciliationLeaseStale
		case "reconciliation_outcomes_pkey":
			return actionPort.ErrReconciliationLeaseStale
		case "action_receipts_pkey":
			return actionPort.ErrReconciliationLeaseStale
		case "operation_ledger_pkey":
			return actionPort.ErrReconciliationLeaseStale
		}
		return err
	}
	// The CHECK backstop of the loyal dictionaries (the attempt bound, the
	// verdict word, the 500-rune details — a refused word is the honest
	// stale refusal, never a masked insert).
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23514" {
		return actionPort.ErrReconciliationLeaseStale
	}
	return err
}

// classifyPendingReconciliationTarget derives the STRUCTURAL
// classification word of one pending target from the derived ledger
// state, the last receipt outcome and the reconciliation attempt facts
// (the state-driven form — the ledger and the receipts are the single
// truth). decided — the operation is no longer unknown-reconcilable (the
// state left unknown OR the last receipt is no longer unknown: the
// at-least-once repeat of a decided operation is the honest skip);
// racing — an active un-expired lease of another consideration owns the
// right (the maximum ONE active chain); exhausted — the three
// reconciliation attempts are spent (the honest human word, never a
// success by timer); waiting — the backoff window of the last attempt is
// active (a zero next_eligible_at means no lease ever — not waiting);
// fresh — the query may run.
func classifyPendingReconciliationTarget(state string, lastReceiptOutcome string,
	attemptCount int, lastLeaseActive bool, nextEligibleAt time.Time, now time.Time) string {
	if state != actionDomain.OperationUnknown || lastReceiptOutcome != actionDomain.ReceiptOutcomeUnknown {
		return actionPort.ReconcileClassDecided
	}
	if lastLeaseActive {
		return actionPort.ReconcileClassRacing
	}
	if attemptCount >= actionDomain.MaxReconciliationAttempts {
		return actionPort.ReconcileClassExhausted
	}
	if !nextEligibleAt.IsZero() && nextEligibleAt.After(now) {
		return actionPort.ReconcileClassWaiting
	}
	return actionPort.ReconcileClassFresh
}

// PendingReconciliationTargets reads the pending reconciliation targets
// of the tenant — the operations with an unknown receipt (the
// at-least-once anchor: a receipt version with the outcome unknown
// exists) joined with the immutable operation, the append-only ledger
// (the state derives), the last receipt facts (the terminal-lock input),
// the dispatch facts the query needs (the FIRST lease digest/key pair,
// the last dispatch correlation) and the reconciliation attempt facts
// (the lease count, the latest lease window, the backoff moment, the
// recorded verdict). The batch is bounded by the reconcile drain limit —
// the rest stay honestly for the next run.
func (s *Postgres) PendingReconciliationTargets(ctx context.Context, scope tenancy.Context, now time.Time, limit int) ([]actionPort.PendingReconciliationTarget, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if limit < 1 {
		limit = actionDomain.ReconcileDrainLimit
	}
	rows, err := tx.Query(ctx, `
		select o.operation_id, o.definition_id, o.definition_version,
		       o.preview_id, o.preview_version_no, o.confirmation_id, o.confirmation_version_no,
		       o.case_id, o.work_item_id, o.target_kind, o.target_ref,
		       o.owner_kind, o.owner_ref, o.owner_epoch, o.release_decision_no, o.profile_version,
		       o.partition, o.effects, o.business_intent_key, o.idempotency_key, o.material_digest, o.created_at,
		       lg.ledger,
		       (select count(*) from action.reconciliation_leases rc where rc.tenant_id=o.tenant_id and rc.operation_id=o.operation_id),
		       rl.attempt_no, rl.lease_token, rl.lease_expires_at, rl.next_eligible_at,
		       (select request_digest from action.dispatch_leases ff where ff.tenant_id=o.tenant_id and ff.operation_id=o.operation_id order by ff.attempt_no asc limit 1),
		       (select external_idempotency_key from action.dispatch_leases ff where ff.tenant_id=o.tenant_id and ff.operation_id=o.operation_id order by ff.attempt_no asc limit 1),
		       (select correlation_id from action.dispatch_outcomes uo where uo.tenant_id=o.tenant_id and uo.operation_id=o.operation_id order by uo.attempt_no desc limit 1),
		       (select verdict from action.reconciliation_outcomes ro where ro.tenant_id=o.tenant_id and ro.operation_id=o.operation_id order by ro.attempt_no desc limit 1),
		       ar.receipt_version_no, ar.outcome
		from action.operations o
		left join lateral (
			select json_agg(x order by x.entry_no) as ledger
			from (select entry_no, kind, details, created_at from action.operation_ledger le
			      where le.tenant_id=o.tenant_id and le.operation_id=o.operation_id) x
		) lg on true
		left join lateral (
			select attempt_no, lease_token, lease_expires_at, next_eligible_at
			from action.reconciliation_leases l
			where l.tenant_id=o.tenant_id and l.operation_id=o.operation_id
			order by l.attempt_no desc limit 1
		) rl on true
		left join lateral (
			select receipt_version_no, outcome
			from action.action_receipts rr
			where rr.tenant_id=o.tenant_id and rr.operation_id=o.operation_id
			order by rr.receipt_version_no desc limit 1
		) ar on true
		where o.tenant_id=$1
		  and exists (select 1 from action.action_receipts r where r.tenant_id=o.tenant_id and r.operation_id=o.operation_id and r.outcome='unknown')
		order by o.created_at
		limit $2`, scope.TenantID(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []actionPort.PendingReconciliationTarget{}
	for rows.Next() {
		target := actionPort.PendingReconciliationTarget{
			LastLeaseExpiresAt: time.Time{},
			NextEligibleAt:     time.Time{},
		}
		var effects []byte
		var ledger []byte
		var attemptCount int
		var lastAttemptNo *int
		var lastLeaseToken *string
		var lastLeaseExpiresAt *time.Time
		var nextEligibleAt *time.Time
		var requestDigest *string
		var externalKey *string
		var lastDispatchCorrelation *string
		var lastReconciliationVerdict *string
		var lastReceiptVersionNo *int64
		var lastReceiptOutcome *string
		if err := rows.Scan(&target.Operation.OperationID, &target.Operation.DefinitionID, &target.Operation.DefinitionVersion,
			&target.Operation.PreviewID, &target.Operation.PreviewVersionNo, &target.Operation.ConfirmationID, &target.Operation.ConfirmationVersionNo,
			&target.Operation.CaseID, &target.Operation.WorkItemID, &target.Operation.Target.Kind, &target.Operation.Target.Ref,
			&target.Operation.OwnerKind, &target.Operation.OwnerRef, &target.Operation.OwnerEpoch, &target.Operation.ReleaseDecisionNo, &target.Operation.ProfileVersion,
			&target.Operation.Partition, &effects, &target.Operation.BusinessIntentKey, &target.Operation.IdempotencyKey, &target.Operation.MaterialDigest, &target.Operation.CreatedAt,
			&ledger, &attemptCount,
			&lastAttemptNo, &lastLeaseToken, &lastLeaseExpiresAt, &nextEligibleAt,
			&requestDigest, &externalKey,
			&lastDispatchCorrelation, &lastReconciliationVerdict,
			&lastReceiptVersionNo, &lastReceiptOutcome); err != nil {
			return nil, err
		}
		target.Operation.TenantID = scope.TenantID()
		if err := json.Unmarshal(effects, &target.Operation.Effects); err != nil {
			return nil, err
		}
		target.Operation.CreatedAt = target.Operation.CreatedAt.UTC()
		if ledger != nil {
			if err := json.Unmarshal(ledger, &target.Ledger); err != nil {
				return nil, err
			}
		} else {
			target.Ledger = []actionDomain.OperationLedgerEntry{}
		}
		target.ReconcileAttemptCount = attemptCount
		if lastAttemptNo != nil {
			target.LastReconcileAttemptNo = *lastAttemptNo
		}
		if lastLeaseToken != nil {
			target.LastLeaseToken = *lastLeaseToken
		}
		if lastLeaseExpiresAt != nil {
			target.LastLeaseExpiresAt = (*lastLeaseExpiresAt).UTC()
			target.LastLeaseActive = target.LastLeaseExpiresAt.After(now)
		}
		if nextEligibleAt != nil {
			target.NextEligibleAt = (*nextEligibleAt).UTC()
		}
		if requestDigest != nil {
			target.RequestDigest = *requestDigest
		}
		if externalKey != nil {
			target.ExternalIDempotencyKey = *externalKey
		}
		if lastDispatchCorrelation != nil {
			target.LastDispatchCorrelation = *lastDispatchCorrelation
		}
		if lastReconciliationVerdict != nil {
			target.LastReconciliationVerdict = *lastReconciliationVerdict
		}
		if lastReceiptVersionNo != nil {
			target.LastReceiptVersionNo = *lastReceiptVersionNo
		}
		if lastReceiptOutcome != nil {
			target.LastReceiptOutcome = *lastReceiptOutcome
		}
		state, err := actionDomain.DeriveOperationState(target.Ledger)
		if err != nil {
			return nil, err
		}
		target.StateWord = state
		target.Classification = classifyPendingReconciliationTarget(state, target.LastReceiptOutcome,
			attemptCount, target.LastLeaseActive, target.NextEligibleAt, now)
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return targets, tx.Commit(ctx)
}

// AcquireReconciliationLease is the WRITE-AHEAD transaction of the query
// right (persist-before-emit — the read contour mirrors the send contour):
// under the reconciliation advisory lock the monotonic attempt_no derives
// (the latest lease + 1, bounded by MaxReconciliationAttempts — the store
// CHECK is the loyal backstop), then the lease row and the ledger
// reconciling entry land in ONE commit — the operation STAYS unknown
// (reconciling is NOT state-bearing). An active un-expired lease of
// another consideration refuses with ErrReconciliationLeaseRaced (the
// honest racing skip — the maximum ONE active chain); the bound refusal
// answers the honest stale word. The signature carries ZERO adapters —
// the network query happens AFTER this commit, never inside it.
func (s *Postgres) AcquireReconciliationLease(ctx context.Context, p actionPort.AcquireReconciliationLeaseParams) (int, error) {
	if !p.Scope.Valid() {
		return 0, tenancy.ErrMissingContext
	}
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	operationID := p.Operation.OperationID
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		reconcileLockKey(tenantID, operationID)); err != nil {
		return 0, err
	}
	// The latest lease under the lock: the active twin refuses with the
	// racing signal, the expired lease yields the monotonic next attempt.
	var lastAttempt int
	var lastExpires time.Time
	var hasLease bool
	err = tx.QueryRow(ctx, `
		select attempt_no, lease_expires_at from action.reconciliation_leases
		where tenant_id=$1 and operation_id=$2 order by attempt_no desc limit 1`,
		tenantID, operationID).Scan(&lastAttempt, &lastExpires)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
	} else {
		hasLease = true
	}
	if hasLease && lastExpires.After(p.Now) {
		// The active lease of another consideration: the honest racing
		// refusal — the whole transaction rolls back, nothing is written.
		return 0, actionPort.ErrReconciliationLeaseRaced
	}
	attemptNo := lastAttempt + 1
	if attemptNo > actionDomain.MaxReconciliationAttempts {
		// The bounded limit (the epic AC-9: the Operation stays unknown
		// until human evidence, never a success by timer) — the store CHECK
		// backstop refuses, the caller speaks the honest bound word.
		return 0, actionPort.ErrReconciliationLeaseStale
	}
	// INSERT 1/2 — the write-ahead lease row: the right and the moment of
	// the query, the keyed identities alone (the raw material NEVER
	// enters) with the material backoff moment of THIS attempt.
	if _, err := tx.Exec(ctx, `
		insert into action.reconciliation_leases (tenant_id, operation_id, attempt_no, lease_token,
			definition_id, definition_version, reconciliation_word, external_idempotency_key, request_digest,
			correlation_id, lease_expires_at, next_eligible_at, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		tenantID, operationID, attemptNo, p.LeaseToken,
		p.DefinitionID, p.DefinitionVersion, p.ReconciliationWord,
		p.ExternalIDempotencyKey, p.RequestDigest, p.CorrelationID,
		p.LeaseExpiresAt.UTC().Truncate(time.Microsecond),
		p.NextEligibleAt.UTC().Truncate(time.Microsecond),
		p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return 0, mapReconcileInsertError(err)
	}
	// INSERT 2/2 — the ledger reconciling entry (NOT state-bearing — the
	// operation STAYS unknown: the reconciliation is an activity in the
	// history, never a state).
	if _, err := tx.Exec(ctx, `
		insert into action.operation_ledger (tenant_id, operation_id, entry_no, kind, details, created_at)
		values ($1,$2,$3,$4,$5,$6)`,
		tenantID, operationID, p.LedgerEntry.EntryNo, p.LedgerEntry.Kind,
		p.LedgerEntry.Details, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return 0, mapReconcileInsertError(err)
	}
	return attemptNo, tx.Commit(ctx)
}

// RecordReconciliationVerdict is the FENCED verdict transaction: the
// (attempt_no, lease_token) pair must own the CURRENT attempt — a lease
// row of the exact pair must exist AND no newer attempt may exist (the
// stale holder NEVER writes history). Inside ONE commit: the outcome row
// of the NORMALIZED verdict (the attempt ALWAYS records its guarded
// truth), the ledger entry (ONLY the state-bearing verdicts —
// committed/no_effect; in_progress/no_answer write NOTHING, unknown
// stays), the receipt v2+ with the outbox receipt.created record (ONLY
// the terminal claims that PASS the receipt terminal lock INSIDE the
// transaction: an append over the last unknown passes, the same-terminal
// confirmation skips silently — the idempotent confirmation, a
// conflicting terminal refuses with ErrTerminalConflict — never an
// auto-replacement). ANY failure rolls back the whole verdict. The
// signature carries ZERO adapters.
func (s *Postgres) RecordReconciliationVerdict(ctx context.Context, p actionPort.RecordReconciliationVerdictParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	operationID := p.Operation.OperationID
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		reconcileLockKey(tenantID, operationID)); err != nil {
		return err
	}
	// The fencing check: the exact (attempt, token) lease row must exist
	// and no newer attempt may exist — the stale holder refuses honestly.
	var leaseToken string
	err = tx.QueryRow(ctx, `
		select lease_token from action.reconciliation_leases
		where tenant_id=$1 and operation_id=$2 and attempt_no=$3`,
		tenantID, operationID, p.AttemptNo).Scan(&leaseToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actionPort.ErrReconciliationLeaseStale
		}
		return err
	}
	if leaseToken != p.LeaseToken {
		return actionPort.ErrReconciliationLeaseStale
	}
	var newer int
	if err := tx.QueryRow(ctx, `
		select count(*) from action.reconciliation_leases
		where tenant_id=$1 and operation_id=$2 and attempt_no > $3`,
		tenantID, operationID, p.AttemptNo).Scan(&newer); err != nil {
		return err
	}
	if newer > 0 {
		return actionPort.ErrReconciliationLeaseStale
	}
	// INSERT 1/3 — the outcome row of the NORMALIZED verdict: the attempt
	// ALWAYS records its guarded truth (the domain refused the unjustified
	// claims BEFORE the store); the evidence triple rides the row (the
	// empty time lands as NULL — committed survives ONLY with the complete
	// triple).
	var externalTime any
	if !p.Evidence.ExternalObjectTime.IsZero() {
		externalTime = p.Evidence.ExternalObjectTime.UTC().Truncate(time.Microsecond)
	}
	if _, err := tx.Exec(ctx, `
		insert into action.reconciliation_outcomes (tenant_id, operation_id, attempt_no, lease_token, verdict,
			correlation_id, external_object_id, external_object_version, external_object_time, details, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, operationID, p.AttemptNo, p.LeaseToken, p.Verdict,
		p.CorrelationID, p.Evidence.ExternalObjectID, p.Evidence.ExternalObjectVersion, externalTime,
		boundReconcileDetails(p.Details), p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return mapReconcileInsertError(err)
	}
	// INSERT 2/3 — the ledger entry (ONLY the state-bearing verdicts:
	// committed/no_effect — the two slash decisions of the epic;
	// in_progress/no_answer write NOTHING, the operation STAYS unknown).
	// The entry number derives under the lock (the monotonic continuation
	// of the append-only history).
	if p.LedgerWord != "" {
		var entryNo int64
		if err := tx.QueryRow(ctx, `
			select coalesce(max(entry_no), 0) + 1 from action.operation_ledger
			where tenant_id=$1 and operation_id=$2`,
			tenantID, operationID).Scan(&entryNo); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into action.operation_ledger (tenant_id, operation_id, entry_no, kind, details, created_at)
			values ($1,$2,$3,$4,$5,$6)`,
			tenantID, operationID, entryNo, p.LedgerWord,
			boundReconcileDetails(actionDomain.ReconciliationVerdictText(p.Verdict)),
			p.Now.UTC().Truncate(time.Microsecond)); err != nil {
			return mapReconcileInsertError(err)
		}
	}
	// INSERT 3/3 — the receipt v2+ with the outbox receipt.created record:
	// the TERMINAL LOCK runs INSIDE the transaction under the lock (the
	// app pre-checked with the target facts — the store re-reads the live
	// last version, the belt over braces).
	if p.ReceiptWord != "" {
		var lastVersion int64
		var lastOutcome string
		err = tx.QueryRow(ctx, `
			select receipt_version_no, outcome from action.action_receipts
			where tenant_id=$1 and operation_id=$2 order by receipt_version_no desc limit 1`,
			tenantID, operationID).Scan(&lastVersion, &lastOutcome)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// A terminal claim without ANY receipt version is the
				// corruption of the anchor (the pending read requires the
				// unknown receipt) — the integrity refusal, never a v1
				// invention inside the reconciliation contour.
				return actionPort.ErrTerminalConflict
			}
			return err
		}
		decision, guardErr := actionDomain.GuardReceiptAppend(lastOutcome, p.ReceiptWord)
		if guardErr != nil {
			return guardErr
		}
		switch decision {
		case actionDomain.ReceiptAppendAppend:
			references, err := marshalJSON(p.ReceiptReferences)
			if err != nil {
				return err
			}
			newVersion := lastVersion + 1
			if _, err := tx.Exec(ctx, `
				insert into action.action_receipts (tenant_id, operation_id, receipt_version_no, outcome, receipt_references, details, created_at)
				values ($1,$2,$3,$4,$5,$6,$7)`,
				tenantID, operationID, newVersion, p.ReceiptWord, references,
				boundReconcileDetails(p.ReceiptDetails), p.Now.UTC().Truncate(time.Microsecond)); err != nil {
				return mapReconcileInsertError(err)
			}
			// The outbox record rides the SAME commit with the payload
			// patched by the derived version and the claimed outcome (the
			// payload structure travels BY VALUE — the local copy carries
			// the store-derived truth).
			payload := p.OutboxPayload
			payload.ReceiptVersionNo = newVersion
			payload.Outcome = p.ReceiptWord
			if err := insertOutboxReceiptRow(ctx, tx, tenantID, p.OutboxEventID, payload,
				p.Now.UTC().Truncate(time.Microsecond)); err != nil {
				return mapReconcileInsertError(err)
			}
		case actionDomain.ReceiptAppendSkip:
			// The idempotent confirmation: the terminal outcome is already
			// recorded with the SAME word — NO new version, NO outbox
			// record (the verdict row above still lands: the attempt's
			// truth is one row per attempt); the silent skip, the honest
			// nil.
		case actionDomain.ReceiptAppendAlarm:
			// The integrity conflict of the terminal outcomes: the append
			// is forbidden — the human review owns the resolution, never
			// an auto-replacement (the whole verdict rolls back).
			return actionPort.ErrTerminalConflict
		}
	}
	return tx.Commit(ctx)
}

// ReadReconciliationAttempts reads the reconciliation attempts of one
// operation ordered by the monotonic attempt number (the view material —
// the attempts land in the closed details of the operation card).
func (s *Postgres) ReadReconciliationAttempts(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]actionPort.ReconciliationAttemptRow, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		select l.attempt_no, l.lease_token, l.lease_expires_at, l.next_eligible_at, l.created_at, l.reconciliation_word,
		       o.verdict, o.correlation_id, o.external_object_id, o.created_at
		from action.reconciliation_leases l
		left join action.reconciliation_outcomes o
		  on o.tenant_id=l.tenant_id and o.operation_id=l.operation_id and o.attempt_no=l.attempt_no
		where l.tenant_id=$1 and l.operation_id=$2
		order by l.attempt_no`, scope.TenantID(), operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attempts := []actionPort.ReconciliationAttemptRow{}
	for rows.Next() {
		var row actionPort.ReconciliationAttemptRow
		var verdict *string
		var correlation *string
		var externalObjectID *string
		var outcomeAt *time.Time
		if err := rows.Scan(&row.AttemptNo, &row.LeaseToken, &row.LeaseExpiresAt, &row.NextEligibleAt,
			&row.LeaseCreatedAt, &row.ReconciliationWord,
			&verdict, &correlation, &externalObjectID, &outcomeAt); err != nil {
			return nil, err
		}
		row.LeaseExpiresAt = row.LeaseExpiresAt.UTC()
		row.NextEligibleAt = row.NextEligibleAt.UTC()
		row.LeaseCreatedAt = row.LeaseCreatedAt.UTC()
		if verdict != nil {
			row.OutcomeVerdict = *verdict
		}
		if correlation != nil {
			row.CorrelationID = *correlation
		}
		if externalObjectID != nil {
			row.ExternalObjectID = *externalObjectID
		}
		if outcomeAt != nil {
			row.OutcomeCreatedAt = (*outcomeAt).UTC()
		}
		attempts = append(attempts, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attempts, tx.Commit(ctx)
}

// LastReconciliationNoEffect reads the recorded no_effect verdict flag of
// one operation (the retry-unlock input of the dispatch pending
// classification — the PROVEN absence of the external effect replaces
// the guarantee role; the dispatch store reads it in its own SQL, this
// read serves the unit fake and the view).
func (s *Postgres) LastReconciliationNoEffect(ctx context.Context, scope tenancy.Context, operationID id.UUID) (bool, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var recorded bool
	if err := tx.QueryRow(ctx, `
		select exists (select 1 from action.reconciliation_outcomes
		where tenant_id=$1 and operation_id=$2 and verdict='no_effect')`,
		scope.TenantID(), operationID).Scan(&recorded); err != nil {
		return false, err
	}
	return recorded, tx.Commit(ctx)
}
