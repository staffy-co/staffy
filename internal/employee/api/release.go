package api

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/employee/domain"
	identity "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Release scope levels: managing the release surface requires the
// configuration scope; viewing requires configuration, review or audit —
// members without any of them get an honest refusal.
const (
	ReleaseScopeManage = "configuration"
	ReleaseScopeView   = "review-or-audit"
)

// AssembleCommand creates or refreshes the mutable release draft from the
// current component facts.
type AssembleCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	ExpectedRevision                    int64 // 0 = first creation
}

// VersionCommand appends one immutable configuration version.
type VersionCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	Description                         string
	ExpectedRevision                    int64
}

// SealCommand seals the manifest of one configuration version under the
// expected draft revision (optimistic concurrency).
type SealCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	ConfigVersion                       int64
	ExpectedRevision                    int64
}

// RestoreCommand starts a new draft from a selected configuration version.
type RestoreCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	ConfigVersion                       int64
	ExpectedRevision                    int64
}

// ManifestSummary is the per-version sealed state in the release view.
type ManifestSummary struct {
	ConfigVersion int64     `json:"config_version"`
	ManifestID    id.UUID   `json:"manifest_id"`
	Status        string    `json:"status"`
	Digest        string    `json:"digest"`
	SchemaVersion int       `json:"schema_version"`
	SealedAt      time.Time `json:"sealed_at"`
}

// ManifestView is the full read of one manifest. When Quarantine is set the
// content stays empty and the raw payload is kept — nothing is silently
// discarded.
type ManifestView struct {
	ConfigVersion    int64                   `json:"config_version"`
	ManifestID       id.UUID                 `json:"manifest_id"`
	Status           string                  `json:"status"`
	Digest           string                  `json:"digest"`
	SchemaVersion    int                     `json:"schema_version"`
	SealedAt         time.Time               `json:"sealed_at"`
	ActorID          id.UUID                 `json:"actor_id"`
	Content          *domain.ManifestContent `json:"content,omitempty"`
	Quarantine       string                  `json:"quarantine"`
	QuarantineReason string                  `json:"quarantine_reason,omitempty"`
	Raw              string                  `json:"raw,omitempty"`
}

// ReleaseView is the page/API projection of the release surface.
type ReleaseView struct {
	EmployeeID id.UUID                `json:"employee_id"`
	Draft      *domain.ReleaseDraft   `json:"draft,omitempty"`
	Versions   []domain.ConfigVersion `json:"versions"`
	Manifests  []ManifestSummary      `json:"manifests"`
	Latest     *ManifestView          `json:"latest,omitempty"`
	CanManage  bool                   `json:"can_manage"`
	ViewScope  string                 `json:"view_scope"`
}

// RestoreResult is the new draft plus the honest compatibility report.
type RestoreResult struct {
	Draft  domain.ReleaseDraft         `json:"draft"`
	Issues []domain.CompatibilityIssue `json:"issues"`
}

// ReleaseService is the application surface of Story 1-12.
type ReleaseService interface {
	ReleaseView(context.Context, identity.Principal, id.UUID, id.UUID) (ReleaseView, error)
	AssembleDraft(context.Context, AssembleCommand) (domain.ReleaseDraft, error)
	CreateConfigVersion(context.Context, VersionCommand) (domain.ConfigVersion, error)
	SealManifest(context.Context, SealCommand) (domain.ReleaseManifest, error)
	ReadReleaseManifest(context.Context, identity.Principal, id.UUID, id.UUID, int64) (ManifestView, error)
	Restore(context.Context, RestoreCommand) (RestoreResult, error)
}
