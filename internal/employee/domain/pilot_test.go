package domain

import (
        "encoding/json"
        "strings"
        "testing"
        "time"
)

// pilotFacts assembles the minimal honest world: the pack pin sealed, no
// capability evidence, no gate instances (the fail-closed start).
func pilotFacts() PilotFacts {
        return PilotFacts{
                PackID:       PilotRequiredPackID,
                PackVersion:  "2.0.0",
                Capabilities: map[string]CapabilityFact{},
                Instances:    []GateInstance{},
        }
}

// pilotVerifiedCapabilities maps every voice_sip requirement to a verified
// live fact (the synthetic probe path of the edge-case matrix).
func pilotVerifiedCapabilities() map[string]CapabilityFact {
        facts := map[string]CapabilityFact{}
        for _, requirement := range pilotRouteByID("voice_sip").RequiredCapabilities {
                facts[requirement.ID] = CapabilityFact{
                        Capability: requirement.ID, State: "verified",
                        Reason: "read_only_probe passed", Gap: false, Owner: "Technical Lead",
                }
        }
        return facts
}

// pilotStateInstance builds one tenant-level instance record in the given
// state with a distinct time (the newest-wins resolution stays honest).
func pilotStateInstance(definitionID, state string, at time.Time) GateInstance {
        return GateInstance{
                TenantID: readinessTenant, EmployeeID: readinessEmployee, InstanceID: readinessApprover,
                DefinitionID: definitionID, State: state,
                ApprovedBy: []GateApproval{{ParticipantID: profileOwner, Role: GateRoleOleg}},
                EvidenceRefs: []string{"evidence://" + strings.ToLower(definitionID) + "/" + state},
                ActorID:      profileOwner, RecordedAt: at,
        }
}

func pilotRouteByID(routeID string) PilotRoute {
        route, ok := PilotRouteByID(routeID)
        if !ok {
                panic("unknown pilot route: " + routeID)
        }
        return route
}

func pilotAdmissionOf(t *testing.T, report PilotAdmissionReport, routeID string) PilotRouteAdmission {
        t.Helper()
        for _, admission := range report.Routes {
                if admission.RouteID == routeID {
                        return admission
                }
        }
        t.Fatalf("route %s missing from the report", routeID)
        return PilotRouteAdmission{}
}

func pilotBlockerOf(t *testing.T, admission PilotRouteAdmission, source, id string) PilotBlocker {
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

func TestPilotRouteDictionaryIsFixed(t *testing.T) {
        routes := PilotRoutes()
        if len(routes) != 6 {
                t.Fatalf("the pilot dictionary carries exactly 6 route words, got %d", len(routes))
        }
        want := []string{"text_jivo", "voice_sip", "parts_catalog_abcp", "parts_order_abcp", "abcp_1c_sync", "human_fallback"}
        for index, routeID := range want {
                if routes[index].ID != routeID {
                        t.Fatalf("route %d = %s, want %s (fixed order)", index, routes[index].ID, routeID)
                }
                if routes[index].Title == "" {
                        t.Fatalf("%s must carry a title", routeID)
                }
        }
        if _, known := PilotRouteByID("supplier_form"); known {
                t.Fatal("an out-of-dictionary route word must not resolve (the Fixed Oleg pilot fence)")
        }
        if _, known := PilotRouteByID("text_telegram"); known {
                t.Fatal("an out-of-dictionary route word must not resolve")
        }
        counts := map[string]int{}
        for _, route := range routes {
                counts[route.ID] = len(route.RequiredCapabilities)
        }
        if counts["text_jivo"] != 8 || counts["voice_sip"] != 6 || counts["parts_catalog_abcp"] != 6 || counts["parts_order_abcp"] != 1 {
                t.Fatalf("capability counts: %+v", counts)
        }
        if !pilotRouteByID("abcp_1c_sync").MatrixDependent || len(pilotRouteByID("abcp_1c_sync").RequiredCapabilities) != 0 {
                t.Fatal("abcp_1c_sync is matrix-dependent with no catalog capability requirements")
        }
        if len(pilotRouteByID("human_fallback").RequiredCapabilities) != 0 || pilotRouteByID("human_fallback").MatrixDependent {
                t.Fatal("human_fallback needs no external connector proof and no matrix")
        }
        if len(pilotRouteByID("parts_order_abcp").FenceWords) != 1 {
                t.Fatal("parts_order_abcp carries the pinned order-fence word")
        }
        if len(pilotRouteByID("human_fallback").FallbackWords) != 2 {
                t.Fatal("human_fallback carries the pinned FR-83/OQ-17 fallback words")
        }
}

func TestPilotRouteGatesMirrorTheRegistryDictionary(t *testing.T) {
        for _, route := range pilotRoutes {
                for _, gateID := range route.RequiredGates {
                        definition, known := GateDefinitionByID(gateID)
                        if !known {
                                t.Fatalf("route %s references gate %s outside the registry dictionary", route.ID, gateID)
                        }
                        word, worded := pilotGateWords[gateID]
                        if !worded {
                                t.Fatalf("gate %s of route %s has no pinned pilot word (dictionary integrity)", gateID, route.ID)
                        }
                        if word.Close == "" || word.SafeState == "" || word.NextOwner == "" {
                                t.Fatalf("gate %s word is incomplete: %+v", gateID, word)
                        }
                        if definition.RouteScoped && gateID != "OQ-11" {
                                t.Fatalf("only OQ-11 is route-scoped (canon fixed), got %s", gateID)
                        }
                }
        }
        // AG-4 is the reused platform definition — present exactly once.
        seen := 0
        for _, definition := range gateDictionary {
                if definition.ID == "AG-4" {
                        seen++
                }
        }
        if seen != 1 {
                t.Fatalf("AG-4 appears %d times — the pilot set reuses the platform definition, duplicates are impossible", seen)
        }
}

func TestPilotMatrixDictionaryIsFixed(t *testing.T) {
        fields := MatrixFields()
        if len(fields) != 8 {
                t.Fatalf("the ABCP→1C matrix declares 8 material fields, got %d", len(fields))
        }
        owners := map[string]bool{MatrixOwnerABCP: true, MatrixOwner1C: true, MatrixOwnerStaffyCase: true}
        for _, field := range fields {
                if !owners[field.OwningSystem] {
                        t.Fatalf("field %s owning system %s outside the allowed set", field.Field, field.OwningSystem)
                }
                if field.ExternalIDText == "" || field.VersionTimeText == "" || field.AllowedProjectionText == "" ||
                        field.ConflictRuleText == "" || field.ReconcileQueryText == "" || field.TerminalEvidenceText == "" {
                        t.Fatalf("field %s must carry every AC-10 dimension", field.Field)
                }
        }
        if fields[0].Field != "client_order_state" || fields[2].Field != "external_1c_id" || fields[7].Field != "terminal_evidence" {
                t.Fatalf("matrix order pinned: %+v", fields)
        }
}

// --- The ladder: rows of the edge-case matrix -------------------------------

func TestDerivePilotAdmissionPackMissingBlocksEveryRoute(t *testing.T) {
        facts := pilotFacts()
        facts.PackID = ""
        facts.PackVersion = ""
        report := DerivePilotAdmission(facts)
        if report.PackPinned {
                t.Fatal("no pack pin must not pass")
        }
        if report.PackWord != PilotPackMissingWord {
                t.Fatalf("pack word = %q", report.PackWord)
        }
        for _, admission := range report.Routes {
                if admission.Stage != PilotStageCandidate {
                        t.Fatalf("route %s stage = %s, want candidate (pack fail-closed)", admission.RouteID, admission.Stage)
                }
                blocker := pilotBlockerOf(t, admission, "pack", PilotRequiredPackID)
                if blocker.Word != PilotPackMissingWord {
                        t.Fatalf("route %s pack blocker word = %q", admission.RouteID, blocker.Word)
                }
        }
        // A pinned but older pack is still a block (auto-parts@1.0.0 < 2.0.0).
        old := pilotFacts()
        old.PackVersion = "1.0.0"
        if report := DerivePilotAdmission(old); report.PackPinned {
                t.Fatal("auto-parts@1.0.0 is below the required 2.0.0 pin")
        }
        // A foreign pack is a block.
        foreign := pilotFacts()
        foreign.PackID = "montagio-furniture"
        if report := DerivePilotAdmission(foreign); report.PackPinned {
                t.Fatal("a foreign pack never satisfies the auto-parts pin")
        }
}

func TestDerivePilotAdmissionAdapterNotImplementedCapsAtCandidate(t *testing.T) {
        report := DerivePilotAdmission(pilotFacts())
        text := pilotAdmissionOf(t, report, "text_jivo")
        if text.Stage != PilotStageCandidate {
                t.Fatalf("text_jivo stage = %s, want candidate (adapter not implemented)", text.Stage)
        }
        blocker := pilotBlockerOf(t, text, "capability", "event.inbound-message")
        if !strings.HasPrefix(blocker.Word, "Jivo-адаптер не реализован (AG-4): capabilities не проверяемы — ") {
                t.Fatalf("jivo blocker word = %q", blocker.Word)
        }
        if blocker.NextOwner != "реализация адаптера после OQ-2 PASS" {
                t.Fatalf("jivo next step = %q", blocker.NextOwner)
        }
        parts := pilotAdmissionOf(t, report, "parts_order_abcp")
        orderBlocker := pilotBlockerOf(t, parts, "capability", "command.prepare-client-order")
        if !strings.HasPrefix(orderBlocker.Word, "ABCP-адаптер не реализован (AG-4): мутирующая capability не проверяема — probe запрещён") {
                t.Fatalf("order blocker word = %q", orderBlocker.Word)
        }
        if len(pilotAdmissionOf(t, report, "text_jivo").Capabilities) != 8 {
                t.Fatal("text_jivo lists all eight required capabilities")
        }
}

func TestDerivePilotAdmissionSyntheticVerifiedOpenGatesStaysBelowAdmitted(t *testing.T) {
        facts := pilotFacts()
        facts.Capabilities = pilotVerifiedCapabilities()
        report := DerivePilotAdmission(facts)
        voice := pilotAdmissionOf(t, report, "voice_sip")
        if voice.Stage != PilotStageVerified {
                t.Fatalf("voice_sip stage = %s, want verified (capabilities proven, gates open)", voice.Stage)
        }
        oq1 := pilotBlockerOf(t, voice, "gate", "OQ-1")
        if oq1.Word != "OQ-1 missing: провайдер не подтверждён фактическими данными аккаунта — Mango Office остаётся кандидатом — применимого инстанса нет" {
                t.Fatalf("OQ-1 blocker word = %q", oq1.Word)
        }
        if oq1.SafeState != "awaiting_verification/limited; неподдерживаемые transfer/caller-заявления запрещены" {
                t.Fatalf("OQ-1 safe state = %q", oq1.SafeState)
        }
        if oq1.NextOwner != "Олег + Technical Lead" {
                t.Fatalf("OQ-1 next owner = %q", oq1.NextOwner)
        }
        // Every required gate shows its honest blocker; none is hidden. The
        // route-scoped OQ-11 emits one view per declared channel route
        // (inbound + outbound), so eight required gates yield nine views.
        if len(voice.Gates) != 9 {
                t.Fatalf("voice_sip gate views = %d, want 9 (8 gates, OQ-11 per direction)", len(voice.Gates))
        }
        for _, view := range voice.Gates {
                if view.State == GateStatePass {
                        t.Fatalf("gate %s must not pass in the empty world", view.ID)
                }
        }
        // The OQ-11 requirement resolves per channel route — both directions.
        oq11Views := 0
        for _, view := range voice.Gates {
                if view.ID == "OQ-11" {
                        oq11Views++
                        if !strings.Contains(view.Word, "маршрут phone:inbound") && !strings.Contains(view.Word, "маршрут phone:outbound") {
                                t.Fatalf("OQ-11 view word must name the channel route: %q", view.Word)
                        }
                }
        }
        if oq11Views != 2 {
                t.Fatalf("OQ-11 views = %d, want 2 (inbound + outbound)", oq11Views)
        }
        // Independence: the text route is not lifted by the voice evidence.
        if text := pilotAdmissionOf(t, report, "text_jivo"); text.Stage != PilotStageCandidate {
                t.Fatalf("text_jivo stage = %s, want candidate (its own capabilities unproven)", text.Stage)
        }
}

func TestDerivePilotAdmissionGateStatesNeverAdmit(t *testing.T) {
        for _, state := range []string{GateStateOpen, GateStateInTest, GateStateFail, GateStateExpired} {
                facts := pilotFacts()
                facts.Capabilities = pilotVerifiedCapabilities()
                at := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
                // All other voice gates PASS; OQ-1 sits in the tested state.
                for _, gateID := range pilotRouteByID("voice_sip").RequiredGates {
                        if gateID == "OQ-1" || gateID == "OQ-11" {
                                continue
                        }
                        facts.Instances = append(facts.Instances, passInstance(gateID, nil, ""))
                }
                facts.Instances = append(facts.Instances, pilotStateInstance("OQ-1", state, at))
                for _, channelRoute := range pilotRouteByID("voice_sip").RequiredOQ11Routes {
                        route := channelRoute
                        facts.Instances = append(facts.Instances, passInstance("OQ-11", &route, ""))
                }
                report := DerivePilotAdmission(facts)
                voice := pilotAdmissionOf(t, report, "voice_sip")
                if voice.Stage == PilotStageAdmitted {
                        t.Fatalf("OQ-1 %s must never admit", state)
                }
                blocker := pilotBlockerOf(t, voice, "gate", "OQ-1")
                if !strings.HasPrefix(blocker.Word, "OQ-1 "+state+": ") {
                        t.Fatalf("OQ-1 %s blocker word = %q", state, blocker.Word)
                }
        }
}

func TestDerivePilotAdmissionAllPassAdmitsHumanFallbackOnly(t *testing.T) {
        facts := pilotFacts()
        at := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
        facts.Instances = append(facts.Instances, passInstance("AG-2", nil, ""))
        facts.Instances = append(facts.Instances, passInstance("AG-4", nil, ""))
        report := DerivePilotAdmission(facts)
        fallback := pilotAdmissionOf(t, report, "human_fallback")
        if fallback.Stage != PilotStageAdmitted {
                t.Fatalf("human_fallback stage = %s, want admitted (the vacuous capability case is honest: no external proof needed)", fallback.Stage)
        }
        if len(fallback.Blockers) != 0 {
                t.Fatalf("human_fallback blockers = %+v", fallback.Blockers)
        }
        // Every other route stays below admitted — the conjunctive path.
        for _, admission := range report.Routes {
                if admission.RouteID == "human_fallback" {
                        continue
                }
                if admission.Stage == PilotStageAdmitted {
                        t.Fatalf("route %s cannot be admitted without its own proof", admission.RouteID)
                }
        }
        // One gate missing drops the route to verified, never admitted.
        oneMissing := pilotFacts()
        oneMissing.Instances = append(oneMissing.Instances, passInstance("AG-2", nil, ""))
        if report := DerivePilotAdmission(oneMissing); pilotAdmissionOf(t, report, "human_fallback").Stage != PilotStageVerified {
                t.Fatal("one missing gate keeps human_fallback at verified")
        }
        _ = at
}

func TestDerivePilotAdmissionStaleEvidenceBlocksOnlyAffectedRoute(t *testing.T) {
        facts := pilotFacts()
        facts.Capabilities = pilotVerifiedCapabilities()
        stale := facts.Capabilities["query.trunk-status"]
        stale.State = "stale"
        stale.Reason = "evidence older than the freshness window"
        facts.Capabilities["query.trunk-status"] = stale
        report := DerivePilotAdmission(facts)
        voice := pilotAdmissionOf(t, report, "voice_sip")
        if voice.Stage != PilotStageCandidate {
                t.Fatalf("voice_sip stage = %s, want candidate (stale evidence drops the ladder)", voice.Stage)
        }
        blocker := pilotBlockerOf(t, voice, "capability", "query.trunk-status")
        if !strings.Contains(blocker.Word, "доказательство устарело — требуется повторная проверка") {
                t.Fatalf("stale word = %q", blocker.Word)
        }
        // The matrix route is untouched by the voice evidence.
        if sync := pilotAdmissionOf(t, report, "abcp_1c_sync"); sync.Stage != PilotStageCandidate {
                t.Fatalf("abcp_1c_sync stage = %s, want candidate (independent derivation)", sync.Stage)
        }
}

func TestDerivePilotAdmissionOQ19FailBlocksOnlyMatrixConsumers(t *testing.T) {
        facts := pilotFacts()
        at := time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC)
        facts.Instances = append(facts.Instances, pilotStateInstance("OQ-19", GateStateFail, at))
        report := DerivePilotAdmission(facts)
        for _, routeID := range []string{"abcp_1c_sync", "parts_order_abcp"} {
                admission := pilotAdmissionOf(t, report, routeID)
                blocker := pilotBlockerOf(t, admission, "gate", "OQ-19")
                if !strings.HasPrefix(blocker.Word, "OQ-19 fail: материальные поля обмена не имеют терминальных доказательств") {
                        t.Fatalf("route %s OQ-19 word = %q", routeID, blocker.Word)
                }
        }
        catalog := pilotAdmissionOf(t, report, "parts_catalog_abcp")
        for _, blocker := range catalog.Blockers {
                if blocker.Source == "gate" && blocker.ID == "OQ-19" {
                        t.Fatal("parts_catalog_abcp does not require OQ-19 — the block never extends to another route")
                }
        }
}

func TestDerivePilotAdmissionOQ3InTestCarriesTheExactFenceWord(t *testing.T) {
        facts := pilotFacts()
        at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
        facts.Instances = append(facts.Instances, pilotStateInstance("OQ-3", GateStateInTest, at))
        report := DerivePilotAdmission(facts)
        order := pilotAdmissionOf(t, report, "parts_order_abcp")
        blocker := pilotBlockerOf(t, order, "gate", "OQ-3")
        if !strings.HasPrefix(blocker.Word, "OQ-3 in_test: ABCP prepared order не закрыт") {
                t.Fatalf("OQ-3 in_test word = %q", blocker.Word)
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
}

func TestDerivePilotAdmissionMatrixStaysUnknownEvenWithGatesPassed(t *testing.T) {
        facts := pilotFacts()
        // Even a world where both matrix gates PASS cannot lift the matrix
        // route: no terminal-evidence source exists this slice.
        facts.Instances = append(facts.Instances, passInstance("OQ-4", nil, ""))
        facts.Instances = append(facts.Instances, passInstance("OQ-19", nil, ""))
        report := DerivePilotAdmission(facts)
        sync := pilotAdmissionOf(t, report, "abcp_1c_sync")
        if sync.Stage != PilotStageCandidate {
                t.Fatalf("abcp_1c_sync stage = %s, want candidate (unknown stays unknown — no vacuous admission)", sync.Stage)
        }
        blocker := pilotBlockerOf(t, sync, "matrix", "abcp_1c_matrix")
        if blocker.Word != "материальные поля обмена ABCP→1С остаются unknown — терминальные доказательства отсутствуют (OQ-19)" {
                t.Fatalf("matrix word = %q", blocker.Word)
        }
        if len(report.Matrix) != 8 {
                t.Fatalf("matrix fields = %d, want 8", len(report.Matrix))
        }
        for _, field := range report.Matrix {
                if field.StateWord != MatrixUnknownWord {
                        t.Fatalf("field %s state word = %q", field.Field, field.StateWord)
                }
        }
}

func TestDerivePilotAdmissionOQ11ExactRouteScope(t *testing.T) {
        facts := pilotFacts()
        facts.Capabilities = pilotVerifiedCapabilities()
        for _, gateID := range pilotRouteByID("voice_sip").RequiredGates {
                if gateID == "OQ-11" {
                        continue
                }
                facts.Instances = append(facts.Instances, passInstance(gateID, nil, ""))
        }
        // OQ-11 PASS covers phone:inbound only — the outbound half stays open.
        inbound := Route{Channel: "phone", Direction: DirectionInbound}
        facts.Instances = append(facts.Instances, passInstance("OQ-11", &inbound, ""))
        report := DerivePilotAdmission(facts)
        voice := pilotAdmissionOf(t, report, "voice_sip")
        if voice.Stage == PilotStageAdmitted {
                t.Fatal("a half-covered OQ-11 route scope must not admit (exact scope, no wildcards)")
        }
        blockedOutbound := false
        for _, blocker := range voice.Blockers {
                if blocker.Source == "gate" && blocker.ID == "OQ-11" && strings.Contains(blocker.Word, "phone:outbound") {
                        blockedOutbound = true
                }
        }
        if !blockedOutbound {
                t.Fatalf("the outbound half must carry its own blocker: %+v", voice.Blockers)
        }
        // A foreign channel PASS never satisfies the requirement.
        foreign := pilotFacts()
        foreign.Capabilities = pilotVerifiedCapabilities()
        for _, gateID := range pilotRouteByID("voice_sip").RequiredGates {
                if gateID == "OQ-11" {
                        continue
                }
                foreign.Instances = append(foreign.Instances, passInstance(gateID, nil, ""))
        }
        foreignChannel := Route{Channel: "jivo", Direction: DirectionInbound}
        foreign.Instances = append(foreign.Instances, passInstance("OQ-11", &foreignChannel, ""))
        if report := DerivePilotAdmission(foreign); pilotAdmissionOf(t, report, "voice_sip").Stage == PilotStageAdmitted {
                t.Fatal("a jivo-scoped OQ-11 PASS never satisfies the phone data route")
        }
}

func TestDerivePilotAdmissionOQ11BackendScopeBoundary(t *testing.T) {
        report := DerivePilotAdmission(pilotFacts())
        catalog := pilotAdmissionOf(t, report, "parts_catalog_abcp")
        blocker := pilotBlockerOf(t, catalog, "gate", "OQ-11")
        if !strings.HasPrefix(blocker.Word, "OQ-11 missing: data-маршрут ABCP не имеет канального scope") {
                t.Fatalf("backend scope word = %q", blocker.Word)
        }
}

func TestDerivePilotAdmissionIsIdempotent(t *testing.T) {
        facts := pilotFacts()
        facts.Capabilities = pilotVerifiedCapabilities()
        facts.Instances = append(facts.Instances, passInstance("AG-2", nil, ""))
        first, err := json.Marshal(DerivePilotAdmission(facts))
        if err != nil {
                t.Fatal(err)
        }
        second, err := json.Marshal(DerivePilotAdmission(facts))
        if err != nil {
                t.Fatal(err)
        }
        if string(first) != string(second) {
                t.Fatal("an unchanged world must yield the byte-stable same derivation")
        }
        // The stage vocabulary is fixed: every emitted stage is a dictionary word.
        allowed := map[string]bool{PilotStageCandidate: true, PilotStageVerified: true, PilotStageAdmitted: true}
        for _, admission := range DerivePilotAdmission(facts).Routes {
                if !allowed[admission.Stage] {
                        t.Fatalf("route %s emitted an out-of-dictionary stage %q", admission.RouteID, admission.Stage)
                }
                if admission.StageWord == "" {
                        t.Fatalf("route %s stage word missing", admission.RouteID)
                }
        }
}

func TestPackVersionAtLeast(t *testing.T) {
        cases := []struct {
                version, minimum string
                want             bool
        }{
                {"2.0.0", "2.0.0", true},
                {"2.1.0", "2.0.0", true},
                {"3.0.0", "2.0.0", true},
                {"1.9.9", "2.0.0", false},
                {"2.0", "2.0.0", true},
                {"", "2.0.0", false},
                {"x.y.z", "2.0.0", false},
        }
        for _, testCase := range cases {
                if got := packVersionAtLeast(testCase.version, testCase.minimum); got != testCase.want {
                        t.Fatalf("packVersionAtLeast(%q, %q) = %v, want %v", testCase.version, testCase.minimum, got, testCase.want)
                }
        }
}

func TestPilotCapabilityAndGateIDHelpers(t *testing.T) {
        if ids := PilotCapabilityIDs("text_jivo"); len(ids) != 8 || ids[0] != "event.inbound-message" || ids[7] != "query.fallback-state" {
                t.Fatalf("text_jivo capability ids = %+v", ids)
        }
        if ids := PilotCapabilityIDs("abcp_1c_sync"); len(ids) != 0 {
                t.Fatalf("abcp_1c_sync capability ids = %+v", ids)
        }
        if ids := PilotRouteGateIDs("human_fallback"); len(ids) != 2 || ids[0] != "AG-2" || ids[1] != "AG-4" {
                t.Fatalf("human_fallback gate ids = %+v", ids)
        }
        if ids := PilotCapabilityIDs("supplier_form"); ids != nil {
                t.Fatalf("out-of-dictionary route resolves to %v", ids)
        }
        if ids := PilotRouteGateIDs("unknown"); ids != nil {
                t.Fatalf("out-of-dictionary route resolves to %v", ids)
        }
}
