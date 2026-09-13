---
title: "Staffy — визуальная система"
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

# Staffy — визуальная система

Этот spine задаёт визуальный контракт для web-админки, операторского слоя и защищённого клиентского продолжения. Он не заменяет PRD и не фиксирует ещё не принятые юридические, канальные или архитектурные решения. При конфликте с `.working`-эскизами этот документ и `EXPERIENCE.md` имеют приоритет; эскизы остаются доказательством направления, а не самостоятельной спецификацией.

## Brand & Style

| Аспект | Правило |
|---|---|
| Образ | Спокойная, компетентная рабочая среда: «сотрудник под контролем», а не робот, конструктор prompt’ов или центр мониторинга. |
| Композиция | Основа — calm trustworthy operations; карточки сотрудников и людей — теплее; один рекомендуемый следующий шаг — направляющий. Плотный command-center режим допустим только в экспертных операционных представлениях. |
| Иерархия | Сначала бизнес-результат, риск и действие; затем статус и причина; технические IDs, trace и версии — по раскрытию. |
| Идентичность | В продуктовой админке бренд Staffy. В клиентском диалоге ИИ представляет Tenant и явно называет AI/цифровую роль. Защищённая страница использует ограниченные токены Tenant и ненавязчивую подпись «Работает на Staffy». |
| Изображения | В MVP не нужны роботы, гуманоиды, 3D-аватары и stock-photo call center. Допустимы реальные логотипы Tenant/коннекторов только с разрешением и нейтральные функциональные пиктограммы. |
| Визуальные референсы | Композиции: [4 направления](.working/directions-4.html). Палитры и выбранный «Северный кедр»: [5 цветовых тем](.working/color-themes-5.html). |

## Colors

### Подтверждённая light-тема «Северный кедр»

Эти semantic tokens обязательны для admin/ops MVP. Dark mode остаётся открытым решением: компоненты должны быть token-ready, но тёмные значения не придумываются. Защищённое продолжение использует только проверенный поднабор Tenant-токенов и не наследует dark mode сайта.

| Token | Hex | Назначение |
|---|---:|---|
| `colors.canvas` | `#F3F5F2` | Фон приложения и спокойные разделители областей. |
| `colors.surface` | `#FFFFFF` | Карточки, панели, формы, overlays. |
| `colors.text` | `#1E2925` | Основной текст и ключевые числа. |
| `colors.muted` | `#66736D` | Вторичный текст; не использовать для обязательного мелкого текста без contrast check. |
| `colors.border` | `#D7DED9` | Границы, таблицы, неактивные разделители. |
| `colors.primary` | `#315E4C` | Единственное доминирующее действие, активная навигация. |
| `colors.on-primary` | `#FFFFFF` | Текст/иконка на primary. |
| `colors.success` | `#247A52` | Только подтверждённый успешный бизнес-результат. |
| `colors.warning` | `#B56617` | Ограничение, ожидание, риск без немедленной остановки. |
| `colors.danger` | `#B4424B` | Блокировка, ошибка, safety/claim interrupt, разрушительное действие. |
| `colors.info` | `#356C9B` | Нейтральная справка, ожидание проверки, provenance. |
| `colors.focus` | `#7C4DFF` | Видимый keyboard focus; не используется как брендовый акцент. |

### Семантика

- Цвет всегда дублируется иконкой, текстовой меткой и, где важно, следствием/доступным fallback.
- `success` появляется только после подтверждения Owning System или требуемого человека; API acceptance, отправка уведомления и модельная уверенность не зелёные.
- Статусы fit, подключения, сотрудника, admission и действия не смешиваются одной шкалой.
- Доменный акцент Montagio/автозапчастей вторичен и не меняет семантику safety, warning или success.

## Typography

| Token | Значение | Применение |
|---|---|---|
| `type.family` | `system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif` | Единая современная Cyrillic-safe гарнитура до отдельного решения о бренд-шрифте. |
| `type.display` | 32/40, 650 | Название главного результата страницы; desktop. |
| `type.h1` | 28/36, 650 | Заголовок раздела. |
| `type.h2` | 22/30, 650 | Заголовок группы/карточки. |
| `type.h3` | 17/24, 650 | Локальный заголовок. |
| `type.body` | 15/22, 400 | Основной интерфейсный текст. |
| `type.small` | 13/18, 400–600 | Статусы, подписи и метаданные; не ниже 12 px. |
| `type.metric` | 24/30, 650, tabular nums | Деньги, SLA, конверсия, количество результатов. |
| `type.mono` | `ui-monospace, SFMono-Regular, Consolas, monospace` | Только VIN/OEM, IDs, версии и payload preview; не для обычного UI. |

Текст выравнивается влево. Заголовки короткие и спокойные; all-caps запрещён кроме микрометок до 12 символов. Числа денег/времени используют табличные цифры; валюты и единицы нельзя отделять от значения переносом.

## Layout & Spacing

| Token/режим | Значение | Правило |
|---|---:|---|
| `space.1` | 4 px | Внутри микрометок. |
| `space.2` | 8 px | Иконка–текст, связанные подписи. |
| `space.3` | 12 px | Компактные строки. |
| `space.4` | 16 px | Базовый внутренний отступ карточки/mobile gutter. |
| `space.5` | 24 px | Группы внутри карточки. |
| `space.6` | 32 px | Разделы desktop/tablet. |
| `space.7` | 48 px | Крупные смысловые секции. |
| `layout.max` | 1440 px | Максимальная ширина admin/ops canvas. |
| `layout.reading` | 720 px | Формы, brief, confirmations, protected continuation. |

- Базовая сетка — 8 px с допустимым шагом 4 px для компактных меток.
- Owner/configurer: medium density; expert operations: dense rows с тем же минимумом touch target; protected continuation: single-column и один главный следующий шаг.
- Desktop: навигация + main content + опциональный context rail. Tablet: rail становится drawer. Mobile: одна колонка; sticky action только если не скрывает условия и состояние.
- Технические детали не занимают постоянную колонку у владельца. Пустое пространство используется для иерархии, не для декоративной «премиальности».

## Elevation & Depth

| Token | Значение | Применение |
|---|---|---|
| `elevation.0` | none | Canvas и вложенные секции. |
| `elevation.1` | `0 1px 2px rgba(30,41,37,.06)` | Рабочая карточка над canvas. |
| `elevation.2` | `0 8px 24px rgba(30,41,37,.10)` | Drawer, popover, sticky decision panel. |
| `elevation.3` | `0 18px 48px rgba(30,41,37,.16)` | Modal только для необратимого/высокорискового решения. |

Границы и смена surface важнее теней. Вложенные карточки не образуют «слоёный пирог»: глубина максимум два уровня до overlay. Статус, риск и кликабельность нельзя передавать одной тенью.

## Shapes

| Token | Значение | Применение |
|---|---:|---|
| `radius.sm` | 6 px | Поля, chips, компактные controls. |
| `radius.md` | 10 px | Карточки, таблицы, панели. |
| `radius.lg` | 14 px | Крупный launch/confirmation panel. |
| `radius.pill` | 999 px | Только короткие status chips; не для всех кнопок. |
| `stroke.icon` | 1.75 px | Простые функциональные иконки 18–20 px. |
| `control.min` | 44×44 px | Touch target на всех form factors. |

### Motion

| Token | Значение | Применение |
|---|---:|---|
| `motion.fast` | 120 ms ease-out | Hover/focus/локальная смена control. |
| `motion.base` | 180 ms ease-out | Drawer, accordion, state transition. |
| `motion.slow` | 240 ms ease-in-out | Только крупная перестройка layout. |

Motion объясняет изменение состояния, владельца или расположения; декоративные loops, bounce и confetti запрещены. При `prefers-reduced-motion` остаются мгновенная смена и текстовое подтверждение.

## Components

Имена ниже каноничны и должны дословно совпадать с поведенческими компонентами в `EXPERIENCE.md`.

| Component | Визуальная строка |
|---|---|
| **App Shell** | Светлый canvas, спокойная навигация по роли, breadcrumb только глубже первого уровня, utility actions справа; мобильная навигация не дублирует primary action. |
| **Role-Adaptive Today** | Один крупный блок «что требует решения», затем сотрудники, результаты и расходы; порядок меняется по роли, но сетка и названия устойчивы. |
| **Recommended Action Card** | Выделенная `surface`-карточка с одной primary CTA, причиной, владельцем, сроком и безопасным вторичным путём. |
| **AI Employee Card** | Человечная рабочая карточка: имя/роль, домен, Employee Status, каналы, текущий результат и следующее действие; без аватара-робота. |
| **Status Badge** | Иконка + короткая метка + semantic color; подробное следствие рядом или по раскрытию, не в tooltip-only. |
| **Integration Card** | Логотип/тип, Tenant account, Connection Status, проверенные Capabilities, ограничение, последняя проверка и следующая CTA. |
| **Capability State** | Строка capability с `verified/limited/unavailable/unknown`, evidence time и fallback; unknown визуально не похож на enabled. |
| **Employee Brief** | Одноколоночное подтверждаемое резюме результата, каналов, действий, запретов и человека; редактирование секциями. |
| **Launch Plan** | Асинхронный список этапов с owner, зависимостью, due/blocked reason и одним recommended next step; не линейный progress bar. |
| **Delegated Task** | Имя ответственного, конкретный результат, входные данные без секрета, срок и состояние acknowledgement. |
| **Knowledge Source Row** | Источник, тип факта, freshness, provenance, conflict/blocking marker и последнее подтверждение. |
| **Conversation Timeline** | Единый Case; канал, отправитель, фактически доставленная/услышанная часть, delivery truth, action/handoff receipts и раскрываемый trace. |
| **Work Control Card** | Decision-ready summary, риск, обещанное клиенту, безопасное действие, SLA и ссылка в единственную reply surface. |
| **Response Owner Control** | Явное имя/роль owner, состояния передачи и одна активная поверхность ответа; takeover/return разнесены. |
| **Human Handoff Panel** | Причина, срочность, summary, подтверждённые/спорные факты, следующий ход и `requested → assigned → acknowledged`; failure заметен. |
| **Commitment Review** | Полный scope доступен, material delta выделена; actor, предмет, цена/срок, получатель, side effect и версия перед Confirmation. |
| **Action Preview** | «Что изменится, где и для кого», exact payload в человекочитаемом виде, Owning System, permission и отдельная CTA. |
| **Action Receipt** | `prepared/confirming/executing/confirmed/failed/unknown/partial`; operation ID, время, источник истины и reconciliation next step. |
| **Protected Continuation** | Tenant-first single-column page: цель, перенесённые факты, expiry/revoke, исправление, step-up по риску и равный альтернативный путь. |
| **Media Uploader** | Нужный ракурс/тип, privacy warning, preview/remove/redact/retry, progress и «продолжить без файла». |
| **Correction Panel** | Два визуально разделённых пути: «исправить этот случай» и «улучшить сотрудника»; before/after, scope, eval, publish/rollback. |
| **Test & Release Panel** | Версия, admission stage, сценарии normal/edge/failure, simulation/live distinction, блокеры, evidence и rollback. |
| **Result Metric** | Название бизнес-исхода, значение, denominator, период, baseline и confidence/evidence; vanity metrics вторичны. |
| **Budget Meter** | Spend, прогноз, лимит, период, включённые компоненты и одна безопасная реакция; warning до жёсткого stop. |
| **Recovery Panel** | Что случилось, что видит клиент, owner, подтверждённое/unknown, следующее обновление и безопасные reconcile/handoff actions. |
| **Domain Slot** | Визуально встроенная секция сотрудника с доменными полями/действиями; сохраняет общие tokens, states, Confirmation и audit. |

## Do's and Don'ts

| Делать | Не делать |
|---|---|
| Показывать один рекомендуемый следующий шаг и рядом последствия/альтернативу. | Превращать каждую страницу в wizard или скрывать асинхронные provider gates за progress bar. |
| Различать Fit Status, Connection Status, Employee Status, admission и business-action outcome. | Использовать один «готов/не готов» или общий зелёный success. |
| Называть AI-роль, Tenant, Response Owner, сторону действия, payee/гарантию там, где это материально. | Имитировать человека, «нашего мастера», выполненный transfer или неподтверждённый договор/платёж. |
| Показывать material delta перед новым Confirmation. | Просить «подтвердить всё ещё раз» без объяснения изменения или считать молчание согласием. |
| Оставлять путь без фото, записи, сложного интерфейса и поддерживать keyboard/zoom/reduced motion. | Делать media, голос или drag-and-drop единственным путём. |
| Показывать limitation, stale/unknown и ручной fallback на месте. | Прятать ограничение в tooltip, логе или странице статуса провайдера. |
| Использовать один Response Owner и одну writable surface. | Давать боту и человеку параллельные composer’ы для одной identity/conversation. |
| Локализовать красный статус и действие к затронутому классу/каналу. | Красить весь Tenant в аварийный цвет из-за одного ограниченного коннектора. |
| Сохранять спокойную геометрию и ограниченную глубину. | Использовать cyberpunk, AI gradients, стеклянные панели, excessive pills, confetti или «игрушечные» карточки. |
| Считать `.working`-визуалы референсами, а этот spine — правилом. | Выдумывать отсутствующие key-screen файлы или переносить устаревшую Stage 0 схему поверх принятой подрядной модели Montagio. |
