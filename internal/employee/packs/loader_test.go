package packs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/artemiosu/staffy/internal/employee/domain"
)

// The rejection matrix of Story 7-1 step-02: every case builds an in-memory
// registry from the REAL committed artifacts and mutates exactly one thing,
// asserting the NAMED load-time refusal. The loader is fail-closed: one
// invalid artifact rejects the whole registry — a partial catalog never
// exists.

type testCase struct {
	name     string
	mutate   func(t *testing.T, fsys fstest.MapFS, index *registryIndex)
	wantWord string // the named refusal substring
}

func baseFS(t *testing.T) (fstest.MapFS, *registryIndex) {
	t.Helper()
	fsys := fstest.MapFS{}
	idxRaw, err := os.ReadFile("registry/index.json")
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var index registryIndex
	if err := json.Unmarshal(idxRaw, &index); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	for _, entry := range index.Entries {
		raw, err := os.ReadFile("registry/" + entry.File)
		if err != nil {
			t.Fatalf("read artifact: %v", err)
		}
		fsys["registry/"+entry.File] = &fstest.MapFile{Data: raw}
	}
	fsys["registry/index.json"] = &fstest.MapFile{Data: idxRaw}
	return fsys, &index
}

// writeManifest stores the manifest as its artifact file and updates (or
// appends) the index entry with the matching sha256 pin.
func writeManifest(t *testing.T, fsys fstest.MapFS, index *registryIndex, m domain.PackManifest) {
	t.Helper()
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	raw = append(raw, '\n')
	file := m.ID + "@" + m.Version + ".json"
	fsys["registry/"+file] = &fstest.MapFile{Data: raw}
	sum := sha256.Sum256(raw)
	entry := indexEntry{ID: m.ID, Version: m.Version, File: file, SHA256: hex.EncodeToString(sum[:])}
	replaced := false
	for i := range index.Entries {
		if index.Entries[i].ID == m.ID && index.Entries[i].Version == m.Version {
			index.Entries[i] = entry
			replaced = true
		}
	}
	if !replaced {
		index.Entries = append(index.Entries, entry)
	}
	flushIndex(t, fsys, index)
}

func flushIndex(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
	t.Helper()
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	raw = append(raw, '\n')
	fsys["registry/index.json"] = &fstest.MapFile{Data: raw}
}

func readManifest(t *testing.T, fsys fstest.MapFS, id, version string) domain.PackManifest {
	t.Helper()
	raw, ok := fsys["registry/"+id+"@"+version+".json"]
	if !ok {
		t.Fatalf("artifact %s@%s missing from fs", id, version)
	}
	var m domain.PackManifest
	if err := json.Unmarshal(raw.Data, &m); err != nil {
		t.Fatalf("parse %s@%s: %v", id, version, err)
	}
	return m
}

func rejections() []testCase {
	return []testCase{
		{
			name: "обязательное поле eval_ref отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				m.EvalRef = ""
				writeManifest(t, fsys, index, m)
			},
			wantWord: "auto-parts@6.0.0: обязательное поле eval_ref отсутствует",
		},
		{
			name: "обязательное поле name отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "montagio-services", "1.0.0")
				m.Name = ""
				writeManifest(t, fsys, index, m)
			},
			wantWord: "montagio-services@1.0.0: обязательное поле name отсутствует",
		},
		{
			name: "обязательное поле journeys отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				m.Journeys = nil
				writeManifest(t, fsys, index, m)
			},
			wantWord: "auto-parts@6.0.0: обязательное поле journeys отсутствует",
		},
		{
			name: "support_status вне словаря",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				m.SupportStatus = "ga"
				writeManifest(t, fsys, index, m)
			},
			wantWord: "support_status: значение ga вне платформенного словаря",
		},
		{
			name: "stage вне словаря",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				m.Stage = "live"
				writeManifest(t, fsys, index, m)
			},
			wantWord: "stage: значение live вне платформенного словаря",
		},
		{
			name: "review_slots вне словаря (replacement_impact)",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "montagio-services", "1.0.0")
				m.ReplacementImpact.ReviewSlots = []string{"billing"}
				writeManifest(t, fsys, index, m)
			},
			wantWord: "review_slots: значение billing вне платформенного словаря",
		},
		{
			name: "review_slots вне словаря (upgrade_from)",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				impact := m.UpgradeFrom["5.0.0"]
				impact.ReviewSlots = []string{"billing"}
				m.UpgradeFrom["5.0.0"] = impact
				writeManifest(t, fsys, index, m)
			},
			wantWord: "review_slots: значение billing вне платформенного словаря",
		},
		{
			name: "тип поля схемы вне словаря",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				field := m.Schema["matching_depth"]
				field.Type = "float"
				m.Schema["matching_depth"] = field
				writeManifest(t, fsys, index, m)
			},
			wantWord: "schema.matching_depth: тип float вне платформенного словаря",
		},
		{
			name: "вид ключа идентификации вне словаря",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				keys := make([]domain.VehicleMaterialKey, len(m.VehicleIdentification.MaterialKeys))
				copy(keys, m.VehicleIdentification.MaterialKeys)
				keys[0].Kind = "date"
				m.VehicleIdentification.MaterialKeys = keys
				writeManifest(t, fsys, index, m)
			},
			wantWord: "vehicle_identification.vehicle.vin: вид date вне платформенного словаря",
		},
		{
			name: "вид ключа запроса вне словаря",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				keys := make([]domain.PartMaterialKey, len(m.PartRequests.MaterialKeys))
				copy(keys, m.PartRequests.MaterialKeys)
				keys[0].Kind = "boolean"
				m.PartRequests.MaterialKeys = keys
				writeManifest(t, fsys, index, m)
			},
			wantWord: "part_requests.part.kind: вид boolean вне платформенного словаря",
		},
		{
			name: "AtLeastOneOf ключ вне schema",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "montagio-services", "1.0.0")
				m.AtLeastOneOf = []string{"plumbing"}
				writeManifest(t, fsys, index, m)
			},
			wantWord: "ключ plumbing не описан в schema",
		},
		{
			name: "UpgradeFrom-цель отсутствует в реестре",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				impact := m.UpgradeFrom["5.0.0"]
				delete(m.UpgradeFrom, "5.0.0")
				m.UpgradeFrom["0.9.0"] = impact
				writeManifest(t, fsys, index, m)
			},
			wantWord: "UpgradeFrom-цель auto-parts@0.9.0 отсутствует в реестре",
		},
		{
			name: "major без UpgradeFrom-записи",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				m.Version = "7.0.0"
				m.UpgradeFrom = nil
				m.PolicyRef = "auto-parts/policy/7.0.0"
				m.KnowledgeRef = "auto-parts/knowledge/7.0.0"
				m.EvalRef = "auto-parts/eval/7.0.0"
				writeManifest(t, fsys, index, m)
			},
			wantWord: "auto-parts@7.0.0: major-переход без UpgradeFrom-записи невозможен",
		},
		{
			name: "replacement_impact пустой",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				m.ReplacementImpact.Changed = nil
				writeManifest(t, fsys, index, m)
			},
			wantWord: "replacement_impact: обязательные поля compatibility/changed/review_slots отсутствуют",
		},
		{
			name: "неверный формат версии",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				m := readManifest(t, fsys, "auto-parts", "6.0.0")
				m.Version = "6.x"
				m.UpgradeFrom = nil
				m.PolicyRef = "auto-parts/policy/6.x"
				m.KnowledgeRef = "auto-parts/knowledge/6.x"
				m.EvalRef = "auto-parts/eval/6.x"
				writeManifest(t, fsys, index, m)
			},
			wantWord: "неверный формат версии 6.x",
		},
		{
			name: "артефакт изменён после индексации",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				file := fsys["registry/auto-parts@6.0.0.json"]
				file.Data = append(append([]byte(nil), file.Data...), []byte(" ")...)
			},
			wantWord: "auto-parts@6.0.0: артефакт изменён после индексации (sha256 не совпадает)",
		},
		{
			name: "битый JSON",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				file := fsys["registry/montagio-services@1.0.0.json"]
				file.Data = []byte("{ not json")
				sum := sha256.Sum256(file.Data)
				for i := range index.Entries {
					if index.Entries[i].ID == "montagio-services" {
						index.Entries[i].SHA256 = hex.EncodeToString(sum[:])
					}
				}
				flushIndex(t, fsys, index)
			},
			wantWord: "montagio-services@1.0.0: JSON:",
		},
		{
			name: "дубликат индексной записи",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				index.Entries = append(index.Entries, index.Entries[len(index.Entries)-1])
				flushIndex(t, fsys, index)
			},
			// Последняя witness-запись (montagio) стабильна как замыкающая
			// golden-подсерии; дублирование головы реестра проверяется
			// отдельным динамическим тестом ниже (голова растёт с реестром).
			wantWord: "индексная запись montagio-services@1.0.0: дубликат",
		},
		{
			name: "имя файла нарушает канон",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				index.Entries[0].File = "weird-name.json"
				flushIndex(t, fsys, index)
			},
			wantWord: "имя файла weird-name.json нарушает канон",
		},
		{
			name: "файл не индексирован",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				fsys["registry/auto-parts@0.1.0.json"] = &fstest.MapFile{Data: fsys["registry/auto-parts@6.0.0.json"].Data}
			},
			wantWord: "файл auto-parts@0.1.0.json не индексирован",
		},
		{
			name: "индексированный файл отсутствует",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				delete(fsys, "registry/auto-parts@5.0.0.json")
			},
			wantWord: "auto-parts@5.0.0:",
		},
		{
			name: "пустой индекс",
			mutate: func(t *testing.T, fsys fstest.MapFS, index *registryIndex) {
				index.Entries = nil
				flushIndex(t, fsys, index)
			},
			wantWord: "индекс реестра: записи отсутствуют",
		},
	}
}

// TestLoaderRejectsDuplicateHeadEntry: дубликат ГОЛОВНОЙ записи индекса
// отказывается поимённо — голова реестра растёт с каждым дополнением
// (7-2 добавила auto-parts@6.1.0), поэтому ключ вычисляется динамически,
// а не пинится словом.
func TestLoaderRejectsDuplicateHeadEntry(t *testing.T) {
	fsys, index := baseFS(t)
	key := index.Entries[0].ID + "@" + index.Entries[0].Version
	index.Entries = append(index.Entries, index.Entries[0])
	flushIndex(t, fsys, index)
	_, err := loadRegistry(fsys)
	if err == nil {
		t.Fatal("реестр загрузился — отказ не сработал")
	}
	if !strings.Contains(err.Error(), "индексная запись "+key+": дубликат") {
		t.Fatalf("отказ без поимённой причины:\nполучено: %v\nожидалось слово: индексная запись %s: дубликат", err, key)
	}
}

func TestLoaderRejectionMatrix(t *testing.T) {
	for _, test := range rejections() {
		t.Run(test.name, func(t *testing.T) {
			fsys, index := baseFS(t)
			test.mutate(t, fsys, index)
			_, err := loadRegistry(fsys)
			if err == nil {
				t.Fatal("реестр загрузился — отказ не сработал")
			}
			if !strings.Contains(err.Error(), test.wantWord) {
				t.Fatalf("отказ без поимённой причины:\nполучено: %v\nожидалось слово: %s", err, test.wantWord)
			}
			t.Logf("отказ: %v", err)
		})
	}
}

// TestLoaderAcceptsMinorWithoutUpgradeFrom: a minor of a published major
// without UpgradeFrom stays compatible by default — the major rule fires
// only for a NEW major (the spec: minor/patch без UpgradeFrom допустимы).
func TestLoaderAcceptsMinorWithoutUpgradeFrom(t *testing.T) {
	fsys, index := baseFS(t)
	base := len(index.Entries)
	m := readManifest(t, fsys, "auto-parts", "6.0.0")
	m.Version = "6.0.1"
	m.UpgradeFrom = nil
	m.PolicyRef = "auto-parts/policy/6.0.1"
	m.KnowledgeRef = "auto-parts/knowledge/6.0.1"
	m.EvalRef = "auto-parts/eval/6.0.1"
	writeManifest(t, fsys, index, m)
	manifests, err := loadRegistry(fsys)
	if err != nil {
		t.Fatalf("minor без UpgradeFrom должен допускаться: %v", err)
	}
	if len(manifests) != base+1 {
		t.Fatalf("ожидалось %d манифестов, получено %d", base+1, len(manifests))
	}
}

// TestLoaderLoadsEmbeddedRegistry: the REAL embedded registry loads clean,
// and the golden sequence appears in it in the same relative order,
// byte-stably equal per entry — registry additions beyond the golden load
// through the same validation and stay allowed.
func TestLoaderLoadsEmbeddedRegistry(t *testing.T) {
	manifests, err := loadRegistry(registryFiles)
	if err != nil {
		t.Fatalf("живой реестр не загрузился: %v", err)
	}
	goldenRaw, err := os.ReadFile("testdata/pack_catalog_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		List []domain.PackManifest `json:"list"`
	}
	if err := json.Unmarshal(goldenRaw, &golden); err != nil {
		t.Fatal(err)
	}
	if len(manifests) < len(golden.List) {
		t.Fatalf("ожидалось не меньше %d манифестов, получено %d", len(golden.List), len(manifests))
	}
	cursor := 0
	for i := range manifests {
		if cursor >= len(golden.List) {
			break
		}
		if manifests[i].ID == golden.List[cursor].ID && manifests[i].Version == golden.List[cursor].Version {
			liveRaw, err := json.Marshal(manifests[i])
			if err != nil {
				t.Fatal(err)
			}
			goldenEntry, err := json.Marshal(golden.List[cursor])
			if err != nil {
				t.Fatal(err)
			}
			if string(liveRaw) != string(goldenEntry) {
				t.Fatalf("манифест %s@%s != golden байт-стабильно", manifests[i].ID, manifests[i].Version)
			}
			cursor++
		}
	}
	if cursor != len(golden.List) {
		t.Fatalf("golden-последовательность нарушена в реестре: совпало %d из %d", cursor, len(golden.List))
	}
}
