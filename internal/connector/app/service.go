package app

import (
        "context"
        "crypto/hmac"
        "crypto/rand"
        "crypto/sha256"
        "encoding/hex"
        "strings"

        "github.com/artemiosu/staffy/internal/connector/api"
        "github.com/artemiosu/staffy/internal/connector/catalog"
        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        "github.com/artemiosu/staffy/internal/connector/port"
        identityAPI "github.com/artemiosu/staffy/internal/identity/api"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

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
        if len(key) < 8 || len(key) > 200 || !c.CorrelationID.Valid() {
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
func (s *Service) Connect(ctx context.Context, c api.ConnectCommand) (connectorDomain.Connection, error) {
        return s.connect(ctx, c, false)
}

// Rotate replaces the accepted secret: the previous seal is revoked and the
// check states are reset because the old evidence no longer applies.
func (s *Service) Rotate(ctx context.Context, c api.ConnectCommand) (connectorDomain.Connection, error) {
        return s.connect(ctx, c, true)
}

func (s *Service) connect(ctx context.Context, c api.ConnectCommand, rotate bool) (connectorDomain.Connection, error) {
        scope, err := s.scope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        if !c.ConnectionID.Valid() || !c.CorrelationID.Valid() || c.ExpectedVersion < 1 {
                return connectorDomain.Connection{}, connectorDomain.ErrInvalid
        }
        current, err := s.repository.Read(ctx, scope, c.ConnectionID)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        if current.Status == connectorDomain.StatusRevoked {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "status", Reason: "Отозванное подключение нельзя проверить; создайте новое."}
        }
        if !rotate && current.SecretActive {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "secret", Reason: "Секрет уже принят. Для замены используйте ротацию."}
        }
        def, ok := catalog.Get(current.ConnectorID, current.ConnectorVer)
        if !ok {
                return connectorDomain.Connection{}, connectorDomain.ErrInvalid
        }
        if err := def.ValidateSecrets(c.Secrets); err != nil {
                return connectorDomain.Connection{}, err
        }
        // Basic check runs on the submitted values in memory; the plaintext secret
        // is never persisted, logged, or echoed back.
        check := s.verifier.BasicCheck(ctx, def, current.Environment, current.Config, c.Secrets)
        now := s.clock.Now().UTC()
        // A secret is sealed only when authentication actually verified: invalid
        // credentials are not stored, so the user can retry connect honestly.
        seal := check.Authentication == connectorDomain.CheckVerified
        secretID, err := s.ids.New(now)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        eventID, err := s.ids.New(now)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        auditID, err := s.ids.New(now)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        var sealed, key []byte
        hint := ""
        if seal {
                key = make([]byte, 32)
                if _, err = rand.Read(key); err != nil {
                        return connectorDomain.Connection{}, err
                }
                sealed, hint, err = sealSecret(key, c.Secrets)
                if err != nil {
                        return connectorDomain.Connection{}, err
                }
        }
        return s.repository.Connect(ctx, port.ConnectParams{Scope: scope, ConnectionID: c.ConnectionID, SecretID: secretID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID, ExpectedVersion: c.ExpectedVersion, Seal: seal, Sealed: sealed, Key: key, Hint: hint, Check: check, Now: now, Rotate: rotate})
}

func (s *Service) Revoke(ctx context.Context, c api.RevokeCommand) (connectorDomain.Connection, error) {
        scope, err := s.scope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        if !c.ConnectionID.Valid() || !c.CorrelationID.Valid() || c.ExpectedVersion < 1 {
                return connectorDomain.Connection{}, connectorDomain.ErrInvalid
        }
        reason := strings.TrimSpace(c.Reason)
        if reason == "" || len([]rune(reason)) > 300 {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "reason", Reason: "Укажите причину отзыва (до 300 символов)."}
        }
        current, err := s.repository.Read(ctx, scope, c.ConnectionID)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        if current.Status == connectorDomain.StatusRevoked {
                return connectorDomain.Connection{}, connectorDomain.RuleError{Rule: "status", Reason: "Подключение уже отозвано."}
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
        return s.repository.Revoke(ctx, port.RevokeParams{Scope: scope, ConnectionID: c.ConnectionID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID, ExpectedVersion: c.ExpectedVersion, Reason: reason, Now: now})
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
