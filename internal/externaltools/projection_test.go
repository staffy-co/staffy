package externaltools

// The projection matrix (Story 7-4 step-02): the subset is TOTAL — every
// descriptor inside {string | integer | boolean + enum + integer bounds +
// required} projects onto the platform typing dictionary; every fact
// outside the subset refuses the row BY NAME (a fractional number, an
// object or array type, an anyOf combinator, a non-string enum item, a
// fractional bound, a missing read-only statement — AR-30). The refusals
// are rows of the dictionary, never silences.

import (
	"strings"
	"testing"
)

func schema(properties map[string]any, required ...string) map[string]any {
	object := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		list := make([]any, 0, len(required))
		for _, name := range required {
			list = append(list, name)
		}
		object["required"] = list
	}
	return object
}

func TestProjectionSubsetIsTotal(t *testing.T) {
	tool := projectedTool{
		Name: "parts.lookup", Title: "Поиск детали", Description: "Читает каталог",
		ReadOnlyHint: true,
		InputSchema: schema(map[string]any{
			"query":  map[string]any{"type": "string", "title": "Запрос"},
			"limit":  map[string]any{"type": "integer", "minimum": float64(1), "maximum": float64(50)},
			"deep":   map[string]any{"type": "boolean"},
			"source": map[string]any{"type": "string", "enum": []any{"catalog", "cross"}},
		}, "query"),
		OutputSchema: schema(map[string]any{
			"found": map[string]any{"type": "boolean"},
			"count": map[string]any{"type": "integer", "minimum": float64(0)},
		}, "found"),
	}
	input, output, refusal := projectTool(tool)
	if refusal != "" {
		t.Fatalf("подмножество обязано проецироваться тотально: %q", refusal)
	}
	if input["query"].Type != "string" || input["query"].Label != "Запрос" || input["query"].Optional {
		t.Fatalf("поле query: %+v", input["query"])
	}
	if field := input["limit"]; field.Type != "integer" || field.Min != 1 || field.Max != 50 || !field.Optional {
		t.Fatalf("поле limit: %+v", field)
	}
	if field := input["source"]; len(field.Values) != 2 || field.Values[0] != "catalog" {
		t.Fatalf("поле source: %+v", field)
	}
	if output["count"].Min != 0 || output["count"].Type != "integer" {
		t.Fatalf("поле count: %+v", output["count"])
	}
	if output["found"].Optional {
		t.Fatalf("поле found обязано быть обязательным")
	}
}

func TestProjectionRefusalMatrixNamesEveryFact(t *testing.T) {
	cases := []struct {
		name    string
		tool    projectedTool
		refusal string
	}{
		{"число с дробной частью", projectedTool{
			Name: "t.one", ReadOnlyHint: true,
			InputSchema:  schema(map[string]any{"ratio": map[string]any{"type": "number"}}),
			OutputSchema: schema(map[string]any{}),
		}, "вне платформенного словаря типизации"},
		{"тип object", projectedTool{
			Name: "t.two", ReadOnlyHint: true,
			InputSchema:  schema(map[string]any{"filter": map[string]any{"type": "object"}}),
			OutputSchema: schema(map[string]any{}),
		}, "вне платформенного словаря типизации"},
		{"тип array", projectedTool{
			Name: "t.three", ReadOnlyHint: true,
			InputSchema:  schema(map[string]any{"rows": map[string]any{"type": "array"}}),
			OutputSchema: schema(map[string]any{}),
		}, "вне платформенного словаря типизации"},
		{"комбинатор anyOf", projectedTool{
			Name: "t.four", ReadOnlyHint: true,
			InputSchema:  map[string]any{"type": "object", "properties": map[string]any{}, "anyOf": []any{}},
			OutputSchema: schema(map[string]any{}),
		}, "комбинатор anyOf"},
		{"дробная граница", projectedTool{
			Name: "t.five", ReadOnlyHint: true,
			InputSchema:  schema(map[string]any{"n": map[string]any{"type": "integer", "minimum": 0.5}}),
			OutputSchema: schema(map[string]any{}),
		}, "вне целых"},
		{"enum не строками", projectedTool{
			Name: "t.six", ReadOnlyHint: true,
			InputSchema:  schema(map[string]any{"k": map[string]any{"type": "string", "enum": []any{"a", 7}}}),
			OutputSchema: schema(map[string]any{}),
		}, "обязан listing строки"},
		{"корень не object", projectedTool{
			Name: "t.seven", ReadOnlyHint: true,
			InputSchema:  map[string]any{"type": "array"},
			OutputSchema: schema(map[string]any{}),
		}, "корень схемы обязан быть object"},
		{"отказ по выходной схеме", projectedTool{
			Name: "t.eight", ReadOnlyHint: true,
			InputSchema:  schema(map[string]any{}),
			OutputSchema: schema(map[string]any{"bad": map[string]any{"type": "object"}}),
		}, "выход"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, refusal := projectTool(testCase.tool)
			if refusal == "" {
				t.Fatal("факт вне подмножества обязан отказываться поимённо")
			}
			if !strings.Contains(refusal, testCase.refusal) {
				t.Fatalf("отказ обязан называть факт %q: %q", testCase.refusal, refusal)
			}
			if !strings.Contains(refusal, testCase.tool.Name) {
				t.Fatalf("отказ обязан называть инструмент: %q", refusal)
			}
		})
	}
}

func TestProjectionReadOnlyHintIsMandatory(t *testing.T) {
	tool := projectedTool{
		Name: "parts.mutate", Title: "Мутация", ReadOnlyHint: false,
		InputSchema:  schema(map[string]any{"x": map[string]any{"type": "string"}}),
		OutputSchema: schema(map[string]any{}),
	}
	_, _, refusal := projectTool(tool)
	if refusal == "" || !strings.Contains(refusal, "AR-30") || !strings.Contains(refusal, "read-only") {
		t.Fatalf("отказ без readOnlyHint обязан называть AR-30: %q", refusal)
	}
}

func TestProjectionRefusesTheNamelessDescriptor(t *testing.T) {
	_, _, refusal := projectTool(projectedTool{Name: "  ", ReadOnlyHint: true, InputSchema: schema(map[string]any{}), OutputSchema: schema(map[string]any{})})
	if refusal == "" || !strings.Contains(refusal, "без имени") {
		t.Fatalf("безымянный дескриптор: %q", refusal)
	}
}
