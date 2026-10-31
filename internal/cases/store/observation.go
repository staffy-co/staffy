package store

import (
	"context"
	"errors"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 2.7: the owning-system observation reads and the read command
// execution
// ---------------------------------------------------------------------------

// mapObservationInsertError maps the raced insert conflicts of the
// observation surface onto the honest domain errors: the observation number
// UNIQUE is the monotonic backstop (the loser under the locks is impossible
// in the normal path — the backstop keeps the material truth anyway), the
// command record UNIQUE is the material idempotency conflict.
func mapObservationInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "owning_system_observations_tenant_id_work_item_id_material_key_observation_no_key":
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// readObservationsTx reads every observation of one case ordered by the
// Work Item, the material field and the monotonic number (the derivation
// input of the CURRENT observation per field of the panel).
func readObservationsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.Observation, error) {
	rows, err := tx.Query(ctx, `
                select observation_id, case_id, work_item_id, material_key, observation_no, owning_system, status,
                       reason, value, projection, external_id, external_version, observed_at, received_at, actor_id, correlation_id, created_at
                from cases.owning_system_observations
                where tenant_id=$1 and case_id=$2 order by work_item_id, material_key, observation_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	observations := []casesDomain.Observation{}
	for rows.Next() {
		var observation casesDomain.Observation
		var reason, externalID, externalVersion *string
		var value *[]byte
		var observedAt *time.Time
		if err := rows.Scan(&observation.ObservationID, &observation.CaseID, &observation.WorkItemID,
			&observation.MaterialKey, &observation.ObservationNo, &observation.OwningSystem, &observation.Status,
			&reason, &value, &observation.Projection, &externalID, &externalVersion, &observedAt,
			&observation.ReceivedAt, &observation.ActorID, &observation.CorrelationID, &observation.CreatedAt); err != nil {
			return nil, err
		}
		if reason != nil {
			observation.Reason = *reason
		}
		if externalID != nil {
			observation.ExternalID = *externalID
		}
		if externalVersion != nil {
			observation.ExternalVersion = *externalVersion
		}
		if value != nil {
			observation.Value = *value
		}
		if observedAt != nil {
			observation.ObservedAt = (*observedAt).UTC()
		}
		observation.TenantID = tenantID
		observation.ReceivedAt = observation.ReceivedAt.UTC()
		observation.CreatedAt = observation.CreatedAt.UTC()
		observations = append(observations, observation)
	}
	return observations, rows.Err()
}

// ReadObservations reads every observation of one case (the own read
// transaction of the view and the command pre-checks).
func (s *Postgres) ReadObservations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.Observation, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	observations, err := readObservationsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return observations, tx.Commit(ctx)
}

// observationNextNoTx resolves the next monotonic observation number of one
// material field of the Work Item under the locks (the max+1 of the
// append-only ladder — the per-field counter of the read command).
func observationNextNoTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID id.UUID, materialKey string) (int64, error) {
	observations, err := readObservationsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return 0, err
	}
	nextNo := int64(0)
	for _, observation := range observations {
		if observation.WorkItemID == workItemID && observation.MaterialKey == materialKey && observation.ObservationNo > nextNo {
			nextNo = observation.ObservationNo
		}
	}
	return nextNo + 1, nil
}

// execReadOwningSystem writes one observation under the case + work item
// locks: the Work Item must be anchored in the case and ALIVE (the terminal
// states refuse honestly — the merged item lives on in its target), the
// material field must be the matrix dictionary (the CHECK backstop), the
// observation number is the per-(work item, material field) monotonic
// sequence under the lock, the port answer materializes as the append-only
// row (ok with the value and the carrier words; failed/unknown with the
// reason word and NO value) and the outbox event carries the metadata only
// (the projection value never enters the envelope).
func (s *Postgres) execReadOwningSystem(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	// The Work Item must be alive: the terminal states refuse honestly (the
	// merged item lives on in its target).
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
	// The matrix field guard under the locks (the honest unknown of the
	// class — the dictionary CHECK backstops the same words).
	if err := casesDomain.GuardMaterialFieldInMatrix(p.MaterialKey); err != nil {
		return casesDomain.WorkRuleError{Rule: "field_not_in_matrix", Reason: err.Error()}
	}
	// The port answer guard (the fail-closed basis against a misbehaving
	// adapter — the store never trusts the app bytes).
	if err := casesDomain.GuardObservationMaterial(p.ReadStatus, p.ReadReason, p.ReadValue, p.ExternalID, p.ExternalVersion, p.ObservedAt); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	observationNo, err := observationNextNoTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.MaterialKey)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.owning_system_observations (tenant_id, case_id, work_item_id, material_key, observation_no,
                        observation_id, owning_system, status, reason, value, projection, external_id, external_version,
                        observed_at, received_at, actor_id, correlation_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		tenantID, p.CaseID, p.WorkItemID, p.MaterialKey, observationNo,
		p.ObservationID, p.OwningSystem, p.ReadStatus, nilText(p.ReadReason), nilBytes(p.ReadValue), p.Projection,
		nilText(p.ExternalID), nilText(p.ExternalVersion), nilTime(p.ObservedAt), p.ReceivedAt, p.Actor, p.CorrelationID, now); err != nil {
		return mapObservationInsertError(err)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	payload, err := casesDomain.OwningSystemObservedPayloadJSON(casesDomain.OwningSystemObservedPayload{
		TenantID: tenantID, CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		ObservationID: p.ObservationID, ObservationNo: observationNo,
		MaterialKey: p.MaterialKey, OwningSystem: p.OwningSystem, Projection: p.Projection,
		Status: p.ReadStatus, Reason: p.ReadReason, ExternalID: p.ExternalID, ExternalVersion: p.ExternalVersion,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventOwningSystemObserved, casesDomain.OutboxEventOwningSystemObservedV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyOwningSystemObserved(p.ObservationID), now); err != nil {
		return mapObservationInsertError(err)
	}
	return nil
}

// nilBytes renders the nullable jsonb parameter of the append-only inserts
// (the value of a failed/unknown read is NULL — never an empty document).
func nilBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

// nilTime renders the nullable timestamptz parameter of the append-only
// inserts (the provider time of a failed/unknown read is NULL — nothing was
// observed).
func nilTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
