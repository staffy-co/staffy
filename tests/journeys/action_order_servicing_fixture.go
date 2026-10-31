package main

import (
	"context"
	"strings"
	"sync"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionApp "github.com/staffy-co/staffy/internal/action/app"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Story 4.10 (step-04): the order servicing contour of the journeys
// world — the in-memory repository of the Order Servicing Request over
// the order world, the LIVE assurance source over the cases fixture
// (the composition-root adapter precedent of the production root) and
// the delegation of the narrow surface the transport type-asserts (the
// status precedent). The repository mirrors the store's semantics: the
// per-case request_no ladder and the UNIQUE full request key at the
// SAME trust state (the idempotent replay BY KEY — the raced twin
// refuses honestly, the app re-reads the saved request; the CHANGED
// trust state is a NEW row, the ladder stays visible); the LOOKUP
// reads the RECORDED material of the fixture world (the committed
// receipts of the order operations ∪ the rereads of the corrections ∪
// the supplier facts — the same three sources the production store
// queries, AG-4/OQ-18) — the journeys walk the SAME honest machine the
// production store runs.

// orderServicingMemoryRepository is the in-memory implementation of
// the servicing repository contract (the status memory precedent).
type orderServicingMemoryRepository struct {
	mu          sync.Mutex
	requests    []actionDomain.OrderServicingRequest
	nextNo      map[id.UUID]int64
	raceHook    func(repo *orderServicingMemoryRepository)
	auth        *authorizationMemoryRepository
	dispatch    *dispatchMemoryRepository
	corrections *orderCorrectionMemoryRepository
	facts       *orderSupplierFactMemoryRepository
}

func newOrderServicingMemoryRepository(auth *authorizationMemoryRepository, dispatch *dispatchMemoryRepository, corrections *orderCorrectionMemoryRepository, facts *orderSupplierFactMemoryRepository) *orderServicingMemoryRepository {
	return &orderServicingMemoryRepository{
		nextNo:      map[id.UUID]int64{},
		auth:        auth,
		dispatch:    dispatch,
		corrections: corrections,
		facts:       facts,
	}
}

// ReadCaseServicingRequests reads every request of one case ordered by
// the monotonic request number (the panel feed — the ladder of the
// recorded history).
func (r *orderServicingMemoryRepository) ReadCaseServicingRequests(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]actionDomain.OrderServicingRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.OrderServicingRequest{}
	for _, request := range r.requests {
		if request.CaseID == caseID {
			out = append(out, request)
		}
	}
	return out, nil
}

// orderOperationsOf collects the ORDER operations of the tenant (the
// lookup anchor set — create_abcp_customer_order only; the auth mutex
// guards the fixture's operations map).
func (r *orderServicingMemoryRepository) orderOperationsOf(scope tenancy.Context) []actionDomain.Operation {
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	prefix := scope.TenantID().String() + "|"
	out := []actionDomain.Operation{}
	for _, operation := range r.auth.operations {
		if strings.HasPrefix(operation.TenantID.String()+"|"+operation.OperationID.String(), prefix) &&
			operation.DefinitionID == actionDomain.OrderActionID {
			out = append(out, operation)
		}
	}
	return out
}

// LookupOrderByIDentifier finds the order operations of the TENANT whose
// RECORDED fixture material carries the named external ID: the ABCP
// kind reads the committed receipts of the order operations (the system
// receipts of the dispatch fixture) UNION the rereads of the correction
// fixture; the supplier kind reads the supplier facts fixture. The
// sources ride the candidates BY NAME — the same three sources the
// production store queries (AG-4/OQ-18: NO live external read exists).
func (r *orderServicingMemoryRepository) LookupOrderByIDentifier(ctx context.Context, scope tenancy.Context, kind, value string) ([]actionDomain.OrderLookupCandidate, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	if !actionDomain.LookupIdentifierKindInDictionary(kind) {
		return []actionDomain.OrderLookupCandidate{}, nil
	}
	operations := r.orderOperationsOf(scope)
	byOperation := map[id.UUID]*actionDomain.OrderLookupCandidate{}
	order := []id.UUID{}
	addCandidate := func(operationID, caseID id.UUID, source string) {
		if candidate, ok := byOperation[operationID]; ok {
			if !actionDomain.WordIn(candidate.Sources, source) {
				candidate.Sources = append(candidate.Sources, source)
			}
			return
		}
		candidate := &actionDomain.OrderLookupCandidate{OperationID: operationID, CaseID: caseID, Sources: []string{source}}
		byOperation[operationID] = candidate
		order = append(order, operationID)
	}
	for _, operation := range operations {
		if kind == actionDomain.LookupKindABCPClientOrderID {
			// The committed receipts of the dispatch fixture (the system
			// receipts with the external ABCP Order ID).
			receipts, err := r.dispatch.ReadActionReceipts(ctx, scope, operation.OperationID)
			if err != nil {
				return nil, err
			}
			for _, receipt := range receipts {
				if receipt.Outcome == actionDomain.ReceiptOutcomeCommitted &&
					receipt.References.ExternalObjectID == value {
					addCandidate(operation.OperationID, operation.CaseID, actionDomain.OrderFactSourceSystemReceipt)
				}
			}
			// The rereads of the correction fixture (the manager records
			// with the external object IDs).
			corrections, err := r.corrections.ReadOperationCorrections(ctx, scope, operation.OperationID)
			if err != nil {
				return nil, err
			}
			for _, correction := range corrections {
				events, err := r.corrections.ReadCorrectionEvents(ctx, scope, correction.CaseID, correction.CorrectionNo)
				if err != nil {
					return nil, err
				}
				for _, event := range events {
					if event.Kind == actionDomain.OrderCorrectionEventReread && event.ExternalObjectID == value {
						addCandidate(operation.OperationID, operation.CaseID, actionDomain.OrderFactSourceManagerRecord)
					}
				}
			}
		} else {
			// The supplier kind: the human facts of the status fixture.
			facts, err := r.facts.ReadOperationSupplierFacts(ctx, scope, operation.OperationID)
			if err != nil {
				return nil, err
			}
			for _, fact := range facts {
				if fact.SupplierOrderID == value {
					addCandidate(operation.OperationID, operation.CaseID, actionDomain.OrderFactSourceManagerRecord)
				}
			}
		}
	}
	candidates := make([]actionDomain.OrderLookupCandidate, 0, len(order))
	for _, operationID := range order {
		candidates = append(candidates, *byOperation[operationID])
	}
	return candidates, nil
}

// ReadResolvedOperation reads the operation row of the resolved anchor
// (the LIVE material assembly of the disclosure view — the honest miss
// never fails the view).
func (r *orderServicingMemoryRepository) ReadResolvedOperation(_ context.Context, _ tenancy.Context, operationID id.UUID) (actionDomain.Operation, bool, error) {
	r.auth.mu.Lock()
	defer r.auth.mu.Unlock()
	for _, operation := range r.auth.operations {
		if operation.OperationID == operationID {
			return operation, true, nil
		}
	}
	return actionDomain.Operation{}, false, nil
}

// AppendOrderServicingRequest appends ONE request under the per-case
// ladder with the UNIQUE full request key at the SAME trust state (the
// idempotency BY KEY — the raced replay of the same command refuses
// with the honest duplicate signal, the app re-reads the saved request;
// the CHANGED trust state is a DIFFERENT key — a new row lands).
func (r *orderServicingMemoryRepository) AppendOrderServicingRequest(_ context.Context, p actionPort.AppendOrderServicingRequestParams) (actionDomain.OrderServicingRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.raceHook != nil {
		hook := r.raceHook
		r.raceHook = nil
		hook(r)
	}
	for _, request := range r.requests {
		if request.CaseID == p.CaseID && request.ActorID == p.ActorID && request.Intent == p.Intent &&
			request.IdentifierKind == p.IdentifierKind && request.IdentifierValue == p.IdentifierValue &&
			request.ProofState == p.ProofState {
			return actionDomain.OrderServicingRequest{}, actionPort.ErrOrderServicingRequestDuplicate
		}
	}
	requestNo := r.nextNo[p.CaseID] + 1
	r.nextNo[p.CaseID] = requestNo
	request := actionDomain.OrderServicingRequest{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, ConversationID: p.ConversationID,
		RequestNo: requestNo, Intent: p.Intent, IdentifierKind: p.IdentifierKind,
		IdentifierValue: p.IdentifierValue, ResolvedOperationID: p.ResolvedOperationID,
		LookupOutcome: p.LookupOutcome, ProofState: p.ProofState, ProofMethod: p.ProofMethod,
		ProofIssuedAt: p.ProofIssuedAt, ProofExpiresAt: p.ProofExpiresAt,
		Disclosure: p.Disclosure, ActorID: p.ActorID, RecordedAt: p.Now,
	}
	r.requests = append(r.requests, request)
	return request, nil
}

// orderAssuranceSource is the LIVE assurance source of the journeys
// world: the composition-root adapter over the cases fixture (the
// production adapter precedent — the CaseView read projects the
// assurance section onto the port; the gate reads ONLY the
// verification records, FR-11 structural).
type orderAssuranceSource struct {
	cases *casesFixture
}

// The structural pin: the fixture satisfies the consumer-owned port.
var _ actionPort.AssuranceSource = orderAssuranceSource{}

func (s orderAssuranceSource) AssuranceView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (casesAPI.AssuranceView, error) {
	view, err := s.cases.CaseView(ctx, actor, tenantID, caseID)
	if err != nil {
		return casesAPI.AssuranceView{}, err
	}
	return view.Assurance, nil
}

// servicingService picks the order world: the Story 4.6 composition when
// its toggle chose it; every other contour answers the honest
// not-configured refusal (the servicing contour lives ONLY in the order
// world — the same routing as the status surface).
func (f *actionsFixture) servicingService() *actionApp.Service {
	if f.orderEnabled && f.order != nil {
		return f.order
	}
	return nil
}

// RecordOrderServicingRequest delegates to the order world's servicing
// contour.
func (f *actionsFixture) RecordOrderServicingRequest(ctx context.Context, command actionAPI.RecordOrderServicingRequestCommand) (actionAPI.OrderServicingOutcome, error) {
	f.mu.Lock()
	service := f.servicingService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.OrderServicingOutcome{}, actionDomain.RuleError{Rule: "record_order_servicing_request", Reason: notConfiguredText}
	}
	return service.RecordOrderServicingRequest(ctx, command)
}

// ReadOrderServicingView delegates to the order world's servicing
// contour.
func (f *actionsFixture) ReadOrderServicingView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (actionDomain.OrderServicingView, error) {
	f.mu.Lock()
	service := f.servicingService()
	f.mu.Unlock()
	if service == nil {
		return actionDomain.OrderServicingView{}, actionDomain.RuleError{Rule: "read_order_servicing", Reason: notConfiguredText}
	}
	return service.ReadOrderServicingView(ctx, actor, tenantID, caseID, now)
}
