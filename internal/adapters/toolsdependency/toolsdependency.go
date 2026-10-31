package toolsdependency

// The production DependencyGate of the MCP surface (Story 7.3): the
// composition-root adapter over the LIVE connection facts of the connector
// machine (Story 1-8). The gate answers the dynamic half of the AC-3
// exposure fact of the 7-2 spec: a dependency is live when the tenant of
// the session holds a live connection of the declared connector type with
// a VERIFIED capability and no FAILED or STALE capability among the live
// connections of the type (the DeriveCapabilityStates vocabulary — a
// rotated secret or a changed configuration honestly reads as stale, the
// derived state is a pure function of the facts). The adapter never
// imports the MCP module and the connector module never knows the gate:
// the root adapts (the ConnectionsFacts precedent of the release surface,
// the capabilityresolver composition precedent of 3-1).

import (
	"context"

	connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
	connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/mcp"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ConnectionsFactsSource is the narrow caller-authority fact port the
// concrete connector service satisfies structurally (the release surface
// precedent of 1-12: the consumer verified its own scope, the facts read
// does not re-require a user principal).
type ConnectionsFactsSource interface {
	ConnectionsFacts(ctx context.Context, scope tenancy.Context) ([]connectorAPI.ConnectionFacts, error)
}

// Gate is the stateless production adapter over the live facts.
type Gate struct {
	connections ConnectionsFactsSource
}

// New wires the live connection facts source.
func New(connections ConnectionsFactsSource) *Gate { return &Gate{connections: connections} }

// The structural pin: the adapter satisfies the consumer-owned port.
var _ mcp.DependencyGate = (*Gate)(nil)

// Live answers the dependency fact of one connector type for the tenant
// of the session. The state word of the refusal carries the vocabulary of
// the connector machine for the audit line: "no_live_connection" when no
// live connection of the type exists, the first failed/stale word when a
// proof invalidates the type, "unverified" when nothing is proven yet;
// the live answer carries "verified".
func (g *Gate) Live(ctx context.Context, actor domain.Principal, tenant id.UUID, connectorType string) (bool, string, error) {
	scope, err := tenancy.New(actor.ID, tenant)
	if err != nil {
		return false, "no_live_connection", nil
	}
	rows, err := g.connections.ConnectionsFacts(ctx, scope)
	if err != nil {
		return false, "", err
	}
	// The live connections of the declared type only: a draft, failed or
	// revoked connection of the type proves nothing and does not poison
	// the word of a live sibling — but it does not answer the fact either.
	var liveSeen, verifiedSeen, failedSeen, staleSeen bool
	for _, row := range rows {
		if row.ConnectorType != connectorType || row.Status != connectorDomain.StatusConnectedUnverified {
			continue
		}
		liveSeen = true
		for _, capability := range row.Capabilities {
			switch capability.State {
			case connectorDomain.CapStateVerified:
				verifiedSeen = true
			case connectorDomain.CapStateFailed:
				failedSeen = true
			case connectorDomain.CapStateStale:
				staleSeen = true
			}
		}
	}
	// The fail-closed order: a failed or stale proof of the live type
	// closes the gate BEFORE an unverified live connection could be read
	// as hopeful; the verified word needs a live connection AND a proven
	// capability; no live connection of the type answers the fact itself.
	switch {
	case failedSeen:
		return false, "failed", nil
	case staleSeen:
		return false, "stale", nil
	case !liveSeen:
		return false, "no_live_connection", nil
	case verifiedSeen:
		return true, "verified", nil
	default:
		return false, "unverified", nil
	}
}
