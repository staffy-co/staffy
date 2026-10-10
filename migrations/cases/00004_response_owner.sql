-- +goose Up
-- Story 2.4: the single response owner and the writing inbox (first slice).
-- The cases schema stays append-only: ownership_terms, transfer_requests,
-- transfer_resolutions, client_replies and client_reply_outcomes carry
-- INSERT/SELECT grants only. The current ownership of a case is the
-- read-time derivation of the maximal epoch term; the pending transfer is
-- the latest request without a resolution (its expiry is a time
-- derivation); the reply state is the absence or the kind of its outcome.
-- The conversations columns of 00001 (CHECK epoch = 1, CHECK surface =
-- 'inbox') are NEVER touched — they stay the creation snapshot; the live
-- ownership state lives in the terms. The reply CONTENT never enters the
-- outbox envelopes, the exceptions or the logs — it lives in the reply
-- record only.

-- The transfer requests: the assigned (not yet resolved) handoff ask. The
-- deadline comes from the human coverage of the acting route decision
-- (AckDeadlineMinutes, re-read through RouteFacts) — never from a command
-- parameter. One pending request per case is the command discipline under
-- the case advisory lock; the UNIQUE request number keeps the per-case
-- numbering monotonic. The request row is a full snapshot: the from/to
-- owners, the surface the target would own and the honest reason.
CREATE TABLE cases.transfer_requests (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 request_id uuid NOT NULL,
 request_no bigint NOT NULL CHECK (request_no >= 1),
 from_owner_kind text NOT NULL CHECK (from_owner_kind IN ('human','staffy')),
 from_owner_ref uuid NOT NULL,
 to_owner_kind text NOT NULL CHECK (to_owner_kind IN ('human','staffy')),
 to_owner_ref uuid NOT NULL,
 writable_surface text NOT NULL CHECK (writable_surface IN ('inbox','external_inbox')),
 reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 200),
 requested_by uuid NOT NULL,
 deadline_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, request_id),
 UNIQUE (tenant_id, case_id, request_no),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 CONSTRAINT transfer_owner_change CHECK (from_owner_ref <> to_owner_ref OR from_owner_kind <> to_owner_kind)
);
CREATE INDEX transfer_requests_case ON cases.transfer_requests (tenant_id, case_id, created_at);

-- The ownership terms: one append-only row per ownership period. The
-- material uniqueness (tenant, case, epoch) is the CAS race backstop —
-- two concurrent transitions with the same expected epoch produce exactly
-- one term; the loser gets the honest conflict. The epoch 1 row is born by
-- the ingest transaction itself (origin ingest_initial, the owner from the
-- human coverage of the admitting decision, the inbox surface); every
-- later row carries the previous owner, the actor and — for the
-- acknowledged transfer — the request it answered.
CREATE TABLE cases.ownership_terms (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 term_id uuid NOT NULL,
 epoch bigint NOT NULL CHECK (epoch >= 1),
 owner_kind text NOT NULL CHECK (owner_kind IN ('human','staffy')),
 owner_ref uuid NOT NULL,
 writable_surface text NOT NULL CHECK (writable_surface IN ('inbox','external_inbox')),
 origin text NOT NULL CHECK (origin IN ('ingest_initial','take','transfer_acknowledge','reassign')),
 previous_owner_kind text CHECK (previous_owner_kind IN ('human','staffy')),
 previous_owner_ref uuid,
 actor_id uuid NOT NULL,
 reason text CHECK (reason IS NULL OR char_length(reason) BETWEEN 1 AND 200),
 transfer_request_id uuid,
 requested_at timestamptz,
 acknowledged_at timestamptz,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, term_id),
 UNIQUE (tenant_id, case_id, epoch),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, transfer_request_id) REFERENCES cases.transfer_requests(tenant_id, request_id),
 CONSTRAINT ownership_first_term CHECK (
   (epoch = 1 AND origin = 'ingest_initial' AND previous_owner_ref IS NULL AND transfer_request_id IS NULL) OR
   (epoch > 1 AND origin <> 'ingest_initial' AND previous_owner_ref IS NOT NULL)),
 CONSTRAINT ownership_request_snapshots CHECK (
   (transfer_request_id IS NULL AND requested_at IS NULL) OR
   (transfer_request_id IS NOT NULL AND requested_at IS NOT NULL))
);
CREATE INDEX ownership_terms_case ON cases.ownership_terms (tenant_id, case_id, epoch);

-- The transfer resolutions: one append-only decision per request (the
-- UNIQUE is the race backstop). The acknowledge resolution points at the
-- resulting epoch of the term the SAME transaction wrote; decline and
-- cancel carry no epoch — the ownership never changed. The expiry needs
-- no row: the deadline derivation voids the request for every later
-- command (a late acknowledge gets the honest request_expired refusal).
CREATE TABLE cases.transfer_resolutions (
 tenant_id uuid NOT NULL,
 resolution_id uuid NOT NULL,
 request_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('acknowledged','declined','cancelled')),
 actor_id uuid NOT NULL,
 resulting_epoch bigint,
 reason text CHECK (reason IS NULL OR char_length(reason) BETWEEN 1 AND 200),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, resolution_id),
 UNIQUE (tenant_id, request_id),
 FOREIGN KEY (tenant_id, request_id) REFERENCES cases.transfer_requests(tenant_id, request_id),
 CONSTRAINT resolution_epoch_only_on_ack CHECK (
   (kind = 'acknowledged' AND resulting_epoch IS NOT NULL) OR
   (kind <> 'acknowledged' AND resulting_epoch IS NULL))
);

-- The client replies: the single client-bound outgoing path of the slice.
-- The row is the full snapshot of the send: the governing case (the case
-- itself or the binding target of an actively bound conversation), the
-- conversation, the owner (kind/ref/epoch) and the surface at the moment
-- of the command, the route of the conversation (connection, provider,
-- channel, direction) and the release lineage (the employee, the decision
-- number and the profile version of the acting decision re-read through
-- RouteFacts). The content is the operator's own words (bounded); it
-- never travels anywhere else. The state is a derivation: no outcome row
-- — prepared; an outcome row — its kind.
CREATE TABLE cases.client_replies (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 reply_id uuid NOT NULL,
 owner_kind text NOT NULL CHECK (owner_kind IN ('human','staffy')),
 owner_ref uuid NOT NULL,
 epoch bigint NOT NULL CHECK (epoch >= 1),
 writable_surface text NOT NULL CHECK (writable_surface IN ('inbox','external_inbox')),
 connection_id uuid NOT NULL,
 provider text NOT NULL,
 channel text NOT NULL,
 direction text NOT NULL CHECK (direction = 'outbound'),
 employee_id uuid NOT NULL,
 decision_no bigint NOT NULL,
 profile_version bigint NOT NULL,
 content text NOT NULL CHECK (char_length(content) BETWEEN 1 AND 2000),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, reply_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id)
);
CREATE INDEX client_replies_case ON cases.client_replies (tenant_id, case_id, created_at);

-- The client reply outcomes: the append-only execute-time revalidation
-- verdict of the drain. The UNIQUE (tenant, reply) is the material
-- idempotency — a repeated drain finds the reply decided and writes
-- nothing. dispatched is the honest synthetic-contour record of the
-- write (the real channel delivery is the deferred outgoing story);
-- stale_needs_review blocks the stale payload — it is never relayed to
-- the new owner automatically.
CREATE TABLE cases.client_reply_outcomes (
 tenant_id uuid NOT NULL,
 outcome_id uuid NOT NULL,
 reply_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('dispatched','stale_needs_review')),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, outcome_id),
 UNIQUE (tenant_id, reply_id),
 FOREIGN KEY (tenant_id, reply_id) REFERENCES cases.client_replies(tenant_id, reply_id)
);

-- The five new commands join the idempotency surface (the 2-2/2-3
-- precedent: the stored outcome replays an exact repeat, the same key
-- with another request hash is an honest conflict).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate','take_ownership','request_transfer','resolve_transfer','reassign_ownership','send_client_reply'));

-- The three ownership outbox event types join the event dictionary of
-- the outbox (the 2-1 precedent: the table carries the bounded type
-- vocabulary).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched'));

-- The ownership attention exception of the drain observation (the expired
-- pending request or the unavailable owner; no PII). The partial UNIQUE
-- index is the material uniqueness that voids the repeats: one exception
-- per observation subject (the correlation id carries the request id or
-- the term id).
ALTER TABLE cases.operational_exceptions DROP CONSTRAINT operational_exceptions_kind_check;
ALTER TABLE cases.operational_exceptions ADD CONSTRAINT operational_exceptions_kind_check CHECK (kind IN ('digest_conflict','ambiguous_employee','idempotency_key_conflict','work_item_dispute','identity_step_up','ownership_attention'));
CREATE UNIQUE INDEX ownership_attention_once ON cases.operational_exceptions (tenant_id, correlation_id) WHERE kind = 'ownership_attention';

ALTER TABLE cases.ownership_terms ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.ownership_terms FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.transfer_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.transfer_requests FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.transfer_resolutions ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.transfer_resolutions FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.client_replies ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.client_replies FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.client_reply_outcomes ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.client_reply_outcomes FORCE ROW LEVEL SECURITY;

CREATE POLICY ownership_terms_scope ON cases.ownership_terms USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY transfer_requests_scope ON cases.transfer_requests USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY transfer_resolutions_scope ON cases.transfer_resolutions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY client_replies_scope ON cases.client_replies USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY client_reply_outcomes_scope ON cases.client_reply_outcomes USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the ownership history
-- and the replies are never edited.
GRANT SELECT, INSERT ON cases.ownership_terms TO staffy_app;
GRANT SELECT, INSERT ON cases.transfer_requests TO staffy_app;
GRANT SELECT, INSERT ON cases.transfer_resolutions TO staffy_app;
GRANT SELECT, INSERT ON cases.client_replies TO staffy_app;
GRANT SELECT, INSERT ON cases.client_reply_outcomes TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.ownership_terms FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.transfer_requests FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.transfer_resolutions FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.client_replies FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.client_reply_outcomes FROM staffy_app;

-- +goose Down
DROP INDEX cases.ownership_attention_once;
DROP TABLE cases.client_reply_outcomes;
DROP TABLE cases.client_replies;
DROP TABLE cases.transfer_resolutions;
DROP TABLE cases.transfer_requests;
DROP TABLE cases.ownership_terms;
