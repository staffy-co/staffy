package domain

import (
        "time"

        "github.com/staffy-co/staffy/internal/platform/id"
)

// Connection states. The overall status never claims capability proof:
// `connected_unverified` is the terminal honest state of this slice.
const (
        StatusDraft               = "draft"
        StatusConnectedUnverified = "connected_unverified"
        StatusFailed              = "failed"
        StatusRevoked             = "revoked"
)

// MaxRevokeReasonRunes bounds the human reason so the stored version summary
// ("revoked: " + reason) always satisfies the 300-character history limit.
const MaxRevokeReasonRunes = 290

// Separate check states for basic authentication and transport availability.
const (
        CheckPending = "pending"
        CheckVerified = "verified"
        CheckFailed  = "failed"
)

// Connection is the tenant-scoped current state of one provider account.
type Connection struct {
        TenantID     id.UUID     `json:"tenant_id"`
        ConnectionID id.UUID     `json:"connection_id"`
        ConnectorID  string      `json:"connector_id"`
        ConnectorVer string      `json:"connector_version"`
        Label        string      `json:"label"`
        Environment  Environment `json:"environment"`
        Status       string      `json:"status"`
        AuthState    string      `json:"authentication_state"`
        Transport    string      `json:"transport_state"`
        Config       map[string]string `json:"config"`
        SecretID     id.UUID     `json:"secret_id,omitempty"`
        SecretHint   string      `json:"secret_hint,omitempty"`
        SecretActive bool        `json:"secret_active"`
        OwnerID      id.UUID     `json:"owner_id"`
        Version      int64       `json:"version"`
        CreatedAt    time.Time   `json:"created_at"`
        UpdatedAt    time.Time   `json:"updated_at"`
}

// ConnectionVersion is one immutable history entry of a Connection.
type ConnectionVersion struct {
        Connection
        ActorID       id.UUID `json:"actor_id"`
        ChangeSummary string  `json:"change_summary"`
}

// CheckResult is the redacted outcome of the synthetic basic check.
// It never carries the secret or the raw provider response.
type CheckResult struct {
        Authentication string `json:"authentication"`
        Transport      string `json:"transport"`
        AuthReason     string `json:"authentication_reason,omitempty"`
        TransportReason string `json:"transport_reason,omitempty"`
}

// Seal is the stored secret envelope: HMAC of the accepted value plus the
// per-connection random key. Neither is ever returned through the API.
type Seal struct {
        TenantID     id.UUID
        ConnectionID id.UUID
        SecretID     id.UUID
        Sealed       []byte
        Key          []byte
        CreatedAt    time.Time
        RevokedAt    *time.Time
}

// ValidTransition guards state machine moves for connection-level status.
// Config changes reset the status to draft because the prior check no longer
// describes the stored values; revoked is terminal.
func ValidTransition(from, to string) bool {
        switch from {
        case StatusDraft:
                return to == StatusConnectedUnverified || to == StatusFailed || to == StatusRevoked
        case StatusConnectedUnverified:
                return to == StatusDraft || to == StatusConnectedUnverified || to == StatusFailed || to == StatusRevoked
        case StatusFailed:
                return to == StatusDraft || to == StatusConnectedUnverified || to == StatusFailed || to == StatusRevoked
        case StatusRevoked:
                return false
        }
        return false
}

// ValidateEnvironment enforces the AG-4 fail-closed ban with a precise reason.
func ValidateEnvironment(env Environment) error {
        switch env {
        case EnvSandbox, EnvTest:
                return nil
        case EnvProduct:
                return ProductionBlockedError{RuleError{"environment", "Production-credentials запрещены до прохождения gate AG-4 (Dependency/provider readiness). Ответственный: Technical Lead + Security. Используйте sandbox или test подключение."}}
        default:
                return RuleError{"environment", "Среда должна быть sandbox или test."}
        }
}
