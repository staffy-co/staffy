#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 5-1 — python-JSON body assembly (precedent pr_story_4_11.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ.get("STAFFY_GH_TOKEN") or open("/tmp/.staffy_gh_token").read().strip()
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Story 5.1 — Допуск каналов и маршрутов Montagio (первый срез: Montagio Admission Registry — ЧИСТЫЙ read-time вывод над живыми фактами)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **ЧИСТЫЙ read-time срез — 0 пишущих путей**: допуск = деривация, не запись; повторное чтение неизменного мира — байт-стабильно тот же ответ; ЕДИНСТВЕННАЯ миграция — SQL-зеркало CHECK словаря гейтов 00014 (14→20, урок 4-1 заранее в спеке; honest-Down прецедента 00013).
- **gateDictionary 14→20** {{+OQ-7A, OQ-8, OQ-9, OQ-10, OQ-13A, OQ-18}} — канон GATE-REGISTRY поимённо (criticality/roles/evidence/safe state; RouteScoped — только OQ-11); расширение словаря = новая версия спеки (прецедент 4-1 второй раз); AG-3 остаётся planning-артефактом.
- **Словарь маршрутов 8 слов compile-time** {{novofon_voice, montagio_web, partner_intake, b2b_intake, protected_web_continuation, human_inbox, avito_candidate, public_commercial}} с **ЧЕТЫРЁХСТАДИЙНОЙ read-time лестницей** {{candidate → verified → controlled → live}}: controlled = verified + действующие PASS {{OQ-7A, OQ-8, OQ-11 (route-scoped: novofon phone in/out, web-маршруты web_chat in/out), AG-1, AG-2, AG-4 [+голос OQ-12/OQ-13B]}}; live = controlled + {{OQ-7B, OQ-14}} — Execution order дословно; конъюнкция: отсутствие/устаревание блокирует ТОЛЬКО затронутый маршрут (AC-13/AC-14); лестница Олега 4-1 не тронута — два словаря сосуществуют.
- **Декларации каталога**: novofon-sip 1.1.0 (12 read-only query AC-3 ПОИМЁННО, ProbeAllowed только read-only, AdapterImplemented false — «общая документация Novofon не доказательство тарифа и аккаунта»), montagio-web 1.0.0 (9 AC-5; command.outbound-reply — существующее слово 4-1, структурно неисполнимо), partner-intake 1.0.0 (5 AC-6, фенс неполной заявки), b2b-intake 1.0.0 (5 AC-7, полномочия unknown не подразумеваются) — полная семантика 1-9; immutability 1.0.0-пинов; **Avito-деклараций 0** (AC-8).
- **Pack-пин Domain Pack «Услуги» montagio-services@≥1.0.0** живым чтением sealed-манифестов reused (AC-1: общее ядро, fork/tenant-модуль невозможны AD-1/AD-2/CI); чужой пак никогда не пинит (fail-closed).
- **Словарь утверждённых формулировок AC-11** (5 слов compile-time: Montagio — договорная сторона; специалист по поручению; партнёрская модель не раскрывается без правового требования; ложные заявления запрещены; best-effort без SLA OQ-13A) — формулировки словарём, не генерацией.
- **avito_candidate — ВЕЧНЫЙ candidate** CapWord OQ-18 (scraping/имитация/обход запрещены); **public_commercial — capped candidate** (машины Order/payment 5-8..5-12/5-17 не построены; OQ-10 блокирует публичный маршрут, НЕ блокируя закрытые validation-сценарии AC-12).
- **Поверхности**: машинный GET /v1/tenants/{{t}}/montagio/admission (200/401/403/404 membership-до-материала, пустое чтение без 400) + страница «Пилот услуг Montagio» GET /app/tenants/{{t}}/montagio (Route Matrix 8×колонки словами, 4 стадии текстом, CapWord/фенсы, гейты с owner/next, evidence в закрытых details, секция AC-11, a11y 320px/клавиатура/skip-link) + OpenAPI +1 путь/+5 closed-схем с enum-зеркалами 8/6/4 (drift = красный контракт-тест).
- **retro-4-2 исполнен SM-фазой** (check-continuity epic-валидация + CI-порог journeys ≥ 48 spec-файлов — хвост трёх переносов закрыт).
- **Честные границы**: адаптеры Novofon/Django/партнёра/B2B не реализуются (AG-4), реальные credentials запрещены; связывание заявки с Case — 5-2; машины Order/payment — 5-8..5-12/5-17; OQ-16/OQ-17 открыты; замороженные машины 4-1 (pilot.go/pilot.html) байт-стабильны.

### Steps
- SM (08a9766 + 3877dea): спека первого среза + retro-4-2 infra (epic-валидация + CI-порог, 4 пути ALL PASS) + DEVLOG 274-sm; CHECKPOINT 1 — развилка A/B решена делегированным решением владельца со сверкой PRODUCT-VISION.md: курс A (42a8542 + 78abd11); «Северная звезда» в каноне (f912536 + a4c4a91, файл в main 8a37d5d)
- step-01 (d96b63c + 0b25422): домен pilot_montagio (лестница 4 стадии + словарь 8 маршрутов + AC-11 словарь + DeriveMontagioAdmission идемпотентный) + gateDictionary 14→20 + миграция 00014 CHECK-зеркало + 15 unit-тестов поимённой матрицы
- step-02 (575cdd5 + d2955c3): каталог 4 декларации полной семантики 1-9 + кросс-зеркало каталог↔словарь в обе стороны + 9 тестов
- step-03 (a55d6d9 + 883fba7): app read-only живыми чтениями reused (0 записей, reflect-зонд портов) + транспорт API/страница + OpenAPI + 14 тестов (app 8 + transport 6)
- step-04 (dc54a37 + 1f80b5d): интеграционная матрица живым PG 12 рядов (миры 385/386, корреляции e2xx) + journeys z49 (живой gate-walk, 54/54) + регресс-ремонт пина 14→20 интеграционного readiness_test (пропуск step-01) + gofmt новых файлов ветки
- step-05 (738bc02 + c3bb5e3): QA-зонды поимённо (нулевые записи/fail-closed/идемпотентность/зеркала/Avito 0/замороженные машины/retro-4-2 жив) + финальный регресс всех контуров свежими прогонами + honest-границы deferred-work 68→76 + флип спека+sprint-status → done (Epic 5: 1/19) — feat БЕЗ [skip ci]

### Regression
build/vet OK · unit 33 пакета зелёные (-count=1) · интеграция ×2 TZ (UTC 65.4s + Europe/Moscow 66.1s, полным прогоном) · journeys 54/54 свежим серийным прогоном (z49 — новая 49-я, 4.3m; 49 spec-файлов ≥ порога 48) · матрица 12/12 поимённо · check-continuity OK (fail=0) · git diff --check чист.

Ветка: 16 коммитов от main 8a37d5d, 29 файлов, +6069/−1737.

Epic 5: 1/19.
"""

payload = json.dumps({
    "title": "feat(story-5-1): Допуск каналов и маршрутов Montagio — Montagio Admission Registry (read-time, 1 миграция-зеркало)",
    "head": "codex/story-5-1-montagio-admission-registry",
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
