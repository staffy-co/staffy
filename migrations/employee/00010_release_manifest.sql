-- +goose Up
-- Story 1.12: configuration versions and Release Manifest.
-- employee.release_drafts is the ONLY mutable table of the slice: its UPDATE
-- grant exists for the revision CAS and ref refresh. employee.config_versions
-- and employee.release_manifests are append-only (INSERT/SELECT, no UPDATE,
-- no DELETE, no TRUNCATE): a sealed manifest and its digest are immutable;
-- under one configuration version there is at most one manifest (UNIQUE).

CREATE TABLE employee.release_drafts (
 tenant_id uuid NOT NULL,
 employee_id uuid NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 refs jsonb NOT NULL,
 restored_from jsonb,
 actor_id uuid NOT NULL,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id)
);

CREATE TABLE employee.config_versions (
 tenant_id uuid NOT NULL,
 employee_id uuid NOT NULL,
 version_id uuid NOT NULL,
 version bigint NOT NULL CHECK (version > 0),
 refs jsonb NOT NULL,
 description text NOT NULL CHECK (char_length(description) BETWEEN 1 AND 300),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id, version),
 UNIQUE (tenant_id, employee_id, version_id),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id)
);

CREATE TABLE employee.release_manifests (
 tenant_id uuid NOT NULL,
 employee_id uuid NOT NULL,
 manifest_id uuid NOT NULL,
 config_version bigint NOT NULL CHECK (config_version > 0),
 schema_version integer NOT NULL CHECK (schema_version > 0),
 status text NOT NULL CHECK (status IN ('candidate','incomplete')),
 digest char(64) NOT NULL,
 content jsonb NOT NULL,
 actor_id uuid NOT NULL,
 sealed_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id, manifest_id),
 UNIQUE (tenant_id, employee_id, config_version),
 FOREIGN KEY (tenant_id, employee_id, config_version) REFERENCES employee.config_versions(tenant_id, employee_id, version)
);

CREATE INDEX release_manifests_sealed_idx ON employee.release_manifests (tenant_id, employee_id, sealed_at DESC);

ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned','behavior_saved','voice_saved','voice_assessed','release_draft_saved','config_version_created','manifest_sealed','release_restored'));

ALTER TABLE employee.release_drafts ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.release_drafts FORCE ROW LEVEL SECURITY;
CREATE POLICY release_drafts_scope ON employee.release_drafts USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT, UPDATE ON employee.release_drafts TO staffy_app;
REVOKE DELETE, TRUNCATE ON employee.release_drafts FROM staffy_app;

ALTER TABLE employee.config_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.config_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY config_versions_scope ON employee.config_versions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT ON employee.config_versions TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.config_versions FROM staffy_app;

ALTER TABLE employee.release_manifests ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.release_manifests FORCE ROW LEVEL SECURITY;
CREATE POLICY release_manifests_scope ON employee.release_manifests USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT ON employee.release_manifests TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.release_manifests FROM staffy_app;

-- The release tables join the employee draft aggregate: foreign keys bind
-- them to employee.drafts, so the honest deletion unit of the employee-draft
-- class stays the whole per-employee aggregate. The purge function is
-- replaced by this migration (the past migration itself is never edited).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION employee.purge_employee_drafts(p_tenant uuid)
RETURNS TABLE(deleted_drafts bigint, dependent_rows bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE drafts bigint; assessments bigint; previews bigint; voice_versions bigint;
        behavior_versions bigint; chunks bigint; kversions bigint; sources bigint;
        kaudit bigint; koutbox bigint; idem bigint; audit_rows bigint; outbox_rows bigint;
        release_drafts bigint; config_versions bigint; manifests bigint;
BEGIN
  DELETE FROM employee.voice_assessments a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS assessments = ROW_COUNT;
  DELETE FROM employee.voice_previews p WHERE p.tenant_id = p_tenant;
  GET DIAGNOSTICS previews = ROW_COUNT;
  DELETE FROM employee.voice_versions v WHERE v.tenant_id = p_tenant;
  GET DIAGNOSTICS voice_versions = ROW_COUNT;
  DELETE FROM employee.behavior_versions b WHERE b.tenant_id = p_tenant;
  GET DIAGNOSTICS behavior_versions = ROW_COUNT;
  DELETE FROM employee.knowledge_chunks c WHERE c.tenant_id = p_tenant;
  GET DIAGNOSTICS chunks = ROW_COUNT;
  DELETE FROM employee.knowledge_versions v WHERE v.tenant_id = p_tenant;
  GET DIAGNOSTICS kversions = ROW_COUNT;
  DELETE FROM employee.knowledge_audit a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS kaudit = ROW_COUNT;
  DELETE FROM employee.knowledge_outbox o WHERE o.tenant_id = p_tenant;
  GET DIAGNOSTICS koutbox = ROW_COUNT;
  DELETE FROM employee.knowledge_sources s WHERE s.tenant_id = p_tenant;
  GET DIAGNOSTICS sources = ROW_COUNT;
  DELETE FROM employee.idempotency_records i WHERE i.tenant_id = p_tenant;
  GET DIAGNOSTICS idem = ROW_COUNT;
  DELETE FROM employee.release_manifests m WHERE m.tenant_id = p_tenant;
  GET DIAGNOSTICS manifests = ROW_COUNT;
  DELETE FROM employee.config_versions v WHERE v.tenant_id = p_tenant;
  GET DIAGNOSTICS config_versions = ROW_COUNT;
  DELETE FROM employee.release_drafts d WHERE d.tenant_id = p_tenant;
  GET DIAGNOSTICS release_drafts = ROW_COUNT;
  DELETE FROM employee.audit a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS audit_rows = ROW_COUNT;
  DELETE FROM employee.outbox_events o WHERE o.tenant_id = p_tenant;
  GET DIAGNOSTICS outbox_rows = ROW_COUNT;
  DELETE FROM employee.drafts d WHERE d.tenant_id = p_tenant;
  GET DIAGNOSTICS drafts = ROW_COUNT;
  deleted_drafts := drafts;
  dependent_rows := assessments + previews + voice_versions + behavior_versions +
                    chunks + kversions + sources + kaudit + koutbox + idem + audit_rows + outbox_rows +
                    release_drafts + config_versions + manifests;
  RETURN NEXT;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM employee.release_manifests) THEN
  RAISE EXCEPTION 'sealed manifests exist; they are immutable history — export and retain them before rollback';
 END IF;
END $$;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION employee.purge_employee_drafts(p_tenant uuid)
RETURNS TABLE(deleted_drafts bigint, dependent_rows bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE drafts bigint; assessments bigint; previews bigint; voice_versions bigint;
        behavior_versions bigint; chunks bigint; kversions bigint; sources bigint;
        kaudit bigint; koutbox bigint; idem bigint; audit_rows bigint; outbox_rows bigint;
BEGIN
  DELETE FROM employee.voice_assessments a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS assessments = ROW_COUNT;
  DELETE FROM employee.voice_previews p WHERE p.tenant_id = p_tenant;
  GET DIAGNOSTICS previews = ROW_COUNT;
  DELETE FROM employee.voice_versions v WHERE v.tenant_id = p_tenant;
  GET DIAGNOSTICS voice_versions = ROW_COUNT;
  DELETE FROM employee.behavior_versions b WHERE b.tenant_id = p_tenant;
  GET DIAGNOSTICS behavior_versions = ROW_COUNT;
  DELETE FROM employee.knowledge_chunks c WHERE c.tenant_id = p_tenant;
  GET DIAGNOSTICS chunks = ROW_COUNT;
  DELETE FROM employee.knowledge_versions v WHERE v.tenant_id = p_tenant;
  GET DIAGNOSTICS kversions = ROW_COUNT;
  DELETE FROM employee.knowledge_audit a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS kaudit = ROW_COUNT;
  DELETE FROM employee.knowledge_outbox o WHERE o.tenant_id = p_tenant;
  GET DIAGNOSTICS koutbox = ROW_COUNT;
  DELETE FROM employee.knowledge_sources s WHERE s.tenant_id = p_tenant;
  GET DIAGNOSTICS sources = ROW_COUNT;
  DELETE FROM employee.idempotency_records i WHERE i.tenant_id = p_tenant;
  GET DIAGNOSTICS idem = ROW_COUNT;
  DELETE FROM employee.audit a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS audit_rows = ROW_COUNT;
  DELETE FROM employee.outbox_events o WHERE o.tenant_id = p_tenant;
  GET DIAGNOSTICS outbox_rows = ROW_COUNT;
  DELETE FROM employee.drafts d WHERE d.tenant_id = p_tenant;
  GET DIAGNOSTICS drafts = ROW_COUNT;
  deleted_drafts := drafts;
  dependent_rows := assessments + previews + voice_versions + behavior_versions +
                    chunks + kversions + sources + kaudit + koutbox + idem + audit_rows + outbox_rows;
  RETURN NEXT;
END $$;
-- +goose StatementEnd
ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned','behavior_saved','voice_saved','voice_assessed'));
DROP TABLE IF EXISTS employee.release_manifests;
DROP TABLE IF EXISTS employee.config_versions;
DROP TABLE IF EXISTS employee.release_drafts;
