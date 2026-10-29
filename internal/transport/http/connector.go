package httptransport

import (
	"bytes"
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
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/connections/{connection_id}/capabilities", s.capabilityStateAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/connections/{connection_id}/capabilities/{capability_id}/verify", s.verifyCapabilityAPI)
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
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/connections/{connection_id}/capabilities/{capability_id}/verify", s.verifyCapabilityForm)
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
		ConnectorID  string            `json:"connector_id"`
		ConnectorVer string            `json:"connector_version"`
		Label        string            `json:"label"`
		Environment  string            `json:"environment"`
		Config       map[string]string `json:"config"`
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
	result, err := service.Connect(r.Context(), connectorAPI.ConnectCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: input.Version, Secrets: input.Secrets, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		s.connectorProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if result.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"connection": result.Connection})
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
	result, err := service.Rotate(r.Context(), connectorAPI.ConnectCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: input.Version, Secrets: input.Secrets, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		s.connectorProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if result.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"connection": result.Connection})
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
	result, err := service.Revoke(r.Context(), connectorAPI.RevokeCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: input.Version, Reason: input.Reason, IdempotencyKey: r.Header.Get("Idempotency-Key")})
	if err != nil {
		s.connectorProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if result.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"connection": result.Connection})
}

func (s *Server) capabilityStateAPI(w http.ResponseWriter, r *http.Request) {
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
	state, err := service.CapabilityState(r.Context(), actor, tenant, connectionID)
	if err != nil {
		s.connectorProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"connection_id": state.Connection.ConnectionID, "contract_version": state.ContractVersion, "adapter_build": state.AdapterBuild, "capabilities": state.Capabilities})
}

func (s *Server) verifyCapabilityAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.connectorScope(w, r)
	if !ok {
		return
	}
	connectionID, err := id.Parse(r.PathValue("connection_id"))
	if err != nil {
		s.connectorProblem(w, r, connectorDomain.ErrNotFound)
		return
	}
	capabilityID := r.PathValue("capability_id")
	service := s.connectorService()
	if service == nil {
		s.connectorProblem(w, r, connectorDomain.ErrNotFound)
		return
	}
	// The optional probe key of the external MCP class (Story 7-4): the
	// secrets ride the command in memory only — the service MAC-checks them
	// against the accepted seal BEFORE any network round (an empty body is
	// the honest unauthenticated refusal of that class, never an error
	// here).
	secrets, err := decodeVerifySecrets(w, r)
	if err != nil {
		s.connectorProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	result, err := service.VerifyCapability(r.Context(), connectorAPI.VerifyCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: capabilityID, IdempotencyKey: r.Header.Get("Idempotency-Key"), Secrets: secrets})
	if err != nil {
		s.connectorProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if result.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"capability": result.Report, "replayed": result.Replayed})
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
	views := prepareConnectionViews(connections, defs)
	// Capability state is derived per connection (facts + evidence); every
	// card also gets its own fresh idempotency key for verify/connect forms.
	for i := range views {
		state, err := service.CapabilityState(r.Context(), actor, tenant, views[i].ConnectionID)
		if err != nil {
			s.connectorProblem(w, r, err)
			return
		}
		views[i].Capabilities = prepareCapabilityViews(state, s.freshKey("verify"))
		views[i].FormKey = s.freshKey("conn")
	}
	key := s.freshKey("")
	data := map[string]any{
		"TenantID":       tenant,
		"Groups":         grouped,
		"Connections":    views,
		"IdempotencyKey": key,
	}
	if v := r.URL.Query().Get("saved"); v == "1" {
		data["Notice"] = "Подключение сохранено."
	}
	if v := r.URL.Query().Get("connected"); v == "1" {
		data["Notice"] = "Синтетическая проверка выполнена. Статус подключения обновлён."
	}
	if v := r.URL.Query().Get("check_failed"); v == "1" {
		data["Notice"] = "Синтетическая проверка не прошла. Статус подключения обновлён; безопасное объяснение и следующий шаг — в карточке подключения."
	}
	if v := r.URL.Query().Get("capability_verified"); v == "1" {
		data["Notice"] = "Проверка возможности выполнена: доказательство сохранено, состояние обновлено в карточке подключения."
	}
	if v := r.URL.Query().Get("capability_failed"); v == "1" {
		data["Notice"] = "Проверка возможности не прошла: состояние и причина — в карточке подключения."
	}
	if v := r.URL.Query().Get("capability_unknown"); v == "1" {
		data["Notice"] = "Проверка возможности не завершилась до срока: результат неопределён, повтор вслепую исключён. Владелец сверки и безопасный шаг — в карточке."
	}
	if v := r.URL.Query().Get("capability_stale"); v == "1" {
		data["Notice"] = "Доказательство устарело (факты подключения изменились): состояние и причина — в карточке подключения; повторите проверку осознанно."
	}
	// Error flash rides a short-lived HttpOnly cookie, never the URL: an
	// attacker-chosen ?error=… can no longer render as a system alert.
	if message := flashError(r, tenant); message != "" {
		data["Error"] = message
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	s.render(w, "integrations", data)
}

// flashCookieName identifies the one-shot error flash.
const flashCookieName = "staffy_flash_error"

// flashCookiePath is scoped to the whole tenant area so the flash set by any
// module form (integrations, data map) is readable on its own page.
func flashCookiePath(tenant id.UUID) string { return "/app/tenants/" + tenant.String() }

func setFlashError(w http.ResponseWriter, tenant id.UUID, message string) {
	http.SetCookie(w, &http.Cookie{
		Name: flashCookieName, Value: url.QueryEscape(message), Path: flashCookiePath(tenant),
		MaxAge: 300, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

func clearFlashCookie(tenant id.UUID) *http.Cookie {
	return &http.Cookie{Name: flashCookieName, Value: "", Path: flashCookiePath(tenant), MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}

func flashError(r *http.Request, tenant id.UUID) string {
	cookie, err := r.Cookie(flashCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	message, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return ""
	}
	message = strings.TrimSpace(message)
	if len([]rune(message)) > 200 {
		return "Действие не выполнено."
	}
	return message
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
	if result.Connection.Status != connectorDomain.StatusDraft {
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?check_failed=1", http.StatusSeeOther)
		return
	}
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
	result, err := service.Connect(r.Context(), connectorAPI.ConnectCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: version, Secrets: secrets, IdempotencyKey: r.FormValue("idempotency_key")})
	if err != nil {
		redirectWithError(w, r, tenant, err)
		return
	}
	if result.Connection.Status == connectorDomain.StatusFailed {
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?check_failed=1", http.StatusSeeOther)
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
	result, err := service.Rotate(r.Context(), connectorAPI.ConnectCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: version, Secrets: secrets, IdempotencyKey: r.FormValue("idempotency_key")})
	if err != nil {
		redirectWithError(w, r, tenant, err)
		return
	}
	if result.Connection.Status == connectorDomain.StatusFailed {
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?check_failed=1", http.StatusSeeOther)
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
	_, err = service.Revoke(r.Context(), connectorAPI.RevokeCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: version, Reason: r.FormValue("reason"), IdempotencyKey: r.FormValue("idempotency_key")})
	if err != nil {
		redirectWithError(w, r, tenant, err)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?saved=1", http.StatusSeeOther)
}

// verifyCapabilityForm runs the safe read-only probe from the connection card.
// The result redirects with an honest outcome-specific notice; failures ride
// the one-shot flash cookie exactly like the other connector forms.
func (s *Server) verifyCapabilityForm(w http.ResponseWriter, r *http.Request) {
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
	capabilityID := r.PathValue("capability_id")
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
	// The optional probe key of the external MCP class (Story 7-4): the
	// same declared-field form discipline as the connect form — the value
	// MAC-checks against the accepted seal before any network round and is
	// never stored again.
	secrets := map[string]string{}
	for _, field := range r.Form["secret_field"] {
		if value := r.FormValue("secret_" + field); value != "" {
			secrets[field] = value
		}
	}
	corr, _ := correlation.FromContext(r.Context())
	result, err := service.VerifyCapability(r.Context(), connectorAPI.VerifyCommand{Actor: actor, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: capabilityID, IdempotencyKey: r.FormValue("idempotency_key"), Secrets: secrets})
	if err != nil {
		redirectWithError(w, r, tenant, err)
		return
	}
	// Honest redirects: every non-success outcome carries its own flag; any
	// state outside the terminal vocabulary redirects without claiming
	// success (R1: a stale replay must never read as a completed check).
	switch result.Report.State {
	case connectorDomain.CapStateVerified:
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?capability_verified=1", http.StatusSeeOther)
	case connectorDomain.CapStateFailed:
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?capability_failed=1", http.StatusSeeOther)
	case connectorDomain.CapStateUnknown:
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?capability_unknown=1", http.StatusSeeOther)
	case connectorDomain.CapStateStale:
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations?capability_stale=1", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations", http.StatusSeeOther)
	}
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
	// The message is always a fixed server-side string (never a submitted
	// value); it travels in a one-shot HttpOnly cookie instead of the URL.
	setFlashError(w, tenant, message)
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/integrations", http.StatusSeeOther)
}

// decodeVerifySecrets reads the OPTIONAL secrets body of the verify
// command: an absent or empty body answers nil (the classes without probe
// keys verify exactly as before); a present body must be the closed
// {"secrets": {...}} form under the declared transport budgets.
func decodeVerifySecrets(w http.ResponseWriter, r *http.Request) (map[string]string, error) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<10))
	if err != nil {
		return nil, connectorDomain.ErrInvalid
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, connectorDomain.ErrInvalid
	}
	var input struct {
		Secrets map[string]string `json:"secrets"`
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err = d.Decode(&input); err != nil {
		return nil, connectorDomain.ErrInvalid
	}
	if err = d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, connectorDomain.ErrInvalid
	}
	return input.Secrets, nil
}
