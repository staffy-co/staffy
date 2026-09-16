# Staffy Graph Health

Checked: 2026-09-16

## Scope

- 148 project files
- 139 documents
- 8 project JSON schemas
- 1 PDF
- Included: `_bmad-output/**` and future product code
- Excluded: `.agents/**`, `_bmad/**`, `.codex/**`, `AGENTS.md`, and `graphify-out/**`
- Excluded: the root source file `чат с ии-поисковиком.md`

## Current graph

- Directed graph (`DiGraph`)
- 368 nodes
- 375 edges
- 24 communities

## Pre-build extraction diagnostics

- Missing-endpoint edges: 0
- Dangling-endpoint edges: 2
- Self-loops: 0
- Exact duplicate edges: 0
- Directed same-endpoint collapsed edges: 0
- Undirected same-endpoint collapsed edges avoided by directed mode: 3

The two dangling edges are the `$ref` links from the aggregate gate-registry schemas to their local item schemas. They are a JSON Schema reference-resolution limitation in the current Graphify extraction, not broken Staffy requirements or runtime dependencies.

## Final graph diagnostics

- Missing-endpoint edges: 0
- Dangling-endpoint edges: 0
- Self-loops: 0
- Directed same-endpoint collapsed edges: 0

The two unresolved extraction-only `$ref` edges were not admitted into the final graph.

The latest incremental update indexed the Sprint Planning artifacts, including
`sprint-status.yaml`, and added dedicated Sprint Planning and Sprint Status
communities without bringing BMAD framework internals into the graph.

## Assessment

The working graph now represents Staffy project knowledge rather than BMAD Method implementation internals. It is suitable for navigating the planning corpus. After Go product code appears, repeat the same health check against both the product code and changed project documentation.
