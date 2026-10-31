package main

import (
	"context"
	"sync"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionApp "github.com/staffy-co/staffy/internal/action/app"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Story 4.8 (step-03): the order correction contour of the journeys
// world — the in-memory repository over the order world and the
// delegation of the narrow surface the transport type-asserts (the
// verification precedent). The repository mirrors the store's semantics:
// the per-case correction_no ladder, the one-open-correction rule, the
// CAS epoch re-validation under the mutex — the journeys walk the SAME
// honest machine the production store runs.

// orderCorrectionMemoryRepository is the in-memory implementation of the
// correction repository contract (the verification memory precedent).
type orderCorrectionMemoryRepository struct {
	mu          sync.Mutex
	corrections []actionDomain.OrderCorrection
	events      map[int64][]actionDomain.OrderCorrectionEvent
	nextNo      map[id.UUID]int64
}

func newOrderCorrectionMemoryRepository() *orderCorrectionMemoryRepository {
	return &orderCorrectionMemoryRepository{events: map[int64][]actionDomain.OrderCorrectionEvent{}, nextNo: map[id.UUID]int64{}}
}

func (r *orderCorrectionMemoryRepository) eventsOf(caseID id.UUID, correctionNo int64) []actionDomain.OrderCorrectionEvent {
	out := []actionDomain.OrderCorrectionEvent{}
	for _, event := range r.events[correctionNo] {
		if event.CaseID == caseID {
			out = append(out, event)
		}
	}
	return out
}

func (r *orderCorrectionMemoryRepository) activeOf(operationID id.UUID, now time.Time) bool {
	for _, correction := range r.corrections {
		if correction.OperationID != operationID {
			continue
		}
		if actionDomain.DeriveOrderCorrectionState(correction, r.eventsOf(correction.CaseID, correction.CorrectionNo), now).Active() {
			return true
		}
	}
	return false
}

func (r *orderCorrectionMemoryRepository) ReadOperationCorrections(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]actionDomain.OrderCorrection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.OrderCorrection{}
	for _, correction := range r.corrections {
		if correction.OperationID == operationID {
			out = append(out, correction)
		}
	}
	return out, nil
}

func (r *orderCorrectionMemoryRepository) ReadCaseCorrections(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]actionDomain.OrderCorrection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.OrderCorrection{}
	for _, correction := range r.corrections {
		if correction.CaseID == caseID {
			out = append(out, correction)
		}
	}
	return out, nil
}

func (r *orderCorrectionMemoryRepository) ReadCorrectionEvents(_ context.Context, _ tenancy.Context, caseID id.UUID, correctionNo int64) ([]actionDomain.OrderCorrectionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]actionDomain.OrderCorrectionEvent(nil), r.eventsOf(caseID, correctionNo)...), nil
}

func (r *orderCorrectionMemoryRepository) AppendOrderCorrection(_ context.Context, p actionPort.AppendOrderCorrectionParams) (actionDomain.OrderCorrection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activeOf(p.OperationID, p.Now) {
		return actionDomain.OrderCorrection{}, actionPort.ErrCorrectionActive
	}
	correctionNo := r.nextNo[p.CaseID] + 1
	r.nextNo[p.CaseID] = correctionNo
	correction := actionDomain.OrderCorrection{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		CorrectionNo: correctionNo, OperationID: p.OperationID,
		VerificationNo: p.VerificationNo, ReceiptVersionNo: p.ReceiptVersionNo,
		ExternalOrderID: p.ExternalOrderID, CreatorID: p.CreatorID, CreatedAt: p.Now,
	}
	r.corrections = append(r.corrections, correction)
	r.events[correctionNo] = append(r.events[correctionNo], actionDomain.OrderCorrectionEvent{
		TenantID: correction.TenantID, CaseID: correction.CaseID, CorrectionNo: correctionNo, EventNo: 1,
		Kind: actionDomain.OrderCorrectionEventOpened, Owner: p.Owner, CorrectDeadline: p.CorrectDeadline,
		DeadlineAt: p.DeadlineAt, Positions: p.Positions, Reason: p.Reason,
		ActorID: p.CreatorID, CreatedAt: p.Now,
	})
	return correction, nil
}

func (r *orderCorrectionMemoryRepository) AppendOrderCorrectionEvent(_ context.Context, p actionPort.AppendOrderCorrectionEventParams) (actionDomain.OrderCorrectionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var correction *actionDomain.OrderCorrection
	for i := range r.corrections {
		if r.corrections[i].CaseID == p.CaseID && r.corrections[i].CorrectionNo == p.CorrectionNo {
			correction = &r.corrections[i]
			break
		}
	}
	if correction == nil {
		return actionDomain.OrderCorrectionEvent{}, actionPort.ErrCorrectionNotFound
	}
	events := r.eventsOf(p.CaseID, p.CorrectionNo)
	view := actionDomain.DeriveOrderCorrectionState(*correction, events, p.Now)
	if view.State != p.ExpectedState || view.AssignmentEpoch != p.ExpectedEpoch {
		return actionDomain.OrderCorrectionEvent{}, actionPort.ErrCorrectionRaced
	}
	closed := false
	for _, event := range events {
		if event.Kind == actionDomain.OrderCorrectionEventReread || event.Kind == actionDomain.OrderCorrectionEventCeased {
			closed = true
		}
	}
	if closed {
		return actionDomain.OrderCorrectionEvent{}, actionPort.ErrCorrectionRaced
	}
	event := p.Event
	event.TenantID = correction.TenantID
	event.CaseID = correction.CaseID
	event.CorrectionNo = correction.CorrectionNo
	event.EventNo = int64(len(events)) + 1
	event.CreatedAt = p.Now
	r.events[p.CorrectionNo] = append(r.events[p.CorrectionNo], event)
	return event, nil
}

// The compile-time proof the actions fixture serves the narrow
// correction surface the transport type-asserts (the delegation over
// the active contour — the verification precedent).
var _ actionAPI.OrderCorrectionService = (*actionsFixture)(nil)

// correctionService picks the order world: the correction contour lives
// ONLY there (the blocked decision of the verification exists only in
// the order world); every other contour answers the honest
// not-configured refusal.
func (f *actionsFixture) correctionService() *actionApp.Service {
	if f.orderEnabled && f.order != nil {
		return f.order
	}
	return nil
}

// OpenOrderCorrection delegates to the order world's correction contour.
func (f *actionsFixture) OpenOrderCorrection(ctx context.Context, command actionAPI.OpenOrderCorrectionCommand) (actionAPI.OrderCorrectionOutcome, error) {
	f.mu.Lock()
	service := f.correctionService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderCorrectionOutcome{}, actionDomain.RuleError{Rule: "open_order_correction", Reason: notConfiguredText}
	}
	return service.OpenOrderCorrection(ctx, command)
}

// ReassignOrderCorrection delegates to the order world's correction
// contour.
func (f *actionsFixture) ReassignOrderCorrection(ctx context.Context, command actionAPI.ReassignOrderCorrectionCommand) (actionAPI.OrderCorrectionOutcome, error) {
	f.mu.Lock()
	service := f.correctionService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderCorrectionOutcome{}, actionDomain.RuleError{Rule: "reassign_order_correction", Reason: notConfiguredText}
	}
	return service.ReassignOrderCorrection(ctx, command)
}

// RecordOrderCorrectionReread delegates to the order world's correction
// contour (the version validations and the composition fence ride the
// same case material the order world serves).
func (f *actionsFixture) RecordOrderCorrectionReread(ctx context.Context, command actionAPI.RecordOrderCorrectionRereadCommand) (actionAPI.OrderCorrectionOutcome, error) {
	f.mu.Lock()
	service := f.correctionService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderCorrectionOutcome{}, actionDomain.RuleError{Rule: "record_order_correction_reread", Reason: notConfiguredText}
	}
	return service.RecordOrderCorrectionReread(ctx, command)
}

// CeaseOrderCorrection delegates to the order world's correction
// contour.
func (f *actionsFixture) CeaseOrderCorrection(ctx context.Context, command actionAPI.CeaseOrderCorrectionCommand) (actionAPI.OrderCorrectionOutcome, error) {
	f.mu.Lock()
	service := f.correctionService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderCorrectionOutcome{}, actionDomain.RuleError{Rule: "cease_order_correction", Reason: notConfiguredText}
	}
	return service.CeaseOrderCorrection(ctx, command)
}

// ReadOrderCorrectionCaseView delegates to the order world's correction
// contour.
func (f *actionsFixture) ReadOrderCorrectionCaseView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) ([]actionDomain.OrderCorrectionStateView, error) {
	f.mu.Lock()
	service := f.correctionService()
	f.mu.Unlock()
	if service == nil {
		return nil, actionDomain.RuleError{Rule: "read_order_corrections", Reason: notConfiguredText}
	}
	return service.ReadOrderCorrectionCaseView(ctx, actor, tenantID, caseID, now)
}
