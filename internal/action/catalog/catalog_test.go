package catalog

import (
	"testing"

	"github.com/artemiosu/staffy/internal/action/domain"
)

// fixtureDefinitions — два определения + две версии одного ID
// (1.2.0 новейшая — первой, 1.1.0 закреплена; второе действие другой ID).
// Порядок новейшими-первыми — конвенция каталога 1-8: List берёт первую
// запись каждого ID.
func fixtureDefinitions() []domain.ActionDefinition {
	base := domain.ActionDefinition{
		ID:      "quote.send-price-quote",
		Version: "1.2.0",
		Name:    "Отправка ценового предложения",
		Purpose: "Ответить клиенту ценой по свежей карточке товара.",
		TypedInputSchema: []domain.FieldSchema{
			{Name: "product_ref", Label: "Товар", Type: domain.FieldText, Required: true, Description: "Ссылка на карточку товара."},
		},
		TypedOutputSchema: []domain.FieldSchema{
			{Name: "quote_word", Label: "Слово цены", Type: domain.FieldText, Required: true, Description: "Ценовое слово для ответа клиенту."},
		},
		PackRef:            domain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
		RequiredCapability: domain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "query.trunk-status"},
		OwningSystem:       "catalog",
		Conditions: domain.Conditions{
			RiskClass:                domain.RiskLow,
			AutonomyClass:            domain.AutonomyCopilot,
			AllowedConversationModes: []string{"safety", "claim", "unclassified"},
			AllowedRoles:             []string{"owner", "operator"},
			RequiredFacts: []domain.FactRequirement{
				{MaterialField: "price", Freshness: "fresh"},
			},
			Confirmation:     domain.ConfirmationNotRequired,
			RequiredGates:    []string{"OQ-19"},
			TimeoutMS:        5000,
			MaterialityClass: domain.MaterialityMaterial,
			ConsumptionRule:  domain.ConsumptionUnlimited,
			AllowedStages:    []string{domain.StageSynthetic, domain.StageClosedAccountTest},
		},
		DeliveryContract: domain.DeliveryContract{
			CapabilityClass:       domain.CapabilityQueryReconcilable,
			IdempotencyScope:      "ключ запроса статуса",
			TerminalEvidence:      "квитанция результата запроса",
			ReconciliationQuery:   "query.trunk-status",
			CancelCompensationSem: "не применимо (read-only)",
		},
	}

	older := base
	older.Version = "1.1.0"
	older.Name = "Отправка ценового предложения (прошлая версия)"

	other := base
	other.ID = "order.check-order-status"
	other.Version = "1.0.0"
	other.Name = "Проверка статуса заказа"
	other.OwningSystem = "erp"
	other.Conditions.RequiredFacts = []domain.FactRequirement{{MaterialField: "order_status", Freshness: "stale"}}

	return []domain.ActionDefinition{base, older, other}
}

// TestListFromOffersOnlyNewest — список отдаёт ТОЛЬКО новейшую версию
// каждого ID (AC-8: новые привязки — только текущий контракт).
func TestListFromOffersOnlyNewest(t *testing.T) {
	defs := fixtureDefinitions()
	listed := ListFrom(defs)
	if len(listed) != 2 {
		t.Fatalf("ожидаются 2 карточки (новейшая каждого ID), получено %d", len(listed))
	}
	versions := map[string]string{}
	for _, d := range listed {
		versions[d.ID] = d.Version
	}
	if versions["quote.send-price-quote"] != "1.2.0" {
		t.Fatalf("новейшая версия quote = %q, ожидается 1.2.0", versions["quote.send-price-quote"])
	}
	if versions["order.check-order-status"] != "1.0.0" {
		t.Fatalf("новейшая версия order = %q, ожидается 1.0.0", versions["order.check-order-status"])
	}
}

// TestGetFromPinsExactVersion — пин точной версии, старые неизменяемы и
// резолвимы (материал AC-8 для будущих Manifest/Operation/Receipt).
func TestGetFromPinsExactVersion(t *testing.T) {
	defs := fixtureDefinitions()
	pinned, ok := GetFrom(defs, "quote.send-price-quote", "1.1.0")
	if !ok {
		t.Fatal("пин старой версии 1.1.0 обязан резолвиться")
	}
	if pinned.Version != "1.1.0" {
		t.Fatalf("пин вернул %q, ожидается 1.1.0", pinned.Version)
	}
	if _, ok := GetFrom(defs, "quote.send-price-quote", "9.9.9"); ok {
		t.Fatal("несуществующая версия не обязана резолвиться")
	}
	if _, ok := GetFrom(defs, "missing.action", "1.1.0"); ok {
		t.Fatal("несуществующий ID не обязан резолвиться")
	}
}

// TestLatestFromSemantics — Latest отдаёт новейшую и сообщает, является ли
// запрошенная версия текущей (прецедент 1-8).
func TestLatestFromSemantics(t *testing.T) {
	defs := fixtureDefinitions()
	newest, isLatest, exists := LatestFrom(defs, "quote.send-price-quote", "1.2.0")
	if !exists || !isLatest || newest.Version != "1.2.0" {
		t.Fatalf("запрос текущей версии: exists=%v isLatest=%v version=%q", exists, isLatest, newest.Version)
	}
	_, isLatest, exists = LatestFrom(defs, "quote.send-price-quote", "1.1.0")
	if !exists || isLatest {
		t.Fatalf("запрос старой версии: exists=%v isLatest=%v — старый пин не является текущим", exists, isLatest)
	}
	_, _, exists = LatestFrom(defs, "missing.action", "1.0.0")
	if exists {
		t.Fatal("отсутствующий ID не существует")
	}
}

// TestBinaryCatalogIsHonestlyEmpty — бинарный каталог продуктового бинарника
// ПУСТ по AG-4: ни один пак не связан с capability-коннектором. Это честное
// состояние, а не дефект (контрольная ветка).
func TestBinaryCatalogIsHonestlyEmpty(t *testing.T) {
	if got := List(); len(got) != 0 {
		t.Fatalf("бинарный каталог обязан быть честно пуст, получено %d записей", len(got))
	}
	if _, ok := Get("quote.send-price-quote", "1.2.0"); ok {
		t.Fatal("пустой каталог не резолвит записей")
	}
	if _, _, exists := Latest("quote.send-price-quote", "1.2.0"); exists {
		t.Fatal("пустой каталог не отдаёт Latest")
	}
	if err := domain.CatalogIntegrity(nil); err != nil {
		t.Fatalf("пустой каталог целостен vacuously: %v", err)
	}
}

// TestListFromReturnsDefensiveCopies — мутация результата не меняет источник
// (clone-паттерн 1-8).
func TestListFromReturnsDefensiveCopies(t *testing.T) {
	defs := fixtureDefinitions()
	listed := ListFrom(defs)
	if len(listed) == 0 {
		t.Fatal("ожидается непустой список")
	}
	listed[0].TypedInputSchema[0].Name = "испорченное-имя"
	listed[0].Conditions.AllowedConversationModes[0] = "испорченный-режим"
	listed[0].Conditions.RequiredFacts[0].MaterialField = "испорченное-поле"
	fresh := ListFrom(defs)
	if fresh[0].TypedInputSchema[0].Name != "product_ref" {
		t.Fatal("мутация результата List не обязана менять источник — схемы")
	}
	if fresh[0].Conditions.AllowedConversationModes[0] != "safety" {
		t.Fatal("мутация результата List не обязана менять источник — режимы")
	}
	if fresh[0].Conditions.RequiredFacts[0].MaterialField != "price" {
		t.Fatal("мутация результата List не обязана менять источник — факты")
	}
}

// TestGetFromReturnsDefensiveCopies — пин тоже отдаёт копию.
func TestGetFromReturnsDefensiveCopies(t *testing.T) {
	defs := fixtureDefinitions()
	pinned, ok := GetFrom(defs, "quote.send-price-quote", "1.2.0")
	if !ok {
		t.Fatal("пин обязан резолвиться")
	}
	pinned.TypedInputSchema[0].Name = "испорченное-имя"
	again, _ := GetFrom(defs, "quote.send-price-quote", "1.2.0")
	if again.TypedInputSchema[0].Name != "product_ref" {
		t.Fatal("мутация пина не обязана менять источник")
	}
}
