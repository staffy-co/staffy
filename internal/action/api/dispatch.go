package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/action/port"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 3.6 (step-03): the execution-contour surface (the DTO layer — the
// transport renders THESE shapes only; the AuthorizationService precedent
// of 3-5)
// ---------------------------------------------------------------------------
//
// The drain is the EXPLICIT operator command (the DrainExternalCommitments
// precedent of 3-3): NO daemon, NO ticker — the scheduler is a future
// infrastructure. The result speaks the honest report in words: every
// considered job leaves a line (dispatched, parked with the exact fence
// reason, or skipped with the honest skip word — FR-28). The operation
// view extension carries the life state (the six-word dictionary), the
// queue fact, the visible block cause, the progressive attempts in the
// closed details and the Action Receipt card with the identifier readback
// (AC-13/FR-31: the decision is reconstructible without the dialog — SAFE
// pins and words only, the material NEVER enters).

// DrainDispatchCommand is the explicit drain command of the execution
// contour: the acting operator principal, the tenant and the moment. The
// batch is bounded by the domain drain limit (DispatchDrainLimit=100) —
// the rest stay queued for the next run, honestly.
type DrainDispatchCommand struct {
	Actor    identityDomain.Principal
	TenantID id.UUID
	Now      time.Time
}

// DispatchReportLineView is ONE spoken line of the drain report: the
// operation, the attempt number of the line and the honest word (every
// boundary names the exact cause and the allowed next step — FR-28).
type DispatchReportLineView struct {
	OperationID id.UUID `json:"operation_id"`
	AttemptNo   int     `json:"attempt_no"`
	Text        string  `json:"text"`
}

// DispatchDrainResult is the honest answer of the drain command: how many
// jobs were considered, how many were dispatched, how many parked on a
// fence and how many skipped (races, replays, alarms) — with the spoken
// line of EVERY job (never a masked silence).
type DispatchDrainResult struct {
	Command    string                   `json:"command"`
	Considered int                      `json:"considered"`
	Dispatched int                      `json:"dispatched"`
	Blocked    int                      `json:"blocked"`
	Skipped    int                      `json:"skipped"`
	Reports    []DispatchReportLineView `json:"reports"`
}

// DispatchAttemptView is one progressive attempt of the operation card
// (AC-13: the attempts render in CLOSED details — the card stays legible
// at 320px): the write-ahead lease facts (the right and the moment of the
// send landed BEFORE the network call), the keyed request digest (the
// redacted trace — the raw payload never exists), the honest claim word
// of the adapter WHEN recorded (the four-word dictionary; the un-recorded
// attempt speaks the write-ahead resting text by name), the provider
// correlation and the recorded guarantee word (the retry policy input).
type DispatchAttemptView struct {
	AttemptNo         int       `json:"attempt_no"`
	LeaseCreatedAt    time.Time `json:"lease_created_at"`
	LeaseExpiresAt    time.Time `json:"lease_expires_at"`
	RequestDigest     string    `json:"request_digest"`
	Outcome           string    `json:"outcome,omitempty"`
	OutcomeRecorded   bool      `json:"outcome_recorded"`
	OutcomeText       string    `json:"outcome_text"`
	CorrelationID     string    `json:"correlation_id"`
	GuaranteeRecorded bool      `json:"guarantee_recorded"`
	OutcomeCreatedAt  time.Time `json:"outcome_recorded_at,omitempty"`
}

// ActionReceiptView is one version of the Action Receipt card (FR-31/AC-12:
// append-only, v1 is never rewritten — the later evidence of 3-7 appends
// v2+): the three-word outcome with its fixed honest text, the SAFE
// reference pins (the identifier readback — the decision reconstructs
// WITHOUT the dialog; the sensitive material NEVER enters) and the safe
// details.
type ActionReceiptView struct {
	ReceiptVersionNo      int64     `json:"receipt_version_no"`
	Outcome               string    `json:"outcome"`
	OutcomeText           string    `json:"outcome_text"`
	CaseID                id.UUID   `json:"case_id"`
	WorkItemID            id.UUID   `json:"work_item_id"`
	PreviewID             id.UUID   `json:"preview_id"`
	PreviewVersionNo      int64     `json:"preview_version_no"`
	ConfirmationID        id.UUID   `json:"confirmation_id"`
	ConfirmationVersionNo int64     `json:"confirmation_version_no"`
	DefinitionID          string    `json:"definition_id"`
	DefinitionVersion     string    `json:"definition_version"`
	ExternalObjectID      string    `json:"external_object_id"`
	ExternalObjectVersion string    `json:"external_object_version"`
	Details               string    `json:"details"`
	CreatedAt             time.Time `json:"created_at"`
}

// DispatchService is the narrow execution-contour surface the transport
// type-asserts (the AuthorizationService precedent): the composite app
// Service implements it, the fixture and transport composition roots bind
// it. The GET-side view extension rides the ReadAuthorizationView of the
// authorization surface — the operation card grows the dispatch fields
// (the life state, the queue fact, the block cause, the attempts, the
// receipts) through the SAME read.
type DispatchService interface {
	// DrainDispatchQueue runs the EXPLICIT bounded drain of the queued
	// operations of the tenant (operator manage — the operations scope
	// word; a non-member is the honest 404 BEFORE the queue is read).
	// The nil mutation adapter of the production root refuses honestly
	// FIRST (AG-4 — the queue stays untouched, zero sends); every job
	// of the bounded batch walks the ordered fences — a boundary parks
	// the operation with the EXACT reason word, never a false failed
	// externally (AC-2).
	DrainDispatchQueue(ctx context.Context, command DrainDispatchCommand) (DispatchDrainResult, error)
}

// The store-contract errors of the execution contour the transport maps
// (the consumer-owned vocabulary of the repository boundary re-exported
// for the render layer; the AuthorizationService precedent).
var (
	// ErrDispatchRefusedAG4 is the honest production refusal of the
	// drain command at a nil mutation adapter (AG-4: no real providers
	// exist — the queue honestly stays pending).
	ErrDispatchRefusedAG4 = port.ErrDispatchRefusedAG4
	// ErrLeaseStale is the honest fenced refusal (the stale holder
	// never writes history — the drain report speaks the skip).
	ErrLeaseStale = port.ErrLeaseStale
	// ErrLeaseRaced is the honest racing skip (an active lease of
	// another consideration owns the operation right now).
	ErrLeaseRaced = port.ErrLeaseRaced
)
