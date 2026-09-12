---
verification_of: go-decision-latency-economics-r1
research_type: technical
validation: high
scope: "C1-C8 semantic support, source independence, and planning-assumption treatment"
verdict: pass_with_material_corrections
source_digest_modified: false
---

# Verification: Go voice-core latency and economics digest

## Overall result

**Pass with material corrections.** The digest is justified in refusing to use **<350 ms**, **280 ms**, **30-50 ms audible interruption**, **4-5x infrastructure savings**, or **2-3 months with 3-4 engineers** as planning inputs. None is supported by a controlled measurement or estimate for the proposed stack. This is a rejection of unsupported planning assumptions, not proof that the targets are impossible.

The principal correction is to the arXiv evidence. In the current v2 paper, **947 ms p50 / 729 ms minimum is an estimated sequential TTFA**, while the authors' separately reported **measured streaming-pipeline TTFA is 755 ms**. The digest therefore must not characterize 947/729 as a measured, reproducible end-to-end run. The [current paper](https://arxiv.org/html/2603.05413v2) explicitly labels the 947/729 row “Est. E2E TTFA” and the 755 ms row “measured end-to-end.” The earlier [v1 abstract](https://arxiv.org/abs/2603.05413v1) called 947 ms a measured p50, but the unversioned citation now resolves to v2, revised 2026-03-17.

The correction weakens the digest's phrase “two reproducible end-to-end data points,” but does not change the decision: the current paper's 755 ms measured result and OpenBenchmarks' caller-side distributions are still useful base-rate evidence, while neither directly tests Go/Yandex/Russian PSTN.

## Claim-by-claim status

| Claim | Verification status | Independence | Required correction or downgrade |
|---|---|---|---|
| **C1** measurement critical path | **Verified with scope qualification** | LiveKit and Pipecat are different publishers and implementations; independent for convergent metric definitions | Keep “measurement model,” not universal physical law. LiveKit's approximation omits text aggregation, leading silence, and caller-side playout; the digest correctly restores them via Pipecat and far-end recording. |
| **C2** Yandex streaming capability; no 280-350 ms SLO | **Verified for capability; latency remains unverified** | S3 and S4 are both Yandex, so they are authoritative primary documentation but not independent publishers | Confidence high for API behavior, not for performance. The two pages support streaming STT and successive-text streaming TTS; they do not provide a Russian-region carrier-to-carrier percentile. |
| **C3** proposed Go architecture returns first audio in <350 ms | **Unverified; exclusion from planning is justified** | OpenBenchmarks and Qiu et al. are independent publishers with different underlying systems, but are base-rate evidence rather than a direct cross-check of the proposed stack | Correct S8: 947/729 is estimated in v2; 755 ms is the measured streaming-pipeline value. Downgrade “materially challenged” to “not validated and inconsistent with the cited public baselines.” Do not call S8 reproducible end-to-end evidence without raw run artifacts or an independent rerun. |
| **C4** token-to-audio streaming yields 280 ms | **Unverified number; qualitative mechanism verified** | Yandex documentation and Qiu et al. are independent and agree that streaming/pipelining is useful; neither validates 280 ms | No numeric status upgrade is warranted. The digest correctly separates mechanism from outcome. Replace any use of 947 ms here with the v2 paper's 755 ms measured pipeline result, while noting its boundary is not a far-end PSTN recording. |
| **C5** local Silero VAD in Go yields 30-50 ms audible stop | **Unverified; exclusion from planning is justified** | LiveKit and Asterisk are independent, but support measurement and media behavior—not the claimed distribution | Dispute the sentence “two 20 ms frames already span 40 ms before those additions.” Asterisk establishes 20 ms `ptime` for its u-law/a-law example, not a two-frame detection requirement or a 40 ms lower bound. |
| **C6** Go reduces infrastructure cost 4-5x | **Unverified; exclusion from business case is justified** | Pion and Pipecat are independent sources, but neither contains the needed comparison | The sources only demonstrate that no cited like-for-like cost study exists. Treat 4-5x as a hypothesis pending matched workload and all-in TCO; it is not overturned. |
| **C7** production Go MVP takes 2-3 months with 3-4 engineers | **Unverified; exclusion from delivery planning is justified** | Qiu et al. and Asterisk are independent, but neither supplies person-month evidence | Downgrade “at most, a prototype estimate” to “an unsupported team estimate.” Public scope complexity does not establish that even a prototype fits this duration. |
| **C8** owning Go now is lower risk | **Not established; staged path is a reasonable decision under uncertainty** | LiveKit and Asterisk independently establish available instrumentation and media plumbing, not comparative total risk | Confidence should be **low-to-medium**, not a factual “verified” conclusion. The staged recommendation follows from reversibility and absent evidence for the rewrite's claimed benefits; it is not directly proven by C1/S5, and Russian availability/compliance/provider risk remains outside this evidence slice. |

## Source checks

### OpenBenchmarks numbers and method — confirmed

The [OpenBenchmarks repository](https://github.com/openbenchmarks-labs/voice-agent-latency) reports:

- 1,296-1,740 ms TTFAB p50 across five platforms.
- 1,768-2,259 ms TTFAB p95.
- 2,078 usable turns from 2,158 measured turns across 550 calls.
- Dual-channel carrier audio, with TTFAB defined as caller speech end to agent audio onset.
- Published per-turn artifacts and a deterministic re-analysis tool.

These figures are semantically represented correctly in the digest. Important limitations are also correctly acknowledged: vendor-default stacks differ, endpointing behavior is not truly equalized, recording-path overhead is included and uncharacterized, quality is not measured, and discarded turns are excluded from latency percentiles. OpenBenchmarks is independent of the five tested vendors, but it remains one benchmark publisher; reproducibility is not the same as independent replication.

### Qiu et al. arXiv numbers — material correction

The digest appears to have retained the wording of the paper's v1 abstract. The [current v2](https://arxiv.org/html/2603.05413v2) reports:

- **947 ms p50 / 729 ms minimum:** “Est. E2E TTFA,” calculated from component results as a sequential upper-bound row.
- **755 ms:** separately measured streaming-pipeline TTFA, decomposed as 296 ms LLM TTFT + 143 ms sentence detection + 316 ms TTS synthesis.
- **702 ms average / 685 ms best:** DashScope Qwen3-Omni cloud audio-to-audio, a different architecture.

The paper releases code, but the reviewed pages do not expose a raw latency dataset or an independent reproduction. S8 should therefore be rated **medium**, not “independent reproducible measurement.” It is independent from OpenBenchmarks by publisher and underlying system, but the two measurements have different boundaries and topologies.

### Asterisk 20 ms framing — confirmed narrowly; inference disputed

The [Asterisk chan_websocket documentation](https://docs.asterisk.org/Configuration/Channel-Drivers/WebSocket/) confirms that u-law/a-law media is sent as 160-byte frames every 20 ms; the driver reports `ptime: 20`, can reframe/retime most codecs, and puts Opus/Speex/G.729 into passthrough mode. It also documents `FLUSH_MEDIA` for discarding queued media during interruption.

What it does **not** establish is that packetization “consumes” a full 20 ms on every path, that Silero needs exactly two telephony frames, or that 40 ms is a lower bound on audible cancellation. Depending on frame phase and buffering, packetization contribution can be less than one full interval or can be much larger because of queued/planned playout. The only defensible conclusion is that 20 ms framing and queue depth are terms to measure.

## Independence audit

- **Strongest independent pair:** S7 OpenBenchmarks and S8 Qiu et al.; independent publishers and systems, but not like-for-like and not two replications of one metric.
- **Convergent implementation documentation:** S1 Pipecat and S2 LiveKit; independent publishers, suitable for validating observability concepts, not performance outcomes.
- **Authoritative but not independent:** S3 and S4 are both Yandex primary documentation. This is adequate for API capability; it cannot provide independent performance validation.
- **Independent but non-confirmatory:** the pairs used for C5-C8 come from different publishers, yet the cited sources document surrounding mechanisms or missing evidence rather than positively validating the disputed number or decision.

## Planning-assumption decision

The digest's negative planning conclusion is sound if phrased precisely:

1. **<350 ms and 280 ms:** reject as committed SLOs because no source tests the proposed stack or supplies the required boundary, workload, region, and percentile. Public systems above 350 ms provide context, not impossibility proof.
2. **30-50 ms interruption:** reject as an audible-stop SLO because VAD compute/frame time is not cancellation-to-far-end-playout latency; no cited distribution measures that outcome.
3. **4-5x savings:** reject from the business case because no matched workload or all-in cost model exists.
4. **2-3 months / 3-4 engineers:** reject from roadmap commitment because neither scope nor empirical team-velocity evidence exists.
5. **Staged modular path:** accept as a reversible risk-management choice, conditional on a benchmark and on separately checking Russian deployment, availability, compliance, and provider economics. Do not present it as an empirically proven universally lower-risk architecture.

## Final disposition

- **Claims retained:** C1, C2 capability, C4 qualitative streaming benefit, and the unverified status of C3-C7.
- **Claims corrected:** S8's 947/729 characterization and the Asterisk-derived 40 ms lower-bound inference.
- **Claim downgraded:** C8 from medium-confidence evidentiary conclusion to low-to-medium-confidence selection judgment.
- **Verdict retained:** managed/modular staged path now; broader Go ownership only after a controlled, far-end benchmark and matched TCO comparison.
