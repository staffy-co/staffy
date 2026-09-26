---
title: 'Story 1.5 — Текстовые Knowledge Sources, первый безопасный срез'
type: 'feature'
created: '2026-09-26'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '0401e8434d174d8d06c4bff6f1e9eccc4b53044f'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** После выбора Domain Pack настройщик не может сохранить проверяемые знания с происхождением, актуальностью и версией.

**Approach:** Первый срез — tenant-scoped реестр вводимых текстом FAQ, регламентов и примеров диалога. Настройщик создаёт источник, проверяет обработанный текст и provenance, видит версии/устаревание/конфликты и может вывести источник из эксплуатации. Этот срез не закрывает всю Story 1.5.

**Решение об объёме:** Реальные URL, загрузка файлов, malware scanning, безопасное извлечение PDF/DOCX/HTML и сетевой egress — следующий отдельный срез после выбора и проверки компонентов AG-4. До того Staffy не принимает их как пригодное знание. Вопрос форматов решается перед тем срезом, а не сейчас.

## Boundaries & Constraints

**Always:** Source хранит ID, Tenant/Employee, тип, название, владельца, provenance, scope/priority, язык, время и срок актуальности. Создание даёт `processing`; отдельный `process` принимает UTF-8 текст ≤256 KiB, сохраняет immutable checksum/version и фрагменты с source/version/position/checksum. `ready` — только просмотр, не eval/публикация. Текст недоверенный: не меняет policy/tools/authority. Повтор checksum идемпотентен; изменение создаёт версию/diff без repin. Человеческая отметка конфликта сохраняет оба provenance; authority/priority не выбирается автоматически. Stale/retired исключаются из будущего использования. Mutation — Identity `configuration`, TenantContext/FORCE RLS, version check, audit/correlation/outbox без raw text. UI: клавиатура, screen reader, 320 px, progress без захвата focus.

**Never:** URL/file/HTML, внешний egress, live retrieval/AI, Release Manifest, публикация Employee, седьмой модуль или Tenant-код. Документ не доказывает текущую цену, наличие, статус, доступность.

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Результат | Защита |
|---|---|---|---|
| Создание/обработка | Draft + Pack, текст FAQ/регламента/диалога | `processing` → previewable `ready`, ID/chunks/provenance | Tenant/auth |
| Отказ | URL/file/HTML, чрезмерный текст | Ошибка; прежняя версия цела | Allowlist/limit |
| Повтор/изменение | Тот же/новый checksum | Replay / immutable версия и diff | Без repin |
| Конфликт/stale | Человеческая отметка/истёк срок | Оба provenance; блокирующий статус | Без auto-truth |
| Retire/изоляция | Свой/чужой Source | Future use исключён / 404 | Audit/RLS |

</frozen-after-approval>

## Code Map

- `internal/employee/{api,app,domain,port,store}` — owning-модуль; reuse `Service.scope`, versioned save, outbox; pack `knowledge_ref` не равен Tenant Source.
- `internal/identity/api`, `internal/platform/{tenancy,id}` — verified principal, `Configuration`, TenantContext, UUIDv7.
- `migrations/employee/00001_employee_draft.sql`, `00002_domain_pack.sql` — образцы FORCE RLS/audit/outbox.
- `internal/transport/http/{employee,server}.go`, `contracts/openapi/staffy.yaml` — API/SSR/problem/correlation; authority не из payload.
- `web/templates/employee.html`, `web/assets/app.css`, `tests/{integration,journeys}` — Row и 320 px.

## Tasks & Acceptance

**Execution:**
- [x] `internal/employee/{domain,api,app,port,store}` — text lifecycle, versions, conflicts, retire, idempotency.
- [x] `migrations/employee/` — tenant source/version/chunk/audit/outbox, FORCE RLS, rollback.
- [x] `internal/transport/http/`, `contracts/openapi/staffy.yaml` — безопасные команды и просмотр.
- [x] `web/templates/employee.html`, `web/assets/app.css` — Row/status/preview/diff и предупреждение.
- [x] `tests/`, `internal/architecture/architecture_test.go` — матрица, изоляция, версии, 320 px.

**Acceptance Criteria:**
- Given draft с Pack, when Owner/Configurator вводит допустимый текст, then получает processing Source с метаданными и ID.
- Given URL/file/HTML либо чужой Tenant, when запрашивают save/read, then нет пригодного знания или утечки.
- Given обработанный Source, when открывают preview, then видны текст, версия, позиция, checksum и provenance.
- Given повтор/изменение, when обрабатывают текст, then нет дубля либо создана новая версия без repin.
- Given stale/conflict/retire, when открывают Row, then причина видима, а publish заблокирован.

## Implementation Notes

Первый срез реализован без приёма URL/файлов и без публикации Employee. Источники имеют tenant-scoped версии, фрагменты, происхождение, состояния устаревания/конфликта/вывода; контрольные суммы защищены отдельным 32-байтным HMAC-ключом Source. Ключ не включён в API и outbox; его ротация и уничтожение оставлены для retention-среза.

Матрица подтверждена запущенными проверками: создание/обработка, отклонение URL/HTML и tenant isolation, replay/новая версия, conflict/stale, retire/RLS — `TestKnowledgeLifecycleIsolationVersionsAndEvidence`; предел размера и UTF-8 — `TestKnowledgeTextValidationAndChunks`; интерфейс при 320 px и с клавиатурой — `zz-knowledge.spec.ts`. Существующий архитектурный тест проверяет границы модулей. Полный набор Go, vet, интеграционных и 7 браузерных тестов прошёл 2026-09-26.

Ревью привело к проверке JSON process-handler, запрету адресов с иными `://`-схемами и возврату сохранённой immutable версии на replay; полный набор проверок после исправлений прошёл. Этот spec завершает только утверждённый первый срез; ключ `1-5-управление-жизненным-циклом-knowledge-sources` в sprint-status остаётся `in-progress` до следующего среза с URL/файлами, scanning, extraction, eval и retention.

## Spec Change Log

## Review Triage Log

| Рецензент / находка | Вердикт и проверенное основание | Маршрут |
|---|---|---|
| Blind 1 — конфликт после новой версии | false: `ProcessKnowledge` намеренно не снимает человеческую отметку; новая редакция текста сама по себе не доказывает разрешение конфликта и не должна автоматически выбирать истину. | reject |
| Blind 2 — replay со старой версией записи | false: replay не мутирует состояние и возвращает актуальный Source, включая его конфликт/срок; разрешение повтора с прежней версией реализует утверждённую идемпотентность. | reject |
| Blind 3 — свободный текст конфликта | low: note и второе provenance действительно свободный текст, но Row показывает оба provenance, а версия текущего Source и аудит сохраняются; отдельная типизированная модель спорного утверждения для этого первого среза добавила бы новый процесс без доказанного потребителя. | reject |
| Blind 4 — нет редактирования метаданных | low: существующий путь не меняет owner/scope/fresh_until после создания; утверждённый первый срез предусматривает создание, просмотр и retire, а не повторную валидацию источника. Изменение API/истории для редко исправляемого источника непропорционально. | reject |
| Blind 5 — создание с истёкшим сроком | false: срок в прошлом допустим для регистрации исторического материала; `EffectiveStatus` немедленно показывает stale, браузерный тест подтверждает явную причину и блокировку. | reject |
| Blind 6 — diff без позиции | low: `KnowledgeDiff` показывает только изменённый участок и может быть неоднозначен при повторах; рядом доступны полные неизменяемые тексты обеих версий, поэтому ущерб при ручном просмотре мал, а контекстный diff значительно сложнее. | reject |
| Blind 7 — все версии на странице | low: SSR действительно читает весь текст всех версий; в ручном первом срезе нет массового импорта и типичная история короткая. Пагинация/ленивая загрузка меняют интерфейс и контракт сверх текущей проверки. | reject |
| Blind 8 — нет квоты источников/версий | low: многократные допустимые записи могут увеличивать хранилище; текущий ввод ручной и ограничен 256 КиБ за запрос, ежедневный пользовательский сценарий не создаёт тысячи версий. Политика квот требует отдельного решения. | reject |
| Blind 9 — неудачные попытки без knowledge-аудита | false: `knowledge_audit` и outbox фиксируют успешно совершённые переходы; отказавшая валидация не является mutation и не создаёт событие состояния. | reject |
| Blind 10 — нет схем успешных ответов OpenAPI | low: описания ответов действительно без типизированных тел, как и соседние существующие операции контракта; в текущем проекте нет генерируемого клиента, поэтому немедленного сбоя потребителя нет. Полная типизация расширила бы контракт. | reject |
| Gap 1 — JSON process не проверен HTTP-тестом | medium: форма и сервис проверены, но тест JSON-handler отсутствует; неверная передача версии или текста останется незамеченной. | patch |
| Gap other — replay отдаёт незаполненные поля фрагмента | medium: replay возвращает вновь собранный `v` до присвоения `Chunks[i].SourceID/Version`, поэтому ответ нарушает требуемую связь фрагмента с источником и версией. | patch |
| Edge 1 — replay отдаёт новый timestamp/пустые ID | medium: код действительно создаёт `v` с текущим `p.Now`, а затем отдаёт без чтения сохранённой версии и её идентичностей. | patch |
| Edge 2 — URL иной схемы проходит | high: `ValidKnowledgeText` блокирует только http(s) и угловые скобки; `ftp://` или `file://` проходит и становится ready вопреки запрету URL/file. | patch |

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.
