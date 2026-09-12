---
digest: go-decision-latency-economics-r1
research_type: technical
decision_shape: select
decision: "own Go voice core now vs managed/modular staged path"
status: complete
round: 1
validation: high
accessed: 2026-09-12
source_count: 8
research_firewall: "Project material used only as hypotheses: imports/чат-с-ии-поисковиком.md"
---

# Go voice core: latency and economics decision digest

## Verdict

**Choose the managed/modular staged path now; do not fund a from-scratch Go voice core on the present evidence.** Keep a narrow media/orchestration interface so a Go component can replace a measured bottleneck later.

Why: the claims used to justify an immediate rewrite — **<350 ms**, **280 ms**, **30–50 ms interruption**, **4–5× infrastructure savings**, and **2–3 months with 3–4 engineers** — are not established for the proposed Russian/Yandex/PSTN stack. Two reproducible end-to-end data points point in the opposite direction for current cascaded/managed stacks: a from-scratch streaming implementation reported **947 ms p50, 729 ms best case** [S8], while a carrier-recording benchmark across 550 real calls reported **1,296–1,740 ms p50** and **1,768–2,259 ms p95** [S7]. These do not prove that 280–350 ms is impossible; they do show that it cannot be treated as a credible product SLO without a Staffy-specific benchmark.

The official frameworks already expose the clocks needed to locate the actual bottleneck. LiveKit approximates conversation latency as `end_of_utterance_delay + llm.ttft + tts.ttfb` [S2]. Pipecat additionally separates audible TTS start from first byte, leading silence, first answer token, model thinking, and text aggregation [S1]. Replacing Python orchestration with Go before collecting these measurements risks optimizing a non-dominant slice.

## End-to-end latency model

The user-visible measure must be **far-end speech stop → far-end first audible agent sample**, from a dual-channel recording. Internal timestamps remain diagnostic, not the verdict; OpenBenchmarks found that platform-side clocks can read materially earlier than what the caller hears [S7].

| Segment | Start → stop | What belongs here | Required instrument |
|---|---|---|---|
| 1. Inbound transport | caller audio sample → agent ingress PCM | PSTN/WebRTC packetization, network RTT, loss/retransmission, jitter buffer, decoding/transcoding | RTP/WebRTC stats, packet arrival timestamps, codec and transcode count |
| 2. Turn completion | physical speech end → turn accepted complete | VAD hangover, endpointing/semantic turn detection, final STT transcript | VAD transition, EOU delay, transcription delay; LiveKit exposes both [S2] |
| 3. LLM start | completed turn → first answer token | queueing, prompt assembly/RAG, network, model TTFT, hidden reasoning/tool decision | request timestamp, TTFB, first answer token/TTFAT; Pipecat distinguishes them [S1] |
| 4. Text aggregation | first answer token → first TTS-safe chunk | sentence/phrase buffering and normalization | first token and first TTS submission; Pipecat exposes text aggregation [S1] |
| 5. TTS start | TTS submission → first audible PCM | provider queue, synthesis TTFB, container/header bytes, leading silence | both TTFB and first non-silent audio/TTFA; Pipecat distinguishes them [S1] |
| 6. Outbound media | first PCM → caller hears it | encode/transcode, application queue, packet pacing, SFU/PBX, jitter/playout buffer | enqueue/dequeue times, RTP timestamps, far-end recording onset |
| 7. Barge-in | caller speech onset → last audible bot sample | detection, cancellation propagation, already-buffered TTS/media, far-end playout | dual-channel onset/offset plus detection and cancel timestamps |

Important accounting rules:

- Streaming overlaps work, so summing component medians is not an end-to-end result. Correlate spans by turn and measure the far-end outcome.
- `TTS TTFB` is not necessarily audible speech. Leading silence must be measured; Pipecat now exposes TTFA as `TTFB + leading_silence` [S1].
- Asterisk sends common telephony codecs in 20 ms frames and warns that mistimed/odd buffers produce gaps; Opus/Speex passthrough leaves timing to the application [S5]. Thus packetization alone consumes discrete 20 ms quanta, while buffers can dominate cancellation.
- Pion's official benchmark currently measures connection setup phases and CPU/peer counts, not speech-stop-to-first-audio or barge-in under a voice pipeline [S6]. It is a useful harness seed, not evidence of voice-agent latency or savings.

## Claim ledger

### C1 — The correct critical path is EOU/STT → LLM first answer token → text aggregation → TTS first audible audio → output transport

- **Status:** verified as the measurement model; numeric budget remains deployment-specific.
- **Class:** architecture/performance measurement.
- **Evidence 1:** URL: https://docs.livekit.io/deploy/observability/data/ · publisher: LiveKit · pub_date: undated living documentation · accessed: 2026-09-12. LiveKit defines EOU and transcription delays, LLM TTFT, TTS TTFB, end-to-end latency, and the approximate additive formula [S2].
- **Evidence 2:** URL: https://docs.pipecat.ai/pipecat/fundamentals/metrics · publisher: Pipecat · pub_date: undated living documentation · accessed: 2026-09-12. Pipecat exposes user-to-bot latency, TTFB, TTFA, TTFAT, thinking time, leading silence, and text aggregation [S1].
- **Confidence:** high.
- **Contrary evidence/gaps:** neither formula fully captures caller-side packet delivery/playout by itself; far-end audio remains required.
- **Decision implication:** instrument the modular baseline first. Go is justified only if its replaceable slice is measured as material at p95/p99.

### C2 — Yandex SpeechKit can participate in a streaming cascaded pipeline, but its reviewed public docs do not establish a 280–350 ms end-to-end SLO

- **Status:** streaming capability verified; latency number unverified.
- **Class:** capability + performance.
- **Evidence 1:** URL: https://yandex.cloud/ru-kz/docs/speechkit/stt/streaming · publisher: Yandex Cloud · pub_date: updated 2026-08-21 · accessed: 2026-09-12. The API returns partial results while audio is still arriving and finalizes at EOU [S3].
- **Evidence 2:** URL: https://yandex.cloud/ru-kz/docs/speechkit/tts/ · publisher: Yandex Cloud · pub_date: updated 2026-08-24 · accessed: 2026-09-12. API v3 accepts successive text chunks for synthesis, explicitly including LLM text as it is generated [S4].
- **Confidence:** high for capability; low/unverified for any latency SLO.
- **Contrary evidence/gaps:** the reviewed pages publish no Russian-region p50/p95/p99 for final STT, TTS first audible sample, or carrier-to-carrier end-to-end latency. The STT page's example of sending 400 ms chunks is a pacing example, not a latency guarantee [S3].
- **Decision implication:** provider streaming removes the need to invent that mechanism in a Go core, but Yandex must be benchmarked from the intended Russian region and telephony path.

### C3 — “The proposed Go architecture returns first audio in <350 ms”

- **Status:** unverified; materially challenged by two independent reproducible measurements.
- **Class:** performance (two-source class).
- **Evidence 1:** URL: https://github.com/openbenchmarks-labs/voice-agent-latency · publisher: OpenBenchmarks Labs · pub_date: benchmark generated 2026-08-01 · accessed: 2026-09-12. Dual-channel carrier recordings across 550 calls / 2,078 usable turns measured 1,296–1,740 ms p50 and 1,768–2,259 ms p95 across five managed platforms [S7].
- **Evidence 2:** URL: https://arxiv.org/abs/2603.05413 · publisher: arXiv preprint by Qiu et al. · pub_date: 2026-03-05 · accessed: 2026-09-12. A released from-scratch streaming STT→LLM→TTS implementation measured 947 ms p50 and 729 ms best case [S8].
- **Confidence:** high that **public evidence does not validate** <350 ms as a production SLO; low about the exact Staffy result until tested.
- **Contrary evidence/gaps:** both sources use other providers/topologies; neither tests Yandex, Russian PSTN, or the proposed Go implementation. Therefore the claim is not overturned as physically impossible, only rejected as an evidenced planning assumption.
- **Decision implication:** do not use <350 ms in roadmap, capacity, or sales commitments. Treat it as an experiment target; initial credible acceptance should be based on measured distribution, not a single best case.

### C4 — “Token-to-audio streaming reduces latency to 280 ms”

- **Status:** unverified; the mechanism is valid, the number is not.
- **Class:** performance (two-source class).
- **Evidence 1:** URL: https://yandex.cloud/ru-kz/docs/speechkit/tts/ · publisher: Yandex Cloud · pub_date: updated 2026-08-24 · accessed: 2026-09-12. Streaming TTS from incremental LLM text is supported [S4].
- **Evidence 2:** URL: https://arxiv.org/abs/2603.05413 · publisher: arXiv preprint by Qiu et al. · pub_date: 2026-03-05 · accessed: 2026-09-12. The authors identify cross-component streaming/pipelining as the key technique, yet their measured result is 947 ms p50 / 729 ms best case [S8].
- **Confidence:** high for the qualitative benefit; low/unverified for 280 ms.
- **Contrary evidence/gaps:** 280 ms has no named model, utterance corpus, region, network, endpointing rule, percentile, or far-end clock. Text aggregation may add latency even after the first LLM token; Pipecat's own documentation treats it as a separate metric [S1].
- **Decision implication:** implement streaming in the modular baseline and measure it. It is not evidence for replacing the orchestrator.

### C5 — “Local Silero VAD in Go makes the bot stop in 30–50 ms after interruption”

- **Status:** unverified.
- **Class:** performance (two-source class).
- **Evidence 1:** URL: https://docs.livekit.io/deploy/observability/data/ · publisher: LiveKit · pub_date: undated living documentation · accessed: 2026-09-12. LiveKit defines interruption `detection_delay` from overlapping-speech onset to final prediction, including model RTT [S2].
- **Evidence 2:** URL: https://docs.asterisk.org/Configuration/Channel-Drivers/WebSocket/ · publisher: Asterisk Project · pub_date: undated living documentation; page covers releases 20.16/21.11/22.6/23.0 and later JSON updates · accessed: 2026-09-12. Common telephony media is paced in 20 ms frames; buffering and mistiming can add audible gaps [S5].
- **Confidence:** low/unverified for 30–50 ms.
- **Contrary evidence/gaps:** no reviewed source reports caller-onset→far-end-last-bot-audio at that distribution. VAD inference time alone omits confirmation/hangover, cancellation propagation, application/SFU/PBX queues, packetization, and already-buffered playout. Two 20 ms frames already span 40 ms before those additions [S5].
- **Decision implication:** specify barge-in as a far-end p50/p95/p99 outcome plus false-interruption/backchannel rates. Do not equate per-frame VAD compute with audible stop latency.

### C6 — “A Go voice core reduces infrastructure cost 4–5× versus Pipecat/LiveKit/Asterisk”

- **Status:** unverified.
- **Class:** cost/scale (two-source class).
- **Evidence 1:** URL: https://github.com/pion/webrtc-bench · publisher: Pion community · pub_date: undated living repository, current 2026 · accessed: 2026-09-12. The harness reports CPU, peer counts, and connection-setup timings for Pion, but publishes no controlled Go-vs-Pipecat voice-agent cost comparison [S6].
- **Evidence 2:** URL: https://docs.pipecat.ai/pipecat/fundamentals/metrics · publisher: Pipecat · pub_date: undated living documentation · accessed: 2026-09-12. Pipecat exposes model usage and latency instrumentation, but no infrastructure ratio against a Go core [S1].
- **Confidence:** low/unverified.
- **Contrary evidence/gaps:** no fixed workload, codec path, concurrency, instance/GPU types, provider fees, utilization, SLO, HA topology, engineering/on-call cost, or amortization horizon was attached to 4–5×. STT/LLM/TTS charges can dominate orchestration compute and do not disappear when the media core is rewritten.
- **Decision implication:** 4–5× must not enter the business case. Compare all-in rubles per successful conversation minute at matched quality and p95/p99 SLO; include engineering and operations.

### C7 — “A production Go MVP takes 2–3 months with 3–4 experienced engineers”

- **Status:** unverified estimate.
- **Class:** delivery/implementation reality.
- **Evidence 1:** URL: https://arxiv.org/abs/2603.05413 · publisher: arXiv preprint by Qiu et al. · pub_date: 2026-03-05 · accessed: 2026-09-12. The paper releases a working tutorial pipeline but gives no person-month evidence for a production multi-tenant platform [S8].
- **Evidence 2:** URL: https://docs.asterisk.org/Configuration/Channel-Drivers/WebSocket/ · publisher: Asterisk Project · pub_date: undated living documentation · accessed: 2026-09-12. Even the PBX boundary includes codec-specific framing, timing, buffering, flow control, and evolving protocol behavior [S5]; this documents scope, not duration.
- **Confidence:** low/unverified.
- **Contrary evidence/gaps:** “MVP” is undefined. The estimate omits HA/failover, tenant isolation, call control, recording/consent, observability, deployment, security, load testing, incident tooling, model/provider failover, and real Russian telephony interop. Public documents cannot infer team velocity.
- **Decision implication:** treat 2–3 months as, at most, a prototype estimate. Fund a time-boxed benchmark spike before a platform rewrite; estimate the production core only after its acceptance scope is explicit.

### C8 — “Owning Go now is the lower-risk architecture decision”

- **Status:** not supported; staged modularity is favored.
- **Class:** selection conclusion.
- **Evidence 1:** URL: https://docs.livekit.io/deploy/observability/data/ · publisher: LiveKit · pub_date: undated living documentation · accessed: 2026-09-12. The managed/modular path exposes stage and per-turn metrics adequate to identify where latency sits [S2].
- **Evidence 2:** URL: https://docs.asterisk.org/Configuration/Channel-Drivers/WebSocket/ · publisher: Asterisk Project · pub_date: undated living documentation · accessed: 2026-09-12. Current Asterisk can handle framing/timing for most codecs over its WebSocket media driver, reducing one class of custom media plumbing [S5].
- **Confidence:** medium.
- **Contrary evidence/gaps:** a Go core could still win at Staffy's eventual concurrency, topology, or compliance boundary; there is simply no demonstrated crossover point. Managed-provider availability and Russian deployment constraints were not costed in this slice.
- **Decision implication:** preserve replaceable provider/transport interfaces and raw metrics. Build only the smallest Go service whose benefit is demonstrated (for example, a media gateway or cancellation controller), not an all-at-once voice platform.

## Benchmark that decides the question

### Candidates

- **A — staged baseline:** Pipecat orchestration + self-hosted LiveKit for WebRTC + Asterisk current WebSocket external media for PSTN/SIP + the same Yandex STT/TTS and selected LLM.
- **B — owned path:** Go service using Pion/direct media handling at the corresponding boundary, with exactly the same STT, LLM, TTS, prompts, tools, region, codec inputs, and output voice.
- If B changes providers/models as well as orchestration, it is not a Go-vs-modular test; run those as separate factors.

### Workload and controls

- Real dual-channel calls, not loopback service timestamps: Russian PSTN/SIP at 8 kHz G.711 and WebRTC/Opus as separate cohorts.
- Pinned corpus spanning short answers, long entity-rich requests, noise, backchannels, mid-word/mid-sentence interruptions, tool calls, and provider timeouts.
- Warm and cold sessions; concurrency steps 1/10/50/100/250 (or forecast-aligned equivalents); identical region placement and instance class.
- At least 1,000 accepted turns per candidate/channel before making a percentile claim; publish exclusions and failed/no-response turns separately.
- Hold WER/entity accuracy, task success, voice, prompt, endpointing sensitivity, and retry policy constant. A faster but less accurate/premature endpoint is a failed comparison.

### Required clocks

`caller speech end → ingress audio → VAD speech-off → final transcript → LLM request → first answer token → first TTS text → first TTS byte → first non-silent PCM → media enqueue → far-end first audible sample`.

For interruption: `far-end caller speech onset → detection → cancel issued → last outbound agent frame → far-end last audible bot sample`. Record queue depth and already-buffered audio at cancel.

Report p50/p95/p99 and bootstrap confidence intervals for every stage and far-end total, plus loss, jitter, retransmits, transcode count/time, GC/event-loop pauses, CPU/RAM/network, failed calls, false barge-ins, missed barge-ins, and premature EOU.

### Economics

Compute **all-in RUB per successful conversation minute** at each concurrency: compute/network/PBX/SFU, Yandex STT/TTS, LLM, storage/egress, observability, HA spare capacity, support/on-call, and amortized build/maintenance headcount. Also report rubles per completed business task so latency gains are not bought with repeat turns or lower task success.

### Proposed Go trigger (decision rule, not an evidence claim)

Proceed from A to a broader owned Go core only if the controlled test shows all of the following:

1. framework + media overhead outside STT/LLM/TTS is **>20% of p95 far-end latency or >100 ms p95**;
2. B improves far-end p95 by **≥25%** at matched p99 reliability and quality, not merely internal service TTFB;
3. B reduces forecast-volume all-in cost/min by **≥30% after amortized engineering and HA/on-call costs**;
4. the advantage persists under PSTN loss/jitter, barge-in, tool-call, and provider-degradation cohorts.

If those gates fail, retain the modular stack and optimize the measured dominant stages (endpointing/STT, LLM TTFT/prompt/tools, TTS TTFA/leading silence, or transport buffers).

## What public evidence can and cannot establish

**Can establish now:** the correct latency clocks and decomposition [S1][S2]; Yandex STT/TTS streaming capability [S3][S4]; Asterisk framing/timing/buffering behavior [S5]; availability and limits of a Pion load-test seed [S6]; and independent caller-side baselines showing that sub-350 ms is not representative of measured present-day end-to-end systems [S7][S8].

**Cannot establish without Staffy measurement:** 280 ms or <350 ms for the proposed Russian stack; 30–50 ms audible interruption; Go-vs-Python orchestration share of total latency; Go-vs-modular concurrency/cost crossover; 4–5× savings; and 2–3 month/3–4 engineer production delivery.

## Source appendix (round 1; 8 sources)

| ID | Source | Publisher | pub_date | accessed | Role / confidence |
|---|---|---|---|---|---|
| S1 | [Pipecat Metrics](https://docs.pipecat.ai/pipecat/fundamentals/metrics) | Pipecat | undated living documentation | 2026-09-12 | Primary framework metrics definitions; high for capability, not a deployment benchmark |
| S2 | [LiveKit Data hooks / latency metrics](https://docs.livekit.io/deploy/observability/data/) | LiveKit | undated living documentation | 2026-09-12 | Primary framework metrics/formula; high for capability, not an independent performance result |
| S3 | [Yandex SpeechKit streaming STT](https://yandex.cloud/ru-kz/docs/speechkit/stt/streaming) | Yandex Cloud | updated 2026-08-21 | 2026-09-12 | Primary API behavior; high for capability, no latency percentile |
| S4 | [Yandex SpeechKit TTS](https://yandex.cloud/ru-kz/docs/speechkit/tts/) | Yandex Cloud | updated 2026-08-24 | 2026-09-12 | Primary streaming TTS behavior; high for capability, no latency percentile |
| S5 | [Asterisk WebSocket channel driver](https://docs.asterisk.org/Configuration/Channel-Drivers/WebSocket/) | Asterisk Project | undated living documentation | 2026-09-12 | Primary framing/timing/buffering behavior; high for behavior, not a production latency benchmark |
| S6 | [Pion webrtc-bench](https://github.com/pion/webrtc-bench) | Pion community | undated living repository, current 2026 | 2026-09-12 | Reproducible transport/load harness; medium, measures setup/CPU rather than voice turns |
| S7 | [OpenBenchmarks Voice Agent Latency](https://github.com/openbenchmarks-labs/voice-agent-latency) | OpenBenchmarks Labs | generated 2026-08-01 | 2026-09-12 | Independent, reproducible carrier-audio measurements; high for tested platforms/topology |
| S8 | [Building Enterprise Realtime Voice Agents from Scratch](https://arxiv.org/abs/2603.05413) | Qiu et al., arXiv | 2026-03-05 | 2026-09-12 | Independent released implementation and measurement; medium-high, preprint and non-Yandex stack |

## Contrary evidence and limits

- The strongest contrary evidence to the staged verdict is architectural, not numeric: a purpose-built Go media plane offers tighter control and may reduce compute or tail latency. No reviewed source, however, connects that control to the claimed 4–5× savings or 280–350 ms far-end result under the proposed workload.
- OpenBenchmarks tests managed platforms and default configurations, not Pipecat/LiveKit/Yandex/Go. Its value is the caller-side method and production-shaped baseline, not vendor ranking for Staffy [S7].
- The paper's 947 ms p50 is a specific cloud/self-hosted model stack and a preprint, not a Russian telephony benchmark [S8]. Agreement in direction between [S7] and [S8] is sufficient to reject unsupported sub-350 ms planning assumptions, not to predict Staffy's exact percentile.
- No second research round was required: the two independent end-to-end sources agree in direction, while the remaining disputed numbers lack public tests rather than conflicting credible tests.
