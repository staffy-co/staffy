-- +goose Up
-- Story 2.12 (first slice): the degraded route slots, the honest reply
-- delivery evidence, the degraded permitted paths of the Policy v3 and
-- the update promise word of the replies.
--
-- A route degradation is an OPERATIONAL record of the service, never a
-- fact of the case: the table lives in the case operations class with
-- INSERT/SELECT grants only — no confirmation statuses, no fact
-- versions, no facts digest, no identity proof (the perimeter probes
-- assert the absence of the rows in the fact channels; the preferences
-- precedent of 2.11). The slot is scoped to the ROUTE — the
-- (connection, channel) pair of one of the case's conversations: an
-- independent route (another connection or another channel) is never
-- declared degraded without its own row (the route guard of the store
-- requires the route to exist in the conversations of the case).
--
-- The history is append-only: every route of the case carries monotonic
-- revisions (the UNIQUE backstop); a new cause or a recovery is a NEW
-- revision; the CURRENT slots are the read-time derivation of the
-- domain (DeriveCurrentDegradations: the latest revision of every route
-- key; a clearing revision restores the route). The note carries the
-- operator's operational material inside the row only — the outbox
-- envelope carries the route words, the cause word, the revision and
-- the clearing mark only, the note never leaves the table.
--
-- The delivery evidence events are the relayed connector observations
-- (the ConfirmerOperatorRelay precedent of 2.2): append-only, monotonic
-- per reply (the UNIQUE backstop), the kind is the three-word
-- dictionary. The delivery STATE of a reply is the read-time derivation
-- (DeriveReplyDeliveryState): dispatched without events is the honest
-- unknown — never delivered; the word delivered_confirmed is only ever
-- an evidence observation. The reply content never enters the envelope.
-- The blind retry is structurally impossible: the drain stays
-- single-record (the 2.4 UNIQUE), a retry command does not exist.
--
-- The Policy v3 dictionaries: the outcomes of a degraded dialog route
-- may carry the permitted continuation paths (the words of the fixed
-- pack dictionary with their honest texts rendered by the panels — the
-- texts never enter the database), the class dictionary gains the
-- degraded_mode word, the reason dictionary gains the degraded_route
-- word, the rule version dictionary gains the v3 word (the historical
-- v1/v2 rows stay readable — append-only, never recomputed). The update
-- promise is the single nullable client_replies column (the
-- accuracy_qualifier 2.11 / playbook_step 2.8 precedent: the column
-- records what the gate read; the integrity is the gate's own
-- re-validation under the locks).

CREATE TABLE cases.route_degradations (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 record_id uuid NOT NULL,
 connection_id uuid NOT NULL,
 channel text NOT NULL CHECK (channel IN ('phone','web_chat','telegram','whatsapp','email','max','jivo','avito','mobile_app_chat')),
 cause text NOT NULL CHECK (cause IN ('timeout','disconnect','rate_limited','capability_revoked','delivery_unknown')),
 revision bigint NOT NULL CHECK (revision >= 1),
 cleared boolean NOT NULL,
 note text NOT NULL DEFAULT '' CHECK (char_length(note) <= 500),
 actor_id uuid NOT NULL,
 recorded_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, record_id),
 UNIQUE (tenant_id, case_id, connection_id, channel, revision),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id)
);
CREATE INDEX route_degradations_case ON cases.route_degradations (tenant_id, case_id, recorded_at);

-- The append-only delivery evidence of the outgoing replies (the
-- relayed connector observations; the reply content never leaves its
-- row).
CREATE TABLE cases.client_reply_delivery_events (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 reply_id uuid NOT NULL,
 event_id uuid NOT NULL,
 event_no bigint NOT NULL CHECK (event_no >= 1),
 kind text NOT NULL CHECK (kind IN ('delivered_confirmed','delivery_unknown','provider_failed')),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id),
 UNIQUE (tenant_id, reply_id, event_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, reply_id) REFERENCES cases.client_replies(tenant_id, reply_id)
);
CREATE INDEX client_reply_delivery_events_case ON cases.client_reply_delivery_events (tenant_id, case_id, reply_id, event_no);

-- The permitted continuation paths of the degraded outcomes (the words
-- of the fixed pack dictionary only; the pre-migration rows read as the
-- honest empty array — the default lands with the column, the rows are
-- never recomputed).
ALTER TABLE cases.policy_outcomes ADD COLUMN degraded_permitted_paths jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(degraded_permitted_paths) = 'array' AND jsonb_array_length(degraded_permitted_paths) <= 5 AND degraded_permitted_paths <@ '["accept_info_no_promise","verified_channel_switch","human_handoff","agree_callback","stop_path"]'::jsonb);
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT policy_outcomes_outcome_class_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_outcome_class_check CHECK (outcome_class IN ('commercial_answer','informational_answer','clarify','mode_blocked','degraded_mode','fail_closed'));
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT policy_outcomes_reason_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_reason_check CHECK (reason IN ('ok_commercial','ok_informational','mandatory_missing','no_answer_basis','mode_blocks_commercial_path','no_alternative_path','degraded_route','unknown_intent'));
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT policy_outcomes_rule_version_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_rule_version_check CHECK (rule_version IN ('intake-policy-v1','intake-policy-v2','intake-policy-v3'));

-- The update promise word of the reply on a degraded route (the nullable
-- word of the approved policy dictionary; the healthy-route rows and the
-- pre-migration rows carry the honest NULL).
ALTER TABLE cases.client_replies ADD COLUMN update_promise text CHECK (update_promise IS NULL OR update_promise IN ('no_promise','within_1h','within_4h','within_next_business_day'));

-- The two commands join the idempotency surface (the 2.2..2.11
-- precedent: the stored outcome replays an exact repeat, the same key
-- with another request hash is an honest conflict).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence'));

-- The two outbox event types join the bounded vocabulary (the envelopes
-- carry the identifiers and the dictionary words only; the degradation
-- note and the reply content never enter).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence'));

ALTER TABLE cases.route_degradations ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.route_degradations FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.client_reply_delivery_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.client_reply_delivery_events FORCE ROW LEVEL SECURITY;

CREATE POLICY route_degradations_scope ON cases.route_degradations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY client_reply_delivery_events_scope ON cases.client_reply_delivery_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only operational surfaces (INSERT/SELECT); the
-- degradations and the evidence events are never edited — a change is a
-- new revision, a new event.
GRANT SELECT, INSERT ON cases.route_degradations TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.route_degradations FROM staffy_app;
GRANT SELECT, INSERT ON cases.client_reply_delivery_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.client_reply_delivery_events FROM staffy_app;

-- +goose Down
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS update_promise;
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT IF EXISTS policy_outcomes_rule_version_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_rule_version_check CHECK (rule_version IN ('intake-policy-v1','intake-policy-v2'));
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT IF EXISTS policy_outcomes_reason_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_reason_check CHECK (reason IN ('ok_commercial','ok_informational','mandatory_missing','no_answer_basis','mode_blocks_commercial_path','no_alternative_path','unknown_intent'));
ALTER TABLE cases.policy_outcomes DROP CONSTRAINT IF EXISTS policy_outcomes_outcome_class_check;
ALTER TABLE cases.policy_outcomes ADD CONSTRAINT policy_outcomes_outcome_class_check CHECK (outcome_class IN ('commercial_answer','informational_answer','clarify','mode_blocked','fail_closed'));
ALTER TABLE cases.policy_outcomes DROP COLUMN IF EXISTS degraded_permitted_paths;
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation'));
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded'));
DROP TABLE cases.client_reply_delivery_events;
DROP TABLE cases.route_degradations;
