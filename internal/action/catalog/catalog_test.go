package catalog

import (
	"strings"
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

// TestBinaryCatalogCarriesTheFirstProductEntry — первый продуктовый энтри
// появился (story 4.6): бинарный каталог несёт РОВНО один энтри —
// create_abcp_customer_order@1.0.0, пин резолвится, Latest работает, guard
// проходит. Каталог перестал быть пустым, потому что декларация 4-1
// (abcp-parts@1.1.0/command.prepare-client-order, Idempotent=true)
// резолвится против живого каталога коннекторов.
func TestBinaryCatalogCarriesTheFirstProductEntry(t *testing.T) {
	listed := List()
	if len(listed) != 1 {
		t.Fatalf("бинарный каталог несёт ровно один продуктовый энтри (4-6), получено %d записей", len(listed))
	}
	first := listed[0]
	if first.ID != "create_abcp_customer_order" || first.Version != "1.0.0" {
		t.Fatalf("первый продуктовый энтри — create_abcp_customer_order@1.0.0, получено %s@%s", first.ID, first.Version)
	}
	pinned, ok := Get("create_abcp_customer_order", "1.0.0")
	if !ok {
		t.Fatal("пин первого продуктового энтри обязан резолвиться")
	}
	if pinned.Name == "" || pinned.Purpose == "" {
		t.Fatal("пин обязан нести полное определение")
	}
	newest, isLatest, exists := Latest("create_abcp_customer_order", "1.0.0")
	if !exists || !isLatest || newest.Version != "1.0.0" {
		t.Fatalf("Latest первого энтри: exists=%v isLatest=%v version=%q", exists, isLatest, newest.Version)
	}
	_, wrongIsLatest, wrongExists := Latest("create_abcp_customer_order", "9.9.9")
	if !wrongExists || wrongIsLatest {
		t.Fatalf("запрос чужой версии: exists=%v isLatest=%v — ID существует, версия не является текущей", wrongExists, wrongIsLatest)
	}
	if _, _, missingExists := Latest("missing.action", "1.0.0"); missingExists {
		t.Fatal("несуществующий ID не существует")
	}
	if err := domain.CatalogIntegrity(All()); err != nil {
		t.Fatalf("каталог с первым продуктовым энтри целостен: %v", err)
	}
}

// TestFirstProductEntryCarriesNoSupplierSendFields — зонд fence OQ-3:
// типовые схемы первого продуктового энтри НЕ содержат НИ ОДНОГО поля
// отправки/формирования поставщику (supplier_form/send отсутствует в Action
// Catalog — «passing OQ-3/OQ-5 does not authorize supplier-send»).
func TestFirstProductEntryCarriesNoSupplierSendFields(t *testing.T) {
	def, ok := Get("create_abcp_customer_order", "1.0.0")
	if !ok {
		t.Fatal("первый продуктовый энтри обязан резолвиться")
	}
	forbidden := []string{"supplier_form", "supplier_send", "send_to_supplier", "supplier_send_form", "delivery_to_supplier", "ship_to_supplier"}
	for _, schema := range []struct {
		label  string
		fields []domain.FieldSchema
	}{
		{"typed_input_schema", def.TypedInputSchema},
		{"typed_output_schema", def.TypedOutputSchema},
	} {
		for _, f := range schema.fields {
			joined := strings.ToLower(f.Name + " " + f.Label + " " + f.Description)
			for _, word := range forbidden {
				if strings.Contains(joined, word) {
					t.Fatalf("схема %s несёт поле отправки поставщику «%s» — fence OQ-3 нарушен", schema.label, f.Name)
				}
			}
		}
	}
	// Пин контура действия: пак/capability/erp/provider_idempotent/required/single_use/critical.
	if def.PackRef.PackID != "auto-parts" || def.PackRef.PackVersion != "6.0.0" {
		t.Fatalf("пин пака обязан быть auto-parts@6.0.0, получено %s@%s", def.PackRef.PackID, def.PackRef.PackVersion)
	}
	if def.RequiredCapability.ConnectorID != "abcp-parts" || def.RequiredCapability.ConnectorVersion != "1.1.0" ||
		def.RequiredCapability.CapabilityID != "command.prepare-client-order" {
		t.Fatal("пин capability обязан быть abcp-parts@1.1.0/command.prepare-client-order")
	}
	if def.OwningSystem != "erp" {
		t.Fatalf("владеющая система заказа — erp, получено %q", def.OwningSystem)
	}
	if def.DeliveryContract.CapabilityClass != domain.CapabilityProviderIdempotent {
		t.Fatalf("класс доставки — provider_idempotent (декларация 4-1: Idempotent=true), получено %q", def.DeliveryContract.CapabilityClass)
	}
	if def.Conditions.Confirmation != domain.ConfirmationRequired {
		t.Fatal("подтверждение клиента обязательно (confirm-before-mutate, AD-6)")
	}
	if def.Conditions.ConsumptionRule != domain.ConsumptionSingleUse {
		t.Fatal("потребление single_use")
	}
	if def.Conditions.RiskClass != domain.RiskCritical {
		t.Fatal("класс риска critical")
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

// TestAllExposesEveryVersionOfTheBinaryDictionary — All() отдаёт ВСЕ записи
// словаря (не только новейшие) — материал прохода допуска; бинарник несёт
// первый продуктовый энтри (4-6) = НЕ-nil срез ровно с одной записью.
func TestAllExposesEveryVersionOfTheBinaryDictionary(t *testing.T) {
	all := All()
	if all == nil {
		t.Fatal("бинарный каталог обязан отдавать НЕ-nil срез (пустота была бы нулевой веткой)")
	}
	if len(all) != 1 {
		t.Fatalf("бинарный каталог несёт первый продуктовый энтри (4-6) — All обязан отдать ровно 1 запись, получено: %d", len(all))
	}
	if all[0].ID != "create_abcp_customer_order" {
		t.Fatalf("первая запись All — первый продуктовый энтри, получено %q", all[0].ID)
	}
	// The From-variant semantics hold for every version: the fixture slice
	// keeps ALL pinned versions visible to the admission pass.
	defs := fixtureDefinitions()
	listed, pinned := ListFrom(defs), defs
	if len(listed) != 2 || len(pinned) != 3 {
		t.Fatalf("List отдаёт новейшие (%d), All-семантика держит все записи (%d)", len(listed), len(pinned))
	}
}
