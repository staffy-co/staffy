package port

import (
	"context"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// SourceVerifier verifies the origin of an ingress delivery before any
// durable record except a rejection: the synthetic verifier of the test
// contour (connection exists, environment test); production sources stay
// fail-closed (no provider signature schemes exist — AG-4).
type SourceVerifier interface {
	VerifySource(ctx context.Context, tenantClaim id.UUID, connectionID id.UUID) (casesDomain.SourceVerification, error)
}

// RouteFactsSource is the narrow employee facade (the
// ConnectionsFacts/MembersFacts precedent): the acting release decisions
// that seal the connection, with the per-route results and the human
// coverage of the pinned profile versions, plus the relayed entitlement
// admission word. The caller (the ingress) has already verified its own
// scope from the trusted connection metadata — this read never re-requires
// a user principal. The concrete employee service satisfies it
// structurally.
type RouteFactsSource interface {
	RouteFacts(ctx context.Context, scope tenancy.Context, connectionID id.UUID) (employeeAPI.RouteFacts, error)
}

// ConnectionFactsSource is the single-connection facts read (connector):
// the trusted-scope read of the connection state for the ladder step that
// follows the replay check.
type ConnectionFactsSource interface {
	ConnectionFactsByID(ctx context.Context, scope tenancy.Context, connectionID id.UUID) (connectorAPI.ConnectionFacts, error)
}

// MembersSource is the identity member facts read (caller-authority: the
// caller verified its scope; the principal slot of the ingress scope
// carries the verified connection id, never a user principal). The
// concrete identity service satisfies it structurally.
type MembersSource interface {
	MembersFacts(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]identityDomain.Membership, error)
}

// Clock and IDs follow the module conventions.
type Clock interface{ Now() time.Time }

type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}

// ---------------------------------------------------------------------------
// Repository
// ---------------------------------------------------------------------------

// ReplayLookup is the outcome of the pre-admission idempotent lookup: the
// material identity (provider event id) AND the idempotency key are both
// resolved — a key held by another accepted event is an honest conflict,
// never an internal error.
type ReplayLookup struct {
	Found         bool
	DigestMatch   bool
	Result        casesDomain.IngestResult
	OriginalEvent casesDomain.ChannelEvent
	// KeyConflict is set when the delivery's idempotency key is already
	// held by an accepted event of ANOTHER provider event id.
	KeyConflict   bool
	KeyOwnerEvent casesDomain.ChannelEvent
}

// AcceptParams is the single-transaction admission write: the event, the
// sealed payload, the case material and the two outbox records.
type AcceptParams struct {
	Scope           tenancy.Context
	Event           casesDomain.ChannelEvent
	Payload         []byte
	SealKey         []byte
	Seal            string
	Session         casesDomain.PreCaseSession
	EmployeeID      id.UUID
	ResponseOwnerID id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	WorkItemID      id.UUID
	SessionID       id.UUID
	OutboxEventID   id.UUID
	AckEventID      id.UUID
	// AcceptedPayload is the struct form of the accepted-event outbox
	// record: the store resolves the case material INSIDE the transaction
	// and marshals the canonical JSON with the true identifiers — the
	// payload is never pre-encoded with fabricated ones.
	AcceptedPayload casesDomain.AcceptedEventPayload
	AckPayload      []byte
	Now             time.Time
}

// QuarantineParams is the quarantined-delivery write.
type QuarantineParams struct {
	Scope           tenancy.Context
	Event           casesDomain.ChannelEvent
	ExceptionKind   string
	ExceptionText   string
	ExceptionID     id.UUID
	OriginalEventID id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	Now             time.Time
}

// RejectionParams is the honest-refusal write (no payload by construction).
type RejectionParams struct {
	Scope         tenancy.Context
	RejectionID   id.UUID
	Kind          string
	Message       string
	CorrelationID id.UUID
	ConnectionID  id.UUID
	Counter       int
	Now           time.Time
}

// Repository is the persistence surface of the cases module.
type Repository interface {
	// LookupEvent resolves the material idempotency before the admission
	// ladder: an accepted event with the same digest replays the original
	// result; another digest marks the delivery a digest conflict; an
	// idempotency key held by another accepted event marks a key conflict.
	LookupEvent(ctx context.Context, scope tenancy.Context, provider, providerEventID, payloadDigest, idempotencyKey string) (ReplayLookup, error)
	// AcceptEvent performs the one-transaction admission: the advisory lock
	// on the material key, the re-check, the find-or-create of the case
	// material and the inserts of the event, the sealed payload and the
	// outbox records.
	AcceptEvent(ctx context.Context, p AcceptParams) (casesDomain.IngestResult, error)
	// QuarantineEvent records the quarantined delivery and the operational
	// exception without touching the accepted material.
	QuarantineEvent(ctx context.Context, p QuarantineParams) (casesDomain.ChannelEvent, casesDomain.OperationalException, error)
	// RecordRejection commits one refusal row in its own transaction.
	RecordRejection(ctx context.Context, p RejectionParams) (casesDomain.IngressRejection, error)
	// CountInflight counts the accepted events of the connection inside the
	// load-envelope window.
	CountInflight(ctx context.Context, scope tenancy.Context, connectionID id.UUID, window time.Duration, now time.Time) (int, error)
	// CountRejections counts the recorded refusals of one KIND inside the
	// window — the bound of the pre-verification rejection rows. The count
	// is per tenant and kind (never per connection: the connection id of
	// an unverifiable delivery is attacker-chosen, varying it must not
	// multiply the budget).
	CountRejections(ctx context.Context, scope tenancy.Context, kind string, window time.Duration, now time.Time) (int, error)
	// ListCases, ReadCase, ReadPayload and ListExceptions are the operator
	// surface reads.
	ListCases(ctx context.Context, scope tenancy.Context) ([]casesDomain.CaseSummary, error)
	ReadCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesDomain.CaseDetail, error)
	ReadPayload(ctx context.Context, scope tenancy.Context, caseID, eventID id.UUID) (casesDomain.SealedPayload, error)
	ListExceptions(ctx context.Context, scope tenancy.Context) ([]casesDomain.OperationalException, error)
	// PendingOutbox returns the outbox events without an append-only
	// delivery outcome (the idempotent drain input).
	PendingOutbox(ctx context.Context, scope tenancy.Context, limit int) ([]casesDomain.OutboxEvent, error)
	// RecordDelivery appends one delivery outcome.
	RecordDelivery(ctx context.Context, scope tenancy.Context, delivery casesDomain.OutboxDelivery) error
}
