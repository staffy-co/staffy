package api

import (
	"context"
	"time"

	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
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
	IdempotencyKey                   string
}

type RevokeCommand struct {
	Actor                            domain.Principal
	TenantID, ConnectionID, CorrelationID id.UUID
	ExpectedVersion                  int64
	Reason                           string
	IdempotencyKey                   string
}

// VerifyCommand runs the safe read-only probe for one declared capability of
// one connection. The idempotency key replays the same evidence instead of
// producing contradictory rows.
// VerifyCommand runs the safe read-only probe for one declared capability of
// one connection. The idempotency key replays the same evidence instead of
// producing contradictory rows. The Secrets member (Story 7-4) carries the
// probe key material of the external MCP class: in-memory only — the service
// MAC-checks it against the accepted seal BEFORE any network round, and the
// material is never persisted, never logged and never echoed (the 1-8
// invariant).
type VerifyCommand struct {
	Actor                            domain.Principal
	TenantID, ConnectionID, CorrelationID id.UUID
	Capability                       string
	IdempotencyKey                   string
	Secrets                          map[string]string
}

type CreateResult struct {
	Connection connectorDomain.Connection
	Replayed   bool
}

type ConnectResult struct {
	Connection connectorDomain.Connection
	Replayed   bool
}

type RevokeResult struct {
	Connection connectorDomain.Connection
	Replayed   bool
}

// VerifyResult returns the derived state of the verified capability and
// whether the command replayed an earlier stored outcome.
type VerifyResult struct {
	Report   connectorDomain.CapabilityReport
	Replayed bool
}

// CapabilityStateResult is the derived capability view of one connection.
type CapabilityStateResult struct {
	Connection      connectorDomain.Connection
	ContractVersion string
	AdapterBuild    string
	Capabilities    []connectorDomain.CapabilityReport
}

// Verifier performs the allowed basic authentication and transport check plus
// the safe read-only capability probe. Implementations must redact secrets and
// provider responses in every result.
type Verifier interface {
	BasicCheck(ctx context.Context, def connectorDomain.ConnectorDefinition, env connectorDomain.Environment, config map[string]string, secrets map[string]string) connectorDomain.CheckResult
	Probe(ctx context.Context, def connectorDomain.ConnectorDefinition, dec connectorDomain.CapabilityDeclaration, env connectorDomain.Environment, config map[string]string, secrets map[string]string) ProbeResult
	Build() string
}

// ProbeResult is the redacted outcome of the synthetic read-only probe.
type ProbeResult struct {
	Outcome         string
	DeliveryLevel   string
	RedactedRequest string
	RedactedResult  string
}

type Service interface {
	Catalog(ctx context.Context, actor domain.Principal, tenantID id.UUID) ([]connectorDomain.ConnectorDefinition, error)
	Create(ctx context.Context, c CreateCommand) (CreateResult, error)
	List(ctx context.Context, actor domain.Principal, tenantID id.UUID) ([]connectorDomain.Connection, error)
	Read(ctx context.Context, actor domain.Principal, tenantID, connectionID id.UUID) (connectorDomain.Connection, error)
	ReadVersions(ctx context.Context, actor domain.Principal, tenantID, connectionID id.UUID) ([]connectorDomain.ConnectionVersion, error)
	Save(ctx context.Context, c SaveCommand) (connectorDomain.Connection, error)
	Connect(ctx context.Context, c ConnectCommand) (ConnectResult, error)
	Rotate(ctx context.Context, c ConnectCommand) (ConnectResult, error)
	Revoke(ctx context.Context, c RevokeCommand) (RevokeResult, error)
	VerifyCapability(ctx context.Context, c VerifyCommand) (VerifyResult, error)
	CapabilityState(ctx context.Context, actor domain.Principal, tenantID, connectionID id.UUID) (CapabilityStateResult, error)
	ConnectionsFacts(ctx context.Context, scope tenancy.Context) ([]ConnectionFacts, error)
	ConnectionFactsByID(ctx context.Context, scope tenancy.Context, connectionID id.UUID) (ConnectionFacts, error)
}

// ConnectionFacts is the safe cross-module projection of one connection for
// consumers that already hold their own scope (the release surface, Story
// 1-12): identifiers, contract and adapter versions, environment, state and
// the DERIVED capability reports — never secrets, secret hints, config
// values or labels.
type ConnectionFacts struct {
	ConnectionID    id.UUID                            `json:"connection_id"`
	ConnectorID     string                             `json:"connector_id"`
	ConnectorType   string                             `json:"connector_type"`
	ContractVersion string                             `json:"contract_version"`
	AdapterBuild    string                             `json:"adapter_build"`
	Environment     string                             `json:"environment"`
	Status          string                             `json:"status"`
	Capabilities    []connectorDomain.CapabilityReport `json:"capabilities"`
}

type Clock interface{ Now() time.Time }
type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}
