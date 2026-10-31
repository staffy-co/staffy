package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 3.5 (step-02): the confirmation persistence of the action module
// ---------------------------------------------------------------------------
//
// The consent ladder mirrors the version ladder of 3-4: one advisory lock
// per confirmation thread (tenant, confirmation_id), the version number
// re-derived INSIDE the lock (the raced twin is the honest stale refusal,
// never a silent merge), the supersede is INSERT-only material of the
// SAME transaction (the NEW version carries the new preview pin, the OLD
// version gets the superseded event — the old row's bytes never move).
// The UNIQUE (tenant, preview_id, preview_version_no) is the structural
// 1:1 stream mapping: no preview version is ever confirmed twice — the
// raced first-confirm twin of two fresh threads and the raced
// re-confirm twin of one thread are both the honest structural refusals.
//
// The store owns NO adapters (the 3-3 discipline): the events append is a
// pure persistence boundary; the revoked event carries the human act of
// the app layer, the consumed event belongs to the ReserveOperation
// transaction of the operation store.

// The structural pins: the same Postgres store satisfies both repository
// contracts of the contour (the preview store file owns the struct).
var (
	_ actionPort.ConfirmationRepository = (*Postgres)(nil)
	_ actionPort.OperationRepository    = (*Postgres)(nil)
)

// confirmationLockKey is the advisory lock key of one consent thread (the
// version ladder and the event appends serialize under it; the
// consumption re-selection of ReserveOperation shares the same lock —
// the raced second authorization of one single_use consent sees the
// first one's consumed event honestly).
func confirmationLockKey(tenantID id.UUID, confirmationID id.UUID) string {
	return "action.confirmation.v1:" + tenantID.String() + ":" + confirmationID.String()
}

// mapConfirmationInsertError maps the raced insert conflicts onto the
// honest consumer-owned errors: the version PK and the version-pin
// bijection are the ladder backstops, the event UNIQUE is the raced
// append twin.
func mapConfirmationInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "confirmations_pkey":
			return actionPort.ErrStaleConfirmationVersion
		case "confirmations_tenant_id_preview_id_preview_version_no_key":
			return actionPort.ErrStaleConfirmationVersion
		case "confirmation_events_pkey":
			return actionPort.ErrStaleConfirmationVersion
		case "confirmation_events_tenant_id_confirmation_id_version_no_event_no_key":
			return actionPort.ErrStaleConfirmationVersion
		}
	}
	return err
}

// readConfirmationRows scans the confirmation version rows of a query —
// the shared scanner of the thread and case reads.
func readConfirmationRows(ctx context.Context, rows pgx.Rows) ([]actionDomain.Confirmation, error) {
	defer rows.Close()
	out := []actionDomain.Confirmation{}
	for rows.Next() {
		var c actionDomain.Confirmation
		var snapshot, effects []byte
		if err := rows.Scan(&c.TenantID, &c.ConfirmationID, &c.VersionNo,
			&c.DefinitionID, &c.DefinitionVersion, &c.CaseID, &c.WorkItemID,
			&c.Target.Kind, &c.Target.Ref, &c.OwnerKind, &c.OwnerRef, &c.OwnerEpoch,
			&c.ReleaseDecisionNo, &c.ProfileVersion,
			&c.PreviewID, &c.PreviewVersionNo, &c.PreviewDigest,
			&snapshot, &effects, &c.ValidUntil, &c.ConfirmedBy, &c.CreatedAt); err != nil {
			return nil, err
		}
		unmarshalled, err := unmarshalConfirmationMaterial(snapshot, effects, &c)
		if err != nil {
			return nil, err
		}
		c = unmarshalled
		c.CreatedAt = c.CreatedAt.UTC()
		c.ValidUntil = c.ValidUntil.UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

// confirmationSelectColumns is the canonical column list of the reads.
const confirmationSelectColumns = `tenant_id, confirmation_id, version_no,
        definition_id, definition_version, case_id, work_item_id,
        target_kind, target_ref, owner_kind, owner_ref, owner_epoch,
        release_decision_no, profile_version,
        preview_id, preview_version_no, preview_digest,
        snapshot, effects, valid_until, confirmed_by, created_at`

// ReadPreviewConfirmations reads every confirmation version of one
// preview thread ordered by the monotonic version.
func (s *Postgres) ReadPreviewConfirmations(ctx context.Context, scope tenancy.Context, previewID id.UUID) ([]actionDomain.Confirmation, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select `+confirmationSelectColumns+`
                from action.confirmations
                where tenant_id=$1 and preview_id=$2 order by version_no`, scope.TenantID(), previewID)
	if err != nil {
		return nil, err
	}
	confirmations, err := readConfirmationRows(ctx, rows)
	if err != nil {
		return nil, err
	}
	return confirmations, tx.Commit(ctx)
}

// ReadCaseConfirmations reads every confirmation version of one case.
func (s *Postgres) ReadCaseConfirmations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionDomain.Confirmation, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select `+confirmationSelectColumns+`
                from action.confirmations
                where tenant_id=$1 and case_id=$2 order by confirmation_id, version_no`, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	confirmations, err := readConfirmationRows(ctx, rows)
	if err != nil {
		return nil, err
	}
	return confirmations, tx.Commit(ctx)
}

// ReadConfirmationVersion reads one exact confirmation version (the pin
// resolve of the authorization); domain.ErrNotFound answers a dead pin.
func (s *Postgres) ReadConfirmationVersion(ctx context.Context, scope tenancy.Context, confirmationID id.UUID, versionNo int64) (actionDomain.Confirmation, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return actionDomain.Confirmation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select `+confirmationSelectColumns+`
                from action.confirmations
                where tenant_id=$1 and confirmation_id=$2 and version_no=$3`,
		scope.TenantID(), confirmationID, versionNo)
	if err != nil {
		return actionDomain.Confirmation{}, err
	}
	confirmations, err := readConfirmationRows(ctx, rows)
	if err != nil {
		return actionDomain.Confirmation{}, err
	}
	if len(confirmations) == 0 {
		return actionDomain.Confirmation{}, actionDomain.ErrNotFound
	}
	return confirmations[0], tx.Commit(ctx)
}

// readConfirmationEventsTx reads the domain events of one exact version
// ordered by the monotonic number (the state derivation input).
func readConfirmationEventsTx(ctx context.Context, tx pgx.Tx, tenantID, confirmationID id.UUID, versionNo int64) ([]actionDomain.ConfirmationEvent, error) {
	rows, err := tx.Query(ctx, `
                select kind, reason, details, created_at, actor_id
                from action.confirmation_events
                where tenant_id=$1 and confirmation_id=$2 and version_no=$3 order by event_no`,
		tenantID, confirmationID, versionNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []actionDomain.ConfirmationEvent{}
	for rows.Next() {
		var e actionDomain.ConfirmationEvent
		if err := rows.Scan(&e.Kind, &e.Reason, &e.Details, &e.At, &e.ActorID); err != nil {
			return nil, err
		}
		e.At = e.At.UTC()
		events = append(events, e)
	}
	return events, rows.Err()
}

// ReadConfirmationVersionEvents reads the events of one exact version
// (the state derivation input — the consumption counts per version).
func (s *Postgres) ReadConfirmationVersionEvents(ctx context.Context, scope tenancy.Context, confirmationID id.UUID, versionNo int64) ([]actionDomain.ConfirmationEvent, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	events, err := readConfirmationEventsTx(ctx, tx, scope.TenantID(), confirmationID, versionNo)
	if err != nil {
		return nil, err
	}
	return events, tx.Commit(ctx)
}

// ReadCaseConfirmationEvents reads every confirmation event of one case
// ordered by time (the view material with the visible exceptions).
func (s *Postgres) ReadCaseConfirmationEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionPort.ConfirmationEventRow, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select confirmation_id, version_no, case_id, event_id, event_no, kind, reason, details, actor_id, created_at
                from action.confirmation_events
                where tenant_id=$1 and case_id=$2 order by created_at, event_no`, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []actionPort.ConfirmationEventRow{}
	for rows.Next() {
		var row actionPort.ConfirmationEventRow
		if err := rows.Scan(&row.ConfirmationID, &row.VersionNo, &row.CaseID, &row.EventID, &row.EventNo,
			&row.Kind, &row.Reason, &row.Details, &row.ActorID, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.CreatedAt = row.CreatedAt.UTC()
		events = append(events, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, tx.Commit(ctx)
}

// maxConfirmationVersionTx re-derives the version head of one consent
// thread under the locks (the raced append is the honest stale refusal).
func maxConfirmationVersionTx(ctx context.Context, tx pgx.Tx, tenantID, confirmationID id.UUID) (int64, error) {
	var maxVersion int64
	err := tx.QueryRow(ctx, `
                select coalesce(max(version_no), 0) from action.confirmations
                where tenant_id=$1 and confirmation_id=$2`, tenantID, confirmationID).Scan(&maxVersion)
	if err != nil {
		return 0, err
	}
	return maxVersion, nil
}

// maxConfirmationEventNoTx reads the monotonic event counter of one
// version scope under the locks.
func maxConfirmationEventNoTx(ctx context.Context, tx pgx.Tx, tenantID, confirmationID id.UUID, versionNo int64) (int64, error) {
	var maxEvent int64
	err := tx.QueryRow(ctx, `
                select coalesce(max(event_no), 0) from action.confirmation_events
                where tenant_id=$1 and confirmation_id=$2 and version_no=$3`,
		tenantID, confirmationID, versionNo).Scan(&maxEvent)
	if err != nil {
		return 0, err
	}
	return maxEvent, nil
}

// insertConfirmationEventRow writes one append-only consent event with
// the re-derived monotonic number of its version scope.
func insertConfirmationEventRow(ctx context.Context, tx pgx.Tx, tenantID, confirmationID id.UUID, versionNo int64,
	caseID, eventID id.UUID, kind, reason, details string, actorID id.UUID, now time.Time) error {
	maxEvent, err := maxConfirmationEventNoTx(ctx, tx, tenantID, confirmationID, versionNo)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
                insert into action.confirmation_events (tenant_id, confirmation_id, version_no, case_id, event_id, event_no, kind, reason, details, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, confirmationID, versionNo, caseID, eventID, maxEvent+1, kind, reason, details, actorID, now); err != nil {
		return mapConfirmationInsertError(err)
	}
	return nil
}

// marshalConfirmationMaterial renders the linked material copies of one
// consent version (the canonical jsonb columns of the migration).
func marshalConfirmationMaterial(c actionDomain.Confirmation) (snapshot, effects []byte, err error) {
	snapshot, err = marshalJSON(c.Snapshot)
	if err != nil {
		return nil, nil, err
	}
	effects, err = marshalJSON(c.Effects)
	if err != nil {
		return nil, nil, err
	}
	return snapshot, effects, nil
}

// unmarshalConfirmationMaterial rehydrates the linked material copies of
// one scanned row (the jsonb columns scan as raw bytes — the readVersionRows
// scanner precedent of the preview store).
func unmarshalConfirmationMaterial(snapshot, effects []byte, c *actionDomain.Confirmation) (actionDomain.Confirmation, error) {
	out := *c
	if err := json.Unmarshal(snapshot, &out.Snapshot); err != nil {
		return actionDomain.Confirmation{}, err
	}
	if err := json.Unmarshal(effects, &out.Effects); err != nil {
		return actionDomain.Confirmation{}, err
	}
	if out.Snapshot == nil {
		// The honest empty set — never a nil (the jsonb array CHECK of the
		// migration; the 3-4 nil-branch lesson).
		out.Snapshot = []actionDomain.MaterialSnapshotRow{}
	}
	if out.Effects == nil {
		out.Effects = []actionDomain.PreviewEffect{}
	}
	return out, nil
}

// AppendConfirmationVersion appends one consent version under the
// confirmation advisory lock — ONE transaction: the version row, the
// confirmed event of the NEW version and (when this is not the first
// version) the superseded event of the OLD version. The store re-derives
// the version head; a moved ladder answers the honest stale refusal. The
// UNIQUE (tenant, preview_id, preview_version_no) is the structural
// bijection backstop — the raced confirm twins (two fresh threads of one
// preview version, or two re-confirms of one thread onto one preview
// version) are the honest structural refusals, nothing merges.
func (s *Postgres) AppendConfirmationVersion(ctx context.Context, p actionPort.AppendConfirmationParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		confirmationLockKey(tenantID, p.Confirmation.ConfirmationID)); err != nil {
		return err
	}
	maxVersion, err := maxConfirmationVersionTx(ctx, tx, tenantID, p.Confirmation.ConfirmationID)
	if err != nil {
		return err
	}
	if maxVersion+1 != p.Confirmation.VersionNo {
		// The raced ladder: the twin appended first — the honest stale
		// refusal, never a merge.
		return actionPort.ErrStaleConfirmationVersion
	}
	if (maxVersion == 0) != (p.PrevVersionNo == 0) {
		// The app's thread resolve and the store's head disagree about the
		// first/existing shape — the honest stale refusal.
		return actionPort.ErrStaleConfirmationVersion
	}
	snapshot, effects, err := marshalConfirmationMaterial(p.Confirmation)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
                insert into action.confirmations (tenant_id, confirmation_id, version_no, definition_id, definition_version,
                        case_id, work_item_id, target_kind, target_ref, owner_kind, owner_ref, owner_epoch,
                        release_decision_no, profile_version, preview_id, preview_version_no, preview_digest,
                        snapshot, effects, valid_until, confirmed_by, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
		tenantID, p.Confirmation.ConfirmationID, p.Confirmation.VersionNo,
		p.Confirmation.DefinitionID, p.Confirmation.DefinitionVersion,
		p.Confirmation.CaseID, p.Confirmation.WorkItemID,
		p.Confirmation.Target.Kind, p.Confirmation.Target.Ref,
		p.Confirmation.OwnerKind, p.Confirmation.OwnerRef, p.Confirmation.OwnerEpoch,
		p.Confirmation.ReleaseDecisionNo, p.Confirmation.ProfileVersion,
		p.Confirmation.PreviewID, p.Confirmation.PreviewVersionNo, p.Confirmation.PreviewDigest,
		snapshot, effects, p.Confirmation.ValidUntil, p.Confirmation.ConfirmedBy,
		p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return mapConfirmationInsertError(err)
	}
	if err := insertConfirmationEventRow(ctx, tx, tenantID, p.Confirmation.ConfirmationID,
		p.Confirmation.VersionNo, p.Confirmation.CaseID, p.ConfirmedEventID,
		actionDomain.ConfirmationEventConfirmed, "", "", p.Confirmation.ConfirmedBy, p.Now); err != nil {
		return err
	}
	if p.PrevVersionNo > 0 {
		if err := insertConfirmationEventRow(ctx, tx, tenantID, p.Confirmation.ConfirmationID,
			p.PrevVersionNo, p.Confirmation.CaseID, p.SupersededEventID,
			actionDomain.ConfirmationEventSuperseded, "", "", p.Confirmation.ConfirmedBy, p.Now); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// AppendConfirmationEvent appends ONE history event of one exact version
// under the confirmation advisory lock (revoked of the human revocation,
// authorization_conflict of the visible digest mismatch,
// authorization_blocked of the visible ladder rejection). The history is
// never rewritten; the kind is the domain's guarded dictionary.
func (s *Postgres) AppendConfirmationEvent(ctx context.Context, p actionPort.AppendConfirmationEventParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		confirmationLockKey(tenantID, p.ConfirmationID)); err != nil {
		return err
	}
	// The version must exist (the honest stale refusal of a dead pin
	// before any material).
	var exists bool
	if err := tx.QueryRow(ctx, `
                select exists(select 1 from action.confirmations
                where tenant_id=$1 and confirmation_id=$2 and version_no=$3)`,
		tenantID, p.ConfirmationID, p.VersionNo).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return actionDomain.ErrNotFound
	}
	if err := insertConfirmationEventRow(ctx, tx, tenantID, p.ConfirmationID, p.VersionNo,
		p.CaseID, p.EventID, p.Kind, p.Reason, p.Details, p.ActorID, p.Now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
