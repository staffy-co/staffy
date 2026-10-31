package mcp

// The surface service (Story 7.3): the membership ladder runs FIRST (a
// non-member of the tenant is the honest 404, a member without the
// integrations|configuration scopes is the 403 with the scope words —
// BEFORE a single byte of JSON-RPC leaves the service), then the declared
// rate window of the principal, then the strict protocol pipeline (the
// parse, the routing-claim coherence, the version negotiation, the
// notification receipt and the method dispatch). Every tools/call writes
// one append-only audit line (the slog stream; the durable PG ledger is
// the deferred work of the first live adapter). The service depends only
// on the ports: the transport and the internal call path stay separable.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/correlation"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Errors of the surface. Authorization failures of members surface as
// ErrForbidden (the honest refusal with the scope words); unknown
// tenants/non-members as ErrNotFound (no disclosure that the tenant has
// an MCP surface at all) — the tools.Service ladder precedent of 7-2.
var (
	ErrNotFound  = errors.New("mcp: not found")
	ErrForbidden = errors.New("mcp: forbidden")
)

// mcpScopeText is the honest scope refusal of the machine surface: the
// words of the two scopes that DO open it, spoken BEFORE any protocol
// material leaves the service (the Story 3.1/7.2 precedent).
const mcpScopeText = "Обращение к MCP-поверхности реестра инструментов требует полномочия integrations или configuration: машинная инструментальная поверхность — материал настройки интеграций и конфигурации, оператору и аудиту она не открывается до вычисления доступности."

// TooManyRequestsError is the rate-window refusal of the declared budget:
// the transport answers the 429 with the Retry-After of the window.
type TooManyRequestsError struct {
	RetryAfter time.Duration
}

func (e *TooManyRequestsError) Error() string {
	return fmt.Sprintf("mcp: окно запросов принципала исчерпано (повтор через %s)", e.RetryAfter)
}

// Outcome is the protocol-level answer of one handled POST: the HTTP
// status of the protocol layer (200 for every JSON-RPC answer, 202 for
// the notification receipt, 400 for the invalid envelope/claims) and the
// JSON-RPC response body (nil for the 202).
type Outcome struct {
	Status int
	Body   *Response
}

// Clock is the time source of the rate window (the fixture world scripts
// it; the production wiring uses the system clock).
type Clock interface{ Now() time.Time }

// systemClock is the production clock.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// The declared rate budget of the surface: the fixed window of one
// principal. The in-memory window is the honest single-node boundary —
// the production deployment of the platform does not exist (AG-4), and a
// horizontal deployment would share the window through the external
// balancer (the deferred boundary, named in the spec).
const (
	defaultRateWindow = time.Minute
	defaultRateLimit  = 120
	rateBucketCeiling = 1024
)

// Service is the MCP surface over the 7-2 registry: the ladder, the rate
// window, the protocol pipeline and the method dispatch.
type Service struct {
	identity   MembershipService
	source     ToolSource
	adapters   AdapterRegistry
	gate       DependencyGate
	logger     *slog.Logger
	window     *rateWindow
	callBudget time.Duration
}

// New wires the surface ports. The adapter registry of the PRODUCTION
// composition root holds zero adapters (AG-4): the callable set is
// honestly empty, the protocol machine answers fully, and every call
// refuses in words (the nil-adapter precedents of 3-6/3-7).
func New(identity MembershipService, source ToolSource, adapters AdapterRegistry, gate DependencyGate, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(discardWriter{}, nil))
	}
	return &Service{
		identity:   identity,
		source:     source,
		adapters:   adapters,
		gate:       gate,
		logger:     logger,
		window:     newRateWindow(defaultRateLimit, defaultRateWindow, systemClock{}),
		callBudget: defaultCallBudget,
	}
}

// WithRateWindow scripts the declared budget (the fixture world of the
// journeys sets the low limit of the 429 leg; production never calls it).
func (s *Service) WithRateWindow(limit int, window time.Duration, clock Clock) *Service {
	s.window = newRateWindow(limit, window, clock)
	return s
}

// WithCallBudget scripts the execution budget of one tool call (the
// fixture world walks the exhausted-budget refusal; production never
// calls it).
func (s *Service) WithCallBudget(budget time.Duration) *Service {
	s.callBudget = budget
	return s
}

// authorize runs the membership ladder FIRST: a non-member of the tenant
// is the honest ErrNotFound, a member without the
// integrations|configuration scopes is the ErrForbidden carrying the
// scope words BEFORE any protocol material (the tools.Service ladder of
// 7-2, verbatim).
func (s *Service) authorize(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) error {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return ErrNotFound
	}
	integrationsErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Integrations)
	if integrationsErr == nil {
		return nil
	}
	if errors.Is(integrationsErr, identityDomain.ErrForbidden) {
		configurationErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Configuration)
		if configurationErr == nil {
			return nil
		}
		if errors.Is(configurationErr, identityDomain.ErrForbidden) {
			return fmt.Errorf("%w: %s", ErrForbidden, mcpScopeText)
		}
		if errors.Is(configurationErr, identityDomain.ErrNotFound) {
			return ErrNotFound
		}
		return configurationErr
	}
	if errors.Is(integrationsErr, identityDomain.ErrNotFound) {
		return ErrNotFound
	}
	return integrationsErr
}

// Handle runs the full pipeline of one POST: the ladder, the rate window,
// the strict parse, the routing-claim coherence, the version negotiation,
// the notification receipt and the method dispatch. The HTTP statuses of
// the Outcome are the protocol layer; the ladder and window refusals
// travel as errors for the transport problem mapping.
func (s *Service) Handle(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, body []byte, claims HeaderClaims) (Outcome, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return Outcome{}, err
	}
	if allowed, retryAfter := s.window.allow(actor.ID); !allowed {
		return Outcome{}, &TooManyRequestsError{RetryAfter: retryAfter}
	}
	envelope, refusal, ok := ParseEnvelope(body)
	if !ok {
		return Outcome{Status: 400, Body: refusal}, nil
	}
	if response, coherent := checkClaims(envelope, claims); !coherent {
		return Outcome{Status: 400, Body: &response}, nil
	}
	claimed := envelope.Meta.ProtocolVersion
	if claimed != ProtocolVersion {
		if envelope.IsNotification() {
			return Outcome{Status: httpStatusAccepted}, nil
		}
		response := unsupportedVersionResponse(envelope.ID, claimed)
		return Outcome{Status: httpStatusOK, Body: &response}, nil
	}
	if envelope.IsNotification() {
		return Outcome{Status: httpStatusAccepted}, nil
	}
	switch envelope.Method {
	case MethodServerDiscover:
		return s.serverDiscover(ctx, envelope)
	case MethodToolsList:
		return s.toolsList(ctx, envelope, actor, tenantID)
	case MethodToolsCall:
		return s.toolsCall(ctx, envelope, actor, tenantID, claims)
	default:
		response := methodNotFoundResponse(envelope.ID, envelope.Method)
		return Outcome{Status: httpStatusOK, Body: &response}, nil
	}
}

// checkClaims holds the routing-claim coherence of the revision: the
// Mcp-Method header is mandatory and must equal the body method; a
// Mcp-Name header on a method that carries no name is incoherent (the
// name-carrying call checks its header against the parsed params); the
// protocol version claim must travel in BOTH the _meta member and the
// MCP-Protocol-Version header, and the two claims must agree.
func checkClaims(envelope *Envelope, claims HeaderClaims) (Response, bool) {
	if claims.Method == "" {
		return invalidRequestResponse(envelope.ID, "Заголовок Mcp-Method обязателен."), false
	}
	if claims.Method != envelope.Method {
		return invalidRequestResponse(envelope.ID, fmt.Sprintf("Заголовок Mcp-Method (%s) расходится с методом сообщения (%s).", claims.Method, envelope.Method)), false
	}
	if envelope.Method != MethodToolsCall && claims.Name != "" {
		return invalidRequestResponse(envelope.ID, "Заголовок Mcp-Name допустим только для tools/call."), false
	}
	if envelope.Meta == nil || envelope.Meta.ProtocolVersion == "" {
		return invalidRequestResponse(envelope.ID, "Заявка ревизии протокола обязательна в _meta каждого сообщения."), false
	}
	if claims.ProtocolVersion == "" {
		return invalidRequestResponse(envelope.ID, "Заголовок MCP-Protocol-Version обязателен."), false
	}
	if envelope.Meta.ProtocolVersion != claims.ProtocolVersion {
		return invalidRequestResponse(envelope.ID, fmt.Sprintf("Заявка ревизии в _meta (%s) расходится с заголовком MCP-Protocol-Version (%s).", envelope.Meta.ProtocolVersion, claims.ProtocolVersion)), false
	}
	return Response{}, true
}

// audit writes the append-only line of one tools/call: the tenant and the
// principal of the session, the pinned tool, the verdict with its honest
// reason and the correlation id of the platform middleware (the durable
// PG ledger is the deferred work — today the production callable set is
// empty, a ledger without rows).
func (s *Service) audit(ctx context.Context, tenantID id.UUID, actor identityDomain.Principal, pin, verdict, reason string) {
	correlationID, _ := correlation.FromContext(ctx)
	s.logger.InfoContext(ctx, "mcp tools/call",
		"code", "mcp_tool_call",
		"tenant_id", tenantID.String(),
		"principal_id", actor.ID.String(),
		"tool", pin,
		"verdict", verdict,
		"reason", reason,
		"correlation_id", correlationID.String(),
	)
}

// ---------------------------------------------------------------------------
// The fixed rate window of one principal
// ---------------------------------------------------------------------------

// rateWindow is the declared fixed window: limit requests per duration
// per principal, held in memory (the honest single-node boundary — AG-4:
// the production deployment of the platform does not exist; a horizontal
// deployment shares the window through the external balancer).
type rateWindow struct {
	mu     sync.Mutex
	limit  int
	span   time.Duration
	clock  Clock
	bucket map[id.UUID]rateBucket
}

type rateBucket struct {
	start time.Time
	count int
}

func newRateWindow(limit int, span time.Duration, clock Clock) *rateWindow {
	return &rateWindow{limit: limit, span: span, clock: clock, bucket: map[id.UUID]rateBucket{}}
}

// allow counts one request of the principal and reports whether the
// declared budget holds; the second return is the honest retry hint of
// the refusal (the time until the current window closes).
func (w *rateWindow) allow(principal id.UUID) (bool, time.Duration) {
	now := w.clock.Now()
	w.mu.Lock()
	defer w.mu.Unlock()
	current, exists := w.bucket[principal]
	if !exists || now.Sub(current.start) >= w.span {
		w.bucket[principal] = rateBucket{start: now, count: 1}
		w.sweep(now)
		return true, 0
	}
	if current.count >= w.limit {
		return false, w.span - now.Sub(current.start)
	}
	current.count++
	w.bucket[principal] = current
	return true, 0
}

// sweep drops the expired buckets when the map grows past the declared
// ceiling (the honest memory bound of the single-node window).
func (w *rateWindow) sweep(now time.Time) {
	if len(w.bucket) <= rateBucketCeiling {
		return
	}
	for principal, bucket := range w.bucket {
		if now.Sub(bucket.start) >= w.span {
			delete(w.bucket, principal)
		}
	}
}

// discardWriter sinks the no-logger wiring (the zero service never
// panics on a nil logger).
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// The protocol-layer statuses the service speaks (named once, so the
// transport and the tests never disagree about the wire).
const (
	httpStatusOK         = 200
	httpStatusAccepted   = 202
	httpStatusBadRequest = 400
)
