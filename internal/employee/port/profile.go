package port

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ProfileDraftParams creates or CAS-updates the mutable operating profile
// draft. The exact manifest link is verified against the stored release
// manifest row before persistence (the app layer performs the verification
// under the caller's authority).
type ProfileDraftParams struct {
	Scope                                       tenancy.Context
	EmployeeID, EventID, AuditID, CorrelationID id.UUID
	Manifest                                    domain.ManifestLink
	Content                                     domain.ProfileContent
	ExpectedRevision                            int64 // 0 = first creation
	Now                                         time.Time
}

// ProfileApproveParams appends the immutable approved profile version. The
// store computes the monotonic version number under the locked draft row and
// enforces the expected draft revision: exactly one version wins a race.
type ProfileApproveParams struct {
	Scope                                       tenancy.Context
	EmployeeID, EventID, AuditID, CorrelationID id.UUID
	Manifest                                    domain.ManifestLink
	Content                                     domain.ProfileContent
	Digest                                      string
	Description                                 string
	EvidenceRefs                                []string
	ApproverRoles                               []string
	ExpectedRevision                            int64
	Now                                         time.Time
}

// ThresholdDecisionParams appends one immutable gate threshold version.
type ThresholdDecisionParams struct {
	Scope                                       tenancy.Context
	EmployeeID, EventID, AuditID, CorrelationID id.UUID
	GateID                                      string
	ThresholdVersion                            int64
	Value, Decision                             string
	EvidenceRefs                                []string
	Supersedes                                  *domain.ThresholdLink
	Now                                         time.Time
}

// ProfileRepository is the persistence port of the operating profile
// surface. operating_profile_drafts is the only mutable table (revision CAS);
// operating_profiles and gate_thresholds are append-only (INSERT/SELECT).
type ProfileRepository interface {
	// ReadProfileDraft returns the current draft; ErrNotFound when none exists.
	ReadProfileDraft(context.Context, tenancy.Context, id.UUID) (domain.ProfileDraft, error)
	// SaveProfileDraft creates the draft (ExpectedRevision 0) or updates it
	// under the expected revision; a lost race returns ProfileConflictError
	// with the current draft state for an honest comparison.
	SaveProfileDraft(context.Context, ProfileDraftParams) (domain.ProfileDraft, error)
	// ApproveProfile appends the next monotonic approved version under the
	// locked draft row and records the approval revision.
	ApproveProfile(context.Context, ProfileApproveParams) (domain.ProfileVersion, error)
	// ReadApprovedProfiles returns every approved version, newest first.
	ReadApprovedProfiles(context.Context, tenancy.Context, id.UUID) ([]domain.ProfileVersion, error)
	// SaveThresholdDecision appends one threshold version for a gate.
	SaveThresholdDecision(context.Context, ThresholdDecisionParams) (domain.GateThresholdVersion, error)
	// ReadThresholdHistory returns every threshold decision of the employee
	// (all gates, any order — the derivation picks the latest per gate).
	ReadThresholdHistory(context.Context, tenancy.Context, id.UUID) ([]domain.GateThresholdVersion, error)
}
