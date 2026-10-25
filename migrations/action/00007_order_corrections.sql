-- +goose Up
-- Story 4.8 (step-01): the order correction contour — the Correction
-- Work Item over the LAST CLOSED blocked decision of the manager order
-- verification (4.7). The SEVENTH migration of the action module: two
-- append-only tables born together with the four lifecycle commands
-- (open / reassign / reread / cease). The discipline is the 00003/00005/
-- 00006 precedent: every table is tenant-scoped with RLS ENABLE+FORCE,
-- the append-only surfaces are INSERT/SELECT by grants (the UPDATE/
-- DELETE probes of the step-04 integration matrix pin the absence of
-- the paths), the CHECK dictionaries are the loyal backstops of the
-- frozen domain words — never a tenant configuration.
--
-- The correction is an OPERATIONAL lifecycle record anchored to the
-- COMMITTED operation of create_abcp_customer_order WITH the LAST
-- CLOSED blocked verification task of that operation (the anchor
-- discipline of the spec — NOT the 2-13 turn_ref, NOT the 2-15 case
-- fact correction: the correction lives over the ORDER operation, the
-- case ownership NEVER moves, the 2-4 terms stay untouched, the 2-15
-- commands stay untouched — the two contours differ BY NAME). The row
-- pins the operation (the module-own FK), the blocked verification
-- number (the decision the correction addresses), the receipt identity
-- pins (the committed receipt version, the external ABCP Order ID of
-- the receipt references — the readback material of the screen) and the
-- creator. The correction number is the per-case monotonic ladder (the
-- repeated correction of one operation after a NEW blocked decision is
-- a NEW correction — the history is never rewritten; the ONE open
-- correction per operation is the app guard under the advisory lock,
-- the honest 409 — the 00006 precedent: the append-only row carries no
-- mutable state column, so the open-uniqueness is the serialized
-- command truth, never an UPDATE).
--
-- The lifecycle events are append-only rows of one correction: one
-- event — one kind; the numbers are the per-correction monotonic
-- sequence derived under the advisory lock; the UNIQUE backstop catches
-- the races the locks already serialize. The material is per-kind (the
-- CHECK below, the handoff_event_material precedent of 00013/00006):
-- opened names the owner and the correction deadline word with its
-- time AND carries the blocked positions with the structured reason
-- word BY NAME (the one-transaction SQL-truth carry of the open command
-- — the original blocked event is READ, never rewritten); reassigned
-- names the NEW owner and the NEW deadline word with its time (a
-- reassignment is a NEW event with a NEW epoch — the event_no of the
-- latest opened-or-reassigned event IS the assignment epoch, the CAS
-- token of every later command); reread carries the reread external
-- ABCP object ID and the pinned identity triple of the NEW confirmed
-- version {proposal, version, confirmation} (the manager's words —
-- Staffy never reads ABCP, AG-4; the closure validations belong to the
-- app layer — the LIVE re-derivation fence of the same code as the
-- ladder 4.5); ceased carries the cessation reason word of the fixed
-- three-word dictionary. The partial UNIQUE index pins the structural
-- one-closure truth: at most ONE closure event (reread OR ceased) per
-- correction — the second closure of one correction is impossible even
-- under a lost race. The AUTO-CLOSURE kind does not exist — the expiry
-- is the read-time overdue word (the honest wait; the automatic
-- decisions do not exist STRUCTURALLY: no event kind can express them).
CREATE TABLE action.order_corrections (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 correction_no bigint NOT NULL CHECK (correction_no >= 1),
 operation_id uuid NOT NULL,
 verification_no bigint NOT NULL CHECK (verification_no >= 1),
 receipt_version_no bigint NOT NULL CHECK (receipt_version_no >= 1),
 external_order_id text NOT NULL CHECK (char_length(external_order_id) BETWEEN 1 AND 200),
 creator_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id, correction_no),
 FOREIGN KEY (tenant_id, operation_id) REFERENCES action.operations(tenant_id, operation_id)
);
CREATE INDEX order_corrections_operation ON action.order_corrections (tenant_id, operation_id, correction_no);
CREATE INDEX order_corrections_case ON action.order_corrections (tenant_id, case_id, created_at);

-- The append-only lifecycle events of one correction (the monotonic
-- event number is the per-correction server truth AND the assignment
-- epoch of the latest opened-or-reassigned round — the CAS expectation
-- of the reread/cease commands; the history is never rewritten — the
-- states are the read-time derivation of the domain).
CREATE TABLE action.order_correction_events (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 correction_no bigint NOT NULL CHECK (correction_no >= 1),
 event_no bigint NOT NULL CHECK (event_no >= 1),
 kind text NOT NULL CHECK (kind IN ('opened', 'reassigned', 'reread', 'ceased')),
 owner uuid,
 correct_deadline text CHECK (correct_deadline IS NULL OR correct_deadline IN ('correct_4h', 'correct_1d')),
 deadline_at timestamptz,
 positions jsonb CHECK (positions IS NULL OR (jsonb_typeof(positions) = 'array' AND jsonb_array_length(positions) BETWEEN 1 AND 8)),
 reason text CHECK (reason IS NULL OR reason IN ('wrong_part', 'wrong_vehicle', 'price_changed', 'availability_changed', 'confirmation_issue')),
 external_object_id text CHECK (external_object_id IS NULL OR char_length(external_object_id) BETWEEN 1 AND 200),
 proposal_id uuid,
 version_no bigint CHECK (version_no IS NULL OR version_no >= 1),
 confirmation_id uuid,
 cessation_reason text CHECK (cessation_reason IS NULL OR cessation_reason IN ('client_refused', 'not_needed', 'abandoned')),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id, correction_no, event_no),
 FOREIGN KEY (tenant_id, case_id, correction_no) REFERENCES action.order_corrections(tenant_id, case_id, correction_no),
 CONSTRAINT order_correction_event_material CHECK (
   (kind = 'opened' AND owner IS NOT NULL AND correct_deadline IS NOT NULL AND deadline_at IS NOT NULL
                        AND positions IS NOT NULL AND reason IS NOT NULL
                        AND external_object_id IS NULL AND proposal_id IS NULL AND version_no IS NULL
                        AND confirmation_id IS NULL AND cessation_reason IS NULL)
   OR (kind = 'reassigned' AND owner IS NOT NULL AND correct_deadline IS NOT NULL AND deadline_at IS NOT NULL
                            AND positions IS NULL AND reason IS NULL
                            AND external_object_id IS NULL AND proposal_id IS NULL AND version_no IS NULL
                            AND confirmation_id IS NULL AND cessation_reason IS NULL)
   OR (kind = 'reread' AND external_object_id IS NOT NULL AND proposal_id IS NOT NULL
                        AND version_no IS NOT NULL AND confirmation_id IS NOT NULL
                        AND owner IS NULL AND correct_deadline IS NULL AND deadline_at IS NULL
                        AND positions IS NULL AND reason IS NULL AND cessation_reason IS NULL)
   OR (kind = 'ceased' AND cessation_reason IS NOT NULL
                        AND owner IS NULL AND correct_deadline IS NULL AND deadline_at IS NULL
                        AND positions IS NULL AND reason IS NULL
                        AND external_object_id IS NULL AND proposal_id IS NULL AND version_no IS NULL
                        AND confirmation_id IS NULL))
);
CREATE INDEX order_correction_events_task ON action.order_correction_events (tenant_id, case_id, correction_no, event_no);
-- The structural one-closure backstop: at most ONE closure event (the
-- reread OR the ceased) per correction row — the CAS epoch guard
-- serializes the races, this index backstops the invariant the locks
-- already hold (a correction never closes twice; a NEW correction is a
-- NEW row by the ladder).
CREATE UNIQUE INDEX order_correction_one_closure ON action.order_correction_events (tenant_id, case_id, correction_no) WHERE kind IN ('reread', 'ceased');

-- Row level security: every table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate — the manager
-- reads ONLY under TenantContext).
ALTER TABLE action.order_corrections ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.order_corrections FORCE ROW LEVEL SECURITY;
ALTER TABLE action.order_correction_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.order_correction_events FORCE ROW LEVEL SECURITY;

CREATE POLICY order_corrections_scope ON action.order_corrections USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY order_correction_events_scope ON action.order_correction_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

-- Grants: the append-only operational lifecycle surfaces (INSERT/
-- SELECT); the corrections and the events are never edited — a change
-- is a new event, a new correction. The history is the truth.
GRANT SELECT, INSERT ON action.order_corrections TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.order_corrections FROM staffy_app;
GRANT SELECT, INSERT ON action.order_correction_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.order_correction_events FROM staffy_app;

-- +goose Down
DROP TABLE action.order_correction_events;
DROP TABLE action.order_corrections;
