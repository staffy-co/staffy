-- +goose Up
-- Story 2.9: the Versioned Commitment, the material snapshot and the
-- exact-version Confirmation (first slice). The cases schema stays
-- append-only: commitments, confirmations, commitment_declines and
-- commitment_hmac_keys carry INSERT/SELECT grants only. A changed offer
-- is a NEW version row (version_no is the per-commitment monotonic
-- counter computed under the case + work item advisory locks — the
-- policy_outcomes 2-6 / objections 2-8 precedent); the old versions
-- never mutate (AC-8). The confirmation binds to the EXACT shown
-- version: the presenting sales reply carries the nullable pair
-- (commitment_id, commitment_version — both or neither), and the
-- confirm command demands the verbatim span excerpts of the client
-- words (the byte-exact containment is the domain+app honesty — the
-- 2.8 span precedent). The MATERIAL CONDITIONS never enter the outbox
-- envelopes, the exceptions or the logs (the «кандидат не покидает
-- строку» precedent of 2.6/2.7/2.8). The dictionary words (the
-- statuses, the action words, the method, the decline outcomes, the
-- unknown term words) are the CHECK backstops of the fixed Domain Pack
-- v0 dictionaries — never a tenant configuration. The content binding
-- is the keyed HMAC-SHA256 token over the canonical material bytes:
-- the per-tenant commitment key v1 (32 bytes, INSERT-only,
-- ensure-pattern — the identifier_hmac_keys 2-3 precedent; the
-- rotation/destruction honestly deferred). The reserved validity words
-- (revoked, consumed) have NO producer column here — the validity is
-- the read-time derivation, never a stored status.

-- The commitments: one append-only row per version of one offer. The
-- stable commitment_id groups the versions; version_no is monotonic
-- (UNIQUE per commitment — the counter backstop of the advisory
-- locks). The conditions jsonb carries the canonical MaterialTerms v1
-- (the domain renders the deterministic bytes; the CHECK bounds the
-- size — the serialization honesty is the domain's own). The relation
-- pins: the work item, the owner epoch of the moment, the release
-- lineage word (employee/decision_no/profile_version — the 2.4
-- reply-snapshot precedent), the current policy outcome number (2-6)
-- and the pinned price observation number (0 = honestly none — the
-- unknown-price hole). The material_unknowns jsonb carries only the
-- mandatory term words (the jsonb subset backstop).
CREATE TABLE cases.commitments (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 commitment_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no >= 1),
 status text NOT NULL CHECK (status IN ('draft','offered')),
 conditions jsonb NOT NULL CHECK (jsonb_typeof(conditions) = 'object' AND octet_length(conditions::text) <= 4096),
 action_set jsonb NOT NULL CHECK (jsonb_typeof(action_set) = 'array' AND jsonb_array_length(action_set) BETWEEN 1 AND 4 AND action_set <@ '["prepare_order","schedule_appointment","issue_document"]'::jsonb),
 valid_until timestamptz NOT NULL,
 consumption_limit int NOT NULL CHECK (consumption_limit BETWEEN 1 AND 10),
 material_unknowns jsonb NOT NULL CHECK (jsonb_typeof(material_unknowns) = 'array' AND jsonb_array_length(material_unknowns) <= 7 AND material_unknowns <@ '["scope","service_items","price","payee","warranty_role","next_step","consequences"]'::jsonb),
 owner_kind text NOT NULL CHECK (owner_kind IN ('human','staffy')),
 owner_ref uuid NOT NULL,
 owner_epoch bigint NOT NULL CHECK (owner_epoch >= 1),
 lineage_word text NOT NULL CHECK (char_length(lineage_word) BETWEEN 1 AND 128),
 policy_outcome_no bigint NOT NULL CHECK (policy_outcome_no >= 0),
 pinned_price_observation_no bigint NOT NULL CHECK (pinned_price_observation_no >= 0),
 content_token char(64) NOT NULL CHECK (char_length(content_token) = 64),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, commitment_id, version_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id)
);
CREATE INDEX commitments_wi ON cases.commitments (tenant_id, work_item_id, commitment_id, version_no);
CREATE INDEX commitments_case ON cases.commitments (tenant_id, case_id, created_at);

-- The confirmations: one append-only row per confirmed subject of one
-- EXACT version. The material uniqueness (tenant, commitment, version,
-- subject) is the fail-closed duplicate backstop — the second subject
-- is a separate row, the repeat of the same subject is the honest 409
-- of the app (the UNIQUE violation maps to the duplicate refusal). The
-- evidence spans carry the array-length bound only (the byte-exact
-- containment is the domain+app honesty — the 2.8 precedent). The
-- material snapshot and the action set snapshot live in the row (the
-- derivation of the validity reads the CURRENT material — a replay of
-- an old event never resurrects a dead confirmation). The method
-- dictionary v1 is explicit_text (the voice contour honestly refuses
-- at the app gate — the modality word of the turn source).
CREATE TABLE cases.confirmations (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 commitment_id uuid NOT NULL,
 version_no bigint NOT NULL,
 confirmation_id uuid NOT NULL,
 subject_kind text NOT NULL CHECK (subject_kind IN ('client')),
 subject_ref text NOT NULL CHECK (char_length(subject_ref) BETWEEN 1 AND 128),
 method text NOT NULL CHECK (method IN ('explicit_text')),
 channel text NOT NULL CHECK (char_length(channel) BETWEEN 1 AND 32),
 expires_at timestamptz NOT NULL,
 consumed_count int NOT NULL CHECK (consumed_count = 0),
 consumption_limit int NOT NULL CHECK (consumption_limit BETWEEN 1 AND 10),
 turn_ref uuid NOT NULL,
 evidence_spans jsonb NOT NULL CHECK (jsonb_typeof(evidence_spans) = 'array' AND jsonb_array_length(evidence_spans) BETWEEN 1 AND 8),
 material_snapshot jsonb NOT NULL CHECK (jsonb_typeof(material_snapshot) = 'object' AND octet_length(material_snapshot::text) <= 4096),
 action_set_snapshot jsonb NOT NULL CHECK (jsonb_typeof(action_set_snapshot) = 'array' AND jsonb_array_length(action_set_snapshot) BETWEEN 1 AND 4),
 content_token char(64) NOT NULL CHECK (char_length(content_token) = 64),
 actor_id uuid NOT NULL,
 confirmed_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, confirmation_id),
 UNIQUE (tenant_id, commitment_id, version_no, subject_kind, subject_ref),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 FOREIGN KEY (tenant_id, commitment_id, version_no) REFERENCES cases.commitments(tenant_id, commitment_id, version_no)
);
CREATE INDEX confirmations_commitment ON cases.confirmations (tenant_id, commitment_id, version_no);
CREATE INDEX confirmations_case ON cases.confirmations (tenant_id, case_id, confirmed_at);

-- The declines: one append-only row per recorded refusal or change
-- request. The decline mutates NOTHING (AC-11): the honest outcome
-- word and the verbatim spans only; the operator creates the new
-- version (version_no+1) as the honest answer to change_requested.
CREATE TABLE cases.commitment_declines (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 commitment_id uuid NOT NULL,
 version_no bigint NOT NULL,
 decline_id uuid NOT NULL,
 outcome text NOT NULL CHECK (outcome IN ('declined','change_requested')),
 turn_ref uuid NOT NULL,
 evidence_spans jsonb NOT NULL CHECK (jsonb_typeof(evidence_spans) = 'array' AND jsonb_array_length(evidence_spans) BETWEEN 1 AND 8),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, decline_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 FOREIGN KEY (tenant_id, commitment_id, version_no) REFERENCES cases.commitments(tenant_id, commitment_id, version_no)
);
CREATE INDEX commitment_declines_case ON cases.commitment_declines (tenant_id, case_id, created_at);

-- The per-tenant commitment keys of the content binding: 32 bytes,
-- INSERT-only (the ensure pattern of identifier_hmac_keys 2-3 — SELECT
-- → INSERT ON CONFLICT DO NOTHING → SELECT; the rotation and the
-- destruction honestly deferred to the retention policy).
CREATE TABLE cases.commitment_hmac_keys (
 tenant_id uuid NOT NULL,
 key bytea NOT NULL CHECK (octet_length(key) = 32),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id)
);

-- The presentation link of the client replies: the sales reply carries
-- the commercial material of the EXACT shown version — the nullable
-- pair (both or neither; the non-sales rows and the pre-migration rows
-- carry the honest NULLs — the policy_outcome_id 2-6 precedent). The
-- FK-style reference stays unconstrained (the 2-6 precedent: the column
-- records what the gate read; the integrity is the gate's own
-- re-validation under the locks).
ALTER TABLE cases.client_replies ADD COLUMN commitment_id uuid;
ALTER TABLE cases.client_replies ADD COLUMN commitment_version bigint CHECK (commitment_version IS NULL OR commitment_version >= 1);
ALTER TABLE cases.client_replies ADD CONSTRAINT client_replies_commitment_pair CHECK ((commitment_id IS NULL AND commitment_version IS NULL) OR (commitment_id IS NOT NULL AND commitment_version IS NOT NULL));

-- The confirmed word joins the state ladder of the work item versions
-- (the additive 2.2 extension: in_work/waiting → confirmed,
-- confirmed → resolved/cancelled; every pre-existing word and
-- transition stays byte-identical — the control branch of the domain
-- tests).
ALTER TABLE cases.work_item_versions DROP CONSTRAINT work_item_versions_state_check;
ALTER TABLE cases.work_item_versions ADD CONSTRAINT work_item_versions_state_check CHECK (state IN ('untriaged','in_work','waiting','confirmed','resolved','cancelled','split','merged'));

-- The three commands join the idempotency surface (the 2-2..2-8
-- precedent: the stored outcome replays an exact repeat, the same key
-- with another request hash is an honest conflict).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event','submit_ai_proposal','evaluate_policy','read_owning_system','record_objection','create_commitment','confirm_commitment','record_commitment_decline'));

-- The three outbox event types join the bounded vocabulary (the
-- envelope carries the identifiers, the version numbers and the
-- dictionary words only; the material conditions and the span excerpts
-- never enter).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined'));

ALTER TABLE cases.commitments ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.commitments FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.confirmations ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.confirmations FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.commitment_declines ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.commitment_declines FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.commitment_hmac_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.commitment_hmac_keys FORCE ROW LEVEL SECURITY;

CREATE POLICY commitments_scope ON cases.commitments USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY confirmations_scope ON cases.confirmations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY commitment_declines_scope ON cases.commitment_declines USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY commitment_hmac_keys_scope ON cases.commitment_hmac_keys USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the commitments,
-- the confirmations and the declines are never edited.
GRANT SELECT, INSERT ON cases.commitments TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.commitments FROM staffy_app;
GRANT SELECT, INSERT ON cases.confirmations TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.confirmations FROM staffy_app;
GRANT SELECT, INSERT ON cases.commitment_declines TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.commitment_declines FROM staffy_app;
GRANT SELECT, INSERT ON cases.commitment_hmac_keys TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.commitment_hmac_keys FROM staffy_app;

-- +goose Down
ALTER TABLE cases.client_replies DROP CONSTRAINT IF EXISTS client_replies_commitment_pair;
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS commitment_id;
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS commitment_version;
DROP TABLE cases.confirmations;
DROP TABLE cases.commitment_declines;
DROP TABLE cases.commitments;
DROP TABLE cases.commitment_hmac_keys;
