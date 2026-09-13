# UX source extraction: финальный PRD Staffy

Дата извлечения: 13 сентября 2026 года  
Статус источников: `final` для `prd.md` и `addendum.md`; `delivery-slices.md` — scope-контракт PRD; `open-items-triage.md` — карта возврата к открытым решениям.  
Источники: `prd.md`, `addendum.md`, `delivery-slices.md`, `open-items-triage.md` из каталога `prd-staffy-2026-09-12`.

Этот файл извлекает требования для UX-проектирования. Он не добавляет продуктовых решений и не заменяет исходные документы.

## 1. Пользователи и роли

### 1.1 Роли платформы — формулировки PRD

| Роль | Определение и UX-потребность |
|---|---|
| **Экономический покупатель** | Собственник, основатель или руководитель малого/среднего бизнеса, где пропущенные звонки, медленные ответы, трудный найм, нестабильное качество или стоимость покрытия часов непосредственно влияют на выручку. Должен понять capability-fit, границы автономности, стоимость, измерение результата и условия продолжения до передачи лишних доступов. |
| **Операционный заказчик** | Руководитель продаж/операций, отвечающий за процесс, людей, исключения и измеримый бизнес-результат. Нужны owner/SLA, исключения, Handoff, scoped controls, отчёты по версиям/классам и безопасный rollback. |
| **Настройщик** | Нетехнический администратор Tenant, консультант Staffy либо интегратор, который подключает готовые Domain Packs и Connectors, знания, полномочия и fallback. Должен выполнить поддерживаемый запуск без кода и обязательного участия программиста. |
| **Человек-оператор/эксперт** | Менеджер, проверяющий, диспетчер, мастер-куратор или специалист по претензиям, принимающий Human Handoff и решения вне полномочий Staffy. Должен получить decision-ready контекст, принять ownership и продолжить без повторного опроса клиента. |
| **Конечный собеседник** | Клиент Tenant, который обращается голосом или текстом и ожидает полезный результат без повторного объяснения истории. Должен исправлять факты, явно подтверждать обязательства, получать альтернативный путь и вызывать человека в любой момент. |

### 1.2 Доменные участники и различия полномочий

#### Montagio

- **Клиент услуги:** Анна, Ольга, Михаил, Валентина и другие конечные заказчики; используют звонок, чат или защищённое web-продолжение.
- **Продавец партнёрского магазина:** Елена; передаёт структурированный lead с source/provenance и consent evidence, но её сведения не считаются автоматически подтверждёнными клиентом.
- **Независимый мастер:** Алексей; принимает decision-ready предложение, подтверждает доступность и фиксирует onsite evidence. Не должен выглядеть штатным сотрудником Montagio.
- **Технический куратор/уполномоченный эксперт:** проверяет нестандартные, высокорисковые и претензионные решения; AI не заменяет его инженерный/причинный вывод.
- **Montagio Operations / Response Owner:** владеет supply, коммерческими исключениями, Recovery и подтверждаемым Handoff.
- **B2B-участники:** заявитель, контакт на объекте, согласующий бюджет и плательщик — отдельные роли; сотрудник смены не получает полномочия согласующего автоматически.

#### Магазин автозапчастей Олега

- **Покупатель детали:** Сергей, Мария, Николай, Алексей, Дмитрий, Елена, Игорь; может обращаться голосом или через Jivo и использовать защищённую ссылку для VIN/media.
- **Проверяющий менеджер:** Антон; принимает `pending_manager_verification`, видит применимость и точную версию Order, подтверждает или блокирует supplier-order продолжение.
- **Владелец/оператор Tenant:** Олег; оценивает Autonomy Classes, бизнес-результат, ручную нагрузку и выполняет точечное повышение/откат.
- **Специалист по подбору/претензиям:** принимает диагностические, противоречивые, возвратные и спорные случаи с уже собранным decision-ready контекстом.

### 1.3 Не пользователи/не обещания первой версии

- Self-service запуск неподготовленной отрасли или неизвестного класса интеграции без готового Domain Pack/Connector.
- Неограниченный автономный агент без human gates, аудита и Confirmation.
- Клиенты, которым для запуска обязательны неподтверждённые каналы, full custom telecom/media stack или гарантированный enterprise SLA до прохождения соответствующих внешних гейтов.

## 2. Каналы, модальности и поверхности

### 2.1 MVP-каналы и подключения

| Контур | Подтверждённый MVP scope | Ограничение/статус |
|---|---|---|
| **Montagio voice** | Novofon, streaming voice/audio | Novofon + Yandex — приоритетная проверяемая связка; end-to-end SLO и compliance подтверждаются до live/production. |
| **Montagio text** | Чат собственного сайта | Основной текстовый канал пилота. |
| **Олег voice** | Отдельная SIP-линия Staffy | Текущий провайдер предположительно Mango Office; фактический аккаунт, trunk/transfer/call events/codec/Caller ID/fallback требуют проверки. |
| **Олег text** | Jivo как первый и записывающий human Inbox | Bot API/media/handoff/fallback/return semantics требуют account-level проверки. |
| **Protected web continuation** | VIN, OEM, документы, статические изображения, выбор вариантов и Confirmation | Используется, если исходный канал неудобен или не поддерживает данные; переносит только разрешённый контекст, отслеживает доставку/ожидание/завершение/истечение. |
| **Business systems** | ABCP + доступная связка с кастомной 1С Олега; Owning Systems Montagio для каталога услуг, цены и supply | Staffy не заменяет эти системы; актуальные материальные факты читаются из Owning System. |

### 2.2 Каналы, которые не считать production-возможностью MVP

- Avito, MAX, VK, WhatsApp, Telegram и другие внешние каналы — `assisted`, `human_only`, `paused` либо roadmap до официального доступа, договора и проверки Capabilities.
- Exolve — резервный кандидат, не обязательная зависимость MVP.
- Непроверенный аккаунт/API-доступ не отображается как поддержанный канал.
- Один номер, чат, сессия или дополнительный поддерживаемый канал той же роли не создаёт нового ИИ-сотрудника сам по себе.

### 2.3 Поддерживаемые и исключённые модальности

**В MVP:** text, streaming voice/audio, static image, document. Каждый фрагмент хранит modality, источник, время, consent/access context и связь с извлечённым фактом.

**Не входят неявно:** video/live camera, shared screen, аватары, voice cloning и emotion inference. Для них требуются отдельная бизнес-задача, consent/privacy model и доказанный provider path.

### 2.4 Пользовательские поверхности

#### До покупки и запуск Tenant

1. **Capability-fit / pilot scope surface:** желаемый результат → покрытие Domain Pack/Connectors/Capabilities → внешние условия → человеческие обязанности → gaps → ориентировочная упаковка/usage → pilot plan → fit/assisted extension/no-fit.
2. **Guided setup:** `Tenant → ИИ-сотрудник → Domain Pack → Connectors → знания/поведение → полномочия/fallback → eval/preview → публикация`.
3. **Integration Catalog:** SIP-, chat-, inbox- и business-system Connectors с Capabilities, ограничениями, статусом проверки и внешними условиями.
4. **Knowledge/behavior configuration:** provenance, scope, freshness, priority, version, conflicts; role, Conversation Modes, tone, Allowed Claims, prohibited phrases, Confirmation/Handoff paths и примеры.
5. **Voice configuration:** разрешённый голос, темп, стиль, произношение брендов/имён/адресов/доменных сущностей и audio preview.
6. **Eval/readiness/publish:** happy, edge, failure и recovery scenarios; expected-vs-actual; sandboxed/non-mutating Action; Handoff/return; blocking gaps; approval summary; publish/rollback.
7. **Entitlement/usage:** состав запуска/подписки, каналы/Connectors, лимиты, расход/бюджет и состояния Entitlement.

#### Операционная работа

1. **Human operator surface:** поддерживаемый существующий Inbox Tenant либо минимальная Staffy surface. Обязательны assignment, acknowledgement, Response Owner, decision-ready context, внешний ответ и Recovery continuation.
2. **Unified Conversation/Case context:** подтверждённые/спорные факты с provenance, Work Items, открытые вопросы, Commitments, Action Receipts, Handoff/Recovery и история переходов каналов.
3. **Autonomy/eval/rollout control:** режим по Autonomy Class, Evidence Gate, метрики по версии/классу/outcome и scoped rollback.
4. **Scoped controls:** Tenant, Connector, канал, услуга, Policy Outcome, Autonomy Class или кампания переводятся в `accept`, `intake_only/no_promises`, `human_only` или `paused`.
5. **Usage/observability:** качество, деловые исходы, ручная нагрузка, carrier/audio/model/tool/Connector costs, budget anomalies и end-to-end correlation.

#### Конечный клиент

1. Голосовой или текстовый разговор с одной ролью и контекстом.
2. Защищённое мультимодальное продолжение для media/сущностей/вариантов/Confirmation.
3. Focused correction/readback спорного факта без повторения всего intake.
4. Human request, callback, reconnect и degraded path с честным статусом.
5. Commitment review: scope/маршрут, цена/диапазон, неизвестные факторы, payee, гарантийная роль, следующий шаг и последствия Confirmation.
6. Recovery/claim continuation с одним Case ID, владельцем, evidence, промежуточными обновлениями и правом оспорить summary/решение.

#### Доменная поверхность автозапчастей

- Сравнение оригинала/аналогов с применимостью, ценой, наличием, сроком и различиями.
- Защищённое предъявление и исправление VIN/OEM/документа.
- Клиентский Order ABCP и правдивые раздельные статусы: создан, проверен, supplier order сформирован, supplier order отправлен.
- Decision-ready manager verification: автомобиль/VIN, запрос, позиции, доказательство применимости, альтернативы, цена/срок, Confirmation, operation ID.

#### Доменная поверхность Montagio

- Service/Work Item qualification и P1–P6 outcome; P7 Safety Interrupt.
- Versioned service×market price/Allowed Claims и supply readiness.
- Decision-ready offer независимому исполнителю, последовательное атомарное назначение и final named-executor Confirmation.
- Extra-work delta/evidence/stop-work/Confirmation.
- Раздельные статусы выполнения, оплаты/документа, комиссии и претензионного окна.

### 2.5 Inbox policy

- В каждый момент Conversation имеет одного Response Owner и один записывающий Inbox.
- После Human Handoff Staffy не отправляет параллельные ответы.
- Staffy не требует заменить подходящий Inbox клиента.
- Для Tenant без подходящего Inbox Architecture выбирает ограниченную внешнюю surface либо минимальную Staffy surface до соответствующего запуска; конкретная реализация открыта до OQ-17.

## 3. Пользовательские пути — точные ID и названия

Ниже ID и названия воспроизведены дословно из заголовков PRD.

### 3.1 Платформа — запуск ИИ-сотрудника

- **UJ-P1 — Ирина проверяет соответствие Staffy до передачи доступов**
- **UJ-P2 — Ирина запускает поддерживаемого ИИ-сотрудника без программиста**

### 3.2 Montagio

- **UJ-M1 — Анна заказывает замену смесителя через Montagio**
- **UJ-M2 — Анна покупает водонагреватель в партнёрском магазине**
- **UJ-M3 — Ольга начинает на Avito и безопасно продолжает в защищённом контуре**
- **UJ-M4 — Дмитрий сообщает об аварийной ситуации в кафе**
- **UJ-M5 — Михаил покупает инженерный осмотр отопления**
- **UJ-M6 — Валентина не может прислать фото или просит человека**
- **UJ-M7 — Алексей обнаруживает дополнительную работу на объекте**
- **UJ-M8 — Анна возвращается с претензией после выполнения**

### 3.3 Автозапчасти

- **UJ-A1 — Сергей подбирает тормозные колодки по телефону**
- **UJ-A2 — Мария подбирает деталь в Jivo и продолжает голосом**
- **UJ-A3 — Николай повторяет заказ по известному OEM/артикулу**
- **UJ-A4 — Алексей описывает симптом вместо требуемой детали**
- **UJ-A5 — VIN, документ и каталог противоречат друг другу**
- **UJ-A6 — Елена просит специалиста или Staffy не может завершить операцию**
- **UJ-A7 — Антон проверяет заказ Staffy и находит ошибку**
- **UJ-A8 — Олег повышает или откатывает автономность класса**
- **UJ-A9 — Игорь обращается по статусу, возврату или рекламации**

### 3.4 Обязательное покрытие journeys

- Montagio: UJ-M1 и UJ-M2 — оба обязательны до scale; M1 предпочтительно первый, но порядок может меняться по фактической готовности.
- Walking skeleton: UJ-A1 до клиентского Order ABCP и manager approval receipt без отправки поставщику; UJ-M1 либо UJ-M2 до самого дальнего безопасного Commitment.
- Business proof автозапчастей: 20 controlled voice/text dialogues, затем 100 target real contacts против сопоставимой человеческой когорты.
- Третий независимый внешний Tenant: UJ-P1/P2 и controlled end-to-end exercise на существующих Domain Pack/Connectors без core changes, клиентского форка и программиста.

## 4. UX-требования

### 4.1 Сквозной разговор и continuity

- Одна Conversation/Case связывает разрешённые телефонные и текстовые сессии; канальная сессия не является единственным деловым идентификатором (FR-9).
- Каждый факт показывает источник, время, канал и статус подтверждения; конфликтующие значения не перезаписываются молча (FR-10).
- До channel merge, раскрытия защищённых данных или материального Action применяется risk-based identity/authority check; слабое совпадение создаёт review, не auto-merge (FR-11).
- При смене темы, канала, сессии или владельца Staffy и человек получают confirmed facts, open questions, variants и last Commitment. Несколько намерений — отдельные Work Items (FR-12).
- После disconnect/reconnect путь продолжается с последней безопасной контрольной точки и сообщает, что было/не было завершено (FR-12, FR-17).
- Клиент видит переход AI→human или human→AI, нового Response Owner, переданный context и next step (FR-13).
- AI-роль раскрывается; при outbound называются источник и цель контакта. Voicemail/third-party/unverified identity не раскрывают чувствительную причину (FR-15).

### 4.2 Conversation Modes и прекращение продажи

- Поддерживаемые режимы: `sales`, `service/status`, `safety`, `claim`, `cancellation`, `privacy/no_contact`, `human_requested`.
- `safety`, `claim`, `cancellation`, `privacy/no_contact`, `human_requested` имеют приоритет над `sales`; запрещённые commercial Actions/убеждение выключаются немедленно (FR-14).
- Возврат в `sales` требует разрешённого перехода, а не только смены тона модели.
- Stop-intent, повторный отказ и просьба человека прекращают убеждение; цена не меняется скрыто (FR-24, NFR-23).

### 4.3 Progressive intake, факты и знания

- Сначала содержательный ответ, затем минимальные вопросы, меняющие safety, Policy Outcome, цену, применимость, полномочие или Commitment (FR-18).
- Подтверждённый известный факт не спрашивается повторно; спорный фрагмент получает focused repair (FR-12, NFR-29).
- Актуальные цена/наличие/статус/права/расписание/supply читаются из Owning System перед изменяемым claim или Commitment (FR-19).
- Knowledge Source объясняет, но не заменяет Owning System. Conflicting/stale/missing source виден и может блокировать Allowed Claim (FR-20, FR-79).
- Неопределённость/противоречие блокирует затронутый Commitment и передаётся человеку с provenance и проверенными вариантами (FR-21).
- Policy Outcome показывает причину, обязательные facts/evidence и next actions; Safety Interrupt приоритетен из любого состояния (FR-22).

### 4.4 Продажа и доверие

- Staffy объясняет Tenant-value применительно к конкретному scope и использует только Allowed Claims (FR-23).
- Верхнерыночная цена Montagio объясняется контролями/результатом; ориентир 60–75-го перцентиля — проверяемая гипотеза service×market, не динамическое решение модели (FR-68).
- Запрещены предполагаемый доход/тревожность/внушаемость для цены, ложная срочность, fake discount, fear/shame, выдуманное преимущество, гарантия, отзыв или доступность, сокрытие AI-роли (FR-23–24, Addendum).
- В safety, claim и materially distressed состояниях коммерческое убеждение приостанавливается.

### 4.5 Commitment и Confirmation

- До материального действия предъявляется конкретная versioned Commitment и получается Confirmation уполномоченного участника (FR-25).
- Confirmation связана со scope, ценой/маршрутом, участниками и временем; material change требует новую версию и новый Confirmation.
- Голосовое согласие должно быть однозначным ответом на одно предъявленное материальное Commitment. Silence, clipped/overlapped speech, low confidence и «угу» требуют focused repair или другой modality.
- Цена/диапазон, неизвестные факторы, payee, гарантийная роль, next step и последствия Confirmation доступны повторно и понятны без технических терминов (NFR-22).
- Montagio требует два семантических подтверждения: понятой задачи/предварительного маршрута; затем named executor/final conditions с material delta (FR-72).

### 4.6 Media и защищённое продолжение

- Переход в protected surface объясняет цель и переносит только разрешённый context (FR-16).
- Основной канал показывает: ссылка доставлена; ожидается ответ; отправка завершена; срок истёк. Один media request связывается ровно с одной отправкой и может быть прекращён.
- Повреждённый/неясный/многозначный input получает feedback о пригодности и Work Item; извлечённый material fact предъявляется для correction.
- Невозможность фото, web, voice или определённого messenger не прекращает service, если разрешён range, diagnostic, inspection, callback или Handoff (FR-26, NFR-21).
- Media проверяется по type/size/content и проходит safe render/quarantine; ссылки Tenant/Case/role scoped, expiring, revocable, audited и retention-bound (NFR-20).

### 4.7 Business Actions и достоверность состояния

- UI предоставляет только разрешённые Tenant/role/Domain Pack/Connector/Conversation Mode Actions (FR-27).
- Mutating action показывает стадии prepare → validate → authority → Confirmation → execute-time revalidation → execute → result (FR-28).
- Непосредственно до dispatch повторно проверяются Mode, Response Owner, Scoped Control generation, authority, Policy/config versions, Confirmation и current facts.
- Retry/replay/double Confirmation не создаёт второй Staffy intent; при слабой provider-idempotency blind retry запрещён (FR-29).
- Timeout/partial completion не показывается успехом; каждый внешний effect reconciled или передаётся в manual unknown (FR-30).
- Action Receipt позволяет восстановить decision без чтения полного свободного transcript (FR-31).
- Commitment сохраняется до попытки outbound delivery; параллельные процессы не отправляют две противоречащие актуальные версии (FR-33).

### 4.8 Human Handoff и Recovery

- Human request доступен в любой момент и во всех поддерживаемых каналах; Staffy прекращает запрещённые автоматические Actions/убеждение (FR-34, NFR-23).
- Уведомление человеку не считается помощью без `acknowledged`; непринятый/брошенный ownership запускает fallback, reassignment или escalation (FR-35).
- Decision-ready context: reason, client, confirmed/disputed facts + provenance, tool results, variants, Commitments, preferred channel/time/language, accommodations и safe next action (FR-36).
- При phone transfer линия удерживает разрешённый route до confirmed answer; иначе согласуется callback с сохранённым Case (FR-37).
- Recovery продолжает original Case/Order и связывает evidence, versions, parties и payment states; один ID и Response Owner (FR-38).
- Staffy может только policy-bound: pause, request evidence, show recorded conditions, offer approved options, report status. Guilt, legal conclusion и non-standard refund/compensation — human-only (FR-39).
- Клиент может исправить/оспорить AI-summary без уничтожения original record (FR-40).
- В ожидании сообщается truthful status и обещание следующего update; неизвестный ETA называется неизвестным (FR-41).

### 4.9 Guided setup и настройка без программиста

- Setup хранится как unpublished draft с owner и save/resume; incomplete setup не выглядит active (FR-78).
- Каждый шаг показывает required inputs, verification state и понятное unblock action; можно делегировать запрос missing input.
- Missing Domain Pack/Connector/Capability создаёт assisted-extension request, не false-ready state.
- Configuration version охватывает Policy, Allowed Claims, Domain Pack, Capabilities, voice/pronunciation; доступно compare/rollback (FR-5, FR-81).
- Publication blocked без обязательного Connector, Capability, Response Owner, fallback, consent или Evidence Gate; причина и затронутый journey понятны (FR-6).
- Preview включает voice/text, changed scenarios, blocking eval, sandboxed/non-mutating Action и Handoff/return (FR-80–81).
- Integration Catalog показывает Capability, limitation, verification state и external prerequisites (FR-7–8).

### 4.10 Управляемая автономность и операционный контроль

- Autonomy Class независимо принимает `shadow`, `copilot`, `assisted`, `auto`, `exception` (FR-42).
- Promotion требует versioned Evidence Gate с sample, metrics, critical errors, approver/date; synthetic eval не разрешает material `auto` (FR-43–44).
- Критическая ошибка вызывает scoped rollback только affected class/version (FR-45).
- Operator metrics сегментируются по Tenant, employee, channel, version, Domain Pack, Autonomy Class и Policy Outcome (FR-48).
- Scoped control локализует проблему и хранит author, reason, generation и re-enable Evidence Gate (FR-32).

### 4.11 Безопасность, privacy и data UX

- Fail-closed при unknown Mode/authority/Confirmation/tool result/safety state (NFR-1).
- AI и человек имеют разные права; UI не позволяет AI решать guilt, non-standard compensation/refund, legal conclusion или unresolved applicability dispute (NFR-3).
- Production PII/audio/transcripts/media/prompts/tool payloads/logs/backups — в утверждённом российском perimeter (NFR-14).
- До production по каждому provider/Connector есть artifact data map: location, purpose, access, subprocessors, retention/deletion, backups/support traces, export (NFR-15).
- Tenant утверждает отдельные retention purposes/periods для contacts, VIN, docs, audio, transcripts, summaries, media, receipts и audit; access/correction/withdrawal/deletion имеют identity proof и completion evidence (NFR-16).
- Recording/transcription и sensitive media processing включаются только по approved legal scenario с доказуемым consent state и альтернативой, если policy требует (NFR-17).
- Доступ к data/media ограничен Tenant, role, Case/Order и purpose; raw content access требует отдельного permission/audit (NFR-18–19).

## 5. Состояния, которые UX обязан различать

| Объект/процесс | Точные состояния/различия из PRD |
|---|---|
| **Conversation Mode** | `sales`, `service/status`, `safety`, `claim`, `cancellation`, `privacy/no_contact`, `human_requested` |
| **Response ownership/Handoff** | `requested`, `assigned`, `acknowledged`, `resolved/returned`; один active Response Owner |
| **Autonomy Class** | `shadow`, `copilot`, `assisted`, `auto`, `exception` |
| **Scoped controls** | `accept`, `intake_only/no_promises`, `human_only`, `paused` |
| **Setup draft/readiness** | unpublished `draft`; `incomplete`, `blocked`, `ready`; partial setup не active |
| **Entitlement** | `draft`, `active`, `paused`, `ended` |
| **Protected continuation** | link delivered, waiting for response, submission completed, expired; ability to stop waiting |
| **Fact/evidence** | confirmed vs disputed/untrusted; source/time/channel/provenance; usable vs unclear/damaged/ambiguous/unsupported; missing angle/field |
| **Knowledge Source** | current/versioned; conflicting, stale или missing; affected Allowed Claim blocked if unresolved |
| **Commitment/delivery** | versioned current Commitment; prior version; pending delivery; accepted Confirmation; invalidated by material delta; contradictory-current prohibited |
| **Voice output** | `generated`, `sent_to_provider`, `customer-confirmed`; где доступно `playback-acknowledged/estimated` |
| **Business Action** | prepare, validate, authority/Confirmation, revalidate, execute, Receipt; confirmed success, timeout/unknown, partial, reconciliation/manual unknown/compensation |
| **Manager verification Олега** | `pending_manager_verification`; task `assigned`, `acknowledged`, `approved/blocked`, `expired` |
| **ABCP/supplier** | customer Order created; verified; supplier Order formed; sent — нельзя сливать в один success status |
| **Montagio money/close** | work completion отдельно; `fee_due`, `fee_collected`; `due`, `paid`, `partially paid`, `disputed`, `reversed/refunded`, `written-off/reconciled`; CM provisional до settlement/claim point |
| **Human operator surface** | available/verified; owner accepted; owner unavailable/lease breached; reassignment/escalation; supported external Inbox vs minimal Staffy-provided surface |
| **Connector/Capability** | verified/current; unverified; verification expired/revoked; degraded; assisted/human-only/paused; provider outage |
| **Cohort/stage** | controlled vs live; in-profile vs explicitly excluded/separate cohort; route blocked vs eligible to advance |

### 5.1 Ошибочные и recovery-состояния, которые нельзя сводить к generic error

- Identity weak match, duplicate candidate и merge review.
- Contradictory material fact или ambiguous voice Confirmation.
- Connector timeout before dispatch vs after possible remote commit.
- Duplicate/reordered webhook, idempotency-key conflict и partial completion.
- Provider outage, disconnect, reconnect, callback и fallback.
- Handoff notification sent, assigned, not acknowledged, acknowledged owner lost.
- Action status unknown/reconcile-required, а не false success.
- Missing/expired Capability, stale Knowledge Source, missing fallback или Evidence Gate.
- Unsupported language/modality/channel и объяснённый альтернативный путь.
- Safety/claim/privacy mode interrupt и invalidated queued/in-flight intent.
- Media rejected/quarantined/missing angle и retry/alternative path.
- No supply, master timeout/decline/no-show/lost eligibility и new Confirmation on replacement.
- Extra work declined/silent/non-confirmed и safe stop/human review.

## 6. Form factors и interaction constraints

### 6.1 Требуемые form factors

- **Телефонный голосовой канал:** inbound/outbound calls, interruption/barge-in, DTMF, transfer, callback, hang-up/reconnect, real 8 kHz noisy telephony.
- **Текстовый Inbox/chat:** Jivo для Олега; web chat Montagio; human continuation в одном записывающем Inbox.
- **Защищённая web-поверхность:** VIN, документы, статические изображения, варианты и Confirmation; используется с телефона или иного web-устройства.
- **Настройка/операционная web-поверхность:** guided setup, integration catalog, operator/Handoff/Recovery, controls, metrics, usage и rollback.

### 6.2 Чего PRD не фиксирует

- Не заданы конкретные breakpoint, размеры экранов, навигационная IA, компонентная библиотека или frontend stack Staffy.
- Не принято решение, всегда ли human operator surface является UI Staffy: существующий подходящий Inbox предпочитается; минимальная Staffy surface требуется только при отсутствии подходящей внешней.
- Addendum прямо оставляет transport/outbox/adapter и component/storage design Architecture workflow.

### 6.3 Accessibility и conversational interaction

- Равноправная альтернатива при невозможности media/web/voice/messenger без снижения safety/Confirmation (NFR-21).
- Human request, correction, summary dispute и stop persuasion во всех supported channels (NFR-23).
- Critical entities VIN/OEM/SKU/names/phones/addresses/dates/money получают structured readback/Confirmation (NFR-30).
- После barge-in неактуальный audible output прекращается; playback не доказывает понимание, поэтому выполняется semantic repair/readback (NFR-31).
- Long, multi-topic, corrected и interrupted utterances входят в controlled eval; mishearing уточняет только disputed fragment (NFR-29).

## 7. UX-метрики и критерии успеха

### 7.1 Обязательные stage gates

| ID | Метрика/условие |
|---|---|
| **SM-1** | В каждой отрасли не менее 20 controlled end-to-end dialogues по voice и text, включая failure/recovery. Gate Definition фиксирует минимальные подкогорты до первого результата; один cross-channel Case — одно наблюдение. |
| **SM-2** | В каждой отрасли 100 target live contacts under observation. Eligibility denominator, exclusions/crossover и human-group assignment/matching фиксируются до этапа. UJ-M2 и voice route нельзя заместить удобным большинством. |
| **SM-3** | 0 material Actions без обязательного Confirmation, duplicate Orders, dangerous recommendations и context loss after acknowledged Handoff. Любой случай блокирует affected gate. |
| **SM-4** | Третий независимый внешний Tenant публикуется и проходит controlled exercise на existing pack/connectors без core changes, client fork и programmer максимум за один рабочий день настройщика после корректных data/access. Срок — проверяемая гипотеза. |

### 7.2 Бизнес- и операционные метрики, влияющие на UX

| ID | Метрика |
|---|---|
| **SM-5** | Autoparts contribution per 100 target contacts после cancellation/returns/variable costs не хуже сопоставимых human managers. |
| **SM-6** | Montagio share of target contacts reaching a correct next Commitment vs parallel manual handling; threshold set after baseline before live. |
| **SM-7** | Montagio business-complete Orders: positive actual contribution margin, collected fee и classified claim outcome; aggregate threshold/sample set before scale. |
| **SM-8** | Conversion target contact → Confirmation → Order/other correct Policy Outcome. |
| **SM-9** | Correct autonomous outcomes и material corrections по Autonomy Class. |
| **SM-10** | Human minutes per target contact и per business-complete Order. |
| **SM-11** | Handoff acknowledged in promised time и clients forced to repeat confirmed context. |
| **SM-12** | p50/p95 end-to-end voice response, interruption stop, tool completion и Handoff on real route; target after baseline. |
| **SM-13** | Cost per correctly completed contact/call split by carrier, AI, tools, infrastructure и human labor. |
| **SM-14** | Unknown Business Actions reconciled without duplicate or false customer message. |
| **SM-15** | Third-Tenant launch time и number of steps requiring code/programmer. |
| **SM-16** | Versioned Operating Profile approved before controlled/live; out-of-profile contact excluded clearly or separate cohort. |
| **SM-17** | Montagio supply drill + Stage-0 legal/payment tabletop before live; paid reconciled micro-cohort before scale. |
| **SM-18** | Recovery: acknowledgement, time-to-named-owner, update promise compliance, containment, resolution time, reopen, contest/acceptance, recurrence. |
| **SM-19** | Honest commercial evidence level: `interest/design partner` → `paid pilot` → `continued paid use/expansion` → `repeatable external sale`. |
| **SM-20** | Separate voice/text reports: semantic task success, repair, repeated questions, critical-entity accuracy, first useful answer, perceived effort/trust и human tone rating. |

### 7.3 Conversation acceptance specifics

- Voice/text separately segmented by journey, channel/modality, Conversation Mode и critical transition.
- Measurement dictionary fixes unit, formula/scale, rater instruction и disagreement resolution.
- Numeric thresholds set in Gate Definition after controlled baseline and before live.
- Real voice set includes noisy 8 kHz, short confirmations, true interruptions, critical entities, DTMF, tool timeout, hang-up/reconnect, inbound/outbound и required transfer/callback.
- Voice p50/p95 measures time to first audible substantive response, speech stop after interruption, full tool response and Handoff, split by carrier/media/model/tool stage.

### 7.4 Counter-metrics

| ID | Нежелательный исход |
|---|---|
| **SM-C1** | Safety/authorization breach — 0. |
| **SM-C2** | Duplicate/contradictory Commitment, duplicate Order или two active executors — 0. |
| **SM-C3** | False success about price/availability/status/send/payment/Handoff — 0 material cases. |
| **SM-C4** | No-supply after promised Montagio route; threshold after live baseline. |
| **SM-C5** | Re-quote/material scope change onsite и changes without stop-work/Confirmation. |
| **SM-C6** | Completed Montagio Order without collected fee или suspected bypass. |
| **SM-C7** | Autoparts material corrections, cancellations и returns by Autonomy Class. |
| **SM-C8** | Human overload: manual minutes, unacknowledged Handoff, overdue exceptions; threshold after baseline. |
| **SM-C9** | Repeated questions, отказ на second Confirmation, early progressive-intake exit, channel-transition отказ. |
| **SM-C10** | Trust harm: pressure, deception, hidden AI/source or incomprehensible premium price. |
| **SM-C11** | Retry storm/tool repetition/usage over pilot budget without correct-result growth. |
| **SM-C12** | Metric masking: missing breakdown by version/class/outcome. |
| **SM-C13** | Handoff substitution: safe but potentially automatable contacts shifted to humans, plus human minutes by difficulty/journey. |
| **SM-C14** | Route masking: critical UX/safety failure in one journey/channel/mode/transition hidden by aggregate score. |

## 8. Открытые decision gates и точный момент решения

Ни один вопрос ниже не блокирует старт UX/Architecture/Epics. Он блокирует указанный design/controlled/live/scale/commercial gate и до решения должен оставаться fail-closed.

| OQ | Решение | Владелец | Решить до |
|---|---|---|---|
| **OQ-1** | Подтвердить Mango Office и доступные trunk, transfer, call-event, codec, Caller ID, fallback Capabilities аккаунта Олега | Олег + Technical Lead | завершения Connector design/controlled calls |
| **OQ-2** | Фактический тариф, `provider_id`, media, timeout/retry, handoff и return semantics Jivo | Олег + Integration Lead | публикации Jivo Connector в пилоте |
| **OQ-3** | Supplier Orders version/API rights/settings и отображение `pending_manager_verification` в реальный ABCP status/notification | Олег + ABCP integrator | первого изменяющего controlled test |
| **OQ-4** | Фактическая граница и reconciliation ABCP→custom 1С | Олег + 1С integrator | сквозного Order test |
| **OQ-5** | Разрешённые catalogs/suppliers и evidence applicability для originals/analogs по Autonomy Class | Олег + эксперт по подбору | публикации соответствующего Autonomy Class |
| **OQ-6** | Цена, срок, variable budget и legal-financial model коммерческого пилота Олега | Product Owner + Олег | подписания/начала платного пилота |
| **OQ-7** | Распределение controlled/live sample Montagio между UJ-M1/M2 и thresholds scale | Product Owner + Montagio Operations | утверждения Operating Profile и перехода к live/scale |
| **OQ-8** | Владельцы supply, safety/technical, commercial/legal/recovery fallback и acknowledgement time | Montagio Operations | controlled stage |
| **OQ-9** | Source, comparison scope и freshness для price hypothesis `service × market` | Product Owner + Pricing/Operations | публикации цены соответствующей услуги |
| **OQ-10** | Contract/payment/KKT-receipt/warranty/claim roles Stage 0 Montagio | Product Owner + Legal/Finance | Stage-0 tabletop и первого paid Order |
| **OQ-11** | Artifact residency/logging/retention/deletion/support access/backup для фактического Yandex Realtime/phone path | Security/Privacy + Technical Lead | production PII/recording |
| **OQ-12** | Real-route Novofon/Mango-or-actual baseline и p50/p95 voice SLO | Voice/Technical Lead | live voice gate |
| **OQ-13** | Paid-pilot level: best-effort+reroute/runbook или availability-bearing+redundancy/on-call | Product Owner + Tenant | commercial Commitment/SLA |
| **OQ-14** | Quantitative thresholds Autonomy Class, human overload, no-supply, re-quote, trust, conversation quality, recovery | Product Owner + Operations/QA | допуска следующего этапа |
| **OQ-15** | Staffy prices, included limits, billable usage и independent willingness-to-pay/retention criterion | Product Owner | repeatable commercial offer after pilots |
| **OQ-16** | Customer Profile/cross-case memory, user-visible/correctable/deletable data | Product Owner + Privacy/UX | проектирования cross-case personalization; не MVP |
| **OQ-17** | Human operator surface для Tenant без подходящего Inbox | Architecture + Product Owner | третьего Tenant без поддерживаемого Inbox |
| **OQ-18** | Sequence of Telegram/MAX/VK-Avito/WhatsApp/Exolve and fallback voice providers | Product Owner + Partnerships/Architecture | roadmap/разработки соответствующего Connector |
| **OQ-19** | State/Authority Matrix по ABCP→1С и Montagio | Architecture + владельцы Owning Systems | фиксации storage ownership и low-level design |

### 8.1 Обязательные gate moments из Delivery Slices

| Момент | UX/продуктовый результат, обязательный к этому моменту |
|---|---|
| **До controlled обоих пилотов** | Tenant isolation/roles/version/readiness/rollback; pilot channels with one Conversation/provenance/identity/one-owner/degraded/reconnect; safe intake/promise; unambiguous Confirmation; productized Handoff state; supported operator surface; Autonomy control; Operating Profile. |
| **До первого mutating controlled Action** | Authorize/revalidate/fence → intent → side effect → Receipt/reconcile; account-level Connector contract tests; для ABCP exact-version manager approval и supplier fence. |
| **До real customer path** | Human operator surface проходит Handoff/return/Recovery rehearsal (FR-83). |
| **До real PII/audio/media** | Artifact data map, consent/retention, access/audit и hostile-file tests (NFR-14–20). |
| **До controlled voice/text завершения** | Segmented conversation acceptance report; safety/reconnect/voice-route/critical outcome suite. |
| **Повторно до live** | Frozen/approved Operating Profile, Gate Definition, Coverage Matrix; conversation thresholds after controlled baseline. |
| **До live voice** | Real carrier/media/model/tool baseline и утверждённые p50/p95 SLO; OQ-11/12 closed. |
| **До live Montagio** | Supply drill, Stage-0 legal/payment tabletop, supply/price/fallback owners and policies. |
| **До платного live pilot** | Versioned Entitlement, manual contract/invoice/activation allowed, usage visibility productized; selected availability level/RTO/RPO. |
| **Live capacity gate** | Pilot Load Envelope × approved safety factor and admission/degraded policy. |
| **До scale Montagio** | Both UJ-M1/M2 proven; paid micro-cohort manually reconciled; finance/claim outcome and thresholds approved. |
| **Third-client/platform proof** | Productized guided supported launch UJ-P1/P2, knowledge/behavior/voice/media preview; existing pack/connectors; no code/programmer; one-day criterion measured. |
| **До repeatable commercial offer** | Full pilot cost, prices/limits/billable usage and required willingness-to-pay evidence level decided. |

## 9. UX-ограничители scope

- Не проектировать отдельное ядро/workflow engine на Tenant или семь отдельных Montagio workflow по P-кодам.
- Не проектировать universal enterprise ticketing, automated exception orchestrator или cohort-freeze subsystem до наблюдаемого gate.
- Не предполагать замену CRM/ERP/ABCP/1С/telephony/payment/existing Inbox.
- Не обещать self-service новой отрасли; V1 no-code/one-day criterion относится только к supported Domain Pack/Connectors и допускает assisted setup reference pilots.
- Не проектировать full custom media core, own STT/TTS, avatar/video/live camera или voice cloning как обязательный MVP.
- Не автоматизировать supplier send Олега до manager gate и отдельного Autonomy Class decision.
- Не автоматизировать Montagio B2B contract/SLA, refunds/compensations, escrow или full multi-object SLA в MVP.
- Ручная/configured операция пилота допустима только с authority, Confirmation, one Response Owner, Action Receipt, fail-closed, audit и measurement; отдельная subsystem появляется, когда текущий механизм не выдерживает следующий gate по volume, auditability, safety или response time.
