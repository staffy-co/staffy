package api

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/action/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Story 4.7: the manager order verification surface — the Delegated Task
// over the committed receipt of create_abcp_customer_order. The five
// lifecycle commands (assign / acknowledge / approve / block / reassign)
// and the case view read of the task states. The commands carry NO
// supplier-send field anywhere (the OQ-3 structural fence) and NO task
// material — the anchors, the receipt pins and the evidence anchors
// derive LIVE by the service.

// AssignOrderVerificationCommand is the owner's assignment ask: the
// anchors (the case, the work item, the committed operation of the order
// action), the assignee and the backup (validated against the LIVE
// member facts with the review scope) and the deadline word of the
// approved policy dictionary.
type AssignOrderVerificationCommand struct {
	Actor       identityDomain.Principal
	TenantID    id.UUID
	CaseID      id.UUID
	WorkItemID  id.UUID
	OperationID id.UUID
	Assignee    id.UUID
	BackupOwner id.UUID
	Deadline    string
	Now         time.Time
}

// AcknowledgeOrderVerificationCommand is the assignee's explicit
// acceptance: the task anchor and the CAS assignment-epoch expectation
// (the event_no of the assigned round the manager saw).
type AcknowledgeOrderVerificationCommand struct {
	Actor              identityDomain.Principal
	TenantID           id.UUID
	CaseID             id.UUID
	VerificationNo     int64
	ExpectedAssignment int64
	Now                time.Time
}

// ApproveOrderVerificationCommand is the «Подтвердить проверку» decision:
// the CAS epoch expectation AND the expected order digest — the 64-hex
// pin of the exact version the manager verified (the decision fence
// re-derives the LIVE composition and compares with the operation's pin).
type ApproveOrderVerificationCommand struct {
	Actor               identityDomain.Principal
	TenantID            id.UUID
	CaseID              id.UUID
	VerificationNo      int64
	ExpectedAssignment  int64
	ExpectedOrderDigest string
	Now                 time.Time
}

// BlockOrderVerificationCommand is the «Нужна коррекция» decision: the
// affected position keys (validated against the composition of the
// OPERATION) and the structured reason word of the five-word dictionary.
type BlockOrderVerificationCommand struct {
	Actor              identityDomain.Principal
	TenantID           id.UUID
	CaseID             id.UUID
	VerificationNo     int64
	ExpectedAssignment int64
	Positions          []string
	Reason             string
	Now                time.Time
}

// ReassignOrderVerificationCommand is the owner's explicit reassignment
// (the overdue fallback — never an automatic takeover): the new round's
// assignee, backup and deadline word.
type ReassignOrderVerificationCommand struct {
	Actor          identityDomain.Principal
	TenantID       id.UUID
	CaseID         id.UUID
	VerificationNo int64
	Assignee       id.UUID
	BackupOwner    id.UUID
	Deadline       string
	Now            time.Time
}

// OrderVerificationOutcome is the honest result of every command: the
// task, the derived state view AFTER the write and the derived words of
// the moment (the boundary word of the assignment, the MANDATORY
// manual-contour word of the approved decision, the correction word of
// the blocked decision). The outcome NEVER claims a delivery.
type OrderVerificationOutcome struct {
	Task      domain.OrderVerification
	View      domain.OrderVerificationStateView
	Word      string
	StateText string
}

// OrderVerificationService is the narrow Story 4.7 surface the transport
// type-asserts over the action service (the OrderProposalService
// precedent): the composite app Service implements it; a composition
// root that binds the action surface WITHOUT the verification contour
// answers the honest 404 of the unwired surface.
type OrderVerificationService interface {
	// AssignOrderVerification creates the verification task of one
	// committed order operation (the honest refusals speak BEFORE any
	// row: the anchor word, the 409 of the active task, the guard words).
	AssignOrderVerification(ctx context.Context, command AssignOrderVerificationCommand) (OrderVerificationOutcome, error)
	// AcknowledgeOrderVerification records the assignee's explicit
	// acceptance (actor=assignee — the server truth; the CAS epoch).
	AcknowledgeOrderVerification(ctx context.Context, command AcknowledgeOrderVerificationCommand) (OrderVerificationOutcome, error)
	// ApproveOrderVerification records the «Подтвердить проверку»
	// decision — the Human Verification Receipt behind the decision fence
	// (the live re-derivation of the dispatch ladder 4.5 reused).
	ApproveOrderVerification(ctx context.Context, command ApproveOrderVerificationCommand) (OrderVerificationOutcome, error)
	// BlockOrderVerification records the «Нужна коррекция» decision (the
	// affected positions with the structured reason word).
	BlockOrderVerification(ctx context.Context, command BlockOrderVerificationCommand) (OrderVerificationOutcome, error)
	// ReassignOrderVerification records the owner's explicit
	// reassignment (the new round with the new epoch).
	ReassignOrderVerification(ctx context.Context, command ReassignOrderVerificationCommand) (OrderVerificationOutcome, error)
	// ReadOrderVerificationCaseView reads the task states of one case
	// (the panel view material — the CURRENT truth, never a stored
	// status).
	ReadOrderVerificationCaseView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) ([]domain.OrderVerificationStateView, error)
	// ReadOrderVerificationLiveMaterial derives the LIVE order
	// commitment material of the case (the SAME derivation the builder
	// and the decision fence run — DeriveOrderCommitmentMaterial over the
	// fresh case view): the screen renders the current composition and
	// the explicit differences against the pinned version. The honest
	// derivation refusals (no selection, a non-proven position, mixed
	// versions) travel BY NAME — the screen speaks them, never a masked
	// empty.
	ReadOrderVerificationLiveMaterial(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (domain.OrderCommitmentMaterial, error)
}
