package main

// The Story 7.3 fixture world of the MCP surface: the REAL registry
// contracts of 7-2 (all honestly not_implemented — they never appear in
// the exposed list) plus ONE implemented demo contract of the TEST
// CONTOUR (the vertical words live here only — the drift probe never
// walks the journeys), the scripted adapter (the valid output; the
// undeclared-key leak behind the toggle) and the scripted dependency
// gate (verified; the stale rotation behind the toggle). The toggles are
// the fixture doors of the z51 journey (the reconcile-verdict precedent
// of the scripted worlds). The production composition root wires the
// EMPTY adapter registry (AG-4) — this world is the honest proof
// machine of the gate, the call path and the refusals.

import (
	"context"
	"io"
	"log/slog"
	"sync"

	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/mcp"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/tools"
)

// The demo contract of the test contour: the implemented artifact with
// the catalog dependency (the rotation leg) and the typed schemas of the
// platform dictionary (the by-name validation legs).
var mcpDemoContract = tools.ToolContract{
	ID:               "auto-parts.demo-fitment",
	Version:          "1.0.0",
	Name:             "Демо-доказательство применимости",
	Purpose:          "Проверяет применимость детали по свежему демо-доказательству каталога (машинный тест-контур поверхности 7-3): возвращает применимость с провенансом",
	DeterminismClass: "deterministic_read",
	AdapterStatus:    "implemented",
	InputSchema: map[string]tools.ToolSchemaField{
		"part.oem_number":             {Label: "OEM-номер", Type: "string"},
		"vehicle.year_of_manufacture": {Label: "Год выпуска", Type: "integer", Min: 1900, Max: 2100, Optional: true},
	},
	OutputSchema: map[string]tools.ToolSchemaField{
		"fitment.link_type":            {Label: "Тип связи", Type: "string", Values: []string{"original", "cross"}},
		"fitment.evidence_age_seconds": {Label: "Возраст доказательства", Type: "integer", Min: 0},
	},
	Owner:       "auto-parts",
	EvidenceRef: "auto-parts/spec/7-3-fixture",
	Dependency:  &tools.ToolDependency{ConnectorType: "catalog"},
}

// mcpFixtureSource serves the real registry contracts plus the demo
// contract of the test contour (the toolResolver precedent of the packs
// loader — the injected source of the fixture world).
type mcpFixtureSource struct{}

func (mcpFixtureSource) List() []tools.ToolContract {
	published := tools.List()
	out := make([]tools.ToolContract, 0, len(published)+1)
	out = append(out, published...)
	out = append(out, mcpDemoContract)
	return out
}

func (mcpFixtureSource) Get(toolID, version string) (tools.ToolContract, bool) {
	if toolID == mcpDemoContract.ID && version == mcpDemoContract.Version {
		return mcpDemoContract, true
	}
	for _, contract := range tools.List() {
		if contract.ID == toolID && contract.Version == version {
			return contract, true
		}
	}
	return tools.ToolContract{}, false
}

// scriptedToolAdapter registers the demo tool only and scripts its
// output: the valid answer, or the undeclared-key leak behind the
// toggle (the fail-closed leg — the internal material never travels).
type scriptedToolAdapter struct {
	mu         sync.Mutex
	leaky      bool
	lastTenant id.UUID
	lastArgs   map[string]any
}

func (a *scriptedToolAdapter) Adapter(toolID, version string) (mcp.Adapter, bool) {
	if toolID != mcpDemoContract.ID || version != mcpDemoContract.Version {
		return nil, false
	}
	return a.execute, true
}

func (a *scriptedToolAdapter) execute(_ context.Context, tenant id.UUID, arguments map[string]any) (map[string]any, error) {
	a.mu.Lock()
	a.lastTenant = tenant
	a.lastArgs = arguments
	leaky := a.leaky
	a.mu.Unlock()
	output := map[string]any{
		"fitment.link_type":            "original",
		"fitment.evidence_age_seconds": int64(42),
	}
	if leaky {
		output["secret.internal_material"] = "внутренний секрет адаптера"
	}
	return output, nil
}

func (a *scriptedToolAdapter) setLeaky(leaky bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.leaky = leaky
}

// scriptedDependencyGate scripts the live fact of the catalog dependency
// (the verified word; the stale rotation behind the toggle — the
// dynamic half of the AC-3 gate).
type scriptedDependencyGate struct {
	mu   sync.Mutex
	live bool
	word string
}

func (g *scriptedDependencyGate) Live(context.Context, identityDomain.Principal, id.UUID, string) (bool, string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.live, g.word, nil
}

func (g *scriptedDependencyGate) setStale(stale bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if stale {
		g.live, g.word = false, "stale"
		return
	}
	g.live, g.word = true, "verified"
}

// mcpFixtureWorld carries the wired service and the scripted pieces the
// toggles drive.
type mcpFixtureWorld struct {
	service *mcp.Service
	gate    *scriptedDependencyGate
	adapter *scriptedToolAdapter
}

// newMCPFixtureWorld wires the REAL mcp.Service over the fixture source,
// the scripted adapter registry, the scripted dependency gate and the
// fixture membership ladder (the toolsMembership precedent of 7-2: the
// owner holds every scope, the review member is the honest 403, every
// other actor or tenant is the honest 404).
func newMCPFixtureWorld() *mcpFixtureWorld {
	gate := &scriptedDependencyGate{live: true, word: "verified"}
	adapter := &scriptedToolAdapter{}
	world := &mcpFixtureWorld{gate: gate, adapter: adapter}
	world.service = mcp.New(toolsMembership{}, mcpFixtureSource{}, adapter, gate, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return world
}
