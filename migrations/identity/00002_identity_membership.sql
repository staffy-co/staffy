-- +goose Up
ALTER TABLE identity.memberships DROP CONSTRAINT memberships_role_check;
ALTER TABLE identity.memberships DROP CONSTRAINT memberships_status_check;
ALTER TABLE identity.memberships ADD COLUMN roles text[] NOT NULL DEFAULT ARRAY['owner']::text[];
ALTER TABLE identity.memberships ADD COLUMN scopes text[] NOT NULL DEFAULT ARRAY['configuration','integrations','operations','review','technical_expertise','claims','audit']::text[];
ALTER TABLE identity.memberships ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0);
ALTER TABLE identity.memberships ADD COLUMN updated_at timestamptz;
ALTER TABLE identity.memberships ADD COLUMN email text;
ALTER TABLE identity.memberships ADD CONSTRAINT memberships_status_check CHECK (status IN ('active','revoked'));

CREATE TABLE identity.invitations (
  tenant_id uuid NOT NULL REFERENCES identity.tenants(tenant_id),
  invitation_id uuid NOT NULL,
  email text NOT NULL,
  token_digest char(64) NOT NULL UNIQUE,
  roles text[] NOT NULL,
  scopes text[] NOT NULL,
  status text NOT NULL CHECK (status IN ('pending','accepted','revoked')),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  PRIMARY KEY (tenant_id, invitation_id),
  UNIQUE (tenant_id, email)
);
CREATE TABLE identity.membership_audit (
  tenant_id uuid NOT NULL REFERENCES identity.tenants(tenant_id),
  audit_id uuid NOT NULL,
  actor_id uuid NOT NULL,
  subject_id uuid,
  old_state jsonb,
  new_state jsonb,
  reason text NOT NULL CHECK (reason IN ('invitation_created','invitation_accepted','access_adjustment','offboarding','security_response')),
  correlation_id uuid NOT NULL,
  occurred_at timestamptz NOT NULL,
  PRIMARY KEY (tenant_id, audit_id)
);
ALTER TABLE identity.invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.invitations FORCE ROW LEVEL SECURITY;
ALTER TABLE identity.membership_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.membership_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY invitation_scope ON identity.invitations
  USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid OR token_digest = NULLIF(current_setting('staffy.invitation_digest', true), ''))
  WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY membership_audit_scope ON identity.membership_audit
  USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
GRANT SELECT, INSERT, UPDATE ON identity.memberships TO staffy_app;
GRANT SELECT, INSERT, UPDATE ON identity.invitations TO staffy_app;
GRANT SELECT, INSERT ON identity.membership_audit TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON identity.membership_audit FROM staffy_app;

-- +goose Down
DROP TABLE identity.membership_audit;
DROP TABLE identity.invitations;
ALTER TABLE identity.memberships DROP COLUMN updated_at;
ALTER TABLE identity.memberships DROP COLUMN email;
ALTER TABLE identity.memberships DROP COLUMN version;
ALTER TABLE identity.memberships DROP COLUMN scopes;
ALTER TABLE identity.memberships DROP COLUMN roles;
ALTER TABLE identity.memberships DROP CONSTRAINT memberships_status_check;
ALTER TABLE identity.memberships ADD CONSTRAINT memberships_status_check CHECK (status IN ('active'));
ALTER TABLE identity.memberships ADD CONSTRAINT memberships_role_check CHECK (role IN ('owner'));
