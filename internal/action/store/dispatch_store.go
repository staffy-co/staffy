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
// Story 3.6 (step-02): the dispatch persistence — the write-ahead lease,
// the fenced verdict and the raced-guarded parking
// ---------------------------------------------------------------------------
//
// The THREE transactions of the execution contour, each with ZERO
// adapters in the signature (a network call is impossible BY TYPE — the
// persist-before-emit discipline of the Enqueue/ReserveOperation
// precedents): AcquireDispatchLease materializes the RIGHT and the MOMENT
// of the send BEFORE the provider call (the lease row plus the ledger
// submitted entry in ONE commit under the dispatch advisory lock);
// RecordDispatchOutcome writes the verdict fenced by the (attempt_no,
// lease_token) pair — a stale holder NEVER writes history; AppendBlocked
// parks the operation with the exact fence word, refusing honestly when
// an active lease of another consideration owns the operation. The
// pending classification is the ONE dispatcher of decisions: every queued
// job is considered, not every job is sent.

// dispatchLockKey is the advisory lock key of one operation's dispatch
// contour (the confirmationLockKey precedent): every transaction of the
// contour takes it — the raced twin of any append waits, then sees the
// winner's rows and answers the honest signal.
func dispatchLockKey(tenantID id.UUID, operationID id.UUID) string {
        return "action:dispatch:" + tenantID.String() + ":" + operationID.String()
}

// mapDispatchInsertError maps the raced insert conflicts and the CHECK
// backstops onto the honest consumer-owned signals of the fencing
// dictionary.
func mapDispatchInsertError(err error) error {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
                switch pgErr.ConstraintName {
                case "dispatch_leases_pkey":
                        return actionPort.ErrOutcomeDuplicate
                case "dispatch_outcomes_pkey":
                        return actionPort.ErrOutcomeDuplicate
                case "action_receipts_pkey":
                        return actionPort.ErrOutcomeDuplicate
                case "operation_ledger_pkey":
                        return actionPort.ErrStaleConfirmationVersion
                }
                if pgErr.SQLState() == "23514" {
                        return actionPort.ErrLeaseStale
                }
        }
        return err
}

// classifyPendingJob derives the STRUCTURAL classification word of one
// queued job from the derived ledger state and the attempt facts (the
// state-driven form: the ledger is the single truth of the operation's
// life). racing — an active lease owns the operation; submitted — the
// write-ahead resting state of a crash (recoverable); unknown — a
// recorded unknown verdict (retryable, the policy decides over the
// recorded guarantee); accepted/committed/failed — decided; authorized
// with a prior dispatch_blocked entry — blocked (the fences re-run, the
// cause may have healed); authorized with nothing — fresh.
func classifyPendingJob(state string, attemptCount int, leaseActive bool, hasBlockedEntry bool) string {
        if leaseActive {
                return actionPort.PendingClassRacing
        }
        switch state {
        case actionDomain.OperationSubmitted:
                return actionPort.PendingClassRecoverable
        case actionDomain.OperationUnknown:
                return actionPort.PendingClassRetryable
        case actionDomain.OperationAccepted, actionDomain.OperationCommitted, actionDomain.OperationFailed:
                return actionPort.PendingClassDecided
        }
        if attemptCount > 0 {
                // authorized with attempt facts and no active lease: the ledger
                // stayed authorized — the attempts belong to a parked history.
                return actionPort.PendingClassBlocked
        }
        if hasBlockedEntry {
                return actionPort.PendingClassBlocked
        }
        return actionPort.PendingClassFresh
}

// PendingDispatchJobs reads the queued jobs of the tenant — every
// operation.queued outbox record (the at-leat-once delivery unit: the
// duplicate rows of one operation are LEGAL, the classification governs
// the repeat) joined with the immutable operation, the append-only
// ledger (the state derives), the latest lease/outcome facts and the
// first recorded digest/key pair (the decided-verify inputs). The batch
// is bounded by the drain limit — the rest stay queued for the next run.
func (s *Postgres) PendingDispatchJobs(ctx context.Context, scope tenancy.Context, now time.Time, limit int) ([]actionPort.PendingDispatchJob, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        if limit < 1 {
                limit = 1
        }
        rows, err := tx.Query(ctx, `
                select e.event_id, e.payload,
                       o.operation_id, o.definition_id, o.definition_version,
                       o.preview_id, o.preview_version_no, o.confirmation_id, o.confirmation_version_no,
                       o.case_id, o.work_item_id, o.target_kind, o.target_ref,
                       o.owner_kind, o.owner_ref, o.owner_epoch, o.release_decision_no, o.profile_version,
                       o.partition, o.effects, o.business_intent_key, o.idempotency_key, o.material_digest, o.created_at,
                       lg.ledger,
                       (select count(*) from action.dispatch_leases lc where lc.tenant_id=e.tenant_id and lc.operation_id=e.operation_id),
                       lf.attempt_no, lf.lease_token, lf.lease_expires_at,
                       (select request_digest from action.dispatch_leases ff where ff.tenant_id=e.tenant_id and ff.operation_id=e.operation_id order by ff.attempt_no asc limit 1),
                       (select external_idempotency_key from action.dispatch_leases ff where ff.tenant_id=e.tenant_id and ff.operation_id=e.operation_id order by ff.attempt_no asc limit 1),
                       uo.outcome, uo.correlation_id, uo.guarantee_recorded
                from action.outbox_events e
                join action.operations o on o.tenant_id=e.tenant_id and o.operation_id=e.operation_id
                left join lateral (
                        select json_agg(x order by x.entry_no) as ledger
                        from (select entry_no, kind, details, created_at from action.operation_ledger le
                              where le.tenant_id=e.tenant_id and le.operation_id=e.operation_id) x
                ) lg on true
                left join lateral (
                        select attempt_no, lease_token, lease_expires_at
                        from action.dispatch_leases l
                        where l.tenant_id=e.tenant_id and l.operation_id=e.operation_id
                        order by l.attempt_no desc limit 1
                ) lf on true
                left join lateral (
                        select outcome, correlation_id, guarantee_recorded
                        from action.dispatch_outcomes u
                        where u.tenant_id=e.tenant_id and u.operation_id=e.operation_id
                        order by u.attempt_no desc limit 1
                ) uo on true
                where e.tenant_id=$1 and e.event_type='operation.queued'
                order by e.created_at, e.event_id
                limit $2`, scope.TenantID(), limit)
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        jobs := []actionPort.PendingDispatchJob{}
        for rows.Next() {
                job := actionPort.PendingDispatchJob{
                        LastLeaseExpiresAt: time.Time{},
                }
                var effects []byte
                var ledger []byte
                var attemptCount int
                var lastAttemptNo *int
                var lastLeaseToken *string
                var lastLeaseExpiresAt *time.Time
                var requestDigest *string
                var externalKey *string
                var lastOutcome *string
                var lastCorrelation *string
                var lastGuarantee *bool
                if err := rows.Scan(&job.EventID, &job.RawPayload,
                        &job.Operation.OperationID, &job.Operation.DefinitionID, &job.Operation.DefinitionVersion,
                        &job.Operation.PreviewID, &job.Operation.PreviewVersionNo, &job.Operation.ConfirmationID, &job.Operation.ConfirmationVersionNo,
                        &job.Operation.CaseID, &job.Operation.WorkItemID, &job.Operation.Target.Kind, &job.Operation.Target.Ref,
                        &job.Operation.OwnerKind, &job.Operation.OwnerRef, &job.Operation.OwnerEpoch, &job.Operation.ReleaseDecisionNo, &job.Operation.ProfileVersion,
                        &job.Operation.Partition, &effects, &job.Operation.BusinessIntentKey, &job.Operation.IdempotencyKey, &job.Operation.MaterialDigest, &job.Operation.CreatedAt,
                        &ledger, &attemptCount,
                        &lastAttemptNo, &lastLeaseToken, &lastLeaseExpiresAt,
                        &requestDigest, &externalKey,
                        &lastOutcome, &lastCorrelation, &lastGuarantee); err != nil {
                        return nil, err
                }
                job.Operation.TenantID = scope.TenantID()
                if err := json.Unmarshal(effects, &job.Operation.Effects); err != nil {
                        return nil, err
                }
                job.Operation.CreatedAt = job.Operation.CreatedAt.UTC()
                if ledger != nil {
                        if err := json.Unmarshal(ledger, &job.Ledger); err != nil {
                                return nil, err
                        }
                } else {
                        job.Ledger = []actionDomain.OperationLedgerEntry{}
                }
                job.AttemptCount = attemptCount
                if lastAttemptNo != nil {
                        job.LastAttemptNo = *lastAttemptNo
                }
                if lastLeaseToken != nil {
                        job.LastLeaseToken = *lastLeaseToken
                }
                if lastLeaseExpiresAt != nil {
                        job.LastLeaseExpiresAt = (*lastLeaseExpiresAt).UTC()
                        job.LastLeaseActive = job.LastLeaseExpiresAt.After(now)
                }
                if requestDigest != nil {
                        job.RequestDigest = *requestDigest
                }
                if externalKey != nil {
                        job.ExternalIDempotencyKey = *externalKey
                }
                if lastOutcome != nil {
                        job.LastOutcomeWord = *lastOutcome
                }
                if lastCorrelation != nil {
                        job.LastOutcomeCorrelation = *lastCorrelation
                }
                if lastGuarantee != nil {
                        job.LastOutcomeGuarantee = *lastGuarantee
                }
                state, err := actionDomain.DeriveOperationState(job.Ledger)
                if err != nil {
                        return nil, err
                }
                job.StateWord = state
                hasBlocked := false
                for _, entry := range job.Ledger {
                        if entry.Kind == actionDomain.LedgerEntryDispatchBlocked {
                                hasBlocked = true
                        }
                }
                job.Classification = classifyPendingJob(state, attemptCount, job.LastLeaseActive, hasBlocked)
                jobs = append(jobs, job)
        }
        if err := rows.Err(); err != nil {
                return nil, err
        }
        return jobs, tx.Commit(ctx)
}

// AcquireDispatchLease is the WRITE-AHEAD transaction of the send right
// (persist-before-emit, AC-1/AC-5): under the dispatch advisory lock the
// monotonic attempt_no derives (the latest lease + 1, bounded by
// MaxDispatchAttempts — the store CHECK is the loyal backstop), then the
// lease row and the ledger submitted entry land in ONE commit. An active
// un-expired lease of another consideration refuses with ErrLeaseRaced
// (the honest racing skip — no second right to send); the bound refusal
// answers the honest stale word (the caller parks with the bound text).
// The signature carries ZERO adapters — the network call happens AFTER
// this commit, never inside it.
func (s *Postgres) AcquireDispatchLease(ctx context.Context, p actionPort.AcquireDispatchLeaseParams) (int, error) {
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
                dispatchLockKey(tenantID, operationID)); err != nil {
                return 0, err
        }
        // The latest lease under the lock: the active twin refuses with the
        // racing signal, the expired lease yields the monotonic next attempt.
        var lastAttempt int
        var lastExpires time.Time
        var hasLease bool
        err = tx.QueryRow(ctx, `
                select attempt_no, lease_expires_at from action.dispatch_leases
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
                return 0, actionPort.ErrLeaseRaced
        }
        attemptNo := lastAttempt + 1
        if attemptNo > actionDomain.MaxDispatchAttempts {
                // The bounded limit (the 3-3 precedent: bounded, no blind loop) —
                // the store CHECK backstop refuses, the caller parks with the
                // honest bound word.
                return 0, actionPort.ErrLeaseStale
        }
        // INSERT 1/2 — the write-ahead lease row: the right and the moment of
        // the send, the keyed identities alone (the raw payload NEVER enters).
        if _, err := tx.Exec(ctx, `
                insert into action.dispatch_leases (tenant_id, operation_id, attempt_no, lease_token,
                        definition_id, definition_version, external_idempotency_key, request_digest, lease_expires_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
                tenantID, operationID, attemptNo, p.LeaseToken,
                p.DefinitionID, p.DefinitionVersion, p.ExternalIDempotencyKey, p.RequestDigest,
                p.LeaseExpiresAt.UTC().Truncate(time.Microsecond), p.Now.UTC().Truncate(time.Microsecond)); err != nil {
                return 0, mapDispatchInsertError(err)
        }
        // INSERT 2/2 — the ledger submitted entry: the state moves BEFORE the
        // network call (the resting state of a crash between the send and the
        // verdict — the expired lease shows the recovery).
        if _, err := tx.Exec(ctx, `
                insert into action.operation_ledger (tenant_id, operation_id, entry_no, kind, details, created_at)
                values ($1,$2,$3,$4,$5,$6)`,
                tenantID, operationID, p.LedgerEntry.EntryNo, p.LedgerEntry.Kind,
                p.LedgerEntry.Details, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
                return 0, mapDispatchInsertError(err)
        }
        return attemptNo, tx.Commit(ctx)
}

// RecordDispatchOutcome is the FENCED verdict transaction: the
// (attempt_no, lease_token) pair must own the CURRENT attempt — a lease
// row of the exact pair must exist AND no newer attempt may exist (the
// stale holder NEVER writes history, AC-1). Inside ONE commit: the
// outcome row (ONLY when the claim word is honest — the refused or
// absent claims record no row, the lease/ledger/receipt speak the
// guarded truth), the ledger kind entry, the receipt v1 (ONLY on the
// first deciding verdict — accepted carries no receipt) and the outbox
// receipt.created record. ANY failure rolls back the whole verdict. The
// signature carries ZERO adapters.
func (s *Postgres) RecordDispatchOutcome(ctx context.Context, p actionPort.RecordDispatchOutcomeParams) error {
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
                dispatchLockKey(tenantID, operationID)); err != nil {
                return err
        }
        // The fencing check: the exact (attempt, token) lease row must exist
        // and no newer attempt may exist — the stale holder refuses honestly.
        var leaseToken string
        err = tx.QueryRow(ctx, `
                select lease_token from action.dispatch_leases
                where tenant_id=$1 and operation_id=$2 and attempt_no=$3`,
                tenantID, operationID, p.AttemptNo).Scan(&leaseToken)
        if err != nil {
                if errors.Is(err, pgx.ErrNoRows) {
                        return actionPort.ErrLeaseStale
                }
                return err
        }
        if leaseToken != p.LeaseToken {
                return actionPort.ErrLeaseStale
        }
        var newer int
        if err := tx.QueryRow(ctx, `
                select count(*) from action.dispatch_leases
                where tenant_id=$1 and operation_id=$2 and attempt_no > $3`,
                tenantID, operationID, p.AttemptNo).Scan(&newer); err != nil {
                return err
        }
        if newer > 0 {
                return actionPort.ErrLeaseStale
        }
        // INSERT 1/4 — the outcome row of the honest claim word ("" records
        // no row: the refused or absent claims — the guarantee stays false,
        // the retry policy of the next drain parks honestly).
        if p.OutcomeWord != "" {
                var externalTime any
                if !p.Evidence.ExternalObjectTime.IsZero() {
                        externalTime = p.Evidence.ExternalObjectTime.UTC().Truncate(time.Microsecond)
                }
                if _, err := tx.Exec(ctx, `
                        insert into action.dispatch_outcomes (tenant_id, operation_id, attempt_no, lease_token, outcome,
                                correlation_id, external_object_id, external_object_version, external_object_time,
                                guarantee_recorded, details, created_at)
                        values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
                        tenantID, operationID, p.AttemptNo, p.LeaseToken, p.OutcomeWord,
                        p.CorrelationID, p.Evidence.ExternalObjectID, p.Evidence.ExternalObjectVersion, externalTime,
                        p.GuaranteeRecorded, p.Details, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
                        return mapDispatchInsertError(err)
                }
        }
        // INSERT 2/4 — the ledger kind entry (the guarded verdict state —
        // unknown for the refused claims, the honest words otherwise).
        if _, err := tx.Exec(ctx, `
                insert into action.operation_ledger (tenant_id, operation_id, entry_no, kind, details, created_at)
                values ($1,$2,$3,$4,$5,$6)`,
                tenantID, operationID, p.LedgerEntry.EntryNo, p.LedgerEntry.Kind,
                p.LedgerEntry.Details, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
                return mapDispatchInsertError(err)
        }
        // INSERT 3/4 — the receipt v1 (the FIRST deciding verdict only: the
        // unique (tenant, operation, version) backstop; accepted carries no
        // receipt — the business result is not confirmed yet).
        if p.ReceiptWord != "" {
                references, err := marshalJSON(p.ReceiptReferences)
                if err != nil {
                        return err
                }
                if _, err := tx.Exec(ctx, `
                        insert into action.action_receipts (tenant_id, operation_id, receipt_version_no, outcome, references, details, created_at)
                        values ($1,$2,$3,$4,$5,$6,$7)`,
                        tenantID, operationID, p.ReceiptVersionNo, p.ReceiptWord, references,
                        p.ReceiptDetails, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
                        return mapDispatchInsertError(err)
                }
        }
        // INSERT 4/4 — the outbox receipt.created record (AD-2: the Work
        // Item/Conversation link rides THIS module's own outbox — the future
        // consumer reads the pending records). ONLY when the verdict is
        // deciding: accepted carries no receipt, no receipt event either.
        if p.ReceiptWord != "" {
                if err := insertOutboxReceiptRow(ctx, tx, tenantID, p.OutboxEventID, p.OutboxPayload,
                        p.Now.UTC().Truncate(time.Microsecond)); err != nil {
                        return err
                }
        }
        return tx.Commit(ctx)
}

// insertOutboxReceiptRow inserts one receipt.created outbox record into
// an open transaction (the shared form of the verdict insert and the
// standalone append — the payload is the SAFE canonical jsonb: ids,
// words and the external object identity alone).
func insertOutboxReceiptRow(ctx context.Context, tx pgx.Tx, tenantID id.UUID, eventID id.UUID,
        payload actionPort.ReceiptCreatedPayload, now time.Time) error {
        body, err := marshalJSON(payload)
        if err != nil {
                return err
        }
        _, err = tx.Exec(ctx, `
                insert into action.outbox_events (tenant_id, event_id, event_type, case_id, operation_id, payload, created_at)
                values ($1,$2,$3,$4,$5,$6,$7)`,
                tenantID, eventID, actionDomain.OutboxEventReceiptCreated, payload.CaseID,
                payload.OperationID, body, now)
        return err
}

// AppendDispatchBlocked parks the operation with the EXACT fence reason
// word (raced-guarded, the GasExternalCommitmentQueue precedent of 3-3):
// under the dispatch advisory lock an ACTIVE un-expired lease of another
// consideration refuses with ErrLeaseRaced — the honest skip (the
// dispatching worker owns the operation now, its verdict will speak);
// otherwise the dispatch_blocked ledger entry appends (NOT state-bearing
// — the operation stays authorized, parked with a visible cause; never a
// false failed externally, AC-2).
func (s *Postgres) AppendDispatchBlocked(ctx context.Context, p actionPort.AppendDispatchBlockedParams) error {
        if !p.Scope.Valid() {
                return tenancy.ErrMissingContext
        }
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        tenantID := p.Scope.TenantID()
        if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
                dispatchLockKey(tenantID, p.OperationID)); err != nil {
                return err
        }
        var active int
        if err := tx.QueryRow(ctx, `
                select count(*) from action.dispatch_leases
                where tenant_id=$1 and operation_id=$2 and lease_expires_at > $3`,
                tenantID, p.OperationID, p.Now.UTC().Truncate(time.Microsecond)).Scan(&active); err != nil {
                return err
        }
        if active > 0 {
                return actionPort.ErrLeaseRaced
        }
        if _, err := tx.Exec(ctx, `
                insert into action.operation_ledger (tenant_id, operation_id, entry_no, kind, details, created_at)
                values ($1,$2,$3,$4,$5,$6)`,
                tenantID, p.OperationID, p.LedgerEntry.EntryNo, actionDomain.LedgerEntryDispatchBlocked,
                p.Details, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
                return mapDispatchInsertError(err)
        }
        return tx.Commit(ctx)
}

// AppendOutboxReceiptCreated appends one receipt.created record
// standalone (the future 3-7 contour form of the verdict insert).
func (s *Postgres) AppendOutboxReceiptCreated(ctx context.Context, p actionPort.AppendOutboxReceiptCreatedParams) error {
        if !p.Scope.Valid() {
                return tenancy.ErrMissingContext
        }
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        if err := insertOutboxReceiptRow(ctx, tx, p.Scope.TenantID(), p.EventID, p.Payload,
                p.Now.UTC().Truncate(time.Microsecond)); err != nil {
                return err
        }
        return tx.Commit(ctx)
}

// ReadDispatchAttempts reads the attempts of one operation ordered by the
// monotonic attempt number (the view material — the attempts land in the
// closed details of the operation card).
func (s *Postgres) ReadDispatchAttempts(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]actionPort.DispatchAttemptRow, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, `
                select l.attempt_no, l.lease_token, l.lease_expires_at, l.created_at, l.request_digest,
                       o.outcome, o.correlation_id, o.guarantee_recorded, o.created_at
                from action.dispatch_leases l
                left join action.dispatch_outcomes o
                  on o.tenant_id=l.tenant_id and o.operation_id=l.operation_id and o.attempt_no=l.attempt_no
                where l.tenant_id=$1 and l.operation_id=$2
                order by l.attempt_no`, scope.TenantID(), operationID)
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        attempts := []actionPort.DispatchAttemptRow{}
        for rows.Next() {
                var row actionPort.DispatchAttemptRow
                var outcome *string
                var correlation *string
                var guarantee *bool
                var outcomeAt *time.Time
                if err := rows.Scan(&row.AttemptNo, &row.LeaseToken, &row.LeaseExpiresAt, &row.LeaseCreatedAt, &row.RequestDigest,
                        &outcome, &correlation, &guarantee, &outcomeAt); err != nil {
                        return nil, err
                }
                row.LeaseExpiresAt = row.LeaseExpiresAt.UTC()
                row.LeaseCreatedAt = row.LeaseCreatedAt.UTC()
                if outcome != nil {
                        row.OutcomeWord = *outcome
                }
                if correlation != nil {
                        row.CorrelationID = *correlation
                }
                if guarantee != nil {
                        row.GuaranteeRecorded = *guarantee
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

// ReadActionReceipts reads the receipt versions of one operation ordered
// by the version number (the view material; v1 is never rewritten).
func (s *Postgres) ReadActionReceipts(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]actionPort.ActionReceiptRow, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, `
                select receipt_version_no, outcome, references, details, created_at
                from action.action_receipts
                where tenant_id=$1 and operation_id=$2 order by receipt_version_no`,
                scope.TenantID(), operationID)
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        receipts := []actionPort.ActionReceiptRow{}
        for rows.Next() {
                var row actionPort.ActionReceiptRow
                var references []byte
                if err := rows.Scan(&row.ReceiptVersionNo, &row.Outcome, &references, &row.Details, &row.CreatedAt); err != nil {
                        return nil, err
                }
                if err := json.Unmarshal(references, &row.References); err != nil {
                        return nil, err
                }
                row.CreatedAt = row.CreatedAt.UTC()
                receipts = append(receipts, row)
        }
        if err := rows.Err(); err != nil {
                return nil, err
        }
        return receipts, tx.Commit(ctx)
}
