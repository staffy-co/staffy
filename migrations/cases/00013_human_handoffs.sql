-- +goose Up
-- Story 2.13 (first slice): the unconditional human request and the
-- lifecycle Handoff record of the case.
--
-- A human handoff is an OPERATIONAL lifecycle record of the case, never
-- a fact and never a mode by itself: both tables live in the case
-- operations class with INSERT/SELECT grants only — no fact versions, no
-- facts digest, no identity proof (the perimeter probes assert the
-- absence of the rows in the fact channels; the 2.11/2.12 precedent).
-- The request row anchors the ACCEPTED client turn that asked for the
-- human (the turn_ref discipline of the 2.5 signals — the anchor of the
-- unconditional ask, AC-1): the store guard requires the event to belong
-- to THIS case and be accepted. The request is UNCONDITIONAL: the reason
-- word is the single dictionary constant client_request — the operator
-- never types a reason; the owner snapshot of the moment is the server
-- truth read under the locks; the target roles are the subset of the
-- fixed two-word dictionary the request addresses; the ack-deadline word
-- of the expected confirmation window belongs to the approved policy v0
-- dictionary with its honest deadline time.
--
-- The lifecycle events are append-only rows of one request: one event —
-- one kind; the numbers are the per-case (requests) and per-request
-- (events) monotonic sequences derived under the advisory locks — the
-- requests never carry them; the UNIQUE backstops catch the races the
-- locks already serialize. The material is per-kind (the CHECK below):
-- assigned names the assignee, the backup owner and the round's own
-- deadline word with its time (a re-assignment is a NEW event with a NEW
-- deadline — the human re-decision); acknowledged carries the resulting
-- ownership epoch — the ONE transition that moves the ownership; declined
-- and cancelled carry no owner material (the owner NEVER changes on the
-- assignment, the refusal or the cancellation — the prior safe owner
-- stays, AC-6/AC-10); cancelled anchors the client's cancelling turn;
-- returned states the result word of the fixed two-word dictionary. The
-- overdue state is NEVER a row — it is the read-time derivation of the
-- open assigned round after the deadline (the honest wait of AC-6; the
-- failed word does not exist in this slice).
--
-- The six commands join the idempotency surface (the 2.2..2.12
-- precedent: the stored outcome replays an exact repeat, the same key
-- with another request hash is an honest conflict). The six outbox event
-- types join the bounded vocabulary: the envelopes carry the identifiers
-- and the dictionary words only — the system reason text of the mode
-- raise and any free material never enter (the «кандидат не покидает
-- строку» precedent of 2.6).

CREATE TABLE cases.human_handoffs (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 request_no bigint NOT NULL CHECK (request_no >= 1),
 reason text NOT NULL CHECK (reason IN ('client_request')),
 owner_kind text NOT NULL CHECK (owner_kind IN ('human','staffy')),
 owner_ref uuid NOT NULL,
 owner_epoch bigint NOT NULL CHECK (owner_epoch >= 1),
 target_roles jsonb NOT NULL CHECK (
        jsonb_typeof(target_roles) = 'array'
        AND jsonb_array_length(target_roles) BETWEEN 1 AND 2
        AND target_roles <@ '["operator","supervision"]'::jsonb
        AND (jsonb_array_length(target_roles) = 1 OR target_roles ?& array['operator','supervision'])),
 ack_deadline text NOT NULL CHECK (ack_deadline IN ('ack_15m','ack_1h','ack_4h')),
 ack_deadline_at timestamptz NOT NULL,
 turn_ref uuid NOT NULL,
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id, request_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id)
);
CREATE INDEX human_handoffs_case ON cases.human_handoffs (tenant_id, case_id, created_at);

-- The append-only lifecycle events of one request (the monotonic event
-- number is the per-request server truth; the history is never
-- rewritten — the states are the read-time derivation of the domain).
CREATE TABLE cases.human_handoff_events (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 request_no bigint NOT NULL CHECK (request_no >= 1),
 event_no bigint NOT NULL CHECK (event_no >= 1),
 kind text NOT NULL CHECK (kind IN ('assigned','acknowledged','declined','cancelled','returned')),
 assignee uuid,
 backup_owner uuid,
 ack_deadline text CHECK (ack_deadline IS NULL OR ack_deadline IN ('ack_15m','ack_1h','ack_4h')),
 deadline_at timestamptz,
 result text CHECK (result IS NULL OR result IN ('issue_resolved','handed_back')),
 turn_ref uuid,
 resulting_epoch bigint CHECK (resulting_epoch IS NULL OR resulting_epoch >= 1),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id, request_no, event_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, case_id, request_no) REFERENCES cases.human_handoffs(tenant_id, case_id, request_no),
 CONSTRAINT handoff_event_material CHECK (
   (kind = 'assigned' AND assignee IS NOT NULL AND backup_owner IS NOT NULL
                           AND ack_deadline IS NOT NULL AND deadline_at IS NOT NULL
                           AND result IS NULL AND turn_ref IS NULL AND resulting_epoch IS NULL)
   OR (kind = 'acknowledged' AND resulting_epoch IS NOT NULL
                           AND assignee IS NULL AND backup_owner IS NULL
                           AND ack_deadline IS NULL AND deadline_at IS NULL
                           AND result IS NULL AND turn_ref IS NULL)
   OR (kind = 'declined' AND assignee IS NULL AND backup_owner IS NULL
                          AND ack_deadline IS NULL AND deadline_at IS NULL
                          AND result IS NULL AND turn_ref IS NULL AND resulting_epoch IS NULL)
   OR (kind = 'cancelled' AND turn_ref IS NOT NULL
                           AND assignee IS NULL AND backup_owner IS NULL
                           AND ack_deadline IS NULL AND deadline_at IS NULL
                           AND result IS NULL AND resulting_epoch IS NULL)
   OR (kind = 'returned' AND result IS NOT NULL
                          AND assignee IS NULL AND backup_owner IS NULL
                          AND ack_deadline IS NULL AND deadline_at IS NULL
                          AND turn_ref IS NULL AND resulting_epoch IS NULL))
);
CREATE INDEX human_handoff_events_case ON cases.human_handoff_events (tenant_id, case_id, request_no, event_no);

-- The six lifecycle commands join the idempotency surface.
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence','request_human','assign_human_handoff','acknowledge_human_handoff','decline_human_handoff','cancel_human_request','return_to_staffy'));

-- The six lifecycle outbox event types join the bounded vocabulary (the
-- envelopes carry the identifiers and the dictionary words only; the
-- system reason text of the mode and any free material never enter).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence','case.human_requested','case.human_assigned','case.human_acknowledged','case.human_declined','case.human_cancelled','case.human_returned'));

ALTER TABLE cases.human_handoffs ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.human_handoffs FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.human_handoff_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.human_handoff_events FORCE ROW LEVEL SECURITY;

CREATE POLICY human_handoffs_scope ON cases.human_handoffs USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY human_handoff_events_scope ON cases.human_handoff_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only operational lifecycle surfaces (INSERT/SELECT);
-- the requests and the events are never edited — a change is a new event,
-- a new request. The history is the truth.
GRANT SELECT, INSERT ON cases.human_handoffs TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.human_handoffs FROM staffy_app;
GRANT SELECT, INSERT ON cases.human_handoff_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.human_handoff_events FROM staffy_app;

-- +goose Down
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link','record_accommodation','record_route_degradation','record_delivery_evidence'));
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence'));
DROP TABLE cases.human_handoff_events;
DROP TABLE cases.human_handoffs;
