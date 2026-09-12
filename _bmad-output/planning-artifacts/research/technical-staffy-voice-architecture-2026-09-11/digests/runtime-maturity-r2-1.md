# Staffy voice architecture — runtime maturity, round 2

Scope: unresolved decision boundary between Yandex AI Studio Realtime end-to-end and a self-managed Russian component/telephony path for first paid pilots in Russia.

Method: official documentation, legal terms, release notes, and current vendor product pages; accessed 2026-09-11. Absence of a public statement is recorded as `not-found`, not treated as proof.

## Findings

### F1 — Default Realtime capacity is pilot-sized and must be raised before material traffic

- claim: AI Studio publishes a default quota of 10 concurrent Realtime voice-agent sessions and 10 session-creation requests per second. Quotas may be increased through support, while hard limits cannot. Ten concurrent sessions is enough only for a small controlled pilot; capacity approval should be a pre-launch dependency rather than an assumption.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/concepts/limits
- publisher: Yandex Cloud / AI Studio
- pub_date: undated, current documentation
- accessed: 2026-09-11
- confidence: high
- class: quota / capacity

### F2 — No numeric maximum Realtime session duration is publicly specified

- claim: A Realtime session exists for the lifetime of its WebSocket connection; after disconnection, a new session must be created. The current Realtime and limits pages do not state a numeric maximum duration. This supports per-call session design but does not establish that long calls are contractually supported without forced reconnects.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/concepts/agents/realtime
- publisher: Yandex Cloud / AI Studio
- pub_date: undated, current documentation
- accessed: 2026-09-11
- confidence: high for documented lifecycle; medium for the not-found boundary
- class: session lifecycle / limitation

### F3 — Realtime is an integrated, programmable speech-agent core, but its documented media interface is WebSocket LPCM

- claim: Yandex Realtime accepts and emits audio over WebSocket, documents LPCM audio, server-side voice activity detection, tool use, and Russian/Kazakh-oriented models with SpeechKit voices. It can therefore own streaming speech recognition, turn handling, LLM reasoning, tool calling, and synthesis inside one session. The public core documentation does not itself establish SIP codec negotiation or contact-center call controls.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/concepts/agents/realtime
- publisher: Yandex Cloud / AI Studio
- pub_date: undated, current documentation
- accessed: 2026-09-11
- confidence: high
- class: product capability / interface

### F4 — The documented Asterisk integration is a media bridge, not a native direct connection

- claim: Asterisk RTP and the Realtime WebSocket PCM interface are incompatible directly. The official guide inserts a Python bridge: dialplan sends the call to a Stasis ARI app; ARI creates an External Media channel and a mixing bridge; RTP is received locally by Python, transcoded and streamed to Yandex; generated audio returns over the same chain. The guide explicitly describes a local `127.0.0.1` RTP leg and only an outbound external connection from the bridge to AI Studio.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/operations/agents/voice-agent-asterisk-integration
- publisher: Yandex Cloud / AI Studio; guide credited by Yandex to Dmitry Voropaev / Torgi223
- pub_date: undated, current documentation
- accessed: 2026-09-11
- confidence: high
- class: telephony architecture

### F5 — The Asterisk bridge transfers meaningful production operations to Staffy

- claim: The guide requires Asterisk 16.6+ for ARI External Media (or Asterisk 18+ for the simpler AudioSocket alternative), ARI HTTP configuration, a dedicated extension and routing, Python 3.8+, dependencies, secrets, a systemd service, and PBX diagnostics. Its sample creates one call object and one Realtime WebSocket per call, maintains media queues/buffers, and invalidates buffered output on `speech_started` to implement interruption. If the bridge stops, the Stasis application is unregistered and calls fail; the sample systemd unit merely restarts it. The guide does not supply load balancing, HA, readiness checks, metrics, capacity tests, outbound dialing, DTMF, transfer orchestration, or contact-center lifecycle management.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/operations/agents/voice-agent-asterisk-integration
- publisher: Yandex Cloud / AI Studio; guide credited by Yandex to Dmitry Voropaev / Torgi223
- pub_date: undated, current documentation
- accessed: 2026-09-11
- confidence: high for documented setup/failure behavior; medium-high for the explicit documentation gaps
- class: operational burden / production readiness

### F6 — The sample bridge validates a narrow codec path, not general SIP interoperability

- claim: The Asterisk example converts G.711 μ-law at 8 kHz to PCM at 16 kHz for Yandex and back, using 20 ms RTP frames; the guide also explains the changes needed for G.711 A-law. Wrong codec selection can prevent transcription, and a wrong local RTP port can cause silence. This validates the sample’s `ulaw`/`alaw` path, not every carrier codec or direct-SIP codec matrix.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/operations/agents/voice-agent-asterisk-integration
- publisher: Yandex Cloud / AI Studio; guide credited by Yandex to Dmitry Voropaev / Torgi223
- pub_date: undated, current documentation
- accessed: 2026-09-11
- confidence: high
- class: codec / interoperability

### F7 — Current public evidence does not establish that direct SIP has graduated from Preview

- claim: Voice assistants and Realtime API entered Preview on 2025-09-24. Release notes current through 2026-09-10 show later Realtime models and breaking format changes, but no GA promotion for Realtime or direct SIP. Therefore the defensible conclusion is not “direct SIP is certainly still Preview,” but “no current public GA evidence was found; contractual status and production support must be confirmed in writing.” The newly documented Asterisk bridge is a fallback transport path, not evidence that native SIP is GA.
- source URL: https://aistudio.yandex.ru/ru/docs/ai-studio/release-notes/
- publisher: Yandex Cloud / AI Studio
- pub_date: continuously updated; latest entry reviewed 2026-09-10
- accessed: 2026-09-11
- confidence: high on release-note history; medium on present status because absence is not proof
- class: maturity / release status

### F8 — Realtime remains a moving compatibility target

- claim: AI Studio changed the Realtime format on 2026-04-20 and ended support for the old format after 2026-05-12; it added a new Speech Realtime model on 2026-05-28 and an experimental DeepSeek-based variant on 2026-07-30 that Yandex describes as noticeably slower. A production integration should pin an explicitly tested model/protocol configuration behind an adapter and treat upgrades as rehearsed migrations.
- source URL: https://aistudio.yandex.ru/ru/docs/ai-studio/release-notes/
- publisher: Yandex Cloud / AI Studio
- pub_date: entries dated 2026-04-20, 2026-05-28, and 2026-07-30
- accessed: 2026-09-11
- confidence: high
- class: versioning / compatibility

### F9 — The published 99.90% SLA is not clearly an end-to-end voice SLA

- claim: The AI Studio service-level document promises 99.90% availability and compensation tiers of 10%, 15%, or 30% of consumed service. Its unavailability calculation is defined using qualifying HTTP 500 / `INTERNAL_SERVER_ERROR` and gRPC code 13 / `INTERNAL` errors, aggregated by minute. The document does not explain how WebSocket disconnects, delayed/stalled audio, SIP setup failures, or the customer-operated bridge are counted. Staffy should not present 99.90% as an end-to-end call-path SLO without written clarification.
- source URL: https://yandex.ru/legal/cloud_sla_yandex_ai_studio/ru/
- publisher: Yandex Cloud
- pub_date: 2025-09-24
- accessed: 2026-09-11
- confidence: high
- class: SLA / contractual boundary

### F10 — Logging opt-out limits persistence, but does not prove a Realtime-specific Russian-only data path

- claim: AI Studio terms allow request/content use for analytics, error correction, debugging, and model training unless logging is disabled. The customer can disable logging; the change takes effect within 24 hours. Thereafter, request information is not stored on Yandex servers except short-term storage objectively required to process a request, and is not used for debugging or training. However, the terms define “Request” as text information and do not explicitly map this rule to raw Realtime audio, generated audio, transcripts, temporary buffers, or the WebSocket transport. They also place lawful-basis and sensitive-data obligations on the customer. This is useful retention evidence, not sufficient voice-specific assurance.
- source URL: https://yandex.ru/legal/cloud_terms_yandex_ai_studio/ru/
- publisher: Yandex Cloud
- pub_date: published 2026-04-17; effective 2026-04-28
- accessed: 2026-09-11
- confidence: high for the terms; medium for applicability to every Realtime artifact
- class: data handling / legal

### F11 — Yandex provides general Russian infrastructure evidence, not component-level residency proof

- claim: Yandex Cloud states that its data centers are located in the Russian Federation and that its infrastructure is designed for compliance with Federal Law 152-FZ. This supports a Russian-cloud deployment position. The page does not enumerate the regions, processors, replicas, logs, transcripts, or temporary media buffers used by AI Studio Realtime, so it is not official proof that every Realtime artifact remains exclusively in Russian regions throughout processing.
- source URL: https://yandex.cloud/ru/security/data-privacy
- publisher: Yandex Cloud
- pub_date: undated, current security page
- accessed: 2026-09-11
- confidence: high for the platform claim; high that service-specific detail is absent from this source
- class: region / data residency

### F12 — MTS Exolve is a genuine managed Russian alternative at product-capability level

- claim: MTS Exolve offers a managed voice-robot product for inbound and outbound calls. Its current product page describes speech recognition and synthesis, context/meaning understanding, user interruption handling, AI agents and knowledge-base answers, SIP trunk/PBX connection, operator transfer with context, REST API, webhooks, MCP, CRM integrations, and a published AI-agent tariff of 6.5 ₽/minute. It states that data is stored on MTS servers in Russia and presents 152-FZ compliance. This clears the threshold of an integrated managed voice-agent product rather than merely separate speech APIs.
- source URL: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/
- publisher: MTS Exolve
- pub_date: undated, current product page
- accessed: 2026-09-11
- confidence: high for advertised product capabilities; medium for production semantics not disclosed publicly
- class: managed alternative / Russian market

### F13 — MTS Exolve is more opaque than Yandex Realtime for programmable runtime decisions

- claim: The Exolve product page establishes the combined managed capability set, but does not specify its realtime transport/API, exact tool-call contract, session-state model, concurrency quotas, maximum call duration, model/provider choices, codec matrix, latency, availability SLA, or failure semantics. It should be evaluated as a turnkey managed-agent candidate through a paid bake-off and written technical questionnaire, not assumed to be a drop-in programmable replacement for the Yandex Realtime WebSocket API.
- source URL: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/
- publisher: MTS Exolve
- pub_date: undated, current product page
- accessed: 2026-09-11
- confidence: medium-high
- class: alternative limitation / procurement

## Contradictions and unresolved tensions

1. **Published SLA vs voice-path reality.** AI Studio publishes 99.90%, but the legal measurement describes HTTP/gRPC errors while Realtime is WebSocket-based; neither native SIP nor a customer Asterisk bridge is explicitly covered end to end.
2. **Russian cloud vs Realtime-specific residency.** Yandex states that its data centers are in Russia, but no reviewed official source traces raw audio, transcripts, generated audio, logs, backups, and temporary buffers through a Realtime-specific Russian-only data flow.
3. **Logging opt-out vs media terminology.** The terms promise no persistence beyond objectively necessary temporary processing after logging is disabled, but define a request as text and do not explicitly bind every Realtime audio artifact or state how the opt-out is applied to WebSockets.
4. **Managed SIP positioning vs maturity evidence.** The last explicit release status found was Preview and no later GA entry was found, while Yandex now documents a substantial self-managed Asterisk bridge. This does not prove native SIP remains Preview, but it prevents treating it as demonstrated GA.
5. **Network isolation vs bridge availability.** Local RTP and an outbound-only Yandex connection reduce exposure, yet the sample bridge is a single operational dependency whose absence makes the Stasis call fail; the guide provides restart behavior, not HA.
6. **Turnkey Russian alternative vs inspectability.** MTS Exolve publicly covers the integrated feature checklist, including SIP and interruption, but exposes far less runtime, quota, SLA, and compatibility detail than Yandex’s developer documentation.

## Updated recommendation

For the first paid pilots, do **not** build a separate STT + LLM + TTS pipeline by default. Use an integrated managed speech-agent core and keep telephony, business tools, and transcript/event sinks behind Staffy-owned adapters.

Run a two-path bake-off:

1. **Programmable path: Yandex AI Studio Realtime + Staffy telephony adapter.** Prefer native Yandex SIP only after Yandex confirms in writing its current release/support status, codecs and call controls, quotas, failure semantics, SLA coverage, and data handling. Otherwise use the documented Asterisk bridge, accepting that Staffy owns PBX routing, RTP/codec conversion, process availability, observability, scaling, and per-call WebSocket lifecycle. The bridge still uses Yandex end to end for STT/turn handling/LLM/TTS; it is a self-managed telephony path, not an independent Russian component stack.
2. **Turnkey path: MTS Exolve managed AI agent.** Include it because it publicly combines telephony, recognition, synthesis, context, interruption, AI/knowledge-base answers, integrations, and Russian data storage. Require a technical and contractual response on APIs/tools, quotas, SLA, models, codecs, portability, and data retention before selecting it.

Use a third, componentized path only if a pilot reveals a hard requirement the integrated products cannot satisfy: mandated provider separation, custom turn-taking, specific voice/model quality, or a contractual data boundary. The reviewed evidence does not show that building a separate Russian STT/LLM/TTS runtime currently buys enough quality, cost, or maturity to justify its extra latency and operations for initial pilots.

Pre-launch gates:

- secure a quota above the expected peak plus headroom; the documented default is 10 concurrent Realtime sessions;
- obtain written clarification of WebSocket/native-SIP SLA semantics and direct SIP status;
- disable AI Studio logging at least 24 hours before production tests if required, then verify the account setting and obtain voice-specific retention/residency assurances;
- pin and regression-test the Realtime protocol/model version behind adapters;
- if using Asterisk, add health/readiness checks, multi-instance call routing, structured metrics, secret rotation, overload behavior, carrier-codec tests, and failover/transfer behavior before accepting paid traffic;
- define adapters at four boundaries: `TelephonyMedia`, `RealtimeAgentSession`, `BusinessToolGateway`, and `ConversationEventSink`. This keeps native SIP, Asterisk, Exolve, or a future component pipeline replaceable without rewriting business tools.

## Not found in reviewed official sources

- A numeric maximum duration for a Yandex Realtime session or call.
- A current explicit GA/Preview declaration for native Yandex direct SIP, plus its own concurrency limits and production support policy.
- A direct-SIP codec matrix, DTMF behavior, call transfer semantics, outbound dialing controls, and SIP failure codes.
- WebSocket- and SIP-specific availability calculations under the AI Studio SLA.
- A Realtime-specific data-flow/residency document proving that raw audio, generated audio, transcripts, logs, replicas, and temporary buffers remain exclusively in Russian regions.
- Explicit official confirmation that the generic logging opt-out covers WebSocket audio and every derived Realtime artifact.
- Asterisk bridge concurrency benchmarks, sizing guidance, HA reference architecture, or load-test results.
- MTS Exolve’s exact realtime API, tool-call semantics, state model, quotas, maximum duration, codec list, latency/SLA, model providers, and retention schedule.
- A second current Russian vendor with official public evidence for the complete combination of realtime STT + LLM + TTS + barge-in + tools + SIP at production-product level. Separate speech APIs and generic bot builders were not counted as proof.
- Official comparative evidence that a self-managed Russian component stack outperforms either integrated path on quality, latency, cost, or operational reliability.

## Sources reviewed (8)

1. https://aistudio.yandex.ru/en/docs/ai-studio/concepts/limits
2. https://aistudio.yandex.ru/en/docs/ai-studio/concepts/agents/realtime
3. https://aistudio.yandex.ru/en/docs/ai-studio/operations/agents/voice-agent-asterisk-integration
4. https://aistudio.yandex.ru/ru/docs/ai-studio/release-notes/
5. https://yandex.ru/legal/cloud_sla_yandex_ai_studio/ru/
6. https://yandex.ru/legal/cloud_terms_yandex_ai_studio/ru/
7. https://yandex.cloud/ru/security/data-privacy
8. https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/
