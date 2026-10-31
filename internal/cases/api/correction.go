package api

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.15: the fact correction surface — the operator command of the
// append-only correction record
// ---------------------------------------------------------------------------

// RecordCorrectionCommand is the operator command of ONE append-only
// correction row: the pinned target (the fact with its EXACT version — the
// optimistic concurrency of the slice: a second correction of the same
// version is a second row and a second fact version, never a silent
// rewrite), the summary link (0 = the event evidence; >=1 = the EXACT
// summary version of THIS case — the store guards the belonging), the
// proposed value (the raw JSON bytes — the domain value guard
// canonicalizes under the fact material discipline of 2.2), the verbatim
// evidence spans and the mandatory reason. The OUTCOME is NEVER a command
// field: the domain matrix gate decides it by the material key of the
// anchor (a matrix field disputes, every other key pours) — the strict
// decode of the transport refuses the field. The correction number is the
// SERVER-side truth: the store derives the NEXT monotonic number under
// the advisory locks. The correction is the act of ANY operator of the
// review|configuration scope — the duty of correctness is never gated by
// the conversation ownership (the honest difference from the 2.14
// producer record of the owner-Staffy).
type RecordCorrectionCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	FactID         id.UUID
	VersionNo      int64
	SummaryNo      int64
	Value          []byte
	Spans          []string
	Reason         string
	IdempotencyKey string
	Now            time.Time
}

// CorrectionService is the narrow command surface of Story 2.15 (the
// HandoffService/DecisionContextService precedent: the composite Service
// is type-asserted, the test fakes of the other stories stay untouched).
// The command gate is review or configuration (an audit-only member gets
// the honest 403 of the scope words BEFORE every material guard); the
// read of the correction state travels through the DecisionContext read
// of 2.14 (the fact rows carry the derived correction markers — no new
// read path exists, the context is ONE resource).
type CorrectionService interface {
	RecordCorrection(context.Context, RecordCorrectionCommand) (WorkOutcome, error)
}
