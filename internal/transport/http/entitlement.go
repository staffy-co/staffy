package httptransport

import (
        "encoding/json"
        "errors"
        "io"
        "mime"
        "net/http"
        "strconv"
        "time"

        "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/correlation"
        "github.com/artemiosu/staffy/internal/platform/id"
        entitlementAPI "github.com/artemiosu/staffy/internal/entitlement/api"
        entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
)

func (s *Server) entitlementService() entitlementAPI.Service { return s.entitlement }

func (s *Server) entitlementScope(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, bool) {
        actor, ok := s.principal(w, r)
        if !ok {
                return actor, "", false
        }
        tenant, err := id.Parse(r.PathValue("tenant_id"))
        if err != nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return actor, "", false
        }
        return actor, tenant, true
}

func (s *Server) entitlementProblem(w http.ResponseWriter, r *http.Request, err error) {
        status, code, message := 500, "internal_error", "Не удалось выполнить действие. Повторите позднее."
        var fields map[string]string
        var rule entitlementDomain.RuleError
        if errors.As(err, &rule) {
                message = rule.Error()
                fields = map[string]string{rule.Rule: rule.Reason}
        }
        switch {
        case errors.Is(err, entitlementDomain.ErrNotFound), errors.Is(err, entitlementDomain.ErrForbidden):
                status, code, message = 404, "not_found", "Ресурс не найден."
        case errors.As(err, &rule):
                status, code = 400, "entitlement_rule"
        case errors.Is(err, entitlementDomain.ErrInvalidEventKey):
                status, code, message = 400, "invalid_event_key", "Ключ события задан неверно (8–200 символов)."
        case errors.Is(err, entitlementDomain.ErrEventKeyConflict):
                status, code, message = 409, "event_key_conflict", "Этот ключ уже использован для другого события: конфликт отправлен на проверку, перезапись исключена."
        }
        s.problem(w, r, status, code, message, status >= 500, fields)
}

// BindEntitlement mounts the commercial perimeter surface: the JSON API and
// the keyboard-accessible entitlement and Budget Meter pages.
func (s *Server) BindEntitlement(service entitlementAPI.Service) {
        s.entitlement = service
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/entitlement", s.entitlementViewAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/entitlement/terms", s.composeTermsAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/entitlement/state", s.stateTransitionAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/entitlement/budgets", s.setBudgetAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/entitlement/sources", s.registerSourceAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/usage-events", s.ingestUsageAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/budget-meter", s.meterViewAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/admission-state", s.admissionStateAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/usage-exceptions", s.exceptionsViewAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/usage-exceptions", s.raiseExceptionAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/usage-exceptions/{exception_id}/acknowledge", s.ackExceptionAPI)
        s.mux.HandleFunc("GET /app/tenants/{tenant_id}/entitlement", s.entitlementPage)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/entitlement/terms", s.termsForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/entitlement/state", s.stateForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/entitlement/budget", s.budgetForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/entitlement/sources", s.sourceForm)
        s.mux.HandleFunc("GET /app/tenants/{tenant_id}/budget", s.budgetPage)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/budget/ingest", s.ingestForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/budget/exception", s.exceptionForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/budget/ack", s.ackForm)
}

func (s *Server) entitlementViewAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        view, err := service.EntitlementView(r.Context(), actor, tenant)
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(view)
}

func (s *Server) meterViewAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        view, err := service.MeterView(r.Context(), actor, tenant)
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(view)
}

func (s *Server) admissionStateAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        view, err := service.AdmissionView(r.Context(), actor, tenant)
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(view)
}

func (s *Server) exceptionsViewAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        view, err := service.ExceptionsView(r.Context(), actor, tenant)
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(view)
}

func (s *Server) composeTermsAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        var input termsInput
        if !decodeEntitlementJSON(w, r, &input, s.entitlementProblem) {
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        terms, err := service.ComposeTerms(r.Context(), input.command(actor, tenant, corr))
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(terms)
}

type termsInput struct {
        Currency           string                                `json:"currency"`
        TaxRegime          string                                `json:"tax_regime"`
        PeriodStart        *time.Time                            `json:"period_start"`
        PeriodEnd          *time.Time                            `json:"period_end"`
        IncludedEmployees  []string                              `json:"included_employees"`
        IncludedChannels   []string                              `json:"included_channels"`
        IncludedConnectors []string                              `json:"included_connectors"`
        Limits             []entitlementDomain.CategoryLimit     `json:"limits"`
        LaunchPrice        *float64                              `json:"launch_price"`
        SubscriptionPrice  *float64                              `json:"subscription_price"`
        ChangeNote         string                                `json:"change_note"`
}

func (in termsInput) command(actor domain.Principal, tenant id.UUID, corr id.UUID) entitlementAPI.TermsCommand {
        return entitlementAPI.TermsCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Currency: in.Currency, TaxRegime: in.TaxRegime,
                PeriodStart: in.PeriodStart, PeriodEnd: in.PeriodEnd,
                IncludedEmployees: in.IncludedEmployees, IncludedChannels: in.IncludedChannels, IncludedConnectors: in.IncludedConnectors,
                Limits: in.Limits, LaunchPrice: in.LaunchPrice, SubscriptionPrice: in.SubscriptionPrice,
                ChangeNote: in.ChangeNote,
        }
}

func (s *Server) stateTransitionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        var input struct {
                Action string `json:"action"`
                Basis  string `json:"basis"`
                Confirm bool  `json:"confirm"`
        }
        if !decodeEntitlementJSON(w, r, &input, s.entitlementProblem) {
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        event, err := service.RunStateTransition(r.Context(), entitlementAPI.StateCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Action: input.Action, Basis: input.Basis, Confirm: input.Confirm,
        })
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(event)
}

func (s *Server) setBudgetAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        var input struct {
                Amount            float64                                   `json:"amount"`
                Currency          string                                    `json:"currency"`
                PeriodStart       time.Time                                 `json:"period_start"`
                PeriodEnd         time.Time                                 `json:"period_end"`
                WarningPercent    int                                       `json:"warning_threshold_percent"`
                HardLimit         *float64                                  `json:"hard_limit"`
                LimitPolicy       string                                    `json:"limit_policy"`
                AnomalyThresholds []entitlementDomain.AnomalyThreshold      `json:"anomaly_thresholds"`
                Note              string                                    `json:"note"`
        }
        if !decodeEntitlementJSON(w, r, &input, s.entitlementProblem) {
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        budget, err := service.SetBudget(r.Context(), entitlementAPI.BudgetCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Amount: input.Amount, Currency: input.Currency,
                PeriodStart: input.PeriodStart, PeriodEnd: input.PeriodEnd,
                WarningPercent: input.WarningPercent, HardLimit: input.HardLimit, LimitPolicy: input.LimitPolicy,
                AnomalyThresholds: input.AnomalyThresholds, Note: input.Note,
        })
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(budget)
}

func (s *Server) registerSourceAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        var input struct {
                Name             string   `json:"name"`
                Categories       []string `json:"categories"`
                FreshnessMinutes int      `json:"freshness_minutes"`
                Retire           bool     `json:"retire"`
                Note             string   `json:"note"`
        }
        if !decodeEntitlementJSON(w, r, &input, s.entitlementProblem) {
                return
        }
        categories := make([]entitlementDomain.UsageCategory, 0, len(input.Categories))
        for _, category := range input.Categories {
                categories = append(categories, entitlementDomain.UsageCategory(category))
        }
        corr, _ := correlation.FromContext(r.Context())
        source, err := service.RegisterSource(r.Context(), entitlementAPI.SourceCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Name: input.Name, Categories: categories, FreshnessMinutes: input.FreshnessMinutes,
                Retire: input.Retire, Note: input.Note,
        })
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(source)
}

func (s *Server) ingestUsageAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        var input struct {
                EventKey       string    `json:"event_key"`
                Source         string    `json:"source"`
                Category       string    `json:"category"`
                Quantity       float64   `json:"quantity"`
                Amount         *float64  `json:"amount"`
                Currency       string    `json:"currency"`
                Estimate       bool      `json:"is_estimate"`
                PeriodStart    time.Time `json:"period_start"`
                PeriodEnd      time.Time `json:"period_end"`
                OccurredAt     time.Time `json:"occurred_at"`
                EmployeeID     string    `json:"employee_id"`
                Channel        string    `json:"channel"`
                ReleaseVersion string    `json:"release_version"`
                Provenance     string    `json:"provenance"`
        }
        if !decodeEntitlementJSON(w, r, &input, s.entitlementProblem) {
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        event, replayed, err := service.IngestUsage(r.Context(), entitlementAPI.UsageEventCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                EventKey: input.EventKey, Source: input.Source,
                Category: entitlementDomain.UsageCategory(input.Category), Quantity: input.Quantity,
                Amount: input.Amount, Currency: input.Currency, Estimate: input.Estimate,
                PeriodStart: input.PeriodStart, PeriodEnd: input.PeriodEnd, OccurredAt: input.OccurredAt,
                EmployeeID: input.EmployeeID, Channel: input.Channel, ReleaseVersion: input.ReleaseVersion,
                Provenance: input.Provenance,
        })
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        if replayed {
                w.Header().Set("Idempotency-Replayed", "true")
        }
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(event)
}

func (s *Server) raiseExceptionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        var input struct {
                Cause             string `json:"cause"`
                AffectedEmployee  string `json:"affected_employee"`
                AffectedRoute     string `json:"affected_route"`
                RecommendedAction string `json:"recommended_action"`
                Message           string `json:"message"`
        }
        if !decodeEntitlementJSON(w, r, &input, s.entitlementProblem) {
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        exception, err := service.RaiseException(r.Context(), entitlementAPI.ManualExceptionCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Cause: input.Cause, AffectedEmployee: input.AffectedEmployee, AffectedRoute: input.AffectedRoute,
                RecommendedAction: input.RecommendedAction, Message: input.Message,
        })
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(exception)
}

func (s *Server) ackExceptionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        exceptionID, err := id.Parse(r.PathValue("exception_id"))
        if err != nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        var input struct {
                Note string `json:"note"`
        }
        if !decodeEntitlementJSON(w, r, &input, s.entitlementProblem) {
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        if err := service.AcknowledgeException(r.Context(), entitlementAPI.AckCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr, ExceptionID: exceptionID, Note: input.Note,
        }); err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        w.WriteHeader(http.StatusNoContent)
}

// decodeEntitlementJSON enforces one JSON object per request with unknown
// fields rejected (the same boundary as the other modules).
func decodeEntitlementJSON(w http.ResponseWriter, r *http.Request, target any, problem func(http.ResponseWriter, *http.Request, error)) bool {
        media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
        if err != nil || media != "application/json" {
                problem(w, r, entitlementDomain.RuleError{Rule: "content_type", Reason: "Используйте Content-Type application/json."})
                return false
        }
        decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
        decoder.DisallowUnknownFields()
        if err = decoder.Decode(target); err != nil {
                problem(w, r, entitlementDomain.ErrInvalidEventKey)
                return false
        }
        if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
                problem(w, r, entitlementDomain.ErrInvalidEventKey)
                return false
        }
        return true
}

// ---------------------------------------------------------------------------
// Pages
// ---------------------------------------------------------------------------

func (s *Server) entitlementPage(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        view, err := service.EntitlementView(r.Context(), actor, tenant)
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        data := map[string]any{
                "TenantID":       tenant,
                "View":           prepareEntitlementView(view),
                "CategoryOptions": usageCategoryOptions(),
                "FormKey":        s.freshKey("terms"),
        }
        for _, flag := range []struct {
                query  string
                notice string
        }{
                {"terms_saved", "Версия условий тарифа опубликована."},
                {"activated", "Коммерческие условия активированы."},
                {"paused", "Entitlement приостановлен."},
                {"resumed", "Entitlement возобновлён."},
                {"ended", "Entitlement завершён."},
                {"budget_saved", "Бюджет опубликован."},
                {"source_saved", "Источник usage зарегистрирован."},
        } {
                if v := r.URL.Query().Get(flag.query); v == "1" {
                        data["Notice"] = flag.notice
                }
        }
        if message := flashError(r, tenant); message != "" {
                data["Error"] = message
                http.SetCookie(w, clearFlashCookie(tenant))
        }
        s.render(w, "entitlement", data)
}

func (s *Server) termsForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
        if err := r.ParseForm(); err != nil {
                s.entitlementProblem(w, r, entitlementDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        command := entitlementAPI.TermsCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Currency: r.FormValue("currency"), TaxRegime: r.FormValue("tax_regime"),
                IncludedEmployees:  splitLines(r.FormValue("included_employees")),
                IncludedChannels:   splitLines(r.FormValue("included_channels")),
                IncludedConnectors: splitLines(r.FormValue("included_connectors")),
                ChangeNote:         r.FormValue("change_note"),
        }
        if start, err := parseDateForm(r.FormValue("period_start")); err != nil {
                redirectEntitlementError(w, r, tenant, err)
                return
        } else if start != nil {
                command.PeriodStart = start
        }
        if end, err := parseDateForm(r.FormValue("period_end")); err != nil {
                redirectEntitlementError(w, r, tenant, err)
                return
        } else if end != nil {
                command.PeriodEnd = end
        }
        if price := r.FormValue("launch_price"); price != "" {
                if value, err := strconv.ParseFloat(price, 64); err == nil {
                        command.LaunchPrice = &value
                }
        }
        if price := r.FormValue("subscription_price"); price != "" {
                if value, err := strconv.ParseFloat(price, 64); err == nil {
                        command.SubscriptionPrice = &value
                }
        }
        if _, err := service.ComposeTerms(r.Context(), command); err != nil {
                redirectEntitlementError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/entitlement?terms_saved=1", http.StatusSeeOther)
}

func (s *Server) stateForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err := r.ParseForm(); err != nil {
                s.entitlementProblem(w, r, entitlementDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        action := r.FormValue("action")
        // Accessible confirmation of the commercial state change: an explicit
        // checkbox with a text label, not a color cue.
        if r.FormValue("confirm_state") != "yes" {
                redirectEntitlementError(w, r, tenant, entitlementDomain.RuleError{Rule: "confirm", Reason: "Подтвердите изменение коммерческого состояния."})
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        if _, err := service.RunStateTransition(r.Context(), entitlementAPI.StateCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Action: action, Basis: r.FormValue("basis"), Confirm: true,
        }); err != nil {
                redirectEntitlementError(w, r, tenant, err)
                return
        }
        // The notice flag names the transition honestly (activate→activated,
        // end→ended); a naive suffix concatenation would double the letters.
        flags := map[string]string{
                entitlementDomain.EventActivate: "activated",
                entitlementDomain.EventPause:    "paused",
                entitlementDomain.EventResume:   "resumed",
                entitlementDomain.EventEnd:      "ended",
        }
        flag, known := flags[action]
        if !known {
                flag = "paused"
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/entitlement?"+flag+"=1", http.StatusSeeOther)
}

func (s *Server) budgetForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err := r.ParseForm(); err != nil {
                s.entitlementProblem(w, r, entitlementDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        command, err := budgetCommandFromForm(r)
        if err != nil {
                redirectEntitlementError(w, r, tenant, err)
                return
        }
        command.Actor, command.TenantID = actor, tenant
        command.CorrelationID, _ = correlation.FromContext(r.Context())
        if _, err := service.SetBudget(r.Context(), command); err != nil {
                redirectEntitlementError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/entitlement?budget_saved=1", http.StatusSeeOther)
}

func budgetCommandFromForm(r *http.Request) (entitlementAPI.BudgetCommand, error) {
        amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
        if err != nil {
                return entitlementAPI.BudgetCommand{}, entitlementDomain.RuleError{Rule: "amount", Reason: "Бюджет задаётся числом."}
        }
        command := entitlementAPI.BudgetCommand{
                Amount:         amount,
                Currency:       r.FormValue("currency"),
                WarningPercent: formInt(r, "warning_threshold_percent", 80),
                LimitPolicy:    r.FormValue("limit_policy"),
                Note:           r.FormValue("note"),
        }
        start, err := parseDateForm(r.FormValue("period_start"))
        if err != nil {
                return entitlementAPI.BudgetCommand{}, err
        }
        end, err := parseDateEndForm(r.FormValue("period_end"))
        if err != nil {
                return entitlementAPI.BudgetCommand{}, err
        }
        if start == nil || end == nil {
                return entitlementAPI.BudgetCommand{}, entitlementDomain.RuleError{Rule: "period", Reason: "Укажите период измерения бюджета."}
        }
        command.PeriodStart, command.PeriodEnd = *start, *end
        if limit := r.FormValue("hard_limit"); limit != "" {
                value, err := strconv.ParseFloat(limit, 64)
                if err != nil {
                        return entitlementAPI.BudgetCommand{}, entitlementDomain.RuleError{Rule: "hard_limit", Reason: "Hard limit задаётся числом."}
                }
                command.HardLimit = &value
        }
        return command, nil
}

// parseDateForm parses an ISO date (YYYY-MM-DD) at midnight UTC. An empty
// value stays nil (an open question); a malformed value is an honest error,
// never a silent drop.
func parseDateForm(value string) (*time.Time, error) {
        if value == "" {
                return nil, nil
        }
        parsed, err := time.Parse(time.RFC3339, value+"T00:00:00Z")
        if err != nil {
                return nil, entitlementDomain.RuleError{Rule: "period", Reason: "Дата не распознана (ожидается ГГГГ-ММ-ДД)."}
        }
        return &parsed, nil
}

// parseDateEndForm parses an ISO date at the last second of the day.
func parseDateEndForm(value string) (*time.Time, error) {
        if value == "" {
                return nil, nil
        }
        parsed, err := time.Parse(time.RFC3339, value+"T23:59:59Z")
        if err != nil {
                return nil, entitlementDomain.RuleError{Rule: "period", Reason: "Дата не распознана (ожидается ГГГГ-ММ-ДД)."}
        }
        return &parsed, nil
}

func formInt(r *http.Request, name string, fallback int) int {
        if value, err := strconv.Atoi(r.FormValue(name)); err == nil {
                return value
        }
        return fallback
}

func (s *Server) sourceForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err := r.ParseForm(); err != nil {
                s.entitlementProblem(w, r, entitlementDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        categories := make([]entitlementDomain.UsageCategory, 0, len(r.Form["categories"]))
        for _, category := range r.Form["categories"] {
                categories = append(categories, entitlementDomain.UsageCategory(category))
        }
        corr, _ := correlation.FromContext(r.Context())
        if _, err := service.RegisterSource(r.Context(), entitlementAPI.SourceCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Name: r.FormValue("name"), Categories: categories,
                FreshnessMinutes: formInt(r, "freshness_minutes", 1440),
                Retire: r.FormValue("retire") == "yes",
                Note:  r.FormValue("note"),
        }); err != nil {
                redirectEntitlementError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/entitlement?source_saved=1", http.StatusSeeOther)
}

func (s *Server) budgetPage(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        meter, err := service.MeterView(r.Context(), actor, tenant)
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        exceptions, err := service.ExceptionsView(r.Context(), actor, tenant)
        if err != nil {
                s.entitlementProblem(w, r, err)
                return
        }
        data := map[string]any{
                "TenantID":        tenant,
                "Meter":           prepareMeterView(meter),
                "Exceptions":      prepareExceptionViews(exceptions),
                "CategoryOptions": usageCategoryOptions(),
                "IngestKey":       s.freshKey("usage"),
        }
        if v := r.URL.Query().Get("ingested"); v == "1" {
                data["Notice"] = "Usage-событие учтено."
        }
        if v := r.URL.Query().Get("replayed"); v == "1" {
                data["Notice"] = "Usage-событие уже было учтено: повторная доставка не считается дважды."
        }
        if v := r.URL.Query().Get("ack_saved"); v == "1" {
                data["Notice"] = "Подтверждение исключения зафиксировано."
        }
        if message := flashError(r, tenant); message != "" {
                data["Error"] = message
                http.SetCookie(w, clearFlashCookie(tenant))
        }
        s.render(w, "budget", data)
}

func (s *Server) ingestForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err := r.ParseForm(); err != nil {
                s.entitlementProblem(w, r, entitlementDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        quantity, err := strconv.ParseFloat(r.FormValue("quantity"), 64)
        if err != nil {
                redirectBudgetError(w, r, tenant, entitlementDomain.RuleError{Rule: "quantity", Reason: "Количество задаётся числом."})
                return
        }
        command := entitlementAPI.UsageEventCommand{
                Actor: actor, TenantID: tenant,
                EventKey:   r.FormValue("event_key"),
                Source:     r.FormValue("source"),
                Category:   entitlementDomain.UsageCategory(r.FormValue("category")),
                Quantity:   quantity,
                Currency:   r.FormValue("currency"),
                Estimate:   r.FormValue("is_estimate") == "yes",
                EmployeeID: r.FormValue("employee_id"),
                Channel:    r.FormValue("channel"),
                Provenance: r.FormValue("provenance"),
        }
        command.CorrelationID, _ = correlation.FromContext(r.Context())
        start, err := parseDateForm(r.FormValue("period_start"))
        if err != nil {
                redirectBudgetError(w, r, tenant, err)
                return
        }
        end, err := parseDateEndForm(r.FormValue("period_end"))
        if err != nil {
                redirectBudgetError(w, r, tenant, err)
                return
        }
        if start == nil || end == nil {
                redirectBudgetError(w, r, tenant, entitlementDomain.RuleError{Rule: "period", Reason: "Укажите период события."})
                return
        }
        command.PeriodStart, command.PeriodEnd = *start, *end
        if amount := r.FormValue("amount"); amount != "" {
                value, err := strconv.ParseFloat(amount, 64)
                if err != nil {
                        redirectBudgetError(w, r, tenant, entitlementDomain.RuleError{Rule: "amount", Reason: "Сумма задаётся числом."})
                        return
                }
                command.Amount = &value
        }
        _, replayed, err := service.IngestUsage(r.Context(), command)
        if err != nil {
                redirectBudgetError(w, r, tenant, err)
                return
        }
        flag := "ingested=1"
        if replayed {
                flag = "replayed=1"
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/budget?"+flag, http.StatusSeeOther)
}

func (s *Server) exceptionForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err := r.ParseForm(); err != nil {
                s.entitlementProblem(w, r, entitlementDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        if _, err := service.RaiseException(r.Context(), entitlementAPI.ManualExceptionCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Cause:             r.FormValue("cause"),
                AffectedEmployee:  r.FormValue("affected_employee"),
                AffectedRoute:     r.FormValue("affected_route"),
                RecommendedAction: r.FormValue("recommended_action"),
                Message:           r.FormValue("message"),
        }); err != nil {
                redirectBudgetError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/budget?ingested=1", http.StatusSeeOther)
}

func (s *Server) ackForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.entitlementScope(w, r)
        if !ok {
                return
        }
        service := s.entitlementService()
        if service == nil {
                s.entitlementProblem(w, r, entitlementDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
        if err := r.ParseForm(); err != nil {
                s.entitlementProblem(w, r, entitlementDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        exceptionID, err := id.Parse(r.FormValue("exception_id"))
        if err != nil {
                redirectBudgetError(w, r, tenant, entitlementDomain.ErrNotFound)
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        if err := service.AcknowledgeException(r.Context(), entitlementAPI.AckCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                ExceptionID: exceptionID, Note: r.FormValue("note"),
        }); err != nil {
                redirectBudgetError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/budget?ack_saved=1", http.StatusSeeOther)
}

func redirectEntitlementError(w http.ResponseWriter, r *http.Request, tenant id.UUID, err error) {
        redirectErrorTo(w, r, tenant, "/entitlement", err)
}

func redirectBudgetError(w http.ResponseWriter, r *http.Request, tenant id.UUID, err error) {
        redirectErrorTo(w, r, tenant, "/budget", err)
}

func redirectErrorTo(w http.ResponseWriter, r *http.Request, tenant id.UUID, path string, err error) {
        var rule entitlementDomain.RuleError
        if errors.As(err, &rule) {
                setFlashError(w, tenant, rule.Error())
        } else {
                setFlashError(w, tenant, "Действие не выполнено. Повторите позднее.")
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+path, http.StatusSeeOther)
}
