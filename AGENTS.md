## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

When the user types `/graphify`, use the installed graphify skill or instructions before doing anything else.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- Keep the Staffy graph limited to project payload: `_bmad-output/**`, root-level project source material, and product code. Never index `.agents/**`, `_bmad/**`, `.codex/**`, `AGENTS.md`, or Graphify-generated files.
- Store Graphify query feedback with `graphify save-result ... --memory-dir .graphify-memory` and run reflections with `graphify reflect --memory-dir .graphify-memory --out graphify-out/reflections/LESSONS.md`. Do not use the default `graphify-out/memory`, because Graphify force-includes that directory in later scans.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).

## staffy — протокол непрерывности для ИИ-агентов

### Миссия
staffy — ИИ-сотрудник для малого бизнеса (Go + PostgreSQL, server-side HTML). Разработка — по BMAD METHOD; артефакты процесса в `_bmad-output/`.

### Коммуникация с владельцем (постоянное правило)
Владелец — НЕ технический специалист и не программист. Все объяснения для владельца — простым языком, без непояснённого жаргона: каждый технический термин расшифровывать «на пальцах» (аналогия → смысл → зачем это нам). Формат: краткое «что это значит» → «что я сделал» → «что дальше». Владелец учится на процессе — по ходу отмечать нюансы, которые стоит запомнить на будущее. Техническая строгость остаётся в артефактах (спеки, DEVLOG, коммиты); чат с владельцем — на понятном языке.

### Где мы сейчас — НЕ в этом файле
Этот файл статичен: не редактируй его под каждую сессию. Позиция живёт в `_bmad-output/implementation-artifacts/sprint-status.yaml` (канон статусов) и в поле `next:` последней записи `DEVLOG.md`. Порядок доверия: **git (origin) > sprint-status.yaml > DEVLOG > пересказ диалога** (пересказ — только подсказка, всё проверяй файлами). Свод позиции из всех источников одной командой: `bash scripts/session-start.sh` (git-факты + `next:` DEVLOG + sprint-status + worklog + check-continuity; секунды, read-only).

### Резюме контекста (сводка диалога) — НЕ источник истины
Сводка диалога (авто-компакция платформы) — снимок РАЗГОВОРА, а не файлов: она не видит коммиты, сделанные после её генерации, и наследует утверждения предшественниц без опровержения. Устарела — значит ложна по умолчанию (фиксации устаревания — в DEVLOG). Правила:
1. Позицию (story/шаг/ветка/HEAD) при генерации резюме-для-продолжения бери ТОЛЬКО из живых файлов момента генерации (`bash scripts/session-start.sh`), никогда — из предыдущей сводки
2. Описывай СОСТОЯНИЕ (что слито/запушено/зелёное), а не планы («заверши шаг X» стареет мгновенно)
3. Вставляй дисклеймер: «позиции в сводке не истина — первое действие преемника: `bash scripts/session-start.sh`»
4. Утверждение предшественницы, противоречащее git-фактам, НЕ наследуется — пиши git-факт

### Протокол возобновления (выполнять по порядку)
1. `bash scripts/session-start.sh` — свод позиции из источников истины (секунды, read-only): git-факты (fetch/дерево/RPO), последняя запись DEVLOG (`next:` + `commit:`), sprint-status, worklog, check-continuity. Сводка диалога (если сессия началась после компакции) — НЕ истина
2. Окружение: живо (`source scripts/env.sh`, go/PG отвечают) → дальше; мертво или новая машина → `STAFFY_GH_TOKEN=<токен от владельца> bash scripts/bootstrap.sh` (~1 мин, идемпотентно), затем `source scripts/env.sh`
3. Сбой инструментов сессии (403/broken session/Bash «умер»): после 2 отказов подряд — СТОП повторам. Живость и работа — через субагентов (Task: у них СВОЙ свежий шелл); тяжёлые задачи дробить с ранним `wip [SOS]`-коммитом+пушем (платформенный дедлайн субагента ~10 мин; писатель умер → грязное дерево = orphan checkpoint, преемник довершает). ⚠️ ЗАПРЕЩЕНЫ пробы порта через /dev/tcp — дважды вешали сессии намертво; живость PG — `pg_ctl status`. После рестарта/продолжения сессии инструменты ОЖИВАЮТ — первым делом прямой зонд `echo alive`: живо → работай сам, мертво → субагенты. Полный протокол: docs/CONTINUITY.md §4.1
4. Грязное дерево = WIP предшественника: сначала `tail -n 60 DEVLOG.md`, затем немедленный чекпоинт-коммит WIP, потом разбирайся
5. Расхождение git/sprint-status/DEVLOG → верь git; зафиксируй `correction:`-запись в DEVLOG
6. Продолжай с `next:` (BMAD-процесс, роль указана в записи)

### BMAD-контур
- Цикл story: SM (bmad-build step-01/02) пишет спеку → CHECKPOINT 1 (одобрение владельца) → `ready-for-dev` → Dev (step-03) на ветке `codex/story-<N>-<slug>` → review R1/R2 свежим контекстом → `done` → PR → merge в main
- Dev читает ТОЛЬКО спеку (source of truth) + файлы из её frontmatter `context:`; Reviewer — спеку + `git diff <baseline_commit>..HEAD`
- Артефакты статуса (спека, sprint-status.yaml, deferred-work.md) коммитятся В ТОМ ЖЕ PR, что и код
- Отложенное — только в `_bmad-output/implementation-artifacts/deferred-work.md` (append-only)

### Ритуалы фиксации (обязательны)
- **Подшаг завершён** (тесты зелёные): запись в DEVLOG → conventional commit → push. WIP-пуши помечай `[skip ci]`
- **Угроза исчерпания контекста** (~60 сек): `git add -A && git commit -m "wip: <шаг> [SOS]" && git push` + запись `[SOS]` в DEVLOG с точным `next:`
- **Конец сессии**: финальная запись DEVLOG → push → `bash scripts/check-continuity.sh` → убедиться `git status -sb` чист и не ahead
- **Отказ push = СТОП**: не начинай новый подшаг; запиши `unpushed: N` в DEVLOG; запроси токен у владельца
- Правило: **незакоммичено или не запушено — не существует**

### Правила git
- main — только через PR (реальный, через GitHub API). Локальный мерж без PR — с честным сообщением `merge: story N-N into main (local merge, no PR)`; ЗАПРЕЩЕНО имитировать «Merge pull request #N» спуфом автора
- Прямой push в main — только для инфраструктуры (scripts/, docs/, .gitignore, hooks) и только зелёным состоянием
- Ветки: `codex/story-<N>-<slug>`; conventional commits; force-push в main запрещён; `--no-verify` запрещён (`.githooks/` сканирует секреты)
- **Слитая ветка — удаляется сразу после merge** (правило владельца, 2026-10-25): как только PR story слит в main, удаляй её ветку в трёх местах: (1) на GitHub `DELETE /repos/{owner}/{repo}/git/refs/heads/<branch>` (проверив сначала compare main...branch: `ahead_by=0`), (2) локально `git branch -d <branch>`, (3) stale remote-tracking рефы `git update-ref -d refs/remotes/origin/<branch>`. Репозиторий держит только `main` + активные (неслитые) ветки. «Удаление слитой ветки — по поручению» в DEVLOG устарело: теперь это штатный ритуал merge

### Окружение (поднимает scripts/bootstrap.sh)
- Go (пинн в скрипте; живёт в `~/.local/go/bin` — вне системного PATH)
- PostgreSQL 16.4 портативный: вендорен в `third_party/portable-pg16.tar.xz` → распаковка в `~/.local/pg`; данные `~/.local/pgdata`; порт 54329; БД `staffy_test` (интеграция, disposable) и `staffy_dev` (ручные прогоны); **psql в дисте НЕТ** — SQL через single-user mode
- Миграции применяет сам integration-суит (`go test -tags=integration ./tests/integration/...`, goose-формат, drop+re-apply)
- e2e: `tests/journeys` (npm ci; Playwright chromium в `~/.cache/ms-playwright`)
- CI: `.github/workflows/ci.yml` на каждый push/PR (полный конвейер Go+PG+Playwright) — поэтому WIP-пуши помечай `[skip ci]`

### Секреты
- GitHub PAT: только от владельца в чате на сессию → env `STAFFY_GH_TOKEN`; remote URL всегда чистый (helper: `scripts/git-credential-staffy.sh`); токен НИКОГДА в файлы/коммиты/DEVLOG/логи — в отчётах только маска `github_pat_***`
- Секреты подключений продукта — через SecretStore (см. спеки)
