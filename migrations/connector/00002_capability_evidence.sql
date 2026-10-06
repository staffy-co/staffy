-- +goose Up
-- Story 1.9: immutable capability evidence.
-- Append-only proof rows for read-only probe results, anchored to a specific
-- connection, secret, environment, contract version and adapter build.
-- staffy_app gets INSERT/SELECT only: no UPDATE, no DELETE, no TRUNCATE.

CREATE TABLE connector.capability_evidence (
 tenant_id uuid NOT NULL,
 evidence_id uuid NOT NULL,
 connection_id uuid NOT NULL,
 secret_id uuid NOT NULL,
 environment text NOT NULL CHECK (environment IN ('sandbox','test')),
 contract_version text NOT NULL CHECK (char_length(contract_version) BETWEEN 1 AND 32),
 adapter_build text NOT NULL CHECK (char_length(adapter_build) BETWEEN 1 AND 64),
 capability text NOT NULL CHECK (char_length(capability) BETWEEN 1 AND 120),
 probe_kind text NOT NULL CHECK (probe_kind IN ('read_only_probe')),
 actor_id uuid NOT NULL,
 requested_at timestamptz NOT NULL,
 deadline_at timestamptz NOT NULL,
 completed_at timestamptz NOT NULL,
 outcome text NOT NULL CHECK (outcome IN ('verified','failed','unknown')),
 delivery_level text NOT NULL CHECK (delivery_level IN ('accepted','delivered','committed','unknown')),
 redacted_request text NOT NULL CHECK (char_length(redacted_request) BETWEEN 1 AND 300),
 redacted_result text NOT NULL CHECK (char_length(redacted_result) BETWEEN 1 AND 300),
 expires_at timestamptz NOT NULL,
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 200),
 request_hash text NOT NULL,
 PRIMARY KEY (tenant_id, evidence_id),
 UNIQUE (tenant_id, connection_id, idempotency_key),
 FOREIGN KEY (tenant_id, connection_id) REFERENCES connector.connections(tenant_id, connection_id)
);

CREATE INDEX capability_evidence_connection_idx ON connector.capability_evidence (tenant_id, connection_id, capability, completed_at DESC);

-- Idempotent connect/rotate/revoke (R2 Gap 5 from Story 1-8): the resulting
-- version is remembered so a retry with the same key replays the stored
-- outcome instead of reporting a false "changed by someone else" conflict.
ALTER TABLE connector.idempotency_records ADD COLUMN resulting_version bigint;

-- Audit learns the new outcomes (capability verification + idempotent replays
-- and conflicts of connect/rotate/revoke/verify).
DO $$
DECLARE c text;
BEGIN
  SELECT conname INTO c FROM pg_constraint
   WHERE conrelid = 'connector.audit'::regclass AND contype = 'c'
     AND pg_get_constraintdef(oid) ILIKE '%outcome%';
  IF c IS NOT NULL THEN
    EXECUTE format('ALTER TABLE connector.audit DROP CONSTRAINT %I', c);
  END IF;
END $$;
ALTER TABLE connector.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN (
 'created','saved','conflict','checked','rotated','revoked',
 'replayed','idempotency_conflict',
 'capability_verified','capability_failed','capability_unknown',
 'capability_replay','capability_conflict'
));

ALTER TABLE connector.capability_evidence ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector.capability_evidence FORCE ROW LEVEL SECURITY;

CREATE POLICY capability_evidence_scope ON connector.capability_evidence
 USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid)
 WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);

GRANT SELECT, INSERT ON connector.capability_evidence TO staffy_app;
GRANT SELECT, INSERT ON connector.idempotency_records TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON connector.capability_evidence FROM staffy_app;

-- +goose Down
DROP TABLE IF EXISTS connector.capability_evidence;
ALTER TABLE connector.audit DROP CONSTRAINT IF EXISTS audit_outcome_check;
ALTER TABLE connector.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','checked','rotated','revoked'));
ALTER TABLE connector.idempotency_records DROP COLUMN IF EXISTS resulting_version;
