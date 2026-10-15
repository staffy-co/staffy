package domain

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.6 unit matrix: the dictionaries, the dedup, the outcome
// classes, the digests (the determinism proof), the submission schema
// guard and the sales reply gate words.

func newID() id.UUID {
	value, err := id.Parse("019a2f4c-1000-7000-8000-000000000001")
	if err != nil {
		panic(err)
	}
	return value
}

func confirmedWorld(keys ...string) FactWorld {
	world := FactWorld{}
	for _, key := range keys {
		world[key] = FactStatusWord{Status: FactStatusConfirmed, Confirmed: &ConfirmedFact{
			FactID: newID(), MaterialKey: key, VersionNo: 1, ValueDigest: "digest-" + key,
		}}
	}
	return world
}

func TestEvaluatePolicyUnknownIntentFailsClosed(t *testing.T) {
	decision := EvaluatePolicy(AIProposal{Intent: "urgent_magic"}, confirmedWorld(), ModeSales, nil, nil, RouteKey{})
	if decision.OutcomeClass != OutcomeFailClosed || decision.Reason != ReasonUnknownIntent {
		t.Fatalf("expected the fail-closed unknown-intent outcome, got %+v", decision)
	}
	if len(decision.NextSteps) != 1 || decision.NextSteps[0].Step != StepHumanHandoff {
		t.Fatalf("expected the human handoff step, got %+v", decision.NextSteps)
	}
	if len(decision.MandatoryFacts) != 0 {
		t.Fatalf("the unknown intent carries no question plan, got %+v", decision.MandatoryFacts)
	}
}

func TestEvaluatePolicyCommercialUnderNonSalesModeBlocked(t *testing.T) {
	for _, mode := range []string{ModeSafety, ModeClaim, ModeCancellation, ModePrivacyNoContact, ModeHumanRequested, ModeUnclassified, ""} {
		decision := EvaluatePolicy(AIProposal{Intent: IntentPriceInquiry, CandidateAnswer: "Диагностика от 3000"}, confirmedWorld(), mode, nil, nil, RouteKey{})
		if decision.OutcomeClass != OutcomeModeBlocked || decision.Reason != ReasonModeBlocksCommercial {
			t.Fatalf("mode %q: expected the blocked outcome, got %+v", mode, decision)
		}
		if decision.OutcomeClass == OutcomeModeBlocked {
			if len(decision.NextSteps) != 1 || decision.NextSteps[0].Step != StepHumanHandoff {
				t.Fatalf("mode %q: expected the handoff step, got %+v", mode, decision.NextSteps)
			}
		}
	}
}

func TestEvaluatePolicyCommercialNoCandidateClarify(t *testing.T) {
	decision := EvaluatePolicy(AIProposal{
		Intent:       IntentPriceInquiry,
		MissingFacts: []MissingFact{{MaterialKey: "service_kind", Reason: MissingReasonMandatory}},
	}, confirmedWorld(), ModeSales, nil, nil, RouteKey{})
	if decision.OutcomeClass != OutcomeClarify || decision.Reason != ReasonNoAnswerBasis {
		t.Fatalf("expected the clarify outcome, got %+v", decision)
	}
	// The question plan of the clarify outcome: the rule-table keys of
	// price_inquiry (service_kind, urgency) — the minimal set, questions
	// only, never the confirmation step.
	asked := map[string]bool{}
	for _, step := range decision.NextSteps {
		if step.Step == StepAskFact {
			asked[step.MaterialKey] = true
		} else {
			t.Fatalf("the clarify outcome carries the questions only, got %+v", decision.NextSteps)
		}
	}
	if !asked["service_kind"] || !asked["urgency"] {
		t.Fatalf("expected the minimal question set of price_inquiry, got %+v", decision.NextSteps)
	}
}

func TestEvaluatePolicyCommercialAnswerProgressive(t *testing.T) {
	world := confirmedWorld("service_kind")
	decision := EvaluatePolicy(AIProposal{
		Intent:          IntentPriceInquiry,
		CandidateAnswer: "Диагностика от 3000",
		MissingFacts:    []MissingFact{{MaterialKey: "urgency", Reason: MissingReasonMandatory}},
	}, world, ModeSales, nil, nil, RouteKey{})
	if decision.OutcomeClass != OutcomeCommercialAnswer || decision.Reason != ReasonOkCommercial {
		t.Fatalf("expected the progressive commercial answer, got %+v", decision)
	}
	// The dedup: the confirmed service_kind is suppressed and reused with
	// the provenance; the urgency question stays in the plan.
	if len(decision.SuppressedQuestions) != 1 || decision.SuppressedQuestions[0].MaterialKey != "service_kind" {
		t.Fatalf("expected the suppressed service_kind repeat, got %+v", decision.SuppressedQuestions)
	}
	if len(decision.ReusedFacts) != 1 || decision.ReusedFacts[0].FactID != newID() || decision.ReusedFacts[0].VersionNo != 1 || decision.ReusedFacts[0].ValueDigest != "digest-service_kind" {
		t.Fatalf("expected the reused service_kind provenance, got %+v", decision.ReusedFacts)
	}
	asked := map[string]bool{}
	for _, step := range decision.NextSteps {
		if step.Step == StepAskFact {
			asked[step.MaterialKey] = true
		}
	}
	if asked["service_kind"] || !asked["urgency"] {
		t.Fatalf("expected only the urgency question, got %+v", decision.NextSteps)
	}
	awaiting := false
	for _, step := range decision.NextSteps {
		if step.Step == StepAwaitConfirmation {
			awaiting = true
		}
	}
	if !awaiting {
		t.Fatalf("the commercial answer awaits the confirmation, got %+v", decision.NextSteps)
	}
	states := map[string]string{}
	for _, fact := range decision.MandatoryFacts {
		states[fact.MaterialKey] = fact.State
	}
	if states["service_kind"] != MandatoryStateReused || states["urgency"] != MandatoryStateAsk {
		t.Fatalf("expected reused/ask states, got %+v", decision.MandatoryFacts)
	}
}

func TestEvaluatePolicyConflictAndUnverifiedKeepQuestion(t *testing.T) {
	world := FactWorld{
		"service_kind": {Status: FactStatusConflicting},
		"urgency":      {Status: FactStatusUnverified},
	}
	decision := EvaluatePolicy(AIProposal{Intent: IntentPriceInquiry, CandidateAnswer: "Есть вариант"}, world, ModeSales, nil, nil, RouteKey{})
	asked := map[string]bool{}
	for _, step := range decision.NextSteps {
		if step.Step == StepAskFact {
			asked[step.MaterialKey] = true
		}
	}
	if !asked["service_kind"] || !asked["urgency"] {
		t.Fatalf("the conflicting and unverified facts keep the questions, got %+v", decision.NextSteps)
	}
	if len(decision.ReusedFacts) != 0 || len(decision.SuppressedQuestions) != 0 {
		t.Fatalf("no reuse of the unconfirmed facts, got %+v", decision.ReusedFacts)
	}
}

func TestEvaluatePolicyExplicitReadbackStaysConfirmed(t *testing.T) {
	world := confirmedWorld("service_kind")
	decision := EvaluatePolicy(AIProposal{
		Intent:          IntentPriceInquiry,
		CandidateAnswer: "Диагностика от 3000",
		MissingFacts:    []MissingFact{{MaterialKey: "service_kind", Reason: MissingReasonReadback}},
	}, world, ModeSales, nil, nil, RouteKey{})
	asked := false
	for _, step := range decision.NextSteps {
		if step.Step == StepAskFact && step.MaterialKey == "service_kind" {
			asked = true
		}
	}
	if !asked {
		t.Fatalf("the explicit readback stays a question even when confirmed, got %+v", decision.NextSteps)
	}
	if len(decision.SuppressedQuestions) != 0 || len(decision.ReusedFacts) != 0 {
		t.Fatalf("the readback key is never suppressed, got %+v", decision.SuppressedQuestions)
	}
}

func TestEvaluatePolicyNonCommercialInformational(t *testing.T) {
	decision := EvaluatePolicy(AIProposal{Intent: IntentGeneralQuestion, CandidateAnswer: "Часы работы: 9-18"}, confirmedWorld(), ModeSafety, nil, nil, RouteKey{})
	if decision.OutcomeClass != OutcomeInformationalAnswer || decision.Reason != ReasonOkInformational {
		t.Fatalf("expected the informational answer, got %+v", decision)
	}
	human := EvaluatePolicy(AIProposal{Intent: IntentHumanRequest}, confirmedWorld(), ModeSafety, nil, nil, RouteKey{})
	handoff := false
	for _, step := range human.NextSteps {
		if step.Step == StepHumanHandoff {
			handoff = true
		}
	}
	if !handoff {
		t.Fatalf("the human request carries the handoff step, got %+v", human.NextSteps)
	}
}

func TestEvaluatePolicyProposalMandatoryJoinsRulePlan(t *testing.T) {
	world := confirmedWorld()
	decision := EvaluatePolicy(AIProposal{
		Intent:          IntentBookingRequest,
		CandidateAnswer: "Есть окно в четверг",
		MissingFacts:    []MissingFact{{MaterialKey: "preferred_time", Reason: MissingReasonMandatory}},
	}, world, ModeSales, nil, nil, RouteKey{})
	asked := map[string]bool{}
	for _, step := range decision.NextSteps {
		if step.Step == StepAskFact {
			asked[step.MaterialKey] = true
		}
	}
	if !asked["slot_preference"] || !asked["preferred_time"] {
		t.Fatalf("the rule keys and the proposal claims join the plan, got %+v", decision.NextSteps)
	}
	// The proposal claim source word travels.
	sources := map[string]string{}
	for _, fact := range decision.MandatoryFacts {
		sources[fact.MaterialKey] = fact.Source
	}
	if sources["slot_preference"] != QuestionSourceRule || sources["preferred_time"] != QuestionSourceProposal {
		t.Fatalf("expected the rule and proposal source words, got %+v", decision.MandatoryFacts)
	}
}

func TestDecisionDigestDeterminism(t *testing.T) {
	snapshot := PolicyInputSnapshot{
		ProposalID: newID(), SchemaVersion: ProposalSchemaVersion,
		ModeVersion: 2, OwnerEpoch: 1,
		FactsDigest:     FactsDigestMap{"service_kind": {VersionNo: 1, ValueDigest: "digest-service_kind"}},
		FactsDigestHash: "hash", WorkItemVersion: 1, RuleVersion: PolicyRuleVersion,
	}
	decision := PolicyDecision{OutcomeClass: OutcomeCommercialAnswer, Reason: ReasonOkCommercial}
	first, err := DecisionDigest(snapshot, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	second, err := DecisionDigest(snapshot, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("the same inputs produce the same digest, got %q vs %q", first, second)
	}
	// Any changed input changes the digest.
	changed := decision
	changed.OutcomeClass = OutcomeClarify
	other, err := DecisionDigest(snapshot, changed, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatalf("a changed decision must change the digest")
	}
	changedInput := snapshot
	changedInput.OwnerEpoch = 2
	other, err = DecisionDigest(changedInput, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatalf("a changed input must change the digest")
	}
}

func TestComputeFactsDigestCanonical(t *testing.T) {
	first, map1, err := ComputeFactsDigest([]ConfirmedFact{
		{FactID: newID(), MaterialKey: "urgency", VersionNo: 2, ValueDigest: "b"},
		{FactID: newID(), MaterialKey: "service_kind", VersionNo: 1, ValueDigest: "a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, map2, err := ComputeFactsDigest([]ConfirmedFact{
		{FactID: newID(), MaterialKey: "service_kind", VersionNo: 1, ValueDigest: "a"},
		{FactID: newID(), MaterialKey: "urgency", VersionNo: 2, ValueDigest: "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("the digest is order-independent, got %q vs %q", first, second)
	}
	if !reflect.DeepEqual(map1, map2) {
		t.Fatalf("the canonical maps are equal, got %+v vs %+v", map1, map2)
	}
	empty, _, err := ComputeFactsDigest(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 64 {
		t.Fatalf("the empty set has a deterministic digest, got %q", empty)
	}
}

func TestComputeFactsDigestKeyOrder(t *testing.T) {
	// The canonical JSON of the map must be the sorted-key form: the raw
	// json.Marshal of a map sorts the keys — the two builds with the
	// different construction order share the bytes.
	first, _, err := ComputeFactsDigest([]ConfirmedFact{
		{MaterialKey: "a", VersionNo: 1, ValueDigest: "x"},
		{MaterialKey: "b", VersionNo: 1, ValueDigest: "y"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(FactsDigestMap{"a": {VersionNo: 1, ValueDigest: "x"}, "b": {VersionNo: 1, ValueDigest: "y"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\"a\"") || !strings.Contains(string(raw), "\"b\"") {
		t.Fatalf("the map renders both keys")
	}
	if len(first) != 64 {
		t.Fatalf("the digest is the hex SHA-256, got %q", first)
	}
}

func TestGuardSubmitProposalFieldProblems(t *testing.T) {
	err := GuardSubmitProposal(AIProposal{
		Intent:   strings.Repeat("и", 65),
		Proposer: "machine",
		MissingFacts: []MissingFact{
			{MaterialKey: "Service Kind", Reason: "maybe"},
			{MaterialKey: "Service Kind", Reason: "mandatory"},
		},
		ExtractedFacts: []ExtractedFact{{MaterialKey: "k", ValueDigest: strings.Repeat("d", 129)}},
	})
	var schema ProposalSchemaError
	if !errors.As(err, &schema) {
		t.Fatalf("expected the schema error, got %v", err)
	}
	fields := map[string]string{}
	for _, problem := range schema.Problems {
		fields[problem.Field] = problem.Problem
	}
	for _, field := range []string{
		"conversation_id", "work_item_id", "turn_ref", "intent", "proposer",
		"missing_facts[0].material_key", "missing_facts[0].reason", "missing_facts[1].material_key",
		"extracted_facts[0].source_event_id", "extracted_facts[0].value_digest",
	} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("expected the field problem %q, got %+v", field, schema.Problems)
		}
	}
	// The intent length is a bounds matter — the dictionary word
	// "urgent_magic" is NOT a submission problem (the policy decides it).
	valid := GuardSubmitProposal(AIProposal{
		ConversationID: newID(), WorkItemID: newID(), TurnRef: newID(),
		Intent: "urgent_magic", Proposer: ProposerSyntheticPort,
		MissingFacts: []MissingFact{{MaterialKey: "urgency", Reason: MissingReasonReadback}},
	})
	if valid != nil {
		t.Fatalf("the unknown intent passes the submission guard (the policy decides), got %v", valid)
	}
}

func TestGuardEvaluateAnchors(t *testing.T) {
	if err := GuardEvaluate(newID(), 1, 1, 1, "digest"); err != nil {
		t.Fatalf("the valid anchors pass, got %v", err)
	}
	for _, bad := range []struct {
		proposal id.UUID
		mode     int64
		epoch    int64
		version  int64
		digest   string
	}{
		{id.UUID(""), 1, 1, 1, "d"},
		{newID(), 0, 1, 1, "d"},
		{newID(), 1, 0, 1, "d"},
		{newID(), 1, 1, 0, "d"},
		{newID(), 1, 1, 1, "  "},
	} {
		if err := GuardEvaluate(bad.proposal, bad.mode, bad.epoch, bad.version, bad.digest); err == nil {
			t.Fatalf("expected the anchor refusal for %+v", bad)
		}
	}
}

func TestOutcomePermittedPathWords(t *testing.T) {
	if OutcomePermittedPath(OutcomeModeBlocked) != PermittedPathExit {
		t.Fatalf("the blocked outcome names the mode exit")
	}
	for _, class := range []string{OutcomeCommercialAnswer, OutcomeInformationalAnswer, OutcomeClarify, OutcomeFailClosed} {
		if OutcomePermittedPath(class) != PermittedPathEvaluatePolicy {
			t.Fatalf("class %q names the evaluation path", class)
		}
	}
}

func TestCurrentPolicyOutcomeMaxNumber(t *testing.T) {
	none := CurrentPolicyOutcome(nil)
	if none != nil {
		t.Fatalf("no outcomes — no current outcome")
	}
	outcomes := []PolicyOutcome{
		{OutcomeID: newID(), OutcomeNo: 1, OutcomeClass: OutcomeCommercialAnswer},
		{OutcomeID: newID(), OutcomeNo: 3, OutcomeClass: OutcomeClarify},
		{OutcomeID: newID(), OutcomeNo: 2, OutcomeClass: OutcomeCommercialAnswer},
	}
	current := CurrentPolicyOutcome(outcomes)
	if current.OutcomeNo != 3 || current.OutcomeClass != OutcomeClarify {
		t.Fatalf("the current outcome is the maximal number, got %+v", current)
	}
}

func TestPolicyOutboxPayloadsCarryNoCandidateText(t *testing.T) {
	payload, err := AIProposalSubmittedPayloadJSON(AIProposalSubmittedPayload{
		TenantID: newID(), CaseID: newID(), ConversationID: newID(), WorkItemID: newID(),
		ProposalID: newID(), TurnRef: newID(), SchemaVersion: ProposalSchemaVersion,
		Intent: IntentPriceInquiry, Proposer: ProposerSyntheticPort,
		OwnerEpoch: 1, ModeVersion: 2, ActorID: newID(), OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "candidate") {
		t.Fatalf("the proposal envelope never carries the candidate text")
	}
	evaluated, err := PolicyEvaluatedPayloadJSON(PolicyEvaluatedPayload{
		TenantID: newID(), CaseID: newID(), ConversationID: newID(), WorkItemID: newID(),
		ProposalID: newID(), OutcomeID: newID(), OutcomeNo: 1,
		OutcomeClass: OutcomeCommercialAnswer, Reason: ReasonOkCommercial,
		RuleVersion: PolicyRuleVersion, Intent: IntentPriceInquiry,
		ActorID: newID(), OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(evaluated), "candidate") || strings.Contains(string(evaluated), "value_digest") {
		t.Fatalf("the evaluation envelope carries the state words only")
	}
}
