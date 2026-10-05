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
