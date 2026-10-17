-- +goose Up
-- Story 2.15 (first slice): the append-only fact correction.
--
-- A correction is one append-only OPERATOR act bound to the EXACT fact
-- version: the pinned target (the fact id with the version number being
-- corrected — the optimistic concurrency of the slice: two corrections
-- of the same version are two rows and two fact versions, never a
-- silent rewrite), the proposed value under the fact material
-- discipline (the domain canonicalizes, bounds to 2048 canonical bytes
-- and refuses the exponential number literals — the 3072 text-byte
-- backstop of the fact values), the verbatim evidence spans (the
-- byte-exact spans of the accepted event or the summary text — the
-- anti-hallucination probe of the 2.8 discipline; the array is 0..8,
-- the domain bounds every span to 1..200 runes) and the mandatory
-- reason (1..200 runes — the conscious act).
--
-- The outcome word is the SERVER decision of the matrix gate: a matrix
-- material field (price, availability, order_status — the State/
-- Authority Matrix v1 of 2.7) NEVER pours a fact version — the value of
-- the Owning System is never rewritten, the correction survives as the
-- honest dispute row; every other fact key pours the new version
-- THROUGH THE EXISTING record_fact discipline (the source is the
-- operator, the source id is correction:<correction_id> — the material
-- idempotency of the fact ladder, the pour is NEVER an automatic
-- confirmation: the resolution is the existing confirm_fact_version
-- command of a human). The request NEVER carries the outcome (the
-- strict decode of the transport refuses the field).
--
-- The summary link is nullable: NULL = the event evidence, >=1 = the
-- EXACT summary version of THIS case the correction corrects (the
-- store guards the belonging; the summary row never mutates — the
-- applied pour grows the fact-version watermark and the staleness of
-- the summary derives read-time, the 2.14 mechanism).
--
-- The correction number is the per-case monotonic sequence derived
-- under the advisory locks — the requests never carry it; the UNIQUE
-- backstop catches the races the locks already serialize. The record
-- command joins the idempotency surface (the 2.2..2.14 precedent: the
-- stored outcome replays an exact repeat, the same key with another
-- request hash is an honest conflict). The ONE outbox event type joins
-- the bounded vocabulary: the envelope carries the identifiers, the
-- outcome word and the span counter ONLY — the proposed value and the
-- spans NEVER enter (the «кандидат не покидает строку» precedent of
-- 2.6/2.7/2.13/2.14).

CREATE TABLE cases.case_corrections (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 correction_id uuid NOT NULL,
 correction_no bigint NOT NULL CHECK (correction_no >= 1),
 fact_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no >= 1),
 -- NULL = the event evidence; >=1 = the summary of THIS case.
 summary_no bigint CHECK (summary_no IS NULL OR summary_no >= 1),
 -- The domain owns the value limit: 2048 CANONICAL bytes with the
 -- digit-form numbers (the fact value discipline); the jsonb text
 -- rendering adds one space after every ':' and ',' — the 3072 text-
 -- byte backstop of the fact values (a domain-valid value always fits,
 -- a domain-invalid one never reaches the table).
 proposed_value jsonb NOT NULL CHECK (octet_length(proposed_value::text) BETWEEN 1 AND 3072),
 -- The verbatim evidence: 0..8 spans, every one a non-empty string (the
 -- domain bounds each to 1..200 runes and probes the byte-exact presence
 -- in the cited source under the locks).
 spans jsonb NOT NULL CHECK (
        jsonb_typeof(spans) = 'array'
        AND jsonb_array_length(spans) BETWEEN 0 AND 8
        AND octet_length(spans::text) <= 4096
        AND NOT jsonb_path_exists(spans, '$[*] ? (@.type() != "string")')
        AND NOT jsonb_path_exists(spans, '$[*] ? (@ == "")')
 ),
 outcome text NOT NULL CHECK (outcome IN ('applied','disputed')),
 actor_id uuid NOT NULL,
 channel text NOT NULL CHECK (channel IN ('phone','web_chat','telegram','whatsapp','email','max','jivo','avito','mobile_app_chat','operator')),
 reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 200),
 observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, correction_id),
 CONSTRAINT case_corrections_case_no UNIQUE (tenant_id, case_id, correction_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, fact_id, version_no) REFERENCES cases.fact_versions(tenant_id, fact_id, version_no),
 CONSTRAINT case_corrections_summary_fk FOREIGN KEY (tenant_id, case_id, summary_no) REFERENCES cases.case_summaries(tenant_id, case_id, summary_no),
 CONSTRAINT correction_times_honest CHECK (received_at >= observed_at - interval '48 hours')
);
CREATE INDEX case_corrections_case ON cases.case_corrections (tenant_id, case_id, created_at);

-- The correction record command joins the idempotency surface (43rd word).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary','record_correction'));

-- The ONE correction outbox event type joins the bounded vocabulary (the
-- envelope carries the identifiers, the outcome word and the span counter
-- only; the proposed value and the spans NEVER enter).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence','case.human_requested','case.human_assigned','case.human_acknowledged','case.human_declined','case.human_cancelled','case.human_returned','case.summary_recorded','case.correction_recorded'));

ALTER TABLE cases.case_corrections ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.case_corrections FORCE ROW LEVEL SECURITY;

CREATE POLICY case_corrections_scope ON cases.case_corrections USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only correction surface (INSERT/SELECT); the rows
-- are never edited — a second correction is a new row, the resolution is
-- the separate human confirmation of the fact ladder. The history is
-- the truth.
GRANT SELECT, INSERT ON cases.case_corrections TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.case_corrections FROM staffy_app;

-- +goose Down
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy','record_case_summary'));
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence','case.human_requested','case.human_assigned','case.human_acknowledged','case.human_declined','case.human_cancelled','case.human_returned','case.summary_recorded'));
DROP TABLE cases.case_corrections;
