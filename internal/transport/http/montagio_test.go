package httptransport

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// montagioPath builds the machine Montagio admission path of the fixture tenant.
func montagioPath(suffix string) string {
	return "/v1/tenants/" + testTenantID.String() + "/montagio/admission" + suffix
}

// montagioAdmissionViewWithPack builds a service view over the honest domain
// derivation of a Montagio world: the pack pinned, every route capability
// verified and the gates PASS-less (the words the wire tests pin).
func montagioAdmissionViewWithPack(t *testing.T, tenant id.UUID) employeeAPI.MontagioAdmissionView {
	t.Helper()
	capabilities := map[string]employeeDomain.CapabilityFact{}
	for _, route := range employeeDomain.MontagioRoutes() {
		for _, requirement := range route.RequiredCapabilities {
			capabilities[requirement.ID] = employeeDomain.CapabilityFact{
				Capability: requirement.ID, State: "verified", Reason: "read_only_probe passed", Gap: false, Owner: "Technical Lead",
			}
		}
	}
	report := employeeDomain.DeriveMontagioAdmission(employeeDomain.MontagioFacts{
		PackID: employeeDomain.MontagioRequiredPackID, PackVersion: "1.0.0",
		Capabilities: capabilities, Instances: []employeeDomain.GateInstance{},
	})
	return employeeAPI.MontagioAdmissionView{TenantID: tenant, Report: report, CanManage: true, ViewScope: employeeAPI.ReadinessScopeManage}
}

// TestMontagioAdmissionWireShapeMatchesContract pins the machine view of the
// spec 5-1 surface: the documented keys of the OpenAPI MontagioAdmissionView
// travel the wire — a contract without wire checks passes for the wrong
// reason (the 1-13 M1 lesson).
func TestMontagioAdmissionWireShapeMatchesContract(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	fake.montagio = montagioAdmissionViewWithPack(t, testTenantID)
	fake.montagioSet = true

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, montagioPath(""), ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	decodeBody(t, response, &body)
	for _, key := range []string{"tenant_id", "montagio_admission", "can_manage", "view_scope"} {
		if _, ok := body[key]; !ok {
			t.Errorf("the view must carry the contract key %q, got %v", key, body)
		}
	}
	report, ok := body["montagio_admission"].(map[string]any)
	if !ok {
		t.Fatalf("montagio_admission must be a mapping, got %T", body["montagio_admission"])
	}
	for _, key := range []string{"pack_pinned", "pack_word", "wording_words", "montagio_gate_ids", "routes"} {
		if _, ok := report[key]; !ok {
			t.Errorf("the report must carry the contract key %q, got %v", key, report)
		}
	}
	gates, ok := report["montagio_gate_ids"].([]any)
	if !ok || len(gates) != 6 {
		t.Fatalf("the Montagio gate dictionary mirror carries 6 words, got %v", report["montagio_gate_ids"])
	}
	if gates[0] != "OQ-7A" || gates[5] != "OQ-18" {
		t.Fatalf("the gate dictionary order is fixed, got %v", gates)
	}
	routes, ok := report["routes"].([]any)
	if !ok || len(routes) != 8 {
		t.Fatalf("the fixed route dictionary carries 8 words, got %v", report["routes"])
	}
	first, ok := routes[0].(map[string]any)
	if !ok {
		t.Fatalf("route row must be a mapping, got %T", routes[0])
	}
	for _, key := range []string{"route_id", "title", "stage", "stage_word", "modality_word", "intents_word", "autonomy_word", "human_inbox_word", "fallback_route_word", "capabilities", "gates", "live_gates", "blockers"} {
		if _, ok := first[key]; !ok {
			t.Errorf("the route row must carry the contract key %q, got %v", key, first)
		}
	}
	if first["route_id"] != "novofon_voice" {
		t.Fatalf("the dictionary order is fixed: first = novofon_voice, got %v", first["route_id"])
	}
}

// TestMontagioAdmissionAnswersTheHonestWords pins the machine honest words of
// the honest worlds: the pack missing word, the Novofon adapter honesty word,
// the eternal Avito cap word, the AC-11 wording dictionary and the honest
// stage ladder of the empty world.
func TestMontagioAdmissionAnswersTheHonestWords(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	fake.montagio = employeeAPI.MontagioAdmissionView{
		TenantID: testTenantID, CanManage: false, ViewScope: employeeAPI.ReadinessScopeView,
		Report: employeeDomain.DeriveMontagioAdmission(employeeDomain.MontagioFacts{
			PackID: "", PackVersion: "", Capabilities: map[string]employeeDomain.CapabilityFact{}, Instances: []employeeDomain.GateInstance{},
		}),
	}
	fake.montagioSet = true

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, montagioPath(""), ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Report employeeDomain.MontagioAdmissionReport `json:"montagio_admission"`
	}
	decodeBody(t, response, &body)
	if body.Report.PackWord != employeeDomain.MontagioPackMissingWord {
		t.Fatalf("pack word = %q", body.Report.PackWord)
	}
	for _, route := range body.Report.Routes {
		if route.Stage != employeeDomain.MontagioStageCandidate {
			t.Fatalf("route %s stage = %s, want candidate in the empty world", route.RouteID, route.Stage)
		}
	}
	// The honest words travel the wire: the Novofon adapter honesty, the
	// eternal Avito cap and the AC-11 wording dictionary.
	var novofonBlocker, avitoCap bool
	for _, route := range body.Report.Routes {
		if route.RouteID == "novofon_voice" {
			for _, blocker := range route.Blockers {
				if strings.HasPrefix(blocker.Word, "Novofon-адаптер не реализован (AG-4)") {
					novofonBlocker = true
				}
			}
		}
		if route.RouteID == "avito_candidate" && strings.Contains(route.CapWord, "scraping") {
			avitoCap = true
		}
	}
	if !novofonBlocker {
		t.Fatal("the novofon blocker must carry the adapter honesty word (the general documentation is never a proof)")
	}
	if !avitoCap {
		t.Fatal("avito_candidate must carry the eternal cap word")
	}
	if len(body.Report.WordingWords) != 5 {
		t.Fatalf("the AC-11 wording dictionary travels the wire (5 words), got %d", len(body.Report.WordingWords))
	}
	if !strings.Contains(strings.Join(body.Report.WordingWords, "|"), "best-effort без SLA") {
		t.Fatalf("the OQ-13A best-effort wording travels the wire: %v", body.Report.WordingWords)
	}
}

// TestMontagioAdmissionSpeaksThe404And403Words pins the honest problem ladder:
// the tenant parse failure and the service refusal map to the readiness
// problem words (the reused rights of the test & release surface).
func TestMontagioAdmissionSpeaksThe404And403Words(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	// A malformed tenant id: the honest 404 before any material.
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/tenants/not-a-uuid/montagio/admission", nil)
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("a malformed tenant must answer 404, got %d", response.Code)
	}
	// The service refusal: the honest 403 of the reused view scope.
	fake.montagioErr = employeeDomain.ErrReadinessForbidden
	fake.montagioSet = true
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, montagioPath(""), ""))
	if response.Code != http.StatusForbidden {
		t.Fatalf("the reused scope refusal must answer 403, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "readiness_forbidden") {
		t.Fatalf("the problem code must name the reused surface: %s", response.Body.String())
	}
	// The service 404: the honest membership-before-material word.
	fake.montagioErr = employeeDomain.ErrNotFound
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, montagioPath(""), ""))
	if response.Code != http.StatusNotFound {
		t.Fatalf("the membership refusal must answer 404, got %d", response.Code)
	}
}

// TestMontagioAdmissionIgnoresFreeQueryFields pins the empty read discipline:
// a GET without a body never yields 400, and unknown query fields stay
// ignored (strict decode belongs to command bodies only).
func TestMontagioAdmissionIgnoresFreeQueryFields(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	fake.montagio = montagioAdmissionViewWithPack(t, testTenantID)
	fake.montagioSet = true
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, montagioPath("?stage=live&detail=full"), ""))
	if response.Code != http.StatusOK {
		t.Fatalf("free query fields must stay ignored, got %d: %s", response.Code, response.Body.String())
	}
}

// TestMontagioPageRendersTheStagesAndOwners pins the accessible page: the four
// stages read as text, the next owners travel, the cap and fence words render,
// the AC-11 wording dictionary renders as a section and the column words of
// the Route Matrix travel.
func TestMontagioPageRendersTheStagesAndOwners(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	fake.montagio = montagioAdmissionViewWithPack(t, testTenantID)
	fake.montagioSet = true

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/montagio", nil)
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	page := response.Body.String()
	for _, want := range []string{
		"Пилот услуг Montagio: реестр допуска",
		"кандидат", "проверен", // the stages read as text (AC-13)
		"контролируемый реальный клиент", // the 4-stage ladder distinguishes controlled (AC-13)
		"классы автономности задаёт версионированный Operating Profile", // the autonomy column word
		"единый подтверждённый Human Inbox",                             // the human inbox column word
		"scraping, имитация клиента и обход ограничений платформы запрещены", // the eternal Avito cap (AC-8)
		"закрытые validation-сценарии НЕ блокируются",                        // the OQ-10 fence (AC-12)
		"Утверждённые правдивые формулировки (AC-11)",                        // the wording dictionary section
		"best-effort без SLA",                                                // the OQ-13A wording
		"К основному содержанию",                                             // the skip link of the a11y skeleton
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the montagio page must contain %q", want)
		}
	}
	// Secrets never travel: the page carries no connection config words.
	if strings.Contains(page, "api_key") || strings.Contains(page, "sandbox-") {
		t.Error("the page must never render secret field names or values")
	}
	// No Avito capability claims exist anywhere on the page.
	if strings.Contains(page, "query.avito") {
		t.Error("the page must never render Avito capability claims (AC-8)")
	}
}

// TestMontagioOpenAPIContractMirrorsDictionaries pins the contract mirrors:
// the montagio admission path, the closed shapes (additionalProperties false)
// and the enum mirrors — routes 8 / Montagio gates 6 / stages 4 — against the
// live domain dictionaries (a drift is red).
func TestMontagioOpenAPIContractMirrorsDictionaries(t *testing.T) {
	contract, err := os.ReadFile("../../../contracts/openapi/staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contract)
	for _, pin := range []string{
		"/v1/tenants/{tenant_id}/montagio/admission",
		"readMontagioAdmission",
		"MontagioAdmissionView",
		"MontagioAdmissionReport",
		"MontagioRouteView",
		"MontagioGateRequirementView",
		"MontagioBlockerView",
		"required: [tenant_id, montagio_admission, can_manage, view_scope]",
		"required: [pack_pinned, pack_word, wording_words, montagio_gate_ids, routes]",
		"required: [route_id, title, stage, stage_word, modality_word, intents_word, autonomy_word, human_inbox_word, fallback_route_word]",
	} {
		if !strings.Contains(text, pin) {
			t.Errorf("the contract must mirror %q", pin)
		}
	}
	// The enum mirrors against the live domain dictionaries.
	routes := employeeDomain.MontagioRoutes()
	var routeIDs []string
	for _, route := range routes {
		routeIDs = append(routeIDs, route.ID)
	}
	routeEnum := "enum: [" + strings.Join(routeIDs, ", ") + "]"
	if !strings.Contains(text, routeEnum) {
		t.Errorf("the route enum must mirror the live 8-word dictionary, want %q", routeEnum)
	}
	gateEnum := "enum: [" + strings.Join(employeeDomain.MontagioGateDictionaryIDs(), ", ") + "]"
	if !strings.Contains(text, gateEnum) {
		t.Errorf("the gate enum must mirror the live 6-word Montagio dictionary, want %q", gateEnum)
	}
	stageEnum := "enum: [candidate, verified, controlled, live]"
	if !strings.Contains(text, stageEnum) {
		t.Errorf("the stage enum must mirror the four-stage ladder, want %q", stageEnum)
	}
	// The closed-shape discipline: every new Montagio schema forbids extra
	// properties.
	for _, schema := range []string{"MontagioAdmissionView:", "MontagioAdmissionReport:", "MontagioRouteView:", "MontagioGateRequirementView:", "MontagioBlockerView:"} {
		index := strings.Index(text, schema)
		if index < 0 {
			t.Fatalf("the schema %s must exist", schema)
		}
		block := text[index : index+400]
		if !strings.Contains(block, "additionalProperties: false") {
			t.Errorf("the schema %s must close the shape (additionalProperties: false)", schema)
		}
	}
	// The blocker source enum carries the cap source of the capped routes.
	if !strings.Contains(text, "enum: [pack, cap, capability, gate, live-gate]") {
		t.Error("the blocker source enum must carry the cap and live-gate sources")
	}
}
