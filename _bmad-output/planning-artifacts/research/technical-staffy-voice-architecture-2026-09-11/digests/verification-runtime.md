# Fresh-context verification — runtime maturity

Verified: 2026-09-11  
Scope: only the three load-bearing claims requested from `runtime-maturity-r1-1.md`  
Method: current web spot-check, with official documentation used for product/interface boundaries and independent evidence used only for measured behavior. Absence findings refer to this bounded search, not to proof that no private evidence exists.

## Verdicts

| Claim | Outcome | Verdict |
|---|---|---|
| A. Yandex Realtime boundary; direct SIP is Preview/support-assisted | **Verified, with qualification** | The managed realtime voice-loop boundary and the direct-SIP status are current. A newer self-managed Asterisk bridge path exists, but it does not turn Yandex's direct SIP integration into self-service GA. |
| B. LiveKit self-hosted Agents/SIP boundary; Russian turn-detector caveat | **Verified, with material update** | Self-hosting retains full Agents and SIP support but leaves agent hosting/scaling, model keys, observability, and a separately deployed SIP service to the operator. Russian is still absent from the current audio detector. The older Russian-capable text detector remains usable but is now deprecated and slated for removal in Agents 2.0. |
| C. Yandex is the shortest pilot path; LiveKit/Pipecat are fallbacks | **Unverified as a comparative performance claim; reasonable only as a conditional architecture inference** | Yandex exposes fewer components and a Russian/Kazakh-oriented managed speech-to-speech runtime, so it is a defensible implementation prior. No source found establishes that it wins on real Russian PSTN quality, latency, reliability, or time-to-production. Keep the recommendation explicitly gated by enablement and recorded-call tests. |

## Evidence and implications

### A — Yandex Realtime and SIP

Yandex's current Realtime documentation describes a WebSocket, bidirectional asynchronous event interface with partial/final responses, PCM audio, Russian/Kazakh-oriented multimodal realtime models, SpeechKit voices, configurable server VAD, and JSON-schema function calling whose execution remains in the client application. This supports the digest's characterization of Realtime as a managed voice-agent loop, not merely a pair of standalone STT/TTS APIs. It does **not** independently establish production latency or call quality. [Yandex Realtime documentation](https://aistudio.yandex.ru/ru/docs/ai-studio/concepts/agents/realtime)

Yandex's 2026-05-28 announcement still explicitly says that direct SIP/RTP connection is in **Preview**, and that Yandex experts perform the integration after a service-support request. That verifies the support-assisted dependency risk. [Yandex SIP announcement](https://yandex.cloud/ru/blog/voice-agent-and-other-updates)

There is now also an official-documentation-hosted, user-contributed guide for connecting FreePBX/Asterisk to Realtime through a self-run RTP-to-WebSocket bridge. This is a credible second path around the Preview integration, but it shifts bridge, codec, Asterisk, and production operations to Staffy; it is not evidence that Yandex's direct SIP edge is GA. [Yandex Asterisk bridge guide](https://aistudio.yandex.ru/ru/docs/ai-studio/operations/agents/voice-agent-asterisk-integration)

**Decision impact:** retain the Preview acceptance gate. Add the documented Asterisk bridge as a candidate contingency alongside LiveKit rather than treating direct Yandex SIP as the only Yandex-compatible telephony route.

### B — LiveKit deployment boundary and Russian turn handling

LiveKit's comparison says self-hosting provides full Agents support and full SIP/telephony support, with SIP deployed separately. It also says the operator runs and scales agent servers, supplies model-provider keys, and provides custom/external observability. This verifies the digest's boundary: LiveKit can be the controlled media/telephony substrate, but it is not a turnkey managed Russian voice-agent product. [LiveKit self-hosting comparison](https://docs.livekit.io/transport/self-hosting/)

The current audio `TurnDetector` supports 14 named languages and Russian is not among them; without STT it defaults to English thresholds. The separate transcript-based `MultilingualModel` still lists Russian and requires a live STT stream when used with a realtime model, plus disabling the realtime model's own turn detection. However, that text detector is now explicitly deprecated and slated for removal in Agents 2.0. [LiveKit turn-detector documentation](https://docs.livekit.io/agents/logic/turns/turn-detector/)

**Decision impact:** the digest's warning remains valid but should be strengthened. Do not base a new Russian production path on the deprecated text detector without a migration plan. Provider-native VAD/turn handling or a separately benchmarked Russian endpointing strategy remains the safer pilot assumption.

### C — ordering of candidates

An independent 2026 benchmark compared Pipecat and LiveKit using the same STT/LLM/TTS components and found their task-completion results inside each other's error bars. Crucially, its published methodology says **telephony was excluded**, and the scenarios are synthetic Acme Bank calls rather than Russian-language calls over a Russian carrier. It therefore confirms that generic framework results cannot choose the Russian PSTN stack. [Veris AI VAmoS comparison](https://veris.ai/blog/vamos-bench-pipecat)

The bounded search found no independent, current benchmark of Yandex Realtime, LiveKit, or Pipecat on the same Russian appointment-call corpus over a Russian PSTN/SIP path. It also found no evidence that would overturn Yandex-first as a low-component-count pilot hypothesis.

**Decision impact:** phrase the recommendation as: **start by testing Yandex Realtime because it minimizes the initially assembled speech stack and is explicitly Russian-oriented; use direct Yandex SIP only if Preview enablement is dated and acceptance-tested; otherwise test the Asterisk bridge or LiveKit/Pipecat path. Promote no stack until the same recorded Russian PSTN suite passes on it.** “Shortest” refers to expected implementation scope, not proven call quality or elapsed delivery time.

## Overall verdict

The runtime digest is directionally sound. Claims A and B survive, with two updates: Yandex now documents a self-managed Asterisk bridge path, and LiveKit's Russian-capable text turn detector is deprecated. Claim C must remain provisional and conditional; current public evidence does not support an unconditional ranking for Russian PSTN production.
