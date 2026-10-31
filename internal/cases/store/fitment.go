package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.4: the fitment evidence reads and the record command execution
// ---------------------------------------------------------------------------

// mapFitmentInsertError maps the raced insert conflicts of the fitment
// surface onto the honest domain errors: the evidence number UNIQUE is the
// monotonic backstop (the loser under the locks is impossible in the
// normal path — the backstop keeps the material truth anyway), the
// command record UNIQUE is the material idempotency conflict.
func mapFitmentInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "fitment_evidences_tenant_id_work_item_id_position_key_source_system_evidence_no_key":
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// readFitmentEvidencesTx reads every fitment evidence record of one case
// ordered by the work item, the position, the source and the monotonic
// number (the derivation input — the read-time ladder elects the current
// record of every source by the number).
func readFitmentEvidencesTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesPort.FitmentEvidenceRecord, error) {
	rows, err := tx.Query(ctx, `
		select work_item_id, evidence_id, evidence_no, position_key, catalog_node, article, link_type,
		       chain, catalog_vehicle_ids, source_system, source_record_id, source_version,
		       source_observed_at, received_at, profile_digest_pin, part_request_digest_pin, fitment_digest, created_at
		from cases.fitment_evidences
		where tenant_id=$1 and case_id=$2 order by work_item_id, position_key, source_system, evidence_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []casesPort.FitmentEvidenceRecord{}
	for rows.Next() {
		var record casesPort.FitmentEvidenceRecord
		var catalogNode, article *string
		var chain, vehicleIDs []byte
		if err := rows.Scan(&record.WorkItemID, &record.EvidenceID, &record.EvidenceNo, &record.PositionKey,
			&catalogNode, &article, &record.LinkType,
			&chain, &vehicleIDs, &record.SourceSystem, &record.SourceRecordID, &record.SourceVersion,
			&record.SourceObservedAt, &record.ReceivedAt, &record.ProfileDigestPin, &record.PartRequestDigestPin,
			&record.FitmentDigest, &record.CreatedAt); err != nil {
			return nil, err
		}
		if catalogNode != nil {
			record.CatalogNode = *catalogNode
		}
		if article != nil {
			record.Article = *article
		}
		if err := json.Unmarshal(chain, &record.Chain); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(vehicleIDs, &record.CatalogVehicleIDs); err != nil {
			return nil, err
		}
		record.SourceObservedAt = record.SourceObservedAt.UTC()
		record.ReceivedAt = record.ReceivedAt.UTC()
		record.CreatedAt = record.CreatedAt.UTC()
		records = append(records, record)
	}
	return records, rows.Err()
}

// ReadFitmentEvidencesOfCase reads every fitment evidence record of the
// case (the own read transaction of the view and the command pre-checks).
func (s *Postgres) ReadFitmentEvidencesOfCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.FitmentEvidenceRecord, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	records, err := readFitmentEvidencesTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return records, tx.Commit(ctx)
}

// fitmentNextNoTx resolves the next monotonic evidence number of one
// position of one source of the Work Item under the locks (the max+1 of
// the append-only ladder — the supersede is the NEW number, never an
// update).
func fitmentNextNoTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID id.UUID, positionKey, sourceSystem string) (int64, error) {
	records, err := readFitmentEvidencesTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return 0, err
	}
	nextNo := int64(0)
	for _, record := range records {
		if record.WorkItemID == workItemID && record.PositionKey == positionKey && record.SourceSystem == sourceSystem && record.EvidenceNo > nextNo {
			nextNo = record.EvidenceNo
		}
	}
	return nextNo + 1, nil
}

// execRecordFitmentEvidence writes ONE append-only fitment evidence record
// under the case + work item locks: the Work Item must be anchored in the
// case and ALIVE (the terminal states refuse honestly — the merged item
// lives on in its target), the guard material re-validates under the
// locks (the store never trusts the app bytes — the SQL CHECK words are
// the backstops), the evidence number is the per-(work item, position,
// source) monotonic sequence under the lock and the record materializes
// as the append-only row. NO outbox event: the proof stays inside the
// case perimeter (the append-only row is the audit; the envelope
// vocabulary is not extended by this story).
func (s *Postgres) execRecordFitmentEvidence(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if p.Fitment == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "материал записи доказательства отсутствует"}
	}
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	// The Work Item must be alive: the terminal states refuse honestly.
	currents, err := readWorkItemCurrentsTx(ctx, tx, tenantID, p.CaseID, []id.UUID{p.WorkItemID})
	if err != nil {
		return err
	}
	current, ok := currents[p.WorkItemID]
	if !ok {
		return casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.TerminalWorkItemReadText}
	}
	// The domain guard re-validation under the locks: the dictionaries and
	// the material shape of the record (the store never trusts the app
	// bytes; the SQL CHECK mirrors are the backstops).
	draft := casesDomain.FitmentEvidenceDraft{
		PositionKey:          p.Fitment.PositionKey,
		CatalogNode:          p.Fitment.CatalogNode,
		Article:              p.Fitment.Article,
		LinkType:             p.Fitment.LinkType,
		Chain:                p.Fitment.Chain,
		CatalogVehicleIDs:    p.Fitment.CatalogVehicleIDs,
		SourceSystem:         p.Fitment.SourceSystem,
		SourceRecordID:       p.Fitment.SourceRecordID,
		SourceVersion:        p.Fitment.SourceVersion,
		SourceObservedAt:     p.Fitment.SourceObservedAt,
		ProfileDigestPin:     p.Fitment.ProfileDigestPin,
		PartRequestDigestPin: p.Fitment.PartRequestDigestPin,
	}
	policy := casesDomain.FitmentPolicyDomain{
		LinkTypes:      p.FitmentPolicy.LinkTypes,
		MaxCrossDepth:  p.FitmentPolicy.MaxCrossDepth,
		AllowedSources: p.FitmentPolicy.AllowedSources,
		MaxAgeSeconds:  p.FitmentPolicy.MaxAgeSeconds,
	}
	if err := casesDomain.GuardFitmentEvidence(policy, draft); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The canonical digest re-computation under the locks: the stored
	// FitmentDigest is the SERVER truth of the record material (the app
	// carries its computation, the store re-derives the same bytes — the
	// byte-stable canonical form).
	evidence := casesDomain.FitmentEvidenceInput{
		EvidenceNo:           0,
		PositionKey:          p.Fitment.PositionKey,
		CatalogNode:          p.Fitment.CatalogNode,
		Article:              p.Fitment.Article,
		LinkType:             p.Fitment.LinkType,
		Chain:                p.Fitment.Chain,
		CatalogVehicleIDs:    p.Fitment.CatalogVehicleIDs,
		SourceSystem:         p.Fitment.SourceSystem,
		SourceRecordID:       p.Fitment.SourceRecordID,
		SourceVersion:        p.Fitment.SourceVersion,
		SourceObservedAt:     p.Fitment.SourceObservedAt,
		ReceivedAt:           p.ReceivedAt,
		ProfileDigestPin:     p.Fitment.ProfileDigestPin,
		PartRequestDigestPin: p.Fitment.PartRequestDigestPin,
	}
	if digest := casesDomain.FitmentDigestOf(evidence); digest != p.Fitment.FitmentDigest {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "канонический дайджест записи не совпадает с материалом — пересчитайте доказательство"}
	}
	// The jsonb columns are ALWAYS arrays: a nil list marshals into the
	// empty array (the honest not-found record carries the EMPTY id list —
	// never SQL NULL, the CHECK mirrors the array shape).
	chain := p.Fitment.Chain
	if chain == nil {
		chain = []string{}
	}
	vehicleIDs := p.Fitment.CatalogVehicleIDs
	if vehicleIDs == nil {
		vehicleIDs = []string{}
	}
	chainJSON, err := json.Marshal(chain)
	if err != nil {
		return err
	}
	vehicleIDsJSON, err := json.Marshal(vehicleIDs)
	if err != nil {
		return err
	}
	evidenceNo, err := fitmentNextNoTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.Fitment.PositionKey, p.Fitment.SourceSystem)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		insert into cases.fitment_evidences (tenant_id, case_id, work_item_id, evidence_id, evidence_no,
			position_key, catalog_node, article, link_type, chain, catalog_vehicle_ids,
			source_system, source_record_id, source_version, source_observed_at, received_at,
			actor_id, correlation_id, profile_digest_pin, part_request_digest_pin, fitment_digest, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
		tenantID, p.CaseID, p.WorkItemID, p.Fitment.EvidenceID, evidenceNo,
		p.Fitment.PositionKey, nilText(p.Fitment.CatalogNode), nilText(p.Fitment.Article), p.Fitment.LinkType, chainJSON, vehicleIDsJSON,
		p.Fitment.SourceSystem, p.Fitment.SourceRecordID, p.Fitment.SourceVersion, p.Fitment.SourceObservedAt, p.ReceivedAt,
		p.Actor, p.CorrelationID, p.Fitment.ProfileDigestPin, p.Fitment.PartRequestDigestPin, p.Fitment.FitmentDigest, now); err != nil {
		return mapFitmentInsertError(err)
	}
	return nil
}
