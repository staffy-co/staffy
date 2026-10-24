-- +goose Up
-- Story 4.5 (Epic 4): Supplier Offers — менеджерские коммерческие условия
-- фактически подключённых ABCP-поставщиков как ЗАПИСЫВАЕМЫЙ append-only
-- материал. ПЕРВЫЙ CREATE TABLE среза по прецедентам 00007
-- owning_system_observations / 00013 human_handoffs / 00020
-- fitment_evidences. Отличие от Fitment Evidence 4-4: ДВА материальных
-- объекта среза (предложение 00022 + выбор клиента 00023) — story несёт
-- два РАЗНЫХ материала: условия поставщика и выбор клиента; оба
-- append-only. Продюсер записи — оператор-менеджер: автоматическое
-- обращение к поставщикам ABCP/Laximo НЕ выполняется и НЕ заявляется
-- (маршрут parts_catalog_abcp не допущен реестром 4-1: OQ-5/AG-4 open,
-- AdapterImplemented false — менеджер переносит фактические условия из
-- интерфейса подключённых поставщиков с полным материалом трассируемости
-- до ответа Owning System по дисциплине 2-7: источник, идентификатор
-- записи источника, версия, время ответа; received_at — наше,
-- source_observed_at — источника, никогда не смешиваются). Клиентская
-- цена — ТОЛЬКО записанная разрешённая цена ABCP: модель НЕ рассчитывает
-- скидку/наценку/итог (AC-3); закупочная цена — ОПЦИОНАЛЬНЫЕ внутренние
-- данные магазина (NULL = честное «не видел / не записал», НЕ ноль;
-- клиентскому материалу НЕ принадлежит — AC-2: поля нет в клиентских
-- формах домена, зонд). Срок — ОЦЕНКА поставщика term_kind
-- supplier_estimate: не обещание магазина и не подтверждённый факт
-- исполнения (AC-8); NULL = срок не указан поставщиком. Пины
-- ProfileDigest (4-2) и PartRequestDigest (4-3) ОБЯЗАТЕЛЬНЫ,
-- FitmentDigest (4-4) опционален (предложение без доказательства
-- записывается честно, но НЕ рекомендуется); новая версия ЛЮБОГО входа
-- устаревает зависимые предложения READ-TIME сверкой пинов (лестница
-- домена 4-5), без триггеров. Суперсид записи поставщика — НОВЫМ
-- offer_no per (work item, позиция, поставщик), обновлений и DELETE нет.
-- CHECK-зеркала словарей пака auto-parts@6.0.0: валюта 1 слово, term_kind
-- 1 слово, source_system 2 слова (зеркало 4-4); канонический position_key
-- (верхний регистр, без пробелов — канонизация NormalizePartNumber 4-3).
-- Ретенция: class case-content периметра 1-10 reused (нового класса нет);
-- полей личности нет, маска не требуется; закупочная цена — коммерческие
-- данные магазина внутри периметра Tenant.

CREATE TABLE cases.supplier_offers (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 offer_id uuid NOT NULL,
 offer_no bigint NOT NULL CHECK (offer_no >= 1),
 position_key text NOT NULL,
 supplier_id text NOT NULL CHECK (char_length(supplier_id) BETWEEN 1 AND 128),
 supplier_name text NOT NULL CHECK (char_length(supplier_name) BETWEEN 1 AND 256),
 brand text NOT NULL CHECK (char_length(brand) BETWEEN 1 AND 128),
 article text NOT NULL CHECK (char_length(article) BETWEEN 1 AND 128),
 client_price numeric NOT NULL CHECK (client_price >= 0),
 purchase_price numeric CHECK (purchase_price >= 0),
 currency text NOT NULL CHECK (currency IN ('RUB')),
 quantity_available int NOT NULL CHECK (quantity_available >= 0),
 term_days int CHECK (term_days >= 0),
 term_kind text NOT NULL CHECK (term_kind IN ('supplier_estimate')),
 conditions text CHECK (char_length(coalesce(conditions,'')) <= 512),
 source_system text NOT NULL CHECK (source_system IN ('abcp_catalog','laximo_catalog')),
 source_record_id text NOT NULL CHECK (char_length(source_record_id) BETWEEN 1 AND 128),
 source_version text NOT NULL CHECK (char_length(source_version) BETWEEN 1 AND 64),
 source_observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL,
 actor_id uuid NOT NULL,
 correlation_id uuid NOT NULL,
 profile_digest_pin text NOT NULL CHECK (char_length(profile_digest_pin) BETWEEN 32 AND 128),
 part_request_digest_pin text NOT NULL CHECK (char_length(part_request_digest_pin) BETWEEN 32 AND 128),
 fitment_digest_pin text CHECK (char_length(fitment_digest_pin) = 64),
 offer_digest text NOT NULL CHECK (char_length(offer_digest) = 64),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, offer_id),
 UNIQUE (tenant_id, work_item_id, position_key, supplier_id, offer_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 CONSTRAINT offer_position_key_canonical CHECK (
        char_length(position_key) BETWEEN 1 AND 128
        AND position_key = upper(position_key)
        AND position_key NOT LIKE '% %')
);
CREATE INDEX supplier_offers_work_item ON cases.supplier_offers (tenant_id, work_item_id, position_key, supplier_id, offer_no);
CREATE INDEX supplier_offers_case ON cases.supplier_offers (tenant_id, case_id, created_at);

-- Команда записи присоединяется к идемпотентной поверхности (46-е слово —
-- прецедент 00020 record_fitment_evidence): сохранённый outcome отвечает
-- на точный повтор, тот же ключ с другим материалом — честный конфликт.
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary','record_correction','promise_handoff_update','record_fitment_evidence','record_supplier_offer'));

ALTER TABLE cases.supplier_offers ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.supplier_offers FORCE ROW LEVEL SECURITY;

CREATE POLICY supplier_offers_scope ON cases.supplier_offers USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only offer surface (INSERT/SELECT); the commercial
-- conditions are never edited — a supersede is a NEW row with a NEW
-- number.
GRANT SELECT, INSERT ON cases.supplier_offers TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.supplier_offers FROM staffy_app;

-- +goose Down
DROP TABLE cases.supplier_offers;

ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary','record_correction','promise_handoff_update','record_fitment_evidence'));
