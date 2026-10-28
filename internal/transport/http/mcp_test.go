package httptransport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/mcp"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// fakeMCPSurface is the transport fake of the MCP endpoint: it answers a
// canned outcome and records the handed material (the transport tests
// pin the HTTP mapping, the ladder order and the budgets — the protocol
// pipeline is the service's own unit truth).
type fakeMCPSurface struct {
	outcome    mcp.Outcome
	err        error
	lastBody   []byte
	lastActor  id.UUID
	lastClaims mcp.HeaderClaims
}

// The compile-time proof the fake satisfies the transport contract.
var _ mcpService = (*fakeMCPSurface)(nil)

func (f *fakeMCPSurface) Handle(_ context.Context, actor domain.Principal, _ id.UUID, body []byte, claims mcp.HeaderClaims) (mcp.Outcome, error) {
	f.lastBody = body
	f.lastActor = actor.ID
	f.lastClaims = claims
	if f.err != nil {
		return mcp.Outcome{}, f.err
	}
	return f.outcome, nil
}

func newMCPTestServer(t *testing.T, fake *fakeMCPSurface, resolver fixedResolver) http.Handler {
	t.Helper()
	server, err := New(nil, resolver, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if fake != nil {
		server.BindMCP(fake)
	}
	return server.Handler()
}

func mcpTenantID(t *testing.T) id.UUID {
	t.Helper()
	tenant, err := id.Parse("01994f64-6c00-7000-8000-0000000000a1")
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	return tenant
}

func mcpPrincipal(t *testing.T) domain.Principal {
	t.Helper()
	principal, err := id.Parse("01994f64-6c00-7000-8000-0000000000c1")
	if err != nil {
		t.Fatalf("parse principal: %v", err)
	}
	return domain.Principal{ID: principal}
}

// mcpRequest builds one POST of the surface with the coherent routing
// headers of the given method.
func mcpRequest(t *testing.T, handler http.Handler, method, protocolVersion, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/tenants/"+mcpTenantID(t).String()+"/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if method != "" {
		request.Header.Set(mcp.HeaderMethod, method)
	}
	if protocolVersion != "" {
		request.Header.Set(mcp.HeaderProtocolVersion, protocolVersion)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// TestMCPEndpointLadderBeforeProtocolMaterial: the resolver 401 and the
// malformed tenant 404 answer with the platform problem contract BEFORE
// the service (and any protocol material) runs — the fake records the
// hand-off, and the refusal legs prove it never happened.
func TestMCPEndpointLadderBeforeProtocolMaterial(t *testing.T) {
	fake := &fakeMCPSurface{outcome: mcp.Outcome{Status: 200}}
	handler := newMCPTestServer(t, fake, fixedResolver{principal: mcpPrincipal(t)})

	// The identity refusal comes first.
	handler = newMCPTestServer(t, fake, fixedResolver{err: domain.ErrNotFound})
	recorder := mcpRequest(t, handler, mcp.MethodServerDiscover, mcp.ProtocolVersion, `{"jsonrpc":"2.0"}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("отсутствие личности обязано получать 401, получено %d", recorder.Code)
	}
	if fake.lastBody != nil {
		t.Fatal("сервис не должен был получить тело без личности")
	}
	if !strings.Contains(recorder.Body.String(), "identity_required") {
		t.Fatalf("401 обязан нести problem-контракт платформы, получено %s", recorder.Body.String())
	}

	// The malformed tenant path is the honest 404 before the service.
	handler = newMCPTestServer(t, fake, fixedResolver{principal: mcpPrincipal(t)})
	request := httptest.NewRequest(http.MethodPost, "/v1/tenants/не-uuid/mcp", strings.NewReader(`{"jsonrpc":"2.0"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("малформед-тенент обязан получать 404, получено %d", recorder.Code)
	}
	if fake.lastBody != nil {
		t.Fatal("сервис не должен был получить тело с малформед-тенентом")
	}

	// The scoped principal reaches the service: the canned outcome maps.
	fake = &fakeMCPSurface{outcome: mcp.Outcome{Status: 200, Body: &mcp.Response{JSONRPC: "2.0"}}}
	handler = newMCPTestServer(t, fake, fixedResolver{principal: mcpPrincipal(t)})
	recorder = mcpRequest(t, handler, mcp.MethodServerDiscover, mcp.ProtocolVersion, `{"jsonrpc":"2.0","id":1,"method":"server/discover"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("валидный запрос обязан доходить до сервиса, получено %d", recorder.Code)
	}
	if string(fake.lastBody) != `{"jsonrpc":"2.0","id":1,"method":"server/discover"}` {
		t.Fatalf("сервис обязан получить тело дословно, получено %s", fake.lastBody)
	}
	if fake.lastClaims.Method != mcp.MethodServerDiscover || fake.lastClaims.ProtocolVersion != mcp.ProtocolVersion {
		t.Fatalf("сервис обязан получить заявки маршрутизации, получено %+v", fake.lastClaims)
	}
}

// TestMCPEndpointTransportBudgets: the declared transport budgets answer
// with the problem contract before the service — the non-JSON content
// type is the 415, the body past 256 KiB is the 413.
func TestMCPEndpointTransportBudgets(t *testing.T) {
	fake := &fakeMCPSurface{outcome: mcp.Outcome{Status: 200}}
	handler := newMCPTestServer(t, fake, fixedResolver{principal: mcpPrincipal(t)})

	// The content-type budget.
	request := httptest.NewRequest(http.MethodPost, "/v1/tenants/"+mcpTenantID(t).String()+"/mcp", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("не-JSON тип обязан получать 415, получено %d", recorder.Code)
	}
	if fake.lastBody != nil {
		t.Fatal("сервис не должен был получить тело с неверным типом")
	}

	// The body budget: 256 KiB + one byte.
	request = httptest.NewRequest(http.MethodPost, "/v1/tenants/"+mcpTenantID(t).String()+"/mcp",
		io.LimitReader(strings.NewReader(strings.Repeat("a", 256<<10+1)), 256<<10+2))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("тело поверх бюджета обязано получать 413, получено %d", recorder.Code)
	}
	if fake.lastBody != nil {
		t.Fatal("сервис не должен был получить тело поверх бюджета")
	}
}

// TestMCPMethodNotAllowed: the GET and DELETE of the endpoint are the
// honest 405 — the pinned revision has no streams and no sessions.
func TestMCPMethodNotAllowed(t *testing.T) {
	handler := newMCPTestServer(t, &fakeMCPSurface{}, fixedResolver{principal: mcpPrincipal(t)})
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		request := httptest.NewRequest(method, "/v1/tenants/"+mcpTenantID(t).String()+"/mcp", nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s обязан получать 405, получено %d", method, recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), "method_not_allowed") {
			t.Fatalf("405 обязан нести problem-контракт, получено %s", recorder.Body.String())
		}
	}
}

// TestMCPEndpointProblemMapping: the service refusals map onto the
// platform problem contract — the ladder 404/403 with the scope words and
// the window 429 with the honest Retry-After.
func TestMCPEndpointProblemMapping(t *testing.T) {
	forbidden := &fakeMCPSurface{err: mcp.ErrForbidden}
	handler := newMCPTestServer(t, forbidden, fixedResolver{principal: mcpPrincipal(t)})
	recorder := mcpRequest(t, handler, mcp.MethodServerDiscover, mcp.ProtocolVersion, `{}`)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("отказ скоупов обязан получать 403, получено %d", recorder.Code)
	}

	windowRefusal := &fakeMCPSurface{err: &mcp.TooManyRequestsError{RetryAfter: 90e9}}
	handler = newMCPTestServer(t, windowRefusal, fixedResolver{principal: mcpPrincipal(t)})
	recorder = mcpRequest(t, handler, mcp.MethodServerDiscover, mcp.ProtocolVersion, `{}`)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("отказ окна обязан получать 429, получено %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "90" {
		t.Fatalf("429 обязан нести Retry-After, получено %q", recorder.Header().Get("Retry-After"))
	}
}
