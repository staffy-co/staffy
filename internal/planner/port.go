package planner

// The ports of the planner machine (Story 8-1): the model port and the
// turn-recorder port. The ModelPort carries ONE operation — Round —
// and NO credentials (the model credentials are the AG-4 boundary of
// Epic 8: the parameterised adapter of Story 8-2 owns them; the port
// form stays credential-free by construction). The adapter returns RAW
// material; the decode and the validation are the MACHINE's
// responsibility, never the adapter's — a future adapter cannot weaken
// the contract by validating on its own side.

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// RoundRequest is the material of one model round: the validated turn
// context and, on the repair round, the named refusal cause of the
// first round (the model learns WHY its material was refused — never
// a silent fix). The same context travels both rounds.
type RoundRequest struct {
	Context TurnContext
	// Repair is empty on round 1 and carries the named cause
	// (rule: detail) on the repair round.
	Repair string
}

// RoundResult is the raw material of the model: bytes the machine
// decodes and validates against the closed contract. The adapter does
// not know the contract — a coherent adapter produces material, the
// machine owns the form.
type RoundResult struct {
	Raw []byte
}

// ModelPort is the single-round port of the model adapter. An error
// from Round is an ADAPTER error: the machine fails the turn honestly
// (a failed record plus the approved fallback) without a blind retry —
// the transport trouble of a model is never a validation cause and
// never a repair trigger.
type ModelPort interface {
	Round(ctx context.Context, request RoundRequest) (RoundResult, error)
}

// TurnRecord is the append-only journal record of one started turn
// (AD-10/AD-11): the ids of the context, the outcome word, the named
// refusal cause (the final cause of a failed turn; the repaired cause
// of a repaired turn), the fallback form that ended a failed turn, the
// proposal schema version, the machine Build constant, the validated
// proposal as compact JSONB (business material behind the tenant RLS —
// the reply-string spirit of Story 2.12, never telemetry) and the
// creation moment. A record is never updated: a new turn is a new
// record, and the uniqueness (tenant, turn_id) makes the second
// recording of the same turn impossible.
type TurnRecord struct {
	TenantID       id.UUID
	TurnID         id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	WorkItemID     *id.UUID
	EmployeeID     id.UUID
	Outcome        string // validated | repaired | failed
	RefusalReason  string
	FallbackForm   string
	SchemaVersion  int
	Build          string
	Proposal       []byte
	CreatedAt      time.Time
}

// TurnRecorder is the journal port of the machine: Record appends one
// turn record behind the tenant RLS. ErrTurnConflict is the honest
// conflict of the uniqueness — the caller learns that this turn was
// already recorded, and no second record exists.
type TurnRecorder interface {
	Record(ctx context.Context, scope tenancy.Context, record TurnRecord) error
}

// Clock is the time port of the machine (the fixed clocks of the tests
// and the wall clock of the composition).
type Clock interface {
	Now() time.Time
}
