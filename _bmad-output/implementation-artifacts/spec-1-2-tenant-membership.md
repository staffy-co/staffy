---
title: 'Story 1.2 — Управление участниками и полномочиями Tenant'
type: 'feature'
created: '2026-09-26'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '57c5da8717f14a3f47c3f5750490c32487f345e8'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** После Story 1.1 только создатель Tenant получает роль Owner. Компания не может безопасно подключить коллег к Staffy.

**Approach:** Добавить приглашение, принятие, роли, области, отзыв доступа и раздел «Участники» в App Shell через общий модуль `identity` и доверенный `PrincipalResolver`.

## Boundaries & Constraints

**Always:** Приглашение истекает; до принятия подтверждённой личностью доступ закрыт. Операции привязаны к Tenant, проверяются сервером и аудируются с actor, прежним/новым состоянием, причиной, временем и correlation ID. Неизвестное право означает запрет. Снижение прав действует со следующего запроса; последний Owner защищён транзакционно. Назначение Owner/Integration Admin и чувствительные действия требуют step-up, иначе состояние не меняется. RLS, `NOBYPASSRLS`, privacy и доступность 320 px сохраняются.

**Never:** Клиентский `tenant_id`, роль, scope или флаг MFA не доказывают право. Не раскрывать чужой Tenant/участника; не логировать адреса, токены и PII; не реализовывать login/OIDC или права ИИ-сотрудника здесь.

**Принятые решения:** Owner приглашает по email и получает копируемую одноразовую ссылку; автоматической отправки пока нет. Доверенный `PrincipalResolver` передаёт только подтверждённый email приглашённого, а секрет ссылки хранится как digest; приглашение действует семь дней. Повторная отправка обновляет pending-приглашение и отзывает старую ссылку. Права — одна или несколько ролей плюс выбранные области в пределах объединения их максимумов. Owner всегда имеет все области и право управления участниками. Максимумы: Configurator — настройка; Operator — операции/претензии; Expert/QA — проверка/техническая экспертиза/аудит; Integration Admin — управление интеграциями внутри настройки. Список областей явно содержит настройку, операции, проверку, техническую экспертизу, претензии и аудит. Повышенное подтверждение поступает только от доверенной identity-границы; пока production-провайдер не подключён, чувствительное изменение закрыто.

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Результат | Ошибка / защита |
|---|---|---|---|
| Приглашение | Owner, получатель, роли/области | Одно `pending` membership с expiry и безопасным способом передачи приглашения | Повтор не создаёт активный дубль |
| Принятие | Действующее приглашение, совпадающая verified identity | Одно `active` membership | Чужая identity, истечение и повтор не расширяют права |
| Изменение/отзыв | Owner и действующий участник | Новые права видны следующему запросу; аудит перехода | Последний Owner, stale version, missing step-up и чужой Tenant блокируются |
| Запрещённая область | Ограниченный участник, UI или API | Безопасный отказ и минимальная audit-запись | Payload не может повысить права |

</frozen-after-approval>

## Code Map

- `internal/identity/domain/model.go`, `api/api.go`, `port/port.go` — расширить principal/membership, команды и API авторизации; сейчас есть лишь `Owner` и `Workspace`.
- `internal/identity/app/service.go` — повторно использовать trusted principal и typed TenantContext; добавить deny-by-default policy, invitation/acceptance, step-up port и запрет последнего Owner.
- `internal/identity/store/postgres.go`, `migrations/identity/00002_identity_membership.sql` — расширить membership/RLS/audit, сериализовать изменения и проверять версию.
- `internal/transport/http/server.go`, `contracts/openapi/staffy.yaml` — tenant-scoped endpoints/формы, безопасные problem-ответы, server-side authorization; не принимать полномочия из заголовка как trust.
- `web/templates/people.html`, `web/assets/app.css` — Organization → People, текстовые статусы, явное подтверждение и recovery при недоступном step-up.
- `tests/integration/membership_test.go`, `internal/transport/http/server_test.go`, `tests/journeys/people.spec.ts` — проверить tenant/permission, expiry/replay/concurrency/last-owner и keyboard/320 px; обновить фальшивые `api.Service`.

## Tasks & Acceptance

**Execution:**
- [x] `internal/identity/domain/model.go`, `internal/identity/api/api.go`, `internal/identity/port/port.go`, `internal/identity/app/service.go` — определить роли, области и переходы invitation → active → revoked с fail-closed step-up.
- [x] `migrations/identity/00002_identity_membership.sql`, `internal/identity/store/postgres.go` — добавить tenant-scoped хранение, RLS, транзакционный last-owner guard и неизменяемый аудит переходов.
- [x] `internal/transport/http/server.go`, `contracts/openapi/staffy.yaml` — открыть операции списка, приглашения, принятия, изменения и отзыва через Identity API.
- [x] `web/templates/people.html`, `web/assets/app.css` — сделать доступный редактор с честными ошибками и состояниями.
- [x] `tests/integration/membership_test.go`, `internal/transport/http/server_test.go`, `tests/journeys/people.spec.ts` — проверить I/O-матрицу, параллельное изменение и отсутствие утечки через пул.

**Acceptance Criteria:**
- Given Owner с разрешённым step-up, when он назначает роли и области, then Tenant видит точное ограниченное membership и аудит перехода.
- Given подтверждённый получатель, when он принимает действующее приглашение, then доступ появляется один раз и только в этом Tenant.
- Given ограниченный участник, when он обращается к запрещённой области через UI/API, then сервер отказывает без раскрытия чужих данных.
- Given отзыв или снижение прав, when поступает следующий запрос, then прежний доступ уже не действует; последний Owner сохраняется.
- Given клавиатура, screen reader и 320 px, when Owner управляет участниками, then статусы и ошибки читаемы, focus логичен, цели ≥44×44 px.

## Implementation Notes

Реализованы приглашения по подтверждённому email, одноразовое принятие, роли и области, смена и отзыв прав, раздел «Участники», RLS и аудит. Чувствительные действия требуют доверенного step-up; текущий test/dev-адаптер его не выдаёт, поэтому эти операции в локальном интерфейсе честно закрыты до подключения провайдера.

## Spec Change Log

## Review Triage Log

- `high`, patch — `store/membership.go:246`: принятие читает приглашение до tenant-lock, а UPDATE не проверяет digest заново; пересоздание ссылки между ними позволяет принять отозванный секрет.
- `medium`, patch — `store/membership.go:246`: срок сравнивается с временем, взятым до ожидания блокировки; принятие возможно после фактического истечения.
- `medium`, patch — `store/membership.go:262`: существующее `revoked` membership всегда даёт conflict, поэтому повторно приглашённый сотрудник не может вернуться; то же замечание blind-hunter.
- `medium`, patch — `store/membership.go:197`: приглашение аудируется с пустым subject и без ID приглашения; несколько приглашений нельзя различить без PII.
- `false`, reject — `store/membership.go:197`: повторная отправка pending без версии следует утверждённому решению «обновляет pending-приглашение и отзывает старую ссылку»; tenant-lock сериализует действия. Версионная UX-семантика здесь не обещана.
- `low`, reject — `store/membership.go:143`: `Authorize` доверяет сохранённым scopes, но единственные прикладные записи проверяют `ValidateGrant`, а прямое изменение БД требует привилегированного доступа вне границы этой истории; опасный путь для обычного пользователя не показан.
- `low`, reject — `00002_identity_membership.sql`: старое scalar `role` допускает значения новой модели, но прикладные записи поддерживают согласованность; workspace использует его лишь для отображения. Отдельная проверка БД повысила бы глубину защиты, но не исправляет показанный доступный сбой.
- `high`, patch — `http/membership.go:222`: form POST пропускает отсутствующий Origin; при cookie-сессии это допускает CSRF на новых путях.
- `maybe-false`, defer — `http/membership.go:263`: секрет содержится в одноразовой ссылке; `no-referrer` и `no-store` выставлены, но конфигурация внешнего reverse proxy и его access-log неизвестна. Надо проверить перед публичным деплоем, логируются ли query strings.
- `medium`, patch — `people.html:10`: форма изменения не отмечает текущие роли/области; пользователь может случайно сбросить несвязанные права.
- `medium`, patch — `people.html:10`: отзыв не просит отдельного явного подтверждения перед отправкой.
- `false`, reject — `app/service.go:121`: fail-closed при отсутствии production step-up прямо предусмотрен frozen intent; это не дефект реализации истории.
- `medium`, patch — `tests/integration/membership_test.go:78`: тесты не проверяют поля и полноту audit-переходов; текущая запись может потерять actor/state и всё равно пройти.
- `medium`, patch — `internal/transport/http/membership_test.go:41`: нет тестов успешных HTTP accept/change/revoke и form-путей; service-тесты не ловят ошибку маршалинга и передачи команд.

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.

2026-09-26: все перечисленные проверки прошли; интеграционные тесты выполнены на изолированной PostgreSQL 16, браузерные — в Chromium (3/3). Матрица: приглашение/повтор и принятие/истечение/повтор — `TestMembershipInvitationAcceptanceAndLastOwner`, `TestExpiredInvitationNeverActivates`; изменение/отзыв/конкуренция — `TestMembershipInvitationAcceptanceAndLastOwner`, `TestConcurrentOwnerRevocationLeavesOneOwner`; запрет области и безопасный API-отказ — `TestIntegrationAdminCannotConfigure`, `TestMemberAPIScopesAndSafeDenial`; UI и 320 px — `people.spec.ts`.

После review исправлены гонки принятия, повторное вступление, подтверждение отзыва, защита form POST и предвыбор текущих прав. Повторно прошли `go test ./...`, `go vet ./...`, `go test -count=1 -tags=integration ./tests/integration/...` (10/10), `npm test --prefix tests/journeys` (3/3), `git diff --check`.
