#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 7-3 — python-JSON body assembly (precedent pr_story_7_2.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ.get("STAFFY_GH_TOKEN") or open("/tmp/.staffy_gh_token").read().strip()
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Story 7.3 — MCP-сервер поверхности реестра tools (первый срез: протокольная машина поверхности без живых адаптеров)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **Протокольная машина MCP по ТЕКУЩЕЙ ревизии 2026-07-28** (stateless-ядро: handshake initialize/initialized и Mcp-Session-Id УДАЛЕНЫ протоколом — SEP-2575/SEP-2567; batch = invalid request «одно JSON-RPC сообщение на POST»; GET/DELETE = 405 — стримов и сессий нет; ответы application/json, сервер никогда не инициирует): единый машинный endpoint `POST /v1/tenants/{{tenant_id}}/mcp` за лестницей членства 3-1/7-2 (401/404/403 со словами скоупов ДО единого байта JSON-RPC); обязательный `server/discover` + `tools/list` + `tools/call`; заголовки `Mcp-Method`/`Mcp-Name`/`MCP-Protocol-Version` сверяются с телом (SEP-2243, рассинхрон = 400); заявка ревизии в `_meta` каждого сообщения + заголовком, чужая = 200 JSON-RPC `UnsupportedProtocolVersionError` со списком ровно [2026-07-28] (handshake-эпоха честно НЕ поддерживается). **SDK НЕ вводится** — stdlib, пиннинг поимёнными тестами; wire-пиннинг честно задокументирован: базовый конверт и CallToolResult подтверждены живым чтением официальной схемы 2025-11-25, ревизия-специфичные имена — пиннинг платформенных тестов (официальная схема 2026-07-28 «Asset not found»).
- **Гейт экспозиции AC-1 fail-closed** = пересечение фактов: артефакт `implemented` ∧ `determinism_class = deterministic_read` **проверяется НА слое** (write-класс структурно невидим независимо от источника — машинная гарантия AD-6/AD-7, не ревью) ∧ адаптер зарегистрирован composition root ∧ зависимость жива по словарю состояний 1-8 (verified ∃ ∧ failed/stale ∅ ∧ live-подключение типа). Продакшн-адаптеров **НОЛЬ** (AG-4, прецедент nil-адаптеров 3-6/3-7): honest empty `[]`, все 3 контракта реестра остаются `not_implemented` — двойная честность 7-2 сохранена. **Ротация → stale → инструмент исчезает из словаря** — закрытие динамической половины AC-3 спеки 7-2 (доказано fixture-тумблером в journey). Гейт повторяется при call (TOCTOU честно).
- **Путь вызова AC-3 ЧЕРЕЗ контракт реестра 7-2**: пин-чтение новейшей версии каталогом (semver-core); strict-валидация аргументов по input_schema — закрытая форма, типы string/integer/boolean, enum, границы, обязательность — **поимённые слова полей** как tool execution error `isError=true` (SEP-1303, модель самокорректируется), НЕ протокольная ошибка; адаптер с ctx-бюджетом (превышение = «бюджет выполнения исчерпан»); **выход валидируется fail-closed**: undeclared ключ/тип = отказ, внутренний материал НЕ пересылается (AC-2); результат = `structuredContent` + пин `<id>@<version>` в content; tenant ИЗ СЕССИИ, никогда из аргументов (AD-11).
- **Проекция словаря типизации → JSON Schema** MCP-провода тотальна 1:1 (title/type/enum/minimum/maximum/required; нулевая граница — незаданное слово); реестр остаётся платформенным языком (json-schema.org в артефакты не входит — Never 7-2); `annotations.readOnlyHint=true` — факт словаря.
- **Capabilities = факт**: tools only, `listChanged=false`; resources/prompts/sampling/elicitation/ttlMs/cacheScope НЕ заявляются (незаявленные отсутствуют на проводе — тест).
- **Rate/budget декларированы в коде**: тело ≤ 256 KiB → 413; Content-Type → 415; фиксированное окно принципала → 429 + Retry-After (honest: in-memory окно — односайтовая граница, AG-4); аудит каждого tools/call slog-строкой (code/tenant/principal/tool@version/verdict+причина/correlation_id; PG-гроссбух — deferred).
- **Проводка main.go `BindMCP`** с ПУСТЫМ реестром адаптеров (первая правка cmd с 4-11 — MCP-сервер сам предмет истории); toolssource над живым реестром 7-2 + toolsdependency над ConnectionsFacts (словарь verified/failed/stale/unverified/no_live_connection для аудита).
- **Drift-зона +internal/mcp** осознанной правкой — словарь фраз НЕ растёт (23 записи, зонд 0); `internal/mcp` вертикально-чист; вертикальные слова демо-контракта — только тест-контур journeys. **0 SQL; 0 правок internal/tools** (субстрат служит как есть).
- **OpenAPI +1 path + 8 схем** (закрытые конверты, словари заголовков, лестница 200/202/400/401/403/404/413/415/429+Retry-After, GET/DELETE = операции 405) + `mcp_contract_test.go` зеркала: словарь методов ↔ mcp-константы, пинн версии ↔ `SupportedProtocolVersions()`, **типы проекции ↔ `tools.ToolFieldTypes()`**, `serverInfo.version` ↔ `info.version` (осознанный шаг в двух местах), listChanged/readOnlyHint const-факты.
- **Journey z{{51}}** (серийное замыкание цепочки, 51-й z-префикс): scripted MCP-клиент fetch без SDK — лестница, discover, версии, заголовки, batch/малформед, 202, 405, list ровно демо (реальные not_implemented исключены), call happy/поимённые отказы/ghost, Mcp-Name когерентность, ротация→исчезновение, leaky→секрет НЕ на проводе, 429+Retry-After, 415/413.

### Steps
- SM (d57d2db + 1704377): спека первого среза (пиннинг 2026-07-28 по внешнему исследованию; CI на голове — success)
- CHECKPOINT 1 (aaa1bb3 + 19591eb): предъявление после живой верификации сброса контекста; одобрено «продолжайте»; флип ready-for-dev одним коммитом (урок 3-1)
- step-01 (ad39102 + 304d0c8): носители протокола + проводка — internal/mcp (protocol/ports/service), adapters/toolssource + toolsdependency, transport BindMCP каркас, main.go ПУСТОЙ реестр AG-4; **correction**: код ушёл одним коммитом под docs-сообщением (git add rc=1 по неякореному игнор-паттерну каталога оборвал цепочку &&; паттерн заякорен `/staffy-core`)
- step-02 (d17382e + 8435073 + 46e5441): машины методов + projection + структурный тест write-невозможности (8435073 — заполнение sha: переменные bash не живут между вызовами)
- step-03 (efe9cd8 + a199926): транспорт полная матрица full-stack + OpenAPI + контракт-тест зеркал
- step-04 (a1c5df6 + 4b406d1): fixture-мир + тумблеры + journey z{{51}} живьём до коммита
- step-05 (3dfb647 + 89bd2af): drift-зона +mcp + QA-closure (спека done, sprint-status done, deferred-work +7) — feat БЕЗ [skip ci], голова 89bd2af активирует CI-ран ветки (урок 282)

### Regression
build/vet OK · unit **38 пакетов** зелёные (-count=1) · интеграция ×2 TZ живым PG (**UTC 62.9s + Europe/Moscow 66.3s**) · journeys **56/56** серийным прогоном 3.6m (+1 spec-файл z-серии; порог ≥48 не затронут) · зонд вертикального словаря: **0 фраз** в зоне +internal/mcp · check-continuity: см. CHECKPOINT 2 · gofmt новых файлов чист.

Ветка: 15 коммитов от main a798161, 29 файлов, +4360/−3.

Epic 7: 3/4.
"""

payload = json.dumps({
    "title": "feat(story-7-3): MCP-сервер поверхности реестра tools — протокольная машина 2026-07-28 + гейт экспозиции + путь вызова (без живых адаптеров)",
    "head": "codex/story-7-3-mcp-registry-surface",
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
