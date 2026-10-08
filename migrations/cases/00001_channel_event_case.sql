-- +goose Up
-- Story 2.1: Channel Event reception and the single Case (first slice).
-- The cases schema is append-only (INSERT/SELECT, no UPDATE, no DELETE, no
-- TRUNCATE): the case history is never rewritten. A channel event is the
-- material idempotency record of the ingress — UNIQUE (tenant, provider,
-- provider_event_id) among accepted events; a redelivery with the same
-- digest replays the original result, a redelivery with another material
-- payload becomes a quarantined row plus an operational exception (the
-- accepted record is never touched). Case and conversation numbers are
-- monotonic under the tenant advisory lock (the decision_no pattern of
-- 1-14). The outbox tables execute the architecture rule claimed since the
-- identity migration: the initiating module commits the domain state and
-- the outbox records in one transaction.

CREATE SCHEMA cases;

CREATE TABLE cases.cases (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 case_no bigint NOT NULL CHECK (case_no > 0),
 employee_id uuid NOT NULL,
 state text NOT NULL CHECK (state IN ('open')),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, case_id),
 UNIQUE (tenant_id, case_no),
 CONSTRAINT case_employee_required CHECK (employee_id IS NOT NULL)
);

CREATE TABLE cases.pre_case_sessions (
 tenant_id uuid NOT NULL,
 session_id uuid NOT NULL,
 connection_id uuid NOT NULL,
 provider text NOT NULL,
 provider_session_id text NOT NULL CHECK (char_length(provider_session_id) BETWEEN 1 AND 128),
 modality text NOT NULL CHECK (modality IN ('voice','text')),
 capture_policy text NOT NULL CHECK (capture_policy IN ('metadata_only','sealed_payload')),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, session_id),
 UNIQUE (tenant_id, connection_id, provider, provider_session_id),
 CONSTRAINT session_capture_policy_honest CHECK (
   (modality = 'voice' AND capture_policy = 'metadata_only') OR
   (modality = 'text' AND capture_policy = 'sealed_payload'))
);

CREATE TABLE cases.conversations (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 conversation_no bigint NOT NULL CHECK (conversation_no > 0),
 employee_id uuid NOT NULL,
 connection_id uuid NOT NULL,
 provider text NOT NULL,
 provider_session_id text NOT NULL CHECK (char_length(provider_session_id) BETWEEN 1 AND 128),
 response_owner_id uuid NOT NULL,
 response_owner_epoch bigint NOT NULL CHECK (response_owner_epoch = 1),
 writable_surface text NOT NULL CHECK (writable_surface IN ('inbox')),
 state text NOT NULL CHECK (state IN ('active')),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, conversation_id),
 UNIQUE (tenant_id, conversation_no),
 UNIQUE (tenant_id, connection_id, provider, provider_session_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id)
);

CREATE TABLE cases.work_items (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 state text NOT NULL CHECK (state IN ('untriaged')),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, work_item_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id)
);

CREATE TABLE cases.channel_events (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 case_id uuid,
 conversation_id uuid,
 connection_id uuid NOT NULL,
 provider text NOT NULL CHECK (char_length(provider) BETWEEN 1 AND 64),
 provider_event_id text NOT NULL CHECK (char_length(provider_event_id) BETWEEN 1 AND 128),
 provider_session_id text NOT NULL CHECK (char_length(provider_session_id) BETWEEN 1 AND 128),
 direction text NOT NULL CHECK (direction IN ('inbound')),
 modality text NOT NULL CHECK (modality IN ('voice','text')),
 channel text NOT NULL CHECK (channel IN ('phone','web_chat','telegram','whatsapp','email','max','jivo','avito','mobile_app_chat')),
 correlation_id uuid NOT NULL,
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
 provider_timestamp timestamptz NOT NULL,
 received_at timestamptz NOT NULL,
 payload_digest char(64) NOT NULL CHECK (char_length(payload_digest) = 64),
 state text NOT NULL CHECK (state IN ('accepted','quarantined')),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id),
 CONSTRAINT event_case_refs_paired CHECK ((case_id IS NULL) = (conversation_id IS NULL)),
 CONSTRAINT event_accepted_scope CHECK (
   state = 'quarantined' OR (case_id IS NOT NULL AND conversation_id IS NOT NULL)),
 CONSTRAINT event_times_honest CHECK (received_at >= provider_timestamp - interval '48 hours')
);

-- Material idempotency: exactly one accepted event per provider event id;
-- quarantined deliveries of an already-accepted id stay visible for the
-- human without a second accepted record.
CREATE UNIQUE INDEX channel_events_material ON cases.channel_events (tenant_id, provider, provider_event_id) WHERE state = 'accepted';
CREATE UNIQUE INDEX channel_events_idempotency ON cases.channel_events (tenant_id, idempotency_key) WHERE state = 'accepted';
CREATE INDEX channel_events_conversation ON cases.channel_events (tenant_id, conversation_id, received_at);
CREATE INDEX channel_events_connection_window ON cases.channel_events (tenant_id, connection_id, received_at) WHERE state = 'accepted';

CREATE TABLE cases.channel_event_payloads (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 seal_key bytea NOT NULL CHECK (octet_length(seal_key) = 32),
 payload bytea NOT NULL CHECK (octet_length(payload) > 0),
 payload_digest char(64) NOT NULL CHECK (char_length(payload_digest) = 64),
 seal char(64) NOT NULL CHECK (char_length(seal) = 64),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id),
 FOREIGN KEY (tenant_id, event_id) REFERENCES cases.channel_events(tenant_id, event_id),
 CONSTRAINT payload_digest_matches CHECK (payload_digest ~ '^[0-9a-f]{64}$')
);

CREATE TABLE cases.ingress_rejections (
 tenant_id uuid NOT NULL,
 rejection_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('malformed','unknown_connection','revoked_connection','production_forbidden','channel_mismatch','route_not_admitted','no_active_release','no_response_owner','entitlement_blocked','envelope_exceeded')),
 message text NOT NULL CHECK (char_length(message) BETWEEN 1 AND 500),
 correlation_id uuid NOT NULL,
 connection_id uuid,
 counter integer NOT NULL DEFAULT 0 CHECK (counter >= 0),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, rejection_id)
);

CREATE TABLE cases.operational_exceptions (
 tenant_id uuid NOT NULL,
 exception_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('digest_conflict','ambiguous_employee','idempotency_key_conflict')),
 summary text NOT NULL CHECK (char_length(summary) BETWEEN 1 AND 500),
 correlation_id uuid NOT NULL,
 event_id uuid NOT NULL,
 original_event_id uuid,
 case_id uuid,
 conversation_id uuid,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, exception_id),
 FOREIGN KEY (tenant_id, event_id) REFERENCES cases.channel_events(tenant_id, event_id),
 FOREIGN KEY (tenant_id, original_event_id) REFERENCES cases.channel_events(tenant_id, event_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id)
);

CREATE TABLE cases.outbox_events (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 event_type text NOT NULL CHECK (event_type IN ('case.event_accepted','case.connector_ack')),
 schema_version integer NOT NULL CHECK (schema_version > 0),
 aggregate_id uuid NOT NULL,
 source_connection_id uuid NOT NULL,
 correlation_id uuid NOT NULL,
 occurred_at timestamptz NOT NULL,
 payload jsonb NOT NULL,
 dedup_key text NOT NULL CHECK (char_length(dedup_key) BETWEEN 1 AND 200),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id),
 UNIQUE (tenant_id, dedup_key)
);

CREATE TABLE cases.outbox_deliveries (
 tenant_id uuid NOT NULL,
 delivery_id uuid NOT NULL,
 event_id uuid NOT NULL,
 attempt bigint NOT NULL CHECK (attempt > 0),
 outcome text NOT NULL CHECK (outcome IN ('deferred')),
 details text NOT NULL CHECK (char_length(details) BETWEEN 1 AND 500),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, delivery_id),
 FOREIGN KEY (tenant_id, event_id) REFERENCES cases.outbox_events(tenant_id, event_id),
 UNIQUE (tenant_id, event_id, attempt)
);

-- Row level security: every table is tenant-scoped and forced; the
-- ingress transaction sets the scope from the verified connection only.
ALTER TABLE cases.cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.cases FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.pre_case_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.pre_case_sessions FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.conversations FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.work_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.work_items FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.channel_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.channel_events FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.channel_event_payloads ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.channel_event_payloads FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.ingress_rejections ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.ingress_rejections FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.operational_exceptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.operational_exceptions FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.outbox_events FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.outbox_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.outbox_deliveries FORCE ROW LEVEL SECURITY;

CREATE POLICY cases_scope ON cases.cases USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY sessions_scope ON cases.pre_case_sessions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY conversations_scope ON cases.conversations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY work_items_scope ON cases.work_items USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY channel_events_scope ON cases.channel_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY event_payloads_scope ON cases.channel_event_payloads USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY ingress_rejections_scope ON cases.ingress_rejections USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY operational_exceptions_scope ON cases.operational_exceptions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY outbox_events_scope ON cases.outbox_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY outbox_deliveries_scope ON cases.outbox_deliveries USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: append-only surface (INSERT/SELECT); the history is not edited.
GRANT USAGE ON SCHEMA cases TO staffy_app;
GRANT SELECT, INSERT ON cases.cases TO staffy_app;
GRANT SELECT, INSERT ON cases.pre_case_sessions TO staffy_app;
GRANT SELECT, INSERT ON cases.conversations TO staffy_app;
GRANT SELECT, INSERT ON cases.work_items TO staffy_app;
GRANT SELECT, INSERT ON cases.channel_events TO staffy_app;
GRANT SELECT, INSERT ON cases.channel_event_payloads TO staffy_app;
GRANT SELECT, INSERT ON cases.ingress_rejections TO staffy_app;
GRANT SELECT, INSERT ON cases.operational_exceptions TO staffy_app;
GRANT SELECT, INSERT ON cases.outbox_events TO staffy_app;
GRANT SELECT, INSERT ON cases.outbox_deliveries TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.cases FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.pre_case_sessions FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.conversations FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.work_items FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.channel_events FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.channel_event_payloads FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.ingress_rejections FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.operational_exceptions FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.outbox_events FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.outbox_deliveries FROM staffy_app;
