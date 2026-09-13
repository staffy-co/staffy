# Advanced Elicitation 04 — Customer Support Theater

**Target:** accepted Montagio consilium synthesis after the simulated persona focus group  
**Status:** proposal only; not yet applied to the synthesis or PRD  
**Method:** complaint → investigation → resolution → prevention

## Scene 1 — “Your bot promised one price; the master demands more”

**Customer:** “You quoted 8,000. The master is here and says 13,000. This is bait-and-switch.”

**Staffy support mode:** “I understand why this looks like the terms changed after you committed. No additional work should begin until you approve a new version. I will compare the offer you confirmed with what was found on site and pass the discrepancy with the photos and both scopes to the claim owner. You will not need to repeat the order history.”

**Investigation:** retrieve the confirmed offer version; identify whether the original route was P1 or P2; compare exclusions, arrival evidence and the proposed extra work; determine who changed what and when; suspend the affected pricing rule if the discrepancy may repeat.

**Resolution boundary:** Staffy may pause work, collect evidence, explain recorded terms and present policy-approved choices. It may not decide fault, force the client to pay, authorise an unapproved refund or accuse the executor.

**Prevention:** immutable offer versions; explicit P1/P2 language; arrival check; enforced stop-work; claim bundle; anomaly-triggered rule freeze.

## Scene 2 — “Why are you calling me? I never agreed to this”

**Customer:** “I only bought a heater. I did not ask your company to call me.”

**Staffy support mode:** “I am sorry the contact was unexpected. The referral is recorded as coming from the named store, but I will not continue the sales conversation. I can register that you do not want further contact and have the consent record reviewed.”

**Investigation:** preserve the source, partner, salesperson, consent representation, time and transferred fields; stop outbound attempts; check whether the same contact exists in another case; route the dispute to privacy/partner review.

**Resolution boundary:** Staffy can honour a no-contact preference and explain the recorded source. It cannot insist that consent existed merely because the partner supplied a flag, expose store-internal information or continue persuasion.

**Prevention:** partner consent contract; explicit source disclosure in the first sentence; proof-quality checks; partner scorecard; automatic suppression across permitted channels; sanctions or pause for repeated bad referrals.

## Scene 3 — “Two masters are coming to the same address”

**Customer:** “One person called from your website order and another from the Avito chat. Which one is real?”

**Staffy support mode:** “This appears to be a duplicate assignment. I have paused further assignment actions while the records are checked. I will confirm one valid order and one next step; please do not coordinate separate visits until that confirmation.”

**Investigation:** find candidate cases by contact, address, work item and time; compare confirmed offers and assignment events; identify which side effects were delivered; place a temporary hold on both records; assign one human owner for the customer reply.

**Resolution boundary:** cancel or merge only under the applicable policy and authority; notify affected executors without leaking unnecessary client data; preserve both histories and the merge rationale.

**Prevention:** canonical case/order identity; suspected-duplicate queue; idempotent assignment; atomic owner; cancellation propagation; recovery drill for out-of-order events.

## Scene 4 — “The installation is leaking and nobody answers”

**Customer:** “The work finished this evening. Water is leaking again and the master is not answering.”

**Staffy support mode:** the conversation immediately enters `safety` plus `claim`. Staffy gives only approved short risk-limiting guidance appropriate to the detected hazard, stops selling, confirms the address/order/context already known and escalates to the current emergency/claim owner with a complete snapshot.

**Investigation:** severity first; order and executor identity; accepted scope and completion evidence; current hazard; contact attempts; warranty/claim route; available human and backup response capability.

**Resolution boundary:** Staffy cannot diagnose hidden causes, assign liability or promise an emergency arrival without verified capacity. It must keep the customer updated even when the next update is “the human owner has not yet accepted.”

**Prevention:** post-work safety intake; on-call ownership; acknowledgement and update states; no invented ETA; evidence bundle; regression case; service/route freeze if systemic.

## Scene 5 — “I said it was too expensive, but the bot kept pushing”

**Customer:** “I declined several times. Your operator kept telling me I would regret choosing someone cheaper.”

**Staffy support mode:** “That pressure is not the experience Montagio intends. I will stop the sales flow, record your preference and have the conversation reviewed. You may end the contact now or speak with a person; neither choice changes the price.”

**Investigation:** inspect objection classification, response count, wording, model/playbook version and whether the client requested a stop or human; check for fear, shame, fake scarcity or unsupported claims; determine whether the same version affected other conversations.

**Resolution boundary:** Staffy may apologise for the experience and stop persuasion. It must not erase audit evidence, promise compensation outside policy or defend manipulative wording as “strong sales.”

**Prevention:** objection-attempt limit; immediate honouring of stop/human intent; prohibited-pattern detector; scoped playbook freeze; cohort search; adversarial eval; pressure/deception counter-metric.

## Scene 6 — “Your company and the store are sending me to each other”

**Customer:** “The store says the heater is fine and installation is your problem. Your master says the product is defective. Nobody owns the issue.”

**Staffy support mode:** “You should not have to coordinate the organisations yourself. I will open one case, separate the product and installation evidence, and keep one Montagio contact while the responsible parties review it.”

**Investigation:** link product/SKU/receipt/delivery condition, compatibility decision, installation scope, before/after evidence, executor notes and each party's warranty role; record contested facts rather than selecting the convenient account.

**Resolution boundary:** Staffy coordinates and communicates status but does not make the final technical or legal fault decision. The authorised human may request inspection, remediation, partner review or a formal refusal with rationale and contest path.

**Prevention:** separate product and work warranties; one claim ID; named claim owner; shared evidence package with least-necessary partner disclosure; clear customer contest route.

## What the theater revealed

1. Support cannot be a disconnected bot or inbox. It needs the same canonical order, offer versions, event history, channel context and ownership state as sales and fulfilment.
2. `claim`, `safety`, `cancellation`, `privacy/no-contact` and `human_requested` are authority-changing modes, not merely changes in tone.
3. A handoff is not complete when Staffy sends a notification. It is complete only when a named human owner acknowledges it or a defined fallback takes ownership.
4. Customers need progress updates during investigation; silence converts a manageable error into perceived abandonment.
5. Staffy needs policy-approved recovery actions, but final fault, non-standard compensation, refund and legal conclusions remain gated.
6. Root-cause prevention requires linking each complaint to the exact rule, prompt/playbook, connector, service, partner, offer and executor versions involved.
7. Premium positioning makes recovery more important, not less: an upper-market price creates a higher expectation of ownership, clarity and follow-through.

## Proposed changes if accepted

### P0 — journey and authority requirements

1. Expand conversation modes with `privacy/no_contact` and ensure every non-sales mode changes allowed actions, not only wording.
2. Define a canonical **service-recovery case** linked to the original order, offer versions, evidence, channel events, payments and parties; the customer receives one case identity and one current owner.
3. Require acknowledged handoff: `requested → assigned → acknowledged → resolved/returned`, with timed fallback if the assignee does not accept.
4. Give Staffy a policy-bounded recovery action set: pause work/assignment, suppress contact, request evidence, present approved options, provide status, escalate and freeze a scoped rule. Fault, exceptional refund/compensation and legal conclusions require authorised human approval.
5. Require proactive status updates with an honest next-update commitment even when resolution is pending; never invent ETA or human acceptance.
6. Preserve disputed facts and allow the customer to correct or contest the AI summary without destroying the audit trail.

### P1 — prevention and measurement

7. Link every incident to versioned decision inputs: service/market rule, offer, prompt/playbook, connector, partner and executor qualification.
8. Add complaint recovery metrics: acknowledgement time, time to named owner, update-promise adherence, time to safe containment, resolution time, reopen rate, customer acceptance/contest, compensation cost and recurrence by root-cause version.
9. Add a scoped cohort search and freeze procedure when one complaint suggests a repeatable pricing, safety, consent or persuasion defect.
10. Include the six theater scenes in operational rehearsal, with failure injected into human acknowledgement and customer-update paths.

## Explicit non-changes

- Do not promise that AI alone resolves warranty fault, refunds or compensation.
- Do not turn premium service into unconditional acceptance of every customer demand; evidence and authority still matter.
- Do not require a separate support core. Recovery reuses the shared Staffy lifecycle, state, policy, skills and tools.
- Do not build a full enterprise ticketing suite for the pilot; a linked recovery case, explicit owner/state and auditable actions are sufficient.

