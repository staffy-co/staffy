package planner

// The Turn Proposal (Story 8-1): the versioned, closed, typed form of
// everything a model may propose for one turn — the AD-7 field list as
// a wire contract. The form is strict and versioned: an undeclared key
// is a refusal, a foreign schema_version is a refusal (the coherence
// discipline of the 7-3/7-4 wire surfaces). The proposal carries
// NAMING, never execution material: the proposed
// commitment/action/handoff is a kind plus a contract reference, and
// the reply is a text within the platform budget — a reply past the
// budget is a validation refusal without truncation.

import "encoding/json"

// FactProposal is one fact the proposal suggests to record, with the
// mandatory provenance of the suggestion. A suggestion without
// provenance does not exist.
type FactProposal struct {
	Key        string     `json:"key"`
	Value      any        `json:"value"`
	Provenance Provenance `json:"provenance"`
}

// Contradiction is one observed conflict between an existing fact and
// a suggested value: the key, the existing material, the proposed
// material and the provenance of the proposal. The contradiction is
// REPORTED, never silently resolved — the honest escalation of the
// unknown (§2 of the product star).
type Contradiction struct {
	Key        string     `json:"key"`
	Existing   any        `json:"existing"`
	Proposed   any        `json:"proposed"`
	Provenance Provenance `json:"provenance"`
}

// ReadRequest is one typed read the proposal asks for: the tool
// name@version and the arguments. The request validates against the
// inventory of the CONTEXT: a tool outside the inventory, an argument
// outside the tool's schema, a missing required argument or a value
// past the enum/bounds is a named refusal. The EXECUTION of reads is
// Story 8.3 — this form only asks.
type ReadRequest struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// Question is one clarifying question of the proposal, within the
// platform reply budget.
type Question struct {
	Text string `json:"text"`
}

// ProposedAction is the closed union of what the proposal proposes to
// do: none | commitment | action | handoff. A kind other than none
// REQUIRES a contract reference (id@version) — the proposal only names
// the thing it proposes; the material snapshot, the confirmation and
// the execution live in the existing commitment/action contours
// (Stories 2.9/3-x, wired to the planner by 8.4 — AD-6).
type ProposedAction struct {
	Kind        string `json:"kind"`
	ContractRef string `json:"contract_ref,omitempty"`
}

// Reply is the reply material of the proposal: one text within the
// platform budget.
type Reply struct {
	Text string `json:"text"`
}

// EvidenceRef is one evidence reference of the proposal: the type of
// the evidence and its id (a fact key, a tool name@version, a Work
// Item, a message — the type word is honest material of the
// proposer; the resolution is the caller's).
type EvidenceRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// MemoryPatchEntry is one versioned-record form of the proposed
// Case-memory patch (AD-8/AD-10): a key, a value and the provenance of
// the entry. The APPLICATION of the patch is Story 8.4 — the proposal
// only carries the closed form.
type MemoryPatchEntry struct {
	Key        string     `json:"key"`
	Value      any        `json:"value"`
	Provenance Provenance `json:"provenance"`
}

// TurnProposal is the closed form of the model output.
type TurnProposal struct {
	SchemaVersion  int                `json:"schema_version"`
	Intent         string             `json:"intent"`
	Mode           string             `json:"mode"`
	Safety         string             `json:"safety"`
	FactProposals  []FactProposal     `json:"fact_proposals"`
	Contradictions []Contradiction    `json:"contradictions"`
	ReadRequests   []ReadRequest      `json:"read_requests"`
	Questions      []Question         `json:"questions"`
	ProposedAction ProposedAction     `json:"proposed_action"`
	Reply          Reply              `json:"reply"`
	EvidenceRefs   []EvidenceRef      `json:"evidence_refs"`
	MemoryPatch    []MemoryPatchEntry `json:"memory_patch"`
}

// DecodeTurnProposal strictly decodes the raw model output into the
// proposal form and checks the version coherence: an undeclared key, a
// wrong type or a foreign schema_version is a NAMED refusal (the
// machine hands the cause to the repair round; the model output is
// never repaired silently).
func DecodeTurnProposal(raw []byte) (TurnProposal, *RuleError) {
	var proposal TurnProposal
	if err := strictDecode(raw, &proposal); err != nil {
		return TurnProposal{}, rule("proposal.strict_decode", "предложение хода не прошло строгий decode: %v", err.Error())
	}
	if proposal.SchemaVersion != ProposalSchemaVersion {
		return TurnProposal{}, rule("proposal.schema_version", "версия формы предложения %d чужая — машина поддерживает только %d (когерентность версий)", proposal.SchemaVersion, ProposalSchemaVersion)
	}
	return proposal, nil
}

// compactJSON marshals the validated proposal into the compact JSONB
// material of the journal record (business material behind the tenant
// RLS — the reply-string spirit of Story 2.12, not telemetry).
func compactJSON(proposal TurnProposal) ([]byte, error) {
	return json.Marshal(proposal)
}
