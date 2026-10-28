package mcpclient

// Package mcpclient is the CLIENT protocol machine of the external MCP
// connector class (Story 7-4): the pinned revision 2026-07-28 of the Model
// Context Protocol — the stateless core. The dictionaries MIRROR the server
// machine of 7-3 (the same revision, the same SEP words: the stateless
// single-message core, the Mcp-Method/Mcp-Name/MCP-Protocol-Version routing
// headers of SEP-2243, the protocol version claim in _meta; the
// cache-control words of SEP-2549 are NOT claimed by this client) — the
// mirror is deliberate: the client is a separate platform zone whose
// dictionaries are pinned by the tests of THIS package, and both packages
// name the same revision constant (a revision change is a conscious step in
// both machines, held red by the pinning tests).
//
// The machine is credential-agnostic (the AD-6/AD-7 discipline of the
// slice): the Authorization material is an explicit caller parameter — it
// travels to the round and is never stored, never logged and never echoed
// by this package. The answers of the external server are UNTRUSTED
// material (AD-11): the strict decode refuses the malformed envelope (the
// result XOR error, the id echo, the version coherence), the reading
// budgets refuse the oversized body without truncation, and the tool
// descriptors are filtered to the known protocol fields at decode — an
// undeclared key of a foreign answer never travels further.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ProtocolVersion is the ONLY protocol revision this client speaks (the
// pinned current revision — every request claims it in _meta and in the
// MCP-Protocol-Version header, and the discover negotiation must select
// exactly this revision).
const ProtocolVersion = "2026-07-28"

// The method dictionary of the pinned revision (the mirror of the server
// machine of 7-3): the three methods of the stateless core. The handshake
// methods of the older era do not exist — this client never sends them.
const (
	MethodServerDiscover = "server/discover"
	MethodToolsList      = "tools/list"
	MethodToolsCall      = "tools/call"
)

// The transport headers of the pinned revision (SEP-2243): every POST
// carries the routing headers coherent with the body.
const (
	HeaderMethod          = "Mcp-Method"
	HeaderName            = "Mcp-Name"
	HeaderProtocolVersion = "MCP-Protocol-Version"
)

// The honest failure identities of the client machine. The probe maps them
// onto the evidence vocabulary: the protocol failures are the failed
// outcome with the named reason; the deadline exhaustion is the unknown
// outcome (a possibly-effective round is never retried blind — the
// discipline of the 1-8 probes).
var (
	// ErrMalformedResponse is the malformed envelope of a foreign server:
	// no result XOR error, a non-2.0 jsonrpc word, an id that does not echo
	// the request, a result that is not the closed shape of the method.
	ErrMalformedResponse = errors.New("mcpclient: малформед-конверт ответа внешнего сервера")
	// ErrForeignRevision is the revision incoherence: the discover answer
	// does not carry the pinned revision, or the response claims a foreign
	// protocol version in _meta.
	ErrForeignRevision = errors.New("mcpclient: чужая ревизия протокола внешнего сервера")
	// ErrResponseBudget is the size budget of the reading: the body past
	// the declared budget is refused whole — no truncation, no passthrough.
	ErrResponseBudget = errors.New("mcpclient: размерный бюджет чтения ответа превышен")
	// ErrDeadline is the context budget of the round: the round may have
	// been received and effected by the foreign server — the honest outcome
	// is unknown, never a blind retry.
	ErrDeadline = errors.New("mcpclient: бюджет раунда исчерпан до завершения")
)

// ServerError is the JSON-RPC error answer of a foreign server. The code is
// the numeric wire fact; the MESSAGE of the foreign server is untrusted
// material (AD-11) and is deliberately not carried by this type.
type ServerError struct{ Code int }

func (e *ServerError) Error() string {
	return fmt.Sprintf("mcpclient: внешний сервер ответил ошибкой JSON-RPC (код %d)", e.Code)
}

// requestEnvelope is the single JSON-RPC 2.0 message of one POST: the
// protocol version claim rides _meta (the closed form of the revision).
type requestEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Meta    requestMeta     `json:"_meta"`
}

// requestMeta is the _meta member of the request: the pinned revision
// claim (the client speaks exactly one revision).
type requestMeta struct {
	ProtocolVersion string `json:"protocolVersion"`
}

// responseEnvelope is the decode of one answer: the result XOR the error
// (never both), the id that echoes the request, and — when a foreign
// server claims a protocol version in _meta — the coherence check of the
// claim. Unknown members of a foreign answer are dropped at decode (the
// AD-11 filtering of the untrusted material).
type responseEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *wireError      `json:"error,omitempty"`
	Meta    *responseMeta   `json:"_meta,omitempty"`
}

// responseMeta is the optional _meta member of an answer: a foreign
// protocol version claim that disagrees with the pinned revision is the
// honest incoherence refusal.
type responseMeta struct {
	ProtocolVersion string `json:"protocolVersion,omitempty"`
}

// wireError is the error member of a foreign answer. The message is
// untrusted material: it is decoded (the envelope validation needs the
// member) but never travels further than this struct.
type wireError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// DiscoverResult is the answer of the revision negotiation: the supported
// list of the foreign server and its identity. The capabilities member of
// the full answer is not needed by the probe surface and is filtered at
// decode (AD-11).
type DiscoverResult struct {
	ProtocolVersions []string   `json:"protocolVersions"`
	ServerInfo       ServerInfo `json:"serverInfo"`
}

// ServerInfo is the identity of the foreign server (the provenance
// material of the dictionary entries).
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolDescriptor is ONE filtered tool descriptor of the foreign inventory:
// exactly the known protocol fields travel further — the unknown members
// of a foreign descriptor are dropped at decode (AD-11: no passthrough).
type ToolDescriptor struct {
	Name         string          `json:"name"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  map[string]any  `json:"inputSchema"`
	OutputSchema map[string]any  `json:"outputSchema"`
	Annotations  ToolAnnotations `json:"annotations"`
}

// ToolAnnotations carries the behavioral hints of the descriptor: the
// read-only hint is the admission fact of the dictionary projection
// (AR-30 — a tool without the read-only statement is never admitted).
type ToolAnnotations struct {
	ReadOnlyHint bool `json:"readOnlyHint"`
}

// ListToolsResult is the filtered inventory answer of the list round.
type ListToolsResult struct {
	Tools []ToolDescriptor `json:"tools"`
}

// CallToolResult is the answer of one tool call round: the content blocks,
// the structured content and the honest error flag of the tool execution.
type CallToolResult struct {
	Content           []ContentBlock `json:"content"`
	StructuredContent map[string]any `json:"structuredContent,omitempty"`
	IsError           bool           `json:"isError,omitempty"`
}

// ContentBlock is one content entry of the call answer.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// decodeEnvelope validates one answer against the closed envelope contract
// of the client: jsonrpc is the literal 2.0, the id echoes the request id,
// exactly one of the result and the error is present, and a foreign
// protocol version claim in _meta is the honest incoherence refusal. The
// unknown members of the answer are dropped (the AD-11 filtering).
func decodeEnvelope(body []byte, requestID string) (*responseEnvelope, error) {
	var envelope responseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%w: конверт не декодирован", ErrMalformedResponse)
	}
	if envelope.JSONRPC != "2.0" {
		return nil, fmt.Errorf("%w: jsonrpc %q не является \"2.0\"", ErrMalformedResponse, envelope.JSONRPC)
	}
	if envelope.ID != requestID {
		return nil, fmt.Errorf("%w: id ответа не эхо запроса", ErrMalformedResponse)
	}
	if (len(envelope.Result) == 0) == (envelope.Error == nil) {
		return nil, fmt.Errorf("%w: ответ несёт результат и ошибку одновременно либо не несёт ни одного", ErrMalformedResponse)
	}
	if envelope.Meta != nil && envelope.Meta.ProtocolVersion != "" && envelope.Meta.ProtocolVersion != ProtocolVersion {
		return nil, fmt.Errorf("%w: ответ заявляет ревизию %s", ErrForeignRevision, envelope.Meta.ProtocolVersion)
	}
	if envelope.Error != nil {
		return nil, &ServerError{Code: envelope.Error.Code}
	}
	return &envelope, nil
}

// decodeDiscover validates the discover answer: the protocolVersions list
// must be a present JSON array (never null — the honest empty state of the
// protocol is the empty array) that carries the pinned revision (the
// negotiation selects exactly 2026-07-28), and the serverInfo must be the
// closed identity form.
func decodeDiscover(result json.RawMessage) (DiscoverResult, error) {
	var probe struct {
		ProtocolVersions json.RawMessage `json:"protocolVersions"`
		ServerInfo       json.RawMessage `json:"serverInfo"`
	}
	if err := json.Unmarshal(result, &probe); err != nil {
		return DiscoverResult{}, fmt.Errorf("%w: результат discover не декодирован", ErrMalformedResponse)
	}
	if len(probe.ProtocolVersions) == 0 || bytes.Equal(bytes.TrimSpace(probe.ProtocolVersions), []byte("null")) {
		return DiscoverResult{}, fmt.Errorf("%w: protocolVersions обязана быть массивом", ErrMalformedResponse)
	}
	if len(probe.ServerInfo) == 0 || bytes.Equal(bytes.TrimSpace(probe.ServerInfo), []byte("null")) {
		return DiscoverResult{}, fmt.Errorf("%w: serverInfo обязана быть объектом", ErrMalformedResponse)
	}
	var answer DiscoverResult
	if err := json.Unmarshal(result, &answer); err != nil {
		return DiscoverResult{}, fmt.Errorf("%w: результат discover не декодирован", ErrMalformedResponse)
	}
	negotiated := false
	for _, revision := range answer.ProtocolVersions {
		if revision == ProtocolVersion {
			negotiated = true
			break
		}
	}
	if !negotiated {
		return DiscoverResult{}, fmt.Errorf("%w: сервер заявил список %v без ревизии %s", ErrForeignRevision, answer.ProtocolVersions, ProtocolVersion)
	}
	return answer, nil
}

// decodeList validates the inventory answer: the tools member must be a
// present JSON array (never null) of the filtered descriptors.
func decodeList(result json.RawMessage) (ListToolsResult, error) {
	var probe struct {
		Tools json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(result, &probe); err != nil {
		return ListToolsResult{}, fmt.Errorf("%w: результат tools/list не декодирован", ErrMalformedResponse)
	}
	if len(probe.Tools) == 0 || bytes.Equal(bytes.TrimSpace(probe.Tools), []byte("null")) {
		return ListToolsResult{}, fmt.Errorf("%w: tools обязана быть массивом", ErrMalformedResponse)
	}
	var answer ListToolsResult
	if err := json.Unmarshal(result, &answer); err != nil {
		return ListToolsResult{}, fmt.Errorf("%w: результат tools/list не декодирован", ErrMalformedResponse)
	}
	return answer, nil
}

// decodeCall validates the call answer: the content member must be a
// present JSON array of the content blocks.
func decodeCall(result json.RawMessage) (CallToolResult, error) {
	var probe struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(result, &probe); err != nil {
		return CallToolResult{}, fmt.Errorf("%w: результат tools/call не декодирован", ErrMalformedResponse)
	}
	if len(probe.Content) == 0 || bytes.Equal(bytes.TrimSpace(probe.Content), []byte("null")) {
		return CallToolResult{}, fmt.Errorf("%w: content обязана быть массивом", ErrMalformedResponse)
	}
	var answer CallToolResult
	if err := json.Unmarshal(result, &answer); err != nil {
		return CallToolResult{}, fmt.Errorf("%w: результат tools/call не декодирован", ErrMalformedResponse)
	}
	return answer, nil
}
