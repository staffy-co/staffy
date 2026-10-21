package port

import (
	"context"
	"errors"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.6 (step-02): the consumer-owned ports of the dispatch contour —
// the mutation adapter, the dispatch repository and the fencing error
// dictionary
// ---------------------------------------------------------------------------
//
// The mutation adapter is the ONLY door to the provider world (AD-6: the
// mutation adapters are importable ONLY by the dispatch composition root
// and the test contours — the import-graph probe of step-04 pins that the
// product binary never imports the synthetic implementation). It carries
// NO secrets: the request is the rebuilt material plus the derived key
// identities; the answer is the claim word, the correlation, the external
// object identity triple and the LIVE guarantee flag read from the
// connector catalog declaration at the call moment.
//
// The dispatch repository owns the THREE transactions of the contour and
// NONE of them takes a single adapter: AcquireDispatchLease is the
// write-ahead right to send (lease + ledger submitted in ONE commit),
// RecordDispatchOutcome is the fenced verdict (outcome + ledger + receipt
// v1 + outbox receipt.created in ONE commit), AppendDispatchBlocked is
// the raced-guarded parking append. A network call is impossible BY TYPE
// inside the store — persist-before-emit is the signature itself (the
// Enqueue precedent of 3-3, the ReserveOperation precedent of 3-5).

// The store-contract errors of the dispatch contour (the consumer-owned
// fencing vocabulary of the repository boundary; the transport maps them
// in step-03).
var (
	// ErrLeaseStale is the honest fenced refusal of the outcome recording:
	// the (attempt_no, lease_token) pair does not own the current attempt —
	// a newer attempt exists, the token is foreign, or the outcome of this
	// attempt is already recorded. The stale holder NEVER writes history
	// (AC-1); the caller reports the refusal, never a merge.
	ErrLeaseStale = errors.New("dispatch lease stale")
	// ErrLeaseRaced is the honest racing signal: an active un-expired lease
	// of another consideration owns the operation right now — the second
	// drain skips honestly (no second right to send), and a raced block
	// append skips the same way (the GasExternalCommitmentQueue precedent
	// of 3-3).
	ErrLeaseRaced = errors.New("dispatch lease raced")
	// ErrDispatchRefusedAG4 is the honest production refusal of the drain
	// command at a nil mutation adapter (AG-4: no real providers exist —
	// the execution contour is not wired, the queue honestly stays
	// pending; the DrainExternalCommitments precedent of 3-3).
	ErrDispatchRefusedAG4 = errors.New("mutation execution not wired")
	// ErrOutcomeDuplicate is the honest repeat signal of the outcome
	// recording: the attempt already carries its outcome row — the verdict
	// never lands twice (the append-only audit stays one row per attempt).
	ErrOutcomeDuplicate = errors.New("dispatch outcome already recorded")
)

// The pending classification words (six): the structural dispatch decision
// vocabulary of the drain — the ONE dispatcher of decisions (every job is
// processed, not every job is sent). fresh — authorized, no lease ever;
// recoverable — the write-ahead resting state of a crash (submitted with
// an expired lease and no outcome); retryable — an unknown verdict is
// recorded (the retry policy decides over the recorded guarantee);
// decided — accepted/committed/failed (a repeat is not a policy question);
// blocked — authorized and parked by a dispatch_blocked boundary (the
// fences re-run: the cause may have healed); racing — an active lease of
// another consideration owns the operation (the honest skip).
const (
	PendingClassFresh       = "fresh"
	PendingClassRecoverable = "recoverable"
	PendingClassRetryable   = "retryable"
	PendingClassDecided     = "decided"
	PendingClassBlocked     = "blocked"
	PendingClassRacing      = "racing"
)

// PendingClassWords is the full dictionary (6 words).
var PendingClassWords = []string{
	PendingClassFresh, PendingClassRecoverable, PendingClassRetryable,
	PendingClassDecided, PendingClassBlocked, PendingClassRacing,
}

// ---------------------------------------------------------------------------
// The mutation adapter: the consumer-owned door to the provider world
// ---------------------------------------------------------------------------

// MutationRequest is the delivery material of ONE attempt: the rebuilt
// payload (deterministic, from the immutable rows only), the attempt
// number, the EXTERNAL idempotency key (ONE key per operation, stable
// across attempts — the provider deduplicates the repeats) and the keyed
// request digest (the redacted trace of exactly this material). The raw
// payload travels ONLY here — it is never stored anywhere (AD-10/AC-5).
type MutationRequest struct {
	TenantID               id.UUID
	OperationID            id.UUID
	AttemptNo              int
	ExternalIDempotencyKey string
	RequestDigest          string
	DefinitionID           string
	DefinitionVersion      string
	Payload                domain.AuthorizedActionPayload
}

// MutationResult is the honest answer of the adapter: the claim word (the
// four-word dictionary — the guards of the domain fail it closed on a
// foreign word), the provider correlation id, the STRUCTURAL evidence of
// a terminal success (the external object identity triple — committed
// survives only with the complete triple), the LIVE guarantee flag read
// from the connector catalog declaration at the call moment (the
// guarantee word is recorded only for the honest timeout claim) and the
// safe details (identifiers and words only, bounded by the store CHECK).
type MutationResult struct {
	Outcome               string
	CorrelationID         string
	ExternalObjectID      string
	ExternalObjectVersion string
	ExternalObjectTime    time.Time
	GuaranteeLive         bool
	Details               string
}

// MutationAdapter is the consumer-owned mutation port (AD-6): Dispatch
// performs the provider call OUTSIDE every store transaction (the app
// layer owns the ordering — lease first, verdict after), ServesCapability
// carries the destination allowlist word the composition root declares
// (the app cross-checks it against the definition's required capability
// BEFORE the call — a foreign capability is the honest refusal without a
// send). The port carries NO secrets: the credentials live behind the
// adapter, never in the request.
type MutationAdapter interface {
	// ServesCapability reports the capability word this adapter serves.
	ServesCapability() string
	// Dispatch performs ONE provider call of ONE attempt. An error is the
	// call failure itself — the caller records the honest unknown without
	// a guarantee (never a masked success, never a blind repeat).
	Dispatch(ctx context.Context, scope tenancy.Context, request MutationRequest) (MutationResult, error)
}

// ---------------------------------------------------------------------------
// The row shapes of the dispatch repository
// ---------------------------------------------------------------------------

// PendingDispatchJob is ONE queued job of the drain: the outbox record
// (the at-least-once delivery unit — the duplicate rows of one operation
// are legal), the immutable Operation, the append-only ledger with the
// derived state word, the structural classification, and the attempt
// facts (the latest lease/outcome, the first recorded request digest and
// external key — the decided-verify inputs).
type PendingDispatchJob struct {
	EventID                id.UUID
	RawPayload             []byte
	Operation              domain.Operation
	Ledger                 []domain.OperationLedgerEntry
	StateWord              string
	Classification         string
	AttemptCount           int
	LastAttemptNo          int
	LastOutcomeWord        string // "" when the latest attempt carries no outcome row
	LastOutcomeGuarantee   bool
	LastOutcomeCorrelation string
	LastLeaseToken         string
	LastLeaseExpiresAt     time.Time
	LastLeaseActive        bool
	RequestDigest          string // the FIRST lease digest ("" when no lease ever)
	ExternalIDempotencyKey string // the recorded external key ("" when no lease ever)
	// ReconciliationNoEffect is the 3-7 retry-unlock fact of the pending
	// classification: the LAST reconciliation outcome verdict of the
	// operation reads no_effect (the PROVEN absence of the external effect —
	// a failed operation unlocks a retry WITHOUT the provider-idempotent
	// guarantee; the store lateral join of 3-7 step-02 reads the verdict).
	ReconciliationNoEffect bool
}

// AcquireDispatchLeaseParams is the write-ahead transaction of the send
// right: the immutable Operation, the definition pins (the lease speaks
// what was sent by which definition version), the derived 64-hex lease
// token of THIS holder, the derived external idempotency key and request
// digest, the lease expiry (the definition timeout plus the grace) and
// the ledger submitted entry. The signature carries ZERO adapters — a
// network call is impossible BY TYPE (the Enqueue/ReserveOperation
// precedents). The store derives the monotonic attempt_no (1..3) under
// the advisory lock; ANY failure rolls back the lease AND the ledger
// entry together.
type AcquireDispatchLeaseParams struct {
	Scope                  tenancy.Context
	Operation              domain.Operation
	DefinitionID           string
	DefinitionVersion      string
	LeaseToken             string
	ExternalIDempotencyKey string
	RequestDigest          string
	LeaseExpiresAt         time.Time
	LedgerEntry            domain.OperationLedgerEntry
	Now                    time.Time
}

// ReceiptReferences is the SAFE pin set of the Action Receipt (FR-31: the
// decision is reconstructible without the dialog — identifiers, words and
// the external object identity only; the material NEVER enters, the
// sensitive values are never duplicated).
type ReceiptReferences struct {
	CaseID                id.UUID `json:"case_id"`
	WorkItemID            id.UUID `json:"work_item_id"`
	PreviewID             id.UUID `json:"preview_id"`
	PreviewVersionNo      int64   `json:"preview_version_no"`
	ConfirmationID        id.UUID `json:"confirmation_id"`
	ConfirmationVersionNo int64   `json:"confirmation_version_no"`
	DefinitionID          string  `json:"definition_id"`
	DefinitionVersion     string  `json:"definition_version"`
	ExternalObjectID      string  `json:"external_object_id"`
	ExternalObjectVersion string  `json:"external_object_version"`
}

// ReceiptCreatedPayload is the SAFE canonical outbox payload of the
// receipt.created record (AD-2: the Work Item/Conversation link is an
// event of THIS module's own outbox — a direct write into a foreign
// module's tables never happens; the future consumer reads the pending
// records, the 3-5 mirror precedent).
type ReceiptCreatedPayload struct {
	EventType        string    `json:"event_type"`
	TenantID         id.UUID   `json:"tenant_id"`
	OperationID      id.UUID   `json:"operation_id"`
	CaseID           id.UUID   `json:"case_id"`
	WorkItemID       id.UUID   `json:"work_item_id"`
	DefinitionID     string    `json:"definition_id"`
	ReceiptVersionNo int64     `json:"receipt_version_no"`
	Outcome          string    `json:"outcome"`
	ExternalObjectID string    `json:"external_object_id"`
	CreatedAt        time.Time `json:"created_at"`
}

// RecordDispatchOutcomeParams is the fenced verdict transaction: the
// attempt identity (the attempt number and the lease token of the holder
// — a stale pair NEVER writes history), the honest claim word of the
// adapter ("" records NO outcome row: the refused or absent claims — the
// lease, the ledger kind and the receipt speak the guarded truth), the
// guarded ledger kind, the receipt outcome word ("" — accepted carries
// no receipt), the structural evidence, the recorded guarantee flag, the
// correlation, the receipt row of the deciding verdict with the SAFE
// references (the version derives under the store lock — the monotonic
// append of the chain) and the outbox receipt.created record. The
// signature carries ZERO adapters; ANY failure rolls back the whole
// verdict atomically.
type RecordDispatchOutcomeParams struct {
	Scope             tenancy.Context
	Operation         domain.Operation
	AttemptNo         int
	LeaseToken        string
	OutcomeWord       string // the honest adapter claim word; "" — no outcome row
	StateWord         string // the guarded ledger kind
	ReceiptWord       string // "" when accepted — no receipt
	CorrelationID     string
	Evidence          domain.MutationEvidence
	GuaranteeRecorded bool
	Details           string
	LedgerEntry       domain.OperationLedgerEntry
	ReceiptVersionNo  int64
	ReceiptReferences ReceiptReferences
	ReceiptDetails    string
	OutboxEventID     id.UUID
	OutboxPayload     ReceiptCreatedPayload
	Now               time.Time
}

// AppendDispatchBlockedParams is the raced-guarded parking append: the
// dispatch_blocked ledger entry with the EXACT fence reason word (the
// operation stays authorized — parked with a visible cause and an allowed
// next step; never a false failed externally, AC-2). An active lease of
// another consideration refuses with ErrLeaseRaced — the honest skip.
type AppendDispatchBlockedParams struct {
	Scope       tenancy.Context
	OperationID id.UUID
	Reason      string
	Details     string
	LedgerEntry domain.OperationLedgerEntry
	Now         time.Time
}

// AppendOutboxReceiptCreatedParams is the standalone outbox append of one
// receipt.created record (the future 3-7 contour reuses it when a later
// evidence appends a new receipt version; the verdict transaction of
// RecordDispatchOutcome performs the same insert INSIDE its commit).
type AppendOutboxReceiptCreatedParams struct {
	Scope   tenancy.Context
	EventID id.UUID
	Payload ReceiptCreatedPayload
	Now     time.Time
}

// DispatchAttemptRow is one attempt of the view material: the lease facts
// (the attempt number, the token, the window) joined with the outcome
// facts when the row exists (the claim word, the correlation, the
// recorded guarantee — the retry policy input).
type DispatchAttemptRow struct {
	AttemptNo         int
	LeaseToken        string
	LeaseExpiresAt    time.Time
	LeaseCreatedAt    time.Time
	RequestDigest     string
	OutcomeWord       string // "" when the attempt carries no outcome row
	CorrelationID     string
	GuaranteeRecorded bool
	OutcomeCreatedAt  time.Time
}

// ActionReceiptRow is one version of the Action Receipt of the view
// material: the version number, the three-word outcome, the SAFE
// references and the details.
type ActionReceiptRow struct {
	ReceiptVersionNo int64
	Outcome          string
	References       ReceiptReferences
	Details          string
	CreatedAt        time.Time
}

// ---------------------------------------------------------------------------
// The dispatch repository contract
// ---------------------------------------------------------------------------

// DispatchRepository is the persistence contract of the execution contour:
// the pending classification read, the write-ahead lease transaction, the
// fenced verdict transaction, the raced-guarded parking append, the
// standalone outbox append and the view reads. The store owns NO adapters
// — every transaction is a pure persistence boundary (the signature is
// the persist-before-emit discipline itself).
type DispatchRepository interface {
	// PendingDispatchJobs reads the queued jobs of the tenant (the
	// at-least-once operation.queued records, bounded by the drain limit)
	// with the structural classification, the derived state word and the
	// attempt facts — the ONE dispatcher of decisions: every job is
	// considered, not every job is sent.
	PendingDispatchJobs(ctx context.Context, scope tenancy.Context, now time.Time, limit int) ([]PendingDispatchJob, error)
	// AcquireDispatchLease is the write-ahead transaction of the send
	// right: under the dispatch advisory lock the monotonic attempt_no
	// derives (1..3), the lease row and the ledger submitted entry land in
	// ONE commit — the right and the moment of the send are material
	// BEFORE the network call (persist-before-emit, AC-1/AC-5). An active
	// lease of another consideration refuses with ErrLeaseRaced; the
	// attempt bound refuses with the honest bound word through
	// ErrLeaseStale (the store CHECK is the loyal backstop).
	AcquireDispatchLease(ctx context.Context, p AcquireDispatchLeaseParams) (int, error)
	// RecordDispatchOutcome is the fenced verdict transaction: the
	// (attempt_no, lease_token) pair must own the current attempt — a
	// stale pair NEVER writes history (ErrLeaseStale, AC-1); the outcome
	// row (when the claim word is honest), the ledger kind, the receipt
	// version of the deciding verdict (the monotonic append under the
	// store lock — v1 first, the next version on a 3-7 retry) and the
	// outbox receipt.created record
	// land in ONE commit — ANY failure rolls back the whole verdict.
	RecordDispatchOutcome(ctx context.Context, p RecordDispatchOutcomeParams) error
	// AppendDispatchBlocked parks the operation with the EXACT fence
	// reason word (dispatch_blocked is NOT state-bearing — the operation
	// stays authorized); an active lease of another consideration refuses
	// with ErrLeaseRaced (the honest raced skip).
	AppendDispatchBlocked(ctx context.Context, p AppendDispatchBlockedParams) error
	// AppendOutboxReceiptCreated appends one receipt.created record (the
	// standalone form of the verdict insert — the future 3-7 contour).
	AppendOutboxReceiptCreated(ctx context.Context, p AppendOutboxReceiptCreatedParams) error
	// ReadDispatchAttempts reads the attempts of one operation ordered by
	// the monotonic attempt number (the view material).
	ReadDispatchAttempts(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]DispatchAttemptRow, error)
	// ReadActionReceipts reads the receipt versions of one operation
	// ordered by the version number (the view material; v1 is never
	// rewritten).
	ReadActionReceipts(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]ActionReceiptRow, error)
}
