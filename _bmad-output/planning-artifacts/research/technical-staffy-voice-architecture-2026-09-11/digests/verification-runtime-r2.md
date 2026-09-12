# Verification — runtime maturity, round 2

Verified artifact: `runtime-maturity-r2-1.md`  
Verification date: 2026-09-11  
Method: fresh-context spot-check against current official public sources. Absence findings below are limited to the named pages and targeted official-domain search; they are not claims that no private, contractual, or unindexed material exists. No independent production-quality source was used because none found in the bounded search could test the specific services more reliably than a paid bake-off or customer telemetry.

## Verdict

**Verified with one bounded wording correction.** The load-bearing conclusions survive: the default Yandex Realtime quota is 10 concurrent sessions; the documented Asterisk route is a customer-operated RTP/WebSocket media bridge with substantial production work left to Staffy; current public evidence does not establish GA for native SIP and an official May 2026 article explicitly calls SIP Preview; the 99.90% AI Studio SLA is not an express end-to-end voice SLA; logging and platform-location statements do not prove the treatment and exclusive Russian residency of every Realtime media artifact; and Exolve advertises a genuinely integrated managed voice product while withholding implementation and contractual detail needed for a programmable-runtime decision.

The correction is to F13: Exolve's product page does say **“unlimited” simultaneous lines**. It therefore should not say concurrency is wholly unspecified. The defensible statement is that Exolve does not publish a technical or contractually enforceable quota/capacity model, sizing method, or overload behavior on that page.

## Claim outcomes

| Claim checked | Outcome | Verification |
|---|---|---|
| Yandex Realtime default quota is 10 concurrent sessions and 10 session-creation requests/s | **verified** | The current quotas table lists both values under Voice agents. The same page distinguishes support-adjustable quotas from immutable limits and directs customers to support for increases. The “pilot-sized” characterization is an architecture judgment, not a vendor fact, but making quota uplift a launch gate is supported. |
| The documented Asterisk connection is a media bridge rather than a native direct media connection | **verified** | The guide says Realtime uses WebSocket/PCM while Asterisk uses RTP and that the protocols are not directly compatible. Its route is dialplan → ARI app → External Media channel → software bridge → Realtime, with local RTP on `127.0.0.1` and only the bridge's outbound AI Studio connection exposed externally. It documents Asterisk 16.6+ for ARI External Media and Asterisk 18+ for AudioSocket. |
| The Asterisk guide leaves production operations with Staffy | **verified, bounded to the guide** | The guide requires PBX/ARI/dialplan configuration, Python dependencies and secrets, a service, per-call media/WebSocket handling, codec conversion, and PBX/audio diagnostics. It explicitly says stopping the bridge prevents calls reaching the agent and shows the `Stasis app ... not registered` failure. In the current guide, targeted checks found no HA topology, load balancer, readiness probe, metrics contract, sizing/load-test result, overload policy, or contact-center transfer/DTMF lifecycle design. This is a documentation-gap finding only; it does not prove such patterns cannot be built or obtained from support. |
| No current public GA evidence for native/direct SIP | **verified; stronger evidence available** | Release notes state that voice assistants and Realtime entered Preview on 2025-09-24, include later Realtime model/protocol changes, and contain no “SIP” entry or later explicit GA promotion in the current page. More directly, Yandex Cloud's 2026-05-28 article says SIP connection is currently in Preview and is performed by Yandex experts after a support request. Thus the memo's bounded conclusion is sound. It should cite this newer explicit SIP-status source rather than relying only on silence in release notes. This still does not establish the contractual status on 2026-09-11; written confirmation remains appropriate. |
| AI Studio's 99.90% SLA is not clearly an end-to-end voice SLA | **verified** | The current SLA publishes 99.90% and 10%/15%/30% compensation bands. It defines unavailability as an error coefficient above 5%, calculated from qualifying HTTP 500 `INTERNAL_SERVER_ERROR` or gRPC 13 `INTERNAL` responses divided by correct requests per minute. It does not expressly describe WebSocket disconnects, stalled/delayed audio, SIP call setup, RTP/media quality, or a customer-operated bridge. The memo correctly avoids treating 99.90% as a call-path SLO. |
| Logging can be disabled, but the public terms do not prove coverage of every Realtime audio artifact | **verified** | The terms allow request/generated-content use for analytics and request use for debugging/training unless the customer disables logging. Clauses 5.5–5.6 say the change is effective after 24 hours and then request information is not stored except for objectively necessary short-term processing and is not used for debugging/training. The defined term `Request` is “textual information.” The terms do not expressly enumerate raw input audio, generated audio, transcripts, WebSocket buffers, or other Realtime-derived artifacts. The memo's voice-specific caution is therefore justified. |
| General Russian infrastructure evidence is not Realtime-specific residency proof | **verified** | Yandex Cloud's privacy page says its data centers are in Russia and their infrastructure meets 152-FZ requirements. The same page discusses responsibilities by service model and notes that cross-border transfer may occur with the data owner's consent after initial placement in Russia. It does not trace AI Studio Realtime raw audio, generated audio, transcripts, logs, replicas, or temporary buffers. The memo correctly treats this as platform evidence rather than proof that every Realtime artifact stays exclusively in Russian regions throughout processing. |
| Exolve advertises an integrated managed voice-agent capability set | **verified as advertising, not independently proven performance** | The current product page advertises inbound and outbound voice flows, speech recognition and synthesis, meaning/context/interruption handling, AI agents and knowledge-base answers, SIP trunk or MTS virtual PBX connection, transfer to an operator with context, CRM actions, REST API, webhooks, MCP, Russian MTS-hosted data, 152-FZ positioning, and an AI-agent price of 6.5 ₽/minute including VAT. This is enough to classify it as an advertised integrated managed product, not enough to validate latency, accuracy, reliability, or contractual semantics. |
| Exolve is opaque for programmable-runtime and production decisions | **disputed in one phrase; otherwise verified** | The product page does not publish an exact realtime media/API contract, tool-call schema, session-state model, numeric maximum call duration, codec matrix, latency target, availability SLA, failure semantics, model/provider list, or retention schedule. However, it does advertise “unlimited” simultaneous lines. Replace “does not specify concurrency quotas” with “does not publish a technical or contractually enforceable concurrency/quota model, capacity methodology, or overload behavior.” A paid bake-off and written questionnaire remain warranted. |

## Recommended source-level correction

For F7, add the 2026-05-28 Yandex Cloud article and tighten the evidence chain:

> Voice assistants and Realtime entered Preview on 2025-09-24. Yandex Cloud then stated on 2026-05-28 that SIP connection was still in Preview and provisioned through support. Current release notes reviewed through 2026-09-10 contain no later explicit SIP GA promotion. Therefore public evidence does not establish GA as of the review date; confirm contractual status and support in writing.

For F13, change the concurrency sentence to:

> The page advertises “unlimited” simultaneous lines but does not publish a technical or contractually enforceable concurrency/quota model, capacity methodology, or overload behavior.

## Official sources checked (8)

1. Yandex AI Studio, quotas and limits: https://aistudio.yandex.ru/en/docs/ai-studio/concepts/limits
2. Yandex AI Studio, Asterisk integration guide: https://aistudio.yandex.ru/en/docs/ai-studio/operations/agents/voice-agent-asterisk-integration
3. Yandex AI Studio, release notes: https://aistudio.yandex.ru/ru/docs/ai-studio/release-notes/
4. Yandex Cloud, “Прототипирование голосовых ИИ‑агентов всего за несколько минут,” 2026-05-28: https://yandex.cloud/ru/blog/voice-agent-and-other-updates
5. Yandex Cloud, AI Studio SLA: https://yandex.ru/legal/cloud_sla_yandex_ai_studio/ru/
6. Yandex Cloud, AI Studio terms: https://yandex.ru/legal/cloud_terms_yandex_ai_studio/ru/
7. Yandex Cloud, data privacy: https://yandex.cloud/ru/security/data-privacy
8. MTS Exolve, AI voice robot product page: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/

