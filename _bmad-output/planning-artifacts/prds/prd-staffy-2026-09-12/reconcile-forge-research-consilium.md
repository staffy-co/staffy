---
title: 'Staffy PRD — Reconciliation of Forge, Research and Consilium Inputs'
status: complete
created: '2026-09-13'
---

# Reconciliation: Forge + Research + Consilium → PRD/Addendum

## 1. Scope and decision rule

This report compares the following input cluster with the current `prd.md` and `addendum.md` without editing either document:

- `../../../forge/staffy-vertical-voice-agent/forged-idea.md`;
- `../../research/competitive-staffy-2026-09-11/research.md`;
- `../../research/technical-staffy-voice-architecture-2026-09-11/research.md`;
- `consilium/synthesis.md` and the five accepted elicitation artifacts;
- the current `prd.md` and `addendum.md`.

The reconciliation rule is strict:

1. a **material accepted gap** is an explicitly fixed or owner-accepted product decision that is absent or not testable in the PRD;
2. technical implementation choices stay in the Addendum/Architecture workflow, even if they explain why a product requirement is feasible;
3. recommendations and vendor/market claims without owner acceptance or live evidence do not become requirements;
4. earlier decisions replaced by later owner confirmation, research correction or Delphi convergence are marked superseded rather than restored.

## 2. Overall verdict

The PRD has strong coverage of the central accepted direction. It preserves the universal multi-tenant product identity, two parallel reference domain packs, one employee identity across channels, connector/capability configuration, safe typed actions, owning-system facts, manager-gated autoparts orders, Montagio policy outcomes, supply readiness, two commitments, human handoff/recovery, versioned autonomy and third-tenant reuse proof.

The consilium's Must-now product controls are substantially represented in FR-14–FR-48 and FR-64–FR-77. No domain-level redesign is required. Four accepted gaps remain: two platform/commercial gaps from Forge, and two evidence/operations gaps accepted through the consilium.

## 3. Material accepted gaps

### G1 — The Staffy commercial product is not specified as a testable offer

**Accepted input.** The Forge fixes the first commercial package as a managed AI employee with assisted setup: launch/integration fee, subscription and actual voice/AI usage. A new reusable connector is estimated separately but becomes a shared Staffy asset. It also states that a tenant may have several AI-employee roles. Competitive research independently supports charging a premium for implementation and responsibility, while explicitly warning that exact willingness to pay is unverified.

**Current coverage.** The PRD specifies assisted setup, reusable connectors, one identity across channels, cost accounting and the third-tenant setup test (`FR-3`, `FR-7`, `FR-25`, `FR-49`, `NFR-25–26`, `SM-4`). It does not define:

- the commercial unit as an AI employee/role rather than a channel session;
- that adding a channel to the same employee does not create another employee;
- the accepted launch + subscription + usage packaging;
- the separate commercial treatment and shared ownership/reuse rule for a new connector;
- a paid Staffy-pilot or equivalent willingness-to-pay gate.

**Why material.** The PRD can prove technical reuse and the economics of the client's orders while still failing to prove that Staffy itself is a sellable repeatable service. This also leaves entitlement and usage presentation ambiguous for UX and later architecture.

**Recommended PRD-level correction.** Add one platform commercial/entitlement requirement and one validation outcome. Keep actual prices, invoice implementation and billing-provider choice out of the PRD. The paid-pilot threshold from competitive research remains a proposed evidence gate until the owner explicitly chooses it; the accepted packaging itself should be recorded now.

### G2 — No positive operator-inbox fallback for a client without a suitable inbox

**Accepted input.** The Forge says a supported client inbox is preserved and, when none exists, an operator surface is offered by Staffy; replacing an existing suitable inbox is prohibited. The technical research narrows the implementation hypothesis: Jivo for Oleg, a bounded Chatwoot spike for clients without an inbox, and a custom Staffy Inbox only if external options block product value.

**Current coverage.** `FR-13` requires one Response Owner and one writable Inbox; `FR-61` fixes Jivo for Oleg; the Non-Goals reject replacing a suitable client inbox. The PRD does not positively require an operator/handoff surface when the tenant has no compatible inbox, nor does it state the minimum product capabilities of that fallback.

**Why material.** A tenant without Jivo or another supported helpdesk cannot complete acknowledged handoff, recovery and human ownership, even if all other connectors exist. This undercuts the platform's easy-onboarding promise and the third-client reuse test.

**Recommended PRD-level correction.** Add a provider-neutral requirement: every production tenant must have one tested human operator surface, either a supported existing inbox or a Staffy-provided option, with assignment, acknowledgement, response ownership, transcript/context access and recovery continuation. Chatwoot versus custom Staffy Inbox remains an Addendum/Architecture decision and is not an MVP commitment until the spike passes.

### G3 — Three accepted pre-launch evidence exercises are absent

**Accepted input.** The owner accepted the Assumption Audit as a whole. Its proposed pre-launch evidence set is: a supply drill, a Stage-0 legal/payment tabletop and a manually reconciled paid-order micro-cohort (`elicitation-01-assumption-audit.md`, items 7–8). The Addendum records the audit as accepted and the PRD already carries most of its other consequences.

**Current coverage.** The PRD contains controlled and live cohorts (`FR-47`, `SM-1`, `SM-2`), supply readiness (`FR-69`), legal/payment open questions and business-close reconciliation (`FR-75`). It does not require those three exercises to occur before the live/scale transition, nor name the evidence each must produce.

**Why material.** General controlled dialogues can pass while the real supply pool cannot accept work, the payment/legal roles remain contradictory or commission reconciliation fails. These are the primary business-model risks of Montagio, not implementation detail.

**Recommended PRD-level correction.** Add the three exercises as explicit pre-live/pre-scale evidence gates, with artifacts and named approvers but without invented numeric thresholds. A manually operated exercise is acceptable; no new subsystem is implied.

### G4 — Accepted recovery quality is required functionally but not measured sufficiently

**Accepted input.** The Customer Support Theater's accepted P1 set requires recovery measurements: acknowledgement time, time to named owner, update-promise adherence, time to safe containment, resolution time, reopen rate, customer acceptance/contest, compensation cost and recurrence by root-cause version. The owner also accepted incident linkage and scoped cohort freeze.

**Current coverage.** `FR-35–FR-41` cover acknowledged handoff, linked Recovery Case, bounded authority, customer contest and truthful updates. `SM-11` measures handoff acceptance and repeated context, while `SM-C10` counts trust complaints. The PRD lacks an explicit recovery metric family covering containment, promised updates, resolution/reopen and recurrence by version.

**Why material.** The current functional controls could exist while prolonged or repeatedly reopened recovery remains invisible to the promotion/rollback decision.

**Recommended PRD-level correction.** Add recovery KPIs or counter-metrics tied to version, root cause and owner. As elsewhere in the PRD, collect baseline first and set scale thresholds later.

## 4. Architecture-how: retain only in Addendum/Architecture

These inputs are useful and largely accepted as current technical direction, but should not be copied into product requirements as implementation mandates:

1. **Runtime shape:** Go as the primary owned core/runtime language; in-process Action Gateway; one append-only mutations table; immutable `runtime_config_hash`; no premature microservices, distributed leases or universal workflow engine.
2. **Voice/media topology:** replaceable `TelephonyMedia` and realtime session seams; Novofon/Yandex as a candidate path; direct SIP only after written confirmation and tests; Asterisk as a possible bridge/fallback; Exolve as reserve; no owned STT/TTS/PSTN/WebRTC stack for the first pilots.
3. **Pipecat/LiveKit use:** sources of typed-event, interruption, heard-output, tool-lifecycle, handoff and test patterns; optional spikes/adapters rather than default domain architecture.
4. **Channel implementation:** minimal channel envelope/outbox, provider identifiers, dedupe, atomic response ownership, one writable adapter path, web/Telegram/MAX/Jivo rollout ordering and account-level contract tests.
5. **Inbox implementation:** bounded pinned-version Chatwoot spike and its pass/fail conditions; custom Staffy Inbox only if external desks block the product requirement.
6. **Business-system adapters:** narrow typed HTTP commands for 1C/ABCP/suppliers, reconciliation after ambiguous timeout, and saga-like compensation only when an operation spans multiple external commits.

The current Addendum explains its purpose but captures only part of this technical spine. These are material Addendum inputs for the subsequent Architecture workflow, not missing FRs. Product requirements should continue to state provider portability, action safety, reconciliation, handoff and capability verification without prescribing the components above.

## 5. Unverified evidence and hypotheses — do not harden into requirements

### Market and commercial evidence

- TAM, exact willingness to pay and live call quality were not measured by competitive research.
- Public competitor performance, customer volume and several compliance/scale claims are mostly vendor assertions.
- The observed public price bands are not directly comparable because telephony, integration, support and concurrency differ.
- The research proposal for 8–12 interviews per vertical, three pilot partners and at least one paid pilot is a management recommendation, not research evidence. It may become a gate only by explicit owner decision.
- Absence of an exact autoparts competitor is a bounded negative, not proof of demand or whitespace.

### Vendor and connector evidence

- Oleg's actual SIP provider is still an assumption: the Forge named Novofon as a candidate, while later discovery says Mango Office is likely. The account/contract is authoritative.
- Native Yandex SIP was last verified publicly as Preview; applicability of SLA and disabled logging to the whole realtime call path remains unresolved.
- Novofon/Yandex interoperability, codecs, DTMF, transfer, callback, failure semantics, Russian PSTN latency and actual TCO require written confirmation and end-to-end testing.
- Jivo bot/handoff/media semantics, ABCP status mapping, supplier-order behavior and the ABCP→custom-1C operation identity require account-level tests.
- Chatwoot/MAX compatibility, full Russian data-egress perimeter, VK/Avito access and WhatsApp Business Platform eligibility remain conditional.
- Published provider quotas and component prices are planning inputs, not Staffy capacity or cost proof.

### Montagio business evidence

- The 60–75th percentile is a versioned `service × market` hypothesis, not an established fact for all services.
- Supply coverage, partner demand, conversion, bypass, cash collection and contribution margin remain live-pilot hypotheses.
- Simulated focus-group and support-theater reactions are design inputs, not user research; wording and friction require tests with real clients, partner staff and executors.

The PRD appropriately keeps most of these in Open Questions, Assumptions and baseline-derived metrics.

## 6. Superseded or narrowed decisions — do not restore

1. **Universal build is a NO-GO.** The competitive report's earlier `GO-DISCOVERY / NO-GO-BUILD` recommendation is superseded by the owner's later confirmed universal-core direction and by the Forge. Its demand warnings remain valid; its product-direction veto does not.
2. **Autoparts versus healthcare as the sole first vertical.** Superseded by two parallel, equal proof packs: Oleg/autoparts and Montagio/services.
3. **A fixed limit of about 50 clients because of Python/GIL.** Rejected. Capacity is measured in concurrent conversations, media/tool load and tail latency; a Go data plane is justified by observed bottlenecks, not tenant count.
4. **Full custom Go/Rust telephony/media stack from day one.** Narrowed to an owned universal platform/action core with replaceable managed media edge; own STT/TTS/PSTN/WebRTC is deferred until a measured provider gap.
5. **`model + tools + RAG + MCP` is sufficient for deep integration.** Rejected as incomplete; typed authorization, confirmation, idempotency, receipts, reconciliation, policy and owning-system reads remain required.
6. **Seven Montagio workflows.** Superseded by P1–P6 as policy outcomes and P7 as a priority Safety Interrupt; there is no seven-workflow architecture.
7. **Mandatory M1-first ordering.** Narrowed: M1-first is preferred, but M1 and M2 are both required before scale and order may change with operational readiness.
8. **The 60–75th percentile as a universal fixed price fact.** Narrowed to a tested and versioned service-by-market hypothesis; the no-dumping strategy remains fixed.
9. **Complex automatic circuit breakers, queues or enterprise ticketing in the pilot.** Narrowed to simple scoped flags, linked exception records, owner/acknowledgement and audit until observed load or repeated failures justify more.

## 7. Reconciliation conclusion

The current PRD should not be reopened for architectural redesign. The highest-value finalization work is bounded:

1. specify Staffy's commercial unit/packaging and how its own willingness to pay will be validated;
2. require a provider-neutral human inbox fallback for tenants without a compatible desk;
3. make the accepted supply, legal/payment and paid-order evidence exercises explicit gates;
4. add measurable recovery-quality outcomes.

Everything else in the reviewed cluster is either already represented, belongs in the Addendum/Architecture workflow, remains unverified, or has been superseded by later decisions.
