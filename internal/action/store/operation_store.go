package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 3.5 (step-02): the operation persistence — the immutable row, the
// append-only ledger, the cross-case reservation and the module outbox
// ---------------------------------------------------------------------------
//
// ReserveOperation is the ONE transactional method of the whole contour
// (the Enqueue precedent of 3-3): the signature carries ZERO adapters and
// ZERO ports — a network call is impossible BY TYPE, the
// persist-before-emit discipline is structural. The chain inside ONE
// commit: the consumed re-selection under the confirmation advisory lock
// (the SAME lock the consent appends take — the raced second
// authorization of one single_use consent sees the first one's consumed
// event and refuses honestly), the allowance check over the definition's
// consumption rule word (a DATA pin of the params), then the INSERTs in
// the frozen order: the consumed event, the reservation, the operation,
// the ledger authorized entry and the outbox operation.queued record.
// ANY failure rolls back EVERYTHING — the consent stays unconsumed, the
// intent/operation/ledger/outbox never existed partially (AC-6
// material).
//
// The UNIQUE races map onto the two INTERNAL signals of the two key
// spaces: the (tenant, idempotency_key) twin is the replay signal (the
// gateway re-reads the winner by the key and compares the digests), the
// (tenant, owning_system, business_intent_key) reservation is the
// cross-case loser signal (the gateway reads the existing Operation and
// answers the honest reference). The two identities are structurally
// distinguishable — never a masked replay.

// mapOperationInsertError maps the raced insert conflicts onto the honest
// consumer-owned signals: the twin of one client key, the loser of the
// cross-case reservation, the raced consent event append.
func mapOperationInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "operations_tenant_id_idempotency_key_key":
			return actionPort.ErrIdempotencyTwin
		case "operations_pkey":
			return actionPort.ErrOperationIntegrity
		case "reservations_pkey":
			return actionPort.ErrReservationConflict
		case "confirmation_events_pkey", "confirmation_events_tenant_id_confirmation_id_version_no_event_no_key":
			return actionPort.ErrStaleConfirmationVersion
		}
	}
	return err
}

// ReadPinnedPreviewVersion reads one exact LIVE preview version row (the
// linked material of the confirm/authorization gates — the digest
// re-check compares this row byte-exact); domain.ErrNotFound answers a
// dead pin BEFORE any material.
func (s *Postgres) ReadPinnedPreviewVersion(ctx context.Context, scope tenancy.Context, previewID id.UUID, versionNo int64) (actionDomain.ActionPreview, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return actionDomain.ActionPreview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	versions, err := readVersionRows(ctx, tx, `
                select tenant_id, preview_id, version_no, definition_id, definition_version, case_id, work_item_id, target_kind, target_ref,
                       typed_input, supporting_facts, supporting_observations, snapshot, effects,
                       partition, recovery_instruction, human_owner_required, unsafe_class, no_reconciliation, human_only_control_mark,
                       owner_epoch, control_generation, profile_version, policy_outcome_no, capability_decl_version, digest, author_id, created_at
                from action.action_previews
                where tenant_id=$1 and preview_id=$2 and version_no=$3`,
		scope.TenantID(), previewID, versionNo)
	if err != nil {
		return actionDomain.ActionPreview{}, err
	}
	if len(versions) == 0 {
		return actionDomain.ActionPreview{}, actionDomain.ErrNotFound
	}
	return versions[0], tx.Commit(ctx)
}

// operationSelectColumns is the canonical column list of the operation
// reads.
const operationSelectColumns = `tenant_id, operation_id, definition_id, definition_version,
        preview_id, preview_version_no, confirmation_id, confirmation_version_no,
        case_id, work_item_id, target_kind, target_ref,
        owner_kind, owner_ref, owner_epoch, release_decision_no, profile_version,
        partition, effects, business_intent_key, idempotency_key, material_digest, created_at`

// scanOperationRows scans the operation rows of an open query — the
// shared scanner of the reads (the effects copy rehydrates from the
// jsonb column).
func scanOperationRows(ctx context.Context, rows pgx.Rows) ([]actionDomain.Operation, error) {
	defer rows.Close()
	out := []actionDomain.Operation{}
	for rows.Next() {
		var op actionDomain.Operation
		var effects []byte
		if err := rows.Scan(&op.TenantID, &op.OperationID, &op.DefinitionID, &op.DefinitionVersion,
			&op.PreviewID, &op.PreviewVersionNo, &op.ConfirmationID, &op.ConfirmationVersionNo,
			&op.CaseID, &op.WorkItemID, &op.Target.Kind, &op.Target.Ref,
			&op.OwnerKind, &op.OwnerRef, &op.OwnerEpoch, &op.ReleaseDecisionNo, &op.ProfileVersion,
			&op.Partition, &effects, &op.BusinessIntentKey, &op.IdempotencyKey, &op.MaterialDigest, &op.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(effects, &op.Effects); err != nil {
			return nil, err
		}
		op.CreatedAt = op.CreatedAt.UTC()
		out = append(out, op)
	}
	return out, rows.Err()
}

// FindOperationByIdempotencyKey reads the operation of one client key
// (the FIRST check of the gateway — the replay comparison).
func (s *Postgres) FindOperationByIdempotencyKey(ctx context.Context, scope tenancy.Context, idempotencyKey string) (actionDomain.Operation, bool, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return actionDomain.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select `+operationSelectColumns+`
                from action.operations
                where tenant_id=$1 and idempotency_key=$2`, scope.TenantID(), idempotencyKey)
	if err != nil {
		return actionDomain.Operation{}, false, err
	}
	ops, err := scanOperationRows(ctx, rows)
	if err != nil {
		return actionDomain.Operation{}, false, err
	}
	if len(ops) == 0 {
		return actionDomain.Operation{}, false, tx.Commit(ctx)
	}
	return ops[0], true, tx.Commit(ctx)
}

// FindOperationByIntent reads the operation reserved under one business
// intent (the loser reference of the cross-case race).
func (s *Postgres) FindOperationByIntent(ctx context.Context, scope tenancy.Context, owningSystem, businessIntentKey string) (actionDomain.Operation, bool, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return actionDomain.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var operationID id.UUID
	err = tx.QueryRow(ctx, `
                select operation_id from action.reservations
                where tenant_id=$1 and owning_system=$2 and business_intent_key=$3`,
		scope.TenantID(), owningSystem, businessIntentKey).Scan(&operationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actionDomain.Operation{}, false, tx.Commit(ctx)
		}
		return actionDomain.Operation{}, false, err
	}
	rows, err := tx.Query(ctx, `
                select `+operationSelectColumns+`
                from action.operations
                where tenant_id=$1 and operation_id=$2`, scope.TenantID(), operationID)
	if err != nil {
		return actionDomain.Operation{}, false, err
	}
	ops, err := scanOperationRows(ctx, rows)
	if err != nil {
		return actionDomain.Operation{}, false, err
	}
	if len(ops) == 0 {
		return actionDomain.Operation{}, false, tx.Commit(ctx)
	}
	return ops[0], true, tx.Commit(ctx)
}

// ReadCaseOperations reads every operation of one case ordered by time
// (the view material).
func (s *Postgres) ReadCaseOperations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionDomain.Operation, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select `+operationSelectColumns+`
                from action.operations
                where tenant_id=$1 and case_id=$2 order by created_at, operation_id`, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	ops, err := scanOperationRows(ctx, rows)
	if err != nil {
		return nil, err
	}
	return ops, tx.Commit(ctx)
}

// ReadOperationLedger reads the append-only ledger of one operation
// ordered by the monotonic entry number (the state derivation input).
func (s *Postgres) ReadOperationLedger(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]actionDomain.OperationLedgerEntry, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select entry_no, kind, details, created_at
                from action.operation_ledger
                where tenant_id=$1 and operation_id=$2 order by entry_no`, scope.TenantID(), operationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []actionDomain.OperationLedgerEntry{}
	for rows.Next() {
		var entry actionDomain.OperationLedgerEntry
		if err := rows.Scan(&entry.EntryNo, &entry.Kind, &entry.Details, &entry.At); err != nil {
			return nil, err
		}
		entry.At = entry.At.UTC()
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, tx.Commit(ctx)
}

// ReadOutboxQueued reports the queue FACT of one operation (the presence
// of the operation.queued record — the spoken word derives from it,
// nothing is stored).
func (s *Postgres) ReadOutboxQueued(ctx context.Context, scope tenancy.Context, operationID id.UUID) (bool, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var queued bool
	if err := tx.QueryRow(ctx, `
                select exists(select 1 from action.outbox_events
                where tenant_id=$1 and operation_id=$2 and event_type='operation.queued')`,
		scope.TenantID(), operationID).Scan(&queued); err != nil {
		return false, err
	}
	return queued, tx.Commit(ctx)
}

// ReserveOperation is the ONE transaction of the authorization (AC-5/AC-6):
// under the confirmation advisory lock — the consumed events of the
// pinned version re-select, the allowance re-derives over the definition's
// consumption rule word, then the INSERTs in the frozen order: the
// consumed event, the reservation, the operation, the ledger authorized
// entry and the outbox operation.queued record. The signature carries
// ZERO adapters — persist-before-emit is STRUCTURAL (the Enqueue
// precedent of 3-3); ANY failure rolls back the WHOLE chain: the consent
// stays unconsumed, the intent/operation/ledger/outbox never existed.
func (s *Postgres) ReserveOperation(ctx context.Context, p actionPort.ReserveOperationParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	// The consumption serialization: the SAME advisory lock the consent
	// appends take — the raced second authorization of one single_use
	// consent waits here, then sees the first one's consumed event.
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		confirmationLockKey(tenantID, p.Operation.ConfirmationID)); err != nil {
		return err
	}
	// The consumed re-selection under the lock: the count of the consumed
	// events of the PINNED version (read-time derivation discipline — the
	// count is never a stored column).
	var consumed int
	if err := tx.QueryRow(ctx, `
                select count(*) from action.confirmation_events
                where tenant_id=$1 and confirmation_id=$2 and version_no=$3 and kind=$4`,
		tenantID, p.Operation.ConfirmationID, p.Operation.ConfirmationVersionNo,
		actionDomain.ConfirmationEventConsumed).Scan(&consumed); err != nil {
		return err
	}
	allowance, err := actionDomain.DeriveConsumptionAllowance(p.ConsumptionRule)
	if err != nil {
		return err
	}
	if allowance.Bounded && consumed >= allowance.Limit {
		// The raced consumption: the honest refusal UNDER the lock — the
		// whole transaction rolls back, this attempt consumed NOTHING.
		return actionPort.ErrConfirmationExhausted
	}
	// INSERT 1/5 — the consumed event of the pinned version (one event per
	// successful authorization; the read-time count is the audit).
	if err := insertConfirmationEventRow(ctx, tx, tenantID,
		p.Operation.ConfirmationID, p.Operation.ConfirmationVersionNo,
		p.Operation.CaseID, p.ConsumedEventID,
		actionDomain.ConfirmationEventConsumed, "", "", p.ActorID, p.Now); err != nil {
		return err
	}
	// INSERT 2/5 — the cross-case reservation (the loser of the UNIQUE is
	// the honest reference, never a masked replay).
	if _, err := tx.Exec(ctx, `
                insert into action.reservations (tenant_id, owning_system, business_intent_key, operation_id, case_id, created_at)
                values ($1,$2,$3,$4,$5,$6)`,
		tenantID, p.OutboxPayload.OwningSystem, p.Operation.BusinessIntentKey,
		p.Operation.OperationID, p.Operation.CaseID, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return mapOperationInsertError(err)
	}
	// INSERT 3/5 — the immutable operation row.
	effects, err := marshalJSON(p.Operation.Effects)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
                insert into action.operations (tenant_id, operation_id, definition_id, definition_version,
                        preview_id, preview_version_no, confirmation_id, confirmation_version_no,
                        case_id, work_item_id, target_kind, target_ref,
                        owner_kind, owner_ref, owner_epoch, release_decision_no, profile_version,
                        partition, effects, business_intent_key, idempotency_key, material_digest, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`,
		tenantID, p.Operation.OperationID, p.Operation.DefinitionID, p.Operation.DefinitionVersion,
		p.Operation.PreviewID, p.Operation.PreviewVersionNo, p.Operation.ConfirmationID, p.Operation.ConfirmationVersionNo,
		p.Operation.CaseID, p.Operation.WorkItemID, p.Operation.Target.Kind, p.Operation.Target.Ref,
		p.Operation.OwnerKind, p.Operation.OwnerRef, p.Operation.OwnerEpoch,
		p.Operation.ReleaseDecisionNo, p.Operation.ProfileVersion,
		p.Operation.Partition, effects, p.Operation.BusinessIntentKey,
		p.Operation.IdempotencyKey, p.Operation.MaterialDigest, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return mapOperationInsertError(err)
	}
	// INSERT 4/5 — the ledger authorized entry (AD-13: the append-only
	// history starts here; 3-6 will append the dispatch words through the
	// same mechanism).
	if _, err := tx.Exec(ctx, `
                insert into action.operation_ledger (tenant_id, operation_id, entry_no, kind, details, created_at)
                values ($1,$2,$3,$4,$5,$6)`,
		tenantID, p.Operation.OperationID, p.LedgerEntry.EntryNo, p.LedgerEntry.Kind,
		p.LedgerEntry.Details, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return err
	}
	// INSERT 5/5 — the outbox operation.queued record (the SAFE payload —
	// the material NEVER enters; the drain contour is 3-6).
	payload, err := marshalJSON(p.OutboxPayload)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
                insert into action.outbox_events (tenant_id, event_id, event_type, case_id, operation_id, payload, created_at)
                values ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, p.OutboxEventID, "operation.queued", p.Operation.CaseID, p.Operation.OperationID,
		payload, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
