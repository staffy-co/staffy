package api

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/action/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 3.7 (step-03): the reconciliation-contour surface (the DTO layer —
// the transport renders THESE shapes only; the DispatchService precedent
// of 3-6)
// ---------------------------------------------------------------------------
//
// The reconcile is an EXPLICIT operator command (the DrainDispatchQueue
// precedent of 3-6): NO daemon, NO ticker — the scheduler is a future
// infrastructure. The result speaks the honest report in words: every
// considered target leaves a line (the recorded verdict with its fixed
// text, or the honest skip word of the exact fence — FR-30). The
// operation view extension carries the reconciliation attempts of the
// operation card (the lease facts, the recorded verdict word with its
// text, the correlation and the external object identity of the
// committed evidence) in the closed details — the card stays legible at
// 320px (AC-13), the material NEVER enters (SAFE pins and words only).

// ReconcileOperationsCommand is the explicit reconcile command of the
// reconciliation contour: the acting operator principal, the tenant and
// the moment. The batch is bounded by the domain drain limit
// (ReconcileDrainLimit=100) — the rest stay honestly for the next run.
type ReconcileOperationsCommand struct {
	Actor    identityDomain.Principal
	TenantID id.UUID
	Now      time.Time
}

// ReconcileReportLineView is ONE spoken line of the reconcile report: the
// operation, the attempt number of the line and the honest word (every
// boundary names the exact cause and the allowed next step — FR-30).
type ReconcileReportLineView struct {
	OperationID id.UUID `json:"operation_id"`
	AttemptNo   int     `json:"attempt_no"`
	Text        string  `json:"text"`
}

// ReconcileResultView is the honest answer of the reconcile command: how
// many targets were considered, how many were queried (the owning-system
// answer calls), how many landed a recorded verdict and how many skipped
// honestly (fences, races, alarms) — with the spoken line of EVERY target
// (never a masked silence).
type ReconcileResultView struct {
	Command    string                    `json:"command"`
	Considered int                       `json:"considered"`
	Queried    int                       `json:"queried"`
	Decided    int                       `json:"decided"`
	Skipped    int                       `json:"skipped"`
	Reports    []ReconcileReportLineView `json:"reports"`
}

// ReconciliationAttemptView is one reconciliation attempt of the
// operation card (AC-13: the attempts render in CLOSED details): the
// write-ahead lease facts (the right and the moment of the query landed
// BEFORE the network call), the material backoff moment
// (next_eligible_at — the window the next command respects), the honest
// verdict word WHEN recorded (the four-word dictionary; the un-recorded
// attempt speaks the write-ahead resting text by name), the provider
// correlation and the external object identity of the committed evidence.
type ReconciliationAttemptView struct {
	AttemptNo        int       `json:"attempt_no"`
	LeaseCreatedAt   time.Time `json:"lease_created_at"`
	LeaseExpiresAt   time.Time `json:"lease_expires_at"`
	NextEligibleAt   time.Time `json:"next_eligible_at"`
	Verdict          string    `json:"verdict,omitempty"`
	VerdictRecorded  bool      `json:"verdict_recorded"`
	VerdictText      string    `json:"verdict_text"`
	CorrelationID    string    `json:"correlation_id"`
	ExternalObjectID string    `json:"external_object_id"`
	VerdictCreatedAt time.Time `json:"verdict_recorded_at,omitempty"`
}

// ReconcileService is the narrow reconciliation-contour surface the
// transport type-asserts (the DispatchService precedent): the composite
// app Service implements it, the fixture and transport composition roots
// bind it. The GET-side view extension rides the ReadAuthorizationView of
// the authorization surface — the operation card grows the reconciliation
// fields (the attempts, the recorded verdict words) through the SAME
// read; the receipt versions grow with the terminal-locked appends of
// the verdict transactions (v2+ — the SAME receipts list of the dispatch
// view).
type ReconcileService interface {
	// ReconcileOperations runs the EXPLICIT bounded reconcile of the
	// pending unknown targets of the tenant (operator manage — the
	// operations scope word; a non-member is the honest 404 BEFORE the
	// targets are read). The nil read-only adapter of the production
	// root refuses honestly FIRST (AG-4 — the pending targets stay
	// untouched, zero queries); every target of the bounded batch walks
	// the light read-only fences — a boundary speaks its honest line,
	// the query is NEVER made, the lease is NEVER taken (AC-1).
	ReconcileOperations(ctx context.Context, command ReconcileOperationsCommand) (ReconcileResultView, error)
}

// The store-contract errors of the reconciliation contour the transport
// maps (the consumer-owned vocabulary of the repository boundary
// re-exported for the render layer; the DispatchService precedent).
var (
	// ErrReconcileRefusedAG4 is the honest production refusal of the
	// reconcile command at a nil read-only adapter (AG-4: no real owning
	// systems exist — the pending targets honestly stay).
	ErrReconcileRefusedAG4 = port.ErrReconcileRefusedAG4
	// ErrReconciliationLeaseStale is the honest fenced refusal (the
	// stale holder never writes history — the reconcile report speaks
	// the skip).
	ErrReconciliationLeaseStale = port.ErrReconciliationLeaseStale
	// ErrReconciliationLeaseRaced is the honest racing skip (an active
	// lease of another consideration owns the reconciliation right now).
	ErrReconciliationLeaseRaced = port.ErrReconciliationLeaseRaced
	// ErrTerminalConflict is the integrity conflict of the receipt
	// terminal lock — the human review word, never an auto-replacement.
	ErrTerminalConflict = port.ErrTerminalConflict
)
