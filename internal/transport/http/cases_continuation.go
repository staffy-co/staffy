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
// The Story 2.10 transport: the two operator continuation commands (the
// machine JSON contract with the one-time capability answer) and the
// operator PRG forms. The honest boundaries of the slice: the issuance
// answer carries the URL and the token EXACTLY ONCE (the stored outcome
// never carries them — a replayed command answers the honest mask with
// the empty URL, the old link never resurrects, a lost URL means a
// revocation and a fresh issuance); the re-render of the case page after
// the issue form sets no-store and no-referrer (the one-time secret of
// the people invitation precedent). The dictionary and bound refusals are
// the honest 400 rule words; the stale race is the honest 409; the key
// conflict is the honest 409; a foreign tenant is a plain 404; the
// audit-only member is the 403 with the scope words BEFORE every material
// guard (the M1 precedent of 2.5..2.9).
// ---------------------------------------------------------------------------

// continuationProblem maps the continuation command errors onto the
// problem contract (the commitmentProblem mirror).
func continuationProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", "Материал изменился: сверьте текущее состояние и повторите команду.", false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение или элемент работы не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Команды ссылок продолжения требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "continuation command failed", "code", "continuation_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду ссылки продолжения. Повторите позднее.", true, nil)
	}
}

// continuationFormMessage renders the honest flash text of the command
// errors (the commitmentFormMessage mirror).
func continuationFormMessage(err error) string {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		return "Материал изменился: сверьте текущее состояние и повторите команду."
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение или элемент работы не найдены."
	case errors.Is(err, domain.ErrForbidden):
		return "Команды ссылок продолжения требуют полномочия review или configuration."
	}
	return "Команда ссылки продолжения не выполнена. Повторите позднее."
}

// continuationFormError flashes the honest message and returns to the
// case page (the PRG target of every continuation form of the operator
// surface — the case page carries the panel).
func continuationFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, continuationFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

// issueContinuationLinkInput is the request of the issuance command: the
// purpose set (the v1 dictionary subset), the two operator texts, the
// participant reference (the channel address of the client), the lifetime
// bounds. The capability token is NEVER a request field — the server
// generates it once per execution.
type issueContinuationLinkInput struct {
	LinkID           *string   `json:"link_id"`
	Purposes         *[]string `json:"purposes"`
	Question         *string   `json:"question"`
	Explanation      *string   `json:"explanation"`
	ParticipantRef   *string   `json:"participant_ref"`
	TTLMinutes       *int      `json:"ttl_minutes"`
	ConsumptionLimit *int      `json:"consumption_limit"`
}

// issueContinuationLinkAPI answers the issuance command: the fresh answer
// carries the one-time capability material (the URL token and the expiry)
// with the no-store and no-referrer hygiene; the replayed command answers
// the stored outcome with the honest mask — the empty URL (the token is
// never recoverable from the storage).
func (s *Server) issueContinuationLinkAPI(w http.ResponseWriter, r *http.Request) {
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
	var input issueContinuationLinkInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	purposes, question, explanation, participant, ttl, limit, ok := continuationLinkMaterial(w, r, input.Purposes, stringOrEmpty(input.Question), stringOrEmpty(input.Explanation), stringOrEmpty(input.ParticipantRef), input.TTLMinutes, input.ConsumptionLimit)
	if !ok {
		return
	}
	linkID := id.UUID("")
	if input.LinkID != nil && *input.LinkID != "" {
		parsed, err := id.Parse(*input.LinkID)
		if err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле link_id должно быть UUID (та же связь = тот же идентификатор).", false, nil)
			return
		}
		linkID = parsed
	}
	correlation, _ := correlationFromRequest(r)
	issuance, err := s.continuations.IssueContinuationLink(r.Context(), casesAPI.IssueContinuationLinkCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		LinkID: linkID, Purposes: purposes, Question: question, Explanation: explanation,
		ParticipantRef: participant, TTLMinutes: ttl, ConsumptionLimit: limit,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		continuationProblem(w, r, s, err)
		return
	}
	writeContinuationIssuance(w, r, issuance)
}

// writeContinuationIssuance answers the issuance: 201 fresh / 200 replay
// with the one-time secret hygiene (the token never enters a cache or a
// referrer — the people invitation precedent).
func writeContinuationIssuance(w http.ResponseWriter, r *http.Request, issuance casesAPI.ContinuationIssuance) {
	status := http.StatusCreated
	if issuance.Outcome.Replay {
		status = http.StatusOK
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	writeJSON(w, status, issuance)
}

// revokeContinuationLinkInput is the request of the revocation command:
// the honest reason of «прекратить ожидание».
type revokeContinuationLinkInput struct {
	Reason *string `json:"reason"`
}

// revokeContinuationLinkAPI answers the revocation command: the
// append-only row blocks the surface read-time; the second revoke of the
// same link refuses honestly (the expectation ended once).
func (s *Server) revokeContinuationLinkAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	linkID, err := id.Parse(r.PathValue("link_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input revokeContinuationLinkInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	reason := stringOrEmpty(input.Reason)
	if reason == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле reason обязательно: честная причина отзыва (1..200 символов).", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.continuations.RevokeContinuationLink(r.Context(), casesAPI.RevokeContinuationLinkCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		LinkID: linkID, Reason: reason, IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		continuationProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// continuationLinkMaterial validates the shared issuance material of the
// API and the form paths (the honest 400 rule words of the transport
// boundary; the domain guards remain the server-side truth).
func continuationLinkMaterial(w http.ResponseWriter, r *http.Request, purposes *[]string, question, explanation, participant string, ttl, limit *int) ([]string, string, string, string, int, int, bool) {
	words := []string{}
	if purposes != nil {
		words = *purposes
	}
	if len(words) == 0 {
		writeWorkProblem(w, r, "invalid_request", "Поле purposes обязательно: непустой набор целей ссылки (словарь: fact_correction).")
		return nil, "", "", "", 0, 0, false
	}
	if question == "" {
		writeWorkProblem(w, r, "invalid_request", "Поле question обязательно: открытый вопрос клиенту (1..500 рун).")
		return nil, "", "", "", 0, 0, false
	}
	if explanation == "" {
		writeWorkProblem(w, r, "invalid_request", "Поле explanation обязательно: объяснение перехода (1..1000 рун).")
		return nil, "", "", "", 0, 0, false
	}
	if participant == "" {
		writeWorkProblem(w, r, "invalid_request", "Поле participant_ref обязательно: адрес участника (канал клиента, 1..128 символов).")
		return nil, "", "", "", 0, 0, false
	}
	ttlMinutes := 0
	if ttl != nil {
		ttlMinutes = *ttl
	}
	if ttlMinutes < casesDomain.ContinuationTTLMinMinutes || ttlMinutes > casesDomain.ContinuationTTLMaxMinutes {
		writeWorkProblem(w, r, "invalid_request", "Поле ttl_minutes обязательно (5..120 минут — короткоживущая ссылка).")
		return nil, "", "", "", 0, 0, false
	}
	limitValue := casesDomain.DefaultContinuationConsumptionLimit
	if limit != nil && *limit != 0 {
		limitValue = *limit
	}
	if limitValue < casesDomain.ContinuationLimitMin || limitValue > casesDomain.ContinuationLimitMax {
		writeWorkProblem(w, r, "invalid_request", "Поле consumption_limit — целое 1..10 (default 1: ровно одна отправка).")
		return nil, "", "", "", 0, 0, false
	}
	return words, question, explanation, participant, ttlMinutes, limitValue, true
}

// ---------------------------------------------------------------------------
// The operator forms (the issuance re-render + the revoke PRG)
// ---------------------------------------------------------------------------

// issueContinuationLinkForm is the operator PRG-less form of the
// issuance: the honest answer is the re-render of the case page with the
// one-time URL presented ONCE (the people invitation precedent — no
// redirect, the material would be lost in a flash; the no-store and
// no-referrer headers keep the one-time secret out of the caches and the
// referrers). A replayed command (the same derived key with the same
// material — the browser refresh) honestly answers the mask: the page
// states the token is not recoverable, the revocation and the fresh
// issuance are the honest path.
func (s *Server) issueContinuationLinkForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		continuationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	purposes := []string{}
	for _, word := range casesDomain.ContinuationPurposes {
		if r.FormValue("purpose_"+word) == "on" {
			purposes = append(purposes, word)
		}
	}
	ttl, err := formInt64(r, "ttl_minutes")
	if err != nil || ttl < casesDomain.ContinuationTTLMinMinutes || ttl > casesDomain.ContinuationTTLMaxMinutes {
		continuationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Срок жизни ссылки — целые минуты 5..120 (короткоживущая ссылка)."})
		return
	}
	limit := int64(casesDomain.DefaultContinuationConsumptionLimit)
	if raw := strings.TrimSpace(r.FormValue("consumption_limit")); raw != "" {
		if limit, err = strconv.ParseInt(raw, 10, 64); err != nil || limit < casesDomain.ContinuationLimitMin || limit > casesDomain.ContinuationLimitMax {
			continuationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Лимит отправок — целое 1..10 (default 1: ровно одна отправка)."})
			return
		}
	}
	question := r.FormValue("question")
	explanation := r.FormValue("explanation")
	participant := r.FormValue("participant_ref")
	linkID := id.UUID("")
	if value := r.FormValue("link_id"); value != "" {
		if linkID, err = id.Parse(value); err != nil {
			continuationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Идентификатор ссылки — UUID (та же связь = тот же идентификатор)."})
			return
		}
	}
	correlation, _ := correlationFromRequest(r)
	issuance, err := s.continuations.IssueContinuationLink(r.Context(), casesAPI.IssueContinuationLinkCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		LinkID: linkID, Purposes: purposes, Question: question, Explanation: explanation,
		ParticipantRef: participant, TTLMinutes: int(ttl), ConsumptionLimit: int(limit),
		IdempotencyKey: formCommandKey("issue-continuation-link", actor.ID.String(), caseID.String(), workItemID.String(), linkID.String(),
			strings.Join(purposes, ","), question, explanation, participant, strconv.FormatInt(ttl, 10), strconv.FormatInt(limit, 10)),
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		continuationFormError(w, r, tenant, caseID, err)
		return
	}
	s.renderCasePageWithIssuance(w, r, actor, tenant, caseID, issuance)
}

// revokeContinuationLinkForm is the operator PRG form of the revocation:
// the append-only row, the flash, the return to the case page.
func (s *Server) revokeContinuationLinkForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	linkID, err := id.Parse(r.PathValue("link_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	if !formParse(w, r) {
		continuationFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	reason := r.FormValue("reason")
	correlation, _ := correlationFromRequest(r)
	if _, err = s.continuations.RevokeContinuationLink(r.Context(), casesAPI.RevokeContinuationLinkCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		LinkID: linkID, Reason: reason,
		IdempotencyKey: formCommandKey("revoke-continuation-link", actor.ID.String(), caseID.String(), workItemID.String(), linkID.String(), reason),
		Now:            s.clock.Now().UTC(),
	}); err != nil {
		continuationFormError(w, r, tenant, caseID, err)
		return
	}
	http.Redirect(w, r, workFormTarget(tenant, caseID)+"?continuation_revoked=1", http.StatusSeeOther)
}

// renderCasePageWithIssuance re-renders the case page with the one-time
// issuance card (the people invitation precedent; the no-store and
// no-referrer headers — the token is a one-time secret).
func (s *Server) renderCasePageWithIssuance(w http.ResponseWriter, r *http.Request, actor domain.Principal, tenant, caseID id.UUID, issuance casesAPI.ContinuationIssuance) {
	view, err := s.cases.CaseView(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	facts, err := s.cases.FactsView(r.Context(), actor, tenant, caseID)
	if err != nil && !errors.Is(err, domain.ErrForbidden) {
		s.casesPageProblem(w, r, err)
		return
	}
	// The Story 2.14 decision context of the «Контекст передачи» section
	// (the honest degradation of the unauthorized read — the page keeps
	// the rest of the surface).
	contextPanel, contextErr := s.decisionContextOf(r.Context(), actor, tenant, caseID)
	if contextErr != nil {
		s.casesPageProblem(w, r, contextErr)
		return
	}
	flash := flashError(r, tenant)
	if flash != "" {
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	page := prepareCaseView(r, view, facts, flash)
	page.DecisionContext = contextPanel
	data := map[string]any{"TenantID": tenant, "View": page}
	if issuance.URL != "" {
		data["Issuance"] = continuationIssuanceCardOf(issuance)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
	}
	s.render(w, "case", data)
}

// continuationIssuanceCard is the one-time material of the issued link:
// the URL (the only place it ever appears after the command answer), the
// expiry, the honest one-time warning and the weaving instruction.
type continuationIssuanceCard struct {
	URL         string
	ExpiresText string
	TokenHint   string
}

// continuationIssuanceCardOf renders the card of the fresh answer (the
// token IS the URL tail — the card shows the URL once).
func continuationIssuanceCardOf(issuance casesAPI.ContinuationIssuance) continuationIssuanceCard {
	return continuationIssuanceCard{
		URL:         issuance.URL,
		ExpiresText: formatTime(issuance.Expires),
		TokenHint:   "Токен — хвост ссылки (64 символа); в базе хранится только его хеш: восстановление невозможно.",
	}
}
