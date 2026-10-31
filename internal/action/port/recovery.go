package port

import (
	"context"
	"errors"
	"time"

	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.8 (step-02): the consumer-owned ports of the recovery-case
// contour — the recovery repository and the honest refusal dictionary
// ---------------------------------------------------------------------------
//
// The recovery contour is LOCAL (the slice honesty core): not ONE
// provider adapter rides the command path — the creation command reads
// the RECORDED facts (this repository), the LIVE ownership/coverage
// words (the CaseMaterialSource/RouteFactsSource ports wired since 3-4 —
// in-process module reads, never the provider world) and writes ONLY its
// own append-only tables. The repository owns the ONE creation
// transaction (CreateRecoveryCase — the advisory lock resolves the
// creation race by the idempotent return, the store re-reads the
// existing case UNDER the lock: the belt over braces of the app
// pre-check) and the read-time fact reads. A network call is impossible
// BY TYPE inside the store — the signature itself is the discipline (the
// AcquireDispatchLease/ReserveOperation precedents).
//
// The events of the outbox accumulate honestly pending: the third word
// recovery_case.created rides the SAME action outbox (the CHECK grew
// 2→3 in 00005 — the receipt.created precedent of 3-6; the claim-mode
// consumer of a future slice reads the records, nobody consumes them
// today).

// The store-contract errors of the recovery contour (the consumer-owned
// refusal vocabulary of the repository boundary; the transport maps them
// in step-03 — the honest WORD rides the wrapped error text).
var (
	// ErrRecoveryNotFound is the honest 404 of the unknown case or the
	// unknown operation of the command (the membership blindness — the
	// foreign tenant reads NOTHING, never a disclosure).
	ErrRecoveryNotFound = errors.New("recovery case not found")
	// ErrRecoveryIneligible is the honest refusal word of the eligibility
	// ladder (the wrapped text names the exact gas — committed, in
	// flight, the reconciliation not exhausted, the repeat available).
	ErrRecoveryIneligible = errors.New("operation not eligible for recovery")
	// ErrRecoveryCoverageUnavailable is the fail-closed assignment word:
	// the human coverage of the original conversation route is unreadable
	// (or carries no deadline word) — the deadline and the backup are
	// never invented, the case is NOT created.
	ErrRecoveryCoverageUnavailable = errors.New("recovery coverage unavailable")
	// ErrRecoveryOwnerUnknown is the fail-closed assignment word: the
	// original Case carries no current ownership term (the pre-migration
	// honest state) — the owner of the recovery is never invented, the
	// case is NOT created.
	ErrRecoveryOwnerUnknown = errors.New("recovery owner unknown")
)

// The creation decision words (three): the outcome vocabulary of the ONE
// creation transaction — created (the new case: the row, the created
// event and the outbox record landed), idempotent (the repeat with the
// UNCHANGED bundle digest: the existing case returns, NO second row, NO
// event — the UNIQUE structural twin), evidence (the repeat with a
// CHANGED digest — the later terminal evidence of the 3-7 contour: ONE
// append-only evidence_appended row with the SAFE delta, the original
// bundle and the original receipts are never rewritten).
const (
	RecoveryDecisionCreated    = "created"
	RecoveryDecisionIdempotent = "idempotent"
	RecoveryDecisionEvidence   = "evidence"
)

// RecoveryDecisionWords is the full dictionary (3 words).
var RecoveryDecisionWords = []string{
	RecoveryDecisionCreated, RecoveryDecisionIdempotent, RecoveryDecisionEvidence,
}

// ---------------------------------------------------------------------------
// The row shapes of the recovery repository
// ---------------------------------------------------------------------------

// RecoveryOperationFacts are the RECORDED facts of ONE operation — the
// read-time inputs of the eligibility ladder and the SAFE bundle build:
// the immutable Operation row, the append-only ledger with the derived
// state word, the attempt counters of both contours (the lease rows),
// the retry inputs of the last dispatch attempt (the recorded guarantee
// flag), the no_effect unlock fact (the last reconciliation verdict),
// the receipt-chain pins (the version count, the latest version with its
// outcome and the external identity triple) and the
// supporting-observations pins of the EXACT preview version (the 2-7
// version pins — the material NEVER travels, AD-10). NOTHING here is a
// command parameter or a live network answer — the eligibility is a
// pure function over the recorded state (the app composes the
// declaredness halves and the wired fact from the definition catalog and
// the composition root).
type RecoveryOperationFacts struct {
	Operation                   domain.Operation
	Ledger                      []domain.OperationLedgerEntry
	StateWord                   string
	DispatchAttemptCount        int
	LastOutcomeGuarantee        bool
	NoEffectProven              bool
	ReconcileAttemptCount       int
	ReceiptVersionsCount        int64
	LatestReceiptVersionNo      int64  // 0 — no receipt exists
	LatestReceiptOutcome        string // "" — no receipt exists
	LatestExternalObjectID      string
	LatestExternalObjectVersion string
	LatestExternalObjectTime    time.Time // zero — the honest unknown
	Observations                []domain.VersionPin
}

// RecoveryCaseRow is ONE Recovery Case with its append-only event
// history: the SAFE link-bundle of the frozen creation moment, the
// assignment of the LIVE readings (the owner triple, the backup owner
// and the materialized deadline), the keyed recovery key, the frozen
// bundle digest (the repeat comparator) and the events. The LIVING
// receipt chain and the LIVING attempt facts of the card derive at the
// read from the dispatch/reconciliation tables (AC-10: the context
// updates by the references — the card is always fresh, the frozen
// bundle never rewrites).
type RecoveryCaseRow struct {
	RecoveryCaseID id.UUID
	RecoveryCaseNo int64
	RecoveryKey    string
	OperationID    id.UUID
	Trigger        string
	Bundle         domain.RecoveryBundle
	BundleDigest   string
	Assignment     domain.RecoveryAssignment
	DeadlineAt     time.Time
	CreatedBy      id.UUID
	CreatedAt      time.Time
	Events         []domain.RecoveryEventRow
}

// CreateRecoveryCaseParams is the ONE creation transaction of the
// contour: the immutable Operation, the derived trigger word, the keyed
// recovery key, the fresh SAFE bundle with its digest (the comparator
// input), the assignment of the LIVE readings with the materialized
// deadline, the pre-minted identities of the case row and the outbox
// record (the events carry NO uuid — the monotonic event number IS the
// identity, the append-only history is keyed by it), the actor and the
// moment. The signature carries ZERO provider adapters — a network call
// is impossible BY TYPE (the Enqueue/ReserveOperation precedents). The
// store re-reads the existing case UNDER the advisory lock: a raced twin
// resolves by the idempotent/evidence decision, the loser's assignment
// derivation is simply not needed (nothing is lost — the winner's row
// returns).
type CreateRecoveryCaseParams struct {
	Scope          tenancy.Context
	Operation      domain.Operation
	Trigger        string
	RecoveryKey    string
	Bundle         domain.RecoveryBundle
	BundleDigest   string
	Assignment     domain.RecoveryAssignment
	DeadlineAt     time.Time
	RecoveryCaseID id.UUID
	OutboxEventID  id.UUID
	ActorID        id.UUID
	Now            time.Time
}

// CreateRecoveryCaseResult is the outcome of the ONE creation
// transaction: the case row (the existing one on the idempotent/evidence
// paths — with the fresh event history) and the decision word of the
// dictionary above.
type CreateRecoveryCaseResult struct {
	Case     RecoveryCaseRow
	Decision string
}

// RecoveryCaseCreatedPayload is the SAFE canonical outbox payload of the
// recovery_case.created record (the ReceiptCreatedPayload precedent):
// identifiers, the trigger word, the assignment pins and the windows —
// the material NEVER enters (AD-10), the future consumer reads the
// pending records (the claim-mode contour of a future slice).
type RecoveryCaseCreatedPayload struct {
	EventType      string    `json:"event_type"`
	TenantID       id.UUID   `json:"tenant_id"`
	RecoveryCaseID id.UUID   `json:"recovery_case_id"`
	RecoveryCaseNo int64     `json:"recovery_case_no"`
	OperationID    id.UUID   `json:"operation_id"`
	CaseID         id.UUID   `json:"case_id"`
	WorkItemID     id.UUID   `json:"work_item_id"`
	DefinitionID   string    `json:"definition_id"`
	Trigger        string    `json:"trigger"`
	OwnerKind      string    `json:"owner_kind"`
	OwnerRef       id.UUID   `json:"owner_ref"`
	BackupOwnerID  id.UUID   `json:"backup_owner_id"`
	DeadlineAt     time.Time `json:"deadline_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// RecoveryRepository is the persistence contract of the recovery-case
// contour: the recorded fact read (the eligibility ladder and the bundle
// build input), the case reads (by the operation pin and by the case
// identity — the card and the idempotent pre-check), the tenant list
// (the panel, bounded) and the ONE creation transaction. The store owns
// NO provider adapters — every transaction is a pure persistence
// boundary (the persist-before-emit discipline: no network call can
// precede a commit because no adapter exists to call).
type RecoveryRepository interface {
	// ReadRecoveryOperationFacts reads the RECORDED facts of one
	// operation (read-time, NO writes, NO locks) — the eligibility ladder
	// and the bundle build input. An unknown operation (or a foreign
	// tenant — the RLS blindness) answers ErrRecoveryNotFound.
	ReadRecoveryOperationFacts(ctx context.Context, scope tenancy.Context, operationID id.UUID) (RecoveryOperationFacts, error)
	// ReadRecoveryCaseByOperation reads the case of one operation with
	// its event history (the idempotent pre-check of the command; the
	// creation transaction re-reads UNDER the lock — the belt over
	// braces). No case answers ErrRecoveryNotFound.
	ReadRecoveryCaseByOperation(ctx context.Context, scope tenancy.Context, operationID id.UUID) (RecoveryCaseRow, error)
	// ReadRecoveryCase reads one case by its identity with its event
	// history (the card view material). Unknown answers
	// ErrRecoveryNotFound.
	ReadRecoveryCase(ctx context.Context, scope tenancy.Context, recoveryCaseID id.UUID) (RecoveryCaseRow, error)
	// ReadRecoveryCases reads the cases of the tenant ordered by the
	// creation moment descending, bounded by the limit (the panel list;
	// limit < 1 reads as the domain bound).
	ReadRecoveryCases(ctx context.Context, scope tenancy.Context, limit int) ([]RecoveryCaseRow, error)
	// CreateRecoveryCase is the ONE creation transaction under the
	// recovery advisory lock (tenant, operation_id): the existing case
	// re-read decides the path — the UNCHANGED digest answers the
	// idempotent return (NO second row, NO event), a CHANGED digest
	// appends ONE evidence_appended row with the SAFE delta (the original
	// bundle is never rewritten), no case creates the row with the
	// monotonic recovery_case_no (coalesce(max)+1 under the lock), the
	// created event and the outbox recovery_case.created record — ONE
	// commit, ANY failure rolls back everything.
	CreateRecoveryCase(ctx context.Context, p CreateRecoveryCaseParams) (CreateRecoveryCaseResult, error)
}

// RecoveryPanelLimit is the bounded panel list read (the drain-limit
// precedents of 3-3/3-6/3-7: one read considers at most this many cases
// — the older history stays honestly for the next page).
const RecoveryPanelLimit = 200
