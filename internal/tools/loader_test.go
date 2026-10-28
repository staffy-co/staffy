package tools

// Story 7-2 step-02: the fail-closed rejection matrix of the tool
// registry loader. Every case mutates the LIVE artifacts (fstest.MapFS
// built from the real registry files) and expects a NAMED refusal — a
// partial tool dictionary never exists. The negative control proves the
// unmutated registry loads. The matrix mirrors the Story 7-1 pack loader
// matrix plus the tool-specific dictionaries: determinism_class (the
// single-value class — a write tool is structurally impossible),
// adapter_status and the typed schema key rules.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func baseFS(t *testing.T) (fstest.MapFS, *registryIndex) {
	t.Helper()
	fsys := fstest.MapFS{}
	indexRaw, err := registryFiles.ReadFile(registryIndexFile)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	fsys[registryIndexFile] = &fstest.MapFile{Data: indexRaw}
	var index registryIndex
	if err := json.Unmarshal(indexRaw, &index); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	for _, entry := range index.Entries {
		path := registryDir + "/" + entry.File
		raw, err := registryFiles.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", entry.File, err)
		}
		fsys[path] = &fstest.MapFile{Data: raw}
	}
	return fsys, &index
}

func flushIndex(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
	t.Helper()
	raw, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	fsys[registryIndexFile] = &fstest.MapFile{Data: raw}
}

func rewriteArtifact(t *testing.T, fsys fstest.MapFS, entry indexEntry, mutate func(map[string]any)) {
	t.Helper()
	path := registryDir + "/" + entry.File
	raw := fsys[path].Data
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", entry.File, err)
	}
	mutate(doc)
	updated, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal %s: %v", entry.File, err)
	}
	fsys[path] = &fstest.MapFile{Data: updated}
}

// index0 re-reads the index from the fs (a helper for the cases that
// need to update the pin of one entry after a rewrite).
func index0(t *testing.T, fsys fstest.MapFS) *registryIndex {
	t.Helper()
	raw := fsys[registryIndexFile].Data
	var index registryIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	return &index
}

func reindexPinned(t *testing.T, fsys fstest.MapFS, id, version string) {
	t.Helper()
	index := index0(t, fsys)
	for i := range index.Entries {
		if index.Entries[i].ID == id && index.Entries[i].Version == version {
			sum := sha256.Sum256(fsys[registryDir+"/"+index.Entries[i].File].Data)
			index.Entries[i].SHA256 = hex.EncodeToString(sum[:])
		}
	}
	flushIndex(t, fsys, index)
}

// renameVersion rewrites the artifact's version AND the index entry AND
// the file name together (keeping the pin honest and the canon intact), so
// the refusal fires at the semver validation, not earlier checks.
func renameVersion(t *testing.T, fsys fstest.MapFS, index *registryIndex, i int, version string) {
	t.Helper()
	entry := index.Entries[i]
	oldPath := registryDir + "/" + entry.File
	rewriteArtifact(t, fsys, entry, func(doc map[string]any) { doc["version"] = version })
	entry.Version = version
	entry.File = entry.ID + "@" + version + ".json"
	newPath := registryDir + "/" + entry.File
	fsys[newPath] = fsys[oldPath]
	delete(fsys, oldPath)
	sum := sha256.Sum256(fsys[newPath].Data)
	entry.SHA256 = hex.EncodeToString(sum[:])
	index.Entries[i] = entry
	flushIndex(t, fsys, index)
}

func rejections(t *testing.T) []struct {
	name     string
	mutate   func(t *testing.T, fsys fstest.MapFS, index *registryIndex)
	wantWord string
} {
	return []struct {
		name     string
		mutate   func(t *testing.T, fsys fstest.MapFS, index *registryIndex)
		wantWord string
	}{
		{
			name: "обязательное поле name отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { delete(doc, "name") })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "обязательное поле name отсутствует",
		},
		{
			name: "обязательное поле purpose отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { delete(doc, "purpose") })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "обязательное поле purpose отсутствует",
		},
		{
			name: "обязательное поле owner отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { delete(doc, "owner") })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "обязательное поле owner отсутствует",
		},
		{
			name: "обязательное поле evidence_ref отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { delete(doc, "evidence_ref") })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "обязательное поле evidence_ref отсутствует",
		},
		{
			name: "обязательное поле adapter_status отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { delete(doc, "adapter_status") })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "обязательное поле adapter_status отсутствует",
		},
		{
			name: "пустая input_schema",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { doc["input_schema"] = map[string]any{} })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "обязательное поле input_schema отсутствует",
		},
		{
			name: "пустая output_schema",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { doc["output_schema"] = map[string]any{} })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "обязательное поле output_schema отсутствует",
		},
		{
			name: "determinism_class вне словаря (write-инструмент)",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { doc["determinism_class"] = "write" })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "determinism_class: значение write вне платформенного словаря",
		},
		{
			name: "determinism_class вне словаря (mutation)",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { doc["determinism_class"] = "mutation" })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "determinism_class: значение mutation вне платформенного словаря",
		},
		{
			name: "adapter_status вне словаря",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { doc["adapter_status"] = "planned" })
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "adapter_status: значение planned вне платформенного словаря",
		},
		{
			name: "тип ключа схемы вне словаря",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) {
					schema := doc["input_schema"].(map[string]any)
					key := schema["vehicle.vin"].(map[string]any)
					key["type"] = "float"
				})
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "input_schema.vehicle.vin: тип float вне платформенного словаря",
		},
		{
			name: "label ключа схемы отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) {
					schema := doc["input_schema"].(map[string]any)
					key := schema["vehicle.vin"].(map[string]any)
					delete(key, "label")
				})
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "input_schema.vehicle.vin: label отсутствует",
		},
		{
			name: "integer-ключ с values",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) {
					schema := doc["output_schema"].(map[string]any)
					key := schema["vehicle.year_of_manufacture"].(map[string]any)
					key["values"] = []string{"2000", "2010"}
				})
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "integer-ключ не может объявлять values",
		},
		{
			name: "string-ключ с min/max",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) {
					schema := doc["input_schema"].(map[string]any)
					key := schema["vehicle.vin"].(map[string]any)
					key["min"] = 1
				})
				reindexPinned(t, fsys, index.Entries[0].ID, index.Entries[0].Version)
			},
			wantWord: "границы min/max допустимы только у integer-ключа",
		},
		{
			name: "version не semver-core (prerelease)",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				renameVersion(t, fsys, index, 0, "1.0.0-rc.1")
			},
			wantWord: "неверный формат версии 1.0.0-rc.1",
		},
		{
			name: "version не semver-core (двухкомпонентная)",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				renameVersion(t, fsys, index, 0, "1.0")
			},
			wantWord: "неверный формат версии 1.0",
		},
		{
			name: "дубликат головной записи",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				index.Entries = append(index.Entries, index.Entries[0])
				flushIndex(t, fsys, index)
			},
			// Голова реестра — первая запись среза (стабильна как начало
			// witness-подсерии); дубликат хвоста (растёт с внешними
			// дополнениями) проверяется динамическим тестом ниже.
			wantWord: "индексная запись auto-parts.vehicle-identification@1.0.0: дубликат",
		},
		{
			name: "имя файла нарушает канон",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				index.Entries[0].File = "weird-name.json"
				flushIndex(t, fsys, index)
			},
			wantWord: "имя файла weird-name.json нарушает канон <id>@<version>.json",
		},
		{
			name: "файл не индексирован",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				fsys[registryDir+"/auto-parts.surprise@1.0.0.json"] = &fstest.MapFile{Data: fsys[registryDir+"/"+index.Entries[0].File].Data}
			},
			wantWord: "файл tools/auto-parts.surprise@1.0.0.json не индексирован",
		},
		{
			name: "индексированный файл отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				delete(fsys, registryDir+"/"+index.Entries[1].File)
			},
			wantWord: "auto-parts.part-request-classification@1.0.0:",
		},
		{
			name: "sha256 не совпадает (правка после индексации)",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[0], func(doc map[string]any) { doc["name"] = "Взломанный контракт" })
				// Пин оставлен старым — правка опубликованного артефакта.
			},
			wantWord: "auto-parts.vehicle-identification@1.0.0: артефакт изменён после индексации (sha256 не совпадает)",
		},
		{
			name: "пустой индекс",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				index.Entries = nil
				flushIndex(t, fsys, index)
			},
			wantWord: "индекс реестра tools: записи отсутствуют",
		},
		{
			name: "dependency без connector_type",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				rewriteArtifact(t, fsys, index.Entries[2], func(doc map[string]any) {
					doc["dependency"] = map[string]any{}
				})
				reindexPinned(t, fsys, index.Entries[2].ID, index.Entries[2].Version)
			},
			wantWord: "dependency: обязательное поле connector_type отсутствует",
		},
	}
}

// TestLoaderRejectsDuplicateTailEntry: дубликат ХВОСТОВОЙ записи индекса
// отказывается поимённо — хвост реестра растёт с каждым внешним
// дополнением (7-2 держит живую привязку + симуляцию внешнего PR),
// поэтому ключ вычисляется динамически, а не пинится словом.
func TestLoaderRejectsDuplicateTailEntry(t *testing.T) {
	fsys, index := baseFS(t)
	key := index.Entries[len(index.Entries)-1].ID + "@" + index.Entries[len(index.Entries)-1].Version
	index.Entries = append(index.Entries, index.Entries[len(index.Entries)-1])
	flushIndex(t, fsys, index)
	_, err := loadRegistry(fsys)
	if err == nil {
		t.Fatal("реестр tools загрузился — отказ не сработал")
	}
	if !strings.Contains(err.Error(), "индексная запись "+key+": дубликат") {
		t.Fatalf("отказ без поимённой причины:\nполучено: %v\nожидалось слово: индексная запись %s: дубликат", err, key)
	}
}

func TestLoaderRejectionMatrix(t *testing.T) {
	for _, test := range rejections(t) {
		t.Run(test.name, func(t *testing.T) {
			fsys, index := baseFS(t)
			test.mutate(t, fsys, index)
			_, err := loadRegistry(fsys)
			if err == nil {
				t.Fatal("реестр tools загрузился — отказ не сработал")
			}
			if !strings.Contains(err.Error(), test.wantWord) {
				t.Fatalf("отказ без поимённой причины:\nполучено: %v\nожидалось слово: %s", err, test.wantWord)
			}
			t.Logf("отказ: %v", err)
		})
	}
}

// TestLoaderNegativeControl: the unmutated LIVE registry loads — every
// refusal above is a mutation effect, never a broken base.
func TestLoaderNegativeControl(t *testing.T) {
	fsys, _ := baseFS(t)
	contracts, err := loadRegistry(fsys)
	if err != nil {
		t.Fatalf("живой реестр tools не грузится: %v", err)
	}
	// The witness-subseries: the three live contracts stay first; registry
	// additions (the external author's PR) are allowed — the negative
	// control never pins the registry size (the Story 7-1 precedent).
	want := []string{
		"auto-parts.vehicle-identification@1.0.0",
		"auto-parts.part-request-classification@1.0.0",
		"auto-parts.fitment-evidence@1.0.0",
	}
	if len(contracts) < len(want) {
		t.Fatalf("реестр tools меньше среза: %d < %d", len(contracts), len(want))
	}
	for i, key := range want {
		if contracts[i].ID+"@"+contracts[i].Version != key {
			t.Fatalf("witness-подсерия нарушена на %d: %s", i, contracts[i].ID+"@"+contracts[i].Version)
		}
	}
}

// TestLoaderFailsWholeRegistry: one invalid artifact refuses EVERYTHING —
// the valid contracts of a broken registry are never served.
func TestLoaderFailsWholeRegistry(t *testing.T) {
	fsys, index := baseFS(t)
	rewriteArtifact(t, fsys, index.Entries[1], func(doc map[string]any) { doc["determinism_class"] = "write" })
	reindexPinned(t, fsys, index.Entries[1].ID, index.Entries[1].Version)
	contracts, err := loadRegistry(fsys)
	if err == nil && len(contracts) > 0 {
		t.Fatal("частичный словарь tools выдан — fail-closed нарушен")
	}
	if err == nil {
		t.Fatal("реестр загрузился — отказ не сработал")
	}
}
