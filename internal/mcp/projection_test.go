package mcp

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/tools"
)

// projectionFixtureField builds one typed field of the projection probe.
func projectionFixtureField(fieldType string, values []string, min, max int, optional bool) tools.ToolSchemaField {
	return tools.ToolSchemaField{Label: "Поле", Type: fieldType, Values: values, Min: min, Max: max, Optional: optional}
}

// TestProjectionIsTotalOverTheTypingDictionary: EVERY word of the
// platform typing dictionary projects onto the JSON Schema wire — the
// mapping is total and one-way (the registry stays the platform
// language; json-schema.org never enters the artifacts).
func TestProjectionIsTotalOverTheTypingDictionary(t *testing.T) {
	for _, fieldType := range tools.ToolFieldTypes() {
		schema := projectSchema(map[string]tools.ToolSchemaField{
			"field.word": projectionFixtureField(fieldType, nil, 0, 0, false),
		})
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("проекция %s не несёт properties: %+v", fieldType, schema)
		}
		property, ok := properties["field.word"].(map[string]any)
		if !ok {
			t.Fatalf("проекция %s не несёт поля: %+v", fieldType, properties)
		}
		if property["type"] != fieldType {
			t.Fatalf("проекция слова %s обязана нести type, получено %v", fieldType, property["type"])
		}
		if property["title"] != "Поле" {
			t.Fatalf("проекция обязана нести человеческий заголовок, получено %v", property["title"])
		}
		// The property key set of the dictionary words: title, type and
		// the optional enum/bounds — nothing else.
		allowed := map[string]bool{"title": true, "type": true, "enum": true, "minimum": true, "maximum": true}
		for key := range property {
			if !allowed[key] {
				t.Fatalf("проекция несёт незадекларированный ключ %q свойства: %+v", key, property)
			}
		}
	}
}

// TestProjectionCarriesTheDictionarySemantics: the enum values, the
// integer bounds (the zero bound is the unset word — it never travels)
// and the required list of the non-optional keys (the stable order).
func TestProjectionCarriesTheDictionarySemantics(t *testing.T) {
	schema := projectSchema(map[string]tools.ToolSchemaField{
		"b.second": projectionFixtureField("string", []string{"да", "нет"}, 0, 0, false),
		"a.first":  projectionFixtureField("integer", nil, 1, 10, false),
		"c.third":  projectionFixtureField("boolean", nil, 0, 0, true),
		"d.unset":  projectionFixtureField("integer", nil, 0, 0, false),
	})
	properties := schema["properties"].(map[string]any)

	enumProperty := properties["b.second"].(map[string]any)
	enum, ok := enumProperty["enum"].([]any)
	if !ok || len(enum) != 2 || enum[0] != "да" || enum[1] != "нет" {
		t.Fatalf("словарь значений обязан проецироваться в enum, получено %v", enumProperty["enum"])
	}

	boundsProperty := properties["a.first"].(map[string]any)
	if boundsProperty["minimum"] != 1 || boundsProperty["maximum"] != 10 {
		t.Fatalf("границы обязаны проецироваться, получено %v", boundsProperty)
	}

	// The zero bound is the unset word: it never travels to the wire.
	unsetProperty := properties["d.unset"].(map[string]any)
	if _, carries := unsetProperty["minimum"]; carries {
		t.Fatal("нулевая граница — незаданное слово, она не обязана попадать на провод")
	}
	if _, carries := unsetProperty["maximum"]; carries {
		t.Fatal("нулевая граница — незаданное слово, она не обязана попадать на провод")
	}

	required, ok := schema["required"].([]string)
	if !ok || len(required) != 3 {
		t.Fatalf("обязательные ключи обязаны попадать в required, получено %v", schema["required"])
	}
	sorted := append([]string{}, required...)
	sort.Strings(sorted)
	if strings.Join(required, ",") != strings.Join(sorted, ",") {
		t.Fatalf("required обязан нести стабильный порядок, получено %v", required)
	}
}

// TestProjectionOfTheEmptySchema: the empty schema dictionary projects
// onto the bare object — no required list, never a null.
func TestProjectionOfTheEmptySchema(t *testing.T) {
	schema := projectSchema(map[string]tools.ToolSchemaField{})
	if schema["type"] != "object" {
		t.Fatalf("пустая схема обязана оставаться объектом, получено %v", schema)
	}
	if _, carries := schema["required"]; carries {
		t.Fatal("пустая схема не обязана нести required")
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("кодирование схемы: %v", err)
	}
	if strings.Contains(string(encoded), "null") {
		t.Fatalf("проекция не обязана нести null, получено %s", encoded)
	}
}

// TestProjectToolCarriesTheWireIdentity: the name is the registry id,
// the title is the human name, the description carries the purpose with
// the pin and the dictionary words, and the annotations state the
// read-only fact of the platform dictionary.
func TestProjectToolCarriesTheWireIdentity(t *testing.T) {
	contract := tools.ToolContract{
		ID: "fixture.tool", Version: "2.1.0",
		Name: "Демонстрационный инструмент", Purpose: "Проверяет проекцию",
		DeterminismClass: "deterministic_read", AdapterStatus: "implemented",
		InputSchema:  map[string]tools.ToolSchemaField{"query.text": {Label: "Запрос", Type: "string"}},
		OutputSchema: map[string]tools.ToolSchemaField{"result.word": {Label: "Слово", Type: "string"}},
		Dependency:   &tools.ToolDependency{ConnectorType: "catalog"},
	}
	tool := ProjectTool(contract)
	if tool.Name != contract.ID {
		t.Fatalf("name обязан быть идентификатором реестра, получено %q", tool.Name)
	}
	if tool.Title != contract.Name {
		t.Fatalf("title обязан быть человеческим именем, получено %q", tool.Title)
	}
	for _, word := range []string{contract.Purpose, "fixture.tool@2.1.0", "deterministic_read", "implemented", "catalog"} {
		if !strings.Contains(tool.Description, word) {
			t.Fatalf("description обязан нести %q, получено %q", word, tool.Description)
		}
	}
	if !tool.Annotations.ReadOnlyHint {
		t.Fatal("readOnlyHint — факт платформенного словаря deterministic_read")
	}
	encoded, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("кодирование инструмента: %v", err)
	}
	for _, wireName := range []string{`"name"`, `"title"`, `"description"`, `"inputSchema"`, `"outputSchema"`, `"annotations"`, `"readOnlyHint"`} {
		if !strings.Contains(string(encoded), wireName) {
			t.Fatalf("провод обязан нести поле %s, получено %s", wireName, encoded)
		}
	}
}
