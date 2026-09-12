# Technical Deep Recon digest — runtime maturity, round 1

Topic: Staffy voice architecture for first paid pilots in Russia  
Decision: which voice-loop components to buy/use as managed services and which to build  
Accessed: 2026-09-11  
Source budget used in this digest: 8 primary sources

## Findings

### Claim 1

- claim: Yandex AI Studio Realtime is a managed, end-to-end voice-agent runtime rather than only separate speech APIs. Its official documentation describes bidirectional asynchronous WebSocket events, partial and final results, LPCM audio, Russian/Kazakh-oriented realtime models, SpeechKit voices, configurable server VAD, native function calling, file/web search, and MCP. This is the shortest credible path to a Russian-language pilot without Staffy building STT, LLM orchestration, TTS, and basic turn detection itself.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/concepts/agents/realtime
- publisher: Yandex Cloud
- pub_date: undated; live documentation crawled 2026-09-11
- accessed: 2026-09-11
- confidence: high for documented capabilities; medium for production quality because no independent performance evidence was read
- class: fact + architectural inference

### Claim 2

- claim: The Yandex tool boundary is usable as a replaceable application adapter. Tools are declared as JSON schemas; the model returns a structured function call, while the client executes it and returns a `function_call_output`. Therefore Staffy can and should own business state, validation, authorization, idempotency, retries, and audit outside the voice vendor.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/concepts/agents/realtime
- publisher: Yandex Cloud
- pub_date: undated; live documentation crawled 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: fact + recommendation

### Claim 3

- claim: Direct Yandex SIP connectivity is not yet a normal self-service GA dependency. Yandex says SIP/RTP connects existing telephony to AI Studio, but the feature is Preview and integration is performed by Yandex experts after a support request. For a paid pilot, it may be used only after a dated enablement commitment and an end-to-end acceptance test; Staffy needs a second telephony path or an explicit manual fallback.
- source URL: https://yandex.cloud/ru/blog/voice-agent-and-other-updates
- publisher: Yandex Cloud
- pub_date: 2026-05-28
- accessed: 2026-09-11
- confidence: high for Preview/support-assisted status; high for the dependency-risk inference
- class: fact + risk + recommendation

### Claim 4

- claim: LiveKit can supply the media/telephony substrate without forcing its managed cloud. The official comparison states that self-hosted LiveKit supports the Agents framework and full SIP/telephony, with the SIP service deployed separately; agent hosting, scaling, observability, and model credentials remain Staffy's responsibility. This makes LiveKit a plausible controlled fallback when Russian data placement or a local SIP trunk matters, but it is an infrastructure choice, not a turnkey Russian voice-agent product.
- source URL: https://docs.livekit.io/transport/self-hosting/
- publisher: LiveKit
- pub_date: undated; live documentation crawled 2026-09-11
- accessed: 2026-09-11
- confidence: high for documented deployment boundaries; medium for Russian-carrier interoperability because it was not verified
- class: fact + architectural inference

### Claim 5

- claim: LiveKit's turn handling has a material Russian-language caveat. Its current audio turn detector lists 14 languages but not Russian; the separate text turn detector does list Russian, yet it requires live streaming STT. When paired with a realtime speech model, LiveKit says an extra STT component is required and the realtime model's own turn detection must be disabled to avoid conflict. Thus the new audio detector should not be assumed to cover Russian calls; use provider VAD for the first Yandex path or a tested Russian streaming-STT + text-detector path.
- source URL: https://docs.livekit.io/agents/logic/turns/turn-detector/
- publisher: LiveKit
- pub_date: undated; live documentation crawled 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: fact + production constraint + recommendation

### Claim 6

- claim: LiveKit Agents is active and deployable but its voice abstractions are still moving quickly. The latest official release is `livekit-agents@1.8.1`, published 2026-09-10, and introduces a new `DuplexModel` abstraction while also carrying fixes in telemetry, voice transport-close handling, audio sink accounting, adapters, and TTS connection pools. Staffy should pin the full compatible package set, upgrade only through recorded-call regression tests, and avoid building business logic against internal voice lifecycle objects.
- source URL: https://github.com/livekit/agents/releases/tag/livekit-agents@1.8.1
- publisher: LiveKit (GitHub)
- pub_date: 2026-09-10
- accessed: 2026-09-11
- confidence: high for release/version facts; high for the version-pinning recommendation when combined with Pipecat's same-day release and deprecations below
- class: fact + maturity signal + recommendation

### Claim 7

- claim: LiveKit's default barge-in is usable, but semantic interruption policy is not yet a clean public extension point. A current issue documents only `vad` and `adaptive` modes and requests a supported custom detector interface; the described workaround is to disable built-in handling and implement from VAD/STT events, which couples application code to internal interruption lifecycle. Do not make a custom semantic barge-in classifier a pilot prerequisite; wrap interruption policy behind a Staffy interface and begin with conservative, measurable defaults.
- source URL: https://github.com/livekit/agents/issues/7105
- publisher: LiveKit community / GitHub issue, assigned to a LiveKit maintainer
- pub_date: 2026-09-03
- accessed: 2026-09-11
- confidence: medium-high; current API limitation is specific and reproducible from the public API description, but the source is an issue rather than release documentation
- class: production constraint + recommendation

### Claim 8

- claim: Pipecat has a broad telephony abstraction: its documentation describes inbound and outbound PSTN/SIP, WebRTC via Daily plus SIP, WebSocket media integrations, and simultaneous use of multiple providers. However the documented ready-made paths are centered on Daily, Twilio, Telnyx, Plivo, and Exotel rather than Russian carriers. Treat Pipecat as a provider-neutral pipeline/orchestration candidate, while budgeting a Russian SIP/WebSocket transport spike rather than assuming plug-and-play compatibility.
- source URL: https://docs.pipecat.ai/pipecat/telephony/overview
- publisher: Pipecat / Daily
- pub_date: undated; live documentation crawled 2026-09-11
- accessed: 2026-09-11
- confidence: high for documented transports; medium for the Russian-integration risk inference
- class: fact + gap + recommendation

### Claim 9

- claim: Pipecat is feature-rich but also changing at a pace that makes exact pinning and an application-owned adapter layer mandatory. Official `v1.9.0` was released 2026-09-11. It adds end-to-end latency contribution reporting, speech/interruption observers, function-call lifecycle observations, eager end-of-turn generation, new STT integrations/settings, and flow/eval features; the same release includes behavior changes, deprecations targeted for removal in 2.0, provider default/model migrations, and fixes to active-call/turn handling. This is good evidence of active production hardening, not evidence that arbitrary provider combinations are already stable.
- source URL: https://github.com/pipecat-ai/pipecat/releases/tag/v1.9.0
- publisher: Pipecat / Daily (GitHub)
- pub_date: 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: fact + maturity signal + recommendation

### Claim 10

- claim: For the first paid pilots, the default architecture should be: buy Yandex's Russian realtime speech/intelligence loop; buy Russian SIP/PSTN termination; build only the Staffy-owned control plane around them. Use Yandex SIP Preview if and only if it passes the pilot acceptance matrix. Keep a self-hosted LiveKit telephony/media path as the first fallback candidate; keep Pipecat as the stronger component-swapping experiment, not the default production runtime until Russian telephony and interruption behavior are exercised with recorded calls.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/concepts/agents/realtime ; https://yandex.cloud/ru/blog/voice-agent-and-other-updates ; https://docs.livekit.io/transport/self-hosting/ ; https://docs.pipecat.ai/pipecat/telephony/overview
- publisher: Yandex Cloud; LiveKit; Pipecat / Daily
- pub_date: mixed; see individual claims
- accessed: 2026-09-11
- confidence: medium-high; it minimizes unverified components, but carrier interoperability and measured Russian call quality remain gates
- class: synthesis + recommendation

### Claim 11

- claim: Staffy should not build its own STT, TTS, VAD, media server, or general agent framework for the first pilots. It should build five replaceable seams: `TelephonyAdapter` (call lifecycle, transfer, DTMF, media), `RealtimeAgentAdapter` (session/audio/events/cancel-or-truncate), `ToolRuntime` (schema, authorization, idempotency, timeout), `TurnPolicy` (VAD/end-of-turn/interruption settings), and a vendor-neutral event/observability model. A cascaded `SpeechAdapter` can be added later if Staffy separates STT and TTS.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/concepts/agents/realtime ; https://docs.livekit.io/transport/self-hosting/ ; https://github.com/pipecat-ai/pipecat/releases/tag/v1.9.0
- publisher: Yandex Cloud; LiveKit; Pipecat / Daily
- pub_date: mixed; see individual claims
- accessed: 2026-09-11
- confidence: high as an architecture recommendation; adapters align with the documented component and event boundaries
- class: synthesis + recommendation

### Claim 12

- claim: The most stable layer is the standard-shaped boundary: SIP/RTP or provider media streams, bidirectional audio/events, JSON-schema tool calls, and independently replaceable STT/LLM/TTS services. The fastest-moving layer is policy and framework behavior around turn detection, barge-in, duplex models, provider defaults, lifecycle events, and convenience workflows. Business invariants must therefore live above frameworks and never depend on a spoken acknowledgement having completed.
- source URL: https://aistudio.yandex.ru/en/docs/ai-studio/concepts/agents/realtime ; https://github.com/livekit/agents/releases/tag/livekit-agents@1.8.1 ; https://github.com/livekit/agents/issues/7105 ; https://github.com/pipecat-ai/pipecat/releases/tag/v1.9.0
- publisher: Yandex Cloud; LiveKit; Pipecat / Daily
- pub_date: mixed; see individual claims
- accessed: 2026-09-11
- confidence: high
- class: synthesis + architecture principle

## Practical pilot gate

Before declaring any stack production-ready, replay a fixed Russian call suite covering: noisy 8 kHz telephony; short acknowledgements versus true interruptions; names, phone numbers, dates and prices; tool timeout; repeated/duplicate tool call; caller hang-up during a tool; inbound and outbound call; DTMF; cold/warm transfer if required; provider reconnect; transcript/audio alignment; and a human takeover. Record per-turn timing by component, but do not set a latency or concurrency promise until measured on the chosen Russian carrier path.

## Contradictions and tensions

1. Yandex presents AI Studio as suitable from prototype to production, but its direct SIP integration is explicitly Preview and support-assisted. The model/API may be production-usable while the turnkey telephony edge is not yet an unconditional dependency.
2. LiveKit describes full self-hosted SIP/telephony support, yet self-hosting also transfers agent scaling, observability, credentials, network/NAT, and SIP operations to Staffy. "Full support" does not mean "low operational burden."
3. LiveKit's current audio turn detector and text turn detector have different language coverage: Russian is absent from the former and present in the latter. Generic "multilingual turn detection" should not be interpreted as one interchangeable implementation.
4. Pipecat's broad provider catalog and same-day feature release indicate strong momentum, while its deprecations, behavior changes, provider-model replacements, and turn fixes indicate a moving compatibility surface. Momentum and stability are not the same property.
5. Both open-source candidates make components replaceable, but replacement can change endpointing, partial transcript timing, interruption semantics, and tool cancellation. A uniform interface alone does not guarantee uniform conversation behavior.

## Leads for round 2

1. Obtain Yandex written answers for Realtime and SIP: service status/GA plan, SLA, quotas/concurrency, maximum session duration, regions/data handling, codecs and sample rates, DTMF, caller ID, inbound/outbound, REFER and warm transfer, recording, call progress/answering-machine detection, and failure semantics.
2. Run a short market pass over current Russian managed alternatives: MTS Exolve/VoiceTech, T-Bank VoiceKit, Sber SaluteSpeech/GigaChat, VK Cloud speech stack, Voximplant availability in Russia, and telecom-integrator turnkey voice agents. The question is specifically whether any offers a managed realtime agent + SIP + tool-calling product, not merely STT/TTS.
3. Verify interoperability of a Russian SIP trunk with self-hosted LiveKit: codecs, RTP/NAT, DTMF mode, REFER, caller ID, call-progress tones, recordings, and lawful routing constraints.
4. Prototype a Yandex Realtime adapter against LiveKit and/or Pipecat only after confirming supported event semantics for cancel/truncate, tool calls, partial transcripts, and audio clocking.
5. Compare Russian STT/TTS quality on Staffy's own appointment vocabulary and phone audio; do not extrapolate from multilingual feature lists.
6. Check exact package compatibility immediately before implementation. The researched latest versions are LiveKit Agents `1.8.1` (2026-09-10) and Pipecat `1.9.0` (2026-09-11), but neither should be adopted solely because it is latest.

## Not found in round 1

- No independent, current benchmark establishing Yandex Realtime latency, concurrency, or error rate on Russian PSTN calls.
- No current official SLA or quota/concurrency page for Yandex Realtime/SIP was verified.
- No official evidence in the capped source set that another Russian vendor currently matches Yandex's full managed combination of realtime STT + LLM + TTS + barge-in + tool calling + SIP.
- No verified production case study for LiveKit or Pipecat with a Russian carrier, Russian-language interruption behavior, and paid call-center traffic.
- No source-supported scale recommendation; capacity planning must wait for measured pilots. Numeric performance claims were deliberately not made.
- No verified compatibility matrix for Yandex Realtime with LiveKit or Pipecat adapters.
- No basis yet to choose LiveKit over Pipecat on latency or throughput. The current preference is about deployment boundary and pilot risk, not measured performance.

