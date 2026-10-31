package modelbounded

// The bounded adapter matrix of Story 8-2 step-02: every budget
// exhaustion is a named honest error reproduced by a LIVE run — the
// round guard (a narrowed MaxRounds refuses the second round), the
// cost guard (a round that would pass the ceiling never runs) and the
// round deadline (a slow inner port expires into the timeout error).
// The wrapper refuses an invalid declaration at construction, and the
// honest pass-through of the inner errors stays intact (an adapter
// error is an adapter error — never a budget word).

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/planner"
)

// immediatePort answers every round at once.
type immediatePort struct {
	rounds int
}

func (p *immediatePort) Round(_ context.Context, request planner.RoundRequest) (planner.RoundResult, error) {
	p.rounds++
	return planner.RoundResult{Raw: request.Material}, nil
}

// slowPort waits for the deadline and reports the context expiry —
// the honest slow model.
type slowPort struct{}

func (p slowPort) Round(ctx context.Context, _ planner.RoundRequest) (planner.RoundResult, error) {
	<-ctx.Done()
	return planner.RoundResult{}, ctx.Err()
}

// brokenPort fails every round with its own named error.
type brokenPort struct{}

func (p brokenPort) Round(_ context.Context, _ planner.RoundRequest) (planner.RoundResult, error) {
	return planner.RoundResult{}, errBrokenPort
}

var errBrokenPort = errors.New("внутренний порт сломан")

// materialRequest builds one honest round request.
func materialRequest() planner.RoundRequest {
	material := planner.AssembleRoundMaterial(planner.TurnContext{Mode: planner.ModeSafety}, "")
	raw, err := material.Bytes()
	if err != nil {
		panic("modelbounded-тест: материал обязан маршализоваться: " + err.Error())
	}
	return planner.RoundRequest{Material: raw}
}

func TestNewRefusesInvalidDeclarations(t *testing.T) {
	invalid := planner.Budgets{MaxRounds: 3, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 4}
	if _, err := New(&immediatePort{}, invalid); err == nil {
		t.Fatal("невалидная декларация обязана давать поимённый отказ конструкции")
	} else if !strings.Contains(err.Error(), "budget.max_rounds") {
		t.Fatalf("отказ конструкции обязан нести поимённую причину, получено %v", err)
	}
	valid := planner.Budgets{MaxRounds: 2, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 2}
	if _, err := New(&immediatePort{}, valid); err != nil {
		t.Fatalf("валидная декларация обязана проходить: %v", err)
	}
}

func TestRoundGuardRefusesTheRoundPastTheCeiling(t *testing.T) {
	// A narrowed MaxRounds of one: the second call is the named budget
	// error — the declaration narrows the structural two honestly.
	budgets := planner.Budgets{MaxRounds: 1, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 4}
	bounded := MustNew(&immediatePort{}, budgets)
	request := materialRequest()
	if _, err := bounded.Round(context.Background(), request); err != nil {
		t.Fatalf("раунд 1: %v", err)
	}
	_, err := bounded.Round(context.Background(), request)
	if !errors.Is(err, ErrBudgetRounds) {
		t.Fatalf("раунд 2 обязан быть поимённой ошибкой бюджета раундов, получено %v", err)
	}
}

func TestCostGuardRefusesTheRoundPastTheCeiling(t *testing.T) {
	// The ceiling of one unit with one-unit rounds: the first round
	// runs and bills, the second would push the cumulative cost past
	// the ceiling — it never runs.
	budgets := planner.Budgets{MaxRounds: 2, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 1}
	bounded := MustNew(&immediatePort{}, budgets)
	request := materialRequest()
	if _, err := bounded.Round(context.Background(), request); err != nil {
		t.Fatalf("раунд 1: %v", err)
	}
	_, err := bounded.Round(context.Background(), request)
	if !errors.Is(err, ErrBudgetCost) {
		t.Fatalf("раунд 2 обязан быть поимённой ошибкой стоимостного бюджета, получено %v", err)
	}
}

func TestRoundDeadlineExpiresIntoTheTimeoutError(t *testing.T) {
	// A slow inner port under a short deadline: the expiry is the
	// named budget timeout — the honest unknown, never a silent wait.
	budgets := planner.Budgets{MaxRounds: 2, RoundTimeout: 40 * time.Millisecond, RoundCostUnits: 1, MaxCostUnits: 4}
	bounded := MustNew(slowPort{}, budgets)
	started := time.Now()
	_, err := bounded.Round(context.Background(), materialRequest())
	elapsed := time.Since(started)
	if !errors.Is(err, ErrBudgetTimeout) {
		t.Fatalf("медленный раунд обязан истекать в поимённую ошибку таймаута бюджета, получено %v", err)
	}
	if elapsed < 30*time.Millisecond {
		t.Fatalf("дедлайн обязан реально ждать предел, истекло %s", elapsed)
	}
}

func TestInnerErrorsPassThroughUntouched(t *testing.T) {
	// An inner error is an inner error: the wrapper never masks it as
	// a budget word and never retries.
	budgets := planner.Budgets{MaxRounds: 2, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 4}
	bounded := MustNew(brokenPort{}, budgets)
	_, err := bounded.Round(context.Background(), materialRequest())
	if !errors.Is(err, errBrokenPort) {
		t.Fatalf("внутренняя ошибка обязана доезжать как есть, получено %v", err)
	}
	if errors.Is(err, ErrBudgetRounds) || errors.Is(err, ErrBudgetTimeout) || errors.Is(err, ErrBudgetCost) {
		t.Fatalf("внутренняя ошибка не обязана маскироваться словом бюджета, получено %v", err)
	}
}

func TestBoundedRidesAnyInnerPortUnchanged(t *testing.T) {
	// The composition proof: the wrapper over the scripted model of
	// the matrix is the machine's future caller shape — the material
	// passes through untouched (the adapter sees exactly the assembled
	// bytes).
	budgets := planner.Budgets{MaxRounds: 2, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 2}
	bounded := MustNew(&immediatePort{}, budgets)
	request := materialRequest()
	result, err := bounded.Round(context.Background(), request)
	if err != nil {
		t.Fatalf("раунд: %v", err)
	}
	if string(result.Raw) != string(request.Material) {
		t.Fatalf("материал обязан доезжать до внутреннего порта как есть")
	}
}
