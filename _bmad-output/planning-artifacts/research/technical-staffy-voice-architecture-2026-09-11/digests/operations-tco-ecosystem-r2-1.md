# Operations / TCO / residency — Round 2 digest

Date: 2026-09-11  
Scope: decision-grade due diligence for first paid Staffy voice pilots in Russia.  
Research boundary: prior operations digest and verification note plus current primary web sources only. Eight external sources were used; no project files or ambient context were read.

## Decision summary

The lowest-regret pilot shape remains **Russian-hosted LiveKit transport/control plus a replaceable adapter to Yandex AI Studio Realtime**, but only conditionally:

1. The Yandex Realtime usage bill is now quantifiable, but it is not the whole TCO. At a stated 60 seconds of input audio and 20 seconds of generated audio per call-minute, the known audio charge is 1.99 RUB/min before model text, tools, telephony, infrastructure, labor, support, recording, and any per-call automated-calling fee. With MTS Exolve's nationwide mobile-route anchor, that stated subtotal is 3.75 RUB/min.
2. Yandex's general region documentation supports storage of user data in the selected Russia region, while AI Studio documentation says model requests are logged by default. The documented opt-out is shown for REST/gRPC, not explicitly for the Realtime WebSocket. The reviewed official sources do **not** establish artifact-by-artifact location or retention for Realtime audio, transcripts, operational/support logs, or backups. Medical-segment use therefore requires a written vendor data-flow/retention answer and an acceptance test proving logging is disabled for the actual Realtime interface.
3. A single-VM LiveKit deployment is a documented production-ready starting configuration, not an HA claim. It is the smallest credible pilot footprint when downtime is contractually tolerated. Strict availability requires additional topology, runbooks, and cost inputs that the sources do not price.
4. The sources do not support a managed-vs-fully-self-hosted break-even. Managed Realtime has measurable usage cost; self-hosted models have unmeasured GPU/CPU, peak-concurrency, engineering, on-call, and support costs. Those unknowns must remain explicit rather than be filled with generic benchmarks.

## Claims

### Claim 1 — current Yandex AI Studio Realtime unit prices

- **claim:** Yandex AI Studio lists Realtime voice-agent pricing, including VAT, at 0.0264 RUB per second of input audio and 0.0203 RUB per second of output audio. The total additionally includes model text-generation and tool-call charges. The same page's worked example prices a 60-second session with 60 seconds input audio, 20 seconds output audio, 7,000 text tokens using Speech Realtime 250923, and one file-search call at 7.89 RUB.
- **source URL:** https://aistudio.yandex.ru/ru/docs/ai-studio/pricing
- **publisher:** Yandex Cloud / Yandex AI Studio
- **pub_date:** n.d.; live pricing page retrieved 2026-09-11
- **accessed:** 2026-09-11
- **confidence:** high for the displayed current prices; medium for forward validity because pricing can change
- **class:** official pricing

### Claim 2 — Realtime model token rates are a separate TCO variable

- **claim:** The current pricing table lists per-1,000-token rates that vary by Realtime model. Speech Realtime 260528 lists input/cached/tool/output rates of 0.10/0.025/0.025/0.20 RUB; Speech Realtime 250923 lists 0.80/0.20/0.20/0.80; Speech Realtime DeepSeek V4 Flash lists 0.30/0.075/0.075/0.50. A call-minute cannot therefore be priced from audio duration alone.
- **source URL:** https://aistudio.yandex.ru/ru/docs/ai-studio/pricing
- **publisher:** Yandex Cloud / Yandex AI Studio
- **pub_date:** n.d.; live pricing page retrieved 2026-09-11
- **accessed:** 2026-09-11
- **confidence:** high
- **class:** official pricing

### Claim 3 — general regional placement is documented, but it is not a Realtime artifact map

- **claim:** Yandex Cloud documents Russia and Kazakhstan regions, states that user data is stored within the region and is available only within it, and states that data is stored in the region where resources are created. This supports a Russia-region control at the platform-resource level. It does not, on its own, enumerate Realtime transient audio, derived transcripts, model request logs, support copies, or backups.
- **source URL:** https://yandex.cloud/ru/docs/overview/concepts/region
- **publisher:** Yandex Cloud
- **pub_date:** updated 2026-09-04
- **accessed:** 2026-09-11
- **confidence:** high for the documented regional rule; high that the reviewed page lacks the artifact-level detail needed here
- **class:** official technical documentation; bounded inference

### Claim 4 — AI Studio request data is logged by default; Realtime opt-out coverage is unproved

- **claim:** AI Studio says models save all request data by default and advises disabling logging for personal, confidential, or other sensitive data. It documents `x-data-logging-enabled: false` for REST, gRPC metadata, the SDK, and an OpenAI-compatible client, and says requests with the option disabled are not saved on Yandex Cloud servers. The reviewed page does not explicitly name the Realtime WebSocket protocol or define retention, deletion latency, diagnostic metadata, replicas, or backups. Therefore the opt-out must be verified against the actual Realtime connection rather than assumed.
- **source URL:** https://aistudio.yandex.ru/ru/docs/ai-studio/operations/disable-logging
- **publisher:** Yandex Cloud / Yandex AI Studio
- **pub_date:** n.d.; current documentation crawled within one week of access
- **accessed:** 2026-09-11
- **confidence:** high for default logging and the documented opt-out; high that Realtime-specific applicability is not stated on this page
- **class:** official technical/security documentation; bounded inference

### Claim 5 — official Russian telephony price anchors

- **claim:** MTS Exolve's Base API tariff, including VAT, lists recorded-call transcription at 0.65 RUB/min; call-recording storage for one month at no charge, three months at 0.04 RUB/min, six months at 0.07 RUB/min, and one year at 0.12 RUB/min; up to three SIP accounts at no charge and each additional account at 95 RUB/month. It also lists a 0.2928–0.30 RUB processing charge per call for mass automated calls, depending on operator. That last item is per call and cannot be converted into a per-minute cost without call count or average duration.
- **source URL:** https://exolve.ru/tariffs/api/
- **publisher:** MTS Exolve
- **pub_date:** n.d.; live tariff retrieved 2026-09-11
- **accessed:** 2026-09-11
- **confidence:** high for listed tariff items; medium on applicability because call classification and selected services are contract-specific
- **class:** official carrier pricing

### Claim 6 — outbound minute anchor

- **claim:** MTS Exolve's official outbound VoIP tariff effective 2026-01-01 lists 1.76 RUB/min including VAT to Russian mobile numbers across federal districts, Moscow, and St Petersburg. Fixed-line examples are 0.85 RUB/min to Moscow and 0.75 RUB/min to St Petersburg; these fixed examples should not be generalized to every Russian destination.
- **source URL:** https://exolve.ru/uploads/VOIP_Exolve_01_01_2026_ea93fe2f76.pdf
- **publisher:** MTS Exolve
- **pub_date:** effective 2026-01-01
- **accessed:** 2026-09-11
- **confidence:** high
- **class:** official carrier tariff PDF

### Claim 7 — phone-number anchor

- **claim:** MTS Exolve's Base API tariff lists an ordinary number at 990 RUB connection and 210 RUB/month, including VAT. Number category, inventory, geography, and contract conditions can change the actual quote.
- **source URL:** https://exolve.ru/tariffs/api/phones/
- **publisher:** MTS Exolve
- **pub_date:** n.d.; live tariff retrieved 2026-09-11
- **accessed:** 2026-09-11
- **confidence:** high for the displayed base price; medium for availability/applicability
- **class:** official carrier pricing

### Claim 8 — minimum documented LiveKit self-host footprint

- **claim:** LiveKit's VM guide describes a production-ready deployment generated around Docker Compose, Caddy, LiveKit configuration, Redis, TLS/TURN, a domain/DNS entry, a public IP, firewall openings, and a system service. It marks Ingress/Egress as optional; Egress is needed when the deployment records or exports media. This is a credible single-node pilot minimum, but the page does not claim that one VM is highly available.
- **source URL:** https://docs.livekit.io/transport/self-hosting/vm/
- **publisher:** LiveKit
- **pub_date:** n.d.; current documentation retrieved 2026-09-11
- **accessed:** 2026-09-11
- **confidence:** high
- **class:** official technical documentation; bounded architecture inference

### Claim 9 — open-source boundary and support boundary

- **claim:** The LiveKit Server repository is Apache-2.0 licensed and points to official Docker images and Helm charts. Its open-source ecosystem identifies separate Server, SIP, Egress, and Ingress components. The repository describes LiveKit Cloud as adding hosting, inference, telephony, and observability. The reviewed sources do not create a contractual support or uptime commitment for a self-hosted installation; paid support, response times, and any component-specific license obligations must be separately contracted and reviewed.
- **source URL:** https://github.com/livekit/livekit
- **publisher:** LiveKit / GitHub
- **pub_date:** n.d.; live repository retrieved 2026-09-11
- **accessed:** 2026-09-11
- **confidence:** high for the server license and component map; high that no self-host support SLA is established by the repository
- **class:** official source repository; bounded contract inference

## Parameterized monthly TCO

### Variables

- `M`: connected carrier call-minutes/month: 1,000; 10,000; or 100,000.
- `s_in`, `s_out`: billable Realtime input/output audio seconds per carrier call-minute.
- `T_in`, `T_cache`, `T_tool`, `T_out`: thousands of model tokens per carrier call-minute.
- `r_in`, `r_cache`, `r_tool`, `r_out`: selected model's per-1,000-token rates.
- `N_calls`: calls/month; needed for any per-call automated-calling fee.
- `N_num`, `N_new`: active ordinary numbers and newly connected ordinary numbers.
- `r_rec`: selected recording-storage rate per minute: 0 for one month, 0.04 for three months, 0.07 for six months, or 0.12 for one year.
- `C_peak`: peak concurrent calls. It is deliberately not inferred from monthly minutes.
- `I(C_peak, topology)`: compute, Redis, load balancer, public IP, storage, network egress, backup, monitoring, and incident tooling.
- `L`: engineering, SRE/on-call, security/compliance, QA/evals, and support labor.
- `S`: contracted vendor support and SLA premium.
- `Tool`: billed Realtime tool calls.

### Managed Realtime + carrier formula

```text
RealtimeAudio = M × (s_in × 0.0264 + s_out × 0.0203)
RealtimeText  = M × (T_in×r_in + T_cache×r_cache + T_tool×r_tool + T_out×r_out)
Carrier       = M × route_rate
Recording     = M × r_rec
Numbers       = 210 × N_num + 990 × N_new
AutoCallFee   = [0.2928..0.30] × N_calls, only if contractually applicable

Monthly known usage = RealtimeAudio + RealtimeText + Tool
                    + Carrier + Recording + Numbers + AutoCallFee
Full TCO = Monthly known usage + I(C_peak, topology) + L + S
```

### Fully self-hosted/open-source voice formula

```text
Full TCO_selfhost = Carrier + Recording + Numbers + AutoCallFee
                  + I_livekit(C_peak, topology)
                  + I_models(model sizes, GPU/CPU, batching, latency target, redundancy)
                  + storage/transcript/backup/egress
                  + L_build + L_eval + L_oncall + S
```

No self-hosted numeric total or break-even is claimed: the required model choices, real-time factors, concurrency, hardware quotes, labor rates, availability target, and support contract were not supplied or established by the source set.

### Stated scenario A — known subtotal, not full TCO

Assumptions: `s_in=60`, `s_out=20`, mobile route 1.76 RUB/min, no model-text or tool charge, no recording, no applicable automated-call fee, one existing ordinary number. First month adds one 990 RUB connection.

| Monthly minutes | Realtime audio | Mobile route | Known variable subtotal | Recurring incl. 1 number | First month incl. connection |
|---:|---:|---:|---:|---:|---:|
| 1,000 | 1,990 RUB | 1,760 RUB | 3,750 RUB | 3,960 RUB | 4,950 RUB |
| 10,000 | 19,900 RUB | 17,600 RUB | 37,500 RUB | 37,710 RUB | 38,700 RUB |
| 100,000 | 199,000 RUB | 176,000 RUB | 375,000 RUB | 375,210 RUB | 376,200 RUB |

This is not a floor for every workload and not a production budget. It is the known subtotal under the stated audio mix; text, tools, infrastructure, labor, support, recording, and any per-call fee remain excluded.

### Stated scenario B — Yandex's worked AI example mapped to the same mobile route

Assumptions: the official 7.89 RUB 60-second Yandex example, plus 1.76 RUB/min mobile route, one existing ordinary number, no recording or per-call fee. This is illustrative, not an upper bound.

| Monthly minutes | AI + mobile variable (`9.65 × M`) | Recurring incl. 1 number | First month incl. connection |
|---:|---:|---:|---:|
| 1,000 | 9,650 RUB | 9,860 RUB | 10,850 RUB |
| 10,000 | 96,500 RUB | 96,710 RUB | 97,700 RUB |
| 100,000 | 965,000 RUB | 965,210 RUB | 966,200 RUB |

Three-month carrier recording storage would add 40/400/4,000 RUB at the three volumes. It does not resolve where or how recordings are protected; residency/security terms require separate verification.

## Minimum credible pilot architecture and operating boundary

```text
Russian carrier number/SIP trunk
  -> self-hosted LiveKit SIP bridge
  -> LiveKit Server + Redis + Caddy/TLS/TURN on a Russian VM
  -> Staffy agent worker and vendor-neutral speech/model adapter
  -> Yandex AI Studio Realtime in a selected Russia region
  -> Staffy-owned observability, eval/replay metadata and redacted audit trail

Optional recording:
  LiveKit Egress -> Staffy-controlled Russian object storage
```

The single-VM topology minimizes pilot operations but is one failure domain. Do not sell it as HA. A paid pilot should choose explicitly between:

- **best-effort pilot:** one scripted/rebuildable VM, encrypted configuration backup, carrier reroute/runbook, defined maintenance windows, and an honest RTO/RPO; or
- **availability-bearing pilot:** redundant media/SIP/agent capacity, external/redundant state, health-checked routing, tested failover, and priced on-call coverage. None of those added costs is established here.

LiveKit's Apache-2.0 server license reduces code-access lock-in, but it does not transfer operations or warranty to LiveKit. The application still owns upgrades, CVE response, capacity, TURN reachability, Redis, recordings, backups, monitoring, regression evaluation, and incident response unless a supplier contract expressly assumes them.

## Pilot supplier obligations and exit gates

Before medical-segment or otherwise sensitive production calls, require written answers/contract exhibits covering:

1. Exact Yandex Realtime endpoint and region; physical processing and storage locations for raw audio, generated audio, transcripts/derived text, prompts/tool payloads, service logs, support traces, replicas, and backups.
2. Default and maximum retention per artifact; deletion latency; whether `x-data-logging-enabled:false` or an equivalent is supported on the Realtime WebSocket; and a testable confirmation that request content is not stored.
3. Subprocessors/support-access geography, breach notification, encryption/key options, access logging, deletion/export on termination, and treatment during support investigations.
4. Measured service levels: availability definition, excluded events, latency percentiles, rate/concurrency limits, escalation path, response/restore targets, maintenance notice, and service credits. A repository claim or marketing availability figure is not a pilot SLA.
5. Carrier terms: route scope, rounding/minimum billable duration, CLI requirements, number portability/release, automated-call classification and fee, recording location/retention/deletion, and emergency reroute.
6. Exit package: exportable prompts/configuration, transcripts and audit metadata in documented formats; number portability; deletion certificate; no proprietary business logic inside vendor-only sessions; provider adapter and contract tests; a tested switch to a second model endpoint; and a LiveKit rebuild/runbook owned by Staffy.

Recommended acceptance gate: one non-medical synthetic call proving region configuration, logging-disable behavior, no transcript/recording persistence outside Staffy-controlled storage, usable request IDs without sensitive payloads, carrier billing reconciliation, forced WebSocket/model failure, carrier reroute, and replay/eval from redacted Staffy-owned artifacts.

## Contradictions and tensions

- **General region rule vs artifact-level assurance:** Yandex says user data is stored in the selected region, but the region page does not enumerate Realtime artifacts. The AI Studio page simultaneously says request data is logged by default. These statements are not necessarily inconsistent, but together they are insufficient for a medical data-flow decision.
- **Logging opt-out vs protocol coverage:** AI Studio documents a no-save request flag for REST/gRPC/SDK/OpenAI-compatible clients; the page does not explicitly bind that behavior to the Realtime WebSocket. Treating it as automatically applicable would be an unsupported extrapolation.
- **“Production-ready” vs HA:** LiveKit calls the VM configuration production-ready, while its documented shape is a single VM. Production-ready configuration does not establish redundancy or a contractual uptime level.
- **Lower vendor lock-in vs lower operational burden:** self-hosted Apache-licensed media infrastructure improves portability, but it adds infrastructure, upgrade, security, and on-call ownership. A hybrid adapter contains model lock-in; it does not eliminate operational work.

## Leads for the next verification round

1. Obtain Yandex's written Realtime data-flow/retention matrix and a protocol-specific answer for the logging-disable control, including logs and backups.
2. Request binding Yandex and MTS Exolve quotes, billing increments, concurrency/rate limits, support tiers, and draft DPA/SLA terms. Public list prices are anchors, not accepted commercial terms.
3. Run a 100–300 call synthetic workload to measure `s_in`, `s_out`, token mix, tool calls, `N_calls`, peak concurrency, reconnect/failure rate, and actual carrier rounding. Recalculate with observed distributions, not averages alone.
4. Quote one single-node and one redundant Russian infrastructure topology; record VM/GPU type, Redis, load balancer, IP, storage, egress, monitoring, backup, and on-call costs separately.
5. Validate the licenses of every deployed LiveKit component/container and model, not only the LiveKit Server repository.
6. Verify a current official consolidated text and effective dates for 152-FZ and 323-FZ Article 13 with counsel before freezing medical-segment requirements.

## Not found / deliberately not claimed

- No official artifact-level statement was found that proves all Yandex Realtime audio, transcripts, service/support logs, and backups are processed and stored only in Russia.
- No reviewed official page explicitly states that the AI Studio logging-disable mechanism applies to the Realtime WebSocket.
- No official MTS Exolve source reviewed here establishes recording/transcript storage location, security controls, deletion latency, or medical-data suitability.
- No current official consolidated 152-FZ/323-FZ text was added within this tightly scoped source budget. Prior verification only established an official 152-FZ consolidation current through 2025-09-01 and did not establish a current official 323-FZ Article 13 consolidation. This digest therefore makes no new statutory interpretation or compliance conclusion.
- No Russian-region LiveKit Cloud availability or contractual SLA was established. Managed LiveKit Cloud is not treated as residency-qualified.
- No credible numeric self-hosted voice-model TCO, concurrency-per-GPU, or break-even was found within the source cap; no such values are invented.
- No support SLA for self-hosted LiveKit is established by the open-source repository or deployment guide.

## Source register (8 external primary sources)

1. Yandex AI Studio pricing — https://aistudio.yandex.ru/ru/docs/ai-studio/pricing
2. Yandex Cloud regions — https://yandex.cloud/ru/docs/overview/concepts/region
3. Yandex AI Studio request logging control — https://aistudio.yandex.ru/ru/docs/ai-studio/operations/disable-logging
4. MTS Exolve API tariff — https://exolve.ru/tariffs/api/
5. MTS Exolve outbound VoIP tariff PDF — https://exolve.ru/uploads/VOIP_Exolve_01_01_2026_ea93fe2f76.pdf
6. MTS Exolve number tariff — https://exolve.ru/tariffs/api/phones/
7. LiveKit self-hosting VM guide — https://docs.livekit.io/transport/self-hosting/vm/
8. LiveKit Server source repository — https://github.com/livekit/livekit
