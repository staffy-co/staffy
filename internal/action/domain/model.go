package domain

import "errors"

// Module action — the AD-6 dictionary core of Authorized Business Actions.
// This file owns the compile-time dictionaries of story 3.1 slice 1: the
// enumerated condition words and the delivery classification. The words are
// frozen in code; runtime or tenant-config extension is NEVER allowed.
//
// Dictionary sources (mirrors below are pinned by the integration contract
// test against the living sources — drift turns the test red; the action
// domain does NOT import cases/employee/connector/identity domain packages):
//   - AutonomyClass words  — PRD FR-42
//   - Conversation modes   — cases ladder 2-5 (ModeOrder)
//   - Roles                — identity domain
//   - OwningSystem/MaterialField/Freshness — cases observation matrix 2-7
//   - Gate IDs             — GATE-REGISTRY (OQ-1..OQ-19 with 7A/7B/13A/13B, AG-1..AG-4)
//   - Stages               — GATE-REGISTRY traffic stages

var (
	// ErrInvalid is the umbrella for every rejected action definition.
	ErrInvalid = errors.New("action definition is invalid")
	// ErrNotFound is the honest 404 of the catalog read: a non-member
	// of the tenant, or an unknown definition pin. The transport maps
	// it (the connector domain precedent).
	ErrNotFound = errors.New("action definition not found")
	// ErrForbidden is the 403 of the read scope gate: a member without
	// the integrations|configuration scopes never reads the action
	// catalog. The scope words are attached at the wrap point in the
	// app layer (the decision-context 2-14 precedent).
	ErrForbidden = errors.New("action catalog read is not allowed")
)

// RuleError keeps a precise, user-safe reason for a rejected definition
// (precedent: connector domain 1-8). Every guard branch fails closed with
// one of these — «отсутствие обязательного условия = запрет».
type RuleError struct{ Rule, Reason string }

func (e RuleError) Error() string { return e.Rule + ": " + e.Reason }
func (e RuleError) Unwrap() error { return ErrInvalid }

// ---------------------------------------------------------------------------
// The condition dictionaries (compile-time, closed)
// ---------------------------------------------------------------------------

// RiskClass words.
const (
	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"
	RiskCritical = "critical"
)

// RiskClasses is the full dictionary.
var RiskClasses = []string{RiskLow, RiskMedium, RiskHigh, RiskCritical}

// AutonomyClass words — exactly the FR-42 vocabulary.
const (
	AutonomyShadow    = "shadow"
	AutonomyCopilot   = "copilot"
	AutonomyAssisted  = "assisted"
	AutonomyAuto      = "auto"
	AutonomyException = "exception"
)

// AutonomyClasses is the full dictionary.
var AutonomyClasses = []string{AutonomyShadow, AutonomyCopilot, AutonomyAssisted, AutonomyAuto, AutonomyException}

// ConfirmationRequirement words. Mutating capability classes are forced to
// required by the guard (confirm-before-mutate, AD-6); not_required survives
// only on pure query classes.
const (
	ConfirmationRequired    = "required"
	ConfirmationNotRequired = "not_required"
)

// ConfirmationRequirements is the full dictionary.
var ConfirmationRequirements = []string{ConfirmationRequired, ConfirmationNotRequired}

// MaterialityClass words.
const (
	MaterialityImmaterial = "immaterial"
	MaterialityMaterial   = "material"
	MaterialityCritical   = "critical"
)

// MaterialityClasses is the full dictionary.
var MaterialityClasses = []string{MaterialityImmaterial, MaterialityMaterial, MaterialityCritical}

// ConsumptionRule words.
const (
	ConsumptionSingleUse = "single_use"
	ConsumptionBounded   = "bounded"
	ConsumptionUnlimited = "unlimited"
)

// ConsumptionRules is the full dictionary.
var ConsumptionRules = []string{ConsumptionSingleUse, ConsumptionBounded, ConsumptionUnlimited}

// CapabilityClass words — the delivery classification of AC-3. The class is
// not self-consistent by declaration: the app layer cross-checks it against
// the provider CapabilityDeclaration (1-8) through resolvers.
const (
	CapabilityProviderIdempotent = "provider_idempotent"
	CapabilityQueryReconcilable  = "query_reconcilable"
	CapabilityCompensatable      = "compensatable"
	CapabilityUnsafe             = "unsafe"
)

// CapabilityClasses is the full dictionary.
var CapabilityClasses = []string{CapabilityProviderIdempotent, CapabilityQueryReconcilable, CapabilityCompensatable, CapabilityUnsafe}

// Stage words — the GATE-REGISTRY traffic stages.
const (
	StageSynthetic              = "synthetic"
	StageClosedAccountTest      = "closed_account_test"
	StageControlledRealCustomer = "controlled_real_customer"
	StageLive                   = "live"
	StageScale                  = "scale"
)

// AllowedStages is the full dictionary.
var AllowedStages = []string{StageSynthetic, StageClosedAccountTest, StageControlledRealCustomer, StageLive, StageScale}

// Timeout bounds: one action execution is bounded 1 ms .. 10 minutes.
const (
	TimeoutMSMin = 1
	TimeoutMSMax = 600000
)

// ---------------------------------------------------------------------------
// The mirrors (pinned by contract tests against the source modules)
// ---------------------------------------------------------------------------

// ConversationModeWords mirrors the conversation-mode ladder of 2-5 (7 words,
// strictest first). Mirror only: the action domain never imports the cases
// domain; the integration contract test pins this slice against ModeOrder.
var ConversationModeWords = []string{
	"safety",
	"claim",
	"cancellation",
	"privacy_no_contact",
	"human_requested",
	"unclassified",
	"sales",
}

// RoleWords mirrors the identity role dictionary (5 words).
var RoleWords = []string{
	"owner",
	"configurator",
	"operator",
	"expert_qa",
	"integration_admin",
}

// OwningSystemWords mirrors the owning-system dictionary of 2-7.
var OwningSystemWords = []string{
	"catalog",
	"erp",
}

// MaterialFieldWords mirrors the material-field dictionary of 2-7.
var MaterialFieldWords = []string{
	"price",
	"availability",
	"order_status",
}

// FreshnessWords mirrors the read-time observation-state words of 2-7
// (the honest derivation words: failed and unknown never dress up as fresh).
var FreshnessWords = []string{
	"fresh",
	"stale",
	"failed",
	"unknown",
}

// GateIDWords mirrors the GATE-REGISTRY gate identifiers. An action declares
// the gates that must PASS before it may run; the registry itself stays the
// canonical source.
var GateIDWords = []string{
	"OQ-1", "OQ-2", "OQ-3", "OQ-4", "OQ-5", "OQ-6", "OQ-7A", "OQ-7B", "OQ-8",
	"OQ-9", "OQ-10", "OQ-11", "OQ-12", "OQ-13A", "OQ-13B", "OQ-14", "OQ-15",
	"OQ-16", "OQ-17", "OQ-18", "OQ-19",
	"AG-1", "AG-2", "AG-3", "AG-4",
}

// WordIn reports whether w belongs to the closed dictionary.
func WordIn(dictionary []string, w string) bool {
	for _, word := range dictionary {
		if w == word {
			return true
		}
	}
	return false
}
