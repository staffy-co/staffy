---
title: 'Story 1.5 — Безопасный приём сайтов и документов, синтетический контур'
type: 'feature'
created: '2026-09-26'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '5f3c935082e6d753e9c8a3b84daca0ebf13ae0b1'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-staffy-2026-09-13/GATE-REGISTRY.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Настройщик может ввести знания текстом, но не добавить страницу или документ.

**Approach:** Добавить приём файла/HTTPS-страницы: карантин → проверка → извлечение → preview. Использовать существующие версии и provenance. До AG-4 — только синтетические фикстуры; тестовый сканер не допускает реальный источник в `ready`.

**Решение об объёме:** В первом приёме поддерживаются UTF-8 TXT и PDF с текстовым слоем; DOCX и OCR сканированных PDF отложены. Для сайта принимается одна публичная HTTPS-страница по точному адресу, без авторизации и обхода ссылок.

## Boundaries & Constraints

**Always:** Показать типы/лимиты до выбора. Проверять фактическую сигнатуру, размер, URL, каждый redirect и адрес назначения; MIME клиента не доверять. Без подтверждённого scanner/safe extraction реальный источник остаётся в карантине. Сбой — `failed`/`quarantined` с безопасной причиной, без raw payload в логах. Фрагменты сохраняют URL/имя, время, версию, позицию, HMAC; повторы идемпотентны. Сохранить Tenant/RLS, CAS, audit/outbox, доступность UI и запрет считать документ Owning System.

**Never:** Не выполнять активный контент, не обращаться к внутренним адресам, не отправлять payload модели, не индексировать/публиковать без eval/release. Не передавать URL/HTML в `ValidKnowledgeText`; HMAC-retention — отдельный срез.

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Результат | Защита |
|---|---|---|---|
| Тестовый источник | Фикстура, draft с Pack | `processing` → тестовый preview, версия/provenance | Не публикуется |
| Опасный файл | Сигнатура/лимит/scan не пройдены | `quarantined`/`failed`; прежняя версия цела | Нет payload |
| Опасный URL | Private IP, rebinding, redirect, схема | Отказ на каждом hop | Нет SSRF |
| Повтор/изменение | Тот же/новый текст | Replay или новая immutable версия/diff | Нет repin |
| Изоляция/UX | Чужой Tenant; 320 px/клавиатура | Нет утечки; доступный прогресс | RLS/role fence |

</frozen-after-approval>

## Code Map

- `internal/employee/domain/knowledge.go` — Source/Version/HMAC; три текстовых типа и 256 KiB, проверку текста не ослаблять.
- `internal/employee/{api,app,port,store}/knowledge.go` — scope, версии, replay, audit/outbox; новый ingestion перед `ProcessKnowledge`.
- `migrations/employee/00003_knowledge_sources.sql` — CHECK типов/состояний, FORCE RLS; расширить новой миграцией.
- `internal/transport/http/knowledge.go`, `contracts/openapi/staffy.yaml` — добавить ingress API/лимиты; authority не из payload.
- `web/templates/employee.html`, `web/assets/app.css` — Row/progress/preview, 320 px.
- `tests/integration/knowledge_test.go`, `tests/journeys/zz-knowledge.spec.ts` — RLS, lifecycle, SSRF, доступность.

## Tasks & Acceptance

**Execution:**
- [x] `internal/employee/{domain,api,app,port,store}/knowledge.go`, `migrations/employee/` — lifecycle/provenance без ослабления RLS/CAS/audit.
- [x] `internal/employee/ingest/` — ограниченный приём, scanner port и синтетическое извлечение; реальное extraction закрыто до AG-4.
- [x] `internal/transport/http/knowledge.go`, `contracts/openapi/staffy.yaml`, `web/` — API, лимиты, безопасные ошибки, доступный UI.
- [x] `tests/`, `internal/architecture/architecture_test.go` — матрица, SSRF/RLS/replay/fail-closed.

**Acceptance Criteria:**
- Given draft с Pack, when добавляют синтетический источник, then есть `processing` и тестовый preview с версией/provenance.
- Given реальный или опасный файл/URL при открытом AG-4, when обрабатывают, then нет `ready`; причина видима без утечки.
- Given повтор/изменение, when извлечение завершено, then replay либо новая immutable версия без repin.

## Implementation Notes

Добавлен отдельный ingress для `website`/`document`. Публичный URL сохраняется в карантине без запроса страницы или DNS-поиска. Реальный файл не сохраняется: попытка получает `failed` с явной просьбой загрузить его повторно после AG-4, чтобы не создавать ложного ожидания от карантина. Реальный scanner/safe extraction представлен fail-closed портом, а встроенные синтетические фикстуры дают только `preview`, никогда `ready` или publishable. Существующий текстовый путь остался отдельным; его нельзя использовать для обработки website/document.

Миграция расширяет типы/статусы и provenance, сохраняя FORCE RLS. Откат отказывается уничтожать новые source/version данные. Создание синтетической фикстуры оставляет наблюдаемое `processing`; отдельная команда с новой mutation envelope переводит её в `preview`, после чего доступны replay и новая версия.

Проверки 2026-09-26: `go test ./...`, `go vet ./...`, `go test -tags=integration ./tests/integration/...` на временной PostgreSQL 16 и все 8 Playwright-сценариев прошли. Целевая PostgreSQL 18 и реальный scanner/egress остаются до AG-4; синтетический путь не объявляется production-ready.

## Spec Change Log

## Review Triage Log

| Замечание | Вердикт / маршрут | Обоснование |
|---|---|---|
| PDF без текстового слоя | low / reject | Сигнатура принимает такой PDF как кандидата, но любой реальный файл сразу получает `failed` и не извлекается. Проверка текстового слоя потребуется безопасному извлекателю после AG-4; сейчас она не изменит результат. |
| Зарезервированные IP-адреса | low / reject | `ValidateAddresses` может принять некоторые зарезервированные диапазоны, но активный путь сохраняет URL в карантине без DNS и сетевого запроса. Перед разрешением egress нужна полная политика публичных адресов. |
| Тип тестовой фикстуры | low / patch | `IngestKnowledge` допускал тип website с документной фикстурой; теперь фикстура ограничена типом document. |
| HTTP 201 для неуспешного файла | false / reject | Создаётся запись источника, а `status=failed` и предупреждение формы явно сообщают, что байты не сохранены. Код 201 относится к записи, не к извлечению. |
| ID фикстуры в каждой версии | low / reject | Неизменяемый текст, checksum, время и происхождение семейства различают версии; ID фикстуры — внутренний тестовый переключатель. Добавление его в публичную модель не оправдано. |
| Обработка после конфликта | false / reject | `EffectiveStatus` отдаёт конфликту приоритет над `preview`: обработка не снимает конфликт и не открывает публикацию. Такое наложение уже действует для текстовых источников. |
| Дубли полей multipart | low / patch | `FormValue` молча выбирал одно значение; неоднозначные поля теперь отвергаются до передачи команды. |
| Поля URL и файла в форме | medium / patch | Оба поля были необязательны при любом типе, что позволяло случайно создать ошибочную запись. Теперь нужное поле обязательно, другое отключено. |
| Происхождение каждого фрагмента в UI | low / reject | `ReadKnowledge` возвращает происхождение и время фрагмента; интерфейс показывает происхождение источника и время версии. Повторять это в каждой строке фрагмента сейчас избыточно. |
| Дополнительные пограничные тесты | low / reject | Текущие тесты проверяют CAS, RLS, replay, опасные URL/файлы и узкий экран. Конкретные пробелы HTTP multipart и дублирования полей рассмотрены отдельно. |
| Межсайтовый multipart API | medium / patch | API принимал браузерную форму без `validFormOrigin`; при cookie-авторизации это позволяло бы межсайтовую отправку. Теперь действует та же проверка Origin. |
| Политика адресов, повторное замечание | low / reject | Подтверждено то же ограничение; fetcher отключён, реальные URL остаются в карантине. |
| PDF только с изображениями, повторное замечание | low / reject | Подтверждено то же ограничение классификации; реальные PDF не извлекаются и не допускаются. |
| Пробел проверки multipart | medium / patch | Тест сервиса не ловил потерю имени или байтов на HTTP-границе. Добавлен запрос multipart с проверкой точных байтов и имени. |

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.
