package planner

// The platform dictionaries of the turn-contract slice (Story 8-1): the
// closed words the contract validates against. A dictionary is data with
// a mirror: the mode ladder mirrors cases/domain.ModeOrder and the
// read-argument typing mirrors tools.ToolFieldTypes — both pinned by
// contract tests (a dictionary change on either side turns the mirror
// red before any code that trusts the words ships). Extending a
// dictionary is a core change with an explicit spec revision, never a
// quiet edit. All words are platform-clean: no vertical vocabulary ever
// enters this package.

import "strconv"

// Build identifies the planner machine binary that produced turn
// records. A build change means the records of the new build carry the
// new word — the prior records stay as they are (append-only, AD-10);
// the honest staleness of the old-build records is the eval work of
// Story 8-5 (the connectorprobe AdapterBuild semantics applied to the
// planner journal).
const Build = "planner/1.0.0"

// ProposalSchemaVersion is the version of the Turn Proposal form of
// this slice. The form is versioned from the first day: a proposal
// carrying a foreign version is refused by name (the wire-coherence
// discipline of 7-3/7-4 — an old form is never silently upgraded); a
// form evolution is a new version plus a conscious migration of the
// machine, never an in-place reinterpretation.
const ProposalSchemaVersion = 1

// ReplyBudgetRunes is the platform budget of the proposal reply text
// (and of every question text): a reply past the budget is a VALIDATION
// refusal, never a silent truncation — the size discipline of the
// 7-3/7-4 wire surfaces applied to the model output. The value is the
// platform text-material budget (the largest honest text constant of
// the cases domain, MaxExternalCommitmentContentRunes).
const ReplyBudgetRunes = 2000

// AssemblyVersion is the version word of the round-material assembly
// form (Story 8-2): the prompt assembly is versioned platform code,
// and every turn record carries the word that produced its rounds —
// the same discipline as the machine Build (a word change means the
// records of the new word carry the new word; the prior records stay
// as they are, append-only, AD-10).
const AssemblyVersion = "assembly/1.0.0"

// RuntimeBuildRef is the composite runtime identity word the Release
// Manifest pins as runtime_build (AR-17): the machine Build and the
// assembly version together — coherently with what a turn record
// carries (Build + AssemblyVersion). The word is platform-clean: two
// version identities joined by '+'.
const RuntimeBuildRef = Build + "+" + AssemblyVersion

// The intent dictionary of the slice: the communicative move the
// proposal declares as its dominant intent. The union kind of the
// proposed action carries the commitment/action/handoff axis; the
// intent word summarizes what the turn is for.
const (
	IntentRespond = "respond"
	IntentClarify = "clarify"
	IntentRead    = "read"
	IntentPropose = "propose"
)

// IntentDictionary is the full closed set.
var IntentDictionary = []string{IntentRespond, IntentClarify, IntentRead, IntentPropose}

// The mode dictionary: the exact mirror of the conversation-mode ladder
// of cases/domain (the FR-14 priority order — the earlier word is the
// stricter one). The proposal mode may not be LOWER on this ladder than
// the context mode: a priority conversation never loosens itself by a
// proposal (the raise-only discipline of Story 2.5 — the only way down
// is the supervised exit, which is not a planner move).
const (
	ModeSafety           = "safety"
	ModeClaim            = "claim"
	ModeCancellation     = "cancellation"
	ModePrivacyNoContact = "privacy_no_contact"
	ModeHumanRequested   = "human_requested"
	ModeUnclassified     = "unclassified"
	ModeSales            = "sales"
)

// ModeLadder is the mirror of cases/domain.ModeOrder, strictness order:
// the SMALLER rank is the STRICTER word. The contract test pins the
// mirror element by element — a change of either ladder is a conscious
// core change, never drift.
var ModeLadder = []string{ModeSafety, ModeClaim, ModeCancellation, ModePrivacyNoContact, ModeHumanRequested, ModeUnclassified, ModeSales}

// ModeRank resolves the strictness position of a ladder word: the
// smaller number is the stricter word. An unknown word is -1 — the
// dictionary check of the decode/validate phases runs BEFORE any rank
// comparison, so the honest -1 never participates in a ladder decision.
func ModeRank(mode string) int {
	for rank, word := range ModeLadder {
		if word == mode {
			return rank
		}
	}
	return -1
}

// The safety dictionary of the proposal: the planner's own safety
// assessment of the material it proposes. The word never removes a
// safety MODE of the context — the ladder rule carries that guarantee
// (a safety-mode conversation stays safety-mode in the proposal); the
// assessment word says how safe the PROPOSED material itself is.
const (
	SafetyClear   = "clear"
	SafetyCaution = "caution"
	SafetyBlocked = "blocked"
)

// SafetyDictionary is the full closed set.
var SafetyDictionary = []string{SafetyClear, SafetyCaution, SafetyBlocked}

// The fallback dictionary: the approved safe fallback forms that may
// end a failed turn (AD-7 — never the model output). The words are
// platform-honest forms; the context declares which one applies (the
// fallback policy of the turn, chosen by the future context builder —
// Story 8.4).
const (
	FallbackHonestStop = "honest_stop"
	FallbackHandoff    = "handoff"
	FallbackCallback   = "callback"
)

// FallbackDictionary is the full closed set.
var FallbackDictionary = []string{FallbackHonestStop, FallbackHandoff, FallbackCallback}

// The read-argument typing dictionary: the exact mirror of
// tools.ToolFieldTypes (the pack typing language string | integer |
// boolean — Story 7-2). The planner reuses the dictionary AS IS: the
// contract test pins the mirror, and an extension on either side turns
// the mirror red before it ships. A read argument of any other JSON
// kind is a named refusal.
var ReadArgTypes = []string{"string", "integer", "boolean"}

// The proposed-action kind dictionary: the closed union of what a
// proposal may propose. A kind other than none REQUIRES a contract
// reference (id@version) — the proposal only NAMES the thing, never
// carries its execution material (AD-6: the material snapshots of the
// Confirmation machine — Story 8.4).
const (
	ActionKindNone       = "none"
	ActionKindCommitment = "commitment"
	ActionKindAction     = "action"
	ActionKindHandoff    = "handoff"
)

// ActionKindDictionary is the full closed set.
var ActionKindDictionary = []string{ActionKindNone, ActionKindCommitment, ActionKindAction, ActionKindHandoff}

// The turn-outcome dictionary of the journal: every started turn ends
// exactly one of these three ways. A turn whose context never decoded
// is NOT a turn of this dictionary — the refusal of the input happens
// before any model round and before any record.
const (
	OutcomeValidated = "validated"
	OutcomeRepaired  = "repaired"
	OutcomeFailed    = "failed"
)

// isDictionaryWord reports membership of a closed dictionary.
func isDictionaryWord(word string, dictionary []string) bool {
	for _, candidate := range dictionary {
		if candidate == word {
			return true
		}
	}
	return false
}

// isRef reports whether value carries the platform reference form
// id@version: non-empty on both sides of exactly one '@'.
func isRef(value string) bool {
	at := -1
	for index, symbol := range value {
		if symbol == '@' {
			if at != -1 {
				return false
			}
			at = index
		}
	}
	if at <= 0 || at == len(value)-1 {
		return false
	}
	return true
}

// isIntegerText reports whether the raw JSON number text is a strict
// integer literal (the platform integer typing: no fraction, no
// exponent — "1" and "42" are integers, "1.5" and "1e3" are not).
func isIntegerText(text string) bool {
	if text == "" {
		return false
	}
	_, err := strconv.ParseInt(text, 10, 64)
	return err == nil
}
