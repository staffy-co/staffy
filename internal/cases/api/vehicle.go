package api

import (
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
)

// ---------------------------------------------------------------------------
// Story 4.2: the vehicle identification views (the aliases of the domain
// forms — the zero divergence of the domain↔contract mirror, the
// PilotAdmissionView precedent of 4-1)
// ---------------------------------------------------------------------------

// VehicleProfileView is the read-time vehicle profile of the case (the
// derivation over the append-only facts of 2.2 under the pack schema —
// the operator review surface and the public confirmation form material).
type VehicleProfileView = casesDomain.VehicleProfileView

// VehicleAttributeView is one attribute of the profile.
type VehicleAttributeView = casesDomain.VehicleAttributeView

// VehicleConfirmationView is the confirmation rendering of one attribute.
type VehicleConfirmationView = casesDomain.VehicleConfirmationView

// VINNormalizationView is the flag answer of the VIN normalization.
type VINNormalizationView = casesDomain.VINNormalization

// VehiclePanelView is the Story 4.2 section of the case view: the honest
// degradation note when the pinned pack carries no identification schema
// (the panel is absent — NEVER a hardcoded attribute set), the profile
// otherwise.
type VehiclePanelView struct {
	SchemaDefined bool               `json:"schema_defined"`
	Note          string             `json:"note,omitempty"`
	Profile       VehicleProfileView `json:"profile"`
}

// VehicleSchemaNoteAbsent is the honest degradation word of the panel
// when the pinned pack carries no identification schema (the 2-2
// unreadable-pin degradation precedent).
const VehicleSchemaNoteAbsent = "Схема идентификации пака не определена: панель недоступна — набор признаков приходит из закреплённого пака, без хардкода."

// ContinuationVehicleView is the vehicle identification block of the
// public continuation form (the protected client surface of FR-55): the
// profile of the confirmable attributes (the facts of the work item of
// the link — the one-link-one-work-item discipline of 2.10), the block
// readback of the VIN and the focus re-ask words of the disputed
// symbols. The full VIN renders HERE and only here (and on the operator
// work card) — every other path carries the mask.
type ContinuationVehicleView struct {
	Profile    VehicleProfileView             `json:"profile"`
	Readback   []casesDomain.VINReadbackBlock `json:"readback"`
	FocusWords []string                       `json:"focus_words"`
}
