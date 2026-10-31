package packs

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The drift probes of Story 7-1 step-04: the vertical probe holds the core
// free of the pack vocabulary, and the append-only comparison holds the
// published index entries immutable (a version is never deleted and never
// rewritten — the live tenant pins depend on it).

// verticalPhrases is the living dictionary taken from the step-01 artifacts
// (the Names, the Purposes, the Journeys and the schema Labels of all seven
// published packs). Updating the dictionary is a conscious step: a new pack
// version extends the vocabulary, and the probe grows with it.
var verticalPhrases = []string{
	"Автозапчасти",
	"Услуги Montagio",
	"Настройка направлений инженерных услуг",
	"Настройка предпочтений для запросов об автозапчастях",
	"Настройка предпочтений и идентификации автомобиля для запросов об автозапчастях",
	"Настройка предпочтений, идентификации автомобиля и материала запроса детали для запросов об автозапчастях",
	"Настройка предпочтений, идентификации автомобиля, материала запроса детали и доказательства применимости для запросов об автозапчастях",
	"Настройка предпочтений, идентификации автомобиля, материала запроса детали, доказательства применимости и сравнения предложений для запросов об автозапчастях",
	"поиск детали",
	"уточнение аналога",
	"идентификация автомобиля",
	"уточнение услуги",
	"передача мастеру",
	"Интерес к деталям",
	"Глубина сопоставления",
	"Инженерная сантехника",
	"Отопление",
	// The Story 7-2 tool artifact phrases: the contract Names and the
	// Purpose openers of the three vertical contracts. The registry
	// artifacts carry the words; the core packages never do.
	"Идентификация автомобиля",
	"Классификация запроса детали",
	"Доказательство применимости",
	"Определяет автомобиль",
	"Определяет режим запроса детали",
	"Проверяет применимость детали",
}

// verticalExceptions are the honest vertical surfaces outside the probe
// zone, named file by file: the frozen pilot machines of Story 4-1
// (byte-stable, their honest words quote the pack) and the part-request
// domain of Story 4.3 (the vertical machinery built around the reference
// pack; its semantic comments name the journey).
var verticalExceptions = []string{
	"internal/employee/domain/pilot.go",
	"internal/cases/domain/part.go",
}

// TestVerticalWordsAbsentFromCore: after the migration the pack vocabulary
// lives in the artifacts only — the probe walks the prod .go of the common
// core packages ({domain, app, port, store} under internal/ plus cmd/),
// skips the legitimate _test.go mirrors and the named exceptions, and
// fails NAMED per phrase per file. The probe fixes the zero and holds it.
func TestVerticalWordsAbsentFromCore(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("корень репозитория не найден: %v", err)
	}
	zones := []struct {
		dir   string
		allGo bool
	}{
		{dir: filepath.Join(root, "internal"), allGo: false},
		{dir: filepath.Join(root, "cmd"), allGo: true},
	}
	violations := 0
	for _, zone := range zones {
		err := filepath.WalkDir(zone.dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if !zone.allGo {
				switch filepath.Base(filepath.Dir(path)) {
				// The Story 7-3 zone: internal/mcp is the platform protocol
				// package — vertically clean by construction (the tool
				// material travels through the ports), so the probe walks
				// it with the same zero (the dictionary of phrases does
				// NOT grow).
				// The Story 7-4 zones: internal/mcpclient (the client
				// protocol machine of the external servers) and
				// internal/externaltools (the tenant-scoped external
				// dictionary) are platform packages too — the vertical
				// words of the fixture world (the tool names of the
				// scripted server) never enter them, the projection and
				// the wire dictionaries stay the only language.
				// The Story 8-1 zones: internal/planner (the turn-contract
				// machine — the platform dictionaries of the slice carry
				// only platform words) and internal/adapters/modelscripted
				// (the scripted model adapter — the honest synthetic
				// material of the production code, vertically clean) walk
				// with the same zero. The planner/store subdir rides the
				// existing "store" zone word.
				// The Story 8-2 zone: internal/adapters/modelbounded (the
				// bounded model adapter — the budget guard words are
				// platform-honest, vertically clean) joins the same zero.
				case "domain", "app", "port", "store", "tools", "mcp", "mcpclient", "externaltools", "planner", "modelscripted", "modelbounded":
				default:
					return nil
				}
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			slashRel := filepath.ToSlash(rel)
			for _, exception := range verticalExceptions {
				if slashRel == exception {
					return nil
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(raw)
			for _, phrase := range verticalPhrases {
				if strings.Contains(content, phrase) {
					violations++
					t.Errorf("вертикальная фраза %q в прод-коде общего пакета: %s", phrase, slashRel)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("обход зоны %s: %v", zone.dir, err)
		}
	}
	if violations == 0 {
		t.Logf("вертикальный зонд: 0 фраз словаря (%d) в зоне общих пакетов ядра", len(verticalPhrases))
	}
}

// compareAppendOnly holds the published index entries immutable: every
// baseline entry must survive with the same sha256. Additions are allowed
// (a new version is a new file plus a new entry); a deletion breaks the
// live tenant pins, a sha change means a published version was rewritten.
func compareAppendOnly(baseline, current []indexEntry) error {
	currentByKey := map[string]indexEntry{}
	for _, entry := range current {
		currentByKey[entry.ID+"@"+entry.Version] = entry
	}
	for _, base := range baseline {
		key := base.ID + "@" + base.Version
		entry, ok := currentByKey[key]
		if !ok {
			return fmt.Errorf("индексная запись %s удалена — append-only нарушен (защита живых пинов тенентов)", key)
		}
		if entry.SHA256 != base.SHA256 {
			return fmt.Errorf("индексная запись %s изменена после публикации (sha256 %s → %s) — опубликованная версия неизменяема", key, base.SHA256[:12], entry.SHA256[:12])
		}
	}
	return nil
}

// TestRegistryAppendOnlyBaseline runs in CI with STAFFY_REGISTRY_BASELINE
// pointing at the index snapshot of origin/main; locally it skips — the
// deletion and rewrite detections are covered by the unit probe below.
func TestRegistryAppendOnlyBaseline(t *testing.T) {
	baselinePath := os.Getenv("STAFFY_REGISTRY_BASELINE")
	if baselinePath == "" {
		t.Skip("STAFFY_REGISTRY_BASELINE не задан — append-only-сверка выполняется в CI (git fetch origin main)")
	}
	baselineRaw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("базовый индекс: %v", err)
	}
	var baseline registryIndex
	if err := json.Unmarshal(baselineRaw, &baseline); err != nil {
		t.Fatalf("базовый индекс: JSON: %v", err)
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
	t.Logf("append-only: %d базовых записей выжили неизменными, текущих записей %d", len(baseline.Entries), len(current.Entries))
}

// TestAppendOnlyComparisonDetectsDeletion: the local simulation of the
// drift — a removed index entry and a rewritten published version are both
// named refusals; pure additions and the identity case stay clean.
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
	ghost.ID, ghost.Version, ghost.File = "ghost-pack", "1.0.0", "ghost-pack@1.0.0.json"
	ghost.SHA256 = strings.Repeat("0", 64)
	withGhost := append(append([]indexEntry(nil), current.Entries...), ghost)
	err = compareAppendOnly(withGhost, current.Entries)
	if err == nil || !strings.Contains(err.Error(), "ghost-pack@1.0.0 удалена") {
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
