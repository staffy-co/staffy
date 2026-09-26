//go:build integration

package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeApp "github.com/artemiosu/staffy/internal/employee/app"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	employeeStore "github.com/artemiosu/staffy/internal/employee/store"
	"github.com/artemiosu/staffy/internal/platform/id"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestKnowledgeLifecycleIsolationVersionsAndEvidence(t *testing.T) {
	ctx := context.Background()
	identity, owner := newService("00000000a101", "00000000a102", "00000000a103")
	tenant := createTenant(t, identity, owner, "Knowledge tenant", "knowledge-tenant-a", "00000000a104").Tenant.ID
	otherIdentity, otherOwner := newService("00000000a111", "00000000a112", "00000000a113")
	otherTenant := createTenant(t, otherIdentity, otherOwner, "Other knowledge tenant", "knowledge-tenant-b", "00000000a114").Tenant.ID
	corr := id.MustParse("01994f64-6c00-7000-8000-00000000a120")
	svc := employeeApp.New(employeeStore.NewPostgres(runtimePool), identity, fixedClock{time.Now().UTC()}, employeeIDs{})
	employee, err := svc.Create(ctx, employeeAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "knowledge-employee", CorrelationID: corr})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Pin(ctx, employeeAPI.PinCommand{Actor: owner, TenantID: tenant, EmployeeID: employee.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: 1, PackID: "auto-parts", PackVersion: "1.0.0", Variants: map[string]any{"parts_interest": "both"}})
	if err != nil {
		t.Fatal(err)
	}
	created, err := svc.CreateKnowledge(ctx, employeeAPI.KnowledgeCreateCommand{Actor: owner, TenantID: tenant, EmployeeID: employee.Draft.EmployeeID, CorrelationID: corr, Source: employeeDomain.KnowledgeSource{Type: "faq", Title: "Вопросы", Owner: "Менеджер", Provenance: "Внутренний FAQ", Scope: "Общие ответы", Priority: 50, Language: "ru"}})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "processing" || created.Version != 1 || created.CurrentVersion != 0 {
		t.Fatalf("created: %+v", created)
	}
	var keyLength int
	if err = adminPool.QueryRow(ctx, "select octet_length(checksum_key) from employee.knowledge_sources where tenant_id=$1 and source_id=$2", tenant, created.SourceID).Scan(&keyLength); err != nil || keyLength != 32 {
		t.Fatalf("source HMAC key length=%d err=%v", keyLength, err)
	}
	encoded, _ := json.Marshal(created)
	if created.ChecksumKey != nil || strings.Contains(string(encoded), "checksum_key") {
		t.Fatal("checksum key leaked in source response")
	}
	process := employeeAPI.KnowledgeProcessCommand{Actor: owner, TenantID: tenant, EmployeeID: employee.Draft.EmployeeID, SourceID: created.SourceID, CorrelationID: corr, ExpectedVersion: 1, Text: "Да"}
	source, version, replayed, err := svc.ProcessKnowledge(ctx, process)
	if err != nil || replayed || source.Status != "ready" || version.Version != 1 || len(version.Chunks) == 0 {
		t.Fatalf("processed: %+v %+v %v %v", source, version, replayed, err)
	}
	raw := sha256.Sum256([]byte(process.Text))
	if version.Checksum == hex.EncodeToString(raw[:]) || version.Chunks[0].Checksum == hex.EncodeToString(raw[:]) {
		t.Fatal("raw hash of short customer text exposed")
	}
	process.Text = "https://example.com"
	if _, _, _, err = svc.ProcessKnowledge(ctx, process); !errors.Is(err, employeeDomain.ErrInvalid) {
		t.Fatalf("URL accepted: %v", err)
	}
	process.Text = "<html>unsafe</html>"
	if _, _, _, err = svc.ProcessKnowledge(ctx, process); !errors.Is(err, employeeDomain.ErrInvalid) {
		t.Fatalf("HTML accepted: %v", err)
	}
	process.Text = "Да"
	restarted := employeeApp.New(employeeStore.NewPostgres(runtimePool), identity, fixedClock{time.Now().UTC()}, employeeIDs{})
	_, replayVersion, replayed, err := restarted.ProcessKnowledge(ctx, process)
	if err != nil || !replayed {
		t.Fatalf("checksum replay: %v %v", replayed, err)
	}
	_, persisted, err := restarted.ReadKnowledge(ctx, owner, tenant, employee.Draft.EmployeeID, created.SourceID)
	if err != nil || len(persisted) != 1 || !reflect.DeepEqual(replayVersion, persisted[0]) {
		t.Fatalf("replay did not return persisted immutable version: %+v %+v %v", replayVersion, persisted, err)
	}
	process.ExpectedVersion = source.Version
	process.Text = "Как сделать заказ? Напишите менеджеру."
	source, version, replayed, err = svc.ProcessKnowledge(ctx, process)
	if err != nil || replayed || version.Version != 2 {
		t.Fatalf("version change: %+v %v", version, err)
	}
	fetched, versions, err := svc.ReadKnowledge(ctx, owner, tenant, employee.Draft.EmployeeID, created.SourceID)
	if err != nil || fetched.CurrentVersion != 2 || len(versions) != 2 || versions[0].Text == versions[1].Text || versions[0].Chunks[0].Checksum == "" {
		t.Fatalf("preview: %+v %+v %v", fetched, versions, err)
	}
	if fetched.Provenance != "Внутренний FAQ" || versions[0].Chunks[0].SourceID != created.SourceID || versions[0].Chunks[0].Version != 2 {
		t.Fatalf("provenance/chunk identity lost: %+v %+v", fetched, versions[0].Chunks[0])
	}
	_, _, err = svc.ReadKnowledge(ctx, owner, otherTenant, employee.Draft.EmployeeID, created.SourceID)
	if !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("foreign tenant read: %v", err)
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
	if err = tx.QueryRow(ctx, "select count(*) from employee.knowledge_sources where source_id=$1", created.SourceID).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("RLS exposed foreign source: %d", visible)
	}
	conflict, err := svc.ConflictKnowledge(ctx, employeeAPI.KnowledgeConflictCommand{Actor: owner, TenantID: tenant, EmployeeID: employee.Draft.EmployeeID, SourceID: created.SourceID, CorrelationID: corr, ExpectedVersion: source.Version, Note: "Другое правило", Provenance: "Сообщение владельца"})
	if err != nil || conflict.EffectiveStatus(time.Now()) != "conflict" {
		t.Fatalf("conflict: %+v %v", conflict, err)
	}
	retired, err := svc.RetireKnowledge(ctx, employeeAPI.KnowledgeRetireCommand{Actor: owner, TenantID: tenant, EmployeeID: employee.Draft.EmployeeID, SourceID: created.SourceID, CorrelationID: corr, ExpectedVersion: conflict.Version})
	if err != nil || retired.Status != "retired" {
		t.Fatalf("retire: %+v %v", retired, err)
	}
	var audit, event int
	err = adminPool.QueryRow(ctx, "select count(*) from employee.knowledge_audit where tenant_id=$1 and source_id=$2", tenant, created.SourceID).Scan(&audit)
	if err != nil {
		t.Fatal(err)
	}
	err = adminPool.QueryRow(ctx, "select count(*) from employee.knowledge_outbox where tenant_id=$1 and source_id=$2 and correlation_id=$3", tenant, created.SourceID, corr).Scan(&event)
	if err != nil {
		t.Fatal(err)
	}
	if audit != 5 || event != 5 {
		t.Fatalf("audit=%d event=%d", audit, event)
	}
	var leaked int
	if err = adminPool.QueryRow(ctx, "select count(*) from employee.knowledge_outbox where tenant_id=$1 and source_id=$2 and payload::text like '%checksum_key%'", tenant, created.SourceID).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("HMAC key leaked to outbox: %d %v", leaked, err)
	}
}
