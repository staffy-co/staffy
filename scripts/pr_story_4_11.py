#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 4-11 — python-JSON body assembly (precedent pr_story_4_10.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ.get("STAFFY_GH_TOKEN") or open("/tmp/.staffy_gh_token").read().strip()
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Story 4.11 — Сквозной бизнес-результат пилота автозапчастей (первый срез: Исход обращения + Отчёт пилота — ЧИСТАЯ read-time деривация над живым материалом контура)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **ЧИСТЫЙ read-time срез — 0 миграций, 0 пишущих путей, 0 OpenAPI-изменений**: исход обращения и агрегаты пилота — НИКОГДА не хранимое слово, живая деривация записанного материала; повторное чтение неизменного мира даёт тот же ответ; поздний факт (запись поставщика 4-9, коррекция 4-8) меняет чтение БЕЗ переписывания истории (факты append-only по построению — AC-13 read-time-исполнением; versioned indicator store честно отложено).
- **Воронка 10 стадий BY NAME** {{inquiry_accepted, vehicle_profile_confirmed, applicability_proven, client_order_created, manager_verified, supplier_sent, fulfilled, paid, cancelled, returned}} **со состояниями {{proven, not_reached, unknown_pending}} BY NAME**: proven ТОЛЬКО при наличии evidence BY NAME с временем и источником; терминальные стадии fulfilled/paid/cancelled/returned — unknown_pending ВСЕГДА с пинами причин (OQ-4/OQ-3/№215) — «не доказано и не опровергнуто», НЕ ноль и НЕ исключение (AC-14).
- **Атрибуция первого принятого inbound канал-события** {{provider, channel, provider_event_id, время}} над Timeline кейса; campaign/referrer — unknown СЛОВОМ («источник не передал — не восстанавливается догадкой»; envelope v1 заморожена, ингресс НЕ трогается — AC-1).
- **Measurement Dictionary v1 (compile-time, рендерится на странице отчёта)**: измеримые {{case_volume, stage counts, конверсии ТОЛЬКО между ДОКАЗАННЫМИ соседними стадиями (numerator/denominator BY NAME; unknown_pending-стадии в знаменатели НЕ входят), corrections_count, handoff_count, recovery_count, unknown_outcome_share}}; НЕизмеримые {{revenue (Owning System 1С не подключён OQ-4; созданный заказ ≠ выручка AC-4), margin (закупочные цены за фенсом 4-9 — «неизвестна», НЕ 0 AC-5), variable_costs (usage 1-11 тенант-уровня без атрибуции на кейс AC-7), human_minutes (утверждённого способа пересчёта нет AC-8)}} — строками-словами причин, чисел НЕТ, нули НЕ подставляются.
- **Отчёт пилота — НОВАЯ страница /app/tenants/{{tenant_id}}/pilot-outcome** (period=YYYY-MM, отсутствие = «весь период наблюдения»): КАЖДОЕ число с размером выборки и периодом (AC-11); OQ-7 гейт-слово в заголовке — baseline/когорт/порогов НЕТ: наблюдаемые показатели без заявления об улучшении, корреляция ≠ причинность (AC-12); доля неизвестных исходов — отдельной строкой (AC-14); словарь измерений v1 с определениями (AC-9); счётчики контуров коррекций/Handoff/Recovery.
- **Панель «Исход обращения» (id=case-outcome) на странице кейса** после «Обслуживания заказа»: цепочка исхода (источник → диалог → кейс → рабочий элемент → профиль → запрос детали → подтверждение → операция → внешний ID заказа BY NAME) + таблица 10 стадий (состояние словом + evidence + время + источник) + атрибуция.
- **Честные границы**: записи выручки/оплаты менеджером НЕ заводятся; baseline/сравнение/улучшение НЕ строятся (OQ-7); campaign/referrer НЕ добавляются в envelope; usage-затраты НЕ атрибутируются на кейсы; человеческие минуты НЕ выводятся из lifecycle; экспорт/JSON-API метрик НЕТ (HTML-поверхности); замороженные машины 2-3/2-13/3-6/3-8/4-6..4-10 — только чтение существующими портами (name-only дифф); VIN/текст диалога/телефон НЕ попадают в агрегаты (структурный зонд полей вью); a11y 320px словами состояний, без опоры на цвет (AC-18).

### Steps
- SM (ff06e6d + 0c89c7a): спека первого среза + qa_spec_scan_4_11.py CLEAN (fail=0 warn=0) + DEVLOG 265-sm; CHECKPOINT 1 одобрен владельцем (2439544) — синхронный флип spec + sprint-status в ready-for-dev
- step-01 (a781ae9): домен pilot_outcome — словари compile-time (10 стадий/3 состояния/атрибуция/Measurement Dictionary v1) + DeriveCaseOutcome + DerivePilotReport + unit-матрица со структурными зондами честности
- step-02 (ce3ac9e + 1a6adb1): порт PilotCaseSource + api/app PilotOutcomeService (сборка материала, review-гейт, честная деградация, отчёт периода) + проводка composition-roots + app unit-тесты
- step-03 (a629e08 + f2a8770): панель «Исход обращения» + страница «Отчёт пилота» GET /app/tenants/{{tenant_id}}/pilot-outcome + маршрут + 5 wire-тестов (рендер/деградация/a11y)
- step-04 (185cff2 + c4b620e + ca6315d + 5ca09d4): интеграционная eval-матрица 12 строк живым PG (атрибуция/профиль/применимость/заказ/проверка/отправка/late-flip/двойной-учёт/агрегат+период/RLS/скоуп) + journeys z48 «pilot-outcome» (новая 53-я: полный путь + период-фильтр + a11y 320px + секреты 0)
- step-05 (2f29fb0 + 5ae1afe): qa_step05_4_11_scan.py CLEAN fail=0 warn=0 + свежий полный регресс + closure (Spec Change Log, спека done, sprint-status done — Epic 4: 11/11) — финальный feat БЕЗ [skip ci]

### Regression
build/vet OK · unit 32 пакета зелёные · интеграция ×2 TZ (UTC 67.6s + Europe/Moscow 69.6s) · journeys 53/53 (z48 — новая 53-я, 4.4m) · wire/unit матрицы зелёные (unit-матрица домена + app unit + 5 wire + 12 интеграционных строк) · git diff --check чист · step-05 qa-скан CLEAN (0 миграций/0 пишущих путей в новом контуре/замороженные машины 2-3/2-13/3-6/3-8/4-6..4-10 name-only/ценовых полей 0/VIN-диалог-телефон полей 0/import-graph домена stdlib+platform-id AD-10/секреты 0).

Ветка: 14 коммитов от main 93f7e26, 26 файлов, +6702/−1122.

Epic 4: 11/11.
"""

payload = json.dumps({
    "title": "feat(story-4-11): Сквозной бизнес-результат пилота автозапчастей — Исход обращения + Отчёт пилота (read-time, 0 миграций)",
    "head": "codex/story-4-11-pilot-business-outcome",
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
try:
    with urllib.request.urlopen(request) as response:
        result = json.load(response)
        print(json.dumps({"number": result["number"], "url": result["html_url"], "head": result["head"]["sha"]}, ensure_ascii=False))
except urllib.error.HTTPError as error:
    print(f"HTTP {error.code}: {error.read().decode()[:500]}")
    sys.exit(1)
