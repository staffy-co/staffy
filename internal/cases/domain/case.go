package domain

import (
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Case material of the first slice
// ---------------------------------------------------------------------------

// Case is the single unit of one customer contact: a stable business
// number under the tenant, the serving employee of the acting decision and
// the honest open state of this slice.
type Case struct {
	TenantID   id.UUID
	CaseID     id.UUID
	CaseNo     int64
	EmployeeID id.UUID
	State      string
	CreatedAt  time.Time
}

// Conversation is one channel session inside a case: the stable business
// number, the channel session as an external identifier (never a proof of
// identity), the initial response owner with the fixed epoch and the only
// writable surface of the slice.
type Conversation struct {
	TenantID           id.UUID
	CaseID             id.UUID
	ConversationID     id.UUID
	ConversationNo     int64
	EmployeeID         id.UUID
	ConnectionID       id.UUID
	Provider           string
	ProviderSessionID  string
	ResponseOwnerID    id.UUID
	ResponseOwnerEpoch int64
	WritableSurface    string
	State              string
	CreatedAt          time.Time
}

// WorkItem is the single untriaged work item of a new conversation: the
// honest "запрос ещё не классифицирован" until Story 2-2.
type WorkItem struct {
	TenantID       id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	WorkItemID     id.UUID
	State          string
	CreatedAt      time.Time
}

// PreCaseSession is the durable record of the channel session behind a
// conversation with its declared capture policy — legal-basis enforcement
// (AC-8) is deliberately deferred to the privacy-binding slice; the record
// and the declared policy are kept here.
type PreCaseSession struct {
	TenantID          id.UUID
	SessionID         id.UUID
	ConnectionID      id.UUID
	Provider          string
	ProviderSessionID string
	Modality          string
	CapturePolicy     string
	CreatedAt         time.Time
}

// IngressRejection is one honest refusal row: the kind, the safe text, the
// correlation ID and the bounded counter. The payload is never stored with
// a refusal.
type IngressRejection struct {
	TenantID      id.UUID
	RejectionID   id.UUID
	Kind          string
	Message       string
	CorrelationID id.UUID
	ConnectionID  id.UUID
	Counter       int
	CreatedAt     time.Time
}

// OperationalException is the human queue of the contour: digest conflicts,
// ambiguous routing and idempotency collisions raise one append-only
// record with safe metadata and the correlation ID.
type OperationalException struct {
	TenantID        id.UUID
	ExceptionID     id.UUID
	Kind            string
	Summary         string
	CorrelationID   id.UUID
	EventID         id.UUID
	OriginalEventID id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	CreatedAt       time.Time
}

// IngestResult is the material outcome the sender receives: the stable
// identifiers of the accepted event and its case material, or the replay
// marker of an identical redelivery.
type IngestResult struct {
	EventID         id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	CaseNo          int64
	ConversationNo  int64
	ResponseOwnerID id.UUID
	EmployeeID      id.UUID
	NewCase         bool
	Replay          bool
}

// ---------------------------------------------------------------------------
// Timeline views (the operator surface reads)
// ---------------------------------------------------------------------------

// TimelineEvent is one event of the conversation timeline: the channel,
// modality, both times (the provider time is never trusted as the receive
// time), the connection and the state.
type TimelineEvent struct {
	EventID           id.UUID
	Channel           string
	Modality          string
	Direction         string
	Provider          string
	ProviderEventID   string
	State             string
	ProviderTimestamp time.Time
	ReceivedAt        time.Time
	PayloadDigest     string
	CorrelationID     id.UUID
}

// CaseSummary is one row of the cases list.
type CaseSummary struct {
	CaseID        id.UUID
	CaseNo        int64
	EmployeeID    id.UUID
	State         string
	CreatedAt     time.Time
	Conversations int
	Events        int
	LastEventAt   time.Time
	OwnerID       id.UUID
}

// CaseDetail is the full read of one case: conversations with their owners
// and surfaces, the timeline, the work items and the related operational
// exceptions.
type CaseDetail struct {
	Case          Case
	Conversations []Conversation
	Timeline      []TimelineEvent
	WorkItems     []WorkItem
	Exceptions    []OperationalException
}

// SealedPayload is the read of one sealed event payload: the digest and
// the HMAC-verified blob (the seal check is the read-time integrity guard).
type SealedPayload struct {
	EventID id.UUID
	Digest  string
	SealOK  bool
	Payload []byte
}
