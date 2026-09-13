# Источник UX: Montagio Slice C → Staffy

Дата извлечения: 13 сентября 2026 года  
Источники: `SLICE_C_UX_SPEC.md`, `SLICE_C_WIREFLOWS.md`  
Назначение: вход для UX-проектирования Staffy; не замена PRD и не решение архитектуры.

## 0. Уровень подтверждения

- В `SLICE_C_UX_SPEC.md` решения зафиксированы как обязательная UX-спецификация Slice C от 4 сентября 2026 года.
- `SLICE_C_WIREFLOWS.md` имеет статус `specified` и описывает шесть потоков для согласования до разработки; внутри документа acceptance review всё ещё сформулирован как условие утверждения.
- Поэтому ниже «подтверждённый паттерн» означает **зафиксированное продуктовое/UX-решение Montagio, пригодное для проверки на переносимость в Staffy**, а не автоматически доказанную рынком универсальность. Указанные тесты удобства и browser gates являются критериями приёмки, а не результатами таких тестов.
- PRD Staffy уже подтверждает большинство сквозных инвариантов: одно ядро, Domain Packs/Connectors, единая Conversation/Case, Response Owner, progressive intake, защищённое мультимодальное продолжение, versioned Commitments, Human Handoff, Recovery и управляемая автономность. Специфика Montagio должна входить в Domain Pack и Operating Profile, а не становиться общей моделью любого Tenant.

## 1. Уже подтверждённые универсализируемые UX-паттерны

### U-01. Один деловой объект и одна временная шкала, разные ролевые представления

**Паттерн.** Все участники работают с одним `order_id`, единым набором фактов и timeline; клиент, исполнитель и оператор видят только своё представление и доступные действия. Канал не является источником истины.

**Перенос в Staffy.** Обобщить `order_id` до `Case/Order + Work Items`, сохраняя Conversation, provenance, Commitments, Actions и Audit в одном контексте. Не создавать отдельные заказы при смене телефона, чата или защищённой web-поверхности.

**Ограничение.** Не всякий Tenant имеет Order или исполнителя; общий UI должен опираться на Case/Work Item, а доменный Order показывать через Domain Pack.

### U-02. Role-specific surfaces вместо отражения базы данных

**Паттерн.** Ponytail-принцип: человек видит один следующий необходимый шаг. Для клиента, исполнителя и оператора предусмотрены самостоятельные поверхности; технический back-office не является ежедневным интерфейсом.

**Перенос в Staffy.** Нужны как минимум: клиентское защищённое продолжение, рабочая поверхность Response Owner/оператора и конфигурационная поверхность Tenant. Исполнительская поверхность подключается как доменная роль, когда процесс включает внешнего исполнителя.

**Ограничение.** Копировать меню Founder Operations UI во все Tenant нельзя: состав ролей и разделов задаётся Operating Profile/Domain Pack.

### U-03. Decision inbox, а не dashboard ради dashboard

**Паттерн.** Стартовая рабочая поверхность — очередь решений, отсортированная по риску, нарушению SLA и срочности действия. Карточка без открытия отвечает: кто/где, какая задача, стадия, почему она в очереди, сколько времени осталось, какое действие рекомендовано.

**Перенос в Staffy.** Общий Inbox должен группировать Conversation/Case по причине вмешательства и предлагать одно next-best action. Минимальные универсальные группы: safety/claim, SLA breach, human decision now, waiting, scheduled/today, follow-up, connector/channel failure.

**Ограничение.** «Выручка» — цель Montagio; для других ролей очередь может оптимизировать сервис, удержание или операционную надёжность. Приоритет safety/privacy/claim остаётся выше продажи.

### U-04. Progressive intake с ранней полезностью

**Паттерн.** Сначала ответ по существу и короткая safety-проверка, затем только 1–2 вопроса, меняющих маршрут, цену, применимость или следующий шаг. Завершённый шаг сохраняется; известное не спрашивается повторно; клиент может исправить резюме.

**Перенос в Staffy.** Вопросы и критерий достаточности задаются Domain Pack, но механика должна быть общей: extracted facts + provenance + confidence/contradictions + missing facts + next question + save/resume.

**Ограничение.** Confidence модели не принимает материальное решение; он только выбирает уточнение или Human Handoff в пределах policy.

### U-05. Мультимодальное продолжение без тупика

**Паттерн.** Голос/текст, фото и защищённая web-ссылка образуют один путь. Upload поддерживает пример требуемого материала, preview, удаление, прогресс, retry отдельного файла и сохранение черновика. Есть маршрут без фото и текстовая альтернатива голосу/изображению.

**Перенос в Staffy.** Защищённая поверхность используется для чувствительных данных, media, выбора и Confirmation, если исходный канал неудобен. При переходе сохраняется контекст, объясняется цель, проверяется идентичность, а клиент продолжает без повторного intake.

**Ограничение.** Не любой мессенджер разрешает безопасно переносить фото, адреса или согласия; Connector capability и data policy должны решать это до показа действия.

### U-06. Чёткое разделение AI, правил и человека

**Паттерн.** AI извлекает факты, резюмирует, предлагает следующий вопрос или черновик. Детерминированные правила проверяют safety, доступ, цену, состояние, consent и переходы. Человек принимает исключительные материальные решения. AI-черновик визуально не выглядит отправленным сообщением.

**Перенос в Staffy.** На каждом экране/шаге должны быть видны actor, status и authority: `AI draft`, `policy result`, `human required`, `sent/confirmed`. Нельзя заменять это общим «AI score».

**Ограничение.** Граница полномочий должна зависеть от Autonomy Class и опубликованной версии Operating Profile, а не быть навсегда одинаковой для всех Tenant.

### U-07. Четыре ответа доверия перед действием

**Паттерн.** Любая материальная поверхность без поиска по документам отвечает: кто действует; что заказано/будет сделано; сколько и кому платить; что произойдёт после кнопки.

**Перенос в Staffy.** Использовать как обязательный review checklist для Confirmation, Action preview, Handoff и Recovery. Для некоммерческого процесса «сколько/кому» заменяется на применимый материальный эффект/получателя данных, но не скрывается, если есть деньги.

### U-08. Versioned commitment и delta-confirmation

**Паттерн.** Оффер, scope, цена, исполнитель, получатель оплаты и гарантийная роль фиксируются версией. Существенное изменение создаёт новую версию; старое подтверждение не переносится автоматически. На повторном подтверждении показывается material delta.

**Перенос в Staffy.** Общий компонент Commitment Review должен показывать текущую версию, подтверждающего, изменившиеся условия, действие и Action Receipt.

**Ограничение.** Конкретный состав обязательства задаётся Domain Pack; «двойное подтверждение» Montagio не должно превращаться в два клика во всех процессах.

### U-09. Значимое действие подтверждает сервер

**Паттерн.** Сообщение, оффер, назначение, изменение денег/scope, претензия и replay считаются выполненными только после канонического ответа. Double click, retry, stale state и позднее событие не создают дубль; 409 показывает новое состояние без тихого перезаписывания.

**Перенос в Staffy.** Общие Action UI и timeline должны различать draft/pending/accepted/failed/reconcile-required и позволять безопасный retry с idempotency.

### U-10. Recovery проектируется вместе с happy path

**Паттерн.** Для каждого рабочего экрана нужны `empty`, `loading`, `success`, `partial`, `error`, `stale`, `permission denied`, `provider outage`. Сбой провайдера не теряет входящие данные или outbound; видны retry/DLQ/manual fallback. Low confidence показывает конфликт и недостающие факты, а не generic error.

**Перенос в Staffy.** Каждый Connector action обязан иметь человекочитаемый degraded outcome, owner и следующий шаг. Timeline сохраняет попытку, ошибку и восстановление.

### U-11. Progressive access и подтверждаемая передача человеку

**Паттерн.** Полный адрес, контакты и media раскрываются роли только в нужный момент; просмотр логируется. Human Handoff назначает owner, переносит контекст и сообщает клиенту SLA/следующий шаг. После handoff AI не должен отвечать параллельно.

**Перенос в Staffy.** Использовать единый компонент Handoff acknowledgement и policy-driven disclosure. Защищённая ссылка истекает и перевыпускается после проверки.

### U-12. Доступность и слабая сеть — gates, а не улучшение «потом»

**Паттерн.** Keyboard-only, visible focus, semantic structure, связанные ошибки, live regions, focus restoration, 44 px target, 200% zoom, reduced motion, текстовые альтернативы и работа на старых Android/слабой сети обязательны. Значимый путь остаётся понятным при partial/offline и сохраняет draft.

**Перенос в Staffy.** Эти критерии применимы ко всем web-поверхностям и должны проверяться на состояниях normal/error/empty/loading/high-risk, а не только на happy path.

### U-13. Объяснимое ранжирование и отсутствие скрытого «AI score»

**Паттерн.** Рекомендация показывает причины и ограничения. В Montagio порядок кандидатов объясняется допуском, зоной, доступностью, надёжностью и fairness; override требует причины.

**Перенос в Staffy.** Для любой AI/policy-рекомендации показывать решающие факторы, блокеры и источник; ручное исключение аудируется. Универсален принцип, но не набор факторов.

### U-14. Минимальная библиотека повторяемых паттернов без отдельного «проекта дизайн-системы»

**Паттерн.** Повторяемые controls/states собираются из небольшого набора: form/error summary, statuses/risk/SLA, timeline/message, drawer/action bar, uploader, money/quote, channel badge, AI confidence/human required. Labels/colors/next action идут из единого presentation registry; цвет не несёт смысл один.

**Перенос в Staffy.** Нужен один конфигурируемый UI kit ядра, расширяемый Domain Pack-компонентами. Это предотвращает отдельный UX-код на клиента без создания тяжёлой дизайн-системы до продуктовой необходимости.

## 2. Специфичное только Montagio

Эти элементы полезны как эталон доменной глубины, но не должны попадать в универсальное ядро в исходном виде.

| Элемент Montagio | Почему доменный | Как хранить в Staffy |
|---|---|---|
| `P1 fixed / P2 range / P3 diagnostic visit / P4 engineering inspection / P5 curator / P6 referral` и safety interrupt | Маршруты продажи инженерных/бытовых услуг | Policy outcomes Domain Pack Montagio; ядро знает только тип, authority, evidence и следующий Commitment |
| Проверки течи, газа, электричества, теплоносителя, общедомовых сетей | Доменная safety-taxonomy | Versioned safety policy Montagio; универсален только interrupt/mode |
| Услуга × город/зона × окно × риск и supply исполнителей | Маркетплейс услуг | Domain Pack + Owning Systems; ядро показывает readiness/degraded state |
| Master offer, payout, TTL, sequential assignment, accept lock | Независимые исполнители Montagio | Доменная роль/Action contracts; общими остаются атомарность, expiry и audit |
| Девятишаговый master stepper, инструменты/допуски, before/after/QA evidence | Полевое выполнение услуги | Настраиваемый domain workflow; не фиксированный workflow ядра |
| Двойное подтверждение исполнителя и финальных условий после принятия мастером | Конкретная модель обязательств Montagio | Commitment policy Montagio; общий механизм version/delta/confirmation |
| Extra work: причина, фото, новая цена/время, stop-work и подтверждение основателя+клиента | Изменение работ на объекте | Domain Work Item и policy; универсален material-change gate |
| Fixed/range/measurement, КАД, материалы, payout, reserve, contribution margin | Экономика услуг Montagio | Price Rule Editor Domain Pack/market override; ядро даёт version/publish/preview/audit |
| Партнёрский магазин: consent evidence, SKU/модель, покупка/доставка, атрибуция | Канал привлечения и договорённости Montagio | Versioned partner lead contract Connector/Domain Pack |
| Гарантийная развилка `warranty / new paid work / reasoned rejection / referral` | Сервисная/товарная ответственность Montagio | Recovery policy Montagio; общий слой хранит case, evidence, owner и appeal |
| Раздельные роли заявителя, контакта на объекте, согласующего бюджет и плательщика | B2B facilities subroute | Role/authority model Montagio, потенциально переиспользуемый будущими B2B Domain Packs |
| Founder Operations navigation и KPI по contribution margin | Текущая операционная модель одного бизнеса | Tenant-configured operator surface; не глобальная IA Staffy |
| Защищённая master-ссылка как основная поверхность, Telegram/MAX только notification/deep link | Выбранный канал исполнителей Montagio | Connector/Operating Profile choice, не универсальное ограничение каналов |
| Django templates + HTMX + Alpine, PWA budgets | Архитектура приложения Montagio | Не переносить как UX-требование Staffy; сохранить наблюдаемые UX/NFR outcomes |

## 3. Конфликты и неясности относительно PRD Staffy

### C-01. Статус wireflows неоднозначен

`SLICE_C_UX_SPEC.md` называет их обязательными до разработки, а `SLICE_C_WIREFLOWS.md` — `specified` и оставляет acceptance review как будущий шаг. При этом канонический контекст Montagio сообщает об одобрении владельцем. Для UX Staffy их следует считать сильным исходным шаблоном, но не автоматически подтверждёнными пользовательским тестом.

**Нужно решить:** какое evidence прикладывается к каждому перенесённому паттерну: owner approval, local verification, production verification или usability proof.

### C-02. Объект `order_id` слишком узок для общего ядра

Montagio формулирует почти всё вокруг одного `order_id`. PRD Staffy использует Conversation, Case, Order и несколько Work Items.

**Разрешение:** не переносить Order как корень платформенной IA. Корнем операторского UX сделать Case/Conversation; Order и исполнения — связанные доменные объекты.

### C-03. Три поверхности Montagio не равны полной IA Staffy

В Montagio описаны клиент, мастер и Founder Operations. Staffy дополнительно требует нетехнического настройщика, каталог Connectors, знания/поведение, полномочия/fallback, eval/preview, публикацию, rollout и entitlement.

**Пробел:** Montagio-источники не проектируют UJ-P1/UJ-P2 и не могут служить полной картой UX Staffy.

### C-04. Жёсткая граница AI в Montagio уже, чем управляемая автономность Staffy

Wireflows утверждают, что AI не назначает мастера без подтверждения и любое изменение денег требует человека. PRD Staffy задаёт Autonomy Class и допускает повышение автономности доказанного класса; при этом безопасность, вина и нестандартная компенсация остаются человеческими.

**Разрешение:** для пилота Montagio оставить ограничительные границы wireflows как Operating Profile. В ядре реализовать versioned authority/policy, а не захардкодить вечный human approval для любого Tenant.

### C-05. «AI draft всегда только черновик» конфликтует с автономными ответами

В Founder Inbox AI создаёт черновик, который проверяет человек. Однако Staffy должен уметь автономно вести разрешённые диалоги.

**Разрешение:** визуально различать `AI autonomous sent`, `AI proposed draft`, `policy-generated`, `human sent` и `delivery pending/failed`. Правило «draft нельзя спутать с sent» универсально; обязательный review каждого ответа — нет.

### C-06. Канальный охват документа шире подтверждённого пилота

Wireflow Inbox принимает Web/MAX/Avito/Telegram/phone, но UX spec прямо говорит, что внешние Avito, MAX и телефонные adapters подключаются после официальных доступов. PRD Staffy также оставляет неподтверждённый канал assisted/human-only/paused и начинает Montagio с Novofon + чат сайта.

**Разрешение:** показывать канал в UI только вместе со статусом Capability (`verified`, `assisted`, `paused`, `unavailable`); не рисовать все каналы как уже подключённые.

### C-07. Revenue Inbox не определено: новый UI Staffy или интеграция в существующий inbox

Montagio требует собственный Founder Revenue Inbox. Staffy PRD требует один записывающий Inbox и для Олега закрепляет Jivo как human Inbox. Для разных Tenant это может быть Staffy UI или существующая внешняя система.

**Нужно решить:** общий контракт Inbox/Response Owner и минимальная Staffy operator surface; конкретная записывающая поверхность выбирается Connector/Operating Profile.

### C-08. Технология Montagio не должна стать неявной технологией Staffy

Server-rendered Django/HTMX/Alpine и локальная PWA — решения Slice C Montagio. PRD Staffy не требует этого frontend-стека.

**Разрешение:** переносить UX outcomes (быстрый server-confirmed interaction, no-store для ПДн, offline/retry, mobile/accessibility), но выбрать стек Staffy отдельно в архитектуре.

### C-09. Confirmation-модель требует примирения формулировок

Montagio wireflow подтверждает offer, затем после принятия мастером раскрывает исполнителя и получает ещё одно подтверждение. PRD Staffy уже требует два разных семантических обязательства и final material delta.

**Неясность:** что именно входит в первое подтверждение, когда оно превращается в коммерческий Commitment, что происходит при смене мастера и какой маршрут допустим до финального подтверждения.

### C-10. Consent-модель источников не полностью описана как conversation UX

Спецификация перечисляет отдельные согласия, запрет precheck и минимальное раскрытие, а PRD добавляет `privacy/no_contact`, проверку основания исходящего контакта и identity linking.

**Пробел:** нужны тексты/состояния для первого партнёрского контакта, спорного consent, записи/транскрибации голоса, отказа от media и прекращения контакта во всех каналах.

### C-11. Состав статусов Order Workspace может быть доменно перегружен

Montagio постоянно показывает статусы разговора, заказа, оплаты и документов, CM, исполнителя и SLA. Это правильно для Montagio, но избыточно или неприменимо для части Staffy Tenant.

**Разрешение:** общий shell показывает Conversation/owner/SLA/risk/next action; доменные status tracks и financial blocks подключаются schema/presentation registry Domain Pack.

### C-12. North Star Montagio нельзя подменять North Star Staffy

Минуты основателя на прибыльный завершённый заказ без обоснованной рекламации — сильная метрика Montagio. Staffy измеряет также time-to-value настройки, корректность делового результата, handoff, омниканальность, себестоимость и переносимость на второй домен.

**Разрешение:** сохранить domain metric в пилотном scorecard Montagio, но не использовать её как единственную метрику платформенного UX.

## 4. Экраны и потоки, которые нельзя потерять

### 4.1 Платформенное ядро Staffy

1. **Unified Decision Inbox / Queue.** Карточки с risk, SLA, reason-in-queue, owner, channel/capability state и one next action; фильтры для missing data, price/commitment, handoff, claim и provider failure.
2. **Case/Conversation Workspace.** Факты и media; status tracks и next action; единая timeline сообщений, consent, versions, Actions, Handoff, failures и audit. На мобильном: risk/next action → commitment/domain task → facts → timeline.
3. **Protected Customer Continuation.** Без обязательного аккаунта там, где policy допускает: resume, targeted questions, media upload, summary correction, option/commitment review, status, human request, claim.
4. **Human Handoff/Acknowledgement flow.** Причина, контекст, conflicts, owner/backup, SLA, принятие человеком, прекращение параллельных AI-ответов, status update клиенту.
5. **Commitment Review and Version Diff.** Кто/что/цена-payee/что дальше, версия, material delta, explicit accept/decline/correct, receipt.
6. **Connector Failure and Recovery.** Pending outbound, retry/DLQ/reconcile/manual contact, provider status и защита от дубля.
7. **Guided Tenant Setup и Readiness.** Domain Pack, Connectors/capabilities, знания, tone/voice, permissions/fallback, eval/preview, publish/rollback — отсутствует в Montagio UX, но обязателен по PRD Staffy.

### 4.2 Domain Pack Montagio

1. **WF-C01 Client Intake.** Проблема текстом/голосом → consent/contact → safety → progressive questions → photo/alternative → correction-ready summary → P1–P6/handoff.
2. **WF-C02 Founder Revenue Inbox.** Normalized event → dedupe/order/consent → AI summary/draft → priority → send/request/offer/handoff/pause → server receipt/recovery.
3. **WF-C03 Order Workspace.** Qualification → price route → immutable offer → client confirmation → assignment → named master confirmation → execution → extra work → acceptance/docs/financial close.
4. **WF-C04 Service & Price Rule Editor.** Draft version → customer offer → eligibility → economics → routing → preview/diff → deterministic checks → review → publish/effective date/rollback.
5. **WF-C05 Master Offer and Job Stepper.** Expiring protected offer → accept/decline/lock → progressive disclosure → onsite validation/evidence → extra work gate → completion/retry.
6. **WF-C06 Warranty/Claim Intake.** Link/order verification → safety → evidence → human diagnosis → warranty/new paid work/rejection/referral → appeal → resolution/follow-up.
7. **Master Registry & Assignment Drawer.** Onboarding, skills/permissions, zones, availability, reliability/evidence, missing documents; explainable candidate order and manual override reason.
8. **Claim/Recovery Workspace.** Original commitment, versions, evidence, extra work, acceptance, timeline, authorized decision and appeal.
9. **Market Readiness/Publishing UI.** Service × market/zone supply readiness, stale review, shadow/draft/published, affected pages, owner task.

### 4.3 Обязательные состояния каждого ключевого экрана

`empty`, `loading`, `success`, `partial`, `error`, `stale`, `permission denied`, `provider outage`; для media дополнительно `local draft`, `uploading`, `retryable failure`, `uploaded`, `rejected/needs another angle`; для действий — `draft`, `pending server`, `confirmed`, `failed`, `reconcile required`, `expired`.

## 5. Decision gates с точным моментом решения

### 5.1 Runtime-гейты пользователя и системы

| Gate | Точный момент решения | Решает | Что проверяется/показывается | Если не пройден |
|---|---|---|---|---|
| Consent/contact gate | При отправке первого шага intake, до записи/обработки по соответствующей цели | Клиент + policy | обязательное consent, контакт, цель; отдельные consent не prechecked | конкретная ошибка, draft сохранён, альтернативный разрешённый путь |
| Safety interrupt | Сразу после первого содержательного описания и повторно при новом risk signal, до sales/price Commitment | Policy; человек принимает исключения | утверждённые признаки риска, безопасный текст, owner/fallback | sales Actions отключены; safety instruction + acknowledged handoff/manual fallback |
| Identity/linking gate | До объединения каналов, раскрытия ПДн/media, полного адреса или материального Action | Identity policy / человек при слабом совпадении | разрешённые идентификаторы, полномочия, риск, provenance | step-up, отдельный Case или review task; не auto-merge |
| Intake sufficiency gate | После каждого короткого блока и до запроса media/выбора Policy Outcome | Policy, AI предлагает | confirmed facts, contradictions, missing route-changing facts | следующий точечный вопрос или Human Handoff без повтора известного |
| Media gate | Когда для допуска не хватает визуального evidence, до fixed/совместимость/commitment | Клиент + deterministic validation | цель/ракурс, тип/качество, привязка к Work Item, безопасность | retry одного файла либо P2/P3/P5/человек; остальной draft не теряется |
| Price/route gate | После correction-ready summary и до сообщения цены/окна как обещания | Versioned policy; человек для исключения | Owning System freshness, eligibility, risk, scope, supply, economics | range/inspection/handoff/referral/intake-only; unknown не становится fixed |
| Supply readiness gate | Непосредственно до обещания доступности/окна/подбора и повторно перед assignment | Owning System/policy | service × zone × window × risk, primary и fallback | другое окно, waiting, human owner или честный no-fit; без выдуманного ETA |
| First customer confirmation | После объяснения понятой задачи и предварительного маршрута, до начала подбора/исполнительского Commitment | Клиент | summary, assumptions, route, scope/price basis, correction | исправление создаёт новую версию; молчание не consent |
| Master offer gate | До раскрытия полного адреса/контакта и до reservation assignment | Мастер + atomic rules | scope, evidence, exclusions, tools/permissions, zone/window, payout, TTL | decline/timeout → release lock и следующий кандидат; late accept blocked |
| Final customer confirmation | После принятия конкретным мастером и до выезда/исполнения | Клиент | named executor, final price/range, payee, warranty role и material delta | pause/reassign/recover; замена мастера инвалидирует Commitment |
| Onsite condition gate | Сразу после прибытия и до начала работы | Мастер + policy | совпадение исходного состояния, risk, required before evidence | stop, evidence, escalation; отдельный emergency protocol |
| Extra-work gate | При обнаружении работы вне scope, до её начала | Мастер сообщает; оператор проверяет; клиент подтверждает | причина, evidence, новый scope, +цена, +время, новый итог | исходный scope либо безопасная остановка/handoff; никакого silent consent |
| Completion gate | Перед переводом выполнения в completed/closed | Мастер + клиент/оператор/policy | before/after/QA evidence, acceptance, документ/чек по policy | draft/retry/task; upload failure не выглядит успехом |
| Financial/business close gate | После work completion, когда доступны payment/document/fee/claim states | Rules + оператор при исключении | plan/fact, payment, fee collected, documents, claim window | provisional/task/reconcile; не считать доход и не silent close |
| Claim outcome gate | После safety и достаточного evidence/диагностики, до обещания решения | Уполномоченный человек | исходный scope/version, evidence, причинная связь, authority | targeted evidence request, inspection, escalation; AI не определяет вину |
| Appeal/review gate | При оспаривании отказа/AI-summary, до окончательного закрытия | Человек другого/разрешённого уровня | исходная запись и исправление/оспаривание отдельно | новое versioned решение; первое не перезаписывается |
| Provider failure gate | При timeout/failure до показа успеха и при каждом retry/replay | System + operator | server receipt, idempotency, delivery/reconcile state | pending/retry/DLQ/manual fallback; клиенту честный status |

### 5.2 UX/product-гейты, которые надо закрыть в Staffy

| Решение | Точный момент, не позже | Выход |
|---|---|---|
| Корневой объект IA: Case/Conversation vs Order | До первого wireflow общего Operator Workspace | Canonical object map и правила показа связанных Work Items/Orders |
| Минимальная общая Staffy operator surface vs внешний Inbox Tenant | До wireframes Inbox/Handoff и до Connector contracts Jivo/других inbox | Inbox capability contract, Response Owner states, источник записи timeline |
| Какие части Montagio UI общие, а какие Domain Pack slots | До component inventory/presentation registry | Матрица `core component / domain schema / tenant configuration / custom forbidden` |
| Отображение authority/autonomy | До прототипирования composer, Commitment и action controls | Состояния AI draft/autonomous/policy/human; block/approval/rollback rules |
| Семантика двух подтверждений Montagio | До первого end-to-end прототипа UJ-M1 | Состав первого Commitment, final delta, invalidation при reassignment/cancel/no-show |
| Protected continuation и identity | До прототипа cross-channel UJ-M1/UJ-M2/UJ-M3 и до передачи реальных media/ПДн | Link/expiry/step-up/merge/correction UX и минимизация данных |
| Реально доступные каналы/Capabilities | До показа интеграции как `ready` и до controlled traffic | Capability badge states и разрешённые degraded routes для Novofon, сайта и будущих каналов |
| Consent/disclosure copy по источнику | До первого outbound из партнёрского lead и до включения recording/transcription | Approved first-contact, recording, privacy/no_contact и no-photo alternatives |
| Human coverage и acknowledgement SLA | До controlled stage | Owner/backup, accept timeout, customer update, manual fallback и escalation |
| Price/supply readiness UI и Owning System | До публикации первой услуги/рынка и до любого price/availability promise | Source/freshness/version/readiness/override schema и hard stops |
| Accessibility/weak-network acceptance | До usability study и повторно до live gate | Test matrix 390/768/1440, keyboard/focus/zoom, retry/offline, 5 clients/5 masters/10 owner cases |
| Productization общего паттерна | После доказанного второго использования, до включения третьего Tenant | Решение: оставить Domain Pack, поднять в core или оформить configurable extension; измеренное основание вместо предположения |

## 6. Итог для UX Staffy

Главная ценность источников Montagio — не конкретный nine-step workflow и не Django UI, а доказательно сформулированный набор сквозных инвариантов: один контекст, role-specific next action, progressive intake, явная граница AI/rules/human, versioned Commitments, server-confirmed Actions, progressive access, recovery и доступность. Staffy должен вынести эти механики в общее ядро и UI kit, но оставлять service outcomes, master lifecycle, price/supply economics, partner lead и warranty decisions в Domain Pack Montagio. Самый опасный перенос — сделать `order_id`, Founder Revenue Inbox и ограничительную human-review модель Montagio единственной архитектурой всех ИИ-сотрудников.
