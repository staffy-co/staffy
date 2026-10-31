package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The four verification commands (Story 2.3)
// ---------------------------------------------------------------------------
//
// Every command is idempotent by the tenant-scoped key with the stored
// outcome (the 2-2 precedent: the replay answer comes BEFORE the domain
// guards); every write is append-only; the binding gate, the step-up
// threshold and the duplicate gate re-validate under the advisory locks of
// the store transaction — the honest raced signals, never a silent second
// execution.

// WithIdentityVerifier wires the contour verifier port. Without it the
// verification commands honestly refuse — an "ok" verification contour is
// never assumed (the ConnectionsFacts precedent).
func (s *Service) WithIdentityVerifier(verifier casesPort.IdentityVerifier) *Service {
	s.identityVerifier = verifier
	return s
}

// StartVerification records one verification: the command material guard,
// the contour verifier (the eligibility + the provider code), the canonical
// outcome mapping, the step-up window check and the append-only record with
// the computed expiry. The crossing of the threshold window writes the
// identity_step_up exception in the SAME transaction (no secrets, no
// identifier values).
func (s *Service) StartVerification(ctx context.Context, command casesAPI.StartVerificationCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("start_verification", casesDomain.GuardStartVerification(command.Kind, command.Method, command.ProviderCode, command.SubjectHandle, command.AuthorityScope, command.Note)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if command.EvidenceEventID.Valid() && command.Note != "" {
		return casesAPI.WorkOutcome{}, rule("start_verification", fmt.Errorf("evidence проверки — событие ИЛИ нота, не оба"))
	}
	if !command.EvidenceEventID.Valid() && command.Note == "" {
		return casesAPI.WorkOutcome{}, rule("start_verification", fmt.Errorf("evidence проверки обязателен: событие кейса или нота"))
	}
	hash := requestHash(struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Kind         string `json:"kind"`
		Method       string `json:"method"`
		Provider     string `json:"provider_code"`
		Subject      string `json:"subject"`
		Scope        string `json:"scope"`
		Evidence     string `json:"evidence"`
		Note         string `json:"note"`
	}{command.CaseID.String(), command.ConversationID.String(), command.Kind, command.Method, command.ProviderCode, command.SubjectHandle, command.AuthorityScope, command.EvidenceEventID.String(), command.Note})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "start_verification", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The verification context: the conversation must belong to the case;
	// its modality drives the method eligibility.
	conversation, err := s.repository.ReadConversationOfCase(ctx, scope, command.CaseID, command.ConversationID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if s.identityVerifier == nil {
		return casesAPI.WorkOutcome{}, rule("start_verification", fmt.Errorf("контур верификации не сконфигурирован: команды проверки честно отклоняются, «ok» не предполагается"))
	}
	verification, err := s.identityVerifier.Verify(ctx, casesPort.IdentityVerificationRequest{
		Kind: command.Kind, Method: command.Method, Modality: conversation.Modality, ProviderCode: command.ProviderCode,
	})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !verification.Eligible {
		return casesAPI.WorkOutcome{}, rule("start_verification", fmt.Errorf("метод %s не разрешён для модальности %s: %s", command.Method, conversation.Modality, verification.IneligibilityText))
	}
	outcome, err := casesDomain.CanonicalOutcome(verification.ProviderCode)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("start_verification", err)
	}
	// The step-up window: an active threshold refuses the new attempts
	// (the rate limit of the slice); the crossing itself is written by the
	// store transaction under the case lock.
	records, err := s.repository.ReadVerificationRecords(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if state := casesDomain.DeriveAssuranceState(records, command.Now); state.StepUp {
		return casesAPI.WorkOutcome{}, rule("start_verification", fmt.Errorf("%s", casesDomain.StepUpText))
	}
	verificationID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	exceptionID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	level := casesDomain.AssuranceNone
	if outcome == casesDomain.VerificationPassed {
		level = casesDomain.MethodAssuranceLevel(command.Method)
	}
	verificationOutcome := casesAPI.WorkOutcome{
		Command: "start_verification", VerificationID: verificationID,
		VerificationKind: command.Kind, VerificationOutcome: outcome, AssuranceLevel: level,
	}
	outcomeJSON, err := json.Marshal(verificationOutcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "start_verification", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		Kind: command.Kind, Method: command.Method, OutcomeName: outcome, AssuranceLevel: level,
		SubjectHandle: command.SubjectHandle, EvidenceEventID: command.EvidenceEventID, Note: command.Note,
		AuthorityScope: command.AuthorityScope, VerificationID: verificationID, ExceptionID: exceptionID,
		ExpiresAt: command.Now.Add(casesDomain.MethodValidity(command.Method)).UTC().Truncate(time.Microsecond),
		Outcome:   outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return verificationOutcome, nil
}

// BindChannel links one conversation into an existing conversation and
// case: the consent handoff requires the verified identity (the authority
// with its scope when the binding is on behalf), the purpose and the
// evidence of the agreement. The gate, the target validation and the
// no-active-binding check re-validate under the store locks.
func (s *Service) BindChannel(ctx context.Context, command casesAPI.BindChannelCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("bind_channel", casesDomain.GuardBindChannel(command.Method, command.Purpose, command.SubjectHandle)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if command.EvidenceEventID.Valid() && command.Note != "" {
		return casesAPI.WorkOutcome{}, rule("bind_channel", fmt.Errorf("evidence связывания — событие ИЛИ нота, не оба"))
	}
	if command.Method == casesDomain.BindingMethodConsentHandoff && !command.EvidenceEventID.Valid() && command.Note == "" {
		return casesAPI.WorkOutcome{}, rule("bind_channel", fmt.Errorf("связывание по согласию требует evidence: событие кейса, где клиент согласился, или нота"))
	}
	if command.CaseID == command.BoundCaseID {
		return casesAPI.WorkOutcome{}, rule("bind_channel", fmt.Errorf("связывание объединяет обращения разных кейсов: цель и источник совпадают"))
	}
	hash := requestHash(struct {
		Case              string `json:"case"`
		Conversation      string `json:"conversation"`
		BoundCase         string `json:"bound_case"`
		BoundConversation string `json:"bound_conversation"`
		Method            string `json:"method"`
		Purpose           string `json:"purpose"`
		OnBehalf          bool   `json:"on_behalf"`
		Subject           string `json:"subject"`
		Evidence          string `json:"evidence"`
		Note              string `json:"note"`
	}{command.CaseID.String(), command.ConversationID.String(), command.BoundCaseID.String(), command.BoundConversationID.String(), command.Method, command.Purpose, command.OnBehalf, command.SubjectHandle, command.EvidenceEventID.String(), command.Note})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "bind_channel", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The target and the bound conversations must exist in their cases;
	// the modality of the target conversation drives the refusal method
	// list (the honest step-up options of the gate).
	target, err := s.repository.ReadConversationOfCase(ctx, scope, command.CaseID, command.ConversationID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if _, err := s.repository.ReadConversationOfCase(ctx, scope, command.BoundCaseID, command.BoundConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The assurance gate of the consent handoff: the verified identity (and
	// the authority when on behalf). The refusal names the required level
	// and the allowed methods of the conversation — never the values that
	// did not match.
	records, err := s.repository.ReadVerificationRecords(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	state := casesDomain.DeriveAssuranceState(records, command.Now)
	if state.StepUp {
		return casesAPI.WorkOutcome{}, rule("bind_channel", fmt.Errorf("%s", casesDomain.StepUpText))
	}
	if !state.BindingAllowed(command.OnBehalf) {
		return casesAPI.WorkOutcome{}, rule("bind_channel", fmt.Errorf("%s Разрешённые методы канала (%s): %s.",
			casesDomain.BindingGateText(command.OnBehalf), target.Modality, methodList(casesDomain.MethodsOfModality(target.Modality))))
	}
	bindingID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	confirmation := casesDomain.BindingClientConfirmation(command.Method)
	bindingOutcome := casesAPI.WorkOutcome{
		Command: "bind_channel", BindingID: bindingID, ConfirmationText: confirmation,
	}
	outcomeJSON, err := json.Marshal(bindingOutcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "bind_channel", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		BoundCaseID: command.BoundCaseID, BoundConversationID: command.BoundConversationID,
		Method: command.Method, Purpose: command.Purpose, OnBehalf: command.OnBehalf,
		SubjectHandle: command.SubjectHandle, EvidenceEventID: command.EvidenceEventID, Note: command.Note,
		BindingID: bindingID, Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return bindingOutcome, nil
}

// UnlinkChannel reverses one binding: the append-only reversal keeps the
// whole history; the derived views recompute. The replay of the unlink is
// the original outcome (the UNIQUE reversal row is the race backstop).
func (s *Service) UnlinkChannel(ctx context.Context, command casesAPI.UnlinkChannelCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("unlink_channel", casesDomain.GuardUnlinkChannel(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case    string `json:"case"`
		Binding string `json:"binding"`
		Reason  string `json:"reason"`
	}{command.CaseID.String(), command.BindingID.String(), command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "unlink_channel", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	reversalID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	unlinkOutcome := casesAPI.WorkOutcome{
		Command: "unlink_channel", BindingID: command.BindingID,
	}
	outcomeJSON, err := json.Marshal(unlinkOutcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "unlink_channel", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, BindingID: command.BindingID, ReversalID: reversalID,
		Reason: command.Reason, Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return unlinkOutcome, nil
}

// ResolveDuplicate decides one duplicate candidate: the same-client
// decision creates the duplicate-review binding atomically (the reviewer is
// the actor; the resolution note is the basis — the client identity gate
// does not apply, the human judgment IS the check); the separate decision
// keeps the requests apart. Until the decision the two cases live
// separately.
func (s *Service) ResolveDuplicate(ctx context.Context, command casesAPI.ResolveDuplicateCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("resolve_duplicate", casesDomain.GuardResolveDuplicate(command.Decision, command.Note, command.SubjectHandle)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if command.ConversationID == command.BoundConversationID {
		return casesAPI.WorkOutcome{}, rule("resolve_duplicate", fmt.Errorf("решение дубля связывает разные разговоры"))
	}
	hash := requestHash(struct {
		Case         string `json:"case"`
		Candidate    string `json:"candidate"`
		Decision     string `json:"decision"`
		Note         string `json:"note"`
		Conversation string `json:"conversation"`
		Bound        string `json:"bound_conversation"`
		Subject      string `json:"subject"`
	}{command.CaseID.String(), command.CandidateID.String(), command.Decision, command.Note, command.ConversationID.String(), command.BoundConversationID.String(), command.SubjectHandle})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "resolve_duplicate", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	resolutionID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	bindingID := id.UUID("")
	confirmation := ""
	if command.Decision == casesDomain.DuplicateDecisionSameClient {
		if bindingID, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		confirmation = casesDomain.BindingClientConfirmation(casesDomain.BindingMethodDuplicateReview)
	}
	resolveOutcome := casesAPI.WorkOutcome{
		Command: "resolve_duplicate", DuplicateCandidateID: command.CandidateID,
		DuplicateDecision: command.Decision, BindingID: bindingID, ConfirmationText: confirmation,
	}
	outcomeJSON, err := json.Marshal(resolveOutcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "resolve_duplicate", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, CandidateID: command.CandidateID, ResolutionID: resolutionID,
		Decision: command.Decision, Note: command.Note, ConversationID: command.ConversationID,
		BoundConversationID: command.BoundConversationID, SubjectHandle: command.SubjectHandle,
		BindingID: bindingID, Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return resolveOutcome, nil
}

// methodList renders the methods for the refusal texts.
func methodList(methods []string) string {
	return joinQuoted(methods)
}
