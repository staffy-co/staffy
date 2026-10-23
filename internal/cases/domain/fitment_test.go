package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The Story 4.4 unit matrix: the state ladder by name (unknown / proven /
// stale-age / stale-pins / conflicted found-vs-not / conflicted
// different-vehicle-ids / ambiguous), the guard refusals by name, the
// digest byte-stability, the JSON idempotency ×2 and the hardcoded-words
// probe (the dictionary comes from the pack policy, never from the store
// code — the position/article literals stay out of the production files).

func fitmentPolicyFixture() FitmentPolicyDomain {
	return FitmentPolicyDomain{
		LinkTypes:      []string{FitmentLinkOriginal, FitmentLinkCross},
		MaxCrossDepth:  3,
		AllowedSources: []string{FitmentSourceAbcpCatalog, FitmentSourceLaximoCatalog},
		MaxAgeSeconds:  604800,
	}
}

func fitmentDraftFixture() FitmentEvidenceDraft {
	return FitmentEvidenceDraft{
		PositionKey:          "WF2B196K666AA",
		CatalogNode:          "Фильтр салона",
		Article:              "WF2B196K666AA",
		LinkType:             FitmentLinkOriginal,
		Chain:                []string{"Фильтр салона (оригинальная позиция)"},
		CatalogVehicleIDs:    []string{"ABCP-12345"},
		SourceSystem:         FitmentSourceAbcpCatalog,
		SourceRecordID:       "rec-1",
		SourceVersion:        "v2026.10",
		SourceObservedAt:     time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC),
		ProfileDigestPin:     strings.Repeat("v", 64),
		PartRequestDigestPin: strings.Repeat("p", 64),
	}
}

func fitmentEvidenceFixture() FitmentEvidenceInput {
	return FitmentEvidenceInput{
		EvidenceNo:           1,
		PositionKey:          "WF2B196K666AA",
		CatalogNode:          "Фильтр салона",
		Article:              "WF2B196K666AA",
		LinkType:             FitmentLinkOriginal,
		Chain:                []string{"Фильтр салона (оригинальная позиция)"},
		CatalogVehicleIDs:    []string{"ABCP-12345"},
		SourceSystem:         FitmentSourceAbcpCatalog,
		SourceRecordID:       "rec-1",
		SourceVersion:        "v2026.10",
		SourceObservedAt:     time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC),
		ReceivedAt:           time.Date(2026, 10, 24, 10, 5, 0, 0, time.UTC),
		ProfileDigestPin:     strings.Repeat("v", 64),
		PartRequestDigestPin: strings.Repeat("p", 64),
	}
}

func fitmentInputFixture() FitmentDerivationInput {
	return FitmentDerivationInput{
		Policy:            fitmentPolicyFixture(),
		ProfileDigest:     strings.Repeat("v", 64),
		PartRequestDigest: strings.Repeat("p", 64),
		Now:               time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC),
	}
}

// TestFitmentUnknownWithoutRecords: NO records at all — the case-level
// honest unknown word + the research recommendation (AC-10: the absence is
// never a negative or a positive fitment).
func TestFitmentUnknownWithoutRecords(t *testing.T) {
	view := DeriveFitment(fitmentInputFixture())
	if view.CaseUnknownWord != FitmentUnknownWord {
		t.Fatalf("case unknown word = %q", view.CaseUnknownWord)
	}
	if len(view.CaseHandoffWords) != 1 || view.CaseHandoffWords[0] != FitmentResearchWord {
		t.Fatalf("case handoff words = %v", view.CaseHandoffWords)
	}
	if view.NumberMatchWord != PartNumberMatchIsNotFitmentText {
		t.Fatalf("constant word of the exact number match is absent")
	}
	if view.PriceNote != FitmentPriceNote {
		t.Fatalf("price note is absent")
	}
	if len(view.Positions) != 0 {
		t.Fatalf("positions of the empty world = %d", len(view.Positions))
	}
}

// TestFitmentProvenOriginal: the found original record with the actual
// pins → proven with the honest word + the byte-stable digest.
func TestFitmentProvenOriginal(t *testing.T) {
	input := fitmentInputFixture()
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{fitmentEvidenceFixture()}}}
	view := DeriveFitment(input)
	if len(view.Positions) != 1 {
		t.Fatalf("positions = %d", len(view.Positions))
	}
	position := view.Positions[0]
	if position.Status != FitmentStatusProven {
		t.Fatalf("status = %q, want proven", position.Status)
	}
	if position.StatusWord != FitmentProvenWord {
		t.Fatalf("status word = %q", position.StatusWord)
	}
	if len(position.FitmentDigest) != 64 {
		t.Fatalf("digest length = %d", len(position.FitmentDigest))
	}
	if len(position.HandoffWords) != 0 {
		t.Fatalf("handoff words of proven = %v", position.HandoffWords)
	}
	// The digest is byte-stable across the repeated derivations.
	again := DeriveFitment(input)
	if again.Positions[0].FitmentDigest != position.FitmentDigest {
		t.Fatalf("digest is not byte-stable")
	}
}

// TestFitmentProvenCrossChain: the cross chain is carried WHOLE (AC-4) and
// the digest pins it.
func TestFitmentProvenCrossChain(t *testing.T) {
	input := fitmentInputFixture()
	evidence := fitmentEvidenceFixture()
	evidence.LinkType = FitmentLinkCross
	evidence.Chain = []string{"Фильтр салона (оригинальная позиция)", "кросс-запись источника", "аналог B330PR"}
	input.Positions = []FitmentPositionInput{{PositionKey: "B330PR", Evidences: []FitmentEvidenceInput{evidence}}}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusProven {
		t.Fatalf("status = %q, want proven", position.Status)
	}
	if len(position.Evidences) != 1 || len(position.Evidences[0].Chain) != 3 {
		t.Fatalf("the whole chain is not carried: %v", position.Evidences)
	}
}

// TestFitmentStaleByAge: the age over the pack TTL → stale with the honest
// word + the re-check word (AC-10).
func TestFitmentStaleByAge(t *testing.T) {
	input := fitmentInputFixture()
	evidence := fitmentEvidenceFixture()
	evidence.ReceivedAt = input.Now.Add(-time.Duration(input.Policy.MaxAgeSeconds+1) * time.Second)
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{evidence}}}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusStale {
		t.Fatalf("status = %q, want stale", position.Status)
	}
	if len(position.Words) == 0 || position.Words[0] != FitmentStaleRecheckWord {
		t.Fatalf("re-check word is absent: %v", position.Words)
	}
	if len(position.Evidences[0].StaleWords) != 1 || position.Evidences[0].StaleWords[0] != FitmentStaleAgeWord {
		t.Fatalf("age stale word = %v", position.Evidences[0].StaleWords)
	}
}

// TestFitmentStaleByProfilePin: the profile pin mismatch → stale with the
// profile word (AC-11: a new version of ANY input expires the evidence).
func TestFitmentStaleByProfilePin(t *testing.T) {
	input := fitmentInputFixture()
	input.ProfileDigest = strings.Repeat("x", 64)
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{fitmentEvidenceFixture()}}}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusStale {
		t.Fatalf("status = %q, want stale", position.Status)
	}
	if len(position.Evidences[0].StaleWords) != 1 || position.Evidences[0].StaleWords[0] != FitmentStaleProfileWord {
		t.Fatalf("profile stale word = %v", position.Evidences[0].StaleWords)
	}
}

// TestFitmentStaleByPartRequestPin: the request pin mismatch → stale with
// the request word (AC-11); the NEW record with the actual pin → proven
// again.
func TestFitmentStaleByPartRequestPin(t *testing.T) {
	input := fitmentInputFixture()
	input.PartRequestDigest = strings.Repeat("y", 64)
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{fitmentEvidenceFixture()}}}
	view := DeriveFitment(input)
	if view.Positions[0].Status != FitmentStatusStale {
		t.Fatalf("status = %q, want stale", view.Positions[0].Status)
	}
	if len(view.Positions[0].Evidences[0].StaleWords) != 1 || view.Positions[0].Evidences[0].StaleWords[0] != FitmentStalePartRequestWord {
		t.Fatalf("request stale word = %v", view.Positions[0].Evidences[0].StaleWords)
	}
	// The new record with the ACTUAL pin → proven again (AC-11).
	fresh := fitmentEvidenceFixture()
	fresh.EvidenceNo = 2
	fresh.PartRequestDigestPin = strings.Repeat("y", 64)
	input.Positions[0].Evidences = append(input.Positions[0].Evidences, fresh)
	view = DeriveFitment(input)
	if view.Positions[0].Status != FitmentStatusProven {
		t.Fatalf("status after the fresh record = %q, want proven", view.Positions[0].Status)
	}
}

// TestFitmentConflictedFoundAgainstNotFound: the not-found of one source
// AND the found of the other (the current records) → conflicted with BOTH
// proofs + the unresolved_contradiction recommendation (AC-9).
func TestFitmentConflictedFoundAgainstNotFound(t *testing.T) {
	input := fitmentInputFixture()
	found := fitmentEvidenceFixture()
	notFound := fitmentEvidenceFixture()
	notFound.EvidenceNo = 2
	notFound.SourceSystem = FitmentSourceLaximoCatalog
	notFound.CatalogVehicleIDs = nil
	notFound.CatalogNode = ""
	notFound.Article = ""
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{found, notFound}}}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusConflicted {
		t.Fatalf("status = %q, want conflicted", position.Status)
	}
	if position.StatusWord != FitmentConflictedSourcesWord {
		t.Fatalf("status word = %q", position.StatusWord)
	}
	if len(position.Evidences) != 2 {
		t.Fatalf("BOTH proofs must be carried: %d", len(position.Evidences))
	}
	if len(position.HandoffWords) != 1 || position.HandoffWords[0] != HandoffReasonUnresolvedContradiction {
		t.Fatalf("handoff words = %v", position.HandoffWords)
	}
}

// TestFitmentConflictedDifferentVehicleIDs: the different catalog vehicle
// ids of the current found records → conflicted (AC-9/AC-12).
func TestFitmentConflictedDifferentVehicleIDs(t *testing.T) {
	input := fitmentInputFixture()
	first := fitmentEvidenceFixture()
	second := fitmentEvidenceFixture()
	second.EvidenceNo = 2
	second.SourceSystem = FitmentSourceLaximoCatalog
	second.CatalogVehicleIDs = []string{"LAXIMO-999"}
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{first, second}}}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusConflicted {
		t.Fatalf("status = %q, want conflicted", position.Status)
	}
	if position.StatusWord != FitmentConflictedVehiclesWord {
		t.Fatalf("status word = %q", position.StatusWord)
	}
}

// TestFitmentAgiguousSeveralVehicleIDs: the found record with several
// catalog vehicle ids → ambiguous with the minimal question of the FIRST
// not-confirmed clarifiable attribute (AC-5: the confirmed attributes are
// never re-asked).
func TestFitmentAgiguousSeveralVehicleIDs(t *testing.T) {
	input := fitmentInputFixture()
	evidence := fitmentEvidenceFixture()
	evidence.CatalogVehicleIDs = []string{"ABCP-12345", "ABCP-12346"}
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{evidence}}}
	input.ClarifiableProfileAttributes = []FitmentProfileAttribute{
		{Key: "vehicle.vin", Label: "VIN", Confirmed: true},
		{Key: "vehicle.trim", Label: "Модификация", Confirmed: false},
	}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusAmbiguous {
		t.Fatalf("status = %q, want ambiguous", position.Status)
	}
	question, ok := position.Questions["vehicle.trim"]
	if !ok || !strings.Contains(question, "модификация") {
		t.Fatalf("the minimal question of the missing attribute is absent: %v", position.Questions)
	}
	if _, asked := position.Questions["vehicle.vin"]; asked {
		t.Fatalf("the CONFIRMED attribute is re-asked")
	}
}

// TestFitmentAmbiguousWithoutClarifiableAttribute: no clarifiable
// attribute left → the honest handoff recommendation (the research word of
// this story).
func TestFitmentAmbiguousWithoutClarifiableAttribute(t *testing.T) {
	input := fitmentInputFixture()
	evidence := fitmentEvidenceFixture()
	evidence.CatalogVehicleIDs = []string{"ABCP-12345", "ABCP-12346"}
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{evidence}}}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusAmbiguous {
		t.Fatalf("status = %q", position.Status)
	}
	if len(position.HandoffWords) != 1 || position.HandoffWords[0] != FitmentResearchWord {
		t.Fatalf("handoff words = %v", position.HandoffWords)
	}
}

// TestFitmentUnknownResearched: the single not-found record → the honest
// researched unknown — NEVER «неприменимо» (AC-10).
func TestFitmentUnknownResearched(t *testing.T) {
	input := fitmentInputFixture()
	evidence := fitmentEvidenceFixture()
	evidence.CatalogVehicleIDs = nil
	evidence.CatalogNode = ""
	evidence.Article = ""
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{evidence}}}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusUnknown {
		t.Fatalf("status = %q, want unknown", position.Status)
	}
	if position.StatusWord != FitmentUnknownResearchedWord {
		t.Fatalf("status word = %q", position.StatusWord)
	}
	if strings.Contains(strings.ToLower(position.StatusWord), "неприменимо") {
		t.Fatalf("the researched unknown must never become «неприменимо»")
	}
}

// TestFitmentSupersedeByNewEvidenceNumber: the NEW record of the same
// source supersedes the former one by the number — never an update (the
// current record is the maximal number one).
func TestFitmentSupersedeByNewEvidenceNumber(t *testing.T) {
	input := fitmentInputFixture()
	stale := fitmentEvidenceFixture()
	fresh := fitmentEvidenceFixture()
	fresh.EvidenceNo = 2
	fresh.CatalogVehicleIDs = nil
	fresh.CatalogNode = ""
	fresh.Article = ""
	// The store order is by the number; the derivation must elect the
	// current one whatever the slice order is.
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{fresh, stale}}}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusUnknown {
		t.Fatalf("status = %q, want unknown (the superseding not-found record wins)", position.Status)
	}
	if len(position.Evidences) != 1 || position.Evidences[0].EvidenceNo != 2 {
		t.Fatalf("the current record of the source is not elected: %v", position.Evidences)
	}
}

// TestFitmentProvenSafetyNode: the proven state on the safety node carries
// the safety note word (the 4.3 precedent — the confirmed material is
// mandatory, the manager gate of 4.7 stands before the supplier).
func TestFitmentProvenSafetyNode(t *testing.T) {
	input := fitmentInputFixture()
	input.SafetyNode = true
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{fitmentEvidenceFixture()}}}
	view := DeriveFitment(input)
	position := view.Positions[0]
	if position.Status != FitmentStatusProven {
		t.Fatalf("status = %q", position.Status)
	}
	found := false
	for _, word := range position.Words {
		if word == PartRequestSafetyNoteText {
			found = true
		}
	}
	if !found {
		t.Fatalf("the safety note word of the proven safety node is absent: %v", position.Words)
	}
}

// TestFitmentAgreeingSourcesProven: the agreeing found records of the two
// sources → proven (the consenting found records — the Implementation
// Notes).
func TestFitmentAgreeingSourcesProven(t *testing.T) {
	input := fitmentInputFixture()
	first := fitmentEvidenceFixture()
	second := fitmentEvidenceFixture()
	second.EvidenceNo = 2
	second.SourceSystem = FitmentSourceLaximoCatalog
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{first, second}}}
	view := DeriveFitment(input)
	if view.Positions[0].Status != FitmentStatusProven {
		t.Fatalf("status = %q, want proven", view.Positions[0].Status)
	}
	if len(view.Positions[0].Evidences) != 2 {
		t.Fatalf("both agreeing proofs must stay in the material: %d", len(view.Positions[0].Evidences))
	}
}

// TestGuardFitmentEvidenceRefusals: the honest refusals BEFORE any SQL —
// the dictionary words with the pack lists (nothing is silently fixed).
func TestGuardFitmentEvidenceRefusals(t *testing.T) {
	policy := fitmentPolicyFixture()
	cases := []struct {
		name string
		mut  func(*FitmentEvidenceDraft)
		rule string
		word string
	}{
		{"empty position key", func(d *FitmentEvidenceDraft) { d.PositionKey = "" }, "fitment_position_key_required", "ключ позиции"},
		{"foreign link type", func(d *FitmentEvidenceDraft) { d.LinkType = "similar" }, "fitment_link_type_out_of_dictionary", "Domain Policy пака"},
		{"foreign source", func(d *FitmentEvidenceDraft) { d.SourceSystem = "web_search" }, "fitment_source_out_of_dictionary", "Domain Policy пака"},
		{"non-canonical position key", func(d *FitmentEvidenceDraft) { d.PositionKey = "wf 2b19 6k666 aa" }, "fitment_position_key_not_canonical", "канонизируйте"},
		{"original with long chain", func(d *FitmentEvidenceDraft) { d.Chain = []string{"a", "b"} }, "fitment_original_chain_shape", "один узел"},
		{"cross too short", func(d *FitmentEvidenceDraft) { d.LinkType = FitmentLinkCross; d.Chain = []string{"only"} }, "fitment_cross_chain_too_short", "минимум два узла"},
		{"cross too long", func(d *FitmentEvidenceDraft) {
			d.LinkType = FitmentLinkCross
			d.Chain = []string{"a", "b", "c", "d", "e"}
		}, "fitment_cross_chain_too_long", "слишком длинная cross-цепочка"},
		{"found without node", func(d *FitmentEvidenceDraft) { d.CatalogNode = "" }, "fitment_catalog_node_required", "каталогический узел"},
		{"found without article", func(d *FitmentEvidenceDraft) { d.Article = "" }, "fitment_article_required", "артикул"},
		{"no source record id", func(d *FitmentEvidenceDraft) { d.SourceRecordID = "" }, "fitment_source_record_id_required", "трассируемость"},
		{"no source version", func(d *FitmentEvidenceDraft) { d.SourceVersion = "" }, "fitment_source_version_required", "версия источника"},
		{"no observed time", func(d *FitmentEvidenceDraft) { d.SourceObservedAt = time.Time{} }, "fitment_source_observed_at_required", "время получения"},
		{"no profile pin", func(d *FitmentEvidenceDraft) { d.ProfileDigestPin = "" }, "fitment_profile_digest_pin_required", "ProfileDigest"},
		{"no request pin", func(d *FitmentEvidenceDraft) { d.PartRequestDigestPin = "" }, "fitment_part_request_digest_pin_required", "PartRequestDigest"},
	}
	for _, testCase := range cases {
		draft := fitmentDraftFixture()
		testCase.mut(&draft)
		err := GuardFitmentEvidence(policy, draft)
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

// TestGuardFitmentEvidenceAccepts: the honest found record and the honest
// not-found record both pass (the not-found record is the honest material
// — the node and the article are optional when the catalog vehicle ids are
// empty).
func TestGuardFitmentEvidenceAccepts(t *testing.T) {
	policy := fitmentPolicyFixture()
	if err := GuardFitmentEvidence(policy, fitmentDraftFixture()); err != nil {
		t.Fatalf("the honest found record is refused: %v", err)
	}
	notFound := fitmentDraftFixture()
	notFound.CatalogVehicleIDs = nil
	notFound.CatalogNode = ""
	notFound.Article = ""
	if err := GuardFitmentEvidence(policy, notFound); err != nil {
		t.Fatalf("the honest not-found record is refused: %v", err)
	}
	cross := fitmentDraftFixture()
	cross.LinkType = FitmentLinkCross
	cross.Chain = []string{"оригинальная позиция", "кросс-запись источника", "аналог"}
	if err := GuardFitmentEvidence(policy, cross); err != nil {
		t.Fatalf("the honest cross record of depth 2 is refused: %v", err)
	}
}

// TestFitmentJSONIdempotency: the derivation is byte-stable — the JSON of
// the same world is identical across the repeated marshals (×2).
func TestFitmentJSONIdempotency(t *testing.T) {
	input := fitmentInputFixture()
	input.Positions = []FitmentPositionInput{{PositionKey: "WF2B196K666AA", Evidences: []FitmentEvidenceInput{fitmentEvidenceFixture()}}}
	view := DeriveFitment(input)
	first, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	repeated, err := json.Marshal(DeriveFitment(input))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(first) != string(repeated) {
		t.Fatalf("the derivation JSON is not byte-stable")
	}
}

// TestFitmentDigestCanonical: the digest is the canonical SHA-256 of the
// proof record — the same material yields the same digest, the different
// chain yields a different one.
func TestFitmentDigestCanonical(t *testing.T) {
	evidence := fitmentEvidenceFixture()
	if FitmentDigestOf(evidence) != FitmentDigestOf(evidence) {
		t.Fatalf("the digest is not deterministic")
	}
	changed := evidence
	changed.Chain = []string{"другая цепочка"}
	if FitmentDigestOf(evidence) == FitmentDigestOf(changed) {
		t.Fatalf("the different chain yields the same digest")
	}
}
