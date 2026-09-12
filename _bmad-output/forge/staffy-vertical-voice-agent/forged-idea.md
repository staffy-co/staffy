# Staffy — HARDENED

## Идея

- Staffy — универсальная multi-tenant омниканальная платформа управляемых AI-сотрудников, а не агент одной отрасли.
- Один AI-сотрудник — роль с единой личностью, контекстом и полномочиями в телефоне и текстовых каналах; tenant может иметь несколько ролей.
- Staffy — AI workforce layer над системами бизнеса, не замена CRM, ERP, складу, телефонии, платежам или helpdesk.

## Граница продукта

- Staffy владеет tenant/employee configuration, правами и policies, conversation/task context, domain capabilities, connectors, mutating-action receipts, audit, eval и release safety.
- Свободный сложный диалог разрешён; исполнение — только через типизированные business actions с validation, authorization, idempotency, risk-based approval и reconciliation.
- RAG отвечает за документы; изменяемые цены, наличие, заказы и права читаются из owning systems непосредственно перед действием.
- Каналы подключаются переиспользуемыми official adapters через minimal channel envelope; один разговор имеет одного активного владельца ответа.
- Поддерживаемый inbox клиента сохраняется; при его отсутствии предлагается Staffy Inbox. Chatwoot — кандидат после однодневного spike, не ядро.

## Технический курс

- Go — основной язык собственного core и realtime runtime.
- Voice/media providers и operator inbox остаются сменными; Pipecat/LiveKit используются как источники runtime-паттернов и optional adapters.
- Для двух пилотов Action Gateway — in-process Go module, Operation Ledger — одна append-only mutations table, versioning — `runtime_config_hash` + небольшой eval/rollback gate.
- Не строить заранее собственные STT/TTS, полный SIP/WebRTC/PSTN core, universal workflow engine, vector platform, multi-agent hierarchy или distributed microservices.

## Первые доказательства

- Два параллельных reference domain pack на одном core: автозапчасти/ABCP и услуги/Montagio.
- В каждом первом релизе — один законченный коммерческий процесс, не полный охват отрасли.
- Платформенность доказана, когда третий клиент на существующем pack/connectors запускается без core changes, client fork и программиста максимум за один рабочий день настройщика после получения корректных доступов и данных.

## Пилот Олега

- Отдельная SIP-линия; основной кандидат Novofon + Yandex Realtime, Exolve — резерв.
- Staffy подбирает каталогически проверяемые позиции и создаёт подтверждённый заказ в ABCP; существующий обмен переносит его в кастомную 1С.
- Во время проверки заказ получает `pending_manager_verification`; менеджер проверяет позиции перед отправкой поставщику. Gate снимается отдельно по классам сложности на основании реальных ошибок.
- Главная метрика — маржинальный доход на 100 целевых звонков относительно человеческой линии; ноль двойных, неподтверждённых и непроверенных заказов.

## Омниканальность

- Первая очередь: web widget, Telegram, MAX и Jivo; VK/Avito — после account-level tests; WhatsApp Platform — после подтверждения Russia-specific eligibility.
- Django и ABCP подключают web widget script loader; Android/iOS используют тот же Staffy protocol до появления отдельного SDK.
- Для Олега Jivo остаётся human inbox. Для Montagio используется существующий операционный контур. Нельзя держать два writable inbox одного разговора.

## Коммерческая модель

- Сначала продаётся управляемый AI-сотрудник с assisted setup: запуск/integration + подписка + фактическое voice/AI usage.
- Новый reusable connector оценивается отдельно, но становится частью Staffy, а не клиентским форком.
- Self-service открывается по мере накопления проверенных шаблонов и не меняет архитектуру.

## Отвергнуто

- Вертикальный агент автозапчастей как конечная идентичность продукта — сужает исходную цель.
- «Полностью переписать Pipecat на 50-м клиенте из-за GIL» — основание опровергнуто.
- Полный собственный media/telephony stack с первого дня — не доказан для двух пилотов.
- `model + tools + RAG + MCP` как достаточная гарантия глубокой интеграции — не закрывает права и side-effect reliability.
- Клиентский fork, прямой произвольный доступ модели к системам, две параллельные бизнес-логики для voice/text и обязательная замена inbox клиента — запрещены.

## Непройденные гейты

- Novofon↔Yandex SIP/transfer/DTMF/data path и фактическая Russian PSTN latency.
- Chatwoot one-day spike, MAX bridge и полный data-egress inventory.
- Реальные пилотные экономика, качество, возвраты и нагрузка; синтетические eval их не заменяют.
