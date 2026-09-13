---
title: "Staffy — спецификация пользовательского опыта"
status: draft
project: staffy
created: 2026-09-13
updated: 2026-09-13
sources:
  - ../../prds/prd-staffy-2026-09-12/prd.md
  - ../../prds/prd-staffy-2026-09-12/addendum.md
  - ../../prds/prd-staffy-2026-09-12/delivery-slices.md
  - ../../prds/prd-staffy-2026-09-12/open-items-triage.md
  - ../../briefs/brief-staffy-2026-09-12/brief.md
  - ../../briefs/brief-staffy-2026-09-12/addendum.md
  - ../../../forge/staffy-vertical-voice-agent/forged-idea.md
  - ../../research/competitive-staffy-2026-09-11/research.md
  - ../../research/technical-staffy-voice-architecture-2026-09-11/research.md
  - ../../../../чат с ии-поисковиком.md
  - /home/art/projects/montagio/app/docs/SLICE_C_UX_SPEC.md
  - /home/art/projects/montagio/app/docs/SLICE_C_WIREFLOWS.md
  - /home/art/projects/montagio/app/docs/AI_OPERATOR_BLUEPRINT_2026.md
  - /home/art/projects/montagio/app/docs/VOICE_STACK_ADR_2026.md
  - /home/art/projects/montagio/Montagio_legal_financial_model.md
  - /home/art/projects/montagio/Montagio_personal_data_consent.md
---

# Staffy — спецификация пользовательского опыта

Этот spine описывает проверяемое поведение интерфейсов Staffy. Он не дублирует PRD и не выбирает реализацию коннекторов. При конфликте с `.working`-схемами этот документ и `DESIGN.md` имеют приоритет; исторические ветки остаются только источником контекста.

## Foundation

### Инварианты опыта

| Инвариант | Проверяемое поведение |
|---|---|
| Один продукт, разные бизнесы | Tenant настраивает сотрудника из общего ядра, Domain Pack, Connector и policy; новая компания не получает отдельный UX-форк. |
| Assisted-first без кода | Короткое интервью создаёт подтверждаемый Employee Brief и персональный Launch Plan. Поддерживаемый pack/connector настраивается без программиста; неизвестное честно становится extension/gate. |
| Один Case | Голос, чат и защищённая web-страница продолжают один Conversation/Case; личности не объединяются по догадке. |
| Один отвечающий | В каждый момент есть один Response Owner и одна writable surface. Handoff считается состоявшимся только после acknowledgement. |
| Свободный диалог, детерминированное обещание | Модель понимает и формулирует; цена, применимость, полномочие, риск и Business Action определяются policy и актуальным Owning System. |
| Правда о состоянии | `sent`, `delivered`, `acknowledged`, `confirmed` и `reconciled` различаются. Unknown, stale и partial не выглядят успехом. |
| Прогрессивная квалификация | Сначала полезный ответ и safety check, затем только вопрос, который меняет маршрут; известное не спрашивается повторно. |
| Два смысловых обязательства | Предварительное Confirmation разрешает supply/подготовку, финальное — точную материальную версию после readiness; финал показывает именованного исполнителя/стороны и material delta. |
| Человек доступен | Просьба человека, safety, privacy, claim и неизвестный исход прекращают selling/auto-actions и запускают подтверждаемый handoff либо честный fallback. |
| Исправление контролируемо | Исправление текущего Case отделено от улучшения employee; изменение проходит draft → eval → publish, имеет scope и rollback. |

### Поверхности и роли

| Роль | Основная поверхность | Ответ, который должна дать |
|---|---|---|
| Владелец Tenant | Role-Adaptive Today, Employees, Results, Organization | Что произошло, где риск/расход, что решить сейчас, можно ли откатить. |
| Настройщик/консультант | Employee Brief, Launch Plan, Integrations, Knowledge, Test & Release | Что уже готово, что блокирует, кто отвечает и какое evidence разрешает запуск. |
| Оператор/менеджер | Внешний Inbox либо минимальная Staffy surface + Work control layer | Что видит клиент, что подтверждено/спорно, кто отвечает и какое действие безопасно. |
| Доменный эксперт/QA | Correction, tests, activity/results | Где причина ошибки, какой scope изменения и прошло ли regression evidence. |
| Клиент бизнеса | Телефон, native chat, protected continuation | Кто отвечает, что понято, что обещано, что изменится и как позвать человека. |
| Исполнитель/контрагент | Tenant-selected operational surface | Какой scope предложен/принят, что можно видеть, что подтвердить и где stop-work. |

### Открытые decision gates

Все OQ ниже остаются открытыми. UI должен показывать blocking/limited и безопасный fallback, а не заполнять неизвестность. Номера и моменты соответствуют PRD.

| Gate | Решить до | UX до решения |
|---|---|---|
| OQ-1 Mango Office/actual SIP Capabilities | Connector design/controlled calls | Voice connection — `awaiting verification/limited`; transfer/codec/caller ID не обещать. |
| OQ-2 Jivo account semantics | Публикации Jivo Connector | Только проверенные media, timeout, handoff/return; одна writable surface. |
| OQ-3 ABCP Supplier Orders/status | Первого изменяющего controlled test | `pending_manager_verification` — нейтральная проекция, не фактическое имя статуса. |
| OQ-4 ABCP→1С boundary | Сквозного Order test | Не показывать terminal success без reconciliation и operation ID. |
| OQ-5 Каталоги/поставщики/autonomy | Публикации Autonomy Class | Не предлагать позицию без разрешённого applicability evidence. |
| OQ-6 Цена и модель пилота Олега | Подписания/платного пилота | Индикативная упаковка не равна счёту/обязательству. |
| OQ-7 Выборки UJ-M1/UJ-M2 и thresholds | Operating Profile и live/scale | Порядок по readiness; оба proofs обязательны до scale. |
| OQ-8 Montagio fallback owners/SLA | Controlled stage | Не обещать немедленную помощь; показывать owner, acknowledgement и честный next update. |
| OQ-9 Источник/freshness цены Montagio | Публикации цены услуги | Цена blocked/stale; доступен range/measurement/human route. |
| **OQ-10 Montagio-подрядчик** | **Публичного оффера и первого публичного оплаченного Order** | **Публичные договор, оплата, чек и гарантия Montagio заблокированы. Допустима только закрытая проверка спроса с правдивыми фактическими сторонами.** |
| OQ-11 Residency/logging/retention | Production ПДн/recording | Recording/media выключены либо разрешён ограниченный альтернативный путь. |
| OQ-12 Voice baseline/SLO | Live voice gate | Не обещать числовую мгновенность; показывать progress/fallback по измеренному route. |
| OQ-13 Availability tier | Коммерческого SLA | Best-effort/availability copy и fallback не смешивать. |
| OQ-14 Quantitative thresholds | Допуска следующего этапа | Повышение автономности вручную и локально; нет «авто по средней точности». |
| OQ-15 Pricing/usage/WTP proof | Повторяемого предложения | Разделять setup/support, traffic и result; не заявлять validated retention. |
| OQ-16 Cross-case Customer Profile | Cross-case personalization design | Память ограничена текущим Case; новый Case не наследует данные молча. |
| OQ-17 Operator surface implementation | Третьего Tenant без Inbox | Внешний Inbox предпочтителен; иначе минимальная surface обязана пройти Human Surface Contract. |
| OQ-18 Channel/provider sequence | Roadmap Connector | Не показывать roadmap-логотип как доступную интеграцию. |
| OQ-19 State/Authority Matrix | Storage ownership/low-level design | Материальный факт без authority/version/time — unknown и не разрешает действие. |

## Information Architecture

Каноническая схема: [IA Staffy](.working/ia-2026-09-13.excalidraw).

| Уровень | Разделы | Правило раскрытия |
|---|---|---|
| Public | Fit interview → fit outcome → pilot/extension plan | До логинов и секретов; объясняет покрытие, условия, людей, usage и proof. |
| Workspace | Today · Employees · Work · Integrations · Results · Organization | Навигация адаптируется к роли/правам, но названия и URL стабильны. |
| Today | Рекомендуемое решение · сотрудники · риски · outcomes · spend | Один главный next step; владельцу не показывать инфраструктурный шум. |
| Employee | Overview · Role & Process · Knowledge · Channels & Systems · Authority & Human · Test & Release · Activity & Results | Domain Slot встраивается внутрь, не создаёт отдельное приложение/ядро. |
| Work | Очередь decision-ready cases + control layer | Reply открывается только в выбранной единственной surface. |
| Integrations | Catalog · setup · account/capability tests · limitations/fallback | Авторизация не равна readiness. |
| Results | Business outcomes · quality · human minutes · spend · evidence | Metric ведёт к Case/version, а не остаётся dashboard vanity. |
| Organization | People · roles/rights · tariff/limits · audit | Least privilege; secrets не отображаются консультанту/модели. |
| Customer | Phone/native chat → protected continuation → тот же Case | Mobile-first, без обязательного аккаунта; step-up только перед merge/sensitive/material step. |

Work следует Human Surface Contract: подходящий Tenant Inbox (для первого пилота Jivo после gate) остаётся единственным composer; иначе используется минимальная Staffy operator surface, но не обе. Отдельное мобильное приложение Staffy не входит в MVP.

## Voice and Tone

### Общие правила

- Russian-first, короткие фразы, один смысловой блок и не более двух тесно связанных вопросов.
- Первый контакт: «Я — ИИ-/цифровой сотрудник [Tenant]»; не маскировать AI и не повторять disclosure в каждом сообщении.
- Сначала ответ по существу, затем объяснимый вопрос: «Уточню X — это изменит Y».
- Уверенность тона не заменяет evidence. Запрещены выдуманные цена, ETA, наличие, диагноз, назначение, transfer и success.
- Нет давления, стыда, ложной срочности, фиктивных скидок или персональной цены по предполагаемой платёжеспособности.
- При критической сущности используется короткий readback только спорного фрагмента; пауза, overlap и «угу» не считаются материальным согласием.

| Mode | Тон и разрешённое поведение | Запрет |
|---|---|---|
| `service/sales` | Спокойно помогает выбрать доказуемый результат; связывает цену только с разрешённой ценностью. | Давление и неподтверждённые сравнения. |
| `safety` | Краткая утверждённая инструкция, локализация угрозы, немедленный handoff/fallback. | Selling, upsell, удалённый диагноз. |
| `privacy/no_contact` | Подтверждает прекращение контакта/проверку consent; объясняет дальнейшее без спора. | Продолжать квалификацию или маркетинг. |
| `claim` | Признаёт обращение, называет owner/next update, собирает только недостающее evidence. | Признавать вину, обещать refund/компенсацию. |
| `human_requested` | Прекращает продажу, сохраняет контекст, сообщает фактический handoff state. | Уговаривать остаться с AI. |
| `wait` | Говорит, чего ждём, кто владелец, что будет дальше; progress cue только при реальном ожидании. | Fake ETA и пустая «активность». |
| `recovery/unknown` | Отделяет подтверждённое от неизвестного, объясняет риск дубля и безопасную проверку. | Blind retry или «готово». |

## Component Patterns

Имена дословно совпадают с `DESIGN.md`.

| Component | Поведенческая строка |
|---|---|
| **App Shell** | Сохраняет Tenant/role context, права и back behavior; deep link возвращает в тот же object/version без раскрытия недоступных данных. |
| **Role-Adaptive Today** | Сортирует по риску и роли; показывает одно решение сейчас, outcomes/spend и причины изменений; локальное ограничение не объявляет весь Tenant аварийным. |
| **Recommended Action Card** | CTA доступна только назначенной роли; показывает side effect, owner, deadline и fallback; после действия заменяется receipt, не исчезает. |
| **AI Employee Card** | Открывает employee workspace; различает Employee Status, channel limitations и operating outcome; pause действует только на выбранный scope. |
| **Status Badge** | Имеет текстовое значение, причину и consequence; никогда не является единственным источником состояния или CTA. |
| **Integration Card** | Ведёт через credentials owner → account check → capability test → trial/live admission; секрет не читается моделью/консультантом. |
| **Capability State** | Разрешает функцию только после account-level evidence; unknown fail-closed, limited предлагает проверенный fallback. |
| **Employee Brief** | Создаётся из интервью, редактируется секциями и отдельно подтверждается owner; изменение материального поля обновляет Launch Plan/readiness. |
| **Launch Plan** | Поддерживает async provider gates, delegation, blocker/resume и переупорядочивание по readiness; завершение этапа требует evidence. |
| **Delegated Task** | Требует assigned owner и acknowledgement; no-answer возвращает задачу в plan/escalation, не считается выполнением. |
| **Knowledge Source Row** | Позволяет увидеть provenance/freshness/conflict и исправить источник; stale material fact блокирует затронутое claim/action. |
| **Conversation Timeline** | Объединяет каналы только после identity proof; показывает фактически доставленное/услышанное, Correction и Action Receipt, сохраняя исходную запись. |
| **Work Control Card** | Даёт situation-first summary и одно безопасное действие; trace раскрывается отдельно; переход к reply surface не создаёт второй composer. |
| **Response Owner Control** | Атомарно проводит requested → assigned → acknowledged → active → returned; owner меняется только на acknowledged/explicit return. |
| **Human Handoff Panel** | Передаёт summary, facts/disputes, risk, promises, tool outcomes и next action; timeout ведёт в callback/reassignment/fallback. |
| **Commitment Review** | Confirmation связан с exact version; первое разрешает подготовку/supply, финальное — точное обязательство; после изменения показывает material delta и инвалидирует старое согласие. |
| **Action Preview** | Показывает actor, target, Owning System, effect, recipient и exact material fields; недоступно при stale/unknown/insufficient authority. |
| **Action Receipt** | Отделяет request от business outcome, поддерживает partial/unknown и idempotency; repeat доступен лишь после доказанной безопасности. |
| **Protected Continuation** | Один expiring/revocable link на request, переносит permitted facts, даёт correction и возвращает result в ведущий Case; no-account и equivalent fallback обязательны. |
| **Media Uploader** | До upload объясняет цель/minimization, предупреждает о третьих лицах, даёт preview/remove/redact/retry и путь без media. |
| **Correction Panel** | «Fix current case» меняет разрешённый факт/summary с provenance; «Improve employee» создаёт scoped draft, regression test, publish и rollback. |
| **Test & Release Panel** | По умолчанию simulation; реальное действие — только test account + явное включение + receipt; promotion проходит Demo → Check → Trial → Controlled Work → Work. |
| **Result Metric** | Показывает denominator/baseline/period/evidence; drill-down связывает outcome с Case, human correction и config version. |
| **Budget Meter** | Предупреждает до лимита и предлагает scoped action; не отключает safety/human path и не скрывает variable provider costs. |
| **Recovery Panel** | Создаёт один linked Recovery Case, отделяет acknowledged от resolved, назначает owner/next update и хранит reconciliation evidence. |
| **Domain Slot** | Валидирует domain fields/policies, но использует общие identity, authority, Confirmation, owner, action и recovery primitives. |

## State Patterns

| State family | Канонические значения | UX-переход/ограничение |
|---|---|---|
| Fit Status | `preliminary fit` · `fit with conditions` · `Staffy extension required` · `no fit now` | До credentials; каждый outcome имеет причину и следующий шаг. |
| Connection Status | `disconnected` · `connected awaiting verification` · `verified` · `verified with limitations` · `needs attention` | OAuth/SIP success не переводит в verified без capability test. |
| Employee Status | `configuring` · `blocked questions` · `ready for trial` · `in trial` · `working` · `working with limitations` · `paused` | Pause/limitation scoped; причина и fallback видимы. |
| Admission | `Demo` · `Connection Check` · `Trial` · `Controlled Work` · `Work` | Повышение только с evidence; critical error откатывает затронутый scope. |
| Conversation Mode | `service/sales` · `safety` · `privacy/no_contact` · `claim` · `human_requested` · `wait` · `recovery/unknown` | Safety/privacy/claim/human request выше sales и блокируют несовместимые Actions. |
| Response Ownership | `AI active` · `requested` · `assigned` · `acknowledged/human active` · `returned` · `failed` | Ровно один owner; assigned ещё не означает помощь. |
| Delivery Truth | `queued` · `API accepted` · `delivered` · `read/heard if proven` · `failed` | Неподдерживаемый receipt не имитируется. |
| Business Action | `prepared` · `awaiting Confirmation` · `executing` · `confirmed` · `failed` · `partial` · `unknown/reconciling` | Success только от Owning System; unknown не допускает blind retry. |
| Protected Link | `delivered` · `waiting` · `completed` · `expired` · `cancelled/revoked` | Resend/revoke не создаёт новый Case; expiry имеет безопасный retry. |
| Consent | `unknown` · `accepted` · `declined` · `ambiguous` · `revoked` | Тишина не consent; отказ ведёт в равноценный допустимый путь. |
| Wait | `waiting media` · `waiting specialist` · `waiting supply` · `waiting reconciliation` | Всегда owner, причина, next update и expiry/fallback; без fake ETA. |
| Evidence/Freshness | `current` · `stale` · `conflicting` · `missing` · `confirmed by customer` | Stale/conflict блокирует только материально затронутый claim/action. |
| Correction/Release | `reported` · `case fixed` · `draft` · `testing` · `ready` · `published` · `rolled back` | Нет silent learning/cross-tenant transfer; production incident может локально pause. |
| Recovery Case | `opened` · `acknowledged` · `investigating` · `waiting` · `resolved` · `reopened` | Claim acknowledgement не равно resolution; исходный Case остаётся связанным. |
| Surface/System | `loading` · `empty` · `partial` · `offline` · `permission denied` · `provider outage` · `stale view` | Сохраняет введённое, сообщает что доступно, даёт retry/fallback без ложного success. |

## Interaction Primitives

| Primitive | Контракт |
|---|---|
| AI disclosure | Один раз в начале содержательного контакта; повтор при смене Tenant/роли или прямом вопросе. |
| Safety first | Короткий gate до длинного intake; P7 — interrupt, не отдельный коммерческий workflow. |
| Progressive question | Один смысловой блок, максимум два связанных вопроса; рядом причина, что изменит ответ. |
| Critical readback | VIN/OEM, адрес, сумма, дата/окно, модель/позиция и иные Domain Pack entities подтверждаются явно; исправляется delta. |
| Two Confirmations | #1 — понятый scope/route перед supply; #2 — exact final version после readiness. Молчание/overlap не согласие. |
| Protected continuation | Объяснить зачем/что переносится/как вернуться; verified leading channel, expiring link, risk-based step-up, no PII in URL. |
| Channel switch | Показать перенесённые факты и provenance; не повторять intake; слабое identity match не merge. |
| Attachment | Запросить только необходимый файл/ракурс; privacy warning, progress, retry/remove/redact и equivalent no-media path. |
| Callback | Согласовать канал/окно и owner; уведомление сотруднику не равно принятому callback. |
| Human takeover | Freeze AI actions, передать decision-ready context; после acknowledgement отвечает только человек; return явный. |
| Material Action | Preview exact effect → explicit Confirmation → idempotent execution → Action Receipt/reconciliation. |
| Admission | Simulation default; Connection Check и Trial не меняют production; Controlled Work ограничен scope/volume/time/coverage. |
| Correction | Сначала исправить Case; улучшение employee — отдельный versioned release с regression evidence. |
| Destructive/high-stakes action | Отдельный control, точный side effect и target, re-auth/step-up при необходимости; никогда не primary by default. |
| Voice turn | Barge-in останавливает слышимую речь; контекст включает только фактически услышанное; после reconnect перепроверяется незавершённое Commitment/action. |

## Accessibility Floor

- WCAG 2.2 AA как минимальная цель: contrast проверяется для текста, controls, focus и всех semantic states.
- Полная клавиатурная навигация, логичный focus order, skip link, видимый `colors.focus`; modal возвращает focus источнику.
- Touch target не меньше 44×44 px; zoom 200% без потери действий, 400% для линейного чтения protected continuation.
- Status/risk/progress не зависят от цвета, hover, анимации или аудио; live updates объявляются без захвата focus.
- У форм явные labels, ошибки связаны с полями и сохраняют ввод; VIN/OEM/телефон допускают посимвольное чтение и copy.
- Reduced motion поддерживается; timeouts/expiry заранее сообщаются и продлеваются там, где это безопасно.
- Голос не единственный путь: DTMF/text/human/callback; фото и drag-and-drop имеют файловую/текстовую альтернативу.
- AI disclosure, consent, critical readback, цена, сторона договора, payee и claim owner доступны в понятном тексте, не только юридической ссылкой.

## Responsive & Platform

| Context | Поведение |
|---|---|
| Desktop ≥1200 | Навигация + рабочая область; context rail только для decision/supporting detail. Dense expert tables допустимы с keyboard support. |
| Tablet 768–1199 | Навигация/rail становятся drawers; primary decision остаётся в viewport, таблицы переходят в карточки или controlled scroll внутри области. |
| Mobile <768 | Одна колонка, один next action, summary до деталей; no horizontal page scroll; high-stakes CTA не прилипает поверх условий. |
| Protected continuation | Mobile-first и low-bandwidth: server-renderable core, возобновляемая загрузка, сохранение draft, точный delivery state. |
| Voice | UI не нужен для базового диалога; protected link используется для media/critical entry/comparison/Confirmation, а не для переноса всего разговора. |
| Native chat/Inbox | Composer следует фактическим Capability; Jivo/другой Inbox — один writer, Staffy control layer не отвечает параллельно. |
| Weak network/outage | Optimistic UI запрещён для Business Actions; queued/local draft различается с server/Owning System receipt. |

Admin/ops MVP — только light «Северный кедр». Dark mode остаётся открытым и не должен молча появиться в реализации. Отдельное native mobile app Staffy — не MVP.

## Inspiration & Anti-patterns

| Перенять как принцип | Граница |
|---|---|
| Inbox: единая история, assignment, acknowledgement | Не копировать конкретный competitor layout; не создавать второй composer. |
| CRM: decision-ready object и audit trail | Не превращать разговор в форму из десятков обязательных полей. |
| Release tools: diff, tests, publish, rollback | Не показывать prompt/code владельцу как основной способ настройки. |
| Consumer checkout: ясный review и delta | Не объединять consent, order, payment, marketing и data transfer одной кнопкой. |
| Incident tooling: owner, severity, next update, recovery | Не выставлять инфраструктурные логи клиенту или владельцу без progressive disclosure. |

Визуальные исследования: [направления](.working/directions-4.html), [темы](.working/color-themes-5.html), [onboarding Ирины](.working/flow-irina-onboarding-2026-09-13.excalidraw), [handoff и смена канала](.working/flow-cross-channel-handoff-2026-09-13.excalidraw), [автозапчасти](.working/flow-autoparts-a1-a7-2026-09-13.excalidraw), [Montagio target](.working/flow-montagio-m1-m7-2026-09-13.excalidraw). Схема Montagio уже отражает contractor target; любые несовместимые исторические Stage 0 узлы не переопределяют OQ-10. Key-screen файлы ещё не созданы и здесь не подразумеваются.

## Key Flows

Каждый flow назван как в PRD. Шаги — UX spine, не новая бизнес-спецификация.

| Journey / protagonist | Нумерованный flow | Climax | Failure / recovery |
|---|---|---|---|
| **UJ-P1 — Ирина проверяет соответствие Staffy до передачи доступов** | 1) описывает результат; 2) видит pack/connector/capability fit; 3) проверяет условия, людей и usage; 4) выбирает pilot/extension/no-fit. | Решение о pilot scope без credentials. | Unknown capability → `fit with conditions/extension`, владелец и проверка; не false-ready. |
| **UJ-P2 — Ирина запускает поддерживаемого ИИ-сотрудника без программиста** | 1) интервью; 2) подтверждает Employee Brief; 3) выполняет/делегирует Launch Plan; 4) подключает и проверяет; 5) simulation/evals; 6) Controlled Work; 7) Today/rollback. | Сквозной Action + acknowledged Handoff в испытании. | Async/provider blocker сохраняет прогресс и fallback; critical regression блокирует publish/откатывает scope. |
| **UJ-M1 — Анна заказывает замену смесителя через Montagio** | 1) AI disclosure+safety; 2) progressive intake/media-or-alternative; 3) P1/P2 и Confirmation #1; 4) supply/contractor readiness; 5) final Commitment; 6) work/acceptance/payment/receipt/claim states. | После OQ-10 PASS Анна подтверждает договор, оплату, чек и гарантию Montagio, а специалист остаётся субподрядчиком. | До OQ-10 public order/payment blocked; no supply → окно/wait/human/P-route без fake ETA. |
| **UJ-M2 — Анна покупает водонагреватель в партнёрском магазине** | 1) Staffy называет Montagio и источник; 2) Анна проверяет provenance/consent; 3) уточняется только compatibility gap; 4) route+Confirmation #1; 5) readiness; 6) final Commitment и ограниченный partner status. | Контекст магазина превращён в точный заказ без повторного intake и с правдивыми сторонами. | Consent disputed → `privacy/no_contact`; OQ-10 блокирует публичное contractor commitment; callback только по выбору/риску/пробелу. |
| **UJ-M3 — Ольга начинает на Avito и безопасно продолжает в защищённом контуре** | 1) ответ по объявлению; 2) scope/value comparison; 3) объяснение protected switch; 4) identity check; 5) correction+media/Confirmation; 6) результат в исходном Case. | Ольга продолжает один диалог без повтора и подтверждает точные данные. | Нет официальной capability/слабый match → assisted/blocked, no merge, другой канал/человек. |
| **UJ-M4 — Дмитрий сообщает об аварийной ситуации в кафе** | 1) P7 safety interrupt; 2) краткая approved инструкция; 3) объект/угроза; 4) owner/budget authority/payee; 5) acknowledged handoff; 6) заново разрешённый P4/P5/P6. | Уполномоченный человек принимает decision-ready emergency context. | Handoff not acknowledged/no supply → честный fallback; никаких SLA, выезда или цены без readiness. |
| **UJ-M5 — Михаил покупает инженерный осмотр отопления** | 1) цели/ограничения; 2) планы/media с protected path; 3) Staffy обозначает предел диагностики; 4) предлагает P4 как отдельный результат; 5) куратор проверяет; 6) уполномоченный подтверждает. | Михаил покупает обследование, а не ложную дистанционную «точную смету». | Недостаточно evidence/high risk → human gate; нельзя проектировать систему или фиксировать сложную цену. |
| **UJ-M6 — Валентина не может прислать фото или просит человека** | 1) сохраняются известные факты; 2) предлагается no-media route; 3) просьба человека включает `human_requested`; 4) requested→assigned→acknowledged; 5) человек продолжает Case. | Валентина не повторяет историю и знает фактического owner. | No acknowledgement → callback/reassignment/другой supported path с честным next update. |
| **UJ-M7 — Алексей обнаруживает дополнительную работу на объекте** | 1) stop affected work; 2) evidence+reason; 3) новая scope/price/material/time version; 4) Анна видит delta; 5) отдельное Confirmation; 6) resume либо safe close. | Новая работа начинается только после material-delta согласия. | Отказ/молчание → не выполнять; восстановить исходное состояние или human review. |
| **UJ-M8 — Анна возвращается с претензией после выполнения** | 1) найти Order; 2) `claim` и при угрозе `safety`; 3) linked Recovery Case; 4) собрать missing evidence; 5) human acknowledgement; 6) updates/resolution/reopen. | Анна получает принятого owner и право оспорить summary/result. | Staffy не определяет вину/refund; no acknowledgement → escalation/fallback, incident может локально pause scope. |
| **UJ-A1 — Сергей подбирает тормозные колодки по телефону** | 1) AI disclosure; 2) intent/work items; 3) protected VIN+readback; 4) catalog applicability; 5) original/analogs; 6) Confirmation; 7) ABCP order `pending_manager_verification`. | Один подтверждённый Order и decision-ready задача менеджеру. | Conflict/unknown outcome → no order/success, human or reconciliation by operation ID. |
| **UJ-A2 — Мария подбирает деталь в Jivo и продолжает голосом** | 1) Jivo intake/media; 2) provenance; 3) согласованный звонок; 4) один Case/identity check; 5) selection+Confirmation; 6) order/handoff. | Канал меняется без повтора VIN и без второго заказа. | Media unsupported → protected link; after acknowledged Jivo remains sole writer; no parallel AI reply. |
| **UJ-A3 — Николай повторяет заказ по известному OEM/артикулу** | 1) OEM/previous item; 2) заново проверить vehicle/applicability; 3) обновить price/stock/ETA; 4) readback+Confirmation; 5) idempotent order; 6) manager gate по текущему class. | Точный повтор завершён по актуальным данным, не по памяти. | Stale/conflict → уточнение/human; старый заказ не считается evidence текущей применимости. |
| **UJ-A4 — Алексей описывает симптом вместо требуемой детали** | 1) распознать diagnostic intent; 2) собрать минимум vehicle/VIN/symptom; 3) объяснить предел; 4) передать менеджеру/approved diagnostic service. | Decision-ready handoff без заказа гипотетической детали. | Нельзя превращать LLM hypothesis в SKU; no human → callback/fallback. |
| **UJ-A5 — VIN, документ и каталог противоречат друг другу** | 1) показать спорное поле; 2) protected correction; 3) readback; 4) повторная catalog check; 5) unresolved conflict handoff с обоими values/provenance/candidates. | Исправленное значение каталогически разрешает продолжение. | Конфликт не снят → заказ blocked; «самый вероятный» вариант запрещён. |
| **UJ-A6 — Елена просит специалиста или Staffy не может завершить операцию** | 1) stop auto flow; 2) собрать context/tool outcomes; 3) requested→assigned; 4) удерживать линию до acknowledgement; 5) человек отвечает либо callback. | Фактический acknowledged takeover с полным контекстом. | Transfer trigger/no-answer не success; fallback сохраняет Case и один Response Owner. |
| **UJ-A7 — Антон проверяет заказ Staffy и находит ошибку** | 1) принимает exact-version task; 2) проверяет applicability evidence; 3a) approve и вручную продолжает supplier flow; 3b) block, звонок, корректная замена; 4) новая delta Confirmation; 5) повторная verification. | Human approval receipt либо исправленная подтверждённая версия. | Staffy не имеет supplier-send Action в пилоте; supplier status только из Owning System, unknown → reconcile. |
| **UJ-A8 — Олег повышает или откатывает автономность класса** | 1) выбирает Autonomy Class; 2) видит cases/corrections/critical errors/human minutes/margin; 3) сверяет evidence window; 4) повышает scope; 5) мониторит; 6) локально откатывает. | Осознанное повышение одного доказанного класса. | Critical error/threshold unknown → pause/rollback только затронутого класса; audit сохраняется. |
| **UJ-A9 — Игорь обращается по статусу, возврату или рекламации** | 1) identity check; 2) найти исходный Order; 3) прочитать актуальный status; 4) сообщить только allowed fact; 5) claim/return/unknown → linked handoff/recovery. | Ответ связан с тем же Order и подтверждён Owning System. | ABCP→1С mismatch/irreversible dispute → no new order/no false success, human acknowledgement и reconciliation. |

Визуальная трассировка ключевых групп: [onboarding UJ-P1/P2](.working/flow-irina-onboarding-2026-09-13.excalidraw), [cross-channel/handoff](.working/flow-cross-channel-handoff-2026-09-13.excalidraw), [UJ-A1/A5/A7](.working/flow-autoparts-a1-a7-2026-09-13.excalidraw), [target UJ-M1/M6/M7](.working/flow-montagio-m1-m7-2026-09-13.excalidraw).
