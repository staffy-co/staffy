-- +goose Up
-- Story 3.3 (first slice): the external commitment dispatch. The cases
-- schema stays append-only: external_commitments, external_commitment_
-- approvals and external_dispatch_events carry INSERT/SELECT grants only.
-- The envelope is the append-only version ladder of 00009/2.9: a stable
-- ext_commitment_id with the monotonic version_no (the per-envelope
-- counter computed under the case + work item advisory locks — the
-- commitments precedent), a changed text is a NEW version row, the old
-- versions never mutate (AC-4: the queued payload pins the version, the
-- determinism comes from the immutability, never from a snapshot). The
-- 2.9 exact-version pin is the nullable all-or-nothing triple
-- (commitment_id, version_no, content_token — both-or-neither), MANDATORY
-- for the offer purpose; the pin token is the STORED token of the pinned
-- 2.9 row (the live match is the app gate — the stored word is the truth,
-- never a re-computation over the jsonb bytes). The content binding of
-- THIS envelope version is the keyed HMAC token over the canonical
-- bytes (the per-tenant commitment key v1 of commitments_hmac_keys — the
-- 2.9 reuse; the token space carries the external_commitment:v1:
-- prefix, materially distinct from commitment:v1:).
-- The approvals bind to the EXACT version and the CURRENT human owner:
-- the UNIQUE (tenant, envelope, version, approver) makes the repeat of
-- the same approver the silent idempotency, a new owner after an epoch
-- move writes a NEW row and the old approval structurally never
-- authorises the changed text (the enqueue gate reads the approver =
-- current owner of the exact version — AC-9).
-- The dispatch_events are the append-only delivery history: the
-- monotonic event_no per version (the UNIQUE backstop of the raced
-- appends), the seven-word kind CHECK, the four-word stale reason
-- (required iff cancelled_stale), the bounded attempt_no (0 for the
-- receipt and the gasp events, 1..3 for the attempts — MaxDispatch
-- Attempts), the external correlation id of the submitted/unknown
-- evidence (AC-8) and the safe details only (correlation, outcome,
-- cause — the content NEVER enters the event material, AC-10). The
-- seven delivery states are NEVER stored: they are the read-time
-- derivation over the append-only history (the 2.12 precedent).
-- The enqueue is persist-before-emit STRUCTURALLY: the ONE transaction
-- inserts the outbox record 'external_commitment.queued' with the safe
-- payload (ids, the version pin, the dictionary words, the idempotency
-- key and the content digest — the content text never enters the
-- envelope, AC-10); the transactional method carries no adapter — the
-- network call is physically impossible before the commit (AC-3), and
-- the drain (a command by the operator/CLI — the client-replies drain
-- precedent; no background daemons exist, AD-13) is the only surface
-- that may call the dispatcher AFTER the commit.

-- The envelope versions: one append-only row per version of one
-- external commitment. The PK is the version pin itself (the raced
-- ladder insert is the honest stale-version conflict). The dictionary
-- words are the CHECK backstops of the frozen Domain Pack vocabulary:
-- the purpose 4, the recipient kind 2 (the executor recipient belongs
-- to Epic 5 — it does not exist here), the nine-word channel mirror of
-- the cases module, the ack requirement 2. The content bound is the
-- byte backstop of the 2000-rune domain limit; the token is 64 hex.
CREATE TABLE cases.external_commitments (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 ext_commitment_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no >= 1),
 purpose text NOT NULL CHECK (purpose IN ('promise','offer','callback','deadline_update')),
 recipient_kind text NOT NULL CHECK (recipient_kind IN ('client','operator')),
 channel text NOT NULL CHECK (channel IN ('phone','web_chat','telegram','whatsapp','email','max','jivo','avito','mobile_app_chat')),
 content_text text NOT NULL CHECK (char_length(content_text) BETWEEN 1 AND 8000),
 valid_until timestamptz NOT NULL,
 expected_next_step text NOT NULL CHECK (char_length(expected_next_step) BETWEEN 1 AND 800),
 ack_requirement text NOT NULL CHECK (ack_requirement IN ('required','not_required')),
 pin_commitment_id uuid,
 pin_version_no bigint CHECK (pin_version_no IS NULL OR pin_version_no >= 1),
 pin_content_token text CHECK (pin_content_token IS NULL OR char_length(pin_content_token) = 64),
 owner_kind text NOT NULL CHECK (owner_kind IN ('human','staffy')),
 owner_ref uuid NOT NULL,
 owner_epoch bigint NOT NULL CHECK (owner_epoch >= 1),
 decision_no bigint NOT NULL CHECK (decision_no >= 0),
 profile_version bigint NOT NULL CHECK (profile_version >= 0),
 policy_outcome_no bigint NOT NULL CHECK (policy_outcome_no >= 0),
 content_token text NOT NULL CHECK (char_length(content_token) = 64),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, ext_commitment_id, version_no),
 CONSTRAINT external_commitments_pin_all_or_nothing CHECK (
  (pin_commitment_id IS NULL AND pin_version_no IS NULL AND pin_content_token IS NULL) OR
  (pin_commitment_id IS NOT NULL AND pin_version_no IS NOT NULL AND pin_content_token IS NOT NULL)),
 CONSTRAINT external_commitments_offer_requires_pin CHECK (
  purpose <> 'offer' OR pin_commitment_id IS NOT NULL),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, work_item_id) REFERENCES cases.work_items(tenant_id, work_item_id)
);
CREATE INDEX external_commitments_case ON cases.external_commitments (tenant_id, case_id, created_at);
CREATE INDEX external_commitments_envelope ON cases.external_commitments (tenant_id, ext_commitment_id, version_no);

-- The approvals: one append-only row per approver of one EXACT version.
-- The UNIQUE (tenant, envelope, version, approver) is the silent
-- idempotency of the repeated approval by the same approver; the
-- approver is the CURRENT human owner at the approval moment (the app
-- gate), the step-up evidence is recorded with the row (the honest
-- provenance of the strongest human confirmation of the slice).
CREATE TABLE cases.external_commitment_approvals (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 work_item_id uuid NOT NULL,
 ext_commitment_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no >= 1),
 approval_id uuid NOT NULL,
 approver_id uuid NOT NULL,
 step_up_verified boolean NOT NULL CHECK (step_up_verified),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, approval_id),
 UNIQUE (tenant_id, ext_commitment_id, version_no, approver_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, ext_commitment_id, version_no) REFERENCES cases.external_commitments(tenant_id, ext_commitment_id, version_no)
);
CREATE INDEX external_commitment_approvals_envelope ON cases.external_commitment_approvals (tenant_id, ext_commitment_id, version_no);

-- The dispatch history: one append-only row per monotonic event of one
-- version. The kind CHECK is the seven-word dispatch vocabulary (the
-- queued word is NOT an event kind — the pending outbox record itself
-- is the queued evidence, the dispatched precedent of 2.12). The reason
-- is required exactly for cancelled_stale (the «слова не ушли»
-- material) and forbidden otherwise. The attempt_no bound mirrors the
-- domain MaxDispatchAttempts: 1..3 for the attempt kinds, 0 for the
-- receipt and the gasp kinds. The idempotent_guarantee word records
-- the capability guarantee the adapter declared with the attempt
-- outcome — the drain policy of the NEXT run reads it (the safe retry
-- of the unknown is decided by the recorded guarantee, never by a
-- guess). The details are the safe text only (bounded; the content
-- and the pin material never enter — AC-10).
CREATE TABLE cases.external_dispatch_events (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 ext_commitment_id uuid NOT NULL,
 version_no bigint NOT NULL CHECK (version_no >= 1),
 event_id uuid NOT NULL,
 event_no bigint NOT NULL CHECK (event_no >= 1),
 kind text NOT NULL CHECK (kind IN ('attempt_submitted','attempt_failed','attempt_unknown','receipt_delivered','receipt_acknowledged','receipt_failed','cancelled_stale')),
 reason text,
 attempt_no bigint NOT NULL CHECK (attempt_no BETWEEN 0 AND 3),
 external_correlation_id text CHECK (char_length(external_correlation_id) <= 200),
 idempotent_guarantee boolean NOT NULL DEFAULT false,
 details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 500),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, event_id),
 UNIQUE (tenant_id, ext_commitment_id, version_no, event_no),
 CONSTRAINT external_dispatch_events_reason_shape CHECK (
  (kind = 'cancelled_stale' AND reason IS NOT NULL AND reason IN ('newer_version','expired','owner_epoch_moved','control_mode')) OR
  (kind <> 'cancelled_stale' AND reason IS NULL)),
 CONSTRAINT external_dispatch_events_attempt_shape CHECK (
  (kind IN ('attempt_submitted','attempt_failed','attempt_unknown') AND attempt_no BETWEEN 1 AND 3) OR
  (kind NOT IN ('attempt_submitted','attempt_failed','attempt_unknown') AND attempt_no = 0)),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, ext_commitment_id, version_no) REFERENCES cases.external_commitments(tenant_id, ext_commitment_id, version_no)
);
CREATE INDEX external_dispatch_events_version ON cases.external_dispatch_events (tenant_id, ext_commitment_id, version_no, event_no);
CREATE INDEX external_dispatch_events_case ON cases.external_dispatch_events (tenant_id, case_id, created_at);

-- The queued outbox record joins the bounded vocabulary (the safe
-- payload only: the identifiers, the version pin, the dictionary
-- words, the idempotency key and the content digest — AC-10).
ALTER TABLE cases.outbox_events DROP CONSTRAINT outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence','case.human_requested','case.human_assigned','case.human_acknowledged','case.human_declined','case.human_cancelled','case.human_returned','case.summary_recorded','case.correction_recorded','case.update_promised','external_commitment.queued'));

-- Row level security: every table is tenant-scoped and forced (the
-- membership blindness is the RLS mirror of the 404 gate — AC-1).
ALTER TABLE cases.external_commitments ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.external_commitments FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.external_commitment_approvals ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.external_commitment_approvals FORCE ROW LEVEL SECURITY;
ALTER TABLE cases.external_dispatch_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE cases.external_dispatch_events FORCE ROW LEVEL SECURITY;

CREATE POLICY external_commitments_scope ON cases.external_commitments USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY external_commitment_approvals_scope ON cases.external_commitment_approvals USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY external_dispatch_events_scope ON cases.external_dispatch_events USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the versions, the
-- approvals and the dispatch history are never edited or deleted
-- through the application role (the append-only probe of the matrix
-- pins the absence of the UPDATE/DELETE paths).
GRANT SELECT, INSERT ON cases.external_commitments TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.external_commitments FROM staffy_app;
GRANT SELECT, INSERT ON cases.external_commitment_approvals TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.external_commitment_approvals FROM staffy_app;
GRANT SELECT, INSERT ON cases.external_dispatch_events TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.external_dispatch_events FROM staffy_app;

-- +goose Down
ALTER TABLE cases.outbox_events DROP CONSTRAINT IF EXISTS outbox_events_event_type_check;
ALTER TABLE cases.outbox_events ADD CONSTRAINT outbox_events_event_type_check CHECK (event_type IN ('case.event_accepted','case.connector_ack','case.ownership_changed','case.client_reply_prepared','case.client_reply_dispatched','case.mode_changed','case.disclosure_recorded','case.ai_proposal_submitted','case.policy_evaluated','case.owning_system_observed','case.objection_recorded','case.commitment_version_created','case.commitment_confirmed','case.commitment_declined','case.continuation_link_issued','case.continuation_submitted','case.continuation_human_requested','case.continuation_returned','case.accommodation_recorded','case.route_degradation_recorded','case.client_reply_delivery_evidence','case.human_requested','case.human_assigned','case.human_acknowledged','case.human_declined','case.human_cancelled','case.human_returned','case.summary_recorded','case.correction_recorded','case.update_promised'));
DROP TABLE IF EXISTS cases.external_dispatch_events;
DROP TABLE IF EXISTS cases.external_commitment_approvals;
DROP TABLE IF EXISTS cases.external_commitments;
