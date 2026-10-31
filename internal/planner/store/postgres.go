package store

// The PostgreSQL turn ledger of the planner (Story 8-1): the append-only
// journal behind the tenant RLS. The runtime role (staffy_app,
// NOBYPASSRLS) holds SELECT and INSERT only — the append-only is the
// grant shape, not a convention. The uniqueness (tenant_id, turn_id)
// surfaces as the honest ErrTurnConflict: the second recording of the
// same turn does not happen.

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/staffy-co/staffy/internal/planner"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Postgres is the journal store over the runtime pool.
type Postgres struct{ pool *pgxpool.Pool }

// NewPostgres wires the store.
func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

// begin opens a transaction with the tenant scope of the RLS policies
// (the platform discipline of the module stores).
func (s *Postgres) begin(ctx context.Context, scope tenancy.Context) (pgx.Tx, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	for _, pair := range [][2]string{
		{"staffy.principal_id", scope.PrincipalID().String()},
		{"staffy.tenant_id", scope.TenantID().String()},
	} {
		if _, err := tx.Exec(ctx, "select set_config($1,$2,true)", pair[0], pair[1]); err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
	}
	return tx, nil
}

// Record appends one turn record behind the tenant RLS. The uniqueness
// violation of (tenant_id, turn_id) is the honest ErrTurnConflict.
func (s *Postgres) Record(ctx context.Context, scope tenancy.Context, record planner.TurnRecord) error {
	tx, err := s.begin(ctx, scope)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// The absent material is NULL (the honest SQL shape of absence):
	// a validated turn carries no refusal cause and no fallback form,
	// and the CHECK dictionaries evaluate NULL as absent.
	var proposal, refusalReason, fallbackForm any
	if len(record.Proposal) > 0 {
		proposal = record.Proposal
	}
	if record.RefusalReason != "" {
		refusalReason = record.RefusalReason
	}
	if record.FallbackForm != "" {
		fallbackForm = record.FallbackForm
	}
	_, err = tx.Exec(ctx, `insert into planner.turns (
		tenant_id, turn_id, case_id, conversation_id, work_item_id, employee_id,
		outcome, refusal_reason, fallback_form, schema_version, build, proposal, created_at
	) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		record.TenantID, record.TurnID, record.CaseID, record.ConversationID, record.WorkItemID, record.EmployeeID,
		record.Outcome, refusalReason, fallbackForm, record.SchemaVersion, record.Build, proposal, record.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return planner.ErrTurnConflict
		}
		return err
	}
	return tx.Commit(ctx)
}

// Turn reads one turn record behind the tenant RLS: the record of
// another tenant is structurally absent (ErrTurnNotFound — the view
// never discloses whether the turn exists elsewhere, AD-11).
func (s *Postgres) Turn(ctx context.Context, scope tenancy.Context, turnID id.UUID) (planner.TurnRecord, error) {
	tx, err := s.begin(ctx, scope)
	if err != nil {
		return planner.TurnRecord{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var record planner.TurnRecord
	var refusalReason, fallbackForm *string
	var proposal []byte
	err = tx.QueryRow(ctx, `select tenant_id, turn_id, case_id, conversation_id, work_item_id, employee_id,
		outcome, refusal_reason, fallback_form, schema_version, build, proposal, created_at
		from planner.turns where tenant_id=$1 and turn_id=$2`, scope.TenantID(), turnID).
		Scan(&record.TenantID, &record.TurnID, &record.CaseID, &record.ConversationID, &record.WorkItemID, &record.EmployeeID,
			&record.Outcome, &refusalReason, &fallbackForm, &record.SchemaVersion, &record.Build, &proposal, &record.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return planner.TurnRecord{}, planner.ErrTurnNotFound
	}
	if err != nil {
		return planner.TurnRecord{}, err
	}
	if refusalReason != nil {
		record.RefusalReason = *refusalReason
	}
	if fallbackForm != nil {
		record.FallbackForm = *fallbackForm
	}
	record.Proposal = proposal
	return record, tx.Commit(ctx)
}

// isUniqueViolation reports the PostgreSQL unique-violation code of
// the honest turn conflict (the action-store discipline of the
// ErrIdempotencyConflict precedent).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}
