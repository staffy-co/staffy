# Integration & reliability digest — round 1

Дата исследования: 2026-09-11  
Направление: Staffy voice architecture для первых оплачиваемых пилотов в России  
Фокус: SIP/телефония и human handoff; 1С/ERP и каталоги запчастей; безопасный tool execution; идемпотентность, retries, compensation и audit trail.  
Ограничения раунда: 7 реально прочитанных первичных/официальных источников; без проектных файлов и ambient context.

## Короткий архитектурный вывод

Для первых пилотов Staffy следует купить/использовать как сервис российский номер, PSTN/SIP, маршрутизацию, hunt-group/резервную переадресацию и статистику звонков. Не следует строить собственный SBC, медиасервер или АТС. Собственная дифференцирующая часть — тонкий control plane между voice runtime и системами клиента:

1. `Call/Handoff Coordinator`: долговечный автомат состояний звонка и его legs, дедупликация событий, тайм-ауты, carrier-side fallback.
2. `Action Gateway`: разделение read-tools и write-tools, tenant policy, авторизация и финальный confirmation gate.
3. `Operation Ledger`: `operation_id`, idempotency key, hash нормализованного payload, состояние `prepared/approved/executing/succeeded/failed/unknown/compensating/compensated/manual_review`, сохраненный ответ.
4. `Connector adapters`: узкие адаптеры к 1С OData/HTTP и API поставщиков, с канонической моделью Staffy и без выдачи LLM прямых credentials.
5. `Audit/Event journal`: append-only история intent → approval → provider request/response → retries → reconciliation/compensation → human override.

Критический принцип: модель предлагает типизированное намерение, но не вызывает 1С или поставщика напрямую. Все опасные эффекты исполняет детерминированный control plane после policy check, подтверждения и регистрации операции.

## Claims

### C1. Телефонию и базовую отказоустойчивую маршрутизацию целесообразно покупать готовыми

- claim: МТС Exolve уже предоставляет переадресацию входящего номера на обычные номера, SIP ID, SIP trunk и внешний SIP-аккаунт; маршрут можно задавать статически либо динамически через API. Поэтому для пилота Staffy не нужен собственный PSTN/SIP carrier core.
- source URL: https://exolve.ru/faq/kak-prinimat-vkhodyashchie-vyzovy-cherez-pereadresatsiyu/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: vendor-capability + architecture-inference

### C2. Handoff можно строить на carrier-side hunt strategy, а не на одном номере оператора

- claim: Exolve документирует последовательный и параллельный дозвон максимум до 10 номеров/SIP-ресурсов; дозвон прекращается после ответа одного абонента. Это пригодно как готовый human-handoff primitive: очередь/группа операторов должна быть целью transfer, а не единственный сотрудник.
- source URL: https://exolve.ru/faq/kak-prinimat-vkhodyashchie-vyzovy-cherez-pereadresatsiyu/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: vendor-capability + recommendation

### C3. Резервный маршрут должен жить у оператора связи

- claim: При динамической переадресации Exolve запрашивает маршрут у сервера клиента и позволяет настроить резервные номера/SIP-соединения на случай, если этот сервер не отвечает. Для предотвращения потери звонка Staffy должен иметь carrier-side fallback, который не зависит от доступности AI/control-plane.
- source URL: https://exolve.ru/faq/kak-prinimat-vkhodyashchie-vyzovy-cherez-pereadresatsiyu/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: vendor-capability + reliability-recommendation

### C4. Для согласованного handoff нужен собственный state machine поверх готовой телефонии

- claim: Exolve сообщает, что одиночный маршрут прекращает дозвон, если абонент не ответил, тогда как multi-resource route имеет single-winner behavior. Следовательно, Staffy должен считать handoff успешным только после наблюдаемого ответа/соединения human leg; до этого bot leg остается активным либо вызывается carrier fallback. Минимальные состояния: `requested → dialing_human → human_connected → bridge_committed`, с ветками `timeout/declined/failed → bot_resume|fallback_queue`.
- source URL: https://exolve.ru/faq/kak-prinimat-vkhodyashchie-vyzovy-cherez-pereadresatsiyu/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: medium-high
- class: source-bounded-inference + reliability-pattern

### C5. Контекст для оператора надо фиксировать до начала перевода

- claim: Exolve позволяет проиграть отдельное аудиосообщение принимающему вызов сотруднику и/или клиенту. Staffy должен до dialing human leg атомарно сохранить handoff packet (`call_id`, кто клиент, цель звонка, выявленные детали авто/запчасти, обещания, незавершенные действия), а затем дать оператору краткий whisper/screen-pop. Это предотвращает ситуацию, когда аудио переведено, а контекст еще не записан.
- source URL: https://exolve.ru/faq/kak-prinimat-vkhodyashchie-vyzovy-cherez-pereadresatsiyu/
- publisher: МТС Exolve / АО «МТТ»
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: medium-high
- class: vendor-capability + ordering-recommendation

### C6. Стандартный интерфейс 1С подходит для чтения и ограниченного CRUD, но его нельзя давать модели напрямую

- claim: Официальный REST-интерфейс 1С публикует объекты приложения через OData, поддерживает metadata discovery, CRUD, а для документов — проведение и отмену проведения. Такой широкий интерфейс следует закрыть адаптером и tenant-scoped service account; LLM получает только узкие tools вроде `find_customer`, `check_stock`, `prepare_order`, но не произвольный OData URL или метод проведения документа.
- source URL: https://v8.1c.ru/platforma/rest-interfeys/
- publisher: фирма «1С»
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: platform-capability + security-recommendation

### C7. Опасные команды в 1С лучше оформить как собственные бизнес-HTTP endpoints

- claim: 1С официально поддерживает собственные HTTP-сервисы, где разработчик контролирует URL/method, request body, response и аутентификацию. Для пилота это более безопасная граница для команд `ReserveParts`, `CreateDraftOrder`, `ConfirmOrder`, `CancelReservation`, чем цепочка универсальных OData CRUD-вызовов: проверка бизнес-инвариантов и запись external operation ID могут выполняться внутри одной локальной транзакции 1С.
- source URL: https://v8.1c.ru/platforma/http-servisy/
- publisher: фирма «1С»
- pub_date: not stated; live official page verified 2026-09-11
- accessed: 2026-09-11
- confidence: high for capability; medium-high for recommended boundary
- class: platform-capability + architecture-recommendation

### C8. Каталог/поставщик должен подключаться через отдельный anti-corruption adapter

- claim: Официальный ROSSKO GetCheckout — SOAP endpoint версии 2.1 с парой ключей `KEY1/KEY2`, параметрами доставки, оплаты, контакта и позиций; ответ возвращает `OrderIDS`, успешные и ошибочные позиции. Это не совпадает с 1С OData/HTTP моделью, поэтому Staffy нужен отдельный adapter, который нормализует supplier offer/order/status в канонические DTO и хранит vendor IDs.
- source URL: https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout
- publisher: ROSSKO
- pub_date: not stated; official endpoint/page verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: vendor-contract + integration-pattern

### C9. Заказ должен иметь явную фазу prepare/confirm/execute

- claim: GetCheckout создает заказ и возвращает его ID, то есть это side-effecting boundary. Staffy должен сначала получить/зафиксировать immutable order draft (поставщик, артикул и бренд, склад, количество, цена, доставка, клиент), затем показать/произнести существенные поля для подтверждения и только после этого вызвать checkout. Изменение любого существенного поля инвалидирует прежнее подтверждение.
- source URL: https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout ; https://cheatsheetseries.owasp.org/cheatsheets/Transaction_Authorization_Cheat_Sheet.html
- publisher: ROSSKO; OWASP Foundation
- pub_date: ROSSKO not stated; OWASP not stated; live pages verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: contract-fact + transaction-authorization-pattern

### C10. Подтверждение опасного действия должно быть связано с конкретным payload и проверяться в последнем gate

- claim: OWASP рекомендует инвалидировать authorization при изменении transaction data, проверять авторизацию непосредственно перед execution, ограничивать срок ее действия и делать credentials/authorization уникальными для операции. Для Staffy confirmation token должен подписывать `tenant_id + actor/call_id + action + payload_hash + expiry + nonce`; финальный executor повторно проверяет policy и hash, а не доверяет флагу из LLM/session memory.
- source URL: https://cheatsheetseries.owasp.org/cheatsheets/Transaction_Authorization_Cheat_Sheet.html
- publisher: OWASP Foundation
- pub_date: not stated; live official cheat sheet verified 2026-09-11
- accessed: 2026-09-11
- confidence: high
- class: security-pattern + concrete-design

### C11. Идемпотентность должна обеспечиваться control plane, а не надеждой на повтор POST/SOAP

- claim: IETF HTTPAPI draft определяет idempotency key как уникальный идентификатор, по которому server распознает retry; key нельзя переиспользовать с другим payload, рекомендуется fingerprint, completed duplicate должен вернуть прежний результат, а concurrent duplicate — conflict. Staffy Operation Ledger должен реализовать эти semantics даже если внешний 1С/supplier API не поддерживает такой header: unique constraint по `(tenant, action, idempotency_key)`, payload hash и сохраненный terminal result.
- source URL: https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-07
- publisher: IETF HTTPAPI Working Group
- pub_date: 2025-10-15 (expired Internet-Draft; work in progress)
- accessed: 2026-09-11
- confidence: high for semantics; medium as a standards reference because the draft expired 2026-04-18
- class: protocol-pattern + implementation-recommendation

### C12. Неизвестный результат внешнего заказа нельзя исправлять слепым retry

- claim: IETF draft описывает классическую неопределенность после timeout неидемпотентного POST: клиент не знает, завершилась ли операция. Страница ROSSKO GetCheckout, прочитанная в этом раунде, не документирует idempotency key/client order ID. Поэтому после timeout Staffy должен поставить операцию в `unknown`, запросить GetOrders/сверить vendor order IDs и только после reconciliation решать retry или manual review; автоматический повтор checkout без сверки создает риск двойного заказа.
- source URL: https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-07 ; https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout
- publisher: IETF HTTPAPI Working Group; ROSSKO
- pub_date: 2025-10-15; ROSSKO not stated
- accessed: 2026-09-11
- confidence: high for timeout ambiguity; medium for ROSSKO limitation because only the published GetCheckout page was examined
- class: failure-mode + retry-policy

### C13. Retry policy должна зависеть от класса ошибки и состояния операции

- claim: Safe policy: retry только transport timeout, connection reset и явно transient 5xx/429 с exponential backoff + jitter + cap; не повторять validation/business decline; для `unknown-after-send` сначала reconcile. Все попытки используют один operation/idempotency key и тот же payload hash. Это следует из IETF semantics duplicate/concurrent request и требования AWS, что saga participants должны быть idempotent при повторном исполнении после crash/orchestrator failure.
- source URL: https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-07 ; https://docs.aws.amazon.com/pdfs/prescriptive-guidance/latest/cloud-design-patterns/cloud-design-patterns.pdf
- publisher: IETF HTTPAPI Working Group; Amazon Web Services
- pub_date: 2025-10-15; AWS document copyright 2026
- accessed: 2026-09-11
- confidence: high for idempotency/state rule; medium-high for exact retry classification
- class: reliability-pattern

### C14. Multi-system order flow требует orchestrated saga только там, где действительно несколько commits

- claim: AWS описывает saga orchestration как coordinator последовательности local transactions с compensating transactions при последующем сбое; одновременно указывает недостаток isolation, необходимость semantic locking, idempotent participants и observability. Для Staffy это подходит для `reserve in 1C → checkout supplier → commit customer order`, но не должно заменять одну локальную транзакцию там, где вся операция может завершиться внутри 1С.
- source URL: https://docs.aws.amazon.com/pdfs/prescriptive-guidance/latest/cloud-design-patterns/cloud-design-patterns.pdf
- publisher: Amazon Web Services, AWS Prescriptive Guidance
- pub_date: 2026 (copyright/version shown in document)
- accessed: 2026-09-11
- confidence: high
- class: distributed-transaction-pattern + scope-guardrail

### C15. Compensation — доменное действие, а не удаление истории

- claim: Для каждого шага заранее нужен `compensate` contract: снять резерв, отменить draft, отменить supplier order если API и бизнес-статус позволяют; если действие необратимо или исход неизвестен — `manual_review`, а не фиктивный rollback. Оригинальное действие, компенсация и их provider IDs сохраняются в одном trace.
- source URL: https://docs.aws.amazon.com/pdfs/prescriptive-guidance/latest/cloud-design-patterns/cloud-design-patterns.pdf
- publisher: Amazon Web Services, AWS Prescriptive Guidance
- pub_date: 2026 (copyright/version shown in document)
- accessed: 2026-09-11
- confidence: high for saga/compensation; medium-high for proposed Staffy states
- class: compensation-pattern + audit-design

### C16. Audit trail является частью correctness, а не только observability

- claim: AWS прямо требует detailed logging/tracing для saga execution и отмечает, что end-to-end monitoring труднее при choreography. Staffy следует использовать один `trace_id` для call, handoff и business operations и писать append-only события с actor, tenant, tool/version, input hash, approval evidence, provider request ID, redacted response, retry/compensation reason и terminal state. Для первых пилотов централизованная orchestration предпочтительнее choreography, потому что ее легче объяснить и расследовать.
- source URL: https://docs.aws.amazon.com/pdfs/prescriptive-guidance/latest/cloud-design-patterns/cloud-design-patterns.pdf
- publisher: Amazon Web Services, AWS Prescriptive Guidance
- pub_date: 2026 (copyright/version shown in document)
- accessed: 2026-09-11
- confidence: high for observability need; medium-high for Staffy event schema/preference
- class: operability-pattern + architecture-recommendation

## Practical prevention patterns

### Двойной заказ

- Один immutable `order_intent_id` создается до confirmation.
- Unique constraint на `(tenant_id, action_type, order_intent_id)`; payload hash запрещает reuse ключа с измененными данными.
- Outbox entry и переход operation state фиксируются в одной локальной транзакции Staffy.
- Один worker исполняет ключ; конкурентный дубль получает `in_progress/conflict`, завершенный — сохраненный result.
- После timeout supplier call → `unknown` → reconciliation через status/order lookup; без подтверждения отсутствия заказа retry запрещен.
- Supplier order IDs сохраняются сразу, partial success по строкам обрабатывается как отдельные sub-operations.

### Потеря звонка

- Номер и SIP routing остаются у российского carrier/CPaaS.
- Для dynamic routing заранее настроен carrier-side reserve target: operator hunt group/дежурный номер.
- Routing endpoint отвечает минимальным решением без обращения к LLM/ERP; тяжелая обработка асинхронна.
- Call state хранится по `provider_call_id + leg_id`, события дедуплицируются; terminal state не может быть отменен поздним событием.
- Если AI media/runtime недоступен, маршрут уходит человеку; этот fallback проверяется отдельным fault-injection тестом до пилота.

### Несогласованный human handoff

- Сначала сохраняются summary/context и `handoff_requested`, затем инициируется human leg.
- Bot leg не завершается до доказанного `human_connected`/bridge success.
- Timeout/decline ведет к следующему ресурсу, carrier fallback либо возврату к боту с честным сообщением клиенту.
- Single-winner guard по `handoff_id`: только один human leg может commit bridge; остальные прекращаются.
- Operator whisper/screen-pop использует уже зафиксированный snapshot, а не текущее изменяемое состояние разговора.

## Что дает платформа из коробки / что строит Staffy

| Слой | Купить/использовать готовым | Тонкий собственный слой Staffy |
|---|---|---|
| PSTN/SIP | российский номер, SIP ID/trunk, внешний SIP, routing, hunt group, recording/statistics по доступности договора | provider adapter, health/fallback policy, normalized call/leg events |
| Handoff | последовательный/параллельный дозвон, reserve forwarding, audio notice | durable handoff state machine, context packet, single-winner commit, audit |
| Voice/LLM runtime | поток речи, ASR/TTS и inference следует выбирать отдельным bake-off; в этом раунде не валидировались | typed intent only; никакого прямого доступа к credentials/ERP |
| 1С | официальный OData CRUD и custom HTTP services | narrow business command facade, tenant mapping, versioned DTOs, local operation ID |
| Каталоги/поставщики | официальный supplier API, например ROSSKO SOAP 2.1 | anti-corruption adapters, canonical offers/orders, rate/error mapping, reconciliation |
| Transactions | provider IDs/status APIs там, где доступны | approval gate, operation ledger, idempotency, retry classifier, saga/compensation, manual-review queue |

## Contradictions / tensions

1. Exolve дает dynamic routing, но при недоступности сервера безопасность звонка зависит от отдельно настроенного reserve route; наличие API само по себе не предотвращает потерю звонка.
2. Одиночная переадресация прекращает дозвон при no-answer, тогда как multi-resource strategy имеет более полезную single-winner семантику. Для handoff default должен быть hunt group, не персональный номер.
3. 1С OData удобно и широко, включая проведение/отмену документа; эта же широта делает его плохой прямой tool surface для LLM. Custom HTTP command boundary требует небольшой доработки конфигурации, но уменьшает blast radius.
4. ROSSKO — официальный supplier API, но прочитанный контракт SOAP 2.1, не REST/OData; единый generic connector без vendor adapter не подтвержден.
5. Saga orchestration делает workflow наблюдаемым, но AWS указывает отсутствие isolation, semantic-locking need и риск single point of failure. Orchestrator должен быть durable/restartable, а в 1С по возможности сохраняется локальная атомарность.
6. `Idempotency-Key` — полезная и свежая семантика IETF HTTPAPI, но версия -07 истекла 2026-04-18 и не является RFC. Использовать как дизайн-паттерн, не заявлять стандартизованную поддержку внешнего API.

## Leads for round 2 / pilot due diligence

1. Получить у Exolve точные developer docs по deadline динамического routing callback, retry policy, signature/auth, duplicate events, status callbacks и SLA; провести failure matrix: timeout, 5xx, slow response, split network, AI media failure.
2. Проверить, поддерживает ли выбранный voice runtime SIP transfer/bridge с подтверждением human-answer, и каковы его call-leg identifiers. В этом раунде runtime vendors не сравнивались.
3. Для конкретной конфигурации/версии 1С проверить доступность OData write/conduct, способ аутентификации, права service account, транзакционность custom HTTP handler и поддерживаемость расширения без изменения типовой конфигурации.
4. Прочитать ROSSKO GetOrders/GetCheckoutDetails и договорные условия: поиск по конкретным IDs, окно появления заказа, cancel capability, rate limits, partial success, тестовый контур, корреляционный клиентский ID.
5. Проверить 2–3 других поставщика пилотного клиента: официальный API, auth, rate limits, order idempotency/cancel/status. Canonical DTO проектировать только после сравнения реальных контрактов.
6. Зафиксировать risk matrix опасных tools: quote/read (без confirmation), reserve (возможно auto до лимита), order/cancel/change customer data (явное confirmation), payment/refund (вне scope первого пилота или step-up).
7. Определить retention/redaction и российские требования к записи разговора/персональным данным отдельным legal/compliance recon; этот раунд юридические выводы не делал.

## Not found / не подтверждено в раунде

- Не найдены и не использованы credible production postmortems именно российских voice-agent handoff/duplicate-order incidents; failed-pattern claims не экстраполировались из памяти.
- Не подтверждены callback retry/order guarantees Exolve, конкретные response deadlines, webhook signatures и event ordering.
- Не подтверждены warm/attended transfer semantics конкретного voice AI runtime; описан Staffy-side desired state machine, а не vendor guarantee.
- Не подтвержден idempotency/client correlation parameter ROSSKO GetCheckout; на прочитанной официальной странице он не документирован, но полный договорный/tenant-specific контракт может отличаться.
- Не подтверждены cancel/compensation endpoints ROSSKO и других каталогов.
- Не сравнивались ASR/TTS/LLM latency, accuracy, стоимость и data residency; performance claims отсутствуют.
- Точные publication dates страниц Exolve, 1С, ROSSKO и OWASP не указаны; актуальная доступность и содержание проверены 2026-09-11, но совместимость конкретных tenant/version должна проверяться на sandbox пилота.

## Read source register (7)

1. МТС Exolve — «Как принимать входящие вызовы через переадресацию»: https://exolve.ru/faq/kak-prinimat-vkhodyashchie-vyzovy-cherez-pereadresatsiyu/
2. 1С — «REST интерфейс»: https://v8.1c.ru/platforma/rest-interfeys/
3. 1С — «HTTP-сервисы»: https://v8.1c.ru/platforma/http-servisy/
4. ROSSKO API — `/GetCheckout`: https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout
5. IETF HTTPAPI — `draft-ietf-httpapi-idempotency-key-header-07`: https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-07
6. AWS Prescriptive Guidance — *Cloud design patterns, architectures, and implementations* (2026): https://docs.aws.amazon.com/pdfs/prescriptive-guidance/latest/cloud-design-patterns/cloud-design-patterns.pdf
7. OWASP Cheat Sheet Series — *Transaction Authorization*: https://cheatsheetseries.owasp.org/cheatsheets/Transaction_Authorization_Cheat_Sheet.html
