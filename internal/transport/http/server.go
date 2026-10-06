package httptransport

import (
        "encoding/json"
        "errors"
        "html/template"
        "io"
        "io/fs"
        "log/slog"
        "mime"
        "net/http"
        "slices"
        "strings"
        "time"

        employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
        "github.com/artemiosu/staffy/internal/identity/api"
        "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/identity/port"
        connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
        privacyAPI "github.com/artemiosu/staffy/internal/privacy/api"
        "github.com/artemiosu/staffy/internal/platform/correlation"
        "github.com/artemiosu/staffy/internal/platform/httpx"
        "github.com/artemiosu/staffy/internal/platform/id"
        staffyweb "github.com/artemiosu/staffy/web"
)

type IDGenerator interface {
        New(time.Time) (id.UUID, error)
}
type Clock interface{ Now() time.Time }

type Server struct {
        identity  api.Service
        employee  employeeAPI.Service
        knowledge employeeAPI.KnowledgeService
        ingestion employeeAPI.KnowledgeIngestService
        connector connectorAPI.Service
        privacy   privacyAPI.Service
        resolver  port.PrincipalResolver
        ids       IDGenerator
        clock     Clock
        logger    *slog.Logger
        templates *template.Template
        mux       *http.ServeMux
}

func New(identity api.Service, resolver port.PrincipalResolver, ids IDGenerator, clock Clock, logger *slog.Logger) (*Server, error) {
        templates, err := template.New("staffy").Funcs(template.FuncMap{
                "hasRole":   func(roles []domain.Role, role string) bool { return slices.Contains(roles, domain.Role(role)) },
                "hasScope":  func(scopes []domain.Scope, scope string) bool { return slices.Contains(scopes, domain.Scope(scope)) },
                "joinLines": func(values []string) string { return strings.Join(values, "\n") },
                "contains":  func(values []string, value string) bool { return slices.Contains(values, value) },
                "statusNote": func(status, auth, transport, environment string) string { return connectionStatusText(status, auth, transport, environment) },
                "stepStatus": func(status string) string {
                        return map[string]string{"complete": "Готово", "incomplete": "Не завершено", "blocked": "Заблокировано", "requires review": "Требует проверки"}[status]
                },
        }).ParseFS(staffyweb.Assets, "templates/*.html")
        if err != nil {
                return nil, err
        }
        assets, err := fs.Sub(staffyweb.Assets, "assets")
        if err != nil {
                return nil, err
        }
        s := &Server{identity: identity, resolver: resolver, ids: ids, clock: clock, logger: logger, templates: templates, mux: http.NewServeMux()}
        s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
        s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
        s.mux.HandleFunc("GET /app", s.setup)
        s.mux.HandleFunc("POST /app/tenants", s.createTenantForm)
        s.mux.HandleFunc("GET /app/tenants/{tenant_id}", s.workspace)
        s.mux.HandleFunc("POST /v1/tenants", s.createTenantAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/members", s.listMembersAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/invitations", s.inviteMemberAPI)
        s.mux.HandleFunc("POST /v1/invitations/accept", s.acceptInvitationAPI)
        s.mux.HandleFunc("PATCH /v1/tenants/{tenant_id}/members/{principal_id}", s.changeMemberAPI)
        s.mux.HandleFunc("DELETE /v1/tenants/{tenant_id}/members/{principal_id}", s.revokeMemberAPI)
        s.mux.HandleFunc("GET /app/tenants/{tenant_id}/people", s.people)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/people", s.inviteMemberForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/people/change", s.changeMemberForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/people/revoke", s.revokeMemberForm)
        s.mux.HandleFunc("GET /app/invitations/accept", s.acceptInvitationPage)
        s.mux.HandleFunc("POST /app/invitations/accept", s.acceptInvitationForm)
        return s, nil
}

func (s *Server) Handler() http.Handler { return s.withCorrelation(s.mux) }

func (s *Server) BindEmployee(service employeeAPI.Service) {
        s.employee = service
        if knowledge, ok := service.(employeeAPI.KnowledgeService); ok {
                s.knowledge = knowledge
                s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources", s.createKnowledgeAPI)
                s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources", s.listKnowledgeAPI)
                s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/{source_id}", s.readKnowledgeAPI)
                s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/{source_id}/process", s.processKnowledgeAPI)
                s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/{source_id}/conflict", s.conflictKnowledgeAPI)
                s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/{source_id}/retire", s.retireKnowledgeAPI)
                s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources", s.createKnowledgeForm)
                s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/{source_id}/process", s.processKnowledgeForm)
                s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/{source_id}/conflict", s.conflictKnowledgeForm)
                s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/{source_id}/retire", s.retireKnowledgeForm)
        }
        if ingestion, ok := service.(employeeAPI.KnowledgeIngestService); ok {
                s.ingestion = ingestion
                s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/ingest", s.ingestKnowledgeAPI)
                s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/{source_id}/ingest-fixture", s.reingestFixtureAPI)
                s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/ingest", s.ingestKnowledgeForm)
                s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/knowledge-sources/{source_id}/ingest-fixture", s.reingestFixtureForm)
        }
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees", s.createEmployeeAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees", s.listEmployeesAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}", s.readEmployeeAPI)
        s.mux.HandleFunc("PATCH /v1/tenants/{tenant_id}/employees/{employee_id}/brief", s.saveBriefAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/domain-packs", s.domainCatalogAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/domain-pack/preview", s.previewPackAPI)
        s.mux.HandleFunc("PUT /v1/tenants/{tenant_id}/employees/{employee_id}/domain-pack", s.pinPackAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/behavior", s.readBehaviorAPI)
        s.mux.HandleFunc("PUT /v1/tenants/{tenant_id}/employees/{employee_id}/behavior", s.saveBehaviorAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/behavior/preview", s.previewBehaviorAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/voice-profiles", s.voiceCatalogAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/voice", s.readVoiceAPI)
        s.mux.HandleFunc("PUT /v1/tenants/{tenant_id}/employees/{employee_id}/voice", s.saveVoiceAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/voice/preview", s.previewVoiceAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/voice/assessments", s.readVoiceAssessmentsAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/voice/assessments", s.assessVoiceAPI)
        s.mux.HandleFunc("GET /app/tenants/{tenant_id}/employees", s.employeesPage)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees", s.createEmployeeForm)
        s.mux.HandleFunc("GET /app/tenants/{tenant_id}/employees/{employee_id}", s.employeePage)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/brief", s.saveBriefForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/domain-pack", s.pinPackForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/behavior", s.saveBehaviorForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/voice", s.saveVoiceForm)
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
        if _, ok := s.principal(w, r); !ok {
                return
        }
        key, err := s.ids.New(s.clock.Now())
        if err != nil {
                s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось подготовить форму.", true, nil)
                return
        }
        s.render(w, "setup", map[string]any{"IdempotencyKey": key.String()})
}

func (s *Server) createTenantForm(w http.ResponseWriter, r *http.Request) {
        principal, ok := s.principal(w, r)
        if !ok {
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
        if err := r.ParseForm(); err != nil {
                s.problem(w, r, http.StatusBadRequest, "invalid_form", "Проверьте данные формы.", false, nil)
                return
        }
        result, err := s.create(r, principal, r.FormValue("name"), r.FormValue("idempotency_key"))
        if err != nil {
                key := r.FormValue("idempotency_key")
                if key == "" || errors.Is(err, domain.ErrInvalidIdempotency) || errors.Is(err, domain.ErrIdempotencyConflict) {
                        key = s.freshKey(key)
                }
                status, message := publicError(err)
                s.renderStatus(w, status, "setup", map[string]any{"IdempotencyKey": key, "Name": r.FormValue("name"), "Error": message})
                return
        }
        http.Redirect(w, r, "/app/tenants/"+result.Tenant.ID.String(), http.StatusSeeOther)
}

func (s *Server) createTenantAPI(w http.ResponseWriter, r *http.Request) {
        principal, ok := s.principal(w, r)
        if !ok {
                return
        }
        mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
        if err != nil || mediaType != "application/json" {
                s.problem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Используйте Content-Type application/json.", false, nil)
                return
        }
        var input struct {
                Name string `json:"name"`
        }
        decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
        decoder.DisallowUnknownFields()
        if err := decoder.Decode(&input); err != nil {
                s.problem(w, r, http.StatusBadRequest, "invalid_request", "Проверьте поля запроса.", false, map[string]string{"name": "required"})
                return
        }
        if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
                s.problem(w, r, http.StatusBadRequest, "invalid_request", "Тело запроса должно содержать один JSON-объект.", false, nil)
                return
        }
        result, err := s.create(r, principal, input.Name, r.Header.Get("Idempotency-Key"))
        if err != nil {
                status, message := publicError(err)
                s.problem(w, r, status, errorCode(err), message, retryable(err), nil)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Location", "/app/tenants/"+result.Tenant.ID.String())
        if result.Replayed {
                w.Header().Set("Idempotency-Replayed", "true")
        }
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(map[string]any{
                "tenant_id": result.Tenant.ID, "name": result.Tenant.Name, "role": result.Role,
                "workspace_url": "/app/tenants/" + result.Tenant.ID.String(),
        })
}

func (s *Server) create(r *http.Request, principal domain.Principal, name, key string) (api.CreateTenantResult, error) {
        correlationID, _ := correlation.FromContext(r.Context())
        return s.identity.CreateTenant(r.Context(), api.CreateTenantCommand{Principal: principal, Name: name, IdempotencyKey: key, CorrelationID: correlationID})
}

func (s *Server) workspace(w http.ResponseWriter, r *http.Request) {
        principal, ok := s.principal(w, r)
        if !ok {
                return
        }
        tenantID, err := id.Parse(r.PathValue("tenant_id"))
        if err != nil {
                s.notFound(w, r, principal)
                return
        }
        workspace, err := s.identity.Workspace(r.Context(), principal, tenantID)
        if err != nil {
                if errors.Is(err, domain.ErrNotFound) {
                        s.notFound(w, r, principal)
                        return
                }
                correlationID, _ := correlation.FromContext(r.Context())
                s.logger.ErrorContext(r.Context(), "workspace read failed", "code", "workspace_read_failed", "principal_id", principal.ID.String(), "correlation_id", correlationID.String())
                s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось открыть пространство. Повторите позднее.", true, nil)
                return
        }
        role := map[string]string{"owner": "Owner", "configurator": "Configurator", "operator": "Operator", "expert_qa": "Expert/QA", "integration_admin": "Integration Admin"}[workspace.Role]
        if role == "" {
                role = "Неизвестна"
        }
        canConfigure := false
        if s.employee != nil {
                if membership, ok := s.identity.(api.MembershipService); ok {
                        canConfigure = membership.Authorize(r.Context(), principal, tenantID, domain.Configuration) == nil
                }
        }
        data := map[string]any{"TenantID": workspace.Tenant.ID, "TenantName": workspace.Tenant.Name, "Role": role, "CanManage": workspace.Role == "owner", "CanConfigure": canConfigure}
        if s.employee != nil && data["CanConfigure"].(bool) {
                items, err := s.employee.List(r.Context(), principal, tenantID)
                if err == nil {
                        data["Employees"] = items
                        data["EmployeeKey"] = s.freshKey("")
                }
        }
        s.render(w, "workspace", data)
}

func (s *Server) principal(w http.ResponseWriter, r *http.Request) (domain.Principal, bool) {
        principal, err := s.resolver.Resolve(r.Context())
        if err != nil || !principal.ID.Valid() {
                s.problem(w, r, http.StatusUnauthorized, "identity_required", "Требуется подтверждённая личность.", false, nil)
                return domain.Principal{}, false
        }
        return principal, true
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request, principal domain.Principal) {
        correlationID, _ := correlation.FromContext(r.Context())
        s.logger.WarnContext(r.Context(), "tenant access denied", "code", "tenant_access_denied", "principal_id", principal.ID.String(), "correlation_id", correlationID.String())
        s.problem(w, r, http.StatusNotFound, "not_found", "Пространство не найдено.", false, nil)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
                s.logger.Error("render template", "template", name, "error", err)
        }
}

func (s *Server) renderStatus(w http.ResponseWriter, status int, name string, data any) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.WriteHeader(status)
        if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
                s.logger.Error("render template", "template", name, "error", err)
        }
}

func (s *Server) freshKey(fallback string) string {
        generated, err := s.ids.New(s.clock.Now())
        if err != nil {
                return fallback
        }
        return generated.String()
}

func (s *Server) problem(w http.ResponseWriter, r *http.Request, status int, code, message string, retryable bool, fields map[string]string) {
        correlationID, _ := correlation.FromContext(r.Context())
        httpx.WriteProblem(w, status, httpx.Problem{Code: code, Message: message, CorrelationID: correlationID.String(), Retryable: retryable, FieldErrors: fields})
}

func (s *Server) withCorrelation(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                correlationID, err := id.Parse(r.Header.Get("X-Correlation-ID"))
                if err != nil {
                        correlationID, err = s.ids.New(s.clock.Now())
                }
                if err != nil {
                        http.Error(w, "internal error", http.StatusInternalServerError)
                        return
                }
                w.Header().Set("X-Correlation-ID", correlationID.String())
                next.ServeHTTP(w, r.WithContext(correlation.WithID(r.Context(), correlationID)))
        })
}

func publicError(err error) (int, string) {
        switch {
        case errors.Is(err, domain.ErrForbidden):
                return http.StatusForbidden, "Недостаточно полномочий."
        case errors.Is(err, domain.ErrInvalidName):
                return http.StatusBadRequest, "Введите название компании от 2 до 160 символов."
        case errors.Is(err, domain.ErrInvalidIdempotency):
                return http.StatusBadRequest, "Обновите форму и повторите попытку."
        case errors.Is(err, domain.ErrIdempotencyConflict):
                return http.StatusConflict, "Этот запрос уже использован с другими данными."
        default:
                return http.StatusInternalServerError, "Не удалось выполнить запрос. Повторите позднее."
        }
}

func errorCode(err error) string {
        switch {
        case errors.Is(err, domain.ErrForbidden):
                return "forbidden"
        case errors.Is(err, domain.ErrInvalidName):
                return "invalid_tenant_name"
        case errors.Is(err, domain.ErrInvalidIdempotency):
                return "invalid_idempotency_key"
        case errors.Is(err, domain.ErrIdempotencyConflict):
                return "idempotency_conflict"
        default:
                return "internal_error"
        }
}

func retryable(err error) bool {
        return !errors.Is(err, domain.ErrForbidden) &&
                !errors.Is(err, domain.ErrInvalidName) &&
                !errors.Is(err, domain.ErrInvalidIdempotency) &&
                !errors.Is(err, domain.ErrIdempotencyConflict)
}
