package openapi_test

import (
        "os"
        "testing"

        controlDomain "github.com/artemiosu/staffy/internal/control/domain"
        "gopkg.in/yaml.v3"
)

// TestScopedControlsContractIsHonest pins the Story 3.2 slice-1 wire
// contract: the four operator paths carry the principal boundary and the
// honest answer/refusal codes (200/400/401/403/404/409 — a non-member is
// the plain 404, a member without operations is the 403 with the scope
// words BEFORE material, the step-up refusal and the generation conflict
// keep their words); the list carries no 400 (no material to refuse); the
// DELETE method does NOT exist (resume is a mutation with a written
// history, never a deletion); the repeated scope query parameter carries
// the domain upper bound (MaxScopeKeys — the read never turns into an
// unbounded scan); the views mirror the domain dictionaries EXACTLY
// (every enum word a compile-time dictionary word — drift turns this
// test red, the slash word intake_only/no_promises pinned verbatim); the
// audit outcome words pin the migration CHECK vocabulary (applied and
// conflict — BOTH attempts are material history, AC-11); the required
// lists are complete and additionalProperties is false everywhere.
func TestScopedControlsContractIsHonest(t *testing.T) {
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

        // The list path: the principal boundary, the honest codes, the
        // history surface; no 400 (the list carries no material to refuse),
        // no DELETE (resume is a state change with history, not a deletion).
        listPath := mapping(t, paths, "/v1/tenants/{tenant_id}/controls")
        listGet := mapping(t, listPath, "get")
        if listGet["operationId"] != "listScopedControls" {
                t.Errorf("the controls list operation id is wrong: %v", listGet["operationId"])
        }
        if required, _ := listGet["x-staffy-principal-required"].(bool); !required {
                t.Errorf("the controls list does not document the principal boundary")
        }
        listResponses := mapping(t, listGet, "responses")
        for _, code := range []string{"200", "401", "403", "404"} {
                if _, documented := listResponses[code]; !documented {
                        t.Errorf("the controls list does not document the honest %s answer", code)
                }
        }
        if _, documented := listResponses["400"]; documented {
                t.Errorf("the controls list must not document a 400: the read carries no material to refuse")
        }
        if _, exists := listPath["delete"]; exists {
                t.Errorf("the DELETE path must not exist: снятие = resume-мутация с историей, НЕ удаление")
        }
        listOK := mapping(t, listResponses, "200")
        listSchema := mapping(t, mapping(t, mapping(t, listOK, "content"), "application/json"), "schema")
        if required, _ := listSchema["required"].([]any); !requiredWord(required, "controls") {
                t.Errorf("the list answer must carry the required controls wrapper, got %v", required)
        }
        listItems, _ := mapping(t, mapping(t, listSchema, "properties"), "controls")["items"].(map[string]any)
        if ref, _ := listItems["$ref"].(string); ref != "#/components/schemas/ScopedControlView" {
                t.Errorf("the list items must reference ScopedControlView, got %v", ref)
        }

        // The set path: the mutation codes — the dictionary 400 family, the
        // step-up/scope 403, the honest 404 and the generation 409.
        setPost := mapping(t, listPath, "post")
        if setPost["operationId"] != "setScopedControl" {
                t.Errorf("the controls set operation id is wrong: %v", setPost["operationId"])
        }
        if required, _ := setPost["x-staffy-principal-required"].(bool); !required {
                t.Errorf("the controls set does not document the principal boundary")
        }
        setResponses := mapping(t, setPost, "responses")
        for _, code := range []string{"200", "400", "401", "403", "404", "409"} {
                if _, documented := setResponses[code]; !documented {
                        t.Errorf("the controls set does not document the honest %s answer", code)
                }
        }
        setBody := mapping(t, mapping(t, setPost, "requestBody"), "content")
        if ref, _ := mapping(t, mapping(t, setBody, "application/json"), "schema")["$ref"].(string); ref != "#/components/schemas/SetScopedControlRequest" {
                t.Errorf("the set request body must reference SetScopedControlRequest")
        }

        // The effective path: the repeated scope parameter with the domain
        // upper bound; the dictionary 400 of a bad key.
        effectivePath := mapping(t, paths, "/v1/tenants/{tenant_id}/controls/effective")
        effectiveGet := mapping(t, effectivePath, "get")
        if effectiveGet["operationId"] != "computeEffectiveControls" {
                t.Errorf("the controls effective operation id is wrong: %v", effectiveGet["operationId"])
        }
        if required, _ := effectiveGet["x-staffy-principal-required"].(bool); !required {
                t.Errorf("the controls effective does not document the principal boundary")
        }
        effectiveResponses := mapping(t, effectiveGet, "responses")
        for _, code := range []string{"200", "400", "401", "403", "404"} {
                if _, documented := effectiveResponses[code]; !documented {
                        t.Errorf("the controls effective does not document the honest %s answer", code)
                }
        }
        scopeBound := ""
        scopeMax := 0
        for _, parameter := range parametersOf(t, effectivePath) {
                if name, _ := parameter["name"].(string); name == "scope" {
                        scopeSchema := mapping(t, parameter, "schema")
                        if max, _ := scopeSchema["maxItems"].(int); max == controlDomain.MaxScopeKeys {
                                scopeMax = max
                        }
                        if min, _ := scopeSchema["minItems"].(int); min == 1 {
                                scopeBound = "min"
                        }
                }
        }
        if scopeMax != controlDomain.MaxScopeKeys {
                t.Errorf("the scope parameter must carry the domain MaxScopeKeys bound %d, got %d", controlDomain.MaxScopeKeys, scopeMax)
        }
        if scopeBound != "min" {
                t.Errorf("the scope parameter must require at least one key")
        }

        // The resume path: the mutation codes of the lift.
        resumePath := mapping(t, paths, "/v1/tenants/{tenant_id}/controls/resume")
        resumePost := mapping(t, resumePath, "post")
        if resumePost["operationId"] != "resumeScopedControl" {
                t.Errorf("the controls resume operation id is wrong: %v", resumePost["operationId"])
        }
        if required, _ := resumePost["x-staffy-principal-required"].(bool); !required {
                t.Errorf("the controls resume does not document the principal boundary")
        }
        resumeResponses := mapping(t, resumePost, "responses")
        for _, code := range []string{"200", "400", "401", "403", "404", "409"} {
                if _, documented := resumeResponses[code]; !documented {
                        t.Errorf("the controls resume does not document the honest %s answer", code)
                }
        }
        resumeBody := mapping(t, mapping(t, resumePost, "requestBody"), "content")
        if ref, _ := mapping(t, mapping(t, resumeBody, "application/json"), "schema")["$ref"].(string); ref != "#/components/schemas/ResumeScopedControlRequest" {
                t.Errorf("the resume request body must reference ResumeScopedControlRequest")
        }

        // The view: the closed dictionaries and the complete required list.
        view := mapping(t, schemas, "ScopedControlView")
        if additional, ok := view["additionalProperties"].(bool); !ok || additional {
                t.Errorf("ScopedControlView must reject additionalProperties (false), got %v", view["additionalProperties"])
        }
        if required, _ := view["required"].([]any); len(required) != 12 {
                t.Errorf("ScopedControlView must be fully required (12 fields), got %v", required)
        }
        viewProperties := mapping(t, view, "properties")
        assertEnumMirrors(t, mapping(t, viewProperties, "scope_kind")["enum"], controlDomain.ScopeKinds, "scope_kind")
        assertEnumMirrors(t, mapping(t, viewProperties, "mode")["enum"], controlDomain.ControlModes, "mode")
        assertEnumMirrors(t, mapping(t, viewProperties, "state")["enum"], controlDomain.ControlStates, "state")
        prevMode := append([]string{""}, controlDomain.ControlModes...)
        assertEnumMirrors(t, mapping(t, viewProperties, "prev_mode")["enum"], prevMode, "prev_mode")
        if generation := mapping(t, viewProperties, "generation"); generation["minimum"].(int) < 1 {
                t.Errorf("the generation must be at least 1: the first installation IS generation 1, got %v", generation["minimum"])
        }

        // The effective view: the same mode dictionary on the intersection.
        effectiveView := mapping(t, schemas, "EffectiveControlView")
        if additional, ok := effectiveView["additionalProperties"].(bool); !ok || additional {
                t.Errorf("EffectiveControlView must reject additionalProperties (false), got %v", effectiveView["additionalProperties"])
        }
        if required, _ := effectiveView["required"].([]any); len(required) != 5 {
                t.Errorf("EffectiveControlView must be fully required (5 fields), got %v", required)
        }
        effectiveProperties := mapping(t, effectiveView, "properties")
        assertEnumMirrors(t, mapping(t, effectiveProperties, "mode")["enum"], controlDomain.ControlModes, "effective_mode")
        assertEnumMirrors(t, mapping(t, mapping(t, effectiveProperties, "intersection"), "items")["enum"], controlDomain.ControlModes, "intersection")

        // The requests: the same dictionaries, the reason bounds.
        setRequest := mapping(t, schemas, "SetScopedControlRequest")
        if additional, ok := setRequest["additionalProperties"].(bool); !ok || additional {
                t.Errorf("SetScopedControlRequest must reject additionalProperties (false), got %v", setRequest["additionalProperties"])
        }
        if required, _ := setRequest["required"].([]any); len(required) != 5 {
                t.Errorf("SetScopedControlRequest must be fully required (5 fields), got %v", required)
        }
        setRequestProperties := mapping(t, setRequest, "properties")
        assertEnumMirrors(t, mapping(t, setRequestProperties, "scope_kind")["enum"], controlDomain.ScopeKinds, "set_scope_kind")
        assertEnumMirrors(t, mapping(t, setRequestProperties, "mode")["enum"], controlDomain.ControlModes, "set_mode")
        reason := mapping(t, setRequestProperties, "reason")
        if reason["minLength"].(int) != controlDomain.MinReasonRunes {
                t.Errorf("the reason minLength must mirror the domain bound %d", controlDomain.MinReasonRunes)
        }
        if reason["maxLength"].(int) != controlDomain.MaxReasonRunes {
                t.Errorf("the reason maxLength must mirror the domain bound %d", controlDomain.MaxReasonRunes)
        }

        resumeRequest := mapping(t, schemas, "ResumeScopedControlRequest")
        if additional, ok := resumeRequest["additionalProperties"].(bool); !ok || additional {
                t.Errorf("ResumeScopedControlRequest must reject additionalProperties (false), got %v", resumeRequest["additionalProperties"])
        }
        if required, _ := resumeRequest["required"].([]any); len(required) != 4 {
                t.Errorf("ResumeScopedControlRequest must be fully required (4 fields), got %v", required)
        }
        resumeProperties := mapping(t, resumeRequest, "properties")
        assertEnumMirrors(t, mapping(t, resumeProperties, "scope_kind")["enum"], controlDomain.ScopeKinds, "resume_scope_kind")

        // The per-kind ref dictionaries: the mirrors of the scope refs.
        channel := mapping(t, schemas, "ControlScopeRefChannel")
        assertEnumMirrors(t, channel["enum"], controlDomain.ChannelWords, "channel_ref")
        outcome := mapping(t, schemas, "ControlScopeRefPolicyOutcome")
        assertEnumMirrors(t, outcome["enum"], controlDomain.PolicyOutcomeWords, "policy_outcome_ref")
        autonomy := mapping(t, schemas, "ControlScopeRefAutonomyClass")
        assertEnumMirrors(t, autonomy["enum"], controlDomain.AutonomyClassWords, "autonomy_ref")

        // The audit outcome words: the migration CHECK vocabulary — both
        // attempts stay material (AC-11); the audit read surface arrives with
        // the future slices, the words are pinned here.
        audit := mapping(t, schemas, "ControlAuditOutcome")
        assertEnumMirrors(t, audit["enum"], []string{"applied", "conflict"}, "audit_outcome")
}

// parametersOf extracts the path-level parameter mappings.
func parametersOf(t *testing.T, path map[string]any) []map[string]any {
        t.Helper()
        raw, _ := path["parameters"].([]any)
        parameters := make([]map[string]any, 0, len(raw))
        for _, item := range raw {
                if parameter, ok := item.(map[string]any); ok {
                        parameters = append(parameters, parameter)
                }
        }
        return parameters
}
