package openapi_test

import (
	"os"
	"testing"

	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	"gopkg.in/yaml.v3"
)

// TestActionPreviewContractIsHonest pins the Story 3.4 first-slice wire
// contract: the five machine paths carry the principal boundary and the
// honest answer/refusal codes (a non-member is the plain 404, a member
// without the scope is the 403 with the app words BEFORE material, the
// typed-invalid attempt is the 400 of the word list, the terminal repeat
// is the 409, the policy block of a typed-valid attempt is the 403 of
// the reason word); the read carries NO 400 (the view carries no material
// to refuse); the views mirror the domain dictionaries EXACTLY (every
// enum word a compile-time dictionary word — drift turns this test red:
// verdict 3/target 3/status 5/readiness 2/Timeline 3/причины свежести 7
// ПО ПОРЯДКУ/причины Policy 10/partition 2/failure 3/dry-run 3); the
// material bounds mirror the domain constants (the typed value, the
// invalid report, the safe details); the required lists are complete
// and additionalProperties is false everywhere.
func TestActionPreviewContractIsHonest(t *testing.T) {
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

	// The read path: the principal boundary, the honest codes; NO 400 (the
	// view carries no material to refuse), the wrapper of the view.
	readPath := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/action-previews")
	readGet := mapping(t, readPath, "get")
	if readGet["operationId"] != "readActionPreviews" {
		t.Errorf("the action previews read operation id is wrong: %v", readGet["operationId"])
	}
	if required, _ := readGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the action previews read does not document the principal boundary")
	}
	readResponses := mapping(t, readGet, "responses")
	for _, code := range []string{"200", "401", "403", "404"} {
		if _, documented := readResponses[code]; !documented {
			t.Errorf("the action previews read does not document the honest %s answer", code)
		}
	}
	if _, documented := readResponses["400"]; documented {
		t.Errorf("the action previews read must not document a 400: the view carries no material to refuse")
	}
	readSchema := mapping(t, mapping(t, mapping(t, mapping(t, readResponses, "200"), "content"), "application/json"), "schema")
	if required, _ := readSchema["required"].([]any); !requiredWord(required, "previews") {
		t.Errorf("the read answer must carry the required previews wrapper, got %v", required)
	}
	if ref, _ := mapping(t, mapping(t, readSchema, "properties"), "previews")["$ref"].(string); ref != "#/components/schemas/PreviewView" {
		t.Errorf("the read wrapper must reference PreviewView, got %v", ref)
	}

	// The thread read: the future 3-5 material of the (preview_id,
	// version_no) pin references.
	threadGet := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}"), "get")
	if threadGet["operationId"] != "readActionPreviewThread" {
		t.Errorf("the thread read operation id is wrong: %v", threadGet["operationId"])
	}
	if required, _ := threadGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the thread read does not document the principal boundary")
	}

	// The proposal submission: the AD-7 verdict ladder of the status codes
	// (400 invalid with the word list, 409 terminal, 403 policy words,
	// 200 the built version) — every documented, none invented.
	submitPath := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/action-previews/proposals")
	submitPost := mapping(t, submitPath, "post")
	if submitPost["operationId"] != "submitActionProposal" {
		t.Errorf("the submit operation id is wrong: %v", submitPost["operationId"])
	}
	if required, _ := submitPost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the submit does not document the principal boundary")
	}
	submitResponses := mapping(t, submitPost, "responses")
	for _, code := range []string{"200", "400", "401", "403", "404", "409"} {
		if _, documented := submitResponses[code]; !documented {
			t.Errorf("the submit does not document the honest %s answer of the AD-7 ladder", code)
		}
	}
	submitBody := mapping(t, mapping(t, submitPost, "requestBody"), "content")
	if ref, _ := mapping(t, mapping(t, submitBody, "application/json"), "schema")["$ref"].(string); ref != "#/components/schemas/SubmitProposalRequest" {
		t.Errorf("the submit request body must reference SubmitProposalRequest")
	}

	// The edit and the dry-run: the full-chain 400/403 BEFORE any write
	// and the AG-4 refusal on the 400.
	for _, spec := range []struct {
		path  string
		codes []string
	}{
		{"/v1/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}/versions", []string{"200", "400", "401", "403", "404", "409"}},
		{"/v1/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}/versions/{version_no}/dry-runs", []string{"200", "400", "401", "403", "404"}},
	} {
		post := mapping(t, paths, spec.path)["post"]
		if post == nil {
			t.Errorf("the path %s is missing its post", spec.path)
			continue
		}
		postMap, ok := post.(map[string]any)
		if !ok {
			t.Errorf("the path %s post is not a mapping", spec.path)
			continue
		}
		if required, _ := postMap["x-staffy-principal-required"].(bool); !required {
			t.Errorf("the %s mutation does not document the principal boundary", spec.path)
		}
		responses := mapping(t, postMap, "responses")
		for _, code := range spec.codes {
			if _, documented := responses[code]; !documented {
				t.Errorf("the %s mutation does not document the honest %s answer", spec.path, code)
			}
		}
	}

	// The enum mirrors — every wire word a compile-time dictionary word,
	// every NORMATIVE order preserved byte-for-byte (the freshness reasons
	// and the policy reasons are ORDERED dictionaries).
	assertEnumMirrors(t, mapping(t, schemas, "ProposalVerdict")["enum"], actionDomain.VerdictWords, "verdict")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewTargetKind")["enum"], actionDomain.TargetKindWords, "target_kind")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewSnapshotStatus")["enum"], actionDomain.SnapshotStatusWords, "snapshot_status")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewReadiness")["enum"], actionDomain.PreviewReadinessWords, "readiness")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewTimelineWord")["enum"], actionDomain.TimelineWords, "timeline")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewFreshnessReason")["enum"], actionDomain.PreviewFreshnessReasonWords, "freshness_reason")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewPolicyReason")["enum"], actionDomain.PolicyReasonWords, "policy_reason")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewPartitionWord")["enum"], actionDomain.ExecutionPartitionWords, "partition")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewFailureSemantics")["enum"], actionDomain.FailureSemanticsWords, "failure_semantics")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewDryRunResult")["enum"], actionDomain.DryRunResultWords, "dry_run_result")

	// The closed shapes: additionalProperties false and the complete
	// required lists (the wire never accepts silence).
	for _, spec := range []struct {
		name     string
		required int
	}{
		{"PreviewView", 5},
		{"PreviewThreadView", 4},
		{"PreviewCardView", 22},
		{"PreviewSnapshotRow", 6},
		{"PreviewEffect", 3},
		{"PreviewAttemptView", 4},
		{"PreviewDryRunEvidenceView", 3},
		{"PreviewVersionPin", 2},
		{"ProposalOutcome", 3},
		{"PreviewVersionOutcome", 5},
		{"DryRunOutcome", 1},
	} {
		schema := mapping(t, schemas, spec.name)
		if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
			t.Errorf("%s must reject additionalProperties (false), got %v", spec.name, schema["additionalProperties"])
		}
		if required, _ := schema["required"].([]any); len(required) != spec.required {
			t.Errorf("%s must be fully required (%d fields), got %v", spec.name, spec.required, required)
		}
	}

	// The submit request: the six mandatory pins (the supporting refs stay
	// optional), the typed-value bound mirrors the domain constant, the
	// epoch is the live positive observation.
	submitRequest := mapping(t, schemas, "SubmitProposalRequest")
	if additional, ok := submitRequest["additionalProperties"].(bool); !ok || additional {
		t.Errorf("SubmitProposalRequest must reject additionalProperties (false), got %v", submitRequest["additionalProperties"])
	}
	if required, _ := submitRequest["required"].([]any); len(required) != 6 {
		t.Errorf("SubmitProposalRequest must require the six mandatory pins (the supporting refs stay optional), got %v", required)
	}
	submitProperties := mapping(t, submitRequest, "properties")
	typedInput := mapping(t, submitProperties, "typed_input")
	if additional, ok := typedInput["additionalProperties"].(map[string]any); !ok {
		t.Errorf("the typed input must declare the open string map of the SCHEMA fields (unknown names refuse at the domain guard by name), got %v", typedInput["additionalProperties"])
	} else if bound, _ := additional["maxLength"].(int); bound != actionDomain.MaxTypedInputValueRunes {
		t.Errorf("the typed value bound must mirror the domain %d, got %v", actionDomain.MaxTypedInputValueRunes, bound)
	}

	// The attempt ladder bound of the wire: attempt_no is the closed {1, 2}
	// (the third attempt does not exist — the structural bound rides the
	// migration CHECK; the wire speaks it).
	attemptProperties := mapping(t, mapping(t, schemas, "ProposalOutcome"), "properties")
	if attemptBound := mapping(t, attemptProperties, "attempt_no"); attemptBound["enum"] == nil {
		t.Errorf("the proposal outcome attempt_no must carry the closed {1,2} enum of the AD-7 bound")
	} else if enum, _ := attemptBound["enum"].([]any); len(enum) != actionDomain.MaxAttemptsProposal {
		t.Errorf("the attempt enum must mirror the domain bound %d, got %v", actionDomain.MaxAttemptsProposal, enum)
	}

	// The invalid report bound of the attempt rows: the loyal backstop of
	// the spoken truncation (the store CHECK 2100).
	attemptView := mapping(t, mapping(t, schemas, "PreviewAttemptView"), "properties")
	if bound := mapping(t, attemptView, "invalid_report"); bound["maxLength"].(int) < actionDomain.MaxInvalidReportRunes {
		t.Errorf("the invalid report bound must be the loyal backstop of the domain %d, got %v", actionDomain.MaxInvalidReportRunes, bound["maxLength"])
	}

	// The safe-details bound of the dry-run evidence (the safe text only).
	dryRunProperties := mapping(t, mapping(t, schemas, "DryRunOutcome"), "properties")
	if bound := mapping(t, dryRunProperties, "details"); bound["maxLength"].(int) != actionDomain.MaxEventDetailsRunes {
		t.Errorf("the safe-details bound must mirror the domain %d, got %v", actionDomain.MaxEventDetailsRunes, bound["maxLength"])
	}
}
