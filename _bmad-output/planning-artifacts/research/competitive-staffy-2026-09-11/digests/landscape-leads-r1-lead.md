# Landscape leads — round 1 (lead)

Accessed: 2026-09-11

## Findings

1. **Claim:** Yandex AI Studio is now a direct platform-level alternative for building Russian-language realtime voice agents, not merely an STT/TTS supplier: its current documentation describes bidirectional audio/text sessions over WebSocket, tool calls, streaming output, interruption support, SpeechKit voices, memory and search integrations.
   - Source: https://aistudio.yandex.ru/ru/docs/ai-studio/concepts/agents/realtime
   - Publisher: Yandex AI Studio
   - Pub date: current documentation, accessed 2026-09-11
   - Confidence: high
   - Class: feature

2. **Claim:** Yandex announced the no/low-code voice-agent interface on 2026-05-28 and explicitly targets contact-center use cases such as product consultation, call summarization and appointment booking, for both SMB and enterprise; this directly weakens any Staffy differentiation based only on “prompt + tools + Russian realtime voice.”
   - Source: https://yandex.ru/company/news/28-05-2026-04
   - Publisher: Yandex
   - Pub date: 2026-05-28
   - Confidence: high for product claim; medium for adoption claim because the evidence is vendor-authored
   - Class: positioning

3. **Claim:** Yandex's current solution page claims a complete realtime pipeline under one second, corporate-system access, thousands of simultaneous calls and MCP/search/memory features. These are vendor claims that require independent validation before they can be used in a Staffy comparison.
   - Source: https://aistudio.yandex.ru/ru/solutions/voice-agent
   - Publisher: Yandex AI Studio
   - Pub date: current page, accessed 2026-09-11
   - Confidence: medium
   - Class: feature

4. **Claim:** Voxagent publicly documents a configurable voice-agent platform with WebRTC, web widget, browser calling, telephony through Twilio/Voximplant, and LiveKit-based sessions. It is a candidate direct competitor, but its Russian-market status, hosting and compliance posture remain unverified.
   - Source: https://docs.voxagent.ru/ru/features
   - Publisher: Voxagent
   - Pub date: current documentation, accessed 2026-09-11
   - Confidence: medium
   - Class: feature

5. **Claim:** Search surfaced additional candidate competitors/adjacent services — Vocamate AI, Replica, PapAI Soft, Dasha and ThunderPhone — but their Russian legal presence, customer base and direct comparability to Staffy are not yet established.
   - Sources: https://vocamate.ru/ ; https://thereplica.ru/ ; https://papaisoft.com/ ; https://dasha.ai/ru-ru ; https://thunderphone.com/ru
   - Publishers: respective vendors
   - Pub date: current pages, accessed 2026-09-11
   - Confidence: low pending qualification
   - Class: competitor_discovery

6. **Claim:** Yandex's SIP connection was still Preview at the 2026-05-28 launch and required assistance from Yandex Cloud support. Therefore “easy SIP connection” existed as a direction, but self-service production readiness was not established by the announcement.
   - Source: https://yandex.ru/company/news/28-05-2026-04
   - Publisher: Yandex
   - Pub date: 2026-05-28
   - Confidence: high
   - Class: integration

7. **Claim:** Replica is a direct Russian-market competitor positioned as a managed “digital employee” for first-line sales and service. Its current site claims CRM task execution, voice-to-voice latency under one second, AI supervisors, rapid website-derived demos, Yandex Cloud hosting in Russia, and names customer examples in restaurants, manufacturing and optical clinics. Performance figures remain vendor claims pending independent evidence.
   - Source: https://thereplica.ru/
   - Publisher: Replica
   - Pub date: current page, accessed 2026-09-11
   - Confidence: high for positioning and advertised feature set; low for performance/accuracy/conversion figures
   - Class: positioning

## Leads

- Treat Yandex AI Studio as a mandatory direct/platform competitor in the final report.
- Treat Replica as a mandatory direct managed-service competitor; qualify Voxagent as a possible platform competitor.
- Check Yandex voice-agent pricing, telephony integration path, customer evidence and data-residency terms.
- Test whether Staffy's defensible layer must sit above foundation models: vertical workflow packs, integration reliability, evaluation/guardrails, deployment and managed outcomes.

## Contradictions and negative findings

- The source chat framed Yandex primarily as STT/LLM/TTS infrastructure. Current first-party evidence shows it has moved up-stack into configurable realtime voice agents.
- No independent evidence was found in this broad pass for Yandex's “thousands of calls” scalability or for production adoption beyond vendor-described pilots.
