package port

import (
	"context"
	"errors"
	"time"

	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Story 4.8: the order correction contour ports. The correction lives
// over the COMMITTED operation of create_abcp_customer_order WITH the
// LAST CLOSED blocked verification decision of that operation (4.7) —
// the anchor reads ride the EXISTING repositories of the action module
// (the operations, the ledger, the receipts, the verifications); the
// lifecycle writes own ONE new repository contract. The LIVE material
// read of the reread fence rides the EXISTING CaseMaterialSource port
// wired since 3-4 — the same fresh CaseView the builder re-derivation
// and the 4.7 decision fence read (the consumer-owned port reuse of the
// spec; NO write path into the cases module exists).

// The store-contract errors of the correction contour (the honest
// vocabulary of the repository boundary — the transport maps them).
var (
	// ErrCorrectionActive is the honest one-open-correction conflict of
	// the repeated open (the 409: the second competing correction of one
	// operation is never created — the ErrVerificationActive precedent
	// of 4.7).
	ErrCorrectionActive = errors.New("коррекция заказа уже открыта")
	// ErrCorrectionRaced is the honest CAS refusal of the stale epoch or
	// the stale state (the raced append never writes: the app re-reads
	// and speaks the CURRENT state words — the honest «коррекция
	// переназначена» family).
	ErrCorrectionRaced = errors.New("эпоха или состояние коррекции изменились")
	// ErrCorrectionNotFound is the honest 404 of the correction reads
	// (the RLS blindness mirror — a foreign tenant sees nothing).
	ErrCorrectionNotFound = errors.New("коррекция заказа не найдена")
)

// AppendOrderCorrectionParams is the ONE creation transaction of the
// contour: the correction row (the anchor pins: the committed operation,
// the blocked verification number, the receipt identity pins, the work
// item, the creator) and the OPENING opened event (the owner, the
// deadline word with its time, the blocked positions with the reason
// word carried BY NAME — the one-transaction SQL-truth carry, the
// original blocked event is READ never rewritten). The store computes
// the monotonic correction_no of the case ladder and the event_no under
// the advisory lock; the signature carries ZERO adapters — the creation
// is a pure persistence boundary (persist-before-emit is STRUCTURAL:
// nothing is emitted anywhere — the OQ-3 fence of the contour).
type AppendOrderCorrectionParams struct {
	Scope            tenancy.Context
	CaseID           id.UUID
	WorkItemID       id.UUID
	OperationID      id.UUID
	VerificationNo   int64
	ReceiptVersionNo int64
	ExternalOrderID  string
	CreatorID        id.UUID
	Owner            id.UUID
	CorrectDeadline  string
	DeadlineAt       time.Time
	Positions        []string
	Reason           string
	Now              time.Time
}

// AppendOrderCorrectionEventParams is the ONE lifecycle append of the
// contour: the event with the per-kind material, the CAS expectation
// (the derived state word AND the assignment epoch the command saw) and
// the event identity. The store re-derives the state under the advisory
// lock — a raced expectation refuses with ErrCorrectionRaced and NEVER
// writes (the parallel closure of the second manager loses honestly).
type AppendOrderCorrectionEventParams struct {
	Scope         tenancy.Context
	CaseID        id.UUID
	CorrectionNo  int64
	Event         domain.OrderCorrectionEvent
	ExpectedState string
	ExpectedEpoch int64
	Now           time.Time
}

// OrderCorrectionRepository is the persistence contract of the order
// correction contour: the correction reads (the operation anchor view,
// the case view, the event history) and the TWO appends (the creation
// with the opening opened event; the lifecycle event with the CAS
// re-validation). The store owns NO adapters — every transaction is a
// pure persistence boundary under the per-case advisory lock.
type OrderCorrectionRepository interface {
	// ReadOperationCorrections reads every correction anchored to one
	// operation ordered by the correction number (the open-correction
	// conflict check and the ladder view).
	ReadOperationCorrections(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]domain.OrderCorrection, error)
	// ReadCaseCorrections reads every correction of one case ordered by
	// the correction number (the view material of the panel).
	ReadCaseCorrections(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]domain.OrderCorrection, error)
	// ReadCorrectionEvents reads the append-only events of one
	// correction ordered by the monotonic event number (the state
	// derivation input).
	ReadCorrectionEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID, correctionNo int64) ([]domain.OrderCorrectionEvent, error)
	// AppendOrderCorrection creates the correction row AND the opening
	// opened event in ONE transaction under the per-case advisory lock:
	// the monotonic correction_no derives under the lock (the ladder of
	// the repeated correction), the one-open-correction rule
	// re-validates (the raced twin of the same operation refuses with
	// ErrCorrectionActive — the honest 409, never a second correction).
	AppendOrderCorrection(ctx context.Context, p AppendOrderCorrectionParams) (domain.OrderCorrection, error)
	// AppendOrderCorrectionEvent appends ONE lifecycle event under the
	// per-case advisory lock with the CAS re-validation: the derived
	// state and the assignment epoch must EQUAL the command's
	// expectation — a raced append (the parallel closure, the
	// reassignment, the expiry word move) refuses with ErrCorrectionRaced
	// and NEVER writes.
	AppendOrderCorrectionEvent(ctx context.Context, p AppendOrderCorrectionEventParams) (domain.OrderCorrectionEvent, error)
}
