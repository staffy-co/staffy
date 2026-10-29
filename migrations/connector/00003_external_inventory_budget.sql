-- +goose Up
-- Story 7-4: the external class inventory budget of the evidence row.
-- The filtered compact JSON inventory of the external MCP probes rides the
-- redacted_result of the EXISTING evidence row (the spec's architectural
-- intent: no new tables, no new columns — the derivation consumes exactly
-- this row) — the 300-character budget of the 1-9 telephony words cannot
-- carry it, so the constraint relaxes to the declared machine reading
-- budget of the class card (256 KiB). The rows stay append-only; the
-- request words keep their 300-character budget.
ALTER TABLE connector.capability_evidence
 DROP CONSTRAINT capability_evidence_redacted_result_check;
ALTER TABLE connector.capability_evidence
 ADD CONSTRAINT capability_evidence_redacted_result_check
 CHECK (char_length(redacted_result) BETWEEN 1 AND 262144);

-- +goose Down
ALTER TABLE connector.capability_evidence
 DROP CONSTRAINT capability_evidence_redacted_result_check;
ALTER TABLE connector.capability_evidence
 ADD CONSTRAINT capability_evidence_redacted_result_check
 CHECK (char_length(redacted_result) BETWEEN 1 AND 300);
