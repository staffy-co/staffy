#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""step-03 OpenAPI патч: reason 3→4, CaseView +fitment, путь записи, схемы-зеркала."""
import io

P = 'contracts/openapi/staffy.yaml'
s = io.open(P, encoding='utf-8').read()

# 1) RequestHumanRequest.reason enum 3→4 (Story 4.4 слово fitment_research)
old = """        reason: { type: string, enum: [diagnostic_uncertainty, safety_critical, unresolved_contradiction], description: 'Слово границы запроса детали (Story 4.3 — операторский продюсер по рекомендации панели; пусто = безусловная просьба клиента 2.13; явный client_request вводом НЕВОЗМОЖЕН — системная истина; словарь — зеркало миграции cases/00019)' }"""
new = """        reason: { type: string, enum: [diagnostic_uncertainty, safety_critical, unresolved_contradiction, fitment_research], description: 'Слово границы (Story 4.3 — операторский продюсер по рекомендации панели «Запрос детали»; Story 4.4 — слово каталогического исследования по рекомендации панели «Применимость»; пусто = безусловная просьба клиента 2.13; явный client_request вводом НЕВОЗМОЖЕН — системная истина; словарь — зеркало миграции cases/00019 + cases/00021, append-only)' }"""
assert old in s, 'reason enum anchor not found'
s = s.replace(old, new, 1)

# 2) CaseView +fitment (после part_request)
old2 = """        part_request: { $ref: '#/components/schemas/PartRequestPanelView', description: 'Секция «Запрос детали» (Story 4.3): read-time запрос по схеме пака (режим/атрибуты/полнота/digest/границы); schema_defined=false — честное слово отсутствия схемы (без хардкода)' }"""
new2 = old2 + """
        fitment: { $ref: '#/components/schemas/FitmentPanelView', description: 'Секция «Применимость» (Story 4.4): read-time лестница состояний над append-only записями доказательств с пинами ProfileDigest/PartRequestDigest; schema_defined=false — честное слово отсутствия политики пака (без хардкода)' }"""
assert old2 in s, 'CaseView part_request anchor not found'
s = s.replace(old2, new2, 1)

# 3) Путь записи: вставим перед /v1/tenants/{tenant_id}/cases/{case_id}/human-requests:
anchor = """  /v1/tenants/{tenant_id}/cases/{case_id}/human-requests:
    parameters:
      - $ref: '#/components/parameters/TenantID'
      - $ref: '#/components/parameters/CaseID'
    post:
      operationId: requestHuman"""
assert anchor in s, 'human-requests path anchor not found'
fitment_path = """  /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/fitment-evidences:
    parameters:
      - $ref: '#/components/parameters/TenantID'
      - $ref: '#/components/parameters/CaseID'
      - $ref: '#/components/parameters/WorkItemID'
    post:
      operationId: recordFitmentEvidence
      summary: 'Записать результат каталогического исследования применимости (Story 4-4, AC-2): продюсер — оператор-менеджер, автоматический запрос к каталогу НЕ выполняется (маршрут parts_catalog_abcp не допущен реестром 4-1 — OQ-5/AG-4); append-only строка с монотонным evidence_no per (work item, позиция, источник) — суперсид НОВЫМ номером, обновлений нет; пины ProfileDigest/PartRequestDigest НЕ поля запроса — живые деривации app ДО записи (AC-1/AC-11); guard отказывает честными словами ДО SQL (словари пака/глубина цепочки/канонический position_key/материал found/трассируемость/оба пина); канонический FitmentDigest — серверная истина под локами'
      x-staffy-principal-required: true
      parameters:
        - $ref: '#/components/parameters/IdempotencyKey'
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/RecordFitmentEvidenceRequest' }
      responses:
        '201': { description: 'Доказательство записано (append-only; номер — серверная истина под локами)', content: { application/json: { schema: { $ref: '#/components/schemas/WorkOutcome' } } } }
        '200': { description: Повтор идемпотентного ключа — сохранённый outcome, content: { application/json: { schema: { $ref: '#/components/schemas/WorkOutcome' } } } }
        '400': { description: 'Словари пака (тип связи/источник) / форма цепочки (original=1 узел; cross 2..MaxCrossDepth+1) / канонический position_key / материал found (узел+артикул при найденном) / трассируемость (идентификатор записи, версия, время) — ДО SQL словами словаря', content: { application/json: { schema: { $ref: '#/components/schemas/Problem' } } } }
        '401': { $ref: '#/components/responses/Problem' }
        '403': { description: 'Гейт прав ДО стражей: команда записи требует review или configuration', content: { application/json: { schema: { $ref: '#/components/schemas/Problem' } } } }
        '404': { $ref: '#/components/responses/Problem' }
        '409': { description: 'Тот же ключ с иным материалом — command_key_conflict', content: { application/json: { schema: { $ref: '#/components/schemas/Problem' } } } }
""" + anchor
s = s.replace(anchor, fitment_path, 1)

# 4) Схемы: вставим после RequestHumanRequest блока (перед AssignHumanHandoffRequest)
anchor2 = """    AssignHumanHandoffRequest:"""
assert anchor2 in s, 'AssignHumanHandoffRequest anchor not found'
schemas = """    RecordFitmentEvidenceRequest:
      type: object
      required: [position_key, link_type, source_system, source_record_id, source_version, source_observed_at]
      additionalProperties: false
      description: 'Результат каталогического исследования применимости (Story 4.4, AC-2): материал записи — позиция (канонический нормализованный артикул 4-3), found/not-found материал (узел, артикул, тип связи, cross-цепочка ЦЕЛИКОМ, идентификаторы автомобиля каталога — пусто = исследовано и не найдено, AC-10), трассируемость до ответа Owning System (источник, идентификатор записи, версия, время получения — дисциплина 2-7); пины digest НЕ поля запроса — app выводит их живыми деривациями 4-2/4-3 ДО записи; evidence_no — серверная истина под локами'
      properties:
        position_key: { type: string, minLength: 1, maxLength: 128, description: 'Канонический ключ позиции: нормализованное представление артикула (верхний регистр, без пробелов, унифицированные разделители — NormalizePartNumber 4-3)' }
        catalog_node: { type: string, maxLength: 256, description: 'Каталогический узел или группа детали (обязателен при найденном доказательстве — guard)' }
        article: { type: string, maxLength: 128, description: 'Артикул/OEM применимой позиции (обязателен при найденном доказательстве — guard)' }
        link_type: { type: string, enum: [original, cross], description: 'Тип связи — словарь Domain Policy пака (зеркало SQL-CHECK cases/00020)' }
        chain: { type: array, minItems: 1, maxItems: 64, items: { type: string, minLength: 1 }, description: 'Cross-цепочка ЦЕЛИКОМ от применимой оригинальной позиции до аналога (AC-4; original — один узел; глубина cross ≤ MaxCrossDepth пака — guard)' }
        catalog_vehicle_ids: { type: array, maxItems: 16, items: { type: string, minLength: 1, maxLength: 64 }, description: 'Идентификаторы автомобиля каталога: один = однозначно, несколько = неоднозначно (ambiguous), ПУСТО = исследовано и не найдено (честная not-found запись — никогда «неприменимо», AC-10)' }
        source_system: { type: string, enum: [abcp_catalog, laximo_catalog], description: 'Источник исследования — словарь Domain Policy пака (зеркало SQL-CHECK cases/00020)' }
        source_record_id: { type: string, minLength: 1, maxLength: 128, description: 'Идентификатор записи источника — трассируемость до ответа Owning System (2-7)' }
        source_version: { type: string, minLength: 1, maxLength: 64, description: 'Версия источника — трассируемость до ответа Owning System (2-7)' }
        source_observed_at: { type: string, format: date-time, description: 'Время получения материала источника (2-7 — никогда не смешивается с received_at ours)' }
    FitmentPanelView:
      type: object
      required: [schema_defined, fitment]
      additionalProperties: false
      description: 'Секция «Применимость» (Story 4.4): read-time лестница состояний над append-only записями доказательств; schema_defined=false — честное слово отсутствия политики пака (панель отсутствует, чтение НИКОГДА не падает)'
      properties:
        schema_defined: { type: boolean }
        note: { type: string }
        fitment: { $ref: '#/components/schemas/FitmentView' }
    FitmentView:
      type: object
      required: [positions, number_match_word, price_note, scope_word]
      additionalProperties: false
      description: 'Применимость кейса read-time (Story 4.4): позиции с состояниями {proven, ambiguous, conflicted, unknown, stale}; Constant-слово 4-3 (совпадение номера ≠ доказательство); цена/наличие/срок — не обязательства (контур 4-5); scope одного обращения (AC-8)'
      properties:
        positions: { type: array, items: { $ref: '#/components/schemas/FitmentPositionView' } }
        case_unknown_word: { type: string, description: 'Честное слово отсутствия доказательств кейса (AC-10: не подтверждение и не отрицание) + рекомендация исследования' }
        case_handoff_words: { type: array, items: { type: string, enum: [fitment_research], description: 'Рекомендация задачи менеджеру — словарь причин 00021' } }
        number_match_word: { type: string, description: 'Constant-слово 4-3: точное совпадение номера не считается доказательством применимости' }
        price_note: { type: string, description: 'Цена/наличие/срок — не зафиксированные обязательства: актуальные предложения контуром 4-5 (AC-12)' }
        scope_word: { type: string, description: 'Решение применимо в пределах одного обращения (AC-8)' }
    FitmentPositionView:
      type: object
      required: [position_key, status, status_word, words, evidences, questions, handoff_words]
      additionalProperties: false
      description: 'Одна позиция применимости: состояние лестницы read-time (текущая запись источника = максимальный номер; суперсид номером, не обновлением), материал записей (ОБЕ при противоречии — AC-9), FitmentDigest ТОЛЬКО в proven (точная версия доказательства — пин контуров 4-5..4-9)'
      properties:
        position_key: { type: string }
        status: { type: string, enum: [proven, ambiguous, conflicted, unknown, stale], description: 'Состояние лестницы — словарь домена (зеркало контракта, drift = красный)' }
        status_word: { type: string, description: 'Честное слово состояния текстом (не только цвет — AC-13/AC-14)' }
        words: { type: array, items: { type: string }, description: 'Дополнительные слова (safety-нота 4-3 на safety-узле; слово повторной проверки при stale)' }
        evidences: { type: array, items: { $ref: '#/components/schemas/FitmentEvidenceView' } }
        fitment_digest: { type: string, description: 'SHA-256 канонического {position_key, link_type, chain, source, source_version, pins} — присутствует ТОЛЬКО в proven' }
        questions: { type: object, additionalProperties: { type: string }, description: 'Минимальные вопросы недостающих признаков профиля (AC-5: подтверждённые не переспрашиваются)' }
        handoff_words: { type: array, items: { type: string, enum: [fitment_research, unresolved_contradiction], description: 'Рекомендации передачи человеку — словари 00019/00021' } }
    FitmentEvidenceView:
      type: object
      required: [evidence_no, source_system, source_record_id, source_version, source_observed_at, received_at, link_type, chain, catalog_vehicle_ids, catalog_node, article, profile_digest_pin, part_request_digest_pin, stale_words, found]
      additionalProperties: false
      description: 'Одна append-only запись доказательства (материал трассируемости 2-7: источник, идентификатор записи, версия, оба времени; пины точных версий входов AC-1/AC-11; слова устаревания записи — возраст TTL / пин профиля / пин запроса)'
      properties:
        evidence_no: { type: integer, minimum: 1, description: 'Монотонный номер per (work item, позиция, источник) — серверная истина под локами' }
        source_system: { type: string, enum: [abcp_catalog, laximo_catalog], description: 'Источник — словарь пака (зеркало SQL-CHECK cases/00020)' }
        source_record_id: { type: string }
        source_version: { type: string }
        source_observed_at: { type: string, format: date-time }
        received_at: { type: string, format: date-time }
        link_type: { type: string, enum: [original, cross], description: 'Тип связи — словарь пака (зеркало SQL-CHECK cases/00020)' }
        chain: { type: array, items: { type: string }, description: 'Cross-цепочка ЦЕЛИКОМ (AC-4)' }
        catalog_vehicle_ids: { type: array, items: { type: string }, description: 'Идентификаторы автомобиля каталога (пусто = не-найдено)' }
        catalog_node: { type: string }
        article: { type: string }
        profile_digest_pin: { type: string, description: 'Пин ProfileDigest 4-2 (точная версия профиля момента записи)' }
        part_request_digest_pin: { type: string, description: 'Пин PartRequestDigest 4-3 (точная версия запроса момента записи)' }
        stale_words: { type: array, items: { type: string }, description: 'Слова устаревания ЭТОЙ записи (возраст/пин профиля/пин запроса — AC-11)' }
        found: { type: boolean, description: 'Найдено (идентификаторы присутствуют) — false = исследовано и не найдено' }
""" + anchor2
s = s.replace(anchor2, schemas, 1)

io.open(P, 'w', encoding='utf-8').write(s)
print('openapi: +reason 4, +CaseView.fitment, +path, +5 schemas')
