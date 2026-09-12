# Auto-parts wedge — round 1 (lead)

Accessed: 2026-09-11

## Findings

1. **Claim:** Myrolit already markets a voice agent for auto-service reception that takes repair bookings, captures vehicle/problem details and writes results to CRM, but the public offer does not claim deep parts selection by VIN/OEM/cross-number.
   - Source: https://myrolit.com/ru/resheniya/avtoservis/
   - Publisher: Myrolit
   - Pub date: 2026-08-10
   - Confidence: high for offer scope
   - Class: vertical

2. **Claim:** Stexa markets a voice agent specifically for auto-service workflows, with advertised out-of-box support for 1C:Auto Service, amoCRM, Bitrix24, AutoCRM and Trinion and scheduling/price logic. This is a direct competitor for service booking, but its public description does not establish production-grade retail parts selection.
   - Source: https://stexa.ru/avtoservis/
   - Publisher: Stexa
   - Pub date: current page, accessed 2026-09-11
   - Confidence: medium; vendor capability claim
   - Class: vertical

3. **Claim:** A current 1C partner case demonstrates that serious auto-parts selection is a systems-integration problem, not simply RAG: a catalogue above two million positions with VIN/model/year, analogues and cross-numbers remained in an external MS SQL system while a dedicated 1C workstation queried it and handled selection/cross-sell logic.
   - Source: https://consulting.1c.ru/cases/396945.html
   - Publisher: 1C Consulting / 1C partner case
   - Pub date: current case, accessed 2026-09-11
   - Confidence: high for described architecture; medium for generalization
   - Class: workflow

4. **Claim:** Adjacent products already automate parts selection in non-voice channels: Quvio advertises an auto-parts CRM with catalogue-based availability/analogue answers, and Yandex/TEBOIL publicly describe an AI text agent selecting lubricant specifications across 140+ products. Staffy would compete with existing vertical software and chat agents as well as voice-agent vendors.
   - Sources: https://quvio.ru/crm-dlya-avtozapchastej.html ; https://teboil.ru/news/teboil-predstavil-ii-agenta-po-smazochnym-materialam/
   - Publishers: Quvio; TEBOIL
   - Pub dates: current / 2026-04-09
   - Confidence: medium
   - Class: adjacent_competition

5. **Claim:** This search found no clear, well-evidenced Russian product publicly offering end-to-end voice-based retail auto-parts selection through VIN/OEM/cross-numbers, supplier availability, margin-aware alternatives and order creation. This is absence of public evidence, not proof of an empty market.
   - Sources searched: Myrolit, Stexa, 1C cases, Quvio, TEBOIL and broad Russian web queries
   - Publisher: multiple
   - Pub date: accessed 2026-09-11
   - Confidence: medium as a scoped negative finding
   - Class: gap

## Leads

- Deepen the auto-parts wedge through interviews and product trials, not another broad web pass.
- Treat catalogue correctness, VIN disambiguation, supplier/stock/price orchestration, auditable recommendations and human escalation as the potential product moat.
- A minimal pilot should constrain itself to one catalogue/ERP topology and one transaction path; “any 1C” is not credible.

## Contradictions and negative findings

- “Load the catalogue into Qdrant and let the LLM choose an analogue” conflicts with evidence that operational catalogues have millions of changing records and structured compatibility logic.
- Auto-service appointment booking is already crowded; retail parts selection may be less publicly served, but evidence remains insufficient to call it a market gap.
