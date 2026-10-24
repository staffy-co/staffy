package main

import (
	"context"
	"fmt"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.4 fitment fixture material: the policy of the pinned
// auto-parts@5.0.0 (the fixture mirror of the pack manifest — the
// fixtureVehicle/fixturePart precedent: the fixture resolves the same
// dictionaries the real catalog resolves, the REAL domain functions
// derive the panel and guard the record), the in-memory append-only
// evidence storage with the monotonic numbers per (case, position,
// source) and the record command of the fixture cases service.
// ---------------------------------------------------------------------------

// fixtureFitmentPackVersion is the Story 4.4 pin of the fitment world
// (the c2xx slot of the retro-3-3 register, the next free step after the
// part request 4.0.0 slot): the sessions carrying the fitment prefix pin
// auto-parts@5.0.0 — the policy-bearing pack the fitment panel derives
// over. Every other session keeps its world pin (the frozen words of the
// neighboring journeys stay byte-stable — the rule 161).
const fixtureFitmentPackVersion = "5.0.0"

// fixtureFitmentSessionPrefix marks the ingress sessions of the fitment
// world (the pin convention of the fixture — deterministic, no new
// toggles: the session id itself carries the world).
const fixtureFitmentSessionPrefix = "journey-fitment-"

// fixtureFitmentSession answers whether one ingress session belongs to
// the fitment world (the 5.0.0 pin).
func fixtureFitmentSession(sessionID string) bool {
	return len(sessionID) >= len(fixtureFitmentSessionPrefix) && sessionID[:len(fixtureFitmentSessionPrefix)] == fixtureFitmentSessionPrefix
}

// fixtureFitmentPolicy is the fitment evidence policy of
// auto-parts@5.0.0 (the mirror of the pack manifest — the link types,
// the maximal cross chain depth, the allowed catalog sources and the
// maximal evidence age).
func fixtureFitmentPolicy() casesDomain.FitmentPolicyDomain {
	return casesDomain.FitmentPolicyDomain{
		LinkTypes:      []string{casesDomain.FitmentLinkOriginal, casesDomain.FitmentLinkCross},
		MaxCrossDepth:  3,
		AllowedSources: []string{casesDomain.FitmentSourceAbcpCatalog, casesDomain.FitmentSourceLaximoCatalog},
		MaxAgeSeconds:  604800,
	}
}

// fixtureFitmentPins derives the CURRENT digest pins of BOTH inputs of
// one case over the REAL domain derivations (the same live derivations
// of the real app command — the operator never inputs the pins; the
// confirmed sets of the vehicle profile of 4.2 and the part request of
// 4.3 are the pin material).
func (f *casesFixture) fixtureFitmentPins(caseID id.UUID) (profile string, request string) {
	facts := f.fixtureVehicleFacts(caseID)
	profile = casesDomain.DeriveVehicleProfile(fixtureVehicleSchema(), facts).ProfileDigest
	request = casesDomain.DerivePartRequest(fixturePartSchema(), f.fixturePartFacts(caseID), f.fixtureActivePartSearches(caseID)).PartRequestDigest
	return profile, request
}

// fixtureFitmentRecord appends one evidence record of the in-memory
// storage (the monotonic number per (case, position, source) — the
// supersede is the NEW number, never an update; the pins derive from the
// live derivations of the moment; the canonical digest is the REAL
// domain function).
func (f *casesFixture) fixtureFitmentRecord(command casesAPI.RecordFitmentEvidenceCommand) (casesAPI.WorkOutcome, error) {
	if command.PositionKey == "" {
		return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "record_fitment_evidence", Reason: "ключ позиции обязателен: запись доказательства привязывается к позиции материала кейса"}
	}
	profilePin, requestPin := f.fixtureFitmentPins(command.CaseID)
	draft := casesDomain.FitmentEvidenceDraft{
		PositionKey:          command.PositionKey,
		CatalogNode:          command.CatalogNode,
		Article:              command.Article,
		LinkType:             command.LinkType,
		Chain:                command.Chain,
		CatalogVehicleIDs:    command.CatalogVehicleIDs,
		SourceSystem:         command.SourceSystem,
		SourceRecordID:       command.SourceRecordID,
		SourceVersion:        command.SourceVersion,
		SourceObservedAt:     command.SourceObservedAt,
		ProfileDigestPin:     profilePin,
		PartRequestDigestPin: requestPin,
	}
	if err := casesDomain.GuardFitmentEvidence(fixtureFitmentPolicy(), draft); err != nil {
		return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "record_fitment_evidence", Reason: err.Error()}
	}
	nextNo := int64(0)
	for _, evidence := range f.fitmentEvidences[command.CaseID] {
		if evidence.PositionKey == command.PositionKey && evidence.SourceSystem == command.SourceSystem && evidence.EvidenceNo > nextNo {
			nextNo = evidence.EvidenceNo
		}
	}
	f.fitmentEvidenceSeq++
	evidence := casesDomain.FitmentEvidenceInput{
		EvidenceNo:           nextNo + 1,
		PositionKey:          command.PositionKey,
		CatalogNode:          command.CatalogNode,
		Article:              command.Article,
		LinkType:             command.LinkType,
		Chain:                command.Chain,
		CatalogVehicleIDs:    command.CatalogVehicleIDs,
		SourceSystem:         command.SourceSystem,
		SourceRecordID:       command.SourceRecordID,
		SourceVersion:        command.SourceVersion,
		SourceObservedAt:     command.SourceObservedAt,
		ReceivedAt:           command.Now,
		ProfileDigestPin:     profilePin,
		PartRequestDigestPin: requestPin,
	}
	f.fitmentEvidences[command.CaseID] = append(f.fitmentEvidences[command.CaseID], evidence)
	return casesAPI.WorkOutcome{
		Command:            "record_fitment_evidence",
		FitmentEvidenceID:  id.MustParse("01994f64-6c00-7000-8000-" + fmt.Sprintf("%012d", 30000+f.fitmentEvidenceSeq)),
		FitmentEvidenceNo:  evidence.EvidenceNo,
		FitmentPositionKey: command.PositionKey,
		FitmentDigest:      casesDomain.FitmentDigestOf(evidence),
	}, nil
}

// RecordFitmentEvidence records one manager's catalog research result of
// the fixture world (the FitmentService surface of the fixture cases
// service — the same guard, pins and digest derivations as the real app).
func (f *casesFixture) RecordFitmentEvidence(_ context.Context, command casesAPI.RecordFitmentEvidenceCommand) (casesAPI.WorkOutcome, error) {
	return f.fixtureFitmentRecord(command)
}

// fixtureFitmentPanelOf derives the Story 4.4 panel of the fixture case
// view over the REAL domain function (the state ladder, the stale words,
// the questions and the digest are the derivations, never fixture
// inventions). The packs without the fitment policy render the honest
// absence word (the 4.2/4.3 panel family degradation mirror).
func (f *casesFixture) fixtureFitmentPanelOf(packVersion string, caseID id.UUID) casesAPI.FitmentPanelView {
	// The packs carrying the fitment policy render the panel: the 5.0.0
	// pin of the fitment world AND the 6.0.0 pin of the offers world (the
	// additive manifest keeps the policy — the honest mirror of the real
	// catalog; every earlier pin renders the honest absence word).
	if packVersion != fixtureFitmentPackVersion && packVersion != fixtureOffersPackVersion {
		return casesAPI.FitmentPanelView{Note: casesAPI.FitmentSchemaNoteAbsent}
	}
	profilePin, requestPin := f.fixtureFitmentPins(caseID)
	// The positions of the case records grouped by the position key (the
	// ordered grouping of the derivation input).
	positions := []casesDomain.FitmentPositionInput{}
	byPosition := map[string]*casesDomain.FitmentPositionInput{}
	positionOrder := []string{}
	for _, evidence := range f.fitmentEvidences[caseID] {
		if _, ok := byPosition[evidence.PositionKey]; !ok {
			byPosition[evidence.PositionKey] = &casesDomain.FitmentPositionInput{PositionKey: evidence.PositionKey, Evidences: []casesDomain.FitmentEvidenceInput{}}
			positionOrder = append(positionOrder, evidence.PositionKey)
		}
		byPosition[evidence.PositionKey].Evidences = append(byPosition[evidence.PositionKey].Evidences, evidence)
	}
	for _, key := range positionOrder {
		positions = append(positions, *byPosition[key])
	}
	// The safety-node flag over the part kind material (the 4.3
	// discipline — the pack markers over the normalized kind).
	safetyNode := false
	for _, fact := range f.fixturePartFacts(caseID) {
		if fact.MaterialKey == "part.kind" && len(fact.Versions) > 0 {
			kind := fmt.Sprintf("%s", fact.Versions[len(fact.Versions)-1].Value)
			for _, marker := range fixturePartSchema().SafetyMarkers {
				if containsFold(kind, marker) {
					safetyNode = true
				}
			}
		}
	}
	// The clarifiable profile attributes (the optional identification
	// keys with the confirmed flags — AC-5).
	clarifiable := []casesDomain.FitmentProfileAttribute{}
	for _, key := range fixtureVehicleSchema().MaterialKeys {
		if !key.Optional {
			continue
		}
		confirmed := false
		for _, fact := range f.fixtureVehicleFacts(caseID) {
			if fact.MaterialKey != key.Key || len(fact.Versions) == 0 {
				continue
			}
			status, _ := casesDomain.DeriveFactStatus(fact.Versions, fact.Confirmations)
			confirmed = status == casesDomain.FactStatusConfirmed
		}
		clarifiable = append(clarifiable, casesDomain.FitmentProfileAttribute{Key: key.Key, Label: key.Label, Confirmed: confirmed})
	}
	return casesAPI.FitmentPanelView{
		SchemaDefined: true,
		Fitment: casesDomain.DeriveFitment(casesDomain.FitmentDerivationInput{
			Policy:                       fixtureFitmentPolicy(),
			Positions:                    positions,
			ProfileDigest:                profilePin,
			PartRequestDigest:            requestPin,
			Now:                          journeyClock.Now(),
			SafetyNode:                   safetyNode,
			ClarifiableProfileAttributes: clarifiable,
		}),
	}
}

// containsFold answers whether the value carries the word (the
// case-insensitive substring — the marker search of the safety flag).
func containsFold(value, word string) bool {
	return len(value) >= len(word) && (value == word ||
		(len(value) > len(word) && (indexOfFold(value, word) >= 0)))
}

// indexOfFold finds the case-insensitive index of the word (the simple
// ASCII fold of the fixture helper).
func indexOfFold(value, word string) int {
	for i := 0; i+len(word) <= len(value); i++ {
		if equalFold(value[i:i+len(word)], word) {
			return i
		}
	}
	return -1
}

// equalFold answers the ASCII case-insensitive equality.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
