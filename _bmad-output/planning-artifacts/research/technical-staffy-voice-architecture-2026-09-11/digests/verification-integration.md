# Verification — integration & reliability

Дата проверки: 2026-09-11  
Проверяемый артефакт: `integration-reliability-r1-1.md`  
Метод: fresh-context spot check; прочитан только указанный digest и 7 текущих официальных источников. Vendor capability отделена от Staffy design inference.

## Итог

Несущая архитектурная линия сохраняется: готовая carrier routing/fallback, узкие 1С-команды, operation ledger, явное подтверждение и saga только для нескольких независимых commits — обоснованы. **Overturned claims нет.** Однако нужны четыре оговорки:

1. Exolve подтверждает переадресацию входящего номера, но не attended transfer/bridge уже принятого ботом звонка.
2. 1С подтверждает программируемый HTTP boundary, но цитируемая страница не гарантирует атомарность handler + запись `external_operation_id`.
3. Отсутствие idempotency/correlation подтверждено только для публичной страницы ROSSKO `/GetCheckout`, не для всех договорных или tenant-specific интерфейсов.
4. IETF `Idempotency-Key` — истекший и неактивный Internet-Draft, не RFC; AWS PDF имеет copyright 2026, но это не дата публикации saga guidance.

## Матрица проверки

| Область / claims | Outcome | Что подтверждено источником | Что остается Staffy inference или требует проверки |
|---|---|---|---|
| **A. Exolve routing, multi-resource dial, carrier fallback (C1–C3)** | **verified** | Exolve разрешает статическую или динамическую по API переадресацию на телефон, Exolve SIP ID, static-IP SIP trunk и внешний SIP account. Для нескольких ресурсов документированы последовательная и параллельная стратегии, максимум 10 ресурсов; после ответа одного дозвон остальных прекращается. Для dynamic routing можно заранее задать резервные номера/SIP на случай, если сервер клиента не отвечает. | Термин `hunt group` — архитектурное описание Staffy, а не название продукта на странице. Выбор carrier-side reserve как fail-safe — обоснованная рекомендация, не гарантия SLA/event ordering. |
| **A. Existing-call human handoff semantics (C4)** | **unverified** | Страница подтверждает single-winner поведение переадресации и прекращение одиночного дозвона при no-answer. | Она описывает прием входящего вызова через forwarding, а не warm/attended transfer, bridge commit, call-leg IDs или сохранение bot leg. Состояния `dialing_human → human_connected → bridge_committed` — нужный Staffy contract, но не доказанная возможность Exolve на этой странице. |
| **A. Audio notice/context ordering (C5)** | **verified** | Exolve позволяет перед соединением проиграть сообщение звонящему, принимающему или обоим. | Атомарная запись handoff packet до dialing и screen-pop — Staffy ordering pattern; источник не подтверждает screen-pop или транзакционную запись контекста. |
| **B. 1С OData surface (C6)** | **verified** | Официальный автоматически публикуемый REST/OData 3.0 интерфейс дает metadata discovery, create/read/update/delete для основных объектов и, среди прочего, проведение/отмену проведения документа. Обычные проверки прав и event handlers выполняются, с указанным на странице исключением проверки заполнения. | Формулировку «для чтения и ограниченного CRUD» следует понимать как выбранную Staffy policy, а не ограничение OData: фактический platform surface широкий. Не давать его модели напрямую — security/least-privilege inference. |
| **B. Custom 1С HTTP command boundary (C7)** | **verified** | 1С позволяет создавать произвольные HTTP services: управлять URL templates и HTTP methods, читать request URL/headers/body, формировать response code/headers/body; аутентификация и data separation работают аналогично web services. Это достаточно, чтобы реализовать узкие business-command endpoints. | Что такой boundary «безопаснее» generic OData — сильная архитектурная рекомендация, а не vendor guarantee; безопасность зависит от handler, прав и публикации. |
| **B. One local transaction inside 1С (часть C7)** | **unverified** | Цитируемая HTTP-services page подтверждает выполнение handler на встроенном языке. | Она не документирует transaction boundaries, rollback behavior или атомарность business mutation вместе с записью `external_operation_id`. Формулировать как требование к реализации и проверять на конкретной конфигурации/версии 1С. |
| **C. ROSSKO `/GetCheckout` side effect (C8–C9)** | **verified** | Страница прямо называет endpoint «Сервис для оформления заказа»; пример считает успехом «Заказ оформлен», а ответ содержит `OrderIDS`, item `order_id`, успешные строки и `ItemsErrorList`. Test orders удаляются через 24 часа и видны через `GetOrders`. Это side-effecting checkout boundary. | Prepare/confirm/execute и immutable draft — Staffy safety design, не функция ROSSKO. |
| **C. No documented idempotency/client correlation on cited page (C11–C12)** | **verified** | Полный опубликованный request schema перечисляет `KEY1`, `KEY2`, delivery/payment/contact, `delivery_parts` и `PARTS`; отдельного idempotency key, request ID или client-order/correlation ID нет. Поиск страницы по `idempot` и `correlation` совпадений не дал. | Это bounded absence: нельзя делать вывод, что механизм отсутствует во всех ROSSKO contracts. `contact.comment` и item `comment` документированы как комментарии, не как уникальные correlation fields; полагаться на них для дедупликации без договора нельзя. `unknown → reconcile → retry/manual_review` — Staffy reliability policy. |
| **D. IETF idempotency semantics (C11–C13)** | **verified** | Текст `-07` описывает unique key, запрет reuse с другим payload, optional fingerprint, возврат результата завершенного duplicate и conflict для concurrent duplicate; также прямо описывает неопределенность после timeout неидемпотентного POST. | Operation Ledger schema, payload hash, exact retry classifier и reconciliation workflow — Staffy implementation choices. |
| **D. Current IETF status** | **disputed** | Datatracker сейчас помечает `draft-ietf-httpapi-idempotency-key-header-07` как **Expired Internet-Draft**, `Expired & archived`, `no longer active`; последняя ревизия — 2025-10-15, истекла 2026-04-18, версии новее `-07` нет. | Его можно использовать только как явно non-normative design pattern. Нельзя называть `Idempotency-Key` действующим IETF standard/RFC или предполагать поддержку header внешним API. Caveat в digest по существу верен; статус нужно поднимать выше, рядом с первым использованием. |
| **D. OWASP transaction authorization (C9–C10)** | **verified** | OWASP требует показывать significant transaction data (WYSIWYS), server-side enforcement, invalidation/reset authorization при изменении данных, final control gate непосредственно перед execution, ограниченный срок и уникальные credentials для каждой операции. | Конкретная подпись `tenant_id + actor/call_id + action + payload_hash + expiry + nonce` — разумный Staffy design, но не предписанный OWASP формат. |
| **D. AWS saga/retry guidance (C13–C16)** | **verified** | AWS определяет saga как последовательность local transactions с compensating transactions; рекомендует orchestration для distributed transactions across stores, когда 2PC недоступен/сложен. Документ прямо отмечает idempotent participants, lack of isolation и semantic locking, observability, compensations и orchestrator SPOF. Retry-with-backoff предназначен для transient throttling/network/unavailability; non-transient failures должны fail fast, а повторяемые операции должны быть idempotent. | Применение к `reserve in 1C → supplier checkout → commit`, перечень Staffy states, append-only event schema и предпочтение централизованной orchestration — обоснованные проектные решения, не AWS requirement. |
| **D. AWS publication metadata** | **disputed** | Текущий PDF содержит `Copyright © 2026`. Его document history датирует добавление saga patterns 2023-11-16 и более поздние перечисленные изменения — 2024 годом. | Не использовать copyright 2026 как доказательство «версии/публикации 2026». Лучше писать «current AWS Prescriptive Guidance PDF, accessed 2026-09-11». |

## Рекомендуемые точечные правки формулировок

- `hunt group` → «последовательная/параллельная переадресация на несколько ресурсов (hunt-group-like primitive)».
- «handoff можно строить» → «подходит как carrier-side fallback/target; transfer/bridge semantics выбранного runtime и Exolve требуют отдельного подтверждения».
- «external operation ID в одной локальной транзакции 1С» → «должен записываться атомарно, если transaction model конкретного handler/configuration это подтверждает».
- «ROSSKO не поддерживает idempotency/correlation» → «на публичной странице `/GetCheckout` такой параметр не документирован».
- IETF source → «expired Internet-Draft used as a design pattern, not a current standard».
- AWS date → «current PDF; copyright 2026; saga guidance added 2023», без вывода о версии из copyright.

## Источники (7)

1. [МТС Exolve — Как принимать входящие вызовы через переадресацию](https://exolve.ru/faq/kak-prinimat-vkhodyashchie-vyzovy-cherez-pereadresatsiyu/)
2. [1С — REST интерфейс](https://v8.1c.ru/platforma/rest-interfeys/)
3. [1С — HTTP-сервисы](https://v8.1c.ru/platforma/http-servisy/)
4. [ROSSKO API — GetCheckout](https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout)
5. [IETF Datatracker — The Idempotency-Key HTTP Header Field (`-07`, status and document formats)](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/)
6. [OWASP — Transaction Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Transaction_Authorization_Cheat_Sheet.html)
7. [AWS Prescriptive Guidance — Cloud design patterns, architectures, and implementations](https://docs.aws.amazon.com/pdfs/prescriptive-guidance/latest/cloud-design-patterns/cloud-design-patterns.pdf)

## Verification disposition

- **verified:** core Exolve routing/multi-resource/fallback capabilities; 1С OData and custom HTTP capabilities; ROSSKO checkout side effect and bounded public-schema absence; OWASP authorization controls; IETF historical semantics; AWS saga/retry guidance.
- **unverified:** attended-transfer/bridge behavior and bot-leg preservation; atomic 1С handler + operation-ID transaction.
- **disputed:** any treatment of the expired IETF draft as a current standard; AWS copyright year as a publication/version date.
- **overturned:** none.
