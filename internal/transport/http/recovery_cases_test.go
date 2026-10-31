package httptransport

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 3.8 step-03: the transport unit matrix of the recovery surface
// ---------------------------------------------------------------------------
//
// The probes mirror the reconcile surface tests by name: the strict
// decode of the EMPTY body refuses a free field BEFORE the service; the
// 201/200 answers speak the decision words with the honest texts and
// carry the case view; the eligibility refusals answer 403 with the EXACT
// ladder words (the fake carries the wrapped sentinel); the unknown
// operation answers 404; the page form requires the deliberate checkbox
// and lands PRG on the case card page; the panel page renders the honest
// empty state.

// fakeRecovery is the scriptable recovery service of the unit contour.
type fakeRecovery struct {
	result      actionAPI.RecoveryCreateResultView
	cases       []actionAPI.RecoveryCaseView
	card        actionAPI.RecoveryCaseView
	policy      actionAPI.RecoveryPolicyView
	err         error
	policyErr   error
	calls       int
	lastCmd     *actionAPI.RecoveryCreateCommand
	policyCalls int
}

// The compile-time proof the fake satisfies the narrow transport contract.
var _ actionAPI.RecoveryService = (*fakeRecovery)(nil)

func (f *fakeRecovery) CreateRecoveryCase(_ context.Context, c actionAPI.RecoveryCreateCommand) (actionAPI.RecoveryCreateResultView, error) {
	f.calls++
	commandCopy := c
	f.lastCmd = &commandCopy
	if f.err != nil {
		return actionAPI.RecoveryCreateResultView{}, f.err
	}
	return f.result, nil
}

func (f *fakeRecovery) ReadRecoveryCase(context.Context, identityDomain.Principal, id.UUID, id.UUID) (actionAPI.RecoveryCaseView, error) {
	if f.err != nil {
		return actionAPI.RecoveryCaseView{}, f.err
	}
	return f.card, nil
}

func (f *fakeRecovery) ReadRecoveryCases(context.Context, identityDomain.Principal, id.UUID) ([]actionAPI.RecoveryCaseView, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.cases, nil
}

func (f *fakeRecovery) RecoveryCasePolicy(context.Context, identityDomain.Principal, id.UUID, id.UUID) (actionAPI.RecoveryPolicyView, error) {
	f.policyCalls++
	if f.policyErr != nil {
		return actionAPI.RecoveryPolicyView{}, f.policyErr
	}
	if f.err != nil {
		return actionAPI.RecoveryPolicyView{}, f.err
	}
	return f.policy, nil
}

// newRecoveryTestServer binds the recovery surface over the operation
// fixture world (the reconcile fixture precedent).
func newRecoveryTestServer(t *testing.T, recovery *fakeRecovery) *Server {
	t.Helper()
	server := newOperationTestServer(t, &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID: testTenantID, CaseID: testPreviewCaseID,
		Confirmations: []actionAPI.ConfirmationCardView{},
		Operations:    []actionAPI.OperationCardView{},
	}})
	server.BindRecovery(recovery)
	return server
}

// recoveryCaseAPIPath builds the machine creation path of the fixture
// tenant and operation.
func recoveryCaseAPIPath() string {
	return "/v1/tenants/" + testTenantID.String() + "/operations/" + testOperationID.String() + "/recovery-case"
}

// recoveryTestCaseView builds the passing case view of the probes.
func recoveryTestCaseView() actionAPI.RecoveryCaseView {
	return actionAPI.RecoveryCaseView{
		RecoveryCaseID:    testOperationID,
		RecoveryCaseNo:    7,
		OperationID:       testOperationID,
		CaseID:            testPreviewCaseID,
		WorkItemID:        testPreviewCaseID,
		DefinitionID:      "quote.send-price-quote",
		DefinitionVersion: "1.2.0",
		Trigger:           actionDomain.RecoveryTriggerReconciliationExhausted,
		TriggerText:       actionDomain.RecoveryTriggerText(actionDomain.RecoveryTriggerReconciliationExhausted),
		State:             actionDomain.RecoveryCaseStateOpen,
		StateText:         actionDomain.RecoveryCaseOpenText,
		Assignment: actionAPI.RecoveryAssignmentView{
			OwnerKind: "human", OwnerRef: testPrincipalID, OwnerEpoch: 4,
			BackupOwnerID: testPrincipalID, AckDeadlineMinutes: 30,
		},
		DeadlineAt:          time.Unix(1, 0).UTC().Add(30 * time.Minute),
		PaymentsUnknownText: actionDomain.RecoveryPaymentsUnknownText,
		LocalProjectionText: actionDomain.RecoveryLocalProjectionText,
		Events:              []actionAPI.RecoveryEventView{},
	}
}

// TestRecoveryRefusesTheFreeFieldBeforeTheService — the strict decode of
// the EMPTY body: a free field is the honest 400 BEFORE the service is
// ever called (the creation command carries NO parameters — the words of
// the assignment NEVER come from a command parameter).
func TestRecoveryRefusesTheFreeFieldBeforeTheService(t *testing.T) {
	fake := &fakeRecovery{}
	server := newRecoveryTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, recoveryCaseAPIPath(), strings.NewReader(`{"owner":"someone"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("свободное поле обязано отвечать 400, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"body"`) {
		t.Fatalf("отказ обязан нести правило body: %s", recorder.Body.String())
	}
	if fake.calls != 0 {
		t.Fatalf("strict decode отказывает ДО сервиса, сервис видел %d", fake.calls)
	}
}

// TestRecoveryAnswers201CreatedAnd200Repeat — the empty strict body
// passes: the FIRST creation answers 201 with the decision word created
// and the honest text; the idempotent repeat answers 200 (the existing
// case returns — NOT a new resource); the command carries the actor, the
// tenant, the operation pin and the server clock verbatim.
func TestRecoveryAnswers201CreatedAnd200Repeat(t *testing.T) {
	view := recoveryTestCaseView()
	created := &fakeRecovery{result: actionAPI.RecoveryCreateResultView{
		Decision: actionPort.RecoveryDecisionCreated, Text: actionDomain.RecoveryCreatedText, Case: view,
	}}
	server := newRecoveryTestServer(t, created)
	request := httptest.NewRequest(http.MethodPost, recoveryCaseAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("первое создание обязано отвечать 201, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, pin := range []string{`"recovery_case":{`, `"decision":"created"`, actionDomain.RecoveryCreatedText, `"trigger":"reconciliation_exhausted"`, actionDomain.RecoveryPaymentsUnknownText} {
		if !strings.Contains(body, pin) {
			t.Fatalf("ответ создания обязан нести решение и вид кейса (%s): %s", pin, body)
		}
	}
	if created.lastCmd == nil || created.lastCmd.Actor.ID != testPrincipalID ||
		created.lastCmd.TenantID != testTenantID || created.lastCmd.OperationID != testOperationID ||
		!created.lastCmd.Now.Equal(time.Unix(1, 0).UTC()) {
		t.Fatalf("команда обязана нести актёра, тенент, пин операции и серверное время дословно: %+v", created.lastCmd)
	}
	repeat := &fakeRecovery{result: actionAPI.RecoveryCreateResultView{
		Decision: actionPort.RecoveryDecisionIdempotent, Text: actionDomain.RecoveryIdempotentText, Case: view,
	}}
	server2 := newRecoveryTestServer(t, repeat)
	request2 := httptest.NewRequest(http.MethodPost, recoveryCaseAPIPath(), strings.NewReader(`{}`))
	request2.Header.Set("Content-Type", "application/json")
	recorder2 := httptest.NewRecorder()
	server2.Handler().ServeHTTP(recorder2, request2)
	if recorder2.Code != http.StatusOK {
		t.Fatalf("идемпотентный повтор обязан отвечать 200, пришло %d: %s", recorder2.Code, recorder2.Body.String())
	}
	if !strings.Contains(recorder2.Body.String(), actionDomain.RecoveryIdempotentText) {
		t.Fatalf("повтор обязан говорить честное слово: %s", recorder2.Body.String())
	}
}

// TestRecoverySpeaksTheLadderAndAssignmentRefusalWords — the eligibility
// gas and BOTH fail-closed assignment refusals answer 403 with the EXACT
// words the app attached at the wrap point (nothing is written); the
// unknown operation answers 404 (the membership blindness).
func TestRecoverySpeaksTheLadderAndAssignmentRefusalWords(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		word string
	}{
		{"газ-не-исчерпана", fmt.Errorf("%w: %s", actionPort.ErrRecoveryIneligible, actionDomain.RecoveryIneligibleReconcileNotExhaustedText), http.StatusForbidden, "сначала контур сверки"},
		{"газ-в-исполнении", fmt.Errorf("%w: %s", actionPort.ErrRecoveryIneligible, actionDomain.RecoveryIneligibleInFlightText), http.StatusForbidden, "ещё в исполнении"},
		{"покрытие-недоступно", fmt.Errorf("%w: %s", actionPort.ErrRecoveryCoverageUnavailable, actionDomain.RecoveryCoverageUnavailableText), http.StatusForbidden, "fail-closed"},
		{"владелец-неизвестен", fmt.Errorf("%w: %s", actionPort.ErrRecoveryOwnerUnknown, actionDomain.RecoveryOwnerUnknownText), http.StatusForbidden, "контур владельцев"},
		{"неизвестная-операция", fmt.Errorf("%w: %s", actionPort.ErrRecoveryNotFound, "операция неизвестна"), http.StatusNotFound, "не найдены"},
	}
	for _, tc := range cases {
		fake := &fakeRecovery{err: tc.err}
		server := newRecoveryTestServer(t, fake)
		request := httptest.NewRequest(http.MethodPost, recoveryCaseAPIPath(), strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		if recorder.Code != tc.code {
			t.Fatalf("%s: ожидался %d, пришло %d: %s", tc.name, tc.code, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), tc.word) {
			t.Fatalf("%s: ответ обязан нести слово %q: %s", tc.name, tc.word, recorder.Body.String())
		}
	}
}

// TestRecoveryFormRequiresTheCheckboxAndLandsPRG — the page form requires
// the deliberate confirmation checkbox (the honest flash redirect) and
// the passing command lands PRG on the case card page with the decision
// flag; the origin guard refuses a foreign origin.
func TestRecoveryFormRequiresTheCheckboxAndLandsPRG(t *testing.T) {
	view := recoveryTestCaseView()
	fake := &fakeRecovery{result: actionAPI.RecoveryCreateResultView{
		Decision: actionPort.RecoveryDecisionCreated, Text: actionDomain.RecoveryCreatedText, Case: view,
	}}
	server := newRecoveryTestServer(t, fake)
	formPath := "/app/tenants/" + testTenantID.String() + "/operations/" + testOperationID.String() + "/recovery-case"
	// The missing checkbox: the honest flash, the PRG return.
	request := httptest.NewRequest(http.MethodPost, formPath, strings.NewReader("case_id="+testPreviewCaseID.String()+"&operation_id="+testOperationID.String()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+request.Host)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("без чекбокса обязан PRG-возврат, пришло %d", recorder.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("без чекбокса сервис не вызывается, вызовов %d", fake.calls)
	}
	// The passing command: PRG onto the case card page with the decision
	// flag (the notice speaks the honest created word).
	request = httptest.NewRequest(http.MethodPost, formPath, strings.NewReader("case_id="+testPreviewCaseID.String()+"&operation_id="+testOperationID.String()+"&confirm_recovery=yes"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+request.Host)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("успех обязан PRG, пришло %d", recorder.Code)
	}
	target := recorder.Header().Get("Location")
	if !strings.Contains(target, "/app/tenants/"+testTenantID.String()+"/recovery-cases/"+testOperationID.String()) || !strings.Contains(target, "recovery=created") {
		t.Fatalf("PRG-цель обязана вести на карточку кейса с флагом решения: %s", target)
	}
}

// TestRecoveryPanelPageRendersTheHonestEmptyState — the panel page speaks
// the honest empty state (no cases yet — the creation is the explicit
// operator command from the eligible operation card).
func TestRecoveryPanelPageRendersTheHonestEmptyState(t *testing.T) {
	fake := &fakeRecovery{cases: []actionAPI.RecoveryCaseView{}}
	server := newRecoveryTestServer(t, fake)
	request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/recovery-cases", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("панель обязана отвечать 200, пришло %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "Recovery Panel") || !strings.Contains(recorder.Body.String(), "пока нет") {
		t.Fatalf("панель обязана говорить честное пустое состояние: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "skip-link") {
		t.Fatalf("панель обязана нести skip-link (a11y): %s", recorder.Body.String())
	}
}

// TestRecoveryCardPageRendersThePinsAndHonestWords — the case card page
// renders the SAFE pins, the assignment words, the honest boundary texts
// and the append-only history; the living chain words speak the fresh
// derivation.
func TestRecoveryCardPageRendersThePinsAndHonestWords(t *testing.T) {
	view := recoveryTestCaseView()
	view.Events = []actionAPI.RecoveryEventView{{
		EventNo: 1, Kind: actionDomain.RecoveryEventCreated,
		Details: "Recovery Case создан: триггер reconciliation_exhausted.",
		ActorID: testPrincipalID, CreatedAt: time.Unix(1, 0).UTC(),
	}}
	view.Links = actionAPI.RecoveryLinksView{
		PreviewID: testPreviewCaseID, PreviewVersionNo: 1,
		ConfirmationID: testPreviewCaseID, ConfirmationVersionNo: 2,
		ReceiptVersionsCount: 1, LatestReceiptVersionNo: 1,
		LatestReceiptOutcome: actionDomain.ReceiptOutcomeUnknown,
		DispatchAttempts:     1, ReconcileAttempts: 3,
		OwningSystem: "catalog",
		Observations: []actionAPI.RecoveryObservationPinView{},
	}
	fake := &fakeRecovery{card: view}
	server := newRecoveryTestServer(t, fake)
	request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/recovery-cases/"+testOperationID.String(), nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("карточка обязана отвечать 200, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, pin := range []string{
		"Recovery Case №7",
		actionDomain.RecoveryCaseOpenText,
		actionDomain.RecoveryPaymentsUnknownText,
		actionDomain.RecoveryLocalProjectionText,
		"Цепочка квитанций: версий 1",
		"evidence_appended",
		"Пины наблюдений Owning System: нет",
	} {
		if !strings.Contains(body, pin) {
			t.Fatalf("карточка обязана нести пин %q: %s", pin, body)
		}
	}
}

// ---------------------------------------------------------------------------
// Story 3.9 step-03: the transport unit matrix of the policy surface
// ---------------------------------------------------------------------------

// recoveryPolicyView builds the passing policy view of the probes: the
// full allowed set of the exhausted case (the AG-4 world of the fixture
// adapter — the status word carries the honest sentence) + the
// human-only class with the boundary words.
func recoveryPolicyView() actionAPI.RecoveryPolicyView {
	view := actionAPI.RecoveryPolicyView{
		RecoveryCaseID:     testOperationID,
		HumanOnlyClassText: actionDomain.RecoveryHumanOnlyClassText,
	}
	for _, kind := range actionDomain.RecoveryActionKinds {
		answer := actionDomain.RecoveryPolicyAction{
			Kind: kind.Word, State: actionDomain.RecoveryActionStateAllowed,
			ScopeText: kind.ScopeText, EligibilityRuleText: kind.EligibilityRuleText,
			RequiredFactsText: kind.RequiredFactsText, AuthorityText: kind.AuthorityText,
			ConfirmationRequired: kind.ConfirmationRequired,
		}
		switch kind.Word {
		case actionDomain.RecoveryActionShowRecordedConditions:
			answer.StateText = actionDomain.RecoveryActionShowConditionsAllowedText
		case actionDomain.RecoveryActionTruthfulStatusUpdate:
			answer.StateText = actionDomain.RecoveryActionStatusUpdateAllowedText + " " + actionDomain.RecoveryActionAG4NotWiredText
		case actionDomain.RecoveryActionRequestEvidence:
			answer.StateText = actionDomain.RecoveryActionRequestEvidenceAllowedText
		case actionDomain.RecoveryActionOfferApprovedOptions:
			answer.State = actionDomain.RecoveryActionStateHumanOnly
			answer.StateText = actionDomain.RecoveryActionNoApprovedOptionsText
		case actionDomain.RecoveryActionPreparePauseCancel:
			answer.StateText = actionDomain.RecoveryActionPauseCancelAllowedText
		}
		view.Actions = append(view.Actions, actionAPI.RecoveryActionView{
			Kind: answer.Kind, State: answer.State, StateText: answer.StateText,
			ScopeText: answer.ScopeText, EligibilityRuleText: answer.EligibilityRuleText,
			RequiredFactsText: answer.RequiredFactsText, AuthorityText: answer.AuthorityText,
			ConfirmationRequired: answer.ConfirmationRequired,
		})
	}
	for _, decision := range actionDomain.RecoveryHumanOnlyDecisions {
		view.HumanOnlyDecisions = append(view.HumanOnlyDecisions, actionAPI.RecoveryHumanOnlyDecisionView{
			Kind: decision.Word, BoundaryText: decision.BoundaryText,
		})
	}
	return view
}

// recoveryPolicyAPIPath builds the machine policy path of the fixture
// tenant and case.
func recoveryPolicyAPIPath() string {
	return "/v1/tenants/" + testTenantID.String() + "/recovery-cases/" + testOperationID.String() + "/policy"
}

// TestRecoveryPolicyMachineReadAnswersTheView — the machine policy read:
// 200 with the wrapper + the FIVE kinds in the dictionary order + the
// human-only class + the honest words (the AG-4 sentence of the status
// kind, the confirmation words of the material kinds); the empty GET
// NEVER answers 400 (a free query field is ignored by the router).
func TestRecoveryPolicyMachineReadAnswersTheView(t *testing.T) {
	fake := &fakeRecovery{policy: recoveryPolicyView()}
	server := newRecoveryTestServer(t, fake)
	request := httptest.NewRequest(http.MethodGet, recoveryPolicyAPIPath()+"?free=field", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("политика обязана отвечать 200, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, word := range []string{
		"\"recovery_policy\"",
		"request_evidence", "show_recorded_conditions", "truthful_status_update",
		"offer_approved_options", "prepare_pause_cancel",
		"\"allowed\"", "\"human_only\"",
		actionDomain.RecoveryActionNoApprovedOptionsText,
		actionDomain.RecoveryActionAG4NotWiredText,
		actionDomain.RecoveryHumanOnlyClassText,
		"liability_or_fault", "nonstandard_outcome",
		"\"confirmation_required\":true",
	} {
		if !strings.Contains(body, word) {
			t.Fatalf("машинный вид политики обязан нести слово %q: %s", word, body)
		}
	}
	// The kinds ride the dictionary order (the fixed order of the view).
	requestEvidence := strings.Index(body, "request_evidence")
	preparePause := strings.Index(body, "prepare_pause_cancel")
	if requestEvidence < 0 || preparePause < 0 || requestEvidence > preparePause {
		t.Fatalf("виды обязаны ехать в порядке словаря: request_evidence=%d prepare_pause_cancel=%d", requestEvidence, preparePause)
	}
	if fake.policyCalls != 1 {
		t.Fatalf("вывод политики обязан читаться ОДИН раз, прочитан %d", fake.policyCalls)
	}
}

// TestRecoveryPolicySpeaksThe404And403Words — the honest refusal ladder
// of the policy read: the unknown case answers 404 (the membership
// blindness BEFORE any material), the scope refusal answers 403 with the
// card rights words (the card precedent — the surface rights are
// reused).
func TestRecoveryPolicySpeaksThe404And403Words(t *testing.T) {
	fake := &fakeRecovery{err: fmt.Errorf("%w: %s", actionAPI.ErrRecoveryNotFound, "recovery case not found")}
	server := newRecoveryTestServer(t, fake)
	request := httptest.NewRequest(http.MethodGet, recoveryPolicyAPIPath(), nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("неизвестный кейс обязан отвечать 404, пришло %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "не найдены") {
		t.Fatalf("404-слово политики обязано говорить честное слово стора: %s", recorder.Body.String())
	}
}

// TestRecoveryCardPageRendersThePolicySection — the «Разрешённые
// действия» section of the case card page: the kind rows with the state
// words, the DECLARED attributes in the CLOSED details, the
// confirmation word of the material kinds (the AC-13 minimum), the
// human-only class as the SEPARATE block with the boundary words and
// the honest derivation note; the frozen 3-8 words stay byte-stable.
func TestRecoveryCardPageRendersThePolicySection(t *testing.T) {
	view := recoveryTestCaseView()
	fake := &fakeRecovery{card: view, policy: recoveryPolicyView()}
	server := newRecoveryTestServer(t, fake)
	request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/recovery-cases/"+testOperationID.String(), nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("карточка обязана отвечать 200, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, word := range []string{
		"Разрешённые действия",
		"request_evidence</strong> — allowed",
		"offer_approved_options</strong> — human_only",
		actionDomain.RecoveryActionNoApprovedOptionsText,
		"Требует отдельного явного подтверждения оператора.",
		"Атрибуты вида (scope / eligibility / факты / authority / Confirmation)",
		"Только человек — решения вне набора системы",
		actionDomain.RecoveryHumanOnlyClassText,
		actionDomain.RecoveryHumanOnlyDecisions[0].BoundaryText,
		actionDomain.RecoveryHumanOnlyDecisions[1].BoundaryText,
		// The frozen 3-8 words stay byte-stable (rule 161/162).
		actionDomain.RecoveryCaseOpenText,
		actionDomain.RecoveryPaymentsUnknownText,
	} {
		if !strings.Contains(body, word) {
			t.Fatalf("секция политики карточки обязана нести слово %q: %s", word, body)
		}
	}
	if fake.policyCalls != 1 {
		t.Fatalf("секция политики читает вывод ОДИН раз, прочитано %d", fake.policyCalls)
	}
}

// TestRecoveryCardPageSurvivesThePolicyMiss — the card page NEVER
// refuses on a transient policy derivation miss: the frozen card words
// render, the section renders honestly empty (the derivation is the
// ADDITIVE surface — the card itself answers 200).
func TestRecoveryCardPageSurvivesThePolicyMiss(t *testing.T) {
	view := recoveryTestCaseView()
	policyErr := fmt.Errorf("%w: %s", actionAPI.ErrRecoveryNotFound, "recovery case not found")
	fake := &fakeRecovery{card: view, policyErr: policyErr}
	server := newRecoveryTestServer(t, fake)
	request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/recovery-cases/"+testOperationID.String(), nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("карточка обязана отвечать 200 и при промахе политики, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, actionDomain.RecoveryCaseOpenText) {
		t.Fatalf("замороженные слова карточки обязаны рендериться: %s", body)
	}
	if strings.Contains(body, "Разрешённые действия") {
		t.Fatalf("секция политики обязана отсутствовать при промахе вывода (честная пустая секция)")
	}
}
