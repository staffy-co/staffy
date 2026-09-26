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

func TestDomainPackPinsIsolationImpactAndEvidence(t *testing.T) {
	ctx := context.Background()
	identityA, ownerA := newService("00000000f001", "00000000f002", "00000000f003")
	tenantA := createTenant(t, identityA, ownerA, "Pack auto parts tenant", "pack-tenant-a", "00000000f004").Tenant.ID
	identityB, ownerB := newService("00000000f011", "00000000f012", "00000000f013")
	tenantB := createTenant(t, identityB, ownerB, "Pack Montagio tenant", "pack-tenant-b", "00000000f014").Tenant.ID
	corr := id.MustParse("01994f64-6c00-7000-8000-00000000f020")
	svcA := employeeApp.New(employeeStore.NewPostgres(runtimePool), identityA, fixedClock{time.Now().UTC()}, employeeIDs{})
	svcB := employeeApp.New(employeeStore.NewPostgres(runtimePool), identityB, fixedClock{time.Now().UTC()}, employeeIDs{})
	a, err := svcA.Create(ctx, employeeAPI.CreateCommand{Actor: ownerA, TenantID: tenantA, IdempotencyKey: "pack-employee-a", CorrelationID: corr})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svcB.Create(ctx, employeeAPI.CreateCommand{Actor: ownerB, TenantID: tenantB, IdempotencyKey: "pack-employee-b", CorrelationID: corr})
	if err != nil {
		t.Fatal(err)
	}
	cmdA := employeeAPI.PinCommand{Actor: ownerA, TenantID: tenantA, EmployeeID: a.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: 1, PackID: "auto-parts", PackVersion: "1.0.0", Variants: map[string]any{"parts_interest": "both"}}
	pinned, err := svcA.Pin(ctx, cmdA)
	if err != nil || pinned.Version != 2 || pinned.Readiness != "incomplete" {
		t.Fatalf("initial pin: %+v %v", pinned, err)
	}
	cmdB := employeeAPI.PinCommand{Actor: ownerB, TenantID: tenantB, EmployeeID: b.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: 1, PackID: "montagio-services", PackVersion: "1.0.0", Variants: map[string]any{"engineering_plumbing": true, "heating": false}}
	if _, err = svcB.Pin(ctx, cmdB); err != nil {
		t.Fatal(err)
	}
	cmdA.ExpectedVersion = 2
	cmdA.Variants = map[string]any{"parts_interest": "both", "policy_ref": "unsafe"}
	if _, err = svcA.Pin(ctx, cmdA); !errors.Is(err, employeeDomain.ErrInvalid) {
		t.Fatalf("protected variant accepted: %v", err)
	}
	current, err := svcA.Read(ctx, ownerA, tenantA, a.Draft.EmployeeID)
	if err != nil || current.Pack.Version != "1.0.0" || current.Version != 2 {
		t.Fatalf("invalid write changed pin: %+v %v", current, err)
	}
	cmdA.PackVersion = "2.0.0"
	cmdA.Variants = map[string]any{"parts_interest": "both", "matching_depth": float64(2)}
	preview, err := svcA.Preview(ctx, cmdA)
	if err != nil || preview.Current.Version != "1.0.0" || preview.Impact.Compatibility != "incompatible" {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if _, err = svcA.Pin(ctx, cmdA); !errors.Is(err, employeeDomain.ErrInvalid) {
		t.Fatalf("unconfirmed upgrade: %v", err)
	}
	cmdA.Confirmed = true
	upgraded, err := svcA.Pin(ctx, cmdA)
	if err != nil || upgraded.Version != 3 || upgraded.Pack.Version != "2.0.0" || len(upgraded.ReviewSlots) == 0 {
		t.Fatalf("upgrade: %+v %v", upgraded, err)
	}
	if _, err = svcA.Pin(ctx, cmdA); !errors.Is(err, employeeDomain.ErrConflict) {
		t.Fatalf("stale version: %v", err)
	}
	cmdA.TenantID = tenantB
	cmdA.ExpectedVersion = 3
	if _, err = svcA.Pin(ctx, cmdA); !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("foreign tenant: %v", err)
	}
	if _, err = svcB.Read(ctx, ownerB, tenantB, a.Draft.EmployeeID); !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("cross-tenant read: %v", err)
	}
	var auditCount, eventCount int
	if err = adminPool.QueryRow(ctx, "select count(*) from employee.audit where tenant_id=$1 and employee_id=$2 and outcome='pack_pinned'", tenantA, a.Draft.EmployeeID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err = adminPool.QueryRow(ctx, "select count(*) from employee.outbox_events where tenant_id=$1 and employee_id=$2 and event_type='employee.domain_pack_pinned' and schema_version=1 and correlation_id=$3", tenantA, a.Draft.EmployeeID, corr).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 2 || eventCount != 2 {
		t.Fatalf("audit=%d outbox=%d", auditCount, eventCount)
	}
	for _, tc := range []struct {
		tenant   id.UUID
		expected int
	}{{tenantA, 1}, {tenantB, 1}} {
		tx, e := runtimePool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(ctx, "select set_config('staffy.tenant_id',$1,true)", tc.tenant.String())
		if e != nil {
			t.Fatal(e)
		}
		var count int
		e = tx.QueryRow(ctx, "select count(*) from employee.drafts where domain_pack is not null").Scan(&count)
		if e != nil {
			t.Fatal(e)
		}
		if count != tc.expected {
			t.Fatalf("RLS tenant %s count=%d", tc.tenant, count)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
	}
}
