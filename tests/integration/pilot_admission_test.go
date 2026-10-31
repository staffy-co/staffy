//go:build integration

package integration_test

// pilot_admission_test.go — Story 4-1 (Epic 4): the LIVE integration matrix
// of the Pilot Admission Registry over the reused readiness world of 1-13/
// 1-14 (real PG, real identity, real connector service with the synthetic
// verifier, the real app service with the wired ConnectorFacts port). The
// derivation is READ-TIME: no row of this matrix writes a pilot record —
// the only writes are the reused contours (the connector lifecycle, the
// gate transitions of the existing test & release surface, the pack pin
// and the sealed manifest of the release surface).
//
// The matrix rows (the spec's I/O & edge-case matrix, live):
//   row-1  a tenant without any sealed manifest: every route honestly
//          blocked by the missing Domain Pack pin (fail-closed start);
//   row-2  the reused world as-is: its sealed manifests pin auto-parts@
//          1.0.0 — BELOW the pilot minimum 2.0.0, the pin stays missing;
//   row-3  the 2.0.0 pin sealed live: the pack contribution passes, the
//          Jivo/ABCP routes stay candidates with the honest adapter word;
//   row-4  the six SIP queries verified live (the synthetic probe): the
//          voice route rises to verified, the gates block by exact words;
//   row-5  OQ-3 in_test (the existing gate contour): the order route
//          blocked by the exact word with the fence and the safe state;
//   row-6  AG-2 + AG-4 PASS (the existing gate contour, real approvers):
//          the human fallback route walks the only admitted path;
//   row-7  the secret rotation: the stale word blocks only the voice
//          route, the other routes' words stay unchanged;
//   row-8  the honest refusals: a foreign principal gets the plain 404
//          before any material, an operations-only member the honest 403;
//   row-9  the derivation is idempotent: two reads of the unchanged world
//          answer the byte-stable same JSON;
//   row-10 the import-graph probe (the production root imports no adapter)
//          and the live dictionary mirrors against the domain words.
//
// The slot registry (the retro-2-3 discipline): the recovery-policy matrix
// of 3-9 owns seed 381, so the pilot owns 383 (the main world) and 384 (the
// bare/foreign world) — the free slots of the 38x decade; the live identity
// slot is the ID sequence 13800 with the member suffixes be01/be02
// (dispatch owns 13600/b8xx, reconcile owns 13700/b9xx).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	identityApp "github.com/staffy-co/staffy/internal/identity/app"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	identityStore "github.com/staffy-co/staffy/internal/identity/store"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// pilotScopeOf builds the caller-authority scope of the world owner.
func pilotScopeOf(w *readinessWorld) tenancy.Context {
	scope, _ := tenancy.New(w.owner.ID, w.tenant)
	return scope
}

// pilotRouteView resolves one route of the live view.
func pilotRouteView(t *testing.T, view employeeAPI.PilotAdmissionView, routeID string) employeeDomain.PilotRouteAdmission {
	t.Helper()
	for _, route := range view.Report.Routes {
		if route.RouteID == routeID {
			return route
		}
	}
	t.Fatalf("route %s missing from the live view", routeID)
	return employeeDomain.PilotRouteAdmission{}
}

// pilotBlockerView resolves one blocker of a route.
func pilotBlockerView(t *testing.T, route employeeDomain.PilotRouteAdmission, source, blockerID string) employeeDomain.PilotBlocker {
	t.Helper()
	for _, blocker := range route.Blockers {
		if blocker.Source == source && blocker.ID == blockerID {
			return blocker
		}
	}
	t.Fatalf("blocker %s/%s missing from route %s", source, blockerID, route.RouteID)
	return employeeDomain.PilotBlocker{}
}

// pilotSealPackTwo seals a fresh manifest whose config version pins
// auto-parts@2.0.0 (the live release contour: the pack pin upgrade with
// the confirmed incompatible impact, the assemble, the version, the seal).
func pilotSealPackTwo(t *testing.T, w *readinessWorld, employee id.UUID) {
	t.Helper()
	ctx := context.Background()
	// The current employee draft version (the pin command is CAS).
	current, err := w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-000000000b91"),
		ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: "2.0.0",
		Variants: map[string]any{"parts_interest": "both", "matching_depth": float64(1)}, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	// The release chain over the upgraded refs (the live revision walk).
	view, err := w.employee.ReleaseView(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	revision := int64(0)
	if view.Draft != nil {
		revision = view.Draft.Revision
	}
	draft := w.assemble(t, employee, revision)
	version := w.createVersion(t, employee, draft.Revision, "Пин пилота 2.0.0")
	if _, err := w.seal(t, employee, version.Version, draft.Revision+1); err != nil {
		t.Fatal(err)
	}
}

// pilotVerifyVoice verifies the six SIP queries of the synthetic telephony
// connection live (the read-only probe contour of 1-9).
func pilotVerifyVoice(t *testing.T, w *readinessWorld, connectionID id.UUID) {
	t.Helper()
	ctx := context.Background()
	for index, capability := range []string{
		"query.transfer-bridge", "query.callback-state", "query.dtmf-support",
		"query.playback-delivery", "query.failure-states",
	} {
		if _, err := w.connector.VerifyCapability(ctx, connectorAPI.VerifyCommand{
			Actor: w.owner, TenantID: w.tenant, ConnectionID: connectionID,
			CorrelationID: id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-000000000b%02d", 92+index)),
			Capability:    capability, IdempotencyKey: fmt.Sprintf("pilot-verify-%s-%d", capability, index),
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// pilotPassGate walks the existing gate contour to one tenant-level PASS of
// the world employee: open → in_test → pass with the full approver
// discipline (the roles covered by distinct principals, the evidence
// references and the sha256 hash).
func pilotPassGate(t *testing.T, w *readinessWorld, employee id.UUID, definitionID string, roles ...string) {
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
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000ba1"),
		DefinitionID:  definitionID, TargetState: employeeDomain.GateStateOpen,
	})
	if err != nil {
		t.Fatal(err)
	}
	inTest, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000ba2"),
		DefinitionID:  definitionID, TargetState: employeeDomain.GateStateInTest,
		Supersedes: &opened.InstanceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000ba3"),
		DefinitionID:  definitionID, TargetState: employeeDomain.GateStatePass,
		Supersedes: &inTest.InstanceID, ApprovedBy: approvers,
		EvidenceRefs: []string{"evidence://" + strings.ToLower(definitionID) + "/pilot-pass"},
		EvidenceHash: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPilotAdmissionMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()

	// row-1: a bare tenant (an employee draft, no sealed manifest at all) —
	// the fail-closed start of every route.
	t.Run("row-1 no sealed manifest: pack missing blocks every route", func(t *testing.T) {
		bare := newReleaseWorld(t, 384)
		bareEmployee := bare.createEmployee(t, false)
		view, err := bare.employee.PilotAdmission(ctx, bare.owner, bare.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if view.Report.PackPinned {
			t.Fatal("a tenant without a sealed manifest has no pack pin")
		}
		if view.Report.PackWord != employeeDomain.PilotPackMissingWord {
			t.Fatalf("pack word = %q", view.Report.PackWord)
		}
		for _, route := range view.Report.Routes {
			if route.Stage != employeeDomain.PilotStageCandidate {
				t.Fatalf("route %s stage = %s, want candidate", route.RouteID, route.Stage)
			}
			blocker := pilotBlockerView(t, route, "pack", employeeDomain.PilotRequiredPackID)
			if blocker.Word != employeeDomain.PilotPackMissingWord {
				t.Fatalf("route %s pack blocker = %q", route.RouteID, blocker.Word)
			}
		}
		// The matrix still renders its eight unknown fields (AC-10).
		if len(view.Report.Matrix) != 8 {
			t.Fatalf("matrix fields = %d, want 8", len(view.Report.Matrix))
		}
		_ = bareEmployee
	})

	// The main world: the reused readiness candidate (the manifest pins
	// auto-parts@1.0.0 — the honest below-minimum start).
	w, employee, _ := newReadinessWorld(t, 383)

	// row-2: the reused world as-is — its sealed manifest pins 1.0.0, which
	// is below the pilot minimum: the pin stays honestly missing.
	t.Run("row-2 auto-parts@1.0.0 is below the pilot minimum", func(t *testing.T) {
		view, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if view.Report.PackPinned {
			t.Fatal("auto-parts@1.0.0 must not satisfy the >=2.0.0 pilot pin")
		}
		if view.Report.PackWord != employeeDomain.PilotPackMissingWord {
			t.Fatalf("pack word = %q", view.Report.PackWord)
		}
	})

	// row-3: seal the 2.0.0 pin live; the Jivo/ABCP routes stay candidates
	// with the honest adapter word (AG-4: a declaration is not an adapter).
	t.Run("row-3 the 2.0.0 pin passes; the adapters stay honestly absent", func(t *testing.T) {
		pilotSealPackTwo(t, w, employee)
		view, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if !view.Report.PackPinned {
			t.Fatalf("the sealed 2.0.0 pin must pass: %q", view.Report.PackWord)
		}
		if !strings.Contains(view.Report.PackWord, "auto-parts@2.0.0") {
			t.Fatalf("pack word = %q", view.Report.PackWord)
		}
		text := pilotRouteView(t, view, "text_jivo")
		if text.Stage != employeeDomain.PilotStageCandidate {
			t.Fatalf("text_jivo stage = %s, want candidate (no live Jivo evidence)", text.Stage)
		}
		blocker := pilotBlockerView(t, text, "capability", "event.inbound-message")
		if !strings.HasPrefix(blocker.Word, "Jivo-адаптер не реализован (AG-4)") {
			t.Fatalf("jivo blocker word = %q", blocker.Word)
		}
		order := pilotRouteView(t, view, "parts_order_abcp")
		if order.Stage != employeeDomain.PilotStageCandidate {
			t.Fatalf("parts_order_abcp stage = %s, want candidate", order.Stage)
		}
	})

	// The connection of the world: the synthetic telephony created and
	// trunk-status-verified by the world builder (the live fact port lists
	// it — never a secret).
	facts, err := w.connector.ConnectionsFacts(ctx, pilotScopeOf(w))
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) == 0 || facts[0].ConnectorID != "staffy-synthetic-telephony" {
		t.Fatalf("the world must carry the synthetic telephony connection, got %+v", facts)
	}
	connectionID := facts[0].ConnectionID

	// row-4: verify the six SIP queries live — the voice route rises to
	// verified, the gates block by the exact words (never hidden).
	t.Run("row-4 verified SIP queries lift voice to verified; gates block by exact words", func(t *testing.T) {
		pilotVerifyVoice(t, w, connectionID)
		view, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		voice := pilotRouteView(t, view, "voice_sip")
		if voice.Stage != employeeDomain.PilotStageVerified {
			t.Fatalf("voice_sip stage = %s, want verified (capabilities proven, gates open)", voice.Stage)
		}
		oq1 := pilotBlockerView(t, voice, "gate", "OQ-1")
		if !strings.HasPrefix(oq1.Word, "OQ-1 missing: провайдер не подтверждён фактическими данными аккаунта") {
			t.Fatalf("OQ-1 word = %q", oq1.Word)
		}
		if oq1.SafeState != "awaiting_verification/limited; неподдерживаемые transfer/caller-заявления запрещены" {
			t.Fatalf("OQ-1 safe state = %q", oq1.SafeState)
		}
		if oq1.NextOwner != "Олег + Technical Lead" {
			t.Fatalf("OQ-1 next owner = %q", oq1.NextOwner)
		}
		// The other routes do not inherit the voice evidence.
		if catalog := pilotRouteView(t, view, "parts_catalog_abcp"); catalog.Stage != employeeDomain.PilotStageCandidate {
			t.Fatalf("parts_catalog_abcp stage = %s, want candidate (independent derivation)", catalog.Stage)
		}
	})

	// row-5: OQ-3 in_test through the existing gate contour — the order
	// route blocked by the exact word, the fence and the safe state.
	t.Run("row-5 OQ-3 in_test: the exact fence word of the order route", func(t *testing.T) {
		opened, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
			Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000bb1"),
			DefinitionID:  "OQ-3", TargetState: employeeDomain.GateStateInTest,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		order := pilotRouteView(t, view, "parts_order_abcp")
		blocker := pilotBlockerView(t, order, "gate", "OQ-3")
		if !strings.HasPrefix(blocker.Word, "OQ-3 in_test: ABCP prepared order не закрыт") {
			t.Fatalf("OQ-3 word = %q", blocker.Word)
		}
		if blocker.SafeState != "нейтральный pending_manager_verification; ручной reread и человеческая квитанция" {
			t.Fatalf("OQ-3 safe state = %q", blocker.SafeState)
		}
		if blocker.NextOwner != "Олег + ABCP integrator" {
			t.Fatalf("OQ-3 next owner = %q", blocker.NextOwner)
		}
		if len(order.FenceWords) != 1 || !strings.HasPrefix(order.FenceWords[0], "автоматическая отправка поставщику не выполняется") {
			t.Fatalf("fence words = %+v", order.FenceWords)
		}
		// The catalog route does not require OQ-3: no such blocker there.
		catalog := pilotRouteView(t, view, "parts_catalog_abcp")
		for _, candidate := range catalog.Blockers {
			if candidate.Source == "gate" && candidate.ID == "OQ-3" {
				t.Fatal("parts_catalog_abcp never requires OQ-3 — the block does not extend")
			}
		}
		_ = opened
	})

	// row-6: AG-2 + AG-4 PASS through the existing gate contour with the
	// real approver discipline — the human fallback route walks the only
	// admitted path of the honest worlds.
	t.Run("row-6 AG-2 + AG-4 PASS: the human fallback admitted path", func(t *testing.T) {
		pilotPassGate(t, w, employee, "AG-2", "ProductOwner", "UXQA")
		pilotPassGate(t, w, employee, "AG-4", "TechnicalLead", "Security")
		view, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		fallback := pilotRouteView(t, view, "human_fallback")
		if fallback.Stage != employeeDomain.PilotStageAdmitted {
			t.Fatalf("human_fallback stage = %s, want admitted", fallback.Stage)
		}
		if len(fallback.Blockers) != 0 {
			t.Fatalf("human_fallback blockers = %+v", fallback.Blockers)
		}
		// Every other route stays below admitted (their own proof is absent).
		for _, route := range view.Report.Routes {
			if route.RouteID == "human_fallback" {
				continue
			}
			if route.Stage == employeeDomain.PilotStageAdmitted {
				t.Fatalf("route %s cannot be admitted without its own proof", route.RouteID)
			}
		}
	})

	// row-7: the secret rotation makes the evidence stale — the stale word
	// blocks only the voice route, the other routes' words stay unchanged.
	t.Run("row-7 rotation: stale blocks only the affected route", func(t *testing.T) {
		before, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		connection, err := w.connector.Read(ctx, w.owner, w.tenant, connectionID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.connector.Rotate(ctx, connectorAPI.ConnectCommand{
			Actor: w.owner, TenantID: w.tenant, ConnectionID: connectionID,
			CorrelationID:  id.MustParse("01994f64-6c00-7000-8000-000000000bc1"),
			IdempotencyKey: fmt.Sprintf("pilot-rotate-%s", connectionID), ExpectedVersion: connection.Version,
			Secrets: map[string]string{"api_key": "sandbox-rotated-pilot"},
		}); err != nil {
			t.Fatal(err)
		}
		after, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		voice := pilotRouteView(t, after, "voice_sip")
		if voice.Stage != employeeDomain.PilotStageCandidate {
			t.Fatalf("voice_sip stage = %s, want candidate (stale evidence drops the ladder)", voice.Stage)
		}
		stale := pilotBlockerView(t, voice, "capability", "query.trunk-status")
		if !strings.Contains(stale.Word, "доказательство устарело — требуется повторная проверка") {
			t.Fatalf("stale word = %q", stale.Word)
		}
		// The other routes' capability words stay unchanged (the Jivo word
		// is not a stale word — different route, different connector).
		textBefore := pilotRouteView(t, before, "text_jivo")
		textAfter := pilotRouteView(t, after, "text_jivo")
		if textBefore.Stage != textAfter.Stage {
			t.Fatalf("text_jivo stage moved: %s -> %s (only the affected route blocks)", textBefore.Stage, textAfter.Stage)
		}
		beforeBlocker := pilotBlockerView(t, textBefore, "capability", "event.inbound-message")
		afterBlocker := pilotBlockerView(t, textAfter, "capability", "event.inbound-message")
		if beforeBlocker.Word != afterBlocker.Word {
			t.Fatalf("the jivo blocker word changed: %q -> %q", beforeBlocker.Word, afterBlocker.Word)
		}
		// The admitted human fallback is untouched by the telephony rotation.
		if fallback := pilotRouteView(t, after, "human_fallback"); fallback.Stage != employeeDomain.PilotStageAdmitted {
			t.Fatalf("human_fallback stage = %s, want admitted (no external capability dependency)", fallback.Stage)
		}
	})

	// row-8: the honest refusals — a foreign principal gets the plain 404
	// before any material; an operations-only member the honest 403.
	t.Run("row-8 the honest 404 and 403 of the reused rights", func(t *testing.T) {
		foreign := newReleaseWorld(t, 384)
		if _, err := w.employee.PilotAdmission(ctx, foreign.owner, w.tenant); !errors.Is(err, employeeDomain.ErrNotFound) {
			t.Fatalf("a foreign principal must get the plain 404, got %v", err)
		}
		identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, newPrivacyIDs(13800))
		operator := inviteMember(t, identityLive, w.owner, w.tenant, "pilot-ops@example.org", identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "00000000be01", "00000000be02")
		if _, err := w.employee.PilotAdmission(ctx, operator, w.tenant); !errors.Is(err, employeeDomain.ErrReadinessForbidden) {
			t.Fatalf("an operations-only member must get the honest 403, got %v", err)
		}
	})

	// row-9: the derivation is idempotent — two reads of the unchanged
	// world answer the byte-stable same JSON.
	t.Run("row-9 the read-time derivation is idempotent", func(t *testing.T) {
		first, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		firstJSON, err := json.Marshal(first)
		if err != nil {
			t.Fatal(err)
		}
		second, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
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

	// row-10: the import-graph probe (the production root imports no
	// adapter — the admission contour is local, AG-4) and the live
	// dictionary mirrors against the domain words.
	t.Run("row-10 import-graph and the live dictionary mirrors", func(t *testing.T) {
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
		view, err := w.employee.PilotAdmission(ctx, w.owner, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		// The route dictionary mirror: the fixed order of six words.
		dictionary := employeeDomain.PilotRoutes()
		if len(view.Report.Routes) != len(dictionary) {
			t.Fatalf("live routes = %d, dictionary = %d", len(view.Report.Routes), len(dictionary))
		}
		for index, route := range view.Report.Routes {
			if route.RouteID != dictionary[index].ID {
				t.Fatalf("route %d = %s, want %s (the fixed dictionary order)", index, route.RouteID, dictionary[index].ID)
			}
			// The gate requirement mirror per route.
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
		}
		// The matrix mirror: the eight fields in the fixed order.
		matrix := employeeDomain.MatrixFields()
		if len(view.Report.Matrix) != len(matrix) {
			t.Fatalf("live matrix = %d fields, dictionary = %d", len(view.Report.Matrix), len(matrix))
		}
		for index, field := range view.Report.Matrix {
			if field.Field != matrix[index].Field || field.StateWord != employeeDomain.MatrixUnknownWord {
				t.Fatalf("matrix field %d = %+v, want %s unknown", index, field, matrix[index].Field)
			}
		}
	})
}
