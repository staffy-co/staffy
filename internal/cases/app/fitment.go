package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
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
