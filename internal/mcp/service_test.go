package mcp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/tools"
)

// membershipFake is the ladder fake of the fixture world (the
// toolsMembership precedent of the journeys): the owner holds every
// scope, the review member is the honest member refusal, the operations
// member exists without the surface scopes, every other actor or tenant
// is the honest not-found.
type membershipFake struct{}

func (membershipFake) ListMembers(context.Context, identityDomain.Principal, id.UUID) ([]identityDomain.Membership, error) {
	return nil, nil
}
func (membershipFake) InviteMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, string, error) {
	return identityDomain.Membership{}, "", identityDomain.ErrStepUpRequired
}
func (membershipFake) AcceptInvitation(context.Context, identityDomain.Principal, string, id.UUID) (identityDomain.Membership, error) {
	return identityDomain.Membership{}, identityDomain.ErrNotFound
}
func (membershipFake) ChangeMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, error) {
	return identityDomain.Membership{}, identityDomain.ErrStepUpRequired
}
func (membershipFake) RevokeMember(context.Context, identityAPI.MembershipCommand) error {
	return identityDomain.ErrStepUpRequired
}
func (membershipFake) Authorize(_ context.Context, actor identityDomain.Principal, tenant id.UUID, scope identityDomain.Scope) error {
	if !actor.ID.Valid() || tenant != testTenantID {
		return identityDomain.ErrNotFound
	}
	switch actor.ID {
	case testOwnerID:
		return nil
	case testReviewID:
		if scope == identityDomain.Review || scope == identityDomain.Audit {
			return nil
		}
		return identityDomain.ErrForbidden
	case testOperationsID:
		return identityDomain.ErrForbidden
	default:
		return identityDomain.ErrNotFound
	}
}

// emptySource is the registry source fake of the pipeline tests (the
// real registry source is the production adapter; the method machines of
// step-02 carry their own fixture source).
type emptySource struct{}

func (emptySource) List() []tools.ToolContract { return nil }
func (emptySource) Get(string, string) (tools.ToolContract, bool) {
	return tools.ToolContract{}, false
}

// emptyAdapters is the honest AG-4 registry of the pipeline tests.
type emptyAdapters struct{}

func (emptyAdapters) Adapter(string, string) (Adapter, bool) { return nil, false }

// gateFake carries the scripted live fact of the dependency gate.
type gateFake struct {
	live bool
	word string
	err  error
}

func (g gateFake) Live(context.Context, identityDomain.Principal, id.UUID, string) (bool, string, error) {
	return g.live, g.word, g.err
}

// staticClock is the frozen time source of the window tests.
type staticClock struct{ now time.Time }

func (c staticClock) Now() time.Time { return c.now }

var (
	testOwnerID       = id.MustParse("01994f64-6c00-7000-8000-0000000000a1")
	testReviewID      = id.MustParse("01994f64-6c00-7000-8000-0000000000a2")
	testOperationsID  = id.MustParse("01994f64-6c00-7000-8000-0000000000a3")
	testTenantID      = id.MustParse("01994f64-6c00-7000-8000-0000000000b1")
	testForeignTenant = id.MustParse("01994f64-6c00-7000-8000-0000000000b2")
)

func newTestService() *Service {
	return New(membershipFake{}, emptySource{}, emptyAdapters{}, gateFake{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// discoverBody builds one coherent request body of the given method.
func discoverBody(method string) []byte {
	return []byte(`{"jsonrpc":"2.0","id":7,"method":"` + method + `","_meta":{"protocolVersion":"` + ProtocolVersion + `"}}`)
}

// coherentClaims builds the routing claims that agree with discoverBody.
func coherentClaims(method string) HeaderClaims {
	return HeaderClaims{Method: method, ProtocolVersion: ProtocolVersion}
}

// TestHandleLadderRunsFirst: the membership ladder answers BEFORE any
// protocol material — the non-member and the foreign tenant are the
// honest not-found, the member without the scopes is the forbidden
// carrying the scope words, and the scoped member reaches the protocol
// pipeline (the malformed body proves the ladder let the call through).
func TestHandleLadderRunsFirst(t *testing.T) {
	service := newTestService()
	owner := identityDomain.Principal{ID: testOwnerID}
	review := identityDomain.Principal{ID: testReviewID}
	operations := identityDomain.Principal{ID: testOperationsID}

	if _, err := service.Handle(context.Background(), owner, testForeignTenant, discoverBody(MethodServerDiscover), coherentClaims(MethodServerDiscover)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой тенент обязан получать честный not-found, получено %v", err)
	}
	if _, err := service.Handle(context.Background(), review, testTenantID, discoverBody(MethodServerDiscover), coherentClaims(MethodServerDiscover)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("участник без скоупов обязан получать отказ, получено %v", err)
	} else if !strings.Contains(err.Error(), "integrations или configuration") {
		t.Fatalf("отказ обязан нести слова скоупов, получено %q", err.Error())
	}
	if _, err := service.Handle(context.Background(), operations, testTenantID, discoverBody(MethodServerDiscover), coherentClaims(MethodServerDiscover)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("operations-участник обязан получать отказ скоупов, получено %v", err)
	}
	// The scoped member reaches the pipeline: the malformed body answers
	// the 400 of the parse, never a ladder refusal.
	outcome, err := service.Handle(context.Background(), owner, testTenantID, []byte("не JSON"), coherentClaims(MethodServerDiscover))
	if err != nil {
		t.Fatalf("лестница не должна была отказать владельцу: %v", err)
	}
	if outcome.Status != 400 || outcome.Body == nil || outcome.Body.Error == nil {
		t.Fatalf("малформед-тело обязано получать 400 с JSON-RPC ошибкой, получено %+v", outcome)
	}
}

// TestHandleRateWindowHoldsTheBudget: the declared fixed window of the
// principal refuses the burst past the limit with the honest retry hint.
func TestHandleRateWindowHoldsTheBudget(t *testing.T) {
	clock := staticClock{now: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
	service := newTestService().WithRateWindow(2, time.Minute, clock)
	owner := identityDomain.Principal{ID: testOwnerID}
	for call := 1; call <= 2; call++ {
		if _, err := service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodServerDiscover), coherentClaims(MethodServerDiscover)); err != nil {
			t.Fatalf("вызов %d внутри окна не должен отказывать: %v", call, err)
		}
	}
	_, err := service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodServerDiscover), coherentClaims(MethodServerDiscover))
	var tooMany *TooManyRequestsError
	if !errors.As(err, &tooMany) {
		t.Fatalf("третий вызов обязан получать отказ окна, получено %v", err)
	}
	if tooMany.RetryAfter <= 0 {
		t.Fatalf("отказ окна обязан нести подсказку повтора, получено %s", tooMany.RetryAfter)
	}
	// The window is per principal: the second actor starts fresh.
	if _, err := service.Handle(context.Background(), identityDomain.Principal{ID: testReviewID}, testTenantID, []byte("x"), coherentClaims(MethodServerDiscover)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("второй принципал обязан получать собственный ответ лестницы, получено %v", err)
	}
}

// TestHandlePipelineProtocolAnswers walks the protocol answers of the
// pipeline: the parse refusal, the routing-claim coherence, the version
// negotiation, the notification receipt and the dispatch.
func TestHandlePipelineProtocolAnswers(t *testing.T) {
	service := newTestService()
	owner := identityDomain.Principal{ID: testOwnerID}

	// The routing-claim coherence: the method header must agree.
	outcome, err := service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodServerDiscover), HeaderClaims{Method: MethodToolsList, ProtocolVersion: ProtocolVersion})
	if err != nil {
		t.Fatalf("живой прогон не должен был упасть раньше отказа когерентности: %v", err)
	}
	if outcome.Status != 400 || !strings.Contains(outcome.Body.Error.Message, "Mcp-Method") {
		t.Fatalf("рассинхрон метода обязан получать 400 с поимённой причиной, получено %+v", outcome)
	}

	// The version claims must travel in both places and agree.
	outcome, err = service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodServerDiscover), HeaderClaims{Method: MethodServerDiscover})
	if err != nil || outcome.Status != 400 || !strings.Contains(outcome.Body.Error.Message, "MCP-Protocol-Version") {
		t.Fatalf("отсутствие заявки ревизии в заголовке обязано получать 400, получено %+v err=%v", outcome, err)
	}
	mismatched := HeaderClaims{Method: MethodServerDiscover, ProtocolVersion: "2025-06-18"}
	outcome, err = service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodServerDiscover), mismatched)
	if err != nil || outcome.Status != 400 || !strings.Contains(outcome.Body.Error.Message, "расходится") {
		t.Fatalf("рассинхрон заявки ревизии обязан получать 400, получено %+v err=%v", outcome, err)
	}

	// The foreign revision claim (coherent) is the honest 200 error with
	// the supported list.
	foreignBody := []byte(`{"jsonrpc":"2.0","id":7,"method":"server/discover","_meta":{"protocolVersion":"2025-11-25"}}`)
	foreignClaims := HeaderClaims{Method: MethodServerDiscover, ProtocolVersion: "2025-11-25"}
	outcome, err = service.Handle(context.Background(), owner, testTenantID, foreignBody, foreignClaims)
	if err != nil || outcome.Status != 200 {
		t.Fatalf("чужая ревизия обязана получать 200 с JSON-RPC ошибкой, получено %+v err=%v", outcome, err)
	}
	if !strings.Contains(outcome.Body.Error.Message, "UnsupportedProtocolVersionError") {
		t.Fatalf("ошибка обязана называть тип, получено %+v", outcome.Body.Error)
	}

	// The notification receipt: the bare 202, no body.
	notification := []byte(`{"jsonrpc":"2.0","method":"tools/list","_meta":{"protocolVersion":"` + ProtocolVersion + `"}}`)
	outcome, err = service.Handle(context.Background(), owner, testTenantID, notification, coherentClaims(MethodToolsList))
	if err != nil || outcome.Status != 202 || outcome.Body != nil {
		t.Fatalf("уведомление обязано получать пустой 202, получено %+v err=%v", outcome, err)
	}

	// The unknown method: the honest 200 method-not-found.
	unknown := []byte(`{"jsonrpc":"2.0","id":"u","method":"resources/list","_meta":{"protocolVersion":"` + ProtocolVersion + `"}}`)
	outcome, err = service.Handle(context.Background(), owner, testTenantID, unknown, HeaderClaims{Method: "resources/list", ProtocolVersion: ProtocolVersion})
	if err != nil || outcome.Status != 200 || outcome.Body.Error.Code != CodeMethodNotFound {
		t.Fatalf("неизвестный метод обязан получать -32601, получено %+v err=%v", outcome, err)
	}

	// The id echoes in every protocol answer.
	if string(outcome.Body.ID) != `"u"` {
		t.Fatalf("id обязан возвращаться в ответе, получено %s", outcome.Body.ID)
	}
}

// TestHandleNotificationForeignVersionStaysSilent: a notification of a
// foreign revision receives the bare 202 — there is no id to address an
// error to, and the stateless core honestly did nothing.
func TestHandleNotificationForeignVersionStaysSilent(t *testing.T) {
	service := newTestService()
	owner := identityDomain.Principal{ID: testOwnerID}
	notification := []byte(`{"jsonrpc":"2.0","method":"server/discover","_meta":{"protocolVersion":"2025-06-18"}}`)
	outcome, err := service.Handle(context.Background(), owner, testTenantID, notification, HeaderClaims{Method: MethodServerDiscover, ProtocolVersion: "2025-06-18"})
	if err != nil || outcome.Status != 202 || outcome.Body != nil {
		t.Fatalf("уведомление чужой ревизии обязано получать пустой 202, получено %+v err=%v", outcome, err)
	}
}

// TestScopeWordsTravelWithRefusal: the 403 words name the two scopes
// that open the surface (the message IS the material of the transport
// mapping — the words never live in two places).
func TestScopeWordsTravelWithRefusal(t *testing.T) {
	err := newTestService().authorize(context.Background(), identityDomain.Principal{ID: testReviewID}, testTenantID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ожидался отказ скоупов, получено %v", err)
	}
	if !strings.Contains(err.Error(), mcpScopeText) {
		t.Fatalf("слова скоупов обязаны следовать с отказом, получено %q", err.Error())
	}
}

// The compile-time pins of the fakes.
var (
	_ MembershipService = membershipFake{}
	_ ToolSource        = emptySource{}
	_ AdapterRegistry   = emptyAdapters{}
	_ DependencyGate    = gateFake{}
)
