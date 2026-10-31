package domain

import (
        "errors"
        "fmt"
        "sort"
        "strings"
        "time"

        "github.com/staffy-co/staffy/internal/platform/id"
)

// Module action, story 4.9 — the Order Status Projection domain: the
// PURE read-time derivation of the five-stage order lifecycle over the
// append-only material of the three machines (the operation+receipt of
// 4.6, the verification events of 4.7, the correction events of 4.8) and
// the Supplier Order Fact Receipt of this slice. The projection writes
// NOTHING — «the computed representation never becomes a new source of
// truth» (AC-2, structural: there is no write in this file at all). The
// facts are the HUMAN receipts of the store contour {formed, sent}: the
// manager records what happened in ABCP by hand — NOTHING is ever sent
// or formed by Staffy (the OQ-3 fence: the kind words are the human
// claims, never commands; no supplier-send field, route or action
// exists here or anywhere).

// ---------------------------------------------------------------------------
// The honest words of the slice (the text, never only color)
// ---------------------------------------------------------------------------

// OrderStageClientCreatedWord is the stage word of the created order
// (AC-1/AC-3): the client order exists in the store with the dispatch
// receipt of the operation.
const OrderStageClientCreatedWord = "заказ клиента создан в магазине (квитанция операции)"

// OrderStagePendingVerificationWord is the stage word of the pending
// manager verification (AC-3): the task of 4.7 is open — assigned,
// acknowledged or overdue.
const OrderStagePendingVerificationWord = "заказ ожидает проверки менеджером (задача проверки открыта)"

// OrderStageManagerVerifiedWord is the stage word of the verified order
// (AC-3): the LAST closed decision of the 4.7 task is approved.
const OrderStageManagerVerifiedWord = "проверка менеджером подтверждена решением «Подтвердить проверку»"

// OrderStageSupplierFormedWord is the stage word of the formed supplier
// order (AC-3/AC-6): a manager recorded the formed fact with the
// supplier order ID — the human receipt, never a system claim.
const OrderStageSupplierFormedWord = "заказ поставщику сформирован (запись менеджера с номером заказа поставщика)"

// OrderStageSupplierSentWord is the stage word of the sent supplier
// order (AC-3/AC-6/AC-15): EVERY recorded formation is covered by a
// sent record — the honest aggregate, never a partial claim.
const OrderStageSupplierSentWord = "заказ поставщику отправлен (все записанные формирования покрыты записями отправки)"

// OrderStageNotConfirmedWord is the shared honest word of the
// unconfirmed stage (AC-4: the absence is spoken, never assumed).
const OrderStageNotConfirmedWord = "не подтверждено"

// OrderStageClientCreatedMissingWord is the honest word of the missing
// client-order receipt: the operation carries no committed receipt.
const OrderStageClientCreatedMissingWord = "нет квитанции операции заказа: стадия создания не подтверждается"

// OrderStatusManualContourWord is the honest AG-4 word of the blocked
// dispatch (the nil-adapter production contour of 4.6): the order is
// formed by hand in the store contour — the machine receipt never
// lands, the stage stays unconfirmed BY NAME.
const OrderStatusManualContourWord = "заказ создаётся вручную в контуре магазина (AG-4, рекомендация order_creation 4-6)"

// OrderStatusPendingMissingWord is the honest word of the missing
// verification task: the stage reads the task machine only.
const OrderStatusPendingMissingWord = "активной задачи проверки нет: назначьте проверку (контур 4-7)"

// OrderStatusVerifiedMissingWord is the honest word of the unapproved
// verification: the last closed decision governs the stage (the ladder
// down reads the history — an approved decision above a NEW blocked
// decision does not survive).
const OrderStatusVerifiedMissingWord = "подтверждённого решения проверки нет: последнее закрытое решение не «Подтвердить проверку»"

// OrderStatusFormedMissingWord is the honest word of the unrecorded
// formation (AC-4/AC-6): no human receipt — no claim.
const OrderStatusFormedMissingWord = "записи о формировании у поставщика нет: утверждение о формировании не делается"

// OrderStatusSentMissingWord is the honest word of the unrecorded send
// (AC-4/AC-15): no human receipt — no claim, the send is NEVER spoken
// on behalf of a formation that lacks a sent record.
const OrderStatusSentMissingWord = "запись об отправке поставщику отсутствует: утверждение об отправке не делается"

// OrderStatusPartialSentWord is the honest word of the PARTIAL send
// (AC-15): some formations lack a sent record — the stage stays
// unconfirmed, every formation is named.
const OrderStatusPartialSentWord = "отправлен частично: не все записанные формирования покрыты записями отправки — стадия «отправлен» не подтверждается"

// OrderStatusSentAfterFormedWord is the guard word of the record ladder
// (AC-6): the sent fact of one supplier order ID lands only after the
// formed fact of the SAME ID — the discipline of the full history, the
// times are named as they happened.
const OrderStatusSentAfterFormedWord = "отправление записывается после записи формирования того же заказа поставщику — сначала запишите формирование (дисциплина истории, время формирования назовите фактическим)"

// OrderStatusSupplierNotRecordedWord is the honest supplier word of no
// facts (AC-6): «не записано» — never «не сформирован» (the machine
// does not know the ABCP reality, AG-4).
const OrderStatusSupplierNotRecordedWord = "не записано: фактов формирования/отправки поставщику нет (Staffy не читает внешние системы — AG-4)"

// OrderStatusSupplierManagerSourceWord is the honest source word of
// every supplier fact (AC-5/AG-4): the human receipt of the store
// contour — never a system claim.
const OrderStatusSupplierManagerSourceWord = "записано менеджером: чтение ABCP адаптером не подключено (AG-4)"

// OrderStatusExternalIDDivergenceWord is the honest AC-8 word of the
// divergent external IDs: BOTH values are shown separately — the
// favorable version is never chosen.
const OrderStatusExternalIDDivergenceWord = "внешние идентификаторы расходятся: показаны оба значения раздельно со своими источниками — благоприятная версия не выбирается"

// OrderStatusBlockedNoCorrectionWord is the honest special-state word
// of the blocked decision without an open correction: the next step is
// the owner's call, never an automatic action.
const OrderStatusBlockedNoCorrectionWord = "проверка вернула «Нужна коррекцию»: коррекция ещё не открыта — решение за владельцем"

// OrderStatusCorrectionBoundaryWord is the cancellation boundary word
// (AC-12): the order cancellation lives in the contour 4-10 — this
// slice speaks the boundary only.
const OrderStatusCorrectionBoundaryWord = "отмена заказа — контур 4-10 (не в этом срезе): спецсостояние произносится словом-границей"

// OrderStatusDisputeBoundaryWord is the dispute boundary word (AC-14):
// the append-only history never rewrites; the source check is a human
// request (2-13); the recovery contour is 3-8.
const OrderStatusDisputeBoundaryWord = "исходное сообщение сохранено (append-only история); проверка источников — запрос человека (контур 2-13); новая версия статуса не переписывает историю; восстановление — контур 3-8"

// OrderStatusFreshnessWord is the freshness word (AC-10): the live
// derivation at the panel open — no workers, no schedule, the external
// reads are not connected.
const OrderStatusFreshnessWord = "живая деривация при открытии панели; внешние чтения ABCP/1С/поставщика не подключены (AG-4) — воркеров и расписания нет"

// OrderStatusNoScheduleWord is the honest no-schedule word (AC-10):
// the stale value is never presented as current AND the «next update
// time» never exists.
const OrderStatusNoScheduleWord = "времени следующего обновления по расписанию нет: расписание не существует (AD-14)"

// OrderStatusNoNotificationsWord is the honest notification word
// (AC-13, OQ-1/OQ-2 — the 2-13 Never precedent): the panel is the ONLY
// status surface; the delivery and the reading are never recorded.
const OrderStatusNoNotificationsWord = "уведомления о переходах не отправляются: канал не подключён (OQ-1/OQ-2); панель — единственная поверхность состояния; доставка и прочтение не фиксируются"

// OrderStatusDeadlineEstimateWord is the AC-11 estimate word: the
// offer term of the order material (4-6) — an estimate, never a
// guarantee of the store.
const OrderStatusDeadlineEstimateWord = "оценка срока предложения (материал 4-6): оценка не является гарантией магазина"

// OrderStatusDeadlineNoEstimateWord is the honest AC-11 word of the
// missing estimate material.
const OrderStatusDeadlineNoEstimateWord = "оценка срока предложения не записана в материале заказа"

// OrderStatusDeadlineConfirmedWord is the AC-11 confirmed-date word:
// the supplier confirmation does not exist in this slice — the honest
// negation, never a silent date.
const OrderStatusDeadlineConfirmedWord = "подтверждённая дата — не существует: подтверждения поставщиком нет"

// OrderStatusDeadlineActualWord is the AC-11 actual-event word: the
// sent record with its time — the fact, never a promise.
const OrderStatusDeadlineActualWord = "фактическое событие — запись отправки с временем (запись менеджера)"

// ErrOrderSupplierFactLadder is the sentinel of the record-ladder
// refusal (the guard word IS the message; the transport maps it).
var ErrOrderSupplierFactLadder = errors.New(OrderStatusSentAfterFormedWord)

// ErrOrderStatusNoOperation is the honest anchor refusal of the record
// command and the projection assembly: the case carries NO order
// operation of create_abcp_customer_order (the RLS blindness of a
// foreign case answers the same word — the 404).
var ErrOrderStatusNoOperation = errors.New("операция заказа не найдена: в этом кейсе нет операции create_abcp_customer_order")

// ---------------------------------------------------------------------------
// The dictionaries (compile-time; never a tenant configuration)
// ---------------------------------------------------------------------------

// The five lifecycle stages of the order (AC-3): created → pending
// manager verification → verified → formed at the supplier → sent to
// the supplier. «Formed» and «sent» are DIFFERENT states — the
// transition carries evidence only (the human receipt of this slice).
const (
        OrderStageClientOrderCreated       = "client_order_created"
        OrderStagePendingManagerVerification = "pending_manager_verification"
        OrderStageManagerVerified          = "manager_verified"
        OrderStageSupplierFormed           = "supplier_formed"
        OrderStageSupplierSent             = "supplier_sent"
)

// OrderStages is the full stage dictionary (the lifecycle order).
var OrderStages = []string{
        OrderStageClientOrderCreated, OrderStagePendingManagerVerification,
        OrderStageManagerVerified, OrderStageSupplierFormed, OrderStageSupplierSent,
}

// OrderStageWords is the honest word of every stage word.
var OrderStageWords = map[string]string{
        OrderStageClientOrderCreated:         OrderStageClientCreatedWord,
        OrderStagePendingManagerVerification: OrderStagePendingVerificationWord,
        OrderStageManagerVerified:            OrderStageManagerVerifiedWord,
        OrderStageSupplierFormed:             OrderStageSupplierFormedWord,
        OrderStageSupplierSent:               OrderStageSupplierSentWord,
}

// OrderStageInDictionary reports whether the stage word belongs to the
// fixed dictionary.
func OrderStageInDictionary(word string) bool {
        return WordIn(OrderStages, word)
}

// The supplier fact kinds (the human receipts of the store contour):
// formed — the manager formed the order at the supplier in ABCP; sent —
// the manager sent it. NO other kind exists (the OQ-3 fence: no
// supplier-send command word lives here — these are the recorded
// claims, never commands).
const (
        OrderSupplierFactFormed = "formed"
        OrderSupplierFactSent   = "sent"
)

// OrderSupplierFactKinds is the full kind dictionary.
var OrderSupplierFactKinds = []string{OrderSupplierFactFormed, OrderSupplierFactSent}

// OrderSupplierFactKindInDictionary reports whether the kind word
// belongs to the fixed dictionary.
func OrderSupplierFactKindInDictionary(kind string) bool {
        return WordIn(OrderSupplierFactKinds, kind)
}

// OrderSupplierFactKindWords is the honest word of every kind word.
var OrderSupplierFactKindWords = map[string]string{
        OrderSupplierFactFormed: "сформирован у поставщика (запись менеджера)",
        OrderSupplierFactSent:   "отправлен поставщику (запись менеджера)",
}

// The fact sources (AC-5): every fact names its source BY NAME — the
// system receipt of the dispatch machine (4.6) or the human record of
// the manager. The human receipt NEVER renders as a system one (AG-4).
const (
        OrderFactSourceSystemReceipt = "system_receipt"
        OrderFactSourceManagerRecord = "manager_record"
)

// OrderFactSources is the full source dictionary.
var OrderFactSources = []string{OrderFactSourceSystemReceipt, OrderFactSourceManagerRecord}

// OrderFactSourceInDictionary reports whether the source word belongs
// to the fixed dictionary.
func OrderFactSourceInDictionary(word string) bool {
        return WordIn(OrderFactSources, word)
}

// OrderFactSourceWords is the honest word of every source word (the
// per-system render of the panel).
var OrderFactSourceWords = map[string]string{
        OrderFactSourceSystemReceipt: "системная квитанция диспетчерского контура",
        OrderFactSourceManagerRecord: "запись менеджера (человеческая квитанция контура магазина)",
}

// The per-system words of the projection (AC-2): ABCP, 1C and the
// supplier are projected SEPARATELY — one source per system, the
// unknown stays unknown.
const (
        OrderStatusSystemABCP     = "abcp"
        OrderStatusSystemOneC     = "one_c"
        OrderStatusSystemSupplier = "supplier"
)

// OrderStatusSystems is the system dictionary of the projection (the
// render order of the panel).
var OrderStatusSystems = []string{OrderStatusSystemABCP, OrderStatusSystemOneC, OrderStatusSystemSupplier}

// The supplier-order-ID length bounds (the typed field of the record
// command; the CHECK of the migration mirrors them).
const (
        OrderSupplierOrderIDMinLen = 1
        OrderSupplierOrderIDMaxLen = 200
)

// ---------------------------------------------------------------------------
// The material (the store reads/writes; the projection derives)
// ---------------------------------------------------------------------------

// OrderSupplierFact is ONE append-only human-receipt row of the case:
// the fact number (the monotonic per-case ladder — the server truth of
// the fold order), the ANCHOR operation of the order contour, the kind
// word, the supplier order ID (the external ID named by the manager),
// the named happened_at time and the server recorded_at time (the two
// times never mix — the provider-observed precedent of 2-7) and the
// actor identity.
type OrderSupplierFact struct {
        TenantID        id.UUID
        CaseID          id.UUID
        WorkItemID      id.UUID
        FactNo          int64
        OperationID     id.UUID
        Kind            string
        SupplierOrderID string
        HappenedAt      time.Time
        ActorID         id.UUID
        RecordedAt      time.Time
}

// OrderStatusReceiptPin is the committed-receipt pin of the material:
// the version number, the external ABCP Order ID of the references and
// the server time (the evidence of the client_order_created stage —
// the system receipt of the dispatch machine).
type OrderStatusReceiptPin struct {
        ReceiptVersionNo int64
        ExternalOrderID  string
        CreatedAt        time.Time
}

// OrderStatusOperationMaterial is the operation anchor material: the
// operation exists and belongs to THIS case (Found), the work item
// anchor, the committed receipt pin (nil when the dispatch never
// committed — the AG-4 manual contour or the honest failure) and the
// offer estimate material of the order composition (the conditions of
// the pinned material of 4-6 — the AC-11 estimate; empty when not
// readable).
type OrderStatusOperationMaterial struct {
        Found           bool
        OperationID     id.UUID
        WorkItemID      id.UUID
        DispatchBlocked bool
        Receipt         *OrderStatusReceiptPin
        OfferEstimate   string
}

// OrderStatusVerificationMaterial is ONE verification task of the
// operation with its append-only events (the raw read — the projection
// derives the states itself, the 4.7 derivation is reused BY FUNCTION).
type OrderStatusVerificationMaterial struct {
        Task   OrderVerification
        Events []OrderVerificationEvent
}

// OrderStatusCorrectionMaterial is ONE correction of the operation
// with its append-only events (the raw read).
type OrderStatusCorrectionMaterial struct {
        Correction OrderCorrection
        Events     []OrderCorrectionEvent
}

// OrderStatusMaterial is the WHOLE read-time input of the projection:
// the operation anchor, the verification tasks, the corrections and the
// supplier facts of the case. Everything is a READ — the projection is
// a pure function over this material (ZERO writes, AC-2). The slices
// may arrive in ANY order (the read order of the SQL): the derivation
// folds them by the MONOTONIC anchors — the shuffled input yields the
// SAME projection (AC-9).
type OrderStatusMaterial struct {
        Operation     OrderStatusOperationMaterial
        Verifications []OrderStatusVerificationMaterial
        Corrections   []OrderStatusCorrectionMaterial
        Facts         []OrderSupplierFact
}

// ---------------------------------------------------------------------------
// The guards of the record command (the honest ladder discipline)
// ---------------------------------------------------------------------------

// GuardOrderSupplierFact refuses the record command honestly: the kind
// word must belong to the dictionary, the supplier order ID must be a
// non-empty word within the typed bound, the happened_at must be named
// (never the zero time) and the sent fact of one supplier order ID
// requires the prior formed fact of the SAME ID (the record ladder —
// the discipline of the full history; the truth is recorded as it is,
// this guard never blocks a «check bypass»: the formation without an
// approved verification records fine and the panel shows BOTH facts).
func GuardOrderSupplierFact(kind, supplierOrderID string, happenedAt time.Time, existing []OrderSupplierFact) error {
        if !OrderSupplierFactKindInDictionary(kind) {
                return RuleError{"order_supplier_fact.kind", fmt.Sprintf(
                        "Вид факта обязан быть словом словаря {%s}.", strings.Join(OrderSupplierFactKinds, ", "))}
        }
        trimmed := strings.TrimSpace(supplierOrderID)
        if len(trimmed) < OrderSupplierOrderIDMinLen || len(trimmed) > OrderSupplierOrderIDMaxLen {
                return RuleError{"order_supplier_fact.supplier_order_id", fmt.Sprintf(
                        "Номер заказа поставщика обязателен (от %d до %d символов) — внешний идентификатор, названный менеджером.",
                        OrderSupplierOrderIDMinLen, OrderSupplierOrderIDMaxLen)}
        }
        if happenedAt.IsZero() {
                return RuleError{"order_supplier_fact.happened_at", "Время факта обязательно: менеджер называет фактическое время события (два времени никогда не смешиваются)."}
        }
        if kind == OrderSupplierFactSent {
                formed := false
                for _, fact := range existing {
                        if fact.Kind == OrderSupplierFactFormed && fact.SupplierOrderID == trimmed {
                                formed = true
                                break
                        }
                }
                if !formed {
                        return RuleError{"order_supplier_fact.sent_after_formed", OrderStatusSentAfterFormedWord}
                }
        }
        return nil
}

// ---------------------------------------------------------------------------
// The read-time projection (never a stored status)
// ---------------------------------------------------------------------------

// OrderStageEvidence is ONE stage of the lifecycle with its honest
// confirmation state and the evidence pins (the time, the source word
// BY NAME, the external ID and the owner of the open round). The
// unconfirmed stage carries its honest word — the absence is spoken,
// never assumed (AC-4).
type OrderStageEvidence struct {
        Stage      string
        Confirmed  bool
        Word       string
        Time       time.Time
        Source     string
        ExternalID string
        OwnerID    id.UUID
        Detail     string
}

// OrderStatusFactView is ONE per-system fact of the projection (the
// render material of the per-system block): the system word, the
// external ID, the two times, the source word BY NAME and the actor of
// the human record.
type OrderStatusFactView struct {
        System        string
        Kind          string
        ExternalID    string
        HappenedAt    time.Time
        RecordedAt    time.Time
        Source        string
        ActorID       id.UUID
        Word          string
}

// OrderStatusSystemView is the per-system projection of ONE system
// (AC-2): the system word, the honest word and the facts (the ABCP
// block carries the receipt and/or the reread; the supplier block
// carries the human facts; the 1C block carries the unknown word only).
type OrderStatusSystemView struct {
        System string
        Word   string
        Facts  []OrderStatusFactView
        // DivergentIDs is the honest AC-8 pin: the receipt external ID and
        // the reread external ID diverge — BOTH are shown, the favorable
        // version is never chosen.
        DivergentIDs bool
}

// OrderStatusTimelineEntry is ONE transition of the ordered fold (the
// render material of the transition feed): the time, the kind word of
// the source machine, the honest word, the source word BY NAME, the
// external ID and the anchor rank of the monotonic ordering.
type OrderStatusTimelineEntry struct {
        At         time.Time
        Machine    string
        Kind       string
        Word       string
        Source     string
        ExternalID string
}

// OrderStatusFreshness is the freshness block (AC-10): the time of the
// last confirmed fact, the live-derivation word and the no-schedule
// word (the stale value never renders as current).
type OrderStatusFreshness struct {
        LastConfirmedAt time.Time
        Word            string
        NoScheduleWord  string
}

// OrderStatusDeadlineBlock is the AC-11 block: the three SEPARATE
// words — the estimate (the offer material of 4-6), the confirmed date
// (which does not exist) and the actual event (the sent record with
// its time).
type OrderStatusDeadlineBlock struct {
        EstimateWord   string
        ConfirmedWord  string
        ActualWord     string
        ActualAt       time.Time
        ActualRecorded bool
}

// OrderStatusProjection is the WHOLE derived view of the order status
// (the pure function output — ZERO writes anywhere): the five stages
// with the evidence, the current stage word, the special state (the
// open correction is never hidden behind «в работе» — AC-12), the
// per-system projections, the freshness, the ordered timeline, the
// deadline block, the boundary words (the notifications, the dispute,
// the cancellation contour) and the client retell words.
type OrderStatusProjection struct {
        Stages        []OrderStageEvidence
        CurrentStage  string
        // OperationID is the anchor operation of the projection (the form
        // target of the record commands — the LIVE order operation of the
        // case; zero when the case carries no order).
        OperationID   id.UUID
        SpecialState  string
        SpecialOwner  id.UUID
        SpecialDetail string
        Systems       []OrderStatusSystemView
        Freshness     OrderStatusFreshness
        Timeline      []OrderStatusTimelineEntry
        Deadline      OrderStatusDeadlineBlock
        NotificationWord string
        DisputeWord      string
        CancellationWord string
        NextStep      string
        ClientWords   []string
}

// DeriveOrderStatusTimeline derives the ordered transition feed of the
// material: the receipt, the verification events, the correction
// events and the supplier facts — EVERY entry with its time, its kind
// word, its source BY NAME and its external ID. The order is the
// monotonic anchor fold (the machine rank, the ladder number, the
// event/fact number) with the DISPLAY time of the happened_at/created_
// at; a shuffled input yields the SAME feed (AC-9 — the fold reads the
// anchors, never the arrival order).
func DeriveOrderStatusTimeline(m OrderStatusMaterial) []OrderStatusTimelineEntry {
        entries := []OrderStatusTimelineEntry{}
        if m.Operation.Receipt != nil {
                entries = append(entries, OrderStatusTimelineEntry{
                        At:         m.Operation.Receipt.CreatedAt,
                        Machine:    "order",
                        Kind:       "receipt_committed",
                        Word:       OrderStageClientCreatedWord,
                        Source:     OrderFactSourceSystemReceipt,
                        ExternalID: m.Operation.Receipt.ExternalOrderID,
                })
        }
        tasks := append([]OrderStatusVerificationMaterial(nil), m.Verifications...)
        sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].Task.VerificationNo < tasks[j].Task.VerificationNo })
        for _, task := range tasks {
                events := append([]OrderVerificationEvent(nil), task.Events...)
                sort.SliceStable(events, func(i, j int) bool { return events[i].EventNo < events[j].EventNo })
                for _, event := range events {
                        word := event.Kind
                        external := event.ExternalOrderID
                        switch event.Kind {
                        case OrderVerificationEventAssigned:
                                word = "задача проверки назначена менеджеру (контур 4-7)"
                        case OrderVerificationEventAcknowledged:
                                word = "менеджер принял задачу проверки"
                        case OrderVerificationEventApproved:
                                word = OrderStageManagerVerifiedWord
                        case OrderVerificationEventBlocked:
                                word = OrderVerificationBlockedWord
                        }
                        entries = append(entries, OrderStatusTimelineEntry{
                                At:         event.CreatedAt,
                                Machine:    "verification",
                                Kind:       "verification_" + event.Kind,
                                Word:       word,
                                Source:     OrderFactSourceManagerRecord,
                                ExternalID: external,
                        })
                }
        }
        corrections := append([]OrderStatusCorrectionMaterial(nil), m.Corrections...)
        sort.SliceStable(corrections, func(i, j int) bool {
                return corrections[i].Correction.CorrectionNo < corrections[j].Correction.CorrectionNo
        })
        for _, correction := range corrections {
                events := append([]OrderCorrectionEvent(nil), correction.Events...)
                sort.SliceStable(events, func(i, j int) bool { return events[i].EventNo < events[j].EventNo })
                for _, event := range events {
                        word := event.Kind
                        external := event.ExternalObjectID
                        switch event.Kind {
                        case OrderCorrectionEventOpened:
                                word = "коррекция открыта над заблокированным решением (контур 4-8)"
                        case OrderCorrectionEventReassigned:
                                word = "коррекция переназначена"
                        case OrderCorrectionEventReread:
                                word = OrderCorrectionRereadWord
                        case OrderCorrectionEventCeased:
                                word = OrderCorrectionCeasedWord
                        }
                        entries = append(entries, OrderStatusTimelineEntry{
                                At:         event.CreatedAt,
                                Machine:    "correction",
                                Kind:       "correction_" + event.Kind,
                                Word:       word,
                                Source:     OrderFactSourceManagerRecord,
                                ExternalID: external,
                        })
                }
        }
        facts := append([]OrderSupplierFact(nil), m.Facts...)
        sort.SliceStable(facts, func(i, j int) bool { return facts[i].FactNo < facts[j].FactNo })
        for _, fact := range facts {
                entries = append(entries, OrderStatusTimelineEntry{
                        At:         fact.HappenedAt,
                        Machine:    "supplier_fact",
                        Kind:       "supplier_fact_" + fact.Kind,
                        Word:       OrderSupplierFactKindWords[fact.Kind] + " — " + fact.SupplierOrderID,
                        Source:     OrderFactSourceManagerRecord,
                        ExternalID: fact.SupplierOrderID,
                })
        }
        // The DISPLAY order: the happened/created time first (the honest
        // chronology of the facts), the monotonic anchor as the stable tie
        // (the same millisecond keeps the ladder order — the fold result is
        // byte-stable under the shuffled input, AC-9).
        sort.SliceStable(entries, func(i, j int) bool {
                if !entries[i].At.Equal(entries[j].At) {
                        return entries[i].At.Before(entries[j].At)
                }
                return orderStatusMachineRank(entries[i].Machine) < orderStatusMachineRank(entries[j].Machine)
        })
        return entries
}

// orderStatusMachineRank is the stable tie rank of the fold (the
// anchor order of the machines within one equal timestamp).
func orderStatusMachineRank(machine string) int {
        switch machine {
        case "order":
                return 0
        case "verification":
                return 1
        case "correction":
                return 2
        default:
                return 3
        }
}

// DeriveOrderStatusProjection derives the WHOLE read-time projection
// of the order status (the pure function — ZERO writes, AC-2). The
// five stages derive by name: the client_order_created reads the
// committed receipt (the system receipt of the dispatch machine; the
// AG-4 blocked dispatch speaks the manual-contour word and stays
// unconfirmed BY NAME); the pending_manager_verification reads the
// ACTIVE task of 4.7 (the open states with the owner and the deadline
// of the live derivation); the manager_verified reads the LAST CLOSED
// decision (the ladder down reads the history — a blocked decision
// above an older approved one governs); the supplier_formed reads the
// formed facts (the human receipts with the supplier order IDs); the
// supplier_sent reads the HONEST AGGREGATE (every recorded formation
// covered by a sent record of the same supplier order ID — the partial
// send names every formation and stays unconfirmed, AC-15). The
// special state never hides behind «в работе» (AC-12); the per-system
// blocks stay separate (AC-2) with the source BY NAME (AC-5) and the
// divergent external IDs BOTH shown (AC-8); the freshness carries the
// last confirmed fact and the no-schedule word (AC-10); the deadline
// block carries the three separate words (AC-11).
func DeriveOrderStatusProjection(m OrderStatusMaterial, now time.Time) OrderStatusProjection {
        projection := OrderStatusProjection{
                NotificationWord: OrderStatusNoNotificationsWord,
                DisputeWord:      OrderStatusDisputeBoundaryWord,
                CancellationWord: OrderStatusCorrectionBoundaryWord,
        }
        // The ordered fold inputs (AC-9): every machine slice sorts by its
        // monotonic anchor BEFORE the derivation reads it — the shuffled
        // input yields the SAME projection.
        tasks := append([]OrderStatusVerificationMaterial(nil), m.Verifications...)
        sort.SliceStable(tasks, func(i, j int) bool {
                return tasks[i].Task.VerificationNo < tasks[j].Task.VerificationNo
        })
        corrections := append([]OrderStatusCorrectionMaterial(nil), m.Corrections...)
        sort.SliceStable(corrections, func(i, j int) bool {
                return corrections[i].Correction.CorrectionNo < corrections[j].Correction.CorrectionNo
        })
        facts := append([]OrderSupplierFact(nil), m.Facts...)
        sort.SliceStable(facts, func(i, j int) bool { return facts[i].FactNo < facts[j].FactNo })

        // The verification fold: the active task material and the LAST
        // CLOSED decision (the max (task, event) anchor of the
        // approved/blocked events).
        var activeTask *OrderVerificationStateView
        var lastClosed *OrderVerificationEvent
        for i := range tasks {
                events := append([]OrderVerificationEvent(nil), tasks[i].Events...)
                sort.SliceStable(events, func(a, b int) bool { return events[a].EventNo < events[b].EventNo })
                view := DeriveOrderVerificationState(tasks[i].Task, events, now)
                if OrderVerificationStateActive(view.State) {
                        if activeTask == nil || tasks[i].Task.VerificationNo >= activeTask.Verification.VerificationNo {
                                taskCopy := tasks[i].Task
                                viewCopy := view
                                viewCopy.Verification = taskCopy
                                activeTask = &viewCopy
                        }
                }
                for j := range events {
                        event := events[j]
                        if event.Kind != OrderVerificationEventApproved && event.Kind != OrderVerificationEventBlocked {
                                continue
                        }
                        if lastClosed == nil ||
                                event.VerificationNo > lastClosed.VerificationNo ||
                                (event.VerificationNo == lastClosed.VerificationNo && event.EventNo >= lastClosed.EventNo) {
                                lastClosed = &events[j]
                        }
                }
        }
        // The correction fold: the active correction material and the LAST
        // reread external object ID (the AC-8 divergence pin).
        var activeCorrection *OrderCorrectionStateView
        var lastReread *OrderCorrectionEvent
        for i := range corrections {
                events := append([]OrderCorrectionEvent(nil), corrections[i].Events...)
                sort.SliceStable(events, func(a, b int) bool { return events[a].EventNo < events[b].EventNo })
                view := DeriveOrderCorrectionState(corrections[i].Correction, events, now)
                if OrderCorrectionStateActive(view.State) {
                        if activeCorrection == nil || corrections[i].Correction.CorrectionNo >= activeCorrection.Correction.CorrectionNo {
                                correctionCopy := corrections[i].Correction
                                viewCopy := view
                                viewCopy.Correction = correctionCopy
                                activeCorrection = &viewCopy
                        }
                }
                for j := range events {
                        event := events[j]
                        if event.Kind != OrderCorrectionEventReread {
                                continue
                        }
                        if lastReread == nil ||
                                event.CorrectionNo > lastReread.CorrectionNo ||
                                (event.CorrectionNo == lastReread.CorrectionNo && event.EventNo >= lastReread.EventNo) {
                                lastReread = &events[j]
                        }
                }
        }

        // Stage 1 — client_order_created (the receipt of the dispatch
        // machine; the AG-4 blocked dispatch stays unconfirmed BY NAME).
        stageCreated := OrderStageEvidence{
                Stage:     OrderStageClientOrderCreated,
                Confirmed: false,
                Word:      OrderStageNotConfirmedWord + ": " + OrderStageClientCreatedMissingWord,
        }
        if !m.Operation.Found {
                stageCreated.Word = OrderStageNotConfirmedWord + ": операции заказа в кейсе нет"
        } else if m.Operation.Receipt != nil {
                stageCreated.Confirmed = true
                stageCreated.Word = OrderStageClientCreatedWord
                stageCreated.Time = m.Operation.Receipt.CreatedAt
                stageCreated.Source = OrderFactSourceSystemReceipt
                stageCreated.ExternalID = m.Operation.Receipt.ExternalOrderID
        } else if m.Operation.DispatchBlocked {
                stageCreated.Word = OrderStageNotConfirmedWord + ": " + OrderStatusManualContourWord
        }
        projection.Stages = append(projection.Stages, stageCreated)

        // Stage 2 — pending_manager_verification (the ACTIVE task of 4.7).
        stagePending := OrderStageEvidence{
                Stage:     OrderStagePendingManagerVerification,
                Confirmed: false,
                Word:      OrderStageNotConfirmedWord + ": " + OrderStatusPendingMissingWord,
        }
        if activeTask != nil {
                stagePending.Confirmed = true
                stagePending.Word = OrderStagePendingVerificationWord + " — " + OrderVerificationStateText(activeTask.State)
                stagePending.Time = now
                stagePending.Source = OrderFactSourceManagerRecord
                if activeTask.LastEvent != nil {
                        stagePending.Time = activeTask.LastEvent.CreatedAt
                }
                stagePending.OwnerID = activeTask.Assignee
                if !activeTask.DeadlineAt.IsZero() {
                        stagePending.Detail = "дедлайн: " + activeTask.DeadlineAt.Format(time.RFC3339) + " (" + activeTask.VerifyDeadline + ")"
                }
        }
        projection.Stages = append(projection.Stages, stagePending)

        // Stage 3 — manager_verified (the LAST CLOSED decision governs —
        // the ladder down reads the history).
        stageVerified := OrderStageEvidence{
                Stage:     OrderStageManagerVerified,
                Confirmed: false,
                Word:      OrderStageNotConfirmedWord + ": " + OrderStatusVerifiedMissingWord,
        }
        if lastClosed != nil && lastClosed.Kind == OrderVerificationEventApproved {
                stageVerified.Confirmed = true
                stageVerified.Word = OrderStageManagerVerifiedWord
                stageVerified.Time = lastClosed.CreatedAt
                stageVerified.Source = OrderFactSourceManagerRecord
                stageVerified.ExternalID = lastClosed.ExternalOrderID
        }
        projection.Stages = append(projection.Stages, stageVerified)

        // Stage 4 — supplier_formed (the human receipts with the supplier
        // order IDs).
        formedFacts := []OrderSupplierFact{}
        sentFacts := []OrderSupplierFact{}
        for _, fact := range facts {
                switch fact.Kind {
                case OrderSupplierFactFormed:
                        formedFacts = append(formedFacts, fact)
                case OrderSupplierFactSent:
                        sentFacts = append(sentFacts, fact)
                }
        }
        stageFormed := OrderStageEvidence{
                Stage:     OrderStageSupplierFormed,
                Confirmed: false,
                Word:      OrderStageNotConfirmedWord + ": " + OrderStatusFormedMissingWord,
        }
        if len(formedFacts) > 0 {
                stageFormed.Confirmed = true
                stageFormed.Word = OrderStageSupplierFormedWord
                stageFormed.Time = formedFacts[0].HappenedAt
                stageFormed.Source = OrderFactSourceManagerRecord
                stageFormed.ExternalID = formedFacts[0].SupplierOrderID
                stageFormed.Detail = orderSupplierFactListWord(formedFacts)
        }
        projection.Stages = append(projection.Stages, stageFormed)

        // Stage 5 — supplier_sent (the HONEST AGGREGATE: every recorded
        // formation covered by a sent record of the SAME supplier order ID
        // — the partial send names every formation, AC-15).
        sentByKey := map[string]OrderSupplierFact{}
        for _, fact := range sentFacts {
                sentByKey[fact.SupplierOrderID] = fact
        }
        uncovered := []OrderSupplierFact{}
        covered := []OrderSupplierFact{}
        for _, formed := range formedFacts {
                if _, ok := sentByKey[formed.SupplierOrderID]; ok {
                        covered = append(covered, formed)
                } else {
                        uncovered = append(uncovered, formed)
                }
        }
        stageSent := OrderStageEvidence{
                Stage:     OrderStageSupplierSent,
                Confirmed: false,
                Word:      OrderStageNotConfirmedWord + ": " + OrderStatusSentMissingWord,
        }
        if len(formedFacts) > 0 && len(uncovered) == 0 {
                stageSent.Confirmed = true
                stageSent.Word = OrderStageSupplierSentWord
                latest := sentFacts[0].HappenedAt
                for _, fact := range sentFacts {
                        if fact.HappenedAt.After(latest) {
                                latest = fact.HappenedAt
                        }
                }
                stageSent.Time = latest
                stageSent.Source = OrderFactSourceManagerRecord
                stageSent.ExternalID = strings.Join(orderSupplierIDs(formedFacts), ", ")
        } else if len(formedFacts) > 0 && len(uncovered) > 0 {
                stageSent.Word = OrderStageNotConfirmedWord + ": " + OrderStatusPartialSentWord
                stageSent.Detail = orderPartialSentListWord(covered, uncovered)
        }
        projection.Stages = append(projection.Stages, stageSent)

        // The current stage: the HIGHEST confirmed stage of the lifecycle
        // (the special state rides separately — AC-12, never hidden).
        projection.CurrentStage = ""
        for _, stage := range projection.Stages {
                if stage.Confirmed {
                        projection.CurrentStage = stage.Stage
                }
        }
        // The anchor operation of the record form (zero when the case
        // carries no order — the honest miss).
        projection.OperationID = m.Operation.OperationID

        // The special state (AC-12): the open correction is spoken FIRST,
        // never hidden behind «в работе»; the blocked decision without an
        // open correction speaks the owner's-call word; the ceased
        // correction speaks the stays-blocked word.
        if activeCorrection != nil {
                projection.SpecialState = OrderCorrectionPendingWord
                projection.SpecialOwner = activeCorrection.Owner
                if !activeCorrection.DeadlineAt.IsZero() {
                        projection.SpecialDetail = "дедлайн коррекции: " + activeCorrection.DeadlineAt.Format(time.RFC3339) + " (" + activeCorrection.CorrectDeadline + ") — " + OrderCorrectionStateText(activeCorrection.State)
                }
        } else if lastClosed != nil && lastClosed.Kind == OrderVerificationEventBlocked {
                projection.SpecialState = OrderStatusBlockedNoCorrectionWord
        }

        // The per-system blocks (AC-2/AC-5/AC-8): ABCP (the receipt and/or
        // the reread — the source BY NAME, the divergent IDs BOTH), 1C
        // (unknown — OQ-4) and the supplier (the human facts or «не
        // записано»).
        abcp := OrderStatusSystemView{System: OrderStatusSystemABCP}
        if m.Operation.Receipt != nil {
                abcp.Word = "создан (квитанция диспетчерского контура)"
                abcp.Facts = append(abcp.Facts, OrderStatusFactView{
                        System:     OrderStatusSystemABCP,
                        Kind:       "receipt_committed",
                        ExternalID: m.Operation.Receipt.ExternalOrderID,
                        RecordedAt: m.Operation.Receipt.CreatedAt,
                        Source:     OrderFactSourceSystemReceipt,
                        Word:       "внешний ID ABCP Order из квитанции операции: " + m.Operation.Receipt.ExternalOrderID,
                })
        } else if m.Operation.DispatchBlocked {
                abcp.Word = OrderStatusManualContourWord
        } else if m.Operation.Found {
                abcp.Word = "квитанции операции заказа нет: заказ в ABCP не подтверждён ничем"
        } else {
                abcp.Word = "операции заказа в кейсе нет"
        }
        if lastReread != nil {
                abcp.Facts = append(abcp.Facts, OrderStatusFactView{
                        System:     OrderStatusSystemABCP,
                        Kind:       "correction_reread",
                        ExternalID: lastReread.ExternalObjectID,
                        RecordedAt: lastReread.CreatedAt,
                        Source:     OrderFactSourceManagerRecord,
                        ActorID:    lastReread.ActorID,
                        Word:       "перечит словами менеджера (контур 4-8): " + lastReread.ExternalObjectID,
                })
                if m.Operation.Receipt != nil && m.Operation.Receipt.ExternalOrderID != lastReread.ExternalObjectID {
                        abcp.DivergentIDs = true
                        abcp.Word = abcp.Word + " — " + OrderStatusExternalIDDivergenceWord
                }
        }
        if len(abcp.Facts) == 0 && abcp.Word == "" {
                abcp.Word = "не записано"
        }
        oneC := OrderStatusSystemView{
                System: OrderStatusSystemOneC,
                Word:   OrderOneCUnknownWord,
        }
        supplier := OrderStatusSystemView{
                System: OrderStatusSystemSupplier,
                Word:   OrderStatusSupplierNotRecordedWord,
        }
        if len(facts) > 0 {
                supplier.Word = OrderStatusSupplierManagerSourceWord
                for _, fact := range facts {
                        supplier.Facts = append(supplier.Facts, OrderStatusFactView{
                                System:        OrderStatusSystemSupplier,
                                Kind:          fact.Kind,
                                ExternalID:    fact.SupplierOrderID,
                                HappenedAt:    fact.HappenedAt,
                                RecordedAt:    fact.RecordedAt,
                                Source:        OrderFactSourceManagerRecord,
                                ActorID:       fact.ActorID,
                                Word:          OrderSupplierFactKindWords[fact.Kind] + " — " + fact.SupplierOrderID,
                        })
                }
        }
        projection.Systems = []OrderStatusSystemView{abcp, oneC, supplier}

        // The freshness (AC-10): the last confirmed fact time — the max of
        // the material times (the receipt, the verification/correction
        // events, the facts with BOTH times); the no-schedule word rides
        // along (the stale value never renders as current).
        last := time.Time{}
        consider := func(candidate time.Time) {
                if !candidate.IsZero() && candidate.After(last) {
                        last = candidate
                }
        }
        if m.Operation.Receipt != nil {
                consider(m.Operation.Receipt.CreatedAt)
        }
        for _, task := range tasks {
                for _, event := range task.Events {
                        consider(event.CreatedAt)
                }
        }
        for _, correction := range corrections {
                for _, event := range correction.Events {
                        consider(event.CreatedAt)
                }
        }
        for _, fact := range facts {
                consider(fact.HappenedAt)
                consider(fact.RecordedAt)
        }
        projection.Freshness = OrderStatusFreshness{
                LastConfirmedAt: last,
                Word:            OrderStatusFreshnessWord,
                NoScheduleWord:  OrderStatusNoScheduleWord,
        }

        // The timeline (the ordered fold of the transitions — AC-9).
        projection.Timeline = DeriveOrderStatusTimeline(m)

        // The deadline block (AC-11): the three SEPARATE words.
        estimate := strings.TrimSpace(m.Operation.OfferEstimate)
        if estimate == "" {
                projection.Deadline = OrderStatusDeadlineBlock{
                        EstimateWord:  OrderStatusDeadlineNoEstimateWord,
                        ConfirmedWord: OrderStatusDeadlineConfirmedWord,
                        ActualWord:    OrderStatusDeadlineActualWord + " — записи отправки нет",
                }
        } else {
                projection.Deadline = OrderStatusDeadlineBlock{
                        EstimateWord:  OrderStatusDeadlineEstimateWord + " — " + estimate,
                        ConfirmedWord: OrderStatusDeadlineConfirmedWord,
                        ActualWord:    OrderStatusDeadlineActualWord + " — записи отправки нет",
                }
        }
        if stageSent.Confirmed {
                projection.Deadline.ActualAt = stageSent.Time
                projection.Deadline.ActualRecorded = true
                projection.Deadline.ActualWord = fmt.Sprintf("%s — %s", OrderStatusDeadlineActualWord, stageSent.Time.Format(time.RFC3339))
        }

        // The next step (the honest derivation of the CURRENT state).
        projection.NextStep = deriveOrderStatusNextStep(m, stageCreated, activeTask, lastClosed, activeCorrection, formedFacts, uncovered)

        // The client retell words (AC-16 — the material of the retell, the
        // simple language of the client).
        projection.ClientWords = deriveOrderStatusClientWords(projection, stageSent.Confirmed)
        return projection
}

// orderSupplierFactListWord renders the per-name list of the formation
// facts (the supplier order IDs with the times).
func orderSupplierFactListWord(facts []OrderSupplierFact) string {
        parts := make([]string, 0, len(facts))
        for _, fact := range facts {
                parts = append(parts, fmt.Sprintf("%s (%s)", fact.SupplierOrderID, fact.HappenedAt.Format(time.RFC3339)))
        }
        return "сформированные заказы поставщику: " + strings.Join(parts, "; ")
}

// orderPartialSentListWord renders the honest partial-send detail
// (AC-15): every formation named — covered or not.
func orderPartialSentListWord(covered, uncovered []OrderSupplierFact) string {
        parts := make([]string, 0, len(covered)+len(uncovered))
        for _, fact := range covered {
                parts = append(parts, fmt.Sprintf("%s отправлен", fact.SupplierOrderID))
        }
        for _, fact := range uncovered {
                parts = append(parts, fmt.Sprintf("%s сформирован и не записан как отправленный", fact.SupplierOrderID))
        }
        return strings.Join(parts, "; ")
}

// orderSupplierIDs collects the supplier order IDs of the facts.
func orderSupplierIDs(facts []OrderSupplierFact) []string {
        ids := make([]string, 0, len(facts))
        for _, fact := range facts {
                ids = append(ids, fact.SupplierOrderID)
        }
        return ids
}

// deriveOrderStatusNextStep derives the honest next-step word of the
// CURRENT projection state (never an automatic action — the human
// steps of the store contour).
func deriveOrderStatusNextStep(m OrderStatusMaterial, created OrderStageEvidence, activeTask *OrderVerificationStateView, lastClosed *OrderVerificationEvent, activeCorrection *OrderCorrectionStateView, formed, uncovered []OrderSupplierFact) string {
        if !m.Operation.Found {
                return "операции заказа в кейсе нет — создайте заказ (контур 4-6)"
        }
        if m.Operation.DispatchBlocked && m.Operation.Receipt == nil {
                return OrderStatusManualContourWord + " — следующий шаг: создать заказ в ABCP вручную и записать подтверждение клиента (контур 4-6)"
        }
        if activeCorrection != nil {
                return "исправить состав и выполнить перечит подтверждённой версии — владелец коррекции назначен (контур 4-8)"
        }
        if activeTask != nil {
                return "проверить заказ менеджером — задача проверки открыта (контур 4-7)"
        }
        if lastClosed != nil && lastClosed.Kind == OrderVerificationEventBlocked {
                return OrderStatusBlockedNoCorrectionWord
        }
        if lastClosed != nil && lastClosed.Kind == OrderVerificationEventApproved && len(formed) == 0 {
                return "сформировать заказ у поставщика в ABCP вручную и записать факт формирования на этой панели"
        }
        if len(uncovered) > 0 {
                return "отправить неотправленные формирования поставщику вручную и записать факты отправки: " + strings.Join(orderSupplierIDs(uncovered), ", ")
        }
        if len(formed) > 0 {
                return "ожидание поставки: записей о поставке в этом срезе нет — стадия «отправлен» подтверждена записями магазина"
        }
        return "продвижение заказа ждёт решения проверки (контур 4-7)"
}

// deriveOrderStatusClientWords builds the client retell material
// (AC-16): the current stage word, the boundary word (the
// never-claimed), the supplier word and the freshness word — the
// simple language of the retell, never the operational jargon.
func deriveOrderStatusClientWords(projection OrderStatusProjection, sentConfirmed bool) []string {
        words := []string{}
        if projection.CurrentStage == "" {
                words = append(words, OrderClientCreatedWord)
        } else {
                words = append(words, OrderStageWords[projection.CurrentStage])
        }
        if projection.SpecialState != "" {
                words = append(words, projection.SpecialState)
        }
        if !sentConfirmed {
                words = append(words, OrderClientBoundaryWord)
        }
        words = append(words, OrderOneCUnknownWord)
        words = append(words, projection.Freshness.Word)
        return words
}
