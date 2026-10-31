#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR infra module-path — python-JSON body assembly (precedent pr_story_7_4.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ.get("STAFFY_GH_TOKEN") or open("/tmp/.staffy_gh_token").read().strip()
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Infra — переезд go.mod module path → github.com/staffy-co/staffy

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Что и почему
- Владелец перенёс репо artemiosu/staffy → организацию staffy-co/staffy и сделал его публичным (DEVLOG 321-infra-repo-transfer); module path главного модуля переезжает вслед за каноническим URL репозитория.
- Объём: go.mod (module-строка) + 528 .go-файлов (все внутренние импорты) + 6 qa_step05-сканеров (матчат module path исходников) = 535 файлов, sed-замена без reformat, +2279/−2279, churn-нулевая (plain == ignore-all-space).

### Коммиты
- a82fc46 refactor(module): чистый sed-переезд module path (без reformat, import-порядок не тронут).
- 62e629c style(module): gofmt-выплата долга 120 файлов, тронутых переездом — re-sort импортов после artemiosu→staffy-co (254 контентных строки) + канонизация форматирования legacy-файлов по правилу урока DEVLOG 319 (расхождение plain-vs-ignore-all-space → gofmt -w); 14 файлов вне скоупа переезда долг сохраняют.

### Regression (локально, живым PG)
build OK · vet OK · unit-суит зелёный · интеграция ×2 TZ (UTC 68.2s / Europe/Moscow 68.5s) · journeys **57/57** серийным прогоном 3.6m · gofmt: 0 файлов в скоупе.

### Честные границы
- go.sum не затронут: module path главного модуля не влияет на require-блоки зависимостей.
- DEVLOG.md и _bmad-output/** упоминают artemiosu исторически — append-only история, не переписывается.
- graphify-out/graph.json и снапшоты дат — производные артефакты со старыми путями нод; graphify CLI в песочнице отсутствует, регенерация `graphify update .` отложена до наличия тулзы (AGENTS.md graphify-секция).
- 14 legacy-файлов вне скоупа переезда остаются не-gofmt-чистыми — известный долг, отдельным шагом по сигналу.
- Не story-BMAD: инфраструктурный рефакторинг по прямому сигналу владельца; спеки/sprint-status не затронуты (check-continuity вне зоны).
"""

payload = json.dumps({
    "title": "refactor(infra): переезд go.mod module path → github.com/staffy-co/staffy (535 файлов, churn-нулевая)",
    "head": "codex/infra-module-path-staffy-co",
    "base": "main",
    "body": body,
}).encode()

request = urllib.request.Request(
    f"{API}/pulls",
    data=payload,
    method="POST",
    headers={
        "Authorization": f"Bearer {TOKEN}",
        "Accept": "application/vnd.github+json",
        "Content-Type": "application/json",
    },
)
with urllib.request.urlopen(request) as response:
    result = json.load(response)
print(json.dumps({
    "number": result["number"],
    "state": result["state"],
    "head": result["head"]["sha"],
    "mergeable": result.get("mergeable"),
    "url": result["html_url"],
}, ensure_ascii=False, indent=2))
