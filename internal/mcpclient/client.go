package mcpclient

// The client machine over the injected RoundTripper port (Story 7-4
// step-01): ONE stateless POST per round — the single JSON-RPC message
// with the pinned revision claim in _meta and the routing headers coherent
// with the body (SEP-2243), the Authorization material as the explicit
// caller parameter, the context budget of the caller's deadline, and the
// declared reading budget of the answer body. The machine holds no
// session, opens no stream and never retries a round: a deadline
// exhaustion is the honest unknown (a possibly-effective round), a
// protocol refusal is the honest failed outcome with the named reason.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is the stateless protocol machine over one RoundTripper. The
// transport port keeps the machine testable against the REAL wire of an
// httptest server (the production composition injects the default
// transport; the probe wires the machine budgets).
type Client struct {
	transport      http.RoundTripper
	responseBudget int64
}

// New wires the transport port and the reading budget of every answer
// body (the machine mirror of the capability SizeLimit declaration — a
// body past the budget is refused whole, without truncation).
func New(transport http.RoundTripper, responseBudget int64) *Client {
	return &Client{transport: transport, responseBudget: responseBudget}
}

// Discover runs the revision negotiation round: server/discover with the
// empty params. The answer must carry the pinned revision in its
// supported list — the client negotiates exactly 2026-07-28.
func (c *Client) Discover(ctx context.Context, endpoint, authorization string) (DiscoverResult, error) {
	envelope, err := c.round(ctx, endpoint, authorization, MethodServerDiscover, nil, "")
	if err != nil {
		return DiscoverResult{}, err
	}
	return decodeDiscover(envelope.Result)
}

// ListTools runs the inventory round: tools/list with the empty params.
// The answer is the filtered descriptor array (the unknown members of a
// foreign descriptor are dropped at decode — AD-11).
func (c *Client) ListTools(ctx context.Context, endpoint, authorization string) (ListToolsResult, error) {
	envelope, err := c.round(ctx, endpoint, authorization, MethodToolsList, nil, "")
	if err != nil {
		return ListToolsResult{}, err
	}
	return decodeList(envelope.Result)
}

// CallTool runs one tool call round: tools/call with the closed params
// form {name, arguments} and the Mcp-Name routing header coherent with
// the body. The Authorization material is the explicit caller parameter.
func (c *Client) CallTool(ctx context.Context, endpoint, authorization, toolName string, arguments map[string]any) (CallToolResult, error) {
	params, err := json.Marshal(map[string]any{"name": toolName, "arguments": arguments})
	if err != nil {
		return CallToolResult{}, fmt.Errorf("mcpclient: аргументы вызова не кодируются: %w", err)
	}
	envelope, err := c.round(ctx, endpoint, authorization, MethodToolsCall, params, toolName)
	if err != nil {
		return CallToolResult{}, err
	}
	return decodeCall(envelope.Result)
}

// round performs ONE POST: the request envelope with the pinned revision
// claim, the routing headers, the Authorization material of the caller,
// the context deadline of the caller and the reading budget of the
// machine; the answer is decoded strictly (the result XOR error, the id
// echo, the version coherence). The round never retries: a deadline
// exhaustion is the honest unknown of a possibly-effective round.
func (c *Client) round(ctx context.Context, endpoint, authorization, method string, params json.RawMessage, nameHeader string) (*responseEnvelope, error) {
	requestID, err := newRequestID()
	if err != nil {
		return nil, fmt.Errorf("mcpclient: идентификатор раунда не сгенерирован: %w", err)
	}
	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  method,
		Params:  params,
		Meta:    requestMeta{ProtocolVersion: ProtocolVersion},
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: запрос не кодируется: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mcpclient: раунд не построен: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set(HeaderMethod, method)
	httpRequest.Header.Set(HeaderProtocolVersion, ProtocolVersion)
	if nameHeader != "" {
		httpRequest.Header.Set(HeaderName, nameHeader)
	}
	if authorization != "" {
		// The credential-agnostic fact: the material travels to the round
		// verbatim and is never stored, logged or echoed by this machine.
		httpRequest.Header.Set("Authorization", authorization)
	}
	httpResponse, err := c.transport.RoundTrip(httpRequest)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %v", ErrDeadline, ctx.Err())
		}
		return nil, fmt.Errorf("mcpclient: транспорт раунда отказал: %w", err)
	}
	defer func() { _ = httpResponse.Body.Close }()
	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcpclient: внешний сервер ответил статусом %d (ожидался 200)", httpResponse.StatusCode)
	}
	answer, err := io.ReadAll(io.LimitReader(httpResponse.Body, c.responseBudget+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %v", ErrDeadline, ctx.Err())
		}
		return nil, fmt.Errorf("mcpclient: тело ответа не прочитано: %w", err)
	}
	if int64(len(answer)) > c.responseBudget {
		return nil, fmt.Errorf("%w: тело превышает бюджет %d байт", ErrResponseBudget, c.responseBudget)
	}
	if len(bytes.TrimSpace(answer)) == 0 {
		return nil, fmt.Errorf("%w: пустое тело ответа", ErrMalformedResponse)
	}
	return decodeEnvelope(answer, requestID)
}

// newRequestID mints the unique round identifier (the echo anchor of the
// answer validation — a foreign server that does not echo it is refused).
func newRequestID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "staffy-mcpclient-" + hex.EncodeToString(raw), nil
}
