# UX-следствия конкурентного и технического исследований Staffy

## Назначение и границы

Этот рабочий источник извлекает только подтверждённые или явно оговорённые UX-следствия из:

- `competitive-staffy-2026-09-11/research.md` — далее ссылки вида **[C…]**;
- `technical-staffy-voice-architecture-2026-09-11/research.md` — далее ссылки вида **[T…]**.

Номера сохранены от исходных source appendix, а URL приведены в конце. Здесь нет выбора компонентной архитектуры. Формулировки «UX-следствие» являются синтезом источников, а не утверждением, что конкретный экран уже существует или что заявленная поставщиком функция прошла Staffy-пилот.

Главный вывод: на рынке уже широко заявлены LLM, RAG, инструменты, перебивания, CRM/API/MCP и no/low-code. Поэтому UX Staffy должен доказывать не количество функций, а **простоту безопасного запуска, честный статус каждого обещания, бесшовное участие человека и проверяемое завершение бизнес-операции** [C1] [C2] [C9] [C11] [C12].

## 1. Setup и подключение бизнеса

### Обязательный путь запуска

1. **Выбрать роль ИИ-сотрудника и готовый Domain Pack.** Пользователь начинает не с prompt-поля, а с понятного результата: какие обращения Staffy принимает, что может обещать, какие действия выполняет и когда передаёт человеку. Рынок уже использует категорию «ИИ-сотрудник», поэтому copy должен конкретизировать рабочий процесс, а не повторять общий ярлык [C2] [C15].
2. **Подключить каналы и рабочие системы отдельно.** Для каждой связи setup показывает фактически проверенные возможности: вход/исход, текст/медиа/голос, receipts, handoff, действия, лимиты и деградацию. Наличие логотипа интеграции не означает production readiness; официальный API и фактический аккаунт требуют contract test [T75] [T78] [T81] [T82] [T83].
3. **Назначить источник истины и разрешённые действия.** Администратор должен видеть, откуда берутся цена, наличие, слот, заказ или статус, и какие операции доступны Staffy. RAG подходит для документов и объяснений, но неполная выдача не гарантирует точность; меняющиеся бизнес-факты должны подтверждаться операционной системой перед обещанием [T69] [T71].
4. **Настроить границы автономности и handoff.** Для каждого класса обращения показываются действия `только ответ`, `подготовить`, `выполнить после подтверждения`, `только человек`, а также адресат и fallback. MCP/tool annotations сами по себе не являются enforcement [T69] [T92].
5. **Проверить на симуляции.** До публикации администратор проходит тестовый диалог, отказ интеграции, неоднозначное подтверждение, перебивание, недоступного оператора и неизвестный исход действия. Тесты должны использовать те же события и состояния, что рабочий runtime [T62] [T65].
6. **Опубликовать версию с возможностью отката.** Экран показывает, какая версия роли, знаний, политик, модели и инструментов активна, результаты pre-publish checks и предыдущую безопасную версию. Версионированные prompts/evals/rollback и incident rollback подтверждены как рабочие практики [T73] [T74].

### Принципы setup UX

- **Assisted-first, self-service where proven.** Публичные цены и отзывы показывают, что внедрение, итерации сценария, интеграции и поддержка — реальная часть покупаемой ценности; нельзя маскировать сложную настройку кнопкой «Создать сотрудника за пять минут» [C4] [C5] [C15] [C20] [C21].
- **Progressive disclosure.** Сначала бизнес-результат и обязательные решения; продвинутые лимиты, протоколы и техническая диагностика раскрываются по необходимости.
- **Readiness вместо декоративного success.** Каждая интеграция имеет состояния `не подключена`, `учётные данные приняты`, `возможности проверяются`, `готова для теста`, `готова для live`, `ограничена`, `ошибка`, `приостановлена`. Успешная авторизация ещё не равна проверенному сквозному действию.
- **Preview фактического поведения.** Перед live администратор видит customer-facing раскрытие роли, разрешённые обещания, handoff copy, действия и деградированный маршрут.
- **Граница no-code обещания.** Новый клиент на существующих Domain Pack и connector настраивается без кода; новая система/семантика требует переиспользуемого расширения. UX не обещает, что prompt + RAG безопасно подключат любую транзакцию [T68] [T69] [T71] [T72].

## 2. Интеграции и Business Actions

### Клиентский UX действия

- Материальное действие проходит видимые этапы `подготовлено → требует подтверждения → выполняется → подтверждено системой / не выполнено / исход неизвестен`. Tool call или HTTP-ответ не должны выглядеть завершённым заказом [T61] [T67] [T70].
- Подтверждение связывается с точным предметом действия. Изменение клиента, артикула/услуги, количества, цены или способа исполнения требует повторного просмотра delta и нового подтверждения [T28].
- При timeout Staffy не говорит «готово» и не предлагает слепо повторить. Состояние `проверяем результат` объясняет, что повтор может создать дубль, и даёт срок/владельца следующего обновления [T25] [T26] [T29] [T72].
- Частичный результат показывается по позициям/частям, а не общим зелёным success. Для пользователя должны быть различимы выполненная, отклонённая и неизвестная части [T25] [T27] [T30].

### Администраторский UX интеграции

- Карточка connector показывает: аккаунт/Tenant, разрешённые функции, последнюю проверку, ограничения, лимиты, состояние авторизации, последние ошибки, retry/reconciliation и доступный ручной маршрут.
- Для action permissions нужен человеческий словарь («может создать черновик заказа», «не может отправить поставщику»), а не только scopes/API-названия.
- Unknown outcome создаёт одну decision-ready операционную задачу: исходная команда, подтверждённый payload, попытки, внешний ID при наличии, какие проверки выполнены и какие действия безопасны. Повтор не является основной CTA.
- Секрет/токен никогда не показывается модели или в расшифровке; в UI видны только владелец, scope, срок/статус и возможность безопасной ротации. Это UX-следствие из границы, где модель предлагает typed intent, а фактические полномочия проверяются отдельно [T21] [T22] [T28].

## 3. Омниканальность

### Единое ощущение разговора

- Клиент продолжает тот же Case/Order между голосом, сайтом и мессенджером без повторного intake, но Staffy не склеивает channel identities автоматически по похожему имени или номеру [T75] [T77] [T78] [T80] [T81] [T82].
- В истории всегда видны канал, время, отправитель, reply context, media type и достоверный delivery state. `API accepted` не отображается как `доставлено`, `прочитано` или `услышано`, если канал этого не подтверждает [T75] [T77] [T78] [T79] [T80] [T81] [T82].
- Composer адаптируется к возможностям канала: скрывает неподдерживаемое, предлагает безопасную альтернативу и объясняет, почему действие недоступно. Не следует имитировать read receipt, media или инициативное сообщение там, где API этого не даёт [T76] [T77] [T82].
- Для одной business identity и разговора одновременно существует одна записывающая поверхность: бот либо человек. Прямой adapter и Jivo/другой inbox не могут параллельно отвечать от одного аккаунта [T81] [T82] [T87] [T88].

### Подключение каналов

| Канал/поверхность | UX-паттерн подключения | Обязательная оговорка |
|---|---|---|
| Staffy web widget | Код вставки, preview домена, origin/session/upload check, тестовый диалог | Установка скрипта не подтверждает корректность identity, media и handoff [T84] |
| Telegram | Официальный bot setup, webhook/secret test, capability preview | Обычный бот не начинает стандартный диалог сам; receipt semantics ограничены [T75] [T76] [T77] |
| MAX | Официальный webhook/secret, canary test, media degradation preview | Handoff/read receipts и часть failure semantics не подтверждены [T78] [T79] [T90] |
| Jivo | Staffy как bot внутри существующего операторского inbox; тест invite-to-agent | Bot API media, подпись/авторизация и operational timings требуют account-level проверки [T81] [T82] |
| Android/iOS | Тот же клиентский смысл: история, attachments, handoff и action confirmation | Публичный SDK/единая доставка не считаются доказанными только по web path [T82] |
| VK | Live-account capability test до публикации | Secret, media, reply window, receipts и limits требуют проверки [T80] |
| Avito | Только официальный Messenger/Jivo path и eligibility check | Технические webhook/media semantics в исследовании не подтверждены [T83] |
| WhatsApp | Не предлагать как доступную production-интеграцию | Доступность Business app не подтверждает Russia Platform API eligibility [T85] |

### Handoff между ботом и человеком

- Клиент видит не «перевожу», а честную последовательность: `ищем сотрудника → сотрудник отвечает/не ответил → вы остаетесь со Staffy / выбран другой путь`.
- Бот перестаёт писать только после атомарной смены владельца; при неудаче управление явно возвращается, а клиент не остаётся в молчании.
- Jivo подходит как наиболее короткий operator path для Tenant, уже использующего Jivo; Chatwoot остаётся только кандидатом после ограниченного spike. Это нельзя превращать в общий UX-обещанный connector без проверки [T81] [T82] [T86] [T87] [T88] [T89] [T90].

## 4. Голос: задержка, turn-taking и recovery

### Восприятие задержки

- UX измеряет и оптимизирует **far-end end-of-turn → first audible response**, а не только внутреннее время модели. Отдельно измеряются EOU/STT, thinking/LLM, TTS и transport [T49] [T50].
- Пока нет Staffy baseline, нельзя обещать числовую «мгновенность» или выбирать стек по чужому benchmark. Публичный benchmark Pipecat/LiveKit исключал телефонию и не описывал русский корпус; сравнительных Yandex/Exolve/LiveKit/Pipecat тестов на российском PSTN не найдено [T17].
- Естественность влияет на воспринимаемую безопасность и ожидания пользователя, но конкурентный UX должен сначала пройти приемлемый голосовой порог, затем выигрывать корректностью процесса и recovery [C32].
- При заметной паузе агент использует краткий, правдивый progress cue только когда реально ждёт систему. Он не заполняет тишину вымышленным результатом и не перебивает клиента длинной заглушкой.

### Перебивание и услышанный контекст

- Speech start/end, partial/final input, agent output и accepted interruption должны иметь различимый пользовательский результат; один silence threshold нельзя считать универсальным для всех языков и каналов [T64].
- При перебивании Staffy прекращает слышимое воспроизведение, отменяет неактуальную очередь речи и сообщает судьбу долгого действия, если оно уже началось. «Остановить TTS» недостаточно [T59] [T61] [T64].
- Продолжение разговора опирается только на фактически доставленную пользователю часть ответа, а не на весь сгенерированный текст [T60]. Это предотвращает ссылки на цену, предупреждение или вопрос, которых клиент не услышал.
- Российский turn-taking должен проверяться отдельно: в исследованных материалах LiveKit audio turn detector не заявлял русский язык, а русская text-модель была deprecated перед Agents 2.0 [T13].

### Разрыв связи и восстановление

- После reconnect UX кратко подтверждает восстановленный контекст и перепроверяет незавершённый вопрос/подтверждение; разрыв WebSocket может означать новую runtime session [T1].
- Hang-up во время action не отменяет и не подтверждает его автоматически. Следующий контакт начинается со сверенного состояния операции.
- Carrier/provider fallback должен давать клиенту человечески понятный следующий путь даже при полном отказе Staffy. Переадресация и резервный маршрут заявлены Exolve, но конкретные mid-call transfer/failure contracts требуют проверки [T18] [T19] [T20].

### Перевод оператору в звонке

- Состояния перевода различаются: `контекст сохранён`, `звоним сотруднику`, `сотрудник ответил`, `соединяем`, `не ответил`, `возвращаемся/предлагаем callback`.
- Нельзя завершать разговор клиента после отправки команды transfer: trigger и фактический ответ сотрудника — разные failure points [T19] [T20] [T66].
- До дозвона фиксируется короткий context packet; оператор получает цель, подтверждённые факты, незавершённые действия, обещания и причину handoff, чтобы клиент не повторял историю.

## 5. Testing и monitoring как часть UX

### До публикации

- Проверять text/tool сценарии до дорогих audio cases: нормальный путь, многотуровый диалог, tool failure, отказ подтверждения, handoff и восстановление [T65].
- Голосовой corpus должен включать русский PSTN/8 kHz, шум, короткие ответы, перебивания, сущности вроде VIN/OEM/брендов/телефонов/дат/цен, hang-up во время tool и недоступного оператора. Предложенный исследованием диапазон 100–300 синтетических и разрешённых обезличенных звонков — план испытаний, не рыночная норма.
- Upgrade/release проходит replay на записанном разрешённом test corpus, поскольку realtime formats, framework APIs, turn detection и defaults быстро меняются [T4] [T14] [T16] [T62].
- Ни синтетический success, ни vendor demo не разрешают автономность. Нужны реальные обращения и отдельный gate для критических ошибок; конкурентное исследование прямо не измеряло live-call quality/ROI [C20] [C21] [C33].

### Операционные метрики, которые должны быть видны

- task completion и корректность бизнес-результата;
- safe escalation и handoff acknowledgement/failure;
- p50/p95 end-of-turn → first audio;
- caller speech onset → last audible bot audio при interruption;
- repeated/abandoned turns и repair success;
- tool states: succeeded, failed, unknown, cancelled; duplicate side effects должны быть нулевыми;
- delivery/receipt truth по каналам;
- cost per **correctly completed** operation, а не только ₽/мин;
- ручные исправления, время оператора и root cause по версии конфигурации.

Разделение метрик подтверждается Pipecat/LiveKit observability definitions [T49] [T50]; стоимость минуты не сравнима без token mix, carrier, support, infrastructure и labor [T8] [T10] [T31] [T32] [T33].

### Monitoring UX

- Главная очередь оператора сортируется по клиентскому риску: активный звонок без владельца, неизвестный outcome, failed handoff, действие без receipt, истекающее подтверждение, stale facts, затем обычные ошибки.
- Каждая проблема показывает «что видит клиент», «что уже обещано», «кто владелец», «что безопасно сделать сейчас» и технические детали по progressive disclosure.
- Alert ведёт к одному безопасному действию или runbook, а не просто к графику. Восстановление сохраняет историю попыток и не предлагает destructive retry по умолчанию.
- Версия prompt/model/tool/policy и business result должны быть связаны в одном trace, иначе невозможно объяснить regression или безопасно откатить [T73] [T74].

## 6. Operator experience

### Рабочее место человека

- **Одна очередь и один текущий Response Owner.** Даже при множестве каналов оператор видит единый Case/Order и понимает, может ли он отвечать сейчас.
- **Decision-ready карточка:** клиент и канал; цель; подтверждённые/спорные факты; последние фактически услышанные/доставленные сообщения; активные Commitments; tool outcomes; media; причина handoff; рекомендуемый следующий безопасный шаг.
- **Два уровня деталей:** сверху человеческая ситуация и CTA, ниже trace, provider IDs, retries и версии. Оператору не нужно читать сырой transcript, чтобы понять решение.
- **Acknowledgement обязателен.** Отправленная задача не считается принятым handoff. Клиент получает обновление при no-answer, timeout или смене владельца.
- **Контролируемый takeover/return.** После takeover бот замолкает; возврат управления требует явного действия человека и обновлённого summary.
- **Неизвестный исход — отдельный режим работы.** CTA сначала `проверить во внешней системе`, затем допустимое завершение/reconciliation; `повторить` появляется только при доказанной безопасности.
- **Исправление как обучающий сигнал:** human override сохраняет причину, затронутую версию и итог, но не обещает автоматическое «самообучение» без review/eval.

Отзывы о Tomoru и Exolve указывают, что итерации сценария, качество внедрения, поддержка и эскалация реально влияют на результат, хотя эти отзывы имеют средне-низкую уверенность и не доказывают приоритет над естественностью [C20] [C21].

## 7. Конкурентные UX-паттерны и позиция Staffy

| Рыночный паттерн | Что уже заявлено | UX-следствие для Staffy |
|---|---|---|
| Конструктор/low-code | Сценарии, NLU, SIP, пакеты генеративных запросов [C8] | Нельзя продавать одно лишь визуальное создание бота; setup должен доводить до проверенного результата |
| Managed AI employee | Настройка, интеграции, эксплуатация и поддержка включаются в продукт [C15] [C16] | Помощь при запуске, readiness и контроль качества — часть основного buyer journey, не скрытый professional service |
| Knowledge + RAG + tools | Заявлены несколькими российскими игроками [C2] [C6] [C9] [C11] [C12] | Не использовать как headline differentiation; показать provenance, актуальность и границы обещания |
| Перебивания и естественный голос | Уже заявлены крупными поставщиками [C1] [C6] [C9] | Это baseline acceptance, а не moat; recovery должен быть видимым и измеримым |
| CRM/1С/API/MCP | Уже присутствуют в предложениях [C1] [C9] [C11] | Логотип connector — входной билет; ценность в точном действии, правах, подтверждении и reconciliation |
| Cloud/on-prem | Предлагаются конкурентами [C6] [C9] [C11] | Buyer UX должен честно объяснять data path и эксплуатационную ответственность, а не сводить выбор к бейджу «в РФ» |
| Поминутные и managed-пакеты | Платформенные минуты и managed-предложения несопоставимы [C1] [C3] [C4] [C8] [C15] | Pricing UX Staffy должен отделять трафик, внедрение/поддержку и бизнес-результат; не обещать «самую дешёвую минуту» |

Рекомендуемая позиция в интерфейсе и демо: **«ИИ-сотрудник, которого можно безопасно подключить к вашим процессам, увидеть границы его полномочий и проверить каждый бизнес-результат»**. Это продуктовый синтез: публичные материалы конкурентов почти не раскрывают authorization, idempotency, reconciliation и recovery, но отсутствие документации не доказывает отсутствие функций у конкурента.

## 8. Caveats: что нельзя превращать в UX-обещание

- Большинство competitive capability claims — страницы самих поставщиков; исследование подтверждает, что функция заявлена, но не её качество в живом звонке.
- Конкурентное исследование не измеряет TAM, willingness to pay, live-call quality, конверсию или полную экономику. Managed pricing нельзя сравнивать с инфраструктурной минутой напрямую.
- Последний найденный публичный статус native SIP Yandex — Preview от 2026-05-28; текущий contractual/GA status нужно получить письменно [T3] [T4]. UX setup не должен показывать `готово к production` до этого.
- Yandex публикует default quota 10 simultaneous Realtime sessions и 10 creations/s; увеличение требует отдельного процесса. Это не предел Staffy, но должно отражаться в capacity readiness [T2].
- Общая региональная локализация Yandex не отвечает на вопросы о raw/generated audio, transcript, prompts/tools, support traces, backups и logging opt-out для Realtime WebSocket [T6] [T7]. Бейдж «данные в РФ» без artifact-level проверки недостаточен.
- Novofon заявляет SIP trunk prerequisites, но Staffy↔Novofon/runtime совместимость, codecs, DTMF, transfer, ACL и failures не проверены [T54].
- Exolve заявляет integrated agent, tools и handoff, но полный runtime/SLA/callback/transfer failure contract в просмотренных материалах отсутствует [T10] [T19] [T20].
- Pipecat «50» означает лимит пула одного cloud deployment с путём расширения, а не доказанный предел клиентов или Python [T40]. Сторонний synthetic benchmark выше 50 не гарантирует production latency [T42].
- Pipecat/LiveKit abstraction не гарантирует одинаковый русский turn-taking, cancellation, recovery, tenant isolation или capacity. Нужны собственные acceptance tests [T13] [T17] [T59] [T63] [T64].
- Jivo, MAX, VK, Avito и Chatwoot требуют account/version-level tests. Cross-channel identity automation, read receipts, media behavior и полный российский data perimeter остаются частично неизвестными [T78] [T80] [T81] [T82] [T83] [T89] [T90].
- Chatwoot подтверждает полезные inbox/handoff patterns, но не готовую MAX-интеграцию и не автоматически допустимый российский egress/data contour [T86] [T87] [T88] [T89] [T90].
- Естественность голоса важна, но не доказано, что покупатели всегда ставят её выше надёжного завершения и handoff [C32].

## 9. Открытые UX-решения

| Решение | Что требуется выбрать/измерить | Когда закрыть |
|---|---|---|
| **OQ-1 Setup promise** | Где заканчивается self-service для готового pack/connector и начинается assisted onboarding; какая проверка разрешает слово «подключено» | До проектирования setup completion screen |
| **OQ-2 Integration readiness** | Единые customer-facing состояния capability/account/data/operation readiness и copy для partial support | До Integration Catalog prototype |
| **OQ-3 Response ownership** | Jivo как operator desk для первых Tenant; fallback для клиента без inbox; запрет двух writable paths | До handoff flow и подключения первого внешнего канала |
| **OQ-4 Identity linking** | Какие доказательства позволяют связать телефон, web и messenger identity; как выглядит запрос step-up | До omnichannel continuation flow |
| **OQ-5 Delivery semantics** | Какие статусы показывать по каждому каналу и где честно ограничиться `отправлено в API` | До общей timeline/composer specification |
| **OQ-6 Voice progress** | Допустимая длина тишины, progress cues и repair copy для русского PSTN | Зафиксировать гипотезу до controlled calls, пороги — после baseline |
| **OQ-7 Interruption policy** | Какие ответы/действия отменяются при barge-in, какие продолжаются и как это объясняется | До voice conversation prototype |
| **OQ-8 Reconnect contract** | Какие факты повторно подтверждаются после разрыва и как не заставить клиента проходить intake заново | До voice recovery testing |
| **OQ-9 Handoff promise** | Тексты и timing для searching/ringing/answered/no-answer/callback; когда ownership считается принятым | До первого live handoff drill |
| **OQ-10 Correct completion** | Доменные определения task success, safe escalation, duplicate, correction и business result | До dashboard и eval acceptance criteria |
| **OQ-11 Latency SLO** | Целевые p50/p95 far-end response и interruption stop по voice route; различия normal/tool/handoff turns | После русского PSTN baseline, до live gate |
| **OQ-12 Monitoring priority** | Порог и owner для active-call failure, unknown action, stale fact, failed handoff и provider degradation | До operator queue prototype |
| **OQ-13 Version change UX** | Кто видит release diff, какие eval блокируют publish, кто может rollback и что видит клиент в деградации | До configuration publish flow |
| **OQ-14 Commercial packaging** | Как отделить setup/support, traffic и результат; какая единица понятна покупателю и связана с value | После первых оплачиваемых пилотов; не выводить из публичной цены минуты |
| **OQ-15 Vendor disclosure/data copy** | Что можно утверждать о регионе, retention, записи и subprocessors для каждого выбранного path | До customer-facing privacy/setup copy и production |

## 10. UX acceptance evidence

UX-артефакты должны предусмотреть и сделать измеримыми как минимум следующие сценарии:

1. Новый Tenant подключает готовый канал, но account-level test выявляет отсутствующую capability: setup не объявляет успех и предлагает точный следующий шаг.
2. Пользователь меняет материальный параметр после Confirmation: финальная CTA показывает delta и требует новое подтверждение.
3. Timeout Business Action: клиент получает `проверяем`, оператор — reconciliation task; повтор не создаёт второй эффект.
4. Один внешний conversation приходит через direct adapter и Jivo: второй writable path блокируется, история не раздваивается.
5. Клиент меняет канал: Staffy предлагает безопасное linking, не объединяя личности по похожим атрибутам.
6. Barge-in во время длинного ответа: звук прекращается, неуслышанный текст не считается доставленным, незавершённое действие получает ясный статус.
7. Разрыв во время подтверждения/action: после reconnect Staffy восстанавливает контекст, но перепроверяет обязательство и внешний outcome.
8. Перевод человеку: caller leg не исчезает после transfer trigger; no-answer приводит к честному fallback.
9. Provider/channel outage: customer-facing канал деградирует предсказуемо, operator queue показывает владельца и runbook.
10. Новая версия runtime/prompt/tool: replay ловит regression, publish блокируется или выполняется rollback с видимым audit trail.

## Source appendix — сохранённые ссылки исходных исследований

### Competitive research

| Ref | Источник | Что используется здесь |
|---|---|---|
| [C1] | [MTS Exolve](https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/) | Функции, интеграции, перебивания и публичная цена |
| [C2] | [MANGO OFFICE](https://retail.mango-office.ru/products/contact-center/ai/voice-robot/) | Сценарный и LLM/RAG «ИИ-сотрудник» |
| [C3] | [MANGO OFFICE tariffs](https://static.mango-office.ru/products/contact-center/tariffs/) | Тарифы робота/API; не доказано равенство LLM-продукту |
| [C4] | [Tomoru pricing](https://tomoru.ru/price) | Поминутная упаковка |
| [C5] | [Tomoru plans](https://tomoru.ru/plan) | Платный запуск/managed packaging |
| [C6] | [TWIN](https://twin24.ai/products/large-business/) | RAG/tools/reporting, cloud/on-prem |
| [C8] | [Just AI / Aimylogic](https://aimylogic.com/ru/price) | Low-code, SIP и пакетная модель |
| [C9] | [Robovoice / SL Soft](https://slsoft.ru/products/robovoice/) | Voice/text, GPT, база знаний, CRM и on-prem |
| [C11] | [ZIAX](https://ziax.ru/) | LLM/RAG/MCP, API/CRM и on-prem |
| [C12] | [Yandex AI Studio Realtime](https://aistudio.yandex.ru/ru/docs/ai-studio/concepts/agents/realtime) | Realtime voice и tools |
| [C15] | [Replica](https://thereplica.ru/) | Managed employee packaging и цены; performance claims не доказаны |
| [C16] | [Voxagent offer](https://voxagent.ru/docs/public-offer) | Managed анализ, интеграции и поддержка |
| [C20] | [Отзывы Tomoru / Otzovik](https://otzovik.com/reviews/tomoru_ru-servis_razrabotki_golosovih_robotov/) | Противоречивый, средне-низкой уверенности сигнал о внедрении/поддержке |
| [C21] | [Отзыв Exolve / Startpack](https://startpack.ru/application/exolve/reviews) | Средне-низкий сигнал о необходимости итераций сценария |
| [C32] | [ACM / University of Surrey](https://openresearch.surrey.ac.uk/esploro/outputs/journalArticle/Influence-of-Robots-Voice-Naturalness-on/991115568402346) | Влияние естественности голоса на восприятие; перенос на Staffy ограничен |
| [C33] | [Reddit / RU_Talk](https://www.reddit.com/r/RU_Talk/comments/1vy5vkq/16_%D0%BB%D0%B5%D1%82_%D0%B2_%D0%BA%D0%BE%D0%BB%D0%BB_%D1%86%D0%B5%D0%BD%D1%82%D1%80%D0%B5_%D1%80%D0%B0%D1%81%D1%81%D0%BA%D0%B0%D0%B7%D1%8B%D0%B2%D0%B0%D0%B5%D0%BC_%D0%BA%D0%B0%D0%BA_%D0%B2%D1%81%D0%B5_%D1%83%D1%81%D1%82%D1%80%D0%BE%D0%B5%D0%BD%D0%BE/) | Низкоуверенный анонимный сигнал; страница была недоступна при финальной проверке |

### Technical research

| Ref | Источник | Что используется здесь |
|---|---|---|
| [T1] | [Yandex Realtime](https://aistudio.yandex.ru/ru/docs/ai-studio/concepts/agents/realtime) | Session/audio/events/VAD/tools |
| [T2] | [Yandex limits](https://aistudio.yandex.ru/en/docs/ai-studio/concepts/limits) | Default Realtime quotas |
| [T3] | [Yandex SIP Preview announcement](https://yandex.cloud/ru/blog/voice-agent-and-other-updates) | Исторический Preview status |
| [T4] | [Yandex release notes](https://aistudio.yandex.ru/ru/docs/ai-studio/release-notes/) | Protocol/model churn; отсутствие найденного GA — bounded negative |
| [T6] | [Yandex regions](https://yandex.cloud/ru/docs/overview/concepts/region) | Общее региональное размещение user data |
| [T7] | [Yandex disable logging](https://aistudio.yandex.ru/ru/docs/ai-studio/operations/disable-logging) | Logging controls; WebSocket coverage unverified |
| [T8] | [Yandex pricing](https://aistudio.yandex.ru/ru/docs/ai-studio/pricing) | Компонентная тарификация |
| [T10] | [MTS Exolve](https://exolve.ru/products/robots/golosovoy-robot-dlya-biznesa/) | Advertised managed agent/handoff/integrations |
| [T13] | [LiveKit turns](https://docs.livekit.io/agents/logic/turns/turn-detector/) | Language support и deprecation caveat |
| [T14] | [LiveKit Agents release](https://github.com/livekit/agents/releases/tag/livekit-agents@1.8.1) | Release churn |
| [T16] | [Pipecat release](https://github.com/pipecat-ai/pipecat/releases/tag/v1.9.0) | Release/deprecation churn |
| [T17] | [Veris benchmark](https://veris.ai/blog/vamos-bench-pipecat) | Ограниченный vendor-run benchmark без telephony/Russian corpus |
| [T18] | [Exolve forwarding](https://exolve.ru/faq/kak-prinimat-vkhodyashchie-vyzovy-cherez-pereadresatsiyu/) | Резервная маршрутизация |
| [T19] | [Exolve dynamic routing](https://docs.exolve.ru/docs/ru/instructions/call-forwarding-to-url/) | Routing/reserve; callback gaps |
| [T20] | [Exolve call events](https://docs.exolve.ru/docs/ru/api-reference/statistics-api/call-events/) | Call/leg/answer/end events |
| [T21] | [1С REST/OData](https://v8.1c.ru/platforma/rest-interfeys/) | Слишком широкий generic CRUD surface для прямого model UX |
| [T22] | [1С HTTP services](https://v8.1c.ru/platforma/http-servisy/) | Узкие domain actions возможны |
| [T25] | [ROSSKO checkout](https://api.rossko.ru/index.php?action=SOAP&path=/GetCheckout) | Multiple IDs и partial results |
| [T26] | [ROSSKO order lookup](https://api.rossko.ru/index.php?action=SOAP&path=/GetOrders) | Lookup limitations after timeout |
| [T27] | [ARMTEK Web Services](https://ws.armtek.ru/?alias=order&page=service) | Allocation rows; primary unavailable in final pass |
| [T28] | [OWASP Transaction Authorization](https://cheatsheetseries.owasp.org/cheatsheets/Transaction_Authorization_Cheat_Sheet.html) | Payload-bound confirmation |
| [T29] | [IETF Idempotency-Key draft](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/) | Useful semantics; expired draft, not RFC |
| [T30] | [AWS saga guidance](https://docs.aws.amazon.com/pdfs/prescriptive-guidance/latest/cloud-design-patterns/cloud-design-patterns.pdf) | Partial operations, compensation, observability |
| [T31] | [Exolve tariffs](https://exolve.ru/tariffs/api/) | Дополнительные тарифные компоненты |
| [T32] | [Exolve official PDF](https://exolve.ru/uploads/VOIP_Exolve_01_01_2026_ea93fe2f76.pdf) | Ранее извлечённая ставка; PDF unavailable in final pass |
| [T33] | [LiveKit pricing](https://livekit.com/pricing) | Калькулятор; применимость к РФ ограничена |
| [T40] | [Pipecat Cloud scaling](https://docs.pipecat.ai/pipecat-cloud/fundamentals/scaling) | 50 — deployment pool limit, не клиентский/GIL предел |
| [T42] | [Flowcat benchmark](https://github.com/AreevAI/flowcat/blob/main/bench/RESULTS.md) | Synthetic >50 sessions; конкурент, без независимой репликации |
| [T49] | [Pipecat metrics](https://docs.pipecat.ai/pipecat/fundamentals/metrics) | TTFB/TTFA/TTFAT decomposition |
| [T50] | [LiveKit observability](https://docs.livekit.io/deploy/observability/data/) | EOU/STT/LLM/TTS/interruption metrics |
| [T54] | [Novofon onboarding](https://novofon.com/instructions/novofon/onboarding-novofon/) | SIP prerequisites; live interoperability unverified |
| [T59] | [Pipecat pipeline](https://docs.pipecat.ai/pipecat/learn/pipeline) | Typed events, priority/control and transport boundary |
| [T60] | [Pipecat context](https://docs.pipecat.ai/pipecat/learn/context-management) | Heard-output context |
| [T61] | [Pipecat function calling](https://docs.pipecat.ai/pipecat/learn/function-calling) | Tool lifecycle/timeouts/interruption |
| [T62] | [Pipecat releases](https://github.com/pipecat-ai/pipecat/releases) | Churn and deprecations |
| [T63] | [LiveKit Agents](https://docs.livekit.io/agents/) | Session lifecycle and transport/job surfaces |
| [T64] | [LiveKit turn handling](https://docs.livekit.io/agents/logic/turns/) | Multiple turn strategies |
| [T65] | [LiveKit testing](https://docs.livekit.io/agents/start/testing/) | Text/audio tools/handoff tests |
| [T66] | [Retell transfer reliability](https://docs.retellai.com/reliability/call-performance) | Transfer trigger vs actual outcome; single publisher |
| [T67] | [OpenAI Realtime Agents guide](https://openai.github.io/openai-agents-python/realtime/guide/) | Interruptions/tools/approvals/handoff pattern |
| [T68] | [MCP Authorization](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization) | OAuth-oriented authorization boundary |
| [T69] | [MCP Tools](https://modelcontextprotocol.io/specification/2025-11-25/server/tools) | Access control, validation, confirmation and audit remain implementation concerns |
| [T70] | [MCP Elicitation](https://modelcontextprotocol.io/specification/2025-11-25/client/elicitation) | Consent is not operation completion |
| [T71] | [Microsoft Foundry RAG](https://learn.microsoft.com/en-us/azure/foundry/concepts/retrieval-augmented-generation) | Retrieval completeness caveat |
| [T72] | [AWS Durable Execution Idempotency](https://docs.aws.amazon.com/durable-execution/patterns/best-practices/idempotency/) | Retry can repeat effects; key persists across attempts |
| [T73] | [OpenAI Prompt Management](https://help.openai.com/en/articles/9824968) | Versions, evals and rollback |
| [T74] | [Anthropic postmortem](https://www.anthropic.com/engineering/a-postmortem-of-three-recent-issues) | Rollback and continuous eval controls |
| [T75] | [Telegram Bot API](https://core.telegram.org/bots/api) | Webhook, secret, IDs, files, business messages |
| [T76] | [Telegram bot basics](https://core.telegram.org/bots) | Initiation constraint |
| [T77] | [Telegram business features](https://core.telegram.org/bots/features) | Business connection, voice/media and reply constraints |
| [T78] | [MAX API](https://dev.max.ru/docs-api) | Auth, IDs, media and documented limits |
| [T79] | [MAX webhooks](https://dev.max.ru/docs-api/methods/POST/subscriptions) | Secret, retry and event surface |
| [T80] | [VK API schema](https://github.com/VKCOM/vk-api-schema) | IDs, attachments/audio and policy errors |
| [T81] | [Jivo Bot API](https://www.jivo.ru/help/api/bot-api.html) | Bot webhook, IDs, invite agent, timeout/retry |
| [T82] | [Jivo Chat API](https://www.jivo.ru/help/api/chat-api.html) | Web/mobile sources, media and operator replies |
| [T83] | [Avito Messenger](https://avito.akamaized.net/business/tools/messenger) | Official access path; technical semantics unverified |
| [T84] | [ABCP interface/content FAQ](https://www.abcp.ru/d/docs/website/faq-interface-and-content) | External script insertion path |
| [T85] | [WhatsApp Business help](https://faq.whatsapp.com/665643701880397/?cms_platform=android&locale=ru_RU) | App availability; Platform eligibility unverified |
| [T86] | [Chatwoot releases](https://github.com/chatwoot/chatwoot/releases) | Current release/churn/security fixes |
| [T87] | [Chatwoot channels/inboxes](https://www.chatwoot.com/hc/user-guide/articles/1784681461-was-ist-ein-kanal-was-ist-ein-posteingang) | Website/Telegram/API channels and assignment |
| [T88] | [Chatwoot AgentBot](https://www.chatwoot.com/hc/user-guide/articles/1788726304-) | Bot↔human handoff states |
| [T89] | [Chatwoot self-hosted](https://www.chatwoot.com/hc/user-guide/articles/1750735898-purchasing-a-paid-self_hosted-chatwoot-license-a-step_by_step-guide) | Self-hosting and operator burden; RU perimeter unverified |
| [T90] | [MAX chat migration](https://dev.max.ru/docs-api/methods/GET/chats) | Chat-ID persistence/dedup |
| [T92] | [MCP tool annotation risks](https://blog.modelcontextprotocol.io/posts/2026-03-16-tool-annotations/) | Annotations are hints, not security enforcement |
