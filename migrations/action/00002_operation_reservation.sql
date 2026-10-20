-- +goose Up
-- Story 3.5 (first slice): the confirmation→authorization contour. The
-- SECOND migration of the action module — six tables born together with
-- ONE transactional identity (ReserveOperation): a confirmation of the
-- consent, the append-only consent history, the immutable authorized
-- operation, the append-only operation ledger, the cross-case business
-- intent reservation and the module's own outbox.
--
-- The discipline is the 00001/00009/00017 precedent: every table is
-- tenant-scoped with RLS ENABLE+FORCE, the append-only surfaces are
-- INSERT/SELECT by grants (the UPDATE/DELETE probes of the integration
-- matrix pin the absence of the paths), the CHECK dictionaries are the
-- loyal backstops of the frozen domain words — never a tenant
-- configuration. Cross-module FKs stay FORBIDDEN (AD-2): case_id and
-- work_item_id are plain uuid pins — the app layer validates them by a
-- LIVE read of the case api BEFORE the insert transaction.
--
-- The 1:1 stream mapping of the confirmations to the preview versions is
-- STRUCTURAL here: UNIQUE (tenant, preview_id, preview_version_no) — one
-- confirmation version row per preview version, both directions. The
-- spec's UNIQUE (tenant, confirmation_id, preview_id) shorthand names the
-- same stream mapping at the thread level; its literal pair form would
-- structurally refuse the frozen domain's re-confirmation (version+1 of
-- the SAME confirmation thread pinning the SAME preview thread — the
-- 2-9/3-4 append-only version pattern), so the materialized bijection
-- lives at the row level: no preview version is ever confirmed twice (the
-- raced first-confirm twin and the raced re-confirm twin are the honest
-- structural refusals), and every confirmation row pins EXACTLY ONE
-- preview version. The thread membership (one confirmation thread never
-- spans two preview threads) is the app resolve + the digest re-check —
-- the authorization compares the linked copy against the LIVE row
-- byte-exact, a mixed thread can never authorize.
--
-- The TWO key spaces of FR-29 are NOT interchangeable (AD-10): the
-- client idempotency_key (the request identity — the repeats carry it
-- again) is UNIQUE on the operations; the server business_intent_key
-- (the mutation identity — the keyed HMAC of the normalized intent) is
-- UNIQUE on the reservations. The replay and the cross-case conflict are
-- structurally distinguishable — the loser of the reservation race gets
-- the honest reference, never a masked replay.
CREATE SCHEMA IF NOT EXISTS action;

-- The append-only consent versions. The row copies the LINKED MATERIAL of
-- the pinned preview version: the exact preview digest (re-checked
-- byte-exact at every authorization — the double digest protection), the
-- material snapshot rows and the finite effect list (the consent carries
-- WHAT was consented to). valid_until is the server truth of the expiry
-- derivation; the state is NEVER a column — DeriveConfirmationState reads
-- the events plus the clock at every read. The owner triple and the
-- release snapshot are the moment pins of the consent (the 2-4/3-3
-- lineage precedents).
CREATE TABLE action.confirmations (
 tenant_id uuid NOT NULL,
 confirmation_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no >= 1),
 definition_id text NOT NULL CHECK (char_length(definition_id) BETWEEN 1 AND 200),
 definition_version text NOT NULL CHECK (char_length(definition_version) BETWEEN 1 AND 32),
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 target_kind text NOT NULL CHECK (target_kind IN ('catalog_item', 'client', 'order')),
 target_ref uuid NOT NULL,
 owner_kind text NOT NULL CHECK (owner_kind IN ('human', 'staffy')),
 owner_ref uuid NOT NULL,
 owner_epoch bigint NOT NULL CHECK (owner_epoch >= 1),
 release_decision_no bigint NOT NULL CHECK (release_decision_no >= 0),
 profile_version bigint NOT NULL CHECK (profile_version >= 0),
 preview_id uuid NOT NULL,
 preview_version_no bigint NOT NULL CHECK (preview_version_no >= 1),
 preview_digest text NOT NULL CHECK (char_length(preview_digest) = 64),
 snapshot jsonb NOT NULL CHECK (jsonb_typeof(snapshot) = 'array' AND octet_length(snapshot::text) <= 65536),
 effects jsonb NOT NULL CHECK (jsonb_typeof(effects) = 'array' AND jsonb_array_length(effects) >= 1),
 valid_until timestamptz NOT NULL,
 confirmed_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, confirmation_id, version_no),
 UNIQUE (tenant_id, preview_id, preview_version_no)
);
CREATE INDEX confirmations_case ON action.confirmations (tenant_id, case_id, created_at);
CREATE INDEX confirmations_thread ON action.confirmations (tenant_id, preview_id, version_no);

-- The append-only consent history: the six-word kind CHECK, the monotonic
-- event_no per version scope (the UNIQUE backstop of the raced appends),
-- the safe reason word and the safe details (identifiers and words only —
-- never a material value, never a secret; the 500 bound is the store
-- backstop of the domain truncation). consumed is appended ONLY inside
-- the ReserveOperation transaction (one event per successful
-- authorization — the read-time count is the audit); superseded lands on
-- the OLD version in the same transaction as the new version's confirmed;
-- authorization_conflict and authorization_blocked are the visible audit
-- words of a rejected authorization attempt.
CREATE TABLE action.confirmation_events (
 tenant_id uuid NOT NULL,
 confirmation_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no >= 1),
 case_id uuid NOT NULL,
 event_id uuid NOT NULL,
 event_no bigint NOT NULL CHECK (event_no >= 1),
 kind text NOT NULL CHECK (kind IN ('confirmed', 'superseded', 'revoked', 'consumed', 'authorization_conflict', 'authorization_blocked')),
 reason text NOT NULL DEFAULT '' CHECK (char_length(reason) <= 200),
 details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 500),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id),
 UNIQUE (tenant_id, confirmation_id, version_no, event_no)
);
CREATE INDEX confirmation_events_case ON action.confirmation_events (tenant_id, case_id, created_at);
CREATE INDEX confirmation_events_version ON action.confirmation_events (tenant_id, confirmation_id, version_no, event_no);

-- The IMMUTABLE authorized operation: ONE row per operation, NO versions
-- (the authorization is a one-time act; a change is a conflict or a NEW
-- operation through a NEW confirmation — never a mutation of this row).
-- The external identifier and the Action Receipt have NO columns (3-6
-- owns them; the honest texts speak the absence). The state is NEVER
-- stored: DeriveOperationState reads the append-only ledger at every
-- read. UNIQUE (tenant, idempotency_key) is the replay dedup — the raced
-- twin of one client key maps to the honest replay comparison; there is
-- NO unique on the intent (the cross-case reservation is the separate
-- table — the loser is distinguishable from the replay). The key words
-- are 64-hex keyed HMAC identities; the client key charset is the loyal
-- backstop of the domain gate.
CREATE TABLE action.operations (
 tenant_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 definition_id text NOT NULL CHECK (char_length(definition_id) BETWEEN 1 AND 200),
 definition_version text NOT NULL CHECK (char_length(definition_version) BETWEEN 1 AND 32),
 preview_id uuid NOT NULL,
 preview_version_no bigint NOT NULL CHECK (preview_version_no >= 1),
 confirmation_id uuid NOT NULL,
 confirmation_version_no bigint NOT NULL CHECK (confirmation_version_no >= 1),
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 target_kind text NOT NULL CHECK (target_kind IN ('catalog_item', 'client', 'order')),
 target_ref uuid NOT NULL,
 owner_kind text NOT NULL CHECK (owner_kind IN ('human', 'staffy')),
 owner_ref uuid NOT NULL,
 owner_epoch bigint NOT NULL CHECK (owner_epoch >= 1),
 release_decision_no bigint NOT NULL CHECK (release_decision_no >= 0),
 profile_version bigint NOT NULL CHECK (profile_version >= 0),
 partition text NOT NULL CHECK (partition IN ('staffy', 'human_only')),
 effects jsonb NOT NULL CHECK (jsonb_typeof(effects) = 'array' AND jsonb_array_length(effects) >= 1),
 business_intent_key text NOT NULL CHECK (char_length(business_intent_key) = 64),
 idempotency_key text NOT NULL CHECK (idempotency_key ~ '^[A-Za-z0-9._-]{8,128}$'),
 material_digest text NOT NULL CHECK (char_length(material_digest) = 64),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, operation_id),
 UNIQUE (tenant_id, idempotency_key)
);
CREATE INDEX operations_case ON action.operations (tenant_id, case_id, created_at);
CREATE INDEX operations_intent ON action.operations (tenant_id, business_intent_key);

-- The append-only Operation Ledger (AD-13): the monotonic entry number of
-- the operation, the single-word kind dictionary of the slice
-- ({authorized} — the dispatch words of 3-6 will extend the CHECK through
-- the same mechanism), the safe details and the moment. The history is
-- never rewritten; the grants are INSERT/SELECT only.
CREATE TABLE action.operation_ledger (
 tenant_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 entry_no bigint NOT NULL CHECK (entry_no >= 1),
 kind text NOT NULL CHECK (kind IN ('authorized')),
 details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 500),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, operation_id, entry_no)
);

-- The cross-case business intent reservation: the PRIMARY KEY IS the
-- unique tenant+Owning-System reservation («допускает только одну
-- Operation» — AC-9 материальна) — the equivalent mutation from another
-- Case does not pass; the loser reads the honest reference of the
-- existing tenant-visible Operation. The FK stays INSIDE the module
-- (AD-2); case_id is a plain pin. The FK is DEFERRABLE INITIALLY
-- DEFERRED: the frozen INSERT order of ReserveOperation writes the
-- reservation BEFORE the operation row (the reservation UNIQUE fires
-- first — the raced same-key twin maps to ErrReservationConflict, the
-- raced cross-case intent maps to the honest reference, the two
-- identities stay distinguishable at the constraint level), so the
-- reference integrity checks at COMMIT — the transaction is one
-- atomic truth anyway (ANY failure rolls back everything, AC-6
-- material).
CREATE TABLE action.reservations (
 tenant_id uuid NOT NULL,
 owning_system text NOT NULL CHECK (char_length(owning_system) BETWEEN 1 AND 64),
 business_intent_key text NOT NULL CHECK (char_length(business_intent_key) = 64),
 operation_id uuid NOT NULL,
 case_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, owning_system, business_intent_key),
 FOREIGN KEY (tenant_id, operation_id) REFERENCES action.operations (tenant_id, operation_id) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX reservations_operation ON action.reservations (tenant_id, operation_id);

-- The module's own outbox (cases.outbox_events is a FOREIGN table — AD-2
-- forbids the cross-module SQL; the 3-3 queued-without-worker precedent).
-- The payload is the SAFE canonical jsonb: ids, pins, words and the two
-- key identities — the material (typed input values, snapshot rows,
-- phone/template words) NEVER enters; the 3-6 worker will rebuild the
-- material payload from the immutable rows. The drain contour is 3-6: in
-- this slice the records honestly accumulate pending — no worker exists,
-- no UPDATE/DELETE grant exists either.
CREATE TABLE action.outbox_events (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 event_type text NOT NULL CHECK (event_type IN ('operation.queued')),
 case_id uuid NOT NULL,
 operation_id uuid NOT NULL,
 payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object' AND octet_length(payload::text) <= 16384),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id)
);
CREATE INDEX outbox_events_operation ON action.outbox_events (tenant_id, operation_id);

-- Row level security: every table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate).
ALTER TABLE action.confirmations ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.confirmations FORCE ROW LEVEL SECURITY;
ALTER TABLE action.confirmation_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.confirmation_events FORCE ROW LEVEL SECURITY;
ALTER TABLE action.operations ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.operations FORCE ROW LEVEL SECURITY;
ALTER TABLE action.operation_ledger ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.operation_ledger FORCE ROW LEVEL SECURITY;
ALTER TABLE action.reservations ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.reservations FORCE ROW LEVEL SECURITY;
ALTER TABLE action.outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE action.outbox_events FORCE ROW LEVEL SECURITY;

CREATE POLICY confirmations_scope ON action.confirmations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY confirmation_events_scope ON action.confirmation_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY operations_scope ON action.operations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY operation_ledger_scope ON action.operation_ledger USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY reservations_scope ON action.reservations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);
CREATE POLICY outbox_events_scope ON action.outbox_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id', true), '')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the consent versions,
-- the events, the operations, the ledger, the reservations and the outbox
-- are never edited or deleted through the application role (the
-- append-only/immutable probes of the integration matrix pin the absence
-- of the UPDATE/DELETE paths).
GRANT USAGE ON SCHEMA action TO staffy_app;
GRANT SELECT, INSERT ON action.confirmations TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.confirmations FROM staffy_app;
GRANT SELECT, INSERT ON action.confirmation_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.confirmation_events FROM staffy_app;
GRANT SELECT, INSERT ON action.operations TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.operations FROM staffy_app;
GRANT SELECT, INSERT ON action.operation_ledger TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.operation_ledger FROM staffy_app;
GRANT SELECT, INSERT ON action.reservations TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.reservations FROM staffy_app;
GRANT SELECT, INSERT ON action.outbox_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON action.outbox_events FROM staffy_app;

-- +goose Down
DROP SCHEMA action CASCADE;
