package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The drift probes of Story 7-2 step-05: the append-only comparison holds
// the published tool index entries immutable (a version is never deleted
// and never rewritten — the pack bindings of the live tenants depend on
// it), and the CI baseline probe compares the current index against the
// snapshot of origin/main (the STAFFY_REGISTRY_BASELINE GITHUB_ENV
// pattern of Story 7-1 — the env is exported only in the success branch
// of git show; a broken CI config fails loudly, the first revision skips
// honestly).

// compareAppendOnly holds the published index entries immutable: every
// baseline entry must survive with the same sha256. Additions are
// allowed (an external author appends a new artifact plus a new entry);
// a deletion breaks the pack bindings, a sha change means a published
// version was rewritten.
func compareAppendOnly(baseline, current []indexEntry) error {
	currentByKey := map[string]indexEntry{}
	for _, entry := range current {
		currentByKey[entry.ID+"@"+entry.Version] = entry
	}
	for _, base := range baseline {
		key := base.ID + "@" + base.Version
		entry, ok := currentByKey[key]
		if !ok {
			return fmt.Errorf("индексная запись tools %s удалена — append-only нарушен (защита привязок паков)", key)
		}
		if entry.SHA256 != base.SHA256 {
			return fmt.Errorf("индексная запись tools %s изменена после публикации (sha256 %s → %s) — опубликованная версия неизменяема", key, base.SHA256[:12], entry.SHA256[:12])
		}
	}
	return nil
}

// TestRegistryAppendOnlyBaseline runs in CI with STAFFY_TOOLS_BASELINE
// pointing at the tool index snapshot of origin/main; locally it skips —
// the deletion and rewrite detections are covered by the unit probe
// below.
func TestRegistryAppendOnlyBaseline(t *testing.T) {
	baselinePath := os.Getenv("STAFFY_TOOLS_BASELINE")
	if baselinePath == "" {
		t.Skip("STAFFY_TOOLS_BASELINE не задан — append-only-сверка выполняется в CI (git fetch origin main)")
	}
	baselineRaw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("базовый индекс tools: %v", err)
	}
	var baseline registryIndex
	if err := json.Unmarshal(baselineRaw, &baseline); err != nil {
		t.Fatalf("базовый индекс tools: JSON: %v", err)
	}
	currentRaw, err := os.ReadFile("registry/index.json")
	if err != nil {
		t.Fatal(err)
	}
	var current registryIndex
	if err := json.Unmarshal(currentRaw, &current); err != nil {
		t.Fatal(err)
	}
	if err := compareAppendOnly(baseline.Entries, current.Entries); err != nil {
		t.Fatal(err)
	}
	t.Logf("append-only tools: %d базовых записей выжили неизменными, текущих записей %d", len(baseline.Entries), len(current.Entries))
}

// TestAppendOnlyComparisonDetectsDeletion: the local simulation of the
// drift — a removed index entry and a rewritten published version are
// both named refusals; pure additions and the identity case stay clean.
func TestAppendOnlyComparisonDetectsDeletion(t *testing.T) {
	currentRaw, err := os.ReadFile("registry/index.json")
	if err != nil {
		t.Fatal(err)
	}
	var current registryIndex
	if err := json.Unmarshal(currentRaw, &current); err != nil {
		t.Fatal(err)
	}
	if len(current.Entries) < 2 {
		t.Fatal("недостаточно записей для симуляции")
	}

	ghost := current.Entries[0]
	ghost.ID, ghost.Version, ghost.File = "ghost-pack.ghost-tool", "1.0.0", "ghost-pack.ghost-tool@1.0.0.json"
	ghost.SHA256 = strings.Repeat("0", 64)
	withGhost := append(append([]indexEntry(nil), current.Entries...), ghost)
	err = compareAppendOnly(withGhost, current.Entries)
	if err == nil || !strings.Contains(err.Error(), "ghost-pack.ghost-tool@1.0.0 удалена") {
		t.Fatalf("удаление записи не поймано: %v", err)
	}

	rewritten := append([]indexEntry(nil), current.Entries...)
	rewritten[0].SHA256 = strings.Repeat("1", 64)
	err = compareAppendOnly(rewritten, current.Entries)
	if err == nil || !strings.Contains(err.Error(), "изменена после публикации") {
		t.Fatalf("перезапись опубликованной версии не поймана: %v", err)
	}

	if err := compareAppendOnly(current.Entries, current.Entries); err != nil {
		t.Fatalf("идентичные индексы не должны давать отказ: %v", err)
	}
}
