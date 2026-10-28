package mcp

// Package mcp is the protocol machine of the MCP tool surface (Story 7.3):
// the pinned revision 2026-07-28 of the Model Context Protocol — the
// stateless core. The handshake methods (initialize/initialized) and the
// session header of the handshake era are GONE from the protocol (the
// revision deleted them): the surface answers ONE JSON-RPC 2.0 message per
// POST, never initiates a stream and holds no session state. The package
// is the platform protocol layer: it carries no vertical vocabulary (the
// drift probe zone), and the tool material comes from the 7-2 registry
// through the ports — the MCP layer never creates a tool and never
// extends a dictionary (the write impossibility of AD-6/AD-7 is
// structural, not reviewed).
//
// Wire-name pinning: the base JSON-RPC envelope fields and the
// CallToolResult members are anchored by the live reading of the official
// schema (2025-11-25); the revision-specific names (server/discover, the
// Mcp-Method/Mcp-Name/MCP-Protocol-Version headers, the _meta version
// claim) are pinned by the platform tests of this package — the official
// 2026-07-28 schema asset was not live-readable at the implementation
// date, and the choice is documented in the DEVLOG entry of the step.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// ProtocolVersion is the ONLY supported protocol revision (the pinned
// current revision — a foreign revision claim gets the honest
// UnsupportedProtocolVersionError carrying exactly this list).
const ProtocolVersion = "2026-07-28"

// SupportedProtocolVersions returns the wire list of the discover answer:
// the single pinned revision.
func SupportedProtocolVersions() []string { return []string{ProtocolVersion} }

// The method dictionary of the pinned revision. The handshake methods of
// the older era do not exist: a client that calls them receives the same
// honest method-not-found as any other unknown method (the capabilities
// never claimed them).
const (
	MethodServerDiscover = "server/discover"
	MethodToolsList      = "tools/list"
	MethodToolsCall      = "tools/call"
)

// The transport headers of the pinned revision (SEP-2243): the routing
// headers are checked against the body — a desync is the invalid request.
const (
	HeaderMethod          = "Mcp-Method"
	HeaderName            = "Mcp-Name"
	HeaderProtocolVersion = "MCP-Protocol-Version"
)

// The JSON-RPC 2.0 error codes of the surface. The unsupported-protocol
// identity rides the invalid-params code with the named message and the
// supportedVersions data — the platform pinning of the revision's error
// shape (the official schema asset was not live-readable; the choice is
// named in the DEVLOG and held by the tests).
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Envelope is the strict single-message decode of one MCP request: the
// CLOSED top-level form of the pinned revision (jsonrpc, id, method,
// params, _meta — an unknown key is the invalid request; a batch array is
// the invalid request: the revision carries one message per POST).
type Envelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Meta    *RequestMeta    `json:"_meta,omitempty"`
}

// RequestMeta is the _meta member of the request: the protocol version
// claim. The form is closed — an unknown _meta key is the invalid request.
type RequestMeta struct {
	ProtocolVersion string `json:"protocolVersion"`
}

// IsNotification reports whether the message carries no id: the sender
// expects no answer, and the honest server answer is the bare 202 without
// action (the stateless core cancels nothing in flight).
func (e *Envelope) IsNotification() bool { return len(e.ID) == 0 }

// Response is the single JSON-RPC response of the surface: the result or
// the error — never both. The id echoes the request id (null only for the
// requests that never parsed far enough to own one).
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *WireError      `json:"error,omitempty"`
}

// WireError is the error member of the JSON-RPC response.
type WireError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// NewResult builds the response of a successful method call.
func NewResult(id json.RawMessage, result any) Response {
	encoded, err := json.Marshal(result)
	if err != nil {
		return NewError(id, CodeInternalError, "Не удалось закодировать ответ.", nil)
	}
	return Response{JSONRPC: "2.0", ID: id, Result: encoded}
}

// NewError builds the error response of a protocol-level refusal.
func NewError(id json.RawMessage, code int, message string, data any) Response {
	response := Response{JSONRPC: "2.0", ID: id, Error: &WireError{Code: code, Message: message}}
	if data != nil {
		if encoded, err := json.Marshal(data); err == nil {
			response.Error.Data = encoded
		}
	}
	return response
}

// ParseEnvelope decodes one request body strictly. The body must be a
// single JSON object (a leading array is the batch of the old era — the
// invalid request of the revision); the top-level form is closed; the id
// is the string|number of JSON-RPC (an object, array, boolean or null id
// is the invalid request; an absent id is the notification); jsonrpc is
// the literal "2.0"; the method is mandatory; params (when present) is an
// object; _meta (when present) is the closed version-claim form. The
// second return is the ready invalid/parse response when the decode
// refused — the caller writes it with the HTTP 400.
func ParseEnvelope(body []byte) (*Envelope, *Response, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, &parseErrorResponse, false
	}
	if trimmed[0] == '[' {
		return nil, &invalidBatchResponse, false
	}
	if trimmed[0] != '{' {
		return nil, &parseErrorResponse, false
	}
	var envelope Envelope
	if err := strictUnmarshal(body, &envelope); err != nil {
		return nil, &parseErrorResponse, false
	}
	if envelope.JSONRPC != "2.0" {
		return nil, responsePointer(invalidRequestResponse(envelope.ID, "Поле jsonrpc обязано быть строкой \"2.0\".")), false
	}
	if envelope.Method == "" {
		return nil, responsePointer(invalidRequestResponse(envelope.ID, "Сообщение обязано нести метод.")), false
	}
	if len(envelope.Params) > 0 && bytes.TrimSpace(envelope.Params)[0] != '{' {
		return nil, responsePointer(invalidRequestResponse(envelope.ID, "Параметры обязаны быть JSON-объектом.")), false
	}
	if !validRequestID(envelope.ID) {
		return nil, responsePointer(invalidRequestResponse(nil, "Идентификатор сообщения обязан быть строкой или числом.")), false
	}
	return &envelope, nil, true
}

// validRequestID holds the id dictionary of the revision: absent (the
// notification), a JSON string or a JSON number. The null (the JSON
// no-op decode — caught by the type switch, never by the error), the
// boolean, the object and the array forms are the invalid request.
func validRequestID(id json.RawMessage) bool {
	if len(id) == 0 {
		return true
	}
	var probe any
	if err := json.Unmarshal(bytes.TrimSpace(id), &probe); err != nil {
		return false
	}
	switch probe.(type) {
	case string, float64:
		return true
	default:
		return false
	}
}

// The shared parse-error response: the id is null because the message
// never parsed far enough to own one.
var parseErrorResponse = NewError(nil, CodeParseError, "Сообщение не является корректным JSON.", nil)

// invalidBatchResponse refuses the batch of the old era by name: the
// revision carries ONE message per POST.
var invalidBatchResponse = NewError(nil, CodeInvalidRequest, "Пакетная отправка не поддерживается: ревизия протокола 2026-07-28 несёт одно JSON-RPC сообщение на POST.", nil)

// invalidRequestResponse builds the named invalid-request refusal.
func invalidRequestResponse(id json.RawMessage, reason string) Response {
	return NewError(id, CodeInvalidRequest, reason, nil)
}

// methodNotFoundResponse is the honest answer of an unknown method: the
// capabilities never claimed it, so the refusal carries the method by
// name and the three methods that DO exist.
func methodNotFoundResponse(id json.RawMessage, method string) Response {
	message := fmt.Sprintf("Метод %q не существует: поверхность заявляет только %s, %s и %s.", method, MethodServerDiscover, MethodToolsList, MethodToolsCall)
	return NewError(id, CodeMethodNotFound, message, nil)
}

// unsupportedVersionResponse is the honest answer of a coherent but
// foreign revision claim: the name of the error identity, the claimed
// revision and the exact supported list travel together.
func unsupportedVersionResponse(id json.RawMessage, claimed string) Response {
	message := fmt.Sprintf("UnsupportedProtocolVersionError: заявлена ревизия %s, сервер поддерживает только [%s].", claimed, ProtocolVersion)
	return NewError(id, CodeInvalidParams, message, map[string]any{"supportedVersions": SupportedProtocolVersions()})
}

// responsePointer lifts a built response to the pointer form of the
// refusal returns.
func responsePointer(response Response) *Response { return &response }

// strictUnmarshal decodes with the closed-form discipline of the platform:
// an unknown key of the destination struct is an error (the strict decode
// of the machine endpoints).
func strictUnmarshal(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return err
	}
	return nil
}
