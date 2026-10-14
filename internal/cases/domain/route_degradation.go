package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The degraded route slots and the honest update promise (Story 2.12,
// first slice): the dictionaries, the read-time derivation, the record
// guards and the outbox perimeter
// ---------------------------------------------------------------------------
//
// A route degradation is an OPERATIONAL record of the service, never a
// fact of the case: the words of the channel world («timeout, disconnect,
// rate limit, revoked capability, неоднозначный delivery outcome») become
// append-only revision rows scoped to the ROUTE — the (connection,
// channel) pair of one of the case's conversations — a table that lives
// OUTSIDE the fact channels: no confirmation statuses, no fact versions,
// no facts digest, no identity proof (the «деградация не факт» perimeter
// of AC-1; the preferences precedent of 2.11). A route is degraded when
// the LATEST revision of its key is not a clearing one; a clearing
// revision restores the route — the honest «восстановление» row of the
// history. The derivation is per-route by the exact key: an independent
// route (another connection or another channel) is NEVER declared
// degraded without its own row («независимые маршруты не объявляются
// недоступными без доказательства»).
//
// The permitted continuation paths of the degraded mode are the
// machine's honest offer: a commercial intent on a degraded route never
// issues the commercial answer — the permitted outcome words travel with
// the Policy Outcome (the class degraded_mode of Story 2.12) and the
// panel renders the fixed honest text of every word. The dictionary is
// compile-time («модель не может самостоятельно расширить этот набор»);
// the texts never enter the database or the outbox.
//
// The update promise is the honest timing of the reply: a message sent
// on a degraded route MUST promise the time of the next update with one
// word of the approved policy dictionary — a word outside the dictionary
// is structurally impossible («время следующего обновления только в
// пределах утверждённой policy»); on a healthy route the field stays
// empty (exactly one meaning). The promise words carry their honest
// texts for the render; the machine never parses them into a schedule
// and never breeds a deadline from a promise.

// ---------------------------------------------------------------------------
// The cause dictionary (AC-1 words of the channel world)
// ---------------------------------------------------------------------------

// The route degradation causes: the fixed dictionary of the slice (the
// AC-1 words: timeout, disconnect, rate limit, revoked capability, an
// ambiguous delivery outcome).
const (
	DegradationCauseTimeout           = "timeout"
	DegradationCauseDisconnect        = "disconnect"
	DegradationCauseRateLimited       = "rate_limited"
	DegradationCauseCapabilityRevoked = "capability_revoked"
	DegradationCauseDeliveryUnknown   = "delivery_unknown"
)

// DegradationCauses is the full dictionary (the canonical order).
var DegradationCauses = []string{
	DegradationCauseTimeout, DegradationCauseDisconnect, DegradationCauseRateLimited,
	DegradationCauseCapabilityRevoked, DegradationCauseDeliveryUnknown,
}

// DegradationCauseTexts is the fixed honest text of every cause word (the
// Domain Pack v0 render material of the «Состояние маршрутов» section; the
// texts never enter the database or the outbox).
var DegradationCauseTexts = map[string]string{
	DegradationCauseTimeout:           "Таймаут маршрута: канал не ответил вовремя.",
	DegradationCauseDisconnect:        "Обрыв связи: подключение канала потеряно.",
	DegradationCauseRateLimited:       "Предел частоты: канал временно ограничивает отправку.",
	DegradationCauseCapabilityRevoked: "Возможность отозвана: право канала на действие отозвано.",
	DegradationCauseDeliveryUnknown:   "Неоднозначный результат доставки: квитанции нет — итог неизвестен.",
}

// DegradationCauseText resolves the honest text of one cause word (the
// empty string of an unknown word — the render layer only).
func DegradationCauseText(cause string) string {
	return DegradationCauseTexts[cause]
}

// DegradationCauseInDictionary reports whether the cause word belongs to
// the fixed dictionary (the fail-closed basis of the record guard).
func DegradationCauseInDictionary(cause string) bool {
	return inDictionary(DegradationCauses, cause)
}

// ---------------------------------------------------------------------------
// The permitted path dictionary of the degraded mode (AC-3 words)
// ---------------------------------------------------------------------------

// The degraded mode permitted paths: the five AC-3 words — принять
// информацию без обещания, переключить на проверенный канал, запросить
// Human Handoff, согласовать callback, остановить путь. Every word
// carries its fixed honest text (the panel renders it; the database and
// the outbox carry the words only). The dictionary is NOT filtered by
// capability — the registry of verified channels does not exist (the
// active_alternatives precedent of 2.11: v1 offers the pack dictionary,
// the operator decides).
const (
	DegradedPathAcceptInfoNoPromise   = "accept_info_no_promise"
	DegradedPathVerifiedChannelSwitch = "verified_channel_switch"
	DegradedPathHumanHandoff          = "human_handoff"
	DegradedPathAgreeCallback         = "agree_callback"
	DegradedPathStopPath              = "stop_path"
)

// DegradedPermittedPathWords is the full dictionary of the pack v0 (the
// policy attaches the whole list — the subset belongs to the future
// capability registry).
var DegradedPermittedPathWords = []string{
	DegradedPathAcceptInfoNoPromise, DegradedPathVerifiedChannelSwitch, DegradedPathHumanHandoff,
	DegradedPathAgreeCallback, DegradedPathStopPath,
}

// DegradedPathTexts is the fixed honest text of every path word (the
// Domain Pack v0 render material of the degraded outcome panel).
var DegradedPathTexts = map[string]string{
	DegradedPathAcceptInfoNoPromise:   "Информация без обещания: сообщу только то, что известно, — без обещаний цены, срока и результата.",
	DegradedPathVerifiedChannelSwitch: "Проверенный канал: продолжим на другом подключённом канале, история обращения сохранена.",
	DegradedPathHumanHandoff:          "Передача человеку: сотрудник примет обращение и доведёт его до результата.",
	DegradedPathAgreeCallback:         "Согласованный обратный звонок: сотрудник сам свяжется в обещанное окно.",
	DegradedPathStopPath:              "Останов пути: обращение закрывается без давления, возобновить можно в любой момент.",
}

// DegradedPathText resolves the honest text of one path word (the empty
// string of an unknown word — the render layer only).
func DegradedPathText(word string) string {
	return DegradedPathTexts[word]
}

// DegradedPathInDictionary reports whether the path word belongs to the
// fixed dictionary.
func DegradedPathInDictionary(word string) bool {
	return inDictionary(DegradedPermittedPathWords, word)
}

// ---------------------------------------------------------------------------
// The update promise dictionary (AC-4 words — the approved policy v0)
// ---------------------------------------------------------------------------

// The update promise words: the approved policy dictionary of the next
// update timing. A promise is a WORD with a fixed honest text — the
// machine gates the word, never parses the text into a schedule and
// never breeds a deadline (the deadline contour of AC-6 is a future
// slice).
const (
	UpdatePromiseNoPromise       = "no_promise"
	UpdatePromiseWithin1h        = "within_1h"
	UpdatePromiseWithin4h        = "within_4h"
	UpdatePromiseNextBusinessDay = "within_next_business_day"
)

// UpdatePromiseWords is the full dictionary (the canonical order).
var UpdatePromiseWords = []string{
	UpdatePromiseNoPromise, UpdatePromiseWithin1h, UpdatePromiseWithin4h, UpdatePromiseNextBusinessDay,
}

// UpdatePromiseTexts is the fixed honest text of every promise word (the
// render material of the reply panel column and the form select).
var UpdatePromiseTexts = map[string]string{
	UpdatePromiseNoPromise:       "Время следующего обновления не называю.",
	UpdatePromiseWithin1h:        "Следующее обновление — в течение часа.",
	UpdatePromiseWithin4h:        "Следующее обновление — в течение четырёх часов.",
	UpdatePromiseNextBusinessDay: "Следующее обновление — в следующий рабочий день.",
}

// UpdatePromiseText resolves the honest text of one promise word (the
// empty string of an unknown word — the render layer only).
func UpdatePromiseText(word string) string {
	return UpdatePromiseTexts[word]
}

// UpdatePromiseInDictionary reports whether the promise word belongs to
// the fixed dictionary (the structural honesty of the timing gate).
func UpdatePromiseInDictionary(word string) bool {
	return inDictionary(UpdatePromiseWords, word)
}

// ---------------------------------------------------------------------------
// The fixed material bound (the domain guard carries it, the table CHECK
// mirrors it)
// ---------------------------------------------------------------------------

// MaxDegradationNoteRunes is the rune bound of the operator note of one
// degradation record (the 2.11 note precedent).
const MaxDegradationNoteRunes = 500

// ---------------------------------------------------------------------------
// The route key (the slot scope — the independent routes stay
// independent)
// ---------------------------------------------------------------------------

// RouteKey is the identity of one route: the connection and the channel
// (the 2.1 channel dictionary word). The degradation slot is scoped to
// this pair — another connection or another channel is ANOTHER route and
// is never touched by the derivation of this one.
type RouteKey struct {
	ConnectionID id.UUID
	Channel      string
}

// String renders the canonical map key of the route: the UUID string and
// the channel word joined by a colon (the canonical form of the snapshot
// map and the derivation map).
func (r RouteKey) String() string {
	return r.ConnectionID.String() + ":" + r.Channel
}

// ---------------------------------------------------------------------------
// The row shape (the store reads/writes; the current slots are derived,
// never stored)
// ---------------------------------------------------------------------------

// RouteDegradationRecord is one append-only revision row of one route of
// one case: the route key, the monotonic revision number, the cause
// word, the clearing mark, the operator note and the actor. The history
// is never rewritten — a new cause or a recovery is a NEW revision; the
// CURRENT slot of the route is the read-time derivation of the latest
// revision.
type RouteDegradationRecord struct {
	TenantID   id.UUID
	CaseID     id.UUID
	RecordID   id.UUID
	Route      RouteKey
	Cause      string
	Revision   int64
	Cleared    bool
	Note       string
	ActorID    id.UUID
	RecordedAt time.Time
}

// ---------------------------------------------------------------------------
// The read-time derivation (the current slots, never stored)
// ---------------------------------------------------------------------------

// DegradationState is the read-time state of one route: the latest
// revision number, its cause word and its clearing mark. A CLEARED state
// stays in the map (the honest revision truth of the snapshot — a
// recovery changes the digest); the DEGRADED set is the routes whose
// latest revision is not a clearing one.
type DegradationState struct {
	Route    RouteKey
	Cause    string
	Revision int64
	Cleared  bool
}

// CurrentDegradations is the derived route state of one case: every
// route with at least one recorded revision carries the state of its
// LATEST revision (the read-time truth of the moment — never a stored
// status).
type CurrentDegradations map[string]DegradationState

// DeriveCurrentDegradations derives the read-time route state: the
// latest revision of every route key wins (the append-only history is
// never rewritten); a clearing revision keeps the route in the state
// with the cleared mark and removes it from the degraded set. The LATER
// record of an equal revision wins the defensive tie (the table UNIQUE
// backstop makes the case unreachable — the derivation stays
// deterministic on any input order).
func DeriveCurrentDegradations(records []RouteDegradationRecord) CurrentDegradations {
	current := CurrentDegradations{}
	for _, record := range records {
		key := record.Route.String()
		state, known := current[key]
		if !known || record.Revision >= state.Revision {
			current[key] = DegradationState{
				Route: record.Route, Cause: record.Cause,
				Revision: record.Revision, Cleared: record.Cleared,
			}
		}
	}
	return current
}

// RouteDegraded reports whether the exact route is degraded: the latest
// revision exists and is not a clearing one. An unknown route (another
// connection or channel, or no records at all) is NOT degraded — the
// independent routes are never declared unavailable without a proof.
func (c CurrentDegradations) RouteDegraded(route RouteKey) bool {
	state, known := c[route.String()]
	return known && !state.Cleared
}

// RouteState resolves the derived state of one route (ok=false when no
// revision exists).
func (c CurrentDegradations) RouteState(route RouteKey) (DegradationState, bool) {
	state, known := c[route.String()]
	return state, known
}

// DegradedRoutes resolves the sorted route keys of the current degraded
// set (the canonical panel order of the «Состояние маршрутов» section).
func (c CurrentDegradations) DegradedRoutes() []string {
	routes := make([]string, 0, len(c))
	for key, state := range c {
		if !state.Cleared {
			routes = append(routes, key)
		}
	}
	sort.Strings(routes)
	return routes
}

// DegradationSnapshotMap is the canonical route state of the policy
// input snapshot: the route key → {cause, revision, cleared} map (the
// encoding order of the map marshal is canonical — the FactsDigestMap
// and AccommodationSnapshotMap precedent).
type DegradationSnapshotMap map[string]struct {
	Cause    string `json:"cause"`
	Revision int64  `json:"revision"`
	Cleared  bool   `json:"cleared"`
}

// DegradationSnapshotOf renders the canonical route map of the policy
// input snapshot. The map is ALWAYS initialized: the same logical state
// renders the same canonical JSON (the empty state is {} — the digest
// determinism; the clearing revisions stay in the map — the honest
// supersede of the recovered routes).
func DegradationSnapshotOf(c CurrentDegradations) DegradationSnapshotMap {
	snapshot := DegradationSnapshotMap{}
	for key, state := range c {
		snapshot[key] = struct {
			Cause    string `json:"cause"`
			Revision int64  `json:"revision"`
			Cleared  bool   `json:"cleared"`
		}{Cause: state.Cause, Revision: state.Revision, Cleared: state.Cleared}
	}
	return snapshot
}

// ---------------------------------------------------------------------------
// The record guard (the domain truth before SQL)
// ---------------------------------------------------------------------------

// GuardRouteDegradationRecord validates the material of one revision
// record: the route carries a valid connection and a word of the 2.1
// channel dictionary; the cause belongs to the fixed dictionary; the
// note respects its rune bound. The revision number is the server truth
// (the store computes the next monotonic revision under the locks — the
// request never carries it).
func GuardRouteDegradationRecord(route RouteKey, cause string, note string) error {
	if !route.ConnectionID.Valid() {
		return fmt.Errorf("поле connection_id обязательно (UUID): слот деградации живёт на подключении маршрута")
	}
	if !inDictionary(ChannelOrder, route.Channel) {
		return fmt.Errorf("канал «%s» не входит в словарь каналов (%s)", route.Channel, strings.Join(ChannelOrder, ", "))
	}
	if !DegradationCauseInDictionary(cause) {
		return fmt.Errorf("причина «%s» не входит в словарь (%s)", cause, joinQuoted(DegradationCauses))
	}
	if utf8.RuneCountInString(note) > MaxDegradationNoteRunes {
		return fmt.Errorf("заметка деградации длиннее %d рун", MaxDegradationNoteRunes)
	}
	return nil
}

// ---------------------------------------------------------------------------
// The honest timing gate of the reply (the classless 2.12 ladder word)
// ---------------------------------------------------------------------------

// UpdatePromiseRequiredText is the fixed refusal text of the reply on a
// degraded route without the update promise word: the honest timing is
// mandatory, never optional («время следующего обновления только в
// пределах утверждённой policy»).
const UpdatePromiseRequiredText = "Маршрут ответа деградирован: сообщение обязано обещать время следующего обновления одним словом утверждённой policy."

// UpdatePromiseOutsideText is the fixed refusal text of the promise word
// outside the approved dictionary: a foreign timing word never leaves
// the machine (the structural honesty of AC-4).
const UpdatePromiseOutsideText = "Слово обещания времени обновления не входит в утверждённую policy: время следующего обновления — только словом словаря."

// UpdatePromiseForeignText is the fixed refusal text of the reply with a
// promise on a healthy route: the field has exactly one meaning (the
// accuracy_qualifier_foreign mirror of 2.11 — the control branch stays
// byte-identical).
const UpdatePromiseForeignText = "Слово обещания времени обновления принадлежит ответу на деградированном маршруте: маршрут не деградирован — оставьте поле пустым."

// GuardUpdatePromise is the classless honest timing gate of Story 2.12
// (AFTER the 2.9 commitment link, BEFORE the CAS — the frozen ladder:
// ownership 2.4 → mode 2.5 → disclosure 2.5 → policy 2.6 → objection
// 2.8 → accuracy 2.11 → commitment link 2.9 → honest timing 2.12 → CAS;
// fires for EVERY reply class — a short failure message can be
// sales/neutral/protected): the reply route is degraded → the promise
// word is REQUIRED and must belong to the approved dictionary; the
// route is healthy → the field must stay EMPTY (the byte-identical old
// path).
func GuardUpdatePromise(degraded bool, promise string) error {
	if degraded {
		if promise == "" {
			return WorkRuleError{Rule: "update_promise_required", Reason: UpdatePromiseRequiredText + " Словарь: " + joinQuoted(UpdatePromiseWords) + "."}
		}
		if !UpdatePromiseInDictionary(promise) {
			return WorkRuleError{Rule: "update_promise_foreign", Reason: UpdatePromiseOutsideText + " Словарь: " + joinQuoted(UpdatePromiseWords) + "."}
		}
		return nil
	}
	if promise != "" {
		return WorkRuleError{Rule: "update_promise_foreign", Reason: UpdatePromiseForeignText}
	}
	return nil
}

// ---------------------------------------------------------------------------
// The outbox payload (the safe canonical JSON — the note never leaves
// the row)
// ---------------------------------------------------------------------------

// Outbox event types of the degradation surface (the 2.1 bounded
// vocabulary).
const (
	OutboxEventRouteDegradationRecorded   = "case.route_degradation_recorded"
	OutboxEventRouteDegradationRecordedV1 = 1
)

// RouteDegradationRecordedPayload is the safe canonical JSON of the
// record event: the identifiers, the route words, the cause word, the
// revision number and the clearing mark — the NOTE never enters the
// envelope (the operational rows stay in the table; the «кандидат не
// покидает строку» precedent of 2.6).
type RouteDegradationRecordedPayload struct {
	TenantID     id.UUID   `json:"tenant_id"`
	CaseID       id.UUID   `json:"case_id"`
	RecordID     id.UUID   `json:"record_id"`
	ConnectionID id.UUID   `json:"connection_id"`
	Channel      string    `json:"channel"`
	Cause        string    `json:"cause"`
	Revision     int64     `json:"revision"`
	Cleared      bool      `json:"cleared"`
	ActorID      id.UUID   `json:"actor_id"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// DedupKeyRouteDegradationRecorded builds the dedup key of one record
// event.
func DedupKeyRouteDegradationRecorded(recordID id.UUID) string {
	return "case.route_degradation_recorded:" + recordID.String()
}

// RouteDegradationRecordedPayloadJSON renders the safe canonical JSON.
func RouteDegradationRecordedPayloadJSON(payload RouteDegradationRecordedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
