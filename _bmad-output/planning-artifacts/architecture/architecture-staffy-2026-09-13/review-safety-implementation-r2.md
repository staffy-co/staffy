---
title: Staffy Architecture — Targeted Safety, Operations and Implementation Re-review R2
reviewed: 2026-09-15
reviewer_lens: closure of R1 findings plus regression scan
artifacts:
  - ARCHITECTURE-SPINE.md
  - GATE-REGISTRY.md
  - gate-instance.schema.json
  - architecture-map.html
  - review-safety-implementation.md
decision: changes-required
---

# Targeted Safety / Operations / Implementation Re-review R2

## Verdict

All **15 R1 findings are substantively resolved** and no architecture redesign is required. The revised Spine is materially safer and remains feasible for a Go V1.

The regression scan found **1 new High and 3 new Medium findings**. They are localized specification fixes. Final safety acceptance should wait for the High finding and the machine-gate ambiguity to be corrected.

Current unresolved tally: **0 Critical · 1 High · 3 Medium · 0 Low**.

## R1 closure matrix

| R1 finding | Result | Evidence in current artifacts |
|---|---|---|
| H-1 OQ-11 unsafe fallback | Resolved | Registry lines 15, 27–29, 46 and per-route OQ-11 instances make processor/artifact/purpose gates conjunctive; no-recording is explicitly insufficient. |
| H-2 OQ-13 false green | Resolved | OQ-13A policy and open OQ-13B recovery proof are split in Spine AD-16/Deferred, Registry lines 48–49/89–91 and HTML. |
| H-3 playback treated as heard | Resolved | Spine AD-9 defines four evidence states and denies heard/understood inference; HTML uses playback acknowledged/estimated and customer confirmation. |
| H-4 incomplete RLS contract | Resolved | Spine AD-11 requires FORCE RLS, non-owner/non-superuser/NOBYPASSRLS roles, transaction-local tenant context, separate maintenance principal and negative tests. |
| H-5 stale writer/media output | Resolved | Spine AD-4/AD-6 use owner/control epochs; AD-9 adds a short-lived one-use signed material-playback permit and stale cancellation. Implementation tests must prove the Media Node receives/currently checks revocation before playback and fails closed if that control path is unavailable. |
| H-6 Oleg supplier-send fence | Resolved | Spine line 97, Registry OQ-3 and fixed-fence section, capability map and HTML all preserve separate customer Confirmation, manager receipt and manual supplier send. |
| M-1 append-only versus deletion | Resolved | Spine AD-10 separates retractable/encrypted payload from minimal non-PII ledger and uses tombstone/cryptographic erasure, legal hold and keyed/versioned binding. |
| M-2 Jivo sole-composer ambiguity | Resolved | Registry OQ-2 permits Staffy-only only with isolated/enforced writer exclusion and tested takeover; shared production inbox is human-only until PASS. |
| M-3 unsafe mutation class | Resolved | Spine AD-6 and Registry line 61 classify mutation semantics and prohibit unsafe automated retry. |
| M-4 gates not conjunctive | Resolved | Spine AD-16 and Registry lines 15–29/107–109 require all scoped data, safety, legal and capability gates together. |
| M-5 Gateway compile/network isolation | Resolved | Spine AD-2/AD-6 restrict imports/composition roots, CI dependency checks, read-only core scopes and narrow worker egress. |
| M-6 runtime dependencies | Resolved | AG-4 blocks real credentials until SecretStore, Russia-S3, safe-render/scanner, compatibility, license, SBOM, vulnerability, migration and rollback evidence. |
| M-7 colocated core/worker starvation | Resolved | Spine AD-14 separates supervised processes/containers, resource limits, pools, health checks and reserved priority capacity; AG-1 gates controlled traffic. |
| L-1 HTML overclaims providers | Resolved | HTML labels SIP/Mango, Jivo/ABCP, Novofon and Avito as candidates/manual or subject to account evidence. |
| L-2 Jivo labelled Owning System | Resolved | Topology now says “Channel providers and Owning Systems”. |

## New High finding

### H2-1 — Confirmation consumption is not explicitly atomic with Operation creation

**Where:** Spine AD-6 line 95.
**Risk:** the Confirmation has consumption limits, and `Operation + outbox` is atomic, but the text does not require the Confirmation consumption/reservation itself to occur in that same transaction. Two concurrent requests could both observe remaining allowance, create separate Operations and produce a duplicate order/assignment despite later idempotency controls. This is a direct SM-3/SM-C2 class risk.

**Exact fix:** require one transaction to (a) lock/CAS the exact Confirmation version, (b) verify expiry/revocation/material token and remaining consumption, (c) consume a numbered allowance, (d) reserve the tenant+Owning-System business-intent key, and (e) insert Operation + outbox. Enforce uniqueness such as `(confirmation_id, action_type, material_token, consumption_ordinal)` plus the scoped business-intent key. A conflict returns an existing Operation or a blocked/reconcile result; it never creates a second dispatch.

## New Medium findings

### M2-1 — Machine gate schema permits ambiguous or effectively wildcard PASS scope

**Where:** `gate-instance.schema.json` lines 28–45.
**Risk:** `scope` needs only one property, but every dimension may be `null`. A record such as `{ "tenant_id": null }` validates. The prose says PASS is exact and non-wildcard, but the schema cannot distinguish not-applicable dimensions from omitted/unknown dimensions.

**Exact fix:** prohibit null-valued scope entries or define explicit typed scope profiles per gate definition with required non-empty fields. Reject PASS when any required definition dimension is absent. Treat scope containment as exact typed matching, never null-as-wildcard, and add negative schema/authorization tests.

### M2-2 — Approval separation is prose-only in the machine contract

**Where:** Registry line 17 versus `gate-instance.schema.json` lines 76–89/109–129.
**Risk:** a PASS requires only one arbitrary `approved_by` item; the schema has no accountable-role policy or distinct-principal constraint. Likewise, free-form `permits` and dependency IDs can be mistyped while still validating. This weakens the claim that scoped instances themselves carry executable permission.

**Exact fix:** give each gate definition a machine-readable approval policy and permit enum/registry. Validate required roles, minimum distinct principals, self-approval prohibition where applicable, referenced dependency existence/state/scope and exact permit identifiers outside JSON Schema if necessary. Keep schema validation as syntax, but do not equate it with release admission.

### M2-3 — Known Montagio data edge has no current OQ-11 instance

**Where:** Registry current instances lines 80–86 versus Spine capability map lines 297–298.
**Risk:** the walking skeleton correctly requires “OQ-11 Staffy+Montagio data”, but no scoped instance names Montagio Django, partner/executor transfer or their notification path. The release compiler should fail closed, so this is not currently an unsafe permission; it is an incomplete executable worklist.

**Exact fix:** add scoped OQ-11 instances for Montagio Django/API and each actual partner/executor/contact processor edge once selected, including payload classes, purpose, logs/support/backups and retention/deletion. Keep unknown notification channels absent from the graph and therefore unavailable rather than folding them into the generic Staffy-data instance.

## Non-blocking observation

FR-15/UX require immutable AI-role disclosure and source/purpose disclosure for outbound partner contact. The Spine declares that it binds FR-15 but does not make the disclosure mechanism explicit. During implementation, model it as deterministic session state/core policy outside the model and tenant prompt; employee configuration may select approved wording but must not disable it. This should be included when AD-11 is next edited, but it does not overturn the current architecture boundary.

## Final acceptance condition

After H2-1 and M2-1 are incorporated, this reviewer can give **PASS** without another broad review. M2-2 and M2-3 should be fixed in the same small revision because they define whether the new gate mechanism is genuinely executable. Validate the JSON schema and add negative fixtures for wildcard scope, insufficient approvers, missing dependencies and duplicate Confirmation consumption.

Recommended decision: **targeted changes required; architecture direction accepted.**
