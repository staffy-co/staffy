#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 7-4 — python-JSON body assembly (precedent pr_story_7_3.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ.get("STAFFY_GH_TOKEN") or open("/tmp/.staffy_gh_token").read().strip()
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/artemiosu/staffy"

body = f"""## Story 7.4 — Внешние MCP-серверы как класс Connector (первый срез: клиентская машина + живые MAC-пробы + тенент-скоупед внешний словарь)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **Класс «внешний MCP-сервер» = карточка каталога** `staffy-external-mcp-server@1.0.0` (Type business system, RequiredAccountType `external-mcp` — маркер маршрута роутера; AdapterImplemented true — клиентская машина и проба реальный продакшн-код; окружения ровно {{sandbox, test}} — **production на карте отсутствует, AG-4 fail-closed на классе**); capabilities поимённо с полной операционной семантикой прецедента телефонии: query.server-discover + query.tools-list (живые пробы), query.tool-call (ProbeAllowed false — declaration ≠ claim, потребитель Planner 8.3), command.external-tool (ActionGatewayBlock машинально — мутации внешних систем за AG), event.server-message (честно нереализован — stateless-клиент подписок не открывает); ровно ОДИН секрет api_key (граница MAC-среза).
- **Клиентская протокольная машина `internal/mcpclient`** ревизии 2026-07-28 (stateless, **stdlib без SDK** — словари-зеркала серверной машины 7-3): одиночные JSON-RPC POST server/discover | tools/list | tools/call, заголовки Mcp-Method/Mcp-Name/MCP-Protocol-Version сверяются с телом; строгий decode конверта (result XOR error, эхо id, когерентность заявленной версии — чужая ревизия = поимённый отказ); размерные бюджеты чтения без усечения (сверх = отказ целого ответа); фильтрация дескрипторов до известных полей протокола (AD-11, без passthrough); **credential-агностика** — Authorization явный параметр вызывающего, никогда не хранится/не логируется/не эхо.
- **Composition-роутер верификатора `internal/adapters/connectorprobe`**: маршрут по RequiredAccountType (external-mcp → MCP-проба над mcpclient; иначе → verifier.Synthetic побайтно — тест-пин телефонии); BasicCheck маршрутизируется тоже (формат ключа sandbox-… + форма URL без сети); Build = `connectorprobe/1.0.0` — смена билда честно инвалиддирует прежние evidence (семантика 1-8, операторы перепробуют).
- **Пробы с секретом ИЗ команды верификации** (инвариант 1-8 дословно — plaintext никогда не персистится): VerifyCommand.Secrets едет только в память; app MAC-проверяет (verifySeal: HMAC seal_key над name\\0value\\0) против принятого seal через узкий ReadSeal **ДО любого сетевого раунда** — отсутствие/несовпадение ключа = честные отказы fail-closed (неаутентифицированный раунд невозможен); ротация → новый SecretID → прежние evidence stale.
- **Тенент-скоупед внешний словарь `internal/externaltools`**: чистая деривация из ConnectionsFacts по якорям Applicable 1-8 (connection/SecretID/окружение/пин контракта/билд/UpdatedAt/TTL-24ч — нулевая вторая запись состояний); **ротация/save/TTL/смена билда → запись ИСЧЕЗАЕТ** (динамическая честность внешнего направления, зеркало 7-3); тотальная проекция JSON Schema → платформенный словарь типизации (string/integer/boolean + enum/целые границы/required; вне подмножества = поимённый честный отказ НА ЗАПИСЬ; **readOnlyHint обязателен — AR-30**: без заявления read-only инструмент не допускается; write-инструмент не существует ни как запись, ни как инструмент модели — AD-6/AD-7); provenance записи: сервер имя/версия + карточка@версия + evidence id.
- **Машина вызова `externaltools.Calls`** credential/target-агностичная под Planner 8.3: пин записи из ТЕКУЩЕЙ деривации (TOCTOU fail-closed) → strict-валидация входа по спроецированному input → раунд с Authorization-параметром вызывающего → strict-валидация выхода fail-closed (undeclared ключ = отказ, внутренний материал не пересылается); **HTTP-поверхности вызова НЕТ** — продакшн-вызывателей ноль (AG-4), форма закреплена под 8.3.
- **Транспорт словаря за лестницей 3-1/7-2**: GET `/v1/tenants/{{id}}/tools/external` (API) + `/app/tenants/{{id}}/tools/external` (страница строго read-only — ни одной формы); 401/404/403 словами скоупов ДО материала; OpenAPI +1 path + 3 закрытые схемы + `external_tools_contract_test.go` зеркала (типизация ↔ tools.ToolFieldTypes, состояния/окружения закрытыми enum).
- **Инвентарь — фильтрованный компактный JSON в redacted_result существующей строки evidence** (маркер «Инвентарь: » — контракт пробы и деривации, пин тестами обоих пакетов); отклонение от «0 SQL» задокументировано: миграция 00003 — релаксация CHECK redacted_result 300→262144 (= машинному бюджету чтения), форма схемы неизменна, без новых таблиц/колонок.
- **Drift-зоны +mcpclient +externaltools** осознанной правкой — словарь фраз НЕ растёт (23 записи, зонд 0 живым прогоном); вертикальные слова fixture-мира — только тест-контур journeys. **0 правок internal/tools и internal/mcp.**
- **Journey z{{52}}** живым scripted внешним сервером на общем листенере (/fixture/external-mcp, тумблеры foreign-revision/oversized/leaky — прецедент reconcile-verdict): карточка → connect формат-проверкой БЕЗ сети → лестница проб fail-closed (отсутствие/чужой ключ — честные 400 словами) → ЖИВЫЕ пробы формами с инвентарём в evidence → honest-отказы поимённо (чужая ревизия / размерный бюджет) + восстановление → словарь (1 допущен с провенансом, 3 отклонённых с поимёнными причинами, 0 форм) → ротация → записи ИСЧЕЗАЮТ словами якоря → старый ключ не матчится → повторная проба возвращает → лестница 403/404 → a11y 320px.

### Steps
- SM (e49c841 + 4206e3b): спека первого среза (108 строк, I/O-матрица 25 кейсов; CI на голове — success)
- CHECKPOINT 1 (61a5d57 + 0610ef5): одобрено «продолжайте»; флип ready-for-dev одним коммитом (урок 3-1)
- step-01 (4575b50 + 6180204): клиентская машина mcpclient + карточка класса + роутер connectorprobe + секреты проб MAC (ReadSeal scoped-RLS, 0 миграций) + main.go — верификатор роутера
- step-02 (03453db + f61fb9e): инвентарь probes + словарь externaltools {projection/derive/service/calls} + адаптер externaltoolsfacts + миграция 00003 (задокументированное отклонение)
- step-03 (7519dde + bc1cd01): транспорт словаря + страница read-only + OpenAPI + контракт-тест зеркал
- step-04 (67280d7 + f25fdd5 + 8622ab7): fixture-мир + секреты VerifyCommand (API+форма) + journey z52 живьём до коммита; три урок-фикса фикстур (идемпотентность-зеркало LookupEvidence + transportIDs v7-mint; UUID-мина rune-конкатенации; supported-локатор двух карточек)
- step-05 (f142090 + f3008e2): drift-зоны + QA-closure (спека done, sprint-status 7-4 done, **epic-7 done 4/4**, deferred-work +7) + стайл-коррекция whitespace-чурна (gofmt -w, кумулятивный дифф = только реальные изменения) — feat БЕЗ [skip ci], голова активирует CI-ран ветки (урок 282)

### Regression
build/vet OK · unit **39 пакетов** зелёные (-count=1) · интеграция ×2 TZ живым PG (**UTC 68.7s + Europe/Moscow 59.5s**) · journeys **57/57** серийным прогоном 4.5m (+1 spec-файл z-серии; порог ≥48 не затронут) · зонд вертикального словаря: **0 фраз** в зонах +mcpclient +externaltools · gofmt восстановленных файлов чист.

Ветка: 15 коммитов от main 12dc41d, 49 файлов, +6654/−1165.

Epic 7: **4/4 — закрывается этим merge** (epic-7-retrospective остаётся optional-слотом).
"""

payload = json.dumps({
    "title": "feat(story-7-4): внешние MCP-серверы как класс Connector — клиентская машина 2026-07-28 + живые MAC-пробы + тенент-скоупед внешний словарь (Epic 7: 4/4)",
    "head": "codex/story-7-4-external-mcp-connectors",
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
