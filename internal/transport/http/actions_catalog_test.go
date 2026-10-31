package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionCatalog "github.com/staffy-co/staffy/internal/action/catalog"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/identity/api"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// fakeActionCatalog is the transport fake of the read-only catalog
// surface: the fake never runs the admission pass (the guard is the
// domain's unit truth); it serves the fixture dictionary and the honest
// refusals of the read gate.
type fakeActionCatalog struct {
	defs    []actionDomain.ActionDefinition
	listErr error
	getErr  error
	lastGet *actionGetCall
}

// The compile-time proof the fake satisfies the transport contract.
var _ actionAPI.ActionCatalogService = (*fakeActionCatalog)(nil)

type actionGetCall struct {
	tenantID     id.UUID
	definitionID string
	version      string
}

func (f *fakeActionCatalog) List(context.Context, domain.Principal, id.UUID) ([]actionDomain.ActionDefinition, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	// The real service serves ListFrom(defs) — the newest version of every
	// ID; the fake mirrors that semantic so the transport sees the same
	// shape the app layer would serve.
	return actionCatalog.ListFrom(f.defs), nil
}

func (f *fakeActionCatalog) Get(_ context.Context, _ domain.Principal, tenantID id.UUID, definitionID, version string) (actionDomain.ActionDefinition, error) {
	f.lastGet = &actionGetCall{tenantID: tenantID, definitionID: definitionID, version: version}
	if f.getErr != nil {
		return actionDomain.ActionDefinition{}, f.getErr
	}
	for _, def := range f.defs {
		if def.ID == definitionID && def.Version == version {
			return def, nil
		}
	}
	return actionDomain.ActionDefinition{}, actionDomain.ErrNotFound
}

// actionFixtureDefinition builds the guard-valid fixture of the slice:
// the SMS notification action of the synthetic telephony pack pair —
// the full condition system and the delivery contract of the words.
func actionFixtureDefinition(id, version string) actionDomain.ActionDefinition {
	return actionDomain.ActionDefinition{
		ID:      id,
		Version: version,
		Name:    "Отправка SMS клиенту",
		Purpose: "Уведомить клиента о готовности заказа",
		TypedInputSchema: []actionDomain.FieldSchema{
			{Name: "phone", Label: "Телефон получателя", Type: actionDomain.FieldText, Required: true, Description: "Номер в формате E.164"},
			{Name: "template_url", Label: "Ссылка на шаблон", Type: actionDomain.FieldURL, Secret: true, Description: "Шаблон сообщения"},
		},
		TypedOutputSchema: []actionDomain.FieldSchema{
			{Name: "message_id", Label: "Идентификатор сообщения", Type: actionDomain.FieldText},
		},
		PackRef:            actionDomain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
		RequiredCapability: actionDomain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "command.send-sms"},
		OwningSystem:       "erp",
		Conditions: actionDomain.Conditions{
			RiskClass:                "medium",
			AutonomyClass:            "assisted",
			AllowedConversationModes: []string{"claim", "cancellation"},
			AllowedRoles:             []string{"owner", "configurator"},
			RequiredFacts:            []actionDomain.FactRequirement{{MaterialField: "order_status", Freshness: "fresh"}},
			Confirmation:             "required",
			RequiredGates:            []string{"OQ-7A", "AG-1"},
			TimeoutMS:                30000,
			MaterialityClass:         "material",
			ConsumptionRule:          "single_use",
			AllowedStages:            []string{"closed_account_test", "controlled_real_customer"},
		},
		DeliveryContract: actionDomain.DeliveryContract{
			CapabilityClass:       "provider_idempotent",
			IdempotencyScope:      "connector-ack",
			TerminalEvidence:      "delivery-receipt",
			ReconciliationQuery:   "query.trunk-status",
			CancelCompensationSem: "не компенсируется — отказ до подтверждения",
		},
	}
}

// actionScopeWords mirrors the app layer wrap of the honest 403: the
// words of the two scopes that DO open the surface, attached to the
// module error at the refusal point.
const actionScopeWords = "Чтение каталога действий требует полномочия integrations или configuration: словарь действий — материал настройки интеграций и конфигурации, оператору и аудиту он не открывается до вычисления доступности."

func newActionTestServer(t *testing.T, fake *fakeActionCatalog, resolver fixedResolver) *Server {
	t.Helper()
	identity := fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) {
			return domain.Workspace{}, nil
		},
	}
	server, err := New(identity, resolver, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindActionCatalog(fake)
	return server
}

func TestActionCatalogListAPIServesNewestVersionsOfEveryID(t *testing.T) {
	fake := &fakeActionCatalog{defs: []actionDomain.ActionDefinition{
		// The dictionary is maintained newest-first (the 1-8 convention):
		// ListFrom serves the first occurrence of every ID.
		actionFixtureDefinition("notify.order-ready", "1.1.0"),
		actionFixtureDefinition("notify.order-ready", "1.0.0"),
	}}
	server := newActionTestServer(t, fake, fixedResolver{principal: domain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/action-definitions", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		ActionDefinitions []actionDefinitionView `json:"action_definitions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.ActionDefinitions) != 1 {
		t.Fatalf("List обязан отдать ТОЛЬКО новейшую версию каждого ID, got %d", len(body.ActionDefinitions))
	}
	view := body.ActionDefinitions[0]
	if view.ID != "notify.order-ready" || view.Version != "1.1.0" {
		t.Fatalf("ожидалась новейшая пин-пара, got %s@%s", view.ID, view.Version)
	}
	if view.Conditions.RiskClass != "medium" || view.OwningSystem != "erp" || view.Conditions.TimeoutMS != 30000 {
		t.Fatalf("условия определения обязаны приехать целиком: %+v", view.Conditions)
	}
	if len(view.Conditions.RequiredFacts) != 1 || view.Conditions.RequiredFacts[0].MaterialField != "order_status" {
		t.Fatalf("материальные зависимости обязаны приехать: %+v", view.Conditions.RequiredFacts)
	}
	if view.DeliveryContract.CapabilityClass != "provider_idempotent" || view.DeliveryContract.ReconciliationQuery != "query.trunk-status" {
		t.Fatalf("delivery-контракт обязателен в DTO: %+v", view.DeliveryContract)
	}
	if len(view.TypedInputSchema) != 2 || !view.TypedInputSchema[1].Secret {
		t.Fatalf("схема входа обязана нести оба поля с пометкой секрета: %+v", view.TypedInputSchema)
	}
}

func TestActionCatalogGetAPIPinsExactVersion(t *testing.T) {
	fake := &fakeActionCatalog{defs: []actionDomain.ActionDefinition{
		actionFixtureDefinition("notify.order-ready", "1.1.0"),
		actionFixtureDefinition("notify.order-ready", "1.0.0"),
	}}
	server := newActionTestServer(t, fake, fixedResolver{principal: domain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/action-definitions/notify.order-ready/1.0.0", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Definition actionDefinitionView `json:"action_definition"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Definition.Version != "1.0.0" {
		t.Fatalf("пин обязан вернуть точную версию 1.0.0 (AC-8), got %s", body.Definition.Version)
	}
	if fake.lastGet == nil || fake.lastGet.tenantID != testTenantID || fake.lastGet.definitionID != "notify.order-ready" || fake.lastGet.version != "1.0.0" {
		t.Fatalf("сервис обязан получить точный пин: %+v", fake.lastGet)
	}
}

func TestActionCatalogGetAPIRefusesMalformedAndUnknownPins(t *testing.T) {
	server := newActionTestServer(t, &fakeActionCatalog{}, fixedResolver{principal: domain.Principal{ID: testPrincipalID}})
	for _, pin := range []string{
		"/v1/tenants/" + testTenantID.String() + "/action-definitions/BIG-ID/1.0.0",
		"/v1/tenants/" + testTenantID.String() + "/action-definitions/notify.order-ready/1.0",
		"/v1/tenants/" + testTenantID.String() + "/action-definitions/notify.order-ready/latest",
		"/v1/tenants/" + testTenantID.String() + "/action-definitions/notify.order-ready/9.9.9",
		"/v1/tenants/" + testTenantID.String() + "/action-definitions/%20/1.0.0",
	} {
		r := httptest.NewRequest(http.MethodGet, pin, nil)
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("пин %s обязан получить честный 404, got %d body=%s", pin, w.Code, w.Body.String())
		}
	}
}

func TestActionCatalogListAPIRequiresIdentity(t *testing.T) {
	server := newActionTestServer(t, &fakeActionCatalog{defs: []actionDomain.ActionDefinition{actionFixtureDefinition("notify.order-ready", "1.0.0")}}, fixedResolver{})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/action-definitions", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("не подтверждённая личность = 401, got %d", w.Code)
	}
}

func TestActionCatalogNonMemberIsPlain404BeforeMaterial(t *testing.T) {
	fake := &fakeActionCatalog{
		defs:    []actionDomain.ActionDefinition{actionFixtureDefinition("notify.order-ready", "1.0.0")},
		listErr: actionDomain.ErrNotFound,
	}
	server := newActionTestServer(t, fake, fixedResolver{principal: domain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/action-definitions", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("не-участник = честный 404, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "Отправка SMS") {
		t.Fatalf("материал каталога не покидает сервис при 404: %s", w.Body.String())
	}
}

func TestActionCatalogScopeRefusalIs403WithWordsBeforeMaterial(t *testing.T) {
	fake := &fakeActionCatalog{
		defs:    []actionDomain.ActionDefinition{actionFixtureDefinition("notify.order-ready", "1.0.0")},
		listErr: fmt.Errorf("%w: %s", actionDomain.ErrForbidden, actionScopeWords),
		getErr:  fmt.Errorf("%w: %s", actionDomain.ErrForbidden, actionScopeWords),
	}
	server := newActionTestServer(t, fake, fixedResolver{principal: domain.Principal{ID: testPrincipalID}})
	for _, path := range []string{
		"/v1/tenants/" + testTenantID.String() + "/action-definitions",
		"/v1/tenants/" + testTenantID.String() + "/action-definitions/notify.order-ready/1.0.0",
		"/app/tenants/" + testTenantID.String() + "/actions",
	} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: оператор без скоупа = 403, got %d body=%s", path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "integrations") || !strings.Contains(w.Body.String(), "configuration") {
			t.Fatalf("%s: 403 обязан говорить слова скоупа ДО материала: %s", path, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "Отправка SMS") {
			t.Fatalf("%s: материал каталога не покидает сервис при 403: %s", path, w.Body.String())
		}
	}
}

func TestActionCatalogEmptyListIsHonestEmptyArray(t *testing.T) {
	server := newActionTestServer(t, &fakeActionCatalog{}, fixedResolver{principal: domain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/action-definitions", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("честная пустота = 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"action_definitions":[]`) {
		t.Fatalf("пустой список обязан быть пустым массивом, не null: %s", w.Body.String())
	}
}

func TestActionsPageRendersCardsWithProgressiveDisclosure(t *testing.T) {
	fake := &fakeActionCatalog{defs: []actionDomain.ActionDefinition{
		actionFixtureDefinition("notify.order-ready", "1.1.0"),
	}}
	server := newActionTestServer(t, fake, fixedResolver{principal: domain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/actions", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"Каталог действий",
		"Отправка SMS клиенту",
		"notify.order-ready",
		"1.1.0",
		"medium",
		"closed_account_test; controlled_real_customer",
		"staffy-synthetic-telephony@1.1.0/command.send-sms",
		"auto-parts@2.0.0",
		"erp",
		"<details class=\"action-conditions\"><summary>Условия действия</summary>",
		"<details class=\"action-delivery\"><summary>Delivery-контракт</summary>",
		"<details class=\"action-schema\"><summary>Схемы полей</summary>",
		"assisted",
		"claim; cancellation",
		"owner; configurator",
		"order_status (fresh)",
		"required",
		"OQ-7A; AG-1",
		"30000 мс",
		"material",
		"single_use",
		"provider_idempotent",
		"query.trunk-status",
		"Телефон получателя",
		"секрет: да",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("страница каталога обязана нести %q: %s", want, body)
		}
	}
	if strings.Contains(body, "Определений пока нет") {
		t.Fatalf("карточки есть — пустое состояние не должно рендериться")
	}
}

func TestActionsPageEmptyStateExplainsReasonByteExact(t *testing.T) {
	server := newActionTestServer(t, &fakeActionCatalog{}, fixedResolver{principal: domain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/actions", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "Определений пока нет") {
		t.Fatalf("пустая страница обязана назвать отсутствие: %s", body)
	}
	if !strings.Contains(body, actionCatalogEmptyText) {
		t.Fatalf("пустое состояние обязано объяснять причину словами (AG-4/AC-11), байт-в-байт: %s", body)
	}
	if strings.Contains(body, "<details") {
		t.Fatalf("без определений нет карточек и раскрытий: %s", body)
	}
}

func TestActionsPageRequiresIdentity(t *testing.T) {
	server := newActionTestServer(t, &fakeActionCatalog{}, fixedResolver{})
	r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/actions", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("не подтверждённая личность = 401, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
