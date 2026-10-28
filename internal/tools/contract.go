package tools

// ToolContract is the versioned typed contract of one deterministic/read
// tool (Story 7-2): declarative metadata only — the id, the version, the
// human name and purpose, the determinism class, the typed input/output
// schemas, the owner, the evidence reference and the honest adapter
// status. The contract is data, never executable code: the registry
// validates it BEFORE any use, and a contract's semantics change only by
// a NEW version (a new artifact file plus a new index entry — the sha256
// pin of the append-only index refuses the in-place edit of a published
// version). Executing a contract is NOT registration: the dispatch lives
// with the future callers (Planner 8.3, MCP server 7.3), and a
// not_implemented adapter is visible in the registry but callable by
// nobody.
type ToolContract struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	// DeterminismClass is the platform safety class of the tool: the
	// dictionary carries the single value deterministic_read — a
	// write-class tool is structurally impossible (a value outside the
	// dictionary refuses the WHOLE registry; extending the dictionary is
	// a core change, never an artifact change — AD-6/AD-7).
	DeterminismClass string `json:"determinism_class"`
	// InputSchema and OutputSchema are the typed key dictionaries of the
	// contract in the platform typing language (the pack variant
	// dictionary: string | integer | boolean) — json-schema.org is not
	// introduced; extending the typing dictionary is a core change.
	InputSchema  map[string]ToolSchemaField `json:"input_schema"`
	OutputSchema map[string]ToolSchemaField `json:"output_schema"`
	// Owner is the owning surface of the contract (the vertical domain
	// that publishes and maintains it).
	Owner string `json:"owner"`
	// EvidenceRef is the symbolic versioned reference of the proof behind
	// the contract (the spec, the machine and the tests) — like the
	// policy_ref/knowledge_ref/eval_ref of a pack; the resolution is the
	// PR review (capability claims stay honest — the Montagio/Avito
	// precedent).
	EvidenceRef string `json:"evidence_ref"`
	// AdapterStatus is the honest implementation state of the adapter:
	// implemented | not_implemented. A not_implemented contract is
	// declared and visible but never callable (the AdapterImplemented
	// discipline of the connectors, applied to the tools).
	AdapterStatus string `json:"adapter_status"`
	// Dependency declares the connector TYPE the tool's adapter needs (a
	// type, never a concrete connector): nil for the self-contained
	// contracts; a declared dependency is cross-validated against the
	// connector_types of the binding pack at load (the static half of
	// AC-3 — the dynamic capability probe is the caller's, 7.3/8.3).
	Dependency *ToolDependency `json:"dependency,omitempty"`
}

// ToolSchemaField is one typed key of a tool contract schema: the human
// label, the type (the pack typing dictionary: string | integer |
// boolean), the enum values, the integer bounds and the optional flag.
// The schema is declarative contract metadata: it never carries
// executable policy.
type ToolSchemaField struct {
	Label    string   `json:"label"`
	Type     string   `json:"type"` // string | integer | boolean
	Values   []string `json:"values,omitempty"`
	Min      int      `json:"min,omitempty"`
	Max      int      `json:"max,omitempty"`
	Optional bool     `json:"optional,omitempty"`
}

// ToolDependency declares the connector type the tool's adapter needs:
// the binding pack must declare the same connector type (a dependency the
// domain does not carry is a load-time refusal). The concrete allowed
// sources stay in the vertical fields of the pack (fitment.allowed_sources).
type ToolDependency struct {
	ConnectorType string `json:"connector_type"`
}
