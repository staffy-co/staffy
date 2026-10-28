package app

import (
        "context"
        "errors"
        "fmt"
        "strings"
        "testing"
        "time"

        "github.com/artemiosu/staffy/internal/connector/api"
        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        "github.com/artemiosu/staffy/internal/connector/port"
        identityAPI "github.com/artemiosu/staffy/internal/identity/api"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

var (
        connTenant       = id.MustParse("01994f64-6c00-7000-8000-000000000021")
        connOtherTenant  = id.MustParse("01994f64-6c00-7000-8000-000000000022")
        connActor        = id.MustParse("01994f64-6c00-7000-8000-000000000023")
        connCorrelation  = id.MustParse("01994f64-6c00-7000-8000-000000000024")
        connConnection   = id.MustParse("01994f64-6c00-7000-8000-000000000025")
        connSecretID     = id.MustParse("01994f64-6c00-7000-8000-000000000026")
        connEventID      = id.MustParse("01994f64-6c00-7000-8000-000000000027")
        connAuditID      = id.MustParse("01994f64-6c00-7000-8000-000000000028")
)

type connClock struct{}

func (connClock) Now() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC) }

type connIDs struct{ seq int }

func (g *connIDs) New(time.Time) (id.UUID, error) {
        g.seq++
        return id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", g.seq)), nil
}

type connIdentity struct{ allowed bool }

func (i connIdentity) ListMembers(context.Context, identityDomain.Principal, id.UUID) ([]identityDomain.Membership, error) {
        return nil, nil
}
func (i connIdentity) InviteMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, string, error) {
        return identityDomain.Membership{}, "", nil
}
func (i connIdentity) AcceptInvitation(context.Context, identityDomain.Principal, string, id.UUID) (identityDomain.Membership, error) {
        return identityDomain.Membership{}, nil
}
func (i connIdentity) ChangeMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, error) {
        return identityDomain.Membership{}, nil
}
func (i connIdentity) RevokeMember(context.Context, identityAPI.MembershipCommand) error { return nil }
func (i connIdentity) Authorize(_ context.Context, _ identityDomain.Principal, tenant id.UUID, _ identityDomain.Scope) error {
        if !i.allowed || tenant != connTenant {
                return identityDomain.ErrForbidden
        }
        return nil
}

// commandRecord is one idempotency record of connect/rotate/revoke.
type commandRecord struct {
        hash     string
        snapshot connectorDomain.Connection
}

// connRepo is a fake repository implementing the full port.Repository with
// in-memory idempotency for commands and capability evidence.
type connRepo struct {
        connections map[id.UUID]connectorDomain.Connection
        seals       map[id.UUID]*connectorDomain.Seal
        idempotency map[string]port.CreateParams
        records     map[string]commandRecord // key: operation + ":" + idempotency key
        evidence    map[string]connectorDomain.CapabilityEvidence
        lastEvent   map[string]any
}

func newConnRepo() *connRepo {
        return &connRepo{connections: map[id.UUID]connectorDomain.Connection{}, seals: map[id.UUID]*connectorDomain.Seal{}, idempotency: map[string]port.CreateParams{}, records: map[string]commandRecord{}, evidence: map[string]connectorDomain.CapabilityEvidence{}, lastEvent: map[string]any{}}
}

func (r *connRepo) recordKey(operation, key string) string { return operation + ":" + key }

func (r *connRepo) LookupCommand(_ context.Context, _ tenancy.Context, operation, key, hash string) (connectorDomain.Connection, bool, error) {
        record, ok := r.records[r.recordKey(operation, key)]
        if !ok {
                return connectorDomain.Connection{}, false, nil
        }
        if record.hash != hash {
                return connectorDomain.Connection{}, false, connectorDomain.ErrIdempotencyConflict
        }
        return record.snapshot, true, nil
}

func (r *connRepo) Create(_ context.Context, p port.CreateParams) (connectorDomain.Connection, bool, error) {
        if old, ok := r.idempotency[p.Key]; ok {
                if old.Hash != p.Hash {
                        return connectorDomain.Connection{}, false, connectorDomain.ErrIdempotencyConflict
                }
                return r.connections[old.ConnectionID], true, nil
        }
        c := connectorDomain.Connection{
                TenantID: p.Scope.TenantID(), ConnectionID: p.ConnectionID, ConnectorID: p.ConnectorID, ConnectorVer: p.ConnectorVer,
                Label: p.Label, Environment: p.Environment, Status: connectorDomain.StatusDraft,
                AuthState: connectorDomain.CheckPending, Transport: connectorDomain.CheckPending,
                Config: p.Config, OwnerID: p.Scope.PrincipalID(), Version: 1, CreatedAt: p.Now, UpdatedAt: p.Now,
        }
        r.connections[c.ConnectionID] = c
        r.idempotency[p.Key] = p
        return c, false, nil
}

func (r *connRepo) List(_ context.Context, scope tenancy.Context) ([]connectorDomain.Connection, error) {
        out := []connectorDomain.Connection{}
        for _, c := range r.connections {
                if c.TenantID == scope.TenantID() {
                        out = append(out, c)
                }
        }
        return out, nil
}

func (r *connRepo) Read(_ context.Context, scope tenancy.Context, connectionID id.UUID) (connectorDomain.Connection, error) {
        c, ok := r.connections[connectionID]
        if !ok || c.TenantID != scope.TenantID() {
                return connectorDomain.Connection{}, connectorDomain.ErrNotFound
        }
        return c, nil
}

func (r *connRepo) ReadVersions(_ context.Context, scope tenancy.Context, connectionID id.UUID) ([]connectorDomain.ConnectionVersion, error) {
        c, err := r.Read(context.Background(), scope, connectionID)
        if err != nil {
                return nil, err
        }
        return []connectorDomain.ConnectionVersion{{Connection: c, ActorID: c.OwnerID, ChangeSummary: "created"}}, nil
}

func (r *connRepo) Save(_ context.Context, p port.SaveParams) (connectorDomain.Connection, error) {
        c, ok := r.connections[p.ConnectionID]
        if !ok {
                return connectorDomain.Connection{}, connectorDomain.ErrNotFound
        }
        if c.Version != p.ExpectedVersion {
                return connectorDomain.Connection{}, connectorDomain.ErrConflict
        }
        c.Version++
        c.Label = p.Label
        c.Config = p.Config
        c.Status = connectorDomain.StatusDraft
        c.AuthState = connectorDomain.CheckPending
        c.Transport = connectorDomain.CheckPending
        c.UpdatedAt = p.Now
        r.connections[p.ConnectionID] = c
        return c, nil
}

func (r *connRepo) Connect(_ context.Context, p port.ConnectParams) (connectorDomain.Connection, bool, error) {
        if record, ok := r.records[r.recordKey(p.Operation, p.IdempotencyKey)]; ok {
                if record.hash != p.RequestHash {
                        return connectorDomain.Connection{}, false, connectorDomain.ErrIdempotencyConflict
                }
                return record.snapshot, true, nil
        }
        c, ok := r.connections[p.ConnectionID]
        if !ok {
                return connectorDomain.Connection{}, false, connectorDomain.ErrNotFound
        }
        if c.Version != p.ExpectedVersion {
                return connectorDomain.Connection{}, false, connectorDomain.ErrConflict
        }
        // Mirrors the SQL store: the single-secret rule lives under the row lock,
        // so the concurrent race resolves as a CAS conflict and the sequential
        // double connect as an honest rule rejection.
        if !p.Rotate && c.SecretActive {
                return connectorDomain.Connection{}, false, connectorDomain.RuleError{Rule: "secret", Reason: "Секрет уже принят. Для замены используйте ротацию."}
        }
        if p.Rotate && c.SecretActive {
                if seal, ok := r.seals[c.SecretID]; ok && seal.RevokedAt == nil {
                        now := p.Now
                        seal.RevokedAt = &now
                }
        }
        c.Version++
        if p.Seal {
                c.SecretID = p.SecretID
                c.SecretHint = p.Hint
                c.SecretActive = true
                r.seals[p.SecretID] = &connectorDomain.Seal{TenantID: c.TenantID, ConnectionID: c.ConnectionID, SecretID: p.SecretID, Sealed: p.Sealed, Key: p.Key, CreatedAt: p.Now}
                r.lastEvent["connect"] = map[string]any{"secret_id": p.SecretID, "hint": p.Hint, "sealed_len": len(p.Sealed), "key_len": len(p.Key)}
        } else if p.Rotate {
                // Mirrors the SQL store: the previous seal was revoked and no new
                // secret was accepted, so no secret stays active.
                c.SecretActive = false
        }
        c.AuthState = p.Check.Authentication
        c.Transport = p.Check.Transport
        if p.Check.Authentication == connectorDomain.CheckVerified && p.Check.Transport == connectorDomain.CheckVerified {
                c.Status = connectorDomain.StatusConnectedUnverified
        } else {
                c.Status = connectorDomain.StatusFailed
        }
        c.UpdatedAt = p.Now
        r.connections[p.ConnectionID] = c
        if p.IdempotencyKey != "" {
                r.records[r.recordKey(p.Operation, p.IdempotencyKey)] = commandRecord{hash: p.RequestHash, snapshot: c}
        }
        return c, false, nil
}

func (r *connRepo) Revoke(_ context.Context, p port.RevokeParams) (connectorDomain.Connection, bool, error) {
        if record, ok := r.records[r.recordKey(p.Operation, p.IdempotencyKey)]; ok {
                if record.hash != p.RequestHash {
                        return connectorDomain.Connection{}, false, connectorDomain.ErrIdempotencyConflict
                }
                return record.snapshot, true, nil
        }
        c, ok := r.connections[p.ConnectionID]
        if !ok {
                return connectorDomain.Connection{}, false, connectorDomain.ErrNotFound
        }
        if c.Version != p.ExpectedVersion {
                return connectorDomain.Connection{}, false, connectorDomain.ErrConflict
        }
        c.Version++
        c.Status = connectorDomain.StatusRevoked
        c.SecretActive = false
        c.UpdatedAt = p.Now
        r.connections[p.ConnectionID] = c
        if seal, ok := r.seals[c.SecretID]; ok && seal.RevokedAt == nil {
                now := p.Now
                seal.RevokedAt = &now
        }
        if p.IdempotencyKey != "" {
                r.records[r.recordKey(p.Operation, p.IdempotencyKey)] = commandRecord{hash: p.RequestHash, snapshot: c}
        }
        return c, false, nil
}

func (r *connRepo) LookupEvidence(_ context.Context, _ tenancy.Context, connectionID id.UUID, key, hash string) (connectorDomain.CapabilityEvidence, bool, error) {
        existing, ok := r.evidence[connectionID.String()+":"+key]
        if !ok {
                return connectorDomain.CapabilityEvidence{}, false, nil
        }
        if existing.RequestHash != hash {
                return connectorDomain.CapabilityEvidence{}, false, connectorDomain.ErrIdempotencyConflict
        }
        return existing, true, nil
}

func (r *connRepo) SaveEvidence(_ context.Context, p port.SaveEvidenceParams) (connectorDomain.CapabilityEvidence, bool, error) {
        slot := p.ConnectionID.String() + ":" + p.IdempotencyKey
        if existing, ok := r.evidence[slot]; ok {
                if existing.RequestHash != p.RequestHash {
                        return connectorDomain.CapabilityEvidence{}, false, connectorDomain.ErrIdempotencyConflict
                }
                return existing, true, nil
        }
        row := connectorDomain.CapabilityEvidence{
                TenantID: p.Scope.TenantID(), EvidenceID: p.EvidenceID, ConnectionID: p.ConnectionID, SecretID: p.SecretID,
                Environment: p.Environment, ContractVersion: p.ContractVersion, AdapterBuild: p.AdapterBuild,
                Capability: p.Capability, ProbeKind: p.ProbeKind, ActorID: p.Scope.PrincipalID(),
                RequestedAt: p.RequestedAt, DeadlineAt: p.DeadlineAt, CompletedAt: p.CompletedAt,
                Outcome: p.Outcome, DeliveryLevel: p.DeliveryLevel,
                RedactedRequest: p.RedactedRequest, RedactedResult: p.RedactedResult,
                ExpiresAt: p.ExpiresAt, IdempotencyKey: p.IdempotencyKey, RequestHash: p.RequestHash,
        }
        r.evidence[slot] = row
        return row, false, nil
}

func (r *connRepo) ListEvidence(_ context.Context, _ tenancy.Context, connectionID id.UUID) ([]connectorDomain.CapabilityEvidence, error) {
        out := []connectorDomain.CapabilityEvidence{}
        for _, row := range r.evidence {
                if row.ConnectionID == connectionID {
                        out = append(out, row)
                }
        }
        return out, nil
}

// ReadSeal serves the MAC matrix of the external class probes: the active
// seal of the connection secret (revoked seals never match).
func (r *connRepo) ReadSeal(_ context.Context, _ tenancy.Context, connectionID, secretID id.UUID) (connectorDomain.Seal, bool, error) {
        seal, ok := r.seals[secretID]
        if !ok || seal.ConnectionID != connectionID || seal.RevokedAt != nil {
                return connectorDomain.Seal{}, false, nil
        }
        return *seal, true, nil
}

func serviceFixture() (*Service, *connRepo) {
        repo := newConnRepo()
        s := New(repo, connIdentity{allowed: true}, stubVerifier{}, connClock{}, &connIDs{})
        return s, repo
}

type stubVerifier struct{}

func (stubVerifier) BasicCheck(_ context.Context, def connectorDomain.ConnectorDefinition, env connectorDomain.Environment, config map[string]string, secrets map[string]string) connectorDomain.CheckResult {
        result := connectorDomain.CheckResult{Authentication: connectorDomain.CheckFailed, Transport: connectorDomain.CheckFailed}
        if v, ok := secrets["api_key"]; ok && strings.HasPrefix(v, "sandbox-") {
                result.Authentication = connectorDomain.CheckVerified
        }
        if v, ok := config["endpoint"]; ok && strings.Contains(v, ".sandbox.example") {
                result.Transport = connectorDomain.CheckVerified
        }
        return result
}

func (stubVerifier) Probe(_ context.Context, _ connectorDomain.ConnectorDefinition, dec connectorDomain.CapabilityDeclaration, _ connectorDomain.Environment, _ map[string]string, _ map[string]string) api.ProbeResult {
        return api.ProbeResult{
                Outcome:         connectorDomain.EvidenceVerified,
                DeliveryLevel:   string(connectorDomain.DeliveryDelivered),
                RedactedRequest: "probe:read-only " + dec.ID + " → demo.sandbox.example (sandbox), адаптер stub/1.0",
                RedactedResult:  "Синтетический read-only ответ получен: транк активен; уровень доставки delivered.",
        }
}

func (stubVerifier) Build() string { return "stub/1.0" }

func createCommand() api.CreateCommand {
        return api.CreateCommand{
                Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, IdempotencyKey: "create-key-123", CorrelationID: connCorrelation,
                // Spec 4-1 raised the synthetic telephony contract to 1.2.0
                // (the SIP probe set of AC-6) — new connections pin the latest.
                ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.2.0", Label: "Тестовая телефония", Environment: connectorDomain.EnvSandbox,
                Config: map[string]string{"endpoint": "https://demo.sandbox.example"},
        }
}

func connectCommand(connectionID id.UUID, version int64, key, secret string) api.ConnectCommand {
        return api.ConnectCommand{
                Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation,
                ExpectedVersion: version, Secrets: map[string]string{"api_key": secret}, IdempotencyKey: key,
        }
}

func TestCreateDraftConnection(t *testing.T) {
        s, repo := serviceFixture()
        result, err := s.Create(context.Background(), createCommand())
        if err != nil {
                t.Fatalf("create: %v", err)
        }
        if result.Connection.Status != connectorDomain.StatusDraft || result.Connection.Version != 1 || result.Connection.SecretActive {
                t.Fatalf("draft = %+v", result.Connection)
        }
        if result.Replayed {
                t.Fatal("first create must not be replayed")
        }
        replayed, err := s.Create(context.Background(), createCommand())
        if err != nil || !replayed.Replayed {
                t.Fatalf("replay: %v %+v", err, replayed)
        }
        if len(repo.connections) != 1 {
                t.Fatalf("idempotent replay created duplicate: %d", len(repo.connections))
        }
        conflict := createCommand()
        conflict.Label = "Другое назначение"
        if _, err = s.Create(context.Background(), conflict); !errors.Is(err, connectorDomain.ErrIdempotencyConflict) {
                t.Fatalf("conflict: %v", err)
        }
}

func TestCreateRejectsStaleContractVersion(t *testing.T) {
        s, _ := serviceFixture()
        cmd := createCommand()
        cmd.ConnectorVer = "1.0.0"
        if _, err := s.Create(context.Background(), cmd); err == nil {
                t.Fatal("an outdated contract pin must be refused: new connections pin the latest version")
        }
}

func TestCreateRejectsProductionFailClosed(t *testing.T) {
        s, _ := serviceFixture()
        cmd := createCommand()
        cmd.Environment = connectorDomain.EnvProduct
        _, err := s.Create(context.Background(), cmd)
        if !errors.Is(err, connectorDomain.ErrProductionBlocked) {
                t.Fatalf("production must be blocked fail-closed, got %v", err)
        }
        var blocked connectorDomain.ProductionBlockedError
        if !errors.As(err, &blocked) || !strings.Contains(blocked.Reason, "AG-4") {
                t.Fatalf("gate evidence missing: %v", err)
        }
}

func TestCreateRejectsUnimplementedConnector(t *testing.T) {
        s, _ := serviceFixture()
        cmd := createCommand()
        var err error
        cmd.ConnectorID = "novofon-sip"
        if _, err = s.Create(context.Background(), cmd); err == nil {
                t.Fatal("unimplemented connector must be rejected")
        }
        cmd.ConnectorID = "exolve-sip"
        if _, err = s.Create(context.Background(), cmd); err == nil {
                t.Fatal("unavailable connector must be rejected")
        }
        cmd.ConnectorID = "unknown-connector"
        if _, err = s.Create(context.Background(), cmd); err == nil {
                t.Fatal("unknown connector must be rejected")
        }
}

func TestConnectSealsSecretAndKeepsCapabilityHonest(t *testing.T) {
        s, repo := serviceFixture()
        ctx := context.Background()
        created, err := s.Create(ctx, createCommand())
        if err != nil {
                t.Fatalf("create: %v", err)
        }
        c, err := s.Connect(ctx, connectCommand(created.Connection.ConnectionID, created.Connection.Version, "connect-key-1", "sandbox-demo-123"))
        if err != nil {
                t.Fatalf("connect: %v", err)
        }
        if c.Connection.Status != connectorDomain.StatusConnectedUnverified {
                t.Fatalf("status = %s, want connected_unverified", c.Connection.Status)
        }
        if c.Connection.AuthState != connectorDomain.CheckVerified || c.Connection.Transport != connectorDomain.CheckVerified {
                t.Fatalf("states = %s/%s", c.Connection.AuthState, c.Connection.Transport)
        }
        if c.Connection.SecretActive != true || c.Connection.SecretHint == "" {
                t.Fatalf("secret bookkeeping missing: %+v", c.Connection)
        }
        if c.Replayed {
                t.Fatal("first connect must not be replayed")
        }
        // The sealed material never leaves the store; reads return only the hint.
        event := repo.lastEvent["connect"].(map[string]any)
        if len(c.Connection.Config["api_key"]) > 0 {
                t.Fatal("secret must not appear in config")
        }
        _ = event
        seal := repo.seals[c.Connection.SecretID]
        if seal == nil || len(seal.Sealed) != 32 || len(seal.Key) != 32 {
                t.Fatalf("seal malformed: %+v", seal)
        }
        // Read path returns no secret material.
        read, err := s.Read(ctx, identityDomain.Principal{ID: connActor}, connTenant, c.Connection.ConnectionID)
        if err != nil {
                t.Fatalf("read: %v", err)
        }
        if read.SecretHint == "sandbox-demo-123" || read.Config["api_key"] != "" {
                t.Fatal("read must never expose the secret value")
        }
}

func TestConnectReplayAfterNetworkFailure(t *testing.T) {
        s, repo := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        first, err := s.Connect(ctx, connectCommand(created.Connection.ConnectionID, 1, "connect-retry-key", "sandbox-first-key"))
        if err != nil {
                t.Fatalf("connect: %v", err)
        }
        // The same key with the same data replays the stored outcome — the
        // "secret already accepted" guard must NOT fire for an idempotent retry.
        retry, err := s.Connect(ctx, connectCommand(created.Connection.ConnectionID, 1, "connect-retry-key", "sandbox-first-key"))
        if err != nil {
                t.Fatalf("idempotent retry must replay, got %v", err)
        }
        if !retry.Replayed || retry.Connection.Version != first.Connection.Version {
                t.Fatalf("replay = %+v (first=%+v)", retry.Connection, first.Connection)
        }
        // The same key with different data is an honest conflict.
        conflicting := connectCommand(created.Connection.ConnectionID, 1, "connect-retry-key", "sandbox-other-key")
        if _, err = s.Connect(ctx, conflicting); !errors.Is(err, connectorDomain.ErrIdempotencyConflict) {
                t.Fatalf("conflicting retry = %v", err)
        }
        // No duplicate versions were created by the retries.
        if len(repo.connections) != 1 {
                t.Fatalf("replays must not duplicate work: %d", len(repo.connections))
        }
}

func TestConnectFailureIsHonest(t *testing.T) {
        s, _ := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        c, err := s.Connect(ctx, connectCommand(created.Connection.ConnectionID, 1, "connect-key-fail", "wrong-secret-value"))
        if err != nil {
                t.Fatalf("failed check must still record state, got %v", err)
        }
        if c.Connection.Status != connectorDomain.StatusFailed || c.Connection.AuthState != connectorDomain.CheckFailed {
                t.Fatalf("failed connection = %+v", c.Connection)
        }
}

func TestSecretAcceptedOnceRotationReplacesIt(t *testing.T) {
        s, repo := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        first, err := s.Connect(ctx, connectCommand(created.Connection.ConnectionID, 1, "rotate-key-a", "sandbox-first-key"))
        if err != nil {
                t.Fatalf("connect: %v", err)
        }
        second := connectCommand(created.Connection.ConnectionID, first.Connection.Version, "rotate-key-b", "not-sandbox-format")
        if _, err = s.Connect(ctx, second); err == nil {
                t.Fatal("double connect must be rejected; use rotation")
        }
        rotated, err := s.Rotate(ctx, second)
        if err != nil {
                t.Fatalf("rotate: %v", err)
        }
        if rotated.Connection.Version != first.Connection.Version+1 {
                t.Fatalf("rotation must create a new version: %d", rotated.Connection.Version)
        }
        oldSeal := repo.seals[first.Connection.SecretID]
        if oldSeal == nil || oldSeal.RevokedAt == nil {
                t.Fatal("previous seal must be revoked on rotation")
        }
        if rotated.Connection.Status != connectorDomain.StatusFailed {
                t.Fatalf("rotation with invalid secret must reset to failed honestly: %s", rotated.Connection.Status)
        }
}

func TestRevokeIsTerminalAndRevokesSeal(t *testing.T) {
        s, repo := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        connected, _ := s.Connect(ctx, connectCommand(created.Connection.ConnectionID, 1, "revoke-key-a", "sandbox-final-key"))
        revoked, err := s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connected.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: connected.Connection.Version, Reason: "Проект завершён", IdempotencyKey: "revoke-key-a"})
        if err != nil {
                t.Fatalf("revoke: %v", err)
        }
        if revoked.Connection.Status != connectorDomain.StatusRevoked || revoked.Connection.SecretActive {
                t.Fatalf("revoked = %+v", revoked.Connection)
        }
        if seal := repo.seals[connected.Connection.SecretID]; seal == nil || seal.RevokedAt == nil {
                t.Fatal("seal must be revoked with the connection")
        }
        if _, err = s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: revoked.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: revoked.Connection.Version, Reason: "Повторно", IdempotencyKey: "revoke-key-b"}); err == nil {
                t.Fatal("revoked connection must reject further commands")
        }
        if _, err = s.Save(ctx, api.SaveCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: revoked.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: revoked.Connection.Version, Config: revoked.Connection.Config, Label: revoked.Connection.Label, ChangeSummary: "Попытка"}); err == nil {
                t.Fatal("revoked connection must reject config changes")
        }
}

func TestRevokeReplayWithSameKey(t *testing.T) {
        s, _ := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        connected, _ := s.Connect(ctx, connectCommand(created.Connection.ConnectionID, 1, "replay-key-c", "sandbox-rev-key"))
        cmd := api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connected.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: connected.Connection.Version, Reason: "Завершение", IdempotencyKey: "revoke-replay-key"}
        first, err := s.Revoke(ctx, cmd)
        if err != nil {
                t.Fatalf("revoke: %v", err)
        }
        // Retry after a network failure: replay, not "already revoked".
        retry, err := s.Revoke(ctx, cmd)
        if err != nil || !retry.Replayed {
                t.Fatalf("idempotent revoke retry = %v %+v", err, retry)
        }
        if retry.Connection.Version != first.Connection.Version {
                t.Fatalf("replay must return the stored outcome version: %d vs %d", retry.Connection.Version, first.Connection.Version)
        }
}

func TestCrossTenantAccessDenied(t *testing.T) {
        s, _ := serviceFixture()
        ctx := context.Background()
        if _, err := s.List(ctx, identityDomain.Principal{ID: connActor}, connOtherTenant); !errors.Is(err, connectorDomain.ErrNotFound) {
                t.Fatalf("cross-tenant list = %v", err)
        }
        denied := New(newConnRepo(), connIdentity{allowed: false}, stubVerifier{}, connClock{}, &connIDs{})
        if _, err := denied.Catalog(ctx, identityDomain.Principal{ID: connActor}, connTenant); !errors.Is(err, connectorDomain.ErrNotFound) {
                t.Fatalf("unauthorized catalog = %v", err)
        }
}

func TestRevokeReasonBoundKeepsStoredSummaryInsideHistoryLimit(t *testing.T) {
        s, _ := serviceFixture()
        ctx := context.Background()
        first, err := s.Create(ctx, createCommand())
        if err != nil {
                t.Fatalf("create: %v", err)
        }
        // MaxRevokeReasonRunes fits with the "revoked: " prefix inside the 300-char
        // history CHECK; one rune more must be rejected up front with a 400, not a 500.
        ok := createCommand()
        ok.IdempotencyKey = "create-key-reason-ok"
        second, err := s.Create(ctx, ok)
        if err != nil {
                t.Fatalf("create second: %v", err)
        }
        if _, err = s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: first.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: first.Connection.Version, Reason: strings.Repeat("ф", connectorDomain.MaxRevokeReasonRunes), IdempotencyKey: "reason-ok-key"}); err != nil {
                t.Fatalf("max-length reason must be accepted: %v", err)
        }
        if _, err = s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: second.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: second.Connection.Version, Reason: strings.Repeat("ф", connectorDomain.MaxRevokeReasonRunes+1), IdempotencyKey: "reason-bad-key"}); err == nil {
                t.Fatal("reason above the bound must be rejected before storage")
        }
}

func TestCreateRejectsMultibyteShortIdempotencyKey(t *testing.T) {
        s, _ := serviceFixture()
        cmd := createCommand()
        // 7 Cyrillic runes = 14 bytes: byte-based validation used to let it pass
        // while the SQL CHECK (char_length) failed with a 500.
        cmd.IdempotencyKey = "ключик"
        if _, err := s.Create(context.Background(), cmd); !errors.Is(err, connectorDomain.ErrInvalidIdempotency) {
                t.Fatalf("multibyte short key must be rejected: %v", err)
        }
        valid := cmd
        valid.IdempotencyKey = "ключ-подлинный-длиный"
        if _, err := s.Create(context.Background(), valid); err != nil {
                t.Fatalf("multibyte key of enough runes must pass: %v", err)
        }
}

func TestFailedRotationLeavesNoActiveSecretAndAllowsConnectRetry(t *testing.T) {
        s, repo := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        first, err := s.Connect(ctx, connectCommand(created.Connection.ConnectionID, 1, "rotate-key-f1", "sandbox-first-key"))
        if err != nil {
                t.Fatalf("connect: %v", err)
        }
        rotate := connectCommand(created.Connection.ConnectionID, first.Connection.Version, "rotate-key-f2", "not-sandbox-format")
        rotated, err := s.Rotate(ctx, rotate)
        if err != nil {
                t.Fatalf("rotate: %v", err)
        }
        if rotated.Connection.Status != connectorDomain.StatusFailed {
                t.Fatalf("rotation with invalid secret must fail honestly: %s", rotated.Connection.Status)
        }
        // The previous seal was revoked and no new secret was accepted: no secret
        // may stay active, and a plain connect retry must be allowed.
        if rotated.Connection.SecretActive {
                t.Fatal("failed rotation must not leave a secret active over a revoked seal")
        }
        if seal := repo.seals[first.Connection.SecretID]; seal == nil || seal.RevokedAt == nil {
                t.Fatal("previous seal must be revoked on rotation")
        }
        retry := connectCommand(created.Connection.ConnectionID, rotated.Connection.Version, "rotate-key-f3", "sandbox-retry-key")
        second, err := s.Connect(ctx, retry)
        if err != nil {
                t.Fatalf("connect retry after failed rotation must be allowed: %v", err)
        }
        if second.Connection.Status != connectorDomain.StatusConnectedUnverified || !second.Connection.SecretActive {
                t.Fatalf("retry must seal the new secret: %+v", second.Connection)
        }
}

func TestUnexpectedStatusIsRejectedInsteadOfSilentWrite(t *testing.T) {
        repo := newConnRepo()
        s := New(repo, connIdentity{allowed: true}, stubVerifier{}, connClock{}, &connIDs{})
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        // Simulate a corrupted/unexpected stored status: the ValidTransition guard
        // must refuse commands instead of writing an undocumented transition.
        corrupt := repo.connections[created.Connection.ConnectionID]
        corrupt.Status = "mystery"
        repo.connections[created.Connection.ConnectionID] = corrupt
        if _, err := s.Save(ctx, api.SaveCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: 1, Config: corrupt.Config, Label: corrupt.Label, ChangeSummary: "Попытка"}); err == nil {
                t.Fatal("unexpected status must block save")
        }
        if _, err := s.Connect(ctx, connectCommand(created.Connection.ConnectionID, 1, "mystery-key-1", "sandbox-any-key")); err == nil {
                t.Fatal("unexpected status must block connect")
        }
        if _, err := s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: 1, Reason: "Причина", IdempotencyKey: "mystery-key-2"}); err == nil {
                t.Fatal("unexpected status must block revoke")
        }
}

func TestVerifyCapabilityLifecycleGuardsAndIdempotency(t *testing.T) {
        s, repo := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        connectionID := created.Connection.ConnectionID

        // Draft connection: verification is guarded.
        if _, err := s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.trunk-status", IdempotencyKey: "verify-key-d1"}); err == nil {
                t.Fatal("draft connection must refuse capability verification")
        }

        connected, _ := s.Connect(ctx, connectCommand(connectionID, 1, "verify-key-c1", "sandbox-verify-key"))

        // CapabilityState shows declared capabilities with honest states.
        state, err := s.CapabilityState(ctx, identityDomain.Principal{ID: connActor}, connTenant, connectionID)
        if err != nil {
                t.Fatalf("capability state: %v", err)
        }
        if len(state.Capabilities) != 8 {
                t.Fatalf("contract 1.2.0 declares 8 capabilities (3 prior + 5 SIP queries of spec 4-1), got %d", len(state.Capabilities))
        }
        byID := map[string]connectorDomain.CapabilityReport{}
        for _, report := range state.Capabilities {
                byID[report.ID] = report
        }
        if byID["query.trunk-status"].State != connectorDomain.CapStateUnverified || byID["query.trunk-status"].CanVerify != true {
                t.Fatalf("query must start unverified and verifiable: %+v", byID["query.trunk-status"])
        }
        if byID["command.send-sms"].State != connectorDomain.CapStateUnverified || byID["command.send-sms"].Block == nil {
                t.Fatalf("mutating capability must carry the Action Gateway block: %+v", byID["command.send-sms"])
        }
        if byID["event.inbound-call"].State != connectorDomain.CapStateUnverified {
                t.Fatalf("inbound event must stay unverified honestly: %+v", byID["event.inbound-call"])
        }

        // Mutating capability: verification is refused fail-closed.
        mutating := api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "command.send-sms", IdempotencyKey: "verify-key-m1"}
        if _, err = s.VerifyCapability(ctx, mutating); err == nil {
                t.Fatal("mutating capability verification must be refused")
        } else {
                var rule connectorDomain.RuleError
                if !errors.As(err, &rule) || !strings.Contains(rule.Reason, "Action Gateway") {
                        t.Fatalf("block reason must name Action Gateway: %v", err)
                }
        }

        // Inbound event: honest refusal.
        if _, err = s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "event.inbound-call", IdempotencyKey: "verify-key-e1"}); err == nil {
                t.Fatal("inbound event verification must be refused in this slice")
        }

        // Unknown capability: refused.
        if _, err = s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.unknown", IdempotencyKey: "verify-key-u1"}); err == nil {
                t.Fatal("undeclared capability must be refused")
        }

        // Happy path: read-only probe stores evidence and derives verified.
        verified, err := s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.trunk-status", IdempotencyKey: "verify-key-v1"})
        if err != nil {
                t.Fatalf("verify: %v", err)
        }
        if verified.Report.State != connectorDomain.CapStateVerified || verified.Report.Evidence == nil {
                t.Fatalf("verified report = %+v", verified.Report)
        }
        if verified.Report.Evidence.DeliveryLevel != string(connectorDomain.DeliveryDelivered) {
                t.Fatalf("delivery level = %s", verified.Report.Evidence.DeliveryLevel)
        }
        if len(repo.evidence) != 1 {
                t.Fatalf("exactly one evidence row expected: %d", len(repo.evidence))
        }

        // Same key + same data: replay without a contradicting row.
        replay, err := s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.trunk-status", IdempotencyKey: "verify-key-v1"})
        if err != nil || !replay.Replayed {
                t.Fatalf("verify replay = %v %+v", err, replay)
        }
        if len(repo.evidence) != 1 {
                t.Fatalf("replay must not add evidence: %d", len(repo.evidence))
        }

        // Same key + different capability: honest conflict.
        if _, err = s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "event.inbound-call", IdempotencyKey: "verify-key-v1"}); !errors.Is(err, connectorDomain.ErrIdempotencyConflict) {
                t.Fatalf("verify key conflict = %v", err)
        }

        // Rotation makes the evidence stale: derivation names the exact fact.
        rotated, err := s.Rotate(ctx, connectCommand(connectionID, connected.Connection.Version, "verify-key-r1", "sandbox-rotate-key"))
        if err != nil {
                t.Fatalf("rotate: %v", err)
        }
        if rotated.Connection.Status != connectorDomain.StatusConnectedUnverified {
                t.Fatalf("rotation with a valid key keeps the connection usable: %s", rotated.Connection.Status)
        }
        state, err = s.CapabilityState(ctx, identityDomain.Principal{ID: connActor}, connTenant, connectionID)
        if err != nil {
                t.Fatalf("state after rotation: %v", err)
        }
        for _, report := range state.Capabilities {
                if report.ID == "query.trunk-status" {
                        if report.State != connectorDomain.CapStateStale {
                                t.Fatalf("after rotation the old evidence must be stale, got %s", report.State)
                        }
                        if !strings.Contains(report.StateReason, "ротирован") {
                                t.Fatalf("stale reason must name the rotation: %s", report.StateReason)
                        }
                }
        }
}


// TestVerifyCapabilityExternalMACMatrix (Story 7-4 step-01): the probes of
// the external MCP class run AUTHENTICATED — the key material of the
// command is MAC-checked against the accepted seal BEFORE any network
// round. The matrix: the matching key runs the probe and stores evidence;
// the mismatching key refuses honestly with the MAC words; the missing key
// refuses with the unauthenticated-round words; the rotation invalidates
// the prior evidence anchor (the SecretID anchor of Applicable) and the
// old key stops matching the new seal.
func TestVerifyCapabilityExternalMACMatrix(t *testing.T) {
        s, repo := serviceFixture()
        ctx := context.Background()
        created, err := s.Create(ctx, api.CreateCommand{
                Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, IdempotencyKey: "create-key-ext1", CorrelationID: connCorrelation,
                ConnectorID: "staffy-external-mcp-server", ConnectorVer: "1.0.0", Label: "Внешний MCP-сервер", Environment: connectorDomain.EnvSandbox,
                Config: map[string]string{"endpoint": "https://demo.sandbox.example"},
        })
        if err != nil {
                t.Fatalf("create: %v", err)
        }
        connectionID := created.Connection.ConnectionID
        if _, err = s.Connect(ctx, connectCommand(connectionID, 1, "connect-key-ext1", "sandbox-external-key")); err != nil {
                t.Fatalf("connect: %v", err)
        }

        // The missing key: the unauthenticated round is impossible fail-closed.
        _, err = s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.server-discover", IdempotencyKey: "verify-ext-m1"})
        if err == nil {
                t.Fatal("проба без секрета обязана отказываться")
        }
        var rule connectorDomain.RuleError
        if !errors.As(err, &rule) || !strings.Contains(rule.Reason, "требует секрет подключения") {
                t.Fatalf("слова отказа отсутствия ключа: %v", err)
        }
        if len(repo.evidence) != 0 {
                t.Fatalf("evidence не пишется до MAC-проверки: %d", len(repo.evidence))
        }

        // The mismatching key: the honest MAC refusal BEFORE the network round.
        _, err = s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.server-discover", IdempotencyKey: "verify-ext-m2", Secrets: map[string]string{"api_key": "sandbox-wrong-key"}})
        if err == nil {
                t.Fatal("несовпадающий ключ обязан отказываться")
        }
        if !errors.As(err, &rule) || !strings.Contains(rule.Reason, "не совпадает с принятым подключением") {
                t.Fatalf("слова отказа MAC: %v", err)
        }
        if len(repo.evidence) != 0 {
                t.Fatalf("evidence не пишется при несовпадении MAC: %d", len(repo.evidence))
        }

        // The matching key: the probe runs and the evidence is stored.
        verified, err := s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.server-discover", IdempotencyKey: "verify-ext-v1", Secrets: map[string]string{"api_key": "sandbox-external-key"}})
        if err != nil {
                t.Fatalf("совпадающий ключ обязан допускать пробу: %v", err)
        }
        if verified.Report.State != connectorDomain.CapStateVerified || verified.Report.Evidence == nil {
                t.Fatalf("отчёт пробы: %+v", verified.Report)
        }

        // The tool-call capability stays honestly un-probeable (declaration ≠
        // claim): the guard refuses before the MAC check runs.
        _, err = s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.tool-call", IdempotencyKey: "verify-ext-t1", Secrets: map[string]string{"api_key": "sandbox-external-key"}})
        if err == nil {
                t.Fatal("query.tool-call обязан отказываться (ProbeAllowed=false)")
        }

        // The rotation: the new seal invalidates the old key AND the prior
        // evidence anchor (the SecretID of Applicable) — the state reads stale
        // until a fresh probe.
        if _, err = s.Rotate(ctx, connectCommand(connectionID, created.Connection.Version+1, "rotate-key-ext1", "sandbox-rotated-key")); err != nil {
                t.Fatalf("rotate: %v", err)
        }
        _, err = s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.server-discover", IdempotencyKey: "verify-ext-v2", Secrets: map[string]string{"api_key": "sandbox-external-key"}})
        if err == nil {
                t.Fatal("старый ключ после ротации обязан не совпадать с новым seal")
        }
        if !errors.As(err, &rule) || !strings.Contains(rule.Reason, "не совпадает с принятым подключением") {
                t.Fatalf("слова отказа ротации: %v", err)
        }
        state, err := s.CapabilityState(ctx, identityDomain.Principal{ID: connActor}, connTenant, connectionID)
        if err != nil {
                t.Fatalf("state: %v", err)
        }
        for _, report := range state.Capabilities {
                if report.ID == "query.server-discover" && report.State != connectorDomain.CapStateStale {
                        t.Fatalf("evidence после ротации обязан читаться stale: %+v", report)
                }
        }
        // The rotated key re-opens the probe; the state returns to verified.
        reVerified, err := s.VerifyCapability(ctx, api.VerifyCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connectionID, CorrelationID: connCorrelation, Capability: "query.server-discover", IdempotencyKey: "verify-ext-v3", Secrets: map[string]string{"api_key": "sandbox-rotated-key"}})
        if err != nil {
                t.Fatalf("ротированный ключ обязан допускать пробу: %v", err)
        }
        if reVerified.Report.State != connectorDomain.CapStateVerified {
                t.Fatalf("повторная проба обязана вернуть verified: %+v", reVerified.Report)
        }
}
