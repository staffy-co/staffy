package httptransport

import (
	"errors"
	"net/http"
	"strconv"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.8 transport: the machine JSON contract of the objection
// record command and the operator PRG form of the «Возражения и бюджет
// убеждения» panel. The dictionary refusals (the type, the signal, the
// spans, the playbook step discipline) are the honest 400 rule words; the
// span outside the payload is the honest 400 span-not-in-source; the
// stopped persuasion is the 400 persuasion_stopped with the allowed
// endings; the key conflict is the honest 409; a foreign tenant is a plain
// 404; the audit-only member is the 403 with the scope words BEFORE any
// record. The persuasion-state changes are announced with aria-live,
// never with a focus move.
// ---------------------------------------------------------------------------

type recordObjectionInput struct {
	TurnRef       *string   `json:"turn_ref"`
	ObjectionType *string   `json:"objection_type"`
	ClientSignal  *string   `json:"client_signal"`
	Spans         *[]string `json:"spans"`
}

// objectionProblem maps the objection command errors onto the problem
// contract: the dictionary and discipline refusals are the machine-
// readable 400 rule words; the stopped persuasion is the 400
// persuasion_stopped with the budget words and the allowed endings; the
// key conflict is the honest 409; a foreign tenant is a plain 404; the
// audit-only member is the 403 with the scope words.
func objectionProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var stopped casesDomain.ObjectionStoppedError
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &stopped):
		server.problem(w, r, http.StatusBadRequest, "persuasion_stopped", stopped.Reason, false, map[string]string{
			"refusals":              strconv.Itoa(stopped.Refusals),
			"threshold":             strconv.Itoa(stopped.Threshold),
			"current_client_signal": stopped.CurrentSignal,
			"permitted_path":        "neutral_completion,protected_request,human_handoff",
		})
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", "Материал изменился: сверьте текущее состояние и повторите запись.", false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение или элемент работы не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Команды записи возражений требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "objection record failed", "code", "objection_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось записать возражение. Повторите позднее.", true, nil)
	}
}

// objectionFormMessage renders the honest flash text of the record command
// errors (the shared shape of the observation form errors).
func objectionFormMessage(err error) string {
	var stopped casesDomain.ObjectionStoppedError
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &stopped):
		return stopped.Reason
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		return "Материал изменился: страница показывает актуальное состояние — сверьте и повторите запись."
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение или элемент работы не найдены."
	case errors.Is(err, domain.ErrForbidden):
		return "Команды записи возражений требуют полномочия review или configuration."
	}
	return "Возражение не записано. Повторите позднее."
}

// objectionFormError flashes the honest message and returns to the case
// page (the machine-readable words travel inside the text — the form path
// shows them directly).
func objectionFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, objectionFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

func (s *Server) recordObjectionAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input recordObjectionInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	turnRef, err := id.Parse(stringOrEmpty(input.TurnRef))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле turn_ref должно быть UUID принятого события кейса.", false, nil)
		return
	}
	objectionType := stringOrEmpty(input.ObjectionType)
	if objectionType == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле objection_type обязательно: выберите слово словаря типов.", false, nil)
		return
	}
	clientSignal := stringOrEmpty(input.ClientSignal)
	if clientSignal == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле client_signal обязательно: выберите слово словаря сигналов.", false, nil)
		return
	}
	spans := []string{}
	if input.Spans != nil {
		spans = *input.Spans
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.cases.RecordObjection(r.Context(), casesAPI.RecordObjectionCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		TurnRef: turnRef, ObjectionType: objectionType, ClientSignal: clientSignal,
		Spans: spans, IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		objectionProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// ---------------------------------------------------------------------------
// The operator form (PRG)
// ---------------------------------------------------------------------------

// objectionSpanFields are the form span inputs (up to eight verbatim
// excerpts; the empty ones never travel).
var objectionSpanFields = [8]string{"span_1", "span_2", "span_3", "span_4", "span_5", "span_6", "span_7", "span_8"}

// recordObjectionForm records one client objection of the Work Item: the
// turn source (the event select of the case), the dictionary selects (the
// type, the signal), the verbatim span inputs and the hidden objection
// anchor of the rendered form (the deterministic idempotency material of
// the PRG double-submit protection — the anchor moves with every landed
// objection, so a re-rendered form records anew).
func (s *Server) recordObjectionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		objectionFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	turnRef, err := formUUID(r, "turn_ref")
	if err != nil {
		objectionFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите событие-источник возражения (принятый текстовый turn кейса)."})
		return
	}
	objectionType := r.FormValue("objection_type")
	clientSignal := r.FormValue("client_signal")
	spans := []string{}
	for _, field := range objectionSpanFields {
		if value := r.FormValue(field); value != "" {
			spans = append(spans, value)
		}
	}
	objectionAnchor := r.FormValue("expected_objection_no")
	correlation, _ := correlationFromRequest(r)
	_, err = s.cases.RecordObjection(r.Context(), casesAPI.RecordObjectionCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		TurnRef:       turnRef,
		ObjectionType: objectionType, ClientSignal: clientSignal,
		Spans:          spans,
		IdempotencyKey: formCommandKey("record-objection", actor.ID.String(), caseID.String(), workItemID.String(), turnRef.String(), objectionType, clientSignal, objectionAnchor),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		objectionFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "objection_recorded")
}
