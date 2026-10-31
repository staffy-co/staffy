package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The truthful waiting updates (Story 2.16, first slice)
// ---------------------------------------------------------------------------
//
// An update promise is one append-only OPERATOR act over the handoff
// round: the operator promises the NEXT client update with a deadline
// WORD of the fixed dictionary. The honest unknown word exists for the
// case where no confirmed ETA exists (FR-41: the absence of a confirmed
// ETA is named directly — never replaced by a fabricated number): the
// material honesty is the CHECK pair unknown ↔ NULL, the unknown word
// never carries a promised time, a known word never lives without the
// computed server time. The promised time and the promise number are
// the SERVER truths (computed under the locks — never request fields).
//
// The promise NEVER replaces the reply itself: the update is sent by
// the HUMAN through the existing reply path (2.4) — the machine stores
// the promise and shows the truth (the «AI proposes, the human acts»
// precedent of 2.6). No timers, no automatic sends, no escalation
// automation (the honest Never of the slice — the producer is
// external).
//
// The client waiting status is the read-time derivation of the round
// state in THREE honest distinctions (searching / assigned-not-yet-
// accepted / accepted) plus the honest overdue text: the client text
// carries ONLY the dictionary words — the operator names, the epochs
// and the identifiers NEVER leak into it (the anti-disclosure of
// AC-1).
//
// The missed promise is a READ-TIME derivation over the append-only row
// (now > promised_at while the round lives): the row and the round
// NEVER mutate by the lateness, and the promise NEVER resolves the
// round — the lifecycle transitions belong to the existing 2.13
// commands with their own guards (AC-3: the handoff is not translated
// into resolved by a promise). A new promise supersedes the former
// DERIVATION-ONLY (the last row speaks — the 2.15 precedent): both
// rows live, the silent rewrite is structurally impossible.

// ---------------------------------------------------------------------------
// The deadline-word dictionary (the approved update policy v0)
// ---------------------------------------------------------------------------

// The update deadline words: the fixed dictionary of the promise. A
// deadline is a WORD with a fixed honest text and a fixed window — the
// machine gates the word, never parses the text and never breeds a
// deadline of its own (the «дедлайн не выдумывается» honesty of 2.4 —
// the ack-deadline precedent). update_unknown is the MATERIAL honesty
// of FR-41: the promise without a confirmed ETA — the word carries NO
// timestamp (the CHECK pair unknown ↔ NULL of the store), the honest
// text names the unknown directly.
const (
	UpdateDeadline15m     = "update_15m"
	UpdateDeadline1h      = "update_1h"
	UpdateDeadline4h      = "update_4h"
	UpdateDeadlineUnknown = "update_unknown"
)

// UpdateDeadlines is the full dictionary (the canonical order).
var UpdateDeadlines = []string{UpdateDeadline15m, UpdateDeadline1h, UpdateDeadline4h, UpdateDeadlineUnknown}

// UpdateDeadlineTexts is the honest text of every deadline word (the
// render material of the promise line and the form; the texts never
// enter the database or the outbox).
var UpdateDeadlineTexts = map[string]string{
	UpdateDeadline15m:     "Обновление обещано: в течение 15 минут.",
	UpdateDeadline1h:      "Обновление обещано: в течение часа.",
	UpdateDeadline4h:      "Обновление обещано: в течение четырёх часов.",
	UpdateDeadlineUnknown: "Срок следующего обновления неизвестен: обещание без выдуманного срока.",
}

// UpdateDeadlineText resolves the honest text of one deadline word.
func UpdateDeadlineText(word string) string {
	return UpdateDeadlineTexts[word]
}

// UpdateDeadlineInDictionary reports whether the deadline word belongs
// to the fixed dictionary (the promise write is fail-closed on it).
func UpdateDeadlineInDictionary(word string) bool {
	return inDictionary(UpdateDeadlines, word)
}

// UpdateDeadlineWindow resolves the time window of one deadline word
// (the zero duration of the unknown word and of a foreign word — the
// guards refuse the word before the window is ever read).
func UpdateDeadlineWindow(word string) time.Duration {
	switch word {
	case UpdateDeadline15m:
		return 15 * time.Minute
	case UpdateDeadline1h:
		return time.Hour
	case UpdateDeadline4h:
		return 4 * time.Hour
	}
	return 0
}

// UpdatePromiseDeadline resolves the promised time of one deadline
// word: the known words compute now + window (the server truth under
// the locks); the honest unknown word returns the ZERO time — the
// promise row stores NULL (the material CHECK pair unknown ↔ NULL; a
// fabricated timestamp NEVER accompanies the unknown word).
func UpdatePromiseDeadline(word string, now time.Time) time.Time {
	window := UpdateDeadlineWindow(word)
	if window == 0 {
		return time.Time{}
	}
	return now.Add(window)
}

// ---------------------------------------------------------------------------
// The promise state dictionary (the read-time words of the moment)
// ---------------------------------------------------------------------------

// The update promise states: the DERIVED words of the moment. pending —
// the last promise deadline is ahead; missed — the deadline passed
// while the round lives (the read-time lateness, never a stored status,
// never a mutation); unknown — the last promise carries the honest
// no-deadline word; superseded — a former promise replaced by a later
// row of the same round (the derivation-only supersede); closed — the
// round itself closed (the cancelled or returned request: the waiting
// ended, the history stays honest, never a silent disappearance).
const (
	UpdatePromisePending    = "pending"
	UpdatePromiseMissed     = "missed"
	UpdatePromiseUnknown    = "unknown"
	UpdatePromiseSuperseded = "superseded"
	UpdatePromiseClosed     = "closed"
)

// UpdatePromiseStates is the full dictionary.
var UpdatePromiseStates = []string{
	UpdatePromisePending, UpdatePromiseMissed, UpdatePromiseUnknown,
	UpdatePromiseSuperseded, UpdatePromiseClosed,
}

// UpdatePromiseStateInDictionary reports whether the state word belongs
// to the fixed dictionary.
func UpdatePromiseStateInDictionary(word string) bool {
	return inDictionary(UpdatePromiseStates, word)
}

// UpdatePromiseStateTexts is the honest text of every state word (the
// render material of the promise line; the texts never enter the
// database or the outbox).
var UpdatePromiseStateTexts = map[string]string{
	UpdatePromisePending:    "Ожидаем обновления к обещанному сроку.",
	UpdatePromiseMissed:     "Обновление просрочено — решения ещё нет.",
	UpdatePromiseUnknown:    "Обновление ожидается.",
	UpdatePromiseSuperseded: "Обещание заменено новым.",
	UpdatePromiseClosed:     "Обещание закрыто вместе с запросом человека.",
}

// UpdatePromiseStateText resolves the honest text of one state word.
func UpdatePromiseStateText(word string) string {
	return UpdatePromiseStateTexts[word]
}

// UpdatePromiseMissedNextStepText is the honest next step of the missed
// promise: the update itself is the EXTERNAL producer — the human sends
// it by the EXISTING reply path of 2.4 (send_client_reply); the machine
// never breeds a timer, an automatic send or an escalation (the honest
// Never of the slice).
const UpdatePromiseMissedNextStepText = "Следующий шаг: сообщите статус существующим ответом клиенту."

// ---------------------------------------------------------------------------
// The client waiting status dictionary (AC-1: the three distinctions)
// ---------------------------------------------------------------------------

// The client waiting statuses of the interface truth: the honest answer
// to «what is happening while I wait for the human». waiting_searching
// — the human is not connected yet (the fresh ask, the refused round:
// the search continues); waiting_assigned — a specialist is named and
// has NOT accepted yet («назначен ≠ принял» — two honest words, never
// one); waiting_accepted — the specialist took the case (the third
// distinction); waiting_overdue — the acceptance is late (the honest
// lateness of the 2.13 overdue derivation, in the client's own words);
// waiting_cancelled and waiting_returned — the waiting ended. The
// client text carries ONLY these dictionary words: the operator names,
// the epochs and the identifiers NEVER enter it (the anti-disclosure
// of AC-1).
const (
	ClientWaitingSearching = "waiting_searching"
	ClientWaitingAssigned  = "waiting_assigned"
	ClientWaitingAccepted  = "waiting_accepted"
	ClientWaitingOverdue   = "waiting_overdue"
	ClientWaitingCancelled = "waiting_cancelled"
	ClientWaitingReturned  = "waiting_returned"
)

// ClientWaitingStatuses is the full dictionary.
var ClientWaitingStatuses = []string{
	ClientWaitingSearching, ClientWaitingAssigned, ClientWaitingAccepted,
	ClientWaitingOverdue, ClientWaitingCancelled, ClientWaitingReturned,
}

// ClientWaitingStatusTexts is the honest text of every waiting status
// word (the render material of the «Ожидание человека» block of the
// case page; the texts never enter the database or the outbox).
var ClientWaitingStatusTexts = map[string]string{
	ClientWaitingSearching: "Ищем специалиста.",
	ClientWaitingAssigned:  "Специалист назначен: ожидаем принятия обращения.",
	ClientWaitingAccepted:  "Специалист принял обращение.",
	ClientWaitingOverdue:   "Ожидание принятия затянулось: специалист ещё не принял обращение.",
	ClientWaitingCancelled: "Запрос человека отменён: ожидание завершено.",
	ClientWaitingReturned:  "Обслуживание человеком завершено: обращение вернулось к цифровому сотруднику.",
}

// ClientWaitingStatusText resolves the honest text of one waiting
// status word.
func ClientWaitingStatusText(word string) string {
	return ClientWaitingStatusTexts[word]
}

// ClientWaitingStatusInDictionary reports whether the waiting status
// word belongs to the fixed dictionary.
func ClientWaitingStatusInDictionary(word string) bool {
	return inDictionary(ClientWaitingStatuses, word)
}

// DeriveClientWaitingStatus derives the honest client waiting status
// word of the derived handoff round state (the read-time derivation of
// the moment — the same material the operator card sees, in the
// client's own dictionary): requested and declined stay searching (the
// human is not connected yet — the refused round keeps the search
// honest); assigned is the named-but-not-accepted wait; overdue names
// the honest lateness of the acceptance; acknowledged is the accepted
// case; cancelled and returned end the waiting. The derivation takes
// the STATE WORD only — the operator names, the epochs and the
// identifiers structurally never enter the client text (AC-1).
func DeriveClientWaitingStatus(state string) string {
	switch state {
	case HandoffStateAssigned:
		return ClientWaitingAssigned
	case HandoffStateOverdue:
		return ClientWaitingOverdue
	case HandoffStateAcknowledged:
		return ClientWaitingAccepted
	case HandoffStateCancelled:
		return ClientWaitingCancelled
	case HandoffStateReturned:
		return ClientWaitingReturned
	default: // requested, declined — the human is not connected yet
		return ClientWaitingSearching
	}
}

// ---------------------------------------------------------------------------
// The promise row (append-only, never updated)
// ---------------------------------------------------------------------------

// UpdatePromiseRecord is one append-only promise row of one handoff
// round: the tenant and the case, the monotonic promise number (the
// store computes it under the locks — the requests never carry it),
// the request anchor (the human handoff round of THIS case — the store
// guards the belonging and the open lifecycle), the deadline word, the
// honest promised time (the known word — the computed server truth;
// the unknown word — the ZERO time, stored NULL: the material CHECK
// pair unknown ↔ NULL), the responsible operator (the actor of the
// command — AC-2 «ответственный за обновление» by the living material)
// and the three times of the 2.2 discipline. The row never mutates; a
// new promise is a new row — the former rows derive superseded.
type UpdatePromiseRecord struct {
	TenantID     id.UUID
	CaseID       id.UUID
	PromiseID    id.UUID
	PromiseNo    int64
	RequestNo    int64
	DeadlineWord string
	PromisedAt   time.Time // zero (NULL) when the word is update_unknown
	ActorID      id.UUID
	ObservedAt   time.Time
	ReceivedAt   time.Time
	CreatedAt    time.Time
}

// ---------------------------------------------------------------------------
// The write guards (the 400 refusal dictionary)
// ---------------------------------------------------------------------------

// GuardPromiseUpdate validates the promise write: the deadline word
// belongs to the fixed dictionary (the honest unknown word included —
// a fabricated deadline NEVER passes; a request carrying a promised
// time or a promise number refuses at the strict decode of the
// transport — those are the server truths, never request fields). The
// round anchor (the open lifecycle of THIS case) is the STORE guard —
// the domain never sees the round here.
func GuardPromiseUpdate(deadlineWord string) error {
	if !UpdateDeadlineInDictionary(deadlineWord) {
		return fmt.Errorf("слово дедлайна обновления «%s» не входит в словарь (%s)", deadlineWord, joinQuoted(UpdateDeadlines))
	}
	return nil
}

// GuardUpdatePromiseTimes holds the 2.2 time discipline: the observed
// time is mandatory and never claims more than the 48h provider skew
// ahead of the receive time; the created time is the server truth
// elsewhere.
func GuardUpdatePromiseTimes(observedAt, receivedAt time.Time) error {
	if observedAt.IsZero() {
		return fmt.Errorf("время наблюдения обещания обязательно")
	}
	if receivedAt.IsZero() {
		return fmt.Errorf("время получения обещания обязательно")
	}
	if observedAt.After(receivedAt.Add(maxProviderClockSkew)) {
		return fmt.Errorf("время наблюдения слишком далеко в будущем относительно времени получения")
	}
	return nil
}

// updatePromiseRoundStates are the round states that accept the update
// promise: the OPEN waiting of the client — the fresh ask, the named
// round (ahead or late), and the ACCEPTED case (the human works — the
// updates are still promised). The declined round refuses: the
// assignment round is consumed, the re-assignment decision belongs to
// the human — a promise over the consumed round would name nobody. The
// closed rounds (cancelled, returned) refuse honestly: the waiting
// ended. The store guards the round with its own mirror words.
var updatePromiseRoundStates = map[string]bool{
	HandoffStateRequested:    true,
	HandoffStateAssigned:     true,
	HandoffStateOverdue:      true,
	HandoffStateAcknowledged: true,
}

// UpdatePromiseRoundAccepts reports whether the derived round state
// accepts the update promise (the honest anchor set of the slice: the
// store refuses the declined and closed rounds by the mirror words of
// its own guard).
func UpdatePromiseRoundAccepts(state string) bool {
	return updatePromiseRoundStates[state]
}

// ---------------------------------------------------------------------------
// The promise state derivation (read-time, never stored)
// ---------------------------------------------------------------------------

// UpdatePromiseStateView is the derived promise state of one round:
// the LAST promise (the store passes the rows ordered by the promise
// number) with its word, number, time, actor and the derived state
// word — the material of the promise line. No promises is the zero
// view (the line appears only with the material — the control branch
// renders byte-identically). The actor is the OPERATOR material (the
// responsible of AC-2) — it never enters the client text.
type UpdatePromiseStateView struct {
	HasPromise   bool
	PromiseNo    int64
	DeadlineWord string
	PromisedAt   time.Time // zero (NULL) when the word is update_unknown
	State        string
	ActorID      id.UUID
	CreatedAt    time.Time
}

// DeriveUpdatePromiseState derives the read-time state of one round
// from its promise rows (ordered by the promise number — the same
// store trust as the correction state derivation): the LAST promise
// speaks. The derivation never mutates the rows: the same history
// derives pending before the deadline and missed after it — the truth
// of the moment, never a stored status.
func DeriveUpdatePromiseState(promises []UpdatePromiseRecord, roundState string, now time.Time) UpdatePromiseStateView {
	if len(promises) == 0 {
		return UpdatePromiseStateView{}
	}
	last := promises[len(promises)-1]
	return UpdatePromiseStateView{
		HasPromise:   true,
		PromiseNo:    last.PromiseNo,
		DeadlineWord: last.DeadlineWord,
		PromisedAt:   last.PromisedAt,
		State:        DeriveUpdatePromiseRowState(true, last.DeadlineWord, last.PromisedAt, roundState, now),
		ActorID:      last.ActorID,
		CreatedAt:    last.CreatedAt,
	}
}

// DeriveUpdatePromiseRowState derives the honest read-time state word
// of one promise row of the round: a former row (isLast=false) is
// always superseded — the later promise of the same round replaced it
// (the derivation-only supersede of the 2.15 precedent: both rows
// live, the history is never rewritten); the last row derives by the
// word, the time and the round state — the closed round (cancelled or
// returned) closes the promise honestly; the unknown word carries no
// deadline to miss; the known word is missed when now is past the
// promised time while the round lives (the declined round keeps the
// promise ticking — the update is still owed to the client by whoever
// continues the case).
func DeriveUpdatePromiseRowState(isLast bool, deadlineWord string, promisedAt time.Time, roundState string, now time.Time) string {
	if !isLast {
		return UpdatePromiseSuperseded
	}
	if roundState == HandoffStateCancelled || roundState == HandoffStateReturned {
		return UpdatePromiseClosed
	}
	if deadlineWord == UpdateDeadlineUnknown {
		return UpdatePromiseUnknown
	}
	if !promisedAt.IsZero() && now.After(promisedAt) {
		return UpdatePromiseMissed
	}
	return UpdatePromisePending
}

// ClientPromiseText renders the honest CLIENT line of the last promise
// of the round: the deadline word text with the state text; the missed
// state appends the honest next step (the human sends the update by
// the existing reply path — never a machine timer). The responsible
// actor NEVER enters the client line (the anti-disclosure of AC-1 —
// the operator names, the epochs and the identifiers are absent from
// every text of this composition); no promises is the empty string
// (the line appears only with the material).
func (v UpdatePromiseStateView) ClientPromiseText() string {
	if !v.HasPromise {
		return ""
	}
	text := UpdateDeadlineText(v.DeadlineWord)
	if state := UpdatePromiseStateText(v.State); state != "" {
		text += " " + state
	}
	if v.State == UpdatePromiseMissed {
		text += " " + UpdatePromiseMissedNextStepText
	}
	return text
}

// ---------------------------------------------------------------------------
// The outbox perimeter (the identifiers and the dictionary words only)
// ---------------------------------------------------------------------------

// Outbox event types of the waiting update surface (the bounded
// vocabulary of the slice): the one promise word.
const (
	OutboxEventUpdatePromised   = "case.update_promised"
	OutboxEventUpdatePromisedV1 = 1
)

// UpdatePromisedPayload is the safe canonical JSON of the promise
// event: the identifiers (the tenant, the case, the request round, the
// promise with its number, the actor) and the dictionary word (the
// deadline word). The client texts and any person material NEVER enter
// the envelope — the perimeter of the outbox stays
// identifiers-and-words (the «кандидат не покидает строку» precedent
// of 2.6/2.13/2.15).
type UpdatePromisedPayload struct {
	TenantID     id.UUID   `json:"tenant_id"`
	CaseID       id.UUID   `json:"case_id"`
	RequestNo    int64     `json:"request_no"`
	PromiseID    id.UUID   `json:"promise_id"`
	PromiseNo    int64     `json:"promise_no"`
	DeadlineWord string    `json:"deadline_word"`
	ActorID      id.UUID   `json:"actor_id"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// DedupKeyUpdatePromised builds the dedup key of one promise event.
func DedupKeyUpdatePromised(caseID id.UUID, requestNo, promiseNo int64) string {
	return fmt.Sprintf("case.handoff_update_promise:%s:%d:%d", caseID.String(), requestNo, promiseNo)
}

// UpdatePromisedPayloadJSON renders the safe canonical JSON.
func UpdatePromisedPayloadJSON(payload UpdatePromisedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
