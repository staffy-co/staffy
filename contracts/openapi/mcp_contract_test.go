package openapi_test

import (
	"os"
	"testing"

	"github.com/artemiosu/staffy/internal/mcp"
	"github.com/artemiosu/staffy/internal/tools"
	"gopkg.in/yaml.v3"
)

// words reads the string list of a yaml enum value.
func words(t *testing.T, value any) []string {
	t.Helper()
	list, ok := value.([]any)
	if !ok {
		t.Fatalf("значение не является списком: %v (%T)", value, value)
	}
	out := make([]string, 0, len(list))
	for _, entry := range list {
		word, ok := entry.(string)
		if !ok {
			t.Fatalf("элемент списка не строка: %v (%T)", entry, entry)
		}
		out = append(out, word)
	}
	return out
}

// TestMCPSurfaceContractIsHonest pins the Story 7.3 wire contract: the
// single machine POST path carries the principal boundary, the routing
// headers of the pinned revision with the exact method and version
// dictionaries, the declared transport budgets (413/415) and the honest
// protocol ladder (200 JSON-RPC, the bare 202 of the notification, the
// 400 of the invalid envelope, the membership problem codes, the 429
// with Retry-After); the GET/DELETE refusals document the streamless
// sessionless core; the envelope schemas are the closed forms; the
// mirrors hold: the projection type words against the platform typing
// dictionary, the protocol versions against the pinned revision list,
// the server identity against the OpenAPI info.version (a platform
// version change is a conscious step in both places).
func TestMCPSurfaceContractIsHonest(t *testing.T) {
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

	path := mapping(t, paths, "/v1/tenants/{tenant_id}/mcp")
	post := mapping(t, path, "post")
	if post["operationId"] != "callMcpSurface" {
		t.Errorf("the surface operation id is wrong: %v", post["operationId"])
	}
	if required, _ := post["x-staffy-principal-required"].(bool); !required {
		t.Errorf("the MCP surface does not document the principal boundary")
	}

	// The honest protocol ladder of the POST: exactly the declared codes.
	responses := mapping(t, post, "responses")
	for _, code := range []string{"200", "202", "400", "401", "403", "404", "413", "415", "429"} {
		if _, documented := responses[code]; !documented {
			t.Errorf("the MCP surface does not document the honest %s answer", code)
		}
	}
	if _, documented := responses["405"]; documented {
		t.Errorf("the POST itself never answers 405 — the method refusals live on their own operations")
	}
	retryAfter := mapping(t, mapping(t, responses, "429"), "headers")
	if _, hasRetry := retryAfter["Retry-After"]; !hasRetry {
		t.Errorf("the 429 must document the honest Retry-After of the declared window")
	}

	// The streamless, sessionless core: GET and DELETE answer the 405
	// only, by name.
	for _, method := range []string{"get", "delete"} {
		operation := mapping(t, path, method)
		methodResponses := mapping(t, operation, "responses")
		if len(methodResponses) != 1 {
			t.Errorf("the %s refusal must document exactly one answer, got %v", method, methodResponses)
		}
		if _, documented := methodResponses["405"]; !documented {
			t.Errorf("the %s refusal must document the 405", method)
		}
	}

	// The routing headers: the method dictionary and the version pin of
	// the revision.
	headerParameters := headerParameterMap(t, post)
	methodHeader := headerParameters["Mcp-Method"]
	if methodHeader == nil {
		t.Fatalf("the Mcp-Method header parameter is not documented")
	}
	methodEnum := words(t, mapping(t, methodHeader, "schema")["enum"])
	if len(methodEnum) != 3 || methodEnum[0] != mcp.MethodServerDiscover || methodEnum[1] != mcp.MethodToolsList || methodEnum[2] != mcp.MethodToolsCall {
		t.Errorf("the Mcp-Method dictionary is wrong: %v", methodEnum)
	}
	versionHeader := headerParameters["MCP-Protocol-Version"]
	if versionHeader == nil {
		t.Fatalf("the MCP-Protocol-Version header parameter is not documented")
	}
	versionEnum := words(t, mapping(t, versionHeader, "schema")["enum"])
	if len(versionEnum) != 1 || versionEnum[0] != mcp.ProtocolVersion {
		t.Errorf("the MCP-Protocol-Version pin is wrong: %v", versionEnum)
	}

	// The request envelope: the closed single-message form of the
	// revision with the mandatory version claim.
	envelope := mapping(t, schemas, "McpRequestEnvelope")
	envelopeProperties := mapping(t, envelope, "properties")
	for _, field := range []string{"jsonrpc", "id", "method", "params", "_meta"} {
		if _, carries := envelopeProperties[field]; !carries {
			t.Errorf("the request envelope must carry the %s member of the revision", field)
		}
	}
	if len(envelopeProperties) != 5 {
		t.Errorf("the request envelope is the closed five-member form, got %d members", len(envelopeProperties))
	}
	meta := mapping(t, envelopeProperties, "_meta")
	protocolVersionProperty := mapping(t, mapping(t, meta, "properties"), "protocolVersion")
	protocolVersionEnum := words(t, protocolVersionProperty["enum"])
	supported := mcp.SupportedProtocolVersions()
	if len(protocolVersionEnum) != len(supported) || protocolVersionEnum[0] != supported[0] {
		t.Errorf("the _meta version claim must pin exactly %v, got %v", supported, protocolVersionEnum)
	}

	// The response envelope: the closed result-or-error form.
	responseEnvelope := mapping(t, schemas, "McpResponseEnvelope")
	responseProperties := mapping(t, responseEnvelope, "properties")
	for _, field := range []string{"jsonrpc", "id", "result", "error"} {
		if _, carries := responseProperties[field]; !carries {
			t.Errorf("the response envelope must carry the %s member", field)
		}
	}
	if len(responseProperties) != 4 {
		t.Errorf("the response envelope is the closed four-member form, got %d members", len(responseProperties))
	}

	// The discover mirror: the exact supported list, the factual
	// capabilities, the server identity pinned to the OpenAPI
	// info.version (the conscious two-place step).
	discover := mapping(t, schemas, "McpDiscoverResult")
	discoverProperties := mapping(t, discover, "properties")
	protocolVersions := mapping(t, discoverProperties, "protocolVersions")
	discoverEnum := words(t, mapping(t, protocolVersions, "items")["enum"])
	if len(discoverEnum) != 1 || discoverEnum[0] != mcp.ProtocolVersion {
		t.Errorf("the discover answer must carry exactly [%s], got %v", mcp.ProtocolVersion, discoverEnum)
	}
	capabilities := mapping(t, discoverProperties, "capabilities")
	capabilityTools := mapping(t, mapping(t, capabilities, "properties"), "tools")
	listChanged := mapping(t, mapping(t, capabilityTools, "properties"), "listChanged")
	if listChangedEnum, ok := listChanged["enum"].([]any); !ok || len(listChangedEnum) != 1 || listChangedEnum[0] != false {
		t.Errorf("listChanged must be the factual false, got %v", listChanged["enum"])
	}
	serverInfo := mapping(t, schemas, "McpServerInfo")
	serverInfoProperties := mapping(t, serverInfo, "properties")
	nameEnum := words(t, mapping(t, serverInfoProperties, "name")["enum"])
	if len(nameEnum) != 1 || nameEnum[0] != mcp.ServerName {
		t.Errorf("the server name must be %q, got %v", mcp.ServerName, nameEnum)
	}
	info := mapping(t, document, "info")
	documentedVersion, _ := info["version"].(string)
	if documentedVersion != mcp.ServerVersion {
		t.Errorf("the serverInfo version must mirror the OpenAPI info.version: contract %q vs platform %q", documentedVersion, mcp.ServerVersion)
	}

	// The tool projection mirror: the JSON Schema property types are the
	// platform typing dictionary (drift turns this probe red), and the
	// read-only hint is the const fact of the safety dictionary.
	propertySchema := mapping(t, schemas, "McpJsonSchemaProperty")
	propertyTypes := words(t, mapping(t, mapping(t, propertySchema, "properties"), "type")["enum"])
	platformTypes := tools.ToolFieldTypes()
	if len(propertyTypes) != len(platformTypes) {
		t.Fatalf("the projection type dictionary drifted: contract %v vs platform %v", propertyTypes, platformTypes)
	}
	for i, platformType := range platformTypes {
		if propertyTypes[i] != platformType {
			t.Errorf("the projection type word %d drifted: contract %q vs platform %q", i, propertyTypes[i], platformType)
		}
	}
	toolSchema := mapping(t, schemas, "McpTool")
	toolProperties := mapping(t, toolSchema, "properties")
	annotations := mapping(t, toolProperties, "annotations")
	readOnlyHint := mapping(t, mapping(t, annotations, "properties"), "readOnlyHint")
	if hintEnum, ok := readOnlyHint["enum"].([]any); !ok || len(hintEnum) != 1 || hintEnum[0] != true {
		t.Errorf("readOnlyHint must be the const fact true, got %v", readOnlyHint["enum"])
	}
	for _, field := range []string{"name", "title", "description", "inputSchema", "outputSchema", "annotations"} {
		if _, carries := toolProperties[field]; !carries {
			t.Errorf("the tool projection must carry the %s member", field)
		}
	}

	// The call result: the typed structured content and the honest error
	// flag.
	callResult := mapping(t, schemas, "McpCallToolResult")
	callResultProperties := mapping(t, callResult, "properties")
	if _, carries := callResultProperties["structuredContent"]; !carries {
		t.Errorf("the call result must carry the typed structuredContent member")
	}
	if _, carries := callResultProperties["isError"]; !carries {
		t.Errorf("the call result must carry the honest isError member")
	}
}

// headerParameterMap collects the header parameters of one operation by
// name.
func headerParameterMap(t *testing.T, operation map[string]any) map[string]map[string]any {
	t.Helper()
	parameters, _ := operation["parameters"].([]any)
	out := map[string]map[string]any{}
	for _, entry := range parameters {
		parameter, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _ := parameter["name"].(string)
		if name == "" {
			continue
		}
		if in, _ := parameter["in"].(string); in != "header" {
			continue
		}
		out[name] = parameter
	}
	return out
}
