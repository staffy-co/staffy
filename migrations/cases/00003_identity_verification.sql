-- +goose Up
-- Story 2.3: identity verification and safe channel binding (first slice).
-- The cases schema stays append-only: verification_records, channel_bindings,
-- binding_reversals, identifier_tokens, identifier_hmac_keys,
-- duplicate_candidates and duplicate_resolutions carry INSERT/SELECT grants
-- only. The assurance state of a case is the read-time derivation of the
-- latest unexpired passed record per kind; the ACTIVE binding is the
-- anti-join with the append-only reversals; the duplicate candidate state is
-- the existence of a resolution. A conversation NEVER moves between cases
-- (the binding is a separate link; the events of the bound conversation keep
-- attaching by the unchanged 2-1 session continuation rule — the unified
-- case view is the read-time JOIN). There is no column for a one-time code,
-- a password or a secret anywhere: the canonical record stores the outcome,
-- never the verification material.

-- The verification records: kind (identity/authority checked SEPARATELY —
-- a confirmed identity never implies authority), the method, the CANONICAL
-- outcome (the provider code is mapped by the domain, never stored), the
-- assurance level (the method level when passed, none otherwise), the
-- declared subject handle (never a proof), the issuing operator, the
-- evidence (an accepted event of the case XOR a note), the authority scope
-- (the authority kind only) and both times with the computed expiry.
CREATE TABLE cases.verification_records (
 tenant_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 verification_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('identity','authority')),
 method text NOT NULL CHECK (method IN ('knowledge_check','known_number_callback','known_address_reply','mandate_note')),
 outcome text NOT NULL CHECK (outcome IN ('passed','failed','expired','cancelled','ambiguous')),
 assurance_level text NOT NULL CHECK (assurance_level IN ('none','basic','verified')),
 subject_handle text NOT NULL CHECK (char_length(subject_handle) BETWEEN 1 AND 64),
 issuer_id uuid NOT NULL,
 evidence_event_id uuid,
 note text CHECK (note IS NULL OR char_length(note) BETWEEN 1 AND 200),
 authority_scope text CHECK (authority_scope IS NULL OR char_length(authority_scope) BETWEEN 1 AND 200),
 issued_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, verification_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id),
 FOREIGN KEY (tenant_id, evidence_event_id) REFERENCES cases.channel_events(tenant_id, event_id),
 CONSTRAINT verification_evidence_required CHECK (
   (evidence_event_id IS NOT NULL AND note IS NULL) OR
   (evidence_event_id IS NULL AND note IS NOT NULL)),
 CONSTRAINT verification_scope_of_kind CHECK (
   (kind = 'authority' AND authority_scope IS NOT NULL) OR
   (kind = 'identity' AND authority_scope IS NULL))
);
CREATE INDEX verification_records_case_window ON cases.verification_records (tenant_id, case_id, issued_at);

-- The channel bindings: the target conversation X of the case A and the
-- bound conversation Y (its own case B stays the owner of record — the FK
-- never moves). The consent handoff requires the purpose and one evidence;
-- the duplicate review carries its basis in the resolution row.
CREATE TABLE cases.channel_bindings (
 tenant_id uuid NOT NULL,
 binding_id uuid NOT NULL,
 case_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 bound_case_id uuid NOT NULL,
 bound_conversation_id uuid NOT NULL,
 method text NOT NULL CHECK (method IN ('consent_handoff','duplicate_review')),
 purpose text CHECK (purpose IS NULL OR char_length(purpose) BETWEEN 1 AND 200),
 on_behalf boolean NOT NULL,
 subject_handle text NOT NULL CHECK (char_length(subject_handle) BETWEEN 1 AND 64),
 evidence_event_id uuid,
 note text CHECK (note IS NULL OR char_length(note) BETWEEN 1 AND 200),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, binding_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id),
 FOREIGN KEY (tenant_id, bound_case_id) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, bound_conversation_id) REFERENCES cases.conversations(tenant_id, conversation_id),
 FOREIGN KEY (tenant_id, evidence_event_id) REFERENCES cases.channel_events(tenant_id, event_id),
 CONSTRAINT binding_two_conversations CHECK (case_id <> bound_case_id),
 CONSTRAINT binding_evidence_xor CHECK (
   (evidence_event_id IS NULL) OR (note IS NULL)),
 CONSTRAINT binding_consent_material CHECK (
   (method = 'consent_handoff' AND purpose IS NOT NULL AND (evidence_event_id IS NOT NULL OR note IS NOT NULL)) OR
   (method = 'duplicate_review'))
);
CREATE INDEX channel_bindings_target ON cases.channel_bindings (tenant_id, case_id, created_at);
CREATE INDEX channel_bindings_bound ON cases.channel_bindings (tenant_id, bound_conversation_id);

-- The append-only unlink: the ACTIVE binding is the anti-join with this
-- table. One reversal per binding (the UNIQUE is the race backstop — the
-- honest conflict of a concurrent unlink).
CREATE TABLE cases.binding_reversals (
 tenant_id uuid NOT NULL,
 reversal_id uuid NOT NULL,
 binding_id uuid NOT NULL,
 reason text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 200),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, reversal_id),
 UNIQUE (tenant_id, binding_id),
 FOREIGN KEY (tenant_id, binding_id) REFERENCES cases.channel_bindings(tenant_id, binding_id)
);

-- The per-tenant identifier key of the keyed-HMAC tokens (AD-10: raw
-- unsalted hashes of phone numbers are forbidden). INSERT-only; the key
-- never leaves the module scope. The rotation/destruction follows the
-- retention policy — honestly deferred (deferred-work).
CREATE TABLE cases.identifier_hmac_keys (
 tenant_id uuid NOT NULL,
 key bytea NOT NULL CHECK (octet_length(key) = 32),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id)
);

-- The identifier tokens of the identifier facts: one row per (case, token).
-- The VALUE never lands here — the HMAC token only. The exact match across
-- two open cases is the material of the duplicate candidates.
CREATE TABLE cases.identifier_tokens (
 tenant_id uuid NOT NULL,
 token char(64) NOT NULL CHECK (char_length(token) = 64),
 case_id uuid NOT NULL,
 material_key text NOT NULL CHECK (char_length(material_key) BETWEEN 1 AND 64),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, token, case_id),
 FOREIGN KEY (tenant_id, case_id) REFERENCES cases.cases(tenant_id, case_id)
);
CREATE INDEX identifier_tokens_token ON cases.identifier_tokens (tenant_id, token);

-- The duplicate candidates: one row per match of the material pair (the
-- new case, the existing case, the token) — several possible matches of
-- one token stay VISIBLE to the human review (AC-7: the task shows every
-- option; the automation never merges). The match reason and the rule
-- confidence are the fixed dictionary (the exact identifier match, high).
CREATE TABLE cases.duplicate_candidates (
 tenant_id uuid NOT NULL,
 candidate_id uuid NOT NULL,
 case_id_new uuid NOT NULL,
 case_id_existing uuid NOT NULL,
 token char(64) NOT NULL CHECK (char_length(token) = 64),
 material_key text NOT NULL CHECK (char_length(material_key) BETWEEN 1 AND 64),
 match_reason text NOT NULL CHECK (match_reason IN ('exact_identifier')),
 rule_confidence text NOT NULL CHECK (rule_confidence IN ('high')),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, candidate_id),
 UNIQUE (tenant_id, case_id_new, case_id_existing, token),
 CHECK (case_id_new <> case_id_existing),
 FOREIGN KEY (tenant_id, case_id_new) REFERENCES cases.cases(tenant_id, case_id),
 FOREIGN KEY (tenant_id, case_id_existing) REFERENCES cases.cases(tenant_id, case_id)
);
CREATE INDEX duplicate_candidates_existing ON cases.duplicate_candidates (tenant_id, case_id_existing);

-- The duplicate resolutions: one append-only decision per candidate (the
-- first resolution wins; the UNIQUE is the race backstop). The same-client
-- decision creates the duplicate-review binding in the SAME transaction of
-- the command — the atomicity of "decision + link".
CREATE TABLE cases.duplicate_resolutions (
 tenant_id uuid NOT NULL,
 resolution_id uuid NOT NULL,
 candidate_id uuid NOT NULL,
 decision text NOT NULL CHECK (decision IN ('same_client','separate')),
 note text NOT NULL CHECK (char_length(note) BETWEEN 1 AND 200),
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (tenant_id, resolution_id),
 UNIQUE (tenant_id, candidate_id),
 FOREIGN KEY (tenant_id, candidate_id) REFERENCES cases.duplicate_candidates(tenant_id, candidate_id)
);

-- The four new commands join the idempotency surface (the 2-2 precedent:
-- the stored outcome replays an exact repeat, the same key with another
-- request hash is an honest conflict).
ALTER TABLE cases.command_records DROP CONSTRAINT command_records_command_check;
ALTER TABLE cases.command_records ADD CONSTRAINT command_records_command_check CHECK (command IN ('create_work_item','classify_work_item','transition_work_item','split_work_item','merge_work_items','link_event','dispute_work_item','record_fact','confirm_fact_version','rescope_fact','start_verification','bind_channel','unlink_channel','resolve_duplicate'));

-- The step-up exception of the threshold window (the values of the
-- identifiers and the secrets never enter the exception).
ALTER TABLE cases.operational_exceptions DROP CONSTRAINT operational_exceptions_kind_check;
ALTER TABLE cases.operational_exceptions ADD CONSTRAINT operational_exceptions_kind_check CHECK (kind IN ('digest_conflict','ambiguous_employee','idempotency_key_conflict','work_item_dispute','identity_step_up'));

CREATE POLICY verification_records_scope ON cases.verification_records USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY channel_bindings_scope ON cases.channel_bindings USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY binding_reversals_scope ON cases.binding_reversals USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY identifier_hmac_keys_scope ON cases.identifier_hmac_keys USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY identifier_tokens_scope ON cases.identifier_tokens USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY duplicate_candidates_scope ON cases.duplicate_candidates USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);
CREATE POLICY duplicate_resolutions_scope ON cases.duplicate_resolutions USING (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting('staffy.tenant_id',true),'')::uuid);

-- Grants: the append-only surface (INSERT/SELECT); the history is not
-- edited. The identifier key table carries the same grants — the key is
-- read under the tenant scope only and never leaves the module.
GRANT SELECT, INSERT ON cases.verification_records TO staffy_app;
GRANT SELECT, INSERT ON cases.channel_bindings TO staffy_app;
GRANT SELECT, INSERT ON cases.binding_reversals TO staffy_app;
GRANT SELECT, INSERT ON cases.identifier_hmac_keys TO staffy_app;
GRANT SELECT, INSERT ON cases.identifier_tokens TO staffy_app;
GRANT SELECT, INSERT ON cases.duplicate_candidates TO staffy_app;
GRANT SELECT, INSERT ON cases.duplicate_resolutions TO staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.verification_records FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.channel_bindings FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.binding_reversals FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.identifier_hmac_keys FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.identifier_tokens FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.duplicate_candidates FROM staffy_app;
REVOKE UPDATE, DELETE, TRUNCATE ON cases.duplicate_resolutions FROM staffy_app;

-- +goose Down
DROP TABLE cases.duplicate_resolutions;
DROP TABLE cases.duplicate_candidates;
DROP TABLE cases.identifier_tokens;
DROP TABLE cases.identifier_hmac_keys;
DROP TABLE cases.binding_reversals;
DROP TABLE cases.channel_bindings;
DROP TABLE cases.verification_records;
