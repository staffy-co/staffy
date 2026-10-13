package api

import (
	"context"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.8: the objection record command and the ObjectionView projection
// ---------------------------------------------------------------------------

// RecordObjectionCommand records one client objection of a Work Item: the
// operator with the review OR the configuration scope (the gate fires
// BEFORE the material guards — the M1 precedent of 2.5/2.7), the case and
// the Work Item anchors, the turn source (an ACCEPTED TEXT event of the
// case — the span carrier), the dictionary words (the type, the signal),
// the verbatim spans (the byte-exact excerpts of the sealed payload of the
// turn — an invented excerpt refuses honestly) and the idempotency key.
// The record never changes the mode: the raise stays the propose_mode
// command of 2.5 — the record only marks the trigger signal.
type RecordObjectionCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	WorkItemID     id.UUID
	TurnRef        id.UUID
	ObjectionType  string
	ClientSignal   string
	Spans          []string
	IdempotencyKey string
	Now            time.Time
}

// ObjectionCard is one objection record card of the view: the record words
// (the dictionary type and signal, the spans — the verbatim client words
// already stored in the row, the turn reference, the playbook version
// snapshot, the time) with the read-time derivations of the Work Item
// state (the lifetime refusal count and the acknowledged word).
type ObjectionCard struct {
	Record *casesDomain.ObjectionRecord `json:"record"`
	// Refusals is the lifetime refusal budget word of the Work Item at
	// the view moment (the state derivation — never stored).
	Refusals int `json:"refusals"`
	// Acknowledged reports whether a linked reply acknowledged THIS
	// objection (the derivation of the outgoing replies).
	Acknowledged bool `json:"acknowledged"`
}

// ObjectionWorkItemView is one Work Item section of the objection view:
// the record history (the CURRENT objection is the maximal number) with
// the derived persuasion state words of the item.
type ObjectionWorkItemView struct {
	WorkItemID     id.UUID                 `json:"work_item_id"`
	ObjectionCount int                     `json:"objection_count"`
	Current        *ObjectionCard          `json:"current"`
	Refusals       int                     `json:"refusals"`
	Threshold      int                     `json:"threshold"`
	Stopped        bool                    `json:"stopped"`
	Acknowledged   bool                    `json:"acknowledged"`
	NextSteps      []string                `json:"next_steps"`
	LinkedReplies  []casesDomain.ReplyLink `json:"linked_replies"`
}

// ObjectionView is the objection section of the case view: the per-Work-
// Item sections with the CURRENT objection (the record history stays
// readable — the append-only audit), the persuasion budget (the lifetime
// refusal count over the fixed threshold), the stopped word with the
// allowed endings, the next-step words of the current playbook and the
// step dictionary of the reply form.
type ObjectionView struct {
	Items           []ObjectionWorkItemView `json:"items"`
	TypeWords       []string                `json:"type_words"`
	SignalWords     []string                `json:"signal_words"`
	StepWords       []string                `json:"step_words"`
	PlaybookVersion string                  `json:"playbook_version"`
	Threshold       int                     `json:"threshold"`
	PanelText       string                  `json:"panel_text"`
}

// ObjectionService is the command surface of Story 2.8 (the record command
// is idempotent by the tenant-scoped key; the objections are append-only
// with the per-Work-Item monotonic number; the persuasion state is the
// read-time derivation — never stored).
type ObjectionService interface {
	RecordObjection(context.Context, RecordObjectionCommand) (WorkOutcome, error)
}
