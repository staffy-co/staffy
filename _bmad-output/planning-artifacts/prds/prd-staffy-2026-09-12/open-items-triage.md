# Open Items Triage — PRD Staffy

Дата: 2026-09-13.

## Итог

Ни один открытый вопрос не блокирует завершение PRD или старт UX/Architecture/Epics: неизвестные Capabilities и полномочия fail-closed, а этапные допуски названы. Вопросы ниже блокируют не планирование вообще, а конкретный design/live/scale/commercial gate.

| Категория | Вопросы PRD | Статус и условие возврата |
|---|---|---|
| Architecture pre-design | OQ-19 | Обязательный State/Authority Matrix создаётся внутри Architecture до фиксации storage ownership и low-level design. Architecture можно начать, завершить этот блок без матрицы нельзя. |
| Oleg connector/controlled | OQ-1–OQ-5 | Закрыть владельцами фактических аккаунтов до соответствующего Connector design, contract test и первого mutating controlled action. |
| Pilot commercial/availability | OQ-6, OQ-13 | Закрыть Product Owner + Tenant до платного обязательства/SLA. |
| Montagio live/scale | OQ-7–OQ-10, OQ-14 | Целевая публичная модель Montagio-подрядчика принята 2026-09-13, но OQ-10 остаётся открытым до юридической, бухгалтерской, кассовой, страховой и операционной проверки первых 3–5 стандартизированных услуг. Закрытая проверка спроса по прямой модели клиент↔мастер не разрешает скрывать фактические договор/оплату/чек/гарантию. Остальные вопросы закрыть в Operating Profile, Gate Definition и supply/evidence до указанного live/scale gate. |
| Data/voice live | OQ-11–OQ-12 | Закрыть Security/Privacy и Voice/Technical до production ПДн/записи и live voice. |
| Product commercialization | OQ-15 | Конкретные цены и требуемый уровень market validation определить после полной себестоимости пилотов, но до повторяемого коммерческого предложения. |
| Deferred product/privacy | OQ-16 | Cross-case Customer Profile не входит в MVP; вернуться до проектирования персонализации. |
| Third-client operator surface | OQ-17 | Architecture выбирает реализацию до запуска внешнего Tenant без поддерживаемого Inbox; FR-83 остаётся provider-neutral. |
| Roadmap channels/providers | OQ-18 | Не влияет на два пилота; вернуться до roadmap/разработки каждого Connector. |

Все владельцы и точные revisit conditions указаны непосредственно в §10 PRD. Никакой вопрос не считается молча решённым.
