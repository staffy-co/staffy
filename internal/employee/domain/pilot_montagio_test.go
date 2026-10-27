package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// montagioFacts assembles the minimal honest world: the «Услуги» pack pin
// sealed, no capability evidence, no gate instances (the fail-closed start).
func montagioFacts() MontagioFacts {
	return MontagioFacts{
		PackID:       MontagioRequiredPackID,
		PackVersion:  "1.0.0",
		Capabilities: map[string]CapabilityFact{},
		Instances:    []GateInstance{},
	}
}

// montagioVerifiedCapabilities maps every requirement of the given routes to
// a verified live fact (the synthetic probe path of the edge-case matrix).
func montagioVerifiedCapabilities(routeIDs ...string) map[string]CapabilityFact {
	facts := map[string]CapabilityFact{}
	for _, routeID := range routeIDs {
		route, ok := MontagioRouteByID(routeID)
		if !ok {
			panic("unknown montagio route: " + routeID)
		}
		for _, requirement := range route.RequiredCapabilities {
			facts[requirement.ID] = CapabilityFact{
				Capability: requirement.ID, State: "verified",
				Reason: "read_only_probe passed", Gap: false, Owner: "Technical Lead",
			}
		}
	}
	return facts
}

// montagioPassAll closes every controlled gate of a route (the tenant-level
// gates as tenant-level PASS records, OQ-11 per declared channel route).
func montagioPassAll(instances []GateInstance, routeID string) []GateInstance {
	route := montagioRouteByID(routeID)
	for _, gateID := range route.RequiredGates {
		if gateID == "OQ-11" {
			for index := range route.RequiredOQ11Routes {
				channelRoute := route.RequiredOQ11Routes[index]
				instances = append(instances, passInstance(gateID, &channelRoute, ""))
			}
			continue
		}
		instances = append(instances, passInstance(gateID, nil, ""))
	}
	return instances
}

// montagioStateInstance builds one tenant-level instance record in the given
// state with a distinct time (the newest-wins resolution stays honest).
func montagioStateInstance(definitionID, state string, at time.Time) GateInstance {
	return GateInstance{
		TenantID: readinessTenant, EmployeeID: readinessEmployee, InstanceID: readinessApprover,
		DefinitionID: definitionID, State: state,
		ApprovedBy:   []GateApproval{{ParticipantID: profileOwner, Role: GateRoleMontagioOperations}},
		EvidenceRefs: []string{"evidence://montagio/" + strings.ToLower(definitionID) + "/" + state},
		ActorID:      profileOwner, RecordedAt: at,
	}
}

func montagioRouteByID(routeID string) MontagioRoute {
	route, ok := MontagioRouteByID(routeID)
	if !ok {
		panic("unknown montagio route: " + routeID)
	}
	return route
}

func montagioAdmissionOf(t *testing.T, report MontagioAdmissionReport, routeID string) MontagioRouteAdmission {
	t.Helper()
	for _, admission := range report.Routes {
		if admission.RouteID == routeID {
			return admission
		}
	}
	t.Fatalf("route %s missing from the report", routeID)
	return MontagioRouteAdmission{}
}

func montagioBlockerOf(t *testing.T, admission MontagioRouteAdmission, source, id string) PilotBlocker {
	t.Helper()
	for _, blocker := range admission.Blockers {
		if blocker.Source == source && blocker.ID == id {
			return blocker
		}
	}
	t.Fatalf("blocker %s/%s missing from route %s (blockers: %+v)", source, id, admission.RouteID, admission.Blockers)
	return PilotBlocker{}
}

// --- The dictionary mirrors -------------------------------------------------

func TestMontagioRouteDictionaryIsFixed(t *testing.T) {
	routes := MontagioRoutes()
	if len(routes) != 8 {
		t.Fatalf("the Montagio dictionary carries exactly 8 route words, got %d", len(routes))
	}
	want := []string{"novofon_voice", "montagio_web", "partner_intake", "b2b_intake", "protected_web_continuation", "human_inbox", "avito_candidate", "public_commercial"}
	for index, routeID := range want {
		if routes[index].ID != routeID {
			t.Fatalf("route word %d = %s, want %s (the fixed order)", index, routes[index].ID, routeID)
		}
		if routes[index].Title == "" || routes[index].ModalityWord == "" || routes[index].IntentsWord == "" ||
			routes[index].AutonomyWord == "" || routes[index].HumanInboxWord == "" || routes[index].FallbackRouteWord == "" {
			t.Fatalf("route %s must carry every Route Matrix column word", routeID)
		}
		if routes[index].AutonomyWord != montagioAutonomyWord {
			t.Fatalf("route %s autonomy column must be the shared honest Operating Profile word", routeID)
		}
	}
	if _, ok := MontagioRouteByID("avito_chat"); ok {
		t.Fatal("an out-of-dictionary route word must not resolve (the Fixed Montagio fence)")
	}
	// The eternal/capped caps are pinned to exactly two routes.
	for _, route := range routes {
		capped := route.ID == "avito_candidate" || route.ID == "public_commercial"
		if (route.CapWord != "") != capped {
			t.Fatalf("route %s cap word presence = %v, want %v", route.ID, route.CapWord != "", capped)
		}
		if capped && route.CapNextOwner == "" {
			t.Fatalf("capped route %s must name the honest next owner of the cap", route.ID)
		}
	}
	// The eternal Avito candidate: no capabilities, no live gates, no channel
	// scopes — the honest absence of declarations (AC-8).
	avito := montagioRouteByID("avito_candidate")
	if len(avito.RequiredCapabilities) != 0 || len(avito.RequiredLiveGates) != 0 || len(avito.RequiredOQ11Routes) != 0 {
		t.Fatalf("avito_candidate carries no capability/live-gate/channel claims, got %+v", avito)
	}
	if len(avito.RequiredGates) != 1 || avito.RequiredGates[0] != "OQ-18" {
		t.Fatalf("avito_candidate requires exactly {OQ-18}, got %v", avito.RequiredGates)
	}
	// The AC-11 wording dictionary: exactly five approved wordings.
	if len(MontagioWordingWords) != 5 {
		t.Fatalf("the AC-11 wording dictionary carries exactly 5 approved wordings, got %d", len(MontagioWordingWords))
	}
}

func TestMontagioRouteGatesMirrorTheRegistryDictionary(t *testing.T) {
	pins := map[string]struct {
		controlled []string
		live       []string
		oq11       []string
	}{
		"novofon_voice":              {[]string{"OQ-7A", "OQ-8", "OQ-11", "OQ-12", "OQ-13B", "AG-1", "AG-2", "AG-4"}, []string{"OQ-7B", "OQ-14"}, []string{"phone:inbound", "phone:outbound"}},
		"montagio_web":               {[]string{"OQ-7A", "OQ-8", "OQ-11", "AG-1", "AG-2", "AG-4"}, []string{"OQ-7B", "OQ-14"}, []string{"web_chat:inbound", "web_chat:outbound"}},
		"partner_intake":             {[]string{"OQ-7A", "OQ-8", "OQ-11", "AG-1", "AG-2", "AG-4"}, []string{"OQ-7B", "OQ-14"}, []string{"web_chat:inbound", "web_chat:outbound"}},
		"b2b_intake":                 {[]string{"OQ-7A", "OQ-8", "OQ-11", "AG-1", "AG-2", "AG-4"}, []string{"OQ-7B", "OQ-14"}, []string{"web_chat:inbound", "web_chat:outbound"}},
		"protected_web_continuation": {[]string{"OQ-7A", "OQ-8", "OQ-11", "AG-1", "AG-2", "AG-4"}, []string{"OQ-7B", "OQ-14"}, []string{"web_chat:inbound", "web_chat:outbound"}},
		"human_inbox":                {[]string{"OQ-7A", "OQ-8", "AG-1", "AG-2", "AG-4"}, []string{"OQ-7B", "OQ-14"}, nil},
		"avito_candidate":            {[]string{"OQ-18"}, nil, nil},
		"public_commercial":          {[]string{"OQ-7A", "OQ-8", "OQ-9", "OQ-10", "OQ-13A", "OQ-11", "AG-1", "AG-2", "AG-4"}, []string{"OQ-7B", "OQ-14"}, []string{"web_chat:inbound", "web_chat:outbound"}},
	}
	for _, route := range MontagioRoutes() {
		pin := pins[route.ID]
		if strings.Join(route.RequiredGates, ",") != strings.Join(pin.controlled, ",") {
			t.Fatalf("route %s controlled gates = %v, want %v", route.ID, route.RequiredGates, pin.controlled)
		}
		if strings.Join(route.RequiredLiveGates, ",") != strings.Join(pin.live, ",") {
			t.Fatalf("route %s live gates = %v, want %v", route.ID, route.RequiredLiveGates, pin.live)
		}
		var scopes []string
		for _, channelRoute := range route.RequiredOQ11Routes {
			scopes = append(scopes, channelRoute.String())
		}
		if strings.Join(scopes, ",") != strings.Join(pin.oq11, ",") {
			t.Fatalf("route %s OQ-11 scopes = %v, want %v", route.ID, scopes, pin.oq11)
		}
		for _, gateID := range append(append([]string(nil), route.RequiredGates...), route.RequiredLiveGates...) {
			if _, known := GateDefinitionByID(gateID); !known {
				t.Fatalf("route %s references gate %s outside the registry dictionary", route.ID, gateID)
			}
			if _, worded := montagioGateWords[gateID]; !worded {
				t.Fatalf("gate %s of route %s has no pinned Montagio word (dictionary integrity)", gateID, route.ID)
			}
		}
	}
}

func TestMontagioGateDictionaryExtendedToTwenty(t *testing.T) {
	dictionary := GateDictionary()
	if len(dictionary) != 20 {
		t.Fatalf("the registry dictionary carries 8 platform + 6 Oleg (4-1) + 6 Montagio (5-1) definitions, got %d", len(dictionary))
	}
	pins := map[string]struct {
		criticality string
		roles       []string
		scoped      bool
	}{
		"OQ-7A":  {"high", []string{GateRoleProductOwner, GateRoleMontagioOperations}, false},
		"OQ-8":   {"critical", []string{GateRoleMontagioOperations}, false},
		"OQ-9":   {"critical", []string{GateRoleProductOwner, GateRolePricingOperations}, false},
		"OQ-10":  {"critical", []string{GateRoleProductOwner, GateRoleLegalFinanceOperations}, false},
		"OQ-13A": {"medium", []string{GateRoleProductOwner, GateRoleTenant}, false},
		"OQ-18":  {"high", []string{GateRoleProductOwner, GateRolePartnershipsArchitecture}, false},
	}
	for id, pin := range pins {
		definition, ok := GateDefinitionByID(id)
		if !ok {
			t.Fatalf("%s must be carried into the product surface by spec 5-1", id)
		}
		if definition.Criticality != pin.criticality {
			t.Fatalf("%s criticality = %s, want %s (GATE-REGISTRY canon)", id, definition.Criticality, pin.criticality)
		}
		if strings.Join(definition.RequiredApproverRoles, ",") != strings.Join(pin.roles, ",") {
			t.Fatalf("%s approver roles = %v, want %v (GATE-REGISTRY canon)", id, definition.RequiredApproverRoles, pin.roles)
		}
		if definition.RouteScoped != pin.scoped {
			t.Fatalf("%s route-scoped = %v, want %v (only OQ-11 carries the route dimension)", id, definition.RouteScoped, pin.scoped)
		}
		if definition.EvidenceRequirement == "" || definition.SafeState == "" {
			t.Fatalf("%s must carry the evidence requirement and the safe state", id)
		}
	}
	// The Oleg ladder of 4-1 stays untouched beside the new dictionary.
	if _, ok := PilotRouteByID("text_jivo"); !ok {
		t.Fatal("the Oleg pilot dictionary of 4-1 must coexist untouched")
	}
}

func TestMontagioWordsAreBounded(t *testing.T) {
	bounded := func(word string) bool { return utf8.RuneCountInString(word) <= 500 }
	for stage, word := range montagioStageWords {
		if !bounded(word) {
			t.Fatalf("stage word %s exceeds the 500-rune bound", stage)
		}
	}
	for _, word := range MontagioWordingWords {
		if !bounded(word) {
			t.Fatalf("AC-11 wording exceeds the 500-rune bound: %s", word)
		}
	}
	if !bounded(MontagioPackMissingWord) || !bounded(montagioAutonomyWord) || !bounded(montagioHumanInboxWord) {
		t.Fatal("the shared honest words exceed the 500-rune bound")
	}
	for _, route := range montagioRoutes {
		for _, word := range []string{route.Title, route.ModalityWord, route.IntentsWord, route.AutonomyWord, route.HumanInboxWord, route.FallbackRouteWord, route.CapWord, route.CapNextOwner} {
			if !bounded(word) {
				t.Fatalf("route %s carries an unbounded word: %s", route.ID, word)
			}
		}
		for _, fence := range append(append([]string(nil), route.FenceWords...), route.FallbackWords...) {
			if !bounded(fence) {
				t.Fatalf("route %s carries an unbounded fence/fallback word: %s", route.ID, fence)
			}
		}
		for _, requirement := range route.RequiredCapabilities {
			if !bounded(requirement.UnprovenWord) {
				t.Fatalf("route %s capability %s carries an unbounded unproven word", route.ID, requirement.ID)
			}
		}
	}
	for gateID, word := range montagioGateWords {
		if !bounded(word.Close) || !bounded(word.SafeState) || !bounded(word.NextOwner) {
			t.Fatalf("gate %s carries an unbounded word", gateID)
		}
	}
}

// --- The read-time derivation ----------------------------------------------

func TestDeriveMontagioAdmissionPackMissingBlocksEveryRoute(t *testing.T) {
	facts := montagioFacts()
	facts.PackID, facts.PackVersion = "", ""
	report := DeriveMontagioAdmission(facts)
	if report.PackPinned || report.PackWord != MontagioPackMissingWord {
		t.Fatalf("the missing pack pin must fail every route closed, got %q", report.PackWord)
	}
	for _, admission := range report.Routes {
		if admission.Stage != MontagioStageCandidate {
			t.Fatalf("route %s must stay candidate without the pack pin, got %s", admission.RouteID, admission.Stage)
		}
		blocker := montagioBlockerOf(t, admission, "pack", MontagioRequiredPackID)
		if blocker.Word != MontagioPackMissingWord {
			t.Fatalf("route %s pack blocker word must be the pinned honest word", admission.RouteID)
		}
	}
}

func TestDeriveMontagioAdmissionForeignPackDoesNotPin(t *testing.T) {
	facts := montagioFacts()
	facts.PackID = PilotRequiredPackID // the Oleg pack does not pin the Montagio pilot
	report := DeriveMontagioAdmission(facts)
	if report.PackPinned {
		t.Fatal("the «Автозапчасти» pack must not pin the Montagio admission")
	}
}

func TestDeriveMontagioAdmissionAdapterNotImplementedCapsAtCandidate(t *testing.T) {
	report := DeriveMontagioAdmission(montagioFacts())
	voice := montagioAdmissionOf(t, report, "novofon_voice")
	if voice.Stage != MontagioStageCandidate {
		t.Fatalf("novofon_voice must stay candidate while the adapter is not implemented, got %s", voice.Stage)
	}
	if len(voice.Capabilities) != 12 {
		t.Fatalf("novofon_voice carries the twelve AC-3 capabilities by name, got %d", len(voice.Capabilities))
	}
	blocker := montagioBlockerOf(t, voice, "capability", "query.inbound-calls")
	if !strings.Contains(blocker.Word, "общая документация Novofon не считается доказательством") {
		t.Fatalf("the honest Novofon adapter word must be carried, got %q", blocker.Word)
	}
	// The Staffy-own surfaces carry no capability claims (the honest vacuous
	// case) — they are not blocked by capability absence.
	inbox := montagioAdmissionOf(t, report, "human_inbox")
	for _, blocker := range inbox.Blockers {
		if blocker.Source == "capability" {
			t.Fatalf("human_inbox is a Staffy-own surface — no capability blockers allowed, got %+v", blocker)
		}
	}
	if inbox.Stage != MontagioStageVerified {
		t.Fatalf("the Staffy-own surface with no capabilities and open gates is verified, got %s", inbox.Stage)
	}
}

func TestDeriveMontagioAdmissionSyntheticVerifiedOpenGatesStaysVerified(t *testing.T) {
	facts := montagioFacts()
	facts.Capabilities = montagioVerifiedCapabilities("novofon_voice", "montagio_web", "partner_intake", "b2b_intake", "protected_web_continuation", "human_inbox", "public_commercial")
	report := DeriveMontagioAdmission(facts)
	for _, routeID := range []string{"novofon_voice", "montagio_web", "partner_intake", "b2b_intake", "protected_web_continuation", "human_inbox"} {
		admission := montagioAdmissionOf(t, report, routeID)
		if admission.Stage != MontagioStageVerified {
			t.Fatalf("route %s with every capability verified and open gates must stay verified, got %s", routeID, admission.Stage)
		}
		for _, blocker := range admission.Blockers {
			if blocker.Source != "gate" && blocker.Source != "live-gate" {
				t.Fatalf("route %s must be blocked by gates/live-gates only at verified, got %+v", routeID, blocker)
			}
		}
	}
	// The capped routes stay candidates with the honest cap word.
	avito := montagioAdmissionOf(t, report, "avito_candidate")
	if avito.Stage != MontagioStageCandidate || avito.CapWord == "" {
		t.Fatalf("avito_candidate stays the eternal candidate, got %s", avito.Stage)
	}
	public := montagioAdmissionOf(t, report, "public_commercial")
	if public.Stage != MontagioStageCandidate || public.CapWord == "" {
		t.Fatalf("public_commercial stays the capped candidate, got %s", public.Stage)
	}
}

func TestDeriveMontagioAdmissionControlledRequiresFullConjunction(t *testing.T) {
	facts := montagioFacts()
	facts.Capabilities = montagioVerifiedCapabilities("novofon_voice")
	// Every controlled gate of novofon_voice carries an acting PASS except
	// OQ-8 (open — the fallback owners are not named).
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	for _, gateID := range montagioRouteByID("novofon_voice").RequiredGates {
		if gateID == "OQ-11" {
			for index := range montagioRouteByID("novofon_voice").RequiredOQ11Routes {
				channelRoute := montagioRouteByID("novofon_voice").RequiredOQ11Routes[index]
				facts.Instances = append(facts.Instances, passInstance(gateID, &channelRoute, ""))
			}
			continue
		}
		if gateID == "OQ-8" {
			facts.Instances = append(facts.Instances, montagioStateInstance(gateID, GateStateOpen, at))
			continue
		}
		facts.Instances = append(facts.Instances, passInstance(gateID, nil, ""))
	}
	report := DeriveMontagioAdmission(facts)
	voice := montagioAdmissionOf(t, report, "novofon_voice")
	if voice.Stage != MontagioStageVerified {
		t.Fatalf("one open controlled gate keeps the route below controlled, got %s", voice.Stage)
	}
	blocker := montagioBlockerOf(t, voice, "gate", "OQ-8")
	if !strings.HasPrefix(blocker.Word, "OQ-8 open: ") {
		t.Fatalf("the blocker word is the exact gate word, got %q", blocker.Word)
	}
	if blocker.SafeState != "no immediate-help promise; named owner/next update or honest intake/callback" {
		t.Fatalf("the OQ-8 safe state is carried beside the blocker, got %q", blocker.SafeState)
	}
	if blocker.NextOwner != "Montagio Operations" {
		t.Fatalf("the OQ-8 next owner is the registry canon owner, got %q", blocker.NextOwner)
	}
}

func TestDeriveMontagioAdmissionLiveRequiresOQ7BAndOQ14(t *testing.T) {
	facts := montagioFacts()
	facts.Capabilities = montagioVerifiedCapabilities("novofon_voice")
	facts.Instances = montagioPassAll(facts.Instances, "novofon_voice")
	// OQ-7B stays open: the baseline-derived thresholds are not approved.
	report := DeriveMontagioAdmission(facts)
	voice := montagioAdmissionOf(t, report, "novofon_voice")
	if voice.Stage != MontagioStageControlled {
		t.Fatalf("controlled gates PASS with OQ-7B open must stay controlled (NOT live), got %s", voice.Stage)
	}
	blocker := montagioBlockerOf(t, voice, "live-gate", "OQ-7B")
	if !strings.HasPrefix(blocker.Word, "OQ-7B ") {
		t.Fatalf("the live-gate blocker names OQ-7B, got %q", blocker.Word)
	}
	if len(voice.LiveGates) != 2 {
		t.Fatalf("the live ladder carries {OQ-7B, OQ-14} by name, got %d views", len(voice.LiveGates))
	}
	// Closing both live gates admits live — the full double conjunction.
	facts.Instances = append(facts.Instances, passInstance("OQ-7B", nil, ""), passInstance("OQ-14", nil, ""))
	report = DeriveMontagioAdmission(facts)
	voice = montagioAdmissionOf(t, report, "novofon_voice")
	if voice.Stage != MontagioStageLive {
		t.Fatalf("the full conjunction must admit live, got %s", voice.Stage)
	}
	if len(voice.Blockers) != 0 {
		t.Fatalf("a live route carries no blockers, got %+v", voice.Blockers)
	}
}

func TestDeriveMontagioAdmissionOQ10InTestBlocksPublicCommercialHonestly(t *testing.T) {
	facts := montagioFacts()
	facts.Capabilities = montagioVerifiedCapabilities("public_commercial")
	at := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	facts.Instances = append(facts.Instances, montagioStateInstance("OQ-10", GateStateInTest, at))
	report := DeriveMontagioAdmission(facts)
	public := montagioAdmissionOf(t, report, "public_commercial")
	if public.Stage != MontagioStageCandidate {
		t.Fatalf("public_commercial is the capped candidate of this slice, got %s", public.Stage)
	}
	blocker := montagioBlockerOf(t, public, "gate", "OQ-10")
	if !strings.HasPrefix(blocker.Word, "OQ-10 in_test: ") {
		t.Fatalf("the blocker word is the exact in_test gate word, got %q", blocker.Word)
	}
	if !strings.Contains(blocker.SafeState, "закрытый тест правдиво называет фактические стороны") {
		t.Fatalf("the OQ-10 safe state is carried beside the blocker, got %q", blocker.SafeState)
	}
	// The fence word carries the honest boundary: closed validation
	// scenarios are NOT blocked by the open gate (AC-12).
	found := false
	for _, fence := range public.FenceWords {
		if strings.Contains(fence, "закрытые validation-сценарии НЕ блокируются") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the AC-12 fence word must be carried by public_commercial: %v", public.FenceWords)
	}
}

func TestDeriveMontagioAdmissionAvitoEternalCandidate(t *testing.T) {
	facts := montagioFacts()
	// Even a perfect world never lifts the eternal candidate: OQ-18 PASS
	// does not turn an unproven integration into an admission.
	facts.Instances = append(facts.Instances, passInstance("OQ-18", nil, ""))
	report := DeriveMontagioAdmission(facts)
	avito := montagioAdmissionOf(t, report, "avito_candidate")
	if avito.Stage != MontagioStageCandidate {
		t.Fatalf("avito_candidate is the eternal candidate even with OQ-18 PASS, got %s", avito.Stage)
	}
	blocker := montagioBlockerOf(t, avito, "cap", "avito_candidate")
	if !strings.Contains(blocker.Word, "scraping, имитация клиента и обход ограничений платформы запрещены") {
		t.Fatalf("the eternal cap word is carried, got %q", blocker.Word)
	}
	if blocker.NextOwner != "Product Owner + Partnerships/Architecture (OQ-18)" {
		t.Fatalf("the cap names its honest next owner, got %q", blocker.NextOwner)
	}
}

func TestDeriveMontagioAdmissionStaleEvidenceBlocksOnlyAffectedRoute(t *testing.T) {
	facts := montagioFacts()
	facts.Capabilities = montagioVerifiedCapabilities("novofon_voice", "montagio_web")
	// One Novofon capability proof goes stale (rotation/expiry).
	facts.Capabilities["query.inbound-calls"] = CapabilityFact{
		Capability: "query.inbound-calls", State: "stale",
		Reason: "evidence rotated", Gap: false, Owner: "Technical Lead",
	}
	report := DeriveMontagioAdmission(facts)
	voice := montagioAdmissionOf(t, report, "novofon_voice")
	if voice.Stage != MontagioStageCandidate {
		t.Fatalf("stale evidence fails the affected route closed below verified, got %s", voice.Stage)
	}
	blocker := montagioBlockerOf(t, voice, "capability", "query.inbound-calls")
	if !strings.Contains(blocker.Word, "доказательство устарело — требуется повторная проверка") {
		t.Fatalf("the stale word is the honest DERIVED-state word, got %q", blocker.Word)
	}
	// The provably independent route continues (AC-14).
	web := montagioAdmissionOf(t, report, "montagio_web")
	if web.Stage != MontagioStageVerified {
		t.Fatalf("the independent route continues its derivation, got %s", web.Stage)
	}
}

func TestDeriveMontagioAdmissionGateStatesNeverControlled(t *testing.T) {
	facts := montagioFacts()
	facts.Capabilities = montagioVerifiedCapabilities("montagio_web")
	at := time.Date(2026, 10, 22, 11, 0, 0, 0, time.UTC)
	for _, state := range []string{GateStateOpen, GateStateInTest, GateStateFail, GateStateExpired, "missing"} {
		world := montagioFacts()
		world.Capabilities = montagioVerifiedCapabilities("montagio_web")
		world.Instances = append([]GateInstance(nil), facts.Instances...)
		if state != "missing" {
			world.Instances = append(world.Instances, montagioStateInstance("OQ-7A", state, at))
		}
		for _, gateID := range []string{"OQ-8", "AG-1", "AG-2", "AG-4"} {
			world.Instances = append(world.Instances, passInstance(gateID, nil, ""))
		}
		for index := range montagioRouteByID("montagio_web").RequiredOQ11Routes {
			channelRoute := montagioRouteByID("montagio_web").RequiredOQ11Routes[index]
			world.Instances = append(world.Instances, passInstance("OQ-11", &channelRoute, ""))
		}
		report := DeriveMontagioAdmission(world)
		web := montagioAdmissionOf(t, report, "montagio_web")
		if web.Stage == MontagioStageControlled || web.Stage == MontagioStageLive {
			t.Fatalf("OQ-7A in state %s must never admit controlled/live, got %s", state, web.Stage)
		}
	}
}

func TestDeriveMontagioAdmissionOQ11IsRouteScoped(t *testing.T) {
	facts := montagioFacts()
	facts.Capabilities = montagioVerifiedCapabilities("novofon_voice")
	// Only the inbound phone route carries an acting OQ-11 PASS — the
	// outbound scope stays unproven, the route stays below controlled.
	facts.Instances = append(facts.Instances, passInstance("OQ-11", &Route{Channel: "phone", Direction: DirectionInbound}, ""))
	report := DeriveMontagioAdmission(facts)
	voice := montagioAdmissionOf(t, report, "novofon_voice")
	if voice.Stage == MontagioStageControlled || voice.Stage == MontagioStageLive {
		t.Fatalf("a partial OQ-11 scope must not admit the route, got %s", voice.Stage)
	}
	scoped := false
	for _, view := range voice.Gates {
		if view.ID == "OQ-11" && strings.Contains(view.Word, "phone:outbound") {
			scoped = true
		}
	}
	if !scoped {
		t.Fatalf("the honest OQ-11 blocker names the unproven channel route, gates: %+v", voice.Gates)
	}
}

func TestDeriveMontagioAdmissionIdempotentByteStable(t *testing.T) {
	facts := montagioFacts()
	facts.Capabilities = montagioVerifiedCapabilities("novofon_voice", "montagio_web")
	at := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	facts.Instances = append(facts.Instances, montagioStateInstance("OQ-8", GateStateOpen, at))
	first := DeriveMontagioAdmission(facts)
	second := DeriveMontagioAdmission(facts)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("a repeated read of an unchanged world must return the same derivation")
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("the derivation is byte-stable in JSON")
	}
}

func TestDeriveMontagioAdmissionWordingDictionaryCarried(t *testing.T) {
	report := DeriveMontagioAdmission(montagioFacts())
	if len(report.WordingWords) != 5 {
		t.Fatalf("the report carries the AC-11 wording dictionary, got %d", len(report.WordingWords))
	}
	found := false
	for _, word := range report.WordingWords {
		if strings.Contains(word, "best-effort без SLA") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the OQ-13A best-effort wording is carried, got %v", report.WordingWords)
	}
}
