package main

import (
	"context"
	"sync"
	"time"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionApp "github.com/artemiosu/staffy/internal/action/app"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// Story 4.7 (step-03): the manager order verification contour of the
// journeys world — the in-memory repository over the order world and the
// delegation of the narrow surface the transport type-asserts (the
// PrepareOrderProposal precedent). The repository mirrors the store's
// semantics: the per-case verification_no ladder, the one-active-task
// rule, the CAS epoch re-validation under the mutex — the journeys walk
// the SAME honest machine the production store runs.

// orderVerificationMemoryRepository is the in-memory implementation of
// the verification repository contract (the dispatch memory precedent).
type orderVerificationMemoryRepository struct {
	mu        sync.Mutex
	tasks     []actionDomain.OrderVerification
	events    map[int64][]actionDomain.OrderVerificationEvent
	nextNo    map[id.UUID]int64
}

func newOrderVerificationMemoryRepository() *orderVerificationMemoryRepository {
	return &orderVerificationMemoryRepository{events: map[int64][]actionDomain.OrderVerificationEvent{}, nextNo: map[id.UUID]int64{}}
}

func (r *orderVerificationMemoryRepository) eventsOf(caseID id.UUID, verificationNo int64) []actionDomain.OrderVerificationEvent {
	out := []actionDomain.OrderVerificationEvent{}
	for _, event := range r.events[verificationNo] {
		if event.CaseID == caseID {
			out = append(out, event)
		}
	}
	return out
}

func (r *orderVerificationMemoryRepository) activeOf(operationID id.UUID, now time.Time) bool {
	for _, task := range r.tasks {
		if task.OperationID != operationID {
			continue
		}
		if actionDomain.DeriveOrderVerificationState(task, r.eventsOf(task.CaseID, task.VerificationNo), now).Active() {
			return true
		}
	}
	return false
}

func (r *orderVerificationMemoryRepository) ReadOperationVerifications(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]actionDomain.OrderVerification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.OrderVerification{}
	for _, task := range r.tasks {
		if task.OperationID == operationID {
			out = append(out, task)
		}
	}
	return out, nil
}

func (r *orderVerificationMemoryRepository) ReadCaseVerifications(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]actionDomain.OrderVerification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.OrderVerification{}
	for _, task := range r.tasks {
		if task.CaseID == caseID {
			out = append(out, task)
		}
	}
	return out, nil
}

func (r *orderVerificationMemoryRepository) ReadVerificationEvents(_ context.Context, _ tenancy.Context, caseID id.UUID, verificationNo int64) ([]actionDomain.OrderVerificationEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]actionDomain.OrderVerificationEvent(nil), r.eventsOf(caseID, verificationNo)...), nil
}

func (r *orderVerificationMemoryRepository) AppendOrderVerification(_ context.Context, p actionPort.AppendOrderVerificationParams) (actionDomain.OrderVerification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activeOf(p.OperationID, p.Now) {
		return actionDomain.OrderVerification{}, actionPort.ErrVerificationActive
	}
	verificationNo := r.nextNo[p.CaseID] + 1
	r.nextNo[p.CaseID] = verificationNo
	task := actionDomain.OrderVerification{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		VerificationNo: verificationNo, OperationID: p.OperationID,
		ReceiptVersionNo: p.ReceiptVersionNo, ExternalOrderID: p.ExternalOrderID,
		CreatorID: p.CreatorID, CreatedAt: p.Now,
	}
	r.tasks = append(r.tasks, task)
	r.events[verificationNo] = append(r.events[verificationNo], actionDomain.OrderVerificationEvent{
		TenantID: task.TenantID, CaseID: task.CaseID, VerificationNo: verificationNo, EventNo: 1,
		Kind: actionDomain.OrderVerificationEventAssigned, Assignee: p.Assignee, BackupOwner: p.BackupOwner,
		VerifyDeadline: p.VerifyDeadline, DeadlineAt: p.DeadlineAt, ActorID: p.CreatorID, CreatedAt: p.Now,
	})
	return task, nil
}

func (r *orderVerificationMemoryRepository) AppendOrderVerificationEvent(_ context.Context, p actionPort.AppendOrderVerificationEventParams) (actionDomain.OrderVerificationEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var task *actionDomain.OrderVerification
	for i := range r.tasks {
		if r.tasks[i].CaseID == p.CaseID && r.tasks[i].VerificationNo == p.VerificationNo {
			task = &r.tasks[i]
			break
		}
	}
	if task == nil {
		return actionDomain.OrderVerificationEvent{}, actionPort.ErrVerificationNotFound
	}
	events := r.eventsOf(p.CaseID, p.VerificationNo)
	view := actionDomain.DeriveOrderVerificationState(*task, events, p.Now)
	if view.State != p.ExpectedState || view.AssignmentEpoch != p.ExpectedEpoch {
		return actionDomain.OrderVerificationEvent{}, actionPort.ErrVerificationRaced
	}
	event := p.Event
	event.TenantID = task.TenantID
	event.CaseID = task.CaseID
	event.VerificationNo = task.VerificationNo
	event.EventNo = int64(len(events)) + 1
	event.CreatedAt = p.Now
	r.events[p.VerificationNo] = append(r.events[p.VerificationNo], event)
	return event, nil
}

// The compile-time proof the actions fixture serves the narrow
// verification surface the transport type-asserts (the delegation over
// the active contour — the OrderProposalService precedent).
var _ actionAPI.OrderVerificationService = (*actionsFixture)(nil)

// verificationService picks the order world: the verification contour
// lives ONLY there (the committed receipt of the order action exists
// only in the order world); every other contour answers the honest
// not-configured refusal.
func (f *actionsFixture) verificationService() *actionApp.Service {
	if f.orderEnabled && f.order != nil {
		return f.order
	}
	return nil
}

// AssignOrderVerification delegates to the order world's verification
// contour.
func (f *actionsFixture) AssignOrderVerification(ctx context.Context, command actionAPI.AssignOrderVerificationCommand) (actionAPI.OrderVerificationOutcome, error) {
	f.mu.Lock()
	service := f.verificationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderVerificationOutcome{}, actionDomain.RuleError{Rule: "assign_order_verification", Reason: notConfiguredText}
	}
	return service.AssignOrderVerification(ctx, command)
}

// AcknowledgeOrderVerification delegates to the order world's
// verification contour.
func (f *actionsFixture) AcknowledgeOrderVerification(ctx context.Context, command actionAPI.AcknowledgeOrderVerificationCommand) (actionAPI.OrderVerificationOutcome, error) {
	f.mu.Lock()
	service := f.verificationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderVerificationOutcome{}, actionDomain.RuleError{Rule: "acknowledge_order_verification", Reason: notConfiguredText}
	}
	return service.AcknowledgeOrderVerification(ctx, command)
}

// ApproveOrderVerification delegates to the order world's verification
// contour (the decision fence rides the same case material the order
// world serves).
func (f *actionsFixture) ApproveOrderVerification(ctx context.Context, command actionAPI.ApproveOrderVerificationCommand) (actionAPI.OrderVerificationOutcome, error) {
	f.mu.Lock()
	service := f.verificationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderVerificationOutcome{}, actionDomain.RuleError{Rule: "approve_order_verification", Reason: notConfiguredText}
	}
	return service.ApproveOrderVerification(ctx, command)
}

// BlockOrderVerification delegates to the order world's verification
// contour.
func (f *actionsFixture) BlockOrderVerification(ctx context.Context, command actionAPI.BlockOrderVerificationCommand) (actionAPI.OrderVerificationOutcome, error) {
	f.mu.Lock()
	service := f.verificationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderVerificationOutcome{}, actionDomain.RuleError{Rule: "block_order_verification", Reason: notConfiguredText}
	}
	return service.BlockOrderVerification(ctx, command)
}

// ReassignOrderVerification delegates to the order world's verification
// contour.
func (f *actionsFixture) ReassignOrderVerification(ctx context.Context, command actionAPI.ReassignOrderVerificationCommand) (actionAPI.OrderVerificationOutcome, error) {
	f.mu.Lock()
	service := f.verificationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderVerificationOutcome{}, actionDomain.RuleError{Rule: "reassign_order_verification", Reason: notConfiguredText}
	}
	return service.ReassignOrderVerification(ctx, command)
}

// ReadOrderVerificationCaseView delegates to the order world's
// verification contour.
func (f *actionsFixture) ReadOrderVerificationCaseView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) ([]actionDomain.OrderVerificationStateView, error) {
	f.mu.Lock()
	service := f.verificationService()
	f.mu.Unlock()
	if service == nil {
		return nil, actionDomain.RuleError{Rule: "read_order_verifications", Reason: notConfiguredText}
	}
	return service.ReadOrderVerificationCaseView(ctx, actor, tenantID, caseID, now)
}

// ReadOrderVerificationLiveMaterial delegates to the order world's
// verification contour (the live derivation of the decision context).
func (f *actionsFixture) ReadOrderVerificationLiveMaterial(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (actionDomain.OrderCommitmentMaterial, error) {
	f.mu.Lock()
	service := f.verificationService()
	f.mu.Unlock()
	if service == nil {
		return actionDomain.OrderCommitmentMaterial{}, actionDomain.RuleError{Rule: "read_order_verification_material", Reason: notConfiguredText}
	}
	return service.ReadOrderVerificationLiveMaterial(ctx, actor, tenantID, caseID)
}
