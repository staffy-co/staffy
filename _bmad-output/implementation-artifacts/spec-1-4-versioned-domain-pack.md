---
title: 'Story 1.4 — Подключение версионированного Domain Pack'
type: 'feature'
created: '2026-09-26'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '516f098b45f0171763ca189257feda2b7cf9e8b6'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Domain Pack остаётся блокером: настройщик не может выбрать пакет и закрепить его версию без клиентского кода.

**Approach:** Встроить каталог и Domain Slot в Employee setup: выбрать пакет, сохранить разрешённые Tenant-варианты и явно подтверждать смену версии после просмотра последствий.

**Решение о стартовом каталоге:** Выпустить два минимальных reference pack — «Автозапчасти» и «Услуги Montagio» — только для стадии настройки. Их versioned metadata/schema и разрешённые варианты доступны для выбора, но отраслевой policy/runtime, реальные подключения, eval и живые обращения остаются явно заблокированы до следующих историй и проверок.

## Boundaries & Constraints

**Always:** Каталог показывает название, назначение, версию, статус поддержки, типы Connector, journeys, критичные сущности, политики, capabilities и gates; доступность зависит от Tenant/стадии. Неизменяемый manifest включает versioned schema, policy, knowledge/eval references и правило несовместимого перехода. Привязка хранит точный ID/version и варианты, но Employee остаётся `incomplete`. Варианты проверяются по schema: тип, диапазон, enum, межполевые правила; неизвестное и protected overrides отклоняются без записи. Новая версия не обновляет pin автоматически. Смена показывает diff/impact до подтверждения и помечает зависимые слоты `stale`/`requires review` без удаления. Mutation требует Identity `configuration`, TenantContext/FORCE RLS, expected Employee version, audit/correlation и versioned outbox. Domain Slot: клавиатура, 320 px, текстовые статусы, focus на предупреждении, одно primary action ≥44×44 px.

**Never:** Никакого исполняемого Tenant-кода, нового модуля, fork ядра, tenant-name branch, live Connector/knowledge/policy runtime, Release Manifest или публикации Employee. Выбор пакета не означает готовность к обращениям.

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Ожидаемый результат | Защита |
|---|---|---|---|
| Выбор | Draft, доступная версия, Owner/Configurator | Метаданные и pin ID/version/вариантов; incomplete | Недоступное не подключается |
| Неверные варианты | Неизвестное/недопустимое/protected | Полевые ошибки; прежний pin цел | Schema allowlist |
| Обновление | Pin и новый manifest | Старый pin остаётся; preview, подтверждение, stale markers | Version check, audit/outbox |
| Гонка/изоляция | Старая версия, Operator, чужой ID | Conflict/отказ без утечки | Identity, FORCE RLS |

</frozen-after-approval>

## Code Map

- `internal/employee/domain/model.go` — `Draft`, `LaunchPlan`; pin/impact без ложной готовности.
- `internal/employee/{api,app,port,store}` — `scope`, Identity authorization, optimistic save, tenant transaction, audit/outbox; расширять только owning-модуль.
- `migrations/employee/00001_employee_draft.sql` — образец FORCE RLS; новая миграция для pin/вариантов/impact.
- `internal/transport/http/{employee,server}.go`, `contracts/openapi/staffy.yaml` — API/problem/SSR маршруты; authority не из payload.
- `web/templates/employee.html`, `web/assets/app.css` — Domain Slot; `cmd/staffy-core/main.go` — wiring.
- `internal/architecture/architecture_test.go`, `tests/{integration,journeys}` — границы, RLS, версии, 320 px.

## Tasks & Acceptance

**Execution:**
- [x] `internal/employee/domain/pack.go`, `internal/employee/packs/` — immutable manifest/schema/compatibility, validation.
- [x] `internal/employee/{api,app,port,store}`, `migrations/employee/` — catalog/pin/preview/confirm, impact, auth/RLS/version/audit/outbox.
- [x] `internal/transport/http/`, `contracts/openapi/staffy.yaml`, `cmd/staffy-core/main.go` — API/SSR и wiring.
- [x] `web/templates/employee.html`, `web/assets/app.css` — доступный Domain Slot и честный Plan.
- [x] `tests/`, `internal/architecture/architecture_test.go` — I/O-матрица, две Tenant-конфигурации, version pin, 320 px/keyboard.

**Acceptance Criteria:**
- Given доступный draft/pack, when выбирают его, then точная версия, требования и варианты переживают перезагрузку.
- Given неверные поля/чужой Tenant, when сохраняют, then прежний pin цел, ошибка безопасна, утечки нет.
- Given новая версия, when открывают pin, then старый pin сохранён до preview и явного подтверждения смены.
- Given несовместимая смена, when подтверждают, then зависимые слоты `stale`/`requires review`, данные сохранены.
- Given клавиатура/320 px, when проходят Domain Slot, then доступные текст/focus/reflow/цели ≥44×44 px.

## Implementation Notes

Каталог содержит два setup-only reference pack и учебную версию автозапчастей 2.0.0 для проверки явного обновления. Контракт, форма и хранилище закрепляют точную версию и варианты; зависимые слоты получают review-маркеры. `cmd/staffy-core` использует существующий `BindEmployee`, через который зарегистрированы новые маршруты; отдельная правка wiring не потребовалась.

Матрица: выбор и сохранение — `TestDomainPackPinsIsolationImpactAndEvidence` и браузерный `Domain Slot pins…`; неверные варианты — `TestPinPreviewAndProtection`, `TestReferencePackVariantsAndImmutableCatalog`, интеграционный тест; обновление — те же app/integration/browser тесты; гонка и изоляция — app/integration тесты плюс существующие tenant/RLS тесты. Все эти тесты выполнены и прошли 2026-09-26.

## Spec Change Log

## Review Triage Log

| Слой / находка | Вердикт и проверка | Маршрут |
|---|---|---|
| Blind: downgrade 2→1 | medium — каталог предлагает 1.0.0 после pin 2.0.0, но `Impact` не находит обратное объявление, поэтому сохранение невозможно. | patch: объявить обратное несовместимое изменение. |
| Blind: preview не проверяет варианты | false — этот endpoint описывает impact смены manifest, а не валидность формы; `Pin` отдельно проверяет все варианты до записи, SSR получает preview без предложенных значений. | reject |
| Blind: варианты той же версии без review | false — на этом этапе нет подключённых зависимых конфигураций, а меняются только setup-предпочтения; версия остаётся прежней, runtime закрыт. | reject |
| Blind: теряются прежние review slots | medium — `Pin` заменяет список impact-списком при повторной смене, потенциально убирая незакрытые маркеры. | patch: сохранять объединение маркеров. |
| Blind: маркеры нельзя очистить | false — подтверждение зависимых Connector/knowledge/behavior слотов относится к следующим историям; сейчас эти слоты ещё заблокированы и данные не стираются. | reject |
| Blind: `Confirmed` не связан с preview token | false — `confirmed` является явным API-подтверждением; невозможно удостоверить факт человеческого чтения техническим токеном, а frozen intent требует preview и подтверждение, не proof-of-view. | reject |
| Blind: checkbox остаётся отмеченным после выбора | medium — смена select не снимает ранее отмеченное подтверждение другого impact. | patch: сбросить checkbox при смене выбора. |
| Blind: каталог одинаков для Tenant | false — `scope` авторизует каждый Tenant, все reference packs доступны для стадии setup, а `Pin` проверяет `Stage`; отдельного правила tenant-specific допуска не утверждено. | reject |
| Blind: preview ошибка скрывается | false — у всех предлагаемых переходов есть impact после исправления downgrade; ошибка preview сейчас возникает лишь для недоступного/устаревшего состояния, которое `Pin` тоже отклонит. | reject |
| Blind: manifest только в бинарном каталоге | maybe-false — текущие pinned версии существуют и возвращаются как копии; удаление их из будущего релиза нарушило бы инвариант, но такое изменение не показано. Нужна проверка политики сохранения старых версий при следующем изменении каталога. | defer (unverified medium) |
| Blind: SSR ошибка вариантов | low — проблемный ответ содержит field errors, а браузерная кнопка назад возвращает форму; повторного отображения введённых значений нет, но текущая форма малого объёма. Полный recovery добавил бы состояние и ветвление. | reject |
| Edge: подтверждение без просмотра | false — отдельный preview API и текстовый impact в SSR существуют; явный `confirmed` фиксирует выбор клиента, а доказательство чтения не является реализуемой гарантией. | reject |
| Edge: eligibility Tenant/stage | false — `scope` отделяет Tenant, `Stage==setup` проверяется при pin, а у обоих reference packs общий допуск в setup; иные eligibility-правила не заявлены. | reject |
| Edge: зависимые Plan slots не marked | medium — `ReviewSlots` и alert показывают review, но соответствующие строки LaunchPlan остаются просто `blocked`, вопреки требованию пометить зависимые слоты. | patch: отразить маркеры в статусах плана. |
| Verification-gap: нет HTTP теста preview | medium — route зарегистрирован, но прямого HTTP-теста нет; удаление маршрута не ломает существующие тесты сервиса и UI. | patch: добавить тест route/response. |
| Финальная проверка: статус без текстовой метки | medium — после появления `requires review` функция `stepStatus` возвращала пустой текст. | patch: добавить русскую метку и браузерную проверку. |

## Design Notes

Версии пакетов — декларативные неизменяемые артефакты; совместимость и последствия обновления объявляются явно в manifest, а не угадываются из свободного текста. Начальные setup-варианты: для автозапчастей — интерес к оригиналам, аналогам или обоим; для Montagio — инженерная сантехника и/или отопление. Это предпочтения настройки, не обещания наличия, цены или выполнения. Межполевые правила проверяются там, где объявлены schema; тестовый manifest покрывает их и обновление v1→v2. Пакет для настройки не означает допуск к runtime.

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.
