# Imported-chat claim extraction: Go vs Pipecat/Asterisk/LiveKit

Imported source: `imports/чат-с-ии-поисковиком.md` (88,143 bytes; 414 lines).  
Role: hypothesis input only, not independent evidence. The links are Google redirect URLs and many quantitative claims have no direct source attached.

## Stable product ideas worth preserving

- Lines 58–66: separate reusable Core from Tenant Config and expose a no-code control plane.
- Lines 113 and 152–154: multi-tenant platform with distinct control and data planes.
- Lines 288–303: separate media transport from business behavior and place integrations behind a gateway.
- Lines 359–363: enter through a small number of verticals before opening a broad no-code SaaS surface.

These are design requirements supplied by the user/import, not empirically proven facts.

## Load-bearing technical claims requiring verification

1. Lines 223–225: Pipecat/Python is called a critical mistake at thousands of concurrent calls; Go is said to reduce infrastructure cost 4–5x.
2. Lines 232, 247 and 352: a custom Go path is said to guarantee first response under 350 ms or 280 ms.
3. Lines 237 and 303: Pion WebRTC is described as directly joining SIP carriers and WebRTC, and new SIP clients as connectable in three minutes.
4. Lines 259 and 352: local Silero VAD in Go is said to yield 30–50 ms interruption and a decisive market advantage.
5. Lines 281–288: custom Go MVP is estimated at 2–3 months with 3–4 engineers; Pipecat is said to require a full rewrite at the 50th commercial customer because of the GIL.
6. Line 312: Go is said to be hundreds of times faster than Python in the relevant workload, with sub-millisecond networking differences and 100–200/80–150 ms external model latencies.

## Internal contradictions and category errors to test

- Lines 26–69 first call Pipecat ideal; lines 169–215 then call Pipecat + Asterisk + LiveKit ideal; lines 223–259 reverse to a custom Go/Pion foundation after the user asks to ignore prior preferences. The verdict changes with framing without new comparative evidence.
- Pion WebRTC and SIP/PSTN are treated as one protocol surface. Whether Pion supplies SIP signalling, carrier interop, DTMF, transfer and PSTN lifecycle must be checked separately.
- Line 288 proposes both excluding Asterisk and using `go-asterisk/go-ari`; ARI normally implies that Asterisk remains in the system.
- “50th customer” mixes tenant count with concurrency. Resource limits normally depend on simultaneous sessions, codecs, audio rate, external APIs and deployment topology, not the number of configured tenants.
- Lines 288–299 claim Prompt + RAG + tools make any business fully no-code. This omits typed domain contracts, permissions, workflow state, transaction semantics, evals and exception handling required for deep business integration.
- Lines 125–138 make blanket Russian compliance claims (including biometric and SORM statements) without a scoped legal/data-flow analysis.

## Questions for evidence acquisition

- Is there any published Pipecat limit tied to 50 tenants or calls?
- Which parts of Pipecat are Python bytecode-bound versus asynchronous I/O or native-code providers?
- What exactly does Pion implement, and what must be built to terminate production SIP/PSTN calls?
- What share of end-to-end latency can Go remove when STT/LLM/TTS and turn detection remain external?
- What measured concurrency or cost threshold makes a custom media core cheaper than replaceable adapters over mature telephony/media components?
