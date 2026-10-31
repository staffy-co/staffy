-- +goose Up
-- The turn provenance extension of the planner journal (Story 8-2):
-- the records of turns run by the 8-2 machine carry the assembly
-- version word that produced their rounds (the versioned prompt
-- assembly — platform code, deterministic, never persisted as
-- material) and the declared budgets of the turn (rounds, round
-- timeout, cost units — the bounded side of AD-7). The columns are
-- NULL on the pre-8-2 history — the HONEST ABSENCE: those turns ran
-- without the assembly surface, and sealed records are never
-- recomputed (AD-10). No CHECK dictionary freezes the version words:
-- a version word EVOLVES by design (a word change means the records
-- of the new word carry the new word — the connectorprobe build
-- semantics); the closed form of the budgets is the platform's Go
-- side. The append-only discipline of 00001 is inherited untouched:
-- the runtime role still holds SELECT and INSERT only.

ALTER TABLE planner.turns ADD COLUMN assembly_version text;
ALTER TABLE planner.turns ADD COLUMN budgets jsonb;

-- +goose Down
ALTER TABLE planner.turns DROP COLUMN IF EXISTS budgets;
ALTER TABLE planner.turns DROP COLUMN IF EXISTS assembly_version;
