package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestProtocolDictionariesPinned holds the wire names of the pinned
// revision: the protocol version, the method dictionary, the routing
// headers and the error identity of the foreign-revision refusal. A drift
// of any name turns this probe red before it reaches the OpenAPI mirror.
func TestProtocolDictionariesPinned(t *testing.T) {
	if ProtocolVersion != "2026-07-28" {
		t.Fatalf("ревизия протокола пиннута как %q", ProtocolVersion)
	}
	if versions := SupportedProtocolVersions(); len(versions) != 1 || versions[0] != ProtocolVersion {
		t.Fatalf("список поддерживаемых ревизий обязан быть ровно [%s], получен %v", ProtocolVersion, versions)
	}
	if MethodServerDiscover != "server/discover" || MethodToolsList != "tools/list" || MethodToolsCall != "tools/call" {
		t.Fatalf("словарь методов пиннут неверно: %q %q %q", MethodServerDiscover, MethodToolsList, MethodToolsCall)
	}
	if HeaderMethod != "Mcp-Method" || HeaderName != "Mcp-Name" || HeaderProtocolVersion != "MCP-Protocol-Version" {
		t.Fatalf("словарь заголовков пиннут неверно: %q %q %q", HeaderMethod, HeaderName, HeaderProtocolVersion)
	}
}

// TestParseEnvelopeDecodeMatrix walks the strict decode of the single
// message: the valid forms (the string id, the number id, the
// notification), the batch refusal of the old era, the parse errors, the
// closed top-level form, the id dictionary and the params object rule.
func TestParseEnvelopeDecodeMatrix(t *testing.T) {
	valid := func(body string) *Envelope {
		t.Helper()
		envelope, refusal, ok := ParseEnvelope([]byte(body))
		if !ok {
			t.Fatalf("корректное сообщение отклонено: %s — отказ %+v", body, refusal)
		}
		return envelope
	}
	invalid := func(body string, messagePart string) {
		t.Helper()
		envelope, refusal, ok := ParseEnvelope([]byte(body))
		if ok {
			t.Fatalf("малформед-сообщение принято: %s → %+v", body, envelope)
		}
		if refusal == nil {
			t.Fatalf("отказ без тела: %s", body)
		}
		if !strings.Contains(refusal.Error.Message, messagePart) {
			t.Fatalf("отказ %s не называет причину %q: %q", body, messagePart, refusal.Error.Message)
		}
	}

	// The valid forms.
	envelope := valid(`{"jsonrpc":"2.0","id":"abc","method":"tools/list","_meta":{"protocolVersion":"2026-07-28"}}`)
	if string(envelope.ID) != `"abc"` || envelope.Method != "tools/list" || envelope.Meta.ProtocolVersion != ProtocolVersion {
		t.Fatalf("декод строкового id неверен: %+v", envelope)
	}
	if envelope.IsNotification() {
		t.Fatal("сообщение с id не является уведомлением")
	}
	envelope = valid(`{"jsonrpc":"2.0","id":7,"method":"server/discover","params":{},"_meta":{"protocolVersion":"2026-07-28"}}`)
	if string(envelope.ID) != "7" || len(envelope.Params) == 0 {
		t.Fatalf("декод числового id неверен: %+v", envelope)
	}
	envelope = valid(`{"jsonrpc":"2.0","method":"tools/call","_meta":{"protocolVersion":"2026-07-28"}}`)
	if !envelope.IsNotification() {
		t.Fatal("сообщение без id является уведомлением")
	}

	// The batch of the old era: the named invalid request.
	invalid(`[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]`, "одно JSON-RPC сообщение на POST")
	// The parse errors.
	invalid(`{`, "не является корректным JSON")
	invalid(``, "не является корректным JSON")
	invalid(`"строка"`, "не является корректным JSON")
	// The closed top-level form.
	invalid(`{"jsonrpc":"2.0","id":1,"method":"tools/list","extra":true}`, "не является корректным JSON")
	// The closed _meta form.
	invalid(`{"jsonrpc":"2.0","id":1,"method":"tools/list","_meta":{"protocolVersion":"2026-07-28","extra":true}}`, "не является корректным JSON")
	// The envelope rules.
	invalid(`{"jsonrpc":"1.0","id":1,"method":"tools/list"}`, "jsonrpc")
	invalid(`{"jsonrpc":"2.0","id":1}`, "метод")
	invalid(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":[]}`, "JSON-объектом")
	// The id dictionary: null, boolean, object and array are invalid.
	invalid(`{"jsonrpc":"2.0","id":null,"method":"tools/list"}`, "строкой или числом")
	invalid(`{"jsonrpc":"2.0","id":true,"method":"tools/list"}`, "строкой или числом")
	invalid(`{"jsonrpc":"2.0","id":{"a":1},"method":"tools/list"}`, "строкой или числом")
	invalid(`{"jsonrpc":"2.0","id":[1],"method":"tools/list"}`, "строкой или числом")
}

// TestUnsupportedVersionResponseCarriesTheList: the honest refusal of a
// foreign revision names the error identity, the claimed revision and the
// exact supported list.
func TestUnsupportedVersionResponseCarriesTheList(t *testing.T) {
	response := unsupportedVersionResponse(json.RawMessage(`7`), "2025-11-25")
	if response.Error == nil || response.Error.Code != CodeInvalidParams {
		t.Fatalf("отказ чужой ревизии обязан нести код invalid params: %+v", response)
	}
	if !strings.Contains(response.Error.Message, "UnsupportedProtocolVersionError") || !strings.Contains(response.Error.Message, "2025-11-25") {
		t.Fatalf("отказ не называет тип ошибки и заявленную ревизию: %q", response.Error.Message)
	}
	var data struct {
		SupportedVersions []string `json:"supportedVersions"`
	}
	if err := json.Unmarshal(response.Error.Data, &data); err != nil {
		t.Fatalf("data отказа не читается: %v", err)
	}
	if len(data.SupportedVersions) != 1 || data.SupportedVersions[0] != ProtocolVersion {
		t.Fatalf("список поддерживаемых ревизий обязан быть ровно [%s]: %v", ProtocolVersion, data.SupportedVersions)
	}
}

// TestMethodNotFoundNamesTheMethod: the honest refusal of an unknown
// method carries the method by name and the three methods that exist.
func TestMethodNotFoundNamesTheMethod(t *testing.T) {
	response := methodNotFoundResponse(json.RawMessage(`"x"`), "resources/list")
	if response.Error == nil || response.Error.Code != CodeMethodNotFound {
		t.Fatalf("отказ неизвестного метода обязан нести -32601: %+v", response)
	}
	for _, word := range []string{"resources/list", MethodServerDiscover, MethodToolsList, MethodToolsCall} {
		if !strings.Contains(response.Error.Message, word) {
			t.Fatalf("отказ не называет %q: %q", word, response.Error.Message)
		}
	}
}
