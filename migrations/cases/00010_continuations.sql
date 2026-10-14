-- +goose Up
-- Story 2.10: the protected continuation (first slice: the purpose-bound
-- Continuation Link and the protected continuation page, the text
-- modality). The cases schema stays append-only: continuation_links,
-- continuation_link_uses and continuation_link_revocations carry
-- INSERT/SELECT grants only. The link state is NEVER a stored status —
-- the read-time DeriveContinuationLinkState derivation of the domain
-- reads the CURRENT material (the expiry, the revocation row, the
-- accepted corrections count, the current owner epoch) — a dead link
-- never resurrects (the DeriveConfirmationValidity precedent of 2.9).
--
-- The capability token is OPAQUE: base64url(tenant 16B || random 32B).
-- The database stores ONLY the SHA-256 of the random half (bytea 32,
-- UNIQUE per tenant) — the preimage is not recoverable, the full token
-- never enters the storage, the outbox, the logs or the exceptions (the
-- short-lived one-use playback-permit precedent of the architecture
-- spine: a request-carried credential; the AD-10 honesty: the scope
-- candidate from the token is verified by the hash BEFORE any case
-- material is read — a wrong tenant finds no hash row under the RLS
-- scope, the generic refusal without disclosure).
--
-- The dictionary words (the purposes v1, the use kinds, the bounded
-- ttl/limit) are the CHECK backstops of the fixed Domain Pack v0
-- dictionaries — never a tenant configuration. The consumption counts
-- the ACCEPTED fact corrections only (the human request and the return
-- are events, not data submissions — they never consume anything).
-- The presentation binding of the outgoing sales reply is the single
-- nullable client_replies.link_id column (the policy_outcome_id 2.6 /
-- objection_id 2.8 precedent: the column records what the gate read;
-- the integrity is the gate's own re-validation under the locks).

-- The links: one append-only row per issued link. The relation pins:
-- the case, the work item, the participant reference (the channel
-- address of the client), the response owner epoch of the moment (the
-- honest boundary of the slice: the full Tenant control generation
-- pinning arrives with the Commitment branch), the purpose set (the
-- non-empty subset of the v1 dictionary), the open question the client
-- answers on the surface and the explanation of the transition. The
-- expiry is the issuance plus the ttl minutes (the CHECK keeps the
-- arithmetic honest); the token hash is the only stored trace of the
-- capability.
CREATE TABLE cases.continuation_links (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 link_id uuid NOT NULL,
 participant_ref text NOT NULL CHECK (char_length(participant_ref) BETWEEN 1 AND 128),
 owner_epoch bigint NOT NULL CHECK (owner_epoch >= 1),
 purposes jsonb NOT NULL CHECK (jsonb_typeof(purposes) = 'array' AND jsonb_array_length(purposes) BETWEEN 1 AND 4 AND purposes <@ '["fact_correction"]'::jsonb),
 question text NOT NULL CHECK (char_length(question) BETWEEN 1 AND 500),
 explanation text NOT NULL CHECK (char_length(explanation) BETWEEN 1 AND 1000),
 ttl_minutes int NOT NULL CHECK (ttl_minutes BETWEEN 5 AND 120),
 issued_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL,
 consumption_limit int NOT NULL CHECK (consumption_limit BETWEEN 1 AND 10),
 token_hash bytea NOT NULL CHECK (octet_length(token_hash) = 32),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, link_id),
 UNIQUE (tenant_id, token_hash),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 CONSTRAINT continuation_link_window CHECK (expires_at = issued_at + make_interval(mins => ttl_minutes))
);
CREATE INDEX continuation_links_case ON cases.continuation_links (tenant_id, case_id, created_at);
CREATE INDEX continuation_links_wi ON cases.continuation_links (tenant_id, work_item_id);

-- The uses: one append-only row per client action on the surface. The
-- accepted fact correction consumes the link limit (the material pair —
-- the corrected fact with its new version number — lands with the row:
-- both or neither); the human request and the return are events, they
-- carry no material and consume nothing. Every row carries its own
-- submitted_at (the FR-16 «the link ties exactly one submission to the
-- request» accounting: the limit counts the accepted corrections only).
CREATE TABLE cases.continuation_link_uses (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 link_id uuid NOT NULL,
 use_id uuid NOT NULL,
 use_kind text NOT NULL CHECK (use_kind IN ('fact_correction_submitted','human_requested','returned_to_conversation')),
 fact_id uuid,
 fact_version_no bigint,
 submitted_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, use_id),
 FOREIGN KEY (tenant_id, link_id) REFERENCES cases.continuation_links(tenant_id, link_id),
 CONSTRAINT continuation_use_material CHECK ((fact_id IS NULL AND fact_version_no IS NULL) OR (fact_id IS NOT NULL AND fact_version_no IS NOT NULL))
);
CREATE INDEX continuation_link_uses_link ON cases.continuation_link_uses (tenant_id, link_id, submitted_at);
CREATE INDEX continuation_link_uses_case ON cases.continuation_link_uses (tenant_id, case_id, submitted_at);

-- The revocations: one append-only row per link (the UNIQUE is the
-- fail-closed backstop — the second revoke of the same link is the
-- honest refusal of the app; the revocation mutates NOTHING: the
-- read-time derivation reads the row and blocks the surface without
-- disclosure).
CREATE TABLE cases.continuation_link_revocations (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 link_id uuid NOT NULL,
 revocation_id uuid NOT NULL,
 reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 500),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, revocation_id),
 UNIQUE (tenant_id, link_id),
 FOREIGN KEY (tenant_id, link_id) REFERENCES cases.continuation_links(tenant_id, link_id)
);
CREATE INDEX continuation_link_revocations_case ON cases.continuation_link_revocations (tenant_id, case_id, created_at);

-- The presentation binding of the outgoing sales reply: the reply that
-- weaves the link URL into its text carries the nullable link reference
-- (the delivery binding — the FR-16 «the main channel shows whether the
-- link was delivered»; the non-sales rows and the pre-migration rows
-- carry the honest NULL; the FK-style reference stays unconstrained —
-- the 2.6 precedent: the gate re-validates the existence under the
-- locks).
ALTER TABLE cases.client_replies ADD COLUMN link_id uuid;

-- The two commands join the idempotency surface (the 2.2..2.9
-- precedent: the stored outcome replays an exact repeat, the same key
-- with another request hash is an honest conflict).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline','issue_continuation_link','revoke_continuation_link'));

-- The four outbox event types join the bounded vocabulary (the envelope
-- carries the identifiers, the purpose words and the version numbers
-- only; the token, the question/explanation texts, the fact values and
-- the participant reference never enter).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned'));

-- The protected-continuation word joins the channel dictionary of the
-- fact provenance (the +1 word of the slice: the provenance channel of
-- the client-submitted fact corrections — the FR-82 modality/source
-- trace of the protected continuation, the text modality; every
-- pre-existing word stays legal — the control branch of the domain
-- tests).
ALTER TABLE cases.fact_versions DROP CONSTRAINT fact_versions_channel_check;
ALTER TABLE cases.fact_versions ADD CONSTRAINT fact_versions_channel_check CHECK (channel IN ('phone','web_chat','telegram','whatsapp','email','max','jivo','avito','mobile_app_chat','operator','protected_continuation'));

ALTER TABLE cases.continuation_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.continuation_links FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.continuation_link_uses ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.continuation_link_uses FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.continuation_link_revocations ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.continuation_link_revocations FORCE ROW LEVEL SECURITY;

CREATE POLICY continuation_links_scope ON cases.continuation_links USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY continuation_link_uses_scope ON cases.continuation_link_uses USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY continuation_link_revocations_scope ON cases.continuation_link_revocations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the links, the uses
-- and the revocations are never edited.
GRANT SELECT, INSERT ON cases.continuation_links TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.continuation_links FROM staffy_app;
GRANT SELECT, INSERT ON cases.continuation_link_uses TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.continuation_link_uses FROM staffy_app;
GRANT SELECT, INSERT ON cases.continuation_link_revocations TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.continuation_link_revocations FROM staffy_app;

-- +goose Down
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS link_id;
DROP TABLE cases.continuation_link_revocations;
DROP TABLE cases.continuation_link_uses;
DROP TABLE cases.continuation_links;
