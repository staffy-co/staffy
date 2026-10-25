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

// fakeProfile implements the profile service surface for the transport
// tests: it answers with honest minimal views and records the commands. The
// R2 regressions assert what the HTTP boundary claims (the wire shape
// follows the OpenAPI contract).
type fakeProfile struct {
	view         employeeAPI.ProfileView
	viewErr      error
	approveErr   error
	thresholdErr error
	approveCalls int
}

func (f *fakeProfile) ProfileView(context.Context, domain.Principal, id.UUID, id.UUID) (employeeAPI.ProfileView, error) {
	if f.viewErr != nil {
		return employeeAPI.ProfileView{}, f.viewErr
	}
	if f.view.EmployeeID == "" {
		f.view.EmployeeID = testPrincipalID
		f.view.Readiness = []employeeDomain.ProfileRuleError{}
		f.view.RouteStates = []employeeDomain.RouteState{}
		f.view.Matrix = []employeeDomain.MatrixCellState{}
		f.view.Thresholds = []employeeDomain.ThresholdState{}
		f.view.CanManage = true
		f.view.ViewScope = employeeAPI.ProfileScopeManage
	}
	return f.view, nil
}

func (f *fakeProfile) UpdateDraft(context.Context, employeeAPI.ProfileDraftCommand) (employeeDomain.ProfileDraft, error) {
	return employeeDomain.ProfileDraft{}, nil
}

func (f *fakeProfile) ApproveProfile(context.Context, employeeAPI.ProfileApproveCommand) (employeeDomain.ProfileVersion, error) {
	f.approveCalls++
	if f.approveErr != nil {
		return employeeDomain.ProfileVersion{}, f.approveErr
	}
	return employeeDomain.ProfileVersion{Version: 1, Digest: strings.Repeat("a", 64)}, nil
}

func (f *fakeProfile) DecideThreshold(context.Context, employeeAPI.ThresholdCommand) (employeeDomain.GateThresholdVersion, error) {
	if f.thresholdErr != nil {
		return employeeDomain.GateThresholdVersion{}, f.thresholdErr
	}
	return employeeDomain.GateThresholdVersion{}, nil
}

func (f *fakeProfile) ThresholdHistory(context.Context, domain.Principal, id.UUID, id.UUID, string) (employeeAPI.ThresholdHistoryView, error) {
	return employeeAPI.ThresholdHistoryView{Decisions: []employeeAPI.ThresholdDecisionView{}}, nil
}

func newProfileTestServer(t *testing.T, fake *fakeProfile) *Server {
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
	server.BindProfile(fake)
	return server
}

func profileJSONRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	return request
}

// R1/M1: the readiness wire shape must follow the OpenAPI contract —
// snake_case keys and the kind enum, not the exported Go field names.
func TestProfileViewReadinessWireShapeMatchesContract(t *testing.T) {
	fake := &fakeProfile{}
	// EmployeeID set: the default view fill is skipped, the preset readiness
	// travels to the response untouched.
	fake.view = employeeAPI.ProfileView{EmployeeID: testPrincipalID, CanManage: true, ViewScope: employeeAPI.ProfileScopeManage}
	fake.view.Readiness = []employeeDomain.ProfileRuleError{{
		Section: "work_scope", Field: "routes", Reason: "нет маршрутов", Kind: employeeDomain.ProfileProblemMissing,
		Problems: []employeeDomain.ProfileRuleError{{Section: "work_scope", Field: "routes", Reason: "нет маршрутов", Kind: employeeDomain.ProfileProblemMissing}},
	}}
	server := newProfileTestServer(t, fake)
	response := httptest.NewRecorder()
	path := "/v1/tenants/" + testTenantID.String() + "/employees/" + testPrincipalID.String() + "/operating-profile"
	server.Handler().ServeHTTP(response, profileJSONRequest(http.MethodGet, path, ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body struct {
		Readiness []map[string]any `json:"readiness"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Readiness) != 1 {
		t.Fatalf("readiness must carry the one honest problem, got %d", len(body.Readiness))
	}
	row := body.Readiness[0]
	for _, key := range []string{"section", "field", "reason", "kind", "problems"} {
		if _, ok := row[key]; !ok {
			t.Errorf("readiness row must carry the contract key %q, got %v", key, row)
		}
	}
	if row["kind"] != employeeDomain.ProfileProblemMissing {
		t.Errorf("kind must serialize the enum value, got %v", row["kind"])
	}
}

// R1/M3: an approval without evidence references must be refused at the
// HTTP boundary exactly as the contract's minItems promises.
func TestProfileApprovalWithoutEvidenceIsRefused(t *testing.T) {
	fake := &fakeProfile{approveErr: employeeDomain.ProfileRuleError{Section: "approval", Field: "evidence_refs", Reason: "Утверждение требует хотя бы одной evidence-ссылки."}}
	server := newProfileTestServer(t, fake)
	response := httptest.NewRecorder()
	path := "/v1/tenants/" + testTenantID.String() + "/employees/" + testPrincipalID.String() + "/operating-profile/approval"
	server.Handler().ServeHTTP(response, profileJSONRequest(http.MethodPost, path, `{"description":"x","evidence_refs":[],"expected_revision":1}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("an empty evidence list must be a 400 refusal, got %d", response.Code)
	}
	var problem struct {
		Code        string            `json:"code"`
		FieldErrors map[string]string `json:"field_errors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.FieldErrors["field"] != "evidence_refs" {
		t.Errorf("the refusal must lead to the concrete field, got %v", problem.FieldErrors)
	}
}

// R1/m3a: the approval conflict carries the current draft content (the
// canonical JSON) for an honest comparison — the spec's Implementation Note.
func TestProfileApprovalConflictCarriesCurrentContent(t *testing.T) {
	current := employeeDomain.ProfileContent{
		Modes:           []string{"sales"},
		AutonomyClasses: []string{"assisted"},
	}
	fake := &fakeProfile{approveErr: employeeDomain.ProfileConflictError{Expected: 3, Current: 4, CurrentContent: &current}}
	server := newProfileTestServer(t, fake)
	response := httptest.NewRecorder()
	path := "/v1/tenants/" + testTenantID.String() + "/employees/" + testPrincipalID.String() + "/operating-profile/approval"
	server.Handler().ServeHTTP(response, profileJSONRequest(http.MethodPost, path, `{"description":"x","evidence_refs":["m://1"],"expected_revision":3}`))
	if response.Code != http.StatusConflict {
		t.Fatalf("the optimistic-concurrency loss must be a 409, got %d", response.Code)
	}
	var problem struct {
		FieldErrors map[string]string `json:"field_errors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.FieldErrors["expected_revision"] != "3" || problem.FieldErrors["current_revision"] != "4" {
		t.Errorf("the conflict must carry the revisions, got %v", problem.FieldErrors)
	}
	if !strings.Contains(problem.FieldErrors["current_content"], `"conversation_modes"`) {
		t.Errorf("the conflict must carry the canonical JSON of the current draft sections, got %q", problem.FieldErrors["current_content"])
	}
}

// The step-up refusal stays honest at the HTTP boundary: 403 with the exact
// code, never a silent bypass.
func TestProfileApprovalStepUpRefusalIsHonest(t *testing.T) {
	fake := &fakeProfile{approveErr: employeeDomain.ErrProfileStepUpRequired}
	server := newProfileTestServer(t, fake)
	response := httptest.NewRecorder()
	path := "/v1/tenants/" + testTenantID.String() + "/employees/" + testPrincipalID.String() + "/operating-profile/approval"
	server.Handler().ServeHTTP(response, profileJSONRequest(http.MethodPost, path, `{"description":"x","evidence_refs":["m://1"],"expected_revision":1}`))
	if response.Code != http.StatusForbidden {
		t.Fatalf("the step-up refusal must stay a 403, got %d", response.Code)
	}
	if body := response.Body.String(); !strings.Contains(body, "step_up_required") {
		t.Fatalf("the problem code must name step_up_required, got %s", body)
	}
}
