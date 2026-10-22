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
// Story 3.7 (step-02): the consumer-owned ports of the reconciliation
// contour — the read-only query adapter, the reconciliation repository
// and the fencing error dictionary
// ---------------------------------------------------------------------------
//
// The reconciliation adapter is the ONLY door to the Owning System's
// answer world (AD-6: the query adapters are importable ONLY by the
// reconcile composition root and the test contours — the import-graph
// probe of step-04 pins that the product binary never imports the
// synthetic implementation). The port is READ-ONLY BY STRUCTURE: not a
// single mutation method — the reconciliation NEVER performs a new
// provider write before the verdict (epic AC-1; the reflect probe of the
// step-02 unit matrix pins the absence). It carries NO secrets: the
// query is the derived key identities — the SAME external idempotency
// key as the dispatch (the provider recognizes the operation), the
// correlation id of the last dispatch attempt and the keyed request
// digest of the expected material; the answer is the claimed verdict
// word, the correlation and the external object identity triple.
//
// The reconciliation repository owns the TWO adapter-free transactions
// of the contour: AcquireReconciliationLease is the write-ahead right to
// query (lease + ledger reconciling in ONE commit — the operation STAYS
// unknown: reconciling is NOT state-bearing), RecordReconciliationVerdict
// is the fenced verdict (outcome + the terminal writes of the ledger /
// the receipt v2+ / the outbox receipt.created in ONE commit — the
// terminal lock of the receipts guards the append). A network call is
// impossible BY TYPE inside the store — the signature itself is the
// discipline (the AcquireDispatchLease precedent of 3-6).

// The store-contract errors of the reconciliation contour (the
// consumer-owned fencing vocabulary of the repository boundary; the
// transport maps them in step-03).
var (
	// ErrReconciliationLeaseStale is the honest fenced refusal of the
	// verdict recording: the (attempt_no, lease_token) pair does not own
	// the current reconciliation attempt — a newer attempt exists, the
	// token is foreign, or the verdict of this attempt is already
	// recorded. The stale holder NEVER writes history (the epic
	// fault-block); the caller reports the refusal, never a merge.
	ErrReconciliationLeaseStale = errors.New("reconciliation lease stale")
	// ErrReconciliationLeaseRaced is the honest racing signal: an active
	// un-expired reconciliation lease of another consideration owns the
	// right now — the parallel command skips honestly (the maximum ONE
	// active chain of the epic fault-block; the ErrLeaseRaced precedent
	// of the dispatch contour).
	ErrReconciliationLeaseRaced = errors.New("reconciliation lease raced")
	// ErrReconcileRefusedAG4 is the honest production refusal of the
	// reconcile command at a nil reconciliation adapter (AG-4: no real
	// owning systems exist — the contour is not wired, the pending
	// targets honestly stay; the ErrDispatchRefusedAG4 precedent).
	ErrReconcileRefusedAG4 = errors.New("reconciliation not wired")
	// ErrTerminalConflict is the integrity conflict of the receipt
	// terminal lock: the last receipt version is terminal with a
	// DIFFERENT outcome than the new terminal claim — the append is
	// forbidden, the human review owns the resolution (epic AC-5: the
	// conflicting terminal outcomes create a human review instead of an
	// automatic selection — never an auto-replacement).
	ErrTerminalConflict = errors.New("terminal receipt conflict")
)

// The pending reconciliation classification words (five): the structural
// decision vocabulary of the reconcile command — the ONE reconciler of
// decisions (every pending target is considered, not every target is
// queried). fresh — the operation is unknown with the last receipt
// unknown, no reconciliation lease ever, the attempts within the bound;
// waiting — the backoff window of the last attempt is active
// (next_eligible_at in the future); racing — an active un-expired lease
// of another consideration owns the right (the honest skip: the maximum
// ONE active chain); exhausted — the three reconciliation attempts are
// spent (the honest human word: the Operation stays unknown until human
// evidence, never a success by timer); decided — the operation is no
// longer unknown-reconcilable (the last receipt is not unknown: the
// at-least-once command repeat of the decided operation is the honest
// skip «the reconciliation already decided»).
const (
	ReconcileClassFresh     = "fresh"
	ReconcileClassWaiting   = "waiting"
	ReconcileClassRacing    = "racing"
	ReconcileClassExhausted = "exhausted"
	ReconcileClassDecided   = "decided"
)

// ReconcileClassWords is the full dictionary (5 words).
var ReconcileClassWords = []string{
	ReconcileClassFresh, ReconcileClassWaiting, ReconcileClassRacing,
	ReconcileClassExhausted, ReconcileClassDecided,
}

// ---------------------------------------------------------------------------
// The read-only query adapter: the consumer-owned door to the answer world
// ---------------------------------------------------------------------------

// ReconciliationQueryRequest is the READ-ONLY query of ONE attempt: the
// derived key identities alone — the SAME external idempotency key as
// the dispatch of this operation (dispatch_key:v1: — the provider
// recognizes the operation), the correlation id of the last dispatch
// attempt (the provider's own trace), the keyed request digest of the
// expected material (the digest-comparator input of the verdict — a
// foreign material with the same key is the integrity conflict of the
// retry-unlock) and the definition pins with the declared
// ReconciliationQuery word. The raw material NEVER enters (AD-10/AC-5).
type ReconciliationQueryRequest struct {
	TenantID               id.UUID
	OperationID            id.UUID
	AttemptNo              int
	ExternalIDempotencyKey string
	CorrelationID          string
	RequestDigest          string
	DefinitionID           string
	DefinitionVersion      string
	ReconciliationQuery    string
}

// ReconciliationQueryResult is the honest answer of the adapter: the
// CLAIMED verdict word (the normalization of the domain fails it closed
// — a committed claim without the structural evidence triple and a
// no_effect claim without the declared absence semantics both read
// no_answer; a foreign word reads no_answer), the provider correlation
// id and the STRUCTURAL evidence of a terminal success (the external
// object identity triple). The adapter NEVER performs a provider write —
// the port is read-only by structure (epic AC-1).
type ReconciliationQueryResult struct {
	ClaimedVerdict        string
	CorrelationID         string
	ExternalObjectID      string
	ExternalObjectVersion string
	ExternalObjectTime    time.Time
	Details               string
}

// ReconciliationAdapter is the consumer-owned READ-ONLY query port
// (AD-6/epic AC-1): QueryOutcome performs ONE answer query of ONE
// attempt OUTSIDE every store transaction (the app layer owns the
// ordering — the lease first, the verdict after); ServesCapability
// carries the destination allowlist word the composition root declares
// (the app cross-checks it against the definition's required capability
// BEFORE the query — a foreign capability is the honest refusal without
// a call). NOT A SINGLE MUTATION METHOD — the reconciliation never
// writes to the provider world before the verdict (the reflect probe of
// the unit matrix pins the shape).
type ReconciliationAdapter interface {
	// ServesCapability reports the capability word this adapter serves.
	ServesCapability() string
	// QueryOutcome asks the Owning System ONE question: what became of
	// the external effect of this operation. An error is the call
	// failure itself — the caller records the honest no_answer (never a
	// masked success, never a masked absence).
	QueryOutcome(ctx context.Context, scope tenancy.Context, request ReconciliationQueryRequest) (ReconciliationQueryResult, error)
}

// ---------------------------------------------------------------------------
// The row shapes of the reconciliation repository
// ---------------------------------------------------------------------------

// PendingReconciliationTarget is ONE pending reconciliation target of
// the bounded command: the immutable Operation, the append-only ledger
// with the derived state word, the structural classification, the
// reconciliation attempt facts (the latest lease, the backoff window,
// the recorded verdict) and the dispatch facts the query needs (the
// FIRST lease digest, the recorded external key, the last dispatch
// correlation) with the last receipt version (the terminal-lock input
// and the unknown marker of the classification).
type PendingReconciliationTarget struct {
	Operation                 domain.Operation
	Ledger                    []domain.OperationLedgerEntry
	StateWord                 string
	Classification            string
	ReconcileAttemptCount     int
	LastReconcileAttemptNo    int
	LastLeaseToken            string
	LastLeaseExpiresAt        time.Time
	LastLeaseActive           bool
	NextEligibleAt            time.Time
	LastReconciliationVerdict string // "" when the latest attempt carries no outcome row
	RequestDigest             string // the FIRST dispatch lease digest ("" when no lease ever)
	ExternalIDempotencyKey    string // the recorded dispatch external key ("" when no lease ever)
	LastDispatchCorrelation   string // the last dispatch outcome correlation ("" when none)
	LastReceiptVersionNo      int64  // 0 when no receipt exists
	LastReceiptOutcome        string // "" when no receipt exists — the terminal-lock input
}

// AcquireReconciliationLeaseParams is the write-ahead transaction of the
// query right: the immutable Operation, the definition pins, the LIVE
// reconciliation word of the connector declaration read at the query
// moment (the declared-absence provenance), the derived 64-hex lease
// token of THIS holder, the SAME external idempotency key and request
// digest as the dispatch of this operation, the correlation id of the
// last dispatch attempt, the lease expiry (the definition timeout plus
// the grace) and the material backoff moment (next_eligible_at — the
// base plus the jitter, derived by the domain) with the ledger
// reconciling entry (NOT state-bearing — the operation STAYS unknown).
// The signature carries ZERO adapters — a network call is impossible BY
// TYPE. The store derives the monotonic attempt_no (1..3) under the
// advisory lock; ANY failure rolls back the lease AND the ledger entry
// together.
type AcquireReconciliationLeaseParams struct {
	Scope                  tenancy.Context
	Operation              domain.Operation
	DefinitionID           string
	DefinitionVersion      string
	ReconciliationWord     string
	LeaseToken             string
	ExternalIDempotencyKey string
	RequestDigest          string
	CorrelationID          string
	LeaseExpiresAt         time.Time
	NextEligibleAt         time.Time
	LedgerEntry            domain.OperationLedgerEntry
	Now                    time.Time
}

// RecordReconciliationVerdictParams is the fenced verdict transaction:
// the attempt identity (the attempt number and the lease token of the
// holder — a stale pair NEVER writes history), the NORMALIZED verdict
// word (the four-word dictionary — the outcome row records the guarded
// truth of the attempt), the ledger word ("" — in_progress/no_answer
// write NO ledger entry: the operation STAYS unknown), the receipt
// outcome word ("" — in_progress/no_answer append NO receipt version),
// the structural evidence of the committed claim, the correlation, the
// SAFE receipt references and details of the appended version and the
// outbox receipt.created record (reused for v2+ — the version rides the
// payload; the store derives the monotonic version number under the
// advisory lock and patches the payload). The signature carries ZERO
// adapters; ANY failure rolls back the whole verdict atomically. The
// terminal lock runs INSIDE the transaction under the lock: an append
// over the last unknown passes, the same-terminal confirmation skips
// the receipt silently, a conflicting terminal refuses with
// ErrTerminalConflict (never an auto-replacement).
type RecordReconciliationVerdictParams struct {
	Scope             tenancy.Context
	Operation         domain.Operation
	AttemptNo         int
	LeaseToken        string
	Verdict           string // the normalized four-word verdict — the outcome row always records
	LedgerWord        string // "" — no ledger entry (unknown stays)
	ReceiptWord       string // "" — no receipt version, no outbox record
	CorrelationID     string
	Evidence          domain.MutationEvidence
	Details           string
	ReceiptReferences ReceiptReferences
	ReceiptDetails    string
	OutboxEventID     id.UUID
	OutboxPayload     ReceiptCreatedPayload
	Now               time.Time
}

// ReconciliationAttemptRow is one reconciliation attempt of the view
// material: the lease facts (the attempt number, the token, the window,
// the backoff moment) joined with the outcome facts when the row exists
// (the verdict word, the correlation, the external object identity).
type ReconciliationAttemptRow struct {
	AttemptNo          int
	LeaseToken         string
	LeaseExpiresAt     time.Time
	NextEligibleAt     time.Time
	LeaseCreatedAt     time.Time
	ReconciliationWord string
	OutcomeVerdict     string // "" when the attempt carries no outcome row
	CorrelationID      string
	ExternalObjectID   string
	OutcomeCreatedAt   time.Time
}

// ---------------------------------------------------------------------------
// The reconciliation repository contract
// ---------------------------------------------------------------------------

// ReconciliationRepository is the persistence contract of the
// reconciliation contour: the pending classification read, the
// write-ahead lease transaction, the fenced verdict transaction and the
// view reads. The store owns NO adapters — every transaction is a pure
// persistence boundary (the signature is the persist-before-emit
// discipline itself; the DispatchRepository precedent of 3-6). The
// receipt reads ride the DispatchRepository (ReadActionReceipts — the
// versions are one chain).
type ReconciliationRepository interface {
	// PendingReconciliationTargets reads the unknown-pending targets of
	// the tenant (the operations with the last receipt unknown, bounded
	// by the reconcile drain limit) with the structural classification,
	// the derived state word and the attempt facts — the ONE reconciler
	// of decisions: every target is considered, not every target is
	// queried.
	PendingReconciliationTargets(ctx context.Context, scope tenancy.Context, now time.Time, limit int) ([]PendingReconciliationTarget, error)
	// AcquireReconciliationLease is the write-ahead transaction of the
	// query right: under the reconciliation advisory lock the monotonic
	// attempt_no derives (1..3), the lease row and the ledger
	// reconciling entry land in ONE commit — the right and the moment
	// of the query are material BEFORE the network call. An active
	// un-expired lease of another consideration refuses with
	// ErrReconciliationLeaseRaced; the attempt bound refuses with
	// ErrReconciliationLeaseStale (the store CHECK is the loyal
	// backstop).
	AcquireReconciliationLease(ctx context.Context, p AcquireReconciliationLeaseParams) (int, error)
	// RecordReconciliationVerdict is the fenced verdict transaction: the
	// (attempt_no, lease_token) pair must own the current attempt — a
	// stale pair NEVER writes history (ErrReconciliationLeaseStale).
	// Inside ONE commit: the outcome row of the normalized verdict (the
	// attempt always records its guarded truth), the ledger entry (ONLY
	// the state-bearing verdicts — committed/no_effect; unknown stays),
	// the receipt v2+ with the outbox receipt.created record (ONLY the
	// terminal verdicts that pass the terminal lock — an append over
	// the last unknown; the same-terminal confirmation skips silently;
	// a conflicting terminal refuses with ErrTerminalConflict). ANY
	// failure rolls back the whole verdict.
	RecordReconciliationVerdict(ctx context.Context, p RecordReconciliationVerdictParams) error
	// ReadReconciliationAttempts reads the reconciliation attempts of
	// one operation ordered by the monotonic attempt number (the view
	// material).
	ReadReconciliationAttempts(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]ReconciliationAttemptRow, error)
	// LastReconciliationNoEffect reads the recorded no_effect verdict
	// flag of one operation (the retry-unlock input of the dispatch
	// pending classification — the PROVEN absence of the external
	// effect replaces the guarantee role; the dispatch store surgical
	// extension of 3-7 step-02 reads it in its own SQL — this read
	// serves the unit fake and the view).
	LastReconciliationNoEffect(ctx context.Context, scope tenancy.Context, operationID id.UUID) (bool, error)
}
