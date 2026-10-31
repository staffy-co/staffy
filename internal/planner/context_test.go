package planner_test

// The context contract matrix of Story 8-1 step-01: the strict decode
// of the Turn Context happens BEFORE any model round — an undeclared
// key, a wrong type or a broken field rule is a NAMED refusal, and a
// refused context is not a turn (no round, no record). The matrix
// walks every rule of the closed form.

import (
	"encoding/json"
	"testing"

	"github.com/staffy-co/staffy/internal/planner"
)

// validContextMaterial builds the wire material of a well-formed turn
// context: two confirmed facts with provenance, the full mode ladder,
// one read tool with a typed input schema (string with enum + integer
// with bounds + optional boolean), the policy pins and the fallback
// policy. The JSON map form keeps the wire honest — the decode sees
// real bytes, never a Go struct shortcut.
func validContextMaterial() map[string]any {
	return map[string]any{
		"turn_id":          "01994f64-6c00-7000-8000-000000008101",
		"tenant_id":        "01994f64-6c00-7000-8000-000000008102",
		"case_id":          "01994f64-6c00-7000-8000-000000008103",
		"conversation_id":  "01994f64-6c00-7000-8000-000000008104",
		"employee_id":      "01994f64-6c00-7000-8000-000000008105",
		"release_manifest": "release-manifest@14.0.0",
		"pack":             "staffy-core-pack@1.0.0",
		"facts": []any{
			map[string]any{
				"key":   "vehicle.make",
				"value": "Renault",
				"provenance": map[string]any{
					"source":  "dialogue.intake",
					"version": "1",
					"at":      "2026-10-31T10:00:00Z",
				},
			},
			map[string]any{
				"key":   "vehicle.year",
				"value": 2019,
				"provenance": map[string]any{
					"source":  "dialogue.intake",
					"version": "1",
					"at":      "2026-10-31T10:00:00Z",
				},
			},
		},
		"mode":   "safety",
		"ladder": []string{"safety", "claim", "cancellation", "privacy_no_contact", "human_requested", "unclassified", "sales"},
		"policy_pins": map[string]any{
			"policy_bundle":  "policy-bundle@3.1.0",
			"allowed_claims": "allowed-claims@2.0.0",
			"autonomy":       "autonomy-envelope@1.0.0",
		},
		"tools": []any{
			map[string]any{
				"name": "catalog.lookup@1.0.0",
				"input_schema": map[string]any{
					"query":                map[string]any{"label": "Поисковый запрос", "type": "string"},
					"limit":                map[string]any{"label": "Лимит", "type": "integer", "min": 1, "max": 50, "optional": true},
					"include_alternatives": map[string]any{"label": "Альтернативы", "type": "boolean", "optional": true},
				},
			},
			map[string]any{
				"name": "vehicle.identify@2.0.0",
				"input_schema": map[string]any{
					"vin": map[string]any{"label": "VIN", "type": "string", "values": []string{"VIN17", "VIN11"}},
				},
			},
		},
		"ai_disclosure_required": true,
		"fallback_policy":        "honest_stop",
	}
}

// decodeContext marshals the material to bytes and decodes it through
// the strict contract surface.
func decodeContext(t *testing.T, material map[string]any) (planner.TurnContext, *planner.RuleError) {
	t.Helper()
	raw, err := json.Marshal(material)
	if err != nil {
		t.Fatalf("маршализация материала контекста: %v", err)
	}
	context, refusal := planner.DecodeTurnContext(raw)
	if refusal != nil {
		return context, refusal
	}
	// A decoded context must round-trip back into compact JSON without
	// losing the closed form (the material of the future prompt
	// assembly — 8.2).
	if _, err := json.Marshal(context); err != nil {
		t.Fatalf("декодированный контекст не маршализуется: %v", err)
	}
	return context, nil
}

func TestContextDecodesValidMaterial(t *testing.T) {
	context, refusal := decodeContext(t, validContextMaterial())
	if refusal != nil {
		t.Fatalf("валидный контекст отклонён: %v", refusal)
	}
	if context.Mode != planner.ModeSafety {
		t.Fatalf("режим контекста обязан быть safety, получен %q", context.Mode)
	}
	if len(context.Facts) != 2 {
		t.Fatalf("контекст обязан нести 2 факта, получено %d", len(context.Facts))
	}
	if len(context.Tools) != 2 {
		t.Fatalf("контекст обязан нести 2 инструмента инвентаря, получено %d", len(context.Tools))
	}
	if !context.AIDisclosureRequired {
		t.Fatal("флаг обязательного раскрытия ИИ потерялся при decode")
	}
	if context.WorkItemID != nil {
		t.Fatal("work_item_id опционален — валидный контекст без него не обязан его нести")
	}
	// The optional work item rides the form when present.
	material := validContextMaterial()
	material["work_item_id"] = "01994f64-6c00-7000-8000-000000008106"
	context, refusal = decodeContext(t, material)
	if refusal != nil {
		t.Fatalf("контекст с work_item_id отклонён: %v", refusal)
	}
	if context.WorkItemID == nil || context.WorkItemID.String() != "01994f64-6c00-7000-8000-000000008106" {
		t.Fatal("work_item_id потерялся при decode")
	}
}

func TestContextRefusesUndeclaredKeyBeforeAnyModelRound(t *testing.T) {
	material := validContextMaterial()
	material["hidden_chain_of_thought"] = "материал, которого не существует в форме"
	_, refusal := decodeContext(t, material)
	if refusal == nil {
		t.Fatal("undeclared-ключ контекста обязан давать поимённый отказ ДО модели")
	}
	if refusal.Rule != "context.strict_decode" {
		t.Fatalf("отказ обязан быть правилом context.strict_decode, получено %q", refusal.Rule)
	}
}

func TestContextRefusesWrongFieldTypeBeforeAnyModelRound(t *testing.T) {
	material := validContextMaterial()
	material["ai_disclosure_required"] = "да"
	_, refusal := decodeContext(t, material)
	if refusal == nil || refusal.Rule != "context.strict_decode" {
		t.Fatalf("неверный тип поля обязан давать отказ context.strict_decode, получено %v", refusal)
	}
}

func TestContextRefusesTrailingMaterial(t *testing.T) {
	raw, err := json.Marshal(validContextMaterial())
	if err != nil {
		t.Fatalf("маршализация: %v", err)
	}
	_, refusal := planner.DecodeTurnContext(append(raw, []byte(`{"second":"object"}`)...))
	if refusal == nil || refusal.Rule != "context.strict_decode" {
		t.Fatalf("хвостовой материал после JSON-объекта обязан давать отказ, получено %v", refusal)
	}
}

func TestContextFieldRuleMatrix(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(material map[string]any)
		rule   string
	}{
		{
			name:   "невалидный UUID случая",
			mutate: func(material map[string]any) { material["case_id"] = "не-uuid" },
			rule:   "context.ids",
		},
		{
			name:   "референс релиза без версии",
			mutate: func(material map[string]any) { material["release_manifest"] = "release-manifest" },
			rule:   "context.refs",
		},
		{
			name: "пин политики без идентификатора",
			mutate: func(material map[string]any) {
				pins := material["policy_pins"].(map[string]any)
				pins["policy_bundle"] = "@3.1.0"
			},
			rule: "context.refs",
		},
		{
			name: "факт без provenance",
			mutate: func(material map[string]any) {
				material["facts"] = []any{map[string]any{
					"key": "vehicle.make", "value": "Renault",
					"provenance": map[string]any{"source": "", "version": "1", "at": "2026-10-31T10:00:00Z"},
				}}
			},
			rule: "context.fact_provenance",
		},
		{
			name: "факт со значением-дробью вне словаря типизации",
			mutate: func(material map[string]any) {
				material["facts"] = []any{map[string]any{
					"key": "vehicle.weight", "value": 1.5,
					"provenance": map[string]any{"source": "dialogue.intake", "version": "1", "at": "2026-10-31T10:00:00Z"},
				}}
			},
			rule: "context.fact_value",
		},
		{
			name: "факт с массивом вместо платформенного значения",
			mutate: func(material map[string]any) {
				material["facts"] = []any{map[string]any{
					"key": "vehicle.tags", "value": []any{"a", "b"},
					"provenance": map[string]any{"source": "dialogue.intake", "version": "1", "at": "2026-10-31T10:00:00Z"},
				}}
			},
			rule: "context.fact_value",
		},
		{
			name:   "пустой список фактов",
			mutate: func(material map[string]any) { material["facts"] = []any{} },
			rule:   "context.facts",
		},
		{
			name: "дубликат ключа факта",
			mutate: func(material map[string]any) {
				fact := material["facts"].([]any)[0].(map[string]any)
				material["facts"] = []any{fact, fact}
			},
			rule: "context.facts",
		},
		{
			name:   "режим вне словаря",
			mutate: func(material map[string]any) { material["mode"] = "friendly" },
			rule:   "context.mode",
		},
		{
			name: "лестница короче платформенной",
			mutate: func(material map[string]any) {
				material["ladder"] = []string{"safety", "claim"}
			},
			rule: "context.ladder",
		},
		{
			name: "лестница в чужом порядке",
			mutate: func(material map[string]any) {
				material["ladder"] = []string{"claim", "safety", "cancellation", "privacy_no_contact", "human_requested", "unclassified", "sales"}
			},
			rule: "context.ladder",
		},
		{
			name: "инструмент без версии",
			mutate: func(material map[string]any) {
				tools := material["tools"].([]any)
				tools[0].(map[string]any)["name"] = "catalog.lookup"
			},
			rule: "context.tools",
		},
		{
			name: "дубликат инструмента инвентаря",
			mutate: func(material map[string]any) {
				tools := material["tools"].([]any)
				material["tools"] = []any{tools[0], tools[0]}
			},
			rule: "context.tools",
		},
		{
			name: "тип поля схемы вне словаря типизации",
			mutate: func(material map[string]any) {
				tool := material["tools"].([]any)[0].(map[string]any)
				tool["input_schema"].(map[string]any)["query"].(map[string]any)["type"] = "float"
			},
			rule: "context.tools",
		},
		{
			name: "enum-значения у нестрокового поля",
			mutate: func(material map[string]any) {
				tool := material["tools"].([]any)[0].(map[string]any)
				tool["input_schema"].(map[string]any)["limit"].(map[string]any)["values"] = []string{"1", "2"}
			},
			rule: "context.tools",
		},
		{
			name:   "fallback-политика вне словаря",
			mutate: func(material map[string]any) { material["fallback_policy"] = "retry" },
			rule:   "context.fallback_policy",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			material := validContextMaterial()
			testCase.mutate(material)
			_, refusal := decodeContext(t, material)
			if refusal == nil {
				t.Fatalf("материал обязан быть отклонён по правилу %s", testCase.rule)
			}
			if refusal.Rule != testCase.rule {
				t.Fatalf("отказ обязан быть правилом %s, получено %q (%s)", testCase.rule, refusal.Rule, refusal.Detail)
			}
		})
	}
}

func TestContextIntegerFactsStayStrictIntegerLiterals(t *testing.T) {
	material := validContextMaterial()
	material["facts"] = []any{map[string]any{
		"key": "vehicle.year", "value": json.Number("2019"),
		"provenance": map[string]any{"source": "dialogue.intake", "version": "1", "at": "2026-10-31T10:00:00Z"},
	}}
	_, refusal := decodeContext(t, material)
	if refusal != nil {
		t.Fatalf("целочисленный факт обязан проходить: %v", refusal)
	}
	// The exponent form of the same number is NOT a strict integer
	// literal — the platform typing refuses it by name.
	_, refusal = planner.DecodeTurnContext([]byte(patchContextFact(`{"facts":[{"key":"vehicle.year","value":2e3,"provenance":{"source":"dialogue.intake","version":"1","at":"2026-10-31T10:00:00Z"}}]}`)))
	if refusal == nil || refusal.Rule != "context.fact_value" {
		t.Fatalf("экспоненциальная форма числа обязана давать отказ context.fact_value, получено %v", refusal)
	}
	// The fraction form is refused by the same rule.
	_, refusal = planner.DecodeTurnContext([]byte(patchContextFact(`{"facts":[{"key":"vehicle.year","value":2019.5,"provenance":{"source":"dialogue.intake","version":"1","at":"2026-10-31T10:00:00Z"}}]}`)))
	if refusal == nil || refusal.Rule != "context.fact_value" {
		t.Fatalf("дробная форма числа обязана давать отказ context.fact_value, получено %v", refusal)
	}
}

// patchContextFact splices a raw facts fragment into a full valid
// context wire — the byte-level probe of the typing rules.
func patchContextFact(factsFragment string) string {
	base := `{"turn_id":"01994f64-6c00-7000-8000-000000008101","tenant_id":"01994f64-6c00-7000-8000-000000008102","case_id":"01994f64-6c00-7000-8000-000000008103","conversation_id":"01994f64-6c00-7000-8000-000000008104","employee_id":"01994f64-6c00-7000-8000-000000008105","release_manifest":"release-manifest@14.0.0","pack":"staffy-core-pack@1.0.0",`
	return base + factsFragment[1:len(factsFragment)-1] + `,"mode":"safety","ladder":["safety","claim","cancellation","privacy_no_contact","human_requested","unclassified","sales"],"policy_pins":{"policy_bundle":"policy-bundle@3.1.0","allowed_claims":"allowed-claims@2.0.0","autonomy":"autonomy-envelope@1.0.0"},"tools":[],"ai_disclosure_required":true,"fallback_policy":"honest_stop"}`
}

func TestContextAcceptsEveryFallbackPolicyWord(t *testing.T) {
	for _, policy := range planner.FallbackDictionary {
		material := validContextMaterial()
		material["fallback_policy"] = policy
		if _, refusal := decodeContext(t, material); refusal != nil {
			t.Fatalf("fallback-политика %q — закрытое слово словаря, отказ недопустим: %v", policy, refusal)
		}
	}
}

func TestContextProvenanceTimeIsMandatory(t *testing.T) {
	material := validContextMaterial()
	material["facts"] = []any{map[string]any{
		"key": "vehicle.make", "value": "Renault",
		"provenance": map[string]any{"source": "dialogue.intake", "version": "1"},
	}}
	_, refusal := decodeContext(t, material)
	if refusal == nil || refusal.Rule != "context.fact_provenance" {
		t.Fatalf("факт без времени наблюдения обязан давать отказ provenance, получено %v", refusal)
	}
}
