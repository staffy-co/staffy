# Offer, feature, pricing & packaging teardown — российские voice agents

Дата исследования: 2026-09-11  
Раунды: 2 (широкий официальный срез; затем только пробелы/противоречия)  
Источники: 16 distinct URLs, все официальные страницы продуктов, документация, release notes или официальные PDF.  
Ограничение свежести: функции и цены должны быть не старше 3 месяцев. Для live-страниц без даты публикации указано `pub_date=undated (live page; freshness unverified)`; они подтверждают то, что страница показывает на дату доступа, но не дату появления функции. Источники старше трёх месяцев помечены явно.

## Decision-relevant synthesis

### S1

- claim: Базовые признаки «современного» voice agent — LLM/AI-agent, ответы по базе знаний (RAG/knowledge base), перебивание и действия через API/MCP — уже публично заявлены несколькими независимыми российскими поставщиками. Поэтому ни LLM, ни RAG, ни barge-in, ни сам по себе MCP/tool use не являются защищаемым преимуществом Staffy; преимущество должно лежать в вертикальном workflow, качестве исполнения и доказуемом бизнес-результате.
- source URL: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/ ; https://retail.mango-office.ru/products/contact-center/ai/voice-robot/ ; https://ziax.ru/ ; https://slsoft.ru/products/robovoice/
- publisher: МТС Exolve; MANGO OFFICE; ZIAX; SL Soft
- pub_date: undated live pages; freshness unverified
- accessed: 2026-09-11
- confidence: high (independent official vendors agree on category availability; exact performance is not verified)
- class: feature

### S2

- claim: Публичные цены распадаются на две категории: сценарные/NLU-боты примерно 3,5–6,5 ₽ за минуту при пакетах или предоплате (Aimylogic, MANGO, Tomoru, Exolve), а более LLM-native managed agent с автономной CRM-работой стоит на порядок дороже — у «Реплики» 30–46 ₽/мин плюс 20 000 ₽ за подключение. Это показывает, что рынок уже монетизирует не только минуты речи, но и внедрение/операционную ответственность.
- source URL: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/ ; https://static.mango-office.ru/products/contact-center/tariffs/ ; https://tomoru.ru/price ; https://aimylogic.com/ru/price ; https://thereplica.ru/
- publisher: МТС Exolve; MANGO OFFICE; Tomoru; Just AI/Aimylogic; Реплика
- pub_date: undated live price pages; live as accessed, publication dates unavailable
- accessed: 2026-09-11
- confidence: high for displayed prices; medium for like-for-like comparability because bundles and telephony differ
- class: pricing

### S3

- claim: On-premise is already an explicit enterprise option у TWIN, Robovoice и ZIAX; cloud-only/localized hosting is offered by other vendors. Therefore «российский контур» alone is not a moat, although packaged compliance and deployment speed can still be a buying criterion.
- source URL: https://twin24.ai/products/large-business/ ; https://slsoft.ru/products/robovoice/ ; https://ziax.ru/
- publisher: TWIN; SL Soft; ZIAX
- pub_date: undated live pages; freshness unverified
- accessed: 2026-09-11
- confidence: high (three independent official product pages)
- class: deployment

### S4

- claim: The clearest white space is not generic calling but a narrow vertical agent that completes a transaction in the system of record and ships with prebuilt integrations, policies, evaluation and escalation. Exolve and ZIAX already expose MCP; «Реплика» already packages CRM actions; Yandex AI Studio exposes Realtime API plus MCP/search. A defensible wedge must therefore be deeper than a connector catalog—e.g. proprietary workflow/evaluation data, vertical outcome guarantees, or operational distribution.
- source URL: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/ ; https://ziax.ru/ ; https://thereplica.ru/ ; https://aistudio.yandex.ru/ru/solutions/voice-agent
- publisher: МТС Exolve; ZIAX; Реплика; Yandex AI Studio
- pub_date: undated live pages; freshness unverified
- accessed: 2026-09-11
- confidence: high for capability saturation; medium for strategic inference
- class: integration

## Vendor findings

## МТС Exolve

### EX-1

- claim: Exolve sells a five-step ladder: template call 0.21 ₽/min, personalized notification 1.20 ₽/min, interactive conversation 2.75 ₽/min, NLP robot 4.75 ₽/min, and AI-agent robot 6.50 ₽/min, all including VAT. The current product page says outbound total cost can also include telecom traffic, authorized-call labeling and add-ons.
- source URL: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/
- publisher: МТС Exolve
- pub_date: undated live price/product page; live as accessed, freshness unverified
- accessed: 2026-09-11
- confidence: high
- class: pricing

### EX-2

- claim: The product page explicitly assigns context and interruption understanding to NLP/AI tiers and knowledge-base answers to the AI-agent tier. It does not name the underlying LLM or publish a conversational latency/SLA, so “AI-agent” is documented packaging but not proof of a realtime LLM-native stack.
- source URL: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/
- publisher: МТС Exolve
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: high for packaged capabilities; medium for architectural interpretation
- class: feature

### EX-3

- claim: Exolve documents SIP trunk/its own virtual PBX, REST API, webhooks and MCP; named business systems include Bitrix24, amoCRM, helpdesk and 1C ERP. The described flows can create a CRM ticket, transfer to a human with context, send SMS and save campaign results.
- source URL: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/
- publisher: МТС Exolve
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: high for interfaces and named systems; medium that MCP is available inside every voice-agent plan
- class: integration

### EX-4

- claim: The public deployment is on MTS infrastructure with data stored on MTS servers in Russia; no on-premise voice-robot option was found in the searched official materials.
- source URL: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/
- publisher: МТС Exolve
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: high for cloud/localization; medium for absence of on-prem (absence-of-evidence only)
- class: deployment

## MANGO OFFICE

### MO-1

- claim: MANGO separates a scenario-based “Digital Employee” (NLU, templates, no-code) from a custom “AI Employee” (LLM, nonlinear dialog, autonomous AI agents and RAG over the customer knowledge base). The LLM/RAG claim is explicit on the live product page but is vendor marketing, not backed by public API docs or a latency benchmark found in this run.
- source URL: https://retail.mango-office.ru/products/contact-center/ai/voice-robot/
- publisher: MANGO OFFICE
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: medium
- class: feature

### MO-2

- claim: The official contact-center tariff page lists “Robots” at 4.5 ₽/min, “Multichannel robot” at 1,000 ₽/month, and open API at 8,000 ₽/month in applicable contact-center packages. The page does not explicitly map the 4.5 ₽ rate to the custom LLM “AI Employee”; that higher product should be treated as quote-based until a proposal says otherwise.
- source URL: https://static.mango-office.ru/products/contact-center/tariffs/
- publisher: MANGO OFFICE
- pub_date: undated live tariff page; live as accessed, freshness unverified
- accessed: 2026-09-11
- confidence: high for listed prices; medium for package mapping
- class: pricing

### MO-3

- claim: MANGO advertises 300+ integrations and names e-staff, iiko, YCLIENTS, Bitrix24, amoCRM and RetailCRM. The product is designed to sit inside MANGO’s virtual PBX/omnichannel contact center; API and CRM process triggers are part of the wider suite.
- source URL: https://retail.mango-office.ru/products/contact-center/ai/voice-robot/
- publisher: MANGO OFFICE
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: medium (official marketing count; not independently verified)
- class: integration

### MO-4

- claim: No explicit barge-in behavior, end-to-end realtime latency, on-premise deployment, or agent tool-calling protocol was found on the two searched official MANGO pages. RAG and autonomous scenarios are claimed, but the operational boundaries of those agents remain undocumented publicly.
- source URL: https://retail.mango-office.ru/products/contact-center/ai/voice-robot/ ; https://static.mango-office.ru/products/contact-center/tariffs/
- publisher: MANGO OFFICE
- pub_date: undated live pages; freshness unverified
- accessed: 2026-09-11
- confidence: medium (bounded absence in searched materials)
- class: feature

## Tomoru

### TO-1

- claim: Usage pricing is prepaid and volume-tiered: 6.4, 6.2, 6.0, 5.9, 5.8, 5.4, 4.9 and 4.4 ₽/min as prepayment rises from up to 9,900 ₽ through 19,900/29,900/39,900/59,900/99,900/150,000 to 150,000+ ₽. Billing is per second and excludes telephony.
- source URL: https://tomoru.ru/price
- publisher: Tomoru
- pub_date: undated live price page (footer ©2024 is not a publication date); live as accessed, freshness unverified
- accessed: 2026-09-11
- confidence: high
- class: pricing

### TO-2

- claim: Tomoru separately charges for managed setup: Fast Track 79,000 ₽ for one business task/up to 10 handled questions; Pro 159,000 ₽ for two tasks/up to 20 questions; “CEO” 299,000 ₽ for an individual solution with unlimited question handling and A/B tests. API integration appears in the package comparison.
- source URL: https://tomoru.ru/plan
- publisher: Tomoru
- pub_date: undated live plan page; live as accessed, freshness unverified
- accessed: 2026-09-11
- confidence: high
- class: pricing

### TO-3

- claim: Tomoru documents API integration into customer CRM, telephony and other software, and says the service runs on certified Russian servers. No on-premise option was found in the searched official pages.
- source URL: https://tomoru.ru/premium
- publisher: Tomoru
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: high for API/cloud; medium for absence of on-prem
- class: deployment

### TO-4

- claim: Tomoru’s official marketing page demonstrates/claims the robot can interrupt a speaker and describes script/Q&A enrichment from prior conversations. It does not explicitly document an LLM, RAG pipeline, tool-calling interface or realtime latency, so “AI/self-learning” should not be equated with a generative realtime agent without a technical demo.
- source URL: https://tomoru.ru/premium
- publisher: Tomoru
- pub_date: undated live product/marketing page; freshness unverified
- accessed: 2026-09-11
- confidence: medium for barge-in claim; high for the documentation gap
- class: feature

## TWIN

### TW-1

- claim: TWIN offers both TWIN.Cloud on its Selectel-hosted servers (Basic/Comfort/Full configurations) and TWIN.On-Premise in the customer perimeter (Branch, Standard for contact centers from 150 operators/shift, Enterprise from 200 operators/shift).
- source URL: https://twin24.ai/products/large-business/
- publisher: TWIN
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: high
- class: deployment

### TW-2

- claim: A 2026-05-13 official release note says chat AI agents received RAG “by analogy with voice agents,” and the same release updated AI-agent interruption behavior and automatic retry of LLM requests. This is the strongest public evidence found that TWIN voice agents already had RAG and barge-in, but it is about four months old and therefore outside the requested 3-month feature window.
- source URL: https://wiki.twin24.ai/ru/history/13052026
- publisher: TWIN
- pub_date: 2026-05-13 (stale: >3 months)
- accessed: 2026-09-11
- confidence: medium (official release note, stale by requested window)
- class: feature

### TW-3

- claim: TWIN’s public large-business page names integration categories (CRM, telephony, messengers, social networks, chat platforms) but publishes no numerical Cloud or On-Premise price. Packaging is sales-led and configuration-dependent; no current public ₽/minute rate was found.
- source URL: https://twin24.ai/products/large-business/
- publisher: TWIN
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: medium
- class: pricing

## Just AI / Aimylogic

### JA-1

- claim: Aimylogic is a low-code scripted voice-bot platform with separate call packages. Current displayed monthly prices are: Free 25 min/0 ₽; Calls Startup 200 min/1,089 ₽; Growth 1,000 min/4,290 ₽; Optimal 3,000 min/10,890 ₽; Premium 5,000 min/17,490 ₽. Additional minutes start at 2.58 ₽; included-minute effective rates are approximately 5.45, 4.29, 3.63 and 3.50 ₽/min for the paid tiers.
- source URL: https://aimylogic.com/ru/price
- publisher: Just AI / Aimylogic
- pub_date: undated live price page; live as accessed, freshness unverified
- accessed: 2026-09-11
- confidence: high
- class: pricing

### JA-2

- claim: Call packages meter speech recognition/synthesis/NLU minutes and include plan-dependent quotas of ChatGPT requests (2,000/4,000/8,000/20,000 in Startup/Growth/Optimal/Premium). This evidences generative capability inside the scenario platform, but not an end-to-end realtime voice-to-voice agent or RAG.
- source URL: https://help.cloud.just-ai.com/aimylogic/overview/pricing/
- publisher: Just AI Conversational Cloud documentation
- pub_date: undated live documentation; freshness unverified
- accessed: 2026-09-11
- confidence: high for quotas; medium for architectural interpretation
- class: feature

### JA-3

- claim: Aimylogic now requires the customer’s own SIP connection for inbound and outbound calling; any provider can be configured, multiple SIP connections are supported, and billing pauses during transfer to a human. Telecom traffic is therefore a separate supplier cost.
- source URL: https://help.cloud.just-ai.com/aimylogic/smart_calls/sip_settings/
- publisher: Just AI Conversational Cloud documentation
- pub_date: undated live documentation; freshness unverified
- accessed: 2026-09-11
- confidence: high
- class: integration

### JA-4

- claim: The public Aimylogic materials searched do not document voice barge-in, sub-second latency, RAG, MCP/tool calling or an Aimylogic on-premise edition. Public integration evidence is strongest for SIP and conventional API/channel connections, not autonomous action execution.
- source URL: https://aimylogic.com/ru/price ; https://help.cloud.just-ai.com/aimylogic/smart_calls/sip_settings/
- publisher: Just AI / Aimylogic
- pub_date: undated live pages; freshness unverified
- accessed: 2026-09-11
- confidence: medium (bounded absence in searched materials)
- class: feature

## Robovoice (SL Soft)

### RV-1

- claim: Robovoice is a no-code platform for voice and text bots with inbound calls and mass outbound campaigns. The live page explicitly names NLU, interruption/barge-in and GPT; an official PDF additionally names LLM and RAG.
- source URL: https://slsoft.ru/products/robovoice/ ; https://slsoft.ru/upload/iblock/d63/5yi2m1cza3721eq8vv0b1lp2nr8ywt1v.pdf
- publisher: SL Soft
- pub_date: live page undated; PDF approximately 2026-05 (indexed ~4 months before access; stale >3 months)
- accessed: 2026-09-11
- confidence: medium (same-publisher corroboration; PDF stale; no independent technical test)
- class: feature

### RV-2

- claim: Robovoice supports on-cloud and on-premise deployments, and the official page claims 1,000+ simultaneous dialogs. Realtime on the page refers to analytics dashboards, not a published conversational latency.
- source URL: https://slsoft.ru/products/robovoice/
- publisher: SL Soft
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: high for deployment; medium for concurrency marketing claim
- class: deployment

### RV-3

- claim: Ready connectors are documented for Bitrix24 and amoCRM; bots can trigger calls from CRM events and update CRM records. Broader API/tool-calling specifics were not found on the searched page.
- source URL: https://slsoft.ru/products/robovoice/
- publisher: SL Soft
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: high for named connectors; medium for the bounded gap
- class: integration

### RV-4

- claim: Packaging uses per-second billing with no minimum threshold and includes a development/testing minute bundle, but no current public numerical ₽/minute or on-premise license price was found.
- source URL: https://slsoft.ru/products/robovoice/
- publisher: SL Soft
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: medium
- class: pricing

## ZIAX

### ZX-1

- claim: ZIAX explicitly offers two modes—NLU and generative AI/LLM—with a multi-agent architecture. The live page names RAG over customer documents, A2A/MCP, and agents using MCP to obtain data from external systems.
- source URL: https://ziax.ru/
- publisher: ZIAX (ГК САТЕЛ)
- pub_date: undated live product page; footer ©2025 is not a publication date; freshness unverified
- accessed: 2026-09-11
- confidence: medium (official marketing page; no separate current API/release-note verification found)
- class: feature

### ZX-2

- claim: ZIAX supports cloud and on-premise deployment, records/transcribes calls, and documents CRM/external-system integration through API. One robot can serve voice and text channels; voice supports inbound and outbound calls.
- source URL: https://ziax.ru/
- publisher: ZIAX (ГК САТЕЛ)
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: high
- class: deployment

### ZX-3

- claim: ZIAX’s page shows a task flow that calls an internal booking system, normalizes entities, completes the transaction and continues the dialog. This is stronger evidence of workflow execution than a generic “API integration” badge, though no public error-handling, authorization or tool-policy documentation was found.
- source URL: https://ziax.ru/
- publisher: ZIAX (ГК САТЕЛ)
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: medium
- class: integration

### ZX-4

- claim: No current public numerical SaaS, per-minute or on-premise license price was found for ZIAX; commercial packaging appears quote-based. A public pricing-policy PDF surfaced in search, but its searchable extract described license units rather than usable current amounts, so it was not used as price evidence.
- source URL: https://ziax.ru/
- publisher: ZIAX (ГК САТЕЛ)
- pub_date: undated live page; freshness unverified
- accessed: 2026-09-11
- confidence: medium
- class: pricing

## Additional truly relevant LLM-native competitor: «Реплика»

### RP-1

- claim: «Реплика» is a direct managed voice-agent competitor, not merely a speech/API component. It packages inbound/outbound calls, a text agent, CRM autonomy, up to 100 parallel calls, REST API, analytics and ongoing agent training in every tier.
- source URL: https://thereplica.ru/
- publisher: Реплика
- pub_date: undated live product/price page (footer ©2026); live as accessed, freshness unverified
- accessed: 2026-09-11
- confidence: medium-high (official current page; no independent product test)
- class: feature

### RP-2

- claim: Current displayed pricing is 29,900 ₽/month for 650 min (46 ₽/min), 49,900 ₽ for 1,150 min (40 ₽/min), and from 79,900 ₽ for 2,000+ min (30–39 ₽/min), plus a one-time 20,000 ₽ integration fee covering agent training, CRM and telephony setup.
- source URL: https://thereplica.ru/
- publisher: Реплика
- pub_date: undated live price page; live as accessed, freshness unverified
- accessed: 2026-09-11
- confidence: high
- class: pricing

### RP-3

- claim: The product page claims voice-to-voice latency below one second, interruption handling, product knowledge from uploaded materials, persistent customer memory and in-call actions (create/update CRM lead, send SMS, book a slot, transfer a call). These are differentiator-critical claims but are supported only by the vendor’s own page, so performance remains unverified.
- source URL: https://thereplica.ru/
- publisher: Реплика
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: medium for capability presence; low for latency/quality performance
- class: feature

### RP-4

- claim: «Реплика» advertises 200+ integrations and names Bitrix24, amoCRM, 1C, YCLIENTS, MANGO, UIS, RetailCRM, Megaplan, Telfin and major telecoms. Its Russian deployment is a protected Yandex Cloud environment; no on-premise option is shown.
- source URL: https://thereplica.ru/
- publisher: Реплика
- pub_date: undated live product page; freshness unverified
- accessed: 2026-09-11
- confidence: medium (official marketing count; no independent verification)
- class: integration

## Adjacent LLM-native platform threat: Yandex AI Studio Voice Agent

### YA-1

- claim: Yandex AI Studio now markets a Voice Agent built on Realtime API/WebSocket with a stated sub-second ready pipeline, MCP integrations, file/web search, short-term memory and access to CRM/knowledge bases. This is a credible build-platform substitute for technically capable Russian buyers, but the page does not establish a turnkey telephony/contact-center package.
- source URL: https://aistudio.yandex.ru/ru/solutions/voice-agent
- publisher: Yandex AI Studio
- pub_date: undated live solution page; freshness unverified
- accessed: 2026-09-11
- confidence: medium for features; low for the latency claim until independently benchmarked
- class: feature

### YA-2

- claim: No packaged SIP/PBX integration, managed vertical workflow or all-in per-minute price was found on the voice-agent solution page. Its competitive role is infrastructure/platform rather than a fully managed digital employee.
- source URL: https://aistudio.yandex.ru/ru/solutions/voice-agent
- publisher: Yandex AI Studio
- pub_date: undated live solution page; freshness unverified
- accessed: 2026-09-11
- confidence: medium (bounded absence on the searched page)
- class: pricing

## Capability matrix (evidence level, not vendor score)

Legend: `D` = explicitly documented on official product/docs/release page; `M` = official marketing claim without technical docs/test; `—` = not found in searched official material, not proof of absence; `stale` = evidence older than 3 months.

| Vendor | LLM/generative | RAG/KB | Barge-in | Realtime latency | Tool/action use | Delivery |
|---|---|---|---|---|---|---|
| МТС Exolve | AI-agent named; underlying LLM — | D: knowledge base | D | — | D: REST/webhooks/MCP; plan scope unclear | MTS cloud/RU |
| MANGO | M: explicit LLM | M: explicit RAG | — | — | M: autonomous scenarios; conventional CRM/API documented | cloud suite; on-prem — |
| Tomoru | — (AI/NLU marketing) | Q&A base, not explicit RAG | M: demo claim | — | API integration, autonomous tool use — | RU cloud; on-prem — |
| TWIN | D: LLM retry in 2026-05 release (stale) | D: voice RAG referenced in release (stale) | D: interruption update (stale) | — | — | Cloud + On-Premise |
| Aimylogic | D: metered ChatGPT calls within scenario platform | — | — | — | conventional API/SIP; MCP — | SaaS; customer SIP |
| Robovoice | M/D: GPT; stale PDF says LLM | stale official PDF says RAG | D/M on live page | — (realtime is analytics only) | CRM triggers; generic agent tools — | Cloud + On-Premise |
| ZIAX | M: explicit LLM/multi-agent | M: explicit RAG | — | streaming ASR, no dialog latency | M: MCP/A2A + booking flow | Cloud + On-Premise |
| Реплика | M: “world models” orchestration | M: customer materials/knowledge | M | M: <1 sec | M/D on product page: CRM/SMS/booking/transfer | Yandex Cloud/RU |
| Yandex AI Studio | D/M: Realtime voice agent | file/web search and KB access | — on searched page | M: <1 sec pipeline | D/M: MCP/search/CRM access | developer platform |

## Pricing/packaging matrix

| Vendor | Public run-rate | Setup / platform fee | Telephony | On-prem price |
|---|---|---|---|---|
| МТС Exolve | 0.21–6.50 ₽/min by capability; AI-agent 6.50 ₽/min incl. VAT | not found | outbound traffic/labeling/add-ons extra | not found |
| MANGO | generic robot 4.5 ₽/min | multichannel robot 1,000 ₽/mo; open API 8,000 ₽/mo in applicable CC package; LLM build quote-based | within MANGO suite / package-dependent | not found |
| Tomoru | 6.4→4.4 ₽/min by prepayment; per-second; telephony excluded | 79k / 159k / 299k ₽ managed setup | extra | not found |
| TWIN | rate not public | configuration-dependent Cloud/On-Premise | contract-dependent | quote |
| Aimylogic | 1,089/200 min; 4,290/1,000; 10,890/3,000; 17,490/5,000 ₽/mo; extra min from 2.58 ₽ | none shown for self-service | customer SIP/provider extra | not found for Aimylogic |
| Robovoice | per-second, numeric rate not public | test/dev minute bundle included | not quantified | quote |
| ZIAX | not public | project/quote | not quantified | quote |
| Реплика | 29,900/650 min; 49,900/1,150; from 79,900/2,000+ ₽/mo (30–46 ₽/min) | 20,000 ₽ one-time | setup included; traffic inclusion not explicit | not offered publicly |
| Yandex AI Studio | no all-in voice-agent price on solution page | build/integration required | no packaged SIP price found | infrastructure-dependent |

## Contradictions and ambiguities

1. **MANGO price-to-product mapping.** The generic tariff says 4.5 ₽/min, while the product page presents a custom LLM/RAG “AI Employee.” It does not say that 4.5 ₽ includes the LLM tier. Treat 4.5 ₽ as the standard robot price and request an AI-employee quote.
2. **“AI” is not a consistent technical category.** Exolve’s “AI-agent,” Tomoru’s “AI/self-learning,” Aimylogic’s ChatGPT call quota and Robovoice’s GPT block may be layered onto scenario engines. Only some pages explicitly say LLM/RAG, and none of the named incumbents published a current end-to-end latency benchmark in the searched technical material.
3. **TWIN freshness conflict.** The most specific RAG/barge-in evidence is a 2026-05-13 release note—official but outside the ≤3-month window. Capability may still exist, but current behavior must be revalidated in product.
4. **Tomoru has two additive-looking price surfaces.** Usage is 4.4–6.4 ₽/min while managed setup is 79k–299k ₽; the pages do not clearly state every combination, renewal rule or whether setup prices are one-time.
5. **“Realtime” is overloaded.** Robovoice documents realtime analytics, not conversational latency; ZIAX documents streaming ASR, not full-agent response time. Only «Реплика» and Yandex claim sub-second voice-agent latency, both without an independent benchmark in this run.
6. **Tool use is unevenly evidenced.** Exolve and ZIAX explicitly name MCP; «Реплика» lists completed CRM/SMS/booking actions; MANGO says autonomous scenarios. Public docs do not establish comparable permissions, retries, idempotency, audit or failure recovery.

## Leads worth chasing

1. Request current commercial proposals for MANGO AI Employee, TWIN Cloud/On-Premise, Robovoice and ZIAX; normalize against telephony, ASR/TTS, LLM tokens, concurrency, support and setup.
2. Run the same scripted live-call benchmark against Exolve AI-agent, MANGO AI Employee, TWIN AI Agent, Robovoice, ZIAX and «Реплика»: time-to-first-audio, interruption recovery, long-turn stability, tool-call failure, hallucination rate and human handoff with context.
3. Verify whether Exolve MCP and ZIAX MCP are callable from voice turns in production or only platform-level integration surfaces.
4. Test the likely first-vertical candidates where transaction completion matters: medical appointment/clinic front desk, service/repair dispatch, and high-intent lead qualification. «Реплика» already markets medicine, retail/e-commerce and inbound sales, so a generic “AI receptionist” wedge is crowded.
5. Other direct LLM-native leads surfaced but were not added to the scored teardown because the 16-source budget left only single marketing pages and no technical verification: Klang (klangy.ru), Velorra (velorra.ru), Vocamate AI (vocamate.ru), TOKLOG (toklog.ru). They merit a separate refresh if SMB/managed-agent competition becomes the focus.

## What was searched but not found

- A current public numerical price for TWIN Cloud or TWIN On-Premise.
- A current public numerical Robovoice per-minute or license price.
- A current public numerical ZIAX SaaS/on-premise price.
- A public MANGO price explicitly tied to its LLM/RAG “AI Employee,” rather than generic robots.
- Public current technical latency/SLA documents for Exolve, MANGO, Tomoru, TWIN, Aimylogic, Robovoice or ZIAX.
- Current technical docs proving barge-in for MANGO, Aimylogic or ZIAX.
- Current technical docs for tool execution semantics—authorization, retries, idempotency, audit and failure recovery—across the named vendors.
- Public evidence that Tomoru’s current production voice product is LLM/RAG-native rather than a scenario/NLU system with AI-assisted training.
- Aimylogic-specific on-premise packaging.
- Independent two-source validation of vendor performance/traction/market-share claims. Therefore customer counts, conversion lifts, ROI and “market leader” statements were deliberately excluded from conclusions.

## Source ledger (16 distinct URLs)

1. https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/ — МТС Exolve — undated live page — accessed 2026-09-11.
2. https://retail.mango-office.ru/products/contact-center/ai/voice-robot/ — MANGO OFFICE — undated live page — accessed 2026-09-11.
3. https://static.mango-office.ru/products/contact-center/tariffs/ — MANGO OFFICE — undated live tariff — accessed 2026-09-11.
4. https://tomoru.ru/price — Tomoru — undated live tariff (footer ©2024) — accessed 2026-09-11.
5. https://tomoru.ru/plan — Tomoru — undated live plan page — accessed 2026-09-11.
6. https://tomoru.ru/premium — Tomoru — undated live product page — accessed 2026-09-11.
7. https://twin24.ai/products/large-business/ — TWIN — undated live product page — accessed 2026-09-11.
8. https://wiki.twin24.ai/ru/history/13052026 — TWIN — 2026-05-13 (stale >3 months) — accessed 2026-09-11.
9. https://aimylogic.com/ru/price — Just AI/Aimylogic — undated live tariff — accessed 2026-09-11.
10. https://help.cloud.just-ai.com/aimylogic/overview/pricing/ — Just AI documentation — undated live documentation — accessed 2026-09-11.
11. https://help.cloud.just-ai.com/aimylogic/smart_calls/sip_settings/ — Just AI documentation — undated live documentation — accessed 2026-09-11.
12. https://slsoft.ru/products/robovoice/ — SL Soft — undated live page — accessed 2026-09-11.
13. https://slsoft.ru/upload/iblock/d63/5yi2m1cza3721eq8vv0b1lp2nr8ywt1v.pdf — SL Soft — approximately 2026-05, stale >3 months — accessed 2026-09-11.
14. https://ziax.ru/ — ZIAX — undated live page (footer ©2025) — accessed 2026-09-11.
15. https://thereplica.ru/ — Реплика — undated live page (footer ©2026) — accessed 2026-09-11.
16. https://aistudio.yandex.ru/ru/solutions/voice-agent — Yandex AI Studio — undated live solution page — accessed 2026-09-11.
