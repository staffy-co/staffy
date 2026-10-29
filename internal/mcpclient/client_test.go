package mcpclient

// The wire tests of the client machine (Story 7-4 step-01): the REAL HTTP
// rounds over the httptest servers — the request shape (the single
// JSON-RPC message, the pinned revision claim in _meta, the routing
// headers, the Authorization material of the caller), the answer matrix
// (the honest result, the malformed envelope — no result/error, both, the
// id that does not echo, the non-2.0 jsonrpc word, the null members, the
// foreign revision lists and claims, the oversized body against the
// reading budget, the non-200 status, the JSON-RPC error answer), the
// descriptor filtering (the unknown members of a foreign descriptor never
// travel) and the deadline semantics (the unknown outcome of the round
// that outlived its budget). No SDK, no mocks of the protocol: every leg
// answers on the real wire.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// scriptedServer answers through the injected responder and records every
// request it saw (the wire shape assertions read the recording).
type scriptedServer struct {
	server    *httptest.Server
	requests  chan *recordedRequest
	responder func(body string, r *http.Request) (int, string)
}

// recordedRequest is the wire fact of one round (the routing headers, the
// Authorization material and the body of the request).
type recordedRequest struct {
	method         string
	nameHeader     string
	protocolHeader string
	authorization  string
	contentType    string
	body           string
}

func newScriptedServer(t *testing.T, responder func(body string, r *http.Request) (int, string)) *scriptedServer {
	t.Helper()
	world := &scriptedServer{requests: make(chan *recordedRequest, 16), responder: responder}
	world.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		record := &recordedRequest{
			method:         r.Header.Get(HeaderMethod),
			nameHeader:     r.Header.Get(HeaderName),
			protocolHeader: r.Header.Get(HeaderProtocolVersion),
			authorization:  r.Header.Get("Authorization"),
			contentType:    r.Header.Get("Content-Type"),
			body:           string(raw),
		}
		select {
		case world.requests <- record:
		default:
		}
		status, answer := world.responder(string(raw), r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(world.server.Close)
	return world
}

// jsonResult builds the honest answer envelope of one result echoing the
// request id of the wire body.
func jsonResult(id, result string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":%s}`, id, result)
}

// discoverAnswer is the coherent answer of the pinned revision.
const discoverAnswer = `{"protocolVersions":["2026-07-28"],"serverInfo":{"name":"fixture-external","version":"1.0.0"}}`

// echoID extracts the request id of the wire body the way an honest
// server echoes it back.
func echoID(body string) string {
	var probe struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &probe); err != nil {
		return ""
	}
	return probe.ID
}

func TestClientRequestShapeCarriesThePinnedRevisionAndTheCallerAuthorization(t *testing.T) {
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		return http.StatusOK, jsonResult(echoID(body), discoverAnswer)
	})
	client := New(world.server.Client().Transport, 256<<10)
	answer, err := client.Discover(context.Background(), world.server.URL, "Bearer sandbox-wire-key")
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if answer.ServerInfo.Name != "fixture-external" || answer.ServerInfo.Version != "1.0.0" {
		t.Fatalf("serverInfo не прочитана: %+v", answer.ServerInfo)
	}
	if len(answer.ProtocolVersions) != 1 || answer.ProtocolVersions[0] != ProtocolVersion {
		t.Fatalf("protocolVersions не прочитана: %v", answer.ProtocolVersions)
	}
	select {
	case record := <-world.requests:
		if record.method != MethodServerDiscover {
			t.Fatalf("заголовок Mcp-Method: %q", record.method)
		}
		if record.protocolHeader != ProtocolVersion {
			t.Fatalf("заголовок MCP-Protocol-Version: %q", record.protocolHeader)
		}
		if record.authorization != "Bearer sandbox-wire-key" {
			t.Fatalf("Authorization материал вызывателя не доставлен: %q", record.authorization)
		}
		if record.contentType != "application/json" {
			t.Fatalf("Content-Type: %q", record.contentType)
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(record.body), &envelope); err != nil {
			t.Fatalf("тело запроса не JSON: %v", err)
		}
		if envelope["jsonrpc"] != "2.0" || envelope["method"] != MethodServerDiscover {
			t.Fatalf("конверт запроса: %v", envelope)
		}
		meta, ok := envelope["_meta"].(map[string]any)
		if !ok || meta["protocolVersion"] != ProtocolVersion {
			t.Fatalf("_meta заявляет не пиннгованную ревизию: %v", envelope["_meta"])
		}
		if id, ok := envelope["id"].(string); !ok || !strings.HasPrefix(id, "staffy-mcpclient-") {
			t.Fatalf("id раунда не сгенерирован: %v", envelope["id"])
		}
	default:
		t.Fatal("раунд не состоялся на wire")
	}
}

func TestClientWithoutAuthorizationSendsNoHeader(t *testing.T) {
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		return http.StatusOK, jsonResult(echoID(body), discoverAnswer)
	})
	client := New(world.server.Client().Transport, 256<<10)
	if _, err := client.Discover(context.Background(), world.server.URL, ""); err != nil {
		t.Fatalf("discover без авторизации: %v", err)
	}
	select {
	case record := <-world.requests:
		if record.authorization != "" {
			t.Fatalf("пустая авторизация обязана не отправлять заголовок: %q", record.authorization)
		}
	default:
		t.Fatal("раунд не состоялся")
	}
}

func TestClientListToolsFiltersUnknownDescriptorMembers(t *testing.T) {
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		// The foreign descriptor carries unknown members (the passthrough
		// probe) — the filtered decode keeps exactly the known protocol
		// fields and drops the rest (AD-11).
		result := `{"tools":[{"name":"parts.lookup","title":"Поиск детали","description":"Читает каталог","inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]},"outputSchema":{"type":"object","properties":{"found":{"type":"boolean"}}},"annotations":{"readOnlyHint":true},"vendorExtension":{"secret":"не пересылается"}}]}`
		return http.StatusOK, jsonResult(echoID(body), result)
	})
	client := New(world.server.Client().Transport, 256<<10)
	answer, err := client.ListTools(context.Background(), world.server.URL, "Bearer sandbox-wire-key")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(answer.Tools) != 1 {
		t.Fatalf("ожидался один дескриптор, получено %d", len(answer.Tools))
	}
	tool := answer.Tools[0]
	if tool.Name != "parts.lookup" || tool.Title != "Поиск детали" || !tool.Annotations.ReadOnlyHint {
		t.Fatalf("дескриптор отфильтрован неверно: %+v", tool)
	}
	raw, err := json.Marshal(tool)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "vendorExtension") || strings.Contains(string(raw), "не пересылается") {
		t.Fatalf("неизвестные члены дескриптора переслались дальше: %s", raw)
	}
	if len(tool.InputSchema) == 0 || len(tool.OutputSchema) == 0 {
		t.Fatalf("схемы дескриптора не прочитаны: %+v", tool)
	}
}

func TestClientCallToolCarriesTheRoutingCoherence(t *testing.T) {
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		result := `{"content":[{"type":"text","text":"parts.lookup@snapshot"}],"structuredContent":{"found":true}}`
		return http.StatusOK, jsonResult(echoID(body), result)
	})
	client := New(world.server.Client().Transport, 256<<10)
	answer, err := client.CallTool(context.Background(), world.server.URL, "Bearer sandbox-wire-key", "parts.lookup", map[string]any{"query": "BR-001"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if answer.IsError || len(answer.Content) != 1 || answer.Content[0].Text != "parts.lookup@snapshot" {
		t.Fatalf("ответ вызова: %+v", answer)
	}
	if v, ok := answer.StructuredContent["found"].(bool); !ok || !v {
		t.Fatalf("structuredContent: %v", answer.StructuredContent)
	}
	select {
	case record := <-world.requests:
		if record.method != MethodToolsCall || record.nameHeader != "parts.lookup" {
			t.Fatalf("маршрутные заголовки вызова: method=%q name=%q", record.method, record.nameHeader)
		}
		if !strings.Contains(record.body, `"name":"parts.lookup"`) || !strings.Contains(record.body, `"query":"BR-001"`) {
			t.Fatalf("параметры вызова: %s", record.body)
		}
	default:
		t.Fatal("раунд не состоялся")
	}
}

func TestClientAnswerMatrixRefusesTheMalformedEnvelopes(t *testing.T) {
	// The malformed legs ride the id echo of an honest wire server, so the
	// refusal names the intended envelope fact, never the echo.
	cases := []struct {
		name   string
		answer func(id string) string
	}{
		{"ни результата ни ошибки", func(id string) string { return fmt.Sprintf(`{"jsonrpc":"2.0","id":%q}`, id) }},
		{"результат и ошибка одновременно", func(id string) string {
			return fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{},"error":{"code":-32601,"message":"no"}}`, id)
		}},
		{"чужой jsonrpc", func(id string) string { return fmt.Sprintf(`{"jsonrpc":"1.0","id":%q,"result":{}}`, id) }},
		{"нулевые protocolVersions", func(id string) string {
			return fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"protocolVersions":null,"serverInfo":{"name":"s","version":"1"}}}`, id)
		}},
		{"отсутствующая serverInfo", func(id string) string {
			return fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"protocolVersions":["2026-07-28"]}}`, id)
		}},
		{"мусорное тело", func(string) string { return "not-json-at-all" }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
				return http.StatusOK, testCase.answer(echoID(body))
			})
			client := New(world.server.Client().Transport, 256<<10)
			_, err := client.Discover(context.Background(), world.server.URL, "")
			if !errors.Is(err, ErrMalformedResponse) {
				t.Fatalf("ожидался малформед-отказ, получено: %v", err)
			}
		})
	}
}

// The foreign revision legs echo the real request id (an honest wire
// server) — the refusal names the revision fact, not the envelope.
func TestClientForeignRevisionRefusals(t *testing.T) {
	cases := []struct {
		name   string
		result string
	}{{
		name:   "чужая ревизия в списке",
		result: `{"protocolVersions":["2025-11-25"],"serverInfo":{"name":"s","version":"1"}}`,
	}, {
		name:   "чужая ревизия в списке из двух",
		result: `{"protocolVersions":["2025-06-18","2025-11-25"],"serverInfo":{"name":"s","version":"1"}}`,
	}}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
				return http.StatusOK, jsonResult(echoID(body), testCase.result)
			})
			client := New(world.server.Client().Transport, 256<<10)
			_, err := client.Discover(context.Background(), world.server.URL, "")
			if !errors.Is(err, ErrForeignRevision) {
				t.Fatalf("ожидался отказ чужой ревизии, получено: %v", err)
			}
		})
	}
	// The incoherent claim of _meta rides an otherwise honest answer.
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		id := echoID(body)
		return http.StatusOK, fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"_meta":{"protocolVersion":"2025-06-18"},"result":%s}`, id, discoverAnswer)
	})
	client := New(world.server.Client().Transport, 256<<10)
	if _, err := client.Discover(context.Background(), world.server.URL, ""); !errors.Is(err, ErrForeignRevision) {
		t.Fatalf("заявленная чужая ревизия в _meta обязана отказываться, получено: %v", err)
	}
}

func TestClientIdEchoRefusal(t *testing.T) {
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		// The foreign server answers with a DIFFERENT id — the echo check
		// refuses the answer whole.
		return http.StatusOK, jsonResult("not-the-request-id", discoverAnswer)
	})
	client := New(world.server.Client().Transport, 256<<10)
	if _, err := client.Discover(context.Background(), world.server.URL, ""); !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("идентификатор ответа не эхо запроса обязан отказываться, получено: %v", err)
	}
}

func TestClientListAnswerMatrix(t *testing.T) {
	cases := []struct {
		name   string
		answer string
		expect error
	}{
		{"tools null", `{"jsonrpc":"2.0","id":"x","result":{"tools":null}}`, ErrMalformedResponse},
		{"tools отсутствует", `{"jsonrpc":"2.0","id":"x","result":{}}`, ErrMalformedResponse},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
				return http.StatusOK, testCase.answer
			})
			client := New(world.server.Client().Transport, 256<<10)
			_, err := client.ListTools(context.Background(), world.server.URL, "")
			if !errors.Is(err, testCase.expect) {
				t.Fatalf("ожидался отказ %v, получено: %v", testCase.expect, err)
			}
		})
	}
	// The honest empty inventory is the empty array, never an error.
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		return http.StatusOK, jsonResult(echoID(body), `{"tools":[]}`)
	})
	client := New(world.server.Client().Transport, 256<<10)
	answer, err := client.ListTools(context.Background(), world.server.URL, "")
	if err != nil || len(answer.Tools) != 0 {
		t.Fatalf("пустой инвентарь: err=%v tools=%d", err, len(answer.Tools))
	}
}

func TestClientResponseBudgetRefusesWithoutTruncation(t *testing.T) {
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		// The oversized inventory: the body grows past the machine budget —
		// the reading refuses the whole answer (no truncation, no
		// passthrough of the partial material).
		pad := strings.Repeat("x", 2048)
		result := fmt.Sprintf(`{"tools":[{"name":"%s","title":"oversized"}]}`, pad)
		return http.StatusOK, jsonResult(echoID(body), result)
	})
	client := New(world.server.Client().Transport, 1024)
	if _, err := client.ListTools(context.Background(), world.server.URL, ""); !errors.Is(err, ErrResponseBudget) {
		t.Fatalf("сверхбюджетный ответ обязан отказываться целиком, получено: %v", err)
	}
}

func TestClientNon200StatusIsTheHonestFailure(t *testing.T) {
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		return http.StatusInternalServerError, `{"error":"boom"}`
	})
	client := New(world.server.Client().Transport, 256<<10)
	_, err := client.Discover(context.Background(), world.server.URL, "")
	if err == nil || strings.Contains(err.Error(), "boom") {
		t.Fatalf("чужой статус обязан быть поимённым отказом без чужого тела: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("отказ обязан называть статус: %v", err)
	}
}

func TestClientServerErrorAnswerCarriesOnlyTheCode(t *testing.T) {
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		// The foreign server answers a JSON-RPC error with a message of
		// untrusted material — the client carries the CODE only (AD-11).
		return http.StatusOK, fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"error":{"code":-32602,"message":"внутренний материал внешнего сервера"}}`, echoID(body))
	})
	client := New(world.server.Client().Transport, 256<<10)
	_, err := client.Discover(context.Background(), world.server.URL, "")
	var serverError *ServerError
	if !errors.As(err, &serverError) {
		t.Fatalf("ожидалась ошибка JSON-RPC внешнего сервера, получено: %v", err)
	}
	if serverError.Code != -32602 {
		t.Fatalf("код ошибки: %d", serverError.Code)
	}
	if strings.Contains(err.Error(), "внутренний материал внешнего сервера") {
		t.Fatalf("сообщение внешнего сервера переслалось (AD-11): %v", err)
	}
}

func TestClientDeadlineIsTheHonestUnknown(t *testing.T) {
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		time.Sleep(200 * time.Millisecond)
		return http.StatusOK, jsonResult(echoID(body), discoverAnswer)
	})
	client := New(world.server.Client().Transport, 256<<10)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, err := client.Discover(ctx, world.server.URL, "")
	if !errors.Is(err, ErrDeadline) {
		t.Fatalf("исчерпанный бюджет раунда обязан быть честным unknown, получено: %v", err)
	}
}

func TestClientTransportFailureIsTheHonestFailure(t *testing.T) {
	// A closed server: the transport refusal of the round (the honest
	// failed outcome — the round never reached the foreign server).
	world := newScriptedServer(t, func(body string, _ *http.Request) (int, string) {
		return http.StatusOK, jsonResult(echoID(body), discoverAnswer)
	})
	url := world.server.URL
	world.server.Close()
	client := New(http.DefaultTransport, 256<<10)
	_, err := client.Discover(context.Background(), url, "")
	if err == nil || errors.Is(err, ErrDeadline) {
		t.Fatalf("транспортный отказ обязан быть поимённым failed, получено: %v", err)
	}
}
