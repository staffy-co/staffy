package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/artemiosu/staffy/internal/control/domain"
	"github.com/artemiosu/staffy/internal/control/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is the control persistence: every table is tenant-scoped with
// RLS; Set and Resume are each ONE transaction (the advisory lock of the
// material identity, SELECT FOR UPDATE of the scope pair, the row write,
// the append-only audit row and the outbox event — AD-13 atomicity); the
// CONFLICT attempt commits its own audit row and returns the current
// material (AC-11); the reads canonicalize timestamps to UTC at the
// boundary (the R2/N1 lesson of the cases module).
type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (s *Postgres) begin(ctx context.Context, scope tenancy.Context, readonly bool) (pgx.Tx, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	opts := pgx.TxOptions{}
	if readonly {
		opts.AccessMode = pgx.ReadOnly
	}
	tx, err := s.pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	for _, pair := range [][2]string{{"staffy.principal_id", scope.PrincipalID().String()}, {"staffy.tenant_id", scope.TenantID().String()}} {
		if _, err := tx.Exec(ctx, "select set_config($1,$2,true)", pair[0], pair[1]); err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
	}
	return tx, nil
}

// controlColumns deliberately omits tenant_id: the tenancy context (and
// the RLS policy behind it) already fixes the tenant of every row — the
// domain record needs no tenant field, the surface never crosses tenants.
const controlColumns = "scope_kind,scope_ref,mode,reason,actor_id,prev_mode,generation,scheduled_start_at,expires_at,state,updated_at"

type rowScanner interface{ Scan(dest ...any) error }

func scanControl(row rowScanner) (domain.ScopedControl, error) {
	var c domain.ScopedControl
	// The mode value is scanned RAW: a corrupted stored word is the reading
	// surface's problem — the domain answers it as paused (fail-closed
	// reading, never an expanded allowance); the list row keeps the honest
	// damage material for the page marker.
	err := row.Scan(&c.ScopeKind, &c.ScopeRef, &c.Mode, &c.Reason, &c.ActorID, &c.PrevMode, &c.Generation, &c.ScheduledStartAt, &c.ExpiresAt, &c.State, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ScopedControl{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ScopedControl{}, err
	}
	c.ScheduledStartAt = utcOpt(c.ScheduledStartAt)
	c.ExpiresAt = utcOpt(c.ExpiresAt)
	c.UpdatedAt = c.UpdatedAt.UTC().Truncate(time.Microsecond)
	return c, nil
}

func utcOpt(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	canonical := t.UTC().Truncate(time.Microsecond)
	return &canonical
}

// scopePair is the material identity of one control point.
type scopePair struct {
	tenantID, kind, ref string
}

// lockKey materializes the advisory lock identity of one control point:
// the SAME key serializes Set AND Resume of the pair. The lock closes the
// creation race of two concurrent first installations BEFORE the SELECT
// FOR UPDATE (the employee Create precedent): the loser then finds the
// committed row, writes its conflict audit row and returns the material.
func (p scopePair) lockKey() string {
	return "control.mutation.v1:" + p.tenantID + ":" + p.kind + ":" + p.ref
}

// outboxPayload is the versioned event material: the scope pair, the
// resulting mode, the resulting state and the new generation. The event
// carries the FACTS of the change — never a snapshot of the effective
// computation (that stays a pure read; expiry materializes nothing).
func outboxPayload(c domain.ScopedControl) []byte {
	payload, _ := json.Marshal(map[string]any{
		"scope_kind": c.ScopeKind,
		"scope_ref":  c.ScopeRef,
		"mode":       c.Mode,
		"state":      c.State,
		"generation": c.Generation,
	})
	return payload
}

// appendAudit inserts the append-only audit row inside the running
// transaction. requestedMode is the operator's requested dictionary word
// of a set — and the honest EMPTY word of a resume (a lift requests no
// mode); prevMode is the material mode before the change; outcome is
// 'applied' or 'conflict' — BOTH are history (AC-11).
func appendAudit(ctx context.Context, tx pgx.Tx, pair scopePair, actor string, auditID id.UUID, action, requestedMode, prevMode, reason, outcome string, expected, resulting int64, correlationID id.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, "insert into control.audit(tenant_id,audit_id,scope_kind,scope_ref,actor_id,action,requested_mode,prev_mode,expected_generation,resulting_generation,outcome,reason,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)",
		pair.tenantID, auditID.String(), pair.kind, pair.ref, actor, action, requestedMode, prevMode, expected, resulting, outcome, reason, correlationID.String(), now)
	return err
}

// appendOutbox inserts the AD-13 transactional outbox event inside the
// running transaction. event_type is the single word of the dictionary;
// aggregate_version IS the new generation; schema_version is 1 (the first
// contract of the event shape).
func appendOutbox(ctx context.Context, tx pgx.Tx, pair scopePair, c domain.ScopedControl, eventID, correlationID id.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, "insert into control.outbox_events(tenant_id,event_id,scope_kind,scope_ref,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,$4,'scoped_control.changed',1,$5,$6,$7,$8,$9)",
		pair.tenantID, eventID.String(), pair.kind, pair.ref, c.Generation, c.ActorID, correlationID.String(), now, outboxPayload(c))
	return err
}

// Set atomically installs or re-installs the control point:
//
//   - the advisory lock of the pair serializes concurrent mutations;
//   - SELECT FOR UPDATE pins the existing row (or the honest absence);
//   - a missing point with ExpectedGeneration 0 is the FIRST installation
//     (generation 1, prev_mode ”);
//   - a missing point with a non-zero expectation is the honest
//     ErrNotFound — there is no material to confirm over, and no audit
//     row: the FK of the audit table honestly refuses a history row of a
//     point that never existed;
//   - a generation mismatch COMMITS the 'conflict' audit row and returns
//     ConflictError with the CURRENT material (AC-11);
//   - the applied path updates the row (generation+1, the new mode and
//     window, state 'active' — a re-installation of a removed point
//     continues the monotonic series) and writes the applied audit row
//     plus the outbox event in the SAME transaction.
func (s *Postgres) Set(ctx context.Context, p port.SetParams) (domain.ScopedControl, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.ScopedControl{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	pair := scopePair{p.Scope.TenantID().String(), p.Kind, p.Ref}
	actor := p.Scope.PrincipalID().String()
	if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", pair.lockKey()); err != nil {
		return domain.ScopedControl{}, err
	}

	current, err := scanControl(tx.QueryRow(ctx, "select "+controlColumns+" from control.scoped_controls where tenant_id=$1 and scope_kind=$2 and scope_ref=$3 for update", p.Scope.TenantID(), p.Kind, p.Ref))
	if errors.Is(err, domain.ErrNotFound) {
		if p.ExpectedGeneration != 0 {
			// Nothing to confirm over: the honest 404 (no audit row — the
			// point never existed, the FK keeps the history truthful).
			return domain.ScopedControl{}, domain.ErrNotFound
		}
		// The tenant_id column is written EXPLICITLY from the tenancy scope: the
		// row is NOT NULL and the RLS WITH CHECK compares it against the tenant
		// config of the transaction â the live matrix of step-04 caught the honest
		// omission of the unit steps (the fake repos never exercised the INSERT
		// against the real PG).
		created, cerr := scanControl(tx.QueryRow(ctx, "insert into control.scoped_controls(tenant_id,scope_kind,scope_ref,mode,reason,actor_id,prev_mode,generation,scheduled_start_at,expires_at,state,updated_at) values($1,$2,$3,$4,$5,$6,'',1,$7,$8,'active',$9) returning "+controlColumns,
			p.Scope.TenantID(), p.Kind, p.Ref, p.Mode, p.Reason, actor, p.ScheduledStartAt, p.ExpiresAt, now))
		if cerr != nil {
			return domain.ScopedControl{}, cerr
		}
		if err = appendAudit(ctx, tx, pair, actor, p.AuditID, "set", p.Mode, "", p.Reason, "applied", 0, created.Generation, p.CorrelationID, now); err != nil {
			return domain.ScopedControl{}, err
		}
		if err = appendOutbox(ctx, tx, pair, created, p.EventID, p.CorrelationID, now); err != nil {
			return domain.ScopedControl{}, err
		}
		return created, tx.Commit(ctx)
	}
	if err != nil {
		return domain.ScopedControl{}, err
	}

	if current.Generation != p.ExpectedGeneration {
		if err = appendAudit(ctx, tx, pair, actor, p.AuditID, "set", p.Mode, current.Mode, p.Reason, "conflict", p.ExpectedGeneration, current.Generation, p.CorrelationID, now); err != nil {
			return domain.ScopedControl{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return domain.ScopedControl{}, err
		}
		return domain.ScopedControl{}, domain.ConflictError{ScopeKind: p.Kind, ScopeRef: p.Ref, Expected: p.ExpectedGeneration, Current: current}
	}

	updated, uerr := scanControl(tx.QueryRow(ctx, "update control.scoped_controls set mode=$4,reason=$5,actor_id=$6,prev_mode=$7,generation=generation+1,scheduled_start_at=$8,expires_at=$9,state='active',updated_at=$10 where tenant_id=$1 and scope_kind=$2 and scope_ref=$3 returning "+controlColumns,
		p.Scope.TenantID(), p.Kind, p.Ref, p.Mode, p.Reason, actor, current.Mode, p.ScheduledStartAt, p.ExpiresAt, now))
	if uerr != nil {
		return domain.ScopedControl{}, uerr
	}
	if err = appendAudit(ctx, tx, pair, actor, p.AuditID, "set", p.Mode, current.Mode, p.Reason, "applied", p.ExpectedGeneration, updated.Generation, p.CorrelationID, now); err != nil {
		return domain.ScopedControl{}, err
	}
	if err = appendOutbox(ctx, tx, pair, updated, p.EventID, p.CorrelationID, now); err != nil {
		return domain.ScopedControl{}, err
	}
	return updated, tx.Commit(ctx)
}

// Resume atomically lifts the control point: the row survives (state
// 'removed'), the generation grows, the mode column keeps the lifted
// word as the honest history material. Lifting a point that does not
// exist is the honest ErrNotFound; lifting a point that is already
// removed is a RuleError — repeating a removal is not a state change and
// does not pretend to be one. The conflict contract mirrors Set: the
// refused attempt is a committed audit row, the current material returns.
func (s *Postgres) Resume(ctx context.Context, p port.ResumeParams) (domain.ScopedControl, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.ScopedControl{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	pair := scopePair{p.Scope.TenantID().String(), p.Kind, p.Ref}
	actor := p.Scope.PrincipalID().String()
	if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", pair.lockKey()); err != nil {
		return domain.ScopedControl{}, err
	}

	current, err := scanControl(tx.QueryRow(ctx, "select "+controlColumns+" from control.scoped_controls where tenant_id=$1 and scope_kind=$2 and scope_ref=$3 for update", p.Scope.TenantID(), p.Kind, p.Ref))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ScopedControl{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ScopedControl{}, err
	}

	if current.Generation != p.ExpectedGeneration {
		if err = appendAudit(ctx, tx, pair, actor, p.AuditID, "resume", "", current.Mode, p.Reason, "conflict", p.ExpectedGeneration, current.Generation, p.CorrelationID, now); err != nil {
			return domain.ScopedControl{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return domain.ScopedControl{}, err
		}
		return domain.ScopedControl{}, domain.ConflictError{ScopeKind: p.Kind, ScopeRef: p.Ref, Expected: p.ExpectedGeneration, Current: current}
	}

	if current.State == domain.ControlStateRemoved {
		// The honest refusal of a no-op lift: the state does not change, so
		// the generation must not pretend it did. No audit row — a refused
		// no-op is not an attempt worth a history line (nothing was
		// attempted over the material, the row already says removed).
		return domain.ScopedControl{}, domain.RuleError{Rule: "state", Reason: "точка контроля уже снята: повторное снятие не меняет состояние"}
	}

	updated, uerr := scanControl(tx.QueryRow(ctx, "update control.scoped_controls set reason=$4,actor_id=$5,prev_mode=$6,generation=generation+1,state='removed',updated_at=$7 where tenant_id=$1 and scope_kind=$2 and scope_ref=$3 returning "+controlColumns,
		p.Scope.TenantID(), p.Kind, p.Ref, p.Reason, actor, current.Mode, now))
	if uerr != nil {
		return domain.ScopedControl{}, uerr
	}
	if err = appendAudit(ctx, tx, pair, actor, p.AuditID, "resume", "", current.Mode, p.Reason, "applied", p.ExpectedGeneration, updated.Generation, p.CorrelationID, now); err != nil {
		return domain.ScopedControl{}, err
	}
	if err = appendOutbox(ctx, tx, pair, updated, p.EventID, p.CorrelationID, now); err != nil {
		return domain.ScopedControl{}, err
	}
	return updated, tx.Commit(ctx)
}

// List returns every control point of the tenant — BOTH states: the
// removed points stay readable (the history surface), newest first with
// the deterministic (kind, ref) tie-break.
func (s *Postgres) List(ctx context.Context, scope tenancy.Context) ([]domain.ScopedControl, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, "select "+controlColumns+" from control.scoped_controls where tenant_id=$1 order by updated_at desc, scope_kind, scope_ref", scope.TenantID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []domain.ScopedControl{}
	for rows.Next() {
		c, scanErr := scanControl(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		list = append(list, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, tx.Commit(ctx)
}

// LoadKeys returns the control points matching the route scope keys (the
// EffectiveFor read): ONE query over the pairs, deterministic (kind, ref)
// order. The caller has already validated the dictionary discipline of
// every key and collapsed the duplicates; an empty key set is the honest
// empty read (absent controls mean accept — the route is unrestricted).
func (s *Postgres) LoadKeys(ctx context.Context, scope tenancy.Context, keys []domain.ScopeKey) ([]domain.ScopedControl, error) {
	if len(keys) == 0 {
		return []domain.ScopedControl{}, nil
	}
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	args := make([]any, 0, 1+2*len(keys))
	args = append(args, scope.TenantID())
	tuples := make([]string, 0, len(keys))
	for _, key := range keys {
		tuples = append(tuples, fmt.Sprintf("($%d,$%d)", len(args)+1, len(args)+2))
		args = append(args, key.Kind, key.Ref)
	}
	rows, err := tx.Query(ctx, "select "+controlColumns+" from control.scoped_controls where tenant_id=$1 and (scope_kind,scope_ref) in ("+strings.Join(tuples, ",")+") order by scope_kind, scope_ref", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []domain.ScopedControl{}
	for rows.Next() {
		c, scanErr := scanControl(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		list = append(list, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, tx.Commit(ctx)
}
