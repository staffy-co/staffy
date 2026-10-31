package planner

// The round material (Story 8-2): the versioned, deterministic, closed
// prompt-assembly form — the ONLY material a model ever sees. The
// assembly is PLATFORM code (never adapter code): an adapter that
// never receives the Turn Context cannot assemble an untracked prompt
// of its own — the value of the story is exactly that ("никогда не
// зависел от неотслеживаемого промпта").
//
// The material is NOT persisted (neither the journal nor a separate
// table): its verifiability is determinism plus version — the same
// validated context and repair cause always assemble byte-identical
// material, and the journal record carries the assembly version word
// that produced the round (AD-10/AD-11: persisting the material would
// duplicate the PII-bearing business material of the case). No secrets
// exist in the form (AD-11), no hidden chain of thought (AD-8), no
// retrieval slot — the extracted-knowledge input does not exist in
// this slice (the retrieval port is a future surface with its own
// spec); the operational facts of the case ride with their full
// provenance and are never substitutable (FR-20).
//
// The §8.1 AI-disclosure pin of the context travels INTO the material:
// the model is required to see the disclosure requirement — the
// contract-side half of the story; the behavioural half is measured by
// the eval set of Story 8.5.

import (
	"encoding/json"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// MaterialRefs are the turn references the material carries: the ids
// of the turn's scope (tenant, case, conversation, Work Item,
// employee, turn) and the release pins (Release Manifest, domain
// pack) as id@version references.
type MaterialRefs struct {
	TurnID             id.UUID  `json:"turn_id"`
	TenantID           id.UUID  `json:"tenant_id"`
	CaseID             id.UUID  `json:"case_id"`
	ConversationID     id.UUID  `json:"conversation_id"`
	WorkItemID         *id.UUID `json:"work_item_id,omitempty"`
	EmployeeID         id.UUID  `json:"employee_id"`
	ReleaseManifestRef string   `json:"release_manifest"`
	PackRef            string   `json:"pack"`
}

// ProposalFormGuide carries the closed answer-form of the contract to
// the model: the proposal schema version and the closed dictionaries
// the proposal words must come from. The mode dictionary IS the ladder
// (already in the material); the reply budget is carried next to the
// guide so a model knows the size discipline before it writes.
type ProposalFormGuide struct {
	SchemaVersion        int      `json:"schema_version"`
	IntentDictionary     []string `json:"intent_dictionary"`
	SafetyDictionary     []string `json:"safety_dictionary"`
	ActionKindDictionary []string `json:"action_kind_dictionary"`
}

// RoundMaterial is the closed form of one model round's input.
type RoundMaterial struct {
	// AssemblyVersion is the version word of the assembly form: a
	// build change means the records of the new word carry the new
	// word, and the prior records stay as they are (append-only,
	// AD-10).
	AssemblyVersion string       `json:"assembly_version"`
	Refs            MaterialRefs `json:"refs"`
	// Facts are the confirmed facts of the snapshot with their FULL
	// provenance — authority/source/version/time (AD-8): the model
	// reasons over evidence, never over unattributed material.
	Facts []ContextFact `json:"facts"`
	// Mode and Ladder are the conversation mode and the active
	// priority ladder (the exact platform mirror).
	Mode   string   `json:"mode"`
	Ladder []string `json:"ladder"`
	// PolicyPins travel as id@version REFERENCES: the material behind
	// the pins never enters the material (the policy material
	// assembly is a future surface).
	PolicyPins PolicyPins `json:"policy_pins"`
	// Tools is the read-tool inventory: what the model may REQUEST
	// (the execution of reads is Story 8.3).
	Tools []InventoryTool `json:"tools"`
	// AIDisclosureRequired is the §8.1 pin: the model is required to
	// see the disclosure requirement of the conversation.
	AIDisclosureRequired bool `json:"ai_disclosure_required"`
	// FallbackPolicy is the declared fallback policy of the turn.
	FallbackPolicy string `json:"fallback_policy"`
	// ReplyBudgetRunes is the platform reply budget the proposal must
	// respect (a reply past the budget is a validation refusal).
	ReplyBudgetRunes int `json:"reply_budget_runes"`
	// ProposalForm is the closed answer-form guide.
	ProposalForm ProposalFormGuide `json:"proposal_form"`
	// Repair carries the NAMED cause of the first round's refusal on
	// the repair round (empty on round 1): the model learns WHY its
	// material was refused — never a silent fix.
	Repair string `json:"repair,omitempty"`
}

// AssembleRoundMaterial builds the round material of the given
// validated turn context: a pure, deterministic function — the same
// context and repair cause always produce the same material. The
// repair cause is empty on round 1 and carries the named refusal on
// the repair round.
func AssembleRoundMaterial(context TurnContext, repair string) RoundMaterial {
	return RoundMaterial{
		AssemblyVersion: AssemblyVersion,
		Refs: MaterialRefs{
			TurnID:             context.TurnID,
			TenantID:           context.TenantID,
			CaseID:             context.CaseID,
			ConversationID:     context.ConversationID,
			WorkItemID:         context.WorkItemID,
			EmployeeID:         context.EmployeeID,
			ReleaseManifestRef: context.ReleaseManifestRef,
			PackRef:            context.PackRef,
		},
		Facts:  append([]ContextFact(nil), context.Facts...),
		Mode:   context.Mode,
		Ladder: append([]string(nil), context.Ladder...),
		PolicyPins: PolicyPins{
			PolicyBundleRef:  context.PolicyPins.PolicyBundleRef,
			AllowedClaimsRef: context.PolicyPins.AllowedClaimsRef,
			AutonomyRef:      context.PolicyPins.AutonomyRef,
		},
		Tools:                append([]InventoryTool(nil), context.Tools...),
		AIDisclosureRequired: context.AIDisclosureRequired,
		FallbackPolicy:       context.FallbackPolicy,
		ReplyBudgetRunes:     ReplyBudgetRunes,
		ProposalForm: ProposalFormGuide{
			SchemaVersion:        ProposalSchemaVersion,
			IntentDictionary:     append([]string(nil), IntentDictionary...),
			SafetyDictionary:     append([]string(nil), SafetyDictionary...),
			ActionKindDictionary: append([]string(nil), ActionKindDictionary...),
		},
		Repair: repair,
	}
}

// Bytes marshals the material deterministically: struct fields marshal
// in declaration order, map keys sort — the byte identity of one
// assembly is the verifiability of the form.
func (m RoundMaterial) Bytes() ([]byte, error) {
	return json.Marshal(m)
}
