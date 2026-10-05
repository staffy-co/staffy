package httptransport

import (
        "encoding/json"
        "errors"
        "io"
        "mime"
        "net/http"
        "net/url"
        "strconv"
        "strings"

        connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/correlation"
        "github.com/artemiosu/staffy/internal/platform/id"
)

func (s *Server) connectorService() connectorAPI.Service { return s.connector }

func (s *Server) connectorScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, bool) {
        actor, ok := s.principal(w, r)
        if !ok {
                return actor, "", false
        }
        tenant, err := id.Parse(r.PathValue("tenant_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return actor, "", false
        }
        return actor, tenant, true
}

func (s *Server) connectorProblem(w http.ResponseWriter, r *http.Request, err error) {
        status, code, message := 500, "internal_error", "Не удалось выполнить действие. Повторите позднее."
        var fields map[string]string
        var rule connectorDomain.RuleError
        var blocked connectorDomain.ProductionBlockedError
        if errors.As(err, &rule) {
                message = rule.Error()
                fields = map[string]string{rule.Rule: rule.Reason}
        }
        if errors.As(err, &blocked) {
                status, code, message = 400, "production_blocked", blocked.Reason
                fields = map[string]string{blocked.Rule: blocked.Reason}
        }
        switch {
        case errors.Is(err, connectorDomain.ErrNotFound), errors.Is(err, connectorDomain.ErrForbidden):
                status, code, message = 404, "not_found", "Ресурс не найден."
        case errors.As(err, &rule):
                status, code = 400, "connector_rule"
        case errors.Is(err, connectorDomain.ErrInvalid):
                status, code, message = 400, "invalid_request", "Проверьте поля подключения и версию."
        case errors.Is(err, connectorDomain.ErrConflict):
                status, code, message = 409, "version_conflict", "Подключение изменено другим участником. Обновите страницу и повторите действие."
        case errors.Is(err, connectorDomain.ErrInvalidIdempotency):
                status, code, message = 400, "invalid_idempotency_key", "Обновите форму и повторите попытку."
        case errors.Is(err, connectorDomain.ErrIdempotencyConflict):
                status, code, message = 409, "idempotency_conflict", "Этот ключ уже использован для другого запроса."
        }
        s.problem(w, r, status, code, message, status >= 500, fields)
}

func decodeConnector(w http.ResponseWriter, r *http.Request, out any) error {
        media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
        if err != nil || media != "application/json" {
                return connectorDomain.ErrInvalid
        }
        d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
        d.DisallowUnknownFields()
        if err = d.Decode(out); err != nil {
                return connectorDomain.ErrInvalid
        }
        if err = d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
                return connectorDomain.ErrInvalid
        }
        return nil
}

func (s *Server) BindConnector(service connectorAPI.Service) {
        s.connector = service
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/connectors", s.connectorCatalogAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/connections", s.createConnectionAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/connections", s.listConnectionsAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/connections/{connection_id}", s.readConnectionAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/connections/{connection_id}/versions", s.readConnectionVersionsAPI)
        s.mux.HandleFunc("PUT /v1/tenants/{tenant_id}/connections/{connection_id}", s.saveConnectionAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/connections/{connection_id}/connect", s.connectConnectionAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/connections/{connection_id}/rotate", s.rotateConnectionAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/connections/{connection_id}/revoke", s.revokeConnectionAPI)
        s.mux.HandleFunc("GET /app/tenants/{tenant_id}/integrations", s.integrationsPage)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/connections", s.createConnectionForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/connections/{connection_id}/config", s.saveConnectionForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/connections/{connection_id}/connect", s.connectConnectionForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/connections/{connection_id}/rotate", s.rotateConnectionForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/connections/{connection_id}/revoke", s.revokeConnectionForm)
}

func (s *Server) connectorCatalogAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        defs, err := service.Catalog(r.Context(), actor, tenant)
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"connectors": defs})
}

func (s *Server) createConnectionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        var input struct {
                ConnectorID    string            `json:"connector_id"`
                ConnectorVer   string            `json:"connector_version"`
                Label          string            `json:"label"`
                Environment    string            `json:"environment"`
                Config         map[string]string `json:"config"`
        }
        if err := decodeConnector(w, r, &input); err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        result, err := service.Create(r.Context(), connectorAPI.CreateCommand{Actor: actor, TenantID: tenant, IdempotencyKey: r.Header.Get("Idempotency-Key"), CorrelationID: corr, ConnectorID: input.ConnectorID, ConnectorVer: input.ConnectorVer, Label: input.Label, Environment: connectorDomain.Environment(input.Environment), Config: input.Config})
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Location", "/v1/tenants/"+tenant.String()+"/connections/"+result.Connection.ConnectionID.String())
        if result.Replayed {
                w.Header().Set("Idempotency-Replayed", "true")
        }
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(map[string]any{"connection": result.Connection})
}

func (s *Server) listConnectionsAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        items, err := service.List(r.Context(), actor, tenant)
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"connections": items})
}

func (s *Server) readConnectionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        connection, err := service.Read(r.Context(), actor, tenant, connectionID)
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"connection": connection})
}

func (s *Server) readConnectionVersionsAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        versions, err := service.ReadVersions(r.Context(), actor, tenant, connectionID)
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"versions": versions})
}

func (s *Server) saveConnectionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        var input struct {
                Version       int64             `json:"version"`
                Label         string            `json:"label"`
                Config        map[string]string `json:"config"`
                ChangeSummary string            `json:"change_summary"`
        }
        if err = decodeConnector(w, r, &input); err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        connection, err := service.Save(r.Context(), connectorAPI.SaveCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: input.Version, Config: input.Config, Label: input.Label, ChangeSummary: input.ChangeSummary})
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"connection": connection})
}

func (s *Server) connectConnectionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        var input struct {
                Version int64             `json:"version"`
                Secrets map[string]string `json:"secrets"`
        }
        if err = decodeConnector(w, r, &input); err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        connection, err := service.Connect(r.Context(), connectorAPI.ConnectCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: input.Version, Secrets: input.Secrets})
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"connection": connection})
}

func (s *Server) rotateConnectionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        var input struct {
                Version int64             `json:"version"`
                Secrets map[string]string `json:"secrets"`
        }
        if err = decodeConnector(w, r, &input); err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        connection, err := service.Rotate(r.Context(), connectorAPI.ConnectCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: input.Version, Secrets: input.Secrets})
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"connection": connection})
}

func (s *Server) revokeConnectionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        var input struct {
                Version int64  `json:"version"`
                Reason  string `json:"reason"`
        }
        if err = decodeConnector(w, r, &input); err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        connection, err := service.Revoke(r.Context(), connectorAPI.RevokeCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: input.Version, Reason: input.Reason})
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"connection": connection})
}

// --- HTML UI ---

func (s *Server) integrationsPage(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        defs, err := service.Catalog(r.Context(), actor, tenant)
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        connections, err := service.List(r.Context(), actor, tenant)
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        grouped := prepareConnectorGroups(defs)
        key := s.freshKey("")
        data := map[string]any{
                "TenantID":       tenant,
                "Groups":         grouped,
                "Connections":    prepareConnectionViews(connections, defs),
                "IdempotencyKey": key,
        }
        if v := r.URL.Query().Get("saved"); v == "1" {
                data["Notice"] = "Подключение сохранено."
        }
        if v := r.URL.Query().Get("connected"); v == "1" {
                data["Notice"] = "Синтетическая проверка выполнена. Статус подключения обновлён."
        }
        if v := r.URL.Query().Get("error"); v != "" {
                data["Error"] = sanitizeQueryMessage(v)
        }
        s.render(w, "integrations", data)
}

func sanitizeQueryMessage(v string) string {
        v = strings.TrimSpace(v)
        if len([]rune(v)) > 200 {
                return "Действие не выполнено."
        }
        return v
}

func (s *Server) createConnectionForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        if !validFormOrigin(r) {
                s.connectorProblem(w, r, connectorDomain.ErrForbidden)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err := r.ParseForm(); err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrInvalid)
                return
        }
        config := map[string]string{}
        for _, field := range r.Form["config_field"] {
                if value := r.FormValue("config_" + field); value != "" {
                        config[field] = value
                }
        }
        corr, _ := correlation.FromContext(r.Context())
        result, err := service.Create(r.Context(), connectorAPI.CreateCommand{Actor: actor, TenantID: tenant, IdempotencyKey: r.FormValue("idempotency_key"), CorrelationID: corr, ConnectorID: r.FormValue("connector_id"), ConnectorVer: r.FormValue("connector_version"), Label: r.FormValue("label"), Environment: connectorDomain.Environment(r.FormValue("environment")), Config: config})
        if err != nil {
                redirectWithError(w, r, tenant, err)
                return
        }
        _ = result
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?saved=1", http.StatusSeeOther)
}

func (s *Server) saveConnectionForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        if !validFormOrigin(r) {
                s.connectorProblem(w, r, connectorDomain.ErrForbidden)
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err = r.ParseForm(); err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrInvalid)
                return
        }
        version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrInvalid)
                return
        }
        connection, err := service.Read(r.Context(), actor, tenant, connectionID)
        if err != nil {
                s.connectorProblem(w, r, err)
                return
        }
        config := map[string]string{}
        for _, field := range r.Form["config_field"] {
                if value := r.FormValue("config_" + field); value != "" {
                        config[field] = value
                }
        }
        corr, _ := correlation.FromContext(r.Context())
        _, err = service.Save(r.Context(), connectorAPI.SaveCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: version, Config: config, Label: r.FormValue("label"), ChangeSummary: r.FormValue("change_summary")})
        if err != nil {
                redirectWithError(w, r, tenant, err)
                return
        }
        _ = connection
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?saved=1", http.StatusSeeOther)
}

func (s *Server) connectConnectionForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        if !validFormOrigin(r) {
                s.connectorProblem(w, r, connectorDomain.ErrForbidden)
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err = r.ParseForm(); err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrInvalid)
                return
        }
        version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrInvalid)
                return
        }
        secrets := map[string]string{}
        for _, field := range r.Form["secret_field"] {
                if value := r.FormValue("secret_" + field); value != "" {
                        secrets[field] = value
                }
        }
        corr, _ := correlation.FromContext(r.Context())
        _, err = service.Connect(r.Context(), connectorAPI.ConnectCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: version, Secrets: secrets})
        if err != nil {
                redirectWithError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?connected=1", http.StatusSeeOther)
}

func (s *Server) rotateConnectionForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        if !validFormOrigin(r) {
                s.connectorProblem(w, r, connectorDomain.ErrForbidden)
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err = r.ParseForm(); err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrInvalid)
                return
        }
        version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrInvalid)
                return
        }
        secrets := map[string]string{}
        for _, field := range r.Form["secret_field"] {
                if value := r.FormValue("secret_" + field); value != "" {
                        secrets[field] = value
                }
        }
        corr, _ := correlation.FromContext(r.Context())
        _, err = service.Rotate(r.Context(), connectorAPI.ConnectCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: version, Secrets: secrets})
        if err != nil {
                redirectWithError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?connected=1", http.StatusSeeOther)
}

func (s *Server) revokeConnectionForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.connectorScope(w, r)
        if !ok {
                return
        }
        if !validFormOrigin(r) {
                s.connectorProblem(w, r, connectorDomain.ErrForbidden)
                return
        }
        connectionID, err := id.Parse(r.PathValue("connection_id"))
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        service := s.connectorService()
        if service == nil {
                s.connectorProblem(w, r, connectorDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err = r.ParseForm(); err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrInvalid)
                return
        }
        version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
        if err != nil {
                s.connectorProblem(w, r, connectorDomain.ErrInvalid)
                return
        }
        if r.FormValue("confirm_revoke") != "yes" {
                redirectWithError(w, r, tenant, connectorDomain.RuleError{Rule: "confirm", Reason: "Подтвердите отзыв подключения."})
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        _, err = service.Revoke(r.Context(), connectorAPI.RevokeCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: version, Reason: r.FormValue("reason")})
        if err != nil {
                redirectWithError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?saved=1", http.StatusSeeOther)
}

func redirectWithError(w http.ResponseWriter, r *http.Request, tenant id.UUID, err error) {
        message := "Действие не выполнено."
        var rule connectorDomain.RuleError
        var blocked connectorDomain.ProductionBlockedError
        if errors.As(err, &rule) {
                message = rule.Reason
        }
        if errors.As(err, &blocked) {
                message = blocked.Reason
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?error="+url.QueryEscape(message), http.StatusSeeOther)
}
