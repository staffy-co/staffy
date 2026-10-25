-- +goose Up
-- Story 4.10 (step-01): the Order Servicing Requests — the NINTH
-- migration of the action module: ONE append-only table of the repeat
-- contact requests over the EXISTING order operations of the tenant.
-- The discipline is the 00003/00005/00006/00007/00008 precedent:
-- tenant-scoped with RLS ENABLE+FORCE, the append-only surface is
-- INSERT/SELECT by grants (the UPDATE/DELETE probes of the step-04
-- integration matrix pin the absence of the paths), the CHECK
-- dictionaries are the loyal backstops of the frozen domain words —
-- never a tenant configuration.
--
-- The request is the RECORDED EVENT of the repeat contact (FR-62): the
-- operator with the review scope records WHAT THE CLIENT ASKED — the
-- intent word (the 8-word dictionary), the identifier the client named
-- (the TYPED kind word with the value) and the LIVE proof pins of the
-- case assurance at the moment of the record. The request NEVER
-- executes anything: no cancel path, no change path, no supplier-send
-- path exists here or anywhere (the OQ-3 fence; the AG-4 manual
-- contour) — the cancellation is SPOKEN in its four read-time states,
-- never performed. The repeated contact NEVER creates a new order
-- operation: resolved_operation_id REFERENCES the existing operation
-- (nullable — not_found and ambiguous anchor nothing), the write path
-- of the slice is this ONE append (the FR-62 count probe of the
-- step-04 matrix).
--
-- The UNIQUE full request key (tenant, case, actor, intent,
-- identifier_kind, identifier_value, proof_state) is the IDEMPOTENCY
-- BY KEY of the repeated record command (the 4-9 fact precedent): the
-- replay of the same command at the SAME trust state is the same saved
-- request, never a second row. The CHANGED trust state (none →
-- verified) is a NEW row — the trust ladder stays visible in the
-- history (both rows live).
--
-- The request_no is the per-case monotonic ladder under the advisory
-- lock (the 00006/00007/00008 precedent — the server truth of the feed
-- order); the conversation anchor is nullable (the request may ride a
-- case without a live conversation); the actor identity pins the
-- operator; the proof pins (the state, the method, the issued/expired
-- times) are the LIVE copy of the assurance derivation of 2-3 at the
-- moment of the record — the scope, the level and the time of the
-- check are preserved (AC-4), the record is never rewritten afterwards.
CREATE TABLE action.order_servicing_requests (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NULL,
 request_no bigint NOT NULL CHECK (request_no >= 1),
 intent text NOT NULL CHECK (intent IN ('order_status', 'clarification', 'change', 'cancel', 'return', 'claim', 'dispute', 'human')),
 identifier_kind text NOT NULL CHECK (identifier_kind IN ('abcp_client_order_id', 'supplier_order_id')),
 identifier_value text NOT NULL CHECK (char_length(identifier_value) BETWEEN 1 AND 200),
 resolved_operation_id uuid NULL,
 lookup_outcome text NOT NULL CHECK (lookup_outcome IN ('unique_match', 'not_found', 'ambiguous')),
 proof_state text NOT NULL CHECK (proof_state IN ('none', 'basic', 'verified', 'step_up')),
 proof_method text NOT NULL DEFAULT '',
 proof_issued_at timestamptz NULL,
 proof_expires_at timestamptz NULL,
 disclosure text NOT NULL CHECK (disclosure IN ('allowed', 'denied')),
 actor_id uuid NOT NULL,
 recorded_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id, request_no),
 FOREIGN KEY (tenant_id, resolved_operation_id) REFERENCES action.operations(tenant_id, operation_id)
);
-- The idempotency BY KEY (the 4-9 fact precedent): the repeated record
-- command of the same request key at the same trust state replays the
-- saved outcome — the second row NEVER lands, the UNIQUE backstop
-- holds even under a lost race. The changed trust state (none →
-- verified) is a DIFFERENT key — a new row, the ladder stays honest.
CREATE UNIQUE INDEX order_servicing_request_key ON action.order_servicing_requests (tenant_id, case_id, actor_id, intent, identifier_kind, identifier_value, proof_state);
CREATE INDEX order_servicing_requests_case ON action.order_servicing_requests (tenant_id, case_id, request_no);
CREATE INDEX order_servicing_requests_operation ON action.order_servicing_requests (tenant_id, resolved_operation_id);

-- Row level security: the table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate — the operator
-- reads and writes ONLY under TenantContext).
ALTER TABLE action.order_servicing_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.order_servicing_requests FORCE ROW LEVEL SECURITY;

CREATE POLICY order_servicing_requests_scope ON action.order_servicing_requests USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

-- Grants: the append-only recorded-event surface (INSERT/SELECT); the
-- requests are never edited — the changed trust state is a NEW request
-- of the new reality. The lookup writes NOTHING — it reads the existing
-- material of the contour (the receipts, the rereads, the facts).
GRANT SELECT, INSERT ON action.order_servicing_requests TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.order_servicing_requests FROM staffy_app;

-- +goose Down
DROP TABLE action.order_servicing_requests;
