---
title: Staffy Architecture Consistency Review
status: review-complete
reviewed: 2026-09-15
targets:
  - ARCHITECTURE-SPINE.md
  - GATE-REGISTRY.md
  - architecture-map.html
---

# Architecture consistency review

## Verdict

**Revise before finalization.** The spine is compact, structurally compliant and broadly faithful to the PRD/UX/memlog, but it is not yet a safe durable build substrate: the gate registry is not executable under its own contract, one mandatory capacity boundary is absent, module dependency/transaction direction is underspecified, and two unverified providers are presented as pilot facts.

Severity counts: **Critical 0 · High 8 · Medium 9 · Low 2** (19 findings).

Deterministic evidence:

- `lint_spine.py`: PASS, 0 findings.
- All six frontmatter source paths resolve.
- Template comments/placeholders and duplicate AD IDs: none.
- Spine size: 2,600 words; the strategic/pyramid structure is appropriate and does not require general condensation.

## High

### H1 — Gate registry cannot implement its declared lifecycle

- **Location:** `GATE-REGISTRY.md:12-34`
- **Finding:** The contract declares one finite state and mandatory PASS metadata, but status cells contain compound/non-enum values (`Open, critical`, `Deferred/open`, dual PASS/open) and the schema has no fields for evidence links, tested time, expiry/retest, approver or permitted decision.
- **Fix:** Use exact `open|in_progress|evidence_ready|pass|fail|expired`; move qualifiers to separate fields; add `scope`, `evidence_links`, `tested_at`, `expires_at_or_trigger`, `approved_by`, `permits` and `criticality`. Represent platform rules separately from scoped gate instances.
- **Risk:** Runtime/release automation can treat stale, partial or differently scoped evidence as permission.

### H2 — OQ-13 reports PASS before recovery readiness is proven

- **Location:** `GATE-REGISTRY.md:28,41`; `ARCHITECTURE-SPINE.md:144,278`
- **Finding:** OQ-13 is PASS while its own row says restore evidence is still required before live, and the PASS lacks the metadata required by the registry contract.
- **Fix:** Split `availability_tier_selected` (PASS: best effort) from `pilot_restore_readiness` (OPEN until a dated restore drill). Make the latter block live admission; reserve reopening the former for a commercial SLA.
- **Risk:** A live pilot can be admitted with recovery objectives that have never been demonstrated.

### H3 — OQ-7 creates a stage-ordering deadlock

- **Location:** `GATE-REGISTRY.md:22,42`; `prd.md:1087-1095,1284-1286,1430`
- **Finding:** The row says OQ-7 blocks Operating Profile approval, but execution order defers it until after controlled baseline; its evidence mixes pre-registered cohort allocation with thresholds that can exist only after baseline.
- **Fix:** Split into a pre-controlled `cohort_plan` gate and a post-baseline `route_thresholds` gate. The first permits controlled work; the second permits live/scale promotion.
- **Risk:** Teams either cannot start controlled work or start without the pre-registered denominator needed for valid evidence.

### H4 — Mandatory Pilot Load Envelope is absent

- **Location:** `ARCHITECTURE-SPINE.md:122-132,272-282`; `GATE-REGISTRY.md`; source `addendum.md:90-96`, `prd.md:1280-1286`
- **Finding:** NFR-13 requires a pre-controlled voice/text load envelope, test at a safety factor and explicit admission degradation, but no AD, convention, gate, owner or evidence record covers it.
- **Fix:** Add a pre-controlled capacity gate binding per-Tenant/total concurrency, burst/duration, codec/bitrate, tool fan-out/timeouts, queue/Handoff limits, provider limits, storage/retention, cost cap, safety factor and over-limit behavior. Bind it from AD-14 and the capability map.
- **Risk:** Tail latency, duplicate actions and Handoff starvation can appear inside a supposedly controlled pilot.

### H5 — Module dependency and transaction direction is not enforceable

- **Location:** `ARCHITECTURE-SPINE.md:35-44,56-60,187-210`; `.memlog.md:72-73`
- **Finding:** The document prohibits private-table access but provides no module import DAG, composition boundary, cross-module transaction ownership or rule for command/API versus outbox/event coordination. Global `adapters`, `jobs` and `migrations` directories make violations easy.
- **Fix:** Add an explicit allowed dependency graph and one orchestration rule: which module owns a transaction, when a consumer-owned port adapts another module API, and when durable domain events are required. Seed per-module `api/ports/internal persistence` ownership or equivalent CI-enforceable boundaries, plus import and SQL-ownership checks.
- **Risk:** The modular monolith can accumulate cycles, shared transactions and cross-module SQL that later cannot be separated safely.

### H6 — `Work Item` disappeared from the Case model

- **Location:** `ARCHITECTURE-SPINE.md:68-72,231-253,261`; sources `prd.md:402-410,936-944`
- **Finding:** FR-12/FR-65 require multiple independent Work Items inside one Case, but the lifecycle, ERD and capability map flatten all intent into Case/Conversation.
- **Fix:** Declare `case` as owner of `Work Item`; connect facts, Commitments, waits and relevant Actions to a Work Item; define the safe behavior for ambiguous topic switches and parallel/pause/resume states.
- **Risk:** Multi-topic conversations can overwrite facts or bind a Confirmation/Action to the wrong customer intent.

### H7 — Connector contract omits required operation classes

- **Location:** `ARCHITECTURE-SPINE.md:74-78,86-90`; source `addendum.md:92-93`, `prd.md:326-378`
- **Finding:** The minimum protocol has Channel Event, Capability Evidence, Action Request and Receipt, but the binding source requires typed query/read, cancel and compensate semantics (including unsupported behavior) for changing capabilities.
- **Fix:** Extend the process-neutral contract with typed Read Request/Observation and optional-but-explicit Cancel/Compensation Request/Receipt contracts, or state how each is represented by the Action protocol. Include timeout, ordering, limits and unsupported-state behavior.
- **Risk:** Reads and recovery become provider-specific escape hatches outside common authority, evidence and reconciliation rules.

### H8 — Avito is both a pilot dependency and deferred

- **Location:** `ARCHITECTURE-SPINE.md:268,277`; `architecture-map.html:380`; source `addendum.md:102`, `prd.md:1193-1196`
- **Finding:** The pilot map names an Avito adapter, while OQ-18 and the MVP cut defer production Avito pending official account/legal/capability evidence.
- **Fix:** Remove Avito from the pilot substrate and label UJ-M3 as a contract/eval journey using assisted/manual or protected continuation until an independently scoped OQ-18 gate passes.
- **Risk:** Engineering or stakeholders can treat an unverified channel as committed MVP scope.

## Medium

### M1 — Mango is presented as confirmed

- **Location:** `ARCHITECTURE-SPINE.md:217,267`; `architecture-map.html:331,374`; source `addendum.md:16,84`; `GATE-REGISTRY.md:16`
- **Finding:** Topology and pilot labels say Mango although OQ-1 still treats Oleg's actual provider as owner-reported and unverified.
- **Fix:** Use `verified Oleg SIP provider (Mango candidate)` everywhere outside the OQ-1 record; instantiate a Mango adapter only after PASS.
- **Risk:** The pilot couples to the wrong carrier or an assumption becomes a partner promise.

### M2 — OQ-11 fallback can still leak PII

- **Location:** `GATE-REGISTRY.md:26`
- **Finding:** OQ-11 blocks production PII, but its fallback allows generic text/human processing without saying that those processors and artifacts must independently pass the same perimeter gate.
- **Fix:** Permit fallback only inside an independently verified OQ-11 scope; otherwise limit it to non-PII intake or no processing.
- **Risk:** Personal data can leave the approved perimeter through the nominal fallback.

### M3 — Playback evidence is conflated with human hearing

- **Location:** `ARCHITECTURE-SPINE.md:98-102`; source `prd.md:1364-1366`, `EXPERIENCE.md:188-209`
- **Finding:** “actually played/heard” collapses measurable playback with unprovable human hearing/understanding.
- **Fix:** Preserve separate states `generated`, `sent_to_provider`, `playback_acknowledged_or_estimated` and `customer_confirmed`; after interruption require semantic repair/readback for material terms.
- **Risk:** Interrupted conditions can be treated as delivered commitments or consent.

### M4 — Minimal Work Record has no module owner

- **Location:** `ARCHITECTURE-SPINE.md:46,62-66,231-253,255-270`; `.memlog.md:21,72-75`
- **Finding:** The spine insists on six modules but allows a Work Record without assigning its state, API or lifecycle to one of them; earlier notes alternately call it a module and a Case-owned record.
- **Fix:** Keep six modules and explicitly make the minimal Work Record a `case` aggregate/surface (or consciously add a seventh module), then add it to the ERD and capability map.
- **Risk:** Implementers can create a hidden CRM or competing work/order truth.

### M5 — Delivery slices are missing from the build substrate

- **Location:** `ARCHITECTURE-SPINE.md:17-26,187-210`; source `addendum.md:90-96`, `prd.md:1205-1209`
- **Finding:** `delivery-slices.md` is a mandated Architecture input but is not a source and the two-path walking skeleton is not mapped to modules/contracts/tests/manual controls/gates.
- **Fix:** Add the source and a compact walking-skeleton row for UJ-A1 and the farthest-safe UJ-M1/M2 slice, with exit gates; do not expand this into a delivery plan.
- **Risk:** A coherent repository can still build the wrong first vertical slices.

### M6 — Binding accessibility admission was dropped

- **Location:** `ARCHITECTURE-SPINE.md:110-120,161,269`; source `EXPERIENCE.md:224-239`
- **Finding:** The spine generically binds accessibility NFRs but does not preserve the adopted WCAG 2.2 AA Evidence Gate that blocks affected release scope.
- **Fix:** Add accessibility evidence to AD-12 release admission and the testing convention: automated plus manual keyboard, screen-reader, reflow/text-spacing, forced-colors, touch, reduced-motion and alternative-path checks.
- **Risk:** A release can pass architecture gates while failing the binding UX admission contract.

### M7 — SM-4 has no executable proof gate

- **Location:** `ARCHITECTURE-SPINE.md:118,255-270`; `architecture-map.html:496-497`; source `prd.md:1376-1379`
- **Finding:** The spine binds SM-4 but never defines the third-external-Tenant, real-input, one-working-day, no-programmer/no-core-change proof as a gate.
- **Fix:** Add a platform-reuse admission record with timer boundaries, qualifying-Tenant rules, code-diff check, participant log, supported Pack/Connector evidence and controlled exercise receipt.
- **Risk:** Universality can be declared without satisfying the PRD's actual product proof.

### M8 — Stack contains floating versions despite exact-pin policy

- **Location:** `ARCHITECTURE-SPINE.md:175,183-185`
- **Finding:** `Protobuf current compatible toolchain` and `Node.js 24.21.x` are not exact versions, contradicting both the template and the next sentence.
- **Fix:** Split gRPC-Go, protobuf runtime and protoc/tool generators into individually pinned rows; pin Node to the verified exact patch or explicitly mark it deferred until bootstrap.
- **Risk:** Two builders can generate incompatible contracts or non-reproducible browser-test environments.

### M9 — HTML gate summary omits live open gates

- **Location:** `architecture-map.html:476-489`; `GATE-REGISTRY.md:21-34`
- **Finding:** The standalone stakeholder map omits OQ-6, OQ-7, OQ-14 and OQ-15 and visually removes OQ-19 from the status cards.
- **Fix:** Show all gate groups, including commercial, cohort, quantitative and per-Connector authority gates, or provide an explicit “other open gates” card with their consequences.
- **Risk:** A nontechnical reader can infer more pilot, scale or commercial readiness than the registry permits.

## Low

### L1 — Paradigm diagram contains a meaningless self-loop

- **Location:** `ARCHITECTURE-SPINE.md:35-44`
- **Finding:** `D -. no provider dependency .-> D` does not express a prohibited Domain-to-provider dependency and weakens the only dependency diagram.
- **Fix:** Remove the self-loop and draw only permitted arrows; accompany the diagram with an explicit forbidden-import statement or CI rule.
- **Risk:** Readers cannot use the diagram to resolve dependency disputes.

### L2 — Frontmatter coverage ranges are hard to validate

- **Location:** `ARCHITECTURE-SPINE.md:11-16`
- **Finding:** Hyphenated scalar ranges such as `UJ-P1-UJ-P2` are ambiguous to tooling, and SM coverage appears only inside AD rows.
- **Fix:** Normalize range notation (`UJ-P1..UJ-P2`) or use explicit/generated arrays; add SM-1..SM-20/counter-metrics to frontmatter or a machine-checkable coverage index.
- **Risk:** Automated completeness checks can accept decorative range claims while missing requirements.

## Resolution order

1. Fix H1-H4 before calling the registry executable or admitting controlled/live work.
2. Fix H5-H7 before epics/stories or Connector implementation.
3. Reconcile H8 and M1 in all three artifacts so assumptions do not appear as scope.
4. Apply M2-M9 during the same finalization pass; L1-L2 are safe mechanical cleanup.

No change is recommended to the selected Go modular-monolith paradigm, six primary business-module strategy, Action Gateway, hybrid persistence, AI authority boundary, two-node pilot topology or broad technology choices.
