package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The Story 4.5 record commands: the manager's commercial conditions
// research result as the append-only supplier offer and the client
// selection as the append-only selection snapshot
// ---------------------------------------------------------------------------
//
// The write is the recorded MATERIAL, never a derivation (the same
// honesty core as the fitment evidence of 4.4): the producer is the
// operator-manager; NO automatic supplier request is performed (the
// parts_catalog_abcp route is not admitted — OQ-5/AG-4). The gate order
// follows the operator command precedent: the rights gate (review OR
// configuration) fires BEFORE every material guard; the pack policy
// resolves through the narrow taxonomy port (a pack without the policy
// refuses honestly BEFORE any record — the honest degradation, never a
// hardcoded policy); the domain guard refuses the dishonest material with
// the dictionary words BEFORE any SQL; the profile and the request digest
// pins derive from the LIVE derivations of 4.2/4.3 and the fitment pin
// from the LIVE 4.4 derivation of the position (the operator never inputs
// them); the selection command reads the CURRENT offer records and runs
// the whole selection guard BEFORE the write (the store re-elects the
// current offer and re-runs the guard under the locks — the server
// truth). The known client sum is the ONLY arithmetic of the slice (the
// quantity × the recorded client price — AC-4). NO ABCP Order is created
// and NOTHING is sent to the supplier (the fence OQ-3/OQ-4 — the order
// contour of 4.6).

// offerRecordScopeText is the honest scope refusal of the record commands
// (the shared «review или configuration» vocabulary of the operator
// rights gates): the app word travels inside the forbidden error only —
// the transport renders its own sentence for the wire and the form.
const offerRecordScopeText = "Команды записи предложений и выбора требуют полномочия review или configuration: коммерческие условия записывает менеджер по результату фактического запроса у подключённых поставщиков."

// offersPackPolicyAbsentText is the honest refusal of a pack without the
// comparison policy (the honest degradation of the panel family — the
// command never assumes a policy, never hardcodes one).
const offersPackPolicyAbsentText = "политика сравнения пака не определена: запись предложения невозможна — пак не несёт Comparison-политики"

// liveFitmentPinOf derives the LIVE fitment pin of ONE position from the
// read-time fitment derivation of 4.4 (the empty word when the position
// carries no proven proof — the honest offer without the proof, recorded
// but never recommended): the app re-derives the fitment states over the
// append-only evidence records and takes the digest of the position's
// proven answer.
func (s *Service) liveFitmentPinOf(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail, positionKey string, inputs []casesDomain.VehicleFactInput) (string, string, error) {
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return "", "", rule("record_supplier_offer", fmt.Errorf("%s", offersPackPolicyAbsentText))
	}
	fitmentPolicyPort, err := s.taxonomy.PackFitmentSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(fitmentPolicyPort.LinkTypes) == 0 {
		// The pack without the fitment policy: the honest empty pin (the
		// offer records honestly reference no proof — the 4.4 panel of a
		// 3.0.0-era pack carries the same honest absence).
		return "", casesDomain.FitmentStatusUnknown, nil
	}
	policy := casesDomain.FitmentPolicyDomain{
		LinkTypes:      fitmentPolicyPort.LinkTypes,
		MaxCrossDepth:  fitmentPolicyPort.MaxCrossDepth,
		AllowedSources: fitmentPolicyPort.AllowedSources,
		MaxAgeSeconds:  fitmentPolicyPort.MaxAgeSeconds,
	}
	records, err := s.repository.ReadFitmentEvidencesOfCase(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return "", "", err
	}
	positions := []casesDomain.FitmentPositionInput{}
	byPosition := map[string]*casesDomain.FitmentPositionInput{}
	for _, record := range records {
		position, ok := byPosition[record.PositionKey]
		if !ok {
			position = &casesDomain.FitmentPositionInput{PositionKey: record.PositionKey, Evidences: []casesDomain.FitmentEvidenceInput{}}
			byPosition[record.PositionKey] = position
		}
		position.Evidences = append(position.Evidences, casesDomain.FitmentEvidenceInput{
			EvidenceNo:           record.EvidenceNo,
			PositionKey:          record.PositionKey,
			CatalogNode:          record.CatalogNode,
			Article:              record.Article,
			LinkType:             record.LinkType,
			Chain:                record.Chain,
			CatalogVehicleIDs:    record.CatalogVehicleIDs,
			SourceSystem:         record.SourceSystem,
			SourceRecordID:       record.SourceRecordID,
			SourceVersion:        record.SourceVersion,
			SourceObservedAt:     record.SourceObservedAt,
			ReceivedAt:           record.ReceivedAt,
			ProfileDigestPin:     record.ProfileDigestPin,
			PartRequestDigestPin: record.PartRequestDigestPin,
		})
	}
	positionKeys := make([]string, 0, len(byPosition))
	for key := range byPosition {
		positionKeys = append(positionKeys, key)
	}
	sort.Strings(positionKeys)
	for _, key := range positionKeys {
		positions = append(positions, *byPosition[key])
	}
	profileDigest, err := s.liveProfileDigest(ctx, scope, detail, inputs)
	if err != nil {
		return "", "", err
	}
	partRequestDigest, err := s.livePartRequestDigest(ctx, scope, detail, inputs)
	if err != nil {
		return "", "", err
	}
	view := casesDomain.DeriveFitment(casesDomain.FitmentDerivationInput{
		Policy:            policy,
		Positions:         positions,
		ProfileDigest:     profileDigest,
		PartRequestDigest: partRequestDigest,
		Now:               s.clock.Now(),
	})
	for _, position := range view.Positions {
		if position.PositionKey == positionKey {
			return position.FitmentDigest, position.Status, nil
		}
	}
	return "", casesDomain.FitmentStatusUnknown, nil
}

// RecordSupplierOffer records ONE manager's commercial conditions research
// result as the append-only supplier offer row: the rights gate, the pack
// policy through the taxonomy port, the LIVE digest pins (the vehicle
// profile of 4.2, the part request of 4.3 and the fitment proof of 4.4 —
// derived BEFORE the write; the operator NEVER inputs them), the domain
// guard of the material, the replay lookup with the whole-material hash,
// the work item aliveness check and the single transactional write under
// the case + work item locks (the store re-validates the guard, derives
// the monotonic number and the canonical OfferDigest under the locks).
func (s *Service) RecordSupplierOffer(ctx context.Context, command casesAPI.RecordSupplierOfferCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, offerRecordScopeText)
	}
	// The case anchor with the pack pin: the policy resolves through the
	// narrow taxonomy port of the PINNED pack (never a hardcoded policy).
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return casesAPI.WorkOutcome{}, rule("record_supplier_offer", fmt.Errorf("%s", offersPackPolicyAbsentText))
	}
	policyPort, err := s.taxonomy.PackComparisonSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(policyPort.Currencies) == 0 {
		return casesAPI.WorkOutcome{}, rule("record_supplier_offer", fmt.Errorf("%s", offersPackPolicyAbsentText))
	}
	policy := casesDomain.ComparisonPolicyDomain{
		Currencies:         policyPort.Currencies,
		MaxOfferAgeSeconds: policyPort.MaxOfferAgeSeconds,
		RankCriteria:       policyPort.RankCriteria,
		RankCriteriaLabels: policyPort.RankCriteriaLabels,
		TieBreakers:        policyPort.TieBreakers,
		MaxVisibleVariants: policyPort.MaxVisibleVariants,
	}
	// The LIVE digest pins of the inputs — derived BEFORE the guard and the
	// write (AC-2/AC-10): the operator NEVER inputs them; the record
	// carries the exact input versions of the write moment. The profile
	// digest of 4.2 and the part request digest of 4.3 derive over the
	// append-only facts of the case; the fitment pin of 4.4 derives from
	// the live fitment derivation of the position (empty when the position
	// carries no proven proof — the honest offer without the proof).
	facts, err := s.repository.ReadFactsOfCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	inputs := make([]casesDomain.VehicleFactInput, 0, len(facts))
	for _, fact := range facts {
		inputs = append(inputs, casesDomain.VehicleFactInput{
			FactID: fact.Anchor.FactID, MaterialKey: fact.Anchor.MaterialKey,
			Versions: fact.Versions, Confirmations: fact.Confirmations,
		})
	}
	profileDigest, err := s.liveProfileDigest(ctx, scope, detail, inputs)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	partRequestDigest, err := s.livePartRequestDigest(ctx, scope, detail, inputs)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	fitmentPin, _, err := s.liveFitmentPinOf(ctx, scope, detail, command.PositionKey, inputs)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The domain guard of the material BEFORE any SQL (the honest 400 with
	// the dictionary words; the store re-validates under the locks, the
	// SQL CHECK mirrors are the backstops).
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
		ProfileDigestPin:     profileDigest,
		PartRequestDigestPin: partRequestDigest,
		FitmentDigestPin:     fitmentPin,
	}
	if err := rule("record_supplier_offer", casesDomain.GuardSupplierOffer(policy, draft)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE command material INCLUDING the
	// derived pins (the replay of the exact repeat returns the stored
	// outcome; the same key with another material is the honest key
	// conflict). The canonical OfferDigest is NOT the hash material: it
	// pins the server-side monotonic number (the store derives it under
	// the locks — the read surface re-derives the same bytes).
	hash := requestHash(struct {
		Case         string  `json:"case"`
		WorkItem     string  `json:"work_item"`
		PositionKey  string  `json:"position_key"`
		SupplierID   string  `json:"supplier_id"`
		Brand        string  `json:"brand"`
		Article      string  `json:"article"`
		ClientPrice  float64 `json:"client_price"`
		Currency     string  `json:"currency"`
		Quantity     int     `json:"quantity_available"`
		TermDays     *int    `json:"term_days"`
		SourceSystem string  `json:"source_system"`
		RecordID     string  `json:"source_record_id"`
		Version      string  `json:"source_version"`
		ProfilePin   string  `json:"profile_digest_pin"`
		RequestPin   string  `json:"part_request_digest_pin"`
		FitmentPin   string  `json:"fitment_digest_pin"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.PositionKey, command.SupplierID,
		command.Brand, command.Article, command.ClientPrice, command.Currency,
		command.QuantityAvailable, command.TermDays, command.SourceSystem, command.SourceRecordID,
		command.SourceVersion, profileDigest, partRequestDigest, fitmentPin})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_supplier_offer", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The work item of the record scope: it must exist in this case and be
	// alive — the terminal states refuse honestly.
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, rule("record_supplier_offer", fmt.Errorf("%s", casesDomain.TerminalWorkItemReadText))
	}
	// The identifiers and the correlation of the command trace.
	offerID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	now := command.Now.UTC().Truncate(time.Microsecond)
	outcome := casesAPI.WorkOutcome{
		Command:          "record_supplier_offer",
		SupplierOfferID:  offerID,
		OfferPositionKey: command.PositionKey,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_supplier_offer", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ReceivedAt: now, CorrelationID: correlation, Outcome: outcomeJSON,
		Offer: &casesPort.SupplierOfferCommandMaterial{
			OfferID:              offerID,
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
			SourceObservedAt:     command.SourceObservedAt.UTC(),
			ProfileDigestPin:     profileDigest,
			PartRequestDigestPin: partRequestDigest,
			FitmentDigestPin:     fitmentPin,
		},
		ComparisonPolicy: casesPort.ComparisonPolicySpec{
			Currencies:         policyPort.Currencies,
			MaxOfferAgeSeconds: policyPort.MaxOfferAgeSeconds,
			RankCriteria:       policyPort.RankCriteria,
			RankCriteriaLabels: policyPort.RankCriteriaLabels,
			TieBreakers:        policyPort.TieBreakers,
			MaxVisibleVariants: policyPort.MaxVisibleVariants,
		},
		Now: now,
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// RecordClientSelection records ONE client selection as the append-only
// selection snapshot row: the rights gate, the pack policy, the LIVE
// derivations of ALL FOUR inputs (the CURRENT offer record read + the
// live 4.4 fitment answer of the position + the CURRENT profile and
// request digests), the whole selection guard BEFORE any SQL (the store
// re-elects the current offer and re-runs the guard under the locks), the
// replay lookup with the whole-material hash and the single transactional
// write. The known client sum is the ONLY arithmetic of the slice (the
// quantity × the recorded client price — AC-4). NO ABCP Order and NO
// supplier send (the fence OQ-3/OQ-4).
func (s *Service) RecordClientSelection(ctx context.Context, command casesAPI.RecordClientSelectionCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, offerRecordScopeText)
	}
	// The case anchor with the pack pin.
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return casesAPI.WorkOutcome{}, rule("record_client_selection", fmt.Errorf("%s", offersPackPolicyAbsentText))
	}
	policyPort, err := s.taxonomy.PackComparisonSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(policyPort.Currencies) == 0 {
		return casesAPI.WorkOutcome{}, rule("record_client_selection", fmt.Errorf("%s", offersPackPolicyAbsentText))
	}
	policy := casesDomain.ComparisonPolicyDomain{
		Currencies:         policyPort.Currencies,
		MaxOfferAgeSeconds: policyPort.MaxOfferAgeSeconds,
		RankCriteria:       policyPort.RankCriteria,
		RankCriteriaLabels: policyPort.RankCriteriaLabels,
		TieBreakers:        policyPort.TieBreakers,
		MaxVisibleVariants: policyPort.MaxVisibleVariants,
	}
	// The LIVE derivations of ALL FOUR inputs BEFORE any SQL: the CURRENT
	// offer record of the (work item, position, supplier) ladder read from
	// the store, the live 4.4 fitment answer of the position, the CURRENT
	// profile and request digests over the append-only facts.
	facts, err := s.repository.ReadFactsOfCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	inputs := make([]casesDomain.VehicleFactInput, 0, len(facts))
	for _, fact := range facts {
		inputs = append(inputs, casesDomain.VehicleFactInput{
			FactID: fact.Anchor.FactID, MaterialKey: fact.Anchor.MaterialKey,
			Versions: fact.Versions, Confirmations: fact.Confirmations,
		})
	}
	profileDigest, err := s.liveProfileDigest(ctx, scope, detail, inputs)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	partRequestDigest, err := s.livePartRequestDigest(ctx, scope, detail, inputs)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	fitmentPin, fitmentStatus, err := s.liveFitmentPinOf(ctx, scope, detail, command.PositionKey, inputs)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	offers, err := s.repository.ReadSupplierOffersOfCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	var chosen *casesPort.SupplierOfferRecord
	for index := range offers {
		offer := offers[index]
		if offer.WorkItemID != command.WorkItemID || offer.PositionKey != command.PositionKey || offer.SupplierID != command.SupplierID {
			continue
		}
		if chosen == nil || offer.OfferNo > chosen.OfferNo {
			chosen = &offers[index]
		}
	}
	if chosen == nil {
		return casesAPI.WorkOutcome{}, rule("record_client_selection", fmt.Errorf("%s", casesDomain.OfferSelectionGuardStaleWord))
	}
	// The whole selection guard BEFORE any SQL (the honest 400 with the
	// dictionary words; the store re-runs it under the locks over the
	// server truth).
	selectionDraft := casesDomain.ClientSelectionDraft{
		PositionKey:       command.PositionKey,
		Offer:             supplierOfferDomainInputOf(*chosen),
		Quantity:          command.Quantity,
		FitmentStatus:     fitmentStatus,
		FitmentDigest:     fitmentPin,
		ProfileDigest:     profileDigest,
		PartRequestDigest: partRequestDigest,
		Now:               command.Now,
	}
	material, err := casesDomain.AssembleClientSelection(policy, selectionDraft)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("record_client_selection", err)
	}
	// The request hash covers the WHOLE command material INCLUDING the
	// derived pins and the canonical digest (the replay of the exact
	// repeat returns the stored outcome; the same key with another
	// material is the honest key conflict).
	hash := requestHash(struct {
		Case        string  `json:"case"`
		WorkItem    string  `json:"work_item"`
		PositionKey string  `json:"position_key"`
		SupplierID  string  `json:"supplier_id"`
		OfferNo     int64   `json:"offer_no"`
		Quantity    int     `json:"quantity"`
		Sum         float64 `json:"known_client_sum"`
		ProfilePin  string  `json:"profile_digest_pin"`
		RequestPin  string  `json:"part_request_digest_pin"`
		FitmentPin  string  `json:"fitment_digest_pin"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.PositionKey, command.SupplierID,
		command.OfferNo, command.Quantity, material.KnownClientSum,
		material.ProfileDigestPin, material.PartRequestDigestPin, material.FitmentDigestPin})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_client_selection", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The work item of the record scope: it must exist in this case and be
	// alive — the terminal states refuse honestly.
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, rule("record_client_selection", fmt.Errorf("%s", casesDomain.TerminalWorkItemReadText))
	}
	// The identifiers and the correlation of the command trace.
	snapshotID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	now := command.Now.UTC().Truncate(time.Microsecond)
	outcome := casesAPI.WorkOutcome{
		Command:              "record_client_selection",
		SelectionSnapshotID:  snapshotID,
		SelectionPositionKey: command.PositionKey,
		SelectionDigest:      material.SelectionDigest,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_client_selection", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ReceivedAt: now, CorrelationID: correlation, Outcome: outcomeJSON,
		Selection: &casesPort.SelectionSnapshotCommandMaterial{
			SnapshotID:           snapshotID,
			PositionKey:          command.PositionKey,
			SupplierID:           command.SupplierID,
			OfferNo:              command.OfferNo,
			OfferDigestPin:       material.OfferDigestPin,
			Quantity:             command.Quantity,
			UnitClientPrice:      material.UnitClientPrice,
			Currency:             material.Currency,
			KnownClientSum:       material.KnownClientSum,
			ProfileDigestPin:     material.ProfileDigestPin,
			PartRequestDigestPin: material.PartRequestDigestPin,
			FitmentDigestPin:     material.FitmentDigestPin,
			SelectionDigest:      material.SelectionDigest,
		},
		ComparisonPolicy: casesPort.ComparisonPolicySpec{
			Currencies:         policyPort.Currencies,
			MaxOfferAgeSeconds: policyPort.MaxOfferAgeSeconds,
			RankCriteria:       policyPort.RankCriteria,
			RankCriteriaLabels: policyPort.RankCriteriaLabels,
			TieBreakers:        policyPort.TieBreakers,
			MaxVisibleVariants: policyPort.MaxVisibleVariants,
		},
		Now: now,
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// supplierOfferDomainInputOf maps ONE stored offer record onto the domain
// selection input (the same fields, the same honest shapes — the app-side
// mirror of the store helper).
func supplierOfferDomainInputOf(record casesPort.SupplierOfferRecord) casesDomain.SupplierOfferInput {
	return casesDomain.SupplierOfferInput{
		OfferNo:              record.OfferNo,
		PositionKey:          record.PositionKey,
		SupplierID:           record.SupplierID,
		SupplierName:         record.SupplierName,
		Brand:                record.Brand,
		Article:              record.Article,
		ClientPrice:          record.ClientPrice,
		PurchasePrice:        record.PurchasePrice,
		Currency:             record.Currency,
		QuantityAvailable:    record.QuantityAvailable,
		TermDays:             record.TermDays,
		Conditions:           record.Conditions,
		SourceSystem:         record.SourceSystem,
		SourceRecordID:       record.SourceRecordID,
		SourceVersion:        record.SourceVersion,
		SourceObservedAt:     record.SourceObservedAt,
		ReceivedAt:           record.ReceivedAt,
		ProfileDigestPin:     record.ProfileDigestPin,
		PartRequestDigestPin: record.PartRequestDigestPin,
		FitmentDigestPin:     record.FitmentDigestPin,
	}
}

// offersView assembles the Story 4.5 comparison panel of the case view:
// the pack comparison policy through the narrow taxonomy port (the panel
// degrades with the honest word when the pin is unreadable or the pack
// carries no policy — the 4.2/4.3/4.4 panel family precedent, the read
// NEVER fails), the append-only offer records of the case grouped by the
// position with the read-time fitment answers of 4.4 (the comparison
// READS the fitment output — the availability NEVER changes the fitment
// status, AC-1), the CONFIRMED criteria of the part request material
// (AC-6 — never the unconfirmed ones), the read-time DeriveComparison
// derivation, the operator record material (the internal section with
// the purchase price and its internal-data word) and the recorded
// selection snapshots.
func (s *Service) offersView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.ComparisonPanelView, error) {
	panel := casesAPI.ComparisonPanelView{Note: casesAPI.ComparisonSchemaNoteAbsent}
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return panel, nil
	}
	policyPort, err := s.taxonomy.PackComparisonSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(policyPort.Currencies) == 0 {
		return panel, nil
	}
	policy := casesDomain.ComparisonPolicyDomain{
		Currencies:         policyPort.Currencies,
		MaxOfferAgeSeconds: policyPort.MaxOfferAgeSeconds,
		RankCriteria:       policyPort.RankCriteria,
		RankCriteriaLabels: policyPort.RankCriteriaLabels,
		TieBreakers:        policyPort.TieBreakers,
		MaxVisibleVariants: policyPort.MaxVisibleVariants,
	}
	// The facts of the case: the CURRENT digest pins of the inputs, the
	// confirmed criteria material and the read-time fitment answers.
	facts, err := s.repository.ReadFactsOfCase(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ComparisonPanelView{}, err
	}
	inputs := make([]casesDomain.VehicleFactInput, 0, len(facts))
	for _, fact := range facts {
		inputs = append(inputs, casesDomain.VehicleFactInput{
			FactID: fact.Anchor.FactID, MaterialKey: fact.Anchor.MaterialKey,
			Versions: fact.Versions, Confirmations: fact.Confirmations,
		})
	}
	profileDigest, err := s.liveProfileDigest(ctx, scope, detail, inputs)
	if err != nil {
		return casesAPI.ComparisonPanelView{}, err
	}
	partRequestDigest, partSchema, partInputs, err := s.livePartRequestDigestWithSchema(ctx, scope, detail, inputs)
	if err != nil {
		return casesAPI.ComparisonPanelView{}, err
	}
	factInputs := make([]casesDomain.PartRequestFactInput, 0, len(partInputs))
	factInputs = append(factInputs, partInputs...)
	criteria := comparisonCriteriaOf(casesDomain.DerivePartRequest(partSchema, factInputs, 0))
	// The read-time fitment answers of the positions (the comparison READS
	// the output of 4.4 — never re-derives its own fitment states; a pack
	// without the fitment policy renders the honest unknown status words).
	fitmentPanel, err := s.fitmentView(ctx, scope, detail)
	if err != nil {
		return casesAPI.ComparisonPanelView{}, err
	}
	fitmentByPosition := map[string]casesDomain.FitmentPositionView{}
	if fitmentPanel.SchemaDefined {
		for _, position := range fitmentPanel.Fitment.Positions {
			fitmentByPosition[position.PositionKey] = position
		}
	}
	// The append-only offer records grouped by the position (the store
	// reads them ordered; the derivation elects the current record of
	// every supplier by the number).
	records, err := s.repository.ReadSupplierOffersOfCase(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ComparisonPanelView{}, err
	}
	positions := []casesDomain.ComparisonPositionInput{}
	byPosition := map[string]*casesDomain.ComparisonPositionInput{}
	offerViews := []casesDomain.OfferView{}
	for _, record := range records {
		position, ok := byPosition[record.PositionKey]
		if !ok {
			position = &casesDomain.ComparisonPositionInput{PositionKey: record.PositionKey, Offers: []casesDomain.SupplierOfferInput{}}
			byPosition[record.PositionKey] = position
		}
		offer := supplierOfferDomainInputOf(record)
		position.Offers = append(position.Offers, offer)
		offerViews = append(offerViews, casesDomain.OfferViewOf(offer))
	}
	for _, fitment := range fitmentByPosition {
		if _, ok := byPosition[fitment.PositionKey]; ok {
			continue
		}
		position := &casesDomain.ComparisonPositionInput{PositionKey: fitment.PositionKey, Offers: []casesDomain.SupplierOfferInput{}}
		byPosition[fitment.PositionKey] = position
	}
	positionKeys := make([]string, 0, len(byPosition))
	for key := range byPosition {
		positionKeys = append(positionKeys, key)
	}
	sort.Strings(positionKeys)
	for _, key := range positionKeys {
		position := byPosition[key]
		if fitment, ok := fitmentByPosition[key]; ok {
			position.LinkType = positionLinkTypeOf(fitment)
			position.FitmentStatus = fitment.Status
			position.FitmentStatusWord = fitment.StatusWord
			position.FitmentDigest = fitment.FitmentDigest
		} else {
			position.FitmentStatus = casesDomain.FitmentStatusUnknown
			position.FitmentStatusWord = casesDomain.FitmentUnknownWord
		}
		positions = append(positions, *position)
	}
	// The recorded selection snapshots (the material of the order contour
	// of 4.6 — NO ABCP Order is ever created by this surface).
	snapshots, err := s.repository.ReadSelectionSnapshotsOfCase(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ComparisonPanelView{}, err
	}
	selectionViews := make([]casesDomain.SelectionSnapshotView, 0, len(snapshots))
	for _, snapshot := range snapshots {
		material := casesDomain.SelectionSnapshotMaterial{
			PositionKey:          snapshot.PositionKey,
			SupplierID:           snapshot.SupplierID,
			OfferNo:              snapshot.OfferNo,
			Quantity:             snapshot.Quantity,
			UnitClientPrice:      snapshot.UnitClientPrice,
			Currency:             snapshot.Currency,
			KnownClientSum:       snapshot.KnownClientSum,
			ProfileDigestPin:     snapshot.ProfileDigestPin,
			PartRequestDigestPin: snapshot.PartRequestDigestPin,
			FitmentDigestPin:     snapshot.FitmentDigestPin,
			OfferDigestPin:       snapshot.OfferDigestPin,
			// The canonical SelectionDigest of the stored snapshot (the pin
			// of the exact version of the choice — the order contour of 4.6
			// carries it into the OrderDigest; the view must never drop it).
			SelectionDigest: snapshot.SelectionDigest,
		}
		selectionViews = append(selectionViews, casesDomain.SelectionSnapshotViewOf(material, snapshot.SnapshotNo, snapshot.CreatedAt))
	}
	panel.SchemaDefined = true
	panel.Note = ""
	panel.Records = offerViews
	panel.Selections = selectionViews
	panel.Comparison = casesDomain.DeriveComparison(casesDomain.ComparisonDerivationInput{
		Policy:            policy,
		Positions:         positions,
		ProfileDigest:     profileDigest,
		PartRequestDigest: partRequestDigest,
		Criteria:          criteria,
		Now:               s.clock.Now(),
	})
	return panel, nil
}

// comparisonCriteriaOf extracts the CONFIRMED criteria material of the
// ranking from the read-time part request view (AC-6: the primary
// criterion comes from the confirmed facts of the part request — never
// from the unconfirmed ones). The material keys belong to the pinned pack
// schema (the auto-parts contour declares the budget, the brands, the
// deadline and the quantity keys); a pack without them honestly yields
// the no-criteria question of the comparison (never a hardcoded core
// list — the keys are looked up in the pack-declared schema, the
// unconfirmed attributes are never ranked).
func comparisonCriteriaOf(view casesDomain.PartRequestView) casesDomain.ComparisonCriteria {
	criteria := casesDomain.ComparisonCriteria{Brands: []string{}}
	for _, attribute := range view.Attributes {
		if attribute.Status != casesDomain.FactStatusConfirmed {
			continue
		}
		switch attribute.MaterialKey {
		case "part.budget":
			criteria.BudgetConfirmed = true
		case "part.brands":
			criteria.BrandsConfirmed = true
			criteria.Brands = parseBrandList(attribute.RawValue)
		case "part.deadline":
			criteria.TermConfirmed = true
		case "part.quantity":
			if quantity, err := strconv.Atoi(strings.TrimSpace(attribute.RawValue)); err == nil && quantity > 0 {
				criteria.RequestedQty = quantity
			}
		}
	}
	return criteria
}

// parseBrandList splits the confirmed brand material into the normalized
// list (the comma-separated words, trimmed; the empty words never appear).
func parseBrandList(raw string) []string {
	brands := []string{}
	for _, word := range strings.Split(raw, ",") {
		word = strings.TrimSpace(word)
		if word != "" {
			brands = append(brands, word)
		}
	}
	return brands
}

// positionLinkTypeOf resolves the link type word of ONE fitment position
// (the grouping material of AC-5): the link type of the first FOUND
// evidence of the position (the proven record's link word; the not-found
// records honestly carry the source material only).
func positionLinkTypeOf(position casesDomain.FitmentPositionView) string {
	for _, evidence := range position.Evidences {
		if len(evidence.CatalogVehicleIDs) > 0 {
			return evidence.LinkType
		}
	}
	if len(position.Evidences) > 0 {
		return position.Evidences[0].LinkType
	}
	return ""
}
