package openapi_test

import (
	"os"
	"testing"

	"github.com/artemiosu/staffy/internal/tools"
	"gopkg.in/yaml.v3"
)

// TestToolsCatalogContractIsHonest pins the Story 7.2 slice wire
// contract: the two read-only GET paths carry the principal boundary and
// the honest answer and refusal codes (200/401/403/404 — a non-member is
// the plain 404, a member without the integrations|configuration scopes
// is the honest 403; the list carries no material to refuse with a 400);
// the list documents the HONEST EMPTY answer (the AG-4 state, never a
// null); the pin path carries the strict path-parameter contracts (the
// identifier charset and the numeric dotted triple — an invented pin
// does not exist); the ToolContractView mirrors the platform
// dictionaries EXACTLY (determinism_class carries the SINGLE value
// deterministic_read — a write-class tool is structurally impossible;
// adapter_status carries the honest implemented|not_implemented pair;
// the schema key types carry the pack typing dictionary — drift turns
// this test red); the mutation surface does NOT exist (no
// POST/PUT/PATCH — the registry is append-only artifacts, the external
// author's path is a git PR).
func TestToolsCatalogContractIsHonest(t *testing.T) {
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
	listPath := mapping(t, paths, "/v1/tenants/{tenant_id}/tools")
	listGet := mapping(t, listPath, "get")
	if listGet["operationId"] != "listToolContracts" {
		t.Errorf("the list operation id is wrong: %v", listGet["operationId"])
	}
	if required, _ := listGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the tools list does not document the principal boundary")
	}
	listResponses := mapping(t, listGet, "responses")
	for _, code := range []string{"200", "401", "403", "404"} {
		if _, documented := listResponses[code]; !documented {
			t.Errorf("the tools list does not document the honest %s answer", code)
		}
	}
	if _, documented := listResponses["400"]; documented {
		t.Errorf("the tools list must not document a 400: the read carries no material to refuse")
	}
	for _, method := range []string{"post", "put", "patch", "delete"} {
		if _, exists := listPath[method]; exists {
			t.Errorf("the tool registry is append-only artifacts: the %s mutation method must not exist", method)
		}
	}
	listOK := mapping(t, listResponses, "200")
	responseSchema := mapping(t, mapping(t, mapping(t, listOK, "content"), "application/json"), "schema")
	responseRequired, _ := responseSchema["required"].([]any)
	if !requiredWord(responseRequired, "tools") {
		t.Errorf("the list answer must carry the required tools wrapper, got %v", responseRequired)
	}
	listItems, _ := mapping(t, mapping(t, responseSchema, "properties"), "tools")["items"].(map[string]any)
	if ref, _ := listItems["$ref"].(string); ref != "#/components/schemas/ToolContractView" {
		t.Errorf("the list items must reference ToolContractView, got %v", ref)
	}

	// The pin path: the strict path-parameter contracts and the same
	// honest codes; the pin of the past stays resolvable.
	pinPath := mapping(t, paths, "/v1/tenants/{tenant_id}/tools/{tool_id}/{tool_version}")
	pinGet := mapping(t, pinPath, "get")
	if pinGet["operationId"] != "getToolContract" {
		t.Errorf("the pin operation id is wrong: %v", pinGet["operationId"])
	}
	if required, _ := pinGet["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the tools pin does not document the principal boundary")
	}
	pinResponses := mapping(t, pinGet, "responses")
	for _, code := range []string{"200", "401", "403", "404"} {
		if _, documented := pinResponses[code]; !documented {
			t.Errorf("the tools pin does not document the honest %s answer", code)
		}
	}
	pinParameters, _ := pinPath["parameters"].([]any)
	for _, parameter := range pinParameters {
		param, _ := parameter.(map[string]any)
		if param["name"] == "tool_id" {
			schema := mapping(t, param, "schema")
			if pattern, _ := schema["pattern"].(string); pattern != "^[a-z0-9._-]+$" {
				t.Errorf("the tool_id pattern is wrong: %v", schema["pattern"])
			}
		}
		if param["name"] == "tool_version" {
			schema := mapping(t, param, "schema")
			if pattern, _ := schema["pattern"].(string); pattern != "^[0-9]+\\.[0-9]+\\.[0-9]+$" {
				t.Errorf("the tool_version pattern is wrong: %v", schema["pattern"])
			}
		}
	}

	// The ToolContractView: the enum mirrors of the platform dictionaries.
	view := mapping(t, schemas, "ToolContractView")
	properties := mapping(t, view, "properties")
	determinism := mapping(t, properties, "determinism_class")
	if !enumMatches(t, determinism, tools.DeterminismClasses()) {
		t.Errorf("determinism_class enum must mirror the platform dictionary EXACTLY: %v (write-класс структурно невозможен)", determinism["enum"])
	}
	adapter := mapping(t, properties, "adapter_status")
	if !enumMatches(t, adapter, tools.AdapterStatuses()) {
		t.Errorf("adapter_status enum must mirror the platform dictionary EXACTLY: %v", adapter["enum"])
	}
	fieldView := mapping(t, schemas, "ToolSchemaFieldView")
	fieldType := mapping(t, mapping(t, fieldView, "properties"), "type")
	if !enumMatches(t, fieldType, tools.ToolFieldTypes()) {
		t.Errorf("the schema key type enum must mirror the platform typing dictionary EXACTLY: %v", fieldType["enum"])
	}
	viewRequired, _ := view["required"].([]any)
	for _, word := range []string{"id", "version", "name", "purpose", "determinism_class", "adapter_status", "owner", "evidence_ref", "input_schema", "output_schema"} {
		if !requiredWord(viewRequired, word) {
			t.Errorf("ToolContractView must require %s, got %v", word, viewRequired)
		}
	}
	if _, documented := properties["adapter_build"]; documented {
		t.Errorf("ToolContractView must not carry fields the contract does not have")
	}
}

// enumMatches checks the yaml enum node word-by-word against the
// dictionary (the exact set, the exact length — no extra, no missing).
func enumMatches(t *testing.T, schema map[string]any, dictionary []string) bool {
	t.Helper()
	words, _ := schema["enum"].([]any)
	if len(words) != len(dictionary) {
		return false
	}
	counts := map[string]int{}
	for _, word := range words {
		text, _ := word.(string)
		counts[text]++
	}
	for _, dictionaryWord := range dictionary {
		if counts[dictionaryWord] != 1 {
			return false
		}
	}
	return true
}
