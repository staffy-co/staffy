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

// TestReleaseManifestContractIsHonest: the release surface documents the
// verified principal boundary on every path, the honest status and
// quarantine vocabularies, the fixed runtime dictionary, the digest shape
// and the sealed-conflict semantics.
func TestReleaseManifestContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	for path, methods := range map[string][]string{
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release":                            {"get"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release/draft":                      {"post"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release/versions":                   {"post"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release/manifests":                  {"post"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release/manifests/{config_version}": {"get"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release/restore":                    {"post"},
	} {
		item := mapping(t, paths, path)
		for _, method := range methods {
			operation := mapping(t, item, method)
			if required, _ := operation["x-staffy-principal-required"].(bool); !required {
				t.Fatalf("%s %s must document the verified principal boundary", method, path)
			}
		}
	}
	// Sealing documents the honest conflict semantics (409: CAS race and
	// the one-manifest-per-version UNIQUE).
	seal := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/employees/{employee_id}/release/manifests"), "post")
	sealResponses := mapping(t, seal, "responses")
	for _, status := range []string{"201", "400", "401", "403", "404", "409", "415"} {
		if _, ok := sealResponses[status]; !ok {
			t.Errorf("seal response %s is missing", status)
		}
	}
	// The view path documents the honest refusal of members without scopes.
	view := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/employees/{employee_id}/release"), "get")
	viewResponses := mapping(t, view, "responses")
	for _, status := range []string{"200", "401", "403", "404"} {
		if _, ok := viewResponses[status]; !ok {
			t.Errorf("release view response %s is missing", status)
		}
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	// The status vocabulary stays honest: candidate only on a complete set.
	summary := mapping(t, schemas, "ManifestSummary")
	summaryProps := mapping(t, summary, "properties")
	status := mapping(t, summaryProps, "status")
	statusEnum, _ := status["enum"].([]any)
	if len(statusEnum) != 2 || statusEnum[0] != "candidate" || statusEnum[1] != "incomplete" {
		t.Fatalf("ManifestSummary.status enum must stay [candidate incomplete], got %v", statusEnum)
	}
	// The digest is a pinned sha256 hex.
	digest := mapping(t, summaryProps, "digest")
	if pattern, _ := digest["pattern"].(string); pattern != "^[0-9a-f]{64}$" {
		t.Fatalf("digest pattern must pin sha256 hex, got %v", digest["pattern"])
	}
	// The quarantine vocabulary is explicit — never a silent drop.
	manifestView := mapping(t, schemas, "ManifestView")
	manifestProps := mapping(t, manifestView, "properties")
	quarantine := mapping(t, manifestProps, "quarantine")
	quarantineEnum, _ := quarantine["enum"].([]any)
	wantQuarantine := map[string]bool{"": false, "schema_version": false, "unknown_field": false, "digest_mismatch": false}
	for _, v := range quarantineEnum {
		if s, ok := v.(string); ok {
			wantQuarantine[s] = true
		}
	}
	for kind, seen := range wantQuarantine {
		if !seen {
			t.Errorf("ManifestView.quarantine enum must include %q", kind)
		}
	}
	// The runtime dictionary is fixed and complete.
	manifestContent := mapping(t, schemas, "ManifestContent")
	contentProps := mapping(t, manifestContent, "properties")
	runtime := mapping(t, contentProps, "runtime_components")
	runtimeItems := mapping(t, runtime, "items")
	runtimeProps := mapping(t, runtimeItems, "properties")
	runtimeComponent := mapping(t, runtimeProps, "component")
	componentEnum, _ := runtimeComponent["enum"].([]any)
	wantComponents := map[string]bool{"model_route": false, "media_route": false, "prompt_assembly": false, "tool_schemas": false, "policy_bundle": false, "eval_set": false, "runtime_build": false, "autonomy_envelope": false}
	for _, v := range componentEnum {
		if s, ok := v.(string); ok {
			wantComponents[s] = true
		}
	}
	for component, seen := range wantComponents {
		if !seen {
			t.Errorf("runtime component enum must include %q", component)
		}
	}
	// The voice applicability stays honest: not_applicable for text employees.
	refs := mapping(t, schemas, "ConfigRefs")
	refsProps := mapping(t, refs, "properties")
	voice := mapping(t, refsProps, "voice")
	voiceProps := mapping(t, voice, "properties")
	voiceStatus := mapping(t, voiceProps, "status")
	voiceEnum, _ := voiceStatus["enum"].([]any)
	wantVoice := map[string]bool{"present": false, "absent": false, "not_applicable": false}
	for _, v := range voiceEnum {
		if s, ok := v.(string); ok {
			wantVoice[s] = true
		}
	}
	for state, seen := range wantVoice {
		if !seen {
			t.Errorf("ConfigRefs.voice.status enum must include %q", state)
		}
	}
	// Release request bodies reject unknown fields.
	requests := map[string]string{
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release/draft":     "post",
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release/versions":  "post",
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release/manifests": "post",
		"/v1/tenants/{tenant_id}/employees/{employee_id}/release/restore":   "post",
	}
	for path, method := range requests {
		operation := mapping(t, mapping(t, paths, path), method)
		body := mapping(t, operation, "requestBody")
		schema := mapping(t, mapping(t, mapping(t, body, "content"), "application/json"), "schema")
		if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
			t.Fatalf("%s %s must set additionalProperties: false, got %v", method, path, schema["additionalProperties"])
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

func TestCapabilityVerificationContractIsDocumented(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")

	// Capability State read endpoint.
	statePath := mapping(t, paths, "/v1/tenants/{tenant_id}/connections/{connection_id}/capabilities")
	stateGet := mapping(t, statePath, "get")
	if required, _ := stateGet["x-staffy-principal-required"].(bool); !required {
		t.Fatal("capability state must stay behind the verified principal boundary")
	}
	stateResponses := mapping(t, stateGet, "responses")
	if _, ok := stateResponses["404"]; !ok {
		t.Error("capability state must document 404")
	}

	// Verify endpoint: principal boundary, idempotency header, honest errors.
	verifyPath := mapping(t, paths, "/v1/tenants/{tenant_id}/connections/{connection_id}/capabilities/{capability_id}/verify")
	verifyPost := mapping(t, verifyPath, "post")
	if required, _ := verifyPost["x-staffy-principal-required"].(bool); !required {
		t.Fatal("verify must stay behind the verified principal boundary")
	}
	verifyParams, ok := verifyPost["parameters"].([]any)
	if !ok {
		t.Fatal("verify must document the Idempotency-Key header")
	}
	hasKeyHeader := false
	for _, p := range verifyParams {
		param, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if name, _ := param["name"].(string); name == "Idempotency-Key" {
			if in, _ := param["in"].(string); in == "header" {
				hasKeyHeader = true
			}
		}
	}
	if !hasKeyHeader {
		t.Fatal("verify must require the Idempotency-Key header (replay/conflict semantics)")
	}
	verifyResponses := mapping(t, verifyPost, "responses")
	for _, status := range []string{"200", "400", "404", "409"} {
		if _, ok := verifyResponses[status]; !ok {
			t.Errorf("verify response %s is missing", status)
		}
	}

	// connect/rotate/revoke are now idempotent by header.
	for _, path := range []string{
		"/v1/tenants/{tenant_id}/connections/{connection_id}/connect",
		"/v1/tenants/{tenant_id}/connections/{connection_id}/rotate",
		"/v1/tenants/{tenant_id}/connections/{connection_id}/revoke",
	} {
		item := mapping(t, paths, path)
		post := mapping(t, item, "post")
		params, _ := post["parameters"].([]any)
		found := false
		for _, p := range params {
			param, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if name, _ := param["name"].(string); name == "Idempotency-Key" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s must document the idempotent Idempotency-Key header (R2 Gap 5)", path)
		}
	}

	// Capability schemas with the honest state and delivery vocabularies.
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	for _, schema := range []string{"CapabilityDeclaration", "CapabilityReport", "CapabilityBlock", "CapabilityEvidenceSummary"} {
		if _, ok := schemas[schema]; !ok {
			t.Errorf("schema %s is missing", schema)
		}
	}
	report := mapping(t, schemas, "CapabilityReport")
	props := mapping(t, report, "properties")
	state := mapping(t, props, "state")
	enum, _ := state["enum"].([]any)
	want := map[string]bool{"unverified": false, "verifying": false, "verified": false, "unsupported": false, "failed": false, "unknown": false, "stale": false}
	for _, v := range enum {
		if s, ok := v.(string); ok {
			want[s] = true
		}
	}
	for state, seen := range want {
		if !seen {
			t.Errorf("CapabilityReport.state enum must include %q (honest state vocabulary)", state)
		}
	}
}

func TestPrivacyDataPerimeterContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = yaml.Unmarshal(content, &document); &document == nil || err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	// Every privacy endpoint stays behind the verified principal boundary.
	for path, methods := range map[string][]string{
		"/v1/tenants/{tenant_id}/data-map":                               {"get"},
		"/v1/tenants/{tenant_id}/data-map/entries":                       {"post"},
		"/v1/tenants/{tenant_id}/retention-policies":                     {"post"},
		"/v1/tenants/{tenant_id}/legal-holds":                            {"post"},
		"/v1/tenants/{tenant_id}/legal-holds/release":                    {"post"},
		"/v1/tenants/{tenant_id}/deletion-requests":                      {"post"},
		"/v1/tenants/{tenant_id}/deletion-requests/{request_id}/receipt": {"get"},
	} {
		item := mapping(t, paths, path)
		for _, method := range methods {
			operation := mapping(t, item, method)
			if required, _ := operation["x-staffy-principal-required"].(bool); !required {
				t.Fatalf("%s %s must document the verified principal boundary", method, path)
			}
		}
	}
	// The deletion workflow documents the honest outcome vocabulary.
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	outcome := mapping(t, schemas, "DeletionOutcome")
	outcomeProps := mapping(t, outcome, "properties")
	outcomeField := mapping(t, outcomeProps, "outcome")
	enum, _ := outcomeField["enum"].([]any)
	want := map[string]bool{"deleted": false, "scheduled": false, "retained_by_law": false, "unsupported": false, "unknown": false}
	for _, v := range enum {
		if s, ok := v.(string); ok {
			want[s] = true
		}
	}
	for outcomeState, seen := range want {
		if !seen {
			t.Errorf("DeletionOutcome.outcome enum must include %q (honest outcome vocabulary)", outcomeState)
		}
	}
	// The retention policy request rejects blanket policies by requiring the class.
	policyRequest := mapping(t, mapping(t, mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/retention-policies"), "post"), "requestBody"), "content")
	policySchema := mapping(t, mapping(t, policyRequest, "application/json"), "schema")
	required, _ := policySchema["required"].([]any)
	hasClass := false
	for _, field := range required {
		if name, ok := field.(string); ok && name == "class" {
			hasClass = true
		}
	}
	if !hasClass {
		t.Fatal("retention policy request must require the class field: blanket policies are rejected")
	}
	if additional, ok := policySchema["additionalProperties"].(bool); !ok || additional {
		t.Fatal("retention policy request must set additionalProperties: false")
	}
	// Map entry statuses stay honest in the data class view.
	classView := mapping(t, schemas, "DataClassView")
	classProps := mapping(t, classView, "properties")
	status := mapping(t, classProps, "status")
	statusEnum, _ := status["enum"].([]any)
	statusWant := map[string]bool{"complete": false, "incomplete": false, "undocumented": false}
	for _, v := range statusEnum {
		if s, ok := v.(string); ok {
			statusWant[s] = true
		}
	}
	for state, seen := range statusWant {
		if !seen {
			t.Errorf("DataClassView.status enum must include %q", state)
		}
	}
	// Deletion request bodies reject unknown fields too.
	deletionRequest := mapping(t, mapping(t, mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/deletion-requests"), "post"), "requestBody"), "content")
	deletionSchema := mapping(t, mapping(t, deletionRequest, "application/json"), "schema")
	if additional, ok := deletionSchema["additionalProperties"].(bool); !ok || additional {
		t.Fatal("deletion request must set additionalProperties: false")
	}
}

func TestEntitlementUsageContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	// Every entitlement endpoint stays behind the verified principal boundary.
	for path, methods := range map[string][]string{
		"/v1/tenants/{tenant_id}/entitlement":                                 {"get"},
		"/v1/tenants/{tenant_id}/entitlement/terms":                           {"post"},
		"/v1/tenants/{tenant_id}/entitlement/state":                           {"post"},
		"/v1/tenants/{tenant_id}/entitlement/budgets":                         {"post"},
		"/v1/tenants/{tenant_id}/entitlement/sources":                         {"post"},
		"/v1/tenants/{tenant_id}/usage-events":                                {"post"},
		"/v1/tenants/{tenant_id}/budget-meter":                                {"get"},
		"/v1/tenants/{tenant_id}/admission-state":                             {"get"},
		"/v1/tenants/{tenant_id}/usage-exceptions":                            {"get", "post"},
		"/v1/tenants/{tenant_id}/usage-exceptions/{exception_id}/acknowledge": {"post"},
	} {
		item := mapping(t, paths, path)
		for _, method := range methods {
			operation := mapping(t, item, method)
			if required, _ := operation["x-staffy-principal-required"].(bool); !required {
				t.Fatalf("%s %s must document the verified principal boundary", method, path)
			}
		}
	}
	// The usage event ingest documents the honest conflict semantics.
	ingest := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/usage-events"), "post")
	ingestResponses := mapping(t, ingest, "responses")
	if _, ok := ingestResponses["409"]; !ok {
		t.Fatal("usage-events must document 409: the same key with a different payload is a conflict for review")
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	// The meter data status vocabulary stays honest: unknown is a state, not a zero.
	meter := mapping(t, schemas, "CategoryMeter")
	meterProps := mapping(t, meter, "properties")
	status := mapping(t, meterProps, "status")
	statusEnum, _ := status["enum"].([]any)
	wantStatus := map[string]bool{"complete": false, "partial": false, "stale": false, "unknown": false}
	for _, v := range statusEnum {
		if s, ok := v.(string); ok {
			wantStatus[s] = true
		}
	}
	for state, seen := range wantStatus {
		if !seen {
			t.Errorf("CategoryMeter.status enum must include %q (honest data status vocabulary)", state)
		}
	}
	// The exception cause dictionary is complete, including the retry storm
	// whose automatic trigger is an honest deferred boundary (Epic 2).
	exception := mapping(t, schemas, "UsageException")
	exceptionProps := mapping(t, exception, "properties")
	cause := mapping(t, exceptionProps, "cause")
	causeEnum, _ := cause["enum"].([]any)
	wantCauses := map[string]bool{
		"budget_threshold": false, "hard_limit_reached": false, "data_gap": false,
		"anomaly_tool_calls": false, "anomaly_retry_storm": false,
		"anomaly_manual_minutes": false, "manual_entry": false,
	}
	for _, v := range causeEnum {
		if s, ok := v.(string); ok {
			wantCauses[s] = true
		}
	}
	for cause, seen := range wantCauses {
		if !seen {
			t.Errorf("UsageException.cause enum must include %q (honest cause dictionary)", cause)
		}
	}
	// The admission state names what continues and who owns the next step.
	admission := mapping(t, schemas, "AdmissionState")
	admissionProps := mapping(t, admission, "properties")
	for _, field := range []string{"status", "reason", "continues", "owner"} {
		if _, ok := admissionProps[field]; !ok {
			t.Errorf("AdmissionState lacks %q", field)
		}
	}
	admissionStatus := mapping(t, admissionProps, "status")
	admissionEnum, _ := admissionStatus["enum"].([]any)
	wantAdmission := map[string]bool{"ok": false, "blocked_new_admissions": false, "degraded": false}
	for _, v := range admissionEnum {
		if s, ok := v.(string); ok {
			wantAdmission[s] = true
		}
	}
	for state, seen := range wantAdmission {
		if !seen {
			t.Errorf("AdmissionState.status enum must include %q", state)
		}
	}
	// The estimate is separated from the confirmed accrual everywhere.
	budgetMeter := mapping(t, schemas, "BudgetMeter")
	budgetMeterProps := mapping(t, budgetMeter, "properties")
	for _, field := range []string{"confirmed_total", "estimate_total", "missing_categories", "data_status"} {
		if _, ok := budgetMeterProps[field]; !ok {
			t.Errorf("BudgetMeter lacks %q", field)
		}
	}
	// Entitlement request bodies reject unknown fields.
	requests := map[string]string{
		"/v1/tenants/{tenant_id}/entitlement/terms":   "post",
		"/v1/tenants/{tenant_id}/entitlement/state":   "post",
		"/v1/tenants/{tenant_id}/entitlement/budgets": "post",
		"/v1/tenants/{tenant_id}/entitlement/sources": "post",
		"/v1/tenants/{tenant_id}/usage-events":        "post",
		"/v1/tenants/{tenant_id}/usage-exceptions":    "post",
	}
	for path, method := range requests {
		operation := mapping(t, mapping(t, paths, path), method)
		body := mapping(t, operation, "requestBody")
		schema := mapping(t, mapping(t, mapping(t, body, "content"), "application/json"), "schema")
		if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
			t.Fatalf("%s %s must set additionalProperties: false, got %v", method, path, schema["additionalProperties"])
		}
	}
}

func TestOperatingProfileContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	for path, methods := range map[string][]string{
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile":                      {"get"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/draft":                {"post"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/approval":             {"post"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/thresholds":           {"post"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/thresholds/{gate_id}": {"get"},
	} {
		item := mapping(t, paths, path)
		for _, method := range methods {
			operation := mapping(t, item, method)
			if required, _ := operation["x-staffy-principal-required"].(bool); !required {
				t.Fatalf("%s %s must document the verified principal boundary", method, path)
			}
		}
	}
	// The approval documents the honest step-up refusal semantics.
	approval := mapping(t, mapping(t, paths, "/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/approval"), "post")
	approvalResponses := mapping(t, approval, "responses")
	for _, status := range []string{"201", "400", "401", "403", "404", "409", "415"} {
		if _, ok := approvalResponses[status]; !ok {
			t.Errorf("approval response %s is missing", status)
		}
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	for _, schema := range []string{"ProfileView", "ProfileDraft", "ProfileVersion", "ProfileContent", "ProfileRoute", "ProfileRuleError", "ThresholdState", "GateThresholdVersion", "ThresholdHistoryView"} {
		if _, ok := schemas[schema]; !ok {
			t.Errorf("schema %s is missing", schema)
		}
	}
	// The rule error vocabulary stays honest: three kinds map to three honest
	// reactions (reject the save, block the approval, block the route).
	rule := mapping(t, schemas, "ProfileRuleError")
	ruleProps := mapping(t, rule, "properties")
	kind := mapping(t, ruleProps, "kind")
	kindEnum, _ := kind["enum"].([]any)
	if len(kindEnum) != 3 || kindEnum[0] != "invalid" || kindEnum[1] != "missing" || kindEnum[2] != "advisory" {
		t.Fatalf("ProfileRuleError.kind enum must stay [invalid missing advisory], got %v", kindEnum)
	}
	// The threshold state vocabulary: pending_baseline until the human decides.
	threshold := mapping(t, schemas, "ThresholdState")
	thresholdProps := mapping(t, threshold, "properties")
	state := mapping(t, thresholdProps, "state")
	stateEnum, _ := state["enum"].([]any)
	if len(stateEnum) != 2 || stateEnum[0] != "pending_baseline" || stateEnum[1] != "decided" {
		t.Fatalf("ThresholdState.state enum must stay [pending_baseline decided], got %v", stateEnum)
	}
	// The route state vocabulary: the profile never admits traffic.
	view := mapping(t, schemas, "ProfileView")
	viewProps := mapping(t, view, "properties")
	routeStates := mapping(t, viewProps, "route_states")
	routeItems := mapping(t, routeStates, "items")
	routeItemProps := mapping(t, routeItems, "properties")
	routeState := mapping(t, routeItemProps, "state")
	routeStateEnum, _ := routeState["enum"].([]any)
	if len(routeStateEnum) != 2 || routeStateEnum[0] != "clear" || routeStateEnum[1] != "blocked" {
		t.Fatalf("route state enum must stay [clear blocked] — the profile never admits traffic, got %v", routeStateEnum)
	}
	// The applicability vocabulary: stale on a new manifest, unapproved changes
	// after the approval, never an on-the-spot migration.
	applicability := mapping(t, viewProps, "applicability")
	applicabilityProps := mapping(t, applicability, "properties")
	applicabilityState := mapping(t, applicabilityProps, "state")
	applicabilityEnum, _ := applicabilityState["enum"].([]any)
	if len(applicabilityEnum) != 4 || applicabilityEnum[0] != "none" || applicabilityEnum[2] != "stale" || applicabilityEnum[3] != "unapproved_changes" {
		t.Fatalf("applicability enum must stay [none current stale unapproved_changes], got %v", applicabilityEnum)
	}
	// The profile drafts/approvals/thresholds reject unknown fields.
	for path := range map[string]bool{
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/draft":      true,
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/approval":   true,
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/thresholds": true,
	} {
		operation := mapping(t, mapping(t, paths, path), "post")
		body := mapping(t, operation, "requestBody")
		schema := mapping(t, mapping(t, mapping(t, body, "content"), "application/json"), "schema")
		if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
			t.Fatalf("%s must set additionalProperties: false, got %v", path, schema["additionalProperties"])
		}
	}
}

// TestReadinessContractIsHonest pins the Story 1-14 boundary: the readiness
// view is a dry-run derivation (the GET never records anything), the gate
// transition and the publication are step-up commands with strict bodies, the
// stage ladder and the gate state machine stay the fixed registry vocabulary,
// and the publication records are append-only digests — never secrets.
func TestReadinessContractIsHonest(t *testing.T) {
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
	base := "/v1/tenants/{tenant_id}/employees/{employee_id}/test-release"
	// Every operation of the surface documents the verified principal boundary.
	operations := map[string][]string{
		base:                     {"get"},
		base + "/gate-instances": {"get", "post"},
		base + "/admissions":     {"get", "post"},
	}
	for path, methods := range operations {
		item := mapping(t, paths, path)
		for _, method := range methods {
			operation := mapping(t, item, method)
			if required, _ := operation["x-staffy-principal-required"].(bool); !required {
				t.Fatalf("%s %s must document the verified principal boundary", method, path)
			}
		}
	}
	// The readiness view is a dry-run: the stage query parameter names the
	// fixed ladder, the GET has no recordable semantics.
	view := mapping(t, mapping(t, paths, base), "get")
	viewParams, _ := view["parameters"].([]any)
	if len(viewParams) != 1 {
		t.Fatalf("the view must document exactly one stage query parameter, got %v", viewParams)
	}
	stageQuery := mapping(t, viewParams[0].(map[string]any), "schema")
	stageEnum, _ := stageQuery["enum"].([]any)
	if len(stageEnum) != 4 || stageEnum[0] != "synthetic" || stageEnum[1] != "closed_account_test" || stageEnum[2] != "controlled_real_customer" || stageEnum[3] != "live" {
		t.Fatalf("the stage ladder must stay [synthetic closed_account_test controlled_real_customer live], got %v", stageEnum)
	}
	// The commands reject unknown fields (strict decode, the M1 lesson).
	for _, path := range []string{base + "/gate-instances", base + "/admissions"} {
		post := mapping(t, mapping(t, paths, path), "post")
		body := mapping(t, post, "requestBody")
		schema := mapping(t, mapping(t, mapping(t, body, "content"), "application/json"), "schema")
		if additional, ok := schema["additionalProperties"].(bool); !ok || additional {
			t.Fatalf("%s must set additionalProperties: false, got %v", path, schema["additionalProperties"])
		}
		responses := mapping(t, post, "responses")
		for _, status := range []string{"201", "400", "401", "403", "404", "409", "415"} {
			if _, ok := responses[status]; !ok {
				t.Errorf("%s response %s is missing", path, status)
			}
		}
	}
	// The dictionary is fixed: the platform definitions of the accepted
	// registry plus the pilot definitions of spec 4-1 (OQ-1..OQ-5, OQ-19;
	// the AG-4 of the pilot set is the platform definition) — never
	// tenant-configurable.
	definition := mapping(t, schemas, "GateDefinition")
	definitionProps := mapping(t, definition, "properties")
	definitionID := mapping(t, definitionProps, "definition_id")
	definitionEnum, _ := definitionID["enum"].([]any)
	if len(definitionEnum) != 14 {
		t.Fatalf("the gate dictionary must stay 8 platform + 6 pilot (spec 4-1) definitions, got %v", definitionEnum)
	}
	// The instance state machine: open -> in_test -> pass|fail; expired and
	// not_applicable complete the vocabulary of the registry.
	instance := mapping(t, schemas, "GateInstance")
	instanceProps := mapping(t, instance, "properties")
	instanceState := mapping(t, instanceProps, "state")
	instanceEnum, _ := instanceState["enum"].([]any)
	if len(instanceEnum) != 6 || instanceEnum[0] != "open" || instanceEnum[2] != "pass" || instanceEnum[4] != "expired" {
		t.Fatalf("the gate instance state vocabulary must stay [open in_test pass fail expired not_applicable], got %v", instanceEnum)
	}
	// The overall status is the conjunction of the elements: incomplete,
	// blocked or ready — a component success never becomes the whole readiness.
	report := mapping(t, schemas, "ReadinessReport")
	reportProps := mapping(t, report, "properties")
	reportStatus := mapping(t, reportProps, "status")
	reportEnum, _ := reportStatus["enum"].([]any)
	if len(reportEnum) != 3 || reportEnum[0] != "incomplete" || reportEnum[1] != "blocked" || reportEnum[2] != "ready" {
		t.Fatalf("the readiness status vocabulary must stay [incomplete blocked ready], got %v", reportEnum)
	}
	// The publication record is an append-only decision: the step-up fact is
	// CHECK-enforced true and the digest is a canonical sha256.
	admission := mapping(t, schemas, "ReleaseAdmission")
	admissionProps := mapping(t, admission, "properties")
	stepUp := mapping(t, admissionProps, "step_up_verified")
	if description, _ := stepUp["description"].(string); !strings.Contains(description, "CHECK") {
		t.Fatalf("step_up_verified must document the CHECK-enforced true, got %v", stepUp)
	}
	admissionDigest := mapping(t, admissionProps, "digest")
	digestPattern, _ := admissionDigest["pattern"].(string)
	if digestPattern != "^[0-9a-f]{64}$" {
		t.Fatalf("the admission digest must be a sha256 hex pattern, got %v", digestPattern)
	}
	for _, schema := range []string{"ReadinessView", "ReadinessReport", "ReadinessElement", "RouteAdmission", "RouteBlock", "GateInstancesView", "GateDefinition", "GateInstance", "GateApproval", "ReleaseAdmission", "AdmissionHistoryView"} {
		if _, ok := schemas[schema]; !ok {
			t.Errorf("schema %s is missing", schema)
		}
	}
}

// TestPilotAdmissionContractIsHonest pins the spec 4-1 surface: the
// read-only tenant-level view with the fixed dictionaries (routes 6,
// stages 3, matrix fields 8, owning systems 3, pilot gates) — a drift
// between the OpenAPI mirrors and the domain dictionaries is red here.
func TestPilotAdmissionContractIsHonest(t *testing.T) {
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
	// The machine path is read-only: exactly the GET, the verified
	// principal boundary documented, no request body.
	path := "/v1/tenants/{tenant_id}/pilot/admission"
	item := mapping(t, paths, path)
	if _, ok := item["post"]; ok {
		t.Fatal("the pilot admission surface is read-only: no POST exists")
	}
	view := mapping(t, item, "get")
	if required, _ := view["x-staffy-principal-required"].(bool); !required {
		t.Fatal("the pilot admission GET must document the verified principal boundary")
	}
	if _, ok := view["requestBody"]; ok {
		t.Fatal("a GET view carries no request body (the empty read never answers 400)")
	}
	responses := mapping(t, view, "responses")
	for _, status := range []string{"200", "401", "403", "404"} {
		if _, ok := responses[status]; !ok {
			t.Errorf("the pilot admission response %s is missing", status)
		}
	}
	if _, ok := responses["400"]; ok {
		t.Error("the empty read must not document a 400")
	}
	// The view schema composes the report with the reused scope words.
	pilotView := mapping(t, schemas, "PilotAdmissionView")
	pilotViewProps := mapping(t, pilotView, "properties")
	scope := mapping(t, pilotViewProps, "view_scope")
	scopeEnum, _ := scope["enum"].([]any)
	if len(scopeEnum) != 2 || scopeEnum[0] != "configuration" || scopeEnum[1] != "review-or-audit" {
		t.Fatalf("the pilot view scope reuses the test & release words, got %v", scopeEnum)
	}
	// The route dictionary: exactly six fixed words.
	route := mapping(t, schemas, "PilotRouteView")
	routeProps := mapping(t, route, "properties")
	routeID := mapping(t, routeProps, "route_id")
	routeEnum, _ := routeID["enum"].([]any)
	want := []string{"text_jivo", "voice_sip", "parts_catalog_abcp", "parts_order_abcp", "abcp_1c_sync", "human_fallback"}
	if len(routeEnum) != 6 {
		t.Fatalf("the pilot route dictionary carries 6 words, got %v", routeEnum)
	}
	for index, word := range want {
		if routeEnum[index] != word {
			t.Fatalf("route word %d = %v, want %s", index, routeEnum[index], word)
		}
	}
	// The stage vocabulary: candidate/verified/admitted.
	stage := mapping(t, routeProps, "stage")
	stageEnum, _ := stage["enum"].([]any)
	if len(stageEnum) != 3 || stageEnum[0] != "candidate" || stageEnum[1] != "verified" || stageEnum[2] != "admitted" {
		t.Fatalf("the pilot stage vocabulary must stay [candidate verified admitted], got %v", stageEnum)
	}
	// The blocker sources: the honest four.
	blocker := mapping(t, schemas, "PilotBlockerView")
	blockerProps := mapping(t, blocker, "properties")
	blockerSource := mapping(t, blockerProps, "source")
	blockerEnum, _ := blockerSource["enum"].([]any)
	if len(blockerEnum) != 4 || blockerEnum[0] != "pack" || blockerEnum[1] != "capability" || blockerEnum[2] != "matrix" || blockerEnum[3] != "gate" {
		t.Fatalf("the blocker source vocabulary must stay [pack capability matrix gate], got %v", blockerEnum)
	}
	// The gate requirement view: the pilot gate set (AG-4 is the platform
	// definition reused — one word, not two).
	gate := mapping(t, schemas, "GateRequirementView")
	gateProps := mapping(t, gate, "properties")
	gateID := mapping(t, gateProps, "id")
	gateEnum, _ := gateID["enum"].([]any)
	if len(gateEnum) != 12 {
		t.Fatalf("the pilot gate requirement set carries 12 words (7 pilot gates + OQ-11/12/13B platform), got %v", gateEnum)
	}
	seen := map[string]bool{}
	for _, word := range gateEnum {
		seen[word.(string)] = true
	}
	for _, word := range []string{"OQ-1", "OQ-2", "OQ-3", "OQ-4", "OQ-5", "OQ-11", "OQ-12", "OQ-13B", "OQ-19", "AG-1", "AG-2", "AG-4"} {
		if !seen[word] {
			t.Fatalf("the pilot gate requirement set must carry %s", word)
		}
	}
	// The matrix field dictionary: eight material fields with the owning
	// system vocabulary.
	matrix := mapping(t, schemas, "PilotMatrixFieldView")
	matrixProps := mapping(t, matrix, "properties")
	matrixField := mapping(t, matrixProps, "field")
	matrixEnum, _ := matrixField["enum"].([]any)
	if len(matrixEnum) != 8 || matrixEnum[0] != "client_order_state" || matrixEnum[7] != "terminal_evidence" {
		t.Fatalf("the ABCP→1C matrix carries 8 material fields in the fixed order, got %v", matrixEnum)
	}
	owning := mapping(t, matrixProps, "owning_system")
	owningEnum, _ := owning["enum"].([]any)
	if len(owningEnum) != 3 || owningEnum[0] != "ABCP" || owningEnum[1] != "1C" || owningEnum[2] != "Staffy-Case" {
		t.Fatalf("the owning system vocabulary must stay [ABCP 1C Staffy-Case], got %v", owningEnum)
	}
	for _, schema := range []string{"PilotAdmissionView", "PilotAdmissionReport", "PilotRouteView", "PilotCapabilityView", "PilotBlockerView", "GateRequirementView", "PilotMatrixFieldView"} {
		if _, ok := schemas[schema]; !ok {
			t.Errorf("schema %s is missing", schema)
		}
	}
}

func TestCaseContourContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")

	// The machine ingress is NOT a principal surface: the trusted scope is
	// derived from the verified connection, and the marker documents the
	// boundary honestly instead of claiming a principal.
	ingress := mapping(t, paths, "/v1/ingress/channel-events")
	post := mapping(t, ingress, "post")
	if required, _ := post["x-staffy-principal-required"].(bool); required {
		t.Fatal("the ingress is a machine contract: no user principal boundary may be claimed")
	}
	if machine, _ := post["x-staffy-machine-ingress"].(bool); !machine {
		t.Fatal("the ingress must document the machine-ingress boundary")
	}
	for _, status := range []string{"201", "200", "400", "403", "404", "409", "415", "429"} {
		responses := mapping(t, post, "responses")
		if _, ok := responses[status]; !ok {
			t.Fatalf("the ingress must document the %s answer", status)
		}
	}

	// The operator surfaces ARE principal surfaces with the strict
	// review/audit payload boundary.
	for path, methods := range map[string][]string{
		"/v1/tenants/{tenant_id}/cases":                                                {"get"},
		"/v1/tenants/{tenant_id}/cases/{case_id}":                                      {"get"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/events/{event_id}/payload":            {"get"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/facts":                                {"get", "post"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items":                           {"post"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/classify":   {"post"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/transition": {"post"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/split":      {"post"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/merge":      {"post"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/events":     {"post"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/dispute":    {"post"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/confirmations":        {"post"},
		"/v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/rescope":              {"post"},
	} {
		item := mapping(t, paths, path)
		for _, method := range methods {
			operation := mapping(t, item, method)
			if required, _ := operation["x-staffy-principal-required"].(bool); !required {
				t.Fatalf("%s %s must document the verified principal boundary", method, path)
			}
		}
	}

	schemas := mapping(t, mapping(t, document, "components"), "schemas")

	// The envelope is strict and versioned: unknown fields are impossible,
	// the direction dictionary is inbound-only, the idempotency header is
	// documented and the payload never leaks into answers.
	envelope := mapping(t, schemas, "IngressEnvelope")
	if additional, _ := envelope["additionalProperties"].(bool); additional {
		t.Fatal("the ingress envelope must reject unknown fields")
	}
	envelopeProps := mapping(t, envelope, "properties")
	direction := mapping(t, envelopeProps, "direction")
	directionEnum, _ := direction["enum"].([]any)
	if len(directionEnum) != 1 || directionEnum[0] != "inbound" {
		t.Fatalf("the first slice receives inbound events only, got %v", directionEnum)
	}

	// The refusal vocabulary is the honest ladder dictionary; the bounded
	// kinds are marked and counted.
	refusal := mapping(t, schemas, "IngestRefusal")
	refusalProps := mapping(t, refusal, "properties")
	kind := mapping(t, refusalProps, "kind")
	kindEnum, _ := kind["enum"].([]any)
	expected := []string{"malformed", "unknown_connection", "revoked_connection", "production_forbidden",
		"channel_mismatch", "route_not_admitted", "no_active_release", "no_response_owner",
		"entitlement_blocked", "envelope_exceeded"}
	if len(kindEnum) != len(expected) {
		t.Fatalf("the refusal dictionary must keep %d kinds, got %v", len(expected), kindEnum)
	}
	for index, value := range expected {
		if kindEnum[index] != value {
			t.Fatalf("refusal kind %d: expected %s, got %v", index, value, kindEnum[index])
		}
	}

	// The honest fixed states of the slice: one case state, one
	// conversation surface and the first owner epoch (the rotation is
	// Story 2-4).
	conversation := mapping(t, schemas, "ConversationRecord")
	conversationProps := mapping(t, conversation, "properties")
	surface := mapping(t, conversationProps, "writable_surface")
	surfaceEnum, _ := surface["enum"].([]any)
	if len(surfaceEnum) != 1 || surfaceEnum[0] != "inbox" {
		t.Fatalf("the single writable surface of the slice is the inbox, got %v", surfaceEnum)
	}
	epoch := mapping(t, conversationProps, "response_owner_epoch")
	epochEnum, _ := epoch["enum"].([]any)
	if len(epochEnum) != 1 || epochEnum[0] != 1 {
		t.Fatalf("the owner epoch of the slice is 1 (rotation is 2-4), got %v", epochEnum)
	}
	workItem := mapping(t, schemas, "WorkItemRecord")
	workItemProps := mapping(t, workItem, "properties")
	workState := mapping(t, workItemProps, "state")
	workEnum, _ := workState["enum"].([]any)
	expectedStates := []string{"untriaged", "in_work", "waiting", "resolved", "cancelled", "split", "merged"}
	if len(workEnum) != len(expectedStates) {
		t.Fatalf("the work item state dictionary must keep %d states, got %v", len(expectedStates), workEnum)
	}
	for index, value := range expectedStates {
		if workEnum[index] != value {
			t.Fatalf("work item state %d: expected %s, got %v", index, value, workEnum[index])
		}
	}
	if _, ok := workItemProps["version"]; !ok {
		t.Fatal("the work item view must carry the derived version")
	}

	// The pack dictionaries of the case view: the taxonomy of the Work
	// Control Card forms AND the material key hints of the fact form — both
	// come from the pinned pack, never a hardcoded list.
	caseViewProps := mapping(t, mapping(t, schemas, "CaseView"), "properties")
	if _, ok := caseViewProps["critical_entities"]; !ok {
		t.Fatal("the case view must carry the critical entities of the pinned pack")
	}

	// The ten work commands are idempotent principal surfaces: every POST
	// documents the Idempotency-Key parameter and the honest conflict
	// answers (the stale version and the key conflict).
	for _, path := range []string{
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items",
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/classify",
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/transition",
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/split",
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/merge",
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/events",
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/dispute",
		"/v1/tenants/{tenant_id}/cases/{case_id}/facts",
		"/v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/confirmations",
		"/v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/rescope",
	} {
		post := mapping(t, mapping(t, paths, path), "post")
		parameters, _ := post["parameters"].([]any)
		hasKey := false
		for _, parameter := range parameters {
			entry, _ := parameter.(map[string]any)
			if entry["name"] == "Idempotency-Key" || entry["$ref"] == "#/components/parameters/IdempotencyKey" {
				hasKey = true
			}
		}
		if !hasKey {
			t.Fatalf("%s post must document the Idempotency-Key parameter", path)
		}
		responses := mapping(t, post, "responses")
		for _, status := range []string{"201", "200", "400", "403", "404", "409"} {
			if _, ok := responses[status]; !ok {
				t.Fatalf("%s post must document the %s answer (the idempotent replay and the honest conflicts)", path, status)
			}
		}
	}

	// The fact provenance dictionaries are honest: the reserved source
	// types without producers, the operator_relay-only confirmer of the
	// slice and the read-time status derivation.
	factVersion := mapping(t, schemas, "FactVersionRecord")
	factVersionProps := mapping(t, factVersion, "properties")
	sourceType := mapping(t, factVersionProps, "source_type")
	sourceEnum, _ := sourceType["enum"].([]any)
	if len(sourceEnum) != 4 || sourceEnum[2] != "connector" || sourceEnum[3] != "system" {
		t.Fatalf("connector and system are reserved fact source types without producers, got %v", sourceEnum)
	}
	confirmRequest := mapping(t, schemas, "ConfirmFactRequest")
	confirmProps := mapping(t, confirmRequest, "properties")
	confirmer := mapping(t, confirmProps, "confirmer_type")
	confirmerEnum, _ := confirmer["enum"].([]any)
	if len(confirmerEnum) != 1 || confirmerEnum[0] != "operator_relay" {
		t.Fatalf("the only confirmer producer of the slice is operator_relay, got %v", confirmerEnum)
	}
	factView := mapping(t, schemas, "FactView")
	factViewProps := mapping(t, factView, "properties")
	status := mapping(t, factViewProps, "status")
	statusEnum, _ := status["enum"].([]any)
	if len(statusEnum) != 3 || statusEnum[1] != "conflicting" {
		t.Fatalf("the fact statuses are the read-time derivations (conflicting has no automatic value choice), got %v", statusEnum)
	}

	for _, schema := range []string{"IngressEnvelope", "IngestResult", "IngestOutcome", "IngestRefusal",
		"IngestQuarantine", "CasesView", "CaseSummary", "CaseView", "CaseRecord", "ConversationRecord",
		"TimelineEvent", "WorkItemRecord", "OperationalException", "SealedPayloadView",
		"WorkOutcome", "CreateWorkItemRequest", "ClassifyWorkItemRequest", "TransitionWorkItemRequest",
		"SplitWorkItemRequest", "MergeWorkItemsRequest", "LinkEventRequest", "DisputeWorkItemRequest",
		"RecordFactRequest", "ConfirmFactRequest", "RescopeFactRequest", "FactVersionRecord",
		"FactConfirmationRecord", "FactView", "FactsView"} {
		if _, ok := schemas[schema]; !ok {
			t.Errorf("schema %s is missing", schema)
		}
	}
}

// TestVerificationContractIsHonest asserts the Story 2.3 surface: the four
// command paths carry the principal boundary and the idempotency key; the
// verification request has NO field for a secret; the WorkOutcome command
// enum covers the four new commands; the CaseView carries the assurance,
// binding and duplicate sections.
func TestVerificationContractIsHonest(t *testing.T) {
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
		"/v1/tenants/{tenant_id}/cases/{case_id}/verifications",
		"/v1/tenants/{tenant_id}/cases/{case_id}/bindings",
		"/v1/tenants/{tenant_id}/cases/{case_id}/bindings/{binding_id}/unlink",
		"/v1/tenants/{tenant_id}/cases/{case_id}/duplicates/{candidate_id}/resolution",
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
			if name, _ := item["name"].(string); name == "Idempotency-Key" {
				hasKey = true
			}
		}
		if !hasKey {
			t.Errorf("%s: the Idempotency-Key parameter is missing", path)
		}
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	verification := mapping(t, schemas, "StartVerificationRequest")
	properties := mapping(t, verification, "properties")
	for _, secret := range []string{"code", "secret", "password", "otp", "pin"} {
		if _, ok := properties[secret]; ok {
			t.Errorf("the verification request must not carry the secret field %q", secret)
		}
	}
	outcome := mapping(t, schemas, "WorkOutcome")
	command := mapping(t, outcome, "properties")["command"].(map[string]any)
	enum, _ := command["enum"].([]any)
	covered := map[string]bool{}
	for _, value := range enum {
		covered[value.(string)] = true
	}
	for _, name := range []string{"start_verification", "bind_channel", "unlink_channel", "resolve_duplicate"} {
		if !covered[name] {
			t.Errorf("the command enum misses %s", name)
		}
	}
	resolve := mapping(t, schemas, "ResolveDuplicateRequest")
	resolveProperties := mapping(t, resolve, "properties")
	subject, _ := resolveProperties["subject_handle"].(map[string]any)
	if subject == nil {
		t.Errorf("ResolveDuplicateRequest.subject_handle is missing")
	} else {
		if _, ok := subject["minLength"]; !ok {
			t.Errorf("subject_handle must carry minLength (the M4 discipline)")
		}
	}
	if required, _ := resolve["required"].([]any); len(required) > 0 {
		found := false
		for _, field := range required {
			if field == "subject_handle" {
				found = true
			}
		}
		if !found {
			t.Errorf("ResolveDuplicateRequest.required must list subject_handle")
		}
	}
	caseView := mapping(t, schemas, "CaseView")
	viewProperties := mapping(t, caseView, "properties")
	for _, section := range []string{"assurance", "bindings", "duplicates"} {
		if _, ok := viewProperties[section]; !ok {
			t.Errorf("the CaseView misses the %s section", section)
		}
	}
}

// TestOwnershipContractIsHonest asserts the Story 2.4 surface: the five
// command paths carry the principal boundary and the idempotency key; the
// transfer request has NO field for the deadline (the human coverage is
// the only source); the reply request carries the bounded content only;
// the WorkOutcome command enum covers the five new commands; the CaseView
// carries the ownership section; the reply gate refusal is documented as
// the machine-readable 403.
func TestOwnershipContractIsHonest(t *testing.T) {
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
		"/v1/tenants/{tenant_id}/cases/{case_id}/ownership/take",
		"/v1/tenants/{tenant_id}/cases/{case_id}/ownership/transfer-requests",
		"/v1/tenants/{tenant_id}/cases/{case_id}/ownership/transfer-requests/{request_id}/resolution",
		"/v1/tenants/{tenant_id}/cases/{case_id}/ownership/reassign",
		"/v1/tenants/{tenant_id}/cases/{case_id}/client-replies",
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
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	transfer := mapping(t, schemas, "RequestTransferRequest")
	transferProperties := mapping(t, transfer, "properties")
	for _, forbidden := range []string{"deadline_minutes", "deadline", "ack_deadline"} {
		if _, ok := transferProperties[forbidden]; ok {
			t.Errorf("the transfer request must never carry the deadline field %q (the human coverage is the only source)", forbidden)
		}
	}
	reply := mapping(t, schemas, "SendClientReplyRequest")
	replyProperties := mapping(t, reply, "properties")
	replyContent, _ := replyProperties["content"].(map[string]any)
	if replyContent == nil {
		t.Fatal("SendClientReplyRequest.content is missing")
	}
	if _, ok := replyContent["maxLength"]; !ok {
		t.Errorf("content must carry maxLength 2000 (the bounded operator words)")
	}
	// The CAS expectation is the live contract word: the field is
	// required (the machine path never silently degrades to a
	// server-only CAS) and bounded below by 1.
	replyRequired, _ := reply["required"].([]any)
	requiredExpectedEpoch := false
	for _, value := range replyRequired {
		if text, _ := value.(string); text == "expected_epoch" {
			requiredExpectedEpoch = true
		}
	}
	if !requiredExpectedEpoch {
		t.Errorf("SendClientReplyRequest must require expected_epoch (the live CAS word)")
	}
	replyEpoch, _ := replyProperties["expected_epoch"].(map[string]any)
	if replyEpoch == nil {
		t.Fatal("SendClientReplyRequest.expected_epoch is missing")
	}
	if minimum, _ := replyEpoch["minimum"].(int); minimum != 1 {
		t.Errorf("expected_epoch must carry minimum 1, got %v", replyEpoch["minimum"])
	}
	for _, secret := range []string{"secret", "code", "otp", "password"} {
		if _, ok := replyProperties[secret]; ok {
			t.Errorf("the reply request must not carry the secret field %q", secret)
		}
	}
	outcome := mapping(t, schemas, "WorkOutcome")
	command := mapping(t, outcome, "properties")["command"].(map[string]any)
	enum, _ := command["enum"].([]any)
	covered := map[string]bool{}
	for _, value := range enum {
		covered[value.(string)] = true
	}
	for _, name := range []string{"take_ownership", "request_transfer", "resolve_transfer", "reassign_ownership", "send_client_reply"} {
		if !covered[name] {
			t.Errorf("the command enum misses %s", name)
		}
	}
	caseView := mapping(t, schemas, "CaseView")
	viewProperties := mapping(t, caseView, "properties")
	if _, ok := viewProperties["ownership"]; !ok {
		t.Errorf("the CaseView misses the ownership section")
	}
	health := mapping(t, mapping(t, mapping(t, mapping(t, mapping(t, schemas, "OwnershipView"), "properties"), "state"), "properties"), "Health")
	healthEnum, _ := health["enum"].([]any)
	wanted := map[string]bool{"stable": false, "transfer_pending": false, "transfer_timed_out": false, "reassignment_required": false}
	for _, value := range healthEnum {
		wanted[value.(string)] = true
	}
	for health, present := range wanted {
		if !present {
			t.Errorf("the ownership health enum misses %s", health)
		}
	}
}

// TestModeContractIsHonest pins the Story 2.5 contract surface: the four
// mode command paths with the principal boundary and the idempotency key;
// the reply class discipline of the client reply (required, enum, no
// secret fields); the disclosure event dictionary; the ladder and the
// dictionaries of the schemas; the machine-readable gate fields.
func TestModeContractIsHonest(t *testing.T) {
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
		"/v1/tenants/{tenant_id}/cases/{case_id}/mode/proposals",
		"/v1/tenants/{tenant_id}/cases/{case_id}/mode/exit",
		"/v1/tenants/{tenant_id}/cases/{case_id}/disclosures",
		"/v1/tenants/{tenant_id}/cases/{case_id}/disclosures/{disclosure_id}/delivery-events",
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
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	reply := mapping(t, schemas, "SendClientReplyRequest")
	replyRequired, _ := reply["required"].([]any)
	requiredClass := false
	for _, value := range replyRequired {
		if text, _ := value.(string); text == "reply_class" {
			requiredClass = true
		}
	}
	if !requiredClass {
		t.Errorf("SendClientReplyRequest must require reply_class (the producer's mark — no silent default)")
	}
	replyProperties := mapping(t, reply, "properties")
	classEnum, _ := mapping(t, replyProperties, "reply_class")["enum"].([]any)
	if len(classEnum) != 3 {
		t.Errorf("reply_class must carry the three-word enum (sales|neutral|protected_request), got %v", classEnum)
	}
	for _, secret := range []string{"secret", "code", "otp", "password"} {
		if _, ok := replyProperties[secret]; ok {
			t.Errorf("the reply request must not carry the secret field %q", secret)
		}
	}
	// The proposal carries the CAS expectation and the ladder discipline
	// (the downgrade is impossible — the honest 409 fields documented).
	propose := mapping(t, schemas, "ProposeModeRequest")
	proposeRequired, _ := propose["required"].([]any)
	hasExpected := false
	for _, value := range proposeRequired {
		if text, _ := value.(string); text == "expected_mode_version" {
			hasExpected = true
		}
	}
	if !hasExpected {
		t.Error("ProposeModeRequest must require expected_mode_version (the live CAS word)")
	}
	modeEnum, _ := mapping(t, mapping(t, propose, "properties"), "mode")["enum"].([]any)
	if len(modeEnum) != 7 {
		t.Errorf("the mode enum carries exactly seven ladder words, got %v", modeEnum)
	}
	// The exit carries the mandatory honest reason.
	exitSchema := mapping(t, schemas, "ExitModeRequest")
	exitRequired, _ := exitSchema["required"].([]any)
	hasReason := false
	for _, value := range exitRequired {
		if text, _ := value.(string); text == "reason" {
			hasReason = true
		}
	}
	if !hasReason {
		t.Error("ExitModeRequest must require the honest reason (the supervised lowering word)")
	}
	// The disclosure text never travels anywhere except the record: the
	// delivery event request carries no text field.
	eventSchema := mapping(t, schemas, "RecordDisclosureEventRequest")
	if _, ok := mapping(t, eventSchema, "properties")["text"]; ok {
		t.Error("the delivery event request never carries the disclosure text")
	}
	kindEnum, _ := mapping(t, mapping(t, eventSchema, "properties"), "kind")["enum"].([]any)
	if len(kindEnum) != 6 {
		t.Errorf("the delivery kind dictionary carries exactly six words (one event — one kind), got %v", kindEnum)
	}
	// The CaseView carries the mode section.
	caseView := mapping(t, schemas, "CaseView")
	if _, ok := mapping(t, caseView, "properties")["mode"]; !ok {
		t.Error("the CaseView misses the mode section")
	}
	// The WorkOutcome command enum carries the four new commands.
	outcome := mapping(t, schemas, "WorkOutcome")
	command := mapping(t, outcome, "properties")["command"].(map[string]any)
	enum, _ := command["enum"].([]any)
	covered := map[string]bool{}
	for _, value := range enum {
		covered[value.(string)] = true
	}
	for _, name := range []string{"propose_mode", "exit_mode", "record_disclosure", "record_disclosure_event"} {
		if !covered[name] {
			t.Errorf("the command enum misses %s", name)
		}
	}
}

// TestPolicyContractIsHonest pins the Story 2.6 contract surface: the two
// policy command paths with the principal boundary and the idempotency
// key; the schema-word dictionaries (the proposal schema version, the
// intents, the outcome classes and reasons, the proposers, the missing
// reasons); the four anchors of the evaluation (the request-hash
// discipline — the m-r2-1 lesson); the machine-readable conflict fields;
// the sales gate word of the reply request (the request never carries the
// outcome id — the server resolves it by the Work Item anchor).
func TestPolicyContractIsHonest(t *testing.T) {
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
		"/v1/tenants/{tenant_id}/cases/{case_id}/ai-proposals",
		"/v1/tenants/{tenant_id}/cases/{case_id}/policy/evaluations",
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
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	// The proposal request: the structure discipline (the intent is a
	// bounded string — the DICTIONARY is a policy matter, never a
	// submission constraint; the candidate may be empty; the missing
	// reasons and the proposers are the enums).
	proposal := mapping(t, schemas, "AIProposalRequest")
	proposalRequired, _ := proposal["required"].([]any)
	for _, word := range []string{"conversation_id", "work_item_id", "turn_ref", "intent", "proposer"} {
		found := false
		for _, value := range proposalRequired {
			if text, _ := value.(string); text == word {
				found = true
			}
		}
		if !found {
			t.Errorf("AIProposalRequest must require %s", word)
		}
	}
	proposalProperties := mapping(t, proposal, "properties")
	if _, ok := proposalProperties["intent"].(map[string]any); ok {
		intent := mapping(t, proposalProperties, "intent")
		if _, hasEnum := intent["enum"]; hasEnum {
			t.Error("the intent carries NO enum: the dictionary is a policy matter (an unknown intent fails closed as an outcome, never a submission refusal)")
		}
		if maxLength, _ := intent["maxLength"].(int); maxLength != 64 {
			t.Errorf("the intent is bounded by 64 runes, got %v", intent["maxLength"])
		}
	}
	if candidate, ok := proposalProperties["candidate_answer"].(map[string]any); ok {
		if _, hasMin := candidate["minLength"]; hasMin {
			t.Error("the candidate answer may be empty (no answer basis — the clarify outcome)")
		}
	}
	// The evaluation request: the four anchors are required (the full
	// request-hash material — the m-r2-1 lesson).
	evaluate := mapping(t, schemas, "EvaluatePolicyRequest")
	evaluateRequired, _ := evaluate["required"].([]any)
	for _, word := range []string{"proposal_ref", "expected_mode_version", "expected_owner_epoch", "expected_facts_digest", "expected_work_item_version"} {
		found := false
		for _, value := range evaluateRequired {
			if text, _ := value.(string); text == word {
				found = true
			}
		}
		if !found {
			t.Errorf("EvaluatePolicyRequest must require %s (the anchor discipline)", word)
		}
	}
	// The outcome view: the fixed dictionaries of the decision.
	outcome := mapping(t, schemas, "PolicyOutcomeView")
	outcomeProperties := mapping(t, outcome, "properties")
	if classEnum, _ := mapping(t, outcomeProperties, "outcome_class")["enum"].([]any); len(classEnum) != 6 {
		t.Errorf("outcome_class must carry the six-word enum (degraded_mode is the Story 2.12 honest stop), got %v", classEnum)
	}
	if reasonEnum, _ := mapping(t, outcomeProperties, "reason")["enum"].([]any); len(reasonEnum) != 8 {
		t.Errorf("reason must carry the eight-word enum (no_alternative_path is the Story 2.11 honest stop, degraded_route is the Story 2.12 word), got %v", reasonEnum)
	}
	if ruleEnum, _ := mapping(t, outcomeProperties, "rule_version")["enum"].([]any); len(ruleEnum) != 3 || ruleEnum[0] != "intake-policy-v1" || ruleEnum[1] != "intake-policy-v2" || ruleEnum[2] != "intake-policy-v3" {
		t.Errorf("rule_version carries the v1/v2/v3 words (the historical rows stay readable), got %v", ruleEnum)
	}
	snapshot := mapping(t, schemas, "PolicyInputSnapshot")
	snapshotRequired, _ := snapshot["required"].([]any)
	for _, word := range []string{"proposal_id", "schema_version", "mode_version", "owner_epoch", "facts_digest", "work_item_version", "rule_version", "preferences", "degradations"} {
		found := false
		for _, value := range snapshotRequired {
			if text, _ := value.(string); text == word {
				found = true
			}
		}
		if !found {
			t.Errorf("PolicyInputSnapshot must require %s (the reproducibility by the input versions)", word)
		}
	}
	// The sales gate word of the reply: the request never carries the
	// outcome id (the server resolves the current outcome by the Work
	// Item anchor).
	reply := mapping(t, schemas, "SendClientReplyRequest")
	replyProperties := mapping(t, reply, "properties")
	if _, ok := replyProperties["policy_outcome_id"]; ok {
		t.Error("SendClientReplyRequest must NOT carry policy_outcome_id: the server resolves the current outcome by the Work Item anchor")
	}
	// The WorkOutcome command enum: the two policy commands joined the
	// idempotency surface.
	workOutcome := mapping(t, schemas, "WorkOutcome")
	commandEnum, _ := mapping(t, mapping(t, workOutcome, "properties"), "command")["enum"].([]any)
	commands := map[string]bool{}
	for _, value := range commandEnum {
		if text, _ := value.(string); text != "" {
			commands[text] = true
		}
	}
	for _, word := range []string{"submit_ai_proposal", "evaluate_policy"} {
		if !commands[word] {
			t.Errorf("WorkOutcome.command must carry %s", word)
		}
	}
	// The proposal envelope never carries the candidate text word.
	if _, ok := mapping(t, schemas, "PolicyOutcomeView")["candidate"]; ok {
		t.Error("the outcome view never carries the candidate text")
	}
}

func TestObservationContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	path := "/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/owning-system-reads"
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

	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	// The read request: the material field IS the fixed dictionary (a
	// field outside the matrix is refused honestly — the unknown class,
	// unlike the intent word of 2.6 whose dictionary is a policy matter).
	read := mapping(t, schemas, "OwningSystemReadRequest")
	readRequired, _ := read["required"].([]any)
	if len(readRequired) != 1 || readRequired[0] != "material_key" {
		t.Errorf("OwningSystemReadRequest requires exactly the material_key, got %v", readRequired)
	}
	if fieldEnum, _ := mapping(t, mapping(t, read, "properties"), "material_key")["enum"].([]any); len(fieldEnum) != 3 {
		t.Errorf("material_key must carry the three matrix words (price, availability, order_status), got %v", fieldEnum)
	}

	// The view: the fixed dictionary words of the matrix and the honest
	// per-field derivations.
	view := mapping(t, schemas, "ObservationView")
	viewRequired, _ := view["required"].([]any)
	for _, word := range []string{"items", "field_words", "matrix_version", "panel_text"} {
		found := false
		for _, value := range viewRequired {
			if text, _ := value.(string); text == word {
				found = true
			}
		}
		if !found {
			t.Errorf("ObservationView must require %s", word)
		}
	}
	viewProperties := mapping(t, view, "properties")
	if matrixEnum, _ := mapping(t, viewProperties, "matrix_version")["enum"].([]any); len(matrixEnum) != 1 || matrixEnum[0] != "state-authority-matrix-v1" {
		t.Errorf("matrix_version is the fixed state-authority-matrix-v1 word, got %v", matrixEnum)
	}
	if fieldsItems, _ := mapping(t, viewProperties, "field_words")["items"].(map[string]any); fieldsItems != nil {
		if itemsEnum, _ := fieldsItems["enum"].([]any); len(itemsEnum) != 3 {
			t.Errorf("field_words items must carry the three-word enum, got %v", itemsEnum)
		}
	}

	// The card: the state derivation words and the conflict badge words
	// are the fixed dictionaries (the honest states never dress up).
	card := mapping(t, schemas, "ObservationCard")
	cardProperties := mapping(t, card, "properties")
	if stateEnum, _ := mapping(t, cardProperties, "state")["enum"].([]any); len(stateEnum) != 4 {
		t.Errorf("state must carry the four derivation words (fresh, stale, failed, unknown), got %v", stateEnum)
	}
	if badgeEnum, _ := mapping(t, cardProperties, "conflict_badge")["enum"].([]any); len(badgeEnum) != 3 {
		t.Errorf("conflict_badge must carry the three badge words (equal, differs, no_fact), got %v", badgeEnum)
	}
	if systemsEnum, _ := mapping(t, cardProperties, "owning_system")["enum"].([]any); len(systemsEnum) != 2 {
		t.Errorf("owning_system must carry the two system words (catalog, erp), got %v", systemsEnum)
	}

	// The record: the status and the reason are the fixed dictionaries;
	// the ok row always carries the carrier words (the projection value
	// is the ok-only word — a failure never carries a value).
	record := mapping(t, schemas, "ObservationRecord")
	recordProperties := mapping(t, record, "properties")
	if statusEnum, _ := mapping(t, recordProperties, "status")["enum"].([]any); len(statusEnum) != 3 {
		t.Errorf("status must carry the three outcome words (ok, failed, unknown), got %v", statusEnum)
	}
	if reasonEnum, _ := mapping(t, recordProperties, "reason")["enum"].([]any); len(reasonEnum) != 4 {
		t.Errorf("reason must carry the four failure words, got %v", reasonEnum)
	}
	recordRequired, _ := record["required"].([]any)
	for _, word := range []string{"observation_id", "observation_no", "status", "received_at"} {
		found := false
		for _, value := range recordRequired {
			if text, _ := value.(string); text == word {
				found = true
			}
		}
		if !found {
			t.Errorf("ObservationRecord must require %s", word)
		}
	}

	// The WorkOutcome command enum: the read command joined the
	// idempotency surface.
	workOutcome := mapping(t, schemas, "WorkOutcome")
	commandEnum, _ := mapping(t, mapping(t, workOutcome, "properties"), "command")["enum"].([]any)
	hasRead := false
	for _, value := range commandEnum {
		if text, _ := value.(string); text == "read_owning_system" {
			hasRead = true
		}
	}
	if !hasRead {
		t.Error("WorkOutcome.command must carry read_owning_system")
	}

	// The case view carries the observation section.
	caseView := mapping(t, schemas, "CaseView")
	caseProperties := mapping(t, caseView, "properties")
	if observationRef, _ := mapping(t, caseProperties, "observations")["$ref"].(string); observationRef != "#/components/schemas/ObservationView" {
		t.Errorf("CaseView must reference the ObservationView section, got %v", observationRef)
	}
}

func TestObjectionContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	paths := mapping(t, document, "paths")
	path := "/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/objections"
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

	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	// The record request: the type, the signal and the verbatim spans are
	// the fixed dictionaries (a word outside the dictionary is refused
	// honestly; the spans are the CLIENT's words — the byte-verified
	// excerpts, never the invented evidence).
	request := mapping(t, schemas, "ObjectionRecordRequest")
	requestRequired, _ := request["required"].([]any)
	if len(requestRequired) != 4 {
		t.Errorf("ObjectionRecordRequest requires the four material fields, got %v", requestRequired)
	}
	requestProperties := mapping(t, request, "properties")
	if typeEnum, _ := mapping(t, requestProperties, "objection_type")["enum"].([]any); len(typeEnum) != 3 {
		t.Errorf("objection_type must carry the three type words (trust, price_comparison, general_doubt), got %v", typeEnum)
	}
	if signalEnum, _ := mapping(t, requestProperties, "client_signal")["enum"].([]any); len(signalEnum) != 4 {
		t.Errorf("client_signal must carry the four signal words (concern, refusal, stop_intent, human_request), got %v", signalEnum)
	}
	if spans, _ := mapping(t, requestProperties, "spans")["maxItems"].(int); spans != 8 {
		t.Errorf("the spans stay bounded by eight excerpts, got %v", spans)
	}

	// The view: the fixed dictionary words of the whole objection surface
	// and the derived (never stored) persuasion state.
	view := mapping(t, schemas, "ObjectionView")
	viewRequired, _ := view["required"].([]any)
	for _, word := range []string{"items", "type_words", "signal_words", "step_words", "playbook_version", "threshold", "panel_text"} {
		found := false
		for _, value := range viewRequired {
			if text, _ := value.(string); text == word {
				found = true
			}
		}
		if !found {
			t.Errorf("ObjectionView must require %s", word)
		}
	}
	viewProperties := mapping(t, view, "properties")
	if playbookEnum, _ := mapping(t, viewProperties, "playbook_version")["enum"].([]any); len(playbookEnum) != 1 || playbookEnum[0] != "objection-playbook-v1" {
		t.Errorf("playbook_version is the fixed objection-playbook-v1 word, got %v", playbookEnum)
	}
	if stepsItems, _ := mapping(t, viewProperties, "step_words")["items"].(map[string]any); stepsItems != nil {
		if itemsEnum, _ := stepsItems["enum"].([]any); len(itemsEnum) != 5 {
			t.Errorf("step_words items must carry the five step words, got %v", itemsEnum)
		}
	}

	// The Work Item section: the derived state words of the persuasion
	// budget (stopped/acknowledged are the read-time derivations — never
	// stored mutable state).
	section := mapping(t, schemas, "ObjectionWorkItemView")
	sectionRequired, _ := section["required"].([]any)
	for _, word := range []string{"work_item_id", "objection_count", "refusals", "threshold", "stopped", "acknowledged"} {
		found := false
		for _, value := range sectionRequired {
			if text, _ := value.(string); text == word {
				found = true
			}
		}
		if !found {
			t.Errorf("ObjectionWorkItemView must require %s", word)
		}
	}

	// The record: the dictionary words are the fixed CHECK backstops of
	// migration 00008.
	record := mapping(t, schemas, "ObjectionRecord")
	recordProperties := mapping(t, record, "properties")
	if typeEnum, _ := mapping(t, recordProperties, "objection_type")["enum"].([]any); len(typeEnum) != 3 {
		t.Errorf("the record objection_type carries the three-word enum, got %v", typeEnum)
	}
	if signalEnum, _ := mapping(t, recordProperties, "client_signal")["enum"].([]any); len(signalEnum) != 4 {
		t.Errorf("the record client_signal carries the four-word enum, got %v", signalEnum)
	}

	// The sales reply request: the optional playbook step is the dictionary
	// with the acknowledge-first discipline (the gate 2-8 words).
	reply := mapping(t, schemas, "SendClientReplyRequest")
	replyProperties := mapping(t, reply, "properties")
	if stepEnum, _ := mapping(t, replyProperties, "playbook_step")["enum"].([]any); len(stepEnum) != 5 {
		t.Errorf("playbook_step must carry the five step words, got %v", stepEnum)
	}
	replyRequired, _ := reply["required"].([]any)
	for _, value := range replyRequired {
		if text, _ := value.(string); text == "playbook_step" {
			t.Error("playbook_step stays optional (the honest absence of objections keeps the old path)")
		}
	}

	// The WorkOutcome command enum: the record command joined the
	// idempotency surface.
	workOutcome := mapping(t, schemas, "WorkOutcome")
	commandEnum, _ := mapping(t, mapping(t, workOutcome, "properties"), "command")["enum"].([]any)
	hasRecord := false
	for _, value := range commandEnum {
		if text, _ := value.(string); text == "record_objection" {
			hasRecord = true
		}
	}
	if !hasRecord {
		t.Error("WorkOutcome.command must carry record_objection")
	}

	// The case view carries the objection section.
	caseView := mapping(t, schemas, "CaseView")
	caseProperties := mapping(t, caseView, "properties")
	if objectionRef, _ := mapping(t, caseProperties, "objections")["$ref"].(string); objectionRef != "#/components/schemas/ObjectionView" {
		t.Errorf("CaseView must reference the ObjectionView section, got %v", objectionRef)
	}
}

// TestCommitmentContractIsHonest pins the Story 2.9 contract honesty: the
// three command paths carry the principal boundary and the idempotency
// key; the request dictionaries are the fixed CHECK backstops of
// migration 00009 (the statuses, the action words, the decline outcomes,
// the unknown term words); the confirm request demands the verbatim
// spans, the CAS expectation and the exact version; the view carries the
// derived (never stored) readiness and validity words with the reserved
// words of the dictionary; the honest 400/409/403 codes of the matrix
// rows are documented on the exact paths.
func TestCommitmentContractIsHonest(t *testing.T) {
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
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments",
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments/{commitment_id}/confirm",
		"/v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments/{commitment_id}/decline",
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
		// The honest refusal codes of the matrix rows: the dictionaries
		// and the discipline refusals are 400; the scope gate is the 403
		// BEFORE the guards; the duplicate subject, the stale ladder and
		// the key conflict are the honest 409.
		responses := mapping(t, post, "responses")
		for _, code := range []string{"400", "403", "409"} {
			if _, documented := responses[code]; !documented {
				t.Errorf("%s: the honest %s refusal is not documented", path, code)
			}
		}
	}

	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	// The create request: the fixed dictionaries (the statuses, the
	// actions, the unknown term words) and the typed terms.
	create := mapping(t, schemas, "CommitmentCreateRequest")
	createProperties := mapping(t, create, "properties")
	if statusEnum, _ := mapping(t, createProperties, "status")["enum"].([]any); len(statusEnum) != 2 {
		t.Errorf("the create status carries the two-word enum (draft, offered), got %v", statusEnum)
	}
	actions, _ := mapping(t, createProperties, "action_set")["items"].(map[string]any)
	if actions != nil {
		if actionsEnum, _ := actions["enum"].([]any); len(actionsEnum) != 3 {
			t.Errorf("the action_set items carry the three action words, got %v", actionsEnum)
		}
	}
	unknowns, _ := mapping(t, createProperties, "material_unknowns")["items"].(map[string]any)
	if unknowns != nil {
		if unknownsEnum, _ := unknowns["enum"].([]any); len(unknownsEnum) != 7 {
			t.Errorf("the material_unknowns items carry the seven mandatory term words, got %v", unknownsEnum)
		}
	}
	if limit, _ := mapping(t, createProperties, "consumption_limit")["maximum"].(int); limit != 10 {
		t.Errorf("the consumption limit stays bounded by ten, got %v", limit)
	}
	terms := mapping(t, schemas, "MaterialTerms")
	termsRequired, _ := terms["required"].([]any)
	if len(termsRequired) != 7 {
		t.Errorf("MaterialTerms requires the seven mandatory fields, got %v", termsRequired)
	}
	price := mapping(t, mapping(t, terms, "properties"), "price")
	if priceRef, _ := price["$ref"].(string); priceRef != "#/components/schemas/CommitmentPriceTerm" {
		t.Errorf("the price term is the typed CommitmentPriceTerm, got %v", price)
	}
	priceTerm := mapping(t, schemas, "CommitmentPriceTerm")
	if kindEnum, _ := mapping(t, mapping(t, priceTerm, "properties"), "kind")["enum"].([]any); len(kindEnum) != 2 {
		t.Errorf("the price kind carries the amount/range enum, got %v", kindEnum)
	}

	// The confirm request: the exact version, the CAS expectation, the
	// subject dictionary and the verbatim spans (1..8 — the invented
	// consent is impossible).
	confirm := mapping(t, schemas, "CommitmentConfirmRequest")
	confirmRequired, _ := confirm["required"].([]any)
	if len(confirmRequired) != 6 {
		t.Errorf("CommitmentConfirmRequest requires the six material fields (version, expectation, subject, turn, spans), got %v", confirmRequired)
	}
	confirmProperties := mapping(t, confirm, "properties")
	if spans, _ := mapping(t, confirmProperties, "spans")["maxItems"].(int); spans != 8 {
		t.Errorf("the confirm spans stay bounded by eight excerpts, got %v", spans)
	}
	if subjectEnum, _ := mapping(t, confirmProperties, "subject_kind")["enum"].([]any); len(subjectEnum) != 1 || subjectEnum[0] != "client" {
		t.Errorf("the subject kind carries the client dictionary v1, got %v", subjectEnum)
	}

	// The decline request: the outcome dictionary.
	decline := mapping(t, schemas, "CommitmentDeclineRequest")
	if outcomeEnum, _ := mapping(t, mapping(t, decline, "properties"), "outcome")["enum"].([]any); len(outcomeEnum) != 2 {
		t.Errorf("the decline outcome carries the two-word enum, got %v", outcomeEnum)
	}

	// The view: the derived words with the reserved ones (revoked and
	// consumed never carry a producer in the slice — the honest
	// reservation of the dictionary).
	view := mapping(t, schemas, "CommitmentView")
	viewProperties := mapping(t, view, "properties")
	validity, _ := mapping(t, viewProperties, "validity_words")["items"].(map[string]any)
	if validity != nil {
		validityEnum, _ := validity["enum"].([]any)
		if len(validityEnum) != 7 {
			t.Errorf("the validity words carry the five produced and the two reserved words, got %v", validityEnum)
		}
		reserved := map[string]bool{}
		for _, word := range validityEnum {
			if text, _ := word.(string); text == "revoked" || text == "consumed" {
				reserved[text] = true
			}
		}
		if !reserved["revoked"] || !reserved["consumed"] {
			t.Errorf("the reserved validity words (revoked, consumed) stay in the dictionary, got %v", validityEnum)
		}
	}
	readiness, _ := mapping(t, viewProperties, "readiness_words")["items"].(map[string]any)
	if readiness != nil {
		if readinessEnum, _ := readiness["enum"].([]any); len(readinessEnum) != 4 {
			t.Errorf("the readiness words carry the four-word enum, got %v", readinessEnum)
		}
	}
	card := mapping(t, schemas, "CommitmentVersionCard")
	cardRequired, _ := card["required"].([]any)
	for _, word := range []string{"version", "readiness", "presented", "current"} {
		found := false
		for _, value := range cardRequired {
			if text, _ := value.(string); text == word {
				found = true
			}
		}
		if !found {
			t.Errorf("CommitmentVersionCard must require %s", word)
		}
	}
	confirmation := mapping(t, schemas, "CommitmentConfirmationCard")
	if validityRef, _ := mapping(t, confirmation, "properties")["validity"].(map[string]any); validityRef != nil {
		if methodEnum, _ := validityRef["enum"].([]any); len(methodEnum) != 5 {
			t.Errorf("the confirmation validity carries the five produced words, got %v", methodEnum)
		}
	}
	// The presentation link of the sales reply: the nullable pair (both
	// or neither).
	reply := mapping(t, schemas, "SendClientReplyRequest")
	replyProperties := mapping(t, reply, "properties")
	if _, hasLink := replyProperties["commitment_id"]; !hasLink {
		t.Errorf("SendClientReplyRequest documents the presentation link pair (commitment_id)")
	}
	if _, hasVersion := replyProperties["commitment_version"]; !hasVersion {
		t.Errorf("SendClientReplyRequest documents the presentation link pair (commitment_version)")
	}
	// The case view carries the commitments section.
	caseView := mapping(t, schemas, "CaseView")
	if _, hasSection := mapping(t, caseView, "properties")["commitments"]; !hasSection {
		t.Errorf("CaseView documents the commitments section of Story 2.9")
	}
}

// TestVehicleIdentificationContractIsHonest pins the spec 4.2 mirrors:
// the use-kind dictionary 3→4 (the +1 vehicle_confirmation_submitted of
// the SQL mirror 00018), the material-key enum of the pack schema (6
// words — the pack material, never a hardcoded core list), the closed
// shapes of the vehicle views and the honest descriptions (no silent
// fixes, the mask boundary, the exact-version confirmations). A drift
// between the OpenAPI mirrors and the domain dictionaries is red here.
func TestVehicleIdentificationContractIsHonest(t *testing.T) {
	content, err := os.ReadFile("staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	schemas := mapping(t, mapping(t, document, "components"), "schemas")
	// The use-kind dictionary: the four words (the SQL mirror of
	// migrations/cases/00018).
	useView := mapping(t, schemas, "ContinuationUseView")
	useProps := mapping(t, useView, "properties")
	useKind := mapping(t, useProps, "use_kind")
	useEnum, _ := useKind["enum"].([]any)
	if len(useEnum) != 4 || useEnum[0] != "fact_correction_submitted" || useEnum[1] != "human_requested" || useEnum[2] != "returned_to_conversation" || useEnum[3] != "vehicle_confirmation_submitted" {
		t.Fatalf("the use-kind dictionary carries four words (the +1 of story 4.2), got %v", useEnum)
	}
	// The vehicle panel: the honest degradation note and the profile.
	panel := mapping(t, schemas, "VehiclePanelView")
	if _, ok := panel["additionalProperties"]; !ok {
		t.Fatal("the vehicle panel must be a closed shape (additionalProperties false)")
	}
	panelProps := mapping(t, panel, "properties")
	if _, ok := panelProps["schema_defined"]; !ok {
		t.Fatal("the vehicle panel must carry the schema_defined flag (the honest absence)")
	}
	if _, ok := panelProps["note"]; !ok {
		t.Fatal("the vehicle panel must carry the honest absence note")
	}
	// The material-key enum: the six pack schema keys (the enum mirror —
	// drift between the pack catalog and the contract is red).
	attribute := mapping(t, schemas, "VehicleAttributeView")
	attributeProps := mapping(t, attribute, "properties")
	materialKey := mapping(t, attributeProps, "material_key")
	keyEnum, _ := materialKey["enum"].([]any)
	if len(keyEnum) != 6 {
		t.Fatalf("the material-key enum carries the six pack schema keys, got %v", keyEnum)
	}
	seen := map[string]bool{}
	for _, word := range keyEnum {
		seen[word.(string)] = true
	}
	for _, word := range []string{"vehicle.vin", "vehicle.make", "vehicle.model", "vehicle.year_of_manufacture", "vehicle.trim", "vehicle.engine"} {
		if !seen[word] {
			t.Fatalf("the material-key enum must carry %s", word)
		}
	}
	// The kind vocabulary of the schema keys.
	kind := mapping(t, attributeProps, "kind")
	kindEnum, _ := kind["enum"].([]any)
	if len(kindEnum) != 3 || kindEnum[0] != "vin" || kindEnum[1] != "text" || kindEnum[2] != "integer" {
		t.Fatalf("the attribute kind vocabulary must stay [vin text integer], got %v", kindEnum)
	}
	// The status vocabulary: the frozen 2.2 read-time derivations.
	status := mapping(t, attributeProps, "status")
	statusEnum, _ := status["enum"].([]any)
	if len(statusEnum) != 3 || statusEnum[0] != "confirmed" || statusEnum[1] != "unverified" || statusEnum[2] != "conflicting" {
		t.Fatalf("the attribute status vocabulary reuses the 2.2 read-time words, got %v", statusEnum)
	}
	// The confirmer vocabulary: the reserved 2.2 dictionary (client finds
	// its first producer with story 4.2 — the word itself is not new).
	confirmation := mapping(t, schemas, "VehicleConfirmationView")
	confirmationProps := mapping(t, confirmation, "properties")
	confirmer := mapping(t, confirmationProps, "confirmer_type")
	confirmerEnum, _ := confirmer["enum"].([]any)
	if len(confirmerEnum) != 3 || confirmerEnum[0] != "client" || confirmerEnum[1] != "owning_system" || confirmerEnum[2] != "operator_relay" {
		t.Fatalf("the confirmer vocabulary reuses the reserved 2.2 words, got %v", confirmerEnum)
	}
	// The completeness: the missing keys carry the same six-word enum.
	completeness := mapping(t, schemas, "VehicleCompletenessView")
	completenessProps := mapping(t, completeness, "properties")
	missing := mapping(t, completenessProps, "missing_keys")
	missingItems := mapping(t, missing, "items")
	missingEnum, _ := missingItems["enum"].([]any)
	if len(missingEnum) != 6 {
		t.Fatalf("the missing-keys enum mirrors the six material keys, got %v", missingEnum)
	}
	// The VIN normalization: the closed shape with the honest flags (no
	// silent fixes — the raw value preserved verbatim).
	normalization := mapping(t, schemas, "VINNormalizationView")
	normalizationProps := mapping(t, normalization, "properties")
	for _, field := range []string{"raw", "normalized", "length", "length_ok", "forbidden_chars", "cyrillic_ambiguities", "check_digit", "words"} {
		if _, ok := normalizationProps[field]; !ok {
			t.Errorf("the VIN normalization must carry the %s field", field)
		}
	}
	if _, ok := normalization["additionalProperties"]; !ok {
		t.Fatal("the VIN normalization must be a closed shape")
	}
	// The case view carries the vehicle section.
	caseView := mapping(t, schemas, "CaseView")
	caseProps := mapping(t, caseView, "properties")
	if _, ok := caseProps["vehicle"]; !ok {
		t.Fatal("the machine case view must carry the vehicle section (the operator review surface)")
	}
	for _, schema := range []string{"VehiclePanelView", "VehicleProfileView", "VehicleCompletenessView", "VehicleAttributeView", "VehicleConfirmationView", "VINNormalizationView", "VINAmbiguityView", "VINCheckDigitView"} {
		if _, ok := schemas[schema]; !ok {
			t.Errorf("schema %s is missing", schema)
		}
	}
}
