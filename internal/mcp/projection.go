package mcp

// The projection of the platform contract dictionary onto the JSON
// Schema wire of the MCP tools (Story 7.3 step-02): the TOTAL 1:1 mapping
// of the pack typing dictionary (string | integer | boolean with the
// enum values, the integer bounds and the optional flag). The projection
// is one-way: the registry stays the platform language (json-schema.org
// never enters the artifacts — the Never of 7-2 holds); this file is the
// MCP WIRE only. The exposed tool carries the read-only hint of the
// safety class (every tool of the registry is a deterministic read — the
// dictionary carries the single value).

import (
	"fmt"
	"sort"

	"github.com/staffy-co/staffy/internal/tools"
)

// Tool is the MCP wire projection of one registry contract.
type Tool struct {
	Name         string          `json:"name"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  map[string]any  `json:"inputSchema"`
	OutputSchema map[string]any  `json:"outputSchema"`
	Annotations  ToolAnnotations `json:"annotations"`
}

// ToolAnnotations carries the behavioral hints of the tool: the read-only
// hint is the fact of the platform dictionary (deterministic_read).
type ToolAnnotations struct {
	ReadOnlyHint bool `json:"readOnlyHint"`
}

// The server identity of the discover answer: the name of the platform
// and the version MIRROR of the OpenAPI info.version (the contract test
// of the OpenAPI mirror holds the equality — a platform version change is
// a conscious step in both places).
const (
	ServerName    = "staffy"
	ServerVersion = "0.1.0"
)

// ProjectTool maps one registry contract onto the MCP wire tool: the
// name is the registry id, the title is the human name, the description
// carries the purpose with the contract pin and the dictionary words,
// the schemas are the total JSON Schema projection of the platform
// typing dictionary, and the annotations state the read-only fact.
func ProjectTool(contract tools.ToolContract) Tool {
	return Tool{
		Name:         contract.ID,
		Title:        contract.Name,
		Description:  toolDescription(contract),
		InputSchema:  projectSchema(contract.InputSchema),
		OutputSchema: projectSchema(contract.OutputSchema),
		Annotations:  ToolAnnotations{ReadOnlyHint: true},
	}
}

// toolDescription speaks the purpose with the honest pin and the
// dictionary words — the wire carries the versioned identity of the
// contract the caller is about to rely on.
func toolDescription(contract tools.ToolContract) string {
	dependency := "нет (самодостаточный контракт)"
	if contract.Dependency != nil {
		dependency = "коннектор типа " + contract.Dependency.ConnectorType
	}
	return fmt.Sprintf("%s (контракт %s@%s, детерминизм %s, адаптер %s, зависимость: %s)",
		contract.Purpose, contract.ID, contract.Version, contract.DeterminismClass, contract.AdapterStatus, dependency)
}

// projectSchema builds the JSON Schema object of one platform schema
// dictionary: the closed property set (the unknown key of the arguments
// is the named refusal of the call path — the schema declares exactly the
// contract keys), the enum of the declared values, the integer bounds
// and the required list of the non-optional keys. The platform bounds
// dictionary carries 0 as the unset word: a zero bound does not travel
// to the wire (the loader discipline of the artifacts).
func projectSchema(schema map[string]tools.ToolSchemaField) map[string]any {
	properties := map[string]any{}
	required := make([]string, 0, len(schema))
	for name, field := range schema {
		property := map[string]any{
			"title": field.Label,
			"type":  field.Type,
		}
		if len(field.Values) > 0 {
			values := make([]any, 0, len(field.Values))
			for _, value := range field.Values {
				values = append(values, value)
			}
			property["enum"] = values
		}
		if field.Type == "integer" {
			if field.Min != 0 {
				property["minimum"] = field.Min
			}
			if field.Max != 0 {
				property["maximum"] = field.Max
			}
		}
		properties[name] = property
		if !field.Optional {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	object := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		object["required"] = required
	}
	return object
}
