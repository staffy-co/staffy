package domain

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// Module action, story 4.8 — the order correction domain: the Correction
// Work Item over the LAST CLOSED blocked decision of the manager order
// verification (4.7). The correction is a LIFECYCLE RECORD anchored to the
// committed operation of create_abcp_customer_order WITH its blocked
// verification decision — NOT the 2-13 turn_ref anchor, NOT the 2-15 case
// fact correction (the operator's resume-facts contour differs BY NAME:
// order_correction), and NEVER a transfer of the case ownership (the 2-4
// terms stay untouched): the machine mirrors the 2-13/4-7 precedents (the
// append-only events, the read-time derivations, the CAS epoch) over the
// ORDER anchor.
//
// The structural progression block is TWO surgical gates: an OPEN
// correction refuses the new verification assignment (the honest 409 —
// the check returns AFTER the correction closes) and the authorization of
// a NEW create_abcp_customer_order is refused BEFORE the reservation
// transaction («новый дублирующий заказ не создаётся» AC-9 — the
// single_use consumption is per confirmation, never per case; the gate
// lives in the 3-5 app command, the 3-6 dispatch ladder is NEVER touched,
// nothing is consumed). The replacement is a NEW proposal version of 4.6
// by the existing EDIT ladder (a replacement entity does NOT exist — the
// correction ↔ confirmed-version link is the reread record). The former
// customer Confirmation is NEVER carried over: the closure (corrected)
// requires the pinned NEW confirmed version {proposal, version,
// confirmation} — the version is the LAST one, confirmed, and the LIVE
// re-derivation of the order material equals the version digest (the
// same fence code as the ladder 4.5 and the 4.7 fence — reused, never a
// new dispatch step; the hidden degradation is forbidden STRUCTURALLY:
// a divergence refuses the closure with the EXPLICIT field differences).
//
// The ABCP order change is the manager's MANUAL contour ONLY (AG-4: no
// change command exists in the live 4-1 declaration, the catalog is never
// extended, the adapter is nil): the reread is recorded by the manager's
// words — Staffy never reads ABCP and never pretends an order change. The
// supplier-send honesty of OQ-3 rides the whole contour: the former
// variant was NEVER sent to the supplier (the word of the recommended
// explanation) and no word of this domain claims a send — the honest
// negations only.

// ---------------------------------------------------------------------------
// The honest words of the slice (the text, never only color)
// ---------------------------------------------------------------------------

// OrderCorrectionPendingWord is the state word text of the open
// correction (AC-1/AC-2): the order is blocked from the progression
// until the fix and the NEW verification; the word is the panel truth of
// pending_correction.
const OrderCorrectionPendingWord = "коррекция открыта: заказ заблокирован от продвижения до исправления и новой проверки"

// OrderCorrectionCorrectedWord is the closure word of the corrected
// outcome (AC-12): the order returns to the manager verification
// (pending_manager_verification) by a NEW task — the former
// verification is never reused (the owner's decision opens the ladder).
const OrderCorrectionCorrectedWord = "исправлен: заказ возвращается на проверку менеджера (pending_manager_verification); новая задача проверки — решением владельца"

// OrderCorrectionCeasedWord is the closure word of the ceased outcome:
// the correction closed without a replacement — the order stays blocked
// from the progression (the blocked decision of 4.7 still lives).
const OrderCorrectionCeasedWord = "коррекция прекращена без замены: заказ остаётся заблокированным от продвижения — решение о дальнейших шагах за владельцем"

// OrderCorrectionOverdueWord is the read-time overdue word: the
// deadline passed without a closing event — the honest wait, the next
// step is the owner's reassignment; the expiry is NEVER an automatic
// closure (structurally: no event kind can express it, AG-4/AD-14 — no
// workers, no timers).
const OrderCorrectionOverdueWord = "просрочен: дедлайн коррекции истёк без закрытия — автоматических действий нет; следующий шаг — переназначение владельцем"

// OrderCorrectionNotStartedWord is the honest assignment boundary word
// (the 4-7 «назначение ≠ принятие» family): the owner assignment is NOT
// the start of the work — nothing records a contact or a start except
// the events themselves.
const OrderCorrectionNotStartedWord = "назначение коррекции ≠ начало работы: контакт и начало не фиксируются ничем, кроме событий"

// OrderCorrectionManualContourWord is the MANDATORY word of the ABCP
// change honesty (AG-4/AC-10): the change of the existing ABCP order is
// the manager's MANUAL act in the store's contour — Staffy neither
// changes the order nor creates a new one (structurally: no change
// command in the catalog, the adapter is nil).
const OrderCorrectionManualContourWord = "изменение существующего заказа ABCP выполняется вручную в контуре магазина; Staffy не изменяет и не создаёт новый заказ автоматически"

// OrderCorrectionRereadWord is the reread honesty word (AG-4): the
// reread is recorded by the manager's words — Staffy never reads ABCP;
// the composition was verified against the customer-confirmed version.
const OrderCorrectionRereadWord = "перечит подтверждён словами менеджера: Staffy не читает ABCP (AG-4) — состав сверён с подтверждённой клиентом версией"

// OrderCorrectionNewConfirmationWord is the new-confirmation boundary
// word (AC-7): the former customer Confirmation never carries over —
// the corrected closure requires the NEW confirmation of the exact
// version.
const OrderCorrectionNewConfirmationWord = "требуется новое подтверждение клиента: прежнее подтверждение не переносится"

// OrderCorrectionNotSentWord is the honest supplier-send boundary word
// of the recommended explanation (AC-6, the OQ-3 fence): the former
// variant was NEVER sent to the supplier — the customer has nothing to
// re-decide about a sent order.
const OrderCorrectionNotSentWord = "прежний вариант поставщику не отправлялся"

// OrderCorrectionCancellationWord is the cancellation recommendation
// word of the client_refused cessation (AC-11): the order cancellation
// is the manager's manual contour — the contour 4-10 is OUTSIDE this
// slice; the absence of the client's consent NEVER becomes a
// replacement.
const OrderCorrectionCancellationWord = "отмена заказа — ручной контур менеджера (контур 4-10): согласие клиента не предполагается — замены без нового подтверждения не существует"

// OrderCorrectionChannelWord is the honest contact-channel boundary
// word (AC-5, the OQ-1/OQ-2 openness, the 2-13 Never precedent): the
// communication channel is the store's manual contour — this slice
// fixes neither the delivery nor the notification of the message.
const OrderCorrectionChannelWord = "канал связи с клиентом — ручной контур магазина (Jivo/телефония): доставка сообщения не фиксируется этим контуром"

// OrderCorrectionAssignGateWord is the structural gate word of the new
// verification assignment while the correction is open (AC-2): the new
// verification task is NOT created until the correction closes — the
// honest 409 of the assign.
const OrderCorrectionAssignGateWord = "коррекция заказа открыта — проверка вернётся после исправления"

// OrderCorrectionOrderGateWord is the structural gate word of the new
// order authorization while the correction is open (AC-9/AC-10): the
// new duplicate order is NEVER created — the change of the existing
// order is the manager's manual contour; the refusal happens BEFORE the
// reservation transaction (nothing is consumed).
const OrderCorrectionOrderGateWord = "коррекция заказа открыта: изменение существующего заказа выполняется вручную в контуре магазина; новый заказ не создаётся"

// OrderCorrectionNoBlockedWord is the honest anchor refusal word of the
// open without a blocked decision: the correction opens BY the «Нужна
// коррекция» decision of the verification — no blocked decision, no
// correction (the anchor is BY NAME).
const OrderCorrectionNoBlockedWord = "коррекция открывается по решению проверки «Нужна коррекция» — заблокированного решения нет"

// OrderCorrectionActiveText is the honest one-open-correction conflict
// word (the 409 of the repeated open — the OrderVerificationActiveText
// precedent of 4.7): the second competing correction of one operation
// is never created.
const OrderCorrectionActiveText = "коррекция заказа уже открыта — дождитесь исхода либо переназначьте: вторая конкурирующая коррекция не создаётся"

// OrderCorrectionReassignedWord is the honest CAS refusal word of the
// stale epoch: the correction was reassigned — the former owner's
// reread/cease attempts answer the CURRENT state honestly.
const OrderCorrectionReassignedWord = "коррекция переназначена: ожидаемая эпоха устарела — перечитайте коррекцию и повторите команду"

// OrderCorrectionNotOwnerWord is the honest actor refusal word: only
// the owner of the correction records the reread or ceases it (the
// server truth — the assignment never impersonates the owner).
const OrderCorrectionNotOwnerWord = "записать перечит или прекратить коррекцию может только владелец коррекции: назначение не передаёт владение другим участникам"

// OrderCorrectionSupersededWord is the old-confirmation refusal word
// (AC-15): the pinned confirmation belongs to a SUPERSEDED version —
// the repeat use of the old Confirmation blocks the route BY NAME.
const OrderCorrectionSupersededWord = "подтверждение клиента устарело (superseded) — требуется новое подтверждение"

// OrderCorrectionCompositionChangedWord is the live-composition refusal
// word (AC-13): the price/availability/conditions changed AFTER the
// customer confirmation — the closure on an unconfirmed composition is
// impossible STRUCTURALLY (the hidden degradation is forbidden), the
// EXPLICIT field differences show on the screen.
const OrderCorrectionCompositionChangedWord = "состав изменился после подтверждения клиента — требуется обновлённое подтверждение"

// OrderCorrectionPurchasingHiddenWord is the honest hiding word of the
// purchasing price without the Commercial scope (the 4-5/4-7
// precedent): the hiding is a WORD, never a blank.
const OrderCorrectionPurchasingHiddenWord = "закупочные данные — внутренние данные магазина; без полномочия не раскрываются"

// ---------------------------------------------------------------------------
// The deadline dictionary (the approved policy v0 of the slice)
// ---------------------------------------------------------------------------

// The correction deadline words: the approved policy dictionary of the
// correction rounds. A deadline is a WORD with a fixed honest text and
// a fixed window — the machine gates the word, never parses the text
// and never breeds a deadline of its own (the «дедлайн не
// выдумывается» honesty of 2.4/2.13/4.7 carried into the correction
// contour).
const (
	Correct4h = "correct_4h"
	Correct1d = "correct_1d"
)

// CorrectionDeadlines is the full dictionary (the canonical order).
var CorrectionDeadlines = []string{Correct4h, Correct1d}

// CorrectionDeadlineTexts is the honest text of every deadline word (the
// render material of the panel and the correction card).
var CorrectionDeadlineTexts = map[string]string{
	Correct4h: "Коррекция подбора в течение четырёх часов.",
	Correct1d: "Коррекция подбора в течение суток.",
}

// CorrectionDeadlineText resolves the honest text of one deadline word.
func CorrectionDeadlineText(word string) string {
	return CorrectionDeadlineTexts[word]
}

// CorrectionDeadlineInDictionary reports whether the deadline word
// belongs to the approved policy dictionary.
func CorrectionDeadlineInDictionary(word string) bool {
	return WordIn(CorrectionDeadlines, word)
}

// CorrectionDeadlineWindow resolves the time window of one deadline word
// (the zero duration of an unknown word — the guards refuse the word
// before the window is ever read).
func CorrectionDeadlineWindow(word string) time.Duration {
	switch word {
	case Correct4h:
		return 4 * time.Hour
	case Correct1d:
		return 24 * time.Hour
	}
	return 0
}

// ---------------------------------------------------------------------------
// The cessation reason dictionary (three structured words)
// ---------------------------------------------------------------------------

// The cessation reasons of the «Прекратить коррекцию» closure: the
// structured three-word dictionary — the manager never types a free
// reason (the word is the dictionary constant; the SQL mirror of 00007
// is the backstop).
const (
	CorrectionCessationClientRefused = "client_refused"
	CorrectionCessationNotNeeded     = "not_needed"
	CorrectionCessationAbandoned     = "abandoned"
)

// CorrectionCessationReasons is the full dictionary (the canonical order).
var CorrectionCessationReasons = []string{
	CorrectionCessationClientRefused,
	CorrectionCessationNotNeeded,
	CorrectionCessationAbandoned,
}

// CorrectionCessationReasonTexts is the honest text of every cessation
// reason word (the render material of the cease form and the history).
var CorrectionCessationReasonTexts = map[string]string{
	CorrectionCessationClientRefused: "Клиент отказался от исправленного варианта: согласие не предполагается — замены без нового подтверждения не существует.",
	CorrectionCessationNotNeeded:     "Коррекция не потребовалась: причина заблокированного решения закрыта без замены позиций.",
	CorrectionCessationAbandoned:     "Коррекция прекращена без исхода: работа по исправлению свёрнута решением владельца.",
}

// CorrectionCessationReasonText resolves the honest text of one reason
// word.
func CorrectionCessationReasonText(word string) string {
	return CorrectionCessationReasonTexts[word]
}

// CorrectionCessationReasonInDictionary reports whether the reason word
// belongs to the fixed dictionary.
func CorrectionCessationReasonInDictionary(word string) bool {
	return WordIn(CorrectionCessationReasons, word)
}

// ---------------------------------------------------------------------------
// The lifecycle event kinds (the transition vocabulary)
// ---------------------------------------------------------------------------

// The correction lifecycle event kinds: one event — one kind; the states
// never mix. opened names the owner and the deadline AND carries the
// blocked positions with the reason word BY NAME (the one-transaction
// SQL-truth carry of the open command — the original blocked event is
// read, never rewritten); reassigned names the NEW owner and the NEW
// deadline (its event_no joins the assignment epoch — the CAS token of
// every later command); reread carries the reread external object ID
// and the pinned NEW confirmed version identity; ceased carries the
// cessation reason word. The AUTO-CLOSURE kind does not exist — the
// expiry never writes a decision (AG-4/AD-14 structurally).
const (
	OrderCorrectionEventOpened     = "opened"
	OrderCorrectionEventReassigned = "reassigned"
	OrderCorrectionEventReread     = "reread"
	OrderCorrectionEventCeased     = "ceased"
)

// OrderCorrectionEventKinds is the full dictionary (the lifecycle order).
var OrderCorrectionEventKinds = []string{
	OrderCorrectionEventOpened, OrderCorrectionEventReassigned,
	OrderCorrectionEventReread, OrderCorrectionEventCeased,
}

// OrderCorrectionEventKindInDictionary reports whether the kind word
// belongs to the fixed dictionary (the event write is fail-closed on it).
func OrderCorrectionEventKindInDictionary(kind string) bool {
	return WordIn(OrderCorrectionEventKinds, kind)
}

// ---------------------------------------------------------------------------
// The lifecycle state and outcome dictionaries (the read-time words)
// ---------------------------------------------------------------------------

// The correction lifecycle states: the DERIVED words of the moment (the
// last event of the correction plus the honest time derivation).
// pending_correction — the round is open and the deadline has not
// passed; overdue — the round's deadline passed without a closure (the
// derivation, never a record; the automatic closure does not exist
// structurally); corrected and ceased — the closed outcomes (the
// history is the truth).
const (
	OrderCorrectionStatePending   = "pending_correction"
	OrderCorrectionStateOverdue   = "overdue"
	OrderCorrectionStateCorrected = "corrected"
	OrderCorrectionStateCeased    = "ceased"
)

// OrderCorrectionStates is the full dictionary (the lifecycle order).
var OrderCorrectionStates = []string{
	OrderCorrectionStatePending, OrderCorrectionStateOverdue,
	OrderCorrectionStateCorrected, OrderCorrectionStateCeased,
}

// OrderCorrectionStateTexts is the honest text of every state word (the
// render material of the «Коррекция заказа» panel; the states are words,
// never colors).
var OrderCorrectionStateTexts = map[string]string{
	OrderCorrectionStatePending:   OrderCorrectionPendingWord,
	OrderCorrectionStateOverdue:   OrderCorrectionOverdueWord,
	OrderCorrectionStateCorrected: OrderCorrectionCorrectedWord,
	OrderCorrectionStateCeased:    OrderCorrectionCeasedWord,
}

// OrderCorrectionStateText resolves the honest text of one state word.
func OrderCorrectionStateText(word string) string {
	return OrderCorrectionStateTexts[word]
}

// OrderCorrectionStateInDictionary reports whether the state word
// belongs to the fixed dictionary.
func OrderCorrectionStateInDictionary(word string) bool {
	return WordIn(OrderCorrectionStates, word)
}

// orderCorrectionOpenStates are the OPEN (active) states of the
// lifecycle: the correction exists without a closure event (reread /
// ceased). The overdue round stays open — the expiry closed nothing
// (the fallback is the owner's explicit reassignment, never an
// auto-decision).
var orderCorrectionOpenStates = map[string]bool{
	OrderCorrectionStatePending: true,
	OrderCorrectionStateOverdue: true,
}

// OrderCorrectionStateActive reports whether the derived state word is
// an OPEN state of the lifecycle (the one-open-correction truth: an
// open correction of one operation refuses the repeated open with the
// honest 409 and holds BOTH structural progression gates).
func OrderCorrectionStateActive(word string) bool {
	return orderCorrectionOpenStates[word]
}

// The correction outcome words: the closure outcomes of the correction
// lifecycle — corrected (the reread closed it with the NEW confirmed
// version pin) and ceased (the cessation reason closed it). The outcome
// is the DERIVED word of the closure event kind, never a stored status.
const (
	OrderCorrectionOutcomeCorrected = "corrected"
	OrderCorrectionOutcomeCeased    = "ceased"
)

// OrderCorrectionOutcomes is the full dictionary (the canonical order).
var OrderCorrectionOutcomes = []string{
	OrderCorrectionOutcomeCorrected, OrderCorrectionOutcomeCeased,
}

// ---------------------------------------------------------------------------
// The material (the store reads/writes; the lifecycle is derived)
// ---------------------------------------------------------------------------

// OrderCorrection is one append-only correction row of the case: the
// correction number (the monotonic per-case ladder — the repeated
// correction of one operation after a NEW blocked decision is a NEW
// correction, the history is never rewritten), the ANCHOR triple (the
// committed operation of the order action, the blocked verification
// number the correction addresses, the receipt identity pins — the
// committed receipt version and the external ABCP Order ID of the
// receipt references, the readback material of the screen), the work
// item anchor and the creator. The correction NEVER carries the case
// ownership (the 2-4 terms stay untouched — it lives over the order
// operation, not over the reply).
type OrderCorrection struct {
	TenantID         id.UUID
	CaseID           id.UUID
	WorkItemID       id.UUID
	CorrectionNo     int64
	OperationID      id.UUID
	VerificationNo   int64
	ReceiptVersionNo int64
	ExternalOrderID  string
	CreatorID        id.UUID
	CreatedAt        time.Time
}

// OrderCorrectionEvent is one append-only lifecycle event row of one
// correction: the monotonic event number (the assignment epoch when the
// kind is opened or reassigned — the CAS expectation of the later
// commands), the kind word and the per-kind material — opened carries
// the owner, the deadline word with its time AND the blocked positions
// with the structured reason word BY NAME (the SQL-truth carry of the
// open command); reassigned carries the NEW owner and the NEW deadline;
// reread carries the reread external object ID and the pinned identity
// triple of the NEW confirmed version {proposal, version,
// confirmation}; ceased carries the cessation reason word. The numbers
// are the server truth (the store computes the next monotonic number
// under the locks — the events never carry them).
type OrderCorrectionEvent struct {
	TenantID         id.UUID
	CaseID           id.UUID
	CorrectionNo     int64
	EventNo          int64
	Kind             string
	Owner            id.UUID
	CorrectDeadline  string
	DeadlineAt       time.Time
	Positions        []string
	Reason           string
	ExternalObjectID string
	ProposalID       id.UUID
	VersionNo        int64
	ConfirmationID   id.UUID
	CessationReason  string
	ActorID          id.UUID
	CreatedAt        time.Time
}

// ---------------------------------------------------------------------------
// The read-time lifecycle derivation (never a stored status)
// ---------------------------------------------------------------------------

// OrderCorrectionStateView is the derived lifecycle state of one
// correction: the correction row, the last event, the state word, the
// material of the CURRENT assignment round (the owner and the deadline
// of the LATEST opened-or-reassigned event — the round the closure
// events answer) and the closure material (the reread/ceased event).
// Everything is a read — the overdue state is the honest time
// derivation of the open round, never a stored status and never a
// worker's write.
type OrderCorrectionStateView struct {
	Correction      OrderCorrection
	LastEvent       *OrderCorrectionEvent
	State           string
	Owner           id.UUID
	CorrectDeadline string
	DeadlineAt      time.Time
	AssignmentEpoch int64
	Closure         *OrderCorrectionEvent
}

// DeriveOrderCorrectionState derives the read-time lifecycle state of
// one correction: the LAST event (the maximal event number — the
// append-only history is never rewritten; the LATER row of an equal
// number wins the defensive tie, the table UNIQUE backstop makes the
// case unreachable) decides the base word, and the honest time
// derivation refines it — an open round whose deadline has passed
// without a closure is OVERDUE (the derivation of the moment; the
// expiry is NEVER an automatic closure — no event kind can express it,
// AG-4/AD-14 structurally). The assignment round material derives from
// the LATEST opened-or-reassigned event (a reassignment is a NEW round
// with a NEW epoch).
func DeriveOrderCorrectionState(correction OrderCorrection, events []OrderCorrectionEvent, now time.Time) OrderCorrectionStateView {
	view := OrderCorrectionStateView{Correction: correction}
	var last *OrderCorrectionEvent
	var assigned *OrderCorrectionEvent
	var closure *OrderCorrectionEvent
	for i := range events {
		event := &events[i]
		if event.CaseID != correction.CaseID || event.CorrectionNo != correction.CorrectionNo {
			continue
		}
		if last == nil || event.EventNo >= last.EventNo {
			last = event
		}
		if event.Kind == OrderCorrectionEventOpened || event.Kind == OrderCorrectionEventReassigned {
			if assigned == nil || event.EventNo >= assigned.EventNo {
				assigned = event
			}
		}
		if event.Kind == OrderCorrectionEventReread || event.Kind == OrderCorrectionEventCeased {
			if closure == nil || event.EventNo >= closure.EventNo {
				closure = event
			}
		}
	}
	if last != nil {
		view.LastEvent = last
		switch last.Kind {
		case OrderCorrectionEventOpened, OrderCorrectionEventReassigned:
			deadline := time.Time{}
			if assigned != nil {
				deadline = assigned.DeadlineAt
			}
			if !deadline.IsZero() && now.After(deadline) {
				view.State = OrderCorrectionStateOverdue
			} else {
				view.State = OrderCorrectionStatePending
			}
		case OrderCorrectionEventReread:
			view.State = OrderCorrectionStateCorrected
		case OrderCorrectionEventCeased:
			view.State = OrderCorrectionStateCeased
		}
	}
	if assigned != nil {
		view.Owner = assigned.Owner
		view.CorrectDeadline = assigned.CorrectDeadline
		view.DeadlineAt = assigned.DeadlineAt
		view.AssignmentEpoch = assigned.EventNo
	}
	if closure != nil {
		view.Closure = closure
	}
	return view
}

// Active reports whether the derived lifecycle is OPEN (the correction
// exists without a closure): the one-open-correction truth — a repeated
// open of the same operation is the honest 409 and BOTH structural
// progression gates stay held while the current correction lives.
func (v OrderCorrectionStateView) Active() bool {
	return OrderCorrectionStateActive(v.State)
}

// Outcome derives the closure outcome word of the correction: the
// corrected outcome rides the reread event, the ceased outcome rides
// the ceased event; an OPEN correction has no outcome (the empty word —
// the panel renders the state word, never a fake outcome).
func (v OrderCorrectionStateView) Outcome() string {
	if v.Closure == nil {
		return ""
	}
	switch v.Closure.Kind {
	case OrderCorrectionEventReread:
		return OrderCorrectionOutcomeCorrected
	case OrderCorrectionEventCeased:
		return OrderCorrectionOutcomeCeased
	}
	return ""
}

// ---------------------------------------------------------------------------
// The recommended explanation diff (the read-time former → current
// honesty of the positions; AC-5/AC-6/AC-13)
// ---------------------------------------------------------------------------

// OrderCorrectionFieldDiff is one explicit field difference of one
// position: the field label word, the former render and the current
// render — the «прежний → новый» is TEXT, never a color only (AC-16).
type OrderCorrectionFieldDiff struct {
	Field   string
	Former  string
	Current string
}

// OrderCorrectionPositionDiff is the per-position diff of the
// recommended explanation: the position key, the presence flags of the
// former/current materials and the explicit field differences
// (артикул/оригинал-аналог/количество/цена/срок — the words carry the
// labels, the values are the renders of the SAME builder material of
// 4.6).
type OrderCorrectionPositionDiff struct {
	PositionKey string
	InFormer    bool
	InCurrent   bool
	Fields      []OrderCorrectionFieldDiff
}

// OrderCorrectionDiff is the read-time diff of the former committed
// material against the LIVE re-derived material: the per-position
// differences (the union of the position keys, the former-material
// order first) and the case-level known-sum difference. The diff is the
// RENDER material of the recommended explanation AND the refusal
// material of the composition-changed fence (AC-13: the EXPLICIT
// differences show, the hidden degradation is impossible).
type OrderCorrectionDiff struct {
	Positions       []OrderCorrectionPositionDiff
	FormerKnownSum  float64
	CurrentKnownSum float64
	KnownSumChanged bool
	FormerCurrency  string
	CurrentCurrency string
	CurrencyChanged bool
}

// Changed reports whether the former and the current materials diverge
// (any position field, the presence, the known sum or the currency).
func (d OrderCorrectionDiff) Changed() bool {
	if d.KnownSumChanged || d.CurrencyChanged {
		return true
	}
	for i := range d.Positions {
		if len(d.Positions[i].Fields) > 0 || d.Positions[i].InFormer != d.Positions[i].InCurrent {
			return true
		}
	}
	return false
}

// The field label words of the per-position diff (the render labels of
// the recommended explanation — the AC-5 field set).
const (
	correctionDiffArticle     = "артикул"
	correctionDiffOriginality = "оригинал/аналог"
	correctionDiffQuantity    = "количество"
	correctionDiffPrice       = "цена за единицу"
	correctionDiffConditions  = "срок и условия"
)

// DeriveOrderCorrectionDiff derives the read-time per-position diff of
// the former committed material against the LIVE re-derived material
// (the inputs are the SAME builder material of 4.6 — the pins of the
// operation version and the live re-derivation): every position key of
// the union shows its explicit field differences (the article, the
// originality, the quantity, the unit price, the conditions) — the
// equal fields never appear (the diff shows the DIFFERENCES only, the
// full composition rides the panel's own read). The case-level known
// sum and currency differences ride the diff too. The diff is PURE —
// the caller owns the reads.
func DeriveOrderCorrectionDiff(former, current OrderCommitmentMaterial) OrderCorrectionDiff {
	diff := OrderCorrectionDiff{
		FormerKnownSum:  former.KnownClientSum,
		CurrentKnownSum: current.KnownClientSum,
		FormerCurrency:  former.Currency,
		CurrentCurrency: current.Currency,
	}
	diff.KnownSumChanged = former.KnownClientSum != current.KnownClientSum
	diff.CurrencyChanged = former.Currency != current.Currency
	formerPositions := map[string]OrderPosition{}
	currentPositions := map[string]OrderPosition{}
	var order []string
	for _, position := range former.Positions {
		formerPositions[position.PositionKey] = position
		order = append(order, position.PositionKey)
	}
	for _, position := range current.Positions {
		if _, exists := formerPositions[position.PositionKey]; !exists {
			order = append(order, position.PositionKey)
		}
		currentPositions[position.PositionKey] = position
	}
	seen := map[string]bool{}
	for _, key := range order {
		if seen[key] {
			continue
		}
		seen[key] = true
		positionDiff := OrderCorrectionPositionDiff{PositionKey: key}
		formerPosition, inFormer := formerPositions[key]
		currentPosition, inCurrent := currentPositions[key]
		positionDiff.InFormer = inFormer
		positionDiff.InCurrent = inCurrent
		if inFormer && inCurrent {
			if formerPosition.Brand != currentPosition.Brand || formerPosition.Article != currentPosition.Article {
				positionDiff.Fields = append(positionDiff.Fields, OrderCorrectionFieldDiff{
					Field:   correctionDiffArticle,
					Former:  formerPosition.Brand + " " + formerPosition.Article,
					Current: currentPosition.Brand + " " + currentPosition.Article,
				})
			}
			if formerPosition.Originality != currentPosition.Originality {
				positionDiff.Fields = append(positionDiff.Fields, OrderCorrectionFieldDiff{
					Field:   correctionDiffOriginality,
					Former:  OrderOriginalityRenderWord(formerPosition.Originality),
					Current: OrderOriginalityRenderWord(currentPosition.Originality),
				})
			}
			if formerPosition.Quantity != currentPosition.Quantity {
				positionDiff.Fields = append(positionDiff.Fields, OrderCorrectionFieldDiff{
					Field:   correctionDiffQuantity,
					Former:  strconv.Itoa(formerPosition.Quantity),
					Current: strconv.Itoa(currentPosition.Quantity),
				})
			}
			if formerPosition.UnitClientPrice != currentPosition.UnitClientPrice {
				positionDiff.Fields = append(positionDiff.Fields, OrderCorrectionFieldDiff{
					Field:   correctionDiffPrice,
					Former:  priceRender(formerPosition.UnitClientPrice, formerPosition.Currency),
					Current: priceRender(currentPosition.UnitClientPrice, currentPosition.Currency),
				})
			}
			if formerPosition.Conditions != currentPosition.Conditions {
				positionDiff.Fields = append(positionDiff.Fields, OrderCorrectionFieldDiff{
					Field:   correctionDiffConditions,
					Former:  formerPosition.Conditions,
					Current: currentPosition.Conditions,
				})
			}
		}
		diff.Positions = append(diff.Positions, positionDiff)
	}
	return diff
}

// priceRender renders the unit price with its currency word (the honest
// render of the diff — never a bare float).
func priceRender(value float64, currency string) string {
	render := strconv.FormatFloat(value, 'f', 2, 64)
	if currency != "" {
		render += " " + currency
	}
	return render
}

// ---------------------------------------------------------------------------
// The record guards (the domain truth before SQL)
// ---------------------------------------------------------------------------

// GuardOrderCorrectionOpen validates the open command shape: the owner
// is a valid member reference (the membership and the Review-scope
// truth belong to the LIVE identity read of the app layer) and the
// correction deadline word belongs to the approved policy dictionary.
// The open NEVER moves the case ownership and NEVER rewrites the
// blocked event — the positions and the reason travel BY NAME from the
// blocked decision of the app layer's one-transaction read.
func GuardOrderCorrectionOpen(owner id.UUID, deadline string) error {
	if !owner.Valid() {
		return fmt.Errorf("owner обязателен (UUID): назначьте участника с полномочием review")
	}
	if !CorrectionDeadlineInDictionary(deadline) {
		return fmt.Errorf("слово дедлайна коррекции «%s» не входит в утверждённую policy (%s)", deadline, quotedCorrectionDeadlines())
	}
	return nil
}

// GuardOrderCorrectionReassign validates the owner's reassignment: the
// derived state is OPEN (the closed corrections refuse — a NEW
// correction is the ladder by a NEW blocked decision, not a
// reassignment), the new owner is a valid member reference and the new
// deadline word belongs to the dictionary. The reassignment is a NEW
// reassigned event with a NEW epoch — the former owner's stale-epoch
// attempts answer the honest «коррекция переназначена».
func GuardOrderCorrectionReassign(state string, owner id.UUID, deadline string) error {
	if !OrderCorrectionStateActive(state) {
		return fmt.Errorf("переназначение невозможно из состояния «%s»: коррекция закрыта — новая коррекция строится по новому заблокированному решению (лестница повторной коррекции)", state)
	}
	if !owner.Valid() {
		return fmt.Errorf("owner обязателен (UUID): назначьте участника с полномочием review")
	}
	if !CorrectionDeadlineInDictionary(deadline) {
		return fmt.Errorf("слово дедлайна коррекции «%s» не входит в утверждённую policy (%s)", deadline, quotedCorrectionDeadlines())
	}
	return nil
}

// GuardOrderCorrectionReread validates the corrected closure: the
// derived state is the OPEN pending round (an overdue round refuses —
// the deadline passed, the honest path is the owner's reassignment, the
// 4-7 precedent), the actor IS the owner (the server truth — the
// assignment never impersonates the owner), the CAS assignment epoch
// expectation is present, the reread external object ID is a non-empty
// bounded word and the pinned NEW confirmed version identity triple is
// complete. The VERSION validations (the last version, the confirmed
// state, the LIVE re-derivation fence against the version digest)
// belong to the app layer — this guard is the shape admission, never
// the fence itself.
func GuardOrderCorrectionReread(state string, actor, owner id.UUID, expectedEpoch int64, externalObjectID string, proposalID id.UUID, versionNo int64, confirmationID id.UUID) error {
	if state == OrderCorrectionStateOverdue {
		return fmt.Errorf("дедлайн коррекции истёк: закрытие невозможно — путь: переназначение владельцем")
	}
	if state != OrderCorrectionStatePending {
		return fmt.Errorf("запись перечита возможна только из состояния «коррекция открыта»: текущее состояние — «%s»", state)
	}
	if actor != owner {
		return fmt.Errorf("%s", OrderCorrectionNotOwnerWord)
	}
	if expectedEpoch <= 0 {
		return fmt.Errorf("expected_assignment обязателен (CAS эпохи назначения): перечитайте коррекцию и повторите команду")
	}
	if len(externalObjectID) < 1 || len(externalObjectID) > 200 {
		return fmt.Errorf("external_object_id обязателен (1..200 символов): укажите внешний идентификатор перечитанного заказа")
	}
	if !proposalID.Valid() {
		return fmt.Errorf("proposal_id обязателен (UUID): пин новой подтверждённой версии предложения")
	}
	if versionNo < 1 {
		return fmt.Errorf("version_no обязателен (>= 1): пин новой подтверждённой версии предложения")
	}
	if !confirmationID.Valid() {
		return fmt.Errorf("confirmation_id обязателен (UUID): пин нового подтверждения клиента")
	}
	return nil
}

// GuardOrderCorrectionCease validates the ceased closure: the derived
// state is the OPEN pending round (an overdue round refuses — the
// honest path is the reassignment), the actor IS the owner, the CAS
// assignment epoch expectation is present and the cessation reason word
// belongs to the fixed three-word dictionary. The manager never types a
// free reason. The client refusal NEVER becomes a replacement — the
// replacement requires the NEW confirmation of the reread path only.
func GuardOrderCorrectionCease(state string, actor, owner id.UUID, expectedEpoch int64, reason string) error {
	if state == OrderCorrectionStateOverdue {
		return fmt.Errorf("дедлайн коррекции истёк: закрытие невозможно — путь: переназначение владельцем")
	}
	if state != OrderCorrectionStatePending {
		return fmt.Errorf("прекращение возможно только из состояния «коррекция открыта»: текущее состояние — «%s»", state)
	}
	if actor != owner {
		return fmt.Errorf("%s", OrderCorrectionNotOwnerWord)
	}
	if expectedEpoch <= 0 {
		return fmt.Errorf("expected_assignment обязателен (CAS эпохи назначения): перечитайте коррекцию и повторите команду")
	}
	if !CorrectionCessationReasonInDictionary(reason) {
		return fmt.Errorf("слово причины прекращения «%s» вне словаря: %s", reason, quotedCorrectionCessationReasons())
	}
	return nil
}

// quotedCorrectionDeadlines renders the honest quoted list of the
// deadline words (the refusal material — never a free input).
func quotedCorrectionDeadlines() string {
	out := ""
	for index, word := range CorrectionDeadlines {
		if index > 0 {
			out += ", "
		}
		out += "«" + word + "»"
	}
	return out
}

// quotedCorrectionCessationReasons renders the honest quoted list of
// the cessation reason words (the refusal material — never a free
// input).
func quotedCorrectionCessationReasons() string {
	out := ""
	for index, word := range CorrectionCessationReasons {
		if index > 0 {
			out += ", "
		}
		out += "«" + word + "»"
	}
	return out
}

// ---------------------------------------------------------------------------
// The app-layer sentinel errors and the machine-readable conflicts (the
// word IS the message; the transport maps them by name)
// ---------------------------------------------------------------------------

var (
	// ErrOrderCorrectionNoBlockedDecision — the anchor refusal: no LAST
	// CLOSED blocked verification decision of the operation, no
	// correction (the correction is built over the blocked decision
	// only).
	ErrOrderCorrectionNoBlockedDecision = errors.New(OrderCorrectionNoBlockedWord)
	// ErrOrderCorrectionSupersededConfirmation — the old-confirmation
	// refusal (AC-15): the pinned confirmation belongs to a superseded
	// version — the repeat use of the old Confirmation blocks the route
	// BY NAME.
	ErrOrderCorrectionSupersededConfirmation = errors.New(OrderCorrectionSupersededWord)
	// ErrOrderCorrectionCompositionChanged — the live-composition fence
	// refusal (AC-13): the LIVE re-derivation diverged from the
	// confirmed version digest — the closure on an unconfirmed
	// composition is impossible; the EXPLICIT differences travel with
	// the screen. The same fence code as the dispatch ladder 4.5 and the
	// 4.7 fence — reused, never a new dispatch step.
	ErrOrderCorrectionCompositionChanged = errors.New(OrderCorrectionCompositionChangedWord)
	// ErrOrderCorrectionOpenGate — the structural authorization gate of
	// the NEW order while the correction is open (AC-9/AC-10): the
	// refusal happens BEFORE the reservation transaction — nothing is
	// consumed, the 3-6 ladder and the 3-4 superseded mechanics stay
	// untouched.
	ErrOrderCorrectionOpenGate = errors.New(OrderCorrectionOrderGateWord)
)

// OrderCorrectionActiveError is the honest one-open-correction conflict
// of the repeated open (the 409, the OrderVerificationActiveError
// precedent of 4.7): the CURRENT lifecycle words travel with the
// conflict — the derived state word with its honest text — so the
// operator sees what already lives instead of a bare refusal.
type OrderCorrectionActiveError struct {
	State     string
	StateText string
}

// Error renders the fixed refusal line (the words travel as the fields
// of the problem contract — the transport carries them, never a
// re-parse of this text).
func (e OrderCorrectionActiveError) Error() string { return OrderCorrectionActiveText }

// OrderCorrectionRacedError is the honest CAS refusal of the raced
// append (the parallel closure of the second manager, the
// reassignment): the raced command NEVER wrote — the CURRENT lifecycle
// words travel with the refusal (the honest «коррекция переназначена»
// family).
type OrderCorrectionRacedError struct {
	State     string
	StateText string
}

// Error renders the fixed refusal line (the current words ride the
// fields of the problem contract).
func (e OrderCorrectionRacedError) Error() string { return OrderCorrectionReassignedWord }

// OrderCorrectionAssignGateError is the structural 409 refusal of the
// new verification assignment while the correction is open (AC-2: the
// order stays blocked from the progression until the fix and the NEW
// verification — the new task is never created before the correction
// closes). The word IS the panel word — the transport maps the 409 BY
// NAME.
type OrderCorrectionAssignGateError struct{}

// Error renders the fixed gate line.
func (e OrderCorrectionAssignGateError) Error() string { return OrderCorrectionAssignGateWord }
