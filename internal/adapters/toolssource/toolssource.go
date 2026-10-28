package toolssource

// The production ToolSource of the MCP surface (Story 7.3): the
// composition-root adapter over the embedded 7-2 registry. The MCP module
// never imports the registry directly — the root adapts the live catalog
// to the consumer-owned port (the capabilityresolver/packresolver
// precedent of 3-1), and the registry stays the single source of the
// contracts (the clone discipline of the catalog: every caller receives
// a copy, the published versions stay immutable).

import (
	"github.com/artemiosu/staffy/internal/mcp"
	"github.com/artemiosu/staffy/internal/tools"
)

// Source is the stateless production adapter over the embedded registry.
type Source struct{}

// New returns the stateless adapter.
func New() *Source { return &Source{} }

// The structural pin: the adapter satisfies the consumer-owned port (the
// method sets are byte-identical).
var _ mcp.ToolSource = Source{}

// List returns every published contract in the canonical index order.
func (Source) List() []tools.ToolContract { return tools.List() }

// Get pins the contract of the exact (id, version) pair — the immutable
// read of a published version, or false when the pin never existed.
func (Source) Get(toolID, version string) (tools.ToolContract, bool) {
	return tools.Get(toolID, version)
}
