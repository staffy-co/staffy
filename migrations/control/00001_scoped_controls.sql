-- +goose Up
-- Story 3-2, slice 1 — the control module schema (AD-12/13/16).
--
-- Honesty decisions materialized here:
--   - scoped_controls is ONE row per (tenant, scope pair): the PK is the
--     spec's UNIQUE; the row is never deleted (state='removed' is a live
--     state), so the generation series of a point stays monotonic forever;
--   - mode/state/action/outcome/event_type carry CHECK dictionaries (the
--     second line of defense behind the compile-time guards; a probe of
--     direct damage has to DROP the constraint first — the read path
--     answers such a corrupted value as paused, fail-closed);
--   - the window CHECK refuses empty/inverted windows: such a control
--     would never act and would only corrupt the operator's model;
--   - audit is append-only BY GRANTS (SELECT+INSERT only, UPDATE/DELETE/
--     TRUNCATE revoked) — and the conflict outcome is a FIRST-CLASS row:
--     the history of BOTH optimistic attempts is material (AC-11);
--   - outbox_events is the AD-13 transactional outbox, byte-identical in
--     shape to the employee precedent: aggregate_version = generation;
--   - RLS ENABLE+FORCE with the staffy.tenant_id policy on all three
--     tables (the employee precedent) — a control never crosses tenants.
CREATE SCHEMA IF NOT EXISTS control;
CREATE TABLE control.scoped_controls (
 tenant_id uuid NOT NULL,
 scope_kind text NOT NULL CHECK (scope_kind IN ('tenant','connector','channel','service','policy_outcome','autonomy_class','campaign')),
 scope_ref text NOT NULL CHECK (char_length(scope_ref) BETWEEN 1 AND 120),
 mode text NOT NULL CHECK (mode IN ('accept','intake_only/no_promises','human_only','paused')),
 reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 500),
 actor_id uuid NOT NULL,
 prev_mode text NOT NULL DEFAULT '' CHECK (prev_mode = '' OR prev_mode IN ('accept','intake_only/no_promises','human_only','paused')),
 generation bigint NOT NULL DEFAULT 1 CHECK (generation >= 1),
 scheduled_start_at timestamptz,
 expires_at timestamptz,
 state text NOT NULL DEFAULT 'active' CHECK (state IN ('active','removed')),
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, scope_kind, scope_ref),
 CONSTRAINT scoped_controls_window CHECK (scheduled_start_at IS NULL OR expires_at IS NULL OR scheduled_start_at < expires_at)
);
CREATE TABLE control.audit (
 tenant_id uuid NOT NULL,
 audit_id uuid NOT NULL,
 scope_kind text NOT NULL,
 scope_ref text NOT NULL CHECK (char_length(scope_ref) BETWEEN 1 AND 120),
 actor_id uuid NOT NULL,
 action text NOT NULL CHECK (action IN ('set','resume')),
 requested_mode text NOT NULL DEFAULT '' CHECK (requested_mode = '' OR requested_mode IN ('accept','intake_only/no_promises','human_only','paused')),
 prev_mode text NOT NULL DEFAULT '' CHECK (prev_mode = '' OR prev_mode IN ('accept','intake_only/no_promises','human_only','paused')),
 expected_generation bigint NOT NULL CHECK (expected_generation >= 0),
 resulting_generation bigint NOT NULL CHECK (resulting_generation >= 1),
 outcome text NOT NULL CHECK (outcome IN ('applied','conflict')),
 reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 500),
 correlation_id uuid NOT NULL,
 occurred_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, audit_id),
 FOREIGN KEY (tenant_id, scope_kind, scope_ref) REFERENCES control.scoped_controls(tenant_id, scope_kind, scope_ref)
);
CREATE TABLE control.outbox_events (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 scope_kind text NOT NULL,
 scope_ref text NOT NULL CHECK (char_length(scope_ref) BETWEEN 1 AND 120),
 event_type text NOT NULL CHECK (event_type = 'scoped_control.changed'),
 schema_version integer NOT NULL CHECK (schema_version > 0),
 aggregate_version bigint NOT NULL CHECK (aggregate_version >= 1),
 actor_id uuid NOT NULL,
 correlation_id uuid NOT NULL,
 occurred_at timestamptz NOT NULL,
 payload jsonb NOT NULL,
 published_at timestamptz,
 PRIMARY KEY (tenant_id, event_id),
 FOREIGN KEY (tenant_id, scope_kind, scope_ref) REFERENCES control.scoped_controls(tenant_id, scope_kind, scope_ref)
);
ALTER TABLE control.scoped_controls ENABLE ROW LEVEL SECURITY;
ALTER TABLE control.scoped_controls FORCE ROW LEVEL SECURITY;
ALTER TABLE control.audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE control.audit FORCE ROW LEVEL SECURITY;
ALTER TABLE control.outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE control.outbox_events FORCE ROW LEVEL SECURITY;
CREATE POLICY scoped_controls_scope ON control.scoped_controls USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY audit_scope ON control.audit USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY outbox_scope ON control.outbox_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
GRANT USAGE ON SCHEMA control TO staffy_app;
GRANT SELECT, INSERT, UPDATE ON control.scoped_controls TO staffy_app;
GRANT SELECT, INSERT ON control.audit TO staffy_app;
GRANT SELECT, INSERT ON control.outbox_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON control.audit, control.outbox_events FROM staffy_app;

-- +goose Down
DROP SCHEMA control CASCADE;
