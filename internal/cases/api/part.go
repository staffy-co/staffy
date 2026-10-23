package api

import (
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
)

// ---------------------------------------------------------------------------
// Story 4.3: the part request views (the aliases of the domain forms —
// the zero divergence of the domain↔contract mirror, the
// VehicleProfileView precedent of 4.2)
// ---------------------------------------------------------------------------

// PartRequestView is the read-time part request of the case (the
// derivation over the append-only facts of 2.2 under the pack schema —
// the operator review surface: the mode with the basis word, the
// attributes with the provenance, the completeness with the minimal
// questions, the PartRequestDigest, the confidence word and the three
// honesty boundaries).
type PartRequestView = casesDomain.PartRequestView

// PartAttributeView is one attribute of the request.
type PartAttributeView = casesDomain.PartAttributeView

// PartConfirmationView is the confirmation rendering of one attribute.
type PartConfirmationView = casesDomain.PartConfirmationView

// PartNumberNormalizationView is the flag answer of the part number
// normalization (the oem_number role only).
type PartNumberNormalizationView = casesDomain.PartNumberNormalization

// PartBoundaryView is one honesty boundary of the request (the
// diagnostic / safety / contradiction word — the alert material).
type PartBoundaryView = casesDomain.PartBoundaryView

// PartMaterialKeyView is one key of the schema dictionary hint (the
// honest empty-material answer: the operator records the material by
// the pack dictionary, never a guess).
type PartMaterialKeyView = casesDomain.PartMaterialKeyDomain

// PartModeSpecView is one mode of the pack dictionary.
type PartModeSpecView = casesDomain.PartModeSpecDomain

// PartPanelView is the Story 4.3 section of the case view: the honest
// degradation note when the pinned pack carries no part request schema
// (the panel is absent — NEVER a hardcoded key set), the read-time part
// request otherwise.
type PartPanelView struct {
	SchemaDefined bool            `json:"schema_defined"`
	Note          string          `json:"note,omitempty"`
	Part          PartRequestView `json:"part"`
}

// PartSchemaNoteAbsent is the honest degradation word of the panel when
// the pinned pack carries no part request schema (the 2.2
// unreadable-pin degradation precedent of 4.2).
const PartSchemaNoteAbsent = "Схема запроса детали пака не определена: панель недоступна — материал-ключи и словарь режимов приходят из закреплённого пака, без хардкода."
