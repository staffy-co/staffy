# Референсные UX-паттерны для Staffy

Дата проверки: 2026-09-13.

## Граница использования

Референсы используются только для извлечения общих паттернов взаимодействия. Staffy не копирует чужие макеты, иллюстрации, фирменные цвета, тексты, названия функций, иконки или композицию экранов. Финальная система должна иметь собственную визуальную идентичность и проверяться на применимость к российским каналам, провайдерам и требованиям к данным.

## Что стоит перенять как принцип

1. **Единый жизненный цикл `собрать → проверить → опубликовать → наблюдать → улучшить`.** Retell прямо организует продукт вокруг Build, Test, Deploy и Monitor; ElevenLabs — вокруг Configure, Deploy и Monitor. В Staffy эти стадии должны быть видны владельцу бизнеса как состояние AI‑сотрудника, а не как набор технических меню.
   - [Retell AI: platform introduction](https://docs.retellai.com/general/introduction)
   - [ElevenLabs: ElevenAgents overview](https://elevenlabs.io/docs/eleven-agents/overview)

2. **Начинать с результата на обычном языке.** Zapier позволяет сначала описать задачу агента, затем предлагает нужные подключения и только после этого ведёт к настройке и публикации. Для Staffy это подтверждает путь «какую работу должен выполнять сотрудник?» до запроса доступов.
   - [Zapier: Build an agent](https://help.zapier.com/hc/en-us/articles/24393442652557-Build-an-agent-in-Zapier-Agents)

3. **Два уровня сложности без двух разных продуктов.** Salesforce сочетает редактирование на естественном языке с более точным Script‑представлением; Decagon описывает процедуры агента естественным языком, сохраняя инженерный контроль и версионирование. В Staffy владелец бизнеса должен видеть понятные обязанности, ограничения и результаты, а специалист — раскрываемые политики, Capability и трассировку.
   - [Salesforce: Explore Agentforce Builder](https://help.salesforce.com/s/articleView?id=ai.agent_builder_tour.htm)
   - [Decagon: Agent Operating Procedures](https://decagon.ai/product/aop)

4. **Preview должен объяснять не только ответ, но и выполненные действия.** Agentforce разделяет понятное резюме взаимодействия и подробную трассировку; ElevenLabs отдельно тестирует полный диалог, следующий ответ и вызов инструмента. Staffy нужен понятный владельцу итог поверх раскрываемых доказательств: источник факта, выбранное правило, действие, результат и причина остановки.
   - [Salesforce: Test in Agentforce Builder](https://help.salesforce.com/s/articleView?id=ai.agent_test_in_builder.htm&language=en_US&type=5)
   - [ElevenLabs: Agent Testing](https://elevenlabs.io/docs/eleven-agents/customization/agent-testing)

5. **Безопасно различать симуляцию и реальное действие.** Salesforce явно разделяет Simulate с тестовыми данными и Live Test, способный изменить реальные записи. Staffy должен ещё заметнее маркировать режимы Preview, Controlled и Live, особенно перед заказом, сообщением, переводом денег или назначением исполнителя.
   - [Salesforce: Agentforce Builder with Agent Script](https://help.salesforce.com/s/articleView?id=005232662&language=en_US&type=1)

6. **Тестировать на реальных формулировках до запуска.** Intercom позволяет брать прошлые обращения, загружать наборы вопросов, оценивать ответы и видеть, какой источник или настройка повлияли на результат. Это подходит Staffy как простая оболочка над eval‑наборами обоих пилотов.
   - [Intercom: Batch Test Fin](https://www.intercom.com/help/en/articles/10521711-batch-test-fin-ai-agent)

7. **Разворачивать по каналам и аудиториям постепенно.** Intercom предлагает отдельный выбор каналов, аудитории, handover и постепенное расширение после анализа. В Staffy публикация должна быть не общей кнопкой «включить всё», а матрицей канал × сценарий × аудитория × уровень автономности.
   - [Intercom: Deploy Fin over chat](https://www.intercom.com/help/en/articles/8286630-deploy-fin-ai-agent-over-chat)
   - [Intercom: Live testing with segments](https://www.intercom.com/help/en/articles/7837527-live-testing-fin-with-your-team-or-customer-segments)

8. **Не допускать двойного владельца разговора.** Intercom отделяет bot inbox и прекращает автоматизацию после передачи в командный inbox. Для Staffy это подтверждает явные состояния Response Owner и запрет одновременных ответов AI и человека.
   - [Intercom: Bot inbox](https://www.intercom.com/help/en/articles/3722087-turn-on-the-bot-inbox)

9. **Интеграции показывать как способности бизнеса, а не только подключения.** Zapier связывает приложения с действиями; Salesforce сводит subagents, actions, variables, connections и data в обозримый Explorer. Staffy должен после подключения показывать: что AI может читать, что может изменять, где требуется подтверждение, где есть ручной fallback и прошла ли проверка.
   - [Zapier: Build an agent](https://help.zapier.com/hc/en-us/articles/24393442652557-Build-an-agent-in-Zapier-Agents)
   - [Salesforce: Explore Agentforce Builder](https://help.salesforce.com/s/articleView?id=ai.agent_builder_tour.htm)

10. **Голос проверять отдельно от текста.** Retell различает текстовый playground, симуляции и реальные web/phone‑звонки с проверкой задержки, перебиваний, шума и DTMF; ElevenLabs даёт живой мониторинг и вмешательство. Staffy не должен выдавать успешный текстовый тест за готовность телефонного канала.
    - [Retell AI: Testing overview](https://docs.retellai.com/test/test-overview)
    - [ElevenLabs: Real-time monitoring](https://elevenlabs.io/docs/eleven-agents/guides/realtime-monitoring)

11. **Версии и среды должны быть видимы.** Salesforce фиксирует неизменяемую опубликованную версию, ElevenLabs позволяет отделять среды и привязывать тестовый номер к нужной ветке. В Staffy владелец должен ясно видеть «Черновик», «Проверяемая версия», «Опубликовано», отличия и доступный откат.
    - [Salesforce: Explore Agentforce Builder](https://help.salesforce.com/s/articleView?id=ai.agent_builder_tour.htm)
    - [ElevenLabs: Environment variables](https://elevenlabs.io/docs/eleven-agents/integrate/environment-variables)

## Антипаттерны для Staffy

- Один огромный prompt как основной интерфейс настройки.
- Технические сущности `tools`, `MCP`, модели и webhooks раньше бизнес‑результата.
- Общая кнопка запуска без видимого охвата каналов, полномочий и реальных действий.
- Успешный демонстрационный чат как доказательство production‑готовности.
- Диаграмма workflow как единственный способ понять поведение сложного сотрудника.
- Скрытая передача AI ↔ человек или два одновременно отвечающих владельца.
- Зелёный статус интеграции только потому, что credential сохранён: нужна проверка реальной Capability.
- Копирование узнаваемого визуального языка конкурента вместо собственной системы Staffy.

## Предварительное направление Staffy

Не «конструктор ботов» и не «панель разработчика», а **рабочее место владельца AI‑сотрудников**. Главная сущность — сотрудник и обещанный им бизнес‑результат; знания, каналы, интеграции, полномочия, тесты и метрики раскрываются как условия его готовности.
