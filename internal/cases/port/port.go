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
	// TermID is the identifier of the initial ownership term the
	// SAME transaction writes (Story 2.4: epoch 1, the owner from the
	// human coverage, the inbox surface, the ingest_initial origin).
	TermID id.UUID
	// ModeTermID is the identifier of the initial mode term the SAME
	// transaction writes (Story 2.5: version 1, the unclassified admission
	// word, the ingest_initial origin — the ownership initial-term
	// precedent).
	ModeTermID id.UUID
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
	// Story 2.4 — the ownership surface:
	// ReadOwnershipMaterial reads the terms, the transfer requests and
	// the resolutions of one case (the derivation input).
	ReadOwnershipMaterial(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.OwnershipTerm, []casesDomain.TransferRequest, []casesDomain.TransferResolution, error)
	// ReadConversationByID reads one conversation by its identifier
	// regardless of the case (the governing-case resolution input).
	ReadConversationByID(ctx context.Context, scope tenancy.Context, conversationID id.UUID) (casesDomain.Conversation, error)
	// ReadClientReplies reads the outgoing replies of the case with
	// their outcome states (the case view material).
	ReadClientReplies(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]ReplyCard, error)
	// PendingClientReplies returns the prepared replies (no outcome
	// row) of the tenant bounded by the limit — the drain input.
	PendingClientReplies(ctx context.Context, scope tenancy.Context, limit int) ([]casesDomain.ClientReply, error)
	// ResolveReplyDrain re-validates one prepared reply under the case
	// advisory lock and writes the append-only outcome with the outbox
	// record (fresh — dispatched; stale — stale_needs_review; the
	// re-observed attention subject raises the exception once). The
	// material UNIQUE (tenant, reply) makes the drain idempotent: a
	// repeated run finds the reply decided and writes nothing.
	ResolveReplyDrain(ctx context.Context, p ReplyDrainParams) (string, bool, error)
	// Story 2.5 — the mode and disclosure surface:
	// ReadModeMaterial reads the mode terms of one case ordered by the
	// version (the derivation input of the active conversation mode, the
	// Work Item contexts and the reply mode gate).
	ReadModeMaterial(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ModeTerm, error)
	// ReadDisclosures reads the disclosure records with their delivery
	// events of one case ordered by the record number (the derivation
	// input of the acting disclosure of a channel).
	ReadDisclosures(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]DisclosureCard, error)
	// ReadLiveConversationChannel reads the channel of the latest accepted
	// event of one conversation (the live route word of the reply gates —
	// the honest derivation, never the modality fallback guess).
	ReadLiveConversationChannel(ctx context.Context, scope tenancy.Context, conversationID id.UUID) (string, bool, error)
	// Story 2.6 — the policy surface:
	// ReadAIProposals reads every AI proposal of one case ordered by the
	// creation time (the claims audit of the panel and the gate words).
	ReadAIProposals(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.AIProposal, error)
	// ReadPolicyOutcomes reads every Policy Outcome of one case ordered by
	// the Work Item and the monotonic number (the derivation input of the
	// CURRENT outcome the sales reply gate reads).
	ReadPolicyOutcomes(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.PolicyOutcome, error)
	// Story 2.7 — the owning-system surface:
	// ReadObservations reads every observation of one case ordered by the
	// Work Item, the material field and the monotonic number (the
	// derivation input of the CURRENT observation per field of the panel).
	ReadObservations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.Observation, error)
	// Story 2.8 — the objection surface:
	// ReadObjections reads every objection record of one case ordered by
	// the Work Item and the monotonic number (the derivation input of the
	// CURRENT objection, the refusal budget and the linked replies of the
	// panel and the sales reply gate).
	ReadObjections(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ObjectionRecord, error)
	// ReadLinkedReplies reads the objection links of the outgoing replies
	// of one case (the objection id with the playbook step word — the
	// acknowledged derivation input; only the linked rows participate).
	ReadLinkedReplies(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ReplyLink, error)
	// Story 2.9 — the commitment surface:
	// ReadCommitments reads every commitment version of one case ordered
	// by the commitment and the monotonic number (the derivation input of
	// the CURRENT version, the readiness and the panel history).
	ReadCommitments(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.CommitmentVersion, error)
	// ReadConfirmations reads every confirmation of one case ordered by
	// the confirmation time (the validity derivation input of the panel).
	ReadConfirmations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ConfirmationRecord, error)
	// ReadCommitmentDeclines reads every decline of one case ordered by
	// the creation time (the panel history input).
	ReadCommitmentDeclines(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]CommitmentDeclineView, error)
	// ReadPresentedCommitments reads the presentation links of the
	// outgoing sales replies of one case (the exact (commitment, version)
	// pairs — the presentation gate input of the confirm command and the
	// panel).
	// Story 2.10 — the continuation surface:
	// LookupContinuationByTokenHash resolves one continuation link by
	// the SHA-256 of the opaque token half under the tenant scope (the
	// AD-10 honesty: the hash is the ONLY lookup key — the scope
	// candidate from the token is verified by the hash BEFORE any case
	// material is read) and derives the read-time state of the given
	// moment. ErrNotFound is the generic-refusal material of the public
	// surface (the caller never distinguishes the causes to the client).
	LookupContinuationByTokenHash(ctx context.Context, scope tenancy.Context, tokenHash []byte, now time.Time) (casesDomain.ContinuationLinkLookup, error)
	// SubmitContinuationCorrection writes one public fact correction in
	// a single transaction (the append-only fact version in the
	// unverified state with the protected-continuation provenance, the
	// limit-consuming use row and the metadata-only outbox event); the
	// answer carries the new version number of the fact.
	SubmitContinuationCorrection(ctx context.Context, p ContinuationCorrectionParams) (int64, error)
	// SubmitContinuationEvent writes one public human/return event (the
	// use row without material and without consumption + the outbox
	// event; nothing is ever mutated).
	SubmitContinuationEvent(ctx context.Context, p ContinuationEventParams) error
	// ReadContinuations reads the whole continuation material of one
	// case (the links, the use feed, the revocations and the delivered
	// binding — the operator panel input; the states are derived
	// read-time).
	ReadContinuations(ctx context.Context, scope tenancy.Context, caseID id.UUID) (ContinuationPanelMaterial, error)
	// Story 2.11 — the accommodation surface:
	// ReadAccommodations reads every append-only revision record of
	// the communication preferences of one case ordered by the key and
	// the monotonic revision (the derivation input of the CURRENT set,
	// the panel history and the transfer view; the states are derived
	// read-time, never stored).
	ReadAccommodations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.AccommodationRecord, error)
	// Story 2.12 — the degradation and the delivery evidence surfaces:
	// ReadDegradations reads every append-only route degradation record
	// of one case ordered by the route key and the monotonic revision
	// (the derivation input of the CURRENT slots, the panel history and
	// the policy fifth input; the states are derived read-time, never
	// stored).
	ReadDegradations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.RouteDegradationRecord, error)
	// ReadDeliveryEvents reads every append-only delivery evidence
	// event of the replies of one case ordered by the reply and the
	// monotonic event number (the derivation input of the honest
	// delivery states — dispatched without events is the unknown, never
	// the delivered).
	ReadDeliveryEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ReplyDeliveryEvent, error)
	ReadPresentedCommitments(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.PresentedCommitment, error)
	// ReadCommitmentKey reads (or creates on the first use) the per-tenant
	// commitment key of the content binding (the ensure pattern of the
	// identifier keys — the app computes the token before the command
	// transaction).
	ReadCommitmentKey(ctx context.Context, scope tenancy.Context, now time.Time) ([]byte, error)
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
	// The Story 2.4 material (the command-specific fields of the five
	// ownership commands and the client reply drain).
	OwnerKind             string
	OwnerRef              id.UUID
	WritableSurface       string
	TransferRequestID     id.UUID
	TransferResolutionID  id.UUID
	DeadlineAt            time.Time
	ReplyID               id.UUID
	ReplyContent          string
	RouteConnectionID     id.UUID
	RouteChannel          string
	RouteDirection        string
	LineageEmployeeID     id.UUID
	LineageDecisionNo     int64
	LineageProfileVersion int64
	OutboxEventID         id.UUID
	// OwnerActiveWord is the app-side member snapshot word of the
	// command: the current owner resolves to an active member (the
	// honest snapshot read before the transaction; the ownership
	// state itself re-validates under the case lock).
	OwnerActiveWord bool
	// The Story 2.5 material (the command-specific fields of the four
	// mode commands and the reply class extension of send_client_reply).
	Mode                string
	ModeScope           string
	ModeTermID          id.UUID
	ExpectedModeVersion int64
	SignalSource        string
	TurnRef             id.UUID
	ExitAuthority       string
	DisclosureID        id.UUID
	ParentRef           id.UUID
	DisclosureText      string
	DeliveryKind        string
	DeliveryEventID     id.UUID
	ReplyClass          string
	// The Story 2.6 material (the command-specific fields of the two
	// policy commands and the policy gate extension of send_client_reply).
	ProposalID              id.UUID
	Intent                  string
	CandidateAnswer         string
	MissingFacts            []casesDomain.MissingFact
	ExtractedFacts          []casesDomain.ExtractedFact
	Proposer                string
	OutcomeID               id.UUID
	OutcomeNo               int64
	OutcomeClass            string
	OutcomeReason           string
	DecisionDigest          string
	InputSnapshot           []byte
	MandatoryFacts          []byte
	NextSteps               []byte
	ReusedFacts             []byte
	SuppressedQuestions     []byte
	ExpectedOwnerEpoch      int64
	ExpectedFactsDigest     string
	ExpectedWorkItemVersion int64
	PolicyOutcomeID         id.UUID
	// The Story 2.7 material (the command-specific fields of the
	// read_owning_system command): the observation id, the matrix row
	// snapshot words and the port answer of the read (the app read the
	// owning system BEFORE the transaction; the store re-validates the
	// anchors under the locks and writes the append-only row with the
	// per-(work item, material field) monotonic number).
	ObservationID   id.UUID
	OwningSystem    string
	Projection      string
	ReadStatus      string
	ReadReason      string
	ReadValue       []byte
	ExternalID      string
	ExternalVersion string
	CorrelationID   id.UUID
	// The Story 2.8 material (the command-specific fields of the
	// record_objection command and the objection gate extension of
	// send_client_reply): the objection id (the new record of the
	// command, or the CURRENT objection the passing gate resolved for the
	// reply row — the request never carries it, the server-side truth),
	// the spans as the canonical JSON array (the verbatim client words of
	// the turn source) and the playbook step word of the reply discipline.
	ObjectionID    id.UUID
	ObjectionType  string
	ClientSignal   string
	ObjectionSpans []byte
	PlaybookStep   string
	// The Story 2.9 material (the command-specific fields of the three
	// commitment commands and the presentation link extension of
	// send_client_reply): the stable commitment id with the exact version
	// number, the canonical conditions/action set/unknown words JSON, the
	// pins of the relations (the owner kind/ref/epoch, the lineage word,
	// the policy outcome number, the pinned price observation), the
	// content token of the version, the confirmation subject and expiry,
	// the evidence spans with the turn payload (the app verified the
	// byte-exact containment BEFORE the transaction; the store re-validates
	// the shape under the locks), the material snapshot of the
	// confirmation and the decline outcome word. The presentation pair of
	// send_client_reply reuses CommitmentID and VersionNo (both or
	// neither — the table CHECK backstop).
	CommitmentID                  id.UUID
	ConfirmationID                id.UUID
	DeclineID                     id.UUID
	CommitmentStatus              string
	CommitmentConditions          []byte
	CommitmentActionSet           []byte
	CommitmentMaterialUnknowns    []byte
	CommitmentValidUntil          time.Time
	CommitmentConsumptionLimit    int
	CommitmentOwnerKind           string
	CommitmentOwnerRef            id.UUID
	CommitmentOwnerEpoch          int64
	CommitmentLineageWord         string
	CommitmentPolicyOutcomeNo     int64
	CommitmentPinnedObservationNo int64
	CommitmentContentToken        string
	CommitmentSubjectKind         string
	CommitmentSubjectRef          string
	CommitmentExpiresAt           time.Time
	CommitmentEvidenceSpans       []byte
	CommitmentTurnPayload         []byte
	CommitmentMaterialSnapshot    []byte
	CommitmentDeclineOutcome      string
	// The Story 2.10 material (the command-specific fields of the two
	// continuation commands): the link id with the purpose set (the
	// canonical JSON array — the dictionary subset), the two operator
	// texts, the lifetime bounds, the token hash (the ONLY stored trace
	// of the capability — the token itself never travels through the
	// params), the participant reference (the channel address of the
	// client) and the revocation material (the append-only row with the
	// honest reason). The owner epoch pin is the server-side truth: the
	// store reads the CURRENT epoch under the locks, the request never
	// carries it.
	LinkID                     id.UUID
	RevocationID               id.UUID
	ContinuationPurposes       []byte
	ContinuationQuestion       string
	ContinuationExplanation    string
	ContinuationParticipantRef string
	ContinuationTTLMinutes     int
	ContinuationLimit          int
	ContinuationTokenHash      []byte
	ContinuationReason         string
	// The Story 2.11 material (the command-specific fields of the
	// record_accommodation command): the record id of the append-only
	// revision row, the preference key word, the valued material (the
	// channel word of preferred_channel or the free availability line —
	// the value and the note never enter the outbox), the clearing mark
	// and the operator note. The revision number is the server-side
	// truth: the store derives the next monotonic number of the key
	// under the locks (the request never carries it).
	AccommodationRecordID id.UUID
	AccommodationKey      string
	AccommodationValue    string
	AccommodationCleared  bool
	AccommodationNote     string
	// The Story 2.12 material (the command-specific fields of the
	// record_route_degradation and the record_delivery_evidence
	// commands): the record id of the append-only degradation revision
	// row with its route (the connection id and the channel word of the
	// 2.1 dictionary), the cause word, the clearing mark and the
	// operator note (the note never enters the outbox); the event id of
	// the append-only evidence row with its reply reference and the
	// kind word. The revision number and the event number are the
	// server-side truth: the store derives the next monotonic numbers
	// under the locks (the request never carries them).
	DegradationRecordID     id.UUID
	DegradationConnectionID id.UUID
	DegradationChannel      string
	DegradationCause        string
	DegradationCleared      bool
	DegradationNote         string
	DeliveryEvidenceEventID id.UUID
	DeliveryEvidenceReplyID id.UUID
	DeliveryEvidenceKind    string
	// The accuracy qualifier word of the send_client_reply extension: the
	// SALES reply against an outcome with active alternatives carries the
	// honest limitation word of the outcome's active set (the word lands
	// in the reply row; the store re-validates the gate under the locks).
	AccuracyQualifier string
	// The Story 2.12 update promise word of the send_class reply
	// extension: a reply on a DEGRADED route carries the promise word of
	// the approved policy dictionary (the word lands in the reply row;
	// the store re-validates the classless timing gate under the locks).
	UpdatePromise string
}

// ContinuationCorrectionParams is the material of one public fact
// correction through the protected surface: the tenant scope (the
// principal is the inert anonymous scope — the RLS checks the tenant
// only; the write rows carry the resolved link id as their actor), the
// token hash (the verified credential half — the lookup key; the token
// itself never enters the store), the corrected fact with its raw value
// bytes (the store canonicalizes through the domain value guard under
// the locks) and the ids of the use row and the outbox event.
type ContinuationCorrectionParams struct {
	Scope         tenancy.Context
	TokenHash     []byte
	FactID        id.UUID
	Value         []byte
	UseID         id.UUID
	OutboxEventID id.UUID
	CorrelationID id.UUID
	Now           time.Time
}

// ContinuationEventParams is the material of one public human/return
// event: the scope, the token hash, the use kind word and the ids of
// the use row and the outbox event. The event mutates nothing: the use
// row and the envelope only.
type ContinuationEventParams struct {
	Scope         tenancy.Context
	TokenHash     []byte
	UseKind       string
	UseID         id.UUID
	OutboxEventID id.UUID
	CorrelationID id.UUID
	Now           time.Time
}

// ContinuationPanelMaterial is the operator panel read of the
// continuation surface: the links of the case with their uses, the
// revocations and the delivered binding set (the link ids the outgoing
// reply rows carry — the FR-16 «the main channel shows whether the link
// was delivered»).
type ContinuationPanelMaterial struct {
	Links       []casesDomain.ContinuationLink
	Uses        []casesDomain.ContinuationLinkUse
	Revocations []casesDomain.ContinuationRevocation
	Delivered   map[id.UUID]bool
}

// ---------------------------------------------------------------------------
// Story 2.4: the ownership persistence surface
// ---------------------------------------------------------------------------

// ReplyDrainParams is the material of one execute-time revalidation of the
// drain: the prepared reply, the app-side facts read at the drain start
// (the owner activity word of the member facts and the route admission
// word of the route facts — honest snapshots), the generated ids and the
// clock. The store re-validates the ownership state under the case
// advisory lock and writes the append-only outcome.
type ReplyDrainParams struct {
	Scope         tenancy.Context
	Reply         casesDomain.ClientReply
	OutcomeID     id.UUID
	OutboxEventID id.UUID
	ExceptionID   id.UUID
	OwnerActive   bool
	RouteAdmitted bool
	Now           time.Time
}

// ReplyCard is one outgoing reply row of the case view: the reply with its
// derived state (the outcome kind, empty while prepared).
type ReplyCard struct {
	Reply       casesDomain.ClientReply
	OutcomeKind string
	OutcomeAt   time.Time
}

// DisclosureCard is one disclosure record of the case view: the record
// with its delivery events and the derived status (the kind of the last
// event; the empty word is the honest pre-migration absence).
type DisclosureCard struct {
	Record casesDomain.DisclosureRecord
	Status string
	Events []casesDomain.DisclosureDeliveryEvent
}

// CommitmentDeclineView is one append-only decline row of the commitment
// surface (the panel history input — the spans stay in the row, the view
// carries the metadata only).
type CommitmentDeclineView struct {
	TenantID     id.UUID
	CaseID       id.UUID
	WorkItemID   id.UUID
	DeclineID    id.UUID
	CommitmentID id.UUID
	VersionNo    int64
	Outcome      string
	TurnRef      id.UUID
	ActorID      id.UUID
	CreatedAt    time.Time
}

// OwnerMember is one active member option of the transfer forms: the
// principal id with the honest name (the email of the membership).
type OwnerMember struct {
	PrincipalID id.UUID
	Name        string
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

// ---------------------------------------------------------------------------
// Story 2.7: the owning-system read port
// ---------------------------------------------------------------------------

// OwningSystemReadRequest is the typed read request of one material field
// of the authority matrix: the scope context (the tenant with the acting
// principal of the command), the material key and the matrix row words (the
// owning system word with the permitted projection — the reader answers in
// the projection terms), the external reference (the anchor the owning
// system names the entity by — the v1 carrier is the work item of the case),
// the correlation context and the read moment (the provider time basis of
// the synthetic contour; the real providers own their clocks). A provider
// DTO never crosses this boundary — the typed request is the only question
// an owning system answers.
type OwningSystemReadRequest struct {
	Scope             tenancy.Context
	MaterialKey       string
	OwningSystem      string
	Projection        string
	ExternalReference string
	CorrelationID     id.UUID
	Now               time.Time
}

// OwningSystemReading is the observation material of one read: the status
// word of the dictionary, the reason word of the failure vocabulary (a
// failed/unknown read only), the canonical projection value bytes (ok
// only), the external id of the value carrier, the provider version of the
// value and the provider-observed time (never the receive time — the 2-2
// discipline). The value bytes never travel into an error, a log or an
// envelope — only into the observation row and the panel.
type OwningSystemReading struct {
	Status          string
	Reason          string
	Value           []byte
	ExternalID      string
	ExternalVersion string
	ObservedAt      time.Time
}

// OwningSystemReader reads the CURRENT value of one material field from the
// owning system declared by the authority matrix (AD-2 consumer-owned
// port): the synthetic adapter of the test contour answers deterministically
// over the fixed scenario dictionary; the real providers (AG-4) will pass
// through the same port without touching the domain. A nil reader refuses
// honestly before any record (the «коннектор без верификатора» precedent —
// an "ok" contour is never assumed). ONE attempt per command: no retry, no
// circuit breaker, no rate limit (the full provider discipline comes with
// the real providers).
type OwningSystemReader interface {
	Read(ctx context.Context, request OwningSystemReadRequest) (OwningSystemReading, error)
}

// The Story 2.3 repository surface is part of Repository: the reads of the
// assurance, binding and duplicate surfaces + the identifier key of the
// HMAC tokens.
