-- +goose Up
-- Story 3.7 (step-02): the reconciliation contour — the reconciliation
-- leases and the reconciliation outcomes. The FOURTH migration of the
-- action module: two new append-only tables born together with the
-- write-ahead reconciliation lease transaction
-- (AcquireReconciliationLease) and the fenced verdict transaction
-- (RecordReconciliationVerdict). The discipline is the 00003 precedent:
-- every table is tenant-scoped with RLS ENABLE+FORCE, the append-only
-- surfaces are INSERT/SELECT by grants (the UPDATE/DELETE probes of the
-- step-04 integration matrix pin the absence of the paths), the CHECK
-- dictionaries are the loyal backstops of the frozen domain words —
-- never a tenant configuration. The module-internal integrity rides the
-- advisory lock + the fenced attempt pair (the 00003 precedent — no
-- cross-module references, AD-2).
--
-- The reconciliation lease is the WRITE-AHEAD right to query (the
-- persist-before-emit discipline of the read contour: the right is
-- material BEFORE the network query): the row + the ledger reconciling
-- entry land in ONE transaction; attempt_no 1..3 is the monotonic
-- fencing token, the 64-hex lease_token identifies the holder at the
-- verdict recording. The lease speaks what the query carried: the SAME
-- external idempotency key as the dispatch (dispatch_key:v1: — the
-- provider recognizes the operation), the keyed request digest of the
-- expected material (the raw payload NEVER enters, AD-10/AC-5), the
-- correlation id of the last dispatch attempt, the definition pins and
-- the LIVE reconciliation word of the connector declaration read at the
-- query moment (the declared-absence provenance). The backoff is
-- MATERIAL: next_eligible_at is recorded per attempt (base 30000 ms +
-- jitter <= 5000 ms — the command before the window speaks the honest
-- «not ready until» word).
--
-- The outcome row audits the NORMALIZED four-word verdict (the domain
-- derivation refuses the unjustified claims BEFORE the store: a
-- committed claim without the structural evidence triple records
-- no_answer, a no_effect claim without the declared absence semantics
-- records no_answer — the terminal success is never taken on word). The
-- external object identity triple is NULLable-empty (committed survives
-- ONLY with the complete triple — the guards of the domain are the
-- first line, this CHECK set is the loyal store backstop of the shapes).
CREATE TABLE action.reconciliation_leases (
 tenant_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 attempt_no bigint NOT NULL CHECK (attempt_no BETWEEN 1 AND 3),
 lease_token text NOT NULL CHECK (lease_token ~ '^[0-9a-f]{64}$'),
 definition_id text NOT NULL CHECK (char_length(definition_id) BETWEEN 1 AND 200),
 definition_version text NOT NULL CHECK (char_length(definition_version) BETWEEN 1 AND 64),
 reconciliation_word text NOT NULL CHECK (char_length(reconciliation_word) BETWEEN 1 AND 64),
 external_idempotency_key text NOT NULL CHECK (char_length(external_idempotency_key) = 64),
 request_digest text NOT NULL CHECK (char_length(request_digest) = 64),
 correlation_id text NOT NULL DEFAULT '' CHECK (char_length(correlation_id) <= 128),
 lease_expires_at timestamptz NOT NULL,
 next_eligible_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, operation_id, attempt_no)
);
CREATE INDEX reconciliation_leases_operation ON action.reconciliation_leases (tenant_id, operation_id, attempt_no);

-- The outcome row: the four-word normalized verdict dictionary, the
-- correlation and the external object identity triple of the structural
-- evidence (empty/NULL — committed survives ONLY with the complete
-- triple; the guards of the domain refuse the claim BEFORE the store)
-- and the safe details (identifiers and words only — never a material
-- value; the 500 bound is the store backstop of the domain truncation).
CREATE TABLE action.reconciliation_outcomes (
 tenant_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 attempt_no bigint NOT NULL CHECK (attempt_no BETWEEN 1 AND 3),
 lease_token text NOT NULL CHECK (lease_token ~ '^[0-9a-f]{64}$'),
 verdict text NOT NULL CHECK (verdict IN ('committed', 'no_effect', 'in_progress', 'no_answer')),
 correlation_id text NOT NULL DEFAULT '' CHECK (char_length(correlation_id) <= 128),
 external_object_id text NOT NULL DEFAULT '' CHECK (char_length(external_object_id) <= 200),
 external_object_version text NOT NULL DEFAULT '' CHECK (char_length(external_object_version) <= 64),
 external_object_time timestamptz,
 details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 500),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, operation_id, attempt_no)
);
CREATE INDEX reconciliation_outcomes_operation ON action.reconciliation_outcomes (tenant_id, operation_id, attempt_no);

-- The dictionary extension of the 00003 table (ALTER — the data is never
-- rewritten, the vocabulary widens): the eight-word operation ledger
-- dictionary (reconciling is NOT state-bearing — the operation STAYS
-- unknown while the reconciliation lease holds: the pair {the state
-- unknown, the activity reconciling in the history} of the epic's
-- unknown/reconciling slash). The outbox event dictionary does NOT grow:
-- receipt.created is REUSED for the v2+ versions (the version rides the
-- payload — the two-word event dictionary stays frozen).
ALTER TABLE action.operation_ledger DROP CONSTRAINT operation_ledger_kind_check;
ALTER TABLE action.operation_ledger ADD CONSTRAINT operation_ledger_kind_check CHECK (kind IN ('authorized', 'submitted', 'accepted', 'committed', 'failed', 'unknown', 'dispatch_blocked', 'reconciling'));

-- Row level security: every table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate — the
-- reconciler reads ONLY under TenantContext, AD-11).
ALTER TABLE action.reconciliation_leases ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.reconciliation_leases FORCE ROW LEVEL SECURITY;
ALTER TABLE action.reconciliation_outcomes ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.reconciliation_outcomes FORCE ROW LEVEL SECURITY;

CREATE POLICY reconciliation_leases_scope ON action.reconciliation_leases USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY reconciliation_outcomes_scope ON action.reconciliation_outcomes USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

-- Grants: the append-only surface (INSERT/SELECT only) — the leases and
-- the outcomes are never edited or deleted through the application role
-- (AC-12: the reconciliation history is never rewritten).
GRANT USAGE ON SCHEMA action TO staffy_app;
GRANT SELECT, INSERT ON action.reconciliation_leases TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.reconciliation_leases FROM staffy_app;
GRANT SELECT, INSERT ON action.reconciliation_outcomes TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.reconciliation_outcomes FROM staffy_app;

-- +goose Down
DROP SCHEMA action CASCADE;
