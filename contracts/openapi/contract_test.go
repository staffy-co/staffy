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

func mapping(t *testing.T, source map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := source[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not a mapping", key)
	}
	return value
}
