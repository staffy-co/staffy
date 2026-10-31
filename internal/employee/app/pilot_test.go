package app

import (
        "context"
        "encoding/json"
        "errors"
        "reflect"
        "testing"
        "time"

        "github.com/staffy-co/staffy/internal/employee/api"
        "github.com/staffy-co/staffy/internal/employee/domain"
        "github.com/staffy-co/staffy/internal/employee/port"
        connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
        connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
        identityAPI "github.com/staffy-co/staffy/internal/identity/api"
        identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
        "github.com/staffy-co/staffy/internal/platform/id"
        "github.com/staffy-co/staffy/internal/platform/tenancy"
)

var pilotTenant = id.MustParse("01994f64-6c00-7000-8000-000000000031")
var pilotEmployee = id.MustParse("01994f64-6c00-7000-8000-000000000032")
var pilotActor = id.MustParse("01994f64-6c00-7000-8000-000000000033")
var pilotSecondEmployee = id.MustParse("01994f64-6c00-7000-8000-000000000034")

// pilotStore is the fake of the unified persistence object: the app derives
// the Repository, ReadinessRepository and ReleaseRepository views from the
// SAME store (the postgres precedent) — the fake mirrors that composition.
// The writes are counted (the derivation must never perform one).
type pilotStore struct {
        drafts     []domain.Draft
        records    []port.ManifestRecord
        instances  []domain.GateInstance
        listReads  int
        manifestReads int
        instanceReads int
        writes     int
}

func (r *pilotStore) Create(context.Context, port.CreateParams) (domain.Draft, bool, error) {
        r.writes++
        return domain.Draft{}, false, errors.New("unused")
}
func (r *pilotStore) List(_ context.Context, scope tenancy.Context) ([]domain.Draft, error) {
        if scope.TenantID() != pilotTenant {
                return nil, domain.ErrNotFound
        }
        r.listReads++
        return append([]domain.Draft(nil), r.drafts...), nil
}
func (r *pilotStore) Read(context.Context, tenancy.Context, id.UUID) (domain.Draft, error) {
        r.writes++
        return domain.Draft{}, domain.ErrNotFound
}
func (r *pilotStore) Save(context.Context, port.SaveParams) (domain.Draft, error) {
        r.writes++
        return domain.Draft{}, errors.New("unused")
}
func (r *pilotStore) Pin(context.Context, port.PinParams) (domain.Draft, error) {
        r.writes++
        return domain.Draft{}, errors.New("unused")
}

func (r *pilotStore) SaveGateInstance(context.Context, port.GateInstanceParams) (domain.GateInstance, error) {
        r.writes++
        return domain.GateInstance{}, errors.New("unused")
}
func (r *pilotStore) ListGateInstances(_ context.Context, scope tenancy.Context, employee id.UUID) ([]domain.GateInstance, error) {
        if scope.TenantID() != pilotTenant {
                return nil, domain.ErrNotFound
        }
        r.instanceReads++
        var out []domain.GateInstance
        for _, instance := range r.instances {
                if instance.EmployeeID == employee {
                        out = append(out, instance)
                }
        }
        return out, nil
}
func (r *pilotStore) SaveReleaseAdmission(context.Context, port.AdmissionParams) (domain.ReleaseAdmission, error) {
        r.writes++
        return domain.ReleaseAdmission{}, errors.New("unused")
}
func (r *pilotStore) ListReleaseAdmissions(context.Context, tenancy.Context, id.UUID) ([]domain.ReleaseAdmission, error) {
        return nil, nil
}

func (r *pilotStore) ReadReleaseDraft(context.Context, tenancy.Context, id.UUID) (domain.ReleaseDraft, error) {
        return domain.ReleaseDraft{}, domain.ErrNotFound
}
func (r *pilotStore) SaveReleaseDraft(context.Context, port.DraftParams) (domain.ReleaseDraft, error) {
        r.writes++
        return domain.ReleaseDraft{}, errors.New("unused")
}
func (r *pilotStore) RestoreReleaseDraft(context.Context, port.RestoreParams) (domain.ReleaseDraft, error) {
        r.writes++
        return domain.ReleaseDraft{}, errors.New("unused")
}
func (r *pilotStore) SaveConfigVersion(context.Context, port.VersionParams) (domain.ConfigVersion, error) {
        r.writes++
        return domain.ConfigVersion{}, errors.New("unused")
}
func (r *pilotStore) SaveManifest(context.Context, port.SealParams) (domain.ReleaseManifest, error) {
        r.writes++
        return domain.ReleaseManifest{}, errors.New("unused")
}
func (r *pilotStore) ReadConfigVersion(context.Context, tenancy.Context, id.UUID, int64) (domain.ConfigVersion, error) {
        return domain.ConfigVersion{}, domain.ErrNotFound
}
func (r *pilotStore) ListConfigVersions(context.Context, tenancy.Context, id.UUID) ([]domain.ConfigVersion, error) {
        return nil, nil
}
func (r *pilotStore) ListManifests(_ context.Context, scope tenancy.Context, employee id.UUID) ([]port.ManifestRecord, error) {
        if scope.TenantID() != pilotTenant {
                return nil, domain.ErrNotFound
        }
        r.manifestReads++
        var out []port.ManifestRecord
        for _, record := range r.records {
                if record.Manifest.EmployeeID == employee {
                        out = append(out, record)
                }
        }
        return out, nil
}

// pilotConnectors is the fake narrow connector facts port: the derived
// states only, never secrets.
type pilotConnectors struct {
        facts []connectorAPI.ConnectionFacts
        reads int
}

func (c *pilotConnectors) ConnectionsFacts(context.Context, tenancy.Context) ([]connectorAPI.ConnectionFacts, error) {
        c.reads++
        return c.facts, nil
}

// pilotIdentity authorizes the view scopes of the reused test & release
// surface.
type pilotIdentity struct {
        member    bool
        scopes    []identityDomain.Scope
        notMember bool
}

func (i pilotIdentity) ListMembers(context.Context, identityDomain.Principal, id.UUID) ([]identityDomain.Membership, error) {
        return nil, nil
}
func (i pilotIdentity) InviteMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, string, error) {
        return identityDomain.Membership{}, "", nil
}
func (i pilotIdentity) AcceptInvitation(context.Context, identityDomain.Principal, string, id.UUID) (identityDomain.Membership, error) {
        return identityDomain.Membership{}, nil
}
func (i pilotIdentity) ChangeMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, error) {
        return identityDomain.Membership{}, nil
}
func (i pilotIdentity) RevokeMember(context.Context, identityAPI.MembershipCommand) error { return nil }
func (i pilotIdentity) Authorize(_ context.Context, _ identityDomain.Principal, _ id.UUID, scope identityDomain.Scope) error {
        if i.notMember {
                return identityDomain.ErrNotFound
        }
        for _, granted := range i.scopes {
                if granted == scope {
                        return nil
                }
        }
        if i.member {
                return identityDomain.ErrForbidden
        }
        return identityDomain.ErrNotFound
}

type pilotClock struct{}

func (pilotClock) Now() time.Time { return time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC) }

type pilotIDs struct{}

func (pilotIDs) New(time.Time) (id.UUID, error) { return pilotActor, nil }

// pilotManifestRecord builds one sealed manifest record pinning the pack.
func pilotManifestRecord(t *testing.T, employee id.UUID, packID, packVersion string) port.ManifestRecord {
        t.Helper()
        refs := domain.ConfigRefs{
                Brief:    domain.BriefRef{Status: domain.RefPresent, DraftVersion: 1},
                Behavior: domain.BehaviorRef{Status: domain.RefPresent, Version: 1},
        }
        if packID != "" {
                refs.Pack = domain.PackRef{Status: domain.RefPresent, ID: packID, Version: packVersion}
        }
        config := domain.ConfigVersion{
                TenantID: pilotTenant, EmployeeID: employee, VersionID: pilotActor, Version: 1,
                Description: "pilot fixture", ActorID: pilotActor,
                CreatedAt: time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC), Refs: refs,
        }
        content := domain.BuildManifestContent(config, nil, nil)
        raw, err := domain.CanonicalManifestJSON(content)
        if err != nil {
                t.Fatal(err)
        }
        digest, err := domain.ManifestDigest(content)
        if err != nil {
                t.Fatal(err)
        }
        return port.ManifestRecord{
                Manifest: domain.ReleaseManifest{
                        TenantID: pilotTenant, EmployeeID: employee, ManifestID: pilotSecondEmployee,
                        ConfigVersion: 1, SchemaVersion: content.SchemaVersion, Status: domain.QuarantineNone,
                        Digest: digest, ActorID: pilotActor, SealedAt: time.Date(2026, 10, 9, 9, 30, 0, 0, time.UTC),
                        Content: content,
                },
                Payload: raw,
        }
}

// newPilotService assembles the service over the fakes.
func newPilotService(store *pilotStore, connectors *pilotConnectors, identity pilotIdentity) *Service {
        service := New(store, identity, pilotClock{}, pilotIDs{})
        service.connectors = connectors
        return service
}

// pilotWorld is the assembled fixture of one test world.
type pilotWorld struct {
        service    *Service
        store      *pilotStore
        connectors *pilotConnectors
}

func newPilotWorld(t *testing.T, packID, packVersion string, connections []connectorAPI.ConnectionFacts, instances []domain.GateInstance) *pilotWorld {
        t.Helper()
        store := &pilotStore{
                drafts: []domain.Draft{
                        {TenantID: pilotTenant, EmployeeID: pilotEmployee, OwnerID: pilotActor, Status: "active"},
                        {TenantID: pilotTenant, EmployeeID: pilotSecondEmployee, OwnerID: pilotActor, Status: "active"},
                },
                records:   []port.ManifestRecord{pilotManifestRecord(t, pilotEmployee, packID, packVersion)},
                instances: instances,
        }
        connectors := &pilotConnectors{facts: connections}
        service := newPilotService(store, connectors, pilotIdentity{member: true, scopes: []identityDomain.Scope{identityDomain.Configuration}})
        return &pilotWorld{service: service, store: store, connectors: connectors}
}

func TestPilotAdmissionPerformsNoWrites(t *testing.T) {
        world := newPilotWorld(t, domain.PilotRequiredPackID, "2.0.0", nil, nil)
        ctx := context.Background()
        for call := 0; call < 3; call++ {
                if _, err := world.service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant); err != nil {
                        t.Fatalf("call %d: %v", call, err)
                }
        }
        if world.store.writes != 0 {
                t.Fatalf("the admission path performs no writes, got %d", world.store.writes)
        }
        if world.store.listReads == 0 || world.store.manifestReads == 0 || world.store.instanceReads == 0 || world.connectors.reads == 0 {
                t.Fatal("the admission path reads every live fact source")
        }
}

func TestPilotAdmissionPackPinResolution(t *testing.T) {
        ctx := context.Background()
        // The sealed manifest pins auto-parts@2.0.0: the pack contribution passes.
        world := newPilotWorld(t, domain.PilotRequiredPackID, "2.0.0", nil, nil)
        view, err := world.service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
        if err != nil {
                t.Fatal(err)
        }
        if !view.Report.PackPinned {
                t.Fatalf("pack word = %q", view.Report.PackWord)
        }
        // A foreign pack never satisfies the pin.
        world = newPilotWorld(t, "montagio-furniture", "1.0.0", nil, nil)
        view, err = world.service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.Report.PackPinned {
                t.Fatal("a foreign pack never satisfies the auto-parts pin")
        }
        for _, route := range view.Report.Routes {
                if route.Stage != domain.PilotStageCandidate {
                        t.Fatalf("route %s stage = %s, want candidate (pack fail-closed)", route.RouteID, route.Stage)
                }
        }
        // An old auto-parts pin is below the minimum.
        world = newPilotWorld(t, domain.PilotRequiredPackID, "1.0.0", nil, nil)
        view, err = world.service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.Report.PackPinned {
                t.Fatal("auto-parts@1.0.0 is below the required 2.0.0 pin")
        }
}

func TestPilotAdmissionAggregatesInstancesAcrossEmployees(t *testing.T) {
        ctx := context.Background()
        // The gate instances live on the SECOND employee aggregate: the
        // tenant-level derivation reads the history of every employee (the
        // platform AG-2/AG-4 PASS anywhere closes the human_fallback route).
        at := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
        instances := []domain.GateInstance{
                {TenantID: pilotTenant, EmployeeID: pilotSecondEmployee, InstanceID: pilotActor,
                        DefinitionID: "AG-2", State: domain.GateStatePass,
                        ApprovedBy:   []domain.GateApproval{{ParticipantID: pilotActor, Role: domain.GateRoleProductOwner}, {ParticipantID: pilotSecondEmployee, Role: domain.GateRoleUXQA}},
                        EvidenceRefs: []string{"evidence://ag-2/wcag"}, ActorID: pilotActor, RecordedAt: at},
                {TenantID: pilotTenant, EmployeeID: pilotSecondEmployee, InstanceID: pilotEmployee,
                        DefinitionID: "AG-4", State: domain.GateStatePass,
                        ApprovedBy:   []domain.GateApproval{{ParticipantID: pilotActor, Role: domain.GateRoleTechnicalLead}, {ParticipantID: pilotSecondEmployee, Role: domain.GateRoleSecurity}},
                        EvidenceRefs: []string{"evidence://ag-4/dependencies"}, ActorID: pilotActor, RecordedAt: at},
        }
        world := newPilotWorld(t, domain.PilotRequiredPackID, "2.0.0", nil, instances)
        view, err := world.service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
        if err != nil {
                t.Fatal(err)
        }
        fallback := pilotRouteOf(t, view, "human_fallback")
        if fallback.Stage != domain.PilotStageAdmitted {
                t.Fatalf("human_fallback stage = %s, want admitted (the union of the tenant's instances)", fallback.Stage)
        }
        if len(fallback.Blockers) != 0 {
                t.Fatalf("human_fallback blockers = %+v", fallback.Blockers)
        }
}

func TestPilotAdmissionCapabilityFactsFlowThrough(t *testing.T) {
        ctx := context.Background()
        // One synthetic connection with verified SIP queries: the voice route
        // rises to verified (the gates stay open).
        verified := []connectorAPI.ConnectionFacts{{
                ConnectionID: pilotEmployee, ConnectorID: "staffy-synthetic-telephony", ConnectorType: "SIP/telephony",
                ContractVersion: "1.2.0", AdapterBuild: "synthetic/1.2.0", Environment: "test", Status: "connected_unverified",
                Capabilities: []connectorDomain.CapabilityReport{
                        {ID: "query.trunk-status", State: connectorDomain.CapStateVerified},
                        {ID: "query.transfer-bridge", State: connectorDomain.CapStateVerified},
                        {ID: "query.callback-state", State: connectorDomain.CapStateVerified},
                        {ID: "query.dtmf-support", State: connectorDomain.CapStateVerified},
                        {ID: "query.playback-delivery", State: connectorDomain.CapStateVerified},
                        {ID: "query.failure-states", State: connectorDomain.CapStateVerified},
                },
        }}
        world := newPilotWorld(t, domain.PilotRequiredPackID, "2.0.0", verified, nil)
        view, err := world.service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
        if err != nil {
                t.Fatal(err)
        }
        voice := pilotRouteOf(t, view, "voice_sip")
        if voice.Stage != domain.PilotStageVerified {
                t.Fatalf("voice_sip stage = %s, want verified (capabilities proven, gates open)", voice.Stage)
        }
        // A stale fact on another connection does not displace the verified
        // proof of the tenant.
        staleToo := append(append([]connectorAPI.ConnectionFacts{}, verified...), connectorAPI.ConnectionFacts{
                ConnectionID: pilotSecondEmployee, ConnectorID: "staffy-synthetic-telephony", ConnectorType: "SIP/telephony",
                ContractVersion: "1.2.0", AdapterBuild: "synthetic/1.2.0", Environment: "test", Status: "connected_unverified",
                Capabilities: []connectorDomain.CapabilityReport{{ID: "query.trunk-status", State: connectorDomain.CapStateStale, StateReason: "ротация секрета"}},
        })
        world = newPilotWorld(t, domain.PilotRequiredPackID, "2.0.0", staleToo, nil)
        view, err = world.service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
        if err != nil {
                t.Fatal(err)
        }
        if voice := pilotRouteOf(t, view, "voice_sip"); voice.Stage != domain.PilotStageVerified {
                t.Fatalf("voice_sip stage = %s, want verified (a live proof on any connection holds)", voice.Stage)
        }
}

func TestPilotAdmissionViewScopeRights(t *testing.T) {
        ctx := context.Background()
        store := &pilotStore{drafts: []domain.Draft{{TenantID: pilotTenant, EmployeeID: pilotEmployee, OwnerID: pilotActor}}}
        connectors := &pilotConnectors{}
        // review scope: the view is honest, the manage flag is false.
        service := newPilotService(store, connectors, pilotIdentity{member: true, scopes: []identityDomain.Scope{identityDomain.Review}})
        view, err := service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.CanManage || view.ViewScope != api.ReadinessScopeView {
                t.Fatalf("review scope: can_manage=%v scope=%s", view.CanManage, view.ViewScope)
        }
        // A member without any view scope: the honest 403 of the reused surface.
        service = newPilotService(store, connectors, pilotIdentity{member: true})
        if _, err := service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant); !errors.Is(err, domain.ErrReadinessForbidden) {
                t.Fatalf("member without scope = %v", err)
        }
        // A non-member: the plain 404 (membership before the material).
        service = newPilotService(store, connectors, pilotIdentity{notMember: true})
        if _, err := service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant); !errors.Is(err, domain.ErrNotFound) {
                t.Fatalf("non-member = %v", err)
        }
}

func TestPilotAdmissionIsIdempotent(t *testing.T) {
        verified := []connectorAPI.ConnectionFacts{{
                ConnectionID: pilotEmployee, ConnectorID: "staffy-synthetic-telephony", ConnectorType: "SIP/telephony",
                ContractVersion: "1.2.0", AdapterBuild: "synthetic/1.2.0", Environment: "test", Status: "connected_unverified",
                Capabilities: []connectorDomain.CapabilityReport{{ID: "query.trunk-status", State: connectorDomain.CapStateVerified}},
        }}
        world := newPilotWorld(t, domain.PilotRequiredPackID, "2.0.0", verified, nil)
        ctx := context.Background()
        firstView, err := world.service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
        if err != nil {
                t.Fatal(err)
        }
        first, err := json.Marshal(firstView)
        if err != nil {
                t.Fatal(err)
        }
        secondView, err := world.service.PilotAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
        if err != nil {
                t.Fatal(err)
        }
        second, err := json.Marshal(secondView)
        if err != nil {
                t.Fatal(err)
        }
        if string(first) != string(second) {
                t.Fatal("an unchanged world must yield the byte-stable same admission")
        }
}

// TestPilotAdmissionAddsNoPortMethods is the reflect probe of the slice:
// the admission is read-time over the REUSED ports only — spec 4-1 adds no
// persistence method to any port (a new method would be a new write path).
func TestPilotAdmissionAddsNoPortMethods(t *testing.T) {
        pins := []struct {
                name  string
                port  any
                count int
        }{
                {"ReadinessRepository", (*port.ReadinessRepository)(nil), 4},
                {"ReleaseRepository", (*port.ReleaseRepository)(nil), 8},
                {"Repository", (*port.Repository)(nil), 5},
                {"ConnectorFacts", (*port.ConnectorFacts)(nil), 1},
        }
        for _, pin := range pins {
                if got := reflect.TypeOf(pin.port).Elem().NumMethod(); got != pin.count {
                        t.Fatalf("%s carries %d methods, want %d (spec 4-1 adds none — the admission path reads the reused ports only)",
                                pin.name, got, pin.count)
                }
        }
}

// pilotRouteOf resolves one route of the view.
func pilotRouteOf(t *testing.T, view api.PilotAdmissionView, routeID string) domain.PilotRouteAdmission {
        t.Helper()
        for _, route := range view.Report.Routes {
                if route.RouteID == routeID {
                        return route
                }
        }
        t.Fatalf("route %s missing", routeID)
        return domain.PilotRouteAdmission{}
}
