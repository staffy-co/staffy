package packs

// Story 7-2 step-02: the cross-validation of the tool binding inside the
// pack loader — the composite publication rule. The cases mutate the LIVE
// pack artifacts (fstest.MapFS) with an INJECTABLE tool resolver: a tool
// ref the tool registry does not carry refuses the whole pack registry;
// a dependency on a connector type the binding pack does not declare is
// a named refusal too. The negative control proves the live registry
// (auto-parts@6.1.0 binding the three real contracts) loads through the
// production resolver.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/staffy-co/staffy/internal/tools"
)

// mapTools builds a resolver over explicit fixtures: id@version →
// contract. Absent refs resolve to false — the refusal cases need no
// files at all.
func mapTools(contracts ...tools.ToolContract) toolResolver {
	return func(id, version string) (tools.ToolContract, bool) {
		for _, c := range contracts {
			if c.ID == id && c.Version == version {
				return c, true
			}
		}
		return tools.ToolContract{}, false
	}
}

// TestToolBindingNegativeControl: the live pack registry loads through
// the production tool resolver — the 6.1.0 refs resolve against the
// embedded tool registry and the catalog dependency is declared.
func TestToolBindingNegativeControl(t *testing.T) {
	fsys, _ := baseFS(t)
	manifests, err := loadRegistry(fsys)
	if err != nil {
		t.Fatalf("живой реестр паков не грузится с production-резолвером: %v", err)
	}
	found := false
	for _, m := range manifests {
		if m.ID == "auto-parts" && m.Version == "6.1.0" {
			found = true
			if len(m.Tools) != 3 {
				t.Fatalf("6.1.0 без 3 tool-рефов: %d", len(m.Tools))
			}
		}
	}
	if !found {
		t.Fatal("auto-parts@6.1.0 отсутствует в реестре")
	}
}

// rePin пересчитывает sha256-пин 6.1.0 в индексе MapFS после мутации
// артефакта (мутация СОЗНАТЕЛЬНАЯ — пин обязан остаться честным, чтобы
// дошло до кросс-валидации, а не до sha256-отказа).
func rePin(t *testing.T, fsys fstest.MapFS, id, version string) {
	t.Helper()
	raw := fsys["registry/index.json"].Data
	var index struct {
		Entries []struct {
			ID      string `json:"id"`
			Version string `json:"version"`
			File    string `json:"file"`
			SHA256  string `json:"sha256"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	for i := range index.Entries {
		if index.Entries[i].ID == id && index.Entries[i].Version == version {
			sum := sha256.Sum256(fsys["registry/"+index.Entries[i].File].Data)
			index.Entries[i].SHA256 = hex.EncodeToString(sum[:])
		}
	}
	updated, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	fsys["registry/index.json"] = &fstest.MapFile{Data: updated}
}

// TestToolBindingRefAbsent: a pack artifact binding a tool ref the tool
// registry does not carry refuses the WHOLE pack registry — a composite
// publication must include its target.
func TestToolBindingRefAbsent(t *testing.T) {
	fsys, _ := baseFS(t)
	manifests, err := loadRegistryWithTools(fsys, mapTools()) // пустой реестр tools
	if err == nil {
		_ = manifests
		t.Fatal("реестр паков загрузился с битой привязкой — отказ не сработал")
	}
	if !strings.Contains(err.Error(), "auto-parts@6.1.0: tool auto-parts.vehicle-identification@1.0.0 отсутствует в реестре tools") {
		t.Fatalf("отказ без поимённой причины:\nполучено: %v", err)
	}
}

// TestToolBindingDependencyOutsideDomain: a bound contract declaring a
// connector type the binding pack does not carry is a named refusal —
// the static half of AC-3.
func TestToolBindingDependencyOutsideDomain(t *testing.T) {
	fsys, _ := baseFS(t)
	// Все три контракта есть, но fitment-evidence декларирует catalog,
	// а пак в этой мутации объявляет только inventory.
	contracts := []tools.ToolContract{
		{ID: "auto-parts.vehicle-identification", Version: "1.0.0", AdapterStatus: "not_implemented"},
		{ID: "auto-parts.part-request-classification", Version: "1.0.0", AdapterStatus: "not_implemented"},
		{ID: "auto-parts.fitment-evidence", Version: "1.0.0", AdapterStatus: "not_implemented", Dependency: &tools.ToolDependency{ConnectorType: "catalog"}},
	}

	// Мутируем 6.1.0: connector_types = ["inventory"] (пин обновляем честно).
	raw := fsys["registry/auto-parts@6.1.0.json"].Data
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse 6.1.0: %v", err)
	}
	doc["connector_types"] = []string{"inventory"}
	updated, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	fsys["registry/auto-parts@6.1.0.json"] = &fstest.MapFile{Data: updated}
	rePin(t, fsys, "auto-parts", "6.1.0")

	manifests, err := loadRegistryWithTools(fsys, mapTools(contracts...))
	if err == nil {
		_ = manifests
		t.Fatal("реестр паков загрузился с недекларированной зависимостью — отказ не сработал")
	}
	if !strings.Contains(err.Error(), "auto-parts@6.1.0: tool auto-parts.fitment-evidence@1.0.0: зависимость connector_type catalog не декларирована доменом пака") {
		t.Fatalf("отказ без поимённой причины:\nполучено: %v", err)
	}
}

// TestToolBindingDependencyDeclaredLoads: the same mutation with the
// dependency DECLARED in the pack's connector_types loads — the rule is
// the cross-check, not a blanket refusal of dependencies.
func TestToolBindingDependencyDeclaredLoads(t *testing.T) {
	fsys, _ := baseFS(t)
	contracts := []tools.ToolContract{
		{ID: "auto-parts.vehicle-identification", Version: "1.0.0", AdapterStatus: "not_implemented"},
		{ID: "auto-parts.part-request-classification", Version: "1.0.0", AdapterStatus: "not_implemented"},
		{ID: "auto-parts.fitment-evidence", Version: "1.0.0", AdapterStatus: "not_implemented", Dependency: &tools.ToolDependency{ConnectorType: "inventory"}},
	}
	raw := fsys["registry/auto-parts@6.1.0.json"].Data
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc["connector_types"] = []string{"inventory"}
	updated, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	fsys["registry/auto-parts@6.1.0.json"] = &fstest.MapFile{Data: updated}
	rePin(t, fsys, "auto-parts", "6.1.0")

	manifests, err := loadRegistryWithTools(fsys, mapTools(contracts...))
	if err != nil {
		t.Fatalf("реестр не загрузился при декларированной зависимости: %v", err)
	}
	if len(manifests) != 8 {
		t.Fatalf("ожидалось 8 манифестов, получено %d", len(manifests))
	}
}

// TestToolBindingWithoutToolsSkipped: packs without a binding skip the
// cross-validation entirely (nil path — no resolver calls needed).
func TestToolBindingWithoutToolsSkipped(t *testing.T) {
	fsys, _ := baseFS(t)
	// Резолвер, который ПАДАЕТ при любом обращении: старые паки (без
	// tools) его не вызывают вовсе.
	boom := func(id, version string) (tools.ToolContract, bool) {
		t.Fatalf("резолвер вызван для %s@%s — пак без привязки обязан миновать кросс-валидацию", id, version)
		return tools.ToolContract{}, false
	}
	// Убираем 6.1.0 из индекса и файлов: остаётся семёрка без привязок.
	raw := fsys["registry/index.json"].Data
	var index struct {
		Entries []struct {
			ID      string `json:"id"`
			Version string `json:"version"`
			File    string `json:"file"`
			SHA256  string `json:"sha256"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	kept := index.Entries[:0]
	for _, entry := range index.Entries {
		if entry.ID == "auto-parts" && entry.Version == "6.1.0" {
			continue
		}
		kept = append(kept, entry)
	}
	index.Entries = kept
	updated, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	fsys["registry/index.json"] = &fstest.MapFile{Data: updated}
	delete(fsys, "registry/auto-parts@6.1.0.json")

	manifests, err := loadRegistryWithTools(fsys, boom)
	if err != nil {
		t.Fatalf("реестр без привязок не загрузился: %v", err)
	}
	if len(manifests) != 7 {
		t.Fatalf("ожидалось 7 манифестов, получено %d", len(manifests))
	}
}
