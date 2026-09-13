---
title: 'Staffy PRD — Reviewer Gate Round 1: Safety and Operations'
status: complete
created: '2026-09-13'
reviewed:
  - prd.md
  - addendum.md
---

# Reviewer Gate Round 1 — Safety and Operations

## Verdict

**REVISE BEFORE IMPLEMENTATION READINESS.** PRD has a strong fail-closed product model and unusually good coverage of safety interrupts, typed actions, confirmation, idempotency, handoff, recovery, provider uncertainty and pilot evidence. However, one critical race remains unspecified and five high-severity operational contracts are incomplete. These are bounded amendments, not a redesign and not a reason to introduce new subsystems.

**Severity counts (unique findings):** Critical 1 · High 5 · Medium 7 · Low 0 · Total 13.

The review treated the documents as a strategic/PRD specification for product, UX, architecture, operations and QA readers. Adversarial and edge-path passes were run over the behavior; structure/prose were checked only where wording could produce an unsafe or impossible implementation. Lens overlap is recorded on each finding and not counted twice.

## Findings

### SO-01 — Critical — Mode/owner/control transitions do not fence already queued or in-flight actions

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-13–FR-14, lines 395–411; FR-32, lines 574–581; FR-34, lines 596–603; NFR-1, lines 1151–1153.
- **Trigger condition:** a Safety Interrupt, cancellation, `privacy/no_contact`, human request, Response Owner change or scoped pause occurs after an action/commitment was prepared but before its external side effect is dispatched or acknowledged.
- **Actionable guard:** require every pending response, Commitment and Business Action to be invalidated or re-authorized against the latest Conversation Mode, Response Owner and scoped-control generation immediately before dispatch; unsafe/ineligible work must stop or reconcile rather than continue from an old snapshot.
- **Potential consequence:** Staffy can create an order, send a commercial message, book a master or continue contact after the customer cancelled, requested a human or reported danger.

### SO-02 — High — Authorization, Confirmation and mutable facts have no execute-time freshness rule

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-19, lines 453–460; FR-25, lines 507–514; FR-28, lines 538–545; FR-31, lines 565–572.
- **Trigger condition:** time passes between quote/Confirmation and execution, or permissions, configuration, price, stock, supply, scope, actor authority or order state changes in that interval.
- **Actionable guard:** require an execute-time precondition check of current permission, configuration/policy version, material owning-system facts and Confirmation validity; define which changes expire Confirmation and require a material-delta re-confirmation.
- **Potential consequence:** a correctly confirmed but now stale or unauthorized operation is executed at the wrong price, availability, scope or authority.

### SO-03 — High — Montagio prevents two executors per order, but not one executor being double-booked across orders

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-69, lines 923–930; FR-71, lines 941–948.
- **Trigger condition:** two Orders concurrently select the same executor, zone and time window while each Order independently satisfies “not more than one executor.”
- **Actionable guard:** require atomic capacity reservation across all Orders for the relevant `executor × service/capability × window × capacity` resource, including reservation expiry and release on rejection, timeout, cancellation and reschedule.
- **Potential consequence:** both customers receive a valid named-executor commitment although only one visit can be fulfilled.

### SO-04 — High — Oleg's manager gate is not explicitly bound to the exact order version or enforced at supplier-send

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` UJ-A7, lines 215–219; FR-57–FR-59, lines 811–836; Open Question 3, line 1345.
- **Trigger condition:** an Order line, quantity, price, vehicle/VIN or applicability evidence changes after manager review, or the existing ABCP supplier-order path can be invoked outside the Staffy guard.
- **Actionable guard:** require a human approval receipt bound to the exact Order/Work Item version and evidence set; supplier formation/send must be unavailable until the current version has that approval. The actual ABCP account must prove the guard cannot be bypassed before the first live mutating test.
- **Potential consequence:** an unreviewed or changed part is ordered from the supplier despite the pilot's defining safety gate.

### SO-05 — High — Customer identity and authority are not a cross-cutting requirement for disclosure and actions

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-11, lines 377–384; FR-25, lines 507–514; FR-38, lines 632–639; FR-62, lines 856–863; FR-76, lines 986–993; NFR-18, lines 1227–1229.
- **Trigger condition:** a caller or chat participant requests order data, starts a claim, changes/cancels a booking or confirms a commitment after a channel switch; only the autoparts status path explicitly mentions identity verification and B2B covers role authority separately.
- **Actionable guard:** add a cross-cutting risk-based identity-and-authority policy for data disclosure and material actions, including channel-link proof, step-up verification, delegated actors, failed-verification fallback and a ban on treating Caller ID/display name alone as proof.
- **Potential consequence:** personal/order data is disclosed or a booking/order is changed by an unauthorized participant.

### SO-06 — High — Partial success across multiple owning systems has no explicit recovery contract

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-30–FR-31, lines 556–572; FR-56, lines 802–809; FR-60, lines 838–845; FR-75, lines 977–984; NFR-7, lines 1177–1179.
- **Trigger condition:** ABCP creates the customer Order but the 1C exchange is lost; an executor accepts but the customer commitment fails to deliver; fulfilment is confirmed but financial/document recording fails; a later retry sees only part of the outcome.
- **Actionable guard:** require each multi-system operation to retain per-side-effect states, correlation and monotonic reconciliation; downstream commitments remain blocked on unknown prerequisites, and partial success must lead to a defined completion, compensation or human-resolution route without replaying completed effects.
- **Potential consequence:** orphaned orders, duplicate assignments/fees or a false “completed/sent” status survive reconciliation.

### SO-07 — Medium — Handoff has no path for an acknowledged human who subsequently becomes unavailable

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-35, lines 605–612; FR-37, lines 623–630; FR-41, lines 659–666; SM-18, line 1320.
- **Trigger condition:** the assignee acknowledges Handoff, becomes the Response Owner and then disconnects, leaves shift or misses the promised update.
- **Actionable guard:** define an ownership lease/SLA-breach state or equivalent simple pilot rule that reassigns/escalates or activates fallback after acknowledgement, while preserving one Response Owner and notifying the customer honestly.
- **Potential consequence:** the system reports a successful handoff but the customer is abandoned with no active responder.

### SO-08 — Medium — The autoparts verification task lacks acknowledgement, deadline and fallback ownership

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` UJ-A1, lines 177–183; UJ-A7, lines 215–219; FR-56, lines 802–809; FR-58, lines 820–827.
- **Trigger condition:** the assigned manager never sees or accepts a `pending_manager_verification` order, is off shift or the review exceeds the price/stock validity window.
- **Actionable guard:** specify simple review-task states (`assigned/acknowledged/approved/blocked/expired`), owner/backup, deadline, stale-fact refresh and truthful customer-update fallback. This can reuse the Handoff/exception primitives rather than create another workflow engine.
- **Potential consequence:** confirmed customer orders stall silently or proceed later using stale price, availability or applicability evidence.

### SO-09 — Medium — No explicit recovery path when the named Montagio executor cancels or no-shows after final Confirmation

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` UJ-M1, lines 73–85; FR-69–FR-72, lines 923–957; FR-77, lines 995–1002.
- **Trigger condition:** the executor cancels, becomes unsafe/ineligible or fails to arrive after the customer confirmed that named executor and final terms.
- **Actionable guard:** require invalidation of the current fulfilment Commitment, proactive notice, safe fallback/reassignment, new final Confirmation for a replacement executor/material delta, and Recovery Case activation for a missed promised window where policy requires it.
- **Potential consequence:** an unnamed substitute arrives, the order remains falsely active, or the customer receives no coordinated remedy.

### SO-10 — Medium — Financial closure omits reversal, partial-payment and dispute states

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-75, lines 977–984; SM-7, line 1306; SM-C6, line 1331.
- **Trigger condition:** a customer partially pays, receives a refund, reverses/charges back payment, disputes cash evidence, or the executor's fee becomes uncollectible after initial `fee_collected` evidence.
- **Actionable guard:** extend the required financial outcome model to distinguish due, paid, partially paid, disputed, reversed/refunded and written-off/reconciled states with source and time; contribution margin is provisional until the defined settlement/claim condition.
- **Potential consequence:** Montagio reports positive unit economics or collected commission that was later reversed or never fully reconciled.

### SO-11 — Medium — Data-subject access, correction, withdrawal and deletion are not operationalized

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-40, lines 650–657; NFR-15–NFR-18, lines 1215–1229; Open Question 16, line 1358.
- **Trigger condition:** a person asks what Staffy holds, withdraws consent, requests export/correction/deletion or disputes retention while an audit/legal hold exists.
- **Actionable guard:** require a verifiable privacy-request lifecycle with identity proof, controller/processor owner, scope across provider copies/backups, legal-hold conflict handling, completion evidence and audit. Keep exact statutory times and legal interpretation in the approved Tenant privacy policy.
- **Potential consequence:** Staffy can define retention but cannot safely fulfil or evidence a person's data-rights request.

### SO-12 — Medium — “Protected” media lacks an explicit hostile-file and expiring-access contract

- **Lens:** adversarial + edge-case
- **Location:** `prd.md` FR-16, lines 422–429; FR-55, lines 793–800; FR-82, lines 1048–1056; NFR-18–NFR-20, lines 1227–1237.
- **Trigger condition:** a customer uploads a malicious/oversized/mislabelled document or a previously shared VIN/photo link remains usable after the Case, role or consent changes.
- **Actionable guard:** require content/type/size validation, quarantine or safe rendering before human/tool access, scoped expiring and revocable links, download audit and retention-linked deletion. NFR-20's prompt-instruction guard alone does not cover file-host threats.
- **Potential consequence:** operator/device compromise or persistent unauthorized access to VIN, documents, addresses and site photos.

### SO-13 — Medium — The evidence-gate sequence is worded inconsistently

- **Lens:** structure + prose + edge-case
- **Location:** `prd.md` MVP In Scope, line 1120; SM-17, line 1319.
- **Trigger condition:** “до live/scale” in MVP scope is read as requiring the paid real-order micro-cohort before live, while SM-17 correctly requires drills/tabletop before live and the paid micro-cohort before scale.
- **Actionable guard:** split the sentence into two explicit gates: supply drill and Stage-0 legal/payment tabletop before live; manually reconciled paid micro-cohort before scale.
- **Potential consequence:** teams either create an impossible pre-live dependency or start live without the intended drills.

## Top findings for the gate decision

1. **SO-01:** fence queued/in-flight work on safety, cancellation, privacy, human takeover and scoped pause.
2. **SO-02:** revalidate authorization, Confirmation and mutable facts immediately before side effects.
3. **SO-03:** reserve executor capacity atomically across Orders, not merely one executor within each Order.
4. **SO-04:** bind Oleg's manager approval to the exact Order version and prove supplier-send cannot bypass it.
5. **SO-05:** apply risk-based customer identity/authority checks consistently across both domains and channels.

## What is already adequately specified

No finding is raised against the following controls: fail-closed unknown outcomes; idempotency keys and conflict escalation; no false success after connector timeout; durable Action Receipts; one writable Inbox/Response Owner; P7 priority over sales; bounded recovery authority; human-only fault/refund/legal conclusions; supply-readiness before promise; two customer Commitments; stop-work for additional scope; separated Montagio fulfilment/payment/fee/claim evidence; Oleg's pilot prohibition on Staffy supplier-send; provider/account assumptions and owner/revisit gates; Russian data perimeter, artifact-level data map, retention, secret isolation, redaction and prompt-injection boundary; baseline-derived thresholds and zero-tolerance critical outcomes.

The Addendum correctly keeps Go/media/provider/component choices out of the product requirements. The fixes above should state required behavior in the PRD; exact locks, leases, transaction boundaries, storage and adapter mechanisms belong to Architecture.
