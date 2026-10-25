package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.8: the order correction surface — the Correction Work Item
// over the LAST CLOSED blocked decision of the manager order
// verification (4.7). The four lifecycle commands (open / reassign /
// reread / cease) and the case view read of the correction states. The
// commands carry NO supplier-send field anywhere (the OQ-3 structural
// fence), NO change-action material (AG-4: the ABCP order change is the
// manager's manual contour — Staffy neither changes nor creates) and NO
// correction material — the anchors, the blocked pins and the diff
// derive LIVE by the service.

// OpenOrderCorrectionCommand is the opening ask of the correction: the
// anchor (the case with the committed order operation whose LAST CLOSED
// verification decision is blocked), the owner (validated against the
// LIVE member facts with the review scope) and the deadline word of the
// approved policy dictionary. The blocked positions and the reason word
// travel BY NAME from the blocked decision inside the ONE creation
// transaction — the command NEVER carries them (the SQL-truth carry).
type OpenOrderCorrectionCommand struct {
	Actor       identityDomain.Principal
	TenantID    id.UUID
	CaseID      id.UUID
	OperationID id.UUID
	Owner       id.UUID
	Deadline    string
	Now         time.Time
}

// ReassignOrderCorrectionCommand is the owner's explicit reassignment
// (the overdue fallback — never an automatic takeover, AG-4/AD-14): the
// new round's owner and deadline word.
type ReassignOrderCorrectionCommand struct {
	Actor        identityDomain.Principal
	TenantID     id.UUID
	CaseID       id.UUID
	CorrectionNo int64
	Owner        id.UUID
	Deadline     string
	Now          time.Time
}

// RecordOrderCorrectionRereadCommand is the corrected closure — the
// manager's reread record (AG-4: the reread is recorded by the
// manager's words, Staffy never reads ABCP): the reread external ABCP
// object ID and the pinned identity triple of the NEW confirmed version
// {proposal, version, confirmation} — the proposal version the customer
// confirmed (validated: the version is confirmed, the LIVE
// re-derivation of the order material equals the version digest — the
// hidden degradation is impossible STRUCTURALLY, AC-13; a superseded
// confirmation refuses BY NAME, AC-15).
type RecordOrderCorrectionRereadCommand struct {
	Actor              identityDomain.Principal
	TenantID           id.UUID
	CaseID             id.UUID
	CorrectionNo       int64
	ExpectedAssignment int64
	ExternalObjectID   string
	ProposalID         id.UUID
	VersionNo          int64
	ConfirmationID     id.UUID
	Now                time.Time
}

// CeaseOrderCorrectionCommand is the ceased closure: the structured
// cessation reason word of the three-word dictionary (client_refused /
// not_needed / abandoned). The client refusal NEVER becomes a
// replacement — the replacement is the reread path only.
type CeaseOrderCorrectionCommand struct {
	Actor              identityDomain.Principal
	TenantID           id.UUID
	CaseID             id.UUID
	CorrectionNo       int64
	ExpectedAssignment int64
	Reason             string
	Now                time.Time
}

// OrderCorrectionOutcome is the honest result of every command: the
// correction, the derived state view AFTER the write and the derived
// words of the moment (the boundary word of the opening, the reread
// word of the corrected closure, the cancellation recommendation word
// of the client_refused cessation). The outcome NEVER claims a send or
// an ABCP change.
type OrderCorrectionOutcome struct {
	Correction domain.OrderCorrection
	View       domain.OrderCorrectionStateView
	Word       string
	StateText  string
}

// OrderCorrectionService is the narrow Story 4.8 surface the transport
// type-asserts over the action service (the OrderVerificationService
// precedent): the composite app Service implements it; a composition
// root that binds the action surface WITHOUT the correction contour
// answers the honest 404 of the unwired surface.
type OrderCorrectionService interface {
	// OpenOrderCorrection opens the correction of one committed order
	// operation with its LAST CLOSED blocked verification decision (the
	// honest refusals speak BEFORE any row: the anchor word, the 409 of
	// the open correction, the guard words).
	OpenOrderCorrection(ctx context.Context, command OpenOrderCorrectionCommand) (OrderCorrectionOutcome, error)
	// ReassignOrderCorrection records the owner's explicit reassignment
	// (the new round with the new epoch).
	ReassignOrderCorrection(ctx context.Context, command ReassignOrderCorrectionCommand) (OrderCorrectionOutcome, error)
	// RecordOrderCorrectionReread records the corrected closure — the
	// manager's reread record behind the version validations and the
	// LIVE composition fence (the same re-derivation code the dispatch
	// ladder 4.5 and the 4.7 fence run — reused, never a new step).
	RecordOrderCorrectionReread(ctx context.Context, command RecordOrderCorrectionRereadCommand) (OrderCorrectionOutcome, error)
	// CeaseOrderCorrection records the ceased closure (the structured
	// cessation reason word).
	CeaseOrderCorrection(ctx context.Context, command CeaseOrderCorrectionCommand) (OrderCorrectionOutcome, error)
	// ReadOrderCorrectionCaseView reads the correction states of one
	// case (the panel view material — the CURRENT truth, never a stored
	// status).
	ReadOrderCorrectionCaseView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) ([]domain.OrderCorrectionStateView, error)
}
