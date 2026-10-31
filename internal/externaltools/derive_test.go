package externaltools

// The derivation tests (Story 7-4 step-02): the dictionary is a pure
// function of the facts — the entries exist exactly while the
// query.tools-list report carries the APPLICABLE evidence, and every
// stale anchor (the rotation, the config save, the build change, the TTL
// expiry — the report state of the connector machine) makes the entries
// DISAPPEAR with the honest reason words. A rejected descriptor becomes a
// rejected ROW (never a silence); a draft, failed or revoked connection
// carries its guard words.

import (
	"strings"
	"testing"
	"time"

	connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

const inventoryJSON = "\nИнвентарь: " + `{"server":{"name":"fixture-external","version":"1.0.0"},"tools":[{"name":"parts.lookup","title":"Поиск детали","description":"Читает каталог","readOnlyHint":true,"inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]},"outputSchema":{"type":"object","properties":{"found":{"type":"boolean"}},"required":["found"]}},{"name":"parts.mutate","title":"Мутация","description":"Меняет каталог","readOnlyHint":false,"inputSchema":{"type":"object","properties":{"x":{"type":"string"}}},"outputSchema":{"type":"object"}},{"name":"parts.search","title":"Поиск с дробью","description":"Число с дробью","readOnlyHint":true,"inputSchema":{"type":"object","properties":{"ratio":{"type":"number"}}},"outputSchema":{"type":"object"}}]}`

func inventoryFacts(t *testing.T, state string) ConnectionInventory {
	t.Helper()
	report := connectorDomain.CapabilityReport{
		CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: "query.tools-list", Title: "Словарь инструментов сервера"},
		State:                 state,
	}
	if state == connectorDomain.CapStateVerified {
		checked := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
		expires := checked.Add(24 * time.Hour)
		report.Evidence = &connectorDomain.CapabilityEvidenceSummary{
			EvidenceID:     id.MustParse("01994f64-6c00-7000-8000-00000000e701"),
			Outcome:        connectorDomain.EvidenceVerified,
			CheckedAt:      checked,
			ExpiresAt:      expires,
			RedactedResult: "Раунды server/discover + tools/list выполнены: read-tools: 3; delivered." + inventoryJSON,
		}
	}
	return ConnectionInventory{
		ConnectionID:    id.MustParse("01994f64-6c00-7000-8000-00000000e702"),
		ConnectorID:     "staffy-external-mcp-server",
		ContractVersion: "1.0.0",
		Environment:     "sandbox",
		Status:          connectorDomain.StatusConnectedUnverified,
		Report:          report,
	}
}

func TestDeriveAdmitsRejectsAndCarriesTheProvenance(t *testing.T) {
	sections := Derive([]ConnectionInventory{inventoryFacts(t, connectorDomain.CapStateVerified)})
	if len(sections) != 1 {
		t.Fatalf("ожидался один раздел: %d", len(sections))
	}
	section := sections[0]
	if !section.HasEntries || len(section.Entries) != 3 {
		t.Fatalf("раздел обязан нести три записи: %+v", section)
	}
	byName := map[string]Entry{}
	for _, entry := range section.Entries {
		byName[entry.Name] = entry
	}
	admitted := byName["parts.lookup"]
	if !admitted.Admitted() || admitted.Input == nil || admitted.Output == nil {
		t.Fatalf("проектабельный read-only инструмент обязан допускаться: %+v", admitted)
	}
	if admitted.Provenance == nil || admitted.Provenance.ServerName != "fixture-external" || admitted.Provenance.ServerVersion != "1.0.0" {
		t.Fatalf("провенанс записи: %+v", admitted.Provenance)
	}
	if admitted.Provenance.ConnectorID != "staffy-external-mcp-server" || admitted.Provenance.ConnectorVersion != "1.0.0" {
		t.Fatalf("пин карточки в провенансе: %+v", admitted.Provenance)
	}
	if admitted.Provenance.CheckedAt.IsZero() || admitted.Provenance.ExpiresAt.IsZero() {
		t.Fatalf("окно доказательства в провенансе: %+v", admitted.Provenance)
	}
	if admitted.Input["query"].Type != "string" || admitted.Input["query"].Optional {
		t.Fatalf("спроецированный вход: %+v", admitted.Input["query"])
	}
	for _, name := range []string{"parts.mutate", "parts.search"} {
		rejected := byName[name]
		if rejected.Admitted() || rejected.Reason == "" || rejected.Input != nil {
			t.Fatalf("вне подмножества обязана быть честная строка отклонения: %+v", rejected)
		}
	}
	if !strings.Contains(byName["parts.mutate"].Reason, "AR-30") {
		t.Fatalf("отказ мутации обязан называть AR-30: %q", byName["parts.mutate"].Reason)
	}
	if !strings.Contains(byName["parts.search"].Reason, "вне платформенного словаря типизации") {
		t.Fatalf("отказ дробного числа обязан называть словарь: %q", byName["parts.search"].Reason)
	}
}

func TestDeriveStaleAnchorsMakeTheEntriesDisappear(t *testing.T) {
	// The four anchors of the connector machine read as the stale report —
	// the entries vanish with the anchor words of the derivation.
	cases := []struct {
		name   string
		reason string
	}{
		{"ротация секрета", "Доказательство устарело: секрет ротирован или отозван — доказательство не переносится на новый секрет."},
		{"save-конфига", "Доказательство устарело: конфигурация или состояние подключения изменились после проверки."},
		{"смена билда", "Доказательство устарело: сборка адаптера изменилась."},
		{"истечение TTL", "Доказательство устарело: срок действия истёк; требуется повторная проверка."},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			facts := inventoryFacts(t, connectorDomain.CapStateStale)
			facts.Report.StateReason = testCase.reason
			sections := Derive([]ConnectionInventory{facts})
			if len(sections) != 1 || sections[0].HasEntries || len(sections[0].Entries) != 0 {
				t.Fatalf("устаревшее доказательство обязано опустошать раздел: %+v", sections)
			}
			if !strings.Contains(sections[0].Reason, testCase.reason) {
				t.Fatalf("слова устаревшего раздела: %q", sections[0].Reason)
			}
		})
	}
}

func TestDeriveConnectionGuardsAndProbeStates(t *testing.T) {
	cases := []struct {
		name   string
		status string
		state  string
		reason string
	}{
		{"черновик подключения", connectorDomain.StatusDraft, "", "базовая проверка не выполнялась"},
		{"проваленная базовая проверка", connectorDomain.StatusFailed, "", "Базовая проверка не прошла"},
		{"отозванное подключение", connectorDomain.StatusRevoked, "", "отозвано"},
		{"не проверено", connectorDomain.StatusConnectedUnverified, connectorDomain.CapStateUnverified, "не проверены"},
		{"проверка не прошла", connectorDomain.StatusConnectedUnverified, connectorDomain.CapStateFailed, "не прошла"},
		{"результат неопределён", connectorDomain.StatusConnectedUnverified, connectorDomain.CapStateUnknown, "не завершилась до бюджета"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			facts := inventoryFacts(t, testCase.state)
			facts.Status = testCase.status
			sections := Derive([]ConnectionInventory{facts})
			if len(sections) != 1 || sections[0].HasEntries {
				t.Fatalf("раздел без применимого доказательства обязан быть пустым: %+v", sections)
			}
			if !strings.Contains(sections[0].Reason, strings.TrimSpace(testCase.reason)) {
				t.Fatalf("слова раздела: %q", sections[0].Reason)
			}
		})
	}
}

func TestDeriveHonestEmptyAndBrokenInventories(t *testing.T) {
	// No connections of the class: the honest empty array (never nil).
	if sections := Derive(nil); sections == nil || len(sections) != 0 {
		t.Fatalf("пустой словарь обязан быть пустым массивом: %v", sections)
	}
	// A verified report WITHOUT the inventory marker: the honest refusal.
	facts := inventoryFacts(t, connectorDomain.CapStateVerified)
	facts.Report.Evidence.RedactedResult = "Раунды выполнены, инвентаря нет."
	sections := Derive([]ConnectionInventory{facts})
	if sections[0].HasEntries || !strings.Contains(sections[0].Reason, "Инвентарь в доказательстве отсутствует") {
		t.Fatalf("слова отсутствия инвентаря: %q", sections[0].Reason)
	}
	// A foreign report row (the wiring drift): the honest words.
	foreign := inventoryFacts(t, connectorDomain.CapStateVerified)
	foreign.Report.ID = "query.server-discover"
	sections = Derive([]ConnectionInventory{foreign})
	if sections[0].HasEntries || !strings.Contains(sections[0].Reason, "Отчёт capabilities словаря не получен") {
		t.Fatalf("слова чужого отчёта: %q", sections[0].Reason)
	}
}
