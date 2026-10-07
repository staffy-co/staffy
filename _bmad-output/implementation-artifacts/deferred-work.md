- source_spec: `spec-1-2-tenant-membership.md`
  summary: Перед публичным развёртыванием проверить, не записывает ли reverse proxy секрет одноразового приглашения из query string.
  evidence: Staffy выставляет no-store и no-referrer, но конфигурация внешнего proxy и его access-log в репозитории отсутствует; проверка фактических логов развёртывания разрешит риск.

- source_spec: `spec-1-4-versioned-domain-pack.md`
  summary: При следующем изменении каталога зафиксировать политику сохранения старых версий manifest, на которые уже указывают Employee draft.
  evidence: Сейчас все закреплённые версии присутствуют в бинарном каталоге и возвращаются неизменяемыми копиями. Риск возникнет, только если будущий релиз удалит прежнюю версию; проверка diff каталога и pinned-версий перед таким релизом подтвердит или опровергнет его.

- source_spec: `spec-1-5-knowledge-sources.md`
  summary: Завершить Story 1.5 безопасным приёмом сайтов и файлов, malware/content scanning, extraction, index/eval и retention-связанным удалением payload.
  evidence: Первый срез ограничен вводимыми текстом FAQ, регламентами и примерами диалога; реальные URL и файлы требуют выбора и проверки scanner, storage и egress по AG-4. Перед следующим срезом решить допустимые форматы (включая необходимость DOCX) и правила сетевого доступа; не считать всю Story 1.5 завершённой раньше.

- source_spec: `spec-1-5-knowledge-sources.md`
  summary: Определить ротацию и уничтожение ключей HMAC источников знаний в общей политике хранения и удаления.
  evidence: Первый срез создаёт отдельный случайный ключ для контрольных сумм каждого Source и не отдаёт его наружу, но процедуры смены ключа и криптографического удаления payload должны быть согласованы с будущим retention-срезом Story 1.5.

- source_spec: `_bmad-output/implementation-artifacts/spec-1-7-voice-and-pronunciation.md`
  summary: До подключения словаря к рабочему voice runtime определить формальную семантику scope и приоритет пересекающихся правил произношения.
  evidence: В Story 1.7 scope сохраняется как метаданные настройки, а synthetic preview применяет правило только по уникальной точной тестовой фразе; runtime-выбор по контексту ещё не реализуется.

- source_spec: `_bmad-output/implementation-artifacts/spec-1-7-voice-and-pronunciation.md`
  summary: До использования оценок голоса в release-gate связать receipt с digest аудио, версией адаптера и полным набором параметров синтеза.
  evidence: Текущий локальный preview подтверждает текст, версии конфигурации, маршрут и slow, но не позволяет доказать, что позднее проверяется тот же WAV; синтетическая оценка сейчас намеренно не открывает телефонный gate.

- source_spec: `_bmad-output/implementation-artifacts/spec-1-8-integrations-catalog.md`
  summary: До подключения реальных провайдеров пройти AG-4: выбрать конкретный SecretStore (внешнее хранилище вместо локальных HMAC-seal), решить ротацию и физическое уничтожение отозванных seals и их ключей.
  evidence: Первый срез запечатывает секреты HMAC-SHA256 с случайным ключом на Connection в `connector.secret_seals` и отзывает ссылки, но физического уничтожения и внешнего SecretStore нет; production-среда закрыта fail-closed доменом и SQL CHECK.

- source_spec: `_bmad-output/implementation-artifacts/spec-1-8-integrations-catalog.md`
  summary: Пометка зависимых capability evidence как `stale` при ротации и отзыве секрета реализуется вместе с capability verification Story 1.9.
  evidence: Текущий срез сбрасывает состояния authentication/transport в pending и не заявляет capabilities; `connected/unverified` — честное терминальное состояние среза, evidence-таблица появится в Story 1.9.
  resolution: closed in 1.9 — derivation `CapStateStale` с конкретной причиной (internal/connector/domain/capability.go, staleReason); подтверждено epic-1-retrospective (2026-10-08).

- source_spec: `_bmad-output/implementation-artifacts/spec-1-8-integrations-catalog.md`
  summary: Восстановление фокуса в конкретное поле после ошибок форм интеграций — вместе с общим accessibility-проходом UI.
  evidence: Ошибки форм сейчас возвращаются redirect-ом с role=alert и текстом; цель ≥44×44 px и клавиатурная навигация проверены journey-тестом, но точечный focus-management в поле не реализован.

- source_spec: `spec-1-8-integrations-catalog.md`
  summary: Добавить конкурентный (параллельные горутины) integration-тест CAS для connect/rotate: доказать, что одновременные команды дают ровно один seal и один конфликт.
  evidence: Последовательный stale-CAS тест и конструкция `select … for update` + сверка версии в транзакции исключают двойное запечатывание; параллельный тест не написан из-за фиксированной последовательности ID в текущей фикстуре (R2 Gap 4).
  resolution: closed in 1.9 — TestConcurrentConnectRotationsProduceExactlyOneSeal (tests/integration/connector_test.go); подтверждено epic-1-retrospective (2026-10-08).

- source_spec: `spec-1-8-integrations-catalog.md`
  summary: Определить идемпотентную семантику повторов connect/rotate/revoke после сетевого сбоя, чтобы собственный retry пользователя не читался как «изменено другим участником».
  evidence: OpenAPI контракт документирует отсутствие Idempotency-Key на этих путях; CAS и блокировка строки исключают дубликаты, но сообщение о конфликте после успешного коммита вводит в заблуждение (R2 Gap 5).
  resolution: closed in 1.9 — replay сохранённого исхода по tenant-scoped ключу + payload hash до стражей (internal/connector/app/service.go, LookupCommand); подтверждено epic-1-retrospective (2026-10-08).

- source_spec: `spec-1-10-data-perimeter.md`
  summary: Закрыть окно потери квитанции между коммитом purge-исполнителя и сохранением deletion-запроса: записывать запрос (in-progress) до исполнения шагов, исходы и квитанцию — после.
  evidence: Сбой процесса после коммита purge-функции, но до SaveDeletion оставляет удалённые строки без квитанции первого выполнения; повтор того же ключа ре-исполняется data-идемпотентно и честно пишет счётчик 0, но доказательство первого выполнения теряется (finding R1). Требует расширения схемы (статус запроса) и нового порядка оркестрации.

- source_spec: `spec-1-10-data-perimeter.md`
  summary: Скрыть управляющие формы карты данных от участников только с audit-скоупом (сейчас сервер честно отклоняет POST, но формы видны).
  evidence: viewScope вычисляет canManage, но DataView его отбрасывает (_ = canManage); отказ fail-closed на сервере соответствует спеке («просмотр без управления»), UX-уточнение — вместе с общим accessibility-проходом UI.

- source_spec: `spec-1-11-entitlement-usage.md`
  summary: Выровнять маппинг JSON-ошибок privacy-модуля с entitlement-паттерном: сбои декода сейчас отдаются как invalid_idempotency_key («Обновите форму и повторите попытку») — причина неточная для битого тела запроса.
  evidence: entitlement после R1 возвращает честные content_type/body rule-ошибки; privacy.go использует privacyDomain.ErrInvalidIdempotency для всех сбоев декода (три вызова на путь); правка механическая, требует аналогичных transport-тестов (finding R1 по 1-11).

- source_spec: `spec-1-11-entitlement-usage.md`
  summary: Заменить тихие fallback-ы числовых полей форм (formInt: warning_threshold_percent→80, freshness_minutes→1440) на честные ошибки парсинга — сейчас мусорный ввод молча публикует значение по умолчанию.
  evidence: Опубликованное значение видно на странице после сохранения, поэтому подмена не скрыта, но не называет саму ошибку ввода; price-поля после R1 честно отклоняются — тот же паттерн нужен для formInt (finding R1 по 1-11, мульти-модульная правка).

- source_spec: `spec-1-13-operating-profile.md`
  summary: Подключить провайдер step-up (реальную повторную аутентификацию) для утверждения Operating Profile: сейчас шаг требует Principal.StepUpVerified, но провайдера в системе нет — в продакшн-бинар никто не проходит step-up и утверждение честно заблокировано.
  evidence: Тот же класс блокировки, что у membership invite/change/revoke (identity ErrStepUpRequired); фикстура journeys использует резолвер с переключаемым флагом (POST /fixture/step-up — только тестовый контур), интеграционные тесты — principals с флагом; поверхность готова к подключению провайдера без изменения домена.
