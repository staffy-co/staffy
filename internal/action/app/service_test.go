package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/artemiosu/staffy/internal/action/domain"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

var (
	testActor  = identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-0000000000a1")}
	testTenant = id.MustParse("01994f64-6c00-7000-8000-0000000000a2")
)

// scopeIdentity answers the membership and scope questions of the read gate
// without a store: a non-member gets the honest 404 (ErrNotFound); a member
// is decided by the scope map (allowed → nil, missing → ErrForbidden).
type scopeIdentity struct {
	identityAPI.MembershipService
	member bool
	scopes map[identityDomain.Scope]bool
}

func (f scopeIdentity) Authorize(_ context.Context, _ identityDomain.Principal, _ id.UUID, required identityDomain.Scope) error {
	if !f.member {
		return identityDomain.ErrNotFound
	}
	if f.scopes[required] {
		return nil
	}
	return identityDomain.ErrForbidden
}

// infraIdentity relays an infrastructure failure of the identity port —
// the gate must propagate it, never mask it as 403/404.
type infraIdentity struct {
	identityAPI.MembershipService
	failure error
}

func (f infraIdentity) Authorize(context.Context, identityDomain.Principal, id.UUID, identityDomain.Scope) error {
	return f.failure
}

// The resolvers of the unit circuit: everything resolves (the resolution
// matrix itself lives in the domain and adapter tests).
type acceptAllPacks struct{}

func (acceptAllPacks) Get(string, string) bool { return true }

type acceptAllCaps struct{}

func (acceptAllCaps) Capability(string, string, string) (domain.CapabilityFacts, bool) {
	return domain.CapabilityFacts{Idempotent: true, Reconciliation: "объявлена"}, true
}

// fixtureDefinitions is the fixture dictionary of the test composition
// root: two versions of one ID (the pin semantics) plus a second query
// definition. Both shapes are guard-valid — the fixtures are honest.
func fixtureDefinitions() []domain.ActionDefinition {
	return []domain.ActionDefinition{
		{
			ID: "quote.send-price-quote", Version: "1.2.0",
			Name: "Отправка ценового предложения", Purpose: "Ответить клиенту ценой по свежей карточке товара.",
			TypedInputSchema: []domain.FieldSchema{
				{Name: "product_ref", Label: "Товар", Type: domain.FieldText, Required: true, Description: "Ссылка на карточку товара."},
			},
			TypedOutputSchema: []domain.FieldSchema{
				{Name: "quote_word", Label: "Слово цены", Type: domain.FieldText, Required: true, Description: "Ценовое слово для ответа клиенту."},
			},
			PackRef:            domain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
			RequiredCapability: domain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "command.send-sms"},
			OwningSystem:       "catalog",
			Conditions: domain.Conditions{
				RiskClass: "medium", AutonomyClass: "assisted",
				AllowedConversationModes: []string{"safety", "claim", "unclassified"},
				AllowedRoles:             []string{"owner", "operator"},
				RequiredFacts: []domain.FactRequirement{
					{MaterialField: "price", Freshness: "fresh"},
					{MaterialField: "availability", Freshness: "fresh"},
				},
				Confirmation: "required", RequiredGates: []string{"OQ-19", "AG-2"},
				TimeoutMS: 15000, MaterialityClass: "material", ConsumptionRule: "single_use",
				AllowedStages: []string{"synthetic", "closed_account_test"},
			},
			DeliveryContract: domain.DeliveryContract{
				CapabilityClass:  "provider_idempotent",
				IdempotencyScope: "ключ отправки предложения", TerminalEvidence: "квитанция доставки провайдера",
				ReconciliationQuery: "query.trunk-status", CancelCompensationSem: "отмена недоступна после accepted",
			},
		},
		{
			ID: "quote.send-price-quote", Version: "1.1.0",
			Name: "Отправка ценового предложения", Purpose: "Ответить клиенту ценой по карточке товара (первая версия слова).",
			TypedInputSchema: []domain.FieldSchema{
				{Name: "product_ref", Label: "Товар", Type: domain.FieldText, Required: true, Description: "Ссылка на карточку товара."},
			},
			TypedOutputSchema: []domain.FieldSchema{
				{Name: "quote_word", Label: "Слово цены", Type: domain.FieldText, Required: true, Description: "Ценовое слово для ответа клиенту."},
			},
			PackRef:            domain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
			RequiredCapability: domain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "command.send-sms"},
			OwningSystem:       "catalog",
			Conditions: domain.Conditions{
				RiskClass: "medium", AutonomyClass: "assisted",
				AllowedConversationModes: []string{"safety", "claim"},
				AllowedRoles:             []string{"owner", "operator"},
				RequiredFacts:            []domain.FactRequirement{{MaterialField: "price", Freshness: "fresh"}},
				Confirmation:             "required", RequiredGates: []string{"OQ-19", "AG-2"},
				TimeoutMS: 15000, MaterialityClass: "material", ConsumptionRule: "single_use",
				AllowedStages: []string{"synthetic", "closed_account_test"},
			},
			DeliveryContract: domain.DeliveryContract{
				CapabilityClass:  "provider_idempotent",
				IdempotencyScope: "ключ отправки предложения", TerminalEvidence: "квитанция доставки провайдера",
				ReconciliationQuery: "query.trunk-status", CancelCompensationSem: "отмена недоступна после accepted",
			},
		},
		{
			ID: "catalog.check-availability", Version: "1.0.0",
			Name: "Проверка наличия детали", Purpose: "Ответить клиенту наличием по карточке товара.",
			TypedInputSchema: []domain.FieldSchema{
				{Name: "product_ref", Label: "Товар", Type: domain.FieldText, Required: true, Description: "Ссылка на карточку товара."},
			},
			TypedOutputSchema: []domain.FieldSchema{
				{Name: "availability_word", Label: "Слово наличия", Type: domain.FieldText, Required: true, Description: "Слово наличия для ответа клиенту."},
			},
			PackRef:            domain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
			RequiredCapability: domain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "query.trunk-status"},
			OwningSystem:       "catalog",
			Conditions: domain.Conditions{
				RiskClass: "low", AutonomyClass: "copilot",
				AllowedConversationModes: []string{"safety", "claim", "unclassified"},
				AllowedRoles:             []string{"owner", "operator", "configurator"},
				RequiredFacts:            []domain.FactRequirement{{MaterialField: "availability", Freshness: "fresh"}},
				Confirmation:             "not_required", RequiredGates: []string{"OQ-19"},
				TimeoutMS: 5000, MaterialityClass: "immaterial", ConsumptionRule: "unlimited",
				AllowedStages: []string{"synthetic", "closed_account_test"},
			},
			DeliveryContract: domain.DeliveryContract{
				CapabilityClass:  "query_reconcilable",
				IdempotencyScope: "ключ проверки наличия", TerminalEvidence: "результат read-only запроса",
				ReconciliationQuery: "query.trunk-status", CancelCompensationSem: "не применимо (read-only)",
			},
		},
	}
}

func newTestService(identity identityAPI.MembershipService, defs []domain.ActionDefinition) *Service {
	return New(identity, acceptAllPacks{}, acceptAllCaps{}, defs)
}

// TestListRefusesNonMemberWithTheHonest404 — не-участник тенанта получает
// честный 404 ДО любого материала каталога (не-раскрытие самого факта
// каталога).
func TestListRefusesNonMemberWithTheHonest404(t *testing.T) {
	service := newTestService(scopeIdentity{member: false}, fixtureDefinitions())
	defs, err := service.List(context.Background(), testActor, testTenant)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("не-участник обязан получать ErrNotFound (404), получено: %v", err)
	}
	if defs != nil {
		t.Fatal("материал каталога не выдаётся не-участнику")
	}
}

// TestListRefusesMemberWithoutScopeWithThe403Words — участник без скоупов
// integrations|configuration (оператор: operations+claims) получает честный
// 403 СЛОВАМИ скоупов ДО материала: слова называют оба допускающих
// полномочия.
func TestListRefusesMemberWithoutScopeWithThe403Words(t *testing.T) {
	operator := scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{
		identityDomain.Operations: true,
		identityDomain.Claims:     true,
	}}
	service := newTestService(operator, fixtureDefinitions())
	defs, err := service.List(context.Background(), testActor, testTenant)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("участник без скоупа обязан получать ErrForbidden (403), получено: %v", err)
	}
	if defs != nil {
		t.Fatal("материал каталога не выдаётся до прохождения скоуп-гейта")
	}
	for _, word := range []string{"integrations", "configuration"} {
		if !strings.Contains(err.Error(), word) {
			t.Fatalf("403 обязан говорить словами скоупов, слово %q не найдено в: %v", word, err)
		}
	}
}

// TestListServesIntegrationAdminAndConfigurator — оба полномочия гейта
// открывают чтение (Integration Admin — integrations; Configurator —
// configuration).
func TestListServesIntegrationAdminAndConfigurator(t *testing.T) {
	integrationAdmin := scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Integrations: true}}
	configurator := scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Configuration: true}}
	for name, identity := range map[string]identityAPI.MembershipService{
		"integration-admin": integrationAdmin,
		"configurator":      configurator,
	} {
		t.Run(name, func(t *testing.T) {
			service := newTestService(identity, fixtureDefinitions())
			defs, err := service.List(context.Background(), testActor, testTenant)
			if err != nil {
				t.Fatalf("чтение обязано открываться, получено: %v", err)
			}
			if len(defs) != 2 {
				t.Fatalf("List отдаёт новейшую версию каждого ID — 2 записи, получено: %d", len(defs))
			}
		})
	}
}

// TestGetPinsTheExactOldVersion — пин точной версии: старая версия
// резолвимая и неизменяемая (AC-8), неизвестный пин — честный 404.
func TestGetPinsTheExactOldVersion(t *testing.T) {
	owner := scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{
		identityDomain.Integrations: true, identityDomain.Configuration: true,
	}}
	service := newTestService(owner, fixtureDefinitions())

	pinned, err := service.Get(context.Background(), testActor, testTenant, "quote.send-price-quote", "1.1.0")
	if err != nil {
		t.Fatalf("пин старой версии 1.1.0 обязан резолвиться: %v", err)
	}
	if pinned.Version != "1.1.0" || pinned.Purpose != "Ответить клиенту ценой по карточке товара (первая версия слова)." {
		t.Fatalf("пин обязан вернуть точную старую версию, получено: %s", pinned.Version)
	}
	if _, err := service.Get(context.Background(), testActor, testTenant, "quote.send-price-quote", "9.9.9"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("неизвестный пин обязан давать ErrNotFound, получено: %v", err)
	}
	if _, err := service.Get(context.Background(), testActor, testTenant, "unknown.action", "1.0.0"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("неизвестный ID обязан давать ErrNotFound, получено: %v", err)
	}
}

// TestGetAppliesTheSameReadGateBeforeMaterial — гейт чтения идентичен на
// пин-пути: не-участник 404, участник без скоупа 403 словами — ДО материала.
func TestGetAppliesTheSameReadGateBeforeMaterial(t *testing.T) {
	nonMember := scopeIdentity{member: false}
	service := newTestService(nonMember, fixtureDefinitions())
	if _, err := service.Get(context.Background(), testActor, testTenant, "quote.send-price-quote", "1.1.0"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("не-участник на пин-пути обязан получать 404, получено: %v", err)
	}
	auditor := scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Audit: true}}
	service = newTestService(auditor, fixtureDefinitions())
	_, err := service.Get(context.Background(), testActor, testTenant, "quote.send-price-quote", "1.1.0")
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("аудит-участник без скоупов обязан получать 403, получено: %v", err)
	}
	if !strings.Contains(err.Error(), "integrations") {
		t.Fatalf("403 на пин-пути обязан говорить словами скоупов: %v", err)
	}
}

// TestInvalidCallerIdentityIsTheHonest404 — невалидный актор/тенант не
// доходит до identity-порта вовсе.
func TestInvalidCallerIdentityIsTheHonest404(t *testing.T) {
	service := newTestService(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Integrations: true}}, fixtureDefinitions())
	if _, err := service.List(context.Background(), identityDomain.Principal{}, testTenant); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("невалидный актор обязан давать 404, получено: %v", err)
	}
	var zeroTenant id.UUID
	if _, err := service.List(context.Background(), testActor, zeroTenant); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("невалидный тенант обязан давать 404, получено: %v", err)
	}
}

// TestInfrastructureFailurePropagatesHonestly — сбой identity-порта не
// маскируется под 403/404.
func TestInfrastructureFailurePropagatesHonestly(t *testing.T) {
	infraFailure := errors.New("identity port unreachable")
	service := newTestService(infraIdentity{failure: infraFailure}, fixtureDefinitions())
	if _, err := service.List(context.Background(), testActor, testTenant); !errors.Is(err, infraFailure) {
		t.Fatalf("инфраструктурный сбой обязан пробрасываться честно, получено: %v", err)
	}
}

// TestValidateBinaryCatalogPassesVacuouslyOnTheEmptyBinary — честная пустота
// бинарного словаря проходит полный проход допуска vacuously (AG-4);
// инъекционный fixture-словарь не подменяет предмет прохода.
func TestValidateBinaryCatalogPassesVacuouslyOnTheEmptyBinary(t *testing.T) {
	service := newTestService(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Integrations: true}}, fixtureDefinitions())
	if err := service.ValidateBinaryCatalog(); err != nil {
		t.Fatalf("честный пустой бинарный каталог обязан проходить vacuously, получено: %v", err)
	}
}
