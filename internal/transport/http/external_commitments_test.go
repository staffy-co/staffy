package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/api"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 3.3 transport tests: the fake serves the honest refusals of
// the gates and the scripted outcomes (the gate LADDER is the app's unit
// truth — these tests pin the MAPPING of every refusal onto its code and
// the wire/page shapes: the strict decode, the verbatim command capture,
// the state words with the fixed domain texts, the honest empty view, the
// PRG answers with the focus notices and the form origin/checkbox
// disciplines).
// ---------------------------------------------------------------------------

var (
	testExtCaseID   = id.MustParse("01994f64-6c00-7000-8000-00000000e001")
	testExtWorkID   = id.MustParse("01994f64-6c00-7000-8000-00000000e002")
	testExtEnvelope = id.MustParse("01994f64-6c00-7000-8000-00000000e003")
)

// fakeExternalCases embeds the cases fake (the composite Service stays
// untouched) and serves the external commitment surface with the
// scripted answers and the captured commands.
type fakeExternalCases struct {
	fakeCases

	view       casesAPI.ExternalCommitmentView
	viewErr    error
	createErr  error
	approveErr error
	enqueueErr error
	relayErr   error
	drainErr   error
	drain      casesAPI.ExternalDrainResult

	createCalls  int
	approveCalls int
	enqueueCalls int
	relayCalls   int
	drainCalls   int
	lastCreate   *casesAPI.CreateExternalCommitmentCommand
	lastApprove  *casesAPI.ApproveExternalCommitmentCommand
	lastEnqueue  *casesAPI.EnqueueExternalCommitmentCommand
	lastRelay    *casesAPI.RelayExternalReceiptCommand
}

// The compile-time proof the fake satisfies the narrow transport contract.
var _ casesAPI.ExternalCommitmentService = (*fakeExternalCases)(nil)

func (f *fakeExternalCases) CreateExternalCommitment(_ context.Context, c casesAPI.CreateExternalCommitmentCommand) (casesAPI.ExternalCommitmentOutcome, error) {
	f.createCalls++
	commandCopy := c
	f.lastCreate = &commandCopy
	if f.createErr != nil {
		return casesAPI.ExternalCommitmentOutcome{}, f.createErr
	}
	return casesAPI.ExternalCommitmentOutcome{Command: "create_external_commitment", ExtCommitmentID: c.ExtCommitmentID, VersionNo: 2, ContentToken: "64hex-token-of-the-exact-material-bytes"}, nil
}

func (f *fakeExternalCases) ApproveExternalCommitment(_ context.Context, c casesAPI.ApproveExternalCommitmentCommand) (casesAPI.ExternalApprovalOutcome, error) {
	f.approveCalls++
	commandCopy := c
	f.lastApprove = &commandCopy
	if f.approveErr != nil {
		return casesAPI.ExternalApprovalOutcome{}, f.approveErr
	}
	return casesAPI.ExternalApprovalOutcome{ExtCommitmentID: c.ExtCommitmentID, VersionNo: c.VersionNo, ApprovalRecorded: true}, nil
}

func (f *fakeExternalCases) EnqueueExternalCommitment(_ context.Context, c casesAPI.EnqueueExternalCommitmentCommand) (casesAPI.ExternalEnqueueOutcome, error) {
	f.enqueueCalls++
	commandCopy := c
	f.lastEnqueue = &commandCopy
	if f.enqueueErr != nil {
		return casesAPI.ExternalEnqueueOutcome{}, f.enqueueErr
	}
	return casesAPI.ExternalEnqueueOutcome{ExtCommitmentID: c.ExtCommitmentID, VersionNo: c.VersionNo, Queued: true}, nil
}

func (f *fakeExternalCases) RelayExternalReceipt(_ context.Context, c casesAPI.RelayExternalReceiptCommand) (casesAPI.ExternalRelayOutcome, error) {
	f.relayCalls++
	commandCopy := c
	f.lastRelay = &commandCopy
	if f.relayErr != nil {
		return casesAPI.ExternalRelayOutcome{}, f.relayErr
	}
	return casesAPI.ExternalRelayOutcome{ExtCommitmentID: c.ExtCommitmentID, VersionNo: c.VersionNo, Kind: casesDomain.RelayObservationDispatchKind(c.Observation)}, nil
}

func (f *fakeExternalCases) DrainExternalCommitments(context.Context, domain.Principal, id.UUID) (casesAPI.ExternalDrainResult, error) {
	f.drainCalls++
	if f.drainErr != nil {
		return casesAPI.ExternalDrainResult{}, f.drainErr
	}
	return f.drain, nil
}

func (f *fakeExternalCases) ReadExternalCommitmentView(context.Context, domain.Principal, id.UUID, id.UUID) (casesAPI.ExternalCommitmentView, error) {
	if f.viewErr != nil {
		return casesAPI.ExternalCommitmentView{}, f.viewErr
	}
	return f.view, nil
}

// newExternalTestServer binds the fake (the composite + the external
// surface) through the production BindCases — the narrow interface is
// type-asserted exactly the way the real wiring does it.
func newExternalTestServer(t *testing.T, fake *fakeExternalCases) *Server {
	t.Helper()
	identity := api.Service(fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) {
			return domain.Workspace{}, nil
		},
	})
	server, err := New(identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, StepUpVerified: true}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindCases(fake)
	return server
}

// externalFixtureView builds the fixture view of the page/wire tests: one
// Work Item section with two version cards (a queued current version
// approved by the fixture principal, and a persisted superseded v1 with
// one attempt event), the domain dictionaries and the honest texts.
func externalFixtureView() casesAPI.ExternalCommitmentView {
	queued := casesAPI.ExternalCommitmentVersionCard{
		Version: casesAPI.FromExternalCommitment(casesDomain.ExternalCommitment{
			TenantID: testTenantID, CaseID: testExtCaseID, WorkItemID: testExtWorkID,
			ExtCommitmentID: testExtEnvelope, VersionNo: 2,
			Purpose: casesDomain.ExternalPurposePromise, RecipientKind: casesDomain.ExternalRecipientClient,
			Channel: "phone", ContentText: "Мы вернёмся с ответом до пятницы.",
			ValidUntil: time.Unix(2, 0).UTC(), ExpectedNextStep: "оператор перезвонит",
			AckRequirement: casesDomain.ExternalAckNotRequired,
			OwnerKind:      casesDomain.OwnerKindHuman, OwnerRef: testPrincipalID, OwnerEpoch: 1,
			ContentToken: "64hex-token-of-the-exact-material-bytes", ActorID: testPrincipalID,
			CreatedAt: time.Unix(1, 0).UTC(),
		}),
		Enqueued:   true,
		Delivery:   casesAPI.FromExternalDeliveryState(casesDomain.DeriveExternalDeliveryState(true, nil)),
		Readiness:  casesDomain.ExternalReadinessReady,
		Current:    true,
		Approved:   true,
		ApproverID: testPrincipalID,
		Events:     []casesAPI.ExternalDispatchEventWire{},
	}
	superseded := queued
	superseded.Version = queued.Version
	superseded.Version.VersionNo = 1
	superseded.Enqueued = false
	superseded.Current = false
	superseded.Approved = false
	superseded.ApproverID = id.UUID("")
	superseded.Readiness = casesDomain.ExternalReadinessSuperseded
	superseded.ReadinessText = casesDomain.ExternalReadinessRefusalTexts[casesDomain.ExternalReadinessSuperseded]
	superseded.Delivery = casesAPI.FromExternalDeliveryState(casesDomain.DeriveExternalDeliveryState(true, []casesDomain.ExternalDispatchEvent{
		{EventNo: 1, Kind: casesDomain.ExternalDispatchAttemptSubmitted, AttemptNo: 1,
			ExternalCorrelationID: "syn-abc123", IdempotentGuarantee: true,
			Details: "исход: accepted; корреляция: syn-abc123", ActorID: testPrincipalID, CreatedAt: time.Unix(1, 0).UTC()},
	}))
	superseded.Events = []casesAPI.ExternalDispatchEventWire{
		casesAPI.FromExternalDispatchEvent(casesDomain.ExternalDispatchEvent{
			EventNo: 1, Kind: casesDomain.ExternalDispatchAttemptSubmitted, AttemptNo: 1,
			ExternalCorrelationID: "syn-abc123", IdempotentGuarantee: true,
			Details: "исход: accepted; корреляция: syn-abc123", ActorID: testPrincipalID, CreatedAt: time.Unix(1, 0).UTC(),
		}),
	}
	return casesAPI.ExternalCommitmentView{
		Items: []casesAPI.ExternalCommitmentWorkItemView{{
			WorkItemID: testExtWorkID,
			Versions:   []casesAPI.ExternalCommitmentVersionCard{queued, superseded},
		}},
		PurposeWords:   append([]string(nil), casesDomain.ExternalPurposeWords...),
		RecipientWords: append([]string(nil), casesDomain.ExternalRecipientKindWords...),
		AckWords:       append([]string(nil), casesDomain.ExternalAckRequirementWords...),
		RelayWords:     append([]string(nil), casesDomain.RelayObservationWords...),
		ChannelWords:   append([]string(nil), casesDomain.ChannelOrder...),
		StateWords:     append([]string(nil), casesDomain.ExternalDeliveryStateWords...),
		PanelText:      "Панель: конверт с неизменяемыми версиями.",
		EmptyText:      "Пустота: конвертов нет.",
	}
}

// externalJSONRequest builds a machine JSON request of the surface.
func externalJSONRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

// externalFormRequest builds a page form request with the browser origin
// of the fixture host.
func externalFormRequest(method, path, form string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(form))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+request.Host)
	return request
}

// externalProblemOf decodes the problem answer of the machine surface.
func externalProblemOf(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var problem map[string]any
	if err := json.Unmarshal(body, &problem); err != nil {
		t.Fatalf("the answer must be the problem JSON, got %s", string(body))
	}
	return problem
}

func TestExternalReadAPIServesTheViewMaterial(t *testing.T) {
	fake := &fakeExternalCases{view: externalFixtureView()}
	server := newExternalTestServer(t, fake)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/v1/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("the view read must answer 200, got %d", response.Code)
	}
	var answer struct {
		ExternalCommitments casesAPI.ExternalCommitmentView `json:"external_commitments"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	view := answer.ExternalCommitments
	if len(view.Items) != 1 || len(view.Items[0].Versions) != 2 {
		t.Fatalf("the fixture view must carry one section with two cards, got %+v", view.Items)
	}
	queued := view.Items[0].Versions[0]
	if !queued.Enqueued || queued.Delivery.State != casesDomain.ExternalDeliveryQueued {
		t.Errorf("the queued card must derive the queued state word, got %+v", queued.Delivery)
	}
	if queued.Delivery.StateText != casesDomain.ExternalDeliveryStateTexts[casesDomain.ExternalDeliveryQueued] {
		t.Errorf("the queued state text must be the FIXED domain render text, got %q", queued.Delivery.StateText)
	}
	superseded := view.Items[0].Versions[1]
	if superseded.Delivery.State != casesDomain.ExternalDeliverySubmitted {
		t.Errorf("the attempted card must derive submitted, got %q", superseded.Delivery.State)
	}
	if superseded.Delivery.StateText != casesDomain.ExternalDeliveryStateTexts[casesDomain.ExternalDeliverySubmitted] {
		t.Errorf("submitted never reads as a delivery: the fixed text must stay, got %q", superseded.Delivery.StateText)
	}
	if superseded.Readiness != casesDomain.ExternalReadinessSuperseded {
		t.Errorf("the superseded card must derive the superseded readiness, got %q", superseded.Readiness)
	}
	if len(superseded.Events) != 1 || superseded.Events[0].Kind != casesDomain.ExternalDispatchAttemptSubmitted {
		t.Errorf("the append-only history must travel with the card, got %+v", superseded.Events)
	}
	if len(view.PurposeWords) != 4 || len(view.RelayWords) != 3 || len(view.ChannelWords) != 9 || len(view.StateWords) != 7 {
		t.Errorf("the dictionaries must travel with the view, got purpose %d relay %d channel %d states %d",
			len(view.PurposeWords), len(view.RelayWords), len(view.ChannelWords), len(view.StateWords))
	}
	if view.PanelText == "" || view.EmptyText == "" {
		t.Errorf("the honest texts must travel with the view")
	}
}

func TestExternalReadAPIHonestEmptyView(t *testing.T) {
	fake := &fakeExternalCases{view: casesAPI.ExternalCommitmentView{Items: []casesAPI.ExternalCommitmentWorkItemView{}, EmptyText: "Пустота: конвертов нет."}}
	server := newExternalTestServer(t, fake)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/v1/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("the empty view is the honest 200, got %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"items":[]`) {
		t.Errorf("the empty items list is the honest empty array, never a null: %s", body)
	}
	if !strings.Contains(body, "Пустота: конвертов нет.") {
		t.Errorf("the empty reason must travel with the view: %s", body)
	}
}

func TestExternalSurfacesRequireIdentity(t *testing.T) {
	server, err := New(fakeIdentity{create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
		return api.CreateTenantResult{}, nil
	}, workspace: func(id.UUID) (domain.Workspace, error) { return domain.Workspace{}, nil }},
		fixedResolver{}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindCases(&fakeExternalCases{})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/v1/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated read is the honest 401, got %d", response.Code)
	}
}

func TestExternalNonMemberIsPlain404BeforeMaterial(t *testing.T) {
	fake := &fakeExternalCases{viewErr: casesDomain.ErrNotFound}
	server := newExternalTestServer(t, fake)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/v1/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("a non-member is the plain 404 before any material, got %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "e003") {
		t.Errorf("the 404 must not disclose the envelope material")
	}
}

func TestExternalScopeRefusalIs403WithTheAppWords(t *testing.T) {
	scopeWords := "Команды внешних обязательств требуют полномочия review или configuration: участники только с audit не создают конверты."
	fake := &fakeExternalCases{viewErr: fmt.Errorf("%w: %s", domain.ErrForbidden, scopeWords)}
	server := newExternalTestServer(t, fake)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/v1/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("a member without the scope is the 403, got %d", response.Code)
	}
	problem := externalProblemOf(t, response)
	if message, _ := problem["message"].(string); message != scopeWords {
		t.Errorf("the 403 words must be the APP words without the sentinel, got %q", message)
	}
}

func TestExternalCreateAPIStrictDecode(t *testing.T) {
	fake := &fakeExternalCases{}
	server := newExternalTestServer(t, fake)
	base := "/v1/tenants/" + testTenantID.String() + "/cases/" + testExtCaseID.String() + "/work-items/" + testExtWorkID.String() + "/external-commitments"
	cases := []struct {
		name    string
		request *http.Request
	}{
		{"no content type", httptest.NewRequest(http.MethodPost, base, strings.NewReader(`{}`))},
		{"unknown field", externalJSONRequest(http.MethodPost, base, `{"purpose":"promise","foreign":1}`)},
		{"two objects", externalJSONRequest(http.MethodPost, base, `{"purpose":"promise"}{"purpose":"offer"}`)},
		{"not an object", externalJSONRequest(http.MethodPost, base, `[]`)},
	}
	for _, testCase := range cases {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, testCase.request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s must be the honest 400, got %d", testCase.name, response.Code)
		}
		if fake.createCalls != 0 {
			t.Fatalf("%s must never reach the service", testCase.name)
		}
	}
}

func TestExternalCreateAPIRecordsTheRequestVerbatim(t *testing.T) {
	fake := &fakeExternalCases{}
	server := newExternalTestServer(t, fake)
	path := "/v1/tenants/" + testTenantID.String() + "/cases/" + testExtCaseID.String() + "/work-items/" + testExtWorkID.String() + "/external-commitments"
	body := `{"ext_commitment_id":"` + testExtEnvelope.String() + `","purpose":"offer","recipient_kind":"client","channel":"sms",` +
		`"content_text":"Предлагаем условия, показанные в версии 3.","valid_until":"2026-10-26T12:00:00Z",` +
		`"expected_next_step":"клиент подтверждает предложение","ack_requirement":"required",` +
		`"pin":{"commitment_id":"` + testExtEnvelope.String() + `","version_no":3,"content_token":"64hex-token-of-the-exact-material-bytes"}}`
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalJSONRequest(http.MethodPost, path, body))
	if response.Code != http.StatusOK {
		t.Fatalf("the honest create answers 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.createCalls != 1 || fake.lastCreate == nil {
		t.Fatal("the command must reach the service exactly once")
	}
	command := fake.lastCreate
	if command.TenantID != testTenantID || command.CaseID != testExtCaseID || command.WorkItemID != testExtWorkID {
		t.Errorf("the path anchors must travel verbatim, got %+v", command)
	}
	if command.ExtCommitmentID != testExtEnvelope || command.Purpose != casesDomain.ExternalPurposeOffer ||
		command.RecipientKind != casesDomain.ExternalRecipientClient || command.Channel != "sms" {
		t.Errorf("the dictionary words must travel verbatim, got %+v", command)
	}
	if command.ContentText != "Предлагаем условия, показанные в версии 3." ||
		command.ExpectedNextStep != "клиент подтверждает предложение" ||
		command.AckRequirement != casesDomain.ExternalAckRequired {
		t.Errorf("the material must travel verbatim, got %+v", command)
	}
	if !command.ValidUntil.Equal(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("the RFC3339 window must parse into the exact instant, got %v", command.ValidUntil)
	}
	if command.Pin.CommitmentID != testExtEnvelope || command.Pin.VersionNo != 3 || command.Pin.ContentToken != "64hex-token-of-the-exact-material-bytes" {
		t.Errorf("the pin triple must travel verbatim, got %+v", command.Pin)
	}
	var answer struct {
		Outcome casesAPI.ExternalCommitmentOutcome `json:"outcome"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Outcome.ExtCommitmentID != testExtEnvelope || answer.Outcome.VersionNo != 2 {
		t.Errorf("the outcome wrapper must carry the material, got %+v", answer.Outcome)
	}
}

func TestExternalCreateAPIRefusesTheShapeRefusals(t *testing.T) {
	fake := &fakeExternalCases{}
	server := newExternalTestServer(t, fake)
	base := "/v1/tenants/" + testTenantID.String() + "/cases/" + testExtCaseID.String() + "/work-items/" + testExtWorkID.String() + "/external-commitments"
	cases := []struct {
		name string
		body string
		code string
	}{
		{"malformed window", `{"purpose":"promise","valid_until":"26-10-2026"}`, "valid_until"},
		{"malformed ext id", `{"ext_commitment_id":"not-a-uuid"}`, "ext_commitment_id"},
	}
	for _, testCase := range cases {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, externalJSONRequest(http.MethodPost, base, testCase.body))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s must be the honest 400, got %d", testCase.name, response.Code)
		}
		problem := externalProblemOf(t, response)
		if problem["code"] != testCase.code {
			t.Errorf("%s must carry the rule code %q, got %v", testCase.name, testCase.code, problem["code"])
		}
		if fake.createCalls != 0 {
			t.Fatalf("%s must never reach the service", testCase.name)
		}
	}
}

func TestExternalApproveAPIPathPinAndStepUpMapping(t *testing.T) {
	fake := &fakeExternalCases{approveErr: domain.ErrStepUpRequired}
	server := newExternalTestServer(t, fake)
	path := "/v1/tenants/" + testTenantID.String() + "/cases/" + testExtCaseID.String() +
		"/work-items/" + testExtWorkID.String() + "/external-commitments/" + testExtEnvelope.String() + "/versions/2/approve"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("the step-up refusal is the honest 403, got %d", response.Code)
	}
	problem := externalProblemOf(t, response)
	if problem["code"] != "step_up_required" {
		t.Errorf("the step-up code must be explicit, got %v", problem["code"])
	}
	if fake.approveCalls != 1 || fake.lastApprove == nil {
		t.Fatal("the approve command must reach the service")
	}
	if fake.lastApprove.ExtCommitmentID != testExtEnvelope || fake.lastApprove.VersionNo != 2 ||
		fake.lastApprove.TenantID != testTenantID || fake.lastApprove.CaseID != testExtCaseID ||
		fake.lastApprove.WorkItemID != testExtWorkID {
		t.Errorf("the path pin must travel verbatim, got %+v", fake.lastApprove)
	}
}

func TestExternalEnqueueAPIWorkRuleIs400WithTheRuleWords(t *testing.T) {
	approvalText := casesDomain.ExternalApprovalRequiredText(2)
	fake := &fakeExternalCases{enqueueErr: casesDomain.WorkRuleError{Rule: "enqueue_external_commitment", Reason: approvalText}}
	server := newExternalTestServer(t, fake)
	path := "/v1/tenants/" + testTenantID.String() + "/cases/" + testExtCaseID.String() +
		"/work-items/" + testExtWorkID.String() + "/external-commitments/" + testExtEnvelope.String() + "/versions/2/enqueue"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("the approval-gate refusal is the honest 400 before the transaction, got %d", response.Code)
	}
	problem := externalProblemOf(t, response)
	if problem["code"] != "enqueue_external_commitment" {
		t.Errorf("the rule code must name the command, got %v", problem["code"])
	}
	if message, _ := problem["message"].(string); message != approvalText {
		t.Errorf("the refusal words must be the app words verbatim, got %q", message)
	}
}

func TestExternalRelayAPIRecordsTheObservation(t *testing.T) {
	fake := &fakeExternalCases{}
	server := newExternalTestServer(t, fake)
	path := "/v1/tenants/" + testTenantID.String() + "/cases/" + testExtCaseID.String() +
		"/work-items/" + testExtWorkID.String() + "/external-commitments/" + testExtEnvelope.String() + "/versions/2/dispatch-receipts"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalJSONRequest(http.MethodPost, path, `{"observation":"acknowledged"}`))
	if response.Code != http.StatusOK {
		t.Fatalf("the relay answers 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.relayCalls != 1 || fake.lastRelay == nil {
		t.Fatal("the relay command must reach the service")
	}
	if fake.lastRelay.Observation != casesDomain.RelayObservationAcknowledged ||
		fake.lastRelay.ExtCommitmentID != testExtEnvelope || fake.lastRelay.VersionNo != 2 {
		t.Errorf("the observation and the pin must travel verbatim, got %+v", fake.lastRelay)
	}
	var answer struct {
		Relay casesAPI.ExternalRelayOutcome `json:"relay"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Relay.Kind != casesDomain.ExternalDispatchReceiptAcknowledged {
		t.Errorf("the relay outcome must answer the receipt kind, got %q", answer.Relay.Kind)
	}
}

func TestExternalDrainAPIServesTheResultAndTheAG4Refusal(t *testing.T) {
	fake := &fakeExternalCases{drain: casesAPI.ExternalDrainResult{
		Considered: 3, Submitted: 1, Gassed: 1, UnknownParked: 1,
		Reports: []casesAPI.ExternalDrainReport{{ExtCommitmentID: testExtEnvelope, VersionNo: 2, Text: "Итог неопределён (timeout без идемпотентной гарантии): слепого повтора нет — передаётся в reconciliation (3-7)."}},
	}}
	server := newExternalTestServer(t, fake)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		"/v1/tenants/"+testTenantID.String()+"/cases/external-commitments/drain", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("the drain answers 200, got %d", response.Code)
	}
	var answer struct {
		Drain casesAPI.ExternalDrainResult `json:"drain"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Drain.Considered != 3 || answer.Drain.Submitted != 1 || len(answer.Drain.Reports) != 1 {
		t.Errorf("the counters and the honest reports must travel, got %+v", answer.Drain)
	}

	refused := &fakeExternalCases{drainErr: casesDomain.WorkRuleError{Rule: "drain_external_commitments",
		Reason: "Диспетчер внешних отправок не сконфигурирован: записи остаются в очереди (производственный контур не подключает провайдеров — AG-4); слова не уходят."}}
	server = newExternalTestServer(t, refused)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost,
		"/v1/tenants/"+testTenantID.String()+"/cases/external-commitments/drain", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("the AG-4 refusal is the honest 400 rule words, got %d", response.Code)
	}
	problem := externalProblemOf(t, response)
	if problem["code"] != "drain_external_commitments" {
		t.Errorf("the refusal code must name the drain command, got %v", problem["code"])
	}
}

func TestExternalPageRendersTheMaterialWordsAndFocus(t *testing.T) {
	fake := &fakeExternalCases{view: externalFixtureView()}
	server := newExternalTestServer(t, fake)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/app/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments?enqueued=2", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("the page must answer 200, got %d", response.Code)
	}
	body := response.Body.String()
	for _, must := range []string{
		"Внешние обязательства",
		"В очереди на отправку; слова ещё не покинули модуль.",
		"Отправлено в канал; доставки и принятия НЕТ.",
		"в очереди — outbox запись, слова не уходили",
		"не утверждена",
		"утверждена: " + testPrincipalID.String(),
		"заменена новой версией",
		"superseded",
		"Существует новее версия конверта — отправка этой версии невозможна.",
		"attempt_submitted",
		"syn-abc123",
		"идемпотентная гарантия записана",
		"Версия 2 поставлена в очередь (queued): слова сохранены до отправки — persist-before-emit, сетевого вызова при постановке нет.",
		`id="page-notice"`,
		"К основному содержанию",
		"Записать наблюдение",
	} {
		if !strings.Contains(body, must) {
			t.Errorf("the page must render %q", must)
		}
	}
	if !strings.Contains(body, "page-notice').focus()") && !strings.Contains(body, "page-notice').focus()") {
		if !strings.Contains(body, "focus()") {
			t.Errorf("the notice must carry the focus transfer")
		}
	}
}

func TestExternalPageEmptyStateIsTheHonestReason(t *testing.T) {
	fake := &fakeExternalCases{view: casesAPI.ExternalCommitmentView{
		Items:     []casesAPI.ExternalCommitmentWorkItemView{},
		EmptyText: "Внешних обязательств в этом кейсе нет. Конверт создаёт оператор с полномочием review или configuration.",
		PanelText: "Панель.",
	}}
	server := newExternalTestServer(t, fake)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/app/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("the empty page answers 200, got %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "Внешних обязательств в этом кейсе нет. Конверт создаёт оператор с полномочием review или configuration.") {
		t.Errorf("the empty state must speak the honest reason verbatim")
	}
	if strings.Contains(body, "action-card") {
		t.Errorf("the empty page must not render any card")
	}
}

func TestExternalCreateFormRedirectsPRGWithTheVersion(t *testing.T) {
	fake := &fakeExternalCases{}
	server := newExternalTestServer(t, fake)
	form := "work_item_id=" + testExtWorkID.String() + "&purpose=promise&recipient_kind=client&channel=sms" +
		"&content_text=" + url.QueryEscape("Мы вернёмся с ответом до пятницы.") +
		"&valid_until=2026-10-26T12%3A00%3A00Z&expected_next_step=" + url.QueryEscape("оператор перезвонит") +
		"&ack_requirement=not_required&pin_commitment_id=&pin_version_no=&pin_content_token="
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalFormRequest(http.MethodPost,
		"/app/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments", form))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("the create form answers PRG 303, got %d", response.Code)
	}
	location := response.Header().Get("Location")
	if !strings.Contains(location, "/external-commitments?created=2") {
		t.Errorf("the PRG target carries the created version, got %q", location)
	}
	if fake.createCalls != 1 || fake.lastCreate == nil {
		t.Fatal("the form command must reach the service")
	}
	if fake.lastCreate.WorkItemID != testExtWorkID || fake.lastCreate.Purpose != casesDomain.ExternalPurposePromise {
		t.Errorf("the form material must travel verbatim, got %+v", fake.lastCreate)
	}
	if fake.lastCreate.Pin.Present() {
		t.Errorf("the empty pin fields must answer the honest zero triple, got %+v", fake.lastCreate.Pin)
	}
}

func TestExternalCreateFormRefusesWithoutTheWorkItem(t *testing.T) {
	fake := &fakeExternalCases{}
	server := newExternalTestServer(t, fake)
	form := "work_item_id=not-a-uuid&purpose=promise"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalFormRequest(http.MethodPost,
		"/app/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments", form))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("the refusal keeps the PRG discipline, got %d", response.Code)
	}
	if fake.createCalls != 0 {
		t.Fatal("a malformed work item must never reach the service")
	}
	if cookie := response.Header().Get("Set-Cookie"); !strings.Contains(cookie, "staffy_flash_error") {
		t.Errorf("the refusal must flash the honest words, got %q", cookie)
	}
}

func TestExternalApproveFormRequiresTheConfirmation(t *testing.T) {
	fake := &fakeExternalCases{}
	server := newExternalTestServer(t, fake)
	path := "/app/tenants/" + testTenantID.String() + "/cases/" + testExtCaseID.String() +
		"/work-items/" + testExtWorkID.String() + "/external-commitments/" + testExtEnvelope.String() + "/versions/2/approve"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalFormRequest(http.MethodPost, path, ""))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("the unconfirmed form keeps the PRG discipline, got %d", response.Code)
	}
	if fake.approveCalls != 0 {
		t.Fatal("the unconfirmed approval must never reach the service")
	}
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalFormRequest(http.MethodPost, path, "confirm_approve=yes"))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("the confirmed form answers PRG, got %d", response.Code)
	}
	if !strings.Contains(response.Header().Get("Location"), "?approved=2") {
		t.Errorf("the PRG target carries the approved version, got %q", response.Header().Get("Location"))
	}
	if fake.approveCalls != 1 || fake.lastApprove == nil {
		t.Fatal("the confirmed approval must reach the service")
	}
}

func TestExternalEnqueueAndRelayFormsTravelThePathPin(t *testing.T) {
	fake := &fakeExternalCases{}
	server := newExternalTestServer(t, fake)
	base := "/app/tenants/" + testTenantID.String() + "/cases/" + testExtCaseID.String() +
		"/work-items/" + testExtWorkID.String() + "/external-commitments/" + testExtEnvelope.String() + "/versions/2"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalFormRequest(http.MethodPost, base+"/enqueue", "confirm_enqueue=yes"))
	if response.Code != http.StatusSeeOther || !strings.Contains(response.Header().Get("Location"), "?enqueued=2") {
		t.Fatalf("the enqueue form answers PRG with the version, got %d %q", response.Code, response.Header().Get("Location"))
	}
	if fake.enqueueCalls != 1 || fake.lastEnqueue == nil || fake.lastEnqueue.VersionNo != 2 || fake.lastEnqueue.WorkItemID != testExtWorkID {
		t.Fatalf("the enqueue pin must travel verbatim, got %+v", fake.lastEnqueue)
	}
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalFormRequest(http.MethodPost, base+"/dispatch-receipts", "observation=delivered"))
	if response.Code != http.StatusSeeOther || !strings.Contains(response.Header().Get("Location"), "?relayed=2") {
		t.Fatalf("the relay form answers PRG with the version, got %d %q", response.Code, response.Header().Get("Location"))
	}
	if fake.relayCalls != 1 || fake.lastRelay == nil || fake.lastRelay.Observation != casesDomain.RelayObservationDelivered {
		t.Fatalf("the observation must travel verbatim, got %+v", fake.lastRelay)
	}
}

func TestExternalDrainFormRequiresTheConfirmation(t *testing.T) {
	fake := &fakeExternalCases{drain: casesAPI.ExternalDrainResult{Considered: 4}}
	server := newExternalTestServer(t, fake)
	path := "/app/tenants/" + testTenantID.String() + "/cases/external-commitments/drain"
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalFormRequest(http.MethodPost, path, "case_id="+testExtCaseID.String()))
	if response.Code != http.StatusSeeOther || fake.drainCalls != 0 {
		t.Fatalf("the unconfirmed drain never reaches the service, got %d calls %d", response.Code, fake.drainCalls)
	}
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, externalFormRequest(http.MethodPost, path,
		"case_id="+testExtCaseID.String()+"&confirm_drain=yes"))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("the confirmed drain answers PRG, got %d", response.Code)
	}
	if !strings.Contains(response.Header().Get("Location"), "/cases/"+testExtCaseID.String()+"/external-commitments?drain=4") {
		t.Errorf("the drain PRG target carries the considered counter, got %q", response.Header().Get("Location"))
	}
	if fake.drainCalls != 1 {
		t.Fatalf("the confirmed drain must reach the service once, got %d", fake.drainCalls)
	}
}

func TestExternalFormsRefuseTheForeignOrigin(t *testing.T) {
	fake := &fakeExternalCases{}
	server := newExternalTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		"/app/tenants/"+testTenantID.String()+"/cases/external-commitments/drain", strings.NewReader("case_id="+testExtCaseID.String()+"&confirm_drain=yes"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("a foreign origin is the honest 403, got %d", response.Code)
	}
	if fake.drainCalls != 0 {
		t.Fatal("a foreign-origin form must never reach the service")
	}
}

func TestExternalPageRendersTheFlashAndClearsIt(t *testing.T) {
	fake := &fakeExternalCases{view: externalFixtureView()}
	server := newExternalTestServer(t, fake)
	request := httptest.NewRequest(http.MethodGet,
		"/app/tenants/"+testTenantID.String()+"/cases/"+testExtCaseID.String()+"/external-commitments", nil)
	request.AddCookie(&http.Cookie{Name: "staffy_flash_error", Value: "%D0%A1%D0%BB%D0%BE%D0%B2%D0%B0%D1%80%D0%BD%D1%8B%D0%B9%20%D0%BE%D1%82%D0%BA%D0%B0%D0%B7", Path: "/app/tenants/" + testTenantID.String()})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("the page with the flash answers 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Словарный отказ") {
		t.Errorf("the flash words must render on the page")
	}
	if cookie := response.Header().Get("Set-Cookie"); !strings.Contains(cookie, "Max-Age=0") && !strings.Contains(cookie, "Max-Age=-1") {
		t.Errorf("the one-shot flash must clear, got %q", cookie)
	}
}
