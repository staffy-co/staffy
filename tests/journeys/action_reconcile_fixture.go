package main

import (
	"context"
	"sort"
	"strings"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	"github.com/staffy-co/staffy/internal/adapters/reconciliationsyn"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// The Story 3.7 (step-03) fixture: the reconciliation contour over the
// REAL app service (the light read-only fence ladder of WithReconcile
// runs the REAL domain derivations and the REAL gates), the in-memory
// reconciliation repository over the SAME authorization rows and the
// SAME dispatch receipts (the append-only ledger, the receipt version
// chain — the "мир пути, не продуктовая персистентность" of the spec)
// and the synthetic READ-ONLY adapter with its /fixture/reconcile*
// toggles (the scriptable verdict / correlation / evidence / failure
// words — the action-dispatch precedent of 3-6 1:1). Production wires
// NO adapter (AG-4) — this package is the test contour the production
// binary never imports.
//
// The honest fencing semantics of the PG store are ported EXACTLY: the
// monotonic attempt derivation under the lock (the active twin refuses
// with the racing signal — the maximum ONE active chain), the bounded
// attempt limit (the loyal CHECK backstop of the store speaks the stale
// word), the fenced verdict ((attempt, token) ownership — a stale pair
// NEVER writes history, a duplicate outcome NEVER lands twice) and the
// TERMINAL LOCK inside the verdict commit (the re-read of the live last
// receipt version: an append over the last unknown passes, the
// same-terminal confirmation skips silently, a conflicting terminal
// refuses with the integrity alarm); the write-ahead lease and the
// ledger reconciling entry land atomically under the single mutex (the
// checks run BEFORE the first mutation, the failure paths leave no
// partial state).

// reconcileLeaseRow is one write-ahead query lease row of the fixture
// world (the keyed identities alone — the raw material NEVER enters).
type reconcileLeaseRow struct {
	AttemptNo              int
	LeaseToken             string
	ReconciliationWord     string
	ExternalIDempotencyKey string
	RequestDigest          string
	CorrelationID          string
	LeaseExpiresAt         time.Time
	NextEligibleAt         time.Time
	LeaseCreatedAt         time.Time
}

// reconcileOutcomeRow is one recorded verdict row of the fixture world
// (the attempt ALWAYS records its guarded truth).
type reconcileOutcomeRow struct {
	AttemptNo        int
	Verdict          string
	CorrelationID    string
	ExternalObjectID string
	CreatedAt        time.Time
}

// reconcileMemoryRepository is the in-memory ReconciliationRepository of
// the fixture world over the SAME locked authorization state and the
// SAME dispatch rows (the advisory lock of the PG store is the single
// mutex here — one lock guards the whole fixture world, the reconcile
// command is strictly sequential by construction).
type reconcileMemoryRepository struct {
	auth     *authorizationMemoryRepository
	dispatch *dispatchMemoryRepository

	leases   map[string][]reconcileLeaseRow   // tenant|operationID → attempts asc
	outcomes map[string][]reconcileOutcomeRow // tenant|operationID|attemptNo
}

// newReconcileMemoryRepository constructs the reconciliation-contour
// repository over the authorization rows and the dispatch facts of the
// fixture world (the receipt versions are ONE chain — the dispatch
// repository owns the rows, the verdicts append through it).
func newReconcileMemoryRepository(auth *authorizationMemoryRepository, dispatch *dispatchMemoryRepository) *reconcileMemoryRepository {
	return &reconcileMemoryRepository{
		auth:     auth,
		dispatch: dispatch,
		leases:   map[string][]reconcileLeaseRow{},
		outcomes: map[string][]reconcileOutcomeRow{},
	}
}

// The compile-time proof the fixture repository satisfies the consumer
// contract of the contour (the narrow interface the app type-asserts).
var _ actionPort.ReconciliationRepository = (*reconcileMemoryRepository)(nil)

// reconcileOutcomeKey builds the attempt-scope key of the verdict rows.
func reconcileOutcomeKey(scope tenancy.Context, operationID id.UUID, attemptNo int) string {
	return scope.TenantID().String() + "|" + operationID.String() + "|" + itoa(attemptNo)
}

// classifyPendingReconciliationTarget ports the PG classification EXACTLY
// (the ONE reconciler of decisions — every target is considered, not
// every target is queried): decided — the state left unknown OR the last
// receipt is no longer unknown (the at-least-once repeat is the honest
// skip); racing — the active lease of another consideration; exhausted —
// the three attempts are spent; waiting — the backoff window is active;
// fresh — the query may run.
func classifyPendingReconciliationTarget(state, lastReceiptOutcome string,
	attemptCount int, lastLeaseActive bool, nextEligibleAt, now time.Time) string {
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

// PendingReconciliationTargets reads the pending targets of the tenant
// over the SAME rows: every operation with an unknown receipt version
// (the at-least-once anchor — a decided operation honestly lands in the
// decided classification and speaks its skip) joined with the immutable
// operation, the append-only ledger (the state derives), the last
// receipt facts (the terminal-lock input), the dispatch facts the query
// needs (the FIRST lease digest/key pair, the last dispatch correlation)
// and the reconciliation attempt facts. The batch is bounded by the
// reconcile drain limit.
func (r *reconcileMemoryRepository) PendingReconciliationTargets(_ context.Context, scope tenancy.Context, now time.Time, limit int) ([]actionPort.PendingReconciliationTarget, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	if limit < 1 {
		limit = 1
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	type candidateUnit struct {
		operation actionDomain.Operation
	}
	units := []candidateUnit{}
	for key, operation := range r.auth.operations {
		if !strings.HasPrefix(key, scope.TenantID().String()+"|") {
			continue
		}
		operationKey := dispatchOperationKey(scope, operation.OperationID)
		hasUnknown := false
		for _, receipt := range r.dispatch.receipts[operationKey] {
			if receipt.Outcome == actionDomain.ReceiptOutcomeUnknown {
				hasUnknown = true
			}
		}
		if hasUnknown {
			units = append(units, candidateUnit{operation: operation})
		}
	}
	sort.Slice(units, func(i, j int) bool {
		if units[i].operation.CreatedAt.Equal(units[j].operation.CreatedAt) {
			return units[i].operation.OperationID.String() < units[j].operation.OperationID.String()
		}
		return units[i].operation.CreatedAt.Before(units[j].operation.CreatedAt)
	})
	if len(units) > limit {
		units = units[:limit]
	}
	targets := []actionPort.PendingReconciliationTarget{}
	for _, unit := range units {
		operationKey := dispatchOperationKey(scope, unit.operation.OperationID)
		ledger := append([]actionDomain.OperationLedgerEntry(nil), r.auth.ledger[operationKey]...)
		state, err := actionDomain.DeriveOperationState(ledger)
		if err != nil {
			return nil, err
		}
		target := actionPort.PendingReconciliationTarget{
			Operation: unit.operation,
			Ledger:    ledger,
			StateWord: state,
		}
		leases := r.leases[operationKey]
		target.ReconcileAttemptCount = len(leases)
		if len(leases) > 0 {
			last := leases[len(leases)-1]
			target.LastReconcileAttemptNo = last.AttemptNo
			target.LastLeaseToken = last.LeaseToken
			target.LastLeaseExpiresAt = last.LeaseExpiresAt
			target.LastLeaseActive = last.LeaseExpiresAt.After(now)
			target.NextEligibleAt = last.NextEligibleAt
		}
		if len(r.dispatch.leases[operationKey]) > 0 {
			first := r.dispatch.leases[operationKey][0]
			target.RequestDigest = first.RequestDigest
			target.ExternalIDempotencyKey = first.ExternalIDempotencyKey
		}
		// The LAST dispatch outcome correlation: the highest attempt with a
		// recorded outcome, its last row (the provider's own trace).
		for attempt := len(r.dispatch.leases[operationKey]); attempt >= 1; attempt-- {
			if outcomes := r.dispatch.outcomes[dispatchOutcomeKey(scope, unit.operation.OperationID, attempt)]; len(outcomes) > 0 {
				target.LastDispatchCorrelation = outcomes[len(outcomes)-1].CorrelationID
				break
			}
		}
		// The LAST reconciliation verdict: the highest attempt with an
		// outcome row, its last row.
		for attempt := len(leases); attempt >= 1; attempt-- {
			if outcomes := r.outcomes[reconcileOutcomeKey(scope, unit.operation.OperationID, attempt)]; len(outcomes) > 0 {
				target.LastReconciliationVerdict = outcomes[len(outcomes)-1].Verdict
				break
			}
		}
		if receipts := r.dispatch.receipts[operationKey]; len(receipts) > 0 {
			last := receipts[len(receipts)-1]
			target.LastReceiptVersionNo = last.ReceiptVersionNo
			target.LastReceiptOutcome = last.Outcome
		}
		target.Classification = classifyPendingReconciliationTarget(state, target.LastReceiptOutcome,
			len(leases), target.LastLeaseActive, target.NextEligibleAt, now)
		targets = append(targets, target)
	}
	return targets, nil
}

// AcquireReconciliationLease is the WRITE-AHEAD transaction of the query
// right (persist-before-emit — the read contour mirrors the send
// contour): under the lock the monotonic attempt derives (bounded by
// MaxReconciliationAttempts), then the lease row and the ledger
// reconciling entry land atomically (the operation STAYS unknown —
// reconciling is NOT state-bearing). An active lease of another
// consideration refuses with the racing signal; the bound refusal
// answers the stale word. The signature carries ZERO adapters.
func (r *reconcileMemoryRepository) AcquireReconciliationLease(_ context.Context, p actionPort.AcquireReconciliationLeaseParams) (int, error) {
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
		return 0, actionPort.ErrReconciliationLeaseRaced
	}
	attemptNo := len(leases) + 1
	if attemptNo > actionDomain.MaxReconciliationAttempts {
		// The bounded limit — the store CHECK backstop speaks the same word.
		return 0, actionPort.ErrReconciliationLeaseStale
	}
	reconcilingEntry := p.LedgerEntry
	reconcilingEntry.At = p.Now.UTC()
	r.leases[operationKey] = append(leases, reconcileLeaseRow{
		AttemptNo: attemptNo, LeaseToken: p.LeaseToken,
		ReconciliationWord:     p.ReconciliationWord,
		ExternalIDempotencyKey: p.ExternalIDempotencyKey, RequestDigest: p.RequestDigest,
		CorrelationID:  p.CorrelationID,
		LeaseExpiresAt: p.LeaseExpiresAt.UTC(), NextEligibleAt: p.NextEligibleAt.UTC(),
		LeaseCreatedAt: p.Now.UTC(),
	})
	r.auth.ledger[operationKey] = append(r.auth.ledger[operationKey], reconcilingEntry)
	return attemptNo, nil
}

// RecordReconciliationVerdict is the FENCED verdict transaction: the
// (attempt, token) pair must own the CURRENT attempt — a stale pair
// NEVER writes history. Inside ONE commit: the outcome row of the
// NORMALIZED verdict (the attempt ALWAYS records its guarded truth),
// the ledger entry (ONLY the state-bearing verdicts — committed /
// no_effect; in_progress / no_answer write NOTHING, unknown stays), the
// receipt v2+ with the outbox receipt.created record (ONLY the terminal
// claims that PASS the receipt terminal lock: an append over the last
// unknown passes, the same-terminal confirmation skips silently, a
// conflicting terminal refuses with the integrity alarm — never an
// auto-replacement).
func (r *reconcileMemoryRepository) RecordReconciliationVerdict(_ context.Context, p actionPort.RecordReconciliationVerdictParams) error {
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
		return actionPort.ErrReconciliationLeaseStale
	}
	outcomeKey := reconcileOutcomeKey(p.Scope, p.Operation.OperationID, p.AttemptNo)
	if len(r.outcomes[outcomeKey]) > 0 {
		// The verdict never lands twice (the append-only audit stays one row
		// per attempt).
		return actionPort.ErrReconciliationLeaseStale
	}
	r.outcomes[outcomeKey] = append(r.outcomes[outcomeKey], reconcileOutcomeRow{
		AttemptNo: p.AttemptNo, Verdict: p.Verdict,
		CorrelationID: p.CorrelationID, ExternalObjectID: p.Evidence.ExternalObjectID,
		CreatedAt: p.Now.UTC(),
	})
	if p.LedgerWord != "" {
		verdictEntry := actionDomain.OperationLedgerEntry{
			Kind:    p.LedgerWord,
			Details: actionDomain.ReconciliationVerdictText(p.Verdict),
			At:      p.Now.UTC(),
		}
		entryNo := int64(1)
		for _, entry := range r.auth.ledger[operationKey] {
			if entry.EntryNo >= entryNo {
				entryNo = entry.EntryNo + 1
			}
		}
		verdictEntry.EntryNo = entryNo
		r.auth.ledger[operationKey] = append(r.auth.ledger[operationKey], verdictEntry)
	}
	if p.ReceiptWord != "" {
		// The TERMINAL LOCK inside the commit: the live last receipt version
		// re-reads under the lock (the belt over braces).
		receipts := r.dispatch.receipts[operationKey]
		if len(receipts) == 0 {
			// A terminal claim without ANY receipt version is the corruption
			// of the anchor — the integrity refusal, never a v1 invention
			// inside the reconciliation contour.
			return actionPort.ErrTerminalConflict
		}
		last := receipts[len(receipts)-1]
		decision, guardErr := actionDomain.GuardReceiptAppend(last.Outcome, p.ReceiptWord)
		if guardErr != nil {
			return guardErr
		}
		switch decision {
		case actionDomain.ReceiptAppendAppend:
			newVersion := last.ReceiptVersionNo + 1
			r.dispatch.receipts[operationKey] = append(receipts, dispatchReceiptRow{
				ReceiptVersionNo: newVersion, Outcome: p.ReceiptWord,
				References: p.ReceiptReferences, Details: p.ReceiptDetails, CreatedAt: p.Now.UTC(),
			})
			// The outbox record rides the SAME commit with the payload patched
			// by the derived version and the claimed outcome (the store-derived
			// truth — the payload travels BY VALUE).
			payload := p.OutboxPayload
			payload.ReceiptVersionNo = newVersion
			payload.Outcome = p.ReceiptWord
			r.dispatch.receiptOutbox = append(r.dispatch.receiptOutbox, payload)
		case actionDomain.ReceiptAppendSkip:
			// The idempotent confirmation: the terminal outcome is already
			// recorded with the SAME word — NO new version, NO outbox record
			// (the verdict row above still lands); the silent skip.
		case actionDomain.ReceiptAppendAlarm:
			// The integrity conflict of the terminal outcomes: the append is
			// forbidden — the human review owns the resolution (the whole
			// verdict rolls back).
			return actionPort.ErrTerminalConflict
		}
	}
	return nil
}

// ReadReconciliationAttempts reads the reconciliation attempts of one
// operation ordered by the monotonic attempt number (the view material).
func (r *reconcileMemoryRepository) ReadReconciliationAttempts(_ context.Context, scope tenancy.Context, operationID id.UUID) ([]actionPort.ReconciliationAttemptRow, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	operationKey := dispatchOperationKey(scope, operationID)
	rows := []actionPort.ReconciliationAttemptRow{}
	for _, lease := range r.leases[operationKey] {
		row := actionPort.ReconciliationAttemptRow{
			AttemptNo:          lease.AttemptNo,
			LeaseToken:         lease.LeaseToken,
			LeaseExpiresAt:     lease.LeaseExpiresAt,
			NextEligibleAt:     lease.NextEligibleAt,
			LeaseCreatedAt:     lease.LeaseCreatedAt,
			ReconciliationWord: lease.ReconciliationWord,
		}
		if outcomes := r.outcomes[reconcileOutcomeKey(scope, operationID, lease.AttemptNo)]; len(outcomes) > 0 {
			outcome := outcomes[len(outcomes)-1]
			row.OutcomeVerdict = outcome.Verdict
			row.CorrelationID = outcome.CorrelationID
			row.ExternalObjectID = outcome.ExternalObjectID
			row.OutcomeCreatedAt = outcome.CreatedAt
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// LastReconciliationNoEffect reads the recorded no_effect verdict flag of
// one operation (the retry-unlock input of the dispatch pending
// classification — the PROVEN absence of the external effect replaces
// the guarantee role).
func (r *reconcileMemoryRepository) LastReconciliationNoEffect(_ context.Context, scope tenancy.Context, operationID id.UUID) (bool, error) {
	if !scope.Valid() {
		return false, tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	operationKey := dispatchOperationKey(scope, operationID)
	for attempt := len(r.leases[operationKey]); attempt >= 1; attempt-- {
		if outcomes := r.outcomes[reconcileOutcomeKey(scope, operationID, attempt)]; len(outcomes) > 0 {
			return outcomes[len(outcomes)-1].Verdict == actionDomain.ReconciliationVerdictNoEffect, nil
		}
	}
	return false, nil
}

// lastReconciliationNoEffectLocked is the retry-unlock reader handed to
// the dispatch repository by the wiring: the CALLER owns the shared auth
// mutex (the fixture composition synchronises on that ONE root — this
// locked reader never takes a lock of its own), it walks the
// reconciliation outcomes of the operation backwards and answers whether
// the LAST verdict is the proven absence.
func (r *reconcileMemoryRepository) lastReconciliationNoEffectLocked(scope tenancy.Context, operationID id.UUID) bool {
	operationKey := dispatchOperationKey(scope, operationID)
	for attempt := len(r.leases[operationKey]); attempt >= 1; attempt-- {
		if outcomes := r.outcomes[reconcileOutcomeKey(scope, operationID, attempt)]; len(outcomes) > 0 {
			return outcomes[len(outcomes)-1].Verdict == actionDomain.ReconciliationVerdictNoEffect
		}
	}
	return false
}

// wireActionReconcileWorld wires the reconciliation contour of the
// fixture world: the in-memory reconciliation repository over the SAME
// authorization rows and the SAME dispatch receipts and the synthetic
// READ-ONLY adapter (AG-4: production wires none) — the composition root
// hands both to WithReconcile of the REAL app service.
//
// The served capability is the VERIFIED channel word of the fixture
// dictionary (query.trunk-status — the notify.manual-only pattern of the
// 3-6 dispatch world: the channel word is the connection probe the
// authorize gates verify, the reconciliation semantics ride the
// definition's delivery contract and the connector's LIVE declaration
// word). The app allowlist gate cross-checks the served word BEFORE the
// lease and the query (FENCE 5).
func wireActionReconcileWorld(auth *authorizationMemoryRepository, dispatch *dispatchMemoryRepository) (*reconcileMemoryRepository, *reconciliationsyn.Synthetic) {
	repo := newReconcileMemoryRepository(auth, dispatch)
	// The 3-7 retry-unlock bridge: the dispatch pending classification
	// reads the recorded no_effect verdicts of THIS repository (the
	// production lateral join port — the locked reader, the caller owns
	// the shared auth mutex).
	dispatch.reconcileNoEffectLocked = repo.lastReconciliationNoEffectLocked
	return repo, reconciliationsyn.NewServing("query.trunk-status")
}

// ---------------------------------------------------------------------------
// The Story 3.7 delegation of the actions fixture: the reconciliation
// contour lives on the SAME world service (WithReconcile extends the one
// service — the light read-only fence ladder of the REAL app answers
// through the fixture path).
// ---------------------------------------------------------------------------

// The compile-time proof the fixture serves the narrow reconcile surface
// the transport type-asserts.
var _ actionAPI.ReconcileService = (*actionsFixture)(nil)

// ReconcileOperations delegates to the active authorization contour (the
// REAL light read-only fence ladder runs there — the write-ahead query
// lease, the fenced verdict with the terminal lock, the honest report of
// every considered target); the contours without the repositories answer
// the honest refusal.
func (f *actionsFixture) ReconcileOperations(ctx context.Context, command actionAPI.ReconcileOperationsCommand) (actionAPI.ReconcileResultView, error) {
	f.mu.Lock()
	service := f.authorizationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.ReconcileResultView{}, actionDomain.RuleError{Rule: "reconcile_operations", Reason: authorizationNotConfiguredText}
	}
	reconcile, ok := service.(actionAPI.ReconcileService)
	if !ok {
		return actionAPI.ReconcileResultView{}, actionDomain.RuleError{Rule: "reconcile_operations", Reason: authorizationNotConfiguredText}
	}
	return reconcile.ReconcileOperations(ctx, command)
}
