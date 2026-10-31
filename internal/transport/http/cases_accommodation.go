package httptransport

import (
	"errors"
	"net/http"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.11 transport: the machine JSON contract of the accommodation
// record command and the operator PRG form of the «Предпочтения связи»
// panel. The dictionary refusals (the key word, the flag/valued shape, the
// channel word, the bounds, the clearing discipline) are the honest 400
// rule words; the raced revision is the honest 409 stale signal; the key
// conflict is the honest 409; a foreign tenant is a plain 404; the
// audit-only member is the 403 with the scope words BEFORE every material
// guard (the M1 precedent of 2.5..2.8). The current-set changes are
// announced with aria-live, never with a focus move.
// ---------------------------------------------------------------------------

// recordAccommodationInput is the request of the record command: the
// preference key word (the fixed dictionary), the valued material of the
// valued keys, the clearing mark and the operator note. The revision
// number is NEVER a request field — the store derives the next monotonic
// number of the key under the locks.
type recordAccommodationInput struct {
	AccommodationKey   *string `json:"accommodation_key"`
	AccommodationValue *string `json:"value"`
	Cleared            *bool   `json:"cleared"`
	Note               *string `json:"note"`
}

// accommodationProblem maps the accommodation command errors onto the
// problem contract: the dictionary and bound refusals are the
// machine-readable 400 rule words; the raced revision is the honest 409
// stale signal; the key conflict is the honest 409; a foreign tenant is a
// plain 404; the audit-only member is the 403 with the scope words.
func accommodationProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", "Материал изменился: сверьте текущее состояние и повторите запись.", false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение не найдено.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Команды записи предпочтений связи требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "accommodation record failed", "code", "accommodation_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось записать предпочтение связи. Повторите позднее.", true, nil)
	}
}

// accommodationFormMessage renders the honest flash text of the record
// command errors (the shared shape of the objection form errors).
func accommodationFormMessage(err error) string {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		return "Материал изменился: страница показывает актуальное состояние — сверьте и повторите запись."
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение не найдено."
	case errors.Is(err, domain.ErrForbidden):
		return "Команды записи предпочтений связи требуют полномочия review или configuration."
	}
	return "Предпочтение связи не записано. Повторите позднее."
}

// accommodationFormError flashes the honest message and returns to the
// case page (the machine-readable words travel inside the text — the form
// path shows them directly).
func accommodationFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, accommodationFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

func (s *Server) recordAccommodationAPI(w http.ResponseWriter, r *http.Request) {
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
	var input recordAccommodationInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	accommodationKey := stringOrEmpty(input.AccommodationKey)
	if accommodationKey == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле accommodation_key обязательно: выберите слово словаря ключей предпочтений.", false, nil)
		return
	}
	cleared := false
	if input.Cleared != nil {
		cleared = *input.Cleared
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.accommodations.RecordAccommodation(r.Context(), casesAPI.RecordAccommodationCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		AccommodationKey:   accommodationKey,
		AccommodationValue: stringOrEmpty(input.AccommodationValue),
		Cleared:            cleared,
		Note:               stringOrEmpty(input.Note),
		IdempotencyKey:     key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		accommodationProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// ---------------------------------------------------------------------------
// The operator form (PRG)
// ---------------------------------------------------------------------------

// recordAccommodationForm records one revision of one communication
// preference: the key select (the fixed dictionary — the empty word «—
// выберите ключ —» carries NO silent default: the operator chooses
// consciously), the value input (the valued keys only — the server guard
// is the truth), the clearing checkbox (its own revision — never together
// with a value) and the note textarea; the hidden accommodation anchor of
// the rendered form (the deterministic idempotency material of the PRG
// double-submit protection — the anchor moves with every landed record,
// so a re-rendered form records anew).
func (s *Server) recordAccommodationForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		accommodationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	accommodationKey := r.FormValue("accommodation_key")
	if accommodationKey == "" {
		accommodationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите ключ предпочтения (словарь фиксирован — пустого ключа не бывает)."})
		return
	}
	cleared := r.FormValue("cleared") == "on"
	accommodationAnchor := r.FormValue("expected_accommodation_records")
	correlation, _ := correlationFromRequest(r)
	_, err := s.accommodations.RecordAccommodation(r.Context(), casesAPI.RecordAccommodationCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		AccommodationKey:   accommodationKey,
		AccommodationValue: r.FormValue("value"),
		Cleared:            cleared,
		Note:               r.FormValue("note"),
		IdempotencyKey:     formCommandKey("record-accommodation", actor.ID.String(), caseID.String(), accommodationKey, r.FormValue("value"), clearedText(cleared), r.FormValue("note"), accommodationAnchor),
		Now:                s.clock.Now().UTC(),
	})
	if err != nil {
		accommodationFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "accommodation_recorded")
}

// clearedText renders the clearing word of the form command key (the
// deterministic idempotency material — the clearing mark is part of it).
func clearedText(cleared bool) string {
	if cleared {
		return "cleared"
	}
	return "active"
}
