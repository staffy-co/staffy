//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeApp "github.com/artemiosu/staffy/internal/employee/app"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	employeeStore "github.com/artemiosu/staffy/internal/employee/store"
	"github.com/artemiosu/staffy/internal/platform/id"
)

func TestBehaviorVersionsCASAuditAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	identity, owner := newService("00000000b101", "00000000b102", "00000000b103")
	tenant := createTenant(t, identity, owner, "Behavior tenant", "behavior-tenant-a", "00000000b104").Tenant.ID
	otherIdentity, otherOwner := newService("00000000b111", "00000000b112", "00000000b113")
	otherTenant := createTenant(t, otherIdentity, otherOwner, "Other behavior tenant", "behavior-tenant-b", "00000000b114").Tenant.ID
	corr := id.MustParse("01994f64-6c00-7000-8000-00000000b120")
	svc := employeeApp.New(employeeStore.NewPostgres(runtimePool), identity, fixedClock{time.Now().UTC()}, employeeIDs{})
	created, err := svc.Create(ctx, employeeAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "behavior-employee", CorrelationID: corr})
	if err != nil {
		t.Fatal(err)
	}
	pin, err := svc.Pin(ctx, employeeAPI.PinCommand{Actor: owner, TenantID: tenant, EmployeeID: created.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: 1, PackID: "auto-parts", PackVersion: "1.0.0", Variants: map[string]any{"parts_interest": "both"}})
	if err != nil {
		t.Fatal(err)
	}
	cmd := employeeAPI.BehaviorCommand{Actor: owner, TenantID: tenant, EmployeeID: created.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: pin.Version, ChangeSummary: "Первичная настройка", Config: employeeDomain.Behavior{Role: "Консультант", Purpose: "Общие ответы", AIDisclosure: true, Tone: "neutral", Modes: []string{"answer"}, Confirmation: "required", Handoff: "required", Claims: []employeeDomain.AllowedClaim{{Text: "Есть консультация", Owner: "Владелец", Scope: "Общий ответ", Channels: []string{"web_chat"}, Modes: []string{"answer"}}}}}
	preview, err := svc.PreviewBehavior(ctx, cmd)
	if err != nil || preview.Claims[0].Status != "blocked" || preview.Publishable {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	v, err := svc.SaveBehavior(ctx, cmd)
	if err != nil || v.Version != 1 || v.DraftVersion != pin.Version+1 || v.Config.Claims[0].Status != "blocked" {
		t.Fatalf("save=%+v err=%v", v, err)
	}
	if _, err = svc.SaveBehavior(ctx, cmd); !errors.Is(err, employeeDomain.ErrConflict) {
		t.Fatalf("stale save=%v", err)
	}
	versions, err := svc.ReadBehavior(ctx, owner, tenant, created.Draft.EmployeeID)
	if err != nil || len(versions) != 1 || versions[0].ActorID != owner.ID || versions[0].ChangeSummary != "Первичная настройка" {
		t.Fatalf("versions=%+v err=%v", versions, err)
	}
	if _, err = svc.ReadBehavior(ctx, otherOwner, otherTenant, created.Draft.EmployeeID); !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("foreign read=%v", err)
	}
	var audit, event, stored int
	if err = adminPool.QueryRow(ctx, "select count(*) from employee.audit where tenant_id=$1 and employee_id=$2 and outcome='behavior_saved'", tenant, created.Draft.EmployeeID).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	if err = adminPool.QueryRow(ctx, "select count(*) from employee.outbox_events where tenant_id=$1 and employee_id=$2 and event_type='employee.behavior_saved'", tenant, created.Draft.EmployeeID).Scan(&event); err != nil {
		t.Fatal(err)
	}
	if err = adminPool.QueryRow(ctx, "select count(*) from employee.behavior_versions where tenant_id=$1 and employee_id=$2", tenant, created.Draft.EmployeeID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if audit != 1 || event != 1 || stored != 1 {
		t.Fatalf("audit=%d event=%d versions=%d", audit, event, stored)
	}
	tx, err := runtimePool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "select set_config('staffy.tenant_id',$1,true)", otherTenant.String()); err != nil {
		t.Fatal(err)
	}
	var visible int
	if err = tx.QueryRow(ctx, "select count(*) from employee.behavior_versions where employee_id=$1", created.Draft.EmployeeID).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("RLS leaked %d versions", visible)
	}
}
