package openapi_test

import (
	"os"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/externaltools"
	"github.com/staffy-co/staffy/internal/tools"
	"gopkg.in/yaml.v3"
)

// TestExternalToolsDictionaryContractIsHonest pins the Story 7.4 wire
// contract: the single machine GET path carries the principal boundary and
// the honest membership ladder (200/401/403/404 — the scope words BEFORE
// any dictionary material); the closed forms of the section, the entry and
// the provenance; the state vocabulary admitted|rejected (a rejected row is
// a ROW with the named reason, never a silence); the typing dictionary of
// the projected schemas mirrors the platform dictionary of the registry
// (the 7-2 discipline applied to the live external material — the mirror
// holds the projection to the same three words); the environment and
// status vocabularies mirror the connector machine; the summary speaks the
// honesty words: the pure derivation, the disappearing entries of the
// stale anchors, the strictly read-only surface and the AR-30 admission.
func TestExternalToolsDictionaryContractIsHonest(t *testing.T) {
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

	path := mapping(t, paths, "/v1/tenants/{tenant_id}/tools/external")
	get := mapping(t, path, "get")
	if get["operationId"] != "listExternalToolSections" {
		t.Errorf("идентификатор операции словаря: %v", get["operationId"])
	}
	if required, _ := get["x-staffy-principal-required"].(bool); !required {
		t.Errorf("словарь не документирует границу принципала")
	}
	// The honest ladder of the read: exactly the declared codes.
	responses := mapping(t, get, "responses")
	for _, code := range []string{"200", "401", "403", "404"} {
		if _, documented := responses[code]; !documented {
			t.Errorf("словарь не документирует честный ответ %s", code)
		}
	}
	if len(responses) != 4 {
		t.Errorf("read-only GET обязан документировать ровно четыре ответа, задокументировано %d", len(responses))
	}
	if _, documented := responses["405"]; documented {
		t.Errorf("GET сам себе не отвечает 405")
	}
	// The 200 answer: the sections array of the closed section form.
	sectionList := mapping(t, mapping(t, mapping(t, mapping(t, responses, "200"), "content"), "application/json"), "schema")
	ok := mapping(t, sectionList, "properties")
	sections := mapping(t, ok["sections"].(map[string]any), "items")
	if ref, _ := sections["$ref"].(string); ref != "#/components/schemas/ExternalToolSectionView" {
		t.Errorf("элемент sections обязан быть закрытой формой раздела: %v", sections["$ref"])
	}

	// The section: the closed form with the connector machine vocabularies.
	section := mapping(t, schemas, "ExternalToolSectionView")
	if additional, _ := section["additionalProperties"].(bool); additional {
		t.Errorf("форма раздела обязана быть закрытой")
	}
	sectionRequired := words(t, section["required"])
	for _, field := range []string{"connection_id", "connector_id", "connector_version", "environment", "status", "has_entries", "entries"} {
		if !containsWord(sectionRequired, field) {
			t.Errorf("форма раздела обязана требовать %s", field)
		}
	}
	sectionProperties := mapping(t, section, "properties")
	environment := words(t, sectionProperties["environment"].(map[string]any)["enum"])
	if len(environment) != 2 || environment[0] != "sandbox" || environment[1] != "test" {
		t.Errorf("словарь окружений раздела обязан зеркалить карту класса (ровно sandbox и test — AG-4): %v", environment)
	}
	status := words(t, sectionProperties["status"].(map[string]any)["enum"])
	if len(status) != 4 || status[0] != "draft" || status[1] != "connected_unverified" || status[2] != "failed" || status[3] != "revoked" {
		t.Errorf("словарь состояний раздела обязан зеркалить машину подключений: %v", status)
	}

	// The entry: the closed form with the admission vocabulary and the
	// projected typing schemas of the registry dictionary.
	entry := mapping(t, schemas, "ExternalToolEntryView")
	if additional, _ := entry["additionalProperties"].(bool); additional {
		t.Errorf("форма записи обязана быть закрытой")
	}
	entryRequired := words(t, entry["required"])
	for _, field := range []string{"name", "title", "description", "state", "input_schema", "output_schema"} {
		if !containsWord(entryRequired, field) {
			t.Errorf("форма записи обязана требовать %s", field)
		}
	}
	entryProperties := mapping(t, entry, "properties")
	state := words(t, entryProperties["state"].(map[string]any)["enum"])
	if len(state) != 2 || state[0] != externaltools.EntryAdmitted || state[1] != externaltools.EntryRejected {
		t.Errorf("словарь состояний записи обязан быть закрытым [admitted, rejected]: %v", state)
	}
	for _, schemaField := range []string{"input_schema", "output_schema"} {
		items := mapping(t, entryProperties[schemaField].(map[string]any), "items")
		if ref, _ := items["$ref"].(string); ref != "#/components/schemas/ToolSchemaFieldView" {
			t.Errorf("схема %s записи обязана зеркалить платформенный словарь типизации: %v", schemaField, items["$ref"])
		}
	}

	// The typing dictionary mirror: the projection never extends the
	// registry dictionary (the 7-2 discipline over the live material).
	field := mapping(t, schemas, "ToolSchemaFieldView")
	fieldType := words(t, mapping(t, field, "properties")["type"].(map[string]any)["enum"])
	if len(fieldType) != len(tools.ToolFieldTypes()) {
		t.Fatalf("словарь типизации расширен: %v против %v", fieldType, tools.ToolFieldTypes())
	}
	for i, word := range tools.ToolFieldTypes() {
		if fieldType[i] != word {
			t.Errorf("слово словаря типизации расходится с реестром: %q против %q", fieldType[i], word)
		}
	}

	// The provenance: the closed snapshot identity of the entry.
	provenance := mapping(t, schemas, "ExternalToolProvenanceView")
	if additional, _ := provenance["additionalProperties"].(bool); additional {
		t.Errorf("форма провенанса обязана быть закрытой")
	}
	provenanceRequired := words(t, provenance["required"])
	for _, name := range []string{"connector_id", "connector_version", "server_name", "server_version", "evidence_id", "checked_at", "expires_at"} {
		if !containsWord(provenanceRequired, name) {
			t.Errorf("форма провенанса обязана требовать %s", name)
		}
	}

	// The summary speaks the honesty words: the pure derivation with the
	// disappearing entries, the strictly read-only surface, the AR-30
	// admission gate and the future Planner caller.
	summary, _ := get["summary"].(string)
	for _, word := range []string{"деривацией", "исчезают", "read-only", "AR-30", "Planner 8.3", "integrations|configuration"} {
		if !strings.Contains(summary, word) {
			t.Errorf("сводка операции обязана говорить слово %q", word)
		}
	}
}

func containsWord(haystack []string, needle string) bool {
	for _, word := range haystack {
		if word == needle {
			return true
		}
	}
	return false
}
