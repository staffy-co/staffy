-- +goose Up
-- Story 4.5 (Epic 4): Selection Snapshots — ВТОРОЙ CREATE TABLE среза:
-- append-only выбор клиента per work item с монотонным snapshot_no.
-- Пины ТОЧНЫХ версий ВСЕХ ЧЕТЫРЁХ входов AC-13: ProfileDigest (4-2),
-- PartRequestDigest (4-3), FitmentDigest (4-4) и OfferDigest (запись
-- 00022) — «клиент подтверждает конкретные позиции, количество, цену и
-- срок до создания Order». Известная клиентская сумма — ЕДИНСТВЕННАЯ
-- арифметика среза: количество × записанная клиентская цена (AC-4;
-- рендер всегда со словом «без доставки и сбора — окончательная
-- стоимость не рассчитана»; термин «итого» НЕ используется). БЕЗ создания
-- ABCP Order и БЕЗ отправки поставщику — fence OQ-3/OQ-4 (SPINE:
-- supplier_form/send отсутствует в Action Catalog; OQ-3/OQ-5 PASS не
-- авторизует отправку): заказ строит контур 4-6 по своему подтверждению.
-- Количество ≥ 1 и ≤ наличию предложения (AC-11: разбие́ние заказа между
-- поставщиками и изменение количества без явного правила и подтверждения
-- клиента НЕ выполняются — guard домена отказывает ДО SQL, SQL-зеркало
-- возвращает). Снапшот НЕ редактируется и НЕ удаляется (append-only;
-- отменённый/изменённый выбор — НОВАЯ запись НОВЫМ номером, прецедент
-- суперсида 00022). SelectionDigest — канонический SHA-256 снапшота
-- (байт-стабильный, пин контуров 4-6..4-9).

CREATE TABLE cases.selection_snapshots (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 snapshot_id uuid NOT NULL,
 snapshot_no bigint NOT NULL CHECK (snapshot_no >= 1),
 position_key text NOT NULL,
 supplier_id text NOT NULL CHECK (char_length(supplier_id) BETWEEN 1 AND 128),
 offer_no bigint NOT NULL CHECK (offer_no >= 1),
 offer_digest_pin text NOT NULL CHECK (char_length(offer_digest_pin) = 64),
 quantity int NOT NULL CHECK (quantity >= 1),
 unit_client_price numeric NOT NULL CHECK (unit_client_price >= 0),
 currency text NOT NULL CHECK (currency IN ('RUB')),
 known_client_sum numeric NOT NULL CHECK (known_client_sum >= 0),
 profile_digest_pin text NOT NULL CHECK (char_length(profile_digest_pin) BETWEEN 32 AND 128),
 part_request_digest_pin text NOT NULL CHECK (char_length(part_request_digest_pin) BETWEEN 32 AND 128),
 fitment_digest_pin text NOT NULL CHECK (char_length(fitment_digest_pin) = 64),
 selection_digest text NOT NULL CHECK (char_length(selection_digest) = 64),
 actor_id uuid NOT NULL,
 correlation_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, snapshot_id),
 UNIQUE (tenant_id, work_item_id, snapshot_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 CONSTRAINT selection_position_key_canonical CHECK (
        char_length(position_key) BETWEEN 1 AND 128
        AND position_key = upper(position_key)
        AND position_key NOT LIKE '% %')
);
CREATE INDEX selection_snapshots_work_item ON cases.selection_snapshots (tenant_id, work_item_id, snapshot_no);
CREATE INDEX selection_snapshots_case ON cases.selection_snapshots (tenant_id, case_id, created_at);

-- Команда записи присоединяется к идемпотентной поверхности (47-е слово —
-- прецеденты 00020/00022): сохранённый outcome отвечает на точный повтор,
-- тот же ключ с другим материалом — честный конфликт.
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary','record_correction','promise_handoff_update','record_fitment_evidence','record_supplier_offer','record_client_selection'));

ALTER TABLE cases.selection_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.selection_snapshots FORCE ROW LEVEL SECURITY;

CREATE POLICY selection_snapshots_scope ON cases.selection_snapshots USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only selection surface (INSERT/SELECT); the choice is
-- never edited — a changed mind is a NEW row with a NEW number.
GRANT SELECT, INSERT ON cases.selection_snapshots TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.selection_snapshots FROM staffy_app;

-- +goose Down
DROP TABLE cases.selection_snapshots;

ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary','record_correction','promise_handoff_update','record_fitment_evidence','record_supplier_offer'));
