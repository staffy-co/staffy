# Verification — demand and verticals

Accessed: 2026-09-11  
Validation: normal; one independent check per load-bearing claim where possible  
Scope: verification of the five claims requested from `customer-voice-r1-1.md`, `vertical-entrants-r1-lead.md`, and `auto-parts-wedge-r1-lead.md`

## Outcome summary

| Ref | Load-bearing claim | Outcome | Confidence after check |
|---|---|---|---|
| A | Russian medical organizations procure voice robots/AI and separately buy support | **unverified (strongly corroborated)** | medium; EIS-derived mirrors agree, but the official EIS record was not retrievable in this check |
| B | Clinic appointment/reception is already offered by multiple vendors | **verified** | high for public supply-side offers; low for production performance |
| C | Serious auto-parts selection relies on structured VIN/OEM/cross-reference catalogues and system integration | **verified** | high for the architecture requirement; medium for broad market generalization |
| D | Public evidence of an end-to-end Russian voice agent for retail parts selection is absent | **verified only as a bounded negative** | medium-low; never evidence that no product or private deployment exists |
| E | Customer pain emphasizes reliability/escalation/integration over voice naturalness | **unverified** | individual pain signals are credible, but relative priority is not established |

Counts: **verified 3, disputed 0, unverified 2, overturned 0**. Claim D counts as verified only because its wording is explicitly limited to this public search scope.

## A — Medical procurement and separately priced support

**Outcome: unverified (strongly corroborated), confidence medium.** A fresh procurement record, different from the two records cited in the digests, corroborates both parts of the claim. Gatchina Clinical Interdistrict Hospital published EIS procurement **0145300006425000117** for a “Voice robot” service in April 2025 with a total of RUB 2,348,313.33. The object breakdown separately prices six months of voice-robot support at RUB 799,999.98, software at RUB 368,313.33, and six monthly 50,000-minute traffic packages at RUB 1,180,000.02. The page also reports the auction and determination dates. Source: [Tenderer mirror of EIS procurement 0145300006425000117](https://tendery-leningradskaja-oblast.tenderer.ru/tenders/3/tender85377657.html), Tenderer / EIS data, published 2025-04-18, accessed 2026-09-11.

A second fresh record shows Ufa Emergency Hospital requesting prices in February 2026 for a voice robot that informs patients of the date of a doctor visit, with an expected procurement period through December 2026. Source: [TenderGuru mirror of EIS price request 0301300275426000107](https://www.tenderguru.ru/tender/92045278), TenderGuru / EIS data, published 2026-02-10, accessed 2026-09-11.

**Caveats:** the corroboration is another procurement record, not a customer outcome study. Tenderer and TenderGuru mirror EIS data; the official EIS page was not retrievable in this check, so the claim remains `unverified` under the requested verification standard rather than being promoted on mirror evidence alone. The Gatchina line item says “voice robot,” not LLM or generative AI. The Ufa record is a price request, not evidence of award, deployment, satisfaction, or ROI. The evidence strongly supports the working conclusion that medical organizations create procurement demand for voice-robot workflows and explicitly budget support, but primary verification is still pending.

## B — Clinic appointment/reception supply is already multi-vendor

**Outcome: verified.** Multiple independent vendor publishers currently offer materially the same clinic reception workflow:

- [Stexa clinic voice bot](https://stexa.ru/klinika/) advertises inbound reception calls, doctor selection, appointment creation in the MIS, reminders, FAQ, and integrations with Infoclinica, MedWork, 1C:Medicine, and Renovatio; current page, accessed 2026-09-11.
- [NeiroStaff MIS integration](https://neirostaff.ru/integraciya-mis) advertises voice-based booking, rescheduling, cancellation, reminders, and administrator handoff through MIS APIs; current page, accessed 2026-09-11.
- [AEREL AI administrator](https://aerel-pro.ru/) advertises 24/7 phone answering, direct booking into a clinic MIS, rescheduling/cancellation, grounded answers, and human handoff; publisher page marked © 2026, accessed 2026-09-11.
- [VitaAI clinic administrator](https://callvita.ru/) advertises inbound calls, confirmation/rescheduling/cancellation in the MIS, context-preserving escalation, and MIS/telephony integration; current page, accessed 2026-09-11.

These sources are independent companies and corroborate the existence of the offer category. They do **not** independently verify deployment counts, speed, conversion, error rates, compliance, or customer satisfaction. The verified conclusion is supply-side crowding, not equivalence of product quality.

## C — Auto-parts selection is a structured-data and integration problem

**Outcome: verified.** TecAlliance’s current first-party product documentation describes TecDoc as standardized product, vehicle, and vehicle-to-part linkage data. It supports VIN, part-number, and OE-number search; its web service returns structured product, vehicle, compatibility, and linkage data in real time and is designed for direct integration with webshops, ERP systems, and business applications. Source: [TecAlliance — TecDoc catalogue and integration solutions](https://www.tecalliance.net/products?family=tecdoc&solution=catalogue-ecommerce), TecAlliance, current page, accessed 2026-09-11.

An independent catalogue provider’s technical documentation corroborates the same data model: Laximo search accepts VIN/Frame context, article/brand/OEM identifiers, vehicle applicability, and OEM cross/replacement resolution through API operations. Source: [Laximo — parts search API](https://doc.laximo.ru/ru/uss/search_functions), Laximo, updated 2026-07-06, accessed 2026-09-11.

**Caveats:** this verifies the architectural premise, not the quality of every catalogue or a claim that TecDoc/Laximo are mandatory vendors. Catalogue coverage, freshness, ambiguous VINs, supplier stock, price, and margins remain separate operational data problems. “Structured catalogue + system integration” is necessary evidence-backed plumbing; it does not by itself guarantee correct selection.

## D — Bounded negative for an end-to-end Russian retail-parts voice agent

**Outcome: verified only as a scoped public-evidence statement.** The fresh check searched the indexed Russian-language web on 2026-09-11 for combinations of “voice agent/voice robot/AI operator,” auto parts, VIN, OEM, cross-numbers, 1C, and public implementation cases. For this check, an exact match required all of the following to be publicly evidenced: a Russian product or customer case; synchronous telephone-style voice dialogue; parts selection by VIN/OEM/cross-reference; live supplier/stock/price or margin-aware alternatives; and order creation.

No exact match was found. The search did surface important adjacent products that narrow the negative:

- [“ДЕТАЛЬный Поиск”](https://vin-zapchasti.ru/) is a Russian Telegram/MAX bot that accepts a voice message, uses VIN to return an OEM number and analogues, and links to Yandex Market. Its own page says direct distributor integration is still in progress. This is voice **input** plus catalogue selection, not a synchronous phone agent and not an integrated retailer order flow.
- [XEME](https://xeme.ru/) publicly describes a Russian beta AI platform for VIN/OEM/cross-number search with REST integration for 1C and CRM, but its public page does not evidence a voice channel or production retail order workflow.
- [Lumina AI’s Auto Parts B2B case](https://lumina-ai.club/case/ford-ua) describes a Telegram workflow with VIN decoding, stock checks, 1C synchronization, alternatives, and order formation, but it is a Ukrainian, text-first vendor case rather than a Russian telephone voice agent; outcome metrics are vendor claims.

**Required wording:** “In a scoped public-web check, no well-evidenced Russian synchronous voice-call agent was found that completes retail auto-parts selection and ordering across VIN/OEM/crosses, availability, alternatives, and transaction creation.” This is **not proof of market absence**. Private deployments, stealth products, poorly indexed pages, closed tender documents, and products described under different terminology may exist. The newly found voice-input bot also means the broader wording “no voice-based parts-selection product” would be false or at least misleading.

## E — Reliability/escalation/integration versus voice naturalness

**Outcome: unverified.** A fresh, independent customer-review source supports the presence of the operational pains but not their claimed relative priority. A March 23, 2026 MTS Exolve review says the system required two script revisions before becoming stable, while also praising human-like speech, interruption handling, escalation of complex cases, and creation of a work request. Source: [Startpack — MTS Exolve reviews](https://startpack.ru/application/exolve/reviews), Startpack, review dated 2026-03-23, accessed 2026-09-11 (the full page returned HTTP 403 during open, so evidence is limited to the indexed excerpt).

There is also counterweight against treating naturalness as incidental: a 2025 peer-reviewed experiment found that participants expected human-like robot voices and that a natural voice affected perceived safety and compliance, although it did not show a direct effect on trust. Source: [Becker et al., “Influence of Robots’ Voice Naturalness on Trust and Compliance”](https://openresearch.surrey.ac.uk/esploro/outputs/journalArticle/Influence-of-Robots-Voice-Naturalness-on/991115568402346), *ACM Transactions on Human-Robot Interaction*, published 2025-01-28.

Neither source measures purchase criteria among Russian clinic or contact-center buyers, and neither supports a frequency comparison. The defensible rewrite is: **“Public evidence identifies reliability, grounded completion, human escalation, integration, and post-sale support as recurring pains; it does not establish that buyers rank them above voice naturalness.”** A systematic buyer interview set, tender-requirement coding exercise, or representative review corpus is still required for the comparative claim.

## Decision implications

1. The medical vertical has real budgeted demand, but clinic reception/appointment automation is visibly crowded at the offer level. A wedge needs narrower operational proof than “natural voice + booking + MIS integration.”
2. Auto-parts remains less publicly served in synchronous phone voice, but adjacent Russian catalogue agents and voice-input bots already exist. The defensible gap is the complete operational loop, not AI-assisted selection itself.
3. Position reliability, escalation, integration, and support as evidence-backed pain areas, but do not claim they dominate naturalness without primary buyer research.
