package openapi_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDegradationContractIsHonest pins the Story 2.12 wire contract: the
// two record paths carry the principal boundary and the Idempotency-Key
// with the honest answer and refusal codes (the fresh record 201, the
// exact repeat 200, the dictionary and bound refusals 400, the scope gate
// 403 BEFORE the guards, the foreign tenant the plain 404, the raced
// number and the key conflict the honest 409); the requests carry the
// fixed dictionaries WITHOUT the server-side numbers (the revision and
// the event number are derived under the locks — the requests never carry
// them); the views document the read-time derivations; the stored outcome
// carries the dictionary words only (the note and the content never
// enter); the reply request carries the promise word of the approved
// policy; the outcome view carries the permitted path words of the pack.
func TestDegradationContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")

	// The degradation record path: the principal boundary, the
	// Idempotency-Key and the honest codes.
	degradationPost := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/route-degradations"), "post")
	if required, _ := degradationPost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the degradation path does not document the principal boundary")
	}
	parameters, _ := degradationPost["parameters"].([]any)
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
		t.Errorf("the degradation path is missing the Idempotency-Key parameter")
	}
	responses := mapping(t, degradationPost, "responses")
	for _, code := range []string{"200", "201", "400", "401", "403", "404", "409"} {
		if _, documented := responses[code]; !documented {
			t.Errorf("the degradation path does not document the honest %s answer", code)
		}
	}

	// The delivery evidence path: the same boundary discipline.
	evidencePost := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/replies/{reply_id}/delivery-evidence"), "post")
	if required, _ := evidencePost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the evidence path does not document the principal boundary")
	}
	evidenceResponses := mapping(t, evidencePost, "responses")
	for _, code := range []string{"200", "201", "400", "401", "403", "404", "409"} {
		if _, documented := evidenceResponses[code]; !documented {
			t.Errorf("the evidence path does not document the honest %s answer", code)
		}
	}

	schemas := mapping(t, mapping(t, document, "components"), "schemas")

	// The degradation request: the route (the connection and the channel
	// word of the 2.1 dictionary), the cause word and the bounded note —
	// and NO revision field (the number is the server-side truth under the
	// locks).
	request := mapping(t, schemas, "RouteDegradationRecordRequest")
	requestProperties := mapping(t, request, "properties")
	if _, hasRevision := requestProperties["revision"]; hasRevision {
		t.Errorf("RouteDegradationRecordRequest must NOT carry the revision: the number is derived server-side under the locks")
	}
	if requestRequired, _ := request["required"].([]any); len(requestRequired) != 3 {
		t.Errorf("the request requires the route and the cause, got %v", requestRequired)
	}
	causeEnum, _ := mapping(t, requestProperties, "cause")["enum"].([]any)
	if len(causeEnum) != 5 {
		t.Errorf("the cause dictionary carries the five AC-1 words, got %v", causeEnum)
	}
	causeWords := map[string]bool{}
	for _, word := range causeEnum {
		if text, _ := word.(string); text == "timeout" || text == "disconnect" || text == "rate_limited" ||
			text == "capability_revoked" || text == "delivery_unknown" {
			causeWords[text] = true
		}
	}
	if len(causeWords) != 5 {
		t.Errorf("the five cause words are the fixed dictionary, got %v", causeEnum)
	}
	channelEnum, _ := mapping(t, requestProperties, "channel")["enum"].([]any)
	if len(channelEnum) != 9 {
		t.Errorf("the channel dictionary carries the nine 2.1 words, got %v", channelEnum)
	}
	if maxLength, _ := mapping(t, requestProperties, "note")["maxLength"].(int); maxLength != 500 {
		t.Errorf("the operator note stays bounded by 500 runes, got %v", maxLength)
	}

	// The evidence request: the kind word — and NO event_no field (the
	// number is the server-side truth under the locks).
	evidenceRequest := mapping(t, schemas, "DeliveryEvidenceRecordRequest")
	evidenceRequestProperties := mapping(t, evidenceRequest, "properties")
	if _, hasEventNo := evidenceRequestProperties["event_no"]; hasEventNo {
		t.Errorf("DeliveryEvidenceRecordRequest must NOT carry the event_no: the number is derived server-side under the locks")
	}
	if evidenceRequired, _ := evidenceRequest["required"].([]any); len(evidenceRequired) != 1 || evidenceRequired[0] != "kind" {
		t.Errorf("the request requires exactly the kind word, got %v", evidenceRequired)
	}
	kindEnum, _ := mapping(t, evidenceRequestProperties, "kind")["enum"].([]any)
	if len(kindEnum) != 3 {
		t.Errorf("the evidence dictionary carries the three observation words, got %v", kindEnum)
	}
	for _, word := range kindEnum {
		text, _ := word.(string)
		if text != "delivered_confirmed" && text != "delivery_unknown" && text != "provider_failed" {
			t.Errorf("the evidence dictionary carries the fixed words only, got %q", text)
		}
	}
	if _, hasContent := evidenceRequestProperties["content"]; hasContent {
		t.Errorf("DeliveryEvidenceRecordRequest must NOT carry the reply content: the material stays in the row")
	}

	// The views: the current slots (the read-time derivation), the honest
	// append-only history including the recoveries, and the derived
	// delivery states (dispatched without events is the UNKNOWN, never the
	// delivered).
	view := mapping(t, schemas, "RouteDegradationView")
	viewProperties := mapping(t, view, "properties")
	for _, field := range []string{"current", "history", "records_count"} {
		if _, documented := viewProperties[field]; !documented {
			t.Errorf("RouteDegradationView documents the %s of the degradation panel", field)
		}
	}
	delivery := mapping(t, schemas, "ReplyDeliveryView")
	stateEnum, _ := mapping(t, mapping(t, delivery, "properties"), "state")["enum"].([]any)
	stateWords := map[string]bool{}
	for _, word := range stateEnum {
		if text, _ := word.(string); text == "prepared" || text == "unknown" || text == "confirmed" || text == "failed" {
			stateWords[text] = true
		}
	}
	if len(stateWords) != 4 {
		t.Errorf("the delivery state dictionary carries the four derived words, got %v", stateEnum)
	}

	// The case view carries the section.
	caseView := mapping(t, schemas, "CaseView")
	if _, hasSection := mapping(t, caseView, "properties")["route_degradations"]; !hasSection {
		t.Errorf("CaseView documents the route_degradations section of Story 2.12")
	}

	// The stored outcome: the dictionary words of the two commands (the
	// record identifier, the cause and the clearing mark; the event
	// identifier with the kind word; the promise word of the reply) — the
	// note and the content NEVER enter.
	outcome := mapping(t, schemas, "WorkOutcome")
	outcomeProperties := mapping(t, outcome, "properties")
	for _, field := range []string{"degradation_record_id", "degradation_cause", "degradation_cleared", "delivery_evidence_id", "delivery_evidence_kind", "update_promise"} {
		if _, documented := outcomeProperties[field]; !documented {
			t.Errorf("WorkOutcome documents the %s of the Story 2.12 commands", field)
		}
	}
	if _, hasNote := outcomeProperties["degradation_note"]; hasNote {
		t.Errorf("WorkOutcome must NOT carry the degradation note: the material stays in the row")
	}

	// The reply request: the promise word of the approved policy (the four
	// dictionary words — a word outside the dictionary never leaves the
	// machine).
	replyRequest := mapping(t, mapping(t, schemas, "SendClientReplyRequest"), "properties")
	promiseProperty, _ := replyRequest["update_promise"].(map[string]any)
	if promiseProperty == nil {
		t.Fatalf("SendClientReplyRequest documents the update_promise word")
	}
	promiseEnum, _ := promiseProperty["enum"].([]any)
	promiseWords := map[string]bool{}
	for _, word := range promiseEnum {
		if text, _ := word.(string); text == "no_promise" || text == "within_1h" || text == "within_4h" || text == "within_next_business_day" {
			promiseWords[text] = true
		}
	}
	if len(promiseWords) != 4 {
		t.Errorf("the promise dictionary carries the four approved policy words, got %v", promiseEnum)
	}

	// The outcome view: the permitted path words of the degraded mode (the
	// five pack words AC-3 — the set never widens).
	outcomeView := mapping(t, schemas, "PolicyOutcomeView")
	pathsEnum, _ := mapping(t, mapping(t, outcomeView, "properties"), "degraded_permitted_paths")["items"].(map[string]any)
	permittedEnum, _ := pathsEnum["enum"].([]any)
	permittedWords := map[string]bool{}
	for _, word := range permittedEnum {
		if text, _ := word.(string); text == "accept_info_no_promise" || text == "verified_channel_switch" ||
			text == "human_handoff" || text == "agree_callback" || text == "stop_path" {
			permittedWords[text] = true
		}
	}
	if len(permittedWords) != 5 {
		t.Errorf("the permitted path dictionary carries the five AC-3 words, got %v", permittedEnum)
	}

	// The 2.12 dictionary words of the outcome view MUST stay documented on
	// every emission surface (the QA closure finding of step-05: the domain
	// and the table CHECK carried the new words while the wire enums lagged
	// behind — the drift is pinned here so it cannot recur).
	outcomeViewProperties := mapping(t, outcomeView, "properties")
	classEnum, _ := outcomeViewProperties["outcome_class"].(map[string]any)["enum"].([]any)
	if !enumHas(classEnum, "degraded_mode") {
		t.Errorf("PolicyOutcomeView.outcome_class documents the degraded_mode word of Story 2.12, got %v", classEnum)
	}
	reasonEnum, _ := outcomeViewProperties["reason"].(map[string]any)["enum"].([]any)
	if !enumHas(reasonEnum, "degraded_route") {
		t.Errorf("PolicyOutcomeView.reason documents the degraded_route word of Story 2.12, got %v", reasonEnum)
	}
	ruleVersionEnum, _ := outcomeViewProperties["rule_version"].(map[string]any)["enum"].([]any)
	for _, word := range []string{"intake-policy-v1", "intake-policy-v2", "intake-policy-v3"} {
		if !enumHas(ruleVersionEnum, word) {
			t.Errorf("PolicyOutcomeView.rule_version keeps every historical word readable, missing %q, got %v", word, ruleVersionEnum)
		}
	}

	// The WorkOutcome of the evaluate_policy command carries the class word
	// of the recorded outcome (degraded_mode included).
	outcomeClassEnum, _ := outcomeProperties["outcome_class"].(map[string]any)["enum"].([]any)
	if !enumHas(outcomeClassEnum, "degraded_mode") {
		t.Errorf("WorkOutcome.outcome_class documents the degraded_mode word of Story 2.12, got %v", outcomeClassEnum)
	}

	// The policy panel view: the class dictionary and the rule word of the
	// panel (the panel emits the domain constants — the class words carry
	// degraded_mode, the rule word travels with the current dictionary).
	policyView := mapping(t, schemas, "PolicyView")
	policyViewProperties := mapping(t, policyView, "properties")
	classWordsItems, _ := policyViewProperties["class_words"].(map[string]any)["items"].(map[string]any)
	classWordsEnum, _ := classWordsItems["enum"].([]any)
	if !enumHas(classWordsEnum, "degraded_mode") {
		t.Errorf("PolicyView.class_words documents the degraded_mode word of Story 2.12, got %v", classWordsEnum)
	}
	panelRuleEnum, _ := policyViewProperties["rule_version"].(map[string]any)["enum"].([]any)
	if !enumHas(panelRuleEnum, "intake-policy-v3") {
		t.Errorf("PolicyView.rule_version documents the intake-policy-v3 word of Story 2.12, got %v", panelRuleEnum)
	}

	// The input snapshot: the canonical degradations map of the fifth input
	// (route -> {cause, revision, cleared} — the honest supersede material
	// of the digest).
	snapshotView := mapping(t, schemas, "PolicyInputSnapshot")
	snapshotProperties := mapping(t, snapshotView, "properties")
	degradationsProperty, _ := snapshotProperties["degradations"].(map[string]any)
	if degradationsProperty == nil {
		t.Fatalf("PolicyInputSnapshot documents the degradations map of the fifth input")
	}
	degradationAdditional, _ := degradationsProperty["additionalProperties"].(map[string]any)
	if degradationAdditional == nil {
		t.Fatalf("PolicyInputSnapshot.degradations is the route-keyed map of the fifth input")
	}
	degradationCause, _ := mapping(t, degradationAdditional, "properties")["cause"].(map[string]any)
	snapshotCauseEnum, _ := degradationCause["enum"].([]any)
	if len(snapshotCauseEnum) != 5 {
		t.Errorf("the snapshot degradations cause dictionary carries the five AC-1 words, got %v", snapshotCauseEnum)
	}
	snapshotRuleEnum, _ := snapshotProperties["rule_version"].(map[string]any)["enum"].([]any)
	if !enumHas(snapshotRuleEnum, "intake-policy-v3") {
		t.Errorf("PolicyInputSnapshot.rule_version documents the intake-policy-v3 word of Story 2.12, got %v", snapshotRuleEnum)
	}
}

// enumHas reports whether a YAML enum array carries the word (the pinned
// dictionary word of the wire contract).
func enumHas(enum []any, word string) bool {
	for _, item := range enum {
		if text, _ := item.(string); text == word {
			return true
		}
	}
	return false
}
