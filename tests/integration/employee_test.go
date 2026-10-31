//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeApp "github.com/staffy-co/staffy/internal/employee/app"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	employeeStore "github.com/staffy-co/staffy/internal/employee/store"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

type employeeIDs struct{}

func (employeeIDs) New(now time.Time) (id.UUID, error) { return id.NewV7(now) }

func TestEmployeeDraftReplaySaveConflictAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	identity, actor := newService("00000000d001", "00000000d002", "00000000d003")
	tenant := createTenant(t, identity, actor, "Employee test tenant", "employee-tenant-key", "00000000d004").Tenant.ID
	ids := &sequenceIDs{values: []id.UUID{
		id.MustParse("01994f64-6c00-7000-8000-00000000d005"), id.MustParse("01994f64-6c00-7000-8000-00000000d006"),
		id.MustParse("01994f64-6c00-7000-8000-00000000d007"), id.MustParse("01994f64-6c00-7000-8000-00000000d008"),
		id.MustParse("01994f64-6c00-7000-8000-00000000d009"), id.MustParse("01994f64-6c00-7000-8000-00000000d00a"),
		id.MustParse("01994f64-6c00-7000-8000-00000000d00b"), id.MustParse("01994f64-6c00-7000-8000-00000000d00c"),
	}}
	svc := employeeApp.New(employeeStore.NewPostgres(runtimePool), identity, fixedClock{time.Now().UTC()}, ids)
	corr := id.MustParse("01994f64-6c00-7000-8000-00000000d00d")
	created, err := svc.Create(ctx, employeeAPI.CreateCommand{Actor: actor, TenantID: tenant, IdempotencyKey: "employee-draft-key", CorrelationID: corr})
	if err != nil {
		t.Fatal(err)
	}
	if created.Draft.Status != "draft" || created.Draft.Readiness != "incomplete" || created.Draft.Version != 1 || created.Draft.OwnerID != actor.ID {
		t.Fatalf("unexpected draft: %+v", created.Draft)
	}
	replayed, err := svc.Create(ctx, employeeAPI.CreateCommand{Actor: actor, TenantID: tenant, IdempotencyKey: "employee-draft-key", CorrelationID: corr})
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || replayed.Draft.EmployeeID != created.Draft.EmployeeID {
		t.Fatal("idempotency replay changed employee")
	}
	otherName, err := employeeApp.New(employeeStore.NewPostgres(runtimePool), identity, fixedClock{time.Now().UTC()}, employeeIDs{}).Create(ctx, employeeAPI.CreateCommand{Actor: actor, TenantID: tenant, Name: "Другое имя", IdempotencyKey: "employee-draft-key", CorrelationID: corr})
	if !errors.Is(err, employeeDomain.ErrIdempotency) || otherName.Draft.EmployeeID.Valid() {
		t.Fatalf("different payload replayed: %+v, %v", otherName, err)
	}
	var replayConflictVersion int64
	var replayConflictActor, replayConflictCorrelation id.UUID
	var replayConflictTime time.Time
	if err = adminPool.QueryRow(ctx, "select resulting_version,actor_id,correlation_id,occurred_at from employee.audit where tenant_id=$1 and employee_id=$2 and outcome='conflict' and attempted_version=0", tenant, created.Draft.EmployeeID).Scan(&replayConflictVersion, &replayConflictActor, &replayConflictCorrelation, &replayConflictTime); err != nil {
		t.Fatal(err)
	}
	if replayConflictVersion != 1 || replayConflictActor != actor.ID || replayConflictCorrelation != corr || replayConflictTime.IsZero() {
		t.Fatalf("missing idempotency conflict evidence: %d %s %s %s", replayConflictVersion, replayConflictActor, replayConflictCorrelation, replayConflictTime)
	}
	brief := employeeDomain.Brief{Purpose: "Помочь гостям", Name: "Аня", Languages: []string{"ru"}, TimeZone: "Europe/Moscow", WorkingHours: employeeDomain.WorkingHours{Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00"}, DesiredChannels: []string{"phone"}, PrimaryTasks: []string{"Запись"}}
	invalid := brief
	invalid.WorkingHours.End = "08:00"
	if _, err = svc.Save(ctx, employeeAPI.SaveCommand{Actor: actor, TenantID: tenant, EmployeeID: created.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: 1, Brief: invalid}); !errors.Is(err, employeeDomain.ErrInvalid) {
		t.Fatalf("invalid Brief accepted: %v", err)
	}
	saved, err := svc.Save(ctx, employeeAPI.SaveCommand{Actor: actor, TenantID: tenant, EmployeeID: created.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: 1, Brief: brief})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != 2 || saved.Brief.Name != "Аня" || saved.CurrentStep != "launch_plan" {
		t.Fatalf("save failed: %+v", saved)
	}
	_, err = svc.Save(ctx, employeeAPI.SaveCommand{Actor: actor, TenantID: tenant, EmployeeID: created.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: 1, Brief: brief})
	if !errors.Is(err, employeeDomain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	read, err := svc.Read(ctx, actor, tenant, created.Draft.EmployeeID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Version != 2 || read.Brief.Name != "Аня" {
		t.Fatal("conflict overwrote saved brief")
	}
	var auditCount, eventCount int
	if err := adminPool.QueryRow(ctx, "select count(*) from employee.audit where tenant_id=$1 and employee_id=$2", tenant, created.Draft.EmployeeID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := adminPool.QueryRow(ctx, "select count(*) from employee.outbox_events where tenant_id=$1 and employee_id=$2", tenant, created.Draft.EmployeeID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 4 || eventCount != 2 {
		t.Fatalf("audit=%d events=%d", auditCount, eventCount)
	}
	rows, err := adminPool.Query(ctx, "select event_type,schema_version,aggregate_version,actor_id,correlation_id,payload from employee.outbox_events where tenant_id=$1 and employee_id=$2 order by aggregate_version", tenant, created.Draft.EmployeeID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var typ string
		var schema int
		var version int64
		var actorID, correlationID id.UUID
		var payload []byte
		if err = rows.Scan(&typ, &schema, &version, &actorID, &correlationID, &payload); err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if err = json.Unmarshal(payload, &body); err != nil {
			t.Fatal(err)
		}
		if schema != 1 || actorID != actor.ID || correlationID != corr || body["employee_id"] != created.Draft.EmployeeID.String() || body["version"] != float64(version) {
			t.Fatalf("bad outbox metadata: %s %d %d %v", typ, schema, version, body)
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	otherTenant := id.MustParse("01994f64-6c00-7000-8000-00000000d00e")
	if _, err = svc.Read(ctx, actor, otherTenant, created.Draft.EmployeeID); !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("cross-tenant read: %v", err)
	}
}

func TestEmployeePermissionsConcurrentSaveAndPoolScope(t *testing.T) {
	ctx := context.Background()
	identity, owner := newService("00000000e001", "00000000e002", "00000000e003")
	tenant := createTenant(t, identity, owner, "Employee permissions tenant", "employee-perm-key", "00000000e004").Tenant.ID
	config := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000e005")}
	operator := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000e006")}
	for _, member := range []struct {
		actor       identityDomain.Principal
		role, scope string
	}{{config, "configurator", "configuration"}, {operator, "operator", "operations"}} {
		if _, err := adminPool.Exec(ctx, "insert into identity.principals(principal_id,created_at) values($1,now())", member.actor.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := adminPool.Exec(ctx, "insert into identity.memberships(tenant_id,principal_id,role,status,created_at,roles,scopes) values($1,$2,$3,'active',now(),array[$3],array[$4])", tenant, member.actor.ID, member.role, member.scope); err != nil {
			t.Fatal(err)
		}
	}
	svc := employeeApp.New(employeeStore.NewPostgres(runtimePool), identity, fixedClock{time.Now().UTC()}, employeeIDs{})
	corr := id.MustParse("01994f64-6c00-7000-8000-00000000e007")
	if _, err := svc.Create(ctx, employeeAPI.CreateCommand{Actor: operator, TenantID: tenant, IdempotencyKey: "operator-denied", CorrelationID: corr}); !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("operator create: %v", err)
	}
	created, err := svc.Create(ctx, employeeAPI.CreateCommand{Actor: config, TenantID: tenant, IdempotencyKey: "config-create-key", CorrelationID: corr})
	if err != nil {
		t.Fatal(err)
	}
	if created.Draft.OwnerID != config.ID {
		t.Fatal("wrong owner")
	}
	brief := employeeDomain.Brief{Purpose: "Помощь", Name: "Тест", Languages: []string{"ru"}, TimeZone: "Europe/Moscow", WorkingHours: employeeDomain.WorkingHours{Days: []string{"mon"}, Start: "09:00", End: "18:00"}, PrimaryTasks: []string{"Запись"}}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := svc.Save(ctx, employeeAPI.SaveCommand{Actor: config, TenantID: tenant, EmployeeID: created.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: 1, Brief: brief})
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, employeeDomain.ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("save results %d %d", success, conflict)
	}
	var attempts int
	if err = adminPool.QueryRow(ctx, "select count(*) from employee.audit where tenant_id=$1 and employee_id=$2 and outcome in ('saved','conflict')", tenant, created.Draft.EmployeeID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("audit attempts=%d", attempts)
	}
	if _, err = svc.Read(ctx, operator, tenant, created.Draft.EmployeeID); !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("operator read: %v", err)
	}
	otherIdentity, otherOwner := newService("00000000e011", "00000000e012", "00000000e013")
	otherTenant := createTenant(t, otherIdentity, otherOwner, "Other employee tenant", "other-employee-key", "00000000e014").Tenant.ID
	if _, err = svc.Read(ctx, config, otherTenant, created.Draft.EmployeeID); !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("foreign read: %v", err)
	}
	if _, err = svc.Save(ctx, employeeAPI.SaveCommand{Actor: otherOwner, TenantID: otherTenant, EmployeeID: created.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: 2, Brief: brief}); !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("foreign save: %v", err)
	}
	for _, tenantScope := range []id.UUID{tenant, otherTenant, tenant} {
		tx, e := runtimePool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(ctx, "select set_config('staffy.tenant_id',$1,true)", tenantScope.String()); e != nil {
			t.Fatal(e)
		}
		var count int
		if e = tx.QueryRow(ctx, "select count(*) from employee.drafts").Scan(&count); e != nil {
			t.Fatal(e)
		}
		expected := 0
		if tenantScope == tenant {
			expected = 1
		}
		if count != expected {
			t.Fatalf("RLS count tenant %s: %d", tenantScope, count)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
	}
}
