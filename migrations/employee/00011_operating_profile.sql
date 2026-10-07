-- +goose Up
-- Story 1.13: Operating Profile and measurement rules.
-- employee.operating_profile_drafts is the ONLY mutable table of the slice:
-- its UPDATE grant exists for the revision CAS, the content refresh and the
-- approved_revision stamp written by the approval transaction.
-- employee.operating_profiles and employee.gate_thresholds are append-only
-- (INSERT/SELECT, no UPDATE, no DELETE, no TRUNCATE): an approved profile
-- version with its digest, and every threshold decision with its history,
-- are immutable. The step_up_verified fact is CHECK-enforced true — no row
-- can claim an approval that skipped step-up.

CREATE TABLE employee.operating_profile_drafts (
 tenant_id uuid NOT NULL,
 employee_id uuid NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 approved_revision bigint CHECK (approved_revision IS NULL OR approved_revision > 0),
 manifest_id uuid NOT NULL,
 config_version bigint NOT NULL CHECK (config_version > 0),
 manifest_digest char(64) NOT NULL CHECK (char_length(manifest_digest) = 64),
 content jsonb NOT NULL,
 actor_id uuid NOT NULL,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id),
 FOREIGN KEY (tenant_id, employee_id, config_version) REFERENCES employee.release_manifests(tenant_id, employee_id, config_version)
);

CREATE TABLE employee.operating_profiles (
 tenant_id uuid NOT NULL,
 employee_id uuid NOT NULL,
 version bigint NOT NULL CHECK (version > 0),
 manifest_id uuid NOT NULL,
 config_version bigint NOT NULL CHECK (config_version > 0),
 manifest_digest char(64) NOT NULL CHECK (char_length(manifest_digest) = 64),
 digest char(64) NOT NULL CHECK (char_length(digest) = 64),
 content jsonb NOT NULL,
 approver_id uuid NOT NULL,
 approver_roles jsonb NOT NULL,
 step_up_verified boolean NOT NULL CHECK (step_up_verified),
 description text NOT NULL CHECK (char_length(description) BETWEEN 1 AND 300),
 evidence_refs jsonb NOT NULL,
 approved_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id, version),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id),
 FOREIGN KEY (tenant_id, employee_id, config_version) REFERENCES employee.release_manifests(tenant_id, employee_id, config_version)
);

CREATE INDEX operating_profiles_latest_idx ON employee.operating_profiles (tenant_id, employee_id, approved_at DESC);

CREATE TABLE employee.gate_thresholds (
 tenant_id uuid NOT NULL,
 employee_id uuid NOT NULL,
 gate_id text NOT NULL CHECK (char_length(gate_id) BETWEEN 1 AND 64),
 threshold_version bigint NOT NULL CHECK (threshold_version > 0),
 value text NOT NULL CHECK (char_length(value) BETWEEN 1 AND 300),
 actor_id uuid NOT NULL,
 decision text NOT NULL CHECK (char_length(decision) BETWEEN 1 AND 300),
 evidence_refs jsonb NOT NULL,
 supersedes_version bigint CHECK (supersedes_version IS NULL OR supersedes_version > 0),
 decided_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id, gate_id, threshold_version),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id)
);

ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned','behavior_saved','voice_saved','voice_assessed','release_draft_saved','config_version_created','manifest_sealed','release_restored','profile_draft_saved','profile_approved','threshold_decided'));

ALTER TABLE employee.operating_profile_drafts ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.operating_profile_drafts FORCE ROW LEVEL SECURITY;
CREATE POLICY operating_profile_drafts_scope ON employee.operating_profile_drafts USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT, UPDATE ON employee.operating_profile_drafts TO staffy_app;
REVOKE DELETE, TRUNCATE ON employee.operating_profile_drafts FROM staffy_app;

ALTER TABLE employee.operating_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.operating_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY operating_profiles_scope ON employee.operating_profiles USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT ON employee.operating_profiles TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.operating_profiles FROM staffy_app;

ALTER TABLE employee.gate_thresholds ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.gate_thresholds FORCE ROW LEVEL SECURITY;
CREATE POLICY gate_thresholds_scope ON employee.gate_thresholds USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT ON employee.gate_thresholds TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.gate_thresholds FROM staffy_app;

-- The profile tables join the employee draft aggregate (foreign keys bind
-- them to employee.drafts); the purge function is replaced by this migration
-- (past migrations are never edited).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION employee.purge_employee_drafts(p_tenant uuid)
RETURNS TABLE(deleted_drafts bigint, dependent_rows bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE drafts bigint; assessments bigint; previews bigint; voice_versions bigint;
        behavior_versions bigint; chunks bigint; kversions bigint; sources bigint;
        kaudit bigint; koutbox bigint; idem bigint; audit_rows bigint; outbox_rows bigint;
        release_drafts bigint; config_versions bigint; manifests bigint;
        profile_drafts bigint; profiles bigint; thresholds bigint;
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
  DELETE FROM employee.gate_thresholds g WHERE g.tenant_id = p_tenant;
  GET DIAGNOSTICS thresholds = ROW_COUNT;
  DELETE FROM employee.operating_profiles p WHERE p.tenant_id = p_tenant;
  GET DIAGNOSTICS profiles = ROW_COUNT;
  DELETE FROM employee.operating_profile_drafts d WHERE d.tenant_id = p_tenant;
  GET DIAGNOSTICS profile_drafts = ROW_COUNT;
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
                    release_drafts + config_versions + manifests +
                    profile_drafts + profiles + thresholds;
  RETURN NEXT;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM employee.operating_profiles) THEN
  RAISE EXCEPTION 'approved operating profiles exist; they are immutable history — export and retain them before rollback';
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
ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned','behavior_saved','voice_saved','voice_assessed','release_draft_saved','config_version_created','manifest_sealed','release_restored'));
DROP TABLE IF EXISTS employee.gate_thresholds;
DROP TABLE IF EXISTS employee.operating_profiles;
DROP TABLE IF EXISTS employee.operating_profile_drafts;
