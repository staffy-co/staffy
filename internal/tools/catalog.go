package tools

// The catalog serves the published typed tool contracts from the artifact
// registry (Story 7-2, the catalog discipline of Story 7-1): JSON
// artifacts pinned by sha256 in the append-only index, embedded into the
// binary and fully validated at load. The registry is private and every
// caller receives a copy, keeping published versions immutable. The load
// is fail-closed: one invalid artifact refuses the WHOLE registry — a
// partial tool dictionary never exists (CI catches the break before
// deploy; the runtime panics instead of serving half a dictionary).
// Registering a contract is NOT availability of a call: the dispatch
// lives with the future callers (Planner 8.3, MCP server 7.3), and a
// not_implemented adapter is visible but callable by nobody.

var registry = mustLoadRegistry()

func mustLoadRegistry() []ToolContract {
        contracts, err := loadRegistry(registryFiles)
        if err != nil {
                panic("tool registry: " + err.Error())
        }
        return contracts
}

// Get returns the contract of the exact published version (the pin read
// of a precise version — the material for the future manifest and
// planner references) or false when the id/version is not published.
func Get(id, version string) (ToolContract, bool) {
        for _, c := range registry {
                if c.ID == id && c.Version == version {
                        return clone(c), true
                }
        }
        return ToolContract{}, false
}

// List returns every published contract in the canonical index order.
func List() []ToolContract {
        out := make([]ToolContract, 0, len(registry))
        for _, c := range registry {
                out = append(out, clone(c))
        }
        return out
}

// clone returns the deep copy of a registry contract: the schemas, the
// values and the dependency are copied so a caller's mutation is
// invisible to the next reader. Nil dependency stays nil (the wire form
// omits it either way).
func clone(c ToolContract) ToolContract {
        for _, schema := range []map[string]ToolSchemaField{c.InputSchema, c.OutputSchema} {
                for name, field := range schema {
                        field.Values = append([]string(nil), field.Values...)
                        schema[name] = field
                }
        }
        if c.Dependency != nil {
                dep := *c.Dependency
                c.Dependency = &dep
        }
        return c
}
