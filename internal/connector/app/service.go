package app

import (
        "context"
        "crypto/hmac"
        "crypto/rand"
        "crypto/sha256"
        "encoding/hex"
        "strings"
        "time"

        "github.com/artemiosu/staffy/internal/connector/api"
        "github.com/artemiosu/staffy/internal/connector/catalog"
        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        "github.com/artemiosu/staffy/internal/connector/port"
        identityAPI "github.com/artemiosu/staffy/internal/identity/api"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

// capabilityEvidenceTTL bounds how long one probe result proves anything.
const capabilityEvidenceTTL = 24 * time.Hour

// Service implements the connector application logic. Authorization uses the
// identity Integrations scope; every command resolves a tenant scope before
// touching the repository.
type Service struct {
        repository port.Repository
        identity   identityAPI.MembershipService
        verifier   api.Verifier
        clock      api.Clock
        ids        api.IDGenerator
}

func New(repository port.Repository, identity identityAPI.MembershipService, verifier api.Verifier, clock api.Clock, ids api.IDGenerator) *Service {
        return &Service{repository: repository, identity: identity, verifier: verifier, clock: clock, ids: ids}
}

func (s *Service) scope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
        if !actor.ID.Valid() || !tenantID.Valid() {
                return tenancy.Context{}, connectorDomain.ErrNotFound
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Integrations); err != nil {
                return tenancy.Context{}, connectorDomain.ErrNotFound
        }
        return tenancy.New(actor.ID, tenantID)
}

func (s *Service) Catalog(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]connectorDomain.ConnectorDefinition, error) {
        if _, err := s.scope(ctx, actor, tenantID); err != nil {
                return nil, err
        }
        defs := catalog.List()
        return defs, nil
}

func (s *Service) Create(ctx context.Context, c api.CreateCommand) (api.CreateResult, error) {
        scope, err := s.scope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return api.CreateResult{}, err
        }
        key := strings.TrimSpace(c.IdempotencyKey)
        // Count runes, not bytes: the storage CHECK uses char_length, so a
        // multibyte key measured in bytes could otherwise pass here and fail
        // in SQL with a 500.
        if runes := len([]rune(key)); runes < 8 || runes > 200 || !c.CorrelationID.Valid() {
                return api.CreateResult{}, connectorDomain.ErrInvalidIdempotency
        }
        label := strings.TrimSpace(c.Label)
        if label == "" || len([]rune(label)) > 120 {
                return api.CreateResult{}, connectorDomain.RuleError{Rule: "label", Reason: "Назначение подключения обязательно (до 120 символов)."}
        }
        if err := connectorDomain.ValidateEnvironment(c.Environment); err != nil {
                return api.CreateResult{}, err
        }
        def, ok := catalog.Get(c.ConnectorID, c.ConnectorVer)
        if !ok {
                return api.CreateResult{}, connectorDomain.RuleError{Rule: "connector", Reason: "Connector definition не найден в каталоге."}
        }
        // New connections pin the current contract version: the catalog card
        // only offers the latest definition, and an older pin is refused honestly.
        if latest, isLatest, exists := catalog.Latest(c.ConnectorID, c.ConnectorVer); exists && !isLatest {
                return api.CreateResult{}, connectorDomain.RuleError{Rule: "connector", Reason: "Версия контракта устарела: доступна " + latest.Version + ". Создайте подключение с актуальной версией каталога."}
        }
        if !def.Connectable() {
                return api.CreateResult{}, connectorDomain.RuleError{Rule: "connector", Reason: "Адаптер этого Connector не реализован; подключение недоступно. Следите за обновлениями каталога."}
        }
        if !def.EnvironmentAllowed(c.Environment) {
                return api.CreateResult{}, connectorDomain.RuleError{Rule: "environment", Reason: "Среда не допускается этим Connector."}
        }
        if err := def.ValidateConfig(c.Config); err != nil {
                return api.CreateResult{}, err
        }
        now := s.clock.Now().UTC()
        connectionID, err := s.ids.New(now)
        if err != nil {
                return api.CreateResult{}, err
        }
        secretID, err := s.ids.New(now)
        if err != nil {
                return api.CreateResult{}, err
        }
        eventID, err := s.ids.New(now)
        if err != nil {
                return api.CreateResult{}, err
        }
        auditID, err := s.ids.New(now)
        if err != nil {
                return api.CreateResult{}, err
        }
        hash := requestHash("connector.create.v1", c)
        connection, replayed, err := s.repository.Create(ctx, port.CreateParams{Scope: scope, ConnectionID: connectionID, SecretID: secretID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID, Key: key, Hash: hash, ConnectorID: def.ID, ConnectorVer: def.Version, Label: label, Environment: c.Environment, Config: cleanConfig(c.Config), Now: now})
        return api.CreateResult{Connection: connection, Replayed: replayed}, err
}

func (s *Service) List(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]connectorDomain.Connection, error) {
        scope, err := s.scope(ctx, actor, tenantID)
        if err != nil {
                return nil, err
        }
        return s.repository.List(ctx, scope)
}

func (s *Service) Read(ctx context.Context, actor identityDomain.Principal, tenantID, connectionID id.UUID) (connectorDomain.Connection, error) {
        scope, err := s.scope(ctx, actor, tenantID)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        return s.repository.Read(ctx, scope, connectionID)
}

func (s *Service) ReadVersions(ctx context.Context, actor identityDomain.Principal, tenantID, connectionID id.UUID) ([]connectorDomain.ConnectionVersion, error) {
        scope, err := s.scope(ctx, actor, tenantID)
        if err != nil {
                return nil, err
        }
        return s.repository.ReadVersions(ctx, scope, connectionID)
}

func (s *Service) Save(ctx context.Context, c api.SaveCommand) (connectorDomain.Connection, error) {
        scope, err := s.scope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        if !c.ConnectionID.Valid() || !c.CorrelationID.Valid() || c.ExpectedVersion < 1 {
                return connectorDomain.Connection{}, connectorDomain.ErrInvalid
        }
        label := strings.TrimSpace(c.Label)
        if label == "" || len([]rune(label)) > 120 {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "label", Reason: "Назначение подключения обязательно (до 120 символов)."}
        }
        summary := strings.TrimSpace(c.ChangeSummary)
        if summary == "" || len([]rune(summary)) > 300 {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "summary", Reason: "Опишите изменение (до 300 символов)."}
        }
        current, err := s.repository.Read(ctx, scope, c.ConnectionID)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        if current.Status == connectorDomain.StatusRevoked {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "status", Reason: "Отозванное подключение нельзя изменить; создайте новое."}
        }
        if !connectorDomain.ValidTransition(current.Status, connectorDomain.StatusDraft) {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "status", Reason: "Состояние подключения не допускает сохранение; обновите страницу."}
        }
        def, ok := catalog.Get(current.ConnectorID, current.ConnectorVer)
        if !ok {
                return connectorDomain.Connection{}, connectorDomain.ErrInvalid
        }
        if err := def.ValidateConfig(c.Config); err != nil {
                return connectorDomain.Connection{}, err
        }
        now := s.clock.Now().UTC()
        eventID, err := s.ids.New(now)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        auditID, err := s.ids.New(now)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        return s.repository.Save(ctx, port.SaveParams{Scope: scope, ConnectionID: c.ConnectionID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID, ExpectedVersion: c.ExpectedVersion, Config: cleanConfig(c.Config), Label: label, Summary: summary, Now: now})
}

// Connect submits secrets once, runs the allowed synthetic basic check and
// stores the sealed secret. The result never claims provider capabilities.
// The command is idempotent: a network-failure retry with the same key and
// the same data replays the stored outcome instead of a false conflict.
func (s *Service) Connect(ctx context.Context, c api.ConnectCommand) (api.ConnectResult, error) {
        return s.connect(ctx, c, false)
}

// Rotate replaces the accepted secret: the previous seal is revoked and the
// check states are reset because the old evidence no longer applies.
func (s *Service) Rotate(ctx context.Context, c api.ConnectCommand) (api.ConnectResult, error) {
        return s.connect(ctx, c, true)
}

func (s *Service) connect(ctx context.Context, c api.ConnectCommand, rotate bool) (api.ConnectResult, error) {
        scope, err := s.scope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return api.ConnectResult{}, err
        }
        key := strings.TrimSpace(c.IdempotencyKey)
        if runes := len([]rune(key)); runes < 8 || runes > 200 || !c.ConnectionID.Valid() || !c.CorrelationID.Valid() || c.ExpectedVersion < 1 {
                return api.ConnectResult{}, connectorDomain.ErrInvalidIdempotency
        }
        // Idempotency is answered BEFORE any domain guard: a network-failure retry
        // with the same key and data replays the stored outcome instead of hitting
        // "secret already accepted" or a false version conflict (R2 Gap 5).
        operation := "connector.connect.v1"
        if rotate {
                operation = "connector.rotate.v1"
        }
        hash := commandHash(operation, c.TenantID, c.ConnectionID, c.ExpectedVersion, secretsDigest(c.Secrets))
        if snapshot, replayed, err := s.repository.LookupCommand(ctx, scope, operation, key, hash); replayed || err != nil {
                return api.ConnectResult{Connection: snapshot, Replayed: replayed}, err
        }
        current, err := s.repository.Read(ctx, scope, c.ConnectionID)
        if err != nil {
                return api.ConnectResult{}, err
        }
        if current.Status == connectorDomain.StatusRevoked {
                return api.ConnectResult{}, connectorDomain.RuleError{Rule: "status", Reason: "Отозванное подключение нельзя проверить; создайте новое."}
        }
        def, ok := catalog.Get(current.ConnectorID, current.ConnectorVer)
        if !ok {
                return api.ConnectResult{}, connectorDomain.ErrInvalid
        }
        if err := def.ValidateSecrets(c.Secrets); err != nil {
                return api.ConnectResult{}, err
        }
        // Basic check runs on the submitted values in memory; the plaintext secret
        // is never persisted, logged, or echoed back.
        check := s.verifier.BasicCheck(ctx, def, current.Environment, current.Config, c.Secrets)
        nextStatus := connectorDomain.StatusFailed
        if check.Authentication == connectorDomain.CheckVerified && check.Transport == connectorDomain.CheckVerified {
                nextStatus = connectorDomain.StatusConnectedUnverified
        }
        if !connectorDomain.ValidTransition(current.Status, nextStatus) {
                return api.ConnectResult{}, connectorDomain.RuleError{Rule: "status", Reason: "Состояние подключения не допускает проверку; обновите страницу."}
        }
        now := s.clock.Now().UTC()
        // A secret is sealed only when authentication actually verified: invalid
        // credentials are not stored, so the user can retry connect honestly.
        seal := check.Authentication == connectorDomain.CheckVerified
        secretID, err := s.ids.New(now)
        if err != nil {
                return api.ConnectResult{}, err
        }
        eventID, err := s.ids.New(now)
        if err != nil {
                return api.ConnectResult{}, err
        }
        auditID, err := s.ids.New(now)
        if err != nil {
                return api.ConnectResult{}, err
        }
        var sealed, keyBytes []byte
        hint := ""
        if seal {
                keyBytes = make([]byte, 32)
                if _, err = rand.Read(keyBytes); err != nil {
                        return api.ConnectResult{}, err
                }
                sealed, hint, err = sealSecret(keyBytes, c.Secrets)
                if err != nil {
                        return api.ConnectResult{}, err
                }
        }
        connection, replayed, err := s.repository.Connect(ctx, port.ConnectParams{Scope: scope, ConnectionID: c.ConnectionID, SecretID: secretID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID, ExpectedVersion: c.ExpectedVersion, Seal: seal, Sealed: sealed, Key: keyBytes, Hint: hint, Check: check, Now: now, Rotate: rotate, Operation: operation, IdempotencyKey: key, RequestHash: hash})
        return api.ConnectResult{Connection: connection, Replayed: replayed}, err
}

func (s *Service) Revoke(ctx context.Context, c api.RevokeCommand) (api.RevokeResult, error) {
        scope, err := s.scope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return api.RevokeResult{}, err
        }
        key := strings.TrimSpace(c.IdempotencyKey)
        if runes := len([]rune(key)); runes < 8 || runes > 200 || !c.ConnectionID.Valid() || !c.CorrelationID.Valid() || c.ExpectedVersion < 1 {
                return api.RevokeResult{}, connectorDomain.ErrInvalidIdempotency
        }
        reason := strings.TrimSpace(c.Reason)
        // The stored summary is "revoked: " + reason; the rune bound keeps the
        // combined value inside the 300-character history CHECK.
        if reason == "" || len([]rune(reason)) > connectorDomain.MaxRevokeReasonRunes {
                return api.RevokeResult{}, connectorDomain.RuleError{Rule: "reason", Reason: "Укажите причину отзыва (до 290 символов)."}
        }
        // Idempotency first: a retry of a revoke with the same key replays the
        // stored outcome instead of "already revoked" (R2 Gap 5).
        hash := commandHash("connector.revoke.v1", c.TenantID, c.ConnectionID, c.ExpectedVersion, map[string]string{"reason": reason})
        if snapshot, replayed, err := s.repository.LookupCommand(ctx, scope, "connector.revoke.v1", key, hash); replayed || err != nil {
                return api.RevokeResult{Connection: snapshot, Replayed: replayed}, err
        }
        current, err := s.repository.Read(ctx, scope, c.ConnectionID)
        if err != nil {
                return api.RevokeResult{}, err
        }
        if current.Status == connectorDomain.StatusRevoked {
                return api.RevokeResult{}, connectorDomain.RuleError{Rule: "status", Reason: "Подключение уже отозвано."}
        }
        if !connectorDomain.ValidTransition(current.Status, connectorDomain.StatusRevoked) {
                return api.RevokeResult{}, connectorDomain.RuleError{Rule: "status", Reason: "Состояние подключения не допускает отзыв; обновите страницу."}
        }
        now := s.clock.Now().UTC()
        eventID, err := s.ids.New(now)
        if err != nil {
                return api.RevokeResult{}, err
        }
        auditID, err := s.ids.New(now)
        if err != nil {
                return api.RevokeResult{}, err
        }
        connection, replayed, err := s.repository.Revoke(ctx, port.RevokeParams{Scope: scope, ConnectionID: c.ConnectionID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID, ExpectedVersion: c.ExpectedVersion, Reason: reason, Now: now, Operation: "connector.revoke.v1", IdempotencyKey: key, RequestHash: hash})
        return api.RevokeResult{Connection: connection, Replayed: replayed}, err
}

// VerifyCapability runs the safe read-only probe for one declared capability
// of one connection and appends immutable evidence. Guarded fail-closed:
// only a completed, outbound, probe-allowed query capability of a
// connected/unverified connection with an active secret may be verified.
// Mutating capabilities are refused with the managed Action Gateway block;
// inbound events are refused honestly as "path not implemented in this slice".
func (s *Service) VerifyCapability(ctx context.Context, c api.VerifyCommand) (api.VerifyResult, error) {
        scope, err := s.scope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return api.VerifyResult{}, err
        }
        key := strings.TrimSpace(c.IdempotencyKey)
        if runes := len([]rune(key)); runes < 8 || runes > 200 || !c.ConnectionID.Valid() || !c.CorrelationID.Valid() {
                return api.VerifyResult{}, connectorDomain.ErrInvalidIdempotency
        }
        // Idempotency is answered before any guard: the same key with the same
        // data replays the stored evidence; the same key with different data is an
        // honest conflict no matter which capability the second request names.
        hash := commandHash("connector.capability.verify.v1", c.TenantID, c.ConnectionID, 0, map[string]string{"capability": strings.TrimSpace(c.Capability)})
        if existing, found, err := s.repository.LookupEvidence(ctx, scope, c.ConnectionID, key, hash); err != nil || found {
                if err != nil {
                        return api.VerifyResult{}, err
                }
                state, err := s.CapabilityState(ctx, c.Actor, c.TenantID, c.ConnectionID)
                if err != nil {
                        return api.VerifyResult{}, err
                }
                for _, report := range state.Capabilities {
                        if report.ID == existing.Capability {
                                return api.VerifyResult{Report: report, Replayed: true}, nil
                        }
                }
                return api.VerifyResult{}, connectorDomain.ErrInvalid
        }
        connection, err := s.repository.Read(ctx, scope, c.ConnectionID)
        if err != nil {
                return api.VerifyResult{}, err
        }
        if connection.Status != connectorDomain.StatusConnectedUnverified {
                return api.VerifyResult{}, connectorDomain.RuleError{Rule: "status", Reason: verifyGuardReason(connection.Status)}
        }
        def, ok := catalog.Get(connection.ConnectorID, connection.ConnectorVer)
        if !ok {
                return api.VerifyResult{}, connectorDomain.ErrInvalid
        }
        var dec connectorDomain.CapabilityDeclaration
        found := false
        for _, candidate := range def.Capabilities {
                if candidate.ID == c.Capability {
                        dec, found = candidate, true
                        break
                }
        }
        if !found {
                return api.VerifyResult{}, connectorDomain.RuleError{Rule: "capability", Reason: "Capability не заявлена контрактом версии " + connection.ConnectorVer + "."}
        }
        if !dec.Complete() {
                return api.VerifyResult{}, connectorDomain.RuleError{Rule: "capability", Reason: "Контракт не декларирует обязательную операционную семантику этой capability: она отображается как unsupported и не проверяется."}
        }
        if dec.Mutating() {
                return api.VerifyResult{}, connectorDomain.RuleError{Rule: "capability", Reason: "Мутирующая capability: проверка заблокирована до Action Gateway (Epic 3), гейт AG-4, ответственный Technical Lead + Security. Read-only возможности этого подключения доступны для проверки независимо."}
        }
        if dec.Operation == connectorDomain.OperationEvent {
                return api.VerifyResult{}, connectorDomain.RuleError{Rule: "capability", Reason: "Путь проверки входящих событий не реализован в текущем срезе: capability остаётся unverified без ложного успеха."}
        }
        if !dec.ProbeVerifiable() {
                return api.VerifyResult{}, connectorDomain.RuleError{Rule: "capability", Reason: "Эта capability не разрешена для read-only probe: проверка отклонена (fail-closed)."}
        }
        // The probe runs under the declared deadline; a timeout produces the
        // honest unknown outcome and is never retried blind.
        now := s.clock.Now().UTC()
        deadline := now.Add(time.Duration(dec.TimeoutMS) * time.Millisecond)
        probeCtx, cancel := context.WithTimeout(ctx, time.Duration(dec.TimeoutMS)*time.Millisecond)
        probe := s.verifier.Probe(probeCtx, def, dec, connection.Environment, connection.Config)
        cancel()
        completed := s.clock.Now().UTC()
        if completed.Before(now) {
                completed = now
        }
        evidenceID, err := s.ids.New(now)
        if err != nil {
                return api.VerifyResult{}, err
        }
        eventID, err := s.ids.New(now)
        if err != nil {
                return api.VerifyResult{}, err
        }
        auditID, err := s.ids.New(now)
        if err != nil {
                return api.VerifyResult{}, err
        }
        _, replayed, err := s.repository.SaveEvidence(ctx, port.SaveEvidenceParams{
                Scope: scope, EvidenceID: evidenceID, ConnectionID: c.ConnectionID, SecretID: connection.SecretID,
                Environment: connection.Environment, ContractVersion: connection.ConnectorVer, AdapterBuild: s.verifier.Build(),
                Capability: dec.ID, ProbeKind: connectorDomain.EvidenceProbeKind,
                RequestedAt: now, DeadlineAt: deadline, CompletedAt: completed,
                Outcome: probe.Outcome, DeliveryLevel: probe.DeliveryLevel,
                RedactedRequest: probe.RedactedRequest, RedactedResult: probe.RedactedResult,
                ExpiresAt: completed.Add(capabilityEvidenceTTL),
                IdempotencyKey: key, RequestHash: hash,
                EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID,
        })
        if err != nil {
                return api.VerifyResult{}, err
        }
        // The reported state is derived from facts (never from the response of
        // this command alone), so the caller sees exactly what any reader sees.
        state, err := s.CapabilityState(ctx, c.Actor, c.TenantID, c.ConnectionID)
        if err != nil {
                return api.VerifyResult{}, err
        }
        for _, report := range state.Capabilities {
                if report.ID == dec.ID {
                        return api.VerifyResult{Report: report, Replayed: replayed}, nil
                }
        }
        return api.VerifyResult{}, connectorDomain.ErrInvalid
}

// CapabilityState derives the honest per-capability view of one connection
// from the pinned contract, the immutable evidence and the connection facts.
// There is no stored state: the derivation is a pure function of facts.
func (s *Service) CapabilityState(ctx context.Context, actor identityDomain.Principal, tenantID, connectionID id.UUID) (api.CapabilityStateResult, error) {
        scope, err := s.scope(ctx, actor, tenantID)
        if err != nil {
                return api.CapabilityStateResult{}, err
        }
        connection, err := s.repository.Read(ctx, scope, connectionID)
        if err != nil {
                return api.CapabilityStateResult{}, err
        }
        def, ok := catalog.Get(connection.ConnectorID, connection.ConnectorVer)
        if !ok {
                return api.CapabilityStateResult{}, connectorDomain.ErrInvalid
        }
        evidence, err := s.repository.ListEvidence(ctx, scope, connectionID)
        if err != nil {
                return api.CapabilityStateResult{}, err
        }
        now := s.clock.Now().UTC()
        reports := connectorDomain.DeriveCapabilityStates(def, connection, evidence, s.verifier.Build(), now)
        return api.CapabilityStateResult{Connection: connection, ContractVersion: connection.ConnectorVer, AdapterBuild: s.verifier.Build(), Capabilities: reports}, nil
}

func verifyGuardReason(status string) string {
        switch status {
        case connectorDomain.StatusDraft:
                return "Подключение в черновике: сначала выполните базовую проверку (подключение секрета), затем проверяйте возможности."
        case connectorDomain.StatusFailed:
                return "Базовая проверка не пройдена: повторите подключение секрета, затем проверяйте возможности."
        case connectorDomain.StatusRevoked:
                return "Подключение отозвано: возможности неприменимы."
        }
        return "Состояние подключения не допускает проверку возможностей; обновите страницу."
}

// commandHash canonicalizes a command for idempotency replay/conflict
// detection: identical retries hash identically, any data change does not.
func commandHash(operation string, tenant, connection id.UUID, expectedVersion int64, payload map[string]string) string {
        h := sha256.New()
        _, _ = h.Write([]byte(operation))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(tenant.String()))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(connection.String()))
        _, _ = h.Write([]byte{0})
        var version [8]byte
        for i := 0; i < 8; i++ {
                version[i] = byte(expectedVersion >> (8 * (7 - i)))
        }
        _, _ = h.Write(version[:])
        for _, name := range sortedKeys(payload) {
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(name))
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(payload[name]))
        }
        return hex.EncodeToString(h.Sum(nil))
}

// secretsDigest renders the submitted secrets as a canonical digest input.
// The plaintext values never leave this function: only their order-stable
// concatenation feeds the hash.
func secretsDigest(secrets map[string]string) map[string]string {
        out := make(map[string]string, len(secrets))
        for name, value := range secrets {
                sum := sha256.Sum256([]byte(value))
                out[name] = hex.EncodeToString(sum[:])
        }
        return out
}

// sealSecret derives the HMAC envelope of the submitted secrets plus a short
// display hint. The hint lets a human confirm which secret is stored without
// ever revealing the value.
func sealSecret(key []byte, secrets map[string]string) (sealed []byte, hint string, err error) {
        var builder strings.Builder
        names := make([]string, 0, len(secrets))
        for name := range secrets {
                names = append(names, name)
        }
        for i := 0; i < len(names); i++ {
                for j := i + 1; j < len(names); j++ {
                        if names[j] < names[i] {
                                names[i], names[j] = names[j], names[i]
                        }
                }
        }
        for _, name := range names {
                builder.WriteString(name)
                builder.WriteByte(0)
                builder.WriteString(secrets[name])
                builder.WriteByte(0)
        }
        mac := hmac.New(sha256.New, key)
        _, err = mac.Write([]byte(builder.String()))
        if err != nil {
                return nil, "", err
        }
        sealed = mac.Sum(nil)
        return sealed, hex.EncodeToString(sealed[:6]), nil
}

func cleanConfig(config map[string]string) map[string]string {
        out := make(map[string]string, len(config))
        for k, v := range config {
                out[k] = strings.TrimSpace(v)
        }
        return out
}

// requestHash canonicalizes a create command so an idempotent replay with
// different data is detected as a conflict.
func requestHash(operation string, c api.CreateCommand) string {
        h := sha256.New()
        _, _ = h.Write([]byte(operation))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(c.TenantID.String()))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(c.ConnectorID))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(c.ConnectorVer))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(strings.TrimSpace(c.Label)))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(c.Environment))
        for _, name := range sortedKeys(c.Config) {
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(name))
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(strings.TrimSpace(c.Config[name])))
        }
        return hex.EncodeToString(h.Sum(nil))
}

func sortedKeys(m map[string]string) []string {
        keys := make([]string, 0, len(m))
        for k := range m {
                keys = append(keys, k)
        }
        for i := 0; i < len(keys); i++ {
                for j := i + 1; j < len(keys); j++ {
                        if keys[j] < keys[i] {
                                keys[i], keys[j] = keys[j], keys[i]
                        }
                }
        }
        return keys
}
