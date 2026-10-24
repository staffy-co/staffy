package port

import (
	"context"
	"errors"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// Story 4.7: the manager order verification contour ports. The task lives
// over the COMMITTED operation/receipt pair of create_abcp_customer_order
// (4.6) — the anchor reads ride the EXISTING repositories of the action
// module (the operations, the ledger, the receipts); the lifecycle writes
// own ONE new repository contract. The LIVE material read of the decision
// fence rides the EXISTING CaseMaterialSource port wired since 3-4 — the
// same fresh CaseView the builder re-derivation reads (the consumer-owned
// port reuse of the spec; NO write path into the cases module exists).

// The store-contract errors of the verification contour (the honest
// vocabulary of the repository boundary — the transport maps them).
var (
	// ErrVerificationActive is the honest one-active-task conflict of the
	// repeated assign (the 409: the second competing task of one operation
	// is never created — the HandoffActiveError precedent of 2.13).
	ErrVerificationActive = errors.New("проверка заказа уже активна")
	// ErrVerificationRaced is the honest CAS refusal of the stale epoch or
	// the stale state (the raced append never writes: the app re-reads and
	// speaks the CURRENT state words — the honest «задача переназначена»
	// family).
	ErrVerificationRaced = errors.New("эпоха или состояние задачи проверки изменились")
	// ErrVerificationNotFound is the honest 404 of the task reads (the
	// RLS blindness mirror — a foreign tenant sees nothing).
	ErrVerificationNotFound = errors.New("задача проверки заказа не найдена")
)

// AppendOrderVerificationParams is the ONE creation transaction of the
// contour: the task row (the anchor pins: the committed operation, the
// receipt version, the external ABCP Order ID, the work item, the
// creator) and the OPENING assigned event (the assignee, the backup
// owner, the deadline word with its time, the actor). The store computes
// the monotonic verification_no of the case ladder and the event_no under
// the advisory lock; the signature carries ZERO adapters — the creation is
// a pure persistence boundary (persist-before-emit is STRUCTURAL: nothing
// is emitted anywhere — the OQ-3 fence of the contour).
type AppendOrderVerificationParams struct {
	Scope            tenancy.Context
	CaseID           id.UUID
	WorkItemID       id.UUID
	OperationID      id.UUID
	ReceiptVersionNo int64
	ExternalOrderID  string
	CreatorID        id.UUID
	Assignee         id.UUID
	BackupOwner      id.UUID
	VerifyDeadline   string
	DeadlineAt       time.Time
	Now              time.Time
}

// AppendOrderVerificationEventParams is the ONE lifecycle append of the
// contour: the event with the per-kind material, the CAS expectation
// (the derived state word AND the assignment epoch the command saw) and
// the event identity. The store re-derives the state under the advisory
// lock — a raced expectation refuses with ErrVerificationRaced and NEVER
// writes (the parallel decision of the second manager loses honestly).
type AppendOrderVerificationEventParams struct {
	Scope          tenancy.Context
	CaseID         id.UUID
	VerificationNo int64
	Event          domain.OrderVerificationEvent
	ExpectedState  string
	ExpectedEpoch  int64
	Now            time.Time
}

// OrderVerificationRepository is the persistence contract of the manager
// verification contour: the task reads (the operation anchor view, the
// case view, the event history) and the TWO appends (the creation with
// the opening assigned event; the lifecycle event with the CAS
// re-validation). The store owns NO adapters — every transaction is a
// pure persistence boundary under the per-case advisory lock.
type OrderVerificationRepository interface {
	// ReadOperationVerifications reads every task anchored to one
	// operation ordered by the verification number (the active-task
	// conflict check and the ladder view).
	ReadOperationVerifications(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]domain.OrderVerification, error)
	// ReadCaseVerifications reads every task of one case ordered by the
	// verification number (the view material of the panel).
	ReadCaseVerifications(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]domain.OrderVerification, error)
	// ReadVerificationEvents reads the append-only events of one task
	// ordered by the monotonic event number (the state derivation input).
	ReadVerificationEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID, verificationNo int64) ([]domain.OrderVerificationEvent, error)
	// AppendOrderVerification creates the task row AND the opening
	// assigned event in ONE transaction under the per-case advisory lock:
	// the monotonic verification_no derives under the lock (the ladder of
	// the repeated verification), the one-active-task rule re-validates
	// (the raced twin of the same operation refuses with
	// ErrVerificationActive — the honest 409, never a second task).
	AppendOrderVerification(ctx context.Context, p AppendOrderVerificationParams) (domain.OrderVerification, error)
	// AppendOrderVerificationEvent appends ONE lifecycle event under the
	// per-case advisory lock with the CAS re-validation: the derived state
	// and the assignment epoch must EQUAL the command's expectation — a
	// raced append (the parallel decision, the reassignment, the expiry
	// word move) refuses with ErrVerificationRaced and NEVER writes.
	AppendOrderVerificationEvent(ctx context.Context, p AppendOrderVerificationEventParams) (domain.OrderVerificationEvent, error)
}
