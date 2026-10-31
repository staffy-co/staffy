package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The honest delivery state of the outgoing reply (Story 2.12, first
// slice): the evidence dictionary, the read-time derivation, the record
// guard and the outbox perimeter
// ---------------------------------------------------------------------------
//
// The 2.4 drain outcome is an honest record of the record: dispatched
// means «the reply row exists and the ownership did not move under it» —
// it is NOT a delivery. The real channel delivery is THIS slice: the
// operator relays the connector's observation (the ConfirmerOperatorRelay
// precedent of 2.2 — the operator passes the machine observation, the
// machine never invents it) as an append-only evidence row of one reply.
// The words are the three honest observations: the receipt is confirmed,
// the outcome is ambiguous, the provider refused terminally.
//
// The delivery STATE of a reply is a read-time derivation — never a
// stored status: without the dispatched outcome the truth of 2.4 stays
// «prepared» (nothing has left); dispatched without any evidence event
// is structurally UNKNOWN — «результат становится unknown, а не
// delivered» (AC-2: dispatched is a record of a record, never a
// delivery; the word delivered_confirmed is only ever an evidence
// observation); the LAST evidence event of the monotonic history decides
// the state otherwise. The blind retry is structurally impossible: the
// drain is single-record (the 2.4 UNIQUE), a retry command does not
// exist and a re-send is a NEW reply row — a conscious human decision
// (AC-2 «reconciliation либо решение человека»).

// ---------------------------------------------------------------------------
// The evidence dictionary (the connector observations relayed by the
// operator)
// ---------------------------------------------------------------------------

// The delivery evidence kinds: the three honest observations of the
// channel world — the receipt confirmed, the outcome ambiguous, the
// provider refused terminally.
const (
	DeliveryEvidenceDeliveredConfirmed = "delivered_confirmed"
	DeliveryEvidenceDeliveryUnknown    = "delivery_unknown"
	DeliveryEvidenceProviderFailed     = "provider_failed"
)

// DeliveryEvidenceWords is the full dictionary (the canonical order).
var DeliveryEvidenceWords = []string{
	DeliveryEvidenceDeliveredConfirmed, DeliveryEvidenceDeliveryUnknown, DeliveryEvidenceProviderFailed,
}

// DeliveryEvidenceTexts is the fixed honest text of every evidence word
// (the Domain Pack v0 render material of the «Доставка» column; the texts
// never enter the database or the outbox).
var DeliveryEvidenceTexts = map[string]string{
	DeliveryEvidenceDeliveredConfirmed: "Доставка подтверждена: квитанция канала получена.",
	DeliveryEvidenceDeliveryUnknown:    "Доставка неизвестна: квитанции нет — итог неопределён.",
	DeliveryEvidenceProviderFailed:     "Отказ провайдера: канал сообщил об ошибке доставки.",
}

// DeliveryEvidenceText resolves the honest text of one evidence word (the
// empty string of an unknown word — the render layer only).
func DeliveryEvidenceText(kind string) string {
	return DeliveryEvidenceTexts[kind]
}

// DeliveryEvidenceInDictionary reports whether the evidence word belongs
// to the fixed dictionary (the fail-closed basis of the record guard).
func DeliveryEvidenceInDictionary(kind string) bool {
	return inDictionary(DeliveryEvidenceWords, kind)
}

// ---------------------------------------------------------------------------
// The derived state words (the panel column «Доставка»)
// ---------------------------------------------------------------------------

// The delivery state words of the derivation: prepared (the 2.4 truth —
// nothing has left), unknown (dispatched without a confirmed receipt —
// the honest silence), confirmed (the receipt observation) and failed
// (the terminal provider refusal). The word delivered NEVER appears
// alone — only the evidence word delivered_confirmed carries it.
const (
	ReplyDeliveryPrepared  = "prepared"
	ReplyDeliveryUnknown   = "unknown"
	ReplyDeliveryConfirmed = "confirmed"
	ReplyDeliveryFailed    = "failed"
)

// ---------------------------------------------------------------------------
// The row shape (the store reads/writes; the state is derived, never
// stored)
// ---------------------------------------------------------------------------

// ReplyDeliveryEvent is one append-only evidence row of one reply: the
// monotonic event number (the readable observation history — the
// DisclosureDeliveryEvent precedent of 2.5), the dictionary kind word,
// the relaying actor and the time. The history is never rewritten; the
// LAST event decides the state.
type ReplyDeliveryEvent struct {
	TenantID  id.UUID
	CaseID    id.UUID
	ReplyID   id.UUID
	EventID   id.UUID
	EventNo   int64
	Kind      string
	ActorID   id.UUID
	CreatedAt time.Time
}

// ---------------------------------------------------------------------------
// The read-time derivation (the honest state, never stored)
// ---------------------------------------------------------------------------

// ReplyDeliveryState is the derived delivery state of one reply: the
// state word, and — when at least one evidence event exists — the number,
// the kind and the time of the LAST event (the timeline anchor of
// AC-9: «Timeline показывает фактический результат и время»).
type ReplyDeliveryState struct {
	State       string
	LastEventNo int64
	LastKind    string
	LastEventAt time.Time
}

// DeriveReplyDeliveryState derives the read-time delivery state of one
// reply: without the dispatched outcome the truth of 2.4 stays prepared
// (nothing has left — the derivation never trusts an evidence of an
// unsent row defensively); dispatched without any evidence event is
// UNKNOWN (AC-2: the record of a record is not a delivery); otherwise
// the LAST event of the monotonic history decides — confirmed, unknown
// or failed. A foreign kind word (impossible through the guard and the
// table CHECK) fails safely to unknown — the confirmed state requires
// the exact evidence word, never a default.
func DeriveReplyDeliveryState(dispatched bool, events []ReplyDeliveryEvent) ReplyDeliveryState {
	if !dispatched {
		return ReplyDeliveryState{State: ReplyDeliveryPrepared}
	}
	if len(events) == 0 {
		return ReplyDeliveryState{State: ReplyDeliveryUnknown}
	}
	var last *ReplyDeliveryEvent
	for i := range events {
		if last == nil || events[i].EventNo >= last.EventNo {
			last = &events[i]
		}
	}
	state := ReplyDeliveryState{State: ReplyDeliveryUnknown, LastEventNo: last.EventNo, LastKind: last.Kind, LastEventAt: last.CreatedAt}
	switch last.Kind {
	case DeliveryEvidenceDeliveredConfirmed:
		state.State = ReplyDeliveryConfirmed
	case DeliveryEvidenceProviderFailed:
		state.State = ReplyDeliveryFailed
	}
	return state
}

// ---------------------------------------------------------------------------
// The record guard (the domain truth before SQL)
// ---------------------------------------------------------------------------

// DeliveryEvidenceForeignText is the fixed refusal text of an evidence
// word outside the dictionary: the machine relays the connector
// observations only, an invented outcome never passes.
const DeliveryEvidenceForeignText = "Слово наблюдения доставки не входит в словарь (%s): передайте наблюдение коннектора — подтверждена, неизвестна или отказ."

// GuardDeliveryEvidence validates the material of one evidence record:
// the kind belongs to the fixed dictionary. The event number is the
// server truth (the store computes the next monotonic number under the
// locks — the request never carries it).
func GuardDeliveryEvidence(kind string) error {
	if !DeliveryEvidenceInDictionary(kind) {
		return fmt.Errorf(DeliveryEvidenceForeignText, joinQuoted(DeliveryEvidenceWords))
	}
	return nil
}

// ---------------------------------------------------------------------------
// The outbox payload (the safe canonical JSON — the reply content never
// leaves the row)
// ---------------------------------------------------------------------------

// Outbox event types of the delivery evidence surface (the 2.1 bounded
// vocabulary).
const (
	OutboxEventClientReplyDeliveryEvidence   = "case.client_reply_delivery_evidence"
	OutboxEventClientReplyDeliveryEvidenceV1 = 1
)

// ClientReplyDeliveryEvidencePayload is the safe canonical JSON of the
// evidence event: the identifiers, the reply reference, the monotonic
// number and the dictionary kind word — the reply CONTENT never enters
// the envelope (the 2.6 candidate precedent).
type ClientReplyDeliveryEvidencePayload struct {
	TenantID   id.UUID   `json:"tenant_id"`
	CaseID     id.UUID   `json:"case_id"`
	ReplyID    id.UUID   `json:"reply_id"`
	EventID    id.UUID   `json:"event_id"`
	EventNo    int64     `json:"event_no"`
	Kind       string    `json:"kind"`
	ActorID    id.UUID   `json:"actor_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// DedupKeyClientReplyDeliveryEvidence builds the dedup key of one
// evidence event.
func DedupKeyClientReplyDeliveryEvidence(eventID id.UUID) string {
	return "case.client_reply_delivery_evidence:" + eventID.String()
}

// ClientReplyDeliveryEvidencePayloadJSON renders the safe canonical
// JSON.
func ClientReplyDeliveryEvidencePayloadJSON(payload ClientReplyDeliveryEvidencePayload) ([]byte, error) {
	return json.Marshal(payload)
}
