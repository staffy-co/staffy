# Epic 1 Context: Защищённая самостоятельная настройка ИИ-сотрудника

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Дать владельцу бизнеса изолированное пространство Staffy, в котором он без изменения общего ядра и обязательного участия программиста создаёт Tenant, управляет доступом, настраивает ИИ-сотрудника через версионированные Domain Pack и Connectors, проверяет реальные capabilities и допускает точную конфигурацию только к разрешённой стадии. Настройка должна быть понятной и поэтапной, а неполные, устаревшие или недоказанные зависимости — оставаться явно заблокированными и fail-closed.

## Stories

- Story 1.1: Создание защищённого пространства Tenant
- Story 1.2: Управление участниками и полномочиями Tenant
- Story 1.3: Создание черновика ИИ-сотрудника через guided setup
- Story 1.4: Подключение версионированного Domain Pack
- Story 1.5: Управление жизненным циклом Knowledge Sources
- Story 1.6: Безопасная настройка поведения ИИ-сотрудника
- Story 1.7: Настройка голоса и произношения
- Story 1.8: Выбор и безопасное подключение интеграций
- Story 1.9: Проверка capabilities конкретного подключения
- Story 1.10: Управление периметром данных, сроками хранения и правами субъекта
- Story 1.11: Entitlement, лимиты и прозрачность usage
- Story 1.12: Версии конфигурации и Release Manifest
- Story 1.13: Утверждение Operating Profile и правил измерения
- Story 1.14: Readiness-проверка и контролируемая публикация

## Requirements & Constraints

- Каждый Tenant и все принадлежащие ему объекты изолированы на уровне приложения и PostgreSQL. Межтенантные чтения, записи, связи и утечки через фоновые задачи, connection pool, prepared statements, rollback, поддержку и восстановление запрещены и покрываются отрицательными интеграционными тестами.
- Проверенная платформенная личность создаёт Tenant идемпотентно, получает роль Owner и оставляет неизменяемое событие с actor, временем и correlation ID. Пользовательский `tenant_id` не является источником доверия.
- Роли и чувствительные действия применяют deny-by-default, server-side authorization и step-up/MFA там, где это требуется. Секреты, PII и чувствительные payload не попадают в обычные логи, prompts или receipts.
- Guided setup хранит только явные draft/versioned состояния. Технический success отдельного элемента не означает общую готовность; публикация разрешена лишь для точной комбинации Tenant, Employee, route, modality, Autonomy Class, Release Manifest и Operating Profile с действующими gates.
- Общая платформа не содержит tenant-name branches, клиентских форков или произвольного tenant-кода. Новая компания настраивается через общее ядро, versioned Domain Pack, конфигурацию Employee и переиспользуемые Connectors.
- Изменения конфигурации, прав, capabilities, consent, entitlement и gate evidence должны быть аудируемыми, версионированными и безопасно отзывать или ограничивать затронутый admission без ложного terminal success.

## Technical Decisions

- Реализация — Go modular monolith с шестью модулями. Tenant, principal, membership, access и entitlement принадлежат `identity`; только модуль-владелец импортирует собственный `store`. Внутренняя форма модуля: `api/`, `app/`, `domain/`, `port/`, `store/`.
- Модули общаются через typed API и versioned outbox events. Cross-module SQL, циклические зависимости, глобальные mutable dependencies, общие `utils/helpers/common` и доменная политика в composition roots запрещены.
- PostgreSQL 18 использует shared schema: каждая tenant-owned таблица содержит `tenant_id`, tenant-aware keys, `ENABLE ROW LEVEL SECURITY` и `FORCE ROW LEVEL SECURITY`. Runtime principal — non-owner, non-superuser, `NOBYPASSRLS`; TenantContext задаётся через `SET LOCAL` только внутри транзакции, а отсутствие контекста останавливает выполнение до SQL.
- Идентификаторы — typed UUIDv7; JSON и БД используют `snake_case`; время — UTC `timestamptz`/RFC3339. HTTP API находится под `/v1`, валидируется OpenAPI 3.1 и возвращает стабильную problem-форму с безопасным сообщением, correlation ID, retryability и field errors.
- Создание Tenant, назначение Owner и outbox record выполняются атомарно. Повтор с тем же idempotency key возвращает тот же результат; конфликтующий material payload не создаёт новый Tenant.
- Обычные логи структурированы через `slog`; raw PII, credentials и содержимое разговоров исключаются из логов, metric labels и span names.

## UX & Interaction Patterns

- После создания Owner попадает в App Shell своего Tenant и видит название компании, роль и один честный следующий шаг — «Создать ИИ-сотрудника». Пустое состояние не заявляет о готовности сотрудника, каналов или интеграций.
- Admin/ops интерфейс использует светлую тему «Северный кедр», role-adaptive навигацию и текстовые статусы. Состояние и роль не кодируются только цветом.
- Все поверхности поддерживают клавиатуру, видимый focus, семантические landmarks, reflow на 320 px без горизонтального скролла страницы и интерактивные цели не менее 44×44 px.

## Cross-Story Dependencies

Story 1.1 создаёт корень безопасности и владения для всех последующих историй. Story 1.2 расширяет membership и полномочия; Story 1.3 создаёт tenant-scoped Employee. Domain Pack, знания, поведение, голос и Connectors формируют версионированную конфигурацию; capability evidence и privacy/entitlement ограничения входят в Release Manifest и Operating Profile. Story 1.14 допускает публикацию только после выполнения всех обязательных зависимостей и gates выбранной стадии.
