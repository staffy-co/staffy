-- +goose Up
CREATE SCHEMA IF NOT EXISTS identity;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'staffy_app') THEN
    CREATE ROLE staffy_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
  END IF;
END
$$;

CREATE TABLE identity.principals (
  principal_id uuid PRIMARY KEY,
  created_at timestamptz NOT NULL
);

CREATE TABLE identity.tenants (
  tenant_id uuid PRIMARY KEY,
  name text NOT NULL CHECK (char_length(name) BETWEEN 2 AND 160),
  created_at timestamptz NOT NULL,
  created_by uuid NOT NULL REFERENCES identity.principals(principal_id),
  CHECK (tenant_id <> '00000000-0000-0000-0000-000000000000'::uuid)
);

CREATE TABLE identity.memberships (
  tenant_id uuid NOT NULL REFERENCES identity.tenants(tenant_id),
  principal_id uuid NOT NULL REFERENCES identity.principals(principal_id),
  role text NOT NULL CHECK (role IN ('owner')),
  status text NOT NULL CHECK (status IN ('active')),
  created_at timestamptz NOT NULL,
  PRIMARY KEY (tenant_id, principal_id)
);

CREATE TABLE identity.idempotency_records (
  principal_id uuid NOT NULL REFERENCES identity.principals(principal_id),
  operation text NOT NULL,
  idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 200),
  request_hash text NOT NULL CHECK (char_length(request_hash) = 64),
  tenant_id uuid NOT NULL REFERENCES identity.tenants(tenant_id),
  response_body jsonb NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (principal_id, operation, idempotency_key)
);

CREATE TABLE identity.outbox_events (
  tenant_id uuid NOT NULL REFERENCES identity.tenants(tenant_id),
  event_id uuid NOT NULL,
  event_type text NOT NULL,
  schema_version integer NOT NULL CHECK (schema_version > 0),
  aggregate_id uuid NOT NULL,
  actor_id uuid NOT NULL REFERENCES identity.principals(principal_id),
  correlation_id uuid NOT NULL,
  occurred_at timestamptz NOT NULL,
  payload jsonb NOT NULL,
  published_at timestamptz,
  PRIMARY KEY (tenant_id, event_id),
  CHECK (event_type <> 'identity.tenant_created' OR aggregate_id = tenant_id)
);

ALTER TABLE identity.principals ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.principals FORCE ROW LEVEL SECURITY;
ALTER TABLE identity.tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.tenants FORCE ROW LEVEL SECURITY;
ALTER TABLE identity.memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.memberships FORCE ROW LEVEL SECURITY;
ALTER TABLE identity.idempotency_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.idempotency_records FORCE ROW LEVEL SECURITY;
ALTER TABLE identity.outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.outbox_events FORCE ROW LEVEL SECURITY;

CREATE POLICY principal_scope ON identity.principals
  USING (principal_id = NULLIF(current_setting('staffy.principal_id', true), '')::uuid)
  WITH CHECK (principal_id = NULLIF(current_setting('staffy.principal_id', true), '')::uuid);
CREATE POLICY tenant_scope ON identity.tenants
  USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY membership_scope ON identity.memberships
  USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY idempotency_principal_scope ON identity.idempotency_records
  USING (
    principal_id = NULLIF(current_setting('staffy.principal_id', true), '')::uuid
    AND (
      NULLIF(current_setting('staffy.tenant_id', true), '') IS NULL
      OR tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid
    )
  )
  WITH CHECK (
    principal_id = NULLIF(current_setting('staffy.principal_id', true), '')::uuid
    AND tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid
  );
CREATE POLICY outbox_tenant_scope ON identity.outbox_events
  USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

GRANT USAGE ON SCHEMA identity TO staffy_app;
GRANT SELECT, INSERT ON identity.principals TO staffy_app;
GRANT SELECT, INSERT ON identity.tenants TO staffy_app;
GRANT SELECT, INSERT ON identity.memberships TO staffy_app;
GRANT SELECT, INSERT ON identity.idempotency_records TO staffy_app;
GRANT SELECT, INSERT ON identity.outbox_events TO staffy_app;

REVOKE UPDATE, DELETE, TRUNCATE ON identity.outbox_events FROM staffy_app;
