-- +goose Up
-- Story 1.10: artifact data perimeter. Every table is append-only: staffy_app
-- gets INSERT/SELECT only, no UPDATE, no DELETE, no TRUNCATE. Current states
-- (map entry, policy, hold) are derived at read time; there is no second writer.

CREATE SCHEMA IF NOT EXISTS privacy;

CREATE TABLE privacy.data_map_entries (
 tenant_id uuid NOT NULL,
 class text NOT NULL CHECK (class IN (
   'tenant-configuration','member-identity','employee-draft','domain-pack-pin',
   'knowledge-source-payload','behavior-configuration','voice-configuration',
   'voice-preview-media','voice-assessment','connection-configuration',
   'connection-secret-seal','capability-evidence','idempotency-records',
   'audit-trail','outbox-events')),
 version bigint NOT NULL CHECK (version > 0),
 purpose text NOT NULL CHECK (char_length(purpose) <= 600),
 legal_basis text NOT NULL CHECK (char_length(legal_basis) <= 300),
 controller text NOT NULL CHECK (char_length(controller) <= 300),
 processor text NOT NULL CHECK (char_length(processor) <= 300),
 recipients jsonb NOT NULL CHECK (jsonb_typeof(recipients) = 'array'),
 sub_processors jsonb NOT NULL CHECK (jsonb_typeof(sub_processors) = 'array'),
 locations jsonb NOT NULL CHECK (jsonb_typeof(locations) = 'array'),
 accessible_roles jsonb NOT NULL CHECK (jsonb_typeof(accessible_roles) = 'array'),
 known_copies jsonb NOT NULL CHECK (jsonb_typeof(known_copies) = 'array'),
 retention_policy_ref text NOT NULL CHECK (char_length(retention_policy_ref) <= 300),
 deletion_method text NOT NULL CHECK (char_length(deletion_method) <= 600),
 export_method text NOT NULL CHECK (char_length(export_method) <= 600),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, class, version)
);

CREATE TABLE privacy.retention_policies (
 tenant_id uuid NOT NULL,
 class text NOT NULL CHECK (class IN (
   'tenant-configuration','member-identity','employee-draft','domain-pack-pin',
   'knowledge-source-payload','behavior-configuration','voice-configuration',
   'voice-preview-media','voice-assessment','connection-configuration',
   'connection-secret-seal','capability-evidence','idempotency-records',
   'audit-trail','outbox-events')),
 version bigint NOT NULL CHECK (version > 0),
 purpose text NOT NULL CHECK (char_length(purpose) BETWEEN 1 AND 300),
 legal_basis text NOT NULL CHECK (char_length(legal_basis) BETWEEN 1 AND 300),
 source text NOT NULL CHECK (source IN ('tenant_decision','legal_requirement','contract','default')),
 terminal_event text NOT NULL CHECK (terminal_event IN (
   'last_change','source_retired','employee_deleted','invitation_expired',
   'membership_revoked','connection_revoked','preview_expired','tenant_closure')),
 active_use_days integer NOT NULL CHECK (active_use_days BETWEEN 0 AND 3650),
 archive_days integer NOT NULL CHECK (archive_days BETWEEN 0 AND 3650),
 deletion_days integer NOT NULL CHECK (deletion_days BETWEEN 0 AND 3650),
 backup_expiry_days integer NOT NULL CHECK (backup_expiry_days BETWEEN 0 AND 3650),
 legal_hold_exception text NOT NULL CHECK (char_length(legal_hold_exception) BETWEEN 1 AND 300),
 owner_note text NOT NULL CHECK (char_length(owner_note) BETWEEN 1 AND 300),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, class, version)
);

CREATE TABLE privacy.legal_hold_events (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 event text NOT NULL CHECK (event IN ('activated','released')),
 classes jsonb NOT NULL CHECK (jsonb_typeof(classes) = 'array'),
 basis text NOT NULL CHECK (char_length(basis) BETWEEN 1 AND 600),
 approver text NOT NULL CHECK (char_length(approver) BETWEEN 1 AND 300),
 review_deadline timestamptz NOT NULL,
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id)
);

CREATE TABLE privacy.deletion_requests (
 tenant_id uuid NOT NULL,
 request_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 200),
 request_hash char(64) NOT NULL,
 scope jsonb NOT NULL CHECK (jsonb_typeof(scope) = 'array'),
 authority text NOT NULL CHECK (authority = 'data_governance'),
 requested_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, request_id),
 UNIQUE (tenant_id, actor_id, idempotency_key)
);

CREATE TABLE privacy.deletion_outcomes (
 tenant_id uuid NOT NULL,
 request_id uuid NOT NULL,
 class text NOT NULL CHECK (class IN (
   'tenant-configuration','member-identity','employee-draft','domain-pack-pin',
   'knowledge-source-payload','behavior-configuration','voice-configuration',
   'voice-preview-media','voice-assessment','connection-configuration',
   'connection-secret-seal','capability-evidence','idempotency-records',
   'audit-trail','outbox-events')),
 outcome text NOT NULL CHECK (outcome IN ('deleted','scheduled','retained_by_law','unsupported','unknown')),
 affected_count bigint NOT NULL CHECK (affected_count >= 0),
 due_at timestamptz,
 note text NOT NULL CHECK (char_length(note) BETWEEN 1 AND 1000),
 block_required text,
 block_gate text,
 block_owner text,
 reconciliation_owner text,
 reconciliation_deadline timestamptz,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, request_id, class),
 FOREIGN KEY (tenant_id, request_id) REFERENCES privacy.deletion_requests(tenant_id, request_id)
);

-- Receipt journal: one row per request execution or later confirmation. Payload
-- columns carry identifiers, counters and hashes only — never deleted content.
CREATE TABLE privacy.lifecycle_receipts (
 tenant_id uuid NOT NULL,
 receipt_id uuid NOT NULL,
 request_id uuid NOT NULL,
 receipt_hash char(64) NOT NULL,
 authority text NOT NULL CHECK (authority = 'data_governance'),
 policy_versions jsonb NOT NULL CHECK (jsonb_typeof(policy_versions) = 'array'),
 outcome_summary jsonb NOT NULL CHECK (jsonb_typeof(outcome_summary) = 'array'),
 completed_at timestamptz NOT NULL,
 actor_id uuid NOT NULL,
 PRIMARY KEY (tenant_id, receipt_id),
 FOREIGN KEY (tenant_id, request_id) REFERENCES privacy.deletion_requests(tenant_id, request_id)
);

CREATE INDEX data_map_current_idx ON privacy.data_map_entries (tenant_id, class, version DESC);
CREATE INDEX retention_current_idx ON privacy.retention_policies (tenant_id, class, version DESC);
CREATE INDEX legal_hold_events_idx ON privacy.legal_hold_events (tenant_id, created_at DESC);
CREATE INDEX deletion_outcomes_idx ON privacy.deletion_outcomes (tenant_id, request_id);
CREATE INDEX lifecycle_receipts_idx ON privacy.lifecycle_receipts (tenant_id, request_id, completed_at DESC);

DO $$
DECLARE t text;
BEGIN
  GRANT USAGE ON SCHEMA privacy TO staffy_app;
  FOREACH t IN ARRAY ARRAY['data_map_entries','retention_policies','legal_hold_events','deletion_requests','deletion_outcomes','lifecycle_receipts']
  LOOP
    EXECUTE format('ALTER TABLE privacy.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE privacy.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY %I ON privacy.%I USING (tenant_id = NULLIF(current_setting(''staffy.tenant_id'', true),'''')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting(''staffy.tenant_id'', true),'''')::uuid)', t || '_scope', t);
    EXECUTE format('GRANT SELECT, INSERT ON privacy.%I TO staffy_app', t);
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON privacy.%I FROM staffy_app', t);
  END LOOP;
END $$;

-- +goose Down
DROP TABLE IF EXISTS privacy.lifecycle_receipts;
DROP TABLE IF EXISTS privacy.deletion_outcomes;
DROP TABLE IF EXISTS privacy.deletion_requests;
DROP TABLE IF EXISTS privacy.legal_hold_events;
DROP TABLE IF EXISTS privacy.retention_policies;
DROP TABLE IF EXISTS privacy.data_map_entries;
DROP SCHEMA IF EXISTS privacy;
