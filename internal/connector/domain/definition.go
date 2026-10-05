package domain

import (
        "errors"
        "net/url"
        "strings"
)

var (
        ErrNotFound            = errors.New("connection not found")
        ErrForbidden           = errors.New("principal is not allowed to manage connections")
        ErrInvalid             = errors.New("connection request is invalid")
        ErrConflict            = errors.New("connection version conflict")
        ErrProductionBlocked   = errors.New("production credentials are blocked until AG-4 passes")
        ErrInvalidIdempotency  = errors.New("idempotency key is invalid")
        ErrIdempotencyConflict = errors.New("idempotency key conflicts with another request")
)

// RuleError keeps a precise, user-safe reason for a rejected connector command.
type RuleError struct{ Rule, Reason string }

func (e RuleError) Error() string { return e.Rule + ": " + e.Reason }
func (e RuleError) Unwrap() error { return ErrInvalid }

// ProductionBlockedError is the fail-closed AG-4 rejection: it carries RuleError
// semantics and unwraps to ErrProductionBlocked for precise matching.
type ProductionBlockedError struct{ RuleError }

func (e ProductionBlockedError) Unwrap() error { return ErrProductionBlocked }

// ConnectorType groups catalog cards. The five values are fixed by the PRD.
type ConnectorType string

const (
        TypeSIP      ConnectorType = "SIP/telephony"
        TypeChat     ConnectorType = "chat/inbox"
        TypeBusiness ConnectorType = "business system"
        TypeAISpeech ConnectorType = "AI/speech"
        TypeStorage  ConnectorType = "storage/supporting service"
)

// SupportStatus is the lifecycle state of a connector definition.
type SupportStatus string

const (
        StatusSupported   SupportStatus = "supported"
        StatusPilot       SupportStatus = "pilot"
        StatusCandidate   SupportStatus = "candidate"
        StatusDeferred    SupportStatus = "deferred"
        StatusUnavailable SupportStatus = "unavailable"
)

// Environment separates account scopes. Production stays fail-closed until AG-4.
type Environment string

const (
        EnvSandbox Environment = "sandbox"
        EnvTest    Environment = "test"
        EnvProduct Environment = "production"
)

// FieldType enumerates declarative config field kinds.
type FieldType string

const (
        FieldText FieldType = "text"
        FieldURL  FieldType = "url"
)

// FieldSchema is one declared field of a versioned connector schema.
type FieldSchema struct {
        Name        string    `json:"name"`
        Label       string    `json:"label"`
        Type        FieldType `json:"type"`
        Required    bool      `json:"required"`
        Secret      bool      `json:"secret"`
        Description string    `json:"description"`
}

// ConnectorDefinition is an immutable, versioned catalog entry.
// It declares intent and schema only: no provider DTOs, no executable policy.
type ConnectorDefinition struct {
        ID                  string        `json:"id"`
        Version             string        `json:"version"`
        Type                ConnectorType `json:"type"`
        Name                string        `json:"name"`
        Purpose             string        `json:"purpose"`
        SupportStatus       SupportStatus `json:"support_status"`
        AdapterImplemented  bool          `json:"adapter_implemented"`
        AllowedEnvironments []Environment `json:"allowed_environments"`
        RequiredAccountType string        `json:"required_account_type"`
        DocumentationURL    string        `json:"documentation_url"`
        PriorityNote        string        `json:"priority_note,omitempty"`
        Fields              []FieldSchema `json:"fields"`
}

// Connectable reports whether this definition may start a new Connection.
// A card is connectable only when its adapter is actually implemented.
func (d ConnectorDefinition) Connectable() bool {
        return d.SupportStatus == StatusSupported && d.AdapterImplemented
}

// EnvironmentAllowed intersects the requested environment with the definition
// policy and the AG-4 production ban.
func (d ConnectorDefinition) EnvironmentAllowed(env Environment) bool {
        if env == EnvProduct {
                return false
        }
        for _, allowed := range d.AllowedEnvironments {
                if allowed == env {
                        return true
                }
        }
        return false
}

// ConfigSchema returns non-secret fields; secret fields go through the SecretStore.
func (d ConnectorDefinition) ConfigSchema() []FieldSchema {
        out := make([]FieldSchema, 0, len(d.Fields))
        for _, f := range d.Fields {
                if !f.Secret {
                        out = append(out, f)
                }
        }
        return out
}

// SecretSchema returns the secret fields declared by this definition.
func (d ConnectorDefinition) SecretSchema() []FieldSchema {
        out := make([]FieldSchema, 0, len(d.Fields))
        for _, f := range d.Fields {
                if f.Secret {
                        out = append(out, f)
                }
        }
        return out
}

// ValidateConfig checks non-secret values against the declared schema.
// Unknown or forbidden parameters are rejected fail-closed.
func (d ConnectorDefinition) ValidateConfig(config map[string]string) error {
        schema := map[string]FieldSchema{}
        for _, f := range d.Fields {
                if f.Secret {
                        continue
                }
                schema[f.Name] = f
        }
        for name := range config {
                if _, ok := schema[name]; !ok {
                        return RuleError{"config", "Поле «" + name + "» не объявлено схемой этого Connector."}
                }
        }
        for _, f := range d.ConfigSchema() {
                value, ok := config[f.Name]
                if !ok || strings.TrimSpace(value) == "" {
                        if f.Required {
                                return RuleError{"config", "Обязательное поле «" + f.Label + "» не заполнено."}
                        }
                        continue
                }
                if len([]rune(value)) > 300 {
                        return RuleError{"config", "Поле «" + f.Label + "» слишком длинное."}
                }
                if strings.ContainsAny(value, "<>\x00") {
                        return RuleError{"config", "Поле «" + f.Label + "» содержит недопустимые символы."}
                }
                if f.Type == FieldURL && !validURL(value) {
                        return RuleError{"config", "Поле «" + f.Label + "» должно быть корректным URL."}
                }
        }
        return nil
}

// ValidateSecrets checks submitted secret values: presence and basic hygiene.
// Secret values themselves are never logged or stored in plaintext.
func (d ConnectorDefinition) ValidateSecrets(secrets map[string]string) error {
        declared := map[string]FieldSchema{}
        for _, f := range d.SecretSchema() {
                declared[f.Name] = f
        }
        for name := range secrets {
                if _, ok := declared[name]; !ok {
                        return RuleError{"secret", "Секретное поле «" + name + "» не объявлено схемой этого Connector."}
                }
        }
        for _, f := range d.SecretSchema() {
                value, ok := secrets[f.Name]
                if !ok || value == "" {
                        if f.Required {
                                return RuleError{"secret", "Секретное поле «" + f.Label + "» обязательно."}
                        }
                        continue
                }
                if len(value) < 8 || len(value) > 256 {
                        return RuleError{"secret", "Секретное поле «" + f.Label + "» должно быть от 8 до 256 символов."}
                }
                if strings.ContainsAny(value, "\x00\r\n") {
                        return RuleError{"secret", "Секретное поле «" + f.Label + "» содержит недопустимые символы."}
                }
        }
        return nil
}

func validURL(raw string) bool {
        u, err := url.Parse(strings.TrimSpace(raw))
        if err != nil {
                return false
        }
        return (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}
