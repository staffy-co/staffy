# Независимая проверка доступности и inclusive multimodal UX

Дата: 2026-09-13  
Объект: `DESIGN.md`, `EXPERIENCE.md`, `.memlog.md`, все визуальные материалы `.working/` и релевантные PRD-extracts.  
Метод: проверка UX-контракта, а не реализации. Базовый норматив — [WCAG 2.2](https://www.w3.org/TR/WCAG22/) уровня AA.

## Вердикт

**Условно не готов к финализации как проверяемый WCAG 2.2 AA-контракт.** Направление сильное и не содержит критической ошибки уровня концепции, но девять High-находок оставляют ключевые требования недоказуемыми или допускают несовместимую с AA реализацию. После их исправления документ можно передавать на повторную accessibility-проверку; Medium/Low следует закрыть до выпуска соответствующих поверхностей.

| Уровень | Количество |
|---|---:|
| Critical | 0 |
| High | 9 |
| Medium | 9 |
| Low | 3 |

## Сильные стороны

- Статус, риск и прогресс не должны передаваться одним цветом; предусмотрены текст, иконка и следствие (`DESIGN.md:62–67`, `EXPERIENCE.md:198–205`).
- Заданы клавиатура, skip link, видимый focus, возврат focus из modal, масштабирование текста, 44×44 px и reduced motion (`DESIGN.md:124–134`, `EXPERIENCE.md:196–205`).
- Голос и фото не являются единственным маршрутом; заявлены DTMF, текст, человек, callback и путь без media (`EXPERIENCE.md:180–194`, `EXPERIENCE.md:196–205`).
- Защищённое продолжение mobile-first, без обязательного аккаунта, со сроком, отзывом ссылки, сохранением черновика и честным network state (`EXPERIENCE.md:147–148`, `EXPERIENCE.md:207–219`).
- Формы сохраняют ввод, ошибки связываются с полями; критические сущности можно читать посимвольно и копировать (`EXPERIENCE.md:198–203`).
- Cognitive load уже снижен: один рекомендуемый шаг, progressive disclosure, не более двух связанных вопросов и focused repair вместо повторного intake (`EXPERIENCE.md:102–121`, `EXPERIENCE.md:178–194`).
- Barge-in, фактически услышанная часть и repair после reconnect описаны корректно для безопасного голосового UX (`EXPERIENCE.md:140`, `EXPERIENCE.md:194`).
- Источник PRD требует передавать preferred channel/time/language и accommodations в handoff (`.working/source-prd.md:222–231`), то есть основание для устранения одной из главных лакун уже существует.

## Findings

### High

#### A11Y-H1 — Reflow ошибочно ограничен защищённым продолжением

- **Место:** `EXPERIENCE.md:198–200`, `EXPERIENCE.md:207–219`; `DESIGN.md:99–102`.
- **Проблема:** 200% zoom заявлен для всех действий, но 400%/линейный режим — только для protected continuation. Для web-продукта WCAG 1.4.10 требует отсутствие двумерной прокрутки при эквивалентной ширине 320 CSS px для всех поверхностей, кроме действительно двумерного содержимого. Dense tables, Today, Work, Integrations и настройки остаются без такого контракта.
- **Исправление:** закрепить для всех web-поверхностей reflow при 320 CSS px без горизонтальной прокрутки страницы и без потери контента/действий; исключения перечислить поимённо (например, схема или таблица, для которой двумерность необходима) и дать им доступное карточное/текстовое представление либо локальную прокрутку с понятной меткой.

#### A11Y-H2 — Семантические tokens допускают недостаточный контраст

- **Место:** `DESIGN.md:47–60`, `DESIGN.md:62–67`, `DESIGN.md:119–124`, `DESIGN.md:140–148`.
- **Проблема:** `colors.warning #B56617` на `colors.surface #FFFFFF` имеет приблизительно 4.31:1 и не проходит 4.5:1 для обычного текста. `colors.border #D7DED9` на белом — приблизительно 1.37:1; если эта граница единственная видимая форма control/field/state, она не проходит 3:1 для non-text contrast. Текущий текст разрешает warning как текст/границу без отдельного токена.
- **Исправление:** разделить минимум на `warning.text` (≥4.5:1 для обычного текста), `warning.graphic/control` (≥3:1 к соседнему фону) и нейтральный `control.border` (≥3:1, если именно граница сообщает наличие/состояние control). Зафиксировать матрицу token × background × font size/state и запретить использовать слабый `border` как единственный признак интерактивного элемента.

#### A11Y-H3 — Focus contract не покрывает overlays и Focus Not Obscured

- **Место:** `DESIGN.md:101`, `DESIGN.md:104–113`, `DESIGN.md:126–134`; `EXPERIENCE.md:129`, `EXPERIENCE.md:198–203`, `EXPERIENCE.md:209–214`.
- **Проблема:** есть возврат focus из modal, но нет запрета keyboard trap, правил initial focus/focus trap для modal, `Escape`, inert-фона, а также требования, чтобы sticky action, drawer, popover, toast или virtual keyboard не скрывали focused control. Это оставляет пробел по WCAG 2.1.2 и 2.4.11 на основных мобильных и операционных поверхностях.
- **Исправление:** добавить единый overlay contract: осмысленный initial focus, trap только в modal, `Escape` для dismissible layer, возврат focus к источнику или логичному successor, фон modal недоступен AT, немодальные слои не захватывают focus, focused element никогда не скрыт sticky/fixed content и автоматически доводится до видимой области.

#### A11Y-H4 — Step-up authentication не имеет доступного контракта

- **Место:** `EXPERIENCE.md:98`, `EXPERIENCE.md:147`, `EXPERIENCE.md:168`, `EXPERIENCE.md:185`; `.working/flow-cross-channel-handoff-2026-09-13.excalidraw`, узел «Личность до объединения».
- **Проблема:** risk-based step-up обязателен перед merge/sensitive/material step, но способ проверки не ограничен. Реализация может потребовать запоминания/перепечатки кода, запретить paste/password manager или использовать недоступную CAPTCHA, заблокировав ключевой путь и нарушив WCAG 3.3.8.
- **Исправление:** явно запретить cognitive-function-only проверку; разрешить copy/paste и autofill OTP, password manager при наличии пароля, доступный resend и альтернативный метод без visual/audio puzzle. Ошибка/истечение не уничтожает Case или заполненные данные; всегда доступен безопасный human/callback путь с тем же уровнем Confirmation.

#### A11Y-H5 — Альтернативы голосу перечислены, но не образуют исполнимый state machine

- **Место:** `EXPERIENCE.md:106–121`, `EXPERIENCE.md:180–194`, `EXPERIENCE.md:203–205`, `EXPERIENCE.md:241–255`.
- **Проблема:** DTMF/text/human/callback названы, но не определены обнаружение, одинаковые глобальные команды, DTMF mapping, no-input/no-match, повтор/замедление/посимвольное чтение и переход без потери контекста. Пользователь с нарушением речи/слуха либо в шумном канале может формально иметь fallback, но не суметь его вызвать или безопасно подтвердить действие.
- **Исправление:** определить Voice Accessibility State Machine: коротко объявлять доступные способы; глобальные команды «повторить», «медленнее», «ввести кнопками», «прислать текстом», «позвать человека»; стабильные DTMF назначения; лимит repair-попыток; no-input/no-match → выбор альтернативы; перенос Case и Confirmation без повторного intake; никакого ухудшения safety/price/claim path.

#### A11Y-H6 — Accommodations теряются между каналами и при handoff

- **Место:** `EXPERIENCE.md:38–45`, `EXPERIENCE.md:140–143`, `EXPERIENCE.md:185–189`, `EXPERIENCE.md:246`; источник `.working/source-prd.md:222–231`.
- **Проблема:** PRD-extract требует передавать `preferred channel/time/language, accommodations`, но Human Handoff, Case и channel switch в spine это поле не сохраняют. Человек может снова звонить пользователю, который просил только текст, говорить слишком быстро или заставить повторять потребность.
- **Исправление:** добавить в Case явные, редактируемые самим пользователем `communication preferences/accommodations`: канал, темп, способ readback, отсутствие фото/голоса, доступное время. Не выводить медицинские диагнозы и не угадывать инвалидность; передавать только заявленную минимально необходимую настройку каждому Response Owner и соблюдать её во всех разрешённых каналах.

#### A11Y-H7 — Русский readback VIN/OEM/телефона недостаточен для смешанных алфавитов

- **Место:** `DESIGN.md:73–83`; `EXPERIENCE.md:106–112`, `EXPERIENCE.md:183`, `EXPERIENCE.md:202`, `EXPERIENCE.md:249–253`; `.working/flow-autoparts-a1-a7-2026-09-13.excalidraw`, узлы «Критические сущности» и «Конфликт».
- **Проблема:** «посимвольное чтение» не задаёт различение `О/0`, `I/1`, латинских и кириллических омографов (`B/В`, `H/Н`), группировку, темп и переход в визуальный/DTMF канал. На шумной 8 kHz телефонии это может привести к неверной применимости детали или адресу/сумме.
- **Исправление:** Domain Pack должен задавать формат readback для каждого critical entity: нормализация и алфавит, группировка, однозначные слова/символы, возможность повторить только группу, замедлить, ввести кнопками/текстом или открыть protected field. В Confirmation показывать/произносить нормализованное значение и источник; омограф/low confidence всегда блокирует действие.

#### A11Y-H8 — Timeout/expiry нельзя проверить и он может уничтожить незавершённый путь

- **Место:** `DESIGN.md:160–161`; `EXPERIENCE.md:147`, `EXPERIENCE.md:168–170`, `EXPERIENCE.md:203`, `EXPERIENCE.md:214`.
- **Проблема:** сказано «заранее сообщаются и продлеваются там, где безопасно», но не определено, когда предупреждать, как продлевать с клавиатуры/AT, что происходит с draft/upload/Confirmation и как восстанавливается сессия после re-auth. Это особенно опасно для медленного чтения, motor/cognitive disability и слабой сети.
- **Исправление:** для каждого client-side time limit задать: видимое и программно объявляемое предупреждение, достаточное время реакции, простое продление/отключение где допустимо, сохранение введённых данных, безопасный re-auth/resume и точный статус server receipt. Expiry ссылки отменяет полномочие ссылки, но не уничтожает Case/черновик; незавершённое material Confirmation никогда не превращается в согласие.

#### A11Y-H9 — WCAG заявлен как цель, но не встроен в admission/release gate

- **Место:** `EXPERIENCE.md:150`, `EXPERIENCE.md:191`, `EXPERIENCE.md:196–205`, `EXPERIENCE.md:239–257`; `DESIGN.md:163`.
- **Проблема:** Test & Release требует normal/edge/failure, но не a11y evidence, assistive technology matrix или запрет promotion при критической accessibility-регрессии. «WCAG 2.2 AA как минимальная цель» нельзя проверить и можно пропустить при Controlled Work/Work.
- **Исправление:** сделать accessibility обязательным Evidence Gate для каждой публикуемой web-поверхности и затронутого channel flow: автоматические проверки + ручная клавиатура + screen reader + zoom/reflow/text spacing + high contrast/forced colors + touch + reduced motion + voice/media alternatives. Critical/High accessibility failure блокирует affected admission scope; исправление получает regression case и версию.

### Medium

#### A11Y-M1 — Screen-reader semantics и live announcements описаны слишком общо

- **Место:** `DESIGN.md:142–167`; `EXPERIENCE.md:127–154`, `EXPERIENCE.md:156–174`, `EXPERIENCE.md:201`.
- **Проблема:** «live updates объявляются» не определяет landmarks/headings, accessible names/state/value, связи error-summary→field, таблицы/карточки, порядок Conversation Timeline и приоритет `polite/assertive`. Streaming transcript, SLA, upload progress и Action Receipt могут либо молчать, либо спамить screen reader.
- **Исправление:** задать semantic contract по ключевым компонентам: HTML native first; name/role/state/value; heading/landmark hierarchy; таблица с headers или карточный эквивалент; один агрегированный polite status channel; assertive только safety/irreversible failure; transcript не объявлять токен за токеном; focus не перемещать при status update.

#### A11Y-M2 — Нет требования Text Spacing и защиты от обрезания

- **Место:** `DESIGN.md:69–83`, `DESIGN.md:85–102`; `EXPERIENCE.md:198–200`.
- **Проблема:** resize text указан, но не описано сохранение контента/функций при пользовательском line-height, paragraph/letter/word spacing (WCAG 1.4.12). Dense rows, pills и `white-space` могут обрезать русский текст.
- **Исправление:** добавить обязательный тест со значениями WCAG Text Spacing, запрет fixed-height для текстовых controls/cards, перенос длинных названий/статусов и отсутствие ellipsis для материальной информации без полностью доступного раскрытия.

#### A11Y-M3 — Раскрываемый detail не покрывает Content on Hover or Focus

- **Место:** `DESIGN.md:36`, `DESIGN.md:146`; `EXPERIENCE.md:139–146`, `EXPERIENCE.md:221–229`.
- **Проблема:** важные детали раскрываются, а tooltip-only запрещён, но нет контракта для hover/focus popover: dismissible, hoverable, persistent. Провайдерские ограничения/provenance могут стать недоступны пользователю клавиатуры или увеличителя.
- **Исправление:** материальные детали открывать по click/keyboard в persistent disclosure; для любого hover/focus content обеспечить `Escape`, возможность переместить указатель на контент и сохранение до dismiss/focus loss; не помещать действие внутрь hover-only tooltip.

#### A11Y-M4 — Media redaction и guidance не определены для non-visual/keyboard use

- **Место:** `DESIGN.md:160–162`; `EXPERIENCE.md:148`, `EXPERIENCE.md:187`, `EXPERIENCE.md:204`, `EXPERIENCE.md:241–247`; `.working/flow-montagio-m1-m7-2026-09-13.excalidraw`, узлы 5–6B.
- **Проблема:** preview/remove/redact/retry и «нужный ракурс» перечислены, но не определены keyboard-operable crop/redaction, текстовое описание изображения, понятное описание дефекта и human-assisted review. Файловая альтернатива drag-and-drop не равна альтернативе визуальной квалификации.
- **Исправление:** разделить альтернативу способу загрузки и альтернативу самому visual evidence: обычный file picker, camera без принудительной ориентации, keyboard redaction либо human privacy review, structured text description/осмотр/callback. Ошибка сообщает конкретно «не виден узел/блик/слишком далеко» и не зависит от цвета/картинки-примера.

#### A11Y-M5 — Weak-network recovery не описывает конфликт двух версий

- **Место:** `EXPERIENCE.md:168`, `EXPERIENCE.md:174`, `EXPERIENCE.md:207–218`; `EXPERIENCE.md:243`, `EXPERIENCE.md:250`.
- **Проблема:** draft и server receipt различаются, но нет поведения, когда после reconnect на сервере уже другая версия, ссылка отозвана, human takeover состоялся или upload завершился поздно. Автоматическое применение локального draft может вернуть устаревшее Commitment.
- **Исправление:** показывать local/server version, время и owner; при конфликте не merge material fields автоматически; предложить сравнение и безопасный выбор/повторную Confirmation. Поздний upload относится к исходному request и не запускает Action; network status и восстановление объявляются AT без focus steal.

#### A11Y-M6 — Russian-first не отделяет внутренние IDs от пользовательских labels

- **Место:** `DESIGN.md:71–83`; `EXPERIENCE.md:102–121`, `EXPERIENCE.md:156–174`.
- **Проблема:** в канонических состояниях и UI-компонентах смешаны английские идентификаторы и русская речь (`partial`, `unknown`, `acknowledged`, `Commitment`). Не сказано, что screen reader получает локализованное название, а иноязычная часть размечается языком; произношение аббревиатур VIN/OEM/ABCP может быть непонятным.
- **Исправление:** закрепить: machine IDs не показываются как primary copy; каждому state/action нужен approved Russian label + consequence. Для неизбежных иностранных слов/аббревиатур — `lang` и/или доступное произношение/расшифровка; тестировать российские screen readers/голоса на VIN, OEM, ABCP, Jivo, Novofon и именах.

#### A11Y-M7 — «Позвать человека» не закреплено как consistently findable help

- **Место:** `EXPERIENCE.md:44`, `EXPERIENCE.md:100`, `EXPERIENCE.md:119`, `EXPERIENCE.md:188–189`, `EXPERIENCE.md:204–205`.
- **Проблема:** человеческий путь доступен «в любой момент», но не задано его стабильное место/команда в одном наборе страниц и одинаковая формулировка в голосе/чате. Пользователь с cognitive disability может не найти помощь в критический момент.
- **Исправление:** определить стабильный глобальный control/команду «Позвать человека» и «Вернуться к разговору» для каждой поддерживаемой surface; название и ожидаемый результат одинаковы, права/доступность не скрывают безопасный fallback, текущий owner/status виден рядом.

#### A11Y-M8 — Dense expert views не имеют доступного чтения без глобального переключателя плотности

- **Место:** `DESIGN.md:35–36`, `DESIGN.md:99–102`; `EXPERIENCE.md:211–213`; `.working/directions-4.html:381–421`.
- **Проблема:** повышенная плотность разрешена по роли, но роль не определяет зрительные или когнитивные возможности. Большие таблицы, SLA и несколько метрик могут стать непроходимыми при zoom/AT. Это не требует отмены подтверждённого решения «без пользовательского переключателя плотности в MVP».
- **Исправление:** для dense таблиц обязательны summary-first, корректная semantic table/navigation, фильтрация, устойчивый reading order и автоматическое карточное/линейное представление при reflow/zoom. Не добавлять глобальный density setting; доступный alternate presentation должен быть свойством компонента.

#### A11Y-M9 — Отзыв согласия на запись во время звонка не имеет UX

- **Место:** `EXPERIENCE.md:64–75`, `EXPERIENCE.md:169`, `EXPERIENCE.md:180`, `EXPERIENCE.md:204–205`; источник `.working/source-montagio-conversation.md:74–84`.
- **Проблема:** до записи есть gated alternative, но источник прямо оставляет `CONSENT_REVOKED` UX открытым. Не определено, как пользователь голосом/DTMF отзывает согласие, что перестаёт записываться и как продолжить равноценное обслуживание.
- **Исправление:** добавить доступную команду/DTMF отзыва, немедленное подтверждение фактического состояния записи, объяснение судьбы уже собранного материала по policy и бесшовный переход к разрешённому no-recording text/human/callback route без потери Case.

### Low

#### A11Y-L1 — Визуальные HTML-референсы нельзя повышать до UI без переработки

- **Место:** `.working/color-themes-5.html:39–95`, особенно `:58–69`, `:80–93`; `.working/directions-4.html:72–128`.
- **Проблема:** референсы используют текст 8–11 px, controls меньше 44 px и focus style только для отдельных кнопок. Это допустимо как иллюстрация направления, поскольку spines явно имеют приоритет, но опасно при копировании в key-screen mock/component implementation.
- **Исправление:** пометить HTML как non-production visual reference и при создании key-screen mocks пересобрать на canonical type/control tokens; запускать автоматическую и ручную a11y-проверку уже на новых mocks/компонентах.

#### A11Y-L2 — Диаграммы не имеют самостоятельных текстовых описаний

- **Место:** `EXPERIENCE.md:86`, `EXPERIENCE.md:231`, `EXPERIENCE.md:259`; все `.working/*.excalidraw`.
- **Проблема:** значительная часть flows дублируется в Key Flows, но ссылки на IA/диаграммы не говорят, где находится эквивалент и какие детали визуала не обязательны. Читателю без доступа к canvas трудно проверить полную трассировку.
- **Исправление:** рядом с каждой promoted diagram дать короткое text summary и ссылку на каноническую таблицу/нумерованный flow; диаграмма не должна быть единственным источником решения.

#### A11Y-L3 — Не зафиксированы orientation и pointer-only ограничения

- **Место:** `EXPERIENCE.md:207–219`; `DESIGN.md:124`, `DESIGN.md:177`.
- **Проблема:** mobile-first и 44×44 заданы, но нет запрета orientation lock, path/multipoint gesture-only действий и требования pointer cancellation. Риск невелик, пока такие controls не нарисованы, но контракт их не предотвращает.
- **Исправление:** разрешить portrait и landscape, если ориентация не essential; вся drag/swipe/pinch-функция имеет single-pointer/button/keyboard equivalent; high-stakes действие срабатывает на `up`/подтверждение и может быть отменено до commit.

## Нерешённые проверки до acceptance

1. **Reflow/zoom:** Today, Work, Integrations, Employee workspace и protected continuation при 320 CSS px; 200% text resize; карточный эквивалент dense tables; отсутствие скрытого focus под sticky elements.
2. **Text spacing:** line-height 1.5, paragraph spacing 2× font size, letter spacing 0.12 em, word spacing 0.16 em — без потери текста/действий.
3. **Keyboard:** полный путь UJ-P2, UJ-M6, UJ-A1/A5/A7; skip, drawers, dialogs, popovers, upload/redaction, Confirmation, error recovery; отсутствие trap.
4. **Screen readers:** NVDA с актуальным Chromium/Firefox; VoiceOver/Safari iOS; TalkBack/Chrome Android — landmarks, labels, status changes, timelines, tables, uploads, validation и receipts.
5. **Visual modes:** Windows forced-colors/high-contrast, увеличитель, 200%/400%, focus visibility, selected/disabled/unknown states; contrast matrix для всех token/background combinations.
6. **Accessible authentication:** link open, OTP/autofill/copy-paste, resend, expiry during entry, revoked link, re-auth with saved draft, no-CAPTCHA/human alternative.
7. **Voice inclusion:** отказ от записи, DTMF-only, text switch, callback, slow/repeat/spell, no-input/no-match, stutter/dysarthria/accent/noisy 8 kHz, hearing-impaired route и human request.
8. **Russian critical entities:** `О/0`, `I/1`, `B/В`, `H/Н`, длинный VIN, OEM/SKU, телефон, адрес, сумма и дата; repair только спорной группы; switch to protected input.
9. **Media:** keyboard file select, camera permission denied, no-camera/no-photo route, screen-reader upload progress, resumable weak-network upload, image rejection, accessible redact/remove и human privacy review.
10. **Weak network/recovery:** disconnect до/после submit, поздний upload, double tap, server/local version conflict, takeover during offline, link expiry and reissue, no duplicate Action/Case.
11. **Cognitive usability:** понятность Russian labels, material delta, двух Confirmation, exact effect/owner/fallback; проверка с пользователями, которые редко работают с SaaS и используют увеличение/простые формулировки.
12. **Accessibility gate:** заранее определить pass/fail evidence, владельца исправления, блокирующую severity и regression suite для переходов Demo → Trial → Controlled Work → Work.

## Итоговое условие повторной проверки

Повторная accessibility-проверка имеет смысл после переноса исправлений A11Y-H1…H9 в `DESIGN.md`/`EXPERIENCE.md` и появления хотя бы первых key-screen mocks. До этого нельзя маркировать оба spine как `final` с доказуемым WCAG 2.2 AA floor.
