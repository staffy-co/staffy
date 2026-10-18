package httptransport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.16 transport fakes: the promise surface is pinned by the
// wire-shape tests (the command semantics live in the integration and
// contract contours — the correction precedent). The strict decode
// probe is the SERVER-truth refusal: a request carrying the promised
// time, the promise number or the round anchor refuses before the
// command (the store computes the time under the locks and derives the
// NEXT monotonic number — the honest unknown word never carries a
// fabricated timestamp).

const updatePromisesPath = decisionContextPath + "/update-promises"
const updatePromisesFormPath = "/app/tenants/" + decisionContextTenant + "/cases/" + decisionContextCase + "/update-promises"

// waitingUpdateCases wraps the transport fake with the narrow interface
// of Story 2.16 (the fakeCases stubs stay untouched).
type waitingUpdateCases struct {
	*fakeCases
	promise func(context.Context, casesAPI.PromiseUpdateCommand) (casesAPI.WorkOutcome, error)
}

func (f *waitingUpdateCases) PromiseUpdate(ctx context.Context, command casesAPI.PromiseUpdateCommand) (casesAPI.WorkOutcome, error) {
	if f.promise == nil {
		return casesAPI.WorkOutcome{}, nil
	}
	return f.promise(ctx, command)
}

const promiseRequestBody = `{"deadline_word": "update_1h"}`

func TestPromiseUpdateWireShapeMatchesContract(t *testing.T) {
	var received casesAPI.PromiseUpdateCommand
	server := newCasesTestServer(t, &waitingUpdateCases{fakeCases: &fakeCases{}, promise: func(_ context.Context, command casesAPI.PromiseUpdateCommand) (casesAPI.WorkOutcome, error) {
		received = command
		return casesAPI.WorkOutcome{Command: "promise_handoff_update", PromiseID: id.MustParse("01994f64-6c00-7000-8000-000000000cd9"), PromiseDeadlineWord: "update_1h"}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, updatePromisesPath, promiseRequestBody))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Idempotency-Replayed"); got != "" {
		t.Fatalf("a fresh record is not a replay, header %q", got)
	}
	var body struct {
		Command             string    `json:"command"`
		PromiseID           id.UUID   `json:"promise_id"`
		PromiseDeadlineWord string    `json:"promise_deadline_word"`
		PromiseNo           *int64    `json:"promise_no"`
		PromisedAt          *struct{} `json:"promised_at"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Command != "promise_handoff_update" || body.PromiseDeadlineWord != "update_1h" || !body.PromiseID.Valid() {
		t.Fatalf("unexpected outcome body: %+v", body)
	}
	// The stored outcome never carries the server truths: the promise
	// number and the computed promised time NEVER enter.
	if body.PromiseNo != nil || body.PromisedAt != nil {
		t.Fatalf("the outcome must NOT carry the promise number or the computed time: %+v", body)
	}
	// The command carries the ONLY request material: the deadline word
	// of the fixed dictionary — NEVER the promised time, the promise
	// number or the round anchor.
	if received.DeadlineWord != "update_1h" {
		t.Fatalf("unexpected command material: %+v", received)
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

func TestPromiseUpdateReplayWireShapeMatchesContract(t *testing.T) {
	server := newCasesTestServer(t, &waitingUpdateCases{fakeCases: &fakeCases{}, promise: func(context.Context, casesAPI.PromiseUpdateCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{Command: "promise_handoff_update", Replay: true, PromiseID: id.MustParse("01994f64-6c00-7000-8000-000000000cd9"), PromiseDeadlineWord: "update_unknown"}, nil
	}})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, updatePromisesPath, promiseRequestBody))
	if recorder.Code != http.StatusOK {
		t.Fatalf("a replay is 200 with the original outcome, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Idempotency-Replayed"); got != "true" {
		t.Fatalf("expected the replay header, got %q", got)
	}
}

func TestPromiseUpdateRefusalWireShapeByKind(t *testing.T) {
	refusals := []struct {
		name     string
		err      error
		expected int
		code     string
	}{
		{"rule", casesDomain.WorkRuleError{Rule: "promise_handoff_update", Reason: "слово дедлайна обновления «update_2h» не входит в словарь («update_15m», «update_1h», «update_4h», «update_unknown»)"}, http.StatusBadRequest, "promise_handoff_update"},
		{"round refused", casesDomain.WorkRuleError{Rule: "promise_handoff_update", Reason: "Обещание обновления якорится открытым ожиданием клиента: раунд в состоянии «declined» не принимает обещаний — переназначение и закрытие остаются решением человека."}, http.StatusBadRequest, "promise_handoff_update"},
		{"forbidden", domain.ErrForbidden, http.StatusForbidden, "cases_forbidden"},
		{"not_found", casesDomain.ErrNotFound, http.StatusNotFound, "not_found"},
		{"key_conflict", casesDomain.ErrCommandKeyConflict, http.StatusConflict, "command_key_conflict"},
		{"stale_version", casesDomain.ErrStaleVersion, http.StatusConflict, "stale_version"},
	}
	for _, refusal := range refusals {
		server := newCasesTestServer(t, &waitingUpdateCases{fakeCases: &fakeCases{}, promise: func(context.Context, casesAPI.PromiseUpdateCommand) (casesAPI.WorkOutcome, error) {
			return casesAPI.WorkOutcome{}, refusal.err
		}})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, updatePromisesPath, promiseRequestBody))
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

func TestPromiseUpdateInputValidationRefusesHonestly(t *testing.T) {
	bodies := map[string]string{
		"missing word": `{}`,
		"empty word":   `{"deadline_word": ""}`,
		"promised_at":  `{"deadline_word": "update_1h", "promised_at": "2026-10-18T12:00:00Z"}`,
		"promise_no":   `{"deadline_word": "update_1h", "promise_no": 1}`,
		"request_no":   `{"deadline_word": "update_1h", "request_no": 1}`,
		"actor field":  `{"deadline_word": "update_1h", "actor_id": "01994f64-6c00-7000-8000-0000000000ff"}`,
	}
	for name, body := range bodies {
		server := newCasesTestServer(t, &waitingUpdateCases{fakeCases: &fakeCases{}})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, updatePromisesPath, body))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", name, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "invalid_request") {
			t.Fatalf("%s: expected the invalid_request problem code, got %s", name, recorder.Body.String())
		}
	}
}

// TestPromiseUpdateFormPRG pins the operator form path: the success is
// the 303 onto the #handoff anchor with the flag of the honest status
// line (the section position survives, the scroll position does not —
// the no-JS boundary); the honest error flashes and returns to the
// SAME anchor; the form material (the deadline word with the
// double-submit anchor) travels the command contour whole.
func TestPromiseUpdateFormPRG(t *testing.T) {
	var received casesAPI.PromiseUpdateCommand
	server := newCasesTestServer(t, &waitingUpdateCases{fakeCases: &fakeCases{}, promise: func(_ context.Context, command casesAPI.PromiseUpdateCommand) (casesAPI.WorkOutcome, error) {
		received = command
		return casesAPI.WorkOutcome{Command: "promise_handoff_update", PromiseID: id.MustParse("01994f64-6c00-7000-8000-000000000cd9"), PromiseDeadlineWord: "update_unknown"}, nil
	}})
	form := url.Values{
		"deadline_word":            {"update_unknown"},
		"expected_update_promises": {"0"},
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, correctionFormRequest(http.MethodPost, updatePromisesFormPath, form))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("the success is the PRG 303, got %d: %s", recorder.Code, recorder.Body.String())
	}
	location := recorder.Header().Get("Location")
	if !strings.Contains(location, "/cases/"+decisionContextCase) || !strings.Contains(location, "update_promised=1") || !strings.Contains(location, "#handoff") {
		t.Fatalf("the PRG lands on the #handoff anchor with the success flag, got %q", location)
	}
	if received.DeadlineWord != "update_unknown" {
		t.Fatalf("the form carries the deadline word of the dictionary, got %+v", received)
	}
	if received.IdempotencyKey == "" {
		t.Fatal("expected the composed idempotency key of the form")
	}
	// The honest error path: the flash cookie with the rule words + the
	// PRG return to the SAME anchor.
	server = newCasesTestServer(t, &waitingUpdateCases{fakeCases: &fakeCases{}, promise: func(context.Context, casesAPI.PromiseUpdateCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{}, domain.ErrForbidden
	}})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, correctionFormRequest(http.MethodPost, updatePromisesFormPath, form))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("the error path is the PRG 303, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); !strings.Contains(location, "#handoff") {
		t.Fatalf("the error path returns to the #handoff anchor, got %q", location)
	}
	if !strings.Contains(recorder.Header().Get("Set-Cookie"), flashCookieName) {
		t.Fatalf("the error path flashes the honest message")
	}
	// The missing word of the form: the honest refusal BEFORE the
	// command (the form guard of the transport).
	server = newCasesTestServer(t, &waitingUpdateCases{fakeCases: &fakeCases{}})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, correctionFormRequest(http.MethodPost, updatePromisesFormPath, url.Values{}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("the missing word path is the PRG 303, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Header().Get("Set-Cookie"), flashCookieName) {
		t.Fatalf("the missing word path flashes the honest message")
	}
}
