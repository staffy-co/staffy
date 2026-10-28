package externaltools

// The call machine tests (Story 7-4 step-02): the entry pin from the
// CURRENT derivation (the honest TOCTOU — a vanished or stale row refuses
// by name BEFORE the round), the strict validation of the arguments
// (every refusal names the field), the fail-closed output (an undeclared
// or mistyped key of the foreign answer never travels — AD-11) and the
// credential-agnostic fact (the Authorization material and the endpoint
// travel to the round verbatim and never anywhere else).

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// scriptedTransport records one round and scripts its answer.
type scriptedTransport struct {
	calls  []recordedCall
	answer map[string]any
	err    error
}

type recordedCall struct {
	endpoint      string
	authorization string
	toolName      string
	arguments     map[string]any
}

func (t *scriptedTransport) CallTool(_ context.Context, endpoint, authorization, toolName string, arguments map[string]any) (map[string]any, error) {
	t.calls = append(t.calls, recordedCall{endpoint: endpoint, authorization: authorization, toolName: toolName, arguments: arguments})
	if t.err != nil {
		return nil, t.err
	}
	if t.answer == nil {
		return map[string]any{}, nil
	}
	return t.answer, nil
}

// callsFacts answers one inventory of the verified snapshot.
func callsFacts(t *testing.T) []ConnectionInventory {
	t.Helper()
	facts := inventoryFacts(t, connectorDomain.CapStateVerified)
	// The inventory carries ONE projectable read tool.
	facts.Report.Evidence.RedactedResult = "Раунды выполнены; delivered." + "\nИнвентарь: " + `{"server":{"name":"fixture-external","version":"1.0.0"},"tools":[{"name":"parts.lookup","title":"Поиск детали","description":"Читает каталог","readOnlyHint":true,"inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]},"outputSchema":{"type":"object","properties":{"found":{"type":"boolean"},"count":{"type":"integer","minimum":0}},"required":["found"]}}]}`
	return []ConnectionInventory{facts}
}

func callsScope(t *testing.T) tenancy.Context {
	t.Helper()
	scope, err := tenancy.New(id.MustParse("01994f64-6c00-7000-8000-0000000000a2"), id.MustParse(ladderTenant))
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func TestCallsHappyPathPinsValidatesAndReturnsTheTypedAnswer(t *testing.T) {
	transport := &scriptedTransport{answer: map[string]any{"found": true, "count": float64(3)}}
	machine := NewCalls(&staticFacts{rows: callsFacts(t)}, transport)
	answer, err := machine.Call(context.Background(), callsScope(t), id.MustParse("01994f64-6c00-7000-8000-00000000e702"), "parts.lookup",
		map[string]any{"query": "BR-001"}, "https://fixture.external.example", "Bearer sandbox-caller-key")
	if err != nil {
		t.Fatalf("вызов: %v", err)
	}
	if answer["found"] != true || answer["count"] != float64(3) {
		t.Fatalf("типизированный ответ: %v", answer)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("ровно один раунд: %d", len(transport.calls))
	}
	round := transport.calls[0]
	if round.endpoint != "https://fixture.external.example" || round.authorization != "Bearer sandbox-caller-key" {
		t.Fatalf("материал вызывателя обязан доходить до раунда дословно: %+v", round)
	}
	if round.toolName != "parts.lookup" || round.arguments["query"] != "BR-001" {
		t.Fatalf("раунд обязан нести инструмент и аргументы: %+v", round)
	}
}

func TestCallsArgumentValidationRefusesByFieldName(t *testing.T) {
	machine := NewCalls(&staticFacts{rows: callsFacts(t)}, &scriptedTransport{})
	connectionID := id.MustParse("01994f64-6c00-7000-8000-00000000e702")
	cases := []struct {
		name      string
		arguments map[string]any
		word      string
	}{
		{"неизвестный ключ", map[string]any{"query": "x", "secret.internal": "y"}, "не объявлен"},
		{"пропущен обязательный", map[string]any{}, "обязательное поле query отсутствует"},
		{"чужой тип значения", map[string]any{"query": 7}, "обязано быть строкой"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := machine.Call(context.Background(), callsScope(t), connectionID, "parts.lookup", testCase.arguments, "https://fixture.external.example", "Bearer k")
			if !errors.Is(err, ErrArguments) {
				t.Fatalf("ожидался отказ аргументов: %v", err)
			}
			if !strings.Contains(err.Error(), testCase.word) {
				t.Fatalf("отказ обязан называть поле: %v", err)
			}
		})
	}
}

func TestCallsOutputValidationFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		answer map[string]any
		word   string
	}{
		{"незадекларированный ключ", map[string]any{"found": true, "secret.internal": "материал адаптера"}, "не объявлен"},
		{"чужой тип выхода", map[string]any{"found": "yes"}, "обязано быть булевым"},
		{"пропущен обязательный выход", map[string]any{}, "обязательное поле found отсутствует"},
		{"дробное целое", map[string]any{"found": true, "count": 1.5}, "целым числом"},
	}
	connectionID := id.MustParse("01994f64-6c00-7000-8000-00000000e702")
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			transport := &scriptedTransport{answer: testCase.answer}
			machine := NewCalls(&staticFacts{rows: callsFacts(t)}, transport)
			answer, err := machine.Call(context.Background(), callsScope(t), connectionID, "parts.lookup", map[string]any{"query": "x"}, "https://fixture.external.example", "Bearer k")
			if !errors.Is(err, ErrOutputRefused) {
				t.Fatalf("ожидался fail-closed отказ выхода: %v", err)
			}
			if !strings.Contains(err.Error(), testCase.word) {
				t.Fatalf("отказ выхода обязан называть факт: %v", err)
			}
			if !strings.Contains(err.Error(), "выход не переслан") {
				t.Fatalf("внутренний материал не пересылается: %v", err)
			}
			if answer != nil {
				t.Fatalf("материал недоверенного выхода не возвращается: %v", answer)
			}
		})
	}
}

func TestCallsPinComesFromTheCurrentDerivation(t *testing.T) {
	connectionID := id.MustParse("01994f64-6c00-7000-8000-00000000e702")
	transport := &scriptedTransport{}
	// The stale anchor: the row vanished before the round (the honest TOCTOU).
	staleFacts := callsFacts(t)
	staleFacts[0].Report.State = connectorDomain.CapStateStale
	machine := NewCalls(&staticFacts{rows: staleFacts}, transport)
	_, err := machine.Call(context.Background(), callsScope(t), connectionID, "parts.lookup", map[string]any{"query": "x"}, "https://fixture.external.example", "Bearer k")
	if !errors.Is(err, ErrEntryNotFound) || !strings.Contains(err.Error(), "исчезли") {
		t.Fatalf("исчезнувшая запись обязана отказываться до раунда: %v", err)
	}
	// The unknown tool name of the live snapshot.
	machine = NewCalls(&staticFacts{rows: callsFacts(t)}, transport)
	_, err = machine.Call(context.Background(), callsScope(t), connectionID, "parts.ghost", map[string]any{}, "https://fixture.external.example", "Bearer k")
	if !errors.Is(err, ErrEntryNotFound) || !strings.Contains(err.Error(), "parts.ghost") {
		t.Fatalf("неизвестный инструмент обязан отказываться поимённо: %v", err)
	}
	// The rejected row (outside the projection subset) never calls.
	rejectedFacts := callsFacts(t)
	rejectedFacts[0].Report.Evidence.RedactedResult = "Раунды выполнены; delivered." + "\nИнвентарь: " + `{"server":{"name":"s","version":"1"},"tools":[{"name":"parts.mutate","title":"Мутация","description":"Меняет","readOnlyHint":false,"inputSchema":{"type":"object","properties":{}},"outputSchema":{"type":"object"}}]}`
	machine = NewCalls(&staticFacts{rows: rejectedFacts}, transport)
	_, err = machine.Call(context.Background(), callsScope(t), connectionID, "parts.mutate", map[string]any{}, "https://fixture.external.example", "Bearer k")
	if !errors.Is(err, ErrEntryNotAdmitted) || !strings.Contains(err.Error(), "AR-30") {
		t.Fatalf("недопущенная запись обязана отказываться с причиной: %v", err)
	}
	// The unknown connection: the honest words.
	_, err = machine.Call(context.Background(), callsScope(t), id.MustParse("01994f64-6c00-7000-8000-0000000000ff"), "parts.lookup", map[string]any{}, "https://fixture.external.example", "Bearer k")
	if !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("неизвестное подключение: %v", err)
	}
	if len(transport.calls) != 0 {
		t.Fatalf("раунды до пина невозможны: %d", len(transport.calls))
	}
}

func TestCallsTransportErrorsTravelHonestly(t *testing.T) {
	connectionID := id.MustParse("01994f64-6c00-7000-8000-00000000e702")
	// The deadline of the caller's context is the honest transport failure
	// of the round (the machine passes the context through and never
	// retries blind).
	deadline := errors.New("mcpclient: бюджет раунда исчерпан до завершения")
	machine := NewCalls(&staticFacts{rows: callsFacts(t)}, &scriptedTransport{err: deadline})
	_, err := machine.Call(context.Background(), callsScope(t), connectionID, "parts.lookup", map[string]any{"query": "x"}, "https://fixture.external.example", "Bearer k")
	if err == nil || !strings.Contains(err.Error(), "бюджет раунда исчерпан") {
		t.Fatalf("ошибка транспорта обязана доходить честно: %v", err)
	}
}

// staticFacts answers one fixed set of rows (the derivation material of
// the call tests).
type staticFacts struct {
	rows []ConnectionInventory
}

func (f *staticFacts) InventoryFacts(context.Context, tenancy.Context) ([]ConnectionInventory, error) {
	return f.rows, nil
}

// Compile-time anchors of the test vocabulary.
var (
	_ = time.Time{}
	_ = connectorDomain.CapStateVerified
)
