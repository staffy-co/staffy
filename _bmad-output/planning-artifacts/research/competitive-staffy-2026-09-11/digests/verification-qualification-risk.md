# Qualification and citation-risk verification

Accessed: 2026-09-11  
Validation: normal; targeted fresh-context verification; 10 web calls  
Input scope: `competitor-qualification-r1-lead.md` and `landscape-leads-r1-lead.md`

## Executive outcome

- **Verified:** Voxagent has a current Russian contracting entity, a public offer, a purchasable commercial framework, and an explicitly managed launch/operation/support scope.
- **Verified:** Replica has a Russian contracting identity and a live public product with ruble pricing. Its exact principal activity is **disputed** across sources and should not be load-bearing.
- **Verified / unverified split:** Stexa's own page is internally contradictory; its registry, FSTEC, SLA, performance, customer and “millions of minutes” claims remain **unverified**. The search did not locate an official registry record or certificate, but this is not proof of non-existence.
- **Verified:** Myrolit's disclosed seller is a Ukrainian TOV with a Ukrainian legal address and UA bank account. The stronger citation is its legal terms, not a Russian vertical landing page.
- **Verified / unverified split:** Yandex explicitly launched SIP connectivity in Preview with integration by Yandex Cloud experts through support. A current SIP General Availability status could not be proven from the live documentation; current availability of voice-agent documentation is not equivalent to SIP GA.
- **Overturned:** the earlier landscape wording that Voxagent's Russian-market status was unverified is now stale for legal/commercial qualification. Hosting and compliance posture remain unverified.

## Claim-by-claim verification

### A. Voxagent: legal/commercial qualification and managed-service scope

**Outcome: verified.**

Voxagent's public offer, version dated 2026-06-16, names ООО «НОУКОД», ОГРН 1237700304742, and states that payment of an invoice or payment link accepts the contract. It defines the service as voice AI agents that handle calls and perform connected actions including CRM updates, request creation and call transfer. The managed scope explicitly includes process analysis, scenario design, integrations, testing, production launch, ongoing operation, monitoring, scenario tuning, support and reporting, mainly under recurring subscription service.

The entity itself is independently corroborated by RBC Companies as an active Moscow ООО with the same ОГРН and ИНН 9714009485 as of 2026-09-03. This verifies legal identity and current commercial qualification, not any performance, hosting or compliance assertion.

The same offer also directly supports the risk-allocation observations: the customer is the personal-data operator and is responsible for lawful grounds and call-recording consents; Voxagent disclaims guarantees of accuracy/completeness and business outcomes; tariffs can change prospectively. These are verified as contractual terms. The analytical implication that trust and risk allocation are competitive product dimensions is an interpretation, not a separately verified market fact.

Sources:

- Voxagent / ООО «НОУКОД», public offer, version 2026-06-16: https://voxagent.ru/docs/public-offer
- RBC Companies, ООО «НОУКОД», accessed 2026-09-11: https://companies.rbc.ru/id/1237700304742-obschestvo-s-ogranichennoj-otvetstvennostyu-noukod/
- Voxagent pricing page, accessed 2026-09-11: https://voxagent.ru/pricing

Status changes:

- `competitor-qualification-r1-lead.md` findings 1-2: keep **verified/high** for what the offer says.
- `landscape-leads-r1-lead.md` finding 4: **overturned in part**. Replace “Russian-market status ... remain unverified” with “Russian legal/commercial qualification verified; hosting and compliance posture unverified.”

### B. Replica: Russian contracting identity and public product/pricing

**Outcome: verified for identity and commercial product; disputed for the exact activity code.**

Replica's one-page company card identifies ИП Шандульский Илья Олегович, ОГРНИП 324665800003784, ИНН 667210615600, with a Yekaterinburg address and bank details. RBC independently corroborates the same entrepreneur, ОГРНИП, ИНН and Sverdlovsk-region registration. The live Replica homepage exposes a concrete product, demo flow, managed integration, three ruble price tiers (29,900 ₽/month, 49,900 ₽/month, and from 79,900 ₽/month) and a 20,000 ₽ one-time integration charge. Russian contracting identity plus public ruble prices qualifies Replica as a current Russian commercial entrant; it does not validate latency, conversion, customer-result or compliance claims.

The exact principal activity is not safe to state as settled. Replica's vendor PDF says ОКВЭД 58.29 (“Издание прочих программных продуктов”), while RBC reports 62.01 (“Разработка компьютерного программного обеспечения”) and another registry aggregator reports advertising. This may reflect source freshness or classification errors, but the evidence is materially inconsistent.

There is also a visible identifier defect on the homepage footer: it renders `ИНН 667210615`, while the company card and RBC show the valid 12-digit `667210615600`. Do not use the homepage footer as the source for the exact ИНН.

Sources:

- Replica company card, accessed 2026-09-11: https://storage.yandexcloud.net/replica-production/documents/company_card.pdf
- Replica product and pricing page, accessed 2026-09-11: https://thereplica.ru/
- RBC Companies, ИП Шандульский Илья Олегович, accessed 2026-09-11: https://companies.rbc.ru/persons/ogrnip/324665800003784-shandulskij-ilya-olegovich/
- Synapse counterparty page, accessed 2026-09-11: https://synapsenet.ru/organizacii/324665800003784-shandulskij-ilya-olegovich

Status changes:

- `competitor-qualification-r1-lead.md` finding 3: keep **verified** for identity and Russian commercial qualification; change the parenthetical “software publishing activity” to **disputed** or omit it.
- `landscape-leads-r1-lead.md` finding 5: **overturned for Replica only**; Russian legal presence and direct commercial comparability are now established. The other named candidates were not in this verification scope.
- `landscape-leads-r1-lead.md` finding 7: positioning, managed implementation and public pricing are **verified** as advertised; performance, customer outcomes and compliance remain vendor claims.

### C. Stexa: contradiction, certifications and traction

**Outcome: verified contradiction; certifications and traction unverified.**

The contradiction is direct and material on one current page. Stexa says that the company is “в процессе оформления” and that legal details will be published later, while the same page claims that Stexa is in the Russian software registry, is FSTEC-certified, supports public procurement, has a 99.9% enterprise SLA, serves millions of monthly minutes, and has precise accuracy and latency metrics.

Targeted searches for `Stexa`, `Стекса`, `stexa.ru`, the claimed Russian-software-registry inclusion and FSTEC certification returned only Stexa-authored pages; no official registry entry, certificate number, named certificate holder, exact certified product/version, customer announcement or independent traffic/traction evidence was located within the search budget. Therefore:

- internal inconsistency: **verified**;
- Russian software registry inclusion: **unverified**;
- FSTEC certification / “4 class” claims: **unverified**;
- “millions of minutes monthly,” customer deployments and named result metrics: **unverified**;
- 99.9% SLA, 98% accuracy and 400-600 ms latency: **unverified**.

This finding should not be rewritten as “Stexa is not certified” or “Stexa has no customers.” The supported conclusion is narrower: official or independent corroboration was not found, and the vendor page lacks the identifiers needed for due diligence.

Source:

- Stexa About page, accessed 2026-09-11: https://stexa.ru/about/

Status changes:

- `competitor-qualification-r1-lead.md` finding 4: keep **verified/high** only for the internal contradiction and the negative search finding; retain **unverified/low** for every certification, compliance, traction, SLA and performance claim. “Unqualified emerging offer” is a screening recommendation, not a verified fact.

### D. Myrolit: legal jurisdiction

**Outcome: verified as the seller's disclosed jurisdiction.**

Myrolit's legal terms name the seller as ТОВ «Мироліт», ЄДРПОУ 46163131, with a legal address in Cherkasy Oblast, Ukraine, a `UA` IBAN and a Ukrainian bank. Its About page explicitly says the company is registered in Ukraine and is the seller of the service. This supports placement in an adjacent/international column rather than treating it as a Russia-local contracting entity. It does not by itself prove the absence of any separate Russian reseller or contracting route.

Sources:

- Myrolit terms of use, version shown as 2026-09-03, accessed 2026-09-11: https://myrolit.com/legal/terms/
- Myrolit About page, accessed 2026-09-11: https://myrolit.com/ru/o-nas/

Status changes:

- `competitor-qualification-r1-lead.md` finding 5: keep **verified** for the disclosed Ukrainian seller/jurisdiction. Replace the cited construction landing page with the legal terms above. Keep “not a Russia-local compliant supplier” as a cautious inference: “no Russia-local contracting entity was evidenced in this pass.”

### E. Yandex SIP launch status and current GA status

**Outcome: launch Preview status verified; current SIP GA unverified.**

Yandex's 2026-05-28 corporate announcement explicitly says SIP connectivity was in Preview and that integration was performed by Yandex Cloud experts after a support request. The matching Yandex Cloud launch article says the same. Thus the launch-status claim is fully supported.

The live voice-agent documentation confirms an available Realtime API over WebSocket, voice-agent tooling and billing visibility, but it does not label SIP connectivity as GA and does not document a self-service SIP procedure on that page. The current Yandex Cloud service list does not mark AI Studio as an overall Preview service, but that cannot be used to infer that the separate SIP integration has reached GA. No current official SIP GA announcement or documentation statement was found. A final report must say “SIP was Preview at launch; current SIP GA status not proven,” not “SIP is GA now” and not “SIP is definitely still Preview now.”

Sources:

- Yandex corporate announcement, published 2026-05-28: https://yandex.ru/company/news/28-05-2026-04
- Yandex Cloud launch article, published 2026-05-28: https://yandex.cloud/ru/blog/voice-agent-and-other-updates
- Current voice-agent documentation, accessed 2026-09-11: https://aistudio.yandex.ru/ru/docs/ai-studio/concepts/agents/realtime
- Yandex Cloud launch-stage definitions, updated 2026-06-15: https://yandex.cloud/ru/docs/overview/concepts/launch-stages
- Current Yandex Cloud service list, updated 2026-09-08: https://yandex.cloud/ru/docs/overview/concepts/services

Status changes:

- `landscape-leads-r1-lead.md` finding 6: keep **verified/high** as a historical launch-status claim.
- Any new claim that SIP is currently GA: **unverified**.

## Citation-mismatch risks for synthesis

1. **Voxagent feature docs are not legal evidence.** `https://docs.voxagent.ru/ru/features` supports product features, but not Russian legal presence, contracting scope, hosting or compliance. Pair it with the public offer and independent entity record.
2. **Replica homepage is unsafe for the exact ИНН.** Its footer drops three digits. Use the company-card PDF plus the independent RBC record for identity; use the homepage only for product and pricing.
3. **Replica's ОКВЭД is inconsistent.** The vendor card's 58.29 should not be presented as independently verified because current third-party records disagree.
4. **Stexa cannot corroborate itself.** Multiple Stexa pages repeating certification and traction assertions remain one publisher and one underlying source. Cite the About page only to show what Stexa claims and the page's internal contradiction; do not cite it as proof of registry/FSTEC status or customer traction.
5. **Myrolit's construction page is a weak jurisdiction citation.** Its footer identifies a TOV but does not itself explain jurisdiction. Cite the legal terms or About page for Ukraine.
6. **Yandex sources answer different time questions.** The 2026-05-28 announcement proves launch-time SIP Preview. The current Realtime documentation proves current feature availability but not current SIP GA. Do not use one to imply the other.
7. **Yandex solution-page accessibility is unstable.** `https://aistudio.yandex.ru/ru/solutions/voice-agent` returned an anti-bot page during verification. Prefer the accessible documentation and Yandex Cloud blog for final citations; if the solution-page performance claims remain, label them vendor-authored and independently unverified.
8. **Vendor offer terms verify wording, not operating quality.** Voxagent's offer is strong evidence of what it contracts to provide and how it allocates risk, but it is not evidence that the service achieves particular reliability, accuracy, compliance outcomes or customer results.

## Recommended synthesis wording

- **Voxagent:** “Russian commercial competitor operated by active ООО «НОУКОД», with a public offer for managed launch, integration, operation and support; hosting, compliance implementation and performance remain to be independently validated.”
- **Replica:** “Russian commercial managed-service entrant with a corroborated ИП contracting identity and public ruble pricing; performance and customer-outcome claims remain vendor-authored.”
- **Stexa:** “Public but not yet due-diligence-qualified offer: its own page says company registration is still in progress while asserting registry/FSTEC status and large-scale traction without locatable official identifiers or independent corroboration.”
- **Myrolit:** “Ukrainian contracting entity serving Russian-language use cases; no Russia-local contracting route was evidenced in this pass.”
- **Yandex SIP:** “SIP integration was Preview and support-assisted at the May 2026 launch; current GA status was not established by the available official documentation.”

