package main

import (
	"context"
	"sort"
	"strings"
	"time"

	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	"github.com/staffy-co/staffy/internal/adapters/actiondispatcher"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// The Story 3.6 (step-03) fixture: the execution contour over the REAL app
// service (the ordered fences of WithDispatch run the REAL domain
// derivations and the REAL gate ladder), the in-memory dispatch repository
// over the SAME authorization rows (the queued outbox rows, the immutable
// operations, the append-only ledger — the "мир пути, не продуктовая
// персистентность" of the spec) and the synthetic mutation adapter with
// its /fixture/action-dispatch toggles (the scriptable outcome /
// correlation / evidence / failure / guarantee words — the 3-3
// dispatch-outcome precedent 1:1). Production wires NO adapter (AG-4) —
// this package is the test contour the production binary never imports.
//
// The honest fencing semantics of the PG store are ported EXACTLY: the
// monotonic attempt derivation under the lock (the active twin refuses
// with the racing signal — no second right to send), the bounded attempt
// limit (the loyal CHECK backstop of the store speaks ErrLeaseStale), the
// fenced verdict ((attempt, token) ownership — a stale holder NEVER
// writes history, a duplicate outcome NEVER lands twice) and the
// raced-guarded parking append; the write-ahead lease and the ledger
// submitted entry land atomically under the single mutex (the checks run
// BEFORE the first mutation, the failure paths leave no partial state).

// authorizationOutboxRow is one queued delivery unit of the fixture world
// (the at-least-once operation.queued record — the duplicate rows of one
// operation are legal, the classification governs the repeat).
type authorizationOutboxRow struct {
	EventID   id.UUID
	Payload   []byte
	CreatedAt time.Time
}

// dispatchLeaseRow is one write-ahead lease row of the fixture world.
type dispatchLeaseRow struct {
	AttemptNo              int
	LeaseToken             string
	LeaseExpiresAt         time.Time
	LeaseCreatedAt         time.Time
	ExternalIDempotencyKey string
	RequestDigest          string
	DefinitionID           string
	DefinitionVersion      string
}

// dispatchOutcomeRow is one recorded verdict row of the fixture world.
type dispatchOutcomeRow struct {
	AttemptNo         int
	OutcomeWord       string
	CorrelationID     string
	GuaranteeRecorded bool
	CreatedAt         time.Time
}

// dispatchReceiptRow is one append-only receipt version of the fixture
// world (v1 is never rewritten).
type dispatchReceiptRow struct {
	ReceiptVersionNo int64
	Outcome          string
	References       actionPort.ReceiptReferences
	Details          string
	CreatedAt        time.Time
}

// dispatchMemoryRepository is the in-memory DispatchRepository of the
// fixture world over the SAME locked authorization state (the advisory
// lock of the PG store is the single mutex here — one lock guards the
// whole fixture world, the drain is strictly sequential by construction).
type dispatchMemoryRepository struct {
	auth *authorizationMemoryRepository

	leases        map[string][]dispatchLeaseRow   // tenant|operationID → attempts asc
	outcomes      map[string][]dispatchOutcomeRow // tenant|operationID|attemptNo
	receipts      map[string][]dispatchReceiptRow // tenant|operationID → versions asc
	receiptOutbox []actionPort.ReceiptCreatedPayload

	// reconcileNoEffectLocked is the 3-7 retry-unlock reader injected by
	// the reconciliation wiring (wireActionReconcileWorld): nil in the
	// dispatch-only worlds — no reconciliation verdicts exist there, the
	// noEffect flag honestly stays false. The reader assumes the CALLER
	// owns the shared auth mutex (the fixture composition synchronises
	// on that ONE root).
	reconcileNoEffectLocked func(scope tenancy.Context, operationID id.UUID) bool
}

// newDispatchMemoryRepository constructs the execution-contour repository
// over the authorization rows of the fixture world.
func newDispatchMemoryRepository(auth *authorizationMemoryRepository) *dispatchMemoryRepository {
	return &dispatchMemoryRepository{
		auth:     auth,
		leases:   map[string][]dispatchLeaseRow{},
		outcomes: map[string][]dispatchOutcomeRow{},
		receipts: map[string][]dispatchReceiptRow{},
	}
}

// The compile-time proof the fixture repository satisfies the consumer
// contract of the contour (the narrow interface the app type-asserts).
var _ actionPort.DispatchRepository = (*dispatchMemoryRepository)(nil)

// dispatchOperationKey builds the operation identity key of the tenant
// scope (the authorization key reuse — the SAME rows).
func dispatchOperationKey(scope tenancy.Context, operationID id.UUID) string {
	return scope.TenantID().String() + "|" + operationID.String()
}

// dispatchOutcomeKey builds the attempt-scope key of the verdict rows.
func dispatchOutcomeKey(scope tenancy.Context, operationID id.UUID, attemptNo int) string {
	return scope.TenantID().String() + "|" + operationID.String() + "|" + itoa(attemptNo)
}

// classifyDispatchJob ports the PG classification EXACTLY (the ONE
// dispatcher of decisions — every job is considered, not every job is
// sent).
func classifyDispatchJob(state string, attemptCount int, leaseActive bool, hasBlockedEntry bool, noEffect bool) string {
	if leaseActive {
		return actionPort.PendingClassRacing
	}
	switch state {
	case actionDomain.OperationSubmitted:
		return actionPort.PendingClassRecoverable
	case actionDomain.OperationUnknown:
		return actionPort.PendingClassRetryable
	case actionDomain.OperationFailed:
		// 3-7 retry-unlock (the production classifyPendingJob port): the
		// recorded no_effect verdict is the PROVEN absence of the external
		// effect — the proof of emptiness replaces the guarantee role of
		// provider_idempotent, the retry unlocks WITHOUT the guarantee;
		// failed without the verdict stays decided (a new launch demands a
		// new Confirmation of 3-5).
		if noEffect {
			return actionPort.PendingClassRetryable
		}
		return actionPort.PendingClassDecided
	case actionDomain.OperationAccepted, actionDomain.OperationCommitted:
		return actionPort.PendingClassDecided
	}
	if attemptCount > 0 || hasBlockedEntry {
		return actionPort.PendingClassBlocked
	}
	return actionPort.PendingClassFresh
}

// PendingDispatchJobs reads the queued jobs of the tenant over the SAME
// authorization rows: every operation.queued outbox record joined with the
// immutable operation, the append-only ledger (the state derives), the
// latest lease/outcome facts and the first recorded digest/key pair (the
// decided-verify inputs). The batch is bounded by the drain limit.
func (r *dispatchMemoryRepository) PendingDispatchJobs(_ context.Context, scope tenancy.Context, now time.Time, limit int) ([]actionPort.PendingDispatchJob, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	if limit < 1 {
		limit = 1
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	jobs := []actionPort.PendingDispatchJob{}
	type queuedUnit struct {
		row       authorizationOutboxRow
		operation actionDomain.Operation
	}
	units := []queuedUnit{}
	for key, operation := range r.auth.operations {
		if !strings.HasPrefix(key, scope.TenantID().String()+"|") {
			continue
		}
		for _, row := range r.auth.outboxRows[key] {
			units = append(units, queuedUnit{row: row, operation: operation})
		}
	}
	sort.Slice(units, func(i, j int) bool {
		if units[i].row.CreatedAt.Equal(units[j].row.CreatedAt) {
			return units[i].row.EventID.String() < units[j].row.EventID.String()
		}
		return units[i].row.CreatedAt.Before(units[j].row.CreatedAt)
	})
	if len(units) > limit {
		units = units[:limit]
	}
	for _, unit := range units {
		operationKey := dispatchOperationKey(scope, unit.operation.OperationID)
		ledger := append([]actionDomain.OperationLedgerEntry(nil), r.auth.ledger[operationKey]...)
		leases := r.leases[operationKey]
		state, err := actionDomain.DeriveOperationState(ledger)
		if err != nil {
			return nil, err
		}
		job := actionPort.PendingDispatchJob{
			EventID:      unit.row.EventID,
			RawPayload:   unit.row.Payload,
			Operation:    unit.operation,
			Ledger:       ledger,
			StateWord:    state,
			AttemptCount: len(leases),
		}
		hasBlocked := false
		for _, entry := range ledger {
			if entry.Kind == actionDomain.LedgerEntryDispatchBlocked {
				hasBlocked = true
			}
		}
		if len(leases) > 0 {
			last := leases[len(leases)-1]
			job.LastAttemptNo = last.AttemptNo
			job.LastLeaseToken = last.LeaseToken
			job.LastLeaseExpiresAt = last.LeaseExpiresAt
			job.LastLeaseActive = last.LeaseExpiresAt.After(now)
			job.RequestDigest = leases[0].RequestDigest
			job.ExternalIDempotencyKey = leases[0].ExternalIDempotencyKey
			if outcomes := r.outcomes[dispatchOutcomeKey(scope, unit.operation.OperationID, last.AttemptNo)]; len(outcomes) > 0 {
				outcome := outcomes[len(outcomes)-1]
				job.LastOutcomeWord = outcome.OutcomeWord
				job.LastOutcomeCorrelation = outcome.CorrelationID
				job.LastOutcomeGuarantee = outcome.GuaranteeRecorded
			}
		}
		// The 3-7 retry-unlock input (the production lateral join port): the
		// last recorded reconciliation verdict of the operation answers
		// whether the absence of the external effect is PROVEN; the reader
		// is injected by the reconciliation wiring (nil in the worlds
		// without the contour — the flag honestly stays false).
		noEffect := false
		if r.reconcileNoEffectLocked != nil {
			noEffect = r.reconcileNoEffectLocked(scope, unit.operation.OperationID)
		}
		job.ReconciliationNoEffect = noEffect
		job.Classification = classifyDispatchJob(state, len(leases), job.LastLeaseActive, hasBlocked, noEffect)
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// AcquireDispatchLease is the WRITE-AHEAD transaction of the send right
// (persist-before-emit): under the lock the monotonic attempt derives
// (bounded by MaxDispatchAttempts), then the lease row and the ledger
// submitted entry land atomically. An active lease of another
// consideration refuses with ErrLeaseRaced; the bound refusal answers
// ErrLeaseStale. The signature carries ZERO adapters.
func (r *dispatchMemoryRepository) AcquireDispatchLease(_ context.Context, p actionPort.AcquireDispatchLeaseParams) (int, error) {
	if !p.Scope.Valid() {
		return 0, tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	operationKey := dispatchOperationKey(p.Scope, p.Operation.OperationID)
	leases := r.leases[operationKey]
	if len(leases) > 0 && leases[len(leases)-1].LeaseExpiresAt.After(p.Now) {
		// The active lease of another consideration: the honest racing
		// refusal — nothing is written.
		return 0, actionPort.ErrLeaseRaced
	}
	attemptNo := len(leases) + 1
	if attemptNo > actionDomain.MaxDispatchAttempts {
		// The bounded limit — the store CHECK backstop speaks the same word.
		return 0, actionPort.ErrLeaseStale
	}
	submittedEntry := p.LedgerEntry
	submittedEntry.At = p.Now.UTC()
	r.leases[operationKey] = append(leases, dispatchLeaseRow{
		AttemptNo: attemptNo, LeaseToken: p.LeaseToken,
		LeaseExpiresAt: p.LeaseExpiresAt.UTC(), LeaseCreatedAt: p.Now.UTC(),
		ExternalIDempotencyKey: p.ExternalIDempotencyKey, RequestDigest: p.RequestDigest,
		DefinitionID: p.DefinitionID, DefinitionVersion: p.DefinitionVersion,
	})
	r.auth.ledger[operationKey] = append(r.auth.ledger[operationKey], submittedEntry)
	return attemptNo, nil
}

// RecordDispatchOutcome is the fenced verdict transaction: the (attempt,
// token) pair must own the current attempt — a stale pair NEVER writes
// history (ErrLeaseStale); the outcome row (when the claim word is
// honest), the ledger kind, the receipt version (when the verdict is
// deciding) and the outbox receipt.created record land atomically.
func (r *dispatchMemoryRepository) RecordDispatchOutcome(_ context.Context, p actionPort.RecordDispatchOutcomeParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	operationKey := dispatchOperationKey(p.Scope, p.Operation.OperationID)
	leases := r.leases[operationKey]
	current := 0
	if len(leases) > 0 {
		current = leases[len(leases)-1].AttemptNo
	}
	if current != p.AttemptNo || len(leases) == 0 || leases[len(leases)-1].LeaseToken != p.LeaseToken {
		// A stale pair never writes history.
		return actionPort.ErrLeaseStale
	}
	outcomeKey := dispatchOutcomeKey(p.Scope, p.Operation.OperationID, p.AttemptNo)
	if len(r.outcomes[outcomeKey]) > 0 {
		// The verdict never lands twice (the append-only audit stays one row
		// per attempt).
		return actionPort.ErrOutcomeDuplicate
	}
	if p.OutcomeWord != "" {
		r.outcomes[outcomeKey] = append(r.outcomes[outcomeKey], dispatchOutcomeRow{
			AttemptNo: p.AttemptNo, OutcomeWord: p.OutcomeWord,
			CorrelationID: p.CorrelationID, GuaranteeRecorded: p.GuaranteeRecorded,
			CreatedAt: p.Now.UTC(),
		})
	}
	verdictEntry := p.LedgerEntry
	verdictEntry.At = p.Now.UTC()
	r.auth.ledger[operationKey] = append(r.auth.ledger[operationKey], verdictEntry)
	if p.ReceiptWord != "" {
		// The receipt version derives UNDER the lock as the monotonic
		// continuation of the append-only chain (the production store port
		// of the step-04 fix: the caller's pin is the expectation, the
		// lock is the truth — the first deciding verdict lands v1, the 3-7
		// retry-unlock verdict appends the NEXT version of its NEW policy
		// decision over the recorded history).
		lastVersion := int64(0)
		if rows := r.receipts[operationKey]; len(rows) > 0 {
			lastVersion = rows[len(rows)-1].ReceiptVersionNo
		}
		r.receipts[operationKey] = append(r.receipts[operationKey], dispatchReceiptRow{
			ReceiptVersionNo: lastVersion + 1, Outcome: p.ReceiptWord,
			References: p.ReceiptReferences, Details: p.ReceiptDetails, CreatedAt: p.Now.UTC(),
		})
		payload := p.OutboxPayload
		payload.ReceiptVersionNo = lastVersion + 1
		payload.Outcome = p.ReceiptWord
		r.receiptOutbox = append(r.receiptOutbox, payload)
	}
	return nil
}

// AppendDispatchBlocked parks the operation with the EXACT fence reason
// word (raced-guarded): an active lease of another consideration refuses
// with ErrLeaseRaced — the honest skip.
func (r *dispatchMemoryRepository) AppendDispatchBlocked(_ context.Context, p actionPort.AppendDispatchBlockedParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	operationKey := dispatchOperationKey(p.Scope, p.OperationID)
	if leases := r.leases[operationKey]; len(leases) > 0 && leases[len(leases)-1].LeaseExpiresAt.After(p.Now) {
		return actionPort.ErrLeaseRaced
	}
	blockedEntry := p.LedgerEntry
	blockedEntry.At = p.Now.UTC()
	r.auth.ledger[operationKey] = append(r.auth.ledger[operationKey], blockedEntry)
	return nil
}

// AppendOutboxReceiptCreated appends one receipt.created record (the
// standalone form of the verdict insert — the future 3-7 contour).
func (r *dispatchMemoryRepository) AppendOutboxReceiptCreated(_ context.Context, p actionPort.AppendOutboxReceiptCreatedParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	r.receiptOutbox = append(r.receiptOutbox, p.Payload)
	return nil
}

// ReadDispatchAttempts reads the attempts of one operation ordered by the
// monotonic attempt number (the view material).
func (r *dispatchMemoryRepository) ReadDispatchAttempts(_ context.Context, scope tenancy.Context, operationID id.UUID) ([]actionPort.DispatchAttemptRow, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	operationKey := dispatchOperationKey(scope, operationID)
	rows := []actionPort.DispatchAttemptRow{}
	for _, lease := range r.leases[operationKey] {
		row := actionPort.DispatchAttemptRow{
			AttemptNo:      lease.AttemptNo,
			LeaseToken:     lease.LeaseToken,
			LeaseExpiresAt: lease.LeaseExpiresAt,
			LeaseCreatedAt: lease.LeaseCreatedAt,
			RequestDigest:  lease.RequestDigest,
		}
		if outcomes := r.outcomes[dispatchOutcomeKey(scope, operationID, lease.AttemptNo)]; len(outcomes) > 0 {
			outcome := outcomes[len(outcomes)-1]
			row.OutcomeWord = outcome.OutcomeWord
			row.CorrelationID = outcome.CorrelationID
			row.GuaranteeRecorded = outcome.GuaranteeRecorded
			row.OutcomeCreatedAt = outcome.CreatedAt
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ReadActionReceipts reads the receipt versions of one operation ordered
// by the version number (the view material; v1 is never rewritten).
func (r *dispatchMemoryRepository) ReadActionReceipts(_ context.Context, scope tenancy.Context, operationID id.UUID) ([]actionPort.ActionReceiptRow, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	rows := []actionPort.ActionReceiptRow{}
	for _, receipt := range r.receipts[dispatchOperationKey(scope, operationID)] {
		rows = append(rows, actionPort.ActionReceiptRow{
			ReceiptVersionNo: receipt.ReceiptVersionNo, Outcome: receipt.Outcome,
			References: receipt.References, Details: receipt.Details, CreatedAt: receipt.CreatedAt,
		})
	}
	return rows, nil
}

// wireActionDispatchWorld wires the execution contour of the fixture
// world: the in-memory dispatch repository over the authorization rows
// and the synthetic mutation adapter (AG-4: production wires none) — the
// composition root hands both to WithDispatch of the REAL app service.
//
// The served capability is the VERIFIED channel word of the fixture
// dictionary (query.trunk-status — the notify.manual-only pattern: the
// channel word is the connection probe the 3-4/3-5 authorize gates
// verify, the mutation semantics ride the definition's delivery
// contract). The honest step-04 finding: a MUTATING capability
// (command.send-sms) is never probe-verifiable before the Action
// Gateway exists (the frozen 3-4 capability gate), so the z33-reused
// operations (notify.manual-only) dispatch through the channel-serving
// adapter — the allowlist stays exact (AC-4), the LIVE guarantee of the
// channel declaration is false (query.trunk-status Idempotent=false),
// and the guarantee rows of the contour ride the documented
// ScriptGuarantee override (the /fixture/action-dispatch-outcome toggle
// guarantee=1 — the 3-3 precedent).
func wireActionDispatchWorld(auth *authorizationMemoryRepository) (*dispatchMemoryRepository, *actiondispatcher.Synthetic) {
	return newDispatchMemoryRepository(auth), actiondispatcher.NewServing("query.trunk-status")
}
