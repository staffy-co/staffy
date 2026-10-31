package mcp

// The strict validation of the call path (Story 7.3 step-02): the
// arguments of the caller are checked against the CLOSED input schema of
// the pinned contract (the unknown key, the type, the enum, the bounds
// and the required list — every refusal names the field, the model
// self-corrects — SEP-1303), and the adapter's output is checked against
// the output schema fail-closed: an undeclared key or a mistyped value
// NEVER travels to the client (the secrets and the internal materials of
// a misbehaving adapter stay inside — AC-2). The validation errors are
// the tool execution errors of the result, never protocol errors.

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/staffy-co/staffy/internal/tools"
)

// decodeCallArguments strictly decodes the arguments member of the call
// params into the raw values of the declared keys (the per-key typing
// happens against the contract schema — the by-name refusals need the
// raw forms).
func decodeCallArguments(raw []byte) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var arguments map[string]json.RawMessage
	if err := strictUnmarshal(raw, &arguments); err != nil {
		return nil, fmt.Errorf("аргументы обязаны быть JSON-объектом")
	}
	return arguments, nil
}

// validateArguments checks the caller's arguments against the closed
// input schema of the contract: every refusal names the field (the
// model self-corrects in one round). The returned values carry the Go
// forms of the declared types (the integers are int64 — the whole
// numbers only).
func validateArguments(schema map[string]tools.ToolSchemaField, arguments map[string]json.RawMessage) (map[string]any, []string) {
	values := map[string]any{}
	refusals := make([]string, 0, len(arguments))
	// The unknown keys: the closed form of the contract (strict decode of
	// the platform — an invented key is a named refusal, never a silent
	// drop).
	declared := make([]string, 0, len(schema))
	for name := range schema {
		declared = append(declared, name)
	}
	sort.Strings(declared)
	for name := range arguments {
		if _, ok := schema[name]; !ok {
			refusals = append(refusals, fmt.Sprintf("неизвестный аргумент %s: контракт не декларирует такого поля", name))
		}
	}
	for _, name := range declared {
		field := schema[name]
		raw, ok := arguments[name]
		if !ok {
			if !field.Optional {
				refusals = append(refusals, fmt.Sprintf("обязательный аргумент %s отсутствует", name))
			}
			continue
		}
		value, refusal := decodeTypedValue(name, field, raw)
		if refusal != "" {
			refusals = append(refusals, refusal)
			continue
		}
		values[name] = value
	}
	return values, refusals
}

// decodeTypedValue decodes one raw argument value against the typed
// field of the contract: the type word, the enum values and the integer
// bounds, every refusal naming the field.
func decodeTypedValue(name string, field tools.ToolSchemaField, raw json.RawMessage) (any, string) {
	trimmed := strings.TrimSpace(string(raw))
	switch field.Type {
	case "string":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Sprintf("аргумент %s обязан быть string (получено %s)", name, describeJSONValue(trimmed))
		}
		if len(field.Values) > 0 && !containsString(field.Values, value) {
			return nil, fmt.Sprintf("аргумент %s вне словаря значений [%s]", name, strings.Join(field.Values, " "))
		}
		return value, ""
	case "integer":
		var number float64
		if err := json.Unmarshal(raw, &number); err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, fmt.Sprintf("аргумент %s обязан быть integer (получено %s)", name, describeJSONValue(trimmed))
		}
		if number != math.Trunc(number) {
			return nil, fmt.Sprintf("аргумент %s обязан быть integer без дробной части (получено %s)", name, trimmed)
		}
		if field.Min != 0 && number < float64(field.Min) {
			return nil, fmt.Sprintf("аргумент %s ниже границы %d (получено %s)", name, field.Min, trimmed)
		}
		if field.Max != 0 && number > float64(field.Max) {
			return nil, fmt.Sprintf("аргумент %s выше границы %d (получено %s)", name, field.Max, trimmed)
		}
		return int64(number), ""
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Sprintf("аргумент %s обязан быть boolean (получено %s)", name, describeJSONValue(trimmed))
		}
		return value, ""
	default:
		// The loader dictionary holds the three words only; an unknown
		// word of a fixture source is the schema's own defect — the named
		// refusal, never a guess.
		return nil, fmt.Sprintf("контракт декларирует неизвестный тип поля %s (%s) — вызов отказан", name, field.Type)
	}
}

// validateOutput checks the adapter's output against the output schema
// fail-closed: an undeclared key, a mistyped value, a value outside the
// enum or the bounds, or a missing required key — the output NEVER
// travels to the client when any refusal exists (the internal material
// of a misbehaving adapter stays inside — AC-2).
func validateOutput(schema map[string]tools.ToolSchemaField, output map[string]any) []string {
	refusals := make([]string, 0, len(output))
	for name := range output {
		if _, ok := schema[name]; !ok {
			refusals = append(refusals, fmt.Sprintf("адаптер вернул незадекларированный ключ %s", name))
		}
	}
	for name, field := range schema {
		value, ok := output[name]
		if !ok {
			if !field.Optional {
				refusals = append(refusals, fmt.Sprintf("адаптер не вернул обязательный ключ %s", name))
			}
			continue
		}
		if refusal := checkOutputValue(name, field, value); refusal != "" {
			refusals = append(refusals, refusal)
		}
	}
	sort.Strings(refusals)
	return refusals
}

// checkOutputValue types one adapter output value against the declared
// field: the Go forms of the adapters (string, the integer kinds, the
// whole float), the enum and the bounds.
func checkOutputValue(name string, field tools.ToolSchemaField, value any) string {
	switch field.Type {
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Sprintf("адаптер вернул ключ %s не строкой", name)
		}
		if len(field.Values) > 0 && !containsString(field.Values, text) {
			return fmt.Sprintf("адаптер вернул ключ %s вне словаря значений [%s]", name, strings.Join(field.Values, " "))
		}
	case "integer":
		number, ok := numberValue(value)
		if !ok {
			return fmt.Sprintf("адаптер вернул ключ %s не целым числом", name)
		}
		if field.Min != 0 && number < float64(field.Min) {
			return fmt.Sprintf("адаптер вернул ключ %s ниже границы %d", name, field.Min)
		}
		if field.Max != 0 && number > float64(field.Max) {
			return fmt.Sprintf("адаптер вернул ключ %s выше границы %d", name, field.Max)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Sprintf("адаптер вернул ключ %s не булевым значением", name)
		}
	default:
		return fmt.Sprintf("контракт декларирует неизвестный тип ключа %s (%s) — выход отказан", name, field.Type)
	}
	return ""
}

// numberValue reads the integer forms of the adapter outputs (int, the
// int variants, the whole float) — never a fractional number.
func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float64:
		if typed != math.Trunc(typed) || math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return typed, true
	default:
		return 0, false
	}
}

// describeJSONValue speaks the honest shape of a refused JSON value (the
// self-correcting model needs the received form).
func describeJSONValue(trimmed string) string {
	switch {
	case trimmed == "":
		return "пусто"
	case trimmed[0] == '"':
		return "строка"
	case trimmed == "true" || trimmed == "false":
		return "булево значение"
	case trimmed == "null":
		return "null"
	case trimmed[0] == '{':
		return "объект"
	case trimmed[0] == '[':
		return "массив"
	default:
		if strings.ContainsAny(trimmed, ".eE") {
			return "дробное число"
		}
		if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return "число"
		}
		return trimmed
	}
}

// containsString is the dictionary membership of the enum words.
func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
