package main

import (
        "context"
        "encoding/binary"
        "fmt"
        "io"
        "log/slog"
        "net/http"
        "sort"
        "sync"
        "time"

        employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
        employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
        "github.com/artemiosu/staffy/internal/employee/ingest"
        "github.com/artemiosu/staffy/internal/employee/packs"
        connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
        connectorCatalog "github.com/artemiosu/staffy/internal/connector/catalog"
        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        "github.com/artemiosu/staffy/internal/connector/verifier"
        "github.com/artemiosu/staffy/internal/identity/api"
        "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        httptransport "github.com/artemiosu/staffy/internal/transport/http"
)

var (
        principalID = id.MustParse("01994f64-6c00-7000-8000-000000000001")
        tenantID    = id.MustParse("01994f64-6c00-7000-8000-000000000002")
)

type identityFixture struct{}

func (identityFixture) CreateTenant(context.Context, api.CreateTenantCommand) (api.CreateTenantResult, error) {
        return api.CreateTenantResult{Tenant: domain.Tenant{ID: tenantID, Name: "Магазин Олега"}, Role: "Owner"}, nil
}
func (identityFixture) Workspace(context.Context, domain.Principal, id.UUID) (domain.Workspace, error) {
        return domain.Workspace{Tenant: domain.Tenant{ID: tenantID, Name: "Магазин Олега"}, Role: "owner"}, nil
}
func (identityFixture) ListMembers(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error) {
        return []domain.Membership{{TenantID: tenantID, PrincipalID: &principalID, Roles: []domain.Role{domain.Owner}, Scopes: domain.AllScopes(), Status: "active", Version: 1}}, nil
}
func (identityFixture) InviteMember(context.Context, api.MembershipCommand) (domain.Membership, string, error) {
        return domain.Membership{}, "", domain.ErrStepUpRequired
}
func (identityFixture) AcceptInvitation(context.Context, domain.Principal, string, id.UUID) (domain.Membership, error) {
        return domain.Membership{}, domain.ErrNotFound
}
func (identityFixture) ChangeMember(context.Context, api.MembershipCommand) (domain.Membership, error) {
        return domain.Membership{}, domain.ErrStepUpRequired
}
func (identityFixture) RevokeMember(context.Context, api.MembershipCommand) error {
        return domain.ErrStepUpRequired
}
func (identityFixture) Authorize(context.Context, domain.Principal, id.UUID, domain.Scope) error {
        return nil
}

type resolverFixture struct{}

func (resolverFixture) Resolve(context.Context) (domain.Principal, error) {
        return domain.Principal{ID: principalID, CanCreateTenant: true}, nil
}

type clockFixture struct{}

func (clockFixture) Now() time.Time { return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC) }

type idsFixture struct{}

func (idsFixture) New(time.Time) (id.UUID, error) {
        return id.MustParse("01994f64-6c00-7000-8000-000000000003"), nil
}

type employeeFixture struct {
        mu               sync.Mutex
        draft            *employeeDomain.Draft
        behavior         []employeeDomain.BehaviorVersion
        voice            []employeeDomain.VoiceVersion
        voicePreviews    map[id.UUID]employeeDomain.VoicePreviewRecord
        voiceAssessments []employeeDomain.VoiceAssessment
        knowledge        *employeeDomain.KnowledgeSource
        versions         []employeeDomain.KnowledgeVersion
}

func (f *employeeFixture) VoiceCatalog(_ context.Context, _ domain.Principal, tenant id.UUID) ([]employeeDomain.VoiceProfile, error) {
        if tenant != tenantID {
                return nil, employeeDomain.ErrNotFound
        }
        profiles := employeeDomain.VoiceCatalog()
        for i := range profiles {
                profiles[i].PreviewStatus = "available"
        }
        return profiles, nil
}
func (f *employeeFixture) ReadVoice(_ context.Context, _ domain.Principal, tenant, employee id.UUID) ([]employeeDomain.VoiceVersion, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || tenant != tenantID || employee != fixtureEmployeeID {
                return nil, employeeDomain.ErrNotFound
        }
        return append([]employeeDomain.VoiceVersion(nil), f.voice...), nil
}
func (f *employeeFixture) SaveVoice(_ context.Context, c employeeAPI.VoiceCommand) (employeeDomain.VoiceVersion, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID {
                return employeeDomain.VoiceVersion{}, employeeDomain.ErrNotFound
        }
        if f.draft.Version != c.ExpectedVersion {
                return employeeDomain.VoiceVersion{}, employeeDomain.ErrConflict
        }
        if err := employeeDomain.ValidateVoice(f.draft.Brief, c.Config); err != nil {
                return employeeDomain.VoiceVersion{}, err
        }
        v := employeeDomain.VoiceVersion{TenantID: tenantID, EmployeeID: fixtureEmployeeID, Version: int64(len(f.voice) + 1), DraftVersion: f.draft.Version + 1, Config: c.Config, ActorID: c.Actor.ID, ChangedAt: time.Now().UTC(), ChangeSummary: c.ChangeSummary}
        f.voice = append([]employeeDomain.VoiceVersion{v}, f.voice...)
        f.draft.Version++
        return v, nil
}
func (f *employeeFixture) PreviewVoice(ctx context.Context, c employeeAPI.VoicePreviewRequest) (employeeAPI.VoicePreview, error) {
        if !c.SyntheticDataConfirmed {
                return employeeAPI.VoicePreview{}, employeeDomain.ErrInvalid
        }
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID {
                return employeeAPI.VoicePreview{}, employeeDomain.ErrNotFound
        }
        if len(f.voice) == 0 || f.voice[0].Version != c.VoiceVersion {
                return employeeAPI.VoicePreview{}, employeeDomain.ErrConflict
        }
        synthesisText, _, err := employeeDomain.ResolveSynthesisText(f.voice[0].Config, c.Text)
        if err != nil {
                return employeeAPI.VoicePreview{}, err
        }
        preview := employeeAPI.VoicePreview{Text: c.Text, SynthesisText: synthesisText, VoiceVersion: c.VoiceVersion, DictionaryVersion: c.VoiceVersion, DraftVersion: f.voice[0].DraftVersion, Route: "offline-test-tts", Capability: "unavailable", PhoneQualityApproved: false}
        preview.Audio = fixtureWAV()
        preview.AudioType = "audio/wav"
        preview.Capability = "available"
        preview.PreviewID, _ = id.NewV7(time.Now().UTC())
        if f.voicePreviews == nil {
                f.voicePreviews = map[id.UUID]employeeDomain.VoicePreviewRecord{}
        }
        f.voicePreviews[preview.PreviewID] = employeeDomain.VoicePreviewRecord{TenantID: tenantID, EmployeeID: fixtureEmployeeID, PreviewID: preview.PreviewID, VoiceVersion: c.VoiceVersion, DictionaryVersion: c.VoiceVersion, DraftVersion: f.voice[0].DraftVersion, Text: c.Text, SynthesisText: synthesisText, Route: preview.Route, Slow: c.Slow, ActorID: c.Actor.ID, CreatedAt: time.Now().UTC()}
        return preview, nil
}

func fixtureWAV() []byte {
        const sampleRate = 8000
        data := make([]byte, sampleRate/10)
        wav := make([]byte, 44+len(data))
        copy(wav[0:4], "RIFF")
        binary.LittleEndian.PutUint32(wav[4:8], uint32(len(wav)-8))
        copy(wav[8:12], "WAVE")
        copy(wav[12:16], "fmt ")
        binary.LittleEndian.PutUint32(wav[16:20], 16)
        binary.LittleEndian.PutUint16(wav[20:22], 1)
        binary.LittleEndian.PutUint16(wav[22:24], 1)
        binary.LittleEndian.PutUint32(wav[24:28], sampleRate)
        binary.LittleEndian.PutUint32(wav[28:32], sampleRate)
        binary.LittleEndian.PutUint16(wav[32:34], 1)
        binary.LittleEndian.PutUint16(wav[34:36], 8)
        copy(wav[36:40], "data")
        binary.LittleEndian.PutUint32(wav[40:44], uint32(len(data)))
        copy(wav[44:], data)
        return wav
}
func (f *employeeFixture) ReadVoiceAssessments(_ context.Context, _ domain.Principal, tenant, employee id.UUID) ([]employeeDomain.VoiceAssessment, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || tenant != tenantID || employee != fixtureEmployeeID {
                return nil, employeeDomain.ErrNotFound
        }
        out := append([]employeeDomain.VoiceAssessment(nil), f.voiceAssessments...)
        for i := range out {
                out[i].Stale = out[i].VoiceVersion != f.voice[0].Version || out[i].DraftVersion != f.draft.Version
        }
        return out, nil
}
func (f *employeeFixture) AssessVoice(_ context.Context, c employeeAPI.VoiceAssessmentCommand) (employeeDomain.VoiceAssessment, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID {
                return employeeDomain.VoiceAssessment{}, employeeDomain.ErrNotFound
        }
        p, ok := f.voicePreviews[c.PreviewID]
        if !ok {
                return employeeDomain.VoiceAssessment{}, employeeDomain.ErrNotFound
        }
        if p.VoiceVersion != c.VoiceVersion || p.VoiceVersion != f.voice[0].Version || p.DraftVersion != f.draft.Version {
                return employeeDomain.VoiceAssessment{}, employeeDomain.ErrConflict
        }
        if c.Verdict != "approved" && c.Verdict != "needs_revision" {
                return employeeDomain.VoiceAssessment{}, employeeDomain.ErrInvalid
        }
        for _, a := range f.voiceAssessments {
                if a.PreviewID == c.PreviewID {
                        return employeeDomain.VoiceAssessment{}, employeeDomain.ErrConflict
                }
        }
        assessmentID, _ := id.NewV7(time.Now().UTC())
        a := employeeDomain.VoiceAssessment{VoicePreviewRecord: p, AssessmentID: assessmentID, Verdict: c.Verdict, AssessedBy: c.Actor.ID, AssessedAt: time.Now().UTC()}
        f.voiceAssessments = append([]employeeDomain.VoiceAssessment{a}, f.voiceAssessments...)
        return a, nil
}

func (f *employeeFixture) ReadBehavior(_ context.Context, _ domain.Principal, tenant, employee id.UUID) ([]employeeDomain.BehaviorVersion, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || tenant != tenantID || employee != fixtureEmployeeID {
                return nil, employeeDomain.ErrNotFound
        }
        return append([]employeeDomain.BehaviorVersion(nil), f.behavior...), nil
}
func (f *employeeFixture) PreviewBehavior(_ context.Context, c employeeAPI.BehaviorCommand) (employeeDomain.BehaviorPreview, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID {
                return employeeDomain.BehaviorPreview{}, employeeDomain.ErrNotFound
        }
        if c.ExpectedVersion != f.draft.Version {
                return employeeDomain.BehaviorPreview{}, employeeDomain.ErrConflict
        }
        if f.draft.Pack == nil {
                return employeeDomain.BehaviorPreview{}, employeeDomain.BehaviorRuleError{Rule: "Domain Pack", Reason: "Сначала закрепите пакет."}
        }
        if err := employeeDomain.ValidateBehavior(c.Config); err != nil {
                return employeeDomain.BehaviorPreview{}, err
        }
        claims := make([]employeeDomain.AllowedClaim, len(c.Config.Claims))
        copy(claims, c.Config.Claims)
        for i := range claims {
                claims[i].Status = "blocked"
                claims[i].BlockReason = "claim: проверяемая привязка утверждения к доказательству отсутствует"
        }
        preview := employeeDomain.BehaviorPreview{Mode: c.Config.Modes[0], Claims: claims, Blockers: []string{"Публикация и реальные действия недоступны в черновике.", "human_requested: человек и маршрут не подтверждены"}, Handoff: "blocked: человек и маршрут не подтверждены", SyntheticOnly: true, Publishable: false}
        switch c.Scenario {
        case "", "general":
        case "refusal":
                preview.Mode = "blocked"
                preview.Blockers = append(preview.Blockers, "cancellation/sales: после отказа убеждение прекращается")
        case "no_contact":
                preview.Mode = "blocked"
                preview.Blockers = append(preview.Blockers, "privacy/no_contact: дальнейший контакт запрещён")
        case "safety":
                preview.Mode = "blocked"
                preview.Blockers = append(preview.Blockers, "safety: передать человеку; продажи прекращены")
        case "human_requested":
                preview.Mode = "blocked"
                preview.Blockers = append(preview.Blockers, "human_requested: без подтверждённого человека handoff заблокирован")
        case "business_action":
                preview.Mode = "blocked"
                preview.Blockers = append(preview.Blockers, "Confirmation: черновик не разрешает Business Action")
        default:
                return employeeDomain.BehaviorPreview{}, employeeDomain.BehaviorRuleError{Rule: "scenario", Reason: "Неизвестный сценарий запрещён."}
        }
        if preview.Mode == "sales" || preview.Mode == "handoff" || len(preview.Claims) > 0 {
                preview.Mode = "blocked"
        }
        return preview, nil
}
func (f *employeeFixture) SaveBehavior(ctx context.Context, c employeeAPI.BehaviorCommand) (employeeDomain.BehaviorVersion, error) {
        preview, err := f.PreviewBehavior(ctx, c)
        if err != nil {
                return employeeDomain.BehaviorVersion{}, err
        }
        f.mu.Lock()
        defer f.mu.Unlock()
        f.draft.Version++
        config := c.Config
        config.Claims = preview.Claims
        v := employeeDomain.BehaviorVersion{TenantID: tenantID, EmployeeID: fixtureEmployeeID, Version: int64(len(f.behavior) + 1), DraftVersion: f.draft.Version, PackID: f.draft.Pack.ID, PackVersion: f.draft.Pack.Version, Config: config, ActorID: c.Actor.ID, ChangedAt: time.Now().UTC(), ChangeSummary: c.ChangeSummary}
        f.behavior = append([]employeeDomain.BehaviorVersion{v}, f.behavior...)
        return v, nil
}

var fixtureEmployeeID = id.MustParse("01994f64-6c00-7000-8000-000000000004")
var fixtureSourceID = id.MustParse("01994f64-6c00-7000-8000-000000000005")
var fixtureConnectionID = id.MustParse("01994f64-6c00-7000-8000-000000000006")
var fixtureSecretID = id.MustParse("01994f64-6c00-7000-8000-000000000007")
var fixtureSecretID2 = id.MustParse("01994f64-6c00-7000-8000-000000000009")
var fixtureEvidenceID = id.MustParse("01994f64-6c00-7000-8000-000000000008")

// connectorFixture serves the integrations journey with the real binary
// catalog and the real synthetic verifier, keeping state in memory only.
type connectorFixture struct {
        mu          sync.Mutex
        connections map[id.UUID]connectorDomain.Connection
        evidence    map[string]connectorDomain.CapabilityEvidence // key: connectionID + ":" + idempotency key
        next        int
        sealSeq     int
}

var fixtureVerifier = verifier.Synthetic{}

func (f *connectorFixture) Catalog(context.Context, domain.Principal, id.UUID) ([]connectorDomain.ConnectorDefinition, error) {
        return connectorCatalog.List(), nil
}

func (f *connectorFixture) Create(_ context.Context, c connectorAPI.CreateCommand) (connectorAPI.CreateResult, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if c.TenantID != tenantID {
                return connectorAPI.CreateResult{}, connectorDomain.ErrNotFound
        }
        def, ok := connectorCatalog.Get(c.ConnectorID, c.ConnectorVer)
        if !ok || !def.Connectable() {
                return connectorAPI.CreateResult{}, connectorDomain.RuleError{Rule: "connector", Reason: "Адаптер этого Connector не реализован; подключение недоступно. Следите за обновлениями каталога."}
        }
        if err := connectorDomain.ValidateEnvironment(c.Environment); err != nil {
                return connectorAPI.CreateResult{}, err
        }
        if err := def.ValidateConfig(c.Config); err != nil {
                return connectorAPI.CreateResult{}, err
        }
        if f.connections == nil {
                f.connections = map[id.UUID]connectorDomain.Connection{}
        }
        f.next++
        connectionID := fixtureConnectionID
        if f.next > 1 {
                connectionID = id.MustParse("01994f64-6c00-7000-8000-0000000000" + string(rune('0'+f.next)) + "0")
        }
        c2 := connectorDomain.Connection{
                TenantID: tenantID, ConnectionID: connectionID, ConnectorID: def.ID, ConnectorVer: def.Version,
                Label: c.Label, Environment: c.Environment, Status: connectorDomain.StatusDraft,
                AuthState: connectorDomain.CheckPending, Transport: connectorDomain.CheckPending,
                Config: c.Config, OwnerID: c.Actor.ID, Version: 1,
                CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
        }
        f.connections[connectionID] = c2
        return connectorAPI.CreateResult{Connection: c2}, nil
}

func (f *connectorFixture) List(_ context.Context, _ domain.Principal, tenant id.UUID) ([]connectorDomain.Connection, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if tenant != tenantID {
                return nil, connectorDomain.ErrNotFound
        }
        out := []connectorDomain.Connection{}
        for _, c := range f.connections {
                out = append(out, c)
        }
        sort.Slice(out, func(i, j int) bool {
                if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
                        return out[i].UpdatedAt.After(out[j].UpdatedAt)
                }
                return out[i].ConnectionID < out[j].ConnectionID
        })
        return out, nil
}

func (f *connectorFixture) Read(_ context.Context, _ domain.Principal, tenant, connectionID id.UUID) (connectorDomain.Connection, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if tenant != tenantID {
                return connectorDomain.Connection{}, connectorDomain.ErrNotFound
        }
        c, ok := f.connections[connectionID]
        if !ok {
                return connectorDomain.Connection{}, connectorDomain.ErrNotFound
        }
        return c, nil
}

func (f *connectorFixture) ReadVersions(_ context.Context, _ domain.Principal, tenant, connectionID id.UUID) ([]connectorDomain.ConnectionVersion, error) {
        c, err := f.Read(context.Background(), domain.Principal{}, tenant, connectionID)
        if err != nil {
                return nil, err
        }
        return []connectorDomain.ConnectionVersion{{Connection: c, ActorID: c.OwnerID, ChangeSummary: "created"}}, nil
}

func (f *connectorFixture) Save(_ context.Context, c connectorAPI.SaveCommand) (connectorDomain.Connection, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        current, ok := f.connections[c.ConnectionID]
        if !ok || c.TenantID != tenantID {
                return connectorDomain.Connection{}, connectorDomain.ErrNotFound
        }
        if current.Version != c.ExpectedVersion {
                return connectorDomain.Connection{}, connectorDomain.ErrConflict
        }
        current.Version++
        current.Label = c.Label
        current.Config = c.Config
        current.Status = connectorDomain.StatusDraft
        current.AuthState = connectorDomain.CheckPending
        current.Transport = connectorDomain.CheckPending
        current.UpdatedAt = time.Now().UTC()
        f.connections[c.ConnectionID] = current
        return current, nil
}

func (f *connectorFixture) Connect(_ context.Context, c connectorAPI.ConnectCommand) (connectorAPI.ConnectResult, error) {
        connection, err := f.check(c, false)
        return connectorAPI.ConnectResult{Connection: connection}, err
}

func (f *connectorFixture) Rotate(_ context.Context, c connectorAPI.ConnectCommand) (connectorAPI.ConnectResult, error) {
        connection, err := f.check(c, true)
        return connectorAPI.ConnectResult{Connection: connection}, err
}

func (f *connectorFixture) check(c connectorAPI.ConnectCommand, rotate bool) (connectorDomain.Connection, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        current, ok := f.connections[c.ConnectionID]
        if !ok || c.TenantID != tenantID {
                return connectorDomain.Connection{}, connectorDomain.ErrNotFound
        }
        if current.Version != c.ExpectedVersion {
                return connectorDomain.Connection{}, connectorDomain.ErrConflict
        }
        if current.Status == connectorDomain.StatusRevoked {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "status", Reason: "Отозванное подключение нельзя проверить; создайте новое."}
        }
        if !rotate && current.SecretActive {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "secret", Reason: "Секрет уже принят. Для замены используйте ротацию."}
        }
        def, _ := connectorCatalog.Get(current.ConnectorID, current.ConnectorVer)
        check := fixtureVerifier.BasicCheck(context.Background(), def, current.Environment, current.Config, c.Secrets)
        current.Version++
        current.AuthState = check.Authentication
        current.Transport = check.Transport
        if check.Authentication == connectorDomain.CheckVerified {
                // Mirrors the real store: every accepted seal is a NEW secret id,
                // so rotation honestly invalidates prior evidence anchors.
                f.sealSeq++
                current.SecretID = id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", 99+f.sealSeq))
                if f.sealSeq == 1 {
                        current.SecretID = fixtureSecretID
                }
                current.SecretHint = "a1b2c3d4e5f6"
                current.SecretActive = true
        }
        if check.Authentication == connectorDomain.CheckVerified && check.Transport == connectorDomain.CheckVerified {
                current.Status = connectorDomain.StatusConnectedUnverified
        } else {
                current.Status = connectorDomain.StatusFailed
        }
        current.UpdatedAt = time.Now().UTC()
        f.connections[c.ConnectionID] = current
        return current, nil
}

func (f *connectorFixture) Revoke(_ context.Context, c connectorAPI.RevokeCommand) (connectorAPI.RevokeResult, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        current, ok := f.connections[c.ConnectionID]
        if !ok || c.TenantID != tenantID {
                return connectorAPI.RevokeResult{}, connectorDomain.ErrNotFound
        }
        if current.Version != c.ExpectedVersion {
                return connectorAPI.RevokeResult{}, connectorDomain.ErrConflict
        }
        current.Version++
        current.Status = connectorDomain.StatusRevoked
        current.SecretActive = false
        current.UpdatedAt = time.Now().UTC()
        f.connections[c.ConnectionID] = current
        return connectorAPI.RevokeResult{Connection: current}, nil
}

// VerifyCapability mirrors the real service contract: fail-closed guards,
// deterministic probe via the real synthetic verifier, in-memory evidence
// with idempotent replay by key.
func (f *connectorFixture) VerifyCapability(_ context.Context, c connectorAPI.VerifyCommand) (connectorAPI.VerifyResult, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        current, ok := f.connections[c.ConnectionID]
        if !ok || c.TenantID != tenantID {
                return connectorAPI.VerifyResult{}, connectorDomain.ErrNotFound
        }
        if current.Status != connectorDomain.StatusConnectedUnverified {
                return connectorAPI.VerifyResult{}, connectorDomain.RuleError{Rule: "status", Reason: "Подключение в черновике: сначала выполните базовую проверку (подключение секрета), затем проверяйте возможности."}
        }
        def, _ := connectorCatalog.Get(current.ConnectorID, current.ConnectorVer)
        var dec connectorDomain.CapabilityDeclaration
        declared := false
        for _, candidate := range def.Capabilities {
                if candidate.ID == c.Capability {
                        dec, declared = candidate, true
                }
        }
        if !declared {
                return connectorAPI.VerifyResult{}, connectorDomain.RuleError{Rule: "capability", Reason: "Capability не заявлена контрактом версии " + current.ConnectorVer + "."}
        }
        if dec.Mutating() {
                return connectorAPI.VerifyResult{}, connectorDomain.RuleError{Rule: "capability", Reason: "Мутирующая capability: проверка заблокирована до Action Gateway (Epic 3), гейт AG-4, ответственный Technical Lead + Security. Read-only возможности этого подключения доступны для проверки независимо."}
        }
        if dec.Operation == connectorDomain.OperationEvent {
                return connectorAPI.VerifyResult{}, connectorDomain.RuleError{Rule: "capability", Reason: "Путь проверки входящих событий не реализован в текущем срезе: capability остаётся unverified без ложного успеха."}
        }
        if !dec.ProbeVerifiable() {
                return connectorAPI.VerifyResult{}, connectorDomain.RuleError{Rule: "capability", Reason: "Эта capability не разрешена для read-only probe: проверка отклонена (fail-closed)."}
        }
        if f.evidence == nil {
                f.evidence = map[string]connectorDomain.CapabilityEvidence{}
        }
        slot := c.ConnectionID.String() + ":" + c.IdempotencyKey
        if _, seen := f.evidence[slot]; seen {
                state, err := f.capabilityStateLocked(current, def)
                if err != nil {
                        return connectorAPI.VerifyResult{}, err
                }
                for _, report := range state.Capabilities {
                        if report.ID == dec.ID {
                                return connectorAPI.VerifyResult{Report: report, Replayed: true}, nil
                        }
                }
        }
        now := time.Now().UTC()
        probe := fixtureVerifier.Probe(context.Background(), def, dec, current.Environment, current.Config)
        row := connectorDomain.CapabilityEvidence{
                TenantID: tenantID, EvidenceID: fixtureEvidenceID, ConnectionID: current.ConnectionID, SecretID: current.SecretID,
                Environment: current.Environment, ContractVersion: current.ConnectorVer, AdapterBuild: fixtureVerifier.Build(),
                Capability: dec.ID, ProbeKind: connectorDomain.EvidenceProbeKind, ActorID: c.Actor.ID,
                RequestedAt: now, DeadlineAt: now.Add(time.Duration(dec.TimeoutMS) * time.Millisecond), CompletedAt: now,
                Outcome: probe.Outcome, DeliveryLevel: probe.DeliveryLevel,
                RedactedRequest: probe.RedactedRequest, RedactedResult: probe.RedactedResult,
                ExpiresAt: now.Add(24 * time.Hour), IdempotencyKey: c.IdempotencyKey,
        }
        f.evidence[slot] = row
        state, err := f.capabilityStateLocked(current, def)
        if err != nil {
                return connectorAPI.VerifyResult{}, err
        }
        for _, report := range state.Capabilities {
                if report.ID == dec.ID {
                        return connectorAPI.VerifyResult{Report: report}, nil
                }
        }
        return connectorAPI.VerifyResult{}, connectorDomain.ErrInvalid
}

// CapabilityState derives the honest view with the real domain function.
func (f *connectorFixture) CapabilityState(_ context.Context, _ domain.Principal, tenant, connectionID id.UUID) (connectorAPI.CapabilityStateResult, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if tenant != tenantID {
                return connectorAPI.CapabilityStateResult{}, connectorDomain.ErrNotFound
        }
        current, ok := f.connections[connectionID]
        if !ok {
                return connectorAPI.CapabilityStateResult{}, connectorDomain.ErrNotFound
        }
        def, _ := connectorCatalog.Get(current.ConnectorID, current.ConnectorVer)
        return f.capabilityStateLocked(current, def)
}

func (f *connectorFixture) capabilityStateLocked(current connectorDomain.Connection, def connectorDomain.ConnectorDefinition) (connectorAPI.CapabilityStateResult, error) {
        rows := []connectorDomain.CapabilityEvidence{}
        for _, row := range f.evidence {
                if row.ConnectionID == current.ConnectionID {
                        rows = append(rows, row)
                }
        }
        now := time.Now().UTC()
        reports := connectorDomain.DeriveCapabilityStates(def, current, rows, fixtureVerifier.Build(), now)
        return connectorAPI.CapabilityStateResult{Connection: current, ContractVersion: current.ConnectorVer, AdapterBuild: fixtureVerifier.Build(), Capabilities: reports}, nil
}

func (f *employeeFixture) Create(_ context.Context, _ employeeAPI.CreateCommand) (employeeAPI.CreateResult, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil {
                f.draft = &employeeDomain.Draft{TenantID: tenantID, EmployeeID: fixtureEmployeeID, OwnerID: principalID, Status: "draft", Readiness: "incomplete", Version: 1, CurrentStep: "brief", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
        }
        return employeeAPI.CreateResult{Draft: *f.draft}, nil
}
func (f *employeeFixture) List(_ context.Context, _ domain.Principal, _ id.UUID) ([]employeeDomain.Draft, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil {
                return []employeeDomain.Draft{}, nil
        }
        return []employeeDomain.Draft{*f.draft}, nil
}
func (f *employeeFixture) Read(_ context.Context, _ domain.Principal, _ id.UUID, employeeID id.UUID) (employeeDomain.Draft, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || employeeID != fixtureEmployeeID {
                return employeeDomain.Draft{}, employeeDomain.ErrNotFound
        }
        return *f.draft, nil
}
func (f *employeeFixture) Save(_ context.Context, c employeeAPI.SaveCommand) (employeeDomain.Draft, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || c.EmployeeID != fixtureEmployeeID {
                return employeeDomain.Draft{}, employeeDomain.ErrNotFound
        }
        if c.ExpectedVersion != f.draft.Version {
                return employeeDomain.Draft{}, employeeDomain.ErrConflict
        }
        if !c.Brief.Valid() {
                return employeeDomain.Draft{}, employeeDomain.ErrInvalid
        }
        f.draft.Version++
        f.draft.Brief = c.Brief
        f.draft.CurrentStep = "launch_plan"
        f.draft.UpdatedAt = time.Now().UTC()
        return *f.draft, nil
}
func (f *employeeFixture) Catalog(context.Context, domain.Principal, id.UUID) ([]employeeDomain.PackManifest, error) {
        return packs.List(), nil
}
func (f *employeeFixture) Preview(_ context.Context, c employeeAPI.PinCommand) (employeeDomain.PackPreview, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        m, ok := packs.Get(c.PackID, c.PackVersion)
        if !ok || f.draft == nil || f.draft.Pack == nil {
                return employeeDomain.PackPreview{}, employeeDomain.ErrInvalid
        }
        impact, allowed := packs.Impact(*f.draft.Pack, m)
        if !allowed {
                return employeeDomain.PackPreview{}, employeeDomain.ErrInvalid
        }
        return employeeDomain.PackPreview{Current: *f.draft.Pack, Target: m, Impact: impact}, nil
}
func (f *employeeFixture) Pin(_ context.Context, c employeeAPI.PinCommand) (employeeDomain.Draft, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil {
                return employeeDomain.Draft{}, employeeDomain.ErrNotFound
        }
        if c.ExpectedVersion != f.draft.Version {
                return employeeDomain.Draft{}, employeeDomain.ErrConflict
        }
        m, ok := packs.Get(c.PackID, c.PackVersion)
        if !ok || len(employeeDomain.ValidateVariants(m, c.Variants)) > 0 {
                return employeeDomain.Draft{}, employeeDomain.ErrInvalid
        }
        if f.draft.Pack != nil && (f.draft.Pack.ID != c.PackID || f.draft.Pack.Version != c.PackVersion) {
                impact, ok := packs.Impact(*f.draft.Pack, m)
                if !ok || !c.Confirmed {
                        return employeeDomain.Draft{}, employeeDomain.ErrInvalid
                }
                f.draft.ReviewSlots = impact.ReviewSlots
        }
        f.draft.Pack = &employeeDomain.PackPin{ID: c.PackID, Version: c.PackVersion, Variants: c.Variants}
        f.draft.Version++
        return *f.draft, nil
}

func (f *employeeFixture) CreateKnowledge(_ context.Context, c employeeAPI.KnowledgeCreateCommand) (employeeDomain.KnowledgeSource, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || f.draft.Pack == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID {
                return employeeDomain.KnowledgeSource{}, employeeDomain.ErrNotFound
        }
        if !c.Source.Valid() {
                return employeeDomain.KnowledgeSource{}, employeeDomain.ErrInvalid
        }
        s := c.Source
        s.TenantID = tenantID
        s.EmployeeID = fixtureEmployeeID
        s.SourceID = fixtureSourceID
        s.Status = "processing"
        s.Version = 1
        s.CreatedAt = time.Now().UTC()
        s.UpdatedAt = s.CreatedAt
        f.knowledge = &s
        f.versions = nil
        return s, nil
}
func (f *employeeFixture) IngestKnowledge(_ context.Context, c employeeAPI.KnowledgeIngestCommand) (employeeDomain.KnowledgeSource, employeeDomain.KnowledgeVersion, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.draft == nil || f.draft.Pack == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID || !c.Fixture || c.Source.Type != "document" || c.URL != "" || len(c.File) != 0 {
                return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, employeeDomain.ErrInvalid
        }
        s := c.Source
        s.TenantID, s.EmployeeID, s.SourceID = tenantID, fixtureEmployeeID, fixtureSourceID
        s.Origin, s.Status, s.Version = ingest.FixtureOrigin, "processing", 1
        s.CreatedAt = time.Now().UTC()
        s.UpdatedAt = s.CreatedAt
        f.knowledge = &s
        f.versions = nil
        return s, employeeDomain.KnowledgeVersion{}, nil
}
func (f *employeeFixture) ReingestSyntheticKnowledge(_ context.Context, c employeeAPI.KnowledgeFixtureCommand) (employeeDomain.KnowledgeSource, employeeDomain.KnowledgeVersion, bool, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.knowledge == nil || c.TenantID != tenantID || c.EmployeeID != fixtureEmployeeID || c.SourceID != fixtureSourceID || f.knowledge.Origin != ingest.FixtureOrigin || f.knowledge.Status == "retired" {
                return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrNotFound
        }
        text, err := ingest.SyntheticText(c.FixtureID)
        if err != nil {
                return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrInvalid
        }
        if len(f.versions) > 0 && f.versions[0].Text == text {
                return *f.knowledge, f.versions[0], true, nil
        }
        if c.ExpectedVersion != f.knowledge.Version {
                return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrConflict
        }
        now := time.Now().UTC()
        key := []byte("fixture-secret-for-knowledge-checksums")
        v := employeeDomain.KnowledgeVersion{SourceID: fixtureSourceID, Version: f.knowledge.CurrentVersion + 1, Checksum: employeeDomain.KnowledgeChecksum(key, text), Text: text, Origin: ingest.FixtureOrigin, CreatedAt: now, Chunks: employeeDomain.KnowledgeChunks(key, text)}
        for i := range v.Chunks {
                v.Chunks[i].SourceID, v.Chunks[i].Version = fixtureSourceID, v.Version
                v.Chunks[i].Origin, v.Chunks[i].CapturedAt = v.Origin, now
        }
        f.versions = append([]employeeDomain.KnowledgeVersion{v}, f.versions...)
        f.knowledge.CurrentVersion = v.Version
        f.knowledge.Version++
        f.knowledge.Status = "preview"
        return *f.knowledge, v, false, nil
}
func (f *employeeFixture) ListKnowledge(_ context.Context, _ domain.Principal, tenant, employee id.UUID) ([]employeeDomain.KnowledgeSource, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if tenant != tenantID || employee != fixtureEmployeeID {
                return nil, employeeDomain.ErrNotFound
        }
        if f.knowledge == nil {
                return []employeeDomain.KnowledgeSource{}, nil
        }
        return []employeeDomain.KnowledgeSource{*f.knowledge}, nil
}
func (f *employeeFixture) ReadKnowledge(_ context.Context, _ domain.Principal, tenant, employee, source id.UUID) (employeeDomain.KnowledgeSource, []employeeDomain.KnowledgeVersion, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if tenant != tenantID || employee != fixtureEmployeeID || source != fixtureSourceID || f.knowledge == nil {
                return employeeDomain.KnowledgeSource{}, nil, employeeDomain.ErrNotFound
        }
        return *f.knowledge, append([]employeeDomain.KnowledgeVersion(nil), f.versions...), nil
}
func (f *employeeFixture) ProcessKnowledge(_ context.Context, c employeeAPI.KnowledgeProcessCommand) (employeeDomain.KnowledgeSource, employeeDomain.KnowledgeVersion, bool, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.knowledge == nil || c.SourceID != fixtureSourceID {
                return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrNotFound
        }
        if !employeeDomain.ValidKnowledgeText(c.Text) {
                return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrInvalid
        }
        if f.knowledge.Status == "retired" {
                return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrInvalid
        }
        key := []byte("fixture-secret-for-knowledge-checksums")
        checksum := employeeDomain.KnowledgeChecksum(key, c.Text)
        if len(f.versions) > 0 && f.versions[0].Checksum == checksum {
                return *f.knowledge, f.versions[0], true, nil
        }
        if c.ExpectedVersion != f.knowledge.Version {
                return employeeDomain.KnowledgeSource{}, employeeDomain.KnowledgeVersion{}, false, employeeDomain.ErrConflict
        }
        v := employeeDomain.KnowledgeVersion{SourceID: fixtureSourceID, Version: f.knowledge.CurrentVersion + 1, Checksum: checksum, Text: c.Text, Chunks: employeeDomain.KnowledgeChunks(key, c.Text), CreatedAt: time.Now().UTC()}
        for i := range v.Chunks {
                v.Chunks[i].SourceID = fixtureSourceID
                v.Chunks[i].Version = v.Version
        }
        if len(f.versions) > 0 {
                v.RemovedFromPrevious, v.AddedFromPrevious = employeeDomain.KnowledgeDiff(f.versions[0].Text, v.Text)
        }
        f.versions = append([]employeeDomain.KnowledgeVersion{v}, f.versions...)
        f.knowledge.CurrentVersion = v.Version
        f.knowledge.Version++
        f.knowledge.Status = "ready"
        return *f.knowledge, v, false, nil
}
func (f *employeeFixture) ConflictKnowledge(_ context.Context, c employeeAPI.KnowledgeConflictCommand) (employeeDomain.KnowledgeSource, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.knowledge == nil || c.SourceID != fixtureSourceID {
                return employeeDomain.KnowledgeSource{}, employeeDomain.ErrNotFound
        }
        if c.ExpectedVersion != f.knowledge.Version {
                return employeeDomain.KnowledgeSource{}, employeeDomain.ErrConflict
        }
        f.knowledge.ConflictNote = c.Note
        f.knowledge.ConflictProvenance = c.Provenance
        f.knowledge.Version++
        return *f.knowledge, nil
}
func (f *employeeFixture) RetireKnowledge(_ context.Context, c employeeAPI.KnowledgeRetireCommand) (employeeDomain.KnowledgeSource, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        if f.knowledge == nil || c.SourceID != fixtureSourceID {
                return employeeDomain.KnowledgeSource{}, employeeDomain.ErrNotFound
        }
        if c.ExpectedVersion != f.knowledge.Version {
                return employeeDomain.KnowledgeSource{}, employeeDomain.ErrConflict
        }
        f.knowledge.Status = "retired"
        f.knowledge.Version++
        return *f.knowledge, nil
}

func main() {
        server, err := httptransport.New(identityFixture{}, resolverFixture{}, idsFixture{}, clockFixture{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
        if err != nil {
                panic(err)
        }
        employees := &employeeFixture{}
        connectors := &connectorFixture{}
        server.BindEmployee(employees)
        server.BindConnector(connectors)
        server.BindPrivacy(&privacyFixture{})
        server.BindEntitlement(&entitlementFixture{})
        server.BindRelease(newReleaseFixture(employees, connectors))
        if err := http.ListenAndServe("127.0.0.1:18081", server.Handler()); err != nil {
                panic(err)
        }
}
