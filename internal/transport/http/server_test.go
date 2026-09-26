package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

var (
	testPrincipalID = id.MustParse("01994f64-6c00-7000-8000-000000000001")
	testTenantID    = id.MustParse("01994f64-6c00-7000-8000-000000000002")
	testCorrelation = id.MustParse("01994f64-6c00-7000-8000-000000000003")
)

type fakeIdentity struct {
	create    func(api.CreateTenantCommand) (api.CreateTenantResult, error)
	workspace func(id.UUID) (domain.Workspace, error)
}

func (f fakeIdentity) CreateTenant(_ context.Context, c api.CreateTenantCommand) (api.CreateTenantResult, error) {
	return f.create(c)
}
func (f fakeIdentity) Workspace(_ context.Context, _ domain.Principal, tenantID id.UUID) (domain.Workspace, error) {
	return f.workspace(tenantID)
}

type fixedResolver struct {
	principal domain.Principal
	err       error
}

func (r fixedResolver) Resolve(context.Context) (domain.Principal, error) { return r.principal, r.err }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Unix(1, 0) }

type fixedIDs struct{}

func (fixedIDs) New(time.Time) (id.UUID, error) { return testCorrelation, nil }

func newTestServer(t *testing.T, identity api.Service, resolver fixedResolver, logOutput io.Writer) http.Handler {
	t.Helper()
	server, err := New(identity, resolver, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(logOutput, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return server.Handler()
}

func TestCreateTenantAPI(t *testing.T) {
	identity := fakeIdentity{
		create: func(command api.CreateTenantCommand) (api.CreateTenantResult, error) {
			if command.Name != "Montagio" || command.IdempotencyKey != "request-0001" || command.CorrelationID != testCorrelation {
				t.Fatalf("unexpected command: %#v", command)
			}
			return api.CreateTenantResult{Tenant: domain.Tenant{ID: testTenantID, Name: command.Name}, Role: "Owner"}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) { return domain.Workspace{}, nil },
	}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, CanCreateTenant: true}}, io.Discard)
	request := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{"name":"Montagio"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request-0001")
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	request.Header.Set("X-Tenant-ID", "01994f64-6c00-7000-8000-00000000dead")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") != "/app/tenants/"+testTenantID.String() {
		t.Fatalf("location = %q", response.Header().Get("Location"))
	}
	var body struct {
		TenantID     id.UUID `json:"tenant_id"`
		Name         string  `json:"name"`
		Role         string  `json:"role"`
		WorkspaceURL string  `json:"workspace_url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.TenantID != testTenantID || body.Name != "Montagio" || body.Role != "Owner" || body.WorkspaceURL != "/app/tenants/"+testTenantID.String() {
		t.Fatalf("body = %#v", body)
	}
}

func TestCreateTenantAPIReplayHeader(t *testing.T) {
	identity := fakeIdentity{
		create: func(command api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{Tenant: domain.Tenant{ID: testTenantID, Name: command.Name}, Role: "Owner", Replayed: true}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) { return domain.Workspace{}, nil },
	}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, CanCreateTenant: true}}, io.Discard)
	request := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{"name":"Montagio"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request-0001")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("status = %d, replay = %q", response.Code, response.Header().Get("Idempotency-Replayed"))
	}
}

func TestCreateTenantAPIConflictIsSafe(t *testing.T) {
	identity := fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{}, domain.ErrIdempotencyConflict
		},
		workspace: func(id.UUID) (domain.Workspace, error) { return domain.Workspace{}, nil },
	}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, CanCreateTenant: true}}, io.Discard)
	request := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{"name":"Other"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request-0001")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"idempotency_conflict"`) || !strings.Contains(response.Body.String(), `"correlation_id":"`+testCorrelation.String()+`"`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCreateTenantFormConflictReturnsHTMLWithFreshKey(t *testing.T) {
	identity := fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{}, domain.ErrIdempotencyConflict
		},
		workspace: func(id.UUID) (domain.Workspace, error) { return domain.Workspace{}, nil },
	}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, CanCreateTenant: true}}, io.Discard)
	form := url.Values{"name": {"Montagio"}, "idempotency_key": {"request-0001"}}
	request := httptest.NewRequest(http.MethodPost, "/app/tenants", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("status = %d, content-type = %q", response.Code, response.Header().Get("Content-Type"))
	}
	if !strings.Contains(response.Body.String(), `value="`+testCorrelation.String()+`"`) {
		t.Fatalf("fresh idempotency key missing: %s", response.Body.String())
	}
}

func TestCreateTenantAPIRejectsPayloadTenantScope(t *testing.T) {
	identity := fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			t.Fatal("service must not receive caller-controlled tenant scope")
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) { return domain.Workspace{}, nil },
	}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, CanCreateTenant: true}}, io.Discard)
	request := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{"name":"Montagio","tenant_id":"01994f64-6c00-7000-8000-00000000dead"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request-0001")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCreateTenantAPIRejectsWrongMediaTypeAndMultipleDocuments(t *testing.T) {
	identity := fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			t.Fatal("invalid request must not reach service")
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) { return domain.Workspace{}, nil },
	}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, CanCreateTenant: true}}, io.Discard)
	for name, testCase := range map[string]struct {
		contentType string
		body        string
		wantStatus  int
	}{
		"media type": {contentType: "text/plain", body: `{"name":"Montagio"}`, wantStatus: http.StatusUnsupportedMediaType},
		"two values": {contentType: "application/json", body: `{"name":"Montagio"}{"name":"Other"}`, wantStatus: http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(testCase.body))
			request.Header.Set("Content-Type", testCase.contentType)
			request.Header.Set("Idempotency-Key", "request-0001")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestWorkspaceShowsHonestEmptyState(t *testing.T) {
	identity := fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) { return api.CreateTenantResult{}, nil },
		workspace: func(tenantID id.UUID) (domain.Workspace, error) {
			if tenantID != testTenantID {
				return domain.Workspace{}, domain.ErrNotFound
			}
			return domain.Workspace{Tenant: domain.Tenant{ID: tenantID, Name: "Магазин Олега"}, Role: "owner"}, nil
		},
	}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID}}, io.Discard)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String(), nil))
	body := response.Body.String()
	for _, expected := range []string{"Магазин Олега", "Роль: Owner", "Начните с ИИ-сотрудника", "Настройка ИИ-сотрудника доступна"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body does not contain %q", expected)
		}
	}
}

func TestForeignWorkspaceDoesNotLeakAndAuditOmitsPayload(t *testing.T) {
	var logs bytes.Buffer
	identity := fakeIdentity{
		create:    func(api.CreateTenantCommand) (api.CreateTenantResult, error) { return api.CreateTenantResult{}, nil },
		workspace: func(id.UUID) (domain.Workspace, error) { return domain.Workspace{}, domain.ErrNotFound },
	}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID}}, &logs)
	foreignID := "01994f64-6c00-7000-8000-00000000dead"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/app/tenants/"+foreignID, nil))
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), foreignID) {
		t.Fatalf("unsafe response: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(logs.String(), foreignID) || !strings.Contains(logs.String(), "tenant_access_denied") {
		t.Fatalf("unsafe audit: %s", logs.String())
	}
}

func TestWorkspaceOperationalFailureIsRetryableAndNotAnAccessDenial(t *testing.T) {
	var logs bytes.Buffer
	identity := fakeIdentity{
		create:    func(api.CreateTenantCommand) (api.CreateTenantResult, error) { return api.CreateTenantResult{}, nil },
		workspace: func(id.UUID) (domain.Workspace, error) { return domain.Workspace{}, errors.New("database unavailable") },
	}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID}}, &logs)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String(), nil))
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"retryable":true`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(logs.String(), "tenant_access_denied") || !strings.Contains(logs.String(), "workspace_read_failed") {
		t.Fatalf("logs = %s", logs.String())
	}
}

func TestMissingPrincipalFailsClosed(t *testing.T) {
	identity := fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			t.Fatal("service must not be called")
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) {
			t.Fatal("service must not be called")
			return domain.Workspace{}, nil
		},
	}
	handler := newTestServer(t, identity, fixedResolver{err: errors.New("no identity")}, io.Discard)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/app", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}
