#!/usr/bin/env python3
# step-03 patch G: contracts/openapi/staffy.yaml — +1 path, +AccommodationView,
# PolicyOutcomeView extensions, SendClientReplyRequest +accuracy_qualifier,
# WorkOutcome extensions, ReplyCard +AccuracyQualifier
import io, sys

def patch(path, anchors):
    with io.open(path, encoding="utf-8") as f:
        text = f.read()
    for old, new in anchors:
        if text.count(old) != 1:
            if new in text:
                print("SKIP (already patched)", path)
                continue
            print("FAIL anchor (count=%d) in %s:\n%r" % (text.count(old), path, old[:150]))
            sys.exit(1)
        text = text.replace(old, new)
    with io.open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    print("patched", path)

# 1. The record_accommodation path (after the objections path block) ---
path_old = """  /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments:
    parameters:
      - $ref: '#/components/parameters/TenantID'
      - $ref: '#/components/parameters/CaseID'
      - $ref: '#/components/parameters/WorkItemID'
    post:
      operationId: createCommitment"""
path_new = """  /v1/tenants/{tenant_id}/cases/{case_id}/accommodations:
    parameters:
      - $ref: '#/components/parameters/TenantID'
      - $ref: '#/components/parameters/CaseID'
    post:
      operationId: recordAccommodation
      summary: 'Записать ревизию предпочтения связи (первый срез 2-11): ключ словаря Domain Pack v0, флаг/значимость, значение значимых ключей (канал 2-1 / строка 1..100 рун), снятие своей ревизией; append-only строка + outbox accommodation_recorded (только key/revision/cleared — значение и заметка никогда); ревизия = следующий монотонный номер ключа под advisory-локами (запрос не несёт); текущий набор — read-time деривация'
      x-staffy-principal-required: true
      parameters:
        - $ref: '#/components/parameters/IdempotencyKey'
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/AccommodationRecordRequest' }
      responses:
        '201': { description: 'Ревизия записана (append-only, UNIQUE tenant+case+key+revision) + outbox accommodation_recorded (метаданные: key/revision/cleared/actor — без value и note)', content: { application/json: { schema: { $ref: '#/components/schemas/WorkOutcome' } } } }
        '200': { description: Повтор идемпотентного ключа, content: { application/json: { schema: { $ref: '#/components/schemas/WorkOutcome' } } } }
        '400': { description: 'Словарь ключей / флаг-значимость / слово канала / пределы / снятие со значением / терминальный кейс', content: { application/json: { schema: { $ref: '#/components/schemas/Problem' } } } }
        '401': { $ref: '#/components/responses/Problem' }
        '403': { description: 'Гейт прав ДО стражей: команды записи предпочтений требуют review или configuration', content: { application/json: { schema: { $ref: '#/components/schemas/Problem' } } } }
        '404': { $ref: '#/components/responses/Problem' }
        '409': { description: 'Гонка ревизии (stale_version) / тот же ключ с иным материалом — command_key_conflict', content: { application/json: { schema: { $ref: '#/components/schemas/Problem' } } } }
  /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/commitments:
    parameters:
      - $ref: '#/components/parameters/TenantID'
      - $ref: '#/components/parameters/CaseID'
      - $ref: '#/components/parameters/WorkItemID'
    post:
      operationId: createCommitment"""

# 2. CaseView: the accommodations property ---
caseview_old = """        continuations: { $ref: '#/components/schemas/ContinuationView', description: 'Секция «Ссылки продолжения» (Story 2.10): ссылки с read-time состояниями, delivered-связкой и лентой событий' }"""
caseview_new = """        continuations: { $ref: '#/components/schemas/ContinuationView', description: 'Секция «Ссылки продолжения» (Story 2.10): ссылки с read-time состояниями, delivered-связкой и лентой событий' }
        accommodations: { $ref: '#/components/schemas/AccommodationView', description: 'Секция «Предпочтения связи» (Story 2.11): текущий набор (read-time деривация), append-only история ревизий, словари формы записи' }"""

# 3. PolicyOutcomeView: active_alternatives + the v2 dictionaries ---
outcome_reason_old = """        reason: { type: string, enum: [ok_commercial, ok_informational, mandatory_missing, no_answer_basis, mode_blocks_commercial_path, unknown_intent] }
        rule_version: { type: string, enum: [intake-policy-v1] }"""
outcome_reason_new = """        reason: { type: string, enum: [ok_commercial, ok_informational, mandatory_missing, no_answer_basis, mode_blocks_commercial_path, unknown_intent, no_alternative_path], description: 'no_alternative_path (2-11): у media-ключа пакета нет разрешённых альтернатив — честная остановка с human_handoff' }
        rule_version: { type: string, enum: [intake-policy-v1, intake-policy-v2], description: 'v2 (Story 2.11): 4-й вход — текущие предпочтения связи; исторические строки v1 читаемы, не пересчитываются' }
        active_alternatives: { type: array, items: { type: string, enum: [text_description, range_estimate, callback, human_handoff] }, description: 'Активные альтернативы (Story 2.11): пара «media-usual факт не подтверждён ∧ no_photo активен» — весь словарь альтернатив пака; вопрос факта остаётся в next_steps; пустой словарь пака → fail_closed no_alternative_path' }"""

# 4. PolicyInputSnapshot: the preferences map ---
snapshot_old = """    PolicyInputSnapshot:
      type: object
      required: [proposal_id, schema_version, mode_version, owner_epoch, facts_digest, facts_digest_hash, work_item_version, rule_version]"""
snapshot_new = """    PolicyInputSnapshot:
      type: object
      required: [proposal_id, schema_version, mode_version, owner_epoch, facts_digest, facts_digest_hash, work_item_version, rule_version, preferences]"""
snapshot_props_old = """        facts_digest_hash: { type: string }
        work_item_version: { type: integer }
        rule_version: { type: string, enum: [intake-policy-v1] }"""
snapshot_props_new = """        facts_digest_hash: { type: string }
        work_item_version: { type: integer }
        rule_version: { type: string, enum: [intake-policy-v1, intake-policy-v2] }
        preferences: { type: object, description: 'Каноническая карта preferences (Story 2.11): key → {revision, cleared} — advisory 4-й вход, изменение = честный supersede дайджеста', additionalProperties: { type: object, required: [revision, cleared], properties: { revision: { type: integer }, cleared: { type: boolean } } } }"""

# 5. SendClientReplyRequest: the accuracy qualifier ---
reply_qual_old = """        link_id: { type: string, format: uuid, description: 'Связь ссылки продолжения (Story 2-10, гейт ПОСЛЕ commitment-гейта): только sales-класс; ссылка обязана существовать в этом кейсе; связка = delivered-бейдж панели FR-16; URL живёт текстом ответа, токен — не поле' }"""
reply_qual_new = """        link_id: { type: string, format: uuid, description: 'Связь ссылки продолжения (Story 2-10, гейт ПОСЛЕ commitment-гейта): только sales-класс; ссылка обязана существовать в этом кейсе; связка = delivered-бейдж панели FR-16; URL живёт текстом ответа, токен — не поле' }
        accuracy_qualifier: { type: string, enum: [text_description, range_estimate, callback, human_handoff], description: 'Ограничение точности (Story 2-11, гейт ПОСЛЕ objection-гейта 2-8, ДО записи): только sales-класс; действующий исход с активными альтернативами ОБЯЗАН получить слово активного множества исхода (подмножество исхода, не словарь вообще — 400 accuracy_qualifier_required); без альтернатив поле обязано быть пустым (400 accuracy_qualifier_foreign); отсутствие media не становится выдуманным точным результатом' }"""

# 6. WorkOutcome: the command word + the 2.11 fields ---
outcome_cmd_old = """        command: { type: string, enum: [create_work_item, classify_work_item, transition_work_item, split_work_item, merge_work_items, link_event, dispute_work_item, record_fact, confirm_fact_version, rescope_fact, start_verification, bind_channel, unlink_channel, resolve_duplicate, take_ownership, request_transfer, resolve_transfer, reassign_ownership, send_client_reply, propose_mode, exit_mode, record_disclosure, record_disclosure_event, submit_ai_proposal, evaluate_policy, read_owning_system, record_objection, issue_continuation_link, revoke_continuation_link] }"""
outcome_cmd_new = """        command: { type: string, enum: [create_work_item, classify_work_item, transition_work_item, split_work_item, merge_work_items, link_event, dispute_work_item, record_fact, confirm_fact_version, rescope_fact, start_verification, bind_channel, unlink_channel, resolve_duplicate, take_ownership, request_transfer, resolve_transfer, reassign_ownership, send_client_reply, propose_mode, exit_mode, record_disclosure, record_disclosure_event, submit_ai_proposal, evaluate_policy, read_owning_system, record_objection, issue_continuation_link, revoke_continuation_link, record_accommodation] }"""
outcome_fields_old = """        continuation_issued: { type: boolean, description: 'Маска выдачи ссылок продолжения: токен никогда в исходе' }
        continuation_expires_at: { type: string, format: date-time, description: 'Истечение выданной ссылки' }"""
outcome_fields_new = """        continuation_issued: { type: boolean, description: 'Маска выдачи ссылок продолжения: токен никогда в исходе' }
        continuation_expires_at: { type: string, format: date-time, description: 'Истечение выданной ссылки' }
        accommodation_record_id: { type: string, format: uuid, description: 'Записанная ревизия предпочтения связи (Story 2.11); номер ревизии — серверная истина, в исходе не хранится' }
        accommodation_key: { type: string, enum: [no_photo, no_web, no_voice, text_first, slowed_pace, mandatory_readback, preferred_channel, available_time] }
        accommodation_cleared: { type: boolean }
        accuracy_qualifier: { type: string, enum: [text_description, range_estimate, callback, human_handoff], description: 'Слово ограничения точности продажного ответа (Story 2.11) — строка ответа несёт его' }"""

# 7. ReplyCard: the qualifier word of the reply row ---
replycard_old = """        Reply: { type: object, properties: { ReplyID: { type: string, format: uuid }, ConversationID: { type: string, format: uuid }, Epoch: { type: integer }, Channel: { type: string }, Content: { type: string }, CreatedAt: { type: string, format: date-time } } }"""
replycard_new = """        Reply: { type: object, properties: { ReplyID: { type: string, format: uuid }, ConversationID: { type: string, format: uuid }, Epoch: { type: integer }, Channel: { type: string }, Content: { type: string }, AccuracyQualifier: { type: string, enum: [text_description, range_estimate, callback, human_handoff], description: 'Слово ограничения точности (Story 2.11) — только sales-строки; пусто = исход без активных альтернатив' }, CreatedAt: { type: string, format: date-time } } }"""

# 8. The AccommodationRecordRequest + AccommodationView schemas (after ObjectionRecordRequest) ---
schemas_anchor = """    ObjectionRecordRequest:
      type: object
      required: [turn_ref, objection_type, client_signal, spans]
      additionalProperties: false"""
schemas_new = """    AccommodationRecordRequest:
      type: object
      required: [accommodation_key]
      additionalProperties: false
      description: 'Запись одной ревизии предпочтения связи: ключ фиксированного словаря Domain Pack v0; флаги не несут значение, значимые требуют; снятие — своей ревизией, никогда вместе со значением; ревизию назначает сервер под локами'
      properties:
        accommodation_key: { type: string, enum: [no_photo, no_web, no_voice, text_first, slowed_pace, mandatory_readback, preferred_channel, available_time], description: 'Ключ предпочтения (словарь фиксирован compile-time — не конфигурация тенанта)' }
        value: { type: string, maxLength: 100, description: 'Значение значимого ключа: preferred_channel — слово словаря каналов 2-1; available_time — свободная операционная строка 1..100 рун (не расписание, машиной не парсится)' }
        cleared: { type: boolean, description: 'Снятие предпочтения: ревизия без значения; ключ покидает текущий набор (деривация чтения)' }
        note: { type: string, maxLength: 500, description: 'Заметка оператора: остаётся в строке, никогда не покидает таблицу (outbox — только key/revision/cleared)' }
    AccommodationView:
      type: object
      required: [current, history, key_words, channel_words, records_count, panel_text]
      description: 'Секция «Предпочтения связи» (Story 2.11): текущий набор — read-time деривация последней ревизии каждого ключа (cleared убирает ключ из активного набора, оставаясь в состоянии честно); история — append-only, включая снятия; операционная настройка, не факт'
      properties:
        current: { type: array, items: { type: object, required: [key, revision, cleared], properties: { key: { type: string, enum: [no_photo, no_web, no_voice, text_first, slowed_pace, mandatory_readback, preferred_channel, available_time] }, revision: { type: integer }, cleared: { type: boolean }, value: { type: string } } } }
        history: { type: array, items: { type: object, required: [key, revision, cleared, actor_id, recorded_at], properties: { key: { type: string }, revision: { type: integer }, value: { type: string }, cleared: { type: boolean }, note: { type: string }, actor_id: { type: string, format: uuid }, recorded_at: { type: string, format: date-time } } } }
        key_words: { type: array, items: { type: string } }
        channel_words: { type: array, items: { type: string } }
        records_count: { type: integer, description: 'Число строк истории — якорь формы записи (защита двойного сабмита PRG)' }
        panel_text: { type: string }
    ObjectionRecordRequest:
      type: object
      required: [turn_ref, objection_type, client_signal, spans]
      additionalProperties: false"""

patch("contracts/openapi/staffy.yaml", [
    (path_old, path_new),
    (caseview_old, caseview_new),
    (outcome_reason_old, outcome_reason_new),
    (snapshot_old, snapshot_new),
    (snapshot_props_old, snapshot_props_new),
    (reply_qual_old, reply_qual_new),
    (outcome_cmd_old, outcome_cmd_new),
    (outcome_fields_old, outcome_fields_new),
    (replycard_old, replycard_new),
    (schemas_anchor, schemas_new),
])

print("ALL PATCHES OK")
