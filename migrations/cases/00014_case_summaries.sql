-- +goose Up
-- Story 2.14 (first slice): the append-only AI summary of the case.
--
-- A summary is an OPERATIONAL record of the case lifecycle, never a
-- fact and never a mode: the table lives in the case operations class
-- with INSERT/SELECT grants only — no fact versions, no facts digest,
-- no identity proof (the perimeter probes assert the absence of the
-- rows in the fact channels; the 2.11..2.13 precedent). The summary
-- never replaces the sources: the events, the commitments and the
-- deliveries render in the same operator section (AD-8) — the row is
-- the DERIVED representation with the honest marking, not the truth.
--
-- The kind dictionary is the SINGLE word of the slice — the producer's
-- summary; the operator summaries are Never (the honest boundary: the
-- notes of 2.3 stay the operator's own surface). The text is 1..1000
-- runes; the claims are 1..32 with the byte-exact span discipline of
-- the domain guard (an assertion is a line of the text — the
-- anti-hallucination probe; the machine checks the structure, the
-- truth is the producer's and the operator's responsibility). The
-- claims jsonb carries the MATERIAL check: every element is an object
-- that either links (the ref-type word of the fixed two-word
-- dictionary WITH a non-empty ref id) or is honestly disputed — the
-- database backstop of the «every assertion is sourced or disputed»
-- rule; the ref GUARD of the store additionally requires the refs to
-- belong to THIS case (the fact anchors and the observation rows).
--
-- The watermark is the SERVER truth snapshot of the moment the summary
-- was recorded: the accepted-event count, the fact-version count, the
-- owner epoch and the mode version — the request NEVER carries them
-- (the store takes them under the locks; the probe pins the absence).
-- The staleness is the read-time derivation of the watermark against
-- the CURRENT counters — never a stored status; the summary row never
-- mutates (the correction and the dispute are Story 2.15 — the
-- append-only versioning by number: a new summary is a NEW row).
--
-- The summary number is the per-case monotonic sequence derived under
-- the advisory locks — the requests never carry it; the UNIQUE
-- backstop catches the races the locks already serialize. The record
-- command joins the idempotency surface (the 2.2..2.13 precedent: the
-- stored outcome replays an exact repeat, the same key with another
-- request hash is an honest conflict). The ONE outbox event type joins
-- the bounded vocabulary: the envelope carries the identifiers, the
-- kind word and the claim counters ONLY — the summary TEXT and the
-- claims NEVER enter (the «кандидат не покидает строку» precedent of
-- 2.6/2.7/2.13).

CREATE TABLE cases.case_summaries (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 summary_no bigint NOT NULL CHECK (summary_no >= 1),
 kind text NOT NULL CHECK (kind IN ('ai_summary')),
 actor_id uuid NOT NULL,
 text text NOT NULL CHECK (char_length(text) BETWEEN 1 AND 1000),
 claims jsonb NOT NULL CHECK (
        jsonb_typeof(claims) = 'array'
        AND jsonb_array_length(claims) BETWEEN 1 AND 32
        AND octet_length(claims::text) <= 16384
        AND NOT jsonb_path_exists(claims, '$[*] ? (@.type() != "object")')
        AND NOT jsonb_path_exists(claims, '$[*] ? (!(( (@.ref_type == "fact_version" || @.ref_type == "observation") && exists(@.ref_id) && @.ref_id != "" ) || @.disputed == true))'
 )),
 event_count bigint NOT NULL CHECK (event_count >= 0),
 fact_version_count bigint NOT NULL CHECK (fact_version_count >= 0),
 owner_epoch bigint NOT NULL CHECK (owner_epoch >= 0),
 mode_version bigint NOT NULL CHECK (mode_version >= 0),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id, summary_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id)
);
CREATE INDEX case_summaries_case ON cases.case_summaries (tenant_id, case_id, created_at);

-- The summary record command joins the idempotency surface.
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary'));

-- The ONE summary outbox event type joins the bounded vocabulary (the
-- envelope carries the identifiers, the kind word and the claim
-- counters only; the summary text and the claims NEVER enter).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence','case.human_requested','case.human_assigned','case.human_acknowledged','case.human_declined','case.human_cancelled','case.human_returned','case.summary_recorded'));

ALTER TABLE cases.case_summaries ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.case_summaries FORCE ROW LEVEL SECURITY;

CREATE POLICY case_summaries_scope ON cases.case_summaries USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only operational summary surface (INSERT/SELECT);
-- the rows are never edited — a correction is Story 2.15, a new summary
-- is a new row. The history is the truth.
GRANT SELECT, INSERT ON cases.case_summaries TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.case_summaries FROM staffy_app;

-- +goose Down
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy'));
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence','case.human_requested','case.human_assigned','case.human_acknowledged','case.human_declined','case.human_cancelled','case.human_returned'));
DROP TABLE cases.case_summaries;
