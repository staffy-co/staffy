# Staffy Product Brief — source extract

Source status legend:

- **[LOCKED]** — treated as a user decision because it appears in `forged-idea.md` under “Staffy — HARDENED.”
- **[RESEARCH]** — finding or recommendation from the technical research; useful evidence, not a user commitment by itself.
- **[UNVERIFIED]** — hypothesis, product inference, or claim that the sources explicitly leave unproven.

## Product vision

- **[LOCKED]** Staffy is a universal, multi-tenant, omnichannel platform for managed AI employees—not a single-industry agent.
- **[LOCKED]** An AI employee is a role with one identity, context, and permission set across phone and text channels; a tenant may operate multiple roles.
- **[LOCKED]** Staffy is an AI-workforce layer over a business’s systems, not a replacement for CRM, ERP/1C, inventory, telephony, payments, or helpdesk.
- **[LOCKED]** The commercial offer starts as a managed AI employee with assisted setup: launch/integration fee, subscription, and actual voice/AI usage. Self-service comes later as proven templates accumulate.

## Buyer, users, and problem

- **[UNVERIFIED] Buyer inference:** Russian SMB/mid-market owners or operations/commercial leaders whose revenue processes span calls, messaging, operators, and systems such as CRM/1C, catalog, supplier, or booking software. The sources do not define or validate a precise ICP, buying trigger, budget holder, or purchase criteria.
- **[UNVERIFIED] End-user inference:** external customers who call or message, and internal managers/operators who receive handoffs, verify risky outcomes, and resolve ambiguous operations.
- **[RESEARCH] Problem:** conversational AI can sound capable while still failing at the economically important part—correctly completing a business process across mutable systems, or transferring it safely to a human. Model/tool/RAG/MCP capability alone does not guarantee authorization, current operational data, reliable side effects, deduplication, recovery, or auditable handoff.
- **[LOCKED] Customer constraint:** businesses should keep supported systems and operator inboxes; Staffy must not force a replacement or create a client-specific core fork.
- **[RESEARCH] Product value statement:** customers pay for a correctly completed or safely handed-off process, not merely a natural-sounding voice.

## Proposed solution and experience

- **[LOCKED]** Free-form conversation is allowed, but execution happens only through typed business actions with validation, authorization, idempotency, risk-based approval, an operation receipt, and reconciliation.
- **[LOCKED]** Documents and explanations use RAG; changing prices, stock, order status, and permissions are read from the owning system immediately before action.
- **[LOCKED]** One shared core owns tenant/employee configuration, role permissions and policies, conversation/task context, domain capabilities, reusable connectors, mutating-action receipts, audit, evaluation, and release safety.
- **[LOCKED]** Voice, web, apps, and messengers connect through replaceable official adapters and a minimal Staffy channel contract. Each conversation has one active response owner—bot or human—and one writable inbox path.
- **[LOCKED]** Preserve a client’s suitable inbox; offer a Staffy Inbox only where needed. For Oleg, Jivo remains the human desk. Montagio uses its existing operating environment.
- **[RESEARCH]** For the first two pilots, keep the reliable core small: an in-process Go Action Gateway, one append-only mutation ledger, immutable runtime configuration/version identifiers, a small evaluation gate, and rollback. Use managed/replaceable telephony and speech components rather than building a full media stack.
- **[LOCKED] Oleg experience:** customer calls a separate SIP line; Staffy finds catalog-verified parts, gets explicit confirmation, creates an ABCP order, and the existing exchange sends it to custom 1C. Initially every order is marked `pending_manager_verification`; gates are removed only by complexity class after observed error evidence.
- **[LOCKED] Initial channel order:** web widget, Telegram, MAX, and Jivo first; VK and Avito after account-level tests; WhatsApp Business Platform only after Russia-specific eligibility is confirmed. Android/iOS use the same Staffy protocol before a dedicated SDK exists.

## Differentiation supported by these sources

- **[RESEARCH + LOCKED PRODUCT THESIS]** Staffy’s intended differentiation is reliable business-process completion: permissioned actions, immutable confirmation, idempotency, explicit unknown outcomes, recovery/reconciliation, and deterministic human handoff—not merely dialogue quality.
- **[LOCKED]** One role, policy, and business-action layer serves voice and text, avoiding separate channel logic and inconsistent behavior.
- **[LOCKED]** Reusable domain capabilities and connectors become part of Staffy; clients are configured rather than forked.
- **[RESEARCH]** Owning business invariants while keeping media/runtime/channel edges replaceable reduces vendor dependence without taking on undifferentiated SIP/STT/TTS infrastructure in v1.
- **[UNVERIFIED]** No market or competitor study in the two sources proves that these attributes are unique, defensible, or purchase-driving. Treat them as differentiated design choices awaiting customer and competitive validation, not as an established moat.

## First two reference domains

1. **[LOCKED] Automotive parts / ABCP (Oleg):** one end-to-end commercial process—catalogically valid product selection through confirmed order creation, followed by manager verification before supplier submission. Relevant integrations: ABCP, custom 1C, and supplier adapters such as ROSSKO/ARMTEK where required.
2. **[LOCKED] Services / Montagio:** the second domain pack on the same core, using Montagio’s existing operational environment. **[MISSING]** The exact actor, customer journey, single commercial process, integrations, write actions, approval points, and success metric are not defined in either source.

## Measurable success

- **[LOCKED] Primary Oleg business metric:** marginal income per 100 target calls versus the human-operated line.
- **[LOCKED] Oleg safety bar:** zero duplicate, unconfirmed, or unverified orders.
- **[LOCKED] Platform proof:** after the two reference packs, a third customer using an existing pack/connectors launches with no core changes, no client fork, and no programmer, requiring at most one configurator workday after correct access and data are supplied.
- **[RESEARCH] Supporting pilot measures:** task completion; catalog accuracy; duplicate side effects; safe escalation; tool success; p50/p95 end-of-turn-to-first-audio; interruption recovery; cost per correctly completed operation; peak concurrency; and failure/recovery behavior.
- **[RESEARCH] Production gate:** every write action retains confirmation evidence and an explicit operation outcome/provider ID or `unknown/manual_review`; failed human handoffs preserve the caller via fallback or return explicitly to the bot; the chosen voice path passes agreed product SLOs and a fact-based TCO recalculation.
- **[UNVERIFIED]** Numeric targets are absent for revenue lift, task completion, conversion, containment/handoff, latency, availability, cost per correct completion, retention, adoption, and time-to-value.

## First-version scope

### In

- **[LOCKED]** One multi-tenant core serving both reference domains; one complete commercial process per domain rather than broad vertical coverage.
- **[LOCKED]** Role/tenant configuration, permissions/policies, conversation and task context, domain capabilities, reusable connectors, action receipts, audit, evaluation, and rollback-safe versioning.
- **[LOCKED/RESEARCH]** Typed `prepare → confirm → execute` write actions, one append-only operation table, manual review for ambiguous outcomes, and narrow 1C/supplier/business-system adapters.
- **[LOCKED]** Replaceable voice/media path; current priority is Novofon + Yandex Realtime, with Exolve as reserve. Existing operator inboxes and deterministic handoff remain part of the experience.
- **[LOCKED]** Web widget plus first-wave Telegram, MAX, and Jivo connections, subject to their stated launch gates.

### Out / deferred

- **[LOCKED]** Full coverage of either industry, client-specific core forks, direct arbitrary model access to business systems, parallel voice/text business logic, or mandatory inbox replacement.
- **[RESEARCH]** Payment/refund writes in the first pilot.
- **[LOCKED/RESEARCH]** Own STT/TTS, full SIP/WebRTC/PSTN core, universal workflow engine, vector platform, multi-agent hierarchy, distributed microservices, Kubernetes solely for the pilot, rich universal messaging model, and public mobile SDKs.
- **[LOCKED]** VK/Avito until live account contract tests; WhatsApp Platform until Russia eligibility is proven; Chatwoot adoption until a bounded one-day spike passes.
- **[LOCKED]** Broad self-service until enough templates/connectors have been proven through assisted deployments.

## Assumptions and open risks

- **[UNVERIFIED] Voice path:** Novofon↔Yandex SIP compatibility, native SIP contractual status, transfer/DTMF/callback behavior, Russian PSTN quality and latency, quotas, full-path SLA, failover, and actual TCO remain unproven. Novofon + Yandex is a project priority, not a proven market winner.
- **[UNVERIFIED] Operational writes:** supplier idempotency/cancel/lookup after timeout, split/partial order behavior, and target 1C versions/extension constraints require contract and fault testing.
- **[UNVERIFIED] Channels/inbox:** MAX bridge behavior, VK/Avito account eligibility and contracts, Chatwoot/MAX compatibility, cross-channel identity linking, receipt semantics, and the full data-egress perimeter remain open.
- **[UNVERIFIED] Compliance:** Realtime WebSocket logging/retention, derived artifacts/backups, Russian personal-data perimeter, and any medical/health-data flow require written vendor answers and legal review.
- **[UNVERIFIED] Economics/operations:** real conversion, quality, returns, call mix, concurrency, support/on-call burden, retention, and cost per correct completion are unknown until live pilots; synthetic evaluation cannot substitute for this evidence.
- **[RESEARCH] Architecture assumption:** a managed speech/runtime edge plus Staffy-owned action/integration core is the lowest-risk v1 boundary. Future scale conclusions remain provisional until Staffy load tests exist.
- **[RESEARCH] Rejected unsupported claims:** a forced Pipecat rewrite at 50 customers due to Python GIL; guaranteed 280–350 ms first audio or 30–50 ms interruption stop from Go; 4–5× infrastructure savings; a 2–3 month production-core estimate; or three-minute SIP onboarding.

## Two-to-three-year vision implications

- **[LOCKED]** Evolve from two assisted reference packs into a catalog of reusable domain capabilities and connectors on one core, supporting multiple AI employee roles per tenant across voice and text.
- **[LOCKED]** Shift progressively from assisted setup toward self-service configuration as templates, policies, evaluations, and connectors become proven; new integrations remain shared Staffy assets rather than forks.
- **[LOCKED]** Prove platform repeatability through third-and-later deployments on existing packs, with configuration measured in hours rather than custom engineering.
- **[RESEARCH]** Keep media, runtime, telephony, and inbox providers replaceable. Expand an owned Go data plane only after controlled tests show a material latency, reliability, residency, capability, or all-in cost advantage; a full owned telecom stack is a later strategic choice, not a v1 prerequisite.
- **[RESEARCH]** Operational scale implies stronger shared registries, canary/release controls, workflow tooling, and richer channel/identity infrastructure only when tenant/process complexity demonstrates the need.
- **[UNVERIFIED]** The sources do not define 2–3-year customer, revenue, geography, vertical, headcount, channel coverage, or unit-economics targets.

## Missing inputs requiring the user

1. Confirm whether every decision in the “HARDENED” forged idea should be treated as locked for the Product Brief, especially Go as the primary language and the named channel rollout order.
2. Define the initial ICP and economic buyer: company size/type, geography, decision-maker, urgent buying trigger, current alternative, and willingness-to-pay/budget range.
3. Provide user evidence: named customer interviews or observed workflows showing the problem’s frequency, severity, and purchase priority; neither source contains market/customer validation.
4. Specify Montagio’s single v1 commercial process, actors, systems/integrations, write actions and approvals, handoff, and primary success metric.
5. Set numeric v1 targets and time horizon for Oleg and Montagio: baseline and target marginal income/conversion, task completion/catalog accuracy, safe handoff, latency/SLO, cost per correct completion, and acceptable error/return rate.
6. Set explicit pilot boundaries: inbound/outbound calling, supported languages/hours, expected volume/concurrency, data retention/recording, operator staffing, and which first-wave channels must ship versus may follow.
7. Confirm commercial packaging and target economics: setup fee logic, subscription bands, usage pass-through/markup, reusable-connector pricing, gross-margin target, pilot duration, and conversion-to-paid criteria.
8. Define 2–3-year ambition in measurable terms: target customers/tenants, revenue or ARR, priority geographies/verticals, role and channel breadth, self-service milestone, and whether Staffy Inbox is strategically intended or only a fallback.
