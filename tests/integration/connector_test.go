//go:build integration

package integration_test

import (
        "context"
        "errors"
        "testing"
        "time"

        connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
        connectorApp "github.com/artemiosu/staffy/internal/connector/app"
        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        connectorStore "github.com/artemiosu/staffy/internal/connector/store"
        "github.com/artemiosu/staffy/internal/connector/verifier"
        identityApp "github.com/artemiosu/staffy/internal/identity/app"
        "github.com/artemiosu/staffy/internal/platform/id"
)

func connectorService(identity *identityApp.Service, ids employeeIDs) *connectorApp.Service {
        return connectorApp.New(connectorStore.NewPostgres(runtimePool), identity, verifier.Synthetic{}, fixedClock{time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}, ids)
}

func TestConnectionLifecycleCatalogRLSIdempotencyAndSecretSealing(t *testing.T) {
        ctx := context.Background()
        identity, owner := newService("00000000e101", "00000000e102", "00000000e103")
        tenant := createTenant(t, identity, owner, "Connector test tenant", "connector-tenant-key", "00000000e104").Tenant.ID
        otherIdentity, otherOwner := newService("00000000e111", "00000000e112", "00000000e113")
        otherTenant := createTenant(t, otherIdentity, otherOwner, "Other connector tenant", "connector-other-key", "00000000e114").Tenant.ID

        ids := employeeIDs{}
        corr := id.MustParse("01994f64-6c00-7000-8000-00000000e301")
        svc := connectorService(identity, ids)

        // Catalog is tenant-scoped and only the synthetic connector is connectable.
        defs, err := svc.Catalog(ctx, owner, tenant)
        if err != nil || len(defs) < 7 {
                t.Fatalf("catalog=%d err=%v", len(defs), err)
        }
        for _, d := range defs {
                if d.ID != "staffy-synthetic-telephony" && d.Connectable() {
                        t.Fatalf("%s must not be connectable", d.ID)
                }
        }

        // Unimplemented connector is rejected honestly.
        _, err = svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "conn-key-novofon", CorrelationID: corr, ConnectorID: "novofon-sip", ConnectorVer: "1.0.0", Label: "Novofon", Environment: connectorDomain.EnvSandbox, Config: map[string]string{}})
        if err == nil {
                t.Fatal("unimplemented connector must be rejected")
        }

        // Production is fail-closed.
        _, err = svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "conn-key-prod", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.0.0", Label: "Prod", Environment: connectorDomain.EnvProduct, Config: map[string]string{"endpoint": "https://demo.sandbox.example"}})
        if !errors.Is(err, connectorDomain.ErrProductionBlocked) {
                t.Fatalf("production must be blocked: %v", err)
        }

        createCmd := connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "conn-key-create", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.0.0", Label: "Тестовая телефония", Environment: connectorDomain.EnvSandbox, Config: map[string]string{"endpoint": "https://demo.sandbox.example"}}
        created, err := svc.Create(ctx, createCmd)
        if err != nil || created.Connection.Status != connectorDomain.StatusDraft || created.Connection.Version != 1 {
                t.Fatalf("create=%+v err=%v", created.Connection, err)
        }
        connectionID := created.Connection.ConnectionID

        // Idempotent replay returns the same connection without a duplicate.
        replayed, err := svc.Create(ctx, createCmd)
        if err != nil || !replayed.Replayed || replayed.Connection.ConnectionID != connectionID {
                t.Fatalf("replay=%+v err=%v", replayed, err)
        }
        // Same key with different data conflicts.
        conflictCmd := createCmd
        conflictCmd.Label = "Другое назначение"
        if _, err = svc.Create(ctx, conflictCmd); !errors.Is(err, connectorDomain.ErrIdempotencyConflict) {
                t.Fatalf("idempotency conflict=%v", err)
        }

        // Cross-tenant access is closed by RLS + authorization scope.
        if _, err = svc.Read(ctx, otherOwner, otherTenant, connectionID); !errors.Is(err, connectorDomain.ErrNotFound) {
                t.Fatalf("cross-tenant read=%v", err)
        }
        if _, err = svc.List(ctx, otherOwner, tenant); !errors.Is(err, connectorDomain.ErrNotFound) {
                t.Fatalf("cross-scope list=%v", err)
        }

        // A second account of the same connector is fully independent.
        second, err := svc2For(identity, employeeIDs{}).Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "conn-key-second", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.0.0", Label: "Второй аккаунт", Environment: connectorDomain.EnvTest, Config: map[string]string{"endpoint": "https://two.test.example"}})
        if err != nil || second.Connection.ConnectionID == connectionID {
                t.Fatalf("second account=%+v err=%v", second.Connection, err)
        }

        // Failed basic check is recorded honestly.
        failed, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: created.Connection.Version, Secrets: map[string]string{"api_key": "real-secret-bad-format"}})
        if err != nil || failed.Status != connectorDomain.StatusFailed || failed.AuthState != connectorDomain.CheckFailed || failed.Transport != connectorDomain.CheckVerified {
                t.Fatalf("failed connect=%+v err=%v", failed, err)
        }

        // Successful check stays connected/unverified — capabilities are not claimed.
        connected, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: failed.Version, Secrets: map[string]string{"api_key": "sandbox-integration-key"}})
        if err != nil || connected.Status != connectorDomain.StatusConnectedUnverified || connected.AuthState != connectorDomain.CheckVerified || connected.Transport != connectorDomain.CheckVerified {
                t.Fatalf("connect=%+v err=%v", connected, err)
        }
        if !connected.SecretActive || connected.SecretHint == "" || connected.SecretID == connectionID {
                t.Fatalf("secret bookkeeping=%+v", connected)
        }

        // Reads never expose the secret: config stays clean, hint is short.
        read, err := svc.Read(ctx, owner, tenant, connectionID)
        if err != nil || read.Config["api_key"] != "" || len(read.SecretHint) != 12 {
                t.Fatalf("read=%+v err=%v", read, err)
        }
        if _, err = svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: connected.Version, Secrets: map[string]string{"api_key": "sandbox-other-value"}}); err == nil {
                t.Fatal("double connect must be rejected; use rotation")
        }

        // CAS conflicts are recorded.
        if _, err = svc.Save(ctx, connectorAPI.SaveCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: 1, Config: map[string]string{"endpoint": "https://new.sandbox.example"}, Label: connected.Label, ChangeSummary: "Устаревшая версия"}); !errors.Is(err, connectorDomain.ErrConflict) {
                t.Fatalf("stale CAS=%v", err)
        }

        // Config change resets the honest states to draft.
        saved, err := svc.Save(ctx, connectorAPI.SaveCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: connected.Version, Config: map[string]string{"endpoint": "https://new.sandbox.example"}, Label: connected.Label, ChangeSummary: "Новый endpoint"})
        if err != nil || saved.Status != connectorDomain.StatusDraft || saved.AuthState != connectorDomain.CheckPending || saved.Transport != connectorDomain.CheckPending {
                t.Fatalf("save=%+v err=%v", saved, err)
        }

        // Rotation revokes the previous seal and re-checks honestly.
        rotated, err := svc.Rotate(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: saved.Version, Secrets: map[string]string{"api_key": "sandbox-rotated-key"}})
        if err != nil || rotated.Status != connectorDomain.StatusConnectedUnverified {
                t.Fatalf("rotate=%+v err=%v", rotated, err)
        }

        // Version history is immutable and ordered.
        versions, err := svc.ReadVersions(ctx, owner, tenant, connectionID)
        if err != nil || len(versions) < 5 {
                t.Fatalf("versions=%d err=%v", len(versions), err)
        }
        if versions[0].Version != rotated.Version {
                t.Fatalf("latest version=%d want %d", versions[0].Version, rotated.Version)
        }

        // Revocation is terminal and revokes the seal.
        revoked, err := svc.Revoke(ctx, connectorAPI.RevokeCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: rotated.Version, Reason: "Тест завершён"})
        if err != nil || revoked.Status != connectorDomain.StatusRevoked || revoked.SecretActive {
                t.Fatalf("revoke=%+v err=%v", revoked, err)
        }
        if _, err = svc.Revoke(ctx, connectorAPI.RevokeCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: revoked.Version, Reason: "Повтор"}); err == nil {
                t.Fatal("revoked connection must reject further revocations")
        }
        if _, err = svc.Save(ctx, connectorAPI.SaveCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: revoked.Version, Config: revoked.Config, Label: revoked.Label, ChangeSummary: "Попытка"}); err == nil {
                t.Fatal("revoked connection must reject config changes")
        }

        // Revocation revokes every remaining seal; nothing stays active.
        var active, revokedSeals int
        if err = adminPool.QueryRow(ctx, "select count(*) filter (where revoked_at is null), count(*) filter (where revoked_at is not null) from connector.secret_seals where tenant_id=$1 and connection_id=$2", tenant, connectionID).Scan(&active, &revokedSeals); err != nil {
                t.Fatal(err)
        }
        if active != 0 || revokedSeals != 2 {
                t.Fatalf("active=%d revoked=%d (want 0/2: only verified secrets are sealed)", active, revokedSeals)
        }

        // Audit and outbox never contain secret material: audit has no payload
        // column at all, and outbox payloads only carry opaque references.
        var leak int
        if err = adminPool.QueryRow(ctx, `select count(*) from connector.outbox_events where tenant_id=$1 and (payload::text like '%sandbox-integration-key%' or payload::text like '%sandbox-rotated-key%' or payload::text like '%real-secret-bad%')`, tenant).Scan(&leak); err != nil {
                t.Fatal(err)
        }
        if leak != 0 {
                t.Fatalf("secret leaked into outbox: %d rows", leak)
        }
        var auditRows, outboxRows int
        if err = adminPool.QueryRow(ctx, "select count(*) from connector.audit where tenant_id=$1", tenant).Scan(&auditRows); err != nil {
                t.Fatal(err)
        }
        if err = adminPool.QueryRow(ctx, "select count(*) from connector.outbox_events where tenant_id=$1", tenant).Scan(&outboxRows); err != nil {
                t.Fatal(err)
        }
        if auditRows == 0 || outboxRows == 0 {
                t.Fatalf("audit=%d outbox=%d", auditRows, outboxRows)
        }

        // The runtime role cannot delete history.
        if _, err = runtimePool.Exec(ctx, "delete from connector.connections where tenant_id=$1", tenant); err == nil {
                t.Fatal("app role must not delete connections")
        }
        // RLS without a tenant scope closes access before SQL: a bare runtime
        // query sees no rows at all.
        var seen int
        if err = runtimePool.QueryRow(ctx, "select count(*) from connector.connections").Scan(&seen); err != nil {
                t.Fatal(err)
        }
        if seen != 0 {
                t.Fatalf("RLS must close access without tenant scope, saw %d rows", seen)
        }
}

// TestConnectionStoreChecksAppRoleGrants verifies staffy_app cannot truncate
// or delete immutable history tables.
func TestConnectionStoreChecksAppRoleGrants(t *testing.T) {
        ctx := context.Background()
        if _, err := runtimePool.Exec(ctx, "truncate connector.secret_seals"); err == nil {
                t.Fatal("app role must not truncate secret seals")
        }
        if _, err := runtimePool.Exec(ctx, "delete from connector.connection_versions"); err == nil {
                t.Fatal("app role must not delete connection versions")
        }
        if _, err := runtimePool.Exec(ctx, "insert into connector.secret_seals(tenant_id,connection_id,secret_id,sealed,seal_key,created_at) values ('01994f64-6c00-7000-8000-00000000e102','01994f64-6c00-7000-8000-000000000001','01994f64-6c00-7000-8000-000000000002',digest('x','sha256'),digest('x','sha256'),now())"); err == nil {
                t.Fatal("RLS must reject inserting seals without tenant scope")
        }
        // environment CHECK enforces the production ban at storage level too.
        if _, dbErr := adminPool.Exec(ctx, "insert into connector.connections(tenant_id,connection_id,connector_id,connector_version,label,environment,status,authentication_state,transport_state,config,owner_id,version,created_at,updated_at) values ('01994f64-6c00-7000-8000-00000000e102','01994f64-6c00-7000-8000-000000000003','x','1','x','production','draft','pending','pending','{}'::jsonb,'01994f64-6c00-7000-8000-00000000e102',1,now(),now())"); dbErr == nil {
                t.Fatal("storage must reject production environment")
        }
}

func svc2For(identity *identityApp.Service, ids employeeIDs) *connectorApp.Service {
        return connectorApp.New(connectorStore.NewPostgres(runtimePool), identity, verifier.Synthetic{}, fixedClock{time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}, ids)
}
