package httptransport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 2.15 transport fakes: the correction surface is pinned by the
// wire-shape tests (the command semantics live in the integration and
// contract contours — the decision context precedent). The OUTCOME probe
// is the strict decode: the request carrying the field refuses before the
// command (the matrix gate of the domain decides the word — never the
// request).

const correctionFact = "01994f64-6c00-7000-8000-00000000ff01"
const correctionsPath = decisionContextPath + "/facts/" + correctionFact + "/corrections"
const correctionsFormPath = "/app/tenants/" + decisionContextTenant + "/cases/" + decisionContextCase + "/facts/" + correctionFact + "/corrections"

// correctionCases wraps the transport fake with the narrow interface of
// Story 2.15 (the fakeCases stubs stay untouched).
type correctionCases struct {
	*fakeCases
	record func(context.Context, casesAPI.RecordCorrectionCommand) (casesAPI.WorkOutcome, error)
}

func (f *correctionCases) RecordCorrection(ctx context.Context, command casesAPI.RecordCorrectionCommand) (casesAPI.WorkOutcome, error) {
	if f.record == nil {
		return casesAPI.WorkOutcome{}, nil
	}
	return f.record(ctx, command)
}

const correctionRequestBody = `{
 "version_no": 1,
 "summary_no": 0,
 "value": {"срок": 3},
 "spans": ["Срок доставки клиенту не назван."],
 "reason": "клиент назвал новый срок в ответе"
}`

func TestRecordCorrectionWireShapeMatchesContract(t *testing.T) {
	var received casesAPI.RecordCorrectionCommand
	server := newCasesTestServer(t, &correctionCases{fakeCases: &fakeCases{}, record: func(_ context.Context, command casesAPI.RecordCorrectionCommand) (casesAPI.WorkOutcome, error) {
		received = command
		return casesAPI.WorkOutcome{Command: "record_correction", CorrectionID: id.MustParse("01994f64-6c00-7000-8000-000000000cc9"), CorrectionOutcome: "applied", CorrectionSpans: 1}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, correctionsPath, correctionRequestBody))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Idempotency-Replayed"); got != "" {
		t.Fatalf("a fresh record is not a replay, header %q", got)
	}
	var body struct {
		Command           string    `json:"command"`
		CorrectionID      id.UUID   `json:"correction_id"`
		CorrectionOutcome string    `json:"correction_outcome"`
		CorrectionSpans   int       `json:"correction_spans"`
		CorrectionNo      *int64    `json:"correction_no"`
		ProposedValue     *struct{} `json:"proposed_value"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Command != "record_correction" || body.CorrectionOutcome != "applied" || body.CorrectionSpans != 1 || !body.CorrectionID.Valid() {
		t.Fatalf("unexpected outcome body: %+v", body)
	}
	// The stored outcome never carries the server number or the material:
	// the correction number, the proposed value and the spans NEVER enter.
	if body.CorrectionNo != nil || body.ProposedValue != nil {
		t.Fatalf("the outcome must NOT carry the correction number or the proposed material: %+v", body)
	}
	// The command carries the operator material: the pinned version, the
	// summary link, the proposed value (the RAW JSON bytes), the spans and
	// the mandatory reason — NEVER the outcome word (the matrix gate of
	// the domain decides it).
	if received.VersionNo != 1 || received.SummaryNo != 0 || received.Reason != "клиент назвал новый срок в ответе" {
		t.Fatalf("unexpected command material: %+v", received)
	}
	if len(received.Spans) != 1 || received.Spans[0] != "Срок доставки клиенту не назван." {
		t.Fatalf("unexpected spans material: %+v", received.Spans)
	}
	if received.IdempotencyKey != "case-key-1" {
		t.Fatalf("expected the idempotency key header, got %q", received.IdempotencyKey)
	}
	if !received.CorrelationID.Valid() {
		t.Fatal("expected the correlation id of the request")
	}
	if !received.CaseID.Valid() || !received.TenantID.Valid() || !received.Actor.ID.Valid() || received.FactID.String() != correctionFact {
		t.Fatal("expected the resolved actor, tenant, case and fact anchors")
	}
}

func TestRecordCorrectionReplayWireShapeMatchesContract(t *testing.T) {
	server := newCasesTestServer(t, &correctionCases{fakeCases: &fakeCases{}, record: func(context.Context, casesAPI.RecordCorrectionCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{Command: "record_correction", Replay: true, CorrectionOutcome: "disputed", CorrectionSpans: 0}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, correctionsPath, correctionRequestBody))
	if recorder.Code != http.StatusOK {
		t.Fatalf("a replay is 200 with the original outcome, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Idempotency-Replayed"); got != "true" {
		t.Fatalf("expected the replay header, got %q", got)
	}
}

func TestRecordCorrectionRefusalWireShapeByKind(t *testing.T) {
	refusals := []struct {
		name     string
		err      error
		expected int
		code     string
	}{
		{"rule", casesDomain.WorkRuleError{Rule: "record_correction", Reason: "выдержка №1 отсутствует в источнике байт-в-байт: выдержка обязана быть строкой события или сводки (анти-фантазийный зонд)"}, http.StatusBadRequest, "record_correction"},
		{"forbidden", domain.ErrForbidden, http.StatusForbidden, "cases_forbidden"},
		{"not_found", casesDomain.ErrNotFound, http.StatusNotFound, "not_found"},
		{"key_conflict", casesDomain.ErrCommandKeyConflict, http.StatusConflict, "command_key_conflict"},
		{"stale_version", casesDomain.ErrStaleVersion, http.StatusConflict, "stale_version"},
	}
	for _, refusal := range refusals {
		server := newCasesTestServer(t, &correctionCases{fakeCases: &fakeCases{}, record: func(context.Context, casesAPI.RecordCorrectionCommand) (casesAPI.WorkOutcome, error) {
			return casesAPI.WorkOutcome{}, refusal.err
		}})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, correctionsPath, correctionRequestBody))
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

func TestRecordCorrectionInputValidationRefusesHonestly(t *testing.T) {
	bodies := map[string]string{
		"missing version":  `{"value": 3, "reason": "причина"}`,
		"missing value":    `{"version_no": 1, "reason": "причина"}`,
		"missing reason":   `{"version_no": 1, "value": 3}`,
		"outcome field":    `{"version_no": 1, "value": 3, "reason": "причина", "outcome": "applied"}`,
		"correction_no":    `{"version_no": 1, "value": 3, "reason": "причина", "correction_no": 1}`,
		"negative summary": `{"version_no": 1, "summary_no": -1, "value": 3, "reason": "причина"}`,
	}
	for name, body := range bodies {
		server := newCasesTestServer(t, &correctionCases{fakeCases: &fakeCases{}})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, correctionsPath, body))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", name, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "invalid_request") {
			t.Fatalf("%s: expected the invalid_request problem code, got %s", name, recorder.Body.String())
		}
	}
}

// TestRecordCorrectionFormPRG pins the operator form path: the success is
// the 303 onto the #decision-context anchor with the flag of the honest
// status line (the section position survives, the scroll position does
// not — the no-JS boundary); the honest error flashes and returns to the
// SAME anchor; the form material (the pinned version, the value, the
// span, the summary link, the reason) travels the command contour whole.
func TestRecordCorrectionFormPRG(t *testing.T) {
	var received casesAPI.RecordCorrectionCommand
	server := newCasesTestServer(t, &correctionCases{fakeCases: &fakeCases{}, record: func(_ context.Context, command casesAPI.RecordCorrectionCommand) (casesAPI.WorkOutcome, error) {
		received = command
		return casesAPI.WorkOutcome{Command: "record_correction", CorrectionOutcome: "applied"}, nil
	}})
	form := url.Values{
		"version_no": {"1"},
		"value":      {`{"срок": 3}`},
		"span":       {"Срок доставки клиенту не назван."},
		"summary_no": {"0"},
		"reason":     {"клиент назвал новый срок"},
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, correctionFormRequest(http.MethodPost, correctionsFormPath, form))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("the success is the PRG 303, got %d: %s", recorder.Code, recorder.Body.String())
	}
	location := recorder.Header().Get("Location")
	if !strings.Contains(location, "/cases/"+decisionContextCase) || !strings.Contains(location, "correction_recorded=1") || !strings.Contains(location, "#decision-context") {
		t.Fatalf("the PRG lands on the #decision-context anchor with the success flag, got %q", location)
	}
	if received.VersionNo != 1 || received.SummaryNo != 0 || received.Reason != "клиент назвал новый срок" {
		t.Fatalf("unexpected form command material: %+v", received)
	}
	if len(received.Spans) != 1 || received.Spans[0] != "Срок доставки клиенту не назван." {
		t.Fatalf("the form span travels the verbatim excerpt, got %+v", received.Spans)
	}
	if !received.FactID.Valid() || received.IdempotencyKey == "" {
		t.Fatal("expected the fact anchor and the composed idempotency key of the form")
	}
	// The honest error path: the flash cookie with the rule words + the
	// PRG return to the SAME anchor.
	server = newCasesTestServer(t, &correctionCases{fakeCases: &fakeCases{}, record: func(context.Context, casesAPI.RecordCorrectionCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{}, domain.ErrForbidden
	}})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, correctionFormRequest(http.MethodPost, correctionsFormPath, form))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("the error path is the PRG 303, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); !strings.Contains(location, "#decision-context") {
		t.Fatalf("the error path returns to the #decision-context anchor, got %q", location)
	}
	if !strings.Contains(recorder.Header().Get("Set-Cookie"), flashCookieName) {
		t.Fatalf("the error path flashes the honest message")
	}
}

// correctionFormRequest builds one operator form request.
func correctionFormRequest(method, target string, form url.Values) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}
