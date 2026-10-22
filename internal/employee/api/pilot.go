package api

import (
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Pilot Admission Registry (spec 4-1): the wire shapes of the tenant-level
// read-only derivation. The domain dictionary is the single source of the
// json tags — the aliases below pin the OpenAPI schemas to the domain
// shapes, so a drift between the dictionary and the contract is impossible
// (the mirror discipline of the compile-time dictionaries).

// PilotAdmissionView is the machine/page projection of the pilot
// admission: the read-time report over the live facts (the sealed pack
// pin, the derived capability states, the gate instance history) with the
// reused test & release view scope. The view never records anything — a
// repeated read of an unchanged world returns the same answer.
type PilotAdmissionView struct {
	TenantID  id.UUID                     `json:"tenant_id"`
	Report    domain.PilotAdmissionReport `json:"pilot_admission"`
	CanManage bool                        `json:"can_manage"`
	ViewScope string                      `json:"view_scope"`
}

// The route/blocker/gate wire views are the domain shapes (aliases: zero
// drift between the domain derivation and the transport contract).
type (
	// PilotRouteView is one route of the fixed six-word dictionary with its
	// stage, honest word, requirement views, blockers and fence words.
	PilotRouteView = domain.PilotRouteAdmission
	// PilotBlockerView is one honest blocking: the source, the exact word,
	// the safe state and the next owner of the check.
	PilotBlockerView = domain.PilotBlocker
	// GateRequirementView is one required gate with its derived state and
	// the pinned honest words (close phrase, safe state, next owner).
	GateRequirementView = domain.PilotGateView
)
