-- +goose Up
CREATE SCHEMA IF NOT EXISTS employee;
CREATE TABLE employee.drafts (
 tenant_id uuid NOT NULL, employee_id uuid NOT NULL, owner_id uuid NOT NULL,
 status text NOT NULL DEFAULT 'draft' CHECK (status = 'draft'),
 readiness text NOT NULL DEFAULT 'incomplete' CHECK (readiness = 'incomplete'),
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
 current_step text NOT NULL DEFAULT 'brief' CHECK (current_step IN ('brief','launch_plan')),
 brief jsonb NOT NULL DEFAULT '{}'::jsonb,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id)
);
CREATE TABLE employee.idempotency_records (
 tenant_id uuid NOT NULL, actor_id uuid NOT NULL, operation text NOT NULL,
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 200),
 request_hash char(64) NOT NULL, employee_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, actor_id, operation, idempotency_key),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id)
);
CREATE TABLE employee.audit (
 tenant_id uuid NOT NULL, audit_id uuid NOT NULL, employee_id uuid NOT NULL,
 actor_id uuid NOT NULL, attempted_version bigint NOT NULL, resulting_version bigint,
 outcome text NOT NULL CHECK (outcome IN ('created','saved','conflict')),
 correlation_id uuid NOT NULL, occurred_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, audit_id),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id)
);
CREATE TABLE employee.outbox_events (
 tenant_id uuid NOT NULL, event_id uuid NOT NULL, employee_id uuid NOT NULL,
 event_type text NOT NULL, schema_version integer NOT NULL CHECK (schema_version > 0),
 aggregate_version bigint NOT NULL, actor_id uuid NOT NULL, correlation_id uuid NOT NULL,
 occurred_at timestamptz NOT NULL, payload jsonb NOT NULL, published_at timestamptz,
 PRIMARY KEY (tenant_id, event_id),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id)
);
ALTER TABLE employee.drafts ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.drafts FORCE ROW LEVEL SECURITY;
ALTER TABLE employee.idempotency_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.idempotency_records FORCE ROW LEVEL SECURITY;
ALTER TABLE employee.audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.audit FORCE ROW LEVEL SECURITY;
ALTER TABLE employee.outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.outbox_events FORCE ROW LEVEL SECURITY;
CREATE POLICY draft_scope ON employee.drafts USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY idempotency_scope ON employee.idempotency_records USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid AND actor_id = NULLIF(current_setting('staffy.principal_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid AND actor_id = NULLIF(current_setting('staffy.principal_id', true),'')::uuid);
CREATE POLICY audit_scope ON employee.audit USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY outbox_scope ON employee.outbox_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
GRANT USAGE ON SCHEMA employee TO staffy_app;
GRANT SELECT, INSERT, UPDATE ON employee.drafts TO staffy_app;
GRANT SELECT, INSERT ON employee.idempotency_records TO staffy_app;
GRANT SELECT, INSERT ON employee.audit TO staffy_app;
GRANT SELECT, INSERT ON employee.outbox_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.audit, employee.outbox_events FROM staffy_app;

-- +goose Down
DROP SCHEMA employee CASCADE;
