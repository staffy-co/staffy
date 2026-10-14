package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.6 fixture: the two policy commands and the policy view over
// the REAL domain derivations (GuardSubmitProposal, GuardEvaluate,
// ConfirmedFactView, ComputeFactsDigest, EvaluatePolicy, DecisionDigest,
// CurrentPolicyOutcome) — the fixture mirrors the store through the same
// domain functions, without convenient defaults (the R2 lesson of 2-3).

// fixtureProposalID generates the next deterministic proposal id.
func (f *casesFixture) fixtureProposalID() id.UUID {
	f.proposalSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(60000+f.proposalSeq)))
}

// fixturePolicyOutcomeID generates the next deterministic outcome id.
func (f *casesFixture) fixturePolicyOutcomeID() id.UUID {
	f.policyOutcomeSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(70000+f.policyOutcomeSeq)))
}

// fixturePolicyFactsSnapshot derives the confirmed fact material of the
// case over the REAL derivation (the reuse and the digest basis).
func (f *casesFixture) fixturePolicyFactsSnapshot(caseID id.UUID) (casesDomain.FactWorld, string, casesDomain.FactsDigestMap) {
	world := casesDomain.FactWorld{}
	confirmed := []casesDomain.ConfirmedFact{}
	keys := make([]string, 0, len(f.factAnchors))
	for key := range f.factAnchors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		factID := f.factAnchors[key]
		versions := f.factVersions[factID]
		if len(versions) == 0 || versions[0].CaseID != caseID {
			continue
		}
		materialKey := key
		if bar := strings.LastIndex(key, "|"); bar >= 0 {
			materialKey = key[bar+1:]
		}
		status, versionNo, digest := casesDomain.ConfirmedFactView(versions, f.factConfirms[factID])
		word := casesDomain.FactStatusWord{Status: status}
		if status == casesDomain.FactStatusConfirmed {
			fact := casesDomain.ConfirmedFact{FactID: factID, MaterialKey: materialKey, VersionNo: versionNo, ValueDigest: digest}
			confirmed = append(confirmed, fact)
			factCopy := fact
			word.Confirmed = &factCopy
		}
		world[materialKey] = word
	}
	digest, digestMap, err := casesDomain.ComputeFactsDigest(confirmed)
	if err != nil {
		panic(err)
	}
	return world, digest, digestMap
}

// fixturePolicyProposalOf resolves one proposal of the case (nil when
// absent).
func (f *casesFixture) fixturePolicyProposalOf(caseID, proposalID id.UUID) *casesDomain.AIProposal {
	for i := range f.aiProposals[caseID] {
		if f.aiProposals[caseID][i].ProposalID == proposalID {
			return &f.aiProposals[caseID][i]
		}
	}
	return nil
}

// SubmitAIProposal records one typed proposal (the REAL schema guard; the
// submission snapshot is the fixture-owned epoch and mode version — the
// mirror of the store truth).
func (f *casesFixture) SubmitAIProposal(_ context.Context, command casesAPI.SubmitAIProposalCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("submit_ai_proposal", casesDomain.GuardSubmitProposal(casesDomain.AIProposal{
		ConversationID: command.ConversationID, WorkItemID: command.WorkItemID, TurnRef: command.TurnRef,
		Intent: command.Intent, CandidateAnswer: command.CandidateAnswer,
		MissingFacts: command.MissingFacts, ExtractedFacts: command.ExtractedFacts, Proposer: command.Proposer,
	})); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("submit_ai_proposal", command.IdempotencyKey, struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		WorkItem     string `json:"work_item"`
		TurnRef      string `json:"turn_ref"`
		Intent       string `json:"intent"`
		Candidate    string `json:"candidate_answer"`
		Proposer     string `json:"proposer"`
	}{command.CaseID.String(), command.ConversationID.String(), command.WorkItemID.String(), command.TurnRef.String(),
		command.Intent, command.CandidateAnswer, command.Proposer})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if _, err := f.fixtureModeConversation(command.CaseID, command.ConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !f.fixtureEventOfCase(command.CaseID, command.TurnRef) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("submit_ai_proposal", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	if err := f.fixtureModeWorkItem(command.CaseID, command.WorkItemID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	proposalID := f.fixtureProposalID()
	f.aiProposals[command.CaseID] = append(f.aiProposals[command.CaseID], casesDomain.AIProposal{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: command.ConversationID,
		WorkItemID: command.WorkItemID, ProposalID: proposalID, TurnRef: command.TurnRef,
		SchemaVersion: casesDomain.ProposalSchemaVersion, Intent: command.Intent,
		CandidateAnswer: command.CandidateAnswer, MissingFacts: command.MissingFacts,
		ExtractedFacts: command.ExtractedFacts, Proposer: command.Proposer,
		OwnerEpoch: f.epochOf(command.CaseID), ModeVersion: casesDomain.ModeTermMaxVersion(f.modeTerms[command.CaseID]),
		ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "submit_ai_proposal", ProposalID: proposalID}
	f.storeOutcome("submit_ai_proposal", command.IdempotencyKey, outcome)
	return outcome, nil
}

// EvaluatePolicy runs the REAL deterministic policy over the anchored
// input versions (the moved anchors refuse honestly; the outcome lands
// append-only with the monotonic number and the decision digest).
func (f *casesFixture) EvaluatePolicy(_ context.Context, command casesAPI.EvaluatePolicyCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("evaluate_policy", casesDomain.GuardEvaluate(command.ProposalID, command.ExpectedModeVersion, command.ExpectedOwnerEpoch, command.ExpectedWorkItemVersion, command.ExpectedFactsDigest)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("evaluate_policy", command.IdempotencyKey, struct {
		Case        string `json:"case"`
		Proposal    string `json:"proposal_ref"`
		ModeVersion int64  `json:"expected_mode_version"`
		OwnerEpoch  int64  `json:"expected_owner_epoch"`
		FactsDigest string `json:"expected_facts_digest"`
		WorkVersion int64  `json:"expected_work_item_version"`
	}{command.CaseID.String(), command.ProposalID.String(), command.ExpectedModeVersion,
		command.ExpectedOwnerEpoch, command.ExpectedFactsDigest, command.ExpectedWorkItemVersion})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	proposal := f.fixturePolicyProposalOf(command.CaseID, command.ProposalID)
	if proposal == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current, known := f.workCurrentOf(proposal.WorkItemID); known {
		if current.Terminal {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("evaluate_policy", fmt.Errorf("%s", casesDomain.TerminalWorkItemText))
		}
		if current.Version != command.ExpectedWorkItemVersion {
			_, digest, _ := f.fixturePolicyFactsSnapshot(command.CaseID)
			return casesAPI.WorkOutcome{}, casesDomain.PolicyConflictError{
				CurrentModeVersion: casesDomain.ModeTermMaxVersion(f.modeTerms[command.CaseID]),
				CurrentOwnerEpoch:  f.epochOf(command.CaseID), CurrentFactsDigest: digest,
				CurrentWorkItemVersion: current.Version,
			}
		}
	}
	terms := f.modeTerms[command.CaseID]
	modeVersion := casesDomain.ModeTermMaxVersion(terms)
	if modeVersion != command.ExpectedModeVersion || f.epochOf(command.CaseID) != command.ExpectedOwnerEpoch {
		_, digest, _ := f.fixturePolicyFactsSnapshot(command.CaseID)
		return casesAPI.WorkOutcome{}, casesDomain.PolicyConflictError{
			CurrentModeVersion: modeVersion, CurrentOwnerEpoch: f.epochOf(command.CaseID),
			CurrentFactsDigest: digest, CurrentWorkItemVersion: command.ExpectedWorkItemVersion,
		}
	}
	world, digest, digestMap := f.fixturePolicyFactsSnapshot(command.CaseID)
	if digest != command.ExpectedFactsDigest {
		return casesAPI.WorkOutcome{}, casesDomain.PolicyConflictError{
			CurrentModeVersion: modeVersion, CurrentOwnerEpoch: f.epochOf(command.CaseID),
			CurrentFactsDigest: digest, CurrentWorkItemVersion: command.ExpectedWorkItemVersion,
		}
	}
	// The effective mode of the Work Item (the REAL derivation).
	active := casesDomain.ActiveConversationMode(terms)
	itemContext := casesDomain.WorkItemModeContext(terms, proposal.WorkItemID)
	effective := casesDomain.EffectiveWorkModeWord(casesDomain.EffectiveWorkMode(active, itemContext))
	// The deterministic decision (the REAL pure function) over the CURRENT
	// preferences of the case (the Story 2.11 fourth input — the honest
	// alternatives trigger of the recorded no_photo; the snapshot stamps
	// the canonical preferences map: the same state renders the same
	// digest, a changed state is the honest supersede).
	preferences := f.fixtureAccommodationCurrent(command.CaseID)
	// The 2.12 fifth input: the route degradations map with the dialog
	// route (nil + the zero route keep the control branch — the honest
	// wiring arrives with step-03; the fixture mirrors the store).
	decision := casesDomain.EvaluatePolicy(*proposal, world, effective, preferences, nil, casesDomain.RouteKey{})
	snapshot := casesDomain.PolicyInputSnapshot{
		ProposalID: proposal.ProposalID, SchemaVersion: proposal.SchemaVersion,
		ModeVersion: modeVersion, OwnerEpoch: f.epochOf(command.CaseID),
		FactsDigest: digestMap, FactsDigestHash: digest,
		WorkItemVersion: command.ExpectedWorkItemVersion, RuleVersion: casesDomain.PolicyRuleVersion,
		Preferences:  casesDomain.AccommodationSnapshotOf(preferences),
		Degradations: casesDomain.DegradationSnapshotOf(nil),
	}
	decisionDigest, err := casesDomain.DecisionDigest(snapshot, decision, proposal.Intent)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The per-Work-Item monotonic outcome number.
	outcomeNo := int64(0)
	for _, outcome := range f.policyOutcomes[command.CaseID] {
		if outcome.WorkItemID == proposal.WorkItemID && outcome.OutcomeNo > outcomeNo {
			outcomeNo = outcome.OutcomeNo
		}
	}
	outcomeNo++
	outcomeID := f.fixturePolicyOutcomeID()
	f.policyOutcomes[command.CaseID] = append(f.policyOutcomes[command.CaseID], casesDomain.PolicyOutcome{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: proposal.ConversationID,
		WorkItemID: proposal.WorkItemID, ProposalID: proposal.ProposalID, OutcomeID: outcomeID,
		OutcomeNo: outcomeNo, OutcomeClass: decision.OutcomeClass, Reason: decision.Reason,
		RuleVersion: casesDomain.PolicyRuleVersion, Intent: proposal.Intent,
		MandatoryFacts: decision.MandatoryFacts, NextSteps: decision.NextSteps,
		ReusedFacts: decision.ReusedFacts, SuppressedQuestions: decision.SuppressedQuestions,
		ActiveAlternatives: decision.ActiveAlternatives,
		InputSnapshot:      snapshot, DecisionDigest: decisionDigest,
		ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "evaluate_policy", ProposalID: command.ProposalID,
		OutcomeID: outcomeID, OutcomeClass: decision.OutcomeClass}
	f.storeOutcome("evaluate_policy", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixturePolicyGate applies the REAL policy gate of the sales reply: the
// Work Item anchor and the CURRENT outcome of that item (the maximal
// number) must be commercial_answer. The return is the Policy Outcome id
// the reply record references.
func (f *casesFixture) fixturePolicyGate(command casesAPI.SendClientReplyCommand) (id.UUID, error) {
	if command.ReplyClass != casesDomain.ReplyClassSales {
		return id.UUID(""), nil
	}
	if !command.WorkItemID.Valid() {
		return id.UUID(""), casesDomain.ReplyPolicyError{PermittedPath: casesDomain.PermittedPathAnchorWorkItem, Reason: casesDomain.ReplyPolicyNoAnchorText}
	}
	outcomesOfItem := []casesDomain.PolicyOutcome{}
	for _, outcome := range f.policyOutcomes[command.CaseID] {
		if outcome.WorkItemID == command.WorkItemID {
			outcomesOfItem = append(outcomesOfItem, outcome)
		}
	}
	current := casesDomain.CurrentPolicyOutcome(outcomesOfItem)
	if current == nil {
		permitted := casesDomain.PermittedPathSubmitProposal
		for _, proposal := range f.aiProposals[command.CaseID] {
			if proposal.WorkItemID == command.WorkItemID {
				permitted = casesDomain.PermittedPathEvaluatePolicy
				break
			}
		}
		return id.UUID(""), casesDomain.ReplyPolicyError{WorkItemID: command.WorkItemID, PermittedPath: permitted, Reason: casesDomain.ReplyPolicyOutcomeRequiredText}
	}
	if current.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
		return id.UUID(""), casesDomain.ReplyPolicyError{
			WorkItemID: command.WorkItemID, CurrentClass: current.OutcomeClass, CurrentReason: current.Reason,
			RuleVersion: current.RuleVersion, OutcomeNo: current.OutcomeNo,
			PermittedPath: casesDomain.PermittedPathEvaluatePolicy, Reason: casesDomain.ReplyPolicyNotCommercialText,
		}
	}
	return current.OutcomeID, nil
}

// fixturePolicyView assembles the policy section of the case view over
// the REAL derivations (the CURRENT outcome per Work Item; the anchors of
// the evaluation form).
func (f *casesFixture) fixturePolicyView(caseID id.UUID) casesAPI.PolicyView {
	_, digest, _ := f.fixturePolicyFactsSnapshot(caseID)
	view := casesAPI.PolicyView{
		IntentWords: append([]string(nil), casesDomain.ProposalIntents...),
		Commercial:  append([]string(nil), casesDomain.CommercialIntents...),
		ClassWords:  append([]string(nil), casesDomain.OutcomeClasses...),
		RuleVersion: casesDomain.PolicyRuleVersion,
		PanelText:   "Предложения ИИ — притязания подателя (порт или оператор): факты из них не записываются автоматически. Действующий исход — последний номер; продажный путь открыт только классом commercial_answer.",
		FactsDigest: digest, OwnerEpoch: f.epochOf(caseID),
		ModeVersion: casesDomain.ModeTermMaxVersion(f.modeTerms[caseID]),
	}
	for _, anchor := range f.items[caseID] {
		section := casesAPI.PolicyWorkItemView{WorkItemID: anchor.WorkItemID}
		for _, proposal := range f.aiProposals[caseID] {
			if proposal.WorkItemID == anchor.WorkItemID {
				section.Proposals = append(section.Proposals, proposal)
			}
		}
		for _, outcome := range f.policyOutcomes[caseID] {
			if outcome.WorkItemID == anchor.WorkItemID {
				section.Outcomes = append(section.Outcomes, outcome)
			}
		}
		section.Current = casesDomain.CurrentPolicyOutcome(section.Outcomes)
		view.Items = append(view.Items, section)
	}
	return view
}

// epochOf resolves the fixture ownership epoch of the case (the honest
// zero of the pre-ownership world).
func (f *casesFixture) epochOf(caseID id.UUID) int64 {
	if current := casesDomain.CurrentOwnership(f.ownershipTerms[caseID]); current != nil {
		return current.Epoch
	}
	return 0
}
