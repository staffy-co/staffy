# Advanced Elicitation 03 — User Persona Focus Group

**Target:** accepted Montagio consilium synthesis after Assumption Audit and Cascading Failure Simulation, including the owner's premium value-selling direction  
**Status:** proposal only; not yet applied to the synthesis or PRD  
**Evidence warning:** this is a simulated persona focus group derived from the consilium and project context. It generates hypotheses for later interviews and usability tests; it is not customer research evidence.  
**Method:** reactions → concerns → priorities

## Participants

| Persona | Situation | Primary job |
|---|---|---|
| Анна, 37 | Homeowner requesting a standard faucet replacement on the website | Solve the task without surprises or a long questionnaire |
| Валентина, 68 | Calls by phone, is uncomfortable sending photos and distrusts bots | Be heard, understand the next step and retain access to a person |
| Ольга, 31 | Comes from Avito and compares with a cheaper private listing | Understand what the higher price buys and whether Montagio is trustworthy |
| Елена, 42 | Salesperson in a partner plumbing/heating store | Refer the installation quickly without inheriting Montagio's promises or complaints |
| Алексей, 39 | Independent executor receiving work from Montagio | Receive an accurate, profitable scope and avoid disputes over promises he did not make |
| Ирина, 35 | Founder/operator watching exceptions and economics | See only decisions requiring intervention and know whether an order produced real cash |
| Дмитрий, 46 | Cafe administrator reporting an urgent problem but unable to approve a large spend | Reduce downtime and give the authorised manager a decision-ready case |

## Focus-group conversation

### First contact and qualification

**Анна:** “Answer my question first. If the first reply is a request for six fields and three photos, I will leave.”

**Елена:** “Same in the store. My referral form must take about as long as telling a colleague the essentials. If Montagio needs a dossier, the salesperson will stop offering it.”

**Алексей:** “But if she sends only ‘install the heater’, I inherit the missing electrical connection, brackets and access problem.”

**Priority revealed:** progressively collect facts, but define a source-specific minimum. Show value before asking for effort. Partner intake must be short while preserving product/SKU, delivery, consent and promises already made.

### Premium price and trust

**Ольга:** “If you only say ‘we are reliable’, that sounds like advertising. Show what is included that the cheaper offer does not include.”

**Анна:** “I can pay more to avoid an unknown master and surprise extras. I will not pay more because a bot has learned persuasive phrases.”

**Ирина:** “Montagio must close sales. A timid agent that apologises for the price destroys the model.”

**Валентина:** “Confidence is good. Pressure is not. Tell me clearly what happens next and let me ask for a person.”

**Priority revealed:** premium selling must be evidence-linked. Staffy explains the concrete controlled result for this scope, confidently recommends a next step and handles objections, but never uses psychological vulnerability to set price or trap the customer.

### Two confirmations

**Анна:** “I do not want to approve the same thing twice.”

**Алексей:** “The first confirmation cannot bind me before I have accepted the order.”

**Ольга:** “Then make the difference obvious: first I confirm the task; later I accept a named master and final terms.”

**Priority revealed:** the UI and dialogue must name the distinct consequence of each confirmation and show only what changed at the second step. Repeating the entire intake will feel like an error.

### Photos, channel switching and accessibility

**Валентина:** “If I cannot send a photo, do not punish me. Tell me whether someone can inspect it.”

**Анна:** “If I move from the call to the website, do not make me start over.”

**Елена:** “And do not make the customer wonder where Montagio got the number. Say it came from our store and why.”

**Priority revealed:** provide a non-photo route; retain context across an explained channel transition; let the customer correct carried facts; state the contact source and consent basis immediately.

### Safety, urgency and complaints

**Дмитрий:** “During a leak in a cafe, do not sell me a maintenance package. Help me limit damage, collect the facts and prepare something my manager can approve.”

**Ирина:** “Claims are the other no-selling zone. If the customer says the installation is leaking, a cross-sell is reputational suicide.”

**Priority revealed:** Staffy needs explicit conversation modes. `sales` may use consultative persuasion; `safety`, `claim`, `cancellation`, material distress and human-request states suspend persuasion and optimise for safe resolution.

### Executor and operator experience

**Алексей:** “Show me the evidence, exclusions, payout and who supplies materials before I accept. Do not score me down for rejecting an unsafe or unprofitable order.”

**Ирина:** “And do not notify me about every normal event. I need ranked exceptions, owner, deadline, financial exposure and the next safe action.”

**Priority revealed:** the master-facing offer must be as explicit as the customer-facing offer. Refusal reason must distinguish capacity, mismatch, risk and economics. Human queues need decision-ready summaries, not raw transcripts or notification floods.

## Main tensions

1. **Short intake vs accurate scope.** Solve with progressive, route-driven questions and source-specific minimums—not a universal long form.
2. **Premium conversion vs customer autonomy.** Solve with confident evidence-based value selling and explicit no-selling modes—not weak sales or manipulation.
3. **Two confirmations vs repetition.** Solve by naming different commitments and showing the delta at final confirmation.
4. **Automation vs dignity/accessibility.** Solve with a human path, non-photo route and no penalty for channel or ability constraints.
5. **More leads vs executor quality.** Do not expose orders whose scope, evidence, payout or risk is too incomplete for a responsible acceptance decision.
6. **More alerts vs operator control.** Exceptions are ranked and owned; routine progress stays out of the founder's attention queue.

## Proposed priority changes if accepted

### P0 — required in the Montagio pilot journeys

1. Add an explicit **conversation-mode policy**: `sales`, `service/status`, `safety`, `claim`, `cancellation`, `human_requested`. Only `sales` permits persuasive objection handling; safety and claims always override it.
2. Require **evidence-linked value explanation**. For a price objection, Staffy must map the premium to concrete scope, controls and outcome; generic “quality/reliability” claims are insufficient.
3. Keep pricing deterministic and forbid emotional or inferred-wealth price discrimination. Marketing skill controls explanation and recommendation, not the numeric price.
4. Make two confirmations visibly different: preliminary task/route confirmation, then a delta-focused named-executor/final-terms confirmation.
5. Require a valid no-photo route and a no-repeat cross-channel continuation with client correction of carried facts.
6. Present the contact source and consent basis at the beginning of partner outreach.
7. Give the executor a decision-ready offer: scope/version, evidence, exclusions, materials, access, risk, customer window and payout; a justified rejection must not be treated as poor performance.
8. Give humans ranked exception summaries with reason, exposure, deadline, current owner and recommended safe action—not transcript dumps.

### P1 — validate during controlled and live cohorts

9. Add trust and pressure counter-metrics: first-answer usefulness, understood price value, perceived pressure/deception, human request, repeated question, second-confirmation abandonment and “I did not agree to this” incidents.
10. Test premium-price objection playbooks against low-price, distrust-of-AI, “exact price now”, “free measurement”, urgency and partner-referred contexts.
11. Let each tenant configure brand voice, proof library, allowed claims and objection playbooks within shared platform safety and anti-manipulation constraints.
12. Validate partner intake with real store staff and executor offers with real candidate executors before building deep automation.

## Questions that require real people, not further simulation

- Which concrete Montagio assurances justify an upper-market price to each customer cohort?
- At what point do two confirmations feel protective rather than repetitive?
- What is the maximum acceptable partner-store intake effort in the real checkout workflow?
- Which executor facts are necessary to accept an order without a clarification call?
- Which wording discloses AI clearly without reducing trust or creating a false human persona?
- Which objection responses feel confident and helpful, and which feel manipulative?

## Explicit non-changes

- Do not lower prices merely to improve conversion.
- Do not weaken Staffy's sales role into passive information delivery.
- Do not let “psychology” become covert manipulation, vulnerability exploitation or dynamic willingness-to-pay pricing.
- Do not build separate conversation engines for each persona or channel; these are modes, policies, proofs and tenant configuration on the shared core.

