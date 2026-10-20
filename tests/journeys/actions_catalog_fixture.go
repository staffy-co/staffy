package main

import (
	"context"
	"sync"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionApp "github.com/artemiosu/staffy/internal/action/app"
	actionCatalog "github.com/artemiosu/staffy/internal/action/catalog"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/adapters/capabilityresolver"
	"github.com/artemiosu/staffy/internal/adapters/packresolver"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 3.1 fixture: the read-only action catalog surface over the
// REAL app service, the REAL composition-root resolvers and the LIVE
// fixture membership semantics. The dictionary of the vertical is the
// fixture pair — two definitions plus two versions of one id, guard-valid
// and admitted at construction over the live catalogs (a dishonest
// fixture panics the server before the first request). The definitions
// NEVER live in the product binary: the production dictionary stays the
// honestly empty compile-time catalog (AG-4), and the fixture-only
// /fixture/actions toggle of the z29 journey switches the read between
// the two contours — both are the real service, the switch only chooses
// which dictionary is served.

// fixtureActionDefinitions builds the fixture dictionary of the slice
// vertical: the SMS notification pair (the materially different 1.1.0
// and the pinned 1.0.0 — risk low→medium, timeout 15000→30000, so the
// pin read proves the old version is served byte-exact, never the newest
// projection) and the trunk-status query definition (query_reconcilable
// with not_required confirmation — the only survival of the pair). The
// dictionary is maintained newest-first (the 1-8 convention: ListFrom
// serves the first occurrence of every id).
func fixtureActionDefinitions() []actionDomain.ActionDefinition {
	sms := func(version string) actionDomain.ActionDefinition {
		risk, timeout := "medium", 30000
		if version == "1.0.0" {
			risk, timeout = "low", 15000
		}
		return actionDomain.ActionDefinition{
			ID:      "notify.order-ready",
			Version: version,
			Name:    "Отправка SMS клиенту",
			Purpose: "Уведомить клиента о готовности заказа",
			TypedInputSchema: []actionDomain.FieldSchema{
				{Name: "phone", Label: "Телефон получателя", Type: actionDomain.FieldText, Required: true, Description: "Номер в формате E.164"},
				{Name: "template_url", Label: "Ссылка на шаблон", Type: actionDomain.FieldURL, Secret: true, Description: "Шаблон сообщения"},
			},
			TypedOutputSchema: []actionDomain.FieldSchema{
				{Name: "message_id", Label: "Идентификатор сообщения", Type: actionDomain.FieldText},
			},
			PackRef:            actionDomain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
			RequiredCapability: actionDomain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "command.send-sms"},
			OwningSystem:       "erp",
			Conditions: actionDomain.Conditions{
				RiskClass:                risk,
				AutonomyClass:            "assisted",
				AllowedConversationModes: []string{"claim", "cancellation"},
				AllowedRoles:             []string{"owner", "configurator"},
				RequiredFacts:            []actionDomain.FactRequirement{{MaterialField: "order_status", Freshness: "fresh"}},
				Confirmation:             "required",
				RequiredGates:            []string{"OQ-7A", "AG-1"},
				TimeoutMS:                timeout,
				MaterialityClass:         "material",
				ConsumptionRule:          "single_use",
				AllowedStages:            []string{"closed_account_test", "controlled_real_customer"},
			},
			DeliveryContract: actionDomain.DeliveryContract{
				CapabilityClass:       "provider_idempotent",
				IdempotencyScope:      "connector-ack",
				TerminalEvidence:      "delivery-receipt",
				ReconciliationQuery:   "query.trunk-status",
				CancelCompensationSem: "не компенсируется — отказ до подтверждения",
			},
		}
	}
	return []actionDomain.ActionDefinition{
		sms("1.1.0"),
		sms("1.0.0"),
		{
			ID:      "query.trunk-status",
			Version: "1.0.0",
			Name:    "Статус телефонной линии",
			Purpose: "Узнать текущий статус телефонной линии перед звонком",
			TypedOutputSchema: []actionDomain.FieldSchema{
				{Name: "status", Label: "Статус линии", Type: actionDomain.FieldText, Description: "Слово статуса транка"},
			},
			PackRef:            actionDomain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
			RequiredCapability: actionDomain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "query.trunk-status"},
			OwningSystem:       "catalog",
			Conditions: actionDomain.Conditions{
				RiskClass:                "low",
				AutonomyClass:            "auto",
				AllowedConversationModes: []string{"safety", "claim"},
				AllowedRoles:             []string{"operator", "expert_qa"},
				RequiredFacts:            []actionDomain.FactRequirement{{MaterialField: "availability", Freshness: "fresh"}},
				Confirmation:             "not_required",
				RequiredGates:            []string{"OQ-7B"},
				TimeoutMS:                5000,
				MaterialityClass:         "immaterial",
				ConsumptionRule:          "unlimited",
				AllowedStages:            []string{"synthetic", "closed_account_test", "controlled_real_customer"},
			},
			DeliveryContract: actionDomain.DeliveryContract{
				CapabilityClass:       "query_reconcilable",
				IdempotencyScope:      "не требуется (read-only)",
				TerminalEvidence:      "query_result",
				ReconciliationQuery:   "query.trunk-status",
				CancelCompensationSem: "не применимо (read-only)",
			},
		},
	}
}

// actionsMembership is the LIVE membership semantics of the fixture
// world for the catalog read gate: the owner holds every scope, the
// second operator holds the review+audit scopes only (the honest
// ErrForbidden member of the 403 words row), and every other actor or
// tenant is the honest ErrNotFound (the store semantics the app layer
// leans on — membership FIRST, the scope words BEFORE any material).
// The mutation surfaces stay the fixture stubs: the catalog is read-only
// and the journey never mutates the membership.
type actionsMembership struct{}

func (actionsMembership) ListMembers(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error) {
	return nil, nil
}

func (actionsMembership) InviteMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, string, error) {
	return domain.Membership{}, "", domain.ErrStepUpRequired
}

func (actionsMembership) AcceptInvitation(context.Context, domain.Principal, string, id.UUID) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrNotFound
}

func (actionsMembership) ChangeMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrStepUpRequired
}

func (actionsMembership) RevokeMember(context.Context, identityAPI.MembershipCommand) error {
	return domain.ErrStepUpRequired
}

func (actionsMembership) Authorize(_ context.Context, actor domain.Principal, tenant id.UUID, scope domain.Scope) error {
	if !actor.ID.Valid() || tenant != tenantID {
		return domain.ErrNotFound
	}
	switch actor.ID {
	case principalID:
		// The owner holds every scope (AllScopes).
		return nil
	case secondOperatorID:
		if scope == domain.Review || scope == domain.Audit {
			return nil
		}
		// The review member: the honest member refusal of the gate.
		return domain.ErrForbidden
	case operationsOperatorID:
		// The operations-only member (the Story 3-4 rights-gate row): the
		// member exists, the scope does not — the honest 403 of the
		// preparation contour (review and configuration both refuse).
		return domain.ErrForbidden
	default:
		return domain.ErrNotFound
	}
}

// actionsFixture is the fixture-only switch between the REAL contours of
// the action module: the production binary dictionary (honestly empty —
// AG-4), the fixture dictionary of the 3-1 vertical and the preview world
// of the Story 3-4 vertical (the z32 dictionary with the full preparation
// contour). The /fixture/actions toggle chooses the served contour:
// enabled=1 → the 3-1 pair, mode=preview → the 3-4 world, the default →
// the production truth (the honest empty catalog). All three are the REAL
// app service over the REAL composition-root resolvers and the same
// membership semantics.
type actionsFixture struct {
	mu      sync.Mutex
	mode    string // "" (production) | "live" | "preview"
	live    *actionApp.Service
	empty   *actionApp.Service
	preview *actionApp.Service
}

// newActionsFixture constructs the contours and proves every fixture
// dictionary admitted at construction: the guard integrity plus the
// reference resolution with the delivery-word cross-check over the LIVE
// catalogs (a dishonest fixture panics the server — the vertical never
// serves an unadmitted definition). The preview contour arrives later
// through setPreviewWorld: its fact sources are the fixture instances the
// server assembles in main().
func newActionsFixture() *actionsFixture {
	defs := fixtureActionDefinitions()
	packs, caps := packresolver.New(), capabilityresolver.New()
	if err := actionDomain.CatalogIntegrity(defs); err != nil {
		panic(err)
	}
	for _, def := range defs {
		if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
			panic(err)
		}
	}
	return &actionsFixture{
		live:  actionApp.New(actionsMembership{}, packs, caps, defs),
		empty: actionApp.New(actionsMembership{}, packs, caps, actionCatalog.All()),
	}
}

// setPreviewWorld wires the Story 3-4 contour (the main assembly hands
// over the fully-wired preview service).
func (f *actionsFixture) setPreviewWorld(service *actionApp.Service) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.preview = service
}

// setLive switches the served contour (the /fixture/actions toggle): the
// legacy enabled word keeps serving the 3-1 pair; the mode word switches
// to the Story 3-4 world.
func (f *actionsFixture) setLive(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if enabled {
		f.mode = "live"
	} else {
		f.mode = ""
	}
}

// setPreview switches the served contour to the Story 3-4 world (the
// mode=preview word of the /fixture/actions toggle).
func (f *actionsFixture) setPreview(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if enabled {
		f.mode = "preview"
	} else {
		f.mode = ""
	}
}

// service picks the contour under the lock (the reads are stateless).
func (f *actionsFixture) service() actionAPI.ActionCatalogService {
	switch f.mode {
	case "live":
		return f.live
	case "preview":
		if f.preview != nil {
			return f.preview
		}
		return f.empty
	default:
		return f.empty
	}
}

// previewService picks the preparation contour: the Story 3-4 world when
// the toggle chose it; every other contour answers the honest
// not-configured refusal (the 3-1 pair carries no preparation ports — the
// vertical never fakes them).
func (f *actionsFixture) previewService() actionAPI.PreviewService {
	if f.mode == "preview" && f.preview != nil {
		return f.preview
	}
	return nil
}

// List serves the newest version of every definition of the active
// contour (the REAL app gate and the REAL ListFrom semantics).
func (f *actionsFixture) List(ctx context.Context, actor domain.Principal, tenant id.UUID) ([]actionDomain.ActionDefinition, error) {
	f.mu.Lock()
	service := f.service()
	f.mu.Unlock()
	return service.List(ctx, actor, tenant)
}

// Get pins one definition of the active contour by id and exact version
// (the REAL app gate and the REAL GetFrom pin semantics — AC-8).
func (f *actionsFixture) Get(ctx context.Context, actor domain.Principal, tenant id.UUID, definitionID, version string) (actionDomain.ActionDefinition, error) {
	f.mu.Lock()
	service := f.service()
	f.mu.Unlock()
	return service.Get(ctx, actor, tenant, definitionID, version)
}

// The compile-time proof the fixture serves the narrow preparation
// surface the transport type-asserts (the delegation over the active
// contour).
var _ actionAPI.PreviewService = (*actionsFixture)(nil)

// notConfiguredText is the honest refusal of the contours that carry no
// preparation ports (the 3-1 pair and the empty production dictionary:
// the vertical never fakes the contour — the same words the app layer
// speaks when the ports are not wired).
const notConfiguredText = "Контур подготовки action preview не сконфигурирован: живые источники не подключены, команда честно отклоняется."

// SubmitProposal delegates to the active preparation contour; the
// contours without the ports answer the honest not-configured refusal.
func (f *actionsFixture) SubmitProposal(ctx context.Context, command actionAPI.SubmitProposalCommand) (actionAPI.ProposalOutcome, error) {
	f.mu.Lock()
	service := f.previewService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.ProposalOutcome{}, actionDomain.RuleError{Rule: "submit_proposal", Reason: notConfiguredText}
	}
	return service.SubmitProposal(ctx, command)
}

// ReadPreviewView delegates to the active preparation contour.
func (f *actionsFixture) ReadPreviewView(ctx context.Context, actor domain.Principal, tenant, caseID id.UUID) (actionAPI.PreviewView, error) {
	f.mu.Lock()
	service := f.previewService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.PreviewView{}, actionDomain.RuleError{Rule: "read_preview_view", Reason: notConfiguredText}
	}
	return service.ReadPreviewView(ctx, actor, tenant, caseID)
}

// NewPreviewVersionFromEdit delegates to the active preparation contour.
func (f *actionsFixture) NewPreviewVersionFromEdit(ctx context.Context, command actionAPI.NewPreviewVersionCommand) (actionAPI.PreviewVersionOutcome, error) {
	f.mu.Lock()
	service := f.previewService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.PreviewVersionOutcome{}, actionDomain.RuleError{Rule: "edit_preview_version", Reason: notConfiguredText}
	}
	return service.NewPreviewVersionFromEdit(ctx, command)
}

// RecordDryRun delegates to the active preparation contour.
func (f *actionsFixture) RecordDryRun(ctx context.Context, command actionAPI.DryRunCommand) (actionAPI.DryRunOutcome, error) {
	f.mu.Lock()
	service := f.previewService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.DryRunOutcome{}, actionDomain.RuleError{Rule: "record_dry_run", Reason: notConfiguredText}
	}
	return service.RecordDryRun(ctx, command)
}
