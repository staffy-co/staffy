package port

import (
	"context"
	"time"

	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/employee/domain"
	identity "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ConnectorFacts is the narrow read port to the connector module: the
// release surface consumes connections and their DERIVED capability states
// (one truth: connector/domain.DeriveCapabilityStates). The employee
// module never re-derives states itself. The method names match
// connector/api.Service so the real service satisfies the port
// structurally, without an adapter.
type ConnectorFacts interface {
	List(context.Context, identity.Principal, id.UUID) ([]connectorDomain.Connection, error)
	CapabilityState(context.Context, identity.Principal, id.UUID, id.UUID) (connectorAPI.CapabilityStateResult, error)
}

// DraftParams creates or CAS-updates the mutable release draft.
type DraftParams struct {
	Scope                                       tenancy.Context
	EmployeeID, EventID, AuditID, CorrelationID id.UUID
	Refs                                        domain.ConfigRefs
	ExpectedRevision                            int64 // 0 = first creation
	Now                                         time.Time
}

// RestoreParams CAS-updates the mutable draft into a restored draft that
// references the selected configuration version as its source.
type RestoreParams struct {
	Scope                                       tenancy.Context
	EmployeeID, EventID, AuditID, CorrelationID id.UUID
	Refs                                        domain.ConfigRefs
	RestoredFrom                                *domain.ConfigVersionLink
	ExpectedRevision                            int64
	Now                                         time.Time
}

// VersionParams appends one immutable configuration version. The store
// computes the monotonic version number under the locked draft row.
type VersionParams struct {
	Scope                                                  tenancy.Context
	EmployeeID, VersionID, EventID, AuditID, CorrelationID id.UUID
	Description                                            string
	ExpectedRevision                                       int64
	Now                                                    time.Time
}

// SealParams appends the immutable manifest. The store enforces the draft
// revision (optimistic concurrency: exactly one manifest per race) and the
// UNIQUE config version (no second manifest under one version).
type SealParams struct {
	Scope                                                   tenancy.Context
	EmployeeID, ManifestID, EventID, AuditID, CorrelationID id.UUID
	ConfigVersion                                           int64
	ExpectedRevision                                        int64
	Status                                                  string
	Digest                                                  string
	Content                                                 domain.ManifestContent
	Now                                                     time.Time
}

// ManifestRecord pairs the manifest metadata with the exact stored payload
// bytes: the honest read path (schema-version check, strict decode, digest
// re-computation) always runs against the stored bytes, never against
// re-serialized ones.
type ManifestRecord struct {
	Manifest domain.ReleaseManifest
	Payload  []byte
}

// ReleaseRepository is the persistence port of the release surface. Only
// release_drafts is mutable (revision CAS); versions and manifests are
// append-only.
type ReleaseRepository interface {
	// ReadReleaseDraft returns the current draft; ErrNotFound when none exists.
	ReadReleaseDraft(context.Context, tenancy.Context, id.UUID) (domain.ReleaseDraft, error)
	// SaveReleaseDraft creates the draft (ExpectedRevision 0) or updates it
	// under the expected revision; a lost race returns ReleaseConflictError
	// with the current state.
	SaveReleaseDraft(context.Context, DraftParams) (domain.ReleaseDraft, error)
	// RestoreReleaseDraft CAS-updates the draft with the source version link.
	RestoreReleaseDraft(context.Context, RestoreParams) (domain.ReleaseDraft, error)
	// SaveConfigVersion appends the next monotonic version under the locked
	// draft row and bumps the revision.
	SaveConfigVersion(context.Context, VersionParams) (domain.ConfigVersion, error)
	// SaveManifest appends the sealed manifest; ErrManifestExists protects
	// the one-manifest-per-version invariant.
	SaveManifest(context.Context, SealParams) (domain.ReleaseManifest, error)
	// ReadConfigVersion returns one immutable version.
	ReadConfigVersion(context.Context, tenancy.Context, id.UUID, int64) (domain.ConfigVersion, error)
	// ListConfigVersions returns every version, newest first.
	ListConfigVersions(context.Context, tenancy.Context, id.UUID) ([]domain.ConfigVersion, error)
	// ListManifests returns every sealed manifest with its stored payload,
	// newest first.
	ListManifests(context.Context, tenancy.Context, id.UUID) ([]ManifestRecord, error)
}
