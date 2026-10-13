-- +goose Up
-- Story 2.8: the Objection Record, the versioned Playbook v1 and the
-- persuasion stop (first slice). The cases schema stays append-only:
-- objections carries INSERT/SELECT grants only. The CURRENT objection of
-- a Work Item is the read-time derivation (the maximal objection_no) —
-- nothing is ever updated; a new record supersedes by a NEW row with a
-- bigger number. The objection number is the per-Work-Item monotonic
-- counter computed under the case + work item advisory locks of the
-- command (the policy_outcomes 2-6 / observations 2-7 precedent). The
-- SPAN TEXT never enters the outbox envelopes, the exceptions or the
-- logs — the client words already live in the sealed payload of 2-1
-- (the «кандидат не покидает строку» precedent of 2.6). The dictionary
-- words (the objection types, the client signals) are the CHECK
-- backstops of the fixed Domain Pack v0 dictionaries — never a tenant
-- configuration. The spans jsonb carries the array-length bound only:
-- the byte-exact substring check of every span against the payload of
-- the turn source is the domain+app honesty (SQL does not count
-- substrings — honestly stated in the spec). The reply gate extension:
-- the sales reply carries the objection reference of the moment (the
-- server-side truth — the request never carries the objection id) and
-- the playbook step word (nullable, only the sales rows carry values;
-- the pre-migration rows carry the honest NULL — the 2-6
-- policy_outcome_id precedent).

-- The objections: one append-only row per recorded client objection. The
-- material uniqueness (tenant, work_item, objection_no) is the monotonic
-- counter backstop — two concurrent records under the advisory locks
-- produce exactly one row per number. The turn reference points at an
-- accepted text event of the case (the span source anchor — the app
-- verifies the byte-exact containment before the insert). The playbook
-- version word is the snapshot of the versioned choice of the moment.
CREATE TABLE cases.objections (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 objection_id uuid NOT NULL,
 objection_no bigint NOT NULL CHECK (objection_no >= 1),
 turn_ref uuid NOT NULL,
 objection_type text NOT NULL CHECK (objection_type IN ('trust','price_comparison','general_doubt')),
 client_signal text NOT NULL CHECK (client_signal IN ('concern','refusal','stop_intent','human_request')),
 spans jsonb NOT NULL CHECK (jsonb_typeof(spans) = 'array' AND jsonb_array_length(spans) BETWEEN 1 AND 8),
 playbook_version text NOT NULL CHECK (char_length(playbook_version) BETWEEN 1 AND 64),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, objection_id),
 UNIQUE (tenant_id, work_item_id, objection_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id)
);
CREATE INDEX objections_wi ON cases.objections (tenant_id, work_item_id, objection_no);
CREATE INDEX objections_case ON cases.objections (tenant_id, case_id, created_at);

-- The objection reference and the playbook step of the client replies:
-- the sales reply gate resolves the CURRENT objection of the Work Item
-- anchor (the request never carries the objection id — the server-side
-- truth); the step word is the honest discipline material of the reply
-- (only the sales rows carry values). The FK-style reference stays
-- unconstrained (the 2-6 precedent: the column records what the gate
-- read; the integrity is the gate's own re-validation under the locks).
ALTER TABLE cases.client_replies ADD COLUMN objection_id uuid;
ALTER TABLE cases.client_replies ADD COLUMN playbook_step text CHECK (playbook_step IS NULL OR playbook_step IN ('acknowledge_concern','present_evidence','honest_no_answer','offer_alternative','clarify_question'));

-- The record command joins the idempotency surface (the 2-2..2-7
-- precedent: the stored outcome replays an exact repeat, the same key
-- with another request hash is an honest conflict — the request hash of
-- record_objection covers the anchors, the turn reference, the
-- dictionary words and the spans).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection'));

-- The objection outbox event type joins the event dictionary of the
-- outbox (the bounded type vocabulary — the envelope carries the
-- identifiers, the dictionary words and the monotonic number only; the
-- span text never enters).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded'));

ALTER TABLE cases.objections ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.objections FORCE ROW LEVEL SECURITY;

CREATE POLICY objections_scope ON cases.objections USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the objections are
-- never edited.
GRANT SELECT, INSERT ON cases.objections TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.objections FROM staffy_app;

-- +goose Down
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS objection_id;
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS playbook_step;
DROP TABLE cases.objections;
