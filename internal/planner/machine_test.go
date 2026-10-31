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
// memory ledger.
func machineFixture(outcome string) (*planner.Machine, *memoryLedger) {
	ledger := &memoryLedger{failAt: -1}
	machine := planner.NewMachine(modelscripted.MustNew(outcome), ledger, fixedTurnClock{now: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)})
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
