-- +goose Up
-- Story 2.6: the progressive intake and the deterministic Policy Outcome
-- (first slice). The cases schema stays append-only: ai_proposals and
-- policy_outcomes carry INSERT/SELECT grants only. The CURRENT outcome of
-- a Work Item is the read-time derivation (the maximal outcome_no) —
-- nothing is ever updated; a re-evaluation supersedes by a NEW row with a
-- bigger number (the reproducibility by the input versions). The outcome
-- number is the per-Work-Item monotonic counter computed under the case +
-- conversation advisory locks of the command. The CANDIDATE ANSWER text
-- never enters the outbox envelopes, the exceptions or the logs — it
-- lives in the proposal row and the operator panel only. The intent is a
-- bounded string claim: the DICTIONARY is a policy matter (an unknown
-- intent fails closed as an outcome — never a submission refusal). The
-- reply gate extension: the sales reply carries the Policy Outcome
-- reference of the moment (the pre-migration rows carry the honest NULL).

-- The AI proposals: one append-only row per typed claim. The proposal is
-- the proposer's material (the synthetic port of the slice or the
-- operator — the future AI Runtime passes through the same command): the
-- intent claim, the bounded candidate answer (possibly empty — no answer
-- basis), the missing-fact claims with their reason words, the
-- extracted-fact claims with their source spans (the AUDIT claims — the
-- facts are never written from them; the only fact writer is the 2-2
-- command), and the submission-moment snapshot (the owner epoch and the
-- mode version). The UNIQUE (tenant, proposal_id) is the identity; the
-- turn reference points at an accepted event of the case.
CREATE TABLE cases.ai_proposals (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 proposal_id uuid NOT NULL,
 turn_ref uuid NOT NULL,
 schema_version text NOT NULL CHECK (schema_version IN ('ai-proposal-v1')),
 intent text NOT NULL CHECK (char_length(intent) BETWEEN 1 AND 64),
 candidate_answer text NOT NULL CHECK (char_length(candidate_answer) <= 2000),
 missing_facts jsonb NOT NULL DEFAULT '[]'::jsonb,
 extracted_facts jsonb NOT NULL DEFAULT '[]'::jsonb,
 proposer text NOT NULL CHECK (proposer IN ('synthetic_port','operator')),
 owner_epoch bigint NOT NULL CHECK (owner_epoch >= 1),
 mode_version bigint NOT NULL CHECK (mode_version >= 1),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, proposal_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 FOREIGN KEY (tenant_id, turn_ref) REFERENCES cases.channel_events(tenant_id, event_id)
);
CREATE INDEX ai_proposals_case ON cases.ai_proposals (tenant_id, case_id, created_at);
CREATE INDEX ai_proposals_work_item ON cases.ai_proposals (tenant_id, work_item_id, created_at);

-- The Policy Outcomes: one append-only row per deterministic decision.
-- The material uniqueness (tenant, work_item, outcome_no) is the
-- monotonic counter backstop — two concurrent evaluations under the
-- advisory locks produce exactly one row per number. The class, the
-- reason and the rule version are the fixed dictionary words; the intent
-- snapshot is the proposal's claim word of the moment. The mandatory
-- fact plan with the states, the next steps, the reused facts with the
-- provenance and the suppressed questions are the JSONB material of the
-- decision; the input snapshot is the full anchored input (the proposal
-- identity, the schema word, the mode version, the owner epoch, the
-- confirmed fact map with its digest, the Work Item version, the rule
-- version); the decision digest is the canonical SHA-256 of the inputs +
-- the decision (the same anchors produce the same digest — the
-- reproducibility, AC-3).
CREATE TABLE cases.policy_outcomes (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 proposal_id uuid NOT NULL,
 outcome_id uuid NOT NULL,
 outcome_no bigint NOT NULL CHECK (outcome_no >= 1),
 outcome_class text NOT NULL CHECK (outcome_class IN ('commercial_answer','informational_answer','clarify','mode_blocked','fail_closed')),
 reason text NOT NULL CHECK (reason IN ('ok_commercial','ok_informational','mandatory_missing','no_answer_basis','mode_blocks_commercial_path','unknown_intent')),
 rule_version text NOT NULL CHECK (rule_version IN ('intake-policy-v1')),
 intent text NOT NULL CHECK (char_length(intent) BETWEEN 1 AND 64),
 mandatory_facts jsonb NOT NULL DEFAULT '[]'::jsonb,
 next_steps jsonb NOT NULL DEFAULT '[]'::jsonb,
 reused_facts jsonb NOT NULL DEFAULT '[]'::jsonb,
 suppressed_questions jsonb NOT NULL DEFAULT '[]'::jsonb,
 input_snapshot jsonb NOT NULL,
 decision_digest text NOT NULL CHECK (char_length(decision_digest) = 64),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, outcome_id),
 UNIQUE (tenant_id, work_item_id, outcome_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 FOREIGN KEY (tenant_id, proposal_id) REFERENCES cases.ai_proposals(tenant_id, proposal_id)
);
CREATE INDEX policy_outcomes_work_item ON cases.policy_outcomes (tenant_id, work_item_id, outcome_no);
CREATE INDEX policy_outcomes_case ON cases.policy_outcomes (tenant_id, case_id, created_at);
CREATE INDEX policy_outcomes_proposal ON cases.policy_outcomes (tenant_id, proposal_id);

-- The Policy Outcome reference of the client replies: the sales reply
-- gate resolves the CURRENT outcome by the Work Item anchor (the request
-- never carries the outcome id — the server-side truth); the
-- pre-migration rows carry the honest NULL. The FK-style reference stays
-- unconstrained (the frozen spec word: the column records what the gate
-- read; the integrity is the gate's own re-validation under the locks).
ALTER TABLE cases.client_replies ADD COLUMN policy_outcome_id uuid;

-- The two new commands join the idempotency surface (the 2-2..2-5
-- precedent: the stored outcome replays an exact repeat, the same key
-- with another request hash is an honest conflict — the request hash of
-- evaluate_policy includes the four anchors, the m-r2-1 lesson).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy'));

-- The two policy outbox event types join the event dictionary of the
-- outbox (the bounded type vocabulary — the envelopes carry the
-- identifiers, the state words and the version numbers only; the
-- candidate answer text never enters).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated'));

ALTER TABLE cases.ai_proposals ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.ai_proposals FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.policy_outcomes ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.policy_outcomes FORCE ROW LEVEL SECURITY;

CREATE POLICY ai_proposals_scope ON cases.ai_proposals USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY policy_outcomes_scope ON cases.policy_outcomes USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the proposals and the
-- outcomes are never edited.
GRANT SELECT, INSERT ON cases.ai_proposals TO staffy_app;
GRANT SELECT, INSERT ON cases.policy_outcomes TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.ai_proposals FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.policy_outcomes FROM staffy_app;

-- +goose Down
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS policy_outcome_id;
DROP TABLE cases.policy_outcomes;
DROP TABLE cases.ai_proposals;
