---
title: 'Story 2.7 — Актуальные данные Owning System и обработка неопределённости (первый срез: State/Authority Matrix v1 и Observation-цикл)'
type: 'feature'
created: '2026-10-12'
status: 'ready-for-dev'
route: 'dispatch'
baseline_commit: '010d2ac92ee6e5e29f18c1f8642e897e9976d8ab'
context:
  - '{project-root}/_bmad-output/planning-artifacts/epics.md'
  - '{project-root}/_bmad-output/planning-artifacts/prds/prd-staffy-2026-09-12/prd.md'
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-staffy-2026-09-13/ARCHITECTURE-SPINE.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-1-channel-event-case.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-2-work-items-facts-provenance.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-4-response-owner-inbox.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-5-conversation-mode-ai-disclosure.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-6-progressive-intake-policy-outcome.md'
  - '{project-root}/_bmad-output/implementation-artifacts/deferred-work.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** 2-6 выдал детерминированный Policy Outcome, но материальные изменяемые поля (цена, наличие, статус заказа) в системе читаются из ниоткуда: State/Authority Matrix не существует — нечему объявить Owning System, внешний ID, разрешённую проекцию и правило конфликта для поля (AC-1); типизированного Read Request → Observation-контракта нет — провайдерские DTO не отделены от модели/UI ни кодом, ни словарём (AC-2, AD-5); freshness-семантики нет — «стало ли значение устаревшим» не вычисляет никто, повторного чтения перед сообщением материального условия требует некому (AC-3); исходы чтения нечестны некуда: timeout/malformed/insufficient scope не имеют словарных статусов failed/unknown, и в системе нет запрета «заполни пропуск наиболее вероятным значением» (AC-6); повторное чтение не идемпотентно на уровне наблюдений — «та же версия провайдера не создаёт дубликат, новая версия создаёт новую строку наблюдения» нечем обеспечить (AC-8); отказ одного Owning System не изолирован от других полей — агрегированного health нет и per-field честных исходов нет (AC-9); оператор не видит материальное поле со значением, источником, временем и свежестью — панели нет (AC-12); конфликт observation с подтверждённым фактом 2-2 не показывается (AC-5). Проверяемые последствия FR-19/FR-20/FR-21 остаются неработающими: RAG/факт 2-2 ничем не отделён от «текущей цены из Owning System», а неопределённость не имеет безопасного следующего шага.

**Approach:** Первый срез строит read-path Owning System целиком на детерминированном сервере (прецедент 2-5/2-6: AI Runtime и реальные провайдеры в контуре НЕТ — честная граница). Четыре части: (1) фиксированная authority-matrix-v1 — Domain Pack v0 (прецедент rule-table 2-6): три материальных поля (price, availability, order_status), каждому — owning system word, внешний ID-носитель, разрешённая проекция, правило конфликта и freshness в секундах; отсутствие строки матрицы = поле честно unknown, чтение невозможно как класс (AC-1); (2) команда read_owning_system (оператор с review ИЛИ configuration, гейт до стражей — прецедент M1 2-5) надевает typed Read Request на порт OwningSystemReader и материализует канонический Observation: append-only строка со статусом словаря {ok, failed, unknown}, reason-словарём, значением-проекцией (только из матрицы), внешним ID/версией/временем провайдера и evidence-ссылкой; провайдерская DTO не пересекает порт (AC-2); (3) идемпотентность и версии: replay по command-ключу 2-1 возвращает исходную запись без дублей; новая provider version → НОВАЯ строка с бо́льшим observation_no, старая остаётся для аудита (AC-8); свежесть — read-time деривация DeriveObservationState: age > freshness → stale, failed/unknown не приукрашиваются (AC-3/6); (4) изоляция и панель: каждое поле читается собственной командой — отказ одного owning system не касается других (AC-9); панель «Материальные поля Owning System» на карточке WI показывает значение/источник/внешний ID/версию/время/свежесть текстом, stale визуально и текстово отличим от fresh, конфликт с подтверждённым фактом 2-2 (байтовое сравнение канонических значений) — честный бейдж «расхождение», авто-разрешение запрещено (AC-5/12). Knowledge Source 1-5 остаётся знанием (объяснение), наблюдение — операционным фактом; generated summary не становится Owning System (AC-4 — матрица единственный источник объявлений).

**Решение владельца:** ожидает CHECKPOINT 1. Предложение SM: спека предъявляется владельцу в ответе сессии (делегированный прецедент 2-6: «одобряю (доверяю вашему экспертскому мнению)», чат 2026-10-10); отзыв/корректировка — по явной просьбе.

## Boundaries & Constraints

**Always:** Append-only: `owning_system_observations` — гранты INSERT/SELECT, RLS + FORCE ROW LEVEL SECURITY, грантов UPDATE/DELETE/TRUNCATE нет; действующее наблюдение поля WI — производная чтения (максимальный observation_no), не хранимое состояние; supersede = новая строка; идемпотентность: тот же command-ключ с тем же материалом → replay исходного observation (command_records 2-1); тот же ключ с иным материалом → ErrCommandKeyConflict; статус записи ∈ {ok, failed, unknown} — словарь CHECK; reason ∈ {provider_timeout, malformed_response, insufficient_scope, read_error} — словарь CHECK; ok ⇒ value NOT NULL + external id/version/observed_at NOT NULL; failed/unknown ⇒ value NULL (предполагаемое значение не пишется никогда — AC-6); значение — канонические JSON-байты ≤2048, проекция — слово матрицы (хранится, аудит); observed_at — время провайдера, received_at — своё (никогда не смешиваются, прецедент 2-2); freshness_seconds — из матрицы, staleness — read-time деривация, не хранится; owning_system word + внешний ID-носитель матрицы фиксируются в записи (снимок чтения); гейты команды: скоуп review ИЛИ configuration ДО материальных стражей (прецедент M1 2-5), не-участник → 404, чужой tenant → RLS 404; терминальный WI → честный 400 work-rule (граница 2-2, прецедент 2-6); observation_no — монотонный счётчик (WI, material_key) под advisory-локами кейса+WI (прецедент policy_outcomes 2-6); outbox-событие owning_system_observed — метаданные (поле, статус, reason, номер, внешний ID/версия), без значения-проекции и без внешнего тела (прецедент 2-6: кандидат не покидает строку); value-байты observation никогда не попадают в исключения/логи; панель: текст, aria-live без перемещения фокуса, 320 px без горизонтального скролла, stale/failed/unknown не отличимы от fresh только цветом.

**Never:** Не подключать реальных провайдеров HTTP/секреты/подписи (AG-4): единственный OwningSystemReader-адаптер — синтетический тестового контура (прецедент ingresssyn/verificationsyn; production-контур без адаптера отказывает честно до записи — прецедент «коннектор без верификатора» 2-1); не подтверждать факты 2-2 из observation (confirmer owning_system и source_type connector остаются зарезервированными словарями 2-2 без продюсера — активация следующими срезами 2-7; авто-выбор значения не появляется как класс); не ставить гейт observation-freshness на send_client_reply/evaluate_policy (порядок гейтов 2-4→2-5→2-6→CAS заморожен, 2-6; enforcement материального unknown — с Commitment-гейтом 2-9+); не строить bounded retry/circuit breaker/rate limit (одна попытка чтения, фиксированный deadline словарём — честная запись в словаре среза; полная дисциплина AD-5 — с реальными провайдерами); не строить фоновые refresh-воркеры/авто-перечитывание (перечитывание — операторская команда, срез честно ручной); не строить полную безопасную нормализацию недоверенного провайдерского текста (анти-injection AC-7: синтетический адаптер возвращает только словарные сценарии и канонические значения — нормализация материализуется с реальными провайдерами; матрица НЕ читается из tool output/сообщений — только фиксированный доменный словарь); не расширять матрицу конфигурацией на Tenant/Domain Pack-загрузкой (богатые per-pack матрицы — Epic 4/5); не трогать словари/гейты/миграции 2-1…2-6 (кроме честной append-only механики команды в command_records и словаря outbox-типов).

## I/O & Edge-Case Matrix

- Given кейс с действующим WI и оператор review, when read_owning_system (material_key=price, work_item, correlation-ключ), then Observation ok: append-only строка №1 (value-проекция price_quote, внешний ID/версия/observed_at провайдера, owning_system=catalog из матрицы), outbox-событие с метаданными без значения; панель показывает значение/источник/время/fresh (AC-1/2).
- Given повторная команда с тем же command-ключом и тем же материалом, when read исполняется, then replay: исходная строка возвращена, новых строк/событий нет (идемпотентность, AC-8).
- Given провайдер вернул новую версию (тот же материал, новый correlation-контекст), when read исполняется, then строка №2 (observation_no монотонный), №1 остаётся для аудита; действующая — №2; конфликтующих дублей нет (AC-8).
- Given сценарий провайдера provider_timeout, when read исполняется, then Observation failed (reason=provider_timeout), value NULL, external version NULL; панель: «чтение не удалось — причина словарём»; никаких предполагаемых значений (AC-6).
- Given сценарии malformed_response и insufficient_scope, when read исполняется, then failed/unknown с reason-словом; значение не записано; повторная команда с новым ключом — новая попытка честно (AC-6).
- Given ok-наблюдение age > freshness_seconds поля, when панель вычисляет состояние, then stale: бейдж «перечитайте перед материальным ответом» + кнопка чтения; fresh при age ≤ freshness (AC-3).
- Given два поля с разными owning system (catalog availability ok, erp order_status failed), when обе панели открыты, then per-field честные состояния; availability не замаскирован отказом erp, агрегированного health нет (AC-9).
- Given у кейса подтверждённый факт (2-2, DeriveFactStatus=confirmed) с тем же material_key и канонические байты observation ≠ байты факта, when панель поля открыта, then бейдж «расхождение с фактом»: обе карточки видны рядом, авто-разрешения нет; равные байты → бейдж «совпадает»; факта нет → без бейджа (AC-5).
- Given material_key вне матрицы («magic_field»), when read исполняется, then честный отказ поле-вне-матрицы (машиночитаемый 400 field-not-in-matrix; словарь полей в тексте отказа) — unknown как класс, чтение невозможно (AC-1).
- Given WI в терминальном состоянии (resolved/merged), when read исполняется, then 400 work-rule (терминальный WI не читается; merged — целевой WI) (граница 2-2).
- Given оператор audit-only (не review/configuration), when read исполняется, then 403 с текстом скоупа ДО стражей; ничего не записано (прецедент M1 2-5); не-участник → 404 ×пути (AC-12-край).
- Given пользователь другого Tenant, when читает/подает, then RLS + серверная авторизация: 404 без раскрытия существования (регресс 2-1).
- Given тот же command-ключ с иным материалом, when replay исполняется, then ErrCommandKeyConflict — честный 409 (AC-8-край, прецедент 2-6).
- Given production-контур без подключённого адаптера, when read исполняется, then честный отказ no-owning-system-reader ДО записи (прецедент 2-1 «коннектор без верификатора»; fail-closed, AC-6).
- Given панель с клавиатуры/screen reader/320 px, when наблюдение обновляется, then значение/источник/время/свежесть/статус доступны текстом; aria-live без неожиданного перемещения фокуса; stale не спутан с fresh без цвета (a11y-срез панели, AC-12).
- Given контракт OpenAPI и contract-тест, when read-путь описан, then схема ObservationRequest/ObservationView со словарями статусов/reason честна зеркально коду (прецедент TestPolicyContractIsHonest).

</frozen-after-approval>

## Code Map

- `internal/cases/domain/observation.go` (новый): словари (статусы, reason-слова, owning system words, материальные поля матрицы, проекции, лимиты), authorityMatrixV1 (Domain Pack v0), GuardReadOwnCmdMaterial, DeriveObservationState (fresh/stale/failed/unknown), ConflictBadgeOf (equal|differs|no_fact — байтовое сравнение канонических значений).
- `internal/cases/port/port.go`: интерфейс OwningSystemReader (typed Read Request → Observation-материал; адаптер — внешний, consumer-owned port, AD-2).
- `internal/adapters/observationsyn/` (новый): синтетический Owning System тестового контура — детерминированные сценарии по (owning_system, external_id): ok (значение/версия/время), новая версия по счётчику, timeout/malformed/insufficient_scope; production-контур не подключает (прецедент ingresssyn/verificationsyn).
- `internal/cases/app/observation.go` (новый): команда ReadOwnCommand — гейты (скоуп → участники → WI-терминальность → матрица-поле) → порт → материализация observation + outbox; replay по command-ключу.
- `migrations/cases/00007_owning_system_observations.sql` (новый): таблица `cases.owning_system_observations` (append-only, RLS+FORCE, INSERT/SELECT гранты, CHECK-словари статусов/reason, NOT NULL-инварианты ok/failed), +1 команда словаря command_records, +1 outbox-тип owning_system_observed.
- `internal/cases/store/observation.go` (новый): запись с монотонным observation_no под advisory-локами (прецедент policy 2-6), чтение производной действующего наблюдения поля WI, чтение истории для панели.
- `internal/cases/api/observation.go` (новый): ReadOwnSystem + view-карточки (значение/статус/freshness/конфликт).
- `internal/transport/http/cases_observation.go` (новый): `POST /v1/tenants/{t}/cases/{c}/work-items/{wi}/owning-system-reads` (API) + `POST /app/.../owning-system-reads` (PRG-форма панели); панель «Материальные поля Owning System» в `web/templates/case.html` (карточки полей: значение, источник, внешний ID, версия, время, свежесть, конфликт-бейдж, кнопки чтения).
- `openapi` (+1 путь, +2 схемы: OwningSystemReadRequest/ObservationView, словари статусов/reason/материальных полей).
- `tests/integration/cases_observation_test.go` (новый): демо-путь FR-19: read ok → replay → новая версия → stale по часам → timeout/malformed/insufficient_scope → изоляция полей → конфликт с фактом 2-2 → поле вне матрицы → терминальный WI → audit-only 403 → не-участник 404 → RLS 404 → ключ-конфликт → гранты pg_class-зонд (прецедент 2-5/2-6).
- `journeys/tests/cases/`: цепь cases-observation (панель, чтение PRG, stale-бейдж, конфликт-бейдж, aria-live, 320px).

## Tasks & Acceptance

- **step-01 — домен и матрица**: словари+матрица+гварды+деривации (observation.go, unit-тесты: словари закрыты, гварды честны, DeriveObservationState по возрасту, ConflictBadgeOf по байтам). Accept: unit зелёный; ни один статус/reason не появляется вне словаря.
- **step-02 — порт, адаптер, хранилище**: OwningSystemReader-порт, observationsyn-сценарии, миграция 00007 (RLS/гранты/словари), store-запись с монотонным номером и чтение производной. Accept: миграция применена; pg_class-зонд RLS/FORCE/гранты INSERT-SELECT; адаптер покрывает 5 сценариев детерминированно.
- **step-03 — команда, транспорт, панель**: app-команда с гейтами (M1-прецедент) и replay, API+PRG, панель case.html (карточки полей, конфликт-бейдж, aria-live, 320px), OpenAPI+контракт-тест. Accept: journeys локально зелёные; контракт зеркален.
- **step-04 — интеграционные и journeys**: полный матрикс-прогон (16 строк выше поимённо), journeys-цепь. Accept: интеграция ×2 TZ зелёная; сценарии изоляции и конфликта покрыты живьём.
- **step-05 — регресс и коммит**: build/vet/unit/интеграция ×2 TZ/journeys/контракт/gofmt/diff-check; коммит feat [без skip ci]. Accept: регресс зелёный во всех контурах; дерево чистое.

## Implementation Notes

- Матрица — фиксированный словарь домена (как ruleTableV1 2-6): богатые per-pack матрицы (внешний ID-носитель реальных систем, reconciliation query, terminal evidence) — Epic 4/5; словарь полей v1: price (catalog, price_quote, freshness 300s), availability (catalog, stock_count, 60s), order_status (erp, status_word, 120s).
- Конфликт-бейдж — байтовое сравнение канонических JSON-значений observation и подтверждённой версии факта 2-2 (одинаковый material_key): честный показ «расхождение — сверьте», решение всегда за человеком (deferred 2-2: «разрешение конфликта — подтверждение человеком»).
- Синтетический адаптер: сценарии детерминированы (map по owning_system+external_id, версия = счётчик успешных чтений поля), без глобального изменяемого состояния вне тест-контура; wiring — только composition roots (AD-2).
- observed_at (провайдер) vs received_at (своё): staleness считается от observed_at; панель показывает оба (AC-10: «значение, время актуальности и ограничения понятным языком» — текст панели оператора).
- Deadline чтения — фиксированное словарное слово (без конфигурации): одна попытка, честный failed (полный retry-контур — с реальными провайдерами, AG-4).

## Spec Change Log

- 2026-10-12, SM (первый срез): спека создана по форматам 2-5/2-6; статус backlog; ожидает CHECKPOINT 1 (предъявление владельцу в ответе сессии, делегированный прецедент 2-6).
- 2026-10-12, CHECKPOINT 1: спека одобрена владельцем без правок («продолжайте», чат 2026-10-12; предъявлена нетехническим пересказом в ответе сессии — делегированный прецедент 2-6 от 2026-10-10). Замороженные секции (Intent / Boundaries & Constraints / I/O & Edge-Case Matrix) не изменялись. Спека и sprint-status.yaml одновременно переведены в `ready-for-dev`; далее — Dev (step-01..05).
