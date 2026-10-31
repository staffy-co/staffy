package port

import (
	"context"
	"errors"
	"time"

	"github.com/staffy-co/staffy/internal/action/domain"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	entitlementDomain "github.com/staffy-co/staffy/internal/entitlement/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.4: the consumer-owned ports of the preparation contour
// ---------------------------------------------------------------------------
//
// The DAG rule (AD-2): the action module never imports the cases /
// employee / connector / entitlement / control MODULES — the composition
// root adapts their published api surfaces onto these narrow contracts.
// The method sets are the structural-typing precedents of 3-1/3-3: any
// adapter with the exact method set satisfies the port, and the concrete
// module services satisfy most of them without any glue at all.
//
// Three ports are the spec's own naming: OwningSystemReader (the fresh
// material reads), DryRunner (the validation evidence), and
// ControlEffectiveReader (the live scoped control of the preparation
// route). The remaining narrow readers (the case material, the member
// facts, the entitlement admission, the employee readiness, the connector
// capability state, the release lineage) are the caller-authority fact
// ports of the casesPort precedent — the app layer has ALREADY verified
// the actor's membership and rights when it calls them, so the reads do
// not re-require a UI authority, exactly like MembersFacts /
// ConnectionsFacts / RouteFacts of the earlier stories.

// The store contract errors (the consumer-owned vocabulary of the
// repository boundary; the transport maps them in step-03).
var (
	// ErrStalePreviewVersion is the honest raced-ladder refusal: the
	// expected version/attempt number moved under the advisory locks —
	// the caller re-derives, never merges.
	ErrStalePreviewVersion = errors.New("preview version ladder moved")
	// ErrAttemptExhausted is the honest terminal refusal of the AD-7
	// ladder: both attempts of the proposal key exist, the third does
	// not exist (the store CHECK is the structural bound).
	ErrAttemptExhausted = errors.New("proposal attempts exhausted")
	// ErrPreviewIntegrity is the loud alarm of a digest that did not
	// re-verify under the locks — the row is not the row the app built
	// (structurally impossible through the guards; never masked).
	ErrPreviewIntegrity = errors.New("preview digest did not re-verify")
)

// ---------------------------------------------------------------------------
// The three named ports of the spec
// ---------------------------------------------------------------------------

// FreshReadRequest is the read-only request of one material snapshot: the
// tenant scope, the owning system word of the definition (catalog / erp),
// the case and work item anchors, the definition pin, and the required
// material fields of the definition (the 2.7 dictionary words). The reader
// answers one row per requested field, never more.
type FreshReadRequest struct {
	Scope             tenancy.Context
	OwningSystem      string
	CaseID            id.UUID
	WorkItemID        id.UUID
	DefinitionID      string
	DefinitionVersion string
	MaterialFields    []string
}

// OwningSystemReader is the consumer-owned read-only port of the fresh
// material of the owning system (AD-3): the adapter answers the CURRENT
// external identity, version, observation time and evidence reference of
// every requested material field with the four-word reader status
// (fresh / stale / conflicting / unknown). A NIL reader is the honest
// production boundary (AG-4: no real providers exist) — the app asks no
// question at all and the readiness derivation blocks with the honest
// blocked_material word; the reader statuses are re-mapped through
// MapSnapshotStatus against the definition's DECLARED tolerance, so a
// lying adapter can never smuggle the allowed_uncertain word in.
type OwningSystemReader interface {
	// FreshReads reads the current material of the requested fields.
	// ONE attempt per command: no retry, no circuit breaker (the full
	// provider discipline comes with the real providers).
	FreshReads(ctx context.Context, request FreshReadRequest) ([]domain.MaterialSnapshotRow, error)
}

// DryRunRequest is the validation request of one EXACT preview version:
// the built material only (the projected typed input — schema fields, no
// secrets, no unknown fields — with the immutable pins). The adapter
// carries NO write tool: an external effect is impossible by the port
// type (AG-4 — a mutating action would be a separate definition).
type DryRunRequest struct {
	Scope             tenancy.Context
	PreviewID         id.UUID
	VersionNo         int64
	DefinitionID      string
	DefinitionVersion string
	Target            domain.ProposalTarget
	TypedInput        map[string]string
}

// DryRunOutcome is the validation evidence of one dry-run call: the
// three-word result dictionary (valid / invalid / error) and the bounded
// safe details. The evidence is NOT an action and NOT a guarantee — the
// fixed card texts carry the honesty (AC-8).
type DryRunOutcome struct {
	Result  string
	Details string
}

// DryRunner is the consumer-owned validation port: the fixture world
// wires the synthetic adapter (scriptable outcomes), the production root
// wires NOTHING (AG-4 — the surface honestly refuses in words, no
// evidence row is ever created).
type DryRunner interface {
	// Validate performs one validation attempt of the pinned material.
	Validate(ctx context.Context, request DryRunRequest) (DryRunOutcome, error)
}

// ControlRoute is the typed control route of the preparation: the
// connector pin of the definition's required capability and the
// autonomy-class word of the definition's conditions (the tenant point is
// implicit — every read is tenant-scoped). These are the 3.2 scope kinds
// that name an action; the delivery-channel point belongs to the 3-3
// drain route, not to the preparation contour.
type ControlRoute struct {
	ConnectorID   string
	AutonomyClass string
}

// ControlEffective is the safe effective control reading of the route:
// the four-word mode (a corrupted stored value READS as paused — the most
// safe applicable, fail-closed by reading), the generation of the winning
// point (0 — honestly nothing restricts the route) and the winning reason.
type ControlEffective struct {
	Mode       string
	Generation int64
	Reason     string
}

// ControlEffectiveReader is the consumer-owned read of the scoped control
// of the preparation route (the 3.2 dictionary verbatim — the step-01
// mirror is byte-exact, the slash inside intake_only/no_promises is part
// of the word). A nil port, a resolution error or a foreign word fails
// closed through GuardPreparePolicy with the control_mode reason — the
// preparation never opens on an unreadable control.
type ControlEffectiveReader interface {
	// EffectiveFor resolves the most-restrictive effective control of
	// the preparation route (tenant + connector + autonomy class).
	EffectiveFor(ctx context.Context, tenantID id.UUID, route ControlRoute) (ControlEffective, error)
}

// ---------------------------------------------------------------------------
// The narrow caller-authority fact ports (the casesPort precedent)
// ---------------------------------------------------------------------------

// CaseMaterialSource is the case surface the preparation reads through
// (the cases api Service satisfies it structurally): the full case view
// (the live mode term, the live owner epoch anchor, the work item
// anchors, the policy outcome numbers, the current observations, the
// conversations with their connections) and the facts view (the live fact
// versions for the freshness re-check). The caller has ALREADY verified
// the actor's membership and rights.
type CaseMaterialSource interface {
	CaseView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (casesAPI.CaseView, error)
	FactsView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (casesAPI.FactsView, error)
}

// MembersSource is the identity member facts read (caller-authority: the
// caller verified its scope; the concrete identity service satisfies it
// structurally — the casesPort.MembersSource precedent).
type MembersSource interface {
	MembersFacts(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]identityDomain.Membership, error)
}

// EntitlementSource is the narrow admission read (the entitlement api
// Service satisfies it structurally): the derived admission state of the
// tenant — the three-word dictionary of 1-11/1-14 (the byte-exact
// mirror is pinned by the step-01 unit matrix; the action module reads
// the published domain type through this port and never imports the
// entitlement module).
type EntitlementSource interface {
	AdmissionView(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (entitlementDomain.AdmissionState, error)
}

// ReadinessSource is the narrow employee test-and-release read (the
// employee api ReadinessService satisfies it structurally): the active
// release admission (the live stage word) and the gate instances (the
// live PASS evidence of the required gates).
type ReadinessSource interface {
	AdmissionHistoryView(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID) (employeeAPI.AdmissionHistoryView, error)
	GateInstancesView(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID) (employeeAPI.GateInstancesView, error)
}

// ConnectorFactsSource is the safe cross-module connection read
// (caller-authority — the connector api Service satisfies it
// structurally): the derived capability reports of the tenant
// connections. The preparation matches the definition's connector pin
// and reads the capability state word plus the connection contract
// version (the freshness pin of the capability evidence).
type ConnectorFactsSource interface {
	ConnectionsFacts(ctx context.Context, scope tenancy.Context) ([]connectorAPI.ConnectionFacts, error)
}

// RouteFactsSource is the release-lineage read (the employee facade —
// the casesPort.RouteFactsSource precedent): the acting release decision
// of the case's connection carries the profile version (the release
// manifest pin of the freshness derivation) and the employee identity.
type RouteFactsSource interface {
	RouteFacts(ctx context.Context, scope tenancy.Context, connectionID id.UUID) (employeeAPI.RouteFacts, error)
}

// ---------------------------------------------------------------------------
// The repository contract of the preview store
// ---------------------------------------------------------------------------

// ProposalAttemptRow is one append-only attempt of the AD-7 ladder: the
// derived proposal key, the bounded attempt number, the stable preview
// thread identity, the typed pins, the verdict word, the policy block
// reason word (” when the attempt was not policy-blocked) and the honest
// invalid report (the spoken word list of the typed violations).
type ProposalAttemptRow struct {
	ProposalKey       string
	AttemptNo         int
	PreviewID         id.UUID
	DefinitionID      string
	DefinitionVersion string
	CaseID            id.UUID
	WorkItemID        id.UUID
	TargetKind        string
	TargetRef         id.UUID
	Verdict           string
	BlockedReason     string
	InvalidReport     string
	ActorID           id.UUID
	CreatedAt         time.Time
}

// PreviewEventRow is one append-only event of the preparation history: the
// six-word kind, the version scope (0 = the proposal stage of the
// thread), the safe reason word, the safe details and the actor.
type PreviewEventRow struct {
	PreviewID id.UUID
	VersionNo int64
	CaseID    id.UUID
	EventNo   int64
	Kind      string
	Reason    string
	Details   string
	ActorID   id.UUID
	CreatedAt time.Time
}

// RecordAttemptParams is the append of one attempt row with its stage
// event (proposed / repaired at version 0): the terminal and blocked
// paths record the ladder honestly WITHOUT building any version.
type RecordAttemptParams struct {
	Scope            tenancy.Context
	Attempt          ProposalAttemptRow
	StageEventKind   string
	StageEventReason string
	StageEventID     id.UUID
	Now              time.Time
}

// BuildPreviewVersionParams is the ONE transaction that builds a version:
// the fully-derived preview material (the store re-derives the version
// ladder, re-validates the attempt ladder and re-computes the keyed
// digest under the advisory locks), the optional attempt row of the
// building submission (nil for the edit path — an edit consumes no
// attempt), the stage event of the attempt, and the superseded version of
// the edit path (0 = the first version). The supersede event of the OLD
// version lands in the SAME transaction — atomic by construction.
type BuildPreviewVersionParams struct {
	Scope               tenancy.Context
	Preview             domain.ActionPreview
	Attempt             *ProposalAttemptRow
	AttemptStageEventID id.UUID
	SupersedesVersion   int64
	SupersededEventID   id.UUID
	PreviewedEventID    id.UUID
	Now                 time.Time
}

// RecordDryRunParams is the append of the dry_run_recorded evidence event
// of one exact version (the event NEVER moves the Timeline words — the
// kind is simply not in DerivePreviewState, AC-8).
type RecordDryRunParams struct {
	Scope     tenancy.Context
	PreviewID id.UUID
	VersionNo int64
	CaseID    id.UUID
	EventID   id.UUID
	Result    string
	Details   string
	ActorID   id.UUID
	Now       time.Time
}

// PreviewRepository is the persistence contract of the preparation
// contour: the attempt ladder appends, the version builds, the dry-run
// evidence appends, and the RLS reads of the view (the versions, the
// events and the attempts of one case). The store owns NO adapters —
// every transaction is a pure persistence boundary (the 3-3
// persist-before-emit discipline: no network call can precede a commit
// because no adapter exists to call).
type PreviewRepository interface {
	// ReadProposalAttempts reads the attempt ladder of one proposal key
	// ordered by the attempt number (the submit chain input).
	ReadProposalAttempts(ctx context.Context, scope tenancy.Context, proposalKey string) ([]ProposalAttemptRow, error)
	// ReadCaseAttempts reads every attempt row of one case ordered by
	// time (the view material of the honest attempt history).
	ReadCaseAttempts(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]ProposalAttemptRow, error)
	// ReadPreviewVersions reads every version row of one case ordered
	// by the preview and the monotonic number (the view input).
	ReadPreviewVersions(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]domain.ActionPreview, error)
	// ReadPreviewEvents reads every event of one case ordered by the
	// preview, the version scope and the monotonic number.
	ReadPreviewEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]PreviewEventRow, error)
	// ReadPreviewKey reads (or creates on the first use) the per-tenant
	// commitment key inside its own transaction (the 2.9/3.3 ensure
	// pattern — INSERT ON CONFLICT DO NOTHING, the INSERT-only
	// discipline, the rotation honestly deferred).
	ReadPreviewKey(ctx context.Context, scope tenancy.Context, now time.Time) ([]byte, error)
	// RecordProposalAttempt appends one attempt row and its stage event
	// under the proposal advisory lock (the raced twin is the honest
	// ladder refusal).
	RecordProposalAttempt(ctx context.Context, p RecordAttemptParams) error
	// BuildPreviewVersion builds one version (and the attempt row, and
	// the stage/previewed/superseded events) under the preview advisory
	// lock — ONE transaction, atomic supersede.
	BuildPreviewVersion(ctx context.Context, p BuildPreviewVersionParams) error
	// RecordDryRunEvent appends the dry_run_recorded evidence event of
	// one exact version under the preview advisory lock.
	RecordDryRunEvent(ctx context.Context, p RecordDryRunParams) error
}
