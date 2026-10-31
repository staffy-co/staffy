package planner

import (
	"errors"
	"fmt"
)

// RuleError is the named refusal of the turn contract: every refusal
// carries the rule that fired and the concrete detail of the material
// that broke it. The named cause is what the repair round hands to the
// model (the model learns WHY the material was refused, never a
// silent fix), and what the journal persists on a failed turn — the
// honest, greppable trace of every refusal the machine ever made.
type RuleError struct {
	Rule   string
	Detail string
}

func (e *RuleError) Error() string { return e.Rule + ": " + e.Detail }

// rule builds a named refusal from a format and its arguments.
func rule(name, format string, arguments ...any) *RuleError {
	return &RuleError{Rule: name, Detail: fmt.Sprintf(format, arguments...)}
}

// ErrTurnConflict is the honest conflict of the journal uniqueness: a
// turn (tenant, turn_id) is recorded exactly once — the second
// recording of the same turn does not happen (the action-queue
// idempotency-conflict semantics applied to the turn ledger).
var ErrTurnConflict = errors.New("запись хода уже существует — повторное записывание того же хода невозможно")

// ErrTurnNotFound is the honest absence of a turn record: either the
// turn never happened, or it belongs to another tenant (the RLS view
// of the caller never discloses which one — AD-11).
var ErrTurnNotFound = errors.New("запись хода не найдена")
