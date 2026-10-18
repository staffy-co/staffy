package capabilityresolver

import (
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/connector/catalog"
)

// Adapter resolves action CapabilityRefs against the published connector
// catalog — the composition-root wiring of the action port: the action
// module never imports the connector module, the root adapts the live
// catalog (with its CapabilityDeclarations of 1-8) to the consumer-owned
// contract.
//
// A capability resolves only when its CapabilityDeclaration is Complete:
// 1-8 renders missing semantics as `unsupported`, and an action definition
// may never lean on an unsupported capability (fail-closed).
type Adapter struct{}

// New returns the stateless adapter.
func New() *Adapter { return &Adapter{} }

// The structural pin: the adapter satisfies the domain resolver interface
// (and therefore the port contract — byte-identical method sets).
var _ domain.CapabilityRefResolver = Adapter{}

// Capability returns the provider contract facts of the exact
// (connector, version, capability) triple and reports whether it resolves
// to a complete declaration. The facts carry exactly the words of the
// delivery-word cross-check (AC-3): Idempotent and the declared
// reconciliation semantics. Unknown connector, unknown version, unknown
// capability or an incomplete declaration all answer false — the honest
// unresolvable, never a guess.
func (Adapter) Capability(connectorID, connectorVersion, capabilityID string) (domain.CapabilityFacts, bool) {
	definition, ok := catalog.Get(connectorID, connectorVersion)
	if !ok {
		return domain.CapabilityFacts{}, false
	}
	for _, capability := range definition.Capabilities {
		if capability.ID != capabilityID {
			continue
		}
		if !capability.Complete() {
			return domain.CapabilityFacts{}, false
		}
		return domain.CapabilityFacts{
			Idempotent:     capability.Idempotent,
			Reconciliation: capability.Reconciliation,
		}, true
	}
	return domain.CapabilityFacts{}, false
}
