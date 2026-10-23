package domain

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// The Story 4.3 unit matrix: the part number normalization (the flags,
// never silent fixes — the raw value preserved, the spaces dropped, the
// «_»/«.» unified into «-», the cyrillic homoglyphs detected without
// replacement), the deterministic mode ladder over the pack dictionary
// (the four modes by name, the symptom dominance, both preference
// values, the empty material), the read-time part request derivation
// (the completeness over the SELECTED mode's set with the minimal
// questions, the PartRequestDigest stability and change, the stale
// word, the three honesty boundaries, the confidence word).

// partTestTime is the fixed derivation moment.
func partTestTime() time.Time {
	return time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
}

// partTestSchema builds the schema of the auto-parts@4.0.0 pack (the
// mirror of the catalog — the pack is the schema source, the domain
// gets it through the narrow port).
func partTestSchema() PartSchemaDomain {
	return PartSchemaDomain{
		MaterialKeys: []PartMaterialKeyDomain{
			{Key: "part.kind", Label: "Деталь", Kind: "text", Role: PartRolePartKind},
			{Key: "part.side", Label: "Сторона", Kind: "text", Optional: true},
			{Key: "part.quantity", Label: "Количество", Kind: "integer", Min: 1, Max: 999},
			{Key: "part.preference", Label: "Предпочтение", Kind: "enum", Values: []string{"original", "analogs", "both"}, Role: PartRolePreference},
			{Key: "part.oem_number", Label: "OEM-номер", Kind: "text", Role: PartRoleOemNumber},
			{Key: "part.symptom", Label: "Симптом", Kind: "text", Role: PartRoleSymptom},
			{Key: "part.budget", Label: "Бюджет", Kind: "text", Optional: true},
			{Key: "part.deadline", Label: "Срок", Kind: "text", Optional: true},
			{Key: "part.brands", Label: "Приемлемые бренды", Kind: "text", Optional: true},
			{Key: "part.constraints", Label: "Ограничения", Kind: "text", Optional: true},
		},
		Modes: []PartModeSpecDomain{
			{Mode: PartModeDiagnostic, Label: "Диагностика по симптому", Priority: 1, Requires: []string{"part.symptom"}},
			{Mode: PartModeExactRepeat, Label: "Точный повтор по номеру", Priority: 2, Requires: []string{"part.kind", "part.oem_number"}},
			{Mode: PartModeOriginalVsAnalog, Label: "Выбор: оригинал или аналог", Priority: 3, Requires: []string{"part.kind", "part.preference"}},
			{Mode: PartModeNamedPart, Label: "Названная деталь", Priority: 4, Requires: []string{"part.kind", "part.quantity"}},
		},
		SafetyMarkers: []string{"тормоз", "рул", "подушк", "ремн безопасн", "абс", "есп"},
	}
}

// partFact builds one fact input with one version per value (the 2.2
// fact material of the tests).
func partFact(key string, values ...string) PartRequestFactInput {
	fact := PartRequestFactInput{MaterialKey: key, Versions: []FactVersion{}, Confirmations: []FactConfirmation{}}
	for index, value := range values {
		canonical, _ := json.Marshal(value)
		fact.Versions = append(fact.Versions, FactVersion{
			VersionNo: int64(index + 1), MaterialKey: key, Value: canonical,
			ValueDigest: digestOfTest(canonical), SourceType: FactSourceChannelEvent,
			Channel: "text", ObservedAt: partTestTime(), ReceivedAt: partTestTime(),
		})
	}
	return fact
}

// partConfirm binds one confirmation of the exact version.
func partConfirm(fact PartRequestFactInput, versionNo int64, confirmer string) PartRequestFactInput {
	fact.Confirmations = append(fact.Confirmations, FactConfirmation{
		VersionNo: versionNo, ConfirmerType: confirmer, Note: "evidence",
		ActorID: continuationTestTenant(), CreatedAt: partTestTime(),
	})
	return fact
}

// TestNormalizePartNumberPreservesRawAndUnifiesSeparators pins AC-4: the
// raw value is preserved verbatim; the normalized rendering trims,
// upper-cases, drops the spaces and unifies the «_»/«.» separators into
// «-» — nothing else changes, the fact material is never touched.
func TestNormalizePartNumberPreservesRawAndUnifiesSeparators(t *testing.T) {
	answer := NormalizePartNumber(" wf 2b19 6k666 aa ")
	if answer.Raw != " wf 2b19 6k666 aa " {
		t.Fatalf("raw value must be preserved verbatim, got %q", answer.Raw)
	}
	if answer.Normalized != "WF2B196K666AA" {
		t.Fatalf("normalized rendering must trim, upper-case and drop the spaces, got %q", answer.Normalized)
	}
	if !answer.AlphabetOK || len(answer.Words) != 0 {
		t.Fatalf("the clean number carries no flags, got alphabet_ok=%v words=%v", answer.AlphabetOK, answer.Words)
	}
	separated := NormalizePartNumber("wf.2b19_6k666.aa")
	if separated.Normalized != "WF-2B19-6K666-AA" {
		t.Fatalf("the «_»/«.» separators must unify into «-», got %q", separated.Normalized)
	}
	// The idempotency: the normalization of the normalized rendering is
	// itself (the repeated derivation changes nothing).
	repeated := NormalizePartNumber(answer.Normalized)
	if repeated.Normalized != answer.Normalized {
		t.Fatalf("normalization must be idempotent, got %q over %q", repeated.Normalized, answer.Normalized)
	}
}

// TestNormalizePartNumberCyrillicHomoglyph pins the VIN discipline of
// 4.2 over the part numbers: the cyrillic homoglyph is DETECTED with
// the look-alike word and NEVER replaced — the re-ask is the only
// honest resolution.
func TestNormalizePartNumberCyrillicHomoglyph(t *testing.T) {
	answer := NormalizePartNumber("WF2B19АK666AA")
	if len(answer.CyrillicAmbiguities) != 1 {
		t.Fatalf("one cyrillic homoglyph expected, got %v", answer.CyrillicAmbiguities)
	}
	ambiguity := answer.CyrillicAmbiguities[0]
	if ambiguity.Rune != "А" || ambiguity.LookAlike != "A" {
		t.Fatalf("the cyrillic А/latin A homoglyph expected, got %+v", ambiguity)
	}
	if strings.Contains(answer.Normalized, "A") && !strings.Contains(answer.Normalized, "А") {
		t.Fatalf("the replacement is FORBIDDEN — the normalized rendering must keep the cyrillic rune verbatim, got %q", answer.Normalized)
	}
	if len(answer.Words) != 1 || !strings.Contains(answer.Words[0], "сверьте номер посимвольно") {
		t.Fatalf("the homoglyph word must ask the symbol-by-symbol check, got %v", answer.Words)
	}
}

// TestNormalizePartNumberAlphabet pins the honest alphabet flag: the
// special symbols outside the allowed alphabet (the latin letters, the
// digits, «-», «_», «.» and the spaces of the raw form) are flagged
// with their positions — never silently discarded.
func TestNormalizePartNumberAlphabet(t *testing.T) {
	answer := NormalizePartNumber("WF#2B196K*666")
	if answer.AlphabetOK {
		t.Fatal("the out-of-alphabet symbols must flag the alphabet")
	}
	if len(answer.BadChars) != 2 {
		t.Fatalf("two out-of-alphabet positions expected, got %v", answer.BadChars)
	}
	if len(answer.Words) != 1 || !strings.Contains(answer.Words[0], "вне разрешённого алфавита") {
		t.Fatalf("the alphabet word must enumerate the positions, got %v", answer.Words)
	}
}

// TestDerivePartRequestModeMatrixByName pins the deterministic ladder
// of the auto-parts@4.0.0 dictionary: the four modes by name, the
// symptom dominance over everything, both preference values (the «both»
// choice selects original_vs_analog; the constraint values leave the
// named-part family) and the honest unstructured word of the empty
// material.
func TestDerivePartRequestModeMatrixByName(t *testing.T) {
	schema := partTestSchema()
	cases := []struct {
		name  string
		facts []PartRequestFactInput
		mode  string
		basis string
	}{
		{
			name:  "symptom only selects diagnostic",
			facts: []PartRequestFactInput{partFact("part.symptom", "грохот при торможении на кочках")},
			mode:  PartModeDiagnostic,
			basis: "основание: записан симптом",
		},
		{
			name:  "kind and oem number select exact repeat",
			facts: []PartRequestFactInput{partFact("part.kind", "фильтр салона"), partFact("part.oem_number", "wf 2b19 6k666 aa")},
			mode:  PartModeExactRepeat,
			basis: "основание: названа деталь; записан номер детали",
		},
		{
			name:  "kind and preference both select the choice mode",
			facts: []PartRequestFactInput{partFact("part.kind", "топливный насос"), partFact("part.preference", "both")},
			mode:  PartModeOriginalVsAnalog,
			basis: "основание: названа деталь; клиент просит выбор (значение both)",
		},
		{
			name:  "kind with the analogs constraint stays named part",
			facts: []PartRequestFactInput{partFact("part.kind", "щётки стеклоочистителя"), partFact("part.preference", "analogs"), partFact("part.quantity", "2")},
			mode:  PartModeNamedPart,
			basis: "основание: названа деталь",
		},
		{
			name:  "kind and quantity select named part",
			facts: []PartRequestFactInput{partFact("part.kind", "щётки стеклоочистителя"), partFact("part.quantity", "2")},
			mode:  PartModeNamedPart,
			basis: "основание: названа деталь",
		},
		{
			name: "symptom dominates over the named part, the number and the choice",
			facts: []PartRequestFactInput{
				partFact("part.kind", "тормозные колодки"), partFact("part.oem_number", "1234567890"),
				partFact("part.preference", "both"), partFact("part.symptom", "грохот при торможении"),
			},
			mode:  PartModeDiagnostic,
			basis: "основание: записан симптом",
		},
	}
	for _, testCase := range cases {
		byKey := map[string]VehicleFactInput{}
		for _, fact := range testCase.facts {
			byKey[fact.MaterialKey] = fact
		}
		answer := DerivePartRequestMode(schema, byKey)
		if answer.Mode != testCase.mode {
			t.Fatalf("%s: mode %q expected, got %q", testCase.name, testCase.mode, answer.Mode)
		}
		if answer.BasisWord != testCase.basis {
			t.Fatalf("%s: basis %q expected, got %q", testCase.name, testCase.basis, answer.BasisWord)
		}
	}
	empty := DerivePartRequestMode(schema, map[string]VehicleFactInput{})
	if empty.Mode != "" || empty.UnstructuredWord != PartRequestUnstructuredWord {
		t.Fatalf("the empty material must carry the honest unstructured word, got %+v", empty)
	}
}

// TestDerivePartRequestUnstructuredDictionary pins the honest empty
// state: no mode, the unstructured word and the schema dictionary with
// the labels (the operator records the material by the pack dictionary
// — never a guess).
func TestDerivePartRequestUnstructuredDictionary(t *testing.T) {
	view := DerivePartRequest(partTestSchema(), nil, 1)
	if view.Mode != "" || view.UnstructuredWord != PartRequestUnstructuredWord {
		t.Fatalf("the empty material must carry the honest unstructured word, got %+v", view.Mode)
	}
	if len(view.SchemaDictionary) != 10 {
		t.Fatalf("the schema dictionary hint must carry the 10 material keys, got %d", len(view.SchemaDictionary))
	}
	if len(view.Attributes) != 0 || len(view.Completeness.MissingKeys) != 0 {
		t.Fatalf("no attributes and no missing keys over the empty material, got %+v", view)
	}
	if view.ConfidenceWord != "материал без подтверждения" {
		t.Fatalf("the honest confidence word of the empty material expected, got %q", view.ConfidenceWord)
	}
}

// TestDerivePartRequestCompletenessMinimalQuestions pins AC-3: the
// missing keys cover ONLY the not-confirmed keys of the SELECTED mode's
// mandatory set (the confirmed kind is never re-asked); every missing
// key carries ONE minimal clarify question; the absent optional keys
// never appear.
func TestDerivePartRequestCompletenessMinimalQuestions(t *testing.T) {
	facts := []PartRequestFactInput{
		partConfirm(partFact("part.kind", "тормозные колодки"), 1, ConfirmerOperatorRelay),
	}
	view := DerivePartRequest(partTestSchema(), facts, 1)
	if view.Mode != PartModeNamedPart {
		t.Fatalf("named_part expected over the confirmed kind, got %q", view.Mode)
	}
	if len(view.Completeness.MissingKeys) != 1 || view.Completeness.MissingKeys[0] != "part.quantity" {
		t.Fatalf("only the not-confirmed quantity is missing, got %v", view.Completeness.MissingKeys)
	}
	if view.Questions["part.quantity"] != "уточните: сколько нужно?" {
		t.Fatalf("the minimal quantity question expected, got %q", view.Questions["part.quantity"])
	}
	if view.ConfidenceWord != "материал подтверждён частично: подтверждены 1 из 2 обязательных ключей выбранного режима" {
		t.Fatalf("the honest partial confidence word expected, got %q", view.ConfidenceWord)
	}
	// The confirmed full set: no missing keys, the complete word.
	full := append(facts, partConfirm(partFact("part.quantity", "2"), 1, ConfirmerOperatorRelay))
	viewFull := DerivePartRequest(partTestSchema(), full, 1)
	if len(viewFull.Completeness.MissingKeys) != 0 {
		t.Fatalf("the confirmed mandatory set carries no missing keys, got %v", viewFull.Completeness.MissingKeys)
	}
	if viewFull.ConfidenceWord != "материал подтверждён" {
		t.Fatalf("the confirmed word expected, got %q", viewFull.ConfidenceWord)
	}
}

// TestDerivePartRequestOemNumberChecksAndConstantWord pins AC-4: the
// oem_number attribute carries the read-time number checks (the raw
// preserved, the normalized rendering, the flags with the honest words)
// and the view carries the constant word — the exact match is NOT a
// fitment proof (FR-52, the catalog contour is 4.4).
func TestDerivePartRequestOemNumberChecksAndConstantWord(t *testing.T) {
	facts := []PartRequestFactInput{
		partFact("part.kind", "фильтр салона"),
		partFact("part.oem_number", " wf 2b19 6k666 aa "),
	}
	view := DerivePartRequest(partTestSchema(), facts, 1)
	if view.Mode != PartModeExactRepeat {
		t.Fatalf("exact_repeat expected, got %q", view.Mode)
	}
	if view.NumberMatchWord != PartNumberMatchIsNotFitmentText {
		t.Fatalf("the constant number-match word expected, got %q", view.NumberMatchWord)
	}
	var oem *PartAttributeView
	for index := range view.Attributes {
		if view.Attributes[index].Role == PartRoleOemNumber {
			oem = &view.Attributes[index]
		}
	}
	if oem == nil || oem.NumberChecks == nil {
		t.Fatal("the oem_number attribute must carry the number checks")
	}
	if oem.NumberChecks.Raw != " wf 2b19 6k666 aa " || oem.NumberChecks.Normalized != "WF2B196K666AA" {
		t.Fatalf("the raw preservation and the normalized rendering expected, got %+v", oem.NumberChecks)
	}
}

// TestDerivePartRequestSafetyBoundaries pins AC-8: the matched safety
// marker of the pack renders the note and the heuristic word; the STOP
// happens only under the diagnostic uncertainty (the symptom with the
// safety node) or a contradiction — a plain named safety part never
// stops the selection.
func TestDerivePartRequestSafetyBoundaries(t *testing.T) {
	// The safety node under the symptom: the stop word and the
	// safety_critical recommendation.
	withSymptom := []PartRequestFactInput{
		partFact("part.symptom", "грохот при торможении на кочках"),
		partFact("part.kind", "тормозные колодки"),
	}
	viewSymptom := DerivePartRequest(partTestSchema(), withSymptom, 1)
	foundStop, foundCritical := false, false
	for _, boundary := range viewSymptom.Boundaries {
		if boundary.Kind == "safety" && boundary.Text == PartRequestSafetyStopText {
			foundStop = true
		}
	}
	for _, word := range viewSymptom.HandoffWords {
		if word == "safety_critical" {
			foundCritical = true
		}
	}
	if !foundStop || !foundCritical {
		t.Fatalf("the safety node under the symptom must stop and recommend safety_critical, got %v / %v", viewSymptom.Boundaries, viewSymptom.HandoffWords)
	}
	// The plain named safety part: the note + the heuristic word, NO
	// stop, NO safety_critical recommendation.
	plain := []PartRequestFactInput{
		partFact("part.kind", "тормозные колодки"),
		partFact("part.quantity", "2"),
	}
	viewPlain := DerivePartRequest(partTestSchema(), plain, 1)
	foundNote, foundHeuristic := false, false
	for _, boundary := range viewPlain.Boundaries {
		if boundary.Kind == "safety" && strings.Contains(boundary.Text, PartRequestSafetyNoteText) {
			foundNote = true
		}
		if boundary.Kind == "safety" && boundary.Text == PartRequestSafetyHeuristicText {
			foundHeuristic = true
		}
		if boundary.Text == PartRequestSafetyStopText {
			t.Fatalf("the plain safety part without the uncertainty must NOT stop, got %v", viewPlain.Boundaries)
		}
	}
	if !foundNote || !foundHeuristic {
		t.Fatalf("the safety note and the heuristic word expected, got %v", viewPlain.Boundaries)
	}
	for _, word := range viewPlain.HandoffWords {
		if word == "safety_critical" {
			t.Fatal("the plain safety part carries no safety_critical recommendation")
		}
	}
	// The non-safety part: no safety boundary at all.
	neutral := []PartRequestFactInput{
		partFact("part.kind", "щётки стеклоочистителя"),
		partFact("part.quantity", "2"),
	}
	viewNeutral := DerivePartRequest(partTestSchema(), neutral, 1)
	for _, boundary := range viewNeutral.Boundaries {
		if boundary.Kind == "safety" {
			t.Fatalf("the non-safety part carries no safety boundary, got %v", viewNeutral.Boundaries)
		}
	}
}

// TestDerivePartRequestContradictionBoundary pins AC-9: the conflicting
// sources render the contradiction boundary with the
// unresolved_contradiction recommendation and the honest confidence
// word (the confidence is not derived over the contradictions — Staffy
// never chooses the most probable variant).
func TestDerivePartRequestContradictionBoundary(t *testing.T) {
	// The client correction: the kind was confirmed as v1, the operator
	// recorded v2 «другое значение» — the read-time 2.2 canon derives
	// conflicting with the stale word of the former confirmation.
	facts := []PartRequestFactInput{
		partConfirm(partFact("part.kind", "тормозные колодки", "тормозные диски"), 1, ConfirmerOperatorRelay),
	}
	view := DerivePartRequest(partTestSchema(), facts, 1)
	foundBoundary, foundWord := false, false
	for _, boundary := range view.Boundaries {
		if boundary.Kind == "contradiction" && boundary.Text == PartRequestContradictionBoundaryText {
			foundBoundary = true
		}
	}
	for _, word := range view.HandoffWords {
		if word == "unresolved_contradiction" {
			foundWord = true
		}
	}
	if !foundBoundary || !foundWord {
		t.Fatalf("the contradiction boundary and word expected, got %v / %v", view.Boundaries, view.HandoffWords)
	}
	if !strings.Contains(view.ConfidenceWord, "противоречия") {
		t.Fatalf("the honest confidence word over the contradiction expected, got %q", view.ConfidenceWord)
	}
	var kind *PartAttributeView
	for index := range view.Attributes {
		if view.Attributes[index].Role == PartRolePartKind {
			kind = &view.Attributes[index]
		}
	}
	if kind == nil || kind.Status != FactStatusConflicting {
		t.Fatalf("the conflicting status of the kind attribute expected, got %+v", kind)
	}
	if kind.Confirmation == nil || kind.Confirmation.StaleWord != "подтверждение относилось к прежней версии (v1)" {
		t.Fatalf("the stale word of the former confirmation expected, got %+v", kind.Confirmation)
	}
}

// TestDerivePartRequestDigestStabilityAndChange pins AC-10: the
// PartRequestDigest is byte-stable over the same world (the idempotent
// JSON ×2) and CHANGES when a new version arrives after the
// confirmation (the stale aggregate note — the dependent contours must
// treat the former results as stale).
func TestDerivePartRequestDigestStabilityAndChange(t *testing.T) {
	facts := []PartRequestFactInput{
		partConfirm(partFact("part.kind", "топливный насос"), 1, ConfirmerOperatorRelay),
		partConfirm(partFact("part.quantity", "1"), 1, ConfirmerOperatorRelay),
	}
	first := DerivePartRequest(partTestSchema(), facts, 1)
	second := DerivePartRequest(partTestSchema(), facts, 1)
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("the derivation must be byte-stable over the same world (the idempotent JSON ×2)")
	}
	if len(first.PartRequestDigest) != 64 {
		t.Fatalf("the SHA-256 hex digest of 64 runes expected, got %q", first.PartRequestDigest)
	}
	if first.StaleNote != "" {
		t.Fatalf("the confirmed world carries no stale note, got %q", first.StaleNote)
	}
	// The client correction: a new version after the confirmation.
	corrected := []PartRequestFactInput{
		partConfirm(partFact("part.kind", "топливный насос", "другая деталь"), 1, ConfirmerOperatorRelay),
		partConfirm(partFact("part.quantity", "1"), 1, ConfirmerOperatorRelay),
	}
	viewCorrected := DerivePartRequest(partTestSchema(), corrected, 1)
	if viewCorrected.PartRequestDigest == first.PartRequestDigest {
		t.Fatal("the new version after the confirmation must CHANGE the digest (the exact version of the request)")
	}
	if viewCorrected.StaleNote != PartRequestStaleNote {
		t.Fatalf("the stale aggregate note expected, got %q", viewCorrected.StaleNote)
	}
}

// TestDerivePartRequestSeveralPartsWord pins the honest several-parts
// word: ONE structured set per case (the fact key space of 2.2); more
// than one active «поиск детали» item renders the split recommendation.
func TestDerivePartRequestSeveralPartsWord(t *testing.T) {
	facts := []PartRequestFactInput{partFact("part.kind", "тормозные колодки")}
	one := DerivePartRequest(partTestSchema(), facts, 1)
	if one.SeveralPartsWord != "" {
		t.Fatalf("a single active item carries no several-parts word, got %q", one.SeveralPartsWord)
	}
	two := DerivePartRequest(partTestSchema(), facts, 2)
	if two.SeveralPartsWord != PartRequestSeveralPartsText {
		t.Fatalf("the honest several-parts word expected, got %q", two.SeveralPartsWord)
	}
}

// TestPartMaterialKeysNeverHardcodedInCases pins the AC-2 probe: the
// part request material keys never appear as literals in the non-test
// sources of the cases domain — the schema is the pack material handed
// through the narrow taxonomy port, never a hardcoded core list.
func TestPartMaterialKeysNeverHardcodedInCases(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"part.kind", "part.side", "part.quantity", "part.preference", "part.oem_number", "part.symptom", "part.budget", "part.deadline", "part.brands", "part.constraints"} {
			if strings.Contains(string(raw), key) {
				t.Fatalf("%s must not hardcode the material key %q — the schema comes from the pinned pack", entry.Name(), key)
			}
		}
	}
}
