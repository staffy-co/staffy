---
title: Staffy Architecture Consistency Review R2
status: review-complete
reviewed: 2026-09-15
targets:
  - ARCHITECTURE-SPINE.md
  - GATE-REGISTRY.md
  - gate-instance.schema.json
  - architecture-map.html
---

# Architecture consistency review — R2

## Verdict

**Narrow revision still required before finalization.** Of the 19 R1 findings, **17 are resolved, 1 is partially resolved but remains High, and 1 remains Low**. The revisions introduced **3 new Medium findings**. Open total: **High 1 · Medium 3 · Low 1 · Critical 0**.

Deterministic checks remain clean: `lint_spine.py` reports 0 findings; all source paths resolve; exact dependency pins are now present.

## R1 resolution tally

| R1 | Result | Evidence |
|---|---|---|
| H1 Gate lifecycle/schema | **Partial — High remains** | Exact states and `gate-instance.schema.json` were added, but no current instance is represented as a schema-valid machine record. |
| H2 OQ-13 false PASS | Resolved | Split into OQ-13A policy and open OQ-13B recovery proof. |
| H3 OQ-7 ordering deadlock | Resolved | Split into pre-controlled OQ-7A and post-baseline OQ-7B. |
| H4 Pilot Load Envelope | Resolved | AG-1 plus AD-14 and execution-order binding. |
| H5 dependency/transaction direction | Resolved | AD-2 now defines package shape, import DAG, transaction/outbox boundary and CI checks. |
| H6 missing Work Item | Resolved | AD-3/AD-4, ERD and capability map assign it to `case`. |
| H7 incomplete Connector protocol | Resolved | Read/Observation and explicit Cancel/Compensation contracts added. |
| H8 Avito pilot/deferred conflict | Resolved fail-closed | Avito is assisted/manual protected continuation until OQ-18 PASS. |
| M1 Mango presented as confirmed | Resolved fail-closed | All pilot labels now say actual verified provider; Mango is only a candidate. |
| M2 OQ-11 PII fallback | Resolved | Fallback must independently pass the same data-route gate. |
| M3 playback equals hearing | Resolved | AD-9 and HTML separate generated/provider/playback/customer-confirmed truth. |
| M4 Work Record owner | Resolved | Explicitly a Case-owned projection/surface, not a seventh module. |
| M5 missing delivery slices | Resolved | Source added; walking-skeleton rows and exit gates added. |
| M6 accessibility admission | Resolved | AG-2 and AD-12 preserve the WCAG 2.2 AA gate. |
| M7 missing SM-4 proof | Resolved | AG-3 and third-Tenant proof row added. |
| M8 floating dependency versions | Resolved | Protobuf generators, protoc and Node are separately pinned. |
| M9 incomplete HTML gate summary | Resolved | OQ-1–OQ-19 and AG-1–AG-4 are all represented. |
| L1 meaningless dependency self-loop | **Unresolved — Low** | The self-loop became `Domain -> Ports` labelled “has no provider dependency”, which still depicts the opposite of its label. |
| L2 ambiguous coverage ranges | Resolved | Normalized `..` ranges and SM/SM-C bindings added. |

## Remaining High

### R2-H1 — The “executable” registry still has no executable current instances

- **Location:** `GATE-REGISTRY.md:15-17,63-103`; `gate-instance.schema.json:7-23,28-47,48-107`
- **Finding:** The schema requires an object scope, structured evidence with URI/hash, dependencies, `control_generation`, rationale/supersession and decision history. The “Current scoped instances” table supplies human strings/`—`, omits several required fields and has no schema-valid instance files. PASS rows such as OQ-13A and OQ-19 therefore cannot be validated against the declared schema or consumed by the release compiler.
- **Fix:** Store each current instance as JSON/YAML conforming to `gate-instance.schema.json` (or a single validated instances document), validate it in CI, and declare the Markdown table a generated human projection. Add a machine-readable gate-definition registry or explicitly define the compiled code/artifact that maps Release Manifest edges to definition IDs.
- **Risk:** The release compiler remains an architectural claim rather than a reproducible admission mechanism; prose PASS can drift from runtime permission.

## New Medium

### R2-M1 — Spine frontmatter uses a status outside the template contract

- **Location:** `ARCHITECTURE-SPINE.md:8`; architecture template contract `status: draft | final`
- **Finding:** `status: adopted` is not a valid architecture-spine lifecycle value and appears before the reviewer gate is clean.
- **Fix:** Keep `status: draft` until all reviewer findings are resolved, then set `status: final`; adoption remains expressed by `[ADOPTED]` decisions and memlog events.
- **Risk:** Tooling and readers cannot distinguish an adopted decision from a finalized artifact.

### R2-M2 — The walking-skeleton labels redefine UJ-A1 as read-only

- **Location:** `ARCHITECTURE-SPINE.md:55,294-296`; sources `prd.md:200-214`, `delivery-slices.md:16`
- **Finding:** Canonical UJ-A1 ends with one confirmed ABCP customer order and manager task, while the spine calls a read-only fragment “UJ-A1” and moves fulfilment to “UJ-A7 prepared order”. UJ-A7 is the manager-review journey, not the order-creation journey.
- **Fix:** Label rows as slices, not complete journeys: e.g. `UJ-A1 slice 1: safe intake/read`, `UJ-A1 slice 2 + UJ-A7: prepared order and manager verification`. State that the full controlled UJ-A1 gate requires both.
- **Risk:** Story decomposition can declare UJ-A1 complete without delivering its required customer Order outcome.

### R2-M3 — HTML states target Montagio ownership as current fact

- **Location:** `architecture-map.html:443`; `GATE-REGISTRY.md:45,79,108,111`
- **Finding:** The stakeholder map says Montagio owns Order/Payment/warranty/claims without qualifying that this is the target public-contractor model only after OQ-10 PASS; the permitted closed test has different actual contract/payment parties.
- **Fix:** Change the card to “Target after OQ-10 PASS” and add: until then, a closed direct-master test records and truthfully presents the actual parties; no public contractor promise/payment is enabled.
- **Risk:** A nontechnical reader can mistake the intended legal model for an already approved operating fact.

## Remaining Low

### R2-L1 — Dependency diagram still uses a contradictory edge

- **Location:** `ARCHITECTURE-SPINE.md:40-49`
- **Finding:** `D -. has no provider dependency .-> P` draws a Domain-to-Port dependency while claiming there is none; AD-2's textual DAG is correct.
- **Fix:** Delete this edge. If a visual prohibition is needed, add a separate note outside the graph or use a clearly styled forbidden edge from Domain to External Provider/Adapter without representing it as an allowed dependency.
- **Risk:** The visual rule and normative prose disagree during dependency disputes.

## Final acceptance condition

Finalize after: (1) schema-valid gate instances exist and the Markdown table is generated or explicitly non-authoritative; (2) frontmatter returns to `draft` then `final`; (3) UJ-A1 slice labels preserve the canonical outcome; (4) Montagio ownership is qualified by OQ-10; and (5) the contradictory diagram edge is removed. No architectural redesign is required.
