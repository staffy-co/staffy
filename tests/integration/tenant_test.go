//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/app"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/identity/store"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	adminPool   *pgxpool.Pool
	runtimePool *pgxpool.Pool
	container   *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	adminURL := os.Getenv("STAFFY_TEST_DATABASE_URL")
	var err error
	if adminURL == "" {
		container, err = postgres.Run(ctx, "postgres:18.6",
			postgres.WithDatabase("staffy"),
			postgres.WithUsername("postgres"),
			postgres.WithPassword("staffy-admin-test"),
			testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)),
		)
		if err != nil {
			panic(err)
		}
		adminURL, err = container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			panic(err)
		}
	}
	adminConfig, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		panic(err)
	}
	adminConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	adminPool, err = pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		panic(err)
	}
	if os.Getenv("STAFFY_TEST_DATABASE_URL") != "" {
		if _, err = adminPool.Exec(ctx, "drop schema if exists employee cascade; drop schema if exists identity cascade; drop schema if exists connector cascade; drop schema if exists privacy cascade; drop schema if exists entitlement cascade; drop schema if exists cases cascade; drop role if exists staffy_app"); err != nil {
			panic(err)
		}
	}
	migration, err := os.ReadFile("../../migrations/identity/00001_identity_tenant.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, string(migration)); err != nil {
		panic(err)
	}
	membershipMigration, err := os.ReadFile("../../migrations/identity/00002_identity_membership.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(membershipMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	employeeMigration, err := os.ReadFile("../../migrations/employee/00001_employee_draft.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(employeeMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	packMigration, err := os.ReadFile("../../migrations/employee/00002_domain_pack.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(packMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	knowledgeMigration, err := os.ReadFile("../../migrations/employee/00003_knowledge_sources.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(knowledgeMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	ingestMigration, err := os.ReadFile("../../migrations/employee/00004_safe_source_ingestion.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(ingestMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	behaviorMigration, err := os.ReadFile("../../migrations/employee/00005_behavior.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(behaviorMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	voiceMigration, err := os.ReadFile("../../migrations/employee/00006_voice.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(voiceMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	voicePreviewMigration, err := os.ReadFile("../../migrations/employee/00007_voice_preview.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(voicePreviewMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	voiceRetentionMigration, err := os.ReadFile("../../migrations/employee/00008_voice_retention.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(voiceRetentionMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	connectorMigration, err := os.ReadFile("../../migrations/connector/00001_connections.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(connectorMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	capabilityMigration, err := os.ReadFile("../../migrations/connector/00002_capability_evidence.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(capabilityMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	employeePurgeMigration, err := os.ReadFile("../../migrations/employee/00009_data_perimeter_purge.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(employeePurgeMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	identityPurgeMigration, err := os.ReadFile("../../migrations/identity/00003_data_perimeter_purge.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(identityPurgeMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	identityCommercialMigration, err := os.ReadFile("../../migrations/identity/00004_entitlement_commercial.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(identityCommercialMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	privacyMigration, err := os.ReadFile("../../migrations/privacy/00001_data_perimeter.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(privacyMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	entitlementMigration, err := os.ReadFile("../../migrations/entitlement/00001_entitlement_usage.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(entitlementMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	releaseMigration, err := os.ReadFile("../../migrations/employee/00010_release_manifest.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(releaseMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	profileMigration, err := os.ReadFile("../../migrations/employee/00011_operating_profile.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(profileMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	readinessMigration, err := os.ReadFile("../../migrations/employee/00012_readiness_publication.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(readinessMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesMigration, err := os.ReadFile("../../migrations/cases/00001_channel_event_case.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesWorkMigration, err := os.ReadFile("../../migrations/cases/00002_work_items_facts.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesWorkMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesVerificationMigration, err := os.ReadFile("../../migrations/cases/00003_identity_verification.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesVerificationMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesOwnerMigration, err := os.ReadFile("../../migrations/cases/00004_response_owner.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesOwnerMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesModeMigration, err := os.ReadFile("../../migrations/cases/00005_conversation_mode.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesModeMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesPolicyMigration, err := os.ReadFile("../../migrations/cases/00006_progressive_policy.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesPolicyMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesObservationMigration, err := os.ReadFile("../../migrations/cases/00007_owning_system_observations.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesObservationMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesObjectionMigration, err := os.ReadFile("../../migrations/cases/00008_objections.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesObjectionMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesCommitmentMigration, err := os.ReadFile("../../migrations/cases/00009_commitments.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesCommitmentMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	casesContinuationMigration, err := os.ReadFile("../../migrations/cases/00010_continuations.sql")
	if err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, strings.Split(string(casesContinuationMigration), "-- +goose Down")[0]); err != nil {
		panic(err)
	}
	if _, err = adminPool.Exec(ctx, "alter role staffy_app login password 'staffy-runtime-test'"); err != nil {
		panic(err)
	}
	runtimeURL, err := url.Parse(adminURL)
	if err != nil {
		panic(err)
	}
	runtimeURL.User = url.UserPassword("staffy_app", "staffy-runtime-test")
	runtimePool, err = pgxpool.New(ctx, runtimeURL.String())
	if err != nil {
		panic(err)
	}
	if err = runtimePool.Ping(ctx); err != nil {
		panic(err)
	}

	code := m.Run()
	runtimePool.Close()
	adminPool.Close()
	if container != nil {
		_ = testcontainers.TerminateContainer(container)
	}
	os.Exit(code)
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type sequenceIDs struct {
	mu     sync.Mutex
	values []id.UUID
	index  int
}

// New hands out the next UUID of the fixed sequence. The hand-out is
// goroutine-safe (the mutex of the 2026-10-14 fix, deferred-work): the
// race tests share ONE generator across concurrent command calls — the
// racy index read of the old code handed the SAME UUID to two goroutines
// (the honest «duplicate key policy_outcomes_pkey» of the PR #24 CI
// flake). The assignment order under concurrency stays arbitrary — no
// caller depends on WHICH value it gets, only on uniqueness.
func (g *sequenceIDs) New(time.Time) (id.UUID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.index >= len(g.values) {
		return "", errors.New("no test UUID left")
	}
	value := g.values[g.index]
	g.index++
	return value, nil
}

// TestSequenceIDsConcurrentHandoutStaysUnique pins the harness invariant
// that closed the CI flake of 2026-10-13 (PR #24, deferred-work): the
// fixed-sequence generator hands out every UUID EXACTLY once — under the
// concurrent callers too (the raced EvaluatePolicy of
// TestPolicyTerminalWorkItemAndRaces shares ONE sequence; a racy index
// read hands the SAME UUID to two goroutines — the honest «duplicate key
// policy_outcomes_pkey» of the CI evidence). The tight-loop probe makes
// the overlap deterministic where the DB-race timing of the policy test
// stays a lottery.
func TestSequenceIDsConcurrentHandoutStaysUnique(t *testing.T) {
	values := make([]id.UUID, 64)
	for i := range values {
		values[i] = id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", i+1))
	}
	generator := &sequenceIDs{values: values}
	const workers = 8
	handed := make(chan id.UUID, len(values))
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				value, err := generator.New(time.Time{})
				if err != nil {
					return
				}
				handed <- value
			}
		}()
	}
	wg.Wait()
	close(handed)
	seen := make(map[id.UUID]bool, len(values))
	for value := range handed {
		if seen[value] {
			t.Fatalf("the concurrent hand-out duplicated the sequence value %s", value)
		}
		seen[value] = true
	}
	if len(seen) != len(values) {
		t.Fatalf("expected exactly %d unique hand-outs, got %d", len(values), len(seen))
	}
}

func newService(principalSuffix, tenantSuffix, eventSuffix string) (*app.Service, domain.Principal) {
	principalID := id.MustParse("01994f64-6c00-7000-8000-" + principalSuffix)
	tenantID := id.MustParse("01994f64-6c00-7000-8000-" + tenantSuffix)
	eventID := id.MustParse("01994f64-6c00-7000-8000-" + eventSuffix)
	return app.New(store.NewPostgres(runtimePool), fixedClock{time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)}, &sequenceIDs{values: []id.UUID{tenantID, eventID, tenantID, eventID, tenantID, eventID}}), domain.Principal{ID: principalID, CanCreateTenant: true}
}

func createTenant(t *testing.T, service *app.Service, principal domain.Principal, name, key, correlationSuffix string) api.CreateTenantResult {
	t.Helper()
	result, err := service.CreateTenant(context.Background(), api.CreateTenantCommand{
		Principal: principal, Name: name, IdempotencyKey: key,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-" + correlationSuffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCreateTenantIsAtomicIdempotentAndConflictSafe(t *testing.T) {
	service, principal := newService("000000000101", "000000000102", "000000000103")
	first := createTenant(t, service, principal, "Montagio", "request-atomic-01", "000000000104")
	second := createTenant(t, service, principal, "Montagio", "request-atomic-01", "000000000105")
	if first.Tenant.ID != second.Tenant.ID || !second.Replayed {
		t.Fatalf("idempotent replay mismatch: %#v %#v", first, second)
	}
	workspace, err := service.Workspace(context.Background(), principal, first.Tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Tenant.ID != first.Tenant.ID || workspace.Tenant.Name != "Montagio" || workspace.Role != "owner" {
		t.Fatalf("workspace mismatch: %#v", workspace)
	}
	if _, err := service.CreateTenant(context.Background(), api.CreateTenantCommand{
		Principal: principal, Name: "Другая компания", IdempotencyKey: "request-atomic-01",
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000106"),
	}); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("conflict error = %v", err)
	}

	var tenants, memberships, events, idempotency int
	for query, target := range map[string]*int{
		"select count(*) from identity.tenants where tenant_id = '01994f64-6c00-7000-8000-000000000102'":                                                  &tenants,
		"select count(*) from identity.memberships where tenant_id = '01994f64-6c00-7000-8000-000000000102' and role = 'owner'":                           &memberships,
		"select count(*) from identity.outbox_events where tenant_id = '01994f64-6c00-7000-8000-000000000102' and event_type = 'identity.tenant_created'": &events,
		"select count(*) from identity.idempotency_records where tenant_id = '01994f64-6c00-7000-8000-000000000102'":                                      &idempotency,
	} {
		if err := adminPool.QueryRow(context.Background(), query).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	if tenants != 1 || memberships != 1 || events != 1 || idempotency != 1 {
		t.Fatalf("counts: tenant=%d membership=%d event=%d idem=%d", tenants, memberships, events, idempotency)
	}
	var eventID, aggregateID, actorID, correlationID, payloadTenant, payloadActor, payloadRole string
	var schemaVersion int
	var occurredAt time.Time
	if err := adminPool.QueryRow(context.Background(), `
                select event_id::text, schema_version, aggregate_id::text, actor_id::text,
                       correlation_id::text, occurred_at,
                       payload->>'tenant_id', payload->>'actor_id', payload->>'role'
                from identity.outbox_events where tenant_id = $1`, first.Tenant.ID,
	).Scan(&eventID, &schemaVersion, &aggregateID, &actorID, &correlationID, &occurredAt, &payloadTenant, &payloadActor, &payloadRole); err != nil {
		t.Fatal(err)
	}
	if eventID != "01994f64-6c00-7000-8000-000000000103" || schemaVersion != 1 ||
		aggregateID != first.Tenant.ID.String() || actorID != principal.ID.String() ||
		correlationID != "01994f64-6c00-7000-8000-000000000104" ||
		!occurredAt.Equal(time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)) ||
		payloadTenant != first.Tenant.ID.String() || payloadActor != principal.ID.String() || payloadRole != "owner" {
		t.Fatalf("event contract mismatch: event=%s schema=%d aggregate=%s actor=%s correlation=%s occurred=%s payload=%s/%s/%s",
			eventID, schemaVersion, aggregateID, actorID, correlationID, occurredAt, payloadTenant, payloadActor, payloadRole)
	}
}

func TestCreateTenantRollsBackAllRecordsWhenOutboxWriteFails(t *testing.T) {
	ctx := context.Background()
	_, err := adminPool.Exec(ctx, `
                create or replace function identity.reject_test_outbox() returns trigger
                language plpgsql as $$ begin raise exception 'forced outbox failure'; end $$;
                create trigger reject_test_outbox before insert on identity.outbox_events
                for each row execute function identity.reject_test_outbox()`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), `
                        drop trigger if exists reject_test_outbox on identity.outbox_events;
                        drop function if exists identity.reject_test_outbox()`)
	})

	service, principal := newService("000000000121", "000000000122", "000000000123")
	_, err = service.CreateTenant(ctx, api.CreateTenantCommand{
		Principal: principal, Name: "Rollback Tenant", IdempotencyKey: "request-rollback-01",
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000124"),
	})
	if err == nil {
		t.Fatal("forced outbox failure unexpectedly succeeded")
	}

	var persisted int
	err = adminPool.QueryRow(ctx, `
                select
                        (select count(*) from identity.principals where principal_id = $1) +
                        (select count(*) from identity.tenants where tenant_id = $2) +
                        (select count(*) from identity.memberships where tenant_id = $2) +
                        (select count(*) from identity.idempotency_records where principal_id = $1) +
                        (select count(*) from identity.outbox_events where tenant_id = $2)`,
		principal.ID, "01994f64-6c00-7000-8000-000000000122",
	).Scan(&persisted)
	if err != nil {
		t.Fatal(err)
	}
	if persisted != 0 {
		t.Fatalf("transaction left %d record(s) after failure", persisted)
	}
}

func TestCrossTenantReadWriteAndForeignLinkAreDenied(t *testing.T) {
	serviceA, principalA := newService("000000000201", "000000000202", "000000000203")
	serviceB, principalB := newService("000000000211", "000000000212", "000000000213")
	tenantA := createTenant(t, serviceA, principalA, "Tenant A", "request-tenant-a", "000000000204").Tenant.ID
	tenantB := createTenant(t, serviceB, principalB, "Tenant B", "request-tenant-b", "000000000214").Tenant.ID
	if _, err := serviceA.Workspace(context.Background(), principalA, tenantB); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign workspace error = %v", err)
	}

	tx, err := runtimePool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), "select set_config('staffy.principal_id', $1, true), set_config('staffy.tenant_id', $2, true)", principalA.ID, tenantA); err != nil {
		t.Fatal(err)
	}
	var visible int
	if err = tx.QueryRow(context.Background(), "select count(*) from identity.tenants where tenant_id = $1", tenantB).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("foreign tenant visible: %d", visible)
	}
	for table, query := range map[string]string{
		"membership":  "select count(*) from identity.memberships where tenant_id = $1",
		"idempotency": "select count(*) from identity.idempotency_records where tenant_id = $1",
		"outbox":      "select count(*) from identity.outbox_events where tenant_id = $1",
	} {
		if err = tx.QueryRow(context.Background(), query, tenantB).Scan(&visible); err != nil {
			t.Fatal(err)
		}
		if visible != 0 {
			t.Fatalf("foreign %s visible: %d", table, visible)
		}
	}
	if _, err = tx.Exec(context.Background(), "insert into identity.memberships (tenant_id, principal_id, role, status, created_at) values ($1, $2, 'owner', 'active', now())", tenantB, principalA.ID); err == nil {
		t.Fatal("cross-tenant membership insert succeeded")
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}

	tx, err = runtimePool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), "select set_config('staffy.principal_id', $1, true), set_config('staffy.tenant_id', $2, true)", principalA.ID, tenantA); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `
                insert into identity.idempotency_records
                (principal_id, operation, idempotency_key, request_hash, tenant_id, response_body, created_at)
                values ($1, 'test.cross_tenant', 'cross-tenant-key', repeat('a', 64), $2, '{}', now())`, principalA.ID, tenantB); err == nil {
		t.Fatal("cross-tenant idempotency insert succeeded")
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}

	tx, err = runtimePool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), "select set_config('staffy.principal_id', $1, true), set_config('staffy.tenant_id', $2, true)", principalA.ID, tenantA); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), `
                insert into identity.outbox_events
                (tenant_id, event_id, event_type, schema_version, aggregate_id, actor_id, correlation_id, occurred_at, payload)
                values ($1, '01994f64-6c00-7000-8000-000000000205', 'identity.tenant_created', 1, $2, $3,
                        '01994f64-6c00-7000-8000-000000000206', now(), '{}')`, tenantA, tenantB, principalA.ID); err == nil {
		t.Fatal("cross-tenant outbox aggregate insert succeeded")
	}
}

func TestMissingContextFailsBeforeDatabaseAccess(t *testing.T) {
	closed, err := pgxpool.New(context.Background(), "postgres://invalid:invalid@127.0.0.1:1/invalid")
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	_, err = store.NewPostgres(closed).Workspace(context.Background(), tenancy.Context{})
	if !errors.Is(err, tenancy.ErrMissingContext) {
		t.Fatalf("error = %v, want missing context", err)
	}
}

func TestPoolPreparedStatementAndRollbackDoNotLeakTenantScope(t *testing.T) {
	serviceA, principalA := newService("000000000301", "000000000302", "000000000303")
	serviceB, principalB := newService("000000000311", "000000000312", "000000000313")
	tenantA := createTenant(t, serviceA, principalA, "Pool A", "request-pool-a1", "000000000304").Tenant.ID
	tenantB := createTenant(t, serviceB, principalB, "Pool B", "request-pool-b1", "000000000314").Tenant.ID

	conn, err := runtimePool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err = conn.Conn().Prepare(context.Background(), "visible_tenants", "select tenant_id::text from identity.tenants order by tenant_id"); err != nil {
		t.Fatal(err)
	}
	assertVisible := func(principalID, tenantID id.UUID, expected string) {
		t.Helper()
		tx, beginErr := conn.Begin(context.Background())
		if beginErr != nil {
			t.Fatal(beginErr)
		}
		defer tx.Rollback(context.Background())
		if _, execErr := tx.Exec(context.Background(), "select set_config('staffy.principal_id', $1, true), set_config('staffy.tenant_id', $2, true)", principalID, tenantID); execErr != nil {
			t.Fatal(execErr)
		}
		var got string
		if queryErr := tx.QueryRow(context.Background(), "visible_tenants").Scan(&got); queryErr != nil {
			t.Fatal(queryErr)
		}
		if got != expected {
			t.Fatalf("visible tenant = %s, want %s", got, expected)
		}
		if rollbackErr := tx.Rollback(context.Background()); rollbackErr != nil {
			t.Fatal(rollbackErr)
		}
	}
	assertVisible(principalA.ID, tenantA, tenantA.String())
	assertVisible(principalB.ID, tenantB, tenantB.String())

	tx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	var count int
	if err = tx.QueryRow(context.Background(), "select count(*) from identity.tenants").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("tenant context leaked after rollback: %d rows", count)
	}
}
