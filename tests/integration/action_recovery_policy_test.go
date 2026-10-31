//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionApp "github.com/staffy-co/staffy/internal/action/app"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	actionStore "github.com/staffy-co/staffy/internal/action/store"
	"github.com/staffy-co/staffy/internal/adapters/capabilityresolver"
	"github.com/staffy-co/staffy/internal/adapters/connectorrefresolver"
	"github.com/staffy-co/staffy/internal/adapters/controleffectiveresolver"
	"github.com/staffy-co/staffy/internal/adapters/owningsystem"
	"github.com/staffy-co/staffy/internal/adapters/packresolver"
	controlAPI "github.com/staffy-co/staffy/internal/control/api"
	controlApp "github.com/staffy-co/staffy/internal/control/app"
	controlDomain "github.com/staffy-co/staffy/internal/control/domain"
	controlStore "github.com/staffy-co/staffy/internal/control/store"
	identityApp "github.com/staffy-co/staffy/internal/identity/app"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	identityStore "github.com/staffy-co/staffy/internal/identity/store"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// TestActionRecoveryPolicyMatrixRowsBySpec is the Story 3.9 step-04
// INTEGRATION MATRIX of the recovery action policy over the LIVE
// PG/RLS world of 3-8 (the same real service, the same exhausted
// operations): the full allowed set of the exhausted case (the FIVE
// kinds with the DECLARED AC-2 attributes, the honest words, the
// human-only class listed SEPARATELY), the derivation idempotence (the
// repeated read answers the SAME view, NOT ONE row lands anywhere — the
// policy is a read-time derivation), the LIVE paused scoped control
// (the tenant point through the REAL 3-2 surface blocks the MATERIAL
// kinds with the EXACT control word while the read-only kinds stay
// allowed), the AG-4 composition world (the nil reconciliation adapter
// — the honest sentence rides the status word, the policy is never
// blocked), the EMPTY catalog semantics (the blanked declaration of a
// variant service answers human_only honestly — the fail-closed AC-1
// tail over the live store), the honest 404 of the unknown case and
// the RLS blindness of the foreign tenant (the membership gate BEFORE
// any material), the import-graph probe (the production root excludes
// the synthetic adapters — AG-4) and the dictionary mirrors of the
// live derivation (the kind set == the compile-time dictionary).
func TestActionRecoveryPolicyMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	w, svc, dispatch, reconcile, owner := recoveryWorld(t, 381)
	caseID, workItemID := w.acceptedCase(t, "evt-recpol-1", "sess-recpol-1", `{"call":"inbound-call","from":"+79000000901"}`)
	now := actionReconcileNow
	_ = reconcile

	// exhaustedCase walks the FULL live path to ONE Recovery Case of an
	// exhausted unknown operation: the typed-valid proposal → confirm →
	// authorize → the BROKEN dispatch (the honest unknown of 3-6) → THREE
	// no_answer reconciliation attempts (the bounded exhaustion of 3-7) →
	// the creation command (the case row with the assignment of the LIVE
	// readings).
	exhaustedCase := func(row int64, key string, service *actionApp.Service) actionAPI.RecoveryCaseView {
		t.Helper()
		op := reconcileAuthorizedOperation(t, w, service, owner, caseID, workItemID, row, authorizeMatrixUnlimited,
			map[string]string{"phone": fmt.Sprintf("+790000009%02d", row)}, nil, nil, key, 3600, now)
		dispatch.inner.Script("", "", errors.New("адаптер недоступен: обрыв соединения до провайдера"))
		dispatch.reset()
		if _, err := service.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: now.Add(10 * time.Minute),
		}); err != nil {
			t.Fatalf("the broken send of the row %d must answer its report: %v", row, err)
		}
		reconcile.inner.Script("", "", errors.New("reconciliation adapter unreachable"))
		for minute := 40; minute <= 120; minute += 40 {
			if _, err := service.ReconcileUnknownOperations(ctx, actionApp.ReconcileCommand{
				Actor: owner, TenantID: w.tenant, Now: now.Add(time.Duration(minute) * time.Minute),
			}); err != nil {
				t.Fatalf("the no_answer attempt of the minute %d must answer: %v", minute, err)
			}
		}
		result, err := service.CreateRecoveryCase(ctx, actionAPI.RecoveryCreateCommand{
			Actor: owner, TenantID: w.tenant, OperationID: op.OperationID, Now: now.Add(200 * time.Minute),
		})
		if err != nil {
			t.Fatalf("the exhausted unknown must create the case: %v", err)
		}
		return result.Case
	}

	// policyAt reads the policy of ONE case at the live service.
	policyAt := func(service *actionApp.Service, caseView actionAPI.RecoveryCaseView) (actionAPI.RecoveryPolicyView, error) {
		return service.RecoveryCasePolicy(ctx, owner, w.tenant, caseView.RecoveryCaseID)
	}

	// policyActionOf resolves ONE kind answer of the view (the by-name
	// probe of the matrix).
	policyActionOf := func(t *testing.T, view actionAPI.RecoveryPolicyView, kind string) actionAPI.RecoveryActionView {
		t.Helper()
		for _, action := range view.Actions {
			if action.Kind == kind {
				return action
			}
		}
		t.Fatalf("the policy view must carry the kind %q", kind)
		return actionAPI.RecoveryActionView{}
	}

	// row-1: полный набор exhausted-кейса — read-only allowed ×2,
	// request_evidence allowed (подтверждение), prepare_pause_cancel
	// allowed (LIVE-семантика каталога объявлена), offer_approved_options
	// human_only «вариантов нет», human-only класс 2 ОТДЕЛЬНО +
	// класс-текст PRD FR-39 дословно; AC-2 атрибуты полны на каждом
	// виде (при wired-сверке мира AG-4 предложение НЕ едет).
	t.Run("row-1 полный набор exhausted-кейса", func(t *testing.T) {
		card := exhaustedCase(1, "recpol-key-1", svc)
		view, err := policyAt(svc, card)
		if err != nil {
			t.Fatalf("the policy derivation must answer: %v", err)
		}
		if len(view.Actions) != 5 || len(view.HumanOnlyDecisions) != 2 {
			t.Fatalf("the view must carry 5 kinds and 2 human-only words, got %d/%d", len(view.Actions), len(view.HumanOnlyDecisions))
		}
		if view.RecoveryCaseID != card.RecoveryCaseID {
			t.Fatalf("the view must pin the case identity")
		}
		for kind, want := range map[string]string{
			actionDomain.RecoveryActionShowRecordedConditions: actionDomain.RecoveryActionStateAllowed,
			actionDomain.RecoveryActionTruthfulStatusUpdate:   actionDomain.RecoveryActionStateAllowed,
			actionDomain.RecoveryActionRequestEvidence:        actionDomain.RecoveryActionStateAllowed,
			actionDomain.RecoveryActionOfferApprovedOptions:   actionDomain.RecoveryActionStateHumanOnly,
			actionDomain.RecoveryActionPreparePauseCancel:     actionDomain.RecoveryActionStateAllowed,
		} {
			if action := policyActionOf(t, view, kind); action.State != want {
				t.Fatalf("the kind %s must answer %s, got %s (%s)", kind, want, action.State, action.StateText)
			}
		}
		pause := policyActionOf(t, view, actionDomain.RecoveryActionPreparePauseCancel)
		if !strings.Contains(pause.StateText, "CancelCompensationSem") {
			t.Fatalf("the allowed pause/cancel must speak the LIVE semantics word: %q", pause.StateText)
		}
		options := policyActionOf(t, view, actionDomain.RecoveryActionOfferApprovedOptions)
		if !strings.Contains(options.StateText, "вариантов решения в конфигурации нет") {
			t.Fatalf("the options kind must speak the honest no-options word: %q", options.StateText)
		}
		status := policyActionOf(t, view, actionDomain.RecoveryActionTruthfulStatusUpdate)
		if strings.Contains(status.StateText, actionDomain.RecoveryActionAG4NotWiredText) {
			t.Fatalf("the wired reconciliation world must NOT carry the AG-4 sentence: %q", status.StateText)
		}
		// AC-2: every kind answer carries the full declared attributes.
		for _, action := range view.Actions {
			decl := actionDomain.RecoveryActionKindByWord(action.Kind)
			if decl == nil || action.ScopeText != decl.ScopeText || action.EligibilityRuleText != decl.EligibilityRuleText ||
				action.RequiredFactsText != decl.RequiredFactsText || action.AuthorityText != decl.AuthorityText ||
				action.ConfirmationRequired != decl.ConfirmationRequired {
				t.Fatalf("the kind %s must carry the DECLARED attributes verbatim (AC-2)", action.Kind)
			}
		}
		// AC-1: the human-only class rides SEPARATELY with the class word.
		if view.HumanOnlyDecisions[0].Kind != actionDomain.RecoveryHumanOnlyLiabilityOrFault ||
			view.HumanOnlyDecisions[1].Kind != actionDomain.RecoveryHumanOnlyNonstandardOutcome {
			t.Fatalf("the human-only class words: %+v", view.HumanOnlyDecisions)
		}
		if view.HumanOnlyClassText != actionDomain.RecoveryHumanOnlyClassText {
			t.Fatalf("the class word must be the PRD FR-39 verbatim text")
		}
	})

	// row-2: идемпотентность вывода — повторный запрос отвечает ТОТ ЖЕ
	// вывод байт-стабильно, НИ ОДНОЙ записи (счётчики строк/событий
	// стабильны).
	t.Run("row-2 идемпотентность вывода: байт-стабильно, записей ноль", func(t *testing.T) {
		card := exhaustedCase(2, "recpol-key-2", svc)
		first, err := policyAt(svc, card)
		if err != nil {
			t.Fatalf("the first derivation must answer: %v", err)
		}
		var rows, events int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_cases where tenant_id=$1`,
			w.tenant).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_events e join action.recovery_cases c on c.tenant_id=e.tenant_id and c.recovery_case_id=e.recovery_case_id where c.tenant_id=$1`,
			w.tenant).Scan(&events); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			repeat, err := policyAt(svc, card)
			if err != nil {
				t.Fatalf("the repeat derivation %d must answer: %v", i, err)
			}
			if !reflect.DeepEqual(first, repeat) {
				t.Fatalf("the repeat derivation must be byte-stable (идемпотентность выводом)")
			}
		}
		var rowsAfter, eventsAfter int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_cases where tenant_id=$1`,
			w.tenant).Scan(&rowsAfter); err != nil || rowsAfter != rows {
			t.Fatalf("the policy writes NO row: %d → %d (%v)", rows, rowsAfter, err)
		}
		if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_events e join action.recovery_cases c on c.tenant_id=e.tenant_id and c.recovery_case_id=e.recovery_case_id where c.tenant_id=$1`,
			w.tenant).Scan(&eventsAfter); err != nil || eventsAfter != events {
			t.Fatalf("the policy appends NO event: %d → %d (%v)", events, eventsAfter, err)
		}
	})

	// row-3: LIVE paused scoped control (the tenant point through the
	// REAL 3-2 surface) — материальные виды blocked ТОЧНЫМ словом
	// scoped control (режим+причина), read-only остаются allowed,
	// offer_approved_options остаётся human_only; лифт лечит мир.
	t.Run("row-3 paused tenant живьём: материальные blocked точным словом", func(t *testing.T) {
		card := exhaustedCase(3, "recpol-key-3", svc)
		identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, newPrivacyIDs(23700))
		operator := inviteMember(t, identityLive, w.owner, w.tenant, "recpol-ops@example.org", identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "00000000d901", "00000000d902")
		operator.StepUpVerified = true
		controls := controlApp.New(controlStore.NewPostgres(runtimePool), identityLive, connectorrefresolver.New(), fixedClock{actionReconcileNow}, &sequenceIDs{values: controlMatrixIDs(21)})
		pausedReason := "восстановление приостановлено: инвентаризация операций тенента"
		if _, err := controls.Set(ctx, operator, w.tenant, controlAPI.SetRequest{
			Kind: controlDomain.ScopeKindTenant, Ref: w.tenant.String(), Mode: controlDomain.ControlModePaused,
			Reason: pausedReason, ExpectedGeneration: 0, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000d911"),
		}); err != nil {
			t.Fatal(err)
		}
		view, err := policyAt(svc, card)
		if err != nil {
			t.Fatalf("the paused world policy must answer (право ограничивается): %v", err)
		}
		for _, kind := range []string{actionDomain.RecoveryActionRequestEvidence, actionDomain.RecoveryActionPreparePauseCancel} {
			action := policyActionOf(t, view, kind)
			if action.State != actionDomain.RecoveryActionStateBlocked {
				t.Fatalf("the paused tenant must block the material kind %s, got %s", kind, action.State)
			}
			if !strings.Contains(action.StateText, actionDomain.ScopedControlPaused+": "+pausedReason) {
				t.Fatalf("the paused word must carry the EXACT mode and reason, got %q", action.StateText)
			}
		}
		for _, kind := range []string{actionDomain.RecoveryActionShowRecordedConditions, actionDomain.RecoveryActionTruthfulStatusUpdate} {
			if action := policyActionOf(t, view, kind); action.State != actionDomain.RecoveryActionStateAllowed {
				t.Fatalf("the paused tenant keeps the read-only kind %s allowed (чтение остаётся), got %s", kind, action.State)
			}
		}
		if action := policyActionOf(t, view, actionDomain.RecoveryActionOfferApprovedOptions); action.State != actionDomain.RecoveryActionStateHumanOnly {
			t.Fatalf("the paused tenant keeps offer_approved_options human_only, got %s", action.State)
		}
		// The lift heals the world (the paused tenant would park every
		// later row — the dispatch matrix lift precedent).
		if _, err := controls.Set(ctx, operator, w.tenant, controlAPI.SetRequest{
			Kind: controlDomain.ScopeKindTenant, Ref: w.tenant.String(), Mode: controlDomain.ControlModeAccept,
			Reason: "инвентаризация завершена: контроль снят", ExpectedGeneration: 1, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000d912"),
		}); err != nil {
			t.Fatal(err)
		}
		healed, err := policyAt(svc, card)
		if err != nil {
			t.Fatal(err)
		}
		if action := policyActionOf(t, healed, actionDomain.RecoveryActionRequestEvidence); action.State != actionDomain.RecoveryActionStateAllowed {
			t.Fatalf("the lift heals the material kinds, got %s", action.State)
		}
	})

	// row-4: AG-4-мир (nil-адаптер сверки) — кейс рождается на триггере
	// reconciliation_unavailable (сверка отказывает честным словом ДО
	// очереди — попытки не тратятся, прецедент row-3 из 3-8);
	// статусное слово несёт «Сверка не подключена», политика НЕ
	// блокируется, набор выводится (факт композиции — самое честное
	// слово, не отказ).
	t.Run("row-4 AG-4-мир: статусное слово честно, политика не блокируется", func(t *testing.T) {
		ag4 := w.buildActionReconcileService(dispatch, nil, capabilityresolver.New(), owningsystem.New(), actionPreviewIDs(7200, 512)).WithRecovery(actionStore.NewPostgres(runtimePool))
		op := reconcileAuthorizedOperation(t, w, ag4, owner, caseID, workItemID, 4, authorizeMatrixUnlimited,
			map[string]string{"phone": "+79000000904"}, nil, nil, "recpol-key-4", 3600, now)
		dispatch.inner.Script("", "", errors.New("адаптер недоступен: обрыв соединения до провайдера"))
		dispatch.reset()
		if _, err := ag4.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: now.Add(10 * time.Minute),
		}); err != nil {
			t.Fatalf("the broken send must answer its report: %v", err)
		}
		created, err := ag4.CreateRecoveryCase(ctx, actionAPI.RecoveryCreateCommand{
			Actor: owner, TenantID: w.tenant, OperationID: op.OperationID, Now: now.Add(60 * time.Minute),
		})
		if err != nil {
			t.Fatalf("the AG-4 unknown must create the case on the unavailable trigger: %v", err)
		}
		if created.Case.Trigger != actionDomain.RecoveryTriggerReconciliationUnavailable {
			t.Fatalf("the AG-4 world trigger: %q, wanted reconciliation_unavailable", created.Case.Trigger)
		}
		view, err := policyAt(ag4, created.Case)
		if err != nil {
			t.Fatalf("the AG-4 world policy must answer: %v", err)
		}
		status := policyActionOf(t, view, actionDomain.RecoveryActionTruthfulStatusUpdate)
		if status.State != actionDomain.RecoveryActionStateAllowed {
			t.Fatalf("the AG-4 fact never blocks the policy, got %s", status.State)
		}
		if !strings.Contains(status.StateText, actionDomain.RecoveryActionAG4NotWiredText) {
			t.Fatalf("the AG-4 sentence must ride the status word: %q", status.StateText)
		}
		if action := policyActionOf(t, view, actionDomain.RecoveryActionRequestEvidence); action.State != actionDomain.RecoveryActionStateAllowed {
			t.Fatalf("the AG-4 fact never blocks the material kinds either, got %s", action.State)
		}
	})

	// row-5: пустая семантика каталога (variant-сервис с очищенной
	// декларацией CancelCompensationSem — admission обойдён осознанно:
	// вывод обязан честно fail-closed даже на невозможимом через
	// GuardValidateActionDefinition определении) — prepare_pause_cancel
	// human_only «семантика не объявлена каталогом», read-only остаются
	// allowed, политика 200-выводом.
	t.Run("row-5 пустая семантика: human_only fail-closed", func(t *testing.T) {
		card := exhaustedCase(5, "recpol-key-5", svc)
		// The variant dictionary: the SAME dictionary with the blanked
		// CancelCompensationSem of the unlimited definition (the case pins
		// resolve against it — the derivation must fail closed honestly).
		variantDefs := operationAuthorizationDictionary()
		for i := range variantDefs {
			if variantDefs[i].ID == authorizeMatrixUnlimited.ID {
				variantDefs[i].DeliveryContract.CancelCompensationSem = ""
			}
		}
		variant := actionApp.New(w.identity, packresolver.New(), capabilityresolver.New(), variantDefs).
			WithPreviews(actionStore.NewPostgres(runtimePool),
				controleffectiveresolver.NewRouteAdapter(controlStore.NewPostgres(runtimePool), fixedClock{operationNow}),
				w.cases, w.identity, w.entitlement, w.employee, w.connector, w.employee, actionPreviewIDs(7300, 512)).
			WithRecovery(actionStore.NewPostgres(runtimePool))
		view, err := policyAt(variant, card)
		if err != nil {
			t.Fatalf("the blanked semantics policy must answer (право ограничивается): %v", err)
		}
		pause := policyActionOf(t, view, actionDomain.RecoveryActionPreparePauseCancel)
		if pause.State != actionDomain.RecoveryActionStateHumanOnly || !strings.Contains(pause.StateText, "семантика не объявлена каталогом") {
			t.Fatalf("the blanked semantics must answer human_only honestly, got %s (%s)", pause.State, pause.StateText)
		}
		if action := policyActionOf(t, view, actionDomain.RecoveryActionRequestEvidence); action.State != actionDomain.RecoveryActionStateAllowed {
			t.Fatalf("the blanked semantics never blocks the independent kinds, got %s", action.State)
		}
	})

	// row-6: неизвестный кейс — честный 404 стора (membership-слепота ДО
	// материала).
	t.Run("row-6 неизвестный кейс: 404", func(t *testing.T) {
		_, err := svc.RecoveryCasePolicy(ctx, owner, w.tenant, id.MustParse("01994f64-6c00-7000-8000-00000000dead"))
		if !errors.Is(err, actionPort.ErrRecoveryNotFound) {
			t.Fatalf("the unknown case must answer the honest 404, got %v", err)
		}
	})

	// row-7: RLS-слепота чужого тенента — гейт membership отвечает 404
	// ДО материала (чужой тенент не видит НИЧЕГО).
	t.Run("row-7 RLS чужого тенента: 404 до материала", func(t *testing.T) {
		foreign := id.MustParse("01994f64-6c00-7000-8000-00000000beef")
		_, err := svc.RecoveryCasePolicy(ctx, owner, foreign, id.MustParse("01994f64-6c00-7000-8000-00000000dead"))
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the foreign tenant must answer the membership 404, got %v", err)
		}
	})

	// row-8: словари-зеркала живой деривации — набор видов вывода ==
	// compile-time словарю (порядок фиксирован), вне-словарное слово не
	// выводится (машинный вид — будущий planner валидирует против него,
	// AD-7).
	t.Run("row-8 словари-зеркала живьём", func(t *testing.T) {
		card := exhaustedCase(8, "recpol-key-8", svc)
		view, err := policyAt(svc, card)
		if err != nil {
			t.Fatal(err)
		}
		for i, kind := range actionDomain.RecoveryActionKindWords {
			if view.Actions[i].Kind != kind {
				t.Fatalf("the view kinds must ride the dictionary order: [%d]=%s, wanted %s", i, view.Actions[i].Kind, kind)
			}
		}
		for i, word := range actionDomain.RecoveryHumanOnlyWords {
			if view.HumanOnlyDecisions[i].Kind != word {
				t.Fatalf("the human-only words must ride the dictionary order: [%d]=%s, wanted %s", i, view.HumanOnlyDecisions[i].Kind, word)
			}
		}
		for _, action := range view.Actions {
			if !actionDomain.WordIn(actionDomain.RecoveryActionStateWords, action.State) {
				t.Fatalf("a foreign state word in the live derivation: %q", action.State)
			}
		}
	})

	// row-9: import-graph — продакшн-корень cmd/staffy-core НЕ импортирует
	// синтетические адаптеры (AG-4: политика — локальный контур, сеть
	// остаётся в dispatch/reconcile; зонд прецедента 3-7 row-22).
	t.Run("row-9 import-graph: прод исключает синтетику", func(t *testing.T) {
		cmd := exec.Command("go", "list", "-deps", "./cmd/staffy-core")
		cmd.Dir = "../.."
		out, err := cmd.Output()
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				t.Fatalf("the go list probe must answer: %v: %s", err, exit.Stderr)
			}
			t.Fatalf("the go list probe must run: %v", err)
		}
		deps := string(out)
		for _, synthetic := range []string{"internal/adapters/reconciliationsyn", "internal/adapters/actiondispatcher"} {
			if strings.Contains(deps, synthetic) {
				t.Fatalf("the production root must never import the synthetic adapter %s (AG-4)", synthetic)
			}
		}
	})
}
