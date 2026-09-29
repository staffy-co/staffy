-- +goose Up
CREATE TABLE employee.voice_previews (
 tenant_id uuid NOT NULL, employee_id uuid NOT NULL, preview_id uuid NOT NULL,
 voice_version bigint NOT NULL, dictionary_version bigint NOT NULL, draft_version bigint NOT NULL,
 source_text text NOT NULL CHECK (char_length(source_text) BETWEEN 1 AND 300),
 synthesis_text text NOT NULL CHECK (char_length(synthesis_text) BETWEEN 1 AND 500),
 route text NOT NULL CHECK (route = 'offline-test-tts'), slow boolean NOT NULL, actor_id uuid NOT NULL, created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id,preview_id), UNIQUE (tenant_id,employee_id,preview_id),
 FOREIGN KEY (tenant_id,employee_id,voice_version) REFERENCES employee.voice_versions(tenant_id,employee_id,version),
 CHECK (dictionary_version = voice_version)
);
CREATE INDEX voice_previews_employee_idx ON employee.voice_previews(tenant_id,employee_id,created_at DESC);
ALTER TABLE employee.voice_previews ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.voice_previews FORCE ROW LEVEL SECURITY;
CREATE POLICY voice_preview_scope ON employee.voice_previews USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT ON employee.voice_previews TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.voice_previews FROM staffy_app;

CREATE TABLE employee.voice_assessments (
 tenant_id uuid NOT NULL, employee_id uuid NOT NULL, assessment_id uuid NOT NULL, preview_id uuid NOT NULL,
 verdict text NOT NULL CHECK (verdict IN ('approved','needs_revision')),
 actor_id uuid NOT NULL, assessed_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id,assessment_id), UNIQUE (tenant_id,preview_id),
 FOREIGN KEY (tenant_id,employee_id,preview_id) REFERENCES employee.voice_previews(tenant_id,employee_id,preview_id),
 FOREIGN KEY (tenant_id,employee_id) REFERENCES employee.drafts(tenant_id,employee_id)
);
ALTER TABLE employee.voice_assessments ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.voice_assessments FORCE ROW LEVEL SECURITY;
CREATE POLICY voice_assessment_scope ON employee.voice_assessments USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
GRANT SELECT, INSERT ON employee.voice_assessments TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.voice_assessments FROM staffy_app;
ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned','behavior_saved','voice_saved','voice_assessed'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM employee.voice_assessments) OR EXISTS (SELECT 1 FROM employee.voice_previews) THEN
  RAISE EXCEPTION 'voice preview or assessment history exists; export and retain it before rollback';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned','behavior_saved','voice_saved'));
DROP TABLE employee.voice_assessments;
DROP TABLE employee.voice_previews;
