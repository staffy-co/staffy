package modelscripted

// The scripted model matrix of Story 8-1 step-02: every outcome word
// of the closed configuration dictionary produces exactly the material
// of its scenario — deterministically, from the given context, with the
// repair cause switching the repairable scenario and the configuration
// refusing a word outside the matrix. The adapter is production code
// with zero production callers (main.go never wires it).

import (
	"context"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/planner"
)

func fixtureContext() planner.TurnContext {
	return planner.TurnContext{Mode: planner.ModeSafety}
}

func TestOutcomeDictionaryIsClosed(t *testing.T) {
	if len(Outcomes) != 7 {
		t.Fatalf("матрица исходов обязана нести 7 слов, получено %d", len(Outcomes))
	}
	seen := map[string]bool{}
	for _, outcome := range Outcomes {
		if outcome == "" || seen[outcome] {
			t.Fatalf("матрица исходов несёт пустое или дублирующееся слово %q", outcome)
		}
		seen[outcome] = true
	}
}

func TestNewRefusesWordsOutsideTheMatrix(t *testing.T) {
	if _, err := New("сверхразум"); err == nil {
		t.Fatal("слово вне закрытой матрицы обязано давать поимённый отказ конфигурации")
	}
}

func TestEveryOutcomeProducesItsScenario(t *testing.T) {
	cases := []struct {
		outcome     string
		repair      string
		expectError bool
		probe       func(t *testing.T, raw []byte)
	}{
		{
			outcome: OutcomeValid,
			probe: func(t *testing.T, raw []byte) {
				if _, refusal := planner.DecodeTurnProposal(raw); refusal != nil {
					t.Fatalf("исход valid обязан давать декодируемое предложение: %v", refusal)
				}
			},
		},
		{
			outcome: OutcomeMalformedUndeclared,
			probe: func(t *testing.T, raw []byte) {
				_, refusal := planner.DecodeTurnProposal(raw)
				if refusal == nil || refusal.Rule != "proposal.strict_decode" {
					t.Fatalf("исход malformed_undeclared обязан давать отказ strict_decode, получено %v", refusal)
				}
			},
		},
		{
			outcome: OutcomeMalformedType,
			probe: func(t *testing.T, raw []byte) {
				_, refusal := planner.DecodeTurnProposal(raw)
				if refusal == nil || refusal.Rule != "proposal.strict_decode" {
					t.Fatalf("исход malformed_type обязан давать отказ strict_decode, получено %v", refusal)
				}
			},
		},
		{
			outcome: OutcomeForeignVersion,
			probe: func(t *testing.T, raw []byte) {
				_, refusal := planner.DecodeTurnProposal(raw)
				if refusal == nil || refusal.Rule != "proposal.schema_version" {
					t.Fatalf("исход foreign_version обязан давать отказ чужой версии, получено %v", refusal)
				}
			},
		},
		{
			outcome: OutcomeRepairable,
			repair:  "proposal.strict_decode: причина первого раунда",
			probe: func(t *testing.T, raw []byte) {
				if _, refusal := planner.DecodeTurnProposal(raw); refusal != nil {
					t.Fatalf("repairable со причиной обязан давать валидный материал, получено %v", refusal)
				}
			},
		},
		{
			outcome: OutcomeUnrecoverable,
			repair:  "proposal.strict_decode: причина",
			probe: func(t *testing.T, raw []byte) {
				_, refusal := planner.DecodeTurnProposal(raw)
				if refusal == nil || refusal.Rule != "proposal.schema_version" {
					t.Fatalf("unrecoverable на repair-раунде обязан менять форму отказа (чужая версия), получено %v", refusal)
				}
			},
		},
		{
			outcome:     OutcomeAdapterError,
			expectError: true,
			probe: func(t *testing.T, raw []byte) {
				if raw != nil {
					t.Fatal("ошибка адаптера не несет материала")
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.outcome, func(t *testing.T) {
			adapter, err := New(testCase.outcome)
			if err != nil {
				t.Fatalf("конфигурация: %v", err)
			}
			result, err := adapter.Round(context.Background(), planner.RoundRequest{Context: fixtureContext(), Repair: testCase.repair})
			if testCase.expectError {
				if err == nil {
					t.Fatal("исход adapter_error обязан возвращать честную ошибку раунда")
				}
				if !strings.Contains(err.Error(), "scripted-модель") {
					t.Fatalf("ошибка обязана быть поимённой ошибкой scripted-модели, получено %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("раунд: %v", err)
			}
			testCase.probe(t, result.Raw)
		})
	}
}

func TestRepairableSwitchesOnTheRepairCause(t *testing.T) {
	adapter := MustNew(OutcomeRepairable)
	// Round 1 (no cause): the malformed material.
	first, err := adapter.Round(context.Background(), planner.RoundRequest{Context: fixtureContext()})
	if err != nil {
		t.Fatalf("раунд 1: %v", err)
	}
	if _, refusal := planner.DecodeTurnProposal(first.Raw); refusal == nil {
		t.Fatal("раунд 1 без причины обязан быть малформед-материалом")
	}
	// The repair round (with the named cause): the valid material.
	second, err := adapter.Round(context.Background(), planner.RoundRequest{Context: fixtureContext(), Repair: "proposal.strict_decode: json: unknown field"})
	if err != nil {
		t.Fatalf("repair-раунд: %v", err)
	}
	if _, refusal := planner.DecodeTurnProposal(second.Raw); refusal != nil {
		t.Fatalf("repair-раунд обязан давать валидный материал, получено %v", refusal)
	}
}

func TestValidMaterialMirrorsTheContextMode(t *testing.T) {
	adapter := MustNew(OutcomeValid)
	for _, mode := range planner.ModeLadder {
		result, err := adapter.Round(context.Background(), planner.RoundRequest{Context: planner.TurnContext{Mode: mode}})
		if err != nil {
			t.Fatalf("режим %s: %v", mode, err)
		}
		proposal, refusal := planner.DecodeTurnProposal(result.Raw)
		if refusal != nil {
			t.Fatalf("режим %s: %v", mode, refusal)
		}
		if proposal.Mode != mode {
			t.Fatalf("синтез обязан зеркалить режим контекста %s, получен %q — иначе лестница не прошла бы", mode, proposal.Mode)
		}
	}
}
