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
// with owners and surfaces, the timeline, the work items and the related
// exceptions. The honest statements of the slice (the request is not
// classified yet; no auto-reply is sent) are part of the view contract.
type CaseView struct {
	TenantID        id.UUID                            `json:"tenant_id"`
	Case            casesDomain.Case                   `json:"case"`
	Conversations   []casesDomain.Conversation         `json:"conversations"`
	Timeline        []casesDomain.TimelineEvent        `json:"timeline"`
	WorkItems       []casesDomain.WorkItem             `json:"work_items"`
	Exceptions      []casesDomain.OperationalException `json:"operational_exceptions"`
	UntriagedText   string                             `json:"untriaged_note"`
	NoAutoReplyText string                             `json:"no_auto_reply_note"`
	CanViewPayload  bool                               `json:"can_view_payload"`
	ViewScope       string                             `json:"view_scope"`
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

// Service is the application surface of Story 2.1 consumed by the
// transport layer. IngestEvent is the machine ingress; the reads are the
// operator surface; DrainOutbox is the idempotent delivery command.
type Service interface {
	IngestEvent(context.Context, IngestCommand) (IngestOutcome, error)
	CasesView(context.Context, domain.Principal, id.UUID) (CasesView, error)
	CaseView(context.Context, domain.Principal, id.UUID, id.UUID) (CaseView, error)
	ReadPayload(context.Context, domain.Principal, id.UUID, id.UUID, id.UUID) (PayloadView, error)
	DrainOutbox(context.Context, domain.Principal, id.UUID) (DrainResult, error)
}

// Clock and IDs follow the module conventions.
type Clock interface{ Now() time.Time }

type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}
