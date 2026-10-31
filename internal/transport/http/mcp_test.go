package httptransport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/adapters/toolssource"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/mcp"
	"github.com/staffy-co/staffy/internal/platform/id"
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

// mcpTransportMembership is the compact ladder fake of the full-stack
// probe: the fixture principal holds every scope, the second actor is
// the honest member refusal, every other actor or tenant is the honest
// not-found.
type mcpTransportMembership struct{}

func (mcpTransportMembership) ListMembers(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error) {
	return nil, nil
}
func (mcpTransportMembership) InviteMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, string, error) {
	return domain.Membership{}, "", domain.ErrStepUpRequired
}
func (mcpTransportMembership) AcceptInvitation(context.Context, domain.Principal, string, id.UUID) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrNotFound
}
func (mcpTransportMembership) ChangeMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrStepUpRequired
}
func (mcpTransportMembership) RevokeMember(context.Context, identityAPI.MembershipCommand) error {
	return domain.ErrStepUpRequired
}
func (mcpTransportMembership) Authorize(_ context.Context, actor domain.Principal, tenant id.UUID, _ domain.Scope) error {
	if !actor.ID.Valid() || tenant != mcpTransportTenantID {
		return domain.ErrNotFound
	}
	return nil
}

// mcpTransportTenantID is the tenant of the transport membership fake
// (the fixture tenant of the probe requests).
var mcpTransportTenantID = id.MustParse("01994f64-6c00-7000-8000-0000000000a1")

// mcpEmptyAdapters is the AG-4 production fact of the full-stack probe.
type mcpEmptyAdapters struct{}

func (mcpEmptyAdapters) Adapter(string, string) (mcp.Adapter, bool) { return nil, false }

// mcpLiveGate answers the live dependency fact of the full-stack probe
// (the self-contained registry contracts never ask it).
type mcpLiveGate struct{}

func (mcpLiveGate) Live(context.Context, domain.Principal, id.UUID, string) (bool, string, error) {
	return true, "verified", nil
}

// newMCPFullStackServer wires the REAL mcp.Service through the
// transport with the production registry source (the live 7-2
// contracts, all honestly not_implemented — the AG-4 world) and the
// empty adapter registry.
func newMCPFullStackServer(t *testing.T) http.Handler {
	t.Helper()
	service := mcp.New(mcpTransportMembership{}, toolssource.New(), mcpEmptyAdapters{}, mcpLiveGate{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	server, err := New(nil, fixedResolver{principal: mcpPrincipal(t)}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindMCP(service)
	return server.Handler()
}

// TestMCPEndpointFullStackProtocolMatrix walks the protocol matrix of
// the REAL service through the wire: the discover answer, the honest
// empty list of the AG-4 world, the batch refusal of the old era, the
// parse error, the notification receipt, the routing-claim coherence
// and the foreign revision — every answer in the single JSON-RPC form.
func TestMCPEndpointFullStackProtocolMatrix(t *testing.T) {
	handler := newMCPFullStackServer(t)

	// The discover answer: the exact supported list and the server
	// identity.
	recorder := mcpRequest(t, handler, mcp.MethodServerDiscover, mcp.ProtocolVersion,
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","_meta":{"protocolVersion":"`+mcp.ProtocolVersion+`"}}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("discover обязан отвечать 200, получено %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, word := range []string{`"protocolVersions":["` + mcp.ProtocolVersion + `"]`, `"listChanged":false`, `"serverInfo"`, `"staffy"`, `"0.1.0"`} {
		if !strings.Contains(body, word) {
			t.Fatalf("discover-ответ обязан нести %q, получено %s", word, body)
		}
	}

	// The honest empty list of the AG-4 world: the live registry
	// contracts are all not_implemented, the adapter registry is empty.
	recorder = mcpRequest(t, handler, mcp.MethodToolsList, mcp.ProtocolVersion,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","_meta":{"protocolVersion":"`+mcp.ProtocolVersion+`"}}`)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"tools":[]`) {
		t.Fatalf("honest empty list обязателен, получено %d: %s", recorder.Code, recorder.Body.String())
	}

	// The batch of the old era: the named 400 with the JSON-RPC error
	// body.
	recorder = mcpRequest(t, handler, mcp.MethodToolsList, mcp.ProtocolVersion, `[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]`)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "одно JSON-RPC сообщение") {
		t.Fatalf("batch обязан получать 400 с поимённым отказом, получено %d: %s", recorder.Code, recorder.Body.String())
	}

	// The parse error of the malformed body.
	recorder = mcpRequest(t, handler, mcp.MethodToolsList, mcp.ProtocolVersion, `{`)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "-32700") {
		t.Fatalf("малформед-JSON обязан получать 400 parse error, получено %d: %s", recorder.Code, recorder.Body.String())
	}

	// The notification receipt: the bare 202 without a body.
	recorder = mcpRequest(t, handler, mcp.MethodToolsList, mcp.ProtocolVersion,
		`{"jsonrpc":"2.0","method":"tools/list","_meta":{"protocolVersion":"`+mcp.ProtocolVersion+`"}}`)
	if recorder.Code != http.StatusAccepted || recorder.Body.Len() != 0 {
		t.Fatalf("уведомление обязано получать пустой 202, получено %d: %q", recorder.Code, recorder.Body.String())
	}

	// The routing-claim coherence: the method header desync is the 400.
	recorder = mcpRequest(t, handler, mcp.MethodToolsList, mcp.ProtocolVersion,
		`{"jsonrpc":"2.0","id":3,"method":"server/discover","_meta":{"protocolVersion":"`+mcp.ProtocolVersion+`"}}`)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "Mcp-Method") {
		t.Fatalf("рассинхрон метода обязан получать 400, получено %d: %s", recorder.Code, recorder.Body.String())
	}

	// The foreign revision claim: the honest 200 error with the list.
	recorder = mcpRequest(t, handler, mcp.MethodServerDiscover, "2025-11-25",
		`{"jsonrpc":"2.0","id":4,"method":"server/discover","_meta":{"protocolVersion":"2025-11-25"}}`)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "UnsupportedProtocolVersionError") || !strings.Contains(recorder.Body.String(), `"supportedVersions":["`+mcp.ProtocolVersion+`"]`) {
		t.Fatalf("чужая ревизия обязана получать 200 с ошибкой и списком, получено %d: %s", recorder.Code, recorder.Body.String())
	}

	// The unknown method: the honest 200 method-not-found.
	recorder = mcpRequest(t, handler, "resources/list", mcp.ProtocolVersion,
		`{"jsonrpc":"2.0","id":5,"method":"resources/list","_meta":{"protocolVersion":"`+mcp.ProtocolVersion+`"}}`)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "-32601") {
		t.Fatalf("неизвестный метод обязан получать -32601, получено %d: %s", recorder.Code, recorder.Body.String())
	}
}

// TestMCPEndpointLadderWordsBeforeProtocol: the member refusal of the
// REAL ladder answers the 403 problem with the scope words BEFORE a
// single JSON-RPC byte — the full-stack walk of the honest order.
func TestMCPEndpointLadderWordsBeforeProtocol(t *testing.T) {
	server, err := New(nil, fixedResolver{principal: mcpPrincipal(t)}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	// The member-without-scopes membership: every actor of the probe is
	// the honest member refusal.
	server.BindMCP(mcp.New(mcpMemberRefusal{}, toolssource.New(), mcpEmptyAdapters{}, mcpLiveGate{}, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	handler := server.Handler()
	recorder := mcpRequest(t, handler, mcp.MethodServerDiscover, mcp.ProtocolVersion,
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","_meta":{"protocolVersion":"`+mcp.ProtocolVersion+`"}}`)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("участник без скоупов обязан получать 403, получено %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "integrations или configuration") {
		t.Fatalf("403 обязан нести слова скоупов, получено %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "jsonrpc") {
		t.Fatalf("ни единого байта JSON-RPC до лестницы, получено %s", recorder.Body.String())
	}
}

// mcpMemberRefusal is the member-without-scopes membership of the
// ladder-words probe.
type mcpMemberRefusal struct{}

func (mcpMemberRefusal) ListMembers(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error) {
	return nil, nil
}
func (mcpMemberRefusal) InviteMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, string, error) {
	return domain.Membership{}, "", domain.ErrStepUpRequired
}
func (mcpMemberRefusal) AcceptInvitation(context.Context, domain.Principal, string, id.UUID) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrNotFound
}
func (mcpMemberRefusal) ChangeMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrStepUpRequired
}
func (mcpMemberRefusal) RevokeMember(context.Context, identityAPI.MembershipCommand) error {
	return domain.ErrStepUpRequired
}
func (mcpMemberRefusal) Authorize(_ context.Context, actor domain.Principal, tenant id.UUID, _ domain.Scope) error {
	if !actor.ID.Valid() || !tenant.Valid() {
		return domain.ErrNotFound
	}
	return domain.ErrForbidden
}
