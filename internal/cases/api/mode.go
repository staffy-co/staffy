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
// Story 2.5: the four mode commands and the ModeView projection
// ---------------------------------------------------------------------------

// ProposeModeCommand raises the mode: the operator (the proposer port of
// this slice — the future autonomous surfaces pass through the same
// command) fixes the signal with the mode word, the scope (the
// conversation or one Work Item), the honest reason, the signal source
// and — for a turn signal — the accepted event it answers. The proposal
// can only RAISE: the deterministic policy refuses the downgrade and the
// equal repeat; the expected mode version is the CAS word of the
// conversation ladder.
type ProposeModeCommand struct {
	Actor               domain.Principal
	TenantID            id.UUID
	CaseID              id.UUID
	ConversationID      id.UUID
	Mode                string
	Scope               string
	WorkItemID          id.UUID
	ExpectedModeVersion int64
	Reason              string
	SignalSource        string
	TurnRef             id.UUID
	IdempotencyKey      string
	Now                 time.Time
}

// ExitModeCommand is the ONLY lowering path of the conversation mode: the
// supervised exit with the honest reason. The authority ladder is the
// server-side word — the current owner with review or the configuration
// supervision; the safety mode is exit-able by the supervision ONLY (the
// platform safety is never weakened by an operator override).
type ExitModeCommand struct {
	Actor               domain.Principal
	TenantID            id.UUID
	CaseID              id.UUID
	ConversationID      id.UUID
	Mode                string
	ExpectedModeVersion int64
	Reason              string
	IdempotencyKey      string
	Now                 time.Time
}

// RecordDisclosureCommand writes one append-only AI disclosure record:
// the exact bounded text, the channel and — for the re-disclosure — the
// parent reference of the voided record. The record is born with its
// generated delivery event; the send observation (the command of the
// operator or the future media adapter) makes it act for the channel.
type RecordDisclosureCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	Channel        string
	Text           string
	ParentRef      id.UUID
	IdempotencyKey string
	Now            time.Time
}

// RecordDisclosureEventCommand appends one delivery observation of a
// disclosure record: sent, a playback word, the customer confirmation or
// the interruption that voids the acting disclosure. One event — one
// kind; the last event is the record's status.
type RecordDisclosureEventCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	DisclosureID   id.UUID
	Kind           string
	IdempotencyKey string
	Now            time.Time
}

// ModeItemContextView is one Work Item context of the mode view: the
// derived context term of the item (nil context is absent from the list).
type ModeItemContextView struct {
	WorkItemID id.UUID              `json:"work_item_id"`
	Term       casesDomain.ModeTerm `json:"term"`
}

// ModeView is the mode section of the case view: the ladder, the active
// conversation mode with its honest words (the reason, the signal source,
// the rule version), the term history, the Work Item contexts, the
// forbidden action words of the current state and the disclosure records
// with their delivery statuses.
type ModeView struct {
	Active        *casesDomain.ModeTerm      `json:"active"`
	Ladder        []string                   `json:"ladder"`
	Terms         []casesDomain.ModeTerm     `json:"terms"`
	ItemContexts  []ModeItemContextView      `json:"item_contexts"`
	ForbiddenText string                     `json:"forbidden_text"`
	NeutralRule   string                     `json:"neutral_rule"`
	Disclosures   []casesPort.DisclosureCard `json:"disclosures"`
}

// ModeService is the command surface of Story 2.5 (all four commands are
// idempotent by the tenant-scoped key; the proposal raises only; the exit
// is the supervised lowering path; the gates are the server-side truth).
type ModeService interface {
	ProposeMode(context.Context, ProposeModeCommand) (WorkOutcome, error)
	ExitMode(context.Context, ExitModeCommand) (WorkOutcome, error)
	RecordDisclosure(context.Context, RecordDisclosureCommand) (WorkOutcome, error)
	RecordDisclosureEvent(context.Context, RecordDisclosureEventCommand) (WorkOutcome, error)
}
