# Fresh-context verification — product, pricing & trajectory

Дата проверки: 2026-09-11  
Режим: normal validation; проверены только load-bearing claims из `offers-pricing-r1-1.md` и `position-trajectory-r1-1.md`.  
Правило статуса: `verified` требует независимого подтверждения там, где оно доступно; несколько страниц одного vendor не считаются независимыми. Для category availability заявления разных vendors считаются независимыми свидетельствами наличия функции в категории, но не доказательством качества или production maturity.

## Итог

| Ref | Проверяемый claim | Outcome | Краткое основание |
|---|---|---|---|
| VP-1 | LLM/RAG, tool actions/MCP и CRM-интеграции стали category baseline у нескольких независимых игроков | **verified** | MANGO прямо заявляет LLM, автономных агентов и RAG; ZIAX — RAG+LLM, MCP/A2A, CRM/API и действия с внешними системами; Exolve — база знаний, CRM actions, REST/webhooks/MCP; Robovoice — GPT и готовые Bitrix24/amoCRM-коннекторы с обновлением CRM. Это независимые vendors, поэтому наличие этих классов функций в категории подтверждено. Не подтверждены сопоставимое качество, latency, permissions/retries/idempotency и production reliability. |
| VP-2 | Публичный split около 3,5–6,5 ₽/мин против 30–46 ₽/мин у «Реплики» плюс setup | **verified** | Нижняя публичная поверхность подтверждается Aimylogic (эффективно ≈3,50–5,45 ₽/мин по включённым пакетам), Exolve (AI-agent 6,5 ₽/мин с НДС), MANGO (generic robots 4,5 ₽/мин) и Tomoru (4,4–6,4 ₽/мин). «Реплика» показывает 46 ₽/мин, 40 ₽/мин и 30–39 ₽/мин по трём пакетам плюс 20 000 ₽ разово. Разрыв виден, но сравнение не like-for-like и годится только как directional pricing signal. |
| VP-3 | On-premise явно предлагают TWIN, Robovoice и ZIAX | **verified** | Три независимые текущие product pages прямо описывают установку в контуре/на серверах заказчика. Это подтверждает category availability on-prem, но не цены, сроки или эквивалентность конфигураций. |
| VP-4 | Yandex voice agents — текущая прямая platform threat | **verified** | Текущая документация Yandex AI Studio описывает Realtime API, voice agents, file/web search, MCP и function tools; отдельная актуальная инструкция описывает интеграцию с Asterisk. Датированный запуск для контакт-центров с SIP подтверждён сообщением Яндекса и освещением CNews. Scope нужно сузить: это прямой build-platform/infrastructure substitute, а не доказанный turnkey managed-agent аналог; GA-статус SIP после первоначального Preview и production quality не подтверждены. |
| VP-5 | TWIN показывает самый явный свежий cadence agent product | **unverified** | Собственный news feed TWIN действительно показывает updates №83–89 с 16.06 по 31.08.2026: transfer/tooling, LLM token and interruption controls, CRM integrations, reports/auth, RAG, knowledge-base tools, variables/clientMetadata. Но независимого publisher для cadence не найдено, а сравнительный superlative «самый явный среди исследованных» не следует из одного vendor feed. Не overturned: фактическая последовательность релизов поддержана первичным источником, но остаётся single-source и должна быть medium-confidence. |

Сводка: **verified 4; disputed 0; unverified 1; overturned 0**.

## Обязательные caveats к ценовому сравнению

- **Разный product scope.** Aimylogic тарифицирует NLU-минуты в сценарной платформе; MANGO 4,5 ₽/мин относится к строке `Роботы` и публично не привязан к индивидуально разрабатываемому LLM/RAG `ИИ Сотруднику`; Exolve 6,5 ₽/мин — отдельный AI-agent tier. «Реплика» включает voice + text agent, поддержку/обучение, CRM autonomy, API и до 100 параллельных звонков.
- **Телефония и add-ons различаются.** Aimylogic требует собственный SIP; Tomoru прямо исключает стоимость телефонии; у Exolve исходящий total дополнительно зависит от telecom traffic, маркировки и сервисов. У «Реплики» 20 000 ₽ покрывают настройку CRM и телефонии, но включение самого телефонного трафика в минуты публично не установлено.
- **Setup не уникален для дорогого сегмента.** Tomoru отдельно показывает запуск 79 000 / 159 000 / 299 000 ₽; поэтому вывод «рынок монетизирует внедрение» поддержан, но противопоставление low-price = no-setup неверно.
- **Внутренняя арифметика «Реплики».** 49 900 ₽ / 1 150 минут ≈ 43,4 ₽/мин, хотя страница подписывает пакет как 40 ₽/мин. Диапазон 30–46 ₽/мин корректен как advertised range, но не как нормализованный effective rate каждого пакета; для 2 000+ минут 30–39 ₽ также зависит от индивидуального объёма.
- Все перечисленные pricing pages недатированы; значения подтверждают состояние страниц на дату проверки, но не историю или гарантию сохранения тарифа.

## Источники проверки

### Category baseline

- MTS Exolve — AI-agent/knowledge base, CRM actions, REST/webhooks/MCP, named CRM/ERP: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/
- MANGO OFFICE — LLM, autonomous AI-agents, RAG: https://retail.mango-office.ru/products/contact-center/ai/voice-robot/
- ZIAX — RAG+LLM, MCP/A2A, CRM/API, external systems: https://ziax.ru/
- Robovoice — GPT, Bitrix24/amoCRM connectors and CRM updates: https://slsoft.ru/products/robovoice/
- Yandex AI Studio voice-agent tools — function calls, file/web search and MCP: https://aistudio.yandex.ru/ru/docs/ai-studio/concepts/agents/realtime

### Pricing

- Aimylogic packages and included minutes: https://aimylogic.com/ru/price
- Aimylogic own-SIP requirement: https://help.cloud.just-ai.com/aimylogic/smart_calls/sip_settings/
- MTS Exolve tier prices and outbound cost caveat: https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/
- MANGO contact-center tariff (`Роботы` 4,5 ₽/мин; Open API separately): https://static.mango-office.ru/products/contact-center/tariffs/
- Tomoru per-minute pricing, prepayment and telephony exclusion: https://tomoru.ru/price
- Tomoru setup packages: https://tomoru.ru/plan
- «Реплика» packages and 20 000 ₽ integration payment: https://thereplica.ru/

### On-premise

- TWIN.On-Premise: https://twin24.ai/products/large-business/
- Robovoice on-cloud/on-premise: https://slsoft.ru/products/robovoice/
- ZIAX On-Premise/Cloud: https://ziax.ru/

### Yandex current platform threat

- Current voice-agent documentation: https://aistudio.yandex.ru/ru/docs/ai-studio/concepts/agents/realtime
- Current Asterisk integration guide: https://aistudio.yandex.ru/ru/docs/ai-studio/operations/agents/voice-agent-asterisk-integration
- Yandex announcement, 2026-05-28 (SIP was Preview at launch): https://yandex.ru/company/news/28-05-2026-04
- CNews coverage, 2026-05-28: https://ai.cnews.ru/news/line/2026-05-28_rossijskie_kompanii_smogut_1

### TWIN cadence

- TWIN news index with updates №83–89 and dates: https://twin24.ai/company/news/
- Update №84 detail (LLM token limit, interruption controls, CRM integrations): https://twin24.ai/company/news/twin-obnovlenie-84-ogranichenie-na-kolichestvo-tokenov-v-otvete-llm-dlya-ii-agentov-nastrojka-raboty-s-perebivaniyami-dlya-ii-agentov-i-novye-integraczii-s-crm/
- Update №89 detail (agent variables/clientMetadata and other changes): https://twin24.ai/company/news/twin-obnovlenie-89-fiksirovanie-znachenij-peremennyh-ii-agenta-novyj-fonovyj-shum-dlya-ii-agentov-podderzhka-peremennyh-clientmetadata-v-ii-agentah-i-drugoe/

## Status changes recommended for synthesis

1. Keep category-baseline conclusion **verified/high for availability**, but retain medium/low confidence for capability quality and operational semantics.
2. Keep public pricing split **verified**, but describe it as **non-like-for-like advertised pricing**, not a normalized unit-economics comparison.
3. Keep on-premise conclusion **verified**.
4. Keep Yandex as a **verified current platform/build threat**, while avoiding the stronger claim that it already matches turnkey managed agents; SIP GA remains open.
5. Downgrade TWIN's comparative cadence claim from **high** to **unverified / medium single-source**; the release sequence itself remains supported, the superlative does not.
