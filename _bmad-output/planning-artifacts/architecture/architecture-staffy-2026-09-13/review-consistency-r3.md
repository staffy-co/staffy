---
title: Staffy Architecture Consistency Review R3
status: review-complete
reviewed: 2026-09-15
---

# Architecture consistency review — R3

## Verdict

**Two narrow material blockers remain; not yet final PASS.** All R2 architecture/content fixes are resolved, and the machine registries are structurally sound. No architectural redesign is required.

Verified:

- `gate-definitions.json`: 25 unique definitions; schema-valid; exactly covers OQ-1–OQ-19 with OQ-7A/B and OQ-13A/B plus AG-1–AG-4.
- `gate-instances.json`: 35 unique instances; schema-valid; every definition/dependency reference exists; dependency graph is acyclic.
- Markdown instance projection: exactly the same 35 IDs, definition IDs and states.
- The only PASS, `oq-13a-montagio`, contains both required roles, valid timestamp/permission/history, an existing evidence artifact and a matching SHA-256.
- Non-PASS instances grant no permissions; scopes contain no null/wildcard values.
- Spine is correctly `draft`; UJ-A1 slices, Montagio/OQ-10 qualification and dependency diagram are fixed.

## Blockers

### R3-1 — Separation-of-duties prose contradicts machine definitions

- **Location:** `GATE-REGISTRY.md:19`; `gate-definitions.json` definitions OQ-7A, OQ-7B, OQ-9 and OQ-13A; PASS instance `oq-13a-montagio`
- **Finding:** Markdown says separation of duties applies whenever a definition names more than one role. The authoritative definitions explicitly set `allow_role_coincidence: true` for four multi-role gates, and the current OQ-13A PASS validly uses the founder for both roles.
- **Fix:** Change the contract sentence to: separation of duties applies when more than one role is required **unless** the definition explicitly sets `allow_role_coincidence: true`; in that case one principal must demonstrably hold every recorded role. Keep the current OQ-13A instance unchanged.
- **Risk:** A valid machine PASS is invalid under the human normative contract, so reviewers and runtime can reach opposite admission decisions.

### R3-2 — Human deadline projection understates two machine blocks

- **Location:** `GATE-REGISTRY.md:49-51,111`; corresponding `gate-definitions.json` entries OQ-12 and OQ-13B
- **Finding:** The human table says OQ-12 and OQ-13B block `live`, while the authoritative definitions and execution order also block `controlled_real_customer` voice. The table is declared a projection, so it must not show a weaker deadline.
- **Fix:** Change OQ-12 to “controlled real-customer voice and live/scale voice admission” and OQ-13B to “controlled real-customer voice and live/scale admission,” matching `blocks_stages` and execution order.
- **Risk:** Operators following the human registry may start controlled real-customer voice before the machine gate permits it.

After these two text-only corrections, this consistency lens can return PASS.
