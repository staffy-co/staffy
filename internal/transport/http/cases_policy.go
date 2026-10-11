package httptransport

import (
	"errors"
	"net/http"
	"strconv"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.6 transport: the machine JSON contract of the two policy
// commands and the operator forms (PRG) of the Progressive Intake / Policy
// panel. The raced anchors are the honest 409 with the current versions;
// the proposal schema problems are the machine-readable 400 field errors;
// the policy gate refusal of the client reply carries the current outcome
// words with the permitted path. The dynamic outcome change is announced
// with aria-live, never with a focus move.
// ---------------------------------------------------------------------------

type submitAIProposalInput struct {
	ConversationID  *string `json:"conversation_id"`
	WorkItemID      *string `json:"work_item_id"`
	TurnRef         *string `json:"turn_ref"`
	Intent          *string `json:"intent"`
	CandidateAnswer *string `json:"candidate_answer"`
	MissingFacts    []struct {
		MaterialKey *string `json:"material_key"`
		Reason      *string `json:"reason"`
	} `json:"missing_facts"`
	ExtractedFacts []struct {
		MaterialKey   *string `json:"material_key"`
		ValueDigest   *string `json:"value_digest"`
		SourceEventID *string `json:"source_event_id"`
	} `json:"extracted_facts"`
	Proposer *string `json:"proposer"`
}

type evaluatePolicyInput struct {
	ProposalID              *string `json:"proposal_ref"`
	ExpectedModeVersion     *int64  `json:"expected_mode_version"`
	ExpectedOwnerEpoch      *int64  `json:"expected_owner_epoch"`
	ExpectedFactsDigest     *string `json:"expected_facts_digest"`
	ExpectedWorkItemVersion *int64  `json:"expected_work_item_version"`
}

// policyProblem maps the policy command errors onto the problem contract:
// the raced anchors are the honest 409 with the current versions; the
// schema problems are the machine-readable 400 with the field errors; the
// policy gate refusal is the 403 with the current outcome words and the
// permitted path; the rule refusals are 400; a foreign tenant is a plain
// 404.
func policyProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var conflict casesDomain.PolicyConflictError
	var schema casesDomain.ProposalSchemaError
	var policyGate casesDomain.ReplyPolicyError
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &conflict):
		server.problem(w, r, http.StatusConflict, "policy_conflict", casesDomain.PolicyConflictText, false, map[string]string{
			"current_mode_version":      strconv.FormatInt(conflict.CurrentModeVersion, 10),
			"current_owner_epoch":       strconv.FormatInt(conflict.CurrentOwnerEpoch, 10),
			"current_facts_digest":      conflict.CurrentFactsDigest,
			"current_work_item_version": strconv.FormatInt(conflict.CurrentWorkItemVersion, 10),
			"permitted_path":            "evaluate_policy",
		})
	case errors.As(err, &schema):
		fields := map[string]string{}
		for _, problem := range schema.Problems {
			fields[problem.Field] = problem.Problem
		}
		server.problem(w, r, http.StatusBadRequest, "proposal_schema", "Схема предложения не принята: проверьте поля.", false, fields)
	case errors.As(err, &policyGate):
		fields := map[string]string{
			"permitted_path": policyGate.PermittedPath,
		}
		if policyGate.WorkItemID.Valid() {
			fields["work_item_id"] = policyGate.WorkItemID.String()
		}
		if policyGate.CurrentClass != "" {
			fields["current_outcome_class"] = policyGate.CurrentClass
			fields["current_outcome_reason"] = policyGate.CurrentReason
			fields["current_rule_version"] = policyGate.RuleVersion
			fields["current_outcome_no"] = strconv.FormatInt(policyGate.OutcomeNo, 10)
		}
		server.problem(w, r, http.StatusForbidden, "reply_policy_gate", policyGate.Reason, false, fields)
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", casesDomain.PolicyConflictText, false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение или материал политики не найдены.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "policy command failed", "code", "policy_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду. Повторите позднее.", true, nil)
	}
}

// policyFormMessage renders the honest flash text of the policy command
// errors (the shared shape of the mode form errors).
func policyFormMessage(err error) string {
	var conflict casesDomain.PolicyConflictError
	var schema casesDomain.ProposalSchemaError
	var policyGate casesDomain.ReplyPolicyError
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &conflict):
		return casesDomain.PolicyConflictText
	case errors.As(err, &schema):
		parts := make([]string, 0, len(schema.Problems))
		for _, problem := range schema.Problems {
			parts = append(parts, problem.Problem)
		}
		return "Схема предложения не принята: " + joinSentences(parts) + " (цикл повторов — обязанность подающего)."
	case errors.As(err, &policyGate):
		return policyGate.Reason
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		return casesDomain.PolicyConflictText
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound):
		return "Обращение или материал политики не найдены."
	}
	return "Действие не выполнено. Повторите позднее."
}

func joinSentences(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += "; "
		}
		result += part
	}
	return result
}

// policyFormError flashes the honest message and returns to the case page
// (the machine-readable words travel inside the text — the form path
// shows them directly).
func policyFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, policyFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

func (s *Server) submitAIProposalAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input submitAIProposalInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	command := casesAPI.SubmitAIProposalCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		Intent: stringOrEmpty(input.Intent), CandidateAnswer: stringOrEmpty(input.CandidateAnswer),
		Proposer: stringOrEmpty(input.Proposer), IdempotencyKey: key, Now: s.clock.Now().UTC(),
	}
	if input.ConversationID != nil {
		conversationID, err := id.Parse(*input.ConversationID)
		if err == nil {
			command.ConversationID = conversationID
		}
	}
	if input.WorkItemID != nil {
		workItemID, err := id.Parse(*input.WorkItemID)
		if err == nil {
			command.WorkItemID = workItemID
		}
	}
	if input.TurnRef != nil {
		turnRef, err := id.Parse(*input.TurnRef)
		if err == nil {
			command.TurnRef = turnRef
		}
	}
	for _, missing := range input.MissingFacts {
		command.MissingFacts = append(command.MissingFacts, casesDomain.MissingFact{
			MaterialKey: stringOrEmpty(missing.MaterialKey), Reason: stringOrEmpty(missing.Reason),
		})
	}
	for _, extracted := range input.ExtractedFacts {
		fact := casesDomain.ExtractedFact{
			MaterialKey: stringOrEmpty(extracted.MaterialKey), ValueDigest: stringOrEmpty(extracted.ValueDigest),
		}
		if extracted.SourceEventID != nil {
			sourceEventID, err := id.Parse(*extracted.SourceEventID)
			if err == nil {
				fact.SourceEventID = sourceEventID
			}
		}
		command.ExtractedFacts = append(command.ExtractedFacts, fact)
	}
	outcome, err := s.cases.SubmitAIProposal(r.Context(), command)
	if err != nil {
		policyProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) evaluatePolicyAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input evaluatePolicyInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	proposalID := id.UUID("")
	if input.ProposalID != nil {
		parsed, err := id.Parse(*input.ProposalID)
		if err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле proposal_ref должно быть UUID: оценка живёт на предложении.", false, nil)
			return
		}
		proposalID = parsed
	}
	if input.ExpectedModeVersion == nil || input.ExpectedOwnerEpoch == nil || input.ExpectedFactsDigest == nil || input.ExpectedWorkItemVersion == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Якоря входных версий обязательны: expected_mode_version, expected_owner_epoch, expected_facts_digest, expected_work_item_version.", false, nil)
		return
	}
	outcome, err := s.cases.EvaluatePolicy(r.Context(), casesAPI.EvaluatePolicyCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ProposalID: proposalID,
		ExpectedModeVersion: *input.ExpectedModeVersion, ExpectedOwnerEpoch: *input.ExpectedOwnerEpoch,
		ExpectedFactsDigest: *input.ExpectedFactsDigest, ExpectedWorkItemVersion: *input.ExpectedWorkItemVersion,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		policyProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// ---------------------------------------------------------------------------
// The operator forms (PRG)
// ---------------------------------------------------------------------------

func (s *Server) submitAIProposalForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		policyFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		policyFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите диалог обращения."})
		return
	}
	workItemID, err := formUUID(r, "work_item_id")
	if err != nil {
		policyFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите элемент работы предложения."})
		return
	}
	turnRef, err := formUUID(r, "turn_ref")
	if err != nil {
		policyFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите принятый turn кейса."})
		return
	}
	intent := r.FormValue("intent")
	candidate := r.FormValue("candidate_answer")
	proposer := r.FormValue("proposer")
	command := casesAPI.SubmitAIProposalCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turnRef,
		Intent: intent, CandidateAnswer: candidate, Proposer: proposer,
		Now: s.clock.Now().UTC(),
	}
	for i := 0; ; i++ {
		key := r.FormValue("missing_key_" + strconv.Itoa(i))
		reason := r.FormValue("missing_reason_" + strconv.Itoa(i))
		if key == "" && reason == "" {
			break
		}
		command.MissingFacts = append(command.MissingFacts, casesDomain.MissingFact{MaterialKey: key, Reason: reason})
	}
	command.IdempotencyKey = formCommandKey("submit-ai-proposal", actor.ID.String(), caseID.String(), conversationID.String(), workItemID.String(), turnRef.String(), intent, candidate, proposer, strconv.Itoa(len(command.MissingFacts)))
	_, err = s.cases.SubmitAIProposal(r.Context(), command)
	if err != nil {
		policyFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "proposal_submitted")
}

func (s *Server) evaluatePolicyForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		policyFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	proposalID, err := formUUID(r, "proposal_ref")
	if err != nil {
		policyFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите предложение оценки."})
		return
	}
	modeVersion, err := formInt64(r, "expected_mode_version")
	if err != nil {
		policyFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сверьте текущую версию лестницы режимов."})
		return
	}
	ownerEpoch, err := formInt64(r, "expected_owner_epoch")
	if err != nil {
		policyFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сверьте текущую эпоху владения."})
		return
	}
	workItemVersion, err := formInt64(r, "expected_work_item_version")
	if err != nil {
		policyFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сверьте текущую версию элемента работы."})
		return
	}
	factsDigest := r.FormValue("expected_facts_digest")
	_, err = s.cases.EvaluatePolicy(r.Context(), casesAPI.EvaluatePolicyCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ProposalID: proposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: formCommandKey("evaluate-policy", actor.ID.String(), caseID.String(), proposalID.String(), strconv.FormatInt(modeVersion, 10), strconv.FormatInt(ownerEpoch, 10), factsDigest, strconv.FormatInt(workItemVersion, 10)),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		policyFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "policy_evaluated")
}
