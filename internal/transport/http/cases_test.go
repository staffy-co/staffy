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

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/httpx"
	"github.com/artemiosu/staffy/internal/platform/id"
)

func casesFixtureID() id.UUID { return id.MustParse("01994f64-6c00-7000-8000-00000000c0de") }

type fakeCases struct {
	ingest  func(context.Context, casesAPI.IngestCommand) (casesAPI.IngestOutcome, error)
	list    func(context.Context, domain.Principal, id.UUID) (casesAPI.CasesView, error)
	detail  func(context.Context, domain.Principal, id.UUID, id.UUID) (casesAPI.CaseView, error)
	payload func(context.Context, domain.Principal, id.UUID, id.UUID, id.UUID) (casesAPI.PayloadView, error)
}

func (f *fakeCases) IngestEvent(ctx context.Context, command casesAPI.IngestCommand) (casesAPI.IngestOutcome, error) {
	if f.ingest == nil {
		return casesAPI.IngestOutcome{}, nil
	}
	return f.ingest(ctx, command)
}

func (f *fakeCases) CasesView(ctx context.Context, actor domain.Principal, tenant id.UUID) (casesAPI.CasesView, error) {
	if f.list == nil {
		return casesAPI.CasesView{}, nil
	}
	return f.list(ctx, actor, tenant)
}

func (f *fakeCases) CaseView(ctx context.Context, actor domain.Principal, tenant, caseID id.UUID) (casesAPI.CaseView, error) {
	if f.detail == nil {
		return casesAPI.CaseView{}, nil
	}
	return f.detail(ctx, actor, tenant, caseID)
}

func (f *fakeCases) ReadPayload(ctx context.Context, actor domain.Principal, tenant, caseID, eventID id.UUID) (casesAPI.PayloadView, error) {
	if f.payload == nil {
		return casesAPI.PayloadView{}, nil
	}
	return f.payload(ctx, actor, tenant, caseID, eventID)
}

func (f *fakeCases) DrainOutbox(context.Context, domain.Principal, id.UUID) (casesAPI.DrainResult, error) {
	return casesAPI.DrainResult{}, nil
}

// The Story 2.4 ownership stubs: the wire-shape tests target the surface
// contract; the command semantics live in the integration suite.
func (f *fakeCases) TakeOwnership(context.Context, casesAPI.TakeOwnershipCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) RequestTransfer(context.Context, casesAPI.RequestTransferCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) ResolveTransfer(context.Context, casesAPI.ResolveTransferCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) ReassignOwnership(context.Context, casesAPI.ReassignOwnershipCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) SendClientReply(context.Context, casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) DrainClientReplies(context.Context, domain.Principal, id.UUID) (casesAPI.DrainRepliesResult, error) {
	return casesAPI.DrainRepliesResult{}, nil
}

// The Story 2.5 mode stubs: the wire-shape tests target the surface
// contract; the command semantics live in the integration suite.
func (f *fakeCases) ProposeMode(context.Context, casesAPI.ProposeModeCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) ExitMode(context.Context, casesAPI.ExitModeCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) RecordDisclosure(context.Context, casesAPI.RecordDisclosureCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) RecordDisclosureEvent(context.Context, casesAPI.RecordDisclosureEventCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) SubmitAIProposal(context.Context, casesAPI.SubmitAIProposalCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}
func (f *fakeCases) EvaluatePolicy(context.Context, casesAPI.EvaluatePolicyCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, nil
}

func newCasesTestServer(t *testing.T, fake casesAPI.Service) *Server {
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
	server.BindCases(fake)
	return server
}

func casesJSONRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "case-key-1")
	request.Header.Set("X-Correlation-ID", "01994f64-6c00-7000-8000-0000000000ff")
	return request
}

const ingressBody = `{
 "schema_version": 1,
 "tenant_id": "01994f64-6c00-7000-8000-000000000011",
 "connection_id": "01994f64-6c00-7000-8000-00000000c0de",
 "provider": "staffy-synthetic-telephony",
 "provider_event_id": "evt-42",
 "provider_session_id": "sess-7",
 "direction": "inbound",
 "modality": "voice",
 "channel": "phone",
 "provider_timestamp": "2026-10-09T10:59:00Z",
 "payload": {"event":"inbound-call","from":"+79000000000"}
}`

func TestIngressAcceptWireShapeMatchesContract(t *testing.T) {
	var received casesAPI.IngestCommand
	server := newCasesTestServer(t, &fakeCases{ingest: func(_ context.Context, command casesAPI.IngestCommand) (casesAPI.IngestOutcome, error) {
		received = command
		return casesAPI.IngestOutcome{Accepted: &casesAPI.IngestResult{
			EventID: casesFixtureID(), CaseID: casesFixtureID(), ConversationID: casesFixtureID(),
			CaseNo: 1, ConversationNo: 1, ResponseOwnerID: testPrincipalID, EmployeeID: testPrincipalID,
			NewCase: true, Replay: false,
		}}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, "/v1/ingress/channel-events", ingressBody))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Idempotency-Replayed"); got != "" {
		t.Fatalf("a fresh accept is not a replay, header %q", got)
	}
	if got := recorder.Header().Get("Location"); !strings.Contains(got, "/cases/") {
		t.Fatalf("expected the case location header, got %q", got)
	}
	var body struct {
		EventID         string `json:"event_id"`
		CaseID          string `json:"case_id"`
		ConversationID  string `json:"conversation_id"`
		CaseNo          int64  `json:"case_no"`
		ConversationNo  int64  `json:"conversation_no"`
		ResponseOwnerID string `json:"response_owner_id"`
		EmployeeID      string `json:"employee_id"`
		NewCase         bool   `json:"new_case"`
		Replay          bool   `json:"replay"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.CaseNo != 1 || body.ConversationNo != 1 || !body.NewCase || body.Replay {
		t.Fatalf("unexpected accepted body: %+v", body)
	}
	// The command carries the canonical envelope: strict fields, canonical
	// payload bytes, measured envelope size, the idempotency key and the
	// correlation id of the request.
	if received.Envelope.SchemaVersion != 1 || received.Envelope.Channel != "phone" {
		t.Fatalf("unexpected envelope: %+v", received.Envelope)
	}
	if received.Envelope.PayloadDigest == "" || !strings.Contains(string(received.Envelope.Payload), "inbound-call") {
		t.Fatalf("expected the canonical payload with digest")
	}
	if received.IdempotencyKey != "case-key-1" {
		t.Fatalf("expected the idempotency key header, got %q", received.IdempotencyKey)
	}
	if !received.CorrelationID.Valid() {
		t.Fatal("expected the correlation id of the request")
	}
	if received.Envelope.EnvelopeBytes == 0 {
		t.Fatal("expected the measured envelope size for the ladder bound")
	}
}

func TestIngressReplayWireShapeMatchesContract(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{ingest: func(context.Context, casesAPI.IngestCommand) (casesAPI.IngestOutcome, error) {
		return casesAPI.IngestOutcome{Accepted: &casesAPI.IngestResult{
			EventID: casesFixtureID(), CaseID: casesFixtureID(), ConversationID: casesFixtureID(),
			CaseNo: 1, ConversationNo: 1, NewCase: false, Replay: true,
		}}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, "/v1/ingress/channel-events", ingressBody))
	if recorder.Code != http.StatusOK {
		t.Fatalf("a replay is 200 with the original result, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Idempotency-Replayed"); got != "true" {
		t.Fatalf("expected the replay header, got %q", got)
	}
}

func TestIngressRefusalWireShapeByKind(t *testing.T) {
	refusals := []struct {
		kind     string
		bounded  bool
		counter  int
		expected int
	}{
		{"malformed", false, 0, http.StatusBadRequest},
		{"unknown_connection", false, 0, http.StatusNotFound},
		{"revoked_connection", false, 0, http.StatusForbidden},
		{"production_forbidden", false, 0, http.StatusForbidden},
		{"channel_mismatch", false, 0, http.StatusBadRequest},
		{"route_not_admitted", false, 0, http.StatusForbidden},
		{"no_active_release", false, 0, http.StatusForbidden},
		{"no_response_owner", false, 0, http.StatusForbidden},
		{"entitlement_blocked", true, 0, http.StatusTooManyRequests},
		{"envelope_exceeded", true, 204800, http.StatusTooManyRequests},
	}
	for _, refusal := range refusals {
		server := newCasesTestServer(t, &fakeCases{ingest: func(context.Context, casesAPI.IngestCommand) (casesAPI.IngestOutcome, error) {
			return casesAPI.IngestOutcome{Refused: &casesAPI.IngestRefusal{
				Kind: refusal.kind, Message: "safe text", Counter: refusal.counter, Bounded: refusal.bounded,
			}}, nil
		}})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, "/v1/ingress/channel-events", ingressBody))
		if recorder.Code != refusal.expected {
			t.Fatalf("kind %s: expected %d, got %d", refusal.kind, refusal.expected, recorder.Code)
		}
		var problem struct {
			Code          string            `json:"code"`
			Message       string            `json:"message"`
			CorrelationID string            `json:"correlation_id"`
			Retryable     bool              `json:"retryable"`
			FieldErrors   map[string]string `json:"field_errors"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
			t.Fatalf("kind %s: %v", refusal.kind, err)
		}
		if problem.Code != refusal.kind {
			t.Fatalf("kind %s: expected the kind as code, got %q", refusal.kind, problem.Code)
		}
		if problem.CorrelationID == "" {
			t.Fatalf("kind %s: expected the correlation id in the refusal", refusal.kind)
		}
		if refusal.counter != 0 && problem.FieldErrors["counter"] != "204800" {
			t.Fatalf("kind %s: expected the counter field, got %v", refusal.kind, problem.FieldErrors)
		}
	}
}

func TestIngressQuarantineWireShape(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{ingest: func(context.Context, casesAPI.IngestCommand) (casesAPI.IngestOutcome, error) {
		return casesAPI.IngestOutcome{Quarantined: &casesAPI.IngestQuarantine{
			Kind: "digest_conflict", ExceptionID: casesFixtureID(), EventID: casesFixtureID(),
		}}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, "/v1/ingress/channel-events", ingressBody))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", recorder.Code)
	}
	var problem struct {
		Code        string            `json:"code"`
		FieldErrors map[string]string `json:"field_errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "digest_conflict" || problem.FieldErrors["exception_id"] == "" {
		t.Fatalf("expected the quarantine kind and exception id, got %+v", problem)
	}
}

func TestIngressContractHonestRefusals(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{})
	handler := server.Handler()

	recorder := httptest.NewRecorder()
	request := casesJSONRequest(http.MethodPost, "/v1/ingress/channel-events", ingressBody)
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("non-JSON content type is 415, got %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = casesJSONRequest(http.MethodPost, "/v1/ingress/channel-events", ingressBody)
	request.Header.Del("Idempotency-Key")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("a missing idempotency key is 400, got %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, casesJSONRequest(http.MethodPost, "/v1/ingress/channel-events", `{"schema_version":1,"tenant_id":"01994f64-6c00-7000-8000-000000000011","connection_id":"01994f64-6c00-7000-8000-00000000c0de","provider":"p","provider_event_id":"e","provider_session_id":"s","direction":"inbound","modality":"voice","channel":"phone","provider_timestamp":"2026-10-09T10:59:00Z","payload":{},"unknown_field":1}`))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown fields are 400, got %d: %s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, casesJSONRequest(http.MethodPost, "/v1/ingress/channel-events", `{"schema_version":1}`))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("a broken envelope is 400, got %d", recorder.Code)
	}
}

func TestCasesListWireShapeMatchesContract(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{list: func(context.Context, domain.Principal, id.UUID) (casesAPI.CasesView, error) {
		return casesAPI.CasesView{
			TenantID:   casesFixtureID(),
			Cases:      []casesDomain.CaseSummary{},
			Exceptions: []casesDomain.OperationalException{},
		}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCasesSurfaceHonestRefusalWire(t *testing.T) {
	forbiddenServer := newCasesTestServer(t, &fakeCases{list: func(context.Context, domain.Principal, id.UUID) (casesAPI.CasesView, error) {
		return casesAPI.CasesView{}, domain.ErrForbidden
	}})
	recorder := httptest.NewRecorder()
	forbiddenServer.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("a member without the scope is 403, got %d", recorder.Code)
	}

	notFoundServer := newCasesTestServer(t, &fakeCases{detail: func(context.Context, domain.Principal, id.UUID, id.UUID) (casesAPI.CaseView, error) {
		return casesAPI.CaseView{}, domain.ErrNotFound
	}})
	recorder = httptest.NewRecorder()
	notFoundServer.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000c0de", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("a foreign tenant read is a plain 404, got %d", recorder.Code)
	}
}

func TestCasePayloadRequiresReviewOrAudit(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{payload: func(context.Context, domain.Principal, id.UUID, id.UUID, id.UUID) (casesAPI.PayloadView, error) {
		return casesAPI.PayloadView{}, domain.ErrForbidden
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000c0de/events/01994f64-6c00-7000-8000-00000000c0de/payload", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("the payload is review/audit only, got %d", recorder.Code)
	}
}

// The Story 2.2 command surface of the fake: every command carries an
// optional outcome or error hook; the default is the honest zero outcome.
func (f *fakeCases) CreateWorkItem(context.Context, casesAPI.CreateWorkItemCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "create_work_item", Version: 1}, nil
}

func (f *fakeCases) ClassifyWorkItem(context.Context, casesAPI.ClassifyCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "classify_work_item", Version: 2, State: "in_work"}, nil
}

func (f *fakeCases) TransitionWorkItem(context.Context, casesAPI.TransitionCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "transition_work_item", Version: 3, State: "waiting"}, nil
}

func (f *fakeCases) SplitWorkItem(context.Context, casesAPI.SplitCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "split_work_item", Version: 4, State: "split"}, nil
}

func (f *fakeCases) MergeWorkItems(context.Context, casesAPI.MergeCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "merge_work_items", State: "merged"}, nil
}

func (f *fakeCases) LinkEvent(context.Context, casesAPI.LinkEventCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "link_event"}, nil
}

func (f *fakeCases) DisputeWorkItem(context.Context, casesAPI.DisputeCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "dispute_work_item"}, nil
}

func (f *fakeCases) RecordFact(context.Context, casesAPI.RecordFactCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "record_fact", FactVersion: 1, FactStatus: "unverified"}, nil
}

func (f *fakeCases) ConfirmFactVersion(context.Context, casesAPI.ConfirmFactVersionCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "confirm_fact_version", FactStatus: "confirmed"}, nil
}

func (f *fakeCases) RescopeFact(context.Context, casesAPI.RescopeFactCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{Command: "rescope_fact", FactVersion: 2}, nil
}

func (f *fakeCases) FactsView(context.Context, domain.Principal, id.UUID, id.UUID) (casesAPI.FactsView, error) {
	return casesAPI.FactsView{CanView: true, ViewScope: casesAPI.CaseScopePayload, Facts: []casesAPI.FactView{}}, nil
}

// ---------------------------------------------------------------------------
// Story 2.2: the work command wire contract
// ---------------------------------------------------------------------------

type workErrorCases struct {
	fakeCases
	classify func(context.Context, casesAPI.ClassifyCommand) (casesAPI.WorkOutcome, error)
	facts    func(context.Context, domain.Principal, id.UUID, id.UUID) (casesAPI.FactsView, error)
}

func (f *workErrorCases) ClassifyWorkItem(ctx context.Context, command casesAPI.ClassifyCommand) (casesAPI.WorkOutcome, error) {
	if f.classify == nil {
		return casesAPI.WorkOutcome{}, nil
	}
	return f.classify(ctx, command)
}

func (f *workErrorCases) FactsView(ctx context.Context, actor domain.Principal, tenant, caseID id.UUID) (casesAPI.FactsView, error) {
	if f.facts == nil {
		return casesAPI.FactsView{}, nil
	}
	return f.facts(ctx, actor, tenant, caseID)
}

const classifyPath = "/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/work-items/01994f64-6c00-7000-8000-00000000bb02/classify"

const classifyBody = `{"expected_version":1,"type":"поиск детали","title":"Подбор детали на Civic","reason":"Клиент ищет деталь"}`

func TestClassifyWorkItemWireShapeMatchesContract(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", classifyPath, classifyBody))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var outcome casesAPI.WorkOutcome
	if err := json.Unmarshal(recorder.Body.Bytes(), &outcome); err != nil {
		t.Fatal(err)
	}
	if outcome.Command != "classify_work_item" || outcome.Version != 2 || outcome.State != "in_work" {
		t.Fatalf("outcome = %+v", outcome)
	}
	if recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type = %s", recorder.Header().Get("Content-Type"))
	}
}

func TestWorkCommandRequiresIdempotencyKey(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{})
	request := casesJSONRequest("POST", classifyPath, classifyBody)
	request.Header.Del("Idempotency-Key")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestWorkCommandRejectsUnknownFields(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{})
	// The unknown field sits INSIDE the JSON object — the strict decode
	// (DisallowUnknownFields) is the surface under test, not the trailing
	// garbage the transport rejects on its own.
	body := strings.TrimSuffix(classifyBody, "}") + `,"surprise":1}`
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", classifyPath, body))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestWorkCommandMapsHonestErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"rule refusal", casesDomain.WorkRuleError{Rule: "classify_work_item", Reason: "Тип не входит в словарь пака."}, http.StatusBadRequest},
		{"stale version", casesDomain.ErrStaleVersion, http.StatusConflict},
		{"key conflict", casesDomain.ErrCommandKeyConflict, http.StatusConflict},
		{"not found", casesDomain.ErrNotFound, http.StatusNotFound},
	}
	for _, testCase := range cases {
		fake := &workErrorCases{classify: func(context.Context, casesAPI.ClassifyCommand) (casesAPI.WorkOutcome, error) {
			return casesAPI.WorkOutcome{}, testCase.err
		}}
		server := newCasesTestServer(t, fake)
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", classifyPath, classifyBody))
		if recorder.Code != testCase.status {
			t.Fatalf("%s: status = %d, body = %s", testCase.name, recorder.Code, recorder.Body.String())
		}
	}
}

func TestFactsViewWireShape(t *testing.T) {
	factID := id.MustParse("01994f64-6c00-7000-8000-00000000ff01")
	fake := &workErrorCases{facts: func(context.Context, domain.Principal, id.UUID, id.UUID) (casesAPI.FactsView, error) {
		return casesAPI.FactsView{
			CanView: true, ViewScope: casesAPI.CaseScopePayload,
			Facts: []casesAPI.FactView{{
				FactID: factID, MaterialKey: "автомобиль", Status: "conflicting",
				NextAction: "Разрешите конфликт: подтвердите точную версию со ссылкой на источник.",
			}},
		}, nil
	}}
	server := newCasesTestServer(t, fake)
	path := "/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/facts"
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var view casesAPI.FactsView
	if err := json.Unmarshal(recorder.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Facts) != 1 || view.Facts[0].MaterialKey != "автомобиль" || view.Facts[0].Status != "conflicting" {
		t.Fatalf("facts = %+v", view.Facts)
	}
}

// The Story 2.3 command surface of the fake (the honest refusal shape —
// the transport tests of the new commands use their own fixtures).
func (f *fakeCases) StartVerification(context.Context, casesAPI.StartVerificationCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "start_verification", Reason: "контур проверки не сконфигурирован в фикстуре"}
}

func (f *fakeCases) BindChannel(context.Context, casesAPI.BindChannelCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "bind_channel", Reason: "контур проверки не сконфигурирован в фикстуре"}
}

func (f *fakeCases) UnlinkChannel(context.Context, casesAPI.UnlinkChannelCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "unlink_channel", Reason: "контур проверки не сконфигурирован в фикстуре"}
}

func (f *fakeCases) ResolveDuplicate(context.Context, casesAPI.ResolveDuplicateCommand) (casesAPI.WorkOutcome, error) {
	return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "resolve_duplicate", Reason: "контур проверки не сконфигурирован в фикстуре"}
}

// ---------------------------------------------------------------------------
// The Story 2.4 ownership wire contract.
// ---------------------------------------------------------------------------

const clientReplyPath = "/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/client-replies"

type ownershipErrorCases struct {
	fakeCases
	reply func(context.Context, casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error)
}

func (f *ownershipErrorCases) SendClientReply(ctx context.Context, command casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
	if f.reply == nil {
		return casesAPI.WorkOutcome{}, nil
	}
	return f.reply(ctx, command)
}

func TestSendClientReplyRequiresExpectedEpoch(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{})
	// The OpenAPI contract declares expected_epoch required: the machine
	// path never silently degrades to a server-only CAS.
	body := `{"conversation_id":"01994f64-6c00-7000-8000-00000000cc01","content":"Добрый день!"}`
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", clientReplyPath, body))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "expected_epoch") {
		t.Fatalf("the refusal names the required field, body = %s", recorder.Body.String())
	}
}

func TestSendClientReplyPassesExpectedEpoch(t *testing.T) {
	var seen int64
	fake := &ownershipErrorCases{reply: func(_ context.Context, command casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
		seen = command.ExpectedEpoch
		return casesAPI.WorkOutcome{}, nil
	}}
	server := newCasesTestServer(t, fake)
	body := `{"conversation_id":"01994f64-6c00-7000-8000-00000000cc01","expected_epoch":3,"reply_class":"neutral","content":"Добрый день!"}`
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", clientReplyPath, body))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if seen != 3 {
		t.Fatalf("the command carries the CAS expectation, got %d", seen)
	}
}

func TestReplyGateWireShapeMatchesContract(t *testing.T) {
	gate := casesDomain.ReplyGateError{
		OwnerKind: "human", OwnerName: "Оператор Один", CurrentEpoch: 2,
		Reason: "Отвечать клиенту может только текущий владелец.",
	}
	fake := &ownershipErrorCases{reply: func(context.Context, casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{}, gate
	}}
	server := newCasesTestServer(t, fake)
	body := `{"conversation_id":"01994f64-6c00-7000-8000-00000000cc01","expected_epoch":2,"reply_class":"neutral","content":"посторонний текст"}`
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", clientReplyPath, body))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var problem httpx.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "reply_gate" {
		t.Fatalf("code = %s, body = %s", problem.Code, recorder.Body.String())
	}
	if problem.FieldErrors["current_owner_kind"] != "human" ||
		problem.FieldErrors["current_owner_name"] != "Оператор Один" ||
		problem.FieldErrors["current_epoch"] != "2" ||
		!strings.Contains(problem.FieldErrors["permitted_path"], "request_transfer") {
		t.Fatalf("the gate refusal carries the owner and the permitted path, fields = %+v", problem.FieldErrors)
	}
}

// ---------------------------------------------------------------------------
// The Story 2.5 mode wire contract.
// ---------------------------------------------------------------------------

const proposeModePath = "/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/mode/proposals"

// modeErrorCases injects the mode command errors of the wire tests (the
// Story 2.5 surface contract; the command semantics live in the
// integration suite).
type modeErrorCases struct {
	fakeCases
	propose func(context.Context, casesAPI.ProposeModeCommand) (casesAPI.WorkOutcome, error)
	reply   func(context.Context, casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error)
}

func (f *modeErrorCases) ProposeMode(ctx context.Context, command casesAPI.ProposeModeCommand) (casesAPI.WorkOutcome, error) {
	if f.propose == nil {
		return casesAPI.WorkOutcome{}, nil
	}
	return f.propose(ctx, command)
}

func (f *modeErrorCases) SendClientReply(ctx context.Context, command casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
	if f.reply == nil {
		return casesAPI.WorkOutcome{}, nil
	}
	return f.reply(ctx, command)
}

func TestReplyModeGateWireShapeMatchesContract(t *testing.T) {
	gate := casesDomain.ReplyModeError{
		EffectiveMode: casesDomain.ModeSafety, ModeReason: "сигнал безопасности",
		SignalSource: casesDomain.ModeSignalTurnEvent, RuleVersion: casesDomain.ModePolicyVersion,
	}
	gate.Reason = casesDomain.ReplyModeGateText
	fake := &modeErrorCases{reply: func(context.Context, casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{}, gate
	}}
	server := newCasesTestServer(t, fake)
	body := `{"conversation_id":"01994f64-6c00-7000-8000-00000000cc01","expected_epoch":2,"reply_class":"sales","content":"предлагаю"}`
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", clientReplyPath, body))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var problem httpx.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "reply_mode_gate" {
		t.Fatalf("code = %s, body = %s", problem.Code, recorder.Body.String())
	}
	for field, want := range map[string]string{
		"active_mode":    casesDomain.ModeSafety,
		"reason":         "сигнал безопасности",
		"signal_source":  casesDomain.ModeSignalTurnEvent,
		"rule_version":   casesDomain.ModePolicyVersion,
		"permitted_path": casesDomain.PermittedPathExit,
	} {
		if problem.FieldErrors[field] != want {
			t.Fatalf("field %s = %q, want %q (body %s)", field, problem.FieldErrors[field], want, recorder.Body.String())
		}
	}
}

func TestReplyDisclosureGateWireShapeMatchesContract(t *testing.T) {
	gate := casesDomain.ReplyDisclosureError{Channel: "phone", Reason: casesDomain.DisclosureGateText}
	fake := &modeErrorCases{reply: func(context.Context, casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{}, gate
	}}
	server := newCasesTestServer(t, fake)
	body := `{"conversation_id":"01994f64-6c00-7000-8000-00000000cc01","expected_epoch":2,"reply_class":"protected_request","content":"подтвердите данные"}`
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", clientReplyPath, body))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var problem httpx.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "reply_disclosure_gate" {
		t.Fatalf("code = %s, body = %s", problem.Code, recorder.Body.String())
	}
	if problem.FieldErrors["channel"] != "phone" || problem.FieldErrors["permitted_path"] != casesDomain.PermittedPathRecordDisclosure {
		t.Fatalf("the channel and the permitted path words travel as fields, body = %s", recorder.Body.String())
	}
}

func TestReplyClassRequiredWireWord(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{})
	body := `{"conversation_id":"01994f64-6c00-7000-8000-00000000cc01","expected_epoch":2,"content":"без класса"}`
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", clientReplyPath, body))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "класс размечает продюсер") {
		t.Fatalf("the refusal names the producer's mark discipline, body = %s", recorder.Body.String())
	}
}

func TestModeConflictWireShapeMatchesContract(t *testing.T) {
	conflict := casesDomain.ModeConflictError{
		ActiveMode: casesDomain.ModeClaim, Reason: "претензия",
		SignalSource: casesDomain.ModeSignalOperator, RuleVersion: casesDomain.ModePolicyVersion, ModeVersion: 2,
	}
	fake := &modeErrorCases{propose: func(context.Context, casesAPI.ProposeModeCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{}, conflict
	}}
	server := newCasesTestServer(t, fake)
	body := `{"conversation_id":"01994f64-6c00-7000-8000-00000000cc01","mode":"safety","scope":"conversation","expected_mode_version":1,"reason":"r","signal_source":"operator"}`
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", proposeModePath, body))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var problem httpx.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "mode_conflict" {
		t.Fatalf("code = %s, body = %s", problem.Code, recorder.Body.String())
	}
	for field, want := range map[string]string{
		"active_mode":    casesDomain.ModeClaim,
		"reason":         "претензия",
		"signal_source":  casesDomain.ModeSignalOperator,
		"rule_version":   casesDomain.ModePolicyVersion,
		"mode_version":   "2",
		"permitted_path": "propose_mode",
	} {
		if problem.FieldErrors[field] != want {
			t.Fatalf("field %s = %q, want %q (body %s)", field, problem.FieldErrors[field], want, recorder.Body.String())
		}
	}
}

func TestProposeModeRequiresExpectedVersion(t *testing.T) {
	server := newCasesTestServer(t, &fakeCases{})
	body := `{"conversation_id":"01994f64-6c00-7000-8000-00000000cc01","mode":"safety","scope":"conversation","reason":"r","signal_source":"operator"}`
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest("POST", proposeModePath, body))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "expected_mode_version") {
		t.Fatalf("the refusal names the required CAS word, body = %s", recorder.Body.String())
	}
}
