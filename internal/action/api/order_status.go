package api

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/action/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Story 4.9: the order status surface — the Supplier Order Fact Receipt
// (the two record commands of the human facts {formed, sent}) and the
// Order Status Projection read (the pure read-time view over the
// material of the three machines plus the facts). The commands carry NO
// supplier-send action anywhere (the OQ-3 structural fence — the record
// IS the human claim of what already happened, never a command to
// perform it), NO adapter material (AG-4) and NO notification material
// (OQ-1/OQ-2 — the panel is the only status surface).

// RecordOrderSupplierFactCommand is the ONE record command of both
// kinds (the app exposes the two named constructors over it — the kind
// word rides the command, the guard refuses the out-of-dictionary
// words): the anchor (the case with the LIVE order operation), the kind
// word {formed, sent}, the supplier order ID (the external ID named by
// the manager — the number of the order at the supplier) and the named
// happened_at time (the manager's word of when the event happened —
// the server recorded_at lands inside the append, the two times never
// mix).
type RecordOrderSupplierFactCommand struct {
	Actor           identityDomain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	OperationID     id.UUID
	Kind            string
	SupplierOrderID string
	HappenedAt      time.Time
	Now             time.Time
}

// OrderSupplierFactOutcome is the honest result of the record command:
// the saved (or replayed) fact, the replay flag (the idempotent repeat
// BY KEY — the SAME fact answers, never a second row) and the honest
// source word (the human receipt of the store contour — AG-4). The
// outcome NEVER claims a send: the fact records the reality, it does
// not perform it.
type OrderSupplierFactOutcome struct {
	Fact       domain.OrderSupplierFact
	Replay     bool
	SourceWord string
}

// OrderStatusService is the narrow Story 4.9 surface the transport
// type-asserts over the action service (the OrderCorrectionService
// precedent): the composite app Service implements it; a composition
// root that binds the action surface WITHOUT the status contour
// answers the honest 404 of the unwired surface.
type OrderStatusService interface {
	// RecordOrderSupplierFormed records the formed fact — the manager's
	// human receipt that the order was formed at the supplier in ABCP
	// (the kind word rides the command; the anchor and the guards run
	// BEFORE the append; the replay of the same key answers the SAVED
	// fact).
	RecordOrderSupplierFormed(ctx context.Context, command RecordOrderSupplierFactCommand) (OrderSupplierFactOutcome, error)
	// RecordOrderSupplierSent records the sent fact — the manager's
	// human receipt that the order was sent to the supplier (the
	// sent-after-formed ladder of the SAME supplier order ID runs BEFORE
	// the append — the discipline of the full history, the times are
	// named as they happened).
	RecordOrderSupplierSent(ctx context.Context, command RecordOrderSupplierFactCommand) (OrderSupplierFactOutcome, error)
	// ReadOrderStatusView reads the Order Status Projection of one case
	// (the panel view material — the PURE read-time derivation over the
	// operation/receipt material, the verification events, the
	// correction events and the supplier facts; ZERO writes — the
	// computed representation never becomes a new source of truth,
	// AC-2).
	ReadOrderStatusView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (domain.OrderStatusProjection, error)
}
