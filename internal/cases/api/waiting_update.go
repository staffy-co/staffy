package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.16: the truthful waiting updates — the operator command of the
// append-only update promise and the read-time waiting view of the case
// page
// ---------------------------------------------------------------------------

// PromiseUpdateCommand is the operator command of ONE append-only update
// promise row of the open handoff round: the deadline WORD of the fixed
// dictionary is the ONLY request-carried material — the honest unknown
// word promises the update without a fabricated timestamp (the promised
// time and the promise number are the SERVER truths: the store computes
// now + window under the locks and derives the NEXT monotonic number; a
// request carrying either is refused by the strict decode of the
// transport). The round anchor is NEVER a command field — the store
// resolves the open lifecycle of THIS case under the locks with its own
// mirror words. The promise is the act of ANY operator of the
// review|configuration scope — the duty of communication is never gated
// by the conversation ownership (the honest difference from the 2.14
// producer record, the precedent of the 2.15 correction).
type PromiseUpdateCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	DeadlineWord   string
	IdempotencyKey string
	Now            time.Time
}

// UpdatePromiseService is the narrow command surface of Story 2.16 (the
// CorrectionService/HandoffService precedent: the composite Service is
// type-asserted, the test fakes of the other stories stay untouched).
// The command gate is review or configuration (an audit-only member gets
// the honest 403 of the scope words BEFORE every material guard); the
// waiting view travels through the CaseView read of the case page (no
// new read path exists — the page is ONE resource).
type UpdatePromiseService interface {
	PromiseUpdate(context.Context, PromiseUpdateCommand) (WorkOutcome, error)
}

// WaitingUpdateView is the read-time waiting projection of the case page
// (the CaseView extension of the slice — NO new read path): the honest
// client waiting status of the last handoff round in the THREE
// distinctions of AC-1 (searching / assigned-not-yet-accepted /
// accepted, plus the honest overdue word) and the state of the last
// promise of that round (the word, the promised time — NULL of the
// honest unknown word, the derived state word pending/missed/unknown/
// superseded/closed with the honest texts and the honest missed next
// step). The block appears ONLY with the promise material — a case
// without promises renders the page byte-for-byte unchanged (the control
// branch of the slice). The view carries the CLIENT-facing material
// only: the operator names, the epochs and the identifiers NEVER enter
// the waiting texts (the anti-disclosure of AC-1 — the responsible
// actor lives in the append-only row and the outbox event, never in the
// client projection).
type WaitingUpdateView struct {
	// HasPromise reports whether the round carries at least one promise
	// row — the render material of the whole «Ожидание человека» block
	// (false renders NOTHING: the control branch of the slice).
	HasPromise bool `json:"has_promise"`
	// The client waiting status of the last round (the dictionary word
	// with the honest text — three distinctions plus the honest ending
	// words).
	ClientStatus     string `json:"client_status,omitempty"`
	ClientStatusText string `json:"client_status_text,omitempty"`
	// The last promise of the round (the append-only rows derive
	// read-time — the LAST speaks; the promised time is the ZERO value
	// of the honest unknown word).
	PromiseNo       int64     `json:"promise_no,omitempty"`
	DeadlineWord    string    `json:"deadline_word,omitempty"`
	DeadlineText    string    `json:"deadline_text,omitempty"`
	PromisedAt      time.Time `json:"promised_at,omitempty"`
	State           string    `json:"state,omitempty"`
	StateText       string    `json:"state_text,omitempty"`
	PromiseLineText string    `json:"promise_line_text,omitempty"`
	// The honest next step of the missed promise (the update is the
	// EXTERNAL producer — the human sends it by the existing reply path;
	// the machine never breeds a timer or an automatic send).
	MissedNextStepText string `json:"missed_next_step_text,omitempty"`
	// The dictionaries of the promise form (the fixed compile-time words
	// — never the invented options).
	DeadlineWords []string `json:"deadline_words,omitempty"`
}
