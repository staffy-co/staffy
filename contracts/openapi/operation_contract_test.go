package openapi_test

import (
	"os"
	"testing"

	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	"gopkg.in/yaml.v3"
)

// TestOperationContractIsHonest pins the Story 3.5 first-slice wire
// contract and the Story 3.6 step-03 execution surface: the six machine
// paths carry the principal boundary and the honest answer/refusal codes
// (the consent 201, the revocation 200, the authorization ladder 200
// replay/reference / 201 first / 400 key gate and strict decode / 403
// step-up, policy and the fail-closed fresh reads / 404 the dead pins /
// 409 the exact conflict and ladder words); the drain path (3-6) carries
// the honest ladder 201/400/401/403/404 (the AG-4 nil-adapter refusal
// speaks 403 with the honest words); the reads carry NO 400 (the views
// carry no material to refuse); the enum mirrors are EXACT (length AND
// membership — every wire word a compile-time dictionary word, drift
// turns this test red: confirmation states 5 / confirmation events 6 /
// ledger kinds 8 / partition 2 / operation states 6 — the life words of
// 3-6 step-03, the queue fact moved to the structured boolean / the
// adapter outcome words 4 / the receipt outcome words 3 / owner kinds
// 2 / the reconciliation verdict words 4 — the 3-7 step-03 mirror); the
// reconcile path (3-7) carries the honest ladder 201/400/401/403/404
// (the AG-4 nil-adapter refusal speaks 403 with the honest words); the idempotency bounds mirror the domain constants; the required
// lists are complete and additionalProperties is false everywhere.
func TestOperationContractIsHonest(t *testing.T) {
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

	// The view read: the principal boundary, the honest codes; NO 400 (the
	// view carries no material to refuse), the wrapper of the view.
	readPath := mapping(t, paths, "/v1/tenants/{tenant_id}/cases/{case_id}/operations")
	readGet := mapping(t, readPath, "get")
	if readGet["operationId"] != "readAuthorizationView" {
		t.Errorf("the operations view read operation id is wrong: %v", readGet["operationId"])
	}
	if required, _ := readGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the operations view read does not document the principal boundary")
	}
	readResponses := mapping(t, readGet, "responses")
	for _, code := range []string{"200", "401", "403", "404"} {
		if _, documented := readResponses[code]; !documented {
			t.Errorf("the operations view read does not document the honest %s answer", code)
		}
	}
	if _, documented := readResponses["400"]; documented {
		t.Errorf("the operations view read must not document a 400: the view carries no material to refuse")
	}
	readSchema := mapping(t, mapping(t, mapping(t, mapping(t, readResponses, "200"), "content"), "application/json"), "schema")
	if required, _ := readSchema["required"].([]any); !requiredWord(required, "authorizations") {
		t.Errorf("the view read answer must carry the required authorizations wrapper, got %v", required)
	}

	// The thread read and the operation read: the future material of the
	// confirmation pin references and the operation card.
	for _, spec := range []struct {
		path        string
		operationID string
		wrapper     string
	}{
		{"/v1/tenants/{tenant_id}/cases/{case_id}/confirmations/{confirmation_id}", "readConfirmationThread", "confirmations"},
		{"/v1/tenants/{tenant_id}/cases/{case_id}/operations/{operation_id}", "readOperation", "operation"},
	} {
		get := mapping(t, mapping(t, paths, spec.path), "get")
		if get["operationId"] != spec.operationID {
			t.Errorf("the read operation id of %s is wrong: %v", spec.path, get["operationId"])
		}
		if required, _ := get["x-staffy-principal-required"].(bool); !required {
			t.Errorf("the read %s does not document the principal boundary", spec.path)
		}
		if _, documented := mapping(t, get, "responses")["400"]; documented {
			t.Errorf("the read %s must not document a 400: the card carries no material to refuse", spec.path)
		}
		schema := mapping(t, mapping(t, mapping(t, mapping(t, mapping(t, get, "responses"), "200"), "content"), "application/json"), "schema")
		if required, _ := schema["required"].([]any); !requiredWord(required, spec.wrapper) {
			t.Errorf("the read %s answer must carry the required %s wrapper, got %v", spec.path, spec.wrapper, required)
		}
	}

	// The consent: the human act with the LIVE step-up — 201 of the new
	// version, 400 of the guard/strict-decode words, 403 of the step-up,
	// 409 of the raced ladder.
	confirmPath := "/v1/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}/versions/{version_no}/confirmations"
	confirmPost := mapping(t, mapping(t, paths, confirmPath), "post")
	if confirmPost["operationId"] != "confirmPreviewVersion" {
		t.Errorf("the confirm operation id is wrong: %v", confirmPost["operationId"])
	}
	if required, _ := confirmPost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the confirm does not document the principal boundary")
	}
	confirmResponses := mapping(t, confirmPost, "responses")
	for _, code := range []string{"201", "400", "401", "403", "404", "409"} {
		if _, documented := confirmResponses[code]; !documented {
			t.Errorf("the confirm does not document the honest %s answer", code)
		}
	}
	if _, documented := confirmResponses["200"]; documented {
		t.Errorf("the confirm must not document a 200: the consent answers 201 (the append-only version exists now)")
	}
	confirmBody := mapping(t, mapping(t, mapping(t, confirmPost, "requestBody"), "content"), "application/json")
	if ref, _ := confirmBody["schema"].(map[string]any)["$ref"].(string); ref != "#/components/schemas/ConfirmPreviewRequest" {
		t.Errorf("the confirm request body must reference ConfirmPreviewRequest, got %v", confirmBody["schema"])
	}

	// The revocation: the human act with the LIVE step-up — 200 of the
	// revoked words, the empty strict body.
	revokePath := "/v1/tenants/{tenant_id}/cases/{case_id}/confirmations/{confirmation_id}/versions/{version_no}/revocations"
	revokePost := mapping(t, mapping(t, paths, revokePath), "post")
	if revokePost["operationId"] != "revokeConfirmation" {
		t.Errorf("the revoke operation id is wrong: %v", revokePost["operationId"])
	}
	if required, _ := revokePost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the revoke does not document the principal boundary")
	}
	revokeResponses := mapping(t, revokePost, "responses")
	for _, code := range []string{"200", "400", "401", "403", "404"} {
		if _, documented := revokeResponses[code]; !documented {
			t.Errorf("the revoke does not document the honest %s answer", code)
		}
	}

	// The authorization: the machine repeat surface — the honest status
	// ladder of the replay semantics (200) and the first authorization
	// (201); the 409 conflict and the ladder words; the 400 key gate and
	// the strict decode of the AG-10 free field.
	authorizePath := "/v1/tenants/{tenant_id}/cases/{case_id}/confirmations/{confirmation_id}/versions/{version_no}/authorizations"
	authorizePost := mapping(t, mapping(t, paths, authorizePath), "post")
	if authorizePost["operationId"] != "authorizeOperation" {
		t.Errorf("the authorize operation id is wrong: %v", authorizePost["operationId"])
	}
	if required, _ := authorizePost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the authorize does not document the principal boundary")
	}
	authorizeResponses := mapping(t, authorizePost, "responses")
	for _, code := range []string{"200", "201", "400", "401", "403", "404", "409"} {
		if _, documented := authorizeResponses[code]; !documented {
			t.Errorf("the authorize does not document the honest %s answer of the status ladder", code)
		}
	}
	replayDescription, _ := authorizeResponses["200"].(map[string]any)["description"].(string)
	if replayDescription == "" || !contains(replayDescription, "идемпотентный") && !contains(replayDescription, "Идемпотентный") {
		t.Errorf("the authorize 200 answer must document the replay semantics (the idempotent return of the ORIGINAL operation), got %q", replayDescription)
	}
	conflictDescription, _ := authorizeResponses["409"].(map[string]any)["description"].(string)
	if conflictDescription == "" || !contains(conflictDescription, "authorization_conflict") {
		t.Errorf("the authorize 409 answer must document the visible conflict exception, got %q", conflictDescription)
	}
	if required, _ := mapping(t, authorizeResponses, "400")["description"].(string); required == "" {
		t.Errorf("the authorize 400 answer must document the key gate and the strict decode words")
	}

	// The authorize request: the client idempotency word ONLY — the bounds
	// mirror the domain constants (the loyal backstop of the gate).
	authorizeBody := mapping(t, mapping(t, mapping(t, authorizePost, "requestBody"), "content"), "application/json")
	authorizeRequestRef, _ := authorizeBody["schema"].(map[string]any)["$ref"].(string)
	if authorizeRequestRef != "#/components/schemas/AuthorizeOperationRequest" {
		t.Fatalf("the authorize request body must reference AuthorizeOperationRequest, got %v", authorizeBody["schema"])
	}
	authorizeRequest := mapping(t, schemas, "AuthorizeOperationRequest")
	if additional, ok := authorizeRequest["additionalProperties"].(bool); !ok || additional {
		t.Errorf("AuthorizeOperationRequest must reject additionalProperties (false), got %v", authorizeRequest["additionalProperties"])
	}
	if required, _ := authorizeRequest["required"].([]any); len(required) != 1 || !requiredWord(required, "idempotency_key") {
		t.Errorf("AuthorizeOperationRequest must require the idempotency word only, got %v", required)
	}
	idempotencyKey := mapping(t, mapping(t, authorizeRequest, "properties"), "idempotency_key")
	if bound, _ := idempotencyKey["minLength"].(int); bound != actionDomain.IdempotencyKeyMinRunes {
		t.Errorf("the idempotency min bound must mirror the domain %d, got %v", actionDomain.IdempotencyKeyMinRunes, bound)
	}
	if bound, _ := idempotencyKey["maxLength"].(int); bound != actionDomain.IdempotencyKeyMaxRunes {
		t.Errorf("the idempotency max bound must mirror the domain %d, got %v", actionDomain.IdempotencyKeyMaxRunes, bound)
	}
	if pattern, _ := idempotencyKey["pattern"].(string); pattern == "" {
		t.Errorf("the idempotency word must carry the charset pattern (the loyal backstop of the domain gate)")
	}

	// The enum mirrors — every wire word a compile-time dictionary word,
	// length AND membership (drift turns this red). The operation state
	// speaks the SIX life words of 3-6 (the 3-5 spoken pair-word
	// authorized/queued matured into the structured queue boolean —
	// step-03, the enum mirrors the LIVE domain dictionary).
	assertEnumMirrors(t, mapping(t, schemas, "ConfirmationStateWord")["enum"], actionDomain.ConfirmationStateWords, "confirmation_state")
	assertEnumMirrors(t, mapping(t, schemas, "ConfirmationEventKindWord")["enum"], actionDomain.ConfirmationEventKindWords, "confirmation_event_kind")
	assertEnumMirrors(t, mapping(t, schemas, "OperationLedgerKindWord")["enum"], actionDomain.LedgerEntryKindWords, "operation_ledger_kind")
	assertEnumMirrors(t, mapping(t, schemas, "PreviewPartitionWord")["enum"], actionDomain.ExecutionPartitionWords, "operation_partition")
	assertEnumMirrors(t, mapping(t, schemas, "OperationStateWord")["enum"], actionDomain.OperationStateWords, "operation_state")
	assertEnumMirrors(t, mapping(t, schemas, "ConfirmationOwnerKindWord")["enum"], actionDomain.OwnerKindWords, "confirmation_owner_kind")
	// The dispatch dictionaries (3-6 step-03): the adapter outcome words
	// (4 — the nested oneOf member carries the enum) and the Action
	// Receipt outcome words (3).
	attemptOutcome := mapping(t, mapping(t, mapping(t, schemas, "DispatchAttemptView"), "properties"), "outcome")
	oneOf, ok := attemptOutcome["oneOf"].([]any)
	if !ok || len(oneOf) == 0 {
		t.Fatalf("the dispatch attempt outcome must carry the oneOf enum member, got %v", attemptOutcome["oneOf"])
	}
	outcomeMember, _ := oneOf[0].(map[string]any)
	assertEnumMirrors(t, outcomeMember["enum"], actionDomain.MutationOutcomeWords, "mutation_outcome")
	assertEnumMirrors(t, mapping(t, mapping(t, mapping(t, schemas, "ActionReceiptView"), "properties"), "outcome")["enum"], actionDomain.ReceiptOutcomeWords, "receipt_outcome")
	// The reconciliation dictionary (3-7 step-03): the verdict words
	// (4 — the referenced enum schema mirrors the LIVE domain
	// dictionary).
	assertEnumMirrors(t, mapping(t, schemas, "ReconciliationVerdictWord")["enum"], actionDomain.ReconciliationVerdictWords, "reconciliation_verdict")
	// The recovery dictionaries (3-8 step-03): the outbox event words
	// (3 — the sanctioned 2→3 growth), the trigger words (3), the event
	// kind words (2), the ONE case state word (read-time open — the
	// closing words do not exist in the slice) and the decision words
	// (3) — every wire word a compile-time dictionary word.
	assertEnumMirrors(t, mapping(t, schemas, "OutboxEventWord")["enum"], actionDomain.OutboxEventWords, "outbox_event")
	assertEnumMirrors(t, mapping(t, schemas, "RecoveryTriggerWord")["enum"], actionDomain.RecoveryTriggerWords, "recovery_trigger")
	assertEnumMirrors(t, mapping(t, schemas, "RecoveryEventKindWord")["enum"], actionDomain.RecoveryEventKindWords, "recovery_event_kind")
	assertEnumMirrors(t, mapping(t, schemas, "RecoveryCaseStateWord")["enum"], actionDomain.RecoveryCaseStateWords, "recovery_case_state")
	assertEnumMirrors(t, mapping(t, schemas, "RecoveryDecisionWord")["enum"], []string{"created", "idempotent", "evidence"}, "recovery_decision")
	// The recovery action policy dictionaries (3-9 step-03): the kind
	// words (5 — the standard recovery policy of the epic), the
	// human-only words (2 — the class listed SEPARATELY from the set,
	// AC-1) and the state words (3 — the per-kind read-time answer) —
	// every wire word a compile-time dictionary word (the drift turns
	// the mirror red).
	assertEnumMirrors(t, mapping(t, schemas, "RecoveryActionKindWord")["enum"], actionDomain.RecoveryActionKindWords, "recovery_action_kind")
	assertEnumMirrors(t, mapping(t, schemas, "RecoveryHumanOnlyWord")["enum"], actionDomain.RecoveryHumanOnlyWords, "recovery_human_only")
	assertEnumMirrors(t, mapping(t, schemas, "RecoveryActionStateWord")["enum"], actionDomain.RecoveryActionStateWords, "recovery_action_state")

	// The drain path (3-6 step-03): the explicit operator command — the
	// honest ladder 201/400/401/403/404, the EMPTY strict body, the
	// principal boundary, the report wrapper and the attempts bound
	// mirroring the domain constant.
	drainPost := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/operations/drain"), "post")
	if drainPost["operationId"] != "drainOperations" {
		t.Errorf("the drain operation id is wrong: %v", drainPost["operationId"])
	}
	if required, _ := drainPost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the drain does not document the principal boundary")
	}
	drainResponses := mapping(t, drainPost, "responses")
	for _, code := range []string{"201", "400", "401", "403", "404"} {
		if _, documented := drainResponses[code]; !documented {
			t.Errorf("the drain does not document the honest %s answer", code)
		}
	}
	for _, forbidden := range []string{"200", "409", "500"} {
		if _, documented := drainResponses[forbidden]; documented {
			t.Errorf("the drain must not document a %s: the explicit command answers 201 with the report or refuses by the honest ladder", forbidden)
		}
	}
	refusalDescription, _ := drainResponses["403"].(map[string]any)["description"].(string)
	if refusalDescription == "" || !contains(refusalDescription, "AG-4") {
		t.Errorf("the drain 403 answer must document the honest AG-4 nil-adapter refusal words, got %q", refusalDescription)
	}
	drainBody := mapping(t, mapping(t, mapping(t, drainPost, "requestBody"), "content"), "application/json")
	if ref, _ := drainBody["schema"].(map[string]any)["$ref"].(string); ref != "#/components/schemas/DrainOperationsRequest" {
		t.Errorf("the drain request body must reference DrainOperationsRequest, got %v", drainBody["schema"])
	}
	drainSchema := mapping(t, mapping(t, mapping(t, mapping(t, mapping(t, drainResponses, "201"), "content"), "application/json"), "schema"), "properties")
	if ref, _ := drainSchema["drain"].(map[string]any)["$ref"].(string); ref != "#/components/schemas/DispatchDrainResultView" {
		t.Errorf("the drain 201 answer must reference DispatchDrainResultView, got %v", drainSchema["drain"])
	}
	attemptNo := mapping(t, mapping(t, mapping(t, schemas, "DispatchAttemptView"), "properties"), "attempt_no")
	if bound, _ := attemptNo["maximum"].(int); bound != actionDomain.MaxDispatchAttempts {
		t.Errorf("the attempt number bound must mirror the domain MaxDispatchAttempts %d, got %v", actionDomain.MaxDispatchAttempts, attemptNo["maximum"])
	}

	// The reconcile path (3-7 step-03): the explicit operator command —
	// the honest ladder 201/400/401/403/404, the EMPTY strict body, the
	// principal boundary, the report wrapper and the attempts bound
	// mirroring the domain constant.
	reconcilePost := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/operations/reconcile"), "post")
	if reconcilePost["operationId"] != "reconcileOperations" {
		t.Errorf("the reconcile operation id is wrong: %v", reconcilePost["operationId"])
	}
	if required, _ := reconcilePost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the reconcile does not document the principal boundary")
	}
	reconcileResponses := mapping(t, reconcilePost, "responses")
	for _, code := range []string{"201", "400", "401", "403", "404"} {
		if _, documented := reconcileResponses[code]; !documented {
			t.Errorf("the reconcile does not document the honest %s answer", code)
		}
	}
	for _, forbidden := range []string{"200", "409", "500"} {
		if _, documented := reconcileResponses[forbidden]; documented {
			t.Errorf("the reconcile must not document a %s: the explicit command answers 201 with the report or refuses by the honest ladder", forbidden)
		}
	}
	reconcileRefusal, _ := reconcileResponses["403"].(map[string]any)["description"].(string)
	if reconcileRefusal == "" || !contains(reconcileRefusal, "AG-4") {
		t.Errorf("the reconcile 403 answer must document the honest AG-4 nil-adapter refusal words, got %q", reconcileRefusal)
	}
	reconcileBody := mapping(t, mapping(t, mapping(t, reconcilePost, "requestBody"), "content"), "application/json")
	if ref, _ := reconcileBody["schema"].(map[string]any)["$ref"].(string); ref != "#/components/schemas/ReconcileOperationsRequest" {
		t.Errorf("the reconcile request body must reference ReconcileOperationsRequest, got %v", reconcileBody["schema"])
	}
	reconcileSchema := mapping(t, mapping(t, mapping(t, mapping(t, mapping(t, reconcileResponses, "201"), "content"), "application/json"), "schema"), "properties")
	if ref, _ := reconcileSchema["reconcile"].(map[string]any)["$ref"].(string); ref != "#/components/schemas/ReconcileResultView" {
		t.Errorf("the reconcile 201 answer must reference ReconcileResultView, got %v", reconcileSchema["reconcile"])
	}
	reconcileAttemptNo := mapping(t, mapping(t, mapping(t, schemas, "ReconciliationAttemptView"), "properties"), "attempt_no")
	if bound, _ := reconcileAttemptNo["maximum"].(int); bound != actionDomain.MaxReconciliationAttempts {
		t.Errorf("the reconcile attempt number bound must mirror the domain MaxReconciliationAttempts %d, got %v", actionDomain.MaxReconciliationAttempts, reconcileAttemptNo["maximum"])
	}

	// The recovery path (3-8 step-03): the explicit creation command of
	// one operation — the honest ladder 201/200/400/401/403/404 (the
	// repeat answers 200 idempotent/evidence — NOT a new resource), the
	// EMPTY strict body (the words of the assignment NEVER come from a
	// command parameter), the principal boundary and the eligibility
	// refusal words of the 403.
	recoveryPost := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/operations/{operation_id}/recovery-case"), "post")
	if recoveryPost["operationId"] != "createRecoveryCase" {
		t.Errorf("the recovery creation operation id is wrong: %v", recoveryPost["operationId"])
	}
	if required, _ := recoveryPost["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the recovery creation does not document the principal boundary")
	}
	recoveryResponses := mapping(t, recoveryPost, "responses")
	for _, code := range []string{"200", "201", "400", "401", "403", "404"} {
		if _, documented := recoveryResponses[code]; !documented {
			t.Errorf("the recovery creation does not document the honest %s answer", code)
		}
	}
	for _, forbidden := range []string{"409", "500"} {
		if _, documented := recoveryResponses[forbidden]; documented {
			t.Errorf("the recovery creation must not document a %s: the command answers 201/200 or refuses by the honest ladder words", forbidden)
		}
	}
	recoveryRefusal, _ := recoveryResponses["403"].(map[string]any)["description"].(string)
	for _, word := range []string{"operations", "лестниц", "fail-closed"} {
		if !contains(recoveryRefusal, word) {
			t.Errorf("the recovery 403 answer must document the eligibility ladder and the fail-closed assignment words (%q missing): %q", word, recoveryRefusal)
		}
	}
	recoveryBody := mapping(t, mapping(t, mapping(t, recoveryPost, "requestBody"), "content"), "application/json")
	if ref, _ := recoveryBody["schema"].(map[string]any)["$ref"].(string); ref != "#/components/schemas/RecoveryCreateRequest" {
		t.Errorf("the recovery creation request body must reference RecoveryCreateRequest, got %v", recoveryBody["schema"])
	}
	recoveryOK, _ := recoveryResponses["201"].(map[string]any)["description"].(string)
	if !contains(recoveryOK, "recovery_case.created") {
		t.Errorf("the recovery 201 answer must document the outbox record word, got %q", recoveryOK)
	}

	// The recovery reads (3-8 step-03): the panel list and the case card
	// — the principal boundary, NO 400 (the views carry no material to
	// refuse).
	recoveryListGet := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/recovery-cases"), "get")
	if recoveryListGet["operationId"] != "listRecoveryCases" {
		t.Errorf("the recovery list operation id is wrong: %v", recoveryListGet["operationId"])
	}
	if required, _ := recoveryListGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the recovery list does not document the principal boundary")
	}
	recoveryCardGet := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/recovery-cases/{recovery_case_id}"), "get")
	if recoveryCardGet["operationId"] != "readRecoveryCase" {
		t.Errorf("the recovery card operation id is wrong: %v", recoveryCardGet["operationId"])
	}
	if required, _ := recoveryCardGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the recovery card does not document the principal boundary")
	}
	for _, readPath := range []string{"/v1/tenants/{tenant_id}/recovery-cases", "/v1/tenants/{tenant_id}/recovery-cases/{recovery_case_id}", "/v1/tenants/{tenant_id}/recovery-cases/{recovery_case_id}/policy"} {
		readResponses := mapping(t, mapping(t, mapping(t, paths, readPath), "get"), "responses")
		for _, code := range []string{"200", "401", "403", "404"} {
			if _, documented := readResponses[code]; !documented {
				t.Errorf("the recovery read %s does not document the honest %s answer", readPath, code)
			}
		}
		if _, documented := readResponses["400"]; documented {
			t.Errorf("the recovery read %s must not document a 400: the view carries no material to refuse", readPath)
		}
	}

	// The recovery policy path (3-9 step-03): the READ-TIME derivation of
	// the allowed set — the principal boundary, the honest ladder
	// 200/401/403/404 with NO 400 (the empty GET never refuses), the
	// policy wrapper reference and the honest words of the summary
	// (записей нет, идемпотентность выводом, AG-4).
	policyGet := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/recovery-cases/{recovery_case_id}/policy"), "get")
	if policyGet["operationId"] != "readRecoveryCasePolicy" {
		t.Errorf("the recovery policy operation id is wrong: %v", policyGet["operationId"])
	}
	if required, _ := policyGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the recovery policy does not document the principal boundary")
	}
	policyResponses := mapping(t, policyGet, "responses")
	policySchema := mapping(t, mapping(t, mapping(t, mapping(t, policyResponses, "200"), "content"), "application/json"), "schema")
	policyProps := mapping(t, policySchema, "properties")
	if ref, _ := policyProps["recovery_policy"].(map[string]any)["$ref"].(string); ref != "#/components/schemas/RecoveryPolicyView" {
		t.Errorf("the policy 200 answer must reference RecoveryPolicyView, got %v", policySchema)
	}
	policySummary, _ := policyGet["summary"].(string)
	for _, word := range []string{"FR-39", "AC-2", "AC-1", "идемпотентность"} {
		if !contains(policySummary, word) {
			t.Errorf("the policy summary must document the honest words (%q missing): %q", word, policySummary)
		}
	}

	// The closed shapes: additionalProperties false and the complete
	// required lists (the wire never accepts silence).
	for _, spec := range []struct {
		name     string
		required int
	}{
		{"ConfirmPreviewRequest", 0},
		{"RevokeConfirmationRequest", 0},
		{"AuthorizeOperationRequest", 1},
		{"ConfirmationEventView", 4},
		{"ConfirmationCardView", 18},
		{"ConfirmationVersionOutcome", 9},
		{"ConfirmationRevocationOutcome", 6},
		{"OperationLedgerEntryView", 4},
		{"OperationCardView", 35},
		{"AuthorizationOutcome", 5},
		{"AuthorizationView", 4},
		{"DrainOperationsRequest", 0},
		{"DispatchReportLineView", 3},
		{"DispatchDrainResultView", 6},
		{"DispatchAttemptView", 8},
		{"ActionReceiptView", 15},
		{"ReconcileOperationsRequest", 0},
		{"ReconcileReportLineView", 3},
		{"ReconcileResultView", 6},
		{"ReconciliationAttemptView", 8},
		{"RecoveryCreateRequest", 0},
		{"RecoveryObservationPinView", 2},
		{"RecoveryAssignmentView", 5},
		{"RecoveryEventView", 5},
		{"RecoveryLinksView", 13},
		{"RecoveryCaseView", 18},
		{"RecoveryCreateResultView", 3},
		{"RecoveryActionView", 8},
		{"RecoveryHumanOnlyDecisionView", 2},
		{"RecoveryPolicyView", 4},
	} {
		schema := mapping(t, schemas, spec.name)
		if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
			t.Errorf("%s must reject additionalProperties (false), got %v", spec.name, schema["additionalProperties"])
		}
		if required, _ := schema["required"].([]any); len(required) != spec.required {
			t.Errorf("%s must be fully required (%d fields), got %v", spec.name, spec.required, required)
		}
	}

	// The material digest bound of the operation card (the keyed-HMAC
	// fingerprint — hex 64).
	operationProperties := mapping(t, mapping(t, schemas, "OperationCardView"), "properties")
	digest := mapping(t, operationProperties, "material_digest")
	if bound, _ := digest["minLength"].(int); bound != 64 {
		t.Errorf("the material digest must pin the 64-hex bound, got %v", digest["minLength"])
	}
	if bound, _ := digest["maxLength"].(int); bound != 64 {
		t.Errorf("the material digest must pin the 64-hex bound, got %v", digest["maxLength"])
	}
}

// contains reports whether the text carries the word (the local probe of
// the honest descriptions).
func contains(text, word string) bool {
	return len(word) > 0 && len(text) >= len(word) && (text == word || len(text) > 0 && indexOf(text, word) >= 0)
}

func indexOf(text, word string) int {
	for i := 0; i+len(word) <= len(text); i++ {
		if text[i:i+len(word)] == word {
			return i
		}
	}
	return -1
}
