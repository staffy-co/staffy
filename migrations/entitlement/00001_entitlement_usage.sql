-- +goose Up
-- Story 1.11: entitlement, limits and usage transparency. Every table is
-- append-only: staffy_app gets INSERT/SELECT only, no UPDATE, no DELETE, no
-- TRUNCATE. Current states (terms version, budget, entitlement state,
-- admission state, meter aggregates) are derived at read time; there is no
-- second writer. Usage events are their own idempotency records: UNIQUE
-- (tenant, event_key) plus the payload hash decide replay or conflict.

CREATE SCHEMA IF NOT EXISTS entitlement;

CREATE TABLE entitlement.terms_versions (
 tenant_id uuid NOT NULL,
 version bigint NOT NULL CHECK (version > 0),
 currency text NOT NULL CHECK (currency = '' OR currency ~ '^[A-Z]{3}$'),
 tax_regime text NOT NULL CHECK (char_length(tax_regime) <= 300),
 period_start date,
 period_end date,
 included_employees jsonb NOT NULL CHECK (jsonb_typeof(included_employees) = 'array'),
 included_channels jsonb NOT NULL CHECK (jsonb_typeof(included_channels) = 'array'),
 included_connectors jsonb NOT NULL CHECK (jsonb_typeof(included_connectors) = 'array'),
 limits jsonb NOT NULL CHECK (jsonb_typeof(limits) = 'array'),
 launch_price numeric(14,2) CHECK (launch_price IS NULL OR launch_price >= 0),
 subscription_price numeric(14,2) CHECK (subscription_price IS NULL OR subscription_price >= 0),
 change_note text NOT NULL CHECK (char_length(change_note) <= 600),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, version)
);

CREATE TABLE entitlement.state_events (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 event text NOT NULL CHECK (event IN ('activate','pause','resume','end')),
 terms_version bigint NOT NULL CHECK (terms_version > 0),
 basis text NOT NULL CHECK (char_length(basis) BETWEEN 1 AND 600),
 limits_check text CHECK (limits_check IS NULL OR limits_check IN ('ok','exceeded','no_budget')),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id)
);

CREATE TABLE entitlement.usage_events (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 event_key text NOT NULL CHECK (char_length(event_key) BETWEEN 8 AND 200),
 payload_hash char(64) NOT NULL,
 source text NOT NULL CHECK (char_length(source) BETWEEN 1 AND 100),
 category text NOT NULL CHECK (category IN ('carrier','audio','model_tokens','tools','connector_charges','infrastructure','manual_minutes')),
 quantity numeric(18,4) NOT NULL CHECK (quantity > 0),
 unit text NOT NULL CHECK (char_length(unit) BETWEEN 1 AND 40),
 amount numeric(14,2) CHECK (amount IS NULL OR amount >= 0),
 currency text NOT NULL CHECK (currency = '' OR currency ~ '^[A-Z]{3}$'),
 is_estimate boolean NOT NULL,
 period_start date NOT NULL,
 period_end date NOT NULL,
 occurred_at timestamptz NOT NULL,
 ingested_at timestamptz NOT NULL,
 employee_id text CHECK (employee_id IS NULL OR char_length(employee_id) <= 100),
 channel text CHECK (channel IS NULL OR char_length(channel) <= 100),
 release_version text CHECK (release_version IS NULL OR char_length(release_version) <= 100),
 provenance text NOT NULL CHECK (char_length(provenance) <= 600),
 agg_rule_version text NOT NULL,
 actor_id uuid NOT NULL,
 PRIMARY KEY (tenant_id, event_id),
 UNIQUE (tenant_id, event_key),
 CHECK (period_end >= period_start)
);

CREATE TABLE entitlement.usage_sources (
 tenant_id uuid NOT NULL,
 registration_id uuid NOT NULL,
 name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
 categories jsonb NOT NULL CHECK (jsonb_typeof(categories) = 'array'),
 freshness_minutes integer NOT NULL CHECK (freshness_minutes BETWEEN 1 AND 100000),
 retired boolean NOT NULL DEFAULT false,
 note text NOT NULL CHECK (char_length(note) <= 600),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, registration_id)
);

CREATE TABLE entitlement.budget_versions (
 tenant_id uuid NOT NULL,
 version bigint NOT NULL CHECK (version > 0),
 amount numeric(14,2) NOT NULL CHECK (amount > 0),
 currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
 period_start date NOT NULL,
 period_end date NOT NULL,
 warning_threshold_percent integer NOT NULL CHECK (warning_threshold_percent BETWEEN 1 AND 100),
 hard_limit numeric(14,2) CHECK (hard_limit IS NULL OR hard_limit > 0),
 limit_policy text NOT NULL CHECK (limit_policy IN ('block_new_admissions','degraded','notify_only')),
 anomaly_thresholds jsonb NOT NULL CHECK (jsonb_typeof(anomaly_thresholds) = 'array'),
 note text NOT NULL CHECK (char_length(note) <= 600),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, version),
 CHECK (period_end >= period_start)
);

CREATE TABLE entitlement.usage_exceptions (
 tenant_id uuid NOT NULL,
 exception_id uuid NOT NULL,
 dedupe_key text NOT NULL CHECK (char_length(dedupe_key) BETWEEN 8 AND 300),
 cause text NOT NULL CHECK (cause IN ('budget_threshold','hard_limit_reached','data_gap','anomaly_tool_calls','anomaly_retry_storm','anomaly_manual_minutes','manual_entry')),
 message text NOT NULL CHECK (char_length(message) BETWEEN 1 AND 1000),
 data_status text NOT NULL CHECK (data_status IN ('complete','partial','stale','unknown')),
 affected_employee text CHECK (affected_employee IS NULL OR char_length(affected_employee) <= 100),
 affected_route text CHECK (affected_route IS NULL OR char_length(affected_route) <= 100),
 recommended_action text NOT NULL CHECK (char_length(recommended_action) BETWEEN 1 AND 600),
 agg_rule_version text NOT NULL,
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, exception_id),
 UNIQUE (tenant_id, dedupe_key)
);

CREATE TABLE entitlement.exception_acknowledgements (
 tenant_id uuid NOT NULL,
 ack_id uuid NOT NULL,
 exception_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 note text NOT NULL CHECK (char_length(note) <= 600),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, ack_id),
 FOREIGN KEY (tenant_id, exception_id) REFERENCES entitlement.usage_exceptions(tenant_id, exception_id)
);

CREATE INDEX terms_current_idx ON entitlement.terms_versions (tenant_id, version DESC);
CREATE INDEX state_events_idx ON entitlement.state_events (tenant_id, created_at DESC);
CREATE INDEX usage_events_period_idx ON entitlement.usage_events (tenant_id, category, period_start, period_end);
CREATE INDEX usage_sources_idx ON entitlement.usage_sources (tenant_id, name, created_at DESC);
CREATE INDEX budget_current_idx ON entitlement.budget_versions (tenant_id, version DESC);
CREATE INDEX usage_exceptions_idx ON entitlement.usage_exceptions (tenant_id, created_at DESC);
CREATE INDEX exception_acknowledgements_idx ON entitlement.exception_acknowledgements (tenant_id, exception_id, created_at DESC);

DO $$
DECLARE t text;
BEGIN
  GRANT USAGE ON SCHEMA entitlement TO staffy_app;
  FOREACH t IN ARRAY ARRAY['terms_versions','state_events','usage_events','usage_sources','budget_versions','usage_exceptions','exception_acknowledgements']
  LOOP
    EXECUTE format('ALTER TABLE entitlement.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE entitlement.%I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY %I ON entitlement.%I USING (tenant_id = NULLIF(current_setting(''staffy.tenant_id'', true),'''')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting(''staffy.tenant_id'', true),'''')::uuid)', t || '_scope', t);
    EXECUTE format('GRANT SELECT, INSERT ON entitlement.%I TO staffy_app', t);
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON entitlement.%I FROM staffy_app', t);
  END LOOP;
END $$;

-- +goose Down
DROP TABLE IF EXISTS entitlement.exception_acknowledgements;
DROP TABLE IF EXISTS entitlement.usage_exceptions;
DROP TABLE IF EXISTS entitlement.budget_versions;
DROP TABLE IF EXISTS entitlement.usage_sources;
DROP TABLE IF EXISTS entitlement.usage_events;
DROP TABLE IF EXISTS entitlement.state_events;
DROP TABLE IF EXISTS entitlement.terms_versions;
DROP SCHEMA IF EXISTS entitlement;
