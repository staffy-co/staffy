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
}

type RevokeParams struct {
        Scope                                       tenancy.Context
        ConnectionID, EventID, AuditID, CorrelationID id.UUID
        ExpectedVersion                             int64
        Reason                                      string
        Now                                         time.Time
}

type Repository interface {
        Create(context.Context, CreateParams) (connectorDomain.Connection, bool, error)
        List(context.Context, tenancy.Context) ([]connectorDomain.Connection, error)
        Read(context.Context, tenancy.Context, id.UUID) (connectorDomain.Connection, error)
        ReadVersions(context.Context, tenancy.Context, id.UUID) ([]connectorDomain.ConnectionVersion, error)
        Save(context.Context, SaveParams) (connectorDomain.Connection, error)
        Connect(context.Context, ConnectParams) (connectorDomain.Connection, error)
        Revoke(context.Context, RevokeParams) (connectorDomain.Connection, error)
}
