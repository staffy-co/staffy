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
// The recorded communication preferences and the honest alternative paths
// (Story 2.11, first slice): the dictionaries, the read-time derivation,
// the record guard and the accuracy qualifier gate
// ---------------------------------------------------------------------------
//
// A communication preference is an OPERATIONAL setting of the service, not
// a fact of the case: the client words («не могу фото», «пишите текстом»,
// «звоните после 18:00») become append-only revision rows of a table that
// lives OUTSIDE the fact channels — no confirmation statuses, no fact
// versions, no facts digest, no commercial material, never a medical
// diagnosis and never an identity proof. The append-only revisions keep the
// full history; the CURRENT set is the read-time derivation of the latest
// revision of every key (a clearing revision removes the key from the
// active set — the honest «редактируемые» of AC-1).
//
// The honest alternatives are the machine's offer of the permitted
// continuation paths: when a MANDATORY media-usual fact of the rule table
// is not confirmed and the client's no_photo is active, the Policy Outcome
// gains the active alternative words (the question NEVER leaves the plan —
// an alternative is an honest path of serving without media, not a
// replacement of the question). Every alternative word carries its fixed
// accuracy-limitation text (the dictionary of the Domain Pack v0 — the
// panel renders it, the database and the outbox carry the words only). A
// media key whose pack dictionary carries no alternatives fails closed
// with the human handoff — the honest stop, never a weakened check.
//
// The sales reply of an outcome with active alternatives MUST carry the
// accuracy qualifier word of the outcome's active set (the subset of the
// OUTCOME, stricter than the dictionary: the absence of media never
// becomes an invented exact result — the structural honesty of AC-2). The
// gate mirrors the objection gate of 2.8: required when the alternatives
// are active, empty when they are not — the field has exactly one meaning.

// ---------------------------------------------------------------------------
// The preference key dictionary (the fixed Domain Pack v0)
// ---------------------------------------------------------------------------

// The preference keys: six flag keys (the boolean marks of the
// communication accommodations) and two valued keys (the preferred channel
// — a word of the 2.1 channel dictionary; the availability line — a free
// operational text, never a schedule: the machine does not parse it and
// promises no timing).
const (
	AccommodationNoPhoto           = "no_photo"
	AccommodationNoWeb             = "no_web"
	AccommodationNoVoice           = "no_voice"
	AccommodationTextFirst         = "text_first"
	AccommodationSlowedPace        = "slowed_pace"
	AccommodationMandatoryReadback = "mandatory_readback"
	AccommodationPreferredChannel  = "preferred_channel"
	AccommodationAvailableTime     = "available_time"
)

// AccommodationFlagKeys is the flag half of the dictionary (no value).
var AccommodationFlagKeys = []string{
	AccommodationNoPhoto, AccommodationNoWeb, AccommodationNoVoice,
	AccommodationTextFirst, AccommodationSlowedPace, AccommodationMandatoryReadback,
}

// AccommodationValuedKeys is the valued half of the dictionary (a value is
// required).
var AccommodationValuedKeys = []string{AccommodationPreferredChannel, AccommodationAvailableTime}

// AccommodationKeys is the full dictionary (the flags first, the valued
// last — the canonical display order of the panel).
var AccommodationKeys = append(append([]string{}, AccommodationFlagKeys...), AccommodationValuedKeys...)

// AccommodationKeyIsFlag reports whether the key is a flag key (the value
// must stay empty — the dictionary is the truth of the CHECK).
func AccommodationKeyIsFlag(key string) bool {
	return inDictionary(AccommodationFlagKeys, key)
}

// AccommodationKeyInDictionary reports whether the key belongs to the
// fixed dictionary (the fail-closed basis of the record guard).
func AccommodationKeyInDictionary(key string) bool {
	return inDictionary(AccommodationKeys, key)
}

// ---------------------------------------------------------------------------
// The alternative path dictionary (the honest machine offer)
// ---------------------------------------------------------------------------

// The alternative path words: the permitted continuation paths of the
// service without the missing media. Every word carries its fixed honest
// accuracy-limitation text — the panel renders the text, the database and
// the outbox carry the words only.
const (
	AlternativeTextDescription = "text_description"
	AlternativeRangeEstimate   = "range_estimate"
	AlternativeCallback        = "callback"
	AlternativeHumanHandoff    = "human_handoff"
)

// AlternativePathWords is the full dictionary.
var AlternativePathWords = []string{
	AlternativeTextDescription, AlternativeRangeEstimate, AlternativeCallback, AlternativeHumanHandoff,
}

// AlternativeLimitationTexts is the fixed honest text of every alternative
// word (the Domain Pack v0 render material — «прямо сообщает ограничения
// точности» of AC-2; the texts never enter the database or the outbox).
var AlternativeLimitationTexts = map[string]string{
	AlternativeTextDescription: "Точность ниже фото: определение по текстовому описанию может ошибаться — итог подтверждает человек.",
	AlternativeRangeEstimate:   "Только диапазон, не точный результат: без фото цена и срок — оценкой диапазона, не точным значением.",
	AlternativeCallback:        "Уточнение голосом с человеком: сотрудник перезвонит и подтвердит детали.",
	AlternativeHumanHandoff:    "Продолжит человек: сотрудник примет обращение и доведёт его до результата.",
}

// AlternativeLimitationText resolves the honest limitation text of one
// alternative word (the empty string of an unknown word — the render layer
// of the panels only; the dictionary words travel, the texts render).
func AlternativeLimitationText(word string) string {
	return AlternativeLimitationTexts[word]
}

// ---------------------------------------------------------------------------
// The usual-modality map of the mandatory material keys (the pack data v0)
// ---------------------------------------------------------------------------

// The usual modality words: how a mandatory material key is usually
// established in the anchor world — by media (a photo) or by text.
const (
	UsualModalityMediaPhoto = "media_photo"
	UsualModalityText       = "text"
)

// UsualModalityWords is the full dictionary.
var UsualModalityWords = []string{UsualModalityMediaPhoto, UsualModalityText}

// usualModalityOfMaterialKey is the pack data map of the mandatory material
// keys: service_kind is usually established by a photo in the anchor world
// of the auto parts; urgency and slot_preference are usually text; an
// unknown key fails OPEN to text (the map is pack data, never a gate).
var usualModalityOfMaterialKey = map[string]string{
	"service_kind":    UsualModalityMediaPhoto,
	"urgency":         UsualModalityText,
	"slot_preference": UsualModalityText,
}

// UsualModalityOfMaterialKey resolves the usual modality word of a
// mandatory material key (the unknown key maps to text — the honest
// fail-open of the pack data).
func UsualModalityOfMaterialKey(materialKey string) string {
	if modality, ok := usualModalityOfMaterialKey[materialKey]; ok {
		return modality
	}
	return UsualModalityText
}

// alternativesOfUsualModality is the pack data map of the alternative
// words per usual modality: the media keys carry the full alternative
// dictionary of the pack; the text keys carry none (the text path IS the
// usual path — no alternative attaches).
var alternativesOfUsualModality = map[string][]string{
	UsualModalityMediaPhoto: AlternativePathWords,
}

// AlternativesOfUsualModality resolves the permitted alternative words of
// one usual modality (the copy of the pack list — the caller may hold it).
func AlternativesOfUsualModality(modality string) []string {
	if words, ok := alternativesOfUsualModality[modality]; ok {
		return append([]string(nil), words...)
	}
	return nil
}

// ---------------------------------------------------------------------------
// The fixed material bounds (the domain guard carries them, the table
// CHECK mirrors them)
// ---------------------------------------------------------------------------

// The accommodation material limits (runes): the operator note of every
// record and the free availability line of the available_time key.
const (
	MaxAccommodationNoteRunes = 500
	MaxAvailableTimeRunes     = 100
)

// ---------------------------------------------------------------------------
// The row shape (the store reads/writes; the current set is derived, never
// stored)
// ---------------------------------------------------------------------------

// AccommodationRecord is one append-only revision row of one preference
// key of one case: the key word, the monotonic revision number, the value
// of the valued keys (empty for the flags), the clearing mark, the operator
// note and the actor. The history is never rewritten — a new value or a
// clearing is a NEW revision.
type AccommodationRecord struct {
	TenantID   id.UUID
	CaseID     id.UUID
	RecordID   id.UUID
	Key        string
	Revision   int64
	Value      string
	Cleared    bool
	Note       string
	ActorID    id.UUID
	RecordedAt time.Time
}

// ---------------------------------------------------------------------------
// The read-time derivation (the current set, never stored)
// ---------------------------------------------------------------------------

// AccommodationState is the read-time state of one preference key: the
// latest revision number, the clearing mark of that revision and the valued
// material. A CLEARED state stays in the map (the honest revision truth of
// the snapshot — a clearing changes the digest); the ACTIVE set is the
// keys whose latest revision is not a clearing one.
type AccommodationState struct {
	Key      string
	Revision int64
	Cleared  bool
	Value    string
}

// CurrentAccommodations is the derived preferences state of one case: every
// key with at least one recorded revision carries the state of its LATEST
// revision (the read-time truth of the moment — never a stored status).
type CurrentAccommodations map[string]AccommodationState

// DeriveCurrentAccommodations derives the read-time preferences state: the
// latest revision of every key wins (the append-only history is never
// rewritten); a clearing revision keeps the key in the state with the
// cleared mark and removes it from the active set. The LATER record of an
// equal revision wins the defensive tie (the table UNIQUE backstop makes
// the case unreachable — the derivation stays deterministic on any input
// order).
func DeriveCurrentAccommodations(records []AccommodationRecord) CurrentAccommodations {
	current := CurrentAccommodations{}
	for _, record := range records {
		state, known := current[record.Key]
		if !known || record.Revision >= state.Revision {
			current[record.Key] = AccommodationState{
				Key: record.Key, Revision: record.Revision,
				Cleared: record.Cleared, Value: record.Value,
			}
		}
	}
	return current
}

// HasActive reports whether the key is in the CURRENT active set: the
// latest revision exists and is not a clearing one.
func (c CurrentAccommodations) HasActive(key string) bool {
	state, known := c[key]
	return known && !state.Cleared
}

// ActiveKeys resolves the sorted active keys of the current set (the
// canonical panel order).
func (c CurrentAccommodations) ActiveKeys() []string {
	keys := make([]string, 0, len(c))
	for key, state := range c {
		if !state.Cleared {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// AccommodationSnapshotMap is the canonical preferences state of the
// policy input snapshot: the key → {revision, cleared} map (the encoding
// order of the map marshal is canonical — the FactsDigestMap precedent).
type AccommodationSnapshotMap map[string]struct {
	Revision int64 `json:"revision"`
	Cleared  bool  `json:"cleared"`
}

// AccommodationSnapshotOf renders the canonical preferences map of the
// policy input snapshot. The map is ALWAYS initialized: the same logical
// state renders the same canonical JSON (the empty state is {} — the
// digest determinism; the clearing revisions stay in the map — the honest
// supersede of the changed preferences).
func AccommodationSnapshotOf(c CurrentAccommodations) AccommodationSnapshotMap {
	snapshot := AccommodationSnapshotMap{}
	for key, state := range c {
		snapshot[key] = struct {
			Revision int64 `json:"revision"`
			Cleared  bool  `json:"cleared"`
		}{Revision: state.Revision, Cleared: state.Cleared}
	}
	return snapshot
}

// ---------------------------------------------------------------------------
// The record guard (the domain truth before SQL)
// ---------------------------------------------------------------------------

// GuardAccommodationRecord validates the material of one revision record:
// the key belongs to the fixed dictionary; a flag key carries NO value; a
// valued key carries a value; the preferred channel value is a word of the
// 2.1 channel dictionary; the availability line respects its rune bound;
// the note respects its rune bound; a clearing revision never carries a
// value (first the value, the clearing is its own revision).
func GuardAccommodationRecord(key, value string, cleared bool, note string) error {
	if !AccommodationKeyInDictionary(key) {
		return fmt.Errorf("ключ предпочтения «%s» не входит в словарь (%s)", key, joinQuoted(AccommodationKeys))
	}
	if AccommodationKeyIsFlag(key) && value != "" {
		return fmt.Errorf("флаговое предпочтение «%s» не несёт значение: оставьте поле пустым", key)
	}
	if key == AccommodationPreferredChannel {
		if value == "" {
			return fmt.Errorf("предпочтение preferred_channel требует значение — слово словаря каналов (%s)", strings.Join(ChannelOrder, ", "))
		}
		if !inDictionary(ChannelOrder, value) {
			return fmt.Errorf("значение preferred_channel «%s» не входит в словарь каналов (%s)", value, strings.Join(ChannelOrder, ", "))
		}
	}
	if key == AccommodationAvailableTime {
		runes := utf8.RuneCountInString(value)
		if runes < 1 || runes > MaxAvailableTimeRunes {
			return fmt.Errorf("строка доступности preferred-времени обязана быть 1..%d рун, получено %d", MaxAvailableTimeRunes, runes)
		}
	}
	if cleared && value != "" {
		return fmt.Errorf("нельзя одновременно записать значение и снять предпочтение «%s»: значение — отдельной ревизией, снятие — своей", key)
	}
	if utf8.RuneCountInString(note) > MaxAccommodationNoteRunes {
		return fmt.Errorf("заметка предпочтения длиннее %d рун", MaxAccommodationNoteRunes)
	}
	return nil
}

// ---------------------------------------------------------------------------
// The accuracy qualifier gate (the sales reply honesty of AC-2)
// ---------------------------------------------------------------------------

// AccuracyQualifierRequiredText is the fixed refusal text of the sales
// reply without the accuracy qualifier against an outcome with active
// alternatives: the honest limitation is mandatory, never optional.
const AccuracyQualifierRequiredText = "Действующий исход несёт активные альтернативы: продажный ответ обязан заявить ограничение точности (accuracy_qualifier) одним из слов активного множества исхода."

// AccuracyQualifierForeignText is the fixed refusal text of the sales
// reply carrying a qualifier without any active alternatives: the field
// has exactly one meaning (the objection_step_foreign mirror of 2.8).
const AccuracyQualifierForeignText = "Слово ограничения точности принадлежит продажному ответу при активных альтернативах исхода: альтернатив в действующем исходе нет — оставьте поле пустым."

// AccuracyQualifierOutsideActiveText is the fixed refusal text of the
// qualifier outside the active set of the outcome: the subset of the
// OUTCOME, never the dictionary in general (the outcome decides which
// words are honest in this context).
const AccuracyQualifierOutsideActiveText = "Слово ограничения точности не входит в активные альтернативы действующего исхода: выберите слово активного множества — подмножество исхода, не словарь вообще."

// GuardSalesReplyAccuracy is the sales-reply gate of Story 2.11 (AFTER the
// 2.8 objection gate, BEFORE the CAS — the frozen ladder: ownership 2.4 →
// mode 2.5 → disclosure 2.5 → policy 2.6 → objection 2.8 → accuracy 2.11 →
// write; the SALES class only): the CURRENT outcome carries active
// alternatives → the qualifier is REQUIRED and must be one of the outcome's
// active words; no active alternatives → the field must stay EMPTY (the
// byte-identical old path). The gate mirrors GuardSalesReplyAgainstObjections
// by the form and is stricter by the dictionary: the permitted words are
// the subset of the outcome, never the dictionary in general.
func GuardSalesReplyAccuracy(activeAlternatives []string, qualifier string) error {
	if len(activeAlternatives) == 0 {
		if qualifier != "" {
			return WorkRuleError{Rule: "accuracy_qualifier_foreign", Reason: AccuracyQualifierForeignText}
		}
		return nil
	}
	if qualifier == "" {
		return WorkRuleError{Rule: "accuracy_qualifier_required", Reason: AccuracyQualifierRequiredText + " Активное множество: " + joinQuoted(activeAlternatives) + "."}
	}
	if !inDictionary(activeAlternatives, qualifier) {
		return WorkRuleError{Rule: "accuracy_qualifier_foreign", Reason: AccuracyQualifierOutsideActiveText + " Активное множество: " + joinQuoted(activeAlternatives) + "."}
	}
	return nil
}

// ---------------------------------------------------------------------------
// The outbox payload (the safe canonical JSON — the value and the note
// never leave the row)
// ---------------------------------------------------------------------------

// Outbox event types of the accommodation surface (the 2.1 bounded
// vocabulary).
const (
	OutboxEventAccommodationRecorded   = "case.accommodation_recorded"
	OutboxEventAccommodationRecordedV1 = 1
)

// AccommodationRecordedPayload is the safe canonical JSON of the record
// event: the identifiers, the key word, the revision number and the
// clearing mark — the VALUE (the channel word, the availability line) and
// the NOTE never enter the envelope (the operational rows stay in the
// table; the «кандидат не покидает строку» precedent of 2.6).
type AccommodationRecordedPayload struct {
	TenantID   id.UUID   `json:"tenant_id"`
	CaseID     id.UUID   `json:"case_id"`
	RecordID   id.UUID   `json:"record_id"`
	Key        string    `json:"key"`
	Revision   int64     `json:"revision"`
	Cleared    bool      `json:"cleared"`
	ActorID    id.UUID   `json:"actor_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// DedupKeyAccommodationRecorded builds the dedup key of one record event.
func DedupKeyAccommodationRecorded(recordID id.UUID) string {
	return "case.accommodation_recorded:" + recordID.String()
}

// AccommodationRecordedPayloadJSON renders the safe canonical JSON.
func AccommodationRecordedPayloadJSON(payload AccommodationRecordedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
