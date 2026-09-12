# Final citation semantic check

Checked: 2026-09-11  
Scope: semantic support for every numbered source `[1]`–`[39]` in `research.md`; this is not a rewrite of the report.  
Method: current primary pages were re-opened within the 15-call web cap. When a primary page could not be retrieved, a named digest from this run was used only as a secondary trace and is explicitly identified below.

## Verdict

**PASS WITH MATERIAL WORDING DOWNGRADES.** The central recommendation — buy a Russian voice/telephony path for the pilot and build Staffy's transaction/control plane — remains a defensible architectural synthesis. The citations establish the relevant capabilities, operational burden, transaction patterns, public-price anchors, and several real documentation gaps. They do **not** prove that Yandex and Exolve are the market-wide best two vendors, that either path will meet Staffy's PSTN quality/SLO, or that the proposed build/buy split has already won a measured TCO comparison. The report generally labels those uncertainties correctly.

The most important semantic corrections are:

1. `[17]` is a third-party vendor-run benchmark, not evidence that should be described without qualification as an “independent comparison”; telephony is explicitly excluded, while Russian is merely **not described**, not explicitly declared excluded.
2. `[39]` proves priced SpeechKit STT/TTS components, but the pricing page alone does not prove **Russian-language support**. Add a supported-languages citation or drop “российский/русский” from the claim attached only to `[39]`.
3. `[24]` supports explicit 1C transaction APIs, but does not define the transaction boundary of a custom HTTP handler. The report's conservative design is sound; it must be stated as an implementation requirement plus target-version test, not as documented handler behavior.
4. `[9]` clearly supports an Asterisk RTP↔WebSocket/PCM bridge, but this final extraction did not independently confirm the report's exact `G.711↔PCM` codec wording.
5. `[10]`, `[19]`, and `[20]` support advertised Exolve integration/routing/event capabilities, not the runtime, SLA, callback-delivery, or already-connected-call transfer contract of the managed AI agent. The report correctly makes these procurement gates, but every negative must stay scoped to the reviewed public pages.
6. `[5]`, `[24]`, `[27]`, `[32]`, `[37]`, and `[38]` could not be retrieved in this final web pass. Their claims are supported only by the named same-run digests listed below and should not carry “high/current primary verified” confidence until the primary is reopened or archived.

## Coverage of sources `[1]`–`[39]`

Legend: **S** = directly supported; **S/I** = source supports the facts and the nearby conclusion is a reasonable, labelled inference; **P** = only partially supported or scope needs narrowing; **D** = primary inaccessible in this pass, supported only by a named same-run digest; **M** = material mismatch.

| Source | Verdict | What the source actually supports | Semantic limit near the citation |
|---|---|---|---|
| `[1]` | **S** | Yandex Realtime is WebSocket/event based; it accepts/returns audio and text, documents server VAD, Russian/Kazakh-oriented models, SpeechKit voices, function tools, and a session lasting until WebSocket close. A new session is required after closure. | “No numeric maximum session lifetime is public” is a bounded absence from this page, not a contractual assurance that no limit exists. |
| `[2]` | **S** | Default quota is 10 concurrent Realtime sessions and 10 session creations/s; quotas can be raised through support. | It does not prove an approved production quota or capacity above the default. |
| `[3]` | **S/I** | Yandex's 2026-05-28 blog explicitly says SIP integration is Preview and support-assisted; RTP carries traffic to AI Studio. | Historical status only. It cannot establish current GA/non-GA status by itself. The report properly calls current status unverified. |
| `[4]` | **S/I** | Release notes show Realtime initially in Preview (2025-09-24), protocol-format change (2026-04-20), and model changes in 2026. | Absence of a later GA notice is only a bounded negative for the reviewed release-notes page. The page does not itself establish current native-SIP status. |
| `[5]` | **D** | `runtime-maturity-r2-1.md` records 99.90% and an unavailability formula based on HTTP 500/`INTERNAL_SERVER_ERROR` and gRPC 13/`INTERNAL`. | The live legal URL was not retrievable in this pass. The conclusion that this is not an end-to-end WebSocket/SIP/RTP/bridge SLA is an architectural reading of scope, not an SLA promise. |
| `[6]` | **S** | Yandex lists Russia and Kazakhstan regions and says user data is stored inside the selected region. | This general regional rule does not enumerate Realtime raw/generated audio, transcripts, operational logs, support traces, replicas, or backups. The report correctly identifies that gap. |
| `[7]` | **S** | AI Studio says request data is saved by default and documents disabling it for REST, gRPC/SDK, and an OpenAI-compatible client header. | No explicit Realtime WebSocket binding is shown. “Coverage unverified” is supported; “the control does not work for WebSocket” would not be. |
| `[8]` | **S** | Current rates are 0.0264 RUB/input-audio second and 0.0203 RUB/output-audio second; the official worked example totals 7.89 RUB for 60 seconds, including audio, text, and a file-search call. | The 1.99 RUB figure is report arithmetic for `60×0.0264 + 20×0.0203`, not a quoted Yandex price. It is correctly labelled an audio-only subtotal and excludes text/tools. |
| `[9]` | **P** | Current guide documents a FreePBX/Asterisk bridge between RTP and Realtime WebSocket/PCM and makes clear that Staffy would operate the bridge. | This final extraction did not independently verify the exact `G.711↔PCM` wording. Narrow to “RTP↔WebSocket/PCM bridge” unless the precise codec lines are quoted. The guide is user-contributed and is not an HA reference architecture. |
| `[10]` | **S/I** | Exolve advertises managed voice robots/AI agents with recognition, synthesis, interruption/context handling, knowledge, SIP/PBX connection, REST/webhook/MCP integrations, CRM/1C integration, operator handoff with context, Russian storage claim, and 6.5 RUB/min for the AI-agent tier. It also says outbound price includes traffic and additional services. | This is an advertised offer page, not a technical runtime/SLA contract. “Tools” should be described as advertised integrations/interfaces, not assumed function-call semantics. SIP/PBX is a connection option, not shown as included in 6.5 RUB/min. |
| `[11]` | **S** | OpenAI says API use is supported only in listed countries; Russia is absent from the current list. | Supports exclusion from a Russia-production baseline, not a technical comparison of Realtime quality. |
| `[12]` | **S/I** | LiveKit self-hosting puts operations on the deployer; SIP, ingress, and egress are separate services; analytics/telemetry are custom/external; TURN configuration and production deployment are operator responsibilities. | “Mature” and the total on-call burden are architecture assessments, not a contractual property of this page. Redis is evidenced more concretely by `[35]`, not `[12]` alone. |
| `[13]` | **S/I** | The current page marks the text turn detector deprecated and slated for removal in Agents 2.0; its multilingual text model explicitly includes Russian. The audio-detector section does not publish a Russian support list. | The safe claim is “the audio-detector page does not explicitly list Russian support.” Do not convert that absence into “audio detector does not support Russian.” |
| `[14]` | **S** | LiveKit Agents 1.8.1 was released 2026-09-10 and contains new duplex-model support plus multiple fixes and behavior changes. | “Significant” is editorial judgement; the release does support version pinning/regression concern. |
| `[15]` | **S/I** | Pipecat documents Daily, Twilio, Telnyx, Plivo, and Exotel paths and states provider-specific implementations/events vary. | “Predominantly non-Russian providers” is a reasonable inference from the published list, not proof that no Russian carrier can be integrated via SIP/custom transport. |
| `[16]` | **S** | Pipecat v1.9.0 was released 2026-09-11 and contains additions, removals, and explicit deprecations scheduled for 2.0. | Replace “breaking/deprecation signals” with “removals and explicit deprecations” because the release page does not label the release itself as breaking. |
| `[17]` | **P** | Veris reports Pipecat 71.0% versus LiveKit 70.3%, within the reported error bars, and says the choice is not a quality choice on that workload. It explicitly excludes telephony costs/paths. | It is a Veris vendor-run benchmark, not peer-reviewed or automatically “independent.” No Russian corpus is described; that is weaker than proving Russian was explicitly excluded. It also does not represent Staffy's domain. |
| `[18]` | **S** | Exolve documents static/dynamic forwarding, one or up to 10 resources, sequential and parallel dialing, and stopping other attempts after one answer. | These are pre-answer forwarding primitives, not proof of attended/mid-call transfer of a bot-connected call. |
| `[19]` | **S/I** | Exolve documents a JSON-RPC dynamic-routing request, `call_sid`, response `client_id`, `event_URL`, routing modes, per-target timeouts, and a reserve forwarding path when the customer's server does not answer. | Reviewed page does not document callback authentication/signing, retry/backoff, delivery ordering, acknowledgement, or platform response deadline. Scope every negative as “not documented on the reviewed public routing page.” |
| `[20]` | **S** | Inbound forwarding events include stable `call_id`/`call_sid`, optional client correlation, B-party actual-answer (`Start`), hangup/end events, target number, reserve flag, and per-leg definition/end reasons. | Applies to the documented forwarding event contract, not automatically to all Exolve AI-agent runtime events or a warm-transfer API. |
| `[21]` | **S/I** | 1C's OData 3.0 REST interface exposes broad create/read/update/delete over major configuration objects and supports document posting/unposting. | “Too broad for direct model access” is a least-privilege architecture inference, not a vendor statement. |
| `[22]` | **S/I** | 1C permits arbitrary custom HTTP services with controlled paths/methods, request body/headers, and response status/body/headers. | It makes narrow command endpoints possible; it does not guarantee their authorization, idempotency, or business correctness. |
| `[23]` | **S** | 1C extensions can adapt a standard configuration without changing it; the base configuration remains on full vendor support and can update automatically. | This does not prove installability/compatibility for a specific customer's configuration and platform version. The report appropriately leaves that open. |
| `[24]` | **D/P** | `verification-integration-r2.md` records that the Developer Guide defines all-or-nothing transactions and the explicit `BeginTransaction`, `CommitTransaction`, and `RollbackTransaction` APIs. | The guide does not establish that a custom HTTP handler is automatically one transaction. Treat explicit transaction + operation-record atomicity as a design requirement to fault-test on the target version. |
| `[25]` | **S** | ROSSKO `GetCheckout` is SOAP v2.1 and returns `OrderIDS`, successful `ItemsList`, and `ItemsErrorList`; multiple vendor order IDs and partial line outcomes are directly represented. | It does not expose a documented client idempotency/correlation key in the published request schema. That absence is correctly used as a bounded risk, not proof no contractual mechanism exists. |
| `[26]` | **S/I** | ROSSKO `GetOrders` supports lookup by one/list of order IDs and also period/latest-N/status filters; it recommends retrieval by identifiers. | Exact automatic reconciliation after a checkout timeout **before** receiving `OrderIDS` is not established. `unknown/manual_review` is a conservative Staffy policy, not a ROSSKO-prescribed state. |
| `[27]` | **D/P** | `verification-integration-r2.md` records ARMTEK REST + Basic Auth, POST `createOrder`, line-level remainder/errors, and an allocation array with supplier order number/quantity/price/date. | The primary URL was blocked in this pass. Multiple allocation rows/order numbers are supported by schema, but actual split-order behavior should be validated in a sandbox before making it a guaranteed runtime claim. |
| `[28]` | **S/I** | OWASP requires the user to identify/acknowledge significant transaction data, server-side enforcement, protection against modification, execution-time authorization checks, per-operation uniqueness, and limited validity. | The exact Staffy `prepare → confirm → execute`, hash fields, nonce, and invalidation rules are a sound derived design, not text copied from OWASP. |
| `[29]` | **S** | The source is explicitly an expired Internet-Draft. It says a key must not be reused with another payload, completed duplicates should return the prior result, and concurrent duplicates should receive a conflict (409; 422 for changed payload). | Correctly usable only as a pattern, not as a current RFC/standard or evidence that suppliers implement it. |
| `[30]` | **S/I** | AWS describes saga local transactions/compensation and explicitly notes lack of isolation, semantic locking, idempotent participants, and observability needs. | “Saga only where there are multiple independent commits” and “never call a supplier while holding a 1C lock” are Staffy architecture rules derived from the pattern, not AWS-specific mandates quoted by the source. |
| `[31]` | **S** | Exolve's current tariff page shows recording/storage free for one month and paid at longer retention, plus per-call mass automated-call processing fees. | Correctly not convertible into a universal per-minute total without call count, duration, route, and contract terms. |
| `[32]` | **D** | `verification-operations-r2.md` records 1.76 RUB/min including VAT for Russian mobile routes in the official linked PDF. | The PDF could not be retrieved in this final pass. The digest also notes that `2026-01-01` is present in the filename/link, not verified as an effective date in extracted body text. Keep “date in filename,” not “effective from.” |
| `[33]` | **S** | LiveKit's current interactive example displays selected components totalling `$0.0672/min`. | It is a mutable calculator scenario, not a Staffy quote, Russian-region price, or full self-hosted TCO. The report labels this correctly. |
| `[34]` | **S/I** | Current published LiveKit Cloud lists for realtime region groups and SIP regional endpoints contain no Russia; the page separately describes agent deployment regions. | This is a bounded public-list negative. It does not exclude private arrangements, global routing that touches Russia, or future availability. |
| `[35]` | **S/I** | LiveKit documents a production-ready single-VM Server deployment using Docker Compose/Caddy; generated files include Redis configuration and the guide exposes TURN/WebRTC ports and upgrade steps. | “Single VM is not HA for the whole pilot” is a valid topology inference, not a LiveKit SLA statement. The guide covers Server footprint, not the complete voice-agent stack. |
| `[36]` | **S** | The LiveKit Server repository is Apache-2.0 and identifies the Server separately from Agents, SDKs, and companion services. | The Server license does not establish licenses/support/security boundaries for SIP, egress, plugins, model weights, or hosted services. The report correctly separates them. |
| `[37]` | **D** | `verification-operations.md` records that the official consolidation current to 2025-09-01 contains 152-FZ Article 18(5) localization language and treats health information as a special category. | It is not a verified current 2026 consolidation. The report's legal disclaimer is essential; do not state present compliance or completeness from this citation. |
| `[38]` | **D** | `verification-operations.md` records that the original official 323-FZ publication defines medical confidentiality to include seeking care, health state, diagnosis, and information obtained in examination/treatment. | Historical definition only; current Article 13 text, exceptions, and applicability were not revalidated. The report correctly requires counsel. |
| `[39]` | **M/P** | Current SpeechKit pricing proves separately priced streaming STT and TTS component services/billing units. | The pricing page alone does not establish that the streaming paths support Russian. Cite SpeechKit supported-languages/model documentation or rewrite to “доступны компонентные streaming STT/TTS” without the language qualifier. |

## Exact recommended downgrades

These are wording substitutions only; they do not change the report's architecture recommendation.

1. **Build/buy conclusion and shortlist**  
   Current implication: “Yandex/Exolve are the final pair and buy/build is high-confidence.”  
   Use: **“Публичные источники обосновывают Yandex Realtime и Exolve как два кандидата для одинакового bake-off, но не доказывают, что это лучшие два варианта рынка. Граница buy voice/telephony + build transactional control plane — архитектурная гипотеза высокой уверенности, которую ещё нужно подтвердить PSTN-качеством, SLO, договором и TCO пилота.”**

2. **Yandex SIP status `[3][4]`**  
   Replace “нет публичного доказательства GA-статуса native SIP Yandex” with: **“Последний найденный явный публичный статус native SIP — Preview от 2026-05-28; в просмотренной истории релизов более позднее объявление GA не найдено. Текущий договорный статус не подтверждён.”**

3. **Yandex SLA `[5]`**  
   Replace an unqualified “AI Studio SLA 99,90%” with: **“Ранее извлечённый текст SLA AI Studio указывает 99,90% и считает недоступность по перечисленным HTTP/gRPC ошибкам; применимость к Realtime WebSocket/SIP/RTP и customer-operated bridge публично не подтверждена. Первичный legal URL нужно повторно открыть/архивировать.”**

4. **Asterisk codecs `[9]`**  
   Replace “RTP↔WebSocket media bridge, G.711↔PCM” with: **“документированный RTP↔WebSocket/PCM bridge для FreePBX 16/Asterisk 16; конкретные codec/transcoding settings нужно закрепить и проверить на целевой телефонии.”**

5. **Exolve advertised surface `[10]`**  
   Replace “SIP/PBX, tools, CRM/MCP” with: **“подключение через SIP-транк/ВАТС и заявленные интеграции через REST API, webhooks, MCP, CRM/1С; публичная страница не задаёт runtime tool-call contract.”**

6. **LiveKit/Pipecat benchmark `[17]`**  
   Replace “Независимое сравнение … исключило телефонию и русский язык” with: **“Сторонний benchmark Veris сообщает статистически неразрешимую разницу task completion между Pipecat и LiveKit на своём workload; telephony явно исключена, а русский корпус не описан. Этот benchmark не выбирает стек Staffy.”**

7. **Pipecat churn `[16]`**  
   Replace “breaking/deprecation signals” with: **“новые возможности, удаления и явные deprecations с планируемым удалением части API в 2.0.”**

8. **Exolve callback/transfer gaps `[19][20]`**  
   Replace broad “Exolve does not publish …” formulations with: **“На просмотренных публичных страницах dynamic forwarding и inbound forwarding events не найдены deadline, retry/backoff, callback signing/auth, delivery ordering/acknowledgement или attended transfer/bridge уже соединённого bot call; получить эти контракты письменно.”**

9. **1C transaction boundary `[24]`**  
   Replace “автоматическую транзакционность всего HTTP handler предполагать нельзя” with: **“Developer Guide подтверждает explicit transaction API, но не определяет границу транзакции custom HTTP handler. Обернуть local mutation + operation record в явную транзакцию и fault-test поведение на целевой версии/конфигурации.”**

10. **ARMTEK allocations `[27]`**  
    Replace “возвращает остаток и массив распределений по заказам” with: **“публично извлечённая schema содержит line remainder/errors и массив allocation rows с supplier order number; фактическое split-order поведение проверить в sandbox/договорном API.”**

11. **TCO arithmetic `[8][10][31][32][33]`**  
    Keep the arithmetic, but label every value as follows: **“иллюстративный public-rate subtotal для заданного traffic mix; не quote, не бюджет, не lower bound и не cost per completed correct operation.”**  
    The calculations themselves are correct: `60×0.0264 + 20×0.0203 = 1.99`; `1.99 + 1.76 = 3.75`; `7.89 + 1.76 = 9.65`; the 1k/10k/100k multiplications are also correct.

12. **Data residency `[6][7][10][34]`**  
    Use: **“Общее региональное правило Yandex и marketing-claim Exolve о хранении в РФ не заменяют artifact-level data-flow/retention map. Для Realtime нужны письменные ответы по raw/generated audio, transcripts, prompt/tool payloads, logs, support traces, subprocessors, replicas/backups и применимости logging opt-out к WebSocket.”**

13. **Russian component stack `[39]`**  
    Replace “российские streaming STT/TTS доступны `[39]`” with either: **“SpeechKit предлагает тарифицируемые streaming STT и TTS как отдельные компоненты `[39]`”** or add a separate official SpeechKit supported-languages/models citation before retaining the Russian-language qualifier.

14. **Legal caveat `[37][38]`**  
    Keep and, if anything, strengthen: **“Проверены только положения найденной официальной редакции 152-ФЗ на 2025-09-01 и исходное определение 323-ФЗ 2011 года. Текущая полнота, исключения и применимость к конкретному call flow не установлены; нужен current-law review юристом.”**

## Primary-page retrieval failures and secondary traces used

- `[5]` Yandex AI Studio SLA: current legal URL not retrievable by the web tool; secondary trace `runtime-maturity-r2-1.md`, lines 89–96.
- `[24]` 1C Developer Guide transaction page: redirected to the generic support home; secondary trace `verification-integration-r2.md`, lines 26–27.
- `[27]` ARMTEK order-services page: URL blocked by the web tool; secondary trace `verification-integration-r2.md`, lines 23–24 and 31–34.
- `[32]` Exolve outbound-tariff PDF: PDF body not retrievable in this pass; secondary trace `verification-operations-r2.md`, lines 63–65.
- `[37]` official 152-FZ consolidation and `[38]` original official 323-FZ publication: official portal pages were not retrievable in this pass; secondary trace `verification-operations.md`, lines 22–27.

No conclusion above relies on unstated model memory. Unsupported negatives are treated as bounded “not found on the reviewed public page,” not as proof that a capability or contract does not exist.
