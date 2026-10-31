#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 8-2 — python-JSON body assembly (precedent pr_story_8_1.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ.get("STAFFY_GH_TOKEN") or open("/tmp/.staffy_gh_token").read().strip()
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Story 8.2 — Модельный адаптер: версионируемая сборка промптов, бюджеты раундов/времени/стоимости, честные presence (адаптерная сторона «мозга» — формы ДО живой модели)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **Версионируемая сборка промптов ПЛАТФОРМЕННЫМ кодом** internal/planner/assembly.go: RoundMaterial — закрытая детерминированная форма (байтовая идентичность — тест; проверяемость = детерминизм + версия, **персистенции материала НЕТ** — без дублирования PII-материала дела AD-10/AD-11; слота для секретов/CoT/извлечённого материала НЕ СУЩЕСТВУЕТ — тест закрытого набора ключей); материал несёт факты с ПОЛНОЙ Provenance (AD-8 authority/source/version/time), режим+лестницу, пины политики РЕФЕРЕНСАМИ (материал политик не входит), инвентарь read-инструментов (имя@версия + схемы словарём tools.ToolFieldTypes БЕЗ расширения), **§8.1-пин раскрытия ИИ доезжает в материал**, fallback-политику, бюджет реплики, гид закрытой формы предложения (schema_version + словари intent/safety/action-kind); слова AssemblyVersion «assembly/1.0.0» + RuntimeBuildRef «planner/1.0.0+assembly/1.0.0».
- **ModelPort эволюционировал до МАТЕРИАЛА раунда**: RoundRequest{{Material}} без контекста — модель структурно НЕ видит сырой Turn Context (**AD-8 evidence-скоупинг по построению**, не по дисциплине адаптера); repair-причина первого отказа путешествует ВНУТРИ материала; рефайнмент ожидания 8-1 «сборка на стороне адаптера» задокументирован в спеке: адаптер без контекста не может собрать неотслеживаемый промпт — единственное исполнение ценности истории.
- **Бюджеты закрытыми формами** internal/planner/budgets.go: MaxRounds ∈ [1,2] (**бюджет сужает структурный предел машины, декларация 3 = поимённый отказ**), RoundTimeout > 0, RoundCostUnits/MaxCostUnits — платформенные честные единицы; + **ограниченный адаптер internal/adapters/modelbounded** (обёртка над любым ModelPort, per-ход без скрытого состояния): гард раундов / дедлайн context.WithTimeout / стоимостной учёт; **ЛЮБОЕ исчерпание = поимённая честная ошибка** {{ErrBudgetRounds, ErrBudgetTimeout, ErrBudgetCost}} → машина завершает ход failed + утверждённый безопасный fallback — **unknown без слепого retry, никогда молчаливое продолжение** (AD-7 «execution time and cost are bounded» дословно); внутренние ошибки адаптера проходят как есть — не маскируются словами бюджета.
- **Машина эволюционировала**: шаг сборки перед каждым раундом; NewMachine несёт валидируемые бюджеты (невалидная декларация = поимённый отказ конструкции); каждая запись хода несёт **ПОЛНУЮ Provenance** {{версия сборки входа + декларация бюджетов + schema_version предложения + Build}}.
- **Журнал migrations/planner/00002**: assembly_version text NULL + budgets jsonb NULL — **NULL истории до 8-2 = честное отсутствие, sealed-записи не пересчитываются (AD-10)**; версии слов НЕ заморожены CHECK-словарями (слово версии эволюционирует по дизайну — семантика connectorprobe); гранты/RLS/FORCE 00001 наследуются нетронутыми (append-only грантами: UPDATE/DELETE запрещены правами); NULL-история читается честно (интеграционный тест вставляет запись до 8-2 админ-пулом).
- **RuntimeDictionary честные presence (AR-17)**: prompt_assembly → Present=true Ref=assembly/1.0.0 (прецедент tool_schemas 7-2: поверхность есть, продакшн-вызыватель 8.4); runtime_build → Present=true Ref=planner/1.0.0+assembly/1.0.0 (когерентно журналу); **model_route → честный false со специфичной запиской** (форма маршрута и bounded-адаптер построены; живой маршрут и credentials — за AG-4; вызыватель 8.4); владелец-метки 7 компонентов Epic 2→8, media_route → Epic 9; производные ТОЛЬКО новых seal (app/release.go не тронут — AD-10); 2 задокументированных отклонения от буквы Code Map в Spec Change Log (transport release_view 1 строка метки; media_route Epic 9).
- **Scripted-модель синтезирует ИЗ МАТЕРИАЛА**: матрица исходов 8-1 сохранена {{valid, malformed_undeclared, malformed_type, foreign_version, repairable, unrecoverable, adapter_error}}; mode из материала; **§8.1-пин → реплика «Я — искусственный интеллект.»** (контрактная сторона §8.1 доказана scripted-матрицей); материал без слова версии сборки = поимённая ошибка (синтез из ничего невозможен); main.go НЕ подключает — продакшн-вызывателей НОЛЬ (вызыватель 8.4).
- **Journeys живьём** (release-поверхность затронута — отличие от 8-1): release-страница рендерит present-строки («присутствует: assembly/1.0.0», «присутствует: planner/1.0.0+assembly/1.0.0») — новые ассерты в zzzzzz-release.spec.ts; ассерт владельца выживает подстрокой «AI Runtime».
- **Вертикальная чистота**: drift-зона +internal/adapters/modelbounded (зонд 0 фраз живым прогоном — словарь фраз НЕ растёт); словари 8-1 НЕ расширены (0 новых слов); 0 правок потребителей (cases/action/tools/mcp/mcpclient/externaltools/main.go — живой git-зонд); 1 SQL-миграция (объявлена в спеке).

### Steps
- SM (c39d4e0 + a1ad297): спека модельного адаптера + DEVLOG 327
- CHECKPOINT 1 (6f29a29): закрыт явным «Продолжайте» владельца — флип ready-for-dev + sprint-status синхронно (урок 3-1)
- step-01 (1de22b9): формы сборки + бюджеты + валидация + unit-матрица (детерминизм/полнота/закрытость/бюджеты)
- step-02 (f338d5e): порт-материал + машина (шаг сборки, Provenance) + modelbounded + scripted-из-материала + машинные тесты композиции
- step-03 (6f86126): миграция 00002 + стор + PG-контур (bounded-составы живьём, NULL-история)
- step-04 (5e53085): presence-производные + journey-ассерты + drift-зона + ПОЛНЫЙ регресс
- step-05 (75437db + TBD-fill 513f3b7): QA-closure — спека done, sprint-status 8-2 done (epic-8 in-progress 2/6), deferred-work (запись 8-1 о бюджетах/сборке закрыта заменой; +3 honest-границы), DEVLOG 328

### Regression
build/vet OK · unit **45 пакетов** зелёные (-count=1) · интеграция ×2 TZ живым PG (**UTC 67.2s + Europe/Moscow 68.6s**) · journeys **57/57 живьём** (4.4m — release-поверхность рендерит present-строки) · вертикальный зонд: **0 фраз** в зоне +modelbounded · gofmt новых файлов чист · git diff --check чист.

Ветка: 9 коммитов от main 26ee9ad, 18 файлов (+~1300 строк): 6 исходников planner + 5 тестов planner + 2 modelbounded + 2 modelscripted + 1 миграция + 1 стор + 3 integration + TestMain + 3 employee domain + 1 transport (1 строка метки) + 1 drift + 1 journey + артефакты + DEVLOG.

Epic 8: **2/6** — контракт хода + адаптерная сторона существуют; следующий срез — 8.3 «Tool-раунды над реестром 7.2 и внешним словарём 7.4» по сигналу владельца.
"""

payload = json.dumps({
    "title": "feat(story-8-2): модельный адаптер — версионируемая сборка промптов платформенным кодом, бюджеты раундов/времени/стоимости + bounded-адаптер, журнал Provenance 00002, RuntimeDictionary presence (Epic 8: 2/6)",
    "head": "codex/story-8-2-model-adapter",
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
