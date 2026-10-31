package tools_test

// Story 7-2 step-01: the three vertical tool artifacts parse into the
// ToolContract type with every mandatory field present, the platform
// dictionaries respected (deterministic_read — the single class;
// not_implemented — the honest adapter state) and the fitment dependency
// declared. The full load-time validation with the reject matrix is the
// loader of step-02; this witness proves the CARRIERS: the artifacts and
// the index are in place and parseable before the loader exists.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/staffy-co/staffy/internal/tools"
)

type toolIndexEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
}

type toolIndexFile struct {
	Entries []toolIndexEntry `json:"entries"`
}

func loadToolIndex(t *testing.T) toolIndexFile {
	t.Helper()
	raw, err := os.ReadFile("registry/index.json")
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var idx toolIndexFile
	if err := json.Unmarshal(raw, &idx); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	return idx
}

// TestToolArtifactsCarriers: every artifact of the tool registry parses,
// carries every mandatory field, respects the platform dictionaries and
// matches its sha256 pin in the append-only index; the file names follow
// the <id>@<version>.json canon.
func TestToolArtifactsCarriers(t *testing.T) {
	idx := loadToolIndex(t)
	// The witness-subseries of the first slice: the three live contracts
	// stay in place; registry additions (the external author's PR) are
	// ALLOWED — the loader's validation covers them (the Story 7-1
	// subseries precedent).
	wantSeries := []string{
		"auto-parts.vehicle-identification@1.0.0",
		"auto-parts.part-request-classification@1.0.0",
		"auto-parts.fitment-evidence@1.0.0",
	}
	if len(idx.Entries) < len(wantSeries) {
		t.Fatalf("реестр tools меньше среза: %d < %d", len(idx.Entries), len(wantSeries))
	}
	cursor := 0
	for _, entry := range idx.Entries {
		if cursor < len(wantSeries) && wantSeries[cursor] == entry.ID+"@"+entry.Version {
			cursor++
		}
	}
	if cursor != len(wantSeries) {
		t.Fatalf("witness-подсерия tools нарушена: совпало %d из %d", cursor, len(wantSeries))
	}
	seen := map[string]bool{}
	for _, entry := range idx.Entries {
		key := entry.ID + "@" + entry.Version
		if seen[key] {
			t.Fatalf("%s: дубликат в индексе", key)
		}
		seen[key] = true
		if entry.File != key+".json" {
			t.Fatalf("%s: имя файла %s нарушает канон <id>@<version>.json", key, entry.File)
		}
		raw, err := os.ReadFile("registry/" + entry.File)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != entry.SHA256 {
			t.Fatalf("%s: артефакт изменён после индексации (sha256 не совпадает)", key)
		}
		var contract tools.ToolContract
		if err := json.Unmarshal(raw, &contract); err != nil {
			t.Fatalf("%s: parse: %v", key, err)
		}
		if contract.ID != entry.ID || contract.Version != entry.Version {
			t.Fatalf("%s: id/version артефакта не совпадают с индексной записью", key)
		}
		if contract.Name == "" || contract.Purpose == "" {
			t.Fatalf("%s: обязательные поля name/purpose отсутствуют", key)
		}
		if contract.Owner == "" || contract.EvidenceRef == "" {
			t.Fatalf("%s: обязательные поля owner/evidence_ref отсутствуют", key)
		}
		if contract.DeterminismClass != "deterministic_read" {
			t.Fatalf("%s: determinism_class %q вне платформенного словаря", key, contract.DeterminismClass)
		}
		if contract.AdapterStatus != "implemented" && contract.AdapterStatus != "not_implemented" {
			t.Fatalf("%s: adapter_status %q вне платформенного словаря", key, contract.AdapterStatus)
		}
		if len(contract.InputSchema) == 0 || len(contract.OutputSchema) == 0 {
			t.Fatalf("%s: схемы input/output отсутствуют", key)
		}
	}
}

// TestToolArtifactsHonestAdapters: every LIVE contract of the slice
// declares the honest adapter state — not_implemented. There is no
// implemented adapter in the system (the Montagio/Avito discipline: a
// capability is never claimed above the proof).
func TestToolArtifactsHonestAdapters(t *testing.T) {
	idx := loadToolIndex(t)
	for _, entry := range idx.Entries {
		raw, err := os.ReadFile("registry/" + entry.File)
		if err != nil {
			t.Fatalf("%s: %v", entry.File, err)
		}
		var contract tools.ToolContract
		if err := json.Unmarshal(raw, &contract); err != nil {
			t.Fatalf("%s: parse: %v", entry.File, err)
		}
		if contract.AdapterStatus != "not_implemented" {
			t.Fatalf("%s@%s: живой контракт среза обязан быть честно not_implemented, получено %q", contract.ID, contract.Version, contract.AdapterStatus)
		}
	}
}

// TestToolFitmentDependency: the fitment evidence contract declares the
// catalog connector type dependency (the cross-validation against the
// binding pack's connector_types is the loader of step-02 — the carrier
// is declared here).
func TestToolFitmentDependency(t *testing.T) {
	raw, err := os.ReadFile("registry/auto-parts.fitment-evidence@1.0.0.json")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var contract tools.ToolContract
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if contract.Dependency == nil {
		t.Fatal("fitment-evidence обязан декларировать dependency")
	}
	if contract.Dependency.ConnectorType != "catalog" {
		t.Fatalf("dependency.connector_type: %q, ожидался catalog", contract.Dependency.ConnectorType)
	}
}
