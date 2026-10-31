package externaltools

// The total projection of the JSON Schema subset onto the platform typing
// dictionary (Story 7-4 step-02): the root object with the properties of
// the types {string | integer | boolean}, the enum values, the INTEGER
// bounds and the required list. The subset is TOTAL — every descriptor
// inside it projects; anything outside (a fractional number, an object or
// array type, an anyOf combinator, a non-string enum item, a fractional
// bound) is the NAMED refusal carried as a rejected row (a row of the
// dictionary, never a silence — AD-11: the material that failed never
// travels further). A descriptor without the read-only statement is never
// admitted (AR-30: the server must state the read-only fact).

import (
	"fmt"
	"sort"
	"strings"

	"github.com/staffy-co/staffy/internal/tools"
)

// The typed words of the platform typing dictionary (the mirror of the
// registry dictionary of 7-2 — the projection never extends it).
var platformTypes = map[string]bool{"string": true, "integer": true, "boolean": true}

// projectedTool is one filtered descriptor of the inventory snapshot.
type projectedTool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	ReadOnlyHint bool           `json:"readOnlyHint"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema"`
}

// projectedInventory is the filtered snapshot form of the probe evidence
// (the compact JSON carried inside the redacted result).
type projectedInventory struct {
	Server struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"server"`
	Tools []projectedTool `json:"tools"`
}

// projectTool maps one filtered descriptor onto the platform typing
// dictionary. The refusal is NAMED per fact; the readOnlyHint statement is
// the admission gate (AR-30).
func projectTool(tool projectedTool) (input, output map[string]tools.ToolSchemaField, refusal string) {
	if strings.TrimSpace(tool.Name) == "" {
		return nil, nil, "дескриптор без имени инструмента"
	}
	if !tool.ReadOnlyHint {
		return nil, nil, fmt.Sprintf("инструмент %s не заявляет read-only (annotations.readOnlyHint): без заявления сервера read-only инструмент не допускается (AR-30)", tool.Name)
	}
	input, refusal = projectSchema("вход", tool.InputSchema)
	if refusal != "" {
		return nil, nil, fmt.Sprintf("инструмент %s: %s", tool.Name, refusal)
	}
	output, refusal = projectSchema("выход", tool.OutputSchema)
	if refusal != "" {
		return nil, nil, fmt.Sprintf("инструмент %s: %s", tool.Name, refusal)
	}
	return input, output, ""
}

// projectSchema maps ONE JSON Schema object onto the platform typing
// dictionary: the root type must be the object word, every property type
// must be a dictionary word, the enum items must be strings, the bounds
// must be whole integers and the required list must name declared
// properties. Keywords outside the typing system (titles, descriptions,
// defaults) do not participate in the projection and are ignored — the
// TYPE facts decide the admission.
func projectSchema(side string, schema map[string]any) (map[string]tools.ToolSchemaField, string) {
	if schema == nil {
		return nil, side + ": схема отсутствует"
	}
	rootType, _ := schema["type"].(string)
	if rootType != "object" {
		return nil, fmt.Sprintf("%s: корень схемы обязан быть object (заявлено %q)", side, rootType)
	}
	for _, combinator := range []string{"anyOf", "oneOf", "allOf"} {
		if _, ok := schema[combinator]; ok {
			return nil, fmt.Sprintf("%s: комбинатор %s вне подмножества проекции", side, combinator)
		}
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, side + ": свойства схемы обязаны быть объектом"
	}
	required := map[string]bool{}
	if list, ok := schema["required"].([]any); ok {
		for _, item := range list {
			name, ok := item.(string)
			if !ok {
				return nil, side + ": список required обязан naming строки"
			}
			required[name] = true
		}
	}
	out := make(map[string]tools.ToolSchemaField, len(properties))
	for name, raw := range properties {
		property, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Sprintf("%s: свойство %s обязано быть объектом", side, name)
		}
		word, _ := property["type"].(string)
		if !platformTypes[word] {
			return nil, fmt.Sprintf("%s: тип свойства %s (%q) вне платформенного словаря типизации (string | integer | boolean)", side, name, word)
		}
		field := tools.ToolSchemaField{Type: word, Optional: !required[name]}
		if label, ok := property["title"].(string); ok && label != "" {
			field.Label = label
		} else {
			field.Label = name
		}
		if values, ok := property["enum"].([]any); ok {
			for _, item := range values {
				value, ok := item.(string)
				if !ok {
					return nil, fmt.Sprintf("%s: enum свойства %s обязан listing строки", side, name)
				}
				field.Values = append(field.Values, value)
			}
		}
		if word == "integer" {
			if raw, ok := property["minimum"]; ok {
				number, whole, valid := wholeNumber(raw)
				if !valid || !whole {
					return nil, fmt.Sprintf("%s: граница minimum свойства %s вне целых", side, name)
				}
				field.Min = number
			}
			if raw, ok := property["maximum"]; ok {
				number, whole, valid := wholeNumber(raw)
				if !valid || !whole {
					return nil, fmt.Sprintf("%s: граница maximum свойства %s вне целых", side, name)
				}
				field.Max = number
			}
		}
		out[name] = field
	}
	return out, ""
}

// wholeNumber decodes a JSON number that must be a whole integer: a
// fractional value is outside the projection subset.
func wholeNumber(raw any) (value int, whole bool, valid bool) {
	switch number := raw.(type) {
	case float64:
		return int(number), number == float64(int(number)), true
	case int:
		return number, true, true
	default:
		return 0, false, false
	}
}

// sortedNames returns the alphabetically sorted keys (the stable row
// order of the dictionary — Go sorts map keys, the reads stay stable).
func sortedNames(schema map[string]tools.ToolSchemaField) []string {
	names := make([]string, 0, len(schema))
	for name := range schema {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
