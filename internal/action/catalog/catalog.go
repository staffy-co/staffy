package catalog

import (
	"github.com/staffy-co/staffy/internal/action/domain"
)

// definitions is the immutable binary catalog of Authorized Business Actions.
//
// The FIRST product entry appears with story 4.6, because the 4-1 admission
// registry finally holds a capability-bearing declaration the entry can
// resolve against: abcp-parts@1.1.0 (StatusCandidate, AdapterImplemented
// false) declares command.prepare-client-order with Idempotent=true — the
// auto-parts@6.0.0 pack is published, so the definition's PackRef and
// RequiredCapability resolve against the LIVE employee and connector
// catalogs, and the delivery-word cross-check (provider_idempotent vs the
// declared Idempotent=true) passes. AdapterImplemented=false does NOT
// block the catalog — it blocks the PRODUCTION dispatch (the nil mutation
// adapter, AG-4): the entry is honest about what Staffy may commit to AFTER
// the customer Confirmation, never about a wired provider. The entry carries
// no supplier-send field at all (the OQ-3 fence: supplier_form/send stays
// absent from the Action Catalog). The test circuit keeps registering
// fixture definitions ON TOP of this product list (the ListFrom pattern).
var definitions = []domain.ActionDefinition{
	{
		// Story 4.6 (step-01): the first product entry — one client
		// order in ABCP through the action contour. The typed input is
		// the builder-derived Order Commitment material (the operator
		// never types it); the typed output is the receipt triple plus
		// the derived state word. NO supplier-send field exists in
		// either schema — structurally (the OQ-3 fence).
		ID:      domain.OrderActionID,
		Version: domain.OrderActionVersion,
		Name:    "Создание клиентского заказа в ABCP",
		Purpose: "Создание одного клиентского заказа в ABCP (без отправки поставщику) по точной версии состава после Confirmation клиента; заказ ожидает проверки менеджером.",
		TypedInputSchema: []domain.FieldSchema{
			{Name: "order_digest", Label: "Пин точной версии заказа", Type: domain.FieldText, Required: true, Description: "64-hex SHA-256 канонического состава заказа над пинами всех пяти входов (профиль, запрос, применимость, предложение, выбор) — подтверждается клиентом именно эта версия."},
			{Name: "order_material", Label: "Канонический материал заказа", Type: domain.FieldText, Required: true, Description: "Детерминированный рендер состава: позиции с брендом/артикулом, оригинал/аналог, количеством, клиентской ценой и известными условиями; известная сумма и слово менеджерской проверки."},
			{Name: "known_client_sum", Label: "Известная клиентская сумма", Type: domain.FieldText, Required: true, Description: "Арифметика выбора 4-5 без доставки и сбора — окончательная стоимость не рассчитана."},
			{Name: "currency", Label: "Валюта", Type: domain.FieldText, Required: true, Description: "Слово словаря пака auto-parts@6.0.0 (RUB)."},
		},
		TypedOutputSchema: []domain.FieldSchema{
			{Name: "external_order_id", Label: "Внешний идентификатор заказа", Type: domain.FieldText, Required: true, Description: "Идентификатор подготовленного заказа в ABCP (квитанция)."},
			{Name: "external_order_version", Label: "Версия внешнего объекта", Type: domain.FieldText, Required: true, Description: "Версия внешнего объекта квитанции ABCP."},
			{Name: "external_order_time", Label: "Время квитанции", Type: domain.FieldText, Required: true, Description: "Время подтверждённой квитанции ABCP."},
			{Name: "order_state", Label: "Состояние заказа", Type: domain.FieldText, Required: true, Description: "Производное слово чтения состояния (pending_manager_verification) — заказ создан в магазине и ожидает проверки менеджера."},
		},
		PackRef:            domain.PackRef{PackID: "auto-parts", PackVersion: "6.0.0"},
		RequiredCapability: domain.CapabilityRef{ConnectorID: "abcp-parts", ConnectorVersion: "1.1.0", CapabilityID: "command.prepare-client-order"},
		OwningSystem:       "erp",
		Conditions: domain.Conditions{
			RiskClass:                domain.RiskCritical,
			AutonomyClass:            domain.AutonomyAssisted,
			AllowedConversationModes: []string{"sales"},
			// AllowedRoles carries BOTH the operator word of the spec AND
			// the owner: the pilot's acting operator-manager IS the store
			// owner (the case surfaces — the selection, the order panel —
			// ride the review|configuration scope the identity grant matrix
			// gives to the owner, never to the bare operator role; the
			// spec's «не-оператор 403» row stays honest: a review-scoped
			// expert_qa member hits role_not_allowed).
			AllowedRoles: []string{"operator", "owner"},
			RequiredFacts: []domain.FactRequirement{
				{MaterialField: "price", Freshness: "fresh"},
				{MaterialField: "availability", Freshness: "fresh"},
			},
			Confirmation:     domain.ConfirmationRequired,
			RequiredGates:    []string{"OQ-3", "AG-4"},
			TimeoutMS:        30000,
			MaterialityClass: domain.MaterialityCritical,
			ConsumptionRule:  domain.ConsumptionSingleUse,
			AllowedStages:    []string{domain.StageSynthetic, domain.StageClosedAccountTest},
		},
		DeliveryContract: domain.DeliveryContract{
			CapabilityClass:       domain.CapabilityProviderIdempotent,
			IdempotencyScope:      "идемпотентность провайдера ABCP (декларация 4-1: Idempotent=true) — повторная доставка команды не создаёт второй заказ",
			TerminalEvidence:      "квитанция ABCP с идентификатором подготовленного заказа (accepted → delivered)",
			ReconciliationQuery:   "query_reconcilable по OQ-3: повторное чтение статуса prepared order (декларация 4-1)",
			CancelCompensationSem: "prepared order не отправляется поставщику автоматически; отмена подготовки — отдельная команда до отправки (декларация 4-1)",
		},
	},
}

// List returns a defensive copy of the newest version of every action
// definition. Pinned older versions stay resolvable through Get only, so
// new references always bind the current contract (precedent 1-8).
func List() []domain.ActionDefinition {
	return ListFrom(definitions)
}

// Get pins one definition by id and exact version. Older versions remain
// immutable and resolvable — the material for future Manifest/Operation/
// Receipt references (AC-8); they are never offered as new.
func Get(id, version string) (domain.ActionDefinition, bool) {
	return GetFrom(definitions, id, version)
}

// Latest returns the newest version of a definition and reports whether the
// requested version is that current version (precedent 1-8).
func Latest(id, requestedVersion string) (domain.ActionDefinition, bool, bool) {
	return LatestFrom(definitions, id, requestedVersion)
}

// All returns defensive copies of EVERY entry of the binary dictionary —
// all versions, not just the newest. This is the material of the admission
// pass (ValidateBinaryCatalog walks it through the guard integrity plus the
// reference resolution and the delivery-word cross-check), so a pinned old
// version is held to the same admission truth as the newest one.
func All() []domain.ActionDefinition {
	out := make([]domain.ActionDefinition, 0, len(definitions))
	for _, d := range definitions {
		out = append(out, clone(d))
	}
	return out
}

// ListFrom is the slice-parameterized core of List: the first entry of every
// ID — the catalog is maintained newest-first (the 1-8 convention), so the
// first occurrence IS the newest version. The test composition root uses it
// to serve fixture definitions (the vertical of this slice) without touching
// the product binary.
func ListFrom(defs []domain.ActionDefinition) []domain.ActionDefinition {
	out := make([]domain.ActionDefinition, 0, len(defs))
	seen := map[string]bool{}
	for _, d := range defs {
		if seen[d.ID] {
			continue
		}
		seen[d.ID] = true
		out = append(out, clone(d))
	}
	return out
}

// GetFrom is the slice-parameterized core of Get: the exact pin pair.
func GetFrom(defs []domain.ActionDefinition, id, version string) (domain.ActionDefinition, bool) {
	for _, d := range defs {
		if d.ID == id && d.Version == version {
			return clone(d), true
		}
	}
	return domain.ActionDefinition{}, false
}

// LatestFrom is the slice-parameterized core of Latest.
func LatestFrom(defs []domain.ActionDefinition, id, requestedVersion string) (domain.ActionDefinition, bool, bool) {
	newest, found := newestOf(defs, id)
	if !found {
		return domain.ActionDefinition{}, false, false
	}
	return clone(newest), requestedVersion == newest.Version, true
}

// newestOf finds the entry with the maximum version of one ID.
func newestOf(defs []domain.ActionDefinition, id string) (domain.ActionDefinition, bool) {
	var newest domain.ActionDefinition
	found := false
	for _, d := range defs {
		if d.ID != id {
			continue
		}
		if !found || compareVersions(d.Version, newest.Version) > 0 {
			newest = d
			found = true
		}
	}
	return newest, found
}

// compareVersions orders simple dotted semver-ish strings ("1.10.0" > "1.9.0").
func compareVersions(a, b string) int {
	partsA, partsB := splitVersion(a), splitVersion(b)
	for i := 0; i < 3; i++ {
		if partsA[i] != partsB[i] {
			if partsA[i] > partsB[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func splitVersion(v string) [3]int {
	var out [3]int
	current, index := 0, 0
	for _, r := range v {
		if r == '.' {
			if index < 2 {
				out[index] = current
				index++
				current = 0
			}
			continue
		}
		if r >= '0' && r <= '9' {
			current = current*10 + int(r-'0')
		}
	}
	out[index] = current
	return out
}

func clone(d domain.ActionDefinition) domain.ActionDefinition {
	d.TypedInputSchema = append([]domain.FieldSchema(nil), d.TypedInputSchema...)
	d.TypedOutputSchema = append([]domain.FieldSchema(nil), d.TypedOutputSchema...)
	d.Conditions.AllowedConversationModes = append([]string(nil), d.Conditions.AllowedConversationModes...)
	d.Conditions.AllowedRoles = append([]string(nil), d.Conditions.AllowedRoles...)
	d.Conditions.RequiredFacts = append([]domain.FactRequirement(nil), d.Conditions.RequiredFacts...)
	d.Conditions.RequiredGates = append([]string(nil), d.Conditions.RequiredGates...)
	d.Conditions.AllowedStages = append([]string(nil), d.Conditions.AllowedStages...)
	return d
}
