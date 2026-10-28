package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/tools"
)

// fixtureSource serves the scripted contracts of the method tests (the
// fixture world of the journeys carries its own vertical words — the
// production source is the registry adapter).
type fixtureSource struct {
	contracts []tools.ToolContract
}

func (f fixtureSource) List() []tools.ToolContract {
	return append([]tools.ToolContract{}, f.contracts...)
}
func (f fixtureSource) Get(toolID, version string) (tools.ToolContract, bool) {
	for _, contract := range f.contracts {
		if contract.ID == toolID && contract.Version == version {
			return contract, true
		}
	}
	return tools.ToolContract{}, false
}

// adapterRegistryFake registers the scripted adapters by pin.
type adapterRegistryFake struct {
	adapters map[string]Adapter
	hook     func(tenant id.UUID, arguments map[string]any)
	latency  time.Duration
	output   map[string]any
	err      error
	mu       sync.Mutex
	lastArgs map[string]any
}

func (r *adapterRegistryFake) Adapter(toolID, version string) (Adapter, bool) {
	adapter, ok := r.adapters[toolID+"@"+version]
	if !ok {
		return nil, false
	}
	return adapter, true
}

// scriptedAdapter executes the scripted behavior and records the call.
func (r *adapterRegistryFake) scriptedAdapter() Adapter {
	return func(ctx context.Context, tenant id.UUID, arguments map[string]any) (map[string]any, error) {
		r.mu.Lock()
		r.lastArgs = arguments
		hook := r.hook
		r.mu.Unlock()
		if hook != nil {
			hook(tenant, arguments)
		}
		if r.latency > 0 {
			select {
			case <-time.After(r.latency):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if r.err != nil {
			return nil, r.err
		}
		if r.output == nil {
			return map[string]any{}, nil
		}
		return r.output, nil
	}
}

// fixtureContract builds one contract of the fixture world (the vertical
// words live in the TEST contour only — the production registry carries
// its own artifacts).
func fixtureContract(contractID, version, adapterStatus, determinism string, dependency *tools.ToolDependency) tools.ToolContract {
	return tools.ToolContract{
		ID: contractID, Version: version,
		Name:             "Демонстрационный инструмент",
		Purpose:          "Проверяет контракт поверхности в тест-контуре",
		DeterminismClass: determinism,
		AdapterStatus:    adapterStatus,
		InputSchema: map[string]tools.ToolSchemaField{
			"query.text":   {Label: "Запрос", Type: "string"},
			"query.limit":  {Label: "Лимит", Type: "integer", Min: 1, Max: 10, Optional: true},
			"query.strict": {Label: "Строгость", Type: "boolean", Optional: true},
		},
		OutputSchema: map[string]tools.ToolSchemaField{
			"result.word": {Label: "Слово", Type: "string", Values: []string{"да", "нет"}},
		},
		Owner:       "fixture",
		EvidenceRef: "fixture/spec/7-3",
		Dependency:  dependency,
	}
}

// newMethodService wires the fixture world of the method tests.
func newMethodService(source ToolSource, registry *adapterRegistryFake, gate DependencyGate) *Service {
	return New(membershipFake{}, source, registry, gate, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// callEnvelope builds one tools/call request of the given tool.
func callEnvelope(t *testing.T, toolID string, arguments string) []byte {
	t.Helper()
	return []byte(`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"` + toolID + `","arguments":` + arguments + `},"_meta":{"protocolVersion":"` + ProtocolVersion + `"}}`)
}

// callOutcome runs one call of the given tool through the full pipeline.
func callOutcome(t *testing.T, service *Service, toolID, arguments string) Outcome {
	t.Helper()
	owner := identityDomain.Principal{ID: testOwnerID}
	outcome, err := service.Handle(context.Background(), owner, testTenantID, callEnvelope(t, toolID, arguments), HeaderClaims{Method: MethodToolsCall, Name: toolID, ProtocolVersion: ProtocolVersion})
	if err != nil {
		t.Fatalf("вызов %s не обязан падать ошибкой сервиса: %v", toolID, err)
	}
	return outcome
}

// decodeResult reads the CallToolResult of an outcome body (the result
// member IS the call answer).
func decodeResult(t *testing.T, outcome Outcome) CallToolResult {
	t.Helper()
	var result CallToolResult
	if err := json.Unmarshal(outcome.Body.Result, &result); err != nil {
		t.Fatalf("результат не читается: %v (сырое: %s)", err, outcome.Body.Result)
	}
	return result
}

// TestServerDiscoverAnswersTheFacts: the negotiation answer carries the
// exact supported list, the factual capabilities and the server identity
// — nothing the platform does not carry is ever claimed.
func TestServerDiscoverAnswersTheFacts(t *testing.T) {
	service := newMethodService(fixtureSource{}, &adapterRegistryFake{}, gateFake{})
	owner := identityDomain.Principal{ID: testOwnerID}
	outcome, err := service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodServerDiscover), coherentClaims(MethodServerDiscover))
	if err != nil || outcome.Status != 200 {
		t.Fatalf("discover обязан отвечать 200, получено %+v err=%v", outcome, err)
	}
	var result DiscoverResult
	if err := json.Unmarshal(outcome.Body.Result, &result); err != nil {
		t.Fatalf("результат discover не разбирается: %v (сырое: %s)", err, outcome.Body.Result)
	}
	payload := outcome.Body.Result
	if len(result.ProtocolVersions) != 1 || result.ProtocolVersions[0] != ProtocolVersion {
		t.Fatalf("список ревизий обязан быть ровно [%s]: %v", ProtocolVersion, result.ProtocolVersions)
	}
	if result.Capabilities.Tools.ListChanged {
		t.Fatal("listChanged обязан быть false: экспозиция зависит от живых доказательств")
	}
	if result.ServerInfo.Name != ServerName || result.ServerInfo.Version != ServerVersion {
		t.Fatalf("идентичность сервера обязана быть %s/%s, получено %s/%s", ServerName, ServerVersion, result.ServerInfo.Name, result.ServerInfo.Version)
	}
	// The unclaimed capabilities never appear on the wire.
	for _, unclaimed := range []string{"resources", "prompts", "sampling", "elicitation"} {
		if strings.Contains(string(payload), `"`+unclaimed+`"`) {
			t.Fatalf("незаявленная capability %q не обязана появляться на проводе: %s", unclaimed, string(payload))
		}
	}
}

// TestToolsListGateFourBranches: the exposure is the intersection of the
// facts — the implemented artifact, the runtime adapter, the live
// dependency — and the write-class contract is structurally invisible
// (the layer checks the platform dictionary itself, never trusting the
// source's words).
func TestToolsListGateFourBranches(t *testing.T) {
	live := fixtureContract("fixture.live-tool", "1.0.0", "implemented", "deterministic_read", nil)
	unimplemented := fixtureContract("fixture.unimplemented-tool", "1.0.0", "not_implemented", "deterministic_read", nil)
	unregistered := fixtureContract("fixture.unregistered-tool", "1.0.0", "implemented", "deterministic_read", nil)
	deadDependency := fixtureContract("fixture.dead-dependency-tool", "1.0.0", "implemented", "deterministic_read", &tools.ToolDependency{ConnectorType: "catalog"})
	writeClass := fixtureContract("fixture.write-class-tool", "1.0.0", "implemented", "write", nil)
	source := fixtureSource{contracts: []tools.ToolContract{live, unimplemented, unregistered, deadDependency, writeClass}}

	registry := &adapterRegistryFake{adapters: map[string]Adapter{}}
	// ONLY the live tool carries the runtime adapter: the unregistered
	// branch stays honestly unregistered, the write-class tool is refused
	// BEFORE the adapter fact matters (the dictionary check runs first).
	registry.adapters[live.ID+"@"+live.Version] = registry.scriptedAdapter()
	service := newMethodService(source, registry, gateFake{live: true, word: "verified"})

	owner := identityDomain.Principal{ID: testOwnerID}
	outcome, err := service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodToolsList), coherentClaims(MethodToolsList))
	if err != nil || outcome.Status != 200 {
		t.Fatalf("tools/list обязан отвечать 200, получено %+v err=%v", outcome, err)
	}
	var listResult ListToolsResult
	if err := json.Unmarshal(outcome.Body.Result, &listResult); err != nil {
		t.Fatalf("результат tools/list не читается: %v (сырое: %s)", err, outcome.Body.Result)
	}
	if len(listResult.Tools) != 1 {
		t.Fatalf("гейт обязан экспонировать ровно один инструмент, получено %+v", listResult.Tools)
	}
	exposed := listResult.Tools[0]
	if exposed.Name != live.ID {
		t.Fatalf("экспонируется неверный инструмент: %+v", exposed)
	}
	if !exposed.Annotations.ReadOnlyHint {
		t.Fatal("readOnlyHint обязан быть фактом платформенного словаря")
	}

	// The dead dependency branch: the gate word closes the dependent tool
	// (a self-contained contract has no dependency to close).
	deadRegistry := &adapterRegistryFake{adapters: map[string]Adapter{}}
	deadRegistry.adapters[deadDependency.ID+"@"+deadDependency.Version] = deadRegistry.scriptedAdapter()
	deadGate := newMethodService(fixtureSource{contracts: []tools.ToolContract{deadDependency}}, deadRegistry, gateFake{live: false, word: "stale"})
	outcome, err = deadGate.Handle(context.Background(), owner, testTenantID, discoverBody(MethodToolsList), coherentClaims(MethodToolsList))
	if err != nil {
		t.Fatalf("закрытый гейт не обязан падать ошибкой: %v", err)
	}
	listResult = ListToolsResult{}
	if err := json.Unmarshal(outcome.Body.Result, &listResult); err != nil {
		t.Fatalf("результат не читается: %v", err)
	}
	if len(listResult.Tools) != 0 || listResult.Tools == nil {
		t.Fatalf("честный пустой список обязателен (массив, не null), получено %+v", listResult.Tools)
	}
}

// TestToolsListHonestEmptyAG4: the production composition registers ZERO
// adapters — the honest empty array of the AG-4 state (a state, not a
// defect).
func TestToolsListHonestEmptyAG4(t *testing.T) {
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}}
	// The real registry contracts, all honestly not_implemented: the
	// source fake mirrors the production shape.
	contracts := []tools.ToolContract{fixtureContract("fixture.artifact", "1.0.0", "not_implemented", "deterministic_read", nil)}
	service := newMethodService(fixtureSource{contracts: contracts}, registry, gateFake{live: true, word: "verified"})
	owner := identityDomain.Principal{ID: testOwnerID}
	outcome, err := service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodToolsList), coherentClaims(MethodToolsList))
	if err != nil {
		t.Fatalf("пустой мир не обязан падать: %v", err)
	}
	if !strings.Contains(string(outcome.Body.Result), `"tools":[]`) {
		t.Fatalf("пустой список обязан быть пустым массивом, получено %s", outcome.Body.Result)
	}
}

// TestToolsCallHappyPath: the validated call answers the typed output
// with the contract pin in the content; the adapter receives the tenant
// of the SESSION and the typed values (the integers arrive as whole
// numbers).
func TestToolsCallHappyPath(t *testing.T) {
	contract := fixtureContract("fixture.live-tool", "1.0.0", "implemented", "deterministic_read", nil)
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}, output: map[string]any{"result.word": "да"}}
	registry.adapters[contract.ID+"@"+contract.Version] = registry.scriptedAdapter()
	service := newMethodService(fixtureSource{contracts: []tools.ToolContract{contract}}, registry, gateFake{live: true, word: "verified"})

	outcome := callOutcome(t, service, contract.ID, `{"query.text":"проверка","query.limit":5}`)
	if outcome.Status != 200 {
		t.Fatalf("валидный вызов обязан отвечать 200, получено %d", outcome.Status)
	}
	result := decodeResult(t, outcome)
	if result.IsError {
		t.Fatalf("валидный вызов не обязан нести isError: %+v", result)
	}
	if len(result.Content) != 1 || result.Content[0].Type != "text" || result.Content[0].Text != contract.ID+"@"+contract.Version {
		t.Fatalf("content обязан нести пин контракта, получено %+v", result.Content)
	}
	if result.StructuredContent["result.word"] != "да" {
		t.Fatalf("structuredContent обязан нести типизированный выход, получено %+v", result.StructuredContent)
	}
	// The session tenant traveled as the adapter's tenant, the typed
	// values as the arguments (the integer as the whole number).
	if registry.lastArgs["query.limit"] != int64(5) {
		t.Fatalf("целочисленный аргумент обязан приезжать целым, получено %T(%v)", registry.lastArgs["query.limit"], registry.lastArgs["query.limit"])
	}
}

// TestToolsCallValidationNamesTheFields: every refusal of the closed
// input schema carries the field name (the model self-corrects in one
// round) — the unknown key, the type, the enum, the bounds and the
// required list.
func TestToolsCallValidationNamesTheFields(t *testing.T) {
	contract := fixtureContract("fixture.live-tool", "1.0.0", "implemented", "deterministic_read", nil)
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}}
	registry.adapters[contract.ID+"@"+contract.Version] = registry.scriptedAdapter()
	service := newMethodService(fixtureSource{contracts: []tools.ToolContract{contract}}, registry, gateFake{live: true, word: "verified"})

	refusal := func(arguments string, words ...string) {
		t.Helper()
		outcome := callOutcome(t, service, contract.ID, arguments)
		if outcome.Status != 200 {
			t.Fatalf("отказ валидации — это 200 tool execution error, получено %d", outcome.Status)
		}
		result := decodeResult(t, outcome)
		if !result.IsError {
			t.Fatalf("отказ валидации обязан нести isError: %+v", result)
		}
		if result.StructuredContent != nil {
			t.Fatalf("отказ не обязан нести structuredContent: %+v", result)
		}
		joined := result.Content[0].Text
		for _, word := range words {
			if !strings.Contains(joined, word) {
				t.Fatalf("отказ %q не называет %q", joined, word)
			}
		}
	}

	refusal(`{"query.text":"x","query.unknown":1}`, "query.unknown", "неизвестный аргумент")
	refusal(`{"query.text":7}`, "query.text", "string")
	refusal(`{"query.text":"x","query.limit":"пять"}`, "query.limit", "integer")
	refusal(`{"query.text":"x","query.limit":2.5}`, "дробн")
	refusal(`{"query.text":"x","query.limit":99}`, "query.limit", "выше границы")
	refusal(`{}`, "query.text", "обязательный аргумент")
	refusal(`{"query.text":"x","query.strict":"да"}`, "query.strict", "boolean")
}

// TestToolsCallGateRefusalsNameTheReasons: the unknown tool, the
// unimplemented artifact, the unregistered adapter and the dead
// dependency each answer the honest tool execution error with the named
// reason.
func TestToolsCallGateRefusalsNameTheReasons(t *testing.T) {
	live := fixtureContract("fixture.live-tool", "1.0.0", "implemented", "deterministic_read", nil)
	unimplemented := fixtureContract("fixture.unimplemented-tool", "1.0.0", "not_implemented", "deterministic_read", nil)
	unregistered := fixtureContract("fixture.unregistered-tool", "1.0.0", "implemented", "deterministic_read", nil)
	deadDependency := fixtureContract("fixture.dead-dependency-tool", "1.0.0", "implemented", "deterministic_read", &tools.ToolDependency{ConnectorType: "catalog"})
	source := fixtureSource{contracts: []tools.ToolContract{live, unimplemented, unregistered, deadDependency}}
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}}
	registry.adapters[live.ID+"@"+live.Version] = registry.scriptedAdapter()
	registry.adapters[deadDependency.ID+"@"+deadDependency.Version] = registry.scriptedAdapter()
	// The unregistered tool carries NO adapter: the honest branch of the
	// composition fact. The gate word is dead for the dependency branch.
	service := newMethodService(source, registry, gateFake{live: false, word: "stale"})

	refusal := func(toolID, reasonPart string) {
		t.Helper()
		outcome := callOutcome(t, service, toolID, `{"query.text":"x"}`)
		result := decodeResult(t, outcome)
		if !result.IsError || !strings.Contains(result.Content[0].Text, reasonPart) {
			t.Fatalf("отказ %s обязан называть %q, получено %+v", toolID, reasonPart, result.Content)
		}
	}
	refusal("fixture.ghost-tool", "не существует в реестре")
	refusal(unimplemented.ID, refusalNotImplemented)
	refusal(unregistered.ID, refusalNoAdapter)
	refusal(deadDependency.ID, "зависимость не жива")
	refusal(deadDependency.ID, "stale")
}

// TestToolsCallOutputFailClosed: the undeclared key of the adapter's
// output NEVER travels to the client — the refusal names the schema
// fact and the structured content stays absent (AC-2).
func TestToolsCallOutputFailClosed(t *testing.T) {
	contract := fixtureContract("fixture.live-tool", "1.0.0", "implemented", "deterministic_read", nil)
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}, output: map[string]any{"result.word": "да", "secret.material": "внутренний секрет"}}
	registry.adapters[contract.ID+"@"+contract.Version] = registry.scriptedAdapter()
	service := newMethodService(fixtureSource{contracts: []tools.ToolContract{contract}}, registry, gateFake{live: true, word: "verified"})

	outcome := callOutcome(t, service, contract.ID, `{"query.text":"x"}`)
	result := decodeResult(t, outcome)
	if !result.IsError {
		t.Fatalf("undeclared выход обязан быть isError: %+v", result)
	}
	if !strings.Contains(result.Content[0].Text, "secret.material") && !strings.Contains(result.Content[0].Text, "незадекларированный ключ") {
		t.Fatalf("отказ обязан называть незадекларированный ключ, получено %q", result.Content[0].Text)
	}
	if strings.Contains(string(outcome.Body.Result), "внутренний секрет") {
		t.Fatal("внутренний материал адаптера НЕ обязан попадать на провод")
	}
	if result.StructuredContent != nil {
		t.Fatalf("отказ выхода не обязан нести structuredContent: %+v", result.StructuredContent)
	}
}

// TestToolsCallBudgetExhausted: the declared execution budget of the
// call refuses the overrunning adapter with the named reason.
func TestToolsCallBudgetExhausted(t *testing.T) {
	contract := fixtureContract("fixture.live-tool", "1.0.0", "implemented", "deterministic_read", nil)
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}, latency: 150 * time.Millisecond}
	registry.adapters[contract.ID+"@"+contract.Version] = registry.scriptedAdapter()
	service := newMethodService(fixtureSource{contracts: []tools.ToolContract{contract}}, registry, gateFake{live: true, word: "verified"}).WithCallBudget(30 * time.Millisecond)

	outcome := callOutcome(t, service, contract.ID, `{"query.text":"x"}`)
	result := decodeResult(t, outcome)
	if !result.IsError || !strings.Contains(result.Content[0].Text, "бюджет выполнения исчерпан") {
		t.Fatalf("превышение бюджета обязано получить поимённый отказ, получено %+v", result.Content)
	}
}

// TestToolsCallRequiresMcpNameCoherence: the call refuses the missing
// and the desynced Mcp-Name routing header with the 400 invalid request.
func TestToolsCallRequiresMcpNameCoherence(t *testing.T) {
	contract := fixtureContract("fixture.live-tool", "1.0.0", "implemented", "deterministic_read", nil)
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}}
	registry.adapters[contract.ID+"@"+contract.Version] = registry.scriptedAdapter()
	service := newMethodService(fixtureSource{contracts: []tools.ToolContract{contract}}, registry, gateFake{live: true, word: "verified"})

	owner := identityDomain.Principal{ID: testOwnerID}
	claims := HeaderClaims{Method: MethodToolsCall, ProtocolVersion: ProtocolVersion}
	outcome, err := service.Handle(context.Background(), owner, testTenantID, callEnvelope(t, contract.ID, `{"query.text":"x"}`), claims)
	if err != nil || outcome.Status != 400 || !strings.Contains(outcome.Body.Error.Message, "Mcp-Name") {
		t.Fatalf("отсутствие Mcp-Name обязано получать 400, получено %+v err=%v", outcome, err)
	}
	claims.Name = "другой-инструмент"
	outcome, err = service.Handle(context.Background(), owner, testTenantID, callEnvelope(t, contract.ID, `{"query.text":"x"}`), claims)
	if err != nil || outcome.Status != 400 || !strings.Contains(outcome.Body.Error.Message, "расходится") {
		t.Fatalf("рассинхрон Mcp-Name обязан получать 400, получено %+v err=%v", outcome, err)
	}
}

// TestToolsCallNewestVersionWins: the list and the call both resolve the
// NEWEST version of the id (the honest semver-core comparison), and the
// content carries the executed pin.
func TestToolsCallNewestVersionWins(t *testing.T) {
	oldVersion := fixtureContract("fixture.live-tool", "1.0.0", "implemented", "deterministic_read", nil)
	newVersion := fixtureContract("fixture.live-tool", "1.2.0", "implemented", "deterministic_read", nil)
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}, output: map[string]any{"result.word": "да"}}
	registry.adapters[oldVersion.ID+"@"+oldVersion.Version] = registry.scriptedAdapter()
	registry.adapters[newVersion.ID+"@"+newVersion.Version] = registry.scriptedAdapter()
	service := newMethodService(fixtureSource{contracts: []tools.ToolContract{oldVersion, newVersion}}, registry, gateFake{live: true, word: "verified"})

	owner := identityDomain.Principal{ID: testOwnerID}
	outcome, err := service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodToolsList), coherentClaims(MethodToolsList))
	if err != nil {
		t.Fatalf("list не обязан падать: %v", err)
	}
	if !strings.Contains(string(outcome.Body.Result), "@1.2.0") || strings.Contains(string(outcome.Body.Result), "@1.0.0") {
		t.Fatalf("list обязан экспонировать новейшую версию, получено %s", outcome.Body.Result)
	}
	outcome = callOutcome(t, service, newVersion.ID, `{"query.text":"x"}`)
	result := decodeResult(t, outcome)
	if result.Content[0].Text != "fixture.live-tool@1.2.0" {
		t.Fatalf("вызов обязан пиннуть новейшую версию, получено %q", result.Content[0].Text)
	}
}

// TestStructuralWriteImpossibility: the exposed set is the subset of the
// registry ∩ deterministic_read ∩ implemented — a write-class contract
// is structurally invisible to BOTH the list and the call, whatever the
// source, the adapter registry or the dependency gate claim (the
// write-impossibility of AD-6/AD-7 is machine-checked at this layer).
func TestStructuralWriteImpossibility(t *testing.T) {
	writeClass := fixtureContract("fixture.write-class-tool", "1.0.0", "implemented", "write", nil)
	alsoRead := fixtureContract("fixture.read-tool", "1.0.0", "implemented", "deterministic_read", nil)
	source := fixtureSource{contracts: []tools.ToolContract{writeClass, alsoRead}}
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}}
	registry.adapters[writeClass.ID+"@"+writeClass.Version] = registry.scriptedAdapter()
	registry.adapters[alsoRead.ID+"@"+alsoRead.Version] = registry.scriptedAdapter()
	service := newMethodService(source, registry, gateFake{live: true, word: "verified"})

	owner := identityDomain.Principal{ID: testOwnerID}
	outcome, err := service.Handle(context.Background(), owner, testTenantID, discoverBody(MethodToolsList), coherentClaims(MethodToolsList))
	if err != nil {
		t.Fatalf("list не обязан падать: %v", err)
	}
	if strings.Contains(string(outcome.Body.Result), writeClass.ID) {
		t.Fatalf("write-класс НЕ обязан быть видим в списке: %s", outcome.Body.Result)
	}
	outcome = callOutcome(t, service, writeClass.ID, `{"query.text":"x"}`)
	result := decodeResult(t, outcome)
	if !result.IsError || !strings.Contains(result.Content[0].Text, refusalWrongClass) {
		t.Fatalf("write-класс обязан получать структурный отказ, получено %+v", result.Content)
	}
}

// TestToolsCallAuditLine: every tools/call writes one append-only audit
// line — the code, the tenant, the principal, the pinned tool, the
// verdict with the reason and the correlation id.
func TestToolsCallAuditLine(t *testing.T) {
	contract := fixtureContract("fixture.live-tool", "1.0.0", "implemented", "deterministic_read", nil)
	registry := &adapterRegistryFake{adapters: map[string]Adapter{}, output: map[string]any{"result.word": "да"}}
	registry.adapters[contract.ID+"@"+contract.Version] = registry.scriptedAdapter()
	var buffer bytes.Buffer
	service := New(membershipFake{}, fixtureSource{contracts: []tools.ToolContract{contract}}, registry, gateFake{live: true, word: "verified"}, slog.New(slog.NewJSONHandler(&buffer, nil)))

	callOutcome(t, service, contract.ID, `{"query.text":"x"}`)
	line := buffer.String()
	for _, word := range []string{"mcp_tool_call", "verdict", "ok", contract.ID + "@" + contract.Version, testTenantID.String(), testOwnerID.String()} {
		if !strings.Contains(line, word) {
			t.Fatalf("аудит-строка обязана нести %q, получено %s", word, line)
		}
	}
	// The refused call appends the refused verdict with the reason.
	buffer.Reset()
	callOutcome(t, service, contract.ID, `{"unknown.key":1}`)
	line = buffer.String()
	for _, word := range []string{"refused", "unknown.key"} {
		if !strings.Contains(line, word) {
			t.Fatalf("аудит-строка отказа обязана нести %q, получено %s", word, line)
		}
	}
}

// The compile-time pins of the fakes of the method tests.
var (
	_ ToolSource      = fixtureSource{}
	_ AdapterRegistry = (*adapterRegistryFake)(nil)
)
