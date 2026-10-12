package httptransport

import (
	"errors"
	"net/http"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.7 transport: the machine JSON contract of the owning-system
// read command and the operator PRG form of the «Материальные поля Owning
// System» panel. The field outside the matrix is the machine-readable 400
// field-not-in-matrix with the field dictionary in the text; the terminal
// Work Item is the honest 400 work rule; the audit-only member gets the
// honest 403 with the scope words BEFORE any record; a foreign tenant is a
// plain 404. The observation state changes are announced with aria-live,
// never with a focus move.
// ---------------------------------------------------------------------------

type readOwningSystemInput struct {
	MaterialKey *string `json:"material_key"`
}

// observationProblem maps the read command errors onto the problem
// contract: the field outside the matrix is the machine-readable 400 with
// the dictionary; the rule refusals (the terminal Work Item, the unconfigured
// contour) are 400; the key conflict is the honest 409; a foreign tenant is
// a plain 404; the audit-only member is the 403 with the scope words.
func observationProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", "Материал изменился: сверьте текущее состояние и повторите чтение.", false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение или элемент работы не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Команды чтения owning-system требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "owning-system read failed", "code", "observation_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить чтение. Повторите позднее.", true, nil)
	}
}

// observationFormMessage renders the honest flash text of the read command
// errors (the shared shape of the policy form errors).
func observationFormMessage(err error) string {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		return "Материал изменился: страница показывает актуальное состояние — сверьте и повторите чтение."
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение или элемент работы не найдены."
	case errors.Is(err, domain.ErrForbidden):
		return "Команды чтения owning-system требуют полномочия review или configuration."
	}
	return "Чтение не выполнено. Повторите позднее."
}

// observationFormError flashes the honest message and returns to the case
// page (the machine-readable words travel inside the text — the form path
// shows them directly).
func observationFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, observationFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

func (s *Server) readOwningSystemAPI(w http.ResponseWriter, r *http.Request) {
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
	var input readOwningSystemInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	materialKey := stringOrEmpty(input.MaterialKey)
	if materialKey == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле material_key обязательно: выберите поле словаря матрицы State/Authority v1.", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.cases.ReadOwningSystem(r.Context(), casesAPI.ReadOwningSystemCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: materialKey, IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		observationProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// ---------------------------------------------------------------------------
// The operator form (PRG)
// ---------------------------------------------------------------------------

// readOwningSystemForm reads one material field of the Work Item from its
// owning system: the select of the matrix dictionary fields (required), the
// hidden observation anchor of the rendered form (the deterministic
// idempotency material of the PRG double-submit protection — the anchor
// moves with every landed observation, so a re-rendered form reads anew).
func (s *Server) readOwningSystemForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		observationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	materialKey := r.FormValue("material_key")
	observationAnchor := r.FormValue("expected_observation_no")
	correlation, _ := correlationFromRequest(r)
	_, err := s.cases.ReadOwningSystem(r.Context(), casesAPI.ReadOwningSystemCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey:    materialKey,
		IdempotencyKey: formCommandKey("owning-system-read", actor.ID.String(), caseID.String(), workItemID.String(), materialKey, observationAnchor),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		observationFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "owning_system_read")
}
