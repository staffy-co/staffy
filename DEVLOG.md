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
