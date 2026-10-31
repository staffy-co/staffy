package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
)

func (s *Postgres) ReadBehavior(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.BehaviorVersion, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, "select tenant_id,employee_id,version,draft_version,pack_id,pack_version,config,actor_id,changed_at,change_summary from employee.behavior_versions where tenant_id=$1 and employee_id=$2 order by version desc", scope.TenantID(), employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := []domain.BehaviorVersion{}
	for rows.Next() {
		var v domain.BehaviorVersion
		var config []byte
		if err = rows.Scan(&v.TenantID, &v.EmployeeID, &v.Version, &v.DraftVersion, &v.PackID, &v.PackVersion, &config, &v.ActorID, &v.ChangedAt, &v.ChangeSummary); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(config, &v.Config); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return versions, tx.Commit(ctx)
}

func (s *Postgres) SaveBehavior(ctx context.Context, p port.BehaviorSaveParams) (domain.BehaviorVersion, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var draftVersion int64
	var packJSON []byte
	err = tx.QueryRow(ctx, "update employee.drafts set version=version+1,updated_at=$3 where tenant_id=$1 and employee_id=$2 and version=$4 and domain_pack is not null returning version,domain_pack", p.Scope.TenantID(), p.EmployeeID, p.Now, p.ExpectedVersion).Scan(&draftVersion, &packJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		var current int64
		e := tx.QueryRow(ctx, "select version from employee.drafts where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&current)
		if errors.Is(e, pgx.ErrNoRows) {
			return domain.BehaviorVersion{}, domain.ErrNotFound
		}
		if e != nil {
			return domain.BehaviorVersion{}, e
		}
		_, e = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'conflict',$7,$8)", p.Scope.TenantID(), p.AuditID, p.EmployeeID, p.Scope.PrincipalID(), p.ExpectedVersion, current, p.CorrelationID, p.Now)
		if e != nil {
			return domain.BehaviorVersion{}, e
		}
		if e = tx.Commit(ctx); e != nil {
			return domain.BehaviorVersion{}, e
		}
		return domain.BehaviorVersion{}, domain.ErrConflict
	}
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	var pin domain.PackPin
	if err = json.Unmarshal(packJSON, &pin); err != nil {
		return domain.BehaviorVersion{}, err
	}
	var version int64
	err = tx.QueryRow(ctx, "select coalesce(max(version),0)+1 from employee.behavior_versions where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&version)
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	config, err := json.Marshal(p.Config)
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	v := domain.BehaviorVersion{TenantID: p.Scope.TenantID(), EmployeeID: p.EmployeeID, Version: version, DraftVersion: draftVersion, PackID: pin.ID, PackVersion: pin.Version, Config: p.Config, ActorID: p.Scope.PrincipalID(), ChangedAt: p.Now, ChangeSummary: p.Summary}
	_, err = tx.Exec(ctx, "insert into employee.behavior_versions(tenant_id,employee_id,version,draft_version,pack_id,pack_version,config,actor_id,changed_at,change_summary) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", v.TenantID, v.EmployeeID, v.Version, v.DraftVersion, v.PackID, v.PackVersion, config, v.ActorID, v.ChangedAt, v.ChangeSummary)
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	_, err = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'behavior_saved',$7,$8)", v.TenantID, p.AuditID, p.EmployeeID, v.ActorID, p.ExpectedVersion, draftVersion, p.CorrelationID, p.Now)
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "draft_version": draftVersion, "behavior_version": version, "change_summary": p.Summary})
	_, err = tx.Exec(ctx, "insert into employee.outbox_events(tenant_id,event_id,employee_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'employee.behavior_saved',1,$4,$5,$6,$7,$8)", v.TenantID, p.EventID, p.EmployeeID, draftVersion, v.ActorID, p.CorrelationID, p.Now, payload)
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	return v, tx.Commit(ctx)
}
