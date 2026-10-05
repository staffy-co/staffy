#!/usr/bin/env bash
# ============================================================================
# check-continuity — проверка консистентности системы непрерывности staffy.
# Выход 0 = консистентно. Запускать из корня репо:
#   - в конце scripts/bootstrap.sh (косвенно — через smoke);
#   - перед финальным push сессии;
#   - в drill (docs/CONTINUITY.md).
# Проверяет: чистое дерево; нет незапушенных коммитов; статусы спек ==
# sprint-status.yaml; коммит из последней записи DEVLOG существует в истории.
# ============================================================================
set -uo pipefail
fail=0; warn=0

# 1. Рабочее дерево чистое (незакоммиченное = риск потери)
if [ -n "$(git status --porcelain)" ]; then
  echo "FAIL: грязное рабочее дерево (незакоммиченная работа = риск потери)"
  git status --short | head -10
  fail=1
fi

# 2. Нет незапушенных коммитов
branch=$(git branch --show-current)
if git rev-parse "origin/$branch" >/dev/null 2>&1; then
  ahead=$(git rev-list --count "origin/$branch..HEAD")
  if [ "${ahead:-0}" -gt 0 ]; then
    echo "FAIL: ветка $branch опережает origin на $ahead коммит(ов) — запушить немедленно"
    fail=1
  fi
else
  echo "WARN: origin/$branch не найден (нет fetch?)"
  warn=1
fi

# 3. Статусы спек синхронны со sprint-status.yaml
python3 <<'PY' || fail=1
import re, glob, sys

sm = {}
sec = False
for line in open('_bmad-output/implementation-artifacts/sprint-status.yaml', encoding='utf-8'):
    if line.startswith('development_status:'):
        sec = True
        continue
    if not sec:
        continue
    m = re.match(r'^  (\S+): (\S+)\s*$', line)
    if m:
        sm[m.group(1)] = m.group(2)

problems = []
for f in sorted(glob.glob('_bmad-output/implementation-artifacts/spec-*.md')):
    m = re.match(r'.*spec-(\d+)-(\d+)-.*\.md', f)
    if not m:
        continue
    story = f"{m.group(1)}-{m.group(2)}"
    head = open(f, encoding='utf-8').read(2500)
    s = re.search(r"^status:\s*'?([\w-]+)", head, re.M)
    spec_status = s.group(1) if s else None
    keys = [k for k in sm if k.startswith(story + '-')]
    if not keys:
        problems.append(f"спека {f}: story {story} отсутствует в sprint-status.yaml")
        continue
    sprint_status = sm[keys[0]]
    if spec_status != sprint_status:
        problems.append(f"story {story}: спека='{spec_status}' а sprint-status='{sprint_status}' ({f})")
if problems:
    print("FAIL: рассинхрон статусов спека/sprint-status: " + "; ".join(problems))
    sys.exit(1)
print("OK: статусы всех спек синхронны со sprint-status.yaml")
PY

# 4. Коммит из последней записи DEVLOG существует в локальной истории
last_sha=$(awk '/^commit: /{v=$2} END{print v}' DEVLOG.md 2>/dev/null | tr -d '\r')
if [ -n "$last_sha" ]; then
  if git cat-file -e "${last_sha}^{commit}" 2>/dev/null; then
    echo "OK: коммит $last_sha из последней записи DEVLOG присутствует в истории"
  else
    echo "WARN: коммит $last_sha из последней записи DEVLOG не найден локально (fetch?)"
    warn=1
  fi
fi

echo "check-continuity: $([ $fail -eq 0 ] && echo OK || echo FAIL) (fail=$fail, warn=$warn)"
exit $fail
