package domain

import (
	"fmt"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// Module action, story 4.7 — the manager order verification domain: the
// Delegated Task over the committed receipt of create_abcp_customer_order
// (4.6). The task is a LIFECYCLE RECORD anchored to the committed
// operation/receipt pair — NOT the 2-13 turn_ref anchor and NEVER a
// transfer of the case ownership (the 2-4 terms stay untouched): the
// machine mirrors the 2-13 precedents (the append-only events, the
// read-time derivations, the CAS epoch) over the ORDER anchor.
//
// The assignment and the acceptance are TWO different truths: назначение ≠
// принятие (AC-1). The assigned event names the assignee, the backup owner
// and the round's own deadline word; the acknowledge is the assignee's
// EXPLICIT act (the server truth — actor=assignee, never a spoof); the
// decision is TWO different actions (approved = the Human Verification
// Receipt with the material pins; blocked = the affected positions with a
// structured reason word); the expiry is a read-time word (overdue), never
// an automatic approval — the auto-approval event kind does not exist
// STRUCTURALLY. The fence honesty: Staffy sends NOTHING to any supplier
// (the OQ-3 structural probe — no supplier-send field, route or word
// exists in this contour) and the interface never pretends the ABCP-side
// fence is guaranteed (the honest word: the bypass by hands in ABCP is
// possible; Staffy guarantees its own contour only — AC-13).

// ---------------------------------------------------------------------------
// The honest words of the slice (the text, never only color)
// ---------------------------------------------------------------------------

// OrderVerificationAssignedWord is the state word text of the assigned
// round: the assignment is NOT the acceptance (AC-1) — the manager is
// named by the task and has NOT taken it yet.
const OrderVerificationAssignedWord = "назначен, ещё не принял: задача проверки адресована менеджеру и ждёт явного принятия"

// OrderVerificationAcknowledgedWord is the state word text of the accepted
// task: the manager took the ownership of the check (the acknowledge is
// the assignee's own act — the assignment or a sent notification never
// counts).
const OrderVerificationAcknowledgedWord = "принял: менеджер подтвердил владение проверкой — проверка идёт"

// OrderVerificationApprovedWord is the state word text of the confirmed
// check: the Human Verification Receipt is recorded with the material pins
// of the exact version.
const OrderVerificationApprovedWord = "проверка подтверждена: решение зафиксировано квитанцией проверки с точной версией заказа"

// OrderVerificationBlockedWord is the state word text of the blocked check:
// the progression is blocked, the correction contour is named (4-8 —
// outside this slice), the original evidence is never changed.
const OrderVerificationBlockedWord = "продвижение заказа заблокировано; запущена коррекция — контур 4-8; исходные доказательства не изменены"

// OrderVerificationOverdueWord is the read-time overdue word (AC-8): the
// deadline passed without a closing event — the honest wait, the backup
// owner by name, the next step is the owner's reassignment; the expiry is
// NEVER an automatic approval (structurally: the auto-approval event kind
// does not exist).
const OrderVerificationOverdueWord = "просрочен: дедлайн проверки истёк без решения — автоматического одобрения нет; следующий шаг — переназначение владельцем (резервный владелец виден на карточке)"

// OrderVerificationDeliveryWord is the honest notification boundary word
// (AC-2, the 2-13 Never precedent): the external delivery channel does
// not exist in this slice — the notification is this surface's card; the
// three truths «назначен ≠ доставлено ≠ принял» are THREE different words.
const OrderVerificationDeliveryWord = "назначен ≠ доставлено ≠ принял: внешнего канала доставки уведомления нет — карточка этой поверхности и есть уведомление; доставка отдельно не фиксируется"

// OrderVerificationManualContourWord is the MANDATORY word of the approved
// render (AC-10/AC-11): after the confirmed check Staffy sends NOTHING to
// the supplier — the formation and the send stay the manager's MANUAL acts
// in the store's actual contour; the subsequent status is read separately
// from the Owning System (the honest 1C-unknown word of 4.6 rides the
// same screen).
const OrderVerificationManualContourWord = "формирование и отправка поставщику — ручное действие менеджера в контуре магазина; Staffy не отправляет автоматически; статус читается отдельно из ABCP/1С"

// OrderVerificationFenceWord is the fence honesty word (AC-13): the
// interface never draws a fence it cannot guarantee — the bypass by hands
// in ABCP is possible; Staffy guarantees BY WORDS its own contour only
// (the supplier-send does not exist here and the task is visible).
const OrderVerificationFenceWord = "обход проверки руками в ABCP возможен — Staffy гарантирует словами только свой контур: автоматической отправки поставщику не существует, задача проверки видна на странице кейса"

// OrderVerificationCorrectionWord is the blocked-decision word of the
// correction launch (AC-12): the correction flow itself is the contour 4-8
// (outside this slice) — the launch is the record plus this word; the
// original evidence is never changed.
const OrderVerificationCorrectionWord = "продвижение заказа заблокировано; запущена коррекция — контур 4-8; исходные доказательства не изменены"

// OrderVerificationPurchasingHiddenWord is the honest hiding word of the
// purchasing price without the Commercial scope (AC-4, the 4-5 precedent
// of the optional internal data): the hiding is a WORD, never a blank.
const OrderVerificationPurchasingHiddenWord = "закупочные данные — внутренние данные магазина; без полномочия не раскрываются"

// OrderVerificationInapplicableWord is the read-time inapplicability word
// (AC-14): the material version changed AFTER the approval — the former
// decision is not applicable to the new version, the changed fields show
// explicitly, the repeated FULL check is required.
const OrderVerificationInapplicableWord = "неприменимо — состав заказа изменился; прежнее решение неприменимо к этой версии: требуется повторная полная проверка"

// OrderVerificationCompositionChangedWord is the decision-fence refusal
// word (AC-9): the live re-derivation of the order digest diverged from
// the pinned version of the operation — the decision is refused BY NAME,
// the explicit field differences show on the screen, the repeated full
// check is required. The same fence code as the dispatch ladder 4.5 —
// reused, never a new dispatch step.
const OrderVerificationCompositionChangedWord = "состав заказа изменился — решение неприменимо к этой версии; требуется повторная полная проверка"

// OrderVerificationNotAcknowledgedWord is the honest refusal word of a
// decision without the acceptance (AC-7): the manager has NOT taken the
// task — the assignment is not the acceptance.
const OrderVerificationNotAcknowledgedWord = "менеджер ещё не принял задачу проверки: назначение не считается принятием — подтвердите владение проверкой"

// OrderVerificationActiveText is the honest one-active-task conflict word
// (the 409 of the repeated assign — the HandoffActiveText precedent of
// 2.13): the second competing task of one operation is never created.
const OrderVerificationActiveText = "проверка заказа уже активна — дождитесь решения либо переназначьте: вторая конкурирующая задача не создаётся"

// OrderVerificationReassignedWord is the honest CAS refusal word of the
// stale epoch (AC-7): the task was reassigned — the former assignee's
// acknowledge/decision attempts answer the CURRENT state honestly.
const OrderVerificationReassignedWord = "задача переназначена: ожидаемая эпоха назначения устарела — перечитайте задачу и повторите команду"

// OrderVerificationNoReceiptWord is the honest anchor refusal word of the
// assign without a committed receipt: the task is built over the CREATED
// order only — the unknown/failed/dispatch_blocked contours belong to
// 3-7/3-8 and the AG-4 recommendation already lives on the 4-6 panel.
const OrderVerificationNoReceiptWord = "нет подтверждённой квитанции заказа — проверять нечего: задача строится над созданным заказом"

// OrderVerificationNotAssigneeWord is the honest actor refusal word: only
// the accepted assignee acknowledges or decides (the server truth — the
// owner's assignment never impersonates the assignee).
const OrderVerificationNotAssigneeWord = "подтвердить или решить задачу может только назначенный и принявший менеджер: назначение не считается принятием"

// ---------------------------------------------------------------------------
// The deadline dictionary (the approved policy v0 of the slice)
// ---------------------------------------------------------------------------

// The verification deadline words: the approved policy dictionary of the
// assignment rounds. A deadline is a WORD with a fixed honest text and a
// fixed window — the machine gates the word, never parses the text and
// never breeds a deadline of its own (the «дедлайн не выдумывается»
// honesty of 2.4/2.13 carried into the verification contour).
const (
	Verify4h = "verify_4h"
	Verify1d = "verify_1d"
)

// VerifyDeadlines is the full dictionary (the canonical order).
var VerifyDeadlines = []string{Verify4h, Verify1d}

// VerifyDeadlineTexts is the honest text of every deadline word (the
// render material of the panel and the assignment card).
var VerifyDeadlineTexts = map[string]string{
	Verify4h: "Проверка заказа в течение четырёх часов.",
	Verify1d: "Проверка заказа в течение суток.",
}

// VerifyDeadlineText resolves the honest text of one deadline word.
func VerifyDeadlineText(word string) string {
	return VerifyDeadlineTexts[word]
}

// VerifyDeadlineInDictionary reports whether the deadline word belongs to
// the approved policy dictionary.
func VerifyDeadlineInDictionary(word string) bool {
	return WordIn(VerifyDeadlines, word)
}

// VerifyDeadlineWindow resolves the time window of one deadline word (the
// zero duration of an unknown word — the guards refuse the word before
// the window is ever read).
func VerifyDeadlineWindow(word string) time.Duration {
	switch word {
	case Verify4h:
		return 4 * time.Hour
	case Verify1d:
		return 24 * time.Hour
	}
	return 0
}

// ---------------------------------------------------------------------------
// The blocked-decision reason dictionary (five structured words)
// ---------------------------------------------------------------------------

// The block reasons of the «Нужна коррекция» decision: the structured
// five-word dictionary — the manager never types a free reason (the word
// is the dictionary constant; the SQL mirror of 00006 is the backstop).
const (
	VerifyBlockReasonWrongPart           = "wrong_part"
	VerifyBlockReasonWrongVehicle        = "wrong_vehicle"
	VerifyBlockReasonPriceChanged        = "price_changed"
	VerifyBlockReasonAvailabilityChanged = "availability_changed"
	VerifyBlockReasonConfirmationIssue   = "confirmation_issue"
)

// VerifyBlockReasons is the full dictionary (the canonical order).
var VerifyBlockReasons = []string{
	VerifyBlockReasonWrongPart,
	VerifyBlockReasonWrongVehicle,
	VerifyBlockReasonPriceChanged,
	VerifyBlockReasonAvailabilityChanged,
	VerifyBlockReasonConfirmationIssue,
}

// VerifyBlockReasonTexts is the honest text of every reason word (the
// render material of the decision form and the history).
var VerifyBlockReasonTexts = map[string]string{
	VerifyBlockReasonWrongPart:           "Неподходящая деталь: позиция не соответствует запросу/автомобилю — нужна коррекция подбора.",
	VerifyBlockReasonWrongVehicle:        "Ошибка автомобиля: подтверждённый профиль не соответствует фактическому — нужна коррекция подбора.",
	VerifyBlockReasonPriceChanged:        "Цена изменилась: условия предложения не актуальны — нужна коррекция подбора.",
	VerifyBlockReasonAvailabilityChanged: "Наличие изменилось: позиция недоступна в подтверждённом объёме — нужна коррекция подбора.",
	VerifyBlockReasonConfirmationIssue:   "Проблема подтверждения: подтверждение клиента не согласуется с материалом — нужна коррекция подбора.",
}

// VerifyBlockReasonText resolves the honest text of one reason word.
func VerifyBlockReasonText(word string) string {
	return VerifyBlockReasonTexts[word]
}

// VerifyBlockReasonInDictionary reports whether the reason word belongs
// to the fixed dictionary.
func VerifyBlockReasonInDictionary(word string) bool {
	return WordIn(VerifyBlockReasons, word)
}

// ---------------------------------------------------------------------------
// The lifecycle event kinds (the transition vocabulary)
// ---------------------------------------------------------------------------

// The verification lifecycle event kinds: one event — one kind; the
// states never mix. assigned names the assignee, the backup and the
// deadline (the assignment round — its event_no IS the assignment epoch,
// the CAS token of every later command); acknowledged carries no
// material (the manager identity and the time are the actor columns);
// approved carries the Human Verification Receipt pins (the order digest
// of the exact version, the receipt identity, the external ABCP Order ID
// and the evidence anchors); blocked carries the affected positions and
// the reason word. The AUTO-APPROVAL kind does not exist — the expiry
// never writes a decision (AC-8 structurally).
const (
	OrderVerificationEventAssigned     = "assigned"
	OrderVerificationEventAcknowledged = "acknowledged"
	OrderVerificationEventApproved     = "approved"
	OrderVerificationEventBlocked      = "blocked"
)

// OrderVerificationEventKinds is the full dictionary (the lifecycle order).
var OrderVerificationEventKinds = []string{
	OrderVerificationEventAssigned, OrderVerificationEventAcknowledged,
	OrderVerificationEventApproved, OrderVerificationEventBlocked,
}

// OrderVerificationEventKindInDictionary reports whether the kind word
// belongs to the fixed dictionary (the event write is fail-closed on it).
func OrderVerificationEventKindInDictionary(kind string) bool {
	return WordIn(OrderVerificationEventKinds, kind)
}

// ---------------------------------------------------------------------------
// The lifecycle state dictionary (the read-time words of the panel)
// ---------------------------------------------------------------------------

// The verification lifecycle states: the DERIVED words of the moment (the
// last event of the task plus the honest time derivation). assigned — the
// round is open and the deadline has not passed; acknowledged — the
// assignee took the check and the deadline has not passed; overdue — the
// round's deadline passed without a closing decision (the derivation,
// never a record; the automatic approval does not exist structurally);
// approved and blocked — the closed decisions (the history is the truth).
const (
	OrderVerificationStateAssigned     = "assigned"
	OrderVerificationStateAcknowledged = "acknowledged"
	OrderVerificationStateOverdue      = "overdue"
	OrderVerificationStateApproved     = "approved"
	OrderVerificationStateBlocked      = "blocked"
)

// OrderVerificationStates is the full dictionary (the lifecycle order).
var OrderVerificationStates = []string{
	OrderVerificationStateAssigned, OrderVerificationStateAcknowledged,
	OrderVerificationStateOverdue, OrderVerificationStateApproved, OrderVerificationStateBlocked,
}

// OrderVerificationStateTexts is the honest text of every state word (the
// render material of the «Проверка заказа» panel; the states are words,
// never colors).
var OrderVerificationStateTexts = map[string]string{
	OrderVerificationStateAssigned:     OrderVerificationAssignedWord,
	OrderVerificationStateAcknowledged: OrderVerificationAcknowledgedWord,
	OrderVerificationStateOverdue:      OrderVerificationOverdueWord,
	OrderVerificationStateApproved:     OrderVerificationApprovedWord,
	OrderVerificationStateBlocked:      OrderVerificationBlockedWord,
}

// OrderVerificationStateText resolves the honest text of one state word.
func OrderVerificationStateText(word string) string {
	return OrderVerificationStateTexts[word]
}

// OrderVerificationStateInDictionary reports whether the state word
// belongs to the fixed dictionary.
func OrderVerificationStateInDictionary(word string) bool {
	return WordIn(OrderVerificationStates, word)
}

// orderVerificationOpenStates are the OPEN (active) states of the
// lifecycle: the task exists without a closing decision (approved /
// blocked). The overdue round stays open — the expiry closed nothing (the
// fallback is the owner's explicit reassignment, never an auto-decision).
var orderVerificationOpenStates = map[string]bool{
	OrderVerificationStateAssigned:     true,
	OrderVerificationStateAcknowledged: true,
	OrderVerificationStateOverdue:      true,
}

// OrderVerificationStateActive reports whether the derived state word is
// an OPEN state of the lifecycle (the one-active-task truth: an active
// task of one operation refuses the repeated assign with the honest 409).
func OrderVerificationStateActive(word string) bool {
	return orderVerificationOpenStates[word]
}

// ---------------------------------------------------------------------------
// The material (the store reads/writes; the lifecycle is derived)
// ---------------------------------------------------------------------------

// OrderVerification is one append-only task row of the case: the task
// number (the monotonic per-case ladder — the repeated verification of
// one operation after a decision is a NEW task, the history is never
// rewritten), the ANCHOR triple (the committed operation of the order
// action, the receipt version the verification addresses, the external
// ABCP Order ID of the receipt references — the readback material of the
// screen), the work item anchor and the creator. The task NEVER carries
// the case ownership (the 2-4 terms stay untouched — the task lives over
// the order, not over the reply).
type OrderVerification struct {
	TenantID         id.UUID
	CaseID           id.UUID
	WorkItemID       id.UUID
	VerificationNo   int64
	OperationID      id.UUID
	ReceiptVersionNo int64
	ExternalOrderID  string
	CreatorID        id.UUID
	CreatedAt        time.Time
}

// OrderVerificationEvent is one append-only lifecycle event row of one
// task: the monotonic event number (the assignment epoch when the kind is
// assigned — the CAS expectation of the later commands), the kind word
// and the per-kind material — assigned carries the assignee, the backup
// owner and the deadline word with its time; acknowledged carries the
// manager identity and the time only; approved carries the Human
// Verification Receipt pins (the 64-hex order digest of the exact
// version, the receipt identity, the external ABCP Order ID and the
// evidence anchors — the SAFE pin set only); blocked carries the affected
// position keys and the reason word. The numbers are the server truth
// (the store computes the next monotonic number under the locks — the
// events never carry them).
type OrderVerificationEvent struct {
	TenantID         id.UUID
	CaseID           id.UUID
	VerificationNo   int64
	EventNo          int64
	Kind             string
	Assignee         id.UUID
	BackupOwner      id.UUID
	VerifyDeadline   string
	DeadlineAt       time.Time
	OrderDigest      string
	ReceiptVersionNo int64
	ExternalOrderID  string
	EvidenceAnchors  []string
	Positions        []string
	Reason           string
	ActorID          id.UUID
	CreatedAt        time.Time
}

// ---------------------------------------------------------------------------
// The read-time lifecycle derivation (never a stored status)
// ---------------------------------------------------------------------------

// OrderVerificationStateView is the derived lifecycle state of one task:
// the task row, the last event, the state word, the material of the
// CURRENT assignment round (the assignee, the backup and the deadline of
// the LATEST assigned event — the round the closing events answer) and
// the closing decision material (the approved/blocked pins). Everything
// is a read — the overdue state is the honest time derivation of the
// open round, never a stored status and never a worker's write.
type OrderVerificationStateView struct {
	Verification    OrderVerification
	LastEvent       *OrderVerificationEvent
	State           string
	Assignee        id.UUID
	BackupOwner     id.UUID
	VerifyDeadline  string
	DeadlineAt      time.Time
	AssignmentEpoch int64
	Decision        *OrderVerificationEvent
}

// DeriveOrderVerificationState derives the read-time lifecycle state of
// one task: the LAST event (the maximal event number — the append-only
// history is never rewritten; the LATER row of an equal number wins the
// defensive tie, the table UNIQUE backstop makes the case unreachable)
// decides the base word, and the honest time derivation refines it — an
// open round (assigned or acknowledged) whose deadline has passed without
// a closing decision is OVERDUE (the derivation of the moment; the expiry
// is NEVER an automatic approval — no event kind can express it, AC-8
// structurally). The assignment round material derives from the LATEST
// assigned event (a reassignment is a NEW round with a NEW epoch).
func DeriveOrderVerificationState(task OrderVerification, events []OrderVerificationEvent, now time.Time) OrderVerificationStateView {
	view := OrderVerificationStateView{Verification: task}
	var last *OrderVerificationEvent
	var assigned *OrderVerificationEvent
	var decision *OrderVerificationEvent
	for i := range events {
		event := &events[i]
		if event.CaseID != task.CaseID || event.VerificationNo != task.VerificationNo {
			continue
		}
		if last == nil || event.EventNo >= last.EventNo {
			last = event
		}
		if event.Kind == OrderVerificationEventAssigned {
			if assigned == nil || event.EventNo >= assigned.EventNo {
				assigned = event
			}
		}
		if event.Kind == OrderVerificationEventApproved || event.Kind == OrderVerificationEventBlocked {
			if decision == nil || event.EventNo >= decision.EventNo {
				decision = event
			}
		}
	}
	if last != nil {
		view.LastEvent = last
		switch last.Kind {
		case OrderVerificationEventAssigned, OrderVerificationEventAcknowledged:
			deadline := time.Time{}
			if assigned != nil {
				deadline = assigned.DeadlineAt
			}
			if !deadline.IsZero() && now.After(deadline) {
				view.State = OrderVerificationStateOverdue
			} else if last.Kind == OrderVerificationEventAssigned {
				view.State = OrderVerificationStateAssigned
			} else {
				view.State = OrderVerificationStateAcknowledged
			}
		case OrderVerificationEventApproved:
			view.State = OrderVerificationStateApproved
		case OrderVerificationEventBlocked:
			view.State = OrderVerificationStateBlocked
		}
	}
	if assigned != nil {
		view.Assignee = assigned.Assignee
		view.BackupOwner = assigned.BackupOwner
		view.VerifyDeadline = assigned.VerifyDeadline
		view.DeadlineAt = assigned.DeadlineAt
		view.AssignmentEpoch = assigned.EventNo
	}
	if decision != nil {
		view.Decision = decision
	}
	return view
}

// Active reports whether the derived lifecycle is OPEN (the task exists
// without a closing decision): the one-active-task truth — a repeated
// assign of the same operation is the honest 409 while the current task
// lives.
func (v OrderVerificationStateView) Active() bool {
	return OrderVerificationStateActive(v.State)
}

// ---------------------------------------------------------------------------
// The decision applicability (the read-time honesty of the former
// decision against the LIVE material version)
// ---------------------------------------------------------------------------

// OrderVerificationApplicability is the derived applicability of the
// former approved decision against the LIVE order digest (AC-14): the
// decision applied to the pinned version; a changed material version
// (any of the five inputs, a re-recorded offer, a re-selection) makes it
// inapplicable — the changed fields show explicitly on the screen, the
// repeated FULL check is required.
type OrderVerificationApplicability struct {
	Applicable bool
	Word       string
}

// DeriveOrderVerificationApplicability derives the read-time
// applicability of a recorded decision: the LIVE digest equals the
// pinned digest of the approved event — the decision still addresses
// the current material version; a divergence — the decision is not
// applicable to this version anymore (the honest word; the NEW task is
// the owner's decision, never an automatic one).
func DeriveOrderVerificationApplicability(pinnedDigest, liveDigest string) OrderVerificationApplicability {
	if pinnedDigest != "" && pinnedDigest == liveDigest {
		return OrderVerificationApplicability{Applicable: true}
	}
	return OrderVerificationApplicability{Applicable: false, Word: OrderVerificationInapplicableWord}
}

// ---------------------------------------------------------------------------
// The record guards (the domain truth before SQL)
// ---------------------------------------------------------------------------

// GuardOrderVerificationAssignment validates the assignment round: the
// assignee and the backup are valid member references (the membership
// and the Review-scope truth belong to the LIVE identity read of the app
// layer); the round's deadline word belongs to the approved policy
// dictionary. The assignment NEVER moves the case ownership (AC-1:
// назначение ≠ принятие — the 2-4 terms stay untouched).
func GuardOrderVerificationAssignment(assignee, backup id.UUID, deadline string) error {
	if !assignee.Valid() {
		return fmt.Errorf("assignee обязателен (UUID): назначьте участника с полномочием review")
	}
	if !backup.Valid() {
		return fmt.Errorf("backup_owner обязателен (UUID): укажите резервного участника раунда проверки")
	}
	if !VerifyDeadlineInDictionary(deadline) {
		return fmt.Errorf("слово дедлайна проверки «%s» не входит в утверждённую policy (%s)", deadline, quotedVerifyDeadlines())
	}
	return nil
}

// GuardOrderVerificationAcknowledge validates the explicit acceptance:
// the derived state is the OPEN assigned round (an overdue round refuses
// — the deadline passed, the honest path is the owner's reassignment,
// the 2-13 precedent); the actor IS the assignee (the server truth —
// the assignment or a notification is never an acceptance, AC-1/AC-2);
// the CAS assignment epoch expectation is present (the event_no of the
// latest assigned round).
func GuardOrderVerificationAcknowledge(state string, actor, assignee id.UUID, expectedEpoch int64) error {
	if state == OrderVerificationStateOverdue {
		return fmt.Errorf("дедлайн проверки истёк: принятие невозможно — путь: переназначение владельцем (резервный владелец виден на карточке)")
	}
	if state != OrderVerificationStateAssigned {
		return fmt.Errorf("принятие возможно только из состояния «назначен»: текущее состояние — «%s»", state)
	}
	if actor != assignee {
		return fmt.Errorf("%s", OrderVerificationNotAssigneeWord)
	}
	if expectedEpoch <= 0 {
		return fmt.Errorf("expected_assignment обязателен (CAS эпохи назначения): перечитайте задачу и повторите команду")
	}
	return nil
}

// GuardOrderVerificationApprove validates the confirmed check: the
// derived state is the acknowledged acceptance (the manager took the
// task — a decision without the acceptance is the honest refusal of
// AC-7), the actor is the assignee, the CAS epoch is present and the
// expected order digest is the 64-hex pin of the exact version. The
// LIVE re-derivation fence (the same code as the dispatch ladder 4.5)
// runs in the app service BEFORE the event is written — this guard is
// the shape admission, never the fence itself.
func GuardOrderVerificationApprove(state string, actor, assignee id.UUID, expectedEpoch int64, expectedDigest string) error {
	if state == OrderVerificationStateOverdue {
		return fmt.Errorf("дедлайн проверки истёк: решение невозможно — путь: переназначение владельцем (резервный владелец виден на карточке)")
	}
	if state == OrderVerificationStateAssigned {
		return fmt.Errorf("%s", OrderVerificationNotAcknowledgedWord)
	}
	if state != OrderVerificationStateAcknowledged {
		return fmt.Errorf("решение возможно только из состояния «принял»: текущее состояние — «%s»", state)
	}
	if actor != assignee {
		return fmt.Errorf("%s", OrderVerificationNotAssigneeWord)
	}
	if expectedEpoch <= 0 {
		return fmt.Errorf("expected_assignment обязателен (CAS эпохи назначения): перечитайте задачу и повторите команду")
	}
	if !orderPinHex(expectedDigest) {
		return fmt.Errorf("expected_order_digest обязателен (64-hex пин точной версии): перечитайте материал и повторите команду")
	}
	return nil
}

// GuardOrderVerificationBlock validates the correction decision: the
// derived state is the acknowledged acceptance, the actor is the
// assignee, the CAS epoch is present, the affected positions are a
// non-empty duplicate-free list inside the order bound (the ⊆ operation
// positions check belongs to the app layer — it reads the LIVE
// operation material) and the reason word belongs to the fixed
// five-word dictionary. The manager never types a free reason.
func GuardOrderVerificationBlock(state string, actor, assignee id.UUID, expectedEpoch int64, positions []string, reason string) error {
	if state == OrderVerificationStateOverdue {
		return fmt.Errorf("дедлайн проверки истёк: решение невозможно — путь: переназначение владельцем (резервный владелец виден на карточке)")
	}
	if state == OrderVerificationStateAssigned {
		return fmt.Errorf("%s", OrderVerificationNotAcknowledgedWord)
	}
	if state != OrderVerificationStateAcknowledged {
		return fmt.Errorf("решение возможно только из состояния «принял»: текущее состояние — «%s»", state)
	}
	if actor != assignee {
		return fmt.Errorf("%s", OrderVerificationNotAssigneeWord)
	}
	if expectedEpoch <= 0 {
		return fmt.Errorf("expected_assignment обязателен (CAS эпохи назначения): перечитайте задачу и повторите команду")
	}
	if len(positions) < OrderPositionsMin || len(positions) > OrderPositionsMax {
		return fmt.Errorf("затронутые позиции обязательны (от %d до %d ключей позиций заказа): получено %d",
			OrderPositionsMin, OrderPositionsMax, len(positions))
	}
	seen := map[string]bool{}
	for _, position := range positions {
		if position == "" {
			return fmt.Errorf("ключ затронутой позиции не может быть пустым")
		}
		if seen[position] {
			return fmt.Errorf("позиция «%s» повторяется: дубль затронутой позиции не допускается", position)
		}
		seen[position] = true
	}
	if !VerifyBlockReasonInDictionary(reason) {
		return fmt.Errorf("слово причины коррекции «%s» вне словаря: %s", reason, quotedVerifyBlockReasons())
	}
	return nil
}

// GuardOrderVerificationReassign validates the owner's reassignment: the
// derived state is OPEN (the closed decisions refuse — a NEW task is the
// ladder, not a reassignment), the new assignee and the backup are valid
// member references and the new deadline word belongs to the dictionary.
// The reassignment is a NEW assigned event with a NEW epoch — the former
// assignee's stale-epoch attempts answer the honest «задача переназначена».
func GuardOrderVerificationReassign(state string, assignee, backup id.UUID, deadline string) error {
	if !OrderVerificationStateActive(state) {
		return fmt.Errorf("переназначение невозможно из состояния «%s»: задача закрыта решением — новая задача строится назначением (лестница повторной проверки)", state)
	}
	if !assignee.Valid() {
		return fmt.Errorf("assignee обязателен (UUID): назначьте участника с полномочием review")
	}
	if !backup.Valid() {
		return fmt.Errorf("backup_owner обязателен (UUID): укажите резервного участника раунда проверки")
	}
	if !VerifyDeadlineInDictionary(deadline) {
		return fmt.Errorf("слово дедлайна проверки «%s» не входит в утверждённую policy (%s)", deadline, quotedVerifyDeadlines())
	}
	return nil
}

// quotedVerifyDeadlines renders the honest quoted list of the deadline
// words (the refusal material — never a free input).
func quotedVerifyDeadlines() string {
	out := ""
	for index, word := range VerifyDeadlines {
		if index > 0 {
			out += ", "
		}
		out += "«" + word + "»"
	}
	return out
}

// quotedVerifyBlockReasons renders the honest quoted list of the reason
// words (the refusal material — never a free input).
func quotedVerifyBlockReasons() string {
	out := ""
	for index, word := range VerifyBlockReasons {
		if index > 0 {
			out += ", "
		}
		out += "«" + word + "»"
	}
	return out
}
