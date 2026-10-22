package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// The Story 3.8 fixture: the recovery-case contour over the REAL app
// service (the read-time eligibility ladder and the LIVE-reading
// assignment of WithRecovery run the REAL domain derivations and the
// REAL gates), the in-memory recovery repository over the SAME
// authorization rows, the SAME dispatch receipts and the SAME
// reconciliation verdicts (the frozen SAFE bundle, the keyed digest
// comparator, the append-only event history — the "мир пути, не
// продуктовая персистентность" of the spec). The contour carries NO
// adapter of its own (the locality of the slice: the command makes not
// ONE provider call — the wiring hands the repository ONLY to
// WithRecovery; production and the fixture world compose identically,
// AG-4 does not refuse this one).
//
// The honest transaction semantics of the PG store are ported EXACTLY:
// the recovery advisory lock (tenant, operation) is the single shared
// auth mutex here (ONE lock guards the whole fixture world — the
// creation command is strictly sequential by construction), the existing
// case re-reads UNDER the lock decide the path (the UNCHANGED digest
// answers the idempotent return — NO second row, NO event; a CHANGED
// digest appends ONE evidence_appended row with the SAFE delta — the
// original bundle and the original receipts are never rewritten), the
// creation lands the case row with the MONOTONIC recovery_case_no
// (coalesce(max)+1 per tenant under the lock), the created event and
// the outbox recovery_case.created record — ONE locked section, ANY
// failure leaves no partial state (the checks run BEFORE the first
// mutation).

// recoveryMemoryRepository is the in-memory RecoveryRepository of the
// fixture world over the SAME locked authorization state, the SAME
// dispatch facts and the SAME reconciliation verdicts (the frozen rows
// of the contour itself live here).
type recoveryMemoryRepository struct {
	auth      *authorizationMemoryRepository
	dispatch  *dispatchMemoryRepository
	reconcile *reconcileMemoryRepository
	previews  *previewMemoryRepository

	cases  map[string]*actionPort.RecoveryCaseRow // tenant|operationID → the ONE case forever
	byID   map[string]*actionPort.RecoveryCaseRow // tenant|recoveryCaseID → the same row
	events map[string][]actionDomain.RecoveryEventRow
	outbox []actionPort.RecoveryCaseCreatedPayload
}

// newRecoveryMemoryRepository constructs the recovery-contour repository
// over the authorization rows, the dispatch receipts, the reconciliation
// verdicts and the preview pins of the fixture world.
func newRecoveryMemoryRepository(auth *authorizationMemoryRepository, dispatch *dispatchMemoryRepository,
	reconcile *reconcileMemoryRepository, previews *previewMemoryRepository) *recoveryMemoryRepository {
	return &recoveryMemoryRepository{
		auth:      auth,
		dispatch:  dispatch,
		reconcile: reconcile,
		previews:  previews,
		cases:     map[string]*actionPort.RecoveryCaseRow{},
		byID:      map[string]*actionPort.RecoveryCaseRow{},
		events:    map[string][]actionDomain.RecoveryEventRow{},
	}
}

// The compile-time proof the fixture repository satisfies the consumer
// contract of the contour (the narrow interface the app type-asserts).
var _ actionPort.RecoveryRepository = (*recoveryMemoryRepository)(nil)

// recoveryCaseKey builds the operation identity key of the tenant scope
// (the authorization key reuse — the SAME rows).
func recoveryCaseKey(scope tenancy.Context, operationID id.UUID) string {
	return scope.TenantID().String() + "|" + operationID.String()
}

// recoveryEventKey builds the case identity key of the event rows.
func recoveryEventKey(scope tenancy.Context, recoveryCaseID id.UUID) string {
	return scope.TenantID().String() + "|" + recoveryCaseID.String()
}

// pinnedObservations reads the supporting-observation pins of the EXACT
// preview version (the 2-7 version pins — the material NEVER travels).
// The nested lock order (auth.mu → previews.mu) is the fixture's only
// order — the preview repository never takes the auth lock, so the
// composition cannot deadlock.
func (r *recoveryMemoryRepository) pinnedObservations(previewID id.UUID, versionNo int64) []actionDomain.VersionPin {
	r.previews.mu.Lock()
	defer r.previews.mu.Unlock()
	pins := []actionDomain.VersionPin{}
	for _, version := range r.previews.versions[previewID.String()] {
		if version.VersionNo == versionNo {
			pins = append(pins, version.SupportingObservations...)
			break
		}
	}
	return pins
}

// lastRecoveryDispatchGuarantee walks the recorded dispatch outcomes of
// the operation backwards: the LATEST attempt with a recorded outcome
// owns the guarantee word (the PG "order by attempt_no desc limit 1").
func (r *recoveryMemoryRepository) lastRecoveryDispatchGuarantee(scope tenancy.Context, operationID id.UUID, attempts int) bool {
	for attempt := attempts; attempt >= 1; attempt-- {
		if outcomes := r.dispatch.outcomes[dispatchOutcomeKey(scope, operationID, attempt)]; len(outcomes) > 0 {
			return outcomes[len(outcomes)-1].GuaranteeRecorded
		}
	}
	return false
}

// lastRecoveryVerdict walks the recorded reconciliation verdicts of the
// operation backwards: the LATEST attempt with a recorded verdict owns
// the no_effect fact (the PG lateral join of the pending read).
func (r *recoveryMemoryRepository) lastRecoveryVerdict(scope tenancy.Context, operationID id.UUID, attempts int) string {
	for attempt := attempts; attempt >= 1; attempt-- {
		if outcomes := r.reconcile.outcomes[reconcileOutcomeKey(scope, operationID, attempt)]; len(outcomes) > 0 {
			return outcomes[len(outcomes)-1].Verdict
		}
	}
	return ""
}

// ReadRecoveryOperationFacts reads the RECORDED facts of one operation
// (read-time, NO writes, NO locks beyond the shared root): the immutable
// operation row, the append-only ledger with the derived state word, the
// attempt counters of both contours, the recorded guarantee of the last
// dispatch attempt, the no_effect unlock fact, the receipt-chain pins
// with the external identity and the supporting-observation pins of the
// EXACT preview version. An unknown operation answers the honest 404.
func (r *recoveryMemoryRepository) ReadRecoveryOperationFacts(_ context.Context, scope tenancy.Context, operationID id.UUID) (actionPort.RecoveryOperationFacts, error) {
	if !scope.Valid() {
		return actionPort.RecoveryOperationFacts{}, tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	operationKey := recoveryCaseKey(scope, operationID)
	operation, ok := r.auth.operations[operationKey]
	if !ok {
		return actionPort.RecoveryOperationFacts{}, actionPort.ErrRecoveryNotFound
	}
	ledger := append([]actionDomain.OperationLedgerEntry(nil), r.auth.ledger[operationKey]...)
	state, err := actionDomain.DeriveOperationState(ledger)
	if err != nil {
		return actionPort.RecoveryOperationFacts{}, err
	}
	facts := actionPort.RecoveryOperationFacts{
		Operation:             operation,
		Ledger:                ledger,
		StateWord:             state,
		DispatchAttemptCount:  len(r.dispatch.leases[operationKey]),
		ReconcileAttemptCount: len(r.reconcile.leases[operationKey]),
	}
	facts.LastOutcomeGuarantee = r.lastRecoveryDispatchGuarantee(scope, operationID, facts.DispatchAttemptCount)
	if verdict := r.lastRecoveryVerdict(scope, operationID, facts.ReconcileAttemptCount); verdict == actionDomain.ReconciliationVerdictNoEffect {
		facts.NoEffectProven = true
	}
	receipts := r.dispatch.receipts[operationKey]
	facts.ReceiptVersionsCount = int64(len(receipts))
	if len(receipts) > 0 {
		latest := receipts[len(receipts)-1]
		facts.LatestReceiptVersionNo = latest.ReceiptVersionNo
		facts.LatestReceiptOutcome = latest.Outcome
		facts.LatestExternalObjectID = latest.References.ExternalObjectID
		facts.LatestExternalObjectVersion = latest.References.ExternalObjectVersion
	}
	facts.Observations = r.pinnedObservations(operation.PreviewID, operation.PreviewVersionNo)
	if facts.Observations == nil {
		facts.Observations = []actionDomain.VersionPin{}
	}
	return facts, nil
}

// recoveryCaseWithEvents copies the row with its append-only history.
func (r *recoveryMemoryRepository) recoveryCaseWithEvents(scope tenancy.Context, row *actionPort.RecoveryCaseRow) actionPort.RecoveryCaseRow {
	out := *row
	out.Events = append([]actionDomain.RecoveryEventRow(nil), r.events[recoveryEventKey(scope, row.RecoveryCaseID)]...)
	return out
}

// ReadRecoveryCaseByOperation reads the case of one operation with its
// event history (the idempotent pre-check of the command; the creation
// transaction re-reads UNDER the lock — the belt over braces). No case
// answers the honest 404.
func (r *recoveryMemoryRepository) ReadRecoveryCaseByOperation(_ context.Context, scope tenancy.Context, operationID id.UUID) (actionPort.RecoveryCaseRow, error) {
	if !scope.Valid() {
		return actionPort.RecoveryCaseRow{}, tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	row, ok := r.cases[recoveryCaseKey(scope, operationID)]
	if !ok {
		return actionPort.RecoveryCaseRow{}, actionPort.ErrRecoveryNotFound
	}
	return r.recoveryCaseWithEvents(scope, row), nil
}

// ReadRecoveryCase reads one case by its identity with its event history
// (the card view material). Unknown answers the honest 404.
func (r *recoveryMemoryRepository) ReadRecoveryCase(_ context.Context, scope tenancy.Context, recoveryCaseID id.UUID) (actionPort.RecoveryCaseRow, error) {
	if !scope.Valid() {
		return actionPort.RecoveryCaseRow{}, tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	row, ok := r.byID[recoveryEventKey(scope, recoveryCaseID)]
	if !ok {
		return actionPort.RecoveryCaseRow{}, actionPort.ErrRecoveryNotFound
	}
	return r.recoveryCaseWithEvents(scope, row), nil
}

// ReadRecoveryCases reads the cases of the tenant ordered by the creation
// moment descending (the No tiebreak), bounded by the limit (the panel
// list; limit < 1 reads as the domain bound).
func (r *recoveryMemoryRepository) ReadRecoveryCases(_ context.Context, scope tenancy.Context, limit int) ([]actionPort.RecoveryCaseRow, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	if limit < 1 {
		limit = actionPort.RecoveryPanelLimit
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	rows := []*actionPort.RecoveryCaseRow{}
	for key, row := range r.cases {
		if len(key) < 36 || key[:36] != scope.TenantID().String() {
			continue
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].CreatedAt.After(rows[j].CreatedAt)
		}
		return rows[i].RecoveryCaseNo > rows[j].RecoveryCaseNo
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]actionPort.RecoveryCaseRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, r.recoveryCaseWithEvents(scope, row))
	}
	return out, nil
}

// CreateRecoveryCase is the ONE creation transaction of the contour under
// the recovery advisory lock (the shared auth mutex of the fixture
// world): the existing case re-read decides the path — the UNCHANGED
// digest answers the IDEMPOTENT return (NO second row, NO event), a
// CHANGED digest appends ONE evidence_appended row with the SAFE delta
// (the original bundle and the original receipts are NEVER rewritten),
// no case CREATES the row with the monotonic recovery_case_no
// (coalesce(max)+1 under the lock), the created event and the outbox
// recovery_case.created record — ONE locked section, ANY failure leaves
// no partial state.
func (r *recoveryMemoryRepository) CreateRecoveryCase(_ context.Context, p actionPort.CreateRecoveryCaseParams) (actionPort.CreateRecoveryCaseResult, error) {
	if !p.Scope.Valid() {
		return actionPort.CreateRecoveryCaseResult{}, tenancy.ErrMissingContext
	}
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	tenantID := p.Scope.TenantID()
	operationKey := recoveryCaseKey(p.Scope, p.Operation.OperationID)
	if existing, ok := r.cases[operationKey]; ok {
		// The repeat comparator UNDER the lock (the 3-6/3-7 discipline:
		// every path with a recorded trace compares BEFORE any write).
		decision, cmpErr := actionDomain.DeriveRecoveryRepeatDecision(existing.BundleDigest, p.BundleDigest)
		if cmpErr != nil {
			return actionPort.CreateRecoveryCaseResult{}, cmpErr
		}
		if decision == actionDomain.RecoveryRepeatIdempotent {
			return actionPort.CreateRecoveryCaseResult{
				Case:     r.recoveryCaseWithEvents(p.Scope, existing),
				Decision: actionPort.RecoveryDecisionIdempotent,
			}, nil
		}
		// The evidence append: ONE row with the SAFE delta, the monotonic
		// event number under the case history (the original bundle row is
		// never rewritten — the append-only reference is the whole update).
		eventKey := recoveryEventKey(p.Scope, existing.RecoveryCaseID)
		eventNo := int64(1)
		if rows := r.events[eventKey]; len(rows) > 0 {
			eventNo = rows[len(rows)-1].EventNo + 1
		}
		delta := actionDomain.DeriveRecoveryEvidenceDelta(existing.Bundle, p.Bundle)
		r.events[eventKey] = append(r.events[eventKey], actionDomain.RecoveryEventRow{
			EventNo:   eventNo,
			Kind:      actionDomain.RecoveryEventEvidenceAppended,
			Details:   delta,
			ActorID:   p.ActorID,
			CreatedAt: p.Now.UTC().Truncate(time.Microsecond),
		})
		return actionPort.CreateRecoveryCaseResult{
			Case:     r.recoveryCaseWithEvents(p.Scope, existing),
			Decision: actionPort.RecoveryDecisionEvidence,
		}, nil
	}
	// The creation path: the monotonic case number UNDER the lock (the
	// caseNo precedent — coalesce(max)+1 per tenant), the case row, the
	// created event and the outbox recovery_case.created record.
	caseNo := int64(0)
	for key, row := range r.cases {
		if len(key) < 36 || key[:36] != tenantID.String() {
			continue
		}
		if row.RecoveryCaseNo > caseNo {
			caseNo = row.RecoveryCaseNo
		}
	}
	caseNo++
	now := p.Now.UTC().Truncate(time.Microsecond)
	row := actionPort.RecoveryCaseRow{
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
		CreatedAt:      now,
	}
	createdEvent := actionDomain.RecoveryEventRow{
		EventNo:   1,
		Kind:      actionDomain.RecoveryEventCreated,
		Details:   fmt.Sprintf("Recovery Case создан: триггер %s.", p.Trigger),
		ActorID:   p.ActorID,
		CreatedAt: now,
	}
	r.cases[operationKey] = &row
	r.byID[recoveryEventKey(p.Scope, p.RecoveryCaseID)] = &row
	r.events[recoveryEventKey(p.Scope, p.RecoveryCaseID)] = append([]actionDomain.RecoveryEventRow(nil), createdEvent)
	r.outbox = append(r.outbox, actionPort.RecoveryCaseCreatedPayload{
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
		DeadlineAt:     row.DeadlineAt,
		CreatedAt:      now,
	})
	return actionPort.CreateRecoveryCaseResult{
		Case:     r.recoveryCaseWithEvents(p.Scope, &row),
		Decision: actionPort.RecoveryDecisionCreated,
	}, nil
}

// ---------------------------------------------------------------------------
// The Story 3.8 delegation of the actions fixture: the recovery contour
// lives on the SAME world service (WithRecovery extends the one service —
// the read-time ladder and the LIVE-reading assignment of the REAL app
// answer through the fixture path).
// ---------------------------------------------------------------------------

// The compile-time proof the fixture serves the narrow recovery surface
// the transport type-asserts.
var _ actionAPI.RecoveryService = (*actionsFixture)(nil)

// CreateRecoveryCase delegates to the active authorization contour (the
// REAL creation ladder runs there — the recorded facts read, the
// existing-case comparator BEFORE the eligibility gate, the honest
// refusal word of the exact gas, the LIVE-reading assignment, the ONE
// creation transaction); the contours without the repositories answer
// the honest refusal.
func (f *actionsFixture) CreateRecoveryCase(ctx context.Context, command actionAPI.RecoveryCreateCommand) (actionAPI.RecoveryCreateResultView, error) {
	f.mu.Lock()
	service := f.authorizationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.RecoveryCreateResultView{}, actionDomain.RuleError{Rule: "create_recovery_case", Reason: authorizationNotConfiguredText}
	}
	recovery, ok := service.(actionAPI.RecoveryService)
	if !ok {
		return actionAPI.RecoveryCreateResultView{}, actionDomain.RuleError{Rule: "create_recovery_case", Reason: authorizationNotConfiguredText}
	}
	return recovery.CreateRecoveryCase(ctx, command)
}

// ReadRecoveryCase delegates to the active authorization contour (the
// frozen SAFE bundle with the LIVING receipt chain derived at the read —
// AC-10: the card is always fresh).
func (f *actionsFixture) ReadRecoveryCase(ctx context.Context, actor domain.Principal, tenantID, recoveryCaseID id.UUID) (actionAPI.RecoveryCaseView, error) {
	f.mu.Lock()
	service := f.authorizationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.RecoveryCaseView{}, actionDomain.RuleError{Rule: "read_recovery_case", Reason: authorizationNotConfiguredText}
	}
	recovery, ok := service.(actionAPI.RecoveryService)
	if !ok {
		return actionAPI.RecoveryCaseView{}, actionDomain.RuleError{Rule: "read_recovery_case", Reason: authorizationNotConfiguredText}
	}
	return recovery.ReadRecoveryCase(ctx, actor, tenantID, recoveryCaseID)
}

// ReadRecoveryCases delegates to the active authorization contour (the
// bounded panel list ordered by the creation moment descending).
func (f *actionsFixture) ReadRecoveryCases(ctx context.Context, actor domain.Principal, tenantID id.UUID) ([]actionAPI.RecoveryCaseView, error) {
	f.mu.Lock()
	service := f.authorizationService()
	f.mu.Unlock()
	if service == nil {
		return nil, actionDomain.RuleError{Rule: "read_recovery_cases", Reason: authorizationNotConfiguredText}
	}
	recovery, ok := service.(actionAPI.RecoveryService)
	if !ok {
		return nil, actionDomain.RuleError{Rule: "read_recovery_cases", Reason: authorizationNotConfiguredText}
	}
	return recovery.ReadRecoveryCases(ctx, actor, tenantID)
}
