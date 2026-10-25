package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.10: the order servicing surface — the ONE record command of
// the repeat-contact request (the operator with the review scope
// records WHAT THE CLIENT ASKED with the LIVE proof pins of the contact
// case assurance) and the Order Servicing View read (the panel feed
// with the LIVE derivations of the client words and the cancel
// feasibility). The command carries NO cancel execution, NO change
// execution, NO supplier-send action anywhere (the OQ-3 fence, the
// AG-4 manual contour — the request IS the recorded ask, never a
// command to perform it), NO lookup adapter of the external systems
// (AG-4/OQ-18 — the lookup reads the recorded material of the contour
// only) and NO notification material (OQ-1/OQ-2 — the panel is the
// only surface, the operator retells the client words by hand).

// RecordOrderServicingRequestCommand is the ONE record command of the
// slice: the anchor (the LIVE contact case with the optional
// conversation anchor), the intent word (the 8-word dictionary), the
// TYPED identifier the client named (the kind word of the 2-word
// dictionary with the value) and the server Now. The LIVE assurance
// read of the contact case rides INSIDE the command execution (the
// proof pins copy at the moment of the record — AC-4); the lookup BY
// KEY runs in the SAME transaction boundary of the append (the outcome
// is the recorded pin of the request, never a re-derivation).
type RecordOrderServicingRequestCommand struct {
	Actor           identityDomain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	Intent          string
	IdentifierKind  string
	IdentifierValue string
	Now             time.Time
}

// OrderServicingOutcome is the honest result of the record command:
// the saved (or replayed) request, the replay flag (the idempotent
// repeat BY KEY at the SAME trust state — the SAVED request answers,
// never a second row; the changed trust state is a NEW request) and
// the honest source word of the record (the append-only recorded
// event — the request fixes the ask, it never claims the result).
type OrderServicingOutcome struct {
	Request    domain.OrderServicingRequest
	Replay     bool
	SourceWord string
}

// OrderServicingService is the narrow Story 4.10 surface the transport
// type-asserts over the action service (the OrderStatusService
// precedent): the composite app Service implements it; a composition
// root that binds the action surface WITHOUT the servicing contour
// answers the honest 404 of the unwired surface.
type OrderServicingService interface {
	// RecordOrderServicingRequest records ONE repeat-contact request:
	// the rights gate (review) runs FIRST, the LIVE case anchor refuses
	// the honest 404, the LIVE assurance read supplies the proof pins
	// (a refused read refuses the command — never a masked «ok» pin),
	// the lookup BY KEY derives the outcome over the RECORDED material
	// of the tenant, the domain guard refuses the out-of-dictionary
	// words (the phone/email kinds do NOT exist — OQ-16) and the replay
	// check BY KEY answers the SAVED request of the repeated command at
	// the same trust state (the changed trust state — a NEW request:
	// the ladder stays visible).
	RecordOrderServicingRequest(ctx context.Context, command RecordOrderServicingRequestCommand) (OrderServicingOutcome, error)
	// ReadOrderServicingView reads the Order Servicing View of one case
	// (the panel feed of the recorded requests with the LIVE
	// derivations — the client words re-derive over the live projection
	// of the resolved operation, the cancel feasibility re-derives over
	// the live supplier facts; ZERO writes — the computed words never
	// become a stored status).
	ReadOrderServicingView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (domain.OrderServicingView, error)
}
