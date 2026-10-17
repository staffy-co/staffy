package openapi_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDecisionContextContractIsHonest pins the Story 2.14 wire contract:
// the two machine paths carry the principal boundary (the GET read with
// the review-or-configuration gate BEFORE the guards — an audit-only
// member gets the honest 403; the POST record with the
// Idempotency-Key); the honest answer and refusal codes (the fresh
// record 201, the exact repeat 200, the dictionary and bound refusals
// 400, the scope gate 403, the foreign tenant the plain 404, the raced
// summary number and the key conflict the honest 409); the request
// carries the FIXED dictionaries WITHOUT the server-side numbers (the
// summary number and the watermark are derived under the locks — the
// request NEVER carries them) and WITHOUT an operator authoring surface
// (the kind dictionary carries the single producer word; the PRG forms
// do not exist); the views document the read-time derivations (the
// staleness word with the honest counters, the three SEPARATE fact
// groups, the recommendation projection); CaseView is NOT extended —
// the context is a separate resource; the stored outcome carries the
// dictionary words and the claim counter only (the summary text and
// the claims NEVER enter).
func TestDecisionContextContractIsHonest(t *testing.T) {
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

	// The GET path: the principal boundary and the honest read codes
	// (200/401/403/404 — the read never fails with a rule 400).
	readPath := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/decision-context")
	get := mapping(t, readPath, "get")
	if required, _ := get["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the decision-context GET does not document the principal boundary")
	}
	readResponses := mapping(t, get, "responses")
	for _, code := range []string{"200", "401", "403", "404"} {
		if _, documented := readResponses[code]; !documented {
			t.Errorf("the decision-context GET does not document the honest %s answer", code)
		}
	}
	if _, documented := readResponses["400"]; documented {
		t.Errorf("the decision-context GET must not document a 400: the read carries no material to refuse")
	}

	// The POST path: the principal boundary, the Idempotency-Key and the
	// honest answer and refusal codes.
	writePath := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/summaries")
	post := mapping(t, writePath, "post")
	if required, _ := post["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the summaries POST does not document the principal boundary")
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
		t.Errorf("the summaries POST is missing the Idempotency-Key parameter")
	}
	writeResponses := mapping(t, post, "responses")
	for _, code := range []string{"200", "201", "400", "401", "403", "404", "409"} {
		if _, documented := writeResponses[code]; !documented {
			t.Errorf("the summaries POST does not document the honest %s answer", code)
		}
	}

	// The producer request: the kind word of the single-word dictionary,
	// the text with the claims (1..32, the claim discipline) — and NO
	// server-side truth fields (the summary number and the watermark are
	// derived under the locks; the request NEVER carries them).
	request := mapping(t, schemas, "RecordCaseSummaryRequest")
	requestProperties := mapping(t, request, "properties")
	if requestRequired, _ := request["required"].([]any); len(requestRequired) != 3 {
		t.Errorf("the summary record requires the kind, the text and the claims, got %v", requestRequired)
	}
	kindEnum, _ := requestProperties["kind"].(map[string]any)["enum"].([]any)
	if len(kindEnum) != 1 || kindEnum[0] != "ai_summary" {
		t.Errorf("the kind dictionary carries the single producer word (operator summaries are Never), got %v", kindEnum)
	}
	claims, _ := requestProperties["claims"].(map[string]any)
	if minItems, _ := claims["minItems"].(int); minItems != 1 {
		t.Errorf("the claims are required: minItems 1, got %v", claims["minItems"])
	}
	if maxItems, _ := claims["maxItems"].(int); maxItems != 32 {
		t.Errorf("the claims carry the material bound: maxItems 32, got %v", claims["maxItems"])
	}
	for _, forbidden := range []string{"summary_no", "event_count", "fact_version_count", "owner_epoch", "mode_version"} {
		if _, hasField := requestProperties[forbidden]; hasField {
			t.Errorf("RecordCaseSummaryRequest must NOT carry %s: the server-side truth under the locks, never a request field", forbidden)
		}
	}

	// The claim view: the ref-type dictionary with the disputed mark.
	claimView := mapping(t, schemas, "ClaimView")
	claimRefEnum, _ := mapping(t, claimView, "properties")["ref_type"].(map[string]any)["enum"].([]any)
	refWords := map[string]bool{}
	for _, word := range claimRefEnum {
		if text, _ := word.(string); text == "fact_version" || text == "observation" {
			refWords[text] = true
		}
	}
	if len(refWords) != 2 {
		t.Errorf("the claim ref-type dictionary carries the two material words, got %v", claimRefEnum)
	}

	// The summary view: the staleness dictionary with the honest counters
	// (the word is a read-time derivation — never a stored status).
	summaryView := mapping(t, schemas, "SummaryView")
	summaryProperties := mapping(t, summaryView, "properties")
	stalenessEnum, _ := summaryProperties["staleness"].(map[string]any)["enum"].([]any)
	stalenessWords := map[string]bool{}
	for _, word := range stalenessEnum {
		if text, _ := word.(string); text == "actual" || text == "stale" {
			stalenessWords[text] = true
		}
	}
	if len(stalenessWords) != 2 {
		t.Errorf("the staleness dictionary carries the two honest words, got %v", stalenessEnum)
	}
	for _, field := range []string{"marking_text", "staleness_counters_text"} {
		if _, documented := summaryProperties[field]; !documented {
			t.Errorf("SummaryView documents the %s of the honest AI marking", field)
		}
	}

	// The context view: the twelve blocks with the three SEPARATE fact
	// groups and the recommendation projection.
	view := mapping(t, schemas, "DecisionContextView")
	viewProperties := mapping(t, view, "properties")
	for _, field := range []string{
		"present", "handoff", "work_items", "mode", "assurance", "owner", "facts",
		"observations", "deliveries", "commitments", "preferences", "recommendation", "summary",
	} {
		if _, documented := viewProperties[field]; !documented {
			t.Errorf("DecisionContextView documents the %s block of the operator section", field)
		}
	}
	facts, _ := viewProperties["facts"].(map[string]any)
	factRequired, _ := facts["required"].([]any)
	if len(factRequired) != 3 {
		t.Errorf("the facts carry the three SEPARATE groups (never one mixed list), got %v", factRequired)
	}
	factRow := mapping(t, schemas, "DecisionContextFactRow")
	factStatusEnum, _ := mapping(t, factRow, "properties")["status"].(map[string]any)["enum"].([]any)
	statusWords := map[string]bool{}
	for _, word := range factStatusEnum {
		if text, _ := word.(string); text == "confirmed" || text == "unconfirmed" || text == "conflicting" {
			statusWords[text] = true
		}
	}
	if len(statusWords) != 3 {
		t.Errorf("the context fact-status dictionary carries the three render words (unverified renders as unconfirmed), got %v", factStatusEnum)
	}

	// CaseView is NOT extended: the context is a separate resource (the
	// probe of the honest boundary — the page composes the section of the
	// same read, the CaseView DTO stays untouched).
	caseView := mapping(t, schemas, "CaseView")
	if _, hasSection := mapping(t, caseView, "properties")["decision_context"]; hasSection {
		t.Errorf("CaseView must NOT carry the decision_context section: the context is a separate resource of the 2.14 read")
	}

	// The stored outcome: the kind word with the claim counter (the
	// summary text and the claims NEVER enter; the number and the
	// watermark are the server-side truth).
	outcome := mapping(t, schemas, "WorkOutcome")
	outcomeProperties := mapping(t, outcome, "properties")
	for _, field := range []string{"summary_kind", "summary_claims"} {
		if _, documented := outcomeProperties[field]; !documented {
			t.Errorf("WorkOutcome documents the %s of the Story 2.14 command", field)
		}
	}
	for _, forbidden := range []string{"summary_no", "summary_text", "summary_watermark"} {
		if _, hasField := outcomeProperties[forbidden]; hasField {
			t.Errorf("WorkOutcome must NOT carry %s: the stored outcome never carries the server numbers or the summary material", forbidden)
		}
	}
}
