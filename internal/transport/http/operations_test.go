package httptransport

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 3.5 transport tests: the fake serves the honest refusals of the
// ordered gateway (the gate ORDER and the counters are the app's unit
// truth — these tests pin the MAPPING of every refusal onto its code and
// the wire/page shapes: the strict decode of the free «confidence» field,
// the verbatim command capture, the honest status ladder of the machine
// surface — the 201 of the first authorization, the 200 of the replay and
// the cross-case reference, the 409 of the exact conflict and ladder
// words, the 403 of the step-up and the fail-closed fresh reads, the 404
// of the dead pins — the honest empty view, the visible exceptions of the
// cards, the confirm-form prefill of the 3-4 primary transition, the
// PRG+focus notice and the form origin/checkbox disciplines).
// ---------------------------------------------------------------------------

var (
	testConfirmationID = id.MustParse("01994f64-6c00-7000-8000-00000000f101")
	testOperationID    = id.MustParse("01994f64-6c00-7000-8000-00000000f102")
)

// fakeAuthorization serves the confirmation→authorization surface with
// the scripted answers and the captured commands (the fakeActionPreviews
// precedent — the counters pin the transport's own gate order: the strict
// decode refuses BEFORE the service).
type fakeAuthorization struct {
	view         actionAPI.AuthorizationView
	viewErr      error
	confirm      actionAPI.ConfirmationVersionOutcome
	confirmErr   error
	revoke       actionAPI.ConfirmationRevocationOutcome
	revokeErr    error
	authorize    actionAPI.AuthorizationOutcome
	authorizeErr error

	confirmCalls   int
	revokeCalls    int
	authorizeCalls int
	lastConfirm    *actionAPI.ConfirmPreviewVersionCommand
	lastRevoke     *actionAPI.RevokeConfirmationCommand
	lastAuthorize  *actionAPI.AuthorizeOperationCommand
}

// The compile-time proof the fake satisfies the narrow transport contract.
var _ actionAPI.AuthorizationService = (*fakeAuthorization)(nil)

func (f *fakeAuthorization) ConfirmPreviewVersion(_ context.Context, c actionAPI.ConfirmPreviewVersionCommand) (actionAPI.ConfirmationVersionOutcome, error) {
	f.confirmCalls++
	commandCopy := c
	f.lastConfirm = &commandCopy
	if f.confirmErr != nil {
		return actionAPI.ConfirmationVersionOutcome{}, f.confirmErr
	}
	return f.confirm, nil
}

func (f *fakeAuthorization) RevokeConfirmation(_ context.Context, c actionAPI.RevokeConfirmationCommand) (actionAPI.ConfirmationRevocationOutcome, error) {
	f.revokeCalls++
	commandCopy := c
	f.lastRevoke = &commandCopy
	if f.revokeErr != nil {
		return actionAPI.ConfirmationRevocationOutcome{}, f.revokeErr
	}
	return f.revoke, nil
}

func (f *fakeAuthorization) AuthorizeOperation(_ context.Context, c actionAPI.AuthorizeOperationCommand) (actionAPI.AuthorizationOutcome, error) {
	f.authorizeCalls++
	commandCopy := c
	f.lastAuthorize = &commandCopy
	if f.authorizeErr != nil {
		return actionAPI.AuthorizationOutcome{}, f.authorizeErr
	}
	return f.authorize, nil
}

func (f *fakeAuthorization) ReadAuthorizationView(_ context.Context, _ domain.Principal, _, _ id.UUID, _ time.Time) (actionAPI.AuthorizationView, error) {
	if f.viewErr != nil {
		return actionAPI.AuthorizationView{}, f.viewErr
	}
	return f.view, nil
}

// newOperationTestServer binds the fake through the production
// BindAuthorization — the narrow interface is type-asserted exactly the
// way the real wiring does it.
func newOperationTestServer(t *testing.T, fake *fakeAuthorization) *Server {
	t.Helper()
	identity := api.Service(fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) {
			return domain.Workspace{}, nil
		},
	})
	server, err := New(identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, StepUpVerified: true}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindCases(&fakeCases{})
	server.BindAuthorization(fake)
	return server
}

// operationsAPIRoot builds the machine path prefix of the fixture anchors.
func operationsAPIRoot() string {
	return "/v1/tenants/" + testTenantID.String() + "/cases/" + testPreviewCaseID.String()
}

// operationsPageRoot builds the page path prefix of the fixture anchors.
func operationsPageRoot() string {
	return "/app/tenants/" + testTenantID.String() + "/cases/" + testPreviewCaseID.String()
}

// fixtureConfirmationCard builds one full confirmation version card: the
// valid state with its fixed text, the consumption words and the VISIBLE
// conflict exception of a rejected authorization attempt (AC-8 — the
// honest history the owner reads).
func fixtureConfirmationCard() actionAPI.ConfirmationCardView {
	return actionAPI.ConfirmationCardView{
		ConfirmationID:    testConfirmationID,
		VersionNo:         1,
		DefinitionID:      "notify.manual-only",
		DefinitionVersion: "1.0.0",
		CaseID:            testPreviewCaseID,
		WorkItemID:        testPreviewWorkID,
		TargetKind:        actionDomain.TargetCatalogItem,
		TargetRef:         testPreviewID,
		OwnerKind:         actionDomain.OwnerHuman,
		OwnerRef:          testPrincipalID,
		PreviewID:         testPreviewID,
		PreviewVersionNo:  1,
		State:             actionDomain.ConfirmationValid,
		StateText:         actionDomain.ConfirmationStateText(actionDomain.ConfirmationValid),
		ValidUntil:        time.Unix(3601, 0).UTC(),
		ConsumptionText:   "Потреблено 0 из 1 (single_use).",
		Events: []actionAPI.ConfirmationEventView{{
			VersionNo: 1, EventNo: 2,
			Kind:      actionDomain.ConfirmationEventAuthorizationConflict,
			Reason:    "конфликт идемпотентности: другой материал под тем же ключом",
			Details:   "попытка отвергнута без потребления",
			CreatedAt: time.Unix(5, 0).UTC(),
		}},
		CreatedAt: time.Unix(2, 0).UTC(),
	}
}

// fixtureOperationCard builds one full immutable operation card: the
// six-word life state with the structured queue fact (3-6 step-03: the
// spoken pair-word matured into the life words), the two key identities,
// the append-only ledger, the progressive attempt with the recorded
// verdict, the Action Receipt version with the identifier readback and
// the fixed honest texts of the slice boundaries (AC-11 — executed never
// exists here).
func fixtureOperationCard() actionAPI.OperationCardView {
	return actionAPI.OperationCardView{
		OperationID:           testOperationID,
		DefinitionID:          "notify.manual-only",
		DefinitionVersion:     "1.0.0",
		PreviewID:             testPreviewID,
		PreviewVersionNo:      1,
		ConfirmationID:        testConfirmationID,
		ConfirmationVersionNo: 1,
		CaseID:                testPreviewCaseID,
		WorkItemID:            testPreviewWorkID,
		TargetKind:            actionDomain.TargetCatalogItem,
		TargetRef:             testPreviewID,
		OwnerKind:             actionDomain.OwnerHuman,
		OwnerRef:              testPrincipalID,
		OwnerEpoch:            1,
		ReleaseDecisionNo:     1,
		ProfileVersion:        1,
		Partition:             actionDomain.PartitionHumanOnly,
		IdempotencyKey:        "order-2026.10.20.001",
		BusinessIntentKey:     "64hex-intent",
		MaterialDigest:        "64hex-digest",
		State:                 actionDomain.OperationAuthorized,
		StateText:             actionDomain.OperationAuthorizedText + " " + actionDomain.OperationQueuedText,
		Queued:                true,
		StateAt:               time.Unix(6, 0).UTC(),
		OwningSystem:          "telephony",
		NextStepText:          actionDomain.OperationNextStepText(actionDomain.OperationAuthorized),
		ExternalIDAbsentText:  actionDomain.OperationExternalIDAbsentText,
		DispatchAbsentText:    actionDomain.OperationDispatchAbsentText,
		Ledger: []actionAPI.OperationLedgerEntryView{{
			EntryNo: 1, Kind: actionDomain.LedgerEntryAuthorized,
			Details:   "потребление single_use, резервация намерения, операция, ledger и outbox одной транзакцией",
			CreatedAt: time.Unix(6, 0).UTC(),
		}},
		Attempts: []actionAPI.DispatchAttemptView{{
			AttemptNo:         1,
			LeaseCreatedAt:    time.Unix(7, 0).UTC(),
			LeaseExpiresAt:    time.Unix(37, 0).UTC(),
			RequestDigest:     "64hex-request-digest-attempt-1",
			Outcome:           actionDomain.MutationOutcomeCommitted,
			OutcomeRecorded:   true,
			OutcomeText:       actionDomain.OperationCommittedText,
			CorrelationID:     "syn-0123456789abcdef",
			GuaranteeRecorded: true,
			OutcomeCreatedAt:  time.Unix(8, 0).UTC(),
		}},
		Receipts: []actionAPI.ActionReceiptView{{
			ReceiptVersionNo:      1,
			Outcome:               actionDomain.ReceiptOutcomeCommitted,
			OutcomeText:           actionDomain.OperationCommittedText,
			CaseID:                testPreviewCaseID,
			WorkItemID:            testPreviewWorkID,
			PreviewID:             testPreviewID,
			PreviewVersionNo:      1,
			ConfirmationID:        testConfirmationID,
			ConfirmationVersionNo: 1,
			DefinitionID:          "notify.manual-only",
			DefinitionVersion:     "1.0.0",
			ExternalObjectID:      "msg-42",
			ExternalObjectVersion: "3",
			Details:               "квитанция v1: терминальный успех с полным тройником evidence",
			CreatedAt:             time.Unix(8, 0).UTC(),
		}},
		CreatedAt: time.Unix(6, 0).UTC(),
	}
}

// TestOperationsViewReadServesTheWrapperAndTheHonestEmptyState — the
// machine read answers the documented authorizations wrapper with the
// honest empty cards (never a null) and the honest empty-state words of
// the domain.
func TestOperationsViewReadServesTheWrapperAndTheHonestEmptyState(t *testing.T) {
	emptyText := "В этом кейсе ещё нет подтверждений и операций — подтверждение строится точной версией preview со страницы Action Preview."
	fake := &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID: testTenantID, CaseID: testPreviewCaseID, EmptyText: emptyText,
		Confirmations: []actionAPI.ConfirmationCardView{},
		Operations:    []actionAPI.OperationCardView{},
	}}
	server := newOperationTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, operationsAPIRoot()+"/operations", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("чтение поверхности обязано отвечать 200, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, pin := range []string{`"authorizations"`, `"confirmations":[]`, `"operations":[]`} {
		if !strings.Contains(body, pin) {
			t.Fatalf("пустой вид обязан нести честные пустые списки и обёртку (%s): %s", pin, body)
		}
	}
	if !strings.Contains(body, emptyText) {
		t.Fatalf("пустой вид обязан говорить честные слова отсутствия: %s", body)
	}
}

// TestOperationsViewReadServesTheCardsWithTheVisibleExceptions — the
// machine read answers the full cards: the state word with its fixed
// text, the consumption words, the VISIBLE conflict exception (AC-8) and
// the queued pair-word with the honest boundary texts (AC-11 — executed
// never exists).
func TestOperationsViewReadServesTheCardsWithTheVisibleExceptions(t *testing.T) {
	card := fixtureConfirmationCard()
	operation := fixtureOperationCard()
	fake := &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID:      testTenantID,
		CaseID:        testPreviewCaseID,
		Confirmations: []actionAPI.ConfirmationCardView{card},
		Operations:    []actionAPI.OperationCardView{operation},
	}}
	server := newOperationTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, operationsAPIRoot()+"/operations", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("чтение поверхности обязано отвечать 200, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"state":"valid"`) {
		t.Fatalf("карточка подтверждения обязана нести слово состояния: %s", body)
	}
	if !strings.Contains(body, actionDomain.ConfirmationStateText(actionDomain.ConfirmationValid)) {
		t.Fatalf("карточка подтверждения обязана нести фиксированный текст состояния: %s", body)
	}
	if !strings.Contains(body, actionDomain.ConfirmationEventAuthorizationConflict) {
		t.Fatalf("исключение конфликта обязано быть видимым (AC-8): %s", body)
	}
	if !strings.Contains(body, `"state":"authorized"`) || !strings.Contains(body, `"queued":true`) {
		t.Fatalf("операция обязана нести слово жизни и факт очереди структурно: %s", body)
	}
	if !strings.Contains(body, actionDomain.OperationNextStepText(actionDomain.OperationAuthorized)) {
		t.Fatalf("карточка операции обязана нести следующий шаг: %s", body)
	}
	if !strings.Contains(body, actionDomain.OperationExternalIDAbsentText) {
		t.Fatalf("карточка операции обязана говорить честный текст границы внешнего идентификатора: %s", body)
	}
	if !strings.Contains(body, actionDomain.OperationDispatchAbsentText) {
		t.Fatalf("карточка операции обязана говорить честный текст границы dispatch: %s", body)
	}
	if !strings.Contains(body, `"kind":"authorized"`) {
		t.Fatalf("ledger обязан нести слово authorized: %s", body)
	}
	if strings.Contains(body, "executed") {
		t.Fatalf("executed НЕ существует в этом срезе (AC-11): %s", body)
	}
}

// TestConfirmationThreadReadAnswersTheVersionsAndTheHonest404 — the
// thread read answers every version of the stable identity; a foreign or
// malformed identity is the plain 404 (no disclosure).
func TestConfirmationThreadReadAnswersTheVersionsAndTheHonest404(t *testing.T) {
	fake := &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID:      testTenantID,
		CaseID:        testPreviewCaseID,
		Confirmations: []actionAPI.ConfirmationCardView{fixtureConfirmationCard()},
	}}
	server := newOperationTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		operationsAPIRoot()+"/confirmations/"+testConfirmationID.String(), nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("чтение потока обязано отвечать 200, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"confirmations":[{`) {
		t.Fatalf("чтение потока обязано нести обёртку версий: %s", recorder.Body.String())
	}
	for _, deadPin := range []string{
		"01994f64-6c00-7000-8000-00000000ffff",
		"not-a-uuid",
	} {
		recorder = httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
			operationsAPIRoot()+"/confirmations/"+deadPin, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("мёртвый пин потока обязан отвечать 404 (%s), пришло %d", deadPin, recorder.Code)
		}
	}
}

// TestOperationReadAnswersTheCardAndTheHonest404 — the operation read
// answers the full immutable card; a foreign identity is the plain 404.
func TestOperationReadAnswersTheCardAndTheHonest404(t *testing.T) {
	fake := &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID:   testTenantID,
		CaseID:     testPreviewCaseID,
		Operations: []actionAPI.OperationCardView{fixtureOperationCard()},
	}}
	server := newOperationTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		operationsAPIRoot()+"/operations/"+testOperationID.String(), nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("чтение операции обязано отвечать 200, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"operation":{`) || !strings.Contains(body, `"idempotency_key":"order-2026.10.20.001"`) {
		t.Fatalf("чтение операции обязано нести карточку с клиентским словом: %s", body)
	}
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		operationsAPIRoot()+"/operations/01994f64-6c00-7000-8000-00000000ffff", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("чужая операция обязана отвечать 404, пришло %d", recorder.Code)
	}
}

// TestConfirmAnswers201WithTheServerTruthOutcome — the consent answers the
// documented 201 with the new version, the valid state word and the
// verbatim command capture (the pins of the path, the requested lifetime,
// the actor and the server clock).
func TestConfirmAnswers201WithTheServerTruthOutcome(t *testing.T) {
	fake := &fakeAuthorization{confirm: actionAPI.ConfirmationVersionOutcome{
		Command: "confirm_preview_version", ConfirmationID: testConfirmationID, VersionNo: 1,
		State: actionDomain.ConfirmationValid, StateText: actionDomain.ConfirmationStateText(actionDomain.ConfirmationValid),
		ValidUntil:      time.Unix(3601, 0).UTC(),
		ConsumptionText: "Потреблено 0 из 1 (single_use).",
		PreviewID:       testPreviewID, PreviewVersionNo: 1,
	}}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/action-previews/"+testPreviewID.String()+"/versions/1/confirmations",
		strings.NewReader(`{"lifetime_seconds":600}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("согласие обязано отвечать 201, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"confirmation":{`) || !strings.Contains(body, `"state":"valid"`) {
		t.Fatalf("ответ согласия обязан нести обёртку и слово состояния: %s", body)
	}
	if fake.lastConfirm == nil {
		t.Fatal("команда обязана дойти до сервиса")
	}
	if fake.lastConfirm.PreviewID != testPreviewID || fake.lastConfirm.PreviewVersionNo != 1 ||
		fake.lastConfirm.LifetimeSeconds != 600 || fake.lastConfirm.Actor.ID != testPrincipalID ||
		fake.lastConfirm.TenantID != testTenantID || fake.lastConfirm.CaseID != testPreviewCaseID {
		t.Fatalf("команда обязана нести пины пути и запрошенное окно дословно: %+v", fake.lastConfirm)
	}
	if !fake.lastConfirm.Now.Equal(time.Unix(1, 0).UTC()) {
		t.Fatalf("команда обязана нести серверное время: %+v", fake.lastConfirm.Now)
	}
}

// TestConfirmRefusesTheFreeConfidenceField — the strict decode refuses the
// model's belief BEFORE the service (AG-4/AC-10: the consent is the human
// act over the pinned material, a «confidence» field is the honest 400);
// the wrong content type answers the honest 400 of the media rule.
func TestConfirmRefusesTheFreeConfidenceField(t *testing.T) {
	server := newOperationTestServer(t, &fakeAuthorization{})
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/action-previews/"+testPreviewID.String()+"/versions/1/confirmations",
		strings.NewReader(`{"lifetime_seconds":600,"confidence":0.9}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("свободное поле confidence обязано отвечать 400, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"body"`) {
		t.Fatalf("отказ обязан нести правило body: %s", recorder.Body.String())
	}
	fake := &fakeAuthorization{}
	server = newOperationTestServer(t, fake)
	request = httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/action-previews/"+testPreviewID.String()+"/versions/1/confirmations",
		strings.NewReader(`{"lifetime_seconds":600}`))
	request.Header.Set("Content-Type", "text/plain")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"content_type"`) {
		t.Fatalf("неверный content-type обязан отвечать 400 правилом content_type: %d %s", recorder.Code, recorder.Body.String())
	}
	if fake.confirmCalls != 0 {
		t.Fatalf("strict decode отказывает ДО сервиса, сервис видел %d", fake.confirmCalls)
	}
}

// TestConfirmSpeaksTheGuardRuleWords — the RuleError of the consent gates
// answers the 400 with the rule name as the code and the honest reason
// verbatim (the dictionary words — the transport never invents its copy).
func TestConfirmSpeaksTheGuardRuleWords(t *testing.T) {
	fake := &fakeAuthorization{confirmErr: actionDomain.RuleError{
		Rule:   "confirmation_gate",
		Reason: "Определение требует подтверждения: запросы идут через наблюдения 2-7.",
	}}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/action-previews/"+testPreviewID.String()+"/versions/1/confirmations",
		strings.NewReader(`{"lifetime_seconds":600}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("гейт подтверждения обязан отвечать 400, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"confirmation_gate"`) || !strings.Contains(body, "запросы идут через наблюдения 2-7") {
		t.Fatalf("отказ гейта обязан нести правило и честную причину дословно: %s", body)
	}
}

// TestConfirmSpeaksTheStepUpRefusal — the consent is the human act: the
// live step-up refusal answers the 403 with the step-up words.
func TestConfirmSpeaksTheStepUpRefusal(t *testing.T) {
	fake := &fakeAuthorization{confirmErr: domain.ErrStepUpRequired}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/action-previews/"+testPreviewID.String()+"/versions/1/confirmations",
		strings.NewReader(`{"lifetime_seconds":600}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("отказ step-up обязан отвечать 403, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"step_up_required"`) || !strings.Contains(body, "повторное подтверждение личности") {
		t.Fatalf("отказ step-up обязан нести свои слова: %s", body)
	}
}

// TestConfirmSpeaksTheRacedLadderByState — the raced append answers the
// 409 with the EXACT state word as the code (AC-1: superseded / revoked /
// expired / exhausted — never a masked silence).
func TestConfirmSpeaksTheRacedLadderByState(t *testing.T) {
	for _, state := range []string{
		actionDomain.ConfirmationSuperseded,
		actionDomain.ConfirmationRevoked,
		actionDomain.ConfirmationExpired,
		actionDomain.ConfirmationExhausted,
	} {
		fake := &fakeAuthorization{confirmErr: actionPort.ConfirmationRejectedError{State: state}}
		server := newOperationTestServer(t, fake)
		request := httptest.NewRequest(http.MethodPost,
			operationsAPIRoot()+"/action-previews/"+testPreviewID.String()+"/versions/1/confirmations",
			strings.NewReader(`{"lifetime_seconds":600}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("состояние %s обязано отвечать 409, пришло %d", state, recorder.Code)
		}
		body := recorder.Body.String()
		if !strings.Contains(body, `"code":"confirmation_`+state+`"`) {
			t.Fatalf("код отказа обязан нести точное слово состояния %s: %s", state, body)
		}
		if !strings.Contains(body, actionDomain.ConfirmationStateText(state)) {
			t.Fatalf("отказ обязан нести фиксированный текст состояния %s: %s", state, body)
		}
	}
}

// TestRevokeAnswers200WithTheRevokedWords — the revocation answers the
// documented 200 with the revoked state word and the verbatim pin capture
// of the path.
func TestRevokeAnswers200WithTheRevokedWords(t *testing.T) {
	fake := &fakeAuthorization{revoke: actionAPI.ConfirmationRevocationOutcome{
		Command: "revoke_confirmation", ConfirmationID: testConfirmationID, VersionNo: 1,
		State: actionDomain.ConfirmationRevoked, StateText: actionDomain.ConfirmationStateText(actionDomain.ConfirmationRevoked),
		RevocationNote: "Будущие авторизации невозможны; прошлое потребление остаётся фактом аудита.",
	}}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/revocations",
		strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("отзыв обязан отвечать 200, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"revocation":{`) || !strings.Contains(body, `"state":"revoked"`) {
		t.Fatalf("ответ отзыва обязан нести обёртку и слово состояния: %s", body)
	}
	if fake.lastRevoke == nil || fake.lastRevoke.ConfirmationID != testConfirmationID || fake.lastRevoke.VersionNo != 1 {
		t.Fatalf("команда отзыва обязана нести пин пути дословно: %+v", fake.lastRevoke)
	}
}

// TestRevokeRefusesTheFreeField — the revocation body carries NOTHING
// (the pin travels in the path): a free field is the honest 400 BEFORE
// the service.
func TestRevokeRefusesTheFreeField(t *testing.T) {
	fake := &fakeAuthorization{}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/revocations",
		strings.NewReader(`{"confidence":1}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("свободное поле отзыва обязано отвечать 400, пришло %d", recorder.Code)
	}
	if fake.revokeCalls != 0 {
		t.Fatalf("strict decode отказывает ДО сервиса, сервис видел %d", fake.revokeCalls)
	}
}

// TestAuthorizeAnswers201OfTheFirstAuthorization — the FIRST authorization
// answers the documented 201 with the immutable operation card and the
// verbatim capture of the client idempotency word and the confirmation pin.
func TestAuthorizeAnswers201OfTheFirstAuthorization(t *testing.T) {
	fake := &fakeAuthorization{authorize: actionAPI.AuthorizationOutcome{
		Command: "authorize_operation", OutcomeText: "Operation авторизована и в очереди.",
		Operation: fixtureOperationCard(),
	}}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/authorizations",
		strings.NewReader(`{"idempotency_key":"order-2026.10.20.001"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("первая авторизация обязана отвечать 201, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, pin := range []string{`"authorization":{`, `"replay":false`, `"reservation_reference":false`} {
		if !strings.Contains(body, pin) {
			t.Fatalf("ответ первой авторизации обязан нести обёртку и честные флаги (%s): %s", pin, body)
		}
	}
	if fake.lastAuthorize == nil || fake.lastAuthorize.IdempotencyKey != "order-2026.10.20.001" ||
		fake.lastAuthorize.ConfirmationID != testConfirmationID || fake.lastAuthorize.ConfirmationVersionNo != 1 {
		t.Fatalf("команда обязана нести клиентское слово повтора и пин подтверждения дословно: %+v", fake.lastAuthorize)
	}
}

// TestAuthorizeAnswers200OfTheReplayAndTheReference — the honest 200
// family: the replay of the SAME client key (AC-7 — the original
// operation returns, the consumption never repeats) and the cross-case
// reservation reference (AC-9 — the loser's consumption never happened).
func TestAuthorizeAnswers200OfTheReplayAndTheReference(t *testing.T) {
	for _, spec := range []struct {
		name string
		out  actionAPI.AuthorizationOutcome
		flag string
	}{
		{
			name: "replay",
			out: actionAPI.AuthorizationOutcome{
				Replay:      true,
				OutcomeText: actionDomain.IdempotentReplayText,
				Operation:   fixtureOperationCard(),
			},
			flag: `"replay":true`,
		},
		{
			name: "reference",
			out: actionAPI.AuthorizationOutcome{
				ReservationReference: true,
				OutcomeText:          actionDomain.ReservationDuplicateText,
				Operation:            fixtureOperationCard(),
			},
			flag: `"reservation_reference":true`,
		},
	} {
		fake := &fakeAuthorization{authorize: spec.out}
		server := newOperationTestServer(t, fake)
		request := httptest.NewRequest(http.MethodPost,
			operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/authorizations",
			strings.NewReader(`{"idempotency_key":"order-2026.10.20.001"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("исход %s обязан отвечать 200, пришло %d", spec.name, recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), spec.flag) {
			t.Fatalf("исход %s обязан нести свой флаг %s: %s", spec.name, spec.flag, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), spec.out.OutcomeText) {
			t.Fatalf("исход %s обязан говорить свой фиксированный текст: %s", spec.name, recorder.Body.String())
		}
	}
}

// TestAuthorizeRefusesTheFreeConfidenceField — the machine authorization
// carries the client idempotency word ONLY: a free «confidence»/
// «assurance» field is the honest 400 BEFORE the service (AG-4/AC-10 —
// the model's confidence never substitutes a Confirmation).
func TestAuthorizeRefusesTheFreeConfidenceField(t *testing.T) {
	fake := &fakeAuthorization{}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/authorizations",
		strings.NewReader(`{"idempotency_key":"order-2026.10.20.001","confidence":0.9}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("свободное поле confidence обязано отвечать 400, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	if fake.authorizeCalls != 0 {
		t.Fatalf("strict decode отказывает ДО сервиса и ДО гейта ключа, сервис видел %d", fake.authorizeCalls)
	}
}

// TestAuthorizeSpeaksTheKeyGateRuleWords — the key gate of the ordered
// gateway answers the 400 with the rule name and the honest charset/length
// words (AC-2 — BEFORE everything, the transport maps the rule verbatim).
func TestAuthorizeSpeaksTheKeyGateRuleWords(t *testing.T) {
	fake := &fakeAuthorization{authorizeErr: actionDomain.RuleError{
		Rule:   "idempotency_key",
		Reason: "Слово повтора вне charset: буквы, цифры, точка, подчёркивание, дефис (8..128 рун).",
	}}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/authorizations",
		strings.NewReader(`{"idempotency_key":"short!"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("ключ-гейт обязан отвечать 400, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"idempotency_key"`) || !strings.Contains(body, "charset") {
		t.Fatalf("ключ-гейт обязан нести правило и честные слова: %s", body)
	}
}

// TestAuthorizeSpeaksTheHonest409Words — the exact honest words of the
// 409 family: the idempotency conflict with the wrapped reason (AC-8),
// the freshness stale reason (AuthorizationStaleError), the changed
// material condition, the changed preview digest and the blocked material
// — every code by name, the wrapped reason verbatim.
func TestAuthorizeSpeaksTheHonest409Words(t *testing.T) {
	for _, spec := range []struct {
		name string
		err  error
		code string
		text string
	}{
		{
			name: "idempotency conflict",
			err:  fmt.Errorf("%w: другой материал под тем же ключом", actionPort.ErrIdempotencyConflict),
			code: `"code":"idempotency_conflict"`,
			text: "другой материал под тем же ключом",
		},
		{
			name: "freshness reason",
			err:  actionPort.AuthorizationStaleError{Reason: "факт order_status несвеж"},
			code: `"code":"authorization_stale"`,
			text: "факт order_status несвеж",
		},
		{
			// The RAW sentinel (no wrap): the transport falls back onto the
			// fixed domain text — the app attaches its own words at the wrap
			// point in production, the transport never invents a copy.
			name: "material condition",
			err:  actionPort.ErrMaterialConditionChanged,
			code: `"code":"material_condition_changed"`,
			text: actionDomain.MaterialConditionChangedText,
		},
		{
			name: "preview digest",
			err:  actionPort.ErrPreviewDigestChanged,
			code: `"code":"confirmation_preview_changed"`,
			text: actionDomain.ConfirmationPreviewChangedText,
		},
		{
			name: "blocked material",
			err:  fmt.Errorf("%w", actionPort.ErrPreviewBlockedMaterial),
			code: `"code":"preview_blocked_material"`,
			text: "Материал preview заблокирован",
		},
		{
			name: "raced ladder",
			err:  actionPort.ErrStaleConfirmationVersion,
			code: `"code":"stale_confirmation_version"`,
			text: "Лестница версий Confirmation двинулась",
		},
	} {
		fake := &fakeAuthorization{authorizeErr: spec.err}
		server := newOperationTestServer(t, fake)
		request := httptest.NewRequest(http.MethodPost,
			operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/authorizations",
			strings.NewReader(`{"idempotency_key":"order-2026.10.20.001"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("исход %s обязан отвечать 409, пришло %d", spec.name, recorder.Code)
		}
		body := recorder.Body.String()
		if !strings.Contains(body, spec.code) || !strings.Contains(body, spec.text) {
			t.Fatalf("исход %s обязан нести код %s и честный текст: %s", spec.name, spec.code, body)
		}
	}
}

// TestAuthorizeSpeaksTheLadderConflictByState — the rejection ladder of
// the authorization answers the 409 with the EXACT state word as the code
// (AC-1 — superseded / revoked / expired / exhausted by name, never a
// masked silence; the consumption never happens on a rejection).
func TestAuthorizeSpeaksTheLadderConflictByState(t *testing.T) {
	for _, state := range []string{
		actionDomain.ConfirmationSuperseded,
		actionDomain.ConfirmationRevoked,
		actionDomain.ConfirmationExpired,
		actionDomain.ConfirmationExhausted,
	} {
		fake := &fakeAuthorization{authorizeErr: actionPort.ConfirmationRejectedError{State: state}}
		server := newOperationTestServer(t, fake)
		request := httptest.NewRequest(http.MethodPost,
			operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/authorizations",
			strings.NewReader(`{"idempotency_key":"order-2026.10.20.001"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("лестница %s обязана отвечать 409, пришло %d", state, recorder.Code)
		}
		body := recorder.Body.String()
		if !strings.Contains(body, `"code":"confirmation_`+state+`"`) {
			t.Fatalf("лестница обязана нести точное слово %s кодом: %s", state, body)
		}
		if !strings.Contains(body, actionDomain.ConfirmationStateText(state)) {
			t.Fatalf("лестница %s обязана нести фиксированный текст: %s", state, body)
		}
	}
}

// TestAuthorizeSpeaksTheFailClosedFreshReadsRefusal — the nil production
// reader answers the fail-closed 403 (AG-4: the authorization honestly
// cannot happen without the fresh reads).
func TestAuthorizeSpeaksTheFailClosedFreshReadsRefusal(t *testing.T) {
	fake := &fakeAuthorization{authorizeErr: fmt.Errorf("%w", actionPort.ErrFreshReadsUnavailable)}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/authorizations",
		strings.NewReader(`{"idempotency_key":"order-2026.10.20.001"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("fail-closed свежие чтения обязан отвечать 403, пришло %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"fresh_reads_unavailable"`) || !strings.Contains(body, actionDomain.FreshReadsUnavailableText) {
		t.Fatalf("fail-closed отказ обязан нести код и честные слова: %s", body)
	}
}

// TestAuthorizeAnswers404OfTheDeadPins — the dead confirmation pin of the
// path answers the plain 404 BEFORE the service; the app's own ErrNotFound
// of the resolve maps the same honest 404 (no disclosure).
func TestAuthorizeAnswers404OfTheDeadPins(t *testing.T) {
	fake := &fakeAuthorization{}
	server := newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/confirmations/not-a-uuid/versions/1/authorizations",
		strings.NewReader(`{"idempotency_key":"order-2026.10.20.001"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("мёртвый пин пути обязан отвечать 404 ДО сервиса, пришло %d", recorder.Code)
	}
	if fake.authorizeCalls != 0 {
		t.Fatalf("мёртвый пин не доходит до сервиса, сервис видел %d", fake.authorizeCalls)
	}
	fake = &fakeAuthorization{authorizeErr: actionDomain.ErrNotFound}
	server = newOperationTestServer(t, fake)
	request = httptest.NewRequest(http.MethodPost,
		operationsAPIRoot()+"/confirmations/"+testConfirmationID.String()+"/versions/1/authorizations",
		strings.NewReader(`{"idempotency_key":"order-2026.10.20.001"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("мёртвое подтверждение сервиса обязано отвечать 404, пришло %d", recorder.Code)
	}
}

// TestUnboundAuthorizationSurfaceAnswersTheHonest404 — the unbound service
// keeps the WHOLE surface absent (the 404, never a 500): the production
// root that wires nothing honestly does not expose the contour.
func TestUnboundAuthorizationSurfaceAnswersTheHonest404(t *testing.T) {
	identity := api.Service(fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) {
			return domain.Workspace{}, nil
		},
	})
	server, err := New(identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, StepUpVerified: true}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindCases(&fakeCases{})
	for _, target := range []string{
		operationsAPIRoot() + "/operations",
		operationsAPIRoot() + "/confirmations/" + testConfirmationID.String(),
		operationsAPIRoot() + "/operations/" + testOperationID.String(),
		operationsAPIRoot() + "/action-previews/" + testPreviewID.String() + "/versions/1/confirmations",
		operationsAPIRoot() + "/confirmations/" + testConfirmationID.String() + "/versions/1/revocations",
		operationsAPIRoot() + "/confirmations/" + testConfirmationID.String() + "/versions/1/authorizations",
		operationsPageRoot() + "/operations",
	} {
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("непривязанная поверхность обязана отсутствовать (404) на %s, пришло %d", target, recorder.Code)
		}
	}
}

// operationsFlashOf reads the honest flash text of a PRG refusal (the
// flash cookie of the tenant path).
func operationsFlashOf(recorder *httptest.ResponseRecorder) string {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name != flashCookieName {
			continue
		}
		decoded, err := url.QueryUnescape(cookie.Value)
		if err != nil {
			return ""
		}
		return decoded
	}
	return ""
}

// operationsFormOf builds the form-encoded page request with the honest
// browser origin.
func operationsFormOf(method, target string, form url.Values) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+request.Host)
	return request
}

// TestOperationReviewPageRendersTheConfirmFormAndTheCards — the page
// answers the confirm form (prefilled by the 3-4 primary transition pin),
// the confirmation cards with the state/consumption words and the VISIBLE
// exception history, the per-card revocation form with the explicit
// checkbox and the authorize form with the machine idempotency word, the
// operation cards with the ledger and the honest boundary texts (a11y:
// every state in TEXT, never by the colour alone).
func TestOperationReviewPageRendersTheConfirmFormAndTheCards(t *testing.T) {
	fake := &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID:      testTenantID,
		CaseID:        testPreviewCaseID,
		Confirmations: []actionAPI.ConfirmationCardView{fixtureConfirmationCard()},
		Operations:    []actionAPI.OperationCardView{fixtureOperationCard()},
	}}
	server := newOperationTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		operationsPageRoot()+"/operations?confirm="+testPreviewID.String()+"@1", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("страница обязана отвечать 200, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	page := recorder.Body.String()
	for _, pin := range []string{
		"Operation Review — подтверждение и авторизация действия",
		`action="/app/tenants/` + testTenantID.String() + `/cases/` + testPreviewCaseID.String() + `/operations/confirmations"`,
		`value="` + testPreviewID.String() + `"`,
		"Подтвердить версию preview",
		actionDomain.ConfirmationStateText(actionDomain.ConfirmationValid),
		"Потреблено 0 из 1 (single_use).",
		"История событий (видимые исключения отвергнутых авторизаций)",
		actionDomain.ConfirmationEventAuthorizationConflict,
		`name="confirm_revocation" value="yes" required`,
		"Idempotency key (машинное слово повтора",
		"Operation Ledger (append-only)",
		actionDomain.OperationExternalIDAbsentText,
		actionDomain.OperationDispatchAbsentText,
		"authorized",
		"В очереди на dispatch: да",
		actionDomain.OperationNextStepText(actionDomain.OperationAuthorized),
		"Попытки отправки (прогрессивный аудит, закрыто по умолчанию)",
		"Карточка Action Receipt (квитанция — readback идентификаторов)",
		"msg-42",
		"syn-0123456789abcdef",
	} {
		if !strings.Contains(page, pin) {
			t.Fatalf("страница обязана нести элемент (%s): %s", pin, page[:400])
		}
	}
	if strings.Contains(page, "executed") {
		t.Fatalf("executed НЕ существует на странице (AC-11): %s", page[:400])
	}
}

// TestOperationReviewPageSpeaksTheHonestEmptyState — the case without
// consents and operations answers the honest empty words, never a null
// silence and never a hidden dispatch.
func TestOperationReviewPageSpeaksTheHonestEmptyState(t *testing.T) {
	emptyText := "В этом кейсе ещё нет подтверждений и операций — подтверждение строится точной версией preview со страницы Action Preview."
	fake := &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID: testTenantID, CaseID: testPreviewCaseID, EmptyText: emptyText,
		Confirmations: []actionAPI.ConfirmationCardView{},
		Operations:    []actionAPI.OperationCardView{},
	}}
	server := newOperationTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		operationsPageRoot()+"/operations", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("страница обязана отвечать 200, пришло %d", recorder.Code)
	}
	page := recorder.Body.String()
	if !strings.Contains(page, emptyText) {
		t.Fatalf("пустое состояние обязано говорить честные слова: %s", page[:400])
	}
	if strings.Contains(page, "Подтверждения (append-only версии)") || strings.Contains(page, "Операции (неизменяемые)") {
		t.Fatalf("пустой кейс не несёт карточек: %s", page[:400])
	}
}

// TestOperationReviewPagePinsTheNoticeFocusAndSpeaksTheFlags — the PRG
// notice of every honest flag carries role=status, the focus transfer
// (tabindex + the script that moves the focus onto the notice) and the
// exact identifier readback of the landed step.
func TestOperationReviewPagePinsTheNoticeFocusAndSpeaksTheFlags(t *testing.T) {
	for _, spec := range []struct {
		flag  string
		value string
		words string
	}{
		{flag: "confirmed", value: "2", words: "Confirmation версии 2 построена"},
		{flag: "revoked", value: "1", words: "Confirmation версии 1 отозвана"},
		{flag: "authorized", value: testOperationID.String(), words: "Operation " + testOperationID.String() + " авторизована и в очереди"},
		{flag: "replayed", value: testOperationID.String(), words: "Повтор авторизации: возвращена первоначальная Operation " + testOperationID.String()},
		{flag: "referenced", value: testOperationID.String(), words: "Эквивалентное бизнес-намерение уже зарезервировано"},
	} {
		fake := &fakeAuthorization{view: actionAPI.AuthorizationView{
			TenantID: testTenantID, CaseID: testPreviewCaseID,
			Confirmations: []actionAPI.ConfirmationCardView{}, Operations: []actionAPI.OperationCardView{},
		}}
		server := newOperationTestServer(t, fake)
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
			operationsPageRoot()+"/operations?"+spec.flag+"="+spec.value, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("флаг %s: страница обязана отвечать 200, пришло %d", spec.flag, recorder.Code)
		}
		page := recorder.Body.String()
		for _, pin := range []string{
			`id="page-notice"`,
			`role="status"`,
			`tabindex="-1"`,
			"getElementById('page-notice').focus()",
			spec.words,
		} {
			if !strings.Contains(page, pin) {
				t.Fatalf("флаг %s: уведомление обязано нести фокус и слова (%s): %s", spec.flag, pin, page[:400])
			}
		}
	}
}

// TestConfirmFormIsThePRGFamilyWithTheOutcomeFlag — the page consent form
// demands the browser origin, parses the pin pairs honestly and lands
// PRG with the confirmed flag of the new version number.
func TestConfirmFormIsThePRGFamilyWithTheOutcomeFlag(t *testing.T) {
	fake := &fakeAuthorization{confirm: actionAPI.ConfirmationVersionOutcome{
		Command: "confirm_preview_version", ConfirmationID: testConfirmationID, VersionNo: 2,
		State: actionDomain.ConfirmationValid, StateText: actionDomain.ConfirmationStateText(actionDomain.ConfirmationValid),
	}}
	server := newOperationTestServer(t, fake)
	form := url.Values{}
	form.Set("preview_id", testPreviewID.String())
	form.Set("version_no", "1")
	form.Set("lifetime_seconds", "600")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost,
		operationsPageRoot()+"/operations/confirmations", form))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("успешная форма обязана ответить PRG 303, пришло %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); !strings.Contains(location, "confirmed=2") {
		t.Fatalf("PRG обязан нести флаг confirmed с номером новой версии: %s", location)
	}
	if fake.lastConfirm == nil || fake.lastConfirm.PreviewID != testPreviewID ||
		fake.lastConfirm.PreviewVersionNo != 1 || fake.lastConfirm.LifetimeSeconds != 600 {
		t.Fatalf("форма обязана разобрать пины и окно честно: %+v", fake.lastConfirm)
	}
	// The origin refusal: the cross-site form never reaches the service.
	fake = &fakeAuthorization{}
	server = newOperationTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost,
		operationsPageRoot()+"/operations/confirmations", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://evil.example")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("чужой origin обязан отказать 403, пришло %d", recorder.Code)
	}
	if fake.confirmCalls != 0 {
		t.Fatalf("origin-отказ происходит ДО сервиса, сервис видел %d", fake.confirmCalls)
	}
}

// TestConfirmFormSpeaksTheParseAndServiceRefusalsInTheFlash — the honest
// parse refusals and the honest service words ride the PRG flash (the
// 3.3/3.4 precedent: the rule words, the ladder words, the step-up words).
func TestConfirmFormSpeaksTheParseAndServiceRefusalsInTheFlash(t *testing.T) {
	for _, spec := range []struct {
		name  string
		form  url.Values
		flash string
	}{
		{
			name:  "bad preview id",
			form:  url.Values{"preview_id": {"not-a-uuid"}, "version_no": {"1"}, "lifetime_seconds": {"600"}},
			flash: "Preview — действительный UUID",
		},
		{
			name:  "bad version",
			form:  url.Values{"preview_id": {testPreviewID.String()}, "version_no": {"0"}, "lifetime_seconds": {"600"}},
			flash: "Версия preview — целое число начиная с 1.",
		},
		{
			name:  "bad lifetime",
			form:  url.Values{"preview_id": {testPreviewID.String()}, "version_no": {"1"}, "lifetime_seconds": {"-5"}},
			flash: "положительное целое число секунд",
		},
	} {
		fake := &fakeAuthorization{}
		server := newOperationTestServer(t, fake)
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost,
			operationsPageRoot()+"/operations/confirmations", spec.form))
		if recorder.Code != http.StatusSeeOther {
			t.Fatalf("отказ разбора %s обязан ответить PRG 303, пришло %d", spec.name, recorder.Code)
		}
		if location := recorder.Header().Get("Location"); !strings.Contains(location, "x=1") {
			t.Fatalf("отказ обязан возвращать на страницу нейтральным флагом: %s", location)
		}
		if flash := operationsFlashOf(recorder); !strings.Contains(flash, spec.flash) {
			t.Fatalf("отказ разбора %s обязан нести честные слова во flash: %q", spec.name, flash)
		}
		if fake.confirmCalls != 0 {
			t.Fatalf("отказ разбора %s не доходит до сервиса, сервис видел %d", spec.name, fake.confirmCalls)
		}
	}
	// The service step-up refusal rides the flash with the step-up words.
	fake := &fakeAuthorization{confirmErr: domain.ErrStepUpRequired}
	server := newOperationTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost,
		operationsPageRoot()+"/operations/confirmations", url.Values{
			"preview_id": {testPreviewID.String()}, "version_no": {"1"}, "lifetime_seconds": {"600"},
		}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("step-up отказ формы обязан ответить PRG, пришло %d", recorder.Code)
	}
	if flash := operationsFlashOf(recorder); !strings.Contains(flash, "повторное подтверждение личности") {
		t.Fatalf("step-up отказ обязан нести слова step-up во flash: %q", flash)
	}
	// The rejection ladder rides the flash with the exact state text.
	fake = &fakeAuthorization{confirmErr: actionPort.ConfirmationRejectedError{State: actionDomain.ConfirmationSuperseded}}
	server = newOperationTestServer(t, fake)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost,
		operationsPageRoot()+"/operations/confirmations", url.Values{
			"preview_id": {testPreviewID.String()}, "version_no": {"1"}, "lifetime_seconds": {"600"},
		}))
	if flash := operationsFlashOf(recorder); !strings.Contains(flash, actionDomain.ConfirmationStateText(actionDomain.ConfirmationSuperseded)) {
		t.Fatalf("лестница обязана нести точный текст состояния во flash: %q", flash)
	}
}

// TestRevokeFormRequiresTheExplicitCheckboxAndSpeaksTheWords — the
// revocation is the deliberate human act: the explicit checkbox is the
// UI discipline (the machine API carries no checkbox field), the words
// ride the flash, the success lands PRG with the revoked flag.
func TestRevokeFormRequiresTheExplicitCheckboxAndSpeaksTheWords(t *testing.T) {
	formURL := operationsPageRoot() + "/confirmations/" + testConfirmationID.String() + "/versions/1/revocations"
	fake := &fakeAuthorization{}
	server := newOperationTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formURL, url.Values{}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("отзыв без чекбокса обязан ответить PRG 303, пришло %d", recorder.Code)
	}
	if flash := operationsFlashOf(recorder); !strings.Contains(flash, "чекбокс согласия") {
		t.Fatalf("отзыв без чекбокса обязан нести честные слова: %q", flash)
	}
	if fake.revokeCalls != 0 {
		t.Fatalf("отзыв без чекбокса не доходит до сервиса, сервис видел %d", fake.revokeCalls)
	}
	fake = &fakeAuthorization{revoke: actionAPI.ConfirmationRevocationOutcome{
		Command: "revoke_confirmation", ConfirmationID: testConfirmationID, VersionNo: 1,
		State: actionDomain.ConfirmationRevoked, StateText: actionDomain.ConfirmationStateText(actionDomain.ConfirmationRevoked),
	}}
	server = newOperationTestServer(t, fake)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formURL,
		url.Values{"confirm_revocation": {"yes"}}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("подтверждённый отзыв обязан ответить PRG 303, пришло %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); !strings.Contains(location, "revoked=1") {
		t.Fatalf("PRG отзыва обязан нести флаг revoked: %s", location)
	}
	if fake.lastRevoke == nil || fake.lastRevoke.ConfirmationID != testConfirmationID || fake.lastRevoke.VersionNo != 1 {
		t.Fatalf("команда отзыва обязана нести пин пути дословно: %+v", fake.lastRevoke)
	}
}

// TestAuthorizeFormRedirectsWithTheHonestFlags — the page authorize form
// carries the machine idempotency word (the page generates NO key for the
// operator): the first authorization lands PRG authorized, the replay
// lands replayed, the reference lands referenced; the empty key is the
// honest flash refusal BEFORE the service.
func TestAuthorizeFormRedirectsWithTheHonestFlags(t *testing.T) {
	formURL := operationsPageRoot() + "/confirmations/" + testConfirmationID.String() + "/versions/1/authorizations"
	fake := &fakeAuthorization{}
	server := newOperationTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formURL,
		url.Values{"idempotency_key": {"   "}}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("пустой ключ обязан ответить PRG 303, пришло %d", recorder.Code)
	}
	if flash := operationsFlashOf(recorder); !strings.Contains(flash, "Idempotency key обязателен") {
		t.Fatalf("пустой ключ обязан нести честные слова: %q", flash)
	}
	if fake.authorizeCalls != 0 {
		t.Fatalf("пустой ключ не доходит до сервиса, сервис видел %d", fake.authorizeCalls)
	}
	for _, spec := range []struct {
		name string
		out  actionAPI.AuthorizationOutcome
		flag string
	}{
		{
			name: "first",
			out:  actionAPI.AuthorizationOutcome{OutcomeText: "авторизована", Operation: fixtureOperationCard()},
			flag: "authorized=" + testOperationID.String(),
		},
		{
			name: "replay",
			out:  actionAPI.AuthorizationOutcome{Replay: true, OutcomeText: "повтор", Operation: fixtureOperationCard()},
			flag: "replayed=" + testOperationID.String(),
		},
		{
			name: "reference",
			out:  actionAPI.AuthorizationOutcome{ReservationReference: true, OutcomeText: "ссылка", Operation: fixtureOperationCard()},
			flag: "referenced=" + testOperationID.String(),
		},
	} {
		fake := &fakeAuthorization{authorize: spec.out}
		server := newOperationTestServer(t, fake)
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formURL,
			url.Values{"idempotency_key": {"order-2026.10.20.001"}}))
		if recorder.Code != http.StatusSeeOther {
			t.Fatalf("форма %s обязана ответить PRG 303, пришло %d", spec.name, recorder.Code)
		}
		if location := recorder.Header().Get("Location"); !strings.Contains(location, spec.flag) {
			t.Fatalf("форма %s обязана нести флаг %s: %s", spec.name, spec.flag, location)
		}
	}
}

// ---------------------------------------------------------------------------
// The Story 3.6 (step-03) dispatch surface: the drain gates and the page
// ---------------------------------------------------------------------------

// fakeDispatch serves the execution-contour surface with the scripted
// answers and the captured commands (the fakeAuthorization precedent —
// the counters pin the transport's own gate order: the strict decode
// refuses BEFORE the service).
type fakeDispatch struct {
	drain     actionAPI.DispatchDrainResult
	drainErr  error
	drainCall int
	lastDrain *actionAPI.DrainDispatchCommand
}

// The compile-time proof the fake satisfies the narrow transport contract.
var _ actionAPI.DispatchService = (*fakeDispatch)(nil)

func (f *fakeDispatch) DrainDispatchQueue(_ context.Context, c actionAPI.DrainDispatchCommand) (actionAPI.DispatchDrainResult, error) {
	f.drainCall++
	commandCopy := c
	f.lastDrain = &commandCopy
	if f.drainErr != nil {
		return actionAPI.DispatchDrainResult{}, f.drainErr
	}
	return f.drain, nil
}

// newDispatchTestServer binds the fake through the production
// BindDispatch — the narrow interface is type-asserted exactly the way
// the real wiring does it (the authorization fake rides along for the
// page reads).
func newDispatchTestServer(t *testing.T, dispatch *fakeDispatch) *Server {
	t.Helper()
	server := newOperationTestServer(t, &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID: testTenantID, CaseID: testPreviewCaseID,
		Confirmations: []actionAPI.ConfirmationCardView{},
		Operations:    []actionAPI.OperationCardView{},
	}})
	server.BindDispatch(dispatch)
	return server
}

// drainAPIPath builds the machine drain path of the fixture tenant.
func drainAPIPath() string {
	return "/v1/tenants/" + testTenantID.String() + "/operations/drain"
}

// TestDrainRefusesTheFreeFieldBeforeTheService — the strict decode of the
// EMPTY body: a free «limit»/«force» field is the honest 400 BEFORE the
// service is ever called (the drain carries NO parameters — the bounded
// operator command of the domain); the wrong content type answers the
// media rule words.
func TestDrainRefusesTheFreeFieldBeforeTheService(t *testing.T) {
	fake := &fakeDispatch{}
	server := newDispatchTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, drainAPIPath(), strings.NewReader(`{"limit":50}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("свободное поле обязано отвечать 400, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"body"`) {
		t.Fatalf("отказ обязан нести правило body: %s", recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, drainAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"content_type"`) {
		t.Fatalf("неверный content-type обязан отвечать 400 правилом content_type: %d %s", recorder.Code, recorder.Body.String())
	}
	if fake.drainCall != 0 {
		t.Fatalf("strict decode отказывает ДО сервиса, сервис видел %d", fake.drainCall)
	}
}

// TestDrainAnswers201WithTheHonestReport — the empty strict body passes,
// the command carries the actor, the tenant and the server clock
// verbatim, and the 201 answer speaks the honest report: the counters and
// the per-line words (every considered job leaves its line — FR-28).
func TestDrainAnswers201WithTheHonestReport(t *testing.T) {
	lineText := actionDomain.DispatchBlockedText("revoked")
	fake := &fakeDispatch{drain: actionAPI.DispatchDrainResult{
		Command: "dispatch_queued_operations", Considered: 2, Dispatched: 1, Blocked: 1, Skipped: 0,
		Reports: []actionAPI.DispatchReportLineView{
			{OperationID: testOperationID, AttemptNo: 1, Text: actionDomain.OperationCommittedText},
			{OperationID: testOperationID, AttemptNo: 0, Text: lineText},
		},
	}}
	server := newDispatchTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, drainAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("drain обязан отвечать 201 с отчётом, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, pin := range []string{
		`"drain":{`,
		`"considered":2`,
		`"dispatched":1`,
		`"blocked":1`,
		actionDomain.OperationCommittedText,
		lineText,
	} {
		if !strings.Contains(body, pin) {
			t.Fatalf("отчёт обязан нести счётчики и построчные слова (%s): %s", pin, body)
		}
	}
	if fake.lastDrain == nil || fake.lastDrain.Actor.ID != testPrincipalID ||
		fake.lastDrain.TenantID != testTenantID || !fake.lastDrain.Now.Equal(time.Unix(1, 0).UTC()) {
		t.Fatalf("команда обязана нести актёра, тенент и серверное время дословно: %+v", fake.lastDrain)
	}
}

// TestDrainSpeaksTheAG4RefusalWords — the honest production refusal of the
// nil mutation adapter answers 403 with the EXACT words the app attached
// at the wrap point (AG-4: the queue stays pending, zero sends — never a
// masked success); the drain never touched the queue.
func TestDrainSpeaksTheAG4RefusalWords(t *testing.T) {
	fake := &fakeDispatch{drainErr: fmt.Errorf("%w: %s",
		actionAPI.ErrDispatchRefusedAG4, "Исполнение не подключено — реальных провайдеров нет: очередь не тронута, отправок ноль (AG-4).")}
	server := newDispatchTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, drainAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("AG-4 отказ обязан отвечать 403, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"mutation_execution_not_wired"`) {
		t.Fatalf("AG-4 отказ обязан нести код mutation_execution_not_wired: %s", body)
	}
	if !strings.Contains(body, "очередь не тронута, отправок ноль") {
		t.Fatalf("AG-4 отказ обязан нести честные слова приложения дословно: %s", body)
	}
}

// TestDrainSpeaksTheScopeAndMissingWords — the rights gate of the app
// answers 403 with the scope words the app attached (the operations scope
// of the operator), the identity NotFound keeps the plain 404 (no
// disclosure that the tenant dispatches actions at all).
func TestDrainSpeaksTheScopeAndMissingWords(t *testing.T) {
	fake := &fakeDispatch{drainErr: fmt.Errorf("%w: %s", actionDomain.ErrForbidden,
		"Команда drain требует полномочия operations: запуск отправок очереди — операторная команда, участники без операторного полномочия очередь не трогают.")}
	server := newDispatchTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, drainAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("отказ полномочия обязан отвечать 403, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "полномочия operations") {
		t.Fatalf("отказ полномочия обязан нести слова скоупа дословно: %s", recorder.Body.String())
	}
	fake = &fakeDispatch{drainErr: actionDomain.ErrNotFound}
	server = newDispatchTestServer(t, fake)
	request = httptest.NewRequest(http.MethodPost, drainAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("не-участник обязан отвечать 404, пришло %d", recorder.Code)
	}
}

// TestDrainFormRequiresTheCheckboxAndLandsPRG — the page drain form is the
// PRG family (the external-commitments precedent): the explicit checkbox
// is the UI discipline of the operator act, the missing checkbox refuses
// BEFORE the service, the AG-4 refusal words ride the flash, the success
// lands PRG with the considered counter; the foreign origin never reaches
// the service.
func TestDrainFormRequiresTheCheckboxAndLandsPRG(t *testing.T) {
	formPath := "/app/tenants/" + testTenantID.String() + "/operations/drain"
	fake := &fakeDispatch{}
	server := newDispatchTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formPath,
		url.Values{"case_id": {testPreviewCaseID.String()}}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("дренаж без чекбокса обязан ответить PRG 303, пришло %d", recorder.Code)
	}
	if flash := operationsFlashOf(recorder); !strings.Contains(flash, "чекбокс согласия") {
		t.Fatalf("дренаж без чекбокса обязан нести честные слова: %q", flash)
	}
	if fake.drainCall != 0 {
		t.Fatalf("дренаж без чекбокса не доходит до сервиса, сервис видел %d", fake.drainCall)
	}
	fake = &fakeDispatch{drainErr: fmt.Errorf("%w: %s",
		actionAPI.ErrDispatchRefusedAG4, "Исполнение не подключено — реальных провайдеров нет: очередь не тронута, отправок ноль (AG-4).")}
	server = newDispatchTestServer(t, fake)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formPath,
		url.Values{"case_id": {testPreviewCaseID.String()}, "confirm_drain": {"yes"}}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("AG-4 отказ формы обязан ответить PRG, пришло %d", recorder.Code)
	}
	if flash := operationsFlashOf(recorder); !strings.Contains(flash, "очередь не тронута, отправок ноль") {
		t.Fatalf("AG-4 отказ обязан нести слова во flash: %q", flash)
	}
	fake = &fakeDispatch{drain: actionAPI.DispatchDrainResult{
		Command: "dispatch_queued_operations", Considered: 3, Dispatched: 2, Blocked: 1,
		Reports: []actionAPI.DispatchReportLineView{},
	}}
	server = newDispatchTestServer(t, fake)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formPath,
		url.Values{"case_id": {testPreviewCaseID.String()}, "confirm_drain": {"yes"}}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("успешный дренаж обязан ответить PRG 303, пришло %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); !strings.Contains(location, "drained=3") {
		t.Fatalf("PRG обязан нести флаг drained со счётчиком: %s", location)
	}
	if location := recorder.Header().Get("Location"); !strings.Contains(location, "/cases/"+testPreviewCaseID.String()+"/operations") {
		t.Fatalf("PRG обязан возвращать на страницу операций кейса: %s", location)
	}
	if fake.lastDrain == nil || fake.lastDrain.TenantID != testTenantID {
		t.Fatalf("команда дренажа обязана нести тенент дословно: %+v", fake.lastDrain)
	}
	fake = &fakeDispatch{}
	server = newDispatchTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, formPath, strings.NewReader(url.Values{
		"case_id": {testPreviewCaseID.String()}, "confirm_drain": {"yes"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://evil.example")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("чужой origin обязан отказать 403, пришло %d", recorder.Code)
	}
	if fake.drainCall != 0 {
		t.Fatalf("origin-отказ происходит ДО сервиса, сервис видел %d", fake.drainCall)
	}
}

// TestOperationReviewPageShowsTheDrainFormOnlyWhenBound — the page speaks
// the drain form exactly when the dispatch surface is bound (the honest
// absence: an unbound contour never pretends the command exists); the
// drained notice flag carries the focus discipline and the counter
// readback.
func TestOperationReviewPageShowsTheDrainFormOnlyWhenBound(t *testing.T) {
	fake := &fakeDispatch{}
	server := newDispatchTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		operationsPageRoot()+"/operations?drained=3", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("страница обязана отвечать 200, пришло %d", recorder.Code)
	}
	page := recorder.Body.String()
	for _, pin := range []string{
		"Запустить дренаж очереди отправок (операторная команда тенента)",
		`action="/app/tenants/` + testTenantID.String() + `/operations/drain"`,
		`name="case_id" value="` + testPreviewCaseID.String() + `"`,
		`name="confirm_drain" value="yes" required`,
		"Дренировать очередь",
		"Очередь отправок дренирована: рассмотрено заданий — 3",
		`id="page-notice"`,
		`tabindex="-1"`,
		"getElementById('page-notice').focus()",
	} {
		if !strings.Contains(page, pin) {
			t.Fatalf("страница с контуром обязана нести форму дренажа и уведомление (%s): %s", pin, page[:400])
		}
	}
	server = newOperationTestServer(t, &fakeAuthorization{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		operationsPageRoot()+"/operations", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("страница обязана отвечать 200, пришло %d", recorder.Code)
	}
	if page := recorder.Body.String(); strings.Contains(page, "Дренировать очередь") {
		t.Fatalf("неподключённый контур НЕ показывает форму дренажа: %s", page[:400])
	}
}

// ---------------------------------------------------------------------------
// The Story 3.7 (step-03) reconcile surface: the command gates and the page
// ---------------------------------------------------------------------------

// fakeReconcile serves the reconciliation-contour surface with the
// scripted answers and the captured commands (the fakeDispatch precedent
// — the counters pin the transport's own gate order: the strict decode
// refuses BEFORE the service).
type fakeReconcile struct {
	result  actionAPI.ReconcileResultView
	err     error
	calls   int
	lastCmd *actionAPI.ReconcileOperationsCommand
}

// The compile-time proof the fake satisfies the narrow transport contract.
var _ actionAPI.ReconcileService = (*fakeReconcile)(nil)

func (f *fakeReconcile) ReconcileOperations(_ context.Context, c actionAPI.ReconcileOperationsCommand) (actionAPI.ReconcileResultView, error) {
	f.calls++
	commandCopy := c
	f.lastCmd = &commandCopy
	if f.err != nil {
		return actionAPI.ReconcileResultView{}, f.err
	}
	return f.result, nil
}

// newReconcileTestServer binds the fake through the production
// BindReconcile — the narrow interface is type-asserted exactly the way
// the real wiring does it (the authorization fake rides along for the
// page reads).
func newReconcileTestServer(t *testing.T, reconcile *fakeReconcile) *Server {
	t.Helper()
	server := newOperationTestServer(t, &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID: testTenantID, CaseID: testPreviewCaseID,
		Confirmations: []actionAPI.ConfirmationCardView{},
		Operations:    []actionAPI.OperationCardView{},
	}})
	server.BindReconcile(reconcile)
	return server
}

// reconcileAPIPath builds the machine reconcile path of the fixture tenant.
func reconcileAPIPath() string {
	return "/v1/tenants/" + testTenantID.String() + "/operations/reconcile"
}

// TestReconcileRefusesTheFreeFieldBeforeTheService — the strict decode of
// the EMPTY body: a free «limit»/«force» field is the honest 400 BEFORE
// the service is ever called (the reconcile carries NO parameters — the
// bounded operator command of the domain); the wrong content type answers
// the media rule words.
func TestReconcileRefusesTheFreeFieldBeforeTheService(t *testing.T) {
	fake := &fakeReconcile{}
	server := newReconcileTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, reconcileAPIPath(), strings.NewReader(`{"limit":50}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("свободное поле обязано отвечать 400, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"body"`) {
		t.Fatalf("отказ обязан нести правило body: %s", recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, reconcileAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"content_type"`) {
		t.Fatalf("неверный content-type обязан отвечать 400 правилом content_type: %d %s", recorder.Code, recorder.Body.String())
	}
	if fake.calls != 0 {
		t.Fatalf("strict decode отказывает ДО сервиса, сервис видел %d", fake.calls)
	}
}

// TestReconcileAnswers201WithTheHonestReport — the empty strict body
// passes, the command carries the actor, the tenant and the server clock
// verbatim, and the 201 answer speaks the honest report: the counters and
// the per-line words (every considered target leaves its line — FR-30).
func TestReconcileAnswers201WithTheHonestReport(t *testing.T) {
	lineText := actionDomain.ReconciliationExhaustedText
	fake := &fakeReconcile{result: actionAPI.ReconcileResultView{
		Command: "reconcile_unknown_operations", Considered: 2, Queried: 1, Decided: 1, Skipped: 1,
		Reports: []actionAPI.ReconcileReportLineView{
			{OperationID: testOperationID, AttemptNo: 1, Text: actionDomain.ReconciliationCommittedText},
			{OperationID: testOperationID, AttemptNo: 0, Text: lineText},
		},
	}}
	server := newReconcileTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, reconcileAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("reconcile обязан отвечать 201 с отчётом, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, pin := range []string{
		`"reconcile":{`,
		`"considered":2`,
		`"queried":1`,
		`"decided":1`,
		`"skipped":1`,
		actionDomain.ReconciliationCommittedText,
		lineText,
	} {
		if !strings.Contains(body, pin) {
			t.Fatalf("отчёт обязан нести счётчики и построчные слова (%s): %s", pin, body)
		}
	}
	if fake.lastCmd == nil || fake.lastCmd.Actor.ID != testPrincipalID ||
		fake.lastCmd.TenantID != testTenantID || !fake.lastCmd.Now.Equal(time.Unix(1, 0).UTC()) {
		t.Fatalf("команда обязана нести актёра, тенент и серверное время дословно: %+v", fake.lastCmd)
	}
}

// TestReconcileSpeaksTheAG4RefusalWords — the honest production refusal
// of the nil read-only adapter answers 403 with the EXACT words the app
// attached at the wrap point (AG-4: the pending targets stay, zero
// queries — never a masked success); the reconcile never touched the
// queue.
func TestReconcileSpeaksTheAG4RefusalWords(t *testing.T) {
	fake := &fakeReconcile{err: fmt.Errorf("%w: %s",
		actionAPI.ErrReconcileRefusedAG4, actionDomain.ReconciliationNotWiredText)}
	server := newReconcileTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, reconcileAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("AG-4 отказ обязан отвечать 403, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":"reconciliation_not_wired"`) {
		t.Fatalf("AG-4 отказ обязан нести код reconciliation_not_wired: %s", body)
	}
	if !strings.Contains(body, "очередь сверки не тронута, запросов ноль") {
		t.Fatalf("AG-4 отказ обязан нести честные слова приложения дословно: %s", body)
	}
}

// TestReconcileSpeaksTheScopeAndMissingWords — the rights gate of the app
// answers 403 with the scope words the app attached (the operations scope
// of the operator), the identity NotFound keeps the plain 404 (no
// disclosure that the tenant reconciles actions at all).
func TestReconcileSpeaksTheScopeAndMissingWords(t *testing.T) {
	fake := &fakeReconcile{err: fmt.Errorf("%w: %s", actionDomain.ErrForbidden,
		"Команда reconcile требует полномочия operations: сверка unknown-операций — операторная команда, участники без операторного полномочия очередь сверки не трогают.")}
	server := newReconcileTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, reconcileAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("отказ полномочия обязан отвечать 403, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "полномочия operations") {
		t.Fatalf("отказ полномочия обязан нести слова скоупа дословно: %s", recorder.Body.String())
	}
	fake = &fakeReconcile{err: actionDomain.ErrNotFound}
	server = newReconcileTestServer(t, fake)
	request = httptest.NewRequest(http.MethodPost, reconcileAPIPath(), strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("не-участник обязан отвечать 404, пришло %d", recorder.Code)
	}
}

// TestReconcileFormRequiresTheCheckboxAndLandsPRG — the page reconcile
// form is the PRG family (the drain form precedent): the explicit
// checkbox is the UI discipline of the operator act, the missing
// checkbox refuses BEFORE the service, the AG-4 refusal words ride the
// flash, the success lands PRG with the considered counter; the foreign
// origin never reaches the service.
func TestReconcileFormRequiresTheCheckboxAndLandsPRG(t *testing.T) {
	formPath := "/app/tenants/" + testTenantID.String() + "/operations/reconcile"
	fake := &fakeReconcile{}
	server := newReconcileTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formPath,
		url.Values{"case_id": {testPreviewCaseID.String()}}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("сверка без чекбокса обязана ответить PRG 303, пришло %d", recorder.Code)
	}
	if flash := operationsFlashOf(recorder); !strings.Contains(flash, "чекбокс согласия") {
		t.Fatalf("сверка без чекбокса обязана нести честные слова: %q", flash)
	}
	if fake.calls != 0 {
		t.Fatalf("сверка без чекбокса не доходит до сервиса, сервис видел %d", fake.calls)
	}
	fake = &fakeReconcile{err: fmt.Errorf("%w: %s",
		actionAPI.ErrReconcileRefusedAG4, actionDomain.ReconciliationNotWiredText)}
	server = newReconcileTestServer(t, fake)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formPath,
		url.Values{"case_id": {testPreviewCaseID.String()}, "confirm_reconcile": {"yes"}}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("AG-4 отказ формы обязан ответить PRG, пришло %d", recorder.Code)
	}
	if flash := operationsFlashOf(recorder); !strings.Contains(flash, "очередь сверки не тронута, запросов ноль") {
		t.Fatalf("AG-4 отказ обязан нести слова во flash: %q", flash)
	}
	fake = &fakeReconcile{result: actionAPI.ReconcileResultView{
		Command: "reconcile_unknown_operations", Considered: 3, Queried: 2, Decided: 2, Skipped: 1,
		Reports: []actionAPI.ReconcileReportLineView{},
	}}
	server = newReconcileTestServer(t, fake)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, operationsFormOf(http.MethodPost, formPath,
		url.Values{"case_id": {testPreviewCaseID.String()}, "confirm_reconcile": {"yes"}}))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("успешная сверка обязана ответить PRG 303, пришло %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); !strings.Contains(location, "reconciled=3") {
		t.Fatalf("PRG обязан нести флаг reconciled со счётчиком: %s", location)
	}
	if location := recorder.Header().Get("Location"); !strings.Contains(location, "/cases/"+testPreviewCaseID.String()+"/operations") {
		t.Fatalf("PRG обязан возвращать на страницу операций кейса: %s", location)
	}
	if fake.lastCmd == nil || fake.lastCmd.TenantID != testTenantID {
		t.Fatalf("команда сверки обязана нести тенент дословно: %+v", fake.lastCmd)
	}
	fake = &fakeReconcile{}
	server = newReconcileTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, formPath, strings.NewReader(url.Values{
		"case_id": {testPreviewCaseID.String()}, "confirm_reconcile": {"yes"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://evil.example")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("чужой origin обязан отказать 403, пришло %d", recorder.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("origin-отказ происходит ДО сервиса, сервис видел %d", fake.calls)
	}
}

// TestOperationReviewPageShowsTheReconcileFormAndTheAttempts — the page
// speaks the reconcile form exactly when the reconcile surface is bound
// (the honest absence: an unbound contour never pretends the command
// exists); the reconciled notice flag carries the focus discipline and
// the counter readback; the operation card renders the reconciliation
// attempts in the CLOSED details with the verdict words, the resting
// word of the un-recorded attempt and the absent-contour word of the
// living boundary (AC-13).
func TestOperationReviewPageShowsTheReconcileFormAndTheAttempts(t *testing.T) {
	card := fixtureOperationCard()
	card.State = actionDomain.OperationUnknown
	card.NextStepText = actionDomain.OperationNextStepText(actionDomain.OperationUnknown)
	card.ReconcileAbsentText = actionDomain.OperationReconcileAbsentText
	card.ReconciliationAttempts = []actionAPI.ReconciliationAttemptView{{
		AttemptNo:        1,
		LeaseCreatedAt:   time.Unix(9, 0).UTC(),
		LeaseExpiresAt:   time.Unix(39, 0).UTC(),
		NextEligibleAt:   time.Unix(69, 0).UTC(),
		Verdict:          actionDomain.ReconciliationVerdictCommitted,
		VerdictRecorded:  true,
		VerdictText:      actionDomain.ReconciliationVerdictText(actionDomain.ReconciliationVerdictCommitted),
		CorrelationID:    "syn-0123456789abcdef",
		ExternalObjectID: "msg-42",
		VerdictCreatedAt: time.Unix(10, 0).UTC(),
	}, {
		AttemptNo:       2,
		LeaseCreatedAt:  time.Unix(79, 0).UTC(),
		LeaseExpiresAt:  time.Unix(109, 0).UTC(),
		NextEligibleAt:  time.Unix(139, 0).UTC(),
		VerdictRecorded: false,
		VerdictText:     actionDomain.ReconciliationAttemptRestingText,
	}}
	server := newOperationTestServer(t, &fakeAuthorization{view: actionAPI.AuthorizationView{
		TenantID: testTenantID, CaseID: testPreviewCaseID,
		Confirmations: []actionAPI.ConfirmationCardView{},
		Operations:    []actionAPI.OperationCardView{card},
	}})
	server.BindReconcile(&fakeReconcile{})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		operationsPageRoot()+"/operations?reconciled=3", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("страница обязана отвечать 200, пришло %d", recorder.Code)
	}
	page := recorder.Body.String()
	for _, pin := range []string{
		"Запустить сверку unknown-операций (операторная команда тенента)",
		`action="/app/tenants/` + testTenantID.String() + `/operations/reconcile"`,
		`name="confirm_reconcile" value="yes" required`,
		"Запустить сверку",
		"Сверка unknown-операций выполнена: рассмотрено целей — 3",
		`id="page-notice"`,
		`tabindex="-1"`,
		"getElementById('page-notice').focus()",
		"Попытки сверки (прогрессивный аудит, закрыто по умолчанию)",
		"вердикт committed: Сверка подтвердила терминальный успех (committed): внешний объект существует, структурные evidence (идентификатор, версия, время) записаны в квитанции v2&#43;.",
		"вердикт не записан: " + actionDomain.ReconciliationAttemptRestingText,
		actionDomain.OperationReconcileAbsentText,
		"Корреляция провайдера: syn-0123456789abcdef",
		"Внешний объект сверки: msg-42",
	} {
		if !strings.Contains(page, pin) {
			t.Fatalf("страница с контуром обязана нести форму сверки, попытки и уведомление (%s): %s", pin, page[:400])
		}
	}
	server = newOperationTestServer(t, &fakeAuthorization{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		operationsPageRoot()+"/operations", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("страница обязана отвечать 200, пришло %d", recorder.Code)
	}
	if page := recorder.Body.String(); strings.Contains(page, "Запустить сверку") {
		t.Fatalf("неподключённый контур НЕ показывает форму сверки: %s", page[:400])
	}
}
