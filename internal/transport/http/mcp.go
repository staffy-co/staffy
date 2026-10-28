package httptransport

// ---------------------------------------------------------------------------
// The Story 7.3 transport: the MCP surface endpoint. ONE machine path —
// POST /v1/tenants/{tenant_id}/mcp — behind the membership ladder of the
// platform: no identity is the 401 of the resolver, an unparseable tenant
// id and a non-member are the honest 404, a member without the
// integrations|configuration scopes is the 403 with the scope words — all
// BEFORE a single byte of JSON-RPC leaves the server (the 3.1/7.2
// precedent). The declared transport budgets run before the service:
// a non-JSON content type is the 415, a body past 256 KiB is the 413. The
// protocol answers of the service map onto the wire: every JSON-RPC
// answer (result or error) is the 200, the notification receipt is the
// bare 202, the invalid envelope/claims are the 400 with the JSON-RPC
// error body; the ladder and window refusals are the platform problem
// contract (the 429 carries Retry-After). GET and DELETE are the honest
// 405 — the pinned revision has no streams and no sessions.
// ---------------------------------------------------------------------------

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"

	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/mcp"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// maxMCPBodyBytes is the declared body budget of the surface (256 KiB):
// the honest transport boundary named in the OpenAPI contract.
const maxMCPBodyBytes = 256 << 10

// mcpService is the narrow transport contract of the MCP surface (the
// app mcp.Service carries the compile-time proof; the transport tests
// inject a fake — the Story 3.1 precedent).
type mcpService interface {
	Handle(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, body []byte, claims mcp.HeaderClaims) (mcp.Outcome, error)
}

// BindMCP registers the MCP surface endpoint (the machine path only —
// the operator dictionary of the registry stays the 7.2 page).
func (s *Server) BindMCP(service mcpService) {
	s.mcp = service
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/mcp", s.mcpEndpoint)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/mcp", s.mcpMethodNotAllowed)
	s.mux.HandleFunc("DELETE /v1/tenants/{tenant_id}/mcp", s.mcpMethodNotAllowed)
}

// mcpMethodNotAllowed answers the honest 405 of the revision: no streams,
// no sessions, one POST per message.
func (s *Server) mcpMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	s.problem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "MCP-поверхность отвечает только на POST: ревизия протокола 2026-07-28 не имеет стримов и сессий.", false, nil)
}

// mcpEndpoint runs the transport ladder and budgets, then hands the body
// and the routing-claim headers to the service pipeline.
func (s *Server) mcpEndpoint(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.problem(w, r, http.StatusNotFound, "not_found", "Пространство не найдено.", false, nil)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		s.problem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Используйте Content-Type application/json.", false, nil)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMCPBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			s.problem(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Тело MCP-запроса превышает декларированный бюджет 256 KiB.", false, nil)
			return
		}
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Тело MCP-запроса не прочитано.", false, nil)
		return
	}
	claims := mcp.HeaderClaims{
		Method:          r.Header.Get(mcp.HeaderMethod),
		Name:            r.Header.Get(mcp.HeaderName),
		ProtocolVersion: r.Header.Get(mcp.HeaderProtocolVersion),
	}
	outcome, err := s.mcp.Handle(r.Context(), actor, tenant, body, claims)
	if err != nil {
		s.mcpProblem(w, r, err)
		return
	}
	if outcome.Status == http.StatusAccepted {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(outcome.Status)
	_ = json.NewEncoder(w).Encode(outcome.Body)
}

// mcpProblem maps the service refusals onto the platform problem
// contract: the module's ErrNotFound is the plain 404 (no disclosure
// that the tenant has an MCP surface); the module's ErrForbidden is the
// 403 whose message IS the scope words the service attached at the wrap
// point; the window refusal is the 429 with the honest Retry-After of
// the declared budget; anything else is the honest 500 with the
// server-side log.
func (s *Server) mcpProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, mcp.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Пространство не найдено.", false, nil)
	case errors.Is(err, mcp.ErrForbidden):
		// The service wrap is fmt.Errorf("%w: %s", ErrForbidden, scopeWords):
		// strip the sentinel prefix and speak the scope words themselves.
		message := err.Error()
		if prefix := mcp.ErrForbidden.Error() + ": "; strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
		}
		s.problem(w, r, http.StatusForbidden, "mcp_forbidden", message, false, nil)
	default:
		var tooMany *mcp.TooManyRequestsError
		if errors.As(err, &tooMany) {
			seconds := int(math.Ceil(tooMany.RetryAfter.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			s.problem(w, r, http.StatusTooManyRequests, "rate_limited", "Окно запросов принципала исчерпано — повторите обращение после Retry-After.", true, nil)
			return
		}
		s.logger.ErrorContext(r.Context(), "mcp surface failed", "code", "mcp_surface_failed", "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось обработать MCP-запрос. Повторите позднее.", true, nil)
	}
}
