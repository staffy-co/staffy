# DEVLOG — журнал разработки staffy (append-only)

> Правила: записи только в конец; прошлое не редактируется (ошибки лечатся
> записью `correction:`); запись создаётся после каждого значимого подшага и
> едет в том же коммите, что и изменения. `next:` последней записи = указатель
> продолжения; `commit:` = последний подтверждённый коммит на момент записи.
> Порядок доверия: git (origin) > sprint-status.yaml > DEVLOG > пересказ диалога.
> Формат записи, ритуалы и режимы отказа: docs/CONTINUITY.md, протокол — AGENTS.md.

---
date: 2026-10-06
task: backfill
role: main (Super Z)
story: process
summary: Backfill журнала из локального worklog песочницы (сессии до 2026-10-06)
done:
- Epic 1: stories 1-1…1-7 реализованы и закрыты полным циклом review (R1+R2), каждая через PR в main (PR #1…#8)
- Story 1-8 «Выбор и безопасное подключение интеграций»: реализация (PR #9, ветка codex/story-1-8-integrations-catalog), R2-review — 9 патчей + 2 defer; закрыта коммитом fa7b315 (локальный мерж, оформленный как «PR #10» — процессный долг, больше не имитировать)
- Окружение на тот момент: Go (~/.local/go, toolchain 1.27.1), портативный PostgreSQL 16.4 в /tmp/pgdist (порт 54329, БД staffy_test), Playwright journeys 11/11
decisions:
- Позиция BMAD: stories 1-1…1-8 done; 1-9 «Проверка capabilities конкретного подключения» — backlog
- Точка остановки: следующий шаг процесса — SM создаёт спеку 1-9
next: SM создаёт спеку story 1-9 (bmad-build step-01/02 → CHECKPOINT 1: одобрение владельцем)
commit: fa7b315

---
date: 2026-10-06
task: 4-8
role: main (Super Z)
story: process
summary: Экспертный консилиум (5 экспертов) и внедрение системы непрерывности «пережить сброс сессии/окружения без потерь»
done:
- Консилиум 2-a GitOps / 2-b BMAD / 2-c AgentOps / 2-d Security / 2-e QA: диагнозы и синтез — docs/CONTINUITY.md
- Внедрено: scripts/bootstrap.sh (идемпотентное восстановление: код → Go → PostgreSQL → БД → миграции → smoke → env.sh), third_party/portable-pg16.tar.xz (вендоренный PG 16.4, 15 МБ), .githooks/pre-commit+pre-push (скан секретов), scripts/git-credential-staffy.sh (токен из env, чистый remote URL), scripts/check-continuity.sh, DEVLOG.md (этот файл), docs/CONTINUITY.md, AGENTS.md расширен протоколом непрерывности (секция graphify сохранена дословно)
- Окружение основной песочницы мигрировано с /tmp на ~/.local (pg, pgdata) — переживает чистки /tmp
- graphify-out/cache/ (637 генерируемых файлов, ~55% репо) убран из git-трекинга; файлы остаются локально (run-local accounting, как cost.json)
decisions:
- STATE.md отвергнут консилиумом (3/5 против): третий источник истины = гарантированный рассинхрон; позиция = sprint-status.yaml (канон) + next: этого журнала; проверяемость — scripts/check-continuity.sh
- PostgreSQL вендорен в репо: внешний источник (EDB) отдаёт 403, /tmp не переживает сброс; Go остаётся на официальной загрузке go.dev (fallback bootstrap)
- Push-протокол: коммит на зелёном чекпоинте, push сразу; WIP-пуши с [skip ci] (CI гоняет полный конвейер на каждый push); отказ push = СТОП работы
- Секреты: токен только в env STAFFY_GH_TOKEN; remote URL чистый; .githooks сканируют стейдж и уходящие коммиты
next: SM создаёт спеку story 1-9 «Проверка capabilities конкретного подключения» (bmad-build; вход: sprint-status.yaml → epic-1-context.md → epics.md §Story 1.9 → spec-1-8 → deferred-work.md, где 3 записи нацелены на 1-9)
commit: fa7b315

---
date: 2026-10-06
task: drill
role: main (Super Z)
story: process
summary: Учения системы непрерывности — полное восстановление с чистого листа, все гейты зелёные
done:
- Фаза 1 (fresh clone с GitHub): HEAD drill-клона совпал с ожидаемым 243dfd6 → PASS-1 (RPO = 0, незапушенной работы нет)
- Фаза 3 (bootstrap в изолированном каталоге, свой PG на порту 54330): клон → Go → PostgreSQL из third_party/portable-pg16.tar.xz → initdb → БД staffy_test/staffy_dev (single-user mode) → миграции через integration-суит → npm ci → build/vet/unit — exit 0, ~4 минуты
- V6 идемпотентность: повторный запуск bootstrap — exit 0 («уже запущен», drop+re-apply миграций переживается)
- V8 консистентность: check-continuity в drill-клоне — OK (fail=0, warn=0)
- V7 ненанесение ущерба: основной клон чист, синхронен с origin, основной PG жив
- Security: remote URL drill-клона автоматически очищен скриптом; credential helper подключён
decisions:
- Drill признан успешным; доверие системе непрерывности подтверждено (до следующего изменения bootstrap — тогда re-drill)
- Замечание окружения песочницы: env-переменные не переживают между вызовами shell — STAFFY_GH_TOKEN экспортировать в той же команде, что и git-операции
next: SM создаёт спеку story 1-9 «Проверка capabilities конкретного подключения» (bmad-build step-01/02; вход: sprint-status.yaml → epic-1-context.md → epics.md §Story 1.9 → spec-1-8 → deferred-work.md; затем CHECKPOINT 1 — одобрение владельцем)
commit: 243dfd6

---
date: 2026-10-06
task: 6-sm
role: main (Super Z, роль SM)
story: 1-9
summary: SM-шаг story 1-9 — спека первого среза написана, запушена на ветку, ожидает CHECKPOINT 1 (одобрение владельцем)
done:
- Возобновление после сброса сессии И окружения по протоколу AGENTS.md: fresh clone → bootstrap (exit 0, полное окружение) → check-continuity OK (fail=0, warn=0) — система непрерывности пережила реальный сброс без потерь
- Входы SM прочитаны: epics.md §Story 1.9 (FR-8), epic-1-context.md, spec-1-8, deferred-work.md (3 записи нацелены на 1-9), GATE-REGISTRY.md (AG-4), PRD FR-8
- Написана и запушена спека первого среза: `_bmad-output/implementation-artifacts/spec-1-9-capability-verification.md` на ветке `codex/story-1-9-capability-verification` (коммит a188786, [skip ci])
- Спека закрывает три долга 1-8: stale-пометка evidence при ротации/отзыве, конкурентный CAS-тест connect/rotate, идемпотентные повторы connect/rotate/revoke
decisions:
- Состояние capabilities — derived из фактов (evidence + активный secret_id + история версий + пинн контракта + срок действия), без mutable-таблицы состояний: один источник истины, рассинхрон невозможен по построению
- Спека и sprint-status.yaml остаются в статусе `backlog` до CHECKPOINT 1; после одобрения владельцем — одновременно `ready-for-dev` (условие синхронности check-continuity)
- Срез остаётся на локальном синтетическом контуре: AG-4 не тронут; Action Gateway, Employee readiness-блокировки и автоматический reconciliation — честные отклонения среза (Design Notes спеки)
next: CHECKPOINT 1 — владелец одобряет или правит спеку 1-9 (файл на ветке codex/story-1-9-capability-verification; ветка запушена). После одобрения: перевести спеку и sprint-status.yaml в ready-for-dev на той же ветке → Dev (step-03) реализует срез по Tasks & Acceptance
commit: a188786

---
date: 2026-10-06
task: 7-sm
role: main (Super Z, роль SM)
story: 1-9
summary: CHECKPOINT 1 пройден — владелец одобрил спеку 1-9 без правок; статус ready-for-dev на ветке и в sprint-status.yaml
done:
- Владелец ответил «одобряю» (чат 2026-10-06) — CHECKPOINT 1 закрыт
- На ветке codex/story-1-9-capability-verification: спека и sprint-status.yaml одновременно переведены в ready-for-dev (коммит 7d20a2d, push); замороженные секции спеки не тронуты; Spec Change Log дополнен записью CHECKPOINT 1
- В AGENTS.md добавлен постоянный протокол коммуникации с владельцем (нетехнический владелец — простые объяснения) — по прямой просьбе владельца
decisions:
- Одобрение зафиксировано как есть, без правок спеки — frozen-секции неизменны
- Правило простых объяснений — постоянное, живёт в AGENTS.md (переживает любой сброс), а не в памяти сессии
next: Dev (step-03) реализует срез story 1-9 по Tasks & Acceptance спеки (файл _bmad-output/implementation-artifacts/spec-1-9-capability-verification.md; входы в frontmatter context:); работа на ветке codex/story-1-9-capability-verification; подшаги с зелёными тестами — чекпоинт-коммит + push
commit: 7d20a2d

---
date: 2026-10-06
task: 8-dev
role: main (Super Z, роль Dev)
story: 1-9
summary: Dev step-03 — реализован весь срез story 1-9 (capability verification); полный регресс зелёный; статус in-progress до review R1
done:
- Домен: internal/connector/domain/capability.go — словарь состояний (unverified/verifying/verified/unsupported/failed/unknown/stale), CapabilityDeclaration с обязательной операционной семантикой (+Complete-валидация), CapabilityEvidence (append-only факты), derivation-функция DeriveCapabilityStates (чистая, без второго писателя), managed-блокировка ActionGatewayBlock (AG-4, Technical Lead + Security)
- Каталог: контракт staffy-synthetic-telephony 1.1.0 с тремя capabilities (query.trunk-status probe-разрешена; command.send-sms мутирующая — блок; event.inbound-call — честный unverified); 1.0.0 неизменяем и резолвится для пиннов; List показывает только актуальные версии; Create отвергает устаревший пинн честной причиной
- Миграция 00002: connector.capability_evidence (append-only, RLS, INSERT/SELECT для staffy_app, CHECK-и outcome/delivery/probe_kind/redaction, UNIQUE (tenant, connection, idempotency_key)); idempotency_records.resulting_version; audit outcome CHECK расширен (replayed/idempotency_conflict/capability_*)
- Верификатор: Probe (детерминированный, host-маркеры timeout./fail., ctx-deadline → unknown, редакция — только host, AdapterBuild synthetic/1.1.0, Build())
- app/port/store: VerifyCapability (guards → probe с deadline → SaveEvidence идемпотентно), CapabilityState (derived при чтении), LookupCommand/LookupEvidence ДО доменных стражей (повтор после сетевого сбоя = replay, а не ложный конфликт), идемпотентность connect/rotate/revoke через idempotency_records + replay снапшота версии; single-secret правило перенесено под row-lock (гонка connect/connect = ровно один seal + один ErrConflict — R2 Gap 4 закрыт конкурентным integration-тестом; R2 Gap 5 закрыт replay-семантикой)
- HTTP/UI: GET .../capabilities, POST .../capabilities/{id}/verify (+Idempotency-Key и Idempotency-Replayed на connect/rotate/revoke/verify); панель Capability State в карточке (заявленные vs доказанные, state/reason/время/срок, managed-блокировка, verify-кнопка ≥44px, прогрессивное раскрытие evidence, role=status); flash/notice по исходу (verified/failed/unknown)
- OpenAPI: 2 новых пути + 4 схемы (CapabilityDeclaration/Report/Block/EvidenceSummary, enum-ы состояний и delivery-уровней), идемпотентные заголовки документированы; контрактный тест
- Тесты: домен (derivation-матрица: stale-причины по фактам, unsupported, блок, unknown+владелец, latest-per-capability), верификатор (probe-сценарии, редакция), app (guards, replay/conflict, stale после ротации), http (API+формы+панель), интеграция (полный жизненный цикл evidence, таймаут→unknown, истечение TTL, изоляция чужих доказательств, гранты immutable, конкурентный CAS), e2e (панель, verify, блок, stale после ротации, 44px, live region)
- Проверки на дереве: go test ./... (16 ok), go vet ./... (чисто), go test -tags=integration ./tests/integration/... (ok), npm test --prefix tests/journeys (12/12), git diff --check (чисто)
decisions:
- Порядок идемпотентность-раньше-стражей: повторы команд и verify отвечаются LookupCommand/LookupEvidence до доменных guard-ов, иначе повтор после сбоя сети падал бы в «секрет уже принят»/«уже отозвано» вместо replay (спека: повтор = replay результата)
- Single-secret rule под row-lock в store (не в service): при конкурентных connect побеждает детерминированный CAS-конфликт, при последовательном двойном connect — честная rule-ошибка 400
- Адаптер и фикстура: каждый принятый seal = НОВЫЙ secret_id + UpdatedAt-bump (как в SQL) — иначе derivation не видит stale при ротации в e2e
- stale-причины выводятся в приоритете: отозвано → секрет ротирован/отозван → конфигурация/состояние изменились → версия контракта → сборка адаптера → истечение срока (точная причина текстом)
next: review R1 свежим контекстом (Reviewer читает спеку + git diff 7d20a2d..HEAD; R1/R2 → правки → PR в main; статусы review/done синхронно со sprint-status.yaml)
commit: f10dda0

---
date: 2026-10-06
task: 10-review
role: main (Super Z, роли Reviewer R1/R2)
story: 1-9
summary: Review R1+R2 свежим контекстом (спека ↔ дифф 7d20a2d..5392388); два honesty-дефекта UI найдены и закрыты; полный регресс зелёный; статус done, ветка готова к PR
done:
- R1: полная сверка спеки с git diff 7d20a2d..f10dda0 (27 файлов, +3279/-331): словарь состояний и derivation (чистая функция фактов), migration 00002 (RLS FORCE, append-only гранты INSERT/SELECT, CHECK-словари, UNIQUE tenant+connection+key), синтетический probe (редация фиксированными строками, deadline, delivery-уровни, timeout → unknown), идемпотентность connect/rotate/revoke/verify через LookupCommand/LookupEvidence ДО доменных стражей, конкурентный CAS-тест (R2 Gap 4) и replay-семантика (R2 Gap 5), OpenAPI 2 пути + 4 схемы с контракт-тестом, e2e-доступность (320px, 44px, live region, редация секрета)
- Независимая регрессия рецензента: go test -count=1 ./... (16 ok), go vet чисто, integration -count=1 (21 тест), journeys 12/12, git diff --check чисто
- Findings R1 (оба закрыты в 5392388): (1) verifyCapabilityForm default-ветка редиректила с capability_verified=1 при stale-replay — нарушение Never-правила «не показывать expired/unknown/stale как успех»; (2) фокус терялся после verify-redirect при спека-требовании «обновление проверки объявляется без потери focus»
- Fixes 5392388: capability_stale=1 для stale-replay + нейтральный default-редирект без флага; фокусируемый #page-notice (tabindex=-1 + focus() по DOMContentLoaded, образец employee.html); http-тесты ×3 (stale-replay честно, non-terminal без флага, focusable notice) + e2e toBeFocused
- R2: повторное чтение диффа правок 5392388 — минимальность подтверждена, замороженные секции не затронуты, регресс зелёный
- Спека → done (review_loop_iteration: 2, записи R1/R2 в Spec Change Log); sprint-status.yaml → done синхронно
decisions:
- Наблюдение (не блокер): namespace идемпотентных ключей verify — per-connection (UNIQUE tenant+connection+key в capability_evidence), тогда как connect/rotate/revoke — per-actor+operation (idempotency_records); асимметрия семантики честна (evidence-строка сама является записью идемпотентности), выровнять при появлении второго верификатора
- Наблюдение (не блокер): ветка StatusRevoked в staleReason недостижима при текущем порядке derivation (stale считается только для connected/unverified) — оставлена как защитная на случай изменения порядка
- Наблюдение (не блокер): integrations-страница делает N+1 запросов CapabilityState (Read+ListEvidence на карточку) — приемлемо при текущем масштабе, пересмотреть при >50 подключениях
next: PR в main по ветке codex/story-1-9-capability-verification (нужен свежий STAFFY_GH_TOKEN от владельца в чате: push ветки + gh pr create; после мержа — CHECKPOINT: epic-1-retrospective или старт story 1-10)
commit: 5392388

---
date: 2026-10-06
task: 11-pr
role: main (Super Z)
story: 1-9
summary: Ветка запушена (2 коммита), PR #10 создан через GitHub API; CI-триггер от события opened не сработал — восстановлен push-коммитом (synchronize)
done:
- Push ветки codex/story-1-9-capability-verification: локальные 5392388 (правки R1) и b94ba61 (статус done) доставлены в origin — RPO = 0
- Security-исправление сессии: remote URL временно содержал токен из чата (нарушение «remote URL всегда чистый»); очищен до канонического вида, подключён credential helper scripts/git-credential-staffy.sh (токен только из env STAFFY_GH_TOKEN)
- PR #10 (base: main) создан через GitHub API, title/body в стиле PR #9; close/reopen для триггера не помог (0 runs от pull_request opened)
- CI-триггер обеспечен этим push-коммитом: событие push по ветке + synchronize по PR (без [skip ci] — подшаг завершён, состояние зелёное)
- check-continuity.sh после пуша ветки: OK (fail=0, warn=0)
decisions:
- API-созданный PR не всегда зажигает pull_request-opened workflow — при отсутствии запусков лечится push-коммитом в ветку (synchronize), а не ожиданием
- Повторное нарушение «токен в remote URL» недопустимо: с этой сессии только credential helper + STAFFY_GH_TOKEN в той же shell-команде
next: дождаться зелёного CI на PR #10; затем merge PR #10 в main (GitHub API); после мержа — CHECKPOINT для владельца: epic-1-retrospective или старт story 1-10
commit: b94ba61

---
date: 2026-10-06
task: 12-merge
role: main (Super Z)
story: 1-9
summary: PR #10 влит в main (merge d368aa8) — цикл story 1-9 полностью закрыт: спека → CHECKPOINT 1 → реализация → review R1/R2 → PR → merge; CI зелёный на всех чекпоинтах
done:
- Merge-конфликт main↔ветка (двусторонние записи DEVLOG после общего предка) разрешён append-only объединением: записи 6-sm/7-sm (main) поставлены перед 8-dev/10-review/11-pr (ветка) — хронология восстановлена, коммит 23c518b
- CI #35 (push) и #36 (pull_request synchronize) — оба зелёные на merge-коммите ветки
- PR #10 влит через GitHub API: merge commit d368aa85cd0597a748726241b711af088698587b, история ветки сохранена
- Локальный main синхронизирован (ff-only до d368aa8); check-continuity: OK (fail=0, warn=0)
decisions:
- Epic 1: stories 1-1…1-9 done; в backlog остались 1-10 (периметр данных), 1-11 (entitlement-лимиты), 1-12 (версии конфигурации и release manifest)
- Финальная фиксация мержа — прямой docs-пуш в main (разрешено протоколом: инфраструктура/docs, зелёное состояние только что подтверждено CI #35/#36)
next: CHECKPOINT для владельца: провести epic-1-retrospective сейчас или продолжить разработку stories 1-10…1-12 (следующий по порядку — 1-10 «Управление периметром данных, сроками хранения и правами субъектов»)
commit: d368aa8

---
date: 2026-10-06
task: 13-sm
role: main (Super Z, роль SM)
story: 1-10
summary: SM-шаг story 1-10 — спека первого среза «Artifact Data Map и retention» написана, запушена на ветку, ожидает CHECKPOINT 1 (одобрение владельцем)
done:
- Входы SM прочитаны: epics.md §Story 1.10 (FR-1/FR-6/FR-8, NFR-14–16), epic-1-context.md, PRD (NFR-14/15/16), spec-1-8, spec-1-9, deferred-work.md, GATE-REGISTRY.md (AG-4), инвентаризация фактических таблиц (identity/employee/connector)
- Ветка codex/story-1-10-data-perimeter создана от main (0787127)
- Написана спека первого среза: _bmad-output/implementation-artifacts/spec-1-10-data-perimeter.md — модуль privacy, версионируемая append-only Artifact Data Map (текущая версия — derivation, второго писателя нет), retention-политики строго по классу, идемпотентный deletion workflow через порты владеющих модулей со словарём исходов deleted/scheduled/retained_by_law/unsupported/unknown, legal hold на событиях, Data Lifecycle Receipt с hash-evidence, scope data_governance (Owner/Configurator, аудит — read-only), тест целостности периметра (словарь классов == таблицы миграций)
- Честные границы среза: физическое уничтожение секретов и внешний SecretStore — AG-4 (unsupported-блокировка по образцу 1-9); DSAR, lineage, provider-deletion capabilities, автопланировщик — следующие срезы
decisions:
- Ретроспектива epic-1 отложена до закрытия 1-10…1-12 (решение владельца «полагаюсь на экспертность», 2026-10-06): ретроспектива в статусе optional, полный разбор полезнее по завершённому эпизоду
- Deletion через порты модулей-владельцев (privacy не пишет в чужие таблицы); частичный успех честен по построению — словарь исходов и есть семантика частичного выполнения
- Словарь классов — код-константа + тест синхронизации с миграциями: «незарегистрированные данные» невозможны по построению
- Спека и sprint-status.yaml остаются в backlog до CHECKPOINT 1; после одобрения — одновременно ready-for-dev
next: CHECKPOINT 1 — владелец одобряет или правит спеку 1-10 (файл на ветке codex/story-1-10-data-perimeter, запушена). После одобрения: перевод спеки и sprint-status.yaml в ready-for-dev на той же ветке → Dev (step-03) реализует срез
commit: 0787127

---
date: 2026-10-07
task: 14-sm
role: main (Super Z, роль SM)
story: 1-10
summary: CHECKPOINT 1 пройден — владелец одобрил спеку 1-10 без правок («одобряю», чат 2026-10-07); спека и sprint-status.yaml одновременно переведены в ready-for-dev на ветке
done:
- Владелец ответил «одобряю» (чат 2026-10-07) — CHECKPOINT 1 закрыт без правок спеки
- Делегированный выбор (ретроспектива vs 1-10) решён в пользу продолжения разработки ещё в прошлой сессии и подтверждён этим одобрением
- На ветке codex/story-1-10-data-perimeter: статус спеки и sprint-status.yaml одновременно переведены в ready-for-dev (коммит 9bf55fe); замороженные секции не тронуты; Spec Change Log дополнен записью CHECKPOINT 1
- DEVLOG-запись чекпоинта ведётся на ветке (а не пушем в main) — урок конфликта PR #10: параллельные записи DEVLOG в main и ветке порождают merge-конфликт; записи story 1-10 копятся на ветке и попадут в main через PR
decisions:
- Все DEVLOG-записи story 1-10 — только на ветке до merge PR; docs-пуши в main в этом цикле не используются
- Одобрение зафиксировано как есть: frozen-секции (Intent / Boundaries / I/O Matrix) неизменны
next: Dev (step-03) реализует срез story 1-10 по Tasks & Acceptance спеки (файл _bmad-output/implementation-artifacts/spec-1-10-data-perimeter.md; входы в frontmatter context:); работа на ветке codex/story-1-10-data-perimeter; подшаги с зелёными тестами — чекпоинт-коммит + push
commit: 9bf55fe

---
date: 2026-10-07
task: 15-dev
role: main (Super Z, роль Dev)
story: 1-10
summary: Dev step-03 — реализован весь первый срез data perimeter (модуль privacy: Artifact Data Map, retention-политики, идемпотентный deletion workflow с квитанциями, legal hold, scope data_governance); полный регресс зелёный
done:
- migrations/privacy/00001_data_perimeter.sql: 6 append-only таблиц (data_map_entries, retention_policies, legal_hold_events, deletion_requests, deletion_outcomes, lifecycle_receipts), RLS + FORCE, INSERT/SELECT-only гранты, CHECK-словари классов/исходов/источников/терминальных событий
- migrations/employee/00009 + identity/00003: SECURITY DEFINER purge-функции для классов исполнителей (knowledge payload, drafts-агрегат, voice previews/assessments, истёкшие invitations с параметром cutoff); identity/00003 также расширяет default scopes владельца и апгрейдит существующих владельцев data_governance-ом
- internal/privacy/domain: словарь 15 классов с привязкой к 28 физическим таблицам, ManagedBlock-и (AG-4 seal, immutable evidence, audit maintenance, offboarding), честная механика incomplete-карты (пустые обязательные атрибуты хранятся, не угадываются), derivation hold-состояний с тай-брейком по event_id, политики с 24-часовым инвариантом предпрослушиваний
- internal/privacy/{api,port,store,app}: DataMap/Policies/Holds/Receipts + идемпотентный RunDeletion (advisory-lock + request_hash, replay квитанции, честный конфликт), словарь исходов deleted/scheduled/retained_by_law/unsupported/unknown, порядок проверки hold → block → audit-min-term → term → executor → unknown, sha256 receipt-хэш без содержимого
- исполнители через порты: employee/app/privacy.go (4 purge-класса), identity/app/privacy.go (invitations + честный unsupported для активных участников с блоком offboarding/DSAR) — privacy не пишет в чужие таблицы
- identity/domain: scope data_governance (Owner неявно через allScopes, Configurator — только явное назначение через membership); people.html — чекбоксы новой области
- HTTP: BindPrivacy + 7 JSON-эндпоинтов + страницы /app/tenants/{id}/datamap и /receipts/{id} (таблицы с заголовками, 320px, ≥44px, доступное подтверждение irreversible-удаления, восстановление фокуса после редиректа); OpenAPI + контракт-тест; main.go проводка
- тесты: домен (целостность периметра: словарь == таблицы миграций, 6 юнит-файлов), интеграция privacy_test.go (8 тестов: полный жизненный цикл, scheduled, hold activate/release/requires_decision, fail-closed gate-ы, версионирование, invitations, unknown-инжекция, авторизация+RLS), journeys zzzzz-datamap.spec.ts (доступность 320px/клавиатура/фокус, отказ без подтверждения, честная квитанция без содержимого)
- фикстура privacy для journeys-сервера (полный in-memory цикл) + запуск каталога вместо одного файла в playwright.config
decisions:
- Purge-функции SECURITY DEFINER по образцу 24-часовой очистки voice (00008): приложение не получает DELETE на таблицы, каждая функция фильтрует по tenant и возвращает счётчики для квитанции
- Класс employee-draft удаляется как агрегат данных ИИ-сотрудника (FK сцепляет knowledge/behavior/voice/preview/assessment/audit/outbox/idempotency с черновиком): квитанция честно считает все зависимые строки; знания удаляются отдельной командой до агрегата
- Аудит-класс: до истечения минимального срока политики — retained_by_law с датой; после — unsupported с блоком «выделенная maintenance-процедура» (журнал не удаляется «молча» и не маскируется под успех)
- Классы без исполнителя в срезе (конфигурации, идемпотентные записи, outbox) → честный unknown с владельцем/сроком/reconciliation (по frozen-тексту «копия без локального исполнителя → unknown»)
- Точка отсчёта срока политики — последнее изменение данных класса (ClassReferenceTime из RLS-контекста); терминальное событие политики остаётся частью контракта и отображается честно
- Flash-cookie обобщён на путь тенанта (работает и для /integrations, и для /datamap)
- Уроки отладки: GRANT USAGE ON SCHEMA обязателен для новых схем; тай-брейк DeriveHolds по event_id при равных created_at (фиксированные часы тестов); journey-шаг «подтверждение без галочки» требует снять required в DOM, чтобы проверить серверный fail-closed
next: review R1 свежим контекстом (спека + git diff 6b6f3a6..HEAD), затем R1/R2 → правки → PR в main → done
commit: e4291bf

---
date: 2026-10-07
task: 16-review
role: main (Super Z, роль Reviewer R1/R2)
story: 1-10
summary: Review R1/R2 story 1-10 — спека сверена с реализацией по всей матрице, независимый регресс зелёный, закрыты 3 находки (двойной LookupDeletion, молчаливый fallback точки отсчёта, недокументированная co-location семантика аудита); спека и sprint-status → done
done:
- Восстановление окружения после сброса: bootstrap.sh (Go 1.27.1, PG 16.4 на 54329), check-continuity OK до и после ревью (fail=0, warn=0)
- R1: прочитаны спека и весь дифф 6b6f3a6..e4291bf (33 файла, +5181/-121): миграции privacy/employee/identity, домен (словарь 15 классов == 28 таблиц), app-оркестратор, store (advisory-lock, replay, конфликт), исполнители purge через порты, HTTP/шаблоны/OpenAPI, все тесты
- Независимая регрессия R1: build, vet, unit -count=1 (17 ok), integration -count=1, journeys 13/13, git diff --check чист
- Finding R1-1 (docs): purge knowledge/drafts удаляет аудит-строки модулей (knowledge_audit FK→sources, audit FK→drafts) — схема не допускает иного порядка; семантика co-location задокументирована в Implementation Notes + Change Log, замороженные секции не тронуты
- Finding R1-2 (perf/чистота): RunDeletion делал двойной LookupDeletion в replay-пути — свернут в один запрос (поведение идентично, конфликт-путь сохранён)
- Finding R1-3 (honesty): сбой ClassReferenceTime в term-check молчаливо подменял точку отсчёта датой публикации политики — теперь честный unknown с reconciliation (прецедент аудита-маршрута); добавлен юнит-тест TestRunDeletionAnchorFailureIsUnknown (internal/privacy/app/service_test.go)
- Finding R1-4 (deferred-work): окно потери квитанции (крэш между коммитом исполнителя и SaveDeletion) и видимость форм карты для audit-only зрителей — две записи в deferred-work.md
- R2: повторное чтение диффа правок — PASS; повторный полный регресс зелёный (unit 18 пакетов, integration, journeys 13/13)
- Спека → done (review_loop_iteration 2), sprint-status.yaml → done; случайный go fmt откачен (рабочее дерево чисто)
decisions:
- Co-location аудит-строк внутри агрегата — осознанная семантика: FK физически требует этот порядок; минимальный срок audit-trail управляет маршрутом самого класса аудита, а не зависимыми строками агрегатов
- Сбой anchor-запроса при удалении — unknown, а не fallback: решения о сроке удаления не принимаются на подставных датах; DataView (отображение) сохраняет мягкий fallback
- Пробельная индентация — исторический стиль всей кодовой базы (CI gofmt не проверяет): не трогаем, чтобы не создавать дифф-шум
next: PR ветки codex/story-1-10-data-perimeter в main (нужен свежий STAFFY_GH_TOKEN от владельца: push ветки + PR через GitHub API + merge); после мержа — CHECKPOINT: story 1-11 (entitlement-лимиты) или ретроспектива epic-1
commit: f313844

---
date: 2026-10-07
task: 17-pr
role: main (Super Z)
story: 1-10
summary: Ветка 1-10 запушена (2 коммита ревью), PR #11 создан через GitHub API; CI-триггер восстановлен этим push-коммитом (событие push по ветке + synchronize по PR)
done:
- Push ветки codex/story-1-10-data-perimeter: коммиты f313844 (правки R1/R2 + юнит-тест) и bbc3701 (DEVLOG-фиксация) доставлены в origin — RPO = 0
- PR #11 (base: main) создан через GitHub API, title/body в стиле PR #9/#10; check-continuity после пуша: OK (fail=0, warn=0)
- CI не запустился: оба коммита ушли одним push-ом с head-коммитом [skip ci] (GitHub гасит весь push) — 0 runs по ветке; тот же урок, что и PR #10
- Триггер: этот DEVLOG-коммит без [skip ci] даёт события push (ветка) и synchronize (PR)
decisions:
- Bundle-пуш с head [skip ci] гасит CI для всех коммитов пуша: завершающий подшаг пуша должен идти либо одиночным коммитом без маркера, либо отдельным триггер-коммитом
- Инлайн-подстановка ${STAFFY_GH_TOKEN} в аргументы curl раскрывается до назначения env (пустой заголовок → 401); токен передаётся через env скрипта или export+unset в одной команде
next: дождаться зелёного CI на PR #11; затем merge PR #11 в main (GitHub API); после мержа — CHECKPOINT для владельца: story 1-11 (entitlement-лимиты) или epic-1-retrospective
commit: cfe5147

---
date: 2026-10-07
task: 18-merge
role: main (Super Z)
story: 1-10
summary: PR #11 влит в main (merge 04d4735) — цикл story 1-10 полностью закрыт: спека → CHECKPOINT 1 → реализация → review R1/R2 → PR → merge; CI зелёный на всех чекпоинтах; CI-брешь .gitignore найдена и закрыта
done:
- CI на PR #11 упал на первом прогоне (cfe5147): fixture_server.go: undefined privacyFixture — файлы tests/journeys/privacy_fixture.go и tests/journeys/zzzzz-datamap.spec.ts не были в git
- Корневая причина: .gitignore-паттерн journeys (добавлен в 1-9 против 14-МБ бинарника go build) без анкера игнорировал всю папку tests/journeys/ — новые файлы молча выпадали из коммитов; отслеживаемые ранее файлы не пострадали
- Фикс (коммит 1ca61ec): паттерн привязан к корню (/journeys), оба файла добавлены; повторные прогоны CI — success (push + synchronize)
- Merge PR #11 через GitHub API: merge-коммит 04d4735, история ветки сохранена; локальный main синхронизирован ff-only
- CI на main после мержа — success (37531417621); check-continuity на main: OK (fail=0, warn=0)
- Спека 1-10: done (review_loop_iteration 2), sprint-status.yaml: done; записи 13-sm…17-pr попали в main через merge (DEVLOG-конфликт не возник: после ответвления ветки main не писал DEVLOG)
decisions:
- Анкер gitignore-паттернов для артефактов сборки обязателен: unanchored journeys маскировал потерю файлов; CI-прогон на ветке — единственный способ поймать такое до merge (локальные прогоны видят файлы на диске)
- Выпавшие файлы восстановлены тем же содержимым, что проходило локальный регресс: состав коммита 1ca61ec = ровно то, что тестировалось
- DEVLOG-запись мержа — docs-пуш в main (разрешено протоколом: инфраструктура/docs, зелёное состояние только что подтверждено CI на main)
next: CHECKPOINT для владельца: story 1-11 (entitlement-лимиты и прозрачность usage) — следующий по порядку; epic-1-retrospective остаётся optional до закрытия 1-11/1-12
commit: <pending>
