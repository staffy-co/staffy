-- +goose Up
-- Story 3.6 (step-02): the execution contour — dispatch leases, dispatch
-- outcomes and the Action Receipts. The THIRD migration of the action
-- module: three new append-only tables born together with the write-ahead
-- lease transaction (AcquireDispatchLease) and the fenced outcome
-- transaction (RecordDispatchOutcome). The discipline is the
-- 00002/00017 precedent: every table is tenant-scoped with RLS
-- ENABLE+FORCE, the append-only surfaces are INSERT/SELECT by grants
-- (the UPDATE/DELETE probes of the step-04 integration matrix pin the
-- absence of the paths), the CHECK dictionaries are the loyal backstops
-- of the frozen domain words — never a tenant configuration.
--
-- The lease is the WRITE-AHEAD right to send (persist-before-emit AD-6):
-- the row + the ledger submitted entry land in ONE transaction BEFORE
-- the network call; attempt_no 1..3 is the monotonic fencing token, the
-- 64-hex lease_token identifies the holder at the outcome recording. The
-- raw payload NEVER enters (AD-10/AC-5): the lease carries the keyed-HMAC
-- request digest and the keyed external idempotency key alone — the
-- worker rebuilds the material deterministically from the immutable
-- rows.
--
-- The outcome row audits the ADAPTER'S honest claim word (the four-word
-- dictionary); a refused or absent claim (a committed claim without the
-- structural evidence, a call error, a foreign word) records NO outcome
-- row — the lease, the ledger kind and the receipt speak the guarded
-- truth, and the recorded guarantee stays false (the guarantee word is
-- recorded ONLY for the honest timeout claim — the LIVE declaration
-- read at the recording moment).
--
-- The Action Receipt is append-only and versioned: v1 lands on the FIRST
-- deciding outcome {committed, failed, unknown} (accepted carries no
-- receipt — the business result is not confirmed yet); the later
-- evidence of the future 3-7 contour appends v2+ through the same
-- table — v1 is never rewritten. The references jsonb is the SAFE pin
-- set only (ids, words, the external object identity — never a material
-- value, never a secret; FR-31: the decision is reconstructible without
-- the dialog, the sensitive material is never duplicated).
--
-- The dictionary extensions of 00002 ride here as ALTER ... DROP/ADD
-- CONSTRAINT (the data is never rewritten — the vocabulary widens):
-- the operation ledger CHECK grows from one word to the seven-word
-- dictionary of the frozen domain (dispatch_blocked is NOT
-- state-bearing), the outbox event_type CHECK grows from one word to
-- the two-word dictionary (operation.queued + receipt.created — the
-- AD-2 discipline: the Work Item/Conversation link is an event of THIS
-- module's own outbox, the future consumer reads the pending records).
CREATE TABLE action.dispatch_leases (
 tenant_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 attempt_no bigint NOT NULL CHECK (attempt_no BETWEEN 1 AND 3),
 lease_token text NOT NULL CHECK (lease_token ~ '^[0-9a-f]{64}$'),
 definition_id text NOT NULL CHECK (char_length(definition_id) BETWEEN 1 AND 200),
 definition_version text NOT NULL CHECK (char_length(definition_version) BETWEEN 1 AND 32),
 external_idempotency_key text NOT NULL CHECK (char_length(external_idempotency_key) = 64),
 request_digest text NOT NULL CHECK (char_length(request_digest) = 64),
 lease_expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, operation_id, attempt_no)
);
CREATE INDEX dispatch_leases_operation ON action.dispatch_leases (tenant_id, operation_id, attempt_no);

-- The outcome row: the four-word adapter claim dictionary, the
-- correlation and the external object identity triple of the structural
-- evidence (NULLable — committed survives ONLY with the complete triple,
-- the guard refuses the claim otherwise), the recorded guarantee flag
-- and the safe details (identifiers and words only — never a material
-- value; the 500 bound is the store backstop of the domain truncation).
CREATE TABLE action.dispatch_outcomes (
 tenant_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 attempt_no bigint NOT NULL CHECK (attempt_no BETWEEN 1 AND 3),
 lease_token text NOT NULL CHECK (lease_token ~ '^[0-9a-f]{64}$'),
 outcome text NOT NULL CHECK (outcome IN ('accepted', 'committed', 'failed', 'timeout')),
 correlation_id text NOT NULL DEFAULT '' CHECK (char_length(correlation_id) <= 128),
 external_object_id text NOT NULL DEFAULT '' CHECK (char_length(external_object_id) <= 200),
 external_object_version text NOT NULL DEFAULT '' CHECK (char_length(external_object_version) <= 64),
 external_object_time timestamptz,
 guarantee_recorded boolean NOT NULL DEFAULT false,
 details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 500),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, operation_id, attempt_no)
);
CREATE INDEX dispatch_outcomes_operation ON action.dispatch_outcomes (tenant_id, operation_id, attempt_no);

-- The Action Receipt: versioned append-only, v1 on the first deciding
-- outcome, UNIQUE versions — v1 is byte-stable forever (the later
-- evidence of 3-7 appends v2+; the append-only probes of the
-- integration matrix pin the absence of the UPDATE/DELETE paths).
-- The SAFE pin column is receipt_references (NEVER references: the
-- SQL-reserved word is a syntax error at the live boundary — the
-- step-04 live-PG finding of the honest naming).
CREATE TABLE action.action_receipts (
 tenant_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 receipt_version_no bigint NOT NULL CHECK (receipt_version_no >= 1),
 outcome text NOT NULL CHECK (outcome IN ('committed', 'failed', 'unknown')),
 receipt_references jsonb NOT NULL CHECK (jsonb_typeof(receipt_references) = 'object' AND octet_length(receipt_references::text) <= 4096),
 details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 500),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, operation_id, receipt_version_no)
);
CREATE INDEX action_receipts_operation ON action.action_receipts (tenant_id, operation_id, receipt_version_no);

-- The dictionary extensions of the 00002 tables (ALTER — the data is
-- never rewritten, the vocabulary widens): the seven-word operation
-- ledger dictionary and the two-word outbox event dictionary.
ALTER TABLE action.operation_ledger DROP CONSTRAINT operation_ledger_kind_check;
ALTER TABLE action.operation_ledger ADD CONSTRAINT operation_ledger_kind_check CHECK (kind IN ('authorized', 'submitted', 'accepted', 'committed', 'failed', 'unknown', 'dispatch_blocked'));
ALTER TABLE action.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE action.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('operation.queued', 'receipt.created'));

-- Row level security: every table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate — the worker
-- reads ONLY under TenantContext, AC-1).
ALTER TABLE action.dispatch_leases ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.dispatch_leases FORCE ROW LEVEL SECURITY;
ALTER TABLE action.dispatch_outcomes ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.dispatch_outcomes FORCE ROW LEVEL SECURITY;
ALTER TABLE action.action_receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.action_receipts FORCE ROW LEVEL SECURITY;

CREATE POLICY dispatch_leases_scope ON action.dispatch_leases USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY dispatch_outcomes_scope ON action.dispatch_outcomes USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY action_receipts_scope ON action.action_receipts USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

-- Grants: the append-only surface (INSERT/SELECT only) — the leases, the
-- outcomes and the receipts are never edited or deleted through the
-- application role (AC-12: v1 is never rewritten).
GRANT USAGE ON SCHEMA action TO staffy_app;
GRANT SELECT, INSERT ON action.dispatch_leases TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.dispatch_leases FROM staffy_app;
GRANT SELECT, INSERT ON action.dispatch_outcomes TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.dispatch_outcomes FROM staffy_app;
GRANT SELECT, INSERT ON action.action_receipts TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.action_receipts FROM staffy_app;

-- +goose Down
DROP SCHEMA action CASCADE;
