package connectorrefresolver

import (
	"context"

	"github.com/artemiosu/staffy/internal/connector/catalog"
	controlPort "github.com/artemiosu/staffy/internal/control/port"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Adapter resolves connector scope refs against the published connector
// catalog — the composition-root wiring of the control port (the
// packresolver/capabilityresolver precedent of 3-1): the control module
// never imports the connector module, the root adapts the live 1-8
// dictionary to the consumer-owned contract.
//
// The honest boundary: the published catalog is the LIVE registry the
// slice can resolve against (every published connector word of 1-8); the
// tenant-scoped narrowing — only the connectors the tenant actually
// connected — arrives with the enforcement slices (the per-tenant
// registry is not a control-module concern: the port keeps the tenant id
// so that future narrowing never breaks the contract). An unknown word
// answers false — the honest unresolvable, never a guess.
type Adapter struct{}

// New returns the stateless adapter.
func New() *Adapter { return &Adapter{} }

// The structural pin: the adapter satisfies the control port contract.
var _ controlPort.ConnectorRefResolver = Adapter{}

// Resolve reports whether connectorID is a published connector word of
// the live 1-8 catalog (any version — the control ref addresses the
// connector, not a version pin). The catalog is read live on every call:
// a future edition of the dictionary is reflected without a restart.
func (Adapter) Resolve(_ context.Context, _ id.UUID, connectorID string) (bool, error) {
	for _, definition := range catalog.List() {
		if definition.ID == connectorID {
			return true, nil
		}
	}
	return false, nil
}
