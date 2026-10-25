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

// Story 4.9 (step-04): the order status contour of the journeys world
// — the in-memory repository of the Supplier Order Fact Receipt over
// the order world and the delegation of the narrow surface the
// transport type-asserts (the correction precedent). The repository
// mirrors the store's semantics: the per-case fact_no ladder and the
// UNIQUE full fact key (the idempotent replay BY KEY — the raced twin
// refuses honestly, the app re-reads the saved fact) — the journeys
// walk the SAME honest machine the production store runs.

// orderSupplierFactMemoryRepository is the in-memory implementation of
// the supplier order fact repository contract (the correction memory
// precedent).
type orderSupplierFactMemoryRepository struct {
        mu        sync.Mutex
        facts     []actionDomain.OrderSupplierFact
        nextNo    map[id.UUID]int64
}

func newOrderSupplierFactMemoryRepository() *orderSupplierFactMemoryRepository {
        return &orderSupplierFactMemoryRepository{nextNo: map[id.UUID]int64{}}
}

func (r *orderSupplierFactMemoryRepository) factsOf(anchor id.UUID, byCase bool) []actionDomain.OrderSupplierFact {
        out := []actionDomain.OrderSupplierFact{}
        for _, fact := range r.facts {
                if (byCase && fact.CaseID == anchor) || (!byCase && fact.OperationID == anchor) {
                        out = append(out, fact)
                }
        }
        return out
}

func (r *orderSupplierFactMemoryRepository) ReadCaseSupplierFacts(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]actionDomain.OrderSupplierFact, error) {
        r.mu.Lock()
        defer r.mu.Unlock()
        return r.factsOf(caseID, true), nil
}

func (r *orderSupplierFactMemoryRepository) ReadOperationSupplierFacts(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]actionDomain.OrderSupplierFact, error) {
        r.mu.Lock()
        defer r.mu.Unlock()
        return r.factsOf(operationID, false), nil
}

func (r *orderSupplierFactMemoryRepository) AppendOrderSupplierFact(_ context.Context, p actionPort.AppendOrderSupplierFactParams) (actionDomain.OrderSupplierFact, error) {
        r.mu.Lock()
        defer r.mu.Unlock()
        // The UNIQUE full fact key (the idempotency BY KEY — the raced replay
        // of the same command refuses with the honest duplicate signal, the
        // app re-reads the saved fact).
        for _, fact := range r.facts {
                if fact.OperationID == p.OperationID && fact.Kind == p.Kind && fact.SupplierOrderID == p.SupplierOrderID {
                        return actionDomain.OrderSupplierFact{}, actionPort.ErrOrderSupplierFactDuplicate
                }
        }
        factNo := r.nextNo[p.CaseID] + 1
        r.nextNo[p.CaseID] = factNo
        fact := actionDomain.OrderSupplierFact{
                TenantID: p.Scope.TenantID(), CaseID: p.CaseID, WorkItemID: p.WorkItemID,
                FactNo: factNo, OperationID: p.OperationID, Kind: p.Kind,
                SupplierOrderID: p.SupplierOrderID, HappenedAt: p.HappenedAt,
                ActorID: p.ActorID, RecordedAt: p.Now,
        }
        r.facts = append(r.facts, fact)
        return fact, nil
}

// statusService picks the order world: the Story 4.6 composition when
// its toggle chose it; every other contour answers the honest
// not-configured refusal (the status projection lives ONLY in the order
// world — the same routing as the correction surface).
func (f *actionsFixture) statusService() *actionApp.Service {
        if f.orderEnabled && f.order != nil {
                return f.order
        }
        return nil
}

// RecordOrderSupplierFormed delegates to the order world's status
// contour.
func (f *actionsFixture) RecordOrderSupplierFormed(ctx context.Context, command actionAPI.RecordOrderSupplierFactCommand) (actionAPI.OrderSupplierFactOutcome, error) {
        f.mu.Lock()
        service := f.statusService()
        f.mu.Unlock()
        if service == nil {
                return actionAPI.OrderSupplierFactOutcome{}, actionDomain.RuleError{Rule: "record_order_supplier_formed", Reason: notConfiguredText}
        }
        return service.RecordOrderSupplierFormed(ctx, command)
}

// RecordOrderSupplierSent delegates to the order world's status contour
// (the sent-after-formed ladder rides the same guard).
func (f *actionsFixture) RecordOrderSupplierSent(ctx context.Context, command actionAPI.RecordOrderSupplierFactCommand) (actionAPI.OrderSupplierFactOutcome, error) {
        f.mu.Lock()
        service := f.statusService()
        f.mu.Unlock()
        if service == nil {
                return actionAPI.OrderSupplierFactOutcome{}, actionDomain.RuleError{Rule: "record_order_supplier_sent", Reason: notConfiguredText}
        }
        return service.RecordOrderSupplierSent(ctx, command)
}

// ReadOrderStatusView delegates to the order world's status contour.
func (f *actionsFixture) ReadOrderStatusView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (actionDomain.OrderStatusProjection, error) {
        f.mu.Lock()
        service := f.statusService()
        f.mu.Unlock()
        if service == nil {
                return actionDomain.OrderStatusProjection{}, actionDomain.RuleError{Rule: "read_order_status", Reason: notConfiguredText}
        }
        return service.ReadOrderStatusView(ctx, actor, tenantID, caseID, now)
}
