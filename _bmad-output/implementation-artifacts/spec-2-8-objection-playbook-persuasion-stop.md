---
title: 'Story 2.8 — Консультативная продажа и этичная работа с возражениями (первый срез: Objection Record, versioned Playbook и останов убеждения)'
type: 'feature'
created: '2026-10-13'
status: 'backlog'
route: 'dispatch'
baseline_commit: '214ef7e70d14ed4a5d9c1cb8804970f0ba590833'
context:
  - '{project-root}/_bmad-output/planning-artifacts/epics.md'
  - '{project-root}/_bmad-output/planning-artifacts/prds/prd-staffy-2026-09-12/prd.md'
  - '{project-root}/_bmad-output/planning-artifacts/architecture/architecture-staffy-2026-09-13/ARCHITECTURE-SPINE.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-2-work-items-facts-provenance.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-4-response-owner-inbox.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-5-conversation-mode-ai-disclosure.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-6-progressive-intake-policy-outcome.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-2-7-owning-system-observations.md'
  - '{project-root}/_bmad-output/implementation-artifacts/deferred-work.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Возражение клиента в системе не существует как объект: нет ни таблицы, ни команды, ни словаря — «тип возражения и supporting source spans сохраняются в Work Item» (AC-4) нечем исполнить; versioned playbook не существует — Policy не выбирает и не проверяет разрешённую последовательность ответа, а «сначала признаёт смысл опасения, затем доказательство или честное отсутствие ответа» остаётся пожеланием без машинного правила. Повторный отказ, stop-intent, просьба не связываться и требование человека не имеют машинного последствия: send_client_reply с классом sales проходит гейты 2-4→2-5→2-6 независимо от числа отказов клиента — «прекращает убеждение» (AC-7) не enforcement. Возражения не привязаны к Work Item — изоляция «отказ по одному предложению не закрывает другие задачи» (AC-10) не обеспечена структурно. Типы «доверие» (AC-5) и «сравнение с более дешёвым» (AC-6) не имеют словарных слов — машинной границы честных техник нет. Проверяемые последствия FR-24 (прекращение после отказа, отсутствие давления) не работают на сервере: домен продаж не имеет скелета.

**Approach:** Первый срез — детерминированный серверный скелет работы с возражениями (прецедент 2-5/2-6/2-7: AI Runtime в контуре НЕТ; продюсер размечает, машина гейтит). Четыре части: (1) словари Domain Pack v0: типы возражений {trust, price_comparison, general_doubt}, сигналы клиента {concern, refusal, stop_intent, human_request}, шаги playbook {acknowledge_concern, present_evidence, honest_no_answer, offer_alternative, clarify_question} + versioned playbook v1 (rule-table прецедент 2-6/2-7): тип → разрешённые шаги + бюджет убеждения (порог = 2 refusal-записи на WI, lifetime); (2) команда record_objection (оператор review ИЛИ configuration, гейт скоупа ДО материальных стражей — прецедент M1 2-5/2-7): источник — принятый входящий turn кейса (channel_event 2-1), spans — дословные выдержки payload этого turn (побайтовая проверка через ReadPayload: выдуманная выдержка невозможна), тип+сигнал словарём, append-only строка + outbox-метаданные; (3) состояние убеждения — производная чтения (прецедент observation/policy 2-6/2-7): текущее возражение = последняя запись WI; stopped = сигнал текущего ∈ {stop_intent, human_request} ИЛИ число refusal-записей ≥ порога; подъём режима (human_requested/privacy_no_contact) остаётся командой 2-5 propose_mode — запись лишь маркирует сигнал-триггер (дублирования пути нет); (4) гейт sales-ответов 2-8 (ПОСЛЕ policy-гейта 2-6, только класс sales): stopped → честный 400 со словами разрешённого завершения (neutral-завершение, protected-просьба, human handoff); при текущем возражении ответ обязан нести playbook_step из playbook[type] (первый связанный ответ — только acknowledge_concern — «сначала признаёт»); возражений на WI нет → путь ответа байт-в-байт прежний (контрольная ветка).

**Решение владельца:** ожидает CHECKPOINT 1. Предложение SM: спека предъявляется владельцу в ответе сессии (делегированный прецедент 2-6/2-7: «одобряю (доверяю вашему экспертскому мнению)»); отзыв/корректировка — по явной просьбе.

## Boundaries & Constraints

**Always:** append-only `cases.objections` — гранты INSERT/SELECT, RLS + FORCE ROW LEVEL SECURITY, UPDATE/DELETE/TRUNCATE нет; objection_no — монотонный счётчик (WI) под advisory-локами кейса+WI (прецедент policy_outcomes 2-6 / observations 2-7); идемпотентность: тот же command-ключ с тем же материалом → replay исходной записи (command_records 2-1, +1 слово словаря команд); тот же ключ с иным материалом → ErrCommandKeyConflict 409; тип/сигнал/шаги — словари CHECK; версия playbook фиксируется в записи (снимок выбора Policy, AC-4 «versioned»); spans: ≤8, каждая ≤200 символов, каждая — побайтовая подстрока payload turn-источника (ReadPayload 2-1), DB CHECK на длину массива, вхождение и длина элементов — домен+app (честная фиксация: SQL не считает подстроки); outbox-событие objection_recorded — метаданные (тип, сигнал, номер, turn_ref, версия playbook) БЕЗ текста span (слова клиента уже хранятся 2-1; прецедент «кандидат не покидает строку» 2-6); гейты record_objection: скоуп review ИЛИ configuration ДО материальных стражей (403 словами скоупа, прецедент M1 2-5/2-7), не-участник → 404 ×пути, чужой tenant → RLS 404; turn-источник — принятый (accepted) event кейса (ReadEventOfCase 2-6), modality=text (голос → честный 400 словарём: разметка выдержек голосового контура — следующие срезы); терминальный WI → 400 work-rule (граница 2-2); гейт sales-ответов: только reply_class=sales, ПОСЛЕ 2-6 policy-гейта, ДО записи; нейтральные/защищённые ответы путь не меняют (байт-в-байт); objection_id записи ответа разрешается ТОЛЬКО сервером (текущее возражение WI — request его не несёт, прецедент «серверная истина» 2-6); поле playbook_step в request: при текущем возражении обязательно (отсутствие → 400 — дисциплина не опциональна), при отсутствии возражений поле пусто → путь прежний; слово вне словаря либо вне playbook типа → 400; первый связанный ответ — только acknowledge_concern, последующие — любой шаг playbook типа (производная «acknowledge отправлен» от связанных client_replies); панель «Возражения и бюджет убеждения» на карточке WI: тип/сигнал/выдержки/turn_ref/время, бюджет (refusal-счёт/порог), бейдж «убеждение остановлено» текстом; a11y: aria-live=polite без перемещения фокуса, 320 px без горизонтального скролла, stopped не отличим от active только цветом (прецедент панелей 2-5…2-7).

**Never:** не строить контентную классификацию/сканирование dark-patterns и repair-цикл (AC-8 — путь AI-кандидата и eval-evidence следующих срезов; словари запрещённых формулировок живут employee-стороне 1-6 — их кросс-проверка не появляется как класс); не проверять Allowed Claims по evidence/channel/mode-скоупу (AC-2 — нужен порт каталога claims, следующий срез; span ссылается только на слова клиента); не ставить гейт observation-freshness на sales-ответ (заморожено 2-7: материальный unknown — с Commitment-гейтом 2-9+); не дублировать подъём режима: propose_mode 2-5 — единственный путь перехода (запись маркирует сигнал, панель подсказывает путь; сама запись НИКОГДА не меняет режим); не обнулять и не списывать бюджет (append-only lifetime — «повторно отказывается в пределах playbook threshold» монотонно, fail-safe); не агрегировать возражения между WI (изоляция AC-10 структурна: состояние убеждения — производная одного WI); не трогать гейты/лестницу/миграции 2-1…2-7 (кроме: +1 слово словаря команд, +1 outbox-тип, client_replies.objection_id uuid + playbook_step text — nullable, только sales-строки несут; прецедент policy_outcome_id 2-6); не строить commit/next-step машины (AC-9 — story 2-9: «интерес клиента не считается Confirmation» — в срезе интерес лишь не создаёт возражение); не размечать выдержки голосовых turn (срез — modality text; голосовой readback — следующие срезы); не строить eval-набор sales (AC-11 — с Release Manifest-контуром); не считать конверсию или «успех убеждения» (машинной цели нет — только честные исходы).

## I/O & Edge-Case Matrix

- Given кейс с действующим WI и принятым text-turn, оператор review, when record_objection (type=price_comparison, signal=concern, spans — дословные выдержки payload), then append-only строка №1 (objection_no=1, словарные слова, снимок playbook v1), outbox-метаданные без выдержек; панель показывает карточку возражения (AC-4).
- Given span не является подстрокой payload turn-источника (выдуманная выдержка), when record, then честный 400 span-not-in-source — выдуманные выдержки невозможны (AC-4 честность).
- Given turn_ref — не принятый event кейса (несуществующий / quarantined / чужой кейс / modality=voice), when record, then 404 либо честный 400 словарём — источник существует, принят и текстовый (границы 2-1).
- Given type или client_signal вне словаря, when record, then 400 словарём (гвард до материальных стражей).
- Given повторная команда с тем же command-ключом и тем же материалом, when record, then replay: исходная строка возвращена, новых строк/событий нет (идемпотентность 2-1).
- Given тот же command-ключ с иным материалом, when record, then ErrCommandKeyConflict — честный 409 (прецедент 2-6/2-7).
- Given первая запись signal=refusal (refusal-счёт 1 < порога 2), when send_client_reply класс sales с playbook_step=acknowledge_concern, then проходит; запись ответа несёт (objection_id, step) — ответ связан с возражением серверно (AC-4 дисциплина).
- Given sales-ответ при текущем возражении БЕЗ playbook_step, when send, then 400 — дисциплина обязательна (AC-4).
- Given первый связанный ответ со step=present_evidence (acknowledge ещё не отправлен), when send, then 400 — «сначала признаёт смысл опасения» (AC-4).
- Given acknowledge уже отправлен (связанный ответ существует), when второй связанный ответ step=honest_no_answer, then проходит — «затем доказательство ИЛИ честное отсутствие ответа» (AC-4).
- Given вторая запись signal=refusal (refusal-счёт 2 ≥ порога), when send_client_reply класс sales (любой step), then честный 400 persuasion_stopped со словами разрешённого завершения; нейтральный/защищённый ответ проходит своим порядком гейтов (AC-7 «повторно отказывается в пределах установленного playbook threshold»).
- Given запись signal=stop_intent либо human_request (первая же), when sales-ответ, then 400 — останов немедленно; панель показывает сигнал-триггер и путь propose_mode 2-5 (AC-7: режим фиксируется существующей командой, не записью).
- Given возражения на WI-A, when sales-ответ по WI-B того же кейса, then проходит свободно — изоляция per-WI; отказ по одному предложению не закрывает другие задачи (AC-10).
- Given оператор audit-only, when record_objection, then 403 словами скоупа ДО стражей (прецедент M1); не-участник → 404 ×пути; пользователь другого Tenant → RLS 404 (регресс 2-1).
- Given WI в терминальном состоянии, when record_objection, then 400 work-rule (граница 2-2).
- Given возражений на WI нет вовсе, when sales-ответ без playbook_step, then путь байт-в-байт прежний (контрольная ветка; регресс 2-6 зелёный).
- Given контракт OpenAPI и contract-тест, when пути описаны, then схемы ObjectionRequest/ObjectionView/панели зеркальны словарям; send_client_reply + optional playbook_step + честные 400 (прецедент TestPolicyContractIsHonest).
- Given панель с клавиатуры/screen reader/320 px, when останов убеждения активен, then бейдж «убеждение остановлено» доступен текстом, aria-live без перемещения фокуса; stopped не спутан с active без цвета (a11y-срез панели).

</frozen-after-approval>

## Code Map

- `internal/cases/domain/objection.go` (новый): словари (типы, сигналы, шаги), playbookV1 (rule table: тип → шаги; порог бюджета), GuardObjectionCommand (словари/пределы spans), DeriveObjectionState (текущее, refusal-счёт, stopped, acknowledged), SpanInPayload (побайтовая проверка), ObjectionPlaybookVersion.
- `migrations/cases/00008_objections.sql` (новый): таблица `cases.objections` (append-only, RLS+FORCE, INSERT/SELECT гранты, CHECK-словари type/signal, spans jsonb с CHECK длины массива, playbook_version, монотонный objection_no), +client_replies.objection_id uuid / +playbook_step text (nullable, слово словаря), +1 команда словаря command_records, +1 outbox-тип objection_recorded.
- `internal/cases/store/objection.go` (новый): запись с монотонным objection_no под advisory-локами (прецедент 2-6/2-7), чтение текущего возражения/истории, refusal-счёт, чтение связанных ответов WI (acknowledged-производная), запись ответа с (objection_id, playbook_step).
- `internal/cases/app/objection.go` (новый): команда RecordObjection — гейты (скоуп → участники → терминальность → turn-якорь → span-вхождение) → запись+outbox, replay; objectionPanelView; salesObjectionGate для SendClientReply.
- `internal/cases/app/ownership.go`: SendClientReply — гейт 2-8 класса sales ПОСЛЕ replyPolicyGate 2-6: stopped → 400 persuasion_stopped; текущее возражение → playbook_step обязателен и словарен, первый — acknowledge; (objection_id, step) уходят в запись ответа (серверная истина).
- `internal/cases/api/objection.go` (новый): DTO команды, ObjectionView/панель-карточки (json-теги snake_case — машинный контракт).
- `internal/transport/http/cases_objection.go` (новый): `POST /v1/tenants/{t}/cases/{c}/work-items/{wi}/objections` (API) + `POST /app/.../objections` (PRG-форма); панель «Возражения и бюджет убеждения» в `web/templates/case.html`.
- `contracts/openapi/staffy.yaml`: +1 путь, +2 схемы, словари зеркальны, send_client_reply + optional playbook_step + честные 400-словари.
- `tests/integration/cases_objection_test.go` (новый): матрица поимённо (18 строк выше) + pg_class-зонд грантов/RLS + INSERT-зонд словарей/NOT NULL-инвариантов (урок R1 2-7: инварианты закрепляются живыми пробами немедленно).
- `tests/journeys/`: cases_objection_fixture.go + цепь zzz…-cases-objection.spec.ts (панель, PRG-запись, дисциплина step, stopped-бейдж, aria-live, 320px).

## Tasks & Acceptance

- **step-01 — домен и словари**: objection.go — словари закрыты, playbook v1, гварды честны, DeriveObjectionState, SpanInPayload; unit-тесты. Accept: unit зелёный; ни одно слово не появляется вне словаря.
- **step-02 — миграция и хранилище**: 00008 (RLS/гранты/словари/колонки ответов), store (монотонный номер, производные чтения), зонды pg_class + INSERT. Accept: миграция применена; зонды зелёные.
- **step-03 — команда, гейт, транспорт, панель**: app-команда с гейтами и replay; гейт sales в SendClientReply (после 2-6); API+PRG; панель case.html; OpenAPI+контракт-тест. Accept: контракт зеркален; journeys локально зелёные.
- **step-04 — интеграционные и journeys**: полный матрикс-прогон (18 строк поимённо), journeys-цепь. Accept: интеграция ×2 TZ зелёная; изоляция и дисциплина покрыты живьём.
- **step-05 — регресс и коммит**: build/vet/unit/интеграция ×2 TZ/journeys/контракт/gofmt/diff-check; коммит feat [без skip ci]. Accept: регресс зелёный во всех контурах; дерево чистое.

## Implementation Notes

- Playbook v1 (Domain Pack v0, fixed rule table): trust → [acknowledge_concern, present_evidence, honest_no_answer, offer_alternative]; price_comparison → [acknowledge_concern, present_evidence, offer_alternative, honest_no_answer]; general_doubt → [acknowledge_concern, clarify_question, present_evidence, honest_no_answer]; порог бюджета = 2 refusal-записи (словарь).
- Сигналы клиента: concern (обычное возражение — не останавливает), refusal (отказ — считает бюджет), stop_intent (немедленная остановка; «просит не связываться» = stop_intent записи + privacy_no_contact режим командой 2-5), human_request (немедленная остановка + путь propose_mode(human_requested) 2-5).
- AC-5/AC-6 честность среза: словарные шаги честных техник (offer_alternative без демпинга, honest_no_answer) — машинная рамка; контентная проверка выдуманных отзывов/сертификатов/опыта — путь AI-кандидата следующих срезов (граница продюсер-разметки: машина контент не читает, прецедент классов ответов 2-5).
- Span-честность: побайтовое вхождение выдержки в payload sealed_payload (ReadPayload 2-1); «supporting source spans» = выдержки слов клиента, не выдуманные свидетельства; голосовые turn — честный отказ словарём.
- Бюджет lifetime: не сбрасывается, не списывается действиями системы (append-only, fail-safe) — «клиент повторно отказывается» монотонно копится; порог словарный, без конфигурации.
- acknowledged — производная связанных client_replies (objection_id + step=acknowledge_concern); состояние нигде не мутируется (прецедент производных 2-6/2-7: действующее = чтение).
- Гейт 2-8 не переупорядочивает замороженную лестницу 2-4→2-5→2-6: он добавляет следующий шаг ТОЛЬКО для sales-класса; нейтральный/защищённый пути и их тесты остаются байт-в-байт (контрольная ветка).

## Spec Change Log

- 2026-10-13, SM (первый срез): спека создана по форматам 2-5/2-6/2-7; статус backlog; ожидает CHECKPOINT 1 (предъявление владельцу в ответе сессии, делегированный прецедент 2-6/2-7).
