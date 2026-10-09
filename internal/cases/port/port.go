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
	// PackID/PackVersion are the Domain Pack pin of the admitting
	// decision — the taxonomy the case row carries for its whole
	// lifecycle (Story 2.2).
	PackID      string
	PackVersion string
	Now         time.Time
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
	// Story 2.2 — the work command surface:
	// LookupCommand answers the idempotency question before any domain
	// guard: the stored outcome replays an exact repeat, another request
	// hash under the same key is an honest conflict (the 1-9 precedent).
	LookupCommand(ctx context.Context, scope tenancy.Context, command, key string) (CommandLookup, error)
	// ReadCasePin reads one case row: the pack pin and the existence
	// (a missing case is an honest not-found).
	ReadCasePin(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesDomain.Case, error)
	// ReadWorkItemCurrents resolves the derived current views of the
	// given work items (missing anchors are absent from the map).
	ReadWorkItemCurrents(ctx context.Context, scope tenancy.Context, caseID id.UUID, ids []id.UUID) (map[id.UUID]casesDomain.WorkItemCurrent, error)
	// ReadEventOfCase reads one accepted event of the case (the link and
	// the evidence validation input).
	ReadEventOfCase(ctx context.Context, scope tenancy.Context, caseID, eventID id.UUID) (casesDomain.ChannelEvent, error)
	// ReadFactMaterial reads one fact with its versions and
	// confirmations.
	ReadFactMaterial(ctx context.Context, scope tenancy.Context, caseID, factID id.UUID) (FactMaterial, error)
	// ReadFactMaterialByKey resolves the fact anchor of one material key
	// of the case (an absent anchor returns the empty material).
	ReadFactMaterialByKey(ctx context.Context, scope tenancy.Context, caseID id.UUID, materialKey string) (FactMaterial, error)
	// ReadFactsOfCase reads every fact of the case with its versions and
	// confirmations (the Work Control Card read).
	ReadFactsOfCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]FactMaterial, error)
	// LookupFactVersionBySource resolves the material idempotency tuple
	// of a fact version (the source wrote the same material value
	// already — the exact repeat replays, another value is a new
	// version).
	LookupFactVersionBySource(ctx context.Context, scope tenancy.Context, sourceType, sourceID, materialKey, valueDigest string) (casesDomain.FactVersion, bool, error)
	// ExecuteWorkCommand runs one transactional work command: the
	// advisory locks of every anchor in the deterministic order, the
	// FOR UPDATE re-validation of the expected versions, the append-only
	// inserts of the command material and the command record with the
	// outcome JSON.
	ExecuteWorkCommand(ctx context.Context, p WorkCommandParams) error
	// Story 2.3 — the verification, binding and duplicate surface:
	// ReadConversationOfCase reads one conversation of the case (the
	// verification context and the binding target validation).
	ReadConversationOfCase(ctx context.Context, scope tenancy.Context, caseID, conversationID id.UUID) (casesDomain.Conversation, error)
	// ReadVerificationRecords reads the verification records of one case
	// ordered by the issue time (the assurance derivation input).
	ReadVerificationRecords(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.VerificationRecord, error)
	// ReadBindingsTouchingCase reads the bindings that target the case or
	// bind one of its conversations, with the reversals (the activity
	// anti-join input).
	ReadBindingsTouchingCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ChannelBinding, []casesDomain.BindingReversal, error)
	// ReadDuplicatesOfCase reads the candidate cards touching the case (the
	// new or the existing side) with the resolutions.
	ReadDuplicatesOfCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]DuplicateCard, error)
	// ReadIdentifierKey reads (or creates on the first use) the per-tenant
	// HMAC key of the identifier tokens. INSERT-only: the creation race
	// resolves by ON CONFLICT DO NOTHING + re-read.
	ReadIdentifierKey(ctx context.Context, scope tenancy.Context, now time.Time) ([]byte, error)
}

// ---------------------------------------------------------------------------
// Story 2.2: the work command persistence surface
// ---------------------------------------------------------------------------

// PackTaxonomy resolves the journeys dictionary of one Domain Pack version
// (the Work Item type taxonomy). The concrete employee service satisfies it
// structurally (the manifest catalog is immutable in the binary: no TOCTOU
// exists between the pin and the read).
type PackTaxonomy interface {
	PackJourneys(packID, packVersion string) ([]string, error)
	// PackCriticalEntities resolves the material key dictionary of the
	// pinned pack — the fact form hints of the Work Control Card. The
	// same immutability argument holds: the catalog resolves the same
	// dictionary for the same (id, version), no TOCTOU exists.
	PackCriticalEntities(packID, packVersion string) ([]string, error)
}

// CommandLookup is the idempotency answer of one command key BEFORE any
// domain guard: the stored request hash and the stored outcome JSON. The
// same hash replays the stored outcome; another hash is an honest conflict.
type CommandLookup struct {
	Found       bool
	RequestHash string
	Outcome     []byte
}

// WorkTargetParams is one split target: the app-generated anchor id with
// the type and the title.
type WorkTargetParams struct {
	WorkItemID id.UUID
	Type       string
	Title      string
}

// WorkSourceParams is one merge source with its own optimistic concurrency
// expectation.
type WorkSourceParams struct {
	WorkItemID      id.UUID
	ExpectedVersion int64
}

// WorkCommandParams is the material of one transactional work command: the
// scope, the command name with the idempotency key and the request hash,
// the anchor expectations and the command-specific material. The outcome
// JSON is the material result the app computed (the store persists it with
// the command record; the identifiers were generated before the
// transaction, the anchor expectations are re-validated under the locks).
type WorkCommandParams struct {
	Scope           tenancy.Context
	Command         string
	Key             string
	RequestHash     string
	Actor           id.UUID
	CaseID          id.UUID
	WorkItemID      id.UUID
	ConversationID  id.UUID
	ExpectedVersion int64
	Type            string
	Title           string
	ToState         string
	Reason          string
	Targets         []WorkTargetParams
	Sources         []WorkSourceParams
	// FactLocks is the merge fact plan: the ids of every fact whose
	// CURRENT scope is one of the sources (the app read them before the
	// transaction; the store re-reads under the locks — a fact cannot
	// move INTO a source scope while the merge holds the source work
	// locks, so the planned set is the executed set).
	FactLocks       []id.UUID
	EventID         id.UUID
	FactID          id.UUID
	TargetWorkItem  id.UUID
	MaterialKey     string
	Value           []byte
	ValueDigest     string
	SourceType      string
	SourceID        string
	Channel         string
	ObservedAt      time.Time
	ReceivedAt      time.Time
	VersionNo       int64
	ConfirmerType   string
	EvidenceEventID id.UUID
	Note            string
	ExceptionID     id.UUID
	Outcome         []byte
	Now             time.Time
	// The Story 2.3 material (the command-specific fields of the four
	// verification commands and the record_fact tokenization).
	Kind                string
	Method              string
	OutcomeName         string
	AssuranceLevel      string
	SubjectHandle       string
	AuthorityScope      string
	ExpiresAt           time.Time
	VerificationID      id.UUID
	BoundCaseID         id.UUID
	BoundConversationID id.UUID
	BindingID           id.UUID
	ReversalID          id.UUID
	CandidateID         id.UUID
	ResolutionID        id.UUID
	Decision            string
	IdentifierToken     string
	Purpose             string
	OnBehalf            bool
}

// FactMaterial is the full read of one fact: the anchor, the versions
// ordered by the version number and the confirmations.
type FactMaterial struct {
	Anchor        casesDomain.FactAnchor
	Versions      []casesDomain.FactVersion
	Confirmations []casesDomain.FactConfirmation
}

// ---------------------------------------------------------------------------
// Story 2.3: the identity verification persistence surface
// ---------------------------------------------------------------------------

// IdentityVerificationRequest is the material of one verification attempt
// the contour verifier answers: the kind, the method, the modality of the
// conversation (the eligibility dictionary) and the provider-observed code.
type IdentityVerificationRequest struct {
	Kind         string
	Method       string
	Modality     string
	ProviderCode string
}

// IdentityVerification is the verifier answer: the eligibility of the
// method for the conversation modality and the provider code the contour
// observed (the domain maps it to the canonical outcome — the provider
// spelling never lands in a record).
type IdentityVerification struct {
	Eligible          bool
	IneligibilityText string
	ProviderCode      string
}

// IdentityVerifier is the verification port: the synthetic contour verifier
// checks the eligibility and passes the observed code; the real verifiers
// (AG-4) will supply their own codes through this port without touching the
// domain. A nil verifier refuses honestly — an "ok" contour is never
// assumed.
type IdentityVerifier interface {
	Verify(ctx context.Context, request IdentityVerificationRequest) (IdentityVerification, error)
}

// DuplicateCard is the minimal review material of one candidate: the
// candidate row, both case numbers, the MASKED identifier value (the last
// two runes of the normalized value — the value itself never travels), the
// material key and the resolution when the review is closed. The protected
// content (payloads, facts) never enters this card.
type DuplicateCard struct {
	Candidate      casesDomain.DuplicateCandidate
	NewCaseNo      int64
	ExistingCaseNo int64
	MaskedValue    string
	// Channel is the channel of the first accepted event of the new case
	// (the review material "which channel" — the card never carries the
	// protected content).
	Channel string
	// IsNewSide reports whether the read case is the NEW side of the
	// candidate (the resolution form lives there — the command's case
	// discipline).
	IsNewSide  bool
	Resolution *casesDomain.DuplicateResolution
}

// BindingCard is one binding of the case view: the binding row, its
// reversal when unlinked, both case numbers and the deterministic client
// confirmation text (without the internal identifiers).
type BindingCard struct {
	Binding          casesDomain.ChannelBinding
	Reversal         *casesDomain.BindingReversal
	TargetCaseNo     int64
	BoundCaseNo      int64
	ConfirmationText string
}

// The Story 2.3 repository surface is part of Repository: the reads of the
// assurance, binding and duplicate surfaces + the identifier key of the
// HMAC tokens.
