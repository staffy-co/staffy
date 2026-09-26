---
title: 'Story 1.1 — Создание защищённого пространства Tenant'
type: 'feature'
created: '2026-09-16'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '4f4ed350068f1f55755ecffd1c11d40a4a204545'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** В репозитории пока нет продуктового кода, а Staffy нуждается в первом безопасном вертикальном срезе: подтверждённый пользователь должен идемпотентно создать изолированный Tenant, стать его Owner и открыть честное пустое рабочее пространство без риска межтенантной утечки.

**Approach:** Создать минимальный Go-каркас modular monolith и реализовать модуль `identity` сквозным срезом PostgreSQL → application API → HTTP/OpenAPI → SSR App Shell. Генерируемый сервером TenantContext, RLS, атомарная транзакция Tenant+Owner+outbox и отрицательные интеграционные тесты образуют обязательную границу безопасности.

## Boundaries & Constraints

**Always:** Tenant ID — UUIDv7, генерируемый сервером; создание требует проверенного principal и permission; один transaction создаёт Tenant, Owner membership, idempotency result и immutable outbox event с actor/time/correlation ID. Все tenant-owned таблицы имеют `tenant_id`, tenant-aware constraints, `ENABLE/FORCE RLS`; runtime DB role — `NOBYPASSRLS`. Tenant scope получается из проверенной identity/membership или trusted scheduler metadata, применяется через `SET LOCAL` внутри transaction и очищается pool автоматически. API использует `/v1`, `snake_case`, `Idempotency-Key` и безопасную problem-форму. UI следует «Северному кедру», доступен с клавиатуры, имеет visible focus, landmarks, 44×44 px targets и reflow 320 px.

**Never:** Не принимать `tenant_id`, роль или permission из payload как доверенные; не раскрывать существование чужого Tenant; не логировать raw payload/PII; не добавлять другие бизнес-модули, Employee, Connectors, готовность каналов или tenant-specific ветвления; не создавать глобальные mutable dependencies, `utils/helpers/common`, cross-module SQL, ORM, SPA, broker или отдельный сервис.

**Identity decision:** Story 1.1 вводит production-neutral `PrincipalResolver`, который передаёт приложению только уже проверенный principal и его permission. Реальный OIDC/login lifecycle остаётся за границей истории. Для тестов и локального запуска разрешён отдельный fixed-principal adapter, активируемый только явной development-конфигурацией без доверия к публичному request header; production-конфигурация без настоящего resolver должна fail closed.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Создание | Verified principal, permission, company name, new key | `201`; стабильный `tenant_id`; Owner; event; ссылка на App Shell | Transaction rollback при любой ошибке |
| Повтор | Те же principal, key и material payload | Тот же результат, второго Tenant нет | Без повторного event |
| Конфликт ключа | Тот же key, другое имя | Ничего не меняется | `409` с correlation ID |
| Чужой объект | Подмена URL/header/payload/связанного ID | Нет чтения, записи или FK | Одинаковый безопасный `404/403`, audit без payload |
| Нет контекста | Request/job/store call без TenantContext | SQL к tenant-owned данным не выполняется | Fail-closed с устойчивым error code |
| Pool reuse | После Tenant A обслуживается B/no-context | Контекст A отсутствует | Тесты pool, prepared statement и rollback |

</frozen-after-approval>

## Code Map

- `go.mod`, `cmd/staffy-core/main.go` — создать модуль Go 1.27.1 и composition root только для HTTP, identity и PostgreSQL.
- `internal/identity/{api,app,domain,port,store}` — владелец Tenant, principal/membership, команды создания и PostgreSQL реализации; store не экспортируется наружу.
- `internal/platform/{tenancy,postgres,id,correlation,httpx}` — малые typed primitives для контекста, transaction scope, UUIDv7 и безопасных ошибок; без доменной политики.
- `migrations/identity/` — создать только principal, tenant, membership, idempotency и outbox структуры, constraints, runtime roles и RLS policies.
- `contracts/openapi/staffy.yaml` — контракт создания Tenant и чтения текущего workspace.
- `web/{templates,assets}` — SSR App Shell и пустое owner-state по DESIGN/EXPERIENCE; не показывать ложную готовность.
- `tests/{integration,journeys}` — PostgreSQL/RLS/idempotency/pool и browser accessibility/reflow проверки.
- `.github/workflows/ci.yml` — Go, migrations, architecture boundary и browser checks.

## Tasks & Acceptance

**Execution:**
- [x] `go.mod`, `cmd/staffy-core/`, `internal/platform/` — поднять минимальный компилируемый runtime с явным dependency wiring, correlation и fail-closed TenantContext.
- [x] `internal/identity/`, `migrations/identity/` — реализовать атомарное идемпотентное создание Tenant/Owner/event и RLS-защищённое чтение workspace.
- [x] `contracts/openapi/staffy.yaml`, `internal/transport/http/` — реализовать и валидировать `/v1/tenants` и tenant App Shell без доверия к payload scope.
- [x] `web/` — реализовать доступное empty state «Создать ИИ-сотрудника» в теме «Северный кедр».
- [x] `tests/`, `.github/workflows/ci.yml` — доказать happy path, конфликт, cross-tenant denial, missing context, pooled connection/prepared/rollback isolation и 320 px/keyboard requirements.

**Acceptance Criteria:**
- Given подтверждённый principal с permission, when он создаёт Tenant, then один commit сохраняет Tenant, Owner, idempotency result и ровно один versioned outbox event.
- Given Owner, when он открывает workspace, then видит Tenant, роль Owner и единственный следующий шаг без ложных ready-состояний.
- Given два Tenant или отсутствующий TenantContext, when выполняется чтение, запись, FK или повторное использование соединения, then application guard и PostgreSQL RLS не допускают межтенантного доступа.
- Given поддерживаемый браузер, when App Shell используется клавиатурой или при ширине 320 px, then landmarks/focus/44×44 px/reflow и текстовые role/state indicators проходят автоматические проверки.

## Implementation Notes

- Создан первый сквозной срез Go modular monolith: production-neutral `PrincipalResolver`, изолированный fixed-principal adapter только для loopback development и fail-closed production composition root.
- Модуль `identity` атомарно сохраняет principal, Tenant, Owner membership, idempotency result и versioned outbox event; TenantContext применяется через transaction-local PostgreSQL settings и `FORCE RLS` для runtime-роли `NOBYPASSRLS`.
- HTTP/OpenAPI и SSR App Shell не принимают caller-controlled Tenant scope, возвращают безопасные problem-ответы и показывают честное пустое состояние «Северный кедр».
- Матрица покрыта unit/HTTP/integration/browser тестами, включая принудительный отказ outbox с полным rollback, чужой Tenant/FK, отсутствующий контекст, pool/prepared/rollback, 320 px, keyboard focus и 44 px targets.
- Проверено: `go test ./...`, `go vet ./...`, `go test -count=1 -v -tags=integration ./tests/integration/...`, `npm test` в `tests/journeys`, разбор OpenAPI YAML и `git diff --check` — успешно.
- Graphify обновлён после появления продуктового Go/SQL-кода; health-check направленного графа: 632 узла, 1009 связей, 0 dangling, 0 self-loop, 0 collapsed-edge групп и 0 источников из исключённых служебных путей/исходного чата.

## Spec Change Log

## Review Triage Log

| ID | Verdict / route | Evidence |
|---|---|---|
| B1 | false / reject | `stat-index.json` — служебный cache, а не источник графа; проверка `graph.json` показала 0 узлов из `.agents`, `_bmad`, `.codex`, `graphify-out`, `AGENTS.md` и исходного чата. |
| B2 | medium / patch | `UUID.Valid` действительно принимал nil, неверную версию и variant; `Parse` теперь требует nonzero UUIDv7 с RFC variant, добавлен отрицательный тест. |
| B3 | medium / patch | Replay действительно генерировал новые ID до чтения persisted result; добавлен preflight `ReplayTenant` и тест с недоступной entropy. |
| B4 | medium / patch | Любая ошибка workspace уходила в 404/audit denial; теперь только `ErrNotFound` даёт 404, operational error даёт безопасный retryable 500 и отдельный audit code. |
| B5 | low / patch | Второй JSON-документ принимался; handler теперь требует EOF после единственного объекта и имеет отрицательный тест. |
| B6 | medium / patch | Неожиданный 500 имел `retryable:false`; deterministic client errors остаются false, неизвестные внутренние ошибки теперь true. |
| B7 | low / patch | Form error действительно коммитил status до HTML Content-Type; добавлен `renderStatus`, это проверяется тестом конфликтующей формы. |
| B8 | low / reject | CSRF зависит от будущего cookie/session resolver, явно исключённого из Story 1.1; текущий dev adapter доступен только loopback. Прямой low-risk body-limit всё же добавлен. |
| B9 | medium / patch | Primary CTA вёл на отсутствующий route; до Story 1.3 он заменён честным disabled next step с пояснением и browser assertions. |
| B10 | low / reject | Конкретный production auth transport намеренно не выбран; стандартная security scheme выдумала бы контракт. Граница trusted `PrincipalResolver` теперь явно описана и помечена extension в OpenAPI. |
| B11 | high / patch | Runtime DSN мог подключить superuser/BYPASSRLS; startup теперь читает фактические role flags и fail-closed, покрыто unit tests. |
| B12 | medium / patch | При установленном TenantContext principal-scoped idempotency rows были видны между Tenant; policy теперь ограничивает чтение текущим Tenant, сохраняя preflight до выбора Tenant. |
| B13 | medium / patch | `identity.tenant_created` допускал чужой aggregate; добавлен DB CHECK и отрицательный интеграционный тест. |
| B14 | medium / patch | Были только header timeouts; добавлены read/write/idle timeouts и header limit. |
| B15 | medium / patch | `pgxpool.New` был lazy и мог запустить нерабочий process; проверка runtime role выполняет реальный startup query. `/healthz` остаётся liveness после успешного startup. |
| B16 | medium / patch | CI не разбирал OpenAPI; добавлен Go contract test структуры, boundary marker, media type, responses и schemas. |
| E1 | medium / patch | Подтверждён тот же replay-before-entropy дефект B3; исправлен preflight lookup и отдельным regression test. |
| E2 | low / patch | App считал bytes, а PostgreSQL/OpenAPI — characters; лимит теперь использует RuneCount. |
| E3 | low / patch | Два zero-width rune проходили минимум; Tenant name теперь требует хотя бы одну Unicode letter/number, покрыто тестом. |
| E4 | medium / patch | Form повторно отправлял конфликтующий key; invalid/conflict render получает новый server key и остаётся явным ручным resubmit. |
| E5 | low / patch | Подтверждён B5; добавлен EOF guard и тест двух JSON values. |
| E6 | medium / patch | Подтверждён B4; operational workspace failure отделён от authorization denial. |
| E7 | medium / patch | Подтверждён B6; internal errors получили retryable semantics. |
| E8 | medium / patch | Подтверждён B14; HTTP server получил bounded timeouts/headers. |
| E9 | medium / patch | Подтверждён startup gap B15; обязательный DB query происходит до ListenAndServe. |
| E10 | medium / patch | Nil UUID мог стать principal/correlation; строгий UUIDv7 parser закрывает путь. |
| E11 | low / patch | Prefix `cmd/staffy-core-evil` проходил архитектурный тест; helper теперь требует directory boundary. |
| E12 | high / patch | Существующий/сконфигурированный unsafe DB role не проверялся; фактические `rolsuper/rolbypassrls` теперь обязательны false. |
| E13 | high / patch | RLS позволял principal A связать idempotency row с Tenant B; `WITH CHECK` теперь требует текущий Tenant, добавлен exploit test. |
| E14 | low / patch | JSON handler принимал иной media type; теперь строго требует `application/json`, OpenAPI и тест синхронизированы. |
| E15 | medium / patch | Journey мерил один control и первый Tab; теперь проверяет все видимые controls и keyboard/focus sequence обеих поверхностей. |
| V1 | medium / patch | Success HTTP test не проверял body/replay header; теперь декодирует все `TenantCreated` fields и отдельно проверяет `Idempotency-Replayed`. |
| V2 | high / patch | Outbox проверялся только count; integration test теперь сверяет ID, version, aggregate, actor, correlation, time и весь payload. |
| V3 | high / patch | Cross-tenant tests не читали outbox/idempotency и не атаковали их write policies; добавлены read/write/link denial checks. |
| V4 | high / patch | Loopback guard composition не был покрыт; добавлены `run` fail-before-DB test и IPv4/IPv6/localhost variants. |

## Verification

**Commands:**
- `go test ./...` — unit, architecture и HTTP tests проходят.
- `go vet ./...` — статические проверки проходят.
- `go test -tags=integration ./tests/integration/...` — PostgreSQL 18 подтверждает migrations, RLS и pool isolation.
- `npm test --prefix tests/journeys` — Playwright проверяет keyboard и viewport 320 px без page overflow.
