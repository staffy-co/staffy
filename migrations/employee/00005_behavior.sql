-- +goose Up
CREATE TABLE employee.behavior_versions (
 tenant_id uuid NOT NULL, employee_id uuid NOT NULL, version bigint NOT NULL CHECK (version > 0),
 draft_version bigint NOT NULL CHECK (draft_version > 0), pack_id text NOT NULL, pack_version text NOT NULL,
 config jsonb NOT NULL, actor_id uuid NOT NULL, changed_at timestamptz NOT NULL,
 change_summary text NOT NULL CHECK (char_length(change_summary) BETWEEN 1 AND 300),
 PRIMARY KEY (tenant_id, employee_id, version),
 FOREIGN KEY (tenant_id, employee_id) REFERENCES employee.drafts(tenant_id, employee_id)
);
ALTER TABLE employee.behavior_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee.behavior_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY behavior_scope ON employee.behavior_versions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);
GRANT SELECT, INSERT ON employee.behavior_versions TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON employee.behavior_versions FROM staffy_app;
ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned','behavior_saved'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM employee.behavior_versions) THEN
  RAISE EXCEPTION 'behavior history exists; export and retain it before rollback';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned'));
DROP TABLE employee.behavior_versions;
