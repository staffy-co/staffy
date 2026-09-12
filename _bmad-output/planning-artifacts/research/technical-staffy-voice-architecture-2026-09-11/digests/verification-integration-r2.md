# Verification — integration & reliability round 2

Дата проверки: 2026-09-11  
Проверяемый файл: `integration-reliability-r2-1.md`  
Режим: независимая spot-check проверка только по первичной документации поставщиков; 7 web calls, 8 distinct primary URLs. Проектные файлы, кроме указанного digest, не читались.

## Итоговый вердикт

**SUPPORTED.** Нагрузочный вывод digest — **единый канонический domain API плюс отдельный проверенный adapter на каждого поставщика** — поддержан контрактами ROSSKO и ARMTEK. Они различаются transport/auth/wire schema, но оба требуют не boolean-результата, а модели `supplier_orders[]` + line outcomes/allocations + partial result. Это архитектурный вывод из подтвержденных контрактных различий, а не гарантия поставщиков.

Ни один из проверенных нагрузочных тезисов не overturned. Остаточные пункты, отмеченные ниже как **unverified**, корректно оставлены в digest как procurement/test gates, а не выданы за capability.

## Claim-level outcomes

| Проверяемый тезис | Outcome | Результат проверки |
|---|---|---|
| Exolve dynamic routing — синхронный POST/JSON-RPC `getControlCallFollowMe` с `call_sid`, выбором single/sequential/parallel target, `event_URL` и `client_id` | **verified** | Официальная инструкция прямо задает request/response contract, поля `sip_id`, `numberA`, `call_sid`, `redirect_type`, `followme_struct`, `event_URL`, `client_id`, target timeout и SIP/PSTN target. |
| Exolve events дают устойчивую корреляцию и фактический human-answer/leg completion | **verified** | `call_id` документирован как одинаковый во всех событиях одного вызова; присутствуют `call_sid`, `application_uuid`, `to`, `date_time`, optional `client_id`. Event `s` — фактический ответ B-party; `h` — окончание вызова; `d` — окончание с cause для каждого target при sequential/parallel forwarding. |
| Exolve callback delivery/auth contract и response deadline | **unverified** как capability; **verified bounded gap** | На двух прочитанных страницах нет documented signature/mTLS, retry/backoff, ordering/at-least-once, acknowledgement rule или deadline routing response. Exact-string checks по `подпис`, `повтор`, `порядок`, `deadline` совпадений не дали. Это отсутствие только в этих public pages, не доказательство отсутствия private contract. |
| Exolve transfer/bridge уже соединенного bot call | **unverified** как capability; **verified bounded gap** | Прочитанный contract касается pre-answer forwarding и последующих lifecycle events. Он не описывает `transfer`, bridge, SIP REFER/Replaces или attended transfer; поиск `transfer` на обеих страницах совпадений не дал. Digest корректно превращает это в hard procurement/demo gate. |
| ROSSKO exact reconciliation через `GetOrders` после получения vendor IDs | **verified** | `GetOrders` прямо поддерживает lookup по одному/списку order IDs, периоду, последним N и типу; сама документация настоятельно рекомендует получать заказы именно по identifiers. |
| ROSSKO ambiguity, если `GetCheckout` timeout случился до возврата `OrderIDS` | **verified bounded risk** | Public `GetCheckout` input перечисляет auth, delivery/payment/contact и part fields, но не client request/idempotency key; response содержит `OrderIDS`, successful `ItemsList` и `ItemsErrorList`. Public `GetOrders` lookup перечисляет vendor IDs/time/latest/type, но не exact client correlator. Поэтому automatic retry/reconciliation до получения IDs контрактом не оправданы; exact recovery остается **unverified**. |
| ARMTEK использует отдельный REST/Basic-auth order contract | **verified** | Официальная главная страница называет WS REST и Basic Authentication; order page v1.1.7 описывает `createOrder`, method POST, и пример `/api/ws_order/createOrder?format=json`. Это явно другая wire/auth family, чем SOAP ROSSKO с KEY1/KEY2 в payload. |
| ARMTEK create result line-oriented и allocation-aware | **verified** | Request line содержит `PIN`, `BRAND`, `KWMENG`, recommended `KEYZAK`, optional `PRICEMAX`, `DATEMAX`, `COMPL_DLV`. Response line содержит `RESULT[]`, `REMAIN`, `ERROR`, `ERROR_MESSAGE`; allocation rows содержат allocated `KWMENG`, price/date, `VBELN` и block state. Массив allocations с order number на каждой записи поддерживает возможность split allocation/multiple created orders; это структурный вывод из schema. |
| 1С extension сохраняет типовую конфигурацию на поддержке и требует compatibility checks | **verified** | Официальная страница 1С говорит: типовую конфигурацию не нужно менять; изменения находятся в расширении; типовая конфигурация остается на полной поддержке и автоматически обновляется. При подключении проверяется, что заимствованные объекты основной конфигурации не изменились. Это подтверждает extension-first и release certification, но не гарантирует business compatibility без тестов. |
| 1С предоставляет explicit all-or-nothing transactions | **verified** | Developer Guide определяет transaction как indivisible/all-or-nothing и прямо перечисляет `BeginTransaction()`, `CommitTransaction()`, `RollbackTransaction()`. Он также описывает implicit transactions для data-modification actions/event handlers. |
| Произвольный HTTP-service handler 1С автоматически является одной транзакцией | **unverified** | Проверенный transaction section такого утверждения не делает и HTTP handler boundary не определяет. Поэтому требование digest об explicit transaction вокруг local mutation + operation record консервативно и поддержано; фактическое поведение нужно fault-test на целевой установке. |

## Почему canonical domain API + adapters поддержан

1. **Transport/auth нельзя унифицировать одним wire connector:** ROSSKO документирует SOAP v2.1 и `KEY1`/`KEY2` request fields; ARMTEK документирует REST, Basic Authentication и POST `createOrder`.
2. **Общий domain shape существует:** оба контракта возвращают результат на уровне строк и допускают частичный outcome. ROSSKO возвращает `OrderIDS`, `ItemsList` и `ItemsErrorList`; ARMTEK — per-line remainder/error и массив allocations с supplier order number.
3. **Lowest-common-denominator boolean недостоверен:** он потеряет multiple order IDs, line failures, remainder и allocation detail.
4. **Vendor semantics остаются локальными adapter-у:** lookup/correlation, status mapping, full-delivery flag, cancellation/edit rules и unknown-after-submit behavior различаются либо не полностью опубликованы.

Следовательно, digest правильно отделяет стабильную domain-команду/результат от supplier-specific transport, authentication, request mapping, reconciliation и compensation logic.

## Не подтверждено и должно остаться gate

- Exolve callback authentication, delivery retries/backoff, ordering, acknowledgement and routing response deadline.
- Exolve mid-call attended transfer/bridge semantics.
- ROSSKO exact client correlation/idempotency and safe automatic recovery before receipt of `OrderIDS`.
- ARMTEK XML response support в рамках этой spot-check проверки; JSON example подтвержден. Это не влияет на adapter conclusion.
- Automatic whole-handler transaction для custom 1С HTTP service и compatibility конкретной client configuration/release.

## Primary-source register

1. Exolve — dynamic forwarding to URL: https://docs.exolve.ru/docs/ru/instructions/call-forwarding-to-url/
2. Exolve — inbound call events: https://docs.exolve.ru/docs/ru/api-reference/statistics-api/call-events/
3. ROSSKO — `GetCheckout`: https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout
4. ROSSKO — `GetOrders`: https://api.rossko.ru/index.php?action=SOAP&path=/GetOrders
5. ARMTEK — Web Services overview/auth: https://ws.armtek.ru/
6. ARMTEK — order services v1.1.7: https://ws.armtek.ru/?alias=order&page=service
7. 1С — configuration extensions: https://v8.1c.ru/platforma/rasshireniya/
8. 1C:Enterprise 8.3.23 Developer Guide — transactions: https://kb.1ci.com/1C_Enterprise_Platform/Guides/Developer_Guides/1C_Enterprise_8.3.23_Developer_Guide/Chapter_9._Data_operations/9.2._Transactions/9.2.1._General_information/

## Outcome tally

- **verified:** 7 contract/platform claims
- **verified bounded gap/risk:** 3 absence-or-ambiguity claims
- **unverified:** 5 vendor/platform capabilities intentionally kept behind gates
- **disputed:** 0
- **overturned:** 0

