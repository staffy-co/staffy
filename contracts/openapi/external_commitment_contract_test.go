package openapi_test

import (
	"os"
	"testing"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"gopkg.in/yaml.v3"
)

// TestExternalCommitmentContractIsHonest pins the Story 3.3 first-slice
// wire contract: the six operator paths carry the principal boundary and
// the honest answer/refusal codes (a non-member is the plain 404, a
// member without the scope is the 403 with the app words BEFORE material,
// the step-up refusal and the AG-4 drain refusal keep their words); the
// read carries no 400 (the view carries no material to refuse); the
// mutations describe the step-up/approval semantics; the views mirror the
// domain dictionaries EXACTLY (every enum word a compile-time dictionary
// word — drift turns this test red: purpose 4/recipient 2/ack 2/состояния
// 7/kinds 7/причины 4/readiness 5/outcome 4/каналы 9/relay 3); the
// material bounds mirror the domain constants (the content, the next
// step, the safe details); the required lists are complete and
// additionalProperties is false everywhere.
func TestExternalCommitmentContractIsHonest(t *testing.T) {
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
	readPath := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/external-commitments")
	readGet := mapping(t, readPath, "get")
	if readGet["operationId"] != "readExternalCommitments" {
		t.Errorf("the external commitments read operation id is wrong: %v", readGet["operationId"])
	}
	if required, _ := readGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the external commitments read does not document the principal boundary")
	}
	readResponses := mapping(t, readGet, "responses")
	for _, code := range []string{"200", "401", "403", "404"} {
		if _, documented := readResponses[code]; !documented {
			t.Errorf("the external commitments read does not document the honest %s answer", code)
		}
	}
	if _, documented := readResponses["400"]; documented {
		t.Errorf("the external commitments read must not document a 400: the view carries no material to refuse")
	}
	readSchema := mapping(t, mapping(t, mapping(t, mapping(t, readResponses, "200"), "content"), "application/json"), "schema")
	if required, _ := readSchema["required"].([]any); !requiredWord(required, "external_commitments") {
		t.Errorf("the read answer must carry the required external_commitments wrapper, got %v", required)
	}
	if ref, _ := mapping(t, mapping(t, readSchema, "properties"), "external_commitments")["$ref"].(string); ref != "#/components/schemas/ExternalCommitmentView" {
		t.Errorf("the read wrapper must reference ExternalCommitmentView, got %v", ref)
	}

	// The create path: the mutation codes, the strict request body.
	createPath := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/external-commitments")
	createPost := mapping(t, createPath, "post")
	if createPost["operationId"] != "createExternalCommitment" {
		t.Errorf("the create operation id is wrong: %v", createPost["operationId"])
	}
	if required, _ := createPost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the create does not document the principal boundary")
	}
	createResponses := mapping(t, createPost, "responses")
	for _, code := range []string{"200", "400", "401", "403", "404"} {
		if _, documented := createResponses[code]; !documented {
			t.Errorf("the create does not document the honest %s answer", code)
		}
	}
	createBody := mapping(t, mapping(t, createPost, "requestBody"), "content")
	if ref, _ := mapping(t, mapping(t, createBody, "application/json"), "schema")["$ref"].(string); ref != "#/components/schemas/CreateExternalCommitmentRequest" {
		t.Errorf("the create request body must reference CreateExternalCommitmentRequest")
	}

	// The per-version mutation paths: the approve (step-up + the owner
	// gate 400), the enqueue (the approval/readiness 400 BEFORE the
	// transaction) and the relay (the observation dictionary 400).
	for _, spec := range []struct {
		action string
		codes  []string
	}{
		{"approve", []string{"200", "400", "401", "403", "404"}},
		{"enqueue", []string{"200", "400", "401", "403", "404"}},
		{"dispatch-receipts", []string{"200", "400", "401", "403", "404"}},
	} {
		path := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/external-commitments/{ext_commitment_id}/versions/{version_no}/"+spec.action)
		post := mapping(t, path, "post")
		if required, _ := post["x-staffy-principal-required"].(bool); !required {
			t.Errorf("the %s mutation does not document the principal boundary", spec.action)
		}
		responses := mapping(t, post, "responses")
		for _, code := range spec.codes {
			if _, documented := responses[code]; !documented {
				t.Errorf("the %s mutation does not document the honest %s answer", spec.action, code)
			}
		}
	}

	// The drain path: the AG-4 refusal is the documented 400 (the honest
	// state of the production contour, not a defect).
	drainPost := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/cases/external-commitments/drain"), "post")
	if drainPost["operationId"] != "drainExternalCommitments" {
		t.Errorf("the drain operation id is wrong: %v", drainPost["operationId"])
	}
	if required, _ := drainPost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the drain does not document the principal boundary")
	}
	drainResponses := mapping(t, drainPost, "responses")
	for _, code := range []string{"200", "400", "401", "403", "404"} {
		if _, documented := drainResponses[code]; !documented {
			t.Errorf("the drain does not document the honest %s answer (the AG-4 refusal rides the 400)", code)
		}
	}

	// The enum mirrors — every wire word a compile-time dictionary word.
	assertEnumMirrors(t, mapping(t, schemas, "ExternalPurpose")["enum"], casesDomain.ExternalPurposeWords, "purpose")
	assertEnumMirrors(t, mapping(t, schemas, "ExternalRecipientKind")["enum"], casesDomain.ExternalRecipientKindWords, "recipient_kind")
	assertEnumMirrors(t, mapping(t, schemas, "ExternalAckRequirement")["enum"], casesDomain.ExternalAckRequirementWords, "ack_requirement")
	assertEnumMirrors(t, mapping(t, schemas, "ExternalChannel")["enum"], casesDomain.ChannelOrder, "channel")
	assertEnumMirrors(t, mapping(t, schemas, "ExternalDeliveryState")["enum"], casesDomain.ExternalDeliveryStateWords, "delivery_state")
	assertEnumMirrors(t, mapping(t, schemas, "ExternalDispatchKind")["enum"], casesDomain.ExternalDispatchKindWords, "dispatch_kind")
	assertEnumMirrors(t, mapping(t, schemas, "ExternalStaleReason")["enum"], casesDomain.ExternalStaleReasonWords, "stale_reason")
	assertEnumMirrors(t, mapping(t, schemas, "ExternalReadinessWord")["enum"], casesDomain.ExternalReadinessWords, "readiness")
	assertEnumMirrors(t, mapping(t, schemas, "ExternalDispatchOutcome")["enum"], casesDomain.DispatchOutcomeWords, "dispatch_outcome")

	// The relay dictionaries: the request observation words and the
	// receipt kinds of the outcome (the RelayObservationDispatchKind
	// mapping of the domain).
	relayRequest := mapping(t, schemas, "RelayExternalReceiptRequest")
	if required, _ := relayRequest["required"].([]any); !requiredWord(required, "observation") {
		t.Errorf("the relay request must require the observation word, got %v", required)
	}
	relayProperties := mapping(t, relayRequest, "properties")
	assertEnumMirrors(t, mapping(t, relayProperties, "observation")["enum"], casesDomain.RelayObservationWords, "relay_observation")
	receiptKinds := []string{
		casesDomain.RelayObservationDispatchKind(casesDomain.RelayObservationDelivered),
		casesDomain.RelayObservationDispatchKind(casesDomain.RelayObservationAcknowledged),
		casesDomain.RelayObservationDispatchKind(casesDomain.RelayObservationFailed),
	}
	relayOutcome := mapping(t, schemas, "ExternalRelayOutcome")
	assertEnumMirrors(t, mapping(t, mapping(t, relayOutcome, "properties"), "kind")["enum"], receiptKinds, "relay_outcome_kind")

	// The closed shapes: additionalProperties false and the complete
	// required lists (the wire never accepts silence).
	for _, spec := range []struct {
		name     string
		required int
	}{
		{"ExternalCommitmentView", 9},
		{"ExternalCommitmentWorkItemView", 2},
		{"ExternalCommitmentVersionCard", 9},
		{"ExternalCommitmentVersionWire", 21},
		{"ExternalCommitmentPinWire", 3},
		{"ExternalDeliveryWire", 8},
		{"ExternalDispatchEventWire", 10},
		{"ExternalCommitmentPinInput", 3},
		{"ExternalCommitmentOutcome", 3},
		{"ExternalApprovalOutcome", 3},
		{"ExternalEnqueueOutcome", 3},
		{"ExternalRelayOutcome", 3},
		{"ExternalDrainResult", 8},
		{"ExternalDrainReport", 3},
	} {
		schema := mapping(t, schemas, spec.name)
		if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
			t.Errorf("%s must reject additionalProperties (false), got %v", spec.name, schema["additionalProperties"])
		}
		if required, _ := schema["required"].([]any); len(required) != spec.required {
			t.Errorf("%s must be fully required (%d fields), got %v", spec.name, spec.required, required)
		}
	}

	// The create request: the seven required material words (the optional
	// anchors stay optional — the stable id and the pin), the bounds of
	// the material mirror the domain constants.
	createRequest := mapping(t, schemas, "CreateExternalCommitmentRequest")
	if additional, ok := createRequest["additionalProperties"].(bool); !ok || additional {
		t.Errorf("CreateExternalCommitmentRequest must reject additionalProperties (false), got %v", createRequest["additionalProperties"])
	}
	if required, _ := createRequest["required"].([]any); len(required) != 7 {
		t.Errorf("CreateExternalCommitmentRequest must require the seven material words (the stable id and the pin stay optional), got %v", required)
	}
	createProperties := mapping(t, createRequest, "properties")
	if bound := mapping(t, createProperties, "content_text"); bound["maxLength"].(int) != casesDomain.MaxExternalCommitmentContentRunes {
		t.Errorf("the content bound must mirror the domain %d, got %v", casesDomain.MaxExternalCommitmentContentRunes, bound["maxLength"])
	}
	if bound := mapping(t, createProperties, "expected_next_step"); bound["maxLength"].(int) != casesDomain.MaxExpectedNextStepRunes {
		t.Errorf("the next-step bound must mirror the domain %d, got %v", casesDomain.MaxExpectedNextStepRunes, bound["maxLength"])
	}

	// The event safe-details bound of the append-only history (AC-10: the
	// safe text never carries the content).
	eventWire := mapping(t, mapping(t, mapping(t, schemas, "ExternalDispatchEventWire"), "properties"), "details")
	if eventWire["maxLength"].(int) != casesDomain.MaxExternalDispatchDetailsRunes {
		t.Errorf("the safe-details bound must mirror the domain %d, got %v", casesDomain.MaxExternalDispatchDetailsRunes, eventWire["maxLength"])
	}

	// The version wire bounds: the same material mirrors plus the
	// content token shape.
	versionWire := mapping(t, mapping(t, schemas, "ExternalCommitmentVersionWire"), "properties")
	if bound := mapping(t, versionWire, "content_text"); bound["maxLength"].(int) != casesDomain.MaxExternalCommitmentContentRunes {
		t.Errorf("the version content bound must mirror the domain, got %v", bound["maxLength"])
	}
	if bound := mapping(t, versionWire, "expected_next_step"); bound["maxLength"].(int) != casesDomain.MaxExpectedNextStepRunes {
		t.Errorf("the version next-step bound must mirror the domain, got %v", bound["maxLength"])
	}
}
