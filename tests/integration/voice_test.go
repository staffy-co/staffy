//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeApp "github.com/staffy-co/staffy/internal/employee/app"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	employeeStore "github.com/staffy-co/staffy/internal/employee/store"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

func TestVoiceVersionsCASAuditRLSAndUnavailablePreview(t *testing.T) {
	ctx := context.Background()
	identity, owner := newService("00000000c101", "00000000c102", "00000000c103")
	tenant := createTenant(t, identity, owner, "Voice tenant", "voice-tenant-a", "00000000c104").Tenant.ID
	otherIdentity, otherOwner := newService("00000000c111", "00000000c112", "00000000c113")
	otherTenant := createTenant(t, otherIdentity, otherOwner, "Other voice tenant", "voice-tenant-b", "00000000c114").Tenant.ID
	corr := id.MustParse("01994f64-6c00-7000-8000-00000000c120")
	svc := employeeApp.New(employeeStore.NewPostgres(runtimePool), identity, fixedClock{time.Now().UTC()}, employeeIDs{})
	created, err := svc.Create(ctx, employeeAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "voice-employee", CorrelationID: corr})
	if err != nil {
		t.Fatal(err)
	}
	brief := employeeDomain.Brief{Purpose: "Тест", Name: "Аня", Languages: []string{"ru"}, TimeZone: "Europe/Moscow", WorkingHours: employeeDomain.WorkingHours{Days: []string{"mon"}, Start: "09:00", End: "18:00"}, DesiredChannels: []string{"phone"}, PrimaryTasks: []string{"Ответ"}}
	d, err := svc.Save(ctx, employeeAPI.SaveCommand{Actor: owner, TenantID: tenant, EmployeeID: created.Draft.EmployeeID, CorrelationID: corr, ExpectedVersion: created.Draft.Version, Brief: brief})
	if err != nil {
		t.Fatal(err)
	}
	cmd := employeeAPI.VoiceCommand{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, CorrelationID: corr, ExpectedVersion: d.Version, ChangeSummary: "Первый голос", Config: employeeDomain.VoiceConfig{ProfileID: "ru-neutral-1", Language: "ru", Style: "neutral", Tempo: 100, Dictionary: []employeeDomain.Pronunciation{{Written: "VIN", Language: "ru", Scope: "Тест", Normalized: "вин", TestPhrase: "Прочтите VIN", SupportStatus: "unverified", CriticalKind: "vin", SyntheticDataConfirmed: true, Readback: &employeeDomain.ReadbackPlan{Segments: []string{"V", "I", "N"}, Confirmation: "ask_customer", OnMismatch: "human_handoff", PhoneQualityGate: "pending_human"}}}}}
	v, err := svc.SaveVoice(ctx, cmd)
	if err != nil || v.Version != 1 {
		t.Fatalf("save=%+v err=%v", v, err)
	}
	if _, err = svc.SaveVoice(ctx, cmd); !errors.Is(err, employeeDomain.ErrConflict) {
		t.Fatalf("stale CAS=%v", err)
	}
	if _, err = svc.PreviewVoice(ctx, employeeAPI.VoicePreviewRequest{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, VoiceVersion: 1, Text: "Прочтите VIN"}); !errors.Is(err, employeeDomain.ErrInvalid) {
		t.Fatalf("preview without synthetic-data confirmation=%v", err)
	}
	preview, err := svc.PreviewVoice(ctx, employeeAPI.VoicePreviewRequest{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, VoiceVersion: 1, Text: "Прочтите VIN", SyntheticDataConfirmed: true})
	if err == nil || len(preview.Audio) != 0 || preview.PhoneQualityApproved {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	var unassessedPreviewID, approvedPreviewID id.UUID
	if binary := os.Getenv("STAFFY_TEST_ESPEAK_NG_BIN"); binary != "" {
		withTTS := employeeApp.NewWithTestTTS(employeeStore.NewPostgres(runtimePool), identity, fixedClock{time.Now().UTC()}, employeeIDs{}, employeeApp.ESpeakNG{Binary: binary, DataParent: os.Getenv("STAFFY_TEST_ESPEAK_DATA_PATH"), LibraryPath: os.Getenv("STAFFY_TEST_ESPEAK_LIBRARY_PATH")})
		profiles, err := withTTS.VoiceCatalog(ctx, owner, tenant)
		if err != nil || profiles[0].PreviewStatus != "available" {
			t.Fatalf("catalog=%+v err=%v", profiles, err)
		}
		preview, err := withTTS.PreviewVoice(ctx, employeeAPI.VoicePreviewRequest{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, VoiceVersion: 1, Text: "Прочтите VIN", SyntheticDataConfirmed: true, Slow: true})
		if err != nil || preview.Capability != "available" || preview.AudioType != "audio/wav" || preview.Text != "Прочтите VIN" || preview.SynthesisText != "Прочтите вин" || preview.DictionaryVersion != 1 || !preview.PreviewID.Valid() || preview.PhoneQualityApproved || len(preview.Audio) < 44 || !bytes.Equal(preview.Audio[:4], []byte("RIFF")) {
			t.Fatalf("positive preview=%+v err=%v", preview, err)
		}
		approvedPreviewID = preview.PreviewID
		assessment, err := withTTS.AssessVoice(ctx, employeeAPI.VoiceAssessmentCommand{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, PreviewID: preview.PreviewID, CorrelationID: corr, VoiceVersion: 1, Verdict: "approved"})
		if err != nil || assessment.Text != "Прочтите VIN" || assessment.SynthesisText != "Прочтите вин" || assessment.DictionaryVersion != 1 || assessment.PhoneQualityApproved {
			t.Fatalf("assessment=%+v err=%v", assessment, err)
		}
		if _, err = withTTS.AssessVoice(ctx, employeeAPI.VoiceAssessmentCommand{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, PreviewID: preview.PreviewID, CorrelationID: corr, VoiceVersion: 1, Verdict: "needs_revision"}); !errors.Is(err, employeeDomain.ErrConflict) {
			t.Fatalf("duplicate assessment=%v", err)
		}
		unassessed, err := withTTS.PreviewVoice(ctx, employeeAPI.VoicePreviewRequest{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, VoiceVersion: 1, Text: "Прочтите VIN", SyntheticDataConfirmed: true})
		if err != nil {
			t.Fatal(err)
		}
		unassessedPreviewID = unassessed.PreviewID
		expired, err := withTTS.PreviewVoice(ctx, employeeAPI.VoicePreviewRequest{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, VoiceVersion: 1, Text: "Прочтите VIN", SyntheticDataConfirmed: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = adminPool.Exec(ctx, "update employee.voice_previews set created_at=now()-interval '25 hours' where tenant_id=$1 and preview_id=$2", tenant, expired.PreviewID); err != nil {
			t.Fatal(err)
		}
		if _, err = withTTS.AssessVoice(ctx, employeeAPI.VoiceAssessmentCommand{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, PreviewID: expired.PreviewID, CorrelationID: corr, VoiceVersion: 1, Verdict: "approved"}); !errors.Is(err, employeeDomain.ErrConflict) {
			t.Fatalf("expired receipt assessed: %v", err)
		}
		if _, err = runtimePool.Exec(ctx, "select * from employee.purge_expired_voice_previews()"); err == nil {
			t.Fatal("app role can purge")
		}
		maintenance, err := adminPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = maintenance.Rollback(ctx) }()
		if _, err = maintenance.Exec(ctx, "set local role staffy_voice_maintenance"); err != nil {
			t.Fatal(err)
		}
		var removedAssessments, removedPreviews int64
		if err = maintenance.QueryRow(ctx, "select * from employee.purge_expired_voice_previews()").Scan(&removedAssessments, &removedPreviews); err != nil {
			t.Fatal(err)
		}
		if removedPreviews != 1 || removedAssessments != 0 {
			t.Fatalf("purge counts=%d/%d", removedAssessments, removedPreviews)
		}
		if err = maintenance.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		assessments, err := withTTS.ReadVoiceAssessments(ctx, owner, tenant, d.EmployeeID)
		if err != nil || len(assessments) != 1 || assessments[0].Stale {
			t.Fatalf("assessments=%+v err=%v", assessments, err)
		}
	}
	cmd.ExpectedVersion = v.DraftVersion
	cmd.ChangeSummary = "Новый темп"
	cmd.Config.Tempo = 90
	v2, err := svc.SaveVoice(ctx, cmd)
	if err != nil || v2.Version != 2 {
		t.Fatalf("second save=%+v err=%v", v2, err)
	}
	if _, err = svc.PreviewVoice(ctx, employeeAPI.VoicePreviewRequest{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, VoiceVersion: 1, Text: "Прочтите VIN", SyntheticDataConfirmed: true}); !errors.Is(err, employeeDomain.ErrConflict) {
		t.Fatalf("stale preview=%v", err)
	}
	if unassessedPreviewID.Valid() {
		if _, err = svc.AssessVoice(ctx, employeeAPI.VoiceAssessmentCommand{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, PreviewID: unassessedPreviewID, CorrelationID: corr, VoiceVersion: 1, Verdict: "approved"}); !errors.Is(err, employeeDomain.ErrConflict) {
			t.Fatalf("stale assessment accepted: %v", err)
		}
		assessments, err := svc.ReadVoiceAssessments(ctx, owner, tenant, d.EmployeeID)
		if err != nil || len(assessments) != 1 || !assessments[0].Stale {
			t.Fatalf("stale status=%+v err=%v", assessments, err)
		}
		if _, err = svc.ReadVoiceAssessments(ctx, otherOwner, otherTenant, d.EmployeeID); !errors.Is(err, employeeDomain.ErrNotFound) {
			t.Fatalf("assessment tenant leak=%v", err)
		}
	}
	versions, err := svc.ReadVoice(ctx, owner, tenant, d.EmployeeID)
	if err != nil || len(versions) != 2 || versions[0].ActorID != owner.ID || versions[1].Config.Tempo != 100 || versions[1].Version != 1 {
		t.Fatalf("versions=%+v err=%v", versions, err)
	}
	if _, err = svc.ReadVoice(ctx, otherOwner, otherTenant, d.EmployeeID); !errors.Is(err, employeeDomain.ErrNotFound) {
		t.Fatalf("tenant leak=%v", err)
	}
	if _, err = svc.Save(ctx, employeeAPI.SaveCommand{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, CorrelationID: corr, ExpectedVersion: v2.DraftVersion, Brief: brief}); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.PreviewVoice(ctx, employeeAPI.VoicePreviewRequest{Actor: owner, TenantID: tenant, EmployeeID: d.EmployeeID, VoiceVersion: 2, Text: "Прочтите VIN", SyntheticDataConfirmed: true}); !errors.Is(err, employeeDomain.ErrConflict) {
		t.Fatalf("stale draft preview=%v", err)
	}
	scope, _ := tenancy.New(owner.ID, tenant)
	stalePreviewID, _ := id.NewV7(time.Now().UTC())
	err = employeeStore.NewPostgres(runtimePool).SaveVoicePreview(ctx, scope, employeeDomain.VoicePreviewRecord{TenantID: tenant, EmployeeID: d.EmployeeID, PreviewID: stalePreviewID, VoiceVersion: 2, DictionaryVersion: 2, DraftVersion: v2.DraftVersion, Text: "Прочтите VIN", SynthesisText: "Прочтите вин", Route: "offline-test-tts", ActorID: owner.ID, CreatedAt: time.Now().UTC()})
	if !errors.Is(err, employeeDomain.ErrConflict) {
		t.Fatalf("stale receipt persisted: %v", err)
	}
	var audits, events int
	if err = adminPool.QueryRow(ctx, "select count(*) from employee.audit where tenant_id=$1 and employee_id=$2 and outcome='voice_saved'", tenant, d.EmployeeID).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err = adminPool.QueryRow(ctx, "select count(*) from employee.outbox_events where tenant_id=$1 and employee_id=$2 and event_type='employee.voice_saved'", tenant, d.EmployeeID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if audits != 2 || events != 2 {
		t.Fatalf("audit=%d events=%d", audits, events)
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
	if err = tx.QueryRow(ctx, "select count(*) from employee.voice_versions where employee_id=$1", d.EmployeeID).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("RLS leaked %d", visible)
	}
	if err = tx.QueryRow(ctx, "select count(*) from employee.voice_assessments where employee_id=$1", d.EmployeeID).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("assessment RLS leaked %d", visible)
	}
	if approvedPreviewID.Valid() {
		if _, err = adminPool.Exec(ctx, "update employee.voice_previews set created_at=now()-interval '25 hours' where tenant_id=$1 and preview_id=$2", tenant, approvedPreviewID); err != nil {
			t.Fatal(err)
		}
		purgeTx, err := adminPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = purgeTx.Rollback(ctx) }()
		if _, err = purgeTx.Exec(ctx, "set local role staffy_voice_maintenance"); err != nil {
			t.Fatal(err)
		}
		if _, err = purgeTx.Exec(ctx, "select set_config('staffy.tenant_id',$1,true)", otherTenant.String()); err != nil {
			t.Fatal(err)
		}
		var removedAssessments, removedPreviews int64
		if err = purgeTx.QueryRow(ctx, "select * from employee.purge_expired_voice_previews()").Scan(&removedAssessments, &removedPreviews); err != nil {
			t.Fatal(err)
		}
		if removedAssessments != 1 || removedPreviews != 1 {
			t.Fatalf("purge with assessment=%d/%d", removedAssessments, removedPreviews)
		}
		if err = purgeTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err = adminPool.QueryRow(ctx, "select count(*) from employee.voice_previews where tenant_id=$1 and preview_id=$2", tenant, approvedPreviewID).Scan(&visible); err != nil || visible != 0 {
			t.Fatalf("preview retained=%d err=%v", visible, err)
		}
		if err = adminPool.QueryRow(ctx, "select count(*) from employee.voice_assessments where tenant_id=$1 and preview_id=$2", tenant, approvedPreviewID).Scan(&visible); err != nil || visible != 0 {
			t.Fatalf("assessment retained=%d err=%v", visible, err)
		}
	}
}
