package main

// The Story 4.6 order commitment vertical of the journeys world: the
// SECOND app service over the SAME live fixture fact sources (the cases
// material, the control route, the entitlement admission, the readiness
// views, the release lineage) with the ORDER-serving test contour — the
// synthetic mutation and reconciliation adapters over the
// command.prepare-client-order capability word, the LIVE order version
// source (the re-derivation of the canonical OrderDigest over the REAL
// fixture case view) and the order connector facts (the verified state of
// the mutating capability — the honest test-contour class: the REAL
// connector surface never probe-verifies a mutating capability before the
// Action Gateway, AG-4; this composition fact is exactly what the
// integration matrix pins with its own orderConnectorFacts).
//
// The repositories are FRESH in-memory instances: the order contour owns
// its own rows (the honest isolation of the vertical — the z32 family
// state never leaks in, the order journey never drains another spec's
// queue). The /fixture/order-world toggle routes EVERY action surface to
// this composition while it is on (the /fixture/recovery-ag4 precedent —
// the composition root IS the world).

import (
	"context"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionApp "github.com/artemiosu/staffy/internal/action/app"
	actionCatalog "github.com/artemiosu/staffy/internal/action/catalog"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/adapters/actiondispatcher"
	"github.com/artemiosu/staffy/internal/adapters/capabilityresolver"
	"github.com/artemiosu/staffy/internal/adapters/owningsystem"
	"github.com/artemiosu/staffy/internal/adapters/packresolver"
	"github.com/artemiosu/staffy/internal/adapters/reconciliationsyn"
	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	domain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// orderConnectorFacts is the ConnectorFactsSource of the order world: the
// abcp-parts connection with command.prepare-client-order in the VERIFIED
// state. The honest test-contour truth: the REAL connector surface
// derives the mutating capability as unverified with the Action Gateway
// block (AG-4 — the frozen 3-4 discipline), and the order vertical serves
// the composition fact of the test circuit (the same class as the
// synthetic mutation adapter — production wires neither).
type orderConnectorFacts struct{}

// The structural pin: the fixture satisfies the consumer-owned fact port.
var _ actionPort.ConnectorFactsSource = orderConnectorFacts{}

func (orderConnectorFacts) ConnectionsFacts(_ context.Context, _ tenancy.Context) ([]connectorAPI.ConnectionFacts, error) {
	return []connectorAPI.ConnectionFacts{{
		ConnectionID:    orderConnectionID,
		ConnectorID:     "abcp-parts",
		ContractVersion: "1.1.0",
		Capabilities: []connectorDomain.CapabilityReport{{
			CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: "command.prepare-client-order"},
			State:                 connectorDomain.CapStateVerified,
		}},
	}}, nil
}

// orderConnectionID is the deterministic connection identity of the order
// fixture facts (the 4-1 pin material of the route lineage).
var orderConnectionID = id.MustParse("01994f64-6c00-7000-8000-00000000c641")

// orderVersionSource is the LIVE order version source of the journeys
// world: the re-derivation reads the REAL fixture case view (the same
// DeriveOrderCommitmentMaterial the builder used — the honest
// revalidation of the dispatch ladder, AC-5).
type orderVersionSource struct {
	cases *casesFixture
}

// The structural pin: the fixture satisfies the consumer-owned port.
var _ actionPort.OrderVersionSource = orderVersionSource{}

func (s orderVersionSource) OrderCommitmentDigest(ctx context.Context, request actionPort.OrderVersionRequest) (string, error) {
	view, err := s.cases.CaseView(ctx, principalOfTheWorld(), request.Scope.TenantID(), request.CaseID)
	if err != nil {
		return "", err
	}
	material, err := actionApp.DeriveOrderCommitmentMaterial(view)
	if err != nil {
		return "", err
	}
	return actionDomain.OrderDigestOf(material), nil
}

// principalOfTheWorld renders the acting fixture principal of the order
// reads (the step-up owner — the caller-authority port precedent: the app
// layer has ALREADY verified the actor's rights when it reads).
func principalOfTheWorld() domain.Principal {
	return domain.Principal{ID: principalID, StepUpVerified: true}
}

// newOrderWorld wires the Story 4.6 order composition: the PRODUCT action
// dictionary (the first product entry create_abcp_customer_order@1.0.0 —
// the admission walk at construction over the LIVE catalogs) on top of
// the z32 fixture definitions (the vertical keeps its neighbors), the
// FRESH in-memory repositories of the order contour, the synthetic
// order-serving adapters and the LIVE order version source.
func newOrderWorld(casesWorld *casesFixture, controls *controlsFixture, entitlement *entitlementFixture, readiness *readinessFixture) (*actionApp.Service, *actiondispatcher.Synthetic, *reconciliationsyn.Synthetic) {
	packs, caps := packresolver.New(), capabilityresolver.New()
	defs := append(actionCatalog.All(), fixturePreviewDefinitions()...)
	// The admission walk at construction (the fixture discipline: a
	// dishonest dictionary panics the server).
	if err := actionDomain.CatalogIntegrity(defs); err != nil {
		panic("the order world dictionary must pass the guard: " + err.Error())
	}
	for _, def := range defs {
		if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
			panic("the order world dictionary must resolve against the live catalogs: " + err.Error())
		}
	}
	previews := newPreviewMemoryRepository()
	authorizations := newAuthorizationMemoryRepository(previews)
	dispatchRepo, dispatchAdapter := newDispatchMemoryRepository(authorizations), actiondispatcher.NewServing("command.prepare-client-order")
	reconcileRepo, reconcileAdapter := newReconcileMemoryRepository(authorizations, dispatchRepo), reconciliationsyn.NewServing("command.prepare-client-order")
	recoveryRepo := newRecoveryMemoryRepository(authorizations, dispatchRepo, reconcileRepo, previews)
	service := actionApp.New(previewMembership{}, packs, caps, defs).
		WithPreviews(previews, previewControlRoute{controls: controls},
			previewCasesMaterial{cases: casesWorld}, previewMembership{}, entitlement,
			readiness, orderConnectorFacts{}, previewRouteFacts{entitlement: entitlement}, previewIDsPool).
		WithAuthorization(authorizations, authorizations).
		WithDispatch(dispatchRepo, dispatchAdapter).
		WithReconcile(reconcileRepo, reconcileAdapter).
		WithRecovery(recoveryRepo).
		WithPreviewReader(owningsystem.New()).
		WithOrderVersions(orderVersionSource{cases: casesWorld}).
		// The Story 4.7 verification contour: the Delegated Task over the
		// committed receipt (the in-memory repository of the journeys
		// world — the same ladder/CAS semantics as the store).
		WithOrderVerifications(newOrderVerificationMemoryRepository())
	return service, dispatchAdapter, reconcileAdapter
}

// The compile-time proof the actions fixture serves the narrow order
// builder surface the transport type-asserts (the delegation over the
// active contour — the PreviewService precedent).
var _ actionAPI.OrderProposalService = (*actionsFixture)(nil)

// PrepareOrderProposal delegates to the active preparation contour (the
// order world while its toggle is on — the AG-4 routing precedent); the
// contours without the builder answer the honest not-configured refusal.
func (f *actionsFixture) PrepareOrderProposal(ctx context.Context, command actionAPI.PrepareOrderProposalCommand) (actionAPI.ProposalOutcome, error) {
	f.mu.Lock()
	service := f.orderService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.ProposalOutcome{}, actionDomain.RuleError{Rule: "prepare_order_proposal", Reason: notConfiguredText}
	}
	return service.PrepareOrderProposal(ctx, command)
}

// orderService picks the order world: the Story 4.6 composition when its
// toggle chose it; every other contour answers the honest
// not-configured refusal (the builder lives ONLY in the order world —
// the z32 dictionary carries no order definition).
func (f *actionsFixture) orderService() *actionApp.Service {
	if f.orderEnabled && f.order != nil {
		return f.order
	}
	return nil
}

// setOrderWorld wires the Story 4.6 order composition (the main assembly
// hands over the fully-wired order service).
func (f *actionsFixture) setOrderWorld(service *actionApp.Service) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.order = service
}

// setOrder switches the served composition (the /fixture/order-world
// toggle — the /fixture/recovery-ag4 precedent): the order world serves
// EVERY action surface of the fixture while the flag is on.
func (f *actionsFixture) setOrder(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orderEnabled = enabled
}
