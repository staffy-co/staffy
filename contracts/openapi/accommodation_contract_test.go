package openapi_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestAccommodationContractIsHonest pins the Story 2.11 contract boundary:
// the record path carries the principal boundary and the honest refusal
// codes (the scope gate 403 BEFORE the guards, the dictionary 400, the
// replay 200, the raced revision and the key conflict 409); the key
// dictionary stays fixed (the eight Domain Pack v0 words — never a tenant
// configuration); the request NEVER carries the revision (the number is
// the server-side truth derived under the locks); the note stays bounded
// in the row and never enters the outbox; the outcome documents the
// record words; the reply documents the accuracy qualifier dictionary;
// the policy outcome documents the active alternatives of the honest
// machine offer; the case view carries the section.
func TestAccommodationContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	post := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/accommodations"), "post")
	if required, _ := post["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the accommodation path does not document the principal boundary")
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
		t.Errorf("the accommodation path is missing the Idempotency-Key parameter")
	}
	// The honest answer and refusal codes: the fresh record is the 201, the
	// idempotent replay of the exact repeat is the 200; the dictionary and
	// bound refusals are the 400; the scope gate is the 403 BEFORE the
	// guards; the foreign tenant is the plain 404; the raced revision and
	// the key conflict are the honest 409.
	responses := mapping(t, post, "responses")
	for _, code := range []string{"200", "201", "400", "401", "403", "404", "409"} {
		if _, documented := responses[code]; !documented {
			t.Errorf("the accommodation path does not document the honest %s answer", code)
		}
	}

	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	// The record request: the fixed eight-word key dictionary, the bounded
	// note — and NO revision field (the number is the server-side truth
	// under the locks, the request never carries it).
	request := mapping(t, schemas, "AccommodationRecordRequest")
	requestProperties := mapping(t, request, "properties")
	if _, hasRevision := requestProperties["revision"]; hasRevision {
		t.Errorf("AccommodationRecordRequest must NOT carry the revision: the number is derived server-side under the locks")
	}
	if requestRequired, _ := request["required"].([]any); len(requestRequired) != 1 || requestRequired[0] != "accommodation_key" {
		t.Errorf("the request requires exactly the accommodation_key, got %v", requestRequired)
	}
	keyEnum, _ := mapping(t, requestProperties, "accommodation_key")["enum"].([]any)
	if len(keyEnum) != 8 {
		t.Errorf("the key dictionary carries the eight Domain Pack v0 words, got %v", keyEnum)
	}
	keyWords := map[string]bool{}
	for _, word := range keyEnum {
		if text, _ := word.(string); text == "no_photo" || text == "no_web" || text == "no_voice" || text == "text_first" || text == "slowed_pace" || text == "mandatory_readback" || text == "preferred_channel" || text == "available_time" {
			keyWords[text] = true
		}
	}
	if len(keyWords) != 8 {
		t.Errorf("the eight key words are the fixed v0 dictionary, got %v", keyEnum)
	}
	if maxLength, _ := mapping(t, requestProperties, "note")["maxLength"].(int); maxLength != 500 {
		t.Errorf("the operator note stays bounded by 500 runes, got %v", maxLength)
	}
	if maxLength, _ := mapping(t, requestProperties, "value")["maxLength"].(int); maxLength != 100 {
		t.Errorf("the valued material stays bounded by 100 runes, got %v", maxLength)
	}

	// The view: the current set (the read-time derivation) and the honest
	// append-only history — the operational setting, never a fact.
	view := mapping(t, schemas, "AccommodationView")
	viewProperties := mapping(t, view, "properties")
	if _, hasCurrent := viewProperties["current"]; !hasCurrent {
		t.Errorf("AccommodationView documents the current set (the read-time derivation)")
	}
	if _, hasHistory := viewProperties["history"]; !hasHistory {
		t.Errorf("AccommodationView documents the append-only history including the clearings")
	}
	if _, hasAnchor := viewProperties["records_count"]; !hasAnchor {
		t.Errorf("AccommodationView documents the records_count anchor of the PRG double-submit protection")
	}

	// The case view carries the section.
	caseView := mapping(t, schemas, "CaseView")
	if _, hasSection := mapping(t, caseView, "properties")["accommodations"]; !hasSection {
		t.Errorf("CaseView documents the accommodations section of Story 2.11")
	}

	// The stored outcome: the record words of the command (the identifier,
	// the key and the clearing mark — the value and the note never enter).
	outcome := mapping(t, schemas, "WorkOutcome")
	outcomeProperties := mapping(t, outcome, "properties")
	for _, field := range []string{"accommodation_record_id", "accommodation_key", "accommodation_cleared"} {
		if _, documented := outcomeProperties[field]; !documented {
			t.Errorf("WorkOutcome documents the %s of the record command", field)
		}
	}
	if _, hasValue := outcomeProperties["accommodation_value"]; hasValue {
		t.Errorf("WorkOutcome must NOT carry the value: the material stays in the row, the outbox carries the metadata only")
	}
	if _, hasNote := outcomeProperties["accommodation_note"]; hasNote {
		t.Errorf("WorkOutcome must NOT carry the note: the material stays in the row, the outbox carries the metadata only")
	}
	commandEnum, _ := mapping(t, outcomeProperties, "command")["enum"].([]any)
	hasCommand := false
	for _, word := range commandEnum {
		if text, _ := word.(string); text == "record_accommodation" {
			hasCommand = true
		}
	}
	if !hasCommand {
		t.Errorf("the WorkOutcome command dictionary documents record_accommodation, got %v", commandEnum)
	}

	// The sales reply documents the accuracy qualifier dictionary (the
	// four honest words — the subset discipline is the gate's, the contract
	// pins the fixed dictionary).
	reply := mapping(t, schemas, "SendClientReplyRequest")
	replyProperties := mapping(t, reply, "properties")
	qualifier, _ := replyProperties["accuracy_qualifier"].(map[string]any)
	qualifierEnum, _ := qualifier["enum"].([]any)
	if len(qualifierEnum) != 4 {
		t.Errorf("the accuracy qualifier carries the four-word dictionary, got %v", qualifierEnum)
	}

	// The policy outcome documents the active alternatives of the honest
	// machine offer (the words of the fixed pack dictionary).
	policyOutcome := mapping(t, schemas, "PolicyOutcomeView")
	policyOutcomeProperties := mapping(t, policyOutcome, "properties")
	alternatives, _ := policyOutcomeProperties["active_alternatives"].(map[string]any)
	alternativeItems, _ := alternatives["items"].(map[string]any)
	alternativesEnum, _ := alternativeItems["enum"].([]any)
	alternativeWords := map[string]bool{}
	for _, word := range alternativesEnum {
		if text, _ := word.(string); text == "text_description" || text == "range_estimate" || text == "callback" || text == "human_handoff" {
			alternativeWords[text] = true
		}
	}
	if len(alternativeWords) != 4 {
		t.Errorf("the active alternatives carry the four honest words of the pack, got %v", alternativesEnum)
	}
}
