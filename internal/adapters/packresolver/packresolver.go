package packresolver

import (
	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/employee/packs"
)

// Adapter resolves action PackRefs against the published employee Domain
// Pack catalog — the composition-root wiring of the action port: the action
// module never imports the employee module, the root adapts the live
// catalog to the consumer-owned contract.
type Adapter struct{}

// New returns the stateless adapter.
func New() *Adapter { return &Adapter{} }

// The structural pin: the adapter satisfies the domain resolver interface
// (and therefore the port contract — byte-identical method sets).
var _ domain.PackRefResolver = Adapter{}

// Get reports whether the exact pack version is published (the pin pair —
// a definition references the version it was admitted with, never "the
// newest"; the published map is private and every call reads a copy, so
// the answer is the immutable catalog truth).
func (Adapter) Get(packID, packVersion string) bool {
	_, ok := packs.Get(packID, packVersion)
	return ok
}
