package domain

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// The declarative schema (FieldSchema pattern of 1-8, mirrored — the action
// domain never imports the connector domain)
// ---------------------------------------------------------------------------

// FieldType enumerates declarative input/output field kinds (mirror 1-8).
type FieldType string

const (
	FieldText FieldType = "text"
	FieldURL  FieldType = "url"
)

// FieldTypes is the full dictionary.
var FieldTypes = []FieldType{FieldText, FieldURL}

// FieldSchema is one declared input or output field of an action definition.
// It is a declaration only: the catalog never carries field VALUES, and a
// Secret field is only ever marked — secret values never live here.
type FieldSchema struct {
	Name        string    `json:"name"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required"`
	Secret      bool      `json:"secret"`
	Description string    `json:"description"`
}

// ---------------------------------------------------------------------------
// The mandatory references (structure here; resolution in the app layer)
// ---------------------------------------------------------------------------

// PackRef references one published Domain Pack version. The domain gates the
// structure (non-empty pair); the app layer resolves it against the employee
// pack catalog through a port — never a direct import.
type PackRef struct {
	PackID      string `json:"pack_id"`
	PackVersion string `json:"pack_version"`
}

// CapabilityRef references the exact connector capability an action needs.
// Resolved against the connector catalog (CapabilityDeclaration 1-8) by the
// app layer, including the delivery-word cross-check of AC-3.
type CapabilityRef struct {
	ConnectorID      string `json:"connector_id"`
	ConnectorVersion string `json:"connector_version"`
	CapabilityID     string `json:"capability_id"`
}

// FactRequirement is one material dependency of the action: a material field
// of 2-7 plus the freshness word its answer requires.
type FactRequirement struct {
	MaterialField string `json:"material_field"`
	Freshness     string `json:"freshness"`
}

// ---------------------------------------------------------------------------
// The typed condition system (fail-closed: every field is a dictionary word,
// every list is non-empty and in-dictionary — absence is a prohibition)
// ---------------------------------------------------------------------------

// Conditions is the complete typed condition system of one action.
// Every field is mandatory; GuardValidateActionDefinition refuses a
// definition with any missing or out-of-dictionary word.
type Conditions struct {
	RiskClass                string            `json:"risk_class"`
	AutonomyClass            string            `json:"autonomy_class"`
	AllowedConversationModes []string          `json:"allowed_conversation_modes"`
	AllowedRoles             []string          `json:"allowed_roles"`
	RequiredFacts            []FactRequirement `json:"required_facts"`
	Confirmation             string            `json:"confirmation"`
	RequiredGates            []string          `json:"required_gates"`
	TimeoutMS                int               `json:"timeout_ms"`
	MaterialityClass         string            `json:"materiality_class"`
	ConsumptionRule          string            `json:"consumption_rule"`
	AllowedStages            []string          `json:"allowed_stages"`
}

// DeliveryContract is the delivery classification of the action (AC-3).
// CapabilityClass is cross-checked against the provider CapabilityDeclaration
// by the app layer; the four declared semantics words are mandatory here.
type DeliveryContract struct {
	CapabilityClass       string `json:"capability_class"`
	IdempotencyScope      string `json:"idempotency_scope"`
	TerminalEvidence      string `json:"terminal_evidence"`
	ReconciliationQuery   string `json:"reconciliation_query"`
	CancelCompensationSem string `json:"cancel_compensation_semantics"`
}

// ActionDefinition is an immutable, versioned entry of the authorized action
// dictionary. (ID, Version) is a pin pair: older versions stay resolvable and
// are never rewritten — the material for future Manifest/Operation/Receipt
// references (AC-8). A definition declares intent, schema and conditions
// only: no executable policy, no provider DTOs, no secrets.
type ActionDefinition struct {
	ID                 string           `json:"id"`
	Version            string           `json:"version"`
	Name               string           `json:"name"`
	Purpose            string           `json:"purpose"`
	TypedInputSchema   []FieldSchema    `json:"typed_input_schema"`
	TypedOutputSchema  []FieldSchema    `json:"typed_output_schema"`
	PackRef            PackRef          `json:"pack_ref"`
	RequiredCapability CapabilityRef    `json:"required_capability"`
	OwningSystem       string           `json:"owning_system"`
	Conditions         Conditions       `json:"conditions"`
	DeliveryContract   DeliveryContract `json:"delivery_contract"`
}

// ---------------------------------------------------------------------------
// GuardValidateActionDefinition — the fail-closed integrity gate
// ---------------------------------------------------------------------------

// GuardValidateActionDefinition refuses a definition that is not complete
// and fully in-dictionary. Fail-closed by construction: every mandatory
// condition must be declared with a valid word — «отсутствие обязательного
// условия = запрет» is material, not a convention. Structural reference
// fields are gated here; their RESOLUTION against the live pack and
// connector catalogs happens in the app layer through ports.
func GuardValidateActionDefinition(d ActionDefinition) error {
	if strings.TrimSpace(d.ID) == "" {
		return RuleError{"id", "Идентификатор действия обязателен."}
	}
	if strings.TrimSpace(d.Version) == "" {
		return RuleError{"version", "Версия определения обязательна (пин-пара ID+Version)."}
	}
	if strings.TrimSpace(d.Name) == "" {
		return RuleError{"name", "Понятное имя действия обязательно."}
	}
	if strings.TrimSpace(d.Purpose) == "" {
		return RuleError{"purpose", "Назначение действия обязательно."}
	}

	if err := validateFieldSchemas("typed_input_schema", d.TypedInputSchema); err != nil {
		return err
	}
	if err := validateFieldSchemas("typed_output_schema", d.TypedOutputSchema); err != nil {
		return err
	}

	if strings.TrimSpace(d.PackRef.PackID) == "" || strings.TrimSpace(d.PackRef.PackVersion) == "" {
		return RuleError{"pack_ref", "Ссылка на Domain Pack обязательна (pack_id + pack_version)."}
	}
	if strings.TrimSpace(d.RequiredCapability.ConnectorID) == "" ||
		strings.TrimSpace(d.RequiredCapability.ConnectorVersion) == "" ||
		strings.TrimSpace(d.RequiredCapability.CapabilityID) == "" {
		return RuleError{"required_capability", "Ссылка на capability коннектора обязательна (connector_id + connector_version + capability_id)."}
	}
	if !WordIn(OwningSystemWords, d.OwningSystem) {
		return RuleError{"owning_system", "Владеющая система должна быть словом словаря {catalog, erp}."}
	}

	c := d.Conditions
	if !WordIn(RiskClasses, c.RiskClass) {
		return RuleError{"risk_class", "Класс риска обязателен и должен быть словом словаря {low, medium, high, critical}."}
	}
	if !WordIn(AutonomyClasses, c.AutonomyClass) {
		return RuleError{"autonomy_class", "Класс автономности обязателен и должен быть словом словаря FR-42 {shadow, copilot, assisted, auto, exception}."}
	}
	if err := validateWordList("allowed_conversation_modes", c.AllowedConversationModes, ConversationModeWords, "режимов разговора 2-5"); err != nil {
		return err
	}
	if err := validateWordList("allowed_roles", c.AllowedRoles, RoleWords, "ролей identity"); err != nil {
		return err
	}
	if len(c.RequiredFacts) == 0 {
		return RuleError{"required_facts", "Действие обязано объявить материальные зависимости: минимум одна пара material_field + freshness."}
	}
	for i, fact := range c.RequiredFacts {
		if !WordIn(MaterialFieldWords, fact.MaterialField) {
			return RuleError{fmt.Sprintf("required_facts[%d].material_field", i), "Материальное поле должно быть словом словаря 2-7 {price, availability, order_status}."}
		}
		if !WordIn(FreshnessWords, fact.Freshness) {
			return RuleError{fmt.Sprintf("required_facts[%d].freshness", i), "Слово свежести обязано быть словом словаря 2-7 {fresh, stale, failed, unknown}."}
		}
	}
	if !WordIn(ConfirmationRequirements, c.Confirmation) {
		return RuleError{"confirmation", "Требование подтверждения обязательно и должно быть словом {required, not_required}."}
	}
	if err := validateWordList("required_gates", c.RequiredGates, GateIDWords, "гейтов GATE-REGISTRY"); err != nil {
		return err
	}
	if c.TimeoutMS < TimeoutMSMin || c.TimeoutMS > TimeoutMSMax {
		return RuleError{"timeout_ms", "Таймаут исполнения обязателен и ограничен 1..600000 мс."}
	}
	if !WordIn(MaterialityClasses, c.MaterialityClass) {
		return RuleError{"materiality_class", "Класс материальности обязателен и должен быть словом словаря {immaterial, material, critical}."}
	}
	if !WordIn(ConsumptionRules, c.ConsumptionRule) {
		return RuleError{"consumption_rule", "Правило потребления обязательно и должно быть словом словаря {single_use, bounded, unlimited}."}
	}
	if err := validateWordList("allowed_stages", c.AllowedStages, AllowedStages, "стадий GATE-REGISTRY"); err != nil {
		return err
	}

	dc := d.DeliveryContract
	if !WordIn(CapabilityClasses, dc.CapabilityClass) {
		return RuleError{"capability_class", "Класс доставки обязателен и должен быть словом словаря {provider_idempotent, query_reconcilable, compensatable, unsafe}."}
	}
	for _, word := range []struct{ field, label string }{
		{dc.IdempotencyScope, "idempotency_scope"},
		{dc.TerminalEvidence, "terminal_evidence"},
		{dc.ReconciliationQuery, "reconciliation_query"},
		{dc.CancelCompensationSem, "cancel_compensation_semantics"},
	} {
		if strings.TrimSpace(word.field) == "" {
			return RuleError{"delivery_contract." + word.label, "Delivery-контракт обязан объявить слово «" + word.label + "» — отсутствие обязательного слова = запрет."}
		}
	}

	// Unsafe discipline (AC-3) goes FIRST: unsafe never leaves the human loop —
	// confirmation forced AND single use only, with the unsafe-specific reason
	// (the by-name probe of the guard matrix).
	if dc.CapabilityClass == CapabilityUnsafe {
		if c.Confirmation != ConfirmationRequired {
			return RuleError{"capability_class", "unsafe-действия остаются только в человеческом контуре: подтверждение обязательно."}
		}
		if c.ConsumptionRule != ConsumptionSingleUse {
			return RuleError{"capability_class", "unsafe-действия остаются только в человеческом контуре: потребление только single_use."}
		}
	}

	// Confirm-before-mutate (AD-6): the mutating capability classes are
	// forced to ConfirmationRequired. not_required survives only on the pure
	// query class (query_reconcilable).
	if dc.CapabilityClass != CapabilityQueryReconcilable && c.Confirmation != ConfirmationRequired {
		return RuleError{"confirmation", "Мутирующий класс доставки требует подтверждения перед изменением (confirm-before-mutate, AD-6): not_required выживает только у чистого query-класса."}
	}

	return nil
}

// validateWordList refuses empty or out-of-dictionary lists (fail-closed:
// a missing list is a missing condition).
func validateWordList(rule string, words []string, dictionary []string, label string) error {
	if len(words) == 0 {
		return RuleError{rule, "Список «" + rule + "» обязателен: минимум одно слово словаря " + label + "."}
	}
	for _, w := range words {
		if !WordIn(dictionary, w) {
			return RuleError{rule, "Слово «" + w + "» вне словаря " + label + "."}
		}
	}
	return nil
}

// validateFieldSchemas checks declared fields: valid type, unique non-empty
// names. Empty schemas are allowed (an action may have no inputs), but a
// present field must be a well-formed declaration.
func validateFieldSchemas(rule string, fields []FieldSchema) error {
	seen := map[string]bool{}
	for i, f := range fields {
		if strings.TrimSpace(f.Name) == "" {
			return RuleError{fmt.Sprintf("%s[%d].name", rule, i), "Имя поля схемы обязательно."}
		}
		if seen[f.Name] {
			return RuleError{fmt.Sprintf("%s[%d].name", rule, i), "Имя поля «" + f.Name + "» повторяется в схеме."}
		}
		seen[f.Name] = true
		if strings.TrimSpace(f.Label) == "" {
			return RuleError{fmt.Sprintf("%s[%d].label", rule, i), "Понятная метка поля «" + f.Name + "» обязательна."}
		}
		if !fieldTypeIn(f.Type) {
			return RuleError{fmt.Sprintf("%s[%d].type", rule, i), "Тип поля «" + f.Name + "» должен быть словом словаря {text, url}."}
		}
	}
	return nil
}

// fieldTypeIn is the typed dictionary membership check for field types.
func fieldTypeIn(t FieldType) bool {
	for _, word := range FieldTypes {
		if t == word {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// CatalogIntegrity — the binary dictionary invariants
// ---------------------------------------------------------------------------

// CatalogIntegrity refuses a dictionary whose pin pairs are not unique or
// whose versions are not semantically parseable (the monotonic order of
// versions per ID must be well-defined, so List/Latest can pick the newest
// honestly). Every entry must individually pass the guard.
func CatalogIntegrity(defs []ActionDefinition) error {
	type key struct{ id, version string }
	seen := map[key]bool{}
	for i, d := range defs {
		if err := GuardValidateActionDefinition(d); err != nil {
			return RuleError{fmt.Sprintf("definitions[%d]", i), "Запись каталога не проходит guard: " + err.Error()}
		}
		k := key{d.ID, d.Version}
		if seen[k] {
			return RuleError{"definitions", "Пин-пара (ID, Version) («" + d.ID + "», «" + d.Version + "») повторяется — версии неизменяемы и уникальны."}
		}
		seen[k] = true
		if !ParseableVersion(d.Version) {
			return RuleError{"definitions", "Версия «" + d.Version + "» должна быть числовой тройкой вида X.Y.Z — порядок версий обязан быть определён."}
		}
	}
	return nil
}

// ParseableVersion reports whether v is a dotted numeric triple ("1.2.3"),
// so version ordering is total and monotonic comparisons are honest.
func ParseableVersion(v string) bool {
	if v == "" {
		return false
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Reference resolution — the resolver interfaces of the domain (consumer
// owned) and the delivery-word cross-check of AC-3
// ---------------------------------------------------------------------------

// PackRefResolver is the consumer-owned port the app layer uses to resolve
// a definition's PackRef against the published Domain Pack catalog. The
// action domain never imports the employee module: the composition root
// adapts the live catalog to this contract (structural typing — any
// adapter with this exact method set satisfies it).
type PackRefResolver interface {
	// Get reports whether the exact pack version is published (the pin
	// pair, never "the newest").
	Get(packID, packVersion string) bool
}

// CapabilityFacts carries the provider contract facts of one resolved
// capability — exactly the words the delivery-word cross-check needs. The
// facts are the CapabilityDeclaration (1-8) truth of the provider, relayed
// by the adapter without importing the connector domain.
type CapabilityFacts struct {
	// Idempotent mirrors the provider declaration word: whether the
	// provider itself declares idempotent execution.
	Idempotent bool
	// Reconciliation is the declared reconciliation semantics ("" —
	// nothing declared).
	Reconciliation string
}

// CapabilityRefResolver is the consumer-owned port the app layer uses to
// resolve a definition's RequiredCapability against the published connector
// catalog. A capability resolves only when its CapabilityDeclaration is
// Complete (1-8 renders incomplete semantics as unsupported — an action
// definition may never lean on an unsupported capability).
type CapabilityRefResolver interface {
	// Capability returns the provider contract facts of the exact
	// (connector, version, capability) triple and reports whether it
	// resolves to a complete declaration.
	Capability(connectorID, connectorVersion, capabilityID string) (CapabilityFacts, bool)
}

// ValidateDefinitionRefs resolves the mandatory references of one
// definition against the live catalogs and cross-checks the delivery words
// of the action against the CapabilityDeclaration of the provider (AC-3 —
// the classification is not self-consistent by declaration):
//
//   - provider_idempotent requires the provider declaration Idempotent=true;
//   - query_reconcilable requires a declared reconciliation semantics.
//
// A mismatch refuses the definition with the honest reason — the
// definition may not enter the catalog. The guard (structure) runs before
// this in the admission pass; here the RESOLUTION is the truth. Both
// resolvers are mandatory: a definition may not be admitted without the
// live catalogs (fail-closed).
func ValidateDefinitionRefs(d ActionDefinition, packs PackRefResolver, caps CapabilityRefResolver) error {
	if packs == nil || caps == nil {
		return RuleError{"refs", "Резолверы ссылок обязательны: определение не может быть допущено без живых каталогов паков и коннекторов."}
	}
	if !packs.Get(d.PackRef.PackID, d.PackRef.PackVersion) {
		return RuleError{"pack_ref", "Domain Pack «" + d.PackRef.PackID + "@" + d.PackRef.PackVersion + "» не опубликован — ссылка определения не резолвится."}
	}
	facts, ok := caps.Capability(d.RequiredCapability.ConnectorID, d.RequiredCapability.ConnectorVersion, d.RequiredCapability.CapabilityID)
	if !ok {
		return RuleError{"required_capability", "Capability «" + d.RequiredCapability.ConnectorID + "@" + d.RequiredCapability.ConnectorVersion + "/" + d.RequiredCapability.CapabilityID + "» не резолвится против опубликованного каталога коннекторов."}
	}
	if d.DeliveryContract.CapabilityClass == CapabilityProviderIdempotent && !facts.Idempotent {
		return RuleError{"capability_class", "Коннектор не объявляет идемпотентность для этой capability — класс provider_idempotent не допускается (сверка delivery-слов против CapabilityDeclaration)."}
	}
	if d.DeliveryContract.CapabilityClass == CapabilityQueryReconcilable && strings.TrimSpace(facts.Reconciliation) == "" {
		return RuleError{"capability_class", "Коннектор не объявляет reconciliation для этой capability — класс query_reconcilable не допускается (сверка delivery-слов против CapabilityDeclaration)."}
	}
	return nil
}
