package httptransport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// pilotPath builds the machine pilot admission path of the fixture tenant.
func pilotPath(suffix string) string {
	return "/v1/tenants/" + testTenantID.String() + "/pilot/admission" + suffix
}

// pilotAdmissionViewWithPack builds a service view over the honest domain
// derivation of a pilot world: the pack pinned, the synthetic voice
// capabilities verified and the platform gates PASS-less (the words the
// wire tests pin).
func pilotAdmissionViewWithPack(t *testing.T, tenant id.UUID) employeeAPI.PilotAdmissionView {
	t.Helper()
	capabilities := map[string]employeeDomain.CapabilityFact{}
	for _, route := range employeeDomain.PilotRoutes() {
		for _, requirement := range route.RequiredCapabilities {
			capabilities[requirement.ID] = employeeDomain.CapabilityFact{
				Capability: requirement.ID, State: "verified", Reason: "read_only_probe passed", Gap: false, Owner: "Technical Lead",
			}
		}
	}
	report := employeeDomain.DerivePilotAdmission(employeeDomain.PilotFacts{
		PackID: employeeDomain.PilotRequiredPackID, PackVersion: "2.0.0",
		Capabilities: capabilities, Instances: []employeeDomain.GateInstance{},
	})
	return employeeAPI.PilotAdmissionView{TenantID: tenant, Report: report, CanManage: true, ViewScope: employeeAPI.ReadinessScopeManage}
}

// TestPilotAdmissionWireShapeMatchesContract pins the machine view of the
// spec 4-1 surface: the documented keys of the OpenAPI PilotAdmissionView
// travel the wire — a contract without wire checks passes for the wrong
// reason (the 1-13 M1 lesson).
func TestPilotAdmissionWireShapeMatchesContract(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	fake.pilot = pilotAdmissionViewWithPack(t, testTenantID)
	fake.pilotSet = true

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, pilotPath(""), ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	decodeBody(t, response, &body)
	for _, key := range []string{"tenant_id", "pilot_admission", "can_manage", "view_scope"} {
		if _, ok := body[key]; !ok {
			t.Errorf("the view must carry the contract key %q, got %v", key, body)
		}
	}
	report, ok := body["pilot_admission"].(map[string]any)
	if !ok {
		t.Fatalf("pilot_admission must be a mapping, got %T", body["pilot_admission"])
	}
	for _, key := range []string{"pack_pinned", "pack_word", "routes", "matrix"} {
		if _, ok := report[key]; !ok {
			t.Errorf("the report must carry the contract key %q, got %v", key, report)
		}
	}
	routes, ok := report["routes"].([]any)
	if !ok || len(routes) != 6 {
		t.Fatalf("the fixed route dictionary carries 6 words, got %v", report["routes"])
	}
	first, ok := routes[0].(map[string]any)
	if !ok {
		t.Fatalf("route row must be a mapping, got %T", routes[0])
	}
	for _, key := range []string{"route_id", "title", "stage", "stage_word", "capabilities", "gates", "blockers"} {
		if _, ok := first[key]; !ok {
			t.Errorf("the route row must carry the contract key %q, got %v", key, first)
		}
	}
	if first["route_id"] != "text_jivo" {
		t.Fatalf("the dictionary order is fixed: first = text_jivo, got %v", first["route_id"])
	}
	matrix, ok := report["matrix"].([]any)
	if !ok || len(matrix) != 8 {
		t.Fatalf("the ABCP→1C matrix carries 8 material fields, got %v", report["matrix"])
	}
	field, ok := matrix[0].(map[string]any)
	if !ok {
		t.Fatalf("matrix row must be a mapping, got %T", matrix[0])
	}
	for _, key := range []string{"field", "title", "owning_system", "state_word"} {
		if _, ok := field[key]; !ok {
			t.Errorf("the matrix row must carry the contract key %q, got %v", key, field)
		}
	}
}

// TestPilotAdmissionAnswersTheHonestWords pins the machine honest words of
// the honest worlds: the pack missing word, the adapter honesty word, the
// fence words and the honest stage ladder.
func TestPilotAdmissionAnswersTheHonestWords(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	// The empty world: no pack, no connections, no instances.
	fake.pilot = employeeAPI.PilotAdmissionView{
		TenantID: testTenantID, CanManage: false, ViewScope: employeeAPI.ReadinessScopeView,
		Report: employeeDomain.DerivePilotAdmission(employeeDomain.PilotFacts{
			PackID: "", PackVersion: "", Capabilities: map[string]employeeDomain.CapabilityFact{}, Instances: []employeeDomain.GateInstance{},
		}),
	}
	fake.pilotSet = true

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, pilotPath(""), ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Pilot employeeDomain.PilotAdmissionReport `json:"pilot_admission"`
	}
	decodeBody(t, response, &body)
	if body.Pilot.PackWord != employeeDomain.PilotPackMissingWord {
		t.Fatalf("pack word = %q", body.Pilot.PackWord)
	}
	for _, route := range body.Pilot.Routes {
		if route.Stage != employeeDomain.PilotStageCandidate {
			t.Fatalf("route %s stage = %s, want candidate in the empty world", route.RouteID, route.Stage)
		}
	}
	// The adapter honesty word and the order fence word travel the wire.
	var jivoBlocker, orderFence bool
	for _, route := range body.Pilot.Routes {
		if route.RouteID == "text_jivo" {
			for _, blocker := range route.Blockers {
				if strings.HasPrefix(blocker.Word, "Jivo-адаптер не реализован (AG-4)") {
					jivoBlocker = true
				}
			}
		}
		if route.RouteID == "parts_order_abcp" {
			for _, fence := range route.FenceWords {
				if strings.HasPrefix(fence, "автоматическая отправка поставщику не выполняется") {
					orderFence = true
				}
			}
		}
	}
	if !jivoBlocker {
		t.Fatal("the jivo blocker must carry the adapter honesty word")
	}
	if !orderFence {
		t.Fatal("parts_order_abcp must carry the pinned order fence word")
	}
}

// TestPilotAdmissionSpeaksThe404And403Words pins the honest problem ladder:
// the tenant parse failure and the service refusal map to the readiness
// problem words (the reused rights of the test & release surface).
func TestPilotAdmissionSpeaksThe404And403Words(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	// A malformed tenant id: the honest 404 before any material.
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/tenants/not-a-uuid/pilot/admission", nil)
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("a malformed tenant must answer 404, got %d", response.Code)
	}
	// The service refusal: the honest 403 of the reused view scope.
	fake.pilotErr = employeeDomain.ErrReadinessForbidden
	fake.pilotSet = true
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, pilotPath(""), ""))
	if response.Code != http.StatusForbidden {
		t.Fatalf("the reused scope refusal must answer 403, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "readiness_forbidden") {
		t.Fatalf("the problem code must name the reused surface: %s", response.Body.String())
	}
	// The service 404: the honest membership-before-material word.
	fake.pilotErr = employeeDomain.ErrNotFound
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, pilotPath(""), ""))
	if response.Code != http.StatusNotFound {
		t.Fatalf("the membership refusal must answer 404, got %d", response.Code)
	}
}

// TestPilotAdmissionIgnoresFreeQueryFields pins the empty read discipline:
// a GET without a body never yields 400, and unknown query fields stay
// ignored (strict decode belongs to command bodies only).
func TestPilotAdmissionIgnoresFreeQueryFields(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	fake.pilot = pilotAdmissionViewWithPack(t, testTenantID)
	fake.pilotSet = true
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, pilotPath("?stage=live&detail=full"), ""))
	if response.Code != http.StatusOK {
		t.Fatalf("free query fields must stay ignored, got %d: %s", response.Code, response.Body.String())
	}
}

// TestPilotPageRendersTheStagesAndOwners pins the accessible page: the
// stages read as text, the next owners travel, the fence word renders and
// the matrix rows stay in closed details.
func TestPilotPageRendersTheStagesAndOwners(t *testing.T) {
	fake := &fakeReadiness{}
	server := newReadinessTestServer(t, fake)
	fake.pilot = pilotAdmissionViewWithPack(t, testTenantID)
	fake.pilotSet = true

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/pilot", nil)
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	page := response.Body.String()
	for _, want := range []string{
		"Пилот автозапчастей: реестр допуска",
		"кандидат", "проверен", // the stages read as text (AC-13)
		"Олег", "Technical Lead", // the next owner of OQ-1 (html/template renders the plus as &#43;)
		"нейтральный pending_manager_verification",          // the OQ-3 safe state word
		"автоматическая отправка поставщику не выполняется", // the order fence word (AC-9)
		"unknown — терминальное доказательство отсутствует", // the matrix unknown word (AC-10)
		"К основному содержанию",                            // the skip link of the a11y skeleton
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the pilot page must contain %q", want)
		}
	}
	// The matrix rows render inside closed details elements.
	if !strings.Contains(page, "<details") {
		t.Error("the matrix fields must render in closed details elements")
	}
	// Secrets never travel: the page carries no connection config words.
	if strings.Contains(page, "api_key") || strings.Contains(page, "sandbox-") {
		t.Error("the page must never render secret field names or values")
	}
}

// The compile-time pin: the fixture implements the extended service
// interface (the spec 4-1 method rides the reused surface).
var _ employeeAPI.ReadinessService = (*fakeReadiness)(nil)
