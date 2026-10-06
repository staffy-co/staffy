---
title: 'Story 1.9 — Проверка capabilities конкретного подключения'
type: 'feature'
created: '2026-10-06'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '3ed539437bf6e8521ceb623b2ba977d75402b2e4'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-staffy-2026-09-13/GATE-REGISTRY.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-1-8-integrations-catalog.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 1.8 дала честное терминальное состояние `connected/unverified`: аутентификация и транспорт проверены, но неизвестно, какие функции конкретного подключения реально работают. Заявленные адаптером возможности не отделены от доказанных для аккаунта; у capability нет состояний, доказательств со сроком действия и правил аннулирования; проверка мутирующих функций без Action Gateway запрещена архитектурой, но нигде не показана как управляемая блокировка. deferred-work из 1-8 фиксирует три долга, решаемых здесь: stale-пометка evidence при ротации/отзыве секрета, конкурентный CAS-тест connect/rotate и идемпотентные повторы connect/rotate/revoke.

**Approach:** Расширить versioned Connector contract декларациями capabilities с обязательной операционной семантикой (направление, тип операции, timeout, ordering, rate/size limits, идемпотентность, receipt semantics, cancel/compensation, reconciliation, probe-разрешённость). Ввести неизменяемую (append-only) capability evidence для конкретного Connection и выводимое состояние каждой capability: `unverified / verifying / verified / unsupported / failed / unknown / stale`. Проверять только безопасные read-only probe-capabilities детерминированным синтетическим верификатором с deadline, редацией и различением delivery-уровней; мутирующие capabilities остаются `unverified` с блокировкой, указывающей требуемый архитектурный capability (Action Gateway), гейт и ответственного. Состояние не хранится вторым писателем — выводится из фактов: evidence + активный secret_id + история версий Connection + пинн контракта + срок действия.

**Решение владельца:** ожидает CHECKPOINT 1. Предложение SM: первый срез остаётся на локальном синтетическом контуре по образцу Stories 1-5/1-7/1-8 — единственный Connector с заявленными capabilities: `staffy-synthetic-telephony` (контракт 1.1.0); реальные провайдеры по-прежнему не подключаются (AG-4 не тронут). Employee readiness-блокировки, Action Gateway и автоматический reconciliation — вне среза (см. Design Notes); их отсутствие показывается честной управляемой блокировкой.

## Boundaries & Constraints

**Always:** Capability State показывается для каждого Connection в отдельности: capabilities, заявленные адаптером (из пинн-версии контракта Connection), и capabilities, доказанные для этого аккаунта (из evidence); каждая capability несёт состояние `unverified / verifying / verified / unsupported / failed / unknown / stale`, время проверки, срок действия и причину текущего состояния текстом. Evidence привязан к connection_id, secret_id активного секрета, среде и версии контракта: доказательство одного аккаунта, среды или Tenant никогда не переносится на другой — изоляция обеспечивается RLS и derivation-инвариантом. Каждая заявленная capability объявляет направление (inbound/outbound), тип операции (query/command/event), timeout, ordering, rate/size limits, идемпотентность, receipt semantics (`accepted / delivered / committed`), cancel/compensation, способ reconciliation и probe-разрешённость; отсутствующая обязательная семантика отображается как `unsupported`, а не выводится из поведения другого провайдера. Проверка запускается только для безопасной read-only (non-effecting) probe-capability, в разрешённой среде, с ограниченным scope и deadline; evidence хранит account (connection), environment, contract version, adapter build, время, actor, redacted request/result и срок действия; повтор проверки с тем же idempotency key replay-ится тем же результатом без противоречащих записей, конфликт ключа отклоняется. Мутирующая capability отображается `unverified` с блокировкой: требуемый архитектурный capability (Action Gateway, Epic 3), гейт AG-4, ответственный Technical Lead + Security; блокировка не мешает независимо доказанным read-only capabilities того же Connection. Результаты различают уровни `accepted / delivered / committed / unknown`: принятие API-запроса не отображается как успешный бизнес-результат; timeout или неоднозначный ответ после возможного эффекта дают `unknown`, вслепую не повторяются, интерфейс показывает владельца reconciliation и безопасный следующий шаг. Evidence становится `stale` при ротации или отзыве секрета, изменении конфигурации Connection, смене версии контракта или adapter build и при истечении срока действия; stale вычисляется при чтении, в опубликованную конфигурацию не переносится автоматически, для затронутой стадии требуется повторная проверка. Идемпотентность распространяется на connect/rotate/revoke (закрытие R2 Gap 5): повтор той же команды после сетевого сбоя читается как replay её результата, а не как «изменено другим участником». Закрывается R2 Gap 4: конкурентный integration-тест CAS для connect/rotate доказывает ровно один seal и один конфликт при параллельных командах. UI доступен: клавиатура, screen reader, 320 px, цели ≥44×44 px, обновление проверки объявляется без потери focus, технические детали evidence раскрываются прогрессивно.

**Never:** Не выполнять тестовую мутацию напрямую из Connector и не выдавать её за проверку capability. Не заявлять capability `verified` на основании проверки аутентификации/транспорта из Story 1-8 или поведения другого провайдера/аккаунта. Не переносить evidence между аккаунтами, средами, Tenant или версиями контракта. Не заводить mutable-таблицу состояний capabilities рядом с evidence (единственный источник — факты; вычисление состояния детерминировано и покрыто тестами). Не показывать expired, unknown или stale как успех и не скрывать блокировки. Не подключать production-среду и реальные credentials до AG-4 — без изменений относительно 1-8. Не протаскивать provider DTO в домен: evidence хранит только redacted-сводку запроса и результата. Не строить в этом срезе Action Gateway, async reconciliation-воркеры, runtime-enforcement rate/size limits и Employee readiness-интеграцию — только честные объявления, доказательства и состояния.

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Результат | Защита |
|---|---|---|---|
| Открытие Capability State | Connection `connected/unverified` | Заявленные vs доказанные capabilities; состояние/время/срок/причина текстом | RLS, роли |
| Capability без обязательной семантики | Определение не декларирует функцию | `unsupported`, не выводится из другого провайдера | Fail-closed объявления |
| Запуск проверки probe | verify-команда (read-only capability) + idempotency key | Evidence и `verified`; deadline соблюдён | Deadline, редация |
| Повтор проверки | Тот же idempotency key | Replay результата, противоречие не создаётся | Idempotency records |
| Конфликт idempotency key | Тот же key, другие данные | Отклонение `idempotency_conflict` | Hash-сверка |
| Timeout probe | Синтетический таймаут | `unknown`, владелец reconciliation, безопасный следующий шаг, без повтора вслепую | Deadline, `unknown` |
| Мутирующая capability | verify command-capability | Отказ: `unverified` + блокировка (Action Gateway, AG-4, ответственный) | Fail-closed |
| Inbound-событие capability | verify event-capability | Честный отказ: путь проверки не реализован в срезе | Честный `unverified` |
| Ротация секрета | rotate | Новая версия; прежняя evidence → `stale` (secret_id не совпадает) | Derivation, CAS |
| Отзыв Connection | revoke | Все capabilities неприменимы/`stale`; история сохранена | Derivation, терминальность |
| Изменение конфигурации | save | Статус `draft`; capabilities не заявлены до повторной проверки | Семантика 1-8 |
| Истечение срока evidence | Чтение после `expires_at` | `stale`, требуется повторная проверка | TTL + ленивый расчёт |
| Параллельные команды | Concurrent connect/rotate | Ровно один seal и один конфликт | CAS + конкурентный тест (R2 Gap 4) |
| Повтор connect/rotate/revoke | Тот же idempotency key после сбоя | Replay результата, без «изменено другим участником» | Idempotency (R2 Gap 5) |
| Чужое доказательство | Evidence другого connection/секрета/среды | Не применяется к запрошенному аккаунту | RLS + derivation |
| Доступность UI | Клавиатура / 320 px / screen reader | Состояния, время, срок и причины текстом; ≥44×44 px; focus сохраняется | Journey-тесты |

</frozen-after-approval>

## Code Map

- `internal/connector/domain/definition.go`, `internal/connector/catalog/catalog.go` — расширение контракта: `CapabilityDeclaration` с обязательной семантикой и probe-разрешённостью; новая версия определения `staffy-synthetic-telephony` 1.1.0 с тремя заявленными capabilities; прежние версии определений неизменяемы, новые Connection пиннят актуальную версию.
- `internal/connector/domain/capability.go` (новый) — словарь состояний, `unsupported`-правила, derivation применимости evidence (совпадение connection_id, secret_id, environment, версии контракта, adapter build; неистёкший срок), описание блокировки с гейтом и ответственным.
- `migrations/connector/00002_capability_evidence.sql` — append-only таблица `capability_evidence` (account, environment, contract version, adapter build, probe kind, actor, время, deadline, redacted request/result, outcome, delivery level, expires_at, idempotency key); RLS, гранты staffy_app без UPDATE/DELETE.
- `internal/connector/verifier/synthetic.go` — синтетический probe-верификатор: детерминированный, deadline через context, редация результата, различение delivery-уровней, таймаут-сценарий → `unknown`.
- `internal/connector/{app,port,store}/` — `VerifyCapability` (idempotent) и `CapabilityState` (derived); идемпотентность connect/rotate/revoke; конкурентные CAS-инварианты.
- `internal/transport/http/connector{,_view}.go`, `contracts/openapi/staffy.yaml`, `web/templates/integrations.html` — Capability State в карточке подключения, verify-действие, прогрессивное раскрытие evidence, доступность.
- `tests/integration/connector_test.go`, `internal/connector/**/*_test.go`, `tests/journeys/zzzz-integrations.spec.ts` — все строки матрицы, включая конкурентный CAS-тест и stale-derivation.

## Tasks & Acceptance

**Execution:**
- [ ] `internal/connector/domain/` — `CapabilityDeclaration`, контракт 1.1.0 (`query.trunk-status` / `command.send-sms` / `event.inbound-call`), состояния и derivation.
- [ ] `migrations/connector/00002_capability_evidence.sql` — immutable evidence, RLS, гранты.
- [ ] `internal/connector/verifier/` — probe с deadline, редацией, delivery-уровнями, таймаут → `unknown`.
- [ ] `internal/connector/{app,port,store}/` — `VerifyCapability` (idempotent replay/conflict), `CapabilityState` (derived), идемпотентность connect/rotate/revoke, конкурентный CAS.
- [ ] `internal/transport/http/`, `contracts/openapi/staffy.yaml`, `web/templates/integrations.html` — Capability State UI и контракт API.
- [ ] `tests/integration/connector_test.go`, unit-набор, `tests/journeys/` — все строки матрицы и доступность.

**Acceptance Criteria:**
- Given Connection `connected/unverified`, when пользователь открывает Capability State, then заявленные адаптером и доказанные для аккаунта capabilities показаны отдельно, каждая с состоянием, временем проверки, сроком действия и причиной текстом.
- Given capability без обязательной семантики в контракте, when строится список, then она отображается `unsupported`, а не выводится из поведения другого провайдера.
- Given read-only probe-capability и verify-команда с idempotency key, when проверка выполняется, then создаётся evidence с account, средой, версией контракта, adapter build, actor, redacted request/result и сроком действия; повтор с тем же ключом replay-ится тем же результатом; конфликт ключа отклоняется.
- Given probe завершается timeout после возможного эффекта, when формируется результат, then состояние `unknown`, повтор вслепую исключён, интерфейс показывает владельца reconciliation и безопасный следующий шаг.
- Given мутирующая capability, when запускается проверка, then операция отклоняется, capability остаётся `unverified` с блокировкой (Action Gateway, гейт AG-4, ответственный), а независимые read-only capabilities остаются доступными для проверки.
- Given ротация или отзыв секрета, when пересчитывается применимость evidence, then затронутые доказательства становятся `stale` и не переносятся на новый секрет.
- Given изменение конфигурации Connection, when читается Capability State, then capabilities не заявлены до повторной проверки (семантика 1-8 сохранена).
- Given срок действия evidence истёк, when читается состояние, then capability `stale` и требуется повторная проверка.
- Given повтор connect/rotate/revoke с тем же idempotency key, when команда отправлена, then результат replay-ится без «изменено другим участником».
- Given параллельные connect/rotate, when выполняются одновременно, then создаётся ровно один seal и один конфликт (конкурентный integration-тест).
- Given evidence принадлежит другому connection/секрету/среде, when запрашивается состояние, then доказательство не применяется (изоляция account/tenant).
- Given Capability State UI, when клавиатура, screen reader или 320 px, then состояния, время, срок и причины доступны текстом, основное действие ≥44×44 px, обновление проверки объявляется без потери focus.

## Implementation Notes

- Состояние capabilities выводится при чтении из фактов: заявленные — пинн-контракт Connection; доказанные — evidence, применимая при совпадении connection_id, secret_id активного seal, environment, contract version, adapter build и неистёкшем сроке; stale-входы — ротация/отзыв/конфиг-изменение (история версий Connection уже фиксирует их) и истечение срока. Второго писателя состояний нет — рассинхрон невозможен по построению.
- adapter build — константа адаптера в бинарнике (например `synthetic/1.1.0`); хранится в evidence и сравнивается derivation-ом; смена сборки аннулирует прежние доказательства как `stale`.
- Контракт 1.1.0 синтетической телефонии: `query.trunk-status` (outbound query, probe-разрешена, timeout 5 s, receipt `query_result`); `command.send-sms` (outbound command, идемпотентность по ключу, receipt `accepted → delivered → committed`, классификация `query_reconcilable` по GATE-REGISTRY, проверка заблокирована до Action Gateway); `event.inbound-call` (inbound event, честный `unverified` — путь проверки не реализован в срезе). Rate/size/ordering декларируются и показываются; runtime-enforcement — будущие срезы.
- Delivery-уровни: синтетический probe возвращает receipt уровня `delivered` с redacted-сводкой; таймаут-сценарий (инжектируется тестом) даёт `unknown` с владельцем reconciliation в UI. Словарь уровней зафиксирован в домене и OpenAPI.
- Идемпотентность connect/rotate/revoke: существующая механика `idempotency_records` расширяется на эти команды; replay возвращает сохранённый результат; OpenAPI фиксирует семантику (закрытие R2 Gap 5 из deferred-work 1-8).
- Гранты `capability_evidence`: INSERT/SELECT для staffy_app без UPDATE/DELETE — неизменяемость на уровне БД, как у истории версий; redacted-поля проверяются SQL- и unit-тестами на отсутствие секрета и сырого ответа.
- Проверки на текущем дереве: `go test ./...`, `go vet ./...`, `go test -tags=integration ./tests/integration/...`, `npm test --prefix tests/journeys`, контракт-тест OpenAPI, `git diff --check`.

## Spec Change Log

- 2026-10-06, review loop 0: создана спека первого среза Story 1.9 (SM). Статус `backlog` до CHECKPOINT 1 — одобрения владельцем; после одобрения спека и sprint-status.yaml одновременно переходят в `ready-for-dev` (условие синхронности check-continuity).
- 2026-10-06, CHECKPOINT 1: спека одобрена владельцем без правок («одобряю», чат 2026-10-06). Замороженные секции (Intent / Boundaries & Constraints / I/O & Edge-Case Matrix) не изменялись. Спека и sprint-status.yaml одновременно переведены в `ready-for-dev`; далее — Dev (step-03).

## Design Notes

Срез сознательно не строит Action Gateway, автоматический reconciliation и Employee-блокировки: Employee ещё не выбирает каналы (Epic 2), readiness-гейты — Stories 1.12–1.14. Управляемая блокировка мутирующих capabilities с точным указанием требуемого архитектурного capability, гейта и ответственного — честная граница вместо молчаливого отсутствия функции. Derived-состояние выбрано вместо mutable-таблицы по принципу «один источник истины на факт»: evidence append-only, состояние — чистая функция фактов. Три долга 1-8 из deferred-work закрываются здесь по решению DEVLOG от 2026-10-06.

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.
