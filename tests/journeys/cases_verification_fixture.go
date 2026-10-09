package main

import (
	"context"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.3 verification command surface of the fixture: the REAL
// domain guards and derivations (GuardStartVerification, CanonicalOutcome,
// DeriveAssuranceState, GuardBindChannel, GuardUnlinkChannel,
// GuardResolveDuplicate, ReversalOf, ResolvedDecision) over the in-memory
// material — the transport contract under test is the production one.
// ---------------------------------------------------------------------------

// fixtureVerificationSeq is the deterministic id counter of the fixture
// verification material.
func (f *casesFixture) nextVerificationID() id.UUID {
	f.workSeq++
	return f.fixtureWorkSequence("verification", 900000+f.workSeq)
}

// conversationOf resolves one conversation of one case of the fixture
// (the containment check of the commands).
func (f *casesFixture) conversationOf(caseID, conversationID id.UUID) (casesDomain.Conversation, bool) {
	material := f.material[caseID]
	if material == nil {
		return casesDomain.Conversation{}, false
	}
	for _, conversation := range material.conversations {
		if conversation.ConversationID == conversationID {
			return conversation, true
		}
	}
	return casesDomain.Conversation{}, false
}

// caseOfConversation resolves the case owning one conversation.
func (f *casesFixture) caseOfConversation(conversationID id.UUID) (id.UUID, bool) {
	for caseID, material := range f.material {
		for _, conversation := range material.conversations {
			if conversation.ConversationID == conversationID {
				return caseID, true
			}
		}
	}
	return id.UUID(""), false
}

// verificationsOf returns the verification records of one case.
func (f *casesFixture) verificationsOf(caseID id.UUID) []casesDomain.VerificationRecord {
	records := []casesDomain.VerificationRecord{}
	for _, record := range f.verifications {
		if record.CaseID == caseID {
			records = append(records, record)
		}
	}
	return records
}

// StartVerification records one verification (the domain guard, the
// canonical outcome mapping, the step-up window, the computed expiry).
func (f *casesFixture) StartVerification(_ context.Context, command casesAPI.StartVerificationCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("start_verification", casesDomain.GuardStartVerification(command.Kind, command.Method, command.ProviderCode, command.SubjectHandle, command.AuthorityScope, command.Note)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if command.EvidenceEventID.Valid() && command.Note != "" {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("start_verification", fmt.Errorf("evidence проверки — событие ИЛИ нота, не оба"))
	}
	if !command.EvidenceEventID.Valid() && command.Note == "" {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("start_verification", fmt.Errorf("evidence проверки обязателен: событие кейса или нота"))
	}
	hashMaterial := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s", command.CaseID, command.ConversationID, command.Kind, command.Method, command.ProviderCode, command.SubjectHandle, command.EvidenceEventID, command.Note)
	outcome, fresh, err := f.workCommandFlow("start_verification", command.IdempotencyKey, hashMaterial)
	if err != nil || !fresh {
		return outcome, err
	}
	conversation, known := f.conversationOf(command.CaseID, command.ConversationID)
	if !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if !casesDomain.MethodAllowedForModality(command.Method, conversation.Modality) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("start_verification", fmt.Errorf("метод %s не разрешён для модальности %s", command.Method, conversation.Modality))
	}
	canonical, err := casesDomain.CanonicalOutcome(command.ProviderCode)
	if err != nil {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("start_verification", err)
	}
	records := f.verificationsOf(command.CaseID)
	if casesDomain.DeriveAssuranceState(records, command.Now).StepUp {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("start_verification", fmt.Errorf("%s", casesDomain.StepUpText))
	}
	level := casesDomain.AssuranceNone
	if canonical == casesDomain.VerificationPassed {
		level = casesDomain.MethodAssuranceLevel(command.Method)
	}
	verificationID := f.nextVerificationID()
	f.verifications = append(f.verifications, casesDomain.VerificationRecord{
		TenantID: command.TenantID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		VerificationID: verificationID, Kind: command.Kind, Method: command.Method,
		Outcome: canonical, AssuranceLevel: level, SubjectHandle: command.SubjectHandle,
		IssuerID: command.Actor.ID, EvidenceEventID: command.EvidenceEventID, Note: command.Note,
		AuthorityScope: command.AuthorityScope, IssuedAt: command.Now, ExpiresAt: command.Now.Add(casesDomain.MethodValidity(command.Method)),
		CreatedAt: command.Now,
	})
	// The threshold crossing: the projected window writes the exception.
	projected := append(append([]casesDomain.VerificationRecord(nil), records...), casesDomain.VerificationRecord{Kind: command.Kind, Outcome: canonical, IssuedAt: command.Now})
	if casesDomain.DeriveAssuranceState(projected, command.Now).StepUp {
		f.exceptionSeq++
		exceptionID := f.fixtureWorkSequence("exception", 800000+f.exceptionSeq)
		f.exceptions = append(f.exceptions, casesDomain.OperationalException{
			TenantID: command.TenantID, ExceptionID: exceptionID, Kind: casesDomain.ExceptionIdentityStepUp,
			Summary:       "Порог неудачных проверок обращения достигнут: шаги проверки и связывания приостановлены на окно порога.",
			CorrelationID: exceptionID, CaseID: command.CaseID, ConversationID: command.ConversationID, CreatedAt: command.Now,
		})
	}
	outcome = casesAPI.WorkOutcome{
		Command: "start_verification", VerificationID: verificationID, VerificationKind: command.Kind,
		VerificationOutcome: canonical, AssuranceLevel: level,
	}
	f.storeOutcome("start_verification", command.IdempotencyKey, outcome)
	return outcome, nil
}

// BindChannel links one conversation into an existing conversation and
// case (the gate, the target validation, the no-active-binding check).
func (f *casesFixture) BindChannel(_ context.Context, command casesAPI.BindChannelCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("bind_channel", casesDomain.GuardBindChannel(command.Method, command.Purpose, command.SubjectHandle)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if command.EvidenceEventID.Valid() && command.Note != "" {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("bind_channel", fmt.Errorf("evidence связывания — событие ИЛИ нота, не оба"))
	}
	if command.Method == casesDomain.BindingMethodConsentHandoff && !command.EvidenceEventID.Valid() && command.Note == "" {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("bind_channel", fmt.Errorf("связывание по согласию требует evidence"))
	}
	if command.CaseID == command.BoundCaseID {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("bind_channel", fmt.Errorf("связывание объединяет обращения разных кейсов"))
	}
	hashMaterial := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", command.CaseID, command.ConversationID, command.BoundCaseID, command.BoundConversationID, command.Purpose, command.SubjectHandle, command.Note)
	outcome, fresh, err := f.workCommandFlow("bind_channel", command.IdempotencyKey, hashMaterial)
	if err != nil || !fresh {
		return outcome, err
	}
	if _, known := f.conversationOf(command.CaseID, command.ConversationID); !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if _, known := f.conversationOf(command.BoundCaseID, command.BoundConversationID); !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	// The step-up covers every binding method; the identity gate stays the
	// consent material (the mirror of the store discipline).
	if state := casesDomain.DeriveAssuranceState(f.verificationsOf(command.CaseID), command.Now); state.StepUp {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("bind_channel", fmt.Errorf("%s", casesDomain.StepUpText))
	} else if command.Method == casesDomain.BindingMethodConsentHandoff && !state.BindingAllowed(command.OnBehalf) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("bind_channel", fmt.Errorf("%s", casesDomain.BindingGateText(command.OnBehalf)))
	}
	for _, binding := range f.bindings {
		if binding.BoundConversationID == command.BoundConversationID && casesDomain.ReversalOf(f.reversals, binding.BindingID) == nil {
			return casesAPI.WorkOutcome{}, casesDomain.ErrActiveBinding
		}
	}
	bindingID := f.nextVerificationID()
	f.bindings = append(f.bindings, casesDomain.ChannelBinding{
		TenantID: command.TenantID, BindingID: bindingID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		BoundCaseID: command.BoundCaseID, BoundConversationID: command.BoundConversationID,
		Method: command.Method, Purpose: command.Purpose, OnBehalf: command.OnBehalf,
		SubjectHandle: command.SubjectHandle, EvidenceEventID: command.EvidenceEventID, Note: command.Note,
		ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{
		Command: "bind_channel", BindingID: bindingID, ConfirmationText: casesDomain.BindingClientConfirmation(command.Method),
	}
	f.storeOutcome("bind_channel", command.IdempotencyKey, outcome)
	return outcome, nil
}

// UnlinkChannel reverses one binding (the append-only reversal).
func (f *casesFixture) UnlinkChannel(_ context.Context, command casesAPI.UnlinkChannelCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("unlink_channel", casesDomain.GuardUnlinkChannel(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hashMaterial := command.CaseID.String() + "|" + command.BindingID.String() + "|" + command.Reason
	outcome, fresh, err := f.workCommandFlow("unlink_channel", command.IdempotencyKey, hashMaterial)
	if err != nil || !fresh {
		return outcome, err
	}
	bound := false
	for _, binding := range f.bindings {
		if binding.BindingID == command.BindingID && binding.CaseID == command.CaseID {
			bound = true
		}
	}
	if !bound {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if casesDomain.ReversalOf(f.reversals, command.BindingID) != nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrCommandKeyConflict
	}
	reversalID := f.nextVerificationID()
	f.reversals = append(f.reversals, casesDomain.BindingReversal{
		TenantID: command.TenantID, ReversalID: reversalID, BindingID: command.BindingID,
		Reason: command.Reason, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "unlink_channel", BindingID: command.BindingID}
	f.storeOutcome("unlink_channel", command.IdempotencyKey, outcome)
	return outcome, nil
}

// ResolveDuplicate decides one candidate (the same-client decision creates
// the duplicate-review binding; the separate decision keeps them apart).
func (f *casesFixture) ResolveDuplicate(_ context.Context, command casesAPI.ResolveDuplicateCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("resolve_duplicate", casesDomain.GuardResolveDuplicate(command.Decision, command.Note, command.SubjectHandle)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if command.ConversationID == command.BoundConversationID {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("resolve_duplicate", fmt.Errorf("решение дубля связывает разные разговоры"))
	}
	hashMaterial := fmt.Sprintf("%s|%s|%s|%s|%s|%s", command.CaseID, command.CandidateID, command.Decision, command.Note, command.ConversationID, command.BoundConversationID)
	outcome, fresh, err := f.workCommandFlow("resolve_duplicate", command.IdempotencyKey, hashMaterial)
	if err != nil || !fresh {
		return outcome, err
	}
	var candidate *casesDomain.DuplicateCandidate
	for i := range f.candidates {
		if f.candidates[i].CandidateID == command.CandidateID && f.candidates[i].CaseIDNew == command.CaseID {
			candidate = &f.candidates[i]
		}
	}
	if candidate == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if casesDomain.ResolvedDecision(f.resolutions, command.CandidateID) != "" {
		return casesAPI.WorkOutcome{}, casesDomain.ErrCommandKeyConflict
	}
	resolutionID := f.nextVerificationID()
	f.resolutions = append(f.resolutions, casesDomain.DuplicateResolution{
		TenantID: command.TenantID, ResolutionID: resolutionID, CandidateID: command.CandidateID,
		Decision: command.Decision, Note: command.Note, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	bindingID := id.UUID("")
	confirmation := ""
	if command.Decision == casesDomain.DuplicateDecisionSameClient {
		bindingID = f.nextVerificationID()
		f.bindings = append(f.bindings, casesDomain.ChannelBinding{
			TenantID: command.TenantID, BindingID: bindingID, CaseID: candidate.CaseIDExisting,
			ConversationID: command.ConversationID, BoundCaseID: candidate.CaseIDNew,
			BoundConversationID: command.BoundConversationID, Method: casesDomain.BindingMethodDuplicateReview,
			SubjectHandle: command.SubjectHandle, ActorID: command.Actor.ID, CreatedAt: command.Now,
		})
		confirmation = casesDomain.BindingClientConfirmation(casesDomain.BindingMethodDuplicateReview)
	}
	outcome = casesAPI.WorkOutcome{
		Command: "resolve_duplicate", DuplicateCandidateID: command.CandidateID,
		DuplicateDecision: command.Decision, BindingID: bindingID, ConfirmationText: confirmation,
	}
	f.storeOutcome("resolve_duplicate", command.IdempotencyKey, outcome)
	return outcome, nil
}

// recordIdentifierToken tokenizes one identifier fact value of the fixture
// (the same keyed HMAC discipline) and creates the duplicate candidates of
// the exact matches in other open cases.
func (f *casesFixture) recordIdentifierToken(caseID id.UUID, materialKey, rawValue string, now time.Time) {
	kind := casesDomain.IdentifierKind(materialKey)
	if kind == "" {
		return
	}
	normalized, err := casesDomain.NormalizeIdentifier(kind, rawValue)
	if err != nil {
		return
	}
	if f.identifierKey == nil {
		key, keyErr := casesDomain.NewIdentifierKey()
		if keyErr != nil {
			return
		}
		f.identifierKey = key
	}
	token := casesDomain.IdentifierToken(f.identifierKey, kind, normalized)
	if _, exists := f.identifierTokens[token]; !exists {
		f.identifierTokens[token] = map[id.UUID]string{}
	}
	if _, exists := f.identifierTokens[token][caseID]; exists {
		return
	}
	f.identifierTokens[token][caseID] = materialKey
	for otherCase := range f.identifierTokens[token] {
		if otherCase == caseID {
			continue
		}
		material := f.material[otherCase]
		if material == nil || material.cases.State != "open" {
			continue
		}
		f.workSeq++
		candidateID := f.fixtureWorkSequence("duplicate", 700000+f.workSeq)
		f.candidates = append(f.candidates, casesDomain.DuplicateCandidate{
			TenantID: f.tenant, CandidateID: candidateID, CaseIDNew: caseID, CaseIDExisting: otherCase,
			Token: token, MaterialKey: materialKey, MatchReason: casesDomain.MatchReasonExactIdentifier,
			RuleConfidence: casesDomain.RuleConfidenceHigh, CreatedAt: now,
		})
	}
}
