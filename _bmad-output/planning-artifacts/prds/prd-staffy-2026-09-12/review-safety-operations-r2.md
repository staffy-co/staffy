# Safety & Operations Reviewer Gate — Round 2

**Дата:** 2026-09-13  
**Объект:** актуальные `prd.md`, `delivery-slices.md`, `addendum.md`; повторная проверка замечаний SO-01–SO-13 из `review-safety-operations.md`.  
**Вердикт:** **PASS WITH NON-BLOCKING EDITORIAL CLEANUP**. Все прежние Critical/High blockers устранены без обнаруженных новых противоречий. До реального запуска по-прежнему должны быть пройдены уже заданные в PRD account-, provider-, legal/payment- и operational evidence gates; это условия исполнения, а не пробелы документа.

## Итог по серьёзности

| Severity | Открыто сейчас |
|---|---:|
| Critical | 0 |
| High | 0 |
| Medium | 1 |
| Low | 0 |

Закрыто полностью: **12 из 13** замечаний. Одно замечание закрыто по существу, но требует редакционного выравнивания формулировки.

## Повторная проверка SO-01–SO-06

| ID | Статус | Проверенное исправление | Проверка на противоречия |
|---|---|---|---|
| **SO-01** | **Resolved** | PRD FR-28 прямо инвалидирует или требует повторной авторизации всех queued/in-flight intent при Safety Interrupt, cancellation, `privacy/no_contact`, Human Handoff и `paused`; FR-32 требует проверять новое поколение scoped control непосредственно перед dispatch и останавливать/сверять старые intent (`prd.md`:565–574, 605–613). | Delivery slice FR-27–FR-33 закрепляет это как productized invariant до первого mutating action, а не как необязательную автоматизацию (`delivery-slices.md`:11). Противоречий нет. |
| **SO-02** | **Resolved** | Execute-time revalidation теперь охватывает Conversation Mode, Response Owner, control generation, полномочия, Policy/config version, Confirmation и актуальные факты Owning System; material delta требует новой Confirmation (`prd.md`:565–574). | Не конфликтует с ручными pilot-controls: вводная delivery slices запрещает manual-with-audit отменять авторизацию, Confirmation, Receipt и fail-closed (`delivery-slices.md`:3, 11). |
| **SO-03** | **Resolved** | FR-71 требует атомарную reservation по `исполнитель × capability/service × окно × capacity`, expiry и освобождение при отказе, timeout, отмене или переносе; одновременно допускается не более одного назначенного исполнителя (`prd.md`:979–987). | Delivery slice FR-69–FR-74 сохраняет assignment/capacity invariants productized, разрешая вручную лишь supply/dispatch (`delivery-slices.md`:20). Противоречий нет. |
| **SO-04** | **Resolved** | FR-57 связывает approval с точной версией Order, Work Items и evidence; изменение позиции, количества, цены, автомобиля/VIN или evidence инвалидирует approval; supplier formation/send доступен только текущей одобренной версии. До первого live mutating action обязателен account-level test отсутствия обходного пути ABCP (`prd.md`:846–855). | Delivery slice требует productized supplier fence, exact-version receipt и доказательство невозможности обхода supplier-send (`delivery-slices.md`:17). Это закрывает не только UI-процесс, но и enforcement boundary. |
| **SO-05** | **Resolved** | FR-11 устанавливает риск-ориентированную проверку личности и полномочий до защищённого раскрытия, связывания каналов и материального действия; Caller ID/display name/channel access недостаточны, делегирование ограничивается ролью, предусмотрены step-up/Handoff и безопасный fallback (`prd.md`:392–401). | Требование входит в productized controlled slice FR-9–FR-17 (`delivery-slices.md`:9). Противоречий нет. |
| **SO-06** | **Resolved** | FR-30 теперь покрывает частично завершённую многошаговую операцию: per-side-effect state/correlation, запрет повторного исполнения уже завершённого эффекта, проверка каждого Owning System и reconciliation к completion/разрешённой compensation/human resolution (`prd.md`:586–594). | Delivery slice фиксирует единый цикл `authorize/revalidate/fence → intent → side effect → Receipt/reconcile` (`delivery-slices.md`:11). Addendum требует до component/storage design зафиксировать State/Authority Matrix и Connector/Capability contract с delivery guarantee, query/reconcile/cancel/compensate (`addendum.md`:80–86). Противоречий нет. |

## Spot-check SO-07–SO-13

| ID | Статус | Результат |
|---|---|---|
| **SO-07** | **Resolved** | Потеря доступности после acknowledgement теперь нарушает ownership lease и запускает reassignment/escalation с честным обновлением клиента и без двух Response Owner (`prd.md`:637–645); Handoff slice требует acknowledgement и fallback/reassignment (`delivery-slices.md`:12). |
| **SO-08** | **Resolved** | Задача проверки автозапчастей имеет `assigned`, `acknowledged`, `approved/blocked`, `expired`, owner/backup, deadline и refresh устаревших цены/наличия/evidence (`prd.md`:857–865). |
| **SO-09** | **Resolved** | Отмена/no-show/потеря допуска исполнителя инвалидирует fulfilment Commitment, уведомляет клиента, запускает Recovery/fallback и требует нового Confirmation для замены (`prd.md`:989–997). |
| **SO-10** | **Resolved** | Денежный контур различает due, paid, partially paid, disputed, reversed/refunded, written-off/reconciled; margin остаётся provisional до settlement/claim и пересчитывается при reversal/refund (`prd.md`:1017–1025). |
| **SO-11** | **Resolved** | Privacy request теперь имеет identity proof, controller/processor owner, охват provider copies/backups, legal-hold conflict handling и completion evidence (`prd.md`:1274–1276). |
| **SO-12** | **Resolved** | Media проходит type/size/content validation и safe render/quarantine; ссылки ограничены Tenant/Case/ролью, имеют expiry/revocation/audit/retention deletion (`prd.md`:1290–1292). |
| **SO-13** | **Substantively resolved; Medium editorial cleanup** | Нормативный MVP scope однозначен: supply drill и Stage-0 legal/payment tabletop **до live**, paid micro-cohort **до scale** (`prd.md`:1167); delivery slices согласованы с этим (`delivery-slices.md`:19–21). Однако SM-17 всё ещё говорит «до live/scale перехода» для первых двух упражнений (`prd.md`:1374). Это не меняет gate благодаря более точной §6.1, но создаёт ненужную двусмысленность в метрике. Заменить на «до live». |

## Gate conclusion

- **Неразрешённых Critical/High: нет.**
- Ранее блокировавшие безопасность и операции механизмы теперь сформулированы как проверяемые продуктовые инварианты и привязаны к этапам поставки.
- Разрешённые ручные операции пилота не обходят authorization, Confirmation, one-owner, Action Receipt, fail-closed, supplier/capacity fences и measurement (`delivery-slices.md`:3, 11–25).
- Архитектурные детали не подменяют требования: обязательные State/Authority Matrix и Connector contract корректно оставлены handoff-решениями до component/storage design (`addendum.md`:80–86).
- Единственное требуемое действие перед финальной редакционной фиксацией — синхронизировать SM-17 с §6.1 заменой «до live/scale» на «до live».
