package openapi_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestCorrectionContractIsHonest pins the Story 2.15 wire contract: the
// machine path of the operator correction record carries the principal
// boundary with the Idempotency-Key and the honest answer and refusal
// codes (the fresh record 201, the exact repeat 200, the dictionary and
// bound refusals 400, the scope gate 403, the foreign tenant the plain
// 404, the raced correction number and the key conflict the honest 409);
// the request carries the pinned target with the material and NEVER the
// server-side truth (the OUTCOME is the domain matrix-gate decision —
// the strict decode refuses the field; the correction number is derived
// under the locks); the fact rows of the decision context document the
// read-time correction markers (the honest outcome dictionary applied and
// disputed with the byte-exact spans behind the disclosure); the stored
// outcome carries the correction id, the outcome word and the span
// counter only (the proposed value and the spans NEVER enter; the
// correction number never enters).
func TestCorrectionContractIsHonest(t *testing.T) {
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
	// honest answer and refusal codes (the read of the correction state
	// travels the decision-context GET of 2.14 — NO new read path).
	writePath := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/corrections")
	post := mapping(t, writePath, "post")
	if required, _ := post["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the corrections POST does not document the principal boundary")
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
		t.Errorf("the corrections POST is missing the Idempotency-Key parameter")
	}
	writeResponses := mapping(t, post, "responses")
	for _, code := range []string{"200", "201", "400", "401", "403", "404", "409"} {
		if _, documented := writeResponses[code]; !documented {
			t.Errorf("the corrections POST does not document the honest %s answer", code)
		}
	}
	// The one write path only: the correction state is READ through the
	// decision context of 2.14 (the context is ONE resource — no new read
	// path of the slice).
	if _, hasRead := writePath["get"]; hasRead {
		t.Errorf("the corrections path must NOT carry a GET: the correction state reads through the decision context")
	}

	// The operator request: the pinned target with the material — and NO
	// server-side truth fields (the OUTCOME is the domain matrix-gate
	// decision, the correction number is the lock-visible derivation; a
	// request carrying either is the strict decode refusal).
	request := mapping(t, schemas, "RecordCorrectionRequest")
	requestProperties := mapping(t, request, "properties")
	if requestRequired, _ := request["required"].([]any); len(requestRequired) != 3 {
		t.Errorf("the correction record requires the version pin, the value and the reason, got %v", requestRequired)
	}
	for _, field := range []string{"version_no", "summary_no", "value", "spans", "reason"} {
		if _, documented := requestProperties[field]; !documented {
			t.Errorf("RecordCorrectionRequest documents the %s of the operator material", field)
		}
	}
	for _, forbidden := range []string{"outcome", "correction_no", "actor_id", "channel"} {
		if _, hasField := requestProperties[forbidden]; hasField {
			t.Errorf("RecordCorrectionRequest must NOT carry %s: the server-side truth of the matrix gate and the locks, never a request field", forbidden)
		}
	}
	spans, _ := requestProperties["spans"].(map[string]any)
	if maxItems, _ := spans["maxItems"].(int); maxItems != 8 {
		t.Errorf("the spans carry the material bound: maxItems 8, got %v", spans["maxItems"])
	}

	// The fact rows of the context view: the read-time correction markers
	// with the honest outcome dictionary (applied pours, disputed never
	// rewrites — the SERVER decision words) and the byte-exact spans.
	factRow := mapping(t, schemas, "DecisionContextFactRow")
	factProperties := mapping(t, factRow, "properties")
	outcomeEnum, _ := factProperties["correction_outcome"].(map[string]any)["enum"].([]any)
	outcomeWords := map[string]bool{}
	for _, word := range outcomeEnum {
		if text, _ := word.(string); text == "applied" || text == "disputed" {
			outcomeWords[text] = true
		}
	}
	if len(outcomeWords) != 2 {
		t.Errorf("the correction outcome dictionary carries the two honest matrix-gate words, got %v", outcomeEnum)
	}
	for _, field := range []string{
		"has_correction", "correction_marker_text", "correction_source_text", "correction_no",
		"correction_at", "correction_spans", "correction_reason",
	} {
		if _, documented := factProperties[field]; !documented {
			t.Errorf("DecisionContextFactRow documents the %s of the read-time correction state", field)
		}
	}

	// The stored outcome: the correction id with the outcome word and the
	// span counter (the proposed value and the spans NEVER enter; the
	// correction number is the server truth — never in the stored
	// outcome).
	outcome := mapping(t, schemas, "WorkOutcome")
	outcomeProperties := mapping(t, outcome, "properties")
	for _, field := range []string{"correction_id", "correction_outcome", "correction_spans"} {
		if _, documented := outcomeProperties[field]; !documented {
			t.Errorf("WorkOutcome documents the %s of the Story 2.15 command", field)
		}
	}
	for _, forbidden := range []string{"correction_no", "correction_value", "correction_span_texts"} {
		if _, hasField := outcomeProperties[forbidden]; hasField {
			t.Errorf("WorkOutcome must NOT carry %s: the stored outcome never carries the server number or the correction material", forbidden)
		}
	}
}
