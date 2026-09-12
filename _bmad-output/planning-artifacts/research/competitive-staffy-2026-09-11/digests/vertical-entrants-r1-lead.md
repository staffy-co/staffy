# Vertical entrants and pricing — round 1 (lead)

Accessed: 2026-09-11

## Findings

1. **Claim:** Replica publishes current self-service-like packages at 29,900 RUB/month for 650 minutes, 49,900 RUB/month for 1,150 minutes, and custom scaling from 79,900 RUB/month / 2,000 minutes; all tiers claim a voice agent, CRM autonomy, REST API, analytics, support and up to 100 parallel calls. This creates a concrete public price anchor of roughly 30–46 RUB/minute including the managed product layer.
   - Source: https://thereplica.ru/
   - Publisher: Replica
   - Pub date: current page, accessed 2026-09-11
   - Confidence: high for advertised pricing; medium for included-capability delivery
   - Class: pricing

2. **Claim:** Yandex AI Studio prices voice agents as separate input-audio, output-audio, model-token and tool-invocation components. Current Russian pricing lists 0.0264 RUB per input-audio second and 0.0203 RUB per output-audio second including VAT, before model text generation and tools. This is an infrastructure cost floor, not a managed-agent retail price.
   - Source: https://aistudio.yandex.ru/ru/docs/ai-studio/pricing
   - Publisher: Yandex AI Studio
   - Pub date: current documentation, accessed 2026-09-11
   - Confidence: high
   - Class: pricing

3. **Claim:** Myrolit currently markets a purpose-built clinic voice agent that books appointments, answers service/preparation questions from clinic knowledge, integrates directly with Altegio when available and otherwise exports structured call results to a MIS or webhook. It is therefore a direct vertical competitor to the proposed medical wedge.
   - Source: https://myrolit.com/ru/resheniya/kliniki/
   - Publisher: Myrolit
   - Pub date: 2026-08-10
   - Confidence: high for advertised offer; low for outcome claims without customer verification
   - Class: vertical

4. **Claim:** Fromtech publicly advertises LLM-based inbound voice automation for a medical-clinic network, including appointment booking/rescheduling/cancellation, result-status handling and real-time CRM/schedule integration. This shows the medical workflow is already served by established conversational-AI integrators, although the named client and outcome methodology require verification.
   - Source: https://www.fromtech.ru/customer-service/
   - Publisher: Fromtech
   - Pub date: current page, accessed 2026-09-11
   - Confidence: medium
   - Class: vertical

5. **Claim:** Stexa publicly advertises a broad voice-agent platform with phone, Telegram and web channels, BYO-MCP CRM/ERP integrations, RAG, live handoff, campaigns, conversation QA, audit logs and clinic-style reactivation/wait-list workflows. It is a candidate direct product competitor, not present in the source chat.
   - Source: https://stexa.ru/
   - Publisher: Stexa
   - Pub date: current page, accessed 2026-09-11
   - Confidence: medium pending product/company qualification
   - Class: feature

6. **Claim:** A 2026 vendor case page from Polygant describes a voice AI agent for MedScan integrated with telephony, schedules, CRM and internal knowledge. It is evidence that integrators are already selling the exact clinic workflow, but not independent evidence of deployment success.
   - Source: https://polygant.ru/cases/golosovoj-ii-agent-dlya-medskan/
   - Publisher: Polygant
   - Pub date: 2026-08-19
   - Confidence: low-to-medium
   - Class: vertical

## Leads

- The medical-clinic wedge is not greenfield: Myrolit, Fromtech, Polygant and Replica all target parts of it.
- Investigate a narrower clinic subproblem where correctness, MIS integration and operational service create defensibility, or test a different vertical.
- Search specifically for auto-parts/catalog-selection agents; absence would be meaningful only after checking vendors, integrators and tender/case sources.
- Compare Yandex infrastructure cost to managed retail prices only after modeling speech share, LLM/tool charges, telephony and support; raw audio price alone is not unit economics.

## Contradictions and negative findings

- The initial proposed differentiator “medical appointment booking with a natural voice and CRM/MIS integration” is already advertised by multiple vendors.
- No independent customer-side confirmation was found in this pass for Myrolit, Fromtech, Stexa or Polygant performance outcomes.
