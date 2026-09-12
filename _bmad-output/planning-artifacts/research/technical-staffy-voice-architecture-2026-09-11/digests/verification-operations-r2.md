# Verification — operations / TCO / ecosystem, round 2

Date: 2026-09-11  
Artifact checked: `operations-tco-ecosystem-r2-1.md`  
Method: fresh-context spot-check against current first-party sources. Eight source documents were used. No project files other than the named digest were read.

## Overall verdict

**PARTIALLY VERIFIED.** The public price anchors and both scenario tables are arithmetically correct. The Yandex region and logging qualifications are appropriately cautious. The LiveKit VM and Apache-2.0 claims are correct only within their stated component boundaries.

The central architecture phrase **“LiveKit transport + Yandex AI Studio Realtime” is UNVERIFIED as an integration**. The reviewed official materials do not document a Yandex AI Studio Realtime plugin, connector, or interoperability path for LiveKit Agents. LiveKit documents that self-hosted agents use model plugins; this does not make an unlisted provider's Realtime protocol a drop-in integration. The combination is not proven incompatible, so it is not overturned, but it must be labeled an **unverified custom-adapter spike** until a working protocol-level test establishes it.

No material arithmetic error was found. No claim was fully overturned.

## Outcome key

- **VERIFIED** — directly supported by the reviewed first-party source and/or independently recomputed.
- **UNVERIFIED** — the source set does not establish the claim; this does not mean it is false.
- **DISPUTED** — the underlying fact is supported, but the wording or scope overreaches the source.
- **OVERTURNED** — contradicted by current first-party evidence.

## Findings

### 1. Yandex Realtime audio prices and worked example — VERIFIED, with one attribution caveat

The current [Yandex AI Studio pricing page](https://aistudio.yandex.ru/ru/docs/ai-studio/pricing) says Russian-ruble prices include VAT and lists:

- input audio: `0.0264 RUB/second`;
- output audio: `0.0203 RUB/second`;
- separate text-generation and tool-call charges.

The page's worked 60-second support-bot example uses 60 seconds of input audio, 20 seconds of output audio, 7,000 text tokens, and one file-search call. Its components recompute exactly:

```text
60 × 0.0264 = 1.584 RUB
20 × 0.0203 = 0.406 RUB
text          = 5.600 RUB
file search   = 0.300 RUB
total         = 7.890 RUB
```

The current model table also matches the digest's per-1,000-token rates:

| Model | Input | Cached | Tool tokens | Output |
|---|---:|---:|---:|---:|
| Speech Realtime 260528 | 0.10 | 0.025 | 0.025 | 0.20 |
| Speech Realtime 250923 | 0.80 | 0.20 | 0.20 | 0.80 |
| Speech Realtime DeepSeek V4 Flash | 0.30 | 0.075 | 0.075 | 0.50 |

**Caveat:** the worked-example text does not explicitly name `Speech Realtime 250923`; it applies `0.8 RUB` to input and output tokens, which matches that model's row. Describing the example as using that model is a reasonable rate-match inference, not a direct label in the example.

### 2. Exolve telephony anchors — VERIFIED, with two scope qualifications

The [MTS Exolve Base API tariff](https://exolve.ru/tariffs/api/) currently lists, including VAT:

- call recording and storage: one month free; three months `0.04 RUB/min`; six months `0.07 RUB/min`; one year `0.12 RUB/min`;
- recorded-call transcription: `0.65 RUB/min`;
- up to three SIP accounts free; additional accounts `95 RUB/account/month`;
- mass automated call processing: `0.2928–0.3000 RUB/call`, varying by operator.

The digest correctly keeps the automated-calling fee per call. It cannot be turned into a per-minute rate without call count or duration assumptions.

The official outbound tariff PDF linked from that page, [VOIP_Exolve_01_01_2026](https://exolve.ru/uploads/VOIP_Exolve_01_01_2026_ea93fe2f76.pdf), lists `1.76 RUB/min` including VAT for Russian mobile routes across the named federal districts, Moscow/Moscow Oblast, St Petersburg, Crimea, and Sevastopol. It lists Moscow fixed-line examples at `0.85 RUB/min` and St Petersburg at `0.75 RUB/min`.

**UNVERIFIED date wording:** `2026-01-01` is present in the official filename/current link, but the extractable PDF body reviewed here does not state an effective date. The rate itself and its current official linkage are verified; “effective 2026-01-01” should be attributed to the filename/link unless a dated tariff cover or contract exhibit is obtained.

The [Exolve number tariff](https://exolve.ru/tariffs/api/phones/) lists the Base-plan row `Без категории` at `990 RUB` connection and `210 RUB/month`, including VAT.

**DISPUTED label/scope:** “ordinary number” is an informal translation of `Без категории`, not the source's terminology. The rendered page presents tabs for city, mobile, 8-800, and 8-804 numbers but does not make the active category unambiguous in the extracted text. Use “the displayed Base-plan `Без категории` number row” and do not assert from this page alone that the row applies to every number type or specifically to a mobile number.

### 3. Yandex region and logging — VERIFIED as a bounded gap, not as Realtime residency assurance

The current [Yandex Cloud region documentation](https://yandex.cloud/ru/docs/overview/concepts/region) lists Russia and Kazakhstan regions, says user data is stored inside the region and available only inside it, and says data is stored in the region where the resources using it are created. This verifies the digest's general platform/resource-level regional rule.

It does **not** provide a Realtime artifact map. The reviewed page does not separately locate raw input audio, generated audio, transcripts or derived text, prompt/tool payloads, model-request logs, support traces, replicas, or backups. Therefore:

- Russia-only placement for every Realtime artifact remains **UNVERIFIED**;
- “Yandex Realtime in a selected Russia region” is safe only as an intended account/resource configuration, not as a proven end-to-end data-residency result;
- the requested written vendor matrix and protocol acceptance test remain necessary.

The current [AI Studio logging-control page](https://aistudio.yandex.ru/ru/docs/ai-studio/operations/disable-logging) says models save all request data by default. It documents `x-data-logging-enabled: false` for REST request headers and gRPC metadata, plus SDK/OpenAI-client examples, and says requests sent with logging disabled will not be saved on Yandex Cloud servers.

The page does not explicitly bind that control to the AI Studio Realtime WebSocket handshake/session, and it gives no retention, deletion-latency, diagnostic-metadata, replica, backup, or support-trace matrix. The digest's Realtime-specific logging and residency gap is therefore **VERIFIED** and appropriately bounded. The control must not be assumed to apply to Realtime merely because a WebSocket client may technically support headers.

### 4. LiveKit single-VM deployment — VERIFIED for LiveKit Server; DISPUTED as a whole-pilot minimum

The [LiveKit VM guide](https://docs.livekit.io/transport/self-hosting/vm/) explicitly describes setting up a “production-ready LiveKit server on a cloud virtual machine.” It documents Docker Compose, Caddy, generated LiveKit and Redis configuration, TURN/TLS, DNS, firewall ports, and a systemd service. It presents Ingress/Egress as optional and says Egress enables recording capabilities.

The digest correctly states that this is not an HA claim. A single VM remains one failure domain.

**Boundary:** the guide establishes a single-VM deployment for **LiveKit Server**. It does not establish that the entire telephony pilot—including SIP bridge, agent worker, custom Yandex adapter, recording/export worker, observability, backup, and failover—fits safely or is supported on that one VM. Thus “smallest credible pilot footprint” is **DISPUTED** if read as the complete pilot footprint. Prefer: “smallest documented LiveKit Server footprint; total pilot topology and capacity remain to be proven.”

### 5. Apache-2.0 and support boundary — VERIFIED

The official [LiveKit Server repository](https://github.com/livekit/livekit) states that LiveKit Server is Apache License 2.0 and identifies Server, Egress, Ingress, and SIP as separate open-source ecosystem components. It also says self-hosted agents use model plugins instead of LiveKit Inference.

The digest correctly limits the license statement to LiveKit Server and separately calls for validation of every deployed component/container/model. Apache-2.0 does not create a support contract, uptime commitment, hosted service, or warranty. No self-hosted SLA is established by the repository or VM guide.

### 6. “LiveKit transport + Yandex Realtime” — UNVERIFIED custom-adapter spike

Targeted searches of the current official LiveKit documentation/repositories and Yandex AI Studio documentation found no first-party Yandex AI Studio Realtime plugin, provider entry, reference implementation, or compatibility statement. The reviewed LiveKit source says self-hosted agents must use model plugins, while the digest itself supplies no Yandex-specific plugin or adapter evidence.

Accordingly:

- **Do not call this a documented integration or a compatible drop-in.**
- **Do not call it incompatible either.** LiveKit's plugin boundary makes it architecturally plausible, but plausibility is not interoperability evidence.
- Label it: **“proposed custom Realtime provider adapter; unverified until spike completion.”**

The spike should minimally prove:

1. bidirectional audio-format/sample-rate conversion and pacing;
2. session creation/authentication and the exact supported endpoint;
3. mapping of LiveKit audio frames to Yandex client/server events;
4. interruption/barge-in, cancel, and partial-response behavior;
5. tool-call/result mapping and conversation-state ownership;
6. reconnect, timeout, backpressure, and duplicate-event behavior;
7. usage/metering reconciliation;
8. logging-disable behavior on the actual Realtime connection;
9. latency and failure behavior through SIP + LiveKit + adapter + Yandex;
10. license, packaging, and operational ownership for the adapter.

Until those pass, the architecture diagram should show a dashed/experimental adapter boundary and should not be used as evidence of production compatibility.

## Arithmetic audit

### Scenario A — VERIFIED

Per stated carrier call-minute:

```text
Realtime audio = 60×0.0264 + 20×0.0203
               = 1.584 + 0.406
               = 1.990 RUB

AI audio + mobile route = 1.990 + 1.760
                        = 3.750 RUB/min
```

| Minutes | Variable | + 210 number/month | + 990 connection in first month |
|---:|---:|---:|---:|
| 1,000 | 3,750 | 3,960 | 4,950 |
| 10,000 | 37,500 | 37,710 | 38,700 |
| 100,000 | 375,000 | 375,210 | 376,200 |

All values match the digest. The exclusions are correctly disclosed. The result is a stated subtotal, not a floor or full TCO.

### Scenario B — VERIFIED

```text
7.89 Yandex example + 1.76 mobile route = 9.65 RUB/min
```

| Minutes | Variable | + 210 number/month | + 990 connection in first month |
|---:|---:|---:|---:|
| 1,000 | 9,650 | 9,860 | 10,850 |
| 10,000 | 96,500 | 96,710 | 97,700 |
| 100,000 | 965,000 | 965,210 | 966,200 |

Three-month Exolve recording adds `M × 0.04`: `40`, `400`, and `4,000 RUB`. All values match.

Scenario B is valid only as a mapping of Yandex's illustrative 60-second AI session to one connected carrier minute. It is not an observed Staffy workload or an upper bound.

## Required corrections before reuse

1. Replace the decision-summary architecture wording with: **“Russian-hosted LiveKit transport plus a proposed, unverified custom adapter to Yandex AI Studio Realtime; proceed only after a protocol spike.”**
2. Replace “smallest credible pilot footprint” with **“smallest documented LiveKit Server footprint”** unless the SIP, agent, adapter, recording, and observability workloads are separately deployed/sized and tested.
3. Keep Realtime residency and logging-disable coverage explicitly open; the general region page is not artifact-level assurance.
4. Describe the number anchor as the displayed Base-plan `Без категории` row, not a generic/mobile-number price.
5. Attribute `2026-01-01` to the official PDF filename/current link unless a dated cover or binding tariff exhibit confirms the effective date.

## Verdict register

| Item | Outcome |
|---|---|
| Yandex audio unit prices | VERIFIED |
| Yandex token-rate table | VERIFIED |
| Yandex 7.89 RUB worked-example arithmetic | VERIFIED |
| Explicit model name inside worked example | UNVERIFIED; inferred by rate match |
| Exolve recording/transcription/SIP/per-call anchors | VERIFIED |
| Exolve 1.76 mobile route and fixed examples | VERIFIED |
| PDF effective-date wording | UNVERIFIED from body; date appears in filename/current link |
| Exolve `Без категории` price row | VERIFIED |
| “Ordinary/mobile number” generalization | DISPUTED |
| General Yandex regional storage rule | VERIFIED |
| Realtime artifact-level Russia residency | UNVERIFIED |
| Default logging and REST/gRPC opt-out | VERIFIED |
| Realtime WebSocket logging opt-out | UNVERIFIED |
| Single-VM LiveKit Server deployment | VERIFIED |
| Single VM as complete pilot footprint | DISPUTED |
| LiveKit Server Apache-2.0 | VERIFIED |
| Self-hosted support/SLA | UNVERIFIED; none established |
| LiveKit + Yandex Realtime official compatibility | UNVERIFIED |
| Custom-adapter spike requirement | VERIFIED as the correct decision label |
| Scenario A arithmetic | VERIFIED |
| Scenario B arithmetic | VERIFIED |
| Overturned claims | NONE |

## Source register (8 first-party documents)

1. Yandex AI Studio pricing: https://aistudio.yandex.ru/ru/docs/ai-studio/pricing
2. Yandex Cloud regions: https://yandex.cloud/ru/docs/overview/concepts/region
3. Yandex AI Studio logging control: https://aistudio.yandex.ru/ru/docs/ai-studio/operations/disable-logging
4. MTS Exolve Base API tariff: https://exolve.ru/tariffs/api/
5. MTS Exolve outbound VoIP tariff PDF: https://exolve.ru/uploads/VOIP_Exolve_01_01_2026_ea93fe2f76.pdf
6. MTS Exolve number tariff: https://exolve.ru/tariffs/api/phones/
7. LiveKit VM deployment guide: https://docs.livekit.io/transport/self-hosting/vm/
8. LiveKit Server repository: https://github.com/livekit/livekit
