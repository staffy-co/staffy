package httptransport

import (
	"net/http"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.3 transport: the machine JSON contract of the four
// verification commands and the operator forms (PRG). The assurance panel,
// the duplicate cards and the binding rows live in the case view template.
// ---------------------------------------------------------------------------

type startVerificationInput struct {
	ConversationID  *string `json:"conversation_id"`
	Kind            *string `json:"kind"`
	Method          *string `json:"method"`
	ProviderCode    *string `json:"provider_code"`
	SubjectHandle   *string `json:"subject_handle"`
	AuthorityScope  *string `json:"authority_scope"`
	EvidenceEventID *string `json:"evidence_event_id"`
	Note            *string `json:"note"`
}

type bindChannelInput struct {
	ConversationID      *string `json:"conversation_id"`
	BoundCaseID         *string `json:"bound_case_id"`
	BoundConversationID *string `json:"bound_conversation_id"`
	Method              *string `json:"method"`
	Purpose             *string `json:"purpose"`
	OnBehalf            *bool   `json:"on_behalf"`
	SubjectHandle       *string `json:"subject_handle"`
	EvidenceEventID     *string `json:"evidence_event_id"`
	Note                *string `json:"note"`
}

type unlinkChannelInput struct {
	Reason *string `json:"reason"`
}

type resolveDuplicateInput struct {
	Decision            *string `json:"decision"`
	Note                *string `json:"note"`
	ConversationID      *string `json:"conversation_id"`
	BoundConversationID *string `json:"bound_conversation_id"`
	SubjectHandle       *string `json:"subject_handle"`
}

func (s *Server) startVerificationAPI(w http.ResponseWriter, r *http.Request) {
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
	var input startVerificationInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	conversationID, err := id.Parse(stringOrEmpty(input.ConversationID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле conversation_id должно быть UUID.", false, nil)
		return
	}
	evidenceEventID := id.UUID("")
	if evidence := stringOrEmpty(input.EvidenceEventID); evidence != "" {
		if evidenceEventID, err = id.Parse(evidence); err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле evidence_event_id должно быть UUID.", false, nil)
			return
		}
	}
	outcome, err := s.cases.StartVerification(r.Context(), casesAPI.StartVerificationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Kind: stringOrEmpty(input.Kind), Method: stringOrEmpty(input.Method), ProviderCode: stringOrEmpty(input.ProviderCode),
		SubjectHandle: stringOrEmpty(input.SubjectHandle), AuthorityScope: stringOrEmpty(input.AuthorityScope),
		EvidenceEventID: evidenceEventID, Note: stringOrEmpty(input.Note),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) bindChannelAPI(w http.ResponseWriter, r *http.Request) {
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
	var input bindChannelInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	conversationID, err := id.Parse(stringOrEmpty(input.ConversationID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле conversation_id должно быть UUID.", false, nil)
		return
	}
	boundCaseID, err := id.Parse(stringOrEmpty(input.BoundCaseID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле bound_case_id должно быть UUID.", false, nil)
		return
	}
	boundConversationID, err := id.Parse(stringOrEmpty(input.BoundConversationID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле bound_conversation_id должно быть UUID.", false, nil)
		return
	}
	evidenceEventID := id.UUID("")
	if evidence := stringOrEmpty(input.EvidenceEventID); evidence != "" {
		if evidenceEventID, err = id.Parse(evidence); err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле evidence_event_id должно быть UUID.", false, nil)
			return
		}
	}
	outcome, err := s.cases.BindChannel(r.Context(), casesAPI.BindChannelCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		BoundCaseID: boundCaseID, BoundConversationID: boundConversationID,
		Method: stringOrEmpty(input.Method), Purpose: stringOrEmpty(input.Purpose),
		OnBehalf: boolOrFalse(input.OnBehalf), SubjectHandle: stringOrEmpty(input.SubjectHandle),
		EvidenceEventID: evidenceEventID, Note: stringOrEmpty(input.Note),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) unlinkChannelAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, bindingID, ok := s.casesBindingActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input unlinkChannelInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	outcome, err := s.cases.UnlinkChannel(r.Context(), casesAPI.UnlinkChannelCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, BindingID: bindingID,
		Reason:         stringOrEmpty(input.Reason),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) resolveDuplicateAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, candidateID, ok := s.casesDuplicateActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input resolveDuplicateInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	conversationID, err := id.Parse(stringOrEmpty(input.ConversationID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле conversation_id должно быть UUID.", false, nil)
		return
	}
	boundConversationID, err := id.Parse(stringOrEmpty(input.BoundConversationID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле bound_conversation_id должно быть UUID.", false, nil)
		return
	}
	outcome, err := s.cases.ResolveDuplicate(r.Context(), casesAPI.ResolveDuplicateCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, CandidateID: candidateID,
		Decision: stringOrEmpty(input.Decision), Note: stringOrEmpty(input.Note),
		ConversationID: conversationID, BoundConversationID: boundConversationID,
		SubjectHandle:  stringOrEmpty(input.SubjectHandle),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// casesBindingActor resolves the path ids of the unlink surface (the case
// and the binding).
func (s *Server) casesBindingActor(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, id.UUID, id.UUID, bool) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return actor, tenant, caseID, id.UUID(""), false
	}
	bindingID, err := id.Parse(r.PathValue("binding_id"))
	if err != nil || !bindingID.Valid() {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Идентификатор связи должен быть UUID.", false, nil)
		return actor, tenant, caseID, id.UUID(""), false
	}
	return actor, tenant, caseID, bindingID, true
}

// casesDuplicateActor resolves the path ids of the duplicate resolution
// surface (the case and the candidate).
func (s *Server) casesDuplicateActor(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, id.UUID, id.UUID, bool) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return actor, tenant, caseID, id.UUID(""), false
	}
	candidateID, err := id.Parse(r.PathValue("candidate_id"))
	if err != nil || !candidateID.Valid() {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Идентификатор кандидата должен быть UUID.", false, nil)
		return actor, tenant, caseID, id.UUID(""), false
	}
	return actor, tenant, caseID, candidateID, true
}

// ---------------------------------------------------------------------------
// The operator forms (PRG)
// ---------------------------------------------------------------------------

func (s *Server) startVerificationForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите диалог проверки."})
		return
	}
	kind := r.FormValue("kind")
	method := r.FormValue("method")
	providerCode := r.FormValue("provider_code")
	subject := r.FormValue("subject_handle")
	scope := r.FormValue("authority_scope")
	note := r.FormValue("note")
	evidenceEventID := id.UUID("")
	if evidence := r.FormValue("evidence_event_id"); evidence != "" {
		if evidenceEventID, err = id.Parse(evidence); err != nil {
			workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите событие-evidence или оставьте поле пустым."})
			return
		}
	}
	_, err = s.cases.StartVerification(r.Context(), casesAPI.StartVerificationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Kind: kind, Method: method, ProviderCode: providerCode,
		SubjectHandle: subject, AuthorityScope: scope,
		EvidenceEventID: evidenceEventID, Note: note,
		IdempotencyKey: formCommandKey("start-verification", actor.ID.String(), caseID.String(), conversationID.String(), kind, method, providerCode, subject, scope, note, evidenceEventID.String()),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "verification_recorded")
}

func (s *Server) bindChannelForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите целевой диалог."})
		return
	}
	boundCaseID, err := formUUID(r, "bound_case_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите кейс связываемого разговора."})
		return
	}
	boundConversationID, err := formUUID(r, "bound_conversation_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите связываемый разговор."})
		return
	}
	purpose := r.FormValue("purpose")
	subject := r.FormValue("subject_handle")
	note := r.FormValue("note")
	onBehalf := r.FormValue("on_behalf") == "on_behalf"
	evidenceEventID := id.UUID("")
	if evidence := r.FormValue("evidence_event_id"); evidence != "" {
		if evidenceEventID, err = id.Parse(evidence); err != nil {
			workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите событие-evidence или оставьте поле пустым."})
			return
		}
	}
	_, err = s.cases.BindChannel(r.Context(), casesAPI.BindChannelCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		BoundCaseID: boundCaseID, BoundConversationID: boundConversationID,
		Method: casesDomain.BindingMethodConsentHandoff, Purpose: purpose, OnBehalf: onBehalf,
		SubjectHandle: subject, EvidenceEventID: evidenceEventID, Note: note,
		IdempotencyKey: formCommandKey("bind-channel", actor.ID.String(), caseID.String(), conversationID.String(), boundCaseID.String(), boundConversationID.String(), purpose, subject, note),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "channel_bound")
}

func (s *Server) unlinkChannelForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, bindingID, ok := s.casesBindingActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	reason := r.FormValue("reason")
	_, err := s.cases.UnlinkChannel(r.Context(), casesAPI.UnlinkChannelCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, BindingID: bindingID,
		Reason:         reason,
		IdempotencyKey: formCommandKey("unlink-channel", actor.ID.String(), caseID.String(), bindingID.String(), reason),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "channel_unlinked")
}

func (s *Server) resolveDuplicateForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, candidateID, ok := s.casesDuplicateActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	decision := r.FormValue("decision")
	note := r.FormValue("note")
	subject := r.FormValue("subject_handle")
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите целевой диалог существующего кейса."})
		return
	}
	boundConversationID, err := formUUID(r, "bound_conversation_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите связываемый диалог этого кейса."})
		return
	}
	_, err = s.cases.ResolveDuplicate(r.Context(), casesAPI.ResolveDuplicateCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, CandidateID: candidateID,
		Decision: decision, Note: note, ConversationID: conversationID,
		BoundConversationID: boundConversationID, SubjectHandle: subject,
		IdempotencyKey: formCommandKey("resolve-duplicate", actor.ID.String(), caseID.String(), candidateID.String(), decision, note),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "duplicate_resolved")
}

// boolOrFalse reads an optional bool of the machine contract.
func boolOrFalse(value *bool) bool {
	return value != nil && *value
}
