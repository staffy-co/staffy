package api

import (
	"context"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
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
