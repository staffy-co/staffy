package openapi_test

import (
	"os"
	"testing"

	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	"gopkg.in/yaml.v3"
)

// TestActionCatalogContractIsHonest pins the Story 3.1 slice-1 wire
// contract: the two read-only GET paths carry the principal boundary and
// the honest answer and refusal codes (200/401/403/404 — a non-member
// is the plain 404, an operator without the integrations|configuration
// scopes is the honest 403; the list carries no material to refuse with
// a 400); the list documents the HONEST EMPTY answer (the AG-4 state,
// never a null); the pin path carries the strict path-parameter
// contracts (the identifier charset and the numeric dotted triple — an
// invented pin does not exist); the ActionDefinitionView mirrors the
// domain dictionaries EXACTLY (every enum word is a compile-time
// dictionary word — drift turns this test red; the timeout bounds, the
// additionalProperties false, the required lists); the mutation surface
// does NOT exist (no POST/PUT/PATCH — the dictionary is compile-time,
// AC-9 satisfied structurally).
func TestActionCatalogContractIsHonest(t *testing.T) {
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

	// The list path: the principal boundary, the honest codes and the
	// honest empty answer; no mutation method on the path.
	listPath := mapping(t, paths, "/v1/tenants/{tenant_id}/action-definitions")
	listGet := mapping(t, listPath, "get")
	if listGet["operationId"] != "listActionDefinitions" {
		t.Errorf("the list operation id is wrong: %v", listGet["operationId"])
	}
	if required, _ := listGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the action-definitions list does not document the principal boundary")
	}
	listResponses := mapping(t, listGet, "responses")
	for _, code := range []string{"200", "401", "403", "404"} {
		if _, documented := listResponses[code]; !documented {
			t.Errorf("the action-definitions list does not document the honest %s answer", code)
		}
	}
	if _, documented := listResponses["400"]; documented {
		t.Errorf("the action-definitions list must not document a 400: the read carries no material to refuse")
	}
	for _, method := range []string{"post", "put", "patch", "delete"} {
		if _, exists := listPath[method]; exists {
			t.Errorf("the action dictionary is compile-time: the %s mutation method must not exist (AC-9)", method)
		}
	}
	listOK := mapping(t, listResponses, "200")
	responseSchema := mapping(t, mapping(t, mapping(t, listOK, "content"), "application/json"), "schema")
	responseRequired, _ := responseSchema["required"].([]any)
	if !requiredWord(responseRequired, "action_definitions") {
		t.Errorf("the list answer must carry the required action_definitions wrapper, got %v", responseRequired)
	}
	listItems, _ := mapping(t, mapping(t, responseSchema, "properties"), "action_definitions")["items"].(map[string]any)
	if ref, _ := listItems["$ref"].(string); ref != "#/components/schemas/ActionDefinitionView" {
		t.Errorf("the list items must reference ActionDefinitionView, got %v", ref)
	}

	// The pin path: the strict path-parameter contracts and the same
	// honest codes; the pin of the past stays resolvable (AC-8).
	pinPath := mapping(t, paths, "/v1/tenants/{tenant_id}/action-definitions/{definition_id}/{definition_version}")
	pinGet := mapping(t, pinPath, "get")
	if pinGet["operationId"] != "getActionDefinition" {
		t.Errorf("the pin operation id is wrong: %v", pinGet["operationId"])
	}
	if required, _ := pinGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the action-definitions pin does not document the principal boundary")
	}
	pinResponses := mapping(t, pinGet, "responses")
	for _, code := range []string{"200", "401", "403", "404"} {
		if _, documented := pinResponses[code]; !documented {
			t.Errorf("the action-definitions pin does not document the honest %s answer", code)
		}
	}
	pinParameters, _ := pinPath["parameters"].([]any)
	boundaries := map[string]string{}
	for _, parameter := range pinParameters {
		item, _ := parameter.(map[string]any)
		if item == nil {
			continue
		}
		name, _ := item["name"].(string)
		schema, _ := item["schema"].(map[string]any)
		if name == "" || schema == nil {
			continue
		}
		if pattern, _ := schema["pattern"].(string); pattern != "" {
			boundaries[name] = pattern
		}
	}
	if boundaries["definition_id"] != "^[a-z0-9._-]+$" {
		t.Errorf("the definition id must carry the strict charset pattern, got %q", boundaries["definition_id"])
	}
	if boundaries["definition_version"] != "^[0-9]+\\.[0-9]+\\.[0-9]+$" {
		t.Errorf("the definition version must carry the numeric triple pattern, got %q", boundaries["definition_version"])
	}
	for _, method := range []string{"post", "put", "patch", "delete"} {
		if _, exists := pinPath[method]; exists {
			t.Errorf("the pin surface is read-only: the %s mutation method must not exist", method)
		}
	}

	// The view: the required core and the closed dictionaries.
	definition := mapping(t, schemas, "ActionDefinitionView")
	if additional, ok := definition["additionalProperties"].(bool); !ok || additional {
		t.Errorf("ActionDefinitionView must reject additionalProperties (false), got %v", definition["additionalProperties"])
	}
	definitionProperties := mapping(t, definition, "properties")
	owningSystem := mapping(t, definitionProperties, "owning_system")
	assertEnumMirrors(t, owningSystem["enum"], actionDomain.OwningSystemWords, "owning_system")

	conditions := mapping(t, schemas, "ActionConditionsView")
	conditionsProperties := mapping(t, conditions, "properties")
	assertEnumMirrors(t, mapping(t, conditionsProperties, "risk_class")["enum"], actionDomain.RiskClasses, "risk_class")
	assertEnumMirrors(t, mapping(t, conditionsProperties, "autonomy_class")["enum"], actionDomain.AutonomyClasses, "autonomy_class")
	assertEnumMirrors(t, mapping(t, conditionsProperties, "confirmation")["enum"], actionDomain.ConfirmationRequirements, "confirmation")
	assertEnumMirrors(t, mapping(t, conditionsProperties, "materiality_class")["enum"], actionDomain.MaterialityClasses, "materiality_class")
	assertEnumMirrors(t, mapping(t, conditionsProperties, "consumption_rule")["enum"], actionDomain.ConsumptionRules, "consumption_rule")
	assertEnumMirrors(t, mapping(t, mapping(t, conditionsProperties, "allowed_conversation_modes"), "items")["enum"], actionDomain.ConversationModeWords, "allowed_conversation_modes")
	assertEnumMirrors(t, mapping(t, mapping(t, conditionsProperties, "allowed_roles"), "items")["enum"], actionDomain.RoleWords, "allowed_roles")
	assertEnumMirrors(t, mapping(t, mapping(t, conditionsProperties, "required_gates"), "items")["enum"], actionDomain.GateIDWords, "required_gates")
	assertEnumMirrors(t, mapping(t, mapping(t, conditionsProperties, "allowed_stages"), "items")["enum"], actionDomain.AllowedStages, "allowed_stages")

	facts := mapping(t, schemas, "ActionFactRequirementView")
	factsProperties := mapping(t, facts, "properties")
	assertEnumMirrors(t, mapping(t, factsProperties, "material_field")["enum"], actionDomain.MaterialFieldWords, "material_field")
	assertEnumMirrors(t, mapping(t, factsProperties, "freshness")["enum"], actionDomain.FreshnessWords, "freshness")

	delivery := mapping(t, schemas, "ActionDeliveryContractView")
	deliveryProperties := mapping(t, delivery, "properties")
	assertEnumMirrors(t, mapping(t, deliveryProperties, "capability_class")["enum"], actionDomain.CapabilityClasses, "capability_class")

	field := mapping(t, schemas, "ActionFieldSchemaView")
	assertEnumMirrors(t, mapping(t, mapping(t, field, "properties"), "type")["enum"], []string{"text", "url"}, "field_type")

	// The timeout bounds: the contract carries the domain bounds.
	timeout := mapping(t, conditionsProperties, "timeout_ms")
	if minimum, _ := timeout["minimum"].(int); minimum != actionDomain.TimeoutMSMin {
		t.Errorf("the timeout minimum must mirror the domain bound %d, got %v", actionDomain.TimeoutMSMin, timeout["minimum"])
	}
	if maximum, _ := timeout["maximum"].(int); maximum != actionDomain.TimeoutMSMax {
		t.Errorf("the timeout maximum must mirror the domain bound %d, got %v", actionDomain.TimeoutMSMax, timeout["maximum"])
	}

	// The complete condition system is required: «отсутствие обязательного
	// условия = запрет» lives in the contract's required list.
	if required, _ := conditions["required"].([]any); len(required) != 11 {
		t.Errorf("the complete condition system must be required (11 fields), got %v", required)
	}
	if required, _ := delivery["required"].([]any); len(required) != 5 {
		t.Errorf("the complete delivery contract must be required (5 words), got %v", required)
	}
}

// assertEnumMirrors pins one contract enum against the domain dictionary
// exactly (length AND membership — no silent extra words, no missing
// words: drift turns the test red).
func assertEnumMirrors(t *testing.T, raw any, dictionary []string, label string) {
	t.Helper()
	words, ok := raw.([]any)
	if !ok {
		t.Fatalf("%s: the contract does not carry an enum", label)
	}
	if len(words) != len(dictionary) {
		t.Fatalf("%s: the contract(%d words) does not mirror the domain(%d words): %v vs %v", label, len(words), len(dictionary), words, dictionary)
	}
	for _, word := range dictionary {
		found := false
		for _, candidate := range words {
			if text, _ := candidate.(string); text == word {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s: the domain word %q is missing from the contract %v", label, word, words)
		}
	}
}

// requiredWord reports whether the required list of one schema carries
// the word (the wrapper of the honest answer).
func requiredWord(required []any, word string) bool {
	for _, candidate := range required {
		if text, _ := candidate.(string); text == word {
			return true
		}
	}
	return false
}
