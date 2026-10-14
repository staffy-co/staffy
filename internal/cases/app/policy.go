package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The two policy commands and the policy view (Story 2.6)
// ---------------------------------------------------------------------------
//
// Both commands are idempotent by the tenant-scoped key with the stored
// outcome (the 2-2..2-5 precedent: the replay answer comes BEFORE the
// domain guards); the request hash of evaluate_policy includes the four
// anchors of the input versions (the m-r2-1 lesson — the full material of
// the command). The evaluation reads the proposal, the facts, the mode
// ladder, the ownership and the Work Item state, checks the anchors and
// runs the deterministic pure function; the store re-validates the same
// order under the case + conversation advisory locks and writes the
// append-only outcome with the monotonic number.

// SubmitAIProposal records one typed proposal of the proposer port: the
// submission validates the structure, the bounds and the field dictionary
// words ONLY (the intent dictionary is a POLICY matter — an unknown intent
// fails closed as an outcome, never as a submission refusal); the
// accepted turn, the case's own conversation and the anchored Work Item
// are the honest references; the submission snapshot (the owner epoch and
// the mode version) is the store-computed truth.
func (s *Service) SubmitAIProposal(ctx context.Context, command casesAPI.SubmitAIProposalCommand) (casesAPI.WorkOutcome, error) {
	// The policy commands need the WRITE scope of the surface — review or
	// configuration (the frozen Boundaries word: «submit/evaluate —
	// участники с review»); an audit-only member reads the case surface
	// but never writes the proposals (the R1 Major fix precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, rule("submit_ai_proposal", fmt.Errorf("%s", ModeWriteScopeText))
	}
	// The schema problems travel UNWRAPPED: the machine-readable field
	// errors of the 400 problem (the repair loop is the proposer's duty —
	// the honest flat field list, never a stack trace).
	if err := casesDomain.GuardSubmitProposal(casesDomain.AIProposal{
		ConversationID: command.ConversationID, WorkItemID: command.WorkItemID, TurnRef: command.TurnRef,
		Intent: command.Intent, CandidateAnswer: command.CandidateAnswer,
		MissingFacts: command.MissingFacts, ExtractedFacts: command.ExtractedFacts, Proposer: command.Proposer,
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		WorkItem     string `json:"work_item"`
		TurnRef      string `json:"turn_ref"`
		Intent       string `json:"intent"`
		Candidate    string `json:"candidate_answer"`
		Missing      string `json:"missing_facts"`
		Extracted    string `json:"extracted_facts"`
		Proposer     string `json:"proposer"`
	}{command.CaseID.String(), command.ConversationID.String(), command.WorkItemID.String(), command.TurnRef.String(),
		command.Intent, command.CandidateAnswer,
		string(mustJSON(command.MissingFacts)), string(mustJSON(command.ExtractedFacts)), command.Proposer})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "submit_ai_proposal", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The conversation of the proposal: the case's own conversation (the
	// unified dialog anchor — the mode precedent).
	if _, err := s.modeConversation(ctx, scope, command.CaseID, command.ConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The accepted turn the proposal answers.
	if _, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, command.TurnRef); err != nil {
		return casesAPI.WorkOutcome{}, rule("submit_ai_proposal", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	// The Work Item anchor of the proposal scope.
	if err := s.modeWorkItem(ctx, scope, command.CaseID, command.WorkItemID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	proposalID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "submit_ai_proposal", ProposalID: proposalID,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "submit_ai_proposal", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		WorkItemID: command.WorkItemID, ProposalID: proposalID, TurnRef: command.TurnRef,
		Intent: command.Intent, CandidateAnswer: command.CandidateAnswer,
		MissingFacts: command.MissingFacts, ExtractedFacts: command.ExtractedFacts, Proposer: command.Proposer,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// EvaluatePolicy runs the deterministic policy over the anchored input
// versions: the proposal reference and the four anchors (the mode
// version, the owner epoch, the confirmed facts digest and the Work Item
// version). A moved anchor is the honest stale conflict with the current
// words; the terminal Work Item refuses honestly; the outcome lands
// append-only with the monotonic number and the full input snapshot with
// the decision digest (the same anchors produce the same digest).
func (s *Service) EvaluatePolicy(ctx context.Context, command casesAPI.EvaluatePolicyCommand) (casesAPI.WorkOutcome, error) {
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, rule("evaluate_policy", fmt.Errorf("%s", ModeWriteScopeText))
	}
	if err := rule("evaluate_policy", casesDomain.GuardEvaluate(command.ProposalID, command.ExpectedModeVersion, command.ExpectedOwnerEpoch, command.ExpectedWorkItemVersion, command.ExpectedFactsDigest)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash carries the WHOLE material INCLUDING the four
	// anchors (the m-r2-1 lesson): the replay of the exact repeat returns
	// the stored outcome; the same key with another anchor set is the
	// honest key conflict.
	hash := requestHash(struct {
		Case            string `json:"case"`
		Proposal        string `json:"proposal_ref"`
		ModeVersion     int64  `json:"expected_mode_version"`
		OwnerEpoch      int64  `json:"expected_owner_epoch"`
		FactsDigest     string `json:"expected_facts_digest"`
		WorkItemVersion int64  `json:"expected_work_item_version"`
	}{command.CaseID.String(), command.ProposalID.String(), command.ExpectedModeVersion,
		command.ExpectedOwnerEpoch, command.ExpectedFactsDigest, command.ExpectedWorkItemVersion})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "evaluate_policy", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The proposal of the evaluation: a proposal of THIS case (the 404
	// without the existence disclosure of another tenant's material).
	proposals, err := s.repository.ReadAIProposals(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	var proposal *casesDomain.AIProposal
	for i := range proposals {
		if proposals[i].ProposalID == command.ProposalID {
			proposal = &proposals[i]
			break
		}
	}
	if proposal == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	// The Work Item state of the proposal scope: the terminal states
	// refuse honestly (the merged item lives on in its target).
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{proposal.WorkItemID})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, ok := currents[proposal.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, rule("evaluate_policy", fmt.Errorf("%s", casesDomain.TerminalWorkItemText))
	}
	// The anchored input versions (the pre-transaction honest read; the
	// store re-validates the same order under the locks).
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	ownership := casesDomain.CurrentOwnership(ownershipTerms)
	if ownership == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	modeTerms, err := s.repository.ReadModeMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	modeVersion := casesDomain.ModeTermMaxVersion(modeTerms)
	facts, err := s.repository.ReadFactsOfCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	snapshot, err := policyReadFactsSnapshot(facts)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if modeVersion != command.ExpectedModeVersion || ownership.Epoch != command.ExpectedOwnerEpoch || current.Version != command.ExpectedWorkItemVersion {
		return casesAPI.WorkOutcome{}, casesDomain.PolicyConflictError{
			CurrentModeVersion: modeVersion, CurrentOwnerEpoch: ownership.Epoch,
			CurrentFactsDigest: snapshot.digest, CurrentWorkItemVersion: current.Version,
		}
	}
	if snapshot.digest != command.ExpectedFactsDigest {
		return casesAPI.WorkOutcome{}, casesDomain.PolicyConflictError{
			CurrentModeVersion: modeVersion, CurrentOwnerEpoch: ownership.Epoch,
			CurrentFactsDigest: snapshot.digest, CurrentWorkItemVersion: current.Version,
		}
	}
	// The effective mode of the Work Item (the 2-5 derivation) and the
	// deterministic decision (the pure function).
	active := casesDomain.ActiveConversationMode(modeTerms)
	itemContext := active
	if itemContext != nil {
		if context := casesDomain.WorkItemModeContext(modeTerms, proposal.WorkItemID); context != nil {
			itemContext = context
		}
	}
	effective := casesDomain.EffectiveWorkMode(active, itemContext)
	// The 2.11 fourth input: the preferences state (nil keeps the control
	// branch here — the honest wiring arrives with step-03; the store
	// re-derives the decision under the locks anyway).
	decision := casesDomain.EvaluatePolicy(*proposal, snapshot.world, casesDomain.EffectiveWorkModeWord(effective), nil)
	outcomeID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "evaluate_policy", ProposalID: command.ProposalID,
		OutcomeID: outcomeID, OutcomeClass: decision.OutcomeClass,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "evaluate_policy", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: proposal.ConversationID,
		WorkItemID: proposal.WorkItemID, ProposalID: command.ProposalID, OutcomeID: outcomeID,
		ExpectedModeVersion: command.ExpectedModeVersion, ExpectedOwnerEpoch: command.ExpectedOwnerEpoch,
		ExpectedFactsDigest: command.ExpectedFactsDigest, ExpectedWorkItemVersion: command.ExpectedWorkItemVersion,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// replyPolicyGate is the app-side policy gate of the sales reply (the
// store re-validates under the locks — the same order): the Work Item
// anchor is required (the sales path belongs to a Work Item), the
// CURRENT (the maximal number) outcome of that item must be
// commercial_answer. The passing gate resolves the Policy Outcome id the
// reply record references (the request never carries it — the
// server-side truth).
func (s *Service) replyPolicyGate(ctx context.Context, scope tenancy.Context, caseID, workItemID id.UUID) (id.UUID, error) {
	if !workItemID.Valid() {
		return id.UUID(""), casesDomain.ReplyPolicyError{PermittedPath: casesDomain.PermittedPathAnchorWorkItem, Reason: casesDomain.ReplyPolicyNoAnchorText}
	}
	outcomes, err := s.repository.ReadPolicyOutcomes(ctx, scope, caseID)
	if err != nil {
		return id.UUID(""), err
	}
	outcomesOfItem := []casesDomain.PolicyOutcome{}
	for _, outcome := range outcomes {
		if outcome.WorkItemID == workItemID {
			outcomesOfItem = append(outcomesOfItem, outcome)
		}
	}
	current := casesDomain.CurrentPolicyOutcome(outcomesOfItem)
	if current == nil {
		// No outcome exists: the permitted path is the evaluation — with
		// a proposal already at hand evaluate_policy, otherwise the
		// submission first.
		permitted := casesDomain.PermittedPathSubmitProposal
		proposals, err := s.repository.ReadAIProposals(ctx, scope, caseID)
		if err != nil {
			return id.UUID(""), err
		}
		for _, proposal := range proposals {
			if proposal.WorkItemID == workItemID {
				permitted = casesDomain.PermittedPathEvaluatePolicy
				break
			}
		}
		return id.UUID(""), casesDomain.ReplyPolicyError{WorkItemID: workItemID, PermittedPath: permitted, Reason: casesDomain.ReplyPolicyOutcomeRequiredText}
	}
	if current.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
		return id.UUID(""), casesDomain.ReplyPolicyError{
			WorkItemID: workItemID, CurrentClass: current.OutcomeClass, CurrentReason: current.Reason,
			RuleVersion: current.RuleVersion, OutcomeNo: current.OutcomeNo,
			PermittedPath: casesDomain.PermittedPathEvaluatePolicy, Reason: casesDomain.ReplyPolicyNotCommercialText,
		}
	}
	return current.OutcomeID, nil
}

// policyView assembles the policy section of the case view: the
// proposals and the outcomes per Work Item with the CURRENT outcome (the
// maximal number), the dictionary hint words of the panel and the fixed
// rule words. The reads degrade honestly — the same view scope.
func (s *Service) policyView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.PolicyView, error) {
	proposals, err := s.repository.ReadAIProposals(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.PolicyView{}, err
	}
	outcomes, err := s.repository.ReadPolicyOutcomes(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.PolicyView{}, err
	}
	view := casesAPI.PolicyView{
		IntentWords: append([]string(nil), casesDomain.ProposalIntents...),
		Commercial:  append([]string(nil), casesDomain.CommercialIntents...),
		ClassWords:  append([]string(nil), casesDomain.OutcomeClasses...),
		RuleVersion: casesDomain.PolicyRuleVersion,
		PanelText:   "Предложения ИИ — притязания подателя (порт или оператор): факты из них не записываются автоматически. Действующий исход — последний номер; продажный путь открыт только классом commercial_answer.",
	}
	// The current anchor words of the evaluation form (the server truth —
	// the honest reads of the same scope).
	facts, err := s.repository.ReadFactsOfCase(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.PolicyView{}, err
	}
	snapshot, err := policyReadFactsSnapshot(facts)
	if err != nil {
		return casesAPI.PolicyView{}, err
	}
	view.FactsDigest = snapshot.digest
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.PolicyView{}, err
	}
	if ownership := casesDomain.CurrentOwnership(ownershipTerms); ownership != nil {
		view.OwnerEpoch = ownership.Epoch
	}
	modeTerms, err := s.repository.ReadModeMaterial(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.PolicyView{}, err
	}
	view.ModeVersion = casesDomain.ModeTermMaxVersion(modeTerms)
	for _, item := range detail.WorkItems {
		section := casesAPI.PolicyWorkItemView{WorkItemID: item.WorkItemID}
		for _, proposal := range proposals {
			if proposal.WorkItemID == item.WorkItemID {
				section.Proposals = append(section.Proposals, proposal)
			}
		}
		for _, outcome := range outcomes {
			if outcome.WorkItemID == item.WorkItemID {
				section.Outcomes = append(section.Outcomes, outcome)
			}
		}
		section.Current = casesDomain.CurrentPolicyOutcome(section.Outcomes)
		view.Items = append(view.Items, section)
	}
	return view, nil
}

// mustJSON renders the canonical JSON of the hash material (the marshal
// failure is impossible for the plain structs — the honest panic of the
// invariant, never a client answer).
func mustJSON(value any) []byte {
	rendered, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("неизвестная ошибка канонического JSON: %v", err))
	}
	return rendered
}

// policyFactsSnapshot is the confirmed fact material of the evaluation:
// the world for the pure function, the digest for the anchor and the map
// for the input snapshot.
type policyFactsSnapshot struct {
	world     casesDomain.FactWorld
	digest    string
	digestMap casesDomain.FactsDigestMap
}

// policyReadFactsSnapshot derives the confirmed fact material of the case
// (the 2-2 derivation: the confirmed exact versions only — the reuse and
// the digest basis).
func policyReadFactsSnapshot(materials []casesPort.FactMaterial) (policyFactsSnapshot, error) {
	world := casesDomain.FactWorld{}
	confirmed := []casesDomain.ConfirmedFact{}
	for _, material := range materials {
		status, versionNo, digest := casesDomain.ConfirmedFactView(material.Versions, material.Confirmations)
		word := casesDomain.FactStatusWord{Status: status}
		if status == casesDomain.FactStatusConfirmed {
			fact := casesDomain.ConfirmedFact{
				FactID: material.Anchor.FactID, MaterialKey: material.Anchor.MaterialKey,
				VersionNo: versionNo, ValueDigest: digest,
			}
			confirmed = append(confirmed, fact)
			factCopy := fact
			word.Confirmed = &factCopy
		}
		world[material.Anchor.MaterialKey] = word
	}
	digest, digestMap, err := casesDomain.ComputeFactsDigest(confirmed)
	if err != nil {
		return policyFactsSnapshot{}, err
	}
	return policyFactsSnapshot{world: world, digest: digest, digestMap: digestMap}, nil
}
