package port

import (
	"github.com/artemiosu/staffy/internal/action/domain"
)

// The consumer-owned resolution ports of the action module. The action
// module never imports the employee or connector modules: the composition
// root adapts their published catalogs to these contracts. The method sets
// are byte-identical to the domain resolver interfaces (the compile-time
// structural pins below) — any adapter implementing the port satisfies the
// domain interface too, so ValidateDefinitionRefs consumes it without glue.

// PackRefResolver resolves a definition's PackRef against the published
// Domain Pack catalog (the composition-root adapter over the employee pack
// catalog).
type PackRefResolver interface {
	// Get reports whether the exact pack version is published (the pin
	// pair, never "the newest").
	Get(packID, packVersion string) bool
}

// CapabilityRefResolver resolves a definition's RequiredCapability against
// the published connector catalog (the composition-root adapter over the
// connector catalog): the triple resolves only when its
// CapabilityDeclaration is Complete, and the returned facts carry the
// provider contract words of the delivery-word cross-check (AC-3).
type CapabilityRefResolver interface {
	// Capability returns the provider contract facts of the exact
	// (connector, version, capability) triple and reports whether it
	// resolves to a complete declaration.
	Capability(connectorID, connectorVersion, capabilityID string) (domain.CapabilityFacts, bool)
}

// The structural pins: the port contracts satisfy the domain resolver
// interfaces exactly (interface-to-interface assignment is the compile-time
// check), so every adapter implements both without conversion glue.
var (
	_ domain.PackRefResolver       = PackRefResolver(nil)
	_ domain.CapabilityRefResolver = CapabilityRefResolver(nil)
)
