package domain

import (
	"strings"
	"testing"
)

// The Story 2.11 unit matrix: the preference dictionaries, the read-time
// derivation of the current set, the record guard, the accuracy qualifier
// gate and the honest alternatives of the deterministic policy (the
// trigger pair, the control branches and the fail-closed stop).

func recordOf(key string, revision int64, value string, cleared bool) AccommodationRecord {
	return AccommodationRecord{
		TenantID: newID(), CaseID: newID(), RecordID: newID(),
		Key: key, Revision: revision, Value: value, Cleared: cleared,
		ActorID: newID(),
	}
}

func noPhotoCurrent(revision int64, cleared bool) CurrentAccommodations {
	return CurrentAccommodations{AccommodationNoPhoto: AccommodationState{
		Key: AccommodationNoPhoto, Revision: revision, Cleared: cleared,
	}}
}

func TestAccommodationDictionariesClosed(t *testing.T) {
	if len(AccommodationFlagKeys) != 6 || len(AccommodationValuedKeys) != 2 {
		t.Fatalf("the dictionary carries six flags and two valued keys, got %d/%d", len(AccommodationFlagKeys), len(AccommodationValuedKeys))
	}
	if len(AccommodationKeys) != 8 {
		t.Fatalf("the full dictionary carries eight keys, got %d", len(AccommodationKeys))
	}
	seen := map[string]bool{}
	for _, key := range AccommodationKeys {
		if seen[key] {
			t.Fatalf("the dictionary key %q repeats", key)
		}
		seen[key] = true
		if !AccommodationKeyInDictionary(key) {
			t.Fatalf("the dictionary key %q is not in the dictionary", key)
		}
		if AccommodationKeyIsFlag(key) != inDictionary(AccommodationFlagKeys, key) {
			t.Fatalf("the flag/valued split is inconsistent for %q", key)
		}
	}
	if AccommodationKeyInDictionary("no_photos") || AccommodationKeyInDictionary("") {
		t.Fatalf("an unknown key never enters the dictionary")
	}
	if len(AlternativePathWords) != 4 {
		t.Fatalf("the alternative dictionary carries four words, got %d", len(AlternativePathWords))
	}
	for _, word := range AlternativePathWords {
		if AlternativeLimitationText(word) == "" {
			t.Fatalf("every alternative word carries its honest limitation text, %q does not", word)
		}
	}
	if AlternativeLimitationText("made_up_path") != "" {
		t.Fatalf("an unknown word renders no text")
	}
	if len(UsualModalityWords) != 2 {
		t.Fatalf("the usual-modality dictionary carries two words, got %d", len(UsualModalityWords))
	}
}

func TestUsualModalityMapOfMandatoryKeys(t *testing.T) {
	if word := UsualModalityOfMaterialKey("service_kind"); word != UsualModalityMediaPhoto {
		t.Fatalf("service_kind is usually established by media, got %q", word)
	}
	for _, key := range []string{"urgency", "slot_preference", "never_seen_key"} {
		if word := UsualModalityOfMaterialKey(key); word != UsualModalityText {
			t.Fatalf("key %q is usually text (the unknown key fails open to text), got %q", key, word)
		}
	}
	// The pack alternatives: the media modality carries the full dictionary;
	// the text modality carries none (the text path IS the usual path).
	if words := AlternativesOfUsualModality(UsualModalityMediaPhoto); len(words) != 4 {
		t.Fatalf("the media modality carries the four alternative words, got %d", len(words))
	}
	if words := AlternativesOfUsualModality(UsualModalityText); len(words) != 0 {
		t.Fatalf("the text modality carries no alternatives, got %v", words)
	}
	if words := AlternativesOfUsualModality("voice_note"); len(words) != 0 {
		t.Fatalf("an unknown modality carries no alternatives, got %v", words)
	}
}

func TestDeriveCurrentAccommodationsLatestRevisionWins(t *testing.T) {
	records := []AccommodationRecord{
		recordOf(AccommodationNoPhoto, 1, "", false),
		recordOf(AccommodationTextFirst, 1, "", false),
		recordOf(AccommodationNoPhoto, 3, "", false),
		recordOf(AccommodationNoPhoto, 2, "", false),
	}
	current := DeriveCurrentAccommodations(records)
	if len(current) != 2 {
		t.Fatalf("two keys carry their latest revisions, got %d", len(current))
	}
	if state := current[AccommodationNoPhoto]; state.Revision != 3 || state.Cleared {
		t.Fatalf("the latest revision of no_photo wins, got %+v", state)
	}
	// The derivation is order-independent: the reversed input derives the
	// same state (the latest revision wins on any input order).
	reversed := DeriveCurrentAccommodations([]AccommodationRecord{records[3], records[2], records[1], records[0]})
	if len(reversed) != len(current) || reversed[AccommodationNoPhoto] != current[AccommodationNoPhoto] {
		t.Fatalf("the derivation is order-independent, got %+v vs %+v", reversed, current)
	}
	// The equal-revision tie: the LATER record of the slice wins (the
	// defensive determinism — the table UNIQUE backstop makes the case
	// unreachable).
	tie := DeriveCurrentAccommodations([]AccommodationRecord{
		recordOf(AccommodationNoPhoto, 2, "", false),
		recordOf(AccommodationNoPhoto, 2, "", true),
	})
	if !tie[AccommodationNoPhoto].Cleared {
		t.Fatalf("the later record of an equal revision wins the defensive tie, got %+v", tie)
	}
}

func TestDeriveCurrentAccommodationsClearedRemovesFromActiveSet(t *testing.T) {
	records := []AccommodationRecord{
		recordOf(AccommodationNoPhoto, 1, "", false),
		recordOf(AccommodationNoPhoto, 2, "", true),
		recordOf(AccommodationPreferredChannel, 1, "phone", false),
	}
	current := DeriveCurrentAccommodations(records)
	// The cleared key stays in the STATE (the honest revision truth of the
	// snapshot) but leaves the ACTIVE set.
	if _, known := current[AccommodationNoPhoto]; !known {
		t.Fatalf("the cleared key stays in the derived state")
	}
	if current.HasActive(AccommodationNoPhoto) {
		t.Fatalf("the clearing revision removes the key from the active set")
	}
	if !current.HasActive(AccommodationPreferredChannel) {
		t.Fatalf("the valued key stays active")
	}
	if current[AccommodationPreferredChannel].Value != "phone" {
		t.Fatalf("the valued material travels with the state, got %+v", current[AccommodationPreferredChannel])
	}
	if keys := current.ActiveKeys(); len(keys) != 1 || keys[0] != AccommodationPreferredChannel {
		t.Fatalf("the active keys are the sorted non-cleared keys, got %v", keys)
	}
	empty := DeriveCurrentAccommodations(nil)
	if len(empty) != 0 || empty.HasActive(AccommodationNoPhoto) {
		t.Fatalf("no records — no state, got %+v", empty)
	}
}

func TestAccommodationSnapshotOfCanonicalForm(t *testing.T) {
	// The snapshot map is ALWAYS initialized: the empty state renders {}
	// (never null) — the canonical digest form; the cleared revisions stay.
	empty := AccommodationSnapshotOf(nil)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("the empty state renders an initialized empty map, got %+v", empty)
	}
	state := DeriveCurrentAccommodations([]AccommodationRecord{
		recordOf(AccommodationNoPhoto, 1, "", false),
		recordOf(AccommodationNoPhoto, 2, "", true),
	})
	snapshot := AccommodationSnapshotOf(state)
	if len(snapshot) != 1 {
		t.Fatalf("the cleared key stays in the snapshot map, got %+v", snapshot)
	}
	entry, ok := snapshot[AccommodationNoPhoto]
	if !ok || entry.Revision != 2 || !entry.Cleared {
		t.Fatalf("the snapshot entry carries the revision and the cleared mark, got %+v", entry)
	}
}

func TestGuardAccommodationRecordRules(t *testing.T) {
	// The happy paths: a flag without a value, a valued channel word, the
	// availability line, the clearing revision of a flag.
	if err := GuardAccommodationRecord(AccommodationNoPhoto, "", false, ""); err != nil {
		t.Fatalf("the flag record passes, got %v", err)
	}
	if err := GuardAccommodationRecord(AccommodationPreferredChannel, "phone", false, "клиент просил звонить"); err != nil {
		t.Fatalf("the preferred channel word passes, got %v", err)
	}
	if err := GuardAccommodationRecord(AccommodationAvailableTime, "после 18:00", false, ""); err != nil {
		t.Fatalf("the availability line passes, got %v", err)
	}
	if err := GuardAccommodationRecord(AccommodationNoPhoto, "", true, ""); err != nil {
		t.Fatalf("the clearing revision of a flag passes, got %v", err)
	}
	for _, bad := range []struct {
		key     string
		value   string
		cleared bool
		note    string
	}{
		{"made_up_key", "", false, ""},                                    // the unknown key
		{AccommodationNoPhoto, "yes", false, ""},                          // a flag with a value
		{AccommodationPreferredChannel, "", false, ""},                    // a valued key without a value
		{AccommodationPreferredChannel, "fax", false, ""},                 // a channel outside the 2.1 dictionary
		{AccommodationAvailableTime, "", false, ""},                       // an empty availability line
		{AccommodationAvailableTime, strings.Repeat("п", 101), false, ""}, // the 100-rune bound
		{AccommodationPreferredChannel, "phone", true, ""},                // a clearing with a value
		{AccommodationNoPhoto, "", false, strings.Repeat("н", 501)},       // the 500-rune note bound
	} {
		if err := GuardAccommodationRecord(bad.key, bad.value, bad.cleared, bad.note); err == nil {
			t.Fatalf("expected the guard refusal for %+v", bad)
		}
	}
}

func TestGuardSalesReplyAccuracyGate(t *testing.T) {
	active := append([]string(nil), AlternativePathWords...)
	// No active alternatives: the empty field keeps the byte-identical old
	// path; a present field is the foreign refusal (the field has exactly
	// one meaning).
	if err := GuardSalesReplyAccuracy(nil, ""); err != nil {
		t.Fatalf("no alternatives and the empty field pass, got %v", err)
	}
	err := GuardSalesReplyAccuracy(nil, AlternativeTextDescription)
	if err == nil {
		t.Fatalf("a qualifier without alternatives is refused")
	}
	if rule := err.(WorkRuleError); rule.Rule != "accuracy_qualifier_foreign" {
		t.Fatalf("the refusal rule is accuracy_qualifier_foreign, got %+v", rule)
	}
	// Active alternatives: the qualifier is REQUIRED and the refusal names
	// the active words.
	err = GuardSalesReplyAccuracy(active, "")
	if err == nil {
		t.Fatalf("a missing qualifier against active alternatives is refused")
	}
	if rule := err.(WorkRuleError); rule.Rule != "accuracy_qualifier_required" || !strings.Contains(rule.Reason, AlternativeTextDescription) {
		t.Fatalf("the required refusal names the active words, got %+v", rule)
	}
	// A qualifier outside the active set: the foreign refusal (the subset of
	// the outcome, never the dictionary in general — a dictionary word of
	// another context is foreign here too).
	err = GuardSalesReplyAccuracy([]string{AlternativeTextDescription}, AlternativeCallback)
	if err == nil {
		t.Fatalf("a dictionary word outside the outcome's active set is refused")
	}
	if rule := err.(WorkRuleError); rule.Rule != "accuracy_qualifier_foreign" || !strings.Contains(rule.Reason, AlternativeTextDescription) {
		t.Fatalf("the outside-active refusal names the active words, got %+v", rule)
	}
	err = GuardSalesReplyAccuracy(active, "made_up_qualifier")
	if err == nil {
		t.Fatalf("an unknown qualifier is refused")
	}
	// The pass: the qualifier of the active set.
	if err := GuardSalesReplyAccuracy(active, AlternativeRangeEstimate); err != nil {
		t.Fatalf("the qualifier of the active set passes, got %v", err)
	}
}

func TestEvaluatePolicyMediaAlternativesAttach(t *testing.T) {
	// The trigger pair: the media-usual mandatory fact is missing AND the
	// active no_photo — the commercial answer gains the active alternatives
	// and the question STAYS in the plan.
	decision := EvaluatePolicy(AIProposal{
		Intent: IntentPriceInquiry, CandidateAnswer: "Сориентирую по описанию",
	}, confirmedWorld(), ModeSales, noPhotoCurrent(1, false))
	if decision.OutcomeClass != OutcomeCommercialAnswer || decision.Reason != ReasonOkCommercial {
		t.Fatalf("the commercial answer stays the commercial answer, got %+v", decision)
	}
	if len(decision.ActiveAlternatives) != 4 {
		t.Fatalf("the full pack dictionary attaches, got %+v", decision.ActiveAlternatives)
	}
	asked := map[string]bool{}
	for _, step := range decision.NextSteps {
		if step.Step == StepAskFact {
			asked[step.MaterialKey] = true
		}
	}
	if !asked["service_kind"] || !asked["urgency"] {
		t.Fatalf("the media question NEVER leaves the plan, got %+v", decision.NextSteps)
	}
	// The clarify outcome carries the alternatives too (the questions only
	// decision of the missing candidate).
	clarify := EvaluatePolicy(AIProposal{
		Intent: IntentPriceInquiry,
	}, confirmedWorld(), ModeSales, noPhotoCurrent(1, false))
	if clarify.OutcomeClass != OutcomeClarify || len(clarify.ActiveAlternatives) != 4 {
		t.Fatalf("the clarify outcome carries the alternatives, got %+v", clarify)
	}
}

func TestEvaluatePolicyAlternativesControlBranches(t *testing.T) {
	proposal := AIProposal{Intent: IntentPriceInquiry, CandidateAnswer: "Сориентирую по описанию"}
	// The control branch: no preferences — the decision stays byte-identical
	// (no alternatives, the former class/reason/next_steps).
	former := EvaluatePolicy(proposal, confirmedWorld(), ModeSales, nil)
	if former.OutcomeClass != OutcomeCommercialAnswer || len(former.ActiveAlternatives) != 0 {
		t.Fatalf("no preferences — no alternatives, got %+v", former)
	}
	// The irrelevant preferences (no no_photo) change nothing.
	other := CurrentAccommodations{AccommodationTextFirst: AccommodationState{Key: AccommodationTextFirst, Revision: 1}}
	irrelevant := EvaluatePolicy(proposal, confirmedWorld(), ModeSales, other)
	if irrelevant.OutcomeClass != former.OutcomeClass || irrelevant.Reason != former.Reason ||
		len(irrelevant.NextSteps) != len(former.NextSteps) || len(irrelevant.ActiveAlternatives) != 0 {
		t.Fatalf("the irrelevant preferences keep the control branch, got %+v vs %+v", irrelevant, former)
	}
	// The cleared no_photo is NOT active: the control branch.
	cleared := EvaluatePolicy(proposal, confirmedWorld(), ModeSales, noPhotoCurrent(2, true))
	if len(cleared.ActiveAlternatives) != 0 {
		t.Fatalf("the cleared no_photo is not active, got %+v", cleared)
	}
	// The confirmed media fact: the fact is not missing — no alternatives.
	confirmed := EvaluatePolicy(proposal, confirmedWorld("service_kind"), ModeSales, noPhotoCurrent(1, false))
	if len(confirmed.ActiveAlternatives) != 0 {
		t.Fatalf("the confirmed media fact carries no alternatives, got %+v", confirmed)
	}
	// The conflicting/unverified media fact: the fact is not confirmed — the
	// alternatives attach (the question stays, the media basis is absent).
	unverified := FactWorld{"service_kind": {Status: FactStatusUnverified}}
	conflicting := EvaluatePolicy(proposal, unverified, ModeSales, noPhotoCurrent(1, false))
	if len(conflicting.ActiveAlternatives) != 4 {
		t.Fatalf("the unconfirmed media fact with the active no_photo attaches the alternatives, got %+v", conflicting)
	}
	// The text-usual mandatory key only (booking_request): no alternatives
	// (the text path IS the usual path).
	booking := EvaluatePolicy(AIProposal{Intent: IntentBookingRequest, CandidateAnswer: "Есть окно"}, confirmedWorld(), ModeSales, noPhotoCurrent(1, false))
	if len(booking.ActiveAlternatives) != 0 {
		t.Fatalf("the text-usual key never triggers the alternatives, got %+v", booking)
	}
	// The non-commercial intent and the blocked mode never carry the offer.
	general := EvaluatePolicy(AIProposal{Intent: IntentGeneralQuestion, CandidateAnswer: "Часы: 9-18"}, confirmedWorld(), ModeSales, noPhotoCurrent(1, false))
	if len(general.ActiveAlternatives) != 0 {
		t.Fatalf("the informational answer carries no alternatives, got %+v", general)
	}
	blocked := EvaluatePolicy(proposal, confirmedWorld(), ModeSafety, noPhotoCurrent(1, false))
	if blocked.OutcomeClass != OutcomeModeBlocked || len(blocked.ActiveAlternatives) != 0 {
		t.Fatalf("the blocked mode carries no alternatives, got %+v", blocked)
	}
}

func TestApplyMediaAlternativesFailClosedOnEmptyPack(t *testing.T) {
	// The empty pack dictionary of the media key: the honest fail-closed
	// stop (the check never weakens, the path hands off to a human). The
	// fixed v0 map always yields the four words for media_photo — the unit
	// probe pins the branch for the future pack forms.
	decision := applyMediaAlternatives(PolicyDecision{OutcomeClass: OutcomeCommercialAnswer, Reason: ReasonOkCommercial}, true, UsualModalityText)
	if decision.OutcomeClass != OutcomeFailClosed || decision.Reason != ReasonNoAlternativePath {
		t.Fatalf("the empty pack dictionary fails closed, got %+v", decision)
	}
	if len(decision.NextSteps) != 1 || decision.NextSteps[0].Step != StepHumanHandoff {
		t.Fatalf("the fail-closed stop hands off to a human, got %+v", decision.NextSteps)
	}
	if len(decision.ActiveAlternatives) != 0 {
		t.Fatalf("the fail-closed stop offers no alternatives, got %+v", decision.ActiveAlternatives)
	}
	// The untriggered call returns the decision untouched.
	same := PolicyDecision{OutcomeClass: OutcomeCommercialAnswer, Reason: ReasonOkCommercial}
	if kept := applyMediaAlternatives(same, false, ""); kept.OutcomeClass != same.OutcomeClass || len(kept.ActiveAlternatives) != 0 {
		t.Fatalf("the untriggered call keeps the decision, got %+v", kept)
	}
}

func TestDecisionDigestPreferencesHonestSupersede(t *testing.T) {
	base := PolicyInputSnapshot{
		ProposalID: newID(), SchemaVersion: ProposalSchemaVersion,
		ModeVersion: 2, OwnerEpoch: 1,
		FactsDigest:     FactsDigestMap{"service_kind": {VersionNo: 1, ValueDigest: "digest-service_kind"}},
		FactsDigestHash: "hash", WorkItemVersion: 1, RuleVersion: PolicyRuleVersion,
	}
	decision := PolicyDecision{OutcomeClass: OutcomeCommercialAnswer, Reason: ReasonOkCommercial}
	// The same preferences state produces the same digest (the map is
	// canonical — the empty state renders {}).
	first := base
	first.Preferences = AccommodationSnapshotOf(nil)
	digestEmpty, err := DecisionDigest(first, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	second := base
	second.Preferences = AccommodationSnapshotOf(nil)
	digestEmpty2, err := DecisionDigest(second, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if digestEmpty != digestEmpty2 {
		t.Fatalf("the same preferences state produces the same digest, got %q vs %q", digestEmpty, digestEmpty2)
	}
	// A recorded preference changes the digest (the honest supersede).
	withNoPhoto := base
	withNoPhoto.Preferences = AccommodationSnapshotOf(noPhotoCurrent(1, false))
	digestNoPhoto, err := DecisionDigest(withNoPhoto, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if digestNoPhoto == digestEmpty {
		t.Fatalf("a recorded preference must change the digest (the honest supersede)")
	}
	// The clearing revision changes the digest again (the state, not just
	// the presence, is the input).
	cleared := base
	cleared.Preferences = AccommodationSnapshotOf(noPhotoCurrent(2, true))
	digestCleared, err := DecisionDigest(cleared, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if digestCleared == digestNoPhoto || digestCleared == digestEmpty {
		t.Fatalf("the clearing revision is the honest supersede of the state, got %q", digestCleared)
	}
	// The alternatives are part of the decision digest.
	withAlternatives := decision
	withAlternatives.ActiveAlternatives = append([]string(nil), AlternativePathWords...)
	digestAlternatives, err := DecisionDigest(withNoPhoto, withAlternatives, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if digestAlternatives == digestNoPhoto {
		t.Fatalf("the active alternatives must change the decision digest")
	}
}
