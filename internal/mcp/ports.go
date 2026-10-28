package mcp

// The ports of the MCP surface (Story 7.3): the narrow consumer-owned
// contracts the composition root adapts to the live modules. The service
// depends ONLY on these ports — the transport and the internal call path
// stay separable (the future Planner 8.3 consumes the same form without
// the MCP transport), and the production adapters of the root carry the
// AG-4 facts: the registry source is the real embedded 7-2 registry, the
// dependency gate reads the live connection facts, and the adapter
// registry holds ZERO adapters — the callable set is honestly empty (the
// nil-adapter precedents of 3-6/3-7: the surface answers, the execution
// refuses in words).

import (
	"context"

	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/tools"
)

// ToolSource is the read-only registry source of the surface: the list of
// the published contracts and the pin read of the exact (id, version)
// pair. The production adapter wraps the embedded 7-2 registry (the clone
// discipline of the catalog); the fixture world injects its own source
// (the toolResolver precedent of the packs loader).
type ToolSource interface {
	List() []tools.ToolContract
	Get(toolID, version string) (tools.ToolContract, bool)
}

// Adapter executes ONE tool call of the pinned contract version with the
// tenant scope of the SESSION (the path + the ladder — the tenant NEVER
// travels in the arguments) and the declared context budget. The adapter
// is vertical by nature (it binds the contract to the domain machine);
// the production composition root registers ZERO adapters (AG-4), and the
// first live adapter arrives as a new artifact version plus a root wiring
// of a future slice.
type Adapter func(ctx context.Context, tenant id.UUID, arguments map[string]any) (map[string]any, error)

// AdapterRegistry resolves the runtime adapter of the exact contract pin.
// An unregistered pin answers false — the honest refusal of the call path
// names the fact (a registered contract is not a callable tool until the
// composition root registers the adapter).
type AdapterRegistry interface {
	Adapter(toolID, version string) (Adapter, bool)
}

// DependencyGate answers the LIVE dependency fact of one connector type
// for the tenant of the session (the dynamic half of the AC-3 gate of the
// 7-2 spec): the returned word carries the state vocabulary of the
// connector machine (verified, unverified, failed, stale, ...) for the
// audit line, and the error is the honest infrastructure failure (the
// gate never guesses a live from a failed read).
type DependencyGate interface {
	Live(ctx context.Context, actor identityDomain.Principal, tenant id.UUID, connectorType string) (bool, string, error)
}

// HeaderClaims carries the transport routing headers of one POST: the
// claims the surface checks against the body (the desync is the invalid
// request of the revision).
type HeaderClaims struct {
	Method          string
	Name            string
	ProtocolVersion string
}

// MembershipService is the identity ladder port of the surface (the
// tools.Service precedent of 7-2): the authorization of the
// integrations|configuration scopes runs FIRST, before any protocol
// material leaves the service.
type MembershipService = identityAPI.MembershipService
