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

- source_spec: `spec-2-3-identity-verification-channel-binding.md`
  summary: Порог step-up считается по записям ВСЕГО кейса, а замороженный Intent говорит «на разговор» (≥3 failed|ambiguous per conversation в окне 1ч). В срезе поведенчески эквивалентно (кейс владеет ровно одним разговором — новые сессии рождают новые кейсы, связанные разговоры принадлежат чужим кейсам), направление консервативнее (кейс-уровень блокирует раньше). Фильтр окна по conversation_id записи — честная правка будущих stories, когда кейс станет многоразговорным (после срезов 2-4+).
  evidence: review R2 n1 (Task 29-r2); domain/verification.go DeriveAssuranceState — StepUpFailures без фильтра conversation; R1 m3 сброшен без трассировки — эта запись восстанавливает честность трассировки.

- source_spec: `spec-2-4-response-owner-inbox.md`
  summary: TOCTOU факт-слов владения (ownerActive/routeAdmitted) читаются app-слоем ДО advisory-лока кейса — окно между перечиткой MembersFacts/RouteFacts и транзакцией стора. Сторова сторона закрыта re-validation под локом (термы/запросы/резолюции — материал cases), но факт-слова приходят снапшотом в параметрах команды; execute-time revalidation drain-а закрывает окно на стороне доставки (устаревший prepared-ответ → stale_needs_review, payload не пересылается). Полное закрытие = перечитка фактов внутри транзакции кейса — перекрёстная зависимость modules (cases-стор звал бы employee/connector-порты) — отложена до реальных сценариев, где окно материально.
  evidence: review R1 m2 (Task 34-r1); app/ownership.go SendClientReply/DrainClientReplies — перечитки фактов до команды и на drain-старте; store/ownership.go execSendClientReply — gate re-validation под локом по cases-материалу, p.OwnerActiveWord снапшотом; ResolveReplyDrain — fresh ownerActive/routeAdmitted на execute-time.

- source_spec: `spec-2-5-conversation-mode-ai-disclosure.md`
  summary: WI-контекст режима — односторонняя трещотка без пути выхода: exit_mode в срезе пишет только scope=conversation (терм понижения диалога), поднятый WI-контекст (например cancellation) переживает любую смену режима диалога и понизить его предложением невозможно (GuardProposeTransition отказывает — «понижение невозможно как класс» верен для обоих скоупов). Направление fail-closed (застрять можно только строже — продажи этого WI заблокированы навсегда), но «единственный путь понижения — exit_mode» материально не выполняется для item-скоупа. Полное закрытие = расширение exit_mode (scope+work_item+authority-лестница для контекста) — механика новой поверхности, спекой не определена.
  evidence: review R1 m1 (Task 48-review-r1); app/mode.go ExitModeCommand без scope; store/mode.go execExitMode — только ModeScopeConversation; живая проба PROBE-WI: cancellation-контекст пережил смену диалога, выхода нет.

- source_spec: `spec-2-5-conversation-mode-ai-disclosure.md`
  summary: Раскрытие на связанном (governing) разговоре: record_disclosure валидирует собственный диалог кейса (modeConversation), а гейт раскрытия ищет записи по (conversation=ответа, channel) — для связанного Y (управляемого кейсом A) записей не существует никогда, продажные/защищённые ответы через Y перманентно заблокированы (fail-closed — запрещает, не разрешает). Мод-гейт governing работает (режим кейса A управляет ответами через Y — материал читается по governing-кейсу). Полное закрытие = разрешить record_disclosure на активно связанный диалог (или запись на Y с material на A) — новая механика связанной поверхности, спекой не определена.
  evidence: review R1 m2 (Task 48-review-r1); app/mode.go modeConversation (conversation.CaseID != command case → отказ); app/ownership.go гейт раскрытия по command.ConversationID; матрица «governing-разговор» — мод-сторона покрыта, disclosure-сторона честно погранична.

- source_spec: `spec-1-13-operating-profile.md`
  summary: Детерминированный порядок audit-строк в TestProfileWriteAuditAndGrants — однострочный фикс: order by occurred_at без tie-breaker недетерминирован при равных occurred_at (вне-срезовый флэйк, блокирует CI-гейт историй); правка order by occurred_at, audit_id даёт стабильный семантичный порядок (audit_id — UUIDv7 реального времени: при коллизии p.Now порядок монотонен по факту генерации).
  evidence: Задокументирован review 2-4 и R2 2-7 (тогда единичные проявления — ниже порога); 2026-10-13 CHECKPOINT 2 story 2-8: push-run 38350534585 и main-run 38351157918 FAIL на одном сообщении «the audit must record the honest outcomes in order: [profile_approved profile_draft_saved]» (profile_test.go:585, order by на строке 572), оба зелёные после rerun-failed-jobs; PR-run 38350679631 того же дерева — success с первой попытки (интермитентность подтверждена); дифф 2-8 не трогает employee-контур (0 строк).

- source_spec: `spec-1-13-operating-profile.md`
  summary: ЗАКРЫТО (2026-10-13, пре-шаг Dev-фазы 2-9, решение владельца из CHECKPOINT 1): флэйк TestProfileWriteAuditAndGrants устранён детерминированным tie-breaker «order by occurred_at, resulting_version». Исходное предложение «order by occurred_at, audit_id» из записи выше было НЕДОСТАТОЧНО и честно откорректировано: id.NewV7 несёт millisecond-префикс + 74 бита crypto-random — порядок внутри одной миллисекунды случаен; releaseClock фиксирован (occurred_at обеих строк идентичны) — audit_id-поряд оставался 50% лотереей, что подтвердил живой bootstrap-прогон с audit_id-tiebreaker (FAIL «[profile_approved profile_draft_saved]» при фиксе в дереве). resulting_version причинно монотонен (draft_saved→1, approved→2), порядок детерминирован; продуктовый код не тронут (0 строк вне теста).
  evidence: bootstrap FAIL с audit_id-фиксом 2026-10-13 (50% живое подтверждение недетерминизма id-порядка); после resulting_version — 8/8 независимых прогонов зелёные + verbose PASS; мотив фиксированного releaseClock — tests/integration/release_test.go (fixedClock 2026-10-08T12:00:00Z); NewV7-механика — internal/platform/id/id.go (UnixMilli + rand.Read).
  resolution: closed in 2.9 пре-шаг — детерминированный tie-breaker resulting_version; разметка epic-2-retrospective (2026-10-18, DEVLOG 102-retro).

- source_spec: `spec-2-6-progressive-intake-policy-outcome.md`
  summary: Гонка данных в тест-харнессе sequenceIDs (tests/integration/tenant_test.go:298) — генератор не goroutine-safe (index++ без мьютекса), TestPolicyTerminalWorkItemAndRaces гонит 4 конкурентных EvaluatePolicy: два горутины могут прочитать один index → одинаковый outcome_id → «duplicate key value violates unique constraint policy_outcomes_pkey». Вне-срезовый флэйк CI (интермитентный, триггерится таймингом раннера). Кандидат на однострочный фикс: мьютекс в sequenceIDs.New (харнесс, продуктовый код не затронут) — решение владельца следующей сессии.
  evidence: 2026-10-13 CHECKPOINT 2 story 2-9: PR-run 38427598831 (head c3f8a24, PR #24) FAIL «racer 2 failed honestly: ERROR: duplicate key value violates unique constraint "policy_outcomes_pkey"» (cases_policy_test.go:844); push-run 38426798763 ТОГО ЖЕ дерева — success с первой попытки; rerun-failed-jobs PR-рана — success; дифф 2-9 не трогает policy-путь и генератор (фикс 2-9 = commitment-гейты).

- source_spec: `spec-2-6-progressive-intake-policy-outcome.md`
  summary: ЗАКРЫТО (2026-10-14, пре-шаг цикла story 2-10, решение владельца «почини флэйк генератора»): гонка данных sequenceIDs устранена — New горутин-безопасен (мьютекс: выдача каждого UUID последовательности ровно один раз; порядок выдачи под конкурентностью произволен — ни один вызывающий не зависит от того, КАКОЕ значение он получил, только от уникальности; guard-зонд закрепляет ровно это). Инвариант закреплён живым зондом TestSequenceIDsConcurrentHandoutStaysUnique (tenant_test.go: 8 горутин × 64 значения, tight-loop — перекрытие детерминировано там, где DB-тайминг policy-теста остаётся лотереей). Мутационная верификация на финальном дереве: снятие Lock/Unlock → зонд краснеет (WARNING: DATA RACE); с фиксом → зелёный (-race, count×10); TestPolicyTerminalWorkItemAndRaces зелёный под -race; полный регресс зелёный (интеграция ×2 TZ UTC/Europe-Moscow, unit 21, vet, build; gofmt файла чист). Продуктовый код не тронут — 0 строк вне tests/integration/tenant_test.go, дифф +57/−0.
  evidence: живое воспроизведение дефекта до фикса (2026-10-14): зонд на исходном генераторе — WARNING: DATA RACE + FAIL за дубликаты выдачи; мутация на финальном дереве (мьютекс снят — красный; восстановлен — зелёный); CI-evidence записи выше (PR-run 38427598831 FAIL / push-run 38426798763 success / rerun success); фикс — коммит пре-шага на ветке codex/story-2-10-protected-multimodal-continuation (прецедент cb1abbc — audit-flake закрыт пре-шагом цикла 2-9).
  resolution: closed in 2.10 пре-шаг — горутин-безопасный sequenceIDs + мутационно-верифицированный зонд; разметка epic-2-retrospective (2026-10-18, DEVLOG 102-retro).

- source_spec: `spec-2-12-degraded-mode-channel-failure-callback.md`
  summary: PascalCase-сериализация доменного PolicyOutcome на машинном JSON-пути: readCaseAPI (GET-ответ кейса) маршалит cases.Domain.PolicyOutcome напрямую — поля структуры без json-тегов (OutcomeClass/Reason/RuleVersion/ActiveAlternatives/DegradedPermittedPaths/InputSnapshot…), тогда как contracts/openapi PolicyOutcomeView документирует snake_case (outcome_class, reason, rule_version, active_alternatives, degraded_permitted_paths, input_snapshot…). Долг существует с 2-6 (поля 2-11 ActiveAlternatives и 2-12 DegradedPermittedPaths следуют конвенции структуры); СЛОВА словарей путешествуют проводом одинаково — расходится только регистр ключей. Собственные DTO 2-12 (RouteDegradationView/ReplyDeliveryView/WorkOutcome/PolicyView/PolicyInputSnapshot) несут настоящие snake_case-теги. Полное закрытие = json-теги доменной структуры PolicyOutcome + wire round-trip контракт-тест (httptest против readCaseAPI, ключи ответа) — правка машинной поверхности 2-6, вне замороженного скоупа 2-12 («не трогать материал 2-6»); цель — отдельный срез выравнивания контракта (одна структура, один тест, ноль семантических изменений значений).
  evidence: QA-саморевью step-05 (2026-10-15): живая проба json.Marshal(PolicyOutcome) — ключи PascalCase; internal/cases/api/policy.go PolicyWorkItemView.Outcomes []casesDomain.PolicyOutcome json:"outcomes" (прямая сериализация домена); OpenAPI PolicyOutcomeView — snake_case required/properties; интеграционная матрица 2-12 ассертит поля структур (не проводной JSON), journeys ходят живым HTML-транспортом — потому расхождение не ловилось контракт-тестами YAML.

- source_spec: `spec-3-6-dispatch-and-action-receipt.md`
  summary: Планировщик/демон для drain (3-3/3-6) и reconcile (3-7): обе команды — явные операторские (POST …/operations/drain, POST …/operations/reconcile), запуск по требованию; фоновое исполнение (ticker/воркер) — будущая инфраструктура, честно задокументированная границей «планировщик — будущая инфраструктура».
  evidence: спека 3-6 «НИКАКОГО фонового демона/ticker (честная граница: планировщик — будущая инфраструктура)»; спека 3-7 «явная команда reconcile (drain-прецедент, БЕЗ демона)»; записи копятся pending до команды оператора; перенесено в регистр epic-3-retrospective (2026-10-23, retro-3-1).

- source_spec: `spec-3-7-reconciliation-of-uncertain-results.md`
  summary: Webhook/приёмник поздних квитанций провайдера: повтор webhook моделируется повтором команды сверки (наследие 3-5/3-6 replay-моделирования); собственный ингресс-контур поздних доказательств не построен.
  evidence: спека 3-7 honest-границы «webhook/приёмник поздних квитанций (повтор webhook моделируется повтором команды сверки — наследие 3-5/3-6)»; перенесено epic-3-retrospective (2026-10-23, retro-3-1).

- source_spec: `spec-3-7-reconciliation-of-uncertain-results.md`
  summary: Многошаговые частичные эффекты сверки (AC-4 эпика отложен): продюсера частичных результатов нет — вердикт-словарь сверки {committed, no_effect, in_progress} + no_answer покрывает одношаговую семантику; многошаговый разбор появится с реальными провайдерами, дающими промежуточные состояния.
  evidence: спека 3-7 honest-границы «многошаговые частичные эффекты (нет продюсера — будущая поверхность, AC-4 эпика отложен)»; перенесено epic-3-retrospective (2026-10-23, retro-3-1).

- source_spec: `spec-3-7-reconciliation-of-uncertain-results.md`
  summary: Cancel/compensation ИСПОЛНЕНИЕ операций (AC-7/AC-8 эпика): семантики CancelCompensationSemantics декларированы каталогом 3-1 (проверяются guard'ом), политика 3-9 разрешает prepare_pause_cancel только при объявленной семантике — но исполнителей отмены/компенсации нет; справедливые будущие срезы объявят контур исполнения.
  evidence: спека 3-7 honest-границы «cancel/compensation операции (AC-7/AC-8 — будущие срезы 3-7)»; спека 3-9 AC-4 «pause/cancel исполнение (декларировано, Action Preview будущий срез)»; перенесено epic-3-retrospective (2026-10-23, retro-3-1).

- source_spec: `spec-3-8-recovery-case-creation.md`
  summary: Claim-режим события recovery_case.created (AC-3 эпика): потребитель события будущий — записи честно копятся pending (прецедент receipt.created 3-6); ингресс внешних претензий по recovery-кейсам отсутствует.
  evidence: спека 3-8 «потребитель будущий — claim-режим и контур 3-9 читают событие, записи честно копятся pending»; спека 3-9 honest-границы «claim-ингресс AC-3 (потребителя нет — событие recovery_case.created копится pending)»; перенесено epic-3-retrospective (2026-10-23, retro-3-1).

- source_spec: `spec-3-9-recovery-action-policy.md`
  summary: Recovery-обновления КЛИЕНТУ (AC-9): правдивые status-слова и дедлайн словами существуют на операторской карточке, обещания обновления не создаётся; отправка обновлений клиенту во время recovery — будущий срез (клиентская поверхность).
  evidence: спека 3-9 honest-границы «отправка обновлений AC-9 (слова 3-8 + дедлайн словами, обещания нет)»; перенесено epic-3-retrospective (2026-10-23, retro-3-1).

- source_spec: `spec-3-9-recovery-action-policy.md`
  summary: Eval-инфраструктура recovery-ответов (AC-12): оценка качества recovery-действий не построена — инфраструктура будущая, за пределами read-time политики.
  evidence: спека 3-9 honest-границы «eval AC-12 (инфраструктура будущая)»; перенесено epic-3-retrospective (2026-10-23, retro-3-1).

- source_spec: `spec-3-1-business-action-catalog.md`
  summary: Tenant-конфигурация разрешённых действий с release-манифестом (AC-4 каталога — второй срез): сегодня каталог compile-time определений fixture-контура (AG-4: провайдеров нет); конфигурирование допустимых действий на тенанта с релизной дисциплиной 1-12 — будущая поверхность.
  evidence: спека 3-1 Never-границы «tenant-конфигурация разрешённых действий (AC-4 — второй срез: employee-конфигурация с release-манифестом 1-12)»; перенесено epic-3-retrospective (2026-10-23, retro-3-1).

- source_spec: `spec-3-7-reconciliation-of-uncertain-results.md`
  summary: Recovery-scope-разрешения сверки при приостановленном коннекторе (долг 3-7): срез 3-9 консервативен — scoped control paused = блок сверки (запроса нет, honest-строка отчёта); разрешение ограниченной сверки в recovery-режиме не построено.
  evidence: спека 3-7 honest-границы «recovery-scope-разрешения сверки при приостановленном коннекторе (3-9 — срез консервативен: paused = блок)»; перенесено epic-3-retrospective (2026-10-23, retro-3-1).

- source_spec: `spec-4-1-pilot-admission-registry.md`
  summary: Реальные адаптеры и credentials пилота (jivo-chat / abcp-parts / staffy-synthetic-telephony): декларации версионированы (AdapterImplemented false), производственный dispatch заблокирован nil-адаптером до AG-4/OQ-1..OQ-5 PASS; Mango Office — кандидат голосового маршрута (OQ-1 open).
  evidence: спека 4-1 Always/never-границы (AG-4 open, OQ-11 synthetic-only); подтверждено 4-4/4-6 (поле версии Connector не заполняется, заказ — nil-адаптер); перенесено epic-4-retrospective (2026-10-26, retro-4-1).

- source_spec: `spec-4-3-part-request-classification.md`
  summary: Реальный AI-классификатор запроса детали (семантическое извлечение фактов из слов клиента) и диагностический маршрут каталога — вне пилота: режим выводится детерминированной лестницей, извлечение — оператор с provenance 2-2.
  evidence: спека 4-3 Implementation Notes «реальный AI-классификатор — вне пилота (AG-4; AD-7 контур со своими stories)»; перенесено epic-4-retrospective (2026-10-26, retro-4-1).

- source_spec: `spec-4-4-fitment-evidence.md`
  summary: Лицензированный каталог-коннектор (Laximo/OQ-5) и поле «версия Connector» в fitment-записях: появится с реальным адаптером после OQ-5 PASS; материальный потребитель ProfileDigest (результаты каталога) — будущий срез.
  evidence: спека 4-4 Implementation Notes «поле не заполняется; трассируемость материалом записи… появится с реальным адаптером по OQ-5 PASS»; спека 4-2 «материальный потребитель digest — будущий срез»; перенесено epic-4-retrospective (2026-10-26, retro-4-1).

- source_spec: `spec-4-5-offers-and-comparison.md`
  summary: Реальное чтение предложений поставщиков через адаптер (сейчас — менеджерские записи Supplier Offer); доставка/сбор не оцениваются и окончательная стоимость не рассчитывается (единственная арифметика — количество × записанная клиентская цена).
  evidence: спека 4-5 Implementation Notes «доставка/сбор НЕ оцениваются и НЕ обещаются»; AG-4; перенесено epic-4-retrospective (2026-10-26, retro-4-1).

- source_spec: `spec-4-7-decision-ready-order-verification.md`
  summary: Внешняя доставка уведомлений (email/SMS/push/Jivo-исходящие) и автоматический backup-переключатель задач/воркеры/таймеры: уведомление — карточка поверхности, overdue — read-time деривация, fallback — явный reassign человеком.
  evidence: спека 4-7 Never-границы (OQ-2 вне среза; AG-4/AD-14 runtime-workers); зеркалится 4-8/4-9/4-10 Never; перенесено epic-4-retrospective (2026-10-26, retro-4-1).

- source_spec: `spec-4-8-order-correction-before-purchase.md`
  summary: Исполнение отмены/изменения ABCP-заказа: change/cancel-команд в живой декларации 4-1 нет (каталог не расширяется), отмена — слово-рекомендация + ручной контур (консолидирует запись №215 Epic 3 об исполнителях cancel/compensation); фиксация канала связи с клиентом при коррекции — OQ-1/OQ-2.
  evidence: спека 4-8 Never-границы «НЕ строить изменение ABCP-заказа действием (AG-4)»; спека 4-10 CancelFeasibility — «подтверждённая отмена не существует»; перенесено epic-4-retrospective (2026-10-26, retro-4-1).

- source_spec: `spec-4-9-truthful-order-status.md`
  summary: Адаптеры чтения ABCP/1С/поставщика и webhook-приёмник/polling/callback поздних состояний (консолидирует запись Epic 3 о webhook поздних квитанций): источники фактов — только квитанции диспетчерного контура и человеческие записи.
  evidence: спека 4-9 Never-границы (AG-4: credential-bearing реализации нет, дедуп — идемпотентность BY KEY и упорядоченный fold); перенесено epic-4-retrospective (2026-10-26, retro-4-1).

- source_spec: `spec-4-9-truthful-order-status.md`
  summary: Расширение authorityMatrixV1 (2-7) заказными полями жизненного цикла заказа: матрица полномочий живёт словарями спеки 4-9; расширение единой матрицы — будущий срез с OQ-19 evidence.
  evidence: спека 4-9 Never-границы «НЕ расширять authorityMatrixV1 (OQ-19 открыт)»; перенесено epic-4-retrospective (2026-10-26, retro-4-1).

- source_spec: `spec-4-10-safe-existing-order-servicing.md`
  summary: Lookup заказа через Connector чтения ABCP/1С/поставщика (OQ-18) и cross-case профиль поиска по телефону/email (OQ-16): поиск читает только записанный контуром материал, case-scoped only.
  evidence: спека 4-10 Never-границы «lookup — ТОЛЬКО материал контура», «cross-case профиль не строится»; перенесено epic-4-retrospective (2026-10-26, retro-4-1).

- source_spec: `spec-4-11-pilot-business-outcome.md`
  summary: OQ-7 baseline-контур и сравнительные выводы пилота (SM-2 живой этап); терминальные стадии воронки (paid/returned) и выручка из 1С (OQ-4); batch-чтения отчёта /pilot-outcome (сейчас ListCases → per-case чтения).
  evidence: спека 4-11 Never-границы «НЕ строить baseline/когорты/пороги (OQ-7 не пройден)», «выручка — только Owning System 1С (OQ-4)»; Implementation Notes «batch-чтения — будущая оптимизация»; перенесено epic-4-retrospective (2026-10-26, retro-4-1).
