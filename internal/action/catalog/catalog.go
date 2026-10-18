package catalog

import (
	"github.com/artemiosu/staffy/internal/action/domain"
)

// definitions is the immutable binary catalog of Authorized Business Actions.
//
// The catalog is HONESTLY EMPTY — this is a state, not a defect. A first
// entry may appear only when ALL of the following hold:
//  1. a Domain Pack is published and references a connector type,
//  2. that pack's connector declares query/command capabilities
//     (a real CapabilityDeclaration, not StatusDeferred with none),
//  3. the definition's PackRef and RequiredCapability resolve against the
//     live employee and connector catalogs (app layer, through ports),
//  4. GuardValidateActionDefinition and the delivery-word cross-check pass.
//
// Today no published pack is linked to a connector that declares any
// capability (auto-parts/montagio-services reference catalog/inventory/
// service_catalog connector types whose connectors are StatusDeferred with
// zero capabilities; the only capability-bearing connector — the synthetic
// telephony — is referenced by no pack). Forcing a pack binding without the
// pack's connector is forbidden (AG-4 gates real providers). The test
// circuit closes the vertical with fixture definitions registered by the
// test composition root — never in this product binary.
var definitions []domain.ActionDefinition

// List returns a defensive copy of the newest version of every action
// definition. Pinned older versions stay resolvable through Get only, so
// new references always bind the current contract (precedent 1-8).
func List() []domain.ActionDefinition {
	return ListFrom(definitions)
}

// Get pins one definition by id and exact version. Older versions remain
// immutable and resolvable — the material for future Manifest/Operation/
// Receipt references (AC-8); they are never offered as new.
func Get(id, version string) (domain.ActionDefinition, bool) {
	return GetFrom(definitions, id, version)
}

// Latest returns the newest version of a definition and reports whether the
// requested version is that current version (precedent 1-8).
func Latest(id, requestedVersion string) (domain.ActionDefinition, bool, bool) {
	return LatestFrom(definitions, id, requestedVersion)
}

// All returns defensive copies of EVERY entry of the binary dictionary —
// all versions, not just the newest. This is the material of the admission
// pass (ValidateBinaryCatalog walks it through the guard integrity plus the
// reference resolution and the delivery-word cross-check), so a pinned old
// version is held to the same admission truth as the newest one.
func All() []domain.ActionDefinition {
	out := make([]domain.ActionDefinition, 0, len(definitions))
	for _, d := range definitions {
		out = append(out, clone(d))
	}
	return out
}

// ListFrom is the slice-parameterized core of List: the first entry of every
// ID — the catalog is maintained newest-first (the 1-8 convention), so the
// first occurrence IS the newest version. The test composition root uses it
// to serve fixture definitions (the vertical of this slice) without touching
// the product binary.
func ListFrom(defs []domain.ActionDefinition) []domain.ActionDefinition {
	out := make([]domain.ActionDefinition, 0, len(defs))
	seen := map[string]bool{}
	for _, d := range defs {
		if seen[d.ID] {
			continue
		}
		seen[d.ID] = true
		out = append(out, clone(d))
	}
	return out
}

// GetFrom is the slice-parameterized core of Get: the exact pin pair.
func GetFrom(defs []domain.ActionDefinition, id, version string) (domain.ActionDefinition, bool) {
	for _, d := range defs {
		if d.ID == id && d.Version == version {
			return clone(d), true
		}
	}
	return domain.ActionDefinition{}, false
}

// LatestFrom is the slice-parameterized core of Latest.
func LatestFrom(defs []domain.ActionDefinition, id, requestedVersion string) (domain.ActionDefinition, bool, bool) {
	newest, found := newestOf(defs, id)
	if !found {
		return domain.ActionDefinition{}, false, false
	}
	return clone(newest), requestedVersion == newest.Version, true
}

// newestOf finds the entry with the maximum version of one ID.
func newestOf(defs []domain.ActionDefinition, id string) (domain.ActionDefinition, bool) {
	var newest domain.ActionDefinition
	found := false
	for _, d := range defs {
		if d.ID != id {
			continue
		}
		if !found || compareVersions(d.Version, newest.Version) > 0 {
			newest = d
			found = true
		}
	}
	return newest, found
}

// compareVersions orders simple dotted semver-ish strings ("1.10.0" > "1.9.0").
func compareVersions(a, b string) int {
	partsA, partsB := splitVersion(a), splitVersion(b)
	for i := 0; i < 3; i++ {
		if partsA[i] != partsB[i] {
			if partsA[i] > partsB[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func splitVersion(v string) [3]int {
	var out [3]int
	current, index := 0, 0
	for _, r := range v {
		if r == '.' {
			if index < 2 {
				out[index] = current
				index++
				current = 0
			}
			continue
		}
		if r >= '0' && r <= '9' {
			current = current*10 + int(r-'0')
		}
	}
	out[index] = current
	return out
}

func clone(d domain.ActionDefinition) domain.ActionDefinition {
	d.TypedInputSchema = append([]domain.FieldSchema(nil), d.TypedInputSchema...)
	d.TypedOutputSchema = append([]domain.FieldSchema(nil), d.TypedOutputSchema...)
	d.Conditions.AllowedConversationModes = append([]string(nil), d.Conditions.AllowedConversationModes...)
	d.Conditions.AllowedRoles = append([]string(nil), d.Conditions.AllowedRoles...)
	d.Conditions.RequiredFacts = append([]domain.FactRequirement(nil), d.Conditions.RequiredFacts...)
	d.Conditions.RequiredGates = append([]string(nil), d.Conditions.RequiredGates...)
	d.Conditions.AllowedStages = append([]string(nil), d.Conditions.AllowedStages...)
	return d
}
