---
title: 'Story 1.6 — Безопасная настройка поведения ИИ-сотрудника'
type: 'feature'
created: '2026-09-27'
status: 'done'
route: 'dispatch'
review_loop_iteration: 1
baseline_commit: '09dc9642639b8324fa579959c41f36855d71f054'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-staffy-2026-09-13/GATE-REGISTRY.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** После выбора Domain Pack настройщик не может задать проверяемое поведение ИИ-сотрудника без свободного системного промпта; маркетинговый текст не должен расширять полномочия.

**Approach:** Ввести tenant-scoped версионированный черновик с управляемыми полями, проверкой старших правил, доказательными Allowed Claims и синтетическим preview; без публикации и реального канала.

## Boundaries & Constraints

**Always:** Управляемые поля: роль/назначение, разрешённое AI-раскрытие, тон, режимы, Allowed Claims, запретные формулировки, Confirmation/Handoff, ограниченные примеры. У каждого есть назначение, scope и безопасный default. `safety`, `claim`, `cancellation`, `privacy/no_contact`, `human_requested` выше `sales`; неизвестное блокируется. Tenant лишь сужает platform/Domain Pack; ошибка называет старшее правило. Claim хранит владельца, scope, каналы/режимы, evidence reference, версию и срок пересмотра; без свежего evidence он `blocked`. **Решение владельца от 2026-09-27:** в этой истории все claims остаются `blocked` даже при свежем источнике; `eligible` запрещён до отдельной проверяемой привязки конкретного утверждения к доказательству. Цена/наличие/статус требуют Owning System. Отказ прекращает убеждение; AI-раскрытие, human-only решения и Confirmation неизменны. Handoff без человека `blocked`. Сохранять CAS, actor/time/change summary, audit/outbox, Tenant/RLS. Свободный текст ограничен и недоверен; UI доступен при 320 px и с клавиатуры.

**Never:** Текст не меняет system prompt, policy precedence, tools, destinations, Tenant или права. Черновик не разрешает Business Action, публикацию или реальный канал. Нет tenant-specific кода.

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Результат | Защита |
|---|---|---|---|
| Настройка | Draft с Pack, допустимые поля | Новая draft-версия и видимые ограничения | CAS, audit |
| Конфликт политики | Отключение раскрытия, ослабление Confirmation/старшего режима | Отказ с названием правила | Fail-closed |
| Claim | Любой evidence, включая свежий и совпадающий ID/версию | `blocked`, не считается допустимым утверждением до отдельной доказательной привязки | Без auto-truth |
| Продажа/preview | Давление либо синтетический сценарий | Запрет примера; mode, claims, причина блока/handoff | Нет внешнего действия |
| Изоляция/гонка | Чужой Tenant/старая версия | 404/конфликт | RLS/CAS |

</frozen-after-approval>

## Code Map

- `internal/employee/domain/model.go:96,117`, `api/api.go`, `app/service.go:30,84,141` — Draft/LaunchPlan, verified scope и конфигурационное полномочие; release остаётся blocked.
- `internal/employee/domain/pack.go`, `packs/catalog.go` — фиксированные `Policies`; `PolicyRef` не является исполняемой проверкой.
- `internal/employee/app/behavior.go`, `store/behavior.go` — текущая проверка свежести ложно ставит `eligible`; заменить на неизменный blocked до отдельной истории, не полагаться на текст источника или `preview` fixture.
- `internal/employee/store/postgres.go:21,160,201`, `migrations/employee/00001_employee_draft.sql` — scoped CAS, audit/outbox, FORCE RLS; миграция после `00004`.
- `internal/transport/http/{server,employee}.go`, `contracts/openapi/staffy.yaml`, `web/templates/employee.html` — API, форма и preview.
- `tests/integration/`, `internal/transport/http/employee_test.go`, `tests/journeys/` — RLS/CAS и доступность.

## Tasks & Acceptance

**Execution:**
- [x] `internal/employee/{domain,api,app,port,store}/` — исключить `eligible` без доказательной привязки, проверять совместимость Pack и инварианты старшинства; явно блокировать неготовый handoff и mode.
- [x] `migrations/employee/` — tenant-scoped история, audit/outbox и безопасный rollback.
- [x] `internal/transport/http/`, `contracts/openapi/staffy.yaml`, `web/` — дать исправлять/удалять claims, не показывать заблокированный mode как доступный, описать ответы API схемами.
- [x] `tests/` — доказать `blocked` без evidence и при свежих metadata (без запроса к источнику, поэтому устаревший/чужой/конфликтный также не могут повысить статус); проверить Pack replacement, отказ, human handoff, CAS/RLS и доступность.

**Acceptance Criteria:**
- Given Employee с Pack, when сохраняют настройку, then новая неизменяемая draft-версия содержит actor/time/change summary; release не меняется.
- Given настройка противоречит старшему правилу, when её сохраняют или запускают preview, then она не применяется и показана конкретная причина.
- Given claim без evidence или human fallback, when открывают preview, then виден блокер без разрешения на публикацию или Business Action.
- Given claim со свежим источником, when сохраняют или открывают preview, then он всё равно `blocked`, потому что привязка утверждения к доказательству ещё не существует.
- Given повторный/устаревший запрос или другой Tenant, when читают и меняют поведение, then нет перезаписи либо утечки.

## Implementation Notes

- После решения владельца путь `ClaimEvidence` удалён из порта и PostgreSQL store: свежесть источника больше не способна открыть claim. На чтении, preview и сохранении статус принудительно `blocked`; ранее сохранённый `eligible` также блокируется при чтении.
- Синтетический preview возвращает `blocked` для claims, sales, неподтверждённого handoff, отказа, no-contact, safety и Business Action. Реальные действия и публикация по-прежнему недоступны.
- Редактор сохраняет тон/режимы, позволяет исправить или удалить claim; история версий остаётся неизменяемой. Контракт API описывает успешные ответы.

## Spec Change Log

- 2026-09-27: review выявил ложный `eligible` от произвольного свежего источника; по явному решению владельца frozen intent уточнён: все claims остаются `blocked` до отдельной проверяемой привязки. Дополнены задачи для Pack compatibility, корректного preview/handoff, редактирования claims и контрактов API. Избегаем прежней модели «свежесть источника = истинность claim». KEEP: Tenant/RLS, CAS, неизменяемые версии, audit/outbox, синтетический preview, UI 320 px и клавиатура.

## Review Triage Log

- Blind 1 — `high`, `intent_gap`: `ClaimEvidence` проверяет лишь состояние источника, а не поддержку конкретного текста claim; произвольный свежий источник делает чужое утверждение `eligible`. Нужен согласованный способ доказательной привязки claim к содержимому или консервативный запрет eligibility.
- Blind 2 — `medium`, `bad_spec`: `prepareBehavior` не сверяет конфигурацию с политиками закреплённого Pack; `PolicyRef` сейчас только декларативен. Черновик может выглядеть разрешённым при несовместимости с Pack, хотя публикация пока блокируется.
- Blind 3 — `medium`, `bad_spec`: после перепривязки Pack редактор берёт последнюю конфигурацию без сверки её `PackID/PackVersion` с текущим закреплением; настройщик может повторно сохранить несовместимое поведение.
- Blind 4 — `medium`, `bad_spec`: проверка evidence и сохранение происходят в разных транзакциях; отзыв источника между ними оставляет ответ сохранения с устаревшим `eligible` (чтение позднее блокирует его).
- Blind 5 — `false`: `ReadBehavior` намеренно не повышает сохранённый `blocked` до `eligible` только потому, что источник стал доступен: автоматическое получение нового разрешения противоречит fail-closed и требованию новой версии/проверки.
- Blind 6 — `medium`, `bad_spec`: HTML-редактор только добавляет claims, хотя API может заменить массив; ошибочный claim нельзя удалить через пользовательский сценарий.
- Blind 7 — `medium`, `bad_spec`: общий preview берёт первый режим даже при заблокированных claims/handoff, из-за чего заголовок `sales` может выглядеть разрешением несмотря на блокеры.
- Blind 8 — `medium`, `bad_spec`: safety/human_requested показывают `mode=handoff` при неподтверждённом человеке и одновременно `handoff=blocked`; состояние неоднозначно для потребителя preview.
- Blind 9 — `high`, `intent_gap`: короткий список опасных фраз не доказывает соблюдение старшинства политик; роль/назначение вообще не проверяются на аналогичные инструкции. Нужно определить безопасную семантику свободного текста без обещания, что фильтр обнаружит все перефразировки.
- Blind 10 — `medium`, `bad_spec`: новые успешные ответы OpenAPI не имеют schema, поэтому клиенты не получают проверяемый контракт чтения версий и preview.
- Gap 1 — `medium`, `patch`: нет теста перехода ранее допустимого claim в `blocked` на чтении после отзыва/истечения evidence; при удалении соответствующей ветки текущие тесты проходят.
- Gap other — `false`: повышение старого `blocked` до `eligible` без новой версии намеренно исключено fail-closed; новое разрешение требует явной переоценки.
- Edge 1 — `high`, `intent_gap`: подтвердил Blind 1: SQL сверяет ID/версию/статус/срок, но не смысл утверждения.
- Edge 2 — `false`: тот же сценарий, что Blind 5; отсутствие автоматического повышения сохраняет запрет по умолчанию.
- Edge 3 — `medium`, `bad_spec`: подтвердил Blind 4: `prepareBehavior` завершается до новой транзакции `SaveBehavior`.

### Повторное ревью после решения владельца

- Blind 1 — `false`: закрепление Pack и сохранение поведения изменяют одну строку `employee.drafts` с `version=version+1`; CAS в `SaveBehavior` отвергает сохранение после конкурентной перепривязки Pack, поэтому непроверенный Pack не может попасть в сохранённую версию.
- Blind 2 — `false`: API-клиент может подать новые настройки под текущим Pack; он не меняет старую неизменяемую версию. Поля поведения не зависят от старых Pack variants, а новые правила всё равно проверяются до нового черновика.
- Blind 3 — `low`, rejected: текущие Pack policies пока декларативны; известные правила ограничены платформенными проверками, а черновик не даёт runtime или публикации. Полный семантический policy engine здесь добавил бы самостоятельную систему без потребителя.
- Blind 4 — `low`, rejected: список фраз не является единственным периметром; свободный текст хранится как недоверенные данные, не попадает в system prompt и не запускает runtime. Семантическую гарантию для всех перефразировок детерминированный фильтр дать не может.
- Blind 5 — `low`, rejected: эвристика Owning System пропускает синонимы в тексте черновика, но любой claim остаётся `blocked`, и реальное обещание невозможно. Расширение словаря без проверяемой системы-владельца не создаст защиту.
- Blind 6 — `false`: утверждённая I/O-матрица определяет синтетический preview как mode, claims и причины блока/handoff без внешнего действия; моделирование реального диалога не входит в эту историю.
- Blind 7 — `low`, rejected: текст/владелец/scope исправляются в форме; остальные поля можно заменить через удаление и добавление claim в следующей неизменяемой версии. Дополнительные поля не нужны для безопасного исправления.
- Blind 8 — `medium`, `patch`: прежний обычный POST заменял страницу ошибкой и мог потерять ввод. Теперь fetch показывает problem в live-region, оставляет форму и позволяет повторить сохранение; браузерный тест проверяет ошибку и успешный повтор.
- Blind 9 — `low`, rejected: история пока пилотная и ограничена одним Employee; добавление пагинации и нового контракта до измеримой нагрузки усложнит историю. При росте числа версий потребуется отдельный лимит и пагинация.
- Blind 10 — `low`, `patch`: повторные каналы/режимы claim проходили проверку; валидатор теперь отклоняет повторы, добавлен unit-тест.
- Edge 1 — `false`: в текущем composition root `Service` всегда получает `*store.Postgres`, реализующий `BehaviorRepository`; тестовые репозитории вызывают behavior только если реализуют порт. Не показан достижимый сценарий пользовательского panic.
- Gap 1 — `medium`, `patch`: ранее сохранённый `eligible` не был проверен тестом на downgrade. Добавлен тест, который подаёт такой статус из репозитория и ожидает `blocked` с причиной.

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.
