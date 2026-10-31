package store

import (
	"context"
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
// Story 4.9 (step-02): the supplier order fact persistence — the case
// reads, the operation reads and the ONE locked idempotent append
// ---------------------------------------------------------------------------

// The structural pin: the store satisfies the consumer-owned repository
// contract of the fact contour (the OrderCorrectionRepository precedent).
var _ actionPort.OrderSupplierFactRepository = (*Postgres)(nil)

// orderSupplierFactLockKey is the advisory lock key of one case's fact
// ladder — its OWN key space (the "action.order_supplier_fact:" prefix
// never mixes with the correction/verification ones): the fact_no
// ladder and the idempotency BY KEY serialize per case, the raced
// replay of the same fact command reads the saved row under the lock
// and answers the honest duplicate signal — never a second row, never
// a masked write.
func orderSupplierFactLockKey(tenantID, caseID id.UUID) string {
	return "action.order_supplier_fact.v1:" + tenantID.String() + ":" + caseID.String()
}

// mapOrderSupplierFactInsertError maps the raced insert conflicts onto
// the honest consumer-owned signals: the fact PK is the per-case ladder
// backstop, the UNIQUE full fact key is the IDEMPOTENCY backstop (the
// replay of the same command — AC-9), the CHECK dictionaries are the
// loyal domain-mirror refusals. The store never masks a lost race as a
// write.
func mapOrderSupplierFactInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		return actionPort.ErrOrderSupplierFactDuplicate
	}
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23514" {
		return actionPort.ErrOrderSupplierFactDuplicate
	}
	return err
}

// orderSupplierFactColumns is the canonical column list of the fact row
// (the SELECT and the INSERT share it — the scan target list mirrors
// it).
const orderSupplierFactColumns = `tenant_id, case_id, work_item_id, fact_no, operation_id,
 kind, supplier_order_id, happened_at, actor_id, recorded_at`

// readOrderSupplierFacts reads the fact rows of one anchor (the case or
// the operation) ordered by the monotonic fact number (the fold order —
// the server truth of the ladder).
func readOrderSupplierFacts(ctx context.Context, tx pgx.Tx, tenantID id.UUID, where string, anchor id.UUID) ([]actionDomain.OrderSupplierFact, error) {
	rows, err := tx.Query(ctx, `select `+orderSupplierFactColumns+`
		from action.order_supplier_facts where tenant_id=$1 and `+where+` order by fact_no`,
		tenantID, anchor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	facts := []actionDomain.OrderSupplierFact{}
	for rows.Next() {
		var fact actionDomain.OrderSupplierFact
		if err := rows.Scan(&fact.TenantID, &fact.CaseID, &fact.WorkItemID, &fact.FactNo,
			&fact.OperationID, &fact.Kind, &fact.SupplierOrderID, &fact.HappenedAt,
			&fact.ActorID, &fact.RecordedAt); err != nil {
			return nil, err
		}
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}

// ReadCaseSupplierFacts reads every fact of one case ordered by the
// monotonic fact number (the projection material of the panel).
func (s *Postgres) ReadCaseSupplierFacts(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionDomain.OrderSupplierFact, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tenantID := scope.TenantID()
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	facts, err := readOrderSupplierFacts(ctx, tx, tenantID, "case_id=$2", caseID)
	if err != nil {
		return nil, err
	}
	return facts, tx.Commit(ctx)
}

// ReadOperationSupplierFacts reads every fact anchored to one operation
// ordered by the monotonic fact number (the guard input of the
// sent-after-formed ladder).
func (s *Postgres) ReadOperationSupplierFacts(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]actionDomain.OrderSupplierFact, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tenantID := scope.TenantID()
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	facts, err := readOrderSupplierFacts(ctx, tx, tenantID, "operation_id=$2", operationID)
	if err != nil {
		return nil, err
	}
	return facts, tx.Commit(ctx)
}

// AppendOrderSupplierFact appends ONE fact under the per-case advisory
// lock: the monotonic fact_no derives under the lock (the ladder of the
// repeated records — the history is never rewritten), the UNIQUE full
// fact key (tenant, case, operation, kind, supplier_order_id)
// backstops the IDEMPOTENCY BY KEY — the raced replay of the same
// command refuses with ErrOrderSupplierFactDuplicate and NEVER writes
// a second row (the app re-reads the saved fact and answers the honest
// replay outcome — AC-9). The recorded_at is the SERVER time of the
// append; the happened_at is the manager's NAMED time — the two times
// never mix. The append is a pure persistence boundary: nothing is
// emitted anywhere (the OQ-3 fence — the fact records the human
// reality, it never performs it).
func (s *Postgres) AppendOrderSupplierFact(ctx context.Context, p actionPort.AppendOrderSupplierFactParams) (actionDomain.OrderSupplierFact, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return actionDomain.OrderSupplierFact{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		orderSupplierFactLockKey(tenantID, p.CaseID)); err != nil {
		return actionDomain.OrderSupplierFact{}, err
	}
	// The per-case ladder number UNDER the lock (the store truth — the
	// fact never carries it).
	var factNo int64
	if err := tx.QueryRow(ctx, `
		select coalesce(max(fact_no), 0) + 1 from action.order_supplier_facts
		where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&factNo); err != nil {
		return actionDomain.OrderSupplierFact{}, err
	}
	recordedAt := p.Now.UTC().Truncate(time.Microsecond)
	fact := actionDomain.OrderSupplierFact{
		TenantID:        tenantID,
		CaseID:          p.CaseID,
		WorkItemID:      p.WorkItemID,
		FactNo:          factNo,
		OperationID:     p.OperationID,
		Kind:            p.Kind,
		SupplierOrderID: p.SupplierOrderID,
		HappenedAt:      p.HappenedAt.UTC().Truncate(time.Microsecond),
		ActorID:         p.ActorID,
		RecordedAt:      recordedAt,
	}
	if _, err := tx.Exec(ctx, `
		insert into action.order_supplier_facts (tenant_id, case_id, work_item_id, fact_no,
			operation_id, kind, supplier_order_id, happened_at, actor_id, recorded_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		fact.TenantID, fact.CaseID, fact.WorkItemID, fact.FactNo,
		fact.OperationID, fact.Kind, fact.SupplierOrderID, fact.HappenedAt,
		fact.ActorID, fact.RecordedAt); err != nil {
		return actionDomain.OrderSupplierFact{}, mapOrderSupplierFactInsertError(err)
	}
	return fact, tx.Commit(ctx)
}
