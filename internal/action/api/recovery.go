package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 3.8 (step-03): the recovery-case surface (the DTO layer — the
// transport renders THESE shapes only; the ReconcileService precedent of
// 3-7)
// ---------------------------------------------------------------------------
//
// The creation is an EXPLICIT operator command (the drain/reconcile
// precedent): NO daemon, NO ticker. The result speaks the honest
// decision word (created/idempotent/evidence) with its fixed text — the
// idempotent repeat returns the EXISTING case (no second row, no event),
// the evidence append speaks the ONE new append-only row. The card view
// carries the SAFE pins of the frozen bundle with the LIVING receipt
// chain derived at the read (AC-10: the context updates by the
// references — the card is always fresh, the frozen bundle never
// rewrites), the assignment words, the append-only event history and the
// honest boundary texts (the payments unknown of AG-4, the local
// projection word). The material NEVER enters (SAFE pins and words
// only, AD-10/AC-9).

// RecoveryCreateCommand is the explicit creation command of the recovery
// contour: the acting operator principal, the tenant, the operation pin
// and the moment. The body carries NOTHING (the words of the assignment
// NEVER come from a command parameter — the LIVE readings own them).
type RecoveryCreateCommand struct {
	Actor       identityDomain.Principal
	TenantID    id.UUID
	OperationID id.UUID
	Now         time.Time
}

// RecoveryAssignmentView is the assignment of the case: the owner triple
// of the CURRENT ownership term of the original Case (the LIVE reading
// of the creation moment), the backup owner and the ack-deadline minutes
// of the human coverage of the original conversation route — the
// materialized deadline_at rides the case view.
type RecoveryAssignmentView struct {
	OwnerKind          string  `json:"owner_kind"`
	OwnerRef           id.UUID `json:"owner_ref"`
	OwnerEpoch         int64   `json:"owner_epoch"`
	BackupOwnerID      id.UUID `json:"backup_owner_id"`
	AckDeadlineMinutes int64   `json:"ack_deadline_minutes"`
}

// RecoveryEventView is ONE append-only row of the case history: the
// monotonic event number, the kind word (created/evidence_appended), the
// SAFE details (the delta words of the evidence appends) and the moment.
type RecoveryEventView struct {
	EventNo   int64     `json:"event_no"`
	Kind      string    `json:"kind"`
	Details   string    `json:"details"`
	ActorID   id.UUID   `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}

// RecoveryLinksView is the SAFE pin set of the case card: the origin and
// manifest pins (the EXACT preview/confirmation versions), the LIVING
// receipt chain pins (the version count, the latest version with its
// outcome — derived at the read, AC-10), the attempt counters of both
// contours, the external identity of the latest receipt (empty — the
// honest unknown) and the owning system word. The transitions to the
// original evidence carry these pins (the context-preserving links).
type RecoveryLinksView struct {
	PreviewID              id.UUID                      `json:"preview_id"`
	PreviewVersionNo       int64                        `json:"preview_version_no"`
	ConfirmationID         id.UUID                      `json:"confirmation_id"`
	ConfirmationVersionNo  int64                        `json:"confirmation_version_no"`
	ReceiptVersionsCount   int64                        `json:"receipt_versions_count"`
	LatestReceiptVersionNo int64                        `json:"latest_receipt_version_no"`
	LatestReceiptOutcome   string                       `json:"latest_receipt_outcome"`
	DispatchAttempts       int                          `json:"dispatch_attempts"`
	ReconcileAttempts      int                          `json:"reconcile_attempts"`
	ExternalObjectID       string                       `json:"external_object_id"`
	ExternalObjectVersion  string                       `json:"external_object_version"`
	ExternalObjectTime     string                       `json:"external_object_time,omitempty"`
	OwningSystem           string                       `json:"owning_system"`
	Observations           []RecoveryObservationPinView `json:"observations"`
}

// RecoveryObservationPinView is ONE supporting-observation pin of the
// bundle (the 2-7 version pin — the reference, never the material).
type RecoveryObservationPinView struct {
	Ref     id.UUID `json:"ref"`
	Version int64   `json:"version"`
}

// RecoveryCaseView is the card view of ONE Recovery Case: the identity
// pins, the trigger word with its fixed text, the read-time state word
// (open — the closing words do not exist in this slice) with its honest
// text, the assignment, the materialized deadline, the append-only event
// history, the SAFE link pins and the honest boundary texts of the
// surface (the payments unknown of AG-4, the local projection word).
type RecoveryCaseView struct {
	RecoveryCaseID      id.UUID                `json:"recovery_case_id"`
	RecoveryCaseNo      int64                  `json:"recovery_case_no"`
	OperationID         id.UUID                `json:"operation_id"`
	CaseID              id.UUID                `json:"case_id"`
	WorkItemID          id.UUID                `json:"work_item_id"`
	DefinitionID        string                 `json:"definition_id"`
	DefinitionVersion   string                 `json:"definition_version"`
	Trigger             string                 `json:"trigger"`
	TriggerText         string                 `json:"trigger_text"`
	State               string                 `json:"state"`
	StateText           string                 `json:"state_text"`
	Assignment          RecoveryAssignmentView `json:"assignment"`
	DeadlineAt          time.Time              `json:"deadline_at"`
	Links               RecoveryLinksView      `json:"links"`
	Events              []RecoveryEventView    `json:"events"`
	PaymentsUnknownText string                 `json:"payments_unknown_text"`
	LocalProjectionText string                 `json:"local_projection_text"`
	CreatedAt           time.Time              `json:"created_at"`
}

// RecoveryCreateResultView is the honest answer of the creation command:
// the decision word (created/idempotent/evidence) with its fixed text
// and the case view (the existing one on the repeat paths).
type RecoveryCreateResultView struct {
	Decision string           `json:"decision"`
	Text     string           `json:"text"`
	Case     RecoveryCaseView `json:"recovery_case"`
}

// RecoveryService is the narrow recovery-contour surface the transport
// type-asserts (the ReconcileService precedent): the composite app
// Service implements it, the fixture and transport composition roots
// bind it. The contour is LOCAL — no adapter exists anywhere in the
// wiring (the AG-4 production root still binds the REAL service: the
// recovery cases get created honestly, the eligibility answers
// reconciliation_unavailable when the reconciliation contour is not
// wired).
type RecoveryService interface {
	// CreateRecoveryCase runs the EXPLICIT creation command of one
	// operation (operator manage — the operations scope word; a
	// non-member is the honest 404 BEFORE the facts are read; an
	// ineligible operation answers the honest refusal word of the exact
	// gas — NOTHING is written; the assignment of the LIVE readings
	// gates the creation, both fail-closed refusals write NOTHING).
	CreateRecoveryCase(ctx context.Context, command RecoveryCreateCommand) (RecoveryCreateResultView, error)
	// ReadRecoveryCase reads ONE case for the card view: the frozen SAFE
	// bundle with the LIVING receipt chain derived at the read (AC-10)
	// and the append-only event history. Unknown answers the honest 404.
	ReadRecoveryCase(ctx context.Context, actor identityDomain.Principal, tenantID, recoveryCaseID id.UUID) (RecoveryCaseView, error)
	// ReadRecoveryCases reads the tenant's cases for the panel list
	// (bounded by the panel limit), ordered by the creation moment
	// descending.
	ReadRecoveryCases(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]RecoveryCaseView, error)
	// RecoveryCasePolicy derives the READ-TIME recovery action policy of
	// ONE case (story 3-9): the allowed set of the FIVE kinds with the
	// DECLARED compile-time attributes (AC-2) and the per-kind state
	// words {allowed/blocked/human_only} with the honest reason texts +
	// the SEPARATE human-only class (AC-1) — a pure derivation over the
	// LIVE readings, NOT ONE row lands anywhere, the repeated request
	// answers the SAME view. Unknown answers the honest 404 (the
	// membership blindness BEFORE any material).
	RecoveryCasePolicy(ctx context.Context, actor identityDomain.Principal, tenantID, recoveryCaseID id.UUID) (RecoveryPolicyView, error)
}

// ---------------------------------------------------------------------------
// Story 3.9 (step-03): the recovery action policy surface (the DTO
// layer — the transport renders THESE shapes only; the machine view the
// future planner validates its proposals against, AD-7)
// ---------------------------------------------------------------------------

// RecoveryActionView is ONE kind's policy answer of the machine view:
// the kind word, the read-time state word {allowed/blocked/human_only}
// with the honest reason text, and the DECLARED compile-time attributes
// of the kind — AC-2 дословно: «каждый вариант имеет scope,
// eligibility, требуемые факты, authority и Confirmation». The state
// NEVER promises the success of the action (the policy is the boundary
// of the RIGHT, not an execution plan).
type RecoveryActionView struct {
	Kind                 string `json:"kind"`
	State                string `json:"state"`
	StateText            string `json:"state_text"`
	ScopeText            string `json:"scope_text"`
	EligibilityRuleText  string `json:"eligibility_rule_text"`
	RequiredFactsText    string `json:"required_facts_text"`
	AuthorityText        string `json:"authority_text"`
	ConfirmationRequired bool   `json:"confirmation_required"`
}

// RecoveryHumanOnlyDecisionView is ONE human-only decision word of the
// SEPARATE class with its boundary text (AC-1: the class is listed
// apart from the allowed set — the words never mix; the boundary words
// are the PRD FR-39 consequences verbatim).
type RecoveryHumanOnlyDecisionView struct {
	Kind         string `json:"kind"`
	BoundaryText string `json:"boundary_text"`
}

// RecoveryPolicyView is the machine policy view of ONE case: the case
// pin, the FIVE kind answers in the dictionary order (the fixed order
// of the machine view and the page section) and the human-only class
// with the class-level boundary word. The material NEVER enters (words
// and attributes only — the SAFE discipline of the module, AD-10).
type RecoveryPolicyView struct {
	RecoveryCaseID     id.UUID                         `json:"recovery_case_id"`
	Actions            []RecoveryActionView            `json:"actions"`
	HumanOnlyDecisions []RecoveryHumanOnlyDecisionView `json:"human_only_decisions"`
	HumanOnlyClassText string                          `json:"human_only_class_text"`
}

// The store-contract errors of the recovery contour the transport maps
// (the consumer-owned vocabulary of the repository boundary re-exported
// for the render layer; the ReconcileService precedent).
var (
	// ErrRecoveryNotFound is the honest 404 of the unknown case or the
	// unknown operation of the command (the membership blindness).
	ErrRecoveryNotFound = port.ErrRecoveryNotFound
	// ErrRecoveryIneligible is the honest refusal word of the
	// eligibility ladder (the wrapped text names the exact gas).
	ErrRecoveryIneligible = port.ErrRecoveryIneligible
	// ErrRecoveryCoverageUnavailable is the fail-closed assignment word
	// of the unreadable human coverage.
	ErrRecoveryCoverageUnavailable = port.ErrRecoveryCoverageUnavailable
	// ErrRecoveryOwnerUnknown is the fail-closed assignment word of the
	// owner-less original Case (the pre-migration honest state).
	ErrRecoveryOwnerUnknown = port.ErrRecoveryOwnerUnknown
)

// RecoveryDecisionWordsView re-exports the decision dictionary for the
// render layer (the mirror probe of the contract test).
var RecoveryDecisionWordsView = port.RecoveryDecisionWords

// RecoveryTriggerWordsView re-exports the trigger dictionary (the enum
// mirror of the contract test — the drift turns it red).
var RecoveryTriggerWordsView = domain.RecoveryTriggerWords

// RecoveryEventKindWordsView re-exports the event kind dictionary.
var RecoveryEventKindWordsView = domain.RecoveryEventKindWords

// RecoveryCaseStateWordsView re-exports the case state dictionary.
var RecoveryCaseStateWordsView = domain.RecoveryCaseStateWords

// RecoveryActionKindWordsView re-exports the policy kind dictionary
// (the enum mirror of the contract test — the drift turns it red).
var RecoveryActionKindWordsView = domain.RecoveryActionKindWords

// RecoveryHumanOnlyWordsView re-exports the human-only dictionary (the
// enum mirror of the contract test — the class is listed SEPARATELY).
var RecoveryHumanOnlyWordsView = domain.RecoveryHumanOnlyWords

// RecoveryActionStateWordsView re-exports the policy state dictionary
// (the enum mirror of the contract test).
var RecoveryActionStateWordsView = domain.RecoveryActionStateWords
