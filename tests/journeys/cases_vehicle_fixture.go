package main

import (
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.2 vehicle identification fixture material: the schema of
// the pinned auto-parts@3.0.0 (the fixture mirror of the pack manifest —
// the fixtureJourneys precedent: the fixture resolves the same
// dictionaries the real catalog resolves), the vehicle profile
// derivation over the fixture facts (the REAL domain function — the
// fixture never invents statuses) and the public surface vehicle block.
// ---------------------------------------------------------------------------

// fixtureVehicleSchema is the identification schema of auto-parts@3.0.0
// (the mirror of the pack manifest — the six material keys with the
// kinds and the alternative sufficient sets).
func fixtureVehicleSchema() casesDomain.VehicleSchema {
	return casesDomain.VehicleSchema{
		MaterialKeys: []casesDomain.VehicleKeySpec{
			{Key: "vehicle.vin", Label: "VIN", Kind: "vin"},
			{Key: "vehicle.make", Label: "Марка", Kind: "text"},
			{Key: "vehicle.model", Label: "Модель", Kind: "text"},
			{Key: "vehicle.year_of_manufacture", Label: "Год выпуска", Kind: "integer", Min: 1900, Max: 2100},
			{Key: "vehicle.trim", Label: "Модификация", Kind: "text", Optional: true},
			{Key: "vehicle.engine", Label: "Двигатель", Kind: "text", Optional: true},
		},
		AlternativeSets: [][]string{
			{"vehicle.vin"},
			{"vehicle.make", "vehicle.model", "vehicle.year_of_manufacture"},
		},
	}
}

// fixtureVehicleFacts collects the vehicle fact inputs of one case (the
// REAL domain derivation input: the anchors, the versions and the
// confirmations of the fixture storage).
func (f *casesFixture) fixtureVehicleFacts(caseID id.UUID) []casesDomain.VehicleFactInput {
	inputs := []casesDomain.VehicleFactInput{}
	for _, factID := range f.factAnchorsOfCase(caseID) {
		versions := f.factVersions[factID]
		if len(versions) == 0 {
			continue
		}
		inputs = append(inputs, casesDomain.VehicleFactInput{
			FactID: factID, MaterialKey: versions[0].MaterialKey,
			Versions: versions, Confirmations: f.factConfirms[factID],
		})
	}
	return inputs
}

// fixtureVehiclePanelOf derives the Story 4.2 panel of the fixture case
// view over the REAL domain function (the statuses, the completeness,
// the digest and the stale words are the derivations, never fixture
// inventions). The packs without the identification schema render the
// honest absence word (the 2-2 unreadable-pin degradation mirror).
func fixtureVehiclePanelOf(packVersion string, facts []casesDomain.VehicleFactInput) casesAPI.VehiclePanelView {
	// The additive manifest mirror: every pack from the identification
	// pin (3.0.0) upward carries the schema — the 4.0.0/5.0.0/6.0.0
	// worlds render the panel (the real auto-parts manifests keep it; the
	// strict equality was a fixture simplification the order contour of
	// 4.6 exposed: its builder reads the LIVE panel digests).
	if packVersion != fixtureVehiclePackVersion && packVersion != fixturePartPackVersion &&
		packVersion != fixtureFitmentPackVersion && packVersion != fixtureOffersPackVersion {
		return casesAPI.VehiclePanelView{Note: casesAPI.VehicleSchemaNoteAbsent}
	}
	return casesAPI.VehiclePanelView{
		SchemaDefined: true,
		Profile:       casesDomain.DeriveVehicleProfile(fixtureVehicleSchema(), facts),
	}
}

// fixtureVehicleSurface builds the public vehicle block of the
// continuation surface: the profile over the facts of the WORK ITEM of
// the link (the one-surface discipline — the confirmable attributes
// only) with the readback blocks and the focus words of the VIN.
func (f *casesFixture) fixtureVehicleSurface(link casesDomain.ContinuationLink) *casesAPI.ContinuationVehicleView {
	if material := f.material[link.CaseID]; material == nil || material.cases.PackVersion != fixtureVehiclePackVersion {
		// The packs without the identification schema render NO vehicle
		// block (the honest absence — the neighboring journeys keep their
		// frozen button counts, the rule 161).
		return nil
	}
	inputs := []casesDomain.VehicleFactInput{}
	for _, factID := range f.factAnchorsOfCase(link.CaseID) {
		versions := f.factVersions[factID]
		if len(versions) == 0 || versions[len(versions)-1].WorkItemID != link.WorkItemID {
			continue
		}
		inputs = append(inputs, casesDomain.VehicleFactInput{
			FactID: factID, MaterialKey: versions[0].MaterialKey,
			Versions: versions, Confirmations: f.factConfirms[factID],
		})
	}
	if len(inputs) == 0 {
		return nil
	}
	profile := casesDomain.DeriveVehicleProfile(fixtureVehicleSchema(), inputs)
	block := &casesAPI.ContinuationVehicleView{
		Profile:    profile,
		Readback:   []casesDomain.VINReadbackBlock{},
		FocusWords: []string{},
	}
	for _, attribute := range profile.Attributes {
		if attribute.Kind == "vin" && attribute.NormalizedValue != nil {
			block.Readback = casesDomain.ReadbackBlocks(attribute.NormalizedValue.Normalized)
			block.FocusWords = casesDomain.ReadbackFocus(attribute.NormalizedValue.Normalized, attribute.NormalizedValue.CyrillicAmbiguities)
			break
		}
	}
	return block
}
