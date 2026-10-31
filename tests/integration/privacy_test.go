//go:build integration

package integration_test

import (
        "context"
        "errors"
        "fmt"
        "testing"
        "time"

        employeeApp "github.com/staffy-co/staffy/internal/employee/app"
        employeeStore "github.com/staffy-co/staffy/internal/employee/store"
        "github.com/staffy-co/staffy/internal/identity/api"
        identityApp "github.com/staffy-co/staffy/internal/identity/app"
        identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
        identityStore "github.com/staffy-co/staffy/internal/identity/store"
        "github.com/staffy-co/staffy/internal/platform/id"
        "github.com/staffy-co/staffy/internal/platform/tenancy"
        privacyAPI "github.com/staffy-co/staffy/internal/privacy/api"
        privacyApp "github.com/staffy-co/staffy/internal/privacy/app"
        privacyDomain "github.com/staffy-co/staffy/internal/privacy/domain"
        privacyPort "github.com/staffy-co/staffy/internal/privacy/port"
        privacyStore "github.com/staffy-co/staffy/internal/privacy/store"
)

// privacyWorld assembles the privacy orchestrator over the real store with
// the real owning-module executors, plus a tenant and fixtures.
type privacyWorld struct {
        identity  *identityApp.Service
        employee  *employeeApp.Service
        privacy   *privacyApp.Service
        principal identityDomain.Principal
        tenant    id.UUID
        ids       *sequenceIDs
}

func newPrivacyIDs(prefix int) *sequenceIDs {
        values := make([]id.UUID, 0, 64)
        for i := 0; i < 64; i++ {
                values = append(values, id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", prefix*1000+i)))
        }
        return &sequenceIDs{values: values}
}

// newPrivacyWorld creates a fresh tenant owned by the principal, with the
// privacy module wired over the real repositories and executors. The seed
// keeps every world's UUIDs unique across tests.
func newPrivacyWorld(t *testing.T, seed int, clock api.Clock, executors map[string]privacyPort.ClassDeletionExecutor) *privacyWorld {
        t.Helper()
        ids := newPrivacyIDs(seed)
        identity := identityApp.New(identityStore.NewPostgres(runtimePool), clock, ids)
        principal := identityDomain.Principal{ID: id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", seed*100000)), CanCreateTenant: true}
        result, err := identity.CreateTenant(context.Background(), api.CreateTenantCommand{
                Principal: principal, Name: "Магазин Олега", IdempotencyKey: fmt.Sprintf("privacy-tenant-%03d", seed),
                CorrelationID: id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", seed*100000+1)),
        })
        if err != nil {
                t.Fatal(err)
        }
        employee := employeeApp.NewWithTestTTS(employeeStore.NewPostgres(runtimePool), identity, clock, ids, nil)
        world := &privacyWorld{
                identity: identity, employee: employee, principal: principal,
                tenant: result.Tenant.ID, ids: ids,
        }
        if executors == nil {
                executors = map[string]privacyPort.ClassDeletionExecutor{
                        privacyDomain.ExecutorEmployee: employee.PrivacyDeletionExecutor(),
                        privacyDomain.ExecutorIdentity: identity.PrivacyDeletionExecutor(),
                }
        }
        world.privacy = privacyApp.New(privacyStore.NewPostgres(runtimePool), identity, executors, clock, ids)
        return world
}

var privacyClock = fixedClock{time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)}

// completeEntry returns a fully documented map entry command of one class.
func completeEntry(world *privacyWorld, class privacyDomain.ArtifactClass) privacyAPI.MapEntryCommand {
        return privacyAPI.MapEntryCommand{
                Actor: world.principal, TenantID: world.tenant, Class: class,
                Purpose: "Обработка обращений магазина", LegalBasis: "Договор с клиентом",
                Controller: "ООО Магазин Олега", Processor: "Staffy",
                Recipients: []string{"Сотрудники магазина"}, SubProcessors: []string{"Хостинг Staffy"},
                Locations: []string{"Локальная база данных (EU)"}, AccessibleRoles: []string{"owner", "configurator"},
                KnownCopies:     []string{"postgres: employee", "бэкапы"},
                PolicyRef:       "ret-" + string(class),
                DeletionMethod:  "Локальная очистка через модуль-владелец",
                ExportMethod:    "Выгрузка администратором данных",
                CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-000000000401"),
        }
}

// zeroTermPolicy publishes an immediately-due retention policy.
func zeroTermPolicy(world *privacyWorld, class privacyDomain.ArtifactClass, deletionDays int) privacyAPI.PolicyCommand {
        return privacyAPI.PolicyCommand{
                Actor: world.principal, TenantID: world.tenant, Class: class,
                Purpose: "Хранение данных класса", LegalBasis: "Баланс интересов",
                Source: "tenant_decision", TerminalEvent: "last_change",
                ActiveUseDays: 0, ArchiveDays: 0, DeletionDays: deletionDays, BackupExpiryDays: 30,
                LegalHoldException: "Судебное удержание продлевает срок",
                OwnerNote:          "Владелец данных магазина",
                CorrelationID:      id.MustParse("01994f64-6c00-7000-8000-000000000402"),
        }
}

// seedDraft inserts one employee draft with one knowledge source, version and
// two chunks (fixtures bypass RLS through the admin pool).
func seedDraft(t *testing.T, world *privacyWorld) {
        t.Helper()
        ctx := context.Background()
        draftID := id.MustParse("01994f64-6c00-7000-8000-000000000501")
        sourceID := id.MustParse("01994f64-6c00-7000-8000-000000000502")
        moment := privacyClock.Now()
        if _, err := adminPool.Exec(ctx, `insert into employee.drafts (tenant_id, employee_id, owner_id, created_at, updated_at) values ($1,$2,$3,$4,$4)`,
                world.tenant, draftID, world.principal.ID, moment); err != nil {
                t.Fatal(err)
        }
        if _, err := adminPool.Exec(ctx, `insert into employee.knowledge_sources (tenant_id, employee_id, source_id, source_type, title, owner_name, provenance, checksum_key, source_scope, priority, language, status, created_at, updated_at) values ($1,$2,$3,'faq','Возвраты','Олег','fixture',decode(repeat('0',64),'hex'),'tenant',50,'ru','ready',$4,$4)`,
                world.tenant, draftID, sourceID, moment); err != nil {
                t.Fatal(err)
        }
        if _, err := adminPool.Exec(ctx, `insert into employee.knowledge_versions (tenant_id, employee_id, source_id, version, checksum, source_text, created_at) values ($1,$2,$3,1,repeat('a',64),'Текст источника',$4)`,
                world.tenant, draftID, sourceID, moment); err != nil {
                t.Fatal(err)
        }
        for position := 0; position < 2; position++ {
                if _, err := adminPool.Exec(ctx, `insert into employee.knowledge_chunks (tenant_id, employee_id, source_id, version, position, chunk_text, checksum) values ($1,$2,$3,1,$4,'Чанк',repeat('b',64))`,
                        world.tenant, draftID, sourceID, position); err != nil {
                        t.Fatal(err)
                }
        }
}

// seedAudit inserts one employee audit row so the audit class has an anchor.
func seedAudit(t *testing.T, world *privacyWorld) {
        t.Helper()
        auditID := id.MustParse("01994f64-6c00-7000-8000-000000000503")
        draftID := id.MustParse("01994f64-6c00-7000-8000-000000000501")
        if _, err := adminPool.Exec(context.Background(), `insert into employee.audit (tenant_id, audit_id, employee_id, actor_id, attempted_version, resulting_version, outcome, correlation_id, occurred_at) values ($1,$2,$3,$4,0,1,'created',$5,$6)`,
                world.tenant, auditID, draftID, world.principal.ID, id.MustParse("01994f64-6c00-7000-8000-000000000504"), privacyClock.Now()); err != nil {
                t.Fatal(err)
        }
}

func outcomeOf(outcomes []privacyDomain.DeletionOutcome, class privacyDomain.ArtifactClass) (privacyDomain.DeletionOutcome, bool) {
        for _, outcome := range outcomes {
                if outcome.Class == class {
                        return outcome, true
                }
        }
        return privacyDomain.DeletionOutcome{}, false
}

// TestPrivacyFullLifecycle walks the honest path: complete map entries and
// policies, deletion with the real executors, the receipt with hash evidence,
// idempotent replay and the honest conflict.
func TestPrivacyFullLifecycle(t *testing.T) {
        world := newPrivacyWorld(t, 71, privacyClock, nil)
        seedDraft(t, world)
        seedAudit(t, world)

        classes := []privacyDomain.ArtifactClass{
                privacyDomain.ClassKnowledgeSourcePayload,
                privacyDomain.ClassEmployeeDraft,
                privacyDomain.ClassConnectionSecretSeal,
                privacyDomain.ClassCapabilityEvidence,
                privacyDomain.ClassAuditTrail,
                privacyDomain.ClassBehaviorConfiguration,
        }
        for _, class := range classes {
                if _, err := world.privacy.PublishMapEntry(context.Background(), completeEntry(world, class)); err != nil {
                        t.Fatalf("publish entry %s: %v", class, err)
                }
                // The connection-secret-seal class carries no executor: the anchor
                // query still runs inside RLS, so publication of a policy is enough.
                if _, err := world.privacy.PublishPolicy(context.Background(), zeroTermPolicy(world, class, 3650)); err != nil {
                        t.Fatalf("publish policy %s: %v", class, err)
                }
        }

        correlation := id.MustParse("01994f64-6c00-7000-8000-000000000705")
        // First command: everything except the draft aggregate. The draft purge
        // removes the whole per-employee aggregate (knowledge included), so the
        // knowledge class is exercised in its own command first.
        firstClasses := []privacyDomain.ArtifactClass{
                privacyDomain.ClassKnowledgeSourcePayload,
                privacyDomain.ClassConnectionSecretSeal,
                privacyDomain.ClassCapabilityEvidence,
                privacyDomain.ClassAuditTrail,
                privacyDomain.ClassBehaviorConfiguration,
        }
        result, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: firstClasses,
                IdempotencyKey: "privacy-delete-01", CorrelationID: correlation,
        })
        if err != nil {
                t.Fatal(err)
        }
        if result.Replayed {
                t.Fatal("first execution must not be a replay")
        }

        knowledge, ok := outcomeOf(result.Receipt.Outcomes, privacyDomain.ClassKnowledgeSourcePayload)
        if !ok || knowledge.Outcome != privacyDomain.OutcomeDeleted || knowledge.AffectedCount != 1 {
                t.Fatalf("knowledge outcome = %#v", knowledge)
        }
        seal, ok := outcomeOf(result.Receipt.Outcomes, privacyDomain.ClassConnectionSecretSeal)
        if !ok || seal.Outcome != privacyDomain.OutcomeUnsupported || seal.Block == nil || seal.Block.Gate != "AG-4" {
                t.Fatalf("seal outcome = %#v", seal)
        }
        evidence, ok := outcomeOf(result.Receipt.Outcomes, privacyDomain.ClassCapabilityEvidence)
        if !ok || evidence.Outcome != privacyDomain.OutcomeUnsupported {
                t.Fatalf("evidence outcome = %#v", evidence)
        }
        audit, ok := outcomeOf(result.Receipt.Outcomes, privacyDomain.ClassAuditTrail)
        if !ok || audit.Outcome != privacyDomain.OutcomeRetainedByLaw || audit.DueAt == nil {
                t.Fatalf("audit outcome = %#v", audit)
        }
        behavior, ok := outcomeOf(result.Receipt.Outcomes, privacyDomain.ClassBehaviorConfiguration)
        if !ok || behavior.Outcome != privacyDomain.OutcomeUnknown || behavior.ReconciliationOwner == "" || behavior.ReconciliationDeadline == nil {
                t.Fatalf("behavior outcome = %#v", behavior)
        }
        if len(result.Receipt.ReceiptHash) != 64 {
                t.Fatalf("receipt hash = %q", result.Receipt.ReceiptHash)
        }
        // The receipt pins every applied policy version.
        if len(result.Receipt.PolicyVersions) != len(firstClasses) {
                t.Fatalf("policy pins = %d", len(result.Receipt.PolicyVersions))
        }

        // The knowledge rows are really gone; the draft still stands.
        var remaining int
        if err := adminPool.QueryRow(context.Background(), "select count(*) from employee.knowledge_sources where tenant_id=$1", world.tenant).Scan(&remaining); err != nil || remaining != 0 {
                t.Fatalf("knowledge rows remain: %d (%v)", remaining, err)
        }
        if err := adminPool.QueryRow(context.Background(), "select count(*) from employee.drafts where tenant_id=$1", world.tenant).Scan(&remaining); err != nil || remaining != 1 {
                t.Fatalf("draft rows = %d (%v)", remaining, err)
        }
        // The audit journal survived the retained_by_law decision.
        if err := adminPool.QueryRow(context.Background(), "select count(*) from employee.audit where tenant_id=$1", world.tenant).Scan(&remaining); err != nil || remaining != 1 {
                t.Fatalf("audit rows = %d (%v)", remaining, err)
        }

        // Second command: the draft aggregate purge removes the drafts (and the
        // dependent rows of the employee data contour).
        draftResult, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{privacyDomain.ClassEmployeeDraft},
                IdempotencyKey: "privacy-delete-02", CorrelationID: correlation,
        })
        if err != nil {
                t.Fatal(err)
        }
        drafts, ok := outcomeOf(draftResult.Receipt.Outcomes, privacyDomain.ClassEmployeeDraft)
        if !ok || drafts.Outcome != privacyDomain.OutcomeDeleted || drafts.AffectedCount != 1 {
                t.Fatalf("drafts outcome = %#v", drafts)
        }
        if err := adminPool.QueryRow(context.Background(), "select count(*) from employee.drafts where tenant_id=$1", world.tenant).Scan(&remaining); err != nil || remaining != 0 {
                t.Fatalf("draft rows remain: %d (%v)", remaining, err)
        }

        // Idempotent replay returns the stored receipt.
        replay, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: firstClasses,
                IdempotencyKey: "privacy-delete-01", CorrelationID: correlation,
        })
        if err != nil {
                t.Fatal(err)
        }
        if !replay.Replayed || replay.Receipt.RequestID != result.Receipt.RequestID || replay.Receipt.ReceiptHash != result.Receipt.ReceiptHash {
                t.Fatalf("replay mismatch: replayed=%v request=%s", replay.Replayed, replay.Receipt.RequestID)
        }

        // The same key with a different scope is an honest conflict.
        _, err = world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: firstClasses[:1],
                IdempotencyKey: "privacy-delete-01", CorrelationID: correlation,
        })
        if !errors.Is(err, privacyDomain.ErrIdempotencyConflict) {
                t.Fatalf("conflict error = %v", err)
        }

        // The receipt is readable through the view scope.
        view, err := world.privacy.ReadReceipt(context.Background(), world.principal, world.tenant, id.MustParse(result.Receipt.RequestID))
        if err != nil {
                t.Fatal(err)
        }
        if view.Receipt.RequestID != result.Receipt.RequestID || len(view.Receipt.Outcomes) != len(firstClasses) {
                t.Fatalf("receipt view mismatch: %#v", view.Receipt)
        }
}

// TestPrivacyScheduledOutcome verifies the honest scheduled result when the
// policy term has not expired yet.
func TestPrivacyScheduledOutcome(t *testing.T) {
        world := newPrivacyWorld(t, 72, privacyClock, nil)
        seedDraft(t, world)
        if _, err := world.privacy.PublishMapEntry(context.Background(), completeEntry(world, privacyDomain.ClassKnowledgeSourcePayload)); err != nil {
                t.Fatal(err)
        }
        command := zeroTermPolicy(world, privacyDomain.ClassKnowledgeSourcePayload, 365)
        command.ActiveUseDays = 30
        if _, err := world.privacy.PublishPolicy(context.Background(), command); err != nil {
                t.Fatal(err)
        }
        result, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{privacyDomain.ClassKnowledgeSourcePayload},
                IdempotencyKey: "privacy-sched-01", CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000713"),
        })
        if err != nil {
                t.Fatal(err)
        }
        knowledge, ok := outcomeOf(result.Receipt.Outcomes, privacyDomain.ClassKnowledgeSourcePayload)
        if !ok || knowledge.Outcome != privacyDomain.OutcomeScheduled || knowledge.DueAt == nil {
                t.Fatalf("scheduled outcome = %#v", knowledge)
        }
        want := privacyClock.Now().Add(30 * 24 * time.Hour)
        if !knowledge.DueAt.Equal(want) {
                t.Fatalf("due = %v, want %v", knowledge.DueAt, want)
        }
        // Nothing was deleted.
        var remaining int
        if err := adminPool.QueryRow(context.Background(), "select count(*) from employee.knowledge_sources where tenant_id=$1", world.tenant).Scan(&remaining); err != nil || remaining != 1 {
                t.Fatalf("knowledge rows = %d (%v)", remaining, err)
        }
}

// TestPrivacyHoldOutcomes covers legal hold: active hold retains the class,
// release lets the deletion execute, and an expired review deadline is shown
// as requiring a decision.
func TestPrivacyHoldOutcomes(t *testing.T) {
        world := newPrivacyWorld(t, 73, privacyClock, nil)
        seedDraft(t, world)
        class := privacyDomain.ClassKnowledgeSourcePayload
        if _, err := world.privacy.PublishMapEntry(context.Background(), completeEntry(world, class)); err != nil {
                t.Fatal(err)
        }
        if _, err := world.privacy.PublishPolicy(context.Background(), zeroTermPolicy(world, class, 365)); err != nil {
                t.Fatal(err)
        }
        correlation := id.MustParse("01994f64-6c00-7000-8000-000000000723")
        if err := world.privacy.ActivateHold(context.Background(), privacyAPI.HoldCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{class},
                Basis: "Судебный запрос по спору клиента", Approver: "Юрист Ромашкова",
                ReviewDeadline: privacyClock.Now().Add(48 * time.Hour), CorrelationID: correlation,
        }); err != nil {
                t.Fatal(err)
        }
        held, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{class},
                IdempotencyKey: "privacy-hold-01", CorrelationID: correlation,
        })
        if err != nil {
                t.Fatal(err)
        }
        outcome, _ := outcomeOf(held.Receipt.Outcomes, class)
        if outcome.Outcome != privacyDomain.OutcomeRetainedByLaw {
                t.Fatalf("held outcome = %#v", outcome)
        }
        var remaining int
        if err := adminPool.QueryRow(context.Background(), "select count(*) from employee.knowledge_sources where tenant_id=$1", world.tenant).Scan(&remaining); err != nil || remaining != 1 {
                t.Fatalf("knowledge rows under hold = %d (%v)", remaining, err)
        }

        if err := world.privacy.ReleaseHold(context.Background(), privacyAPI.HoldCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{class},
                Basis: "Спор закрыт", Approver: "Юрист Ромашкова", CorrelationID: correlation,
        }); err != nil {
                t.Fatal(err)
        }
        released, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{class},
                IdempotencyKey: "privacy-hold-02", CorrelationID: correlation,
        })
        if err != nil {
                t.Fatal(err)
        }
        outcome, _ = outcomeOf(released.Receipt.Outcomes, class)
        if outcome.Outcome != privacyDomain.OutcomeDeleted {
                t.Fatalf("released outcome = %#v", outcome)
        }

        // An activated hold whose review deadline has passed requires a decision.
        if err := world.privacy.ActivateHold(context.Background(), privacyAPI.HoldCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{privacyDomain.ClassEmployeeDraft},
                Basis: "Второй спор", Approver: "Юрист Ромашкова",
                ReviewDeadline: privacyClock.Now().Add(1 * time.Hour), CorrelationID: correlation,
        }); err != nil {
                t.Fatal(err)
        }
        laterClock := fixedClock{privacyClock.Now().Add(72 * time.Hour)}
        view, err := privacyApp.New(privacyStore.NewPostgres(runtimePool), world.identity, nil, laterClock, newPrivacyIDs(998)).DataView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        for _, classView := range view.Classes {
                if classView.Spec.ID == privacyDomain.ClassEmployeeDraft {
                        if classView.Hold.Status != privacyDomain.HoldRequiresDecision {
                                t.Fatalf("expired hold = %s", classView.Hold.Status)
                        }
                }
        }
}

// TestPrivacyFailClosedGates covers the honest rejections: an incomplete map
// entry blocks the class route, a blanket policy without a class is refused,
// and the 24-hour preview invariant rejects weaker policies.
func TestPrivacyFailClosedGates(t *testing.T) {
        world := newPrivacyWorld(t, 74, privacyClock, nil)
        seedDraft(t, world)

        // Undocumented class blocks the deletion.
        _, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{privacyDomain.ClassKnowledgeSourcePayload},
                IdempotencyKey: "privacy-gate-01", CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000733"),
        })
        var rule privacyDomain.RuleError
        if !errors.As(err, &rule) || rule.Rule != "map" {
                t.Fatalf("undocumented gate = %v", err)
        }

        // An incomplete entry is stored honestly and blocks the route.
        incomplete := completeEntry(world, privacyDomain.ClassKnowledgeSourcePayload)
        incomplete.LegalBasis = ""
        incomplete.KnownCopies = nil
        if _, err = world.privacy.PublishMapEntry(context.Background(), incomplete); err != nil {
                t.Fatal(err)
        }
        _, err = world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{privacyDomain.ClassKnowledgeSourcePayload},
                IdempotencyKey: "privacy-gate-02", CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000734"),
        })
        if !errors.As(err, &rule) || rule.Rule != "map" {
                t.Fatalf("incomplete gate = %v", err)
        }
        // The data view derives the entry as incomplete with the missing names.
        view, err := world.privacy.DataView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        for _, classView := range view.Classes {
                if classView.Spec.ID == privacyDomain.ClassKnowledgeSourcePayload {
                        if classView.Status != privacyDomain.MapStatusIncomplete || len(classView.Missing) != 2 {
                                t.Fatalf("incomplete view = %s %v", classView.Status, classView.Missing)
                        }
                }
        }

        // A blanket policy without a class is rejected with an honest cause.
        _, err = world.privacy.PublishPolicy(context.Background(), zeroTermPolicy(world, "", 365))
        if !errors.As(err, &rule) || rule.Rule != "class" {
                t.Fatalf("blanket policy gate = %v", err)
        }

        // The 24-hour preview invariant rejects a weaker policy.
        preview := zeroTermPolicy(world, privacyDomain.ClassVoicePreviewMedia, 365)
        preview.ActiveUseDays = 5
        _, err = world.privacy.PublishPolicy(context.Background(), preview)
        if !errors.As(err, &rule) {
                t.Fatalf("preview invariant gate = %v", err)
        }
        // A policy inside the invariant is accepted.
        tight := zeroTermPolicy(world, privacyDomain.ClassVoicePreviewMedia, 365)
        tight.ActiveUseDays = 1
        tight.DeletionDays = 3650
        if _, err = world.privacy.PublishPolicy(context.Background(), tight); err != nil {
                t.Fatal(err)
        }
}

// TestPrivacyMapVersioning checks append-only versioning: two versions, the
// latest wins in the derived view, history stays.
func TestPrivacyMapVersioning(t *testing.T) {
        world := newPrivacyWorld(t, 75, privacyClock, nil)
        first := completeEntry(world, privacyDomain.ClassEmployeeDraft)
        first.Purpose = "Первая версия"
        second := completeEntry(world, privacyDomain.ClassEmployeeDraft)
        second.Purpose = "Вторая версия"
        published1, err := world.privacy.PublishMapEntry(context.Background(), first)
        if err != nil {
                t.Fatal(err)
        }
        published2, err := world.privacy.PublishMapEntry(context.Background(), second)
        if err != nil {
                t.Fatal(err)
        }
        if published1.Version != 1 || published2.Version != 2 {
                t.Fatalf("versions = %d %d", published1.Version, published2.Version)
        }
        view, err := world.privacy.DataView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        for _, classView := range view.Classes {
                if classView.Spec.ID == privacyDomain.ClassEmployeeDraft {
                        if classView.Entry == nil || classView.Entry.Version != 2 || classView.Entry.Purpose != "Вторая версия" {
                                t.Fatalf("current entry = %#v", classView.Entry)
                        }
                }
        }
        // History is never rewritten.
        var versions int
        if err := adminPool.QueryRow(context.Background(), "select count(*) from privacy.data_map_entries where tenant_id=$1 and class='employee-draft'", world.tenant).Scan(&versions); err != nil || versions != 2 {
                t.Fatalf("stored versions = %d (%v)", versions, err)
        }
}

// TestPrivacyMemberIdentityInvitations verifies the identity executor:
// expired invitations are deleted with a count, active memberships keep the
// managed offboarding block.
func TestPrivacyMemberIdentityInvitations(t *testing.T) {
        earlyClock := fixedClock{time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
        world := newPrivacyWorld(t, 76, earlyClock, nil)
        // The privacy orchestrator runs at a later clock than the invitation
        // creation, so the seven-day invitation is already expired.
        world.privacy = privacyApp.New(privacyStore.NewPostgres(runtimePool), world.identity, map[string]privacyPort.ClassDeletionExecutor{
                privacyDomain.ExecutorEmployee: world.employee.PrivacyDeletionExecutor(),
                privacyDomain.ExecutorIdentity: world.identity.PrivacyDeletionExecutor(),
        }, privacyClock, newPrivacyIDs(96))
        // Invite a configurator at the early clock: the invitation expires seven
        // days later, which is before the privacy clock.
        _, _, err := world.identity.InviteMember(context.Background(), api.MembershipCommand{
                Actor: world.principal, TenantID: world.tenant, Email: "colleague@example.com",
                Roles: []identityDomain.Role{identityDomain.Configurator}, Scopes: []identityDomain.Scope{identityDomain.Configuration},
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000753"),
        })
        if err != nil {
                t.Fatal(err)
        }
        if _, err = world.privacy.PublishMapEntry(context.Background(), completeEntry(world, privacyDomain.ClassMemberIdentity)); err != nil {
                t.Fatal(err)
        }
        if _, err = world.privacy.PublishPolicy(context.Background(), zeroTermPolicy(world, privacyDomain.ClassMemberIdentity, 365)); err != nil {
                t.Fatal(err)
        }
        result, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{privacyDomain.ClassMemberIdentity},
                IdempotencyKey: "privacy-member-01", CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000754"),
        })
        if err != nil {
                t.Fatal(err)
        }
        outcome, _ := outcomeOf(result.Receipt.Outcomes, privacyDomain.ClassMemberIdentity)
        if outcome.Outcome != privacyDomain.OutcomeUnsupported || outcome.Block == nil {
                t.Fatalf("member identity outcome = %#v", outcome)
        }
        if outcome.AffectedCount != 1 {
                t.Fatalf("expired invitations deleted = %d", outcome.AffectedCount)
        }
        var invitations int
        if err := adminPool.QueryRow(context.Background(), "select count(*) from identity.invitations where tenant_id=$1", world.tenant).Scan(&invitations); err != nil || invitations != 0 {
                t.Fatalf("invitations remain = %d (%v)", invitations, err)
        }
        // Active memberships survived.
        var memberships int
        if err := adminPool.QueryRow(context.Background(), "select count(*) from identity.memberships where tenant_id=$1 and status='active'", world.tenant).Scan(&memberships); err != nil || memberships != 1 {
                t.Fatalf("memberships = %d (%v)", memberships, err)
        }
}

// failingExecutor injects the ambiguous refusal for the unknown path.
type failingExecutor struct{}

func (failingExecutor) ExecuteClassDeletion(context.Context, tenancy.Context, privacyDomain.ArtifactClass, time.Time) (privacyPort.ExecutorResult, error) {
        return privacyPort.ExecutorResult{}, errors.New("ambiguous infrastructure failure")
}

// TestPrivacyUnknownInjection verifies the honest unknown outcome with the
// reconciliation owner, deadline and note.
func TestPrivacyUnknownInjection(t *testing.T) {
        world := newPrivacyWorld(t, 77, privacyClock, map[string]privacyPort.ClassDeletionExecutor{
                privacyDomain.ExecutorEmployee: failingExecutor{},
        })
        seedDraft(t, world)
        class := privacyDomain.ClassKnowledgeSourcePayload
        if _, err := world.privacy.PublishMapEntry(context.Background(), completeEntry(world, class)); err != nil {
                t.Fatal(err)
        }
        if _, err := world.privacy.PublishPolicy(context.Background(), zeroTermPolicy(world, class, 365)); err != nil {
                t.Fatal(err)
        }
        result, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: world.principal, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{class},
                IdempotencyKey: "privacy-unknown-01", CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000763"),
        })
        if err != nil {
                t.Fatal(err)
        }
        outcome, _ := outcomeOf(result.Receipt.Outcomes, class)
        if outcome.Outcome != privacyDomain.OutcomeUnknown {
                t.Fatalf("unknown outcome = %#v", outcome)
        }
        if outcome.ReconciliationOwner == "" || outcome.ReconciliationDeadline == nil {
                t.Fatalf("reconciliation missing: %#v", outcome)
        }
        // The data is still there: no blind retry happened.
        var remaining int
        if err := adminPool.QueryRow(context.Background(), "select count(*) from employee.knowledge_sources where tenant_id=$1", world.tenant).Scan(&remaining); err != nil || remaining != 1 {
                t.Fatalf("knowledge rows = %d (%v)", remaining, err)
        }
}

// TestPrivacyAuthorizationAndIsolation verifies the data_governance boundary:
// audit-only members may view but not manage, strangers get 404 semantics,
// and tenant rows never leak across tenants.
func TestPrivacyAuthorizationAndIsolation(t *testing.T) {
        world := newPrivacyWorld(t, 78, privacyClock, nil)
        if _, err := world.privacy.PublishMapEntry(context.Background(), completeEntry(world, privacyDomain.ClassEmployeeDraft)); err != nil {
                t.Fatal(err)
        }
        // An expert_qa member with the audit scope may view the map.
        auditor := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000000781"), CanCreateTenant: true}
        if _, err := adminPool.Exec(context.Background(), `insert into identity.principals (principal_id, created_at) values ($1,$2) on conflict do nothing`, auditor.ID, privacyClock.Now()); err != nil {
                t.Fatal(err)
        }
        if _, err := adminPool.Exec(context.Background(), `insert into identity.memberships (tenant_id, principal_id, role, status, scopes, created_at) values ($1,$2,'expert_qa','active',ARRAY['audit'],$3)`,
                world.tenant, auditor.ID, privacyClock.Now()); err != nil {
                t.Fatal(err)
        }
        if _, err := world.privacy.DataView(context.Background(), auditor, world.tenant); err != nil {
                t.Fatalf("audit view = %v", err)
        }
        _, err := world.privacy.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: auditor, TenantID: world.tenant, Classes: []privacyDomain.ArtifactClass{privacyDomain.ClassEmployeeDraft},
                IdempotencyKey: "privacy-audit-01", CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000782"),
        })
        if !errors.Is(err, privacyDomain.ErrNotFound) {
                t.Fatalf("audit manage = %v", err)
        }

        // A principal without any membership gets the 404 semantics.
        stranger := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000000783"), CanCreateTenant: true}
        if _, err = world.privacy.DataView(context.Background(), stranger, world.tenant); !errors.Is(err, privacyDomain.ErrNotFound) {
                t.Fatalf("stranger view = %v", err)
        }

        // Another tenant never sees the documented classes (RLS).
        other := newPrivacyWorld(t, 79, privacyClock, nil)
        otherView, err := other.privacy.DataView(context.Background(), other.principal, other.tenant)
        if err != nil {
                t.Fatal(err)
        }
        for _, classView := range otherView.Classes {
                if classView.Entry != nil {
                        t.Fatalf("tenant isolation leaked entry for %s", classView.Spec.ID)
                }
        }
}
