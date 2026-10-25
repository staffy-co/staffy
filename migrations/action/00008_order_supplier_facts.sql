-- +goose Up
-- Story 4.9 (step-01): the Supplier Order Fact Receipt — the EIGHTH
-- migration of the action module: ONE append-only table of the human
-- facts of the store contour {formed, sent} recorded by a manager over
-- the COMMITTED operation of create_abcp_customer_order. The discipline
-- is the 00003/00005/00006/00007 precedent: tenant-scoped with RLS
-- ENABLE+FORCE, the append-only surface is INSERT/SELECT by grants (the
-- UPDATE/DELETE probes of the step-04 integration matrix pin the absence
-- of the paths), the CHECK dictionaries are the loyal backstops of the
-- frozen domain words — never a tenant configuration.
--
-- The fact is the HUMAN RECEIPT of the store contour (OQ-3 safe state:
-- manual action + human record): the manager forms the order in ABCP by
-- hand and records WHAT HAPPENED — the supplier order ID, the kind word
-- and TWO times: happened_at (the named time of the manager — the
-- provider-observed precedent of 2.7) and recorded_at (the server time
-- of the append — never mixed). NO supplier-send path exists here or
-- anywhere: the table records the reality, it never performs it (the
-- OQ-3 fence — passing OQ-3/OQ-5 does not authorize supplier-send; the
-- kind words are the human claims, not commands).
--
-- The UNIQUE full fact key (tenant, case, operation, kind,
-- supplier_order_id) is the IDEMPOTENCY BY KEY of the repeated record
-- command (the AC-9 «webhook repeat» discipline WITHOUT a webhook — no
-- integrations exist, AG-4): the replay of the same command is the same
-- saved fact, never a second row. A formed fact of ANOTHER supplier
-- order ID is a SECOND fact — the multiple suppliers live by name. The
-- sent-after-formed ladder is the APP guard (the honest word refusal);
-- the table stores the history whichever way it was recorded — the truth
-- is recorded as it is, never guarded against a «check bypass» (AC-12
-- spirit: the panel shows both facts, it never hides or blocks them).
--
-- The fact_no is the per-case monotonic ladder under the advisory lock
-- (the 00006/00007 precedent — the server truth of the fold order); the
-- work item anchor and the actor identity pin the origin; the operation
-- FK is the module-own anchor of the order contour.
CREATE TABLE action.order_supplier_facts (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 fact_no bigint NOT NULL CHECK (fact_no >= 1),
 operation_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('formed', 'sent')),
 supplier_order_id text NOT NULL CHECK (char_length(supplier_order_id) BETWEEN 1 AND 200),
 happened_at timestamptz NOT NULL,
 actor_id uuid NOT NULL,
 recorded_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id, fact_no),
 FOREIGN KEY (tenant_id, operation_id) REFERENCES action.operations(tenant_id, operation_id)
);
-- The idempotency BY KEY (AC-9): the repeated record command of the same
-- fact key replays the saved outcome — the second row NEVER lands, the
-- UNIQUE backstop holds even under a lost race.
CREATE UNIQUE INDEX order_supplier_fact_key ON action.order_supplier_facts (tenant_id, case_id, operation_id, kind, supplier_order_id);
CREATE INDEX order_supplier_facts_operation ON action.order_supplier_facts (tenant_id, operation_id, fact_no);

-- Row level security: the table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate — the manager
-- reads and writes ONLY under TenantContext).
ALTER TABLE action.order_supplier_facts ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.order_supplier_facts FORCE ROW LEVEL SECURITY;

CREATE POLICY order_supplier_facts_scope ON action.order_supplier_facts USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

-- Grants: the append-only human-receipt surface (INSERT/SELECT); the
-- facts are never edited — a change is a new fact of the new reality.
-- The projection (Order Status) writes NOTHING — it derives read-time.
GRANT SELECT, INSERT ON action.order_supplier_facts TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.order_supplier_facts FROM staffy_app;

-- +goose Down
DROP TABLE action.order_supplier_facts;
