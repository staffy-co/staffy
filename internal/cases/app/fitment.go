package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
// The Story 4.4 record command: the manager's catalog research result as
// the append-only fitment evidence
// ---------------------------------------------------------------------------
//
// The write is the recorded MATERIAL, never a derivation (the difference
// from the part request of 4.3): the producer is the operator-manager; NO
// automatic catalog request is performed (the parts_catalog_abcp route is
// not admitted — OQ-5/AG-4). The gate order follows the operator command
// precedent: the rights gate (review OR configuration) fires BEFORE every
// material guard; the pack policy resolves through the narrow taxonomy
// port (a pack without the policy refuses honestly BEFORE any record —
// the honest degradation, never a hardcoded policy); the domain guard
// refuses the dishonest material with the dictionary words BEFORE any
// SQL; BOTH digest pins derive from the LIVE derivations of the vehicle
// profile (4-2) and the part request (4-3) — the operator never inputs
// them, the record carries the exact input versions of the write moment
// (AC-1/AC-11); the canonical FitmentDigest derives from the same
// material (the byte-stable «exact version of the proof»); the store
// re-validates the guard and the digest under the locks and derives the
// monotonic evidence number per (work item, position, source).

// fitmentRecordScopeText is the honest scope refusal of the record
// command (the shared «review или configuration» vocabulary of the
// operator rights gates): the app word travels inside the forbidden error
// only — the transport renders its own sentence for the wire and the
// form.
const fitmentRecordScopeText = "Команды записи доказательства применимости требуют полномочия review или configuration: доказательство записывает менеджер по результату фактического исследования каталога."

// fitmentPackPolicyAbsentText is the honest refusal of a pack without the
// fitment policy (the honest degradation of the panel family — the
// command never assumes a policy, never hardcodes one).
const fitmentPackPolicyAbsentText = "политика применимости пака не определена: запись доказательства невозможна — пак не несёт Fitment-политики"

// RecordFitmentEvidence records ONE manager's catalog research result as
// the append-only fitment evidence row: the rights gate, the pack policy
// through the taxonomy port, the domain guard of the material, the LIVE
// digest pins of BOTH inputs (the vehicle profile of 4.2 and the part
// request of 4.3 — derived BEFORE the write), the replay lookup with the
// whole-material hash, the work item aliveness check and the single
// transactional write under the case + work item locks (the store
// re-validates the guard, the digest and the monotonic number).
func (s *Service) RecordFitmentEvidence(ctx context.Context, command casesAPI.RecordFitmentEvidenceCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, fitmentRecordScopeText)
	}
	// The case anchor with the pack pin: the policy resolves through the
	// narrow taxonomy port of the PINNED pack (never a hardcoded policy).
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return casesAPI.WorkOutcome{}, rule("record_fitment_evidence", fmt.Errorf("%s", fitmentPackPolicyAbsentText))
	}
	policyPort, err := s.taxonomy.PackFitmentSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(policyPort.LinkTypes) == 0 {
		return casesAPI.WorkOutcome{}, rule("record_fitment_evidence", fmt.Errorf("%s", fitmentPackPolicyAbsentText))
	}
	policy := casesDomain.FitmentPolicyDomain{
		LinkTypes:      policyPort.LinkTypes,
		MaxCrossDepth:  policyPort.MaxCrossDepth,
		AllowedSources: policyPort.AllowedSources,
		MaxAgeSeconds:  policyPort.MaxAgeSeconds,
	}
	// The LIVE digest pins of BOTH inputs — derived BEFORE the guard and
	// the write (AC-1/AC-11): the operator NEVER inputs them; the record
	// carries the exact input versions of the write moment. The vehicle
	// profile digest of 4.2 and the part request digest of 4.3 derive over
	// the append-only facts of the case (the same read-time derivations of
	// the panels).
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
	// The domain guard of the material BEFORE any SQL (the honest 400 with
	// the dictionary words; the store re-validates under the locks, the
	// SQL CHECK mirrors are the backstops).
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
		ProfileDigestPin:     profileDigest,
		PartRequestDigestPin: partRequestDigest,
	}
	if err := rule("record_fitment_evidence", casesDomain.GuardFitmentEvidence(policy, draft)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The canonical FitmentDigest of the record (the byte-stable «exact
	// version of the proof» — the store re-derives the same bytes under
	// the locks).
	evidenceMaterial := casesDomain.FitmentEvidenceInput{
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
		ProfileDigestPin:     profileDigest,
		PartRequestDigestPin: partRequestDigest,
	}
	fitmentDigest := casesDomain.FitmentDigestOf(evidenceMaterial)
	// The request hash covers the WHOLE command material INCLUDING the
	// derived pins and the canonical digest (the replay of the exact
	// repeat returns the stored outcome; the same key with another
	// material is the honest key conflict).
	hash := requestHash(struct {
		Case         string   `json:"case"`
		WorkItem     string   `json:"work_item"`
		PositionKey  string   `json:"position_key"`
		LinkType     string   `json:"link_type"`
		Chain        []string `json:"chain"`
		VehicleIDs   []string `json:"catalog_vehicle_ids"`
		SourceSystem string   `json:"source_system"`
		RecordID     string   `json:"source_record_id"`
		Version      string   `json:"source_version"`
		ProfilePin   string   `json:"profile_digest_pin"`
		RequestPin   string   `json:"part_request_digest_pin"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.PositionKey, command.LinkType,
		command.Chain, command.CatalogVehicleIDs, command.SourceSystem, command.SourceRecordID,
		command.SourceVersion, profileDigest, partRequestDigest})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_fitment_evidence", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The work item of the record scope: it must exist in this case and be
	// alive — the terminal states refuse honestly (the merged item lives
	// on in its target).
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, rule("record_fitment_evidence", fmt.Errorf("%s", casesDomain.TerminalWorkItemReadText))
	}
	// The identifiers and the correlation of the command trace.
	evidenceID, err := s.ids.New(command.Now)
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
		Command:            "record_fitment_evidence",
		FitmentEvidenceID:  evidenceID,
		FitmentPositionKey: command.PositionKey,
		FitmentDigest:      fitmentDigest,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_fitment_evidence", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ReceivedAt: now, CorrelationID: correlation, Outcome: outcomeJSON,
		Fitment: &casesPort.FitmentEvidenceCommandMaterial{
			EvidenceID:           evidenceID,
			PositionKey:          command.PositionKey,
			CatalogNode:          command.CatalogNode,
			Article:              command.Article,
			LinkType:             command.LinkType,
			Chain:                command.Chain,
			CatalogVehicleIDs:    command.CatalogVehicleIDs,
			SourceSystem:         command.SourceSystem,
			SourceRecordID:       command.SourceRecordID,
			SourceVersion:        command.SourceVersion,
			SourceObservedAt:     command.SourceObservedAt.UTC(),
			ProfileDigestPin:     profileDigest,
			PartRequestDigestPin: partRequestDigest,
			FitmentDigest:        fitmentDigest,
		},
		FitmentPolicy: casesPort.FitmentPolicySpec{
			LinkTypes:      policyPort.LinkTypes,
			MaxCrossDepth:  policyPort.MaxCrossDepth,
			AllowedSources: policyPort.AllowedSources,
			MaxAgeSeconds:  policyPort.MaxAgeSeconds,
		},
		Now: now,
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// liveProfileDigest derives the CURRENT ProfileDigest of the case (the
// live derivation of 4.2 over the append-only facts — the pin of the
// fitment evidence record; the digest of the empty confirmed set is the
// honest sha256 of the canonical empty object, never an empty string).
func (s *Service) liveProfileDigest(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail, inputs []casesDomain.VehicleFactInput) (string, error) {
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return "", rule("record_fitment_evidence", fmt.Errorf("%s", fitmentPackPolicyAbsentText))
	}
	schemaPort, err := s.taxonomy.PackIdentificationSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(schemaPort.MaterialKeys) == 0 {
		return "", rule("record_fitment_evidence", fmt.Errorf("схема идентификации пака не определена: пин профиля невозможен"))
	}
	schema := casesDomain.VehicleSchema{
		MaterialKeys:    make([]casesDomain.VehicleKeySpec, 0, len(schemaPort.MaterialKeys)),
		AlternativeSets: schemaPort.AlternativeSets,
	}
	for _, key := range schemaPort.MaterialKeys {
		schema.MaterialKeys = append(schema.MaterialKeys, casesDomain.VehicleKeySpec{
			Key: key.Key, Label: key.Label, Kind: key.Kind, Min: key.Min, Max: key.Max, Optional: key.Optional,
		})
	}
	return casesDomain.DeriveVehicleProfile(schema, inputs).ProfileDigest, nil
}

// livePartRequestDigestWithSchema derives the CURRENT PartRequestDigest
// of the case AND hands the assembled schema with the inputs back (the
// panel view reuses the schema for the safety-node flag — one read, one
// derivation family).
func (s *Service) livePartRequestDigestWithSchema(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail, inputs []casesDomain.VehicleFactInput) (string, casesDomain.PartSchemaDomain, []casesDomain.VehicleFactInput, error) {
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return "", casesDomain.PartSchemaDomain{}, nil, rule("record_fitment_evidence", fmt.Errorf("%s", fitmentPackPolicyAbsentText))
	}
	schemaPort, err := s.taxonomy.PackPartRequestSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(schemaPort.MaterialKeys) == 0 {
		return "", casesDomain.PartSchemaDomain{}, nil, rule("record_fitment_evidence", fmt.Errorf("схема запроса детали пака не определена: пин запроса невозможен"))
	}
	schema := casesDomain.PartSchemaDomain{
		MaterialKeys:  make([]casesDomain.PartMaterialKeyDomain, 0, len(schemaPort.MaterialKeys)),
		Modes:         make([]casesDomain.PartModeSpecDomain, 0, len(schemaPort.Modes)),
		SafetyMarkers: schemaPort.SafetyMarkers,
	}
	for _, key := range schemaPort.MaterialKeys {
		schema.MaterialKeys = append(schema.MaterialKeys, casesDomain.PartMaterialKeyDomain{
			Key: key.Key, Label: key.Label, Kind: key.Kind, Values: key.Values,
			Min: key.Min, Max: key.Max, Optional: key.Optional, Role: key.Role,
		})
	}
	for _, mode := range schemaPort.Modes {
		schema.Modes = append(schema.Modes, casesDomain.PartModeSpecDomain{
			Mode: mode.Mode, Label: mode.Label, Priority: mode.Priority, Requires: mode.Requires,
		})
	}
	factInputs := make([]casesDomain.PartRequestFactInput, 0, len(inputs))
	factInputs = append(factInputs, inputs...)
	return casesDomain.DerivePartRequest(schema, factInputs, 0).PartRequestDigest, schema, inputs, nil
}

// livePartRequestDigest derives the CURRENT PartRequestDigest of the case
// (the live derivation of 4.3 — the pin of the fitment evidence record;
// the digest of the empty confirmed set is the honest sha256 of the
// canonical empty object, never an empty string).
func (s *Service) livePartRequestDigest(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail, inputs []casesDomain.VehicleFactInput) (string, error) {
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return "", rule("record_fitment_evidence", fmt.Errorf("%s", fitmentPackPolicyAbsentText))
	}
	schemaPort, err := s.taxonomy.PackPartRequestSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(schemaPort.MaterialKeys) == 0 {
		return "", rule("record_fitment_evidence", fmt.Errorf("схема запроса детали пака не определена: пин запроса невозможен"))
	}
	schema := casesDomain.PartSchemaDomain{
		MaterialKeys:  make([]casesDomain.PartMaterialKeyDomain, 0, len(schemaPort.MaterialKeys)),
		Modes:         make([]casesDomain.PartModeSpecDomain, 0, len(schemaPort.Modes)),
		SafetyMarkers: schemaPort.SafetyMarkers,
	}
	for _, key := range schemaPort.MaterialKeys {
		schema.MaterialKeys = append(schema.MaterialKeys, casesDomain.PartMaterialKeyDomain{
			Key: key.Key, Label: key.Label, Kind: key.Kind, Values: key.Values,
			Min: key.Min, Max: key.Max, Optional: key.Optional, Role: key.Role,
		})
	}
	for _, mode := range schemaPort.Modes {
		schema.Modes = append(schema.Modes, casesDomain.PartModeSpecDomain{
			Mode: mode.Mode, Label: mode.Label, Priority: mode.Priority, Requires: mode.Requires,
		})
	}
	factInputs := make([]casesDomain.PartRequestFactInput, 0, len(inputs))
	factInputs = append(factInputs, inputs...)
	return casesDomain.DerivePartRequest(schema, factInputs, 0).PartRequestDigest, nil
}

// fitmentView assembles the Story 4.4 applicability panel of the case
// view: the pack fitment policy through the narrow taxonomy port (the
// panel degrades with the honest word when the pin is unreadable or the
// pack carries no policy), the append-only evidence records of the case
// grouped by the position, the CURRENT digest pins of BOTH inputs (the
// same live derivations of the record command — the read-time ladder
// re-derives the stale states on every read, AC-11), the safety-node flag
// (the pack markers over the normalized part kind — the 4.3 discipline)
// and the clarifiable profile attributes of the ambiguous questions (the
// optional identification keys of 4.2 with the confirmed flags — the
// confirmed attributes are never re-asked, AC-5).
func (s *Service) fitmentView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.FitmentPanelView, error) {
	panel := casesAPI.FitmentPanelView{Note: casesAPI.FitmentSchemaNoteAbsent}
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return panel, nil
	}
	policyPort, err := s.taxonomy.PackFitmentSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(policyPort.LinkTypes) == 0 {
		return panel, nil
	}
	policy := casesDomain.FitmentPolicyDomain{
		LinkTypes:      policyPort.LinkTypes,
		MaxCrossDepth:  policyPort.MaxCrossDepth,
		AllowedSources: policyPort.AllowedSources,
		MaxAgeSeconds:  policyPort.MaxAgeSeconds,
	}
	// The facts of the case: the CURRENT digest pins of BOTH inputs (the
	// same live derivations of the record command) and the safety-node
	// flag over the part kind material.
	facts, err := s.repository.ReadFactsOfCase(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.FitmentPanelView{}, err
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
		return casesAPI.FitmentPanelView{}, err
	}
	partRequestDigest, partSchema, partInputs, err := s.livePartRequestDigestWithSchema(ctx, scope, detail, inputs)
	if err != nil {
		return casesAPI.FitmentPanelView{}, err
	}
	// The safety-node flag: the pack markers over the normalized part kind
	// value (the 4.3 discipline — the declared heuristic of the pack,
	// honestly a heuristic, never a proof).
	safetyNode := fitmentSafetyNodeOf(partSchema, partInputs)
	// The clarifiable profile attributes of the ambiguous questions: the
	// optional identification keys with the confirmed flags (AC-5 — the
	// confirmed attributes are never re-asked).
	clarifiable, err := s.fitmentClarifiableAttributes(ctx, scope, detail, inputs)
	if err != nil {
		return casesAPI.FitmentPanelView{}, err
	}
	// The append-only evidence records grouped by the position (the store
	// reads them ordered; the ladder elects the current record of every
	// source by the number).
	records, err := s.repository.ReadFitmentEvidencesOfCase(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.FitmentPanelView{}, err
	}
	positions := make([]casesDomain.FitmentPositionInput, 0, len(records))
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
	panel.SchemaDefined = true
	panel.Note = ""
	panel.Fitment = casesDomain.DeriveFitment(casesDomain.FitmentDerivationInput{
		Policy:                       policy,
		Positions:                    positions,
		ProfileDigest:                profileDigest,
		PartRequestDigest:            partRequestDigest,
		Now:                          s.clock.Now(),
		SafetyNode:                   safetyNode,
		ClarifiableProfileAttributes: clarifiable,
	})
	return panel, nil
}

// fitmentRawValueText renders the raw text of one fact value (the quoted
// JSON string form — the 2.2 value discipline; the same rendering as the
// domain view helper, kept local for the app-side flag derivation).
func fitmentRawValueText(value []byte) string {
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return text
	}
	return string(value)
}

// fitmentSafetyNodeOf answers whether the part kind material of the case
// matches any declared safety marker of the pack (the 4.3 discipline: the
// substring search over the lower-cased trimmed kind — the markers are
// the declared heuristic of the pack, honestly a heuristic, never a
// proof).
func fitmentSafetyNodeOf(schema casesDomain.PartSchemaDomain, facts []casesDomain.VehicleFactInput) bool {
	kindKey, kindSpec := "", (*casesDomain.PartMaterialKeyDomain)(nil)
	for index := range schema.MaterialKeys {
		if schema.MaterialKeys[index].Role == casesDomain.PartRolePartKind {
			kindKey, kindSpec = schema.MaterialKeys[index].Key, &schema.MaterialKeys[index]
			break
		}
	}
	if kindSpec == nil || kindKey == "" {
		return false
	}
	byKey := map[string]casesDomain.VehicleFactInput{}
	for _, fact := range facts {
		byKey[fact.MaterialKey] = fact
	}
	fact, present := byKey[kindKey]
	if !present || len(fact.Versions) == 0 {
		return false
	}
	kind := strings.ToLower(strings.TrimSpace(fitmentRawValueText(fact.Versions[len(fact.Versions)-1].Value)))
	if kind == "" {
		return false
	}
	for _, marker := range schema.SafetyMarkers {
		if strings.Contains(kind, strings.ToLower(strings.TrimSpace(marker))) {
			return true
		}
	}
	return false
}

// fitmentClarifiableAttributes resolves the clarifiable profile
// attributes of the ambiguous questions (AC-5): the OPTIONAL
// identification keys of 4.2 with the confirmed flags — the confirmed
// attributes are never re-asked, the question asks the FIRST missing one.
func (s *Service) fitmentClarifiableAttributes(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail, inputs []casesDomain.VehicleFactInput) ([]casesDomain.FitmentProfileAttribute, error) {
	attributes := []casesDomain.FitmentProfileAttribute{}
	if detail.Case.PackID == "" || s.taxonomy == nil {
		return attributes, nil
	}
	schemaPort, err := s.taxonomy.PackIdentificationSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil {
		return attributes, nil
	}
	byKey := map[string]casesDomain.VehicleFactInput{}
	for _, fact := range inputs {
		byKey[fact.MaterialKey] = fact
	}
	for _, key := range schemaPort.MaterialKeys {
		if !key.Optional {
			continue
		}
		confirmed := false
		if fact, present := byKey[key.Key]; present && len(fact.Versions) > 0 {
			status, _ := casesDomain.DeriveFactStatus(fact.Versions, fact.Confirmations)
			confirmed = status == casesDomain.FactStatusConfirmed
		}
		attributes = append(attributes, casesDomain.FitmentProfileAttribute{Key: key.Key, Label: key.Label, Confirmed: confirmed})
	}
	return attributes, nil
}
