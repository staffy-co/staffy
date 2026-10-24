package domain

import (
        "encoding/json"
        "os"
        "strings"
        "testing"
        "time"
)

// The Story 4.5 unit matrix: the state ladder by name (unknown / current /
// stale-age / stale-pins ×3 / supersede by number / change words /
// shortfall), the ranking by name (budget / brands / term with the
// explanation words, the no-criteria question with the limited set), the
// grouping by the link type, the voice block, the guard refusals by name
// (the offer family and the selection family), the digest byte-stability,
// the JSON idempotency ×2 and the honesty probes (the hardcoded literals,
// the purchase price OUT of the client material, the marketing words OUT
// of the production file).

func comparisonPolicyFixture() ComparisonPolicyDomain {
        return ComparisonPolicyDomain{
                Currencies:         []string{OfferCurrencyRUB},
                MaxOfferAgeSeconds: 86400,
                RankCriteria:       []string{OfferRankBudget, OfferRankBrand, OfferRankTerm},
                RankCriteriaLabels: map[string]string{OfferRankBudget: "Бюджет", OfferRankBrand: "Бренд", OfferRankTerm: "Срок"},
                TieBreakers:        []string{OfferTiePrice, OfferTieTerm, OfferTieBrand, OfferTieSupplier},
                MaxVisibleVariants: 4,
        }
}

func offerDraftFixture() SupplierOfferDraft {
        purchase := 3900.0
        term := 2
        return SupplierOfferDraft{
                PositionKey:          "WF2B196K666AA",
                SupplierID:           "SUP-1",
                SupplierName:         "Поставщик один",
                Brand:                "Bosch",
                Article:              "WF2B196K666AA",
                ClientPrice:          4850,
                PurchasePrice:        &purchase,
                Currency:             OfferCurrencyRUB,
                QuantityAvailable:    4,
                TermDays:             &term,
                Conditions:           "самовывоз со склада",
                SourceSystem:         OfferSourceAbcpCatalog,
                SourceRecordID:       "rec-1",
                SourceVersion:        "v2026.10",
                SourceObservedAt:     time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC),
                ProfileDigestPin:     strings.Repeat("v", 64),
                PartRequestDigestPin: strings.Repeat("p", 64),
        }
}

func offerFixture() SupplierOfferInput {
        purchase := 3900.0
        term := 2
        return SupplierOfferInput{
                OfferNo:              1,
                PositionKey:          "WF2B196K666AA",
                SupplierID:           "SUP-1",
                SupplierName:         "Поставщик один",
                Brand:                "Bosch",
                Article:              "WF2B196K666AA",
                ClientPrice:          4850,
                PurchasePrice:        &purchase,
                Currency:             OfferCurrencyRUB,
                QuantityAvailable:    4,
                TermDays:             &term,
                Conditions:           "самовывоз со склада",
                SourceSystem:         OfferSourceAbcpCatalog,
                SourceRecordID:       "rec-1",
                SourceVersion:        "v2026.10",
                SourceObservedAt:     time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC),
                ReceivedAt:           time.Date(2026, 10, 24, 10, 5, 0, 0, time.UTC),
                ProfileDigestPin:     strings.Repeat("v", 64),
                PartRequestDigestPin: strings.Repeat("p", 64),
                FitmentDigestPin:     strings.Repeat("f", 64),
        }
}

func comparisonInputFixture() ComparisonDerivationInput {
        return ComparisonDerivationInput{
                Policy:            comparisonPolicyFixture(),
                ProfileDigest:     strings.Repeat("v", 64),
                PartRequestDigest: strings.Repeat("p", 64),
                Now:               time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC),
        }
}

func provenPositionFixture(offers ...SupplierOfferInput) ComparisonPositionInput {
        return ComparisonPositionInput{
                PositionKey:       offers[0].PositionKey,
                LinkType:          FitmentLinkOriginal,
                FitmentStatus:     FitmentStatusProven,
                FitmentStatusWord: FitmentProvenWord,
                FitmentDigest:     strings.Repeat("f", 64),
                Offers:            offers,
        }
}

// TestOfferUnknownWithoutRecords: NO offers at all — the case-level honest
// unknown word + the research recommendation (AC-12: the old price never
// repeats as the current one).
func TestOfferUnknownWithoutRecords(t *testing.T) {
        view := DeriveComparison(comparisonInputFixture())
        if view.CaseUnknownWord != OfferUnknownWord {
                t.Fatalf("case unknown word = %q", view.CaseUnknownWord)
        }
        if len(view.CaseHandoffWords) != 1 || view.CaseHandoffWords[0] != OfferResearchWord {
                t.Fatalf("case handoff words = %v", view.CaseHandoffWords)
        }
        if len(view.Positions) != 0 {
                t.Fatalf("positions = %d, want 0", len(view.Positions))
        }
}

// TestOfferPositionUnknownWithoutOffers: the position WITHOUT offers (the
// proven fitment of 4.4, no commercial conditions yet) — the honest
// position-level unknown + the research recommendation (the fitment panel
// status stays untouched — AC-1); the case-level unknown fires only when
// NO offer exists at all (the I/O row 41 contract).
func TestOfferPositionUnknownWithoutOffers(t *testing.T) {
        input := comparisonInputFixture()
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offerFixture())}
        input.Positions[0].Offers = nil
        // The case-level unknown word when NO offer exists at all.
        if view := DeriveComparison(input); len(view.Positions) != 0 || view.CaseUnknownWord != OfferUnknownWord {
                t.Fatalf("the empty case is not the case-level unknown: %d positions / %q", len(view.Positions), view.CaseUnknownWord)
        }
        // The mixed case: the no-offer position renders its own honest unknown.
        other := offerFixture()
        other.PositionKey = "K1234X"
        second := ComparisonPositionInput{PositionKey: "K1234X", LinkType: FitmentLinkCross, FitmentStatus: FitmentStatusProven, FitmentStatusWord: FitmentProvenWord, FitmentDigest: strings.Repeat("f", 64), Offers: []SupplierOfferInput{other}}
        input.Positions = append(input.Positions, second)
        view := DeriveComparison(input)
        if view.CaseUnknownWord != "" {
                t.Fatalf("the case unknown word is set in the mixed case: %q", view.CaseUnknownWord)
        }
        if len(view.Positions) != 2 || view.Positions[0].Status != OfferStatusUnknown {
                t.Fatalf("position status = %q, want unknown", view.Positions[0].Status)
        }
        if view.Positions[0].StatusWord != OfferUnknownWord {
                t.Fatalf("position status word = %q", view.Positions[0].StatusWord)
        }
        if len(view.Positions[0].HandoffWords) != 1 || view.Positions[0].HandoffWords[0] != OfferResearchWord {
                t.Fatalf("handoff words = %v", view.Positions[0].HandoffWords)
        }
        if view.Positions[0].FitmentStatus != FitmentStatusProven {
                t.Fatalf("the fitment status is not re-shown: %q", view.Positions[0].FitmentStatus)
        }
}

// TestOfferCurrentLiveRecord: the live current offer — the current status
// word, the price authority word and the term estimate word on the case
// level, the variant with the byte-stable OfferDigest.
func TestOfferCurrentLiveRecord(t *testing.T) {
        input := comparisonInputFixture()
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offerFixture())}
        view := DeriveComparison(input)
        if view.CaseUnknownWord != "" {
                t.Fatalf("the case unknown word is set: %q", view.CaseUnknownWord)
        }
        if view.PriceAuthorityWord != OfferPriceAuthorityWord {
                t.Fatalf("price authority word = %q", view.PriceAuthorityWord)
        }
        if view.TermEstimateWord != OfferTermEstimateWord {
                t.Fatalf("term estimate word = %q", view.TermEstimateWord)
        }
        position := view.Positions[0]
        if position.Status != OfferStatusCurrent || position.StatusWord != OfferCurrentWord {
                t.Fatalf("position status = %q / %q", position.Status, position.StatusWord)
        }
        if len(position.Variants) != 1 {
                t.Fatalf("variants = %d, want 1", len(position.Variants))
        }
        variant := position.Variants[0]
        if variant.Status != OfferStatusCurrent {
                t.Fatalf("variant status = %q", variant.Status)
        }
        if variant.OfferDigest != OfferDigestOf(offerFixture()) {
                t.Fatalf("the offer digest is not the canonical one")
        }
        if variant.TermWord != OfferTermEstimateWord {
                t.Fatalf("term word = %q", variant.TermWord)
        }
}

// TestOfferStaleByAge: the age over the pack TTL — the stale position word
// + the per-offer stale age word (AC-9).
func TestOfferStaleByAge(t *testing.T) {
        input := comparisonInputFixture()
        input.Now = offerFixture().ReceivedAt.Add(time.Duration(comparisonPolicyFixture().MaxOfferAgeSeconds)*time.Second + time.Second)
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offerFixture())}
        view := DeriveComparison(input)
        position := view.Positions[0]
        if position.Status != OfferStatusStale {
                t.Fatalf("position status = %q, want stale", position.Status)
        }
        if position.StatusWord != OfferStalePositionWord {
                t.Fatalf("position status word = %q", position.StatusWord)
        }
        if len(position.Variants) != 1 || len(position.Variants[0].StaleWords) != 1 {
                t.Fatalf("the stale words of the expired offer = %v", position.Variants[0].StaleWords)
        }
        if position.Variants[0].StaleWords[0] != OfferStaleAgeWord {
                t.Fatalf("stale word = %q", position.Variants[0].StaleWords[0])
        }
        if position.Variants[0].Rank != 0 {
                t.Fatalf("the expired offer is ranked: %d", position.Variants[0].Rank)
        }
}

// TestOfferStaleByProfilePin: the profile pin mismatch — the separate
// honest word (AC-10: a new version of the vehicle profile expires the
// dependent offer).
func TestOfferStaleByProfilePin(t *testing.T) {
        input := comparisonInputFixture()
        input.ProfileDigest = strings.Repeat("x", 64)
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offerFixture())}
        view := DeriveComparison(input)
        if words := view.Positions[0].Variants[0].StaleWords; len(words) != 1 || words[0] != OfferStaleProfileWord {
                t.Fatalf("stale words = %v, want the profile pin word", words)
        }
}

// TestOfferStaleByPartRequestPin: the request pin mismatch — the separate
// honest word (AC-10).
func TestOfferStaleByPartRequestPin(t *testing.T) {
        input := comparisonInputFixture()
        input.PartRequestDigest = strings.Repeat("x", 64)
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offerFixture())}
        view := DeriveComparison(input)
        if words := view.Positions[0].Variants[0].StaleWords; len(words) != 1 || words[0] != OfferStalePartRequestWord {
                t.Fatalf("stale words = %v, want the request pin word", words)
        }
}

// TestOfferStaleByFitmentPin: the fitment pin mismatch — the separate
// honest word (AC-10: a new version of the proof expires the dependent
// offer).
func TestOfferStaleByFitmentPin(t *testing.T) {
        input := comparisonInputFixture()
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offerFixture())}
        input.Positions[0].FitmentDigest = strings.Repeat("x", 64)
        view := DeriveComparison(input)
        if words := view.Positions[0].Variants[0].StaleWords; len(words) != 1 || words[0] != OfferStaleFitmentWord {
                t.Fatalf("stale words = %v, want the fitment pin word", words)
        }
}

// TestOfferSupersedeByNewOfferNumber: the NEW record of the same supplier
// supersedes the old one BY THE NUMBER (never an update — the old record
// stays the numbered material, the current conditions are the maximal
// number's ones, AC-10/AC-12).
func TestOfferSupersedeByNewOfferNumber(t *testing.T) {
        older := offerFixture()
        newer := offerFixture()
        newer.OfferNo = 2
        newer.ClientPrice = 5100
        input := comparisonInputFixture()
        input.Positions = []ComparisonPositionInput{provenPositionFixture(older, newer)}
        view := DeriveComparison(input)
        position := view.Positions[0]
        if position.Status != OfferStatusCurrent {
                t.Fatalf("position status = %q", position.Status)
        }
        if len(position.Variants) != 1 {
                t.Fatalf("variants = %d, want the ONE current record of the supplier", len(position.Variants))
        }
        if position.Variants[0].OfferNo != 2 || position.Variants[0].ClientPrice != 5100 {
                t.Fatalf("the current record is not the maximal number's one: no=%d price=%.0f", position.Variants[0].OfferNo, position.Variants[0].ClientPrice)
        }
}

// TestOfferChangeWords: the changed conditions between the consecutive
// records of one supplier — the honest change word naming the changed
// aspects (AC-10).
func TestOfferChangeWords(t *testing.T) {
        older := offerFixture()
        newer := offerFixture()
        newer.OfferNo = 2
        newer.ClientPrice = 5100
        newer.QuantityAvailable = 3
        input := comparisonInputFixture()
        input.Positions = []ComparisonPositionInput{provenPositionFixture(older, newer)}
        view := DeriveComparison(input)
        words := view.Positions[0].Variants[0].ChangeWords
        if len(words) != 1 {
                t.Fatalf("change words = %v", words)
        }
        if !strings.Contains(words[0], "цена") || !strings.Contains(words[0], "наличие") || !strings.Contains(words[0], "требуется новая точная версия подтверждения") {
                t.Fatalf("the change word does not name the changed aspects: %q", words[0])
        }
        if strings.Contains(words[0], "срок") {
                t.Fatalf("the unchanged term is named as changed: %q", words[0])
        }
}

// TestOfferRankByBudget: the confirmed budget criterion — the price
// ascending order with the explanation word (AC-6); the order is
// deterministic across the repeated derivations.
func TestOfferRankByBudget(t *testing.T) {
        cheaper := offerFixture()
        cheaper.SupplierID, cheaper.SupplierName, cheaper.Brand = "SUP-2", "Поставщик два", "Mann"
        cheaper.ClientPrice = 4200
        pricier := offerFixture()
        input := comparisonInputFixture()
        input.Criteria = ComparisonCriteria{BudgetConfirmed: true}
        input.Positions = []ComparisonPositionInput{provenPositionFixture(pricier, cheaper)}
        view := DeriveComparison(input)
        variants := view.Positions[0].Variants
        if variants[0].SupplierID != "SUP-2" || variants[0].Rank != 1 {
                t.Fatalf("the cheaper variant is not the first: %v", variants[0].SupplierID)
        }
        if variants[0].RankWord != OfferRankBudgetWord {
                t.Fatalf("rank word = %q", variants[0].RankWord)
        }
        if variants[1].Rank != 2 || variants[1].RankWord != OfferRankBudgetWord {
                t.Fatalf("the second variant rank/word = %d / %q", variants[1].Rank, variants[1].RankWord)
        }
        repeated := DeriveComparison(input)
        if repeated.Positions[0].Variants[0].SupplierID != "SUP-2" {
                t.Fatalf("the ranking is not deterministic")
        }
}

// TestOfferRankByBrands: the confirmed brand criterion — the listed brands
// above the unlisted ones with the explanation word on the listed ones
// (AC-6: the hidden supplier benefit is never the order basis).
func TestOfferRankByBrands(t *testing.T) {
        listed := offerFixture()
        unlisted := offerFixture()
        unlisted.SupplierID, unlisted.SupplierName, unlisted.Brand = "SUP-2", "Поставщик два", "Febi"
        unlisted.ClientPrice = 1000
        input := comparisonInputFixture()
        input.Criteria = ComparisonCriteria{BrandsConfirmed: true, Brands: []string{"bosch", "Mann"}}
        // The unlisted variant is CHEAPER — the confirmed brand criterion wins,
        // the price never reorders the primary criterion.
        input.Positions = []ComparisonPositionInput{provenPositionFixture(listed, unlisted)}
        view := DeriveComparison(input)
        variants := view.Positions[0].Variants
        if variants[0].SupplierID != "SUP-1" || variants[0].Rank != 1 {
                t.Fatalf("the listed brand is not the first: %q", variants[0].SupplierID)
        }
        if variants[0].RankWord != OfferRankBrandWord {
                t.Fatalf("rank word = %q", variants[0].RankWord)
        }
        if variants[1].RankWord != "" {
                t.Fatalf("the unlisted variant carries the criterion word: %q", variants[1].RankWord)
        }
}

// TestOfferRankByTerm: the confirmed term criterion — the estimate
// ascending order; the absent term is the LAST with the honest no-term
// word (never the fastest one, AC-6).
func TestOfferRankByTerm(t *testing.T) {
        faster := offerFixture()
        term := 1
        faster.TermDays = &term
        slower := offerFixture()
        slower.SupplierID, slower.SupplierName, slower.Brand = "SUP-2", "Поставщик два", "Mann"
        term2 := 5
        slower.TermDays = &term2
        noTerm := offerFixture()
        noTerm.SupplierID, noTerm.SupplierName, noTerm.Brand = "SUP-3", "Поставщик три", "Febi"
        noTerm.TermDays = nil
        noTerm.ClientPrice = 1
        input := comparisonInputFixture()
        input.Criteria = ComparisonCriteria{TermConfirmed: true}
        input.Positions = []ComparisonPositionInput{provenPositionFixture(slower, noTerm, faster)}
        view := DeriveComparison(input)
        variants := view.Positions[0].Variants
        if variants[0].SupplierID != "SUP-1" || variants[1].SupplierID != "SUP-2" || variants[2].SupplierID != "SUP-3" {
                t.Fatalf("the term order is wrong: %q %q %q", variants[0].SupplierID, variants[1].SupplierID, variants[2].SupplierID)
        }
        if variants[0].RankWord != OfferRankTermWord {
                t.Fatalf("rank word = %q", variants[0].RankWord)
        }
        if variants[2].TermWord != OfferNoTermWord {
                t.Fatalf("the absent term word = %q", variants[2].TermWord)
        }
}

// TestOfferNoCriteriaQuestionAndLimitedSet: NO confirmed criterion and
// more variants than the pack bound — the honest clarifying question + the
// limited set word + the truncation to MaxVisibleVariants (AC-7: never the
// «best» word without a defined basis; never a silent truncation).
func TestOfferNoCriteriaQuestionAndLimitedSet(t *testing.T) {
        offers := make([]SupplierOfferInput, 0, 6)
        for index := 1; index <= 6; index++ {
                offer := offerFixture()
                offer.SupplierID = "SUP-" + string(rune('0'+index))
                offer.SupplierName = "Поставщик " + string(rune('0'+index))
                offer.Brand = "Brand" + string(rune('0'+index))
                offer.ClientPrice = float64(1000 * index)
                offers = append(offers, offer)
        }
        input := comparisonInputFixture()
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offers...)}
        view := DeriveComparison(input)
        position := view.Positions[0]
        if position.Question != OfferNoCriteriaQuestion {
                t.Fatalf("question = %q", position.Question)
        }
        if len(position.Variants) != comparisonPolicyFixture().MaxVisibleVariants {
                t.Fatalf("variants = %d, want the limited set of %d", len(position.Variants), comparisonPolicyFixture().MaxVisibleVariants)
        }
        limited := false
        for _, word := range position.Words {
                if strings.Contains(word, "ограниченный набор из 4 вариантов (всего 6)") {
                        limited = true
                }
        }
        if !limited {
                t.Fatalf("the limited set word is absent: %v", position.Words)
        }
        for _, variant := range position.Variants {
                if variant.Rank != 0 || variant.RankWord != "" {
                        t.Fatalf("the no-criteria variant carries a rank: %d / %q", variant.Rank, variant.RankWord)
                }
        }
}

// TestOfferShortfallWord: the confirmed requested quantity over the
// availability — the honest shortfall word (AC-11: the split and the
// silent quantity change never happen).
func TestOfferShortfallWord(t *testing.T) {
        input := comparisonInputFixture()
        input.Criteria = ComparisonCriteria{RequestedQty: 4}
        offer := offerFixture()
        offer.QuantityAvailable = 2
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offer)}
        view := DeriveComparison(input)
        if word := view.Positions[0].Variants[0].ShortfallWord; word != "у этого поставщика меньше запрошенного: 2 из 4" {
                t.Fatalf("shortfall word = %q", word)
        }
        // The UNCONFIRMED quantity never renders the shortfall word (the honest
        // absence — the ranking material stays the confirmed one).
        unconfirmed := comparisonInputFixture()
        unconfirmed.Positions = []ComparisonPositionInput{provenPositionFixture(offer)}
        if word := DeriveComparison(unconfirmed).Positions[0].Variants[0].ShortfallWord; word != "" {
                t.Fatalf("the unconfirmed quantity renders the shortfall word: %q", word)
        }
}

// TestOfferGroupingOriginalCross: the two positions of the different link
// types — the group words «оригинал» and «аналог по cross-цепочке» with
// the confirmed fitment words (AC-5).
func TestOfferGroupingOriginalCross(t *testing.T) {
        original := provenPositionFixture(offerFixture())
        crossPosition := offerFixture()
        crossPosition.PositionKey = "K1234X"
        crossPosition.SupplierID, crossPosition.SupplierName, crossPosition.Brand = "SUP-2", "Поставщик два", "Mann"
        cross := ComparisonPositionInput{
                PositionKey:       "K1234X",
                LinkType:          FitmentLinkCross,
                FitmentStatus:     FitmentStatusProven,
                FitmentStatusWord: FitmentProvenWord,
                FitmentDigest:     strings.Repeat("f", 64),
                Offers:            []SupplierOfferInput{crossPosition},
        }
        input := comparisonInputFixture()
        input.Positions = []ComparisonPositionInput{cross, original}
        view := DeriveComparison(input)
        if view.Positions[0].LinkTypeWord != "аналог по cross-цепочке" {
                t.Fatalf("cross group word = %q", view.Positions[0].LinkTypeWord)
        }
        if view.Positions[1].LinkTypeWord != "оригинал" {
                t.Fatalf("original group word = %q", view.Positions[1].LinkTypeWord)
        }
        if view.Positions[0].FitmentStatusWord != FitmentProvenWord {
                t.Fatalf("the fitment word is not re-shown: %q", view.Positions[0].FitmentStatusWord)
        }
}

// TestOfferVoiceBlock: the phone block — the key differences by the
// CONFIRMED criteria only, the recommended variant word ONLY on the
// confirmed criterion and the full comparison word (AC-14).
func TestOfferVoiceBlock(t *testing.T) {
        cheaper := offerFixture()
        cheaper.SupplierID, cheaper.SupplierName, cheaper.Brand = "SUP-2", "Поставщик два", "Mann"
        cheaper.ClientPrice = 4200
        input := comparisonInputFixture()
        input.Criteria = ComparisonCriteria{BudgetConfirmed: true}
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offerFixture(), cheaper)}
        view := DeriveComparison(input)
        block := view.VoiceBlock
        if block.FullWord != OfferVoiceFullWord {
                t.Fatalf("full word = %q", block.FullWord)
        }
        if len(block.KeyDifferences) != 1 || !strings.HasPrefix(block.KeyDifferences[0], "цена: от 4200 до 4850") {
                t.Fatalf("key differences = %v", block.KeyDifferences)
        }
        if !strings.Contains(block.RecommendedWord, "по подтверждённому приоритету «Бюджет»") || !strings.Contains(block.RecommendedWord, "Поставщик два") {
                t.Fatalf("recommended word = %q", block.RecommendedWord)
        }
        if strings.Contains(strings.ToLower(block.RecommendedWord), "лучший") {
                t.Fatalf("the «best» word is spoken: %q", block.RecommendedWord)
        }
        // The NO-criteria case: no key differences, no recommended word — the
        // honest absence (AC-7).
        noCriteria := comparisonInputFixture()
        noCriteria.Positions = input.Positions
        bare := DeriveComparison(noCriteria).VoiceBlock
        if len(bare.KeyDifferences) != 0 || bare.RecommendedWord != "" {
                t.Fatalf("the no-criteria voice block is not honest: %v / %q", bare.KeyDifferences, bare.RecommendedWord)
        }
}

// TestGuardSupplierOfferRefusals: the dishonest offer records are refused
// BEFORE any SQL with the honest dictionary words — by name.
func TestGuardSupplierOfferRefusals(t *testing.T) {
        policy := comparisonPolicyFixture()
        cases := []struct {
                name string
                mut  func(*SupplierOfferDraft)
                rule string
                word string
        }{
                {"empty position key", func(d *SupplierOfferDraft) { d.PositionKey = "" }, "offer_position_key_required", "ключ позиции"},
                {"foreign currency", func(d *SupplierOfferDraft) { d.Currency = "USD" }, "offer_currency_out_of_dictionary", "Domain Policy пака"},
                {"foreign source", func(d *SupplierOfferDraft) { d.SourceSystem = "web_search" }, "offer_source_out_of_dictionary", "Domain Policy пака"},
                {"non-canonical position key", func(d *SupplierOfferDraft) { d.PositionKey = "wf 2b19 6k666 aa" }, "offer_position_key_not_canonical", "канонизируйте"},
                {"no supplier", func(d *SupplierOfferDraft) { d.SupplierID = "" }, "offer_supplier_required", "поставщика"},
                {"no brand", func(d *SupplierOfferDraft) { d.Brand = "" }, "offer_brand_required", "бренд"},
                {"no article", func(d *SupplierOfferDraft) { d.Article = "" }, "offer_article_required", "артикул"},
                {"negative client price", func(d *SupplierOfferDraft) { d.ClientPrice = -1 }, "offer_client_price_negative", "модель не рассчитывает"},
                {"negative purchase price", func(d *SupplierOfferDraft) { d.PurchasePrice = &[]float64{-1}[0] }, "offer_purchase_price_negative", "внутренние данные"},
                {"negative quantity", func(d *SupplierOfferDraft) { d.QuantityAvailable = -1 }, "offer_quantity_negative", "отрицательным"},
                {"negative term", func(d *SupplierOfferDraft) { d.TermDays = &[]int{-1}[0] }, "offer_term_negative", "отрицательным"},
                {"no source record id", func(d *SupplierOfferDraft) { d.SourceRecordID = "" }, "offer_source_record_id_required", "трассируемость"},
                {"no source version", func(d *SupplierOfferDraft) { d.SourceVersion = "" }, "offer_source_version_required", "версия источника"},
                {"no observed time", func(d *SupplierOfferDraft) { d.SourceObservedAt = time.Time{} }, "offer_source_observed_at_required", "время получения"},
                {"no profile pin", func(d *SupplierOfferDraft) { d.ProfileDigestPin = "" }, "offer_profile_digest_pin_required", "ProfileDigest"},
                {"no request pin", func(d *SupplierOfferDraft) { d.PartRequestDigestPin = "" }, "offer_part_request_digest_pin_required", "PartRequestDigest"},
        }
        for _, testCase := range cases {
                draft := offerDraftFixture()
                testCase.mut(&draft)
                err := GuardSupplierOffer(policy, draft)
                if err == nil {
                        t.Fatalf("%s: the dishonest record is accepted", testCase.name)
                }
                rule, ok := err.(WorkRuleError)
                if !ok {
                        t.Fatalf("%s: the refusal is not a WorkRuleError: %v", testCase.name, err)
                }
                if rule.Rule != testCase.rule {
                        t.Fatalf("%s: rule = %q, want %q", testCase.name, rule.Rule, testCase.rule)
                }
                if !strings.Contains(rule.Reason, testCase.word) {
                        t.Fatalf("%s: the honest word %q is absent in %q", testCase.name, testCase.word, rule.Reason)
                }
        }
}

// TestGuardSupplierOfferAccepts: the honest offer record passes — with the
// optional purchase price (the internal data) and without it (the honest
// «не видел / не записал», never a zero), with the term and without it.
func TestGuardSupplierOfferAccepts(t *testing.T) {
        policy := comparisonPolicyFixture()
        if err := GuardSupplierOffer(policy, offerDraftFixture()); err != nil {
                t.Fatalf("the honest record with the purchase price is refused: %v", err)
        }
        withoutPurchase := offerDraftFixture()
        withoutPurchase.PurchasePrice = nil
        if err := GuardSupplierOffer(policy, withoutPurchase); err != nil {
                t.Fatalf("the honest record without the purchase price is refused: %v", err)
        }
        withoutTerm := offerDraftFixture()
        withoutTerm.TermDays = nil
        if err := GuardSupplierOffer(policy, withoutTerm); err != nil {
                t.Fatalf("the honest record without the term is refused: %v", err)
        }
        withoutFitmentPin := offerDraftFixture()
        withoutFitmentPin.FitmentDigestPin = ""
        if err := GuardSupplierOffer(policy, withoutFitmentPin); err != nil {
                t.Fatalf("the honest record without the optional fitment pin is refused: %v", err)
        }
}

// TestGuardClientSelectionRefusals: the dishonest selections are refused
// BEFORE any SQL with the honest words — by name (the non-proven position,
// the expired offer by every pin and by the age, the quantity bounds).
func TestGuardClientSelectionRefusals(t *testing.T) {
        policy := comparisonPolicyFixture()
        draftFixture := func() ClientSelectionDraft {
                return ClientSelectionDraft{
                        PositionKey:       "WF2B196K666AA",
                        Offer:             offerFixture(),
                        Quantity:          2,
                        FitmentStatus:     FitmentStatusProven,
                        FitmentDigest:     strings.Repeat("f", 64),
                        ProfileDigest:     strings.Repeat("v", 64),
                        PartRequestDigest: strings.Repeat("p", 64),
                        Now:               time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC),
                }
        }
        cases := []struct {
                name string
                mut  func(*ClientSelectionDraft)
                rule string
                word string
        }{
                {"non-proven fitment", func(d *ClientSelectionDraft) { d.FitmentStatus = FitmentStatusUnknown }, "selection_fitment_not_proven", OfferSelectionGuardFitmentWord},
                {"stale by profile pin", func(d *ClientSelectionDraft) { d.ProfileDigest = strings.Repeat("x", 64) }, "selection_offer_stale", OfferSelectionGuardStaleWord},
                {"stale by request pin", func(d *ClientSelectionDraft) { d.PartRequestDigest = strings.Repeat("x", 64) }, "selection_offer_stale", OfferSelectionGuardStaleWord},
                {"stale by fitment pin", func(d *ClientSelectionDraft) { d.Offer.FitmentDigestPin = strings.Repeat("x", 64) }, "selection_offer_stale", OfferSelectionGuardStaleWord},
                {"stale by age", func(d *ClientSelectionDraft) { d.Now = offerFixture().ReceivedAt.Add(time.Duration(policy.MaxOfferAgeSeconds)*time.Second + time.Second) }, "selection_offer_stale", OfferSelectionGuardStaleWord},
                {"zero quantity", func(d *ClientSelectionDraft) { d.Quantity = 0 }, "selection_quantity_min", "не меньше единицы"},
                {"over availability", func(d *ClientSelectionDraft) { d.Quantity = 5 }, "selection_quantity_over_availability", "разбие́ние заказа"},
        }
        for _, testCase := range cases {
                draft := draftFixture()
                testCase.mut(&draft)
                err := GuardClientSelection(policy, draft)
                if err == nil {
                        t.Fatalf("%s: the dishonest selection is accepted", testCase.name)
                }
                rule, ok := err.(WorkRuleError)
                if !ok {
                        t.Fatalf("%s: the refusal is not a WorkRuleError: %v", testCase.name, err)
                }
                if rule.Rule != testCase.rule {
                        t.Fatalf("%s: rule = %q, want %q", testCase.name, rule.Rule, testCase.rule)
                }
                if !strings.Contains(rule.Reason, testCase.word) {
                        t.Fatalf("%s: the honest word %q is absent in %q", testCase.name, testCase.word, rule.Reason)
                }
        }
}

// TestAssembleClientSelection: the honest selection assembles the snapshot
// with the pins of ALL FOUR inputs (AC-13), the known client sum as the
// ONLY arithmetic of the slice (2 × 4850 = 9700) and the byte-stable
// SelectionDigest.
func TestAssembleClientSelection(t *testing.T) {
        policy := comparisonPolicyFixture()
        draft := ClientSelectionDraft{
                PositionKey:       "WF2B196K666AA",
                Offer:             offerFixture(),
                Quantity:          2,
                FitmentStatus:     FitmentStatusProven,
                FitmentDigest:     strings.Repeat("f", 64),
                ProfileDigest:     strings.Repeat("v", 64),
                PartRequestDigest: strings.Repeat("p", 64),
                Now:               time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC),
        }
        material, err := AssembleClientSelection(policy, draft)
        if err != nil {
                t.Fatalf("the honest selection is refused: %v", err)
        }
        if material.KnownClientSum != 9700 {
                t.Fatalf("known client sum = %.0f, want 9700", material.KnownClientSum)
        }
        if material.ProfileDigestPin != strings.Repeat("v", 64) || material.PartRequestDigestPin != strings.Repeat("p", 64) || material.FitmentDigestPin != strings.Repeat("f", 64) || material.OfferDigestPin != OfferDigestOf(offerFixture()) {
                t.Fatalf("the snapshot does not pin all four inputs")
        }
        if material.SelectionDigest != SelectionDigestOf(material) {
                t.Fatalf("the selection digest is not canonical")
        }
}

// TestOfferDigestCanonical: the digest is the canonical SHA-256 of the
// conditions record — the same material yields the same digest, the
// changed price yields a different one, the changed PURCHASE price yields
// the SAME one (the internal data never changes the client-relevant
// version).
func TestOfferDigestCanonical(t *testing.T) {
        offer := offerFixture()
        if OfferDigestOf(offer) != OfferDigestOf(offer) {
                t.Fatalf("the digest is not deterministic")
        }
        changed := offer
        changed.ClientPrice = 5100
        if OfferDigestOf(offer) == OfferDigestOf(changed) {
                t.Fatalf("the different price yields the same digest")
        }
        internal := offer
        other := 5000.0
        internal.PurchasePrice = &other
        if OfferDigestOf(offer) != OfferDigestOf(internal) {
                t.Fatalf("the purchase price change yields a different client-relevant digest")
        }
}

// TestComparisonJSONIdempotency: the derivation is byte-stable — the JSON
// of the same world is identical across the repeated marshals (×2).
func TestComparisonJSONIdempotency(t *testing.T) {
        input := comparisonInputFixture()
        input.Criteria = ComparisonCriteria{BudgetConfirmed: true, RequestedQty: 4}
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offerFixture())}
        first, err := json.Marshal(DeriveComparison(input))
        if err != nil {
                t.Fatalf("marshal: %v", err)
        }
        repeated, err := json.Marshal(DeriveComparison(input))
        if err != nil {
                t.Fatalf("marshal: %v", err)
        }
        if string(first) != string(repeated) {
                t.Fatalf("the derivation JSON is not byte-stable")
        }
}

// TestOfferHardcodedProbe: the production file carries NO hardcoded store
// literals — the dictionary words come from the pack policy, the position,
// the brand, the supplier and the conditions literals stay out of the
// production files (the test fixtures only).
func TestOfferHardcodedProbe(t *testing.T) {
        source, err := os.ReadFile("offers.go")
        if err != nil {
                t.Fatalf("read: %v", err)
        }
        text := string(source)
        for _, literal := range []string{"WF2B196K666AA", "SUP-1", "Bosch", "Mann", "Febi", "самовывоз со склада", "Поставщик один", "ABCP-12345"} {
                if strings.Contains(text, literal) {
                        t.Fatalf("the production file carries the hardcoded literal %q", literal)
                }
        }
}

// TestPurchasePriceNotInClientMaterial: the CLIENT material family carries
// NO purchase price — structurally (the JSON of the whole comparison view
// never contains the purchase price field; the internal data lives in the
// record material of the operator section only — AC-2).
func TestPurchasePriceNotInClientMaterial(t *testing.T) {
        input := comparisonInputFixture()
        input.Criteria = ComparisonCriteria{BudgetConfirmed: true, RequestedQty: 4}
        input.Positions = []ComparisonPositionInput{provenPositionFixture(offerFixture())}
        encoded, err := json.Marshal(DeriveComparison(input))
        if err != nil {
                t.Fatalf("marshal: %v", err)
        }
        if strings.Contains(string(encoded), "purchase") {
                t.Fatalf("the client material carries the purchase price")
        }
}

// TestMarketingWordsProbe: the production file carries NO marketing words
// — no quality, resource or superiority claims are ever created (AC-5).
func TestMarketingWordsProbe(t *testing.T) {
        source, err := os.ReadFile("offers.go")
        if err != nil {
                t.Fatalf("read: %v", err)
        }
        text := strings.ToLower(string(source))
        for _, word := range []string{"лучший на рынке", "непревзойдённ", "высокое качество", "долговечн", "премиальн", "гарантированно надёжн"} {
                if strings.Contains(text, word) {
                        t.Fatalf("the production file carries the marketing word %q", word)
                }
        }
}
