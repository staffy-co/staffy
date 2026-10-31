//go:build integration

package integration_test

// The Story 7-4 step-01 integration contour: the external MCP class over
// the LIVE PostgreSQL — the connection lifecycle of the class (the format
// BasicCheck of the router branch, the seal of the accepted sandbox key)
// and the MAC matrix of the authenticated probes (the missing key, the
// mismatching key, the matching key with the REAL wire round over an
// httptest MCP server of the pinned revision, the rotation that
// invalidates the old key AND the prior evidence anchor). The verifier is
// the production composition — the connectorprobe router over the
// synthetic fallback.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/adapters/connectorprobe"
	"github.com/staffy-co/staffy/internal/adapters/externaltoolsfacts"
	connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
	connectorApp "github.com/staffy-co/staffy/internal/connector/app"
	connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
	connectorStore "github.com/staffy-co/staffy/internal/connector/store"
	"github.com/staffy-co/staffy/internal/connector/verifier"
	"github.com/staffy-co/staffy/internal/externaltools"
	identityApp "github.com/staffy-co/staffy/internal/identity/app"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// externalMCPWire serves the coherent answers of the pinned revision
// 2026-07-28 (the mirror of the 7-3 server machine) and records the
// Authorization headers of the rounds.
func externalMCPWire(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	authorizations := &[]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*authorizations = append(*authorizations, r.Header.Get("Authorization"))
		var probe struct {
			ID     string          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&probe)
		var answer string
		switch probe.Method {
		case "server/discover":
			answer = fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"protocolVersions":["2026-07-28"],"capabilities":{"tools":{"listChanged":false}},"serverInfo":{"name":"integration-external","version":"1.0.0"}}}`, probe.ID)
		case "tools/list":
			answer = fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"tools":[{"name":"parts.lookup","title":"Поиск детали","description":"Читает каталог","inputSchema":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]},"outputSchema":{"type":"object","properties":{"found":{"type":"boolean"}}},"annotations":{"readOnlyHint":true}}]}}`, probe.ID)
		case "tools/call":
			answer = fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"content":[{"type":"text","text":"parts.lookup@snapshot"}],"structuredContent":{"found":true}}}`, probe.ID)
		default:
			answer = fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"error":{"code":-32601,"message":"метод не существует"}}`, probe.ID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	return server, authorizations
}

// wallClock advances with the real time: the repeated probes of one
// capability after a rotation get DISTINCT completion moments, so the
// latestEvidence pick stays deterministic (the fixed clock would tie the
// rows and flip on the EvidenceID tie-break).
type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now().UTC() }

func externalConnectorService(identity *identityApp.Service, ids employeeIDs) *connectorApp.Service {
	return connectorApp.New(connectorStore.NewPostgres(runtimePool), identity, connectorprobe.New(verifier.Synthetic{}), wallClock{}, ids)
}

func TestExternalMCPConnectionMACMatrixAndLiveProbeOverPostgres(t *testing.T) {
	ctx := context.Background()
	identity, owner := newService("00000000e601", "00000000e602", "00000000e603")
	tenant := createTenant(t, identity, owner, "External MCP tenant", "external-mcp-tenant-key", "00000000e604").Tenant.ID
	ids := employeeIDs{}
	svc := externalConnectorService(identity, ids)
	wire, authorizations := externalMCPWire(t)
	corr := id.MustParse("01994f64-6c00-7000-8000-00000000e605")

	// The class connection: the format BasicCheck of the router branch —
	// the sandbox key form and the URL form, without any network round.
	created, err := svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "ext-key-create", CorrelationID: corr, ConnectorID: "staffy-external-mcp-server", ConnectorVer: "1.0.0", Label: "Внешний сервер", Environment: connectorDomain.EnvSandbox, Config: map[string]string{"endpoint": wire.URL}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	connectionID := created.Connection.ConnectionID

	// The production map is fail-closed on the class.
	if _, err = svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "ext-key-prod", CorrelationID: corr, ConnectorID: "staffy-external-mcp-server", ConnectorVer: "1.0.0", Label: "Прод", Environment: connectorDomain.EnvProduct, Config: map[string]string{"endpoint": wire.URL}}); !errors.Is(err, connectorDomain.ErrProductionBlocked) {
		t.Fatalf("production обязан быть закрыт на классе: %v", err)
	}

	// The invalid key is never sealed (the connect discipline of 1-8).
	failed, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: 1, Secrets: map[string]string{"api_key": "real-production-key"}, IdempotencyKey: "ext-key-bad"})
	if err != nil {
		t.Fatalf("connect с невалидным ключом: %v", err)
	}
	if failed.Connection.Status != connectorDomain.StatusFailed || failed.Connection.SecretActive {
		t.Fatalf("невалидный ключ не запечатывается: %+v", failed.Connection)
	}

	// The accepted sandbox key seals the connection.
	connected, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: 2, Secrets: map[string]string{"api_key": "sandbox-external-live"}, IdempotencyKey: "ext-key-connect"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if connected.Connection.Status != connectorDomain.StatusConnectedUnverified || !connected.Connection.SecretActive {
		t.Fatalf("connection: %+v", connected.Connection)
	}

	// The missing key: the unauthenticated round is impossible fail-closed.
	_, err = svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.server-discover", IdempotencyKey: "ext-verify-missing"})
	var rule connectorDomain.RuleError
	if !errors.As(err, &rule) || !strings.Contains(rule.Reason, "требует секрет подключения") {
		t.Fatalf("отказ без ключа: %v", err)
	}

	// The mismatching key: the honest MAC refusal BEFORE the network round.
	_, err = svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.server-discover", IdempotencyKey: "ext-verify-wrong", Secrets: map[string]string{"api_key": "sandbox-wrong-key"}})
	if !errors.As(err, &rule) || !strings.Contains(rule.Reason, "не совпадает с принятым подключением") {
		t.Fatalf("отказ несовпадения MAC: %v", err)
	}
	if len(*authorizations) != 0 {
		t.Fatalf("раунды до MAC-проверки невозможны: %d", len(*authorizations))
	}

	// The matching key: the LIVE round over the real wire — the evidence
	// carries the redacted words with the server identity, the secret never
	// appears anywhere.
	verified, err := svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.server-discover", IdempotencyKey: "ext-verify-live", Secrets: map[string]string{"api_key": "sandbox-external-live"}})
	if err != nil {
		t.Fatalf("живая проба: %v", err)
	}
	if verified.Report.State != connectorDomain.CapStateVerified || verified.Report.Evidence == nil {
		t.Fatalf("отчёт живой пробы: %+v", verified.Report)
	}
	if !strings.Contains(verified.Report.Evidence.RedactedResult, "integration-external") {
		t.Fatalf("редацитированные слова не несут провенанс: %q", verified.Report.Evidence.RedactedResult)
	}
	if strings.Contains(verified.Report.Evidence.RedactedRequest, "sandbox-external-live") || strings.Contains(verified.Report.Evidence.RedactedResult, "sandbox-external-live") {
		t.Fatal("секрет эхо в evidence")
	}
	if len(*authorizations) != 1 || (*authorizations)[0] != "Bearer sandbox-external-live" {
		t.Fatalf("живой раунд не аутентифицирован: %v", *authorizations)
	}
	if !strings.Contains(verified.Report.Evidence.RedactedRequest, "connectorprobe/1.0.0") {
		t.Fatalf("билд роутера в редацтированном запросе: %q", verified.Report.Evidence.RedactedRequest)
	}

	// The rotation: the new seal invalidates the old key and the prior
	// evidence anchor — the state honestly reads stale until a fresh probe.
	rotated, err := svc.Rotate(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: connected.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-rotated-live"}, IdempotencyKey: "ext-key-rotate"})
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	_, err = svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.server-discover", IdempotencyKey: "ext-verify-old", Secrets: map[string]string{"api_key": "sandbox-external-live"}})
	if !errors.As(err, &rule) || !strings.Contains(rule.Reason, "не совпадает с принятым подключением") {
		t.Fatalf("старый ключ после ротации: %v", err)
	}
	state, err := svc.CapabilityState(ctx, owner, tenant, connectionID)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	for _, report := range state.Capabilities {
		if report.ID == "query.server-discover" && report.State != connectorDomain.CapStateStale {
			t.Fatalf("evidence после ротации обязан читать stale: %+v", report)
		}
	}
	if rotated.Connection.SecretHint == "" || strings.Contains(rotated.Connection.SecretHint, "sandbox") {
		t.Fatalf("подсказка секрета: %q", rotated.Connection.SecretHint)
	}

	// The rotated key re-opens the live probe.
	reVerified, err := svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.tools-list", IdempotencyKey: "ext-verify-list", Secrets: map[string]string{"api_key": "sandbox-rotated-live"}})
	if err != nil {
		t.Fatalf("повторная проба: %v", err)
	}
	if reVerified.Report.State != connectorDomain.CapStateVerified {
		t.Fatalf("повторная проба обязана вернуть verified: %+v", reVerified.Report)
	}
	if !strings.Contains(reVerified.Report.Evidence.RedactedResult, "read-tools: 1") {
		t.Fatalf("проба словаря обязана нести счётчик: %q", reVerified.Report.Evidence.RedactedResult)
	}

	// The evidence rows stay tenant-scoped (RLS + the scope of the reads).
	rows, err := svc.CapabilityState(ctx, owner, tenant, connectionID)
	if err != nil || len(rows.Capabilities) != 5 {
		t.Fatalf("state после проб: err=%v caps=%d", err, len(rows.Capabilities))
	}
}

// The Story 7-4 step-02 contour: the derivation over the live facts — the
// dictionary entries exist exactly while the applicable evidence holds
// (the rotation makes them disappear, the repeated probe returns them),
// and the call machine drives the REAL wire round of the admitted tool of
// the live snapshot with the strict validations.
func TestExternalMCPDictionaryDerivationAndCallMachineOverPostgres(t *testing.T) {
	ctx := context.Background()
	identity, owner := newService("00000000e611", "00000000e612", "00000000e613")
	tenant := createTenant(t, identity, owner, "External MCP dictionary tenant", "external-mcp-dictionary-key", "00000000e614").Tenant.ID
	ids := employeeIDs{}
	svc := externalConnectorService(identity, ids)
	wire, _ := externalMCPWire(t)
	corr := id.MustParse("01994f64-6c00-7000-8000-00000000e615")

	created, err := svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "ext2-key-create", CorrelationID: corr, ConnectorID: "staffy-external-mcp-server", ConnectorVer: "1.0.0", Label: "Словарь", Environment: connectorDomain.EnvSandbox, Config: map[string]string{"endpoint": wire.URL}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	connectionID := created.Connection.ConnectionID
	if _, err = svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: 1, Secrets: map[string]string{"api_key": "sandbox-dictionary-live"}, IdempotencyKey: "ext2-key-connect"}); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// The dictionary BEFORE the probe: the honest empty section (the tools
	// are not proven yet — never a false success).
	facts := externaltoolsfacts.New(svc)
	dictionary := externaltools.New(identity, facts)
	sections, err := dictionary.Dictionary(ctx, owner, tenant)
	if err != nil {
		t.Fatalf("dictionary: %v", err)
	}
	if len(sections) != 1 || sections[0].HasEntries {
		t.Fatalf("непроверенный словарь обязан быть пустым разделом: %+v", sections)
	}

	// The live tools-list probe with the matching key.
	if _, err = svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.tools-list", IdempotencyKey: "ext2-verify-list", Secrets: map[string]string{"api_key": "sandbox-dictionary-live"}}); err != nil {
		t.Fatalf("verify tools-list: %v", err)
	}
	sections, err = dictionary.Dictionary(ctx, owner, tenant)
	if err != nil {
		t.Fatalf("dictionary: %v", err)
	}
	if len(sections) != 1 || !sections[0].HasEntries || len(sections[0].Entries) != 1 {
		t.Fatalf("словарь после пробы обязан нести запись: %+v", sections)
	}
	entry := sections[0].Entries[0]
	if !entry.Admitted() || entry.Name != "parts.lookup" {
		t.Fatalf("запись словаря: %+v", entry)
	}
	if entry.Provenance == nil || entry.Provenance.ServerName != "integration-external" {
		t.Fatalf("провенанс записи: %+v", entry.Provenance)
	}

	// The call machine over the REAL wire: the strict round of the admitted
	// tool with the caller's Authorization material.
	calls := externaltools.NewCalls(facts, externaltoolsfacts.NewWireTransport(wire.Client().Transport, 256<<10))
	scope, scopeErr := tenancy.New(owner.ID, tenant)
	if scopeErr != nil {
		t.Fatal(scopeErr)
	}
	answer, err := calls.Call(ctx, scope, connectionID, "parts.lookup", map[string]any{"query": "BR-001"}, wire.URL, "Bearer sandbox-dictionary-live")
	if err != nil {
		t.Fatalf("вызов: %v", err)
	}
	if answer["found"] != true {
		t.Fatalf("типизированный ответ вызова: %v", answer)
	}

	// The fail-closed arguments: the round never happens.
	if _, err = calls.Call(ctx, scope, connectionID, "parts.lookup", map[string]any{"query": 7}, wire.URL, "Bearer sandbox-dictionary-live"); !errors.Is(err, externaltools.ErrArguments) {
		t.Fatalf("валидация аргументов: %v", err)
	}

	// The rotation: the entries DISAPPEAR (the stale anchor) and the call
	// pin refuses BEFORE the round.
	if _, err = svc.Rotate(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: 2, Secrets: map[string]string{"api_key": "sandbox-rotated-dictionary"}, IdempotencyKey: "ext2-key-rotate"}); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	sections, err = dictionary.Dictionary(ctx, owner, tenant)
	if err != nil {
		t.Fatalf("dictionary после ротации: %v", err)
	}
	if len(sections) != 1 || sections[0].HasEntries {
		t.Fatalf("ротация обязана опустошать словарь: %+v", sections)
	}
	if _, err = calls.Call(ctx, scope, connectionID, "parts.lookup", map[string]any{"query": "x"}, wire.URL, "Bearer sandbox-rotated-dictionary"); !errors.Is(err, externaltools.ErrEntryNotFound) {
		t.Fatalf("пин вызова после ротации: %v", err)
	}

	// The repeated probe returns the entries.
	if _, err = svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.tools-list", IdempotencyKey: "ext2-verify-relist", Secrets: map[string]string{"api_key": "sandbox-rotated-dictionary"}}); err != nil {
		t.Fatalf("повторная проба: %v", err)
	}
	sections, err = dictionary.Dictionary(ctx, owner, tenant)
	if err != nil {
		t.Fatalf("dictionary после повторной пробы: %v", err)
	}
	if len(sections) != 1 || !sections[0].HasEntries || sections[0].Entries[0].Name != "parts.lookup" {
		t.Fatalf("повторная проба обязана возвращать записи: %+v", sections)
	}
}
