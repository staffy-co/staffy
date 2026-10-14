package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The progressive intake and the deterministic Policy Outcome (Story 2.6)
// ---------------------------------------------------------------------------
//
// The AI proposal is a typed CLAIM of a proposer (the synthetic port of
// the slice or the operator — the future AI Runtime passes through the
// same command): the intent, the candidate answer, the missing facts and
// the extracted facts with their source spans. The submission validates
// the structure and the bounds ONLY — the intent dictionary is a POLICY
// matter, never a submission matter: an unknown intent reaches the
// evaluation and fails closed as an outcome (the model result is a
// proposal, not a Policy Outcome). The facts of the extracted claims are
// never written automatically — the only fact writer is the 2-2 command
// with the full provenance.
//
// The Policy Outcome is the deterministic server-side decision over the
// anchored input versions: the proposal, the confirmed fact set of the
// case, the effective Work Item mode and the rule table of the fixed
// Domain Pack v0. The outcomes are append-only with the per-Work-Item
// monotonic number; the CURRENT outcome is the maximal number — a
// re-evaluation supersedes by a NEW row, never by an update. Every
// outcome carries the full input snapshot and the decision digest: the
// same anchors produce the same digest (the reproducibility by the input
// versions, AC-3).
//
// The sales reply gate is load-bearing: a sales-class client reply needs
// the Work Item anchor AND the current outcome of that item to be
// commercial_answer — otherwise the machine-readable refusal names the
// current outcome words and the permitted path. The gate order of
// send_client_reply stays frozen: ownership 2-4 → mode 2-5 → disclosure
// 2-5 → policy 2-6 → CAS.

// ---------------------------------------------------------------------------
// Dictionaries
// ---------------------------------------------------------------------------

// ProposalSchemaVersion is the fixed typed schema word of every AI
// proposal of the slice (the versioned contract of the model result).
const ProposalSchemaVersion = "ai-proposal-v1"

// PolicyRuleVersion is the fixed rule dictionary word every outcome
// carries (the deterministic policy of the slice). The v2 word of Story
// 2.11: the evaluation input gains the fourth entry (the preferences
// state) and the decision gains the active alternatives — the honest
// form versioning (the historical v1 rows stay readable, never
// recomputed).
const PolicyRuleVersion = "intake-policy-v2"

// The proposal intents: the fixed dictionary of the slice. The COMMERCIAL
// words open the sales path; the rest are the safe words.
const (
	IntentGeneralQuestion = "general_question"
	IntentServiceRequest  = "service_request"
	IntentPriceInquiry    = "price_inquiry"
	IntentBookingRequest  = "booking_request"
	IntentHumanRequest    = "human_request"
)

// ProposalIntents is the full dictionary.
var ProposalIntents = []string{IntentGeneralQuestion, IntentServiceRequest, IntentPriceInquiry, IntentBookingRequest, IntentHumanRequest}

// CommercialIntents is the commercial subset of the intent dictionary
// (the sales-path words).
var CommercialIntents = []string{IntentServiceRequest, IntentPriceInquiry, IntentBookingRequest}

// IntentIsCommercial reports whether the intent word is commercial.
func IntentIsCommercial(intent string) bool {
	for _, word := range CommercialIntents {
		if word == intent {
			return true
		}
	}
	return false
}

// IntentInDictionary reports whether the intent word belongs to the fixed
// dictionary (the fail-closed basis of the unknown-intent outcome).
func IntentInDictionary(intent string) bool {
	return inDictionary(ProposalIntents, intent)
}

// The outcome classes: the fixed dictionary of the deterministic policy.
// commercial_answer is the ONLY class that permits the sales reply.
const (
	OutcomeCommercialAnswer    = "commercial_answer"
	OutcomeInformationalAnswer = "informational_answer"
	OutcomeClarify             = "clarify"
	OutcomeModeBlocked         = "mode_blocked"
	OutcomeFailClosed          = "fail_closed"
)

// OutcomeClasses is the full dictionary.
var OutcomeClasses = []string{OutcomeCommercialAnswer, OutcomeInformationalAnswer, OutcomeClarify, OutcomeModeBlocked, OutcomeFailClosed}

// The outcome reasons: the fixed dictionary. mandatory_missing is the
// reserved word of the rich Domain Pack rules (Epic 5): the v1 table
// never emits it — the progressive commercial answer carries the open
// questions in next_steps instead of failing the class; the word stays in
// the dictionary so the future rules never rename the vocabulary.
// no_alternative_path is the honest fail-closed stop of Story 2.11: a
// media-usual mandatory key whose pack data carries no permitted
// alternative — the check never weakens, the path hands off to a
// human.
const (
	ReasonOkCommercial         = "ok_commercial"
	ReasonOkInformational      = "ok_informational"
	ReasonMandatoryMissing     = "mandatory_missing"
	ReasonNoAnswerBasis        = "no_answer_basis"
	ReasonModeBlocksCommercial = "mode_blocks_commercial_path"
	ReasonNoAlternativePath    = "no_alternative_path"
	ReasonUnknownIntent        = "unknown_intent"
)

// OutcomeReasons is the full dictionary.
var OutcomeReasons = []string{ReasonOkCommercial, ReasonOkInformational, ReasonMandatoryMissing, ReasonNoAnswerBasis, ReasonModeBlocksCommercial, ReasonNoAlternativePath, ReasonUnknownIntent}

// The next-step words of the outcome: the finite plan vocabulary.
const (
	StepAskFact           = "ask_fact"
	StepAwaitConfirmation = "await_confirmation"
	StepHumanHandoff      = "human_handoff"
)

// PolicySteps is the full dictionary.
var PolicySteps = []string{StepAskFact, StepAwaitConfirmation, StepHumanHandoff}

// The proposers: the synthetic port of the slice (the future AI Runtime
// passes through the same command) and the operator.
const (
	ProposerSyntheticPort = "synthetic_port"
	ProposerOperator      = "operator"
)

// ProposalProposers is the full dictionary.
var ProposalProposers = []string{ProposerSyntheticPort, ProposerOperator}

// The missing-fact reasons: a mandatory gap of the rule or the explicit
// readback need (the only re-ask reason of a confirmed fact in v1).
const (
	MissingReasonMandatory = "mandatory"
	MissingReasonReadback  = "readback"
)

// MissingReasons is the full dictionary.
var MissingReasons = []string{MissingReasonMandatory, MissingReasonReadback}

// The mandatory material keys of the rule table v1 (the fixed Domain Pack
// v0): the rich per-pack rules are the Epic 5 boundary. The keys are the
// normalized 2-2 material words.
var ruleTableV1 = map[string][]string{
	IntentGeneralQuestion: {},
	IntentHumanRequest:    {},
	IntentServiceRequest:  {"service_kind"},
	IntentPriceInquiry:    {"service_kind", "urgency"},
	IntentBookingRequest:  {"slot_preference"},
}

// MandatoryFactsOfIntent resolves the rule-table mandatory material keys
// of one intent (empty for an unknown intent — the fail-closed class
// carries no question plan).
func MandatoryFactsOfIntent(intent string) []string {
	keys := ruleTableV1[intent]
	if keys == nil {
		return nil
	}
	return append([]string(nil), keys...)
}

// The material limits of the proposal dictionary (the material key
// limit is the shared 2-2 dictionary word of work.go).
const (
	MaxIntentRunes          = 64
	MaxCandidateAnswerRunes = 2000
	MaxValueDigestRunes     = 128
)

// The permitted-path words of the policy gate of the sales reply.
const (
	PermittedPathAnchorWorkItem = "anchor_work_item"
	PermittedPathSubmitProposal = "submit_ai_proposal"
	PermittedPathEvaluatePolicy = "evaluate_policy"
)

// ---------------------------------------------------------------------------
// The proposal material
// ---------------------------------------------------------------------------

// MissingFact is one claim of a missing fact: the normalized material key
// and the reason word. The mandatory reason joins the question plan; the
// readback reason is the explicit need to say the value aloud — the only
// re-ask word of a confirmed fact in v1.
type MissingFact struct {
	MaterialKey string
	Reason      string
}

// ExtractedFact is one claim of an extracted fact: the material key, the
// value digest and the accepted event of the case the claim spans. The
// claim is an AUDIT artifact — the facts are never written from it (the
// only fact writer is the 2-2 command with the full provenance).
type ExtractedFact struct {
	MaterialKey   string
	ValueDigest   string
	SourceEventID id.UUID
}

// AIProposal is one append-only typed proposal: the Work Item scope, the
// accepted turn it answers, the intent claim, the bounded candidate
// answer (possibly empty — no answer basis), the missing-fact claims with
// their reasons, the extracted-fact claims with their spans, the proposer
// word and the input snapshot of the submission moment (the owner epoch
// and the mode version).
type AIProposal struct {
	TenantID        id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	WorkItemID      id.UUID
	ProposalID      id.UUID
	TurnRef         id.UUID
	SchemaVersion   string
	Intent          string
	CandidateAnswer string
	MissingFacts    []MissingFact
	ExtractedFacts  []ExtractedFact
	Proposer        string
	OwnerEpoch      int64
	ModeVersion     int64
	ActorID         id.UUID
	CreatedAt       time.Time
}

// ---------------------------------------------------------------------------
// The outcome material
// ---------------------------------------------------------------------------

// ReusedFact is the provenance of one deduplicated question: the
// confirmed exact version of the fact whose value answers the question
// (the fact id, the version number, the value digest) and the source of
// the removed question (the rule table or the proposal claim).
type ReusedFact struct {
	MaterialKey string  `json:"material_key"`
	FactID      id.UUID `json:"fact_id"`
	VersionNo   int64   `json:"version_no"`
	ValueDigest string  `json:"value_digest"`
	Source      string  `json:"source"`
}

// SuppressedQuestion is the audit row of one removed repeat: the material
// key and the source of the question that the dedup removed.
type SuppressedQuestion struct {
	MaterialKey string `json:"material_key"`
	Source      string `json:"source"`
}

// MandatoryFactStatus is one mandatory fact of the outcome with its
// state: reused (the value answers it — the provenance travels in
// reused_facts) or ask (the question remains in the plan).
type MandatoryFactStatus struct {
	MaterialKey string `json:"material_key"`
	Source      string `json:"source"`
	State       string `json:"state"`
}

// The mandatory-fact state words.
const (
	MandatoryStateReused = "reused"
	MandatoryStateAsk    = "ask"
)

// NextStep is one planned step of the outcome: the finite dictionary word
// with the material key of an ask_fact step.
type NextStep struct {
	Step        string `json:"step"`
	MaterialKey string `json:"material_key,omitempty"`
}

// FactsDigestMap is the canonical confirmed fact material of the input
// snapshot: the material key → the exact confirmed version.
type FactsDigestMap map[string]struct {
	VersionNo   int64  `json:"version_no"`
	ValueDigest string `json:"value_digest"`
}

// PolicyInputSnapshot is the full anchored input material of one outcome:
// the proposal identity with its schema word, the mode version, the owner
// epoch, the confirmed fact map with its digest, the Work Item version
// and the rule version. The v2 form of Story 2.11 adds the preferences
// map (the advisory fourth input: key -> {revision, cleared} — never a
// load-bearing material; a change between the evaluations is the honest
// supersede while the fact, mode, ownership and Work Item anchors still
// guard the load-bearing path). The same anchors produce the same
// decision digest (the reproducibility, AC-3).
type PolicyInputSnapshot struct {
	ProposalID      id.UUID                  `json:"proposal_id"`
	SchemaVersion   string                   `json:"schema_version"`
	ModeVersion     int64                    `json:"mode_version"`
	OwnerEpoch      int64                    `json:"owner_epoch"`
	FactsDigest     FactsDigestMap           `json:"facts_digest"`
	FactsDigestHash string                   `json:"facts_digest_hash"`
	WorkItemVersion int64                    `json:"work_item_version"`
	RuleVersion     string                   `json:"rule_version"`
	Preferences     AccommodationSnapshotMap `json:"preferences"`
}

// PolicyOutcome is one append-only decision: the class, the reason, the
// rule version, the intent snapshot, the mandatory facts with their
// states, the next steps, the reused facts with the provenance, the
// suppressed questions, the active alternatives (the 2.11 extension),
// the input snapshot, the decision digest, the
// monotonic number and the actor. The CURRENT outcome of a Work Item is
// the maximal outcome_no — a re-evaluation supersedes by a NEW row.
type PolicyOutcome struct {
	TenantID            id.UUID
	CaseID              id.UUID
	ConversationID      id.UUID
	WorkItemID          id.UUID
	ProposalID          id.UUID
	OutcomeID           id.UUID
	OutcomeNo           int64
	OutcomeClass        string
	Reason              string
	RuleVersion         string
	Intent              string
	MandatoryFacts      []MandatoryFactStatus
	NextSteps           []NextStep
	ReusedFacts         []ReusedFact
	SuppressedQuestions []SuppressedQuestion
	ActiveAlternatives  []string
	InputSnapshot       PolicyInputSnapshot
	DecisionDigest      string
	ActorID             id.UUID
	CreatedAt           time.Time
}

// CurrentPolicyOutcome resolves the CURRENT outcome of the Work Item: the
// maximal outcome number (nil when no outcome exists — the pre-evaluation
// honesty).
func CurrentPolicyOutcome(outcomes []PolicyOutcome) *PolicyOutcome {
	var current *PolicyOutcome
	for i := range outcomes {
		if current == nil || outcomes[i].OutcomeNo > current.OutcomeNo {
			current = &outcomes[i]
		}
	}
	return current
}

// OutcomePermittedPath resolves the fixed permitted-path word of one
// outcome class for the panel and the gate words: the blocked commercial
// path names the mode exit (the 2-5 ladder word); the rest name the
// evaluation path.
func OutcomePermittedPath(class string) string {
	if class == OutcomeModeBlocked {
		return PermittedPathExit
	}
	return PermittedPathEvaluatePolicy
}

// ---------------------------------------------------------------------------
// The confirmed fact material (the 2-2 derivation inputs)
// ---------------------------------------------------------------------------

// ConfirmedFact is the exact confirmed version of one fact of the case:
// the anchor identity, the normalized material key, the confirmed version
// number and its value digest. The reuse and the digest map are built
// from these exact versions only.
type ConfirmedFact struct {
	FactID      id.UUID
	MaterialKey string
	VersionNo   int64
	ValueDigest string
}

// ConfirmedFactView derives the honest fact state for the policy: the
// status word (the 2-2 DeriveFactStatus), and — for the confirmed state —
// the exact confirmed version number and its digest (the latest
// confirmation points at the version whose digest equals it).
func ConfirmedFactView(versions []FactVersion, confirmations []FactConfirmation) (status string, versionNo int64, valueDigest string) {
	status, digest := DeriveFactStatus(versions, confirmations)
	if status != FactStatusConfirmed {
		return status, 0, ""
	}
	// The exact confirmed version: the version of the LATEST
	// confirmation (DeriveFactStatus confirms the digest of that exact
	// version).
	confirmedNo := int64(-1)
	for _, confirmation := range confirmations {
		if confirmation.VersionNo > confirmedNo {
			confirmedNo = confirmation.VersionNo
		}
	}
	if confirmedNo < 0 {
		return status, 0, ""
	}
	for _, version := range versions {
		if version.VersionNo == confirmedNo {
			return status, confirmedNo, version.ValueDigest
		}
	}
	return status, 0, digest
}

// ComputeFactsDigest renders the canonical SHA-256 of the confirmed fact
// set: the sorted map material_key → {version_no, value_digest} (the
// encoding/json map order), compacted and hashed. The same confirmed set
// always yields the same digest — the anchor of the evaluation.
func ComputeFactsDigest(confirmed []ConfirmedFact) (string, FactsDigestMap, error) {
	digestMap := FactsDigestMap{}
	for _, fact := range confirmed {
		digestMap[fact.MaterialKey] = struct {
			VersionNo   int64  `json:"version_no"`
			ValueDigest string `json:"value_digest"`
		}{VersionNo: fact.VersionNo, ValueDigest: fact.ValueDigest}
	}
	canonical, err := json.Marshal(digestMap)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), digestMap, nil
}

// ---------------------------------------------------------------------------
// The deterministic evaluation
// ---------------------------------------------------------------------------

// FactStatusWord is the policy view of one fact of the case: the 2-2
// status word with the confirmed exact version when the status is
// confirmed.
type FactStatusWord struct {
	Status    string
	Confirmed *ConfirmedFact
}

// FactWorld is the policy view of the whole fact set of the case, keyed
// by the normalized material key.
type FactWorld map[string]FactStatusWord

// PolicyDecision is the pure result of the evaluation: the class, the
// reason, the mandatory fact plan with the states, the next steps, the
// reused facts with the provenance, the suppressed questions and the
// active alternative words (the 2.11 extension: empty when the trigger
// did not fire — the control branch decision stays byte-identical).
type PolicyDecision struct {
	OutcomeClass        string
	Reason              string
	MandatoryFacts      []MandatoryFactStatus
	NextSteps           []NextStep
	ReusedFacts         []ReusedFact
	SuppressedQuestions []SuppressedQuestion
	ActiveAlternatives  []string `json:"active_alternatives,omitempty"`
}

// QuestionSource is the source word of one question of the plan: the rule
// table or the proposal claim.
const (
	QuestionSourceRule     = "rule"
	QuestionSourceProposal = "proposal"
)

// EvaluatePolicy is the deterministic v2 rule: the unknown intent fails
// closed with the human handoff; the commercial intent under a non-sales
// effective mode is blocked with the mode words; the commercial intent
// with an empty candidate is the clarify outcome (questions only); the
// commercial intent with a candidate under the sales mode is the
// progressive commercial answer (the open questions stay in the plan);
// the non-commercial intent is the informational answer. The dedup
// removes the questions of the confirmed facts (unless the proposal
// claims an explicit readback) — the reuse carries the provenance and the
// suppressed repeats the audit. The 2.11 fourth input is the preferences
// state of the case: a mandatory media-usual key without a confirmed
// fact AND the recorded active no_photo attach the active alternatives
// of the pack dictionary to the commercial decision (the question NEVER
// leaves the plan — an alternative is a permitted path of the service,
// never a replacement of the question); an empty pack dictionary of the
// key's modality fails closed with the no_alternative_path reason and
// the human handoff. The control branch stays honest: no preferences, a
// confirmed media fact or a text-usual key produce the byte-identical
// former decision.
func EvaluatePolicy(proposal AIProposal, facts FactWorld, effectiveMode string, accommodations CurrentAccommodations) PolicyDecision {
	// The unknown intent fails closed: the proposal is saved (the claim
	// audit), the outcome is the safe handoff — never a submission
	// refusal (the model result is a proposal, not a Policy Outcome).
	if !IntentInDictionary(proposal.Intent) {
		return PolicyDecision{
			OutcomeClass: OutcomeFailClosed, Reason: ReasonUnknownIntent,
			NextSteps: []NextStep{{Step: StepHumanHandoff}},
		}
	}
	// The commercial intents under a non-sales effective mode: the mode
	// words decide (the fail-closed pre-migration absence is the
	// non-sales word too).
	if IntentIsCommercial(proposal.Intent) && effectiveMode != ModeSales {
		return PolicyDecision{
			OutcomeClass: OutcomeModeBlocked, Reason: ReasonModeBlocksCommercial,
			NextSteps: []NextStep{{Step: StepHumanHandoff}},
		}
	}
	// The question plan: the rule-table mandatory keys ∪ the proposal
	// mandatory claims; the readback claims stay questions regardless of
	// the confirmation. The dedup removes the confirmed keys (the exact
	// version is reused with the provenance); a conflicting or an
	// unverified fact keeps the question; the explicit readback keeps it
	// even when confirmed.
	plan := map[string]string{}
	for _, key := range ruleTableV1[proposal.Intent] {
		plan[key] = QuestionSourceRule
	}
	readback := map[string]bool{}
	for _, missing := range proposal.MissingFacts {
		if missing.Reason == MissingReasonReadback {
			readback[missing.MaterialKey] = true
		}
		if _, ok := plan[missing.MaterialKey]; !ok {
			plan[missing.MaterialKey] = QuestionSourceProposal
		}
	}
	keys := make([]string, 0, len(plan))
	for key := range plan {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	decision := PolicyDecision{}
	questions := []string{}
	for _, key := range keys {
		source := plan[key]
		fact, known := facts[key]
		if readback[key] {
			// The explicit readback: the question stays even when the
			// value is confirmed (the only v1 re-ask word).
			decision.MandatoryFacts = append(decision.MandatoryFacts, MandatoryFactStatus{MaterialKey: key, Source: source, State: MandatoryStateAsk})
			questions = append(questions, key)
			continue
		}
		if known && fact.Status == FactStatusConfirmed && fact.Confirmed != nil {
			// The confirmed exact version answers the question: the
			// repeat is suppressed, the value is reused with the
			// provenance.
			decision.MandatoryFacts = append(decision.MandatoryFacts, MandatoryFactStatus{MaterialKey: key, Source: source, State: MandatoryStateReused})
			decision.ReusedFacts = append(decision.ReusedFacts, ReusedFact{
				MaterialKey: key, FactID: fact.Confirmed.FactID,
				VersionNo: fact.Confirmed.VersionNo, ValueDigest: fact.Confirmed.ValueDigest, Source: source,
			})
			decision.SuppressedQuestions = append(decision.SuppressedQuestions, SuppressedQuestion{MaterialKey: key, Source: source})
			continue
		}
		// A conflicting, an unverified or an absent fact: the question
		// stays (the conflict and the readback are the only v1 re-ask
		// reasons; an unconfirmed value is never a basis).
		decision.MandatoryFacts = append(decision.MandatoryFacts, MandatoryFactStatus{MaterialKey: key, Source: source, State: MandatoryStateAsk})
		questions = append(questions, key)
	}
	// The honest alternatives trigger (Story 2.11): computed before the
	// class branches — the clarify and the commercial answer both carry
	// the active alternatives when the trigger fires; the informational
	// and the blocked paths never reach the attachment (no commercial
	// plan, no offer).
	alternativesTriggered, alternativesModality := mediaAlternativeTrigger(proposal.Intent, facts, accommodations)
	if IntentIsCommercial(proposal.Intent) {
		if strings.TrimSpace(proposal.CandidateAnswer) == "" {
			// No answer basis: the clarify outcome — the questions only.
			decision.OutcomeClass = OutcomeClarify
			decision.Reason = ReasonNoAnswerBasis
			for _, key := range questions {
				decision.NextSteps = append(decision.NextSteps, NextStep{Step: StepAskFact, MaterialKey: key})
			}
			return applyMediaAlternatives(decision, alternativesTriggered, alternativesModality)
		}
		// The progressive commercial answer: the short answer now, the
		// minimal questions alongside, the confirmation at the end.
		decision.OutcomeClass = OutcomeCommercialAnswer
		decision.Reason = ReasonOkCommercial
		for _, key := range questions {
			decision.NextSteps = append(decision.NextSteps, NextStep{Step: StepAskFact, MaterialKey: key})
		}
		decision.NextSteps = append(decision.NextSteps, NextStep{Step: StepAwaitConfirmation})
		return applyMediaAlternatives(decision, alternativesTriggered, alternativesModality)
	}
	// The non-commercial intent: the informational answer. The human
	// request carries the handoff step (the client asked for a human —
	// the honest plan).
	decision.OutcomeClass = OutcomeInformationalAnswer
	decision.Reason = ReasonOkInformational
	if proposal.Intent == IntentHumanRequest {
		decision.NextSteps = append(decision.NextSteps, NextStep{Step: StepHumanHandoff})
	}
	return decision
}

// mediaAlternativeTrigger resolves the v1 trigger pair of the honest
// alternatives: a rule-table MANDATORY key of the media-usual modality
// whose fact is NOT confirmed (the 2-6 reuse basis did not happen — the
// question stays in the plan) AND the recorded active no_photo. The
// modality word of the first missing media key travels for the pack
// alternatives resolution; the empty modality word means no trigger.
func mediaAlternativeTrigger(intent string, facts FactWorld, accommodations CurrentAccommodations) (bool, string) {
	if !accommodations.HasActive(AccommodationNoPhoto) {
		return false, ""
	}
	for _, key := range ruleTableV1[intent] {
		if !factConfirmed(facts, key) && UsualModalityOfMaterialKey(key) == UsualModalityMediaPhoto {
			return true, UsualModalityMediaPhoto
		}
	}
	return false, ""
}

// factConfirmed reports whether the fact of the key carries the
// confirmed exact version (the reuse basis of the 2-6 dedup).
func factConfirmed(facts FactWorld, key string) bool {
	fact, known := facts[key]
	return known && fact.Status == FactStatusConfirmed && fact.Confirmed != nil
}

// applyMediaAlternatives attaches the honest alternatives to the decision
// of the fired trigger: the pack dictionary of the key's modality fills
// the active words; an EMPTY pack dictionary is the honest fail-closed
// stop (the no_alternative_path reason and the human handoff — the check
// never weakens); the question plan stays untouched (an alternative is a
// permitted path, never a replacement of the question).
func applyMediaAlternatives(decision PolicyDecision, triggered bool, modality string) PolicyDecision {
	if !triggered {
		return decision
	}
	alternatives := AlternativesOfUsualModality(modality)
	if len(alternatives) == 0 {
		return PolicyDecision{
			OutcomeClass: OutcomeFailClosed,
			Reason:       ReasonNoAlternativePath,
			NextSteps:    []NextStep{{Step: StepHumanHandoff}},
		}
	}
	decision.ActiveAlternatives = alternatives
	return decision
}

// DecisionDigest renders the canonical SHA-256 of the decision over the
// anchored inputs: the compact canonical JSON of the input snapshot and
// the decision fields (the class, the reason, the plan, the steps, the
// reuse, the suppression). The same inputs produce the same digest — the
// determinism proof of the slice.
func DecisionDigest(snapshot PolicyInputSnapshot, decision PolicyDecision, intent string) (string, error) {
	canonical, err := json.Marshal(struct {
		Input    PolicyInputSnapshot `json:"input"`
		Intent   string              `json:"intent"`
		Decision PolicyDecision      `json:"decision"`
	}{snapshot, intent, decision})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// ---------------------------------------------------------------------------
// Command guards
// ---------------------------------------------------------------------------

// ProposalFieldProblem is one machine-readable field problem of the
// proposal submission: the field path and the honest text.
type ProposalFieldProblem struct {
	Field   string `json:"field"`
	Problem string `json:"problem"`
}

// ProposalSchemaError is the machine-readable submission refusal: the
// structure and the bounds problems of the typed schema (the intent
// dictionary is NEVER a submission matter — the policy decides it; the
// repair loop is the proposer's duty, the server answers with the field
// problems only).
type ProposalSchemaError struct {
	Problems []ProposalFieldProblem
}

func (e ProposalSchemaError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, problem := range e.Problems {
		parts = append(parts, problem.Field+": "+problem.Problem)
	}
	return "схема предложения не принята: " + strings.Join(parts, "; ")
}

// GuardSubmitProposal validates the structure, the bounds and the field
// dictionary words of the submission: the intent is a bounded non-empty
// string (the dictionary is a policy matter), the candidate answer is
// bounded (possibly empty), the missing facts carry the normalized keys
// and the reason words without duplicates, the extracted facts carry the
// normalized keys, the bounded digests and the event references, the
// proposer word is the dictionary. The problems accumulate — one honest
// refusal lists every field problem.
func GuardSubmitProposal(proposal AIProposal) error {
	problems := []ProposalFieldProblem{}
	if !proposal.ConversationID.Valid() {
		problems = append(problems, ProposalFieldProblem{Field: "conversation_id", Problem: "поле обязательно (UUID): предложение живёт на диалоге обращения"})
	}
	if !proposal.WorkItemID.Valid() {
		problems = append(problems, ProposalFieldProblem{Field: "work_item_id", Problem: "поле обязательно (UUID): предложение принадлежит элементу работы"})
	}
	if !proposal.TurnRef.Valid() {
		problems = append(problems, ProposalFieldProblem{Field: "turn_ref", Problem: "поле обязательно (UUID): предложение отвечает принятому событию кейса"})
	}
	if strings.TrimSpace(proposal.Intent) == "" {
		problems = append(problems, ProposalFieldProblem{Field: "intent", Problem: "поле обязательно: намерение предложения не может быть пустым"})
	} else if utf8.RuneCountInString(proposal.Intent) > MaxIntentRunes {
		problems = append(problems, ProposalFieldProblem{Field: "intent", Problem: fmt.Sprintf("длиннее %d символов", MaxIntentRunes)})
	}
	if utf8.RuneCountInString(proposal.CandidateAnswer) > MaxCandidateAnswerRunes {
		problems = append(problems, ProposalFieldProblem{Field: "candidate_answer", Problem: fmt.Sprintf("длиннее %d символов", MaxCandidateAnswerRunes)})
	}
	if !inDictionary(ProposalProposers, proposal.Proposer) {
		problems = append(problems, ProposalFieldProblem{Field: "proposer", Problem: "не входит в словарь (" + joinQuoted(ProposalProposers) + ")"})
	}
	seenMissing := map[string]bool{}
	for i, missing := range proposal.MissingFacts {
		field := fmt.Sprintf("missing_facts[%d]", i)
		key, err := NormalizeMaterialKey(missing.MaterialKey)
		if err != nil {
			problems = append(problems, ProposalFieldProblem{Field: field + ".material_key", Problem: err.Error()})
		} else if key != missing.MaterialKey {
			problems = append(problems, ProposalFieldProblem{Field: field + ".material_key", Problem: "ключ материала обязан быть нормализован (обрезан и в нижнем регистре, как факты 2-2)"})
		}
		if !inDictionary(MissingReasons, missing.Reason) {
			problems = append(problems, ProposalFieldProblem{Field: field + ".reason", Problem: "не входит в словарь (" + joinQuoted(MissingReasons) + ")"})
		}
		if seenMissing[missing.MaterialKey] {
			problems = append(problems, ProposalFieldProblem{Field: field + ".material_key", Problem: "повтор ключа: один ключ — одна причина"})
		}
		seenMissing[missing.MaterialKey] = true
	}
	for i, extracted := range proposal.ExtractedFacts {
		field := fmt.Sprintf("extracted_facts[%d]", i)
		key, err := NormalizeMaterialKey(extracted.MaterialKey)
		if err != nil {
			problems = append(problems, ProposalFieldProblem{Field: field + ".material_key", Problem: err.Error()})
		} else if key != extracted.MaterialKey {
			problems = append(problems, ProposalFieldProblem{Field: field + ".material_key", Problem: "ключ материала обязан быть нормализован (обрезан и в нижнем регистре, как факты 2-2)"})
		}
		if utf8.RuneCountInString(extracted.ValueDigest) > MaxValueDigestRunes {
			problems = append(problems, ProposalFieldProblem{Field: field + ".value_digest", Problem: fmt.Sprintf("длиннее %d символов", MaxValueDigestRunes)})
		}
		if !extracted.SourceEventID.Valid() {
			problems = append(problems, ProposalFieldProblem{Field: field + ".source_event_id", Problem: "поле обязательно (UUID): притязание факта обязано нести источник-событие"})
		}
	}
	if len(problems) > 0 {
		return ProposalSchemaError{Problems: problems}
	}
	return nil
}

// GuardEvaluate validates the evaluation command material: the proposal
// reference and the four anchors of the input versions (the mode
// version, the owner epoch, the confirmed facts digest and the Work Item
// version — the TOCTOU guard of the fact reads).
func GuardEvaluate(proposalID id.UUID, expectedModeVersion, expectedOwnerEpoch, expectedWorkItemVersion int64, expectedFactsDigest string) error {
	if !proposalID.Valid() {
		return fmt.Errorf("поле proposal_ref обязательно (UUID): оценка живёт на предложении")
	}
	if expectedModeVersion < 1 {
		return fmt.Errorf("якорь expected_mode_version обязателен (>= 1): сверьте текущую версию лестницы режимов")
	}
	if expectedOwnerEpoch < 1 {
		return fmt.Errorf("якорь expected_owner_epoch обязателен (>= 1): сверьте текущую эпоху владения")
	}
	if expectedWorkItemVersion < 1 {
		return fmt.Errorf("якорь expected_work_item_version обязателен (>= 1): сверьте текущую версию элемента работы")
	}
	if strings.TrimSpace(expectedFactsDigest) == "" {
		return fmt.Errorf("якорь expected_facts_digest обязателен: дайджест подтверждённого факт-набора кейса")
	}
	return nil
}

// TerminalWorkItemText is the honest refusal of the evaluation of a
// terminal Work Item (the resolved and the merged states are final; the
// merged item lives on in its target).
const TerminalWorkItemText = "Терминальный элемент работы не оценивается: resolved и merged — финальные состояния; для merged оценивайте целевой элемент работы."

// PolicyConflictText is the fixed refusal text of the raced evaluation:
// one of the anchored input versions moved under the command.
const PolicyConflictText = "Якорь входных версий изменился: перечитайте текущие версии (режим, владение, факты, элемент работы) и повторите оценку."

// PolicyConflictError is the machine-readable raced-anchor refusal: the
// CURRENT versions travel with the conflict (the honest 409 — the
// operator re-reads the state and re-anchors).
type PolicyConflictError struct {
	CurrentModeVersion     int64
	CurrentOwnerEpoch      int64
	CurrentFactsDigest     string
	CurrentWorkItemVersion int64
}

func (e PolicyConflictError) Error() string { return PolicyConflictText }

// ---------------------------------------------------------------------------
// The sales reply gate (the load-bearing policy gate)
// ---------------------------------------------------------------------------

// ReplyPolicyError is the machine-readable refusal of the policy gate of
// the sales reply: the Work Item anchor word, the CURRENT outcome words
// (the class, the reason, the rule version, the number — empty when no
// outcome exists) and the permitted path. The gate order stays frozen:
// the ownership, the mode and the disclosure refusals fire EARLIER.
type ReplyPolicyError struct {
	WorkItemID    id.UUID
	CurrentClass  string
	CurrentReason string
	RuleVersion   string
	OutcomeNo     int64
	PermittedPath string
	Reason        string
}

func (e ReplyPolicyError) Error() string { return e.Reason }

// ReplyPolicyNoAnchorText is the fixed refusal text of the sales reply
// without the Work Item anchor: the sales path belongs to a Work Item.
const ReplyPolicyNoAnchorText = "Продажный путь принадлежит элементу работы: якорь work_item_id обязателен для ответа класса sales. Разрешённый путь — привязать элемент работы."

// ReplyPolicyOutcomeRequiredText is the fixed refusal text of the sales
// reply without a Policy Outcome: the server-side truth of the permitted
// answer is the evaluation, never the mode or the disclosure alone.
const ReplyPolicyOutcomeRequiredText = "Действующий Policy Outcome отсутствует: продажный ответ требует оценки предложения (submit_ai_proposal + evaluate_policy). Разрешённый путь — подать и оценить предложение."

// ReplyPolicyNotCommercialText is the fixed refusal text of the sales
// reply against a non-commercial current outcome: only the
// commercial_answer class permits the sales reply.
const ReplyPolicyNotCommercialText = "Действующий исход не разрешает продажный ответ: только класс commercial_answer открывает продажный путь. Разрешённый путь — переоценить предложение с новыми якорями."

// ---------------------------------------------------------------------------
// The outbox payloads (the safe canonical JSON)
// ---------------------------------------------------------------------------

// Outbox event types of the policy surface (the 2-1 bounded vocabulary).
const (
	OutboxEventAIProposalSubmitted = "case.ai_proposal_submitted"
	OutboxEventAIProposalSubmitV1  = 1
	OutboxEventPolicyEvaluated     = "case.policy_evaluated"
	OutboxEventPolicyEvaluatedV1   = 1
)

// AIProposalSubmittedPayload is the safe canonical JSON of the proposal
// submission: the identifiers, the dictionary state words and the version
// numbers only — the candidate answer text NEVER enters the envelope.
type AIProposalSubmittedPayload struct {
	TenantID       id.UUID   `json:"tenant_id"`
	CaseID         id.UUID   `json:"case_id"`
	ConversationID id.UUID   `json:"conversation_id"`
	WorkItemID     id.UUID   `json:"work_item_id"`
	ProposalID     id.UUID   `json:"proposal_id"`
	TurnRef        id.UUID   `json:"turn_ref"`
	SchemaVersion  string    `json:"schema_version"`
	Intent         string    `json:"intent"`
	Proposer       string    `json:"proposer"`
	OwnerEpoch     int64     `json:"owner_epoch"`
	ModeVersion    int64     `json:"mode_version"`
	ActorID        id.UUID   `json:"actor_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// PolicyEvaluatedPayload is the safe canonical JSON of the evaluation:
// the identifiers, the outcome state words and the monotonic number —
// no candidate text, no fact values.
type PolicyEvaluatedPayload struct {
	TenantID       id.UUID   `json:"tenant_id"`
	CaseID         id.UUID   `json:"case_id"`
	ConversationID id.UUID   `json:"conversation_id"`
	WorkItemID     id.UUID   `json:"work_item_id"`
	ProposalID     id.UUID   `json:"proposal_id"`
	OutcomeID      id.UUID   `json:"outcome_id"`
	OutcomeNo      int64     `json:"outcome_no"`
	OutcomeClass   string    `json:"outcome_class"`
	Reason         string    `json:"reason"`
	RuleVersion    string    `json:"rule_version"`
	Intent         string    `json:"intent"`
	ActorID        id.UUID   `json:"actor_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// DedupKeys of the two outbox records.
func DedupKeyAIProposalSubmitted(proposalID id.UUID) string {
	return "case.ai_proposal_submitted:" + proposalID.String()
}

func DedupKeyPolicyEvaluated(outcomeID id.UUID) string {
	return "case.policy_evaluated:" + outcomeID.String()
}

// AIProposalSubmittedPayloadJSON renders the safe canonical JSON.
func AIProposalSubmittedPayloadJSON(payload AIProposalSubmittedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// PolicyEvaluatedPayloadJSON renders the safe canonical JSON.
func PolicyEvaluatedPayloadJSON(payload PolicyEvaluatedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
