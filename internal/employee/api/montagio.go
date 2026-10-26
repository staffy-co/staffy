package api

import (
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Montagio Admission Registry (spec 5-1): the wire shapes of the tenant-level
// read-only derivation. The domain dictionary is the single source of the
// json tags — the aliases below pin the OpenAPI schemas to the domain
// shapes, so a drift between the dictionary and the contract is impossible
// (the mirror discipline of the compile-time dictionaries).

// MontagioAdmissionView is the machine/page projection of the Montagio
// admission: the read-time report over the live facts (the sealed «Услуги»
// pack pin, the derived capability states, the gate instance history, the
// approved AC-11 wording dictionary) with the reused test & release view
// scope. The view never records anything — a repeated read of an unchanged
// world returns the same answer.
type MontagioAdmissionView struct {
	TenantID  id.UUID                       `json:"tenant_id"`
	Report    domain.MontagioAdmissionReport `json:"montagio_admission"`
	CanManage bool                          `json:"can_manage"`
	ViewScope string                        `json:"view_scope"`
}

// The route/blocker/gate wire views are the domain shapes (aliases: zero
// drift between the domain derivation and the transport contract).
type (
	// MontagioRouteView is one route of the fixed eight-word dictionary with
	// its stage, honest word, Route Matrix column words, requirement views,
	// blockers and fence words.
	MontagioRouteView = domain.MontagioRouteAdmission
	// MontagioBlockerView is one honest blocking: the source, the exact word,
	// the safe state and the next owner of the check.
	MontagioBlockerView = domain.PilotBlocker
)
