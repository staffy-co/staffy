package api

import (
	"time"

	"context"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 3.4: the proposal→preview preparation surface (the DTO layer —
// the transport renders THESE shapes only, the domain records never leak
// to the wire; the external commitment 3-3 precedent)
// ---------------------------------------------------------------------------

// SubmitProposalCommand is the machine-readable preparation request of one
// human author (AG-4: the AI Runtime does not exist). Every reference is
// a typed pin; the typed input is a strict schema-name→value map validated
// against the pinned definition's TypedInputSchema. The owner epoch is
// the author's LIVE observation of the case ownership — the policy gate
// re-checks it against the live source (a moved epoch blocks the
// preparation with the exact reason word).
type SubmitProposalCommand struct {
	Actor                  identityDomain.Principal
	TenantID               id.UUID
	CaseID                 id.UUID
	WorkItemID             id.UUID
	DefinitionID           string
	DefinitionVersion      string
	Target                 domain.ProposalTarget
	TypedInput             map[string]string
	SupportingFacts        []id.UUID
	SupportingObservations []id.UUID
	OwnerEpoch             int64
	Now                    time.Time
}

// ProposalOutcome is the honest answer of one submitted attempt: the
// verdict word of the AD-7 ladder (valid / invalid / rejected), the full
// violation word list (iff the attempt was typed-invalid — the author
// reads the exact fields back), the stable preview identity and the
// version number of the BUILT preview (iff the chain reached the build),
// the attempt number, and the policy reason word of a preparation block
// (iff a LIVE gate refused a typed-valid attempt — the transport speaks
// PolicyReasonText of the word).
type ProposalOutcome struct {
	Command       string   `json:"command"`
	Verdict       string   `json:"verdict"`
	Violations    []string `json:"violations,omitempty"`
	PreviewID     id.UUID  `json:"preview_id,omitempty"`
	VersionNo     int64    `json:"version_no,omitempty"`
	AttemptNo     int      `json:"attempt_no"`
	BlockedReason string   `json:"blocked_reason,omitempty"`
	PreviewDigest string   `json:"preview_digest,omitempty"`
}

// NewPreviewVersionCommand is the edit of one existing preview: the
// edited typed input and the edited supporting references. The pins
// (definition, case, work item, target) are the preview's own identity —
// structurally immutable per version thread; the edit re-runs the FULL
// chain (validation + policy + the live snapshot) and lands as the NEXT
// append-only version with the superseded event on the old one in the
// SAME transaction.
type NewPreviewVersionCommand struct {
	Actor                  identityDomain.Principal
	TenantID               id.UUID
	CaseID                 id.UUID
	PreviewID              id.UUID
	TypedInput             map[string]string
	SupportingFacts        []id.UUID
	SupportingObservations []id.UUID
	Now                    time.Time
}

// PreviewVersionOutcome is the answer of a built version (the edit path).
type PreviewVersionOutcome struct {
	Command       string  `json:"command"`
	PreviewID     id.UUID `json:"preview_id"`
	VersionNo     int64   `json:"version_no"`
	Supersedes    int64   `json:"supersedes_version"`
	AttemptNo     int     `json:"attempt_no"`
	PreviewDigest string  `json:"preview_digest"`
}

// DryRunCommand records the validation evidence of one EXACT preview
// version. The version pin addresses the built material; the adapter
// carries no write tool (AG-4) — the result is the three-word evidence
// vocabulary, never an action and never a guarantee.
type DryRunCommand struct {
	Actor     identityDomain.Principal
	TenantID  id.UUID
	CaseID    id.UUID
	PreviewID id.UUID
	VersionNo int64
	Now       time.Time
}

// DryRunOutcome is the honest answer of one dry-run call: the result word
// with the fixed honesty text of the card (the evidence is NOT an action
// and NOT a guarantee — AC-8), the bounded safe details, and the honest
// unsupported refusal when the production root wires no adapter (AG-4 —
// the surface never pretends).
type DryRunOutcome struct {
	Command     string `json:"command"`
	Result      string `json:"result,omitempty"`
	Details     string `json:"details,omitempty"`
	Unsupported bool   `json:"unsupported,omitempty"`
}

// PreviewSnapshotRowView is the wire shape of one material snapshot row
// (the five-word status dictionary; the price/sum of the card is spoken
// ONLY from this row's material — the preview never invents a word).
type PreviewSnapshotRowView struct {
	MaterialField   string    `json:"material_field"`
	Status          string    `json:"status"`
	ExternalID      string    `json:"external_id"`
	ExternalVersion string    `json:"external_version"`
	ObservedAt      time.Time `json:"observed_at"`
	EvidenceRef     string    `json:"evidence_ref"`
}

// PreviewEffectView is the wire shape of one effect: the order, the
// definition action pin, the depends-on orders and the failure-semantics
// word — derived ONLY from the definition, the submission cannot add one.
type PreviewEffectView struct {
	Order            int    `json:"order"`
	ActionRef        string `json:"action_ref"`
	DependsOn        []int  `json:"depends_on"`
	FailureSemantics string `json:"failure_semantics"`
}

// PreviewCardView is the full material card of ONE preview version: the
// pins, the projected typed input (schema fields only — no secrets, no
// unknown fields), the material snapshot, the finite effects, the
// partition with its honest markers, the READ-TIME derivations (the
// readiness, the freshness with the exact reason word, the Timeline word
// over the append-only events), the integrity re-check of the stored
// digest, the current-version flag of the thread and the diff lines of
// the edit that produced it.
type PreviewCardView struct {
	PreviewID                id.UUID                  `json:"preview_id"`
	VersionNo                int64                    `json:"version_no"`
	SupersedesVersion        int64                    `json:"supersedes_version,omitempty"`
	DefinitionID             string                   `json:"definition_id"`
	DefinitionVersion        string                   `json:"definition_version"`
	CaseID                   id.UUID                  `json:"case_id"`
	WorkItemID               id.UUID                  `json:"work_item_id"`
	TargetKind               string                   `json:"target_kind"`
	TargetRef                id.UUID                  `json:"target_ref"`
	TypedInput               map[string]string        `json:"typed_input"`
	SupportingFacts          []domain.VersionPin      `json:"supporting_facts"`
	SupportingObservations   []domain.VersionPin      `json:"supporting_observations"`
	OwnerEpoch               int64                    `json:"owner_epoch"`
	ControlGeneration        int64                    `json:"control_generation"`
	ProfileVersion           int64                    `json:"profile_version"`
	PolicyOutcomeNo          int64                    `json:"policy_outcome_no"`
	CapabilityDeclVersion    string                   `json:"capability_decl_version"`
	Snapshot                 []PreviewSnapshotRowView `json:"snapshot"`
	Effects                  []PreviewEffectView      `json:"effects"`
	Partition                string                   `json:"partition"`
	PartitionText            string                   `json:"partition_text"`
	RecoveryInstruction      string                   `json:"recovery_instruction,omitempty"`
	HumanOwnerRequired       bool                     `json:"human_owner_required"`
	UnsafeClass              bool                     `json:"unsafe_class"`
	NoReconciliation         bool                     `json:"no_reconciliation"`
	HumanOnlyControlMark     bool                     `json:"human_only_control_mark"`
	Readiness                string                   `json:"readiness"`
	ReadinessText            string                   `json:"readiness_text"`
	FreshnessStale           bool                     `json:"freshness_stale"`
	FreshnessReason          string                   `json:"freshness_reason,omitempty"`
	FreshnessReasonText      string                   `json:"freshness_reason_text,omitempty"`
	Timeline                 string                   `json:"timeline"`
	TimelineText             string                   `json:"timeline_text"`
	Current                  bool                     `json:"current"`
	Digest                   string                   `json:"digest"`
	DigestAlarm              bool                     `json:"digest_alarm"`
	AuthorizationAbsenceText string                   `json:"authorization_absence_text"`
	DiffLines                []string                 `json:"diff_lines,omitempty"`
	AuthorID                 id.UUID                  `json:"author_id"`
	CreatedAt                time.Time                `json:"created_at"`
}

// PreviewAttemptView is the honest attempt ladder row of one thread: the
// bounded attempt number, the verdict word, the policy block reason word,
// the invalid report and the time. The counter of the submission history
// is shown on the page honestly (the repeat invalidity is terminal, the
// third attempt does not exist).
type PreviewAttemptView struct {
	PreviewID     id.UUID   `json:"preview_id"`
	AttemptNo     int       `json:"attempt_no"`
	Verdict       string    `json:"verdict"`
	BlockedReason string    `json:"blocked_reason,omitempty"`
	InvalidReport string    `json:"invalid_report,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// PreviewDryRunEvidenceView is one validation evidence row of the thread
// (the exact version it validated, the three-word result, the time) —
// the Timeline words NEVER move on the evidence (AC-8).
type PreviewDryRunEvidenceView struct {
	VersionNo int64     `json:"version_no"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}

// PreviewThreadView is one preparation thread of the case: the stable
// preview identity, every version card (oldest first — the audit stays
// readable forever), the attempt ladder, the dry-run evidence and the
// material diff of the last edit.
type PreviewThreadView struct {
	PreviewID      id.UUID                     `json:"preview_id"`
	Cards          []PreviewCardView           `json:"cards"`
	Attempts       []PreviewAttemptView        `json:"attempts"`
	DryRunEvidence []PreviewDryRunEvidenceView `json:"dry_run_evidence"`
}

// PreviewView is the page/API projection of the Preview Review surface of
// one case: the fixed panel text of the slice disciplines (the app and
// the fixture share the ONE domain text — a private copy would drift),
// the honest empty-state words, and the threads.
type PreviewView struct {
	TenantID  id.UUID             `json:"tenant_id"`
	CaseID    id.UUID             `json:"case_id"`
	PanelText string              `json:"panel_text"`
	EmptyText string              `json:"empty_text,omitempty"`
	Threads   []PreviewThreadView `json:"threads"`
}

// IDGenerator follows the module conventions of identity/cases: the
// preview thread identities and the event identifiers are minted BEFORE
// the transaction so the store stays a pure persistence boundary.
type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}

// PreviewService is the narrow preparation surface the transport
// type-asserts (the BindCases/ActionCatalogService precedent): the
// composite app Service implements it, the fixture world binds it.
type PreviewService interface {
	// SubmitProposal runs the preparation chain of one attempt: the
	// rights gate (review|configuration) fires BEFORE any material; the
	// pinned definition resolves or the honest 404 answers; the strict
	// validation ladder (one repair, the terminal repeat) speaks the
	// exact violation words; the policy gates read the LIVE sources in
	// the frozen order (the typed text is never an input); the material
	// snapshot is read ONLY after every gate passed (the fake-repo
	// counters pin the order); the build is ONE transaction.
	SubmitProposal(ctx context.Context, command SubmitProposalCommand) (ProposalOutcome, error)

	// ReadPreviewView renders the case's Preview Review surface: every
	// version card with the live read-time derivations, the attempt
	// ladders, the dry-run evidence and the material diffs. The old
	// versions stay readable for the audit forever and lose the
	// authorization suitability through the freshness derivation — never
	// through a deletion.
	ReadPreviewView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (PreviewView, error)

	// NewPreviewVersionFromEdit re-runs the FULL chain over the edited
	// material and lands the next append-only version: the old version
	// is superseded by the event of the SAME transaction (atomic), its
	// digest stays byte-true, the future Confirmation 3-5 pins the
	// (preview_id, version_no) pair — the old version structurally never
	// authorises the changed material.
	NewPreviewVersionFromEdit(ctx context.Context, command NewPreviewVersionCommand) (PreviewVersionOutcome, error)

	// RecordDryRun appends the validation evidence of one exact version.
	// A nil adapter (the production root, AG-4) is the honest refusal in
	// words — no evidence row is ever created; the Timeline words never
	// move on the evidence (AC-8).
	RecordDryRun(ctx context.Context, command DryRunCommand) (DryRunOutcome, error)
}

// The store-contract errors the transport maps (the consumer-owned
// vocabulary of the repository boundary re-exported for the render layer).
var (
	ErrStalePreviewVersion = port.ErrStalePreviewVersion
	ErrAttemptExhausted    = port.ErrAttemptExhausted
	ErrPreviewIntegrity    = port.ErrPreviewIntegrity
)
