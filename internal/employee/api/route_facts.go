package api

import (
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// RouteFacts is the safe cross-module projection of the acting release
// decisions that seal one connection (the ingress ladder of Story 2-1
// consumes it): per candidate the stage, the decision number, the
// per-route results and the human coverage of the exact pinned profile
// version, plus the relayed entitlement admission word. Never secrets,
// never raw payloads, never unsealed configuration content.
type RouteFacts struct {
	ConnectionID id.UUID              `json:"connection_id"`
	Candidates   []RouteCandidate     `json:"candidates"`
	Entitlement  RouteEntitlementWord `json:"entitlement"`
}

// RouteCandidate is one employee whose acting decision's sealed manifest
// contains the connection. PackID/PackVersion carry the Domain Pack of the
// sealed configuration version — the taxonomy source the case contour pins
// onto the case row at ingest (Story 2.2).
type RouteCandidate struct {
	EmployeeID     id.UUID                 `json:"employee_id"`
	DecisionNo     int64                   `json:"decision_no"`
	Stage          string                  `json:"stage"`
	ProfileVersion int64                   `json:"profile_version"`
	PackID         string                  `json:"pack_id,omitempty"`
	PackVersion    string                  `json:"pack_version,omitempty"`
	Routes         []domain.RouteAdmission `json:"routes"`
	Coverage       []domain.HumanCoverage  `json:"human_coverage"`
}

// RouteEntitlementWord is the relayed entitlement admission state (the
// hard limit word of the ingress ladder; existing commitments always
// continue).
type RouteEntitlementWord struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}
