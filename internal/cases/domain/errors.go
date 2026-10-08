package domain

import "errors"

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
	// ErrIdempotencyKeyConflict marks a delivery reusing an idempotency
	// key that an accepted event of another provider event id already
	// holds: the honest outcome is a quarantine with an operational
	// exception, never an internal error.
	ErrIdempotencyKeyConflict = errors.New("cases: idempotency key conflict")
)
