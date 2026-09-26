-- +goose Up
ALTER TABLE employee.drafts ADD COLUMN domain_pack jsonb, ADD COLUMN review_slots jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict','pack_pinned'));

-- +goose Down
ALTER TABLE employee.audit DROP CONSTRAINT audit_outcome_check;
ALTER TABLE employee.audit ADD CONSTRAINT audit_outcome_check CHECK (outcome IN ('created','saved','conflict'));
ALTER TABLE employee.drafts DROP COLUMN review_slots, DROP COLUMN domain_pack;
