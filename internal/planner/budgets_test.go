package planner_test

// The budget matrix of Story 8-2 step-01: the closed budget rules as
// named refusals — the round range [1, 2] (a declaration never widens
// the machine's structural ceiling), a positive round timeout,
// non-negative costs and a turn ceiling that declares at least one
// round. The marshaled form is compact and deterministic (the journal
// carries it as JSONB).

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/planner"
)

func TestBudgetsValidationMatrix(t *testing.T) {
	valid := planner.Budgets{MaxRounds: 2, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 2}
	if refusal := valid.Validate(); refusal != nil {
		t.Fatalf("валидный бюджет обязан проходить, получен отказ: %v", refusal)
	}
	narrow := planner.Budgets{MaxRounds: 1, RoundTimeout: 500 * time.Millisecond, RoundCostUnits: 0, MaxCostUnits: 0}
	if refusal := narrow.Validate(); refusal != nil {
		t.Fatalf("суженный бюджет (1 раунд, нулевая стоимость) обязан проходить: %v", refusal)
	}

	cases := []struct {
		name    string
		budgets planner.Budgets
		rule    string
	}{
		{"нулевой бюджет раундов", planner.Budgets{MaxRounds: 0, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 2}, "budget.max_rounds"},
		{"отрицательный бюджет раундов", planner.Budgets{MaxRounds: -1, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 2}, "budget.max_rounds"},
		{"декларация расширяет структурный предел", planner.Budgets{MaxRounds: 3, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 4}, "budget.max_rounds"},
		{"нулевой таймаут раунда", planner.Budgets{MaxRounds: 2, RoundTimeout: 0, RoundCostUnits: 1, MaxCostUnits: 2}, "budget.round_timeout"},
		{"отрицательный таймаут раунда", planner.Budgets{MaxRounds: 2, RoundTimeout: -time.Second, RoundCostUnits: 1, MaxCostUnits: 2}, "budget.round_timeout"},
		{"отрицательная стоимость раунда", planner.Budgets{MaxRounds: 2, RoundTimeout: time.Second, RoundCostUnits: -1, MaxCostUnits: 2}, "budget.cost"},
		{"отрицательный потолок стоимости", planner.Budgets{MaxRounds: 2, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: -1}, "budget.cost"},
		{"потолок ниже стоимости одного раунда", planner.Budgets{MaxRounds: 2, RoundTimeout: time.Second, RoundCostUnits: 5, MaxCostUnits: 4}, "budget.cost_max"},
	}
	for _, testCase := range cases {
		refusal := testCase.budgets.Validate()
		if refusal == nil {
			t.Fatalf("%s: обязан быть поимённый отказ", testCase.name)
		}
		if refusal.Rule != testCase.rule {
			t.Fatalf("%s: правило отказа %q, ожидается %q (%s)", testCase.name, refusal.Rule, testCase.rule, refusal.Error())
		}
	}
}

func TestBudgetsJSONIsCompactAndDeterministic(t *testing.T) {
	budgets := planner.Budgets{MaxRounds: 2, RoundTimeout: 1500 * time.Millisecond, RoundCostUnits: 3, MaxCostUnits: 6}
	first, err := json.Marshal(budgets)
	if err != nil {
		t.Fatalf("маршализация бюджетов: %v", err)
	}
	second, err := json.Marshal(budgets)
	if err != nil {
		t.Fatalf("повторная маршализация: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("JSON бюджетов обязан быть детерминированным: %s ≠ %s", first, second)
	}
	for _, key := range []string{`"max_rounds"`, `"round_timeout"`, `"round_cost_units"`, `"max_cost_units"`} {
		if !strings.Contains(string(first), key) {
			t.Fatalf("JSON бюджетов обязан нести ключ %s, получено %s", key, first)
		}
	}
	var decoded planner.Budgets
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("JSON бюджетов обязан декодироваться: %v", err)
	}
	if decoded != budgets {
		t.Fatalf("декодированные бюджеты обязаны совпадать: %+v ≠ %+v", decoded, budgets)
	}
}
