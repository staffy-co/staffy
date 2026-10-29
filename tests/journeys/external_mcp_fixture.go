package main

// The Story 7.4 fixture world of the external MCP connectors: the
// scripted external MCP server on the SAME listener (/fixture/external-mcp
// — no second port) speaking the pinned revision 2026-07-28 with the tool
// matrix of the journey (the projectable read-only tool, the read-only
// tool outside the typing dictionary, the tool without the read-only
// statement and the mutating tool — the dictionary admission legs), the
// valid call answer with the leaky toggle (the fail-closed output leg —
// the undeclared key never travels), the foreign-revision and
// oversized-inventory toggles (the probe refusal legs of the I/O matrix)
// and the authenticated-round requirement (the Authorization material of
// the MAC-checked command). The connector-side MAC logic is the REAL
// sealSecret/verifySeal mirror of the 1-8 application service — only the
// persistence is doubled into the fixture map (the connectorFixture
// precedent).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// The fixture identity of the external server (the provenance material of
// the dictionary entries).
const (
	externalFixtureServerName    = "fixture-external-mcp"
	externalFixtureServerVersion = "1.2.0"
	externalFixtureProtocol      = "2026-07-28"
)

// externalMCPFixture is the scripted external server with the honest
// toggles of the journey (the reconcile-verdict precedent of the scripted
// worlds).
type externalMCPFixture struct {
	mu       sync.Mutex
	foreign  bool
	oversize bool
	leaky    bool
}

// newExternalMCPFixture builds the scripted world with the honest default
// (the coherent revision, the bounded inventory, the valid call answer).
func newExternalMCPFixture() *externalMCPFixture {
	return &externalMCPFixture{}
}

// handle answers ONE JSON-RPC POST of the pinned revision. The round must
// be AUTHENTICATED: the Authorization material of the MAC-checked command
// travels with the sandbox format — an unauthenticated round is the honest
// 401 of the external server (the platform never sends one: the MAC gate
// closes first).
func (f *externalMCPFixture) handle(w http.ResponseWriter, r *http.Request) {
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer sandbox-") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthenticated"}`))
		return
	}
	f.mu.Lock()
	foreign, oversize, leaky := f.foreign, f.oversize, f.leaky
	f.mu.Unlock()
	var probe struct {
		ID     string          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&probe); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	versions := []string{externalFixtureProtocol}
	if foreign {
		versions = []string{"2025-11-25"}
	}
	answer := fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"protocolVersions":%s,"capabilities":{"tools":{"listChanged":false}},"serverInfo":{"name":%q,"version":%q}}}`, probe.ID, marshalStrings(versions), externalFixtureServerName, externalFixtureServerVersion)
	switch probe.Method {
	case "tools/list":
		inventory := externalFixtureInventory()
		encoded, _ := json.Marshal(inventory)
		if oversize {
			// The oversized inventory: the body grows past the machine
			// budget — the client refuses the whole answer (no truncation).
			encoded = append(encoded, []byte(fmt.Sprintf(`,{"padding":"%s"}`, repeat("x", 300*1024)))...)
			answer = fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"tools":%s}}`, probe.ID, "["+string(encoded)+"]")
		} else {
			answer = fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":{"tools":%s}}`, probe.ID, string(encoded))
		}
	case "tools/call":
		structured := map[string]any{"found": true, "count": float64(3)}
		if leaky {
			// The undeclared-key leak of the misbehaving external server:
			// the fail-closed output validation refuses the whole answer —
			// the internal material never travels (AD-11).
			structured["secret.internal_material"] = "внутренний материал внешнего сервера"
		}
		result := map[string]any{
			"content":           []map[string]string{{"type": "text", "text": "parts-lookup@snapshot"}},
			"structuredContent": structured,
		}
		encoded, _ := json.Marshal(result)
		answer = fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"result":%s}`, probe.ID, string(encoded))
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(answer))
}

// setForeignRevision scripts the foreign revision refusal leg.
func (f *externalMCPFixture) setForeignRevision(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.foreign = enabled
}

// setOversizedInventory scripts the budget refusal leg.
func (f *externalMCPFixture) setOversizedInventory(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.oversize = enabled
}

// setLeaky scripts the undeclared-key leak leg.
func (f *externalMCPFixture) setLeaky(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.leaky = enabled
}

// externalFixtureInventory is the tool matrix of the journey: the
// projectable read-only tool (admitted), the read-only tool outside the
// typing dictionary (rejected by the projection), the tool without the
// read-only statement (rejected by AR-30) and the mutating tool (the
// honest rejection — a write tool never becomes a dictionary row).
func externalFixtureInventory() []map[string]any {
	return []map[string]any{
		{
			"name":         "parts-lookup",
			"title":        "Поиск детали",
			"description":  "Читает каталог внешнего сервера (read-only, проектабельный)",
			"annotations":  map[string]any{"readOnlyHint": true},
			"inputSchema":  map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "title": "Запрос"}}, "required": []string{"query"}},
			"outputSchema": map[string]any{"type": "object", "properties": map[string]any{"found": map[string]any{"type": "boolean"}, "count": map[string]any{"type": "integer", "minimum": float64(0)}}, "required": []string{"found"}},
		},
		{
			"name":         "parts-ratio",
			"title":        "Дробная метрика",
			"description":  "Read-only метрика с дробным числом — вне платформенного словаря типизации",
			"annotations":  map[string]any{"readOnlyHint": true},
			"inputSchema":  map[string]any{"type": "object", "properties": map[string]any{"ratio": map[string]any{"type": "number"}}},
			"outputSchema": map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean"}}},
		},
		{
			"name":         "parts-unstated",
			"title":        "Без заявления read-only",
			"description":  "Инструмент не заявляет read-only — не допускается (AR-30)",
			"inputSchema":  map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}},
			"outputSchema": map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean"}}},
		},
		{
			"name":         "parts-send",
			"title":        "Отправка внешней команды",
			"description":  "Мутирующий инструмент внешнего сервера — write не существует ни как запись словаря, ни как инструмент модели",
			"annotations":  map[string]any{"readOnlyHint": false},
			"inputSchema":  map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}},
			"outputSchema": map[string]any{"type": "object", "properties": map[string]any{"receipt": map[string]any{"type": "string"}}},
		},
	}
}

// fixtureSealSecret mirrors the 1-8 application service seal: the HMAC of
// the per-connection random key over the sorted name\0value\0
// concatenation of the submitted secrets (the fixture persistence is the
// in-memory map — the connectorFixture precedent).
func fixtureSealSecret(key []byte, secrets map[string]string) []byte {
	names := make([]string, 0, len(secrets))
	for name := range secrets {
		names = append(names, name)
	}
	sort.Strings(names)
	mac := hmac.New(sha256.New, key)
	for _, name := range names {
		_, _ = mac.Write([]byte(name))
		_, _ = mac.Write([]byte{0})
		_, _ = mac.Write([]byte(secrets[name]))
		_, _ = mac.Write([]byte{0})
	}
	return mac.Sum(nil)
}

// fixtureVerifySeal mirrors verifySeal: the constant-time equality of the
// computed and the stored MAC — the proof the submitted secrets ARE the
// accepted ones (the plaintext never exists in the fixture storage).
func fixtureVerifySeal(key []byte, secrets map[string]string, sealed []byte) bool {
	return hmac.Equal(fixtureSealSecret(key, secrets), sealed)
}

func marshalStrings(values []string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func repeat(pattern string, count int) string {
	out := make([]byte, 0, len(pattern)*count)
	for i := 0; i < count; i++ {
		out = append(out, pattern...)
	}
	return string(out)
}
