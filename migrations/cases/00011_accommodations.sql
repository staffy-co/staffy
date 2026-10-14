-- +goose Up
-- Story 2.11: the recorded communication preferences (first slice: the
-- append-only revision table of the operational accommodations and the
-- honest alternative paths of the Policy v2). A preference is an
-- OPERATIONAL setting, never a fact: the table lives in the case
-- operations class with INSERT/SELECT grants only — no confirmation
-- statuses, no fact versions, no facts digest, never a medical diagnosis
-- and never an identity proof (the perimeter probes assert the absence of
-- the rows in the fact channels).
--
-- The history is append-only: every key of the case carries monotonic
-- revisions (the UNIQUE backstop), a NEW value or a clearing is a NEW
-- revision; the CURRENT set is the read-time derivation of the domain
-- (DeriveCurrentAccommodations: the latest revision of every key; a
-- clearing revision removes the key from the active set). The valued keys
-- carry their material inside the row only: the preferred channel word of
-- the 2.1 dictionary and the free availability line (an operational
-- string, never a schedule) — the outbox envelope carries the key, the
-- revision and the clearing mark only, the value and the note never leave
-- the table.
--
-- The Policy v2 dictionaries: the outcomes may carry the active
-- alternatives of the honest machine offer (the words of the fixed pack
-- dictionary with their accuracy-limitation texts rendered by the panels —
-- the texts never enter the database), the reason dictionary gains the
-- no_alternative_path fail-closed stop, the rule version dictionary gains
-- the v2 word (the historical v1 rows stay readable — append-only, never
-- recomputed). The sales reply accuracy qualifier is the single nullable
-- client_replies column (the playbook_step 2.8 / link_id 2.10 precedent:
-- the column records what the gate read; the integrity is the gate's own
-- re-validation under the locks).

CREATE TABLE cases.case_accommodations (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 record_id uuid NOT NULL,
 accommodation_key text NOT NULL CHECK (accommodation_key IN ('no_photo','no_web','no_voice','text_first','slowed_pace','mandatory_readback','preferred_channel','available_time')),
 revision bigint NOT NULL CHECK (revision >= 1),
 value text,
 cleared boolean NOT NULL,
 note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
 actor_id uuid NOT NULL,
 recorded_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, record_id),
 UNIQUE (tenant_id, case_id, accommodation_key, revision),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 CONSTRAINT accommodation_value_shape CHECK (
   (accommodation_key IN ('no_photo','no_web','no_voice','text_first','slowed_pace','mandatory_readback') AND value IS NULL)
   OR (accommodation_key = 'preferred_channel' AND value IS NOT NULL AND value IN ('phone','web_chat','telegram','whatsapp','email','max','jivo','avito','mobile_app_chat'))
   OR (accommodation_key = 'available_time' AND value IS NOT NULL AND char_length(value) BETWEEN 1 AND 100)
 ),
 CONSTRAINT accommodation_cleared_shape CHECK (NOT cleared OR value IS NULL)
);
CREATE INDEX case_accommodations_case ON cases.case_accommodations (tenant_id, case_id, recorded_at);

-- The active alternatives of the outcomes (the words of the fixed pack
-- dictionary only; the pre-migration rows read as the honest empty array
-- — the default lands with the column, the rows are never recomputed).
ALTER TABLE cases.policy_outcomes ADD COLUMN active_alternatives jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(active_alternatives) = 'array' AND jsonb_array_length(active_alternatives) <= 4 AND active_alternatives <@ '["text_description","range_estimate","callback","human_handoff"]'::jsonb);
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT policy_outcomes_reason_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_reason_check CHECK (reason IN ('ok_commercial','ok_informational','mandatory_missing','no_answer_basis','mode_blocks_commercial_path','no_alternative_path','unknown_intent'));
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT policy_outcomes_rule_version_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_rule_version_check CHECK (rule_version IN ('intake-policy-v1','intake-policy-v2'));

-- The sales reply accuracy qualifier (the nullable word of the honest
-- limitation claim; the non-sales rows and the pre-migration rows carry
-- the honest NULL).
ALTER TABLE cases.client_replies ADD COLUMN accuracy_qualifier text CHECK (accuracy_qualifier IS NULL OR accuracy_qualifier IN ('text_description','range_estimate','callback','human_handoff'));

-- The one command joins the idempotency surface (the 2.2..2.10
-- precedent: the stored outcome replays an exact repeat, the same key
-- with another request hash is an honest conflict).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation'));

-- The one outbox event type joins the bounded vocabulary (the envelope
-- carries the identifiers, the key word, the revision number and the
-- clearing mark only; the value and the note never enter).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded'));

ALTER TABLE cases.case_accommodations ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.case_accommodations FORCE ROW LEVEL SECURITY;

CREATE POLICY case_accommodations_scope ON cases.case_accommodations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only operational surface (INSERT/SELECT); the
-- preferences are never edited — a change is a new revision.
GRANT SELECT, INSERT ON cases.case_accommodations TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.case_accommodations FROM staffy_app;

-- +goose Down
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS accuracy_qualifier;
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT IF EXISTS policy_outcomes_rule_version_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_rule_version_check CHECK (rule_version IN ('intake-policy-v1'));
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT IF EXISTS policy_outcomes_reason_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_reason_check CHECK (reason IN ('ok_commercial','ok_informational','mandatory_missing','no_answer_basis','mode_blocks_commercial_path','unknown_intent'));
ALTER TABLE cases.policy_outcomes DROP COLUMN IF EXISTS active_alternatives;
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link'));
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned'));
DROP TABLE cases.case_accommodations;
