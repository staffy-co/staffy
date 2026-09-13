---
title: 'Reviewer Gate Round 2 — Technical Feasibility and Architecture Handoff'
status: complete
created: '2026-09-13'
reviewed_artifacts:
  - prd.md
  - addendum.md
reviewer_lens: 'technical feasibility, integration boundaries, reliability, multi-tenancy, voice and MVP scope'
---

# Reviewer Gate Round 2 — Technical Feasibility

## Вердикт

**Условно готов к Architecture workflow.** Выбранное техническое направление непротиворечиво: собственный универсальный runtime/action core на Go, сменные provider layers и отказ от преждевременного собственного media/STT/TTS-контура образуют разумную основу (`addendum.md:68–77`). PRD также правильно задаёт fail-closed поведение, проверяемые Capabilities, Action Receipts, reconciliation, Operating Profile и сквозные NFR (`prd.md:328–353`, `prd.md:538–590`, `prd.md:1068–1076`, `prd.md:1147–1291`).

Однако перед разбиением архитектуры на компоненты нужно закрыть три высоких разрыва: формальную границу источников истины; исполнимый контракт Connector/Capability вместо универсальной подразумеваемой «идемпотентности»; и архитектурный срез пилота внутри заявленного общего MVP. Без этого разные команды могут построить технически качественные, но несовместимые реализации либо незаметно превратить Staffy во вторую CRM/систему заказов.

Критических противоречий, требующих отмены продуктового курса, не найдено.

## Сводка серьёзности

| Severity | Количество |
|---|---:|
| Critical | 0 |
| High | 3 |
| Medium | 4 |
| Low | 0 |

## Что уже достаточно хорошо для передачи архитектору

- PRD различает Knowledge Source и Owning System и запрещает выдавать RAG/память за актуальный операционный факт (`prd.md:251–255`, `prd.md:453–469`).
- Изменяющее действие отделено от свободного диалога и имеет Confirmation, Action Receipt, uncertain outcome и reconciliation (`prd.md:507–514`, `prd.md:525–572`).
- Voice-путь не привязан к недоказанной лабораторной задержке, а требует сквозного измерения реальных Mango/Novofon-маршрутов (`prd.md:1181–1197`, `prd.md:1353–1355`).
- Capacity правильно выражена в одновременных разговорах, длительности, tool load и tail latency, а не в количестве клиентов (`prd.md:1199–1207`).
- MVP явно исключает собственный полный telecom/media/STT/TTS stack, универсальный workflow engine, все каналы одновременно и преждевременную HA (`prd.md:1088–1104`, `prd.md:1122–1132`).

## Находки

### TF-01 — High: канонические сущности Staffy и Owning Systems допускают два источника истины

**Наблюдение.** Глоссарий одновременно определяет `Case` и `Order` как канонические сущности внутри продукта и называет CRM/ERP/1С/систему заказов источниками истины (`prd.md:255–260`). Staffy должен читать актуальные изменяемые факты из Owning System (`prd.md:453–460`), но также обязан хранить состояния исполнения Order, выполнять reconciliation и сквозное закрытие (`prd.md:556–572`, `prd.md:865–872`, `prd.md:977–984`). Non-Goal при этом запрещает заменять CRM, ERP/1С и систему заказов (`prd.md:1090–1094`). Для Олега остаётся неизвестной даже фактическая граница ABCP→1С (`prd.md:1343–1346`), а Montagio уже является отдельным бизнес-продуктом с собственным жизненным циклом заявки.

**Почему это мешает реализации.** Команда Go-core может естественно поместить цену, scope, assignment, payment и claim state в Staffy как state machine, тогда как интеграционная команда будет считать те же поля авторитетными в ABCP, 1С или Montagio. При конкурентном изменении, позднем webhook или ручной правке невозможно однозначно выбрать победившее состояние. FR-21 сохраняет конфликт, но не отвечает, какая система вправе разрешить его (`prd.md:471–478`).

**Требуемое решение для Architecture.** До проектирования схемы хранения создать по каждому пилоту **State/Authority Matrix**: сущность и поле; authoritative system; внешний и внутренний IDs; допускаемые Staffy snapshot/derived/projection states; версия/ETag; направление синхронизации; источник времени; правило конфликта; terminal states; способ reconciliation. Staffy Order следует явно определить как одно из двух: либо оркестрационная проекция внешнего заказа, либо самостоятельное обязательство с формальным mapping к внешним объектам. Нельзя оставлять выбор на уровне реализации.

**Acceptance bound.** Для каждого материального поля тест с ручным изменением Owning System и опоздавшим/повторным событием должен иметь единственный ожидаемый итог, а audit обязан показать применённое правило разрешения.

### TF-02 — High: Capability недостаточно описана, а абсолютная идемпотентность не гарантируема всеми провайдерами

**Наблюдение.** Каталог показывает Capabilities и ограничения, а подключение проходит тест (`prd.md:337–353`). Но минимальный контракт Capability не перечислен. Одновременно FR-29 требует, чтобы любой retry/webhook/повтор подтверждения никогда не создавал вторую эквивалентную операцию, и предполагает устойчивый ключ (`prd.md:547–554`); FR-30 требует сверки результата после неопределённого ответа (`prd.md:556–563`). В автозапчастях это конкретизировано как внешний idempotent operation ID и lookup в ABCP (`prd.md:802–809`), хотя наличие нужных API-прав и семантики ещё открыто (`prd.md:1343–1346`). Для Jivo, SIP-провайдеров, отправки сообщений или legacy API внешняя сторона может не поддерживать idempotency key либо надёжный read-after-write lookup.

**Почему это мешает реализации.** Локальный dedupe защищает от повторного запуска Staffy, но не даёт exactly-once на удалённой системе после timeout. Слепой retry может создать дубль, а отказ от retry может потерять действие. Бинарное `Capability available/unavailable` недостаточно, потому что одна и та же операция может быть безопасна только в `assisted` режиме.

**Требуемое решение для Architecture.** Утвердить версионированный **Connector/Capability contract**, включающий как минимум:

- typed input/output и schema version;
- auth scopes и tenant binding;
- sync/async модель, timeout и error taxonomy;
- delivery guarantee: native idempotency, correlation+lookup, Staffy-only dedupe или none;
- reconcile/query/cancel/compensate возможности и окно eventual consistency;
- rate/size/concurrency limits, ordering и webhook replay semantics;
- PII/media classes, residency и retention;
- materiality/irreversibility, требуемые Confirmation/Autonomy Class и допустимый degraded outcome;
- sandbox/account-level verification и срок действия результата проверки.

Если внешняя сторона не поддерживает достаточную гарантию, формулировка архитектурного инварианта должна быть честной: **at-most-one Staffy intent + controlled remote retry + reconciliation/manual unknown**, а не недоказуемое exactly-once. Capability публикации материального действия блокируется, если её guarantee не соответствует процессу.

**Acceptance bound.** Contract tests должны воспроизводить timeout до ответа, timeout после commit, duplicate webhook, reordered webhook, retry с тем же ключом и конфликт того же ключа с другим payload.

### TF-03 — High: MVP перечисляет платформу почти целиком, но не задаёт архитектурный walking skeleton

**Наблюдение.** В Scope включены все FR-1–FR-49 и FR-78–FR-85 плюс два законченных отраслевых процесса, четыре channel/provider-интеграции, мультимодальность, Recovery, Entitlement, rollback, human surface, наблюдаемость, две выборки по каждой отрасли и третий Tenant (`prd.md:1107–1120`). Это 57 общих требований до учёта двух Domain Packs. При этом addendum справедливо требует минимальные статусы, ручные flags/queues и отсутствие отдельных подсистем (`addendum.md:64–66`), а Non-Goals исключают универсальный оркестратор и enterprise ticketing (`prd.md:1095–1101`). Требования не помечены как обязательные для `controlled`, `live`, `scale` или `third-tenant proof`.

**Почему это мешает реализации.** Архитектор не может понять, что должно быть полноценной общей платформенной подсистемой до первого звонка, а что допустимо реализовать тонким вертикальным срезом или ручной операцией. Буквальное прочтение ведёт к overengineering; выборочная реализация без явного stage contract — к формальному несоответствию PRD.

**Требуемое решение для Architecture/Product.** До оценки epics добавить **stage applicability matrix** для каждого FR/NFR: `design invariant`, `controlled automation`, `live gate`, `scale gate`, `manual pilot control`, `deferred`. Первый deployable walking skeleton должен пройти один материальный путь на каждом пилоте через общие primitives: ingest → identity/provenance → policy → confirmation → one action → receipt/reconcile → handoff/recovery → outcome. Entitlement UI, общий каталог, расширенная аналитика и third-tenant automation не должны становиться блокирующими подсистемами раньше своего гейта, если их инвариант уже сохранён в модели.

**Acceptance bound.** Architecture document содержит трассировку каждого Must-now FR к одному компоненту/внешней системе/ручному runbook и этапу; ни одно требование не подразумевает отдельную подсистему без проверяемого пилотного исхода.

### TF-04 — Medium: «фактически услышанная» часть речи не наблюдаема как строгий инвариант PSTN

**Наблюдение.** NFR-31 требует после barge-in сохранить только фактически доставленную/подтверждённую часть (`prd.md:1285–1287`), а addendum предлагает использовать паттерн `heard-output` (`addendum.md:73–75`). NFR-8 правильно требует измерять остановку речи по реальному маршруту (`prd.md:1183–1189`), но SIP/media/provider API обычно способны подтвердить лишь отправленные/проигранные аудиофреймы или media marker, а не то, что человек действительно услышал и понял.

**Риск.** Если архитектура трактует `heard-output` как точный бизнес-факт, следующий turn может считать цену, срок или условия уже сообщёнными. При jitter, буфере оператора, громкой связи, half-duplex или перебивании это недоказуемо.

**Требуемое уточнение.** Разделить `generated`, `sent_to_provider`, `playback-acknowledged/estimated` и `customer-confirmed`. Только последнее может закрывать материальный Confirmation. После прерванного материального сообщения Staffy должен выполнять короткий semantic repair/readback независимо от приблизительного playback cursor. Для каждого voice Connector Capability должна сообщать доступный уровень media acknowledgement.

**Acceptance bound.** На реальных Novofon и Mango-or-actual-provider маршрутах измеряются p50/p95 stop latency и доля корректного semantic repair; ни одна цена, критическая сущность или Confirmation не принимается только по оценке проигранного аудио.

### TF-05 — Medium: baseline-first SLO полезен для честности, но архитектуре всё равно нужен исходный load envelope

**Наблюдение.** Operating Profile фиксирует лимит одновременных Conversations/calls перед controlled/live (`prd.md:1068–1076`), числовые voice SLO устанавливаются после controlled baseline (`prd.md:1183–1189`), а рост concurrency допускается после нагрузочного прогона (`prd.md:1199–1207`). Уровень best-effort/availability-bearing также остаётся открытым до коммерческого обязательства (`prd.md:1173–1175`, `prd.md:1354–1355`).

**Риск.** Это правильно как продуктовая политика, но недостаточно как вход для Architecture: без стартового upper bound нельзя осознанно выбрать размер очередей, backpressure, timeouts, connection pools, partitioning, media worker model, admission control и стоимость отказа. «Измерим позже» может закрепить прототип, который невозможно безопасно довести до live, либо спровоцировать преждевременное горизонтальное масштабирование.

**Требуемое решение.** До low-level design задать консервативный **Pilot Load Envelope** отдельно для голоса и текста: max concurrent calls/conversations per Tenant и total; arrival burst; call/message duration; media bitrate/codec; tool fan-out и timeout budget; maximum queued handoffs; provider rate limits; storage/retention volume; cost cap; admission/degraded behavior при превышении. Это не клиентский SLA и может быть пересмотрено после baseline.

**Acceptance bound.** Controlled stage не стартует без load test как минимум на утверждённом envelope × safety factor; превышение вызывает детерминированный `intake_only`, callback/handoff или reject, а не неконтролируемое ухудшение latency.

### TF-06 — Medium: версионирование Tenant-конфигурации не полностью задаёт безопасное обновление общего runtime и adapters

**Наблюдение.** FR-4 позволяет обновлять Domain Pack независимо от ядра, FR-5 версионирует опубликованную конфигурацию и разрешённые Capabilities (`prd.md:310–326`), NFR-28 требует точечного rollback Policy/Domain Pack/Connector adapter (`prd.md:1271–1273`). Однако Action Receipt указывает «версию конфигурации» (`prd.md:565–572`), но PRD не требует pin полного исполняемого release manifest: adapter build и contract schema, model/provider policy, prompt/template compiler, media pipeline, tool runtime. Общие Domain Packs/Connectors используются несколькими Tenant, а изоляция данных сама по себе не защищает от функционального blast radius (`prd.md:283–290`).

**Риск.** Обновление общего Connector adapter или runtime может молча изменить поведение опубликованных Tenant-конфигураций, сделать старый Action Receipt невоспроизводимым или одновременно повредить оба пилота. Rollback одной конфигурации не поможет, если несовместим уже общий adapter/schema.

**Требуемое решение.** Ввести immutable **Execution/Release Manifest** для каждого turn/action: tenant config, Domain Pack, policy/eval set, connector contract+adapter build, model route/policy, media route и runtime build. Нужны backward/forward compatibility rules, tenant-scoped canary, staged rollout, dual-read/migration policy и запрет удаления adapter/schema до закрытия reconciliation window.

**Acceptance bound.** Можно воспроизвести решение по Receipt, откатить один Tenant/Autonomy Class без отката остальных и обработать старое webhook/event новой версией без неверной интерпретации.

### TF-07 — Medium: граница переиспользования данных между Tenant не сформулирована

**Наблюдение.** Tenant-данные изолируются (`prd.md:283–290`), а диагностика/eval/replay по умолчанию минимизируются или обезличиваются (`prd.md:1231–1233`). Одновременно новая фактическая ошибка должна становиться регрессионным сценарием общего Domain Pack/Autonomy Class (`prd.md:681–688`), а Domain Packs и Connectors — переиспользуемые активы. Не сказано, какие production-артефакты одного Tenant разрешено превратить в общий сценарий, пример диалога, словарь или улучшение и требуется ли отдельное основание/согласие.

**Риск.** Архитектура может либо случайно перенести клиентские данные/коммерческие знания в общий asset, либо полностью запретить безопасное обобщение и потерять механизм обучения платформы. Обезличивание не всегда устраняет коммерческую тайну и повторную идентификацию по редким VIN, адресам, B2B-объектам или аудио.

**Требуемое решение.** Зафиксировать **cross-tenant learning boundary**: исходные Conversations/media/receipts tenant-private; общий regression asset создаётся только как разрешённая минимизированная/синтетическая производная с provenance, reviewer и purpose; запретить cross-tenant retrieval; отдельно управлять opt-in/contract basis для использования реальных фрагментов. NFR-15 data map (`prd.md:1211–1217`) должен включать не только provider processing, но и такое внутреннее продвижение артефакта.

**Acceptance bound.** Тесты доступа подтверждают отсутствие cross-tenant retrieval; для каждого общего сценария из production существует audit его происхождения, редактирования и разрешённого основания, без исходных идентификаторов и медиа по умолчанию.

## Архитектурная рамка, которую можно принять уже сейчас

Это не предписание реализации, а минимальная граница, следующая из PRD:

1. **Go owned core** отвечает за tenant-scoped policy enforcement, conversation/case correlation, commitments/confirmations, action intents/receipts, ownership/handoff и audit — но не становится владельцем внешних бизнес-фактов без явной записи в State/Authority Matrix.
2. **Provider/media adapters** переводят конкретные события в версионированный neutral envelope и честно объявляют уровень transport/media acknowledgement; они не содержат Domain Pack-политику.
3. **Business connectors** реализуют только объявленные Capability contracts; оркестратор выбирает retry/reconcile/manual unknown по их гарантиям, а не предполагает exactly-once.
4. **Domain Packs** задают typed policies/evals и обязательные evidence, но не получают прямой неограниченный доступ к внешним системам.
5. **Pilot control plane** может быть минимальным и ручным, если обеспечивает tenant scope, acknowledgement, immutable audit и проверяемый rollback; отдельные универсальные subsystems до доказанной потребности не требуются.

## Решения, обязательные до завершения Architecture workflow

1. State/Authority Matrix отдельно для ABCP→1С и Montagio.
2. Версионированная схема Connector/Capability contract и классификация гарантий каждой пилотной операции.
3. Stage applicability matrix / walking skeleton для Must-now MVP.
4. Pilot Load Envelope и provider-specific voice acceptance profile.
5. Execution/Release Manifest и cross-tenant learning policy.

## Gate recommendation

**Передачу в Architecture начать можно**, поскольку неизвестные провайдерские способности корректно fail-closed и перечислены как открытые вопросы. **Фиксировать component diagram, storage ownership и epic estimates до решений TF-01–TF-03 нельзя.** TF-04–TF-07 должны стать явными ADR/acceptance sections Architecture-документа и быть закрыты до соответствующего controlled/live gate.
