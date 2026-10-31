//go:build integration

package integration_test

// The Story 8-1 integration contour: the turn-contract machine over
// the LIVE PostgreSQL — the scripted model matrix (valid, repairable,
// unrecoverable with the approved fallback, adapter error), the
// context refusal that never starts a turn, the journal uniqueness
// (the honest conflict of the same turn_id), the tenant RLS isolation
// (the records of tenant A are structurally absent for tenant B) and
// the append-only shape of the ledger (the runtime role cannot UPDATE
// or DELETE — the grants ARE the discipline). The machine is the
// production composition: the planner machine over the modelscripted
// adapter and the Postgres journal.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/adapters/modelbounded"
	"github.com/staffy-co/staffy/internal/adapters/modelscripted"
	"github.com/staffy-co/staffy/internal/planner"
	plannerStore "github.com/staffy-co/staffy/internal/planner/store"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// plannerFixture builds the tenant scope, the machine over one
// scripted outcome and the Postgres journal.
type plannerFixture struct {
	scope   tenancy.Context
	machine *planner.Machine
	ledger  *plannerStore.Postgres
}

// plannerDefaultBudgets are the honest default budgets of the
// integration contour (the structural two rounds, a generous timeout
// and ceiling — the budget NARROWING compositions have their own
// fixtures).
func plannerDefaultBudgets() planner.Budgets {
	return planner.Budgets{MaxRounds: 2, RoundTimeout: 5 * time.Second, RoundCostUnits: 1, MaxCostUnits: 4}
}

func newPlannerFixture(t *testing.T, outcome string, suffix string) *plannerFixture {
	t.Helper()
	return newPlannerBudgetedFixture(t, suffix, plannerDefaultBudgets(), modelscripted.MustNew(outcome))
}

// newPlannerBudgetedFixture wires the machine over the GIVEN model
// port (a bounded composition or a plain scripted adapter) and the
// declared budgets — the composition shape of Story 8-2.
func newPlannerBudgetedFixture(t *testing.T, suffix string, budgets planner.Budgets, model planner.ModelPort) *plannerFixture {
	t.Helper()
	identity, owner := newService("00000000"+suffix+"01", "00000000"+suffix+"02", "00000000"+suffix+"03")
	tenant := createTenant(t, identity, owner, "Planner tenant "+suffix, "planner-tenant-"+suffix, "00000000"+suffix+"04").Tenant.ID
	scope, err := tenancy.New(owner.ID, tenant)
	if err != nil {
		t.Fatalf("скоуп: %v", err)
	}
	ledger := plannerStore.NewPostgres(runtimePool)
	machine, err := planner.NewMachine(model, ledger, integrationWallClock{}, budgets)
	if err != nil {
		t.Fatalf("машина: %v", err)
	}
	return &plannerFixture{scope: scope, machine: machine, ledger: ledger}
}

// plannerContextMaterial builds the wire material of a turn context
// for the given tenant and turn id.
func plannerContextMaterial(t *testing.T, tenant id.UUID, turn id.UUID) map[string]any {
	t.Helper()
	return map[string]any{
		"turn_id":          turn.String(),
		"tenant_id":        tenant.String(),
		"case_id":          "01994f64-6c00-7000-8000-000000008a01",
		"conversation_id":  "01994f64-6c00-7000-8000-000000008a02",
		"work_item_id":     "01994f64-6c00-7000-8000-000000008a03",
		"employee_id":      "01994f64-6c00-7000-8000-000000008a04",
		"release_manifest": "release-manifest@14.0.0",
		"pack":             "staffy-core-pack@1.0.0",
		"facts": []any{
			map[string]any{
				"key":   "vehicle.make",
				"value": "Renault",
				"provenance": map[string]any{
					"source": "dialogue.intake", "version": "1", "at": "2026-10-31T10:00:00Z",
				},
			},
		},
		"mode":   "safety",
		"ladder": []string{"safety", "claim", "cancellation", "privacy_no_contact", "human_requested", "unclassified", "sales"},
		"policy_pins": map[string]any{
			"policy_bundle":  "policy-bundle@3.1.0",
			"allowed_claims": "allowed-claims@2.0.0",
			"autonomy":       "autonomy-envelope@1.0.0",
		},
		"tools": []any{
			map[string]any{
				"name": "catalog.lookup@1.0.0",
				"input_schema": map[string]any{
					"query": map[string]any{"label": "Поисковый запрос", "type": "string"},
				},
			},
		},
		"ai_disclosure_required": true,
		"fallback_policy":        "handoff",
	}
}

func plannerRawContext(t *testing.T, tenant id.UUID, turn id.UUID) []byte {
	t.Helper()
	raw, err := json.Marshal(plannerContextMaterial(t, tenant, turn))
	if err != nil {
		t.Fatalf("маршализация контекста: %v", err)
	}
	return raw
}

func TestPlannerTurnContractMachineOverPostgres(t *testing.T) {
	ctx := context.Background()

	// The valid turn: the record is validated, the JSONB proposal rides
	// behind the RLS, the read returns the full business material.
	{
		fixture := newPlannerFixture(t, modelscripted.OutcomeValid, "e7")
		turn := id.MustParse("01994f64-6c00-7000-8000-000000008b01")
		result, err := fixture.machine.Run(ctx, fixture.scope, plannerRawContext(t, fixture.scope.TenantID(), turn))
		if err != nil {
			t.Fatalf("валидный ход: %v", err)
		}
		if result.Outcome != planner.OutcomeValidated || result.Rounds != 1 || result.Proposal == nil {
			t.Fatalf("исход обязан быть validated за 1 раунд с предложением, получено %+v", result)
		}
		record, err := fixture.ledger.Turn(ctx, fixture.scope, turn)
		if err != nil {
			t.Fatalf("чтение записи: %v", err)
		}
		if record.Outcome != planner.OutcomeValidated || record.Build != planner.Build {
			t.Fatalf("запись обязана нести исход и билд, получено %+v", record)
		}
		if record.AssemblyVersion != planner.AssemblyVersion {
			t.Fatalf("запись обязана несть версию сборки входа (Provenance хода 8-2), получено %q", record.AssemblyVersion)
		}
		// The budgets ride as JSONB: PG normalizes the spacing, so the
		// declaration is decoded back into the form and compared by
		// value — format-independent honesty.
		var declaredBudgets planner.Budgets
		if err := json.Unmarshal(record.Budgets, &declaredBudgets); err != nil {
			t.Fatalf("декларация бюджетов обязана декодироваться из JSONB: %v (байты %s)", err, record.Budgets)
		}
		if declaredBudgets != plannerDefaultBudgets() {
			t.Fatalf("запись обязана нести декларацию бюджетов хода, получено %s", record.Budgets)
		}
		var persisted map[string]any
		if err := json.Unmarshal(record.Proposal, &persisted); err != nil {
			t.Fatalf("JSONB-предложение обязано быть валидным JSON: %v", err)
		}
		if persisted["intent"] != planner.IntentRespond {
			t.Fatalf("JSONB-предложение обязано нести материал формы, получено %v", persisted["intent"])
		}
		if record.RefusalReason != "" || record.FallbackForm != "" {
			t.Fatalf("валидная запись не несёт причины и fallback, получено %+v", record)
		}
		if record.WorkItemID == nil {
			t.Fatal("реф Work Item обязан доехать в запись")
		}
	}

	// The repairable turn: exactly two rounds, one repaired record with
	// the honest cause of the first refusal.
	{
		fixture := newPlannerFixture(t, modelscripted.OutcomeRepairable, "e8")
		turn := id.MustParse("01994f64-6c00-7000-8000-000000008b02")
		result, err := fixture.machine.Run(ctx, fixture.scope, plannerRawContext(t, fixture.scope.TenantID(), turn))
		if err != nil {
			t.Fatalf("repairable-ход: %v", err)
		}
		if result.Outcome != planner.OutcomeRepaired || result.Rounds != 2 {
			t.Fatalf("исход обязан быть repaired за 2 раунда, получено outcome=%q rounds=%d", result.Outcome, result.Rounds)
		}
		record, err := fixture.ledger.Turn(ctx, fixture.scope, turn)
		if err != nil {
			t.Fatalf("чтение записи: %v", err)
		}
		if record.Outcome != planner.OutcomeRepaired || !strings.HasPrefix(record.RefusalReason, "proposal.strict_decode: ") {
			t.Fatalf("repaired-запись обязана нести причину первого отказа, получено %+v", record)
		}
	}

	// The unrecoverable turn: failed, the approved fallback of the
	// declared policy (handoff), NEVER the model material.
	{
		fixture := newPlannerFixture(t, modelscripted.OutcomeUnrecoverable, "e9")
		turn := id.MustParse("01994f64-6c00-7000-8000-000000008b03")
		result, err := fixture.machine.Run(ctx, fixture.scope, plannerRawContext(t, fixture.scope.TenantID(), turn))
		if err != nil {
			t.Fatalf("unrecoverable-ход: %v", err)
		}
		if result.Outcome != planner.OutcomeFailed || result.Proposal != nil {
			t.Fatalf("проваленный ход не несёт материала модели (AD-7), получено %+v", result)
		}
		if result.Fallback != planner.FallbackHandoff {
			t.Fatalf("fallback обязан следовать политике контекста handoff, получено %q", result.Fallback)
		}
		record, err := fixture.ledger.Turn(ctx, fixture.scope, turn)
		if err != nil {
			t.Fatalf("чтение записи: %v", err)
		}
		if record.Outcome != planner.OutcomeFailed || record.FallbackForm != planner.FallbackHandoff || len(record.Proposal) != 0 {
			t.Fatalf("failed-запись обязана нести fallback и НЕ нести предложения, получено %+v", record)
		}
		if !strings.Contains(record.RefusalReason, "proposal.schema_version") {
			t.Fatalf("failed-запись обязана нести поимённую причину второго отказа, получено %q", record.RefusalReason)
		}
	}

	// The adapter error: a failed turn without a blind retry.
	{
		fixture := newPlannerFixture(t, modelscripted.OutcomeAdapterError, "ea")
		turn := id.MustParse("01994f64-6c00-7000-8000-000000008b04")
		result, err := fixture.machine.Run(ctx, fixture.scope, plannerRawContext(t, fixture.scope.TenantID(), turn))
		if err != nil {
			t.Fatalf("ошибка адаптера — это ход: %v", err)
		}
		if result.Outcome != planner.OutcomeFailed || result.Rounds != 1 || result.Proposal != nil {
			t.Fatalf("ошибка адаптера = failed за 1 раунд без материала, получено %+v", result)
		}
		if result.Refusal == nil || result.Refusal.Rule != "planner.adapter_round" {
			t.Fatalf("причина обязана быть поимённой ошибкой адаптера, получено %v", result.Refusal)
		}
	}

	// The refused context: no round, no record.
	{
		fixture := newPlannerFixture(t, modelscripted.OutcomeValid, "eb")
		turn := id.MustParse("01994f64-6c00-7000-8000-000000008b05")
		material := plannerContextMaterial(t, fixture.scope.TenantID(), turn)
		material["mode"] = "не-слово-словаря"
		raw, err := json.Marshal(material)
		if err != nil {
			t.Fatalf("маршализация: %v", err)
		}
		result, err := fixture.machine.Run(ctx, fixture.scope, raw)
		if err != nil {
			t.Fatalf("отказ контекста — не ошибка вызова: %v", err)
		}
		if result.Started || result.Rounds != 0 {
			t.Fatalf("отказанный контекст не начинает ход, получено %+v", result)
		}
		if _, err := fixture.ledger.Turn(ctx, fixture.scope, turn); !errors.Is(err, planner.ErrTurnNotFound) {
			t.Fatalf("отказ входа журналом не является, получено %v", err)
		}
	}
}

func TestPlannerTurnLedgerUniquenessAndRLSIsolation(t *testing.T) {
	ctx := context.Background()
	fixture := newPlannerFixture(t, modelscripted.OutcomeValid, "ec")
	turn := id.MustParse("01994f64-6c00-7000-8000-000000008b06")
	raw := plannerRawContext(t, fixture.scope.TenantID(), turn)

	// The first recording happens exactly once.
	if _, err := fixture.machine.Run(ctx, fixture.scope, raw); err != nil {
		t.Fatalf("первый ход: %v", err)
	}
	// The repeated command of the same turn is the honest conflict —
	// the second recording of the same turn does not happen.
	_, err := fixture.machine.Run(ctx, fixture.scope, raw)
	if !planner.IsTurnConflict(err) {
		t.Fatalf("повторный ход обязан дать честный конфликт уникальности, получено %v", err)
	}

	// Tenant B: the records of tenant A are structurally absent (the
	// RLS view of the other tenant never discloses the existence).
	other := newPlannerFixture(t, modelscripted.OutcomeValid, "ed")
	if _, err := other.ledger.Turn(ctx, other.scope, turn); !errors.Is(err, planner.ErrTurnNotFound) {
		t.Fatalf("запись чужого тенента структурно отсутствует, получено %v", err)
	}
	// The count probe runs through the runtime pool of the OTHER scope:
	// the RLS row filter hides the tenant A rows.
	{
		tx, err := runtimePool.Begin(ctx)
		if err != nil {
			t.Fatalf("транзакция: %v", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		for _, pair := range [][2]string{
			{"staffy.principal_id", other.scope.PrincipalID().String()},
			{"staffy.tenant_id", other.scope.TenantID().String()},
		} {
			if _, err := tx.Exec(ctx, "select set_config($1,$2,true)", pair[0], pair[1]); err != nil {
				t.Fatalf("скоуп: %v", err)
			}
		}
		var count int
		if err := tx.QueryRow(ctx, "select count(*) from planner.turns").Scan(&count); err != nil {
			t.Fatalf("подсчёт: %v", err)
		}
		if count != 0 {
			t.Fatalf("зона видимости тенента B обязана быть пустой, получено %d записей", count)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("коммит: %v", err)
		}
	}

	// The append-only shape: the runtime role CANNOT update or delete —
	// the grants ARE the discipline (the journal correction does not
	// exist).
	{
		_, err := runtimePool.Exec(ctx, "update planner.turns set outcome='validated'")
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("UPDATE журнала обязан быть запрещён правами (append-only), получено %v", err)
		}
		_, err = runtimePool.Exec(ctx, "delete from planner.turns")
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("DELETE журнала обязан быть запрещён правами (append-only), получено %v", err)
		}
	}

	// The tenant A side still sees its record — the isolation probe
	// never broke the material.
	record, err := fixture.ledger.Turn(ctx, fixture.scope, turn)
	if err != nil {
		t.Fatalf("запись тенента A жива: %v", err)
	}
	if record.Outcome != planner.OutcomeValidated {
		t.Fatalf("запись тенента A обязана остаться валидной, получено %+v", record)
	}
}

// integrationWallClock advances with the real time: the journal
// moments of the matrix stay distinct (the fixed clock would tie the
// rows of the same turn id across fixtures).
type integrationWallClock struct{}

func (integrationWallClock) Now() time.Time { return time.Now().UTC() }

func TestPlannerBoundedBudgetsFailTurnsHonestlyOverPostgres(t *testing.T) {
	ctx := context.Background()

	// The narrowed round ceiling: the machine over the bounded adapter
	// with MaxRounds=1 — round 1 is malformed (the unrecoverable
	// scenario), the repair round is refused by the round guard, and
	// the turn ends failed with the approved fallback and the NAMED
	// budget cause in the LIVE journal (unknown — no blind retry).
	{
		budgets := planner.Budgets{MaxRounds: 1, RoundTimeout: 5 * time.Second, RoundCostUnits: 1, MaxCostUnits: 4}
		model := modelbounded.MustNew(modelscripted.MustNew(modelscripted.OutcomeUnrecoverable), budgets)
		fixture := newPlannerBudgetedFixture(t, "ee", budgets, model)
		turn := id.MustParse("01994f64-6c00-7000-8000-000000008b07")
		result, err := fixture.machine.Run(ctx, fixture.scope, plannerRawContext(t, fixture.scope.TenantID(), turn))
		if err != nil {
			t.Fatalf("исчерпание бюджета раундов — это ход: %v", err)
		}
		if result.Outcome != planner.OutcomeFailed || result.Proposal != nil || result.Fallback != planner.FallbackHandoff {
			t.Fatalf("исчерпание бюджета раундов: failed + fallback политики контекста без материала, получено %+v", result)
		}
		record, err := fixture.ledger.Turn(ctx, fixture.scope, turn)
		if err != nil {
			t.Fatalf("чтение записи: %v", err)
		}
		if record.Outcome != planner.OutcomeFailed || !strings.Contains(record.RefusalReason, "бюджет раундов исчерпан") {
			t.Fatalf("failed-запись обязана нести поимённую причину бюджета раундов, получено %+v", record)
		}
		if record.AssemblyVersion != planner.AssemblyVersion || len(record.Budgets) == 0 {
			t.Fatalf("failed-запись обязана нести полную Provenance хода, получено %+v", record)
		}
	}

	// The cost ceiling: one-unit rounds under a one-unit ceiling — the
	// repair round never runs (the named cost error fires BEFORE it),
	// and the failed record carries the cause by name.
	{
		budgets := planner.Budgets{MaxRounds: 2, RoundTimeout: 5 * time.Second, RoundCostUnits: 1, MaxCostUnits: 1}
		model := modelbounded.MustNew(modelscripted.MustNew(modelscripted.OutcomeRepairable), budgets)
		fixture := newPlannerBudgetedFixture(t, "ef", budgets, model)
		turn := id.MustParse("01994f64-6c00-7000-8000-000000008b08")
		result, err := fixture.machine.Run(ctx, fixture.scope, plannerRawContext(t, fixture.scope.TenantID(), turn))
		if err != nil {
			t.Fatalf("исчерпание стоимостного бюджета — это ход: %v", err)
		}
		if result.Outcome != planner.OutcomeFailed || result.Proposal != nil {
			t.Fatalf("исчерпание стоимости: failed без материала, получено %+v", result)
		}
		if result.Refusal == nil || !strings.Contains(result.Refusal.Error(), "стоимостной бюджет хода исчерпан") {
			t.Fatalf("причина обязана нести поимённое слово стоимостного бюджета, получено %v", result.Refusal)
		}
		record, err := fixture.ledger.Turn(ctx, fixture.scope, turn)
		if err != nil {
			t.Fatalf("чтение записи: %v", err)
		}
		if !strings.Contains(record.RefusalReason, "стоимостной бюджет хода исчерпан") {
			t.Fatalf("failed-запись обязана нести поимённую причину стоимости, получено %q", record.RefusalReason)
		}
	}
}

func TestPlannerTurnLedgerPre82HistoryReadsAsHonestNULL(t *testing.T) {
	ctx := context.Background()
	fixture := newPlannerFixture(t, modelscripted.OutcomeValid, "fd")
	turn := id.MustParse("01994f64-6c00-7000-8000-000000008b09")

	// A pre-8-2 record: inserted through the ADMIN pool directly with
	// the provenance columns untouched (NULL) — the shape of every
	// journal row written before the Story 8-2 machine existed.
	admin := adminPool
	_, err := admin.Exec(ctx, `insert into planner.turns (
		tenant_id, turn_id, case_id, conversation_id, employee_id,
		outcome, schema_version, build, created_at
	) values ($1,$2,$3,$4,$5,'failed',$6,$7,$8)`,
		fixture.scope.TenantID(), turn,
		id.MustParse("01994f64-6c00-7000-8000-000000008a01"),
		id.MustParse("01994f64-6c00-7000-8000-000000008a02"),
		id.MustParse("01994f64-6c00-7000-8000-000000008a04"),
		planner.ProposalSchemaVersion, planner.Build, time.Now().UTC())
	if err != nil {
		t.Fatalf("вставка истории до 8-2: %v", err)
	}

	// The read maps the NULL columns to the honest absence: no assembly
	// version, no budgets — sealed history is never recomputed (AD-10).
	record, err := fixture.ledger.Turn(ctx, fixture.scope, turn)
	if err != nil {
		t.Fatalf("чтение исторической записи: %v", err)
	}
	if record.AssemblyVersion != "" || record.Budgets != nil {
		t.Fatalf("история до 8-2 читается как честное отсутствие Provenance-колонок, получено assembly=%q budgets=%s", record.AssemblyVersion, record.Budgets)
	}
	if record.Outcome != planner.OutcomeFailed {
		t.Fatalf("историческая запись обязана доезжать как есть, получено %+v", record)
	}
}
