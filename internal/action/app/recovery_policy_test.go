package app

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/action/api"
	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/action/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The 3.9 step-02 unit matrix of the recovery action policy read — the
// zero-write property on EVERY path (the fake-repo counters: the policy
// is a READ-TIME derivation, NOT ONE row lands anywhere — the idempotence
// is the derivation itself), the frozen read order (the gate ladder
// BEFORE the reads: the membership and the scope word answer 404/403
// with the case counter at zero; the case read and the facts read ride
// ONE call each), the full derivation over the unit world (the allowed
// set of the created case, the paused control blocking the material
// kinds with the EXACT control word, the unreadable control fail-closed,
// the AG-4 honest sentence of the unwired reconciliation, the frozen
// bundle pins fallback of the facts miss, the unknown case 404), and
// the locality probes (the reflect signature probe: no provider adapter
// rides the policy path; the store-method probe: the recovery
// repository contract stays at the FIVE reused methods — the policy
// adds NO store method, the reads are reused as they stand).

// ---------------------------------------------------------------------------
// The unit world of the policy read
// ---------------------------------------------------------------------------

// policyFixture wires the unit world with a CREATED case: the fixture
// of the creation contour runs its honest command first (the case row
// with the created event lands ONCE), then the policy reads reuse the
// same fakes (the control is scriptable per test).
func policyFixture(t *testing.T, control port.ControlEffective, controlErr error) *recoveryFixture {
	t.Helper()
	f := newRecoveryFixture(t)
	service := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Operations: true}},
		acceptAllPacks{}, acceptAllCaps{}, f.defs).
		WithPreviews(f.previews, fakeControl{mode: control, err: controlErr},
			f.material, liveMembers{world: &previewLiveFixture{}}, fakeEntitlement{status: "active"},
			fakeReadiness{stage: "admitted"}, fakeConnectors{state: "active"}, f.routes, &fakeIDs{}).
		WithRecovery(f.repo)
	result, err := service.createRecoveryCase(context.Background(), RecoveryCreateCommand{
		Actor: testActor, TenantID: testTenant, OperationID: recoveryOperationID, Now: f.now,
	})
	if err != nil {
		t.Fatalf("фикстура: создание кейса обязано проходить: %v", err)
	}
	if result.Decision != port.RecoveryDecisionCreated {
		t.Fatalf("фикстура: решение создания: %q, ожидалось created", result.Decision)
	}
	f.policyCaseID = result.Case.RecoveryCaseID
	f.policyService = service
	return f
}

// policyAccept is the accept control of the full-allowed world.
var policyAccept = port.ControlEffective{Mode: domain.ScopedControlAccept, Generation: 3}

// policyPaused is the paused control of the blocked-material world (the
// winning reason rides the EXACT word).
var policyPaused = port.ControlEffective{Mode: domain.ScopedControlPaused, Generation: 4, Reason: "connector paused оператором"}

// policyState resolves ONE kind answer of the policy view.
func policyState(t *testing.T, policy api.RecoveryPolicyView, kind string) api.RecoveryActionView {
	t.Helper()
	for _, action := range policy.Actions {
		if action.Kind == kind {
			return action
		}
	}
	t.Fatalf("вид «%s» отсутствует в выводе политики", kind)
	return api.RecoveryActionView{}
}

// TestRecoveryPolicyZeroWritesOnEveryPath pins the ZERO-WRITE property
// of the policy read: на ЛЮБОМ пути вывода (полный набор, paused,
// нечитаемый контроль, AG-4, неизвестный кейс, гейты) — create=0,
// строк не прибавляется, назначение живыми чтениями НЕ читается
// (слова политики — вывод, не запись: идемпотентность выводом).
func TestRecoveryPolicyZeroWritesOnEveryPath(t *testing.T) {
	f := policyFixture(t, policyAccept, nil)
	rowsBefore := len(f.repo.rows)
	createBefore := f.repo.createCalls
	materialBefore := f.material.caseCalls
	routesBefore := f.routes.routeCalls
	for i := 0; i < 3; i++ {
		if _, err := f.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, f.policyCaseID); err != nil {
			t.Fatalf("повтор[%d]: вывод политики отказал: %v", i, err)
		}
	}
	if f.repo.createCalls != createBefore || len(f.repo.rows) != rowsBefore {
		t.Fatalf("политика пишет: create=%d rows=%d (до: %d/%d) — вывод обязан быть без записей",
			f.repo.createCalls, len(f.repo.rows), createBefore, rowsBefore)
	}
	if f.material.caseCalls != materialBefore || f.routes.routeCalls != routesBefore {
		t.Fatalf("политика читает назначение живыми чтениями: caseView=%d routeFacts=%d — слова политики не выводок назначения",
			f.material.caseCalls, f.routes.routeCalls)
	}
	// The paused and the unreadable-control worlds write nothing either.
	paused := policyFixture(t, policyPaused, nil)
	rowsBefore, createBefore = len(paused.repo.rows), paused.repo.createCalls
	if _, err := paused.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, paused.policyCaseID); err != nil {
		t.Fatalf("paused: вывод политики отказал: %v", err)
	}
	if paused.repo.createCalls != createBefore || len(paused.repo.rows) != rowsBefore {
		t.Fatalf("paused: политика пишет — вывод обязан быть без записей")
	}
	unreadable := policyFixture(t, port.ControlEffective{}, contextDeadlineErr{})
	rowsBefore, createBefore = len(unreadable.repo.rows), unreadable.repo.createCalls
	if _, err := unreadable.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, unreadable.policyCaseID); err != nil {
		t.Fatalf("нечитаемый контроль: вывод политики отказал: %v", err)
	}
	if unreadable.repo.createCalls != createBefore || len(unreadable.repo.rows) != rowsBefore {
		t.Fatalf("нечитаемый контроль: политика пишет — вывод обязан быть без записей")
	}
}

// contextDeadlineErr is the scripted control resolution error of the
// unreadable-control world (any error word serves).
type contextDeadlineErr struct{}

func (contextDeadlineErr) Error() string { return "route resolution failed" }

// TestRecoveryPolicyDerivationIdempotent pins the byte-stable
// idempotence: повторный запрос того же кейса без изменений мира
// отвечает ТОТ ЖЕ вывод (DeepEqual), чтения кейса/фактов ровно по
// одному на вызов, записей НОЛЬ.
func TestRecoveryPolicyDerivationIdempotent(t *testing.T) {
	f := policyFixture(t, policyAccept, nil)
	factsBase, caseBase := f.repo.factsCalls, f.repo.caseCalls
	first, err := f.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, f.policyCaseID)
	if err != nil {
		t.Fatalf("первый вывод отказал: %v", err)
	}
	second, err := f.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, f.policyCaseID)
	if err != nil {
		t.Fatalf("второй вывод отказал: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("повторный вывод политики не байт-стабилен — идемпотентность выводом нарушена")
	}
	if f.repo.caseCalls-caseBase != 2 || f.repo.factsCalls-factsBase != 2 {
		t.Fatalf("чтения: case=%d facts=%d сверх базовой линии — по одному на вызов", f.repo.caseCalls-caseBase, f.repo.factsCalls-factsBase)
	}
}

// TestRecoveryPolicyFullAllowedSet pins the full derivation over the
// unit world (accept control, readable definition with the declared
// CancelCompensationSem, the AG-4 nil adapter — the production fact of
// the fixture): показ условий/статус allowed, request_evidence allowed
// с подтверждением, prepare_pause_cancel allowed (семантика объявлена),
// offer_approved_options human_only «вариантов нет», human-only класс
// отдельно; статусное слово несёт AG-4 предложение (сверка не
// подключена — факт композиции, политику не блокирует).
func TestRecoveryPolicyFullAllowedSet(t *testing.T) {
	f := policyFixture(t, policyAccept, nil)
	policy, err := f.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, f.policyCaseID)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	if len(policy.Actions) != 5 || len(policy.HumanOnlyDecisions) != 2 {
		t.Fatalf("вывод: виды=%d human-only=%d, ожидалось 5/2", len(policy.Actions), len(policy.HumanOnlyDecisions))
	}
	if state := policyState(t, policy, domain.RecoveryActionShowRecordedConditions).State; state != domain.RecoveryActionStateAllowed {
		t.Fatalf("show_recorded_conditions state=%s, ожидался allowed", state)
	}
	status := policyState(t, policy, domain.RecoveryActionTruthfulStatusUpdate)
	if status.State != domain.RecoveryActionStateAllowed {
		t.Fatalf("truthful_status_update state=%s, ожидался allowed", status.State)
	}
	if !strings.Contains(status.StateText, domain.RecoveryActionAG4NotWiredText) {
		t.Fatalf("AG-4: слово «сверка не подключена» обязано ехать в статусном слове (адаптер фикстуры nil), text=%q", status.StateText)
	}
	if state := policyState(t, policy, domain.RecoveryActionRequestEvidence).State; state != domain.RecoveryActionStateAllowed {
		t.Fatalf("request_evidence state=%s, ожидался allowed", state)
	}
	pause := policyState(t, policy, domain.RecoveryActionPreparePauseCancel)
	if pause.State != domain.RecoveryActionStateAllowed {
		t.Fatalf("prepare_pause_cancel state=%s, ожидался allowed (семантика каталога объявлена)", pause.State)
	}
	if options := policyState(t, policy, domain.RecoveryActionOfferApprovedOptions); options.State != domain.RecoveryActionStateHumanOnly {
		t.Fatalf("offer_approved_options state=%s, ожидался human_only", options.State)
	}
	// The read order: the case read and the facts read ride the reused
	// reads (ONE call each over the fixture baseline — the creation
	// contour read the facts once during its own command), the creation
	// counter stays at ONE (the fixture's own creation).
	if f.repo.caseCalls != 1 || f.repo.factsCalls != 2 || f.repo.createCalls != 1 {
		t.Fatalf("чтения политики: case=%d facts=%d create=%d — вывод читает по одному сверх базовой линии, пишет ноль",
			f.repo.caseCalls, f.repo.factsCalls, f.repo.createCalls)
	}
}

// TestRecoveryPolicyPausedControl pins the paused scoped control over
// the unit world: материальные {request_evidence, prepare_pause_cancel}
// blocked ТОЧНЫМ словом scoped control (режим+причина), read-only
// остаются allowed, offer_approved_options остаётся human_only.
func TestRecoveryPolicyPausedControl(t *testing.T) {
	f := policyFixture(t, policyPaused, nil)
	policy, err := f.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, f.policyCaseID)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	for _, kind := range []string{domain.RecoveryActionRequestEvidence, domain.RecoveryActionPreparePauseCancel} {
		action := policyState(t, policy, kind)
		if action.State != domain.RecoveryActionStateBlocked {
			t.Fatalf("paused: вид «%s» state=%s, ожидался blocked", kind, action.State)
		}
		if !strings.Contains(action.StateText, "paused: connector paused оператором") {
			t.Fatalf("paused: вид «%s» обязан нести ТОЧНОЕ слово scoped control, text=%q", kind, action.StateText)
		}
	}
	for _, kind := range []string{domain.RecoveryActionShowRecordedConditions, domain.RecoveryActionTruthfulStatusUpdate} {
		if state := policyState(t, policy, kind).State; state != domain.RecoveryActionStateAllowed {
			t.Fatalf("paused: read-only вид «%s» state=%s, ожидался allowed (чтение остаётся)", kind, state)
		}
	}
	if state := policyState(t, policy, domain.RecoveryActionOfferApprovedOptions).State; state != domain.RecoveryActionStateHumanOnly {
		t.Fatalf("paused: offer_approved_options state=%s, ожидался human_only", state)
	}
}

// TestRecoveryPolicyControlUnreadable pins the control resolution
// error: материальные виды blocked честным словом нечитаемого контроля
// (fail-closed чтением), read-only остаются allowed, вывод НЕ
// отказывает (право ограничивается, политика 200 словами домена).
func TestRecoveryPolicyControlUnreadable(t *testing.T) {
	f := policyFixture(t, port.ControlEffective{}, contextDeadlineErr{})
	policy, err := f.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, f.policyCaseID)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	for _, kind := range []string{domain.RecoveryActionRequestEvidence, domain.RecoveryActionPreparePauseCancel} {
		action := policyState(t, policy, kind)
		if action.State != domain.RecoveryActionStateBlocked {
			t.Fatalf("нечитаемый контроль: вид «%s» state=%s, ожидался blocked", kind, action.State)
		}
		if !strings.Contains(action.StateText, "нечитаем") {
			t.Fatalf("нечитаемый контроль: вид «%s» text=%q — ожидалось честное слово", kind, action.StateText)
		}
	}
	for _, kind := range []string{domain.RecoveryActionShowRecordedConditions, domain.RecoveryActionTruthfulStatusUpdate} {
		if state := policyState(t, policy, kind).State; state != domain.RecoveryActionStateAllowed {
			t.Fatalf("нечитаемый контроль: read-only вид «%s» обязан остаться allowed", kind)
		}
	}
}

// TestRecoveryPolicyUnknownCase404 pins the honest 404 of the unknown
// case (the membership blindness BEFORE any material — AD-11): вывод
// отказывает 404-словом стора, факты НЕ читаются (кейс не найден —
// гейт материала не пройден).
func TestRecoveryPolicyUnknownCase404(t *testing.T) {
	f := policyFixture(t, policyAccept, nil)
	factsBefore, caseBefore := f.repo.factsCalls, f.repo.caseCalls
	_, err := f.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, id.MustParse("01994f64-6c00-7000-8000-00000000dead"))
	if err == nil || !strings.Contains(err.Error(), port.ErrRecoveryNotFound.Error()) {
		t.Fatalf("неизвестный кейс: ожидался 404-отказ стора, err=%v", err)
	}
	if f.repo.caseCalls != caseBefore+1 || f.repo.factsCalls != factsBefore {
		t.Fatalf("неизвестный кейс: case=%d facts=%d — кейс читается один раз, факты не читаются",
			f.repo.caseCalls, f.repo.factsCalls)
	}
}

// TestRecoveryPolicyGatesBeforeReads pins the gate ladder: не-участник
// (404) и участник без полномочия operations (403) получают отказ ДО
// чтения кейса (caseCalls=0) — слова BEFORE any material (прецедент
// карточки 3-8).
func TestRecoveryPolicyGatesBeforeReads(t *testing.T) {
	f := policyFixture(t, policyAccept, nil)
	nonMember := New(scopeIdentity{member: false}, acceptAllPacks{}, acceptAllCaps{}, f.defs).
		WithPreviews(f.previews, fakeControl{mode: policyAccept},
			f.material, liveMembers{world: &previewLiveFixture{}}, fakeEntitlement{status: "active"},
			fakeReadiness{stage: "admitted"}, fakeConnectors{state: "active"}, f.routes, &fakeIDs{}).
		WithRecovery(f.repo)
	if _, err := nonMember.RecoveryCasePolicy(context.Background(), testActor, testTenant, f.policyCaseID); err == nil {
		t.Fatalf("не-участник: ожидался 404-отказ гейта membership")
	}
	noScope := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{}}, acceptAllPacks{}, acceptAllCaps{}, f.defs).
		WithPreviews(f.previews, fakeControl{mode: policyAccept},
			f.material, liveMembers{world: &previewLiveFixture{}}, fakeEntitlement{status: "active"},
			fakeReadiness{stage: "admitted"}, fakeConnectors{state: "active"}, f.routes, &fakeIDs{}).
		WithRecovery(f.repo)
	if _, err := noScope.RecoveryCasePolicy(context.Background(), testActor, testTenant, f.policyCaseID); err == nil {
		t.Fatalf("участник без полномочия: ожидался 403-отказ гейта скоупа")
	}
	if f.repo.caseCalls != 0 {
		t.Fatalf("гейты обязаны стоять ДО чтения кейса: case=%d", f.repo.caseCalls)
	}
}

// TestRecoveryPolicyFactsMissFallsBackToBundlePins pins the honest
// fallback of the facts miss: ошибка чтения фактов НЕ отказывает вывод
// (замороженные пины bundle — те же самые пины, операция immutable) —
// политика выводится по тем же словам каталога.
func TestRecoveryPolicyFactsMissFallsBackToBundlePins(t *testing.T) {
	f := policyFixture(t, policyAccept, nil)
	f.repo.factsErr = contextDeadlineErr{}
	policy, err := f.policyService.RecoveryCasePolicy(context.Background(), testActor, testTenant, f.policyCaseID)
	if err != nil {
		t.Fatalf("промах фактов: вывод политики отказал: %v", err)
	}
	pause := policyState(t, policy, domain.RecoveryActionPreparePauseCancel)
	if pause.State != domain.RecoveryActionStateAllowed {
		t.Fatalf("промах фактов: prepare_pause_cancel state=%s — пины bundle обязаны резолвить каталог теми же словами", pause.State)
	}
}

// TestRecoveryPolicyLocalityProbe pins the LOCALITY of the policy path
// (the reflect probe — the 3-8 command precedent): сигнатура вывода,
// факты домена и контракт репозитория НЕ несут НИ ОДНОГО
// провайдер-адаптера (мутация, сверка, owning-system, dry-run) — сеть
// остаётся в контурах dispatch 3-6/reconcile 3-7, путь политики локален
// по типу.
func TestRecoveryPolicyLocalityProbe(t *testing.T) {
	providerTypes := []reflect.Type{
		reflect.TypeOf((*port.MutationAdapter)(nil)).Elem(),
		reflect.TypeOf((*port.ReconciliationAdapter)(nil)).Elem(),
		reflect.TypeOf((*port.OwningSystemReader)(nil)).Elem(),
		reflect.TypeOf((*port.DryRunner)(nil)).Elem(),
	}
	method, found := reflect.TypeOf((*Service)(nil)).MethodByName("RecoveryCasePolicy")
	if !found {
		t.Fatalf("метод RecoveryCasePolicy отсутствует на сервисе")
	}
	signature := method.Type
	for i := 0; i < signature.NumIn(); i++ {
		arg := signature.In(i)
		for _, provider := range providerTypes {
			if arg == provider || (arg.Kind() == reflect.Ptr && arg.Elem() == provider) {
				t.Fatalf("сигнатура RecoveryCasePolicy несёт провайдер-адаптер %v — путь обязан быть локален", provider)
			}
		}
	}
	facts := reflect.TypeOf(domain.RecoveryPolicyFacts{})
	for i := 0; i < facts.NumField(); i++ {
		field := facts.Field(i).Type
		for _, provider := range providerTypes {
			if field == provider || (field.Kind() == reflect.Interface && field.Implements(provider)) {
				t.Fatalf("RecoveryPolicyFacts несёт провайдер-адаптер %v — факты обязаны быть локальными", provider)
			}
		}
	}
	// The store contract of the policy: the repository rides the FIVE
	// reused methods ONLY — the policy adds NO store method (the reads
	// are reused as they stand; a new method would be a contract drift).
	repo := reflect.TypeOf((*port.RecoveryRepository)(nil)).Elem()
	if got := repo.NumMethod(); got != 5 {
		t.Fatalf("контракт RecoveryRepository несёт %d методов, ожидалось ровно 5 reused (ReadRecoveryOperationFacts/ReadRecoveryCaseByOperation/ReadRecoveryCase/ReadRecoveryCases/CreateRecoveryCase) — политика НЕ добавляет методов стора", got)
	}
}
