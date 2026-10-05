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

// connRepo is a fake repository implementing the full port.Repository.
type connRepo struct {
        connections map[id.UUID]connectorDomain.Connection
        seals       map[id.UUID]*connectorDomain.Seal
        idempotency map[string]port.CreateParams
        lastEvent   map[string]any
}

func newConnRepo() *connRepo {
        return &connRepo{connections: map[id.UUID]connectorDomain.Connection{}, seals: map[id.UUID]*connectorDomain.Seal{}, idempotency: map[string]port.CreateParams{}, lastEvent: map[string]any{}}
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

func (r *connRepo) Connect(_ context.Context, p port.ConnectParams) (connectorDomain.Connection, error) {
        c, ok := r.connections[p.ConnectionID]
        if !ok {
                return connectorDomain.Connection{}, connectorDomain.ErrNotFound
        }
        if c.Version != p.ExpectedVersion {
                return connectorDomain.Connection{}, connectorDomain.ErrConflict
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
        return c, nil
}

func (r *connRepo) Revoke(_ context.Context, p port.RevokeParams) (connectorDomain.Connection, error) {
        c, ok := r.connections[p.ConnectionID]
        if !ok {
                return connectorDomain.Connection{}, connectorDomain.ErrNotFound
        }
        if c.Version != p.ExpectedVersion {
                return connectorDomain.Connection{}, connectorDomain.ErrConflict
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
        return c, nil
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

func createCommand() api.CreateCommand {
        return api.CreateCommand{
                Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, IdempotencyKey: "create-key-123", CorrelationID: connCorrelation,
                ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.0.0", Label: "Тестовая телефония", Environment: connectorDomain.EnvSandbox,
                Config: map[string]string{"endpoint": "https://demo.sandbox.example"},
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
        connect := api.ConnectCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: created.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-demo-123"}}
        c, err := s.Connect(ctx, connect)
        if err != nil {
                t.Fatalf("connect: %v", err)
        }
        if c.Status != connectorDomain.StatusConnectedUnverified {
                t.Fatalf("status = %s, want connected_unverified", c.Status)
        }
        if c.AuthState != connectorDomain.CheckVerified || c.Transport != connectorDomain.CheckVerified {
                t.Fatalf("states = %s/%s", c.AuthState, c.Transport)
        }
        if c.SecretActive != true || c.SecretHint == "" {
                t.Fatalf("secret bookkeeping missing: %+v", c)
        }
        // The sealed material never leaves the store; reads return only the hint.
        event := repo.lastEvent["connect"].(map[string]any)
        if len(c.Config["api_key"]) > 0 {
                t.Fatal("secret must not appear in config")
        }
        _ = event
        seal := repo.seals[c.SecretID]
        if seal == nil || len(seal.Sealed) != 32 || len(seal.Key) != 32 {
                t.Fatalf("seal malformed: %+v", seal)
        }
        // Read path returns no secret material.
        read, err := s.Read(ctx, identityDomain.Principal{ID: connActor}, connTenant, c.ConnectionID)
        if err != nil {
                t.Fatalf("read: %v", err)
        }
        if read.SecretHint == "sandbox-demo-123" || read.Config["api_key"] != "" {
                t.Fatal("read must never expose the secret value")
        }
}

func TestConnectFailureIsHonest(t *testing.T) {
        s, _ := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        connect := api.ConnectCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: 1, Secrets: map[string]string{"api_key": "wrong-secret-value"}}
        c, err := s.Connect(ctx, connect)
        if err != nil {
                t.Fatalf("failed check must still record state, got %v", err)
        }
        if c.Status != connectorDomain.StatusFailed || c.AuthState != connectorDomain.CheckFailed {
                t.Fatalf("failed connection = %+v", c)
        }
}

func TestSecretAcceptedOnceRotationReplacesIt(t *testing.T) {
        s, repo := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        connect := api.ConnectCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: 1, Secrets: map[string]string{"api_key": "sandbox-first-key"}}
        first, err := s.Connect(ctx, connect)
        if err != nil {
                t.Fatalf("connect: %v", err)
        }
        second := connect
        second.ExpectedVersion = first.Version
        second.Secrets = map[string]string{"api_key": "not-sandbox-format"}
        if _, err = s.Connect(ctx, second); err == nil {
                t.Fatal("double connect must be rejected; use rotation")
        }
        rotated, err := s.Rotate(ctx, second)
        if err != nil {
                t.Fatalf("rotate: %v", err)
        }
        if rotated.Version != first.Version+1 {
                t.Fatalf("rotation must create a new version: %d", rotated.Version)
        }
        oldSeal := repo.seals[first.SecretID]
        if oldSeal == nil || oldSeal.RevokedAt == nil {
                t.Fatal("previous seal must be revoked on rotation")
        }
        if rotated.Status != connectorDomain.StatusFailed {
                t.Fatalf("rotation with invalid secret must reset to failed honestly: %s", rotated.Status)
        }
}

func TestRevokeIsTerminalAndRevokesSeal(t *testing.T) {
        s, repo := serviceFixture()
        ctx := context.Background()
        created, _ := s.Create(ctx, createCommand())
        connect := api.ConnectCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: 1, Secrets: map[string]string{"api_key": "sandbox-final-key"}}
        connected, _ := s.Connect(ctx, connect)
        revoked, err := s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: connected.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: connected.Version, Reason: "Проект завершён"})
        if err != nil {
                t.Fatalf("revoke: %v", err)
        }
        if revoked.Status != connectorDomain.StatusRevoked || revoked.SecretActive {
                t.Fatalf("revoked = %+v", revoked)
        }
        if seal := repo.seals[connected.SecretID]; seal == nil || seal.RevokedAt == nil {
                t.Fatal("seal must be revoked with the connection")
        }
        if _, err = s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: revoked.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: revoked.Version, Reason: "Повторно"}); err == nil {
                t.Fatal("revoked connection must reject further commands")
        }
        if _, err = s.Save(ctx, api.SaveCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: revoked.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: revoked.Version, Config: revoked.Config, Label: revoked.Label, ChangeSummary: "Попытка"}); err == nil {
                t.Fatal("revoked connection must reject config changes")
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
        if _, err = s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: first.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: first.Connection.Version, Reason: strings.Repeat("ф", connectorDomain.MaxRevokeReasonRunes)}); err != nil {
                t.Fatalf("max-length reason must be accepted: %v", err)
        }
        if _, err = s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: second.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: second.Connection.Version, Reason: strings.Repeat("ф", connectorDomain.MaxRevokeReasonRunes+1)}); err == nil {
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
        first, err := s.Connect(ctx, api.ConnectCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: 1, Secrets: map[string]string{"api_key": "sandbox-first-key"}})
        if err != nil {
                t.Fatalf("connect: %v", err)
        }
        rotate := api.ConnectCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: first.Version, Secrets: map[string]string{"api_key": "not-sandbox-format"}}
        rotated, err := s.Rotate(ctx, rotate)
        if err != nil {
                t.Fatalf("rotate: %v", err)
        }
        if rotated.Status != connectorDomain.StatusFailed {
                t.Fatalf("rotation with invalid secret must fail honestly: %s", rotated.Status)
        }
        // The previous seal was revoked and no new secret was accepted: no secret
        // may stay active, and a plain connect retry must be allowed.
        if rotated.SecretActive {
                t.Fatal("failed rotation must not leave a secret active over a revoked seal")
        }
        if seal := repo.seals[first.SecretID]; seal == nil || seal.RevokedAt == nil {
                t.Fatal("previous seal must be revoked on rotation")
        }
        retry := rotate
        retry.ExpectedVersion = rotated.Version
        retry.Secrets = map[string]string{"api_key": "sandbox-retry-key"}
        second, err := s.Connect(ctx, retry)
        if err != nil {
                t.Fatalf("connect retry after failed rotation must be allowed: %v", err)
        }
        if second.Status != connectorDomain.StatusConnectedUnverified || !second.SecretActive {
                t.Fatalf("retry must seal the new secret: %+v", second)
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
        if _, err := s.Connect(ctx, api.ConnectCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: 1, Secrets: map[string]string{"api_key": "sandbox-any-key"}}); err == nil {
                t.Fatal("unexpected status must block connect")
        }
        if _, err := s.Revoke(ctx, api.RevokeCommand{Actor: identityDomain.Principal{ID: connActor}, TenantID: connTenant, ConnectionID: created.Connection.ConnectionID, CorrelationID: connCorrelation, ExpectedVersion: 1, Reason: "Причина"}); err == nil {
                t.Fatal("unexpected status must block revoke")
        }
}
