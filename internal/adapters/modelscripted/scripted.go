package modelscripted

// The scripted model adapter (Story 8-1; the material synthesis of
// Story 8-2): an explicitly fictional model over the planner
// ModelPort — the honest test material of PRODUCTION code, the
// verifier.Synthetic precedent of Story 1-8 applied to the model
// port. The adapter is configured with ONE outcome word of the closed
// matrix; every Round produces exactly the material of that outcome,
// deterministically, FROM THE ASSEMBLED ROUND MATERIAL (Story 8-2:
// the adapter answers only what arrived in the material — the
// completeness proof of the platform assembly):
//
//      valid               — a well-formed proposal that passes the whole
//                            validation matrix (mode mirrors the material
//                            mode, the §8.1 pin disclosure rides the reply
//                            when the material demands it, reply within
//                            the budget)
//      malformed_undeclared — a proposal carrying an undeclared key
//      malformed_type      — a proposal carrying a wrong field type
//      foreign_version     — a proposal carrying a foreign schema version
//      repairable          — round 1 malformed, round 2 valid (the repair
//                            cause inside the material switches it)
//      unrecoverable       — round 1 malformed, round 2 a foreign version
//      adapter_error       — an honest adapter error
//
// main.go NEVER wires this adapter (zero production callers — the
// caller of the planner arrives with Story 8.4). No credentials, no
// network, no SDK: the standard library only. The adapter never sees
// the Turn Context — the port carries the assembled material only
// (AD-8 by construction).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/staffy-co/staffy/internal/planner"
)

// The outcome dictionary of the scripted matrix.
const (
	OutcomeValid               = "valid"
	OutcomeMalformedUndeclared = "malformed_undeclared"
	OutcomeMalformedType       = "malformed_type"
	OutcomeForeignVersion      = "foreign_version"
	OutcomeRepairable          = "repairable"
	OutcomeUnrecoverable       = "unrecoverable"
	OutcomeAdapterError        = "adapter_error"
)

// Outcomes is the full closed dictionary of the matrix.
var Outcomes = []string{
	OutcomeValid,
	OutcomeMalformedUndeclared,
	OutcomeMalformedType,
	OutcomeForeignVersion,
	OutcomeRepairable,
	OutcomeUnrecoverable,
	OutcomeAdapterError,
}

// Adapter is the scripted model. The outcome word is fixed at
// construction: one adapter, one scenario of the matrix.
type Adapter struct {
	outcome string
}

// New builds the scripted adapter for one outcome word of the closed
// matrix; a word outside the dictionary is a named refusal — the
// configuration cannot drift from the matrix.
func New(outcome string) (*Adapter, error) {
	for _, candidate := range Outcomes {
		if candidate == outcome {
			return &Adapter{outcome: outcome}, nil
		}
	}
	return nil, fmt.Errorf("scripted-модель: исход %q вне закрытой матрицы исходов %v", outcome, Outcomes)
}

// MustNew builds the scripted adapter or panics — the test-contour
// constructor for the fixed scenarios.
func MustNew(outcome string) *Adapter {
	adapter, err := New(outcome)
	if err != nil {
		panic(err)
	}
	return adapter
}

// Round produces the scripted material of the configured outcome from
// the assembled round material. The repair cause INSIDE the material
// (non-empty on the machine's repair round) switches the repairable
// scenario to its valid material; the unrecoverable scenario changes
// its refusal shape but never becomes valid. A missing or unreadable
// material is a named honest error — the adapter cannot synthesize
// from nothing.
func (a *Adapter) Round(_ context.Context, request planner.RoundRequest) (planner.RoundResult, error) {
	material, err := decodeMaterial(request.Material)
	if err != nil {
		return planner.RoundResult{}, fmt.Errorf("scripted-модель: материал раунда отсутствует или нечитаем: %w", err)
	}
	switch a.outcome {
	case OutcomeAdapterError:
		return planner.RoundResult{}, errors.New("scripted-модель: раунд завершился ошибкой по конфигурации")
	case OutcomeValid:
		return planner.RoundResult{Raw: validMaterial(material)}, nil
	case OutcomeMalformedUndeclared:
		return planner.RoundResult{Raw: mutateMaterial(validMaterial(material), map[string]any{"необъявленный_ключ": "материал вне формы"})}, nil
	case OutcomeMalformedType:
		broken := materialMap(validMaterial(material))
		reply, _ := broken["reply"].(map[string]any)
		reply["text"] = 5
		return planner.RoundResult{Raw: mustJSON(broken)}, nil
	case OutcomeForeignVersion:
		foreign := materialMap(validMaterial(material))
		foreign["schema_version"] = planner.ProposalSchemaVersion + 98
		return planner.RoundResult{Raw: mustJSON(foreign)}, nil
	case OutcomeRepairable:
		if material.Repair == "" {
			return planner.RoundResult{Raw: mutateMaterial(validMaterial(material), map[string]any{"необъявленный_ключ": "материал вне формы"})}, nil
		}
		return planner.RoundResult{Raw: validMaterial(material)}, nil
	case OutcomeUnrecoverable:
		if material.Repair == "" {
			return planner.RoundResult{Raw: mutateMaterial(validMaterial(material), map[string]any{"необъявленный_ключ": "материал вне формы"})}, nil
		}
		foreign := materialMap(validMaterial(material))
		foreign["schema_version"] = planner.ProposalSchemaVersion + 98
		return planner.RoundResult{Raw: mustJSON(foreign)}, nil
	}
	return planner.RoundResult{}, fmt.Errorf("scripted-модель: неизвестный исход %q", a.outcome)
}

// decodeMaterial reads the assembled round material bytes into the
// typed platform form (the machine produced them — a plain decode; the
// typing rules of the wire belong to the machine, not here).
func decodeMaterial(raw []byte) (planner.RoundMaterial, error) {
	var material planner.RoundMaterial
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&material); err != nil {
		return planner.RoundMaterial{}, err
	}
	if material.AssemblyVersion == "" {
		return planner.RoundMaterial{}, errors.New("материал без слова версии сборки — это не материал раунда")
	}
	return material, nil
}

// validMaterial synthesizes the proposal material that passes the
// whole validation matrix of the given round material: the mode
// mirrors the MATERIAL mode (the assembly completeness — the ladder
// rule), the §8.1 disclosure rides the reply when the material
// demands it (the pin travels INTO the material and the model answers
// it), the reply stays inside the budget, the action is the none kind
// and the lists are empty. The synthesis derives from the MATERIAL —
// never from a canned answer and never from a context it never saw.
func validMaterial(material planner.RoundMaterial) []byte {
	reply := "Синтетическая реплика scripted-модели: материал собран из материала раунда."
	if material.AIDisclosureRequired {
		reply = "Я — искусственный интеллект. " + reply
	}
	proposal := map[string]any{
		"schema_version":  planner.ProposalSchemaVersion,
		"intent":          planner.IntentRespond,
		"mode":            material.Mode,
		"safety":          planner.SafetyClear,
		"fact_proposals":  []any{},
		"contradictions":  []any{},
		"read_requests":   []any{},
		"questions":       []any{},
		"proposed_action": map[string]any{"kind": planner.ActionKindNone},
		"reply":           map[string]any{"text": reply},
		"evidence_refs":   []any{},
		"memory_patch":    []any{},
	}
	return mustJSON(proposal)
}

// mutateMaterial re-marshals the material through a map with optional
// extra keys — the malformed shapes stay byte-honest JSON objects.
func mutateMaterial(raw []byte, extra map[string]any) []byte {
	material := materialMap(raw)
	for key, value := range extra {
		material[key] = value
	}
	return mustJSON(material)
}

// materialMap decodes the raw material into a mutable map.
func materialMap(raw []byte) map[string]any {
	material := map[string]any{}
	if err := jsonUnmarshal(raw, &material); err != nil {
		panic("scripted-модель: базовый материал обязан быть JSON: " + err.Error())
	}
	return material
}

// mustJSON marshals without the error path (the synthesized material
// is always marshalable).
func mustJSON(material map[string]any) []byte {
	raw, err := json.Marshal(material)
	if err != nil {
		panic("scripted-модель: материал обязан маршализоваться: " + err.Error())
	}
	return raw
}

// jsonUnmarshal decodes the raw material into a map (the numbers stay
// literal — the machine's strict decode owns the typing rules).
func jsonUnmarshal(raw []byte, destination *map[string]any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(destination)
}
