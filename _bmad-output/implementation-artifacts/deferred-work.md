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

- source_spec: `_bmad-output/implementation-artifacts/spec-1-8-integrations-catalog.md`
  summary: Восстановление фокуса в конкретное поле после ошибок форм интеграций — вместе с общим accessibility-проходом UI.
  evidence: Ошибки форм сейчас возвращаются redirect-ом с role=alert и текстом; цель ≥44×44 px и клавиатурная навигация проверены journey-тестом, но точечный focus-management в поле не реализован.
