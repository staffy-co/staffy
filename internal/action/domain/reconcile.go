package domain

import (
	"fmt"
	"strings"
	"time"
)

// Module action — story 3.7 slice 1 (step-01): the reconciliation domain
// core (FR-30, the AC-1..AC-13 blocks of epic 3.7). This file owns the
// reconciliation-contour dictionaries (the FOUR verdict words of the
// reconciliation answer, the THREE decision words of the receipt terminal
// lock), the lease/backoff constants of the bounded reconciliation
// (MaxReconciliationAttempts / ReconcileLeaseGraceMS /
// ReconcileBackoffBaseMS / ReconcileBackoffJitterMS /
// ReconcileDrainLimit — the 3-3/3-6 bounded-loop precedents), the
// fail-closed verdict normalization (a committed claim survives ONLY with
// the structural evidence triple — the GuardCommittedEvidence discipline
// of 3-6 reused on the reconciliation answer; a no_effect claim survives
// ONLY under the DECLARED absence semantics — a bare 404 without a
// declared semantics is no_answer, never a proof; a foreign word reads
// as no_answer — never a masked success), the verdict→writes mapping
// (the two slash decisions of the epic: unknown/reconciling — the
// operation STAYS unknown, reconciling is the 8th ledger word, NOT
// state-bearing, owned by operation.go; failed/no_effect — the verdict
// no_effect lands as the failed state with the absence proof riding the
// receipt CONTENT, the dictionaries of states (6) and receipts (3) never
// grow), the receipt append terminal lock (GuardReceiptAppend: an append
// is allowed ONLY over the last unknown version; over a terminal version
// the same terminal claim is the idempotent skip «reconciliation confirms
// the outcome», a conflicting terminal claim is the integrity alarm
// «conflict of terminal outcomes — a human is required», never an
// auto-replacement — epic AC-5), and the honest texts of the
// reconciliation report and the next steps. The words are frozen in
// code; runtime or tenant-config extension is NEVER allowed.
//
// The reconciliation READS, it never sends and never consumes a consent
// (AD-6 «reconcile before retry»): the full
// confirmation/freshness/readiness/policy ladder of the dispatch contour
// (3-6) is deliberately NOT repeated here — the light read-only fence
// ladder of the app layer (step-02) checks the fingerprint integrity,
// the scoped controls, the declared reconciliation and the decided state
// before any query. The raw material never travels through this file:
// the query carries the EXTERNAL idempotency key of the operation
// (dispatch_key:v1: — the same key the dispatch used, so the provider
// recognizes the operation), the correlation id of the last dispatch
// attempt and the keyed request digest (AD-10); the evidence triple of
// the committed verdict is the structural identifiers only.

// ---------------------------------------------------------------------------
// The dictionaries (compile-time, closed)
// ---------------------------------------------------------------------------

// Reconciliation verdict words — the answer vocabulary of the
// reconciliation query (four). committed — the terminal success WITH the
// structural evidence (the external object exists); no_effect — the
// PROVEN absence of the external effect under the declared absence
// semantics (the operation lands failed, the retry unlocks — AC-3/AC-6);
// in_progress — the declared intermediate state of the owning system
// (never terminal, no ledger word, no receipt — the operation stays
// unknown «the owning system is processing»); no_answer — the honest
// absence of a verdict: the call failed, timed out, spoke a foreign word
// or claimed a terminal outcome without the required evidence — unknown
// stays, the retry does NOT unlock. The Owning System never gets the
// terminal success on word (State/Authority — AC-2 of the epic).
const (
	ReconciliationVerdictCommitted  = "committed"
	ReconciliationVerdictNoEffect   = "no_effect"
	ReconciliationVerdictInProgress = "in_progress"
	ReconciliationVerdictNoAnswer   = "no_answer"
)

// ReconciliationVerdictWords is the full dictionary (4 words).
var ReconciliationVerdictWords = []string{
	ReconciliationVerdictCommitted, ReconciliationVerdictNoEffect,
	ReconciliationVerdictInProgress, ReconciliationVerdictNoAnswer,
}

// Receipt append decision words — the terminal lock of the Action
// Receipt chain (three). append — the ONLY allowed write path: a new
// receipt version over the LAST UNKNOWN version (v2+ over v1 unknown,
// the 3-6 append promise executes); skip — the idempotent confirmation:
// the last version is ALREADY terminal with the SAME terminal outcome,
// no new version, no downgrade (AC-5 «a weaker or older event never
// downgrades a confirmed terminal state»); alarm — the integrity
// conflict: the last version is terminal with a DIFFERENT terminal
// outcome — a human review, never an automatic choice (AC-5 «conflicting
// terminal outcomes create a human review instead of an automatic
// selection»).
const (
	ReceiptAppendAppend = "append"
	ReceiptAppendSkip   = "skip"
	ReceiptAppendAlarm  = "alarm"
)

// ReceiptAppendDecisionWords is the full dictionary (3 words).
var ReceiptAppendDecisionWords = []string{
	ReceiptAppendAppend, ReceiptAppendSkip, ReceiptAppendAlarm,
}

// ---------------------------------------------------------------------------
// The bounded reconciliation constants (the 3-3/3-6 precedents)
// ---------------------------------------------------------------------------

const (
	// MaxReconciliationAttempts is the bounded reconciliation limit — one
	// operation never takes more than three reconciliation attempts; the
	// exhaustion speaks the honest human word instead of a blind loop
	// (epic AC-9 «the Operation stays unknown until human evidence, not
	// a success by timer»).
	MaxReconciliationAttempts = 3
	// ReconcileLeaseGraceMS is the reconciliation lease grace: the lease
	// window is the definition timeout PLUS the grace — the lease
	// outlives the possible in-flight query call; an expired lease means
	// the reconciler died mid-flight, the crash recovery continues within
	// the bound (the 3-6 dispatch lease precedent).
	ReconcileLeaseGraceMS = 30000
	// ReconcileBackoffBaseMS is the backoff base between the
	// reconciliation attempts: the next attempt of the same operation is
	// eligible no earlier than now + base + jitter (the honest «the
	// owning system may need time» pause — never a hot loop).
	ReconcileBackoffBaseMS = 30000
	// ReconcileBackoffJitterMS is the backoff jitter ceiling: the store
	// draws the jitter in [0..5000] and records the exact
	// next_eligible_at in the lease row (the materiality of the backoff
	// — AC-1: the command before the window speaks «the reconciliation
	// is not ready until <time>»).
	ReconcileBackoffJitterMS = 5000
	// ReconcileDrainLimit is the bounded reconciliation batch: one
	// reconcile command considers at most this many pending targets —
	// the rest stay honestly for the next run (the cases
	// drain-limiter and DispatchDrainLimit precedents).
	ReconcileDrainLimit = 100
)

// DeriveReconciliationLeaseExpiresAt derives the reconciliation lease
// expiry moment: now + the definition timeout + the grace
// (lease_expires_at — the window outlives the possible in-flight query;
// an expiry under a live call would be false). The timeout is re-checked
// against the definition bounds — the derivation never trusts the caller
// (fail-closed, the DeriveDispatchLeaseExpiresAt precedent).
func DeriveReconciliationLeaseExpiresAt(now time.Time, definitionTimeoutMS int) (time.Time, error) {
	if definitionTimeoutMS < TimeoutMSMin || definitionTimeoutMS > TimeoutMSMax {
		return time.Time{}, RuleError{Rule: "DeriveReconciliationLeaseExpiresAt", Reason: fmt.Sprintf(
			"Таймаут определения %d мс вне границ [%d..%d] — окно lease сверки не выводится (fail-closed).",
			definitionTimeoutMS, TimeoutMSMin, TimeoutMSMax)}
	}
	return now.Add(time.Duration(definitionTimeoutMS)*time.Millisecond +
		time.Duration(ReconcileLeaseGraceMS)*time.Millisecond), nil
}

// DeriveReconciliationBackoffUntil derives the next-eligibility moment of
// the NEXT reconciliation attempt: now + the backoff base + the jitter
// (next_eligible_at — recorded in the lease row of the attempt, the
// materiality of the backoff). The jitter is passed BY THE CALLER and
// re-checked against the ceiling [0..ReconcileBackoffJitterMS] — the
// derivation stays deterministic and testable, the random draw lives in
// the store (fail-closed on a foreign jitter — never a silent clamp).
func DeriveReconciliationBackoffUntil(now time.Time, jitterMS int) (time.Time, error) {
	if jitterMS < 0 || jitterMS > ReconcileBackoffJitterMS {
		return time.Time{}, RuleError{Rule: "DeriveReconciliationBackoffUntil", Reason: fmt.Sprintf(
			"Jitter backoff сверки %d мс вне окна [0..%d] — момент next_eligible_at не выводится (fail-closed).",
			jitterMS, ReconcileBackoffJitterMS)}
	}
	return now.Add(time.Duration(ReconcileBackoffBaseMS+jitterMS) * time.Millisecond), nil
}

// ---------------------------------------------------------------------------
// The declared absence semantics (the no_effect gate — epic AC-3)
// ---------------------------------------------------------------------------

// DeclaredReconciliationSemantics is the DECLARED absence semantics gate
// of the no_effect verdict: the LIVE reconciliation word of the
// connector's CapabilityDeclaration (facts.Reconciliation — «» means
// nothing declared) AND the non-empty ReconciliationQuery word of the
// definition's DeliveryContract. The absence of an external result is
// NEVER derived from a bare HTTP 404 without a declared semantics
// (AC-3 дословно): only the Owning System that DECLARES how its absence
// speaks may prove the absence. Both words are read by the app layer
// (the definition resolve and the live catalog facts) and handed here —
// the domain never resolves the catalog itself (AD-6, the 3-1 gate
// query_reconcilable×LIVE is the definition-side mirror of this gate).
type DeclaredReconciliationSemantics struct {
	// LiveWord is the LIVE reconciliation word of the connector's
	// capability declaration ("" — nothing declared).
	LiveWord string
	// Query is the ReconciliationQuery word of the definition's
	// DeliveryContract ("" — the definition declares no query).
	Query string
}

// Declared reports whether the absence semantics is declared: BOTH the
// LIVE word and the query word are non-empty. A declared query without
// a live word (the connector dropped the declaration) is NOT declared —
// the verdict no_effect refuses to no_answer (fail-closed).
func (s DeclaredReconciliationSemantics) Declared() bool {
	return strings.TrimSpace(s.LiveWord) != "" && strings.TrimSpace(s.Query) != ""
}

// ---------------------------------------------------------------------------
// The verdict normalization (fail-closed — the State/Authority of 3-7)
// ---------------------------------------------------------------------------

// DeriveReconciliationVerdict normalizes the reconciliation answer into
// the four-word verdict dictionary — the total fail-closed function: ANY
// input lands on one of the four words, the least-believing one when the
// claim does not hold. committed survives ONLY with the complete
// external object triple (id/version/time — the GuardCommittedEvidence
// discipline of 3-6 reused: the terminal success is never taken on word
// even from the reconciliation); a committed claim without the evidence
// is REFUSED to no_answer (the attempt is spent, unknown stays, no
// receipt — AC-2). no_effect survives ONLY under the declared absence
// semantics; without the declaration the claim reads no_answer (unknown
// stays, the retry does NOT unlock — AC-3 дословно). in_progress passes
// as the declared intermediate (never terminal). A foreign word, an
// empty word, a call error rendered as a word — everything else reads
// no_answer (the honest absence of a verdict, never a masked success).
func DeriveReconciliationVerdict(claimed string, evidence MutationEvidence, declared DeclaredReconciliationSemantics) string {
	switch claimed {
	case ReconciliationVerdictCommitted:
		if strings.TrimSpace(evidence.ExternalObjectID) == "" ||
			strings.TrimSpace(evidence.ExternalObjectVersion) == "" ||
			evidence.ExternalObjectTime.IsZero() {
			return ReconciliationVerdictNoAnswer
		}
		return ReconciliationVerdictCommitted
	case ReconciliationVerdictNoEffect:
		if !declared.Declared() {
			return ReconciliationVerdictNoAnswer
		}
		return ReconciliationVerdictNoEffect
	case ReconciliationVerdictInProgress:
		return ReconciliationVerdictInProgress
	default:
		return ReconciliationVerdictNoAnswer
	}
}

// ReconciliationVerdictEffects is the WRITE PLAN of one normalized
// verdict: which ledger word the worker appends and which receipt
// outcome a NEW receipt version would carry («» — no write of that
// kind). The two slash decisions of the epic ride here: the
// unknown/reconciling slash — in_progress and no_answer write NEITHER a
// ledger word NOR a receipt (the operation STAYS unknown, the
// reconciliation is an activity in the history, never a state); the
// failed/no_effect slash — no_effect writes the failed state AND a
// failed receipt whose CONTENT carries the absence proof (the
// dictionaries of states (6) and receipts (3) never grow — the proof
// rides the receipt references, not a new word).
type ReconciliationVerdictEffects struct {
	// LedgerWord is the state-bearing ledger word of the verdict («» —
	// the ledger is not written).
	LedgerWord string
	// ReceiptWord is the receipt outcome word of the appended version
	// («» — no receipt version is appended).
	ReceiptWord string
}

// DeriveReconciliationVerdictEffects maps the normalized verdict onto
// its write plan: committed → ledger committed + receipt committed (the
// terminal success with the evidence); no_effect → ledger failed +
// receipt failed (the proven absence — the retry unlocks through the
// dispatch policy extension, DeriveDispatchRetryPolicy of dispatch.go);
// in_progress / no_answer → no ledger word, no receipt (unknown stays).
// Fail-closed on a foreign word: the caller passes the NORMALIZED
// verdict of DeriveReconciliationVerdict — anything else is a
// programming error, never a silent write.
func DeriveReconciliationVerdictEffects(verdict string) (ReconciliationVerdictEffects, error) {
	switch verdict {
	case ReconciliationVerdictCommitted:
		return ReconciliationVerdictEffects{LedgerWord: LedgerEntryCommitted, ReceiptWord: ReceiptOutcomeCommitted}, nil
	case ReconciliationVerdictNoEffect:
		return ReconciliationVerdictEffects{LedgerWord: LedgerEntryFailed, ReceiptWord: ReceiptOutcomeFailed}, nil
	case ReconciliationVerdictInProgress, ReconciliationVerdictNoAnswer:
		return ReconciliationVerdictEffects{}, nil
	default:
		return ReconciliationVerdictEffects{}, RuleError{Rule: "DeriveReconciliationVerdictEffects", Reason: fmt.Sprintf(
			"Вердикт сверки «%s» не входит в словарь (%s) — план записей не выводится (fail-closed, вердикт обязан быть нормализован DeriveReconciliationVerdict).",
			verdict, strings.Join(ReconciliationVerdictWords, ", "))}
	}
}

// ---------------------------------------------------------------------------
// The receipt terminal lock (GuardReceiptAppend — epic AC-5)
// ---------------------------------------------------------------------------

// GuardReceiptAppend decides the append discipline of the Action
// Receipt chain — the TERMINAL LOCK. The last receipt version's outcome
// and the NEW terminal claim decide among the three words: append — the
// last version is unknown, the ONLY path a new version (v2+) may be
// written on (v1 unknown is byte-stable — the append promise of 3-6
// «later evidence appends v2+» executes here); skip — the last version
// is terminal with the SAME outcome (the idempotent confirmation «the
// reconciliation confirms the outcome», no new version, no downgrade);
// alarm — the last version is terminal with a DIFFERENT outcome (the
// integrity conflict «conflict of terminal outcomes — a human is
// required», no auto-replacement — the conflict surfaces for the human
// review, epic AC-5). Both arguments must be receipt outcome words, the
// claim must be TERMINAL (committed/failed — in_progress and no_answer
// never write a receipt: calling this guard for them is a category
// error, fail-closed).
func GuardReceiptAppend(lastReceiptOutcome string, claimedReceiptOutcome string) (string, error) {
	if !WordIn(ReceiptOutcomeWords, lastReceiptOutcome) {
		return "", RuleError{Rule: "GuardReceiptAppend", Reason: fmt.Sprintf(
			"Исход последней квитанции «%s» не входит в словарь (%s).",
			lastReceiptOutcome, strings.Join(ReceiptOutcomeWords, ", "))}
	}
	if claimedReceiptOutcome != ReceiptOutcomeCommitted && claimedReceiptOutcome != ReceiptOutcomeFailed {
		return "", RuleError{Rule: "GuardReceiptAppend", Reason: fmt.Sprintf(
			"Заявка append «%s» не терминальна (%s/%s) — нетерминальные вердикты квитанцию не пишут, замок не спрашивается.",
			claimedReceiptOutcome, ReceiptOutcomeCommitted, ReceiptOutcomeFailed)}
	}
	switch lastReceiptOutcome {
	case ReceiptOutcomeUnknown:
		return ReceiptAppendAppend, nil
	}
	if lastReceiptOutcome == claimedReceiptOutcome {
		return ReceiptAppendSkip, nil
	}
	return ReceiptAppendAlarm, nil
}

// ---------------------------------------------------------------------------
// The honest texts of the reconciliation contour (the report words)
// ---------------------------------------------------------------------------

// The fixed honest texts of the reconciliation verdicts — the card's
// truth of the reconciliation result (AC-13 of the epic: the verdicts
// are distinguishable BY TEXT, the surface never invents an outcome).
const (
	// ReconciliationCommittedText speaks the terminal success WITH the
	// evidence — the new receipt version carries the identifiers.
	ReconciliationCommittedText = "Сверка подтвердила терминальный успех (committed): внешний объект существует, структурные evidence (идентификатор, версия, время) записаны в квитанции v2+."
	// ReconciliationNoEffectText speaks the PROVEN absence — the
	// operation lands failed, the retry unlocks by the policy decision.
	ReconciliationNoEffectText = "Сверка доказала отсутствие эффекта (no_effect): внешнего объекта нет по объявленной семантике отсутствия — операция завершена как failed, повтор разрешён решением политики."
	// ReconciliationInProgressText speaks the declared intermediate —
	// never terminal, the operation honestly stays unknown.
	ReconciliationInProgressText = "Система-владелец обрабатывает (in_progress): итога ещё нет, операция остаётся unknown — новая попытка сверки по окну backoff."
	// ReconciliationNoAnswerText speaks the honest absence of a verdict
	// — never a success and never a proof of absence.
	ReconciliationNoAnswerText = "Сверка не дала вердикта (no_answer): ответ не получен либо непризнан — операция остаётся unknown, повтор не разблокирован."
)

// ReconciliationVerdictTexts is the fixed honest text of every verdict
// word; a foreign word fails closed (the honest refusal names the
// dictionary).
var ReconciliationVerdictTexts = map[string]string{
	ReconciliationVerdictCommitted:  ReconciliationCommittedText,
	ReconciliationVerdictNoEffect:   ReconciliationNoEffectText,
	ReconciliationVerdictInProgress: ReconciliationInProgressText,
	ReconciliationVerdictNoAnswer:   ReconciliationNoAnswerText,
}

// ReconciliationVerdictText speaks the verdict word; a foreign word
// fails closed.
func ReconciliationVerdictText(word string) string {
	if text, ok := ReconciliationVerdictTexts[word]; ok {
		return text
	}
	return fmt.Sprintf("Вердикт сверки «%s» не входит в словарь (%s).", word, strings.Join(ReconciliationVerdictWords, ", "))
}

// The fixed honest texts of the receipt terminal lock decisions — the
// append discipline speaks itself (AC-5: the idempotent confirmation and
// the integrity conflict are distinguishable BY TEXT, never a silent
// divergence).
const (
	// ReceiptAppendSkipText speaks the idempotent confirmation: the
	// terminal outcome is already recorded — no new version, no
	// downgrade.
	ReceiptAppendSkipText = "Сверка подтверждает итог: терминальная квитанция уже записана с тем же исходом — новая версия не создаётся, понижения нет (идемпотентность терминального замка)."
	// ReceiptAppendAlarmText speaks the integrity conflict — the human
	// review word, never an auto-replacement.
	ReceiptAppendAlarmText = "Конфликт терминальных итогов — требуется человек: поздний вердикт противоречит записанной терминальной квитанции, авто-замены нет."
)

// The fixed honest texts of the reconciliation report lines — the fence
// ladder of the reconcile command speaks EVERY refusal by the exact
// cause (FR-30: the operator sees at which stage and why the
// reconciliation stopped; the fences are report-only — no ledger word is
// written for a transient block, the honest word lives in the report of
// the command and in the next step of the operation view).
const (
	// ReconciliationNotWiredText is the AG-4 honest refusal: the
	// production binary has no owning systems — the reconciliation
	// refuses in words, the queue is never touched (the DrainExternal
	// Commitments precedent of 3-3).
	ReconciliationNotWiredText = "Сверка не подключена — реальных систем-владельцев нет (AG-4): команда отказывает честно, очередь сверки не тронута, запросов ноль."
	// ReconciliationNotDeclaredText is the fence word of an undeclared
	// reconciliation (AC-9-minimum of the epic — the manual task of 3-8).
	ReconciliationNotDeclaredText = "Сверка не объявлена — требуется человек: определение не объявляет запроса сверки либо коннектор не объявляет LIVE-слово, операция остаётся unknown."
	// ReconciliationPausedTextTemplate is the control-fence word: the
	// exact paused word of the scoped controls rides the line (AC-10;
	// the recovery-scope permissions of a paused connector are the 3-9
	// slice — this slice is conservative: paused blocks).
	ReconciliationPausedTextTemplate = "Сверка заблокирована: %s"
	// ReconciliationRacingText is the single active chain word (the
	// epic fault-block: a parallel command is an honest racing-skip).
	ReconciliationRacingText = "Сверка уже идёт: активная lease держит право — максимум одна активная цепь сверки, параллельная команда не получает второго права."
	// ReconciliationBackoffTextTemplate is the backoff window word: the
	// exact next_eligible_at rides the line (AC-1 — the backoff and the
	// jitter are material, the /fixture/clock governs them in tests).
	ReconciliationBackoffTextTemplate = "Сверка не готова до %s — окно backoff активно, lease не берётся, запроса нет."
	// ReconciliationExhaustedText is the bounded attempts word (AC-9:
	// the Operation stays unknown until human evidence, never a success
	// by timer).
	ReconciliationExhaustedText = "Автоматические попытки сверки исчерпаны — требуется человек: операция остаётся unknown до human evidence (bounded, без слепого цикла)."
	// ReconciliationDecidedTextTemplate is the decided-verify word of
	// the at-least-once command repeat (the epic fault-block: the
	// already-decided operation is skipped honestly — no new query, no
	// new receipt version).
	ReconciliationDecidedTextTemplate = "Сверка уже решила: итог %s — нового запроса нет, новой версии квитанции нет."
	// ReconciliationEvidenceIncompleteText is the refused terminal claim
	// word (AC-2: the committed claim without the evidence triple spends
	// the attempt as no_answer — the terminal success is never taken on
	// word).
	ReconciliationEvidenceIncompleteText = "Evidence неполные — терминальная заявка сверки отвергнута: попытка записана как no_answer, операция остаётся unknown."
)
