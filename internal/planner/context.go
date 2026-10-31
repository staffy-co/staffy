package planner

// The Turn Context (Story 8-1): the minimized, closed, typed form of
// everything a turn needs — the snapshot material the future context
// builder (Story 8.4) assembles from the live case, conversation and
// Work Item, and the ONLY material a model ever sees (through the
// adapter's prompt assembly — Story 8.2). The form is strict: the
// decode refuses undeclared keys and wrong types BEFORE any model
// round (AD-11 — the model input is a validated, closed surface), no
// secrets exist in it, no hidden chain of thought (AD-8), no foreign
// tenant material. Every reference is a REF (id@version), never the
// material itself — the pins travel as references so the journal and
// the runtime can correlate without carrying policy bundles around.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// Provenance is the mandatory origin record of every fact: the source,
// its version and the moment of the observation. A fact without
// provenance does not exist in the platform (Story 2.2 — the same
// discipline the proposal's own fact suggestions carry).
type Provenance struct {
	Source  string    `json:"source"`
	Version string    `json:"version"`
	At      time.Time `json:"at"`
}

// ContextFact is one confirmed fact of the snapshot with its
// provenance. The value is a platform typed value: string | integer |
// boolean (the typing dictionary of the read arguments — the same
// language the tool schemas speak).
type ContextFact struct {
	Key        string     `json:"key"`
	Value      any        `json:"value"`
	Provenance Provenance `json:"provenance"`
}

// PolicyPins are the release-scoped policy references of the turn:
// the policy bundle, the Allowed Claims set and the autonomy mode —
// all as id@version REFS. The material behind the pins is assembled by
// the prompt builder (8.2/8.4); the contract only pins the identity.
type PolicyPins struct {
	PolicyBundleRef  string `json:"policy_bundle"`
	AllowedClaimsRef string `json:"allowed_claims"`
	AutonomyRef      string `json:"autonomy"`
}

// ToolSchemaField is one typed key of an inventory tool's input
// schema: the form mirror of tools.ToolSchemaField (Story 7-2 —
// label, type, enum values, integer bounds, optional flag; 0 bounds
// mean unset). The planner never extends the typing dictionary: the
// words are the mirror of tools.ToolFieldTypes, pinned by the
// contract test.
type ToolSchemaField struct {
	Label    string   `json:"label"`
	Type     string   `json:"type"` // string | integer | boolean
	Values   []string `json:"values,omitempty"`
	Min      int      `json:"min,omitempty"`
	Max      int      `json:"max,omitempty"`
	Optional bool     `json:"optional,omitempty"`
}

// InventoryTool is one read tool available to the turn: the name@version
// of the tool (the registry identity of 7-2 or the external dictionary
// identity of 7-4) plus its typed input schema. The inventory is the
// VALIDATION surface of the proposal's read requests — a request for a
// tool that is not in this inventory is a named refusal, and so is an
// argument outside the tool's schema (the TOCTOU-fail-closed spirit of
// 7-4: the turn validates against what the context declared, not
// against what the world might currently have).
type InventoryTool struct {
	Name        string                     `json:"name"`
	InputSchema map[string]ToolSchemaField `json:"input_schema"`
}

// TurnContext is the closed form of the turn input.
type TurnContext struct {
	TurnID         id.UUID  `json:"turn_id"`
	TenantID       id.UUID  `json:"tenant_id"`
	CaseID         id.UUID  `json:"case_id"`
	ConversationID id.UUID  `json:"conversation_id"`
	WorkItemID     *id.UUID `json:"work_item_id,omitempty"`
	EmployeeID     id.UUID  `json:"employee_id"`
	// ReleaseManifestRef and PackRef pin the release material the
	// conversation runs on (id@version refs — the Release Manifest of
	// Story 1-12 and the domain pack of 1-4).
	ReleaseManifestRef string        `json:"release_manifest"`
	PackRef            string        `json:"pack"`
	Facts              []ContextFact `json:"facts"`
	// Mode is the conversation mode word (the ladder dictionary); the
	// proposal may not be lower on the ladder than this word.
	Mode string `json:"mode"`
	// Ladder is the active priority ladder the context carries for the
	// model to reason with: it must be EXACTLY the platform ModeLadder
	// mirror (the fixed FR-14 order — a context never invents a
	// private ladder).
	Ladder []string `json:"ladder"`
	// PolicyPins, Tools, AIDisclosureRequired and FallbackPolicy close
	// the form: the policy refs, the read-tool inventory, the §8.1 AI
	// disclosure requirement flag and the declared fallback policy of
	// the turn (which approved safe form ends a failed turn).
	PolicyPins           PolicyPins      `json:"policy_pins"`
	Tools                []InventoryTool `json:"tools"`
	AIDisclosureRequired bool            `json:"ai_disclosure_required"`
	FallbackPolicy       string          `json:"fallback_policy"`
}

// DecodeTurnContext strictly decodes and validates the raw turn
// context: an undeclared key, a wrong type or a broken field rule is a
// NAMED refusal, and the refusal happens BEFORE any model round — the
// model input is a validated closed surface or it is nothing at all.
// A refused context is not a turn: no round, no journal record.
func DecodeTurnContext(raw []byte) (TurnContext, *RuleError) {
	var context TurnContext
	if err := strictDecode(raw, &context); err != nil {
		return TurnContext{}, rule("context.strict_decode", "контекст хода не прошёл строгий decode: %v", err.Error())
	}
	if refusal := validateTurnContext(context); refusal != nil {
		return TurnContext{}, refusal
	}
	return context, nil
}

// validateTurnContext holds the closed-form rules of the context: the
// mandatory identifier fields, the ref formats, the provenance of
// every fact, the mode and ladder membership, the tool inventory form
// and the fallback policy word. Every rule is a named refusal.
func validateTurnContext(context TurnContext) *RuleError {
	for _, pair := range []struct {
		name  string
		value id.UUID
	}{
		{"turn_id", context.TurnID},
		{"tenant_id", context.TenantID},
		{"case_id", context.CaseID},
		{"conversation_id", context.ConversationID},
		{"employee_id", context.EmployeeID},
	} {
		if !pair.value.Valid() {
			return rule("context.ids", "поле %s обязано быть валидным UUID", pair.name)
		}
	}
	if context.WorkItemID != nil && !context.WorkItemID.Valid() {
		return rule("context.ids", "поле work_item_id обязано быть валидным UUID")
	}
	for _, pair := range []struct {
		name  string
		value string
	}{
		{"release_manifest", context.ReleaseManifestRef},
		{"pack", context.PackRef},
		{"policy_pins.policy_bundle", context.PolicyPins.PolicyBundleRef},
		{"policy_pins.allowed_claims", context.PolicyPins.AllowedClaimsRef},
		{"policy_pins.autonomy", context.PolicyPins.AutonomyRef},
	} {
		if !isRef(pair.value) {
			return rule("context.refs", "поле %s обязано быть референсом вида id@версия (пины — референсами, не материалом)", pair.name)
		}
	}
	if len(context.Facts) == 0 {
		return rule("context.facts", "снимок хода обязан нести хотя бы один подтверждённый факт с provenance")
	}
	seenFacts := make(map[string]bool, len(context.Facts))
	for _, fact := range context.Facts {
		if fact.Key == "" {
			return rule("context.facts", "ключ факта не может быть пустым")
		}
		if seenFacts[fact.Key] {
			return rule("context.facts", "дубликат ключа факта %q — снимок не может нести один ключ дважды", fact.Key)
		}
		seenFacts[fact.Key] = true
		if refusal := platformValueRule("context.fact_value", fact.Key, fact.Value); refusal != nil {
			return refusal
		}
		if refusal := provenanceRule("context.fact_provenance", fact.Key, fact.Provenance); refusal != nil {
			return refusal
		}
	}
	if !isDictionaryWord(context.Mode, ModeLadder) {
		return rule("context.mode", "режим разговора %q вне закрытого словаря лестницы приоритетов", context.Mode)
	}
	if len(context.Ladder) != len(ModeLadder) {
		return rule("context.ladder", "активная лестница обязана быть зеркалом платформенной лестницы приоритетов целиком")
	}
	for index, word := range ModeLadder {
		if context.Ladder[index] != word {
			return rule("context.ladder", "активная лестница обязана быть точным зеркалом платформенной лестницы (позиция %d: %q ≠ %q)", index, context.Ladder[index], word)
		}
	}
	seenTools := make(map[string]bool, len(context.Tools))
	for _, tool := range context.Tools {
		if !isRef(tool.Name) {
			return rule("context.tools", "инструмент инвентаря %q обязан быть именем@версией", tool.Name)
		}
		if seenTools[tool.Name] {
			return rule("context.tools", "дубликат инструмента инвентаря %q", tool.Name)
		}
		seenTools[tool.Name] = true
		for field, schema := range tool.InputSchema {
			if schema.Label == "" {
				return rule("context.tools", "поле %q инструмента %q обязано нести человекочитаемую метку", field, tool.Name)
			}
			if !isDictionaryWord(schema.Type, ReadArgTypes) {
				return rule("context.tools", "тип поля %q инструмента %q вне платформенного словаря типизации", field, tool.Name)
			}
			if len(schema.Values) > 0 && schema.Type != "string" {
				return rule("context.tools", "enum-значения поля %q инструмента %q допустимы только для строкового типа", field, tool.Name)
			}
			if (schema.Min != 0 || schema.Max != 0) && schema.Type != "integer" {
				return rule("context.tools", "целые границы поля %q инструмента %q допустимы только для целого типа", field, tool.Name)
			}
		}
	}
	if !isDictionaryWord(context.FallbackPolicy, FallbackDictionary) {
		return rule("context.fallback_policy", "fallback-политика хода %q вне закрытого словаря утверждённых безопасных форм", context.FallbackPolicy)
	}
	return nil
}

// platformValueRule checks a platform typed value: string | integer |
// boolean. The integer arrives as json.Number from the strict decoder
// (UseNumber) and must be a strict integer literal — a fraction or an
// exponent is a typing refusal, not a rounding.
func platformValueRule(name, key string, value any) *RuleError {
	switch typed := value.(type) {
	case string, bool:
		return nil
	case json.Number:
		if !isIntegerText(typed.String()) {
			return rule(name, "значение ключа %q обязано быть строгим целым литералом, получено %q", key, typed.String())
		}
		return nil
	default:
		return rule(name, "значение ключа %q вне платформенного словаря типизации (string | integer | boolean)", key)
	}
}

// provenanceRule checks the mandatory provenance record of a fact.
func provenanceRule(name, key string, provenance Provenance) *RuleError {
	if provenance.Source == "" || provenance.Version == "" {
		return rule(name, "факт %q обязан нести provenance {источник, версия, время} целиком", key)
	}
	if provenance.At.IsZero() {
		return rule(name, "факт %q обязан нести время наблюдения provenance", key)
	}
	return nil
}

// strictDecode is the closed-form decode of the platform: an unknown
// key of the destination struct is an error, trailing material is an
// error, and numbers stay as json.Number so the integer typing rule
// can see the literal shape (the 7-3 strictUnmarshal discipline plus
// UseNumber for the any-typed values).
func strictDecode(raw []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("хвостовой материал после JSON-объекта")
	}
	return nil
}
