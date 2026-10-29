package externaltoolsfacts

// The composition adapter of the external tools dictionary (Story 7-4
// step-02): the LIVE connection facts of the connector machine become the
// InventoryFactsSource of the dictionary — the adapter pins the class card
// and the query.tools-list capability at the WIRING level (the
// toolsdependency precedent of 7-3: the consumer module never knows the
// pin, the root owns it). The adapter also ships the mcpclient-backed
// RoundTransport of the call machine — the production wire over the
// default transport with the machine budget.

import (
	"context"
	"fmt"
	"net/http"

	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	"github.com/artemiosu/staffy/internal/externaltools"
	"github.com/artemiosu/staffy/internal/mcpclient"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// The wiring-level pins of the dictionary: the class card of the catalog
// and the capability row the inventory rides.
const (
	externalConnectorID   = "staffy-external-mcp-server"
	toolsListCapabilityID = "query.tools-list"
)

// ConnectionsFactsSource is the narrow caller-authority fact port the
// concrete connector service satisfies structurally (the release surface
// precedent of 1-12: the consumer verified its own scope, the facts read
// does not re-require a user principal).
type ConnectionsFactsSource interface {
	ConnectionsFacts(ctx context.Context, scope tenancy.Context) ([]connectorAPI.ConnectionFacts, error)
}

// Source is the stateless production adapter over the live facts.
type Source struct {
	connections ConnectionsFactsSource
}

// New wires the live connection facts source.
func New(connections ConnectionsFactsSource) *Source { return &Source{connections: connections} }

// The structural pin: the adapter satisfies the consumer-owned port.
var _ externaltools.InventoryFactsSource = (*Source)(nil)

// InventoryFacts answers the per-connection inventories of the tenant
// scope: the connections of the pinned class card with their derived
// query.tools-list report (the evidence summary carries the filtered
// inventory of the probe). Connections of other classes never enter the
// dictionary material.
func (s *Source) InventoryFacts(ctx context.Context, scope tenancy.Context) ([]externaltools.ConnectionInventory, error) {
	rows, err := s.connections.ConnectionsFacts(ctx, scope)
	if err != nil {
		return nil, err
	}
	out := make([]externaltools.ConnectionInventory, 0, len(rows))
	for _, row := range rows {
		if row.ConnectorID != externalConnectorID {
			continue
		}
		for _, candidate := range row.Capabilities {
			if candidate.ID != toolsListCapabilityID {
				continue
			}
			out = append(out, externaltools.ConnectionInventory{
				ConnectionID:    row.ConnectionID,
				ConnectorID:     row.ConnectorID,
				ContractVersion: row.ContractVersion,
				Environment:     row.Environment,
				Status:          row.Status,
				Report:          candidate,
			})
			break
		}
	}
	return out, nil
}

// WireTransport is the mcpclient-backed RoundTransport of the call
// machine: the production wire over the injected http.RoundTripper with
// the declared reading budget. The Authorization material and the endpoint
// arrive as the caller's explicit parameters and travel to the round
// verbatim (never stored, never logged, never echoed).
type WireTransport struct {
	client *mcpclient.Client
}

// NewWireTransport builds the production round transport (the composition
// root injects the default transport; the machine budget mirrors the
// declared SizeLimit of the class card).
func NewWireTransport(transport http.RoundTripper, responseBudget int64) *WireTransport {
	return &WireTransport{client: mcpclient.New(transport, responseBudget)}
}

// The structural pin: the transport satisfies the consumer-owned port.
var _ externaltools.RoundTransport = (*WireTransport)(nil)

// CallTool runs one authenticated round and answers the structured content
// of the tool. The honest error identities of the client machine travel
// as-is (the deadline is the honest unknown of a possibly-effective
// round); a tool execution error of the foreign server is refused without
// its untrusted words (AD-11: only the fact travels).
func (t *WireTransport) CallTool(ctx context.Context, endpoint, authorization, toolName string, arguments map[string]any) (map[string]any, error) {
	answer, err := t.client.CallTool(ctx, endpoint, authorization, toolName, arguments)
	if err != nil {
		return nil, err
	}
	if answer.IsError {
		return nil, fmt.Errorf("mcpclient: внешний сервер сообщил об ошибке выполнения инструмента (isError)")
	}
	if answer.StructuredContent == nil {
		return map[string]any{}, nil
	}
	return answer.StructuredContent, nil
}
