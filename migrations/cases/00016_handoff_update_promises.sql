-- +goose Up
-- Story 2.16 (first slice): the truthful waiting updates.
--
-- An update promise is one append-only OPERATOR act over the human
-- handoff round of THIS case: the operator promises the NEXT client
-- update with a deadline WORD of the fixed dictionary. The honest
-- update_unknown word exists for the case where no confirmed ETA
-- exists (FR-41: the absence of a confirmed ETA is named directly —
-- never replaced by a fabricated number): the material honesty is the
-- CHECK pair below — the unknown word NEVER carries a promised time,
-- a known word NEVER lives without the computed server time. A
-- fabricated timestamp is structurally impossible in the table.
--
-- The promised time and the promise number are the SERVER truths
-- (never request fields — the strict decode of the transport refuses
-- them): the store computes now + window under the advisory locks and
-- derives the NEXT monotonic number (the UNIQUE backstop catches the
-- races the locks already serialize). The row NEVER mutates: a new
-- promise is a new row — the former rows derive superseded read-time
-- (the derivation-only supersede of the 2.15 correction precedent:
-- both rows live, the silent rewrite is structurally impossible). The
-- missed lateness is a READ-TIME derivation too — the row and the
-- round are never mutated by it, and the promise NEVER resolves the
-- round (the lifecycle transitions belong to the existing 2.13
-- commands with their own guards).
--
-- The promise NEVER replaces the reply itself: the update is sent by
-- the HUMAN through the existing reply path (2.4) — the machine
-- stores the promise and shows the truth (no timers, no automatic
-- sends, no escalation automation — the producer is external).
--
-- The request anchor pins the EXACT handoff round of THIS case (the
-- FK to human_handoffs); the store guards the open lifecycle with its
-- own mirror words (requested, assigned, overdue or acknowledged
-- accept; the declined round consumed the assignment — the
-- re-assignment decision belongs to the human; the cancelled and
-- returned rounds ended the waiting honestly). The responsible actor
-- is the living material of the command (AC-2 — the record carries
-- the operator, the client text never enters the database).
--
-- The ONE outbox event type joins the bounded vocabulary: the
-- envelope carries the identifiers and the deadline word ONLY — the
-- client texts and any person material NEVER enter (the «кандидат
-- не покидает строку» precedent of 2.6/2.7/2.13/2.14/2.15). The
-- promise record command joins the idempotency surface (44th word).

CREATE TABLE cases.handoff_update_promises (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 promise_id uuid NOT NULL,
 promise_no bigint NOT NULL CHECK (promise_no >= 1),
 request_no bigint NOT NULL CHECK (request_no >= 1),
 deadline_word text NOT NULL CHECK (deadline_word IN ('update_15m','update_1h','update_4h','update_unknown')),
 -- The material honesty pair of FR-41: the unknown word carries NO
 -- timestamp, a known word carries ONLY the computed server
 -- timestamp — the dishonest pairs (unknown with a time, known
 -- without one) are refused by the database, never only by the
 -- domain guard.
 promised_at timestamptz CHECK (
        (deadline_word = 'update_unknown') = (promised_at IS NULL)),
 actor_id uuid NOT NULL,
 observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, promise_id),
 CONSTRAINT handoff_update_promises_case_no UNIQUE (tenant_id, case_id, promise_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 CONSTRAINT handoff_update_promises_round_fk FOREIGN KEY (tenant_id, case_id, request_no) REFERENCES cases.human_handoffs(tenant_id, case_id, request_no),
 CONSTRAINT update_promise_times_honest CHECK (received_at >= observed_at - interval '48 hours')
);
CREATE INDEX handoff_update_promises_case ON cases.handoff_update_promises (tenant_id, case_id, created_at);

-- The promise record command joins the idempotency surface (44th word).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary','record_correction','promise_handoff_update'));

-- The ONE promise outbox event type joins the bounded vocabulary (the
-- envelope carries the identifiers and the deadline word only; the
-- client texts and any person material never enter).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence','case.human_requested','case.human_assigned','case.human_acknowledged','case.human_declined','case.human_cancelled','case.human_returned','case.summary_recorded','case.correction_recorded','case.update_promised'));

ALTER TABLE cases.handoff_update_promises ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.handoff_update_promises FORCE ROW LEVEL SECURITY;

CREATE POLICY handoff_update_promises_scope ON cases.handoff_update_promises USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only promise surface (INSERT/SELECT); the rows
-- are never edited — a new promise is a new row, the lateness and the
-- supersede are the read-time derivations of the domain. The history
-- is the truth. The record is OPERATIONAL (the deadline word and the
-- responsible actor — no client material is stored): the
-- case-operations class of the perimeter registry, never the
-- case-content class.
GRANT SELECT, INSERT ON cases.handoff_update_promises TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.handoff_update_promises FROM staffy_app;

-- +goose Down
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary','record_correction'));
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence','case.human_requested','case.human_assigned','case.human_acknowledged','case.human_declined','case.human_cancelled','case.human_returned','case.summary_recorded','case.correction_recorded'));
DROP TABLE cases.handoff_update_promises;
