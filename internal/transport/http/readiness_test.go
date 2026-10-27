package httptransport

import (
        "context"
        "encoding/json"
        "io"
        "log/slog"
        "net/http"
        "net/http/httptest"
        "strings"
        "testing"

        employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
        employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
        "github.com/artemiosu/staffy/internal/identity/api"
        "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
)

// fakeReadiness implements the readiness service surface for the transport
// wire tests: honest minimal views and recorded commands. The tests assert
// what the HTTP boundary claims — the wire shape follows the OpenAPI
// contract (the 1-13 M1 lesson: a contract without wire checks passes for
// the wrong reason).
type fakeReadiness struct {
        view         employeeAPI.ReadinessView
        gates        employeeAPI.GateInstancesView
        history      employeeAPI.AdmissionHistoryView
        pilot        employeeAPI.PilotAdmissionView
        pilotSet     bool
        pilotErr     error
        montagio     employeeAPI.MontagioAdmissionView
        montagioSet  bool
        montagioErr  error
        viewErr      error
        gateErr      error
        publishErr   error
        gateCalls    int
        publishCalls int
}

func (f *fakeReadiness) ReadinessView(context.Context, domain.Principal, id.UUID, id.UUID, string) (employeeAPI.ReadinessView, error) {
        if f.viewErr != nil {
                return employeeAPI.ReadinessView{}, f.viewErr
        }
        if f.view.EmployeeID == "" {
                f.view.EmployeeID = testPrincipalID
                f.view.Stage = employeeDomain.StageSynthetic
                f.view.Report = employeeDomain.ReadinessReport{
                        Stage: employeeDomain.StageSynthetic, Status: employeeDomain.ReadinessReady, StatusText: "готов",
                        Elements: []employeeDomain.ReadinessElement{{Key: "brief", Title: "Brief", State: employeeDomain.ElementPass, Reason: "запечатан", Source: "release_manifests"}},
                        Routes:   []employeeDomain.RouteAdmission{{Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, Modality: employeeDomain.ModalityVoice, AutonomyClasses: []string{"assisted"}, State: employeeDomain.RouteAdmitted}},
                }
                f.view.History = []employeeDomain.ReleaseAdmission{}
                f.view.CanManage = true
                f.view.ViewScope = employeeAPI.ReadinessScopeManage
        }
        return f.view, nil
}

func (f *fakeReadiness) GateInstancesView(context.Context, domain.Principal, id.UUID, id.UUID) (employeeAPI.GateInstancesView, error) {
        if f.gateErr != nil {
                return employeeAPI.GateInstancesView{}, f.gateErr
        }
        if f.gates.EmployeeID == "" {
                f.gates = employeeAPI.GateInstancesView{EmployeeID: testPrincipalID, Definitions: employeeDomain.GateDictionary(),
                        Instances: []employeeDomain.GateInstance{{TenantID: testTenantID, EmployeeID: testPrincipalID, InstanceID: testCorrelation,
                                DefinitionID: "AG-1", State: employeeDomain.GateStatePass,
                                ApprovedBy:   []employeeDomain.GateApproval{{ParticipantID: testPrincipalID, Role: employeeDomain.GateRoleTechnicalLead}},
                                EvidenceRefs: []string{"evidence://ag-1"}, EvidenceHash: strings.Repeat("a", 64),
                                ActorID: testPrincipalID, RecordedAt: fixedClock{}.Now()}},
                        CanManage: true, ViewScope: employeeAPI.ReadinessScopeManage}
        }
        return f.gates, nil
}

func (f *fakeReadiness) RecordGateTransition(context.Context, employeeAPI.GateTransitionCommand) (employeeDomain.GateInstance, error) {
        f.gateCalls++
        if f.gateErr != nil {
                return employeeDomain.GateInstance{}, f.gateErr
        }
        return employeeDomain.GateInstance{TenantID: testTenantID, EmployeeID: testPrincipalID, InstanceID: testCorrelation,
                DefinitionID: "AG-1", State: employeeDomain.GateStatePass, ApprovedBy: []employeeDomain.GateApproval{}, EvidenceRefs: []string{},
                ActorID: testPrincipalID, RecordedAt: fixedClock{}.Now()}, nil
}

func (f *fakeReadiness) PublishAdmission(context.Context, employeeAPI.PublishAdmissionCommand) (employeeDomain.ReleaseAdmission, error) {
        f.publishCalls++
        if f.publishErr != nil {
                return employeeDomain.ReleaseAdmission{}, f.publishErr
        }
        return employeeDomain.ReleaseAdmission{TenantID: testTenantID, EmployeeID: testPrincipalID, DecisionNo: 1, Stage: employeeDomain.StageSynthetic,
                Manifest:       employeeDomain.ManifestLink{ManifestID: testCorrelation, ConfigVersion: 1, Digest: strings.Repeat("b", 64)},
                ProfileVersion: 1, ProfileDigest: strings.Repeat("c", 64),
                Routes:  []employeeDomain.RouteAdmission{{Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, Modality: employeeDomain.ModalityVoice, State: employeeDomain.RouteAdmitted}},
                ActorID: testPrincipalID, ApproverRoles: []string{"owner"}, StepUpVerified: true,
                DecidedAt: fixedClock{}.Now(), Digest: strings.Repeat("d", 64)}, nil
}

func (f *fakeReadiness) AdmissionHistoryView(context.Context, domain.Principal, id.UUID, id.UUID) (employeeAPI.AdmissionHistoryView, error) {
        if f.history.Admissions == nil {
                f.history = employeeAPI.AdmissionHistoryView{EmployeeID: testPrincipalID, Admissions: []employeeDomain.ReleaseAdmission{}}
        }
        return f.history, nil
}

// PilotAdmission serves the spec 4-1 machine/page view for the wire tests:
// the honest minimal derivation of an empty world (the pilot fail-closed
// start — the pack missing, the adapters not implemented, the gates open).
func (f *fakeReadiness) PilotAdmission(_ context.Context, _ domain.Principal, tenant id.UUID) (employeeAPI.PilotAdmissionView, error) {
        if f.pilotErr != nil {
                return employeeAPI.PilotAdmissionView{}, f.pilotErr
        }
        if !f.pilotSet {
                f.pilot = employeeAPI.PilotAdmissionView{
                        TenantID: tenant, CanManage: true, ViewScope: employeeAPI.ReadinessScopeManage,
                        Report: employeeDomain.DerivePilotAdmission(employeeDomain.PilotFacts{
                                PackID: "", PackVersion: "", Capabilities: map[string]employeeDomain.CapabilityFact{}, Instances: []employeeDomain.GateInstance{},
                        }),
                }
        }
        return f.pilot, nil
}

func (f *fakeReadiness) MontagioAdmission(_ context.Context, _ domain.Principal, tenant id.UUID) (employeeAPI.MontagioAdmissionView, error) {
        if f.montagioErr != nil {
                return employeeAPI.MontagioAdmissionView{}, f.montagioErr
        }
        if !f.montagioSet {
                f.montagio = employeeAPI.MontagioAdmissionView{
                        TenantID: tenant, CanManage: true, ViewScope: employeeAPI.ReadinessScopeManage,
                        Report: employeeDomain.DeriveMontagioAdmission(employeeDomain.MontagioFacts{
                                PackID: "", PackVersion: "", Capabilities: map[string]employeeDomain.CapabilityFact{}, Instances: []employeeDomain.GateInstance{},
                        }),
                }
        }
        return f.montagio, nil
}

func newReadinessTestServer(t *testing.T, fake *fakeReadiness) *Server {
        t.Helper()
        identity := fakeIdentity{create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
                return api.CreateTenantResult{}, nil
        }, workspace: func(id.UUID) (domain.Workspace, error) {
                return domain.Workspace{}, nil
        }}
        server, err := New(identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
        if err != nil {
                t.Fatal(err)
        }
        server.BindReadiness(fake)
        return server
}

func readinessJSONRequest(method, path, body string) *http.Request {
        request := httptest.NewRequest(method, path, strings.NewReader(body))
        request.Header.Set("Content-Type", "application/json")
        request.Header.Set("X-Correlation-ID", testCorrelation.String())
        return request
}

func readinessPath(suffix string) string {
        return "/v1/tenants/" + testTenantID.String() + "/employees/" + testPrincipalID.String() + "/test-release" + suffix
}

func decodeBody(t *testing.T, response *httptest.ResponseRecorder, out any) {
        t.Helper()
        if err := json.Unmarshal(response.Body.Bytes(), out); err != nil {
                t.Fatalf("body must be valid JSON: %v — %s", err, response.Body.String())
        }
}

// The readiness view wire shape follows the OpenAPI ReadinessView schema:
// every documented key present, the report nested with its own contract
// keys, the snake_case spelling exact.
func TestReadinessViewWireShapeMatchesContract(t *testing.T) {
        server := newReadinessTestServer(t, &fakeReadiness{})
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, readinessPath(""), ""))
        if response.Code != http.StatusOK {
                t.Fatalf("status = %d", response.Code)
        }
        var body map[string]any
        decodeBody(t, response, &body)
        for _, key := range []string{"employee_id", "stage", "readiness", "applicability", "admission_history", "can_manage", "view_scope"} {
                if _, ok := body[key]; !ok {
                        t.Errorf("the view must carry the contract key %q, got %v", key, body)
                }
        }
        report, ok := body["readiness"].(map[string]any)
        if !ok {
                t.Fatalf("readiness must be a mapping, got %T", body["readiness"])
        }
        for _, key := range []string{"stage", "status", "status_text", "elements", "routes"} {
                if _, ok := report[key]; !ok {
                        t.Errorf("the report must carry the contract key %q, got %v", key, report)
                }
        }
        elements, _ := report["elements"].([]any)
        if len(elements) != 1 {
                t.Fatalf("the elements list must travel untouched, got %v", elements)
        }
        row, _ := elements[0].(map[string]any)
        for _, key := range []string{"key", "title", "state", "reason", "source"} {
                if _, ok := row[key]; !ok {
                        t.Errorf("the element row must carry the contract key %q, got %v", key, row)
                }
        }
        routes, _ := report["routes"].([]any)
        route, _ := routes[0].(map[string]any)
        for _, key := range []string{"route", "modality", "autonomy_classes", "state"} {
                if _, ok := route[key]; !ok {
                        t.Errorf("the route row must carry the contract key %q, got %v", key, route)
                }
        }
}

// The gate registry view wire shape: the fixed dictionary rows with every
// documented key and the lifecycle instance rows with theirs.
func TestGateInstancesViewWireShapeMatchesContract(t *testing.T) {
        server := newReadinessTestServer(t, &fakeReadiness{})
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, readinessPath("/gate-instances"), ""))
        if response.Code != http.StatusOK {
                t.Fatalf("status = %d", response.Code)
        }
        var body struct {
                EmployeeID  id.UUID          `json:"employee_id"`
                Definitions []map[string]any `json:"definitions"`
                Instances   []map[string]any `json:"instances"`
                CanManage   bool             `json:"can_manage"`
                ViewScope   string           `json:"view_scope"`
        }
        decodeBody(t, response, &body)
        // Spec 4-1 extends the dictionary: 8 platform + 6 Oleg pilot definitions;
        // spec 5-1 extends it again: + 6 Montagio pilot definitions.
        if len(body.Definitions) != 20 {
                t.Fatalf("the dictionary must carry 8 platform + 6 Oleg (spec 4-1) + 6 Montagio (spec 5-1) definitions, got %d", len(body.Definitions))
        }
        definition := body.Definitions[0]
        for _, key := range []string{"definition_id", "criticality", "required_approver_roles", "evidence_requirement", "safe_state", "allow_role_coincidence", "route_scoped"} {
                if _, ok := definition[key]; !ok {
                        t.Errorf("the definition row must carry the contract key %q, got %v", key, definition)
                }
        }
        if len(body.Instances) != 1 {
                t.Fatalf("the instance list must travel untouched, got %v", body.Instances)
        }
        instance := body.Instances[0]
        for _, key := range []string{"tenant_id", "employee_id", "instance_id", "definition_id", "state", "approved_by", "evidence_refs", "actor_id", "recorded_at"} {
                if _, ok := instance[key]; !ok {
                        t.Errorf("the instance row must carry the contract key %q, got %v", key, instance)
                }
        }
}

// The admission decision wire shape: every documented key of
// ReleaseAdmission present in the response.
func TestPublicationWireShapeMatchesContract(t *testing.T) {
        server := newReadinessTestServer(t, &fakeReadiness{})
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodPost, readinessPath("/admissions"), `{"stage":"synthetic"}`))
        if response.Code != http.StatusCreated {
                t.Fatalf("status = %d", response.Code)
        }
        var body map[string]any
        decodeBody(t, response, &body)
        for _, key := range []string{"tenant_id", "employee_id", "decision_no", "stage", "manifest", "profile_version", "profile_digest", "routes", "actor_id", "approver_roles", "step_up_verified", "decided_at", "digest"} {
                if _, ok := body[key]; !ok {
                        t.Errorf("the decision must carry the contract key %q, got %v", key, body)
                }
        }
        manifest, _ := body["manifest"].(map[string]any)
        for _, key := range []string{"manifest_id", "config_version", "digest"} {
                if _, ok := manifest[key]; !ok {
                        t.Errorf("the manifest link must carry the contract key %q, got %v", key, manifest)
                }
        }
}

// The admission history wire shape: the documented key set of
// AdmissionHistoryView with the acting decision derived.
func TestAdmissionHistoryWireShapeMatchesContract(t *testing.T) {
        server := newReadinessTestServer(t, &fakeReadiness{})
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodGet, readinessPath("/admissions"), ""))
        if response.Code != http.StatusOK {
                t.Fatalf("status = %d", response.Code)
        }
        var body struct {
                EmployeeID id.UUID          `json:"employee_id"`
                Active     *map[string]any  `json:"active_release"`
                Admissions []map[string]any `json:"admissions"`
        }
        decodeBody(t, response, &body)
        if body.Admissions == nil || len(body.Admissions) != 0 {
                t.Fatalf("the empty history must serialize as an empty list, got %v", body.Admissions)
        }
}

// The commands reject unknown fields (the strict decode boundary).
func TestReadinessCommandsRejectUnknownFields(t *testing.T) {
        server := newReadinessTestServer(t, &fakeReadiness{})
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodPost, readinessPath("/admissions"), `{"stage":"synthetic","force":true}`))
        if response.Code != http.StatusBadRequest {
                t.Fatalf("an unknown field must be a 400 refusal, got %d", response.Code)
        }
        response = httptest.NewRecorder()
        server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodPost, readinessPath("/gate-instances"), `{"definition_id":"AG-1","state":"open","surprise":1}`))
        if response.Code != http.StatusBadRequest {
                t.Fatalf("an unknown gate field must be a 400 refusal, got %d", response.Code)
        }
}

// A non-JSON content type is refused with 415 exactly as the contract
// documents (the m3 doc/behavior mismatch).
func TestReadinessCommandsRefuseNonJSONContentType(t *testing.T) {
        server := newReadinessTestServer(t, &fakeReadiness{})
        request := httptest.NewRequest(http.MethodPost, readinessPath("/admissions"), strings.NewReader(`stage=synthetic`))
        request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
        request.Header.Set("X-Correlation-ID", testCorrelation.String())
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, request)
        if response.Code != http.StatusUnsupportedMediaType {
                t.Fatalf("a form body against the JSON API must be a 415 refusal, got %d", response.Code)
        }
}

// The honest refusals keep their codes: the publication conflict carries
// the current decision, the step-up refusal stays 403 with its code.
func TestReadinessHonestRefusalWire(t *testing.T) {
        current := employeeDomain.ReleaseAdmission{TenantID: testTenantID, EmployeeID: testPrincipalID, DecisionNo: 7, Stage: employeeDomain.StageSynthetic,
                Manifest:       employeeDomain.ManifestLink{ManifestID: testCorrelation, ConfigVersion: 1, Digest: strings.Repeat("b", 64)},
                ProfileVersion: 1, ProfileDigest: strings.Repeat("c", 64), Digest: strings.Repeat("d", 64)}
        fake := &fakeReadiness{publishErr: employeeDomain.AdmissionConflictError{Stage: employeeDomain.StageSynthetic, Current: &current}}
        server := newReadinessTestServer(t, fake)
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodPost, readinessPath("/admissions"), `{"stage":"synthetic"}`))
        if response.Code != http.StatusConflict {
                t.Fatalf("a publication conflict must be 409, got %d", response.Code)
        }
        var problem map[string]any
        decodeBody(t, response, &problem)
        if problem["code"] != "admission_conflict" {
                t.Errorf("the conflict code must be honest, got %v", problem["code"])
        }
        fields, _ := problem["field_errors"].(map[string]any)
        if fields["decision_no"] != "7" {
                t.Errorf("the conflict must carry the current decision number, got %v", fields["decision_no"])
        }

        stepUp := &fakeReadiness{publishErr: employeeDomain.ErrReadinessStepUpRequired}
        server = newReadinessTestServer(t, stepUp)
        response = httptest.NewRecorder()
        server.Handler().ServeHTTP(response, readinessJSONRequest(http.MethodPost, readinessPath("/admissions"), `{"stage":"synthetic"}`))
        if response.Code != http.StatusForbidden {
                t.Fatalf("a step-up refusal must be 403, got %d", response.Code)
        }
        decodeBody(t, response, &problem)
        if problem["code"] != "step_up_required" {
                t.Errorf("the step-up code must be honest, got %v", problem["code"])
        }
}
