package main

import (
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.3 part request fixture material: the schema of the pinned
// auto-parts@4.0.0 (the fixture mirror of the pack manifest — the
// fixtureVehicle precedent: the fixture resolves the same dictionaries the
// real catalog resolves, the REAL domain function derives the panel), the
// active part-search count by the pack-declared journey word and the
// part request panel of the case view.
// ---------------------------------------------------------------------------

// fixturePartPackVersion is the Story 4.3 pin of the part request world
// (the c2xx slot of the retro-3-3 register, the next free step after the
// vehicle 3.0.0 slot): the sessions carrying the part prefix pin
// auto-parts@4.0.0 — the schema-bearing pack the part request panel
// derives over. Every other session keeps its world pin (the frozen words
// of the neighboring journeys stay byte-stable — the rule 161).
const fixturePartPackVersion = "4.0.0"

// fixturePartSessionPrefix marks the ingress sessions of the part request
// world (the pin convention of the fixture — deterministic, no new
// toggles: the session id itself carries the world).
const fixturePartSessionPrefix = "journey-part-"

// fixturePartSession answers whether one ingress session belongs to the
// part request world (the 4.0.0 pin).
func fixturePartSession(sessionID string) bool {
	return len(sessionID) >= len(fixturePartSessionPrefix) && sessionID[:len(fixturePartSessionPrefix)] == fixturePartSessionPrefix
}

// fixturePartSchema is the part request schema of auto-parts@4.0.0 (the
// mirror of the pack manifest — the ten material keys with the roles, the
// four modes with the priorities and the mandatory sets, the safety
// markers and the part-search journey word).
func fixturePartSchema() casesDomain.PartSchemaDomain {
	return casesDomain.PartSchemaDomain{
		MaterialKeys: []casesDomain.PartMaterialKeyDomain{
			{Key: "part.kind", Label: "Деталь", Kind: "text", Role: casesDomain.PartRolePartKind},
			{Key: "part.side", Label: "Сторона", Kind: "text", Optional: true},
			{Key: "part.quantity", Label: "Количество", Kind: "integer", Min: 1, Max: 999},
			{Key: "part.preference", Label: "Предпочтение", Kind: "enum", Values: []string{"original", "analogs", "both"}, Role: casesDomain.PartRolePreference},
			{Key: "part.oem_number", Label: "OEM-номер", Kind: "text", Role: casesDomain.PartRoleOemNumber},
			{Key: "part.symptom", Label: "Симптом", Kind: "text", Role: casesDomain.PartRoleSymptom},
			{Key: "part.budget", Label: "Бюджет", Kind: "text", Optional: true},
			{Key: "part.deadline", Label: "Срок", Kind: "text", Optional: true},
			{Key: "part.brands", Label: "Приемлемые бренды", Kind: "text", Optional: true},
			{Key: "part.constraints", Label: "Ограничения", Kind: "text", Optional: true},
		},
		Modes: []casesDomain.PartModeSpecDomain{
			{Mode: casesDomain.PartModeDiagnostic, Label: "Диагностика по симптому", Priority: 1, Requires: []string{"part.symptom"}},
			{Mode: casesDomain.PartModeExactRepeat, Label: "Точный повтор по номеру", Priority: 2, Requires: []string{"part.kind", "part.oem_number"}},
			{Mode: casesDomain.PartModeOriginalVsAnalog, Label: "Выбор: оригинал или аналог", Priority: 3, Requires: []string{"part.kind", "part.preference"}},
			{Mode: casesDomain.PartModeNamedPart, Label: "Названная деталь", Priority: 4, Requires: []string{"part.kind", "part.quantity"}},
		},
		SafetyMarkers: []string{"тормоз", "рул", "подушк", "ремн безопасн", "абс", "есп"},
	}
}

// fixturePartSearchJourney is the pack-declared journey word of the
// part-search items (the SeveralPartsWord count input — the STORE word of
// the pack, never a core literal).
const fixturePartSearchJourney = "поиск детали"

// fixturePartFacts collects the part request fact inputs of one case (the
// REAL domain derivation input: the anchors, the versions and the
// confirmations of the fixture storage).
func (f *casesFixture) fixturePartFacts(caseID id.UUID) []casesDomain.PartRequestFactInput {
	inputs := []casesDomain.PartRequestFactInput{}
	for _, factID := range f.factAnchorsOfCase(caseID) {
		versions := f.factVersions[factID]
		if len(versions) == 0 {
			continue
		}
		inputs = append(inputs, casesDomain.PartRequestFactInput{
			FactID: factID, MaterialKey: versions[0].MaterialKey,
			Versions: versions, Confirmations: f.factConfirms[factID],
		})
	}
	return inputs
}

// fixtureActivePartSearches counts the ACTIVE part-search work items of
// one case by the pack-declared journey word (the live app counting
// mirror — the non-terminal items of the declared type).
func (f *casesFixture) fixtureActivePartSearches(caseID id.UUID) int {
	count := 0
	for _, anchor := range f.items[caseID] {
		current, known := f.workCurrentOf(anchor.WorkItemID)
		if !known || current.Terminal || current.Type != fixturePartSearchJourney {
			continue
		}
		count++
	}
	return count
}

// fixturePartPanelOf derives the Story 4.3 panel of the fixture case view
// over the REAL domain function (the mode, the attributes, the
// completeness, the digest, the confidence word and the boundaries are
// the derivations, never fixture inventions). The packs without the part
// request schema render the honest absence word (the 2.2
// unreadable-pin degradation mirror).
func (f *casesFixture) fixturePartPanelOf(packVersion string, caseID id.UUID) casesAPI.PartPanelView {
	if packVersion != fixturePartPackVersion {
		return casesAPI.PartPanelView{Note: casesAPI.PartSchemaNoteAbsent}
	}
	return casesAPI.PartPanelView{
		SchemaDefined: true,
		Part:          casesDomain.DerivePartRequest(fixturePartSchema(), f.fixturePartFacts(caseID), f.fixtureActivePartSearches(caseID)),
	}
}
