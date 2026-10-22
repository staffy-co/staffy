-- +goose Up
-- Story 3.8 (step-02): the recovery-case contour — the linked Recovery
-- Case aggregate and its append-only event history. The FIFTH migration
-- of the action module: two new append-only tables born together with
-- the ONE creation transaction of the contour (CreateRecoveryCase — the
-- advisory lock (tenant, operation_id) resolves the creation race by the
-- idempotent return, the UNIQUE (tenant_id, operation_id) is the
-- structural twin of the keyed recovery key: ONE case per failed
-- Operation FOREVER — the reopen of a closed case is a transition of the
-- SAME case, a future slice). The discipline is the 00003/00004
-- precedent: every table is tenant-scoped with RLS ENABLE+FORCE, the
-- append-only surfaces are INSERT/SELECT by grants (the UPDATE/DELETE
-- probes of the step-04 integration matrix pin the absence of the
-- paths), the CHECK dictionaries are the loyal backstops of the frozen
-- domain words — never a tenant configuration.
--
-- The case row carries the SAFE link-bundle of the frozen creation
-- moment (AD-10/AC-9): identifiers, versions, counters, words and 64-hex
-- digests ONLY — the material NEVER enters (the sensitive payload stays
-- in its own immutable preview rows; the recovery speaks minimal
-- references instead of copies), the phone and the template are
-- structurally absent. The assignment columns pin the LIVE readings of
-- the creation moment (the current ownership term of the original Case,
-- the backup owner and the deadline of the human coverage of the
-- original conversation route — the coverageWord canon: the words NEVER
-- come from a command parameter). The bundle_digest is the keyed
-- canonical identity of the SAFE bundle (the repeat comparator: equal —
-- the idempotent return; a delta — ONE append-only evidence_appended
-- row; the original bundle and the original receipts are NEVER
-- rewritten).
--
-- The outbox event dictionary grows 2→3 (the sanctioned ALTER precedent
-- of the ledger 7→8 in 00004): recovery_case.created accumulates
-- honestly pending — the claim-mode consumer of a future slice reads the
-- records (the receipt.created precedent of 3-6).
CREATE TABLE action.recovery_cases (
 tenant_id uuid NOT NULL,
 recovery_case_id uuid NOT NULL,
 recovery_case_no bigint NOT NULL CHECK (recovery_case_no > 0),
 recovery_key text NOT NULL CHECK (recovery_key ~ '^[0-9a-f]{64}$'),
 operation_id uuid NOT NULL,
 trigger_word text NOT NULL CHECK (trigger_word IN ('reconciliation_exhausted', 'reconciliation_unavailable', 'operation_failed')),
 -- origin pins
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 definition_id text NOT NULL CHECK (char_length(definition_id) BETWEEN 1 AND 200),
 definition_version text NOT NULL CHECK (char_length(definition_version) BETWEEN 1 AND 64),
 -- execution-manifest pins (the EXACT versions)
 preview_id uuid NOT NULL,
 preview_version_no bigint NOT NULL CHECK (preview_version_no >= 1),
 confirmation_id uuid NOT NULL,
 confirmation_version_no bigint NOT NULL CHECK (confirmation_version_no >= 1),
 -- authorization pins (the operation's owner triple at the authorization)
 auth_owner_kind text NOT NULL CHECK (char_length(auth_owner_kind) BETWEEN 1 AND 64),
 auth_owner_ref uuid NOT NULL,
 auth_owner_epoch bigint NOT NULL CHECK (auth_owner_epoch >= 1),
 release_decision_no bigint NOT NULL,
 profile_version bigint NOT NULL,
 business_intent_key text NOT NULL CHECK (char_length(business_intent_key) = 64),
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
 material_digest text NOT NULL CHECK (char_length(material_digest) = 64),
 -- attempt counters of both contours
 dispatch_attempts int NOT NULL CHECK (dispatch_attempts >= 0),
 reconcile_attempts int NOT NULL CHECK (reconcile_attempts >= 0),
 -- receipt-chain pins (the frozen moment; the living chain derives at the read)
 receipt_versions_count bigint NOT NULL CHECK (receipt_versions_count >= 0),
 latest_receipt_version_no bigint NOT NULL CHECK (latest_receipt_version_no >= 0),
 latest_receipt_outcome text NOT NULL DEFAULT '' CHECK (latest_receipt_outcome IN ('', 'committed', 'failed', 'unknown')),
 -- supporting-observations pins of the preview (2-7 version pins — bounded)
 observations jsonb NOT NULL CHECK (jsonb_typeof(observations) = 'array' AND jsonb_array_length(observations) <= 8 AND octet_length(observations::text) <= 2048),
 -- party pins
 target_kind text NOT NULL CHECK (target_kind IN ('catalog_item', 'client', 'order')),
 target_ref uuid NOT NULL,
 owning_system text NOT NULL CHECK (char_length(owning_system) BETWEEN 1 AND 64),
 -- external identity of the last receipt (empty/NULL — the honest unknown)
 external_object_id text NOT NULL DEFAULT '' CHECK (char_length(external_object_id) <= 200),
 external_object_version text NOT NULL DEFAULT '' CHECK (char_length(external_object_version) <= 64),
 external_object_time timestamptz,
 -- assignment (the LIVE readings of the creation moment — the coverageWord canon)
 assign_owner_kind text NOT NULL CHECK (char_length(assign_owner_kind) BETWEEN 1 AND 64),
 assign_owner_ref uuid NOT NULL,
 assign_owner_epoch bigint NOT NULL CHECK (assign_owner_epoch >= 1),
 backup_owner_id uuid NOT NULL,
 deadline_at timestamptz NOT NULL,
 -- the frozen creation digest (the repeat comparator)
 bundle_digest text NOT NULL CHECK (bundle_digest ~ '^[0-9a-f]{64}$'),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, recovery_case_id),
 UNIQUE (tenant_id, operation_id),
 UNIQUE (tenant_id, recovery_case_no),
 FOREIGN KEY (tenant_id, operation_id) REFERENCES action.operations (tenant_id, operation_id)
);
CREATE INDEX recovery_cases_created ON action.recovery_cases (tenant_id, created_at DESC);

-- The append-only event history of the case: created (ONE per operation
-- forever — the UNIQUE above) and evidence_appended (the later terminal
-- evidence of the 3-7 contour pinned as ONE SAFE delta row: the original
-- bundle is never rewritten, the context updates by the reference). The
-- closing words do NOT exist in this slice (the read-time state derives
-- open — a closure would be a NEW dictionary review, never a runtime
-- extension).
CREATE TABLE action.recovery_events (
 tenant_id uuid NOT NULL,
 recovery_case_id uuid NOT NULL,
 event_no bigint NOT NULL CHECK (event_no >= 1),
 kind text NOT NULL CHECK (kind IN ('created', 'evidence_appended')),
 details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 500),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, recovery_case_id, event_no),
 FOREIGN KEY (tenant_id, recovery_case_id) REFERENCES action.recovery_cases (tenant_id, recovery_case_id)
);
CREATE INDEX recovery_events_case ON action.recovery_events (tenant_id, recovery_case_id, event_no);

-- The dictionary extension of the 00003 table (ALTER — the data is never
-- rewritten, the vocabulary widens): the three-word outbox event
-- dictionary (recovery_case.created — the AD-2 discipline: the link
-- travels as an event of THIS module's own outbox, the future consumer
-- reads the pending records, the receipt.created precedent).
ALTER TABLE action.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE action.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('operation.queued', 'receipt.created', 'recovery_case.created'));

-- Row level security: every table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate — the recovery
-- contour reads ONLY under TenantContext, AD-11).
ALTER TABLE action.recovery_cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.recovery_cases FORCE ROW LEVEL SECURITY;
ALTER TABLE action.recovery_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.recovery_events FORCE ROW LEVEL SECURITY;

CREATE POLICY recovery_cases_scope ON action.recovery_cases USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY recovery_events_scope ON action.recovery_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

-- Grants: the append-only surface (INSERT/SELECT only) — the cases and
-- the events are never edited or deleted through the application role
-- (the recovery history is never rewritten: the original bundle, the
-- original receipts and the append-only events stay as they landed).
GRANT USAGE ON SCHEMA action TO staffy_app;
GRANT SELECT, INSERT ON action.recovery_cases TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.recovery_cases FROM staffy_app;
GRANT SELECT, INSERT ON action.recovery_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.recovery_events FROM staffy_app;

-- +goose Down
DROP SCHEMA action CASCADE;
