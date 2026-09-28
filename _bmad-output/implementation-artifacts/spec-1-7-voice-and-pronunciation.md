---
title: 'Story 1.7 — Настройка голоса и произношения'
type: 'feature'
created: '2026-09-27'
status: 'done'
route: 'dispatch'
review_loop_iteration: 1
baseline_commit: '623bf027680dbe30dac657c69778aac34f4c017e'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-staffy-2026-09-13/GATE-REGISTRY.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Настройщик не может выбрать допустимый голос и проверить произношение важных слов. Простое сохранение фонетической записи не доказывает, что клиент верно услышит критическую сущность.

**Approach:** Добавить версионированные provider-neutral Voice Profile и словарь произношения в черновик Employee; показывать совместимость и честные preview capabilities, а аудиопроверку проводить только разрешённым синтетическим маршрутом.

**Решение владельца от 2026-09-27:** Для первой версии использовать локальный офлайн test-TTS (вариант A). До включения конкретного движка проверить лицензию, состав зависимостей и безопасную обработку входного текста. Если допуск не пройден или движок недоступен, preview возвращает явную ошибку; синтетическое звучание не засчитывается как проверка телефонного качества.

**Уточнение владельца от 2026-09-27:** Критичность каждой словарной записи явно классифицирует настройщик: «не критическая» либо конкретный вид критической сущности. Пустая классификация не принимается; критическая запись требует readback и будущей человеческой проверки. Для словаря и preview разрешены только вымышленные тестовые данные: настройщик явно подтверждает это, система не заявляет, что умеет автоматически распознавать все персональные данные. Результаты preview и оценки хранятся ограниченное время; для локального пилота выбран срок 24 часа с автоматическим физическим удалением отдельным обслуживающим процессом не позднее следующего минутного цикла при его непрерывной работе (включая текст) и отказом в оценке истёкшего результата. При запуске обслуживающий процесс сначала удаляет накопившиеся просроченные результаты. Обычная роль приложения не может запускать очистку; deployment обязан выдать отдельной LOGIN-учётной записи членство только в NOLOGIN-роли очистки и запускать процесс с отдельным Database URL. Постоянный audit/outbox не содержит исходного или синтезированного текста.

## Boundaries & Constraints

**Always:** Профили допускаются каталогом с языком, стилями, диапазоном темпа, ограничениями и статусом preview; неизвестное/неподтверждённое не выбирается. Настройка только для Employee с `phone` и совместимым языком. Словарь хранит исходное написание, язык, scope, нормализованное произношение, тестовую фразу и статус поддержки; дубли и конфликты отклоняются. Для VIN/OEM, артикула, телефона, адреса, даты, суммы и Pack-critical сущностей нужны структурированный readback и будущая проверка человеком на телефонном качестве. Каждое сохранение — новая неизменяемая tenant-scoped версия с CAS, actor/time, audit/outbox; изменение версии делает зависимые preview/eval устаревшими. Preview фиксирует точный текст, версии и маршрут, позволяет повтор/замедление/оценку, а недоступность сообщает как ошибку и сохраняет draft. Текстовый путь и доступность UI обязательны.

**Never:** Не использовать production credentials, реальный клиентский маршрут, публикацию или Business Action. Не помещать секреты в конфигурацию, transcript, аудиометаданные, лог или Release Manifest. Синтетическое аудио не доказывает телефонное качество или понимание клиента. Provider DTO не входят в домен; не строить собственный STT/TTS или универсальную media-платформу.

## I/O & Edge-Case Matrix

| Сценарий | Вход / состояние | Результат | Защита |
|---|---|---|---|
| Выбор и сохранение | `phone`, совместимый язык, разрешённый профиль/стиль/темп | Новая версия профиля и словаря | Tenant/RLS, CAS, audit |
| Неизвестный профиль или несовместимость | Чужой язык, неподтверждённый профиль, неверный темп | Не сохранять; показать точную причину | Fail-closed |
| Критическая сущность | VIN/OEM и т. п. в словаре/тесте | Требовать структурированный readback; human phone-quality gate остаётся открытым | Никакого auto-pass |
| Классификация и тестовые данные | Новая словарная запись/preview без явного выбора и подтверждения вымышленных данных | Не сохранять и не синтезировать | Нет ложного утверждения об автоматической PII-проверке |
| Preview | Разрешённый synthetic route и текст | Аудио + текст, версии, маршрут, повтор/замедление/оценка | Без реального клиента |
| Ошибка preview | Нет capability, истечение или сбой | Честная ошибка, draft остаётся; повтор/другой профиль/человек | Нет ложного успеха |
| Истечение 24 часов | Receipt и связанная оценка достигли срока | Автоматически удалить обе записи и текст; оценку отклонить | Короткое хранение, RLS чтения |

</frozen-after-approval>

## Code Map

- `internal/employee/domain/model.go`, `domain/pack.go`, `packs/catalog.go` — Brief языки/каналы, Pack pin и допустимость стадии; не превращать Pack policy в произвольный исполняемый текст.
- `internal/employee/{api,app,port,store}/behavior.go`, `migrations/employee/00005_behavior.sql` — образец версий, CAS, RLS, audit/outbox и scope; голос хранить отдельным контрактом, не в Behavior prompt.
- `internal/transport/http/{server,employee,behavior}.go`, `contracts/openapi/staffy.yaml`, `web/templates/employee.html` — маршруты, схемы, редактор и доступное отображение статусов/preview.
- `tests/integration/employee_test.go`, `internal/transport/http/employee_test.go`, `tests/journeys/` — проверять tenant-isolation, stale CAS, ошибки capability и управление с клавиатуры.

## Tasks & Acceptance

**Execution:**
- [x] `internal/employee/{domain,api,app,port,store}/voice.go` — модель, каталог, валидация, версии, preview и маршрут через узкий TTS-порт.
- [x] `migrations/employee/` — неизменяемые tenant-scoped версии и audit/outbox без секретов; безопасный rollback.
- [x] `internal/transport/http/`, `contracts/openapi/staffy.yaml`, `web/` — редактор, словарь, статусы и synthetic preview с текстовым альтернативным путём.
- [x] `tests/` — все строки матрицы, человеческий release gate, stale preview/eval, RLS/CAS, 320 px и клавиатура.
- [x] `domain/voice.go`, `http/voice.go`, `web/templates/employee.html` — явная классификация каждой записи, подтверждение вымышленных данных, точная проверка readback и совместимые селекторы.
- [x] `migrations/employee/00008_voice_retention.sql`, `store/voice.go`, `cmd/staffy-voice-retention/` — 24-часовой TTL, отдельная минимально привилегированная очистка без tenant-leak и отказ в оценке истёкшего receipt.
- [x] `tests/` — отказ при пустой классификации/неподтверждённых данных, разделители readback, истечение и физическое удаление receipt с assessment.

**Acceptance Criteria:**
- Given новая версия голоса/словаря, when она сохранена, then прежняя версия и её результаты неизменны, а зависимые результаты обозначены точной версией или `stale`.
- Given preview без подтверждённого synthetic capability, when его запрашивают, then аудио не подменяется успехом и настройка остаётся draft.
- Given критическая сущность, when тестируют произношение, then фонетическая запись не закрывает readback и человеческую проверку телефонного качества.
- Given новая словарная запись или preview без явной классификации и подтверждения вымышленных данных, when запрос отправлен, then сохранение/синтез отклоняется.
- Given receipt старше 24 часов, when пытаются сохранить оценку, then оценка недоступна; when обслуживающий процесс выполняет следующий цикл, then receipt и связанный assessment физически удаляются; audit/outbox не раскрывает текст.

## Implementation Notes

- Review-loop уточнения реализованы без изменения frozen intent: новая словарная запись требует явный `critical_kind` (`none` либо конкретный kind) и подтверждение вымышленных данных; каждый preview требует отдельное подтверждение. Автоматического распознавания персональных данных нет. Readback сравнивается с исходным написанием целиком, включая значимые пробелы и разделители; язык выводится из выбранного профиля, отдельного несовместимого селектора нет.
- 24-часовой срок применяется в SQL при показе оценок и в момент оценки receipt. Физическая очистка вынесена в `cmd/staffy-voice-retention` с отдельным URL, стартовым запуском и интервалом 1 минута. Миграция `00008_voice_retention.sql` создаёт NOLOGIN/NOBYPASSRLS роль `staffy_voice_maintenance` с EXECUTE только узкой SECURITY DEFINER функции; `staffy_app` не может её вызвать и не получает DELETE. Оператор должен отдельно создать LOGIN роль/секрет, дать ей право `SET ROLE staffy_voice_maintenance` и запустить процесс с `STAFFY_VOICE_MAINTENANCE_DATABASE_URL`. Пока процесс не развёрнут, физическое удаление не происходит; при его непрерывной успешной работе верхняя граница хранения — около 24 часов + 1 минута + время выполнения, а при остановке/ошибке граница не гарантирована. Текст не копируется в audit/outbox.
- Проверки уточнения и полный регрессионный набор прошли на текущем дереве: `go test ./...`, `go vet ./...`, полный PostgreSQL integration suite с реальным offline WAV, отказом в просроченной оценке, запретом purge для `staffy_app` и физическим удалением receipt/assessment, а также Playwright 10/10 при 320 px и клавиатуре.

- Только development runtime (`STAFFY_ENV=development`) может подключить отдельный eSpeak NG через `STAFFY_DEV_ESPEAK_NG_BIN`, `STAFFY_DEV_ESPEAK_DATA_PATH` и `STAFFY_DEV_ESPEAK_LIBRARY_PATH`. Без всех корректных путей каталог показывает `unavailable`, preview возвращает ошибку и draft сохраняется. Бинарник и данные голоса не включены в репозиторий.
- Официальная лицензия eSpeak NG — GPL-3.0-or-later; текущий маршрут предназначен только для локальной синтетической проверки. Коммерческое/production использование и телефонное качество остаются без допуска. Adapter передаёт короткий текст через stdin отдельному процессу без shell и без переменных окружения приложения, ограничивает время и размер аудио и принимает только WAV.
- Для сохранённой словарной записи preview требует точную тестовую фразу, заменяет единственное исходное написание на нормализованное произношение и показывает оба текста. Неподдерживаемая запись или неточная фраза блокируется. Критические сущности задают сегменты точного readback, обязательное подтверждение клиента, передачу человеку при несовпадении и открытый человеческий gate телефонного качества.
- Успешный synthetic preview оставляет неизменяемый tenant-scoped receipt с точными voice/dictionary/draft версиями, исходным и синтезируемым текстом, маршрутом, темпом, актором и временем. Оценка записывается отдельно с audit/outbox, а после изменения draft/voice версия показывается как `stale`; синтетическое «Одобрено» не закрывает телефонный gate.
- В текущем окружении `go test ./...` (включая реальный WAV unit test), `go vet ./...`, `git diff --check` и полный PostgreSQL integration suite прошли на выделенной временной базе. Browser journey для голоса прошёл с Playwright Chromium после установки браузера в `/tmp/staffy-playwright`.

## Spec Change Log

- 2026-09-27, review loop 1: по подтверждению владельца уточнены ручная обязательная классификация, только вымышленные тестовые данные и 24-часовое физическое удаление preview/assessment. Уточнена точность readback (значимые разделители нельзя терять). Сохранять работающие tenant/RLS/CAS, offline test-TTS, честный phone-quality gate, текущий UI и пройденные проверки; не возвращаться к известному плохому состоянию с неограниченным хранением и неявной классификацией.

## Review Triage Log

| Finding | Verdict / route | Evidence |
|---|---|---|
| Blind 1 · critical kind optional | medium / intent_gap | `ValidateVoice` требует readback только при явном `CriticalKind`; произвольный VIN можно пометить обычным словом. Универсального надёжного распознавания всех Pack-critical сущностей в текущем контракте нет; требуется правило классификации до выпуска. |
| Blind 2 · stale Brief preview | high / patch | До исправления `PreviewVoice` синтезировал аудио при несовпадении версии черновика. Теперь устаревшая версия отклоняется до синтеза. |
| Blind 3 · concurrent Brief edit | high / patch | До исправления `SaveVoicePreview` проверял только voice version. Теперь транзакция блокирует черновик и сверяет draft version. |
| Blind 4 · overlapping spellings | false / reject | `ResolveSynthesisText` выбирает только точную уникальную тестовую фразу; произвольное вхождение другого написания вне её отклоняется, поэтому конкурирующего применения правил в этом preview нет. |
| Blind 5 · scope unused | medium / defer | `Scope` хранится, но не влияет на выбор произношения; сегодня выбор задаёт точная тестовая фраза. При подключении runtime потребуется формальная семантика scope и тесты. |
| Blind 6 · punctuation in readback | medium / bad_spec | `foldEntity` убирает значимые разделители суммы, даты и адреса. Нынешняя проверка может принять искажённый readback; нужна точная структурная проверка. |
| Blind 7 · PII in preview text | high / intent_gap | `safePreviewText` не определяет персональные данные, а `source_text` сохраняется в БД. Сообщение теперь не заявляет автоматическую защиту, но нужен отдельный выбор допустимых тестовых данных. |
| Blind 8 · retention | medium / intent_gap | `employee.voice_previews` не имеет срока хранения или механизма очистки; решение связано с политикой хранения тестового текста. |
| Blind 9 · audio provenance | medium / defer | Receipt связывает текст и версии, но не содержит digest WAV и версии адаптера; до release-решений нужна воспроизводимая привязка услышанного аудио. |
| Blind 10 · warm style | medium / patch | Каталог допускает только neutral, а форма предлагала warm; лишний пункт удалён. |
| Blind 11 · independent selectors | medium / patch | При нескольких языках Brief можно выбрать несовместимые профиль и язык; сервер отклоняет сохранение, но редактор должен исключать эту пару заранее. |
| Blind 12 · approval before playback | medium / patch | Кнопка «Одобрено» активировалась сразу после ответа API; теперь она доступна только после события окончания воспроизведения. |
| Blind 13 · no entry ID in receipt | false / reject | Точный текст receipt и сохранённая версия словаря позволяют однозначно восстановить запись по уникальной тестовой фразе; общего одобрения словаря система не выдаёт. |
| Edge 1 · in-flight preview race | high / patch | Старый ответ мог восстановить кнопки после изменения текста/скорости; теперь ответы сверяются с активным запросом и текущим вводом. |
| Edge 2 · empty folded readback | medium / patch | Пунктуация/emoji могли нормализоваться в пустую строку; пустое нормализованное значение теперь отклоняется. |
| Gap 1 · warm style | medium / patch | Независимое подтверждение Blind 10: видимый warm гарантированно приводил к отказу валидации; пункт удалён. |
| R2 Blind 1 · development-only eSpeak | false / reject | `run` завершает работу для любого `STAFFY_ENV`, кроме `development`, до создания TTS; production route недостижим в текущем executable. |
| R2 Blind 2 · audio provenance | medium / defer | Receipt не содержит digest WAV и версии адаптера; это понадобится до использования synthetic assessment в release-gate, который Story 1.7 оставляет закрытым. |
| R2 Blind 3 · scope не применяется | medium / defer | Scope пока метаданные; точная тестовая фраза однозначно выбирает правило preview. Семантика runtime scope вынесена в deferred work. |
| R2 Blind/Edge 4 · synthesis_text > 500 | medium / patch | Подстановка могла превысить SQL-limit после успешного synth; теперь длина проверяется до вызова TTS. |
| R2 Blind 5 · subprocess catalog probe | low / reject | В development-каталоге только два профиля; кэш добавил бы состояние и инвалидирование ради недоказанного пользовательского вреда. |
| R2 Blind 6 · pre-existing maintenance role | high / patch | Миграция теперь принудительно задаёт безопасные атрибуты, запрещает членство роли в других ролях и отзывает прежние table/sequence/function grants до выдачи единственного EXECUTE. |
| R2 Blind 7 · assessment employee FK | high / patch | Составной FK теперь связывает tenant, employee и preview, исключая внутренне противоречивую строку. |
| R2 Blind 8 · application clock retention | medium / patch | Время receipt и проверка TTL теперь используют `clock_timestamp()` PostgreSQL; часы приложения не управляют сроком удаления. |
| R2 Blind/Edge 9 · OpenAPI responses | medium / patch | Для preview/assessment добавлены реальные 400/404 ответы. |
| R2 Blind 10 · permissive request schemas | low / patch | Voice command/config/pronunciation/readback и inline preview/assessment requests теперь запрещают неизвестные поля. |
| R2 Blind 11 · approval after seek | medium / patch | Кнопка активируется только после `ended` и теперь является явным утверждением человека, что образец прослушан полностью; это attestation, не автоматическое доказательство. |
| R2 Blind 12 · Graphify artifacts | false / reject | AGENTS.md требует `graphify update .` после кода, а репозиторий уже отслеживает canonical outputs/cache; файлы не входят в продуктовый бинарник. |
| R2 Blind 13 · sprint status | false / reject | Во время BMAD review sprint остаётся `in-progress`; он синхронизируется в `review` только после успешного завершения шага. |
| R2 Blind 14 · verification wording | medium / patch | Implementation Notes обновлены до однозначного полного набора команд на текущем дереве. |
| R2 Edge 4 · assessment attempt deletion claim | medium / patch | AC уточнён: попытка оценки отклоняется, физическое удаление выполняет следующий цикл отдельного retention-процесса. |
| R2 Gap 1 · retention runner untested | high / patch | Добавлены unit-проверки немедленного первого запуска, активации роли, вызова узкой функции и commit; integration отдельно проверяет SQL/RLS и физическое удаление. |
| R2 Gap 2 · CI requires eSpeak | high / patch | Journey fixture теперь выдаёт детерминированный синтетический WAV без внешней установки; реальный eSpeak остаётся в Go/integration проверках. |

## Design Notes

Один профиль и словарь на версию Employee; выбор провайдера — только за адаптером. Статус capability относится к конкретному профилю и маршруту, а не ко всей функции «голос».

## Verification

**Commands:** `go test ./...`; `go vet ./...`; `go test -tags=integration ./tests/integration/...`; `npm test --prefix tests/journeys`; `git diff --check`.
