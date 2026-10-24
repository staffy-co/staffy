package openapi_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestHandoffContractIsHonest pins the Story 2.13 wire contract: the six
// lifecycle paths carry the principal boundary and the Idempotency-Key
// with the honest answer and refusal codes (the fresh record 201, the
// exact repeat 200, the dictionary and state refusals 400, the scope
// gate 403 BEFORE the guards, the foreign tenant the plain 404, the
// one-competing-task conflict and the raced epochs the honest 409); the
// requests carry the fixed dictionaries WITHOUT the server-side numbers
// (the request and event numbers are derived under the locks — the
// requests never carry them) and WITHOUT the reason field (the ask is
// unconditional — the dictionary constant is the system truth) and
// WITHOUT the assignee role field (the server resolves it from the
// membership truth); the views document the read-time derivations (the
// state and the client status — назначение ≠ принятие are TWO words);
// the stored outcome carries the dictionary words only (the mode reason
// text NEVER enters).
func TestHandoffContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")

	// The six lifecycle paths: the principal boundary, the
	// Idempotency-Key and the honest codes.
	for _, path := range []string{
		"/v1/tenants/{tenant_id}/cases/{case_id}/human-requests",
		"/v1/tenants/{tenant_id}/cases/{case_id}/human-requests/assignment",
		"/v1/tenants/{tenant_id}/cases/{case_id}/human-requests/acknowledgement",
		"/v1/tenants/{tenant_id}/cases/{case_id}/human-requests/decline",
		"/v1/tenants/{tenant_id}/cases/{case_id}/human-requests/cancellation",
		"/v1/tenants/{tenant_id}/cases/{case_id}/human-requests/return",
	} {
		post := mapping(t, mapping(t, paths, path), "post")
		if required, _ := post["x-staffy-principal-required"].(bool); !required {
			t.Errorf("%s does not document the principal boundary", path)
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
			t.Errorf("%s is missing the Idempotency-Key parameter", path)
		}
		responses := mapping(t, post, "responses")
		for _, code := range []string{"200", "201", "400", "401", "403", "404", "409"} {
			if _, documented := responses[code]; !documented {
				t.Errorf("%s does not document the honest %s answer", path, code)
			}
		}
	}

	schemas := mapping(t, mapping(t, document, "components"), "schemas")

	// The unconditional ask request: the anchor turn, the CAS expectation,
	// the target roles and the deadline word — the reason of the
	// unconditional ask is NEVER typed (client_request stays the system
	// truth — the Story 4.3 extension admits the three operator boundary
	// words of the part request panel, the Story 4.4 extension adds the
	// fitment research word of the applicability panel, all OPTIONAL,
	// never the client word) and NO request_no field (the number is the
	// server-side truth under the locks).
	request := mapping(t, schemas, "RequestHumanRequest")
	requestProperties := mapping(t, request, "properties")
	if reason, hasReason := requestProperties["reason"].(map[string]any); hasReason {
		reasonEnum, _ := reason["enum"].([]any)
		reasonWords := map[string]bool{}
		for _, word := range reasonEnum {
			if text, _ := word.(string); text == "diagnostic_uncertainty" || text == "safety_critical" || text == "unresolved_contradiction" || text == "fitment_research" || text == "offers_research" {
				reasonWords[text] = true
			} else {
				t.Errorf("the reason enum carries an out-of-dictionary word %q", text)
			}
		}
		if len(reasonWords) != 5 {
			t.Errorf("the reason enum carries the three Story 4.3 operator boundary words plus the Story 4.4 and 4.5 research words (the 00019 + cases/00024 + 00021 mirrors), got %v", reasonEnum)
		}
	} else {
		t.Errorf("RequestHumanRequest must carry the OPTIONAL boundary reason (the three Story 4.3 operator words plus the Story 4.4 research word)")
	}
	requestRequired, _ := request["required"].([]any)
	if len(requestRequired) != 4 {
		t.Errorf("the ask requires the turn, the CAS expectation, the roles and the deadline (the reason stays OPTIONAL), got %v", requestRequired)
	}
	for _, requiredWord := range requestRequired {
		if text, _ := requiredWord.(string); text == "reason" {
			t.Errorf("the reason is OPTIONAL — the empty word stays the unconditional client ask")
		}
	}
	if _, hasRequestNo := requestProperties["request_no"]; hasRequestNo {
		t.Errorf("RequestHumanRequest must NOT carry the request_no: the number is derived server-side under the locks")
	}
	rolesItems, _ := mapping(t, requestProperties, "target_roles")["items"].(map[string]any)
	rolesEnum, _ := rolesItems["enum"].([]any)
	roleWords := map[string]bool{}
	for _, word := range rolesEnum {
		if text, _ := word.(string); text == "operator" || text == "supervision" {
			roleWords[text] = true
		}
	}
	if len(roleWords) != 2 {
		t.Errorf("the target role dictionary carries the two AC-3 words, got %v", rolesEnum)
	}
	deadlineEnum, _ := mapping(t, requestProperties, "ack_deadline")["enum"].([]any)
	deadlineWords := map[string]bool{}
	for _, word := range deadlineEnum {
		if text, _ := word.(string); text == "ack_15m" || text == "ack_1h" || text == "ack_4h" {
			deadlineWords[text] = true
		}
	}
	if len(deadlineWords) != 3 {
		t.Errorf("the ack-deadline dictionary carries the three approved policy words, got %v", deadlineEnum)
	}

	// The assignment request: the assignee, the backup and the deadline
	// word — and NO role field (the server resolves the word from the
	// membership truth — the operator never inputs it) and NO event_no
	// field (the number is the server-side truth under the locks).
	assignment := mapping(t, schemas, "AssignHumanHandoffRequest")
	assignmentProperties := mapping(t, assignment, "properties")
	if _, hasRole := assignmentProperties["role"]; hasRole {
		t.Errorf("AssignHumanHandoffRequest must NOT carry the assignee role: the server resolves it from the membership truth")
	}
	if _, hasEventNo := assignmentProperties["event_no"]; hasEventNo {
		t.Errorf("AssignHumanHandoffRequest must NOT carry the event_no: the number is derived server-side under the locks")
	}
	if assignmentRequired, _ := assignment["required"].([]any); len(assignmentRequired) != 3 {
		t.Errorf("the assignment requires the assignee, the backup and the deadline, got %v", assignmentRequired)
	}

	// The acknowledgement request: the CAS epoch expectation ONLY (the
	// actor is the assignee — the server truth, never an input).
	acknowledgement := mapping(t, schemas, "AcknowledgeHumanHandoffRequest")
	if acknowledgementRequired, _ := acknowledgement["required"].([]any); len(acknowledgementRequired) != 1 || acknowledgementRequired[0] != "expected_epoch" {
		t.Errorf("the acknowledgement requires exactly the CAS epoch expectation, got %v", acknowledgementRequired)
	}

	// The cancellation request: the anchor turn ONLY.
	cancellation := mapping(t, schemas, "CancelHumanRequestRequest")
	if cancellationRequired, _ := cancellation["required"].([]any); len(cancellationRequired) != 1 || cancellationRequired[0] != "turn_ref" {
		t.Errorf("the cancellation requires exactly the anchor turn, got %v", cancellationRequired)
	}

	// The return request: the result word of the fixed dictionary and the
	// CAS epoch expectation.
	returnRequest := mapping(t, schemas, "ReturnToStaffyRequest")
	resultEnum, _ := mapping(t, mapping(t, returnRequest, "properties"), "result")["enum"].([]any)
	resultWords := map[string]bool{}
	for _, word := range resultEnum {
		if text, _ := word.(string); text == "issue_resolved" || text == "handed_back" {
			resultWords[text] = true
		}
	}
	if len(resultWords) != 2 {
		t.Errorf("the return result dictionary carries the two AC-9 words, got %v", resultEnum)
	}

	// The view: the requests with the read-time derivations, the card flag
	// of the assignee and the dictionaries of the forms.
	view := mapping(t, schemas, "HandoffView")
	viewProperties := mapping(t, view, "properties")
	for _, field := range []string{"requests", "requests_count", "acknowledge_for_actor", "role_words", "deadline_words", "result_words"} {
		if _, documented := viewProperties[field]; !documented {
			t.Errorf("HandoffView documents the %s of the human handoff panel", field)
		}
	}
	// The client status dictionary: назначение ≠ принятие are TWO words
	// (human_assigned and human_acknowledged are separate — the interface
	// never claims the specialist is connected before the confirmed
	// acceptance).
	clientStatusEnum, _ := viewProperties["client_status"].(map[string]any)["enum"].([]any)
	clientWords := map[string]bool{}
	for _, word := range clientStatusEnum {
		if text, _ := word.(string); text == "human_requested" || text == "human_assigned" || text == "human_acknowledged" || text == "human_cancelled" {
			clientWords[text] = true
		}
	}
	if len(clientWords) != 4 {
		t.Errorf("the client status dictionary carries the four honest words (назначение ≠ принятие), got %v", clientStatusEnum)
	}

	// The event view: the five lifecycle kinds with the honest texts.
	eventView := mapping(t, schemas, "HandoffEventView")
	kindEnum, _ := mapping(t, mapping(t, eventView, "properties"), "kind")["enum"].([]any)
	kindWords := map[string]bool{}
	for _, word := range kindEnum {
		if text, _ := word.(string); text == "assigned" || text == "acknowledged" || text == "declined" || text == "cancelled" || text == "returned" {
			kindWords[text] = true
		}
	}
	if len(kindWords) != 5 {
		t.Errorf("the lifecycle event dictionary carries the five transition words, got %v", kindEnum)
	}

	// The case view carries the section.
	caseView := mapping(t, schemas, "CaseView")
	if _, hasSection := mapping(t, caseView, "properties")["handoff"]; !hasSection {
		t.Errorf("CaseView documents the handoff section of Story 2.13")
	}

	// The stored outcome: the dictionary words of the six commands (the
	// anchor turn, the assignee and the backup of the round, the deadline
	// and the result words) — the mode reason text NEVER enters.
	outcome := mapping(t, schemas, "WorkOutcome")
	outcomeProperties := mapping(t, outcome, "properties")
	for _, field := range []string{"handoff_turn_ref", "handoff_assignee", "handoff_backup", "handoff_ack_deadline", "handoff_result"} {
		if _, documented := outcomeProperties[field]; !documented {
			t.Errorf("WorkOutcome documents the %s of the Story 2.13 commands", field)
		}
	}
	if _, hasReason := outcomeProperties["handoff_reason"]; hasReason {
		t.Errorf("WorkOutcome must NOT carry a handoff reason text: the mode reason never enters the stored outcome")
	}
	if _, hasRequestNo := outcomeProperties["handoff_request_no"]; hasRequestNo {
		t.Errorf("WorkOutcome must NOT carry the request_no: the numbers are the server-side truth, the stored outcome never carries them")
	}
}
