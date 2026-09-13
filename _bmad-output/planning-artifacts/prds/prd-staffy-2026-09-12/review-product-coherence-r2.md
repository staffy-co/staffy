# Reviewer Gate — Product coherence round 2

Дата: 2026-09-13  
Артефакты: `prd.md`, `delivery-slices.md`, `addendum.md`  
Предыдущий review: `review-product-coherence.md`

## Gate verdict

**PASS — PC-01, PC-02 и PC-03 устранены; unresolved Critical/High findings нет.**

PRD теперь достаточно однозначно защищает доказательность когорт, требует независимого внешнего третьего клиента и отделяет обязательные инварианты от допустимой ручной работы по этапам. PC-04 остаётся осознанным Medium-level commercial deferral, который не блокирует UX/Architecture/Epics; PC-05–PC-07 закрыты.

## Severity summary

| Severity | Count |
|---|---:|
| Critical | 0 |
| High | 0 |
| Medium | 1 |
| Low | 1 |

## Блокеры PC-01–PC-03

### PC-01 — resolved

FR-84 теперь требует до оценки результатов версионированные Operating Profile и Gate Definition с eligibility/intent-to-treat denominator, assignment человеческой группы, формулами, исключениями, минимальными подкогортами, approvers и Coverage Matrix (`prd.md:1114–1121`). SM-1 задаёт предварительную фиксацию обязательного представительства voice/text, journeys, Autonomy Classes и материальных failure cases; SM-2 прямо запрещает заменить UJ-M2 или голосовой путь удобным большинством другого источника и определяет denominator/crossover/matching (`prd.md:1352–1353`). Один cross-channel Case считается одним наблюдением, а SM-C13/SM-C14 закрывают gaming через массовый Handoff и агрегатное маскирование route failure (`prd.md:1393–1394`).

**Вывод:** исходная возможность формально пройти пилот нерепрезентативной выборкой или почти полным Handoff устранена. Конкретные числовые thresholds могут оставаться baseline-derived, потому что правила выборки и момент заморозки теперь обязательны заранее.

### PC-02 — resolved

Видение, FR-49 и SM-4 согласованно требуют третьего **независимого внешнего** бизнеса/Tenant с собственным администратором, реальными входами и доступами, не участвовавшего в создании reference packs (`prd.md:27`, `prd.md:770–778`, `prd.md:1355`). Внутренний или синтетический Tenant явно считается rehearsal; proof требует публикации и controlled end-to-end exercise. Та же семантика сохранена в `delivery-slices.md:15`.

**Вывод:** внутренний демонстрационный стенд больше не может пройти platform proof.

### PC-03 — resolved

Новый §6.3 объявляет Delivery Slices scope-контрактом, задаёт первый deployable walking skeleton и прямо говорит, что guided setup UI, универсальная аналитика и автоматизированный billing не блокируют первые controlled paths при сохранении инвариантов (`prd.md:1181–1185`). `delivery-slices.md` назначает каждому диапазону FR/NFR earliest gate, delivery mode и минимальное доказательство; отдельно разрешены `connector/configuration` и `manual-with-audit`, но не ослаблены Confirmation, authorization, one-owner, Action Receipt и fail-closed.

**Вывод:** 85 FR и 32 NFR больше не выглядят одним программным блоком до первого пилота; принятую свёртку до safe intake → safe promise → safe fulfilment → safe close/recovery → business proof можно последовательно реализовать и проверить.

## Spot-check PC-04–PC-07

### PC-04 — partially resolved; Medium residual

§7 и SM-19 теперь вводят честную лестницу `interest/design partner → paid pilot → continued paid use/expansion → repeatable external sale`, требуют раскрывать скидку/полную себестоимость и запрещают выдавать внутренний Montagio или технический success за доказанный внешний спрос (`prd.md:1198–1200`, `prd.md:1376`). Это закрывает основную возможность завысить уровень доказательства.

Остаётся один осознанный пробел: сам §7 говорит, что требуемый уровень для Definition of Done первой версии остаётся Open Question 15, причём срок решения — до повторяемого коммерческого предложения после пилотов (`prd.md:1200`, `prd.md:1414`). Это приемлемо для текущей передачи в UX/Architecture/Epics, но до объявления первой версии завершённой Product Owner должен выбрать минимальный требуемый уровень.

**Residual finding — Medium:** коммерческий Definition of Done ещё не выбран. *Required follow-up:* закрыть Open Question 15 до утверждения release/market-validation claim; более низкий уровень доказательства продолжать маркировать буквально.

### PC-05 — resolved

UJ-P1 добавляет pre-access capability-fit, packaging и no-fit путь (`prd.md:55–63`), UJ-P2 сохраняет activation/setup. FR-85 определяет Entitlement `draft/active/paused/ended`, ручной договор/счёт и активацию для MVP, изменения каналов/лимитов, завершение пилота и usage dispute (`prd.md:1124–1134`); `delivery-slices.md:25` фиксирует это до платного live-пилота.

### PC-06 — resolved

§1.4 задаёт проверяемый launch ICP: выручкообразующие обращения, повторяемый нетривиальный процесс, достаточный поток, готовые pack/connectors и Owning System, human owner/fallback, реальные доступы и готовность к paid/controlled evidence (`prd.md:51–53`). Выбор только по лёгкости подключения явно не считается доказательством ICP.

### PC-07 — resolved

Addendum больше не называет два подтверждения неподтверждённым предложением: исторический статус отделён от принятого решения и прямо сказано, что два Confirmation закреплены в PRD (`addendum.md:27`).

## Other coherence note

- **Low** P7 снова назван как обычный outcome в одном delivery-slice (`delivery-slices.md:19`) — строка требует доказательство `P1–P7`, тогда как PRD и Addendum последовательно определяют P1–P6 как Policy Outcomes, а P7 как приоритетный Safety Interrupt. Это не меняет gate благодаря FR-22/FR-66, но создаёт локальную терминологическую неоднозначность. *Fix:* заменить `P1–P7` на `P1–P6 + Safety Interrupt P7`.

## Final gate statement

Unresolved Critical: **0**. Unresolved High: **0**. Product-coherence gate можно считать пройденным; Medium commercial DoD должен остаться явным tracked deferral, Low P7 wording — редакционная правка до финального polish.
