package planner

// The turn budgets (Story 8-2): the closed platform forms of the
// bounded-model discipline — AD-7 «tool rounds, execution time and
// cost are bounded» as declarable data. A budget NARROWS the machine's
// structural limits, never widens them: the machine of Story 8-1 has
// at most two model rounds by construction, so a declared MaxRounds
// above two is a configuration refusal, not a licence. The enforcement
// is the bounded adapter (internal/adapters/modelbounded): every
// exhaustion is a named honest error, and the machine ends the turn
// failed with the approved safe fallback — unknown, without a blind
// retry, never a silent continuation.
//
// The cost units are platform-honest abstract units: live prices and
// tokens are the parameterised side of a future live adapter behind
// AG-4; the FORM of the budgets does not change when the live adapter
// arrives.

import (
	"encoding/json"
	"time"
)

// Budgets are the declared bounds of one planner turn: the round
// ceiling (within the structural two of the machine), the per-round
// deadline, the per-round cost units and the turn cost ceiling.
type Budgets struct {
	// MaxRounds is the maximum number of model rounds of the turn:
	// 1 or 2 (the structural ceiling of the machine — a declaration
	// above two is refused; a declaration of one honestly narrows the
	// repair round away).
	MaxRounds int `json:"max_rounds"`
	// RoundTimeout is the deadline of ONE model round. An expired
	// round is a named budget error: the turn fails with the approved
	// fallback — an honest timeout, never a silent wait.
	RoundTimeout time.Duration `json:"round_timeout"`
	// RoundCostUnits is the declared cost of one model round in
	// platform units.
	RoundCostUnits int64 `json:"round_cost_units"`
	// MaxCostUnits is the declared cost ceiling of the whole turn: a
	// round that would push the cumulative cost past the ceiling
	// never runs (the named cost error fires BEFORE the round).
	MaxCostUnits int64 `json:"max_cost_units"`
}

// Validate checks the closed budget rules with named refusals: the
// round range [1, 2], a positive round timeout, non-negative costs
// and a turn ceiling that declares at least one round.
func (b Budgets) Validate() *RuleError {
	if b.MaxRounds < 1 || b.MaxRounds > 2 {
		return rule("budget.max_rounds", "бюджет раундов %d вне структурной границы машины [1, 2] — декларация сужает предел, никогда не расширяет", b.MaxRounds)
	}
	if b.RoundTimeout <= 0 {
		return rule("budget.round_timeout", "таймаут раунда обязан быть положительным, получено %s", b.RoundTimeout.String())
	}
	if b.RoundCostUnits < 0 || b.MaxCostUnits < 0 {
		return rule("budget.cost", "стоимостные единицы не могут быть отрицательными: раунд %d, потолок %d", b.RoundCostUnits, b.MaxCostUnits)
	}
	if b.MaxCostUnits < b.RoundCostUnits {
		return rule("budget.cost_max", "потолок стоимости хода %d ниже стоимости одного раунда %d — ход минимум одного раунда обязан быть декларируем", b.MaxCostUnits, b.RoundCostUnits)
	}
	return nil
}

// compactBudgetsJSON marshals the declaration into the compact JSONB
// the turn record carries (deterministic — the same budgets always
// produce the same bytes).
func compactBudgetsJSON(budgets Budgets) ([]byte, error) {
	return json.Marshal(budgets)
}
