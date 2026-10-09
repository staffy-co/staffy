-- +goose Up
-- Story 2.2: Work Items, facts and provenance inside the Case (first slice).
-- The cases schema stays append-only: work_item_versions, work_item_event_links,
-- fact_versions and fact_confirmations carry INSERT/SELECT grants only. The
-- current state/type/title of a work item is the derivation of the maximal
-- version; the conflicting flag of a fact is a read-time derivation — nothing
-- is ever updated. The work_items.state column is DROPPED: versions are the
-- single carrier of the state. The pack pin (id + version) is written onto
-- the case by the ingress transaction from the RouteCandidate of the acting
-- decision — the taxonomy of an open case never depends on the current
-- admission (the existing commitments continue).

ALTER TABLE cases.cases ADD COLUMN pack_id text;
ALTER TABLE cases.cases ADD COLUMN pack_version text;
ALTER TABLE cases.cases ADD CONSTRAINT case_pack_pin_paired CHECK ((pack_id IS NULL) = (pack_version IS NULL));

-- The state column dies with the slice that needed it: the version ladder is
-- the only carrier of the state.
ALTER TABLE cases.work_items DROP COLUMN state;

CREATE TABLE cases.work_item_versions (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no > 0),
 kind text NOT NULL CHECK (kind IN ('create','classify','transition','split','merge')),
 type text CHECK (type IS NULL OR char_length(type) BETWEEN 1 AND 200),
 title text CHECK (title IS NULL OR char_length(title) BETWEEN 1 AND 200),
 state text NOT NULL CHECK (state IN ('untriaged','in_work','waiting','resolved','cancelled','split','merged')),
 reason text CHECK (reason IS NULL OR char_length(reason) BETWEEN 1 AND 500),
 actor_id uuid,
 ref uuid,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, work_item_id, version_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 FOREIGN KEY (tenant_id, ref) REFERENCES cases.work_items(tenant_id, work_item_id),
 CONSTRAINT version_actor_or_system_honest CHECK (kind = 'create' OR actor_id IS NOT NULL)
);

CREATE INDEX work_item_versions_case ON cases.work_item_versions (tenant_id, case_id, created_at, work_item_id);

-- Event links: the explicit operator binding (there is no automatic link of
-- events to work items as a class). The first event of a new conversation is
-- linked by the ingress itself (actor_id NULL = the system).
CREATE TABLE cases.work_item_event_links (
 tenant_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 event_id uuid NOT NULL,
 actor_id uuid,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, work_item_id, event_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 FOREIGN KEY (tenant_id, event_id) REFERENCES cases.channel_events(tenant_id, event_id)
);

CREATE INDEX work_item_event_links_event ON cases.work_item_event_links (tenant_id, event_id);

-- The fact anchor: one material key per case. Fact rows are never duplicated:
-- a rescope writes a new version, the anchor stays.
CREATE TABLE cases.facts (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 fact_id uuid NOT NULL,
 material_key text NOT NULL CHECK (char_length(material_key) BETWEEN 1 AND 64),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, fact_id),
 CONSTRAINT facts_case_material UNIQUE (tenant_id, case_id, material_key),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id)
);

-- The fact versions: the full provenance on every row. connector and system
-- are reserved dictionary values WITHOUT a producer in this slice (the domain
-- refuses them; future write-path connectors arrive with their own stories).
-- The UNIQUE constraint is the material idempotency: the same source may not
-- write the same material twice — an exact repeat replays, another value is a
-- new version (and a read-time conflict).
CREATE TABLE cases.fact_versions (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 fact_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no > 0),
 work_item_id uuid NOT NULL,
 material_key text NOT NULL CHECK (char_length(material_key) BETWEEN 1 AND 64),
 value jsonb NOT NULL CHECK (octet_length(value::text) BETWEEN 1 AND 2048),
 value_digest char(64) NOT NULL CHECK (char_length(value_digest) = 64),
 source_type text NOT NULL CHECK (source_type IN ('channel_event','operator','connector','system')),
 source_id text NOT NULL CHECK (char_length(source_id) BETWEEN 1 AND 200),
 channel text NOT NULL CHECK (channel IN ('phone','web_chat','telegram','whatsapp','email','max','jivo','avito','mobile_app_chat','operator')),
 actor_id uuid NOT NULL,
 observed_at timestamptz NOT NULL,
 received_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, fact_id, version_no),
 FOREIGN KEY (tenant_id, fact_id) REFERENCES cases.facts(tenant_id, fact_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 CONSTRAINT fact_versions_source_material UNIQUE (tenant_id, source_type, source_id, material_key, value_digest),
 CONSTRAINT fact_times_honest CHECK (received_at >= observed_at - interval '48 hours')
);

CREATE INDEX fact_versions_scope ON cases.fact_versions (tenant_id, work_item_id, version_no);
CREATE INDEX fact_versions_case ON cases.fact_versions (tenant_id, case_id, version_no);

-- The confirmations: append-only, bound to the EXACT version. client and
-- owning_system are reserved dictionary values without a producer (the
-- outgoing dialogs and the write-path connectors of the later stories); the
-- only producer of this slice is operator_relay. The evidence is an accepted
-- event of the case OR a note — exactly one of them.
CREATE TABLE cases.fact_confirmations (
 tenant_id uuid NOT NULL,
 fact_id uuid NOT NULL,
 confirmation_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no > 0),
 confirmer_type text NOT NULL CHECK (confirmer_type IN ('client','owning_system','operator_relay')),
 evidence_event_id uuid,
 note text CHECK (note IS NULL OR char_length(note) BETWEEN 1 AND 200),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, confirmation_id),
 FOREIGN KEY (tenant_id, fact_id) REFERENCES cases.facts(tenant_id, fact_id),
 FOREIGN KEY (tenant_id, evidence_event_id) REFERENCES cases.channel_events(tenant_id, event_id),
 CONSTRAINT confirmation_evidence_required CHECK (
   (evidence_event_id IS NOT NULL AND note IS NULL) OR
   (evidence_event_id IS NULL AND note IS NOT NULL))
);

-- The idempotency records of the ten work commands (the connector 1-9
-- precedent): the stored outcome replays an exact repeat; the same key with
-- another request hash is an honest conflict. The record is actor-scoped.
CREATE TABLE cases.command_records (
 tenant_id uuid NOT NULL,
 actor_id uuid NOT NULL,
 command text NOT NULL CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact')),
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 200),
 request_hash text NOT NULL CHECK (char_length(request_hash) = 64),
 outcome jsonb NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, actor_id, command, idempotency_key)
);

-- The dispute vocabulary of the split/merge commands: the operational
-- exception kind dictionary grows the work_item_dispute value (the human
-- queue of the contour). A dispute has no event — the event_id becomes
-- nullable (the correlation id and the case/conversation references carry
-- the dispute material).
ALTER TABLE cases.operational_exceptions DROP CONSTRAINT operational_exceptions_kind_check;
ALTER TABLE cases.operational_exceptions ADD CONSTRAINT operational_exceptions_kind_check CHECK (kind IN ('digest_conflict','ambiguous_employee','idempotency_key_conflict','work_item_dispute'));
ALTER TABLE cases.operational_exceptions ALTER COLUMN event_id DROP NOT NULL;

-- Row level security: every table is tenant-scoped and forced; the command
-- records are additionally actor-scoped (the replay of one operator never
-- serves another operator).
ALTER TABLE cases.work_item_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.work_item_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.work_item_event_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.work_item_event_links FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.facts ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.facts FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.fact_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.fact_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.fact_confirmations ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.fact_confirmations FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.command_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.command_records FORCE ROW LEVEL SECURITY;

CREATE POLICY work_item_versions_scope ON cases.work_item_versions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY work_item_event_links_scope ON cases.work_item_event_links USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY facts_scope ON cases.facts USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY fact_versions_scope ON cases.fact_versions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY fact_confirmations_scope ON cases.fact_confirmations USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY command_records_scope ON cases.command_records USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid AND actor_id = NULLIF(current_setting('staffy.principal_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid AND actor_id = NULLIF(current_setting('staffy.principal_id',true),'')::uuid);

-- Grants: append-only surface (INSERT/SELECT); the history is not edited.
GRANT SELECT, INSERT ON cases.work_item_versions TO staffy_app;
GRANT SELECT, INSERT ON cases.work_item_event_links TO staffy_app;
GRANT SELECT, INSERT ON cases.facts TO staffy_app;
GRANT SELECT, INSERT ON cases.fact_versions TO staffy_app;
GRANT SELECT, INSERT ON cases.fact_confirmations TO staffy_app;
GRANT SELECT, INSERT ON cases.command_records TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.work_item_versions FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.work_item_event_links FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.facts FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.fact_versions FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.fact_confirmations FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.command_records FROM staffy_app;
