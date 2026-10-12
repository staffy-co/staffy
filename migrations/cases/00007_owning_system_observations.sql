-- +goose Up
-- Story 2.7: the Owning System Observations (first slice). The cases
-- schema stays append-only: owning_system_observations carries INSERT/SELECT
-- grants only. The CURRENT observation of a material field of a Work Item
-- is the read-time derivation (the maximal observation_no) — nothing is
-- ever updated; a re-read supersedes by a NEW row with a bigger number
-- (the provider version moves forward, the old row stays for the audit).
-- The observation number is the per-(Work Item, material field) monotonic
-- counter computed under the case + work item advisory locks of the
-- command. The PROJECTION VALUE never enters the outbox envelopes, the
-- exceptions or the logs — it lives in the observation row and the operator
-- panel only (the 2-6 «кандидат не покидает строку» precedent). The status
-- is the fixed dictionary word {ok, failed, unknown}: ok ⇒ value + external
-- id + provider version + observed_at NOT NULL (the successful read always
-- names its carrier); failed/unknown ⇒ value NULL — the assumed value is
-- never written (AC-6), the failure names its reason word of the fixed
-- vocabulary. observed_at is the provider time, received_at is ours (never
-- mixed — the 2-2 discipline). The matrix dictionary words (the material
-- fields, the owning systems, the projections) are the CHECK backstops of
-- the fixed State/Authority Matrix v1 — the matrix is a Domain Pack v0
-- dictionary, never a tenant configuration.

-- The observations: one append-only row per owning-system read. The
-- material uniqueness (tenant, work_item, material_key, observation_no) is
-- the monotonic counter backstop — two concurrent reads under the advisory
-- locks produce exactly one row per number. The matrix row snapshot (the
-- owning system word, the projection word) travels in the row: the audit
-- reads what the matrix said at the read moment. The external id with the
-- provider version and the provider-observed time are the carrier words of
-- the ok read (NULL for a failure — the read did not complete, nothing was
-- observed). The correlation id ties the row to the command trace.
CREATE TABLE cases.owning_system_observations (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 material_key text NOT NULL CHECK (material_key IN ('price','availability','order_status')),
 observation_no bigint NOT NULL CHECK (observation_no >= 1),
 observation_id uuid NOT NULL,
 owning_system text NOT NULL CHECK (owning_system IN ('catalog','erp')),
 status text NOT NULL CHECK (status IN ('ok','failed','unknown')),
 reason text CHECK (reason IN ('provider_timeout','malformed_response','insufficient_scope','read_error')),
 value jsonb,
 projection text NOT NULL CHECK (projection IN ('price_quote','stock_count','status_word')),
 external_id text CHECK (external_id IS NULL OR char_length(external_id) BETWEEN 1 AND 64),
 external_version text CHECK (external_version IS NULL OR char_length(external_version) BETWEEN 1 AND 64),
 observed_at timestamptz,
 received_at timestamptz NOT NULL,
 actor_id uuid NOT NULL,
 correlation_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, observation_id),
 UNIQUE (tenant_id, work_item_id, material_key, observation_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 CONSTRAINT observation_ok_carries_material CHECK (
   (status = 'ok' AND value IS NOT NULL AND external_id IS NOT NULL AND external_version IS NOT NULL AND observed_at IS NOT NULL)
   OR status <> 'ok'),
 CONSTRAINT observation_failure_carries_no_value CHECK (
   (status <> 'ok' AND value IS NULL) OR status = 'ok')
);
CREATE INDEX owning_system_observations_work_item ON cases.owning_system_observations (tenant_id, work_item_id, material_key, observation_no);
CREATE INDEX owning_system_observations_case ON cases.owning_system_observations (tenant_id, case_id, created_at);

-- The read command joins the idempotency surface (the 2-2..2-6 precedent:
-- the stored outcome replays an exact repeat, the same key with another
-- request hash is an honest conflict — the request hash of
-- read_owning_system covers the material field and the work item anchor).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system'));

-- The observation outbox event type joins the event dictionary of the
-- outbox (the bounded type vocabulary — the envelope carries the
-- identifiers, the dictionary state words and the monotonic number only;
-- the projection value and the external body never enter).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed'));

ALTER TABLE cases.owning_system_observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.owning_system_observations FORCE ROW LEVEL SECURITY;

CREATE POLICY owning_system_observations_scope ON cases.owning_system_observations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the observations are
-- never edited.
GRANT SELECT, INSERT ON cases.owning_system_observations TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.owning_system_observations FROM staffy_app;

-- +goose Down
DROP TABLE cases.owning_system_observations;
