-- +goose Up
-- Story 1.14: Readiness-проверка и контролируемая публикация.
-- employee.gate_instances and employee.release_admissions are append-only
-- (INSERT/SELECT, no UPDATE, no DELETE, no TRUNCATE). A gate instance is
-- one lifecycle record against the fixed platform dictionary; every state
-- change inserts a new record linked by supersedes_instance_id (at most one
-- successor — the partial UNIQUE index; a failed, expired or superseded
-- PASS is never overwritten). A release admission is one publication
-- decision: the monotonic decision_no under the employee aggregate lock,
-- the exact manifest and profile digests, the per-route results, the
-- step_up fact (CHECK-enforced true) and the canonical digest. The UNIQUE
-- (stage, manifest, profile_version) triple enforces "exactly one decision
-- per candidate and stage" — a raced or repeated publication surfaces as
-- an honest conflict, never as a second record.

CREATE TABLE employee.gate_instances (
 tenant_id uuid NOT NULL,
 employee_id uuid NOT NULL,
 instance_id uuid NOT NULL,
 definition_id text NOT NULL CHECK (definition_id IN ('OQ-7B','OQ-11','OQ-12','OQ-13B','OQ-14','AG-1','AG-2','AG-4')),
 route_channel text CHECK (route_channel IS NULL OR route_channel IN ('phone','web_chat','telegram','whatsapp','email','max','jivo','avito','mobile_app_chat')),
 route_direction text CHECK (route_direction IS NULL OR route_direction IN ('inbound','outbound')),
 modality text CHECK (modality IS NULL OR modality IN ('voice','text')),
 state text NOT NULL CHECK (state IN ('open','in_test','pass','fail','expired','not_applicable')),
 approved_by jsonb NOT NULL DEFAULT '[]'::jsonb,
 evidence_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
 evidence_hash text,
 not_applicable_reason text,
 tested_at timestamptz,
 expiry_trigger text,
 supersedes_instance_id uuid,
 actor_id uuid NOT NULL,
 recorded_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, employee_id, instance_id),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id),
 FOREIGN KEY (tenant_id, employee_id, supersedes_instance_id) REFERENCES employee.gate_instances(tenant_id, employee_id, instance_id),
 CONSTRAINT gate_scope_route_both CHECK ((route_channel IS NULL AND route_direction IS NULL) OR (route_channel IS NOT NULL AND route_direction IS NOT NULL)),
 CONSTRAINT gate_scope_route_dictionary CHECK (route_channel IS NULL OR definition_id = 'OQ-11'),
 CONSTRAINT gate_pass_discipline CHECK (state <> 'pass' OR (
   jsonb_array_length(approved_by) > 0 AND
   jsonb_array_length(evidence_refs) > 0 AND
   evidence_hash ~ '^[0-9a-f]{64}$')),
 CONSTRAINT gate_not_applicable_reason CHECK (state <> 'not_applicable' OR char_length(not_applicable_reason) BETWEEN 1 AND 300),
 CONSTRAINT gate_pass_payload_only CHECK (state <> 'pass' OR (not_applicable_reason IS NULL OR char_length(not_applicable_reason) BETWEEN 1 AND 300))
);

CREATE UNIQUE INDEX gate_instances_one_successor ON employee.gate_instances (tenant_id, supersedes_instance_id) WHERE supersedes_instance_id IS NOT NULL;
CREATE INDEX gate_instances_chain ON employee.gate_instances (tenant_id, employee_id, definition_id, recorded_at DESC);

CREATE TABLE employee.release_admissions (
 tenant_id uuid NOT NULL,
 employee_id uuid NOT NULL,
 decision_no bigint NOT NULL CHECK (decision_no > 0),
 stage text NOT NULL CHECK (stage IN ('synthetic','closed_account_test','controlled_real_customer','live')),
 manifest_id uuid NOT NULL,
 config_version bigint NOT NULL CHECK (config_version > 0),
 manifest_digest char(64) NOT NULL CHECK (char_length(manifest_digest) = 64),
 profile_version bigint NOT NULL CHECK (profile_version > 0),
 profile_digest char(64) NOT NULL CHECK (char_length(profile_digest) = 64),
 routes jsonb NOT NULL,
 actor_id uuid NOT NULL,
 approver_roles jsonb NOT NULL DEFAULT '[]'::jsonb,
 step_up_verified boolean NOT NULL CHECK (step_up_verified),
 decided_at timestamptz NOT NULL,
 digest char(64) NOT NULL CHECK (char_length(digest) = 64),
 PRIMARY KEY (tenant_id, employee_id, decision_no),
 UNIQUE (tenant_id, employee_id, stage, manifest_id, profile_version),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id),
 FOREIGN KEY (tenant_id, employee_id, config_version) REFERENCES employee.release_manifests(tenant_id, employee_id, config_version),
 FOREIGN KEY (tenant_id, employee_id, profile_version) REFERENCES employee.operating_profiles(tenant_id, employee_id, version)
);

CREATE INDEX release_admissions_ladder ON employee.release_admissions (tenant_id, employee_id, decision_no DESC);

ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned','behavior_saved','voice_saved','voice_assessed','release_draft_saved','config_version_created','manifest_sealed','release_restored','profile_draft_saved','profile_approved','threshold_decided','gate_recorded','release_published'));

ALTER TABLE employee.gate_instances ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.gate_instances FORCE ROW LEVEL SECURITY;
CREATE POLICY gate_instances_scope ON employee.gate_instances USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT ON employee.gate_instances TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.gate_instances FROM staffy_app;

ALTER TABLE employee.release_admissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.release_admissions FORCE ROW LEVEL SECURITY;
CREATE POLICY release_admissions_scope ON employee.release_admissions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT ON employee.release_admissions TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.release_admissions FROM staffy_app;

-- The gate instances are tenant-level records inside the employee purge
-- boundary; the release admissions join the employee draft aggregate (the
-- foreign keys bind them to drafts, manifests and approved profiles). The
-- purge function is replaced by this migration (past migrations are never
-- edited).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION employee.purge_employee_drafts(p_tenant uuid)
RETURNS TABLE(deleted_drafts bigint, dependent_rows bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE drafts bigint; assessments bigint; previews bigint; voice_versions bigint;
        behavior_versions bigint; chunks bigint; kversions bigint; sources bigint;
        kaudit bigint; koutbox bigint; idem bigint; audit_rows bigint; outbox_rows bigint;
        release_drafts bigint; config_versions bigint; manifests bigint;
        profile_drafts bigint; profiles bigint; thresholds bigint;
        gate_rows bigint; admissions bigint;
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
  DELETE FROM employee.release_admissions a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS admissions = ROW_COUNT;
  DELETE FROM employee.gate_instances g WHERE g.tenant_id = p_tenant;
  GET DIAGNOSTICS gate_rows = ROW_COUNT;
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
                    profile_drafts + profiles + thresholds + gate_rows + admissions;
  RETURN NEXT;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM employee.release_admissions) THEN
  RAISE EXCEPTION 'release admission decisions exist; they are immutable history — export and retain them before rollback';
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
ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned','behavior_saved','voice_saved','voice_assessed','release_draft_saved','config_version_created','manifest_sealed','release_restored','profile_draft_saved','profile_approved','threshold_decided'));
DROP TABLE IF EXISTS employee.release_admissions;
DROP TABLE IF EXISTS employee.gate_instances;
