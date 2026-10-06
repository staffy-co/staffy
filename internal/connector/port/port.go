package port

import (
        "context"
        "time"

        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

type CreateParams struct {
        Scope                                       tenancy.Context
        ConnectionID, SecretID, EventID, AuditID, CorrelationID id.UUID
        Key, Hash                                   string
        ConnectorID, ConnectorVer, Label            string
        Environment                                 connectorDomain.Environment
        Config                                      map[string]string
        Now                                         time.Time
}

type SaveParams struct {
        Scope                                       tenancy.Context
        ConnectionID, EventID, AuditID, CorrelationID id.UUID
        ExpectedVersion                             int64
        Config                                      map[string]string
        Label, Summary                              string
        Now                                         time.Time
}

type ConnectParams struct {
        Scope                                       tenancy.Context
        ConnectionID, SecretID, EventID, AuditID, CorrelationID id.UUID
        ExpectedVersion                             int64
        Seal                                        bool
        Sealed, Key                                 []byte
        Hint                                        string
        Check                                       connectorDomain.CheckResult
        Now                                         time.Time
        Rotate                                      bool
        Operation                                   string
        IdempotencyKey, RequestHash                 string
}

type RevokeParams struct {
        Scope                                       tenancy.Context
        ConnectionID, EventID, AuditID, CorrelationID id.UUID
        ExpectedVersion                             int64
        Reason                                      string
        Now                                         time.Time
        Operation                                   string
        IdempotencyKey, RequestHash                 string
}

// SaveEvidenceParams carries one immutable evidence row plus its idempotency
// anchors and event identifiers.
type SaveEvidenceParams struct {
        Scope                                       tenancy.Context
        EvidenceID, ConnectionID, SecretID, EventID, AuditID, CorrelationID id.UUID
        Environment                                 connectorDomain.Environment
        ContractVersion, AdapterBuild, Capability   string
        ProbeKind                                   string
        RequestedAt, DeadlineAt, CompletedAt        time.Time
        Outcome, DeliveryLevel                      string
        RedactedRequest, RedactedResult             string
        ExpiresAt                                   time.Time
        IdempotencyKey, RequestHash                 string
}

type Repository interface {
        Create(context.Context, CreateParams) (connectorDomain.Connection, bool, error)
        List(context.Context, tenancy.Context) ([]connectorDomain.Connection, error)
        Read(context.Context, tenancy.Context, id.UUID) (connectorDomain.Connection, error)
        ReadVersions(context.Context, tenancy.Context, id.UUID) ([]connectorDomain.ConnectionVersion, error)
        Save(context.Context, SaveParams) (connectorDomain.Connection, error)
        Connect(context.Context, ConnectParams) (connectorDomain.Connection, bool, error)
        Revoke(context.Context, RevokeParams) (connectorDomain.Connection, bool, error)
        SaveEvidence(context.Context, SaveEvidenceParams) (connectorDomain.CapabilityEvidence, bool, error)
        ListEvidence(context.Context, tenancy.Context, id.UUID) ([]connectorDomain.CapabilityEvidence, error)
        // LookupEvidence answers the verify-command idempotency question before
        // any guard: same key+hash replays the stored evidence, a different hash is
        // a conflict. No side effects.
        LookupEvidence(ctx context.Context, scope tenancy.Context, connectionID id.UUID, key, hash string) (connectorDomain.CapabilityEvidence, bool, error)
        // LookupCommand answers the idempotency question before any domain guard:
        // a matching record replays the immutable version snapshot of the command,
        // a different request hash is an honest conflict. No audit side effects.
        LookupCommand(ctx context.Context, scope tenancy.Context, operation, key, hash string) (connectorDomain.Connection, bool, error)
}
