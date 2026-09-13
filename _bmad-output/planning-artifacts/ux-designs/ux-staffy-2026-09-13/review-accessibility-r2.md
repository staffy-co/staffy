# R2 — независимая повторная проверка доступности Staffy UX

Дата: 2026-09-13  
Объект: текущие `DESIGN.md`, `EXPERIENCE.md`, четыре promoted key-screen mock и исходный `review-accessibility.md`.  
Нормативный уровень контракта: WCAG 2.2 AA.

## Вердикт

**PASS для UX-контракта; не является подтверждением соответствия готового продукта WCAG 2.2 AA.** Все прежние High-, Medium- и Low-дефекты устранены в spine. Обязательные проверки работающей реализации остаются admission/release gates и не должны ошибочно считаться доказанными документацией или статичными макетами.

Четыре ключевых макета действительно перестраиваются при ширине 320 CSS px без горизонтальной прокрутки страницы. Они дают полезное визуальное evidence для protected continuation, weak network, expiry, media scanning/rejection, accommodations, step-up и операционного review. При этом mock не следует считать доказательством screen-reader, voice/DTMF, text-spacing, forced-colors или overlay поведения.

### Счётчик текущих находок

| Тип | Critical | High | Medium | Low |
|---|---:|---:|---:|---:|
| Открытые дефекты UX-документов | 0 | 0 | 0 | 0 |
| Открытые отклонения в non-production mock | 0 | 0 | 0 | 0 |

Disposition прежних 21 находок: **21 закрыта как дефект контракта**. Проверки готовой реализации перечислены отдельно и не считаются открытыми дефектами документа.

## Перепроверка прежних High

| ID | Статус в UX-контракте | Визуальное evidence | Что остаётся gate реализации |
|---|---|---|---|
| A11Y-H1 Reflow | **Закрыт.** Все web surfaces обязаны работать при 320 CSS px без page-level horizontal scroll; для двумерных данных задан локальный scroll и linear/card equivalent (`EXPERIENCE.md:227–229`, `245–247`). | **Есть частично.** Today, Employee, ABCP review и Montagio: `scrollWidth ≤ innerWidth` при viewport 320 px. | 400% zoom, все spine-only surfaces, text spacing и focus-not-obscured проверяются на реализации. |
| A11Y-H2 Contrast tokens | **Закрыт.** Разделены `warning-text`, `warning`, `control-border` и on-container пары (`DESIGN.md:139–163`). Контрольные значения: warning-text/surface 8.18:1; warning graphic/surface 4.31:1; control-border/surface 4.41:1; focus/surface 4.81:1; container pairs 7.72–11.26:1. | **Есть для основной палитры.** Макеты используют semantic пары и не передают load-bearing state одним цветом. | Полная token × background × state matrix, forced-colors и browser-rendered contrast входят в release evidence. |
| A11Y-H3 Focus/overlays | **Закрыт.** Заданы initial focus, modal-only trap/inert, Escape, return focus, запрет focus steal и перекрытия sticky/keyboard (`EXPERIENCE.md:227–228`). | **Ограниченно.** Во всех key mocks есть заметный `:focus-visible`; high-stakes sticky CTA не показана. Modal/overlay сценария среди четырёх экранов нет. | Keyboard walkthrough реальных dialog/drawer/popover, virtual keyboard и 2.4.11. |
| A11Y-H4 Accessible step-up | **Закрыт.** OTP paste/autofill/resend, password manager, запрет puzzle-only CAPTCHA, human/callback и сохранение Case/draft закреплены (`EXPERIENCE.md:233–237`). | **Есть частично.** Montagio mock показывает `autocomplete="one-time-code"`, целую вставку, новый код, human route и сохранённый Case. | Ошибка, resend, re-auth/resume и альтернативный метод должны быть интерактивно проверены. |
| A11Y-H5 Voice alternatives | **Закрыт.** Создана исполнимая Voice Accessibility State Machine: repeat/slower/DTMF/text/human, стабильный mapping, repair limit, no-input/no-match и сохранение safety/Confirmation (`EXPERIENCE.md:151–156`). | **Не визуализируется web-mock по смыслу.** | Телефонные eval: DTMF-only, нарушения речи/слуха, шумная 8 kHz линия, reconnect и фактический transfer/callback receipt. |
| A11Y-H6 Accommodations | **Закрыт.** Case хранит только заявленные настройки коммуникации, передаёт их каждому Response Owner и не выводит диагноз (`EXPERIENCE.md:234–237`). | **Есть.** Montagio mock показывает chat-only, короткие шаги и no-photo path. | Cross-channel/handoff tests должны доказать сохранение и соблюдение предпочтений. |
| A11Y-H7 VIN/OEM readback | **Закрыт.** Domain Rule задаёт нормализацию, алфавит, группировку, омографы, повтор группы, замедление и protected field (`EXPERIENCE.md:153–156`). | **Есть только объект.** ABCP mock показывает нормализованные VIN/OEM и provenance, но не голосовой repair. | Russian voice eval для `О/0`, `I/1`, `B/В`, `H/Н`, длинных VIN/OEM, суммы, даты и адреса. |
| A11Y-H8 Timeout/expiry | **Закрыт.** Предупреждение, доступное продление, saved draft, re-auth/resume и правило «ссылка истекла, Case сохранён» закреплены (`EXPERIENCE.md:232–233`). | **Есть.** Montagio mock показывает weak-network queue, expired/revoked link, сохранённый Case и возврат в исходный канал. | Программное announcement, timing, продление и expiry во время upload/OTP/Confirmation проверяются интерактивно. |
| A11Y-H9 Release gate | **Закрыт.** WCAG 2.2 AA является обязательным Evidence Gate; automated + manual matrix, Critical/High block и regression case записаны (`EXPERIENCE.md:185`, `225–227`). | Не является свойством статичного mock. | Gate должен быть реализован в CI/release process с владельцем, артефактами и pass/fail record. |

## Перепроверка прежних Medium

| ID | Статус | Обоснование / остаток |
|---|---|---|
| A11Y-M1 Semantics/live | **Закрыт как контракт; eval gate.** Native semantics, landmarks/headings, name/role/state/value, table/card alternative и polite/assertive правила заданы (`EXPERIENCE.md:230–232`). Макеты в основном используют landmarks, headings, labels и progressbar, но статичный mock не доказывает live updates. |
| A11Y-M2 Text spacing/clipping | **Закрыт как контракт; eval gate.** Запрещены fixed-height text controls, material ellipsis; WCAG Text Spacing включён в gate (`EXPERIENCE.md:227`, `232`). Фактическая spacing-инъекция и 200/400% нужны на реализации. |
| A11Y-M3 Hover/focus content | **Закрыт как контракт; eval gate.** Material detail — click/keyboard persistent; hover/focus content dismissible, hoverable, persistent (`EXPERIENCE.md:239`). |
| A11Y-M4 Non-visual media | **Закрыт.** Отдельно заданы file/text/inspection/human path и keyboard-/human-assisted redaction (`EXPERIENCE.md:154`, `183`, `234`). Montagio mock показывает no-photo и конкретное rejection. |
| A11Y-M5 Weak-network conflict | **Закрыт как контракт; eval gate.** Recovery обязан сравнивать local/server version и timestamps, current Response Owner/takeover state и отдельный статус позднего upload; material fields не auto-merge, а поздний файл не запускает Action (`EXPERIENCE.md:210`, `223`, `252`). Интерактивный reconnect и reconciliation остаются gate реализации. |
| A11Y-M6 Russian labels | **Закрыт как контракт; eval gate.** Machine IDs не primary copy, есть approved Russian label + consequence и правило `lang`/произношения (`EXPERIENCE.md:238`). Проверка реального SR/голоса для ABCP, VIN, OEM, Jivo, Novofon остаётся gate. |
| A11Y-M7 Consistently findable human help | **Закрыт как контракт; eval gate.** На всех customer web/native-chat surfaces закреплены одинаковые действие и команда «Позвать человека», устойчивое доступное место, фактические owner/acknowledgement/next update, «Вернуться в разговор» и запрет автоматического возврата к ИИ до явного `return` (`EXPERIENCE.md:238`). Voice использует то же название (`EXPERIENCE.md:153`). |
| A11Y-M8 Dense views | **Закрыт как контракт; частичное visual evidence.** Summary-first, semantic table/card equivalent, local scroll и 320 reflow заданы (`EXPERIENCE.md:229`, `231`, `245–247`). ABCP review перестраивает evidence rows в линейное представление на 320 px. |
| A11Y-M9 Recording revoke | **Закрыт как контракт; voice eval gate.** Voice/DTMF revoke, подтверждённый state change, судьба ранее записанного и no-recording continuation записаны (`EXPERIENCE.md:148–149`, `204`). |

## Перепроверка прежних Low

| ID | Статус | Обоснование |
|---|---|---|
| A11Y-L1 Draft visual references | **Закрыт в spine.** Направления и палитры помечены non-production; promoted key screens объявлены иллюстрациями, а не источником правил (`DESIGN.md:239`; `EXPERIENCE.md:262–281`). Небольшие отклонения самих mock перечислены ниже. |
| A11Y-L2 Text equivalent for diagrams | **Закрыт.** IA и все flow diagrams связаны с каноническими таблицами и пронумерованными UJ; spine явно является текстовым эквивалентом (`EXPERIENCE.md:90–104`, `262–304`). |
| A11Y-L3 Orientation/pointer | **Закрыт как контракт; eval gate.** Нет non-essential orientation lock; gesture имеет single-pointer/keyboard equivalent; high-stakes commit отменяем до release/Confirmation (`EXPERIENCE.md:239`). |

## Проверка четырёх key-screen mock при 320 CSS px

| Mock | Viewport / scrollWidth | Evidence | Ограничение evidence |
|---|---:|---|---|
| `key-today-owner.html` | 320 / 320 | Одна колонка, primary action, status+consequence, budget progress, alternate stale state. | Static screen; не доказывает live update, drawer и focus after action. |
| `key-employee-workspace.html` | 320 / 305 | Brief, plan, capabilities, release blockers, permission/stale alternatives; локально прокручиваемая nav не создаёт page overflow. | Не доказывает modal/publish/rollback и AT announcement. |
| `key-autoparts-order-review.html` | 320 / 305 | Exact version, evidence/provenance, owner lifecycle, disabled supplier action, version conflict/reconciliation. | ARIA-like evidence grid не является доказательством корректной production table semantics. |
| `key-montagio-continuation.html` | 320 / 305 | Tenant/AI role, no-photo, preferences, route lock, weak network, scanning/rejection, accessible OTP hint, expiry и human route. | Не доказывает реальную resumable upload, step-up fallback, timer announcement или consent revoke. |

## Повторная проверка non-production mock

Все четыре локальных отклонения устранены и синхронизированы между `.working/` и `mockups/`:

1. В ключевых mock больше нет текста ниже канонического минимума 12 px.
2. Skip links Today и Employee имеют `min-height: 44px`; обычные controls и label hit-area сохраняют 44 px floor.
3. Recovery helper ABCP использует `on-danger-container`, то есть доступную semantic pair вместо недостаточно контрастного muted.
4. Evidence grid ABCP получил `columnheader`; ключевые VIN/OEM/ABCP labels получили языковую разметку. Production по-прежнему обязана предпочитать native semantics и проверяться screen reader, но текущий mock больше не противоречит spine.

## Обязательные implementation/eval gates

Это не открытые дефекты текущего UX-документа; без их evidence нельзя заявлять WCAG 2.2 AA или повышать affected admission scope:

- keyboard + screen reader: NVDA/Chromium/Firefox, VoiceOver/iOS Safari, TalkBack/Android Chrome;
- 200% text resize, 400%/320 CSS px reflow, WCAG Text Spacing, forced-colors/high contrast, focus not obscured;
- accessible overlay, step-up, timer/expiry, saved draft/re-auth/resume и weak-network local/server conflict;
- voice/DTMF/no-input/no-match, Russian VIN/OEM homographs, repeat/slower/text/human and recording revoke;
- media permission denied, no-camera path, upload progress/resume/rejection/redaction и late-upload isolation;
- regression evidence и blocking severity в каждом Demo → Check → Trial → Controlled Work → Work переходе.

## Финальная готовность

Открытых accessibility-дефектов в текущем UX spine и четырёх ключевых non-production mock не осталось. Документы можно финализировать как достаточный accessibility contract и передавать в Architecture. Статус WCAG 2.2 AA присваивается только конкретной реализованной поверхности после перечисленных evidence gates.
