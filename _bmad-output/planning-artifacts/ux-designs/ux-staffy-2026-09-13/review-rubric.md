# Spine Pair Review — Staffy

## Overall verdict

Пара spine-документов полно охватывает продукт на уровне маршрутов и компонентов: все 19 исходных UJ присутствуют под точными именами, а 26 именованных компонентов согласованы между файлами. Однако как исполнимый downstream-контракт пара пока **broken**: токены не заданы в обязательном machine-readable frontmatter, канонический `Conversation Mode` изменён, а state coverage и подтверждённые key-screen references ещё не доведены до проверяемого уровня.

## 1. Flow coverage — adequate

Сопоставлены все journey-заголовки PRD с разделом `Key Flows`: 19 из 19, без пропусков, добавлений и переименований; у каждого маршрута есть нумерованные шаги, climax и failure/recovery. Проверены также ссылки на четыре flow-визуала.

### Findings

- **[medium]** Единственный маршрут без названного протагониста в самом UX spine — `UJ-A5 — VIN, документ и каталог противоречат друг другу`; в исходном PRD это Дмитрий (`EXPERIENCE.md:253`). *Fix:* сохранить исходное имя UJ дословно и добавить рядом явную помету протагониста, например `Протагонист: Дмитрий`.

## 2. Token completeness — broken

Проверены обязательные токен-группы, определения значений и ссылки из EXPERIENCE/компонентов. Цвета, типографика, spacing, radius, elevation и motion описаны в body, но не образуют требуемый token contract.

### Findings

- **[critical]** Frontmatter `DESIGN.md` содержит только метаданные и `sources`; отсутствуют обязательные `name`, `description`, `colors`, `typography`, `rounded`, `spacing`, `components`, а значения токенов находятся лишь в prose-таблицах (`DESIGN.md:1`, `DESIGN.md:41`). Ни DESIGN-компоненты, ни EXPERIENCE не используют обязательные ссылки вида `{colors.focus}` / `{spacing...}`; встречается только текстовое ``colors.focus`` (`EXPERIENCE.md:199`). *Fix:* перенести канонические токены и component map в YAML frontmatter по spec, затем заменить визуальные литералы/имена на валидные `{path.to.token}` references и проверить, что каждая ссылка разрешается.
- **[medium]** Для семантических цветов не зафиксированы foreground/container-пары и границы применения; например `colors.warning #B56617` на белом даёт около 4.31:1 и не проходит AA для обычного текста (`DESIGN.md:57`). *Fix:* определить `on-warning`/container tokens либо явно ограничить warning ролью крупного текста/иконки/обводки и записать contrast matrix для всех load-bearing пар.

## 3. Component coverage — strong

Сверены именованные component inventories в `DESIGN.md` и `EXPERIENCE.md`: 26 из 26 названий совпадают, у каждого есть визуальная анатомия и поведенческий контракт; непарных именованных компонентов не найдено.

### Findings

- Материальных расхождений не найдено.

## 4. State coverage — thin

Проверены IA-поверхности и 15 state families, включая authority, delivery truth, consent, recovery и degraded states. Высокорисковые состояния перечислены, но не сведены в исполнимые surface-level treatments.

### Findings

- **[high]** Универсальная строка `Surface/System` перечисляет loading, empty, partial, offline, permission denied, provider outage и stale view, но не определяет для каждой IA-поверхности конкретные retained data/input, сообщение, CTA/fallback, focus/announcement и допустимые действия (`EXPERIENCE.md:84`, `EXPERIENCE.md:156`). *Fix:* добавить компактную surface × state матрицу как минимум для Today, Employee, Work, Integrations, Results, Organization и Protected Continuation; явно покрыть cold-load, empty, error, offline, permission-denied и recovery.

## 5. Visual reference coverage — thin

Все семь существующих `.working`-визуалов доступны по inline-ссылкам; пять Excalidraw-файлов валидны, HTML-референсы автономны. Проверена трассировка flow-групп и заявленная политика spine-wins-on-conflict.

### Findings

- **[high]** Четыре подтверждённых key screens из memlog ещё не представлены mock-файлами; `mockups/` и `wireframes/` отсутствуют, а все keepers остаются в `.working`. EXPERIENCE прямо признаёт отсутствие key-screen файлов (`EXPERIENCE.md:231`). *Fix:* создать и проверить Role-adaptive Today, Employee Workspace, ABCP Order Review и Montagio Protected Continuation; промотировать утверждённые визуалы в `mockups/`/`wireframes/`, добавить inline-ссылки и coverage ledger «mocked / spine-only» по всем IA-поверхностям.

## 6. Bloat & overspecification — adequate

Документы в основном остаются decision-dense и не пересказывают целиком продуктовые источники; открытые вопросы и 19 journeys занимают место пропорционально риску. Найдено одно доменное правило, ошибочно поднятое до универсального инварианта.

### Findings

- **[high]** `Два смысловых обязательства` и primitive `Two Confirmations` сформулированы как общеплатформенные, хотя двухступенчатая семантика относится к целевому сценарию Montagio, а общий контракт платформы требует versioned Confirmation на материальное действие (`EXPERIENCE.md:43`, `EXPERIENCE.md:176`). *Fix:* оставить в foundation универсальное правило versioned/material Confirmation, а preliminary+final Confirmation перенести в Domain Pack Montagio и применяющие его UJ.

## 7. Inheritance discipline — broken

Проверены 16 frontmatter sources, journey names, glossary-sensitive enums, open gates и paired names. Все source paths разрешаются, но один authority-bearing enum унаследован не дословно.

### Findings

- **[high]** Канонический PRD enum `Conversation Mode` изменён: `sales` и `service/status` слиты в `service/sales`, `cancellation` исчез, а `wait` и `recovery/unknown` добавлены как modes (`EXPERIENCE.md:115`, `EXPERIENCE.md:164`). Это меняет priority/authority semantics. *Fix:* восстановить канонические значения дословно; `wait` и `recovery/unknown` оформить как ортогональные operational/recovery states либо сначала изменить источник истины.
- **[low]** `.memlog.md` фиксирует OQ-17 закрытым на уровне UX-контракта, но EXPERIENCE без оговорки утверждает, что все OQ открыты (`.memlog.md:20`, `EXPERIENCE.md:60`, `EXPERIENCE.md:80`). *Fix:* пометить OQ-17 как `UX contract closed / architecture implementation open`, чтобы downstream-команды не переоткрывали уже принятое решение.

## 8. Shape fit — strong

`DESIGN.md` следует каноническому порядку восьми body-разделов; `EXPERIENCE.md` содержит все обязательные разделы и уместные triggered sections для voice, safety, multimodal и responsive поведения. Дополнительный раздел decision gates оправдан риском и используется downstream; дефект frontmatter учтён в разделе 2.

### Findings

- Материальных расхождений не найдено.

## Mechanical notes

- Все 16 путей `sources` в обоих frontmatter разрешаются и существуют; списки источников совпадают.
- Exact UJ coverage: 19/19, duplicates 0, extras 0, missing 0.
- Paired component names: 26/26, mismatches 0.
- Token definitions in required YAML shape: 0; token references вида `{path.to.token}`: 0.
- Все семь inline visual references существуют; пять `.excalidraw` проходят JSON parsing. `imports/` пуст; `mockups/` и `wireframes/` отсутствуют.
- Mermaid-блоков нет, поэтому синтаксических ошибок Mermaid не обнаружено.
- Severity totals: **critical 1 · high 4 · medium 2 · low 1**.
