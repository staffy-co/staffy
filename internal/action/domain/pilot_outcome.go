package domain

import (
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// Module action, story 4.11 — the Pilot Business Outcome domain: the PURE
// read-time derivation of the ten-stage inquiry funnel, the attribution of
// the first accepted inbound channel event and the aggregate pilot report
// over the recorded material of the pilot contour (the case material, the
// vehicle/part/fitment material of 4.2..4.4, the order machine material of
// 4.6..4.9 and the servicing material of 4.10). The derivation writes
// NOTHING — the computed representation never becomes a new source of
// truth (AC-13 read-time: a repeated read of the same world answers the
// same; a late supplier fact of 4.9 changes the reading WITHOUT rewriting
// the append-only history). The terminal stages of the funnel are
// unknown_pending ALWAYS in this slice: the owning sources of the
// execution, the payment, the cancellation and the return live OUTSIDE
// the contour (OQ-4/OQ-3, deferred-work No. 215) — «не доказано И не
// опровергнуто», never zero and never a silent exclusion (AC-14). The
// non-measurable indicators of the Measurement Dictionary v1 — revenue,
// margin, variable_costs, human_minutes — are WORDS of reasons only: the
// dictionary rows carry NO numeric fields (the structural honesty probe
// of the unit matrix).

// ---------------------------------------------------------------------------
// The honest words of the slice (the text, never only color)
// ---------------------------------------------------------------------------

// The stage words of the proven funnel (the evidence words BY NAME).
const (
	PilotInquiryAcceptedWord     = "обращение принято: кейс создан первым принятым канальным событием"
	PilotVehicleConfirmedWord    = "профиль автомобиля подтверждён: атрибуты идентификации подтверждены"
	PilotApplicabilityProvenWord = "применимость доказана: лестница доказательств контура 4-4"
	PilotClientOrderCreatedWord  = "заказ клиента создан: операция с подтверждённой квитанцией и внешним ID"
	PilotManagerVerifiedWord     = "проверка менеджером подтверждена: последнее закрытое решение — «Подтвердить проверку»"
	PilotSupplierSentWord        = "отправка поставщику записана: запись менеджера с номером заказа поставщика"
)

// The unknown_pending pin words of the terminal stages (OQ-4/OQ-3/No. 215
// — the owning sources live outside the contour; the success is never
// claimed while the terminal proof is absent).
const (
	PilotFulfilledPinWord = "исполнение заказа — владеющий источник вне контура (1С/исполнение поставщика, OQ-4/AG-4): не доказано и не опровергнуто"
	PilotPaidPinWord      = "оплата — владеющий источник вне контура (1С, OQ-4): не доказано и не опровергнуто"
	PilotCancelledPinWord = "исполнение отмены не существует в контуре (OQ-3 fence + отложенная работа №215; CancelFeasibility 4-10 — только запрос): не доказано и не опровергнуто"
	PilotReturnedPinWord  = "материала возврата нет (претензии — контур 3-8 словами): не доказано и не опровергнуто"
)

// The shared honest words of the states and the misses.
const (
	PilotNotReachedWord         = "не достигнуто: материал стадии отсутствует"
	PilotVehicleNotReachedWord  = "материал идентификации отсутствует — обращение не дошло до подтверждения профиля"
	PilotFitmentNotReachedWord  = "применимость не проверена: доказательств контура 4-4 нет"
	PilotOrderNotReachedWord    = "операции заказа в кейсе нет"
	PilotVerifiedNotReachedWord = "подтверждённого решения проверки нет"
	PilotSentNotReachedWord     = "записей об отправке поставщику нет"
	PilotUnknownPendingWord     = "не доказано и не опровергнуто — молчаливое исключение запрещено (AC-14)"
)

// The attribution words (AC-1: only what the source carried; the unknown
// is spoken by the word, never restored by a guess).
const (
	PilotAttributionWordPrefix   = "источник обращения: "
	PilotAttributionUnknownWord  = "источник не передал — не восстанавливается догадкой"
	PilotAttributionMissingWord  = "принятых канальных событий нет: источник обращения неизвестен"
	PilotAttributionProviderWord = "провайдер"
	PilotAttributionChannelWord  = "канал"
	PilotAttributionEventIDWord  = "ID события провайдера"
	PilotAttributionTimeWord     = "время приёма"
)

// The OQ-7 boundary word of the report (AC-12): no approved baselines —
// observable indicators WITHOUT an improvement claim.
const PilotOQ7BoundaryWord = "утверждённых baseline/когорт/порогов НЕТ — наблюдаемые показатели без заявления об улучшении; корреляция — не причинность"

// The honest sample word of every number of the report (AC-11).
const PilotSampleWordPrefix = "выборка: "

// The honest share word of the unknown outcomes (AC-14).
const PilotUnknownShareWord = "терминальные исходы не измеримы в контуре — достоверность результата ограничена"

// The honest zero-sample word of the empty period.
const PilotEmptySampleWord = "кейсов периода нет: честная нулевая выборка (ноль допустим только как счёт кейсов)"

// The measurement dictionary version word (AC-9: a definition change is a
// NEW dictionary version through the Spec Change Log, never silent).
const PilotDictionaryVersion = "v1"

// The dictionary owner word (AC-9: the dictionary is approved BEFORE any
// comparative conclusion — the spec approval of CHECKPOINT 1).
const PilotDictionaryOwnerWord = "словарь утверждается одобрением спеки (CHECKPOINT 1); смена определения — новая версия словаря через Spec Change Log, не молча"

// ---------------------------------------------------------------------------
// The dictionaries (compile-time; never a tenant configuration)
// ---------------------------------------------------------------------------

// The ten stages of the inquiry funnel (BY NAME, the lifecycle order).
const (
	PilotStageInquiryAccepted         = "inquiry_accepted"
	PilotStageVehicleProfileConfirmed = "vehicle_profile_confirmed"
	PilotStageApplicabilityProven     = "applicability_proven"
	PilotStageClientOrderCreated      = "client_order_created"
	PilotStageManagerVerified         = "manager_verified"
	PilotStageSupplierSent            = "supplier_sent"
	PilotStageFulfilled               = "fulfilled"
	PilotStagePaid                    = "paid"
	PilotStageCancelled               = "cancelled"
	PilotStageReturned                = "returned"
)

// The three states of one stage (BY NAME — the text, never only color).
const (
	PilotStageStateProven         = "proven"
	PilotStageStateNotReached     = "not_reached"
	PilotStageStateUnknownPending = "unknown_pending"
)

// PilotOutcomeStages is the full stage dictionary (the funnel order).
var PilotOutcomeStages = []string{
	PilotStageInquiryAccepted, PilotStageVehicleProfileConfirmed, PilotStageApplicabilityProven,
	PilotStageClientOrderCreated, PilotStageManagerVerified, PilotStageSupplierSent,
	PilotStageFulfilled, PilotStagePaid, PilotStageCancelled, PilotStageReturned,
}

// PilotTerminalStages is the terminal stage dictionary: unknown_pending
// ALWAYS in this slice (the owning sources are outside the contour).
var PilotTerminalStages = []string{
	PilotStageFulfilled, PilotStagePaid, PilotStageCancelled, PilotStageReturned,
}

// PilotMeasurableStages is the measurable stage dictionary: the proven
// ladder of the funnel — the conversions live between the ADJACENT ones
// only (the unknown_pending stages never enter a denominator).
var PilotMeasurableStages = []string{
	PilotStageInquiryAccepted, PilotStageVehicleProfileConfirmed, PilotStageApplicabilityProven,
	PilotStageClientOrderCreated, PilotStageManagerVerified, PilotStageSupplierSent,
}

// PilotStageStateWords is the honest word of every state (the words render
// text — the states are never distinguished by color only, AC-18).
var PilotStageStateWords = map[string]string{
	PilotStageStateProven:         "доказано (proven)",
	PilotStageStateNotReached:     "не достигнуто (not_reached)",
	PilotStageStateUnknownPending: "неизвестно/ожидает (unknown_pending)",
}

// The evidence source words of the proven stages (BY NAME).
const (
	PilotSourceChannelEventWord         = "канальное событие (принятое inbound)"
	PilotSourceVehicleConfirmationWord  = "подтверждение атрибута идентификации (контур 4-2)"
	PilotSourceFitmentEvidenceWord      = "запись доказательства применимости (контур 4-4)"
	PilotSourceOperationReceiptWord     = "квитанция операции заказа (контур 4-6)"
	PilotSourceVerificationDecisionWord = "закрытое решение проверки (контур 4-7)"
	PilotSourceSupplierFactWord         = "запись факта поставщика менеджером (контур 4-9)"
)

// ---------------------------------------------------------------------------
// The material of the per-case fold (pure primitives and the EXISTING
// action-domain types; the domain imports platform/id only — AD-10)
// ---------------------------------------------------------------------------

// PilotInboundEvent is one accepted inbound channel event of the case
// timeline (the attribution material: only what the source carried).
type PilotInboundEvent struct {
	Provider        string
	Channel         string
	ProviderEventID string
	ReceivedAt      time.Time
}

// PilotVehicleMaterial is the vehicle identification material of 4-2 (the
// honest primitives: the confirmed attribute keys BY NAME and the last
// confirmation time; the absence is the honest not_reached).
type PilotVehicleMaterial struct {
	SchemaDefined           bool
	IdentificationSatisfied bool
	ConfirmedKeys           []string
	LastConfirmedAt         time.Time
}

// PilotFitmentPositionMaterial is one fitment position state of 4-4.
type PilotFitmentPositionMaterial struct {
	PositionKey string
	Status      string
	ProvenAt    time.Time
}

// PilotFitmentMaterial is the fitment material of the case.
type PilotFitmentMaterial struct {
	Positions []PilotFitmentPositionMaterial
}

// PilotOrderMaterial is the order machine material of ONE order operation
// of the case (the EXISTING domain types of 4-6..4-9 — the same material
// the Order Status Projection reads; this slice only DERIVES, it never
// rewrites the machines).
type PilotOrderMaterial struct {
	Operation     OrderStatusOperationMaterial
	Verifications []OrderStatusVerificationMaterial
	Facts         []OrderSupplierFact
}

// PilotCaseMaterial is the whole read-time material of one case outcome.
type PilotCaseMaterial struct {
	CaseFound            bool
	CaseID               id.UUID
	CaseNo               int64
	CreatedAt            time.Time
	InboundEvents        []PilotInboundEvent
	Vehicle              PilotVehicleMaterial
	Fitment              PilotFitmentMaterial
	Orders               []PilotOrderMaterial
	WorkItemsCount       int
	PartRequestConfirmed bool
	HandoffRequests      int
	Corrections          int
	RecoveryCases        int
}

// ---------------------------------------------------------------------------
// The views of the derivation
// ---------------------------------------------------------------------------

// PilotAttributionView is the attribution of the inquiry: the FIRST
// accepted inbound channel event {provider, channel, provider_event_id,
// time} — only what the source carried; campaign/referrer are the unknown
// words (the envelope v1 carries no such fields — the ingress is never
// touched).
type PilotAttributionView struct {
	Found           bool
	Provider        string
	Channel         string
	ProviderEventID string
	ReceivedAt      time.Time
	CampaignWord    string
	ReferrerWord    string
	Word            string
}

// PilotStageEvidence is one stage of the funnel: the state word, the
// evidence word, the time, the source word and the unknown pin word of
// the terminal stages.
type PilotStageEvidence struct {
	Stage      string
	State      string
	StateWord  string
	Evidence   string
	Time       time.Time
	SourceWord string
	PinWord    string
}

// PilotCaseOutcome is the derived outcome of one case: the attribution,
// the outcome chain words (the AC-2 graph), the ten stages and the
// counters of the recorded corrections/handoffs/recovery cases.
type PilotCaseOutcome struct {
	CaseFound        bool
	CaseID           id.UUID
	CaseNo           int64
	Attribution      PilotAttributionView
	ChainWords       []string
	Stages           []PilotStageEvidence
	HandoffCount     int
	CorrectionsCount int
	RecoveryCount    int
}

// DeriveCaseOutcome derives the outcome of ONE case over the recorded
// material (READ-TIME: zero writes, idempotent — the same world answers
// the same; a late fact changes the reading, never the history).
func DeriveCaseOutcome(m PilotCaseMaterial) PilotCaseOutcome {
	outcome := PilotCaseOutcome{
		CaseFound:        m.CaseFound,
		CaseID:           m.CaseID,
		CaseNo:           m.CaseNo,
		HandoffCount:     m.HandoffRequests,
		CorrectionsCount: m.Corrections,
		RecoveryCount:    m.RecoveryCases,
	}
	if !m.CaseFound {
		for _, stage := range PilotOutcomeStages {
			outcome.Stages = append(outcome.Stages, PilotStageEvidence{
				Stage:     stage,
				State:     PilotStageStateNotReached,
				StateWord: PilotStageStateWords[PilotStageStateNotReached],
				Evidence:  PilotNotReachedWord,
			})
		}
		return outcome
	}
	outcome.Attribution = derivePilotAttribution(m.InboundEvents)
	outcome.ChainWords = derivePilotChainWords(m)
	outcome.Stages = appendPilotInquiryStage(m, outcome.Stages)
	outcome.Stages = appendPilotVehicleStage(m, outcome.Stages)
	outcome.Stages = appendPilotFitmentStage(m, outcome.Stages)
	outcome.Stages = appendPilotOrderStages(m, outcome.Stages)
	outcome.Stages = appendPilotTerminalStages(outcome.Stages)
	return outcome
}

// derivePilotAttribution folds the FIRST accepted inbound event (the
// earliest received time — the events of one case arrive ordered, the
// fold is honest over any order).
func derivePilotAttribution(events []PilotInboundEvent) PilotAttributionView {
	view := PilotAttributionView{
		CampaignWord: PilotAttributionUnknownWord,
		ReferrerWord: PilotAttributionUnknownWord,
	}
	var first *PilotInboundEvent
	for i := range events {
		if first == nil || events[i].ReceivedAt.Before(first.ReceivedAt) {
			first = &events[i]
		}
	}
	if first == nil {
		view.Word = PilotAttributionWordPrefix + PilotAttributionMissingWord
		return view
	}
	view.Found = true
	view.Provider = first.Provider
	view.Channel = first.Channel
	view.ProviderEventID = first.ProviderEventID
	view.ReceivedAt = first.ReceivedAt
	view.Word = PilotAttributionWordPrefix + PilotAttributionProviderWord + " " + first.Provider +
		"; " + PilotAttributionChannelWord + " " + first.Channel +
		"; " + PilotAttributionEventIDWord + " " + first.ProviderEventID
	return view
}

// derivePilotChainWords renders the outcome chain of the case (AC-2: the
// chain of the inquiry — the source, the dialog, the case, the work
// item, the vehicle profile, the part request, the applicability proof,
// the operation, the external order ID — BY NAME, anchored by the case
// and operation IDs, never by a phone number).
func derivePilotChainWords(m PilotCaseMaterial) []string {
	attribution := derivePilotAttribution(m.InboundEvents)
	chain := []string{
		"источник: " + attribution.Word,
	}
	eventsWord := "диалог: принятых канальных событий нет"
	if len(m.InboundEvents) > 0 {
		eventsWord = "диалог: " + itoa(len(m.InboundEvents)) + " принятых канальных событий"
	}
	chain = append(chain, eventsWord)
	chain = append(chain, "кейс: создан (номер "+itoa(int(m.CaseNo))+")")
	workWord := "рабочий элемент: рабочих элементов нет"
	if m.WorkItemsCount > 0 {
		workWord = "рабочий элемент: " + itoa(m.WorkItemsCount)
	}
	chain = append(chain, workWord)
	vehicleWord := "профиль автомобиля: не подтверждён"
	if m.Vehicle.IdentificationSatisfied {
		vehicleWord = "профиль автомобиля: подтверждён (" + joinWords(m.Vehicle.ConfirmedKeys) + ")"
	}
	chain = append(chain, vehicleWord)
	partWord := "запрос детали: не подтверждён"
	if m.PartRequestConfirmed {
		partWord = "запрос детали: подтверждён (контур 4-3)"
	}
	chain = append(chain, partWord)
	fitmentWord := "подтверждение применимости: не доказано"
	for _, position := range m.Fitment.Positions {
		if position.Status == "proven" {
			fitmentWord = "подтверждение применимости: доказано (позиция " + position.PositionKey + ")"
			break
		}
	}
	chain = append(chain, fitmentWord)
	orderWord := "операция заказа: нет"
	externalID := ""
	for i := range m.Orders {
		if m.Orders[i].Operation.Receipt != nil && m.Orders[i].Operation.Receipt.ExternalOrderID != "" {
			orderWord = "операция заказа: создана"
			externalID = m.Orders[i].Operation.Receipt.ExternalOrderID
			break
		}
	}
	chain = append(chain, orderWord)
	externalWord := "внешний ID заказа: нет"
	if externalID != "" {
		externalWord = "внешний ID заказа: " + externalID
	}
	chain = append(chain, externalWord)
	return chain
}

// appendPilotInquiryStage appends the inquiry_accepted stage (the case
// itself is the proof — the case exists because the first accepted
// channel event created it).
func appendPilotInquiryStage(m PilotCaseMaterial, stages []PilotStageEvidence) []PilotStageEvidence {
	evidence := PilotStageEvidence{
		Stage:      PilotStageInquiryAccepted,
		State:      PilotStageStateProven,
		StateWord:  PilotStageStateWords[PilotStageStateProven],
		Evidence:   PilotInquiryAcceptedWord,
		Time:       m.CreatedAt,
		SourceWord: PilotSourceChannelEventWord,
	}
	if len(m.InboundEvents) > 0 {
		first := m.InboundEvents[0]
		for _, event := range m.InboundEvents {
			if event.ReceivedAt.Before(first.ReceivedAt) {
				first = event
			}
		}
		evidence.Time = first.ReceivedAt
	}
	return append(stages, evidence)
}

// appendPilotVehicleStage appends the vehicle_profile_confirmed stage.
func appendPilotVehicleStage(m PilotCaseMaterial, stages []PilotStageEvidence) []PilotStageEvidence {
	evidence := PilotStageEvidence{
		Stage:     PilotStageVehicleProfileConfirmed,
		State:     PilotStageStateNotReached,
		StateWord: PilotStageStateWords[PilotStageStateNotReached],
		Evidence:  PilotVehicleNotReachedWord,
	}
	if m.Vehicle.IdentificationSatisfied {
		evidence.State = PilotStageStateProven
		evidence.StateWord = PilotStageStateWords[PilotStageStateProven]
		evidence.Evidence = PilotVehicleConfirmedWord + " (" + joinWords(m.Vehicle.ConfirmedKeys) + ")"
		evidence.Time = m.Vehicle.LastConfirmedAt
		evidence.SourceWord = PilotSourceVehicleConfirmationWord
	}
	return append(stages, evidence)
}

// appendPilotFitmentStage appends the applicability_proven stage.
func appendPilotFitmentStage(m PilotCaseMaterial, stages []PilotStageEvidence) []PilotStageEvidence {
	evidence := PilotStageEvidence{
		Stage:     PilotStageApplicabilityProven,
		State:     PilotStageStateNotReached,
		StateWord: PilotStageStateWords[PilotStageStateNotReached],
		Evidence:  PilotFitmentNotReachedWord,
	}
	for _, position := range m.Fitment.Positions {
		if position.Status == "proven" {
			evidence.State = PilotStageStateProven
			evidence.StateWord = PilotStageStateWords[PilotStageStateProven]
			evidence.Evidence = PilotApplicabilityProvenWord + " (позиция " + position.PositionKey + ")"
			evidence.Time = position.ProvenAt
			evidence.SourceWord = PilotSourceFitmentEvidenceWord
			break
		}
	}
	return append(stages, evidence)
}

// appendPilotOrderStages appends the three order-machine stages of the
// funnel: client_order_created (the committed receipt with the external
// ID), manager_verified (the LAST CLOSED approved decision of any order
// operation — the same ladder semantics as the Order Status Projection)
// and supplier_sent (the human receipt {sent} of any order operation).
// The EARLIEST proven material of the case is the evidence — the stage
// speaks the moment the case reached it, honestly over every operation.
func appendPilotOrderStages(m PilotCaseMaterial, stages []PilotStageEvidence) []PilotStageEvidence {
	created := PilotStageEvidence{
		Stage:     PilotStageClientOrderCreated,
		State:     PilotStageStateNotReached,
		StateWord: PilotStageStateWords[PilotStageStateNotReached],
		Evidence:  PilotOrderNotReachedWord,
	}
	verified := PilotStageEvidence{
		Stage:     PilotStageManagerVerified,
		State:     PilotStageStateNotReached,
		StateWord: PilotStageStateWords[PilotStageStateNotReached],
		Evidence:  PilotVerifiedNotReachedWord,
	}
	sent := PilotStageEvidence{
		Stage:     PilotStageSupplierSent,
		State:     PilotStageStateNotReached,
		StateWord: PilotStageStateWords[PilotStageStateNotReached],
		Evidence:  PilotSentNotReachedWord,
	}
	var receipt *OrderStatusReceiptPin
	var approved *OrderVerificationEvent
	var sentFact *OrderSupplierFact
	for i := range m.Orders {
		order := m.Orders[i]
		if order.Operation.Found && order.Operation.Receipt != nil && order.Operation.Receipt.ExternalOrderID != "" {
			if receipt == nil || order.Operation.Receipt.CreatedAt.Before(receipt.CreatedAt) {
				pin := *order.Operation.Receipt
				receipt = &pin
			}
		}
		approved = earlierPilotApproved(order.Verifications, approved)
		sentFact = earlierPilotSentFact(order.Facts, sentFact)
	}
	if receipt != nil {
		created.State = PilotStageStateProven
		created.StateWord = PilotStageStateWords[PilotStageStateProven]
		created.Evidence = PilotClientOrderCreatedWord + " (внешний ID " + receipt.ExternalOrderID + ")"
		created.Time = receipt.CreatedAt
		created.SourceWord = PilotSourceOperationReceiptWord
	}
	if approved != nil {
		verified.State = PilotStageStateProven
		verified.StateWord = PilotStageStateWords[PilotStageStateProven]
		verified.Evidence = PilotManagerVerifiedWord
		verified.Time = approved.CreatedAt
		verified.SourceWord = PilotSourceVerificationDecisionWord
	}
	if sentFact != nil {
		sent.State = PilotStageStateProven
		sent.StateWord = PilotStageStateWords[PilotStageStateProven]
		sent.Evidence = PilotSupplierSentWord + " (заказ поставщика " + sentFact.SupplierOrderID + ")"
		sent.Time = sentFact.HappenedAt
		sent.SourceWord = PilotSourceSupplierFactWord
	}
	return append(append(append(stages, created), verified), sent)
}

// earlierPilotApproved folds the verification events of ONE order
// operation: the LAST CLOSED decision (the maximal (task, event) anchor
// of the approved/blocked events — the ladder-down semantics of the Order
// Status Projection) — approved proves the stage; the earliest such
// approved moment across the operations wins the evidence.
func earlierPilotApproved(material []OrderStatusVerificationMaterial, current *OrderVerificationEvent) *OrderVerificationEvent {
	var lastClosed *OrderVerificationEvent
	for i := range material {
		events := material[i].Events
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
	if lastClosed == nil || lastClosed.Kind != OrderVerificationEventApproved {
		return current
	}
	if current == nil || lastClosed.CreatedAt.Before(current.CreatedAt) {
		return lastClosed
	}
	return current
}

// earlierPilotSentFact folds the supplier facts of ONE order operation:
// the EARLIEST {sent} human receipt proves the stage (the honest
// aggregate of the recorded material).
func earlierPilotSentFact(facts []OrderSupplierFact, current *OrderSupplierFact) *OrderSupplierFact {
	for i := range facts {
		if facts[i].Kind != OrderSupplierFactSent {
			continue
		}
		if current == nil || facts[i].HappenedAt.Before(current.HappenedAt) {
			current = &facts[i]
		}
	}
	return current
}

// appendPilotTerminalStages appends the four terminal stages —
// unknown_pending ALWAYS in this slice (the pins BY NAME).
func appendPilotTerminalStages(stages []PilotStageEvidence) []PilotStageEvidence {
	pins := map[string]string{
		PilotStageFulfilled: PilotFulfilledPinWord,
		PilotStagePaid:      PilotPaidPinWord,
		PilotStageCancelled: PilotCancelledPinWord,
		PilotStageReturned:  PilotReturnedPinWord,
	}
	for _, stage := range PilotTerminalStages {
		stages = append(stages, PilotStageEvidence{
			Stage:     stage,
			State:     PilotStageStateUnknownPending,
			StateWord: PilotStageStateWords[PilotStageStateUnknownPending],
			Evidence:  PilotUnknownPendingWord,
			PinWord:   pins[stage],
		})
	}
	return stages
}

// ---------------------------------------------------------------------------
// The Measurement Dictionary v1 (compile-time; AC-9)
// ---------------------------------------------------------------------------

// PilotIndicatorView is ONE measurable indicator of the dictionary: the
// id BY NAME, the title, the formula word, the grain, the numerator and
// the denominator BY NAME, the maturity window word, the owning source,
// the exclusions word, the version and the owner word.
type PilotIndicatorView struct {
	ID           string
	Title        string
	Formula      string
	Grain        string
	Numerator    string
	Denominator  string
	Maturity     string
	OwningSource string
	Exclusions   string
	Version      string
	Owner        string
}

// PilotNotMeasurableIndicator is ONE non-measurable indicator of the
// dictionary: the id BY NAME, the title and the reason word — the row
// carries NO numeric fields (the structural honesty probe: the missing
// component is NEVER replaced by a zero, AC-4/AC-5/AC-7/AC-8).
type PilotNotMeasurableIndicator struct {
	ID     string
	Title  string
	Reason string
}

// PilotNotMeasurableIndicators is the compile-time list of the
// non-measurable indicators (the words of the reasons only).
var PilotNotMeasurableIndicators = []PilotNotMeasurableIndicator{
	{
		ID:     "revenue",
		Title:  "Выручка",
		Reason: "владеющий источник — Owning System 1С — не подключён (OQ-4); созданный клиентский заказ НЕ выручка (AC-4): записей выручки/оплаты менеджером контур не заводит — подмена источника запрещена",
	},
	{
		ID:     "margin",
		Title:  "Маржа",
		Reason: "закупочные цены вне контура (фенс 4-9: ценовые поля — 0 в материале): маржа «неизвестна» — НЕ 0; отсутствующая составляющая делает маржу неполной или неизвестной (AC-5)",
	},
	{
		ID:     "variable_costs",
		Title:  "Переменные расходы",
		Reason: "usage-записи 1-11 — тенант-уровня, кейса не несут: атрибуция затрат на обращение невозможна (AC-7); затраты тенанта — страница бюджета словом-ссылкой",
	},
	{
		ID:     "human_minutes",
		Title:  "Человеческие минуты",
		Reason: "lifecycle-события 2-13 записаны, утверждённого способа пересчёта НЕТ: ожидание клиента и активная работа материалом не различимы (AC-8); 0 не подставляется",
	},
}

// PilotMeasurementDictionary is the compile-time Measurement Dictionary
// v1 (AC-9): every measurable indicator with the full definition.
var PilotMeasurementDictionary = buildPilotMeasurementDictionary()

// buildPilotMeasurementDictionary assembles the dictionary: the case
// volume, the stage counts, the conversions between the ADJACENT proven
// stages, the counters and the unknown share.
func buildPilotMeasurementDictionary() []PilotIndicatorView {
	dictionary := []PilotIndicatorView{
		{
			ID:           "case_volume",
			Title:        "Объём обращений",
			Formula:      "число кейсов периода",
			Grain:        "case",
			Numerator:    "кейсы периода",
			Denominator:  "—",
			Maturity:     "период закрывается концом месяца",
			OwningSource: "Staffy (кейс-контур 2-1)",
			Exclusions:   "кейсы чужих тенантов (RLS-слепота)",
			Version:      PilotDictionaryVersion,
			Owner:        PilotDictionaryOwnerWord,
		},
		{
			ID:           "corrections_count",
			Title:        "Коррекции",
			Formula:      "число коррекций 4-8 по кейсам периода",
			Grain:        "case",
			Numerator:    "коррекции кейсов периода",
			Denominator:  "—",
			Maturity:     "append-only история коррекций",
			OwningSource: "Staffy (контур 4-8)",
			Exclusions:   "—",
			Version:      PilotDictionaryVersion,
			Owner:        PilotDictionaryOwnerWord,
		},
		{
			ID:           "handoff_count",
			Title:        "Передачи человеку",
			Formula:      "число запросов человека 2-13 по кейсам периода",
			Grain:        "case",
			Numerator:    "запросы человека кейсов периода",
			Denominator:  "—",
			Maturity:     "append-only lifecycle-история",
			OwningSource: "Staffy (контур 2-13)",
			Exclusions:   "—",
			Version:      PilotDictionaryVersion,
			Owner:        PilotDictionaryOwnerWord,
		},
		{
			ID:           "recovery_count",
			Title:        "Recovery-кейсы",
			Formula:      "число recovery-кейсов 3-8 по кейсам периода",
			Grain:        "case",
			Numerator:    "recovery-кейсы кейсов периода",
			Denominator:  "—",
			Maturity:     "append-only история recovery",
			OwningSource: "Staffy (контур 3-8)",
			Exclusions:   "—",
			Version:      PilotDictionaryVersion,
			Owner:        PilotDictionaryOwnerWord,
		},
		{
			ID:           "unknown_outcome_share",
			Title:        "Доля неизвестных исходов",
			Formula:      "кейсы без терминальных proven-стадий / все кейсы периода",
			Grain:        "case",
			Numerator:    "кейсы без терминальных proven-стадий",
			Denominator:  "все кейсы периода",
			Maturity:     "терминальные исходы не измеримы в контуре (OQ-4/OQ-3)",
			OwningSource: "Staffy (деривация среза 4-11)",
			Exclusions:   "—",
			Version:      PilotDictionaryVersion,
			Owner:        PilotDictionaryOwnerWord,
		},
	}
	stageTitles := map[string]string{
		PilotStageInquiryAccepted:         "Обращение принято",
		PilotStageVehicleProfileConfirmed: "Профиль автомобиля подтверждён",
		PilotStageApplicabilityProven:     "Применимость доказана",
		PilotStageClientOrderCreated:      "Заказ клиента создан",
		PilotStageManagerVerified:         "Проверка менеджером подтверждена",
		PilotStageSupplierSent:            "Отправка поставщику записана",
	}
	for _, stage := range PilotMeasurableStages {
		dictionary = append(dictionary, PilotIndicatorView{
			ID:           "stage_" + stage + "_count",
			Title:        stageTitles[stage],
			Formula:      "число кейсов периода со стадией " + stage + " = proven",
			Grain:        "case",
			Numerator:    "кейсы со стадией proven",
			Denominator:  "—",
			Maturity:     "read-time деривация",
			OwningSource: "Staffy (материал контуров 2-1/4-2/4-4/4-6/4-7/4-9)",
			Exclusions:   "unknown_pending-стадии в числители не входят",
			Version:      PilotDictionaryVersion,
			Owner:        PilotDictionaryOwnerWord,
		})
	}
	for i := 0; i+1 < len(PilotMeasurableStages); i++ {
		from := PilotMeasurableStages[i]
		to := PilotMeasurableStages[i+1]
		dictionary = append(dictionary, PilotIndicatorView{
			ID:           "conversion_" + from + "_" + to,
			Title:        "Конверсия " + from + " → " + to,
			Formula:      "кейсы с доказанными И " + from + ", И " + to + " / кейсы с доказанным " + from,
			Grain:        "case",
			Numerator:    "кейсы с " + from + " = proven И " + to + " = proven",
			Denominator:  "кейсы с " + from + " = proven",
			Maturity:     "read-time деривация",
			OwningSource: "Staffy (деривация среза 4-11)",
			Exclusions:   "unknown_pending-стадии в знаменатели НЕ входят (честная граница измеримости)",
			Version:      PilotDictionaryVersion,
			Owner:        PilotDictionaryOwnerWord,
		})
	}
	return dictionary
}

// ---------------------------------------------------------------------------
// The aggregate report (the pure fold over the per-case outcomes)
// ---------------------------------------------------------------------------

// PilotConversion is ONE conversion of the funnel: the adjacent stages
// BY NAME with the numerator and the denominator (the honest word names
// both; a zero denominator never divides — the conversion is not
// derived, the word speaks it).
type PilotConversion struct {
	From        string
	To          string
	Numerator   int
	Denominator int
	Word        string
}

// PilotShare is the unknown outcome share: the numerator, the
// denominator and the honest word (AC-14 — the influence of the unknown
// is visible, never hidden).
type PilotShare struct {
	Numerator   int
	Denominator int
	Word        string
}

// PilotReport is the aggregate pilot report of one period: the case
// volume, the sample size and the period word (AC-11 — every number of
// the render carries them), the stage counts, the conversions, the
// counters, the unknown share, the OQ-7 boundary word and the
// Measurement Dictionary v1 with the non-measurable rows.
type PilotReport struct {
	CaseVolume       int
	SampleSize       int
	PeriodWord       string
	EmptySampleWord  string
	StageCounts      map[string]int
	Conversions      []PilotConversion
	CorrectionsCount int
	HandoffCount     int
	RecoveryCount    int
	UnknownShare     PilotShare
	BoundaryWord     string
	Dictionary       []PilotIndicatorView
	NotMeasurable    []PilotNotMeasurableIndicator
}

// DerivePilotReport folds the per-case outcomes into the aggregate
// report of one period (READ-TIME: zero writes, idempotent).
func DerivePilotReport(outcomes []PilotCaseOutcome, periodWord string) PilotReport {
	report := PilotReport{
		CaseVolume:    len(outcomes),
		SampleSize:    len(outcomes),
		PeriodWord:    periodWord,
		StageCounts:   map[string]int{},
		BoundaryWord:  PilotOQ7BoundaryWord,
		Dictionary:    PilotMeasurementDictionary,
		NotMeasurable: PilotNotMeasurableIndicators,
	}
	for _, stage := range PilotOutcomeStages {
		report.StageCounts[stage] = 0
	}
	if len(outcomes) == 0 {
		report.EmptySampleWord = PilotEmptySampleWord
		report.UnknownShare = PilotShare{
			Numerator:   0,
			Denominator: 0,
			Word:        PilotUnknownShareWord,
		}
		return report
	}
	proven := map[string]bool{}
	terminalProven := false
	for _, outcome := range outcomes {
		report.CorrectionsCount += outcome.CorrectionsCount
		report.HandoffCount += outcome.HandoffCount
		report.RecoveryCount += outcome.RecoveryCount
		caseTerminalProven := false
		for _, stage := range outcome.Stages {
			if stage.State != PilotStageStateProven {
				continue
			}
			report.StageCounts[stage.Stage]++
			proven[stage.Stage] = true
			for _, terminal := range PilotTerminalStages {
				if stage.Stage == terminal {
					caseTerminalProven = true
				}
			}
		}
		if caseTerminalProven {
			terminalProven = true
		}
	}
	for i := 0; i+1 < len(PilotMeasurableStages); i++ {
		from := PilotMeasurableStages[i]
		to := PilotMeasurableStages[i+1]
		numerator := 0
		denominator := 0
		for _, outcome := range outcomes {
			fromProven := false
			toProven := false
			for _, stage := range outcome.Stages {
				if stage.State != PilotStageStateProven {
					continue
				}
				if stage.Stage == from {
					fromProven = true
				}
				if stage.Stage == to {
					toProven = true
				}
			}
			if fromProven {
				denominator++
				if toProven {
					numerator++
				}
			}
		}
		word := "конверсия " + from + " → " + to + ": числитель — кейсы с доказанными И " + from + ", И " + to + "; знаменатель — кейсы с доказанным " + from
		if denominator == 0 {
			word += "; знаменатель нуль — конверсия не выводится"
		}
		report.Conversions = append(report.Conversions, PilotConversion{
			From:        from,
			To:          to,
			Numerator:   numerator,
			Denominator: denominator,
			Word:        word,
		})
	}
	unknownNumerator := 0
	if !terminalProven {
		unknownNumerator = len(outcomes)
	}
	report.UnknownShare = PilotShare{
		Numerator:   unknownNumerator,
		Denominator: len(outcomes),
		Word:        PilotUnknownShareWord,
	}
	return report
}

// ---------------------------------------------------------------------------
// The small honest helpers (the local words, no dependencies)
// ---------------------------------------------------------------------------

// itoa is the honest integer word (the counts of the chain words).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// joinWords joins the words with the comma separator.
func joinWords(words []string) string {
	out := ""
	for i, word := range words {
		if i > 0 {
			out += ", "
		}
		out += word
	}
	return out
}

// PilotStageEvidenceSortedByDictionary reports whether the stages arrive
// in the dictionary order (the render contract; the fold appends in the
// dictionary order by construction).
func PilotStageEvidenceSortedByDictionary(stages []PilotStageEvidence) bool {
	if len(stages) != len(PilotOutcomeStages) {
		return false
	}
	for i, stage := range stages {
		if stage.Stage != PilotOutcomeStages[i] {
			return false
		}
	}
	return true
}
