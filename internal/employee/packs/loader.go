package packs

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/artemiosu/staffy/internal/employee/domain"
)

// The registry is the versioned artifact store of the published Domain
// Packs (Story 7-1): every published manifest is a JSON artifact pinned by
// its sha256 in the append-only index, embedded into the binary and
// validated BEFORE any use. The loader is fail-closed: one invalid artifact
// fails the WHOLE registry — a partial catalog never exists, the CI catches
// the break before deploy, and the runtime panics at load instead of
// serving half a catalog. Published versions are immutable: touching an
// artifact's bytes changes its sha256 and fails the pin; a new version is
// a new file plus a new index entry.

//go:embed registry/*.json
var registryFiles embed.FS

const registryDir = "registry"
const registryIndexFile = registryDir + "/index.json"

// indexEntry pins one published artifact: the pack id, the version, the
// artifact file name inside registry/ and the sha256 of its exact bytes.
type indexEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
}

type registryIndex struct {
	Entries []indexEntry `json:"entries"`
}

// The platform dictionaries: the pack USES these words, it never extends
// them — a value outside a dictionary is a load-time rejection (extending
// a dictionary is a core change, never a pack change).
var (
	packSupportStatusDictionary = map[string]bool{"reference_setup_only": true}
	packStageDictionary         = map[string]bool{"setup": true}
	variantTypeDictionary       = map[string]bool{"string": true, "integer": true, "boolean": true}
	vehicleKindDictionary       = map[string]bool{"vin": true, "text": true, "integer": true}
	partKindDictionary          = map[string]bool{"text": true, "integer": true, "enum": true}
	reviewSlotDictionary        = map[string]bool{"connectors": true, "knowledge": true, "behavior": true}
)

// loadRegistry reads the index and every artifact from fsys, verifies the
// sha256 pins, validates every manifest and the cross-manifest rules, and
// returns the manifests in the canonical index order — or the first named
// reason the registry is not loadable.
func loadRegistry(fsys fs.FS) ([]domain.PackManifest, error) {
	indexRaw, err := fs.ReadFile(fsys, registryIndexFile)
	if err != nil {
		return nil, fmt.Errorf("индекс реестра: %w", err)
	}
	var index registryIndex
	if err := json.Unmarshal(indexRaw, &index); err != nil {
		return nil, fmt.Errorf("индекс реестра: JSON: %v", err)
	}
	if len(index.Entries) == 0 {
		return nil, fmt.Errorf("индекс реестра: записи отсутствуют")
	}

	byKey := map[string]domain.PackManifest{}
	manifests := make([]domain.PackManifest, 0, len(index.Entries))
	referenced := map[string]bool{}
	seen := map[string]bool{}
	for _, entry := range index.Entries {
		key := entry.ID + "@" + entry.Version
		if seen[key] || referenced[entry.File] {
			return nil, fmt.Errorf("индексная запись %s: дубликат", key)
		}
		seen[key] = true
		if entry.File != key+".json" {
			return nil, fmt.Errorf("индексная запись %s: имя файла %s нарушает канон <id>@<version>.json", key, entry.File)
		}
		path := registryDir + "/" + entry.File
		raw, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", key, err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != entry.SHA256 {
			return nil, fmt.Errorf("%s: артефакт изменён после индексации (sha256 не совпадает)", key)
		}
		var manifest domain.PackManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, fmt.Errorf("%s: JSON: %v", key, err)
		}
		if err := validateManifest(manifest); err != nil {
			return nil, err
		}
		byKey[key] = manifest
		manifests = append(manifests, manifest)
		referenced[entry.File] = true
	}

	// Every artifact file must be indexed: an unindexed file means the
	// registry and the directory disagree — an author forgot the index
	// entry, and fail-closed refuses the ambiguity.
	entries, err := fs.ReadDir(fsys, registryDir)
	if err != nil {
		return nil, fmt.Errorf("реестр: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "index.json" || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !referenced[entry.Name()] {
			return nil, fmt.Errorf("файл %s не индексирован", entry.Name())
		}
	}

	for _, manifest := range manifests {
		for fromVersion := range manifest.UpgradeFrom {
			if _, ok := byKey[manifest.ID+"@"+fromVersion]; !ok {
				return nil, fmt.Errorf("%s@%s: UpgradeFrom-цель %s@%s отсутствует в реестре", manifest.ID, manifest.Version, manifest.ID, fromVersion)
			}
		}
	}
	if err := validateMajorTransitions(manifests); err != nil {
		return nil, err
	}
	return manifests, nil
}

// validateManifest checks the single-artifact rules: the mandatory fields,
// the platform dictionaries, and the AtLeastOneOf declaration.
func validateManifest(m domain.PackManifest) error {
	key := m.ID + "@" + m.Version
	if m.ID == "" || m.Version == "" {
		return fmt.Errorf("артефакт %s: обязательные поля id/version отсутствуют", key)
	}
	for _, field := range []struct {
		name  string
		empty bool
	}{
		{"name", m.Name == ""},
		{"purpose", m.Purpose == ""},
		{"support_status", m.SupportStatus == ""},
		{"stage", m.Stage == ""},
		{"connector_types", len(m.ConnectorTypes) == 0},
		{"journeys", len(m.Journeys) == 0},
		{"critical_entities", len(m.CriticalEntities) == 0},
		{"policies", len(m.Policies) == 0},
		{"capabilities", len(m.Capabilities) == 0},
		{"gates", len(m.Gates) == 0},
		{"schema", len(m.Schema) == 0},
		{"policy_ref", m.PolicyRef == ""},
		{"knowledge_ref", m.KnowledgeRef == ""},
		{"eval_ref", m.EvalRef == ""},
	} {
		if field.empty {
			return fmt.Errorf("%s: обязательное поле %s отсутствует", key, field.name)
		}
	}
	if _, _, _, err := parseSemver(m.Version); err != nil {
		return fmt.Errorf("%s: %v", key, err)
	}
	if !packSupportStatusDictionary[m.SupportStatus] {
		return fmt.Errorf("%s: support_status: значение %s вне платформенного словаря", key, m.SupportStatus)
	}
	if !packStageDictionary[m.Stage] {
		return fmt.Errorf("%s: stage: значение %s вне платформенного словаря", key, m.Stage)
	}
	for name, field := range m.Schema {
		if !variantTypeDictionary[field.Type] {
			return fmt.Errorf("%s: schema.%s: тип %s вне платформенного словаря", key, name, field.Type)
		}
	}
	if m.VehicleIdentification != nil {
		for _, materialKey := range m.VehicleIdentification.MaterialKeys {
			if !vehicleKindDictionary[materialKey.Kind] {
				return fmt.Errorf("%s: vehicle_identification.%s: вид %s вне платформенного словаря", key, materialKey.Key, materialKey.Kind)
			}
		}
	}
	if m.PartRequests != nil {
		for _, materialKey := range m.PartRequests.MaterialKeys {
			if !partKindDictionary[materialKey.Kind] {
				return fmt.Errorf("%s: part_requests.%s: вид %s вне платформенного словаря", key, materialKey.Key, materialKey.Kind)
			}
		}
	}
	for _, slot := range append(append([]string(nil), m.ReplacementImpact.ReviewSlots...), reviewSlotsOf(m.UpgradeFrom)...) {
		if !reviewSlotDictionary[slot] {
			return fmt.Errorf("%s: review_slots: значение %s вне платформенного словаря", key, slot)
		}
	}
	if m.ReplacementImpact.Compatibility == "" || len(m.ReplacementImpact.Changed) == 0 || len(m.ReplacementImpact.ReviewSlots) == 0 {
		return fmt.Errorf("%s: replacement_impact: обязательные поля compatibility/changed/review_slots отсутствуют", key)
	}
	for _, name := range m.AtLeastOneOf {
		if _, ok := m.Schema[name]; !ok {
			return fmt.Errorf("%s: ключ %s не описан в schema", key, name)
		}
	}
	return nil
}

// validateMajorTransitions enforces the published-major rule: a version
// whose major is NEW for the pack (no other published version of the same
// id carries that major) must declare an UpgradeFrom entry — a jump across
// majors without a declared impact is a refusal. Minor and patch versions
// of an already published major stay compatible by default (the current
// catalog semantics, preserved).
func validateMajorTransitions(manifests []domain.PackManifest) error {
	majorOf := func(m domain.PackManifest) (int, error) {
		major, _, _, err := parseSemver(m.Version)
		if err != nil {
			return 0, fmt.Errorf("%s@%s: %v", m.ID, m.Version, err)
		}
		return major, nil
	}
	for _, m := range manifests {
		if len(m.UpgradeFrom) > 0 {
			continue
		}
		major, err := majorOf(m)
		if err != nil {
			return err
		}
		others, sameMajor := 0, false
		for _, w := range manifests {
			if w.ID != m.ID || w.Version == m.Version {
				continue
			}
			others++
			wMajor, err := majorOf(w)
			if err != nil {
				return err
			}
			if wMajor == major {
				sameMajor = true
			}
		}
		if others > 0 && !sameMajor {
			return fmt.Errorf("%s@%s: major-переход без UpgradeFrom-записи невозможен", m.ID, m.Version)
		}
	}
	return nil
}

// parseSemver accepts the strict semantic core major.minor.patch — the
// registry publishes released versions only, without prerelease or build
// tags (an honest boundary of the slice, extendable by a core change).
func parseSemver(version string) (int, int, int, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("неверный формат версии %s (ожидается major.minor.patch)", version)
	}
	numbers := make([]int, 3)
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 || (len(part) > 1 && part[0] == '0') {
			return 0, 0, 0, fmt.Errorf("неверный формат версии %s (ожидается major.minor.patch)", version)
		}
		numbers[i] = number
	}
	return numbers[0], numbers[1], numbers[2], nil
}

func reviewSlotsOf(upgrades map[string]domain.PackImpact) []string {
	slots := make([]string, 0, len(upgrades))
	for _, impact := range upgrades {
		slots = append(slots, impact.ReviewSlots...)
	}
	return slots
}
