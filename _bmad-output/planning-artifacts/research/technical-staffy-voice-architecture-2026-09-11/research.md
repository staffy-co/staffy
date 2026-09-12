---
title: 'Staffy: минимальная platform, voice и omnichannel architecture'
type: 'technical'
topic: 'Staffy voice architecture'
decision: 'Какие компоненты Staffy брать готовыми, а какие создавать самостоятельно для первых оплачиваемых пилотов'
source: 'native web research'
status: complete
preset: 'standard'
validation: 'high for 2026-09-12 deepening; normal for original run'
created: '2026-09-11'
updated: '2026-09-12'
claims_verified: 32
claims_unverified: 43
claims_disputed: 1
claims_overturned: 1
---

# Staffy: минимальная platform, voice и omnichannel architecture

**Решение, которому служит исследование:** какие компоненты Staffy брать готовыми, а какие создавать самостоятельно для первых оплачиваемых пилотов.

## Резюме решения

**Рекомендация: построить универсальное platform/action/integration core Staffy, а телефонию и media edge сначала подключать сменными компонентами.** Staffy владеет tenant configuration, разговорным состоянием, правами, действиями, domain capabilities, connectors, аудитом и eval. Для первых пилотов не создавать собственные STT, TTS, SIP/PSTN server или полный WebRTC/media stack. Стабильный контракт между Conversation Runtime и voice/media edge следует определить сразу; позднее он позволит вынести latency-sensitive участок в отдельный Go service [40] [43] [46] [55] [56].

1. **Основной пилотный путь:** Novofon предоставляет номер и SIP trunk, Yandex AI Studio Realtime — программируемый voice runtime, Staffy — состояние, tools и бизнес-логику. Прямое SIP-подключение Novofon к Yandex Realtime допускается только после письменного подтверждения статуса функции и приёмочных испытаний; Asterisk-мост остаётся сменным резервным edge, а не ядром продукта [1] [3] [9] [54].
2. **Резервный сравнительный путь:** MTS Exolve AI-agent. Публичная страница заявляет телефонию, распознавание и синтез, перебивания, знания, интеграции и передачу оператору, но не задаёт достаточный технический контракт runtime, SLA и отказов [10].

**Не выбирать заранее LiveKit или Pipecat как production-default.** Self-hosted LiveKit даёт контроль над медиаслоем, но добавляет SIP, TURN, Redis, обновления, мониторинг и on-call; его audio turn detector не заявляет русский язык, а русскоязычная text-модель deprecated перед Agents 2.0 [12] [13]. Pipecat активен и гибок, но готовые телефонные пути ориентированы преимущественно на нероссийских провайдеров [15] [16]. Интеграция LiveKit с Yandex Realtime публично не документирована и является отдельным экспериментом, а не готовым резервом.

**В основе Staffy должны лежать бизнес-инварианты:** конечный автомат управления звонком и передачей оператору, безопасный Action Gateway, реестр идемпотентных операций, узкие адаптеры 1С и поставщиков, компенсации, аудит и воспроизводимые тесты. Модель предлагает типизированное намерение, но не получает учётные данные для прямого доступа и не вызывает 1С или поставщика напрямую [21] [22] [25] [28] [30].

**Ponytail-уточнение (архитектурный синтез Staffy):** эти инварианты не требуют набора отдельных микросервисов. Для первых двух пилотов исследование рекомендует реализовать Action Gateway как модуль Go-приложения, Operation Ledger — как одну append-only таблицу только для изменяющих бизнес-операций, а версионирование — как immutable `runtime_config_hash` плюс версии модели, prompt/toolset/policy, короткий eval gate и откат. Надёжностные основания для idempotency/action record и версионированного выпуска с rollback подтверждают [72] [73] [74], но конкретная форма «модуль + таблица + hash» является проектным решением Staffy, а не предписанием этих источников. Pipecat, LiveKit Agents и managed runtimes используются как источники проверенных runtime-паттернов и сменные реализации, а не превращаются в доменную модель Staffy [59] [60] [61] [63] [65] [67].

**Омниканальный курс:** единого подтверждённого внешнего API для Telegram, MAX, VK, Jivo, Avito, произвольного сайта и мобильного приложения не найдено. Staffy определяет минимальный channel envelope/outbox и подключает официальные каналы тонкими адаптерами; Jivo или прошедший spike self-hosted Chatwoot могут владеть операторским inbox/handoff для конкретного клиента. Первая очередь подключения — web, Telegram, MAX и Jivo; VK и Avito подключаются после account-level contract tests; WhatsApp Business Platform для российского юрлица остаётся непроверенным и отложенным [75] [77] [78] [81] [82] [83] [85] [87] [88].

**Главные блокеры до production:** последний явно опубликованный статус native SIP Yandex, который удалось найти, — Preview от 2026-05-28; более позднего объявления GA в просмотренной истории релизов нет; применимость SLA AI Studio к полному WebSocket/SIP call path не подтверждена; отключение логирования не привязано явно к Realtime WebSocket; на просмотренных страницах Exolve не найдены полные callback/transfer/failure contracts; безопасное автоматическое восстановление заказа ROSSKO после timeout до получения vendor ID не подтверждено [3] [4] [5] [7] [19] [20] [25] [26].

**Уверенность:** высокая в отсутствии подтверждённого фиксированного порога «50 клиентов из-за GIL» и средняя в выводах о будущей масштабируемости до нагрузочного теста Staffy; средне-высокая для поэтапной границы владения. Novofon + Yandex — приоритет проекта, а не доказанный победитель рынка; окончательный voice path остаётся гипотезой до русских PSTN-измерений, SLO, vendor due diligence, проверки data flow и фактического TCO.

**Уточнение после проверки исходного чата от 12 сентября:** продуктовый курс Staffy не должен сужаться до одного вертикального агента или одного голосового стека. Staffy создаёт собственное универсальное multi-tenant ядро конфигурации, разговорного состояния, прав, действий, интеграций, аудита и eval; телефония, media transport и realtime runtime остаются сменными адаптерами. Источники опровергают фиксированный порог «50 клиентов из-за GIL» и уточняют границы Pion/ARI [40] [41] [42] [43] [44] [45]. Отделение собственного platform core от сменного media edge — архитектурное решение Staffy, а не прямой вывод этих источников. Собственный Go data plane остаётся допустимой ранней оптимизацией, когда измерение покажет конкретный bottleneck, но полный SIP/WebRTC/PSTN core не является обоснованным условием первых двух пилотов.

## 1. Ландшафт и зрелость

| Путь | Что даёт | Зрелость для пилота | Нагрузочный риск | Роль в решении |
| --- | --- | --- | --- | --- |
| Novofon SIP trunk | Российский номер, inbound/outbound SIP trunk, маршрутизация и API [54] | Договорный кандидат; реальный Staffy call path не измерен | Codec, DTMF, transfer, firewall/ACL и Novofon↔runtime совместимость требуют приёмки | **Основная телефония первого пилота** |
| Yandex AI Studio Realtime | WebSocket-аудио, server VAD, русскоязычные модели, SpeechKit voices, tool calling [1] | Средняя–высокая для API; качество PSTN не измерено | Native SIP был Preview; default quota — 10 сессий [2] [3] | **Основной программируемый кандидат** |
| Yandex + Asterisk bridge | Документированный RTP↔WebSocket/PCM bridge для FreePBX 16/Asterisk 16 [9] | Документированный пример, не HA reference architecture | PBX, bridge, codecs/transcoding, scaling и failures принадлежат Staffy и требуют проверки | Запасной telephony path |
| MTS Exolve AI-agent | Managed voice loop; подключение через SIP-транк/ВАТС; заявленные REST/webhook/MCP/CRM/1С integrations и handoff [10] | Коммерчески упакован, но технически непрозрачен | Публичная страница не задаёт runtime tool-call contract, capacity methodology и SLA/failure semantics | Резервный turnkey-кандидат |
| Self-hosted LiveKit | Media/SIP/Agents с контролем размещения [12] | Зрелая инфраструктура, но не готовый русский агент | Operations burden; Russian turn handling требует проверки [13] | Резервный spike при жёстком требовании к контролю |
| Pipecat | Provider-neutral pipeline и широкий telephony abstraction [15] | Активно развивается; v1.9.0 содержит новые наблюдатели и breaking/deprecation signals [16] | Российский carrier path не доказан | Spike для переносимости компонентов |
| Компонентный STT+LLM+TTS | Максимальная заменяемость; SpeechKit предлагает тарифицируемые streaming STT и TTS как отдельные компоненты [39] | Технически возможно | Больше задержки, соединений и on-call; выигрыш не доказан | Отложить до появления hard requirement |
| OpenAI Realtime | Технически имеет WebRTC/WebSocket/SIP | Не подходит как стандартный российский production dependency | Россия отсутствует в официальном списке поддерживаемых стран [11] | Не брать в baseline |

### Что стабильно, а что меняется

Стабильнее всего границы: SIP/RTP или provider media stream, двунаправленные audio/events, JSON-schema tool calls и отдельно исполняемые бизнес-команды. Быстро меняются turn detection, interruption policy, provider defaults и lifecycle events. Yandex менял Realtime-формат весной 2026 года; LiveKit Agents и Pipecat выпустили значимые обновления непосредственно перед датой исследования [4] [14] [16]. Поэтому версии и модели нужно закреплять, а обновления пропускать через replay записанных тестовых звонков.

Сторонний benchmark Veris сообщает статистически неразрешимую разницу task completion между Pipecat и LiveKit на своём workload; telephony явно исключена, а русский корпус не описан. Этот benchmark не выбирает стек Staffy [17]. Публичных сопоставимых тестов Yandex, Exolve, LiveKit и Pipecat на российском PSTN не найдено.

## 2. Интеграция и границы системы

Рекомендуемый spine первого пилота:

```text
Российский номер / SIP / carrier-side fallback
        ↓
TelephonyMedia adapter
        ↓
RealtimeAgentSession adapter ── Yandex Realtime | Exolve | future runtime
        ↓ typed intent only
Staffy Action Gateway + durable Operation Ledger
        ↓
1С command adapter ── Supplier adapter: ROSSKO | ARMTEK | другой
        ↓
Conversation/Event Sink + audit + eval/replay
```

Пять собственных границ:

- `TelephonyMedia`: call/leg IDs, DTMF, media, hang-up, transfer и carrier fallback.
- `RealtimeAgentSession`: audio/events, partial/final transcript, interruption, cancel/truncate и tool-call envelope.
- `ActionGateway`: схема команды, tenant policy, авторизация, подтверждение опасного действия и timeouts.
- `OperationLedger`: `operation_id`, idempotency key, payload hash, состояние, provider IDs, retry/compensation и сохранённый результат.
- `ConversationEventSink`: единый trace звонка, модели, инструментов, handoff и бизнес-результата без зависимости от схемы конкретного runtime.

### Телефония и передача человеку

Exolve уже документирует статическую и динамическую переадресацию, последовательный/параллельный дозвон и резервный маршрут на случай недоступности сервера Staffy [18] [19]. События содержат стабильные идентификаторы вызова (`call IDs`), клиентский идентификатор корреляции, фактический ответ вызываемой стороны (`B-party`) и сведения о завершении участков вызова (`legs`) [20]. Это позволяет не строить PSTN/SIP core и держать аварийный маршрут на стороне провайдера телефонии.

На просмотренных публичных страницах dynamic forwarding и inbound forwarding events не найдены предельный срок ответа, правила повторной доставки (`retry/backoff`), подпись или авторизация `callback`, порядок доставки и подтверждения либо attended transfer/bridge уже соединённого с ботом вызова [19] [20]. Поэтому передачу оператору во время звонка нужно отдельно подтвердить до закупки: `dial human leg → observe answer → bridge/transfer → release bot`. До подтверждения ответа оператора нельзя завершать участок вызова абонента (`caller leg`); контекст для оператора нужно зафиксировать до начала перевода.

### 1С и поставщики

1С OData предоставляет широкий набор операций CRUD и проведение документов. Этот набор слишком широк, чтобы напрямую предоставлять его модели как интерфейс инструментов [21]. Узкие команды (`FindCustomer`, `CheckStock`, `PrepareOrder`, `ConfirmOrder`) разумнее реализовать собственным HTTP-сервисом в расширении: 1С поддерживает программируемые HTTP endpoints и расширения без изменения типовой конфигурации [22] [23]. Developer Guide подтверждает explicit transaction API, но не определяет границу транзакции custom HTTP handler. Поэтому каждая write-команда должна оборачивать local mutation + operation record в явную транзакцию, проверять уникальный `operation_id` и fault-test поведение на целевой версии и конфигурации [24].

Единый протокольный коннектор для всех поставщиков нереалистичен. ROSSKO использует SOAP и возвращает несколько order IDs, успешные и ошибочные строки [25]. В ранее извлечённой публичной схеме ARMTEK используются REST/Basic Auth, line remainder/errors и массив allocation rows с supplier order number; фактическое split-order поведение нужно проверить в sandbox или договорном API [27]. Реалистична каноническая модель Staffy с `supplier_orders[]` и `line_outcomes[]`, но каждый поставщик требует отдельный адаптер.

## 3. Транзакционная надёжность

### Опасное действие

Путь заказа должен быть `prepare → confirm → execute`. Подтверждение связывается с неизменяемым payload: tenant, caller/actor, action, payload hash, expiry и nonce. Изменение артикула, количества, цены, доставки или клиента аннулирует подтверждение; финальная проверка происходит непосредственно перед исполнением [28].

### Повтор и неизвестный исход

Срок действия черновика IETF об `Idempotency-Key` истёк, поэтому это не действующий RFC, но описанный в нём паттерн полезен: один и тот же ключ нельзя использовать с другим набором данных; при повторе завершённой операции возвращается сохранённый результат, а конкурентный повтор вызывает конфликт [29]. Staffy должен реализовать эти свойства в собственном ledger даже тогда, когда внешнее API не знает такого заголовка.

После истечения времени ожидания неидемпотентного вызова `checkout` его нельзя автоматически повторять. ROSSKO рекомендует сверять заказы по идентификаторам поставщика, но, если время ожидания истекло до их получения, публичный API не предоставляет точный клиентский идентификатор корреляции [25] [26]. До появления договорной идемпотентности или надёжного поиска такую операцию нужно переводить в состояние `unknown/manual_review`.

### Несколько систем

Шаблон saga нужен только там, где есть несколько независимых транзакционных фиксаций (`commits`): например, подготовить документ в 1С → оформить заказ у поставщика → привязать идентификаторы и подтвердить локальный заказ. Локальные изменения выполняются в короткой транзакции 1С; внешний вызов происходит вне её блокировки. Компенсация — отдельное доменное действие, а не удаление истории. AWS также отмечает, что saga не обеспечивает изоляцию: её участники должны быть идемпотентными, а система должна поддерживать семантические блокировки и наблюдаемость [30].

## 4. Эксплуатационная реальность

- **Capacity:** Yandex Realtime публикует default 10 concurrent sessions и 10 session creations/s; увеличение quota оформляется заранее [2].
- **Session lifetime:** числовой максимум Realtime-сессии публично не указан; после разрыва WebSocket создаётся новая сессия [1].
- **SLA:** ранее извлечённый текст SLA AI Studio указывает 99,90% и считает недоступность по перечисленным HTTP/gRPC ошибкам; применимость к Realtime WebSocket/SIP/RTP и customer-operated bridge публично не подтверждена. Первичный legal URL нужно повторно открыть или архивировать [5].
- **LiveKit self-hosting:** SIP и agent hosting разворачиваются отдельно; масштабирование, keys и observability принадлежат оператору [12]. Single-VM guide — минимальный документированный Server footprint, а не HA всего пилота [35].
- **Turn handling:** LiveKit audio detector не перечисляет русский; text detector поддерживает русский, но deprecated перед Agents 2.0 [13].
- **Framework churn:** LiveKit и Pipecat добавляют возможности, исправления, удаления и явные deprecations; часть Pipecat API планируется удалить в 2.0. Закреплять точные версии, хранить собственную event schema и проверять upgrade на recorded-call regression suite [14] [16].

Не вводить Kubernetes только ради пилота. Сначала измерить пиковое число одновременных сессий и подтвердить, что одну установку можно воспроизводимо развернуть и восстановить. Если платный SLA требует высокой доступности, отдельно оценить резервирование медиаслоя, SIP и состояния агента, маршрутизацию по результатам проверок работоспособности и дежурство специалистов.

## 5. TCO

### Известные ценовые ориентиры

| Состав | Публичный ориентир | Что не включено |
| --- | --- | --- |
| Yandex Realtime audio при 60 c input + 20 c output на минуту звонка | 1,99 ₽/мин [8] | text tokens, tools, carrier, infra, labor |
| Тот же audio + Exolve mobile route 1,76 ₽/мин | **3,75 ₽/мин** [8] [32] | text/tools, number, support, infra, labor |
| Worked example Yandex 7,89 ₽ + mobile route | **9,65 ₽/мин** [8] [32] | Staffy infra/labor/support; это не реальный профиль Staffy |
| Exolve managed AI-agent | 6,5 ₽/мин, телефония и дополнения отдельно [10] | Точная scope/SLA/интеграционная работа |
| LiveKit Cloud calculator example | $0,0672/мин для выбранного набора [33] | Не российский регион; применимость компонентов |

При ставке 3,75 ₽/мин известная сумма переменных затрат составляет 3 750, 37 500 и 375 000 ₽ для 1 000, 10 000 и 100 000 минут соответственно. Для официального примера Yandex с мобильным маршрутом соответствующие суммы составляют 9 650, 96 500 и 965 000 ₽. Все значения — иллюстративные public-rate subtotals для заданного traffic mix: это не quote, не бюджет, не нижняя граница и не cost per completed correct operation. Итог зависит от token mix, tool calls, количества звонков, записи, номеров, peak concurrency, инфраструктуры, QA, безопасности, поддержки и on-call. Break-even полного self-hosting без этих данных не доказуем.

Exolve отдельно тарифицирует часть дополнительных услуг: например, хранение записей сверх первого месяца и обработку массовых автоматических звонков [31]. Эти строки нельзя переводить в единую стоимость минуты без фактических длительностей, количества звонков и условий договора.

## 6. Данные, размещение и compliance

Yandex документирует российские регионы и хранение user data в выбранном регионе [6]. Одновременно AI Studio сохраняет request data по умолчанию; отключение документировано для REST, gRPC, SDK и OpenAI-compatible client, но страница не связывает его явно с Realtime WebSocket [7]. Общее региональное правило Yandex и marketing-claim Exolve о хранении в РФ не заменяют artifact-level data-flow/retention map. Для Realtime нужны письменные ответы по raw/generated audio, transcripts, prompt/tool payloads, logs, support traces, subprocessors, replicas/backups и применимости logging opt-out к WebSocket [6] [7] [10].

LiveKit Cloud не публикует российский realtime/SIP/agent region [34]. Self-hosted LiveKit Server имеет Apache-2.0, но лицензии, поддержка и security boundary каждого дополнительного SIP/Egress/model компонента проверяются отдельно [36].

Проверены только положения найденной официальной редакции 152-ФЗ на 2025-09-01 о локализации при сборе и специальной категории сведений о здоровье [37], а также исходное определение врачебной тайны в публикации 323-ФЗ 2011 года [38]. Текущая полнота, исключения и применимость к конкретному call flow не установлены: для медицинского пилота требуются юрист и отдельный data-flow review; этот отчёт не является юридическим заключением.

## 7. Межфакторные выводы

1. **Managed voice runtime и собственный Staffy control plane совместимы.** Tool call остаётся предложением модели; права, подтверждение, исполнение, повтор и аудит живут вне voice vendor.
2. **Data residency меняет shortlist, но не автоматически требует self-hosting.** Сначала получить Realtime-specific договорные ответы Yandex и Exolve; self-hosted media — резерв, а не самоцель.
3. **Конкурентное преимущество вертикального решения определяется схемой данных и семантикой отказов.** ROSSKO и ARMTEK показывают, что модель результата должна хранить распределения по заказам, частичные результаты и неизвестные исходы, а не только логическое значение успеха.
4. **Handoff и order recovery важнее лабораторной естественности.** Они одновременно затрагивают UX, SLA и финансовую корректность; их нельзя делегировать prompt-у.
5. **Публичная цена минуты не выбирает стек.** Managed products скрывают часть внедрения, а self-hosted OSS скрывает труд и on-call. Сравнивать нужно TCO на завершённую корректную операцию.

## 8. Противоречащие свидетельства

- Yandex — самый прозрачный программируемый кандидат, но его native SIP и Realtime-specific data path имеют публичные пробелы.
- Exolve сокращает число компонентов и уже продаёт integrated agent, но непрозрачность runtime может помешать переносимости и проверке инвариантов.
- Self-hosted LiveKit лучше контролирует размещение, но переносит на Staffy значительную недифференцирующую эксплуатацию; публичной интеграции с Yandex Realtime нет.
- Pipecat обещает гибкую замену providers, но широкий abstraction не доказывает одинаковое turn-taking, cancellation и latency после замены.
- Компонентный российский stack может снизить vendor lock-in, но пока нет измерений, оправдывающих добавочную сложность.

## 9. Рекомендация build / buy / defer

| Решение | Слой | Уверенность |
| --- | --- | --- |
| **Buy / managed** | российский номер, PSTN/SIP, routing, carrier-side reserve; integrated realtime speech loop | Высокая для buy; средняя для поставщика |
| **Build** | `TelephonyMedia` и `RealtimeAgentSession` contracts/adapters; in-process Action Executor; одна append-only operations table; 1С/supplier adapters; минимальный audit/eval harness | Высокая для инвариантов; средне-высокая для pilot scope |
| **Contract/test** | native SIP, mid-call transfer, callback auth/retries, quota/SLA, Realtime logging/residency, supplier idempotency/cancel | Высокая |
| **Spike** | LiveKit↔Yandex custom adapter; Pipecat portability; Asterisk HA bridge; self-hosted Chatwoot как web/Telegram/operator inbox | Средняя–низкая до теста |
| **Defer** | собственные STT/TTS/VAD/media server/general framework; Kubernetes; full self-hosted model stack | Высокая |

### Вход для architecture spine

- Staffy отвечает за соблюдение бизнес-инвариантов; адаптер позволяет заменять voice runtime.
- Каждый внешний event считается потенциально повторным и пришедшим не по порядку.
- Write-tools проходят `prepare/confirm/execute`; payment/refund исключить из первого пилота.
- В расширении 1С реализуются узкие версионированные команды с явной транзакцией и `operation_id`.
- Доменная модель поставщика хранит несколько заказов, распределения по заказам, частичные результаты и исходное состояние поставщика (`raw vendor state`).
- Неизвестный внешний outcome блокирует повтор и уходит в reconciliation/manual review.
- Carrier-side fallback работает даже при полном отказе Staffy.
- Один trace связывает call legs, prompt/model version, tools, approvals, provider IDs и business result.

### Вход для Product Brief

Ценность, за которую платит клиент, — не «естественный голос», а корректно завершённый или безопасно переданный процесс. Обещание первого пилота должно ограничиваться сценариями и write-действиями, для которых подтверждены idempotency, recovery и ответственность.

## 10. План сравнительного пилота

### Основной и резервный пути

1. Основной: Novofon SIP trunk → telephony/media adapter Staffy → Yandex Realtime; native SIP или документированный Asterisk path после due diligence и приёмки [9] [54].
2. Резервный: Exolve turnkey agent с теми же tool contracts, test data и критериями результата [10].

LiveKit, Pipecat или тонкий Go media adapter запускаются как отдельные spikes, если основной путь не удовлетворяет жёсткому требованию к размещению, управлению репликами, протоколам, переносимости, качеству или измеренному p95/p99.

### Набор испытаний

- 100–300 синтетических и разрешённых обезличенных русских звонков: 8 kHz, шум, перебивания, короткие подтверждения, VIN, OEM, бренды, телефоны, даты и цены.
- Tool failures: timeout, 429/5xx, duplicate event/call, process restart, hang-up during tool, unknown supplier result.
- Handoff: answer, busy, no-answer, duplicate transfer, runtime death during dial, сохранение caller leg и context packet.
- Operations: quota, reconnect, model/protocol upgrade, carrier fallback, logging-disable test, deletion/export и billing reconciliation.
- Метрики: task completion, каталогическая корректность, duplicate side effects, безопасная эскалация, p50/p95 end-of-turn→first-audio, interruption recovery, tool success, cost per completed correct operation.
- Go A/B: одинаковые providers, prompts, tools, region и codec; отдельно измеряются framework/media overhead, far-end first audible audio, caller-onset→last-bot-audio, CPU/RAM и cost per successful call-minute [49] [50].

### Критерий перехода к production

- Ноль двойных заказов и неконтролируемых повторов во всём test corpus.
- В каждом тесте передачи оператору с искусственно вызванным отказом вызов либо уходит на резервный маршрут оператора, либо явно возвращается к боту.
- Для каждого действия записи сохранены данные о подтверждении, состояние операции и идентификаторы поставщика либо состояние `unknown/manual_review`.
- Для основного и резервного voice path получены письменные подтверждения параметров SIP, callback, SLA, потоков данных и сроков хранения.
- Согласованная квота превышает измеренный пик с предусмотренным запасом.
- Выбранный стек проходит согласованные product SLO; числовые latency targets назначаются до теста владельцем продукта, а не выводятся из публичного маркетинга.
- TCO пересчитан по фактическому распределению audio seconds, tokens, calls, concurrency и engineering labor.
- Расширение собственного Go data plane проходит четыре decision gates из §12.6; язык не выбирается по числу tenants или рекламной latency.

## 11. Открытые вопросы и vendor gates

1. Каков текущий contractual status native SIP Yandex и какие call controls/codecs/SLA реально включены?
2. Применяется ли отключение логирования к Realtime WebSocket и где находятся все производные артефакты и backups?
3. Как Novofon реализует тёплый transfer с наблюдаемым human answer, какие codec/DTMF/ACL использует и каков callback/event delivery contract; те же вопросы сохраняются для резервного Exolve?
4. Какой путь выигрывает на одном русском PSTN corpus по task completion, latency, interruption и cost per correct completion?
5. Есть ли у конкретного ROSSKO/ARMTEK договора client idempotency, cancel и точный lookup после timeout?
6. Какие версии 1С у первых партнёров и можно ли установить extension без снятия поддержки?
7. Каковы peak concurrency, duty cycle, availability target и стоимость on-call — параметры, без которых self-host break-even не вычисляется?
8. Какая актуальная правовая модель и retention допустимы для медицинских звонков? Требуется отдельная юридическая проверка.

## 12. Углубление: собственный Go media core или поэтапная архитектура

### 12.1. Что в исходном замысле сохраняется

Импортированный чат верно задаёт продуктовую цель: **Core отделён от Tenant Config, один multi-tenant продукт обслуживает голосовые и текстовые каналы, а глубокие действия выполняются через интеграции**. Этот курс совместим с двумя первыми domain pack — автозапчастями/ABCP и услугами/Montagio — и не требует отдельного ядра для каждого клиента.

«Лёгкая настройка» имеет честную границу:

- если нужные domain capability и connector уже существуют, новый клиент действительно подключается конфигурацией, маппингом полей, правами, знаниями, политиками и тестами;
- если у клиента новая ERP, уникальные статусы или неизвестная бизнес-семантика, создаётся один переиспользуемый connector/capability;
- клиентский форк ядра запрещён, но обещание «любой процесс только промптом и RAG» также запрещено: транзакции, права, исключения и проверяемые действия требуют типизированных контрактов.

### 12.2. Что проверка исходного чата изменила

| Утверждение из чата | Итог проверки |
| --- | --- |
| Pipecat придётся полностью переписать на 50-м коммерческом клиенте из-за Python GIL | **Не подтверждено; основание про фиксированный порог 50 из-за GIL опровергнуто.** В документации 50 — текущий максимум пула instances одного Pipecat Cloud deployment с путём запроса большей ёмкости, а сторонний, опубликованный конкурентом синтетический benchmark запускал Pipecat выше 50 одновременных сессий [40] [42]. Это не гарантирует отсутствие будущей переработки при иной нагрузке. |
| GIL не позволяет масштабировать голосовой runtime | **Неверно как общее правило.** В обычном CPython GIL ограничивает одновременное исполнение Python bytecode потоками; I/O-bound concurrency остаётся применимой, а multiprocessing обходит GIL для использования нескольких ядер. Горизонтальное масштабирование контейнерами — архитектурный вывод, а tail latency нужно измерять на Staffy workload [41] [42]. |
| Pion WebRTC напрямую заменяет SIP/PSTN и Asterisk | **Опровергнуто как описание компонента.** Pion реализует WebRTC API; официальный пример для входящего SIP добавляет отдельный `sipgo` [43] [44]. Эти источники не дают готового production-контракта для carrier interoperability, записи, полного call control и HA; такие функции нужно закрыть и проверить отдельно. |
| `Pion + go-ari` исключает Asterisk | **Противоречие.** `CyCoreSystems/ari` — Go-клиент к работающему Asterisk ARI, а не реализация Asterisk [45]. |
| Go гарантирует 280–350 мс first audio и 30–50 мс слышимое прекращение речи | **Не подтверждено.** Приведённые caller-side и pipeline TTFA результаты выше и относятся к другим стекам [51] [52]. Для 30–50 мс от начала речи пользователя до последнего слышимого bot sample в [49]–[53] результата нет; источники лишь задают component metrics и transport/buffer semantics. |
| Go уменьшит инфраструктурные расходы в 4–5 раз, а production core займёт 2–3 месяца | **Не подтверждено.** Нет одинакового workload, полной HA-топологии, engineering/on-call cost и определения production scope. Эти числа нельзя помещать в бюджет или roadmap до A/B spike. |
| SIP нового бизнеса подключается за три минуты | **Не подтверждено.** Документированы IP/port-auth, направления, номер/E.164, статические адреса, сетевой доступ и подтверждение адреса [54]. Codec, DTMF, transfer, ACL/firewall и failure testing — рекомендуемый Staffy acceptance checklist, а не содержание [54]. |

### 12.3. Исправленная архитектурная граница

```text
Tenant Control Plane
  конфигурация · роли · знания · политики · credentials · версии · eval · billing
                              │
Voice / Text / Messenger adapters
  Novofon/SIP · WebRTC · web chat · Telegram/другие каналы
                              │
Conversation Runtime
  session state · memory · model/runtime adapters · turn/handoff policy
                              │
Staffy Action Gateway + Operation Ledger
  schemas · permissions · confirm/execute · idempotency · audit · recovery
                              │
Domain capabilities                 Reusable connectors
  автозапчасти · услуги · ...       ABCP · 1С · Montagio CRM · ...
```

Voice/media edge располагается **перед** Conversation Runtime и заменяется без изменения tenant/domain logic:

- PSTN/SIP-first: Novofon → проверенный SIP/PBX/media adapter; Asterisk допустим как готовый edge, но не становится ядром Staffy. Novofon + Yandex остаётся первым выбранным пилотным направлением, Exolve — резервом; это приоритет проекта, а не доказательство завершённой совместимости [9] [46] [54].
- WebRTC/room-first: self-hosted LiveKit или узкий Pion adapter после проверки конкретной потребности. LiveKit SIP — отдельный service с Redis и публичными SIP/RTP портами, а не бесплатное свойство библиотеки [47]. LiveKit документирует DTMF, transfers/call forwarding и secure trunking; carrier compatibility остаётся непроверенной [48].
- Pipecat: необязательный заменяемый orchestration adapter. Он не является ни стратегической идентичностью Staffy, ни запрещённой технологией.
- Go: допустим с первого дня для тонкого latency-sensitive media service или adapter, если это упрощает конкретный стык. Полный собственный SIP/PSTN core — отдельная инвестиция после доказанного требования.

### 12.4. Матрица решения

Оценки 1–5 являются прозрачным продуктово-архитектурным суждением по утверждённым приоритетам, а не внешним benchmark. Их можно пересчитать после появления команды и нагрузки.

| Вариант | Два пилота, 25% | Надёжность/ops, 20% | Универсальность, 20% | Latency potential, 15% | TCO evidence, 10% | Команда, 10% | Итог |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Полный собственный Go SIP/WebRTC/PSTN core сейчас | 1 | 2 | 5 | 5 | 1 | 2 | **2,75** |
| **Собственное platform core + сменные готовые media edges** | 5 | 4 | 5 | 4 | 4 | 4 | **4,45** |
| Pipecat как центр всей платформы | 4 | 3 | 3 | 3 | 3 | 4 | **3,35** |
| Managed voice runtime как центр всей платформы | 5 | 4 | 2 | 4 | 3 | 4 | **3,75** |

Рекомендация — второй вариант. Это **не отказ от собственной платформы**: Staffy владеет слоями, где живут клиентская ценность и переносимость, а готовые компоненты временно или постоянно закрывают стандартную телеком-инфраструктуру.

### 12.5. Сильнейший аргумент за ранний Go

Red-team нашёл важное производственное свидетельство: OpenAI описывает Go/Pion transceiver для преимущественно 1:1 voice topology, а перенос Python `asyncio` media/inference path на Go улучшил smoothness так, что новый p95 сравнялся со старым p50 [55] [56]. Это показывает, что Go оказался удачным для конкретного OpenAI media/inference hot path и обосновывает сохранение Go как кандидата для Staffy spike; источник не доказывает преимущество Go на Staffy/Yandex/PSTN workload.

Но это не доказывает необходимость собственного полного SIP/PSTN core. Тот же red-team нашёл независимый контрпример: Mattermost имел собственный Go/Pion media daemon `rtcd`, а затем в ветке Calls v2 заменил собственный RTC/server integration на LiveKit [57] [58]. Причина перехода в PR не названа, поэтому нельзя объявлять собственный core ошибкой; пример лишь опровергает тезис, что владение медиаслоем гарантированно исключает будущую миграцию.

### 12.6. Измеримые триггеры для Go data plane

На основе component-level definitions [49] [50] Staffy должен добавить собственные ingress- и far-end-маркеры, зафиксировать vendor-neutral audio/session event contract и инструментировать цепочку `far-end speech stop → ingress → EOU/STT final → LLM first answer token → first TTS text → first non-silent PCM → far-end first audible sample`. Для перебивания измеряется `caller speech onset → last audible bot sample`, а не только время VAD inference.

Расширять собственный Go data plane следует, если контролируемый A/B test на одинаковых providers, prompts, tools, region и codec показывает одновременно:

1. framework/media overhead вне STT/LLM/TTS превышает 20% p95 либо 100 мс p95;
2. Go-вариант сокращает p95 интервала `far-end speech stop → far-end first audible sample` минимум на 25% при одинаковой p99 надёжности и качестве;
3. all-in cost на успешную минуту снижается минимум на 30% после амортизации разработки, HA и on-call;
4. преимущество сохраняется при PSTN loss/jitter, barge-in, tool calls и деградации провайдера.

Эти значения — предложенный decision rule, не найденные рыночные нормы. До A/B test они помогают избежать как преждевременной переработки, так и бесконечного откладывания собственного runtime.

### 12.7. Итог углубления

**Курс продукта восстановлен:** Staffy — универсальная multi-tenant omnichannel платформа AI-сотрудников; автозапчасти и Montagio — первые два reference domain pack. **Технический курс уточнён:** строить собственное platform/action/integration core сейчас, использовать сменный voice/media edge для пилотов и заранее определить стабильный контракт, за которым latency-sensitive участок можно позже вынести в отдельный Go service. Полный собственный SIP/WebRTC/PSTN core не запрещён, но должен выиграть измеримый benchmark или закрыть недоступное готовым компонентам hard requirement.

Уверенность высокая в отсутствии подтверждённого фиксированного порога «50 клиентов из-за GIL» и средняя в выводах о будущей масштабируемости до нагрузочного теста Staffy. Уверенность **средне-высокая** для поэтапного решения. Неопределённости: реальная Novofon↔Yandex совместимость, Russian PSTN latency, calls-per-node, HA/failover, data-flow/retention и фактический TCO.

## 13. Углубление: Pipecat-паттерны, Ponytail и омниканальные подключения

### 13.1. Прямые ответы

**Pipecat и сильные аналоги следует брать в расчёт.** Их ценность для Staffy — не только готовый код. Pipecat, LiveKit Agents и зрелые voice-platform contracts показывают повторяющиеся runtime-паттерны: типизированные события, приоритет управляющих сигналов, сменные transport edges, явное состояние реплики, согласованная отмена при перебивании, фиксация фактически доставленной речи, lifecycle tool jobs, детерминированный handoff и единая событийная основа для traces/evals [59] [60] [61] [63] [64] [65] [66]. Staffy должен перенять эти инварианты, но не копировать точное дерево Pipecat Frames, positional processor API или LiveKit room/job objects в собственную бизнес-модель.

**Связка `model + tools/skills + STT/TTS + RAG + MCP` достаточна как набор возможностей, но не как полный бизнес-runtime.** OpenAI Realtime уже даёт speech session, interruptions, tools, approvals, guardrails и handoff [67]. MCP стандартизует discovery/invocation и OAuth-oriented transport authorization [68]. Спецификация возлагает на реализацию server-side validation, access control и sanitization, а confirmation для чувствительных операций, client-side result validation, timeouts и audit logging формулирует как рекомендуемые защитные меры [69]; elicitation consent при этом не является подтверждением завершения операции [70]. [71] подтверждает, что качество RAG зависит от полноты и релевантности retrieval и не гарантирует точный ответ при неполной выдаче. **Архитектурный вывод Staffy:** использовать RAG для документов и объяснений, а изменяемые цены, наличие, права и статусы читать из операционной системы непосредственно перед действием. [72] подтверждает, что retry/replay может выполнить операцию более одного раза и что внешний idempotency key должен сохраняться между попытками. **Архитектурный вывод Staffy:** для mutating actions дополнительно фиксировать явный outcome и запускать reconciliation после неоднозначного timeout.

**Предыдущая архитектура Staffy была правильной по ответственности, но слишком крупной по форме реализации для двух пилотов.** Red-team подтвердил необходимость контроля бизнес-действий, но рекомендовал свести его реализацию к минимальному модулю и таблице. Отдельные универсальные microservices, собственный workflow engine, vector database, multi-agent hierarchy, distributed write lease и полный channel platform сейчас не нужны. Managed/provider runtime и существующий operator inbox могут закрыть conversation plumbing, пока Staffy сохраняет собственные business invariants [86] [87] [88] [91] [92] [93] [94].

### 13.2. Что именно забирается из Pipecat и аналогов

| Проверенный паттерн | Минимальная реализация Staffy | Что не копировать |
| --- | --- | --- |
| Typed events и priority control lane | Стабильный session/turn/correlation ID; interrupt/cancel/error обходят очередь обычного media | Полную иерархию Pipecat Frames [59] |
| Replaceable transport edge | Канал нормализует ingress/egress; conversation/actions не зависят от SDK | Framework-specific pipeline/room как business source of truth [59] [63] |
| Explicit turn controller | Speech start/end, partial/final, agent output, accepted interruption; профиль по языку/каналу | Один silence threshold для всех каналов [64] |
| Coordinated barge-in | Остановить playback, отбросить queued speech, определить судьбу generation/tool job, записать outcome | «Просто остановить TTS» [59] [61] [64] |
| Heard-output context | В историю попадает доставленная часть ответа, а не весь сгенерированный текст | Считать неуслышанный текст сказанным [60] |
| Tool lifecycle | `requested → authorized → running → succeeded/failed/unknown/cancelled` с timeout policy | Считать function call подтверждением side effect [61] [67] |
| Deterministic handoff | Модель просит перевод; policy выбирает адресата; channel подтверждает outcome; человеку передаётся context packet | Завершать AI-сессию по факту отправки команды transfer [66] |
| Event-aligned tests/telemetry | Одни event names в runtime, traces, replay и assertions; сначала text/tool tests, затем audio cases | Строить собственный observability product [62] [65] |

Эти паттерны подтверждают необходимость хорошей внутренней семантики, но не требуют использования Pipecat в production. Сам Pipecat остаётся допустимым сменным orchestration adapter/spike, особенно для component STT/LLM/TTS path. Документация не доказывает transparent crash resume, exactly-once внешних действий, tenant isolation или production capacity; эти свойства нельзя приписывать фреймворку без отдельных тестов.

### 13.3. Ponytail-аудит: минимальная форма без потери надёжности

| Область | Решение для первых двух пилотов | Когда усложнять |
| --- | --- | --- |
| Conversation/runtime | Использовать provider/framework loop за интерфейсом Staffy | Владеть большей частью только при измеренном latency, capability или residency gap |
| Action Gateway | Один in-process Go-модуль: schema, principal/tenant rights, risk, approval, execute | Выделить сервис, когда несколько независимо развёрнутых runtimes используют один boundary |
| Operation Ledger | Одна append-only `operations` table только для mutating actions и `unknown` outcomes | Workflow engine/event sourcing после появления многошаговых процессов на часы/дни |
| Версионирование | Immutable `runtime_config_hash` + IDs модели, prompt/toolset/policy; небольшой eval gate и rollback | Отдельный registry/canary platform при множестве команд и tenants [73] [74] |
| RAG | Готовый поиск/index для документов с provenance и ACL | Собственный vector/knowledge platform только после измеренного quality/scale gap [71] |
| MCP | Один из connector protocols; native API connector равноправен | Marketplace/discovery после накопления переиспользуемых connectors [68] [69] [92] |
| Channel core | Малый envelope, dedupe, outbox, attachment reference и atomic bot/human owner | Богатая canonical messaging model и публичные SDK после доказанной повторяемости |
| Human inbox | Существующий Jivo; Chatwoot-кандидат для клиентов без подходящего inbox | Собственный Staffy inbox только если внешние решения блокируют ценность продукта |
| Speech/media | Managed STT/TTS/realtime и сменный edge | Thin Go media path по измеримым gates; полный owned stack значительно позже |

Action Gateway и Operation Ledger в этом виде — не «enterprise architecture на будущее». Первый не даёт модели обойти права и validation, второй отвечает на один дорогой вопрос: **была ли внешняя операция выполнена, не выполнена или её исход неизвестен?** Всё остальное — generic event sourcing, сложная compensation platform, универсальный workflow designer — откладывается.

### 13.4. Минимальный channel contract

Универсальность достигается не единым сторонним API, а малым Staffy-owned contract:

```text
InboundMessage
  tenant / connection
  external_conversation_id / external_message_id
  sender / occurred_at / reply_to
  content_parts[text|image|audio|voice|document|location]
  raw_event_ref / channel_capabilities

OutboundCommand
  conversation / idempotency_key
  content_parts / reply_to
  required_capability / expiry
  expected_owner[bot|human]
```

В первом релизе Staffy хранит только dedupe receipt, историю попыток и известный status исходящей доставки, channel-local identity link, attachment pointer и текущего владельца ответа; для изменяющей бизнес-операции отдельно хранится receipt. HTTP `200` означает только принятие API, если сам канал не прислал отдельное событие `delivered/read/listened`. Для Telegram, MAX, VK и Jivo различия в identity, media и receipt semantics подтверждены в пределах описанных capability gaps [75] [77] [78] [79] [80] [81] [82]; для Avito эти технические semantics остаются непроверенными [83]. **Архитектурный вывод Staffy:** channel-local identities нельзя склеивать по имени или похожему номеру.

Webhook adapter должен быстро проверить предусмотренный каналом secret или signature, определить tenant по конфигурации endpoint, записать raw envelope и dedupe key и вернуть acknowledgement; нормализация и подготовка ответа выполняются асинхронно. Повторы считаются штатными. При handoff применяется atomic ownership state: после перехода к человеку бот не отправляет сообщения, пока человек явно не вернул управление. Для одного внешнего аккаунта выбирается один writable path — прямой adapter **или** Jivo/Chatwoot, но не два одновременно.

### 13.5. Каналы и порядок подключения

| Очередь | Канал | Подтверждённый путь | Ограничение до запуска |
| --- | --- | --- | --- |
| Tier 0 | Staffy web widget | Один external loader script; для Django — base template, для ABCP — HTML/footer infoblock [84] | Signed short-lived visitor session, origin allowlist, upload policy |
| Tier 1 | Telegram | Официальный Bot API webhook/secret; Business Connection тестируется отдельно [75] [76] [77] | Бот не начинает обычный диалог сам; receipt semantics ограничены |
| Tier 1 | MAX | Официальный webhook с secret; text/media и mini-app path [78] [79] | Canary после изменений API; handoff/read receipts не подтверждены |
| Tier 1 | Jivo | Bot API для Staffy внутри существующего Jivo; Chat API для собственного widget/app с операторским inbox Jivo [81] [82] | Bot API media и cryptographic request signature требуют contract test |
| Tier 1 protocol | Android/iOS | Тот же Staffy HTTPS/WebSocket/REST protocol; Jivo Chat API возможен, если Jivo — inbox [82] | Публичный Staffy SDK только после стабилизации web protocol |
| Tier 2 | VK | Community messaging adapter [80] | Live account test: callback secret, media, reply window, receipts, limits |
| Tier 2 | Avito | Официальный Messenger API либо существующий Jivo path [83] | Tariff/eligibility и developer contract проверяются в реальном аккаунте |
| Deferred | WhatsApp Business Platform | Не выбран | Официальная Russia-specific API eligibility не подтверждена; app не доказывает Platform access [85] |

Первичное хранилище ПДн и Staffy ledger размещаются в согласованном российском контуре; для каждого connector ведётся data-flow map. Сам факт self-hosting Staffy не доказывает, где канал хранит payload, attachments, support logs и backups. Трансграничные потоки требуют отдельной правовой проверки по фактической схеме [37].

### 13.6. Jivo и Chatwoot: не два ядра, а выбор operator desk

Jivo уже предоставляет bot-to-agent flow и подходит клиенту, чьи сотрудники работают в Jivo [81] [82]. Для компании Олега это самый короткий первый путь: Staffy подключается как bot provider, а Jivo остаётся human inbox; прямой Telegram/MAX adapter не должен параллельно отвечать от той же business identity.

Self-hosted Chatwoot — новый кандидат на однодневный spike для клиентов без подходящего inbox. Текущие материалы показывают website и Telegram inbox, generic API Channel, assignment, contact/conversation UI и AgentBot handoff через `pending ↔ open`; Community Edition допускает self-hosting, но эксплуатация остаётся на владельце [86] [87] [88] [89]. Нативный MAX-адаптер в изученных материалах Chatwoot не подтверждён [87]; поэтому тонкий bridge через API Channel остаётся гипотезой для spike, а необходимость хранить provider IDs и dedupe state следует из требований MAX [90]. Полная граница российского data perimeter также не доказана. Поэтому решение — **не adopt now**, а один bounded spike на pinned version.

Spike проходит, только если подтверждены: web+Telegram round trip; AgentBot↔human handoff без двойного ответа; MAX bridge с dedupe/restart/media degradation; допустимый egress inventory; export/restore; upgrade contract tests. Если любой из первых трёх пунктов не проходит, Chatwoot остаётся только источником patterns. Jivo и Chatwoot нельзя держать двумя writable operator desks одного разговора.

Red-team также нашёл Twilio Agent Connect и managed Agent Sessions как доказательство, что channel/session plumbing можно покупать, а не строить [91] [93] [94]. Но это не готовый выбор для Staffy: найденный Twilio channel set не включает Telegram/MAX/Jivo, а Managed Agents session API находится в beta и не доказывает требуемый российский data/retention contract. Эти решения подтверждают принцип reuse, но не заменяют vendor due diligence.

### 13.7. Итог для Staffy

Универсальная архитектура конкурентов не является обманом: модель, tools, RAG/MCP и speech providers действительно дают готовый набор возможностей для быстрого построения разговорного слоя. Заблуждение начинается, когда этот слой объявляют глубоко интегрированным AI-сотрудником без проверки прав, актуальности данных, side effects, повторов, handoff и управления версиями.

Staffy должен отличаться не количеством инфраструктуры, а **надёжным завершением бизнес-процесса при минимальном собственном коде**:

1. брать voice/runtime/channel patterns и готовые компоненты;
2. владеть конфигурацией роли, business capabilities, правами и mutating-action receipts;
3. начинать с in-process modules и одной таблицы, а не microservices;
4. подключать сайты одним widget loader, приложения — тем же protocol, мессенджеры — official adapters или выбранный operator desk;
5. превращать новый integration work в переиспользуемый connector, а не tenant fork.

Уверенность **высокая** в необходимости application-owned authorization/validation для mutating tools и **средне-высокая** в минимальном action record/reconciliation boundary [68] [69] [72] [92]. Уверенность **средняя** в channel rollout order и **низкая до spike** в Chatwoot/MAX compatibility, cross-channel identity automation, full data perimeter и WhatsApp Business Platform eligibility.

## Приложение A. Источники

| [n] | Что подтверждает | Издатель | Дата публикации | Дата доступа | Уверенность |
| --- | --- | --- | --- | --- | --- |
| [1] | Realtime events, audio, VAD, tools и session lifecycle | [Yandex AI Studio](https://aistudio.yandex.ru/ru/docs/ai-studio/concepts/agents/realtime) | текущая документация | 2026-09-11 | высокая для интерфейса |
| [2] | Realtime quotas: 10 sessions / 10 creations per second | [Yandex AI Studio](https://aistudio.yandex.ru/en/docs/ai-studio/concepts/limits) | текущая документация | 2026-09-11 | высокая |
| [3] | SIP Preview и support-assisted integration | [Yandex Cloud](https://yandex.cloud/ru/blog/voice-agent-and-other-updates) | 2026-05-28 | 2026-09-11 | высокая исторически; текущий статус unverified |
| [4] | Realtime Preview и изменения протокола/моделей | [Yandex AI Studio](https://aistudio.yandex.ru/ru/docs/ai-studio/release-notes/) | обновляется; просмотрено до 2026-09-10 | 2026-09-11 | высокая; отсутствие GA — bounded negative |
| [5] | Ранее извлечённый SLA AI Studio 99,90% и формула расчёта | [Yandex Cloud](https://yandex.ru/legal/cloud_sla_yandex_ai_studio/ru/) | 2025-09-24 | 2026-09-11 | средняя; primary недоступен в финальном проходе |
| [6] | Региональное размещение user data | [Yandex Cloud](https://yandex.cloud/ru/docs/overview/concepts/region) | 2026-09-04 | 2026-09-11 | высокая для общего правила |
| [7] | Default logging и способы отключения | [Yandex AI Studio](https://aistudio.yandex.ru/ru/docs/ai-studio/operations/disable-logging) | текущая документация | 2026-09-11 | высокая; WebSocket coverage unverified |
| [8] | Realtime audio/token/tool pricing и worked example | [Yandex AI Studio](https://aistudio.yandex.ru/ru/docs/ai-studio/pricing) | текущая документация | 2026-09-11 | высокая |
| [9] | Asterisk RTP↔WebSocket/PCM bridge | [Yandex AI Studio](https://aistudio.yandex.ru/ru/docs/ai-studio/operations/agents/voice-agent-asterisk-integration) | текущая документация | 2026-09-11 | высокая для примера; не HA |
| [10] | Managed AI-agent, SIP, tools, handoff, data claim и цена | [MTS Exolve](https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/) | текущая страница | 2026-09-11 | высокая для advertised offer |
| [11] | Поддерживаемые страны OpenAI API | [OpenAI](https://help.openai.com/en/articles/5347006-openai-api-supported-countries-and-territories) | обновляется | 2026-09-11 | высокая |
| [12] | LiveKit self-hosting boundary для Agents/SIP/observability | [LiveKit](https://docs.livekit.io/transport/self-hosting/) | текущая документация | 2026-09-11 | высокая |
| [13] | Turn-detector language support и deprecation | [LiveKit](https://docs.livekit.io/agents/logic/turns/turn-detector/) | текущая документация | 2026-09-11 | высокая |
| [14] | LiveKit Agents 1.8.1 changes and fixes | [LiveKit GitHub](https://github.com/livekit/agents/releases/tag/livekit-agents@1.8.1) | 2026-09-10 | 2026-09-11 | высокая |
| [15] | Pipecat telephony transports/providers | [Pipecat](https://docs.pipecat.ai/pipecat/telephony/overview) | текущая документация | 2026-09-11 | высокая для listed paths |
| [16] | Pipecat 1.9.0 changes/deprecations | [Pipecat GitHub](https://github.com/pipecat-ai/pipecat/releases/tag/v1.9.0) | 2026-09-11 | 2026-09-11 | высокая |
| [17] | Сторонний benchmark Pipecat/LiveKit; telephony исключена, русский корпус не описан | [Veris AI](https://veris.ai/blog/vamos-bench-pipecat) | 2026 | 2026-09-11 | средняя; vendor-run ограниченный benchmark |
| [18] | Переадресация и multi-resource routing | [MTS Exolve](https://exolve.ru/faq/kak-prinimat-vkhodyashchie-vyzovy-cherez-pereadresatsiyu/) | текущая документация | 2026-09-11 | высокая |
| [19] | Dynamic routing JSON-RPC, reserve и client_id | [MTS Exolve](https://docs.exolve.ru/docs/ru/instructions/call-forwarding-to-url/) | текущая документация | 2026-09-11 | высокая; callback gaps bounded |
| [20] | Call events, IDs, answer/end states | [MTS Exolve](https://docs.exolve.ru/docs/ru/api-reference/statistics-api/call-events/) | текущая документация | 2026-09-11 | высокая |
| [21] | Широкий OData/CRUD surface 1С | [1С](https://v8.1c.ru/platforma/rest-interfeys/) | текущая документация | 2026-09-11 | высокая |
| [22] | Программируемые HTTP-сервисы 1С | [1С](https://v8.1c.ru/platforma/http-servisy/) | текущая документация | 2026-09-11 | высокая |
| [23] | Расширения и сохранение поддержки типовой конфигурации | [1С](https://v8.1c.ru/platforma/rasshireniya/) | текущая документация | 2026-09-11 | высокая |
| [24] | Explicit transactions 1С | [1C:Enterprise Developer Guide](https://kb.1ci.com/1C_Enterprise_Platform/Guides/Developer_Guides/1C_Enterprise_8.3.23_Developer_Guide/Chapter_9._Data_operations/9.2._Transactions/9.2.1._General_information/) | версия 8.3.23 | 2026-09-11 | средняя; primary недоступен в финальном проходе, handler boundary unverified |
| [25] | ROSSKO checkout, multiple IDs и partial result | [ROSSKO API](https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout) | текущая public v2.1 | 2026-09-11 | высокая |
| [26] | ROSSKO order lookup/status by vendor IDs | [ROSSKO API](https://api.rossko.ru/index.php?action=SOAP&path=/GetOrders) | текущая public v2.1 | 2026-09-11 | высокая |
| [27] | Ранее извлечённая ARMTEK schema: REST/auth и allocation rows | [ARMTEK Web Services](https://ws.armtek.ru/?alias=order&page=service) | public v1.1.7 | 2026-09-11 | средняя; primary недоступен в финальном проходе |
| [28] | Transaction authorization и payload binding | [OWASP](https://cheatsheetseries.owasp.org/cheatsheets/Transaction_Authorization_Cheat_Sheet.html) | текущая страница | 2026-09-11 | высокая |
| [29] | Idempotency semantics и timeout ambiguity | [IETF Datatracker](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/) | draft -07, 2025-10-15; expired | 2026-09-11 | средняя; паттерн, не RFC |
| [30] | Saga, compensation, idempotency и observability | [AWS Prescriptive Guidance](https://docs.aws.amazon.com/pdfs/prescriptive-guidance/latest/cloud-design-patterns/cloud-design-patterns.pdf) | текущий PDF; saga added 2023-11-16 | 2026-09-11 | высокая для patterns |
| [31] | Exolve recording/SIP/per-call tariff anchors | [MTS Exolve](https://exolve.ru/tariffs/api/) | текущая страница | 2026-09-11 | высокая для тарифа |
| [32] | Ранее извлечённая ставка Exolve mobile route 1,76 ₽/мин | [MTS Exolve](https://exolve.ru/uploads/VOIP_Exolve_01_01_2026_ea93fe2f76.pdf) | дата в имени официального PDF: 2026-01-01 | 2026-09-11 | средняя; PDF недоступен в финальном проходе |
| [33] | LiveKit Cloud pricing calculator | [LiveKit](https://livekit.com/pricing) | текущая страница | 2026-09-11 | высокая для цен; применимость средняя |
| [34] | LiveKit Cloud realtime/SIP/agent regions | [LiveKit](https://docs.livekit.io/deploy/admin/regions/endpoints/) | списки обновлены 2026-03—2026-05 | 2026-09-11 | высокая; private arrangements unverified |
| [35] | Single-VM LiveKit Server deployment | [LiveKit](https://docs.livekit.io/transport/self-hosting/vm/) | текущая документация | 2026-09-11 | высокая для Server footprint |
| [36] | LiveKit Server Apache-2.0 и component map | [LiveKit GitHub](https://github.com/livekit/livekit) | текущий репозиторий | 2026-09-11 | высокая для Server license |
| [37] | Найденная редакция 152-ФЗ: localization и health-data category | [Официальный портал правовой информации](https://ips.pravo.gov.ru/api/ips/legislation/document?baseid=None&hash=98490812b3409e2a8d78a11ca9010f434ea3d9250a11dbbdb78690cd5551bdd6) | консолидация на 2025-09-01 | 2026-09-11 | средняя; primary недоступен в финальном проходе, текущая полнота unverified |
| [38] | Исходное определение врачебной тайны в 323-ФЗ | [Официальное опубликование правовых актов](https://publication.pravo.gov.ru/Document/View/0001201111220007) | 2011-11-22 | 2026-09-11 | средняя; primary недоступен в финальном проходе, текущие исключения unverified |
| [39] | Тарифицируемые streaming STT/TTS как отдельные компоненты | [Yandex SpeechKit](https://aistudio.yandex.ru/ru/docs/speechkit/pricing) | текущая документация | 2026-09-11 | высокая для pricing/availability |
| [40] | 50 — лимит pool одного Pipecat Cloud deployment, а не клиентов/Python | [Pipecat Cloud Scaling](https://docs.pipecat.ai/pipecat-cloud/fundamentals/scaling) | текущая документация | 2026-09-12 | высокая для документированного контракта; не независимый benchmark |
| [41] | GIL ограничивает Python bytecode threads; threading подходит для I/O-bound concurrency; CPU parallelism достигается процессами | [Python threading](https://docs.python.org/3/library/threading.html) | Python 3.14.7 docs | 2026-09-12 | высокая для runtime semantics |
| [42] | Pipecat прошёл выше 50 synthetic sessions, но tail резко ухудшался под высокой нагрузкой | [AreevAI Flowcat benchmark](https://github.com/AreevAI/flowcat/blob/main/bench/RESULTS.md) | тест 2026-05-31 | 2026-09-12 | средняя: конкурент, synthetic workload, без независимой репликации |
| [43] | Pion — pure-Go WebRTC API/primitives, а не packaged SIP/PSTN platform | [Pion WebRTC](https://github.com/pion/webrtc) | текущий репозиторий | 2026-09-12 | высокая для capability boundary |
| [44] | Официальный SIP-to-WebRTC example добавляет отдельный sipgo | [Pion SIP-to-WebRTC example](https://github.com/pion/example-webrtc-applications/tree/main/sip-to-webrtc) | текущий репозиторий | 2026-09-12 | высокая |
| [45] | Go-библиотека управляет работающим Asterisk через ARI | [CyCoreSystems/ari](https://github.com/CyCoreSystems/ari) | текущий репозиторий | 2026-09-12 | высокая; exact version compatibility требует теста |
| [46] | Asterisk ARI и External Media дают call-control/media seam для внешнего worker | [Asterisk External Media and ARI](https://docs.asterisk.org/Development/Reference-Information/Asterisk-Framework-and-API-Examples/External-Media-and-ARI/) | текущая документация | 2026-09-12 | высокая для interface; release-specific support проверить |
| [47] | Self-hosted LiveKit telephony требует отдельный SIP service, Redis и публичные SIP/RTP порты | [LiveKit SIP server](https://docs.livekit.io/transport/self-hosting/sip-server/) | текущая документация | 2026-09-12 | высокая для deployment boundary |
| [48] | DTMF, forwarding, transfers и secure trunking заявлены как telephony features | [LiveKit telephony features](https://docs.livekit.io/telephony/features/) | текущая документация | 2026-09-12 | высокая для documented capability; carrier compatibility unverified |
| [49] | Раздельные метрики TTFB, TTFA, TTFAT, thinking и aggregation | [Pipecat Metrics](https://docs.pipecat.ai/pipecat/fundamentals/metrics) | текущая документация | 2026-09-12 | высокая для measurement capability |
| [50] | EOU/STT, LLM, TTS и interruption latency metrics | [LiveKit observability data](https://docs.livekit.io/deploy/observability/data/) | текущая документация | 2026-09-12 | высокая для definitions; не performance result |
| [51] | Caller-side 550-call latency corpus и p50/p95 современных managed platforms | [OpenBenchmarks Voice Agent Latency](https://github.com/openbenchmarks-labs/voice-agent-latency) | generated 2026-08-01 | 2026-09-12 | высокая для опубликованного corpus; без независимой репликации |
| [52] | Streaming-pipeline TTFA 755 ms в опубликованной реализации; не Yandex/PSTN | [Qiu et al., arXiv v2](https://arxiv.org/html/2603.05413v2) | 2026-03-17 | 2026-09-12 | средняя: preprint, иной stack |
| [53] | Telephony media framing, pacing, buffering и flush behavior | [Asterisk WebSocket channel driver](https://docs.asterisk.org/Configuration/Channel-Drivers/WebSocket/) | текущая документация | 2026-09-12 | высокая для documented behavior |
| [54] | Novofon inbound/outbound SIP trunk prerequisites, IP-auth, routing and API | [Novofon 2.0 onboarding](https://novofon.com/instructions/novofon/onboarding-novofon/) | текущая документация | 2026-09-12 | высокая для advertised configuration; live interoperability unverified |
| [55] | Production Go/Pion 1:1 transceiver and reasons for owning media/signaling | [OpenAI Engineering: low-latency voice at scale](https://openai.com/index/delivering-low-latency-voice-ai-at-scale/) | 2026-05-04 | 2026-09-12 | высокая для OpenAI architecture; applicability to Staffy medium |
| [56] | Production Python asyncio → Go media/inference change improved tail smoothness | [OpenAI Engineering: responsive voice AI](https://openai.com/index/continuous-voice-interaction-with-gpt-live/) | 2026-08-03 | 2026-09-12 | высокая for reported outcome; absolute numbers/cause split absent |
| [57] | Production-oriented Go/Pion media daemon pattern | [Mattermost rtcd](https://github.com/mattermost/rtcd) | текущий репозиторий | 2026-09-12 | высокая для existence/architecture |
| [58] | Calls v2 removed owned RTC/RTCD integration in favor of LiveKit | [Mattermost Calls PR #1163](https://github.com/mattermost/mattermost-plugin-calls/pull/1163) | 2026-04-23 | 2026-09-12 | высокая для code change; motivation unverified |
| [59] | Typed frames, priority/order lanes и pipeline/transport boundary | [Pipecat Pipeline & Frame Processing](https://docs.pipecat.ai/pipecat/learn/pipeline) | текущая документация; дата не указана | 2026-09-12 | высокая для mechanics; Staffy contract — synthesis |
| [60] | Prompt context из фактически доставленной речи и placement after transport output | [Pipecat Context Management](https://docs.pipecat.ai/pipecat/learn/context-management) | текущая документация; дата не указана | 2026-09-12 | высокая для documented invariant |
| [61] | Tool timeouts, interruption cancellation, async/progress и grouped calls | [Pipecat Function Calling](https://docs.pipecat.ai/pipecat/learn/function-calling) | текущая документация; дата не указана | 2026-09-12 | высокая для runtime contract |
| [62] | Pipecat v1.9/v1.10 churn, latency breakdown и deprecations | [Pipecat Releases](https://github.com/pipecat-ai/pipecat/releases) | 2026-09-11—2026-09-12 | 2026-09-12 | высокая для release facts |
| [63] | Stateful realtime bridge, room transport, job dispatch и session lifecycle | [LiveKit Agents](https://docs.livekit.io/agents/) | текущая документация; дата не указана | 2026-09-12 | высокая для advertised interfaces; scale unverified |
| [64] | Несколько turn-detection strategies и разделение endpointing/interruption | [LiveKit Turns](https://docs.livekit.io/agents/logic/turns/) | текущая документация; дата не указана | 2026-09-12 | высокая для capability surface |
| [65] | Text/audio tests для messages, tools, handoffs и multi-turn outcomes | [LiveKit Testing and Evaluation](https://docs.livekit.io/agents/start/testing/) | текущая документация; дата не указана | 2026-09-12 | высокая для test surface |
| [66] | Transfer trigger и фактический SIP transfer outcome — разные failure points | [Retell Call Transfer Reliability](https://docs.retellai.com/reliability/call-performance) | текущая документация; дата не указана | 2026-09-12 | средняя: один publisher, quantitative guarantee не выводится |
| [67] | Realtime session: audio, interruptions, tools, approvals, guardrails и handoff | [OpenAI Realtime Agents Guide](https://openai.github.io/openai-agents-python/realtime/guide/) | текущая документация; дата не указана | 2026-09-12 | высокая для runtime contract |
| [68] | MCP OAuth-oriented transport authorization и host/server boundary | [MCP Authorization](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization) | spec 2025-11-25 | 2026-09-12 | высокая для normative contract |
| [69] | MCP оставляет implementation access control, validation, confirmation, timeout, sanitization и audit | [MCP Tools](https://modelcontextprotocol.io/specification/2025-11-25/server/tools) | spec 2025-11-25 | 2026-09-12 | высокая для normative/recommended controls |
| [70] | MCP elicitation consent не равен completion; sensitive credentials запрещены в form mode | [MCP Elicitation](https://modelcontextprotocol.io/specification/2025-11-25/client/elicitation) | spec 2025-11-25 | 2026-09-12 | высокая для protocol semantics |
| [71] | RAG retrieve/augment/generate и риск incomplete/irrelevant retrieval | [Microsoft Foundry RAG](https://learn.microsoft.com/en-us/azure/foundry/concepts/retrieval-augmented-generation) | 2026-05-20 | 2026-09-12 | высокая; operational source-of-truth boundary — synthesis |
| [72] | Durable retries могут выполнить operation более одного раза; idempotency key должен переживать attempts | [AWS Durable Execution Idempotency](https://docs.aws.amazon.com/durable-execution/patterns/best-practices/idempotency/) | текущая документация; дата не указана | 2026-09-12 | высокая для execution semantics |
| [73] | Immutable prompt versions, pinning, linked evals и rollback | [OpenAI Prompt Management](https://help.openai.com/en/articles/9824968) | обновлено 2026-08 | 2026-09-12 | высокая для advertised release controls |
| [74] | Production incident remediation через rollback, detection tests и continuous evals | [Anthropic Postmortem](https://www.anthropic.com/engineering/a-postmortem-of-three-recent-issues) | 2025-09-17 | 2026-09-12 | высокая для reported controls; impact — first-party report |
| [75] | Telegram webhook/getUpdates, secret header, IDs, files и Business messages | [Telegram Bot API](https://core.telegram.org/bots/api) | rolling docs | 2026-09-12 | средняя: official single publisher; account test required |
| [76] | Обычный Telegram bot не начинает диалог без действия пользователя | [Telegram Bots Introduction](https://core.telegram.org/bots) | rolling docs | 2026-09-12 | средняя: official single publisher |
| [77] | Telegram Business Connection, media/voice и `can_reply` constraints | [Telegram Bot Features](https://core.telegram.org/bots/features) | rolling docs, current 2026 features | 2026-09-12 | средняя: official single publisher |
| [78] | MAX auth, IDs, media, mini-app и documented rate limits | [MAX API](https://dev.max.ru/docs-api) | rolling docs, changes 2026-09 | 2026-09-12 | средняя; limits требуют staging test |
| [79] | MAX HTTPS webhook, secret, retry и event surface | [MAX Webhook Subscriptions](https://dev.max.ru/docs-api/methods/POST/subscriptions) | rolling docs, current 2026-09 | 2026-09-12 | средняя; failure behavior single-publisher |
| [80] | VK community-message schema, IDs, attachments/audio-message и policy errors | [VK API Schema](https://github.com/VKCOM/vk-api-schema) | API schema 5.199 | 2026-09-12 | средняя; live account/portal test required |
| [81] | Jivo Bot API webhook, IDs, `INVITE_AGENT`, timeout/retry and tier | [Jivo Bot API](https://www.jivo.ru/help/api/bot-api.html) | rolling docs; footer 2026 | 2026-09-12 | средняя; operational timings require staging |
| [82] | Jivo Chat API for custom web/mobile sources, media и operator replies | [Jivo Chat API](https://www.jivo.ru/help/api/chat-api.html) | rolling docs; footer 2026 | 2026-09-12 | средняя; delivery/read not documented current |
| [83] | Avito Messenger API for own CRM, eligible tariff and Jivo connection | [Avito Messenger](https://avito.akamaized.net/business/tools/messenger) | rolling business docs | 2026-09-12 | средняя for access path; technical webhook/media semantics unverified |
| [84] | ABCP HTML-mode infoblocks/footer permit external scripts | [ABCP Interface and Content FAQ](https://www.abcp.ru/d/docs/website/faq-interface-and-content) | rolling docs | 2026-09-12 | средняя; installation requires live site test |
| [85] | WhatsApp Business app availability does not establish Russia Platform API eligibility | [WhatsApp Business Help](https://faq.whatsapp.com/665643701880397/?cms_platform=android&locale=ru_RU) | rolling help | 2026-09-12 | medium for app; Russia Platform eligibility unverified |
| [86] | Chatwoot v4.17.1 current release and recent duplicate/channel/security fixes | [Chatwoot Releases](https://github.com/chatwoot/chatwoot/releases) | 2026-08-27 | 2026-09-12 | высокая для release facts |
| [87] | Chatwoot Website, Telegram, API Channel, inbox/contact and assignment model | [Chatwoot Channels and Inboxes](https://www.chatwoot.com/hc/user-guide/articles/1784681461-was-ist-ein-kanal-was-ist-ein-posteingang) | 2026-07-22 | 2026-09-12 | высокая for documented model; exact 4.17 parity medium |
| [88] | Chatwoot AgentBot events and `pending ↔ open` human handoff | [Chatwoot Agent Bots](https://www.chatwoot.com/hc/user-guide/articles/1788726304-) | 2026-09-08 | 2026-09-12 | высокая for current documented behavior |
| [89] | Self-hosted data control, MIT Community Edition and operator-owned maintenance | [Chatwoot Self-hosted License](https://www.chatwoot.com/hc/user-guide/articles/1750735898-purchasing-a-paid-self_hosted-chatwoot-license-a-step_by_step-guide) | 2025-07-01 | 2026-09-12 | средняя; full Russian egress perimeter unverified |
| [90] | MAX chat-ID persistence/dedup requirement after `GET /chats` removal | [MAX Chats Migration](https://dev.max.ru/docs-api/methods/GET/chats) | change from 2026-06; current docs | 2026-09-12 | высокая for provider requirement |
| [91] | Managed channel/session/routing/handoff layer for Voice, SMS, WhatsApp, RCS and Chat | [Twilio Agent Connect](https://www.twilio.com/docs/conversations/agent-connect/overview) | live docs; date not stated | 2026-09-12 | высокая for advertised channel set; Russian fit unverified |
| [92] | MCP tool annotations are hints, not enforcement against prompt injection | [MCP Tool Annotations Risk Vocabulary](https://blog.modelcontextprotocol.io/posts/2026-03-16-tool-annotations/) | 2026-03-16 | 2026-09-12 | высокая for security boundary |
| [93] | Provider-managed Agent session state, status, required actions and usage | [OpenAI Managed Agent Sessions](https://developers.openai.com/api/reference/typescript/resources/beta/subresources/agents/subresources/sessions/methods/retrieve) | live beta API; date not stated | 2026-09-12 | высокая for API shape; durability/residency unverified |
| [94] | Agents SDK sessions, HITL, MCP, guardrails and tracing | [OpenAI Agents SDK](https://openai.github.io/openai-agents-python/) | live docs; date not stated | 2026-09-12 | высокая for advertised capabilities; Russian fit unverified |

## Приложение B. Карта устаревания

Автоматический расчёт использует интервалы повторной проверки, заданные для пакета технического исследования: версии, совместимость и цены — 1 месяц; функции ИИ, развёртывание, интеграции и предложения поставщиков — 3 месяца; состояние экосистемы — 6 месяцев; нормативные положения — 12 месяцев; архитектурные шаблоны и шаблоны надёжности — 24 месяца.

| Группа утверждений | Дата повторной проверки | Состояние |
| --- | --- | --- |
| Yandex native SIP: статус после Preview | 2026-06-28 | **Уже просрочено:** заменить публичный поиск письменным ответом Yandex |
| Найденная редакция 152-ФЗ | 2026-09-01 | **Уже просрочено:** проверить актуальную редакцию с юристом до medical-пилота |
| SpeechKit five-minute streaming limit | 2026-09-18 | Проверить только при выборе component pipeline |
| Runtime versions, quotas, совместимость и цены | 2026-10-11 | Ближайший полный технический refresh |
| Pipecat deployment quota, Pion/ARI/LiveKit surfaces и Novofon contract | 2026-10-12 | Перепроверить перед фиксацией architecture spine |
| Telegram/MAX/VK/Jivo/Avito/ABCP/Chatwoot API и eligibility | 2026-10-12 | Перепроверить перед реализацией каждого adapter или inbox spike |
| Yandex/Exolve функции; Exolve callbacks; supplier APIs; LiveKit deployment/regions | 2026-12-11 | Повторить docs и договорный due diligence |
| MCP/managed-agent contracts и agent-runtime ecosystem | 2026-12-12 | Перепроверить перед фиксацией runtime/tool protocol |
| LiveKit/Pipecat ecosystem health | 2027-03-11 | Проверить release и maintainer trajectory |
| Realtime data path и WebSocket logging | 2027-09-11 | Раньше, если изменятся условия или появится sensitive pilot |
| Architecture/reliability patterns | 2028-09-11 | Пересмотреть вместе с architecture spine |

Самая ранняя автоматическая дата повторной проверки — **2026-06-28**; она уже прошла, потому что последний явный публичный статус Yandex SIP относится к Preview-анонсу. Практическое действие — получить письменный ответ сейчас. Go/Pipecat/Pion/LiveKit/Novofon slice перепроверяется **2026-10-12** либо раньше после первых 20 контролируемых звонков и первого нагрузочного теста.
