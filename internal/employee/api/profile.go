package api

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/employee/domain"
	identity "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Profile scope levels: managing the profile surface requires the
// configuration scope; viewing requires configuration, review or audit —
// members without any of them get an honest refusal.
const (
	ProfileScopeManage = "configuration"
	ProfileScopeView   = "review-or-audit"
)

// ProfileDraftCommand creates or CAS-updates the mutable operating profile
// draft. The manifest link is exact (manifest_id, config_version, digest) and
// re-verified against the stored manifest row on every write.
type ProfileDraftCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	Manifest                            domain.ManifestLink
	Content                             domain.ProfileContent
	ExpectedRevision                    int64
}

// ProfileApproveCommand approves the current draft: step-up is required, all
// mandatory sections must be complete and valid. The approval is atomic and
// append-only: exactly one version wins a revision race.
type ProfileApproveCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	Description                         string
	EvidenceRefs                        []string
	ExpectedRevision                    int64
}

// ThresholdCommand records one human threshold decision. Only the participant
// declared as the evidence owner of the gate in the last approved profile
// version may decide; everyone else is honestly refused.
type ThresholdCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	GateID                              string
	Value, Decision                     string
	EvidenceRefs                        []string
}

// ThresholdDecisionView is one recorded decision of the threshold history.
type ThresholdDecisionView struct {
	GateID           string                `json:"gate_id"`
	ThresholdVersion int64                 `json:"threshold_version"`
	Value            string                `json:"value"`
	Decision         string                `json:"decision"`
	ActorID          id.UUID               `json:"actor_id"`
	EvidenceRefs     []string              `json:"evidence_refs"`
	Supersedes       *domain.ThresholdLink `json:"supersedes,omitempty"`
	DecidedAt        time.Time             `json:"decided_at"`
}

// ProfileView is the page/API projection of the operating profile surface:
// the mutable draft with the honest readiness list, the approved version with
// its digest, the derived route states, the derived matrix coverage, the
// derived threshold states and the derived applicability. The profile is
// rules and boundaries of work — never a traffic admission.
type ProfileView struct {
	EmployeeID     id.UUID                      `json:"employee_id"`
	Draft          *domain.ProfileDraft         `json:"draft,omitempty"`
	Readiness      []domain.ProfileRuleError    `json:"readiness"`
	Approved       *domain.ProfileVersion       `json:"approved,omitempty"`
	Applicability  domain.ProfileApplicability  `json:"applicability"`
	RouteStates    []domain.RouteState          `json:"route_states"`
	Matrix         []domain.MatrixCellState     `json:"matrix_coverage"`
	Thresholds     []domain.ThresholdState      `json:"threshold_states"`
	LatestManifest *domain.ManifestLink         `json:"latest_manifest,omitempty"`
	CanManage      bool                         `json:"can_manage"`
	ViewScope      string                       `json:"view_scope"`
}

// ThresholdHistoryView is the append-only history of one gate's decisions.
type ThresholdHistoryView struct {
	EmployeeID  id.UUID                  `json:"employee_id"`
	GateID      string                   `json:"gate_id"`
	Decisions   []ThresholdDecisionView  `json:"decisions"`
}

// ProfileService is the application surface of Story 1-13.
type ProfileService interface {
	ProfileView(context.Context, identity.Principal, id.UUID, id.UUID) (ProfileView, error)
	UpdateDraft(context.Context, ProfileDraftCommand) (domain.ProfileDraft, error)
	ApproveProfile(context.Context, ProfileApproveCommand) (domain.ProfileVersion, error)
	DecideThreshold(context.Context, ThresholdCommand) (domain.GateThresholdVersion, error)
	ThresholdHistory(context.Context, identity.Principal, id.UUID, id.UUID, string) (ThresholdHistoryView, error)
}
