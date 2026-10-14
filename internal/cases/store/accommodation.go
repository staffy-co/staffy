package store

import (
	"context"
	"errors"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 2.11: the accommodation revision writes and the case reads
// ---------------------------------------------------------------------------

// TerminalCaseAccommodationText is the honest refusal of the record on a
// terminal case (the v1 backstop: the case-state dictionary carries only
// open today, the future closure words arrive with their own stories —
// the reserved-word precedent of mandatory_missing).
const TerminalCaseAccommodationText = "Терминальное обращение не принимает записей предпочтений связи: обслуживание завершено — записывать нечего."

// mapAccommodationInsertError maps the raced insert conflicts of the
// accommodation surface onto the honest domain errors: the command record
// UNIQUE is the material idempotency conflict; the revision UNIQUE is the
// raced revision derivation (the loser of the advisory-lock race sees the
// winner's committed revision — the honest stale-retry signal, the
// continuation_links_pkey precedent).
func mapAccommodationInsertError(err error, command string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "case_accommodations_pkey":
			// The raced identical record (the same record id under the same
			// command key): the honest stale-retry signal.
			return casesDomain.ErrStaleVersion
		case "case_accommodations_tenant_id_case_id_accommodation_key_revision_key":
			// The raced revision of the same key: the loser re-derives under
			// the locks and retries (the advisory locks serialize the normal
			// path — the UNIQUE is the defensive backstop).
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// scanAccommodationRecord reads one revision record row.
func scanAccommodationRecord(scan func(dest ...any) error, tenantID id.UUID) (casesDomain.AccommodationRecord, error) {
	var record casesDomain.AccommodationRecord
	var value *string
	err := scan(&record.RecordID, &record.CaseID, &record.Key, &record.Revision,
		&value, &record.Cleared, &record.Note, &record.ActorID, &record.RecordedAt)
	if err != nil {
		return record, err
	}
	if value != nil {
		record.Value = *value
	}
	record.TenantID = tenantID
	record.RecordedAt = record.RecordedAt.UTC()
	return record, nil
}

// readAccommodationsTx reads every revision record of one case ordered by
// the key and the monotonic revision (the derivation input of the CURRENT
// set and the honest panel history).
func readAccommodationsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.AccommodationRecord, error) {
	rows, err := tx.Query(ctx, `
                select record_id, case_id, accommodation_key, revision, value, cleared, note, actor_id, recorded_at
                from cases.case_accommodations
                where tenant_id=$1 and case_id=$2 order by accommodation_key, revision`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []casesDomain.AccommodationRecord{}
	for rows.Next() {
		record, err := scanAccommodationRecord(rows.Scan, tenantID)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// ReadAccommodations reads every append-only revision record of the
// communication preferences of one case (the own read transaction of the
// panel, the transfer view and the policy wiring input — the CURRENT set
// is the read-time domain derivation, never a stored status).
func (s *Postgres) ReadAccommodations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.AccommodationRecord, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	records, err := readAccommodationsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return records, tx.Commit(ctx)
}

// execRecordAccommodation writes one revision record under the case +
// conversation locks: the case must exist under the tenant scope (the
// honest 404 of the non-participant path) and stay open (the terminal
// backstop — the v1 dictionary carries the open word only), the domain
// guard re-validates under the locks (the key dictionary, the flag/valued
// shape, the channel word, the bounds and the clearing discipline — the
// table CHECK words are the backstops), the revision is the NEXT
// monotonic number of the key derived under the locks (the request never
// carries it), and the outbox event carries the metadata only (the key,
// the revision and the clearing mark — the value and the note never enter
// the envelope).
func (s *Postgres) execRecordAccommodation(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	var caseState string
	err := tx.QueryRow(ctx, `
                select state from cases.cases where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&caseState)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if caseState != casesDomain.CaseStateOpen {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: TerminalCaseAccommodationText}
	}
	// The domain guard under the locks (the honest 400 backstopped by the
	// table CHECK words).
	if err := casesDomain.GuardAccommodationRecord(p.AccommodationKey, p.AccommodationValue, p.AccommodationCleared, p.AccommodationNote); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The revision: the next monotonic number of the key under the locks
	// (the request never carries it; the UNIQUE backstop catches the race
	// the advisory locks already serialize).
	var revision int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(revision),0)+1 from cases.case_accommodations
                where tenant_id=$1 and case_id=$2 and accommodation_key=$3`,
		tenantID, p.CaseID, p.AccommodationKey).Scan(&revision); err != nil {
		return err
	}
	var value any
	if p.AccommodationValue != "" {
		value = p.AccommodationValue
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.case_accommodations (tenant_id, case_id, record_id, accommodation_key, revision, value, cleared, note, actor_id, recorded_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		tenantID, p.CaseID, p.AccommodationRecordID, p.AccommodationKey, revision,
		value, p.AccommodationCleared, p.AccommodationNote, p.Actor, now); err != nil {
		return mapAccommodationInsertError(err, p.Command)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	// The outbox event: the metadata only — the VALUE (the channel word, the
	// availability line) and the NOTE never enter the envelope.
	payload, err := casesDomain.AccommodationRecordedPayloadJSON(casesDomain.AccommodationRecordedPayload{
		TenantID: tenantID, CaseID: p.CaseID, RecordID: p.AccommodationRecordID,
		Key: p.AccommodationKey, Revision: revision, Cleared: p.AccommodationCleared,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventAccommodationRecorded, casesDomain.OutboxEventAccommodationRecordedV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyAccommodationRecorded(p.AccommodationRecordID), now); err != nil {
		return mapAccommodationInsertError(err, p.Command)
	}
	return nil
}
