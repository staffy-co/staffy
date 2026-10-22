package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// Module action — story 3.8 slice 1 (step-01): the recovery-case domain
// core (FR-38, the AC-1..AC-12 blocks of epic 3.8, first slice: the
// creation contour of a linked Recovery Case from eligible failed
// operations). This file owns the recovery-contour dictionaries (the THREE
// trigger words of the read-time eligibility ladder, the TWO event kind
// words of the append-only recovery history, the ONE case state word —
// read-time «open», the closing words do not exist in this slice), the
// bounded constants (the safe-details rune bound of the store CHECK, the
// observations pin bound of the SAFE bundle), the read-time eligibility
// ladder over the RECORDED facts (DeriveRecoveryEligibility — every gas is
// an honest refusal word, NEVER a write), the tenant-scoped recovery key
// (the keyed HMAC space recovery_key:v1: — the fourth prefix family after
// commitment:v1:/external_commitment:v1:/action_preview:v1:/
// business_intent:v1:/operation_material:v1:), the SAFE link-bundle of the
// exact-version pins with its canonical digest (DeriveBundleDigest — the
// repeat comparator: equal = the idempotent return, a delta = ONE
// append-only evidence_appended row), the assignment derivation from the
// LIVE readings ONLY (DeriveRecoveryAssignment — the owner is the current
// ownership term of the original Case, the backup and the deadline are the
// human coverage of the original conversation route; an unreadable
// coverage or an owner-less case is the honest fail-closed refusal — the
// assignment NEVER comes from a command parameter), and the fixed honest
// texts of the recovery surfaces. The words are frozen in code; runtime
// or tenant-config extension is NEVER allowed.
//
// The creation contour is LOCAL (the slice honesty core): not ONE network
// call rides the command path — the adapters are structurally absent from
// the eligibility, the key, the bundle, the assignment and the digest
// derivations of this file (the network stays in the dispatch contour of
// 3-6 and the reconciliation contour of 3-7; the reflect probe of the app
// layer pins it in step-02). The material NEVER enters this file: the
// bundle carries pins, versions, counters, words and 64-hex digests only
// (AD-10/AC-9 — the sensitive payload stays in its own immutable preview
// rows; the recovery speaks minimal references instead of copies). The
// original Case, the operation and the receipts are NEVER rewritten
// (append-only everywhere — «the original Case is not reopened for a
// rewrite» is structural).

// ---------------------------------------------------------------------------
// The dictionaries (compile-time, closed)
// ---------------------------------------------------------------------------

// Recovery trigger words — the answer vocabulary of the read-time
// eligibility ladder (three). reconciliation_exhausted — the operation
// stays unknown AND the bounded automatic reconciliation took its three
// lease attempts (the honest human word of the 3-7 exhaustion, now with a
// material work object); reconciliation_unavailable — the operation stays
// unknown AND the automatic contour does not exist for it (the definition
// declares no ReconciliationQuery, the connector declaration carries no
// LIVE word, or the reconciliation adapter is not wired — the AG-4
// production composition fact: the human is mandatory); operation_failed
// — the terminal refusal AND no repeat right remains (the dispatch
// attempts are exhausted or the retry policy answers without the retry
// right: the recorded no_effect unlock is absent — «requiring a human»
// means the automation is spent, never «any failure»). The terminal
// conflict of 3-7 is deliberately NOT a separate trigger: the full
// rollback leaves no record — the conflicting attempt spends the lease
// attempt, the honest trigger word stays reconciliation_exhausted (the
// nuance lives in the reconciliation report words the operator already
// saw; «no word without a material event» — the 2-13 lesson).
const (
	RecoveryTriggerReconciliationExhausted   = "reconciliation_exhausted"
	RecoveryTriggerReconciliationUnavailable = "reconciliation_unavailable"
	RecoveryTriggerOperationFailed           = "operation_failed"
)

// RecoveryTriggerWords is the full dictionary (3 words).
var RecoveryTriggerWords = []string{
	RecoveryTriggerReconciliationExhausted,
	RecoveryTriggerReconciliationUnavailable,
	RecoveryTriggerOperationFailed,
}

// Recovery event kind words — the append-only history vocabulary of the
// Recovery Case (two). created — the birth row of the case (ONE per
// operation forever: the UNIQUE (tenant_id, operation_id) of the store);
// evidence_appended — the later terminal evidence of the 3-7 contour
// (a receipt v3+, new attempts, a fresh external object) pinned as ONE
// SAFE delta row onto the EXISTING case: the original bundle is never
// rewritten, the original receipts are never rewritten (AC-5 «new
// evidence is added to the existing Case» + the AC-10 minimum «the
// context updates by a new append-only reference»). The closing words do
// not exist in this slice (AC-11 is a future slice: the state derives
// read-time «open», a closure would be a NEW dictionary review — never a
// runtime extension).
const (
	RecoveryEventCreated          = "created"
	RecoveryEventEvidenceAppended = "evidence_appended"
)

// RecoveryEventKindWords is the full dictionary (2 words).
var RecoveryEventKindWords = []string{
	RecoveryEventCreated, RecoveryEventEvidenceAppended,
}

// RecoveryCaseStateOpen is the ONE case state word of this slice: the
// state is a READ-TIME derivation over the append-only events (the
// closing kinds do not exist in the dictionary — the case is open BY
// CONSTRUCTION; «no word without a material event» — the 2-13 lesson).
const RecoveryCaseStateOpen = "open"

// RecoveryCaseStateWords is the full dictionary (1 word).
var RecoveryCaseStateWords = []string{RecoveryCaseStateOpen}

// Recovery repeat decision words — the comparator vocabulary of the
// repeated command (two): idempotent — the fresh SAFE bundle digest
// equals the stored one, the existing case returns (200, the honest
// word, NO second row, NO event); evidence — the digest moved (later
// terminal evidence), ONE append-only evidence_appended row lands with
// the SAFE delta. The digest comparator is pinned by the unit matrix for
// the future claim-triggers too (the 3-6/3-7 discipline: every path with
// a recorded trace compares BEFORE any write).
const (
	RecoveryRepeatIdempotent = "idempotent"
	RecoveryRepeatEvidence   = "evidence"
)

// RecoveryRepeatWords is the full dictionary (2 words).
var RecoveryRepeatWords = []string{RecoveryRepeatIdempotent, RecoveryRepeatEvidence}

// ---------------------------------------------------------------------------
// The bounded constants (the 3-3/3-6/3-7 precedents)
// ---------------------------------------------------------------------------

const (
	// RecoverySafeDetailsRunes is the rune bound of the SAFE details
	// column of the recovery events (the operation-ledger bound
	// precedent — the store CHECK mirrors it; identifiers, words and
	// windows only, never a material value).
	RecoverySafeDetailsRunes = 500
	// RecoveryObservationsPinBound is the structural bound of the
	// supporting-observations pins of the SAFE bundle: the preview's own
	// list pins at most this many versioned observation references (a
	// defensive bound — the store CHECK of 00005 mirrors it in the
	// bounded JSON column; the material of the observations NEVER
	// travels, only the version pins).
	RecoveryObservationsPinBound = 8
)

// RecoveryKeyPrefix is the key-space prefix of the recovery key: the
// FOURTH family after commitment:v1:/external_commitment:v1:/
// action_preview:v1: (2-9/3-3/3-4 — the per-tenant commitment key v1
// reuse, the INSERT-only ensure pattern; business_intent:v1: and
// operation_material:v1: ride the same key). The families never collide.
const RecoveryKeyPrefix = "recovery_key:v1:"

// ---------------------------------------------------------------------------
// The read-time eligibility ladder over the RECORDED facts
// ---------------------------------------------------------------------------

// RecoveryEligibilityFacts are the RECORDED facts of ONE operation the
// read-time ladder derives from — nothing here is a command parameter or
// a live network answer: the state word is the ledger derivation, the
// attempt counters are the recorded lease rows of the 3-6/3-7 contours,
// the declaredness halves are the definition's own query word and the
// LIVE connector declaration word read through the composition, the
// wired flag is the composition fact (AG-4: a nil reconciliation adapter
// is the honest production boundary, not an error), and the retry inputs
// are the recorded facts of the last attempt. The store read of step-02
// joins them in ONE query — the eligibility is a pure function over the
// recorded state (NEVER a write on any gas: the unit matrix pins the
// zero-write property with the fake-repo counters).
type RecoveryEligibilityFacts struct {
	// StateWord is the recorded operation state word (the ledger
	// derivation — the ONLY state truth; the foreign word fails closed).
	StateWord string
	// ReconcileAttemptCount is the recorded reconciliation lease rows of
	// the operation (the exhaustion input — MaxReconciliationAttempts=3).
	ReconcileAttemptCount int
	// ReconciliationQueryWord is the definition's DeliveryContract query
	// word ("" — the definition declares no reconciliation query).
	ReconciliationQueryWord string
	// ReconciliationLiveWord is the LIVE word of the connector capability
	// declaration read at the command moment ("" — nothing declared).
	ReconciliationLiveWord string
	// ReconciliationWired reports whether the reconciliation adapter is
	// wired (the composition fact — AG-4 nil is the honest production).
	ReconciliationWired bool
	// DispatchAttemptCount is the recorded dispatch lease rows of the
	// operation (the bound input — MaxDispatchAttempts=3).
	DispatchAttemptCount int
	// CapabilityClass is the delivery contract class word of the
	// definition (the retry policy input of the failed branch).
	CapabilityClass string
	// GuaranteeRecorded is the recorded provider_idempotent guarantee of
	// the last dispatch attempt (the retry policy input).
	GuaranteeRecorded bool
	// NoEffectProven is the recorded reconciliation verdict no_effect of
	// the operation (the 3-7 retry-unlock fact — the retry right).
	NoEffectProven bool
}

// RecoveryEligibility is the ladder answer: the trigger word ("" — NOT
// eligible) with the honest refusal text of the gas ("" when eligible).
// An ineligible answer is NOT an error — the command refuses by the word
// (403 with the text); only a foreign input word is an error (fail-closed).
type RecoveryEligibility struct {
	// Trigger is the trigger word of the eligible operation ("" when the
	// operation is not eligible — the refusal text carries the reason).
	Trigger string
	// RefusalText is the honest refusal word of the gas ("" when
	// eligible — the ladder never writes on any gas).
	RefusalText string
}

// Eligible reports whether the ladder answered with a trigger word.
func (e RecoveryEligibility) Eligible() bool { return e.Trigger != "" }

// DeriveRecoveryEligibility derives the read-time recovery eligibility of
// ONE operation over the RECORDED facts — the ordered ladder (the spec
// order: state → the reconciliation lease counter → the declaredness →
// the retry policy):
//
//   - committed — the terminal success: the honest refusal «recovery is
//     not required» (the operation is DONE, no work object exists);
//   - authorized/submitted/accepted — the operation is in flight (the
//     queue, the write-ahead send, the awaited business result): the
//     honest refusal «still executing» — recovery is premature;
//   - unknown — the reconciliation ladder: the recorded lease counter at
//     the bound (3/3) answers reconciliation_exhausted (the recorded
//     fact WINS over the declaredness — the automatic contour had its
//     chance); below the bound, an unwired adapter or an undeclared
//     semantics (either half — the query word or the LIVE word) answers
//     reconciliation_unavailable (the automatic contour does not exist
//     for this operation — the AG-4 production path: a human is
//     mandatory); below the bound with both halves alive — the honest
//     refusal «the automatic reconciliation is not exhausted» (the
//     automation right is not taken away, the 3-7 contour speaks first);
//   - failed — the repeat ladder: the dispatch counter at the bound
//     (3/3) answers operation_failed; below the bound, the RECORDED
//     retry policy of the last attempt (DeriveDispatchRetryPolicy — the
//     single source of the repeat truth) answers retry (the no_effect
//     unlock inside the budget) with the honest refusal «a repeat is
//     available by the policy decision»; any other policy word (decided
//     without the recorded no_effect, park_bound) answers
//     operation_failed — «requiring a human» = the automation is spent.
//
// Fail-closed on a foreign state word (the six-word dictionary is the
// frozen truth). The ladder NEVER writes: the callers pin it with the
// fake-repo counters (step-02); a failed operation with zero dispatch
// attempts is a structural corruption (the failed word rides an outcome
// of an attempt) — the honest fail-closed error, never a masked verdict.
func DeriveRecoveryEligibility(f RecoveryEligibilityFacts) (RecoveryEligibility, error) {
	if !WordIn(OperationStateWords, f.StateWord) {
		return RecoveryEligibility{}, RuleError{Rule: "DeriveRecoveryEligibility", Reason: fmt.Sprintf(
			"Состояние Operation «%s» вне словаря (%s).", f.StateWord, strings.Join(OperationStateWords, ", "))}
	}
	switch f.StateWord {
	case OperationCommitted:
		return RecoveryEligibility{RefusalText: RecoveryIneligibleCommittedText}, nil
	case OperationAuthorized, OperationSubmitted, OperationAccepted:
		return RecoveryEligibility{RefusalText: RecoveryIneligibleInFlightText}, nil
	case OperationUnknown:
		if f.ReconcileAttemptCount >= MaxReconciliationAttempts {
			return RecoveryEligibility{Trigger: RecoveryTriggerReconciliationExhausted}, nil
		}
		declared := strings.TrimSpace(f.ReconciliationQueryWord) != "" &&
			strings.TrimSpace(f.ReconciliationLiveWord) != ""
		if !f.ReconciliationWired || !declared {
			return RecoveryEligibility{Trigger: RecoveryTriggerReconciliationUnavailable}, nil
		}
		return RecoveryEligibility{RefusalText: RecoveryIneligibleReconcileNotExhaustedText}, nil
	case OperationFailed:
		if f.DispatchAttemptCount >= MaxDispatchAttempts {
			return RecoveryEligibility{Trigger: RecoveryTriggerOperationFailed}, nil
		}
		if f.DispatchAttemptCount < 1 {
			return RecoveryEligibility{}, RuleError{Rule: "DeriveRecoveryEligibility", Reason: fmt.Sprintf(
				"Failed-операция без dispatch-попыток структурно невозможна (слово failed едет исходом попытки): порча данных, отказ (fail-closed).")}
		}
		policy, err := DeriveDispatchRetryPolicy(f.StateWord, f.CapabilityClass, f.GuaranteeRecorded,
			f.DispatchAttemptCount, f.NoEffectProven)
		if err != nil {
			return RecoveryEligibility{}, err
		}
		if policy == DispatchRetryRetry {
			return RecoveryEligibility{RefusalText: RecoveryIneligibleRetryAvailableText}, nil
		}
		return RecoveryEligibility{Trigger: RecoveryTriggerOperationFailed}, nil
	}
	return RecoveryEligibility{}, RuleError{Rule: "DeriveRecoveryEligibility", Reason: fmt.Sprintf(
		"Состояние Operation «%s» вне словаря (%s).", f.StateWord, strings.Join(OperationStateWords, ", "))}
}

// ---------------------------------------------------------------------------
// The recovery key: the keyed identity of ONE CASE per operation
// ---------------------------------------------------------------------------

// DeriveRecoveryKey derives the tenant-scoped recovery key of one
// operation: HMAC-SHA256(key, "recovery_key:v1:" + tenant + FS +
// operation id) in hex — the 64-hex recovery_key column of the case row,
// the UNIQUE (tenant_id, operation_id) structural twin (the key derives
// FROM the operation pin, so a material-object conflict against the key
// is impossible in this slice — the comparator is still pinned by the
// unit matrix for the future claim-triggers, the 3-6/3-7 discipline).
// The prefix separates the fourth key family from commitment:v1:/
// external_commitment:v1:/action_preview:v1: (2-9/3-3/3-4) and from
// business_intent:v1:/operation_material:v1: (operation.go) — the spaces
// never mix; the key is the per-tenant commitment key v1 (the
// commitments_hmac_keys INSERT-only ensure reuse). ONE case per failed
// operation FOREVER (the reopen of a closed case is a transition of the
// SAME case — the future AC-11 slice): the UNIQUE holds over the whole
// history, not only the active rows.
func DeriveRecoveryKey(key []byte, tenant id.UUID, operationID id.UUID) string {
	fields := []string{
		tenant.String(),
		operationID.String(),
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(RecoveryKeyPrefix))
	mac.Write([]byte(strings.Join(fields, canonicalFieldSep)))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// The SAFE link-bundle: the exact-version pins of the whole evidence
// chain (AD-10/AC-9 — references instead of copies)
// ---------------------------------------------------------------------------

// RecoveryBundle is the SAFE link-bundle of ONE Recovery Case: the
// origin pins (the case, the work item, the operation, the definition),
// the execution-manifest pins (the preview and the confirmation with
// their EXACT versions), the authorization pins (the owner triple with
// the epoch, the release decision, the profile version, the two key
// words and the material digest), the attempt counters of both contours,
// the receipt-chain pins (the version count, the last version and its
// outcome), the supporting-observations pins of the preview (the 2-7
// version pins — bounded), the party pins (the target and the owning
// system) and the external identity triple of the last receipt
// (NULLable — the honest unknown). IDENTIFIERS, VERSIONS, COUNTERS,
// WORDS AND 64-HEX DIGESTS ONLY: the material NEVER enters (the
// sensitive payload stays in its own immutable rows — the minimal
// references instead of the copies, AD-10/AC-9); the phone, the
// template and the typed-input values are structurally absent.
type RecoveryBundle struct {
	// The origin pins.
	CaseID        id.UUID `json:"case_id"`
	WorkItemID    id.UUID `json:"work_item_id"`
	OperationID   id.UUID `json:"operation_id"`
	DefinitionID  string  `json:"definition_id"`
	DefinitionVer string  `json:"definition_version"`
	// The execution-manifest pins (the EXACT versions).
	PreviewID             id.UUID `json:"preview_id"`
	PreviewVersionNo      int64   `json:"preview_version_no"`
	ConfirmationID        id.UUID `json:"confirmation_id"`
	ConfirmationVersionNo int64   `json:"confirmation_version_no"`
	// The authorization pins.
	OwnerKind         string  `json:"owner_kind"`
	OwnerRef          id.UUID `json:"owner_ref"`
	OwnerEpoch        int64   `json:"owner_epoch"`
	ReleaseDecisionNo int64   `json:"release_decision_no"`
	ProfileVersion    int64   `json:"profile_version"`
	BusinessIntentKey string  `json:"business_intent_key"`
	IdempotencyKey    string  `json:"idempotency_key"`
	MaterialDigest    string  `json:"material_digest"`
	// The attempt counters of both contours.
	DispatchAttempts  int `json:"dispatch_attempts"`
	ReconcileAttempts int `json:"reconcile_attempts"`
	// The receipt-chain pins (the living chain derives at the read — the
	// bundle pins the frozen moment of the creation/append).
	ReceiptVersionsCount int64  `json:"receipt_versions_count"`
	LatestReceiptVersion int64  `json:"latest_receipt_version_no"`
	LatestReceiptOutcome string `json:"latest_receipt_outcome"` // "" — no receipt yet
	// The supporting-observations pins of the preview (the 2-7 version
	// pins, bounded by RecoveryObservationsPinBound).
	Observations []VersionPin `json:"observations"`
	// The party pins.
	TargetKind   string  `json:"target_kind"`
	TargetRef    id.UUID `json:"target_ref"`
	OwningSystem string  `json:"owning_system"`
	// The external identity of the last receipt (NULLable — the honest
	// unknown: "" fields speak the absence, never an invented outcome).
	ExternalObjectID      string `json:"external_object_id"`
	ExternalObjectVersion string `json:"external_object_version"`
	ExternalObjectTime    string `json:"external_object_time"` // RFC3339 or ""
}

// sortedObservationsPins renders the observations pins deterministically
// (the sortedPins precedent of preview.go — the order never changes the
// digest of the same set).
func sortedObservationsPins(pins []VersionPin) string {
	ordered := append([]VersionPin(nil), pins...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Ref.String() != ordered[j].Ref.String() {
			return ordered[i].Ref.String() < ordered[j].Ref.String()
		}
		return ordered[i].Version < ordered[j].Version
	})
	records := make([]string, 0, len(ordered))
	for _, pin := range ordered {
		records = append(records, pin.Ref.String()+canonicalRecordSep+fmt.Sprintf("%d", pin.Version))
	}
	return strings.Join(records, canonicalGroupSep)
}

// canonicalBundleFields renders the canonical field list of the SAFE
// bundle digest: every pin in the fixed order, the separators of the
// preview.go precedent (the ASCII unit separator between the top-level
// fields, the record/group separators inside the nested list).
func canonicalBundleFields(b RecoveryBundle) []string {
	return []string{
		b.CaseID.String(),
		b.WorkItemID.String(),
		b.OperationID.String(),
		b.DefinitionID,
		b.DefinitionVer,
		b.PreviewID.String(),
		fmt.Sprintf("%d", b.PreviewVersionNo),
		b.ConfirmationID.String(),
		fmt.Sprintf("%d", b.ConfirmationVersionNo),
		b.OwnerKind,
		b.OwnerRef.String(),
		fmt.Sprintf("%d", b.OwnerEpoch),
		fmt.Sprintf("%d", b.ReleaseDecisionNo),
		fmt.Sprintf("%d", b.ProfileVersion),
		b.BusinessIntentKey,
		b.IdempotencyKey,
		b.MaterialDigest,
		fmt.Sprintf("%d", b.DispatchAttempts),
		fmt.Sprintf("%d", b.ReconcileAttempts),
		fmt.Sprintf("%d", b.ReceiptVersionsCount),
		fmt.Sprintf("%d", b.LatestReceiptVersion),
		b.LatestReceiptOutcome,
		sortedObservationsPins(b.Observations),
		b.TargetKind,
		b.TargetRef.String(),
		b.OwningSystem,
		b.ExternalObjectID,
		b.ExternalObjectVersion,
		b.ExternalObjectTime,
	}
}

// DeriveBundleDigest derives the canonical digest of the SAFE bundle:
// HMAC-SHA256(key, "recovery_bundle:v1:" + the canonical field bytes) in
// hex — the PreviewDigest precedent (the keyed identity of the whole
// shown row over the canonical bytes with the separators; the material
// never enters, and the key keeps the digest unsalted-hash-free — the
// AD-10 module discipline: every digest of the contour is a keyed HMAC,
// the single legacy raw SHA-256 of the module is the 3-4 proposal key
// only). The digest is the repeat comparator of the command (equal =
// the idempotent return, a delta = ONE evidence_appended row) and the
// integrity pin of the frozen creation moment — the LIVING chain of the
// card derives at the read (AC-10: the context updates by references).
func DeriveBundleDigest(key []byte, b RecoveryBundle) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("recovery_bundle:v1:"))
	mac.Write([]byte(strings.Join(canonicalBundleFields(b), canonicalFieldSep)))
	return hex.EncodeToString(mac.Sum(nil))
}

// DeriveRecoveryRepeatDecision compares the stored bundle digest against
// the fresh derivation of the SAME operation facts (the 3-6/3-7 digest
// comparator discipline: every path with a recorded trace compares
// BEFORE any write): equal answers idempotent (the existing case
// returns — no second row, no event), a delta answers evidence (ONE
// append-only evidence_appended row with the SAFE delta). Fail-closed on
// a malformed digest (the 64-hex shape is the store CHECK — a foreign
// shape is a corruption alarm, never a masked equality).
func DeriveRecoveryRepeatDecision(storedBundleDigest, freshBundleDigest string) (string, error) {
	if !isHex64(storedBundleDigest) || !isHex64(freshBundleDigest) {
		return "", RuleError{Rule: "DeriveRecoveryRepeatDecision", Reason: fmt.Sprintf(
			"Bundle digest обязан быть 64-hex (SHA-256): порча данных, отказ (fail-closed).")}
	}
	if storedBundleDigest == freshBundleDigest {
		return RecoveryRepeatIdempotent, nil
	}
	return RecoveryRepeatEvidence, nil
}

// isHex64 reports the exact 64-lowercase-hex shape (the digest columns).
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// DeriveRecoveryEvidenceDelta renders the SAFE delta word of the
// evidence append: the names of the CHANGED pins with their old→new
// values — identifiers, versions, counters, words and the external
// identity ONLY (NEVER a material value, AD-10) — joined into ONE
// bounded line (RecoverySafeDetailsRunes, the store CHECK backstop).
// The delta speaks what moved between the frozen creation bundle and
// the fresh derivation of the SAME operation facts: the later terminal
// evidence of the 3-7 contour (a receipt v3+, new attempts, a fresh
// external object). The LONG key words (business_intent_key,
// idempotency_key, material_digest) speak by NAME only — their values
// are identities, not delta material; an unchanged set answers the
// honest «no pin moved» word (the comparator discipline: the delta row
// lands ONLY on a digest change, the equal case answered idempotent
// BEFORE this derivation).
func DeriveRecoveryEvidenceDelta(stored, fresh RecoveryBundle) string {
	segments := []string{}
	word := func(name, oldV, newV string) {
		if oldV != newV {
			segments = append(segments, fmt.Sprintf("%s %s→%s", name, oldV, newV))
		}
	}
	word("preview-версия", fmt.Sprintf("%d", stored.PreviewVersionNo), fmt.Sprintf("%d", fresh.PreviewVersionNo))
	word("confirmation-версия", fmt.Sprintf("%d", stored.ConfirmationVersionNo), fmt.Sprintf("%d", fresh.ConfirmationVersionNo))
	word("owner-эпоха", fmt.Sprintf("%d", stored.OwnerEpoch), fmt.Sprintf("%d", fresh.OwnerEpoch))
	word("dispatch-попытки", fmt.Sprintf("%d", stored.DispatchAttempts), fmt.Sprintf("%d", fresh.DispatchAttempts))
	word("reconcile-попытки", fmt.Sprintf("%d", stored.ReconcileAttempts), fmt.Sprintf("%d", fresh.ReconcileAttempts))
	word("квант-квитанций", fmt.Sprintf("%d", stored.ReceiptVersionsCount), fmt.Sprintf("%d", fresh.ReceiptVersionsCount))
	word("последняя-квитанция", fmt.Sprintf("%d", stored.LatestReceiptVersion), fmt.Sprintf("%d", fresh.LatestReceiptVersion))
	word("исход-квитанции", stored.LatestReceiptOutcome, fresh.LatestReceiptOutcome)
	word("внешний-id", stored.ExternalObjectID, fresh.ExternalObjectID)
	word("внешняя-версия", stored.ExternalObjectVersion, fresh.ExternalObjectVersion)
	word("внешнее-время", stored.ExternalObjectTime, fresh.ExternalObjectTime)
	word("наблюдения", fmt.Sprintf("%d", len(stored.Observations)), fmt.Sprintf("%d", len(fresh.Observations)))
	if len(segments) == 0 {
		return "Дельта доказательств: ни один SAFE-пин не двигался (дайджест разошёлся вне пинов среза)."
	}
	delta := "Дельта доказательств: " + strings.Join(segments, "; ")
	runes := []rune(delta)
	if len(runes) > RecoverySafeDetailsRunes {
		return string(runes[:RecoverySafeDetailsRunes])
	}
	return delta
}

// ---------------------------------------------------------------------------
// The assignment: the LIVE readings ONLY (the coverageWord canon)
// ---------------------------------------------------------------------------

// RecoveryAssignmentInput is the LIVE-reading input of the assignment
// derivation: the current ownership term of the ORIGINAL Case (the term
// of the maximal epoch — CaseView.Ownership.State.Current; no terms =
// the honest owner-unknown refusal) and the human coverage of the
// ORIGINAL conversation route (the RouteFactsSource read — the channel
// by the modality, the inbound candidate; an unreadable coverage = the
// honest coverage-unavailable refusal). The words NEVER come from the
// command parameters (the coverageWord canon of cases/app: the
// assignment is material, the assignment is mandatory — «assigns» never
// happens without the assignment, AC-1).
type RecoveryAssignmentInput struct {
	// HasOwnerTerm reports whether the original Case carries a current
	// ownership term (false — the pre-migration honest state).
	HasOwnerTerm bool
	// The current term of the original Case (the maximal epoch).
	OwnerKind  string  `json:"owner_kind"`
	OwnerRef   id.UUID `json:"owner_ref"`
	OwnerEpoch int64   `json:"owner_epoch"`
	// CoverageReadable reports whether the human coverage of the original
	// conversation route was read (false — the fail-closed refusal).
	CoverageReadable bool
	// The human coverage of the route (the inbound candidate of the
	// channel by the modality — the coverageWord canon).
	CoverageBackupOwnerID      id.UUID `json:"coverage_backup_owner_id"`
	CoverageAckDeadlineMinutes int64   `json:"coverage_ack_deadline_minutes"`
}

// RecoveryAssignment is the derived assignment of ONE Recovery Case: the
// owner (the CURRENT response owner of the original Case — whoever owns
// the conversation owns the recovery), the backup owner and the
// ack-deadline minutes of the human coverage (the deadline_at derives at
// the creation transaction: now + minutes, the request-transfer
// precedent), all from the LIVE readings.
type RecoveryAssignment struct {
	OwnerKind          string  `json:"owner_kind"`
	OwnerRef           id.UUID `json:"owner_ref"`
	OwnerEpoch         int64   `json:"owner_epoch"`
	BackupOwnerID      id.UUID `json:"backup_owner_id"`
	AckDeadlineMinutes int64   `json:"ack_deadline_minutes"`
}

// DeriveRecoveryAssignment derives the assignment of the case from the
// LIVE readings — the two fail-closed refusals are the honest boundary
// of the mandatory assignment (AC-1):
//
//   - no current ownership term of the original Case (the pre-migration
//     state) — the honest refusal «the owner of the original Case is
//     unknown — the ownership contour (2-4) first»: the case is NOT
//     created, nothing is written;
//   - the coverage unreadable (the port not wired, the route not
//     readable, or the ack-deadline word missing — minutes < 1, the
//     request-transfer precedent) — the honest refusal «the coverage is
//     unavailable»: the deadline and the backup are never invented, the
//     case is NOT created, nothing is written.
//
// The derivation takes NO adapter and NO command parameter — the
// signature is the locality property itself (the reflect probe of
// step-02 pins the whole command path).
func DeriveRecoveryAssignment(in RecoveryAssignmentInput) (RecoveryAssignment, error) {
	if !in.HasOwnerTerm {
		return RecoveryAssignment{}, RuleError{Rule: "DeriveRecoveryAssignment",
			Reason: RecoveryOwnerUnknownText}
	}
	if !in.CoverageReadable {
		return RecoveryAssignment{}, RuleError{Rule: "DeriveRecoveryAssignment",
			Reason: RecoveryCoverageUnavailableText}
	}
	if in.CoverageAckDeadlineMinutes < 1 {
		return RecoveryAssignment{}, RuleError{Rule: "DeriveRecoveryAssignment",
			Reason: RecoveryCoverageUnavailableText}
	}
	return RecoveryAssignment{
		OwnerKind:          in.OwnerKind,
		OwnerRef:           in.OwnerRef,
		OwnerEpoch:         in.OwnerEpoch,
		BackupOwnerID:      in.CoverageBackupOwnerID,
		AckDeadlineMinutes: in.CoverageAckDeadlineMinutes,
	}, nil
}

// DeriveRecoveryDeadlineAt derives the deadline moment of the case: the
// creation moment plus the ack-deadline minutes of the coverage, in UTC
// with the microsecond truncation (the request-transfer precedent — the
// store column shape stays stable across the drivers).
func DeriveRecoveryDeadlineAt(now time.Time, ackDeadlineMinutes int64) time.Time {
	return now.Add(time.Duration(ackDeadlineMinutes) * time.Minute).UTC().Truncate(time.Microsecond)
}

// DeriveRecoveryCaseState derives the read-time state word of ONE case
// over its append-only events: every kind must be in the event
// dictionary (fail-closed on a foreign word — the honest refusal, never
// a masked silence), and the state is open BY CONSTRUCTION (the closing
// kinds do not exist in the dictionary of this slice — the AC-11 closure
// is a future dictionary review, never a runtime extension; «no word
// without a material event» — the 2-13 lesson).
func DeriveRecoveryCaseState(events []RecoveryEventRow) (string, error) {
	for _, e := range events {
		if !WordIn(RecoveryEventKindWords, e.Kind) {
			return "", RuleError{Rule: "DeriveRecoveryCaseState", Reason: fmt.Sprintf(
				"Событие Recovery Case «%s» вне словаря (%s).", e.Kind, strings.Join(RecoveryEventKindWords, ", "))}
		}
	}
	return RecoveryCaseStateOpen, nil
}

// RecoveryEventRow is one append-only row of the recovery history (the
// domain shape of the store row — the monotonic event number under the
// case advisory lock, the kind word, the SAFE details within the rune
// bound, the actor and the moment).
type RecoveryEventRow struct {
	EventNo   int64     `json:"event_no"`
	Kind      string    `json:"kind"`
	Details   string    `json:"details"`
	ActorID   id.UUID   `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// The honest texts of the recovery surfaces (AC-12/FR-38)
// ---------------------------------------------------------------------------

// The fixed honest texts of the recovery surfaces: the card, the API
// view and the page share these exact texts (an app-private copy would
// drift — the operation-view precedent). The unknown stays unknown, the
// local projection never dresses up as the external state, the payment
// state is spoken honestly (AG-4: no payment systems are wired).
const (
	// RecoveryIneligibleCommittedText is the committed refusal word.
	RecoveryIneligibleCommittedText = "Восстановление не требуется: итог committed — терминальный успех подтверждён структурным evidence."
	// RecoveryIneligibleInFlightText is the in-flight refusal word.
	RecoveryIneligibleInFlightText = "Операция ещё в исполнении (authorized/submitted/accepted): очередь, отправка и сверка живы — восстановление преждевременно."
	// RecoveryIneligibleReconcileNotExhaustedText is the not-exhausted
	// refusal word: the automation right is not taken away.
	RecoveryIneligibleReconcileNotExhaustedText = "Автоматическая сверка не исчерпана — сначала контур сверки (3-7): право автоматики не отбирается."
	// RecoveryIneligibleRetryAvailableText is the retry-right refusal
	// word: the policy still answers retry.
	RecoveryIneligibleRetryAvailableText = "Повтор доступен решением политики — восстановление преждевременно: автоматика не исчерпана."
	// RecoveryCoverageUnavailableText is the fail-closed coverage word
	// (the CoverageUnavailableText precedent of the cases domain).
	RecoveryCoverageUnavailableText = "Human coverage маршрута исходной conversation недоступна: дедлайн и backup не выдумываются — Recovery Case не создаётся (fail-closed)."
	// RecoveryOwnerUnknownText is the fail-closed owner word.
	RecoveryOwnerUnknownText = "Владелец исходного Case неизвестен (термов Ownership нет — pre-migration): сначала контур владельцев (2-4) — Recovery Case не создаётся (fail-closed)."
	// RecoveryCreatedText is the honest 201 word of the creation.
	RecoveryCreatedText = "Recovery Case создан: назначение живыми чтениями (владелец — текущий терм исходного Case, backup и дедлайн — human coverage маршрута), доказательства — SAFE-пины точных версий; исходные записи не переписаны."
	// RecoveryIdempotentText is the honest 200 word of the repeat with
	// the unchanged bundle (AC-5: no second row, no event).
	RecoveryIdempotentText = "Recovery Case уже существует: возвращен существующий кейс — второй строки и события нет (идемпотентность UNIQUE (tenant, operation))."
	// RecoveryEvidenceAppendedText is the honest 200 word of the repeat
	// with the changed bundle (the later terminal evidence).
	RecoveryEvidenceAppendedText = "Новые доказательства добавлены: ОДНА append-only строка evidence_appended с SAFE-дельтой — исходный bundle и исходные квитанции не переписаны."
	// RecoveryPaymentsUnknownText speaks the honest payment boundary
	// (AG-4: the payment systems are not wired — never a masked state).
	RecoveryPaymentsUnknownText = "Платёжное состояние unknown: платёжные системы не подключены (AG-4) — система не выдумывает ни успех, ни отказ."
	// RecoveryLocalProjectionText speaks the local-projection boundary
	// (the freshness is confirmed by the reconciliation only).
	RecoveryLocalProjectionText = "Локальная проекция — не актуальное внешнее состояние: актуальность подтверждается сверкой (контур 3-7)."
	// RecoveryCaseOpenText speaks the one state word of the slice.
	RecoveryCaseOpenText = "Recovery Case открыт (open): закрывающих событий в словаре среза нет — закрытие и уведомление клиента остаются будущим срезом (AC-11)."
	// RecoveryNextStepText is the next-step word of an ELIGIBLE operation
	// (the domain word change of the journeys z36 — rule 161: the slice
	// changes the next-step word of the eligible operations, the sьют is
	// mandatory).
	RecoveryNextStepText = "Следующий шаг: требуется человек — создайте Recovery Case (контур 3-8): автоматика исчерпана либо недоступна, операция остаётся в своём состоянии."
)

// RecoveryTriggerTexts is the fixed honest text of every trigger word
// (the card speaks the trigger BY WORD — the state texts stay
// distinguishable by text, the AC-13 discipline of the operation view).
var RecoveryTriggerTexts = map[string]string{
	RecoveryTriggerReconciliationExhausted:   "Триггер восстановления: автоматические попытки сверки исчерпаны (3/3) — операция остаётся unknown до человеческого разбора.",
	RecoveryTriggerReconciliationUnavailable: "Триггер восстановления: сверка недоступна (не объявлена либо не подключена — AG-4) — автоматического контура нет, человек обязателен.",
	RecoveryTriggerOperationFailed:           "Триггер восстановления: доказанная ошибка без права повтора (попытки исчерпаны либо гарантия отсутствует) — требуется человек.",
}

// RecoveryTriggerText speaks the trigger word; a foreign word fails
// closed (the honest refusal names the dictionary).
func RecoveryTriggerText(word string) string {
	if text, ok := RecoveryTriggerTexts[word]; ok {
		return text
	}
	return fmt.Sprintf("Слово триггера восстановления «%s» не входит в словарь (%s).", word, strings.Join(RecoveryTriggerWords, ", "))
}

// DeriveOperationNextStepText speaks the next step of one operation
// view: an ELIGIBLE operation (the non-empty trigger word — the 3-8
// read-time derivation over the recorded facts) speaks the recovery
// word, any other state speaks the inherited state word text (the
// frozen 3-6 map — the state dictionary and the inherited texts NEVER
// change; the override is additive). The trigger word must be empty or
// in the trigger dictionary — a foreign word fails closed (the honest
// refusal names the dictionary, the 3-6 precedent).
func DeriveOperationNextStepText(stateWord string, eligibilityTrigger string) string {
	if eligibilityTrigger == "" {
		return OperationNextStepText(stateWord)
	}
	if !WordIn(RecoveryTriggerWords, eligibilityTrigger) {
		return fmt.Sprintf("Слово триггера восстановления «%s» не входит в словарь (%s).",
			eligibilityTrigger, strings.Join(RecoveryTriggerWords, ", "))
	}
	return RecoveryNextStepText
}
