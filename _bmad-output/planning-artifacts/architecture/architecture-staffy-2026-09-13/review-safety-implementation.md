---
title: Staffy Architecture — Independent Safety, Operations and Implementation Review
reviewed: 2026-09-15
reviewer_lens: tenant isolation, authority, privacy, reliability, pilot boundaries, Go V1 feasibility
artifacts:
  - ARCHITECTURE-SPINE.md
  - GATE-REGISTRY.md
  - architecture-map.html
  - .memlog.md
  - ../../prds/prd-staffy-2026-09-12/prd.md
  - ../../ux-designs/ux-staffy-2026-09-13/EXPERIENCE.md
decision: changes-required
---

# Independent Safety / Operations / Implementation Review

## Verdict

The architecture is coherent and implementable as a Go V1: the modular-monolith boundary, typed Planner, Action Gateway, transactional outbox, explicit Owning Systems and route-scoped gates are sound. It is **not yet safe to mark final** because several claims are stronger than the mechanisms currently specified.

Severity count: **0 Critical · 6 High · 7 Medium · 2 Low**.

## High findings

### H-1 — OQ-11 fallback can still send PII to an unapproved processor

**Where:** Gate Registry line 26; Spine AD-11/AD-16; HTML gate summary.
**Risk:** “no-recording/text/human alternative” is not automatically safe: text, live STT, prompts, tool payloads, support traces and provider backups may still process PII outside the approved artifact route. This weakens PRD NFR-14–NFR-17.

**Exact fix:** make OQ-11 conjunctive per `provider × route × artifact class × purpose`. Before PASS, allow only synthetic controlled data or an independently approved path that does not call the unapproved processor. State explicitly that disabling recording does not authorize STT/model/text processing. Consent/legal basis must be checked before capture/transfer, and withdrawal must stop the affected capture/queued transfer through a purpose-scoped generation while preserving legally required records.

### H-2 — OQ-13 is green before recoverability is proven

**Where:** Gate Registry line 28 and execution order line 41; Spine AD-16 and Deferred line 278; HTML line 483.
**Risk:** `PASS for pilots` visually and normatively implies readiness although restore evidence is still required before live. The 15-minute PostgreSQL RPO, 4-hour recovery and 2-hour media replacement are objectives, not demonstrated capabilities.

**Exact fix:** split the concern: `OQ-13A service-level policy = PASS (best effort, no SLA)` and `OQ-13B recoverability = OPEN until restore/rebind/reconcile and media-replacement drills pass`. Bind live admission to 13B. Change the HTML badge to “policy chosen; recovery proof open”. Record measured restore duration/data point and tested backup/PITR scope before claiming RTO/RPO achieved.

### H-3 — “heard” is asserted from a playback boundary

**Where:** Spine AD-9 line 102; HTML truth card line 444; UX correctly distinguishes this in NFR-31.
**Risk:** Staffy can usually prove generated/sent/playback-acknowledged or estimated audio, not that a person heard and understood a material condition. Treating playback as heard can fabricate a Commitment or Confirmation after barge-in, silence or disconnect.

**Exact fix:** replace “actually played/heard content is committed” with explicit states `generated`, `sent_to_provider`, `playback_acknowledged|estimated`, and `customer_confirmed`. Never infer “heard/understood/confirmed” from playback. After interruption or reconnect, require semantic repair/readback before using the condition as confirmed. Update the HTML provider card accordingly.

### H-4 — RLS is a principle, not yet a mechanically safe PostgreSQL contract

**Where:** Spine AD-11 line 114 and database convention line 151.
**Risk:** PostgreSQL table owners, superusers and `BYPASSRLS` roles can bypass ordinary RLS; pooled connections can leak stale tenant session state. A shared schema makes this the primary cross-tenant blast-radius control.

**Exact fix:** require tenant tables to use `ENABLE ROW LEVEL SECURITY` + `FORCE ROW LEVEL SECURITY`; runtime roles must be non-owner, non-superuser and `NOBYPASSRLS`. Set verified tenant context with `SET LOCAL` only inside a transaction, fail before SQL when absent, and never trust request/job payload tenant IDs. Add migration checks plus negative integration tests for cross-tenant reads/writes, foreign-key linkage, prepared statements, pooled-connection reuse, worker jobs, support access and backup/restore. A privileged maintenance path must be a separate audited principal, not an application switch.

### H-5 — Response ownership and media output lack an execute-time writer fence

**Where:** Spine AD-4/AD-6/AD-9; topology gives Media Node a direct speech route.
**Risk:** an answer queued before handoff/pause can be sent after ownership changes; a stale or compromised Media Node can play material speech without proving current owner/mode/control generation. “One owner” in storage does not prevent two writers at dispatch.

**Exact fix:** every outbound text and media frame must bind `tenant_id`, `case_id`, `turn_id`, `response_owner_epoch`, `control_generation`, `release_manifest_id`, content hash/materiality, deadline and idempotency key. Revalidate these immediately before provider send/playback; stale epochs fail closed. Define requested-handoff interim behavior: safety/privacy/claim/human-request freezes selling and mutations immediately, while owner changes only on acknowledgement. Use CAS/unique constraints for ownership transitions and test delayed jobs, double acknowledgement, return and barge-in races.

### H-6 — Oleg’s human supplier-send fence is not a durable architecture invariant

**Where:** PRD FR-57 requires it; Gate Registry OQ-3 only covers capability/status discovery; Spine does not state the pilot restriction.
**Risk:** passing OQ-3 could be misread as permission for Staffy to send an order to a supplier, contradicting the accepted pilot algorithm. Customer Confirmation and manager approval could be collapsed into one generic Confirmation.

**Exact fix:** add an explicit Autoparts pilot rule: Staffy may create only the exact-version ABCP client/prepared order after customer Confirmation; `supplier_form/send` is absent from the AI/Staffy Action Catalog for the controlled class. A named manager must reread the current ABCP version and applicability evidence, acknowledge, approve/block and produce a separate human receipt. Any VIN/vehicle/item/quantity/price/fitment change invalidates both approval and affected customer Confirmation. Lifting this fence requires a new scoped autonomy decision and Release Manifest, not merely OQ-3 PASS.

## Medium findings

### M-1 — Append-only evidence conflicts with retention/deletion unless payload handling is defined

**Where:** Spine AD-10 versus AD-11 and PRD NFR-16.
**Exact fix:** state that append-only means immutable logical/audit transitions, not indefinite PII retention. Separate encrypted/retractable payloads from minimal non-PII ledger metadata; retention expiry or approved deletion tombstones/crypto-shreds payload and provider copies, records completion evidence, and respects legal hold. Define which hashes are safe to retain and prevent dictionary recovery of low-entropy PII.

### M-2 — Jivo “Staffy as sole composer” is not safe without physical writer exclusion

**Where:** Gate Registry OQ-2 safe state.
**Exact fix:** until writer exclusion is proven, production shared-inbox mode is human-only. A Staffy-only mode is permitted only in an isolated account/inbox or with enforceable removal of human send rights and a tested emergency takeover. Operational instruction alone is not evidence.

### M-3 — Providers without idempotency/reconciliation need an explicit fail-closed class

**Where:** Spine AD-5/AD-6/AD-13 and OQ-19.
**Exact fix:** classify each mutation as: provider-idempotent; query-reconcilable; compensatable; or unsafe. If neither idempotency nor authoritative reconciliation exists, automated retry is prohibited and the action is `assisted/human_only` (or one-shot followed by mandatory human reconciliation where explicitly accepted). Encode this in Capability Evidence and contract tests.

### M-4 — Applicable gates are not explicitly conjunctive

**Where:** Gate Registry execution order.
**Exact fix:** add a global rule that a route is admitted only when **all** applicable gates pass simultaneously. OQ-1–OQ-5 controlled tests before OQ-11 must use synthetic/minimized test data and approved test accounts; no ordering statement may override privacy, safety, identity or legal gates.

### M-5 — Action Gateway isolation needs compile-time and network enforcement

**Where:** Spine AD-2/AD-6 and topology.
**Exact fix:** make mutation adapter packages importable only from the action dispatch composition root; add dependency-graph tests/lint. Give `staffy-core` read-only credentials/scopes and egress, and `staffy-worker` narrow destination allowlists per Connector. Where a provider combines read/write credentials, document the residual risk and require audit/rate/operation binding; never claim credential separation exists when the provider cannot supply it.

### M-6 — Go V1 operational dependencies need an admission checklist

**Where:** Stack section and unspecified SecretStore/S3/scanner implementations.
**Exact fix:** before credential-bearing development, select and test concrete SecretStore, Russia-S3 and malware/safe-render implementations. Add a pinned compatibility test for Go 1.27 + PostgreSQL 18 + pgx/sqlc/goose + pre-1.0 River, plus license review, SBOM, vulnerability scanning and rollback/migration tests. River remains replaceable; its schema/state must not leak into domain APIs.

### M-7 — Single Application Node needs resource isolation to protect the control plane

**Where:** Spine AD-14 and topology lines 219–225.
**Exact fix:** run core and worker as separate supervised processes/containers with independent CPU/memory/file-descriptor limits, database pools and health checks even when colocated. Reserve capacity for ingress, safety controls, handoff and reconciliation; shed eval/sync first. Prove the Pilot Load Envelope from PRD NFR-13 before controlled traffic.

## Low findings

### L-1 — HTML presents unverified integrations as established capabilities

**Where:** HTML lines 331, 374–375 and 481.
**Exact fix:** label Mango, Jivo, ABCP/Laximo traversal and Novofon/Yandex as pilot candidates or `awaiting account/route evidence`; keep the architecture promise, but do not visually imply connector availability before OQ PASS.

### L-2 — Topology labels Jivo as an Owning System

**Where:** Spine diagram line 225 (`ABCP, Montagio, Jivo and other Owning Systems`).
**Exact fix:** rename the node to “channel providers and Owning Systems”, or split outbound channel delivery from business-system mutations. This prevents implementers from confusing delivery evidence with domain truth.

## Required re-review evidence

Before final acceptance, verify that:

1. all six High findings are reflected in the normative Spine/Gate Registry and the HTML no longer overclaims them;
2. route admission tests prove conjunctive gates, owner/control epochs and RLS isolation;
3. Oleg’s supplier-send restriction and Montagio OQ-10 legal/data-sharing boundary are executable policies, not only prose;
4. the restore gate is open until measured drill evidence exists;
5. generated diagrams and prose use `playback-acknowledged/estimated`, never “heard”, without customer evidence.

Recommended decision: **changes required, then targeted safety re-review; no architecture redesign required.**
