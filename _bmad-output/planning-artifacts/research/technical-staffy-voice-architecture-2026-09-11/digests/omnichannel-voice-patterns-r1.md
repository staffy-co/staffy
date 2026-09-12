# Omnichannel voice runtime patterns — Pipecat and strong analogs

**Decision served:** Which product/runtime patterns Staffy should reuse conceptually without adopting Pipecat or analog code, and which patterns are framework-specific.

**Scope:** Pipeline/frame/event model; transports; turn-taking and barge-in; context/state aggregation; tool invocation; human handoff; observability/evals; failure recovery; scaling/multi-tenancy; channel separation.

**Research boundary:** Official primary documentation, repository release notes, and one official production-platform troubleshooting guide only. Eight distinct source pages were read. No Staffy project files were assessed. Accessed 2026-09-12.

## Decision digest

Adopt the **contracts and state transitions**, not the framework object model. The most transferable architecture is a channel-neutral session runtime built around typed, correlated events; explicit priority/ordering rules; a pluggable turn controller; prompt context based on what the user actually heard; tool calls represented as cancellable, observable jobs; deterministic transfer orchestration; and test/telemetry artifacts that share stable event names.

Do **not** reproduce Pipecat's exact `Frame` taxonomy, processor-push semantics, aggregator placement API, or LiveKit's room/job/Cloud abstractions as Staffy's domain model. Those mechanisms are useful evidence for the underlying concerns, but they couple application behavior to a particular runtime and transport.

The evidence is strongest for in-session orchestration. It is materially thinner for crash recovery, cross-node session resume, multi-tenant isolation, production capacity, and completed human-transfer semantics. Those areas should remain explicit Staffy design work, not be inferred from framework claims.

## Reusable patterns

### R1. Use one typed event contract, with separate ordering lanes for urgent control and ordinary media/data

- **Claim:** Pipecat models audio, text, images, context, errors, turn signals, and lifecycle boundaries as typed frames flowing through modular processors. It gives system frames a high-priority ordered lane and queues data/control frames together in a separate ordered lane. This supports a reusable Staffy pattern: define channel-neutral event envelopes with stable type, session/turn/correlation identity, timestamp, direction, and causation; give interruption/cancel/error/end events an explicit priority path rather than letting them wait behind audio or model output.
- **Source URL:** https://docs.pipecat.ai/pipecat/learn/pipeline
- **Publisher:** Pipecat / Daily
- **Publication/update date:** Live documentation; the page exposes no publication or last-update date. The content is consistent with Pipecat v1.10.0 release material dated 2026-09-12.
- **Accessed:** 2026-09-12
- **Confidence:** Medium-high. The mechanism is directly documented; the Staffy envelope fields are an architectural inference, not a Pipecat prescription.
- **Class:** Architecture pattern / synthesis

### R2. Keep the transport edge replaceable and keep channel semantics out of core orchestration

- **Claim:** Pipecat places `transport.input()` and `transport.output()` at the pipeline edge and describes service processors as swappable. LiveKit similarly treats the agent as a stateful bridge: realtime user media is carried through a room over WebRTC or telephony while backend communication uses HTTP/WebSockets. The reusable pattern is a per-channel adapter that normalizes ingress into canonical session events and renders egress back to WebRTC, PSTN/SIP, chat, or another channel. Core dialog, policy, tool, and state code should depend on capabilities and events, not a transport SDK.
- **Source URLs:** https://docs.pipecat.ai/pipecat/learn/pipeline ; https://docs.livekit.io/agents/
- **Publishers:** Pipecat / Daily; LiveKit
- **Publication/update dates:** Both are live documentation pages with no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** High for the separation principle because two independent framework publishers expose the same boundary; medium for any claim about how Staffy should implement adapters.
- **Class:** Integration pattern / cross-source synthesis

### R3. Separate channel payloads logically even when they share one session timeline

- **Claim:** Pipecat uses distinct frame types for raw audio, transcription, LLM text, TTS text, context, errors, and lifecycle signals, while LiveKit explicitly supports speech/audio, text/transcriptions, and vision as multiple channels in one agent session. Staffy should therefore preserve a single correlated session timeline but avoid collapsing audio, transcript, UI/control, tool, and business-state messages into one undifferentiated payload. Consumers should subscribe only to the event families they own.
- **Source URLs:** https://docs.pipecat.ai/pipecat/learn/pipeline ; https://docs.livekit.io/agents/
- **Publishers:** Pipecat / Daily; LiveKit
- **Publication/update dates:** Live documentation; neither page shows a publication or last-update date.
- **Accessed:** 2026-09-12
- **Confidence:** High for distinct modalities/types; medium-high for the proposed subscription boundary, which is a synthesis.
- **Class:** Channel-separation pattern / synthesis

### R4. Model turn-taking as an explicit, pluggable state machine rather than a single silence timer

- **Claim:** LiveKit exposes several turn-boundary strategies—semantic/acoustic turn detection on top of VAD, realtime-model detection, VAD only, STT endpointing, and manual control—and separates endpointing from interruption handling. Its STT endpointing mode still uses VAD for interruption detection. The reusable pattern is an explicit turn controller with states and events for speech-start, partial/final transcript, possible-end, committed-end, agent-start, interruption candidate, accepted barge-in, backchannel, and cancelled/finished output. Select and tune the detector by channel and language rather than assuming one global threshold.
- **Source URL:** https://docs.livekit.io/agents/logic/turns/
- **Publisher:** LiveKit
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** Medium-high. Modes and separation are directly documented; the proposed state names are a Staffy-facing synthesis.
- **Class:** Realtime interaction pattern

### R5. Make barge-in a coordinated cancellation transaction, not merely “stop TTS”

- **Claim:** In Pipecat, interruption and user-speech signals travel as high-priority system frames. Its tool model also distinguishes calls cancelled on interruption from asynchronous calls allowed to continue. LiveKit documents explicit interruption behavior and manual turn APIs that stop current agent speech, clear buffered user input, enable listening, commit a turn, or discard it. Together these support a reusable rule: an accepted barge-in should atomically stop or truncate playback, invalidate queued speech, decide what happens to model generation and each tool job, preserve the correct heard transcript, and emit one auditable interruption outcome.
- **Source URLs:** https://docs.pipecat.ai/pipecat/learn/pipeline ; https://docs.pipecat.ai/pipecat/learn/function-calling ; https://docs.livekit.io/agents/logic/turns/
- **Publishers:** Pipecat / Daily; LiveKit
- **Publication/update dates:** Live documentation; no publication or last-update dates shown.
- **Accessed:** 2026-09-12
- **Confidence:** High for the need to coordinate speech, input, and work cancellation; medium-high for the proposed atomic transaction boundary.
- **Class:** Turn-taking / cancellation pattern

### R6. Build conversational context from committed user input and actually delivered assistant output

- **Claim:** Pipecat's context aggregator adds user transcriptions and uses TTS-emitted text to represent what was actually spoken. It documents placing assistant aggregation after transport output so context remains correct at word level when the assistant is interrupted. The reusable pattern is to distinguish planned/generated response text from delivered/heard output and commit only the delivered portion to canonical conversation history. Prompt context, transcript storage, and human-handoff summaries should derive from that canonical record.
- **Source URL:** https://docs.pipecat.ai/pipecat/learn/context-management
- **Publisher:** Pipecat / Daily
- **Publication/update date:** Live documentation; no publication or last-update date shown. The page notes deprecation behavior introduced in Pipecat 1.9.0, and the current release page shows v1.10.0 on 2026-09-12.
- **Accessed:** 2026-09-12
- **Confidence:** High for the heard-output rule; medium-high for extending it to durable transcript and handoff summaries.
- **Class:** Context/state pattern

### R7. Keep three state layers separate: ephemeral turn buffers, prompt context, and durable business/session state

- **Claim:** Pipecat's universal LLM context combines messages, tool definitions, and tool-choice settings and can be appended, replaced, transformed, or summarized. LiveKit describes the agent itself as a stateful realtime bridge and offers handoffs/shared state as higher-level logic. These sources show that prompt context is mutable runtime material, not the authoritative business record. Staffy should keep transient audio/partial-transcript buffers, model-facing context, and validated durable facts/workflow state as separately owned stores linked by event IDs.
- **Source URLs:** https://docs.pipecat.ai/pipecat/learn/context-management ; https://docs.livekit.io/agents/
- **Publishers:** Pipecat / Daily; LiveKit
- **Publication/update dates:** Live documentation; neither page shows a publication or last-update date.
- **Accessed:** 2026-09-12
- **Confidence:** Medium. The source facts are direct; the three-layer split is a design inference and is not named verbatim by either framework.
- **Class:** State-management synthesis

### R8. Represent tool use as a first-class job lifecycle with explicit interruption and timeout policy

- **Claim:** Pipecat documents per-tool timeout, cancellation on user interruption, opt-in asynchronous continuation, model-directed cancellation for selected long-running calls, parallel/grouped calls, and intermediate results. Timeouts cancel the handler and allow the model to explain that work did not complete. Staffy should give every call a stable ID and explicit lifecycle (`requested`, `authorized`, `running`, `progress`, `succeeded`, `failed`, `timed_out`, `cancelled`, `result_ignored`) and require each tool definition to state interruption policy, deadline, side-effect class, and user-visible recovery behavior.
- **Source URL:** https://docs.pipecat.ai/pipecat/learn/function-calling
- **Publisher:** Pipecat / Daily
- **Publication/update date:** Live documentation; no publication or last-update date shown. Its per-tool cancellation API is current in the page and deprecates a global flag for removal in 2.0.0.
- **Accessed:** 2026-09-12
- **Confidence:** High for timeouts/cancellation/async/progress as first-class concerns; medium-high for the proposed lifecycle and side-effect metadata.
- **Class:** Tool-orchestration pattern

### R9. Treat human handoff as deterministic orchestration around a model request

- **Claim:** Retell documents two ways to trigger phone transfer—a registered `transfer_call` function or an explicit transfer node—and recommends clear transfer conditions plus scenario tests. It also separates “transfer not triggered” from “transfer triggered but failed,” with SIP logs used to diagnose the latter and optional human detection able to fail. The reusable pattern is: the model may request handoff, but deterministic policy validates eligibility/channel/destination; the runtime enters an explicit transfer state; telephony performs the action; and success/failure is confirmed from provider events before the AI session ends. Preserve a compact summary and reason code for the human recipient.
- **Source URL:** https://docs.retellai.com/reliability/call-performance
- **Publisher:** Retell AI
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** Medium. Trigger/failure separation is directly documented. End-to-end success confirmation and recipient summary are recommended inferences. This single-publisher failure evidence was not independently verified and must not support a reliability guarantee.
- **Class:** Human-handoff pattern / partially unverified

### R10. Use one event vocabulary across runtime telemetry, traces, and regression tests

- **Claim:** Pipecat's frames receive unique identifiers for debugging, and its v1.9.0 release added a latency breakdown with stable keys and ownership categories for turn contributions. LiveKit advertises transcripts/traces and documents tests that assert messages, tool calls, arguments, handoffs, and multi-turn outcomes; text tests run locally/CI while simulations can exercise text or the full audio pipeline. The reusable pattern is to make runtime events the shared substrate for traces, dashboards, replay fixtures, and assertions. Start with deterministic text/tool/state tests; add audio scenarios for endpointing, interruption, recognition, and playback-specific regressions.
- **Source URLs:** https://docs.pipecat.ai/pipecat/learn/pipeline ; https://github.com/pipecat-ai/pipecat/releases ; https://docs.livekit.io/agents/ ; https://docs.livekit.io/agents/start/testing/
- **Publishers:** Pipecat / Daily; GitHub-hosted Pipecat release notes; LiveKit
- **Publication/update dates:** Pipecat v1.9.0 released 2026-09-11 and v1.10.0 released 2026-09-12; the documentation pages are live and undated.
- **Accessed:** 2026-09-12
- **Confidence:** High. The telemetry and test capabilities are directly documented by two independent framework publishers; the unified Staffy event vocabulary is a synthesis.
- **Class:** Observability/evaluation pattern

### R11. Isolate each live session behind a dispatchable worker boundary, but keep placement infrastructure replaceable

- **Claim:** LiveKit documents an agent server that registers with its control plane, receives dispatch, starts a job subprocess for a room, and provides load balancing and graceful shutdowns. This supports a conceptual Staffy boundary: each live session has isolated in-memory turn/prompt state and a lifecycle owned by one worker; a separate scheduler admits and assigns sessions; durable business state and audit records live outside the worker. The exact room, job subprocess, dispatch protocol, and hosted scheduler are LiveKit-specific.
- **Source URL:** https://docs.livekit.io/agents/
- **Publisher:** LiveKit
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** Medium. Process-per-job and orchestration capabilities are vendor-documented. No independent production capacity, failover, or isolation evidence was found in the source budget, so this does not verify scale or multi-tenant safety.
- **Class:** Scaling/runtime-boundary pattern / advertised behavior

### R12. Design recovery around explicit checkpoints and degraded modes; do not infer transparent resume from either framework

- **Claim:** Current Pipecat tool handling exposes timeouts and cancellation cleanup, and LiveKit separates agent server lifecycle from per-session jobs and advertises graceful shutdown. Those are useful primitives, but the reviewed sources do not establish cross-process media-stream continuation, exactly-once tool effects, session migration, or automatic reconstruction of partially heard output after a worker crash. Staffy should checkpoint committed transcript, durable facts, completed tool effects, transfer state, and last acknowledged output boundary; on loss, choose an explicit policy—reconnect and resume, restart with disclosure, transfer to a human, or terminate safely—rather than pretending the stream is transparent.
- **Source URLs:** https://docs.pipecat.ai/pipecat/learn/function-calling ; https://docs.livekit.io/agents/
- **Publishers:** Pipecat / Daily; LiveKit
- **Publication/update dates:** Live documentation; no publication or last-update dates shown.
- **Accessed:** 2026-09-12
- **Confidence:** Medium for the gap finding; medium-low for the precise checkpoint set, which is an architecture recommendation not validated by these framework docs.
- **Class:** Failure-recovery gap / synthesis

## Framework-specific mechanisms to avoid copying into the domain model

### F1. Pipecat's exact `Frame` inheritance tree and “processors always push frames” rule

- **Claim:** `SystemFrame`, `DataFrame`, `ControlFrame`, upstream/downstream `push_frame`, and `ParallelPipeline` are Pipecat runtime mechanics. Reuse typed messages, priority, ordering, branching, and backpressure concerns, but express them in Staffy-owned contracts so orchestration can span processes and non-Python services without emulating Pipecat internals.
- **Source URL:** https://docs.pipecat.ai/pipecat/learn/pipeline
- **Publisher:** Pipecat / Daily
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** High.
- **Class:** Framework-specific

### F2. Positional context-aggregator wiring

- **Claim:** Putting the assistant aggregator after `transport.output()` is the Pipecat mechanism for capturing actual spoken text. The invariant is portable; the positional pipeline API is not. Staffy can enforce the invariant through renderer acknowledgements or delivered-output events instead of requiring a particular processor order.
- **Source URL:** https://docs.pipecat.ai/pipecat/learn/context-management
- **Publisher:** Pipecat / Daily
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** High.
- **Class:** Framework-specific mechanism with reusable invariant

### F3. Provider- or framework-owned turn-detection defaults and exact thresholds

- **Claim:** LiveKit's recommended/default detector, its realtime-model mode, and example telephony thresholds are integrations of a specific SDK and model stack. Staffy should retain a detector interface and channel profiles, not encode LiveKit's detector selection, Python-only dynamic endpointing behavior, or example thresholds as product policy.
- **Source URL:** https://docs.livekit.io/agents/logic/turns/
- **Publisher:** LiveKit
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** High.
- **Class:** Framework/provider-specific

### F4. LiveKit rooms, agent servers, job subprocesses, dispatch, and Cloud observability

- **Claim:** These are LiveKit deployment/control-plane abstractions. Reuse session isolation, admission/dispatch, graceful drain, and per-session telemetry as capabilities; do not make a LiveKit room ID or job lifecycle the authoritative Staffy business/session model unless LiveKit itself is deliberately chosen as infrastructure.
- **Source URL:** https://docs.livekit.io/agents/
- **Publisher:** LiveKit
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** High.
- **Class:** Framework/cloud-specific

### F5. Model-directed cancellation as the only stale-work safeguard

- **Claim:** Pipecat can expose cancellation tools to the LLM for selected asynchronous functions and explicitly warns that a model may cancel too readily. Reuse it only as one signal. Runtime policy should still cancel or ignore work deterministically when the owning turn/session is superseded, timed out, unauthorized, or ended.
- **Source URL:** https://docs.pipecat.ai/pipecat/learn/function-calling
- **Publisher:** Pipecat / Daily
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** High for the limitation; medium-high for the recommended policy layering.
- **Class:** Rejected single-mechanism pattern

## Weakly evidenced or rejected patterns

### W1. “A generic sequential pipeline is the complete product architecture” — reject

- **Claim:** Pipecat's pipeline is an effective in-session media/control abstraction, but the reviewed material does not make it a system of record, tenant boundary, deployment scheduler, workflow engine, or recovery log. Using the in-memory pipeline graph as the whole Staffy architecture would conflate realtime processing with durable product state.
- **Source URLs:** https://docs.pipecat.ai/pipecat/learn/pipeline ; https://docs.pipecat.ai/pipecat/learn/context-management
- **Publishers:** Pipecat / Daily
- **Publication/update dates:** Live documentation; no dates shown.
- **Accessed:** 2026-09-12
- **Confidence:** Medium-high as a scope conclusion from what the pages explicitly cover; absence cannot prove that other Pipecat components do not address these concerns.
- **Class:** Rejected architecture extrapolation

### W2. “Framework-managed load balancing proves production scale” — unsupported

- **Claim:** LiveKit advertises automatic load balancing and job/session lifecycle management, but this source set contains no independently verified capacity, latency-under-load, noisy-neighbor, regional failover, or recovery measurements. Use the control-plane pattern, not a scale conclusion.
- **Source URL:** https://docs.livekit.io/agents/
- **Publisher:** LiveKit
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** High that the claim is advertised; low that any production scale implication is established.
- **Class:** Scale claim / unverified

### W3. “Phone transfer equals successful human handoff” — reject

- **Claim:** Retell's troubleshooting page shows that request triggering, transfer execution, SIP outcome, and optional human detection are distinct failure points; it also says transfer is phone-only in that product. Therefore an API/function success signal alone is insufficient evidence of a completed human handoff, and the exact channel restriction is Retell-specific.
- **Source URL:** https://docs.retellai.com/reliability/call-performance
- **Publisher:** Retell AI
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** Medium. Directly documented by one platform, but the failure pattern was not independently corroborated within this round and is not used as a quantitative reliability conclusion.
- **Class:** Failure pattern / single-source unverified

### W4. “A single automatic turn detector is correct for every channel and language” — reject

- **Claim:** LiveKit itself documents distinct semantic/acoustic, realtime-model, VAD-only, STT, and manual strategies, with different applicability. Staffy should treat turn strategy as a channel/language/provider capability decision and preserve manual override.
- **Source URL:** https://docs.livekit.io/agents/logic/turns/
- **Publisher:** LiveKit
- **Publication/update date:** Live documentation; no publication or last-update date shown.
- **Accessed:** 2026-09-12
- **Confidence:** High.
- **Class:** Rejected turn-taking simplification

### W5. Recent release churn argues against copying internal APIs

- **Claim:** Pipecat released v1.9.0 on 2026-09-11 and v1.10.0 on 2026-09-12. The current documentation already marks a context-system-message pattern deprecated since 1.9.0 and a global async-tool-cancellation flag deprecated for 2.0.0. This is fresh evidence that public concepts are evolving quickly; Staffy's stable internal contracts should be narrower than any one framework's current API surface.
- **Source URLs:** https://github.com/pipecat-ai/pipecat/releases ; https://docs.pipecat.ai/pipecat/learn/context-management ; https://docs.pipecat.ai/pipecat/learn/function-calling
- **Publisher:** Pipecat / Daily (GitHub and official docs)
- **Publication/update dates:** Releases dated 2026-09-11 and 2026-09-12; documentation pages undated.
- **Accessed:** 2026-09-12
- **Confidence:** High for release/deprecation facts; medium for the architecture implication.
- **Class:** Version/compatibility / verified within publisher

## Leads for a next round

1. **Crash/reconnect semantics:** Pipecat error handling and production-hosting docs; LiveKit job lifecycle, events/error handling, dispatch, and self-hosting docs. Establish which failures are retryable and whether any supported resume token exists.
2. **Human handoff completion contract:** Primary Twilio/LiveKit SIP transfer events plus a second production voice-platform source. Define ringing, answered, human-confirmed, conference-joined, failed, and AI-disconnected states.
3. **Multi-tenancy:** Official authorization/dispatch docs from chosen transport plus Staffy-owned requirements for tenant-scoped credentials, quotas, data residency, audit access, and per-tenant configuration versions.
4. **Idempotent tools:** Primary provider/webhook documentation for appointment, CRM, messaging, and telephony actions. Determine idempotency keys, reconciliation, and compensation behavior for timeout-after-side-effect ambiguity.
5. **Backpressure and overload:** Framework source and deployment docs covering queue bounds, admission control, worker health, load shedding, graceful drain, and overload UX. Obtain two independent production sources before relying on any capacity or recovery claim.
6. **Observability schema:** Pipecat metrics/observers and OpenTelemetry semantic conventions, plus LiveKit session events. Decide canonical span/event names and privacy/redaction requirements.
7. **Audio-specific evals:** Pipecat eval lifecycle and LiveKit audio simulations, then independently validate reproducibility, interruption assertions, and channel noise fixtures.

## What this round could not establish

- No independently verified throughput, concurrent-session capacity, latency-at-scale, regional failover, or cost claim. No scale number is used in the recommendation.
- No evidence that Pipecat or LiveKit provides transparent cross-worker continuation of a live media stream after process loss.
- No complete, cross-vendor state machine for warm human handoff, attended transfer, consent, or post-transfer AI teardown.
- No primary evidence for tenant isolation, tenant-specific credential routing, quotas, data residency, retention, or encryption boundaries in the runtime abstractions reviewed.
- No proof that tool cancellation prevents an external side effect already committed; timeout, retry, idempotency, reconciliation, and compensation remain application responsibilities.
- No independent production retrospective validating the failure modes surfaced in Retell's troubleshooting guide; those claims remain single-source and are not used to assert a failure rate.
- No comparative benchmark supporting a specific turn detector or numeric endpointing threshold across Staffy's intended channels and languages.
- No publication/update dates were exposed on the seven live documentation pages. Recency was confirmed only by retrieval on 2026-09-12 and, for Pipecat version facts, the official release page.

## Source register (8 distinct pages)

1. Pipecat, **Pipeline & Frame Processing** — https://docs.pipecat.ai/pipecat/learn/pipeline — live page, no date shown — accessed 2026-09-12.
2. Pipecat, **Context Management** — https://docs.pipecat.ai/pipecat/learn/context-management — live page, no date shown — accessed 2026-09-12.
3. Pipecat, **Function Calling** — https://docs.pipecat.ai/pipecat/learn/function-calling — live page, no date shown — accessed 2026-09-12.
4. Pipecat GitHub, **Releases** — https://github.com/pipecat-ai/pipecat/releases — v1.9.0 dated 2026-09-11; v1.10.0 dated 2026-09-12 — accessed 2026-09-12.
5. LiveKit, **Agents introduction** — https://docs.livekit.io/agents/ — live page, no date shown — accessed 2026-09-12.
6. LiveKit, **Turns overview** — https://docs.livekit.io/agents/logic/turns/ — live page, no date shown — accessed 2026-09-12.
7. LiveKit, **Testing and evaluation** — https://docs.livekit.io/agents/start/testing/ — live page, no date shown — accessed 2026-09-12.
8. Retell AI, **Debug call transfer failure** — https://docs.retellai.com/reliability/call-performance — live page, no date shown — accessed 2026-09-12.
