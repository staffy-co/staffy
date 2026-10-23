-- +goose Up
-- Story 4.4 (Epic 4): Fitment Evidence — менеджерское каталогическое
-- доказательство применимости как ЗАПИСЫВАЕМЫЙ append-only материал.
-- Отличие от Part Request 4-3 (чистого read-time вывода): исследование
-- каталога менеджером есть событие мира — CREATE TABLE по прецедентам
-- 00007 owning_system_observations / 00013 human_handoffs. Append-only:
-- INSERT/SELECT гранты только; суперсид записи источника — НОВЫМ
-- evidence_no per (work item, позиция, источник), обновлений и DELETE
-- нет. Продюсер — оператор-менеджер: автоматический запрос к
-- ABCP/Laximo НЕ выполняется (маршрут parts_catalog_abcp не допущен
-- реестром 4-1: OQ-5/AG-4 open, AdapterImplemented false) — менеджер
-- записывает результат фактического исследования с полным материалом
-- трассируемости до ответа Owning System по дисциплине 2-7 (источник,
-- идентификатор записи, версия, время получения; received_at — наше,
-- source_observed_at — источника, никогда не смешиваются). Пины
-- ProfileDigest (4-2) и PartRequestDigest (4-3) — «точные версии входов»
-- AC-1/AC-11: любая новая версия входа устаревает зависимые
-- доказательства READ-TIME сверкой пинов (лестница домена), без
-- триггеров. Состояния {proven, ambiguous, conflicted, unknown, stale} —
-- read-time деривация домена, НЕ хранимое слово: пустой
-- catalog_vehicle_ids = честная не-найдено запись (исследовано и не
-- найдено — отсутствие доказательства не является ни подтверждением, ни
-- отрицанием, AC-10). CHECK-зеркала словарей пака auto-parts@5.0.0:
-- link_type 2 слова, source_system 2 слова; канонический position_key
-- (верхний регистр, без пробелов — канонизация NormalizePartNumber 4-3);
-- found-запись обязана нести узел и артикул (зеркало guard домена).
-- Ретенция: class case-content периметра 1-10 reused (нового класса нет);
-- полей личности нет, маска не требуется.

CREATE TABLE cases.fitment_evidences (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 evidence_id uuid NOT NULL,
 evidence_no bigint NOT NULL CHECK (evidence_no >= 1),
 position_key text NOT NULL,
 catalog_node text,
 article text,
 link_type text NOT NULL CHECK (link_type IN ('original','cross')),
 chain jsonb NOT NULL CHECK (
        jsonb_typeof(chain) = 'array'
        AND jsonb_array_length(chain) BETWEEN 1 AND 64),
 catalog_vehicle_ids jsonb NOT NULL CHECK (
        jsonb_typeof(catalog_vehicle_ids) = 'array'
        AND jsonb_array_length(catalog_vehicle_ids) BETWEEN 0 AND 16),
 source_system text NOT NULL CHECK (source_system IN ('abcp_catalog','laximo_catalog')),
 source_record_id text NOT NULL CHECK (char_length(source_record_id) BETWEEN 1 AND 128),
 source_version text NOT NULL CHECK (char_length(source_version) BETWEEN 1 AND 64),
 source_observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL,
 actor_id uuid NOT NULL,
 correlation_id uuid NOT NULL,
 profile_digest_pin text NOT NULL CHECK (char_length(profile_digest_pin) BETWEEN 32 AND 128),
 part_request_digest_pin text NOT NULL CHECK (char_length(part_request_digest_pin) BETWEEN 32 AND 128),
 fitment_digest text NOT NULL CHECK (char_length(fitment_digest) = 64),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, evidence_id),
 UNIQUE (tenant_id, work_item_id, position_key, source_system, evidence_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 CONSTRAINT fitment_position_key_canonical CHECK (
        char_length(position_key) BETWEEN 1 AND 128
        AND position_key = upper(position_key)
        AND position_key NOT LIKE '% %'),
 CONSTRAINT fitment_found_carries_material CHECK (
        jsonb_array_length(catalog_vehicle_ids) = 0
        OR (char_length(coalesce(catalog_node,'')) >= 1 AND char_length(coalesce(article,'')) >= 1))
);
CREATE INDEX fitment_evidences_work_item ON cases.fitment_evidences (tenant_id, work_item_id, position_key, source_system, evidence_no);
CREATE INDEX fitment_evidences_case ON cases.fitment_evidences (tenant_id, case_id, created_at);

-- Команда записи присоединяется к идемпотентной поверхности (45-е слово —
-- прецедент 00007 read_owning_system): сохранённый outcome отвечает на
-- точный повтор, тот же ключ с другим материалом — честный конфликт.
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary','record_correction','promise_handoff_update','record_fitment_evidence'));

ALTER TABLE cases.fitment_evidences ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.fitment_evidences FORCE ROW LEVEL SECURITY;

CREATE POLICY fitment_evidences_scope ON cases.fitment_evidences USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only evidence surface (INSERT/SELECT); the proof is
-- never edited — a supersede is a NEW row with a NEW number.
GRANT SELECT, INSERT ON cases.fitment_evidences TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.fitment_evidences FROM staffy_app;

-- +goose Down
DROP TABLE cases.fitment_evidences;

ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary','record_correction','promise_handoff_update'));
