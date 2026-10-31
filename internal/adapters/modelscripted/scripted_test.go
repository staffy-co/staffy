package modelscripted

// The scripted model matrix of Story 8-1 step-02 (the material
// synthesis of Story 8-2): every outcome word of the closed
// configuration dictionary produces exactly the material of its
// scenario — deterministically, from the assembled round material,
// with the repair cause INSIDE the material switching the repairable
// scenario and the configuration refusing a word outside the matrix.
// The adapter is production code with zero production callers (main.go
// never wires it).

import (
	"context"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/planner"
)

// fixtureMaterial assembles the round material of a turn context with
// the given mode, disclosure pin and repair cause — the honest input
// of the adapter since Story 8-2 (the material, never the context).
func fixtureMaterial(mode string, disclosure bool, repair string) planner.RoundRequest {
	material := planner.AssembleRoundMaterial(planner.TurnContext{Mode: mode, AIDisclosureRequired: disclosure}, repair)
	raw, err := material.Bytes()
	if err != nil {
		panic("scripted-тест: материал обязан маршализоваться: " + err.Error())
	}
	return planner.RoundRequest{Material: raw}
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
			request := fixtureMaterial(planner.ModeSafety, true, testCase.repair)
			result, err := adapter.Round(context.Background(), request)
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
	first, err := adapter.Round(context.Background(), fixtureMaterial(planner.ModeSafety, true, ""))
	if err != nil {
		t.Fatalf("раунд 1: %v", err)
	}
	if _, refusal := planner.DecodeTurnProposal(first.Raw); refusal == nil {
		t.Fatal("раунд 1 без причины обязан быть малформед-материалом")
	}
	// The repair round (the named cause INSIDE the material): the valid
	// material.
	second, err := adapter.Round(context.Background(), fixtureMaterial(planner.ModeSafety, true, "proposal.strict_decode: json: unknown field"))
	if err != nil {
		t.Fatalf("repair-раунд: %v", err)
	}
	if _, refusal := planner.DecodeTurnProposal(second.Raw); refusal != nil {
		t.Fatalf("repair-раунд обязан давать валидный материал, получено %v", refusal)
	}
}

func TestValidMaterialMirrorsTheMaterialMode(t *testing.T) {
	adapter := MustNew(OutcomeValid)
	for _, mode := range planner.ModeLadder {
		result, err := adapter.Round(context.Background(), fixtureMaterial(mode, false, ""))
		if err != nil {
			t.Fatalf("режим %s: %v", mode, err)
		}
		proposal, refusal := planner.DecodeTurnProposal(result.Raw)
		if refusal != nil {
			t.Fatalf("режим %s: %v", mode, refusal)
		}
		if proposal.Mode != mode {
			t.Fatalf("синтез обязан зеркалить режим МАТЕРИАЛА %s, получен %q — иначе лестница не прошла бы", mode, proposal.Mode)
		}
	}
}

func TestValidMaterialDisclosesWhenThePinArrives(t *testing.T) {
	adapter := MustNew(OutcomeValid)
	// The §8.1 pin travels INTO the material and the scripted model
	// answers it — the contract-side proof that the disclosure
	// requirement reaches the model.
	disclosed, err := adapter.Round(context.Background(), fixtureMaterial(planner.ModeSafety, true, ""))
	if err != nil {
		t.Fatalf("раунд с пином: %v", err)
	}
	proposal, refusal := planner.DecodeTurnProposal(disclosed.Raw)
	if refusal != nil {
		t.Fatalf("раунд с пином: %v", refusal)
	}
	if !strings.Contains(proposal.Reply.Text, "Я — искусственный интеллект") {
		t.Fatalf("реплика обязана нести раскрытие ИИ, когда материал требует его, получено %q", proposal.Reply.Text)
	}
	// Without the pin the reply carries no disclosure — the pin is the
	// only switch, never a canned behaviour.
	plain, err := adapter.Round(context.Background(), fixtureMaterial(planner.ModeSafety, false, ""))
	if err != nil {
		t.Fatalf("раунд без пина: %v", err)
	}
	proposal, refusal = planner.DecodeTurnProposal(plain.Raw)
	if refusal != nil {
		t.Fatalf("раунд без пина: %v", refusal)
	}
	if strings.Contains(proposal.Reply.Text, "Я — искусственный интеллект") {
		t.Fatalf("реплика без пина не обязана нести раскрытия, получено %q", proposal.Reply.Text)
	}
}

func TestRoundRefusesMissingOrUnreadableMaterial(t *testing.T) {
	adapter := MustNew(OutcomeValid)
	if _, err := adapter.Round(context.Background(), planner.RoundRequest{}); err == nil {
		t.Fatal("отсутствующий материал обязан давать поимённую ошибку — синтез из ничего невозможен")
	} else if !strings.Contains(err.Error(), "материал раунда отсутствует или нечитаем") {
		t.Fatalf("ошибка обязана быть поимённой, получено %v", err)
	}
	if _, err := adapter.Round(context.Background(), planner.RoundRequest{Material: []byte("не-json")}); err == nil {
		t.Fatal("нечитаемый материал обязан давать поимённую ошибку")
	}
	// The word of the assembly version is the material's identity: a
	// random JSON object without it is not round material.
	if _, err := adapter.Round(context.Background(), planner.RoundRequest{Material: []byte("{}")}); err == nil {
		t.Fatal("JSON без слова версии сборки — не материал раунда")
	}
}
