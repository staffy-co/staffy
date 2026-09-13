# Advanced Elicitation 01 — Assumption Audit

**Target:** `consilium/synthesis.md`  
**Status:** proposal only; not yet applied to the synthesis or PRD  
**Method:** list → rate → stress-test → shore up

## Rating scale

- **Confidence:** how strongly the assumption is supported now, before real paid Montagio orders.
- **Impact:** damage to the launch claim if the assumption is false.
- `Critical` means the business proof or customer safety can fail even if Staffy conducts a good conversation.

## Assumption register

| ID | Assumption underlying the current synthesis | Confidence now | Impact if false | What supports it / what is missing |
|---|---|---|---|---|
| A1 | For each exposed service, zone, time window and risk class, Montagio can actually supply a suitable primary or backup executor. | Low | Critical | The operational model and readiness gates exist, but there are no verified live-order supply metrics. |
| A2 | In the Stage 0 direct-payment model, the executor will reliably pay Montagio its fee and every order can be reconciled. | Low | Critical | The model reduces legal/payment complexity, but collected commission has not been proven on real orders. |
| A3 | Legal roles, consent, personal-data processing, receipts, cancellations, claims and guarantees can be approved before the first paid order without materially changing the flow. | Low–medium | Critical | Known as an external go-live gate; no final lawyer/accountant-approved operating scheme is recorded. |
| A4 | Launch prices and executor payouts leave positive contribution margin after acquisition, Staffy/telephony, manual work, repeat visits, claims, refunds and bad debt. | Low | Critical | Catalog ranges are planning inputs, not public offers or business-verified unit economics. |
| A5 | A partner store can obtain provable consent and submit a complete enough lead without making the referral process unattractive. | Low | High | A proposed lead contract exists; no store-side workflow has been tested with real staff and leads. |
| A6 | Customers will accept disclosed AI, progressive qualification and requests for photos without unacceptable abandonment or distrust. | Low | High | Conversation design is coherent; controlled and live user evidence is still absent. |
| A7 | A low-risk allowlist can be qualified remotely with an acceptably low rate of false fixed-price promises. | Medium–low | High | Service routing and safety taxonomy exist; field calibration against actual site discoveries is missing. |
| A8 | The founder can absorb early exceptions until an operator or technical curator is added, without breaking response time or decision quality. | Low–medium | High | The founder bottleneck is recognized; capacity and escalation thresholds are not yet evidenced. |
| A9 | Preliminary route confirmation plus final named-executor confirmation protects the customer without damaging conversion too much. | Medium–low | High | The two confirmations solve a real marketplace gap; their friction has not been measured. |
| A10 | One shared lifecycle can serve store, website, Avito and B2B demand without leaking B2B complexity into simple B2C orders. | Medium | High | The seven-route model is structurally plausible; no cross-source live cohort exists. |
| A11 | Cross-channel identity and deduplication can prevent duplicate orders without false merges or privacy harm. | Medium–low | High | Montagio has technical reliability foundations, but real multi-channel identity behaviour is unverified. |
| A12 | Novofon and the selected text channels will pass account-level end-to-end quality, DTMF, media, handoff and reliability gates. | Medium for documented capability; low for account E2E | High | Novofon direction is fixed and contract exists, but the live SIP trunk and controlled calls are externally blocked. Avito/MAX also have external gates. |
| A13 | Stores, consumers and later B2B customers have enough demand and willingness to pay for Montagio's controlled-result proposition. | Low | High | Market logic exists; there are no paid-order cohorts or verified conversion data yet. |
| A14 | Customer and executor bypass will stay below a level that destroys commission economics. | Low | High | Contract, follow-up and value-based controls are proposed; actual leakage is unknown. |
| A15 | The chosen 14 launch services are the right initial operating set, rather than only a sensible catalog hypothesis. | Low–medium | Medium–high | They were selected from a 54-service operational catalog, but not validated by real demand, supply and incident data. |
| A16 | Partner referral economics are valuable enough to Montagio and acceptable enough to stores to sustain the channel. | Low | Medium–high | Attribution and commission events are designed; no negotiated partner terms or paid cohort is verified. |

## Weakest-foundation stress test

### 1. Demand arrives before supply is ready

**Failure:** Staffy qualifies perfectly and promises a route, but nobody suitable accepts it. The customer experiences Montagio as an efficient lead form with no service behind it.

**Shore-up:** expose a service/zone/window/risk combination only after a supply-readiness drill has verified a primary and a fallback route. If readiness expires or both reject, stop selling that combination and offer an honest alternative instead of an invented ETA.

### 2. Work is completed, but Montagio does not collect

**Failure:** the dashboard shows completed orders and theoretical margin while cash goes directly to the executor and Montagio's fee is late, disputed or bypassed.

**Shore-up:** define a business-complete order as one with linked scope/version, executor acceptance, proof of customer payment, executor receipt/check where applicable, Montagio fee receipt or recorded exception, and claim-window state. Run a small manual cash-closure test before automating or scaling.

### 3. Catalog margin disappears in reality

**Failure:** access complications, extra work, founder time, acquisition cost, repeat visits and claims turn an attractive midpoint calculation into negative contribution margin.

**Shore-up:** treat all current prices, payouts and margins as hypotheses. Recalibrate after a paid-order cohort using actual collected revenue, executor payout, acquisition cost, Staffy/telephony cost, manual minutes, revisit/claim reserve and bad debt. Do not use catalog `target_margin` as proof of profitability.

### 4. Partner leads create distrust instead of revenue

**Failure:** the customer did not understand that their number would be passed on, the product or delivery context is incomplete, and Staffy calls at the wrong time. The store blames Montagio; Montagio cannot attribute the commission.

**Shore-up:** test one store and a small manually reviewed lead batch before API integration. Require consent evidence, preferred channel/time, SKU/model, purchase/delivery state and promises already made. Measure completeness, contactability, duplicate rate, opt-out and paid conversion.

### 5. The safe conversation becomes too laborious

**Failure:** progressive intake, photos, role disclosure and two confirmations are individually reasonable but collectively feel like bureaucracy; the customer drops or asks for a human.

**Shore-up:** compare controlled and live journeys by time-to-first-useful-answer, meaningful question blocks, repeated questions, photo refusal, abandonment, human requests and confirmation completion. Keep the safeguards, but earn each question from the route being decided.

### 6. The legal operating model forces a redesign at the end

**Failure:** payment wording, guarantee allocation, consent, KKT or claim ownership contradict the designed journey immediately before launch.

**Shore-up:** run a concrete tabletop with lawyer and accountant before the first paid order: one ordinary order, cancellation, extra work, refund/claim and partner lead. Approve the exact Stage 0 role and customer-facing wording; keep full agent payment out until separately approved.

### 7. B2B language outruns real capacity

**Failure:** a cafe hears “emergency service” or an SLA, but there is no authorised buyer, contract, night access or backup executor.

**Shore-up:** keep B2B at discovery and human commercial approval. Do not advertise a response commitment until the contract, authority chain, access rules and primary/backup supply have been demonstrated for that service and zone.

## Proposed changes if accepted

1. Add an **Evidence gates before scale** section to the consilium synthesis and later derive PRD requirements from it.
2. Reframe the first proof as two linked tracks:
   - UJ-M1 proves the Staffy customer journey on a low-risk website order;
   - UJ-M2 proves the partner/referral business model and cash closure.
3. Make `service × zone × window × risk` supply readiness a prerequisite for exposing availability or promising a route.
4. Define a business-complete order by reconciled evidence, not merely operational status “work completed.”
5. Mark price, margin, conversion, partner demand, B2B demand and bypass assumptions explicitly as hypotheses until live evidence exists.
6. Add counter-metrics alongside conversion and completed orders: `no_supply`, re-quote after arrival, uncollected commission, founder/manual minutes, bypass suspicion, repeat visit/claim and complaint rate.
7. Add three pre-launch evidence exercises: supply drill, Stage 0 legal/payment tabletop and a manually reconciled paid-order micro-cohort.
8. Avoid invented performance thresholds in the PRD. Record baselines first, then set approval thresholds with named owners from the observed cohort.

## Explicit non-changes

- Do not replace the shared Staffy core with a Montagio-specific core.
- Do not remove the seven outcome routes or the eight journeys.
- Do not delay all learning until every integration is automated.
- Do not claim that any hypothesis above has already failed; this audit identifies what is not yet proven.

