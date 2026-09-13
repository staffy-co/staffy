# Advanced Elicitation 02 — Cascading Failure Simulation

**Target:** accepted Montagio consilium synthesis after Assumption Audit  
**Status:** proposal only; not yet applied to the synthesis or PRD  
**Method:** trigger failure → trace propagation → find amplifiers → decouple

## What the simulation tests

The dangerous failure is rarely one bad model answer. In Montagio, a local error can cross conversation, supply, payment, channel and reputation boundaries. The simulation therefore follows failures until they stop producing downstream commitments.

## Cascade 1 — partner promotion overwhelms real supply

**Trigger:** a partner store promotes installation with a product campaign and sends many leads into a service/zone/window whose readiness record is stale.

**Propagation:**

1. Staffy qualifies leads and records preliminary confirmations.
2. The same small executor pool receives more offers than it can accept.
3. Masters decline, time out or accept and later cancel.
4. The founder manually searches for replacements and becomes the dispatch bottleneck.
5. Client responses slow down; Staffy and the founder may send conflicting updates.
6. Previously discussed windows and prices expire, forcing re-qualification and re-quote.
7. Customers cancel or complain to the store; the store disputes lead quality and commission.
8. A successful acquisition campaign damages both brands and produces negative cash contribution.

**Amplifiers:** readiness without expiry; no per-route capacity reservation; promotion disconnected from supply; one human exception owner; no single reply owner; duplicated cross-channel cases; no automatic stop-sell.

**Decoupling proposal:** expiring readiness; capacity tokens or explicit pending status; campaign admission control by service/zone/window/risk; circuit breaker that disables new promises while preserving honest intake; atomic response ownership; cancellation/recovery flow with one order identity.

## Cascade 2 — false fixed price becomes a trust and claim incident

**Trigger:** an apparently ordinary request is routed to P1, but hidden conditions make the fixed scope impossible on arrival.

**Propagation:**

1. The customer treats the preliminary number as a promise.
2. The master discovers extra work, missing materials or unsafe access.
3. Without enforced stop-work, work begins or the price changes verbally.
4. The customer perceives bait-and-switch, refuses payment or pressures the master to proceed unsafely.
5. Montagio lacks one agreed version of scope, price and evidence.
6. The complaint expands into a dispute between customer, executor and product-selling partner.
7. Commission is lost; the same bad rule continues pricing other orders.

**Amplifiers:** AI confidence treated as evidence; partner-provided SKU accepted without client verification; photos requested without required evidence rules; mutable offer; completion without final version; no automatic suspension after anomalies.

**Decoupling proposal:** deterministic P1 eligibility; uncertainty routes to P2/P3/P4/P5; immutable offer versions; enforced stop-work and reconfirmation; anomaly counter that suspends the pricing rule/service combination for review; product and installation warranty recorded separately.

## Cascade 3 — operational completion hides a cash failure

**Trigger:** an executor marks work complete, but payment and Montagio's fee are not evidenced.

**Propagation:**

1. Reporting counts a successful order and theoretical contribution margin.
2. Acquisition or partner payouts are increased on false economics.
3. Missing fees, off-platform continuation and disputed attribution accumulate.
4. The founder discovers the cash gap only during manual reconciliation.
5. Claim handling is weakened because receipts, scope and payment parties are incomplete.
6. Montagio scales a channel that may be economically negative.

**Amplifiers:** one overloaded `completed` state; projected margin presented beside collected cash without distinction; no reconciliation queue; no exception ageing; weak value for staying on-platform.

**Decoupling proposal:** separate `work_completed`, `customer_payment_evidenced`, `fee_due`, `fee_collected`, `claim_window_open/closed` states; cash-based business success metrics; aged-exception queue; manual reconciliation in the pilot; channel scaling blocked when financial closure deteriorates.

## Cascade 4 — channel degradation creates duplicates and contradictory promises

**Trigger:** a Novofon call drops while a web continuation is delayed or a connector retries an event.

**Propagation:**

1. The customer repeats the request in another channel.
2. Retry or identity mismatch creates a second case/order.
3. Staffy and a human each believe they own the reply.
4. Two versions of scope, price or appointment are sent.
5. Two masters can be offered or assigned the same work.
6. Cancellation reaches only one record; the other continues.
7. Customer trust, privacy and partner attribution are damaged.

**Amplifiers:** channel session used as order identity; non-idempotent side effects; automatic uncertain merge; no delivery ledger; ownership changes without acknowledgement; promises emitted before durable recording.

**Decoupling proposal:** channel-independent case/order identity; idempotency at every external side effect; suspected-duplicate review instead of uncertain auto-merge; durable outbox/inbox and delivery status; atomic takeover/return; persist commitment before sending; degraded mode that collects contact/context but does not promise unavailable actions.

## Cascade 5 — missed safety signal turns sales automation into an incident

**Trigger:** the client describes a leak near electricity, hot coolant, gas-related equipment or a common-building system in ambiguous everyday language, and the case stays in a normal sales route.

**Propagation:**

1. Staffy continues qualification or asks for photos instead of prioritising safety.
2. The client waits, touches unsafe equipment or assumes Montagio has taken responsibility.
3. Damage or injury escalates while assignment is still being attempted.
4. A late human handoff inherits incomplete facts and no clear emergency actions.
5. The incident produces a claim, regulator attention and reputational damage across Montagio and Staffy.
6. If the faulty rule remains active, the same class of incident can recur immediately.

**Amplifiers:** safety check after commercial intent; generative advice outside an approved playbook; unsafe certainty; no immediate route freeze; evidence scattered across channels.

**Decoupling proposal:** safety triage before commercial steps; conservative P7/P5 routing under material uncertainty; only approved short emergency guidance; immediate human escalation with an atomic context snapshot; incident switch that freezes the affected rule/service version; mandatory regression case before reactivation.

## Hidden coupling and single points of failure

| Coupling / single point | Why it is dangerous | Required separation |
|---|---|---|
| Founder owns every exception | Demand, safety, supply and claims all queue behind one person | Separate operational, technical/safety and commercial/legal queues with explicit on-call ownership |
| One `completed` status | Service success, cash success and claim status become indistinguishable | Independent operational, financial and claim states |
| Availability used as a promise | Stale capacity converts intake into broken commitments | Expiring readiness plus reservation/pending semantics |
| Channel session equals customer/order | Retries and channel switching create duplicates | Canonical identity with reviewed merge and idempotency |
| One pricing rule affects all new cases | A local field anomaly repeats at platform speed | Versioned rules, cohort monitoring and scoped circuit breaker |
| AI/human handoff without ownership protocol | Simultaneous or absent replies | Atomic owner, acknowledged transfer and timeout fallback |
| Partner campaign independent of operations | Marketing can exceed fulfilment capacity instantly | Campaign-level admission control tied to live readiness |

## Proposed changes if accepted

1. Add a **Containment and degraded-mode principles** section to the synthesis and derive explicit PRD requirements later.
2. Require scoped circuit breakers for availability promises, P1 pricing rules, unsafe routes, connector side effects, campaigns and financial scaling.
3. Split operational completion, customer-payment evidence, fee due/collected and claim-window state; never collapse them into one order status.
4. Require every customer commitment to be durably recorded and versioned before it is sent; retries must be idempotent.
5. Add an explicit degraded mode: Staffy may acknowledge and preserve context but cannot invent availability, price, delivery or SLA when a dependency is uncertain.
6. Establish three human exception queues with named ownership semantics: operations/supply, technical/safety and commercial/legal/claims. The exact staffing model remains a later operating decision.
7. Add five cascade scenarios above to the required eval and operational drill set, including recovery—not only failure detection.
8. Define scoped freeze/re-enable rules: an anomalous service, pricing rule, route, connector or campaign can be stopped without disabling the entire Staffy tenant.

## Explicit non-changes

- Do not build a complex global orchestration system before the pilot; circuit breakers may initially be simple operator-controlled switches with audit history.
- Do not promise automatic recovery where the safe action is to stop commitments and hand over.
- Do not turn all exceptions into founder notifications; queue ownership must prevent a new single point of failure.
- Do not treat every connector outage as a full service outage if safe intake and later continuation remain possible.

