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
// Story 4.2: the vehicle identification domain (VIN normalization with
// flags, the three-row readback dictionary, the masked rendering and the
// read-time vehicle profile derivation)
// ---------------------------------------------------------------------------
//
// The honesty core of the slice: VIN is NEVER silently fixed — the
// homoglyphs are detected and re-asked, the check digit is computed and
// compared with words, the length and the alphabet are flags; the
// normalized rendering never replaces the raw fact value. The vehicle
// profile is a READ-TIME derivation over the append-only facts of 2.2
// (the DerivePilotAdmission / DeriveContinuationLinkState family): no
// new table, byte-stable idempotent JSON, zero writes. The full VIN is
// rendered on exactly two surfaces (the protected client form of FR-55
// and the operator work card); every other rendering path carries the
// mask.

// ---------------------------------------------------------------------------
// The readback dictionaries (three rows, AC-4)
// ---------------------------------------------------------------------------

// VINReadbackLatin is the latin row of the readback dictionary: the 22
// letters of the VIN alphabet (I, O and Q are not VIN letters — the
// dictionary refuses them by absence, never by silent replacement).
var VINReadbackLatin = map[rune]string{
	'A': "Альфа", 'B': "Браво", 'C': "Цезарь", 'D': "Дельта", 'E': "Эхо",
	'F': "Фокстрот", 'G': "Гольф", 'H': "Отель", 'J': "Джульетта",
	'K': "Кило", 'L': "Лима", 'M': "Майк", 'N': "Ноябрь", 'P': "Папа",
	'R': "Ромео", 'S': "Сьерра", 'T': "Танго", 'U': "Юниформ",
	'V': "Виктор", 'W': "Виски", 'X': "Икс-рей", 'Y': "Янки",
}

// VINReadbackDigits is the digit row of the readback dictionary.
var VINReadbackDigits = map[rune]string{
	'0': "ноль", '1': "один", '2': "два", '3': "три", '4': "четыре",
	'5': "пять", '6': "шесть", '7': "семь", '8': "восемь", '9': "девять",
}

// VINReadbackCyrillic is the cyrillic row of the readback dictionary:
// the twelve homoglyph letters (the cyrillic runes that look like the
// latin VIN letters). The row exists for the homoglyph re-asks — the
// disputed symbol is re-asked with ITS row word, the replacement is
// never performed.
var VINReadbackCyrillic = map[rune]string{
	'А': "Анна", 'В': "Виктор", 'Е': "Елена", 'К': "Константин",
	'М': "Михаил", 'Н': "Николай", 'О': "Ольга", 'Р': "Роман",
	'С': "Семён", 'Т': "Татьяна", 'У': "Ульяна", 'Х': "Харитон",
}

// vinCyrillicLookAlike maps the cyrillic homoglyph runes to the latin
// letters they are confused with (the LookAlike word of the ambiguity).
var vinCyrillicLookAlike = map[rune]rune{
	'А': 'A', 'В': 'B', 'Е': 'E', 'К': 'K', 'М': 'M', 'Н': 'H',
	'О': 'O', 'Р': 'P', 'С': 'C', 'Т': 'T', 'У': 'Y', 'Х': 'X',
}

// isVINLetter answers whether the rune belongs to the latin VIN
// alphabet (no I, O, Q).
func isVINLetter(r rune) bool {
	_, ok := VINReadbackLatin[r]
	return ok
}

// isCyrillicRune answers whether the rune belongs to the cyrillic
// unicode block (U+0400..U+04FF) — the homoglyph detection boundary.
func isCyrillicRune(r rune) bool {
	return r >= 0x0400 && r <= 0x04FF
}

// ---------------------------------------------------------------------------
// The VIN normalization (flags, never silent fixes — AC-3)
// ---------------------------------------------------------------------------

// VINAmbiguity is one cyrillic homoglyph position: the rune, its
// position in the normalized rendering and the latin look-alike. The
// detection NEVER replaces the symbol — the re-ask is the only honest
// resolution (AC-3 tail «неоднозначные символы не исправляются молча»).
type VINAmbiguity struct {
	Position  int    `json:"position"`
	Rune      string `json:"rune"`
	LookAlike string `json:"look_alike"`
}

// VINCheckDigit is the ISO 3779 check-digit comparison of the position 9:
// the expected rune (the stored one), the computed rune (the weighted
// sum over the transliteration table) and the match flag. Applicable is
// honest — a non-standard length, a forbidden letter, a homoglyph or an
// out-of-alphabet rune makes the computation honestly unavailable (a
// European VIN without an actual check digit never gets a false
// verdict).
type VINCheckDigit struct {
	Applicable bool   `json:"applicable"`
	Expected   string `json:"expected"`
	Computed   string `json:"computed"`
	Match      bool   `json:"match"`
}

// VINNormalization is the full flag answer of NormalizeVIN: the raw
// value (ALWAYS preserved verbatim), the normalized rendering (the trim
// + the upper case + the ASCII separators dropped — nothing else), the
// length flag, the forbidden-letter positions, the homoglyph
// ambiguities, the check-digit comparison and the honest words of every
// flag. The normalization is idempotent: the normalization of the
// normalized rendering is itself.
type VINNormalization struct {
	Raw                 string         `json:"raw"`
	Normalized          string         `json:"normalized"`
	Length              int            `json:"length"`
	LengthOK            bool           `json:"length_ok"`
	ForbiddenChars      []int          `json:"forbidden_chars"`
	CyrillicAmbiguities []VINAmbiguity `json:"cyrillic_ambiguities"`
	CheckDigit          VINCheckDigit  `json:"check_digit"`
	Words               []string       `json:"words"`
}

// vinTransliteration is the ISO 3779 transliteration table of the latin
// VIN letters (I, O, Q absent from the alphabet).
var vinTransliteration = map[rune]int{
	'A': 1, 'B': 2, 'C': 3, 'D': 4, 'E': 5, 'F': 6, 'G': 7, 'H': 8,
	'J': 1, 'K': 2, 'L': 3, 'M': 4, 'N': 5, 'P': 7, 'R': 9,
	'S': 2, 'T': 3, 'U': 4, 'V': 5, 'W': 6, 'X': 7, 'Y': 8, 'Z': 9,
}

// vinCheckDigitWeights is the ISO 3779 position-weight table (the
// position 9 carries the check digit itself — weight 0).
var vinCheckDigitWeights = [17]int{8, 7, 6, 5, 4, 3, 2, 10, 0, 9, 8, 7, 6, 5, 4, 3, 2}

// NormalizeVIN derives the honest flag answer of one raw VIN value. The
// raw value is preserved verbatim (the fact material of 2.2); the
// normalized rendering only trims, upper-cases and drops the ASCII
// separators/spaces; EVERY check is a flag with an honest word — no
// symbol is ever replaced, no value is ever discarded. The check digit
// (ISO 3779) is computed for the full 17-rune normalized rendering
// without forbidden letters, homoglyphs or out-of-alphabet runes; a
// mismatch is the flag and the word «сверьте посимвольно», never a
// rejection.
func NormalizeVIN(raw string) VINNormalization {
	answer := VINNormalization{
		Raw: raw, ForbiddenChars: []int{}, CyrillicAmbiguities: []VINAmbiguity{},
		Words: []string{},
	}
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	var builder strings.Builder
	for _, r := range normalized {
		if strings.ContainsRune(" \t\r\n-_./", r) {
			continue
		}
		builder.WriteRune(r)
	}
	answer.Normalized = builder.String()
	runes := []rune(answer.Normalized)
	answer.Length = len(runes)
	answer.LengthOK = answer.Length == 17
	outOfAlphabet := []int{}
	for index, r := range runes {
		switch {
		case r == 'I' || r == 'O' || r == 'Q':
			answer.ForbiddenChars = append(answer.ForbiddenChars, index)
		case isVINLetter(r) || (r >= '0' && r <= '9'):
			// the alphabet members carry no flag
		case isCyrillicRune(r):
			if lookAlike, ok := vinCyrillicLookAlike[r]; ok {
				answer.CyrillicAmbiguities = append(answer.CyrillicAmbiguities, VINAmbiguity{
					Position: index, Rune: string(r), LookAlike: string(lookAlike),
				})
			} else {
				outOfAlphabet = append(outOfAlphabet, index)
			}
		default:
			outOfAlphabet = append(outOfAlphabet, index)
		}
	}
	// The check digit: applicable only over the standard rendering.
	answer.CheckDigit = VINCheckDigit{Expected: "-", Computed: "-"}
	if answer.LengthOK && len(answer.ForbiddenChars) == 0 && len(answer.CyrillicAmbiguities) == 0 && len(outOfAlphabet) == 0 {
		answer.CheckDigit.Applicable = true
		answer.CheckDigit.Expected = string(runes[8])
		answer.CheckDigit.Computed = vinComputeCheckDigit(runes)
		answer.CheckDigit.Match = answer.CheckDigit.Expected == answer.CheckDigit.Computed
	}
	answer.Words = vinHonestWords(answer, outOfAlphabet)
	return answer
}

// vinComputeCheckDigit computes the ISO 3779 check digit of the full
// 17-rune VIN (the remainder of the weighted transliteration sum; the
// remainder 10 renders as X).
func vinComputeCheckDigit(runes []rune) string {
	sum := 0
	for index, r := range runes {
		weight := vinCheckDigitWeights[index]
		if weight == 0 {
			continue
		}
		if digit := int(r - '0'); r >= '0' && r <= '9' {
			sum += digit * weight
			continue
		}
		sum += vinTransliteration[r] * weight
	}
	remainder := sum % 11
	if remainder == 10 {
		return "X"
	}
	return string(rune('0' + remainder))
}

// vinHonestWords renders the honest words of every flag (AC-14: the
// flags are text, never only color).
func vinHonestWords(normalization VINNormalization, outOfAlphabet []int) []string {
	words := []string{}
	if !normalization.LengthOK {
		words = append(words, fmt.Sprintf("длина %d ≠ 17 — сверьте количество символов", normalization.Length))
	}
	if len(normalization.ForbiddenChars) > 0 {
		positions := make([]string, 0, len(normalization.ForbiddenChars))
		for _, position := range normalization.ForbiddenChars {
			positions = append(positions, fmt.Sprintf("%d", position+1))
		}
		words = append(words, fmt.Sprintf("буквы I, O, Q в VIN не используются — сверьте позицию %s", strings.Join(positions, ", ")))
	}
	for _, ambiguity := range normalization.CyrillicAmbiguities {
		words = append(words, fmt.Sprintf("позиция %d: кириллица %s (похожа на латиницу %s) — требуется сверка", ambiguity.Position+1, ambiguity.Rune, ambiguity.LookAlike))
	}
	for _, position := range outOfAlphabet {
		words = append(words, fmt.Sprintf("позиция %d: символ вне VIN-алфавита — сверьте посимвольно", position+1))
	}
	if normalization.CheckDigit.Applicable && !normalization.CheckDigit.Match {
		words = append(words, fmt.Sprintf("контрольная цифра не совпала (позиция 9: ожидается %s, вычислено %s) — сверьте посимвольно", normalization.CheckDigit.Expected, normalization.CheckDigit.Computed))
	}
	return words
}

// ---------------------------------------------------------------------------
// The readback blocks (4-4-4-4-1) and the focus re-ask (AC-4)
// ---------------------------------------------------------------------------

// VINReadbackSymbol is one symbol of one readback block: the row word
// (латиница | цифра | кириллица — the rows are distinguished BY WORDS),
// the symbol and the dictionary word of its row.
type VINReadbackSymbol struct {
	Position int    `json:"position"`
	Row      string `json:"row"`
	Char     string `json:"char"`
	Word     string `json:"word"`
}

// VINReadbackBlock is one readback block (4-4-4-4-1) with its symbols.
type VINReadbackBlock struct {
	Block   string              `json:"block"`
	Symbols []VINReadbackSymbol `json:"symbols"`
}

// ReadbackBlocks renders the block readback of the normalized VIN: the
// blocks 4-4-4-4-1, every symbol with its row word and its dictionary
// word (the text alternative of the voice readback — it exists always,
// AC-14). An unknown rune renders the honest out-of-dictionary word —
// the readback never guesses.
func ReadbackBlocks(normalized string) []VINReadbackBlock {
	runes := []rune(normalized)
	blocks := []VINReadbackBlock{}
	sizes := []int{4, 4, 4, 4, 1}
	offset := 0
	for _, size := range sizes {
		if offset >= len(runes) {
			break
		}
		end := offset + size
		if end > len(runes) {
			end = len(runes)
		}
		block := VINReadbackBlock{Block: string(runes[offset:end]), Symbols: []VINReadbackSymbol{}}
		for index := offset; index < end; index++ {
			r := runes[index]
			symbol := VINReadbackSymbol{Position: index, Char: string(r)}
			switch {
			case isVINLetter(r):
				symbol.Row, symbol.Word = "латиница", VINReadbackLatin[r]
			case r >= '0' && r <= '9':
				symbol.Row, symbol.Word = "цифра", VINReadbackDigits[r]
			case isCyrillicRune(r):
				if word, ok := VINReadbackCyrillic[r]; ok {
					symbol.Row, symbol.Word = "кириллица", word
				} else {
					symbol.Row, symbol.Word = "кириллица", "символ вне readback-словаря"
				}
			default:
				symbol.Row, symbol.Word = "другое", "символ вне readback-словаря"
			}
			block.Symbols = append(block.Symbols, symbol)
		}
		blocks = append(blocks, block)
		offset = end
	}
	return blocks
}

// ReadbackFocus renders the focus re-ask words of the disputed symbols
// ONLY (the homoglyph ambiguities — AC-4 tail «переспрашивает только
// спорные»): each disputed position with both row words (the cyrillic
// word of the stored rune and the latin word of the look-alike — the
// client answers with the row distinction), then the honest «остальные
// символы приняты».
func ReadbackFocus(normalized string, ambiguities []VINAmbiguity) []string {
	words := []string{}
	for _, ambiguity := range ambiguities {
		cyrillicWord := VINReadbackCyrillic[[]rune(ambiguity.Rune)[0]]
		latinWord := VINReadbackLatin[[]rune(ambiguity.LookAlike)[0]]
		words = append(words, fmt.Sprintf("позиция %d: кириллица %s («%s») либо латиница %s («%s»)?", ambiguity.Position+1, ambiguity.Rune, cyrillicWord, ambiguity.LookAlike, latinWord))
	}
	if len(ambiguities) > 0 {
		words = append(words, "остальные символы приняты")
	}
	return words
}

// ---------------------------------------------------------------------------
// The masked rendering (AC-9)
// ---------------------------------------------------------------------------

// MaskedVIN renders the masked VIN: the first three runes (the WMI) +
// «…» + the last four runes; the full length is never disclosed by the
// mask. A value shorter than eight runes degrades honestly (the first,
// the ellipsis and the last rune — never an invented length).
func MaskedVIN(normalized string) string {
	runes := []rune(strings.ToUpper(strings.TrimSpace(normalized)))
	switch {
	case len(runes) >= 8:
		return string(runes[:3]) + "…" + string(runes[len(runes)-4:])
	case len(runes) >= 2:
		return string(runes[0]) + "…" + string(runes[len(runes)-1])
	case len(runes) == 1:
		return string(runes[0]) + "…"
	default:
		return "…"
	}
}

// ---------------------------------------------------------------------------
// The read-time vehicle profile derivation (AC-12, AC-8)
// ---------------------------------------------------------------------------

// VehicleKeySpec is the domain form of one identification material key
// of the pack schema (the app maps the narrow port answer onto it — the
// domain never imports the port).
type VehicleKeySpec struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // vin | text | integer
	Min      int    `json:"min,omitempty"`
	Max      int    `json:"max,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

// VehicleSchema is the identification schema of the derivation input.
type VehicleSchema struct {
	MaterialKeys    []VehicleKeySpec `json:"material_keys"`
	AlternativeSets [][]string       `json:"alternative_sets"`
}

// VehicleFactInput is one fact of the case handed to the derivation:
// the material key with the append-only versions and the confirmations
// (the port FactMaterial mapping of the app).
type VehicleFactInput struct {
	FactID        id.UUID
	MaterialKey   string
	Versions      []FactVersion
	Confirmations []FactConfirmation
}

// VehicleConfirmationView is the confirmation rendering of one
// attribute: the confirmer type, the EXACT confirmed version number,
// the moment and the stale word when a newer version arrived after the
// confirmation (AC-8 — the confirmation never rewrites; the new version
// makes it honestly stale).
type VehicleConfirmationView struct {
	ConfirmerType string    `json:"confirmer_type"`
	VersionNo     int64     `json:"version_no"`
	At            time.Time `json:"at"`
	StaleWord     string    `json:"stale_word,omitempty"`
}

// VehicleAttributeView is one attribute of the profile: the material
// key with the pack label, the raw value, the VIN normalization (only
// the vin kind — NormalizedValue is the derivation, never a stored
// rendering), the fact version with the provenance, the read-time 2.2
// status and the confirmation view when one exists.
type VehicleAttributeView struct {
	FactID          id.UUID                  `json:"fact_id"`
	MaterialKey     string                   `json:"material_key"`
	Label           string                   `json:"label"`
	Kind            string                   `json:"kind"`
	RawValue        string                   `json:"raw_value"`
	NormalizedValue *VINNormalization        `json:"normalized_value,omitempty"`
	FactVersion     int64                    `json:"fact_version"`
	ObservedAt      time.Time                `json:"observed_at"`
	ReceivedAt      time.Time                `json:"received_at"`
	SourceType      string                   `json:"source_type"`
	Channel         string                   `json:"channel"`
	Status          string                   `json:"status"`
	Confirmation    *VehicleConfirmationView `json:"confirmation,omitempty"`
}

// VehicleCompleteness is the honest completeness answer: the satisfied
// alternative sets (any ONE set satisfies the identification — presence
// based) and the missing keys (only the NOT confirmed — the confirmed
// attributes are never re-asked, AC-2).
type VehicleCompleteness struct {
	SatisfiedBy []string `json:"satisfied_by"`
	MissingKeys []string `json:"missing_keys"`
}

// VehicleProfileView is the read-time profile: the attributes in the
// schema dictionary order, the completeness answer, the ProfileDigest
// (the SHA-256 of the canonical {key: {version, value_digest}} map of
// the CONFIRMED set — «the exact version of the profile», AC-12: any
// new version of any attribute changes it) and the aggregate stale note.
type VehicleProfileView struct {
	Attributes    []VehicleAttributeView `json:"attributes"`
	Completeness  VehicleCompleteness    `json:"completeness"`
	ProfileDigest string                 `json:"profile_digest"`
	StaleNote     string                 `json:"stale_note,omitempty"`
}

// VehicleProfileStaleNote is the honest aggregate stale word of the
// profile (AC-8: the dependent results — the future catalog results —
// must treat the profile as stale).
const VehicleProfileStaleNote = "Часть подтверждений относится к прежним версиям фактов: зависимые результаты считаются устаревшими — подтвердите точные версии."

// DeriveVehicleProfile derives the vehicle profile of one case over the
// append-only facts (READ-TIME: zero writes, idempotent — the JSON of
// the same world is byte-stable). The statuses are the 2.2 read-time
// derivations (DeriveFactStatus — the frozen canon); a new version of
// an attribute after a confirmation makes the attribute conflicting
// (the canon of 2.2: the confirmed value and the new value disagree)
// with the stale word «подтверждение относилось к прежней версии (vN)»
// — the ProfileDigest changes, the dependent results must treat the
// profile as stale. Facts outside the schema dictionary are ignored
// (the schema is the pack material — never a hardcoded core list).
func DeriveVehicleProfile(schema VehicleSchema, facts []VehicleFactInput) VehicleProfileView {
	view := VehicleProfileView{
		Attributes:   []VehicleAttributeView{},
		Completeness: VehicleCompleteness{SatisfiedBy: []string{}, MissingKeys: []string{}},
	}
	byKey := map[string]VehicleFactInput{}
	for _, fact := range facts {
		byKey[fact.MaterialKey] = fact
	}
	confirmedSet := map[string]struct {
		VersionNo   int64
		ValueDigest string
	}{}
	stale := false
	for _, key := range schema.MaterialKeys {
		fact, present := byKey[key.Key]
		if !present || len(fact.Versions) == 0 {
			view.Completeness.MissingKeys = append(view.Completeness.MissingKeys, key.Key)
			continue
		}
		latest := fact.Versions[len(fact.Versions)-1]
		status, _ := DeriveFactStatus(fact.Versions, fact.Confirmations)
		attribute := VehicleAttributeView{
			FactID: fact.FactID, MaterialKey: key.Key, Label: key.Label, Kind: key.Kind,
			RawValue: vehicleRawValueText(latest.Value), FactVersion: latest.VersionNo,
			ObservedAt: latest.ObservedAt, ReceivedAt: latest.ReceivedAt,
			SourceType: latest.SourceType, Channel: latest.Channel, Status: status,
		}
		if key.Kind == "vin" {
			normalization := NormalizeVIN(attribute.RawValue)
			attribute.NormalizedValue = &normalization
		}
		// The latest confirmation of the fact (the EXACT version binding).
		var latestConfirmation *FactConfirmation
		for index := range fact.Confirmations {
			confirmation := &fact.Confirmations[index]
			if latestConfirmation == nil || confirmation.VersionNo > latestConfirmation.VersionNo {
				latestConfirmation = confirmation
			}
		}
		if latestConfirmation != nil {
			confirmationView := &VehicleConfirmationView{
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
		} else {
			view.Completeness.MissingKeys = append(view.Completeness.MissingKeys, key.Key)
		}
		view.Attributes = append(view.Attributes, attribute)
	}
	// The satisfied alternative sets (presence based — the identification
	// material exists; the confirmation state lives in the statuses).
	for _, set := range schema.AlternativeSets {
		satisfied := true
		for _, key := range set {
			fact, present := byKey[key]
			if !present || len(fact.Versions) == 0 {
				satisfied = false
				break
			}
		}
		if satisfied {
			view.Completeness.SatisfiedBy = append(view.Completeness.SatisfiedBy, strings.Join(set, "+"))
		}
	}
	view.ProfileDigest = vehicleProfileDigest(confirmedSet)
	if stale {
		view.StaleNote = VehicleProfileStaleNote
	}
	return view
}

// vehicleProfileDigest renders the SHA-256 of the canonical JSON of the
// confirmed set — the map {key: {version_no, value_digest}} with the
// sorted keys (the byte-stable canonical form of «the exact version of
// the profile», AC-12; an empty set digests the honest empty object).
func vehicleProfileDigest(confirmedSet map[string]struct {
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
	// The deterministic marshal of the fixed shape (map[string]map[string]any
	// marshals with the sorted keys in Go).
	encoded, err := json.Marshal(canonical)
	if err != nil {
		// The shape is fixed (strings, int64, strings) — the marshal cannot
		// fail; the honest fallback keeps the derivation total.
		encoded = []byte("{}")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// vehicleRawValueText renders the human text of one canonical fact
// value: a JSON string decodes to its plain content (the fact storage
// carries the canonical quoted form — the 2.2 value discipline); any
// other JSON value renders verbatim. The helper never mutates the fact
// material — it is the rendering of the read-time view only.
func vehicleRawValueText(value []byte) string {
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return text
	}
	return string(value)
}

// GuardVehicleClientConfirmation validates the client confirmation of
// the vehicle attributes through the protected continuation: the fact
// exists (the version bounds), the note is bounded (the evidence of the
// client confirmation — the note carries the link reference, the
// evidence-event branch belongs to the operator path of 2.2). This is
// the OWN gate of the client path (the GuardConfirmFactVersion of 2.2
// guards the operator_relay producer — the ContinuationCorrection
// precedent: the surface write paths carry their own gates, never the
// operator guards).
func GuardVehicleClientConfirmation(versionCount int, versionNo int64, note string) error {
	if versionCount == 0 || versionNo < 1 || versionNo > int64(versionCount) {
		return fmt.Errorf("версия %d не существует у этого факта", versionNo)
	}
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return fmt.Errorf("подтверждение клиента требует ноту-источник: ссылка продолжения")
	}
	if len([]rune(trimmed)) > MaxNoteRunes {
		return fmt.Errorf("нота подтверждения длиннее %d символов", MaxNoteRunes)
	}
	return nil
}
