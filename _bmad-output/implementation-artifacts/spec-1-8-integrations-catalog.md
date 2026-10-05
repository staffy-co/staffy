---
title: 'Story 1.8 — Выбор и безопасное подключение интеграций'
type: 'feature'
created: '2026-10-05'
status: 'review'
route: 'dispatch'
review_loop_iteration: 1
baseline_commit: 'f68bd5696ff75b1a02d41c925278d0e42ba43a0a'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-staffy-2026-09-13/GATE-REGISTRY.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Integration Admin не может выбрать допустимые каналы и бизнес-системы из каталога и безопасно сохранить подключение: неизвестно, какие Connector definitions реально реализованы, как хранить секреты без их раскрытия и как честно отражать состояние подключения до проверки capabilities.

**Approach:** Ввести бинарный версионированный каталог Connector definitions со статусами поддержки и декларативной схемой полей; создать tenant-scoped Connection с CAS-версиями, отдельными состояниями authentication/transport и непрозрачной ссылкой на секрет в отдельном хранилище-оттисках. Базовую проверку аутентификации и транспорта выполнять только разрешённым синтетическим маршрутом; production-credentials блокируются fail-closed до прохождения AG-4.

**Решение владельца от 2026-10-05:** Первый срез реализует локальный синтетический контур по образцу Story 1.5/1.7: единственный подключаемый Connector — `staffy-synthetic-telephony` со статусом `supported` (адаптер реализован локально и детерминированно). Реальные провайдеры (Novofon, Mango Office, Jivo, ABCP/Laximo, Яндекс) присутствуют в каталоге со статусами `candidate`/`deferred` без реализованных адаптеров — их подключение отклоняется честным объяснением. Exolve опубликован со статусом `unavailable` и не предлагается для подключения. Секреты принимаются ровно один раз, запечатываются HMAC-SHA256 с отдельным случайным ключом на Connection и никогда не возвращаются через UI или API; отображается только короткий отпечаток. Production-среда отклоняется на уровне доменной валидации с указанием gate AG-4 и ответственного; изменение payload или среды не обходит запрет.

## Boundaries & Constraints

**Always:** Каталог группируется по типам `SIP/telephony`, `chat/inbox`, `business system`, `AI/speech`, `storage/supporting service`; каждая карточка показывает назначение, статус поддержки, допустимые среды, требуемый тип учётной записи и проверяемую ссылку на документацию; статусы `supported / pilot / candidate / deferred / unavailable` доступны текстом и не смешиваются. Connection создаётся tenant-scoped со стабильным идентификатором, назначением, средой, владельцем и состоянием `draft`; сохраняются только поля, объявленные versioned connector schema; неизвестные и запрещённые параметры отклоняются. Секреты записываются через SecretStore и хранятся в Connection только как непрозрачная ссылка; исходное значение после приёма нельзя получить через UI или API; секрет не попадает в audit payload, outbox, telemetry или лог. Успешная аутентификация не означает подтверждение send/receive/delivery receipt/business mutation capabilities: общий статус остаётся `connected/unverified` до отдельной capability verification (Story 1.9). Неверные или недостаточные credentials дают состояние `failed` без ложного успеха; пользователь видит безопасное объяснение и следующий шаг; секретные значения и ответы провайдера редактируются перед журналированием. Повтор операции с тем же idempotency key не создаёт дубликат; конфликт с другими данными отклоняется. Ротация создаёт новую версию Connection и отзывает прежний secret reference; отзыв перевозит Connection в `revoked`. Доступность UI обязательна: клавиатура, screen reader, 320 px, цели не менее 44×44 px.

**Never:** Не принимать production credentials и не допускать среду `production` до прохождения AG-4 — fail-closed, без обхода изменением payload или среды. Не хранить исходные секреты в config, audit, outbox, логах или Release Manifest. Не выдавать успешную проверку аутентификации/транспорта за подтверждение capabilities. Не показывать нереализованные типы подключения как доступные; не представлять Exolve или будущих провайдеров доступными до публикации их Connector. Provider DTO и вызовы AI-модели не проникают в доменное ядро; добавление провайдера не требует ветвления по Tenant или изменения Domain Pack. Не строить универсальную integration-платформу — только декларативные definitions и безопасный жизненный цикл Connection.

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Результат | Защита |
|---|---|---|---|
| Каталог | Любой участник с областью Integrations | Группированные карточки со статусами текстом | Tenant/RLS, роли |
| Подключение нереализованного Connector | Novofon/Mango/Jivo/ABCP/Яндекс/Exolve, команда connect | Отклонить с честным объяснением «адаптер не реализован» | Fail-closed |
| Создание draft | Разрешённый Connector, sandbox/test среда, поля по schema | Новый tenant-scoped Connection `draft` | Idempotency, RLS |
| Неизвестное поле config | Поле вне versioned schema | Отклонить с точной причиной | DisallowUnknownFields, домен |
| Production среда | `environment: production` в любой команде | Отклонить fail-closed, показать gate AG-4 и ответственного | Доменная валидация |
| Приём секрета | Секрет по schema, synthetic-маршрут | Запечатать HMAC, opaque reference + отпечаток; проверить auth/transport | Один приём, без возврата |
| Синтетический успех | Секрет формата `sandbox-*`, endpoint `*.sandbox.example` | `connected/unverified`, auth/transport `verified`; capabilities не заявлены | Честный статус |
| Синтетический сбой | Неверный формат секрета/endpoint | `failed` c безопасным объяснением и следующим шагом | Без ложного успеха |
| Повтор idempotency key | Та же команда | Replay без дубликата | Idempotency records |
| Конфликт idempotency key | Тот же key, другие данные | Отклонить `idempotency_conflict` | Hash-сверка |
| Параллельное сохранение | Устаревшая версия | `conflict` через CAS | CAS, audit |
| Ротация секрета | Новое значение на живом Connection | Новая версия, прежний seal отозван, состояния сброшены | Аудит, отзыв |
| Отзыв Connection | Подтверждённая команда | Статус `revoked`, seal отозван | Без тихого удаления |
| Чтение Connection | Любая команда чтения | Конфиг и состояния без секрета | Секрет только opaque ref |
| Секрет в журналах | Audit/outbox payload | Только `secret_id` и отпечаток; исходное значение отсутствует | Redaction, тесты |

</frozen-after-approval>

## Code Map

- `internal/employee/packs/catalog.go`, `domain/pack.go` — образец бинарного версионированного каталога со статусами; Connector definitions оформлять так же, без исполняемого кода в definitions.
- `internal/employee/{domain,api,app,port,store}/voice.go`, `migrations/employee/00006_voice.sql` — образец CAS-версий, RLS, audit/outbox и неизменяемой истории; Connection хранить отдельным агрегатом в собственной схеме `connector`.
- `internal/employee/app/knowledge.go` (ChecksumKey, `crypto/rand`), `domain/knowledge.go` (`json:"-"`) — образец секрета-ключа, который хранится, но не экспонируется.
- `internal/adapters/authdev/`, `internal/employee/app/espeak.go` — образец development-only адаптера с явными переменными окружения и fail-closed поведением вне development.
- `internal/transport/http/{server,voice,membership}.go`, `contracts/openapi/staffy.yaml`, `web/templates/{people,employee}.html` — маршруты, схема контракта и доступный UI со статусами текстом.
- `internal/architecture/architecture_test.go` — расширить правилом импорта `connector/store` только владельцем и composition root.
- `tests/integration/{voice,tenant}_test.go`, `tests/journeys/fixture_server.go` — проверять RLS, CAS, idempotency, redaction и 320 px/клавиатуру.

## Tasks & Acceptance

**Execution:**
- [x] `internal/connector/{domain,catalog}/` — ConnectorDefinition со статусами, типами, средами, schema полей; бинарный каталог и валидация против schema.
- [x] `migrations/connector/00001_connections.sql` — схема `connector`: connections, connection_versions, secret_seals, idempotency_records, audit, outbox; RLS, гранты staffy_app.
- [x] `internal/connector/{api,app,port,store}/` — Service с областью Integrations, CAS-версии, idempotency, sealed secrets, honest states.
- [x] `internal/connector/verifier/` — синтетический верификатор auth/transport (детерминированный, redacted errors).
- [x] `internal/transport/http/`, `contracts/openapi/staffy.yaml`, `web/templates/integrations.html` — каталог, карточки, создание/connect/rotate/revoke, доступность.
- [x] `tests/integration/connector_test.go`, `internal/connector/**/*_test.go`, `tests/journeys/zzzz-integrations.spec.ts` — все строки матрицы.

**Acceptance Criteria:**
- Given каталог, when участник открывает Integrations, then карточки сгруппированы по типам, статусы доступны текстом, нереализованные Connector не предлагают подключение.
- Given команда создания с полем вне versioned schema, when она отправлена, then сохранение отклоняется с точной причиной.
- Given среда `production`, when любая команда до AG-4, then операция блокируется fail-closed с указанием gate и ответственного; смена payload/среды не обходит запрет.
- Given секрет принят, when Connection читается через UI/API, then возвращается только opaque reference и отпечаток; исходное значение недоступно; audit/outbox не содержат секрета.
- Given синтетическая проверка прошла, when отображается Connection, then общий статус `connected/unverified`, auth/transport `verified`, и capabilities не заявлены.
- Given неверный секрет или endpoint, when проверка выполняется, then статус `failed` с безопасным объяснением и следующим шагом, без ложного успеха.
- Given повтор с тем же idempotency key, when команда отправлена, then дубликат не создаётся; с другими данными — конфликт.
- Given ротация, when подтверждена, then новая версия Connection, прежний seal отозван, состояния сброшены; повторная проверка требуется заново.
- Given отзыв Connection, when подтверждён, then статус `revoked`, seal отозван, история сохранена.
- Given интеграции UI, when навигация клавиатурой и 320 px, then статусы/ошибки текстом, focus предсказуем, цели ≥44×44 px, секретное поле только для записи.

## Implementation Notes

- Роли: подключение выполняет Integration Admin или Owner через область `identity.Integrations`; проверка `Authorize` идентична другим модулям (`scope` → `tenancy.Context`). Отказ возвращает `404` без раскрытия существования пространства.
- Синтетический верификатор детерминирован: секрет совпадает с `^sandbox-[a-z0-9][a-z0-9-]{7,63}$` → authentication `verified`; endpoint host оканчивается на `.sandbox.example` (для test-среды — `.test.example`) → transport `verified`. Правило опубликовано в UI, чтобы не создавать иллюзию реальной проверки провайдера.
- Оттиск секрета: HMAC-SHA256 со случайным 32-байтным ключом на Connection; ключ хранится в строке seal и не экспонируется; отпечаток — первые 12 hex-символов HMAC для подтверждения человеком без раскрытия. Секрет запечатывается только при успешной аутентификации: невалидные credentials не хранятся, повторная попытка connect разрешена честно; смена секрета — только через ротацию.
- Секреты физически не покидают `connector.secret_seals`: грант staffy_app — SELECT/INSERT/UPDATE без DELETE; audit не имеет колонки payload вовсе; outbox payload содержит только `secret_id` и отпечаток — проверено SQL-проверкой в интеграционном тесте.
- Состояние `connected/unverified` — терминально честное для этого среза: UI прямо пишет, что успешная проверка аутентификации и транспорта не подтверждает отправку, приём и бизнес-операции (capabilities — Story 1.9).
- Изменение конфигурации сбрасывает статусы проверки в `pending`/`draft`; ротация отзывает прежний seal и проверяет заново; отзыв терминален, история версий неизменяема, `DELETE` закрыт грантами.
- Каталог: `staffy-synthetic-telephony` (supported, адаптер реализован), `novofon-sip` и `mango-office-sip` (candidate, адаптер не реализован — приоритетная пометка Novofon скрыта до появления адаптера), `jivo-chat`, `abcp-parts`, `yandex-speechkit` (deferred), `exolve-sip` (unavailable). Определения иммутабельны и хранят только декларативную schema без исполняемого кода.
- Проверки на текущем дереве: `go test ./...` (16 пакетов), `go vet ./...`, `go vet -tags=integration ./tests/integration/...`, полный PostgreSQL integration suite на выделенной базе 18.6 (RLS-изоляция, CAS, idempotency replay/conflict, fail-closed production на домене и на SQL CHECK, утечка секретов в outbox, гранты staffy_app), Playwright 11/11 при 320 px и клавиатуре, OpenAPI-контракт (production-enum, словарь статусов, DisallowUnknownFields), `git diff --check`.

## Spec Change Log

- 2026-10-05, review loop 0: создана спека первого среза Story 1.8.
- 2026-10-05, review loop 1: уточнена семантика запечатывания — секрет сохраняется только при успешной аутентификации; невалидные credentials не хранятся и допускают честный retry. Зафиксировано, что audit не содержит payload вовсе, а гранты seals запрещают DELETE.

## Review Triage Log

| Finding | Verdict / route | Evidence |
|---|---|---|
| R1 Blind · невалидный секрет запечатывался | high / patch | До исправления Connect запечатывал любой принятый секрет и блокировал retry «секрет уже принят». Теперь seal создаётся только при authentication=verified; проверено unit- и integration-тестами (ровно 2 seal, обе отозваны после revoke). |
| R1 Blind · ProductionBlockedError не матчился в транспорте | high / patch | `errors.As(err, &RuleError)` не находил вложенный ProductionBlockedError, ответ деградировал до 500 «internal_error». Транспорт и редирект теперь явно матчат ProductionBlockedError и возвращают 400 `production_blocked` с gate-доказательством AG-4 и ответственным. |
| R1 Blind · NULL secret_id ломал scan | high / patch | `scanConnection` не принимал NULL в secret_id/secret_hint до подключения секрета; добавлены nullable-переменные и `uuidArg` для всех SQL-путей. |
| R1 Blind · число плейсхолдеров insert | medium / patch | Вставка connection_versions в Connect содержала 16 выражений для 17 колонок; исправлено. |
| R1 Blind · приоритетная пометка Novofon | false / reject | Карточка Novofon скрывает PriorityNote, пока `AdapterImplemented=false` — прямо по AC; проверено каталог-тестом. |
| R1 Edge · подделка полей формы | false / reject | Клиентская подмена `config_field`/`environment=production` отклоняется доменной валидацией и SQL CHECK; проверено unit- и integration-тестами. |
| R1 Edge · порядок карточек в journeys | medium / patch | Фикстура List возвращала map в случайном порядке — тест переписан на hasText-локаторы, фикстура сортирует по updated_at desc как реальный store. |
| R1 Gap · секрет в config JSON | false / reject | ValidateConfig отклоняет секретные поля схемы; config содержит только несекретные значения; проверено unit-тестом и интеграционной проверкой read.Config. |
| R1 Gap · capability evidence при ротации | medium / defer | Пометка зависимых capability evidence как stale — объём Story 1.9; текущий срез сбрасывает состояния проверки и не заявляет capabilities. |
| R1 Gap · хранение отозванных seals | medium / defer | Физическое уничтожение/ротация ключей запечатывания и отозванных seals не реализованы; политика хранения согласуется с будущим retention-срезом (см. deferred-work). |
| R1 Gap · focus после ошибки формы | low / defer | Ошибки форм показываются через redirect с role=alert; восстановление фокуса в поле — общий UI-долг, задокументирован для следующего среза доступности. |

## Design Notes

Connection — отдельный агрегат в схеме `connector`, не привязан к Employee draft: подключение аккаунта провайдера предшествует выбору каналов конкретным Employee (Epic 2). Определение Connector пиннится версией, как Domain Pack. Проверка capabilities остаётся за Story 1.9; `connected/unverified` — терминальное состояние этого среза, а не успех.

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.
