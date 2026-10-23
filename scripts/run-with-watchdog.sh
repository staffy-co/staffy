#!/bin/sh
# run-with-watchdog.sh — liveness watchdog of the long local runs (the
# infra of 2026-10-23, the owner request: «проверяющий каждую минуту что
# выполнение не висит»). The tool layer caps one command at 10 minutes
# and an over-cap request is canceled by the platform — a long run needs
# a verifiable heartbeat, not a blind wait.
#
# Usage:
#   scripts/run-with-watchdog.sh <label> <stale-minutes> <command...>
#
# Env:
#   WATCHDOG_INTERVAL  poll seconds (default 60 — the every-minute check)
#   WATCHDOG_MAX_MIN   hard cap in minutes, 0 = disabled (default 0)
#   WATCHDOG_DIR       artifact dir (default /tmp)
#
# Artifacts:
#   <dir>/watchdog-<label>.log     the command output (tail for progress)
#   <dir>/watchdog-<label>.watch   the watchdog decisions (the heartbeat)
#   <dir>/watchdog-<label>.status  running | <exit code> | 124 (stale kill)
#
# Liveness = the output grew OR the CPU time of the WHOLE process tree
# advanced (the tool parent often waits while the spawned children work —
# the per-group CPU sum of ps counts them all). No progress for
# <stale-minutes> consecutive polls → TERM → KILL of the group, the
# status records 124. The exit code of the command propagates otherwise.
#
# Long runs beyond one tool call:
#   nohup sh scripts/run-with-watchdog.sh <label> 5 <command...> &
#   cat /tmp/watchdog-<label>.status   # running | code
#   tail /tmp/watchdog-<label>.log     # live progress
set -u

LABEL="${1:?usage: run-with-watchdog.sh <label> <stale-minutes> <command...>}"
STALE_MIN="${2:?usage: run-with-watchdog.sh <label> <stale-minutes> <command...>}"
case "$STALE_MIN" in ''|*[!0-9]*) echo "stale-minutes must be a number" >&2; exit 2;; esac
shift 2

INTERVAL="${WATCHDOG_INTERVAL:-60}"
MAX_MIN="${WATCHDOG_MAX_MIN:-0}"
DIR="${WATCHDOG_DIR:-/tmp}"
LOG="$DIR/watchdog-$LABEL.log"
WATCH="$DIR/watchdog-$LABEL.watch"
STATUS="$DIR/watchdog-$LABEL.status"

# cpu_of_group sums the CPU seconds of every process of one process group
# (the command plus all spawned workers — a waiting parent with working
# children stays alive by the children's CPU).
cpu_of_group() {
        ps -eo pgid=,time= 2>/dev/null | awk -v g="$1" '
                $1 == g {
                        n = split($2, t, ":")
                        s = 0
                        for (i = 1; i <= n; i++) s = s * 60 + t[i]
                        total += s
                }
                END { print total + 0 }
        '
}

mkdir -p "$DIR"
: > "$LOG"
: > "$WATCH"
echo running > "$STATUS"

START=$(date +%s)
if command -v setsid >/dev/null 2>&1; then
        setsid "$@" > "$LOG" 2>&1 &
else
        "$@" > "$LOG" 2>&1 &
fi
PID=$!
PGID=$(ps -o pgid= -p "$PID" 2>/dev/null | tr -d ' ')
[ -n "$PGID" ] || PGID=$PID
echo "$(date -u +%FT%TZ) start pid=$PID pgid=$PGID stale=${STALE_MIN}m interval=${INTERVAL}s max=${MAX_MIN}m cmd=$*" >> "$WATCH"

stop_group() {
        kill -TERM "-$PGID" 2>/dev/null || kill -TERM "$PID" 2>/dev/null
        sleep 3
        kill -KILL "-$PGID" 2>/dev/null || kill -KILL "$PID" 2>/dev/null
}

LAST_SIZE=0
LAST_CPU=$(cpu_of_group "$PGID")
STRIKES=0

while kill -0 "$PID" 2>/dev/null; do
        sleep "$INTERVAL"
        kill -0 "$PID" 2>/dev/null || break
        SIZE=$(wc -c < "$LOG" | tr -d ' ')
        CPU=$(cpu_of_group "$PGID")
        ELAPSED=$(( $(date +%s) - START ))
        if [ "$MAX_MIN" -gt 0 ] && [ "$ELAPSED" -ge $(( MAX_MIN * 60 )) ]; then
                echo "$(date -u +%FT%TZ) hard cap ${MAX_MIN}m reached — killing" >> "$WATCH"
                stop_group
                echo 124 > "$STATUS"
                exit 124
        fi
        if [ "$SIZE" != "$LAST_SIZE" ] || [ "$CPU" != "$LAST_CPU" ]; then
                STRIKES=0
        else
                STRIKES=$(( STRIKES + 1 ))
                echo "$(date -u +%FT%TZ) no progress ${STRIKES}x (log=${SIZE}B cpu=${CPU}s elapsed=${ELAPSED}s)" >> "$WATCH"
                if [ "$STRIKES" -ge "$STALE_MIN" ]; then
                        echo "$(date -u +%FT%TZ) STALE for ${STALE_MIN} polls — killing" >> "$WATCH"
                        stop_group
                        echo 124 > "$STATUS"
                        exit 124
                fi
        fi
        LAST_SIZE="$SIZE"
        LAST_CPU="$CPU"
done

wait "$PID"
CODE=$?
echo "$CODE" > "$STATUS"
echo "$(date -u +%FT%TZ) done code=$CODE elapsed=$(( $(date +%s) - START ))s" >> "$WATCH"
exit "$CODE"
