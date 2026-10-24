package main

import (
	"context"
	"fmt"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.5 offers fixture material: the policy of the pinned
// auto-parts@6.0.0 (the fixture mirror of the pack manifest — the
// fixtureVehicle/fixturePart/fixtureFitment precedent: the fixture
// resolves the same dictionaries the real catalog resolves, the REAL
// domain functions derive the panel and guard the records), the
// in-memory append-only offer and selection storages with the monotonic
// numbers and the record commands of the fixture cases service.
// ---------------------------------------------------------------------------

// fixtureOffersPackVersion is the Story 4.5 pin of the offers world (the
// d2xx slot of the retro-3-3 register, the next free step after the
// fitment 5.0.0 slot): the sessions carrying the offers prefix pin
// auto-parts@6.0.0 — the policy-bearing pack the comparison panel derives
// over. Every other session keeps its world pin (the frozen words of the
// neighboring journeys stay byte-stable — the rule 161).
const fixtureOffersPackVersion = "6.0.0"

// fixtureOffersSessionPrefix marks the ingress sessions of the offers
// world (the pin convention of the fixture — deterministic, no new
// toggles: the session id itself carries the world).
const fixtureOffersSessionPrefix = "journey-offers-"

// fixtureOffersSession answers whether one ingress session belongs to
// the offers world (the 6.0.0 pin).
func fixtureOffersSession(sessionID string) bool {
	return len(sessionID) >= len(fixtureOffersSessionPrefix) && sessionID[:len(fixtureOffersSessionPrefix)] == fixtureOffersSessionPrefix
}

// fixtureComparisonPolicy is the offer comparison policy of
// auto-parts@6.0.0 (the mirror of the pack manifest — the currencies, the
// maximal offer age, the ranking criteria with the labels, the
// deterministic tie breakers and the maximal visible variant count).
func fixtureComparisonPolicy() casesDomain.ComparisonPolicyDomain {
	return casesDomain.ComparisonPolicyDomain{
		Currencies:         []string{casesDomain.OfferCurrencyRUB},
		MaxOfferAgeSeconds: 86400,
		RankCriteria:       []string{casesDomain.OfferRankBudget, casesDomain.OfferRankBrand, casesDomain.OfferRankTerm},
		RankCriteriaLabels: map[string]string{casesDomain.OfferRankBudget: "Бюджет", casesDomain.OfferRankBrand: "Бренд", casesDomain.OfferRankTerm: "Срок"},
		TieBreakers:        []string{casesDomain.OfferTiePrice, casesDomain.OfferTieTerm, casesDomain.OfferTieBrand, casesDomain.OfferTieSupplier},
		MaxVisibleVariants: 4,
	}
}

// fixtureOffersPins derives the CURRENT digest pins of BOTH fact inputs
// of one case over the REAL domain derivations (the same live
// derivations of the real app command — the operator never inputs the
// pins).
func (f *casesFixture) fixtureOffersPins(caseID id.UUID) (profile string, request string) {
	facts := f.fixtureVehicleFacts(caseID)
	profile = casesDomain.DeriveVehicleProfile(fixtureVehicleSchema(), facts).ProfileDigest
	request = casesDomain.DerivePartRequest(fixturePartSchema(), f.fixturePartFacts(caseID), f.fixtureActivePartSearches(caseID)).PartRequestDigest
	return profile, request
}

// fixtureOffersFitmentAnswer derives the read-time fitment answer of one
// position over the REAL 4.4 derivation (the comparison READS the fitment
// output — the availability never changes the fitment status, AC-1).
func (f *casesFixture) fixtureOffersFitmentAnswer(caseID id.UUID, positionKey string) (status string, digest string, linkType string) {
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
	profilePin, requestPin := f.fixtureOffersPins(caseID)
	view := casesDomain.DeriveFitment(casesDomain.FitmentDerivationInput{
		Policy:            fixtureFitmentPolicy(),
		Positions:         positions,
		ProfileDigest:     profilePin,
		PartRequestDigest: requestPin,
		Now:               journeyClock.Now(),
	})
	for _, position := range view.Positions {
		if position.PositionKey == positionKey {
			for _, evidence := range position.Evidences {
				if len(evidence.CatalogVehicleIDs) > 0 {
					return position.Status, position.FitmentDigest, evidence.LinkType
				}
			}
			if len(position.Evidences) > 0 {
				return position.Status, position.FitmentDigest, position.Evidences[0].LinkType
			}
			return position.Status, position.FitmentDigest, ""
		}
	}
	return casesDomain.FitmentStatusUnknown, "", ""
}

// fixtureSupplierOfferRecord appends one offer record of the in-memory
// storage (the monotonic number per (case, position, supplier) — the
// supersede is the NEW number, never an update; the pins derive from the
// live derivations of the moment; the canonical digest is the REAL
// domain function; the store-side derivation mirrors the real store: the
// number is the server truth).
func (f *casesFixture) fixtureSupplierOfferRecord(command casesAPI.RecordSupplierOfferCommand) (casesAPI.WorkOutcome, error) {
	profilePin, requestPin := f.fixtureOffersPins(command.CaseID)
	_, fitmentPin, _ := f.fixtureOffersFitmentAnswer(command.CaseID, command.PositionKey)
	draft := casesDomain.SupplierOfferDraft{
		PositionKey:          command.PositionKey,
		SupplierID:           command.SupplierID,
		SupplierName:         command.SupplierName,
		Brand:                command.Brand,
		Article:              command.Article,
		ClientPrice:          command.ClientPrice,
		PurchasePrice:        command.PurchasePrice,
		Currency:             command.Currency,
		QuantityAvailable:    command.QuantityAvailable,
		TermDays:             command.TermDays,
		Conditions:           command.Conditions,
		SourceSystem:         command.SourceSystem,
		SourceRecordID:       command.SourceRecordID,
		SourceVersion:        command.SourceVersion,
		SourceObservedAt:     command.SourceObservedAt,
		ProfileDigestPin:     profilePin,
		PartRequestDigestPin: requestPin,
		FitmentDigestPin:     fitmentPin,
	}
	if err := casesDomain.GuardSupplierOffer(fixtureComparisonPolicy(), draft); err != nil {
		return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "record_supplier_offer", Reason: err.Error()}
	}
	nextNo := int64(0)
	for _, offer := range f.supplierOffers[command.CaseID] {
		if offer.PositionKey == command.PositionKey && offer.SupplierID == command.SupplierID && offer.OfferNo > nextNo {
			nextNo = offer.OfferNo
		}
	}
	f.supplierOfferSeq++
	offer := casesDomain.SupplierOfferInput{
		OfferNo:              nextNo + 1,
		PositionKey:          command.PositionKey,
		SupplierID:           command.SupplierID,
		SupplierName:         command.SupplierName,
		Brand:                command.Brand,
		Article:              command.Article,
		ClientPrice:          command.ClientPrice,
		PurchasePrice:        command.PurchasePrice,
		Currency:             command.Currency,
		QuantityAvailable:    command.QuantityAvailable,
		TermDays:             command.TermDays,
		Conditions:           command.Conditions,
		SourceSystem:         command.SourceSystem,
		SourceRecordID:       command.SourceRecordID,
		SourceVersion:        command.SourceVersion,
		SourceObservedAt:     command.SourceObservedAt,
		ReceivedAt:           command.Now,
		ProfileDigestPin:     profilePin,
		PartRequestDigestPin: requestPin,
		FitmentDigestPin:     fitmentPin,
	}
	f.supplierOffers[command.CaseID] = append(f.supplierOffers[command.CaseID], offer)
	return casesAPI.WorkOutcome{
		Command:          "record_supplier_offer",
		SupplierOfferID:  id.MustParse("01994f64-6c00-7000-8000-" + fmt.Sprintf("%012d", 40000+f.supplierOfferSeq)),
		OfferPositionKey: command.PositionKey,
	}, nil
}

// RecordSupplierOffer records one manager's commercial conditions
// research result of the fixture world (the OffersService surface of the
// fixture cases service — the same guard, pins and digest derivations as
// the real app).
func (f *casesFixture) RecordSupplierOffer(_ context.Context, command casesAPI.RecordSupplierOfferCommand) (casesAPI.WorkOutcome, error) {
	return f.fixtureSupplierOfferRecord(command)
}

// fixtureClientSelectionRecord assembles and appends one selection
// snapshot of the in-memory storage (the whole guard over the LIVE
// material — the current offer of the supplier elected by the maximal
// number, the proven fitment answer, the CURRENT pins; the snapshot pins
// ALL FOUR inputs, AC-13; the monotonic number per case).
func (f *casesFixture) fixtureClientSelectionRecord(command casesAPI.RecordClientSelectionCommand) (casesAPI.WorkOutcome, error) {
	profilePin, requestPin := f.fixtureOffersPins(command.CaseID)
	fitmentStatus, fitmentPin, _ := f.fixtureOffersFitmentAnswer(command.CaseID, command.PositionKey)
	var chosen *casesDomain.SupplierOfferInput
	for index := range f.supplierOffers[command.CaseID] {
		offer := f.supplierOffers[command.CaseID][index]
		if offer.PositionKey != command.PositionKey || offer.SupplierID != command.SupplierID {
			continue
		}
		if chosen == nil || offer.OfferNo > chosen.OfferNo {
			chosen = &f.supplierOffers[command.CaseID][index]
		}
	}
	if chosen == nil {
		return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "record_client_selection", Reason: casesDomain.OfferSelectionGuardStaleWord}
	}
	draft := casesDomain.ClientSelectionDraft{
		PositionKey:         command.PositionKey,
		Offer:               *chosen,
		Quantity:            command.Quantity,
		FitmentStatus:       fitmentStatus,
		FitmentDigest:       fitmentPin,
		ProfileDigest:       profilePin,
		PartRequestDigest:   requestPin,
		Now:                 command.Now,
	}
	material, err := casesDomain.AssembleClientSelection(fixtureComparisonPolicy(), draft)
	if err != nil {
		return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "record_client_selection", Reason: err.Error()}
	}
	f.selectionSnapshotSeq++
	snapshotNo := int64(0)
	for _, snapshot := range f.selectionSnapshots[command.CaseID] {
		if snapshot.SnapshotNo > snapshotNo {
			snapshotNo = snapshot.SnapshotNo
		}
	}
	f.selectionSnapshots[command.CaseID] = append(f.selectionSnapshots[command.CaseID], casesDomain.SelectionSnapshotViewOf(material, snapshotNo+1, command.Now))
	return casesAPI.WorkOutcome{
		Command:              "record_client_selection",
		SelectionSnapshotID:  id.MustParse("01994f64-6c00-7000-8000-" + fmt.Sprintf("%012d", 41000+f.selectionSnapshotSeq)),
		SelectionPositionKey: command.PositionKey,
		SelectionDigest:      material.SelectionDigest,
	}, nil
}

// RecordClientSelection records one client selection of the fixture
// world (the OffersService surface of the fixture cases service).
func (f *casesFixture) RecordClientSelection(_ context.Context, command casesAPI.RecordClientSelectionCommand) (casesAPI.WorkOutcome, error) {
	return f.fixtureClientSelectionRecord(command)
}

// fixtureOffersCriteria derives the CONFIRMED criteria material of the
// ranking from the fixture part facts (the same extraction as the real
// app: the confirmed budget, the brands, the deadline and the quantity —
// never the unconfirmed ones).
func (f *casesFixture) fixtureOffersCriteria(caseID id.UUID) casesDomain.ComparisonCriteria {
	criteria := casesDomain.ComparisonCriteria{Brands: []string{}}
	view := casesDomain.DerivePartRequest(fixturePartSchema(), f.fixturePartFacts(caseID), f.fixtureActivePartSearches(caseID))
	for _, attribute := range view.Attributes {
		if attribute.Status != casesDomain.FactStatusConfirmed {
			continue
		}
		switch attribute.MaterialKey {
		case "part.budget":
			criteria.BudgetConfirmed = true
		case "part.brands":
			criteria.BrandsConfirmed = true
			for _, word := range splitAndTrim(attribute.RawValue) {
				criteria.Brands = append(criteria.Brands, word)
			}
		case "part.deadline":
			criteria.TermConfirmed = true
		case "part.quantity":
			if quantity, err := atoi(attribute.RawValue); err == nil && quantity > 0 {
				criteria.RequestedQty = quantity
			}
		}
	}
	return criteria
}

// fixtureOffersPanelOf derives the Story 4.5 panel of the fixture case
// view over the REAL domain function (the state ladder, the ranking, the
// stale words, the voice block and the digests are the derivations, never
// fixture inventions). The packs without the comparison policy render the
// honest absence word (the 4.2/4.3/4.4 panel family degradation mirror).
func (f *casesFixture) fixtureOffersPanelOf(packVersion string, caseID id.UUID) casesAPI.ComparisonPanelView {
	if packVersion != fixtureOffersPackVersion {
		return casesAPI.ComparisonPanelView{Note: casesAPI.ComparisonSchemaNoteAbsent}
	}
	profilePin, requestPin := f.fixtureOffersPins(caseID)
	// The positions of the case records grouped by the position key with
	// the read-time fitment answers (the ordered grouping of the
	// derivation input).
	positions := []casesDomain.ComparisonPositionInput{}
	byPosition := map[string]*casesDomain.ComparisonPositionInput{}
	positionOrder := []string{}
	for _, offer := range f.supplierOffers[caseID] {
		if _, ok := byPosition[offer.PositionKey]; !ok {
			status, digest, linkType := f.fixtureOffersFitmentAnswer(caseID, offer.PositionKey)
			statusWord := casesDomain.FitmentStatusWords[status]
			byPosition[offer.PositionKey] = &casesDomain.ComparisonPositionInput{
				PositionKey: offer.PositionKey, LinkType: linkType,
				FitmentStatus: status, FitmentStatusWord: statusWord, FitmentDigest: digest,
				Offers: []casesDomain.SupplierOfferInput{},
			}
			positionOrder = append(positionOrder, offer.PositionKey)
		}
		byPosition[offer.PositionKey].Offers = append(byPosition[offer.PositionKey].Offers, offer)
	}
	for _, key := range positionOrder {
		positions = append(positions, *byPosition[key])
	}
	records := []casesDomain.OfferView{}
	for _, offer := range f.supplierOffers[caseID] {
		records = append(records, casesDomain.OfferViewOf(offer))
	}
	selections := append([]casesDomain.SelectionSnapshotView(nil), f.selectionSnapshots[caseID]...)
	return casesAPI.ComparisonPanelView{
		SchemaDefined: true,
		Comparison: casesDomain.DeriveComparison(casesDomain.ComparisonDerivationInput{
			Policy:            fixtureComparisonPolicy(),
			Positions:         positions,
			ProfileDigest:     profilePin,
			PartRequestDigest: requestPin,
			Criteria:          f.fixtureOffersCriteria(caseID),
			Now:               journeyClock.Now(),
		}),
		Records:    records,
		Selections: selections,
	}
}

// splitAndTrim splits the comma-separated words (the fixture-local helper
// of the confirmed brand list).
func splitAndTrim(raw string) []string {
	words := []string{}
	start := 0
	for index := 0; index <= len(raw); index++ {
		if index == len(raw) || raw[index] == ',' {
			word := trimSpace(raw[start:index])
			if word != "" {
				words = append(words, word)
			}
			start = index + 1
		}
	}
	return words
}

// trimSpace trims the spaces of both ends (the tiny local helper).
func trimSpace(value string) string {
	for len(value) > 0 && (value[0] == ' ' || value[0] == '\t' || value[0] == '\n' || value[0] == '\r') {
		value = value[1:]
	}
	for len(value) > 0 && (value[len(value)-1] == ' ' || value[len(value)-1] == '\t' || value[len(value)-1] == '\n' || value[len(value)-1] == '\r') {
		value = value[:len(value)-1]
	}
	return value
}

// atoi parses the decimal integer (the tiny local helper of the quantity
// material).
func atoi(value string) (int, error) {
	value = trimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("пустое число")
	}
	result := 0
	for index := 0; index < len(value); index++ {
		digit := int(value[index]) - '0'
		if digit < 0 || digit > 9 {
			return 0, fmt.Errorf("не число: %s", value)
		}
		result = result*10 + digit
	}
	return result, nil
}
