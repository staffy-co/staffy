package openapi_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestWaitingUpdateContractIsHonest pins the Story 2.16 wire contract:
// the machine path of the operator update promise carries the principal
// boundary with the Idempotency-Key and the honest answer and refusal
// codes (the fresh record 201, the exact repeat 200, the dictionary and
// round refusals 400, the scope gate 403, the foreign tenant the plain
// 404, the raced promise number and the key conflict the honest 409);
// the request carries the deadline word ONLY — the honest unknown word
// included — and NEVER the server-side truth (the promised time, the
// promise number and the round anchor are refused by the strict decode;
// the store computes the time under the locks and derives the NEXT
// monotonic number); the waiting view of the case page documents the
// read-time client status dictionary with the THREE distinctions of
// AC-1 and the promise state dictionary of the last row (the block
// appears only with the material — the control branch); the stored
// outcome carries the promise id with the deadline word only (the
// promised time and the promise number NEVER enter).
func TestWaitingUpdateContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	schemas := mapping(t, mapping(t, document, "components"), "schemas")

	// The POST path: the principal boundary, the Idempotency-Key and the
	// honest answer and refusal codes (the waiting view travels the
	// CaseView read of the case page — NO new read path of the slice).
	writePath := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/update-promises")
	post := mapping(t, writePath, "post")
	if required, _ := post["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the update-promises POST does not document the principal boundary")
	}
	parameters, _ := post["parameters"].([]any)
	hasKey := false
	for _, parameter := range parameters {
		item, _ := parameter.(map[string]any)
		if item == nil {
			continue
		}
		if ref, _ := item["$ref"].(string); ref == "#/components/parameters/IdempotencyKey" {
			hasKey = true
		}
	}
	if !hasKey {
		t.Errorf("the update-promises POST is missing the Idempotency-Key parameter")
	}
	writeResponses := mapping(t, post, "responses")
	for _, code := range []string{"200", "201", "400", "401", "403", "404", "409"} {
		if _, documented := writeResponses[code]; !documented {
			t.Errorf("the update-promises POST does not document the honest %s answer", code)
		}
	}
	// The one write path only: the waiting state is READ through the
	// CaseView of the case page (the page is ONE resource — no new read
	// path of the slice).
	if _, hasRead := writePath["get"]; hasRead {
		t.Errorf("the update-promises path must NOT carry a GET: the waiting state reads through the case view")
	}

	// The operator request: the deadline word of the fixed dictionary
	// with the honest unknown word — and NO server-side truth fields
	// (the promised time and the promise number are the lock-visible
	// derivations; the round anchor is resolved by the store; a request
	// carrying any of them is the strict decode refusal).
	request := mapping(t, schemas, "PromiseUpdateRequest")
	requestProperties := mapping(t, request, "properties")
	if requestRequired, _ := request["required"].([]any); len(requestRequired) != 1 {
		t.Errorf("the promise update requires the deadline word only, got %v", requestRequired)
	}
	word, _ := requestProperties["deadline_word"].(map[string]any)
	wordEnum, _ := word["enum"].([]any)
	deadlineWords := map[string]bool{}
	for _, candidate := range wordEnum {
		if text, _ := candidate.(string); text == "update_15m" || text == "update_1h" || text == "update_4h" || text == "update_unknown" {
			deadlineWords[text] = true
		}
	}
	if len(deadlineWords) != 4 {
		t.Errorf("the deadline dictionary carries the four honest words (the material unknown included), got %v", wordEnum)
	}
	for _, forbidden := range []string{"promised_at", "promise_no", "request_no", "actor_id"} {
		if _, hasField := requestProperties[forbidden]; hasField {
			t.Errorf("PromiseUpdateRequest must NOT carry %s: the server-side truth of the locks and the store-resolved round, never a request field", forbidden)
		}
	}

	// The waiting view of the case page: the read-time client status
	// dictionary with the THREE distinctions of AC-1 (plus the honest
	// overdue and ending words) and the promise state dictionary of the
	// last row (pending/missed/unknown/superseded/closed — the
	// derivation-only supersede, the read-time missed without a
	// mutation).
	waiting := mapping(t, schemas, "WaitingUpdateView")
	waitingProperties := mapping(t, waiting, "properties")
	status, _ := waitingProperties["client_status"].(map[string]any)
	statusEnum, _ := status["enum"].([]any)
	statusWords := map[string]bool{}
	for _, candidate := range statusEnum {
		if text, _ := candidate.(string); text == "waiting_searching" || text == "waiting_assigned" || text == "waiting_accepted" {
			statusWords[text] = true
		}
	}
	if len(statusWords) != 3 {
		t.Errorf("the client waiting status carries the three distinctions of AC-1, got %v", statusEnum)
	}
	state, _ := waitingProperties["state"].(map[string]any)
	stateEnum, _ := state["enum"].([]any)
	stateWords := map[string]bool{}
	for _, candidate := range stateEnum {
		if text, _ := candidate.(string); text == "pending" || text == "missed" || text == "unknown" || text == "superseded" || text == "closed" {
			stateWords[text] = true
		}
	}
	if len(stateWords) != 5 {
		t.Errorf("the promise state dictionary carries the five honest read-time words, got %v", stateEnum)
	}
	for _, field := range []string{"has_promise", "client_status", "client_status_text", "promise_no", "deadline_word", "deadline_text", "promised_at", "state", "state_text", "promise_line_text", "missed_next_step_text", "deadline_words"} {
		if _, documented := waitingProperties[field]; !documented {
			t.Errorf("WaitingUpdateView documents the %s of the read-time waiting state", field)
		}
	}
	// The CaseView extension carries the waiting block (no new read
	// path — the case page is ONE resource).
	caseView := mapping(t, schemas, "CaseView")
	caseProperties := mapping(t, caseView, "properties")
	if _, documented := caseProperties["waiting"]; !documented {
		t.Errorf("CaseView documents the waiting block of Story 2.16")
	}

	// The stored outcome: the promise id with the deadline word (the
	// promised time, the promise number and the client texts NEVER
	// enter — the texts are the read-time render of the dictionary).
	outcome := mapping(t, schemas, "WorkOutcome")
	outcomeProperties := mapping(t, outcome, "properties")
	for _, field := range []string{"promise_id", "promise_deadline_word"} {
		if _, documented := outcomeProperties[field]; !documented {
			t.Errorf("WorkOutcome documents the %s of the Story 2.16 command", field)
		}
	}
	for _, forbidden := range []string{"promise_no", "promised_at", "promise_text", "promise_next_step"} {
		if _, hasField := outcomeProperties[forbidden]; hasField {
			t.Errorf("WorkOutcome must NOT carry %s: the stored outcome never carries the server number, the computed time or the client texts", forbidden)
		}
	}
}
