package httptransport

import (
	"errors"
	"net/http"
	"strings"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.16 transport: the machine JSON contract of the operator
// update promise and the PRG form «Обещать обновление» of the handoff
// section. The deadline word of the fixed dictionary is the ONLY request
// material (the honest unknown word included — the promise without a
// fabricated timestamp); the promised time and the promise number are
// the SERVER truths — the strict decode refuses them (the store computes
// the time under the locks and derives the NEXT monotonic number); the
// round anchor is NEVER a request field (the store resolves the open
// lifecycle with its own mirror words). The dictionary refusals are the
// honest 400 rule words; the raced promise number and the idempotency
// key conflicts are the honest 409; a foreign tenant is a plain 404; an
// audit-only member is the 403 with the scope words BEFORE every
// material guard (the M1 precedent of 2.5..2.15 — the promise is the
// duty of communication, never gated by the ownership). The PRG form
// lands on the #handoff anchor (the section position survives, the
// exact scroll position does not — the no-JS boundary of the slice);
// the waiting block renders only with the promise material (the control
// branch renders byte-identically).
// ---------------------------------------------------------------------------

// promiseUpdateInput is the request of the operator update promise: the
// deadline word of the fixed dictionary — the ONLY request material.
// The promised time and the promise number are NEVER request fields:
// the strict decode refuses them (the store computes the time under the
// locks and derives the NEXT monotonic number; a fabricated timestamp
// NEVER passes — the material CHECK pair of the table owns the
// invariant).
type promiseUpdateInput struct {
	DeadlineWord *string `json:"deadline_word"`
}

// waitingUpdateProblem maps the promise command errors onto the problem
// contract: the dictionary and round refusals are the machine-readable
// 400 rule words (the store mirror words of the declined and closed
// rounds); the raced promise number is the honest 409 stale signal; the
// key conflict is the honest 409; a foreign tenant is a plain 404; the
// audit-only member is the 403 with the scope words BEFORE every
// material guard.
func waitingUpdateProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", "Материал изменился: сверьте текущее состояние и повторите команду.", false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение не найдено.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Обещание обновления требует полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "promise command failed", "code", "promise_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить обещание обновления. Повторите позднее.", true, nil)
	}
}

// promiseUpdateFormMessage renders the honest flash text of the promise
// command errors (the PRG error path — the message rides the flash
// cookie, the return lands on the #handoff anchor).
func promiseUpdateFormMessage(err error) string {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		return "Материал изменился: страница показывает актуальное состояние — сверьте и повторите команду."
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение не найдено."
	case errors.Is(err, domain.ErrForbidden):
		return "Обещание обновления требует полномочия review или configuration."
	}
	return "Обещание обновления не записано. Повторите позднее."
}

// promiseUpdateFormError flashes the honest message and returns to the
// case page at the #handoff anchor (the PRG target of the promise form
// — the section position survives, the scroll position does not).
func promiseUpdateFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, promiseUpdateFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID)+"#handoff", http.StatusSeeOther)
}

// promiseUpdateAPI answers the machine POST of ONE append-only promise
// row: the fresh record is the honest 201, the exact repeat the 200 with
// the Idempotency-Replayed header, the dictionary and round refusals the
// 400 rule words, the raced conflicts the 409. The promised time and the
// promise number are NEVER request fields (the strict decode refusal —
// the server truths of the locks).
func (s *Server) promiseUpdateAPI(w http.ResponseWriter, r *http.Request) {
	if s.waitingUpdates == nil {
		s.problem(w, r, http.StatusServiceUnavailable, "cases_unavailable", "Контур обращений не сконфигурирован.", false, nil)
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
	var input promiseUpdateInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	word := stringOrEmpty(input.DeadlineWord)
	if word == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле deadline_word обязательно: слово дедлайна словаря {update_15m, update_1h, update_4h, update_unknown}.", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.waitingUpdates.PromiseUpdate(r.Context(), casesAPI.PromiseUpdateCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		DeadlineWord:   word,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		waitingUpdateProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// promiseUpdateForm answers the PRG form «Обещать обновление» of the
// handoff section: the deadline select of the fixed dictionary (the
// honest unknown word included — no silent defaults) and the hidden
// promise anchor of the rendered form (the deterministic idempotency
// material of the PRG double-submit protection). The double submit of
// the SAME material is the honest replay (the composed key covers the
// whole form material); the outcome lands on the #handoff anchor (the
// waiting block appears at the section — the position survives, the
// scroll does not).
func (s *Server) promiseUpdateForm(w http.ResponseWriter, r *http.Request) {
	if s.waitingUpdates == nil {
		s.problem(w, r, http.StatusServiceUnavailable, "cases_unavailable", "Контур обращений не сконфигурирован.", false, nil)
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		promiseUpdateFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	word := strings.TrimSpace(r.FormValue("deadline_word"))
	if word == "" {
		promiseUpdateFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите слово дедлайна обновления (словарь фиксирован — честное «срок неизвестен» включено)."})
		return
	}
	anchor := r.FormValue("expected_update_promises")
	correlation, _ := correlationFromRequest(r)
	_, err := s.waitingUpdates.PromiseUpdate(r.Context(), casesAPI.PromiseUpdateCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		DeadlineWord:   word,
		IdempotencyKey: formCommandKey("promise-update", actor.ID.String(), caseID.String(), word, anchor),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		promiseUpdateFormError(w, r, tenant, caseID, err)
		return
	}
	http.Redirect(w, r, workFormTarget(tenant, caseID)+"?update_promised=1#handoff", http.StatusSeeOther)
}
