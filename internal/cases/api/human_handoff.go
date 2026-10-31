package api

import (
	"context"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.13: the six handoff lifecycle commands, the HandoffView
// projection of the case panel and the honest client status of CaseView
// ---------------------------------------------------------------------------

// RequestHumanCommand records the client's unconditional ask of a human
// (AC-1): the operator with the review OR the configuration scope (the
// gate fires BEFORE the material guards — the M1 precedent of
// 2.5..2.12), the case anchor, the accepted client turn that asked for
// the human (the anchor of the unconditional ask — the store guard
// proves the turn belongs to THIS case), the CAS expectation of the mode
// ladder version, the target roles of the ask (the non-empty subset of
// the fixed dictionary) and the ack-deadline word of the expected
// confirmation window (the approved policy v0 dictionary). The reason of
// the unconditional ask is NEVER an input: the dictionary constant
// client_request is the system truth (the operator never types it). The
// Story 4.3 extension: the operator MAY carry one boundary word
// (diagnostic_uncertainty / safety_critical / unresolved_contradiction —
// the recommendation of the part request panel, the operator's decision
// — GuardHandoffRequestReason is fail-closed on the dictionary; the SQL
// CHECK of 00019 is the backstop). The mode raise is the conditional
// ratchet term (a stricter active mode is never touched — the ask is
// still recorded); the request and event numbers are the server-side
// truth (the store derives the next monotonic numbers under the locks —
// the request never carries them).
type RequestHumanCommand struct {
	Actor               domain.Principal
	CorrelationID       id.UUID
	TenantID            id.UUID
	CaseID              id.UUID
	TurnRef             id.UUID
	ExpectedModeVersion int64
	TargetRoles         []string
	AckDeadline         string
	// Reason is the OPTIONAL Story 4.3 boundary word (empty = the
	// unconditional client ask of 2.13 — the reason stays the system
	// truth client_request; an explicit client_request input is the
	// honest refusal — the word is never typed).
	Reason         string
	IdempotencyKey string
	Now            time.Time
}

// AssignHumanHandoffCommand records one assignment round of the open
// lifecycle (AC-3): the assignee and the backup member references with
// the round's ack-deadline word. The assignee's role word is the
// SERVER-side resolution of the membership truth (the app resolves it —
// the operator never inputs the role; the store re-validates it against
// the request's target roles under the locks). The assignment NEVER
// changes the owner (назначение ≠ принятие — AC-3/AC-11); the event
// number is the server-side truth under the locks.
type AssignHumanHandoffCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	Assignee       id.UUID
	Backup         id.UUID
	AckDeadline    string
	IdempotencyKey string
	Now            time.Time
}

// AcknowledgeHumanHandoffCommand is the assignee's confirmed acceptance
// (AC-5): the actor IS the assignee (the server truth — the assignment
// or the notification is never an acknowledgement), the CAS expectation
// of the ownership epoch (the ONE transition that moves the ownership —
// the exec-composition of the 2.4 transfer machinery in the SAME
// transaction). The raced twin of the two-operator race loses on the
// locks and the UNIQUE + CAS backstops (the honest ErrStaleVersion).
type AcknowledgeHumanHandoffCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	ExpectedEpoch  int64
	IdempotencyKey string
	Now            time.Time
}

// DeclineHumanHandoffCommand is the assignee's honest refusal of the
// assigned round (AC-6): the actor IS the assignee; the owner NEVER
// changes (the prior safe owner stays); the request stays open for the
// human re-assignment (the fresh ask, the overdue round, the declined
// round — the human re-decision).
type DeclineHumanHandoffCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	IdempotencyKey string
	Now            time.Time
}

// CancelHumanRequestCommand records the client's cancellation of the
// open lifecycle (AC-10): the anchor is the accepted client turn that
// cancelled the ask (the store guard proves the turn belongs to THIS
// case). The owner NEVER changes and the history is never deleted; the
// mode stays (the 2.5 ratchet — the exit is the authoritative
// exit_mode); the acknowledged lifecycle refuses (the path back is
// return_to_staffy).
type CancelHumanRequestCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	TurnRef        id.UUID
	IdempotencyKey string
	Now            time.Time
}

// ReturnToStaffyCommand is the human owner's explicit return of the
// acknowledged lifecycle (AC-9): the result word of the fixed dictionary
// (issue_resolved / handed_back — the free material never exists) and
// the CAS expectation of the ownership epoch. The return composes the
// 2.4 transfer machinery in the SAME transaction (the owner=staffy, the
// external_inbox surface, the epoch+1); the mode is NEVER touched (the
// ladder keeps its honest word).
type ReturnToStaffyCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	Result         string
	ExpectedEpoch  int64
	IdempotencyKey string
	Now            time.Time
}

// HandoffEventView is one append-only lifecycle event row of the panel
// history: the request and event numbers (the monotonic server truth),
// the kind word with its honest text, the per-kind material (the
// assigned round's assignee/backup/deadline; the acknowledged epoch; the
// cancelled anchor turn; the returned result word) and the actor with
// the time. The history is never rewritten — every transition is a new
// row; the read-time derivations never travel in the rows.
type HandoffEventView struct {
	RequestNo      int64     `json:"request_no"`
	EventNo        int64     `json:"event_no"`
	Kind           string    `json:"kind"`
	KindText       string    `json:"kind_text"`
	Assignee       id.UUID   `json:"assignee,omitempty"`
	BackupOwner    id.UUID   `json:"backup_owner,omitempty"`
	AckDeadline    string    `json:"ack_deadline,omitempty"`
	DeadlineAt     time.Time `json:"deadline_at,omitempty"`
	Result         string    `json:"result,omitempty"`
	ResultText     string    `json:"result_text,omitempty"`
	TurnRef        id.UUID   `json:"turn_ref,omitempty"`
	ResultingEpoch int64     `json:"resulting_epoch,omitempty"`
	ActorID        id.UUID   `json:"actor_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// HandoffRequestView is the derived lifecycle of one append-only request
// row: the state word with its honest text (the read-time derivation —
// never stored), the client status word with its honest text (назначение
// ≠ принятие are TWO words — AC-11), the material of the current
// assignment round (the assignee, the backup and the deadline of the
// LATEST assigned event), the target roles, the ask's anchor turn and
// the append-only event history of the request.
type HandoffRequestView struct {
	RequestNo        int64              `json:"request_no"`
	Reason           string             `json:"reason"`
	ReasonText       string             `json:"reason_text"`
	State            string             `json:"state"`
	StateText        string             `json:"state_text"`
	ClientStatus     string             `json:"client_status"`
	ClientStatusText string             `json:"client_status_text"`
	Active           bool               `json:"active"`
	OwnerKind        string             `json:"owner_kind"`
	OwnerRef         id.UUID            `json:"owner_ref"`
	TargetRoles      []string           `json:"target_roles"`
	AckDeadline      string             `json:"ack_deadline"`
	AckDeadlineText  string             `json:"ack_deadline_text"`
	TurnRef          id.UUID            `json:"turn_ref"`
	ActorID          id.UUID            `json:"actor_id"`
	CreatedAt        time.Time          `json:"created_at"`
	Assignee         id.UUID            `json:"assignee,omitempty"`
	BackupOwner      id.UUID            `json:"backup_owner,omitempty"`
	DeadlineAt       time.Time          `json:"deadline_at,omitempty"`
	Events           []HandoffEventView `json:"events"`
}

// HandoffView is the «Запрос человека» section of the case view: every
// append-only request row with its derived lifecycle and its event
// history (ordered by the monotonic numbers), the honest client status
// of the LAST request (the word + the text of the interface truth —
// CaseView carries it for the client surfaces), the dictionaries of the
// forms (the target roles, the ack deadlines, the return results — the
// fixed compile-time words, never the invented options) and the fixed
// panel words. The handoff is a LIFECYCLE RECORD of the case: never a
// fact, never an identity proof, never a stored status — the overdue
// state is the read-time derivation of the open assigned round.
type HandoffView struct {
	Requests         []HandoffRequestView `json:"requests"`
	RequestsCount    int                  `json:"requests_count"`
	ClientStatus     string               `json:"client_status"`
	ClientStatusText string               `json:"client_status_text"`
	// AcknowledgeForActor reports whether the acting principal IS the
	// assignee of the open assigned round (the card «Передача ждёт вашего
	// подтверждения» is the assignee's surface — the assignment or the
	// notification is never an acknowledgement, AC-3/AC-5; the other
	// operators see the honest state line of the panel only).
	AcknowledgeForActor bool `json:"acknowledge_for_actor"`
	// The round the assignee answers: the assignee of the LATEST assigned
	// event of the open lifecycle with the deadline (zero values when no
	// open round lives — the card degrades honestly).
	RoundAssignee   id.UUID   `json:"round_assignee,omitempty"`
	RoundDeadlineAt time.Time `json:"round_deadline_at,omitempty"`
	RoleWords       []string  `json:"role_words"`
	DeadlineWords   []string  `json:"deadline_words"`
	ResultWords     []string  `json:"result_words"`
	PanelText       string    `json:"panel_text"`
}

// HandoffService is the command surface of Story 2.13 (all six commands
// are idempotent by the tenant-scoped key with the stored outcome; the
// request and event numbers are the server-side truth under the locks;
// the histories are append-only; the lifecycle states and the client
// statuses are the read-time derivations — never stored).
type HandoffService interface {
	RequestHuman(context.Context, RequestHumanCommand) (WorkOutcome, error)
	AssignHumanHandoff(context.Context, AssignHumanHandoffCommand) (WorkOutcome, error)
	AcknowledgeHumanHandoff(context.Context, AcknowledgeHumanHandoffCommand) (WorkOutcome, error)
	DeclineHumanHandoff(context.Context, DeclineHumanHandoffCommand) (WorkOutcome, error)
	CancelHumanRequest(context.Context, CancelHumanRequestCommand) (WorkOutcome, error)
	ReturnToStaffy(context.Context, ReturnToStaffyCommand) (WorkOutcome, error)
}

// handoffKindTexts renders the honest text of every lifecycle event kind
// (the render material of the panel history — the texts never enter the
// database or the outbox).
var handoffKindTexts = map[string]string{
	casesDomain.HandoffEventAssigned:     "Назначение специалиста: ожидание подтверждения.",
	casesDomain.HandoffEventAcknowledged: "Подтверждение специалиста: передача принята.",
	casesDomain.HandoffEventDeclined:     "Отказ специалиста: ожидание нового назначения.",
	casesDomain.HandoffEventCancelled:    "Отмена клиентом: запрос человека закрыт.",
	casesDomain.HandoffEventReturned:     "Возврат цифровому сотруднику.",
}

// HandoffEventKindText resolves the honest text of one event kind word.
func HandoffEventKindText(kind string) string {
	return handoffKindTexts[kind]
}
