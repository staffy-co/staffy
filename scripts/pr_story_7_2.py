#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 7-2 — python-JSON body assembly (precedent pr_story_7_1.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ.get("STAFFY_GH_TOKEN") or open("/tmp/.staffy_gh_token").read().strip()
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/artemiosu/staffy"

body = f"""## Story 7.2 — Реестр tools с типизированными контрактами (первый срез: субстрат без исполнения — артефакты контрактов + fail-closed загрузчик + привязка пак→tools + честный presence tool_schemas)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **Субстрат AR-17/AD-7 зерном дисциплины 7-1**: версионируемые артефакты контрактов `internal/tools/registry/<id>@<version>.json` + `index.json` sha256 append-only + go:embed; невалидный артефакт = реестр tools ЦЕЛИКОМ не грузится (fail-closed); смена семантики только новой версией (sha-пин).
- **Write-инструмент структурно невозможен**: платформенный словарь `determinism_class` = {{deterministic_read}} ЕДИНСТВЕННОЕ значение — значение вне словаря = отказ реестра; расширение словаря = правка ядра (машинальная гарантия AD-6/AD-7); `adapter_status` ∈ {{implemented, not_implemented}} — дисциплина AdapterImplemented; типы схем — словарь паков; semver-core строгий; reject-матрица 24 поимённо (fstest.MapFS на живых артефактах) + негативный контроль + fail-closed целиком.
- **Привязка пак→tools декларативна**: опциональное поле `tools` артефакта пака (nil у всех без привязки — неизменность опубликованных версений); первая живая привязка **auto-parts@6.1.0** (minor major 6, совместимая по умолчанию, semantic diff только tools — байт-стабильность 6.0.0 верифицирована) → **3 вертикальных контракта** над машинами 4-2/4-3/4-4, все честно `not_implemented` (двойная честность: адаптеров нет и диспетчера нет); кросс-валидация в загрузчике паков: рефы разрешаются в реестре tools (составная публикация fail-closed), `dependency.connector_type ∈ connector_types` пака — поимённые отказы; witness-подсерия паков переживает 8-ю запись.
- **Честный presence tool_schemas в Release Manifest**: производное состояние из привязки пина пака (4 ветви: пак не закреплён / без привязки / not_implemented «вызов невозможен» / fixture implemented → Present) вместо статичной записки 1-12; sealed-история не пересчитывается (AD-10); RuntimeDictionary/RuntimeComponentState/порядок словаря — БЕЗ изменений.
- **Read-only поверхность оператора по прецеденту «Каталог действий» 3-1**: GET `/v1/tenants/{{id}}/tools` (list новейших честным semver-сравнением) + пин-чтение точной версии + страница `/app/tenants/{{id}}/tools` (словари текстом, details/summary, honest empty state AG-4); read-gate FIRST: не-member 404, member без скоупов 403 со словами ДО материала; малформед-пин = честный 404 ДО сервиса; OpenAPI +2 path + ToolContractView/ToolSchemaFieldView с **enum-зеркалами** словарей (contract-тест сверяет против платформенных источников).
- **Drift-CI**: tools-база append-only против origin/main (GITHUB_ENV-паттерн 7-1, только в ветке успеха); вертикальный зонд 23 фразы (+6 фраз артефактов tools) + зона **+internal/tools**; симуляции удаления/перезаписи поимённо.
- **Путь внешнего автора = PR «артефакт + индексная запись», 0 правок .go** — доказан живой симуляцией (+logistics-freight.quote-lookup@1.0.0: полный сьют 35 ok и journey зелёные, откачена); хрупкие пины «ровно 3»/«дубликат хвоста» переформулированы подсерией/динамически (урок 7-1, применён ДО слияния).
- **Честные границы**: исполнение tool-вызовов и runtime capability-probe НЕ строятся (Planner 8.3 / MCP 7.3, deferred-work +3); 0 SQL-миграций; замороженные машины не тронуты.

### Steps
- SM (7f5a512 + 7467d23 + 87b7158): спека первого среза (исправление опечатки sha поймано check-continuity WARN)
- CHECKPOINT 1 (53f5097 + 08610d2): предъявление доставлено возобновлением после смерти контекста (checkpoint1-7-2, 0 правок репо); одобрено «продолжайте»; флип ready-for-dev одним коммитом (урок 3-1); внешняя консультация «мозги/руки» сверена со спайном ДО флипа
- step-01 (37780e0 + 8f63d42): носители — ToolContract + 3 артефакта + индекс sha256 + поле PackManifest.Tools/ToolRef + auto-parts@6.1.0 + 8-я запись индекса + clone-дисциплина
- step-02 (9791aad + b31a6e9): loader.go fail-closed + reject-матрица 23 (позже 24) + catalog Get/List + кросс-валидация привязки (инъекцируемый резолвер)
- step-03 (1946c79 + 9af1376): DeriveToolSchemasState 4 ветви + BuildRuntimeState с входом + app seal явная сборка (инвариант append-only) + journeys fixture production-путь
- step-04 (1d1a081 + 390f8e8): service read-gate + transport list/пин/страница + шаблон + OpenAPI + зеркала + contract-тест + journey z55
- step-05 (02ae5b8 + 20a07d3 + 4fdcb16): drift-CI + симуляция внешнего PR + подсерии + спека done + sprint-status done (Epic 7: 2/4) — feat БЕЗ [skip ci]; 4fdcb16 — активация push-рана (урок 282)

### Regression
build/vet OK · unit 35 пакетов зелёные (-count=1) · интеграция ×2 TZ (UTC 65.3s + Europe/Moscow 66.8s, живой PG) · journeys **55/55** серийным прогоном 3.5m (+1 spec-файл z-серии — 55-й замыкает цепочку; порог ≥48 не затронут) · reject-матрица поимённо · симуляция внешнего PR зелёная полным сьютом · check-continuity OK · gofmt новых файлов чист.

Ветка: 20 коммитов от main 40b795f, 28 файлов, +4622/−1625.

Epic 7: 2/4.
"""

payload = json.dumps({
    "title": "feat(story-7-2): Реестр tools с типизированными контрактами — субстрат без исполнения + привязка пак→tools + честный presence",
    "head": "codex/story-7-2-typed-tools-registry",
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
