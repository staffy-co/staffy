package externaltools

// The dictionary entry form (Story 7-4 step-02): the registry discipline
// of 7-2 applied to the live external material — the typed contract shape
// in the platform typing dictionary, the provenance of the snapshot (the
// class card pin, the server identity of the discover round, the evidence
// row and its TTL) and the honest admission state (a descriptor outside
// the projection subset is a REJECTED row with the named reason, never a
// silence and never a passthrough).

import (
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/tools"
)

// The admission states of one dictionary row.
const (
	EntryAdmitted = "admitted"
	EntryRejected = "rejected"
)

// Provenance is the snapshot identity of the entry: the class card pin,
// the external server identity, the evidence row and its validity window.
// The external tools carry NO versions in the pinned revision — the
// provenance IS the version statement of the entry (a named tool version
// arrives only if the protocol ever introduces one).
type Provenance struct {
	ConnectorID      string    `json:"connector_id"`
	ConnectorVersion string    `json:"connector_version"`
	ServerName       string    `json:"server_name"`
	ServerVersion    string    `json:"server_version"`
	EvidenceID       id.UUID   `json:"evidence_id"`
	CheckedAt        time.Time `json:"checked_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// Entry is one row of the dictionary: the filtered descriptor of the
// external inventory with its projected typed schemas and the admission
// state. A rejected row carries the named reason and no schemas — the
// material that failed the projection never travels further.
type Entry struct {
	Name        string                           `json:"name"`
	Title       string                           `json:"title"`
	Description string                           `json:"description"`
	State       string                           `json:"state"`
	Reason      string                           `json:"reason,omitempty"`
	Provenance  *Provenance                      `json:"provenance,omitempty"`
	Input       map[string]tools.ToolSchemaField `json:"input_schema,omitempty"`
	Output      map[string]tools.ToolSchemaField `json:"output_schema,omitempty"`
}

// Admitted reports whether the row passed the projection.
func (e Entry) Admitted() bool { return e.State == EntryAdmitted }

// Section is the per-connection block of the dictionary: the entries of
// the applicable evidence snapshot, or the honest reason the block carries
// no entries (the connection guard, the probe state or the stale anchors —
// the words of the connector machine vocabulary).
type Section struct {
	ConnectionID    id.UUID `json:"connection_id"`
	ConnectorID     string  `json:"connector_id"`
	ContractVersion string  `json:"contract_version"`
	Environment     string  `json:"environment"`
	Status          string  `json:"status"`
	HasEntries      bool    `json:"has_entries"`
	Reason          string  `json:"reason,omitempty"`
	Entries         []Entry `json:"entries"`
}
