package httptransport

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.4 transport: the machine JSON contract of the five ownership
// commands and the reply drain, and the operator forms (PRG) of the
// Response Owner Control. The gate refusal of the client reply is
// machine-readable (the problem fields carry the current owner, the epoch
// and the permitted path — the UI shows them directly).
// ---------------------------------------------------------------------------

type takeOwnershipInput struct {
	ExpectedEpoch *int64  `json:"expected_epoch"`
	Reason        *string `json:"reason"`
}

type requestTransferInput struct {
	ExpectedEpoch   *int64  `json:"expected_epoch"`
	ToOwnerKind     *string `json:"to_owner_kind"`
	ToOwnerRef      *string `json:"to_owner_ref"`
	WritableSurface *string `json:"writable_surface"`
	Reason          *string `json:"reason"`
}

type resolveTransferInput struct {
	Decision *string `json:"decision"`
	Reason   *string `json:"reason"`
}

type reassignOwnershipInput struct {
	ExpectedEpoch   *int64  `json:"expected_epoch"`
	ToOwnerKind     *string `json:"to_owner_kind"`
	ToOwnerRef      *string `json:"to_owner_ref"`
	WritableSurface *string `json:"writable_surface"`
	Reason          *string `json:"reason"`
}

type sendClientReplyInput struct {
	ConversationID *string `json:"conversation_id"`
	WorkItemID     *string `json:"work_item_id"`
	ExpectedEpoch  *int64  `json:"expected_epoch"`
	ReplyClass     *string `json:"reply_class"`
	Content        *string `json:"content"`
}

// ownershipProblem maps the ownership command errors onto the problem
// contract: the machine-readable reply gate refusal carries its words as
// the problem fields; the rule refusals are 400; the CAS conflicts are
// 409; a foreign tenant is a plain 404. The Story 2.5 mode and disclosure
// gate refusals map through the shared modeProblem words.
func ownershipProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	var gate casesDomain.ReplyGateError
	var modeGate casesDomain.ReplyModeError
	var disclosureGate casesDomain.ReplyDisclosureError
	var modeConflict casesDomain.ModeConflictError
	switch {
	case errors.As(err, &gate):
		server.problem(w, r, http.StatusForbidden, "reply_gate", gate.Reason, false, map[string]string{
			"current_owner_kind": gate.OwnerKind,
			"current_owner_name": gate.OwnerName,
			"current_epoch":      strconv.FormatInt(gate.CurrentEpoch, 10),
			"permitted_path":     casesDomain.ReplyGatePermittedPath,
		})
	case errors.As(err, &modeConflict):
		modeProblem(w, r, server, err)
	case errors.As(err, &modeGate):
		modeProblem(w, r, server, err)
	case errors.As(err, &disclosureGate):
		modeProblem(w, r, server, err)
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", casesDomain.StaleEpochText, false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение или материал владения не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Команды владения требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "ownership command failed", "code", "ownership_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду. Повторите позднее.", true, nil)
	}
}

// ownershipFormError flashes the honest message and returns to the case
// page (the machine-readable gate words travel inside the text — the form
// path shows them directly).
func ownershipFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	var rule casesDomain.WorkRuleError
	var gate casesDomain.ReplyGateError
	var modeConflict casesDomain.ModeConflictError
	var modeGate casesDomain.ReplyModeError
	var disclosureGate casesDomain.ReplyDisclosureError
	message := "Действие не выполнено. Повторите позднее."
	switch {
	case errors.As(err, &gate):
		message = gate.Reason + " Текущий владелец: " + gate.OwnerName + " (epoch " + strconv.FormatInt(gate.CurrentEpoch, 10) + "). " + casesDomain.ReplyGatePermittedPath
	case errors.As(err, &modeConflict), errors.As(err, &modeGate), errors.As(err, &disclosureGate):
		message = modeFormMessage(err)
	case errors.As(err, &rule):
		message = rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		message = casesDomain.StaleEpochText
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		message = "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		message = "Обращение или материал владения не найдены."
	case errors.Is(err, domain.ErrForbidden):
		message = "Команды владения требуют полномочия review или configuration."
	}
	setFlashError(w, tenant, message)
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

func (s *Server) takeOwnershipAPI(w http.ResponseWriter, r *http.Request) {
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
	var input takeOwnershipInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	outcome, err := s.cases.TakeOwnership(r.Context(), casesAPI.TakeOwnershipCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ExpectedEpoch: int64OrZero(input.ExpectedEpoch), Reason: stringOrEmpty(input.Reason),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) requestTransferAPI(w http.ResponseWriter, r *http.Request) {
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
	var input requestTransferInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	toRef, err := id.Parse(stringOrEmpty(input.ToOwnerRef))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле to_owner_ref должно быть UUID.", false, nil)
		return
	}
	outcome, err := s.cases.RequestTransfer(r.Context(), casesAPI.RequestTransferCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ExpectedEpoch: int64OrZero(input.ExpectedEpoch),
		ToOwnerKind:   stringOrEmpty(input.ToOwnerKind), ToOwnerRef: toRef,
		WritableSurface: stringOrEmpty(input.WritableSurface), Reason: stringOrEmpty(input.Reason),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) resolveTransferAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, requestID, ok := s.casesTransferActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input resolveTransferInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	outcome, err := s.cases.ResolveTransfer(r.Context(), casesAPI.ResolveTransferCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, RequestID: requestID,
		Decision: stringOrEmpty(input.Decision), Reason: stringOrEmpty(input.Reason),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) reassignOwnershipAPI(w http.ResponseWriter, r *http.Request) {
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
	var input reassignOwnershipInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	toRef, err := id.Parse(stringOrEmpty(input.ToOwnerRef))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле to_owner_ref должно быть UUID.", false, nil)
		return
	}
	outcome, err := s.cases.ReassignOwnership(r.Context(), casesAPI.ReassignOwnershipCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ExpectedEpoch: int64OrZero(input.ExpectedEpoch),
		ToOwnerKind:   stringOrEmpty(input.ToOwnerKind), ToOwnerRef: toRef,
		WritableSurface: stringOrEmpty(input.WritableSurface), Reason: stringOrEmpty(input.Reason),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) sendClientReplyAPI(w http.ResponseWriter, r *http.Request) {
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
	var input sendClientReplyInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	conversationID, err := id.Parse(stringOrEmpty(input.ConversationID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле conversation_id должно быть UUID.", false, nil)
		return
	}
	// The CAS expectation is part of the machine contract (the OpenAPI
	// required word): the absent field is an honest 400 — the command
	// never silently degrades to a server-only CAS.
	if input.ExpectedEpoch == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле expected_epoch обязательно: сверьте текущую версию владения.", false, nil)
		return
	}
	// The reply class is the producer's mark (the OpenAPI required word:
	// the machine never classifies the content — no silent default).
	if input.ReplyClass == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", casesDomain.ReplyClassRequiredText, false, nil)
		return
	}
	workItemID := id.UUID("")
	if raw := stringOrEmpty(input.WorkItemID); raw != "" {
		if workItemID, err = id.Parse(raw); err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле work_item_id должно быть UUID.", false, nil)
			return
		}
	}
	outcome, err := s.cases.SendClientReply(r.Context(), casesAPI.SendClientReplyCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID:    workItemID,
		ExpectedEpoch: *input.ExpectedEpoch,
		ReplyClass:    stringOrEmpty(input.ReplyClass),
		Content:       stringOrEmpty(input.Content), IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) drainClientRepliesAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	result, err := s.cases.DrainClientReplies(r.Context(), actor, tenant)
	if err != nil {
		ownershipProblem(w, r, s, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// casesTransferActor resolves the path ids of the transfer resolution
// surface (the case and the request).
func (s *Server) casesTransferActor(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, id.UUID, id.UUID, bool) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return actor, tenant, caseID, id.UUID(""), false
	}
	requestID, err := id.Parse(r.PathValue("request_id"))
	if err != nil || !requestID.Valid() {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Идентификатор запроса передачи должен быть UUID.", false, nil)
		return actor, tenant, caseID, id.UUID(""), false
	}
	return actor, tenant, caseID, requestID, true
}

// ---------------------------------------------------------------------------
// The operator forms (PRG)
// ---------------------------------------------------------------------------

func (s *Server) takeOwnershipForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expected, err := formInt64(r, "expected_epoch")
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сверьте текущую версию владения (epoch)."})
		return
	}
	reason := r.FormValue("reason")
	_, err = s.cases.TakeOwnership(r.Context(), casesAPI.TakeOwnershipCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ExpectedEpoch: expected, Reason: reason,
		IdempotencyKey: formCommandKey("take-ownership", actor.ID.String(), caseID.String(), strconv.FormatInt(expected, 10), reason),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "ownership_taken")
}

func (s *Server) requestTransferForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expected, err := formInt64(r, "expected_epoch")
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сверьте текущую версию владения (epoch)."})
		return
	}
	target := r.FormValue("to_owner_ref")
	toKind := casesDomain.OwnerKindHuman
	if strings.HasPrefix(target, "staffy:") {
		toKind = casesDomain.OwnerKindStaffy
		target = strings.TrimPrefix(target, "staffy:")
	}
	toRef, err := id.Parse(target)
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите цель передачи."})
		return
	}
	surface := r.FormValue("writable_surface")
	reason := r.FormValue("reason")
	_, err = s.cases.RequestTransfer(r.Context(), casesAPI.RequestTransferCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ExpectedEpoch: expected, ToOwnerKind: toKind, ToOwnerRef: toRef,
		WritableSurface: surface, Reason: reason,
		IdempotencyKey: formCommandKey("request-transfer", actor.ID.String(), caseID.String(), strconv.FormatInt(expected, 10), toKind, toRef.String(), surface, reason),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "transfer_requested")
}

func (s *Server) resolveTransferForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, requestID, ok := s.casesTransferActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	decision := r.FormValue("decision")
	reason := r.FormValue("reason")
	_, err := s.cases.ResolveTransfer(r.Context(), casesAPI.ResolveTransferCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, RequestID: requestID,
		Decision: decision, Reason: reason,
		IdempotencyKey: formCommandKey("resolve-transfer", actor.ID.String(), caseID.String(), requestID.String(), decision, reason),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "transfer_resolved")
}

func (s *Server) reassignOwnershipForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expected, err := formInt64(r, "expected_epoch")
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сверьте текущую версию владения (epoch)."})
		return
	}
	target := r.FormValue("to_owner_ref")
	toKind := casesDomain.OwnerKindHuman
	if strings.HasPrefix(target, "staffy:") {
		toKind = casesDomain.OwnerKindStaffy
		target = strings.TrimPrefix(target, "staffy:")
	}
	toRef, err := id.Parse(target)
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите нового владельца."})
		return
	}
	surface := r.FormValue("writable_surface")
	reason := r.FormValue("reason")
	_, err = s.cases.ReassignOwnership(r.Context(), casesAPI.ReassignOwnershipCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ExpectedEpoch: expected, ToOwnerKind: toKind, ToOwnerRef: toRef,
		WritableSurface: surface, Reason: reason,
		IdempotencyKey: formCommandKey("reassign-ownership", actor.ID.String(), caseID.String(), strconv.FormatInt(expected, 10), toKind, toRef.String(), surface, reason),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "ownership_reassigned")
}

func (s *Server) sendClientReplyForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите диалог ответа."})
		return
	}
	content := r.FormValue("content")
	expected, err := formInt64(r, "expected_epoch")
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сверьте текущую версию владения (epoch)."})
		return
	}
	// The reply class (the producer's mark) and the Work Item anchor of
	// the mode gate (the Story 2.5 form material — required without a
	// silent default).
	replyClass, workItemID, err := s.sendClientReplyClass(r)
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, err)
		return
	}
	_, err = s.cases.SendClientReply(r.Context(), casesAPI.SendClientReplyCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID:     workItemID,
		ExpectedEpoch:  expected,
		ReplyClass:     replyClass,
		Content:        content,
		IdempotencyKey: formCommandKey("send-client-reply", actor.ID.String(), caseID.String(), conversationID.String(), workItemID.String(), strconv.FormatInt(expected, 10), replyClass, content),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		ownershipFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "reply_prepared")
}
