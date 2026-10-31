package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 4.3: the part request domain (the number normalization with
// flags, the deterministic mode ladder over the pack dictionary and the
// read-time part request derivation with the completeness, the digest,
// the confidence word and the three honesty boundaries)
// ---------------------------------------------------------------------------
//
// The honesty core of the slice: the mode is a DETERMINISTIC derivation
// over the recorded material — the priority ladder of the pinned pack
// over the role keys, symptom dominates ALWAYS (a probabilistic guess
// never becomes a catalog request); the named part under a symptom is a
// CANDIDATE with the «причина не доказана» word. Nothing is silently
// fixed: NormalizePartNumber is flags only (the raw value stays verbatim,
// the homoglyphs are detected without replacement); MissingKeys covers
// ONLY the not-confirmed keys of the SELECTED mode (the confirmed
// material is never re-asked); an absent optional key is an honest
// absence, never a guess. The exact number match is NOT a fitment proof
// (the constant word — the catalog contour is 4.4). The part request is
// a READ-TIME derivation over the append-only facts of 2.2: no new
// table, byte-stable idempotent JSON, zero writes.

// ---------------------------------------------------------------------------
// The role and mode dictionaries (the pack material, mirrored by the
// narrow port — the domain never imports the port)
// ---------------------------------------------------------------------------

// The request material roles of the pack schema (the ladder steps).
const (
	PartRolePartKind   = "part_kind"
	PartRoleSymptom    = "symptom"
	PartRoleOemNumber  = "oem_number"
	PartRolePreference = "preference"
)

// The mode words of the auto-parts@4.0.0 dictionary (the enum mirror of
// the port answer — the words are pinned by the pack, the domain renders
// the honest words of the selected one).
const (
	PartModeDiagnostic       = "diagnostic"
	PartModeExactRepeat      = "exact_repeat"
	PartModeOriginalVsAnalog = "original_vs_analog"
	PartModeNamedPart        = "named_part"
)

// The handoff recommendation words of the boundaries — the Story 4.3
// reason dictionary extension of 2.13: the constants live in
// human_handoff.go (the dictionary home — HandoffReasonDiagnosticUncertainty
// / HandoffReasonSafetyCritical / HandoffReasonUnresolvedContradiction);
// the SQL mirror 00019 admits them from this spec on (the write path
// guard: GuardHandoffRequestReason, fail-closed on the dictionary).

// PartNumberMatchIsNotFitmentText is the constant word of the exact
// number match (AC-4 / FR-52: the match is not a fitment proof — the
// catalog proof contour is 4.4).
const PartNumberMatchIsNotFitmentText = "точное совпадение номера не считается доказательством применимости к подтверждённому автомобилю"

// PartRequestUnstructuredWord is the honest word of the empty material:
// the request is not structured, the mode is honestly absent (never a
// guess — the operator records the material by the pack dictionary).
const PartRequestUnstructuredWord = "запрос не структурирован — запишите материал детали по словарю схемы"

// PartRequestDiagnosticBoundaryText is the diagnostic boundary word
// (AC-7: the symptom never silently becomes a catalog request — the
// selection is stopped with the honest word, the diagnosis belongs to
// the specialist).
const PartRequestDiagnosticBoundaryText = "симптом не является названной деталью: подбор остановлен — диагноз у специалиста; соберите разрешённые наблюдения и передайте человеку"

// PartRequestCandidateUnderSymptomText is the candidate word of the
// named part under a symptom (AC-7: the cause is not proven).
const PartRequestCandidateUnderSymptomText = "названная деталь — КАНДИДАТ при симптоме: причина неисправности не доказана"

// PartRequestSafetyNoteText is the safety note of the matched marker
// (AC-8: the confirmed material is mandatory; the manager gate of 4.7
// stands before the supplier — the note, not the stop: the stop happens
// only under the diagnostic uncertainty or a contradiction).
const PartRequestSafetyNoteText = "safety-узел: подтверждённый материал обязателен; менеджерский гейт 4-7 стоит до поставщика"

// PartRequestSafetyStopText is the stop word of the safety boundary
// under the diagnostic uncertainty (AC-8: the automatic selection stops).
const PartRequestSafetyStopText = "автоматический подбор остановлен: safety-узел с диагностической неопределённостью"

// PartRequestSafetyHeuristicText is the honest word of the marker list:
// the declared list is the pack heuristic, never a proof.
const PartRequestSafetyHeuristicText = "список safety-узлов — эвристика пака; при сомнении передайте специалисту"

// PartRequestContradictionBoundaryText is the contradiction boundary
// word (AC-9: Staffy never chooses the most probable variant).
const PartRequestContradictionBoundaryText = "Staffy не выбирает наиболее вероятный вариант: спорные факты передаются человеку с исходным запросом, подтверждённым автомобилем и выполненными проверками"

// PartRequestSeveralPartsText is the honest word of the several active
// «поиск детали» items: ONE structured set per case (the fact key space
// of 2.2 — the material key is unique per case); several parts are the
// existing split + the honest word (the per-item evolution is the
// deferred work, never a debt of this slice).
const PartRequestSeveralPartsText = "обращение содержит несколько деталей: материал ведёт один структурированный набор на обращение — разделяйте элементы работы; одновременные структурированные наборы вне первого среза"

// PartRequestStaleNote is the honest aggregate stale word of the request
// (AC-10: the dependent contours — the search, the fitment, the
// comparison, the commitment of 4.4..4.9 — must pin the request digest
// and treat the former results as stale).
const PartRequestStaleNote = "Часть подтверждений относится к прежним версиям фактов: зависимые результаты (поиск, применимость, сравнение, commitment — контуры 4-4..4-9) обязаны пиннить digest запроса и считать прежние устаревшими; дальнейшая работа — только новая подтверждённая версия."

// PartRequestPreferenceBothValue is the preference value that selects
// the original_vs_analog mode (the client asks for the CHOICE — the
// other values are constraints, never a choice request).
const PartRequestPreferenceBothValue = "both"

// ---------------------------------------------------------------------------
// The part number normalization (flags, never silent fixes — AC-4)
// ---------------------------------------------------------------------------

// partCyrillicLookAlike maps the cyrillic homoglyph runes to the latin
// letters they are confused with (the VIN discipline of 4.2 — the same
// boundary, the part numbers carry the same homoglyph ambiguity).
var partCyrillicLookAlike = map[rune]rune{
	'А': 'A', 'В': 'B', 'Е': 'E', 'К': 'K', 'М': 'M', 'Н': 'H',
	'О': 'O', 'Р': 'P', 'С': 'C', 'Т': 'T', 'У': 'Y', 'Х': 'X',
}

// isCyrillicPartRune answers whether the rune belongs to the cyrillic
// unicode block (U+0400..U+04FF) — the homoglyph detection boundary.
func isCyrillicPartRune(r rune) bool {
	return r >= 0x0400 && r <= 0x04FF
}

// PartNumberAmbiguity is one cyrillic homoglyph position: the rune, its
// position in the normalized rendering and the latin look-alike. The
// detection NEVER replaces the symbol — the re-ask is the only honest
// resolution (the VIN discipline of 4.2).
type PartNumberAmbiguity struct {
	Position  int    `json:"position"`
	Rune      string `json:"rune"`
	LookAlike string `json:"look_alike"`
}

// PartNumberNormalization is the full flag answer of
// NormalizePartNumber: the raw value (ALWAYS preserved verbatim), the
// normalized rendering (the trim + the upper case + the spaces dropped +
// the «_»/«.» unified into «-» — nothing else), the alphabet flag with
// the out-of-alphabet positions and the homoglyph ambiguities with the
// honest words of every flag. The normalization is idempotent.
type PartNumberNormalization struct {
	Raw                 string                `json:"raw"`
	Normalized          string                `json:"normalized"`
	AlphabetOK          bool                  `json:"alphabet_ok"`
	BadChars            []int                 `json:"bad_chars"`
	CyrillicAmbiguities []PartNumberAmbiguity `json:"cyrillic_ambiguities"`
	Words               []string              `json:"words"`
}

// NormalizePartNumber derives the honest flag answer of one raw part
// number value. The raw value is preserved verbatim (the fact material
// of 2.2); the normalized rendering only trims, upper-cases, drops the
// spaces and unifies the «_»/«.» separators into «-»; EVERY check is a
// flag with an honest word — no symbol is ever replaced, no value is
// ever discarded. The allowed alphabet: the latin letters, the digits,
// «-», «_», «.» and the spaces (the separators of the raw form).
func NormalizePartNumber(raw string) PartNumberNormalization {
	answer := PartNumberNormalization{Raw: raw, BadChars: []int{}, CyrillicAmbiguities: []PartNumberAmbiguity{}, Words: []string{}}
	trimmed := strings.ToUpper(strings.TrimSpace(raw))
	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case r == ' ' || r == '\t':
			continue
		case r == '_' || r == '.':
			builder.WriteRune('-')
		default:
			builder.WriteRune(r)
		}
	}
	answer.Normalized = builder.String()
	answer.AlphabetOK = true
	for index, r := range answer.Normalized {
		switch {
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-':
			// the alphabet members carry no flag
		case isCyrillicPartRune(r):
			if lookAlike, ok := partCyrillicLookAlike[r]; ok {
				answer.CyrillicAmbiguities = append(answer.CyrillicAmbiguities, PartNumberAmbiguity{
					Position: index, Rune: string(r), LookAlike: string(lookAlike),
				})
			} else {
				answer.AlphabetOK = false
				answer.BadChars = append(answer.BadChars, index)
			}
		default:
			answer.AlphabetOK = false
			answer.BadChars = append(answer.BadChars, index)
		}
	}
	answer.Words = partNumberHonestWords(answer)
	return answer
}

// partNumberHonestWords renders the honest words of every flag (the
// flags are text, never only color).
func partNumberHonestWords(normalization PartNumberNormalization) []string {
	words := []string{}
	for _, ambiguity := range normalization.CyrillicAmbiguities {
		words = append(words, fmt.Sprintf("позиция %d: кириллица %s (похожа на латиницу %s) — сверьте номер посимвольно", ambiguity.Position+1, ambiguity.Rune, ambiguity.LookAlike))
	}
	if len(normalization.BadChars) > 0 {
		positions := make([]string, 0, len(normalization.BadChars))
		for _, position := range normalization.BadChars {
			positions = append(positions, fmt.Sprintf("%d", position+1))
		}
		words = append(words, fmt.Sprintf("символы вне разрешённого алфавита (буквы, цифры, дефис) — сверьте позиции %s", strings.Join(positions, ", ")))
	}
	return words
}

// ---------------------------------------------------------------------------
// The deterministic mode ladder (AC-1 — the pack dictionary over the
// role keys, symptom dominates always)
// ---------------------------------------------------------------------------

// PartMaterialKeyDomain is the domain form of one request material key
// of the pack schema (the app maps the narrow port answer onto it — the
// domain never imports the port).
type PartMaterialKeyDomain struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Kind     string   `json:"kind"` // text | integer | enum
	Values   []string `json:"values,omitempty"`
	Min      int      `json:"min,omitempty"`
	Max      int      `json:"max,omitempty"`
	Optional bool     `json:"optional,omitempty"`
	Role     string   `json:"role,omitempty"`
}

// PartModeSpecDomain is the domain form of one request mode of the pack
// dictionary.
type PartModeSpecDomain struct {
	Mode     string   `json:"mode"`
	Label    string   `json:"label"`
	Priority int      `json:"priority"`
	Requires []string `json:"requires"`
}

// PartSchemaDomain is the request schema of the derivation input.
type PartSchemaDomain struct {
	MaterialKeys  []PartMaterialKeyDomain `json:"material_keys"`
	Modes         []PartModeSpecDomain    `json:"modes"`
	SafetyMarkers []string                `json:"safety_markers"`
}

// PartModeAnswer is the mode derivation answer: the selected mode word
// with the pack label and the transparent basis word (the honest
// «почему этот режим» — the ladder step that selected it), or the honest
// unstructured word when no material exists.
type PartModeAnswer struct {
	Mode             string `json:"mode"`
	Label            string `json:"label"`
	BasisWord        string `json:"basis_word"`
	UnstructuredWord string `json:"unstructured_word,omitempty"`
}

// partFactMaterial is the material presence answer of one key: whether
// the fact exists with at least one version and its latest raw text.
type partFactMaterial struct {
	Present bool
	Text    string
}

// partMaterialOf resolves the material presence of one key over the
// facts by key (the latest version raw text — the fact material of 2.2).
func partMaterialOf(facts map[string]VehicleFactInput, key string) partFactMaterial {
	fact, present := facts[key]
	if !present || len(fact.Versions) == 0 {
		return partFactMaterial{}
	}
	return partFactMaterial{Present: true, Text: vehicleRawValueText(fact.Versions[len(fact.Versions)-1].Value)}
}

// DerivePartRequestMode derives the request mode over the recorded
// material: the DETERMINISTIC ladder of the pack dictionary — the modes
// are walked by the declared priority; a mode is applicable when every
// ROLE key of its Requires carries material (the preference role
// additionally requires the «both» value — a choice request, the other
// values are constraints); the FIRST applicable mode wins (symptom
// dominates always — the lowest priority number). No material — the
// honest unstructured word, never a guess.
func DerivePartRequestMode(schema PartSchemaDomain, facts map[string]VehicleFactInput) PartModeAnswer {
	modes := append([]PartModeSpecDomain(nil), schema.Modes...)
	sort.SliceStable(modes, func(i, j int) bool { return modes[i].Priority < modes[j].Priority })
	for _, mode := range modes {
		applicable, basis := partModeApplicable(mode, schema, facts)
		if applicable {
			return PartModeAnswer{Mode: mode.Mode, Label: mode.Label, BasisWord: basis}
		}
	}
	return PartModeAnswer{UnstructuredWord: PartRequestUnstructuredWord}
}

// partModeApplicable answers whether one mode is applicable over the
// material and renders its basis word. The role keys of the Requires set
// drive the ladder: the symptom material selects the diagnostic step,
// the oem_number material the exact repeat step, the part_kind material
// the named-part family; the preference value «both» is the CHOICE
// request (original_vs_analog), the other values are constraints. The
// role-less keys of the Requires set (the quantity) are the completeness
// material only — they never select the mode.
func partModeApplicable(mode PartModeSpecDomain, schema PartSchemaDomain, facts map[string]VehicleFactInput) (bool, string) {
	roleEvidence := []string{}
	for _, requiredKey := range mode.Requires {
		spec := partKeySpecOf(schema, requiredKey)
		if spec == nil || spec.Role == "" {
			continue
		}
		material := partMaterialOf(facts, requiredKey)
		switch spec.Role {
		case PartRoleSymptom:
			if !material.Present {
				return false, ""
			}
			roleEvidence = append(roleEvidence, "записан симптом")
		case PartRoleOemNumber:
			if !material.Present {
				return false, ""
			}
			roleEvidence = append(roleEvidence, "записан номер детали")
		case PartRolePartKind:
			if !material.Present {
				return false, ""
			}
			roleEvidence = append(roleEvidence, "названа деталь")
		case PartRolePreference:
			if !material.Present || material.Text != PartRequestPreferenceBothValue {
				return false, ""
			}
			roleEvidence = append(roleEvidence, "клиент просит выбор (значение both)")
		}
	}
	if len(roleEvidence) == 0 {
		return false, ""
	}
	return true, "основание: " + strings.Join(roleEvidence, "; ")
}

// partKeySpecOf resolves one material key spec of the schema.
func partKeySpecOf(schema PartSchemaDomain, key string) *PartMaterialKeyDomain {
	for index := range schema.MaterialKeys {
		if schema.MaterialKeys[index].Key == key {
			return &schema.MaterialKeys[index]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// The read-time part request derivation (AC-2/AC-3/AC-5/AC-10)
// ---------------------------------------------------------------------------

// PartRequestFactInput is one fact of the case handed to the derivation
// (the VehicleFactInput mapping — the same append-only versions and the
// confirmations of 2.2).
type PartRequestFactInput = VehicleFactInput

// PartConfirmationView is the confirmation rendering of one attribute
// (the vehicle form — the exact version binding with the stale word).
type PartConfirmationView = VehicleConfirmationView

// PartAttributeView is one attribute of the request: the material key
// with the pack label and the role, the raw value, the number
// normalization (only the oem_number role — the derivation, never a
// stored rendering), the fact version with the provenance, the
// read-time 2.2 status and the confirmation view when one exists.
type PartAttributeView struct {
	FactID       id.UUID                  `json:"fact_id"`
	MaterialKey  string                   `json:"material_key"`
	Label        string                   `json:"label"`
	Role         string                   `json:"role"`
	Kind         string                   `json:"kind"`
	RawValue     string                   `json:"raw_value"`
	NumberChecks *PartNumberNormalization `json:"number_checks,omitempty"`
	FactVersion  int64                    `json:"fact_version"`
	ObservedAt   time.Time                `json:"observed_at"`
	ReceivedAt   time.Time                `json:"received_at"`
	SourceType   string                   `json:"source_type"`
	Channel      string                   `json:"channel"`
	Status       string                   `json:"status"`
	Confirmation *PartConfirmationView    `json:"confirmation,omitempty"`
}

// PartCompleteness is the honest completeness answer: the completeness
// word and the missing keys — ONLY the not-confirmed keys of the
// SELECTED mode's Requires set (the confirmed material is never
// re-asked, AC-3; the absent optional keys are honest absences, never
// missing).
type PartCompleteness struct {
	CompleteWord string   `json:"complete_word"`
	MissingKeys  []string `json:"missing_keys"`
}

// PartBoundaryView is one honesty boundary of the derivation: the
// boundary kind word (diagnostic | safety | contradiction) and the text
// (role=alert material — the uncertainty is text, never only color).
type PartBoundaryView struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// PartRequestView is the read-time part request: the selected mode with
// the basis word, the attributes in the schema dictionary order, the
// completeness answer with the minimal clarify questions, the
// PartRequestDigest (the SHA-256 of the canonical confirmed set — «the
// exact version of the request», AC-10), the confidence word (the
// confirmedness conclusion, never an invented number), the honesty
// boundaries, the handoff recommendation words (the 2.13 dictionary
// extension of 4.3) and the several-parts word when the case carries
// more than one active «поиск детали» item.
type PartRequestView struct {
	Mode             string                  `json:"mode"`
	ModeLabel        string                  `json:"mode_label"`
	ModeBasisWord    string                  `json:"mode_basis_word"`
	UnstructuredWord string                  `json:"unstructured_word,omitempty"`
	SchemaDictionary []PartMaterialKeyDomain `json:"schema_dictionary,omitempty"`
	Attributes       []PartAttributeView     `json:"attributes"`
	// NumberMatchWord carries the constant word of the exact number
	// match (AC-4 / FR-52: the match is not a fitment proof — the
	// catalog contour is 4.4) whenever the oem_number material exists.
	NumberMatchWord   string             `json:"number_match_word,omitempty"`
	Completeness      PartCompleteness   `json:"completeness"`
	Questions         map[string]string  `json:"questions"`
	PartRequestDigest string             `json:"part_request_digest"`
	ConfidenceWord    string             `json:"confidence_word"`
	Boundaries        []PartBoundaryView `json:"boundaries"`
	HandoffWords      []string           `json:"handoff_words"`
	SeveralPartsWord  string             `json:"several_parts_word,omitempty"`
	StaleNote         string             `json:"stale_note,omitempty"`
}

// DerivePartRequest derives the part request of one case over the
// append-only facts (READ-TIME: zero writes, idempotent — the JSON of
// the same world is byte-stable). The statuses are the 2.2 read-time
// derivations (DeriveFactStatus — the frozen canon); a new version of
// an attribute after a confirmation makes the attribute conflicting with
// the stale word — the PartRequestDigest changes, the dependent contours
// must treat the request as stale (AC-10). Facts outside the schema
// dictionary are ignored (the schema is the pack material — never a
// hardcoded core list). The activePartSearches is the count of the
// active «поиск детали» work items of the case (the app input — the
// SeveralPartsWord is the honest split recommendation of 2.2).
func DerivePartRequest(schema PartSchemaDomain, facts []PartRequestFactInput, activePartSearches int) PartRequestView {
	view := PartRequestView{
		Attributes:   []PartAttributeView{},
		Questions:    map[string]string{},
		Boundaries:   []PartBoundaryView{},
		HandoffWords: []string{},
	}
	byKey := map[string]VehicleFactInput{}
	for _, fact := range facts {
		byKey[fact.MaterialKey] = fact
	}
	modeAnswer := DerivePartRequestMode(schema, byKey)
	view.Mode = modeAnswer.Mode
	view.ModeLabel = modeAnswer.Label
	view.ModeBasisWord = modeAnswer.BasisWord
	view.UnstructuredWord = modeAnswer.UnstructuredWord
	if view.UnstructuredWord != "" {
		// The honest empty dictionary hint: the schema keys with the
		// labels (the operator records the material by the dictionary).
		view.SchemaDictionary = append([]PartMaterialKeyDomain(nil), schema.MaterialKeys...)
	}
	confirmedSet := map[string]struct {
		VersionNo   int64
		ValueDigest string
	}{}
	stale := false
	conflicting := false
	confirmedCount := 0
	requiresTotal := 0
	confirmedOfRequires := 0
	for _, key := range schema.MaterialKeys {
		fact, present := byKey[key.Key]
		if !present || len(fact.Versions) == 0 {
			continue
		}
		latest := fact.Versions[len(fact.Versions)-1]
		status, _ := DeriveFactStatus(fact.Versions, fact.Confirmations)
		attribute := PartAttributeView{
			FactID: fact.FactID, MaterialKey: key.Key, Label: key.Label, Role: key.Role, Kind: key.Kind,
			RawValue: vehicleRawValueText(latest.Value), FactVersion: latest.VersionNo,
			ObservedAt: latest.ObservedAt, ReceivedAt: latest.ReceivedAt,
			SourceType: latest.SourceType, Channel: latest.Channel, Status: status,
		}
		if key.Role == PartRoleOemNumber {
			checks := NormalizePartNumber(attribute.RawValue)
			attribute.NumberChecks = &checks
			view.NumberMatchWord = PartNumberMatchIsNotFitmentText
		}
		var latestConfirmation *FactConfirmation
		for index := range fact.Confirmations {
			confirmation := &fact.Confirmations[index]
			if latestConfirmation == nil || confirmation.VersionNo > latestConfirmation.VersionNo {
				latestConfirmation = confirmation
			}
		}
		if latestConfirmation != nil {
			confirmationView := &PartConfirmationView{
				ConfirmerType: latestConfirmation.ConfirmerType,
				VersionNo:     latestConfirmation.VersionNo,
				At:            latestConfirmation.CreatedAt,
			}
			if latestConfirmation.VersionNo < latest.VersionNo {
				confirmationView.StaleWord = fmt.Sprintf("подтверждение относилось к прежней версии (v%d)", latestConfirmation.VersionNo)
				stale = true
			}
			attribute.Confirmation = confirmationView
		}
		if status == FactStatusConfirmed {
			confirmedSet[key.Key] = struct {
				VersionNo   int64
				ValueDigest string
			}{VersionNo: latest.VersionNo, ValueDigest: latest.ValueDigest}
			confirmedCount++
		}
		if status == FactStatusConflicting {
			conflicting = true
		}
		view.Attributes = append(view.Attributes, attribute)
	}
	// The completeness over the SELECTED mode's Requires set: the missing
	// keys are ONLY the not-confirmed ones (AC-3 — the confirmed material
	// is never re-asked); the absent optional keys never appear.
	if view.Mode != "" {
		for _, mode := range schema.Modes {
			if mode.Mode != view.Mode {
				continue
			}
			for _, requiredKey := range mode.Requires {
				requiresTotal++
				if _, confirmed := confirmedSet[requiredKey]; confirmed {
					confirmedOfRequires++
					continue
				}
				view.Completeness.MissingKeys = append(view.Completeness.MissingKeys, requiredKey)
				view.Questions[requiredKey] = partQuestionWord(schema, requiredKey)
			}
		}
		if len(view.Completeness.MissingKeys) == 0 {
			view.Completeness.CompleteWord = "материал запроса полон по обязательному набору выбранного режима"
		} else {
			view.Completeness.CompleteWord = "материал запроса неполон по обязательному набору выбранного режима — задайте минимальные уточняющие вопросы"
		}
	}
	view.PartRequestDigest = partRequestDigest(confirmedSet)
	view.ConfidenceWord = partConfidenceWord(conflicting, confirmedOfRequires, requiresTotal, confirmedCount)
	partBoundaries(schema, view.Mode, byKey, conflicting, &view)
	if activePartSearches > 1 {
		view.SeveralPartsWord = PartRequestSeveralPartsText
	}
	if stale {
		view.StaleNote = PartRequestStaleNote
	}
	return view
}

// partRequestDigest renders the SHA-256 of the canonical JSON of the
// confirmed set — the map {key: {version_no, value_digest}} with the
// sorted keys (the byte-stable canonical form of «the exact version of
// the request», AC-10; an empty set digests the honest empty object).
func partRequestDigest(confirmedSet map[string]struct {
	VersionNo   int64
	ValueDigest string
}) string {
	keys := make([]string, 0, len(confirmedSet))
	for key := range confirmedSet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	canonical := make(map[string]map[string]any, len(keys))
	for _, key := range keys {
		entry := confirmedSet[key]
		canonical[key] = map[string]any{"version_no": entry.VersionNo, "value_digest": entry.ValueDigest}
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		// The shape is fixed (strings, int64, strings) — the marshal
		// cannot fail; the honest fallback keeps the derivation total.
		encoded = []byte("{}")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// partConfidenceWord renders the honest confidence word — the
// confirmedness conclusion over the SELECTED mode's mandatory set, never
// an invented number (AC-2).
func partConfidenceWord(conflicting bool, confirmedOfRequires, requiresTotal, confirmedCount int) string {
	if conflicting {
		return "уверенность не выводится: материал содержит противоречия — Staffy не выбирает вариант"
	}
	if requiresTotal > 0 && confirmedOfRequires == requiresTotal {
		return "материал подтверждён"
	}
	if confirmedOfRequires > 0 {
		return fmt.Sprintf("материал подтверждён частично: подтверждены %d из %d обязательных ключей выбранного режима", confirmedOfRequires, requiresTotal)
	}
	if confirmedCount > 0 {
		return "материал подтверждён частично: обязательные ключи выбранного режима не подтверждены"
	}
	return "материал без подтверждения"
}

// partQuestionWord renders the minimal clarify question of one missing
// key (AC-3: the question is asked ONCE — the confirmed keys are never
// re-asked). The question form follows the ROLE and the kind of the
// schema key (never a hardcoded key literal — the dictionary comes from
// the pack); the role-less keys degrade to the label question.
func partQuestionWord(schema PartSchemaDomain, key string) string {
	spec := partKeySpecOf(schema, key)
	if spec == nil {
		return "уточните значение"
	}
	switch spec.Role {
	case PartRolePartKind:
		return "уточните: какая деталь нужна?"
	case PartRoleSymptom:
		return "уточните и подтвердите симптом: что именно беспокоит клиента?"
	case PartRoleOemNumber:
		return "уточните: каталожный номер детали?"
	case PartRolePreference:
		return "уточните: оригинал, аналог или выбор клиента?"
	}
	if spec.Kind == "integer" {
		return "уточните: сколько нужно?"
	}
	return fmt.Sprintf("уточните: %s?", strings.ToLower(spec.Label))
}

// partBoundaries renders the three honesty boundaries of the request:
// the diagnostic boundary (the symptom never silently becomes a catalog
// request; the named part under a symptom is a candidate), the safety
// boundary (the pack markers over the normalized part kind value — the note
// always, the stop only under the diagnostic uncertainty or a
// contradiction) and the contradiction boundary (the conflicting
// sources are handed to the human with the full material). The handoff
// recommendation words follow the 2.13 dictionary extension of 4.3.
func partBoundaries(schema PartSchemaDomain, mode string, facts map[string]VehicleFactInput, conflicting bool, view *PartRequestView) {
	// The part_kind material for the safety search and the candidate
	// word — resolved BY ROLE over the schema (never a hardcoded key
	// literal: the dictionary comes from the pack).
	kindKey, kindSpec := partRoleKeyOf(schema, PartRolePartKind)
	kindMaterial := partFactMaterial{}
	if kindSpec != nil {
		kindMaterial = partMaterialOf(facts, kindKey)
	}
	safetyMatched, matchedMarkers := partSafetyMarkersOf(schema, kindMaterial.Text)
	if mode == PartModeDiagnostic {
		view.Boundaries = append(view.Boundaries, PartBoundaryView{Kind: "diagnostic", Text: PartRequestDiagnosticBoundaryText})
		view.HandoffWords = append(view.HandoffWords, HandoffReasonDiagnosticUncertainty)
		if kindMaterial.Present {
			view.Boundaries = append(view.Boundaries, PartBoundaryView{Kind: "diagnostic", Text: PartRequestCandidateUnderSymptomText})
		}
	}
	if safetyMatched {
		view.Boundaries = append(view.Boundaries, PartBoundaryView{Kind: "safety", Text: PartRequestSafetyNoteText + " (маркеры: " + strings.Join(matchedMarkers, ", ") + ")"})
		view.Boundaries = append(view.Boundaries, PartBoundaryView{Kind: "safety", Text: PartRequestSafetyHeuristicText})
		if mode == PartModeDiagnostic || conflicting {
			view.Boundaries = append(view.Boundaries, PartBoundaryView{Kind: "safety", Text: PartRequestSafetyStopText})
			view.HandoffWords = append(view.HandoffWords, HandoffReasonSafetyCritical)
		}
	}
	if conflicting {
		view.Boundaries = append(view.Boundaries, PartBoundaryView{Kind: "contradiction", Text: PartRequestContradictionBoundaryText})
		view.HandoffWords = append(view.HandoffWords, HandoffReasonUnresolvedContradiction)
	}
}

// partRoleKeyOf resolves the key and the spec of the FIRST schema
// material key carrying the given role (the role drives the ladder and
// the boundary searches — the key names belong to the pack).
func partRoleKeyOf(schema PartSchemaDomain, role string) (string, *PartMaterialKeyDomain) {
	for index := range schema.MaterialKeys {
		if schema.MaterialKeys[index].Role == role {
			return schema.MaterialKeys[index].Key, &schema.MaterialKeys[index]
		}
	}
	return "", nil
}

// partSafetyMarkersOf answers whether any declared safety marker of the
// pack matches the normalized part kind value (the substring search over the
// lower-cased trimmed kind — the markers are the declared heuristic of
// the pack, honestly a heuristic, never a proof).
func partSafetyMarkersOf(schema PartSchemaDomain, kindRaw string) (bool, []string) {
	normalized := strings.ToLower(strings.TrimSpace(kindRaw))
	if normalized == "" {
		return false, nil
	}
	matched := []string{}
	for _, marker := range schema.SafetyMarkers {
		if strings.Contains(normalized, strings.ToLower(strings.TrimSpace(marker))) {
			matched = append(matched, marker)
		}
	}
	return len(matched) > 0, matched
}
