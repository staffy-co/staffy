//go:build integration

package integration_test

import (
        "context"
        "encoding/json"
        "errors"
        "fmt"
        "strings"
        "sync"
        "testing"
        "time"

        connectorApp "github.com/staffy-co/staffy/internal/connector/app"
        connectorStore "github.com/staffy-co/staffy/internal/connector/store"
        connectorVerifier "github.com/staffy-co/staffy/internal/connector/verifier"
        employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
        employeeApp "github.com/staffy-co/staffy/internal/employee/app"
        employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
        employeeStore "github.com/staffy-co/staffy/internal/employee/store"
        "github.com/staffy-co/staffy/internal/identity/api"
        identityApp "github.com/staffy-co/staffy/internal/identity/app"
        identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
        identityStore "github.com/staffy-co/staffy/internal/identity/store"
        connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
        connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
        "github.com/staffy-co/staffy/internal/platform/id"
)

var releaseClock = fixedClock{time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)}

// releaseWorld wires the employee release surface over the real store, the
// identity service and the connector facts port.
type releaseWorld struct {
        identity  *identityApp.Service
        employee  *employeeApp.Service
        connector *connectorApp.Service
        owner     identityDomain.Principal
        tenant    id.UUID
}

func newReleaseWorld(t *testing.T, seed int) *releaseWorld {
        t.Helper()
        // Numeric UUID suffixes (12 hex chars) keep every world unique. The
        // identity clock is real time: invitation acceptance compares against
        // clock_timestamp() in SQL, so a fixed past clock would read as expired.
        suffix := func(slot int) string { return fmt.Sprintf("0000000%05d", seed*10+slot) }
        identity := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC().Add(time.Hour)}, &employeeIDs{})
        owner := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-" + suffix(1)), CanCreateTenant: true}
        tenant := createTenant(t, identity, owner, "Release tenant", fmt.Sprintf("release-tenant-%03d", seed), suffix(2)).Tenant.ID
        ids := &employeeIDs{}
        connector := connectorApp.New(connectorStore.NewPostgres(runtimePool), identity, connectorVerifier.Synthetic{}, releaseClock, ids)
        employee := employeeApp.New(employeeStore.NewPostgres(runtimePool), identity, releaseClock, ids).WithConnectorFacts(connector)
        return &releaseWorld{identity: identity, employee: employee, connector: connector, owner: owner, tenant: tenant}
}

func (w *releaseWorld) createEmployee(t *testing.T, phone bool) id.UUID {
        t.Helper()
        corr := id.MustParse("01994f64-6c00-7000-8000-000000000901")
        created, err := w.employee.Create(context.Background(), employeeAPI.CreateCommand{
                Actor: w.owner, TenantID: w.tenant, Name: "Аня", IdempotencyKey: fmt.Sprintf("release-employee-%d", time.Now().UnixNano()), CorrelationID: corr,
        })
        if err != nil {
                t.Fatal(err)
        }
        channels := []string{"phone"}
        if !phone {
                channels = []string{"web_chat"}
        }
        brief := employeeDomain.Brief{Purpose: "Помогать клиентам магазина", Name: "Аня", Languages: []string{"ru"}, TimeZone: "Europe/Moscow",
                WorkingHours:    employeeDomain.WorkingHours{Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00"},
                DesiredChannels: channels, PrimaryTasks: []string{"Консультации"}}
        saved, err := w.employee.Save(context.Background(), employeeAPI.SaveCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: created.Draft.EmployeeID, ExpectedVersion: 1, Brief: brief, CorrelationID: corr,
        })
        if err != nil {
                t.Fatal(err)
        }
        return saved.EmployeeID
}

func (w *releaseWorld) pinPack(t *testing.T, employeeID id.UUID) {
        t.Helper()
        if _, err := w.employee.Pin(context.Background(), employeeAPI.PinCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employeeID, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000902"),
                ExpectedVersion: 2, PackID: "auto-parts", PackVersion: "1.0.0", Variants: map[string]any{"parts_interest": "both"},
        }); err != nil {
                t.Fatal(err)
        }
}

func (w *releaseWorld) saveBehavior(t *testing.T, employeeID id.UUID, expectedVersion int64) {
        t.Helper()
        if _, err := w.employee.SaveBehavior(context.Background(), employeeAPI.BehaviorCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employeeID, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000903"),
                ExpectedVersion: expectedVersion, ChangeSummary: "Первичная настройка",
                Config: employeeDomain.Behavior{Role: "Консультант", Purpose: "Общие ответы", AIDisclosure: true, Tone: "neutral",
                        Modes: []string{"answer"}, Confirmation: "required", Handoff: "required"},
        }); err != nil {
                t.Fatal(err)
        }
}

func (w *releaseWorld) assemble(t *testing.T, employeeID id.UUID, expected int64) employeeDomain.ReleaseDraft {
        t.Helper()
        draft, err := w.employee.AssembleDraft(context.Background(), employeeAPI.AssembleCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employeeID, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000904"),
                ExpectedRevision: expected,
        })
        if err != nil {
                t.Fatal(err)
        }
        return draft
}

func (w *releaseWorld) createVersion(t *testing.T, employeeID id.UUID, expected int64, description string) employeeDomain.ConfigVersion {
        t.Helper()
        version, err := w.employee.CreateConfigVersion(context.Background(), employeeAPI.VersionCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employeeID, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000905"),
                Description: description, ExpectedRevision: expected,
        })
        if err != nil {
                t.Fatal(err)
        }
        return version
}

func (w *releaseWorld) seal(t *testing.T, employeeID id.UUID, configVersion, expected int64) (employeeDomain.ReleaseManifest, error) {
        return w.employee.SealManifest(context.Background(), employeeAPI.SealCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employeeID, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000906"),
                ConfigVersion: configVersion, ExpectedRevision: expected,
        })
}

// TestReleaseVersionFlowHonestImmutable: the full walk — assemble with
// honest absences, immutable version, sealed manifest with digest, and the
// snapshot never changes after later component edits.
func TestReleaseVersionFlowHonestImmutable(t *testing.T) {
        w := newReleaseWorld(t, 95)
        ctx := context.Background()
        employeeID := w.createEmployee(t, true) // phone brief: voice applicable

        // Assemble the first draft from a partially configured employee.
        draft := w.assemble(t, employeeID, 0)
        if draft.Revision != 1 {
                t.Fatalf("first assemble revision = %d", draft.Revision)
        }
        if draft.Refs.Pack.Status != employeeDomain.RefAbsent || draft.Refs.Pack.Note == "" {
                t.Fatalf("pack absence must be recorded: %+v", draft.Refs.Pack)
        }
        if draft.Refs.Behavior.Status != employeeDomain.RefAbsent || draft.Refs.Knowledge.Status != employeeDomain.RefAbsent {
                t.Fatalf("behavior/knowledge absences must be recorded: %+v", draft.Refs)
        }
        if draft.Refs.Voice.Status != employeeDomain.RefAbsent || draft.Refs.Voice.Note == "" {
                t.Fatalf("phone brief without voice must record an applicable absence: %+v", draft.Refs.Voice)
        }

        // A stale revision is an honest conflict carrying the current state.
        _, err := w.employee.AssembleDraft(ctx, employeeAPI.AssembleCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employeeID, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000907"),
                ExpectedRevision: 0,
        })
        var conflict employeeDomain.ReleaseConflictError
        if !errors.As(err, &conflict) || conflict.Current != 1 {
                t.Fatalf("stale assemble = %v", err)
        }

        // Complete the configuration and re-assemble: only the draft changes.
        w.pinPack(t, employeeID)
        w.saveBehavior(t, employeeID, 3)
        draft = w.assemble(t, employeeID, draft.Revision)
        if draft.Revision != 2 || draft.Refs.Pack.Status != employeeDomain.RefPresent || draft.Refs.Behavior.Status != employeeDomain.RefPresent {
                t.Fatalf("re-assembled draft = %+v", draft)
        }

        // Create the immutable version from the locked draft refs.
        version := w.createVersion(t, employeeID, draft.Revision, "Пилотная конфигурация №1")
        if version.Version != 1 || version.VersionID == "" || version.Refs.Pack.ID != "auto-parts" || version.Refs.Pack.Version != "1.0.0" {
                t.Fatalf("version = %+v", version)
        }
        // The draft revision bumped: a stale version command conflicts.
        if _, err := w.employee.CreateConfigVersion(ctx, employeeAPI.VersionCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employeeID, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000908"),
                Description: "stale", ExpectedRevision: draft.Revision,
        }); !errors.As(err, &conflict) {
                t.Fatalf("stale version create = %v", err)
        }

        // Seal: incomplete (runtime gaps), digest over canonical bytes, unique.
        sealRevision := draft.Revision + 1 // version creation bumped the draft
        manifest, err := w.seal(t, employeeID, version.Version, sealRevision)
        if err != nil {
                t.Fatal(err)
        }
        if manifest.Status != employeeDomain.ManifestStatusIncomplete {
                t.Fatalf("sealed status = %q, want incomplete (no runtime surfaces)", manifest.Status)
        }
        if len(manifest.Digest) != 64 || manifest.SchemaVersion != employeeDomain.ManifestSchemaCurrent {
                t.Fatalf("manifest digest/schema = %q/%d", manifest.Digest, manifest.SchemaVersion)
        }
        if len(manifest.Content.Runtime) != 8 {
                t.Fatalf("runtime components = %d", len(manifest.Content.Runtime))
        }
        for _, component := range manifest.Content.Runtime {
                if component.Present {
                        t.Fatalf("runtime component %q claimed present", component.Component)
                }
        }

        // The manifest reads back cleanly with a matching digest.
        view, err := w.employee.ReadReleaseManifest(ctx, w.owner, w.tenant, employeeID, version.Version)
        if err != nil || view.Quarantine != employeeDomain.QuarantineNone || view.Content == nil {
                t.Fatalf("manifest read = %+v err=%v", view, err)
        }
        if view.Digest != manifest.Digest {
                t.Fatalf("read digest = %q, want %q", view.Digest, manifest.Digest)
        }

        // Later component edits change only the mutable draft: the sealed
        // manifest and its digest stay byte-identical.
        w.saveBehavior(t, employeeID, 4)
        current := sealRevision + 1 // sealing bumped the draft revision
        draft = w.assemble(t, employeeID, current)
        if draft.Revision != current+1 {
                t.Fatalf("draft revision after re-assemble = %d, want %d", draft.Revision, current+1)
        }
        view, err = w.employee.ReadReleaseManifest(ctx, w.owner, w.tenant, employeeID, version.Version)
        if err != nil {
                t.Fatal(err)
        }
        if view.Quarantine != "" || view.Digest != manifest.Digest {
                t.Fatalf("sealed manifest changed after component edit: %+v", view)
        }
}

// TestSealRaceProducesExactlyOneManifest: two concurrent seals under one
// expected revision — exactly one manifest wins, the loser gets the honest
// conflict with the current revision.
func TestSealRaceProducesExactlyOneManifest(t *testing.T) {
        w := newReleaseWorld(t, 96)
        employeeID := w.createEmployee(t, false) // text employee: voice not applicable
        draft := w.assemble(t, employeeID, 0)
        if draft.Refs.Voice.Status != employeeDomain.RefNotApplicable {
                t.Fatalf("text employee voice = %+v, want not_applicable", draft.Refs.Voice)
        }
        version := w.createVersion(t, employeeID, draft.Revision, "Гонка запечатывания")

        // Both racers hold the same expected revision.
        revision := draft.Revision + 1
        const racers = 2
        results := make(chan error, racers)
        var start sync.WaitGroup
        start.Add(1)
        for i := 0; i < racers; i++ {
                go func() {
                        start.Wait()
                        _, err := w.seal(t, employeeID, version.Version, revision)
                        results <- err
                }()
        }
        start.Done()
        var winner, loser int
        var conflict employeeDomain.ReleaseConflictError
        for i := 0; i < racers; i++ {
                err := <-results
                switch {
                case err == nil:
                        winner++
                case errors.As(err, &conflict):
                        loser++
                        if conflict.Current <= revision {
                                t.Fatalf("conflict must carry the bumped current revision, got %d", conflict.Current)
                        }
                default:
                        t.Fatalf("unexpected race error: %v", err)
                }
        }
        if winner != 1 || loser != 1 {
                t.Fatalf("race outcome winner=%d loser=%d, want 1/1", winner, loser)
        }

        // Exactly one manifest row exists for the version.
        ctx := context.Background()
        var stored int
        if err := adminPool.QueryRow(ctx, "select count(*) from employee.release_manifests where tenant_id=$1 and employee_id=$2 and config_version=$3", w.tenant, employeeID, version.Version).Scan(&stored); err != nil {
                t.Fatal(err)
        }
        if stored != 1 {
                t.Fatalf("stored manifests = %d, want exactly 1", stored)
        }

        // A fresh sequential re-seal of the same version is refused honestly
        // even with the current revision (UNIQUE, no second manifest).
        current := conflict.Current
        if current == 0 {
                current = revision + 1
        }
        if _, err := w.seal(t, employeeID, version.Version, current); !errors.Is(err, employeeDomain.ErrManifestExists) {
                t.Fatalf("repeat seal = %v, want ErrManifestExists", err)
        }
}

// TestRestoreFromPastVersionReportsIncompatibilities: restore creates a new
// draft referencing the version, reports every drift, and never changes the
// past.
func TestRestoreFromPastVersionReportsIncompatibilities(t *testing.T) {
        w := newReleaseWorld(t, 97)
        ctx := context.Background()
        employeeID := w.createEmployee(t, true)
        rev := int64(0)
        draft := w.assemble(t, employeeID, rev) // rev 1
        version := w.createVersion(t, employeeID, draft.Revision, "Версия до дрейфа") // rev 2
        if _, err := w.seal(t, employeeID, version.Version, draft.Revision+1); err != nil { // rev 3
                t.Fatal(err)
        }

        // Drift: the pack pins and the behavior moves on after the seal
        // (the employee draft was at version 2 after the brief save).
        w.pinPack(t, employeeID)
        w.saveBehavior(t, employeeID, 3)
        draft = w.assemble(t, employeeID, draft.Revision+2) // rev 4
        v2 := w.createVersion(t, employeeID, draft.Revision, "Версия после дрейфа") // rev 5
        if _, err := w.seal(t, employeeID, v2.Version, draft.Revision+1); err != nil { // rev 6
                t.Fatal(err)
        }

        result, err := w.employee.Restore(ctx, employeeAPI.RestoreCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employeeID, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000909"),
                ConfigVersion: version.Version, ExpectedRevision: draft.Revision + 2, // rev 6
        })
        if err != nil {
                t.Fatal(err)
        }
        if result.Draft.RestoredFrom == nil || result.Draft.RestoredFrom.Version != version.Version {
                t.Fatalf("restored draft source = %+v", result.Draft.RestoredFrom)
        }
        joined := ""
        for _, issue := range result.Issues {
                joined += issue.Area + " "
                if issue.Subject == "" || issue.Problem == "" {
                        t.Fatalf("issue must name subject and problem: %+v", issue)
                }
        }
        for _, area := range []string{"behavior", "brief"} {
                if !contains(joined, area) {
                        t.Fatalf("restore report must mention %q, issues: %+v", area, result.Issues)
                }
        }

        // The past is untouched: the first manifest still reads with its digest.
        manifest, err := w.employee.ReadReleaseManifest(ctx, w.owner, w.tenant, employeeID, version.Version)
        if err != nil || manifest.Quarantine != employeeDomain.QuarantineNone {
                t.Fatalf("restored past changed: %+v err=%v", manifest, err)
        }
        if manifest.Content == nil || manifest.Content.Config.Version != version.Version {
                t.Fatalf("manifest content lost after restore: %+v", manifest)
        }

        // Version numbers stay monotonic after the restore.
        v3 := w.createVersion(t, employeeID, result.Draft.Revision, "Версия после восстановления")
        if v3.Version != 3 {
                t.Fatalf("version after restore = %d, want 3", v3.Version)
        }
}

// TestManifestQuarantineOnTamperedRows: tampering with the stored payload
// surfaces as explicit quarantine — unknown schema version, unknown field
// and digest mismatch — with the payload kept, never silently discarded.
func TestManifestQuarantineOnTamperedRows(t *testing.T) {
        w := newReleaseWorld(t, 98)
        ctx := context.Background()
        employeeID := w.createEmployee(t, false)
        draft := w.assemble(t, employeeID, 0)
        version := w.createVersion(t, employeeID, draft.Revision, "Карантин")
        manifest, err := w.seal(t, employeeID, version.Version, draft.Revision+1)
        if err != nil {
                t.Fatal(err)
        }
        var content []byte
        if err = adminPool.QueryRow(ctx, "select content from employee.release_manifests where tenant_id=$1 and employee_id=$2 and config_version=$3", w.tenant, employeeID, version.Version).Scan(&content); err != nil {
                t.Fatal(err)
        }

        // Unknown schema version.
        tampered := map[string]any{}
        if err = json.Unmarshal(content, &tampered); err != nil {
                t.Fatal(err)
        }
        tampered["schema_version"] = 99
        writeManifestContent(t, ctx, w.tenant, employeeID, version.Version, tampered)
        view, err := w.employee.ReadReleaseManifest(ctx, w.owner, w.tenant, employeeID, version.Version)
        if err != nil || view.Quarantine != employeeDomain.QuarantineSchemaVersion || view.Content != nil || view.Raw == "" {
                t.Fatalf("schema quarantine = %+v err=%v", view, err)
        }

        // Unknown field inside the current schema version.
        tampered["schema_version"] = employeeDomain.ManifestSchemaCurrent
        tampered["model_route_override"] = "silent replacement"
        writeManifestContent(t, ctx, w.tenant, employeeID, version.Version, tampered)
        view, err = w.employee.ReadReleaseManifest(ctx, w.owner, w.tenant, employeeID, version.Version)
        if err != nil || view.Quarantine != employeeDomain.QuarantineUnknownField {
                t.Fatalf("field quarantine = %+v err=%v", view, err)
        }

        // Tampered configuration with the old digest: integrity quarantine.
        tampered = map[string]any{}
        if err = json.Unmarshal(content, &tampered); err != nil {
                t.Fatal(err)
        }
        tampered["configuration"] = map[string]any{"version": 2, "description": "подмена"}
        writeManifestContent(t, ctx, w.tenant, employeeID, version.Version, tampered)
        view, err = w.employee.ReadReleaseManifest(ctx, w.owner, w.tenant, employeeID, version.Version)
        if err != nil || view.Quarantine != employeeDomain.QuarantineDigestMismatch || view.Digest != manifest.Digest {
                t.Fatalf("digest quarantine = %+v err=%v", view, err)
        }
}

func writeManifestContent(t *testing.T, ctx context.Context, tenant, employee id.UUID, version int64, content map[string]any) {
        t.Helper()
        raw, err := json.Marshal(content)
        if err != nil {
                t.Fatal(err)
        }
        if _, err = adminPool.Exec(ctx, "update employee.release_manifests set content=$4::jsonb where tenant_id=$1 and employee_id=$2 and config_version=$3", tenant, employee, version, string(raw)); err != nil {
                t.Fatal(err)
        }
}

// TestReleaseScopeAndTenantBoundaries: members without scopes get an
// honest refusal; review members view but cannot manage; foreign tenants
// get a plain 404 (RLS).
func TestReleaseScopeAndTenantBoundaries(t *testing.T) {
        w := newReleaseWorld(t, 99)
        ctx := context.Background()
        employeeID := w.createEmployee(t, false)
        draft := w.assemble(t, employeeID, 0)
        version := w.createVersion(t, employeeID, draft.Revision, "Полномочия")
        if _, err := w.seal(t, employeeID, version.Version, draft.Revision+1); err != nil {
                t.Fatal(err)
        }

        // An operator member (operations/claims) is a member of the tenant but
        // holds no release scope: an honest refusal, not a leak.
        operator := inviteMember(t, w.identity, w.owner, w.tenant, "release-operator@example.test", identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations, identityDomain.Claims}, "0000000090a1", "0000000090a2")
        if _, err := w.employee.ReleaseView(ctx, operator, w.tenant, employeeID); !errors.Is(err, employeeDomain.ErrReleaseForbidden) {
                t.Fatalf("operator view = %v, want honest refusal", err)
        }

        // An expert QA member (review/audit) views but cannot manage.
        reviewer := inviteMember(t, w.identity, w.owner, w.tenant, "release-reviewer@example.test", identityDomain.ExpertQA, []identityDomain.Scope{identityDomain.Review, identityDomain.TechnicalExpertise, identityDomain.Audit}, "0000000090a3", "0000000090a4")
        view, err := w.employee.ReleaseView(ctx, reviewer, w.tenant, employeeID)
        if err != nil || view.CanManage {
                t.Fatalf("reviewer view = %+v err=%v", view, err)
        }
        if view.Latest == nil || view.Latest.Quarantine != employeeDomain.QuarantineNone {
                t.Fatalf("reviewer must read the sealed manifest: %+v", view.Latest)
        }
        if _, err := w.employee.AssembleDraft(ctx, employeeAPI.AssembleCommand{
                Actor: reviewer, TenantID: w.tenant, EmployeeID: employeeID, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000090a"),
                ExpectedRevision: 0,
        }); !errors.Is(err, employeeDomain.ErrReleaseForbidden) {
                t.Fatalf("reviewer manage = %v, want honest refusal", err)
        }

        // A foreign principal gets a plain 404: the tenant is not disclosed.
        otherIdentity, otherOwner := newService("0000000099a1", "0000000099a2", "0000000099a3")
        otherTenant := createTenant(t, otherIdentity, otherOwner, "Other release tenant", "release-other-99", "0000000099a4").Tenant.ID
        if _, err := w.employee.ReleaseView(ctx, otherOwner, otherTenant, employeeID); !errors.Is(err, employeeDomain.ErrNotFound) {
                t.Fatalf("foreign view = %v, want not found", err)
        }
        _ = otherTenant
}

// inviteMember creates one member with the given roles and scopes.
func inviteMember(t *testing.T, identity *identityApp.Service, owner identityDomain.Principal, tenant id.UUID, email string, role identityDomain.Role, scopes []identityDomain.Scope, correlationSuffix, memberSuffix string) identityDomain.Principal {
        t.Helper()
        correlation := id.MustParse("01994f64-6c00-7000-8000-" + correlationSuffix)
        _, token, err := identity.InviteMember(context.Background(), api.MembershipCommand{
                Actor: owner, TenantID: tenant, Email: email, Roles: []identityDomain.Role{role}, Scopes: scopes, CorrelationID: correlation,
        })
        if err != nil {
                t.Fatal(err)
        }
        member := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-" + memberSuffix), VerifiedEmail: email}
        if _, err = identity.AcceptInvitation(context.Background(), member, token, correlation); err != nil {
                t.Fatal(err)
        }
        return member
}

// TestReleaseWriteAuditAndGrants: every write lands in audit, and the
// runtime role cannot mutate the append-only tables.
func TestReleaseWriteAuditAndGrants(t *testing.T) {
        w := newReleaseWorld(t, 100)
        ctx := context.Background()
        employeeID := w.createEmployee(t, false)
        draft := w.assemble(t, employeeID, 0)
        version := w.createVersion(t, employeeID, draft.Revision, "Аудит")
        if _, err := w.seal(t, employeeID, version.Version, draft.Revision+1); err != nil {
                t.Fatal(err)
        }
        for outcome, want := range map[string]int{
                "release_draft_saved":    1,
                "config_version_created": 1,
                "manifest_sealed":        1,
        } {
                var count int
                if err := adminPool.QueryRow(ctx, "select count(*) from employee.audit where tenant_id=$1 and employee_id=$2 and outcome=$3", w.tenant, employeeID, outcome).Scan(&count); err != nil {
                        t.Fatal(err)
                }
                if count != want {
                        t.Fatalf("audit %s = %d, want %d", outcome, count, want)
                }
        }
        // The runtime role has no UPDATE/DELETE on the append-only tables.
        if _, err := runtimePool.Exec(ctx, "update employee.config_versions set description='forge' where tenant_id=$1", w.tenant); err == nil {
                t.Fatal("config_versions must reject UPDATE for staffy_app")
        }
        if _, err := runtimePool.Exec(ctx, "delete from employee.release_manifests where tenant_id=$1", w.tenant); err == nil {
                t.Fatal("release_manifests must reject DELETE for staffy_app")
        }
        // Cross-tenant write is blocked by RLS: the transaction carries the real
        // tenant setting (transaction-scoped, the pooled connection never leaks
        // it), and the policy rejects the foreign tenant_id row.
        rlsTx, err := runtimePool.Begin(ctx)
        if err != nil {
                t.Fatal(err)
        }
        defer rlsTx.Rollback(ctx)
        if _, err = rlsTx.Exec(ctx, "select set_config('staffy.tenant_id',$1,true)", w.tenant.String()); err != nil {
                t.Fatal(err)
        }
        foreign := id.MustParse("01994f64-6c00-7000-8000-0000000000ff")
        foreignVersionID, err := id.NewV7(time.Now().UTC())
        if err != nil {
                t.Fatal(err)
        }
        if _, err = rlsTx.Exec(ctx, "insert into employee.config_versions(tenant_id,employee_id,version_id,version,refs,description,actor_id,created_at) values($1,$2,$3,1,'{}'::jsonb,'foreign',$4,now())", foreign, employeeID, foreignVersionID, w.owner.ID); err == nil {
                t.Fatal("RLS must block cross-tenant inserts")
        } else if !strings.Contains(err.Error(), "row-level security") {
                // Discriminate the honest cause: the policy rejected the row,
                // not a constraint or an argument mismatch.
                t.Fatalf("cross-tenant insert must fail on the RLS policy, got: %v", err)
        }
}

// TestConfiguratorSealsManifestThroughFactsPort: a member holding only the
// configuration scope (a Configurator, no integrations) can seal — the
// connection facts cross through the caller-authority port without a
// second scope check, and only the safe projection lands in the manifest.
func TestConfiguratorSealsManifestThroughFactsPort(t *testing.T) {
        w := newReleaseWorld(t, 101)
        ctx := context.Background()
        employeeID := w.createEmployee(t, false)

        // A sandbox connection exists but is never verified: its capabilities
        // stay honest gaps in the manifest.
        created, err := w.connector.Create(ctx, connectorAPI.CreateCommand{
                Actor: w.owner, TenantID: w.tenant, IdempotencyKey: "release-connection-101",
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000911"),
                ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.2.0",
                Label: "Телефония Олега", Environment: connectorDomain.EnvSandbox,
                Config: map[string]string{"endpoint": "https://synthetic.example.test"},
        })
        if err != nil {
                t.Fatal(err)
        }

        draft := w.assemble(t, employeeID, 0)
        version := w.createVersion(t, employeeID, draft.Revision, "Конфигуратор запечатывает")

        // The Configurator holds configuration without integrations: sealing
        // must work for them by the spec (the facts port is caller-authority).
        configurator := inviteMember(t, w.identity, w.owner, w.tenant, "release-configurator@example.test",
                identityDomain.Configurator, []identityDomain.Scope{identityDomain.Configuration}, "0000000090c1", "0000000090c2")
        if err := w.identity.Authorize(ctx, configurator, w.tenant, identityDomain.Integrations); err == nil {
                t.Fatal("precondition: configurator must NOT hold the integrations scope")
        }
        draft, err = w.employee.AssembleDraft(ctx, employeeAPI.AssembleCommand{
                Actor: configurator, TenantID: w.tenant, EmployeeID: employeeID,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000912"), ExpectedRevision: draft.Revision + 1,
        })
        if err != nil {
                t.Fatal(err)
        }
        // The sealed manifest carries the connection with honest gap states and
        // no secret material.
        manifest, err := w.employee.SealManifest(ctx, employeeAPI.SealCommand{
                Actor: configurator, TenantID: w.tenant, EmployeeID: employeeID,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000913"),
                ConfigVersion: version.Version, ExpectedRevision: draft.Revision,
        })
        if err != nil {
                t.Fatalf("configurator seal = %v, want success through the facts port", err)
        }
        if manifest.Status != employeeDomain.ManifestStatusIncomplete {
                t.Fatalf("status = %q, want incomplete (capability gaps)", manifest.Status)
        }
        found := false
        for _, conn := range manifest.Content.Connections {
                if conn.ConnectionID != created.Connection.ConnectionID {
                        continue
                }
                found = true
                if conn.Status != connectorDomain.StatusDraft {
                        t.Fatalf("connection status = %q, want draft", conn.Status)
                }
                if len(conn.Capabilities) == 0 {
                        t.Fatal("manifest must list the declared capabilities as honest gaps")
                }
                for _, capability := range conn.Capabilities {
                        if !capability.Gap {
                                t.Fatalf("capability %q must be an honest gap for an unverified connection", capability.Capability)
                        }
                }
                joined, _ := json.Marshal(conn)
                if strings.Contains(string(joined), "synthetic.example.test") || strings.Contains(string(joined), "secret") {
                        t.Fatalf("manifest leaked connection material: %s", joined)
                }
        }
        if !found {
                t.Fatalf("sealed manifest must include the tenant connection: %+v", manifest.Content.Connections)
        }
}

// TestRestoreReportsQuarantinedManifest: a quarantined source manifest
// leaves an explicit note in the restore report instead of silently
// skipping the connection comparison.
func TestRestoreReportsQuarantinedManifest(t *testing.T) {
        w := newReleaseWorld(t, 102)
        ctx := context.Background()
        employeeID := w.createEmployee(t, false)
        draft := w.assemble(t, employeeID, 0)
        version := w.createVersion(t, employeeID, draft.Revision, "Источник карантина")
        if _, err := w.seal(t, employeeID, version.Version, draft.Revision+1); err != nil {
                t.Fatal(err)
        }
        // Tamper the sealed payload into an unknown schema version.
        writeManifestContent(t, ctx, w.tenant, employeeID, version.Version, map[string]any{"schema_version": 77, "configuration": map[string]any{}})

        result, err := w.employee.Restore(ctx, employeeAPI.RestoreCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employeeID,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000914"),
                ConfigVersion: version.Version, ExpectedRevision: draft.Revision + 2,
        })
        if err != nil {
                t.Fatal(err)
        }
        found := false
        for _, issue := range result.Issues {
                if issue.Area == "manifest" && contains(issue.Problem, "карантине") {
                        found = true
                }
        }
        if !found {
                t.Fatalf("restore report must mention the quarantined manifest: %+v", result.Issues)
        }
}
