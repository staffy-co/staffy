---
title: 'Story 1.10 — Управление периметром данных: Artifact Data Map и retention (первый срез)'
type: 'feature'
created: '2026-10-06'
status: 'done'
route: 'dispatch'
review_loop_iteration: 2
baseline_commit: '07871271280ed8da327766027ea299b5df6a6520'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-staffy-2026-09-13/GATE-REGISTRY.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-1-8-integrations-catalog.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-1-9-capability-verification.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Система уже хранит девять видов артефактов (настройки Tenant, данные участников, черновики Employee, знания, поведение, голос, предпрослушивания, подключения, запечатанные секреты, evidence, аудит), но нигде нет честной карты: какой класс зачем обрабатывается, на каком правовом основании, где лежит, кто его видит, сколько хранится и как удаляется. Retention существует только как одна точечная механика (24-часовая очистка voice previews, миграция 00008) — без версионированных политик, без различения результатов удаления, без квитанций. Платформенные требования NFR-14–NFR-16 требуют artifact-level data map, раздельные сроки хранения по классам и проверяемое удаление с честными состояниями `unknown`/`unsupported` вместо ложного «всё удалено». Отсутствие карты не позволяет readiness (Stories 1.12–1.14) проверять периметр данных.

**Approach:** Ввести модуль `privacy` с версионируемой Artifact Data Map: append-only записи атрибутов по каждому классу артефактов (назначение, правовое основание, controller/processor, получатели, место обработки и хранения, доступные роли, известные копии, ссылка на retention-политику, способ удаления/экспорта); текущая версия выводится при чтении, второго писателя нет. Классы артефактов — фиксированный словарь, синхронизированный с реальными таблицами системы тестом целостности периметра (незарегистрированный класс данных невозможен). Retention-политики версионируются строго по классу (одна политика не покрывает несколько классов неявно). Deletion workflow — идемпотентная команда data-governance-полномочия, оркестрирующая удаление через порты владеющих модулей (employee, identity), с результатом по каждому классу: `deleted / scheduled / retained_by_law / unsupported / unknown`; unknown получает владельца, срок и reconciliation вместо слепого повтора. Legal hold — append-only события с выводимым состоянием, удаление удерживаемого scope даёт `retained_by_law`. Каждая операция завершается Data Lifecycle Receipt с хэш-evidence и без повторного хранения удалённого содержимого.

**Решение владельца:** ожидает CHECKPOINT 1. Предложение SM: первый срез остаётся на локальном синтетическом контуре по образцу Stories 1-5/1-7/1-8/1-9 — карта и политики описывают реально существующие классы данных системы; физическое уничтожение запечатанных секретов и внешний SecretStore остаются за AG-4 (честный `unsupported` с блокировкой, как мутирующие capabilities в 1-9); DSAR-кейсы (запросы субъектов с проверкой личности), lineage производных артефактов и provider-deletion capabilities — следующие срезы (см. Design Notes).

## Boundaries & Constraints

**Always:** Artifact Data Map ведётся на каждый Tenant: для каждого класса артефактов из системного словаря показаны назначение, правовое основание, controller/processor, получатели и sub-processors, место обработки и хранения, доступные роли, известные копии, ссылка на retention-политику, способ удаления и экспорта. Записи карты append-only; текущее состояние выводится при чтении (последняя версия по tenant+классу); материальная перемена создаёт новую версию, прошлое не переписывается. Неизвестный обязательный атрибут помечает запись `incomplete` и блокирует затронутый маршрут (класс не может участвовать в readiness-проверке), а не заполняется предположением. Словарь классов синхронизирован с фактическими таблицами БД тестом целостности: класс без записи карты или запись без реального класса — красный тест, «незарегистрированные данные» невозможны. Retention-политика версионируется по purpose, legal basis, Tenant, source, artifact class и terminal event; содержит сроки active use, archive и deletion, правила backup expiry, исключения legal hold и владельца; привязка политики строго к одному классу — blanket-политика без класса отклоняется с честной причиной. Deletion workflow запускается только обладателем полномочия `data_governance` (новый scope, доступный Owner и назначаемый Configurator; просмотр карты — `data_governance` или `audit`), идемпотентен по ключу: повтор запроса replay-ит сохранённую квитанцию, конфликт ключа отклоняется. Результаты различают `deleted / scheduled / retained_by_law / unsupported / unknown`: локально выполненное удаление честно считается только по объявленным в карте копиям класса; копия без локального исполнителя или неоднозначный сбой шага дают `unknown` с владельцем, сроком и reconciliation — без слепого повтора и без заявления «удалено полностью». Legal hold сохраняет только минимально необходимый защищённый набор с основанием, scope, approver и сроком пересмотра; остальная разрешённая часть запроса выполняется; истёкший срок пересмотра показывается честно как требующий решения. Запечатанные секреты (`connection-secret-seal`) и immutable evidence получают честный `unsupported` с блокировкой: требуемый архитектурный шаг (AG-4: внешний SecretStore, физическое уничтожение), ответственный Technical Lead + Security. Журнал аудита удаляется не раньше минимального срока, заданного политикой класса, — попытка раннего удаления даёт `retained_by_law` с основанием. Data Lifecycle Receipt содержит request identity, authority, scope, версии применённых политик, выполненные операции, исключения, unknown-состояния, время и hash evidence; receipt не хранит удаляемое содержимое повторно (только идентификаторы, счётчики и хэши); позднее подтверждение обновляет состояние через append-only запись в journal квитанции. Изоляция Tenant — RLS во всех новых таблицах; полномочия применяются на сервере. UI доступен: клавиатура, screen reader, 320 px, цели ≥44×44 px, таблицы карты с корректными заголовками и адаптивным представлением, irreversible-удаление требует доступного подтверждения (не только цвета), результат операции объявляется без потери focus.

**Never:** Не заполнять неизвестный обязательный атрибут карты предположением и не показывать `incomplete`-класс как готовый. Не допускать одну политику хранения, неявно покрывающую несколько классов. Не заявлять удаление выполненным при `unknown`/`unsupported` копиях и не удалять «молча» часть scope. Не удалять журнал аудита ранее минимального срока и не маскировать это под успех. Не хранить в квитанции содержимое удалённых данных. Не заводить mutable-таблицу текущих версий карты/политик/hold-ов рядом с append-only фактами (второй писатель запрещён; состояние — чистая функция фактов). Не подключать внешний SecretStore и физическое уничтожение секретов (AG-4 не тронут). Не строить в этом срезе DSAR-кейсы с identity proofing, lineage производных артефактов, provider-deletion capabilities и автоматический фоновый планировщик удаления — только честные объявления и управляемые блокировки. Не давать полномочие `data_governance` неявно ролям, кроме Owner (назначается явно через membership).

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Результат | Защита |
|---|---|---|---|
| Открытие Data Map | Tenant с артефактами | Классы, атрибуты, версии, статусы (`complete`/`incomplete`), due-позиции | RLS, роли |
| Класс без обязательного атрибута | Запись с пустым/словарь-чужим значением | `incomplete` + блокировка маршрута класса; не «предположение» | Fail-closed объявления |
| Новый класс данных в системе | Миграция добавила таблицу без записи карты | Красный тест целостности периметра; runtime — класс отсутствует в словаре отклонён | Тест-инвариант |
| Публикация retention-политики | Политика с классом, purpose, basis, terminal event, сроками | Новая версия append-only; текущая — выводится | Версионирование |
| Blanket-политика без класса | Попытка создать политику «на всё» | Отклонение с честной причиной | Fail-closed |
| Запуск deletion workflow | Команда data_governance + idempotency key | Результаты по классам: deleted/scheduled/retained_by_law/unsupported/unknown; квитанция | Полномочия, идемпотентность |
| Повтор deletion-запроса | Тот же idempotency key | Replay квитанции; конфликт ключа отклоняется | Idempotency records |
| Класс с local-копиями только | knowledge payload / drafts / invitations / voice previews | `deleted` (счётчики, без содержимого в квитанции) | Хэш-evidence |
| Неистёкший срок политики | Запрос удаления раньше term | `scheduled` с датой исполнения | Политика |
| Legal hold активен | Запрос удаления удерживаемого scope | `retained_by_law` (минимальный набор, основание, approver, срок пересмотра); остальное выполняется | Hold-события |
| Срок пересмотра hold истёк | Чтение hold-состояния | Честная пометка «требует решения», не «активен» | Derivation |
| Запечатанные секреты | Удаление connection-secret-seal | `unsupported` + блокировка (AG-4, Technical Lead + Security) | Честный отказ |
| Evidence append-only | Удаление capability-evidence | `unsupported` + причина (immutable integrity; отдельный будущий срез) | Честный отказ |
| Аудит раньше минимального срока | Запрос удаления audit-trail | `retained_by_law` с основанием и сроком | Политика аудита |
| Неоднозначный сбой шага | Исполнитель класса вернул unknown (инжекция теста) | `unknown` + владелец + deadline + reconciliation; без слепого повтора | Словарь исходов |
| Чужой Tenant | Доступ к карте/квитанции другого Tenant | Отказ; изоляция | RLS |
| Права на просмотр | Участник только с `audit` | Просмотр карты/квитанций без управления | Скоупы |
| Необратимое удаление в UI | Подтверждение удаления | Доступное подтверждение (не только цвет), focus сохраняется, результат объявляется | Journey-тесты |
| Доступность UI | Клавиатура / 320 px / screen reader | Таблицы с заголовками, адаптивность, ≥44×44 px | Journey-тесты |

</frozen-after-approval>

## Code Map

- `internal/privacy/domain/` (новый модуль) — словарь классов артефактов (синхронизирован с реальными таблицами), словари атрибутов/локаций/исходов удаления, модель версий карты и политик, derivation текущих версий и hold-состояний, правила `incomplete`/блокировок.
- `migrations/privacy/00001_data_perimeter.sql` — append-only таблицы `privacy.data_map_entries`, `privacy.retention_policies`, `privacy.deletion_requests`, `privacy.deletion_outcomes`, `privacy.legal_hold_events`, `privacy.lifecycle_receipts` (+ journal); RLS, гранты staffy_app INSERT/SELECT без UPDATE/DELETE, CHECK-словари, UNIQUE (tenant, ключ, версия) / (tenant, idempotency key).
- `internal/privacy/{app,port,store}/` — DataMap (чтение выведенных версий), ManageMap/ManagePolicies (новые версии), RunDeletion (идемпотентная оркестрация через порты владеющих модулей, словарь исходов), Holds (события + derivation), Receipts (hash-evidence, journal).
- `internal/employee/app`, `internal/identity/app` — локальные исполнители удаления для своих классов (knowledge payload + chunks + checksum-ключи, employee drafts, voice previews/assessments, истёкшие invitations), экспонированные портами; отказ/неоднозначность возвращаются как исход, а не паника.
- `internal/identity/domain/model.go`, `web/templates/people.html` — новый scope `data_governance` (разрешён Owner и Configurator; выдаётся явно через membership), отображение в управлении полномочиями.
- `internal/transport/http/privacy{,_view}.go`, `contracts/openapi/staffy.yaml`, `web/templates/datamap.html` — UI карты/политик/удаления/hold-ов/квитанций, доступное подтверждение irreversible-действия, API-контракт.
- `tests/integration/privacy_test.go`, `internal/privacy/**/*_test.go`, `tests/journeys/zzzz-datamap.spec.ts` — все строки матрицы: полный жизненный цикл (карта → политика → удаление → квитанция), hold, unsupported/unknown-пути, replay, RLS, тест целостности периметра (словарь классов == фактические таблицы миграций), доступность.

## Tasks & Acceptance

**Execution:**
- [x] `internal/privacy/domain/` — словарь классов, словари атрибутов/исходов, версионирование, derivation, правила блокировок; тест целостности периметра.
- [x] `migrations/privacy/00001_data_perimeter.sql` — append-only таблицы, RLS, гранты, CHECK-словари.
- [x] `internal/privacy/{app,port,store}/` + исполнители в `employee`/`identity` — DataMap, Policies, RunDeletion (идемпотентный, исходы по классам), Holds, Receipts.
- [x] `internal/identity/domain/model.go` + `people.html` — scope `data_governance`.
- [x] `internal/transport/http/`, `contracts/openapi/staffy.yaml`, `web/templates/datamap.html` — UI и API-контракт.
- [x] `tests/integration/privacy_test.go`, unit-набор, `tests/journeys/` — все строки матрицы и доступность.

**Acceptance Criteria:**
- Given Tenant с артефактами, when Data/Privacy Admin открывает Data Map, then для каждого класса показаны назначение, правовое основание, controller/processor, получатели, место обработки/хранения, доступные роли, известные копии, ссылка на политику, способы удаления/экспорта, версия записи и статус.
- Given обязательный атрибут класса неизвестен, when открывается карта, then запись помечена `incomplete`, маршрут класса показан заблокированным, значение не заполнено предположением.
- Given миграция добавила таблицу без класса в словаре, when запускается тест целостности периметра, then тест красный (незарегистрированные данные невозможны).
- Given утверждение retention-политики, when она публикуется, then создаётся версия по purpose/legal basis/Tenant/source/class/terminal event со сроками active use, archive, deletion, backup expiry, legal-hold исключением и владельцем; blanket-политика без класса отклоняется.
- Given deletion workflow запущен обладателем `data_governance` с idempotency key, when выполняется, then по каждому классу формируется исход `deleted/scheduled/retained_by_law/unsupported/unknown`, создаётся квитанция с версиями политик, операциями, исключениями, временем и hash evidence; повтор ключа replay-ит квитанцию; конфликт ключа отклоняется.
- Given класс с истёкшим сроком и локальными копиями, when выполняется удаление, then исход `deleted` со счётчиком, квитанция не содержит удалённого содержимого.
- Given срок политики не истёк, when запрошено удаление, then исход `scheduled` с датой исполнения.
- Given активный legal hold на scope, when запрошено удаление, then удерживаемая часть — `retained_by_law` (основание, approver, срок пересмотра), остальная часть выполняется; истёкший срок пересмотра показан честно.
- Given запрошено удаление secret-seal или capability-evidence, when формируется результат, then исход `unsupported` с блокировкой (AG-4 / immutable integrity, ответственный), без молчаливого пропуска.
- Given запрошено удаление audit-trail раньше минимального срока, when формируется результат, then исход `retained_by_law` с основанием.
- Given исполнитель класса вернул неоднозначный результат, when формируется квитанция, then исход `unknown` с владельцем, сроком и reconciliation; слепой повтор исключён.
- Given участник без `data_governance` (только `audit`), when открывает карту, then доступен просмотр, управление запрещено; чужой Tenant изолирован.
- Given irreversible-удаление в UI, when подтверждается, then подтверждение доступно (не только цвет), цель ≥44×44 px, focus сохраняется, результат объявляется; таблицы карты доступны на клавиатуре, screen reader и 320 px.

## Implementation Notes

- Словарь классов первого среза (реальные таблицы): `tenant-configuration`, `member-identity` (principals/memberships/invitations), `employee-draft`, `domain-pack-pin`, `knowledge-source-payload` (sources/versions/chunks + checksum-ключи), `behavior-configuration`, `voice-configuration`, `voice-preview-media`, `voice-assessment`, `connection-configuration` (+versions), `connection-secret-seal`, `capability-evidence`, `idempotency-records` (identity+connector), `audit-trail`, `outbox-events`. Тест целостности сверяет словарь с инвентарём таблиц миграций — единый источник «что система реально хранит».
- Исполнение удаления — только через порты владеющих модулей («модули владеют своими таблицами»): privacy-оркестратор не пишет в employee/identity таблицы напрямую. Каждый шаг — самостоятельная транзакция исполнителя; исход шага фиксируется в `deletion_outcomes`; частичный успех честен по построению (это и есть словарь исходов).
- Аудит-строки, физически сцепленные внешним ключом с агрегатом класса (`employee.knowledge_audit` → `knowledge_sources`, `employee.audit` → `drafts`), удаляются вместе с агрегатом и честно считаются в квитанции как зависимые строки: схема БД не допускает другого порядка удаления. Минимальный срок класса `audit-trail` управляет маршрутом удаления самого класса аудита (запрос deletion по audit-trail), а не сцепленными строками агрегатов других классов — это осознанная семантика co-location, зафиксированная review R1.
- Voice previews: существующая 24-часовая очистка (миграция 00008) остаётся security-инвариантом; policy-срок для класса не может быть слабее (CHECK/валидация при публикации); on-demand удаление — дополнительный путь.
- `unknown`-путь тестируется инжекцией неоднозначного отказа исполнителя (по образцу host-маркеров timeout./fail. синтетического probe 1-9), включая сбой после фиксации исхода — квитанция получает owner + deadline + reconciliation.
- Идемпотентность: механика `idempotency_records` расширяется на deletion-команду (per-actor+operation, как connect/rotate/revoke из 1-9); replay возвращает сохранённую квитанцию.
- Hash evidence: sha256 по (request id, scope, применённые версии политик, строки исходов) — детерминированно проверяем в тестах; содержимое строк не входит.
- Проверки на текущем дереве: `go test ./...`, `go vet ./...`, `go test -tags=integration ./tests/integration/...`, `npm test --prefix tests/journeys`, контракт-тест OpenAPI, `git diff --check`.

## Spec Change Log

- 2026-10-06, review loop 0: создана спека первого среза Story 1.10 (SM). Статус `backlog` до CHECKPOINT 1 — одобрения владельцем; после одобрения спека и sprint-status.yaml одновременно переходят в `ready-for-dev` (условие синхронности check-continuity).
- 2026-10-07, CHECKPOINT 1: спека одобрена владельцем без правок («одобряю», чат 2026-10-07; выбор «ретроспектива vs 1-10» делегирован экспертности и решён в пользу продолжения разработки). Замороженные секции (Intent / Boundaries & Constraints / I/O & Edge-Case Matrix) не изменялись. Спека и sprint-status.yaml одновременно переведены в `ready-for-dev`; далее — Dev (step-03).
- 2026-10-07, Dev step-03: реализован весь срез (Execution 6/6). Полный регресс зелёный: build, vet, unit (17 пакетов), integration (включая 8 новых privacy-тестов), journeys 13/13 (включая zzzzz-datamap), git diff --check чист. Статус → `in-progress` до review R1.
- 2026-10-07, review R1: закрыты три находки — (1) RunDeletion больше не делает двойной LookupDeletion в replay-пути (один запрос вместо двух); (2) сбой запроса точки отсчёта класса при term-check теперь честный `unknown` с reconciliation (раньше — молчаливый fallback на дату публикации политики; добавлен юнит-тест TestRunDeletionAnchorFailureIsUnknown); (3) в Implementation Notes задокументирована сцепленность аудит-строк FK с агрегатами классов (co-location семантика). Замороженные секции (Intent / Boundaries / I/O Matrix) не изменялись. Статус → `done` после R2.
- 2026-10-07, review R2 (дифф правок R1): повторное чтение — правки минимальны, конфликт-путь идемпотентности идентичен прежнему, honest-unknown соответствует прецеденту аудита-маршрута; повторный полный регресс зелёный (build, vet, unit 18 пакетов включая новый app-тест, integration, journeys 13/13, git diff --check чист). Статус → `done`; ветка готова к PR в main. Наблюдения (не блокеры): окно потери квитанции между коммитом исполнителя и SaveDeletion и видимость форм карты для audit-only зрителей — зафиксированы в deferred-work.md; пробельная индентация — исторический стиль кодовой базы, CI gofmt не проверяет.

## Design Notes

Срез сознательно не строит DSAR-кейсы, lineage и provider-deletion capabilities: реальные потоки субъектов и производных артефактов появятся в Epic 2+, а provider-lifecycle capabilities лягут в evidence-модель 1-9 при появлении реальных провайдеров (AG-4). Управляемые блокировки с точным указанием требуемого шага, гейта и ответственного — честная граница вместо молчаливого отсутствия функции. Права `data_governance` — явное назначение через membership (никаких неявных ролей); аудит-скоуп читает карту — проверяемость для аудита без права изменения. Автоматический планировщик удаления отложен: срез даёт явную команду + due-видимость; автоматизация — с появлением фоновых воркеров (outbox-паттерн уже есть). Отложенное фиксируется в deferred-work.md.

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.
