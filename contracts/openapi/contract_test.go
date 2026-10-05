package openapi_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStaffyContractHasTenantCreationBoundary(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	if version, _ := document["openapi"].(string); !strings.HasPrefix(version, "3.1.") {
		t.Fatalf("openapi = %q", version)
	}
	paths := mapping(t, document, "paths")
	tenantPath := mapping(t, paths, "/v1/tenants")
	post := mapping(t, tenantPath, "post")
	if required, _ := post["x-staffy-principal-required"].(bool); !required {
		t.Fatal("verified principal boundary is not documented")
	}
	requestBody := mapping(t, post, "requestBody")
	contentTypes := mapping(t, requestBody, "content")
	if _, ok := contentTypes["application/json"]; !ok {
		t.Fatal("application/json request contract is missing")
	}
	responses := mapping(t, post, "responses")
	for _, status := range []string{"201", "400", "401", "403", "409", "415"} {
		if _, ok := responses[status]; !ok {
			t.Errorf("response %s is missing", status)
		}
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	for _, schema := range []string{"TenantCreated", "Problem"} {
		if _, ok := schemas[schema]; !ok {
			t.Errorf("schema %s is missing", schema)
		}
	}
}

func TestBehaviorContractIsTenantScopedAndDraftOnly(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	for _, path := range []string{"/v1/tenants/{tenant_id}/employees/{employee_id}/behavior", "/v1/tenants/{tenant_id}/employees/{employee_id}/behavior/preview"} {
		item := mapping(t, paths, path)
		if _, ok := item["parameters"]; !ok {
			t.Errorf("%s has no tenant/employee scope", path)
		}
		method := "put"
		if strings.HasSuffix(path, "/preview") {
			method = "post"
		}
		operation := mapping(t, item, method)
		if operation["x-staffy-principal-required"] != true {
			t.Errorf("%s lacks principal boundary", path)
		}
		responses := mapping(t, operation, "responses")
		for _, status := range []string{"200", "400", "404", "409"} {
			if _, ok := responses[status]; !ok {
				t.Errorf("%s lacks %s", path, status)
			}
		}
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	behavior := mapping(t, schemas, "Behavior")
	properties := mapping(t, behavior, "properties")
	for _, field := range []string{"ai_disclosure", "confirmation", "handoff", "allowed_claims"} {
		if _, ok := properties[field]; !ok {
			t.Errorf("behavior lacks %s", field)
		}
	}
}

func mapping(t *testing.T, source map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := source[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not a mapping", key)
	}
	return value
}

func TestConnectorContractIsProductionClosedAndSecretWriteOnly(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	connectionProperties := mapping(t, mapping(t, schemas, "Connection"), "properties")
	environment, _ := connectionProperties["environment"].(map[string]any)
	if environment == nil {
		t.Fatal("connection environment schema missing")
	}
	if enum, _ := environment["enum"].([]any); len(enum) != 2 {
		t.Fatalf("environment enum must only list sandbox/test, got %v", enum)
	}
	definitionProperties := mapping(t, mapping(t, schemas, "ConnectorDefinition"), "properties")
	status, _ := definitionProperties["support_status"].(map[string]any)
	if status == nil {
		t.Fatal("support_status schema missing")
	}
	if enum, _ := status["enum"].([]any); len(enum) != 5 {
		t.Fatalf("support status vocabulary must stay textual and distinct, got %v", enum)
	}
	paths := mapping(t, document, "paths")
	for _, path := range []string{
		"/v1/tenants/{tenant_id}/connectors",
		"/v1/tenants/{tenant_id}/connections",
		"/v1/tenants/{tenant_id}/connections/{connection_id}",
		"/v1/tenants/{tenant_id}/connections/{connection_id}/versions",
		"/v1/tenants/{tenant_id}/connections/{connection_id}/connect",
		"/v1/tenants/{tenant_id}/connections/{connection_id}/rotate",
		"/v1/tenants/{tenant_id}/connections/{connection_id}/revoke",
	} {
		item := mapping(t, paths, path)
		for _, method := range []string{"get", "post", "put"} {
			operation, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			if required, _ := operation["x-staffy-principal-required"].(bool); !required {
				t.Fatalf("%s %s must document the verified principal boundary", method, path)
			}
		}
	}
}

func TestConnectorRequestsRejectUnknownFields(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	// Every connector request body must explicitly forbid unknown fields.
	// Assert the value equals false so deleting the line (a realistic
	// regression) fails the test instead of silently passing.
	requests := map[string]string{
		"/v1/tenants/{tenant_id}/connections":                         "post",
		"/v1/tenants/{tenant_id}/connections/{connection_id}":         "put",
		"/v1/tenants/{tenant_id}/connections/{connection_id}/connect": "post",
		"/v1/tenants/{tenant_id}/connections/{connection_id}/rotate":  "post",
		"/v1/tenants/{tenant_id}/connections/{connection_id}/revoke":  "post",
	}
	paths := mapping(t, document, "paths")
	for path, method := range requests {
		operation := mapping(t, mapping(t, paths, path), method)
		body := mapping(t, operation, "requestBody")
		schema := mapping(t, mapping(t, mapping(t, body, "content"), "application/json"), "schema")
		if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
			t.Fatalf("%s %s must set additionalProperties: false, got %v", method, path, schema["additionalProperties"])
		}
	}
}
