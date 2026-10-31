package planner_test

// The machine matrix of Story 8-1 step-02: the protocol machine over
// the ModelPort with the scripted model of the matrix and an in-memory
// journal — valid, repaired, failed + approved fallback, adapter
// error, the round ceiling of two, the journal uniqueness and the
// context refusal that never starts a turn. The AD-7 discipline is the
// assertion set: a failed turn never carries model material out.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/adapters/modelbounded"
	"github.com/staffy-co/staffy/internal/adapters/modelscripted"
	"github.com/staffy-co/staffy/internal/planner"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// memoryLedger is the in-memory journal of the unit matrix: the honest
// test material of the recorder port.
type memoryLedger struct {
	records []planner.TurnRecord
	failAt  int
}

func (l *memoryLedger) Record(_ context.Context, _ tenancy.Context, record planner.TurnRecord) error {
	for _, existing := range l.records {
		if existing.TenantID == record.TenantID && existing.TurnID == record.TurnID {
			return planner.ErrTurnConflict
		}
	}
	if l.failAt == len(l.records) {
		return errJournalBroken
	}
	l.records = append(l.records, record)
	return nil
}

var errJournalBroken = errBroken{}

type errBroken struct{}

func (errBroken) Error() string { return "журнал сломан" }

type fixedTurnClock struct{ now time.Time }

func (c fixedTurnClock) Now() time.Time { return c.now }

// machineFixture wires the machine over the scripted outcome and the
// memory ledger with the honest default budgets of the matrix (the
// structural two rounds, a generous timeout — the budget NARROWING
// has its own tests).
func machineFixture(outcome string) (*planner.Machine, *memoryLedger) {
	ledger := &memoryLedger{failAt: -1}
	machine, err := planner.NewMachine(modelscripted.MustNew(outcome), ledger, fixedTurnClock{now: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)}, planner.Budgets{MaxRounds: 2, RoundTimeout: 5 * time.Second, RoundCostUnits: 1, MaxCostUnits: 4})
	if err != nil {
		panic("фикстура машины: " + err.Error())
	}
	return machine, ledger
}

// runScope builds the tenant scope of the material context.
func runScope(t *testing.T) tenancy.Context {
	t.Helper()
	scope, err := tenancy.New(
		id.MustParse("01994f64-6c00-7000-8000-000000008107"),
		id.MustParse("01994f64-6c00-7000-8000-000000008102"),
	)
	if err != nil {
		t.Fatalf("скоуп: %v", err)
	}
	return scope
}

// rawContextBytes marshals the valid context material to the bytes the
// machine consumes.
func rawContextBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := json.Marshal(validContextMaterial())
	if err != nil {
		t.Fatalf("маршализация контекста: %v", err)
	}
	return raw
}

func TestMachineValidTurnIsJournaledValidated(t *testing.T) {
	machine, ledger := machineFixture(modelscripted.OutcomeValid)
	result, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t))
	if err != nil {
		t.Fatalf("валидный ход: %v", err)
	}
	if !result.Started || result.Outcome != planner.OutcomeValidated || result.Rounds != 1 {
		t.Fatalf("исход обязан быть validated за 1 раунд, получено started=%v outcome=%q rounds=%d", result.Started, result.Outcome, result.Rounds)
	}
	if result.Refusal != nil {
		t.Fatalf("валидный ход не несёт отказа: %v", result.Refusal)
	}
	if result.Proposal == nil {
		t.Fatal("валидный ход обязан нести валидированное предложение")
	}
	if result.Fallback != "" {
		t.Fatalf("валидный ход не несёт fallback, получено %q", result.Fallback)
	}
	if len(ledger.records) != 1 {
		t.Fatalf("журнал обязан нести ровно одну запись, получено %d", len(ledger.records))
	}
	record := ledger.records[0]
	if record.Outcome != planner.OutcomeValidated || record.RefusalReason != "" || record.FallbackForm != "" {
		t.Fatalf("запись валидного хода обязана быть чистой: %+v", record)
	}
	if record.Build != planner.Build || record.SchemaVersion != planner.ProposalSchemaVersion {
		t.Fatalf("запись обязана нести Build и версию формы: %q %d", record.Build, record.SchemaVersion)
	}
	if record.AssemblyVersion != planner.AssemblyVersion {
		t.Fatalf("запись обязана нести версию сборки входа (полная Provenance хода 8-2): %q", record.AssemblyVersion)
	}
	if len(record.Budgets) == 0 || !strings.Contains(string(record.Budgets), `"max_rounds":2`) {
		t.Fatalf("запись обязана нести декларацию бюджетов хода, получено %s", record.Budgets)
	}
	if len(record.Proposal) == 0 {
		t.Fatal("запись валидного хода обязана нести компактное JSONB-предложение")
	}
	if !strings.Contains(string(record.Proposal), `"intent":"respond"`) {
		t.Fatalf("JSONB-предложение обязано быть компактным материалом формы, получено %s", record.Proposal)
	}
}

func TestMachineRepairableTurnIsJournaledRepairedInExactlyTwoRounds(t *testing.T) {
	machine, ledger := machineFixture(modelscripted.OutcomeRepairable)
	result, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t))
	if err != nil {
		t.Fatalf("repairable-ход: %v", err)
	}
	if result.Outcome != planner.OutcomeRepaired || result.Rounds != 2 {
		t.Fatalf("исход обязан быть repaired ровно за 2 раунда, получено outcome=%q rounds=%d", result.Outcome, result.Rounds)
	}
	if result.Proposal == nil {
		t.Fatal("repaired-ход обязан нести валидированное предложение второго раунда")
	}
	if result.Refusal == nil || result.Refusal.Rule != "proposal.strict_decode" {
		t.Fatalf("repaired-ход обязан нести причину первого раунда, получено %v", result.Refusal)
	}
	if len(ledger.records) != 1 {
		t.Fatalf("журнал обязан нести ровно ОДНУ запись (second записи нет), получено %d", len(ledger.records))
	}
	if ledger.records[0].Outcome != planner.OutcomeRepaired {
		t.Fatalf("запись обязана быть repaired, получено %q", ledger.records[0].Outcome)
	}
	if ledger.records[0].RefusalReason == "" {
		t.Fatal("запись repaired-хода обязана нести честную причину первого отказа")
	}
}

func TestMachineUnrecoverableTurnFailsWithApprovedFallbackNeverModelOutput(t *testing.T) {
	machine, ledger := machineFixture(modelscripted.OutcomeUnrecoverable)
	result, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t))
	if err != nil {
		t.Fatalf("unrecoverable-ход: %v", err)
	}
	if result.Outcome != planner.OutcomeFailed || result.Rounds != 2 {
		t.Fatalf("исход обязан быть failed за 2 раунда, получено outcome=%q rounds=%d", result.Outcome, result.Rounds)
	}
	if result.Proposal != nil {
		t.Fatal("проваленный ход НИКОГДА не несёт материала модели наружу (AD-7)")
	}
	if result.Fallback != planner.FallbackHonestStop {
		t.Fatalf("ход обязан завершить утверждённый fallback honest_stop (политика контекста), получено %q", result.Fallback)
	}
	if result.Refusal == nil || result.Refusal.Rule != "proposal.schema_version" {
		t.Fatalf("проваленный ход обязан нести причину второго раунда (чужая версия), получено %v", result.Refusal)
	}
	if len(ledger.records) != 1 || ledger.records[0].Outcome != planner.OutcomeFailed {
		t.Fatalf("журнал обязан нести одну failed-запись, получено %d", len(ledger.records))
	}
	record := ledger.records[0]
	if record.FallbackForm != planner.FallbackHonestStop || len(record.Proposal) != 0 {
		t.Fatalf("failed-запись обязана нести fallback-форму и НЕ нести предложения: %+v", record)
	}
	if !strings.Contains(record.RefusalReason, "proposal.schema_version") {
		t.Fatalf("failed-запись обязана нести поимённую причину, получено %q", record.RefusalReason)
	}
}

func TestMachineAdapterErrorFailsWithoutBlindRetry(t *testing.T) {
	machine, ledger := machineFixture(modelscripted.OutcomeAdapterError)
	result, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t))
	if err != nil {
		t.Fatalf("ошибка адаптера — это ход, а не ошибка вызова: %v", err)
	}
	if result.Outcome != planner.OutcomeFailed || result.Rounds != 1 {
		t.Fatalf("ошибка адаптера = failed за 1 раунд БЕЗ слепого retry и БЕЗ repair, получено outcome=%q rounds=%d", result.Outcome, result.Rounds)
	}
	if result.Proposal != nil {
		t.Fatal("ошибка адаптера не несёт материала модели")
	}
	if result.Fallback != planner.FallbackHonestStop {
		t.Fatalf("fallback обязана закрыть ход, получено %q", result.Fallback)
	}
	if result.Refusal == nil || result.Refusal.Rule != "planner.adapter_round" {
		t.Fatalf("причина обязана быть поимённой ошибкой адаптера, получено %v", result.Refusal)
	}
	if len(ledger.records) != 1 || ledger.records[0].Outcome != planner.OutcomeFailed {
		t.Fatalf("журнал обязан нести одну failed-запись, получено %d", len(ledger.records))
	}
}

func TestMachineRoundCeilingIsStructurallyTwo(t *testing.T) {
	// The malformed outcomes that repeat on both rounds still end in
	// exactly two model rounds — the machine has no third round to
	// call, and every repeat-refusal is journaled failed.
	for _, outcome := range []string{
		modelscripted.OutcomeMalformedUndeclared,
		modelscripted.OutcomeMalformedType,
		modelscripted.OutcomeForeignVersion,
	} {
		machine, ledger := machineFixture(outcome)
		result, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t))
		if err != nil {
			t.Fatalf("исход %s: %v", outcome, err)
		}
		if result.Outcome != planner.OutcomeFailed || result.Rounds != 2 {
			t.Fatalf("исход %s обязан быть failed ровно за 2 раунда, получено outcome=%q rounds=%d", outcome, result.Outcome, result.Rounds)
		}
		if result.Proposal != nil || result.Fallback == "" {
			t.Fatalf("исход %s: провал без материала и с fallback, получено %+v", outcome, result)
		}
		if len(ledger.records) != 1 {
			t.Fatalf("исход %s: одна запись журнала, получено %d", outcome, len(ledger.records))
		}
	}
}

func TestMachineContextRefusalNeverStartsATurn(t *testing.T) {
	machine, ledger := machineFixture(modelscripted.OutcomeValid)
	material := validContextMaterial()
	material["mode"] = "не-слово-словаря"
	raw, err := json.Marshal(material)
	if err != nil {
		t.Fatalf("маршализация: %v", err)
	}
	result, err := machine.Run(context.Background(), runScope(t), raw)
	if err != nil {
		t.Fatalf("отказ контекста — это не ошибка вызова: %v", err)
	}
	if result.Started || result.Outcome != "" || result.Rounds != 0 {
		t.Fatalf("отказанный контекст не начинает ход, получено %+v", result)
	}
	if result.Refusal == nil || result.Refusal.Rule != "context.mode" {
		t.Fatalf("отказ обязан быть поимённым правилом контекста, получено %v", result.Refusal)
	}
	if len(ledger.records) != 0 {
		t.Fatalf("отказ входа журналом НЕ является — ход не начинался, получено %d записей", len(ledger.records))
	}
}

func TestMachineTenantScopeGuardRefusesForeignTenantContext(t *testing.T) {
	machine, ledger := machineFixture(modelscripted.OutcomeValid)
	material := validContextMaterial()
	material["tenant_id"] = "01994f64-6c00-7000-8000-000000009999"
	raw, err := json.Marshal(material)
	if err != nil {
		t.Fatalf("маршализация: %v", err)
	}
	result, err := machine.Run(context.Background(), runScope(t), raw)
	if err != nil {
		t.Fatalf("скоуп-отказ: %v", err)
	}
	if result.Started || result.Rounds != 0 {
		t.Fatalf("чужой тенент контекста не начинает ход, получено %+v", result)
	}
	if result.Refusal == nil || result.Refusal.Rule != "context.tenant_scope" {
		t.Fatalf("отказ обязан быть правилом context.tenant_scope, получено %v", result.Refusal)
	}
	if len(ledger.records) != 0 {
		t.Fatalf("ход не начинался — записей быть не может, получено %d", len(ledger.records))
	}
}

func TestMachineTurnUniquenessSurfacesTheHonestConflict(t *testing.T) {
	machine, _ := machineFixture(modelscripted.OutcomeValid)
	raw := rawContextBytes(t)
	if _, err := machine.Run(context.Background(), runScope(t), raw); err != nil {
		t.Fatalf("первый ход: %v", err)
	}
	_, err := machine.Run(context.Background(), runScope(t), raw)
	if err == nil {
		t.Fatal("повторная команда того же хода обязана дать честный конфликт уникальности")
	}
	if !planner.IsTurnConflict(err) {
		t.Fatalf("конфликт обязан быть ErrTurnConflict, получено %v", err)
	}
}

func TestMachineLedgerFailureSurfacesHonestly(t *testing.T) {
	machine, ledger := machineFixture(modelscripted.OutcomeValid)
	ledger.failAt = 0
	result, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t))
	if err == nil || !strings.Contains(err.Error(), "журнал сломан") {
		t.Fatalf("поломка журнала обязана всплывать честно, получено %v", err)
	}
	if !result.Started || result.Outcome != planner.OutcomeValidated {
		t.Fatalf("ход состоялся и результат честен, получено %+v", result)
	}
	if len(ledger.records) != 0 {
		t.Fatalf("записи быть не должно, получено %d", len(ledger.records))
	}
}

func TestMachineFallbackFollowsTheContextPolicy(t *testing.T) {
	for _, policy := range planner.FallbackDictionary {
		material := validContextMaterial()
		material["fallback_policy"] = policy
		raw, err := json.Marshal(material)
		if err != nil {
			t.Fatalf("маршализация: %v", err)
		}
		machine, ledger := machineFixture(modelscripted.OutcomeUnrecoverable)
		result, err := machine.Run(context.Background(), runScope(t), raw)
		if err != nil {
			t.Fatalf("политика %s: %v", policy, err)
		}
		if result.Outcome != planner.OutcomeFailed || result.Fallback != policy {
			t.Fatalf("политика %s обязана закрыть провал своей формой, получено outcome=%q fallback=%q", policy, result.Outcome, result.Fallback)
		}
		if ledger.records[0].FallbackForm != policy {
			t.Fatalf("запись обязана нести fallback-форму %s поимённо, получено %q", policy, ledger.records[0].FallbackForm)
		}
	}
}

func TestMachineRepairCauseCarriesTheNamedReason(t *testing.T) {
	// The repair round receives the NAMED cause of the first refusal —
	// the scripted adapter switches on it, and the machine hands it
	// verbatim (rule: detail).
	machine, ledger := machineFixture(modelscripted.OutcomeRepairable)
	if _, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t)); err != nil {
		t.Fatalf("repairable-ход: %v", err)
	}
	record := ledger.records[0]
	if !strings.HasPrefix(record.RefusalReason, "proposal.strict_decode: ") {
		t.Fatalf("причина первого отказа обязана быть поимённой (правило: деталь), получено %q", record.RefusalReason)
	}
}

func TestMachineConstructionRefusesInvalidBudgets(t *testing.T) {
	ledger := &memoryLedger{failAt: -1}
	invalid := planner.Budgets{MaxRounds: 3, RoundTimeout: time.Second, RoundCostUnits: 1, MaxCostUnits: 4}
	if _, err := planner.NewMachine(modelscripted.MustNew(modelscripted.OutcomeValid), ledger, fixedTurnClock{}, invalid); err == nil {
		t.Fatal("невалидная декларация бюджетов обязана давать поимённый отказ конструкции машины")
	} else if !strings.Contains(err.Error(), "budget.max_rounds") {
		t.Fatalf("отказ конструкции обязан нести поимённую причину, получено %v", err)
	}
}

func TestMachineOverBoundedAdapterFailsTurnWhenTheRoundBudgetExhausts(t *testing.T) {
	// The composed proof of Story 8-2: machine(bounded(scripted)) with
	// a NARROWED MaxRounds of one — the first round is malformed (the
	// unrecoverable scenario), the repair round is refused by the
	// round guard, and the machine ends the turn failed with the
	// approved fallback and the NAMED budget cause in the journal —
	// unknown, no blind retry, never a silent continuation.
	ledger := &memoryLedger{failAt: -1}
	budgets := planner.Budgets{MaxRounds: 1, RoundTimeout: 5 * time.Second, RoundCostUnits: 1, MaxCostUnits: 4}
	model := modelbounded.MustNew(modelscripted.MustNew(modelscripted.OutcomeUnrecoverable), budgets)
	machine, err := planner.NewMachine(model, ledger, fixedTurnClock{now: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)}, budgets)
	if err != nil {
		t.Fatalf("машина: %v", err)
	}
	result, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t))
	if err != nil {
		t.Fatalf("исчерпание бюджета — это ход: %v", err)
	}
	if result.Outcome != planner.OutcomeFailed || result.Rounds != 2 || result.Proposal != nil {
		t.Fatalf("исчерпание бюджета раундов обязано давать failed без материала, получено %+v", result)
	}
	if result.Fallback != planner.FallbackHonestStop {
		t.Fatalf("исчерпание бюджета обязано закрываться утверждённым fallback политики контекста, получено %q", result.Fallback)
	}
	if result.Refusal == nil || !strings.Contains(result.Refusal.Error(), "бюджет раундов исчерпан") {
		t.Fatalf("причина обязана нести поимённое слово бюджета раундов, получено %v", result.Refusal)
	}
	if len(ledger.records) != 1 {
		t.Fatalf("журнал обязан нести ровно одну запись, получено %d", len(ledger.records))
	}
	record := ledger.records[0]
	if record.Outcome != planner.OutcomeFailed || !strings.Contains(record.RefusalReason, "бюджет раундов исчерпан") {
		t.Fatalf("failed-запись обязана нести поимённую причину бюджета, получено %+v", record)
	}
	if record.AssemblyVersion != planner.AssemblyVersion || len(record.Budgets) == 0 {
		t.Fatalf("failed-запись обязана нести полную Provenance хода (версия сборки + бюджеты), получено %+v", record)
	}
}

func TestMachineOverBoundedAdapterFailsTurnWhenTheCostBudgetExhausts(t *testing.T) {
	// The cost ceiling of one unit with one-unit rounds: round 1 runs
	// and bills; the repair round would push the cumulative cost past
	// the ceiling — the guard refuses it BY NAME and the turn ends
	// failed with the approved fallback.
	ledger := &memoryLedger{failAt: -1}
	budgets := planner.Budgets{MaxRounds: 2, RoundTimeout: 5 * time.Second, RoundCostUnits: 1, MaxCostUnits: 1}
	model := modelbounded.MustNew(modelscripted.MustNew(modelscripted.OutcomeRepairable), budgets)
	machine, err := planner.NewMachine(model, ledger, fixedTurnClock{now: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)}, budgets)
	if err != nil {
		t.Fatalf("машина: %v", err)
	}
	result, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t))
	if err != nil {
		t.Fatalf("исчерпание стоимости — это ход: %v", err)
	}
	if result.Outcome != planner.OutcomeFailed || result.Proposal != nil {
		t.Fatalf("исчерпание стоимостного бюджета обязано давать failed без материала, получено %+v", result)
	}
	if result.Refusal == nil || !strings.Contains(result.Refusal.Error(), "стоимостной бюджет хода исчерпан") {
		t.Fatalf("причина обязана нести поимённое слово стоимостного бюджета, получено %v", result.Refusal)
	}
	if result.Fallback == "" {
		t.Fatal("исчерпание стоимости обязано закрываться утверждённым fallback")
	}
}

func TestMachineRepairRoundReceivesTheAssembledMaterialWithTheCause(t *testing.T) {
	// The port carries the assembled MATERIAL — the repair cause rides
	// INSIDE it (the adapter switches on the material, never on a
	// context it never saw). A recording port proves the wire shape.
	recorder := &recordingPort{inner: modelscripted.MustNew(modelscripted.OutcomeRepairable)}
	ledger := &memoryLedger{failAt: -1}
	machine, err := planner.NewMachine(recorder, ledger, fixedTurnClock{}, planner.Budgets{MaxRounds: 2, RoundTimeout: 5 * time.Second, RoundCostUnits: 1, MaxCostUnits: 4})
	if err != nil {
		t.Fatalf("машина: %v", err)
	}
	if _, err := machine.Run(context.Background(), runScope(t), rawContextBytes(t)); err != nil {
		t.Fatalf("repairable-ход: %v", err)
	}
	if len(recorder.requests) != 2 {
		t.Fatalf("порт обязан видеть ровно два раунда, получено %d", len(recorder.requests))
	}
	var material planner.RoundMaterial
	if err := json.Unmarshal(recorder.requests[0], &material); err != nil {
		t.Fatalf("материал первого раунда обязан быть читаемой формой: %v", err)
	}
	if material.Repair != "" {
		t.Fatalf("материал первого раунда не несёт причины: %q", material.Repair)
	}
	if err := json.Unmarshal(recorder.requests[1], &material); err != nil {
		t.Fatalf("repair-материал обязан быть читаемой формой: %v", err)
	}
	if !strings.HasPrefix(material.Repair, "proposal.strict_decode: ") {
		t.Fatalf("repair-материал обязан нести поимённую причину внутри, получено %q", material.Repair)
	}
}

// recordingPort records the material bytes of every round.
type recordingPort struct {
	inner    planner.ModelPort
	requests [][]byte
}

func (p *recordingPort) Round(ctx context.Context, request planner.RoundRequest) (planner.RoundResult, error) {
	p.requests = append(p.requests, request.Material)
	return p.inner.Round(ctx, request)
}
