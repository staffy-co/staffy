# Staffy UX — независимая проверка safety и реальной эксплуатации

Дата: 2026-09-13  
Scope: `DESIGN.md`, `EXPERIENCE.md`, UX memlog, PRD/addendum/delivery slices/open items и ключевые Excalidraw-потоки двух пилотов.  
Линзы: adversarial + exhaustive edge-path tracing; отдельно применён Ponytail/Pareto cut. Основные UX-spines не изменялись.

## Вердикт

**Условно принимается к доработке, но пока не готово к Architecture handoff без разрешения четырёх critical gaps ниже.** Концепция не выглядит ошибочным оверинжинирингом: один Response Owner, fail-closed Capabilities, версионированные Commitments, Action Receipt/reconciliation, human gate и OQ-10 — необходимые свойства продукта, который меняет заказы, деньги и обещания. Оверинжиниринг возникает только если все описанные компоненты и dashboards реализовывать до walking skeleton.

Рекомендуемый срез: сохранить универсальные контракты ядра, но запустить каждый пилот через минимальные поверхности и `manual-with-audit`. Не строить отдельное ядро под отрасль и не строить всю будущую платформу до первых доказательств.

Итого material findings: **14** — critical: **4**, high: **8**, medium: **2**.

## Fix before Architecture handoff / controlled mutation

### F-01 — Critical — подрядная модель Montagio конфликтует со старыми требованиями PRD

- **Location:** `prd.md:1028–1036`, `prd.md:1162`, `prd.md:1178`, `prd.md:1385`; override — `addendum.md:19–25`; UX — `EXPERIENCE.md:73, 241–242`.
- **Trigger:** UX проектирует Montagio как продавца/подрядчика, а FR-75 и метрики всё ещё хранят `fee_due/fee_collected`, документ исполнителя и Stage-0 комиссию; Non-Goals одновременно исключают приём всей оплаты до gate.
- **Fix:** до Architecture создать одну normative contractor-state table для первых 3–5 услуг: customer payment → Montagio, receipt issuer → Montagio, subcontractor payable/payout, acceptance, refund/reversal, warranty/claim, provisional/final margin. Старую прямую/комиссионную ветку пометить `closed-validation-only`, не как публичный runtime default.
- **Consequence:** команды реализуют несовместимые денежные модели; возможны ложные чеки, отчётность, оффер и ответственность.

### F-02 — Critical — manager supplier gate описан, но его техническая необходимость ещё не доказана фактическим ABCP-путём

- **Location:** `prd.md:857–867`, `EXPERIENCE.md:66–67, 255`; `.working/flow-autoparts-a1-a7-2026-09-13.excalidraw`, nodes `Одобрение exact version` и `Supplier-send fence`.
- **Trigger:** менеджер подтверждает версию в Staffy, затем вручную продолжает процесс в ABCP; внешний интерфейс или обмен ABCP→1С может позволить сформировать/отправить supplier order после изменения позиции, цены, количества или evidence, минуя Staffy approval.
- **Fix:** до первого mutating test закрыть OQ-3/OQ-4 и доказать account-level fence. Если атомарную блокировку нельзя обеспечить, UI не должен называть состояние «одобрено к отправке»: использовать `manual verification required`, повторный read текущей ABCP-версии непосредственно перед ручной отправкой и отдельный human receipt. Любое расхождение/нет correlation ID → block + reconciliation.
- **Consequence:** неверная деталь уйдёт поставщику несмотря на видимый human gate.

### F-03 — Critical — нет применимой State/Authority Matrix, хотя UX показывает материальные terminal states

- **Location:** `EXPERIENCE.md:67, 145–146, 167, 249, 257`; `addendum.md:92`; `prd.md:1429`; `.working/flow-autoparts-a1-a7-2026-09-13.excalidraw`, node `Правдивый статус`.
- **Trigger:** ABCP, кастомная 1С, supplier state и Staffy receipt могут расходиться; аналогично Montagio имеет заказ, оплату, чек, выплату и claim в разных источниках.
- **Fix:** сделать OQ-19 обязательным входом Architecture: для каждого material field указать Owning System, external ID, version/time, allowed Staffy projection, conflict rule, reconcile query и terminal evidence. В key screen показывать источник/время и раздельные статусы; общий success только когда требуемые источники согласованы.
- **Consequence:** false-success, двойная операция или неверный ответ по существующему заказу.

### F-04 — Critical — единый `Consent` state слишком широк для разных юридических целей

- **Location:** `EXPERIENCE.md:169, 180, 185, 205`; `DESIGN.md:160–161`; `prd.md:1277–1303` (NFR-14–20).
- **Trigger:** `accepted/declined/revoked` может быть ошибочно переиспользован для записи звонка, STT/AI-обработки, media, передачи магазину/мастеру, исходящего контакта и маркетинга.
- **Fix:** не строить отдельный consent subsystem в пилоте, но хранить отдельные purpose-bound receipts: `purpose + artifact/data class + recipient/provider + channel + policy/version + time + expiry/revocation`. В UI спрашивать согласие в точке необходимости и показывать равноценный путь отказа; отзыв останавливает только затронутое использование.
- **Consequence:** незаконная запись/передача данных или чрезмерная остановка обслуживания после частичного отказа.

### F-05 — High — Human Surface Contract зависит от неподтверждённых возможностей Jivo

- **Location:** `EXPERIENCE.md:65, 94, 100, 142–143, 216`; `prd.md:212`, `prd.md:896–912`; `.working/flow-cross-channel-handoff-2026-09-13.excalidraw`, nodes `ОДНО МЕСТО ОТВЕТА`, `ВНЕШНИЙ INBOX ПОДХОДИТ`.
- **Trigger:** Jivo может принять уведомление, но не дать атомарно выключить AI writer, подтвердить takeover или вернуть ownership.
- **Fix:** публикация Jivo Connector требует account test четырёх функций: external reply, assignment/ack, writer exclusion, explicit return. При отсутствии любой — `human_only` с операционной блокировкой AI либо минимальная Staffy surface как единственный composer; никогда не два composer.
- **Consequence:** AI и менеджер дадут противоречивые ответы или создадут два заказа.

### F-06 — High — ownership после acknowledgement не имеет явного lease-loss состояния

- **Location:** `EXPERIENCE.md:138, 142–143, 165, 188–189, 246, 254`; `DESIGN.md:151, 155–156`.
- **Trigger:** человек принял обращение, но закрыл смену, потерял связь или не ответил до обещанного срока; `acknowledged/human active` может оставаться визуально здоровым.
- **Fix:** использовать минимальные состояния `active`, `overdue`, `reassignment required`, `returned/closed`; для каждого assignment задать acknowledgement deadline, ownership lease/next update, backup и verified escalation channel. Клиенту сообщать только фактического owner и новый срок обновления.
- **Consequence:** обращение считается принятым, но фактически остаётся без владельца.

### F-07 — High — pause/rollback может создать ложное ощущение, что внешние действия остановлены

- **Location:** `EXPERIENCE.md:132, 146, 150, 163, 190–193, 240, 256`; `DESIGN.md:145, 159, 163, 166`.
- **Trigger:** оператор нажимает pause после dispatch; queued/in-flight вызов уже мог совершить внешний side effect.
- **Fix:** pause receipt должен показывать generation/fence time и количества `blocked before dispatch`, `in flight`, `unknown/reconciling`, `confirmed`. Нельзя писать «остановлено», пока in-flight outcomes не reconciled; rollback конфигурации не отменяет уже совершённые Actions.
- **Consequence:** оператор повторит/скомпенсирует действие вслепую или сообщит клиенту неверный статус.

### F-08 — High — OQ-10 блокирует коммерцию, но route-level lock может затеряться за общим Employee Status

- **Location:** `EXPERIENCE.md:73, 92–93, 130, 162–164, 241–242`; `.working/flow-montagio-m1-m7-2026-09-13.excalidraw`, gate `OQ-10 · PUBLIC READINESS`.
- **Trigger:** сотрудник Montagio может корректно отвечать и квалифицировать обращения, поэтому выглядеть `working`, хотя публичный оффер, заказ и оплата запрещены.
- **Fix:** не добавлять новую сложную state machine; использовать существующий scoped control на route `public_contractor_order = intake_only/no_promises`. На Today, Employee и Commitment Review показывать один блокирующий gate с последствиями и владельцем; CTA оплаты/заказа отсутствует до PASS.
- **Consequence:** рабочий статус сотрудника ошибочно интерпретируется как юридическая готовность принимать деньги.

### F-09 — High — закрытая direct-master validation упомянута, но не имеет безопасного UX-варианта

- **Location:** UX memlog `:28`; `EXPERIENCE.md:73, 231, 241–242`; `.working/flow-montagio-m1-m7-2026-09-13.excalidraw`, footer `CLOSED CONTROLLED DEMAND VALIDATION ONLY`.
- **Trigger:** команда переиспользует публичный contractor flow до OQ-10 или скрывает реальную сторону договора/оплаты в закрытом тесте.
- **Fix:** выбрать один простой вариант: либо закрытый тест полностью staff-mediated без customer self-service screen, либо отдельная короткая copy/state table, где до Confirmation названы master as contracting party, payee, receipt issuer и warranty owner. Не смешивать обе модели в одном route/config.
- **Consequence:** договорная роль и гарантия будут представлены клиенту ложно.

### F-10 — High — notification менеджеру автозапчастей не имеет полного operational receipt

- **Location:** `EXPERIENCE.md:138, 249, 255`; `DESIGN.md:151, 156`; `.working/flow-autoparts-a1-a7-2026-09-13.excalidraw`, node `Задача Антону`.
- **Trigger:** Order создан, но уведомление не доставлено/не принято; клиенту уже сказано, что идёт финальная проверка.
- **Fix:** задача проверки должна иметь `assigned → delivered(if proven) → acknowledged → approved/blocked/expired`, deadline, backup owner и escalation. До acknowledgement клиент получает обещание следующего обновления, не обещание проверки; истечение блокирует supplier path.
- **Consequence:** заказы зависают без проверки, а клиент получает ложное ожидание отправки.

### F-11 — High — Media Uploader не отражает quarantine/unsafe-file path

- **Location:** `DESIGN.md:161`; `EXPERIENCE.md:148, 174, 187, 214`; `prd.md:1301–1303`.
- **Trigger:** файл загружен, но неверного типа, заражён, содержит данные третьих лиц, не прошёл безопасный render или извлечение оборвалось.
- **Fix:** добавить состояния `uploading → scanning → usable / needs correction / quarantined / rejected`, причину без раскрытия опасного payload и путь удалить/заменить/описать текстом/позвать человека. EXIF и лишние данные минимизировать до передачи downstream.
- **Consequence:** опасный файл попадёт оператору/tool либо клиент увидит ложное «загружено = принято».

### F-12 — High — исчерпание бюджета не имеет определённого customer-safe перехода

- **Location:** `DESIGN.md:165`; `EXPERIENCE.md:130, 152, 217`.
- **Trigger:** hard cap достигнут посреди Conversation, активного Commitment, Handoff или Recovery.
- **Fix:** в Operating Profile определить один деградированный исход по scope: новые обращения → human/callback/intake-only; активные подтверждённые обязательства и safety/recovery не обрываются; owner получает alert и receipt. Не нужен автоматический billing/dunning.
- **Consequence:** клиент брошен после обещания либо расходы продолжаются без контроля.

### F-13 — Medium — «Two Confirmations» сформулированы как глобальный primitive

- **Location:** `EXPERIENCE.md:43, 144, 184`; journeys `:241–255`.
- **Trigger:** правило механически применяется к автозапчастям, простому status inquiry или иным маршрутам, где есть одно customer Commitment и отдельный human approval.
- **Fix:** scope rule: два customer confirmations только там, где предварительная readiness/supply стадия материализует новые условия (прежде всего Montagio). Autoparts: одно exact customer Confirmation перед ABCP Order + отдельный manager approval; изменение material fields инвалидирует соответствующее согласие.
- **Consequence:** лишнее трение, отказы и попытки операторов обходить формальные подтверждения.

### F-14 — Medium — AI disclosure не повторяется при возможной смене участника в том же Case

- **Location:** `EXPERIENCE.md:107, 180`; `.working/flow-cross-channel-handoff-2026-09-13.excalidraw`, nodes `ЛИЧНОСТЬ ДО ОБЪЕДИНЕНИЯ`, `ОБРЫВ / RECONNECT`.
- **Trigger:** общий телефон/чат, forwarded link, callback или reconnect приводит другого человека, но Tenant и role не изменились.
- **Fix:** повторять краткое disclosure при новом/неподтверждённом participant, новом outbound callback и начале записываемого сегмента, а не на каждом сообщении.
- **Consequence:** новый участник не понимает, что общается с ИИ и что разговор может обрабатываться/записываться.

## Keep — обязательные инварианты

1. Один Response Owner и одна writable surface; notification/assignment не равны acknowledgement.
2. `unknown/stale/partial` не зелёные; success только из Owning System или требуемого human receipt.
3. Exact-version Commitment, material delta и новая Confirmation после изменения.
4. Idempotency + reconciliation вместо blind retry.
5. Account-level Capability evidence; Mango/Jivo/ABCP/Novofon не считаются готовыми по названию интеграции.
6. VIN/OEM applicability только из разрешённого каталога/Owning System; symptom не становится SKU.
7. OQ-10 как hard public gate и правдивая contractor identity Montagio.
8. Purpose-bound privacy, путь без записи/media и российский data perimeter.

## Simplify for the two pilots

1. Реализовать четыре поверхности: `Today`, `Employee/readiness`, `Case/control`, `Integration evidence`; domain review встроить как slot. Остальные компоненты сначала могут быть секциями этих экранов.
2. Использовать существующие Jivo, ABCP и операционные поверхности Montagio как system/inbox of work; Staffy добавляет контрольный слой и receipts, а не заменяет CRM/inbox.
3. Supplier approval, Montagio supply, fallback roster, legal gate и cohort marking сначала допустимы как manual-with-audit с одним owner и deadline.
4. Один Action Receipt schema и один Handoff schema на оба пилота; отраслевые поля остаются в Domain Pack, не создают отдельную машину состояний.

## Defer until evidence

1. Полированный public fit configurator и полностью self-service third-tenant launch — после успешных assisted pilots; сохраняется измерение setup steps/time.
2. Универсальный realtime Results dashboard — сначала сверяемый offline/manual report с links на Case/version/receipts.
3. Полностью self-service `Improve employee`/publish/rollback UI — первые исправления могут идти через Staffy expert, но обязаны оставаться versioned и regression-tested.
4. Собственный универсальный Staffy Inbox — только если Jivo/другой выбранный Inbox не проходит Human Surface Contract; не строить «на всякий случай».
5. Автоматический billing/dunning, dark mode, native mobile app, cross-case profile и новые roadmap connectors — остаются вне пилотного критического пути.

## Acceptance after fixes

Перед Architecture handoff достаточно закрыть F-01, зафиксировать формат OQ-19 и явно внести F-04/F-07/F-08 в UX contract. F-02/F-05 закрываются account-level evidence до соответствующего Connector design/controlled mutation; их нельзя считать реализованными заранее. Остальные findings входят в key-screen review и Operating Profile двух пилотов.
