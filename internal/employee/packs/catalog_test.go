package packs_test

// Story 7-1 step-01: the golden fixtures below were snapshot from the LIVE
// binary catalog BEFORE the registry migration (a one-shot throwaway
// generator, deleted after the snapshot; the golden is a proof, never
// regenerated). These tests are the equivalence witness: they must keep
// passing byte-stably after the catalog switches to the artifact registry
// in step-03 — any divergence is a bug of the slice, not an "update the
// golden" situation.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/packs"
)

type indexEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
}

type indexFile struct {
	Entries []indexEntry `json:"entries"`
}

type impactProbe struct {
	From      domain.PackPin    `json:"from"`
	ToID      string            `json:"to_id"`
	ToVersion string            `json:"to_version"`
	Impact    domain.PackImpact `json:"impact"`
	OK        bool              `json:"ok"`
}

type goldenFile struct {
	List    []domain.PackManifest `json:"list"`
	Impacts []impactProbe         `json:"impacts"`
}

// canonical marshals deterministically (Go sorts map keys), so equal
// semantics produce equal bytes: the wire-level byte-stable contract.
func canonical(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	raw, err := os.ReadFile("testdata/pack_catalog_golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var golden goldenFile
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	return golden
}

func loadIndex(t *testing.T) indexFile {
	t.Helper()
	raw, err := os.ReadFile("registry/index.json")
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var idx indexFile
	if err := json.Unmarshal(raw, &idx); err != nil {
		t.Fatalf("parse index: %v", err)
	}
	return idx
}

// TestRegistryIndexCanonicalOrder: the golden sequence appears in the
// index in the same relative order — the registry grows (an external
// author appends new packs and versions with zero core edits), but the
// published order of the snapshot seven never reshuffles.
func TestRegistryIndexCanonicalOrder(t *testing.T) {
	golden := loadGolden(t)
	idx := loadIndex(t)
	live := packs.List()
	if len(idx.Entries) < len(golden.List) || len(live) < len(golden.List) {
		t.Fatalf("реестр меньше снапшота: index=%d golden=%d live=%d", len(idx.Entries), len(golden.List), len(live))
	}
	cursor := 0
	for _, entry := range idx.Entries {
		if cursor < len(golden.List) && golden.List[cursor].ID == entry.ID && golden.List[cursor].Version == entry.Version {
			cursor++
		}
	}
	if cursor != len(golden.List) {
		t.Fatalf("golden-последовательность нарушена в индексе: совпало %d из %d", cursor, len(golden.List))
	}
	cursor = 0
	for _, m := range live {
		if cursor < len(golden.List) && golden.List[cursor].ID == m.ID && golden.List[cursor].Version == m.Version {
			cursor++
		}
	}
	if cursor != len(golden.List) {
		t.Fatalf("golden-последовательность нарушена в List(): совпало %d из %d", cursor, len(golden.List))
	}
}

// TestRegistryArtifactsMatchGolden: every golden entry stays pinned in
// the index by the sha256 of its exact file bytes and its parsed semantics
// equal the golden snapshot byte-stably (labels and replacement_impact
// included). Registry additions beyond the golden are validated by the
// loader — the golden covers the snapshot seven.
func TestRegistryArtifactsMatchGolden(t *testing.T) {
	golden := loadGolden(t)
	idx := loadIndex(t)
	byKey := map[string]domain.PackManifest{}
	for _, m := range golden.List {
		byKey[m.ID+"@"+m.Version] = m
	}
	seen := map[string]bool{}
	for _, entry := range idx.Entries {
		want, ok := byKey[entry.ID+"@"+entry.Version]
		if !ok {
			continue // a registry addition — the loader's validation covers it
		}
		seen[entry.ID+"@"+entry.Version] = true
		raw, err := os.ReadFile("registry/" + entry.File)
		if err != nil {
			t.Fatalf("%s@%s: %v", entry.ID, entry.Version, err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != entry.SHA256 {
			t.Fatalf("%s@%s: sha256 drift — artifact changed after indexing", entry.ID, entry.Version)
		}
		var manifest domain.PackManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			t.Fatalf("%s@%s: parse: %v", entry.ID, entry.Version, err)
		}
		if got, wantJSON := canonical(t, manifest), canonical(t, want); got != wantJSON {
			t.Fatalf("%s@%s: artifact != golden\ngot:  %s\nwant: %s", entry.ID, entry.Version, got, wantJSON)
		}
		if !reflect.DeepEqual(manifest, parsed(t, canonical(t, want))) {
			t.Fatalf("%s@%s: artifact deep-equal mismatch", entry.ID, entry.Version)
		}
	}
	for _, m := range golden.List {
		if !seen[m.ID+"@"+m.Version] {
			t.Fatalf("%s@%s: golden-запись исчезла из индекса", m.ID, m.Version)
		}
	}
}

func parsed(t *testing.T, raw string) domain.PackManifest {
	t.Helper()
	var m domain.PackManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	return m
}

// TestCatalogImmutability: a caller's mutation of a returned manifest — at
// every nested spot, including the shared-backing arrays a shallow copy
// would leak — is invisible to the next reader: the registry serves deep
// copies only.
func TestCatalogImmutability(t *testing.T) {
	golden := loadGolden(t)
	m, ok := packs.Get("auto-parts", "6.0.0")
	if !ok {
		t.Fatal("missing auto-parts@6.0.0")
	}
	m.Name = "hacked"
	m.ConnectorTypes[0] = "hacked"
	m.Journeys[0] = "hacked"
	m.CriticalEntities[0] = "hacked"
	m.Policies[0] = "hacked"
	m.Capabilities[0] = "hacked"
	m.Gates[0] = "hacked"
	m.Schema["parts_interest"] = domain.VariantField{Type: "boolean"}
	m.VehicleIdentification.MaterialKeys[0].Label = "hacked"
	m.VehicleIdentification.AlternativeSets[0][0] = "hacked"
	m.PartRequests.MaterialKeys[0].Label = "hacked"
	m.PartRequests.MaterialKeys[3].Values[0] = "hacked"
	m.PartRequests.Modes[0].Mode = "hacked"
	m.PartRequests.SafetyMarkers[0] = "hacked"
	m.Fitment.AllowedSources[0] = "hacked"
	m.Comparison.RankCriteriaLabels["budget"] = "hacked"
	m.ReplacementImpact.Changed[0] = "hacked"
	upgradeImpact := m.UpgradeFrom["5.0.0"]
	upgradeImpact.Changed[0] = "hacked"
	m.UpgradeFrom["5.0.0"] = domain.PackImpact{Compatibility: "hacked"}

	again, _ := packs.Get("auto-parts", "6.0.0")
	if got, want := canonical(t, again), canonical(t, golden.List[0]); got != want {
		t.Fatalf("мутация копии видна следующему читателю\ngot:  %s\nwant: %s", got, want)
	}

	listed := packs.List()
	var listedFirst domain.PackManifest
	for _, m := range listed {
		if m.ID == "auto-parts" && m.Version == "6.0.0" {
			listedFirst = m
		}
	}
	listedFirst.Name = "hacked"
	listedFirst.Schema["parts_interest"] = domain.VariantField{Type: "boolean"}
	listedFirst.ReplacementImpact.ReviewSlots[0] = "hacked"
	fresh := packs.List()
	var freshFirst domain.PackManifest
	for _, m := range fresh {
		if m.ID == "auto-parts" && m.Version == "6.0.0" {
			freshFirst = m
		}
	}
	if got, want := canonical(t, freshFirst), canonical(t, golden.List[0]); got != want {
		t.Fatal("мутация List() видна следующему читателю")
	}
}

// TestGoldenMatchesLiveCatalog: the golden snapshot equals the live
// catalog semantics byte-stably — every golden manifest appears in List()
// in the same relative order and byte-identical, every golden Get()
// returns the golden bytes, and every Impact() probe replays (upgrade
// edges, cross-id replacement, the missing edge). Registry additions
// beyond the golden pass through the loader validation — this is the
// equivalence contract the registry switch must keep for the published
// seven.
func TestGoldenMatchesLiveCatalog(t *testing.T) {
	golden := loadGolden(t)

	live := packs.List()
	cursor := 0
	for _, m := range live {
		if cursor < len(golden.List) && golden.List[cursor].ID == m.ID && golden.List[cursor].Version == m.Version {
			if got, want := canonical(t, m), canonical(t, golden.List[cursor]); got != want {
				t.Fatalf("List(): %s@%s != golden\ngot:  %s\nwant: %s", m.ID, m.Version, got, want)
			}
			cursor++
		}
	}
	if cursor != len(golden.List) {
		t.Fatalf("golden-последовательность нарушена в List(): совпало %d из %d", cursor, len(golden.List))
	}

	for _, m := range golden.List {
		liveManifest, ok := packs.Get(m.ID, m.Version)
		if !ok {
			t.Fatalf("%s@%s: live Get missing", m.ID, m.Version)
		}
		if got, want := canonical(t, liveManifest), canonical(t, m); got != want {
			t.Fatalf("Get(%s@%s) != golden\ngot:  %s\nwant: %s", m.ID, m.Version, got, want)
		}
	}

	if _, ok := packs.Get("auto-parts", "7.0.0"); ok {
		t.Fatal("Get returned an unpublished version")
	}

	for _, probe := range golden.Impacts {
		to, ok := packs.Get(probe.ToID, probe.ToVersion)
		if !ok {
			t.Fatalf("probe target %s@%s missing", probe.ToID, probe.ToVersion)
		}
		impact, ok := packs.Impact(probe.From, to)
		if ok != probe.OK {
			t.Fatalf("Impact(%s@%s -> %s@%s): ok=%t, golden ok=%t", probe.From.ID, probe.From.Version, probe.ToID, probe.ToVersion, ok, probe.OK)
		}
		if got, want := canonical(t, impact), canonical(t, probe.Impact); got != want {
			t.Fatalf("Impact(%s@%s -> %s@%s)\ngot:  %s\nwant: %s", probe.From.ID, probe.From.Version, probe.ToID, probe.ToVersion, got, want)
		}
	}
}
