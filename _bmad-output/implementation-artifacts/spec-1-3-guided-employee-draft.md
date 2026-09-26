---
title: 'Story 1.3 — Создание черновика ИИ-сотрудника через guided setup'
type: 'feature'
created: '2026-09-26'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'ef122d22e8651a5b924aee93d11485ecc77f649f'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** После создания Tenant и настройки людей Staffy ещё не даёт Owner/Configurator создать ИИ-сотрудника. Пустое состояние Today не ведёт к понятной настройке, а запуск без обязательных зависимостей был бы обманчивым.

**Approach:** Создать tenant-scoped неопубликованный Employee draft и guided Employee Brief с Launch Plan: сохранение по шагам, возобновление, явные блокеры и один рекомендуемый следующий шаг.

## Boundaries & Constraints

**Always:** Создание и изменение доступны только действующему Owner или участнику с областью `configuration`, проверенной сервером через Identity API. `employee_id` — серверный UUIDv7; создание идемпотентно в пределах Tenant/actor/key. Черновик хранит owner настройки, версию, подтверждённо сохранённые поля и текущий шаг; конкурентное сохранение требует ожидаемую версию и не перезаписывает данные. Успешные изменения и конфликтные попытки имеют actor/version/time/correlation evidence; успешные изменения публикуют versioned outbox event. Данные, idempotency, audit и outbox изолированы TenantContext и FORCE RLS. Brief содержит управляемые поля назначения, имени, языков, локальных рабочих часов с часовым поясом, желаемых каналов и основных задач. Launch Plan отражает `Tenant → Brief → Domain Pack → Connectors → знания/поведение → полномочия/fallback → eval/preview → публикация` текстовыми состояниями и блокерами; будущие зависимости не притворяются готовыми. UI доступен с клавиатуры и на 320 px.

**Never:** Не публиковать Employee, не принимать клиентские обращения и не подключать реальные каналы, pack, knowledge или connectors в этой истории. Не принимать роль, `tenant_id` или owner из payload как доверенную authority; не создавать произвольные prompt-поля, tenant-specific код, cross-module SQL или прямое чтение таблиц другого модуля. Не называть желаемый канал доступным без проверки capability.

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Ожидаемый результат | Защита |
|---|---|---|---|
| Создание/повтор | Owner/Configurator, Tenant, idempotency key | Один draft/incomplete и переход к Brief | Повтор возвращает тот же ID, другой payload того же ключа — conflict |
| Сохранение/возврат | Валидные поля Brief и версия | Новая версия, сохранённый шаг, Launch Plan и Today/Card | Невалидное поле не частично сохраняется; future blockers остаются |
| Гонка | Два редактора одной версии | Первый save, второму conflict с перечитыванием | Новые данные не затираются; обе попытки имеют evidence |
| Нет права/чужой Tenant | Operator, подмена URL/body или другого ID | Безопасный отказ без чужих данных | Серверная authorization, FORCE RLS, no PII logs |

</frozen-after-approval>

## Code Map

- `internal/employee/{api,app,domain,port,store}` — новый owning-модуль Employee draft; auth только через `internal/identity/api`, без импорта identity/store.
- `internal/identity/api/api.go`, `internal/identity/domain/model.go` — переиспользовать `MembershipService.Authorize` и scope `configuration` для Owner/Configurator; не изменять таблицы Identity.
- `internal/platform/{tenancy,id,correlation}` и `migrations/identity/00001_identity_tenant.sql` — образцы typed TenantContext, UUIDv7, RLS/idempotency/outbox; новая миграция только в `migrations/employee/`.
- `cmd/staffy-core/main.go`, `internal/transport/http/server.go`, `contracts/openapi/staffy.yaml` — явно подключить модуль, `/v1` API и безопасные SSR routes/problem; не доверять payload scope.
- `web/templates/{workspace,setup}.html`, `web/assets/app.css` — существующий App Shell/тема; добавить Employees, Brief и Launch Plan без ложной публикации.
- `tests/integration/`, `tests/journeys/`, `internal/architecture/architecture_test.go` — расширить tenant/RLS/concurrency/HTML 320 px и boundary проверки.

## Tasks & Acceptance

**Execution:**
- [x] `internal/employee/{api,app,domain,port}` — описать typed draft/Brief/Launch Plan и команды с validation, optimistic version, idempotency и Identity authorization.
- [x] `migrations/employee/`, `internal/employee/store/` — сохранить Employee, Brief, version, current step, audit, idempotency и outbox в транзакциях с tenant-aware keys/FORCE RLS; конфликтные попытки аудировать без отката записи о попытке.
- [x] `cmd/staffy-core/main.go`, `internal/transport/http/`, `contracts/openapi/staffy.yaml` — подключить Employee API и маршруты create/list/read/save с безопасными status/problem ответами.
- [x] `web/templates/`, `web/assets/app.css` — сделать Today/Employees/Card/Brief/Launch Plan с одним следующим действием, сохранением/recovery, видимыми блокерами и доступностью.
- [x] `tests/`, `internal/architecture/architecture_test.go` — покрыть I/O-матрицу, outbox/audit, cross-tenant/pool, права Owner/Configurator/Operator, конкурентный save и 320 px/keyboard.

**Acceptance Criteria:**
- Given действующий Owner/Configurator, when он создаёт draft из Today или Employees, then видит стабильный employee_id, owner настройки, `draft/incomplete` и первый шаг Brief.
- Given заполненный Brief, when он сохраняется и открывается позже, then подтверждённые значения и этап восстанавливаются, а Today/Card показывают честное продолжение без заявления о запуске.
- Given отсутствующие Domain Pack, Connectors и остальные будущие зависимости, when открыт Launch Plan, then каждый этап имеет текстовый status/причину, а доступный Brief остаётся редактируемым.
- Given конфликт версии или отсутствие permission, when идёт API/UI mutation, then нет скрытого перезаписывания или межтенантной утечки и есть безопасное объяснение следующего действия.
- Given клавиатура, screen reader и ширина 320 px, when настройщик проходит Brief и Plan, then labels/errors/focus/reflow и цели ≥44×44 px проверяются.

## Implementation Notes

Черновик Employee и Brief сохраняются отдельно. Каналы в Brief обозначают только пожелания, а не готовые подключения. Конфликт версии показывает две версии для сравнения и требует явного подтверждения перед заменой.

## Spec Change Log

## Review Triage Log

- blind-1 — `false`: создание draft и затем сохранение Brief — два последовательных сохранённых шага; намерение не требует автосохранения каждого поля незавершённой формы.
- blind-2 — `false`: `launch_plan` наступает после валидного сохранения Brief; отдельное утверждение Owner не входит в утверждённый сценарий этой истории.
- blind-3 — `medium`, `patch`: после конфликтного ответа обновление страницы теряет значения попытки. Сохранить их в текущей форме, показать полный сохранённый Brief и разрешить осознанное повторное сохранение с новой версией без обновления страницы.
- blind-4 — `medium`, `patch`: форма ошибочно возвращает 400 и сообщение о полях на 404/500; развести validation и остальные ошибки.
- blind-5 — `medium`, `patch`: `Local` даёт неустойчивую серверно-зависимую интерпретацию часов; запретить это значение как неявный часовой пояс.
- blind-6 — `low`, `reject`: текущий Brief ограничивает рабочий интервал одним календарным днём. Ночные смены потребуют отдельной модели дня и уточнения UX; это не повседневный пилотный сценарий и прямого требования нет.
- blind-7 — `medium`, `patch`: конфликт material payload идемпотентного ключа не оставляет audit evidence; записать попытку с текущей версией и correlation ID.
- blind-8 — `low`, `reject`: отсутствие пагинации заметно лишь при большом числе черновиков одного Tenant; добавление новой поверхности/лимитов сейчас сложнее продемонстрированного риска.
- blind-9 — `false`: будущие зависимости намеренно отмечены `blocked` без проверки неподключённых модулей; текущий Tenant существует при наличии черновика.
- blind-10 — `low`, `patch`: аудит и outbox допускают осиротевшие ссылки в собственной схеме; tenant-aware FK напрямую укрепляет целостность evidence.
- edge-1 — `medium`, `patch` (совпадает с blind-4): серверная ошибка и отсутствующий Employee в форме маскируются как неверные поля.
- edge-2 — `medium`, `patch`: повторное чтение после конфликта может завершиться ошибкой, но страница не объясняет отсутствие сравнения; отдать безопасный статус ошибки.
- verification-gap-1 — `medium`, `patch`: отсутствует браузерная проверка сохранения введённых значений и показа актуального Brief при конфликте двух редакторов.

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.

Итог: все команды прошли; 12 интеграционных и 5 браузерных сценариев успешны. Три независимых review-слоя выполнены; подтверждённые замечания исправлены, два низкоприоритетных отклонены с обоснованием в triage log. Отложенной работы нет.
