package httptransport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 2.14 transport fakes: the decision context surface is pinned
// by the wire-shape tests (the command semantics live in the integration
// and contract contours — the continuation precedent).

const decisionContextTenant = "01994f64-6c00-7000-8000-000000000011"
const decisionContextCase = "01994f64-6c00-7000-8000-00000000aa01"
const decisionContextPath = "/v1/tenants/" + decisionContextTenant + "/cases/" + decisionContextCase
const summariesPath = decisionContextPath + "/summaries"
const decisionContextReadPath = decisionContextPath + "/decision-context"

// decisionContextCases wraps the transport fake with the narrow
// interface of Story 2.14 (the fakeCases stubs stay untouched).
type decisionContextCases struct {
	*fakeCases
	record func(context.Context, casesAPI.RecordCaseSummaryCommand) (casesAPI.WorkOutcome, error)
	read   func(context.Context, domain.Principal, id.UUID, id.UUID) (casesAPI.DecisionContextView, error)
}

func (f *decisionContextCases) RecordCaseSummary(ctx context.Context, command casesAPI.RecordCaseSummaryCommand) (casesAPI.WorkOutcome, error) {
	if f.record == nil {
		return casesAPI.WorkOutcome{}, nil
	}
	return f.record(ctx, command)
}

func (f *decisionContextCases) DecisionContext(ctx context.Context, actor domain.Principal, tenantID, caseID id.UUID) (casesAPI.DecisionContextView, error) {
	if f.read == nil {
		return casesAPI.DecisionContextView{}, nil
	}
	return f.read(ctx, actor, tenantID, caseID)
}

const summaryRequestBody = `{
 "kind": "ai_summary",
 "text": "Клиент ожидает звонка сегодня; адрес подтверждён сообщением клиента.",
 "claims": [
   {"ref_type": "fact_version", "ref_id": "01994f64-6c00-7000-8000-00000000ff01", "text": "адрес подтверждён сообщением клиента"},
   {"disputed": true, "text": "Клиент ожидает звонка сегодня"}
 ]
}`

func TestRecordCaseSummaryWireShapeMatchesContract(t *testing.T) {
	var received casesAPI.RecordCaseSummaryCommand
	server := newCasesTestServer(t, &decisionContextCases{fakeCases: &fakeCases{}, record: func(_ context.Context, command casesAPI.RecordCaseSummaryCommand) (casesAPI.WorkOutcome, error) {
		received = command
		return casesAPI.WorkOutcome{Command: "record_case_summary", SummaryKind: "ai_summary", SummaryClaims: 2}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, summariesPath, summaryRequestBody))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Idempotency-Replayed"); got != "" {
		t.Fatalf("a fresh record is not a replay, header %q", got)
	}
	var body struct {
		Command       string `json:"command"`
		SummaryKind   string `json:"summary_kind"`
		SummaryClaims int    `json:"summary_claims"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Command != "record_case_summary" || body.SummaryKind != "ai_summary" || body.SummaryClaims != 2 {
		t.Fatalf("unexpected outcome body: %+v", body)
	}
	// The command carries the producer material: the kind word, the text
	// with the claims (the refs and the disputed marks), the idempotency
	// key and the correlation id of the request — NEVER the summary
	// number or the watermark (the server truth).
	if received.Kind != "ai_summary" || !strings.Contains(received.Text, "адрес подтверждён") {
		t.Fatalf("unexpected command material: %+v", received)
	}
	if len(received.Claims) != 2 || received.Claims[0].RefType != "fact_version" || !received.Claims[1].Disputed {
		t.Fatalf("unexpected claims material: %+v", received.Claims)
	}
	if !received.Claims[0].RefID.Valid() {
		t.Fatal("expected the parsed ref id of the linked claim")
	}
	if received.IdempotencyKey != "case-key-1" {
		t.Fatalf("expected the idempotency key header, got %q", received.IdempotencyKey)
	}
	if !received.CorrelationID.Valid() {
		t.Fatal("expected the correlation id of the request")
	}
	if !received.CaseID.Valid() || !received.TenantID.Valid() || !received.Actor.ID.Valid() {
		t.Fatal("expected the resolved actor, tenant and case anchors")
	}
}

func TestRecordCaseSummaryReplayWireShapeMatchesContract(t *testing.T) {
	server := newCasesTestServer(t, &decisionContextCases{fakeCases: &fakeCases{}, record: func(context.Context, casesAPI.RecordCaseSummaryCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{Command: "record_case_summary", Replay: true, SummaryKind: "ai_summary", SummaryClaims: 2}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, summariesPath, summaryRequestBody))
	if recorder.Code != http.StatusOK {
		t.Fatalf("a replay is 200 with the original outcome, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Idempotency-Replayed"); got != "true" {
		t.Fatalf("expected the replay header, got %q", got)
	}
}

func TestRecordCaseSummaryRefusalWireShapeByKind(t *testing.T) {
	refusals := []struct {
		name     string
		err      error
		expected int
		code     string
	}{
		{"rule", casesDomain.WorkRuleError{Rule: "record_case_summary", Reason: "текст утверждения отсутствует в тексте сводки байт-в-байт"}, http.StatusBadRequest, "record_case_summary"},
		{"forbidden", domain.ErrForbidden, http.StatusForbidden, "cases_forbidden"},
		{"not_found", casesDomain.ErrNotFound, http.StatusNotFound, "not_found"},
		{"key_conflict", casesDomain.ErrCommandKeyConflict, http.StatusConflict, "command_key_conflict"},
		{"stale_version", casesDomain.ErrStaleVersion, http.StatusConflict, "stale_version"},
	}
	for _, refusal := range refusals {
		server := newCasesTestServer(t, &decisionContextCases{fakeCases: &fakeCases{}, record: func(context.Context, casesAPI.RecordCaseSummaryCommand) (casesAPI.WorkOutcome, error) {
			return casesAPI.WorkOutcome{}, refusal.err
		}})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, summariesPath, summaryRequestBody))
		if recorder.Code != refusal.expected {
			t.Fatalf("%s: expected %d, got %d: %s", refusal.name, refusal.expected, recorder.Code, recorder.Body.String())
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != refusal.code {
			t.Fatalf("%s: expected the %s problem code, got %q", refusal.name, refusal.code, body.Code)
		}
	}
}

func TestRecordCaseSummaryInputValidationRefusesHonestly(t *testing.T) {
	bodies := map[string]string{
		"missing kind":       `{"text": "текст", "claims": [{"disputed": true, "text": "текст"}]}`,
		"missing text":       `{"kind": "ai_summary", "claims": [{"disputed": true, "text": "текст"}]}`,
		"missing claims":     `{"kind": "ai_summary", "text": "текст"}`,
		"claim without text": `{"kind": "ai_summary", "text": "текст", "claims": [{"disputed": true}]}`,
		"ref not uuid":       `{"kind": "ai_summary", "text": "текст", "claims": [{"ref_type": "fact_version", "ref_id": "not-a-uuid", "text": "текст"}]}`,
		"unknown field":      `{"kind": "ai_summary", "text": "текст", "claims": [{"disputed": true, "text": "текст"}], "summary_no": 1}`,
	}
	for name, body := range bodies {
		server := newCasesTestServer(t, &decisionContextCases{fakeCases: &fakeCases{}})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, summariesPath, body))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", name, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "invalid_request") {
			t.Fatalf("%s: expected the invalid_request problem code, got %s", name, recorder.Body.String())
		}
	}
}

func TestDecisionContextReadWireShapeMatchesContract(t *testing.T) {
	var received struct {
		actor  domain.Principal
		tenant id.UUID
		caseID id.UUID
	}
	server := newCasesTestServer(t, &decisionContextCases{fakeCases: &fakeCases{}, read: func(_ context.Context, actor domain.Principal, tenantID, caseID id.UUID) (casesAPI.DecisionContextView, error) {
		received.actor, received.tenant, received.caseID = actor, tenantID, caseID
		return casesAPI.DecisionContextView{
			Present:   true,
			PanelText: "Контекст передачи — производное чтение собранного материала.",
			Handoff:   casesAPI.ContextHandoffView{Present: true, Reason: "client_request", State: "assigned"},
			Facts: casesAPI.ContextFactsView{
				Confirmed: []casesAPI.ContextFactRow{{FactID: "01994f64-6c00-7000-8000-00000000ff01", MaterialKey: "address", Status: "confirmed"}},
			},
			Summary: casesAPI.SummaryView{
				Present: true, SummaryNo: 1, Kind: "ai_summary", Text: "адрес подтверждён", Staleness: "actual",
			},
		}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, decisionContextReadPath, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Present bool `json:"present"`
		Handoff struct {
			Present bool   `json:"present"`
			Reason  string `json:"reason"`
		} `json:"handoff"`
		Facts struct {
			Confirmed []struct {
				MaterialKey string `json:"material_key"`
				Status      string `json:"status"`
			} `json:"confirmed"`
		} `json:"facts"`
		Summary struct {
			Present   bool   `json:"present"`
			SummaryNo int64  `json:"summary_no"`
			Staleness string `json:"staleness"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Present || !body.Handoff.Present || body.Handoff.Reason != "client_request" {
		t.Fatalf("unexpected context body: %+v", body)
	}
	if len(body.Facts.Confirmed) != 1 || body.Facts.Confirmed[0].Status != "confirmed" {
		t.Fatalf("unexpected fact groups: %+v", body.Facts)
	}
	if !body.Summary.Present || body.Summary.SummaryNo != 1 || body.Summary.Staleness != "actual" {
		t.Fatalf("unexpected summary block: %+v", body.Summary)
	}
	if !received.caseID.Valid() || !received.tenant.Valid() || !received.actor.ID.Valid() {
		t.Fatal("expected the resolved actor, tenant and case anchors of the read")
	}
}

func TestDecisionContextReadRefusalWireShapeByKind(t *testing.T) {
	refusals := []struct {
		name     string
		err      error
		expected int
		code     string
	}{
		{"forbidden", domain.ErrForbidden, http.StatusForbidden, "cases_forbidden"},
		{"not_found", casesDomain.ErrNotFound, http.StatusNotFound, "not_found"},
	}
	for _, refusal := range refusals {
		server := newCasesTestServer(t, &decisionContextCases{fakeCases: &fakeCases{}, read: func(context.Context, domain.Principal, id.UUID, id.UUID) (casesAPI.DecisionContextView, error) {
			return casesAPI.DecisionContextView{}, refusal.err
		}})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, decisionContextReadPath, nil))
		if recorder.Code != refusal.expected {
			t.Fatalf("%s: expected %d, got %d: %s", refusal.name, refusal.expected, recorder.Code, recorder.Body.String())
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != refusal.code {
			t.Fatalf("%s: expected the %s problem code, got %q", refusal.name, refusal.code, body.Code)
		}
	}
}
