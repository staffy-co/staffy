package domain

import (
        "encoding/json"
        "os"
        "strings"
        "testing"
        "time"
)

// The Story 4.2 unit matrix: the VIN normalization (the flags, never
// silent fixes — the raw value preserved, the separators dropped only
// on the normalized rendering, the ISO 3779 check digit), the readback
// dictionaries of the three rows (pinned by name), the block readback
// 4-4-4-4-1 with the focus re-ask of the disputed symbols only, the
// masked rendering and the read-time vehicle profile derivation (the
// completeness over the alternative sets, the EXACT-version
// confirmations, the stale word, the ProfileDigest stability).

// vehicleTestTime is the fixed derivation moment.
func vehicleTestTime() time.Time {
        return time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
}

// vehicleTestSchema builds the schema of the auto-parts@3.0.0 pack
// (the mirror of the catalog — the pack is the schema source, the
// domain gets it through the narrow port).
func vehicleTestSchema() VehicleSchema {
        return VehicleSchema{
                MaterialKeys: []VehicleKeySpec{
                        {Key: "vehicle.vin", Label: "VIN", Kind: "vin"},
                        {Key: "vehicle.make", Label: "Марка", Kind: "text"},
                        {Key: "vehicle.model", Label: "Модель", Kind: "text"},
                        {Key: "vehicle.year_of_manufacture", Label: "Год выпуска", Kind: "integer", Min: 1900, Max: 2100},
                        {Key: "vehicle.trim", Label: "Модификация", Kind: "text", Optional: true},
                        {Key: "vehicle.engine", Label: "Двигатель", Kind: "text", Optional: true},
                },
                AlternativeSets: [][]string{
                        {"vehicle.vin"},
                        {"vehicle.make", "vehicle.model", "vehicle.year_of_manufacture"},
                },
        }
}

// vehicleFact builds one fact input with one version per value.
func vehicleFact(key string, values ...string) VehicleFactInput {
        fact := VehicleFactInput{MaterialKey: key, Versions: []FactVersion{}, Confirmations: []FactConfirmation{}}
        for index, value := range values {
                canonical, _ := json.Marshal(value)
                fact.Versions = append(fact.Versions, FactVersion{
                        VersionNo: int64(index + 1), MaterialKey: key, Value: canonical,
                        ValueDigest: digestOfTest(canonical), SourceType: FactSourceChannelEvent,
                        Channel: "text", ObservedAt: vehicleTestTime(), ReceivedAt: vehicleTestTime(),
                })
        }
        return fact
}

// vehicleConfirm binds one confirmation of the exact version.
func vehicleConfirm(fact VehicleFactInput, versionNo int64, confirmer string) VehicleFactInput {
        fact.Confirmations = append(fact.Confirmations, FactConfirmation{
                VersionNo: versionNo, ConfirmerType: confirmer, Note: "evidence",
                ActorID: continuationTestTenant(), CreatedAt: vehicleTestTime(),
        })
        return fact
}

// digestOfTest renders the stable digest helper of the test values.
func digestOfTest(canonical []byte) string {
        return string(canonical) + "-digest"
}

// TestNormalizeVINPreservesRawAndDropsSeparators pins AC-3: the raw
// value is preserved verbatim, the normalized rendering trims,
// upper-cases and drops the ASCII separators/spaces — nothing else
// changes, the provenance material is never touched.
func TestNormalizeVINPreservesRawAndDropsSeparators(t *testing.T) {
        answer := NormalizeVIN(" wf0xxxtt4s-1234567 ")
        if answer.Raw != " wf0xxxtt4s-1234567 " {
                t.Fatalf("raw value must be preserved verbatim, got %q", answer.Raw)
        }
        if answer.Normalized != "WF0XXXTT4S1234567" {
                t.Fatalf("normalized rendering must trim, upper-case and drop separators, got %q", answer.Normalized)
        }
        if answer.Length != 17 || !answer.LengthOK {
                t.Fatalf("17 runes expected, got %d", answer.Length)
        }
        // The idempotency: the normalization of the normalized rendering is
        // itself (the repeated derivation changes nothing).
        repeated := NormalizeVIN(answer.Normalized)
        if repeated.Normalized != answer.Normalized || repeated.Raw != answer.Normalized {
                t.Fatalf("normalization must be idempotent, got %q over %q", repeated.Normalized, answer.Normalized)
        }
}

// TestNormalizeVINValidCheckDigit pins the clean baseline: the
// mathematically valid VIN (the ISO 3779 check digit of the position 9
// matches) carries no words and no flags.
func TestNormalizeVINValidCheckDigit(t *testing.T) {
        answer := NormalizeVIN("JH4KA7561PC008269")
        if len(answer.Words) != 0 {
                t.Fatalf("a valid VIN carries no honest words, got %v", answer.Words)
        }
        if !answer.CheckDigit.Applicable || !answer.CheckDigit.Match {
                t.Fatalf("the check digit must be applicable and matching, got %+v", answer.CheckDigit)
        }
        if answer.CheckDigit.Expected != "1" || answer.CheckDigit.Computed != "1" {
                t.Fatalf("the check digit of JH4KA7561PC008269 is 1, got expected %q computed %q", answer.CheckDigit.Expected, answer.CheckDigit.Computed)
        }
}

// TestNormalizeVINCheckDigitMismatch pins the honest mismatch: the
// spec-example VIN WF0XXXTT4S1234567 does not carry a matching ISO
// 3779 check digit (the stored position 9 is 4, the computed one is X)
// — the honest derivation flags it with the «сверьте посимвольно» word
// instead of discarding the value (the canon of the ISO table wins
// over the spec example — the honest deviation of the slice,
// documented in the Spec Change Log).
func TestNormalizeVINCheckDigitMismatch(t *testing.T) {
        answer := NormalizeVIN("WF0XXXTT4S1234567")
        if !answer.CheckDigit.Applicable {
                t.Fatal("a full standard rendering is computable — Applicable must be true")
        }
        if answer.CheckDigit.Match || answer.CheckDigit.Computed != "X" || answer.CheckDigit.Expected != "4" {
                t.Fatalf("the mismatch must be pinned: expected %q, computed %q", answer.CheckDigit.Expected, answer.CheckDigit.Computed)
        }
        if len(answer.Words) != 1 || !strings.Contains(answer.Words[0], "контрольная цифра не совпала") || !strings.Contains(answer.Words[0], "сверьте посимвольно") {
                t.Fatalf("the mismatch word must be honest, got %v", answer.Words)
        }
}

// TestNormalizeVINLengthMismatch pins the length flag: a rendering of
// 15 runes carries LengthOK false and the honest word — the attribute
// never silently passes as complete.
func TestNormalizeVINLengthMismatch(t *testing.T) {
        answer := NormalizeVIN("WF0XXXTT4S12345")
        if answer.LengthOK || answer.Length != 15 {
                t.Fatalf("15 runes must flag the length, got %d", answer.Length)
        }
        if !strings.Contains(strings.Join(answer.Words, " "), "длина 15 ≠ 17") {
                t.Fatalf("the length word must be honest, got %v", answer.Words)
        }
        if answer.CheckDigit.Applicable {
                t.Fatal("a non-17 rendering is honestly not computable — no false verdict")
        }
}

// TestNormalizeVINForbiddenChars pins the I/O/Q flag: the forbidden
// letters are flagged by position with the honest word, the value is
// neither discarded nor replaced.
func TestNormalizeVINForbiddenChars(t *testing.T) {
        answer := NormalizeVIN("WFOQXTT4SI2345O78")
        if len(answer.ForbiddenChars) != 4 {
                t.Fatalf("the I, O and Q positions must be flagged, got %v", answer.ForbiddenChars)
        }
        if answer.Normalized != "WFOQXTT4SI2345O78" {
                t.Fatalf("the forbidden letters stay in the rendering (no silent fix), got %q", answer.Normalized)
        }
        joined := strings.Join(answer.Words, " ")
        if !strings.Contains(joined, "буквы I, O, Q в VIN не используются") || !strings.Contains(joined, "сверьте позицию") {
                t.Fatalf("the forbidden word must be honest, got %v", answer.Words)
        }
        if answer.CheckDigit.Applicable {
                t.Fatal("a rendering with forbidden letters is honestly not computable")
        }
}

// TestNormalizeVINCyrillicAmbiguityDetectedWithoutReplacement pins the
// homoglyph honesty: the cyrillic А among the latin letters is
// detected with its position and its look-alike, the rendering KEEPS
// the cyrillic rune (no silent replacement), the honest word asks for
// the сверка.
func TestNormalizeVINCyrillicAmbiguityDetectedWithoutReplacement(t *testing.T) {
        answer := NormalizeVIN("АF0XXXTT4S1234567")
        if len(answer.CyrillicAmbiguities) != 1 {
                t.Fatalf("one homoglyph expected, got %v", answer.CyrillicAmbiguities)
        }
        ambiguity := answer.CyrillicAmbiguities[0]
        if ambiguity.Position != 0 || ambiguity.Rune != "А" || ambiguity.LookAlike != "A" {
                t.Fatalf("the homoglyph must be pinned, got %+v", ambiguity)
        }
        if !strings.Contains(answer.Normalized, "А") {
                t.Fatalf("the cyrillic rune must stay in the rendering (never replaced), got %q", answer.Normalized)
        }
        joined := strings.Join(answer.Words, " ")
        if !strings.Contains(joined, "позиция 1: кириллица А (похожа на латиницу A) — требуется сверка") {
                t.Fatalf("the homoglyph word must be honest, got %v", answer.Words)
        }
        if answer.CheckDigit.Applicable {
                t.Fatal("a rendering with a homoglyph is honestly not computable")
        }
}

// TestReadbackDictionariesPinnedByName pins the three rows of the
// readback dictionary by name (AC-4: the latin 22, the digits 10, the
// cyrillic homoglyph 12 — the rows are distinguished BY WORDS).
func TestReadbackDictionariesPinnedByName(t *testing.T) {
        if len(VINReadbackLatin) != 22 {
                t.Fatalf("the latin row carries 22 words, got %d", len(VINReadbackLatin))
        }
        for r, word := range map[rune]string{
                'A': "Альфа", 'B': "Браво", 'C': "Цезарь", 'H': "Отель", 'J': "Джульетта",
                'K': "Кило", 'N': "Ноябрь", 'P': "Папа", 'R': "Ромео", 'S': "Сьерра",
                'W': "Виски", 'Y': "Янки",
        } {
                if VINReadbackLatin[r] != word {
                        t.Fatalf("latin word of %q must be %q", string(r), word)
                }
        }
        if len(VINReadbackDigits) != 10 {
                t.Fatalf("the digit row carries 10 words, got %d", len(VINReadbackDigits))
        }
        if VINReadbackDigits['0'] != "ноль" || VINReadbackDigits['9'] != "девять" {
                t.Fatalf("digit words must be pinned, got %q %q", VINReadbackDigits['0'], VINReadbackDigits['9'])
        }
        if len(VINReadbackCyrillic) != 12 {
                t.Fatalf("the cyrillic row carries the 12 homoglyph words, got %d", len(VINReadbackCyrillic))
        }
        if VINReadbackCyrillic['А'] != "Анна" || VINReadbackCyrillic['В'] != "Виктор" || VINReadbackCyrillic['Х'] != "Харитон" {
                t.Fatalf("cyrillic words must be pinned, got %q %q %q", VINReadbackCyrillic['А'], VINReadbackCyrillic['В'], VINReadbackCyrillic['Х'])
        }
}

// TestReadbackBlocksFourBlocks pins the block readback: the blocks
// 4-4-4-4-1 over the normalized rendering, every symbol with its row
// word (латиница | цифра) and its dictionary word — the text
// alternative of the voice readback exists always.
func TestReadbackBlocksFourBlocks(t *testing.T) {
        blocks := ReadbackBlocks("JH4KA7561PC008269")
        if len(blocks) != 5 {
                t.Fatalf("5 blocks (4-4-4-4-1) expected, got %d", len(blocks))
        }
        if blocks[0].Block != "JH4K" || blocks[4].Block != "9" {
                t.Fatalf("the block sizes must be 4-4-4-4-1, got %q … %q", blocks[0].Block, blocks[4].Block)
        }
        first := blocks[0].Symbols[0]
        if first.Row != "латиница" || first.Char != "J" || first.Word != "Джульетта" {
                t.Fatalf("the latin symbol must be row-distinguished, got %+v", first)
        }
        digit := blocks[0].Symbols[2]
        if digit.Row != "цифра" || digit.Char != "4" || digit.Word != "четыре" {
                t.Fatalf("the digit symbol must be row-distinguished, got %+v", digit)
        }
        cyrillicBlock := ReadbackBlocks("АF0XXXTT4S1234567")
        cyrillicSymbol := cyrillicBlock[0].Symbols[0]
        if cyrillicSymbol.Row != "кириллица" || cyrillicSymbol.Char != "А" || cyrillicSymbol.Word != "Анна" {
                t.Fatalf("the cyrillic symbol must be row-distinguished, got %+v", cyrillicSymbol)
        }
}

// TestReadbackFocusOnlyDisputed pins the focus re-ask: ONLY the
// disputed symbols are re-asked (with both row words — the client
// answers with the row distinction), then the honest «остальные
// символы приняты».
func TestReadbackFocusOnlyDisputed(t *testing.T) {
        normalization := NormalizeVIN("АF0XXXTT4S1234567")
        words := ReadbackFocus(normalization.Normalized, normalization.CyrillicAmbiguities)
        if len(words) != 2 {
                t.Fatalf("one disputed symbol + the accepted-rest word expected, got %v", words)
        }
        if !strings.Contains(words[0], "позиция 1: кириллица А («Анна») либо латиница A («Альфа»)?") {
                t.Fatalf("the focus word must name both row words, got %q", words[0])
        }
        if words[1] != "остальные символы приняты" {
                t.Fatalf("the accepted-rest word must be honest, got %q", words[1])
        }
}

// TestMaskedVINFormat pins the mask: the first three runes (the WMI) +
// «…» + the last four runes; a short value degrades honestly, the full
// length is never disclosed by the mask.
func TestMaskedVINFormat(t *testing.T) {
        if masked := MaskedVIN("WF0XXXTT4S1234567"); masked != "WF0…4567" {
                t.Fatalf("the mask must be WMI-3 + … + last-4, got %q", masked)
        }
        if masked := MaskedVIN("AB1"); masked != "A…1" {
                t.Fatalf("a 3-rune value degrades honestly, got %q", masked)
        }
        if masked := MaskedVIN(""); masked != "…" {
                t.Fatalf("the empty value degrades honestly, got %q", masked)
        }
}

// TestDeriveVehicleProfileCompletenessByAlternatives pins AC-1/AC-2:
// the completeness follows the alternative sets of the pack schema (any
// ONE set satisfies — vin | make+model+year), the missing keys carry
// only the not-confirmed attributes (the confirmed ones are never
// re-asked).
func TestDeriveVehicleProfileCompletenessByAlternatives(t *testing.T) {
        schema := vehicleTestSchema()
        // The vin-only world: one present unverified vin.
        profile := DeriveVehicleProfile(schema, []VehicleFactInput{vehicleFact("vehicle.vin", "WF0XXXTT4S1234567")})
        if len(profile.Completeness.SatisfiedBy) != 1 || profile.Completeness.SatisfiedBy[0] != "vehicle.vin" {
                t.Fatalf("the vin set is satisfied by presence, got %v", profile.Completeness.SatisfiedBy)
        }
        if len(profile.Completeness.MissingKeys) != 6 {
                t.Fatalf("everything unconfirmed is honestly missing, got %v", profile.Completeness.MissingKeys)
        }
        // The attributes world: make+model+year present and confirmed.
        facts := []VehicleFactInput{
                vehicleConfirm(vehicleFact("vehicle.make", "Ford"), 1, ConfirmerOperatorRelay),
                vehicleConfirm(vehicleFact("vehicle.model", "Focus"), 1, ConfirmerOperatorRelay),
                vehicleConfirm(vehicleFact("vehicle.year_of_manufacture", "2021"), 1, ConfirmerOperatorRelay),
        }
        profile = DeriveVehicleProfile(schema, facts)
        if len(profile.Completeness.SatisfiedBy) != 1 || profile.Completeness.SatisfiedBy[0] != "vehicle.make+vehicle.model+vehicle.year_of_manufacture" {
                t.Fatalf("the attribute set is satisfied, got %v", profile.Completeness.SatisfiedBy)
        }
        // The confirmed keys never enter MissingKeys (AC-2).
        joined := strings.Join(profile.Completeness.MissingKeys, ",")
        if strings.Contains(joined, "vehicle.make") || strings.Contains(joined, "vehicle.model") || strings.Contains(joined, "vehicle.year_of_manufacture") {
                t.Fatalf("the confirmed attributes are never re-asked, got %v", profile.Completeness.MissingKeys)
        }
        if !strings.Contains(joined, "vehicle.vin") {
                t.Fatalf("the absent vin stays honestly missing, got %v", profile.Completeness.MissingKeys)
        }
}

// TestDeriveVehicleProfileConfirmedAndDigest pins AC-12: the confirmed
// attribute carries the confirmation view (the confirmer, the EXACT
// version, the moment) and the ProfileDigest covers the confirmed set
// (the exact version of the profile).
func TestDeriveVehicleProfileConfirmedAndDigest(t *testing.T) {
        schema := vehicleTestSchema()
        fact := vehicleConfirm(vehicleFact("vehicle.vin", "JH4KA7561PC008269"), 1, "client")
        profile := DeriveVehicleProfile(schema, []VehicleFactInput{fact})
        attribute := profile.Attributes[0]
        if attribute.Status != FactStatusConfirmed {
                t.Fatalf("the confirmed version derives confirmed, got %q", attribute.Status)
        }
        if attribute.Confirmation == nil || attribute.Confirmation.ConfirmerType != "client" || attribute.Confirmation.VersionNo != 1 {
                t.Fatalf("the client confirmation must be pinned to the exact version, got %+v", attribute.Confirmation)
        }
        if attribute.Confirmation.StaleWord != "" {
                t.Fatalf("no stale word while the confirmed version is the latest, got %q", attribute.Confirmation.StaleWord)
        }
        if len(profile.ProfileDigest) != 64 {
                t.Fatalf("the ProfileDigest is the SHA-256 hex, got %q", profile.ProfileDigest)
        }
        if profile.StaleNote != "" {
                t.Fatalf("no stale note without a newer version, got %q", profile.StaleNote)
        }
        // The vin attribute carries the normalization view (only the vin kind).
        if attribute.NormalizedValue == nil || attribute.NormalizedValue.Normalized != "JH4KA7561PC008269" {
                t.Fatalf("the vin attribute carries the normalization view, got %+v", attribute.NormalizedValue)
        }
}

// TestDeriveVehicleProfileNewVersionAfterConfirmation pins AC-8: a new
// version of a confirmed attribute makes the attribute conflicting (the
// frozen 2.2 canon: the confirmed value and the new value disagree) with
// the stale word «подтверждение относилось к прежней версии (vN)», the
// ProfileDigest changes and the profile carries the aggregate stale
// note. (The spec text says unverified; the frozen DeriveFactStatus
// canon wins — the honest deviation of the slice, documented in the
// Spec Change Log.)
func TestDeriveVehicleProfileNewVersionAfterConfirmation(t *testing.T) {
        schema := vehicleTestSchema()
        fact := vehicleFact("vehicle.make", "Ford", "Ford Focus RS")
        fact = vehicleConfirm(fact, 1, "client")
        profile := DeriveVehicleProfile(schema, []VehicleFactInput{fact})
        attribute := profile.Attributes[0]
        if attribute.Status != FactStatusConflicting {
                t.Fatalf("the new version after the confirmation derives conflicting (2.2 canon), got %q", attribute.Status)
        }
        if attribute.Confirmation == nil || attribute.Confirmation.StaleWord != "подтверждение относилось к прежней версии (v1)" {
                t.Fatalf("the stale word must name the former version, got %+v", attribute.Confirmation)
        }
        if attribute.FactVersion != 2 {
                t.Fatalf("the current version is 2, got %d", attribute.FactVersion)
        }
        if profile.StaleNote != VehicleProfileStaleNote {
                t.Fatalf("the aggregate stale note must be present, got %q", profile.StaleNote)
        }
        // The digest of the confirmed set changed: the make left the set.
        fresh := DeriveVehicleProfile(schema, []VehicleFactInput{vehicleConfirm(vehicleFact("vehicle.make", "Ford"), 1, "client")})
        if profile.ProfileDigest == fresh.ProfileDigest {
                t.Fatal("the ProfileDigest must change when a confirmed attribute becomes stale")
        }
}

// TestDeriveVehicleProfileConflictingSources pins AC-7: two values of
// one attribute from different sources derive conflicting (read-time
// 2.2) — Staffy never picks the variant itself.
func TestDeriveVehicleProfileConflictingSources(t *testing.T) {
        schema := vehicleTestSchema()
        fact := vehicleFact("vehicle.make", "Ford", "Mazda")
        fact.Versions[1].SourceType = FactSourceOperator
        profile := DeriveVehicleProfile(schema, []VehicleFactInput{fact})
        attribute := profile.Attributes[0]
        if attribute.Status != FactStatusConflicting {
                t.Fatalf("two distinct values derive conflicting, got %q", attribute.Status)
        }
        joined := strings.Join(profile.Completeness.MissingKeys, ",")
        if !strings.Contains(joined, "vehicle.make") {
                t.Fatalf("the conflicting attribute is honestly not confirmed, got %v", profile.Completeness.MissingKeys)
        }
}

// TestDeriveVehicleProfileJSONIdempotent pins the read-time honesty:
// the repeated derivation over the same world renders byte-stable JSON
// (the idempotency of the output — the DerivePilotAdmission family).
func TestDeriveVehicleProfileJSONIdempotent(t *testing.T) {
        schema := vehicleTestSchema()
        facts := []VehicleFactInput{
                vehicleFact("vehicle.vin", "JH4KA7561PC008269"),
                vehicleConfirm(vehicleFact("vehicle.make", "Ford"), 1, "client"),
        }
        first, err := json.Marshal(DeriveVehicleProfile(schema, facts))
        if err != nil {
                t.Fatal(err)
        }
        second, err := json.Marshal(DeriveVehicleProfile(schema, facts))
        if err != nil {
                t.Fatal(err)
        }
        if string(first) != string(second) {
                t.Fatalf("the profile JSON must be byte-stable, got\n%s\n%s", first, second)
        }
        // The attributes keep the schema dictionary order.
        profile := DeriveVehicleProfile(schema, facts)
        if profile.Attributes[0].MaterialKey != "vehicle.vin" || profile.Attributes[1].MaterialKey != "vehicle.make" {
                t.Fatalf("the attributes follow the schema order, got %v %v", profile.Attributes[0].MaterialKey, profile.Attributes[1].MaterialKey)
        }
}

// TestDeriveVehicleProfileIgnoresNonSchemaFacts pins the pack-sourced
// schema honesty: the facts outside the schema dictionary never enter
// the profile (the dictionary comes from the pinned pack — never a
// hardcoded core list).
func TestDeriveVehicleProfileIgnoresNonSchemaFacts(t *testing.T) {
        schema := vehicleTestSchema()
        facts := []VehicleFactInput{
                vehicleFact("vehicle.vin", "JH4KA7561PC008269"),
                vehicleFact("part.articul", "AB123456"),
        }
        profile := DeriveVehicleProfile(schema, facts)
        for _, attribute := range profile.Attributes {
                if attribute.MaterialKey == "part.articul" {
                        t.Fatal("the non-schema fact must never enter the vehicle profile")
                }
        }
}

// TestVehicleMaterialKeysNeverHardcodedInCases pins the AC-1 probe: the
// vehicle material keys never appear as literals in the non-test
// sources of the cases domain — the schema is the pack material handed
// through the narrow taxonomy port, never a hardcoded core list.
func TestVehicleMaterialKeysNeverHardcodedInCases(t *testing.T) {
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
                for _, key := range []string{"vehicle.vin", "vehicle.make", "vehicle.model", "vehicle.year_of_manufacture", "vehicle.trim", "vehicle.engine"} {
                        if strings.Contains(string(raw), key) {
                                t.Fatalf("%s must not hardcode the material key %q — the schema comes from the pinned pack", entry.Name(), key)
                        }
                }
        }
}

// TestGuardVehicleClientConfirmation pins the client-path gate: the
// version bounds and the note bounds (the evidence of the client
// confirmation — the note carries the link reference).
func TestGuardVehicleClientConfirmation(t *testing.T) {
        if err := GuardVehicleClientConfirmation(2, 1, "ссылка продолжения …"); err != nil {
                t.Fatalf("the bounded confirmation must pass, got %v", err)
        }
        if err := GuardVehicleClientConfirmation(2, 3, "ссылка"); err == nil {
                t.Fatal("the version beyond the count must refuse")
        }
        if err := GuardVehicleClientConfirmation(2, 1, "  "); err == nil {
                t.Fatal("the empty note must refuse")
        }
        if err := GuardVehicleClientConfirmation(2, 1, strings.Repeat("а", MaxNoteRunes+1)); err == nil {
                t.Fatal("the over-long note must refuse")
        }
}
