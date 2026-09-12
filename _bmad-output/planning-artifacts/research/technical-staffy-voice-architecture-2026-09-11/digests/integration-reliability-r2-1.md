# Integration & reliability digest — round 2

Дата исследования: 2026-09-11  
Фокус: unresolved pilot contracts — Exolve callbacks/handoff, ROSSKO lifecycle/idempotency, второй supplier contract, 1С extension/transaction boundary.  
Источники: 8 официальных/public vendor sources; project context не читался, кроме разрешенного `verification-integration.md` и собственного round-1 digest.

## Decision delta

1. **Exolve пригоден как carrier-side inbound router и источник lifecycle events**, включая устойчивые call identifiers, client correlation, событие фактического ответа и причины завершения legs. Но публичный контракт не подтверждает authenticated/signed inbound callbacks, delivery retries, response deadline или transfer уже соединенного с ботом вызова. Для mid-call handoff нужен отдельный подтвержденный bridge/transfer primitive voice runtime/PBX.
2. **ROSSKO reconciliation надежен только после получения vendor order IDs.** `GetOrders` рекомендует lookup именно по IDs; если `GetCheckout` timeout произошел до получения IDs, публичный API не дает надежного client correlation lookup. Такое состояние должно быть `unknown/manual_review`, а не automatic retry.
3. **Один канонический domain API реалистичен, один wire connector — нет.** ROSSKO использует SOAP и свой checkout/result, ARMTEK — REST/Basic Auth и line allocation result. Каноническая модель обязана представлять `supplier_order[]`, line allocations и partial outcome, а каждый поставщик требует отдельного adapter.
4. **Узкие HTTP-команды 1С разумно поставлять расширением**, чтобы не снимать типовую конфигурацию с поддержки. Локальная mutation + operation record может быть явной транзакцией 1С; atomicity HTTP handler по умолчанию публичными источниками не подтверждена, поэтому ее нельзя предполагать.

---

# A. Verified contract facts

Ниже только то, что прямо подтверждено прочитанными публичными контрактами.

## Exolve

### F1. Control-plane API авторизуется Bearer key, но это не доказывает auth входящего callback

- claim: `SetCallForwarding` — POST `https://api.exolve.ru/number/v1/SetCallForwarding`; клиентский вызов этого management API требует `Authorization: Bearer <API-key приложения>`.
- source URL: https://docs.exolve.ru/docs/ru/api-reference/numbering-api/call-forwarding/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live docs verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: verified-contract-fact

### F2. Dynamic routing — синхронный JSON-RPC request/response contract

- claim: При входящем звонке Exolve отправляет на заданный URL POST/JSON-RPC method `getControlCallFollowMe` с `sip_id`, `numberA`, `call_sid`; сервер клиента возвращает `redirect_type`, массив targets (`redirect_number` или `sip_uri`) и target answer timeout. Ответ также может задать `event_URL` и `client_id`.
- source URL: https://docs.exolve.ru/docs/ru/instructions/call-forwarding-to-url/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live docs verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: verified-contract-fact

### F3. Резерв задается при конфигурации, а не в каждом routing response

- claim: Для dynamic forwarding `SetCallForwarding` принимает `call_forwarding_ipcr.url` и `call_forwarding_ipcr.reserve`; публичная схема описывает `reserve` как резервный номер, используемый если URL недоступен. Management response — пустой JSON с 200 OK.
- source URL: https://docs.exolve.ru/docs/ru/api-reference/numbering-api/call-forwarding/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live docs verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: verified-contract-fact

### F4. Exolve events позволяют доказать ответ human target и завершение отдельных legs

- claim: Inbound call events включают общий для вызова `call_id`, `call_sid`, `date_time`, target `to`, `redirect_type`, optional `client_id` и признак `reserve`. Type `s` документирован как фактический ответ B-party; `h` — окончание вызова; `d` — окончание с cause по каждому номеру при sequential/parallel forwarding. `client_id` возвращается, если ранее был задан клиентом.
- source URL: https://docs.exolve.ru/docs/ru/api-reference/statistics-api/call-events/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live docs verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: verified-contract-fact

### F5. Callback security/reliability contract публично неполон

- claim: На прочитанных страницах dynamic forwarding и inbound call events не документированы: response deadline для `getControlCallFollowMe`, retries/backoff, требуемый 2xx/response body для event receipt, ordering/at-least-once guarantee, request signature, mTLS, allowlist или иной механизм аутентификации входящих POST. Поиск `signature` и `повтор` на event page совпадений не дал. Это bounded absence только для прочитанных public pages.
- source URL: https://docs.exolve.ru/docs/ru/instructions/call-forwarding-to-url/ ; https://docs.exolve.ru/docs/ru/api-reference/statistics-api/call-events/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live docs verified 2026-09-11
- accessed: 2026-09-11
- confidence: medium-high
- class: verified-public-doc-gap

### F6. Existing-call transfer/bridge остается неподтвержденным

- claim: Прочитанные Exolve contracts описывают выбор target для входящего forwarding до соединения с B-party и lifecycle переадресованного звонка. Они не описывают API `transfer`, `bridge`, SIP REFER/Replaces, attended transfer или сохранение уже соединенного bot leg. Поэтому mid-call human handoff нельзя считать capability Exolve на основании этих страниц.
- source URL: https://docs.exolve.ru/docs/ru/instructions/call-forwarding-to-url/ ; https://docs.exolve.ru/docs/ru/api-reference/statistics-api/call-events/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live docs verified 2026-09-11
- accessed: 2026-09-11
- confidence: high as a bounded documentation conclusion
- class: verified-public-doc-gap

## ROSSKO

### F7. Checkout может вернуть несколько заказов и частичный результат по строкам

- claim: Public ROSSKO v2.1 `/GetCheckout` — side-effecting SOAP order creation. Ответ содержит `OrderIDS`, список успешно заказанных строк и `ItemsErrorList`; значит один request нельзя моделировать как единственный `order_id + success`.
- source URL: https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout
- publisher: ROSSKO
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: verified-contract-fact

### F8. После получения ROSSKO IDs есть точный status lookup

- claim: `/GetOrders` получает изменения заказа по одному ID, списку IDs, периоду, последним N либо типу; документация настоятельно рекомендует получать данные именно по идентификаторам. Ответ содержит payment status и line status, включая `комплектуется`, `отгружено`, `нет в наличии`, `отменён клиентом`, `просрочен` и возвратные состояния.
- source URL: https://api.rossko.ru/index.php?action=SOAP&path=/GetOrders
- publisher: ROSSKO
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: verified-contract-fact

### F9. Timeout до получения IDs остается плохо коррелируемым

- claim: Public `/GetOrders` не документирует lookup по client request/order ID, телефону, комментарию или payload fingerprint; точный lookup использует vendor IDs. Public `/GetCheckout` request schema не показывает idempotency/client-correlation field. Поиск public ROSSKO docs в этом раунде не обнаружил отдельный cancel или idempotency endpoint; статус `8` лишь показывает, что отмена клиентом существует как business state.
- source URL: https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout ; https://api.rossko.ru/index.php?action=SOAP&path=/GetOrders ; https://api.rossko.ru/
- publisher: ROSSKO
- pub_date: home page has update 2026-05-15; method pages not dated
- accessed: 2026-09-11
- confidence: medium-high; bounded to public v2.1 pages/menu
- class: verified-public-contract-gap

### F10. Public ROSSKO API 2.1 теперь рекомендует HTTPS; wire migration нельзя откладывать

- claim: ROSSKO public API home page сообщает, что HTTPS включен 2026-05-15, планируется отказ от HTTP и рекомендуется заменить protocol во всех service URLs. Актуальная public версия обозначена как 2.1.
- source URL: https://api.rossko.ru/
- publisher: ROSSKO
- pub_date: 2026-05-15 update notice
- accessed: 2026-09-11
- confidence: high
- class: verified-version/compatibility-fact

## Второй supplier contract: ARMTEK

### F11. ARMTEK — другой transport/auth/schema family

- claim: Official ARMTEK Web Services v1.1.7 позиционируются как REST, используют Basic Authentication и могут возвращать JSON/XML. Order API использует POST `/api/ws_order/createOrder`; public docs также перечисляют test order, getOrder и editOrder.
- source URL: https://ws.armtek.ru/ ; https://ws.armtek.ru/?alias=order&page=service
- publisher: ARMTEK
- pub_date: not stated; version 1.1.7 live docs verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: verified-contract-fact

### F12. ARMTEK order result также line-oriented и может распределить строку по нескольким заказам

- claim: `createOrder` input использует `PIN`, `BRAND`, quantity `KWMENG`, recommended warehouse `KEYZAK`, optional maximum price/date и full-delivery flag. Per-line response имеет `REMAIN`, `ERROR`, `ERROR_MESSAGE` и `RESULT[]`; allocation result включает allocated quantity, price/date, created order number `VBELN` и block status. Следовательно, одна requested line может дать partial/unordered remainder и несколько supplier allocations/orders.
- source URL: https://ws.armtek.ru/?alias=order&page=service
- publisher: ARMTEK
- pub_date: not stated; version 1.1.7 live docs verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: verified-contract-fact

### F13. ARMTEK status model заметно богаче общего order status

- claim: `getOrder` получает заказ по `ORDER`; header status включает `Создан/В работе/Закрыт/Отклонен`, а строки отдельно отражают ordered, expected, in-transit, delivered, rejected quantities и partial/full supplied/rejected status. Header response документирует `DOC_NUM` как номер заказа в учетной системе клиента, но в прочитанной секции public `createOrder` он не был входным параметром.
- source URL: https://ws.armtek.ru/?alias=order&page=service
- publisher: ARMTEK
- pub_date: not stated; version 1.1.7 live docs verified 2026-09-11
- accessed: 2026-09-11
- confidence: high for status fields; medium for correlation gap, bounded to the read createOrder section
- class: verified-contract-fact + bounded-gap

## 1С

### F14. Расширение сохраняет типовую конфигурацию на поддержке, но имеет compatibility check

- claim: Официальная 1С strategy — выполнять клиентские доработки в расширении, не меняя типовую конфигурацию; базовая конфигурация остается на полной поддержке и обновляется поставщиком. При подключении платформа проверяет, что заимствованные расширением объекты и свойства не изменились; возможность применения расширения можно проверить заранее.
- source URL: https://v8.1c.ru/platforma/rasshireniya/
- publisher: фирма «1С»
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: verified-platform/supportability-fact

### F15. 1С поддерживает локальные all-or-nothing transactions, но HTTP handler default boundary не установлен

- claim: 1С Developer Guide описывает транзакцию как all-or-nothing sequence и предоставляет explicit `BeginTransaction()`, `CommitTransaction()`, `RollbackTransaction()`; modification event handlers могут выполняться в implicit transaction. Прочитанный источник не утверждает, что произвольный HTTP-service handler целиком автоматически обернут в одну transaction.
- source URL: https://kb.1ci.com/1C_Enterprise_Platform/Guides/Developer_Guides/1C_Enterprise_8.3.23_Developer_Guide/Chapter_9._Data_operations/9.2._Transactions/9.2.1._General_information/
- publisher: 1Ci / 1C:Enterprise Platform Developer Guide
- pub_date: guide version 8.3.23; live page indexed 2026-09-11
- accessed: 2026-09-11
- confidence: high for transaction semantics; high for bounded non-claim about HTTP handler
- class: verified-platform-fact + boundary-gap

---

# B. Proposed Staffy patterns

Эти пункты — design decisions, выведенные из контрактов выше; они не являются vendor guarantees.

## P1. Exolve routing endpoint: ultra-thin, allow-fail, precomputed

- proposed pattern: `getControlCallFollowMe` endpoint не обращается синхронно к LLM, 1С или supplier API. Он читает precomputed tenant route/presence, возвращает JSON-RPC route и `client_id=staffy_call_id`. Carrier `reserve` всегда настроен заранее.
- rationale from facts: deadline и retries не опубликованы; любая downstream dependency увеличивает вероятность недоступности route URL; Exolve уже имеет reserve fallback.
- acceptance test: kill Staffy routing endpoint и доказать, что звонок уходит на reserve; измерить фактический provider deadline серией controlled delays до 60 s и получить письменное подтверждение Exolve до production.

## P2. Event ingestion assumes duplicates and reordering until vendor says otherwise

- proposed pattern: Receiver быстро сохраняет raw event и отвечает; unique key как минимум `(provider, application_uuid, call_id, type, target/to, date_time|event fingerprint)`. Projection is monotonic: terminal call state не откатывается поздним `ring/open`; `s` подтверждает human answer, `d/h/e` закрывают leg/call согласно tested event matrix.
- rationale from facts: stable IDs и event types доступны, но delivery retry/order guarantees не опубликованы.
- acceptance test: replay event corpus in every permutation and with duplicates; итоговая state projection одинакова.

## P3. Exolve callbacks must be isolated as an untrusted ingress

- proposed pattern: До письменного подтверждения signature/mTLS использовать отдельный ingress URL с high-entropy tenant token, IP allowlist если Exolve предоставит ranges, rate limits, strict schema/size limits и сверку `application_uuid/called number` с tenant configuration. Bearer API key хранится только для outbound management calls и не считается callback verifier.
- rationale from facts: Bearer auth документирован для Staffy→Exolve API, но не для Exolve→Staffy callbacks.
- acceptance gate: production запрещен без согласованного способа source authentication либо formally accepted compensating risk.

## P4. Mid-call handoff capability is a hard procurement gate

- proposed pattern: Разделить два сценария: (a) carrier pre-answer routing/fallback — подтвержден Exolve; (b) перевод уже принятого AI runtime звонка — требует runtime/PBX capability `dial second leg → observe answered → bridge/transfer → release bot`. Vendor demo должен доказать no-answer/decline/timeout и сохранение caller leg.
- rationale from facts: Exolve forwarding docs не подтверждают existing-call bridge/attended transfer.
- acceptance test: five cases — human answers, no answer, busy, runtime dies during dial, duplicate transfer command; ни в одном caller не теряется и bridge commits максимум один раз.

## P5. Supplier canonical model is a domain envelope, not lowest-common-denominator DTO

- proposed canonical command:
  - `SubmitOrder(intent_id, supplier, customer_account, fulfillment, lines[])`
  - each line: `requested_part`, `brand`, `supplier_offer_id/warehouse`, `qty`, price/date ceilings
  - result: `supplier_orders[]`, `line_outcomes[]` with `requested/accepted/remainder`, allocations, vendor line/order IDs, state, error code/message
- rationale from facts: ROSSKO returns `OrderIDS` plus successful/error lists; ARMTEK returns per-line remainder and allocation results that may carry multiple `VBELN` orders.
- consequence: shared orchestration/policy is realistic; SOAP/REST auth, request shapes, status maps and cancellation remain vendor-specific adapters.

## P6. ROSSKO unknown-after-submit is not automatically retryable

- proposed pattern: If `/GetCheckout` response with `OrderIDS` is received, persist IDs before any next step and reconcile only by them via `/GetOrders`. If transport result is unknown before IDs are received, stop at `UNKNOWN_EXTERNAL_RESULT`; query recent/time-window orders only as evidence, not proof, and route to operator/ROSSKO manager unless a private contract supplies an exact client correlator or idempotency rule.
- rationale from facts: ROSSKO itself recommends ID lookup; public checkout/order contracts show no exact client correlation or idempotency key.
- consequence: Do not promise automatic recovery from this ambiguity in pilot SLA.

## P7. Partial success is a first-class state, never a boolean error

- proposed pattern: Each line/allocation has its own terminal/intermediate state. Confirmation snapshot records whether split orders and partial fill are permitted. If not permitted, adapter must request full-delivery semantics where supported or stop for approval; compensation/manual cancellation is applied per vendor order/line, not to an abstract whole.
- rationale from facts: both ROSSKO and ARMTEK explicitly expose partial/multiple-order outcomes, but with different schemas and flags.

## P8. 1С command facade should be an extension with explicit local transaction

- proposed pattern: Deliver `StaffyIntegration` as a versioned extension, not direct edits to the typical configuration. Each write command explicitly opens one 1С transaction, checks unique `(tenant, operation_id)`, validates current business state, mutates the document/register and writes the operation result, then commits; on exception, rolls back and rethrows. Do not call Exolve/supplier/network while holding the 1С transaction.
- rationale from facts: extension preserves supplier support and is compatibility-checkable; 1С provides explicit transactions; automatic whole-handler transaction was not established.
- supportability gate: certify extension against exact platform + configuration releases; run compatibility check and regression suite before every 1С update. Automatic merge is not treated as automatic business compatibility.

## P9. Keep cross-system saga outside 1С, local invariants inside 1С

- proposed pattern: Staffy orchestrator owns long-running steps and compensation; 1С endpoint owns only one short atomic local command. Example: `Prepare/lock local draft (1С tx) → supplier submit (outside tx) → attach supplier IDs/commit local order (1С tx)`. Unknown supplier result never keeps a 1С lock open.
- rationale from facts: 1С transaction protects one infobase; supplier/telephony APIs are independent remote systems with incomplete idempotency contracts.

---

# C. Contract comparison for pilot connector scope

| Dimension | ROSSKO public v2.1 | ARMTEK public v1.1.7 | Canonicalization decision |
|---|---|---|---|
| Transport | SOAP | REST-style HTTP, JSON/XML | separate wire adapters |
| Auth | KEY1 + KEY2 request fields | Basic Authentication | secret-per-tenant adapter config; never model-visible |
| Offer/order key | partnumber + brand + stock | PIN + BRAND + KEYZAK/ARTID | canonical `supplier_offer_ref`, preserve raw components |
| Create result | `OrderIDS`, successful items, `ItemsErrorList` | per-line `REMAIN`, errors, `RESULT[]` allocations with `VBELN` | result must allow many orders and partial line outcomes |
| Lookup | vendor ID/list, period/latest N/type; ID recommended | getOrder by vendor `ORDER` | exact reconciliation requires stored vendor IDs |
| Status | payment + numeric line status | header + quantitative line lifecycle/partial states | canonical coarse state + raw vendor state/details |
| Client correlation | not documented on read public create/lookup schemas | `DOC_NUM` visible in get response, not in read create inputs | do not assume; negotiate/test per tenant contract |
| Cancel/edit | no public cancel method found; canceled status exists | editOrder listed; cancel semantics not read/verified | capability flag per adapter; default manual review |
| Test mode | test checkout orders removed after 24h | explicit `createTestOrder` | adapter-specific test harness |

**Result:** A single generic implementation that maps arbitrary supplier fields at runtime is not credible for first pilots. A stable Staffy domain interface plus one tested adapter per contracted supplier is credible.

---

# D. Pilot blockers and exact questions to vendors

## Exolve — must be answered in writing or demonstrated

1. Maximum response time for `getControlCallFollowMe`; behavior for timeout, malformed JSON-RPC, non-2xx and connection reset.
2. Exact reserve activation conditions and whether reserve can be SIP, not only PSTN number, under the contracted product.
3. Event delivery success criterion, retry count/schedule, duplicate semantics, ordering guarantees and retention/replay mechanism.
4. Callback source authentication: signature algorithm/headers, mTLS or published fixed IP ranges; key rotation.
5. Meaning/ordering matrix for `b/o/s/h/d/e`, especially sequential/parallel targets and reserve.
6. Whether any product/API supports transfer of an already connected call, second-leg dialing, attended transfer or bridge; identifiers and failure semantics.

## ROSSKO — must be answered in writing or sandbox-tested

1. Is there a private/client order ID or idempotency mechanism for GetCheckout? Scope, TTL and duplicate response semantics.
2. How to resolve checkout timeout before `OrderIDS` returns without ambiguous content/time matching?
3. API method and allowed state window for canceling whole order/line; partial cancellation and confirmation semantics.
4. Delay/consistency SLA from GetCheckout success to GetOrders visibility; rate limits for order endpoints.
5. Exact meaning of partial success and whether successfully created orders persist when other lines fail.
6. HTTPS cutoff date and current supported WSDL/version under the customer's agreement.

## ARMTEK — must be answered before adopting as second pilot adapter

1. Can `DOC_NUM` be supplied during createOrder in the contracted version, and is it unique/searchable?
2. editOrder cancellation/quantity rules, state windows and idempotency semantics.
3. Whether HTTPS is mandatory/supported for every endpoint and how Basic credentials/IP allowlisting are managed.
4. Meaning of `NUM_ZAK`, allocation splitting and atomicity when some lines/allocations fail.

## 1С client — must be proven on exact installation

1. Platform and typical-configuration release; extension compatibility mode and vendor support policy.
2. Permission to publish a custom HTTP service from an extension, reverse-proxy/TLS/auth scheme and service account rights.
3. Fault test: mutation and operation record rollback together on handler exception.
4. Concurrency test: two identical `operation_id` requests and two different operations competing for the same stock/document.
5. Update test: extension applicability and regression suite after the next vendor configuration update.

---

# E. Not found / unresolved after round 2

- Public Exolve callback deadline, retries/backoff, delivery acknowledgement contract, ordering guarantee and cryptographic request authentication.
- Public Exolve attended/warm transfer or existing-call bridge semantics.
- Public ROSSKO exact lookup without vendor order ID, explicit idempotency/client request key and cancel endpoint.
- Verified ARMTEK edit/cancel contract and whether `DOC_NUM` is accepted on create in the customer's version.
- Official statement that a custom 1С HTTP handler is automatically one transaction; design therefore requires explicit transaction code and a fault test.
- Current exact compatibility of the proposed 1С extension with any pilot configuration; this is installation-specific and cannot be established from platform-level docs.
- A second additional supplier beyond ARMTEK was not included: one comparison was enough to falsify the idea of a shared wire contract within the source/call budget.

---

# F. Read source register (8)

1. Exolve `SetCallForwarding`: https://docs.exolve.ru/docs/ru/api-reference/numbering-api/call-forwarding/
2. Exolve dynamic forwarding to URL: https://docs.exolve.ru/docs/ru/instructions/call-forwarding-to-url/
3. Exolve inbound call events: https://docs.exolve.ru/docs/ru/api-reference/statistics-api/call-events/
4. ROSSKO `/GetCheckout`: https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout
5. ROSSKO `/GetOrders` and current public API index: https://api.rossko.ru/index.php?action=SOAP&path=/GetOrders ; https://api.rossko.ru/
6. ARMTEK Web Services/order services v1.1.7: https://ws.armtek.ru/ ; https://ws.armtek.ru/?alias=order&page=service
7. 1С configuration extensions: https://v8.1c.ru/platforma/rasshireniya/
8. 1C:Enterprise 8.3.23 Developer Guide — transactions: https://kb.1ci.com/1C_Enterprise_Platform/Guides/Developer_Guides/1C_Enterprise_8.3.23_Developer_Guide/Chapter_9._Data_operations/9.2._Transactions/9.2.1._General_information/
