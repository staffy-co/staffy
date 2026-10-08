package api

import (
	"context"

	"github.com/artemiosu/staffy/internal/employee/domain"
	identity "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Readiness scope levels: managing the test & release surface (gate
// transitions and the publication) requires the configuration scope plus
// step-up; viewing requires configuration, review or audit — members
// without any of them get an honest refusal.
const (
	ReadinessScopeManage = "configuration"
	ReadinessScopeView   = "review-or-audit"
)

// GateTransitionCommand records one gate lifecycle transition: step-up is
// required for every state change (the single rule of the surface). The
// PASS payload (approver roles, evidence references, hash) travels only
// with the pass target; not_applicable carries its scoped reason.
type GateTransitionCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	DefinitionID                        string
	Route                               *domain.Route
	Modality                            string
	TargetState                         string
	Supersedes                          *id.UUID
	ApprovedBy                          []domain.GateApproval
	EvidenceRefs                        []string
	EvidenceHash                        string
	NotApplicableReason                 string
	ExpiryTrigger                       string
}

// PublishAdmissionCommand publishes the candidate to the requested stage:
// step-up is required, the derivation re-runs atomically and the stage
// ladder is enforced (the exact candidate digests).
type PublishAdmissionCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	Stage                               string
}

// ReadinessView is the page/API projection of the test & release surface:
// the derived report for the requested (or defaulted) stage, the
// applicability of the approved profile, the acting release decision and
// the publication history. The view is a dry-run by construction — it
// never records anything.
type ReadinessView struct {
	EmployeeID    id.UUID                     `json:"employee_id"`
	Stage         string                      `json:"stage"`
	Report        domain.ReadinessReport      `json:"readiness"`
	Applicability domain.ProfileApplicability `json:"applicability"`
	Active        *domain.ReleaseAdmission    `json:"active_release,omitempty"`
	History       []domain.ReleaseAdmission   `json:"admission_history"`
	CanManage     bool                        `json:"can_manage"`
	ViewScope     string                      `json:"view_scope"`
}

// GateInstancesView is the projection of the gate registry surface: the
// fixed platform dictionary and every lifecycle record with the supersedes
// history visible.
type GateInstancesView struct {
	EmployeeID  id.UUID                 `json:"employee_id"`
	Definitions []domain.GateDefinition `json:"definitions"`
	Instances   []domain.GateInstance   `json:"instances"`
	CanManage   bool                    `json:"can_manage"`
	ViewScope   string                  `json:"view_scope"`
}

// AdmissionHistoryView is the append-only publication history, newest
// first, with the acting decision derived.
type AdmissionHistoryView struct {
	EmployeeID id.UUID                   `json:"employee_id"`
	Active     *domain.ReleaseAdmission  `json:"active_release,omitempty"`
	Admissions []domain.ReleaseAdmission `json:"admissions"`
}

// ReadinessService is the application surface of Story 1-14.
type ReadinessService interface {
	ReadinessView(context.Context, identity.Principal, id.UUID, id.UUID, string) (ReadinessView, error)
	GateInstancesView(context.Context, identity.Principal, id.UUID, id.UUID) (GateInstancesView, error)
	RecordGateTransition(context.Context, GateTransitionCommand) (domain.GateInstance, error)
	PublishAdmission(context.Context, PublishAdmissionCommand) (domain.ReleaseAdmission, error)
	AdmissionHistoryView(context.Context, identity.Principal, id.UUID, id.UUID) (AdmissionHistoryView, error)
}

// GateInstanceStateText renders one instance state honestly for displays.
func GateInstanceStateText(state string) string {
	switch state {
	case domain.GateStateOpen:
		return "открыт"
	case domain.GateStateInTest:
		return "в проверке"
	case domain.GateStatePass:
		return "PASS"
	case domain.GateStateFail:
		return "провален"
	case domain.GateStateExpired:
		return "истёк"
	case domain.GateStateNotApplicable:
		return "неприменим"
	}
	return state
}

// ReadinessDefaultStage picks the honest default stage of the panel: the
// next step of the ladder after the acting decision (or synthetic when
// nothing acts yet) — the preview leads to the only available next step.
func ReadinessDefaultStage(active *domain.ReleaseAdmission) string {
	if active == nil {
		return domain.StageSynthetic
	}
	index, known := domain.StageIndex(active.Stage)
	if !known || index+1 >= len(domain.StageOrder) {
		return domain.StageLive
	}
	return domain.StageOrder[index+1]
}
