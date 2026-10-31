-- +goose Up
CREATE SCHEMA IF NOT EXISTS planner;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'staffy_app') THEN
    CREATE ROLE staffy_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
  END IF;
END
$$;

-- The turn ledger of the planner (Story 8-1): the append-only business
-- record of every STARTED turn (AD-10) — validated, repaired or failed.
-- A refused context never reaches this table (the refusal of the input
-- happens before any model round and before any record). The record
-- carries the ids of the context, the outcome word, the named refusal
-- cause (the final cause of a failed turn; the repaired cause of a
-- repaired turn), the approved fallback form that ended a failed turn,
-- the proposal schema version, the machine Build constant, the
-- validated proposal as compact JSONB (business material behind the
-- tenant RLS — the reply-string spirit of Story 2.12, NEVER telemetry;
-- the AR-18 telemetry binding without PII arrives with Story 8.6) and
-- the creation moment. The uniqueness (tenant_id, turn_id) makes the
-- second recording of the same turn impossible — the honest conflict.
-- A record is NEVER updated: corrections do not exist (a new turn is a
-- new record), which the grants enforce with the runtime role holding
-- SELECT and INSERT only.
CREATE TABLE planner.turns (
 tenant_id uuid NOT NULL,
 turn_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 work_item_id uuid,
 employee_id uuid NOT NULL,
 outcome text NOT NULL CHECK (outcome IN ('validated','repaired','failed')),
 refusal_reason text,
 fallback_form text CHECK (fallback_form IN ('honest_stop','handoff','callback')),
 schema_version integer NOT NULL CHECK (schema_version > 0),
 build text NOT NULL,
 proposal jsonb,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, turn_id)
);

ALTER TABLE planner.turns ENABLE ROW LEVEL SECURITY;
ALTER TABLE planner.turns FORCE ROW LEVEL SECURITY;

CREATE POLICY turn_scope ON planner.turns USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true),'')::uuid);

GRANT USAGE ON SCHEMA planner TO staffy_app;
GRANT SELECT, INSERT ON planner.turns TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON planner.turns FROM staffy_app;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM planner.turns) THEN
  RAISE EXCEPTION 'turn history exists; export and retain it before rollback';
 END IF;
END $$;
-- +goose StatementEnd
DROP TABLE IF EXISTS planner.turns;
DROP SCHEMA IF EXISTS planner;
