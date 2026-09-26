-- +goose Up
CREATE TABLE employee.knowledge_sources (
 tenant_id uuid NOT NULL, employee_id uuid NOT NULL, source_id uuid NOT NULL,
 source_type text NOT NULL CHECK (source_type IN ('faq','regulation','dialogue')),
 title text NOT NULL, owner_name text NOT NULL, provenance text NOT NULL,
 checksum_key bytea NOT NULL CHECK (octet_length(checksum_key) = 32),
 source_scope text NOT NULL, priority integer NOT NULL CHECK (priority BETWEEN 0 AND 100),
 language text NOT NULL, status text NOT NULL DEFAULT 'processing' CHECK (status IN ('processing','ready','retired')),
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0), current_version bigint NOT NULL DEFAULT 0 CHECK (current_version >= 0),
 conflict_note text NOT NULL DEFAULT '', conflict_provenance text NOT NULL DEFAULT '',
 fresh_until timestamptz, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id, source_id),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id)
);
CREATE TABLE employee.knowledge_versions (
 tenant_id uuid NOT NULL, employee_id uuid NOT NULL, source_id uuid NOT NULL,
 version bigint NOT NULL CHECK (version > 0), checksum char(64) NOT NULL,
 source_text text NOT NULL, created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id, source_id, version),
 FOREIGN KEY (tenant_id, employee_id, source_id) REFERENCES employee.knowledge_sources(tenant_id, employee_id, source_id)
);
CREATE TABLE employee.knowledge_chunks (
 tenant_id uuid NOT NULL, employee_id uuid NOT NULL, source_id uuid NOT NULL,
 version bigint NOT NULL, position integer NOT NULL CHECK (position >= 0),
 chunk_text text NOT NULL, checksum char(64) NOT NULL,
 PRIMARY KEY (tenant_id, employee_id, source_id, version, position),
 FOREIGN KEY (tenant_id, employee_id, source_id, version) REFERENCES employee.knowledge_versions(tenant_id, employee_id, source_id, version)
);
CREATE TABLE employee.knowledge_audit (
 tenant_id uuid NOT NULL, audit_id uuid NOT NULL, employee_id uuid NOT NULL, source_id uuid NOT NULL,
 actor_id uuid NOT NULL, attempted_version bigint NOT NULL, resulting_version bigint,
 outcome text NOT NULL, correlation_id uuid NOT NULL, occurred_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id,audit_id),
 FOREIGN KEY (tenant_id,employee_id,source_id) REFERENCES employee.knowledge_sources(tenant_id,employee_id,source_id)
);
CREATE TABLE employee.knowledge_outbox (
 tenant_id uuid NOT NULL, event_id uuid NOT NULL, employee_id uuid NOT NULL, source_id uuid NOT NULL,
 event_type text NOT NULL, schema_version integer NOT NULL DEFAULT 1, aggregate_version bigint NOT NULL,
 actor_id uuid NOT NULL, correlation_id uuid NOT NULL, occurred_at timestamptz NOT NULL,
 payload jsonb NOT NULL, published_at timestamptz,
 PRIMARY KEY (tenant_id,event_id),
 FOREIGN KEY (tenant_id,employee_id,source_id) REFERENCES employee.knowledge_sources(tenant_id,employee_id,source_id)
);
ALTER TABLE employee.knowledge_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.knowledge_sources FORCE ROW LEVEL SECURITY;
ALTER TABLE employee.knowledge_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.knowledge_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE employee.knowledge_chunks ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.knowledge_chunks FORCE ROW LEVEL SECURITY;
ALTER TABLE employee.knowledge_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.knowledge_audit FORCE ROW LEVEL SECURITY;
ALTER TABLE employee.knowledge_outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.knowledge_outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY knowledge_source_scope ON employee.knowledge_sources USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY knowledge_version_scope ON employee.knowledge_versions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY knowledge_chunk_scope ON employee.knowledge_chunks USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY knowledge_audit_scope ON employee.knowledge_audit USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY knowledge_outbox_scope ON employee.knowledge_outbox USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
GRANT SELECT,INSERT,UPDATE ON employee.knowledge_sources TO staffy_app;
GRANT SELECT,INSERT ON employee.knowledge_versions,employee.knowledge_chunks,employee.knowledge_audit,employee.knowledge_outbox TO staffy_app;
REVOKE UPDATE,DELETE,TRUNCATE ON employee.knowledge_versions,employee.knowledge_chunks,employee.knowledge_audit,employee.knowledge_outbox FROM staffy_app;

-- +goose Down
DROP TABLE employee.knowledge_outbox;
DROP TABLE employee.knowledge_audit;
DROP TABLE employee.knowledge_chunks;
DROP TABLE employee.knowledge_versions;
DROP TABLE employee.knowledge_sources;
