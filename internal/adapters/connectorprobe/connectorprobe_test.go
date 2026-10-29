package connectorprobe

// The unit probes of the composition router (Story 7-4 step-01): the
// routing branches (the external MCP class against the synthetic
// fallback), the BasicCheck format matrix of the class, the Build
// constant, the fail-closed probe guard and the live discover/list rounds
// over the REAL httptest wire with the Authorization material of the
// caller. The MAC check itself is the application service's (the unit
// matrix lives there); the router receives the already-trusted material.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	connectorCatalog "github.com/artemiosu/staffy/internal/connector/catalog"
	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/connector/verifier"
	"github.com/artemiosu/staffy/internal/mcpclient"
)

// externalCard returns the live catalog card of the class (the production
// artifact is the test world — the catalog precedent of 7-2).
func externalCard(t *testing.T) connectorDomain.ConnectorDefinition {
	t.Helper()
	def, ok := connectorCatalog.Get("staffy-external-mcp-server", "1.0.0")
	if !ok {
		t.Fatal("карточка staffy-external-mcp-server@1.0.0 отсутствует в каталоге")
	}
	return def
}

func capability(def connectorDomain.ConnectorDefinition, id string) connectorDomain.CapabilityDeclaration {
	for _, dec := range def.Capabilities {
		if dec.ID == id {
			return dec
		}
	}
	return connectorDomain.CapabilityDeclaration{}
}

func TestRouterRoutesByAccountTypeAndKeepsTheSyntheticFallback(t *testing.T) {
	router := New(verifier.Synthetic{})
	card := externalCard(t)
	telephony := telephonyDefinition(t)

	// The class routes to the MCP branch: the BasicCheck words of the class.
	check := router.BasicCheck(context.Background(), card, connectorDomain.EnvSandbox,
		map[string]string{"endpoint": "https://fixture.external.example"}, map[string]string{"api_key": "sandbox-router-key"})
	if check.Authentication != connectorDomain.CheckVerified || check.Transport != connectorDomain.CheckVerified {
		t.Fatalf("ветвь класса: %+v", check)
	}

	// The telephony fallback is byte-identical to the synthetic.
	synthetic := verifier.Synthetic{}
	fallbackCheck := router.BasicCheck(context.Background(), telephony, connectorDomain.EnvSandbox,
		map[string]string{"endpoint": "https://demo.sandbox.example"}, map[string]string{"api_key": "sandbox-router-key"})
	directCheck := synthetic.BasicCheck(context.Background(), telephony, connectorDomain.EnvSandbox,
		map[string]string{"endpoint": "https://demo.sandbox.example"}, map[string]string{"api_key": "sandbox-router-key"})
	if fallbackCheck != directCheck {
		t.Fatalf("фолбэк телефонии изменился: %+v против %+v", fallbackCheck, directCheck)
	}
	if router.Build() != AdapterBuild || AdapterBuild != "connectorprobe/1.0.0" {
		t.Fatalf("Build роутера: %q", router.Build())
	}
}

func TestRouterBasicCheckFormatMatrix(t *testing.T) {
	router := New(verifier.Synthetic{})
	card := externalCard(t)
	cases := []struct {
		name      string
		endpoint  string
		key       string
		authWord  string
		authState string
		transport string
	}{
		{"валидные ключ и endpoint", "https://fixture.external.example/mcp", "sandbox-router-key", "", connectorDomain.CheckVerified, connectorDomain.CheckVerified},
		{"ключ не формата sandbox", "https://fixture.external.example/mcp", "real-production-key", "формату sandbox", connectorDomain.CheckFailed, connectorDomain.CheckVerified},
		{"отсутствующий ключ", "https://fixture.external.example/mcp", "", "формату sandbox", connectorDomain.CheckFailed, connectorDomain.CheckVerified},
		{"малформед endpoint", "not-a-url", "sandbox-router-key", "", connectorDomain.CheckVerified, connectorDomain.CheckFailed},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			check := router.BasicCheck(context.Background(), card, connectorDomain.EnvSandbox,
				map[string]string{"endpoint": testCase.endpoint}, map[string]string{"api_key": testCase.key})
			if check.Authentication != testCase.authState {
				t.Fatalf("аутентификация: %+v", check)
			}
			if check.Transport != testCase.transport {
				t.Fatalf("транспорт: %+v", check)
			}
			if testCase.authWord != "" && !strings.Contains(check.AuthReason, testCase.authWord) {
				t.Fatalf("слово отказа аутентификации: %q", check.AuthReason)
			}
		})
	}
}

func TestRouterProbeGuardRefusesTheNonVerifiableCapabilities(t *testing.T) {
	router := New(verifier.Synthetic{})
	card := externalCard(t)
	for _, capabilityID := range []string{"query.tool-call", "command.external-tool", "event.server-message"} {
		dec := capability(card, capabilityID)
		result := router.Probe(context.Background(), card, dec, connectorDomain.EnvSandbox,
			map[string]string{"endpoint": "https://fixture.external.example"}, map[string]string{"api_key": "sandbox-router-key"})
		if result.Outcome != connectorDomain.EvidenceFailed {
			t.Fatalf("capability %s обязана отказываться до сети, получено %q", capabilityID, result.Outcome)
		}
	}
}

// mcpWire serves the coherent fixture answers of the pinned revision and
// records the Authorization header of the rounds.
func mcpWire(t *testing.T, foreignRevision bool) (*httptest.Server, *[]string) {
	t.Helper()
	authorizations := &[]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*authorizations = append(*authorizations, r.Header.Get("Authorization"))
		body := make([]byte, 4096)
		n, _ := r.Body.Read(body)
		var probe struct {
			ID     string `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body[:n], &probe)
		versions := `["2026-07-28"]`
		if foreignRevision {
			versions = `["2025-11-25"]`
		}
		answer := fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"protocolVersions":%s,"serverInfo":{"name":"fixture-external","version":"1.0.0"}}}`, probe.ID, versions)
		if probe.Method == mcpclient.MethodToolsList {
			answer = fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"tools":[{"name":"parts.lookup","title":"Поиск детали","description":"Читает каталог","inputSchema":{"type":"object"},"outputSchema":{"type":"object"},"annotations":{"readOnlyHint":true}}]}}`, probe.ID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	return server, authorizations
}

func TestRouterDiscoverProbeRunsTheAuthenticatedRound(t *testing.T) {
	server, authorizations := mcpWire(t, false)
	router := New(verifier.Synthetic{})
	card := externalCard(t)
	dec := capability(card, "query.server-discover")
	result := router.Probe(context.Background(), card, dec, connectorDomain.EnvSandbox,
		map[string]string{"endpoint": server.URL}, map[string]string{"api_key": "sandbox-router-key"})
	if result.Outcome != connectorDomain.EvidenceVerified {
		t.Fatalf("исход пробы: %+v", result)
	}
	if !strings.Contains(result.RedactedResult, "fixture-external") || !strings.Contains(result.RedactedResult, "2026-07-28") {
		t.Fatalf("слова исхода не несут провенанс: %q", result.RedactedResult)
	}
	if len(*authorizations) != 1 || (*authorizations)[0] != "Bearer sandbox-router-key" {
		t.Fatalf("раунд не аутентифицирован материалом команды: %v", *authorizations)
	}
	if strings.Contains(result.RedactedResult, "sandbox-router-key") || strings.Contains(result.RedactedRequest, "sandbox-router-key") {
		t.Fatalf("секрет эхо в редацтированных словах: %q / %q", result.RedactedRequest, result.RedactedResult)
	}
}

func TestRouterListProbeRunsBothRoundsAndStoresTheFilteredInventory(t *testing.T) {
	server, authorizations := mcpWire(t, false)
	router := New(verifier.Synthetic{})
	card := externalCard(t)
	dec := capability(card, "query.tools-list")
	result := router.Probe(context.Background(), card, dec, connectorDomain.EnvSandbox,
		map[string]string{"endpoint": server.URL}, map[string]string{"api_key": "sandbox-router-key"})
	if result.Outcome != connectorDomain.EvidenceVerified {
		t.Fatalf("исход пробы: %+v", result)
	}
	if !strings.Contains(result.RedactedResult, "read-tools: 1") {
		t.Fatalf("слова исхода не несут счётчик инвентаря: %q", result.RedactedResult)
	}
	// The compact FILTERED JSON inventory rides the marker line: the server
	// identity of the discover round plus the descriptors with the known
	// protocol fields only (the dictionary contract — pinned here and in
	// the derive tests).
	marker := strings.LastIndex(result.RedactedResult, "\nИнвентарь: ")
	if marker < 0 {
		t.Fatalf("маркер инвентаря отсутствует: %q", result.RedactedResult)
	}
	var inventory struct {
		Server struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"server"`
		Tools []struct {
			Name         string         `json:"name"`
			ReadOnlyHint bool           `json:"readOnlyHint"`
			InputSchema  map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.RedactedResult[marker+len("\nИнвентарь: "):])), &inventory); err != nil {
		t.Fatalf("инвентарь не читается: %v", err)
	}
	if inventory.Server.Name != "fixture-external" || inventory.Server.Version != "1.0.0" {
		t.Fatalf("провенанс инвентаря: %+v", inventory.Server)
	}
	if len(inventory.Tools) != 1 || inventory.Tools[0].Name != "parts.lookup" || !inventory.Tools[0].ReadOnlyHint {
		t.Fatalf("инвентарь не отфильтрован до известных полей: %+v", inventory.Tools)
	}
	if len(inventory.Tools[0].InputSchema) == 0 {
		t.Fatal("схема дескриптора не попала в инвентарь")
	}
	if strings.Contains(result.RedactedResult, "sandbox-router-key") {
		t.Fatal("секрет эхо в редацтированных словах")
	}
	if len(*authorizations) != 2 {
		t.Fatalf("проба словаря обязана выполнять два раунда (discover + list), выполнено %d", len(*authorizations))
	}
}

func TestRouterProbeForeignRevisionFailsHonestly(t *testing.T) {
	server, _ := mcpWire(t, true)
	router := New(verifier.Synthetic{})
	card := externalCard(t)
	dec := capability(card, "query.server-discover")
	result := router.Probe(context.Background(), card, dec, connectorDomain.EnvSandbox,
		map[string]string{"endpoint": server.URL}, map[string]string{"api_key": "sandbox-router-key"})
	if result.Outcome != connectorDomain.EvidenceFailed {
		t.Fatalf("чужая ревизия обязана давать failed: %+v", result)
	}
	if !strings.Contains(result.RedactedResult, "чужая ревизия") {
		t.Fatalf("причина отказа не названа: %q", result.RedactedResult)
	}
}

func TestRouterProbeUnreachableEndpointFailsHonestly(t *testing.T) {
	router := New(verifier.Synthetic{})
	card := externalCard(t)
	dec := capability(card, "query.server-discover")
	result := router.Probe(context.Background(), card, dec, connectorDomain.EnvSandbox,
		map[string]string{"endpoint": "https://unreachable.invalid.example"}, map[string]string{"api_key": "sandbox-router-key"})
	if result.Outcome != connectorDomain.EvidenceFailed {
		t.Fatalf("недостижимый endpoint обязан давать failed: %+v", result)
	}
}

// telephonyDefinition reaches the live telephony card of the catalog.
func telephonyDefinition(t *testing.T) connectorDomain.ConnectorDefinition {
	t.Helper()
	def, ok := connectorCatalog.Get("staffy-synthetic-telephony", "1.2.0")
	if !ok {
		t.Fatal("карточка телефонии 1.2.0 отсутствует")
	}
	return def
}
