#!/usr/bin/env bash
# ============================================================================
# session-start — быстрый вход в сессию staffy: свод позиции из источников
# истины, одной командой, за секунды.
#
# ЗАЧЕМ: сводка контекста диалога (авто-компакция платформы) — снимок ДИАЛОГА,
# а не файлов: она не видит коммиты, сделанные после её генерации, и наследует
# утверждения предшественниц. Устарела — значит ложна по умолчанию. Истина
# живёт в файлах: git > sprint-status.yaml > DEVLOG > пересказ. Этот скрипт —
# дешёвая фиксация позиции (шаг «тёплого старта», docs/CONTINUITY.md §1.5),
# исполненная командой вместо ручного разбора.
#
# Использование:
#   bash scripts/session-start.sh            # из любого каталога клона
#   WORKLOG=... bash scripts/session-start.sh   # нестандартный worklog
#
# Свойства: read-only (ничего не меняет, кроме git fetch); не требует
# токена/сети/окружения (деградирует честно: скажет, чего не удалось);
# идемпотентен. bootstrap.sh поднимает ОКРУЖЕНИЕ (~1 мин), session-start
# показывает ПОЗИЦИЮ (~сек) — сначала позиция, потом окружение.
# ============================================================================
set -uo pipefail

# --- Где репо: переопределение → git toplevel → сосед с скриптом ------------
REPO_DIR="${REPO_DIR:-}"
if [ -z "$REPO_DIR" ]; then
  REPO_DIR="$(git rev-parse --show-toplevel 2>/dev/null)" || true
fi
if [ -z "$REPO_DIR" ] || [ ! -d "$REPO_DIR" ]; then
  REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
fi
cd "$REPO_DIR" 2>/dev/null || { echo "[session-start] ERROR: не нашёл репозиторий ($REPO_DIR)" >&2; exit 1; }

WORKLOG="${WORKLOG:-$(dirname "$REPO_DIR")/worklog.md}"
SPRINT="_bmad-output/implementation-artifacts/sprint-status.yaml"

say()  { echo; echo "$*"; }
hdr()  { say "== $* =="; }   # НЕ называть head/tail — затеняют команды в пайпах

say "[session-start] staffy — свод позиции из источников истины ($(date '+%F %R'))"
echo   "ДИСКЛЕЙМЕР: сводка диалога/память сессии — НЕ истина (снимок разговора, стареет"
echo   "с каждым коммитом). Истина = этот свод + сами файлы. Порядок доверия:"
echo   "git (origin) > sprint-status.yaml > DEVLOG > пересказ диалога."

# --- 1. GIT (факт) -----------------------------------------------------------
hdr "1. GIT (факт)"
if timeout 15 git fetch --quiet origin 2>/dev/null; then
  echo "fetch origin: ок (origin/* свежие)"
else
  echo "fetch origin: недоступен (нет сети/токена) — сверяюсь с локальным origin/*"
fi
branch="$(git branch --show-current)"
[ -n "$branch" ] || branch="(detached HEAD)"
echo "ветка: $branch | HEAD: $(git log -1 --format='%h (%ad)' --date=format:'%F %R' 2>/dev/null)"
if [ -n "$(git status --porcelain)" ]; then
  echo "дерево: ГРЯЗНОЕ — это WIP предшественника! Сначала tail -n 60 DEVLOG.md,"
  echo "        затем немедленный чекпоинт-коммит WIP, потом разбирайся (AGENTS.md):"
  git status --short | head -15
else
  echo "дерево: чистое"
fi
if git rev-parse "origin/$branch" >/dev/null 2>&1; then
  ahead=$(git rev-list --count "origin/$branch..HEAD" 2>/dev/null || echo "?")
  behind=$(git rev-list --count "HEAD..origin/$branch" 2>/dev/null || echo "?")
  if [ "$ahead" = "0" ] && [ "$behind" = "0" ]; then
    echo "RPO: 0 — синхронизировано с origin/$branch"
  else
    echo "ВНИМАНИЕ: опережаем origin на $ahead, отстаём на $behind — НЕ ЗАПУШЕНО или УСТАРЕЛО (правило: не запушено — не существует)"
  fi
else
  echo "origin/$branch: нет локального remote-рефа (fetch не удался?)"
fi
git log --format='%h %ad %s' --date=short -5 | cut -c1-160 | sed 's/^/  /'

# --- 2. DEVLOG (указатель продолжения) ---------------------------------------
hdr "2. DEVLOG (указатель продолжения)"
if [ -f DEVLOG.md ]; then
  last_date="$(awk '/^date: /{d=$2} END{print d}' DEVLOG.md | tr -d '\r')"
  last_task="$(awk '/^task: /{t=$2} END{print t}' DEVLOG.md | tr -d '\r')"
  last_sha="$(awk '/^commit: /{v=$2} END{print v}' DEVLOG.md | tr -d '\r')"
  echo "последняя запись: task ${last_task:-?} от ${last_date:-?}"
  if [ -n "$last_sha" ] && git cat-file -e "${last_sha}^{commit}" 2>/dev/null; then
    echo "commit $last_sha из записи: присутствует в истории — OK"
  else
    echo "commit ${last_sha:-<пусто>} из записи: НЕ НАЙДЕН локально (расхождение! верь git, correction-запись)"
  fi
  echo "next: $(grep '^next: ' DEVLOG.md | tail -1 | sed 's/^next: //' | tr -d '\r')"
else
  echo "DEVLOG.md не найден — репо в нештатном состоянии, смотри AGENTS.md"
fi

# --- 3. SPRINT-STATUS (канон статусов BMAD) -----------------------------------
hdr "3. SPRINT-STATUS (канон статусов BMAD)"
if [ -f "$SPRINT" ]; then
  sed -n '/^development_status:/,$p' "$SPRINT"
else
  echo "sprint-status.yaml не найден"
fi

# --- 4. WORKLOG песочницы (черновик агента; НЕ источник истины) --------------
hdr "4. WORKLOG песочницы (черновик; не источник истины)"
if [ -f "$WORKLOG" ]; then
  echo "хвост ($WORKLOG):"
  tail -n 25 "$WORKLOG" | sed 's/^/  /'
else
  echo "отсутствует ($WORKLOG) — ок: он машино-локален и погибает при сбросе окружения"
fi

# --- 5. CHECK-CONTINUITY (сверка консистентности) -----------------------------
hdr "5. CHECK-CONTINUITY (сверка)"
if [ -x scripts/check-continuity.sh ] || [ -f scripts/check-continuity.sh ]; then
  bash scripts/check-continuity.sh 2>&1 | sed 's/^/  /' || true
else
  echo "scripts/check-continuity.sh не найден"
fi

# --- Дальше -------------------------------------------------------------------
hdr "ДАЛЬШЕ (протокол — AGENTS.md)"
if [ -f scripts/env.sh ]; then
  echo "- Окружение: source scripts/env.sh (go/PG); если мертво — STAFFY_GH_TOKEN=*** bash scripts/bootstrap.sh"
else
  echo "- scripts/env.sh отсутствует — окружение не поднято: STAFFY_GH_TOKEN=*** bash scripts/bootstrap.sh (~1 мин)"
fi
echo "- Продолжать с next: (раздел 2 выше). Расхождение источников → верь git + correction-запись в DEVLOG."
