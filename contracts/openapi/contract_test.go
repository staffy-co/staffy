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
                "/v1/tenants/{tenant_id}/employees/{employee_id}/release/draft":    "post",
                "/v1/tenants/{tenant_id}/employees/{employee_id}/release/versions": "post",
                "/v1/tenants/{tenant_id}/employees/{employee_id}/release/manifests": "post",
                "/v1/tenants/{tenant_id}/employees/{employee_id}/release/restore":  "post",
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
                "/v1/tenants/{tenant_id}/data-map":                              {"get"},
                "/v1/tenants/{tenant_id}/data-map/entries":                      {"post"},
                "/v1/tenants/{tenant_id}/retention-policies":                    {"post"},
                "/v1/tenants/{tenant_id}/legal-holds":                           {"post"},
                "/v1/tenants/{tenant_id}/legal-holds/release":                   {"post"},
                "/v1/tenants/{tenant_id}/deletion-requests":                     {"post"},
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
                "/v1/tenants/{tenant_id}/entitlement":                                      {"get"},
                "/v1/tenants/{tenant_id}/entitlement/terms":                                {"post"},
                "/v1/tenants/{tenant_id}/entitlement/state":                                {"post"},
                "/v1/tenants/{tenant_id}/entitlement/budgets":                              {"post"},
                "/v1/tenants/{tenant_id}/entitlement/sources":                              {"post"},
                "/v1/tenants/{tenant_id}/usage-events":                                     {"post"},
                "/v1/tenants/{tenant_id}/budget-meter":                                     {"get"},
                "/v1/tenants/{tenant_id}/admission-state":                                  {"get"},
                "/v1/tenants/{tenant_id}/usage-exceptions":                                 {"get", "post"},
                "/v1/tenants/{tenant_id}/usage-exceptions/{exception_id}/acknowledge":      {"post"},
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
                "/v1/tenants/{tenant_id}/entitlement/terms":       "post",
                "/v1/tenants/{tenant_id}/entitlement/state":       "post",
                "/v1/tenants/{tenant_id}/entitlement/budgets":     "post",
                "/v1/tenants/{tenant_id}/entitlement/sources":     "post",
                "/v1/tenants/{tenant_id}/usage-events":            "post",
                "/v1/tenants/{tenant_id}/usage-exceptions":        "post",
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
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile":                            {"get"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/draft":                      {"post"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/approval":                   {"post"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/thresholds":                 {"post"},
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/thresholds/{gate_id}":       {"get"},
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
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/draft":    true,
		"/v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/approval": true,
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
