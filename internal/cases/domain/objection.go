package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The objection surface (Story 2.8): the dictionaries, the versioned
// playbook v1, the record guards and the persuasion-stop derivations
// ---------------------------------------------------------------------------
//
// The dictionaries of the Domain Pack v0 are fixed: the objection types
// (trust, price_comparison, general_doubt), the client signals (concern,
// refusal, stop_intent, human_request) and the playbook steps
// (acknowledge_concern, present_evidence, honest_no_answer,
// offer_alternative, clarify_question). No word appears outside its
// dictionary — the CHECK backstops of the table and the guards carry the
// same words (the ruleTableV1 precedent of 2.6/2.7).
//
// The playbook v1 is the fixed rule table: every objection type names the
// ordered allowed steps of the honest persuasion discipline. The FIRST
// linked answer of a current objection is ALWAYS acknowledge_concern
// («сначала признаёт смысл опасения»); the budget of the persuasion is
// the fixed threshold of the refusal records of one Work Item (lifetime,
// never reset — the append-only monotone accumulation). The stop word is
// the read-time derivation: the stop_intent or the human_request signal
// of the CURRENT objection stops immediately; the refusal count at the
// threshold stops the persuasion for good. The mode raise itself NEVER
// happens here: propose_mode (2.5) stays the only path of the transition
// — the record only marks the trigger signal.
//
// The spans are the verbatim excerpts of the client words of the accepted
// text turn (2-1, the sealed payload): the honest record demands the
// byte-exact substring of the payload — an invented excerpt is impossible
// (the span honesty of AC-4). The voice turns refuse honestly: the
// excerpt markup of the voice contour belongs to the later slices.

// ---------------------------------------------------------------------------
// Dictionaries
// ---------------------------------------------------------------------------

// ObjectionPlaybookVersion is the fixed dictionary word of the v1 playbook
// (the snapshot word every objection record carries — the versioned
// playbook of AC-4).
const ObjectionPlaybookVersion = "objection-playbook-v1"

// The objection types: the fixed dictionary of the material kinds of the
// client objections (AC-5 trust, AC-6 price_comparison, the honest
// general_doubt of the unclear concern).
const (
	ObjectionTypeTrust           = "trust"
	ObjectionTypePriceComparison = "price_comparison"
	ObjectionTypeGeneralDoubt    = "general_doubt"
)

// ObjectionTypes is the full dictionary.
var ObjectionTypes = []string{ObjectionTypeTrust, ObjectionTypePriceComparison, ObjectionTypeGeneralDoubt}

// The client signals: the fixed dictionary of what the client said. concern
// is the ordinary objection (the persuasion continues); refusal is the
// refusal (the budget counts it — «повторно отказывается»); stop_intent is
// the immediate stop («просит не связываться» — the privacy_no_contact
// raise stays the propose_mode command of 2.5); human_request is the
// immediate stop with the handoff path (the human_requested mode raise
// stays the propose_mode command of 2.5).
const (
	ClientSignalConcern      = "concern"
	ClientSignalRefusal      = "refusal"
	ClientSignalStopIntent   = "stop_intent"
	ClientSignalHumanRequest = "human_request"
)

// ClientSignals is the full dictionary.
var ClientSignals = []string{ClientSignalConcern, ClientSignalRefusal, ClientSignalStopIntent, ClientSignalHumanRequest}

// The playbook steps: the fixed dictionary of the honest persuasion
// techniques (the machine frame of AC-5/AC-6 — the content check of the
// invented reviews/certificates is the AI-candidate path of the later
// slices; the machine reads no content).
const (
	PlaybookStepAcknowledgeConcern = "acknowledge_concern"
	PlaybookStepPresentEvidence    = "present_evidence"
	PlaybookStepHonestNoAnswer     = "honest_no_answer"
	PlaybookStepOfferAlternative   = "offer_alternative"
	PlaybookStepClarifyQuestion    = "clarify_question"
)

// PlaybookSteps is the full dictionary.
var PlaybookSteps = []string{PlaybookStepAcknowledgeConcern, PlaybookStepPresentEvidence, PlaybookStepHonestNoAnswer, PlaybookStepOfferAlternative, PlaybookStepClarifyQuestion}

// PersuasionThresholdV1 is the fixed budget word of the playbook v1: the
// number of the refusal records of one Work Item that stops the
// persuasion (lifetime, never reset, never spent by the system — the
// append-only fail-safe of AC-7).
const PersuasionThresholdV1 = 2

// The span limits (the fixed dictionary of the excerpt discipline): one
// record carries at most 8 spans, every span at most 200 runes.
const (
	MaxObjectionSpans     = 8
	MaxObjectionSpanRunes = 200
)

// ---------------------------------------------------------------------------
// The playbook v1
// ---------------------------------------------------------------------------

// playbookV1 is the fixed rule table of the versioned playbook (the
// ruleTableV1 precedent of 2.6): every objection type names the ordered
// allowed steps of the honest persuasion. trust begins with the
// acknowledgement and the evidence; price_comparison offers the
// alternative before the honest absence; general_doubt clarifies the
// question before the evidence.
var playbookV1 = map[string][]string{
	ObjectionTypeTrust: {
		PlaybookStepAcknowledgeConcern, PlaybookStepPresentEvidence,
		PlaybookStepHonestNoAnswer, PlaybookStepOfferAlternative,
	},
	ObjectionTypePriceComparison: {
		PlaybookStepAcknowledgeConcern, PlaybookStepPresentEvidence,
		PlaybookStepOfferAlternative, PlaybookStepHonestNoAnswer,
	},
	ObjectionTypeGeneralDoubt: {
		PlaybookStepAcknowledgeConcern, PlaybookStepClarifyQuestion,
		PlaybookStepPresentEvidence, PlaybookStepHonestNoAnswer,
	},
}

// PlaybookStepsOf resolves the ordered allowed steps of one objection type
// (the copy of the slice — the caller never mutates the rule table).
func PlaybookStepsOf(objectionType string) []string {
	steps, ok := playbookV1[objectionType]
	if !ok {
		return nil
	}
	return append([]string(nil), steps...)
}

// PlaybookAllowsStep reports whether the step belongs to the playbook of
// the objection type (the step outside the playbook of the type is the
// honest 400 — the machine frame of the honest techniques).
func PlaybookAllowsStep(objectionType, step string) bool {
	for _, allowed := range playbookV1[objectionType] {
		if allowed == step {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// The objection record
// ---------------------------------------------------------------------------

// ObjectionRecord is one append-only row of the client objection: the Work
// Item scope, the verbatim spans of the client words of the accepted turn
// (the source anchor), the dictionary words (the type, the signal), the
// snapshot of the playbook version (the versioned choice of the moment,
// AC-4), the acting operator and the monotonic per-Work-Item number. The
// CURRENT objection of a Work Item is the maximal number — a new record
// supersedes by a NEW row, never by an update.
type ObjectionRecord struct {
	TenantID        id.UUID
	CaseID          id.UUID
	WorkItemID      id.UUID
	ObjectionID     id.UUID
	ObjectionNo     int64
	TurnRef         id.UUID
	ObjectionType   string
	ClientSignal    string
	Spans           []string
	PlaybookVersion string
	ActorID         id.UUID
	CreatedAt       time.Time
}

// CurrentObjection resolves the CURRENT objection of one Work Item: the
// maximal objection number (nil when no objection exists — the honest
// pre-objection state of the panel).
func CurrentObjection(records []ObjectionRecord) *ObjectionRecord {
	var current *ObjectionRecord
	for i := range records {
		if current == nil || records[i].ObjectionNo > current.ObjectionNo {
			current = &records[i]
		}
	}
	return current
}

// ---------------------------------------------------------------------------
// Command guards
// ---------------------------------------------------------------------------

// GuardObjectionRecord validates the material of one objection record: the
// type and the signal are the dictionary words; the spans are 1..8, every
// span a non-empty bounded string without duplicates (the byte-exact
// substring check of the payload is the app-side honesty — the SQL never
// counts substrings, honestly).
func GuardObjectionRecord(objectionType, clientSignal string, spans []string) error {
	if !inDictionary(ObjectionTypes, objectionType) {
		return fmt.Errorf("тип возражения «%s» не входит в словарь (%s)", objectionType, joinQuoted(ObjectionTypes))
	}
	if !inDictionary(ClientSignals, clientSignal) {
		return fmt.Errorf("сигнал клиента «%s» не входит в словарь (%s)", clientSignal, joinQuoted(ClientSignals))
	}
	if len(spans) < 1 {
		return fmt.Errorf("возражение обязано нести хотя бы одну дословную выдержку слов клиента (1..%d)", MaxObjectionSpans)
	}
	if len(spans) > MaxObjectionSpans {
		return fmt.Errorf("возражение несёт не больше %d выдержек, получено %d", MaxObjectionSpans, len(spans))
	}
	seen := make(map[string]bool, len(spans))
	for _, span := range spans {
		if span == "" {
			return fmt.Errorf("выдержка не бывает пустой: дословные слова клиента обязательны")
		}
		if utf8.RuneCountInString(span) > MaxObjectionSpanRunes {
			return fmt.Errorf("выдержка длиннее %d символов", MaxObjectionSpanRunes)
		}
		if seen[span] {
			return fmt.Errorf("выдержки не повторяются: дубликат «%s»", span)
		}
		seen[span] = true
	}
	return nil
}

// GuardPlaybookStep validates the step word of the reply request against
// the step dictionary (the empty word is the honest absence — the gate
// decides whether the absence is permitted).
func GuardPlaybookStep(step string) error {
	if step == "" {
		return nil
	}
	if !inDictionary(PlaybookSteps, step) {
		return WorkRuleError{Rule: "objection_step_dictionary", Reason: fmt.Sprintf("шаг playbook «%s» не входит в словарь (%s)", step, joinQuoted(PlaybookSteps))}
	}
	return nil
}

// SpanInPayload reports the byte-exact containment of one span in the
// sealed payload bytes: the honest excerpt is the substring of the client
// words — an invented excerpt is impossible (AC-4).
func SpanInPayload(span string, payload []byte) bool {
	return bytes.Contains(payload, []byte(span))
}

// TerminalWorkItemObjectionText is the honest refusal of the objection
// record of a terminal Work Item (the write-command variant of the 2-2
// boundary — the merged item lives on in its target).
const TerminalWorkItemObjectionText = "Терминальный элемент работы не принимает возражений: resolved и merged — финальные состояния; для merged записывайте возражение на целевой элемент работы."

// ---------------------------------------------------------------------------
// The persuasion-state derivations
// ---------------------------------------------------------------------------

// ReplyLink is one outgoing reply linked to an objection: the objection id
// with the playbook step word the reply carried (the acknowledged
// derivation reads the linked replies — the state is never stored).
type ReplyLink struct {
	ObjectionID id.UUID
	Step        string
}

// ObjectionState is the read-time persuasion state of one Work Item: the
// CURRENT objection (the maximal number), the lifetime refusal count, the
// stopped word (the immediate stop signal of the current objection OR the
// refusal count at the threshold) and the acknowledged word (a linked
// reply with acknowledge_concern of the current objection). The state is
// the derivation of the append-only rows — nothing is ever mutated.
type ObjectionState struct {
	Current      *ObjectionRecord
	Refusals     int
	Stopped      bool
	Acknowledged bool
}

// DeriveObjectionState derives the persuasion state of one Work Item: the
// current objection is the maximal number; the refusals count EVERY
// refusal record of the item (the lifetime budget — never reset, never
// spent); the stop word fires on the stop signal of the current objection
// (stop_intent / human_request — the immediate stop) OR on the refusal
// count reaching the threshold (the repeated refusals of AC-7); the
// acknowledged word fires when a linked reply carried acknowledge_concern
// FOR THE CURRENT objection (the first answer of the new objection
// acknowledges anew).
func DeriveObjectionState(records []ObjectionRecord, linkedReplies []ReplyLink) ObjectionState {
	state := ObjectionState{Current: CurrentObjection(records)}
	for _, record := range records {
		if record.ClientSignal == ClientSignalRefusal {
			state.Refusals++
		}
	}
	if state.Current != nil && (state.Current.ClientSignal == ClientSignalStopIntent || state.Current.ClientSignal == ClientSignalHumanRequest) {
		state.Stopped = true
	}
	if state.Refusals >= PersuasionThresholdV1 {
		state.Stopped = true
	}
	if state.Current != nil {
		for _, link := range linkedReplies {
			if link.ObjectionID == state.Current.ObjectionID && link.Step == PlaybookStepAcknowledgeConcern {
				state.Acknowledged = true
				break
			}
		}
	}
	return state
}

// ---------------------------------------------------------------------------
// The sales-reply gate
// ---------------------------------------------------------------------------

// ObjectionStoppedError is the honest persuasion-stop refusal of the sales
// reply: the refusal words with the budget numbers and the allowed
// endings (the neutral completion, the protected request, the human
// handoff — the machine-readable permitted path of AC-7).
type ObjectionStoppedError struct {
	WorkItemID    id.UUID
	Refusals      int
	Threshold     int
	CurrentSignal string
	PlaybookStep  string
	Reason        string
}

func (e ObjectionStoppedError) Error() string { return e.Reason }

// ObjectionStoppedText is the fixed refusal text of the stopped
// persuasion: the sales path is closed; the allowed endings are the
// neutral completion, the protected request and the human handoff.
const ObjectionStoppedText = "Убеждение остановлено: клиент отказал повторно или попросил прекратить/передать человеку. Продажные ответы запрещены; разрешены только вежливое завершение (нейтральный класс), запрос приватности (protected_request) и передача человеку."

// ObjectionStepRequiredText is the fixed refusal text of the sales reply
// without the playbook step against a current objection: the discipline is
// mandatory, never optional.
const ObjectionStepRequiredText = "При действующем возражении продажный ответ обязан нести шаг playbook (playbook_step): дисциплина ответа не опциональна."

// ObjectionStepForeignText is the fixed refusal text of the sales reply
// carrying a playbook step without any objection: the step words belong to
// the objection path only.
const ObjectionStepForeignText = "Шаг playbook принадлежит ответу при действующем возражении: возражений на элементе работы нет — оставьте поле пустым."

// ObjectionStepOutsidePlaybookText is the fixed refusal text of the step
// outside the playbook of the objection type.
const ObjectionStepOutsidePlaybookText = "Шаг playbook не входит в последовательность типа возражения: выберите шаг словаря playbook этого типа."

// ObjectionAcknowledgeFirstText is the fixed refusal text of the first
// linked answer with a non-acknowledging step: «сначала признаёт смысл
// опасения» (AC-4).
const ObjectionAcknowledgeFirstText = "Сначала отправьте acknowledge_concern: первый связанный ответ признаёт смысл опасения клиента, лишь затем доказательство или честное отсутствие ответа."

// GuardSalesReplyAgainstObjections is the sales-reply gate of Story 2.8
// (AFTER the 2-6 policy gate, the sales class only): the stopped
// persuasion refuses with the allowed endings; no objection exists → the
// step must be empty (the byte-identical old path); a current objection →
// the step is REQUIRED, dictionary-bounded, inside the playbook of the
// type and the first linked answer is ALWAYS acknowledge_concern.
func GuardSalesReplyAgainstObjections(state ObjectionState, step string) error {
	if state.Stopped {
		return ObjectionStoppedError{
			Refusals: state.Refusals, Threshold: PersuasionThresholdV1,
			CurrentSignal: signalOfCurrent(state.Current), Reason: ObjectionStoppedText,
		}
	}
	if state.Current == nil {
		if step != "" {
			return WorkRuleError{Rule: "objection_step_foreign", Reason: ObjectionStepForeignText}
		}
		return nil
	}
	if step == "" {
		return WorkRuleError{Rule: "objection_step_required", Reason: ObjectionStepRequiredText}
	}
	if !inDictionary(PlaybookSteps, step) {
		return WorkRuleError{Rule: "objection_step_dictionary", Reason: fmt.Sprintf("шаг playbook «%s» не входит в словарь (%s)", step, joinQuoted(PlaybookSteps))}
	}
	if !PlaybookAllowsStep(state.Current.ObjectionType, step) {
		return WorkRuleError{Rule: "objection_step_playbook", Reason: ObjectionStepOutsidePlaybookText}
	}
	if !state.Acknowledged && step != PlaybookStepAcknowledgeConcern {
		return WorkRuleError{Rule: "objection_acknowledge_first", Reason: ObjectionAcknowledgeFirstText}
	}
	return nil
}

// signalOfCurrent resolves the signal word of the current objection (the
// empty word of the honest absence).
func signalOfCurrent(current *ObjectionRecord) string {
	if current == nil {
		return ""
	}
	return current.ClientSignal
}

// ---------------------------------------------------------------------------
// The outbox payload (the safe canonical JSON)
// ---------------------------------------------------------------------------

// Outbox event types of the objection surface (the 2-1 bounded
// vocabulary).
const (
	OutboxEventObjectionRecorded   = "case.objection_recorded"
	OutboxEventObjectionRecordedV1 = 1
)

// ObjectionRecordedPayload is the safe canonical JSON of the objection
// event: the identifiers, the dictionary words and the monotonic number —
// the SPAN TEXT never enters the envelope (the client words already live
// in the sealed payload of 2-1; the «кандидат не покидает строку»
// precedent of 2.6).
type ObjectionRecordedPayload struct {
	TenantID        id.UUID   `json:"tenant_id"`
	CaseID          id.UUID   `json:"case_id"`
	WorkItemID      id.UUID   `json:"work_item_id"`
	ObjectionID     id.UUID   `json:"objection_id"`
	ObjectionNo     int64     `json:"objection_no"`
	TurnRef         id.UUID   `json:"turn_ref"`
	ObjectionType   string    `json:"objection_type"`
	ClientSignal    string    `json:"client_signal"`
	PlaybookVersion string    `json:"playbook_version"`
	ActorID         id.UUID   `json:"actor_id"`
	OccurredAt      time.Time `json:"occurred_at"`
}

// DedupKeyObjectionRecorded builds the dedup key of one objection event.
func DedupKeyObjectionRecorded(objectionID id.UUID) string {
	return "case.objection_recorded:" + objectionID.String()
}

// ObjectionRecordedPayloadJSON renders the safe canonical JSON.
func ObjectionRecordedPayloadJSON(payload ObjectionRecordedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
