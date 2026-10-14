package httptransport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.10 public protected surface: the opaque token IS the
// credential (no session, no principal — the playback-permit precedent of
// the request-carried credentials). GET /continue/{token} renders the
// surface of the moment: the valid word renders the form page (the header
// with the tenant name of the identity service and the role constant, the
// explanation, the confirmed facts of the Work Item of the link, the open
// question, the correction form and the persistent «Позвать человека» /
// «Вернуться в разговор» actions); the consumed word renders the terminal
// page; every other word — the expired, the revoked, the owner-stale and
// the unknown-token/parse refusals — renders the SINGLE generic refusal
// without disclosure (the causes are never distinguished to the client).
// The client POSTs are PRG (303 + the landing flag); a validation error
// re-renders the form with the preserved input (the data never
// disappears); a dead-link action redirects to the plain GET (the
// derived state renders the honest page — the double-click of the limit
// is the terminal page by design). The pages carry no-store and
// no-referrer (the token is the URL itself).
// ---------------------------------------------------------------------------

// continuationFormInput is the preserved material of the correction form:
// the honest re-render of a validation error keeps the chosen fact and
// the typed value (the data never disappears on a refusal).
type continuationFormInput struct {
	FactID string
	Value  string
	Error  string
}

// continuationFactOption is one confirmed fact of the correction select.
type continuationFactOption struct {
	Value string
	Text  string
}

// continuationPageView is the public page model (the single template of
// the three surfaces — the valid form page, the consumed terminal page,
// the generic refusal).
type continuationPageView struct {
	Kind          string
	Valid         bool
	Consumed      bool
	Invalid       bool
	TenantName    string
	HasTenantName bool
	RoleWord      string
	Explanation   string
	Question      string
	FactOptions   []continuationFactOption
	ExpiresText   string
	LimitLeft     string
	Notice        string
	Preserved     continuationFormInput
	CorrectURL    string
	HumanURL      string
	ReturnURL     string
}

// continuationPage answers GET /continue/{token}: the surface of the
// moment (the valid and the consumed words render 200; the invalid word
// renders the generic page with 404 — the honest web semantics of a
// resource the holder cannot enter).
func (s *Server) continuationPage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	surface, err := s.continuations.ContinueSurface(r.Context(), token, s.clock.Now().UTC())
	if err != nil {
		s.continuationServerError(w, r)
		return
	}
	status := http.StatusOK
	if surface.Kind == casesAPI.ContinuationSurfaceInvalid {
		status = http.StatusNotFound
	}
	s.renderContinuation(w, r, token, surface, status, continuationFormInput{})
}

// continuationCorrectForm answers POST /continue/{token}/correct: the
// PRG submission of one fact correction (the landing flag announces
// «отправлено — оператор проверит»); the dead-link refusals redirect to
// the plain GET (the derived page is the honest answer — the double-click
// of the limit is the terminal page); the validation errors re-render the
// form with the preserved input (the honest 400 — the data never
// disappears).
func (s *Server) continuationCorrectForm(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if !formParse(w, r) {
		http.Redirect(w, r, casesAPI.ContinuationURLPath(token), http.StatusSeeOther)
		return
	}
	input := continuationFormInput{FactID: r.FormValue("fact_id"), Value: r.FormValue("value")}
	factID, err := id.Parse(strings.TrimSpace(input.FactID))
	if err != nil {
		input.Error = "Выберите факт из списка подтверждённых фактов этой страницы."
		s.continuationReRender(w, r, token, input)
		return
	}
	// The text modality of the slice: the client value is one JSON string
	// (the plain textarea text becomes the quoted canonical value — the
	// same 2.2 value guards as the operator path).
	rawValue, err := json.Marshal(input.Value)
	if err != nil {
		s.continuationServerError(w, r)
		return
	}
	if _, err := s.continuations.SubmitCorrection(r.Context(), token, factID, rawValue, s.clock.Now().UTC()); err != nil {
		if continuationRedirectsToSurface(err) {
			http.Redirect(w, r, casesAPI.ContinuationURLPath(token), http.StatusSeeOther)
			return
		}
		var rule casesDomain.WorkRuleError
		if errors.As(err, &rule) {
			input.Error = rule.Reason
			s.continuationReRender(w, r, token, input)
			return
		}
		s.continuationServerError(w, r)
		return
	}
	http.Redirect(w, r, casesAPI.ContinuationURLPath(token)+"?submitted=1", http.StatusSeeOther)
}

// continuationHumanForm answers POST /continue/{token}/human: the PRG
// event (the use row without material and without consumption — the
// landing announces «оператор свяжется»); the dead-link refusal redirects
// to the plain GET.
func (s *Server) continuationHumanForm(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if err := s.continuations.SubmitHuman(r.Context(), token, s.clock.Now().UTC()); err != nil {
		s.continuationEventFallback(w, r, token, err)
		return
	}
	http.Redirect(w, r, casesAPI.ContinuationURLPath(token)+"?human=1", http.StatusSeeOther)
}

// continuationReturnForm answers POST /continue/{token}/return: the PRG
// event (the use row without material and without consumption — the
// landing announces «возвращайтесь в ваш разговор»); the dead-link
// refusal redirects to the plain GET.
func (s *Server) continuationReturnForm(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if err := s.continuations.SubmitReturn(r.Context(), token, s.clock.Now().UTC()); err != nil {
		s.continuationEventFallback(w, r, token, err)
		return
	}
	http.Redirect(w, r, casesAPI.ContinuationURLPath(token)+"?returned=1", http.StatusSeeOther)
}

// continuationEventFallback maps the event failures: the dead-link
// refusals redirect to the plain GET (the derived page is the honest
// answer); the unknown failures render the honest server-error page.
func (s *Server) continuationEventFallback(w http.ResponseWriter, r *http.Request, token string, err error) {
	if continuationRedirectsToSurface(err) {
		http.Redirect(w, r, casesAPI.ContinuationURLPath(token), http.StatusSeeOther)
		return
	}
	s.continuationServerError(w, r)
}

// continuationRedirectsToSurface answers whether the failure of a public
// action answers the DERIVED page honestly (the dead link: the unknown
// token, the foreign tenant, the expired, the revoked, the consumed
// limit, the owner-stale word and the race loser — the redirect renders
// the generic refusal or the terminal page; the causes are never
// distinguished in the answer).
func continuationRedirectsToSurface(err error) bool {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, casesDomain.ErrStaleVersion):
		return true
	case errors.As(err, &rule):
		return rule.Reason == casesDomain.ContinuationNotUsableText
	}
	return false
}

// continuationReRender re-renders the valid form page with the preserved
// input and the honest error (the 400 of the validation refusals — the
// data never disappears; the aria-live region announces the error and the
// focus moves to the description).
func (s *Server) continuationReRender(w http.ResponseWriter, r *http.Request, token string, input continuationFormInput) {
	surface, err := s.continuations.ContinueSurface(r.Context(), token, s.clock.Now().UTC())
	if err != nil {
		s.continuationServerError(w, r)
		return
	}
	status := http.StatusBadRequest
	if surface.Kind == casesAPI.ContinuationSurfaceInvalid {
		// The link died while the client typed: the derived page is the
		// honest answer (the generic refusal without disclosure).
		status = http.StatusNotFound
		input.Error = ""
	} else if surface.Kind != casesAPI.ContinuationSurfaceValid {
		// The terminal page answers the dead-limit submission honestly.
		input.Error = ""
	}
	s.renderContinuation(w, r, token, surface, status, input)
}

// continuationServerError renders the honest server-error page of the
// public surface (the infra failures never disclose anything).
func (s *Server) continuationServerError(w http.ResponseWriter, r *http.Request) {
	correlationID, _ := correlationFromRequest(r)
	s.logger.ErrorContext(r.Context(), "continuation surface failed", "code", "continuation_surface_failed", "correlation_id", correlationID.String())
	s.renderStatus(w, http.StatusInternalServerError, "continuation", map[string]any{
		"View": continuationPageView{Invalid: true, Notice: "Страница недоступна. Повторите позднее — или свяжитесь с нами по каналу обращения."},
	})
}

// renderContinuation composes the page model and renders the single
// public template: the tenant name of the identity service (the header of
// the valid page — the honest degradation omits the word when the read
// fails), the role constant of the slice, the landing announcements of
// the PRG flags and the preserved form material.
func (s *Server) renderContinuation(w http.ResponseWriter, r *http.Request, token string, surface casesAPI.ContinuationSurface, status int, input continuationFormInput) {
	view := continuationPageView{
		Kind: surface.Kind, RoleWord: casesAPI.ContinuationRoleWord,
		CorrectURL: casesAPI.ContinuationURLPath(token) + "/correct",
		HumanURL:   casesAPI.ContinuationURLPath(token) + "/human",
		ReturnURL:  casesAPI.ContinuationURLPath(token) + "/return",
		Preserved:  input,
	}
	switch surface.Kind {
	case casesAPI.ContinuationSurfaceValid:
		view.Valid = true
		view.Explanation = surface.Explanation
		view.Question = surface.Question
		view.ExpiresText = formatTime(surface.ExpiresAt)
		view.LimitLeft = pluralizeContinuationLimit(surface.LimitLeft)
		view.FactOptions = make([]continuationFactOption, 0, len(surface.Facts))
		for _, fact := range surface.Facts {
			view.FactOptions = append(view.FactOptions, continuationFactOption{
				Value: fact.FactID.String(),
				Text:  fact.MaterialKey + " — " + fact.ValueText,
			})
		}
		if tenantID, _, err := casesDomain.ParseContinuationToken(token); err == nil && s.tenantNamer != nil {
			if name, nameErr := s.tenantNamer.TenantName(r.Context(), tenantID); nameErr == nil && name != "" {
				view.TenantName = name
				view.HasTenantName = true
			}
		}
	case casesAPI.ContinuationSurfaceConsumed:
		view.Consumed = true
	default:
		view.Invalid = true
	}
	// The landing announcements of the PRG flags (aria-live polite — the
	// statuses are announced; the focus moves to the notice).
	switch r.URL.Query().Get("submitted") {
	case "1":
		view.Notice = "Исправление отправлено — оператор проверит и подтвердит его штатным путём. Ссылка завершена: повторная отправка по ней невозможна."
	}
	switch r.URL.Query().Get("human") {
	case "1":
		view.Notice = "Запрос передан: оператор свяжется с вами."
	}
	switch r.URL.Query().Get("returned") {
	case "1":
		view.Notice = "Возвращайтесь в ваш разговор: обращение и всё сохранённое продолжаются, повторный набор вопросов не нужен."
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	s.renderStatus(w, status, "continuation", map[string]any{"View": view})
}

// pluralizeContinuationLimit renders the honest remaining-submissions
// word of the form note.
func pluralizeContinuationLimit(left int) string {
	if left == 1 {
		return "1 отправка"
	}
	return pluralRu(left, "отправка", "отправки", "отправок")
}

// pluralRu renders the Russian plural forms of one counted noun (the
// honest grammar of the public page).
func pluralRu(count int, one, few, many string) string {
	switch {
	case count%10 == 1 && count%100 != 11:
		return one
	case count%10 >= 2 && count%10 <= 4 && (count%100 < 10 || count%100 >= 20):
		return few
	default:
		return many
	}
}
