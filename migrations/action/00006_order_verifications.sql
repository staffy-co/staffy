-- +goose Up
-- Story 4.7 (step-01): the manager order verification contour — the
-- Delegated Task over the committed receipt of 4.6. The SIXTH migration
-- of the action module: two append-only tables born together with the
-- five lifecycle commands (assign / acknowledge / approve / block /
-- reassign). The discipline is the 00003/00005 precedent: every table
-- is tenant-scoped with RLS ENABLE+FORCE, the append-only surfaces are
-- INSERT/SELECT by grants (the UPDATE/DELETE probes of the step-04
-- integration matrix pin the absence of the paths), the CHECK
-- dictionaries are the loyal backstops of the frozen domain words —
-- never a tenant configuration.
--
-- The task is an OPERATIONAL lifecycle record anchored to the
-- COMMITTED operation of create_abcp_customer_order with its receipt
-- (the anchor discipline of the spec — NOT the 2-13 turn_ref: the task
-- lives over the order operation, the case ownership NEVER moves, the
-- 2-4 terms stay untouched). The row pins the operation (the module-own
-- FK), the receipt identity (the committed receipt version the
-- verification addresses), the external ABCP Order ID of the receipt
-- references (the readback material of the screen) and the creator.
-- The verification number is the per-case monotonic ladder (the
-- repeated verification of one operation after a decision is a NEW
-- task — the history is never rewritten; the ONE active task per
-- operation is the app guard under the advisory lock, the honest 409).
--
-- The lifecycle events are append-only rows of one task: one event —
-- one kind; the numbers are the per-task monotonic sequence derived
-- under the advisory lock; the UNIQUE backstop catches the races the
-- locks already serialize. The material is per-kind (the CHECK below,
-- the handoff_event_material precedent of 00013): assigned names the
-- assignee, the backup owner and the round's own deadline word with
-- its time (a re-assignment is a NEW event with a NEW epoch — the
-- event_no of the latest assigned event IS the assignment epoch, the
-- CAS token of every later command); acknowledged carries no material
-- (the manager identity and the time are the actor columns); approved
-- carries the material pins of the Human Verification Receipt — the
-- 64-hex order digest of the exact version, the receipt identity, the
-- external ABCP Order ID and the evidence anchors (the SAFE pin set
-- only — identifiers and words, never a material value, never a
-- secret); blocked carries the affected position keys and the reason
-- word of the fixed five-word dictionary. The OVERDUE state is NEVER a
-- row — it is the read-time derivation of the open round after the
-- deadline (the honest wait; the automatic approval does not exist
-- STRUCTURALLY: no event kind can express it).
CREATE TABLE action.order_verifications (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 verification_no bigint NOT NULL CHECK (verification_no >= 1),
 operation_id uuid NOT NULL,
 receipt_version_no bigint NOT NULL CHECK (receipt_version_no >= 1),
 external_order_id text NOT NULL CHECK (char_length(external_order_id) BETWEEN 1 AND 200),
 creator_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id, verification_no),
 FOREIGN KEY (tenant_id, operation_id) REFERENCES action.operations(tenant_id, operation_id)
);
CREATE INDEX order_verifications_operation ON action.order_verifications (tenant_id, operation_id, verification_no);
CREATE INDEX order_verifications_case ON action.order_verifications (tenant_id, case_id, created_at);

-- The append-only lifecycle events of one task (the monotonic event
-- number is the per-task server truth AND the assignment epoch of the
-- latest assigned round — the CAS expectation of the acknowledge/
-- approve/block commands; the history is never rewritten — the states
-- are the read-time derivation of the domain).
CREATE TABLE action.order_verification_events (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 verification_no bigint NOT NULL CHECK (verification_no >= 1),
 event_no bigint NOT NULL CHECK (event_no >= 1),
 kind text NOT NULL CHECK (kind IN ('assigned', 'acknowledged', 'approved', 'blocked')),
 assignee uuid,
 backup_owner uuid,
 verify_deadline text CHECK (verify_deadline IS NULL OR verify_deadline IN ('verify_4h', 'verify_1d')),
 deadline_at timestamptz,
 order_digest text CHECK (order_digest IS NULL OR order_digest ~ '^[0-9a-f]{64}$'),
 receipt_version_no bigint CHECK (receipt_version_no IS NULL OR receipt_version_no >= 1),
 external_order_id text CHECK (external_order_id IS NULL OR char_length(external_order_id) BETWEEN 1 AND 200),
 evidence_anchors jsonb CHECK (evidence_anchors IS NULL OR (jsonb_typeof(evidence_anchors) = 'array' AND jsonb_array_length(evidence_anchors) <= 16)),
 positions jsonb CHECK (positions IS NULL OR (jsonb_typeof(positions) = 'array' AND jsonb_array_length(positions) BETWEEN 1 AND 8)),
 reason text CHECK (reason IS NULL OR reason IN ('wrong_part', 'wrong_vehicle', 'price_changed', 'availability_changed', 'confirmation_issue')),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id, verification_no, event_no),
 FOREIGN KEY (tenant_id, case_id, verification_no) REFERENCES action.order_verifications(tenant_id, case_id, verification_no),
 CONSTRAINT order_verification_event_material CHECK (
   (kind = 'assigned' AND assignee IS NOT NULL AND backup_owner IS NOT NULL
                           AND verify_deadline IS NOT NULL AND deadline_at IS NOT NULL
                           AND order_digest IS NULL AND receipt_version_no IS NULL
                           AND external_order_id IS NULL AND evidence_anchors IS NULL
                           AND positions IS NULL AND reason IS NULL)
   OR (kind = 'acknowledged' AND assignee IS NULL AND backup_owner IS NULL
                               AND verify_deadline IS NULL AND deadline_at IS NULL
                               AND order_digest IS NULL AND receipt_version_no IS NULL
                               AND external_order_id IS NULL AND evidence_anchors IS NULL
                               AND positions IS NULL AND reason IS NULL)
   OR (kind = 'approved' AND order_digest IS NOT NULL AND receipt_version_no IS NOT NULL
                          AND external_order_id IS NOT NULL AND evidence_anchors IS NOT NULL
                          AND assignee IS NULL AND backup_owner IS NULL
                          AND verify_deadline IS NULL AND deadline_at IS NULL
                          AND positions IS NULL AND reason IS NULL)
   OR (kind = 'blocked' AND positions IS NOT NULL AND reason IS NOT NULL
                         AND assignee IS NULL AND backup_owner IS NULL
                         AND verify_deadline IS NULL AND deadline_at IS NULL
                         AND order_digest IS NULL AND receipt_version_no IS NULL
                         AND external_order_id IS NULL AND evidence_anchors IS NULL))
);
CREATE INDEX order_verification_events_task ON action.order_verification_events (tenant_id, case_id, verification_no, event_no);

-- Row level security: every table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate — the manager
-- reads ONLY under TenantContext).
ALTER TABLE action.order_verifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.order_verifications FORCE ROW LEVEL SECURITY;
ALTER TABLE action.order_verification_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.order_verification_events FORCE ROW LEVEL SECURITY;

CREATE POLICY order_verifications_scope ON action.order_verifications USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY order_verification_events_scope ON action.order_verification_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

-- Grants: the append-only operational lifecycle surfaces (INSERT/
-- SELECT); the tasks and the events are never edited — a change is a
-- new event, a new task. The history is the truth.
GRANT SELECT, INSERT ON action.order_verifications TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.order_verifications FROM staffy_app;
GRANT SELECT, INSERT ON action.order_verification_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.order_verification_events FROM staffy_app;

-- +goose Down
DROP TABLE action.order_verification_events;
DROP TABLE action.order_verifications;
