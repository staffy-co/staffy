# Staffy UX — повторная независимая проверка safety и эксплуатации (R2)

Дата: 2026-09-13  
Scope: текущие `DESIGN.md`, `EXPERIENCE.md`, PRD/addendum и четыре promoted key mocks.  
Метод: повторная трассировка F-01…F-14, затем проверка critical/high synthesis и поиск ложного success, authority или promise в ключевых экранах. Spine-файлы не изменялись этим reviewer.

## Итог

**UX/spec готовы к Architecture handoff.** Из 14 исходных safety/operations findings **11 закрыты контрактом**, **3 корректно сохранены как обязательные evidence/architecture gates**, **0 остаются UX-дефектами**. Незакрытые внешние факты не представлены как готовые возможности.

По объединённому critical/high списку первого validation run: **23 из 26 пунктов закрыты на уровне UX/PRD**, а **3 из 26** намеренно остаются fail-closed gates — фактический ABCP/1С путь, State/Authority Matrix и семантика аккаунта Jivo. Это не разрешает live mutation или публикацию соответствующего Connector до PASS.

OQ-10 также остаётся реальным legal/operations gate для публичной подрядной коммерции Montagio, однако исходный UX finding F-08 закрыт: route уже заблокирован и не маскируется общим статусом сотрудника.

## Повторная проверка F-01…F-14

| ID | R2 статус | Проверенное основание |
|---|---|---|
| F-01 | **Закрыто** | Нормативная contractor-state table разносит contract/order, client payment, receipt, subcontractor assignment/payout, acceptance, warranty/claim и margin; direct-master/commission только `closed-validation-only` (`prd.md:1028–1049`). Scope и evidence согласованы (`prd.md:1175, 1185, 1191, 1385, 1398`; `addendum.md:21–25, 45`). |
| F-02 | **Открытый account gate, UX безопасен** | OQ-3/OQ-4 имеют точные моменты и владельцев (`prd.md:1426–1427`; `EXPERIENCE.md:70–71`). Без доказанного fence — ручная проверка, exact re-read, human receipt и reconciliation. Mock не называет approval отправкой, блокирует Staffy supplier-send и разделяет ABCP/1С (`key-autoparts-order-review.html:373–375, 428–479, 485–500`). |
| F-03 | **Обязательный Architecture gate, UX безопасен** | OQ-19 теперь обязательный вход: Owning System, external ID, version/time, projection, conflict, query и terminal evidence (`EXPERIENCE.md:86`; `prd.md:1442`; `addendum.md:90–96`). До матрицы факт `unknown`; key review показывает source/time/version и раздельные ABCP/1С состояния (`key-autoparts-order-review.html:382–405, 428–436, 477–500`). |
| F-04 | **Закрыто** | Consent разделён по purpose и receipt полям; отказ/отзыв останавливает только затронутое использование (`EXPERIENCE.md:148–149, 204, 235`; `prd.md:1294–1304`). |
| F-05 | **Открытый account gate, UX безопасен** | OQ-2 требует account test reply/assignment/ack/writer exclusion/timeout/return и fail-closed fallback (`EXPERIENCE.md:69, 104`; `prd.md:896–903, 1425`). Employee mock маркирует Jivo ограниченным и недоказанным (`key-employee-workspace.html:329–345, 360–361`); Today уточняет, что Jivo/ABCP доступны только в ранее подтверждённых ограниченных режимах (`key-today-owner.html:597–605`). |
| F-06 | **Закрыто** | Ownership имеет `overdue`, `reassignment required`, lease/next update, backup/escalation (`EXPERIENCE.md:173, 177–178, 200, 219`; `prd.md:650–656`). |
| F-07 | **Закрыто** | Pause receipt различает blocked/in-flight/unknown/confirmed и fence time; rollback конфигурации не отменяет внешний side effect (`EXPERIENCE.md:181`; `prd.md:618–623`). |
| F-08 | **Закрыто; OQ-10 остаётся public gate** | Route lock `public_contractor_order=intake_only/no_promises` явно блокирует public contract/payment/receipt/warranty (`EXPERIENCE.md:77`; `prd.md:1175, 1185, 1433`). Gate видим на Today (`key-today-owner.html:554–569`) и в customer continuation без CTA заказа/оплаты (`key-montagio-continuation.html:414–476`). |
| F-09 | **Закрыто** | Closed direct-master validation только staff-mediated, без customer self-service purchase и с правдивыми сторонами (`EXPERIENCE.md:77`; `prd.md:1185, 1433`; `addendum.md:21–25`). Protected mock не создаёт транзакцию и прямо говорит, что order/payment недоступны (`key-montagio-continuation.html:463–476, 524–533`). |
| F-10 | **Закрыто** | Delegated Task контракт включает `assigned → delivered(if proven) → acknowledged → approved/blocked/expired`, deadline, backup и escalation (`EXPERIENCE.md:173`). Mock показывает принятого owner, срок, резерв и receipt принятия; supplier path зависит от проверки (`key-autoparts-order-review.html:441–468`). |
| F-11 | **Закрыто** | Media имеет scanning/usable/correction/quarantine/reject и равноценный text/human path (`EXPERIENCE.md:183, 210`; `prd.md:1316`). Protected mock показывает optional upload, scanning, rejection, remove/replace/no-photo/human (`key-montagio-continuation.html:438–460, 497–514`). |
| F-12 | **Закрыто** | Hard cap переводит новые обращения в configured human/callback/intake-only, не обрывает active Commitment/safety/claim/Recovery и создаёт owner alert/receipt (`EXPERIENCE.md:187, 211`). Today отображает spend, limit и forecast без ложного billing outcome (`key-today-owner.html:527–531, 582–588`). |
| F-13 | **Закрыто** | Универсален exact-version material Confirmation; два customer confirmations только в Domain Pack route, где readiness создаёт новую версию; manager approval отдельно (`EXPERIENCE.md:47, 179, 293, 306`; `addendum.md:72–76`). |
| F-14 | **Закрыто** | Disclosure обновляется при новом/неподтверждённом participant, outbound callback и начале записываемого сегмента (`EXPERIENCE.md:148`). Protected mock называет ИИ и Tenant до продолжения (`key-montagio-continuation.html:406–422`). |

## Critical/high synthesis

- Machine-readable design contract теперь содержит colors, typography, rounded, spacing и 26 component mappings (`DESIGN.md:1–83`); warning/control/container semantic tokens разделены.
- Surface × state contract добавлен для Today, Employee, Work, Integrations, Results, Organization и Protected Continuation (`EXPERIENCE.md:213–223`); canonical Conversation Mode восстановлен (`EXPERIENCE.md:199`).
- Четыре key screens созданы и внесены в coverage ledger (`EXPERIENCE.md:265–276`). На R2 устранены три найденных рассогласования mock-данных: Camry year, двусмысленный completed connection step и безусловное «Jivo/ABCP работают»; в delegated-task mock добавлены deadline/backup/ack evidence.
- Accessibility high contract items присутствуют в gate, reflow 320, overlay/focus, voice alternatives, step-up, accommodations, VIN/OEM readback, expiry/resume и release evidence (`EXPERIENCE.md:225–239`). Детальный accessibility verdict остаётся за отдельным R2 reviewer.

## Проверка четырёх mocks на ложные обещания

- **Today:** заказ Сергея назван клиентским заказом ABCP и отдельно — не отправленным поставщику; Montagio имеет видимый public-order lock; Connector limitation локализована (`key-today-owner.html:516–524, 538–569, 597–605`).
- **Employee/readiness:** configuration/demo не подменяют account capability PASS; Mango, Jivo, ABCP/1С показывают ограничения, fallback и открытые gates (`key-employee-workspace.html:317–369`).
- **ABCP review:** customer Confirmation, manager receipt, supplier-send и ABCP→1С terminal state разнесены; mismatch инвалидирует approval и запрещает blind retry (`key-autoparts-order-review.html:412–500`).
- **Montagio continuation:** tenant/AI identity явны; UI собирает данные и preliminary estimate, но не создаёт public Order, payment, receipt, warranty или visit promise; recovery states не теряют Case (`key-montagio-continuation.html:414–476, 482–533`).

Новых ложных success, authority или commercial promises после исправлений R2 не обнаружено.

## Оставшиеся обязательные решения

1. **OQ-2 / Jivo** — закрыть account-level controlled test до публикации Connector.
2. **OQ-3/OQ-4 / ABCP→1С** — доказать фактические права, supplier fence, correlation и reconciliation до mutating/end-to-end tests; до этого только описанный manual-with-audit путь.
3. **OQ-19** — Architecture обязана создать применимую State/Authority Matrix до storage ownership/low-level design.
4. **OQ-10** — Legal/Finance/Operations обязаны дать PASS до публичного подрядного оффера и первого публичного оплаченного Order Montagio.

## Final readiness verdict

**PASS для передачи в Architecture с явными gates; FAIL для live mutation/public commerce до соответствующих PASS.** Architecture не вправе трактовать этот handoff как доказательство Jivo, ABCP/1С или OQ-10. Требуемое поведение до доказательств уже fail-closed и показано в UX.
