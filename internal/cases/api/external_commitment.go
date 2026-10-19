package api

import (
	"context"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 3.3 command and view surface of the external commitment
// dispatch (the first slice)
// ---------------------------------------------------------------------------
//
// The command DTOs (the create/approve/enqueue/relay commands and the
// outcomes) live in api.go together with the drain report; this file owns
// the SERVICE interface and the READ-TIME view material of the Commitment
// Review page and the machine read: the per-Work-Item sections with the
// immutable envelope version rows, the append-only dispatch histories, the
// queue evidence of the outbox record (the words have NOT left by
// construction — AC-3) and the DERIVED material (the seven-word delivery
// state and the five-word readiness — read-time derivations over the
// consistent rows, never stored words). The wire DTOs carry json tags and
// mappers (the ScopedControlView precedent of 3.2): the machine contract
// and the OpenAPI mirror share the exact wire words, the domain row never
// serialises with Go field names. The empty section is the honest empty: a
// Work Item without envelopes answers the empty version list, a case
// without any envelope answers the empty items list with the panel reason,
// never a null and never a guess.

// ExternalCommitmentVersionWire is the immutable envelope version row on
// the wire: every material field of the append-only row in the fixed
// dictionary words of the epic. The pin triple is all-or-nothing (the
// empty triple is the honest absence); the content token binds the exact
// bytes; the provenance fields (the actor and the creation time) stay —
// the review surface is the history, not a projection.
type ExternalCommitmentVersionWire struct {
	CaseID           id.UUID                   `json:"case_id"`
	WorkItemID       id.UUID                   `json:"work_item_id"`
	ExtCommitmentID  id.UUID                   `json:"ext_commitment_id"`
	VersionNo        int64                     `json:"version_no"`
	Purpose          string                    `json:"purpose"`
	RecipientKind    string                    `json:"recipient_kind"`
	Channel          string                    `json:"channel"`
	ContentText      string                    `json:"content_text"`
	ValidUntil       time.Time                 `json:"valid_until"`
	ExpectedNextStep string                    `json:"expected_next_step"`
	AckRequirement   string                    `json:"ack_requirement"`
	Pin              ExternalCommitmentPinWire `json:"pin"`
	OwnerKind        string                    `json:"owner_kind"`
	OwnerRef         id.UUID                   `json:"owner_ref"`
	OwnerEpoch       int64                     `json:"owner_epoch"`
	DecisionNo       int64                     `json:"decision_no"`
	ProfileVersion   int64                     `json:"profile_version"`
	PolicyOutcomeNo  int64                     `json:"policy_outcome_no"`
	ContentToken     string                    `json:"content_token"`
	ActorID          id.UUID                   `json:"actor_id"`
	CreatedAt        time.Time                 `json:"created_at"`
}

// ExternalCommitmentPinWire is the 2.9 exact-version pin triple on the
// wire: all-or-nothing (the all-empty triple is the honest absence — the
// offer demands the full triple, the domain guard is the one refusal
// source).
type ExternalCommitmentPinWire struct {
	CommitmentID id.UUID `json:"commitment_id"`
	VersionNo    int64   `json:"version_no"`
	ContentToken string  `json:"content_token"`
}

// FromExternalCommitment maps the domain row onto the wire DTO (the pure
// projection — the words stay untouched, the reader never mutates).
func FromExternalCommitment(row casesDomain.ExternalCommitment) ExternalCommitmentVersionWire {
	return ExternalCommitmentVersionWire{
		CaseID:           row.CaseID,
		WorkItemID:       row.WorkItemID,
		ExtCommitmentID:  row.ExtCommitmentID,
		VersionNo:        row.VersionNo,
		Purpose:          row.Purpose,
		RecipientKind:    row.RecipientKind,
		Channel:          row.Channel,
		ContentText:      row.ContentText,
		ValidUntil:       row.ValidUntil,
		ExpectedNextStep: row.ExpectedNextStep,
		AckRequirement:   row.AckRequirement,
		Pin:              ExternalCommitmentPinWire{CommitmentID: row.Pin.CommitmentID, VersionNo: row.Pin.VersionNo, ContentToken: row.Pin.ContentToken},
		OwnerKind:        row.OwnerKind,
		OwnerRef:         row.OwnerRef,
		OwnerEpoch:       row.OwnerEpoch,
		DecisionNo:       row.DecisionNo,
		ProfileVersion:   row.ProfileVersion,
		PolicyOutcomeNo:  row.PolicyOutcomeNo,
		ContentToken:     row.ContentToken,
		ActorID:          row.ActorID,
		CreatedAt:        row.CreatedAt,
	}
}

// ExternalDispatchEventWire is one append-only history row on the wire:
// the monotonic event number, the seven-word kind, the stale reason word
// with the fixed render text of the domain map, the attempt number, the
// external correlation id (the AC-8 evidence), the RECORDED idempotency
// guarantee word, the bounded safe details and the actor. The history is
// never rewritten; the LAST event decides the derived state.
type ExternalDispatchEventWire struct {
	EventNo               int64     `json:"event_no"`
	Kind                  string    `json:"kind"`
	Reason                string    `json:"reason"`
	ReasonText            string    `json:"reason_text"`
	AttemptNo             int64     `json:"attempt_no"`
	ExternalCorrelationID string    `json:"external_correlation_id"`
	IdempotentGuarantee   bool      `json:"idempotent_guarantee"`
	Details               string    `json:"details"`
	ActorID               id.UUID   `json:"actor_id"`
	CreatedAt             time.Time `json:"created_at"`
}

// FromExternalDispatchEvent maps one history row onto the wire DTO: the
// reason text resolves from the DOMAIN map (the «слова не ушли» material
// of the cancelled queue); an absent reason keeps the honest empty string.
func FromExternalDispatchEvent(event casesDomain.ExternalDispatchEvent) ExternalDispatchEventWire {
	return ExternalDispatchEventWire{
		EventNo:               event.EventNo,
		Kind:                  event.Kind,
		Reason:                event.Reason,
		ReasonText:            casesDomain.ExternalStaleReasonText(event.Reason),
		AttemptNo:             event.AttemptNo,
		ExternalCorrelationID: event.ExternalCorrelationID,
		IdempotentGuarantee:   event.IdempotentGuarantee,
		Details:               event.Details,
		ActorID:               event.ActorID,
		CreatedAt:             event.CreatedAt,
	}
}

// ExternalDeliveryWire is the derived delivery state on the wire: the
// seven-word state, the FIXED honest render text of the domain map (the
// submitted text never reads as a delivery, the unknown text names the
// reconciliation path — AC-7), and the material of the LAST event when at
// least one exists (the timeline anchor; the zero values are the honest
// absence of the persisted/queued states).
type ExternalDeliveryWire struct {
	State                     string     `json:"state"`
	StateText                 string     `json:"state_text"`
	LastEventNo               int64      `json:"last_event_no"`
	LastKind                  string     `json:"last_kind"`
	LastReason                string     `json:"last_reason"`
	LastAttemptNo             int64      `json:"last_attempt_no"`
	LastExternalCorrelationID string     `json:"last_external_correlation_id"`
	LastEventAt               *time.Time `json:"last_event_at"`
}

// FromExternalDeliveryState maps the domain derivation onto the wire DTO.
func FromExternalDeliveryState(state casesDomain.ExternalDeliveryState) ExternalDeliveryWire {
	wire := ExternalDeliveryWire{
		State:                     state.State,
		StateText:                 casesDomain.ExternalDeliveryStateText(state.State),
		LastEventNo:               state.LastEventNo,
		LastKind:                  state.LastKind,
		LastReason:                state.LastReason,
		LastAttemptNo:             state.LastAttemptNo,
		LastExternalCorrelationID: state.LastExternalCorrelationID,
	}
	if !state.LastEventAt.IsZero() {
		occurred := state.LastEventAt.UTC()
		wire.LastEventAt = &occurred
	}
	return wire
}

// ExternalCommitmentVersionCard is one envelope version row of the view:
// the immutable version material on the wire, the queue evidence (the
// outbox record with the version pin — the words have NOT left by
// construction), the append-only dispatch history of this version, the
// seven-word delivery derivation, the five-word readiness derivation, the
// approval material of the EXACT version and the current flag (the
// maximal number wins — a superseded version is stated in TEXT, never by
// the colour alone).
type ExternalCommitmentVersionCard struct {
	Version       ExternalCommitmentVersionWire `json:"version"`
	Enqueued      bool                          `json:"enqueued"`
	Delivery      ExternalDeliveryWire          `json:"delivery"`
	Readiness     string                        `json:"readiness"`
	ReadinessText string                        `json:"readiness_text"`
	Current       bool                          `json:"current"`
	Approved      bool                          `json:"approved"`
	ApproverID    id.UUID                       `json:"approver_id"`
	Events        []ExternalDispatchEventWire   `json:"events"`
}

// ExternalCommitmentWorkItemView is one Work Item section of the external
// commitment view: the envelope version cards grouped by the Work Item
// (the page surface of one Work Item reads its section).
type ExternalCommitmentWorkItemView struct {
	WorkItemID id.UUID                         `json:"work_item_id"`
	Versions   []ExternalCommitmentVersionCard `json:"versions"`
}

// ExternalCommitmentView is the external commitment section of the case
// view: the per-Work-Item sections, the dictionary words of the forms (the
// compile-time dictionaries of the domain — the render texts live in the
// domain maps, never here) and the honest panel text. The empty items
// list IS the honest answer of a case without envelopes.
type ExternalCommitmentView struct {
	Items          []ExternalCommitmentWorkItemView `json:"items"`
	PurposeWords   []string                         `json:"purpose_words"`
	RecipientWords []string                         `json:"recipient_words"`
	AckWords       []string                         `json:"ack_words"`
	RelayWords     []string                         `json:"relay_words"`
	ChannelWords   []string                         `json:"channel_words"`
	StateWords     []string                         `json:"state_words"`
	PanelText      string                           `json:"panel_text"`
	EmptyText      string                           `json:"empty_text"`
}

// ExternalCommitmentService is the command surface of Story 3.3 (the
// first slice): the five commands of the envelope lifecycle — the version
// creation, the exact-version approval of the current human owner, the
// persist-before-emit enqueue, the operator receipt relay and the bounded
// drain (no daemons exist — AD-13; the drain is an operator command) —
// together with the read-time view of the Commitment Review surface. The
// version ladder, the UNIQUE rows and the outbox dedup key are the
// idempotency of the surface — the commands carry no client key; the
// state words are read-time derivations, never stored.
type ExternalCommitmentService interface {
	CreateExternalCommitment(context.Context, CreateExternalCommitmentCommand) (ExternalCommitmentOutcome, error)
	ApproveExternalCommitment(context.Context, ApproveExternalCommitmentCommand) (ExternalApprovalOutcome, error)
	EnqueueExternalCommitment(context.Context, EnqueueExternalCommitmentCommand) (ExternalEnqueueOutcome, error)
	RelayExternalReceipt(context.Context, RelayExternalReceiptCommand) (ExternalRelayOutcome, error)
	DrainExternalCommitments(context.Context, domain.Principal, id.UUID) (ExternalDrainResult, error)
	ReadExternalCommitmentView(context.Context, domain.Principal, id.UUID, id.UUID) (ExternalCommitmentView, error)
}
