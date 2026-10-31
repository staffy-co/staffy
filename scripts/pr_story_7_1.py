#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 7-1 — python-JSON body assembly (precedent pr_story_5_1.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ.get("STAFFY_GH_TOKEN") or open("/tmp/.staffy_gh_token").read().strip()
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Story 7.1 — Декларативный каталог Domain Pack (первый срез: вынос 7 манифестов в версионируемые JSON-артефакты registry/ + fail-closed загрузчик + байт-стабильная эквивалентность)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **Смена носителя без смены контрактов**: 7 опубликованных манифестов {{auto-parts@1.0.0–6.0.0, montagio-services@1.0.0}} вынесены из Go-карты `packs/catalog.go` в JSON-артефакты `registry/<id>@<version>.json` + `index.json` с sha256 hash-пинами (append-only); сигнатуры `Get/List/Impact` неизменны; потребители {{packresolver, action, transport, employee app/store/release}} — **0 правок ни строкой** (дифф-зонд).
- **Fail-closed загрузчик ДО использования**: обязательные поля 17 поимённо (урок ValidateVariants 5-1); enum-словари ПЛАТФОРМЫ пак не расширяет {{support_status; stage; VariantField.Type; Vehicle.Kind; Part.Kind; review_slots ⊆ connectors|knowledge|behavior}}; AtLeastOneOf ⊆ Schema; UpgradeFrom-цели существуют в реестре; **major-переход без UpgradeFrom = отказ** (новый major; minor/patch существующего major совместимы по умолчанию); строгий semver-core; невалидный артефакт = **реестр целиком не загружается** (паника загрузки; частичный каталог запрещён); orphan-файлы/дубликаты/канон имён — поимённые отказы.
- **Эквивалентность байт-стабильно**: golden-фикстуры сняты ОДНОРАЗОВЫМ генератором с живого бинарного каталога ДО миграции (генератор удалён; golden = доказательство, не перегенерируется); labels Schema {{«Интерес к деталям», «Глубина сопоставления», «Инженерная сантехника», «Отопление»}} и ReplacementImpact переехали в артефакты; witness-тесты: golden-семёрка в индексе/List()/embed-реестре байт-стабильно ПОДСЕРИЕЙ, 9 impact-проб реплеем, immutability 20 мутаций (мутация копии невидима следующему читателю).
- **Drift-CI**: вертикальный зонд — словарь 17 фраз артефактов (Names/Purposes/Journeys/labels) ФИКСИРОВАН в тесте, зона прод-.go общих пакетов ядра {{domain,app,port,store}}+cmd, 0 нарушений, 2 поимённых честных исключения {{pilot.go 4-1 заморожен, cases/domain/part.go 4.3 вертикаль}}; append-only база origin/main (git fetch → git show index.json → STAFFY_REGISTRY_BASELINE env) — удаление записи/перезапись sha = поимённые отказы, дополнения разрешены.
- **Путь внешнего автора = PR «артефакт + индексная запись» с 0 правок .go** — доказан живой симуляцией (+logistics-freight@1.0.0: полный сьют и полный регресс репо зелёные с 8-м манифестом, симуляция откачена); witness-тесты переформулированы на подсерию именно для этого (дефект дизайна пойман симуляцией и закрыт в шаге-04).
- **Честные границы**: механика review-гейтов внешних авторов = существующий PR-процесс (машины гейтов нет — deferred-work +3); HTTP-поверхность публикации/внешние хранилища вне среза (go:embed, реестр версионируется git'ом с ядром); реестр tools (7.2), MCP-сервер (7.3), коннекторы (7.4) — следующие stories эпика.
- **0 SQL / 0 миграций / 0 OpenAPI / 0 страниц / 0 journeys-spec** — поведение неизменно (эквивалентность); замороженные машины 4-1 байт-стабильны.

### Steps
- SM (eb2f62b + 898247f): спека первого среза; маркетплейс-аннотация Story 7.4 в main (382d46b + 6d1fd85) до ветки
- CHECKPOINT 1 (bdae0c5 + c2cf452): предъявление доставлено возобновлением после смерти контекста (checkpoint1-7-1); одобрено «продолжайте»; флип ready-for-dev + epic-7 in-progress одним коммитом (урок 3-1)
- step-01 (c61ed09 + 60e8cc5): golden-снимок живого каталога ДО правок + 7 артефактов + индекс sha256 + witness-тесты (генератор throwaway, удалён до коммита)
- step-02 (70183eb + 752ef26): loader.go fail-closed + reject-матрица 21 поимённо (fstest.MapFS на реальных артефактах) + негативный контроль minor + embed-реестр байт-стабилен golden
- step-03 (0181200 + 85117cc): переключение каталога (литералы/replacementImpact/label-инъекция удалены; clone() чистое копирование c nil-сохранением) + immutability 20 мутаций + witness-зелёное над реестром
- step-04 (29c4da2 + 5e5145e): drift-CI (зонд 17 фраз + append-only + симуляции отказов) + ci.yml +шаг базы реестра + симуляция внешнего PR (0 правок .go) + переформулировка witness-тестов на подсерию + deferred-work +3
- step-05 (fdf5dcf + 4440f74): QA-closure — саморевью по диффу, финальный регресс всех контуров, спека done + sprint-status done (Epic 7: 1/4) — feat БЕЗ [skip ci]

### Regression
build/vet OK · unit 34 пакета зелёные (-count=1) · интеграция ×2 TZ (UTC 66.3s + Europe/Moscow 63.5s, живой PG, полным прогоном) · journeys 54/54 серийным прогоном 3.6m (0 новых spec-файлов — поведение неизменно, порог ≥48 не затронут) · reject-матрица 21/21 поимённо · симуляция внешнего PR зелёная полным регрессом · check-continuity OK (fail=0, warn=0) · gofmt packs/ чист целиком · git diff --check чист.

Ветка: 16 коммитов от main 6d1fd85, 19 файлов, +3962/−99.

Epic 7: 1/4.
"""

payload = json.dumps({
    "title": "feat(story-7-1): Декларативный каталог Domain Pack — артефактный реестр + fail-closed загрузчик + байт-стабильная эквивалентность",
    "head": "codex/story-7-1-declarative-domain-pack-catalog",
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
