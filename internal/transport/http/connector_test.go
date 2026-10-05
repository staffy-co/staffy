package httptransport

import (
        "context"
        "io"
        "log/slog"
        "net/http"
        "net/http/httptest"
        "strings"
        "testing"

        connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        "github.com/artemiosu/staffy/internal/identity/api"
        "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
)

type fakeConnector struct {
        catalog      []connectorDomain.ConnectorDefinition
        createErr    error
        connectErr   error
        connectCalls int
        lastConnect  *connectorAPI.ConnectCommand
        lastCreate   *connectorAPI.CreateCommand
        revokeErr    error
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
        return nil, nil
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

func (f *fakeConnector) Connect(_ context.Context, c connectorAPI.ConnectCommand) (connectorDomain.Connection, error) {
        f.connectCalls++
        f.lastConnect = &c
        if f.connectErr != nil {
                return connectorDomain.Connection{}, f.connectErr
        }
        return connectorDomain.Connection{TenantID: c.TenantID, ConnectionID: c.ConnectionID, Status: connectorDomain.StatusConnectedUnverified, AuthState: connectorDomain.CheckVerified, Transport: connectorDomain.CheckVerified, SecretID: testCorrelation, SecretHint: "abcdef012345", SecretActive: true, Version: 2}, nil
}

func (f *fakeConnector) Rotate(context.Context, connectorAPI.ConnectCommand) (connectorDomain.Connection, error) {
        return connectorDomain.Connection{}, connectorDomain.ErrInvalid
}

func (f *fakeConnector) Revoke(context.Context, connectorAPI.RevokeCommand) (connectorDomain.Connection, error) {
        if f.revokeErr != nil {
                return connectorDomain.Connection{}, f.revokeErr
        }
        return connectorDomain.Connection{Status: connectorDomain.StatusRevoked}, nil
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
