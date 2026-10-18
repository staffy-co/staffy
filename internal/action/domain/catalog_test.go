package domain

import (
	"errors"
	"strings"
	"testing"
)

// validDefinition is the reference fixture of the guard matrix: a complete,
// fully in-dictionary definition. Every probe below mutates exactly one
// mandatory condition and expects the named refusal — the by-name
// materialization of «отсутствие обязательного условия = запрет».
func validDefinition() ActionDefinition {
	return ActionDefinition{
		ID:      "quote.send-price-quote",
		Version: "1.2.0",
		Name:    "Отправка ценового предложения",
		Purpose: "Ответить клиенту ценой по свежей карточке товара.",
		TypedInputSchema: []FieldSchema{
			{Name: "product_ref", Label: "Товар", Type: FieldText, Required: true, Description: "Ссылка на карточку товара."},
			{Name: "customer_note", Label: "Комментарий", Type: FieldText, Required: false, Description: "Пояснение к предложению."},
		},
		TypedOutputSchema: []FieldSchema{
			{Name: "quote_word", Label: "Слово цены", Type: FieldText, Required: true, Description: "Ценовое слово для ответа клиенту."},
		},
		PackRef:            PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
		RequiredCapability: CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "command.send-sms"},
		OwningSystem:       "catalog",
		Conditions: Conditions{
			RiskClass:                RiskMedium,
			AutonomyClass:            AutonomyAssisted,
			AllowedConversationModes: []string{"safety", "claim", "unclassified"},
			AllowedRoles:             []string{"owner", "operator"},
			RequiredFacts: []FactRequirement{
				{MaterialField: "price", Freshness: "fresh"},
				{MaterialField: "availability", Freshness: "fresh"},
			},
			Confirmation:     ConfirmationRequired,
			RequiredGates:    []string{"OQ-19", "AG-2"},
			TimeoutMS:        15000,
			MaterialityClass: MaterialityMaterial,
			ConsumptionRule:  ConsumptionSingleUse,
			AllowedStages:    []string{StageSynthetic, StageClosedAccountTest},
		},
		DeliveryContract: DeliveryContract{
			CapabilityClass:       CapabilityProviderIdempotent,
			IdempotencyScope:      "ключ отправки предложения",
			TerminalEvidence:      "квитанция доставки провайдера",
			ReconciliationQuery:   "query.trunk-status",
			CancelCompensationSem: "отмена недоступна после accepted",
		},
	}
}

// TestGuardAcceptsCompleteDefinition — the control branch: the reference
// fixture passes. Without this green branch every refusal below would be
// vacuous.
func TestGuardAcceptsCompleteDefinition(t *testing.T) {
	if err := GuardValidateActionDefinition(validDefinition()); err != nil {
		t.Fatalf("полное определение обязано проходить guard, получен отказ: %v", err)
	}
}

// TestGuardMatrixRefusesEveryMissingConditionByName — каждая ветка матрицы
// поимённо: одно мутированное условие → отказ с точным именем правила.
func TestGuardMatrixRefusesEveryMissingConditionByName(t *testing.T) {
	probes := []struct {
		name string
		rule string
		mut  func(*ActionDefinition)
	}{
		{"id-пустой", "id", func(d *ActionDefinition) { d.ID = "" }},
		{"id-пробельный", "id", func(d *ActionDefinition) { d.ID = "   " }},
		{"version-пустая", "version", func(d *ActionDefinition) { d.Version = "" }},
		{"name-пустое", "name", func(d *ActionDefinition) { d.Name = "" }},
		{"purpose-пустое", "purpose", func(d *ActionDefinition) { d.Purpose = " " }},
		{"pack_ref-пустой", "pack_ref", func(d *ActionDefinition) { d.PackRef = PackRef{} }},
		{"pack_ref-без-версии", "pack_ref", func(d *ActionDefinition) { d.PackRef.PackVersion = "" }},
		{"required_capability-пустая", "required_capability", func(d *ActionDefinition) { d.RequiredCapability = CapabilityRef{} }},
		{"required_capability-без-capability_id", "required_capability", func(d *ActionDefinition) { d.RequiredCapability.CapabilityID = "" }},
		{"owning_system-вне-словаря", "owning_system", func(d *ActionDefinition) { d.OwningSystem = "crm" }},
		{"risk_class-отсутствует", "risk_class", func(d *ActionDefinition) { d.Conditions.RiskClass = "" }},
		{"risk_class-вне-словаря", "risk_class", func(d *ActionDefinition) { d.Conditions.RiskClass = "extreme" }},
		{"autonomy_class-отсутствует", "autonomy_class", func(d *ActionDefinition) { d.Conditions.AutonomyClass = "" }},
		{"autonomy_class-вне-словаря", "autonomy_class", func(d *ActionDefinition) { d.Conditions.AutonomyClass = "full_auto" }},
		{"allowed_conversation_modes-пусто", "allowed_conversation_modes", func(d *ActionDefinition) { d.Conditions.AllowedConversationModes = nil }},
		{"allowed_conversation_modes-вне-словаря", "allowed_conversation_modes", func(d *ActionDefinition) { d.Conditions.AllowedConversationModes = []string{"safety", "marketing"} }},
		{"allowed_roles-пусто", "allowed_roles", func(d *ActionDefinition) { d.Conditions.AllowedRoles = nil }},
		{"allowed_roles-вне-словаря", "allowed_roles", func(d *ActionDefinition) { d.Conditions.AllowedRoles = []string{"admin"} }},
		{"required_facts-пусто", "required_facts", func(d *ActionDefinition) { d.Conditions.RequiredFacts = nil }},
		{"required_facts-material_field-вне-словаря", "required_facts[0].material_field", func(d *ActionDefinition) { d.Conditions.RequiredFacts[0].MaterialField = "discount" }},
		{"required_facts-freshness-вне-словаря", "required_facts[0].freshness", func(d *ActionDefinition) { d.Conditions.RequiredFacts[0].Freshness = "recent" }},
		{"confirmation-отсутствует", "confirmation", func(d *ActionDefinition) { d.Conditions.Confirmation = "" }},
		{"confirmation-вне-словаря", "confirmation", func(d *ActionDefinition) { d.Conditions.Confirmation = "maybe" }},
		{"required_gates-пусто", "required_gates", func(d *ActionDefinition) { d.Conditions.RequiredGates = nil }},
		{"required_gates-вне-словаря", "required_gates", func(d *ActionDefinition) { d.Conditions.RequiredGates = []string{"OQ-99"} }},
		{"timeout_ms-ноль", "timeout_ms", func(d *ActionDefinition) { d.Conditions.TimeoutMS = 0 }},
		{"timeout_ms-вне-границ", "timeout_ms", func(d *ActionDefinition) { d.Conditions.TimeoutMS = TimeoutMSMax + 1 }},
		{"materiality_class-отсутствует", "materiality_class", func(d *ActionDefinition) { d.Conditions.MaterialityClass = "" }},
		{"materiality_class-вне-словаря", "materiality_class", func(d *ActionDefinition) { d.Conditions.MaterialityClass = "heavy" }},
		{"consumption_rule-отсутствует", "consumption_rule", func(d *ActionDefinition) { d.Conditions.ConsumptionRule = "" }},
		{"consumption_rule-вне-словаря", "consumption_rule", func(d *ActionDefinition) { d.Conditions.ConsumptionRule = "reusable" }},
		{"allowed_stages-пусто", "allowed_stages", func(d *ActionDefinition) { d.Conditions.AllowedStages = nil }},
		{"allowed_stages-вне-словаря", "allowed_stages", func(d *ActionDefinition) { d.Conditions.AllowedStages = []string{"beta"} }},
		{"capability_class-отсутствует", "capability_class", func(d *ActionDefinition) { d.DeliveryContract.CapabilityClass = "" }},
		{"capability_class-вне-словаря", "capability_class", func(d *ActionDefinition) { d.DeliveryContract.CapabilityClass = "fire_and_forget" }},
		{"idempotency_scope-отсутствует", "delivery_contract.idempotency_scope", func(d *ActionDefinition) { d.DeliveryContract.IdempotencyScope = "" }},
		{"terminal_evidence-отсутствует", "delivery_contract.terminal_evidence", func(d *ActionDefinition) { d.DeliveryContract.TerminalEvidence = " " }},
		{"reconciliation_query-отсутствует", "delivery_contract.reconciliation_query", func(d *ActionDefinition) { d.DeliveryContract.ReconciliationQuery = "" }},
		{"cancel_compensation_semantics-отсутствует", "delivery_contract.cancel_compensation_semantics", func(d *ActionDefinition) { d.DeliveryContract.CancelCompensationSem = "" }},
		{"typed_input_schema-имя-пустое", "typed_input_schema[0].name", func(d *ActionDefinition) { d.TypedInputSchema[0].Name = "" }},
		{"typed_input_schema-имя-повторяется", "typed_input_schema[1].name", func(d *ActionDefinition) { d.TypedInputSchema[1].Name = "product_ref" }},
		{"typed_input_schema-метка-пустая", "typed_input_schema[0].label", func(d *ActionDefinition) { d.TypedInputSchema[0].Label = "" }},
		{"typed_input_schema-тип-вне-словаря", "typed_input_schema[0].type", func(d *ActionDefinition) { d.TypedInputSchema[0].Type = "number" }},
		{"typed_output_schema-имя-пустое", "typed_output_schema[0].name", func(d *ActionDefinition) { d.TypedOutputSchema[0].Name = " " }},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			d := validDefinition()
			probe.mut(&d)
			err := GuardValidateActionDefinition(d)
			if err == nil {
				t.Fatalf("зонд «%s» обязан получить отказ — отсутствие/вне-словарность условия = запрет", probe.name)
			}
			re, ok := err.(RuleError)
			if !ok {
				t.Fatalf("отказ обязан быть RuleError, получен %T: %v", err, err)
			}
			if !strings.HasPrefix(re.Rule, probe.rule) {
				t.Fatalf("зонд «%s»: правило «%s», ожидается префикс «%s»", probe.name, re.Rule, probe.rule)
			}
			if re.Reason == "" {
				t.Fatalf("зонд «%s»: причина отказа пуста — честная причина обязательна", probe.name)
			}
		})
	}
}

// TestGuardUnwrapIsErrInvalid — RuleError распаковывается в ErrInvalid
// (паттерн 1-8: точная причина + зонтик для верхних слоёв).
func TestGuardUnwrapIsErrInvalid(t *testing.T) {
	d := validDefinition()
	d.Conditions.RiskClass = ""
	err := GuardValidateActionDefinition(d)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("RuleError обязан распаковываться в ErrInvalid, получено: %v", err)
	}
}

// TestUnsafeDiscipline — unsafe никогда не покидает человеческий контур:
// подтверждение forced + потребление single_use; otherwise отказ
// с unsafe-причиной. unsafe + человеческая семантика допускается.
func TestUnsafeDiscipline(t *testing.T) {
	t.Run("unsafe-без-подтверждения-отказ", func(t *testing.T) {
		d := validDefinition()
		d.DeliveryContract.CapabilityClass = CapabilityUnsafe
		d.Conditions.Confirmation = ConfirmationNotRequired
		err := GuardValidateActionDefinition(d)
		if err == nil {
			t.Fatal("unsafe с not_required обязан получить отказ")
		}
		re := err.(RuleError)
		if re.Rule != "capability_class" {
			t.Fatalf("правило отказа %q, ожидается capability_class", re.Rule)
		}
	})
	t.Run("unsafe-не-single_use-отказ", func(t *testing.T) {
		d := validDefinition()
		d.DeliveryContract.CapabilityClass = CapabilityUnsafe
		d.Conditions.Confirmation = ConfirmationRequired
		d.Conditions.ConsumptionRule = ConsumptionUnlimited
		err := GuardValidateActionDefinition(d)
		if err == nil {
			t.Fatal("unsafe с unlimited обязан получить отказ")
		}
		re := err.(RuleError)
		if re.Rule != "capability_class" {
			t.Fatalf("правило отказа %q, ожидается capability_class", re.Rule)
		}
	})
	t.Run("unsafe-с-человеческой-семантикой-допускается", func(t *testing.T) {
		d := validDefinition()
		d.DeliveryContract.CapabilityClass = CapabilityUnsafe
		d.Conditions.Confirmation = ConfirmationRequired
		d.Conditions.ConsumptionRule = ConsumptionSingleUse
		if err := GuardValidateActionDefinition(d); err != nil {
			t.Fatalf("unsafe с required + single_use обязан проходить: %v", err)
		}
	})
}

// TestConfirmBeforeMutatePair — словарная пара AD-6: мутирующие классы
// (provider_idempotent, compensatable) с not_required → отказ; чистый
// query-класс (query_reconcilable) с not_required выживает; required
// допустим везде.
func TestConfirmBeforeMutatePair(t *testing.T) {
	for _, class := range []string{CapabilityProviderIdempotent, CapabilityCompensatable} {
		d := validDefinition()
		d.DeliveryContract.CapabilityClass = class
		d.Conditions.Confirmation = ConfirmationNotRequired
		err := GuardValidateActionDefinition(d)
		if err == nil {
			t.Fatalf("класс %q с not_required обязан получить отказ (confirm-before-mutate)", class)
		}
		re := err.(RuleError)
		if re.Rule != "confirmation" {
			t.Fatalf("правило отказа %q, ожидается confirmation", re.Rule)
		}
	}
	t.Run("query-класс-с-не_required-выживает", func(t *testing.T) {
		d := validDefinition()
		d.DeliveryContract.CapabilityClass = CapabilityQueryReconcilable
		d.Conditions.Confirmation = ConfirmationNotRequired
		if err := GuardValidateActionDefinition(d); err != nil {
			t.Fatalf("чистый query-класс с not_required обязан проходить: %v", err)
		}
	})
	t.Run("query-класс-с-required-тоже-допустим", func(t *testing.T) {
		d := validDefinition()
		d.DeliveryContract.CapabilityClass = CapabilityQueryReconcilable
		d.Conditions.Confirmation = ConfirmationRequired
		if err := GuardValidateActionDefinition(d); err != nil {
			t.Fatalf("query-класс с required обязан проходить: %v", err)
		}
	})
}

// TestCatalogIntegrityUniquePinsAndParseableVersions — пин-пары уникальны,
// версии семантически разбираемы (порядок новизны определён), каждая запись
// проходит guard.
func TestCatalogIntegrityUniquePinsAndParseableVersions(t *testing.T) {
	t.Run("пустой-каталог-целостен-vacuously", func(t *testing.T) {
		if err := CatalogIntegrity(nil); err != nil {
			t.Fatalf("пустой каталог целостен vacuously: %v", err)
		}
	})
	t.Run("две-версии-одного-ID-целостны", func(t *testing.T) {
		d1 := validDefinition()
		d2 := validDefinition()
		d2.Version = "1.1.0"
		if err := CatalogIntegrity([]ActionDefinition{d1, d2}); err != nil {
			t.Fatalf("две версии одного ID обязаны быть целостны: %v", err)
		}
	})
	t.Run("дубликат-пин-пары-отказ", func(t *testing.T) {
		d1 := validDefinition()
		d2 := validDefinition()
		err := CatalogIntegrity([]ActionDefinition{d1, d2})
		if err == nil {
			t.Fatal("повтор пин-пары обязан получить отказ")
		}
		if !strings.Contains(err.Error(), "повторяется") {
			t.Fatalf("причина отказа про повтор пин-пары, получено: %v", err)
		}
	})
	t.Run("не-числовая-версия-отказ", func(t *testing.T) {
		d := validDefinition()
		d.Version = "latest"
		err := CatalogIntegrity([]ActionDefinition{d})
		if err == nil {
			t.Fatal("версия вне числовой тройки обязана получить отказ — порядок новизны должен быть определён")
		}
	})
	t.Run("запись-не-проходящая-guard-отказ", func(t *testing.T) {
		d := validDefinition()
		d.Conditions.RiskClass = ""
		err := CatalogIntegrity([]ActionDefinition{d})
		if err == nil {
			t.Fatal("каталог с записью мимо guard обязан получить отказ")
		}
	})
}

// TestParseableVersion — форма X.Y.Z.
func TestParseableVersion(t *testing.T) {
	valid := []string{"0.0.1", "1.2.0", "10.20.30"}
	invalid := []string{"", "1", "1.2", "v1.2.3", "1.2.3.4", "1.x.3", "1..3"}
	for _, v := range valid {
		if !ParseableVersion(v) {
			t.Errorf("«%s» обязан разбираться как версия", v)
		}
	}
	for _, v := range invalid {
		if ParseableVersion(v) {
			t.Errorf("«%s» обязан отвергаться как версия", v)
		}
	}
}
