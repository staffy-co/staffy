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

- source_spec: `spec-1-12-release-manifest.md`
  summary: Выровнять выбор «последнего манифеста» в ReleaseView (surface 1-12) с детерминированным паттерном 1-13 (максимальный config_version): сейчас records[0] по сортировке sealed_at desc, manifest_id desc — под фиксированным clock это coin flip (тот же корень, что M2 review R1 по 1-13).
  evidence: В продакшене sealed_at практически уникален, риск латентный; профильная поверхность уже исправлена (app/profile.go — выбор по config_version), паттерн стоит вынести в общий хелпер и применить к release при случае (замечание R2 по 1-13, P4).

- source_spec: `spec-1-14-readiness-publication.md`
  summary: Перечитать кросс-модульные факты (entitlement admission, текущие connections) ВНУТРИ транзакции публикации: сейчас они читаются непосредственно перед командой под той же верифицированной скоуп-проверкой, но технически вне tx — миллисекундное окно TOCTOU между чтением и коммитом остаётся.
  evidence: In-module факты (профиль, манифест, инстансы, история) перечитываются в транзакции (store/readiness.go), порты фактов (WithEntitlementFacts/connectorFacts) не предлагают tx-scoped чтение — архитектурное ограничение факт-портов; комментарий в порте честно документирует границу (замечание m4 review R1 по 1-14).

- source_spec: `spec-1-14-readiness-publication.md`
  summary: Точный per-route маппинг capability→маршрут для fail-closed блокировки маршрута: сейчас элемент capabilities требует «хотя бы одно действующее подключение с verified capability» (aggregate), потому что ConnectionFacts не несёт связи маршрут↔capability.
  evidence: Решение DEVLOG 37-dev/R1-m5: непроверенные остатки — честные ноты элемента, полная дисциплина — gate AG-4 стадий от controlled_real_customer; точная блокировка требует расширения факт-порта связью маршрута (Epic 2, когда появится направление трафика).

- source_spec: `spec-1-14-readiness-publication.md`
  summary: Спека: уточнить формулировку «переход в pass/fail требует … evidence-ссылок и hash» — прочитанная буквально она покрывает и fail-переходы, реализация (по Never-клаузе и edge-кейсам, упоминающим только pass) принимает payload только с PASS.
  evidence: Замечание review R1 по 1-14 (confidence note); механика корректна по Never-клаузе, уточнение — к следующей версии спеки.

- source_spec: `spec-1-14-readiness-publication.md`
  summary: Спека содержит два прочтения strict-конъюнкции для controlled+ стадий: «независимые маршруты живут / блокировка маршрута не блокирует кандидата целиком» vs «ready только если все обязательные элементы... дают pass» + AC-5 «все обязательные проверки стадии имеют действующий PASS». Реализация (R1-M3) держит элементную конъюнкцию: смешанный кандидат (web_chat чист, phone без OQ-12) не публикует controlled+ пока ВСЕ маршруты не пройдут гейты.
  evidence: Замечание R2/N4: прочтение защитимо (gates — обязательный элемент, AC-5 явен), но замороженный текст допускает оба; решение владельца — к следующей версии спеки (Epic 2 enforcement уточнит).

- source_spec: `spec-2-1-channel-event-case.md`
  summary: Реальные провайдерские адаптеры и схемы подписи для ingress (AG-4): сейчас единственный верификатор — синтетический тестового контура (staffy-synthetic-telephony в sandbox/test); все прочие коннекторы fail-closed («коннектор без верификатора»), продакшн-трафик честно невозможен.
  evidence: internal/adapters/ingresssyn — порт SourceVerifier с одной реализацией; ingresssyn-тест покрывает production_forbidden и connector-without-verifier ветки поверх фейкового факт-порта; интеграционное покрытие этих веток структурно недостижимо — каталог admits только один креируемый коннектор.

- source_spec: `spec-2-1-channel-event-case.md`
  summary: Фоновые outbox-воркеры (leased jobs) и реальная доставка connector-ack провайдеру: сейчас DrainOutbox — идемпотентная команда с append-only исходами «deferred» (нет ни воркеров, ни адаптеров — AG-4); транспорта у команды нет (только сервис-метод, интеграционно покрыт).
  evidence: cases.DrainOutbox в api.Service; исходы deferred с честной причиной; спека честно откладывает фоновую доставку; перенесено сюда из соображения «команда без триггера — мёртвый код в продакшн-контуре».

- source_spec: `spec-2-1-channel-event-case.md`
  summary: Транспортные записи ingress_rejections для нераскодируемых тел (400 на уровне декода): строка отказа не создаётся, если body не распарсилось или tenant-claim невалиден — нет честного скоупа для записи; correlation ID ответа остаётся единственной связью.
  evidence: transport cases.go — decode-фейлы возвращают 400 problem без записи; app recordRejection честно пропускает запись при невалидном скоупе; потенциальный enhancement — отдельная global-scope таблица для decode-фейлов при появлении реального трафика.

- source_spec: `spec-2-1-channel-event-case.md`
  summary: Проверка provider-клейма против ConnectorID коннектора и cross-check подписи в ingress-контракте: сейчас provider — строка происхождения события (материальная часть идемпотентности), совпадение с коннектором проверяется неявно (верификатор знает единственный коннектора контура).
  evidence: Envelope.Provider валидируется форматом ≤64; материальный ключ (tenant, provider, provider_event_id); реальная дисциплина — с провайдерскими адаптерами (AG-4), когда подпись и sender identity станут проверяемыми.

- source_spec: `spec-2-1-channel-event-case.md`
  summary: Load envelope по AG-1/AG-4-доказанным значениям и bounded-очередь вместо честного bounded-отказа: сейчас фиксированный консервативный словарь (256К/128К/1000 in-flight/окно 1ч) без конфигурации на Tenant, превышение — честный отказ envelope_exceeded со счётчиком.
  evidence: cases/domain dictionaries.go FixedLoadEnvelope; спека Never-клауза прямо запрещает выдуманные значения; замена словаря доказанными метриками — после измерительного фундамента (Epic 6).

- source_spec: `spec-2-1-channel-event-case.md`
  summary: Legal-basis enforcement против data perimeter для pre-case sessions (AC-8): запись сессии с объявленной capture policy хранится, проверка основания отложена до связки privacy (privacy не входит в DAG case — архитектура).
  evidence: cases.pre_case_sessions.capture_policy CHECK-словарь (metadata_only|sealed_payload); спека честно откладывает enforcement; privacy-классы case-operations/case-content зарегистрированы.

- source_spec: `spec-2-1-channel-event-case.md`
  summary: Signed TTL-ссылки на запечатанный payload (вместо прямой серверной авторизации review/audit) и текстовые коннекторы end-to-end (chat-адаптер для восьми текстовых каналов; словарь и контракт ingress уже готовы — channel_mismatch честно拒绝ывает текстовые события на SIP/telephony).
  evidence: ReadPayload — серверная авторизация review/audit + HMAC-проверка печати; интеграционный тест TestIngestRefusesEveryLadderStep покрывает channel_mismatch; спека откладывает оба пункта явно.

- source_spec: `spec-2-1-channel-event-case.md`
  summary: Напряжение замороженных клозул по записям отказов верификации источника: Boundaries требует «без скоупа никакого SQL» (скоуп только из верифицированного connection), а матрица требует строку ingress_rejections для unknown_connection — реализация пишет отказ под заявленным (непроверенным) скоупом с ограничениями: тенант заявки обязан существовать (RLS-чит membership-фактов) и фиксированный словарный лимит 100 записей (connection×kind×окно 1ч).
  evidence: app/service.go recordRejection (trusted=false путь + claimedTenantIsReal + CountRejections); DEVLOG 43-review finding R1-M4; решение владельца (тенант-нейтральная зона / полный запрет записи / подтверждение ограниченного варианта) — к следующей версии спеки.

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: Confirmer-типы client и owning_system без продюсеров: механика подтверждений (append-only, привязка к точной версии, evidence-словарь) готова к их приёму, но исходящих диалогов (client) и write-path коннекторов (owning_system) в контуре нет — продюсер среза только operator_relay.
  evidence: domain/work.go ConfirmerTypes словарь + GuardConfirmFactVersion; миграция 00002 CHECK confirmer_type IN ('client','owning_system','operator_relay'); контракт ConfirmFactRequest enum [operator_relay]; интеграционный тест отказывает client-подтверждение.

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: Fact source-типы connector и system зарезервированы без продюсеров: write-path коннекторы (прямые записи внешних систем как источники фактов) прибудут со своими stories; домен честно отказывает их при записи.
  evidence: domain/work.go FactSourceTypes/FactSourceProducers; миграция 00002 CHECK source_type IN ('channel_event','operator','connector','system'); контракт FactVersionRecord enum с пометкой reserved.

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: Authority rules / Owning System / State-Authority Matrix: авто-выбора значения факта нет как класса (разрешение конфликта — подтверждение человеком); выбор значения по авторитетному источнику — архитектурный артефакт будущих stories.
  evidence: domain/work.go DeriveFactStatus — производная без авто-выбора; спека Never-клауза прямо запрещает выдуманные authority rules; Design Notes фиксируют отложение.

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: Extraction-источники фактов (AI в контуре нет): факты записывает оператор; автоматическое извлечение материальных ключей/значений из payload — будущая AI-story с собственными gate.
  evidence: RecordFact — операторская команда с полным provenance; IngestEvent не создаёт фактов (только пин+version 1+линк первого события); спека Never-клауза «не создавать факты автоматически из событий».

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: Автосвязь событий с рабочими элементами (2-3 — человек связывает каналы): привязка события к WI — только явная команда LinkEvent/CreateWorkItem; слабых совпадений нет как класса.
  evidence: GuardLinkEvent + LinkEvent/транспорт; интеграционный TestSessionContinuationGrowsTimelineWithoutNewWorkItems — продолжение сессии не создаёт WI/линк автоматически.

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: Закрытие кейса и Conversation (lifecycle за пределами среза): завершение WI никогда не закрывает диалог/кейс (case остаётся open); правила закрытия — последующие stories контура.
  evidence: интеграционные тесты: переходы в waiting/resolved/split/merged не меняют case.state='open' и conversation.state='active'; домен лестницы без кейс-переходов.

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: Merge-форма оператора сливает один источник за команду (API поддерживает список источников): многоисточниковое слияние в одной команде — расширение формы WCC по мере реальных сценариев; срез честно покрывает ≥1 источник (матрица).
  evidence: transport cases_work.go mergeWorkItemForm — один select-источник; app MergeCommand.Sources — список; интеграционный тест покрывает одноисточниковое слияние полного цикла.

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: Work item версии и dispute имеют НЕСКОЛЬКО advisory-локов вместо row FOR UPDATE: SELECT FOR UPDATE требует привилегию UPDATE, которую append-only гранты (INSERT/SELECT) сознательно не выдают; якоря сериализуются advisory-локами в детерминированном порядке (прецедент AcceptEvent) — материальная безопасность идентична, но гранулярность ниже строковой.
  evidence: store/work.go ExecuteWorkCommand — lockKeys sorted + mapWorkInsertError; комментарий currentForUpdate фиксирует разрешение напряжения «FOR UPDATE якоря» против «грантов INSERT/SELECT только»; спека Boundaries требует оба — конфликт разрешён в пользу append-only (суть дисциплины).

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: N+1 чтения производных представлений: ReadWorkItemCurrents — запрос JOIN LATERAL на каждый WI, ReadFactsOfCase — два запроса на каждый факт (якорь + версии). Для среза допустимо (кейс стартует с 1-2 элементами), с возрастом кейса чтение деградирует линейно; пакетные чтения одной CASE WHEN-проецией — оптимизация будущих stories с бенчмарком.
  evidence: review R1 m10 (Task 25-r1); store/work.go ReadWorkItemCurrents/ReadFactsOfCase; интеграционные тесты не измеряют план чтения.

- source_spec: `spec-2-2-work-items-facts-provenance.md`
  summary: Coverage локов после фиксов R1: унифицированная пара факт-ключей (material key + fact id) закрывает все пути записи фактов (record/confirm/rescope/merge — факты источника в плане локов merge), record_fact дополнительно держит work-лок скоупа с терминальной проверкой под локом; ОСТАЮЩИЙСЯ класс — три+ одновременных писателя одного факта, где merge ре-скоупит факт, спланированный ДО чужой записи (merge не держит факт-локи фактов, появившихся после планирования) — закрыт backstop-маппингом fact_versions_pkey → ErrStaleVersion (честный 409-stale, никогда сырой 500). Полная защита от этого класса = case-уровневый advisory-лок на merge (сериализация всех команд кейса) — отложена до реальных нагрузочных сценариев.
  evidence: review R1 M1/M2 (Task 25-r1) + фиксы; store/work.go lock-план и mapWorkInsertError; конкурентные интеграционные тесты TestRacedFactWritersSerializeOnTheUnifiedFactLocks/TestMergeConcurrentRecordNeverLeaksRawConflicts; this-запись честности.

- source_spec: `spec-2-3-identity-verification-channel-binding.md`
  summary: N+1 чтения карточек дублей и реверсий: readDuplicatesOfCaseTx делает по два запроса на кандидата (версии факта + резолюция + канал) и readBindingsTouchingCaseTx читает ВСЕ реверсии тенанта. Для среза допустимо (кейс содержит 1-3 кандидата), с ростом числа кейсов чтение деградирует линейно; пакетные чтения одной проекцией — оптимизация будущих stories с бенчмарком (прецедент N+1 фактов 2-2).
  evidence: review R1 m8 (Task 28-r1); store/verification.go readDuplicatesOfCaseTx/readBindingsTouchingCaseTx; интеграционные тесты не измеряют план чтения.
