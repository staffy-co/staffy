package httptransport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	controlAPI "github.com/staffy-co/staffy/internal/control/api"
	controlDomain "github.com/staffy-co/staffy/internal/control/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/correlation"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 3.2 step-03 transport: the operator surface of the scoped
// controls module — four machine paths and the «Аварийное ограничение
// работы» page. The gate ladder lives in the app service and runs FIRST:
// a non-member of the tenant is the honest 404 (before any control
// material), a member without the operations scope is the 403 with the
// scope words BEFORE any material; the mutations add the step-up gate
// (ErrStepUpRequired — the ChangeMember identity precedent) and the
// mandatory reason. The dictionary refusals (RuleError) answer the honest
// 400 family with the rule words; the optimistic generation conflict
// answers 409 — the machine client re-reads the list, the PAGE renders
// the current material for the repeat confirmation (AC-11). The decode
// is strict: one JSON object, known fields only, Content-Type
// application/json. The dangerous modes (paused, human_only) carry the
// explicit checkbox of the UI discipline; the server-side protection is
// step-up + reason — the double discipline is documented honestly.
// ---------------------------------------------------------------------------

// controlsScope extracts the principal and the tenant path value of the
// control surfaces. An unauthenticated request is the 401 of the resolver;
// an unparseable tenant id is the honest 404 (the action-catalog
// precedent).
func (s *Server) controlsScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return actor, "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.controlsProblem(w, r, controlDomain.ErrNotFound)
		return actor, "", false
	}
	return actor, tenant, true
}

// controlsProblem maps the control errors onto the problem contract: the
// module's ErrNotFound is the plain 404 (no disclosure that the surface
// exists); the module's ErrForbidden is the 403 whose message IS the scope
// words the app attached at the wrap point (one source — the transport
// never invents its own copy, the sentinel is sliced off); the identity
// step-up sentinel is the 403 with the step-up words (the membership
// precedent); the generation conflict is the honest 409 with the
// repeat-confirmation words (the stale-version precedent of the cases
// commitment surface); the dictionary RuleError is the 400 family with
// the rule and the reason (the entitlement precedent — the field errors
// carry the rule name); anything else is the honest 500 with the
// server-side log.
func (s *Server) controlsProblem(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "Не удалось выполнить действие. Повторите позднее."
	var fields map[string]string
	var rule controlDomain.RuleError
	if errors.As(err, &rule) {
		message = rule.Reason
		fields = map[string]string{rule.Rule: rule.Reason}
	}
	switch {
	case errors.Is(err, controlDomain.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "Ресурс не найден."
	case errors.Is(err, controlDomain.ErrForbidden):
		status, code = http.StatusForbidden, "controls_forbidden"
		message = err.Error()
		if prefix := controlDomain.ErrForbidden.Error() + ": "; strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
		}
	case errors.Is(err, identityDomain.ErrStepUpRequired):
		status, code, message = http.StatusForbidden, "step_up_required", "Для действия требуется повторное подтверждение личности."
	case errors.Is(err, controlDomain.ErrGenerationConflict):
		status, code, message = http.StatusConflict, "generation_conflict", "Контрольная точка изменилась другим оператором: перечитайте актуальное состояние и подтвердите изменение повторно."
	case errors.As(err, &rule):
		status, code = http.StatusBadRequest, "control_rule"
	}
	if status >= 500 {
		s.logger.ErrorContext(r.Context(), "scoped controls request failed", "code", "controls_failed", "error", err.Error())
	}
	s.problem(w, r, status, code, message, status >= 500, fields)
}

// BindControls registers the scoped control surface: the four machine
// paths (list, effective, set, resume) and the operator page with its
// forms. The service is the narrow api.ControlService (the
// HandoffService/ActionCatalog type-assert precedent — the app Service
// carries the compile-time proof).
func (s *Server) BindControls(service controlAPI.ControlService) {
	s.controls = service
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/controls", s.controlsListAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/controls/effective", s.controlsEffectiveAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/controls", s.controlsSetAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/controls/resume", s.controlsResumeAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/controls", s.controlsPage)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/controls", s.controlsSetForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/controls/resume", s.controlsResumeForm)
}

// controlsListAPI answers the machine list: every control point of the
// tenant, both states — the removed points stay readable (the history
// surface). The honest empty list is the 200 with the empty array (never
// null).
func (s *Server) controlsListAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.controlsScope(w, r)
	if !ok {
		return
	}
	if s.controls == nil {
		s.controlsProblem(w, r, controlDomain.ErrNotFound)
		return
	}
	controls, err := s.controls.List(r.Context(), actor, tenant)
	if err != nil {
		s.controlsProblem(w, r, err)
		return
	}
	views := make([]controlAPI.ScopedControlView, 0, len(controls))
	for _, control := range controls {
		views = append(views, controlAPI.FromScopedControl(control))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"controls": views})
}

// controlsEffectiveAPI answers the machine intersection computation: the
// repeated scope query parameter carries the route keys (kind:ref pairs);
// the service validates every key with the dictionary discipline (an
// out-of-dictionary kind or a mis-shaped ref is the honest 400, not a
// silent empty match) and answers the most-restrictive intersection with
// the provenance of every influencing control (AC-3).
func (s *Server) controlsEffectiveAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.controlsScope(w, r)
	if !ok {
		return
	}
	if s.controls == nil {
		s.controlsProblem(w, r, controlDomain.ErrNotFound)
		return
	}
	keys, err := parseScopeQuery(r)
	if err != nil {
		s.controlsProblem(w, r, err)
		return
	}
	effective, err := s.controls.EffectiveFor(r.Context(), actor, tenant, keys)
	if err != nil {
		s.controlsProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"effective": controlAPI.FromEffectiveControl(effective)})
}

// parseScopeQuery parses the repeated scope query parameter into the
// route scope keys. The pair shape is kind:ref (the first colon splits);
// the dictionary discipline of every key belongs to the service — the
// transport refuses only the shapeless values (no colon at all), so the
// refusal words have ONE source: the domain guard.
func parseScopeQuery(r *http.Request) ([]controlDomain.ScopeKey, error) {
	values := r.URL.Query()["scope"]
	if len(values) == 0 {
		return nil, controlDomain.RuleError{Rule: "scope", Reason: "укажите хотя бы один скоуп-ключ маршрута: повторяемый параметр scope вида kind:ref"}
	}
	keys := make([]controlDomain.ScopeKey, 0, len(values))
	for _, value := range values {
		kind, ref, found := strings.Cut(value, ":")
		if !found {
			return nil, controlDomain.RuleError{Rule: "scope", Reason: "скоуп-ключ обязан иметь вид kind:ref, получено: " + value}
		}
		keys = append(keys, controlDomain.ScopeKey{Kind: kind, Ref: ref})
	}
	return keys, nil
}

// controlsSetInput is the strict machine shape of the set mutation. The
// window fields are RFC3339 timestamps; nil means unbounded on that side.
// Empty strings travel to the service — the dictionary guard refuses
// them with the honest words (one source of the refusal text).
type controlsSetInput struct {
	ScopeKind          string     `json:"scope_kind"`
	ScopeRef           string     `json:"scope_ref"`
	Mode               string     `json:"mode"`
	Reason             string     `json:"reason"`
	ExpectedGeneration int64      `json:"expected_generation"`
	ScheduledStartAt   *time.Time `json:"scheduled_start_at"`
	ExpiresAt          *time.Time `json:"expires_at"`
}

// controlsResumeInput is the strict machine shape of the resume mutation.
type controlsResumeInput struct {
	ScopeKind          string `json:"scope_kind"`
	ScopeRef           string `json:"scope_ref"`
	Reason             string `json:"reason"`
	ExpectedGeneration int64  `json:"expected_generation"`
}

// decodeControlsJSON strictly decodes one JSON object: the exact
// Content-Type, the known fields only, nothing after the object (the
// membership decodeMember precedent, spoken through the control
// RuleError so the 400 words have one shape).
func decodeControlsJSON(w http.ResponseWriter, r *http.Request, out any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return controlDomain.RuleError{Rule: "content_type", Reason: "Используйте Content-Type application/json."}
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return controlDomain.RuleError{Rule: "body", Reason: "Проверьте JSON-запрос: один объект с известными полями."}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return controlDomain.RuleError{Rule: "body", Reason: "Ожидается ровно один JSON-объект без данных после него."}
	}
	return nil
}

// controlsSetAPI answers the machine set mutation. The gates run in the
// service (membership-404 → operations-403 words → step-up → guard →
// connector resolution); a generation mismatch answers the honest 409.
func (s *Server) controlsSetAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.controlsScope(w, r)
	if !ok {
		return
	}
	if s.controls == nil {
		s.controlsProblem(w, r, controlDomain.ErrNotFound)
		return
	}
	var input controlsSetInput
	if err := decodeControlsJSON(w, r, &input); err != nil {
		s.controlsProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	control, err := s.controls.Set(r.Context(), actor, tenant, controlAPI.SetRequest{
		Kind:               input.ScopeKind,
		Ref:                input.ScopeRef,
		Mode:               input.Mode,
		Reason:             input.Reason,
		ExpectedGeneration: input.ExpectedGeneration,
		ScheduledStartAt:   input.ScheduledStartAt,
		ExpiresAt:          input.ExpiresAt,
		CorrelationID:      corr,
	})
	if err != nil {
		s.controlsProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"control": controlAPI.FromScopedControl(control)})
}

// controlsResumeAPI answers the machine resume (lift) mutation: the state
// becomes removed, the generation grows, the history stays. A missing
// point is the honest 404; lifting an already-removed point is the
// dictionary 400.
func (s *Server) controlsResumeAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.controlsScope(w, r)
	if !ok {
		return
	}
	if s.controls == nil {
		s.controlsProblem(w, r, controlDomain.ErrNotFound)
		return
	}
	var input controlsResumeInput
	if err := decodeControlsJSON(w, r, &input); err != nil {
		s.controlsProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	control, err := s.controls.Resume(r.Context(), actor, tenant, controlAPI.ResumeRequest{
		Kind:               input.ScopeKind,
		Ref:                input.ScopeRef,
		Reason:             input.Reason,
		ExpectedGeneration: input.ExpectedGeneration,
		CorrelationID:      corr,
	})
	if err != nil {
		s.controlsProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"control": controlAPI.FromScopedControl(control)})
}

// controlRedirectTarget answers the PRG target of a successful control
// form mutation: the page with the applied/resumed flag carrying the new
// generation — the notice speaks the words, the focus moves onto it (the
// release page precedent).
func controlRedirectTarget(tenant id.UUID, flag string, generation int64) string {
	return "/app/tenants/" + tenant.String() + "/controls?" + flag + "=" + strconv.FormatInt(generation, 10)
}

// controlsFormOriginRefused is the honest form-origin refusal of the
// control forms: the wrap keeps the module sentinel so the mapping stays
// one-shaped, and the words explain the browser-only discipline (the
// membership validFormOrigin precedent).
func controlsFormOriginRefused() error {
	return fmt.Errorf("%w: %s", controlDomain.ErrForbidden, "форма изменения контрольных точек доступна только из браузера приложения (origin).")
}

// ---------------------------------------------------------------------------
// The «Аварийное ограничение работы» page
// ---------------------------------------------------------------------------

// controlModeDescriptions speaks every mode word WITH the honest
// description of the allowed/forbidden behaviour (AC-2: the word alone is
// never enough). The descriptions are presentation words of the page —
// the domain keeps the words; the polarity note keeps the control modes
// visually and textually apart from the conversation modes 2-5 (the
// Implementation Notes discipline).
var controlModeDescriptions = map[string]string{
	controlDomain.ControlModeAccept:     "маршрут не ограничен: контроль на этом объекте не останавливает работу",
	controlDomain.ControlModeIntakeOnly: "приём данных без новых Обещаний: события принимаются, новые обязательства НЕ создаются",
	controlDomain.ControlModeHumanOnly:  "только человек: автономные действия остановлены, маршрутизация человеку",
	controlDomain.ControlModePaused:     "полная остановка маршрута: и приём событий, и автономные действия остановлены",
}

// controlModeOptions is the form radio projection: the ladder order of
// the domain dictionary with the description beside every word, the
// dangerous words marked (AC-12 — the explicit checkbox discipline).
type controlModeOption struct {
	Word        string
	Description string
	Dangerous   bool
}

// newControlModeOptions builds the options from the DOMAIN dictionary
// (the ladder order is normative — no transport-side mirror to drift).
func newControlModeOptions() []controlModeOption {
	options := make([]controlModeOption, 0, len(controlDomain.ControlModes))
	for _, word := range controlDomain.ControlModes {
		options = append(options, controlModeOption{
			Word:        word,
			Description: controlModeDescriptions[word],
			Dangerous:   word == controlDomain.ControlModePaused || word == controlDomain.ControlModeHumanOnly,
		})
	}
	return options
}

// controlCardView is the card projection of one control point: the mode
// spoken as a word WITH the description (AC-2), the owner, the time of
// the last change, the generation, the reason, the read-time window and
// the state — every word textual, the corrupted marker honest (the value
// reads paused, the damage is named).
type controlCardView struct {
	ScopeKind       string
	ScopeRef        string
	Mode            string
	ModeDescription string
	Corrupted       bool
	Reason          string
	ActorID         string
	PrevMode        string
	Generation      int64
	State           string
	StateText       string
	WindowText      string
	UpdatedText     string
}

// newControlCardView maps one domain record onto the card. The mode is
// the SAFE read (corrupted values read paused — fail-closed, never
// expanding the allowance); the state words stay honest (removed is
// history, not absence).
func newControlCardView(c controlDomain.ScopedControl) controlCardView {
	card := controlCardView{
		ScopeKind:       c.ScopeKind,
		ScopeRef:        c.ScopeRef,
		Mode:            controlDomain.SafeReadMode(c.Mode),
		ModeDescription: controlModeDescriptions[controlDomain.SafeReadMode(c.Mode)],
		Corrupted:       !controlDomain.WordIn(controlDomain.ControlModes, c.Mode),
		Reason:          c.Reason,
		ActorID:         c.ActorID,
		PrevMode:        controlDomain.SafeReadMode(c.PrevMode),
		Generation:      c.Generation,
		State:           c.State,
		WindowText:      formatControlWindow(c.ScheduledStartAt, c.ExpiresAt),
		UpdatedText:     c.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if c.State == controlDomain.ControlStateActive {
		card.StateText = "действует"
	} else {
		card.StateText = "снято — история и аудит сохранены"
	}
	return card
}

// formatControlWindow renders the read-time window [start, expires) in
// honest words: an unbounded side says so, never a fabricated bound.
func formatControlWindow(start, expires *time.Time) string {
	startText, expiresText := "без ограничения", "бессрочно"
	if start != nil {
		startText = start.UTC().Format(time.RFC3339)
	}
	if expires != nil {
		expiresText = expires.UTC().Format(time.RFC3339)
	}
	return "с " + startText + " до " + expiresText
}

// controlConflictView is the AC-11 material of a lost optimistic race:
// the stale expectation, the CURRENT state of the point, and the form
// refilled with the current generation — the operator re-confirms over
// the actual truth, nothing is silently overwritten.
type controlConflictView struct {
	ExpectedGeneration int64
	Current            controlCardView
}

// controlCalcView is the calculator answer: the intersection mode with
// the description, the provenance of every influencing control, the
// winning reason, the crossing words and the honest corruption marker.
type controlCalcView struct {
	Mode             string
	ModeDescription  string
	Reason           string
	IntersectionText string
	Corrupted        bool
	Influencing      []controlCardView
	Unrestricted     bool
}

// controlFormState carries the form values for the honest re-render: the
// submitted values travel back into the form (the operator corrects one
// field, not the whole form).
type controlFormState struct {
	Kind               string
	Ref                string
	Mode               string
	Reason             string
	ScheduledStart     string
	Expires            string
	ExpectedGeneration string
	ConfirmDangerous   bool
}

// controlsPageView carries the page material: the tenant, the cards (the
// history surface included), the honest empty state, the PRG notice, the
// form, the conflict material and the calculator answer.
type controlsPageView struct {
	TenantID    string
	StepUp      bool
	HasCards    bool
	Cards       []controlCardView
	EmptyText   string
	Error       string
	Notice      string
	Conflict    *controlConflictView
	Form        controlFormState
	ModeOptions []controlModeOption
	KindOptions []string
	CalcKeys    string
	HasCalc     bool
	Calc        *controlCalcView
}

// controlsEmptyText is the honest reason of the empty page (the spec
// words): the absence of controls MEANS the routes are unrestricted — a
// readable state, never a bare stub.
const controlsEmptyText = "Активных контрольных точек нет — маршруты не ограничены. Контроль появится, когда оператор с полномочием operations установит первую точку; снятые точки остаются в истории ниже."

// controlsNotice speaks the PRG status of a successful mutation: the
// generation of the new state, the words of what happened.
func controlsNotice(flag, raw string) string {
	generation, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return ""
	}
	if flag == "applied" {
		return "Контрольная точка применена: поколение " + strconv.FormatInt(generation, 10) + ". Фокус переведён на этот статус."
	}
	return "Контрольная точка снята: поколение " + strconv.FormatInt(generation, 10) + " — история и аудит сохранены, маршрут больше не ограничен этой точкой."
}

// controlsPage renders the operator page: the cards of every control
// point (the mode word with the description, the owner, the generation,
// the reason, the window), the set form (the mode descriptions beside
// the radios, the explicit dangerous-mode checkbox, the expected
// generation), the calculator and the honest empty state.
func (s *Server) controlsPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.controlsScope(w, r)
	if !ok {
		return
	}
	if s.controls == nil {
		s.controlsProblem(w, r, controlDomain.ErrNotFound)
		return
	}
	data, status, failed := s.controlsPageData(w, r, actor, tenant)
	if failed {
		return
	}
	s.renderStatus(w, status, "controls", data)
}

// controlsPageData assembles the page view: the list, the PRG notice and
// the calculator answer. The 403/404 answers travel as problems (the
// page cannot load its material anyway); the calculator dictionary
// refusals stay on the page with the honest 400 status.
func (s *Server) controlsPageData(w http.ResponseWriter, r *http.Request, actor identityDomain.Principal, tenant id.UUID) (controlsPageView, int, bool) {
	data := controlsPageView{
		TenantID:    tenant.String(),
		StepUp:      actor.StepUpVerified,
		ModeOptions: newControlModeOptions(),
		KindOptions: controlDomain.ScopeKinds,
	}
	status := http.StatusOK
	controls, err := s.controls.List(r.Context(), actor, tenant)
	if err != nil {
		// The page cannot load its material: the honest problem answer (the
		// 403 with the scope words, the 404 of a non-member, the loud 500).
		s.controlsProblem(w, r, err)
		return data, 0, true
	}
	if len(controls) > 0 {
		data.HasCards = true
		for _, control := range controls {
			data.Cards = append(data.Cards, newControlCardView(control))
		}
	} else {
		data.EmptyText = controlsEmptyText
	}
	for _, flag := range []string{"applied", "resumed"} {
		if raw := r.URL.Query().Get(flag); raw != "" {
			data.Notice = controlsNotice(flag, raw)
		}
	}
	if rawKeys := r.URL.Query().Get("keys"); rawKeys != "" {
		data.CalcKeys = rawKeys
		keys, parseErr := parseScopeKeysText(rawKeys)
		if parseErr != nil {
			var parseRule controlDomain.RuleError
			if errors.As(parseErr, &parseRule) {
				data.Error = parseRule.Reason
			} else {
				data.Error = parseErr.Error()
			}
			status = http.StatusBadRequest
		} else {
			effective, effectiveErr := s.controls.EffectiveFor(r.Context(), actor, tenant, keys)
			if effectiveErr != nil {
				var rule controlDomain.RuleError
				if errors.As(effectiveErr, &rule) {
					// The dictionary refusal of a key stays on the page: the honest
					// 400 with the rule words.
					data.Error = rule.Reason
					status = http.StatusBadRequest
				} else {
					s.controlsProblem(w, r, effectiveErr)
					return data, 0, true
				}
			} else {
				data.HasCalc = true
				data.Calc = newControlCalcView(effective)
			}
		}
	}
	return data, status, false
}

// parseScopeKeysText parses the calculator textarea: one kind:ref pair
// per line, blank lines skipped. The lexical refusal stays here; the
// dictionary discipline of every key belongs to the service.
func parseScopeKeysText(text string) ([]controlDomain.ScopeKey, error) {
	keys := make([]controlDomain.ScopeKey, 0, 8)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kind, ref, found := strings.Cut(line, ":")
		if !found {
			return nil, controlDomain.RuleError{Rule: "scope", Reason: "скоуп-ключ обязан иметь вид kind:ref, получено: " + line}
		}
		keys = append(keys, controlDomain.ScopeKey{Kind: strings.TrimSpace(kind), Ref: strings.TrimSpace(ref)})
	}
	if len(keys) == 0 {
		return nil, controlDomain.RuleError{Rule: "scope", Reason: "введите хотя бы один скоуп-ключ маршрута вида kind:ref"}
	}
	return keys, nil
}

// newControlCalcView maps the domain computation onto the calculator
// projection: the unrestricted answer stays honest (absent controls mean
// accept — the route is not restricted), the provenance lists every
// influencing control, the crossing words join in the ladder order.
func newControlCalcView(e controlDomain.EffectiveControl) *controlCalcView {
	view := &controlCalcView{
		Mode:             e.Mode,
		ModeDescription:  controlModeDescriptions[e.Mode],
		Reason:           e.Reason,
		IntersectionText: strings.Join(e.Intersection, "; "),
		Corrupted:        e.Corrupted,
	}
	for _, control := range e.Influencing {
		view.Influencing = append(view.Influencing, newControlCardView(control))
	}
	view.Unrestricted = len(e.Influencing) == 0
	return view
}

// controlsSetForm answers the page set form. The ladder: the form origin,
// the parse, the dangerous-mode checkbox (the UI discipline of AC-12 —
// the server-side protection is step-up + reason, the double
// discipline), then the service mutation. A success answers PRG (303 to
// the page with the applied flag — the focus moves onto the notice); a
// conflict re-renders the page with the CURRENT material and the form
// refilled with the current generation (AC-11).
func (s *Server) controlsSetForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.controlsScope(w, r)
	if !ok {
		return
	}
	if s.controls == nil {
		s.controlsProblem(w, r, controlDomain.ErrNotFound)
		return
	}
	if !validFormOrigin(r) {
		s.controlsProblem(w, r, controlsFormOriginRefused())
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.controlsFormResult(w, r, actor, tenant, controlDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."}, controlFormState{})
		return
	}
	form := controlFormState{
		Kind:               r.FormValue("scope_kind"),
		Ref:                r.FormValue("scope_ref"),
		Mode:               r.FormValue("mode"),
		Reason:             r.FormValue("reason"),
		ScheduledStart:     r.FormValue("scheduled_start_at"),
		Expires:            r.FormValue("expires_at"),
		ExpectedGeneration: r.FormValue("expected_generation"),
		ConfirmDangerous:   r.FormValue("confirm_dangerous") == "yes",
	}
	generation, err := strconv.ParseInt(strings.TrimSpace(form.ExpectedGeneration), 10, 64)
	if err != nil {
		s.controlsFormResult(w, r, actor, tenant, controlDomain.RuleError{Rule: "expected_generation", Reason: "Ожидаемое поколение — целое число (0 для первой установки)."}, form)
		return
	}
	scheduled, expires, err := parseControlWindowForm(form.ScheduledStart, form.Expires)
	if err != nil {
		s.controlsFormResult(w, r, actor, tenant, err, form)
		return
	}
	if (form.Mode == controlDomain.ControlModePaused || form.Mode == controlDomain.ControlModeHumanOnly) && r.FormValue("confirm_dangerous") != "yes" {
		// The UI discipline of AC-12: the dangerous modes need the explicit
		// checkbox; the server-side protection stays step-up + reason (the
		// double discipline, honestly documented — the machine API carries
		// no checkbox field).
		s.controlsFormResult(w, r, actor, tenant, controlDomain.RuleError{Rule: "confirm_dangerous", Reason: "Опасный режим требует явного подтверждения: отметьте чекбокс согласия."}, form)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	control, err := s.controls.Set(r.Context(), actor, tenant, controlAPI.SetRequest{
		Kind:               form.Kind,
		Ref:                form.Ref,
		Mode:               form.Mode,
		Reason:             form.Reason,
		ExpectedGeneration: generation,
		ScheduledStartAt:   scheduled,
		ExpiresAt:          expires,
		CorrelationID:      corr,
	})
	if err != nil {
		s.controlsFormResult(w, r, actor, tenant, err, form)
		return
	}
	http.Redirect(w, r, controlRedirectTarget(tenant, "applied", control.Generation), http.StatusSeeOther)
}

// controlsResumeForm answers the page resume form (behind the disclosure
// of every card): the lift needs the reason, the expected generation and
// the explicit confirmation checkbox (a lift re-opens the route — an
// equally deliberate act).
func (s *Server) controlsResumeForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.controlsScope(w, r)
	if !ok {
		return
	}
	if s.controls == nil {
		s.controlsProblem(w, r, controlDomain.ErrNotFound)
		return
	}
	if !validFormOrigin(r) {
		s.controlsProblem(w, r, controlsFormOriginRefused())
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.controlsFormResult(w, r, actor, tenant, controlDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."}, controlFormState{})
		return
	}
	form := controlFormState{
		Kind:               r.FormValue("scope_kind"),
		Ref:                r.FormValue("scope_ref"),
		Reason:             r.FormValue("reason"),
		ExpectedGeneration: r.FormValue("expected_generation"),
	}
	generation, err := strconv.ParseInt(strings.TrimSpace(form.ExpectedGeneration), 10, 64)
	if err != nil {
		s.controlsFormResult(w, r, actor, tenant, controlDomain.RuleError{Rule: "expected_generation", Reason: "Ожидаемое поколение — целое число."}, form)
		return
	}
	if r.FormValue("confirm_resume") != "yes" {
		s.controlsFormResult(w, r, actor, tenant, controlDomain.RuleError{Rule: "confirm_resume", Reason: "Возобновление требует явного подтверждения: отметьте чекбокс согласия."}, form)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	control, err := s.controls.Resume(r.Context(), actor, tenant, controlAPI.ResumeRequest{
		Kind:               form.Kind,
		Ref:                form.Ref,
		Reason:             form.Reason,
		ExpectedGeneration: generation,
		CorrelationID:      corr,
	})
	if err != nil {
		s.controlsFormResult(w, r, actor, tenant, err, form)
		return
	}
	http.Redirect(w, r, controlRedirectTarget(tenant, "resumed", control.Generation), http.StatusSeeOther)
}

// parseControlWindowForm parses the two window fields: an empty field is
// unbounded (nil); a filled field must be RFC3339 — a malformed
// timestamp is the honest 400 words, never a silently dropped bound.
func parseControlWindowForm(start, expires string) (*time.Time, *time.Time, error) {
	parsedStart, err := parseControlTimestamp("scheduled_start_at", start)
	if err != nil {
		return nil, nil, err
	}
	parsedExpires, err := parseControlTimestamp("expires_at", expires)
	if err != nil {
		return nil, nil, err
	}
	return parsedStart, parsedExpires, nil
}

// parseControlTimestamp parses one RFC3339 window bound (empty =
// unbounded nil).
func parseControlTimestamp(field, raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, controlDomain.RuleError{Rule: field, Reason: "Граница окна — время RFC3339, например 2026-10-19T12:00:00Z."}
	}
	return &parsed, nil
}

// controlsFormResult re-renders the page after a failed form mutation:
// the list is re-read (a 403/404 there answers the plain problem — the
// page material does not exist for this actor), the error words sit in
// the alert, the conflict material renders the CURRENT state with the
// form refilled over the current generation (AC-11), and the submitted
// values travel back into the form (the operator corrects one field,
// not the whole form).
func (s *Server) controlsFormResult(w http.ResponseWriter, r *http.Request, actor identityDomain.Principal, tenant id.UUID, err error, form controlFormState) {
	data, status, failed := s.controlsPageData(w, r, actor, tenant)
	if failed {
		return
	}
	data.Form = form
	var conflict controlDomain.ConflictError
	if errors.As(err, &conflict) {
		// The repeat confirmation of AC-11: the operator sees the actual
		// state and re-submits over the CURRENT generation.
		refill := form
		refill.ExpectedGeneration = strconv.FormatInt(conflict.Current.Generation, 10)
		data.Form = refill
		data.Conflict = &controlConflictView{
			ExpectedGeneration: conflict.Expected,
			Current:            newControlCardView(conflict.Current),
		}
	}
	errorStatus, errorText := controlFormError(err)
	if errorText != "" {
		data.Error = errorText
	}
	if status == http.StatusOK {
		status = errorStatus
	}
	s.renderStatus(w, status, "controls", data)
}

// controlFormError maps a form mutation error onto the page status and
// the honest words (the membership memberError precedent): step-up keeps
// its 403 with the confirmation words; the conflict is the 409 with the
// repeat-confirmation words; the dictionary refusals are the 400 family;
// the missing point of a resume is the honest 404.
func controlFormError(err error) (int, string) {
	switch {
	case errors.Is(err, identityDomain.ErrStepUpRequired):
		return http.StatusForbidden, "Для изменения контрольной точки требуется повторное подтверждение личности. Пока провайдер недоступен, изменение заблокировано."
	case errors.Is(err, controlDomain.ErrGenerationConflict):
		return http.StatusConflict, "Поколение изменилось: ниже актуальное состояние точки — проверьте его и подтвердите изменение повторно."
	case errors.Is(err, controlDomain.ErrNotFound):
		return http.StatusNotFound, "Контрольная точка не найдена: перечитайте список и повторите."
	case errors.Is(err, controlDomain.ErrForbidden):
		return http.StatusForbidden, "Недостаточно полномочий."
	}
	var rule controlDomain.RuleError
	if errors.As(err, &rule) {
		return http.StatusBadRequest, rule.Reason
	}
	return http.StatusInternalServerError, "Не удалось изменить контрольную точку. Повторите позднее."
}
