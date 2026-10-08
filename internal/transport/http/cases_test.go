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

func newCasesTestServer(t *testing.T, fake *fakeCases) *Server {
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
