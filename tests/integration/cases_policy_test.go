//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 2.6 matrix: the typed proposals, the deterministic outcomes
// with the four anchors, the dedup against the confirmed facts, the
// load-bearing sales reply gate, the reproducibility digests, the
// idempotency and the races — over the real world of the cases contour.

// policyView reads the policy section of the case view.
func (w *casesWorld) policyView(t *testing.T, caseID id.UUID) casesAPI.PolicyView {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	return detail.Policy
}

// policyAnchors resolves the current anchor words of the case view (the
// honest pre-read of the evaluation commands).
func (w *casesWorld) policyAnchors(t *testing.T, caseID, workItemID id.UUID) (modeVersion, ownerEpoch, workItemVersion int64, factsDigest string) {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range detail.WorkItems {
		if item.WorkItemID == workItemID {
			workItemVersion = item.Version
		}
	}
	return detail.Policy.ModeVersion, detail.Policy.OwnerEpoch, workItemVersion, detail.Policy.FactsDigest
}

// policySubmit is the submission shorthand of the tests.
func (w *casesWorld) policySubmit(t *testing.T, command casesAPI.SubmitAIProposalCommand) casesAPI.WorkOutcome {
	t.Helper()
	outcome, err := w.cases.SubmitAIProposal(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// policyEvaluate is the evaluation shorthand of the tests (the honest
// error return — the anchor conflicts are the matrix rows).
func (w *casesWorld) policyEvaluate(t *testing.T, command casesAPI.EvaluatePolicyCommand) (casesAPI.WorkOutcome, error) {
	t.Helper()
	return w.cases.EvaluatePolicy(context.Background(), command)
}

// policyEvaluateOK is the evaluation shorthand that must pass.
func (w *casesWorld) policyEvaluateOK(t *testing.T, command casesAPI.EvaluatePolicyCommand) casesAPI.WorkOutcome {
	t.Helper()
	outcome, err := w.cases.EvaluatePolicy(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// recordConfirmedFact records and confirms one fact of the case (the
// confirmed exact version — the dedup basis).
func (w *casesWorld) recordConfirmedFact(t *testing.T, caseID, workItemID id.UUID, key, value, evidenceEvent string, keyNo string) {
	t.Helper()
	recorded, err := w.cases.RecordFact(context.Background(), casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: key, Value: []byte(value),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: eventIDOf(t, w, caseID, evidenceEvent).String(),
		Reason: "Клиент назвал значение", IdempotencyKey: "policy-fact-" + keyNo, Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := w.cases.ConfirmFactVersion(context.Background(), casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: recorded.FactID,
		VersionNo: recorded.FactVersion, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		EvidenceEventID: eventIDOf(t, w, caseID, evidenceEvent),
		IdempotencyKey:  "policy-confirm-" + keyNo, Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.FactStatus != casesDomain.FactStatusConfirmed {
		t.Fatalf("expected the confirmed fact, got %+v", confirmed)
	}
}

// policySalesPrelude opens the sales mode with the acting disclosure of
// the channel (the 2-5 prelude of the policy demo path).
func (w *casesWorld) policySalesPrelude(t *testing.T, caseID, conversationID id.UUID, turn id.UUID, key string) {
	t.Helper()
	if _, err := w.cases.ProposeMode(context.Background(), casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "клиент интересуется ценой", SignalSource: casesDomain.ModeSignalTurnEvent, TurnRef: turn,
		IdempotencyKey: key + "-sales", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	disclosure, err := w.cases.RecordDisclosure(context.Background(), casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Я цифровой сотрудник компании. Человек подключится по вашему запросу.",
		IdempotencyKey: key + "-disclosure", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.cases.RecordDisclosureEvent(context.Background(), casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: disclosure.DisclosureID, Kind: casesDomain.DeliverySent,
		IdempotencyKey: key + "-sent", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestPolicyDemoPathDedupGateSupersede covers the owner-approved demo
// chain: the port submits the proposal (the service_kind fact is already
// confirmed); the evaluation with the anchors produces the v1 commercial
// outcome with the DEDUP (the service_kind question is suppressed and
// reused with the provenance, the urgency question stays); the sales
// reply refuses without the Work Item anchor, refuses without an
// outcome, passes with the anchor and the acting outcome; the urgency
// fact lands and confirms; the re-evaluation with the stale anchor
// refuses with the honest conflict, with the fresh anchors it supersedes
// by the v2 row (v1 stays readable, the current is v2).
func TestPolicyDemoPathDedupGateSupersede(t *testing.T) {
	w := newCasesWorld(t, 760)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-policy-1", "sess-policy-1", `{"call":"inbound-call","from":"+79000000080"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-1", "evt-policy-1-turn-1", `{"say":"сколько стоит диагностика"}`)

	// The service_kind fact is confirmed BEFORE the proposal (the dedup
	// basis — the confirmed exact version).
	w.recordConfirmedFact(t, caseID, workItemID, "service_kind", `"диагностика"`, "evt-policy-1", "1")

	// The sales prelude: the classification + the acting disclosure.
	w.policySalesPrelude(t, caseID, conversationID, turn, "policy-demo-1")

	// The port submits the proposal: the price inquiry with the candidate
	// and the two missing-fact claims + one extracted claim (the audit
	// span over the accepted turn).
	proposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Диагностика от 3000",
		MissingFacts: []casesDomain.MissingFact{
			{MaterialKey: "service_kind", Reason: casesDomain.MissingReasonMandatory},
			{MaterialKey: "urgency", Reason: casesDomain.MissingReasonMandatory},
		},
		ExtractedFacts: []casesDomain.ExtractedFact{
			{MaterialKey: "service_kind", ValueDigest: "claim-digest", SourceEventID: turn},
		},
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-proposal-1", Now: ownershipNow,
	})
	if proposal.Replay || !proposal.ProposalID.Valid() {
		t.Fatalf("expected the fresh proposal, got %+v", proposal)
	}

	// The exact repeat replays the stored outcome; the same key with
	// another material is the honest key conflict (AC-10).
	replay, err := w.cases.SubmitAIProposal(ctx, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Диагностика от 3000",
		MissingFacts: []casesDomain.MissingFact{
			{MaterialKey: "service_kind", Reason: casesDomain.MissingReasonMandatory},
			{MaterialKey: "urgency", Reason: casesDomain.MissingReasonMandatory},
		},
		ExtractedFacts: []casesDomain.ExtractedFact{
			{MaterialKey: "service_kind", ValueDigest: "claim-digest", SourceEventID: turn},
		},
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-proposal-1", Now: ownershipNow,
	})
	if err != nil || !replay.Replay || replay.ProposalID != proposal.ProposalID {
		t.Fatalf("expected the replayed proposal, got %+v %v", replay, err)
	}
	_, err = w.cases.SubmitAIProposal(ctx, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Другой кандидат",
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-proposal-1", Now: ownershipNow,
	})
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("expected the command key conflict, got %v", err)
	}

	// The evaluation with the CURRENT anchors: the v1 commercial outcome
	// with the dedup (the confirmed service_kind answers its question —
	// the suppressed repeat and the reused provenance; the urgency
	// question stays in the plan).
	modeVersion, ownerEpoch, workItemVersion, factsDigest := w.policyAnchors(t, caseID, workItemID)
	evaluateCommand := casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "policy-evaluate-1", Now: ownershipNow,
	}
	outcome := w.policyEvaluateOK(t, evaluateCommand)
	if outcome.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
		t.Fatalf("expected the commercial outcome, got %+v", outcome)
	}

	// The exact repeat replays the stored outcome; the same key with
	// another anchors material is the honest key conflict (the anchors
	// ARE the request-hash material — the m-r2-1 lesson).
	replayed, err := w.cases.EvaluatePolicy(ctx, evaluateCommand)
	if err != nil || !replayed.Replay || replayed.OutcomeID != outcome.OutcomeID {
		t.Fatalf("expected the replayed evaluation, got %+v %v", replayed, err)
	}
	otherAnchors := evaluateCommand
	otherAnchors.ExpectedModeVersion = modeVersion + 1
	_, err = w.cases.EvaluatePolicy(ctx, otherAnchors)
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("expected the anchor key conflict, got %v", err)
	}

	// The view: the v1 outcome with the dedup material.
	view := w.policyView(t, caseID)
	section := policySectionOf(t, view, workItemID)
	if len(section.Outcomes) != 1 || section.Current == nil || section.Current.OutcomeNo != 1 {
		t.Fatalf("expected the one v1 current outcome, got %+v", section.Outcomes)
	}
	current := section.Current
	if current.OutcomeClass != casesDomain.OutcomeCommercialAnswer || current.Reason != casesDomain.ReasonOkCommercial || current.RuleVersion != casesDomain.PolicyRuleVersion {
		t.Fatalf("expected the ok_commercial v1 words, got %+v", current)
	}
	// The dedup: service_kind reused with the provenance, suppressed; the
	// urgency question stays; the next steps carry the question + the
	// confirmation.
	reusedOf := map[string]casesDomain.ReusedFact{}
	for _, reused := range current.ReusedFacts {
		reusedOf[reused.MaterialKey] = reused
	}
	serviceKind, ok := reusedOf["service_kind"]
	if !ok || !serviceKind.FactID.Valid() || serviceKind.VersionNo != 1 || serviceKind.ValueDigest == "" {
		t.Fatalf("expected the reused service_kind provenance, got %+v", current.ReusedFacts)
	}
	suppressedOf := map[string]bool{}
	for _, suppressed := range current.SuppressedQuestions {
		suppressedOf[suppressed.MaterialKey] = true
	}
	if !suppressedOf["service_kind"] || suppressedOf["urgency"] {
		t.Fatalf("expected only the service_kind suppressed repeat, got %+v", current.SuppressedQuestions)
	}
	statesOf := map[string]string{}
	for _, fact := range current.MandatoryFacts {
		statesOf[fact.MaterialKey] = fact.State
	}
	if statesOf["service_kind"] != casesDomain.MandatoryStateReused || statesOf["urgency"] != casesDomain.MandatoryStateAsk {
		t.Fatalf("expected reused/ask states, got %+v", current.MandatoryFacts)
	}
	steps := map[string]string{}
	for _, step := range current.NextSteps {
		steps[step.Step] = step.MaterialKey
	}
	if steps[casesDomain.StepAskFact] != "urgency" {
		t.Fatalf("expected the urgency question, got %+v", current.NextSteps)
	}
	awaiting := false
	for _, step := range current.NextSteps {
		if step.Step == casesDomain.StepAwaitConfirmation {
			awaiting = true
		}
	}
	if !awaiting {
		t.Fatalf("the commercial answer awaits the confirmation, got %+v", current.NextSteps)
	}

	// The sales reply WITHOUT the Work Item anchor: the machine-readable
	// refusal with the anchor path (the 2-4/2-5 gates fired earlier —
	// the ownership, the mode and the disclosure are green here).
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Диагностика от 3000", IdempotencyKey: "policy-sales-noanchor-1", Now: ownershipNow,
	})
	var policyGate casesDomain.ReplyPolicyError
	if !errors.As(err, &policyGate) || policyGate.PermittedPath != casesDomain.PermittedPathAnchorWorkItem {
		t.Fatalf("expected the anchor path refusal, got %v", err)
	}

	// The sales reply with the anchor and the ACTING outcome: passes (the
	// ownership → mode → disclosure → policy order; the record references
	// the outcome).
	sales, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Диагностика от 3000; уточню срочность", IdempotencyKey: "policy-sales-ok-1", Now: ownershipNow,
	})
	if err != nil || sales.ReplyState != "prepared" {
		t.Fatalf("expected the sales reply to pass, got %+v %v", sales, err)
	}
	if !sales.PolicyOutcomeID.Valid() || sales.PolicyOutcomeID != current.OutcomeID {
		t.Fatalf("the reply references the acting outcome, got %+v", sales)
	}

	// The urgency fact lands and confirms (the operator asked the
	// question with the neutral class; the client answered in a turn).
	w.modeTurnEvent(t, caseID, "sess-policy-1", "evt-policy-1-turn-2", `{"say":"срочно"}`)
	w.recordConfirmedFact(t, caseID, workItemID, "urgency", `"срочно"`, "evt-policy-1-turn-2", "2")

	// The re-evaluation with the STALE facts digest: the honest conflict
	// with the current words (the operator re-reads and re-anchors).
	stale := evaluateCommand
	stale.IdempotencyKey = "policy-evaluate-2"
	var conflict casesDomain.PolicyConflictError
	_, err = w.cases.EvaluatePolicy(ctx, stale)
	if !errors.As(err, &conflict) || conflict.CurrentFactsDigest == factsDigest {
		t.Fatalf("expected the stale facts digest conflict with the current words, got %v", err)
	}

	// The fresh anchors: the v2 outcome — no questions, v1 stays readable,
	// the current is v2 (the supersede by the new row).
	modeVersion2, ownerEpoch2, workItemVersion2, factsDigest2 := w.policyAnchors(t, caseID, workItemID)
	outcome2 := w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion2, ExpectedOwnerEpoch: ownerEpoch2,
		ExpectedFactsDigest: factsDigest2, ExpectedWorkItemVersion: workItemVersion2,
		IdempotencyKey: "policy-evaluate-3", Now: ownershipNow,
	})
	if outcome2.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
		t.Fatalf("expected the v2 commercial outcome, got %+v", outcome2)
	}
	view2 := w.policyView(t, caseID)
	section2 := policySectionOf(t, view2, workItemID)
	if len(section2.Outcomes) != 2 || section2.Current == nil || section2.Current.OutcomeNo != 2 {
		t.Fatalf("expected the v2 current outcome with the readable v1, got %+v", section2.Outcomes)
	}
	if len(section2.Current.MandatoryFacts) != 2 || len(section2.Current.ReusedFacts) != 2 {
		t.Fatalf("expected both facts reused in v2, got %+v", section2.Current)
	}

	// The determinism: the repeated evaluation with the SAME anchors (the
	// world did not change) writes the new row with the SAME decision
	// digest (the reproducibility by the input versions, AC-3).
	outcome3 := w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion2, ExpectedOwnerEpoch: ownerEpoch2,
		ExpectedFactsDigest: factsDigest2, ExpectedWorkItemVersion: workItemVersion2,
		IdempotencyKey: "policy-evaluate-4", Now: ownershipNow,
	})
	view3 := w.policyView(t, caseID)
	section3 := policySectionOf(t, view3, workItemID)
	if len(section3.Outcomes) != 3 || section3.Current.OutcomeNo != 3 {
		t.Fatalf("expected the third row, got %+v", section3.Outcomes)
	}
	if section3.Current.DecisionDigest != section2.Current.DecisionDigest {
		t.Fatalf("the same anchors produce the same decision digest, got %q vs %q",
			section3.Current.DecisionDigest, section2.Current.DecisionDigest)
	}
	if outcome3.OutcomeID == outcome2.OutcomeID {
		t.Fatalf("the repeated evaluation is a NEW row, got %+v", outcome3)
	}

	// The replay of the passed sales reply: the idempotent replay of the
	// stored outcome (the original outcome — AC-10).
	replied, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Диагностика от 3000; уточню срочность", IdempotencyKey: "policy-sales-ok-1", Now: ownershipNow,
	})
	if err != nil || !replied.Replay || replied.ReplyID != sales.ReplyID {
		t.Fatalf("expected the replayed reply, got %+v %v", replied, err)
	}
}

// policySectionOf resolves the Work Item section of the policy view.
func policySectionOf(t *testing.T, view casesAPI.PolicyView, workItemID id.UUID) casesAPI.PolicyWorkItemView {
	t.Helper()
	for _, section := range view.Items {
		if section.WorkItemID == workItemID {
			return section
		}
	}
	t.Fatalf("the policy view carries the section of the work item")
	return casesAPI.PolicyWorkItemView{}
}

// TestPolicyOutcomeRequiredAndNotCommercial covers the gate refusals of
// the sales reply: the Work Item without any outcome (the permitted path
// names the submission first, the evaluation when the proposal exists);
// the non-commercial current outcome (the clarify of the empty candidate)
// refuses with the current outcome words and the evaluation path.
func TestPolicyOutcomeRequiredAndNotCommercial(t *testing.T) {
	w := newCasesWorld(t, 761)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-policy-2", "sess-policy-2", `{"call":"inbound-call","from":"+79000000081"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-2", "evt-policy-2-turn-1", `{"say":"хочу заказать"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "policy-gate-1")

	// The sales reply with the anchor but WITHOUT any outcome: the
	// permitted path names the SUBMISSION first (no proposal exists).
	_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю", IdempotencyKey: "policy-gate-none-1", Now: ownershipNow,
	})
	var policyGate casesDomain.ReplyPolicyError
	if !errors.As(err, &policyGate) || policyGate.PermittedPath != casesDomain.PermittedPathSubmitProposal {
		t.Fatalf("expected the submit_ai_proposal permitted path, got %v", err)
	}

	// The proposal without the outcome: the permitted path names the
	// EVALUATION.
	proposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Диагностика от 3000",
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-gate-proposal-1", Now: ownershipNow,
	})
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю", IdempotencyKey: "policy-gate-unevaluated-1", Now: ownershipNow,
	})
	if !errors.As(err, &policyGate) || policyGate.PermittedPath != casesDomain.PermittedPathEvaluatePolicy {
		t.Fatalf("expected the evaluate_policy permitted path, got %v", err)
	}

	// The clarify outcome of the EMPTY candidate: the sales reply refuses
	// with the current outcome words (the class, the reason, the rule
	// version, the number).
	emptyCandidate := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "",
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-gate-proposal-2", Now: ownershipNow,
	})
	modeVersion, ownerEpoch, workItemVersion, factsDigest := w.policyAnchors(t, caseID, workItemID)
	outcome := w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: emptyCandidate.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "policy-gate-evaluate-1", Now: ownershipNow,
	})
	if outcome.OutcomeClass != casesDomain.OutcomeClarify {
		t.Fatalf("expected the clarify outcome of the empty candidate, got %+v", outcome)
	}
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю", IdempotencyKey: "policy-gate-clarify-1", Now: ownershipNow,
	})
	if !errors.As(err, &policyGate) || policyGate.CurrentClass != casesDomain.OutcomeClarify ||
		policyGate.CurrentReason != casesDomain.ReasonNoAnswerBasis || policyGate.OutcomeNo != 1 ||
		policyGate.RuleVersion != casesDomain.PolicyRuleVersion {
		t.Fatalf("expected the clarify current-outcome refusal words, got %+v", policyGate)
	}
	if policyGate.PermittedPath != casesDomain.PermittedPathEvaluatePolicy {
		t.Fatalf("expected the evaluation permitted path, got %+v", policyGate)
	}

	// The commercial outcome of the first proposal supersedes the clarify:
	// the current flips, the sales reply passes (the acting outcome).
	_ = proposal
	outcome2 := w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "policy-gate-evaluate-2", Now: ownershipNow,
	})
	if outcome2.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
		t.Fatalf("expected the commercial supersede, got %+v", outcome2)
	}
	sales, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Диагностика от 3000", IdempotencyKey: "policy-gate-sales-1", Now: ownershipNow,
	})
	if err != nil || sales.ReplyState != "prepared" {
		t.Fatalf("expected the sales reply to pass after the supersede, got %+v %v", sales, err)
	}

	// The neutral replies never meet the policy gate (the explicit v1
	// rule of 2-5 — the safe independent work).
	neutral, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "Принято в работу", IdempotencyKey: "policy-gate-neutral-1", Now: ownershipNow,
	})
	if err != nil || neutral.ReplyState != "prepared" {
		t.Fatalf("expected the neutral reply to pass without the policy gate, got %+v %v", neutral, err)
	}
}

// TestPolicyUnknownIntentFailsClosed covers the honest boundary of the
// intent dictionary: the submission accepts the unknown intent word (a
// bounded string — the dictionary is a POLICY matter); the evaluation
// fails closed with the human handoff step; the sales reply against the
// fail_closed outcome refuses (only commercial_answer opens the sales
// path).
func TestPolicyUnknownIntentFailsClosed(t *testing.T) {
	w := newCasesWorld(t, 762)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-policy-3", "sess-policy-3", `{"call":"inbound-call","from":"+79000000082"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-3", "evt-policy-3-turn-1", `{"say":"что-то непонятное"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "policy-unknown-1")

	// The unknown intent passes the SUBMISSION (the model result is a
	// proposal, not a Policy Outcome).
	proposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: "urgent_magic", CandidateAnswer: "Непонятно, передаю человеку",
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-unknown-proposal-1", Now: ownershipNow,
	})
	if !proposal.ProposalID.Valid() {
		t.Fatalf("the unknown intent reaches the store (the claim audit), got %+v", proposal)
	}

	// The evaluation fails closed with the human handoff.
	modeVersion, ownerEpoch, workItemVersion, factsDigest := w.policyAnchors(t, caseID, workItemID)
	outcome := w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "policy-unknown-evaluate-1", Now: ownershipNow,
	})
	if outcome.OutcomeClass != casesDomain.OutcomeFailClosed {
		t.Fatalf("expected the fail_closed outcome, got %+v", outcome)
	}
	view := w.policyView(t, caseID)
	current := policySectionOf(t, view, workItemID).Current
	if current.Reason != casesDomain.ReasonUnknownIntent || len(current.NextSteps) != 1 || current.NextSteps[0].Step != casesDomain.StepHumanHandoff {
		t.Fatalf("expected the unknown_intent handoff plan, got %+v", current)
	}
	if len(current.MandatoryFacts) != 0 {
		t.Fatalf("the unknown intent carries no question plan, got %+v", current.MandatoryFacts)
	}

	// The sales reply against the fail_closed outcome: the refusal with
	// the current words.
	_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Всё продаю", IdempotencyKey: "policy-unknown-sales-1", Now: ownershipNow,
	})
	var policyGate casesDomain.ReplyPolicyError
	if !errors.As(err, &policyGate) || policyGate.CurrentClass != casesDomain.OutcomeFailClosed {
		t.Fatalf("expected the fail_closed gate refusal, got %v", err)
	}
}

// TestPolicyModeBlockedOutcome covers the commercial intent under the
// non-sales effective mode: the outcome is mode_blocked (the mode words
// decide; the sales reply is refused by the MODE gate earlier — the
// frozen order never changes); the informational intent of another Work
// Item under the same mode produces informational_answer (the multi-WI
// scope, AC-7).
func TestPolicyModeBlockedOutcome(t *testing.T) {
	w := newCasesWorld(t, 763)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-policy-4", "sess-policy-4", `{"call":"inbound-call","from":"+79000000083"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-4", "evt-policy-4-turn-1", `{"say":"есть вопрос по заказу"}`)

	// NO sales classification: the mode stays unclassified (the honest
	// non-sales word). The proposal still lands (the audit).
	proposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Диагностика от 3000",
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-blocked-proposal-1", Now: ownershipNow,
	})
	modeVersion, ownerEpoch, workItemVersion, factsDigest := w.policyAnchors(t, caseID, workItemID)
	outcome := w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "policy-blocked-evaluate-1", Now: ownershipNow,
	})
	if outcome.OutcomeClass != casesDomain.OutcomeModeBlocked {
		t.Fatalf("expected the mode_blocked outcome under the unclassified mode, got %+v", outcome)
	}
	view := w.policyView(t, caseID)
	current := policySectionOf(t, view, workItemID).Current
	if current.Reason != casesDomain.ReasonModeBlocksCommercial {
		t.Fatalf("expected the mode_blocks_commercial_path reason, got %+v", current)
	}

	// The sales reply under the unclassified mode: the MODE gate (2-5)
	// refuses EARLIER — the policy gate never fires (the frozen order).
	_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю", IdempotencyKey: "policy-blocked-sales-1", Now: ownershipNow,
	})
	var modeGate casesDomain.ReplyModeError
	if !errors.As(err, &modeGate) || modeGate.EffectiveMode != casesDomain.ModeUnclassified {
		t.Fatalf("expected the EARLIER mode gate refusal, got %v", err)
	}

	// The second Work Item with the informational intent: the
	// informational_answer outcome (the WI-scope, the multi-intent
	// boundary — the separate proposal, AC-7).
	second := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentGeneralQuestion, CandidateAnswer: "Часы работы: 9-18",
		Proposer: casesDomain.ProposerOperator, IdempotencyKey: "policy-blocked-proposal-2", Now: ownershipNow,
	})
	secondOutcome := w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: second.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "policy-blocked-evaluate-2", Now: ownershipNow,
	})
	if secondOutcome.OutcomeClass != casesDomain.OutcomeInformationalAnswer {
		t.Fatalf("expected the informational outcome, got %+v", secondOutcome)
	}
	// The neutral reply passes (the informational path is the neutral
	// work of 2-5 — the mode gate allows it by the explicit v1 rule).
	neutral, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "Часы работы: 9-18", IdempotencyKey: "policy-blocked-neutral-1", Now: ownershipNow,
	})
	if err != nil || neutral.ReplyState != "prepared" {
		t.Fatalf("expected the neutral informational reply, got %+v %v", neutral, err)
	}
}

// TestPolicySchemaFieldProblems covers the submission discipline: the
// structure and the bounds problems accumulate as the machine-readable
// field errors (the intent length, the unknown reason word, the
// unnormalized key, the duplicate key, the bad proposer word, the missing
// event reference); the repeat of the invalid submission never creates
// records (AC-4 edge).
func TestPolicySchemaFieldProblems(t *testing.T) {
	w := newCasesWorld(t, 764)
	caseID, workItemID := w.acceptedCase(t, "evt-policy-5", "sess-policy-5", `{"call":"inbound-call","from":"+79000000084"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-5", "evt-policy-5-turn-1", `{"say":"привет"}`)

	_, err := w.cases.SubmitAIProposal(context.Background(), casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: strings.Repeat("н", 65), CandidateAnswer: "Кандидат",
		MissingFacts: []casesDomain.MissingFact{
			{MaterialKey: "Service Kind", Reason: "maybe"},
			{MaterialKey: "Service Kind", Reason: "mandatory"},
		},
		ExtractedFacts: []casesDomain.ExtractedFact{{MaterialKey: "к", ValueDigest: strings.Repeat("d", 129)}},
		Proposer:       "machine", IdempotencyKey: "policy-schema-1", Now: ownershipNow,
	})
	var schema casesDomain.ProposalSchemaError
	if !errors.As(err, &schema) {
		t.Fatalf("expected the schema error, got %v", err)
	}
	fields := map[string]string{}
	for _, problem := range schema.Problems {
		fields[problem.Field] = problem.Problem
	}
	for _, field := range []string{
		"intent", "proposer", "missing_facts[0].material_key", "missing_facts[0].reason",
		"missing_facts[1].material_key", "extracted_facts[0].source_event_id", "extracted_facts[0].value_digest",
	} {
		if _, ok := fields[field]; !ok {
			t.Fatalf("expected the field problem %q, got %+v", field, schema.Problems)
		}
	}
	// The invalid submission created no records (the view stays empty).
	view := w.policyView(t, caseID)
	if section := policySectionOf(t, view, workItemID); len(section.Proposals) != 0 {
		t.Fatalf("the schema-invalid submission writes nothing, got %+v", section.Proposals)
	}
}

// TestPolicyDedupConflictingUnverifiedReadback covers the dedup edges:
// the conflicting fact keeps the question (the conflict is a re-ask
// reason); the unverified fact keeps the question; the explicit readback
// keeps the question even when the value is confirmed (AC-6).
func TestPolicyDedupConflictingUnverifiedReadback(t *testing.T) {
	w := newCasesWorld(t, 765)
	caseID, workItemID := w.acceptedCase(t, "evt-policy-6", "sess-policy-6", `{"call":"inbound-call","from":"+79000000085"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-6", "evt-policy-6-turn-1", `{"say":"хочу деталь"}`)

	// The conflicting fact: two versions without a confirmation.
	w.recordConfirmedFact(t, caseID, workItemID, "service_kind", `"диагностика"`, "evt-policy-6", "1")
	_, err := w.cases.RecordFact(context.Background(), casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "service_kind", Value: []byte(`"ремонт"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "policy-conflict-2",
		Reason: "Другое значение", IdempotencyKey: "policy-fact-conflict-2", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// The unconfirmed (unverified) fact.
	w.modeTurnEvent(t, caseID, "sess-policy-6", "evt-policy-6-turn-2", `{"say":"срочно"}`)
	_, err = w.cases.RecordFact(context.Background(), casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "urgency", Value: []byte(`"срочно"`),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: eventIDOf(t, w, caseID, "evt-policy-6-turn-2").String(),
		Reason: "Клиент назвал срочность", IdempotencyKey: "policy-fact-unverified-1", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// The confirmed slot_preference with the explicit READBACK claim.
	w.modeTurnEvent(t, caseID, "sess-policy-6", "evt-policy-6-turn-3", `{"say":"утром"}`)
	w.recordConfirmedFact(t, caseID, workItemID, "slot_preference", `"утром"`, "evt-policy-6-turn-3", "3")

	w.policySalesPrelude(t, caseID, conversationID, turn, "policy-dedup-1")
	// The price_inquiry plan carries the rule keys (the conflicting
	// service_kind, the unverified urgency) plus the explicit readback
	// claim of the CONFIRMED slot_preference (the third re-ask word).
	proposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Есть окно в четверг утром",
		MissingFacts: []casesDomain.MissingFact{
			{MaterialKey: "slot_preference", Reason: casesDomain.MissingReasonReadback},
		},
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-dedup-proposal-1", Now: ownershipNow,
	})
	modeVersion, ownerEpoch, workItemVersion, factsDigest := w.policyAnchors(t, caseID, workItemID)
	outcome := w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "policy-dedup-evaluate-1", Now: ownershipNow,
	})
	if outcome.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
		t.Fatalf("expected the commercial outcome, got %+v", outcome)
	}
	view := w.policyView(t, caseID)
	current := policySectionOf(t, view, workItemID).Current
	states := map[string]string{}
	for _, fact := range current.MandatoryFacts {
		states[fact.MaterialKey] = fact.State
	}
	// The conflicting service_kind keeps its question; the unverified
	// urgency keeps its question; the confirmed slot_preference with the
	// explicit readback keeps its question too.
	for _, key := range []string{"service_kind", "urgency", "slot_preference"} {
		if states[key] != casesDomain.MandatoryStateAsk {
			t.Fatalf("the %s question stays (conflict/unverified/readback), got %+v", key, current.MandatoryFacts)
		}
	}
	if len(current.ReusedFacts) != 0 || len(current.SuppressedQuestions) != 0 {
		t.Fatalf("no reuse under the conflict/unverified/readback words, got %+v %+v", current.ReusedFacts, current.SuppressedQuestions)
	}
}

// TestPolicyTerminalWorkItemAndRaces covers the honest boundaries: the
// evaluation of the terminal Work Item refuses (the work rule); the
// concurrent evaluations with the same anchors produce the monotonic
// numbers with the identical digests; the stale anchor under the lock is
// the honest conflict.
func TestPolicyTerminalWorkItemAndRaces(t *testing.T) {
	w := newCasesWorld(t, 766)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-policy-7", "sess-policy-7", `{"call":"inbound-call","from":"+79000000086"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-7", "evt-policy-7-turn-1", `{"say":"хочу деталь"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "policy-races-1")
	proposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Диагностика от 3000",
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-races-proposal-1", Now: ownershipNow,
	})

	// The terminal Work Item: the resolved state refuses the evaluation.
	classifyBase := w.workBaseInput(caseID, workItemID, 1)
	classifyBase.IdempotencyKey = "policy-terminal-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: classifyBase, Type: "поиск детали", Title: "Подбор детали",
	}); err != nil {
		t.Fatal(err)
	}
	transitionBase := w.workBaseInput(caseID, workItemID, 2)
	transitionBase.IdempotencyKey = "policy-terminal-transition-1"
	transitionBase.Reason = "Задача закрыта"
	if _, err := w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{
		WorkCommandInput: transitionBase, ToState: casesDomain.WorkItemStateResolved,
	}); err != nil {
		t.Fatal(err)
	}
	modeVersion, ownerEpoch, workItemVersion, factsDigest := w.policyAnchors(t, caseID, workItemID)
	_, err := w.cases.EvaluatePolicy(ctx, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "policy-terminal-evaluate-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "Терминальный элемент работы не оценивается")

	// The race material: the second case with the alive item.
	raceCase, raceItem := w.acceptedCase(t, "evt-policy-8", "sess-policy-8", `{"call":"inbound-call","from":"+79000000087"}`)
	raceConversation := w.conversationFirst(t, raceCase)
	raceTurn := w.modeTurnEvent(t, raceCase, "sess-policy-8", "evt-policy-8-turn-1", `{"say":"хочу деталь"}`)
	w.policySalesPrelude(t, raceCase, raceConversation, raceTurn, "policy-races-2")
	raceProposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: raceCase,
		ConversationID: raceConversation, WorkItemID: raceItem, TurnRef: raceTurn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Диагностика от 3000",
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-races-proposal-2", Now: ownershipNow,
	})
	raceMode, raceEpoch, raceVersion, raceDigest := w.policyAnchors(t, raceCase, raceItem)

	// The concurrent evaluations: both anchors match, both write — the
	// monotonic numbers 1 and 2, the identical decision digests.
	const racers = 4
	results := make([]casesAPI.WorkOutcome, racers)
	errs := make([]error, racers)
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = w.cases.EvaluatePolicy(context.Background(), casesAPI.EvaluatePolicyCommand{
				Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: raceCase, ProposalID: raceProposal.ProposalID,
				ExpectedModeVersion: raceMode, ExpectedOwnerEpoch: raceEpoch,
				ExpectedFactsDigest: raceDigest, ExpectedWorkItemVersion: raceVersion,
				IdempotencyKey: "policy-races-evaluate-" + string(rune('a'+i)), Now: ownershipNow,
			})
		}(i)
	}
	wg.Wait()
	for i := 0; i < racers; i++ {
		if errs[i] != nil {
			t.Fatalf("racer %d failed honestly: %v", i, errs[i])
		}
	}
	view := w.policyView(t, raceCase)
	section := policySectionOf(t, view, raceItem)
	if len(section.Outcomes) != racers {
		t.Fatalf("expected the %d monotonic rows, got %d", racers, len(section.Outcomes))
	}
	digests := map[string]bool{}
	numbers := map[int64]bool{}
	for _, outcome := range section.Outcomes {
		digests[outcome.DecisionDigest] = true
		numbers[outcome.OutcomeNo] = true
	}
	if len(digests) != 1 {
		t.Fatalf("the same anchors produce the identical digests, got %d distinct", len(digests))
	}
	for no := int64(1); no <= racers; no++ {
		if !numbers[no] {
			t.Fatalf("the monotonic numbers are dense, missing %d", no)
		}
	}

	// The stale anchor: the moved mode ladder refuses with the current
	// words (the 409 conflict — the re-read and the re-anchor path).
	if _, err := w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: raceCase, ConversationID: raceConversation,
		Mode: casesDomain.ModeHumanRequested, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 2,
		Reason: "клиент просит человека", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "policy-races-mode-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	var conflict casesDomain.PolicyConflictError
	_, err = w.cases.EvaluatePolicy(ctx, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: raceCase, ProposalID: raceProposal.ProposalID,
		ExpectedModeVersion: raceMode, ExpectedOwnerEpoch: raceEpoch,
		ExpectedFactsDigest: raceDigest, ExpectedWorkItemVersion: raceVersion,
		IdempotencyKey: "policy-races-evaluate-stale-1", Now: ownershipNow,
	})
	if !errors.As(err, &conflict) || conflict.CurrentModeVersion != 3 {
		t.Fatalf("expected the moved ladder conflict with the current words, got %v", err)
	}
}

// TestPolicyForeignTenantIsolation covers the tenant isolation: the
// commands of the foreign tenant see the plain 404 (the existence is
// never disclosed).
func TestPolicyForeignTenantIsolation(t *testing.T) {
	w := newCasesWorld(t, 767)
	caseID, workItemID := w.acceptedCase(t, "evt-policy-9", "sess-policy-9", `{"call":"inbound-call","from":"+79000000088"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-9", "evt-policy-9-turn-1", `{"say":"хочу деталь"}`)

	// The foreign tenant gets the plain 404 (the tenant existence is
	// never disclosed — the RLS + the server-side authorization).
	_, err := w.cases.SubmitAIProposal(context.Background(), casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: id.MustParse("01994f64-6c00-7000-8000-0000000000f1"),
		CaseID: caseID, ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Чужой кандидат",
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-foreign-1", Now: ownershipNow,
	})
	if !errors.Is(err, casesDomain.ErrNotFound) && !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("expected the honest not-found of the foreign tenant, got %v", err)
	}
}

// TestPolicyTablesRLSForced asserts the append-only discipline of the
// policy surface at the database level: RLS + FORCE on both tables (the
// mode precedent of 2-5).
func TestPolicyTablesRLSForced(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('ai_proposals','policy_outcomes')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string][2]bool{}
	for rows.Next() {
		var name string
		var rls, force bool
		if err = rows.Scan(&name, &rls, &force); err != nil {
			t.Fatal(err)
		}
		seen[name] = [2]bool{rls, force}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"ai_proposals", "policy_outcomes"} {
		state, ok := seen[table]
		if !ok || !state[0] || !state[1] {
			t.Fatalf("%s must carry RLS + FORCE (the append-only discipline), got %+v", table, state)
		}
	}
}

// TestPolicyGrantsAppendOnly asserts the grant discipline: the policy
// tables carry INSERT/SELECT for staffy_app and no UPDATE or DELETE (the
// append-only proposals and outcomes — the mode precedent of 2-5).
func TestPolicyGrantsAppendOnly(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	for _, table := range []string{"ai_proposals", "policy_outcomes"} {
		for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
			var allowed bool
			if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases."+table, grant).Scan(&allowed); err != nil {
				t.Fatal(err)
			}
			if (grant == "INSERT" || grant == "SELECT") != allowed {
				t.Fatalf("%s on cases.%s: allowed=%v, want %v", grant, table, allowed, grant == "INSERT" || grant == "SELECT")
			}
		}
	}
}

// TestPolicyCommandsRefuseAuditOnlyMember covers the rights gate of the
// policy write surface (the R1 Major precedent of 2-5): an audit-only
// member reads the case view but never writes the proposals or the
// outcomes (the frozen Boundaries word: «submit/evaluate — участники с
// review»).
func TestPolicyCommandsRefuseAuditOnlyMember(t *testing.T) {
	w := newCasesWorld(t, 768)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-policy-audit-1", "sess-policy-audit-1", `{"call":"inbound-call","from":"+79000000089"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-audit-1", "evt-policy-audit-1-turn-1", `{"say":"хочу деталь"}`)
	// The member ids avoid the world suffixes (the owner is seed*10+1, the
	// backup is seed*10+8): seed*10+6/9 stay free.
	auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "policy-auditor-768@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Audit}, "000000007686", "000000007689")
	auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()

	_, err := w.cases.SubmitAIProposal(ctx, casesAPI.SubmitAIProposalCommand{
		Actor: auditOnly, TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Аудитор предлагает",
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "policy-audit-submit-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "review или configuration")

	_, err = w.cases.EvaluatePolicy(ctx, casesAPI.EvaluatePolicyCommand{
		Actor: auditOnly, TenantID: w.tenant, CaseID: caseID,
		ProposalID:          id.MustParse("01994f64-6c00-7000-8000-0000000000e1"),
		ExpectedModeVersion: 1, ExpectedOwnerEpoch: 1, ExpectedFactsDigest: "0000000000000000000000000000000000000000000000000000000000000000",
		ExpectedWorkItemVersion: 1, IdempotencyKey: "policy-audit-evaluate-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "review или configuration")

	// Nothing landed: the proposal and the outcome surface stays clean.
	view := w.policyView(t, caseID)
	if section := policySectionOf(t, view, workItemID); len(section.Proposals) != 0 || len(section.Outcomes) != 0 {
		t.Fatalf("the audit-only member writes nothing, got %d proposals and %d outcomes", len(section.Proposals), len(section.Outcomes))
	}
}

// TestPolicyInformationalOutcomeRefusesSalesReply covers the remaining
// matrix row of the sales gate: the sales conversation with the acting
// disclosure, the Work Item anchored, but the CURRENT outcome of that
// item is informational_answer (the informational path never opens the
// sales reply — only commercial_answer does; the refusal carries the
// current outcome words).
func TestPolicyInformationalOutcomeRefusesSalesReply(t *testing.T) {
	w := newCasesWorld(t, 769)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-policy-info-1", "sess-policy-info-1", `{"call":"inbound-call","from":"+79000000090"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-policy-info-1", "evt-policy-info-1-turn-1", `{"say":"какие часы работы"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "policy-info-1")

	// The informational proposal under the SALES conversation: the second
	// Work Item path of the matrix (the WI-scope outcome).
	proposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentGeneralQuestion, CandidateAnswer: "Часы работы: 9-18",
		Proposer: casesDomain.ProposerOperator, IdempotencyKey: "policy-info-proposal-1", Now: ownershipNow,
	})
	modeVersion, ownerEpoch, workItemVersion, factsDigest := w.policyAnchors(t, caseID, workItemID)
	outcome := w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "policy-info-evaluate-1", Now: ownershipNow,
	})
	if outcome.OutcomeClass != casesDomain.OutcomeInformationalAnswer {
		t.Fatalf("expected the informational outcome, got %+v", outcome)
	}

	// The sales reply against the informational current outcome: the
	// machine-readable refusal with the current words (the mode and the
	// disclosure gates pass — the POLICY gate decides; the permitted path
	// is the evaluation).
	_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Всё продаю", IdempotencyKey: "policy-info-sales-1", Now: ownershipNow,
	})
	var policyGate casesDomain.ReplyPolicyError
	if !errors.As(err, &policyGate) || policyGate.CurrentClass != casesDomain.OutcomeInformationalAnswer ||
		policyGate.CurrentReason != casesDomain.ReasonOkInformational || policyGate.OutcomeNo != 1 {
		t.Fatalf("expected the informational current-outcome refusal words, got %+v", policyGate)
	}
	if policyGate.PermittedPath != casesDomain.PermittedPathEvaluatePolicy {
		t.Fatalf("expected the evaluation permitted path, got %+v", policyGate)
	}

	// The neutral reply of the same Work Item passes (the informational
	// path IS the neutral work of 2-5).
	neutral, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "Часы работы: 9-18", IdempotencyKey: "policy-info-neutral-1", Now: ownershipNow,
	})
	if err != nil || neutral.ReplyState != "prepared" {
		t.Fatalf("expected the neutral informational reply, got %+v %v", neutral, err)
	}
	if neutral.PolicyOutcomeID.Valid() {
		t.Fatalf("the neutral reply never references a Policy Outcome, got %+v", neutral)
	}
}
