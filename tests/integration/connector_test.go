//go:build integration

package integration_test

import (
        "context"
        "errors"
        "strings"
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
        _, err = svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "conn-key-prod", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.2.0", Label: "Prod", Environment: connectorDomain.EnvProduct, Config: map[string]string{"endpoint": "https://demo.sandbox.example"}})
        if !errors.Is(err, connectorDomain.ErrProductionBlocked) {
                t.Fatalf("production must be blocked: %v", err)
        }

        createCmd := connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "conn-key-create", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.2.0", Label: "Тестовая телефония", Environment: connectorDomain.EnvSandbox, Config: map[string]string{"endpoint": "https://demo.sandbox.example"}}
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
        second, err := svc2For(identity, employeeIDs{}).Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "conn-key-second", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.2.0", Label: "Второй аккаунт", Environment: connectorDomain.EnvTest, Config: map[string]string{"endpoint": "https://two.test.example"}})
        if err != nil || second.Connection.ConnectionID == connectionID {
                t.Fatalf("second account=%+v err=%v", second.Connection, err)
        }

        // A max-length revoke reason still fits the stored history limit
        // ("revoked: " + 290 = 299 characters, CHECK allows 300).
        longRevoked, err := svc.Revoke(ctx, connectorAPI.RevokeCommand{Actor: owner, TenantID: tenant, ConnectionID: second.Connection.ConnectionID, CorrelationID: corr, IdempotencyKey: "int-key-07", ExpectedVersion: second.Connection.Version, Reason: strings.Repeat("я", 290)})
        if err != nil || longRevoked.Connection.Status != connectorDomain.StatusRevoked {
                t.Fatalf("long reason revoke=%+v err=%v", longRevoked, err)
        }

        // Failed basic check is recorded honestly.
        failed, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, IdempotencyKey: "int-key-01", ExpectedVersion: created.Connection.Version, Secrets: map[string]string{"api_key": "real-secret-bad-format"}})
        if err != nil || failed.Connection.Status != connectorDomain.StatusFailed || failed.Connection.AuthState != connectorDomain.CheckFailed || failed.Connection.Transport != connectorDomain.CheckVerified {
                t.Fatalf("failed connect=%+v err=%v", failed, err)
        }

        // A failed check writes no orphan secret reference and keeps the
        // redacted explanation in the immutable history.
        var hasSecretKey bool
        if err = adminPool.QueryRow(ctx, "select payload ? 'secret_id' from connector.outbox_events where tenant_id=$1 and connection_id=$2 and aggregate_version=$3", tenant, connectionID, failed.Connection.Version).Scan(&hasSecretKey); err != nil {
                t.Fatal(err)
        }
        if hasSecretKey {
                t.Fatal("failed check must not advertise a secret it never sealed")
        }
        var summary string
        if err = adminPool.QueryRow(ctx, "select change_summary from connector.connection_versions where tenant_id=$1 and connection_id=$2 and version=$3", tenant, connectionID, failed.Connection.Version).Scan(&summary); err != nil {
                t.Fatal(err)
        }
        if !strings.Contains(summary, "Синтетическая проверка аутентификации не прошла") {
                t.Fatalf("history must keep the redacted explanation: %s", summary)
        }

        // Successful check stays connected/unverified — capabilities are not claimed.
        connected, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, IdempotencyKey: "int-key-02", ExpectedVersion: failed.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-integration-key"}})
        if err != nil || connected.Connection.Status != connectorDomain.StatusConnectedUnverified || connected.Connection.AuthState != connectorDomain.CheckVerified || connected.Connection.Transport != connectorDomain.CheckVerified {
                t.Fatalf("connect=%+v err=%v", connected, err)
        }
        if !connected.Connection.SecretActive || connected.Connection.SecretHint == "" || connected.Connection.SecretID == connectionID {
                t.Fatalf("secret bookkeeping=%+v", connected)
        }

        // Reads never expose the secret: config stays clean, hint is short.
        read, err := svc.Read(ctx, owner, tenant, connectionID)
        if err != nil || read.Config["api_key"] != "" || len(read.SecretHint) != 12 {
                t.Fatalf("read=%+v err=%v", read, err)
        }
        if _, err = svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, IdempotencyKey: "int-key-03", ExpectedVersion: connected.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-other-value"}}); err == nil {
                t.Fatal("double connect must be rejected; use rotation")
        }

        // CAS conflicts are recorded.
        if _, err = svc.Save(ctx, connectorAPI.SaveCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: 1, Config: map[string]string{"endpoint": "https://new.sandbox.example"}, Label: connected.Connection.Label, ChangeSummary: "Устаревшая версия"}); !errors.Is(err, connectorDomain.ErrConflict) {
                t.Fatalf("stale CAS=%v", err)
        }

        // Config change resets the honest states to draft.
        saved, err := svc.Save(ctx, connectorAPI.SaveCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: connected.Connection.Version, Config: map[string]string{"endpoint": "https://new.sandbox.example"}, Label: connected.Connection.Label, ChangeSummary: "Новый endpoint"})
        if err != nil || saved.Status != connectorDomain.StatusDraft || saved.AuthState != connectorDomain.CheckPending || saved.Transport != connectorDomain.CheckPending {
                t.Fatalf("save=%+v err=%v", saved, err)
        }

        // Failed rotation leaves no active secret over the revoked seal, and a
        // plain connect retry stays honest and allowed.
        failedRotate, err := svc.Rotate(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, IdempotencyKey: "int-key-04", ExpectedVersion: saved.Version, Secrets: map[string]string{"api_key": "not-sandbox-rotation"}})
        if err != nil || failedRotate.Connection.Status != connectorDomain.StatusFailed {
                t.Fatalf("failed rotate=%+v err=%v", failedRotate, err)
        }
        if failedRotate.Connection.SecretActive {
                t.Fatal("failed rotation must not keep a secret active over a revoked seal")
        }
        var storedActive bool
        if err = adminPool.QueryRow(ctx, "select secret_active from connector.connections where tenant_id=$1 and connection_id=$2", tenant, connectionID).Scan(&storedActive); err != nil {
                t.Fatal(err)
        }
        if storedActive {
                t.Fatal("stored secret_active must be false after failed rotation")
        }
        retried, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, IdempotencyKey: "int-key-05", ExpectedVersion: failedRotate.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-retry-key"}})
        if err != nil || retried.Connection.Status != connectorDomain.StatusConnectedUnverified || !retried.Connection.SecretActive {
                t.Fatalf("connect retry=%+v err=%v", retried, err)
        }

        // Rotation revokes the previous seal and re-checks honestly.
        rotated, err := svc.Rotate(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, IdempotencyKey: "int-key-06", ExpectedVersion: retried.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-rotated-key"}})
        if err != nil || rotated.Connection.Status != connectorDomain.StatusConnectedUnverified {
                t.Fatalf("rotate=%+v err=%v", rotated, err)
        }

        // Version history is immutable and ordered.
        versions, err := svc.ReadVersions(ctx, owner, tenant, connectionID)
        if err != nil || len(versions) < 5 {
                t.Fatalf("versions=%d err=%v", len(versions), err)
        }
        if versions[0].Version != rotated.Connection.Version {
                t.Fatalf("latest version=%d want %d", versions[0].Version, rotated.Connection.Version)
        }

        // Revocation is terminal and revokes the seal.
        revoked, err := svc.Revoke(ctx, connectorAPI.RevokeCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, IdempotencyKey: "int-key-08", ExpectedVersion: rotated.Connection.Version, Reason: "Тест завершён"})
        if err != nil || revoked.Connection.Status != connectorDomain.StatusRevoked || revoked.Connection.SecretActive {
                t.Fatalf("revoke=%+v err=%v", revoked, err)
        }
        if _, err = svc.Revoke(ctx, connectorAPI.RevokeCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, IdempotencyKey: "int-key-09", ExpectedVersion: revoked.Connection.Version, Reason: "Повтор"}); err == nil {
                t.Fatal("revoked connection must reject further revocations")
        }
        if _, err = svc.Save(ctx, connectorAPI.SaveCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: revoked.Connection.Version, Config: revoked.Connection.Config, Label: revoked.Connection.Label, ChangeSummary: "Попытка"}); err == nil {
                t.Fatal("revoked connection must reject config changes")
        }

        // Revocation revokes every remaining seal; nothing stays active.
        var active, revokedSeals int
        if err = adminPool.QueryRow(ctx, "select count(*) filter (where revoked_at is null), count(*) filter (where revoked_at is not null) from connector.secret_seals where tenant_id=$1 and connection_id=$2", tenant, connectionID).Scan(&active, &revokedSeals); err != nil {
                t.Fatal(err)
        }
        if active != 0 || revokedSeals != 3 {
                t.Fatalf("active=%d revoked=%d (want 0/3: only verified secrets are sealed)", active, revokedSeals)
        }

        // Audit and outbox never contain secret material: audit has no payload
        // column at all, and outbox payloads only carry opaque references.
        var leak int
        if err = adminPool.QueryRow(ctx, `select count(*) from connector.outbox_events where tenant_id=$1 and (payload::text like '%sandbox-integration-key%' or payload::text like '%sandbox-rotated-key%' or payload::text like '%sandbox-retry-key%' or payload::text like '%not-sandbox-rotation%' or payload::text like '%real-secret-bad%')`, tenant).Scan(&leak); err != nil {
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

// TestCapabilityEvidenceLifecycle walks the full Story 1-9 matrix on the real
// PostgreSQL store: declared vs proven capabilities, read-only probe evidence,
// idempotent replay/conflict, timeout→unknown, stale on rotation/config-change/
// revoke/expiry, isolation of foreign evidence and honest blocked capabilities.
func TestCapabilityEvidenceLifecycle(t *testing.T) {
        ctx := context.Background()
        identity, owner := newService("00000000e201", "00000000e202", "00000000e203")
        tenant := createTenant(t, identity, owner, "Capability test tenant", "capability-tenant-key", "00000000e204").Tenant.ID
        ids := employeeIDs{}
        corr := id.MustParse("01994f64-6c00-7000-8000-00000000e401")
        svc := connectorService(identity, ids)

        created, err := svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "cap-key-create", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.2.0", Label: "Возможности", Environment: connectorDomain.EnvSandbox, Config: map[string]string{"endpoint": "https://demo.sandbox.example"}})
        if err != nil {
                t.Fatalf("create: %v", err)
        }
        connectionID := created.Connection.ConnectionID

        // Draft: capabilities are guarded, verification refused.
        if _, err = svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.trunk-status", IdempotencyKey: "cap-verify-draft"}); err == nil {
                t.Fatal("draft connection must refuse verification")
        }
        connected, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: created.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-capability-key"}, IdempotencyKey: "cap-key-connect"})
        if err != nil || connected.Connection.Status != connectorDomain.StatusConnectedUnverified {
                t.Fatalf("connect=%+v err=%v", connected.Connection, err)
        }

        // CapabilityState: declared vs proven with honest states and the block.
        state, err := svc.CapabilityState(ctx, owner, tenant, connectionID)
        if err != nil {
                t.Fatalf("state: %v", err)
        }
        if len(state.Capabilities) != 8 {
                t.Fatalf("contract 1.2.0 declares 8 capabilities (3 prior + 5 SIP queries of spec 4-1), got %d", len(state.Capabilities))
        }
        var trunk, sms, call *connectorDomain.CapabilityReport
        for i, report := range state.Capabilities {
                switch report.ID {
                case "query.trunk-status":
                        trunk = &state.Capabilities[i]
                case "command.send-sms":
                        sms = &state.Capabilities[i]
                case "event.inbound-call":
                        call = &state.Capabilities[i]
                }
        }
        if trunk == nil || sms == nil || call == nil {
                t.Fatalf("missing capabilities: %+v", state.Capabilities)
        }
        if trunk.State != connectorDomain.CapStateUnverified || !trunk.CanVerify {
                t.Fatalf("trunk must be unverified and verifiable: %+v", trunk)
        }
        if sms.State != connectorDomain.CapStateUnverified || sms.Block == nil || sms.Block.Gate != "AG-4" {
                t.Fatalf("sms must carry the managed Action Gateway block: %+v", sms)
        }
        if call.State != connectorDomain.CapStateUnverified {
                t.Fatalf("inbound call must stay unverified: %+v", call)
        }

        // Mutating capability: fail-closed refusal naming the gate and owner.
        if _, err = svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "command.send-sms", IdempotencyKey: "cap-verify-sms"}); err == nil {
                t.Fatal("mutating verification must be refused")
        } else {
                var rule connectorDomain.RuleError
                if !errors.As(err, &rule) || !strings.Contains(rule.Reason, "Action Gateway") || !strings.Contains(rule.Reason, "AG-4") {
                        t.Fatalf("block reason must name the gate: %v", err)
                }
        }

        // Happy path: read-only probe stores immutable evidence.
        verified, err := svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.trunk-status", IdempotencyKey: "cap-verify-trunk"})
        if err != nil {
                t.Fatalf("verify: %v", err)
        }
        if verified.Report.State != connectorDomain.CapStateVerified || verified.Report.Evidence == nil || verified.Replayed {
                t.Fatalf("verified report=%+v", verified.Report)
        }
        if verified.Report.Evidence.DeliveryLevel != string(connectorDomain.DeliveryDelivered) {
                t.Fatalf("delivery level=%s", verified.Report.Evidence.DeliveryLevel)
        }

        // Idempotent replay: same key, same data, no contradicting rows.
        replay, err := svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "query.trunk-status", IdempotencyKey: "cap-verify-trunk"})
        if err != nil || !replay.Replayed || replay.Report.State != connectorDomain.CapStateVerified {
                t.Fatalf("replay=%+v err=%v", replay, err)
        }
        var evidenceRows int
        if err = adminPool.QueryRow(ctx, "select count(*) from connector.capability_evidence where tenant_id=$1 and connection_id=$2", tenant, connectionID).Scan(&evidenceRows); err != nil {
                t.Fatal(err)
        }
        if evidenceRows != 1 {
                t.Fatalf("replay must not add evidence rows: %d", evidenceRows)
        }

        // Key conflict: same key, different capability → honest 409 semantics.
        if _, err = svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, Capability: "event.inbound-call", IdempotencyKey: "cap-verify-trunk"}); !errors.Is(err, connectorDomain.ErrIdempotencyConflict) {
                t.Fatalf("key conflict=%v", err)
        }

        // Redaction: no secret material and no raw payloads in evidence.
        var leak int
        if err = adminPool.QueryRow(ctx, `select count(*) from connector.capability_evidence where tenant_id=$1 and (redacted_request like '%sandbox-%' or redacted_result like '%sandbox-%' or redacted_result like '%api_key%')`, tenant).Scan(&leak); err != nil {
                t.Fatal(err)
        }
        if leak != 0 {
                t.Fatalf("evidence leaked secret material: %d rows", leak)
        }

        // Timeout scenario: the timeout. host makes the probe exceed its deadline
        // → honest unknown, no blind retry, reconciliation owner in the reason.
        timeoutConn, err := svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "cap-key-timeout", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.2.0", Label: "Таймаут", Environment: connectorDomain.EnvSandbox, Config: map[string]string{"endpoint": "https://timeout.demo.sandbox.example"}})
        if err != nil {
                t.Fatalf("create timeout conn: %v", err)
        }
        timeoutConnected, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: timeoutConn.Connection.ConnectionID, CorrelationID: corr, ExpectedVersion: timeoutConn.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-timeout-key"}, IdempotencyKey: "cap-key-conn-to"})
        if err != nil {
                t.Fatalf("connect timeout conn: %v", err)
        }
        unknown, err := svc.VerifyCapability(ctx, connectorAPI.VerifyCommand{Actor: owner, TenantID: tenant, ConnectionID: timeoutConn.Connection.ConnectionID, CorrelationID: corr, Capability: "query.trunk-status", IdempotencyKey: "cap-verify-unknown"})
        if err != nil {
                t.Fatalf("verify timeout: %v", err)
        }
        if unknown.Report.State != connectorDomain.CapStateUnknown || unknown.Report.Evidence == nil || unknown.Report.Evidence.DeliveryLevel != string(connectorDomain.DeliveryUnknown) {
                t.Fatalf("timeout must yield honest unknown: %+v", unknown.Report)
        }
        if !strings.Contains(unknown.Report.StateReason, "Technical Lead") {
                t.Fatalf("unknown reason must name the reconciliation owner: %s", unknown.Report.StateReason)
        }
        _ = timeoutConnected

        // Rotation invalidates the evidence as stale with the concrete reason.
        rotated, err := svc.Rotate(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: connected.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-rotated-cap"}, IdempotencyKey: "cap-key-rotate"})
        if err != nil {
                t.Fatalf("rotate: %v", err)
        }
        state, err = svc.CapabilityState(ctx, owner, tenant, connectionID)
        if err != nil {
                t.Fatalf("state after rotate: %v", err)
        }
        for _, report := range state.Capabilities {
                if report.ID == "query.trunk-status" {
                        if report.State != connectorDomain.CapStateStale || !strings.Contains(report.StateReason, "ротирован") {
                                t.Fatalf("rotation must mark evidence stale: %s / %s", report.State, report.StateReason)
                        }
                }
        }

        // Config change returns the connection to draft: capabilities not claimed.
        saved, err := svc.Save(ctx, connectorAPI.SaveCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: rotated.Connection.Version, Config: map[string]string{"endpoint": "https://new.sandbox.example"}, Label: rotated.Connection.Label, ChangeSummary: "Новый endpoint"})
        if err != nil || saved.Status != connectorDomain.StatusDraft {
                t.Fatalf("save=%+v err=%v", saved, err)
        }
        state, err = svc.CapabilityState(ctx, owner, tenant, connectionID)
        if err != nil {
                t.Fatalf("state after save: %v", err)
        }
        for _, report := range state.Capabilities {
                if report.State != connectorDomain.CapStateUnverified {
                        t.Fatalf("draft connection must not claim capabilities: %+v", report)
                }
        }

        // The config save keeps the accepted seal: rotation replaces it honestly
        // (the plain connect over an active secret is a rule rejection).
        reconnected, err := svc.Rotate(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: saved.Version, Secrets: map[string]string{"api_key": "sandbox-final-cap"}, IdempotencyKey: "cap-key-reconnect"})
        if err != nil || reconnected.Connection.Status != connectorDomain.StatusConnectedUnverified {
                t.Fatalf("rotate after save=%+v err=%v", reconnected.Connection, err)
        }

        // Expiry: admin-inserted evidence with a past TTL reads as stale (lazy
        // derivation at read time; the fixed test clock sits at 2026-10-05 12:00).
        var expiredEvidence int
        if err = adminPool.QueryRow(ctx, `insert into connector.capability_evidence(tenant_id,evidence_id,connection_id,secret_id,environment,contract_version,adapter_build,capability,probe_kind,actor_id,requested_at,deadline_at,completed_at,outcome,delivery_level,redacted_request,redacted_result,expires_at,idempotency_key,request_hash) select tenant_id,'fffffffe-6c00-7000-8000-00000000e402',connection_id,secret_id,environment,connector_version,'synthetic/1.1.0','query.trunk-status','read_only_probe',owner_id,'2026-10-05 12:00:00+00','2026-10-05 12:00:00+00','2026-10-05 12:00:00+00','verified','delivered','probe:read-only redacted','expired by test','2026-10-05 11:30:00+00','expired-key-42','expired-hash' from connector.connections where tenant_id=$1 and connection_id=$2 returning 1`, tenant, connectionID).Scan(&expiredEvidence); err != nil {
                t.Fatal(err)
        }
        if expiredEvidence != 1 {
                t.Fatalf("expired evidence insert failed: %d", expiredEvidence)
        }
        state, err = svc.CapabilityState(ctx, owner, tenant, connectionID)
        if err != nil {
                t.Fatalf("state after expiry insert: %v", err)
        }
        for _, report := range state.Capabilities {
                if report.ID == "query.trunk-status" {
                        if report.State != connectorDomain.CapStateStale || !strings.Contains(report.StateReason, "истёк") {
                                t.Fatalf("expired evidence must read stale: %s / %s", report.State, report.StateReason)
                        }
                }
        }

        // Foreign evidence never applies to another connection (isolation).
        foreign, err := svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "cap-key-foreign", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.2.0", Label: "Чужое", Environment: connectorDomain.EnvTest, Config: map[string]string{"endpoint": "https://other.test.example"}})
        if err != nil {
                t.Fatalf("create foreign: %v", err)
        }
        foreignConnected, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: foreign.Connection.ConnectionID, CorrelationID: corr, ExpectedVersion: foreign.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-foreign-key"}, IdempotencyKey: "cap-key-foreign-conn"})
        if err != nil {
                t.Fatalf("connect foreign: %v", err)
        }
        foreignState, err := svc.CapabilityState(ctx, owner, tenant, foreign.Connection.ConnectionID)
        if err != nil {
                t.Fatalf("foreign state: %v", err)
        }
        for _, report := range foreignState.Capabilities {
                if report.ID == "query.trunk-status" && report.State != connectorDomain.CapStateUnverified {
                        t.Fatalf("foreign evidence must not apply: %+v", report)
                }
        }
        _ = foreignConnected

        // Revoke: capabilities are not applicable, history preserved.
        revoked, err := svc.Revoke(ctx, connectorAPI.RevokeCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: reconnected.Connection.Version, Reason: "Тест завершён", IdempotencyKey: "cap-key-revoke"})
        if err != nil {
                t.Fatalf("revoke: %v", err)
        }
        if revoked.Connection.Status != connectorDomain.StatusRevoked {
                t.Fatalf("revoked=%+v", revoked.Connection)
        }
        state, err = svc.CapabilityState(ctx, owner, tenant, connectionID)
        if err != nil {
                t.Fatalf("state after revoke: %v", err)
        }
        for _, report := range state.Capabilities {
                if report.State != connectorDomain.CapStateUnverified || !strings.Contains(report.StateReason, "отозвано") && !strings.Contains(report.StateReason, "отзыв") && report.ID != "command.send-sms" && report.ID != "event.inbound-call" {
                        t.Fatalf("revoked connection capabilities must be guarded: %+v", report)
                }
        }

        // Evidence history is preserved after revocation.
        if err = adminPool.QueryRow(ctx, "select count(*) from connector.capability_evidence where tenant_id=$1 and connection_id=$2", tenant, connectionID).Scan(&evidenceRows); err != nil {
                t.Fatal(err)
        }
        if evidenceRows < 2 {
                t.Fatalf("evidence history must survive revocation: %d", evidenceRows)
        }
}

// TestConcurrentConnectRotationsProduceExactlyOneSeal closes R2 Gap 4: two
// parallel commands against the same expected version produce exactly one
// sealed secret and exactly one honest version conflict.
func TestConcurrentConnectRotationsProduceExactlyOneSeal(t *testing.T) {
        ctx := context.Background()
        identity, owner := newService("00000000e301", "00000000e302", "00000000e303")
        tenant := createTenant(t, identity, owner, "CAS test tenant", "cas-tenant-key", "00000000e304").Tenant.ID
        ids := employeeIDs{}
        corr := id.MustParse("01994f64-6c00-7000-8000-00000000e501")
        svc := connectorService(identity, ids)

        created, err := svc.Create(ctx, connectorAPI.CreateCommand{Actor: owner, TenantID: tenant, IdempotencyKey: "cas-key-create", CorrelationID: corr, ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.2.0", Label: "CAS", Environment: connectorDomain.EnvSandbox, Config: map[string]string{"endpoint": "https://cas.sandbox.example"}})
        if err != nil {
                t.Fatalf("create: %v", err)
        }
        connectionID := created.Connection.ConnectionID

        const workers = 2
        results := make(chan error, workers)
        for i := 0; i < workers; i++ {
                go func(i int) {
                        _, err := svc.Connect(ctx, connectorAPI.ConnectCommand{Actor: owner, TenantID: tenant, ConnectionID: connectionID, CorrelationID: corr, ExpectedVersion: created.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-cas-key-" + string(rune('a'+i))}, IdempotencyKey: "cas-key-worker-" + string(rune('0'+i))})
                        results <- err
                }(i)
        }
        successes, conflicts := 0, 0
        for i := 0; i < workers; i++ {
                if err := <-results; err == nil {
                        successes++
                } else if errors.Is(err, connectorDomain.ErrConflict) {
                        conflicts++
                } else {
                        t.Fatalf("unexpected concurrent error: %v", err)
                }
        }
        if successes != 1 || conflicts != 1 {
                t.Fatalf("concurrent connect must yield exactly one success and one conflict: %d/%d", successes, conflicts)
        }
        var seals int
        if err = adminPool.QueryRow(ctx, "select count(*) from connector.secret_seals where tenant_id=$1 and connection_id=$2", tenant, connectionID).Scan(&seals); err != nil {
                t.Fatal(err)
        }
        if seals != 1 {
                t.Fatalf("exactly one seal expected, got %d", seals)
        }
}

// TestCapabilityEvidenceGrantsKeepRowsImmutable verifies staffy_app cannot
// update or delete capability evidence (append-only at the DB level) and RLS
// rejects inserts without a tenant scope.
func TestCapabilityEvidenceGrantsKeepRowsImmutable(t *testing.T) {
        ctx := context.Background()
        if _, err := runtimePool.Exec(ctx, "update connector.capability_evidence set outcome='verified'"); err == nil {
                t.Fatal("app role must not update capability evidence")
        }
        if _, err := runtimePool.Exec(ctx, "delete from connector.capability_evidence"); err == nil {
                t.Fatal("app role must not delete capability evidence")
        }
        if _, err := runtimePool.Exec(ctx, "truncate connector.capability_evidence"); err == nil {
                t.Fatal("app role must not truncate capability evidence")
        }
        if _, err := runtimePool.Exec(ctx, `insert into connector.capability_evidence(tenant_id,evidence_id,connection_id,secret_id,environment,contract_version,adapter_build,capability,probe_kind,actor_id,requested_at,deadline_at,completed_at,outcome,delivery_level,redacted_request,redacted_result,expires_at,idempotency_key,request_hash) values ('01994f64-6c00-7000-8000-00000000e302','01994f64-6c00-7000-8000-00000000e303','01994f64-6c00-7000-8000-00000000e304','01994f64-6c00-7000-8000-00000000e305','sandbox','1.1.0','synthetic/1.1.0','query.trunk-status','read_only_probe','01994f64-6c00-7000-8000-00000000e306',now(),now(),now(),'verified','delivered','r','r',now(),'rls-key-123','h')`); err == nil {
                t.Fatal("RLS must reject inserting evidence without tenant scope")
        }
}
