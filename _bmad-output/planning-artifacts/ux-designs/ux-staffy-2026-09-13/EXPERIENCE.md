---
title: "Staffy — спецификация пользовательского опыта"
status: final
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
  - .working/source-montagio-ux.md
  - .working/source-montagio-conversation.md
  - .working/source-montagio-safety-commerce.md
---

# Staffy — спецификация пользовательского опыта

Этот spine описывает проверяемое поведение интерфейсов Staffy. Он не дублирует PRD и не выбирает реализацию коннекторов. При конфликте с `.working`-схемами этот документ и `DESIGN.md` имеют приоритет; исторические ветки остаются только источником контекста.

## Pilot Implementation Cut

В первых двух пилотах 26 компонентов не становятся 26 самостоятельными подсистемами. Walking skeleton состоит из четырёх поверхностей Staffy: `Today`, `Employee/readiness`, `Case/control` и `Integration evidence`; остальные компоненты встраиваются в них как секции. Jivo, ABCP и существующие интерфейсы Montagio остаются основными рабочими системами, если соответствуют контракту возможностей. Редкие операции — одобрение заказа поставщику, управление пулом исполнителей и юридические проверки — сначала могут выполняться вручную с аудитом. Собственный Inbox Staffy, Results в реальном времени и полный самостоятельный publish/rollback создаются только после подтверждённого пробела в существующих поверхностях.

## Foundation

### Инварианты опыта

| Инвариант | Проверяемое поведение |
|---|---|
| Один продукт, разные бизнесы | Tenant настраивает сотрудника с помощью общего ядра, Domain Pack, Connector и policy; для новой компании не создаётся отдельный UX-форк. |
| Assisted-first без кода | Короткое интервью создаёт подтверждаемый Employee Brief и персональный Launch Plan. Поддерживаемый pack/connector настраивается без программиста; неизвестное честно становится extension/gate. |
| Один Case | Голос, чат и защищённая web-страница продолжают один Conversation/Case; личности не объединяются по догадке. |
| Один отвечающий | В каждый момент есть один Response Owner и одна writable surface. Handoff считается состоявшимся только после acknowledgement. |
| Свободный диалог, детерминированное обещание | Модель понимает и формулирует; цена, применимость, полномочие, риск и Business Action определяются policy и актуальным Owning System. |
| Правда о состоянии | `sent`, `delivered`, `acknowledged`, `confirmed` и `reconciled` различаются. Unknown, stale и partial не выглядят успехом. |
| Прогрессивная квалификация | Сначала полезный ответ и safety check, затем только вопрос, который меняет маршрут; известное не спрашивается повторно. |
| Версионированное материальное согласие | Каждое Confirmation связано с точной версией обязательства и теряет силу после material delta. Два клиентских Confirmation применяются только там, где Domain Pack явно разделяет предварительный scope и возникшую после readiness финальную версию; для автозапчастей customer Confirmation и manager approval — разные события. |
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
| OQ-2 Jivo account semantics | Публикации Jivo Connector | Account-level controlled test обязан отдельно доказать reply, assignment, acknowledgement, writer exclusion, timeout и return. До полного PASS — human-only либо Staffy как единственный composer в проверенном scope; неизвестная capability fail-closed. |
| OQ-3 ABCP Supplier Orders/status | Первого изменяющего controlled test | `pending_manager_verification` — нейтральная проекция, не фактическое имя статуса. Без доказанного account-level fence UI говорит «требуется ручная проверка», повторно читает exact ABCP version непосредственно перед ручной supplier-send и требует human receipt. |
| OQ-4 ABCP→1С boundary | Сквозного Order test | Не показывать terminal success без correlation/operation ID, повторного чтения текущей версии и reconciliation; mismatch блокирует действие. |
| OQ-5 Каталоги/поставщики/autonomy | Публикации Autonomy Class | Не предлагать позицию без разрешённого applicability evidence. |
| OQ-6 Цена и модель пилота Олега | Подписания/платного пилота | Индикативная упаковка не равна счёту/обязательству. |
| OQ-7 Выборки UJ-M1/UJ-M2 и thresholds | Operating Profile и live/scale | Порядок по readiness; оба proofs обязательны до scale. |
| OQ-8 Montagio fallback owners/SLA | Controlled stage | Не обещать немедленную помощь; показывать owner, acknowledgement и честный next update. |
| OQ-9 Источник/freshness цены Montagio | Публикации цены услуги | Цена blocked/stale; доступен range/measurement/human route. |
| **OQ-10 Montagio-подрядчик** | **Публичного оффера и первого публичного оплаченного Order** | **Действует ограничение `public_contractor_order = intake_only/no_promises`. Пока OQ-10 не пройден, публичные договор, оплата, чек и гарантия Montagio заблокированы. Закрытую проверку direct-master проводит сотрудник: самостоятельной клиентской страницы покупки нет, а стороны договора, платежа, чека и гарантии указываются правдиво.** |
| OQ-11 Residency/logging/retention | Production ПДн/recording | Recording/media выключены либо разрешён ограниченный альтернативный путь. |
| OQ-12 Voice baseline/SLO | Live voice gate | Не обещать числовую мгновенность; показывать progress/fallback по измеренному route. |
| OQ-13 Availability tier | Коммерческого SLA | Best-effort/availability copy и fallback не смешивать. |
| OQ-14 Quantitative thresholds | Допуска следующего этапа | Повышение автономности вручную и локально; нет «авто по средней точности». |
| OQ-15 Pricing/usage/WTP proof | Повторяемого предложения | Разделять setup/support, traffic и result; не заявлять validated retention. |
| OQ-16 Cross-case Customer Profile | Cross-case personalization design | Память ограничена текущим Case; новый Case не наследует данные молча. |
| OQ-17 Operator surface implementation | Третьего Tenant без Inbox | **UX contract closed / Architecture implementation open:** внешний Inbox предпочтителен; иначе минимальная surface обязана пройти Human Surface Contract. |
| OQ-18 Channel/provider sequence | Roadmap Connector | Не показывать roadmap-логотип как доступную интеграцию. |
| OQ-19 State/Authority Matrix | Storage ownership/low-level design | Это обязательный вход для Architecture. Для каждого материального поля матрица указывает Owning System, внешний ID, версию и время, разрешённую проекцию Staffy, правило разрешения конфликта, запрос сверки и доказательство финального состояния. Без матрицы факт имеет статус `unknown` и не разрешает ни внешнее действие, ни отображение финального успеха. |

## Information Architecture

Каноническая схема: [IA Staffy](wireframes/ia-2026-09-13.excalidraw).

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

На поверхности Work действует Human Surface Contract. Подходящий Inbox Tenant — в первом пилоте Jivo после прохождения gate — остаётся единственной поверхностью ответа. Если он не подходит, используется минимальная операторская поверхность Staffy; одновременно обе поверхности не используются. Отдельное мобильное приложение Staffy не входит в MVP.

## Voice and Tone

### Общие правила

- Russian-first, короткие фразы, один смысловой блок и не более двух тесно связанных вопросов.
- В первом контакте используется один утверждённый вариант: «Я — ИИ-сотрудник [Tenant]» или «Я — цифровой сотрудник [Tenant]». Роль ИИ нельзя маскировать, но это сообщение не нужно повторять в каждой реплике.
- Сначала ответ по существу, затем объяснимый вопрос: «Уточню X — это изменит Y».
- Уверенный тон не заменяет доказательств. Нельзя выдумывать цену, ожидаемое время, наличие, диагноз, назначение, факт перевода на человека или успешный результат.
- Нет давления, стыда, ложной срочности, фиктивных скидок или персональной цены по предполагаемой платёжеспособности.
- При критической сущности используется короткий readback только спорного фрагмента; пауза, overlap и «угу» не считаются материальным согласием.

| Mode | Тон и разрешённое поведение | Запрет |
|---|---|---|
| `sales` | Спокойно помогает выбрать доказуемый результат; связывает цену только с разрешённой ценностью. | Давление и неподтверждённые сравнения. |
| `service/status` | Сообщает только подтверждённое состояние существующего Case/Order и допустимый следующий шаг. | Создавать новый заказ или обещать исход вместо чтения Owning System. |
| `safety` | Краткая утверждённая инструкция, локализация угрозы, немедленный handoff/fallback. | Selling, upsell, удалённый диагноз. |
| `privacy/no_contact` | Подтверждает прекращение контакта/проверку consent; объясняет дальнейшее без спора. | Продолжать квалификацию или маркетинг. |
| `claim` | Признаёт обращение, называет owner/next update, собирает только недостающее evidence. | Признавать вину, обещать refund/компенсацию. |
| `cancellation` | Проверяет объект и состояние отмены, объясняет фактические последствия и разрешённый путь. | Скрытая продажа, автоматическая отмена без authority/receipt. |
| `human_requested` | Прекращает продажу, сохраняет контекст, сообщает фактический handoff state. | Уговаривать остаться с AI. |

`wait` и `recovery/unknown` — не Conversation Modes, а ортогональные operational/recovery states. Они не понижают приоритет канонического Mode.

## Interaction Primitives

| Primitive | Контракт |
|---|---|
| AI disclosure | Один раз в начале содержательного контакта; повтор при смене Tenant/роли или прямом вопросе. |
| Safety first | Короткий gate до длинного intake; P7 — interrupt, не отдельный коммерческий workflow. |
| Progressive question | Один смысловой блок, максимум два связанных вопроса; рядом причина, что изменит ответ. |
| Critical readback | VIN/OEM, адрес, сумма, дата/окно, модель/позиция и иные Domain Pack entities подтверждаются явно; исправляется delta. |
| Versioned Confirmation | Для каждого material Action показывается exact version/effect; material delta инвалидирует согласие. Если Domain Pack Montagio разделяет readiness/supply, #1 разрешает только подготовку, #2 — финальную версию. В автозапчастях customer Confirmation и manager approval не сливаются. |
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
| AI disclosure renewal | Кратко повторяется для нового/неподтверждённого участника, нового outbound callback и начала записываемого сегмента; не повторяется механически в каждом сообщении. |
| Recording consent revoke | Команда голосом/DTMF прекращает затронутую запись после подтверждённого state change; объясняется судьба уже записанного по policy и предлагается no-recording text/human/callback continuation. |

### Voice Accessibility State Machine

- В начале и после repair-limit доступны одинаковые команды: «повторить», «медленнее», «ввести кнопками», «прислать текстом», «позвать человека»; Tenant может добавлять, но не переименовывать safety/help команды.
- DTMF mapping стабилен внутри route и проговаривается кратко; no-input/no-match не повторяется бесконечно, а после настроенного лимита предлагает text/callback/human без потери Case.
- Critical entity Domain Rule задаёт нормализацию, алфавит, группировку и однозначное озвучивание. Для VIN/OEM различаются `О/0`, `I/1` и латинско-кириллические омографы; можно повторить одну группу, замедлить или перейти в protected field.
- Voice, DTMF, text и human paths сохраняют одинаковые требования safety, price, claim, versioned Confirmation и provenance.

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
| **Delegated Task** | Требует `assigned → delivered(if proven) → acknowledged → approved/blocked/expired`, deadline и backup owner; no-answer возвращает задачу в plan/escalation и блокирует зависимое действие. |
| **Knowledge Source Row** | Позволяет увидеть provenance/freshness/conflict и исправить источник; stale material fact блокирует затронутое claim/action. |
| **Conversation Timeline** | Объединяет каналы только после identity proof; показывает фактически доставленное/услышанное, Correction и Action Receipt, сохраняя исходную запись. |
| **Work Control Card** | Даёт situation-first summary и одно безопасное действие; trace раскрывается отдельно; переход к reply surface не создаёт второй composer. |
| **Response Owner Control** | Атомарно проводит requested → assigned → acknowledged/active → overdue → reassignment required → returned; owner меняется только на acknowledged/explicit return, а lease/next-update deadline и backup видимы. |
| **Human Handoff Panel** | Передаёт summary, facts/disputes, risk, promises, tool outcomes и next action; timeout ведёт в callback/reassignment/fallback. |
| **Commitment Review** | Confirmation связан с exact version и инвалидируется material delta. Предварительное и финальное customer Confirmation показываются только для route, где это явно определено Domain Pack; manager approval остаётся отдельным human receipt. |
| **Action Preview** | Показывает actor, target, Owning System, effect, recipient и exact material fields; недоступно при stale/unknown/insufficient authority. |
| **Action Receipt** | Отделяет запрос от бизнес-результата и поддерживает состояния `partial`/`unknown` и идемпотентность. После `Pause` показывает время введения запрета и количество действий в четырёх группах: `blocked before dispatch`, `in flight`, `unknown/reconciling` и `confirmed`. Откат конфигурации не отменяет уже произошедший внешний эффект. |
| **Protected Continuation** | Один expiring/revocable link на request, переносит permitted facts, даёт correction и возвращает result в ведущий Case; no-account и equivalent fallback обязательны. |
| **Media Uploader** | До upload объясняет цель/minimization, предупреждает о третьих лицах и проходит `uploading → scanning → usable / needs correction / quarantined / rejected`; доступны preview/remove, keyboard- или human-assisted redaction, retry и равноценный text/inspection/human path. |
| **Correction Panel** | «Fix current case» меняет разрешённый факт/summary с provenance; «Improve employee» создаёт scoped draft, regression test, publish и rollback. |
| **Test & Release Panel** | По умолчанию simulation; реальное действие — только test account + явное включение + receipt. Promotion Demo → Check → Trial → Controlled Work → Work требует accessibility evidence; Critical/High failure блокирует только affected scope. |
| **Result Metric** | Показывает denominator/baseline/period/evidence; drill-down связывает outcome с Case, human correction и config version. |
| **Budget Meter** | Предупреждает до лимита и предлагает scoped action. При hard cap новые обращения переходят в configured human/callback/intake-only; активные Commitment, safety, claim и Recovery не обрываются, owner получает alert/receipt. |
| **Recovery Panel** | Создаёт один linked Recovery Case, отделяет acknowledged от resolved, назначает owner/next update и хранит reconciliation evidence. |
| **Domain Slot** | Валидирует domain fields/policies, но использует общие identity, authority, Confirmation, owner, action и recovery primitives. |

## State Patterns

| State family | Канонические значения | UX-переход/ограничение |
|---|---|---|
| Fit Status | `preliminary fit` · `fit with conditions` · `Staffy extension required` · `no fit now` | До credentials; каждый outcome имеет причину и следующий шаг. |
| Connection Status | `disconnected` · `connected awaiting verification` · `verified` · `verified with limitations` · `needs attention` | OAuth/SIP success не переводит в verified без capability test. |
| Employee Status | `configuring` · `blocked questions` · `ready for trial` · `in trial` · `working` · `working with limitations` · `paused` | Pause/limitation scoped; причина и fallback видимы. |
| Admission | `Demo` · `Connection Check` · `Trial` · `Controlled Work` · `Work` | Повышение только с evidence; critical error откатывает затронутый scope. |
| Conversation Mode | `sales` · `service/status` · `safety` · `claim` · `cancellation` · `privacy/no_contact` · `human_requested` | Дословный PRD enum; более приоритетный Mode блокирует несовместимые Actions, возврат в `sales` — только разрешённым переходом. |
| Response Ownership | `AI active` · `requested` · `assigned` · `acknowledged/human active` · `overdue` · `reassignment required` · `returned` · `failed` | Ровно один owner; assigned ещё не означает помощь; lease/next-update expiry требует backup/escalation. |
| Delivery Truth | `queued` · `API accepted` · `delivered` · `read/heard if proven` · `failed` | Неподдерживаемый receipt не имитируется. |
| Business Action | `prepared` · `awaiting Confirmation` · `executing` · `confirmed` · `failed` · `partial` · `unknown/reconciling` | Success только от Owning System; unknown не допускает blind retry. |
| Protected Link | `delivered` · `waiting` · `completed` · `expired` · `cancelled/revoked` | Resend/revoke не создаёт новый Case; expiry имеет безопасный retry. |
| Purpose-bound Consent Receipt | Для каждой цели: `unknown` · `accepted` · `declined` · `ambiguous` · `revoked`; поля `purpose, artifact/data class, recipient/provider, channel, policy/version, time, expiry/revocation` | Recording, STT/AI processing, media, transfer, outbound contact и marketing не разделяют один receipt. Тишина не consent; отказ/отзыв останавливает только затронутое использование и ведёт в равноценный путь. |
| Wait | `waiting media` · `waiting specialist` · `waiting supply` · `waiting reconciliation` | Всегда owner, причина, next update и expiry/fallback; без fake ETA. |
| Evidence/Freshness | `current` · `stale` · `conflicting` · `missing` · `confirmed by customer` | Stale/conflict блокирует только материально затронутый claim/action. |
| Correction/Release | `reported` · `case fixed` · `draft` · `testing` · `ready` · `published` · `rolled back` | Нет silent learning/cross-tenant transfer; production incident может локально pause. |
| Recovery Case | `opened` · `acknowledged` · `investigating` · `waiting` · `resolved` · `reopened` | Claim acknowledgement не равно resolution; исходный Case остаётся связанным. |
| Surface/System | `loading` · `empty` · `partial` · `offline` · `permission denied` · `provider outage` · `stale view` | Сохраняет введённое, сообщает что доступно, даёт retry/fallback без ложного success. |
| Media Safety | `uploading` · `scanning` · `usable` · `needs correction` · `quarantined` · `rejected` | `uploaded` не равно usable; поздний/unsafe файл не запускает Action и имеет удалить/заменить/text/human path. |
| Budget Degradation | `normal` · `warning` · `hard cap` · `safety/commitment exception` | Hard cap не бросает активное обязательство; configured degradation и owner receipt обязательны. |

### Surface × state contract

| Surface | Loading / empty | Offline / partial / stale | Permission / provider error | Retained state, announcement and action |
|---|---|---|---|---|
| Public fit | Skeleton сохраняет порядок будущих вопросов; empty означает «ещё не описана работа». | Локальный draft помечен и не считается рассчитанным fit outcome без серверной версии. | Ошибка проверки Connector не просит credentials и переводит результат в fit with conditions/extension. | Ответы сохраняются; объявляется изменившаяся секция; доступны retry, assisted path и выход без передачи доступа. |
| Today | Скелет повторяет будущие блоки; empty означает «нет решений», не «нет данных». | Последняя подтверждённая версия с time/source; spend/outcomes не обновляются оптимистично. | Локализованный Connector gate, не авария всего Tenant. | Focus остаётся; `polite` summary; retry, открыть source или safe fallback. |
| Employees list | Skeleton сохраняет будущие карточки; empty предлагает создать или продолжить первый Employee Brief. | Последние подтверждённые статусы видны с time/source; действия допуска и pause недоступны до refresh. | Недоступный сотрудник не раскрывает Case/PII и объясняет требуемую роль. | Фильтр и focus сохраняются; доступны открыть разрешённого сотрудника, запросить доступ или вернуться в Today. |
| Employee workspace | Секции загружаются независимо; неизвестная readiness не зелёная. | Draft и published version раздельны; material local/server conflict не auto-merge. | Недоступная секция объясняет требуемую роль/owner. | Brief/Launch Plan draft сохранён; compare versions, request access, resume. |
| Work | Empty = нет ожидающих решений; counts имеют timestamp. | Case остаётся read-only либо draft-only; external action disabled до refresh. | Потеря Inbox/assignment создаёт overdue/reassignment, не takeover success. | Текущий Response Owner объявлен; перейти в единственную writer surface или backup. |
| Integrations | До account test — awaiting verification. | Capability evidence показывает age; stale material capability fail-closed. | Auth/provider error сохраняет setup step без секрета. | Retry test, reconnect by credential owner, manual fallback/extension. |
| Results | Empty отделяет «нет трафика» от «нет измерения». | Последний verified period/baseline; provisional margin помечена. | Нет доступа к Case — aggregate остаётся без раскрытия PII. | Filter/state объявляются; запросить доступ или открыть разрешённый evidence. |
| Organization | Пустая роль/owner — blocker для зависимого route. | Последние сохранённые настройки read-only; изменения queued не «сохранены». | Permission denied объясняет владельца права без раскрытия секрета. | Ввод сохраняется; request access/cancel безопасны. |
| Protected Continuation | Skeleton краткий; expired/revoked не выглядит пустой формой. | Draft сохраняется; server/local material conflict требует compare и новое Confirmation. | Step-up/provider/upload error не уничтожает Case. | `polite` status, re-auth/resend/revoke, вернуться в chat/phone/human. |

## Accessibility Floor

- WCAG 2.2 AA — обязательный Evidence Gate: automated checks + ручные keyboard, screen reader, zoom/reflow/text spacing, forced colors, touch, reduced motion и voice/media alternatives. Critical/High failure блокирует affected admission scope и получает regression case.
- Полная клавиатурная навигация, логичный focus order, skip link, видимый `{colors.focus}`. Overlay contract: meaningful initial focus; trap и inert-фон только у modal; `Escape` у dismissible layer; возврат источнику/successor; немодальный слой не захватывает focus; sticky/keyboard не скрывает focused control.
- Touch target не меньше 44×44 px; все web surfaces reflow при ширине 320 CSS px без page-level horizontal scroll. Действительно двумерные данные имеют локальную именованную прокрутку и доступное linear/card представление.
- Status/risk/progress не зависят от цвета, hover, анимации или аудио; live updates объявляются без захвата focus.
- Native semantics first: landmarks/headings, name/role/state/value, table headers/card equivalent; один агрегированный `polite` status channel, `assertive` только для safety/необратимой ошибки; streaming transcript не объявляется токен за токеном.
- У форм явные labels, error summary связан с полями и сохраняет ввод; нет fixed-height text controls; WCAG Text Spacing и длинная русская строка не обрезают material content и действия.
- Reduced motion поддерживается. Каждый timeout/expiry имеет видимое и программно объявляемое предупреждение, достаточное время, доступное продление где безопасно, saved draft и re-auth/resume; expiry ссылки отменяет полномочие ссылки, но не Case.
- Голос не единственный путь: DTMF/text/human/callback; фото и drag-and-drop имеют файловую/текстовую альтернативу.
- AI disclosure, consent, critical readback, цена, сторона договора, payee и claim owner доступны в понятном тексте, не только юридической ссылкой.
- Step-up auth не требует одной когнитивной функции: OTP поддерживает paste/autofill/resend; password manager не блокируется; puzzle-only CAPTCHA запрещена; доступен безопасный human/callback alternative без потери Case/draft.
- Case хранит только заявленные пользователем communication preferences/accommodations (канал, темп, readback, no-photo/no-voice, доступное время), не диагноз; они редактируются и передаются каждому Response Owner.
- Во всех клиентских web- и native-chat поверхностях действие и команда «Позвать человека» называются одинаково и остаются в устойчивом доступном месте. После запроса показываются фактические owner/acknowledgement/next update и действие «Вернуться в разговор»; автоматический возврат ответа к ИИ запрещён до явного `return`.
- Machine IDs не являются primary copy: каждому state/action соответствует утверждённая русская label + consequence; иностранные аббревиатуры получают корректный `lang`/доступное произношение.
- Существенные условия открываются мышью и клавиатурой и остаются доступными. Контент, появляющийся при наведении или фокусе, можно закрыть, удержать курсором и прочитать без исчезновения. Портретная и альбомная ориентации не блокируются без существенной причины. Для перетаскивания, свайпа и жеста масштабирования предусмотрены эквиваленты с одним указателем и клавиатурой. Высокорисковое действие запускается при отпускании кнопки либо отдельном подтверждении; до запуска его можно отменить.

## Responsive & Platform

| Context | Поведение |
|---|---|
| Desktop ≥1200 | Навигация + рабочая область; context rail только для decision/supporting detail. Dense expert tables допустимы с keyboard support. |
| Tablet 768–1199 | Навигация/rail становятся drawers; primary decision остаётся в viewport, таблицы переходят в карточки или controlled scroll внутри области. |
| Mobile <768 | Одна колонка, один next action, summary до деталей; no horizontal page scroll; high-stakes CTA не прилипает поверх условий. То же reflow-поведение обязательно при 320 CSS px на всех web surfaces независимо от device label. |
| Protected continuation | Mobile-first и low-bandwidth: server-renderable core, возобновляемая загрузка, сохранение draft, точный delivery state. |
| Voice | UI не нужен для базового диалога; protected link используется для media/critical entry/comparison/Confirmation, а не для переноса всего разговора. |
| Native chat/Inbox | Composer следует фактическим Capability; Jivo/другой Inbox — один writer, Staffy control layer не отвечает параллельно. |
| Weak network/outage | Optimistic UI запрещён для Business Actions. Recovery сравнивает локальную и серверную версии, их timestamps, текущего owner и takeover state; поздняя загрузка имеет отдельный статус. Material fields не объединяются автоматически, а queued/local draft отличается от server/Owning System receipt. |

Admin/ops MVP — только light «Северный кедр». Dark mode остаётся открытым и не должен молча появиться в реализации. Отдельное native mobile app Staffy — не MVP.

## Inspiration & Anti-patterns

| Перенять как принцип | Граница |
|---|---|
| Inbox: единая история, assignment, acknowledgement | Не копировать конкретный competitor layout; не создавать второй composer. |
| CRM: decision-ready object и audit trail | Не превращать разговор в форму из десятков обязательных полей. |
| Release tools: diff, tests, publish, rollback | Не показывать prompt/code владельцу как основной способ настройки. |
| Consumer checkout: ясный review и delta | Не объединять consent, order, payment, marketing и data transfer одной кнопкой. |
| Incident tooling: owner, severity, next update, recovery | Не выставлять инфраструктурные логи клиенту или владельцу без progressive disclosure. |

Визуальные исследования: [направления](mockups/directions-4.html), [темы](mockups/color-themes-5.html), [onboarding Ирины](wireframes/flow-irina-onboarding-2026-09-13.excalidraw), [handoff и смена канала](wireframes/flow-cross-channel-handoff-2026-09-13.excalidraw), [автозапчасти](wireframes/flow-autoparts-a1-a7-2026-09-13.excalidraw), [Montagio target](wireframes/flow-montagio-m1-m7-2026-09-13.excalidraw). Они являются non-production references: spines имеют приоритет при конфликте. Текстовым эквивалентом диаграмм служат IA, Interaction Primitives и соответствующие UJ ниже. Схема Montagio отражает contractor target; несовместимые исторические Stage 0 узлы не переопределяют OQ-10.

### Mock coverage ledger

| IA surface | Coverage | Visual reference / implementation source |
|---|---|---|
| Today | Mocked | [Role-Adaptive Today владельца](mockups/key-today-owner.html). |
| Employee/readiness + Integrations evidence | Mocked | [Рабочее место сотрудника](mockups/key-employee-workspace.html); Integrations остаются встроенной секцией walking skeleton. |
| Case/control — автозапчасти | Mocked | [Exact-version ABCP review](mockups/key-autoparts-order-review.html). |
| Protected Continuation — Montagio | Mocked | [Mobile-first уточнение задачи](mockups/key-montagio-continuation.html); до PASS OQ-10 без публичного заказа/оплаты/чека/гарантии. |
| Public fit, Employees list, Results, Organization | Spine-only | Строятся из IA, Component Patterns, State Patterns и Surface × State; отдельный mock не требуется для walking skeleton. |
| Остальные Case/claim/recovery варианты | Spine-only | Строятся из UJ, Conversation Modes, Interaction Primitives и state contracts; иллюстрируются только после доказанного layout gap. |

## Key Flows

Каждый сценарий носит то же имя, что и в PRD. Здесь зафиксированы только UX-шаги, а не новые бизнес-требования. P1–P6 — последовательные Policy Outcomes Montagio; P7 — прерывающий Safety Interrupt.

### Pilot & Onboarding

| Journey / protagonist | Нумерованный flow | Climax | Failure / recovery |
|---|---|---|---|
| **UJ-P1 — Ирина проверяет соответствие Staffy до передачи доступов** | 1) описывает результат; 2) видит pack/connector/capability fit; 3) проверяет условия, людей и usage; 4) выбирает pilot/extension/no-fit. | Решение о pilot scope без credentials. | Unknown capability → `fit with conditions/extension`, владелец и проверка; не false-ready. |
| **UJ-P2 — Ирина запускает поддерживаемого ИИ-сотрудника без программиста** | 1) интервью; 2) подтверждает Employee Brief; 3) выполняет/делегирует Launch Plan; 4) подключает и проверяет; 5) simulation/evals; 6) Controlled Work; 7) Today/rollback. | Сквозной Action + acknowledged Handoff в испытании. | Async/provider blocker сохраняет прогресс и fallback; critical regression блокирует publish/откатывает scope. |

### Montagio

| Journey / protagonist | Нумерованный flow | Climax | Failure / recovery |
|---|---|---|---|
| **UJ-M1 — Анна заказывает замену смесителя через Montagio** | 1) AI disclosure+safety; 2) progressive intake/media-or-alternative; 3) P1/P2 и Confirmation #1; 4) supply/contractor readiness; 5) final Commitment; 6) work/acceptance/payment/receipt/claim states. | После OQ-10 PASS Анна подтверждает договор, оплату, чек и гарантию Montagio, а специалист остаётся субподрядчиком. | До OQ-10 public order/payment blocked; no supply → окно/wait/human/P-route без fake ETA. |
| **UJ-M2 — Анна покупает водонагреватель в партнёрском магазине** | 1) Staffy называет Montagio и источник; 2) Анна проверяет provenance/consent; 3) уточняется только compatibility gap; 4) route+Confirmation #1; 5) readiness; 6) final Commitment и ограниченный partner status. | Контекст магазина превращён в точный заказ без повторного intake и с правдивыми сторонами. | Consent disputed → `privacy/no_contact`; OQ-10 блокирует публичное contractor commitment; callback только по выбору/риску/пробелу. |
| **UJ-M3 — Ольга начинает на Avito и безопасно продолжает в защищённом контуре** | 1) ответ по объявлению; 2) scope/value comparison; 3) объяснение protected switch; 4) identity check; 5) correction+media/Confirmation; 6) результат в исходном Case. | Ольга продолжает один диалог без повтора и подтверждает точные данные. | Нет официальной capability/слабый match → assisted/blocked, no merge, другой канал/человек. |
| **UJ-M4 — Дмитрий сообщает об аварийной ситуации в кафе** | 1) P7 safety interrupt; 2) краткая approved инструкция; 3) объект/угроза; 4) owner/budget authority/payee; 5) acknowledged handoff; 6) заново разрешённый P4/P5/P6. | Уполномоченный человек принимает decision-ready emergency context. | Handoff not acknowledged/no supply → честный fallback; никаких SLA, выезда или цены без readiness. |
| **UJ-M5 — Михаил покупает инженерный осмотр отопления** | 1) цели/ограничения; 2) планы/media с protected path; 3) Staffy обозначает предел диагностики; 4) предлагает P4 как отдельный результат; 5) куратор проверяет; 6) уполномоченный подтверждает. | Михаил покупает обследование, а не ложную дистанционную «точную смету». | Недостаточно evidence/high risk → human gate; нельзя проектировать систему или фиксировать сложную цену. |
| **UJ-M6 — Валентина не может прислать фото или просит человека** | 1) сохраняются известные факты; 2) предлагается no-media route; 3) просьба человека включает `human_requested`; 4) requested→assigned→acknowledged; 5) человек продолжает Case. | Валентина не повторяет историю и знает фактического owner. | No acknowledgement → callback/reassignment/другой supported path с честным next update. |
| **UJ-M7 — Алексей обнаруживает дополнительную работу на объекте** | 1) stop affected work; 2) evidence+reason; 3) новая scope/price/material/time version; 4) Анна видит delta; 5) отдельное Confirmation; 6) resume либо safe close. | Новая работа начинается только после material-delta согласия. | Отказ/молчание → не выполнять; восстановить исходное состояние или human review. |
| **UJ-M8 — Анна возвращается с претензией после выполнения** | 1) найти Order; 2) `claim` и при угрозе `safety`; 3) linked Recovery Case; 4) собрать missing evidence; 5) human acknowledgement; 6) updates/resolution/reopen. | Анна получает принятого owner и право оспорить summary/result. | Staffy не определяет вину/refund; no acknowledgement → escalation/fallback, incident может локально pause scope. |

### Автозапчасти

| Journey / protagonist | Нумерованный flow | Climax | Failure / recovery |
|---|---|---|---|
| **UJ-A1 — Сергей подбирает тормозные колодки по телефону** | 1) AI disclosure; 2) intent/work items; 3) protected VIN+readback; 4) catalog applicability; 5) original/analogs; 6) Confirmation; 7) ABCP order `pending_manager_verification`. | Один подтверждённый Order и decision-ready задача менеджеру. | Conflict/unknown outcome → no order/success, human or reconciliation by operation ID. |
| **UJ-A2 — Мария подбирает деталь в Jivo и продолжает голосом** | 1) Jivo intake/media; 2) provenance; 3) согласованный звонок; 4) один Case/identity check; 5) selection+Confirmation; 6) order/handoff. | Канал меняется без повтора VIN и без второго заказа. | Media unsupported → protected link; after acknowledged Jivo remains sole writer; no parallel AI reply. |
| **UJ-A3 — Николай повторяет заказ по известному OEM/артикулу** | 1) OEM/previous item; 2) заново проверить vehicle/applicability; 3) обновить price/stock/ETA; 4) readback+Confirmation; 5) idempotent order; 6) manager gate по текущему class. | Точный повтор завершён по актуальным данным, не по памяти. | Stale/conflict → уточнение/human; старый заказ не считается evidence текущей применимости. |
| **UJ-A4 — Алексей описывает симптом вместо требуемой детали** | 1) распознать diagnostic intent; 2) собрать минимум vehicle/VIN/symptom; 3) объяснить предел; 4) передать менеджеру/approved diagnostic service. | Decision-ready handoff без заказа гипотетической детали. | Нельзя превращать LLM hypothesis в SKU; no human → callback/fallback. |
| **UJ-A5 — VIN, документ и каталог противоречат друг другу** | Дмитрий: 1) видит спорное поле; 2) protected correction; 3) readback; 4) повторная catalog check; 5) unresolved conflict handoff с обоими values/provenance/candidates. | Исправленное значение каталогически разрешает продолжение. | Конфликт не снят → заказ blocked; «самый вероятный» вариант запрещён. |
| **UJ-A6 — Елена просит специалиста или Staffy не может завершить операцию** | 1) stop auto flow; 2) собрать context/tool outcomes; 3) requested→assigned; 4) удерживать линию до acknowledgement; 5) человек отвечает либо callback. | Фактический acknowledged takeover с полным контекстом. | Transfer trigger/no-answer не success; fallback сохраняет Case и один Response Owner. |
| **UJ-A7 — Антон проверяет заказ Staffy и находит ошибку** | 1) принимает exact-version task; 2) проверяет applicability evidence; 3a) approve и вручную продолжает supplier flow; 3b) block, звонок, корректная замена; 4) новая delta Confirmation; 5) повторная verification. | Human approval receipt либо исправленная подтверждённая версия. | Staffy не имеет supplier-send Action в пилоте; supplier status только из Owning System, unknown → reconcile. |
| **UJ-A8 — Олег повышает или откатывает автономность класса** | 1) выбирает Autonomy Class; 2) видит cases/corrections/critical errors/human minutes/margin; 3) сверяет evidence window; 4) повышает scope; 5) мониторит; 6) локально откатывает. | Осознанное повышение одного доказанного класса. | Critical error/threshold unknown → pause/rollback только затронутого класса; audit сохраняется. |
| **UJ-A9 — Игорь обращается по статусу, возврату или рекламации** | 1) identity check; 2) найти исходный Order; 3) прочитать актуальный status; 4) сообщить только allowed fact; 5) claim/return/unknown → linked handoff/recovery. | Ответ связан с тем же Order и подтверждён Owning System. | ABCP→1С mismatch/irreversible dispute → no new order/no false success, human acknowledgement и reconciliation. |
