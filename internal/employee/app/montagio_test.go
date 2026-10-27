package app

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/port"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
)

// TestMontagioAdmissionPerformsNoWrites pins the honesty core of spec 5-1:
// the admission path reads every live fact source and performs no writes
// (the store counters stay at zero across repeated calls).
func TestMontagioAdmissionPerformsNoWrites(t *testing.T) {
	world := newPilotWorld(t, domain.MontagioRequiredPackID, "1.0.0", nil, nil)
	ctx := context.Background()
	for call := 0; call < 3; call++ {
		if _, err := world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant); err != nil {
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

// TestMontagioAdmissionPackPinResolution pins the «Услуги» pin: the Oleg
// pack never pins the Montagio pilot, the old pin is below the minimum and
// the honest pin passes the fail-closed prerequisite.
func TestMontagioAdmissionPackPinResolution(t *testing.T) {
	ctx := context.Background()
	world := newPilotWorld(t, domain.MontagioRequiredPackID, "1.0.0", nil, nil)
	view, err := world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Report.PackPinned {
		t.Fatalf("pack word = %q", view.Report.PackWord)
	}
	if view.Report.PackWord != "Domain Pack «Услуги» прикреплён к запечатанному контуру: montagio-services@1.0.0" {
		t.Fatalf("the pinned pack word carries the pin, got %q", view.Report.PackWord)
	}
	// The Oleg «Автозапчасти» pack never pins the Montagio pilot.
	world = newPilotWorld(t, domain.PilotRequiredPackID, "6.0.0", nil, nil)
	view, err = world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
	if err != nil {
		t.Fatal(err)
	}
	if view.Report.PackPinned {
		t.Fatal("the «Автозапчасти» pack never satisfies the «Услуги» pin")
	}
	for _, route := range view.Report.Routes {
		if route.Stage != domain.MontagioStageCandidate {
			t.Fatalf("route %s stage = %s, want candidate (pack fail-closed)", route.RouteID, route.Stage)
		}
	}
}

// TestMontagioAdmissionAggregatesInstancesAcrossEmployees pins the tenant
// union: a PASS recorded by any employee aggregate of the tenant closes the
// tenant-level gates of the Montagio routes.
func TestMontagioAdmissionAggregatesInstancesAcrossEmployees(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	instances := []domain.GateInstance{
		{TenantID: pilotTenant, EmployeeID: pilotSecondEmployee, InstanceID: pilotActor,
			DefinitionID: "OQ-7A", State: domain.GateStatePass,
			ApprovedBy:   []domain.GateApproval{{ParticipantID: pilotActor, Role: domain.GateRoleProductOwner}, {ParticipantID: pilotSecondEmployee, Role: domain.GateRoleMontagioOperations}},
			EvidenceRefs: []string{"evidence://oq-7a/cohort"}, ActorID: pilotActor, RecordedAt: at},
		{TenantID: pilotTenant, EmployeeID: pilotSecondEmployee, InstanceID: pilotEmployee,
			DefinitionID: "OQ-8", State: domain.GateStatePass,
			ApprovedBy:   []domain.GateApproval{{ParticipantID: pilotActor, Role: domain.GateRoleMontagioOperations}},
			EvidenceRefs: []string{"evidence://oq-8/fallback-owners"}, ActorID: pilotActor, RecordedAt: at},
		{TenantID: pilotTenant, EmployeeID: pilotEmployee, InstanceID: pilotActor,
			DefinitionID: "AG-1", State: domain.GateStatePass,
			ApprovedBy:   []domain.GateApproval{{ParticipantID: pilotActor, Role: domain.GateRoleTechnicalLead}, {ParticipantID: pilotSecondEmployee, Role: domain.GateRoleOperations}},
			EvidenceRefs: []string{"evidence://ag-1/load"}, ActorID: pilotActor, RecordedAt: at},
		{TenantID: pilotTenant, EmployeeID: pilotEmployee, InstanceID: pilotSecondEmployee,
			DefinitionID: "AG-2", State: domain.GateStatePass,
			ApprovedBy:   []domain.GateApproval{{ParticipantID: pilotActor, Role: domain.GateRoleProductOwner}, {ParticipantID: pilotSecondEmployee, Role: domain.GateRoleUXQA}},
			EvidenceRefs: []string{"evidence://ag-2/wcag"}, ActorID: pilotActor, RecordedAt: at},
		{TenantID: pilotTenant, EmployeeID: pilotEmployee, InstanceID: pilotActor,
			DefinitionID: "AG-4", State: domain.GateStatePass,
			ApprovedBy:   []domain.GateApproval{{ParticipantID: pilotActor, Role: domain.GateRoleTechnicalLead}, {ParticipantID: pilotSecondEmployee, Role: domain.GateRoleSecurity}},
			EvidenceRefs: []string{"evidence://ag-4/dependencies"}, ActorID: pilotActor, RecordedAt: at},
	}
	world := newPilotWorld(t, domain.MontagioRequiredPackID, "1.0.0", nil, instances)
	view, err := world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
	if err != nil {
		t.Fatal(err)
	}
	inbox := montagioRouteOf(t, view, "human_inbox")
	if inbox.Stage != domain.MontagioStageControlled {
		t.Fatalf("human_inbox stage = %s, want controlled (controlled gates PASS, live gates stay open — the four-stage ladder)", inbox.Stage)
	}
	// Closing the live gates on the SECOND aggregate admits live.
	instances = append(instances,
		domain.GateInstance{TenantID: pilotTenant, EmployeeID: pilotSecondEmployee, InstanceID: pilotActor,
			DefinitionID: "OQ-7B", State: domain.GateStatePass,
			ApprovedBy:   []domain.GateApproval{{ParticipantID: pilotActor, Role: domain.GateRoleProductOwner}, {ParticipantID: pilotSecondEmployee, Role: domain.GateRoleMontagioOperations}},
			EvidenceRefs: []string{"evidence://oq-7b/baseline"}, ActorID: pilotActor, RecordedAt: at.Add(time.Minute)},
		domain.GateInstance{TenantID: pilotTenant, EmployeeID: pilotSecondEmployee, InstanceID: pilotEmployee,
			DefinitionID: "OQ-14", State: domain.GateStatePass,
			ApprovedBy:   []domain.GateApproval{{ParticipantID: pilotActor, Role: domain.GateRoleProductOwner}, {ParticipantID: pilotSecondEmployee, Role: domain.GateRoleOperationsQA}},
			EvidenceRefs: []string{"evidence://oq-14/thresholds"}, ActorID: pilotActor, RecordedAt: at.Add(time.Minute)})
	world = newPilotWorld(t, domain.MontagioRequiredPackID, "1.0.0", nil, instances)
	view, err = world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
	if err != nil {
		t.Fatal(err)
	}
	if inbox := montagioRouteOf(t, view, "human_inbox"); inbox.Stage != domain.MontagioStageLive {
		t.Fatalf("human_inbox stage = %s, want live (the union of the tenant's instances closes the full conjunction)", inbox.Stage)
	}
}

// TestMontagioAdmissionCapabilityFactsFlowThrough pins the connector facts
// composition: verified Montagio capabilities on any current connection
// raise the routes to verified while the gates stay open.
func TestMontagioAdmissionCapabilityFactsFlowThrough(t *testing.T) {
	ctx := context.Background()
	verified := []connectorAPI.ConnectionFacts{{
		ConnectionID: pilotEmployee, ConnectorID: "novofon-sip", ConnectorType: "SIP/telephony",
		ContractVersion: "1.1.0", AdapterBuild: "novofon/1.1.0", Environment: "test", Status: "connected_unverified",
		Capabilities: []connectorDomain.CapabilityReport{
			{ID: "query.inbound-calls", State: connectorDomain.CapStateVerified},
			{ID: "query.outbound-calls", State: connectorDomain.CapStateVerified},
			{ID: "query.media-codec", State: connectorDomain.CapStateVerified},
			{ID: "query.streaming", State: connectorDomain.CapStateVerified},
			{ID: "query.dtmf-support", State: connectorDomain.CapStateVerified},
			{ID: "query.transfer-bridge", State: connectorDomain.CapStateVerified},
			{ID: "query.callback-state", State: connectorDomain.CapStateVerified},
			{ID: "query.caller-id", State: connectorDomain.CapStateVerified},
			{ID: "query.recording", State: connectorDomain.CapStateVerified},
			{ID: "query.webhooks", State: connectorDomain.CapStateVerified},
			{ID: "query.delivery-states", State: connectorDomain.CapStateVerified},
			{ID: "query.failure-states", State: connectorDomain.CapStateVerified},
		},
	}}
	world := newPilotWorld(t, domain.MontagioRequiredPackID, "1.0.0", verified, nil)
	view, err := world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
	if err != nil {
		t.Fatal(err)
	}
	voice := montagioRouteOf(t, view, "novofon_voice")
	if voice.Stage != domain.MontagioStageVerified {
		t.Fatalf("novofon_voice stage = %s, want verified (capabilities proven, gates open)", voice.Stage)
	}
	// A stale fact on another connection does not displace the verified
	// proof of the tenant.
	staleToo := append(append([]connectorAPI.ConnectionFacts{}, verified...), connectorAPI.ConnectionFacts{
		ConnectionID: pilotSecondEmployee, ConnectorID: "novofon-sip", ConnectorType: "SIP/telephony",
		ContractVersion: "1.1.0", AdapterBuild: "novofon/1.1.0", Environment: "test", Status: "connected_unverified",
		Capabilities: []connectorDomain.CapabilityReport{{ID: "query.inbound-calls", State: connectorDomain.CapStateStale, StateReason: "ротация секрета"}},
	})
	world = newPilotWorld(t, domain.MontagioRequiredPackID, "1.0.0", staleToo, nil)
	view, err = world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
	if err != nil {
		t.Fatal(err)
	}
	if voice := montagioRouteOf(t, view, "novofon_voice"); voice.Stage != domain.MontagioStageVerified {
		t.Fatalf("novofon_voice stage = %s, want verified (a live proof on any connection holds)", voice.Stage)
	}
}

// TestMontagioAdmissionViewScopeRights pins the reused rights: the review
// scope views honestly, a member without a scope gets the honest 403 and a
// non-member the plain 404 (membership before the material).
func TestMontagioAdmissionViewScopeRights(t *testing.T) {
	ctx := context.Background()
	store := &pilotStore{drafts: []domain.Draft{{TenantID: pilotTenant, EmployeeID: pilotEmployee, OwnerID: pilotActor}}}
	connectors := &pilotConnectors{}
	service := newPilotService(store, connectors, pilotIdentity{member: true, scopes: []identityDomain.Scope{identityDomain.Review}})
	view, err := service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
	if err != nil {
		t.Fatal(err)
	}
	if view.CanManage || view.ViewScope != api.ReadinessScopeView {
		t.Fatalf("review scope: can_manage=%v scope=%s", view.CanManage, view.ViewScope)
	}
	service = newPilotService(store, connectors, pilotIdentity{member: true})
	if _, err := service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant); !errors.Is(err, domain.ErrReadinessForbidden) {
		t.Fatalf("member without scope = %v", err)
	}
	service = newPilotService(store, connectors, pilotIdentity{notMember: true})
	if _, err := service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("non-member = %v", err)
	}
}

// TestMontagioAdmissionIsIdempotent pins the byte-stable derivation: an
// unchanged world yields the same admission on the wire.
func TestMontagioAdmissionIsIdempotent(t *testing.T) {
	verified := []connectorAPI.ConnectionFacts{{
		ConnectionID: pilotEmployee, ConnectorID: "montagio-web", ConnectorType: "chat/inbox",
		ContractVersion: "1.0.0", AdapterBuild: "montagio/1.0.0", Environment: "test", Status: "connected_unverified",
		Capabilities: []connectorDomain.CapabilityReport{{ID: "query.contract-schema", State: connectorDomain.CapStateVerified}},
	}}
	world := newPilotWorld(t, domain.MontagioRequiredPackID, "1.0.0", verified, nil)
	ctx := context.Background()
	firstView, err := world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
	if err != nil {
		t.Fatal(err)
	}
	first, err := json.Marshal(firstView)
	if err != nil {
		t.Fatal(err)
	}
	secondView, err := world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
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

// TestMontagioAdmissionAddsNoPortMethods is the reflect probe of the slice:
// the admission is read-time over the REUSED ports only — spec 5-1 adds no
// persistence method to any port (a new method would be a new write path).
func TestMontagioAdmissionAddsNoPortMethods(t *testing.T) {
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
			t.Fatalf("%s carries %d methods, want %d (spec 5-1 adds none — the admission path reads the reused ports only)",
				pin.name, got, pin.count)
		}
	}
}

// TestMontagioAdmissionCarriesTheDictionaries pins the wire dictionaries of
// the service view: the 8 routes in order, the 6-word gate mirror and the
// AC-11 wording dictionary.
func TestMontagioAdmissionCarriesTheDictionaries(t *testing.T) {
	ctx := context.Background()
	world := newPilotWorld(t, domain.MontagioRequiredPackID, "1.0.0", nil, nil)
	view, err := world.service.MontagioAdmission(ctx, identityDomain.Principal{ID: pilotActor}, pilotTenant)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Report.Routes) != 8 {
		t.Fatalf("the view carries the 8-word route dictionary, got %d", len(view.Report.Routes))
	}
	if view.Report.Routes[0].RouteID != "novofon_voice" || view.Report.Routes[7].RouteID != "public_commercial" {
		t.Fatalf("the dictionary order is fixed, got %s…%s", view.Report.Routes[0].RouteID, view.Report.Routes[7].RouteID)
	}
	if len(view.Report.GateIDs) != 6 || view.Report.GateIDs[0] != "OQ-7A" {
		t.Fatalf("the view carries the 6-word Montagio gate dictionary mirror, got %v", view.Report.GateIDs)
	}
	if len(view.Report.WordingWords) != 5 {
		t.Fatalf("the view carries the AC-11 wording dictionary (5 words), got %d", len(view.Report.WordingWords))
	}
}

// montagioRouteOf resolves one route of the view.
func montagioRouteOf(t *testing.T, view api.MontagioAdmissionView, routeID string) domain.MontagioRouteAdmission {
	t.Helper()
	for _, route := range view.Report.Routes {
		if route.RouteID == routeID {
			return route
		}
	}
	t.Fatalf("route %s missing", routeID)
	return domain.MontagioRouteAdmission{}
}
