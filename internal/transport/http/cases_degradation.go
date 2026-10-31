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
// The Story 2.12 transport: the machine JSON contracts of the two record
// commands (the route degradation revision and the delivery evidence
// event) and the operator PRG forms of the «Состояние маршрутов» panel and
// the reply «Доставка» column. The dictionary refusals (the channel word,
// the cause word, the kind word, the note bound) are the honest 400 rule
// words; the raced revision and the raced event number are the honest 409
// stale signals; the key conflict is the honest 409; a foreign tenant is
// a plain 404; the audit-only member is the 403 with the scope words
// BEFORE every material guard (the M1 precedent of 2.5..2.11). The
// current-slot changes are announced with aria-live, never with a focus
// move.
// ---------------------------------------------------------------------------

// recordRouteDegradationInput is the request of the record command: the
// route (the connection id and the channel word of the 2.1 dictionary),
// the cause word of the fixed dictionary, the clearing mark and the
// operator note. The revision number is NEVER a request field — the store
// derives the next monotonic number of the route under the locks; the
// route itself must exist in the conversations of the case (the store
// route guard).
type recordRouteDegradationInput struct {
	ConnectionID *string `json:"connection_id"`
	Channel      *string `json:"channel"`
	Cause        *string `json:"cause"`
	Cleared      *bool   `json:"cleared"`
	Note         *string `json:"note"`
}

// recordDeliveryEvidenceInput is the request of the evidence command: the
// reply reference and the kind word of the fixed dictionary. The event
// number is NEVER a request field — the store derives the next monotonic
// number of the reply under the locks.
type recordDeliveryEvidenceInput struct {
	ReplyID *string `json:"reply_id"`
	Kind    *string `json:"kind"`
}

// degradationProblem maps the two command errors onto the problem
// contract: the dictionary and bound refusals are the machine-readable
// 400 rule words (including the honest timing gate words
// update_promise_required/update_promise_foreign of the reply); the raced
// revision and event number are the honest 409 stale signals; the key
// conflict is the honest 409; a foreign tenant is a plain 404; the
// audit-only member is the 403 with the scope words.
func degradationProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
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
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Команды записи деградации маршрута и наблюдения доставки требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "degradation record failed", "code", "degradation_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось записать состояние маршрута. Повторите позднее.", true, nil)
	}
}

// degradationFormMessage renders the honest flash text of the command
// errors.
func degradationFormMessage(err error) string {
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
		return "Команды записи деградации маршрута и наблюдения доставки требуют полномочия review или configuration."
	}
	return "Состояние маршрута не записано. Повторите позднее."
}

// degradationFormError flashes the honest message and returns to the case
// page.
func degradationFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, degradationFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The machine JSON contracts
// ---------------------------------------------------------------------------

func (s *Server) recordRouteDegradationAPI(w http.ResponseWriter, r *http.Request) {
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
	var input recordRouteDegradationInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	connection := stringOrEmpty(input.ConnectionID)
	if connection == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле connection_id обязательно: слот деградации живёт на подключении маршрута.", false, nil)
		return
	}
	connectionID, err := id.Parse(connection)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле connection_id обязано быть UUID: слот деградации живёт на подключении маршрута.", false, nil)
		return
	}
	channel := stringOrEmpty(input.Channel)
	if channel == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле channel обязательно: слот деградации живёт на маршруте (подключение, канал).", false, nil)
		return
	}
	cause := stringOrEmpty(input.Cause)
	if cause == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле cause обязательно: слово словаря причин деградации.", false, nil)
		return
	}
	cleared := false
	if input.Cleared != nil {
		cleared = *input.Cleared
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.degradations.RecordRouteDegradation(r.Context(), casesAPI.RecordRouteDegradationCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		ConnectionID: connectionID, Channel: channel,
		Cause: cause, Cleared: cleared, Note: stringOrEmpty(input.Note),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		degradationProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) recordDeliveryEvidenceAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, _, ok := s.casesActorAndCaseAndReply(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input recordDeliveryEvidenceInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	reply := stringOrEmpty(input.ReplyID)
	if reply == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле reply_id обязательно: наблюдение доставки живёт на исходящем ответе.", false, nil)
		return
	}
	replyID, err := id.Parse(reply)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле reply_id обязано быть UUID: наблюдение доставки живёт на исходящем ответе.", false, nil)
		return
	}
	kind := stringOrEmpty(input.Kind)
	if kind == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле kind обязательно: слово словаря наблюдений доставки.", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.degradations.RecordDeliveryEvidence(r.Context(), casesAPI.RecordDeliveryEvidenceCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		ReplyID: replyID, Kind: kind,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		degradationProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// ---------------------------------------------------------------------------
// The operator forms (PRG)
// ---------------------------------------------------------------------------

// recordRouteDegradationForm records one revision of one route
// degradation: the route selects (the connection options of the case's
// conversations and the channel dictionary — the empty words carry NO
// silent defaults), the cause select (the fixed dictionary), the clearing
// checkbox and the note textarea; the hidden degradation anchor of the
// rendered form (the deterministic idempotency material of the PRG
// double-submit protection).
func (s *Server) recordRouteDegradationForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		degradationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	connection := r.FormValue("connection_id")
	if connection == "" {
		degradationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите подключение маршрута: слот деградации живёт на маршруте (подключение, канал)."})
		return
	}
	connectionID, err := id.Parse(connection)
	if err != nil {
		degradationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Подключение маршрута обязано быть UUID."})
		return
	}
	channel := r.FormValue("channel")
	if channel == "" {
		degradationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите канал маршрута (словарь фиксирован)."})
		return
	}
	cause := r.FormValue("cause")
	if cause == "" {
		degradationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите причину деградации (словарь фиксирован)."})
		return
	}
	cleared := r.FormValue("cleared") == "on"
	anchor := r.FormValue("expected_degradation_records")
	correlation, _ := correlationFromRequest(r)
	_, err = s.degradations.RecordRouteDegradation(r.Context(), casesAPI.RecordRouteDegradationCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		ConnectionID: connectionID, Channel: channel,
		Cause: cause, Cleared: cleared, Note: r.FormValue("note"),
		IdempotencyKey: formCommandKey("record-degradation", actor.ID.String(), caseID.String(), connection, channel, cause, clearedText(cleared), r.FormValue("note"), anchor),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		degradationFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "route_degradation_recorded")
}

// recordDeliveryEvidenceForm records one delivery evidence event of one
// outgoing reply: the reply select (the case's outgoing replies — the
// derived honest delivery state renders beside every option) and the kind
// select (the fixed dictionary of the three connector observations).
func (s *Server) recordDeliveryEvidenceForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		degradationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	reply := r.FormValue("reply_id")
	if reply == "" {
		degradationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите ответ: наблюдение доставки живёт на исходящем ответе."})
		return
	}
	replyID, err := id.Parse(reply)
	if err != nil {
		degradationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Ответ обязан быть UUID."})
		return
	}
	kind := r.FormValue("kind")
	if kind == "" {
		degradationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите слово наблюдения доставки (словарь фиксирован — подтверждена, неизвестна или отказ)."})
		return
	}
	correlation, _ := correlationFromRequest(r)
	_, err = s.degradations.RecordDeliveryEvidence(r.Context(), casesAPI.RecordDeliveryEvidenceCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		ReplyID: replyID, Kind: kind,
		IdempotencyKey: formCommandKey("record-evidence", actor.ID.String(), caseID.String(), reply, kind),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		degradationFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "delivery_evidence_recorded")
}

// casesActorAndCaseAndReply resolves the actor, the tenant, the case and
// the reply path anchors of the evidence command surface (the reply is
// the nested resource of the evidence route).
func (s *Server) casesActorAndCaseAndReply(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, id.UUID, id.UUID, bool) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return domain.Principal{}, id.UUID(""), id.UUID(""), id.UUID(""), false
	}
	replyID, err := id.Parse(r.PathValue("reply_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return domain.Principal{}, id.UUID(""), id.UUID(""), id.UUID(""), false
	}
	return actor, tenant, caseID, replyID, true
}
