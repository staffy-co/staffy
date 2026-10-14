package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.11: the accommodation record command and the AccommodationView
// projection
// ---------------------------------------------------------------------------

// RecordAccommodationCommand records one revision of one communication
// preference of a case: the operator with the review OR the configuration
// scope (the gate fires BEFORE the material guards — the M1 precedent of
// 2.5..2.8), the case anchor, the preference key word (the fixed Domain
// Pack v0 dictionary), the valued material of the valued keys (the channel
// word of preferred_channel or the free availability line — an OPERATIONAL
// row, never a fact), the clearing mark (its own revision — a clearing
// never carries a value) and the operator note. The revision number is the
// server-side truth: the store derives the next monotonic number of the
// key under the locks (the request never carries it); the repeat of the
// same key is a NEW revision (the append-only history — «редактируемые»
// is the read-time derivation of the current set).
type RecordAccommodationCommand struct {
	Actor              domain.Principal
	CorrelationID      id.UUID
	TenantID           id.UUID
	CaseID             id.UUID
	AccommodationKey   string
	AccommodationValue string
	Cleared            bool
	Note               string
	IdempotencyKey     string
	Now                time.Time
}

// AccommodationStateView is one key of the CURRENT set of the case: the
// key word, the latest revision number, the clearing mark (a cleared key
// stays readable in the state — the honest revision truth) and the valued
// material (the channel word or the availability line; empty for the
// flags). The current set is the read-time derivation — never stored.
type AccommodationStateView struct {
	Key      string `json:"key"`
	Revision int64  `json:"revision"`
	Cleared  bool   `json:"cleared"`
	Value    string `json:"value,omitempty"`
}

// AccommodationRevisionView is one append-only revision row of the honest
// history: the key word, the monotonic revision, the valued material, the
// clearing mark, the operator note and the actor with the time. The
// history is never rewritten — a new value or a clearing is a new row.
type AccommodationRevisionView struct {
	Key        string    `json:"key"`
	Revision   int64     `json:"revision"`
	Value      string    `json:"value,omitempty"`
	Cleared    bool      `json:"cleared"`
	Note       string    `json:"note,omitempty"`
	ActorID    id.UUID   `json:"actor_id"`
	RecordedAt time.Time `json:"recorded_at"`
}

// AccommodationView is the «Предпочтения связи» section of the case view:
// the CURRENT set (the read-time derivation of the latest revision of
// every key — the cleared keys stay visible with the honest mark), the
// full append-only revision history, the key dictionary of the record
// form, the channel dictionary of the preferred_channel value and the
// fixed panel words. The preferences are an OPERATIONAL setting of the
// service: never facts, never a diagnosis, never an identity proof.
type AccommodationView struct {
	Current      []AccommodationStateView    `json:"current"`
	History      []AccommodationRevisionView `json:"history"`
	KeyWords     []string                    `json:"key_words"`
	ChannelWords []string                    `json:"channel_words"`
	RecordsCount int                         `json:"records_count"`
	PanelText    string                      `json:"panel_text"`
}

// AccommodationService is the command surface of Story 2.11 (the record
// command is idempotent by the tenant-scoped key with the stored outcome;
// the revision number is the server-side truth under the locks; the
// history is append-only; the current set is the read-time derivation —
// never stored).
type AccommodationService interface {
	RecordAccommodation(context.Context, RecordAccommodationCommand) (WorkOutcome, error)
}
