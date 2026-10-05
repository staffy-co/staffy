#!/usr/bin/env bash
# ============================================================================
# staffy bootstrap — идемпотентное восстановление окружения разработки.
#
# Использование:
#   bash scripts/bootstrap.sh                       # лёгкий режим (репо есть → обновить)
#   STAFFY_GH_TOKEN=... bash scripts/bootstrap.sh  # полное восстановление (клон с GitHub)
#   bash scripts/bootstrap.sh /путь/к/клону        # нестандартный целевой каталог
#
# Переопределяемые переменные (для drill/тестов — см. docs/CONTINUITY.md):
#   REPO_DIR, GO_DIR, GO_PIN, PG_HOME, PG_DATA, PG_PORT, PG_SOCK_DIR,
#   STAFFY_GH_TOKEN, SKIP_INTEGRATION=1
#
# Свойства: идемпотентен (повторный запуск безопасен); relocatable (все пути
# через переменные); атомарные загрузки (.part + rename); самопроверка шагов.
# Полный разбор и сценарий учений (drill): docs/CONTINUITY.md.
# ============================================================================
set -euo pipefail

REPO_URL="https://github.com/artemiosu/staffy.git"
REPO_DIR="${REPO_DIR:-${1:-/home/z/my-project/staffy}}"
GO_DIR="${GO_DIR:-$HOME/.local/go}"
GO_PIN="${GO_PIN:-1.27.1}"
PG_HOME="${PG_HOME:-$HOME/.local/pg}"
PG_DATA="${PG_DATA:-$HOME/.local/pgdata}"
PG_PORT="${PG_PORT:-54329}"
PG_SOCK_DIR="${PG_SOCK_DIR:-/tmp}"
DB_TEST="${DB_TEST:-staffy_test}"
DB_DEV="${DB_DEV:-staffy_dev}"
SKIP_INTEGRATION="${SKIP_INTEGRATION:-0}"

log() { echo -e "\n[bootstrap] $*"; }
die() { echo "[bootstrap] ERROR: $*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# 1. Код: клон (полное восстановление) или обновление (лёгкий режим)
# ---------------------------------------------------------------------------
if [ ! -d "$REPO_DIR/.git" ]; then
  if [ -n "${STAFFY_GH_TOKEN:-}" ]; then
    log "Клонирую репозиторий (по токену) → $REPO_DIR"
    mkdir -p "$(dirname "$REPO_DIR")"
    git -c credential.helper= clone "https://x-access-token:${STAFFY_GH_TOKEN}@github.com/artemiosu/staffy.git" "$REPO_DIR"
  else
    die "репозиторий отсутствует в $REPO_DIR и нет STAFFY_GH_TOKEN — запроси токен у владельца (выдаётся в чате на сессию)"
  fi
fi
cd "$REPO_DIR"
log "Код: ветка $(git branch --show-current), HEAD $(git rev-parse --short HEAD)"

if [ -n "$(git status --porcelain)" ]; then
  log "ВНИМАНИЕ: рабочее дерево ГРЯЗНОЕ — это WIP предшественника."
  log "Действия: 1) tail -n 60 DEVLOG.md; 2) немедленный чекпоинт-коммит WIP; 3) продолжить с next: (см. docs/CONTINUITY.md, ритуал «orphan checkpoint»)."
  git status --short | head -20
else
  if git fetch origin 2>/dev/null; then
    git pull --ff-only 2>/dev/null && log "git pull --ff-only: ок" || log "WARN: pull --ff-only не удался (расхождение?) — разбирайся вручную, не резет"
  else
    log "WARN: git fetch не удался (нет токена/сети) — работаю с локальной копией"
  fi
fi

# ---------------------------------------------------------------------------
# 2. Git-конфигурация: hooks (скан секретов), credential helper, чистый remote
# ---------------------------------------------------------------------------
log "Настройка git: core.hooksPath, credential helper, чистый remote URL"
git config core.hooksPath .githooks
if [ -n "${STAFFY_GH_TOKEN:-}" ]; then
  git config credential.https://github.com.helper "$REPO_DIR/scripts/git-credential-staffy.sh"
fi
current_url=$(git remote get-url origin 2>/dev/null || echo "")
case "$current_url" in
  *github.com*) [ "$current_url" = "$REPO_URL" ] || git remote set-url origin "$REPO_URL" ;;
esac

# ---------------------------------------------------------------------------
# 3. Go (в песочнице go НЕ в системном PATH — живёт в ~/.local/go/bin)
# ---------------------------------------------------------------------------
if ! command -v go >/dev/null 2>&1 && [ -x "$GO_DIR/bin/go" ]; then
  export PATH="$GO_DIR/bin:$PATH"
fi
if ! command -v go >/dev/null 2>&1; then
  log "Go не найден — устанавливаю go$GO_PIN с go.dev в $GO_DIR"
  mkdir -p "$HOME/.local"
  curl -fsSL -o /tmp/go.tgz.part "https://go.dev/dl/go${GO_PIN}.linux-amd64.tar.gz"
  mv /tmp/go.tgz.part /tmp/go.tgz
  rm -rf "$GO_DIR"
  tar -xzf /tmp/go.tgz -C "$HOME/.local"
  rm -f /tmp/go.tgz
  export PATH="$GO_DIR/bin:$PATH"
  go env -w GOTOOLCHAIN=local
fi
command -v go >/dev/null 2>&1 || die "Go недоступен после установки"
log "Go: $(go version)"

# ---------------------------------------------------------------------------
# 4. PostgreSQL 16.4: портативный дистрибутив из third_party/ (вендорен в репо,
#    внешний источник EDB отдаёт 403; /tmp не переживает сброс окружения)
# ---------------------------------------------------------------------------
if [ ! -x "$PG_HOME/bin/postgres" ]; then
  log "PostgreSQL: распаковываю third_party/portable-pg16.tar.xz → $PG_HOME"
  mkdir -p "$PG_HOME"
  tar -xJf "$REPO_DIR/third_party/portable-pg16.tar.xz" -C "$PG_HOME"
fi
[ -x "$PG_HOME/bin/postgres" ] || die "postgres-бинарник отсутствует после распаковки"
log "PostgreSQL: $($PG_HOME/bin/postgres --version) ($PG_HOME)"

server_running() { "$PG_HOME/bin/pg_ctl" -D "$PG_DATA" status >/dev/null 2>&1; }

if [ ! -f "$PG_DATA/PG_VERSION" ]; then
  log "Инициализирую кластер: $PG_DATA (initdb, auth=trust, UTF8)"
  mkdir -p "$(dirname "$PG_DATA")"
  "$PG_HOME/bin/initdb" -D "$PG_DATA" -U postgres --auth=trust -E UTF8 \
    > "$PG_DATA.initdb.log" 2>&1 || { tail -20 "$PG_DATA.initdb.log"; die "initdb упал (лог: $PG_DATA.initdb.log)"; }
fi

# Создание БД: single-user mode (psql в минимальном дисте НЕТ; сервер должен быть остановлен)
ensure_db() {
  local db="$1" out
  out=$(printf 'CREATE DATABASE %s;\n' "$db" | "$PG_HOME/bin/postgres" --single -D "$PG_DATA" -E postgres 2>&1) && \
    log "БД $db создана" || {
      if echo "$out" | grep -q "already exists"; then log "БД $db уже есть"; else echo "$out" | tail -5 >&2; die "не удалось создать БД $db"; fi
    }
}

if server_running; then
  log "PostgreSQL уже запущен (порт $PG_PORT) — ок"
else
  ensure_db "$DB_TEST"
  ensure_db "$DB_DEV"
  log "Запускаю PostgreSQL: порт $PG_PORT, сокет $PG_SOCK_DIR, только 127.0.0.1"
  "$PG_HOME/bin/pg_ctl" -D "$PG_DATA" -w -t 30 -l "$PG_DATA/server.log" \
    -o "-p $PG_PORT -k $PG_SOCK_DIR -h 127.0.0.1" start >/dev/null 2>&1 || \
    { tail -20 "$PG_DATA/server.log" 2>/dev/null; die "pg_ctl start упал (лог: $PG_DATA/server.log)"; }
fi

# ---------------------------------------------------------------------------
# 5. Миграции + интеграционные тесты: канонический applier — сам TestMain
#    (goose-формат migrations/*; drop+re-apply; БД $DB_TEST — disposable)
# ---------------------------------------------------------------------------
TEST_URL="postgres://postgres@127.0.0.1:${PG_PORT}/${DB_TEST}?sslmode=disable"
if [ "$SKIP_INTEGRATION" = "1" ]; then
  log "SKIP_INTEGRATION=1 — пропускаю миграции/интеграцию (БД может быть не инициализирована)"
else
  log "Миграции + интеграционные тесты (применяет TestMain; ~1 мин)"
  if ! STAFFY_TEST_DATABASE_URL="$TEST_URL" go test -tags=integration ./tests/integration/... > "$PG_DATA.integration.log" 2>&1; then
    tail -30 "$PG_DATA.integration.log"
    die "интеграционные тесты упали — миграции/БД не в порядке (лог: $PG_DATA.integration.log)"
  fi
  log "Интеграция: ok (схема применена, роль staffy_app готова)"
fi

# ---------------------------------------------------------------------------
# 6. e2e-зависимости (Playwright; PG им не нужен — fixture-сервер in-memory)
# ---------------------------------------------------------------------------
if [ -f tests/journeys/package.json ]; then
  if [ ! -d tests/journeys/node_modules ]; then
    log "npm ci (tests/journeys)"
    (cd tests/journeys && npm ci --silent) || log "WARN: npm ci упал — e2e недоступны, остальное работает"
  fi
  if [ ! -d "$HOME/.cache/ms-playwright" ] && [ -d tests/journeys/node_modules ]; then
    log "Playwright chromium (первая установка, ~200 МБ)"
    (cd tests/journeys && npx playwright install chromium) || log "WARN: chromium не установлен — e2e отложены (CI прогонит)"
  fi
fi

# ---------------------------------------------------------------------------
# 7. Smoke: сборка + vet + unit-тесты
# ---------------------------------------------------------------------------
log "go build ./... && go vet ./..."
go build ./... || die "go build упал"
go vet ./... || die "go vet упал"
log "go test ./... (unit)"
go test ./... > "$PG_DATA.unit.log" 2>&1 || { tail -30 "$PG_DATA.unit.log"; die "unit-тесты упали (лог: $PG_DATA.unit.log)"; }

# ---------------------------------------------------------------------------
# 8. scripts/env.sh — машино-локальное окружение (генерируется, в .gitignore)
# ---------------------------------------------------------------------------
cat > "$REPO_DIR/scripts/env.sh" <<ENV
# Сгенерировано scripts/bootstrap.sh $(date '+%F %R') — машино-локально, НЕ коммитить.
export PATH="$GO_DIR/bin:\$PATH"
export PG_HOME="$PG_HOME"
export PG_DATA="$PG_DATA"
export PG_PORT="$PG_PORT"
export STAFFY_TEST_DATABASE_URL="postgres://postgres@127.0.0.1:$PG_PORT/$DB_TEST?sslmode=disable"
export STAFFY_DEV_DATABASE_URL="postgres://postgres@127.0.0.1:$PG_PORT/$DB_DEV?sslmode=disable"
# Runtime-сервер (ручной прогон; principal — из cmd/staffy-core/main.go):
#   STAFFY_ENV=development STAFFY_DEV_PRINCIPAL_ID=<dev-principal-id> \
#   STAFFY_DATABASE_URL="postgres://staffy_app:staffy-runtime-test@127.0.0.1:$PG_PORT/$DB_TEST?sslmode=disable" \
#   go run ./cmd/staffy-core
ENV

# ---------------------------------------------------------------------------
# 9. Точка продолжения (для агента-преемника)
# ---------------------------------------------------------------------------
log "=== ГОТОВО. Точка продолжения: ==="
echo "— Канон статусов: _bmad-output/implementation-artifacts/sprint-status.yaml"
sed -n '/^development_status:/,/^  epic-2:/p' _bmad-output/implementation-artifacts/sprint-status.yaml | head -25 || true
echo "— Журнал (последняя запись):"
tail -n 30 DEVLOG.md 2>/dev/null || echo "  DEVLOG.md не найден"
echo "— git: $(git log --oneline -3 | tr '\n' ' | ')"
echo "— Далее: source scripts/env.sh; продолжить с next: последней записи DEVLOG.md (протокол — AGENTS.md)."
