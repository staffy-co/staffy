-- +goose Up
-- Story 2.5: the conversation mode and the mandatory AI disclosure (first
-- slice). The cases schema stays append-only: mode_terms,
-- disclosure_records and disclosure_delivery_events carry INSERT/SELECT
-- grants only. The active mode of the conversation, the Work Item
-- contexts and the disclosure status of a channel are read-time
-- derivations — nothing is ever updated. The mode version is the monotonic
-- CAS counter of the conversation ladder (the epoch precedent of 00004);
-- a proposal can only raise, the only way down is the supervised exit.
-- The disclosure TEXT never enters the outbox envelopes, the exceptions
-- or the logs — it lives in the record only. The reply class of the
-- client replies is the producer's mark (the machine never classifies the
-- content); the pre-migration rows carry the honest NULL.

-- The mode terms: one append-only row per transition. The material
-- uniqueness (tenant, conversation, version) is the CAS race backstop —
-- two concurrent transitions with the same expected version produce
-- exactly one term; the loser gets the honest conflict with the current
-- mode words. The version 1 row is born by the ingest transaction itself
-- (origin ingest_initial, the unclassified admission word — the 2-1 canon
-- «не классифицирован», the safe non-sales path); every later row carries
-- the previous mode snapshot, the actor and — for the supervised exit —
-- the authority word. The turn reference points at an accepted event of
-- the case (the operator fixes the signal against a real client turn).
CREATE TABLE cases.mode_terms (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 term_id uuid NOT NULL,
 mode_version bigint NOT NULL CHECK (mode_version >= 1),
 mode text NOT NULL CHECK (mode IN ('safety','claim','cancellation','privacy_no_contact','human_requested','unclassified','sales')),
 scope text NOT NULL CHECK (scope IN ('conversation','work_item')),
 work_item_id uuid,
 origin text NOT NULL CHECK (origin IN ('ingest_initial','proposal','exit_override')),
 reason text CHECK (reason IS NULL OR char_length(reason) BETWEEN 1 AND 200),
 signal_source text NOT NULL CHECK (signal_source IN ('turn_event','operator','policy_rule')),
 turn_ref uuid,
 actor_id uuid NOT NULL,
 prior_mode text CHECK (prior_mode IS NULL OR prior_mode IN ('safety','claim','cancellation','privacy_no_contact','human_requested','unclassified','sales')),
 exit_authority text CHECK (exit_authority IS NULL OR exit_authority IN ('operator_owner','supervisor')),
 rule_version text NOT NULL CHECK (char_length(rule_version) BETWEEN 1 AND 64),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, term_id),
 UNIQUE (tenant_id, conversation_id, mode_version),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id),
 FOREIGN KEY (tenant_id, turn_ref) REFERENCES cases.channel_events(tenant_id, event_id),
 CONSTRAINT mode_scope_item CHECK (
   (scope = 'conversation' AND work_item_id IS NULL) OR
   (scope = 'work_item' AND work_item_id IS NOT NULL)),
 CONSTRAINT mode_turn_only_with_signal CHECK (
   (signal_source = 'turn_event' AND turn_ref IS NOT NULL) OR
   (signal_source <> 'turn_event' AND turn_ref IS NULL)),
 CONSTRAINT mode_first_term CHECK (
   (mode_version = 1 AND origin = 'ingest_initial' AND scope = 'conversation' AND prior_mode IS NULL AND exit_authority IS NULL) OR
   (mode_version > 1 AND origin <> 'ingest_initial' AND prior_mode IS NOT NULL)),
 CONSTRAINT mode_authority_only_on_exit CHECK (
   (origin = 'exit_override' AND exit_authority IS NOT NULL) OR
   (origin <> 'exit_override' AND exit_authority IS NULL))
);
CREATE INDEX mode_terms_conversation ON cases.mode_terms (tenant_id, conversation_id, mode_version);
CREATE INDEX mode_terms_case ON cases.mode_terms (tenant_id, case_id, created_at);

-- The disclosure records: one append-only row per disclosure. The record
-- is the full snapshot: the exact bounded text, the channel, the release
-- lineage (the employee, the decision number and the profile version of
-- the acting decision — the reply precedent), the actor and the parent
-- reference of the re-disclosure (the interruption voids the acting word
-- — the honest repeat names its parent). The per-conversation monotonic
-- number decides the LAST record of a channel deterministically.
CREATE TABLE cases.disclosure_records (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 disclosure_id uuid NOT NULL,
 disclosure_no bigint NOT NULL CHECK (disclosure_no >= 1),
 channel text NOT NULL CHECK (channel IN ('phone','web_chat','telegram','whatsapp','email','max','jivo','avito','mobile_app_chat')),
 text text NOT NULL CHECK (char_length(text) BETWEEN 1 AND 2000),
 parent_ref uuid,
 employee_id uuid NOT NULL,
 decision_no bigint NOT NULL CHECK (decision_no >= 1),
 profile_version bigint NOT NULL CHECK (profile_version >= 1),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, disclosure_id),
 UNIQUE (tenant_id, conversation_id, disclosure_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id),
 FOREIGN KEY (tenant_id, parent_ref) REFERENCES cases.disclosure_records(tenant_id, disclosure_id)
);
CREATE INDEX disclosure_records_conversation ON cases.disclosure_records (tenant_id, conversation_id, disclosure_no);
CREATE INDEX disclosure_records_case ON cases.disclosure_records (tenant_id, case_id, created_at);

-- The disclosure delivery events: one append-only observation per row
-- (one event — one kind; the states never mix). The per-record monotonic
-- number makes the LAST event the record's status deterministically; the
-- number is computed under the conversation advisory lock of the command.
-- The playback words are NOT a proof the customer heard or understood —
-- the gate needs the send word only; the interruption voids the acting
-- disclosure until the operator re-discloses.
CREATE TABLE cases.disclosure_delivery_events (
 tenant_id uuid NOT NULL,
 event_id uuid NOT NULL,
 disclosure_id uuid NOT NULL,
 event_no bigint NOT NULL CHECK (event_no >= 1),
 kind text NOT NULL CHECK (kind IN ('generated','sent','playback_acknowledged','playback_estimated','customer_confirmed','interrupted')),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id),
 UNIQUE (tenant_id, disclosure_id, event_no),
 FOREIGN KEY (tenant_id, disclosure_id) REFERENCES cases.disclosure_records(tenant_id, disclosure_id)
);
CREATE INDEX disclosure_delivery_events_record ON cases.disclosure_delivery_events (tenant_id, disclosure_id, event_no);

-- The reply class and the Work Item anchor of the client replies: the
-- producer marks the class of every NEW reply (the machine never
-- classifies the content — the honest boundary); the pre-migration rows
-- carry the honest NULL (the view degrades, the gates of the new commands
-- never see them). The Work Item anchor records what the mode gate read
-- (the effective mode derivation input of the moment).
ALTER TABLE cases.client_replies ADD COLUMN reply_class text CHECK (reply_class IN ('sales','neutral','protected_request'));
ALTER TABLE cases.client_replies ADD COLUMN work_item_id uuid;
ALTER TABLE cases.client_replies ADD CONSTRAINT client_replies_work_item_fk FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id);

-- The four new commands join the idempotency surface (the 2-2/2-3/2-4
-- precedent: the stored outcome replays an exact repeat, the same key
-- with another request hash is an honest conflict).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply','propose_mode','exit_mode','record_disclosure','record_disclosure_event'));

-- The two mode outbox event types join the event dictionary of the
-- outbox (the 2-1 precedent: the table carries the bounded type
-- vocabulary).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded'));

ALTER TABLE cases.mode_terms ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.mode_terms FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.disclosure_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.disclosure_records FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.disclosure_delivery_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.disclosure_delivery_events FORCE ROW LEVEL SECURITY;

CREATE POLICY mode_terms_scope ON cases.mode_terms USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY disclosure_records_scope ON cases.disclosure_records USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY disclosure_delivery_events_scope ON cases.disclosure_delivery_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the mode history, the
-- disclosures and the delivery events are never edited.
GRANT SELECT, INSERT ON cases.mode_terms TO staffy_app;
GRANT SELECT, INSERT ON cases.disclosure_records TO staffy_app;
GRANT SELECT, INSERT ON cases.disclosure_delivery_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.mode_terms FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.disclosure_records FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.disclosure_delivery_events FROM staffy_app;

-- +goose Down
ALTER TABLE cases.client_replies DROP CONSTRAINT IF EXISTS client_replies_work_item_fk;
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS work_item_id;
ALTER TABLE cases.client_replies DROP COLUMN IF EXISTS reply_class;
DROP TABLE cases.disclosure_delivery_events;
DROP TABLE cases.disclosure_records;
DROP TABLE cases.mode_terms;
