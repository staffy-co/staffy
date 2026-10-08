package httptransport

import (
        "context"
        "io"
        "log/slog"
        "net/http"
        "net/http/httptest"
        "net/url"
        "strings"
        "testing"

        connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        "github.com/artemiosu/staffy/internal/identity/api"
        "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
"github.com/artemiosu/staffy/internal/platform/tenancy"
)

type fakeConnector struct {
        catalog      []connectorDomain.ConnectorDefinition
        createErr    error
        connectErr   error
        connectCalls int
        connectStatus string
        lastConnect  *connectorAPI.ConnectCommand
        lastCreate   *connectorAPI.CreateCommand
        revokeErr    error
        verifyErr    error
        verifyState  string
        verifyReplayed bool
        withConnection bool
}

func (f *fakeConnector) Catalog(context.Context, domain.Principal, id.UUID) ([]connectorDomain.ConnectorDefinition, error) {
        if f.catalog != nil {
                return f.catalog, nil
        }
        return []connectorDomain.ConnectorDefinition{
                {ID: "staffy-synthetic-telephony", Version: "1.0.0", Type: connectorDomain.TypeSIP, Name: "Staffy Synthetic Telephony", Purpose: "Синтетический контур", SupportStatus: connectorDomain.StatusSupported, AdapterImplemented: true, AllowedEnvironments: []connectorDomain.Environment{connectorDomain.EnvSandbox}, RequiredAccountType: "synthetic", DocumentationURL: "https://example.com", Fields: []connectorDomain.FieldSchema{{Name: "endpoint", Label: "Endpoint", Type: connectorDomain.FieldURL, Required: true}}},
                {ID: "exolve-sip", Version: "1.0.0", Type: connectorDomain.TypeSIP, Name: "Exolve", Purpose: "Будущий провайдер", SupportStatus: connectorDomain.StatusUnavailable, AdapterImplemented: false, AllowedEnvironments: []connectorDomain.Environment{connectorDomain.EnvSandbox}, RequiredAccountType: "exolve", DocumentationURL: "https://example.com"},
        }, nil
}

func (f *fakeConnector) Create(_ context.Context, c connectorAPI.CreateCommand) (connectorAPI.CreateResult, error) {
        f.lastCreate = &c
        if f.createErr != nil {
                return connectorAPI.CreateResult{}, f.createErr
        }
        return connectorAPI.CreateResult{Connection: connectorDomain.Connection{TenantID: c.TenantID, ConnectionID: testCorrelation, ConnectorID: c.ConnectorID, ConnectorVer: c.ConnectorVer, Label: c.Label, Environment: c.Environment, Status: connectorDomain.StatusDraft, AuthState: connectorDomain.CheckPending, Transport: connectorDomain.CheckPending, Config: c.Config, OwnerID: c.Actor.ID, Version: 1}}, nil
}

func (f *fakeConnector) List(context.Context, domain.Principal, id.UUID) ([]connectorDomain.Connection, error) {
        if f.connectCalls == 0 && !f.withConnection {
                return nil, nil
        }
        return []connectorDomain.Connection{{TenantID: testTenantID, ConnectionID: testCorrelation, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.1.0", Label: "Тестовая телефония", Status: connectorDomain.StatusConnectedUnverified, AuthState: connectorDomain.CheckVerified, Transport: connectorDomain.CheckVerified, SecretID: testCorrelation, SecretHint: "abcdef012345", SecretActive: true, Version: 2, Config: map[string]string{"endpoint": "https://demo.sandbox.example"}}}, nil
}

func (f *fakeConnector) Read(context.Context, domain.Principal, id.UUID, id.UUID) (connectorDomain.Connection, error) {
        return connectorDomain.Connection{}, connectorDomain.ErrNotFound
}

func (f *fakeConnector) ReadVersions(context.Context, domain.Principal, id.UUID, id.UUID) ([]connectorDomain.ConnectionVersion, error) {
        return nil, nil
}

func (f *fakeConnector) Save(context.Context, connectorAPI.SaveCommand) (connectorDomain.Connection, error) {
        return connectorDomain.Connection{}, connectorDomain.ErrConflict
}

func (f *fakeConnector) Connect(_ context.Context, c connectorAPI.ConnectCommand) (connectorAPI.ConnectResult, error) {
        f.connectCalls++
        f.lastConnect = &c
        if f.connectErr != nil {
                return connectorAPI.ConnectResult{}, f.connectErr
        }
        status := f.connectStatus
        if status == "" {
                status = connectorDomain.StatusConnectedUnverified
        }
        return connectorAPI.ConnectResult{Connection: connectorDomain.Connection{TenantID: c.TenantID, ConnectionID: c.ConnectionID, Status: status, AuthState: connectorDomain.CheckVerified, Transport: connectorDomain.CheckVerified, SecretID: testCorrelation, SecretHint: "abcdef012345", SecretActive: true, Version: 2}}, nil
}

func (f *fakeConnector) Rotate(context.Context, connectorAPI.ConnectCommand) (connectorAPI.ConnectResult, error) {
        return connectorAPI.ConnectResult{}, connectorDomain.ErrInvalid
}

func (f *fakeConnector) Revoke(_ context.Context, c connectorAPI.RevokeCommand) (connectorAPI.RevokeResult, error) {
        if f.revokeErr != nil {
                return connectorAPI.RevokeResult{}, f.revokeErr
        }
        return connectorAPI.RevokeResult{Connection: connectorDomain.Connection{Status: connectorDomain.StatusRevoked, ConnectionID: c.ConnectionID}}, nil
}

func (f *fakeConnector) VerifyCapability(_ context.Context, c connectorAPI.VerifyCommand) (connectorAPI.VerifyResult, error) {
        if f.verifyErr != nil {
                return connectorAPI.VerifyResult{}, f.verifyErr
        }
        state := f.verifyState
        if state == "" {
                state = connectorDomain.CapStateVerified
        }
        return connectorAPI.VerifyResult{Report: connectorDomain.CapabilityReport{
                CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: c.Capability, Title: "Статус транка", TimeoutMS: 5000, Receipt: "query_result", ReceiptLevel: connectorDomain.DeliveryDelivered, RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа", Ordering: "unordered", CancelCompensation: "не применимо", Reconciliation: "не требуется", ReconciliationOwner: "Technical Lead"},
                State:     state,
                CanVerify: true,
        }, Replayed: f.verifyReplayed}, nil
}

func (f *fakeConnector) CapabilityState(_ context.Context, _ domain.Principal, _ id.UUID, connectionID id.UUID) (connectorAPI.CapabilityStateResult, error) {
        return connectorAPI.CapabilityStateResult{
                Connection:      connectorDomain.Connection{ConnectionID: connectionID, Status: connectorDomain.StatusConnectedUnverified, ConnectorVer: "1.1.0"},
                ContractVersion: "1.1.0",
                AdapterBuild:    "synthetic/1.1.0",
                Capabilities: []connectorDomain.CapabilityReport{
                        {CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: "query.trunk-status", Title: "Статус транка", Direction: connectorDomain.DirectionOutbound, Operation: connectorDomain.OperationQuery, TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа", Receipt: "query_result", ReceiptLevel: connectorDomain.DeliveryDelivered, CancelCompensation: "не применимо", Reconciliation: "не требуется", ReconciliationOwner: "Technical Lead", ProbeAllowed: true}, State: connectorDomain.CapStateVerified, StateReason: "Доказано синтетическим read-only probe.", CanVerify: true},
                        {CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: "command.send-sms", Title: "Отправка SMS", Direction: connectorDomain.DirectionOutbound, Operation: connectorDomain.OperationCommand, TimeoutMS: 10000, Ordering: "fifo", RateLimit: "30 сообщений/мин", SizeLimit: "1 КБ", Receipt: "accepted → delivered → committed", ReceiptLevel: connectorDomain.DeliveryCommitted, CancelCompensation: "отмена недоступна", Reconciliation: "query_reconcilable", ReconciliationOwner: "Technical Lead", ProbeAllowed: false}, State: connectorDomain.CapStateUnverified, StateReason: "Мутирующая capability: тестовое выполнение запрещено архитектурой до Action Gateway.", Block: &connectorDomain.ActionGatewayBlock},
                        {CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: "event.inbound-call", Title: "Входящий звонок", Direction: connectorDomain.DirectionInbound, Operation: connectorDomain.OperationEvent, TimeoutMS: 5000, Ordering: "fifo", RateLimit: "120 событий/мин", SizeLimit: "16 КБ", Receipt: "event_ack", ReceiptLevel: connectorDomain.DeliveryAccepted, CancelCompensation: "не применимо", Reconciliation: "дедупликация по event_id", ReconciliationOwner: "Technical Lead", ProbeAllowed: false}, State: connectorDomain.CapStateUnverified, StateReason: "Путь проверки входящих событий не реализован в текущем срезе."},
                },
        }, nil
}

// ConnectionsFacts is the caller-authority facts read of the fake: the same
// projection as the real service, scoped by the passed tenancy.
func (f *fakeConnector) ConnectionsFacts(ctx context.Context, _ tenancy.Context) ([]connectorAPI.ConnectionFacts, error) {
	state, err := f.CapabilityState(ctx, domain.Principal{}, "", "")
	if err != nil {
		return nil, err
	}
	return []connectorAPI.ConnectionFacts{{
		ConnectionID: state.Connection.ConnectionID, ConnectorID: state.Connection.ConnectorID,
		ConnectorType: "SIP/telephony",
		ContractVersion: state.ContractVersion, AdapterBuild: state.AdapterBuild,
		Environment: string(state.Connection.Environment), Status: state.Connection.Status,
		Capabilities: state.Capabilities,
	}}, nil
}

// ConnectionFactsByID is the single-connection facts read of the fake.
func (f *fakeConnector) ConnectionFactsByID(ctx context.Context, _ tenancy.Context, connectionID id.UUID) (connectorAPI.ConnectionFacts, error) {
	state, err := f.CapabilityState(ctx, domain.Principal{}, "", "")
	if err != nil || !connectionID.Valid() {
		return connectorAPI.ConnectionFacts{}, connectorDomain.ErrNotFound
	}
	return connectorAPI.ConnectionFacts{
		ConnectionID: state.Connection.ConnectionID, ConnectorID: state.Connection.ConnectorID,
		ConnectorType: "SIP/telephony",
		ContractVersion: state.ContractVersion, AdapterBuild: state.AdapterBuild,
		Environment: string(state.Connection.Environment), Status: state.Connection.Status,
		Capabilities: state.Capabilities,
	}, nil
}

func newConnectorTestServer(t *testing.T, fake *fakeConnector) *Server {
        t.Helper()
        identity := fakeIdentity{create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
                return api.CreateTenantResult{}, nil
        }, workspace: func(id.UUID) (domain.Workspace, error) {
                return domain.Workspace{}, nil
        }}
        server, err := New(identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
        if err != nil {
                t.Fatal(err)
        }
        server.BindConnector(fake)
        return server
}

func TestConnectorCatalogAPIGroupsByTypeTextually(t *testing.T) {
        server := newConnectorTestServer(t, &fakeConnector{})
        r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/connectors", nil)
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusOK {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
        body := w.Body.String()
        for _, want := range []string{"staffy-synthetic-telephony", "supported", "SIP/telephony", "exolve-sip", "unavailable"} {
                if !strings.Contains(body, want) {
                        t.Fatalf("catalog missing %q: %s", want, body)
                }
        }
}

func TestCreateConnectionAPIRejectsUnknownFields(t *testing.T) {
        fake := &fakeConnector{}
        server := newConnectorTestServer(t, fake)
        r := httptest.NewRequest(http.MethodPost, "/v1/tenants/"+testTenantID.String()+"/connections", strings.NewReader(`{"connector_id":"staffy-synthetic-telephony","connector_version":"1.0.0","label":"Тест","environment":"sandbox","config":{"endpoint":"https://demo.sandbox.example"},"extra":"nope"}`))
        r.Header.Set("Content-Type", "application/json")
        r.Header.Set("Idempotency-Key", "request-0001")
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusBadRequest {
                t.Fatalf("unknown fields must be rejected: %d %s", w.Code, w.Body.String())
        }
        if fake.lastCreate != nil {
                t.Fatal("rejected request must not reach the service")
        }
}

func TestCreateConnectionAPIProductionBlockedExplainsGate(t *testing.T) {
        fake := &fakeConnector{createErr: connectorDomain.ProductionBlockedError{RuleError: connectorDomain.RuleError{Rule: "environment", Reason: "Production-credentials запрещены до прохождения gate AG-4 (Dependency/provider readiness). Ответственный: Technical Lead + Security."}}}
        server := newConnectorTestServer(t, fake)
        r := httptest.NewRequest(http.MethodPost, "/v1/tenants/"+testTenantID.String()+"/connections", strings.NewReader(`{"connector_id":"staffy-synthetic-telephony","connector_version":"1.0.0","label":"Prod","environment":"sandbox","config":{}}`))
        r.Header.Set("Content-Type", "application/json")
        r.Header.Set("Idempotency-Key", "request-0002")
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusBadRequest {
                t.Fatalf("status=%d", w.Code)
        }
        body := w.Body.String()
        if !strings.Contains(body, "production_blocked") || !strings.Contains(body, "AG-4") || !strings.Contains(body, "Technical Lead") {
                t.Fatalf("problem must explain the gate: %s", body)
        }
}

func TestConnectConnectionAPIReceivesSecretsOnceAndNeverEchoes(t *testing.T) {
        fake := &fakeConnector{}
        server := newConnectorTestServer(t, fake)
        path := "/v1/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/connect"
        r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"version":1,"secrets":{"api_key":"sandbox-transport-key"}}`))
        r.Header.Set("Content-Type", "application/json")
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusOK {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
        if fake.lastConnect == nil || fake.lastConnect.ConnectionID != testCorrelation || fake.lastConnect.ExpectedVersion != 1 || fake.lastConnect.Secrets["api_key"] != "sandbox-transport-key" {
                t.Fatalf("connect command=%+v", fake.lastConnect)
        }
        if strings.Contains(w.Body.String(), "sandbox-transport-key") {
                t.Fatalf("response must never echo the secret: %s", w.Body.String())
        }
        if !strings.Contains(w.Body.String(), "connected_unverified") || !strings.Contains(w.Body.String(), `"secret_hint":"abcdef012345"`) {
                t.Fatalf("response must show honest status and hint: %s", w.Body.String())
        }
}

func TestIntegrationsPageShowsStatusesAndSecretInstructions(t *testing.T) {
        fake := &fakeConnector{}
        server := newConnectorTestServer(t, fake)
        r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/integrations", nil)
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusOK {
                t.Fatalf("status=%d", w.Code)
        }
        body := w.Body.String()
        for _, want := range []string{"Подключение недоступно: адаптер не реализован", "unavailable", "Статус:", "Подключений пока нет"} {
                if !strings.Contains(body, want) {
                        t.Fatalf("page missing %q", want)
                }
        }
        if strings.Contains(body, "sandbox-transport-key") {
                t.Fatal("page must never contain secret values")
        }
}

func TestConnectConnectionAPIConflictMapsTo409(t *testing.T) {
        fake := &fakeConnector{connectErr: connectorDomain.ErrConflict}
        server := newConnectorTestServer(t, fake)
        path := "/v1/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/connect"
        r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"version":1,"secrets":{"api_key":"sandbox-stale-value"}}`))
        r.Header.Set("Content-Type", "application/json")
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusConflict {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
}

func connectorFormRequest(method, path string, form url.Values) *http.Request {
        request := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
        request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
        request.Header.Set("Origin", "http://example.com")
        request.Header.Set("X-Correlation-ID", testCorrelation.String())
        return request
}

func TestConnectionFormErrorTravelsInFlashCookieNotURL(t *testing.T) {
        fake := &fakeConnector{connectErr: connectorDomain.RuleError{Rule: "secret", Reason: "Секрет уже принят. Для замены используйте ротацию."}}
        server := newConnectorTestServer(t, fake)
        path := "/app/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/connect"
        form := url.Values{"version": {"1"}, "secret_field": {"api_key"}, "secret_api_key": {"sandbox-form-value"}}
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, connectorFormRequest(http.MethodPost, path, form))
        if response.Code != http.StatusSeeOther {
                t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
        }
        location := response.Header().Get("Location")
        if strings.Contains(location, "error=") {
                t.Fatalf("error message must not travel in the URL: %s", location)
        }
        if location != "/app/tenants/"+testTenantID.String()+"/integrations" {
                t.Fatalf("unexpected redirect: %s", location)
        }
        var flash *http.Cookie
        for _, cookie := range response.Result().Cookies() {
                if cookie.Name == flashCookieName {
                        flash = cookie
                }
        }
        if flash == nil || flash.Value == "" {
                t.Fatalf("error flash cookie missing: %v", response.Result().Cookies())
        }
        if !flash.HttpOnly || flash.MaxAge <= 0 {
                t.Fatalf("flash cookie must be HttpOnly and expiring: %+v", flash)
        }
        // The page renders the message once and clears the cookie.
        page := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/integrations", nil)
        page.AddCookie(flash)
        rendered := httptest.NewRecorder()
        server.Handler().ServeHTTP(rendered, page)
        if rendered.Code != http.StatusOK {
                t.Fatalf("page status=%d", rendered.Code)
        }
        if !strings.Contains(rendered.Body.String(), "Секрет уже принят") {
                t.Fatalf("flash message must be rendered once: %s", rendered.Body.String())
        }
        var cleared bool
        for _, cookie := range rendered.Result().Cookies() {
                if cookie.Name == flashCookieName && cookie.MaxAge < 0 {
                        cleared = true
                }
        }
        if !cleared {
                t.Fatal("flash cookie must be cleared after rendering")
        }
        // A second render without the cookie shows no stale error.
        again := httptest.NewRecorder()
        server.Handler().ServeHTTP(again, httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/integrations", nil))
        if strings.Contains(again.Body.String(), "Секрет уже принят") {
                t.Fatal("flash error must be one-shot")
        }
}

func TestIntegrationsPageIgnoresSpoofedErrorQueryParam(t *testing.T) {
        server := newConnectorTestServer(t, &fakeConnector{})
        request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/integrations?error="+url.QueryEscape("Ваш аккаунт скомпрометирован, нажмите сюда"), nil)
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, request)
        if response.Code != http.StatusOK {
                t.Fatalf("status=%d", response.Code)
        }
        if strings.Contains(response.Body.String(), "скомпрометирован") {
                t.Fatalf("attacker-chosen error text must not render as a system alert: %s", response.Body.String())
        }
}

func TestConnectFormFailedCheckRedirectsToHonestNotice(t *testing.T) {
        fake := &fakeConnector{connectStatus: connectorDomain.StatusFailed}
        server := newConnectorTestServer(t, fake)
        path := "/app/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/connect"
        form := url.Values{"version": {"1"}, "secret_field": {"api_key"}, "secret_api_key": {"not-sandbox-format"}}
        response := httptest.NewRecorder()
        server.Handler().ServeHTTP(response, connectorFormRequest(http.MethodPost, path, form))
        if response.Code != http.StatusSeeOther {
                t.Fatalf("status=%d", response.Code)
        }
        if location := response.Header().Get("Location"); !strings.Contains(location, "check_failed=1") {
                t.Fatalf("failed check must redirect with check_failed, got %s", location)
        }
        page := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/integrations?check_failed=1", nil)
        rendered := httptest.NewRecorder()
        server.Handler().ServeHTTP(rendered, page)
        if rendered.Code != http.StatusOK {
                t.Fatalf("page status=%d", rendered.Code)
        }
        if !strings.Contains(rendered.Body.String(), "Синтетическая проверка не прошла") {
                t.Fatalf("honest failure notice missing: %s", rendered.Body.String())
        }
}

func TestCapabilityStateAPIShowsDeclaredAndProven(t *testing.T) {
        server := newConnectorTestServer(t, &fakeConnector{})
        path := "/v1/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/capabilities"
        r := httptest.NewRequest(http.MethodGet, path, nil)
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusOK {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
        body := w.Body.String()
        for _, want := range []string{`"contract_version":"1.1.0"`, "query.trunk-status", `"verified"`, "command.send-sms", `"unverified"`, "event.inbound-call", "Action Gateway", `"AG-4"`, "Technical Lead + Security"} {
                if !strings.Contains(body, want) {
                        t.Fatalf("capability state missing %q: %s", want, body)
                }
        }
}

func TestVerifyCapabilityAPIRunsProbeAndReportsReplayHeader(t *testing.T) {
        fake := &fakeConnector{verifyReplayed: true}
        server := newConnectorTestServer(t, fake)
        path := "/v1/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/capabilities/query.trunk-status/verify"
        r := httptest.NewRequest(http.MethodPost, path, nil)
        r.Header.Set("Idempotency-Key", "verify-http-key")
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusOK {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
        if w.Header().Get("Idempotency-Replayed") != "true" {
                t.Fatalf("replay header missing: %s", w.Body.String())
        }
        if !strings.Contains(w.Body.String(), `"replayed":true`) || !strings.Contains(w.Body.String(), "query.trunk-status") {
                t.Fatalf("verify response body: %s", w.Body.String())
        }
}

func TestVerifyCapabilityAPIBlockedCapabilityMapsTo400WithBlockReason(t *testing.T) {
        fake := &fakeConnector{verifyErr: connectorDomain.RuleError{Rule: "capability", Reason: "Мутирующая capability: проверка заблокирована до Action Gateway (Epic 3), гейт AG-4, ответственный Technical Lead + Security."}}
        server := newConnectorTestServer(t, fake)
        path := "/v1/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/capabilities/command.send-sms/verify"
        r := httptest.NewRequest(http.MethodPost, path, nil)
        r.Header.Set("Idempotency-Key", "verify-http-key")
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusBadRequest {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
        if !strings.Contains(w.Body.String(), "Action Gateway") {
                t.Fatalf("block reason missing: %s", w.Body.String())
        }
}

func TestIntegrationsPageRendersCapabilityPanelWithBlockAndEvidence(t *testing.T) {
        server := newConnectorTestServer(t, &fakeConnector{withConnection: true})
        r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/integrations", nil)
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusOK {
                t.Fatalf("status=%d", w.Code)
        }
        body := w.Body.String()
        for _, want := range []string{"Возможности (Capability State)", "Заявленные контрактом", "проверено", "Проверка заблокирована: требуется Action Gateway (Epic 3)", "гейт AG-4", "Проверить «Статус транка»", "cap-verify"} {
                if !strings.Contains(body, want) {
                        t.Fatalf("page missing %q", want)
                }
        }
}

func TestVerifyCapabilityFormRedirectsWithOutcomeNotice(t *testing.T) {
        fake := &fakeConnector{}
        server := newConnectorTestServer(t, fake)
        path := "/app/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/capabilities/query.trunk-status/verify"
        form := url.Values{"idempotency_key": {"verify-form-key"}}
        r := connectorFormRequest(http.MethodPost, path, form)
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusSeeOther {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
        location := w.Header().Get("Location")
        if !strings.Contains(location, "capability_verified=1") {
                t.Fatalf("redirect must carry the honest outcome: %s", location)
        }
}

// R1: a replay whose derived state went stale (e.g. the secret was rotated
// between the original verify and the retry) must never read as success.
func TestVerifyCapabilityFormStaleReplayRedirectsHonestly(t *testing.T) {
        fake := &fakeConnector{verifyState: connectorDomain.CapStateStale, verifyReplayed: true}
        server := newConnectorTestServer(t, fake)
        path := "/app/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/capabilities/query.trunk-status/verify"
        form := url.Values{"idempotency_key": {"verify-form-key"}}
        r := connectorFormRequest(http.MethodPost, path, form)
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusSeeOther {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
        location := w.Header().Get("Location")
        if !strings.Contains(location, "capability_stale=1") {
                t.Fatalf("stale replay must carry the stale flag, got: %s", location)
        }
        if strings.Contains(location, "capability_verified=1") {
                t.Fatalf("stale replay must never claim success: %s", location)
        }
}

// R1: any state outside the terminal outcome vocabulary redirects without a
// success flag at all — the card carries the honest state.
func TestVerifyCapabilityFormUnknownStateRedirectsWithoutSuccessClaim(t *testing.T) {
        fake := &fakeConnector{verifyState: connectorDomain.CapStateVerifying}
        server := newConnectorTestServer(t, fake)
        path := "/app/tenants/" + testTenantID.String() + "/connections/" + testCorrelation.String() + "/capabilities/query.trunk-status/verify"
        form := url.Values{"idempotency_key": {"verify-form-key"}}
        r := connectorFormRequest(http.MethodPost, path, form)
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusSeeOther {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
        location := w.Header().Get("Location")
        for _, flag := range []string{"capability_verified", "capability_failed", "capability_unknown", "capability_stale"} {
                if strings.Contains(location, flag) {
                        t.Fatalf("non-terminal state must not claim any outcome flag (%s): %s", flag, location)
                }
        }
}

// R1: the outcome notice is focusable so the post-verify announcement does
// not silently drop focus to the top of the reloaded page.
func TestIntegrationsPageNoticeIsFocusableAfterCapabilityRedirect(t *testing.T) {
        server := newConnectorTestServer(t, &fakeConnector{withConnection: true})
        r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/integrations?capability_stale=1", nil)
        w := httptest.NewRecorder()
        server.Handler().ServeHTTP(w, r)
        if w.Code != http.StatusOK {
                t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
        }
        body := w.Body.String()
        for _, want := range []string{`id="page-notice"`, `tabindex="-1"`, "Доказательство устарело", "page-notice').focus()"} {
                if !strings.Contains(body, want) {
                        t.Fatalf("focusable notice missing %q", want)
                }
        }
}
