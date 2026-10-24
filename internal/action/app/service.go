package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/artemiosu/staffy/internal/action/api"
	actionCatalog "github.com/artemiosu/staffy/internal/action/catalog"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// actionCatalogScopeText is the honest scope refusal of the catalog read:
// the words of the two scopes that DO open the surface, spoken BEFORE any
// catalog material (AC-9). The action dictionary is configuration and
// integration material — an operator or audit member reads the working and
// audit contours, never the setup material (the decision-context 2-14
// precedent of the words-first 403).
const actionCatalogScopeText = "Чтение каталога действий требует полномочия integrations или configuration: словарь действий — материал настройки интеграций и конфигурации, оператору и аудиту он не открывается до вычисления доступности."

// The compile-time proof of the narrow interface registration: the
// transport type-asserts api.ActionCatalogService (the HandoffService
// precedent).
var _ api.ActionCatalogService = (*Service)(nil)

// Service is the application surface of the action module: the read-only
// CatalogService of story 3-1 (step-02: ports, resolvers, the app layer —
// it owns NO store for the catalog; the definition dictionary is
// compile-time) plus the preparation contour of story 3-4 (step-02 of
// 3-4: the preview repository and the LIVE fact ports). The 3-4 fields
// are wired through the With* builders — the 3-1 New() signature stays
// byte-stable (the transport and the existing tests are untouched), and
// an unwired 3-4 port is the HONEST refusal, never an assumed "ok"
// (the ConnectionsFacts precedent).
type Service struct {
	identity identityAPI.MembershipService
	packs    domain.PackRefResolver
	caps     domain.CapabilityRefResolver
	defs     []domain.ActionDefinition

	// Story 3-4 (step-02): the preparation contour.
	previews       port.PreviewRepository  // the append-only store (mandatory for the contour)
	previewReader  port.OwningSystemReader // nil = the honest blocked_material (AG-4)
	dryRunner      port.DryRunner          // nil = the honest refusal in words (AG-4)
	previewControl port.ControlEffectiveReader
	caseMaterial   port.CaseMaterialSource
	members        port.MembersSource
	entitlement    port.EntitlementSource
	readiness      port.ReadinessSource
	connectors     port.ConnectorFactsSource
	routeFacts     port.RouteFactsSource
	ids            api.IDGenerator

	// Story 3.5 (step-02): the confirmation→authorization contour. The two
	// repositories of the consent and the operations share the SAME
	// Postgres store of the module; the LIVE fact sources above are the
	// ones the freshness/policy re-checks of the gateway read (the 3-4
	// wiring — «3-5 читает тот же вывод»).
	confirmations port.ConfirmationRepository
	operations    port.OperationRepository

	// Story 3.6 (step-02): the execution contour — the dispatch worker.
	// The dispatch repository owns the THREE adapter-free transactions
	// (the write-ahead lease, the fenced verdict, the raced-guarded
	// parking); the mutation adapter is the consumer-owned door to the
	// provider world — nil is the HONEST production state (AG-4: no
	// real providers exist, the drain refuses in words, the queue
	// honestly stays pending). The adapter is called ONLY outside every
	// store transaction (persist-before-emit, AD-6).
	dispatches      port.DispatchRepository
	dispatchAdapter port.MutationAdapter

	// Story 3.7 (step-02): the reconciliation contour — the reconcile worker.
	// The reconciliation repository owns the TWO adapter-free transactions
	// (the write-ahead query lease, the fenced verdict with the terminal
	// lock); the READ-ONLY query adapter is the consumer-owned door to the
	// answer world — nil is the HONEST production state (AG-4: no real
	// owning systems exist, the reconcile command refuses in words, the
	// pending targets honestly stay). The adapter is called ONLY outside
	// every store transaction (persist-before-emit, AD-6).
	reconciliations  port.ReconciliationRepository
	reconcileAdapter port.ReconciliationAdapter

	// Story 3.8 (step-02): the recovery-case contour — the creation
	// worker. The recovery repository owns the ONE creation transaction
	// (the advisory lock resolves the creation race by the idempotent
	// return) and the read-time fact reads; there is NO adapter at all —
	// the contour is LOCAL (the assignment reads the LIVE ownership and
	// coverage words through the caseMaterial/routeFacts ports wired since
	// 3-4 — in-process module reads, never the provider world; the
	// network stays in the dispatch/reconciliation contours of 3-6/3-7,
	// AG-4).
	recoveries port.RecoveryRepository

	// Story 4.6 (step-02): the order composition revalidation source — the
	// consumer-owned port the dispatch ladder reads AFTER the confirmation
	// fence and BEFORE the case liveness (AC-5: the live re-derivation of
	// the ONE canonical OrderDigest; any substantial change of ANY of the
	// five inputs blocks the dispatch with the exact word). A NIL source is
	// the HONEST production boundary (the FreshReads precedent, AG-4): the
	// order dispatch fails CLOSED — never an assumed «unchanged».
	orderVersions port.OrderVersionSource
}

// New wires the service. The resolvers are the composition-root adapters
// over the live pack and connector catalogs (the ports). defs is the
// definition dictionary this instance serves: the PRODUCTION composition
// root passes the binary catalog (honestly empty today, AG-4); the TEST
// composition root registers fixture definitions (the vertical of the
// slice — fixtures never live in the product binary). The slice is treated
// as immutable from here on; every read returns a defensive copy.
func New(identity identityAPI.MembershipService, packs port.PackRefResolver, caps port.CapabilityRefResolver, defs []domain.ActionDefinition) *Service {
	return &Service{identity: identity, packs: packs, caps: caps, defs: defs}
}

// WithPreviews wires the whole preparation contour of story 3-4 in one
// call (the fixture and the transport composition roots): the
// repository, the LIVE fact sources and the ID generator. The optional
// AG-4 adapters (the owning-system reader, the dry-runner) are wired
// separately — their absence is a HONEST state, not a misconfiguration
// (nil reader = the blocked_material readiness word, nil runner = the
// spoken unsupported refusal).
func (s *Service) WithPreviews(repository port.PreviewRepository, control port.ControlEffectiveReader,
	material port.CaseMaterialSource, memberFacts port.MembersSource,
	admissions port.EntitlementSource, readinessFacts port.ReadinessSource,
	connections port.ConnectorFactsSource, routes port.RouteFactsSource, ids api.IDGenerator) *Service {
	s.previews = repository
	s.previewControl = control
	s.caseMaterial = material
	s.members = memberFacts
	s.entitlement = admissions
	s.readiness = readinessFacts
	s.connectors = connections
	s.routeFacts = routes
	s.ids = ids
	return s
}

// WithPreviewReader wires the material reader of the snapshot (the test
// contour wires the synthetic adapter; the production root wires NOTHING
// — AG-4: no real providers exist, the readiness honestly blocks).
func (s *Service) WithPreviewReader(reader port.OwningSystemReader) *Service {
	s.previewReader = reader
	return s
}

// WithPreviewDryRunner wires the validation adapter (the test contour
// only — AG-4: the production root wires nothing, the dry-run surface
// honestly refuses in words).
func (s *Service) WithPreviewDryRunner(runner port.DryRunner) *Service {
	s.dryRunner = runner
	return s
}

// WithAuthorization wires the confirmation→authorization contour of story
// 3.5 in one call (the fixture and the transport composition roots): the
// consent repository, the operation repository. The LIVE fact sources of
// the gateway are the WithPreviews wiring (the same ports the preparation
// reads); the optional AG-4 owning-system reader is the WithPreviewReader
// wiring — its absence is the HONEST fail-closed authorization refusal
// (the production root wires nothing).
func (s *Service) WithAuthorization(consents port.ConfirmationRepository, ops port.OperationRepository) *Service {
	s.confirmations = consents
	s.operations = ops
	return s
}

// WithDispatch wires the execution contour of story 3.6 (the fixture and
// the transport composition roots): the dispatch repository and the
// mutation adapter. The adapter may stay nil — the HONEST production
// state (AG-4: no real providers exist; the drain command refuses in
// words, the queue honestly stays pending; the synthetic adapter of the
// test contour is importable ONLY by the composition roots, never by the
// product binary — the import-graph probe of step-04).
func (s *Service) WithDispatch(dispatches port.DispatchRepository, adapter port.MutationAdapter) *Service {
	s.dispatches = dispatches
	s.dispatchAdapter = adapter
	return s
}

// WithReconcile wires the reconciliation contour of story 3.7 (the fixture
// and the transport composition roots): the reconciliation repository and
// the READ-ONLY query adapter. The adapter may stay nil — the HONEST
// production state (AG-4: no real owning systems exist; the reconcile
// command refuses in words, the pending targets honestly stay; the
// synthetic adapter of the test contour is importable ONLY by the
// composition roots, never by the product binary — the import-graph probe
// of step-04).
func (s *Service) WithReconcile(repository port.ReconciliationRepository, adapter port.ReconciliationAdapter) *Service {
	s.reconciliations = repository
	s.reconcileAdapter = adapter
	return s
}

// WithRecovery wires the recovery-case contour of story 3.8 (the fixture
// and the transport composition roots): the recovery repository ONLY —
// the contour carries NO adapter of its own (the locality of the slice:
// the creation command makes not ONE provider call; the assignment
// reads the LIVE ownership/coverage words through the ports wired since
// 3-4 — the WithPreviews wiring; the eligibility reads the declaredness
// through the caps resolver of the catalog). The reconciliation adapter
// nil IS the honest production fact the eligibility ladder reads
// (AG-4: the unwired contour answers reconciliation_unavailable — the
// human is mandatory — and the recovery cases still get created).
func (s *Service) WithRecovery(repository port.RecoveryRepository) *Service {
	s.recoveries = repository
	return s
}

// WithOrderVersions wires the order composition revalidation source of
// story 4.6 (the fixture and the transport composition roots): the port
// answers the LIVE OrderDigest derivation of one case. The source may stay
// nil — the HONEST production state (AG-4: the in-process derivation is
// wired by the roots that serve the order contour; a root that wires
// nothing fails the order dispatch CLOSED with the exact word, the
// FreshReads precedent).
func (s *Service) WithOrderVersions(source port.OrderVersionSource) *Service {
	s.orderVersions = source
	return s
}

// authorize is the read gate, in the honest order:
//
//  1. membership FIRST through the identity port — a non-member of the
//     tenant gets the 404 (Authorize answers ErrNotFound for a missing
//     active membership; no disclosure that the tenant has an action
//     catalog at all);
//  2. the scope words 403 BEFORE any catalog material — a member holding
//     NEITHER integrations NOR configuration reads nothing: the honest
//     refusal names the two scopes that would open the surface.
//
// The member-with-integrations path is one Authorize call; only the
// integrations-less member pays the second (configuration) call.
func (s *Service) authorize(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) error {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return domain.ErrNotFound
	}
	integrationsErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Integrations)
	if integrationsErr == nil {
		return nil
	}
	if errors.Is(integrationsErr, identityDomain.ErrForbidden) {
		// The member exists: configuration is the second honest way in.
		configurationErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Configuration)
		if configurationErr == nil {
			return nil
		}
		if errors.Is(configurationErr, identityDomain.ErrForbidden) {
			return fmt.Errorf("%w: %s", domain.ErrForbidden, actionCatalogScopeText)
		}
		if errors.Is(configurationErr, identityDomain.ErrNotFound) {
			// The membership vanished between the two calls: the honest 404.
			return domain.ErrNotFound
		}
		return configurationErr
	}
	if errors.Is(integrationsErr, identityDomain.ErrNotFound) {
		// A non-member of the tenant: the module's own 404 identity (the
		// transport maps the action domain error, never the identity one).
		return domain.ErrNotFound
	}
	return integrationsErr
}

// List serves the newest version of every definition of the injected
// dictionary (the read gate FIRST; a defensive copy — the caller may
// mutate the result, never the dictionary).
func (s *Service) List(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]domain.ActionDefinition, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return actionCatalog.ListFrom(s.defs), nil
}

// Get pins one definition by id and exact version (AC-8). The read gate
// runs FIRST; an unknown or unparseable pin is the honest ErrNotFound —
// the pinned past stays resolvable, the invented future does not exist.
func (s *Service) Get(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, definitionID, version string) (domain.ActionDefinition, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return domain.ActionDefinition{}, err
	}
	def, ok := actionCatalog.GetFrom(s.defs, definitionID, version)
	if !ok {
		return domain.ActionDefinition{}, domain.ErrNotFound
	}
	return def, nil
}

// ValidateBinaryCatalog walks the PRODUCTION binary dictionary through the
// full admission pass: the guard integrity (CatalogIntegrity — every entry
// individually guarded, pin pairs unique, versions parseable) plus the
// reference resolution and the delivery-word cross-check of every entry
// (ValidateDefinitionRefs) through the live resolvers. The honest empty
// catalog passes vacuously (AG-4); the integration test keeps this pass
// live so the first real entry can only appear through the full admission
// truth. Nothing here touches the injected fixture dictionary — the
// product binary is the subject.
func (s *Service) ValidateBinaryCatalog() error {
	defs := actionCatalog.All()
	if err := domain.CatalogIntegrity(defs); err != nil {
		return err
	}
	for _, d := range defs {
		if err := domain.ValidateDefinitionRefs(d, s.packs, s.caps); err != nil {
			return err
		}
	}
	return nil
}
