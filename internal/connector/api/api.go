package api

import (
	"context"
	"time"

	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

type CreateCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	IdempotencyKey string
	CorrelationID  id.UUID
	ConnectorID    string
	ConnectorVer   string
	Label          string
	Environment    connectorDomain.Environment
	Config         map[string]string
}

type SaveCommand struct {
	Actor                            domain.Principal
	TenantID, ConnectionID, CorrelationID id.UUID
	ExpectedVersion                  int64
	Config                           map[string]string
	Label                            string
	ChangeSummary                    string
}

type ConnectCommand struct {
	Actor                            domain.Principal
	TenantID, ConnectionID, CorrelationID id.UUID
	ExpectedVersion                  int64
	Secrets                          map[string]string
}

type RevokeCommand struct {
	Actor                            domain.Principal
	TenantID, ConnectionID, CorrelationID id.UUID
	ExpectedVersion                  int64
	Reason                           string
}

type CreateResult struct {
	Connection connectorDomain.Connection
	Replayed   bool
}

// Verifier performs the allowed basic authentication and transport check.
// Implementations must redact secrets and provider responses in every result.
type Verifier interface {
	BasicCheck(ctx context.Context, def connectorDomain.ConnectorDefinition, env connectorDomain.Environment, config map[string]string, secrets map[string]string) connectorDomain.CheckResult
}

type Service interface {
	Catalog(ctx context.Context, actor domain.Principal, tenantID id.UUID) ([]connectorDomain.ConnectorDefinition, error)
	Create(ctx context.Context, c CreateCommand) (CreateResult, error)
	List(ctx context.Context, actor domain.Principal, tenantID id.UUID) ([]connectorDomain.Connection, error)
	Read(ctx context.Context, actor domain.Principal, tenantID, connectionID id.UUID) (connectorDomain.Connection, error)
	ReadVersions(ctx context.Context, actor domain.Principal, tenantID, connectionID id.UUID) ([]connectorDomain.ConnectionVersion, error)
	Save(ctx context.Context, c SaveCommand) (connectorDomain.Connection, error)
	Connect(ctx context.Context, c ConnectCommand) (connectorDomain.Connection, error)
	Rotate(ctx context.Context, c ConnectCommand) (connectorDomain.Connection, error)
	Revoke(ctx context.Context, c RevokeCommand) (connectorDomain.Connection, error)
}

type Clock interface{ Now() time.Time }
type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}
