package openapi_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestContinuationContractIsHonest pins the Story 2.10 contract boundary:
// the two operator paths carry the principal boundary and the honest
// refusal codes (the scope gate 403 BEFORE the guards, the dictionary
// 400, the key conflict 409); the dictionaries stay fixed (the purposes
// v1, the use kinds, the state words — never a tenant configuration);
// the issuance answer carries the one-time url/token of the FRESH answer
// while the stored outcome NEVER carries the token; the sales reply
// documents the link_id delivery binding; the case view carries the
// section; the public HTML routes stay OUT of the JSON contract (the
// client of the protected page is a human, not an API consumer).
func TestContinuationContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	for _, path := range []string{
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/continuation-links",
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/continuation-links/{link_id}/revoke",
	} {
		post := mapping(t, mapping(t, paths, path), "post")
		if required, _ := post["x-staffy-principal-required"].(bool); !required {
			t.Errorf("%s: the principal boundary is not documented", path)
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
			t.Errorf("%s: the Idempotency-Key parameter is missing", path)
		}
		// The honest refusal codes: the dictionary and bound refusals are
		// 400; the scope gate is the 403 BEFORE the guards; the key
		// conflict is the honest 409.
		responses := mapping(t, post, "responses")
		for _, code := range []string{"400", "403", "409"} {
			if _, documented := responses[code]; !documented {
				t.Errorf("%s: the honest %s refusal is not documented", path, code)
			}
		}
	}
	// The public protected routes stay OUT of the machine contract (the
	// honest boundary of the slice: the client is a human).
	for _, path := range paths {
		if name, _ := path.(string); strings.HasPrefix(name, "/continue") {
			t.Errorf("the public HTML route %s must not enter the JSON contract", name)
		}
	}

	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	// The issue request: the fixed purpose dictionary, the short ttl
	// bounds, the bounded consumption limit — and NO token field (the
	// capability is generated server-side, never a request material).
	issue := mapping(t, schemas, "ContinuationLinkIssueRequest")
	issueProperties := mapping(t, issue, "properties")
	if _, hasToken := issueProperties["token"]; hasToken {
		t.Errorf("ContinuationLinkIssueRequest must NOT carry the token: the capability is generated server-side")
	}
	purposes, _ := issueProperties["purposes"].(map[string]any)
	if purposes != nil {
		items, _ := purposes["items"].(map[string]any)
		if items != nil {
			if purposeEnum, _ := items["enum"].([]any); len(purposeEnum) != 1 || purposeEnum[0] != "fact_correction" {
				t.Errorf("the purposes carry the one-word v1 dictionary, got %v", purposeEnum)
			}
		}
		if minItems, _ := purposes["minItems"].(int); minItems != 1 {
			t.Errorf("the purpose set is never empty, got %v", purposes["minItems"])
		}
	}
	ttl := mapping(t, issueProperties, "ttl_minutes")
	if min, _ := ttl["minimum"].(int); min != 5 {
		t.Errorf("the ttl stays bounded below by five minutes, got %v", min)
	}
	if max, _ := ttl["maximum"].(int); max != 120 {
		t.Errorf("the ttl stays bounded above by two hours, got %v", max)
	}
	limit := mapping(t, issueProperties, "consumption_limit")
	if max, _ := limit["maximum"].(int); max != 10 {
		t.Errorf("the consumption limit stays bounded by ten, got %v", max)
	}
	if one, _ := limit["default"].(int); one != 1 {
		t.Errorf("the default consumption limit is one (the FR-16 exactly-one submission), got %v", one)
	}

	// The revoke request: the bounded honest reason.
	revoke := mapping(t, schemas, "ContinuationLinkRevokeRequest")
	if maxLength, _ := mapping(t, mapping(t, revoke, "properties"), "reason")["maxLength"].(int); maxLength != 200 {
		t.Errorf("the revoke reason stays bounded by 200, got %v", maxLength)
	}

	// The issuance answer: the url/token exist ONLY in the fresh answer;
	// the stored outcome (WorkOutcome) NEVER carries them (the mask word
	// only — the preimage is not recoverable from the hash).
	issuance := mapping(t, schemas, "ContinuationIssuance")
	issuanceRequired, _ := issuance["required"].([]any)
	if len(issuanceRequired) != 1 || issuanceRequired[0] != "outcome" {
		t.Errorf("ContinuationIssuance requires only the stored outcome (url/token/expires are the fresh-answer extras), got %v", issuanceRequired)
	}
	outcome := mapping(t, schemas, "WorkOutcome")
	outcomeProperties := mapping(t, outcome, "properties")
	if _, hasURL := outcomeProperties["url"]; hasURL {
		t.Errorf("WorkOutcome must NOT carry the url: the stored outcome never reconstructs the capability")
	}
	if _, hasToken := outcomeProperties["token"]; hasToken {
		t.Errorf("WorkOutcome must NOT carry the token: the stored outcome never reconstructs the capability")
	}
	commandProp := mapping(t, outcomeProperties, "command")
	commandEnum, _ := commandProp["enum"].([]any)
	words := map[string]bool{}
	for _, word := range commandEnum {
		if text, _ := word.(string); text == "issue_continuation_link" || text == "revoke_continuation_link" {
			words[text] = true
		}
	}
	if !words["issue_continuation_link"] || !words["revoke_continuation_link"] {
		t.Errorf("the WorkOutcome command words of the two continuation commands are documented, got %v", commandEnum)
	}

	// The view: the derived state dictionary (the reserved unknown word
	// never carries a producer of the derivation — the honest
	// reservation), the delivered binding and the event kinds.
	view := mapping(t, schemas, "ContinuationView")
	viewProperties := mapping(t, view, "properties")
	if _, hasLinks := viewProperties["links"]; !hasLinks {
		t.Errorf("ContinuationView documents the links array")
	}
	if _, hasUses := viewProperties["uses"]; !hasUses {
		t.Errorf("ContinuationView documents the uses feed")
	}
	linkView := mapping(t, schemas, "ContinuationLinkView")
	linkStateProp := mapping(t, mapping(t, linkView, "properties"), "state")
	linkState, _ := linkStateProp["enum"].([]any)
	stateWords := map[string]bool{}
	for _, word := range linkState {
		if text, _ := word.(string); text == "valid" || text == "expired" || text == "revoked" || text == "consumed" || text == "owner_stale" {
			stateWords[text] = true
		}
		if text, _ := word.(string); text == "unknown" {
			t.Errorf("the unknown word is the lookup-layer refusal: the view derivation never produces it, got %v", linkState)
		}
	}
	if len(stateWords) != 5 {
		t.Errorf("the link state carries the five produced words, got %v", linkState)
	}
	useView := mapping(t, schemas, "ContinuationUseView")
	useKindProp := mapping(t, mapping(t, useView, "properties"), "use_kind")
	useKind, _ := useKindProp["enum"].([]any)
	if len(useKind) != 3 {
		t.Errorf("the use kinds carry the three-word dictionary (the correction, the human request, the return), got %v", useKind)
	}
	if _, hasFact := mapping(t, useView, "properties")["fact_id"]; !hasFact {
		t.Errorf("ContinuationUseView documents the correction material pair")
	}

	// The sales reply documents the delivery binding (the FR-16 delivered
	// badge); the public routes carry no JSON surface.
	reply := mapping(t, schemas, "SendClientReplyRequest")
	if _, hasLink := mapping(t, reply, "properties")["link_id"]; !hasLink {
		t.Errorf("SendClientReplyRequest documents the continuation link_id delivery binding")
	}

	// The case view carries the continuations section.
	caseView := mapping(t, schemas, "CaseView")
	if _, hasSection := mapping(t, caseView, "properties")["continuations"]; !hasSection {
		t.Errorf("CaseView documents the continuations section of Story 2.10")
	}
}
