package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	actionApp "github.com/artemiosu/staffy/internal/action/app"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/adapters/capabilityresolver"
	"github.com/artemiosu/staffy/internal/adapters/dryrunner"
	"github.com/artemiosu/staffy/internal/adapters/owningsystem"
	"github.com/artemiosu/staffy/internal/adapters/packresolver"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	controlDomain "github.com/artemiosu/staffy/internal/control/domain"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// The Story 3-4 fixture: the proposal→preview preparation contour over the
// REAL app service, the REAL domain derivations and the LIVE fixture fact
// sources (the cases view of the fixture case world, the identity member
// facts, the entitlement admission of the fixture commercial perimeter, the
// readiness views of the fixture employee, the connector capability facts
// of the fixture connections, the release lineage of the fixture route)
// with the in-memory repository of the fixture world (the "мир пути, не
// продуктовая персистентность" of the spec — the append-only ladders, the
// attempt bound, the atomic supersede and the monotonic event numbers over
// the memory rows). The synthetic OwningSystemReader and DryRunner of the
// test contour carry the material toggles (the /fixture/preview-material
// and /fixture/preview-dry-run switches — production never exposes either,
// and the production root wires no adapter at all: AG-4).
//
// The dictionary of the z32 vertical lives on the THIRD contour of the
// actionsFixture (the /fixture/actions mode=preview switch): the z29 pair
// stays byte-identical, the empty production contour stays the honest AG-4
// boundary (no definitions; the preview contour wired but NO reader and NO
// runner — the readiness honestly blocks, the dry-run honestly refuses).

// previewIDs is the monotonic id generator of the preview fixture world
// (the REAL v7 mint of the journey clock — the preview threads and the
// events demand unique identities; the fixed idsFixture of the transport
// surface would collide the threads).
type previewIDs struct {
	mu sync.Mutex
}

func (p *previewIDs) New(now time.Time) (id.UUID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return id.NewV7(now)
}

// previewMemoryRepository is the in-memory PreviewRepository of the fixture
// world: the attempt ladder keyed by the derived proposal key (the AD-7
// bound enforced structurally — the raced twin and the third attempt are
// the honest ladder refusals), the version ladder keyed by the stable
// preview identity (the atomic supersede under the one lock), and the
// append-only events with the monotonic per-version numbers.
type previewMemoryRepository struct {
	mu        sync.Mutex
	attempts  map[string][]actionPort.ProposalAttemptRow
	versions  map[string][]actionDomain.ActionPreview
	events    map[string][]actionPort.PreviewEventRow
	caseIndex map[id.UUID][]id.UUID // case → the preview ids (the case reads)
	key       []byte
}

func newPreviewMemoryRepository() *previewMemoryRepository {
	return &previewMemoryRepository{
		attempts:  map[string][]actionPort.ProposalAttemptRow{},
		versions:  map[string][]actionDomain.ActionPreview{},
		events:    map[string][]actionPort.PreviewEventRow{},
		caseIndex: map[id.UUID][]id.UUID{},
		key:       []byte("fixture-action-preview-hmac-key"),
	}
}

// The structural pin: the memory rows satisfy the consumer-owned contract.
var _ actionPort.PreviewRepository = (*previewMemoryRepository)(nil)

func (r *previewMemoryRepository) ReadProposalAttempts(_ context.Context, _ tenancy.Context, proposalKey string) ([]actionPort.ProposalAttemptRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]actionPort.ProposalAttemptRow(nil), r.attempts[proposalKey]...)
	sort.Slice(out, func(i, j int) bool { return out[i].AttemptNo < out[j].AttemptNo })
	return out, nil
}

func (r *previewMemoryRepository) ReadCaseAttempts(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]actionPort.ProposalAttemptRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionPort.ProposalAttemptRow{}
	for _, rows := range r.attempts {
		for _, row := range rows {
			if row.CaseID == caseID {
				out = append(out, row)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *previewMemoryRepository) ReadPreviewVersions(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]actionDomain.ActionPreview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.ActionPreview{}
	for _, previewID := range r.caseIndex[caseID] {
		out = append(out, r.versions[previewID.String()]...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PreviewID != out[j].PreviewID {
			return out[i].PreviewID < out[j].PreviewID
		}
		return out[i].VersionNo < out[j].VersionNo
	})
	return out, nil
}

func (r *previewMemoryRepository) ReadPreviewEvents(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]actionPort.PreviewEventRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionPort.PreviewEventRow{}
	for _, previewID := range r.caseIndex[caseID] {
		out = append(out, r.events[previewID.String()]...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PreviewID != out[j].PreviewID {
			return out[i].PreviewID < out[j].PreviewID
		}
		if out[i].VersionNo != out[j].VersionNo {
			return out[i].VersionNo < out[j].VersionNo
		}
		return out[i].EventNo < out[j].EventNo
	})
	return out, nil
}

func (r *previewMemoryRepository) ReadPreviewKey(context.Context, tenancy.Context, time.Time) ([]byte, error) {
	return r.key, nil
}

func (r *previewMemoryRepository) RecordProposalAttempt(_ context.Context, p actionPort.RecordAttemptParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, row := range r.attempts[p.Attempt.ProposalKey] {
		if row.AttemptNo == p.Attempt.AttemptNo {
			// The raced twin of the ladder: the honest stale refusal.
			return actionPort.ErrStalePreviewVersion
		}
	}
	if p.Attempt.AttemptNo > actionDomain.MaxAttemptsProposal {
		return actionPort.ErrStalePreviewVersion
	}
	r.attempts[p.Attempt.ProposalKey] = append(r.attempts[p.Attempt.ProposalKey], p.Attempt)
	r.indexThread(p.Attempt.PreviewID, p.Attempt.CaseID)
	r.appendEventLocked(actionPort.PreviewEventRow{
		PreviewID: p.Attempt.PreviewID, VersionNo: 0, CaseID: p.Attempt.CaseID,
		EventNo: r.nextEventNoLocked(p.Attempt.PreviewID, 0), Kind: p.StageEventKind,
		Reason: p.StageEventReason, ActorID: p.Attempt.ActorID, CreatedAt: p.Now.UTC(),
	}, p.Attempt.CaseID)
	return nil
}

func (r *previewMemoryRepository) BuildPreviewVersion(_ context.Context, p actionPort.BuildPreviewVersionParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	thread := p.Preview.PreviewID.String()
	for _, version := range r.versions[thread] {
		if version.VersionNo == p.Preview.VersionNo {
			// The raced build of the same version number: the honest ladder
			// refusal — the caller re-derives, never merges.
			return actionPort.ErrStalePreviewVersion
		}
	}
	if p.Attempt != nil {
		for _, row := range r.attempts[p.Attempt.ProposalKey] {
			if row.AttemptNo == p.Attempt.AttemptNo {
				return actionPort.ErrStalePreviewVersion
			}
		}
		r.attempts[p.Attempt.ProposalKey] = append(r.attempts[p.Attempt.ProposalKey], *p.Attempt)
	}
	// The digest re-derivation under the lock (the 3-3 token discipline):
	// the built row must re-verify over the canonical bytes.
	if digest := actionDomain.PreviewDigest(r.key, p.Preview.PreviewID, p.Preview.VersionNo, actionDomain.PreviewCanonicalBytes(p.Preview)); digest != p.Preview.Digest {
		return actionPort.ErrPreviewIntegrity
	}
	r.versions[thread] = append(r.versions[thread], p.Preview)
	r.indexThread(p.Preview.PreviewID, p.Preview.CaseID)
	if p.Attempt != nil {
		r.appendEventLocked(actionPort.PreviewEventRow{
			PreviewID: p.Preview.PreviewID, VersionNo: 0, CaseID: p.Preview.CaseID,
			EventNo: r.nextEventNoLocked(p.Preview.PreviewID, 0), Kind: actionDomain.EventProposed,
			Reason: "", ActorID: p.Preview.AuthorID, CreatedAt: p.Now.UTC(),
		}, p.Preview.CaseID)
	}
	r.appendEventLocked(actionPort.PreviewEventRow{
		PreviewID: p.Preview.PreviewID, VersionNo: p.Preview.VersionNo, CaseID: p.Preview.CaseID,
		EventNo: r.nextEventNoLocked(p.Preview.PreviewID, p.Preview.VersionNo), Kind: actionDomain.EventPreviewed,
		Reason: "", ActorID: p.Preview.AuthorID, CreatedAt: p.Now.UTC(),
	}, p.Preview.CaseID)
	if p.SupersedesVersion >= 1 {
		// The atomic supersede: the OLD version's event lands in the SAME
		// locked write (the Timeline derivation reads it — the stored rows
		// never mutate).
		r.appendEventLocked(actionPort.PreviewEventRow{
			PreviewID: p.Preview.PreviewID, VersionNo: p.SupersedesVersion, CaseID: p.Preview.CaseID,
			EventNo: r.nextEventNoLocked(p.Preview.PreviewID, p.SupersedesVersion), Kind: actionDomain.EventSuperseded,
			Reason: "", ActorID: p.Preview.AuthorID, CreatedAt: p.Now.UTC(),
		}, p.Preview.CaseID)
	}
	return nil
}

func (r *previewMemoryRepository) RecordDryRunEvent(_ context.Context, p actionPort.RecordDryRunParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appendEventLocked(actionPort.PreviewEventRow{
		PreviewID: p.PreviewID, VersionNo: p.VersionNo, CaseID: p.CaseID,
		EventNo: r.nextEventNoLocked(p.PreviewID, p.VersionNo), Kind: actionDomain.EventDryRunRecorded,
		Reason: p.Result, Details: p.Details, ActorID: p.ActorID, CreatedAt: p.Now.UTC(),
	}, p.CaseID)
	return nil
}

// indexThread registers the thread of the case (the case reads walk the
// index — the case keeps every thread of its material).
func (r *previewMemoryRepository) indexThread(previewID id.UUID, caseID id.UUID) {
	for _, existing := range r.caseIndex[caseID] {
		if existing == previewID {
			return
		}
	}
	r.caseIndex[caseID] = append(r.caseIndex[caseID], previewID)
}

// appendEventLocked appends one event row (the caller holds the lock).
func (r *previewMemoryRepository) appendEventLocked(row actionPort.PreviewEventRow, caseID id.UUID) {
	r.indexThread(row.PreviewID, caseID)
	r.events[row.PreviewID.String()] = append(r.events[row.PreviewID.String()], row)
}

// nextEventNoLocked derives the monotonic event number of one version scope
// (0 = the proposal stage).
func (r *previewMemoryRepository) nextEventNoLocked(previewID id.UUID, versionNo int64) int64 {
	var max int64
	for _, row := range r.events[previewID.String()] {
		if row.VersionNo == versionNo && row.EventNo > max {
			max = row.EventNo
		}
	}
	return max + 1
}

// previewRouteFacts is the release-lineage adapter of the fixture world:
// the acting route of the case connection serves the fixed candidate of
// the fixture employee (the stage word, the profile version 1 pin and the
// relayed admission word — the same route material the casesFixture
// ingests with).
type previewRouteFacts struct {
	entitlement *entitlementFixture
}

func (previewRouteFacts) RouteFacts(_ context.Context, _ tenancy.Context, connectionID id.UUID) (employeeAPI.RouteFacts, error) {
	// The route material of the fixture family: BOTH fixture connections
	// (the phone and the chat) serve the same acting candidate — the
	// release lineage pin of the freshness derivation reads the FIRST
	// conversation of the case, whichever channel opened it.
	if connectionID != fixtureConnectionID && connectionID != fixtureChatConnectionID {
		return employeeAPI.RouteFacts{}, fmt.Errorf("the fixture route facts serve the fixture connections only, got %s", connectionID)
	}
	return employeeAPI.RouteFacts{
		ConnectionID: connectionID,
		Candidates: []employeeAPI.RouteCandidate{{
			EmployeeID: fixtureEmployeeID, DecisionNo: 1, Stage: "synthetic", ProfileVersion: 1,
		}},
	}, nil
}

// previewControlRoute adapts the controls fixture world onto the
// preparation-route port of Story 3.4 (the fixtureControlReader precedent
// of 3-3: the REAL control service over the memory world, the route keys
// of the preparation — the tenant + connector + autonomy class points).
type previewControlRoute struct {
	controls *controlsFixture
}

var _ actionPort.ControlEffectiveReader = previewControlRoute{}

func (r previewControlRoute) EffectiveFor(ctx context.Context, tenantID id.UUID, route actionPort.ControlRoute) (actionPort.ControlEffective, error) {
	effective, err := r.controls.service.EffectiveFor(ctx, domain.Principal{ID: principalID, StepUpVerified: true}, tenantID,
		[]controlDomain.ScopeKey{
			{Kind: controlDomain.ScopeKindTenant, Ref: tenantID.String()},
			{Kind: controlDomain.ScopeKindConnector, Ref: route.ConnectorID},
			{Kind: controlDomain.ScopeKindAutonomyClass, Ref: route.AutonomyClass},
		})
	if err != nil {
		// The unreadable control of the fixture world: the fail-closed
		// block (the empty mode word — the policy gate blocks with the
		// control_mode reason, never a guess).
		return actionPort.ControlEffective{}, err
	}
	reading := actionPort.ControlEffective{Mode: effective.Mode, Reason: effective.Reason}
	for _, control := range effective.Influencing {
		if controlDomain.SafeReadMode(control.Mode) == effective.Mode && control.Generation > reading.Generation {
			reading.Generation = control.Generation
		}
	}
	return reading, nil
}

// previewMembership is the member-facts source of the preparation contour
// over the fixture family: the owner, the second operator and the
// operations-only member (the honest 403 row of the rights gate). The
// Authorize semantics live on the embedded actionsMembership (the third
// member answers ErrForbidden — the member exists, the scope does not).
type previewMembership struct {
	actionsMembership
}

func (previewMembership) MembersFacts(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error) {
	owner := principalID
	second := secondOperatorID
	ops := operationsOperatorID
	return []domain.Membership{
		{TenantID: tenantID, PrincipalID: &owner, Roles: []domain.Role{domain.Owner}, Status: "active", Version: 1},
		{TenantID: tenantID, PrincipalID: &second, Roles: []domain.Role{domain.Owner}, Status: "active", Version: 1},
		{TenantID: tenantID, PrincipalID: &ops, Roles: []domain.Role{domain.Operator}, Status: "active", Version: 1},
	}, nil
}

// fixturePreviewDefinitions builds the dictionary of the z32 vertical: the
// trunk-status query definition of the 3-1 pair (the full live path — its
// capability verifies through the REAL transport path), the manual-only
// definition of the unsafe delivery class (the human_only partition with
// the honest markers and the recovery instruction) and the manual-sms
// definition over the AG-4-guarded mutating capability (the honest
// capability_not_verified block — the mutating capability never verifies).
// The dictionary is admitted at construction over the live catalogs (a
// dishonest fixture panics the server).
func fixturePreviewDefinitions() []actionDomain.ActionDefinition {
	base := func(defID string) actionDomain.ActionDefinition {
		return actionDomain.ActionDefinition{
			ID:      defID,
			Version: "1.0.0",
			Name:    "Матричная проверка подготовки",
			Purpose: "Пройти полную цепочку подготовки по живым источникам мира",
			TypedInputSchema: []actionDomain.FieldSchema{
				{Name: "phone", Label: "Телефон", Type: actionDomain.FieldText, Required: true, Description: "Номер в формате E.164"},
				{Name: "template_url", Label: "Ссылка на шаблон", Type: actionDomain.FieldURL, Secret: true, Description: "Секретное поле — не проецируется"},
			},
			TypedOutputSchema:  []actionDomain.FieldSchema{{Name: "status", Label: "Статус", Type: actionDomain.FieldText}},
			PackRef:            actionDomain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
			RequiredCapability: actionDomain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "query.trunk-status"},
			OwningSystem:       "erp",
			Conditions: actionDomain.Conditions{
				RiskClass:                "low",
				AutonomyClass:            "assisted",
				AllowedConversationModes: []string{"unclassified"},
				AllowedRoles:             []string{"owner"},
				RequiredFacts:            []actionDomain.FactRequirement{{MaterialField: "order_status", Freshness: "fresh"}},
				Confirmation:             "not_required",
				RequiredGates:            []string{"OQ-7B"},
				TimeoutMS:                5000,
				MaterialityClass:         "immaterial",
				ConsumptionRule:          "unlimited",
				AllowedStages:            []string{"synthetic", "closed_account_test"},
			},
			DeliveryContract: actionDomain.DeliveryContract{
				CapabilityClass:       actionDomain.CapabilityQueryReconcilable,
				IdempotencyScope:      "не требуется (read-only)",
				TerminalEvidence:      "query_result",
				ReconciliationQuery:   "query.trunk-status",
				CancelCompensationSem: "не применимо (read-only)",
			},
		}
	}
	manualOnly := base("notify.manual-only")
	manualOnly.Name = "Ручная отмена созвона"
	manualOnly.Purpose = "Отменить созвон вручную — unsafe-класс доставки остаётся в человеческом контуре"
	manualOnly.DeliveryContract.CapabilityClass = actionDomain.CapabilityUnsafe
	manualOnly.Conditions.Confirmation = "required"
	manualOnly.Conditions.ConsumptionRule = "single_use"
	manualOnly.DeliveryContract.CancelCompensationSem = "вручную: созвон с клиентом и отметка в журнале"

	manualSMS := base("notify.manual-sms")
	manualSMS.Name = "Ручная SMS об отмене"
	manualSMS.Purpose = "Уведомить клиента вручную — мутирующая capability под гейтом AG-4"
	manualSMS.RequiredCapability.CapabilityID = "command.send-sms"
	manualSMS.Conditions.Confirmation = "required"
	manualSMS.Conditions.ConsumptionRule = "single_use"
	manualSMS.DeliveryContract.CapabilityClass = actionDomain.CapabilityProviderIdempotent
	manualSMS.DeliveryContract.IdempotencyScope = "connector-ack"
	manualSMS.DeliveryContract.TerminalEvidence = "delivery-receipt"
	manualSMS.DeliveryContract.ReconciliationQuery = "query.trunk-status"
	manualSMS.DeliveryContract.CancelCompensationSem = "не компенсируется — отказ до подтверждения"

	trunk := base("query.trunk-status")
	trunk.Name = "Статус телефонной линии"
	trunk.Purpose = "Узнать текущий статус телефонной линии перед созвоном"

	return []actionDomain.ActionDefinition{trunk, manualOnly, manualSMS}
}

// The AG-4 adapters of the preview world: the synthetic reader and the
// scriptable dry-runner (the toggles of the fixture endpoints).
var (
	previewReader  = owningsystem.New()
	previewRunner  = dryrunner.New()
	previewIDsPool = &previewIDs{}
)

// newPreviewWorld wires the z32 dictionary over the REAL app service, the
// in-memory repository, the LIVE fixture fact sources and the AG-4 test
// adapters; the admission walk runs at construction (a dishonest
// dictionary panics the server before the first request). The world is
// built INSIDE main(): the fact sources are the fixture instances the
// server assembles there (a package-level initializer could not see
// them).
func newPreviewWorld(casesWorld *casesFixture, controls *controlsFixture, entitlement *entitlementFixture,
	readiness *readinessFixture, connectors *connectorFixture) *actionApp.Service {
	defs := fixturePreviewDefinitions()
	packs, caps := packresolver.New(), capabilityresolver.New()
	if err := actionDomain.CatalogIntegrity(defs); err != nil {
		panic(err)
	}
	for _, def := range defs {
		if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
			panic(err)
		}
	}
	return actionApp.New(previewMembership{}, packs, caps, defs).
		WithPreviews(newPreviewMemoryRepository(), previewControlRoute{controls: controls},
			previewCasesMaterial{cases: casesWorld}, previewMembership{}, entitlement,
			readiness, connectors, previewRouteFacts{}, previewIDsPool).
		WithPreviewReader(previewReader).
		WithPreviewDryRunner(previewRunner)
}

// previewCasesMaterial adapts the cases fixture material onto the narrow
// case port (the REAL fixture view over the fixture case world — the CaseView
// and FactsView of the journeys' case material).
type previewCasesMaterial struct {
	cases *casesFixture
}

func (m previewCasesMaterial) CaseView(ctx context.Context, actor domain.Principal, tenant, caseID id.UUID) (casesAPI.CaseView, error) {
	return m.cases.CaseView(ctx, actor, tenant, caseID)
}

func (m previewCasesMaterial) FactsView(ctx context.Context, actor domain.Principal, tenant, caseID id.UUID) (casesAPI.FactsView, error) {
	return m.cases.FactsView(ctx, actor, tenant, caseID)
}
