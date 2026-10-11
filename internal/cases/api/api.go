package api

import (
	"context"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Case contour scope levels (the readiness precedent): viewing the cases
// surface requires review, configuration or audit; the sealed payload
// requires review or audit — a member without any of them gets an honest
// refusal, a non-member a plain 404 so tenant existence is never
// disclosed.
const (
	CaseScopeView    = "review-or-configuration-or-audit"
	CaseScopePayload = "review-or-audit"
)

// IngestCommand is the decoded, structurally valid ingress delivery. The
// ingress is not a user principal: the tenant scope is derived from the
// verified connection, never from the envelope claim alone.
type IngestCommand struct {
	Envelope       casesDomain.Envelope
	IdempotencyKey string
	CorrelationID  id.UUID
	Now            time.Time
}

// IngestOutcome is the material result of one ingress delivery.
type IngestOutcome struct {
	// Accepted is set when the event (or its identical redelivery) is
	// material.
	Accepted *IngestResult `json:"accepted,omitempty"`
	// Refused is the honest refusal: the ladder kind, the safe text and
	// the bounded counter.
	Refused *IngestRefusal `json:"refused,omitempty"`
	// Quarantined names the quarantine kind and the operational exception
	// raised for the human.
	Quarantined *IngestQuarantine `json:"quarantined,omitempty"`
}

type IngestResult struct {
	EventID         id.UUID `json:"event_id"`
	CaseID          id.UUID `json:"case_id"`
	ConversationID  id.UUID `json:"conversation_id"`
	CaseNo          int64   `json:"case_no"`
	ConversationNo  int64   `json:"conversation_no"`
	ResponseOwnerID id.UUID `json:"response_owner_id"`
	EmployeeID      id.UUID `json:"employee_id"`
	NewCase         bool    `json:"new_case"`
	Replay          bool    `json:"replay"`
}

type IngestRefusal struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Counter int    `json:"counter,omitempty"`
	Bounded bool   `json:"bounded"`
}

type IngestQuarantine struct {
	Kind        string  `json:"kind"`
	ExceptionID id.UUID `json:"exception_id"`
	EventID     id.UUID `json:"event_id"`
}

// CasesView is the page/API projection of the cases list.
type CasesView struct {
	TenantID   id.UUID                            `json:"tenant_id"`
	Cases      []casesDomain.CaseSummary          `json:"cases"`
	Exceptions []casesDomain.OperationalException `json:"operational_exceptions"`
	CanView    bool                               `json:"can_view"`
	ViewScope  string                             `json:"view_scope"`
}

// CaseView is the full page/API projection of one case: the conversations
// with owners and surfaces, the timeline, the DERIVED work items (the Work
// Control Card material) and the related exceptions. The honest statements
// of the slice are part of the view contract; the fact VALUES follow the
// same review/audit boundary as the payload (CanViewFacts).
type CaseView struct {
	TenantID        id.UUID                            `json:"tenant_id"`
	Case            casesDomain.Case                   `json:"case"`
	Conversations   []casesDomain.Conversation         `json:"conversations"`
	Timeline        []casesDomain.TimelineEvent        `json:"timeline"`
	WorkItems       []casesDomain.WorkItemCurrent      `json:"work_items"`
	Exceptions      []casesDomain.OperationalException `json:"operational_exceptions"`
	UntriagedText   string                             `json:"untriaged_note"`
	NoAutoReplyText string                             `json:"no_auto_reply_note"`
	CanViewPayload  bool                               `json:"can_view_payload"`
	CanViewFacts    bool                               `json:"can_view_facts"`
	CanManageWork   bool                               `json:"can_manage_work"`
	ViewScope       string                             `json:"view_scope"`
	// TaxonomyTypes is the journeys dictionary of the pack pinned on the
	// case (the Work Item type dictionary of the Work Control Card forms);
	// empty when the pin has no readable pack (the honest pre-migration
	// state — the classification forms fall back to the text input).
	TaxonomyTypes []string `json:"taxonomy_types"`
	// CriticalEntities is the material key dictionary of the pinned pack:
	// the UI hint of the fact forms (the frozen spec: the hints come from
	// the pack, never a hardcoded list). Empty mirrors TaxonomyTypes — the
	// form degrades to the generic placeholder, honestly.
	CriticalEntities []string `json:"critical_entities"`
	// The Story 2.3 sections: the derived assurance state with the records
	// and the allowed methods, the channel bindings (with the reversals)
	// and the duplicate candidate cards (the masked minimal material).
	Assurance  AssuranceView             `json:"assurance"`
	Bindings   []casesPort.BindingCard   `json:"bindings"`
	Duplicates []casesPort.DuplicateCard `json:"duplicates"`
	// The Story 2.4 section: the derived ownership state with the
	// history, the outgoing replies and the member options of the
	// transfer forms.
	Ownership OwnershipView `json:"ownership"`
	// The Story 2.5 section: the conversation mode derivation with the
	// ladder, the term history, the Work Item contexts and the AI
	// disclosure records with their delivery statuses.
	Mode ModeView `json:"mode"`
	// The Story 2.6 section: the progressive intake proposals and the
	// deterministic Policy Outcomes per Work Item (the CURRENT outcome is
	// the maximal number — the sales reply gate reads it).
	Policy PolicyView `json:"policy"`
}

// PayloadView is the sealed payload read under review/audit: the digest,
// the seal check result and the payload itself.
type PayloadView struct {
	EventID       id.UUID `json:"event_id"`
	PayloadDigest string  `json:"payload_digest"`
	SealOK        bool    `json:"seal_ok"`
	Payload       []byte  `json:"-"`
}

// DrainResult is the outcome of the idempotent outbox drain command.
type DrainResult struct {
	Considered int `json:"considered"`
	Deferred   int `json:"deferred"`
}

// Service is the application surface consumed by the transport layer.
// IngestEvent is the machine ingress; the reads are the operator surface;
// DrainOutbox is the idempotent delivery command; the Work* commands and
// the facts view are the Story 2.2 surface (idempotent, optimistic).
type Service interface {
	IngestEvent(context.Context, IngestCommand) (IngestOutcome, error)
	CasesView(context.Context, domain.Principal, id.UUID) (CasesView, error)
	CaseView(context.Context, domain.Principal, id.UUID, id.UUID) (CaseView, error)
	ReadPayload(context.Context, domain.Principal, id.UUID, id.UUID, id.UUID) (PayloadView, error)
	DrainOutbox(context.Context, domain.Principal, id.UUID) (DrainResult, error)
	WorkService
	VerificationService
	OwnershipService
	ModeService
	PolicyService
}

// Clock and IDs follow the module conventions.
type Clock interface{ Now() time.Time }

type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}

// ---------------------------------------------------------------------------
// Story 2.2: the ten work commands, the facts view and the WCC projections
// ---------------------------------------------------------------------------

// WorkCommandInput is the shared material of one operator work command: the
// actor, the tenant, the case and the work item anchor, the optimistic
// concurrency expectation, the honest reason and the idempotency key.
type WorkCommandInput struct {
	Actor           domain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	WorkItemID      id.UUID
	ExpectedVersion int64
	Reason          string
	IdempotencyKey  string
	Now             time.Time
}

// CreateWorkItemCommand creates one typed and titled work item for a new
// intention of an EXISTING conversation (the automatic link does not exist
// as a class — the operator binds events explicitly).
type CreateWorkItemCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	Type           string
	Title          string
	Reason         string
	IdempotencyKey string
	Now            time.Time
}

// ClassifyCommand classifies (or re-classifies) one work item: the type
// comes from the taxonomy of the pack pinned on the case, the title and the
// reason are the operator material. The command writes a new version — the
// history is never rewritten.
type ClassifyCommand struct {
	WorkCommandInput
	Type  string
	Title string
}

// TransitionCommand moves one work item along the state ladder; the
// completion of one work item never closes the conversation or the case.
type TransitionCommand struct {
	WorkCommandInput
	ToState string
}

// SplitCommand splits one work item into at least two typed and titled
// targets: the source goes terminal `split`, the targets are born in_work
// with the backward reference.
type SplitCommand struct {
	WorkCommandInput
	Targets []casesDomain.SplitTarget
}

// MergeSourceInput is one merge source with its own optimistic concurrency
// expectation.
type MergeSourceInput struct {
	WorkItemID      id.UUID
	ExpectedVersion int64
}

// MergeCommand merges the sources into one target: the sources go terminal
// `merged` with merged_into, the fact scopes move by NEW versions (no fact
// rows are duplicated), the event links are appended onto the target.
type MergeCommand struct {
	WorkCommandInput
	Sources []MergeSourceInput
}

// LinkEventCommand binds one accepted event of the case to one active work
// item — the explicit operator action (there is no automatic link).
type LinkEventCommand struct {
	WorkCommandInput
	EventID id.UUID
}

// DisputeCommand disputes one split/merge (or any other work command
// outcome): the dispute is an append-only operational exception for the
// human, never a rewrite.
type DisputeCommand struct {
	WorkCommandInput
	Note string
}

// RecordFactCommand records one fact: the material key, the canonical value
// with the full provenance. A channel_event source cites the event of this
// case; an operator source carries the operator's own words.
type RecordFactCommand struct {
	Actor           domain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	WorkItemID      id.UUID
	MaterialKey     string
	Value           []byte
	SourceType      string
	SourceID        string
	EventChannel    string
	EventObservedAt time.Time
	EventReceivedAt time.Time
	Reason          string
	IdempotencyKey  string
	Now             time.Time
}

// ConfirmFactVersionCommand confirms the EXACT version of one fact: the
// confirmer of this slice is operator_relay, the evidence is an accepted
// event of the case or a note.
type ConfirmFactVersionCommand struct {
	Actor           domain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	FactID          id.UUID
	VersionNo       int64
	ConfirmerType   string
	EvidenceEventID id.UUID
	Note            string
	IdempotencyKey  string
	Now             time.Time
}

// RescopeFactCommand moves the scope of one fact to another active work
// item of the same case by writing a NEW version (the fact anchor and the
// rows are never duplicated).
type RescopeFactCommand struct {
	Actor           domain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	FactID          id.UUID
	TargetWorkItem  id.UUID
	ExpectedVersion int64
	Reason          string
	IdempotencyKey  string
	Now             time.Time
}

// WorkOutcome is the stored and replayed material result of one work
// command: the command name, the material identifiers it wrote and the
// replay marker of an idempotent redelivery.
type WorkOutcome struct {
	Command     string    `json:"command"`
	Replay      bool      `json:"replay"`
	WorkItemID  id.UUID   `json:"work_item_id,omitempty"`
	Version     int64     `json:"version,omitempty"`
	State       string    `json:"state,omitempty"`
	Type        string    `json:"type,omitempty"`
	Title       string    `json:"title,omitempty"`
	EventID     id.UUID   `json:"event_id,omitempty"`
	TargetIDs   []id.UUID `json:"target_ids,omitempty"`
	SourceIDs   []id.UUID `json:"source_ids,omitempty"`
	MergedInto  id.UUID   `json:"merged_into,omitempty"`
	FactID      id.UUID   `json:"fact_id,omitempty"`
	FactVersion int64     `json:"fact_version,omitempty"`
	FactStatus  string    `json:"fact_status,omitempty"`
	ExceptionID id.UUID   `json:"exception_id,omitempty"`
	// The Story 2.3 material: the verification, binding and duplicate
	// identifiers with the canonical outcome, the assurance level, the
	// resolution decision and the client confirmation text.
	VerificationID       id.UUID `json:"verification_id,omitempty"`
	VerificationKind     string  `json:"verification_kind,omitempty"`
	VerificationOutcome  string  `json:"verification_outcome,omitempty"`
	AssuranceLevel       string  `json:"assurance_level,omitempty"`
	BindingID            id.UUID `json:"binding_id,omitempty"`
	DuplicateCandidateID id.UUID `json:"duplicate_candidate_id,omitempty"`
	DuplicateDecision    string  `json:"duplicate_decision,omitempty"`
	ConfirmationText     string  `json:"confirmation_text,omitempty"`
	// The Story 2.4 material: the epoch of the ownership transition,
	// the transfer request, the reply and its outcome, and the honest
	// gate refusal of the client reply (the current owner, the epoch
	// and the permitted path — the UI shows it directly).
	Epoch             int64   `json:"epoch,omitempty"`
	TransferRequestID id.UUID `json:"transfer_request_id,omitempty"`
	TransferDecision  string  `json:"transfer_decision,omitempty"`
	ReplyID           id.UUID `json:"reply_id,omitempty"`
	ReplyState        string  `json:"reply_state,omitempty"`
	GateOwnerKind     string  `json:"gate_owner_kind,omitempty"`
	GateOwnerName     string  `json:"gate_owner_name,omitempty"`
	GateCurrentEpoch  int64   `json:"gate_current_epoch,omitempty"`
	GatePermittedPath string  `json:"gate_permitted_path,omitempty"`
	// The Story 2.5 material: the mode term of the transition, the
	// disclosure record with its number, the delivery event kind and the
	// reply class the producer marked.
	ModeTermID   id.UUID `json:"mode_term_id,omitempty"`
	ModeVersion  int64   `json:"mode_version,omitempty"`
	ModeScope    string  `json:"mode_scope,omitempty"`
	DisclosureID id.UUID `json:"disclosure_id,omitempty"`
	DisclosureNo int64   `json:"disclosure_no,omitempty"`
	DeliveryKind string  `json:"delivery_kind,omitempty"`
	ReplyClass   string  `json:"reply_class,omitempty"`
	// The Story 2.6 material: the proposal of the submission, the
	// outcome of the evaluation (the class word) and the Policy
	// Outcome the sales reply passed through.
	ProposalID      id.UUID `json:"proposal_id,omitempty"`
	OutcomeID       id.UUID `json:"outcome_id,omitempty"`
	OutcomeNo       int64   `json:"outcome_no,omitempty"`
	OutcomeClass    string  `json:"outcome_class,omitempty"`
	PolicyOutcomeID id.UUID `json:"policy_outcome_id,omitempty"`
}

// FactView is one fact of the facts view: the anchor, the derived status,
// the versions with the full provenance (both values of a conflict are
// always present) and the confirmations.
type FactView struct {
	FactID        id.UUID                        `json:"fact_id"`
	MaterialKey   string                         `json:"material_key"`
	Status        string                         `json:"status"`
	NextAction    string                         `json:"next_action"`
	Versions      []casesDomain.FactVersion      `json:"versions"`
	Confirmations []casesDomain.FactConfirmation `json:"confirmations"`
}

// FactsView is the read of the facts of one case: the review/audit boundary
// applies (the values are case content).
type FactsView struct {
	TenantID  id.UUID    `json:"tenant_id"`
	CaseID    id.UUID    `json:"case_id"`
	Facts     []FactView `json:"facts"`
	CanView   bool       `json:"can_view"`
	ViewScope string     `json:"view_scope"`
}

// The command surface of Story 2.2 (all ten commands are idempotent by the
// tenant-scoped key; every anchor mutation carries expected_version).
type WorkService interface {
	CreateWorkItem(context.Context, CreateWorkItemCommand) (WorkOutcome, error)
	ClassifyWorkItem(context.Context, ClassifyCommand) (WorkOutcome, error)
	TransitionWorkItem(context.Context, TransitionCommand) (WorkOutcome, error)
	SplitWorkItem(context.Context, SplitCommand) (WorkOutcome, error)
	MergeWorkItems(context.Context, MergeCommand) (WorkOutcome, error)
	LinkEvent(context.Context, LinkEventCommand) (WorkOutcome, error)
	DisputeWorkItem(context.Context, DisputeCommand) (WorkOutcome, error)
	RecordFact(context.Context, RecordFactCommand) (WorkOutcome, error)
	ConfirmFactVersion(context.Context, ConfirmFactVersionCommand) (WorkOutcome, error)
	RescopeFact(context.Context, RescopeFactCommand) (WorkOutcome, error)
	FactsView(context.Context, domain.Principal, id.UUID, id.UUID) (FactsView, error)
}

// ---------------------------------------------------------------------------
// Story 2.3: the four verification commands and the surface projections
// ---------------------------------------------------------------------------

// StartVerificationCommand records one verification of the case participant:
// the kind, the method, the provider-observed code (the domain maps it into
// the canonical outcome — the provider spelling never lands in a record),
// the declared subject handle, the evidence (an accepted event of the case
// or a note — never a bare code) and the authority scope (the authority
// kind only). There is NO field for a one-time code, a password or a
// secret: the command surface itself refuses to carry one.
type StartVerificationCommand struct {
	Actor           domain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	Kind            string
	Method          string
	ProviderCode    string
	SubjectHandle   string
	AuthorityScope  string
	EvidenceEventID id.UUID
	Note            string
	IdempotencyKey  string
	Now             time.Time
}

// BindChannelCommand links one conversation (the new channel session, born
// with its own case) into an existing conversation and case: the consent
// handoff requires the verified identity (and the authority with its scope
// when the binding is on behalf), the purpose and the evidence of the
// client's agreement.
type BindChannelCommand struct {
	Actor               domain.Principal
	TenantID            id.UUID
	CaseID              id.UUID
	ConversationID      id.UUID
	BoundCaseID         id.UUID
	BoundConversationID id.UUID
	Method              string
	Purpose             string
	OnBehalf            bool
	SubjectHandle       string
	EvidenceEventID     id.UUID
	Note                string
	IdempotencyKey      string
	Now                 time.Time
}

// UnlinkChannelCommand reverses one binding: the append-only reversal keeps
// the whole history (the events, the access records), the derived views
// recompute for the subsequent operations.
type UnlinkChannelCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	CaseID         id.UUID
	BindingID      id.UUID
	Reason         string
	IdempotencyKey string
	Now            time.Time
}

// ResolveDuplicateCommand decides one duplicate candidate: the same-client
// decision creates the duplicate-review binding atomically (the resolution
// and the link are one transaction); the separate decision keeps the two
// requests apart. The note is the reviewer's recorded basis.
type ResolveDuplicateCommand struct {
	Actor               domain.Principal
	TenantID            id.UUID
	CaseID              id.UUID
	CandidateID         id.UUID
	Decision            string
	Note                string
	ConversationID      id.UUID
	BoundConversationID id.UUID
	SubjectHandle       string
	IdempotencyKey      string
	Now                 time.Time
}

// AssuranceView is the assurance section of the case view: the derived
// state (identity/authority separately, the step-up flag), the records with
// the canonical outcomes, the allowed methods of the conversation modality
// (the honest step-up list of the refusal) and the fixed fallback options
// dictionary. The absence of a record is the honest none — it never becomes
// a confirmed identity.
type AssuranceView struct {
	State           casesDomain.AssuranceState       `json:"state"`
	Records         []casesDomain.VerificationRecord `json:"records"`
	AllowedMethods  []string                         `json:"allowed_methods"`
	FallbackOptions []string                         `json:"fallback_options"`
	StepUpText      string                           `json:"step_up_text"`
}

// VerificationService is the command surface of Story 2.3 (all four
// commands are idempotent by the tenant-scoped key; the binding gate and
// the step-up threshold are the honest protected steps of the slice).
type VerificationService interface {
	StartVerification(context.Context, StartVerificationCommand) (WorkOutcome, error)
	BindChannel(context.Context, BindChannelCommand) (WorkOutcome, error)
	UnlinkChannel(context.Context, UnlinkChannelCommand) (WorkOutcome, error)
	ResolveDuplicate(context.Context, ResolveDuplicateCommand) (WorkOutcome, error)
}
