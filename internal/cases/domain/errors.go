package domain

import (
	"errors"
	"fmt"
)

// The honest error surface of the cases module: the transport maps these
// onto the problem contract; nothing internal ever leaks as text.
var (
	// ErrNotFound marks a read of an absent case, event or payload: the
	// tenant existence is never disclosed through it.
	ErrNotFound = errors.New("cases: not found")
	// ErrDigestConflict marks a raced delivery of the same provider event
	// id with another material payload.
	ErrDigestConflict = errors.New("cases: digest conflict")
	// ErrEventRaced marks the defensive backstop of the material insert
	// (the advisory lock already serializes the normal path).
	ErrEventRaced = errors.New("cases: event raced")
	// ErrActiveBinding marks the race loser of the channel binding: the
	// conversation already carries an ACTIVE link — the honest 409 of the
	// concurrent binding attempts (never a plain rule refusal).
	ErrActiveBinding = errors.New("cases: active binding exists")
	// ErrIdempotencyKeyConflict marks a delivery reusing an idempotency
	// key that an accepted event of another provider event id already
	// holds: the honest outcome is a quarantine with an operational
	// exception, never an internal error.
	ErrIdempotencyKeyConflict = errors.New("cases: idempotency key conflict")
	// ErrStaleVersion marks a work command whose expected_version no
	// longer matches the material: the honest optimistic concurrency
	// refusal (the material stays consistent, the command is not
	// silently merged).
	ErrStaleVersion = errors.New("cases: stale version")
	// ErrCommandKeyConflict marks a work command reusing an idempotency
	// key with another request material: the honest conflict (the stored
	// outcome never replays a different command).
	ErrCommandKeyConflict = errors.New("cases: command key conflict")
	// ErrFactMaterialExists marks a raced fact write whose material
	// idempotency tuple was written concurrently: one fresh lookup
	// decides the honest answer (replay of the original version or the
	// cross-case conflict) — never an internal error.
	ErrFactMaterialExists = errors.New("cases: fact material exists")
	// ErrCommitmentDuplicate marks a confirm command of the same subject
	// of the SAME version under another command key: one confirmation per
	// subject per version (the fail-closed duplicate — the honest 409,
	// never a silent merge; the replay of the SAME key returns the stored
	// outcome instead).
	ErrCommitmentDuplicate = errors.New("cases: commitment duplicate")
)

// WorkRuleError is the honest work-command refusal: the command name and
// the safe reason (the ladder, the taxonomy, the provenance or the evidence
// rule that fired). The transport maps it onto a 400 problem — internal
// details never leak as text.
type WorkRuleError struct {
	Rule   string
	Reason string
}

func (e WorkRuleError) Error() string {
	return fmt.Sprintf("work command %s: %s", e.Rule, e.Reason)
}
