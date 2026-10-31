//go:build integration

package integration_test

// employee_montagio_admission_test.go — Story 5-1 (Epic 5): the LIVE
// integration matrix of the Montagio Admission Registry over the reused
// readiness world of 1-13/1-14 (real PG, real identity, real connector
// service with the synthetic verifier, the real app service with the wired
// ConnectorFacts port). The derivation is READ-TIME: no row of this matrix
// writes a Montagio record — the only writes are the reused contours (the
// pack pin switch with the confirmed impact, the release chain, the gate
// transitions of the existing test & release surface).
//
// The matrix rows (the spec's I/O & edge-case matrix, live):
//   row-1  a bare tenant (an employee draft, no sealed manifest): every
//          route honestly blocked by the missing «Услуги» pack pin;
//   row-2  the reused world as-is: its sealed manifests pin auto-parts@
//          1.0.0 — the Oleg pack never pins the Montagio pilot;
//   row-3  the montagio-services@1.0.0 pin sealed live: the pack
//          contribution passes, the connector routes stay candidates with
//          the honest adapter words, the Staffy-own human_inbox walks to
//          verified with the gates blocking by exact words;
//   row-4  OQ-18 PASS (the existing gate contour): avito_candidate stays
//          the eternal candidate — the cap never lifts;
//   row-5  OQ-7A + OQ-8 + AG-1 + AG-2 + AG-4 PASS: human_inbox walks the
//          controlled stage while the live gates stay open (controlled NOT
//          live); protected_web_continuation stays verified — OQ-11 blocks
//          per channel route by the exact word;
//   row-6  OQ-11 route-scoped: web_chat:in PASS alone keeps the route
//          verified with the outbound word; the full pair lifts it to
//          controlled (a tenant-level fact never leaks into a route scope);
//   row-7  the live conjunction: OQ-7B PASS alone keeps both routes
//          controlled (OQ-14 open); + OQ-14 PASS walks them to live;
//   row-8  public_commercial: OQ-9 + OQ-10 + OQ-13A PASS never lifts the
//          capped candidate (the Order/payment machines are future slices);
//   row-9  the expired evidence: the OQ-11 web_chat:in PASS expires — only
//          protected_web_continuation drops (verified), human_inbox stays
//          live (a route-scoped fact never extends to other routes);
//   row-10 the honest refusals: a foreign principal gets the plain 404
//          before any material, an operations-only member the honest 403;
//   row-11 the derivation is idempotent: two reads of the unchanged world
//          answer the byte-stable same JSON;
//   row-12 the import-graph probe (the production root imports no adapter),
//          the live dictionary mirrors against the domain words and the
//          live CHECK-constraint mirror of migration 00014 (20 words).
//
// The slot registry (the retro-4-3 discipline): the connector matrix owns
// the e201-e204 UUID prefixes, the Oleg pilot 4-1 owns seeds 383/384 — the
// Montagio matrix owns the seed pair 385/386 as the e2xx slot family of
// Epic 5 (the free range); the correlation IDs of the walk live in the
// e205+ hex range; the live identity slot is the ID sequence 13900 with
// the member suffixes bf01/bf02 (the pilot owns 13800/be01-be02, dispatch
// 13600/b8xx, reconcile 13700/b9xx).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	identityApp "github.com/staffy-co/staffy/internal/identity/app"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	identityStore "github.com/staffy-co/staffy/internal/identity/store"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// montagioRouteOf resolves one route of the live view.
func montagioRouteOf(t *testing.T, view employeeAPI.MontagioAdmissionView, routeID string) employeeDomain.MontagioRouteAdmission {
	t.Helper()
	for _, route := range view.Report.Routes {
		if route.RouteID == routeID {
			return route
		}
	}
	t.Fatalf("route %s missing from the live view", routeID)
	return employeeDomain.MontagioRouteAdmission{}
}

// montagioBlockerOf resolves one blocker of a route.
func montagioBlockerOf(t *testing.T, route employeeDomain.MontagioRouteAdmission, source, blockerID string) employeeDomain.PilotBlocker {
	t.Helper()
	for _, blocker := range route.Blockers {
		if blocker.Source == source && blocker.ID == blockerID {
			return blocker
		}
	}
	t.Fatalf("blocker %s/%s missing from route %s", source, blockerID, route.RouteID)
	return employeeDomain.PilotBlocker{}
}

// montagioSealServicesPack pins the «Услуги» Domain Pack on the world
// employee (the live pack switch from auto-parts with the confirmed
// impact) and seals a fresh manifest carrying the pin through the real
// release chain (the assemble, the version, the seal).
func montagioSealServicesPack(t *testing.T, w *readinessWorld, employee id.UUID) {
	t.Helper()
	ctx := context.Background()
	// The current employee draft version (the pin command is CAS).
	current, err := w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000e205"),
		ExpectedVersion: current.Version, PackID: employeeDomain.MontagioRequiredPackID, PackVersion: "1.0.0",
		Variants:  map[string]any{"engineering_plumbing": true, "heating": false},
		Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	// The release chain over the switched refs (the live revision walk).
	view, err := w.employee.ReleaseView(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	revision := int64(0)
	if view.Draft != nil {
		revision = view.Draft.Revision
	}
	draft := w.assemble(t, employee, revision)
	version := w.createVersion(t, employee, draft.Revision, "Пин услуг Montagio 1.0.0")
	if _, err := w.seal(t, employee, version.Version, draft.Revision+1); err != nil {
		t.Fatal(err)
	}
}

// montagioPassGate walks the existing gate contour to one tenant-level PASS
// of the world employee: open → in_test → pass with the full approver
// discipline (the roles covered by distinct principals, the evidence
// references and the sha256 hash). The correlation IDs of the walk live in
// the e2xx range of the slot registry.
func montagioPassGate(t *testing.T, w *readinessWorld, employee id.UUID, definitionID string, roles ...string) {
	t.Helper()
	ctx := context.Background()
	approvers := []employeeDomain.GateApproval{}
	for index, role := range roles {
		participant := w.owner.ID
		if index > 0 {
			participant = w.backup
		}
		approvers = append(approvers, employeeDomain.GateApproval{ParticipantID: participant, Role: role})
	}
	opened, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000e206"),
		DefinitionID:  definitionID, TargetState: employeeDomain.GateStateOpen,
	})
	if err != nil {
		t.Fatal(err)
	}
	inTest, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000e207"),
		DefinitionID:  definitionID, TargetState: employeeDomain.GateStateInTest,
		Supersedes: &opened.InstanceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000e208"),
		DefinitionID:  definitionID, TargetState: employeeDomain.GateStatePass,
		Supersedes: &inTest.InstanceID, ApprovedBy: approvers,
		EvidenceRefs: []string{"evidence://" + strings.ToLower(definitionID) + "/montagio-pass"},
		EvidenceHash: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatal(err)
	}
}

// montagioPassOQ11Route walks the route-scoped OQ-11 contour for one exact
// channel route: open → in_test → pass with the SecurityPrivacy +
// TechnicalLead approver pair (the registry canon of the definition).
func montagioPassOQ11Route(t *testing.T, w *readinessWorld, employee id.UUID, channel, direction string) id.UUID {
	t.Helper()
	ctx := context.Background()
	route := employeeDomain.Route{Channel: channel, Direction: direction}
	approvers := []employeeDomain.GateApproval{
		{ParticipantID: w.owner.ID, Role: employeeDomain.GateRoleSecurityPrivacy},
		{ParticipantID: w.backup, Role: employeeDomain.GateRoleTechnicalLead},
	}
	opened, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000e209"),
		DefinitionID:  "OQ-11", TargetState: employeeDomain.GateStateOpen, Route: &route,
	})
	if err != nil {
		t.Fatal(err)
	}
	inTest, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000e20a"),
		DefinitionID:  "OQ-11", TargetState: employeeDomain.GateStateInTest, Route: &route,
		Supersedes: &opened.InstanceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	passed, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000e20b"),
		DefinitionID:  "OQ-11", TargetState: employeeDomain.GateStatePass, Route: &route,
		Supersedes: &inTest.InstanceID, ApprovedBy: approvers,
		EvidenceRefs: []string{fmt.Sprintf("evidence://oq-11/%s/%s/montagio-pass", channel, direction)},
		EvidenceHash: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	return passed.InstanceID
}

// montagioExpireInstance walks the existing contour to the honest expiry of
// one settled instance (pass → expired with the named trigger): the record
// stays append-only, the acting PASS disappears from the derivation.
func montagioExpireInstance(t *testing.T, w *readinessWorld, employee id.UUID, instance id.UUID, route *employeeDomain.Route) {
	t.Helper()
	ctx := context.Background()
	if _, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000e20c"),
		DefinitionID:  "OQ-11", TargetState: employeeDomain.GateStateExpired, Route: route,
		Supersedes:    &instance,
		ExpiryTrigger: "данные consent отозваны владельцем — доказательство перестало действовать",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMontagioAdmissionMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()

	// row-1: a bare tenant (an employee draft, no sealed manifest at all) —
	// the fail-closed start of every route, the dictionaries still carried.
	t.Run("row-1 no sealed manifest: the services pack pin blocks every route", func(t *testing.T) {
		bare := newReleaseWorld(t, 386)
		bareEmployee := bare.createEmployee(t, false)
		view, err := bare.employee.MontagioAdmission(ctx, bare.owner, bare.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if view.Report.PackPinned {
			t.Fatal("a tenant without a sealed manifest has no services pack pin")
		}
		if view.Report.PackWord != employeeDomain.MontagioPackMissingWord {
			t.Fatalf("pack word = %q", view.Report.PackWord)
		}
		if len(view.Report.Routes) != 8 || len(view.Report.GateIDs) != 6 || len(view.Report.WordingWords) != 5 {
			t.Fatalf("dictionaries: routes=%d gate_ids=%d wording=%d", len(view.Report.Routes), len(view.Report.GateIDs), len(view.Report.WordingWords))
		}
		for _, route := range view.Report.Routes {
			if route.Stage != employeeDomain.MontagioStageCandidate {
				t.Fatalf("route %s stage = %s, want candidate", route.RouteID, route.Stage)
			}
			blocker := montagioBlockerOf(t, route, "pack", employeeDomain.MontagioRequiredPackID)
			if blocker.Word != employeeDomain.MontagioPackMissingWord {
				t.Fatalf("route %s pack blocker = %q", route.RouteID, blocker.Word)
			}
		}
		_ = bareEmployee
	})

	// The main world: the reused readiness candidate (its manifests pin the
	// Oleg pack auto-parts@1.0.0 — an honest different-pack start).
	w, employee, _ := newReadinessWorld(t, 385)

	// row-2: the reused world as-is — the Oleg pack never pins the Montagio
	// pilot (the wrong-pack fail-closed prerequisite, never an assumed pin).
	t.Run("row-2 auto-parts@1.0.0 never pins the Montagio pilot", func(t *testing.T) {
		view, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if view.Report.PackPinned {
			t.Fatal("the auto-parts pin must never satisfy the «Услуги» pin")
		}
		if view.Report.PackWord != employeeDomain.MontagioPackMissingWord {
			t.Fatalf("pack word = %q", view.Report.PackWord)
		}
		for _, route := range view.Report.Routes {
			if route.Stage != employeeDomain.MontagioStageCandidate {
				t.Fatalf("route %s stage = %s, want candidate (pack fail-closed)", route.RouteID, route.Stage)
			}
		}
	})

	// row-3: seal the montagio-services@1.0.0 pin live; the connector routes
	// stay candidates with the honest adapter words (AG-4: a declaration is
	// not an adapter — the connector service refuses the connection), the
	// Staffy-own human_inbox rises to verified with the gates blocking by
	// the exact words; the capped routes carry their cap words.
	t.Run("row-3 the services pin passes; the adapters stay honestly absent", func(t *testing.T) {
		montagioSealServicesPack(t, w, employee)
		view, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if !view.Report.PackPinned {
			t.Fatalf("the sealed «Услуги» pin must pass: %q", view.Report.PackWord)
		}
		if !strings.Contains(view.Report.PackWord, "montagio-services@1.0.0") {
			t.Fatalf("pack word = %q", view.Report.PackWord)
		}
		// The voice route: the twelve novofon-sip queries stay unproven (the
		// adapter is not implemented — no connection can exist), the honest
		// adapter word names the exact reason and the next step.
		voice := montagioRouteOf(t, view, "novofon_voice")
		if voice.Stage != employeeDomain.MontagioStageCandidate {
			t.Fatalf("novofon_voice stage = %s, want candidate (no live Novofon evidence)", voice.Stage)
		}
		blocker := montagioBlockerOf(t, voice, "capability", "query.inbound-calls")
		if !strings.HasPrefix(blocker.Word, "Novofon-адаптер не реализован (AG-4)") {
			t.Fatalf("novofon blocker word = %q", blocker.Word)
		}
		if !strings.Contains(blocker.Word, "подключения с живым доказательством нет") {
			t.Fatalf("novofon blocker word = %q", blocker.Word)
		}
		if blocker.NextOwner != "реализация адаптера + account-scoped read-only probe после гейтов (AG-4); общая документация Novofon — не доказательство" {
			t.Fatalf("novofon next owner = %q", blocker.NextOwner)
		}
		if len(voice.Capabilities) != 12 {
			t.Fatalf("novofon_voice carries the twelve AC-3 requirements, got %d", len(voice.Capabilities))
		}
		// The web route: the same honest adapter discipline of the Django site.
		web := montagioRouteOf(t, view, "montagio_web")
		if web.Stage != employeeDomain.MontagioStageCandidate {
			t.Fatalf("montagio_web stage = %s, want candidate", web.Stage)
		}
		if blocker := montagioBlockerOf(t, web, "capability", "query.contract-schema"); !strings.HasPrefix(blocker.Word, "адаптер сайта Montagio (Django) не реализован (AG-4)") {
			t.Fatalf("montagio_web blocker word = %q", blocker.Word)
		}
		// The Staffy-own human inbox: no capabilities to prove, the controlled
		// gates stay open — the honest verified stage with the exact words.
		inbox := montagioRouteOf(t, view, "human_inbox")
		if inbox.Stage != employeeDomain.MontagioStageVerified {
			t.Fatalf("human_inbox stage = %s, want verified (no capabilities, gates open)", inbox.Stage)
		}
		if len(inbox.Capabilities) != 0 {
			t.Fatalf("human_inbox carries no capability requirements, got %d", len(inbox.Capabilities))
		}
		oq7a := montagioBlockerOf(t, inbox, "gate", "OQ-7A")
		if !strings.HasPrefix(oq7a.Word, "OQ-7A missing: план контролируемой когорты Montagio (UJ-M1/UJ-M2) не предзарегистрирован") {
			t.Fatalf("OQ-7A word = %q", oq7a.Word)
		}
		if oq7a.SafeState != "simulation only; ни один маршрут не подменяет другой" {
			t.Fatalf("OQ-7A safe state = %q", oq7a.SafeState)
		}
		if oq7a.NextOwner != "Product Owner + Montagio Operations" {
			t.Fatalf("OQ-7A next owner = %q", oq7a.NextOwner)
		}
		// The capped routes carry their cap words beside the stage (never hidden).
		avito := montagioRouteOf(t, view, "avito_candidate")
		capBlocker := montagioBlockerOf(t, avito, "cap", "avito_candidate")
		if !strings.HasPrefix(capBlocker.Word, "официальный разрешённый способ интеграции не доказан") {
			t.Fatalf("avito cap word = %q", capBlocker.Word)
		}
		if len(avito.FenceWords) != 1 || !strings.Contains(avito.FenceWords[0], "Avito-деклараций в каталоге нет") {
			t.Fatalf("avito fence words = %+v", avito.FenceWords)
		}
		public := montagioRouteOf(t, view, "public_commercial")
		if blocker := montagioBlockerOf(t, public, "cap", "public_commercial"); !strings.HasPrefix(blocker.Word, "бизнес-машины Order/payment Montagio не построены") {
			t.Fatalf("public_commercial cap word = %q", blocker.Word)
		}
		// The voice route does not inherit the synthetic telephony proof of
		// the world (its six verified SIP queries are not the novofon twelve).
		for _, candidate := range voice.Capabilities {
			if candidate.State == "verified" {
				t.Fatalf("novofon capability %s must stay unproven in this world", candidate.ID)
			}
		}
	})

	// row-4: OQ-18 PASS through the existing gate contour — the eternal
	// candidate never lifts: the cap rides beside any gate state.
	t.Run("row-4 OQ-18 PASS: avito_candidate stays the eternal candidate", func(t *testing.T) {
		montagioPassGate(t, w, employee, "OQ-18", employeeDomain.GateRoleProductOwner, employeeDomain.GateRolePartnershipsArchitecture)
		view, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		avito := montagioRouteOf(t, view, "avito_candidate")
		if avito.Stage != employeeDomain.MontagioStageCandidate {
			t.Fatalf("avito_candidate stage = %s, want candidate (the cap is eternal this slice)", avito.Stage)
		}
		if blocker := montagioBlockerOf(t, avito, "cap", "avito_candidate"); blocker.Word == "" {
			t.Fatal("the cap word must stay visible beside the PASS gate")
		}
		// The OQ-18 gate view of the route carries the acting PASS.
		for _, gate := range avito.Gates {
			if gate.ID == "OQ-11" {
				t.Fatal("avito_candidate never requires OQ-11")
			}
		}
	})

	// row-5: the controlled conjunction of human_inbox PASSes live — the
	// route walks to controlled while the live gates stay open (the honest
	// four-stage ladder: controlled is NOT live); protected_web_continuation
	// stays verified: OQ-11 blocks per channel route with the exact word.
	t.Run("row-5 controlled conjunction: controlled NOT live; OQ-11 blocks per channel", func(t *testing.T) {
		montagioPassGate(t, w, employee, "OQ-7A", employeeDomain.GateRoleProductOwner, employeeDomain.GateRoleMontagioOperations)
		montagioPassGate(t, w, employee, "OQ-8", employeeDomain.GateRoleMontagioOperations)
		montagioPassGate(t, w, employee, "AG-1", employeeDomain.GateRoleTechnicalLead, employeeDomain.GateRoleOperations)
		montagioPassGate(t, w, employee, "AG-2", employeeDomain.GateRoleProductOwner, employeeDomain.GateRoleUXQA)
		montagioPassGate(t, w, employee, "AG-4", employeeDomain.GateRoleTechnicalLead, employeeDomain.GateRoleSecurity)
		view, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		inbox := montagioRouteOf(t, view, "human_inbox")
		if inbox.Stage != employeeDomain.MontagioStageControlled {
			t.Fatalf("human_inbox stage = %s, want controlled (controlled gates PASS, live gates open)", inbox.Stage)
		}
		// Only the two live-gate blockers remain — by name, with the safe states.
		if len(inbox.Blockers) != 2 {
			t.Fatalf("human_inbox blockers = %+v", inbox.Blockers)
		}
		oq7b := montagioBlockerOf(t, inbox, "live-gate", "OQ-7B")
		if !strings.HasPrefix(oq7b.Word, "OQ-7B missing: baseline-derived пороги маршрута и достаточность выборки не утверждены") {
			t.Fatalf("OQ-7B word = %q", oq7b.Word)
		}
		if oq7b.SafeState != "controlled learning only" || oq7b.NextOwner != "Product Owner + Montagio Operations" {
			t.Fatalf("OQ-7B safe state/owner = %q / %q", oq7b.SafeState, oq7b.NextOwner)
		}
		if blocker := montagioBlockerOf(t, inbox, "live-gate", "OQ-14"); !strings.HasPrefix(blocker.Word, "OQ-14 missing: количественные baseline-пороги live не утверждены") {
			t.Fatalf("OQ-14 word = %q", blocker.Word)
		}
		// The protected continuation stays verified: OQ-11 missing per channel.
		protected := montagioRouteOf(t, view, "protected_web_continuation")
		if protected.Stage != employeeDomain.MontagioStageVerified {
			t.Fatalf("protected_web_continuation stage = %s, want verified (OQ-11 open per channel)", protected.Stage)
		}
		if blocker := montagioBlockerOf(t, protected, "gate", "OQ-11"); !strings.HasPrefix(blocker.Word, "OQ-11 missing (маршрут web_chat:inbound): data-маршрут не подтверждён purpose-specific consent") {
			t.Fatalf("OQ-11 word = %q", blocker.Word)
		}
		// The route carries TWO OQ-11 gate views (one per channel route).
		oq11Views := 0
		for _, gate := range protected.Gates {
			if gate.ID == "OQ-11" {
				oq11Views++
			}
		}
		if oq11Views != 2 {
			t.Fatalf("protected_web_continuation OQ-11 views = %d, want 2 (inbound + outbound)", oq11Views)
		}
	})

	// row-6: the route-scoped OQ-11 discipline: the inbound PASS alone keeps
	// the route verified with the outbound blocker naming the exact channel;
	// the full pair lifts the route to controlled (the scope is exact, never
	// a wildcard and never a tenant-level leak).
	t.Run("row-6 OQ-11 route scope: exact channel, never a wildcard", func(t *testing.T) {
		montagioPassOQ11Route(t, w, employee, "web_chat", employeeDomain.DirectionInbound)
		view, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		protected := montagioRouteOf(t, view, "protected_web_continuation")
		if protected.Stage != employeeDomain.MontagioStageVerified {
			t.Fatalf("protected_web_continuation stage = %s, want verified (only web_chat:in proven)", protected.Stage)
		}
		blocker := montagioBlockerOf(t, protected, "gate", "OQ-11")
		if !strings.HasPrefix(blocker.Word, "OQ-11 missing (маршрут web_chat:outbound)") {
			t.Fatalf("the partial scope must name the missing channel, got %q", blocker.Word)
		}
		// The human inbox never required OQ-11 — it stays controlled untouched.
		if inbox := montagioRouteOf(t, view, "human_inbox"); inbox.Stage != employeeDomain.MontagioStageControlled {
			t.Fatalf("human_inbox stage = %s, want controlled (no OQ-11 dependency)", inbox.Stage)
		}
		// The full pair closes the conjunction: controlled (live gates open).
		montagioPassOQ11Route(t, w, employee, "web_chat", employeeDomain.DirectionOutbound)
		view, err = w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if protected := montagioRouteOf(t, view, "protected_web_continuation"); protected.Stage != employeeDomain.MontagioStageControlled {
			t.Fatalf("protected_web_continuation stage = %s, want controlled (the exact pair closed)", protected.Stage)
		}
	})

	// row-7: the live conjunction: OQ-7B PASS alone keeps both routes
	// controlled (OQ-14 open — the honest «controlled NOT live»); the OQ-14
	// PASS walks them to live (the full double conjunction).
	t.Run("row-7 live conjunction: OQ-7B alone is not live; + OQ-14 walks live", func(t *testing.T) {
		montagioPassGate(t, w, employee, "OQ-7B", employeeDomain.GateRoleProductOwner, employeeDomain.GateRoleMontagioOperations)
		view, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		for _, routeID := range []string{"human_inbox", "protected_web_continuation"} {
			if route := montagioRouteOf(t, view, routeID); route.Stage != employeeDomain.MontagioStageControlled {
				t.Fatalf("%s stage = %s, want controlled (OQ-14 still open)", routeID, route.Stage)
			}
		}
		montagioPassGate(t, w, employee, "OQ-14", employeeDomain.GateRoleProductOwner, employeeDomain.GateRoleOperationsQA)
		view, err = w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		for _, routeID := range []string{"human_inbox", "protected_web_continuation"} {
			route := montagioRouteOf(t, view, routeID)
			if route.Stage != employeeDomain.MontagioStageLive {
				t.Fatalf("%s stage = %s, want live (the full double conjunction closed)", routeID, route.Stage)
			}
			if len(route.Blockers) != 0 {
				t.Fatalf("%s blockers = %+v", routeID, route.Blockers)
			}
		}
		// Every other route stays below live (its own proof is absent).
		for _, route := range view.Report.Routes {
			switch route.RouteID {
			case "human_inbox", "protected_web_continuation":
			default:
				if route.Stage == employeeDomain.MontagioStageLive || route.Stage == employeeDomain.MontagioStageControlled {
					t.Fatalf("route %s cannot reach controlled/live without its own proof, got %s", route.RouteID, route.Stage)
				}
			}
		}
	})

	// row-8: public_commercial: OQ-9 + OQ-10 + OQ-13A PASS (the existing
	// contour, real approvers) never lifts the capped candidate — the
	// Order/payment machines are future slices; the OQ-10 fence word rides
	// the route (the closed validation scenarios stay unblocked).
	t.Run("row-8 public_commercial: the cap holds over the passing gates", func(t *testing.T) {
		montagioPassGate(t, w, employee, "OQ-9", employeeDomain.GateRoleProductOwner, employeeDomain.GateRolePricingOperations)
		montagioPassGate(t, w, employee, "OQ-10", employeeDomain.GateRoleProductOwner, employeeDomain.GateRoleLegalFinanceOperations)
		montagioPassGate(t, w, employee, "OQ-13A", employeeDomain.GateRoleProductOwner, employeeDomain.GateRoleTenant)
		view, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		public := montagioRouteOf(t, view, "public_commercial")
		if public.Stage != employeeDomain.MontagioStageCandidate {
			t.Fatalf("public_commercial stage = %s, want candidate (the cap holds)", public.Stage)
		}
		if blocker := montagioBlockerOf(t, public, "cap", "public_commercial"); !strings.Contains(blocker.Word, "OQ-10 блокирует публичный маршрут, НЕ блокируя закрытые validation-сценарии") {
			t.Fatalf("public_commercial cap word = %q", blocker.Word)
		}
		if len(public.FenceWords) == 0 || !strings.Contains(strings.Join(public.FenceWords, " "), "закрытые validation-сценарии НЕ блокируются (AC-12)") {
			t.Fatalf("public_commercial fence words = %+v", public.FenceWords)
		}
	})

	// row-9: the expired evidence — the OQ-11 web_chat:in PASS expires
	// through the existing contour (pass → expired with the named trigger):
	// only protected_web_continuation drops back to verified; human_inbox
	// stays live (a route-scoped fact never extends to other routes).
	t.Run("row-9 expired OQ-11: only the affected route drops", func(t *testing.T) {
		// Resolve the acting inbound PASS instance through the live history.
		history, err := w.employee.GateInstancesView(ctx, w.readinessActor(true), w.tenant, employee)
		if err != nil {
			t.Fatal(err)
		}
		var inboundPASS id.UUID
		for _, instance := range history.Instances {
			if instance.DefinitionID == "OQ-11" && instance.State == employeeDomain.GateStatePass &&
				instance.Route != nil && instance.Route.String() == "web_chat:"+employeeDomain.DirectionInbound {
				inboundPASS = instance.InstanceID
			}
		}
		if !inboundPASS.Valid() {
			t.Fatal("the acting OQ-11 web_chat:inbound PASS instance must exist")
		}
		route := employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}
		montagioExpireInstance(t, w, employee, inboundPASS, &route)
		after, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		protected := montagioRouteOf(t, after, "protected_web_continuation")
		if protected.Stage != employeeDomain.MontagioStageVerified {
			t.Fatalf("protected_web_continuation stage = %s, want verified (the inbound PASS expired)", protected.Stage)
		}
		blocker := montagioBlockerOf(t, protected, "gate", "OQ-11")
		if !strings.HasPrefix(blocker.Word, "OQ-11 expired (маршрут web_chat:inbound)") {
			t.Fatalf("the expired state must be named, got %q", blocker.Word)
		}
		if inbox := montagioRouteOf(t, after, "human_inbox"); inbox.Stage != employeeDomain.MontagioStageLive {
			t.Fatalf("human_inbox stage = %s, want live (no OQ-11 dependency — the block never extends)", inbox.Stage)
		}
		// The outbound scope stays proven: the blocker names ONLY inbound.
		if strings.Contains(blocker.Word, "web_chat:outbound") {
			t.Fatalf("the inbound expiry must not touch the outbound scope, got %q", blocker.Word)
		}
	})

	// row-10: the honest refusals — a foreign principal gets the plain 404
	// before any material; an operations-only member the honest 403.
	t.Run("row-10 the honest 404 and 403 of the reused rights", func(t *testing.T) {
		foreign := newReleaseWorld(t, 386)
		if _, err := w.employee.MontagioAdmission(ctx, foreign.owner, w.tenant); !errors.Is(err, employeeDomain.ErrNotFound) {
			t.Fatalf("a foreign principal must get the plain 404, got %v", err)
		}
		identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, newPrivacyIDs(13900))
		operator := inviteMember(t, identityLive, w.owner, w.tenant, "montagio-ops@example.org", identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "00000000bf01", "00000000bf02")
		if _, err := w.employee.MontagioAdmission(ctx, operator, w.tenant); !errors.Is(err, employeeDomain.ErrReadinessForbidden) {
			t.Fatalf("an operations-only member must get the honest 403, got %v", err)
		}
	})

	// row-11: the derivation is idempotent — two reads of the unchanged
	// world answer the byte-stable same JSON.
	t.Run("row-11 the read-time derivation is idempotent", func(t *testing.T) {
		first, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		firstJSON, err := json.Marshal(first)
		if err != nil {
			t.Fatal(err)
		}
		second, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		secondJSON, err := json.Marshal(second)
		if err != nil {
			t.Fatal(err)
		}
		if string(firstJSON) != string(secondJSON) {
			t.Fatal("two reads of the unchanged world must answer the byte-stable same JSON")
		}
	})

	// row-12: the import-graph probe (the production root imports no adapter
	// — the admission contour is local, AG-4), the live dictionary mirrors
	// against the domain words and the live CHECK-constraint mirror of
	// migration 00014 (the 20-word SQL dictionary, the six Montagio words
	// by name).
	t.Run("row-12 import graph, dictionary mirrors and the live CHECK mirror", func(t *testing.T) {
		cmd := exec.Command("go", "list", "-deps", "./cmd/staffy-core")
		cmd.Dir = "../.."
		out, err := cmd.Output()
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				t.Fatalf("the go list probe must answer: %v: %s", err, exit.Stderr)
			}
			t.Fatalf("the go list probe must run: %v", err)
		}
		deps := string(out)
		for _, adapter := range []string{"internal/adapters/reconciliationsyn", "internal/adapters/actiondispatcher"} {
			if strings.Contains(deps, adapter) {
				t.Fatalf("the production root must never import the adapter %s (AG-4)", adapter)
			}
		}
		view, err := w.employee.MontagioAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		// The route dictionary mirror: the fixed order of eight words.
		dictionary := employeeDomain.MontagioRoutes()
		if len(view.Report.Routes) != len(dictionary) {
			t.Fatalf("live routes = %d, dictionary = %d", len(view.Report.Routes), len(dictionary))
		}
		for index, route := range view.Report.Routes {
			if route.RouteID != dictionary[index].ID {
				t.Fatalf("route %d = %s, want %s (the fixed dictionary order)", index, route.RouteID, dictionary[index].ID)
			}
			// The gate requirement mirrors per route (controlled and live).
			liveGates := map[string]bool{}
			for _, gate := range route.Gates {
				if _, known := employeeDomain.GateDefinitionByID(gate.ID); !known {
					t.Fatalf("route %s carries a gate outside the registry dictionary: %s", route.RouteID, gate.ID)
				}
				liveGates[gate.ID] = true
			}
			for _, gateID := range dictionary[index].RequiredGates {
				if !liveGates[gateID] {
					t.Fatalf("route %s misses its required gate %s in the live view", route.RouteID, gateID)
				}
			}
			liveGateViews := map[string]bool{}
			for _, gate := range route.LiveGates {
				liveGateViews[gate.ID] = true
			}
			for _, gateID := range dictionary[index].RequiredLiveGates {
				if !liveGateViews[gateID] {
					t.Fatalf("route %s misses its required live gate %s in the live view", route.RouteID, gateID)
				}
			}
		}
		// The Montagio gate dictionary mirror: the six words in fixed order.
		for index, gateID := range employeeDomain.MontagioGateDictionaryIDs() {
			if view.Report.GateIDs[index] != gateID {
				t.Fatalf("gate dictionary mirror %d = %s, want %s", index, view.Report.GateIDs[index], gateID)
			}
		}
		// The AC-11 wording dictionary mirror: five words.
		if len(view.Report.WordingWords) != len(employeeDomain.MontagioWordingWords) {
			t.Fatalf("wording dictionary = %d words, want %d", len(view.Report.WordingWords), len(employeeDomain.MontagioWordingWords))
		}
		// The live CHECK-constraint mirror of migration 00014: the SQL
		// dictionary carries exactly the 20 words, the six Montagio words by
		// name (the material mirror of the domain dictionary).
		var constraint string
		if err := adminPool.QueryRow(ctx,
			"select pg_get_constraintdef(oid) from pg_constraint where conname = 'gate_instances_definition_id_check' and conrelid = 'employee.gate_instances'::regclass",
		).Scan(&constraint); err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(constraint, "'"); got != 40 {
			t.Fatalf("the live CHECK dictionary carries %d quoted words, want 20 (got %q)", got/2, constraint)
		}
		for _, gateID := range employeeDomain.MontagioGateDictionaryIDs() {
			if !strings.Contains(constraint, "'"+gateID+"'") {
				t.Fatalf("the live CHECK dictionary misses the Montagio word %q (got %q)", gateID, constraint)
			}
		}
	})
}
