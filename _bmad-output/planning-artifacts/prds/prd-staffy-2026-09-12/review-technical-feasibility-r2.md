---
title: 'Reviewer Gate Round 2 — Technical Feasibility Re-review'
status: complete
created: '2026-09-13'
reviewed_artifacts:
  - prd.md
  - delivery-slices.md
  - addendum.md
  - review-technical-feasibility.md
focus: 'TF-01–TF-07 closure and Architecture handoff'
---

# Technical Feasibility Re-review

## Gate verdict

**PASS TO ARCHITECTURE WITH EXPLICIT PRE-DESIGN GATES.** Неразрешённых Critical/High-дефектов PRD не осталось. TF-02 и TF-03 исправлены в требованиях и delivery contract; TF-01 корректно не «выдуман» в PRD, а превращён в обязательный Architecture deliverable до фиксации component/storage design. TF-04–TF-07 закрыты требованиями либо переданы в Architecture с проверяемым сроком.

Это разрешает начать Architecture workflow. Оно не разрешает начинать low-level storage design или первый изменяющий controlled test, пока соответствующие gates ниже не материализованы.

## Severity counts текущего состояния

| Severity | Количество |
|---|---:|
| Critical | 0 |
| High | 0 |
| Medium | 1 |
| Low | 1 |

## Повторная проверка TF-01–TF-03

| ID | Статус | Проверенное закрытие | Остаточное обязательство |
|---|---|---|---|
| TF-01 — два источника истины | **Properly gated; больше не PRD blocker** | FR-10 теперь требует единственный Owning System для каждого изменяемого материального поля и ограничивает Staffy разрешёнными snapshot/derived/orchestration states (`prd.md:382–390`). State/Authority Matrix внесена в обязательные handoff-решения до component/storage design (`addendum.md:80–86`) и назначена владельцам со сроком до storage ownership/low-level design (`prd.md:1418`). | Architecture должна фактически создать две матрицы — ABCP→1С и Montagio — до схемы хранения. Формулировка gate достаточна; самой матрицы в PRD быть не обязано. |
| TF-02 — Capability contract и идемпотентность | **Resolved** | FR-7 перечисляет schema/version, scopes, tenant binding, error model, delivery guarantee, reconciliation и limits (`prd.md:349–357`); FR-8 добавляет account/environment evidence и expiry (`prd.md:359–367`). FR-29 теперь гарантирует один Staffy intent, запрещает blind retry без idempotency/correlation+lookup и допускает manual unknown (`prd.md:576–584`). FR-30 отдельно обрабатывает partial multi-side-effect outcome (`prd.md:586–594`), а FR-56 применяет эту семантику к ABCP (`prd.md:837–844`). | Нужна конкретная contract-test matrix до включения каждой изменяющей Capability; см. R2-01. Это acceptance detail, а не скрытое противоречие ядра. |
| TF-03 — слишком широкий MVP без walking skeleton | **Resolved** | PRD явно запрещает трактовать 85 FR/32 NFR как отдельные одновременно создаваемые подсистемы и задаёт первые пути UJ-A1 + UJ-M1/M2 (`prd.md:1181–1185`). `delivery-slices.md:3–31` назначает диапазонам FR/NFR первый gate, delivery mode и минимальное доказательство; `delivery-slices.md:33–41` задаёт outcome order и запрещает новую подсистему без наблюдаемой необходимости. Addendum делает Delivery Slices обязательным Architecture input (`addendum.md:80–86`). | Архитектура должна сохранить трассировку slice→component/external system/manual runbook, но scope-контракт для этого уже достаточен. |

## Spot-check TF-04–TF-07

| ID | Статус | Проверенное закрытие |
|---|---|---|
| TF-04 — недоказуемый heard output | **Resolved** | NFR-31 различает `generated`, `sent_to_provider`, `playback-acknowledged/estimated`, `customer-confirmed`, прямо запрещает считать provider playback доказательством понимания и требует semantic repair/readback (`prd.md:1340–1342`). |
| TF-05 — отсутствующий load envelope | **Resolved / gated** | NFR-13 требует Pilot Load Envelope и прогон с safety factor до controlled stage, включая admission/degraded outcome (`prd.md:1254–1262`); Addendum закрепляет его как обязательный Architecture handoff (`addendum.md:80–86`); delivery gate отражён в `delivery-slices.md:27`. |
| TF-06 — неполный release manifest | **Resolved / gated** | Action Receipt включает immutable execution/release manifest (`prd.md:596–603`); NFR-28 перечисляет Tenant config, pack/policy/eval, connector contract+adapter, model/media route и runtime build, а также tenant-scoped rollback и обработку старых событий (`prd.md:1322–1328`). Gate отражён в `delivery-slices.md:30` и `addendum.md:86`. |
| TF-07 — cross-tenant learning boundary | **Resolved / gated** | NFR-15 делает исходные Conversations/media/receipts Tenant-private и разрешает общий regression asset только как минимизированную/синтетическую производную с provenance/reviewer/purpose и без cross-tenant retrieval (`prd.md:1264–1272`). Addendum требует закрепить эту границу в Architecture (`addendum.md:86`). |

## Остаточные находки

### R2-01 — Medium: evidence для Connector contract test ещё не покрывает отрицательные delivery-сценарии

**Где:** `delivery-slices.md:8`; связано с `prd.md:576–594` и `addendum.md:83`.

**Условие.** Gate FR-7–FR-8 требует только «account-level contract test». Он не перечисляет обязательные отрицательные сценарии, из которых следует корректность новой delivery guarantee: timeout до dispatch, timeout после remote commit, duplicate/reordered webhook, конфликт ключа и partial multi-side-effect completion.

**Почему это не High.** Требуемое безопасное поведение уже однозначно задано FR-29/FR-30, а Connector/Capability contract обязан быть закрыт до design. Пропущен не инвариант, а явная полнота приёмочной матрицы.

**Gate.** Architecture/QA должны включить эти сценарии в contract-test evidence до первой изменяющей Capability controlled-пилота. Публикация Capability блокируется, если конкретный сценарий недоступен или не имеет безопасного manual-unknown/reconciliation результата.

### R2-02 — Low: Delivery Slices снова называет P7 обычным policy outcome

**Где:** `delivery-slices.md:19`.

**Условие.** Минимальное доказательство указано как «P1–P7», тогда как каноническая модель определяет P1–P6 как Policy Outcomes, а P7 — как приоритетный Safety Interrupt (`prd.md:83`, `prd.md:505–513`; `addendum.md:64`).

**Последствие.** Тест-каталог или архитектурная state machine могут ошибочно реализовать семь равноправных outcome-маршрутов, хотя Non-Goals это запрещают (`prd.md:1138–1145`).

**Исправление.** При следующей редакции заменить доказательство на «P1–P6 + P7 Safety Interrupt». До этого каноническими остаются PRD и addendum; Architecture не должна создавать отдельный P7 workflow.

## Итоговая граница допуска

- **Architecture workflow:** разрешён.
- **Component/storage design:** только после готовой State/Authority Matrix из TF-01.
- **Первый mutating controlled test:** только после capability-specific negative contract-test matrix из R2-01 и подтверждения фактических аккаунтов/прав.
- **Controlled voice:** только после Pilot Load Envelope и реального provider route test.
- **Новых Critical/High исправлений в PRD перед передачей:** не требуется.

## Редакционная проверка

Документы образуют подходящую для Architecture аудиторию пирамиду: PRD задаёт инварианты и gates, `delivery-slices.md` — компактный scope-контракт, Addendum — обязательные технические решения без преждевременного component design. Структурного переноса или сокращения, влияющего на gate, не требуется. Единственная терминологическая правка — R2-02.
