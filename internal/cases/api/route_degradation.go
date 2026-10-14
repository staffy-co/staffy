package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.12: the two record commands, the RouteDegradationView projection
// and the honest delivery states of the outgoing replies
// ---------------------------------------------------------------------------

// RecordRouteDegradationCommand records one revision of one route
// degradation of a case: the operator with the review OR the configuration
// scope (the gate fires BEFORE the material guards — the M1 precedent of
// 2.5..2.8), the case anchor, the route (the connection id and the channel
// word of the 2.1 dictionary — the guard requires the route to exist in the
// conversations of the case: a degradation of a foreign route is
// impossible), the cause word of the fixed dictionary, the clearing mark
// (its own revision — the recovery row) and the operator note. The revision
// number is the server-side truth: the store derives the next monotonic
// number of the route under the locks (the request never carries it); the
// repeat of the same route is a NEW revision (the append-only history); a
// degradation is an OPERATIONAL record, never a fact.
type RecordRouteDegradationCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	ConnectionID   id.UUID
	Channel        string
	Cause          string
	Cleared        bool
	Note           string
	IdempotencyKey string
	Now            time.Time
}

// RecordDeliveryEvidenceCommand records one delivery evidence event of one
// outgoing reply of the case: the operator with the review OR the
// configuration scope relays the connector observation (the
// ConfirmerOperatorRelay precedent of 2.2 — the machine never invents the
// observation), the kind word of the fixed dictionary
// (delivered_confirmed / delivery_unknown / provider_failed). The event
// number is the server-side truth: the store derives the next monotonic
// number of the reply under the locks; the history is append-only; the
// LAST event decides the derived delivery state.
type RecordDeliveryEvidenceCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	ReplyID        id.UUID
	Kind           string
	IdempotencyKey string
	Now            time.Time
}

// RouteDegradationStateView is one route slot of the CURRENT degraded set:
// the route (the connection and the channel word), the cause word of the
// latest revision, the revision number, the honest cause text (the render
// of the panel — the texts never enter the database or the outbox) and the
// time of the latest record. The current set is the read-time derivation —
// never stored; a recovered route leaves the set (its history stays
// readable).
type RouteDegradationStateView struct {
	ConnectionID id.UUID   `json:"connection_id"`
	Channel      string    `json:"channel"`
	Cause        string    `json:"cause"`
	CauseText    string    `json:"cause_text"`
	Revision     int64     `json:"revision"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// RouteDegradationRevisionView is one append-only revision row of the
// honest history: the route, the cause word, the monotonic revision, the
// clearing mark, the operator note and the actor with the time. The
// history is never rewritten — a new cause or a recovery is a new row.
type RouteDegradationRevisionView struct {
	ConnectionID id.UUID   `json:"connection_id"`
	Channel      string    `json:"channel"`
	Cause        string    `json:"cause"`
	Revision     int64     `json:"revision"`
	Cleared      bool      `json:"cleared"`
	Note         string    `json:"note,omitempty"`
	ActorID      id.UUID   `json:"actor_id"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// RouteDegradationView is the «Состояние маршрутов» section of the case
// view: the CURRENT degraded set (the read-time derivation of the latest
// revision of every route — the recovered routes stay visible in the
// history with the honest clearing mark), the full append-only revision
// history, the cause dictionary of the record form, the channel dictionary
// of the route and the fixed panel words. The degradations are an
// OPERATIONAL record of the service: never facts, never an identity proof.
type RouteDegradationView struct {
	Current      []RouteDegradationStateView    `json:"current"`
	History      []RouteDegradationRevisionView `json:"history"`
	CauseWords   []string                       `json:"cause_words"`
	ChannelWords []string                       `json:"channel_words"`
	RecordsCount int                            `json:"records_count"`
	PanelText    string                         `json:"panel_text"`
}

// ReplyDeliveryView is the derived delivery state of one outgoing reply of
// the case: the state word (prepared / unknown / confirmed / failed — the
// AC-2 honesty: dispatched without evidence is the UNKNOWN, never the
// delivered; the word delivered only ever belongs to the evidence
// observation) with the last event number, the kind word and the time (the
// timeline anchor of AC-9). The state is the read-time derivation — never
// stored.
type ReplyDeliveryView struct {
	ReplyID     id.UUID   `json:"reply_id"`
	State       string    `json:"state"`
	LastEventNo int64     `json:"last_event_no,omitempty"`
	LastKind    string    `json:"last_kind,omitempty"`
	LastEventAt time.Time `json:"last_event_at,omitempty"`
}

// DegradationService is the command surface of Story 2.12 (both commands
// are idempotent by the tenant-scoped key with the stored outcome; the
// revision and the event numbers are the server-side truth under the
// locks; the histories are append-only; the current slots and the delivery
// states are the read-time derivations — never stored).
type DegradationService interface {
	RecordRouteDegradation(context.Context, RecordRouteDegradationCommand) (WorkOutcome, error)
	RecordDeliveryEvidence(context.Context, RecordDeliveryEvidenceCommand) (WorkOutcome, error)
}
