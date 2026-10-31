package externaltools

// Package externaltools is the tenant-scoped operator dictionary of the
// read-tools of the external MCP connections (Story 7-4 step-02). The
// dictionary is a PURE FUNCTION of facts (the DeriveCapabilityStates
// precedent of 1-8): the entries exist exactly while the live connection
// of the class holds the APPLICABLE evidence of the query.tools-list probe
// and the descriptor projects onto the platform typing dictionary — a
// secret rotation, a config save, a build change or the TTL expiry makes
// the entries DISAPPEAR (no second record of states ever exists, a read
// never mutates anything). The registry discipline of 7-2 applies to the
// FORM only (the typed contract shape, the platform typing dictionary, the
// operator ladder, the honest refusals): the SOURCE of the entries is the
// live external evidence, not a git artifact — the embedded registry of
// 7-2 stays untouched (0 edits of internal/tools, the boundary of the
// spec).
//
// The call machine (Calls) is the credential- and target-agnostic form
// under the future Planner 8.3: the entry pin comes from the CURRENT
// derivation (the honest TOCTOU), the arguments and the answer are
// validated strictly against the projected dictionaries fail-closed, and
// the Authorization material with the endpoint are the explicit caller
// parameters (the production key source arrives with the AG-4 caller
// slice). No HTTP surface of the call exists in this slice.

import (
	"context"

	connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ConnectionInventory is one connection of the external MCP class with its
// DERIVED query.tools-list report: the report state already encodes the
// Applicable anchors (a rotated secret, a changed config, a build change
// or the TTL expiry read as stale — the entries disappear), and the
// evidence summary carries the filtered inventory JSON of the probe.
type ConnectionInventory struct {
	ConnectionID    id.UUID
	ConnectorID     string
	ContractVersion string
	Environment     string
	Status          string
	Report          connectorDomain.CapabilityReport
}

// InventoryFactsSource answers the per-connection inventories of the
// tenant scope: the caller has already verified its own authority (the
// composition adapter pins the class card and the query.tools-list
// capability at the wiring level — the toolsdependency precedent of 7-3).
type InventoryFactsSource interface {
	InventoryFacts(ctx context.Context, scope tenancy.Context) ([]ConnectionInventory, error)
}

// RoundTransport executes ONE authenticated tool-call round for the call
// machine: the endpoint and the Authorization material are the CALLER's
// explicit parameters (the machine never stores, logs or echoes them and
// never touches the connection config — the future Planner owns the
// target and the key source; the production caller arrives with the AG-4
// slice). The default implementation rides the mcpclient machine; the
// tests inject scripted transports.
type RoundTransport interface {
	CallTool(ctx context.Context, endpoint, authorization, toolName string, arguments map[string]any) (map[string]any, error)
}
