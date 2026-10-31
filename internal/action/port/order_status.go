package port

import (
	"context"
	"errors"
	"time"

	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Story 4.9: the supplier order fact ports. The facts are the HUMAN
// receipts of the store contour {formed, sent} recorded by a manager
// over the LIVE order operation of the case; the projection reads the
// EXISTING repositories of the action module (the operations, the
// receipts, the verifications, the corrections) and owns ONE new
// repository contract of the fact reads and the ONE idempotent append.
// NO adapter, NO webhook receiver, NO polling — the write path is the
// manager's record command only (AG-4/OQ-3).

// The store-contract errors of the fact contour (the honest vocabulary
// of the repository boundary — the transport maps them).
var (
	// ErrOrderSupplierFactDuplicate is the honest replay signal of the
	// idempotency BY KEY: the fact of the SAME full key (the
	// operation, the kind, the supplier order ID) already exists — the
	// repeated command replays the SAVED fact, never a second row
	// (AC-9: the «webhook repeat» discipline without a webhook).
	ErrOrderSupplierFactDuplicate = errors.New("факт заказа поставщику уже записан (идемпотентный повтор)")
)

// AppendOrderSupplierFactParams is the ONE append of the contour: the
// fact row with the anchor pins (the case, the work item, the LIVE
// operation), the kind word, the supplier order ID, the named
// happened_at time and the actor identity. The store computes the
// monotonic fact_no of the case ladder under the advisory lock; the
// recorded_at is the server time of the append (the two times never
// mix — the provider-observed precedent of 2.7). The signature carries
// ZERO adapters — the append is a pure persistence boundary
// (persist-before-emit is STRUCTURAL: nothing is emitted anywhere —
// the OQ-3 fence of the contour).
type AppendOrderSupplierFactParams struct {
	Scope           tenancy.Context
	CaseID          id.UUID
	WorkItemID      id.UUID
	OperationID     id.UUID
	Kind            string
	SupplierOrderID string
	HappenedAt      time.Time
	ActorID         id.UUID
	Now             time.Time
}

// OrderSupplierFactRepository is the persistence contract of the
// supplier order facts: the case reads (the projection material, the
// operation anchor view) and the ONE idempotent append. The store owns
// NO adapters — every transaction is a pure persistence boundary under
// the per-case advisory lock; the UNIQUE full fact key backstops the
// idempotency the lock already serializes.
type OrderSupplierFactRepository interface {
	// ReadCaseSupplierFacts reads every fact of one case ordered by the
	// monotonic fact number (the projection material of the panel).
	ReadCaseSupplierFacts(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]domain.OrderSupplierFact, error)
	// ReadOperationSupplierFacts reads every fact anchored to one
	// operation ordered by the monotonic fact number (the guard input
	// of the sent-after-formed ladder).
	ReadOperationSupplierFacts(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]domain.OrderSupplierFact, error)
	// AppendOrderSupplierFact appends ONE fact under the per-case
	// advisory lock: the monotonic fact_no derives under the lock (the
	// ladder of the repeated records); the UNIQUE full fact key
	// (tenant, case, operation, kind, supplier_order_id) backstops the
	// IDEMPOTENCY BY KEY — the raced replay of the same command refuses
	// with ErrOrderSupplierFactDuplicate and NEVER writes a second row
	// (the app re-reads the saved fact and answers the honest replay
	// outcome).
	AppendOrderSupplierFact(ctx context.Context, p AppendOrderSupplierFactParams) (domain.OrderSupplierFact, error)
}
