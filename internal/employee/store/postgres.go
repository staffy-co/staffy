package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

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

func scanDraft(row pgx.Row) (domain.Draft, error) {
	var d domain.Draft
	var brief, pack, slots []byte
	err := row.Scan(&d.TenantID, &d.EmployeeID, &d.OwnerID, &d.Status, &d.Readiness, &d.Version, &d.CurrentStep, &brief, &pack, &slots, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Draft{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Draft{}, err
	}
	if err := json.Unmarshal(brief, &d.Brief); err != nil {
		return domain.Draft{}, err
	}
	if len(pack) > 0 {
		if err := json.Unmarshal(pack, &d.Pack); err != nil {
			return domain.Draft{}, err
		}
	}
	if err := json.Unmarshal(slots, &d.ReviewSlots); err != nil {
		return domain.Draft{}, err
	}
	return d, nil
}

const draftColumns = "tenant_id,employee_id,owner_id,status,readiness,version,current_step,brief,domain_pack,review_slots,created_at,updated_at"

func (s *Postgres) Create(ctx context.Context, p port.CreateParams) (domain.Draft, bool, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.Draft{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	lock := p.Scope.TenantID().String() + ":" + p.Scope.PrincipalID().String() + ":employee.create.v1:" + p.Key
	if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", lock); err != nil {
		return domain.Draft{}, false, err
	}
	var oldHash string
	var oldID id.UUID
	err = tx.QueryRow(ctx, "select request_hash,employee_id from employee.idempotency_records where tenant_id=$1 and actor_id=$2 and operation='employee.create.v1' and idempotency_key=$3", p.Scope.TenantID(), p.Scope.PrincipalID(), p.Key).Scan(&oldHash, &oldID)
	if err == nil {
		if oldHash != p.Hash {
			var currentVersion int64
			if err = tx.QueryRow(ctx, "select version from employee.drafts where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), oldID).Scan(&currentVersion); err != nil {
				return domain.Draft{}, false, err
			}
			if _, err = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,0,$5,'conflict',$6,$7)", p.Scope.TenantID(), p.EventID, oldID, p.Scope.PrincipalID(), currentVersion, p.CorrelationID, p.Now); err != nil {
				return domain.Draft{}, false, err
			}
			if err = tx.Commit(ctx); err != nil {
				return domain.Draft{}, false, err
			}
			return domain.Draft{}, false, domain.ErrIdempotency
		}
		d, e := scanDraft(tx.QueryRow(ctx, "select "+draftColumns+" from employee.drafts where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), oldID))
		if e != nil {
			return domain.Draft{}, false, e
		}
		return d, true, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Draft{}, false, err
	}
	initialBrief, _ := json.Marshal(domain.Brief{Name: p.Name})
	if _, err = tx.Exec(ctx, "insert into employee.drafts(tenant_id,employee_id,owner_id,brief,created_at,updated_at) values($1,$2,$3,$4,$5,$5)", p.Scope.TenantID(), p.EmployeeID, p.Scope.PrincipalID(), initialBrief, p.Now); err != nil {
		return domain.Draft{}, false, err
	}
	if _, err = tx.Exec(ctx, "insert into employee.idempotency_records(tenant_id,actor_id,operation,idempotency_key,request_hash,employee_id,created_at) values($1,$2,'employee.create.v1',$3,$4,$5,$6)", p.Scope.TenantID(), p.Scope.PrincipalID(), p.Key, p.Hash, p.EmployeeID, p.Now); err != nil {
		return domain.Draft{}, false, err
	}
	if _, err = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,0,1,'created',$5,$6)", p.Scope.TenantID(), p.EventID, p.EmployeeID, p.Scope.PrincipalID(), p.CorrelationID, p.Now); err != nil {
		return domain.Draft{}, false, err
	}
	if _, err = tx.Exec(ctx, "insert into employee.outbox_events(tenant_id,event_id,employee_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'employee.draft_created',1,1,$4,$5,$6,$7)", p.Scope.TenantID(), p.EventID, p.EmployeeID, p.Scope.PrincipalID(), p.CorrelationID, p.Now, []byte(fmt.Sprintf(`{"employee_id":%q,"version":1}`, p.EmployeeID.String()))); err != nil {
		return domain.Draft{}, false, err
	}
	d, e := scanDraft(tx.QueryRow(ctx, "select "+draftColumns+" from employee.drafts where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID))
	if e != nil {
		return domain.Draft{}, false, e
	}
	return d, false, tx.Commit(ctx)
}

func (s *Postgres) List(ctx context.Context, scope tenancy.Context) ([]domain.Draft, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, "select "+draftColumns+" from employee.drafts where tenant_id=$1 order by updated_at desc,employee_id", scope.TenantID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []domain.Draft{}
	for rows.Next() {
		d, e := scanDraft(rows)
		if e != nil {
			return nil, e
		}
		list = append(list, d)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, tx.Commit(ctx)
}
func (s *Postgres) Read(ctx context.Context, scope tenancy.Context, employeeID id.UUID) (domain.Draft, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return domain.Draft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	d, err := scanDraft(tx.QueryRow(ctx, "select "+draftColumns+" from employee.drafts where tenant_id=$1 and employee_id=$2", scope.TenantID(), employeeID))
	if err != nil {
		return domain.Draft{}, err
	}
	return d, tx.Commit(ctx)
}
func (s *Postgres) Save(ctx context.Context, p port.SaveParams) (domain.Draft, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.Draft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	brief, err := json.Marshal(p.Brief)
	if err != nil {
		return domain.Draft{}, err
	}
	d, err := scanDraft(tx.QueryRow(ctx, "update employee.drafts set brief=$3,current_step='launch_plan',version=version+1,updated_at=$4 where tenant_id=$1 and employee_id=$2 and version=$5 returning "+draftColumns, p.Scope.TenantID(), p.EmployeeID, brief, p.Now, p.ExpectedVersion))
	if errors.Is(err, domain.ErrNotFound) {
		var current int64
		readErr := tx.QueryRow(ctx, "select version from employee.drafts where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&current)
		if errors.Is(readErr, pgx.ErrNoRows) {
			return domain.Draft{}, domain.ErrNotFound
		}
		if readErr != nil {
			return domain.Draft{}, readErr
		}
		_, err = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'conflict',$7,$8)", p.Scope.TenantID(), p.AuditID, p.EmployeeID, p.Scope.PrincipalID(), p.ExpectedVersion, current, p.CorrelationID, p.Now)
		if err != nil {
			return domain.Draft{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return domain.Draft{}, err
		}
		return domain.Draft{}, domain.ErrConflict
	}
	if err != nil {
		return domain.Draft{}, err
	}
	if _, err = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'saved',$7,$8)", p.Scope.TenantID(), p.AuditID, p.EmployeeID, p.Scope.PrincipalID(), p.ExpectedVersion, d.Version, p.CorrelationID, p.Now); err != nil {
		return domain.Draft{}, err
	}
	if _, err = tx.Exec(ctx, "insert into employee.outbox_events(tenant_id,event_id,employee_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'employee.brief_saved',1,$4,$5,$6,$7,$8)", p.Scope.TenantID(), p.EventID, p.EmployeeID, d.Version, p.Scope.PrincipalID(), p.CorrelationID, p.Now, []byte(fmt.Sprintf(`{"employee_id":%q,"version":%d}`, p.EmployeeID.String(), d.Version))); err != nil {
		return domain.Draft{}, err
	}
	return d, tx.Commit(ctx)
}

func (s *Postgres) Pin(ctx context.Context, p port.PinParams) (domain.Draft, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.Draft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	pack, err := json.Marshal(p.Pin)
	if err != nil {
		return domain.Draft{}, err
	}
	slots, err := json.Marshal(p.ReviewSlots)
	if err != nil {
		return domain.Draft{}, err
	}
	d, err := scanDraft(tx.QueryRow(ctx, "update employee.drafts set domain_pack=$3,review_slots=$4,version=version+1,updated_at=$5 where tenant_id=$1 and employee_id=$2 and version=$6 returning "+draftColumns, p.Scope.TenantID(), p.EmployeeID, pack, slots, p.Now, p.ExpectedVersion))
	if errors.Is(err, domain.ErrNotFound) {
		var current int64
		readErr := tx.QueryRow(ctx, "select version from employee.drafts where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&current)
		if errors.Is(readErr, pgx.ErrNoRows) {
			return domain.Draft{}, domain.ErrNotFound
		}
		if readErr != nil {
			return domain.Draft{}, readErr
		}
		_, err = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'conflict',$7,$8)", p.Scope.TenantID(), p.AuditID, p.EmployeeID, p.Scope.PrincipalID(), p.ExpectedVersion, current, p.CorrelationID, p.Now)
		if err != nil {
			return domain.Draft{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return domain.Draft{}, err
		}
		return domain.Draft{}, domain.ErrConflict
	}
	if err != nil {
		return domain.Draft{}, err
	}
	_, err = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'pack_pinned',$7,$8)", p.Scope.TenantID(), p.AuditID, p.EmployeeID, p.Scope.PrincipalID(), p.ExpectedVersion, d.Version, p.CorrelationID, p.Now)
	if err != nil {
		return domain.Draft{}, err
	}
	payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "version": d.Version, "pack_id": p.Pin.ID, "pack_version": p.Pin.Version, "review_slots": p.ReviewSlots})
	_, err = tx.Exec(ctx, "insert into employee.outbox_events(tenant_id,event_id,employee_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'employee.domain_pack_pinned',1,$4,$5,$6,$7,$8)", p.Scope.TenantID(), p.EventID, p.EmployeeID, d.Version, p.Scope.PrincipalID(), p.CorrelationID, p.Now, payload)
	if err != nil {
		return domain.Draft{}, err
	}
	return d, tx.Commit(ctx)
}
