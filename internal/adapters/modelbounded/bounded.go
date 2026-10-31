package modelbounded

// The bounded model adapter (Story 8-2): the wrapper over ANY
// planner.ModelPort that enforces the declared turn budgets — the
// AD-7 «execution time and cost are bounded» as executable code.
// Every exhaustion is a NAMED honest error, and the machine ends the
// turn failed with the approved safe fallback: unknown, without a
// blind retry, never a silent continuation. The wrapper is
// constructed PER TURN by the composer (the tests today, the
// production caller of Story 8-4 later) — no hidden global state
// exists, and a future live adapter behind AG-4 rides the same
// wrapper without a machine change.
//
// The honest error taxonomy (the journal carries each cause by name
// through the machine's adapter-error path):
//
//      ErrBudgetRounds  — the round call count passed the declared ceiling
//      ErrBudgetTimeout — the round deadline expired
//      ErrBudgetCost    — a round that would push the cumulative cost past
//                         the turn ceiling never runs
//
// No credentials, no network, no SDK: the standard library only. The
// adapter never sees the Turn Context (the port carries the assembled
// material — AD-8 by construction).

import (
	"context"
	"errors"
	"fmt"

	"github.com/staffy-co/staffy/internal/planner"
)

// The named budget errors: honest, greppable causes.
var (
	// ErrBudgetRounds is the honest round ceiling: the call count
	// passed the declared limit of the turn.
	ErrBudgetRounds = errors.New("бюджет раундов исчерпан — декларированный предел хода достигнут, слепого повтора нет")
	// ErrBudgetTimeout is the honest round deadline expiry: the turn
	// ends unknown with the approved fallback, never a silent wait.
	ErrBudgetTimeout = errors.New("таймаут раунда бюджета — время исполнения раунда исчерпано, слепого повтора нет")
	// ErrBudgetCost is the honest cost ceiling: a round that would
	// exceed the declared turn cost never runs.
	ErrBudgetCost = errors.New("стоимостной бюджет хода исчерпан — раунд сверх декларированного потолка не запускается")
)

// Bounded wraps one model port with the declared budgets of one turn.
type Bounded struct {
	inner   planner.ModelPort
	budgets planner.Budgets
	rounds  int
	cost    int64
}

// New wraps the inner port with the declared budgets. An invalid
// declaration is a named construction refusal — the honest
// configuration failure, never a runtime surprise.
func New(inner planner.ModelPort, budgets planner.Budgets) (*Bounded, error) {
	if refusal := budgets.Validate(); refusal != nil {
		return nil, fmt.Errorf("modelbounded: %w", refusal)
	}
	return &Bounded{inner: inner, budgets: budgets}, nil
}

// MustNew wraps or panics — the test-contour constructor for the fixed
// compositions.
func MustNew(inner planner.ModelPort, budgets planner.Budgets) *Bounded {
	bounded, err := New(inner, budgets)
	if err != nil {
		panic(err)
	}
	return bounded
}

// Round executes one model round inside the declared budgets: the
// round guard first (the call count never passes the ceiling), then
// the cost guard (a round that would push the cumulative cost past
// the turn ceiling never starts — it bills nothing it never runs),
// then the deadline. A started round bills its declared cost even
// when it times out: the honest accounting counts every round that
// was actually attempted.
func (b *Bounded) Round(ctx context.Context, request planner.RoundRequest) (planner.RoundResult, error) {
	b.rounds++
	if b.rounds > b.budgets.MaxRounds {
		return planner.RoundResult{}, fmt.Errorf("%w: раунд %d сверх декларированных %d", ErrBudgetRounds, b.rounds, b.budgets.MaxRounds)
	}
	if b.cost+b.budgets.RoundCostUnits > b.budgets.MaxCostUnits {
		return planner.RoundResult{}, fmt.Errorf("%w: кумулятив %d + раунд %d сверх потолка %d", ErrBudgetCost, b.cost, b.budgets.RoundCostUnits, b.budgets.MaxCostUnits)
	}
	deadline, cancel := context.WithTimeout(ctx, b.budgets.RoundTimeout)
	defer cancel()
	b.cost += b.budgets.RoundCostUnits
	result, err := b.inner.Round(deadline, request)
	if errors.Is(err, context.DeadlineExceeded) {
		return planner.RoundResult{}, fmt.Errorf("%w: предел %s исчерпан", ErrBudgetTimeout, b.budgets.RoundTimeout.String())
	}
	return result, err
}
