package packs_test

// Story 7-2 step-01: the pack artifact auto-parts@6.1.0 is the first
// LIVE tool binding — the semantic diff against 6.0.0 is exactly the
// tools field (every other field stays byte-stable, the immutability of
// the published content), the index entry is pinned by the sha256 of the
// exact artifact bytes, and the golden witness-subseries of Story 7-1
// survives the eighth index entry (the registry grows, the published
// order of the snapshot seven never reshuffles).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/packs"
)

// TestPackToolBindingArtifact: the 6.1.0 artifact carries exactly the
// three declared tool refs and nothing else changes against 6.0.0.
func TestPackToolBindingArtifact(t *testing.T) {
	raw60, err := os.ReadFile("registry/auto-parts@6.0.0.json")
	if err != nil {
		t.Fatalf("read 6.0.0: %v", err)
	}
	raw61, err := os.ReadFile("registry/auto-parts@6.1.0.json")
	if err != nil {
		t.Fatalf("read 6.1.0: %v", err)
	}
	var m60, m61 domain.PackManifest
	if err := json.Unmarshal(raw60, &m60); err != nil {
		t.Fatalf("parse 6.0.0: %v", err)
	}
	if err := json.Unmarshal(raw61, &m61); err != nil {
		t.Fatalf("parse 6.1.0: %v", err)
	}
	if len(m61.Tools) != 3 {
		t.Fatalf("ожидалось 3 tool-рефа, получено %d", len(m61.Tools))
	}
	want := []domain.ToolRef{
		{ID: "auto-parts.vehicle-identification", Version: "1.0.0"},
		{ID: "auto-parts.part-request-classification", Version: "1.0.0"},
		{ID: "auto-parts.fitment-evidence", Version: "1.0.0"},
	}
	if !reflect.DeepEqual(m61.Tools, want) {
		t.Fatalf("tool-рефы не совпадают: %+v", m61.Tools)
	}
	// The semantic diff against 6.0.0 is exactly version + tools.
	m60Tools := m60.Tools
	m61Tools := m61.Tools
	m60.Tools, m61.Tools = nil, nil
	defer func() { m60.Tools, m61.Tools = m60Tools, m61Tools }()
	m60Version, m61Version := m60.Version, m61.Version
	m60.Version, m61.Version = "", ""
	if !reflect.DeepEqual(m60, m61) {
		t.Fatal("semantic diff за пределами version+tools: содержимое 6.1.0 не байт-стабильно к 6.0.0")
	}
	_ = m60Version
	_ = m61Version
}

// TestPackToolBindingIndexPin: the eighth index entry pins the artifact
// bytes and the index still validates against the loader (the catalog
// serves the binding).
func TestPackToolBindingIndexPin(t *testing.T) {
	raw, err := os.ReadFile("registry/auto-parts@6.1.0.json")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	idxRaw, err := os.ReadFile("registry/index.json")
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var idx struct {
		Entries []struct {
			ID      string `json:"id"`
			Version string `json:"version"`
			File    string `json:"file"`
			SHA256  string `json:"sha256"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(idxRaw, &idx); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	found := false
	for _, entry := range idx.Entries {
		if entry.ID == "auto-parts" && entry.Version == "6.1.0" {
			found = true
			if entry.File != "auto-parts@6.1.0.json" {
				t.Fatalf("имя файла %s нарушает канон", entry.File)
			}
			sum := sha256.Sum256(raw)
			if hex.EncodeToString(sum[:]) != entry.SHA256 {
				t.Fatal("sha256-пин 6.1.0 не совпадает с байтами артефакта")
			}
		}
	}
	if !found {
		t.Fatal("запись auto-parts@6.1.0 отсутствует в индексе")
	}
	// The catalog serves the binding (the loader accepted the eighth entry).
	m, ok := packs.Get("auto-parts", "6.1.0")
	if !ok {
		t.Fatal("каталог не отдаёт auto-parts@6.1.0")
	}
	if len(m.Tools) != 3 {
		t.Fatalf("каталог отдал манифест без 3 tool-рефов: %d", len(m.Tools))
	}
}

// TestPackWithoutToolsStaysNil: every pack published before the binding
// carries nil tools — the immutability of the published versions is never
// violated (the wire form omits the field entirely).
func TestPackWithoutToolsStaysNil(t *testing.T) {
	for _, key := range [][2]string{
		{"auto-parts", "6.0.0"},
		{"auto-parts", "5.0.0"},
		{"auto-parts", "4.0.0"},
		{"auto-parts", "3.0.0"},
		{"auto-parts", "2.0.0"},
		{"auto-parts", "1.0.0"},
		{"montagio-services", "1.0.0"},
	} {
		m, ok := packs.Get(key[0], key[1])
		if !ok {
			t.Fatalf("%s@%s: отсутствует в каталоге", key[0], key[1])
		}
		if m.Tools != nil {
			t.Fatalf("%s@%s: опубликованная версия обязана оставаться без tool-привязки (nil)", key[0], key[1])
		}
	}
	// The 6.1.0 binding mutation is invisible to the next reader (the
	// immutability discipline of the catalog).
	m, _ := packs.Get("auto-parts", "6.1.0")
	m.Tools[0].ID = "hacked"
	m.Tools = append(m.Tools, domain.ToolRef{ID: "hacked", Version: "9.9.9"})
	fresh, _ := packs.Get("auto-parts", "6.1.0")
	if len(fresh.Tools) != 3 || fresh.Tools[0].ID != "auto-parts.vehicle-identification" {
		t.Fatal("мутация tool-привязки видна следующему читателю")
	}
}
