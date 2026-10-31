package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// Module action, story 4.10 — the Order Servicing domain: the SAFE
// servicing of the EXISTING order. The request is the RECORDED EVENT of
// the repeat contact (FR-62): the operator with the review scope
// records WHAT THE CLIENT ASKED — the intent word, the identifier the
// client named and the LIVE proof pins of the case assurance at the
// moment of the record. The servicing NEVER executes anything: no
// cancel path, no change path, no supplier-send path exists here or
// anywhere (the OQ-3 fence, the AG-4 manual contour) — the cancellation
// is SPOKEN in its four read-time states, never performed; the change
// is the manager's handwork in ABCP; the claim/dispute words route to
// the EXISTING contours 3-8/2-13 which the machines of THIS slice
// never touch. The repeated contact NEVER creates a new order
// operation (the resolved operation REFERENCES the existing one — the
// FR-62 structural pin). NO lookup adapter of the external systems
// exists (AG-4/OQ-18): the lookup reads the RECORDED material of the
// contour only — the receipts, the rereads, the facts. NO client
// delivery surface exists (OQ-1/OQ-2 — the 2-13 Never precedent of
// 4-9): the panel is the ONLY surface, the operator retells the client
// words by hand.

// ---------------------------------------------------------------------------
// The honest words of the slice (the text, never only color)
// ---------------------------------------------------------------------------

// ServicingIntentOrderStatusWord is the intent word of the status ask:
// the retell rides the LIVE projection words of 4-9 — the confirmed
// material only, never an invented state (AC-5).
const ServicingIntentOrderStatusWord = "статус заказа: слова живой проекции статуса (стадии/свежесть/следующий шаг/три слова срока) — только подтверждённые сведения"

// ServicingIntentClarificationWord is the intent word of the
// clarification ask (AC-6): the system facts stay separate from the
// estimate; the unknown routes to the human or the agreed callback.
const ServicingIntentClarificationWord = "объяснение: факты системы отделены от оценки (три слова срока 4-9); при неизвестном — проверка человеком или согласованный callback (контур 2-13/2-16)"

// ServicingIntentChangeWord is the intent word of the change ask
// (AG-4): the request is recorded, the change itself is the manager's
// handwork in ABCP — Staffy never claims the order changed before the
// human confirmation (AC-8 of the epic: the manual contour).
const ServicingIntentChangeWord = "запрос зафиксирован; изменение заказа менеджер выполняет в ABCP вручную (AG-4); Staffy не утверждает, что заказ изменён, до человеческого подтверждения; коррекция подбора до закупки — контур 4-8"

// ServicingIntentCancelWord is the intent word of the cancel ask
// (AC-7): the four read-time states ride the CancelFeasibility view —
// the cancel is NEVER promised (the catalog carries no cancel — the
// OQ-3 fence; the receipt does not exist — the honest negation).
const ServicingIntentCancelWord = "отмена различается четырьмя состояниями: запрос записан / подготовленное действие не существует / подтверждённая отмена не существует / невозможность как обычное редактирование — отмена не обещается (AG-4/OQ-3)"

// ServicingIntentReturnWord is the intent word of the return ask
// (AC-9): the claim/recovery mode — the recorded words route to the
// EXISTING contours 3-8/2-13, the legal liability is never admitted,
// the compensation is never promised.
const ServicingIntentReturnWord = "режим claim/recovery: исходные слова клиента сохранены (append-only запрос); создайте Recovery Case (контур 3-8) / Handoff (контур 2-13); юридическая ответственность не признаётся; компенсация не обещается; обращение передаётся уполномоченному человеку"

// ServicingIntentClaimWord is the intent word of the claim ask (AC-9):
// the advertising claim of the client — the recovery contour by hand.
const ServicingIntentClaimWord = "режим claim/recovery: исходные слова клиента сохранены (append-only запрос); создайте Recovery Case (контур 3-8) / Handoff (контур 2-13); юридическая ответственность не признаётся; компенсация не обещается; обращение передаётся уполномоченному человеку"

// ServicingIntentDisputeWord is the intent word of the dispute ask
// (AC-9/AC-14 of 4-9): the recorded words stay append-only — the
// history never rewrites; the recovery and the human check route to
// the EXISTING contours.
const ServicingIntentDisputeWord = "режим claim/recovery: исходные слова клиента сохранены (append-only запрос); история не переписывается; создайте Recovery Case (контур 3-8) / Handoff (контур 2-13); юридическая ответственность не признаётся; компенсация не обещается"

// ServicingIntentHumanWord is the intent word of the human ask: the
// handoff contour 2-13 with the order anchor — the owner and the next
// update word ride the machine of 2-13 (this slice speaks the routing
// word only).
const ServicingIntentHumanWord = "передача человеку: создайте Handoff (контур 2-13) с якорем заказа (operation_id в запросе) и причиной «запрос клиента по существующему заказу»; клиенту сообщаются owner и ожидаемое следующее обновление (машина 2-13)"

// ServicingGateWordPrefix is the gate word head (AC-1/FR-11/2-3): the
// disclosure — INCLUDING the very existence of the order — requires
// the LIVE verified trust of the contact case; the Caller ID, the chat
// profile name and the knowledge of the order number are NEVER a
// proof (the gate reads the verification records only — structurally).
const ServicingGateWordPrefix = "раскрытие требует проверки личности (уровень verified); разрешённые методы канала: "

// ServicingGateNoMethodsWord is the honest gate word when the LIVE
// assurance read carries no methods (the unwired contour or the
// unknown modality — never an invented list).
const ServicingGateNoMethodsWord = "раскрытие требует проверки личности (уровень verified); разрешённые методы канала не прочитаны из живого контура — сверьте панель проверки личности (контур 2-3)"

// ServicingLookupNotFoundWord is the generic not_found word (AC-2/
// AC-12): the absence NEVER confirms or denies the orders of another
// client — the honest generic word only.
const ServicingLookupNotFoundWord = "заказ по названному признаку не найден: проверьте номер с клиентом или передайте менеджеру"

// ServicingLookupAmbiguousWord is the ambiguous word (AC-2): the
// identifier matches several orders — the clarification stays with the
// manager, the LIST never renders (the enumeration does not exist).
const ServicingLookupAmbiguousWord = "признак соответствует нескольким заказам: уточнение у менеджера"

// ServicingLookupUniqueWord is the unique_match word (the operator
// pin): exactly one order of the tenant carries the named identifier.
const ServicingLookupUniqueWord = "найден единственный заказ по названному признаку"

// ServicingPhoneSearchBoundaryWord is the OQ-16 boundary word: the
// cross-case profile does not exist — the search by the phone/email is
// NEVER a lookup kind of this contour.
const ServicingPhoneSearchBoundaryWord = "поиск по телефону или email не существует: сквозной профиль по кейсам не строится (OQ-16) — поиск только по названному признаку заказа"

// ServicingConnectorBoundaryWord is the AG-4/OQ-18 boundary word: the
// lookup reads the RECORDED material of the contour only — never the
// live external systems.
const ServicingConnectorBoundaryWord = "поиск читает только записанный контуром материал: квитанции диспетчера, перечиты менеджера, факты поставщика (AG-4/OQ-18) — чтение внешних систем не подключено"

// ServicingNoDeliveryWord is the honest delivery word (OQ-1/OQ-2 —
// the 2-13 Never precedent of 4-9): the panel is the ONLY surface of
// the slice; nothing is sent to the client by the machine.
const ServicingNoDeliveryWord = "уведомления клиенту не отправляются: канал не подключён (OQ-1/OQ-2); панель — единственная поверхность; оператор пересказывает слова вручную"

// ServicingRequestRecordedWord is the honest always-word of the record
// command (AC-6): the request is recorded — the RESULT is never
// claimed (the append fixes the ask, not the outcome).
const ServicingRequestRecordedWord = "запрос клиента записан (append-only): фиксирует запрос, но не утверждает результат"

// The four CancelFeasibility states (AC-7) — the words, never only
// color; the cancellation is NEVER promised and NEVER executed.

// ServicingCancelRequestRecordedWord is the FIRST state: the cancel
// request is recorded (the append-only fact of this slice).
const ServicingCancelRequestRecordedWord = "запрос на отмену записан (append-only): запрос клиента сохранён, результат не утверждается"

// ServicingCancelNoPreparedActionWord is the SECOND state: the
// prepared action does not exist — the catalog carries no cancel (the
// OQ-3 fence; the 4-1 declaration names a command that does not exist
// — the deferred work №215).
const ServicingCancelNoPreparedActionWord = "подготовленное действие не существует: cancel отсутствует в каталоге действий (OQ-3 fence; декларация 4-1 — отмена подготовки отдельная команда до отправки, которой нет — deferred-work №215)"

// ServicingCancelNoConfirmationWord is the THIRD state: the confirmed
// cancellation does not exist — the cancel receipt is absent, the
// cancellation is never promised before the Action Receipt or the
// human confirmation of the Owning System.
const ServicingCancelNoConfirmationWord = "подтверждённая отмена не существует: квитанции отмены нет — не подтверждено; отмена не обещается до Action Receipt или человеческого подтверждения Owning System"

// ServicingCancelNotDefinedWord is the FOURTH state (no supplier
// facts): the impossibility-as-routine-editing is not defined — the
// supplier facts are absent, the honest negation speaks.
const ServicingCancelNotDefinedWord = "невозможность отмены как обычное редактирование: не определена — фактов поставщика нет"

// ServicingCancelSupplierTouchedWord is the FOURTH state (the supplier
// facts exist): the order touched the supplier — the cancel is NOT a
// routine editing, the human decides.
const ServicingCancelSupplierTouchedWord = "заказ затронул поставщика (записи формирования/отправки есть) — отмена НЕ выполняется как обычное редактирование: передаётся человеку"

// ServicingCancelManualWord is the AG-4 manual word of the cancel
// panel: the manager performs the cancel by hand — the order is not
// cancelled until the human confirmation.
const ServicingCancelManualWord = "отмену выполняет менеджер вручную (AG-4): до человеческого подтверждения заказ НЕ считается отменённым"

// ErrOrderServicingNoCase is the honest case refusal of the record
// command: the case does not exist (or the RLS blindness of a foreign
// case answers the same word — the 404).
var ErrOrderServicingNoCase = errors.New("кейс обращения не найден")

// ---------------------------------------------------------------------------
// The dictionaries (compile-time; never a tenant configuration)
// ---------------------------------------------------------------------------

// The eight servicing intents (BY NAME — the word of every intent is
// its boundary word). NO automatic classification exists: the operator
// picks the intent and records the client words by hand (the slice
// carries NO automatic client path).
const (
	ServicingIntentOrderStatus   = "order_status"
	ServicingIntentClarification = "clarification"
	ServicingIntentChange        = "change"
	ServicingIntentCancel        = "cancel"
	ServicingIntentReturn        = "return"
	ServicingIntentClaim         = "claim"
	ServicingIntentDispute       = "dispute"
	ServicingIntentHuman         = "human"
)

// ServicingIntents is the full intent dictionary (the form order).
var ServicingIntents = []string{
	ServicingIntentOrderStatus, ServicingIntentClarification, ServicingIntentChange,
	ServicingIntentCancel, ServicingIntentReturn, ServicingIntentClaim,
	ServicingIntentDispute, ServicingIntentHuman,
}

// ServicingIntentInDictionary reports whether the intent word belongs
// to the fixed dictionary.
func ServicingIntentInDictionary(word string) bool {
	return WordIn(ServicingIntents, word)
}

// ServicingIntentWords is the honest boundary word of every intent.
var ServicingIntentWords = map[string]string{
	ServicingIntentOrderStatus:   ServicingIntentOrderStatusWord,
	ServicingIntentClarification: ServicingIntentClarificationWord,
	ServicingIntentChange:        ServicingIntentChangeWord,
	ServicingIntentCancel:        ServicingIntentCancelWord,
	ServicingIntentReturn:        ServicingIntentReturnWord,
	ServicingIntentClaim:         ServicingIntentClaimWord,
	ServicingIntentDispute:       ServicingIntentDisputeWord,
	ServicingIntentHuman:         ServicingIntentHumanWord,
}

// ServicingIntentShortWords is the short form word of every intent
// (the select options of the record form — the readable labels of the
// operator surface; a11y: the text, never only color).
var ServicingIntentShortWords = map[string]string{
	ServicingIntentOrderStatus:   "статус заказа",
	ServicingIntentClarification: "объяснение (статус/условия)",
	ServicingIntentChange:        "изменение заказа",
	ServicingIntentCancel:        "отмена заказа",
	ServicingIntentReturn:        "возврат",
	ServicingIntentClaim:         "рекламация",
	ServicingIntentDispute:       "спор",
	ServicingIntentHuman:         "передача человеку",
}

// The two lookup identifier kinds (AC-2 of the epic: the identical
// external identifiers of DIFFERENT systems differ BY TYPE and BY
// SOURCE — the ABCP ID and the supplier ID are DIFFERENT TYPES).
const (
	LookupKindABCPClientOrderID = "abcp_client_order_id"
	LookupKindSupplierOrderID   = "supplier_order_id"
)

// LookupIdentifierKinds is the full kind dictionary.
var LookupIdentifierKinds = []string{LookupKindABCPClientOrderID, LookupKindSupplierOrderID}

// LookupIdentifierKindInDictionary reports whether the kind word
// belongs to the fixed dictionary.
func LookupIdentifierKindInDictionary(kind string) bool {
	return WordIn(LookupIdentifierKinds, kind)
}

// LookupKindSourceWords is the honest source word of every kind (the
// BY NAME pin of the panel: WHICH material carries the external ID).
var LookupKindSourceWords = map[string]string{
	LookupKindABCPClientOrderID: "номер заказа клиента в ABCP: квитанция диспетчерского контура 4-6 / перечит словами менеджера 4-8 (раздельные источники BY NAME)",
	LookupKindSupplierOrderID:   "номер заказа у поставщика: факты-записи менеджера 4-9 (квитанции контура магазина)",
}

// LookupKindShortWords is the short form word of every kind (the select
// options of the record form).
var LookupKindShortWords = map[string]string{
	LookupKindABCPClientOrderID: "номер заказа клиента в ABCP",
	LookupKindSupplierOrderID:   "номер заказа у поставщика",
}

// The three lookup outcomes (BY NAME — the enumeration of the orders
// does NOT exist: the unique match anchors ONE operation; the not_found
// and the ambiguous anchor NOTHING and never list).
const (
	LookupOutcomeUnique = "unique_match"
	LookupOutcomeNone   = "not_found"
	LookupOutcomeMany   = "ambiguous"
)

// LookupOutcomes is the full outcome dictionary.
var LookupOutcomes = []string{LookupOutcomeUnique, LookupOutcomeNone, LookupOutcomeMany}

// LookupOutcomeInDictionary reports whether the outcome word belongs to
// the fixed dictionary.
func LookupOutcomeInDictionary(word string) bool {
	return WordIn(LookupOutcomes, word)
}

// LookupOutcomeWords is the honest word of every outcome (the client
// words of not_found/ambiguous NEVER disclose another client's orders
// — the generic words only).
var LookupOutcomeWords = map[string]string{
	LookupOutcomeUnique: ServicingLookupUniqueWord,
	LookupOutcomeNone:   ServicingLookupNotFoundWord,
	LookupOutcomeMany:   ServicingLookupAmbiguousWord,
}

// The proof states — the BY NAME mirror of the 2-3 assurance
// dictionary {none, basic, verified} plus the step_up window word (the
// threshold flag of the derivation). The DISCLOSURE requires verified;
// none, basic and step_up answer the gate word (the Caller ID, the
// profile name and the order-number knowledge are NEVER a proof — the
// gate reads the verification records only, structurally).
const (
	ProofStateNone     = "none"
	ProofStateBasic    = "basic"
	ProofStateVerified = "verified"
	ProofStateStepUp   = "step_up"
)

// ProofStates is the full proof-state dictionary (the 2-3 mirror plus
// the step-up window word).
var ProofStates = []string{ProofStateNone, ProofStateBasic, ProofStateVerified, ProofStateStepUp}

// ProofStateInDictionary reports whether the proof-state word belongs
// to the fixed dictionary.
func ProofStateInDictionary(word string) bool {
	return WordIn(ProofStates, word)
}

// ProofStateAllowsDisclosure reports whether the proof state opens the
// order disclosure: verified ONLY (AC-1). The none/basic/step_up states
// answer the gate word — the lookup outcome NEVER rides the client
// words of a denied request (the existence of the order is not
// disclosed).
func ProofStateAllowsDisclosure(state string) bool {
	return state == ProofStateVerified
}

// The two disclosure outcomes of the record (BY NAME): the allowed
// disclosure rides the LIVE client words of the 4-9 projection; the
// denied disclosure rides the gate word.
const (
	DisclosureAllowed = "allowed"
	DisclosureDenied  = "denied"
)

// DisclosureOutcomes is the full disclosure dictionary.
var DisclosureOutcomes = []string{DisclosureAllowed, DisclosureDenied}

// DisclosureInDictionary reports whether the disclosure word belongs
// to the fixed dictionary.
func DisclosureInDictionary(word string) bool {
	return WordIn(DisclosureOutcomes, word)
}

// The identifier value length bounds (the typed field of the record
// command; the CHECK of the migration mirrors them).
const (
	ServicingIdentifierMinLen = 1
	ServicingIdentifierMaxLen = 200
)

// ---------------------------------------------------------------------------
// The material (the store reads/writes; the derivations are pure)
// ---------------------------------------------------------------------------

// OrderServicingRequest is ONE append-only recorded-event row of the
// case: the request number (the monotonic per-case ladder — the server
// truth of the feed order), the intent word, the TYPED identifier the
// client named, the resolved operation anchor (zero when not_found or
// ambiguous — the enumeration does not exist), the lookup outcome, the
// LIVE proof pins copied at the moment of the record (the state, the
// method, the issued/expiry times — the scope, the level and the time
// of the check are preserved, AC-4), the disclosure outcome and the
// actor identity with the server recorded time.
type OrderServicingRequest struct {
	TenantID            id.UUID
	CaseID              id.UUID
	ConversationID      id.UUID
	RequestNo           int64
	Intent              string
	IdentifierKind      string
	IdentifierValue     string
	ResolvedOperationID id.UUID
	LookupOutcome       string
	ProofState          string
	ProofMethod         string
	ProofIssuedAt       time.Time
	ProofExpiresAt      time.Time
	Disclosure          string
	ActorID             id.UUID
	RecordedAt          time.Time
}

// OrderLookupCandidate is ONE operation of the lookup BY KEY: the
// operation anchor with its case (the view material assembly of the
// disclosure) and the SOURCE pins of the match (BY NAME — the receipt
// of the dispatch machine 4-6 and/or the reread of the manager 4-8 for
// the ABCP kind; the human facts 4-9 for the supplier kind). The
// candidate carries NO price material, NO send material and NO
// notification material — the identifier match only.
type OrderLookupCandidate struct {
	OperationID id.UUID
	CaseID      id.UUID
	Sources     []string
}

// ServicingGateWord renders the gate word with the LIVE allowed methods
// of the contact case conversation (the port read supplies them — never
// a domain constant): the methods ride the word of the refusal, the
// identifier values NEVER leak. The empty method list answers the
// honest no-methods word (the unwired contour — never «ok»).
func ServicingGateWord(allowedMethods []string) string {
	if len(allowedMethods) == 0 {
		return ServicingGateNoMethodsWord
	}
	return ServicingGateWordPrefix + strings.Join(allowedMethods, ", ")
}

// DeriveLookupOutcome derives the lookup outcome BY KEY over the
// candidates of the recorded material: ZERO candidates — not_found
// (the generic word, never a disclosure of another client's orders);
// ONE operation — unique_match with the anchor (the sources ride the
// panel BY NAME); TWO OR MORE operations — ambiguous (the word without
// the list — the enumeration does not exist). The SAME operation
// matched by SEVERAL sources stays the unique match (the sources are
// the pins, not the rows).
func DeriveLookupOutcome(candidates []OrderLookupCandidate) (outcome string, resolved id.UUID) {
	distinct := map[id.UUID]bool{}
	for _, candidate := range candidates {
		distinct[candidate.OperationID] = true
	}
	switch len(distinct) {
	case 0:
		return LookupOutcomeNone, id.UUID("")
	case 1:
		var only id.UUID
		for operationID := range distinct {
			only = operationID
		}
		return LookupOutcomeUnique, only
	default:
		return LookupOutcomeMany, id.UUID("")
	}
}

// DeriveCancelFeasibility derives the FOUR read-time states of the
// cancel ask over the LIVE Order Status Projection of the resolved
// operation (AC-7): the recorded request (the append-only fact of this
// slice), the absent prepared action (the catalog carries no cancel —
// the OQ-3 fence), the absent confirmation (the cancel receipt does
// not exist — never promised) and the fourth state — the supplier
// touch: the human {formed, sent} facts of 4-9 name the order that
// touched the supplier (the cancel is NOT a routine editing — the
// human decides). The derivation writes NOTHING and NEVER cancels:
// the words are the material of the retell.
func DeriveCancelFeasibility(p OrderStatusProjection) CancelFeasibilityView {
	touched := false
	for _, stage := range p.Stages {
		if (stage.Stage == OrderStageSupplierFormed || stage.Stage == OrderStageSupplierSent) && stage.Confirmed {
			touched = true
		}
	}
	view := CancelFeasibilityView{
		RequestRecordedWord: ServicingCancelRequestRecordedWord,
		PreparedActionWord:  ServicingCancelNoPreparedActionWord,
		ConfirmedWord:       ServicingCancelNoConfirmationWord,
		SupplierTouched:     touched,
		ManualContourWord:   ServicingCancelManualWord,
		NotEditableWord:     ServicingCancelNotDefinedWord,
	}
	if touched {
		view.NotEditableWord = ServicingCancelSupplierTouchedWord
	}
	return view
}

// CancelFeasibilityView is the four-state view of the cancel ask
// (AC-7 — the words, never only color): the recorded request, the
// absent prepared action, the absent confirmation and the
// impossibility-as-routine-editing state (the supplier touch flag
// rides along — the panel renders the human word either way) with the
// AG-4 manual contour word.
type CancelFeasibilityView struct {
	RequestRecordedWord string
	PreparedActionWord  string
	ConfirmedWord       string
	NotEditableWord     string
	SupplierTouched     bool
	ManualContourWord   string
}

// DeriveServicingClientWords derives the CLIENT retell words of ONE
// request at read time (AC-5/AC-1 — the LIVE derivation, never a
// stored status): the DENIED request answers the gate word with the
// LIVE allowed methods and the fallback words of 2-3 — the lookup
// outcome NEVER rides the denied words (the very existence of the
// order is not disclosed, AC-1); the ALLOWED request of a unique match
// answers the LIVE client words of the 4-9 projection of the resolved
// operation (the confirmed stages, the freshness, the next step, the
// three deadline words, the boundaries) plus the intent boundary word;
// the ALLOWED request of not_found/ambiguous answers the GENERIC
// outcome word (safe by construction — never a disclosure of another
// client's orders) plus the intent boundary word. The projection is
// the LIVE read of the panel (nil when the resolved operation is
// not readable anymore — the honest anchor-less retell). The words are
// the RETELL MATERIAL — nothing is sent to the client by the machine
// (OQ-1/OQ-2).
func DeriveServicingClientWords(r OrderServicingRequest, projection *OrderStatusProjection, allowedMethods []string, fallbackWords []string) []string {
	if !ProofStateAllowsDisclosure(r.ProofState) {
		words := []string{ServicingGateWord(allowedMethods)}
		return append(words, fallbackWords...)
	}
	words := []string{}
	if r.LookupOutcome == LookupOutcomeUnique {
		if projection != nil {
			words = append(words, projection.ClientWords...)
		}
	} else if outcomeWord, ok := LookupOutcomeWords[r.LookupOutcome]; ok {
		words = append(words, outcomeWord)
	}
	if intentWord, ok := ServicingIntentWords[r.Intent]; ok {
		words = append(words, intentWord)
	}
	return words
}

// GuardOrderServicingRequest refuses the record command honestly: the
// intent word must belong to the 8-word dictionary, the identifier
// kind must belong to the 2-word dictionary (the phone/email kinds do
// NOT exist — OQ-16), the identifier value must be a non-empty word
// within the typed bound. The guard NEVER reads the proof state: the
// request records at ANY trust level (AC-6 — the append fixes the ask),
// the DISCLOSURE gate lives in the derivation, not here.
func GuardOrderServicingRequest(intent, identifierKind, identifierValue string) error {
	if !ServicingIntentInDictionary(intent) {
		return RuleError{"order_servicing.intent", fmt.Sprintf(
			"Интент обращения обязан быть словом словаря {%s}.", strings.Join(ServicingIntents, ", "))}
	}
	if !LookupIdentifierKindInDictionary(identifierKind) {
		return RuleError{"order_servicing.identifier_kind", fmt.Sprintf(
			"Тип признака обязан быть словом словаря {%s}; поиск по телефону или email не существует (OQ-16).",
			strings.Join(LookupIdentifierKinds, ", "))}
	}
	trimmed := strings.TrimSpace(identifierValue)
	if len(trimmed) < ServicingIdentifierMinLen || len(trimmed) > ServicingIdentifierMaxLen {
		return RuleError{"order_servicing.identifier_value", fmt.Sprintf(
			"Значение признака обязательно (от %d до %d символов) — признак, названный клиентом.",
			ServicingIdentifierMinLen, ServicingIdentifierMaxLen)}
	}
	return nil
}

// ---------------------------------------------------------------------------
// The view (the read-time panel material — the words re-derive live)
// ---------------------------------------------------------------------------

// OrderServicingRequestView is ONE request of the panel feed: the
// recorded row (the append-only history — the proof pins stay as they
// were recorded, AC-4) with the LIVE-derived render words (the client
// words re-derive over the live projection of the resolved operation —
// never a stored status; the cancel feasibility re-derives over the
// live supplier facts). The short words of the form dictionaries ride
// along (the readable labels of the operator surface).
type OrderServicingRequestView struct {
	Request         OrderServicingRequest
	IntentShortWord string
	KindShortWord   string
	KindSourceWord  string
	OutcomeWord     string
	// ClientWords is the LIVE retell material of the request (the gate
	// word + the fallback words of a denied request — WITHOUT the
	// lookup outcome; the live projection words + the intent boundary
	// word of an allowed unique match; the generic outcome word + the
	// intent word of an allowed miss).
	ClientWords []string
	// Cancel is the LIVE four-state view of the cancel intent (nil for
	// the other intents — the derivation rides the cancel ask only).
	Cancel *CancelFeasibilityView
	// Sources is the BY NAME source pin list of the resolved operation
	// match (empty when not_found/ambiguous — the enumeration does not
	// exist; the sources of the unique match ride the panel).
	Sources []string
}

// OrderServicingView is the WHOLE panel view of the case servicing
// contour: the recorded requests (the append-only feed ordered by the
// request ladder) with the LIVE derivations, and the honest boundary
// words of the contour (the OQ-16 phone-search refusal, the AG-4/OQ-18
// connector boundary, the OQ-1/OQ-2 no-delivery word and the AC-6
// recorded-word). The view writes NOTHING — the read is pure.
type OrderServicingView struct {
	Requests              []OrderServicingRequestView
	PhoneBoundaryWord     string
	ConnectorBoundaryWord string
	NoDeliveryWord        string
	RequestRecordedWord   string
}
