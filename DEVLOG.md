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
