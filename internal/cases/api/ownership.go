package api

import (
	"context"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.4: the five ownership commands, the reply drain and the
// OwnershipView projection
// ---------------------------------------------------------------------------

// TakeOwnershipCommand takes the ownership of a case in a takeable state:
// the acknowledgement deadline of the assigned transfer expired without a
// decision, or the current owner is unavailable. A healthy ownership is
// never takeable — the honest refusal names the transfer path. The actor
// needs the review scope; the CAS expectation guards the transition.
type TakeOwnershipCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	CaseID         id.UUID
	ExpectedEpoch  int64
	Reason         string
	IdempotencyKey string
	Now            time.Time
}

// RequestTransferCommand creates the assigned handoff ask: the current
// owner (or a configuration member) names the target — an active member
// or the Staffy employee of the case — with the surface the target would
// own and the honest reason. The deadline NEVER comes from the command:
// the service reads the human coverage word of the acting route decision.
type RequestTransferCommand struct {
	Actor           domain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	ExpectedEpoch   int64
	ToOwnerKind     string
	ToOwnerRef      id.UUID
	WritableSurface string
	Reason          string
	IdempotencyKey  string
	Now             time.Time
}

// ResolveTransferCommand decides one request: acknowledge (the target
// takes the ownership atomically — the resolution, the new term and the
// outbox event in one transaction), decline (the target refuses) or
// cancel (the author withdraws). The late acknowledge past the deadline
// gets the honest request_expired refusal — the derivation voided the
// ask.
type ResolveTransferCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	CaseID         id.UUID
	RequestID      id.UUID
	Decision       string
	Reason         string
	IdempotencyKey string
	Now            time.Time
}

// ReassignOwnershipCommand is the supervision path of a configuration
// member: the direct term of the new epoch with the honest reason; the
// pending request is voided by the cancel resolution of the SAME
// transaction.
type ReassignOwnershipCommand struct {
	Actor           domain.Principal
	TenantID        id.UUID
	CaseID          id.UUID
	ExpectedEpoch   int64
	ToOwnerKind     string
	ToOwnerRef      id.UUID
	WritableSurface string
	Reason          string
	IdempotencyKey  string
	Now             time.Time
}

// SendClientReplyCommand is the ONLY client-bound outgoing path of the
// slice: the actor must be the current human owner, the expected epoch
// must match, the target conversation must belong to the case or be
// actively bound into it (the governing case), and the health word must
// not be reassignment_required. The record is the full send snapshot; the
// content never leaves the reply row. Story 2.5 extends the material with
// the reply class (the producer's mark — the machine never classifies the
// content) and the optional Work Item anchor (the mode gate reads the
// effective mode of that item; an absent anchor reads the conversation
// mode). Story 2.8 extends the material with the optional playbook step
// word: the SALES reply against a CURRENT objection of the Work Item MUST
// carry a step of the playbook of the objection type (the first linked
// answer — always acknowledge_concern); without an objection the empty
// word keeps the old path byte-identical. Story 2.11 extends the material
// with the accuracy qualifier word: the SALES reply against an outcome
// with active alternatives carries the honest limitation word of the
// outcome's active set (a subset of the OUTCOME, never the dictionary in
// general); without alternatives the word stays empty.
type SendClientReplyCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	WorkItemID     id.UUID
	ExpectedEpoch  int64
	ReplyClass     string
	PlaybookStep   string
	Content        string
	// The Story 2.9 presentation link: the SALES reply carries the
	// commercial material of the EXACT shown version — the nullable pair
	// (both or neither; the non-sales classes never carry it — the honest
	// 400 of the dictionary).
	CommitmentID        id.UUID
	CommitmentVersionNo int64
	// The Story 2.10 delivery binding: the SALES reply may carry ONE
	// continuation link of the case — the delivered badge of the operator
	// panel (the FR-16 «the main channel shows whether the link was
	// delivered»); the neutral and the protected classes never carry it
	// (the honest 400 of the dictionary); the link must exist and belong
	// to THIS case (the store re-validates under the locks).
	LinkID id.UUID
	// The Story 2.11 accuracy qualifier: the SALES reply against an
	// outcome with active alternatives MUST carry the qualifier word of
	// the outcome's active set (the absence of media never becomes an
	// invented exact result); without alternatives the word stays EMPTY
	// (the field has exactly one meaning — the playbook_step mirror).
	AccuracyQualifier string
	// The Story 2.12 update promise: a reply on a DEGRADED route MUST
	// promise the time of the next update with one word of the approved
	// policy dictionary (no_promise / within_1h / within_4h /
	// within_next_business_day — «время следующего обновления только в
	// пределах утверждённой policy»); on a healthy route the word stays
	// EMPTY (exactly one meaning). The gate is CLASSLESS: a short
	// failure message can be sales, neutral or protected.
	UpdatePromise  string
	IdempotencyKey string
	Now            time.Time
}

// OwnershipView is the ownership section of the case view: the derived
// state (the current owner with the honest name, the pending transfer,
// the health, the backup owner and the deadline words of the coverage),
// the term history, the outgoing replies with their states and the active
// member options of the transfer forms.
type OwnershipView struct {
	State             casesDomain.OwnershipState  `json:"state"`
	OwnerName         string                      `json:"owner_name"`
	PendingTargetName string                      `json:"pending_target_name"`
	Terms             []casesDomain.OwnershipTerm `json:"terms"`
	Replies           []casesPort.ReplyCard       `json:"replies"`
	Members           []casesPort.OwnerMember     `json:"members"`
	// The Story 2.12 delivery states: the derived honest state of every
	// outgoing reply (prepared / unknown / confirmed / failed with the
	// last evidence anchor — the «Доставка» column of the reply panel;
	// dispatched without evidence is the UNKNOWN, never the delivered).
	Deliveries []ReplyDeliveryView `json:"deliveries"`
}

// DrainRepliesResult is the outcome of the idempotent reply drain
// command: the considered batch with the dispatched and stale counts and
// the attention exceptions raised (the material uniqueness keeps the
// repeats void).
type DrainRepliesResult struct {
	Considered int `json:"considered"`
	Dispatched int `json:"dispatched"`
	Stale      int `json:"stale_needs_review"`
}

// OwnershipService is the command surface of Story 2.4 (all five commands
// are idempotent by the tenant-scoped key; every transition carries the
// expected epoch — the CAS discipline; the reply gate is the server-side
// truth).
type OwnershipService interface {
	TakeOwnership(context.Context, TakeOwnershipCommand) (WorkOutcome, error)
	RequestTransfer(context.Context, RequestTransferCommand) (WorkOutcome, error)
	ResolveTransfer(context.Context, ResolveTransferCommand) (WorkOutcome, error)
	ReassignOwnership(context.Context, ReassignOwnershipCommand) (WorkOutcome, error)
	SendClientReply(context.Context, SendClientReplyCommand) (WorkOutcome, error)
	DrainClientReplies(context.Context, domain.Principal, id.UUID) (DrainRepliesResult, error)
}
