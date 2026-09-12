# Operations / TCO / ecosystem digest — round 1

**Тема:** Staffy voice architecture для первых оплачиваемых пилотов в России  
**Дата доступа ко всем источникам:** 2026-09-11  
**Research scope:** только веб-источники из brief; файлы проекта не читались. Юридические положения ниже — констатация текста официальных источников, не юридическое заключение.  
**Source budget:** 8 уникальных источников, перечисленных в claims.

## Decision signal

Для первых оплачиваемых пилотов наиболее убедителен **гибридный российский контур**:

- self-hosted LiveKit Server + SIP/agent runtime в российском регионе;
- управляемые российские STT/TTS как заменяемые адаптеры (первый кандидат — Yandex SpeechKit);
- записи, транскрипты, traces и eval-наборы — только в контролируемом российском хранилище;
- на старте один отказоустойчивый production-контур без отдельного Kubernetes/SRE-проекта, но с заранее отрепетированным восстановлением и возможностью перейти к multi-node;
- медицинский контрольный сегмент — отдельный tenant/ключи/хранилище/retention policy, без записи аудио по умолчанию и без зарубежных inference-провайдеров до отдельного согласования оснований.

Это минимизирует operational burden по сравнению с полностью self-hosted STT/TTS/LLM, а lock-in — по сравнению с end-to-end managed иностранной voice-платформой. LiveKit Cloud удобнее операционно, но опубликованный список его realtime/SIP/agent regions не содержит Россию. Поэтому region pinning не решает требование российского размещения для этого пилота. Полностью self-hosted AI-стек пока преждевременен: break-even нельзя посчитать без concurrency, duty cycle, GPU/CPU-профиля и стоимости on-call.

Практический выбор между LiveKit Agents и Pipecat следует оставить за 1–2-недельным portability spike. Для минимального числа компонентов default — LiveKit Server + LiveKit Agents; Pipecat стоит выбирать, только если spike покажет существенно более простой swap STT/TTS/LLM и eval harness без второго транспортного слоя.

## Cost anchors and TCO model

- LiveKit Cloud Ship: от $50/мес.; Scale: от $500/мес. Пример калькулятора на текущей странице: agent session $0.0100/мин + telephony $0.0100/мин + LLM $0.0014/мин + STT $0.0058/мин + TTS $0.0300/мин + observability $0.0100/мин = $0.0672/мин, или $672 на 10 000 минут при тех же выбранных компонентах. Это ценовой ориентир, а не смета Staffy.
- На Ship включено 5 000 agent-minutes, далее $0.01/мин; session recordings — 5 000 минут, затем $0.005/мин; observability events — 500 000, затем $0.00003/event. Region pinning появляется на Scale, а contractual support SLA — только Enterprise.
- Yandex SpeechKit streaming STT: 0,1626 ₽ за 15 секунд, с НДС; непрерывная минута = 0,6504 ₽, 10 000 минут = 6 504 ₽ до TTS/LLM/телефонии/хранения. Каждый короткий фрагмент округляется вверх до 15 секунд, поэтому churn сессий увеличивает счёт.
- Yandex SpeechKit TTS v3: 0,1626 ₽ за запрос; запрос обычно ограничен 250 символами и 24 секундами, более длинный текст в streaming/unsafe mode тарифицируется блоками по 250 символов. TTS v1: 1 342 ₽ за 1 млн символов.
- Self-hosted TCO считать как: `(media/SIP/agent compute + Redis/HA + TURN/LB/IP/egress + recording/object storage + logs/traces/metrics + backups + security controls + on-call/support + upgrade/testing labor) / successful call-minutes`, отдельно показывая idle-capacity и peak-concurrency. OSS-лицензия не означает нулевой TCO.

## Claims

### C1 — managed LiveKit has a visible, multi-part per-minute cost and paid operational features

- **claim:** LiveKit Cloud публикует Ship от $50/мес. и Scale от $500/мес.; agent hosting, recordings, observability events, inference and telephony тарифицируются отдельными ресурсами/allowances. Текущий калькулятор показывает пример $0.0672/мин для выбранного end-to-end набора. Это делает managed TCO прозрачнее, но привязывает стоимость и часть runtime/observability к одному поставщику.
- **source_url:** https://livekit.com/pricing
- **publisher:** LiveKit
- **pub_date:** live pricing page; page update date not stated
- **accessed:** 2026-09-11
- **confidence:** high for listed prices; medium for applicability to Staffy until exact selections and volume are fixed
- **class:** fact + inference

### C2 — managed/self-hosted operational split is material

- **claim:** По официальному сравнению LiveKit, при self-hosting команда сама запускает и масштабирует agent servers, подключает model providers, строит observability через внешние/custom OpenTelemetry hooks и отдельно разворачивает SIP и Egress. Cloud включает autoscaling/rollbacks, observability (transcripts, traces, recordings, log drains), SIP/Egress and an uptime target of 99.99%. Поэтому главный self-hosted cost driver на пилоте — инженерная эксплуатация, а не media-minute.
- **source_url:** https://docs.livekit.io/transport/self-hosting/
- **publisher:** LiveKit Documentation
- **pub_date:** live documentation; page update date not stated
- **accessed:** 2026-09-11
- **confidence:** high
- **class:** fact + inference

### C3 — LiveKit Cloud does not publish a Russia region

- **claim:** В опубликованных LiveKit region groups перечислены US, Asia Pacific, Canada, Europe, India, Middle East, Africa, Australia, Israel and South America; SIP endpoints similarly omit Russia; agent deployment regions are US East, EU Central and AP South. Russia is absent. Region pinning also disables nearest-region automatic failover, while multi-location regions provide in-region redundancy. Поэтому LiveKit Cloud cannot be treated as Russian-resident on published evidence.
- **source_url:** https://docs.livekit.io/deploy/admin/regions/endpoints/
- **publisher:** LiveKit Documentation
- **pub_date:** realtime region list updated 2026-05-14; SIP list updated 2026-03-13
- **accessed:** 2026-09-11
- **confidence:** high for published list; medium for unpublished enterprise arrangements
- **class:** fact + decision inference

### C4 — managed Russian speech has a low usage anchor but billing granularity matters

- **claim:** Yandex SpeechKit charges streaming STT at 0,1626 ₽ per 15 seconds including VAT, with each fragment rounded up to 15 seconds; TTS v3 is 0,1626 ₽ per request and longer payloads are billed in 250-character units. This supports managed STT/TTS for low-volume pilots, while requiring connection reuse and usage telemetry to avoid rounding waste.
- **source_url:** https://aistudio.yandex.ru/ru/docs/speechkit/pricing
- **publisher:** Yandex AI Studio / Yandex Cloud
- **pub_date:** live pricing page; page update date not stated
- **accessed:** 2026-09-11
- **confidence:** high for prices and billing units; medium for total voice cost because carrier, LLM and storage are excluded
- **class:** fact + recommendation

### C5 — localization applies to database operations performed during collection

- **claim:** Part 5 of Article 18 of Federal Law No. 152-FZ states that, when collecting personal data of Russian citizens, recording, systematization, accumulation, storage, update/change and extraction using databases outside Russia are not permitted, subject to the statute's listed exceptions. The same official consolidated text defines processing broadly, including recording, storage, use, transfer, depersonalization, blocking, deletion and destruction.
- **source_url:** https://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=98490812b3409e2a8d78a11ca9010f434ea3d9250a11dbbdb78690cd5551bdd6
- **publisher:** Официальный интернет-портал правовой информации (pravo.gov.ru)
- **pub_date:** consolidated text shown as of 2025-09-01
- **accessed:** 2026-09-11
- **confidence:** high for quoted statutory rule; medium for current completeness because the fetched consolidation date is 2025-09-01
- **class:** statutory text, not legal conclusion

### C6 — health information receives additional statutory treatment

- **claim:** The official consolidated 152-FZ text treats personal data concerning health as a special category and provides enumerated grounds/conditions for its processing. Architectural consequence for the pilot (a conservative control, not a legal conclusion): isolate the medical segment, minimize stored content, keep access/audit boundaries separate, and do not assume ordinary call-consent covers medical processing or disclosure.
- **source_url:** https://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=98490812b3409e2a8d78a11ca9010f434ea3d9250a11dbbdb78690cd5551bdd6
- **publisher:** Официальный интернет-портал правовой информации (pravo.gov.ru)
- **pub_date:** consolidated text shown as of 2025-09-01
- **accessed:** 2026-09-11
- **confidence:** high for classification; medium for architecture consequence pending counsel/security review
- **class:** statutory text + conservative architecture recommendation

### C7 — medical-call content may include medical secrecy

- **claim:** The official publication of Federal Law No. 323-FZ defines information about the fact of seeking medical care, health condition, diagnosis, and other information obtained during examination/treatment as medical secrecy, with disclosure governed by statutory rules. For a medical control segment, recordings and transcripts should therefore be treated as potentially containing medical secrecy from capture, not only after structured extraction.
- **source_url:** https://publication.pravo.gov.ru/Document/View/0001201111220007
- **publisher:** Официальное опубликование правовых актов (pravo.gov.ru)
- **pub_date:** 2011-11-22 official publication of law dated 2011-11-21
- **accessed:** 2026-09-11
- **confidence:** high for the original statutory definition; medium for current exceptions because round 1 did not verify the latest consolidated Article 13 on an official portal
- **class:** statutory text + conservative architecture recommendation

### C8 — LiveKit current maintenance is active, but five-year concentration risk is not proven

- **claim:** The official LiveKit server release stream shows recent 1.9.x patch releases with fixes across SIP, telemetry, WebRTC and egress integration. This is evidence of active maintenance in the last six months, not proof of a five-year trajectory or low maintainer concentration. Operationally, pin exact versions and require upgrade/replay tests rather than tracking latest automatically.
- **source_url:** https://github.com/livekit/livekit/releases
- **publisher:** LiveKit on GitHub
- **pub_date:** releases observed through v1.9.12; source crawled within 3 months of access
- **accessed:** 2026-09-11
- **confidence:** high for recent release activity; low for five-year prognosis
- **class:** ecosystem fact + risk inference

### C9 — Pipecat release activity is also strong, but major-version churn is a pilot risk

- **claim:** Pipecat's official releases show v1.0 through v1.7, with v1.7 dated 2026-08-01 and earlier 1.x releases in the preceding months; release notes include OpenTelemetry semantic-attribute changes and dependency-bound changes. This indicates active maintenance, while also demonstrating migration cost for dashboards/integrations. Pin versions and treat observability schema as an owned contract.
- **source_url:** https://github.com/pipecat-ai/pipecat/releases
- **publisher:** Pipecat AI on GitHub
- **pub_date:** latest observed release v1.7.0 dated 2026-08-01
- **accessed:** 2026-09-11
- **confidence:** high for release cadence/change examples; low for five-year prognosis and maintainer concentration
- **class:** ecosystem fact + risk inference

## Observability, eval/replay, scaling and resilience acceptance bar

Pilot should not be called production-ready until the following are demonstrated:

1. **Per-call correlation:** one immutable call/session ID across SIP, media, STT, LLM, tools, TTS and business outcome.
2. **Latency/cost SLOs:** p50/p95 time-to-first-audio, end-of-turn → first-token/audio, interruption success, ASR empty/error rate, TTS/provider errors, hang-up/transfer result, and cost per successful call.
3. **Controlled replay:** encrypted Russian-resident audio/transcript fixture store; deterministic tool mocks; pinned prompts/models/framework versions; replay report before deploy. Medical fixtures must be synthetic or separately governed.
4. **Failure drills:** STT timeout, TTS timeout, LLM timeout, SIP carrier failure, provider rate limit, agent crash, media-node restart and storage outage. Each needs caller-safe fallback, timeout budget and idempotent business actions.
5. **Scale proof:** load test at 2× paid-pilot peak concurrency plus reconnect burst; document headroom and admission control. Do not introduce Kubernetes solely for the pilot unless single-host recovery/peak tests fail.
6. **Recovery:** scripted rebuild from versioned config, encrypted backups, explicit RPO/RTO, restore rehearsal and drain/rollback procedure.
7. **Data lifecycle:** configurable per-tenant recording on/off, retention, deletion, export, access audit, redaction and legal-hold override; default shortest retention.

## Supplier/SLA obligations for a paid pilot

Require these in the order form/DPA/SLA rather than relying on marketing pages:

- exact service region and confirmation that audio, temporary buffers, transcripts, logs, traces, backups and support-access copies remain in the agreed Russian region;
- named legal entity and subprocessors, data flows, purposes, retention, training/no-training terms and deletion certificate;
- availability SLO for each critical service, measurement window, exclusions, service credits and termination right after repeated breach;
- p1/p2 response and restore targets, 24×7 escalation for call-stopping incidents, incident notification deadline and postmortem deadline;
- committed concurrency/rate limits, quota-change process, burst behavior, deprecation notice and price-change notice;
- export APIs/formats for raw audio, transcripts, traces, prompts/config and usage/billing; no fee or technical block on bulk export;
- termination assistance and a tested 30-day exit runbook to a second STT/TTS provider or self-hosted alternative;
- security responsibilities, access logging, key ownership/rotation, vulnerability notification and audit evidence appropriate to the segment;
- no silent model-version changes for production, or at minimum advance notice plus model pinning/rollback and Staffy acceptance tests.

## Contradictions / tensions

1. **Operational ease vs residency:** LiveKit Cloud bundles autoscaling, rollback and observability, but its published regions omit Russia. Self-hosting resolves placement control but moves these responsibilities to Staffy.
2. **99.99% target vs contractual SLA:** LiveKit documentation advertises a 99.99% Cloud uptime target, while the pricing page places “Support SLA” only in Enterprise. The exact contractual availability/remedy at Ship/Scale is not established.
3. **Region pinning vs resilience:** LiveKit says pinning turns off nearest-region automatic failover; multi-location groups retain only in-region redundancy. Residency choice changes the failure model.
4. **Cheap usage vs session granularity:** SpeechKit's per-15-second price is low, but every short fragment is rounded up; reconnect and segmentation patterns can materially raise effective STT cost.
5. **OSS portability vs integration churn:** Both LiveKit and Pipecat show active releases, but Pipecat's OTel field renames demonstrate that dashboards/replay baselines can break even without provider lock-in.

## Leads for round 2

- Obtain written Russia-region/data-flow answers and pilot SLA from Yandex SpeechKit and the selected SIP carrier; verify whether transient audio buffers, provider support logs and backups are inside Russia.
- Read official current consolidated Article 13 of 323-FZ and any 2026 amendments on pravo.gov.ru; have counsel map lawful bases/consents and cross-border rules to each call flow.
- Run GitHub API analysis over 24 months for LiveKit Server/Agents and Pipecat: releases/month, unique human contributors, top-5 commit share, maintainer bus factor, issue/PR median close time. Do not infer trajectory from star snapshots.
- Benchmark one representative Russian call flow on identical fixtures: LiveKit Agents vs Pipecat, each with the same self-hosted transport and SpeechKit adapters. Measure code/config surface, p95 latency, memory, recovery behavior and provider swap time.
- Price Russian SIP minutes/numbers/recording and a minimal HA Russian deployment; include engineering/on-call cost and idle capacity before calculating break-even.
- Verify official licenses, governance, enterprise-support entity and security/deprecation policies for LiveKit and Pipecat in primary sources.

## Not found / unresolved in round 1

- No primary-source production retrospective with reproducible Russian telephony load and failure data was found within the 8-source/15-call budget.
- No defensible self-hosted break-even volume: required traffic profile, concurrency, availability target, instance sizing and labor cost are missing.
- Pipecat backer/enterprise support terms and LiveKit/Pipecat maintainer concentration were not verified in the selected source set.
- Exact current official consolidated wording of 323-FZ Article 13 was not verified; only the official publication source was retained.
- No Russia region appears in LiveKit's published list, but unpublished enterprise/private deployment options were not checked with sales.
- Yandex SpeechKit contractual SLA, data-retention behavior and model-version pinning were not established from the selected pricing source.
- No claim is made here that every voice recording is biometric personal data; that depends on use for identification and requires separate legal mapping.

