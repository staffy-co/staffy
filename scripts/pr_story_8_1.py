#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 8-1 — python-JSON body assembly (precedent pr_story_7_4.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ.get("STAFFY_GH_TOKEN") or open("/tmp/.staffy_gh_token").read().strip()
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Story 8.1 — Контракт хода типизированного ограниченного планировщика (первый срез: протокольная машина «мозга» ДО живых адаптеров — зерно 7-3)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **Turn Context — закрытая типизированная форма входа** (strict decode ДО модели, AD-11: undeclared-ключ/неверный тип = поимённый отказ, ход не начинается): референсы {{tenant, case, conversation, Work Item, employee}}; пины Release Manifest/pack/policy bundle/Allowed Claims/autonomy РЕФЕРЕНСАМИ id@версия (не материалом); подтверждённые факты с обязательным provenance {{источник, версия, время}}; режим + активная лестница ТОЧНЫМ зеркалом ModeOrder FR-14 (контракт-тест); инвентарь read-инструментов имя@версия + схемы входа зеркалом tools.ToolFieldTypes 7-2 БЕЗ расширения словаря (контракт-тест); требование раскрытия ИИ §8.1; fallback-политика хода; платформенные значения {{string, integer, boolean}} — дробь/экспонента = отказ типизации.
- **Turn Proposal — версионируемая закрытая форма выхода** (schema_version=1; чужая версия = поимённый отказ когерентности — зеркало wire-дисциплины 7-3/7-4): intent/mode/safety закрытыми словарями среза; fact-предложения/противоречия/патч Case-памяти с provenance; read-запросы валидируются против инвентаря КОНТЕКСТА (несуществующий инструмент / аргумент вне схемы / вне enum / вне границ = отказ — словарная дисциплина 7-2/7-4, TOCTOU-дух); предлагаемое действие закрытым union {{none, commitment, action, handoff}} + контракт-реф для kind ≠ none БЕЗ материала исполнения (AD-6 — предложение только называет); реплика и вопросы в бюджете 2000 рун — сверх = отказ БЕЗ усечения.
- **Машина internal/planner**: раунд 1 → strict decode → валидация → поимённый отказ → **ровно ОДИН repair-проход** (модель получает поимённую причину) → вторая неудача персистится честной записью + ход завершает **ТОЛЬКО утверждённый безопасный fallback** {{honest_stop | handoff | callback по политике контекста}} — НИКОГДА вывод модели (AD-7 дословно; тестами: проваленный ход не несёт Proposal); ошибка адаптера = failed без слепого retry; модельных раундов структурно ≤2; скоуп-гард тенента до раунда (AD-11); Build «planner/1.0.0» в каждой записи.
- **ModelPort без credentials** (одна операция Round; AG-4 — граница эпика) + **scripted-модель internal/adapters/modelscripted** — честная синтетика продакшн-кода по прецеденту verifier.Synthetic: матрица 7 исходов {{valid, malformed_undeclared, malformed_type, foreign_version, repairable, unrecoverable, adapter_error}}, синтез из контекста (mode зеркалит контекст); **main.go НЕ подключает — продакшн-вызывателей НОЛЬ** (вызыватель — 8.4).
- **Turn-журнал migrations/planner/00001 за tenant RLS/FORCE** (AD-10/AD-11): append-only **грантами** (staffy_app только SELECT+INSERT — UPDATE/DELETE запрещены правами, интеграционный тест ловит permission denied живьём); уникальность (tenant, turn_id) = честный ErrTurnConflict (23505 — дисциплина action-стора); NULL = честное отсутствие; JSONB-предложение за RLS — бизнес-материал духа reply-строк 2.12, НЕ телеметрия (AR-18-обвязка — 8.6).
- **Вертикальная чистота**: drift-зоны +internal/planner +internal/adapters/modelscripted осознанной правкой — зонд 0 фраз словаря (23 записи) живым прогоном, словарь фраз НЕ растёт; платформенные словари без продакшн-импортов вертикалей (зеркала контракт-тестами).
- **0 правок потребителей**: cases/action/tools/mcp/mcpclient/externaltools/transport/main.go — не тронуты (живой git-зонд); RuntimeDictionary БЕЗ presence-флипов (release.go не тронут — поверхности 8.2/8.4/8.5; AD-10); 1 SQL-миграция (объявлена в спеке с самого начала); HTTP-поверхности хода НЕТ (прецедент чистых Go-срезов 7-1); journeys не затронуты.

### Steps
- SM (1e651e5 + 640a9a4): спека первого среза (37КБ, I/O-матрица 21 кейс) + DEVLOG 324
- CHECKPOINT 1 (d9d47e1): закрыт явным «Продолжайте» владельца — флип ready-for-dev + sprint-status синхронно (урок 3-1)
- step-01 (917a5f8): формы + словари + валидация + контракт-матрица + зеркала ModeOrder/ToolFieldTypes
- step-02 (1e5b7a7): машина + ModelPort + fallback + scripted-адаптер + машина-матрица (дефект repaired-пути пойман юнит-матрицей ДО коммита)
- step-03 (71736d2): turn-журнал + PG-стор + TestMain-вайринг + интеграционный контур живым PG
- step-04 (d106de1): drift-зоны + полный регресс ×2 TZ
- step-05 (f3215b6 + TBD-fill): QA-closure — саморевью живыми зондами, спека done, sprint-status 8-1 done (epic-8 in-progress 1/6), deferred-work +6 honest-границ, DEVLOG 325

### Regression
build/vet OK · unit **44 пакета** зелёные (-count=1) · интеграция ×2 TZ живым PG (**UTC 66.4s + Europe/Moscow 55.4s**) · вертикальный зонд: **0 фраз** в зонах +planner +modelscripted · gofmt новых файлов чист · journeys 57 файлов не затронуты (поверхности нет).

Ветка: 10 коммитов от main b589eb2, 23 файла, +3409/−6 (код 13 файлов, артефакты 5, DEVLOG, TestMain/drift surgical).

Epic 8: **1/6** — контракт хода существует; следующий срез — 8.2 «Модельный адаптер» по сигналу владельца.
"""

payload = json.dumps({
    "title": "feat(story-8-1): контракт хода — Turn Context + Turn Proposal закрытыми формами, машина repair×1/fallback AD-7, scripted-модель, turn-журнал за RLS (Epic 8: 1/6)",
    "head": "codex/story-8-1-turn-contract",
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
}))
