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

- Каждый Tenant и все принадлежащие ему настройки, данные, Connectors, секреты и доказательства изолированы. Проверенная личность создаёт Tenant и получает роль Owner; полномочия людей и машинных principals разделены, чувствительное администрирование Owner/Integration Admin требует MFA.
- Guided setup ведёт от Tenant и Employee Brief через Domain Pack, Connectors, знания, поведение и fallback к simulation/eval и публикации. Прогресс хранится как неопубликованный draft с владельцем и явным состоянием incomplete/blocked/ready; асинхронный blocker не теряет уже введённое.
- Knowledge Sources имеют происхождение, scope, приоритет, актуальность и версию. Путь публикации проходит quarantine/validation, extraction, index и eval; retrieval не подменяет актуальные операционные цену, наличие, доступность или статус.
- Поведение задаётся управляемыми полями и правилами: роль, AI-раскрытие, тон, Conversation Modes, Allowed Claims, Confirmation/Handoff и human fallback. Голос, темп, стиль и словарь произношения получают preview и человеческую проверку на телефонном качестве перед публикацией.
- Подключение Connector требует проверки конкретного account и каждой Capability; неизвестная, ограниченная или устаревшая возможность не становится разрешением на клиентское обещание или изменение внешней системы. Секреты не раскрываются модели и обычным логам.
- Настройка включает цели обработки и сроки хранения отдельно по классам данных, права доступа/исправления/отзыва/удаления, а также Entitlement, каналы, лимиты, бюджет и usage. Hard cap переводит новые обращения в заданный безопасный fallback, сохраняя активные обязательства, safety и Recovery.
- Публикация разрешена лишь для точной комбинации Tenant, Employee, route, modality, Autonomy Class, Release Manifest и Operating Profile с действующими gates. До controlled/live Owner утверждает Operating Profile с каналами, языками, часами, capacity, human coverage, retention и правилами измерения; неподтверждённые зависимости остаются явно заблокированными.
- Общая платформа не содержит tenant-name branches, клиентских форков или произвольного tenant-кода. Новая компания настраивается через общее ядро, versioned Domain Pack, конфигурацию Employee и переиспользуемые Connectors.
- Изменения конфигурации, прав, capabilities, consent, entitlement и gate evidence аудируются и версионируются. Отзыв или истечение допуска ограничивает затронутый scope и инвалидирует ожидающую работу; rollback не отменяет уже совершённые внешние действия.

## Technical Decisions

- Реализация — Go modular monolith с шестью модулями. Tenant, principal, membership, access и entitlement принадлежат `identity`; только модуль-владелец импортирует собственный `store`. Внутренняя форма модуля: `api/`, `app/`, `domain/`, `port/`, `store/`.
- Модули общаются через typed API и versioned outbox events. Cross-module SQL, циклические зависимости, глобальные mutable dependencies, общие `utils/helpers/common` и доменная политика в composition roots запрещены.
- PostgreSQL 18 использует shared schema: каждая tenant-owned таблица содержит `tenant_id`, tenant-aware keys, `ENABLE ROW LEVEL SECURITY` и `FORCE ROW LEVEL SECURITY`. Runtime principal — non-owner, non-superuser, `NOBYPASSRLS`; TenantContext задаётся через `SET LOCAL` только внутри транзакции, а отсутствие контекста останавливает выполнение до SQL.
- Идентификаторы — typed UUIDv7; JSON и БД используют `snake_case`; время — UTC `timestamptz`/RFC3339. HTTP API находится под `/v1`, валидируется OpenAPI 3.1 и возвращает стабильную problem-форму с безопасным сообщением, correlation ID, retryability и field errors.
- Опубликованные конфигурации неизменяемы и связаны Release Manifest с версиями Domain Pack, policy/eval, Connector contract/build, model/media route и runtime. Материальное изменение создаёт новый manifest; точечный rollback и совместимость текущего/предыдущего event/job contract обязательны.
- Обычные логи структурированы через `slog`; raw PII, credentials и содержимое разговоров исключаются из логов, metric labels и span names.

## UX & Interaction Patterns

- После создания Owner попадает в App Shell своего Tenant и видит название компании, роль и следующий шаг. Employee Brief создаётся из интервью, редактируется по секциям и отдельно утверждается Owner; Launch Plan показывает блокеры, ответственного, делегирование и возможность продолжить.
- Integration Card ведёт от владельца credentials к account check, capability test и admission. Knowledge Source Row показывает provenance/freshness/conflict; Test & Release Panel начинает с simulation и объясняет причину каждого блокирующего gate. Draft и published версии остаются различимыми, а материальный конфликт не сливается автоматически.
- Admin/ops интерфейс использует светлую тему «Северный кедр», role-adaptive навигацию и текстовые статусы. Состояние и роль не кодируются только цветом.
- Все поверхности поддерживают клавиатуру, видимый focus, семантические landmarks, reflow на 320 px без горизонтального скролла страницы и интерактивные цели не менее 44×44 px.

## Cross-Story Dependencies

Story 1.1 создаёт корень безопасности и владения для всех последующих историй. Story 1.2 расширяет membership и полномочия; Story 1.3 создаёт tenant-scoped Employee. Domain Pack, знания, поведение, голос и Connectors формируют версионированную конфигурацию; capability evidence и privacy/entitlement ограничения входят в Release Manifest и Operating Profile. Story 1.14 допускает публикацию только после выполнения всех обязательных зависимостей и gates выбранной стадии.
