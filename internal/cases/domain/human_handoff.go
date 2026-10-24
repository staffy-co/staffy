package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The unconditional human request and the lifecycle Handoff (Story 2.13,
// first slice): the dictionaries, the read-time derivation of the request
// lifecycle, the record guards, the conditional mode term and the freeze
// gate of the new commitments
// ---------------------------------------------------------------------------
//
// A human handoff is a LIFECYCLE RECORD of the case, never a fact and
// never a mode by itself: the client's unconditional ask («позовите
// человека») becomes one append-only request row anchored to the ACCEPTED
// client turn that asked for the human (the turn_ref discipline of the
// 2.5 signals — the anchor of the unconditional request, AC-1), plus the
// append-only event rows of the lifecycle transitions. The request is
// UNCONDITIONAL: the operator never types a reason — the reason word is
// the single dictionary constant client_request and the mode reason of
// the raise is the fixed system text (the honest «безусловность»: the
// ask does not need a repeated refusal or an operator justification).
// The mode raise is the existing 2.5 machinery (the ratchet ladder):
// human_requested is proposed with the turn_event signal source, and a
// STRICTER active mode is never touched — the ask still creates the
// handoff row (the ratchet honesty of AC-1: «режим не понижается,
// просьба безусловна»).
//
// The lifecycle is honest about the difference between the assignment
// and the acceptance: НАЗНАЧЕНИЕ ≠ ПРИНЯТИЕ is the structural truth of
// AC-3/AC-4/AC-11. The assigned event names the assignee, the backup
// owner and the acknowledgement deadline word; the owner NEVER changes
// on the assignment — only the acknowledged and returned transitions
// move the ownership (through the exec-composition of the 2.4 transfer
// machinery in ONE transaction — a call, never a copy). The overdue
// state is a READ-TIME derivation (the assigned round whose deadline
// has passed without a closing event — never a stored status, never a
// worker); the decline keeps the request open (the assignment round is
// consumed, the human re-assigns); the cancel of the client and the
// return to Staffy close the lifecycle. There is no failed word in this
// slice: a failed handoff is an honest decline or an overdue derivation
// — a word without a material event would be a lie.
//
// The client status is the four honest words of the interface truth:
// requested (специалист запрошен), assigned (назначен, ещё не принял),
// acknowledged (принял), cancelled (отменён) — «назначен ≠ принял» are
// TWO words, never one. The freeze gate of the new commitments is the
// AC-2 law «requested handoff freezes incompatible actions until the
// target acknowledges»: the effective human_requested mode OR an active
// handoff refuses the CreateCommitment command with the honest mode
// word — the old path without the mode and the handoff stays
// byte-identical (the control branch of 2.9).

// ---------------------------------------------------------------------------
// The reason dictionary (AC-1: the unconditional word of the client ask;
// Story 4.3 adds the three operator boundary words — a new dictionary
// version by the gateDictionary precedent of 4.1)
// ---------------------------------------------------------------------------

// The handoff request reasons: the dictionary of the first slice extended
// by the Story 4.3 boundary words. client_request stays the word of the
// UNCONDITIONAL client ask — the operator never types it (the system
// truth of 2.13); the operator transfer of 2.4 (the free reason ≤ 200
// runes) is a DIFFERENT surface and is never touched by this story. The
// three Story 4.3 words are the OPERATOR-produced reasons of the part
// request boundaries (the panel recommends, the operator decides): the
// producer never types a free text — the word is the dictionary constant
// (fail-closed on GuardHandoffRequestReason; the SQL mirror of 00019 is
// the backstop). The dictionary extension is the new spec version by the
// gateDictionary precedent of 4.1: the old word keeps its meaning, the
// new words are PARALLEL to the unconditional client ask (never its
// replacement — the client's ask needs no reason, ever).
const (
	HandoffReasonClientRequest           = "client_request"
	HandoffReasonDiagnosticUncertainty   = "diagnostic_uncertainty"
	HandoffReasonSafetyCritical          = "safety_critical"
	HandoffReasonUnresolvedContradiction = "unresolved_contradiction"
	// HandoffReasonFitmentResearch is the Story 4.4 research task word:
	// the fitment panel recommends the manager's catalog research — the
	// decision-ready task of 2.14 with the recorded material (the SQL
	// mirror 00021 admits it from this spec on; the automatic catalog
	// request is never performed — OQ-5/AG-4).
	HandoffReasonFitmentResearch = "fitment_research"
	// HandoffReasonOffersResearch is the Story 4.5 research task word:
	// the comparison panel recommends the manager's conditions research —
	// the decision-ready task of 2.14 with the recorded material (the SQL
	// mirror 00024 admits it from this spec on; the automatic supplier
	// request is never performed — OQ-5/AG-4).
	HandoffReasonOffersResearch = "offers_research"
	// HandoffReasonOrderCreation is the Story 4.6 order task word: the
	// order panel of the parked AG-4 operation recommends the manager
	// create the client order in the ABCP interface by the exact-version
	// material — the decision-ready task of 2.14 (the SQL mirror 00025
	// admits it from this spec on; the automatic dispatch is honestly
	// blocked by the nil adapter — AG-4 — and the hidden interface
	// automation is never a capability replacement).
	HandoffReasonOrderCreation = "order_creation"
)

// HandoffReasons is the full dictionary (the +3 Story 4.3 words mirror
// the SQL CHECK of migrations/cases/00019 — the append-only extension of
// the 00013 word list).
var HandoffReasons = []string{
	HandoffReasonClientRequest,
	HandoffReasonDiagnosticUncertainty,
	HandoffReasonSafetyCritical,
	HandoffReasonUnresolvedContradiction,
	// The Story 4.4 word — the fifth: the dictionary extension is the new
	// spec version, the SQL mirror 00021 backstops the same list.
	HandoffReasonFitmentResearch,
	// The Story 4.5 word — the sixth: the dictionary extension is the new
	// spec version, the SQL mirror 00024 backstops the same list.
	HandoffReasonOffersResearch,
	// The Story 4.6 word — the seventh: the dictionary extension is the new
	// spec version, the SQL mirror 00025 backstops the same list.
	HandoffReasonOrderCreation,
}

// HandoffReasonTexts is the honest text of every reason word (the render
// material of the panel; the texts never enter the database or the
// outbox).
var HandoffReasonTexts = map[string]string{
	HandoffReasonClientRequest:           "Безусловная просьба клиента: клиент попросил человека.",
	HandoffReasonDiagnosticUncertainty:   "Диагностическая неопределённость: симптом не доказывает деталь — диагноз у специалиста.",
	HandoffReasonSafetyCritical:          "Safety-узел с неопределённостью: автоматический подбор остановлен — нужны подтверждённый материал и решение специалиста.",
	HandoffReasonUnresolvedContradiction: "Неустранённое противоречие источников: Staffy не выбирает вероятный вариант — материал передан человеку.",
	HandoffReasonFitmentResearch:         "Каталогическое исследование применимости: доказательства нет либо оно устарело — менеджеру задача исследовать каталог и записать результат.",
	HandoffReasonOffersResearch:          "Запрос актуальных условий: предложений нет либо они устарели — менеджеру задача запросить условия у подключённых поставщиков и записать предложения.",
	HandoffReasonOrderCreation:           "Создание клиентского заказа: адаптер ABCP не реализован (AG-4) — менеджер создаёт заказ в интерфейсе ABCP по точной версии материала; автоматическая отправка поставщику не выполняется",
}

// HandoffReasonText resolves the honest text of one reason word.
func HandoffReasonText(reason string) string {
	return HandoffReasonTexts[reason]
}

// HandoffReasonInDictionary reports whether the reason word belongs to
// the fixed dictionary (the request write is fail-closed on it).
func HandoffReasonInDictionary(reason string) bool {
	return inDictionary(HandoffReasons, reason)
}

// GuardHandoffRequestReason validates the reason word of the request
// creation (Story 4.3 — the honest extension of the existing creation
// path): the EMPTY word is the unconditional client ask of 2.13 (the
// client_request constant is the SYSTEM truth — the operator never types
// it, an explicit client_request input is refused); the three operator
// words of the part request boundaries are the honest inputs (the panel
// recommends, the operator decides); every other word is the honest
// refusal with the full dictionary list. The store re-validates the same
// guard under the locks (the SQL CHECK of 00019 is the backstop).
func GuardHandoffRequestReason(reason string) error {
	if reason == "" {
		return nil
	}
	if reason == HandoffReasonClientRequest {
		return fmt.Errorf("слово причины %q — системная истина безусловной просьбы клиента, оно не вводится оператором", reason)
	}
	if HandoffReasonInDictionary(reason) {
		return nil
	}
	return fmt.Errorf("слово причины передачи %q вне словаря: %s", reason, handoffReasonDictionaryText())
}

// handoffReasonDictionaryText renders the honest quoted list of the
// operator reason words (the refusal material — never a free input).
func handoffReasonDictionaryText() string {
	words := make([]string, 0, 3)
	for _, reason := range HandoffReasons {
		if reason == HandoffReasonClientRequest {
			continue
		}
		words = append(words, "«"+reason+"»")
	}
	joined := ""
	for index, word := range words {
		if index > 0 {
			joined += ", "
		}
		joined += word
	}
	return joined
}

// ---------------------------------------------------------------------------
// The target role dictionary (AC-3: the roles the request addresses)
// ---------------------------------------------------------------------------

// The target roles of the handoff request: the operator role and the
// supervision role (the assignee's own role must belong to the request's
// target set — the membership truth belongs to the store). The
// dictionary is the LIMIT of the slice, never the automatic picker: the
// «Policy и Operating Profile определяют получателей» contour does not
// exist (the honest Never of the spec — the assignment is the explicit
// human decision).
const (
	HandoffTargetRoleOperator    = "operator"
	HandoffTargetRoleSupervision = "supervision"
)

// HandoffTargetRoles is the full dictionary.
var HandoffTargetRoles = []string{HandoffTargetRoleOperator, HandoffTargetRoleSupervision}

// HandoffTargetRoleInDictionary reports whether the role word belongs to
// the fixed dictionary.
func HandoffTargetRoleInDictionary(role string) bool {
	return inDictionary(HandoffTargetRoles, role)
}

// ---------------------------------------------------------------------------
// The ack-deadline dictionary (the approved policy v0 of the slice)
// ---------------------------------------------------------------------------

// The acknowledgement deadline words: the approved policy dictionary of
// the assignment rounds. A deadline is a WORD with a fixed honest text
// and a fixed window — the machine gates the word, never parses the text
// and never breeds a deadline of its own (the «дедлайн не выдумывается»
// honesty of 2.4).
const (
	HandoffAck15m = "ack_15m"
	HandoffAck1h  = "ack_1h"
	HandoffAck4h  = "ack_4h"
)

// HandoffAckDeadlines is the full dictionary (the canonical order).
var HandoffAckDeadlines = []string{HandoffAck15m, HandoffAck1h, HandoffAck4h}

// HandoffAckDeadlineTexts is the honest text of every deadline word (the
// render material of the panel and the assignment card).
var HandoffAckDeadlineTexts = map[string]string{
	HandoffAck15m: "Подтверждение в течение 15 минут.",
	HandoffAck1h:  "Подтверждение в течение часа.",
	HandoffAck4h:  "Подтверждение в течение четырёх часов.",
}

// HandoffAckDeadlineText resolves the honest text of one deadline word.
func HandoffAckDeadlineText(word string) string {
	return HandoffAckDeadlineTexts[word]
}

// HandoffAckDeadlineInDictionary reports whether the deadline word
// belongs to the approved policy dictionary.
func HandoffAckDeadlineInDictionary(word string) bool {
	return inDictionary(HandoffAckDeadlines, word)
}

// HandoffAckDeadlineWindow resolves the time window of one deadline word
// (the zero duration of an unknown word — the guards refuse the word
// before the window is ever read).
func HandoffAckDeadlineWindow(word string) time.Duration {
	switch word {
	case HandoffAck15m:
		return 15 * time.Minute
	case HandoffAck1h:
		return time.Hour
	case HandoffAck4h:
		return 4 * time.Hour
	}
	return 0
}

// ---------------------------------------------------------------------------
// The return result dictionary (AC-9: the honest words of the return)
// ---------------------------------------------------------------------------

// The return results: the words the human owner states when returning
// the case to Staffy (the explicit resolution of the handoff lifecycle).
const (
	HandoffResultIssueResolved = "issue_resolved"
	HandoffResultHandedBack    = "handed_back"
)

// HandoffResultWords is the full dictionary.
var HandoffResultWords = []string{HandoffResultIssueResolved, HandoffResultHandedBack}

// HandoffResultTexts is the honest text of every result word (the render
// material of the panel history).
var HandoffResultTexts = map[string]string{
	HandoffResultIssueResolved: "Вопрос решён человеком: обращение возвращается к цифровому сотруднику с результатом.",
	HandoffResultHandedBack:    "Возврат без решения: человек передаёт обращение цифровому сотруднику обратно.",
}

// HandoffResultText resolves the honest text of one result word.
func HandoffResultText(word string) string {
	return HandoffResultTexts[word]
}

// HandoffResultInDictionary reports whether the result word belongs to
// the fixed dictionary.
func HandoffResultInDictionary(word string) bool {
	return inDictionary(HandoffResultWords, word)
}

// ---------------------------------------------------------------------------
// The lifecycle event kinds (the transition vocabulary)
// ---------------------------------------------------------------------------

// The handoff lifecycle event kinds: one event — one kind; the states
// never mix. assigned names the assignee, the backup and the deadline
// (the assignment round); acknowledged carries the resulting epoch (the
// ONE transition that moves the ownership — the exec-composition of the
// 2.4 machinery in the same transaction); declined consumes the
// assignment round without touching the owner; cancelled anchors the
// client's cancelling turn; returned states the result word and moves
// the ownership back to Staffy.
const (
	HandoffEventAssigned     = "assigned"
	HandoffEventAcknowledged = "acknowledged"
	HandoffEventDeclined     = "declined"
	HandoffEventCancelled    = "cancelled"
	HandoffEventReturned     = "returned"
)

// HandoffEventKinds is the full dictionary (the lifecycle order).
var HandoffEventKinds = []string{
	HandoffEventAssigned, HandoffEventAcknowledged, HandoffEventDeclined, HandoffEventCancelled, HandoffEventReturned,
}

// HandoffEventKindInDictionary reports whether the kind word belongs to
// the fixed dictionary (the event write is fail-closed on it).
func HandoffEventKindInDictionary(kind string) bool {
	return inDictionary(HandoffEventKinds, kind)
}

// ---------------------------------------------------------------------------
// The lifecycle state dictionary (the read-time words of the panel)
// ---------------------------------------------------------------------------

// The handoff lifecycle states: the DERIVED words of the moment (the
// last event of the request plus the honest time derivation). requested
// — the ask waits for the assignment; assigned — the round is open and
// the deadline has not passed; overdue — the round's deadline passed
// without a closing event (the derivation, never a record; the «honest
// wait» of AC-6); acknowledged — the assignee took the ownership (the
// lifecycle is closed by the acceptance); declined — the assignee
// refused this round, the ask stays open; cancelled — the client
// cancelled; returned — the human owner returned the case to Staffy.
const (
	HandoffStateRequested    = "requested"
	HandoffStateAssigned     = "assigned"
	HandoffStateOverdue      = "overdue"
	HandoffStateAcknowledged = "acknowledged"
	HandoffStateDeclined     = "declined"
	HandoffStateCancelled    = "cancelled"
	HandoffStateReturned     = "returned"
)

// HandoffStates is the full dictionary (the lifecycle order).
var HandoffStates = []string{
	HandoffStateRequested, HandoffStateAssigned, HandoffStateOverdue,
	HandoffStateAcknowledged, HandoffStateDeclined, HandoffStateCancelled, HandoffStateReturned,
}

// HandoffStateTexts is the honest text of every state word (the render
// material of the «Запрос человека» panel section; the states are words,
// never colors).
var HandoffStateTexts = map[string]string{
	HandoffStateRequested:    "Человек запрошен: ожидание назначения.",
	HandoffStateAssigned:     "Назначен: ожидание подтверждения специалистом.",
	HandoffStateOverdue:      "Просрочено: дедлайн подтверждения истёк без решения.",
	HandoffStateAcknowledged: "Принято: специалист подтвердил передачу.",
	HandoffStateDeclined:     "Отклонено: специалист отказался — ожидание нового назначения.",
	HandoffStateCancelled:    "Отменено: клиент отменил запрос человека.",
	HandoffStateReturned:     "Возвращено: человек вернул обращение цифровому сотруднику.",
}

// HandoffStateText resolves the honest text of one state word.
func HandoffStateText(word string) string {
	return HandoffStateTexts[word]
}

// HandoffStateInDictionary reports whether the state word belongs to the
// fixed dictionary.
func HandoffStateInDictionary(word string) bool {
	return inDictionary(HandoffStates, word)
}

// handoffOpenStates are the OPEN (active) states of the lifecycle: the
// ask exists and no closing transition (acknowledged / cancelled /
// returned) has happened. The declined round stays open — the client's
// ask does not disappear when one assignee refuses; the honest paths of
// the re-assignment (assign / reassign / take) stay alive.
var handoffOpenStates = map[string]bool{
	HandoffStateRequested: true, HandoffStateAssigned: true,
	HandoffStateOverdue: true, HandoffStateDeclined: true,
}

// HandoffStateActive reports whether the derived state word is an OPEN
// state of the lifecycle (the one-competing-task truth of AC-4 and the
// freeze truth of AC-2: an active handoff refuses the repeated request
// and freezes the new commitments).
func HandoffStateActive(word string) bool {
	return handoffOpenStates[word]
}

// HandoffActiveText is the fixed refusal line of the one-competing-task
// conflict: the second ask is never created while the current ask lives
// (AC-4); the current lifecycle words travel with the conflict.
const HandoffActiveText = "Запрос человека уже активен: вторая конкурирующая задача не создаётся — дождитесь закрытия текущего запроса (подтверждение, отмена клиента либо возврат)."

// HandoffAbsentText is the honest refusal of every lifecycle command
// without a request: there is nothing to assign, acknowledge, decline,
// cancel or return — the lifecycle begins with the client's ask. The
// app pre-transaction guards and the store re-validation share this
// sentence (the domain truth of the absent lifecycle).
const HandoffAbsentText = "Запроса человека не существует: назначать, подтверждать, отклонять, отменять или возвращать нечего — сначала безусловная просьба клиента."

// HandoffActiveError is the honest one-competing-task refusal of the
// repeated request_human (the 409 of the second competing task, AC-4):
// the CURRENT lifecycle words travel with the conflict — the derived
// state word with its honest text and the client status word with its
// honest text — so the operator and the client see what already lives
// instead of a bare refusal (the ModeConflictError precedent of the
// machine-readable conflicts).
type HandoffActiveError struct {
	State        string
	ClientStatus string
}

// Error renders the fixed refusal line (the words travel as the fields
// of the problem contract — the transport carries them, never a re-parse
// of this text).
func (e HandoffActiveError) Error() string { return HandoffActiveText }

// ---------------------------------------------------------------------------
// The client status dictionary (AC-11: «назначен ≠ принял» — two words)
// ---------------------------------------------------------------------------

// The client status words of the interface truth: the honest answer to
// «did a specialist connect?». human_requested — the ask is registered;
// human_assigned — a specialist is assigned and has NOT confirmed yet;
// human_acknowledged — a specialist took the case; human_cancelled — the
// client cancelled the ask. The assignment and the acceptance are TWO
// different words — the interface never claims the specialist is
// connected before the confirmed acceptance (AC-11: «интерфейс не
// сообщает, что специалист уже подключился» — до human_acknowledged).
const (
	HandoffClientRequested    = "human_requested"
	HandoffClientAssigned     = "human_assigned"
	HandoffClientAcknowledged = "human_acknowledged"
	HandoffClientCancelled    = "human_cancelled"
)

// HandoffClientStatuses is the full dictionary.
var HandoffClientStatuses = []string{
	HandoffClientRequested, HandoffClientAssigned, HandoffClientAcknowledged, HandoffClientCancelled,
}

// HandoffClientStatusTexts is the honest text of every client status
// word (the Domain Pack v0 material of the outgoing neutral replies and
// the CaseView render).
var HandoffClientStatusTexts = map[string]string{
	HandoffClientRequested:    "Запрос человека зарегистрирован: специалист будет назначен.",
	HandoffClientAssigned:     "Специалист назначен и ещё не подтвердил принятие обращения.",
	HandoffClientAcknowledged: "Специалист подтвердил принятие обращения.",
	HandoffClientCancelled:    "Запрос человека отменён.",
}

// HandoffClientStatusText resolves the honest text of one client status
// word.
func HandoffClientStatusText(word string) string {
	return HandoffClientStatusTexts[word]
}

// HandoffClientStatusInDictionary reports whether the client status word
// belongs to the fixed dictionary.
func HandoffClientStatusInDictionary(word string) bool {
	return inDictionary(HandoffClientStatuses, word)
}

// ---------------------------------------------------------------------------
// The system mode reason of the unconditional ask (AC-1: the operator
// never types it)
// ---------------------------------------------------------------------------

// HumanRequestReasonText is the fixed system reason of the human mode
// raise: the unconditional ask carries its own honest explanation — the
// operator NEVER inputs a reason for the request (the «повторный отказ
// от AI» is structurally impossible: the ask is not a proposal that can
// be refused, it is the unconditional record). The text respects the
// mode reason rune bound (MaxModeReasonRunes).
const HumanRequestReasonText = "Клиент попросил человека: безусловная просьба реплики, режим поднят машиной."

// ---------------------------------------------------------------------------
// The material (the store reads/writes; the lifecycle is derived)
// ---------------------------------------------------------------------------

// HandoffRequest is one append-only request row of the case: the request
// number (monotonic per case), the reason word (the dictionary constant
// — the request never carries a free reason), the owner snapshot of the
// moment, the target roles (the subset of the dictionary the request
// addresses), the ack-deadline word of the expected confirmation window
// with its honest deadline time (the created_at plus the word's window),
// the anchor turn of the accepted client ask and the actor. The history
// is never rewritten — the lifecycle is the read-time derivation of the
// event rows.
type HandoffRequest struct {
	TenantID      id.UUID
	CaseID        id.UUID
	RequestNo     int64
	Reason        string
	OwnerKind     string
	OwnerRef      id.UUID
	OwnerEpoch    int64
	TargetRoles   []string
	AckDeadline   string
	AckDeadlineAt time.Time
	TurnRef       id.UUID
	ActorID       id.UUID
	CreatedAt     time.Time
}

// HandoffEvent is one append-only lifecycle event row of one request:
// the monotonic event number, the kind word, and the per-kind material —
// assigned carries the assignee, the backup owner and the deadline word
// with its time (the round's own deadline: a re-assignment is a NEW
// event with a NEW deadline — the human re-decision); acknowledged
// carries the resulting ownership epoch; declined and cancelled carry
// no owner material (the owner never changes); cancelled carries the
// anchor turn of the client's cancelling reply; returned carries the
// result word. The request/event numbers are the server truth (the
// store computes the next monotonic number under the locks — the
// requests never carry them).
type HandoffEvent struct {
	TenantID       id.UUID
	CaseID         id.UUID
	RequestNo      int64
	EventNo        int64
	Kind           string
	Assignee       id.UUID
	BackupOwner    id.UUID
	AckDeadline    string
	DeadlineAt     time.Time
	Result         string
	TurnRef        id.UUID
	ResultingEpoch int64
	ActorID        id.UUID
	CreatedAt      time.Time
}

// ---------------------------------------------------------------------------
// The read-time lifecycle derivation (never a stored status)
// ---------------------------------------------------------------------------

// HandoffStateView is the derived lifecycle state of one request: the
// request row, the last event, the state word, and the material of the
// current assignment round (the assignee, the backup and the deadline
// of the LATEST assigned event — the round the closing events answer).
// Everything is a read — the overdue state is the honest time
// derivation of the open assigned round, never a stored status and
// never a worker's write.
type HandoffStateView struct {
	Request        HandoffRequest
	LastEvent      *HandoffEvent
	State          string
	Assignee       id.UUID
	BackupOwner    id.UUID
	DeadlineAt     time.Time
	Result         string
	ResultingEpoch int64
}

// DeriveHandoffState derives the read-time lifecycle state of one
// request: the LAST event (the maximal event number — the append-only
// history is never rewritten; the LATER row of an equal number wins the
// defensive tie, the table UNIQUE backstop makes the case unreachable)
// decides the base word, and the honest time derivation refines it — an
// assigned round whose deadline has passed without a closing event is
// OVERDUE (the derivation of the moment; the same history derives
// «assigned» again before the deadline and after a re-assignment).
func DeriveHandoffState(request HandoffRequest, events []HandoffEvent, now time.Time) HandoffStateView {
	view := HandoffStateView{Request: request, State: HandoffStateRequested}
	var last *HandoffEvent
	var assigned *HandoffEvent
	for i := range events {
		event := &events[i]
		if event.CaseID != request.CaseID || event.RequestNo != request.RequestNo {
			continue
		}
		if last == nil || event.EventNo >= last.EventNo {
			last = event
		}
		if event.Kind == HandoffEventAssigned {
			if assigned == nil || event.EventNo >= assigned.EventNo {
				assigned = event
			}
		}
	}
	if last != nil {
		view.LastEvent = last
		switch last.Kind {
		case HandoffEventAssigned:
			if !last.DeadlineAt.IsZero() && now.After(last.DeadlineAt) {
				view.State = HandoffStateOverdue
			} else {
				view.State = HandoffStateAssigned
			}
		case HandoffEventAcknowledged:
			view.State = HandoffStateAcknowledged
			view.ResultingEpoch = last.ResultingEpoch
		case HandoffEventDeclined:
			view.State = HandoffStateDeclined
		case HandoffEventCancelled:
			view.State = HandoffStateCancelled
		case HandoffEventReturned:
			view.State = HandoffStateReturned
			view.Result = last.Result
		}
	}
	if assigned != nil {
		view.Assignee = assigned.Assignee
		view.BackupOwner = assigned.BackupOwner
		view.DeadlineAt = assigned.DeadlineAt
	}
	return view
}

// Active reports whether the derived lifecycle is OPEN (the ask exists
// without a closing transition): the one-competing-task truth of AC-4 —
// a repeated request_human is the honest 409 while the current ask
// lives; the freeze truth of AC-2 — an active handoff freezes the new
// commitments together with the mode.
func (v HandoffStateView) Active() bool {
	return HandoffStateActive(v.State)
}

// DeriveHandoffClientStatus derives the honest client status word of
// the derived lifecycle state: requested / declined / overdue-without-
// assignment stay human_requested (the specialist is not connected
// yet); assigned and overdue (the round is named, the acceptance has
// not happened) are human_assigned — «назначен, ещё не принял»;
// acknowledged and returned are human_acknowledged (the specialist DID
// take the case — the honest word survives the return); cancelled is
// human_cancelled. The assignment and the acceptance are always two
// different words (AC-11).
func DeriveHandoffClientStatus(state string) string {
	switch state {
	case HandoffStateAssigned, HandoffStateOverdue:
		return HandoffClientAssigned
	case HandoffStateAcknowledged, HandoffStateReturned:
		return HandoffClientAcknowledged
	case HandoffStateCancelled:
		return HandoffClientCancelled
	}
	return HandoffClientRequested
}

// ---------------------------------------------------------------------------
// The conditional mode term of the unconditional ask (the 2.5 ratchet)
// ---------------------------------------------------------------------------

// HandoffModeTerm builds the conversation mode term of the human ask —
// the raise to human_requested through the EXISTING 2.5 machinery (the
// origin proposal, the signal source turn_event, the system reason
// constant). The term exists ONLY when the raise is honest: an absent
// ladder, sales or unclassified raise (the ratchet direction); an equal
// human_requested or a STRICTER mode returns false — the mode is not
// touched (the ask is unconditional: the handoff row is still created,
// the ladder keeps its strictest word — AC-1 «режим не понижается,
// просьба безусловна»). The mode version is the CAS expectation of the
// write (the store re-validates it under the advisory locks).
func HandoffModeTerm(active *ModeTerm, tenantID, caseID, conversationID id.UUID, turnRef, actorID id.UUID, now time.Time) (*ModeTerm, bool) {
	if active != nil && proposalRank(active.Mode) >= proposalRank(ModeHumanRequested) {
		return nil, false
	}
	version := int64(1)
	if active != nil {
		version = active.ModeVersion + 1
	}
	return &ModeTerm{
		TenantID: tenantID, CaseID: caseID, ConversationID: conversationID,
		ModeVersion: version, Mode: ModeHumanRequested, Scope: ModeScopeConversation,
		Origin: ModeOriginProposal, Reason: HumanRequestReasonText,
		SignalSource: ModeSignalTurnEvent, TurnRef: turnRef, ActorID: actorID,
		PriorMode: priorModeOf(active), RuleVersion: ModePolicyVersion,
		CreatedAt: now,
	}, true
}

// priorModeOf resolves the honest prior mode word of the raise (the
// absent ladder starts from unclassified — the admission floor).
func priorModeOf(active *ModeTerm) string {
	if active != nil {
		return active.Mode
	}
	return ModeUnclassified
}

// ---------------------------------------------------------------------------
// The record guards (the domain truth before SQL)
// ---------------------------------------------------------------------------

// GuardRequestHuman validates the material of the unconditional ask:
// the turn anchor is a valid event reference (the store guard proves it
// belongs to THIS case and is accepted — the anchor of the 2.5 turn
// signal discipline); the CAS mode version expectation is present; the
// target roles are a non-empty duplicate-free subset of the dictionary;
// the ack-deadline word belongs to the approved policy dictionary. The
// reason is NEVER an input — the dictionary constant is the system
// truth (AC-1).
func GuardRequestHuman(turnRef id.UUID, expectedModeVersion int64, targetRoles []string, ackDeadline string) error {
	if !turnRef.Valid() {
		return fmt.Errorf("поле turn_ref обязательно (UUID): просьба человека якорится принятой репликой клиента")
	}
	if expectedModeVersion <= 0 {
		return fmt.Errorf("expected_mode_version обязателен (CAS записи режима): перечитайте активный режим и повторите команду")
	}
	if len(targetRoles) == 0 {
		return fmt.Errorf("целевые роли обязательны: укажите хотя бы одно слово словаря (%s)", joinQuoted(HandoffTargetRoles))
	}
	seen := map[string]bool{}
	for _, role := range targetRoles {
		if !HandoffTargetRoleInDictionary(role) {
			return fmt.Errorf("целевая роль «%s» не входит в словарь (%s)", role, joinQuoted(HandoffTargetRoles))
		}
		if seen[role] {
			return fmt.Errorf("целевая роль «%s» повторяется", role)
		}
		seen[role] = true
	}
	if !HandoffAckDeadlineInDictionary(ackDeadline) {
		return fmt.Errorf("слово дедлайна подтверждения «%s» не входит в утверждённую policy (%s)", ackDeadline, joinQuoted(HandoffAckDeadlines))
	}
	return nil
}

// GuardAssignHandoff validates the assignment round: the derived state
// is OPEN and assignable (the fresh ask, the overdue round, or the
// declined round — the human re-assignment; the closed lifecycles
// refuse); the assignee and the backup are valid member references; the
// assignee's own role belongs to the request's target roles (the
// membership truth belongs to the store); the round's deadline word
// belongs to the approved policy. The assignment NEVER changes the
// owner (AC-3: назначение ≠ принятие).
func GuardAssignHandoff(state string, assignee id.UUID, assigneeRole string, targetRoles []string, backup id.UUID, ackDeadline string) error {
	switch state {
	case HandoffStateRequested, HandoffStateOverdue, HandoffStateDeclined:
	default:
		return fmt.Errorf("назначение невозможно из состояния «%s»: жизненный цикл закрыт (путь возврата — return_to_staffy либо новая просьба клиента)", state)
	}
	if !assignee.Valid() {
		return fmt.Errorf("assignee обязателен (UUID): назначьте конкретного участника")
	}
	if !HandoffTargetRoleInDictionary(assigneeRole) {
		return fmt.Errorf("роль назначаемого «%s» не входит в словарь целевых ролей (%s)", assigneeRole, joinQuoted(HandoffTargetRoles))
	}
	if !inDictionary(targetRoles, assigneeRole) {
		return fmt.Errorf("роль назначаемого «%s» не входит в целевые роли запроса (%s)", assigneeRole, joinQuoted(targetRoles))
	}
	if !backup.Valid() {
		return fmt.Errorf("backup обязателен (UUID): укажите резервного участника раунда")
	}
	if !HandoffAckDeadlineInDictionary(ackDeadline) {
		return fmt.Errorf("слово дедлайна подтверждения «%s» не входит в утверждённую policy (%s)", ackDeadline, joinQuoted(HandoffAckDeadlines))
	}
	return nil
}

// GuardHandoffAcknowledge validates the confirmed acceptance: the
// derived state is the OPEN assigned round (an overdue round refuses —
// the deadline passed, the honest paths are the re-assignment, the 2.4
// reassign or take); the actor IS the assignee (the server truth — the
// assignment or the notification is never an acknowledgement, AC-3/5);
// the CAS ownership epoch expectation is present. The acceptance is the
// ONE transition that moves the ownership (the exec-composition with
// the 2.4 machinery in the same transaction).
func GuardHandoffAcknowledge(state string, actor, assignee id.UUID, expectedEpoch int64) error {
	if state == HandoffStateOverdue {
		return fmt.Errorf("дедлайн подтверждения истёк: подтверждение невозможно — путь: переназначение, reassign или take")
	}
	if state != HandoffStateAssigned {
		return fmt.Errorf("подтверждение возможно только из состояния «назначен»: текущее состояние — «%s»", state)
	}
	if actor != assignee {
		return fmt.Errorf("подтвердить передачу может только назначенный специалист (assignee): назначение или уведомление не считается подтверждением")
	}
	if expectedEpoch <= 0 {
		return fmt.Errorf("expected_epoch обязателен (CAS владения): перечитайте текущего владельца и повторите команду")
	}
	return nil
}

// GuardHandoffDecline validates the honest refusal of the assigned
// round: the state is the open assigned round (the expired round
// refuses — the deadline already closed it honestly), the actor is the
// assignee. The decline NEVER changes the owner — the prior safe owner
// stays (AC-6 «прежний безопасный owner сохраняется»); the request
// stays open for the human re-assignment.
func GuardHandoffDecline(state string, actor, assignee id.UUID) error {
	if state == HandoffStateOverdue {
		return fmt.Errorf("дедлайн подтверждения истёк: отказ уже не нужен — путь: переназначение, reassign или take")
	}
	if state != HandoffStateAssigned {
		return fmt.Errorf("отказ возможен только из состояния «назначен»: текущее состояние — «%s»", state)
	}
	if actor != assignee {
		return fmt.Errorf("отклонить назначение может только назначенный специалист (assignee)")
	}
	return nil
}

// GuardCancelHumanRequest validates the client's cancellation: the
// lifecycle is OPEN (the closed lifecycles refuse — the acknowledged
// case returns through return_to_staffy), and the cancelling turn
// anchor is a valid event reference (the store guard proves it belongs
// to THIS case and is accepted). The cancellation NEVER changes the
// owner and NEVER deletes the history (the append-only truth); the mode
// stays (the 2.5 ratchet — the exit is the authoritative exit_mode).
func GuardCancelHumanRequest(state string, turnRef id.UUID) error {
	if !HandoffStateActive(state) {
		return fmt.Errorf("отмена невозможна: жизненный цикл закрыт («%s»); путь возврата — return_to_staffy", state)
	}
	if !turnRef.Valid() {
		return fmt.Errorf("поле turn_ref обязательно (UUID): отмена якорится принятой репликой клиента")
	}
	return nil
}

// GuardReturnToStaffy validates the explicit return: the lifecycle is
// the acknowledged acceptance (the human owner is the confirmed owner),
// and the result word belongs to the fixed dictionary. The return moves
// the ownership back to Staffy in the same transaction (the owner=staffy,
// the surface external_inbox, the epoch+1 — the exec-composition of the
// 2.4 machinery); the mode is NEVER touched by the return (AC-9: Staffy
// does not resume the sales path under the active safety/claim/
// cancellation/privacy modes — the mode ladder keeps the honest word).
func GuardReturnToStaffy(state string, result string) error {
	if state != HandoffStateAcknowledged {
		return fmt.Errorf("возврат возможен только из состояния «принято»: текущее состояние — «%s»", state)
	}
	if !HandoffResultInDictionary(result) {
		return fmt.Errorf("слово результата возврата «%s» не входит в словарь (%s)", result, joinQuoted(HandoffResultWords))
	}
	return nil
}

// ---------------------------------------------------------------------------
// The freeze gate of the new commitments (AC-2: the requested handoff
// freezes the incompatible actions until the target acknowledges)
// ---------------------------------------------------------------------------

// CommitmentFrozenText is the fixed refusal text of the new commitment
// under the active human request: the word of the mode travels with the
// refusal (the honest 400 «словом режима» of the spec).
const CommitmentFrozenText = "Запрос человека активен (режим human_requested): создание обязательств заморожено до подтверждённой передачи — несовместимые автоматические действия прекращаются."

// GuardCommitmentFreeze is the freeze gate of the CreateCommitment
// command (Story 2.13): the effective mode of the Work Item (the
// stricter of the conversation ladder and the item's own context)
// EQUALS human_requested, OR the case carries an ACTIVE human handoff —
// the new commitments refuse with the honest mode word. The control
// branch stays byte-identical: without the mode and without the handoff
// the gate returns nil and the 2.9 path never differs (the tests of
// 2.9 are not touched). A STRICTER mode without an active handoff is
// not this gate's word (the mode ladder of 2.5 governs its own paths);
// the active handoff freezes under any mode — the ask exists.
func GuardCommitmentFreeze(effective *ModeTerm, handoffActive bool) error {
	if handoffActive {
		return fmt.Errorf("%s", CommitmentFrozenText)
	}
	if effective != nil && effective.Mode == ModeHumanRequested {
		return fmt.Errorf("%s", CommitmentFrozenText)
	}
	return nil
}

// ---------------------------------------------------------------------------
// The outbox perimeter (the identifiers and the dictionary words only)
// ---------------------------------------------------------------------------

// Outbox event types of the human handoff surface (the 2.1 bounded
// vocabulary): the six lifecycle words.
const (
	OutboxEventHumanRequested      = "case.human_requested"
	OutboxEventHumanRequestedV1    = 1
	OutboxEventHumanAssigned       = "case.human_assigned"
	OutboxEventHumanAssignedV1     = 1
	OutboxEventHumanAcknowledged   = "case.human_acknowledged"
	OutboxEventHumanAcknowledgedV1 = 1
	OutboxEventHumanDeclined       = "case.human_declined"
	OutboxEventHumanDeclinedV1     = 1
	OutboxEventHumanCancelled      = "case.human_cancelled"
	OutboxEventHumanCancelledV1    = 1
	OutboxEventHumanReturned       = "case.human_returned"
	OutboxEventHumanReturnedV1     = 1
)

// HandoffEventPayload is the safe canonical JSON of every lifecycle
// event: the identifiers (the tenant, the case, the request and event
// numbers, the actors), the dictionary words (the kind, the reason, the
// target roles, the deadline, the result) and the server truths (the
// resulting epoch, the deadline time, the anchor turns). The system
// reason text of the mode and any free material NEVER enter the
// envelope (the «кандидат не покидает строку» precedent of 2.6).
type HandoffEventPayload struct {
	TenantID       id.UUID   `json:"tenant_id"`
	CaseID         id.UUID   `json:"case_id"`
	RequestNo      int64     `json:"request_no"`
	EventNo        int64     `json:"event_no"`
	Kind           string    `json:"kind"`
	Reason         string    `json:"reason"`
	TargetRoles    []string  `json:"target_roles"`
	AckDeadline    string    `json:"ack_deadline"`
	DeadlineAt     time.Time `json:"deadline_at"`
	Assignee       id.UUID   `json:"assignee"`
	BackupOwner    id.UUID   `json:"backup_owner"`
	Result         string    `json:"result"`
	ResultingEpoch int64     `json:"resulting_epoch"`
	TurnRef        id.UUID   `json:"turn_ref"`
	ActorID        id.UUID   `json:"actor_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// DedupKeyHandoffEvent builds the dedup key of one lifecycle event.
func DedupKeyHandoffEvent(caseID id.UUID, requestNo, eventNo int64) string {
	return fmt.Sprintf("case.human_handoff_event:%s:%d:%d", caseID.String(), requestNo, eventNo)
}

// HandoffEventPayloadJSON renders the safe canonical JSON.
func HandoffEventPayloadJSON(payload HandoffEventPayload) ([]byte, error) {
	return json.Marshal(payload)
}
