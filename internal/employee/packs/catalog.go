package packs

import "github.com/staffy-co/staffy/internal/employee/domain"

// The catalog serves the published Domain Pack manifests from the artifact
// registry (Story 7-1): JSON artifacts pinned by sha256 in the append-only
// index, embedded into the binary and fully validated at load. The registry
// is private and every caller receives a copy, keeping published versions
// immutable. The load is fail-closed: one invalid artifact refuses the
// WHOLE registry — a partial catalog never exists (CI catches the break
// before deploy; the runtime panics instead of serving half a catalog).

var registry = mustLoadRegistry()

func mustLoadRegistry() []domain.PackManifest {
	manifests, err := loadRegistry(registryFiles)
	if err != nil {
		panic("pack registry: " + err.Error())
	}
	return manifests
}

func Get(id, version string) (domain.PackManifest, bool) {
	for _, m := range registry {
		if m.ID == id && m.Version == version {
			return clone(m), true
		}
	}
	return domain.PackManifest{}, false
}
func List() []domain.PackManifest {
	out := make([]domain.PackManifest, 0, len(registry))
	for _, m := range registry {
		out = append(out, clone(m))
	}
	return out
}
func Impact(from domain.PackPin, to domain.PackManifest) (domain.PackImpact, bool) {
	if from.ID != to.ID {
		return to.ReplacementImpact, to.ReplacementImpact.Compatibility != ""
	}
	impact, ok := to.UpgradeFrom[from.Version]
	return impact, ok
}

// clone returns the deep copy of a registry manifest: the labels of the
// schema fields and the replacement impact arrive FROM the artifacts
// (Story 7-1 moved them out of the code), and every slice, map and nested
// struct is copied so a caller's mutation is invisible to the next reader.
// Nil optionals stay nil (the wire form omits them either way).
func clone(m domain.PackManifest) domain.PackManifest {
	m.ConnectorTypes = append([]string(nil), m.ConnectorTypes...)
	m.Journeys = append([]string(nil), m.Journeys...)
	m.CriticalEntities = append([]string(nil), m.CriticalEntities...)
	m.Policies = append([]string(nil), m.Policies...)
	m.Capabilities = append([]string(nil), m.Capabilities...)
	m.Gates = append([]string(nil), m.Gates...)
	m.AtLeastOneOf = append([]string(nil), m.AtLeastOneOf...)
	m.Tools = append([]domain.ToolRef(nil), m.Tools...)
	m.ReplacementImpact.Changed = append([]string(nil), m.ReplacementImpact.Changed...)
	m.ReplacementImpact.ReviewSlots = append([]string(nil), m.ReplacementImpact.ReviewSlots...)
	if m.VehicleIdentification != nil {
		keys := make([]domain.VehicleMaterialKey, 0, len(m.VehicleIdentification.MaterialKeys))
		for _, key := range m.VehicleIdentification.MaterialKeys {
			keys = append(keys, key)
		}
		sets := make([][]string, 0, len(m.VehicleIdentification.AlternativeSets))
		for _, set := range m.VehicleIdentification.AlternativeSets {
			sets = append(sets, append([]string(nil), set...))
		}
		m.VehicleIdentification = &domain.VehicleIdentificationSchema{MaterialKeys: keys, AlternativeSets: sets}
	}
	if m.Fitment != nil {
		fit := *m.Fitment
		fit.LinkTypes = append([]string(nil), m.Fitment.LinkTypes...)
		fit.AllowedSources = append([]string(nil), m.Fitment.AllowedSources...)
		m.Fitment = &fit
	}
	if m.Comparison != nil {
		cmp := *m.Comparison
		cmp.Currencies = append([]string(nil), m.Comparison.Currencies...)
		cmp.RankCriteria = append([]string(nil), m.Comparison.RankCriteria...)
		cmp.RankCriteriaLabels = map[string]string{}
		for criterion, label := range m.Comparison.RankCriteriaLabels {
			cmp.RankCriteriaLabels[criterion] = label
		}
		cmp.TieBreakers = append([]string(nil), m.Comparison.TieBreakers...)
		m.Comparison = &cmp
	}
	if m.PartRequests != nil {
		partKeys := make([]domain.PartMaterialKey, 0, len(m.PartRequests.MaterialKeys))
		for _, key := range m.PartRequests.MaterialKeys {
			key.Values = append([]string(nil), key.Values...)
			partKeys = append(partKeys, key)
		}
		modes := make([]domain.PartModeSpec, 0, len(m.PartRequests.Modes))
		for _, mode := range m.PartRequests.Modes {
			mode.Requires = append([]string(nil), mode.Requires...)
			modes = append(modes, mode)
		}
		m.PartRequests = &domain.PartRequestSchema{MaterialKeys: partKeys, Modes: modes, SafetyMarkers: append([]string(nil), m.PartRequests.SafetyMarkers...), PartSearchJourney: m.PartRequests.PartSearchJourney}
	}
	schema := map[string]domain.VariantField{}
	for k, v := range m.Schema {
		v.Enum = append([]string(nil), v.Enum...)
		schema[k] = v
	}
	m.Schema = schema
	if m.UpgradeFrom != nil {
		upgrades := map[string]domain.PackImpact{}
		for k, v := range m.UpgradeFrom {
			v.Changed = append([]string(nil), v.Changed...)
			v.ReviewSlots = append([]string(nil), v.ReviewSlots...)
			upgrades[k] = v
		}
		m.UpgradeFrom = upgrades
	}
	return m
}
