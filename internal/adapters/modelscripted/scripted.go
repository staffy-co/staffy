package modelscripted

// The scripted model adapter (Story 8-1): an explicitly fictional
// model over the planner ModelPort — the honest test material of
// PRODUCTION code, the verifier.Synthetic precedent of Story 1-8
// applied to the model port. The adapter is configured with ONE
// outcome word of the closed matrix; every Round produces exactly the
// material of that outcome, deterministically, from the given context:
//
//	valid               — a well-formed proposal that passes the whole
//	                      validation matrix (mode mirrors the context
//	                      mode, reply within the budget)
//	malformed_undeclared — a proposal carrying an undeclared key
//	malformed_type      — a proposal carrying a wrong field type
//	foreign_version     — a proposal carrying a foreign schema version
//	repairable          — round 1 malformed, round 2 valid (the repair
//	                      cause switches the material)
//	unrecoverable       — round 1 malformed, round 2 a foreign version
//	adapter_error       — an honest adapter error
//
// main.go NEVER wires this adapter (zero production callers — the
// caller of the planner arrives with Story 8.4). No credentials, no
// network, no SDK: the standard library only.

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

// Round produces the scripted material of the configured outcome. The
// repair cause (non-empty on the machine's repair round) switches the
// repairable scenario to its valid material; the unrecoverable
// scenario changes its refusal shape but never becomes valid.
func (a *Adapter) Round(_ context.Context, request planner.RoundRequest) (planner.RoundResult, error) {
	switch a.outcome {
	case OutcomeAdapterError:
		return planner.RoundResult{}, errors.New("scripted-модель: раунд завершился ошибкой по конфигурации")
	case OutcomeValid:
		return planner.RoundResult{Raw: validMaterial(request.Context)}, nil
	case OutcomeMalformedUndeclared:
		return planner.RoundResult{Raw: mutateMaterial(validMaterial(request.Context), map[string]any{"необъявленный_ключ": "материал вне формы"})}, nil
	case OutcomeMalformedType:
		material := materialMap(validMaterial(request.Context))
		reply, _ := material["reply"].(map[string]any)
		reply["text"] = 5
		return planner.RoundResult{Raw: mustJSON(material)}, nil
	case OutcomeForeignVersion:
		material := materialMap(validMaterial(request.Context))
		material["schema_version"] = planner.ProposalSchemaVersion + 98
		return planner.RoundResult{Raw: mustJSON(material)}, nil
	case OutcomeRepairable:
		if request.Repair == "" {
			return planner.RoundResult{Raw: mutateMaterial(validMaterial(request.Context), map[string]any{"необъявленный_ключ": "материал вне формы"})}, nil
		}
		return planner.RoundResult{Raw: validMaterial(request.Context)}, nil
	case OutcomeUnrecoverable:
		if request.Repair == "" {
			return planner.RoundResult{Raw: mutateMaterial(validMaterial(request.Context), map[string]any{"необъявленный_ключ": "материал вне формы"})}, nil
		}
		material := materialMap(validMaterial(request.Context))
		material["schema_version"] = planner.ProposalSchemaVersion + 98
		return planner.RoundResult{Raw: mustJSON(material)}, nil
	}
	return planner.RoundResult{}, fmt.Errorf("scripted-модель: неизвестный исход %q", a.outcome)
}

// validMaterial synthesizes the proposal material that passes the
// whole validation matrix of the given context: the mode mirrors the
// context mode (the ladder rule), the reply stays inside the budget,
// the action is the none kind and the lists are empty. The synthesis
// derives from the CONTEXT — the same way a live adapter would build
// from the material it was given, never from a canned answer.
func validMaterial(context planner.TurnContext) []byte {
	material := map[string]any{
		"schema_version":  planner.ProposalSchemaVersion,
		"intent":          planner.IntentRespond,
		"mode":            context.Mode,
		"safety":          planner.SafetyClear,
		"fact_proposals":  []any{},
		"contradictions":  []any{},
		"read_requests":   []any{},
		"questions":       []any{},
		"proposed_action": map[string]any{"kind": planner.ActionKindNone},
		"reply":           map[string]any{"text": "Синтетическая реплика scripted-модели: материал собран из контекста хода."},
		"evidence_refs":   []any{},
		"memory_patch":    []any{},
	}
	return mustJSON(material)
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
