-- +goose Up
-- Story 3.4 (first slice): the action preview preparation contour. This is
-- the FIRST migration of the action module — the schema is born here. The
-- three tables are tenant-scoped, RLS ENABLE+FORCE, and append-only BY
-- GRANTS (SELECT/INSERT only — the 00017/00009 precedent; the versions and
-- the events are never edited or deleted through the application role).
--
-- Cross-module FKs are FORBIDDEN (AD-2): case_id and work_item_id are
-- plain uuid pins — the app layer validates them by a LIVE read of the
-- case api BEFORE the insert transaction (a dead reference is the honest
-- 404 before any material, never a FK refusal). The action tables carry
-- no reference into cases.*, and cases.* carries no reference here.
--
-- The AD-7 repair bound is STRUCTURAL here, not a discipline: attempt_no
-- is CHECK {1, 2} and the (tenant, proposal_key, attempt_no) PK makes the
-- raced twin the honest ladder conflict. A third attempt does not exist
-- at the database level. proposal_key is the SHA-256 derived identity of
-- the pin tuple (DeriveProposalKey — WITHOUT the typed input: a repair is
-- the same key at attempt 2, the bound does not leak). preview_id is the
-- stable identity of the whole preparation thread: minted with the first
-- attempt row, reused by every attempt of the key and by every version
-- the thread builds.
--
-- The versions (action_previews) are the append-only ladder of 00009/2.9:
-- a stable preview_id with the monotonic version_no re-derived under the
-- action advisory lock; a changed submission is a NEW version row, the
-- old rows never mutate. The supersede is INSERT-only material: the NEW
-- row carries supersedes_version (the backward pointer of the edit), the
-- event kind 'superseded' marks the OLD version (the Timeline derivation
-- reads the events — DerivePreviewState — the stored row stays bytes).
-- The digest is the keyed HMAC over the canonical bytes with the
-- action_preview:v1: prefix (the key: the per-tenant commitment key v1 of
-- cases.commitment_hmac_keys — the 2.9/3.3 reuse, INSERT-only ensure
-- pattern; the token spaces never collide). The CHECK dictionaries are
-- the loyal backstops of the frozen domain words — never a tenant
-- configuration.
--
-- The events (action_preview_events) are the append-only history of the
-- preparation contour: the six-word kind CHECK, the monotonic event_no
-- per version (the UNIQUE backstop of the raced appends), version_no = 0
-- for the proposal-stage rows (proposed/repaired of the attempts that did
-- not build a version yet — the stage events of the thread; the Timeline
-- derivation of a version never reads them). dry_run_recorded and
-- fresh_read_recorded NEVER move the Timeline words (AC-8 — the kinds are
-- simply not in DerivePreviewState). The reason carries the policy reason
-- word of a blocked proposal; the details are the safe text only
-- (identifiers and words — never a material value, never a secret).

CREATE SCHEMA IF NOT EXISTS action;

-- The attempt ladder: one append-only row per submitted attempt of one
-- proposal key. verdict is the three-word AD-7 ladder; blocked_reason is
-- the policy reason word of the preparation refusal (the 10-word
-- dictionary; '' when the attempt was not policy-blocked); the invalid
-- report is required exactly for the invalid/rejected verdicts (the
-- honest word list the author reads back, bounded by the domain's
-- spoken truncation — the CHECK is the loyal backstop).
CREATE TABLE action.action_proposals (
 tenant_id uuid NOT NULL,
 proposal_key text NOT NULL CHECK (char_length(proposal_key) = 64),
 attempt_no int NOT NULL CHECK (attempt_no IN (1, 2)),
 preview_id uuid NOT NULL,
 definition_id text NOT NULL CHECK (char_length(definition_id) BETWEEN 1 AND 200),
 definition_version text NOT NULL CHECK (char_length(definition_version) BETWEEN 1 AND 32),
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 target_kind text NOT NULL CHECK (target_kind IN ('catalog_item', 'client', 'order')),
 target_ref uuid NOT NULL,
 verdict text NOT NULL CHECK (verdict IN ('valid', 'invalid', 'rejected')),
 blocked_reason text NOT NULL DEFAULT '' CHECK (blocked_reason = '' OR blocked_reason IN (
   'role_not_allowed', 'conversation_mode_not_allowed', 'response_owner_changed', 'control_mode',
   'entitlement_blocked', 'stage_not_allowed', 'gate_not_satisfied', 'capability_not_verified',
   'capability_class_unsafe', 'definition_not_resolved')),
 invalid_report text NOT NULL DEFAULT '' CHECK (char_length(invalid_report) <= 2100),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, proposal_key, attempt_no),
 CONSTRAINT action_proposals_report_shape CHECK (
   (verdict <> 'valid' AND char_length(invalid_report) >= 1) OR
   (verdict = 'valid' AND invalid_report = ''))
);
CREATE INDEX action_proposals_case ON action.action_proposals (tenant_id, case_id, created_at);
CREATE INDEX action_proposals_thread ON action.action_proposals (tenant_id, preview_id, attempt_no);

-- The version ladder: one append-only row per preview version of one
-- preparation thread. The jsonb columns carry the canonical domain
-- material (the projected typed input — schema fields only, no secrets;
-- the version pins of the supporting refs; the material snapshot rows;
-- the finite effect list derived ONLY from the definition). The
-- partition shape CHECK keeps the human-only marking honest: a human_only
-- partition always names the mandatory human owner, and a staffy
-- partition never does; a human_only partition always carries at least
-- one honest marker (the unsafe class, the missing reconciliation path,
-- or the human_only scoped-control mark of the build moment).
CREATE TABLE action.action_previews (
 tenant_id uuid NOT NULL,
 preview_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no >= 1),
 supersedes_version bigint CHECK (supersedes_version IS NULL OR supersedes_version >= 1),
 definition_id text NOT NULL CHECK (char_length(definition_id) BETWEEN 1 AND 200),
 definition_version text NOT NULL CHECK (char_length(definition_version) BETWEEN 1 AND 32),
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 target_kind text NOT NULL CHECK (target_kind IN ('catalog_item', 'client', 'order')),
 target_ref uuid NOT NULL,
 typed_input jsonb NOT NULL CHECK (jsonb_typeof(typed_input) = 'object' AND octet_length(typed_input::text) <= 65536),
 supporting_facts jsonb NOT NULL CHECK (jsonb_typeof(supporting_facts) = 'array' AND octet_length(supporting_facts::text) <= 16384),
 supporting_observations jsonb NOT NULL CHECK (jsonb_typeof(supporting_observations) = 'array' AND octet_length(supporting_observations::text) <= 16384),
 snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot) = 'array' AND octet_length(snapshot::text) <= 65536),
 effects jsonb NOT NULL CHECK (jsonb_typeof(effects) = 'array' AND jsonb_array_length(effects) >= 1),
 partition text NOT NULL CHECK (partition IN ('staffy', 'human_only')),
 recovery_instruction text NOT NULL DEFAULT '' CHECK (char_length(recovery_instruction) <= 2000),
 human_owner_required boolean NOT NULL,
 unsafe_class boolean NOT NULL,
 no_reconciliation boolean NOT NULL,
 human_only_control_mark boolean NOT NULL,
 owner_epoch bigint NOT NULL CHECK (owner_epoch >= 1),
 control_generation bigint NOT NULL CHECK (control_generation >= 0),
 profile_version bigint NOT NULL CHECK (profile_version >= 0),
 policy_outcome_no bigint NOT NULL CHECK (policy_outcome_no >= 0),
 capability_decl_version text NOT NULL CHECK (char_length(capability_decl_version) BETWEEN 1 AND 64),
 digest text NOT NULL CHECK (char_length(digest) = 64),
 author_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, preview_id, version_no),
 CONSTRAINT action_previews_supersedes_shape CHECK (
   (version_no = 1 AND supersedes_version IS NULL) OR
   (version_no > 1 AND supersedes_version IS NOT NULL AND supersedes_version < version_no)),
 CONSTRAINT action_previews_partition_owner_shape CHECK (
   (partition = 'human_only' AND human_owner_required) OR
   (partition = 'staffy' AND NOT human_owner_required)),
 CONSTRAINT action_previews_human_only_marker_shape CHECK (
   partition = 'staffy' OR unsafe_class OR no_reconciliation OR human_only_control_mark)
);
CREATE INDEX action_previews_case ON action.action_previews (tenant_id, case_id, created_at);

-- The append-only event history of the preparation contour. version_no
-- = 0 is the proposal stage (the thread's attempt events — they never
-- enter the Timeline derivation of any version); version_no >= 1 scopes
-- the events of that exact version. The UNIQUE (tenant, preview,
-- version, event_no) is the raced-append backstop of the monotonic
-- counter derived under the action advisory lock.
CREATE TABLE action.action_preview_events (
 tenant_id uuid NOT NULL,
 preview_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no >= 0),
 case_id uuid NOT NULL,
 event_id uuid NOT NULL,
 event_no bigint NOT NULL CHECK (event_no >= 1),
 kind text NOT NULL CHECK (kind IN ('proposed', 'repaired', 'previewed', 'superseded', 'dry_run_recorded', 'fresh_read_recorded')),
 reason text NOT NULL DEFAULT '' CHECK (char_length(reason) <= 200),
 details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 500),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id),
 UNIQUE (tenant_id, preview_id, version_no, event_no)
);
CREATE INDEX action_preview_events_case ON action.action_preview_events (tenant_id, case_id, created_at);
CREATE INDEX action_preview_events_version ON action.action_preview_events (tenant_id, preview_id, version_no, event_no);

-- Row level security: every table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate).
ALTER TABLE action.action_proposals ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.action_proposals FORCE ROW LEVEL SECURITY;
ALTER TABLE action.action_previews ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.action_previews FORCE ROW LEVEL SECURITY;
ALTER TABLE action.action_preview_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.action_preview_events FORCE ROW LEVEL SECURITY;

CREATE POLICY action_proposals_scope ON action.action_proposals USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY action_previews_scope ON action.action_previews USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY action_preview_events_scope ON action.action_preview_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the attempts, the
-- versions and the events are never edited or deleted through the
-- application role (the append-only probe of the integration matrix
-- pins the absence of the UPDATE/DELETE paths).
GRANT USAGE ON SCHEMA action TO staffy_app;
GRANT SELECT, INSERT ON action.action_proposals TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.action_proposals FROM staffy_app;
GRANT SELECT, INSERT ON action.action_previews TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.action_previews FROM staffy_app;
GRANT SELECT, INSERT ON action.action_preview_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.action_preview_events FROM staffy_app;

-- +goose Down
DROP SCHEMA action CASCADE;
