-- +goose Up
CREATE SCHEMA IF NOT EXISTS connector;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'staffy_app') THEN
    CREATE ROLE staffy_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
  END IF;
END
$$;

CREATE TABLE connector.connections (
 tenant_id uuid NOT NULL,
 connection_id uuid NOT NULL,
 connector_id text NOT NULL,
 connector_version text NOT NULL,
 label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 120),
 environment text NOT NULL CHECK (environment IN ('sandbox','test')),
 status text NOT NULL CHECK (status IN ('draft','connected_unverified','failed','revoked')),
 authentication_state text NOT NULL CHECK (authentication_state IN ('pending','verified','failed')),
 transport_state text NOT NULL CHECK (transport_state IN ('pending','verified','failed')),
 config jsonb NOT NULL,
 secret_id uuid,
 secret_hint text,
 secret_active boolean NOT NULL DEFAULT false,
 owner_id uuid NOT NULL,
 version bigint NOT NULL CHECK (version > 0),
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, connection_id)
);

CREATE TABLE connector.connection_versions (
 tenant_id uuid NOT NULL,
 connection_id uuid NOT NULL,
 version bigint NOT NULL CHECK (version > 0),
 connector_id text NOT NULL,
 connector_version text NOT NULL,
 label text NOT NULL,
 environment text NOT NULL CHECK (environment IN ('sandbox','test')),
 status text NOT NULL,
 authentication_state text NOT NULL,
 transport_state text NOT NULL,
 config jsonb NOT NULL,
 secret_id uuid,
 secret_hint text,
 secret_active boolean NOT NULL DEFAULT false,
 actor_id uuid NOT NULL,
 change_summary text NOT NULL CHECK (char_length(change_summary) BETWEEN 1 AND 300),
 changed_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, connection_id, version),
 FOREIGN KEY (tenant_id, connection_id) REFERENCES connector.connections(tenant_id, connection_id)
);

-- Sealed secrets: HMAC of the accepted value plus the per-connection random key.
-- Neither column is ever returned through the API; staffy_app may only insert.
CREATE TABLE connector.secret_seals (
 tenant_id uuid NOT NULL,
 connection_id uuid NOT NULL,
 secret_id uuid NOT NULL,
 sealed bytea NOT NULL CHECK (octet_length(sealed) = 32),
 seal_key bytea NOT NULL CHECK (octet_length(seal_key) = 32),
 created_at timestamptz NOT NULL,
 revoked_at timestamptz,
 PRIMARY KEY (tenant_id, connection_id, secret_id),
 FOREIGN KEY (tenant_id, connection_id) REFERENCES connector.connections(tenant_id, connection_id)
);

CREATE TABLE connector.idempotency_records (
 tenant_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 operation text NOT NULL,
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 200),
 request_hash text NOT NULL,
 connection_id uuid,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, actor_id, operation, idempotency_key)
);

CREATE TABLE connector.audit (
 tenant_id uuid NOT NULL,
 audit_id uuid NOT NULL,
 connection_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 attempted_version bigint NOT NULL,
 resulting_version bigint,
 outcome text NOT NULL CHECK (outcome IN ('created','saved','conflict','checked','rotated','revoked')),
 correlation_id uuid NOT NULL,
 occurred_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, audit_id)
);

CREATE TABLE connector.outbox_events (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 connection_id uuid NOT NULL,
 event_type text NOT NULL,
 schema_version integer NOT NULL DEFAULT 1,
 aggregate_version bigint NOT NULL,
 actor_id uuid NOT NULL,
 correlation_id uuid NOT NULL,
 occurred_at timestamptz NOT NULL,
 payload jsonb NOT NULL,
 published_at timestamptz,
 PRIMARY KEY (tenant_id, event_id)
);

ALTER TABLE connector.connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector.connections FORCE ROW LEVEL SECURITY;
ALTER TABLE connector.connection_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector.connection_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE connector.secret_seals ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector.secret_seals FORCE ROW LEVEL SECURITY;
ALTER TABLE connector.idempotency_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector.idempotency_records FORCE ROW LEVEL SECURITY;
ALTER TABLE connector.audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector.audit FORCE ROW LEVEL SECURITY;
ALTER TABLE connector.outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector.outbox_events FORCE ROW LEVEL SECURITY;

CREATE POLICY connection_scope ON connector.connections USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY connection_version_scope ON connector.connection_versions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY secret_seal_scope ON connector.secret_seals USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY connector_idempotency_scope ON connector.idempotency_records USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid AND actor_id = NULLIF(current_setting('staffy.principal_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid AND actor_id = NULLIF(current_setting('staffy.principal_id', true),'')::uuid);
CREATE POLICY connector_audit_scope ON connector.audit USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
CREATE POLICY connector_outbox_scope ON connector.outbox_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);

GRANT USAGE ON SCHEMA connector TO staffy_app;
GRANT SELECT, INSERT, UPDATE ON connector.connections TO staffy_app;
GRANT SELECT, INSERT ON connector.connection_versions, connector.audit, connector.outbox_events TO staffy_app;
GRANT SELECT, INSERT, UPDATE ON connector.secret_seals TO staffy_app;
GRANT SELECT, INSERT ON connector.idempotency_records TO staffy_app;
REVOKE DELETE, TRUNCATE ON connector.connections, connector.connection_versions, connector.secret_seals, connector.idempotency_records, connector.audit, connector.outbox_events FROM staffy_app;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM connector.connections) THEN
  RAISE EXCEPTION 'connection history exists; export and retain it before rollback';
 END IF;
END $$;
-- +goose StatementEnd
DROP TABLE IF EXISTS connector.outbox_events;
DROP TABLE IF EXISTS connector.audit;
DROP TABLE IF EXISTS connector.idempotency_records;
DROP TABLE IF EXISTS connector.secret_seals;
DROP TABLE IF EXISTS connector.connection_versions;
DROP TABLE IF EXISTS connector.connections;
DROP SCHEMA IF EXISTS connector;
