package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// Module action — story 3.6 slice 1 (step-01): the dispatch/receipt domain
// core (FR-28/FR-31, AC-1..AC-13). This file owns the execution-contour
// dictionaries (the adapter outcome words, the Action Receipt outcome
// words, the confirmation dispatch fences WITHOUT exhausted, the retry
// policy words, the outbox event words), the two keyed-HMAC spaces of the
// dispatch contour (dispatch_key:v1: — the EXTERNAL identity of ONE
// operation, one key per operation, stable across attempts, the material
// never enters; dispatch_request:v1: — the redacted digest of the rebuilt
// provider request), the deterministic rebuild of the material payload
// from the immutable rows (the 3-5 promise «the worker rebuilds the
// payload from the immutable strings» — the BuildOutboundExternalPayload
// precedent of 3-3), the fail-closed guards of the adapter answer (the
// dictionary gate and the committed-evidence gate of the State/Authority
// discipline — the terminal success is never taken on word), the
// confirmation dispatch fence derivation, the bounded retry policy and
// the lease constants, with the honest texts of the operation life and
// the next steps. The words are frozen in code; runtime or tenant-config
// extension is NEVER allowed.
//
// The raw payload is NEVER stored anywhere (AD-10 + AC-5): only the keyed
// HMAC digest travels in the lease/outcome/receipt rows, and the short
// sensitive values never pass through an unsalted hash — the value probes
// of the unit matrix pin both properties, and the module-wide source
// probe pins the single legacy raw SHA-256 (the 3-4 proposal key) as the
// only one outside this contour.

// ---------------------------------------------------------------------------
// The dictionaries (compile-time, closed)
// ---------------------------------------------------------------------------

// Mutation outcome words — the adapter answer vocabulary (four, the form
// of the 3-3 dispatch dictionary: delivered → committed, the semantics of
// the business result of 3-6). The adapter claims one word; the guards
// below fail it closed on anything else (a foreign word reads as unknown
// without a guarantee — never a masked success).
const (
	MutationOutcomeAccepted  = "accepted"
	MutationOutcomeCommitted = "committed"
	MutationOutcomeFailed    = "failed"
	MutationOutcomeTimeout   = "timeout"
)

// MutationOutcomeWords is the full dictionary (4 words).
var MutationOutcomeWords = []string{
	MutationOutcomeAccepted, MutationOutcomeCommitted,
	MutationOutcomeFailed, MutationOutcomeTimeout,
}

// Action Receipt outcome words — the receipt speaks the DECIDING outcomes
// only (three): the receipt reflects the uncertainty, never a success at
// unknown (AC-9); accepted carries no receipt at all — the business
// result is not confirmed yet.
const (
	ReceiptOutcomeCommitted = "committed"
	ReceiptOutcomeFailed    = "failed"
	ReceiptOutcomeUnknown   = "unknown"
)

// ReceiptOutcomeWords is the full dictionary (3 words).
var ReceiptOutcomeWords = []string{ReceiptOutcomeCommitted, ReceiptOutcomeFailed, ReceiptOutcomeUnknown}

// Confirmation dispatch fence words — the queued-intent killers of the
// execute-time revalidation (three): a newer confirmation version, the
// revocation event, the passed valid_until. exhausted is DELIBERATELY
// ABSENT (the documented decision of this slice): the consumption after a
// successful authorization is the EXPECTED state of the confirmation —
// the 3-5 ladder governs NEW authorizations, not the dispatch of the
// already-created Operation (the unit probe pins the absence).
const (
	DispatchFenceSuperseded = "superseded"
	DispatchFenceRevoked    = "revoked"
	DispatchFenceExpired    = "expired"
)

// DispatchFenceWords is the full dictionary (3 words — no exhausted).
var DispatchFenceWords = []string{DispatchFenceSuperseded, DispatchFenceRevoked, DispatchFenceExpired}

// Dispatch retry policy words — the repeat decision vocabulary (four):
// retry (the safe repeat), park_unknown (the blind repeat forbidden),
// park_bound (the bounded limit), decided (the operation is already
// decided — a repeat is not a policy question at all).
const (
	DispatchRetryRetry       = "retry"
	DispatchRetryParkUnknown = "park_unknown"
	DispatchRetryParkBound   = "park_bound"
	DispatchRetryDecided     = "decided"
)

// DispatchRetryWords is the full dictionary (4 words).
var DispatchRetryWords = []string{
	DispatchRetryRetry, DispatchRetryParkUnknown, DispatchRetryParkBound, DispatchRetryDecided,
}

// Outbox event type words of the action module's own outbox (three): the
// queued job of 3-5, the receipt link of 3-6 and the recovery-case link
// of 3-8 (AD-2: the Work Item/Conversation reference travels as an event
// of THIS module's outbox — a direct write into a foreign module's tables
// never happens; the future consumer reads the pending records, the 3-5
// mirror precedent; the recovery_case.created records of 3-8 honestly
// accumulate pending — the claim-mode consumer of a future slice reads
// them, the receipt.created precedent).
const (
	OutboxEventOperationQueued     = "operation.queued"
	OutboxEventReceiptCreated      = "receipt.created"
	OutboxEventRecoveryCaseCreated = "recovery_case.created"
)

// OutboxEventWords is the full dictionary (3 words).
var OutboxEventWords = []string{OutboxEventOperationQueued, OutboxEventReceiptCreated, OutboxEventRecoveryCaseCreated}

// ---------------------------------------------------------------------------
// The lease constants and the lease window
// ---------------------------------------------------------------------------

// The material bounds of the dispatch contour (the 3-3 precedents: the
// bounded retry and the bounded drain batch — no blind loops anywhere).
const (
	// MaxDispatchAttempts is the bounded retry limit — one operation never
	// takes more than three dispatch attempts; the exhaustion parks with
	// the honest bound word instead of a blind loop (AC-5/AC-8).
	MaxDispatchAttempts = 3
	// DispatchLeaseGraceMS is the lease grace: the lease window is the
	// definition timeout PLUS the grace — the lease outlives the possible
	// in-flight adapter call (an expiry under a live call would be false);
	// an expired lease means the worker died, and the crash-recovery
	// policy governs the repeat.
	DispatchLeaseGraceMS = 30000
	// DispatchDrainLimit is the bounded drain batch: one drain command
	// considers at most this many pending jobs — the rest stay queued for
	// the next run, honestly (the cases drain-limiter precedent).
	DispatchDrainLimit = 100
)

// DeriveDispatchLeaseExpiresAt derives the lease expiry moment: now + the
// definition timeout + the grace (lease_expires_at). The timeout is
// re-checked against the definition bounds — the derivation never trusts
// the caller (fail-closed).
func DeriveDispatchLeaseExpiresAt(now time.Time, definitionTimeoutMS int) (time.Time, error) {
	if definitionTimeoutMS < TimeoutMSMin || definitionTimeoutMS > TimeoutMSMax {
		return time.Time{}, RuleError{Rule: "DeriveDispatchLeaseExpiresAt", Reason: fmt.Sprintf(
			"Таймаут определения %d мс вне границ [%d..%d] — окно lease не выводится (fail-closed).",
			definitionTimeoutMS, TimeoutMSMin, TimeoutMSMax)}
	}
	return now.Add(time.Duration(definitionTimeoutMS)*time.Millisecond +
		time.Duration(DispatchLeaseGraceMS)*time.Millisecond), nil
}

// ---------------------------------------------------------------------------
// The external idempotency key: the keyed identity of ONE OPERATION
// ---------------------------------------------------------------------------

// DeriveExternalIdempotencyKey derives the EXTERNAL idempotency key of one
// operation (AC-10): HMAC-SHA256(key, "dispatch_key:v1:" + tenant + FS +
// operation_id) in hex — ONE key per operation, deterministic and STABLE
// ACROSS ATTEMPTS (every repeat carries the same key — the provider
// deduplicates; a new Staffy intent is never created). The MATERIAL never
// enters (the key identifies the operation, not its material — the short
// sensitive values are protected structurally, AD-10), and the attempt
// number never enters either (otherwise the safe repeat would stop being
// safe). The prefix separates this key space from business_intent:v1: and
// operation_material:v1: (operation.go) and from action_preview:v1: /
// commitment:v1: / external_commitment:v1: (3-4/2-9/3-3) — the families
// never collide and none is derivable from another. The key is the
// per-tenant commitment key v1 (commitments_hmac_keys — the 2-9/3-3/3-4
// reuse, the INSERT-only ensure pattern).
func DeriveExternalIdempotencyKey(key []byte, tenant id.UUID, operationID id.UUID) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("dispatch_key:v1:"))
	mac.Write([]byte(strings.Join([]string{tenant.String(), operationID.String()}, canonicalFieldSep)))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// The authorized action payload: the deterministic rebuild
// ---------------------------------------------------------------------------

// AuthorizedActionPayload is the delivery material the dispatch worker
// hands to the mutation adapter: rebuilt DETERMINISTICALLY from the
// immutable rows only — the Operation (the pins and the target), the
// pinned preview version (the projected typed input and the snapshot rows
// — the material the human confirmed) and the resolved definition (the
// definition pin; the effects are re-derived and cross-checked against the
// pinned copy). A later draft change is a NEW preview version row — it
// never alters the rebuilt payload of the pinned version (the 3-3
// OutboundExternalCommitment precedent, the 3-5 promise fulfilled). The
// raw payload travels ONLY here: it is never stored — the lease, the
// outcome and the receipt carry the keyed digest alone (AD-10/AC-5).
type AuthorizedActionPayload struct {
	TenantID          id.UUID               `json:"tenant_id"`
	OperationID       id.UUID               `json:"operation_id"`
	DefinitionID      string                `json:"definition_id"`
	DefinitionVersion string                `json:"definition_version"`
	Target            ProposalTarget        `json:"target"`
	TypedInput        map[string]string     `json:"typed_input"`
	Snapshot          []MaterialSnapshotRow `json:"snapshot"`
	Effects           []PreviewEffect       `json:"effects"`
}

// BuildAuthorizedActionPayload rebuilds the material payload from the
// immutable strings: the Operation row, the PINNED preview version (read
// live) and the definition resolved by the 3-1 resolver. The pin alignment
// is verified fail-closed BEFORE any rebuild — mixed rows (another tenant,
// another preview version, another definition version) and a preview whose
// effects are not reproduced by the resolved definition are an integrity
// refusal, never a silent rebuild. A pure function otherwise: the same
// rows always yield the same payload byte-to-byte (the digest below).
func BuildAuthorizedActionPayload(op Operation, preview ActionPreview, def ActionDefinition) (AuthorizedActionPayload, error) {
	if op.TenantID != preview.TenantID {
		return AuthorizedActionPayload{}, RuleError{Rule: "BuildAuthorizedActionPayload",
			Reason: "Тенант Operation и пиннутой версии preview не совпадают — смешанные строки не пересобираются (integrity)."}
	}
	if op.PreviewID != preview.PreviewID || op.PreviewVersionNo != preview.VersionNo {
		return AuthorizedActionPayload{}, RuleError{Rule: "BuildAuthorizedActionPayload",
			Reason: "Operation пинит другую версию preview — смешанные строки не пересобираются (integrity)."}
	}
	if op.DefinitionID != preview.DefinitionID || op.DefinitionID != def.ID ||
		op.DefinitionVersion != preview.DefinitionVersion || op.DefinitionVersion != def.Version {
		return AuthorizedActionPayload{}, RuleError{Rule: "BuildAuthorizedActionPayload",
			Reason: "Пин определения расходится между Operation, preview и определением — смешанные строки не пересобираются (integrity)."}
	}
	derived := DerivePreviewEffects(def)
	if len(derived) != len(preview.Effects) {
		return AuthorizedActionPayload{}, RuleError{Rule: "BuildAuthorizedActionPayload",
			Reason: "Effects preview не воспроизводятся определением — целостность пересборки не подтверждена (integrity)."}
	}
	for i := range derived {
		if derived[i].Order != preview.Effects[i].Order ||
			derived[i].ActionRef != preview.Effects[i].ActionRef ||
			derived[i].FailureSemantics != preview.Effects[i].FailureSemantics {
			return AuthorizedActionPayload{}, RuleError{Rule: "BuildAuthorizedActionPayload",
				Reason: "Effects preview не воспроизводятся определением — целостность пересборки не подтверждена (integrity)."}
		}
	}
	return AuthorizedActionPayload{
		TenantID:          op.TenantID,
		OperationID:       op.OperationID,
		DefinitionID:      op.DefinitionID,
		DefinitionVersion: op.DefinitionVersion,
		Target:            op.Target,
		TypedInput:        preview.TypedInput,
		Snapshot:          preview.Snapshot,
		Effects:           preview.Effects,
	}, nil
}

// dispatchRequestCanonicalBytes renders the canonical bytes of the rebuilt
// payload: the pins and the target, then the sorted typed input, the
// sorted snapshot rows and the sorted effects (the preview.go canonical
// helpers — the map iteration and the slice order never enter; the digest
// is deterministic under any input shuffling).
func dispatchRequestCanonicalBytes(p AuthorizedActionPayload) []byte {
	fields := []string{
		p.TenantID.String(),
		p.OperationID.String(),
		p.DefinitionID,
		p.DefinitionVersion,
		p.Target.Kind,
		p.Target.Ref.String(),
		sortedTypedInput(p.TypedInput),
		sortedSnapshot(p.Snapshot),
		sortedEffects(p.Effects),
	}
	return []byte(strings.Join(fields, canonicalFieldSep))
}

// DeriveDispatchRequestDigest derives the redacted trace of the rebuilt
// provider request (AC-5): HMAC-SHA256(key, "dispatch_request:v1:" +
// canonical bytes) in hex. The RAW payload is never stored — the lease
// row, the outcome row, the receipt and every log carry this keyed digest
// alone (AD-10); the repeat verification (the at-least-once redelivery of
// AC-10) compares the freshly rebuilt digest against the recorded one —
// the same operation to the same addressee, never a stored payload. The
// prefix separates this space from dispatch_key:v1: (above) and from the
// 3-4/3-5/2-9/3-3 families. The key is the per-tenant commitment key v1
// (commitments_hmac_keys — the reused ensure pattern).
func DeriveDispatchRequestDigest(key []byte, payload AuthorizedActionPayload) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("dispatch_request:v1:"))
	mac.Write(dispatchRequestCanonicalBytes(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// The guards of the adapter answer (fail-closed, the State/Authority
// discipline)
// ---------------------------------------------------------------------------

// GuardMutationOutcome validates the outcome word of the adapter answer
// against the closed dictionary (the GuardDispatchOutcome precedent of
// 3-3): a foreign word is the honest refusal — the caller reads it as
// unknown WITHOUT a guarantee (a foreign word never dresses up as a
// success, AC-9), never a masked silence.
func GuardMutationOutcome(outcome string) error {
	if !WordIn(MutationOutcomeWords, outcome) {
		return RuleError{Rule: "GuardMutationOutcome", Reason: fmt.Sprintf(
			"Исход мутационного адаптера «%s» не входит в словарь (%s) — чужое слово читается как unknown без гарантии.",
			outcome, strings.Join(MutationOutcomeWords, ", "))}
	}
	return nil
}

// MutationEvidence is the STRUCTURAL evidence of a terminal success: the
// external object identity triple — the id, the version and the time
// (State/Authority: the declared TerminalEvidence word of the definition's
// DeliveryContract rides as a provenance word in the details — the domain
// never parses the free words; the structural gate is the identifiers).
// The declared word is the definition's promise; the triple is the proof.
type MutationEvidence struct {
	ExternalObjectID      string
	ExternalObjectVersion string
	ExternalObjectTime    time.Time
}

// GuardCommittedEvidence validates the terminal-success claim against the
// structural evidence (AC-6/AC-7): committed survives ONLY with the
// complete external object triple (id/version/time all non-empty); a
// committed claim without evidence is REFUSED to unknown — the terminal
// success is never taken on word (the receipt speaks the uncertainty with
// DispatchEvidenceIncompleteText, never a dressed-up success). The honest
// outcome space of the return is the adapter dictionary PLUS the honest
// «unknown» of the refused claim; every other word passes unchanged (the
// evidence is irrelevant for the non-committed claims).
func GuardCommittedEvidence(outcome string, evidence MutationEvidence) (string, error) {
	if !WordIn(MutationOutcomeWords, outcome) {
		return "", RuleError{Rule: "GuardCommittedEvidence", Reason: fmt.Sprintf(
			"Исход мутационного адаптера «%s» не входит в словарь (%s).",
			outcome, strings.Join(MutationOutcomeWords, ", "))}
	}
	if outcome != MutationOutcomeCommitted {
		return outcome, nil
	}
	if strings.TrimSpace(evidence.ExternalObjectID) == "" ||
		strings.TrimSpace(evidence.ExternalObjectVersion) == "" ||
		evidence.ExternalObjectTime.IsZero() {
		return OperationUnknown, nil
	}
	return outcome, nil
}

// DeriveMutationStateWord maps the honest outcome word onto the Operation
// ledger state word the worker appends: accepted → accepted (the provider
// confirmed the request reception — no business result yet), committed →
// committed, failed → failed, timeout → unknown (the possible-send
// ambiguity), unknown (the refused claims of the guards above) → unknown.
// Fail-closed on a foreign word.
func DeriveMutationStateWord(outcome string) (string, error) {
	switch outcome {
	case MutationOutcomeAccepted:
		return OperationAccepted, nil
	case MutationOutcomeCommitted:
		return OperationCommitted, nil
	case MutationOutcomeFailed:
		return OperationFailed, nil
	case MutationOutcomeTimeout, OperationUnknown:
		return OperationUnknown, nil
	}
	return "", RuleError{Rule: "DeriveMutationStateWord", Reason: fmt.Sprintf(
		"Исход «%s» не входит в пространство исходов ({%s, unknown}).",
		outcome, strings.Join(MutationOutcomeWords, ", "))}
}

// DeriveReceiptOutcomeWord maps the honest outcome word onto the Action
// Receipt outcome word: accepted carries NO receipt (the empty word —
// не-терминальное, the business result is not confirmed yet, the waiting
// of the 3-7 contour); committed/failed/unknown are the deciding outcomes
// (AC-9: the receipt reflects the uncertainty, never a success at
// unknown). Fail-closed on a foreign word.
func DeriveReceiptOutcomeWord(outcome string) (string, error) {
	switch outcome {
	case MutationOutcomeAccepted:
		return "", nil
	case MutationOutcomeCommitted:
		return ReceiptOutcomeCommitted, nil
	case MutationOutcomeFailed:
		return ReceiptOutcomeFailed, nil
	case MutationOutcomeTimeout, OperationUnknown:
		return ReceiptOutcomeUnknown, nil
	}
	return "", RuleError{Rule: "DeriveReceiptOutcomeWord", Reason: fmt.Sprintf(
		"Исход «%s» не входит в пространство исходов ({%s, unknown}).",
		outcome, strings.Join(MutationOutcomeWords, ", "))}
}

// ---------------------------------------------------------------------------
// The confirmation dispatch fence: the queued-intent killers
// ---------------------------------------------------------------------------

// DeriveConfirmationDispatchFence derives the dispatch fence word of the
// pinned confirmation version from its read-time state word (the output
// of the 3-5 DeriveConfirmationState): superseded (a newer version exists
// — the material was re-confirmed, the old intention is stale), revoked
// (the revocation event kills the queued intent BEFORE the send — FR-28)
// and expired (now passed the pinned valid_until — a new send requires a
// new confirmation) fence the dispatch; valid passes (the empty word — no
// fence). exhausted is DELIBERATELY NOT a fence (the documented decision
// of this slice): the consumption after a successful authorization is the
// EXPECTED state — the 3-5 ladder governs NEW authorizations, not the
// dispatch of the created Operation (a single_use operation with
// consumed=1 dispatches freely when the other fences are green). Fail
// closed on a foreign state word.
func DeriveConfirmationDispatchFence(confirmationState string) (string, error) {
	switch confirmationState {
	case ConfirmationSuperseded, ConfirmationRevoked, ConfirmationExpired:
		return confirmationState, nil
	case ConfirmationValid, ConfirmationExhausted:
		return "", nil
	}
	return "", RuleError{Rule: "DeriveConfirmationDispatchFence", Reason: fmt.Sprintf(
		"Состояние Confirmation «%s» не входит в словарь (%s).",
		confirmationState, strings.Join(ConfirmationStateWords, ", "))}
}

// ---------------------------------------------------------------------------
// The retry policy: the bounded repeat decision
// ---------------------------------------------------------------------------

// DeriveDispatchRetryPolicy derives the repeat policy word of one
// operation: a repeat is allowed ONLY by the provider_idempotent class
// AND the recorded guarantee AND the attempts within the bound
// (MaxDispatchAttempts=3 — the 3-3 precedent: bounded, no blind loop).
// The decided operations (committed — terminal, AC-8; accepted —
// не-терминальное waiting without a re-send) answer decided: a repeat is
// not a policy question at all. failed stays decided TOO — with ONE 3-7
// exception: a RECORDED reconciliation verdict no_effect (the proven
// absence of the external effect) unlocks a retry WITHOUT the
// provider-idempotent guarantee (AC-3/AC-6 of the epic 3.7 — the proof
// of the empty effect replaces the guarantee role in the blind-repeat
// prohibition; the retry rides the SAME operation, the SAME external key
// and request digest, NO new intent or reservation; the bound is still
// checked FIRST — the unlock works INSIDE the MaxDispatchAttempts
// budget, never above it). The guarantee is the
// RECORDED word of the attempt (the live declaration is read at the
// outcome-recording moment — the repeat decision reads the recorded fact,
// the 3-3 «belt over braces» precedent); for a crashed attempt (no
// outcome row) the caller passes the live-read guarantee. The bound is
// checked FIRST (the normative order — the unit matrix pins it): the
// exhaustion parks regardless of the guarantee. Fail-closed on the foreign
// state/class words and on the attempt numbers outside [1..MaxDispatchAttempts]
// — the first attempt is not a policy question (the ordered fences govern
// it), and an attempt above the bound is structurally impossible.
func DeriveDispatchRetryPolicy(operationState string, capabilityClass string, guarantee bool, attemptNo int, noEffectProven bool) (string, error) {
	if !WordIn(OperationStateWords, operationState) {
		return "", RuleError{Rule: "DeriveDispatchRetryPolicy", Reason: fmt.Sprintf(
			"Состояние Operation «%s» вне словаря (%s).", operationState, strings.Join(OperationStateWords, ", "))}
	}
	if !WordIn(CapabilityClasses, capabilityClass) {
		return "", RuleError{Rule: "DeriveDispatchRetryPolicy", Reason: fmt.Sprintf(
			"Класс capability «%s» вне словаря (%s).", capabilityClass, strings.Join(CapabilityClasses, ", "))}
	}
	if attemptNo < 1 || attemptNo > MaxDispatchAttempts {
		return "", RuleError{Rule: "DeriveDispatchRetryPolicy", Reason: fmt.Sprintf(
			"Номер попытки %d вне окна повтора [1..%d]: первая попытка — не вопрос политики повтора, попытка выше предела структурно невозможна.",
			attemptNo, MaxDispatchAttempts)}
	}
	if noEffectProven && operationState != OperationFailed {
		return "", RuleError{Rule: "DeriveDispatchRetryPolicy", Reason: fmt.Sprintf(
			"Доказанное no_effect (вердикт сверки 3-7) несовместимо с состоянием «%s»: вердикт записывается парой {ledger failed, receipt failed} одной транзакцией — несовпадение = порча данных, отказ (fail-closed).",
			operationState)}
	}
	switch operationState {
	case OperationAccepted, OperationCommitted:
		return DispatchRetryDecided, nil
	case OperationFailed:
		// Retry-unlock сверки 3-7 (AC-3/AC-6 эпика): записанный вердикт
		// no_effect — доказательство пустоты внешнего эффекта — заменяет
		// роль гарантии provider_idempotent в запрете слепого повтора
		// (повтор не может дублировать эффект, которого доказанно нет).
		// Повтор — НОВАЯ dispatch-попытка ТОЙ ЖЕ операции тем же внешним
		// ключом и request_digest; НОВОГО intent/резервации нет (UNIQUE
		// 3-5). Предел MaxDispatchAttempts продолжает действовать: unlock
		// работает внутри бюджета, не сверх. Без вердикта no_effect —
		// честный decided (новый запуск = новая Confirmation 3-5).
		if noEffectProven {
			if attemptNo >= MaxDispatchAttempts {
				return DispatchRetryParkBound, nil
			}
			return DispatchRetryRetry, nil
		}
		return DispatchRetryDecided, nil
	}
	if attemptNo >= MaxDispatchAttempts {
		return DispatchRetryParkBound, nil
	}
	if capabilityClass != CapabilityProviderIdempotent || !guarantee {
		return DispatchRetryParkUnknown, nil
	}
	return DispatchRetryRetry, nil
}

// ---------------------------------------------------------------------------
// The honest texts of the operation life and the next steps (AC-13)
// ---------------------------------------------------------------------------

// The fixed honest texts of the operation life states (AC-13: the states
// are distinguishable BY TEXT — the time, the Owning System and the next
// step are spoken by the surface from the live rows; the words here are
// the card's truth). The authorized/queued texts live in operation.go
// (the inherited 3-5 semantics — the queue is a FACT of the outbox).
const (
	// OperationSubmittedText speaks the write-ahead state: the request
	// left, the answer is not recorded yet (the resting state of a crash
	// between the send and the outcome — the expired lease shows the
	// recovery).
	OperationSubmittedText = "Запрос отправлен — ответ не записан (submitted): попытка отправки идёт либо оборвалась до записи исхода; истёкшая lease покажет восстановление."
	// OperationAcceptedText speaks the non-terminal provider reception —
	// the business result is NOT confirmed (the UI never announces
	// completeness).
	OperationAcceptedText = "Принято провайдером, бизнес-результат не подтверждён (accepted) — итога ещё нет; ожидание подтверждения контуром 3-7."
	// OperationCommittedText speaks the terminal success with the
	// structural evidence.
	OperationCommittedText = "Завершено: внешний объект создан (committed) — терминальный успех подтверждён структурным evidence (идентификаторы внешнего объекта)."
	// OperationFailedText speaks the terminal refusal without an effect —
	// the auto-repeat of the same intention is forbidden.
	OperationFailedText = "Не выполнено без эффекта (failed) — терминальный отказ; новый запуск требует новой Confirmation, авто-повтора нет."
	// OperationUnknownText speaks the honest uncertainty — never a
	// success and never a no-effect.
	OperationUnknownText = "Исход неизвестен (unknown) — неопределённость: повтор запрещён, сверка контуром 3-7."
)

// The fixed honest texts of the dispatch boundaries and the next steps —
// every boundary word names the exact cause and the allowed next step
// (FR-28: the operator sees at which stage and why the action stopped).
const (
	// DispatchBlindRepeatForbiddenText is the crash/unknown parking word
	// (AC-9/AC-11): a possible previous call forbids the blind re-dispatch.
	DispatchBlindRepeatForbiddenText = "Возможен предыдущий вызов — слепой повтор запрещён."
	// DispatchAttemptsBoundText is the bounded limit word (AC-5).
	DispatchAttemptsBoundText = "Повтор не предпринимается: предел попыток (3) — bounded, без слепого цикла."
	// DispatchRedeliveryDecidedText is the at-least-once duplicate skip
	// word (AC-10): the operation is already decided, no new intent.
	DispatchRedeliveryDecidedText = "Повтор задания: операция уже решена — новой отправки нет, новый intent не создаётся."
	// DispatchEvidenceIncompleteText is the State/Authority refusal word
	// (AC-6/AC-7): the terminal success is not confirmed.
	DispatchEvidenceIncompleteText = "Evidence неполные — терминальный успех не подтверждён."
	// DispatchNoEffectUnlockText is the retry-unlock word of the 3-7
	// reconciliation (AC-3/AC-6 of the epic): the PROVEN absence of the
	// external effect replaces the provider-idempotent guarantee role in
	// the blind-repeat prohibition — the retry rides the SAME operation,
	// the SAME external key and request digest, with NO new intent or
	// reservation (the UNIQUE of 3-5 stays intact).
	DispatchNoEffectUnlockText = "Эффект доказанно отсутствует — повтор разрешён решением политики: вердикт сверки no_effect заменяет гарантию идемпотентности, тот же внешний ключ и request digest, новой резервации нет."
)

// DispatchBlockedText renders the honest parking lead of the
// dispatch_blocked boundary: the lead plus the exact reason word of the
// fence/material derivation (the operation stays authorized — parked with
// a visible cause; the adapter was never called, the lease was never
// taken; never a false failed externally, AC-2).
func DispatchBlockedText(reason string) string {
	return "Отправка заблокирована: " + reason
}

// DispatchFenceTexts is the fixed honest text of every confirmation
// dispatch fence word (FR-28: the revocation, the replacement and the
// expiry kill the queued intent BEFORE the send).
var DispatchFenceTexts = map[string]string{
	DispatchFenceSuperseded: "Confirmation заменена: существует версия новее пиннутой — старое намерение устарело, требуется новая версия Preview и Confirmation.",
	DispatchFenceRevoked:    "Confirmation отозвана: queued intent погашен до отправки (FR-28).",
	DispatchFenceExpired:    "Срок Confirmation истёк: новая отправка требует новой Confirmation.",
}

// DispatchFenceText speaks the fence word; a foreign word fails closed.
func DispatchFenceText(word string) string {
	if text, ok := DispatchFenceTexts[word]; ok {
		return text
	}
	return fmt.Sprintf("Слово фенции dispatch «%s» не входит в словарь (%s).", word, strings.Join(DispatchFenceWords, ", "))
}

// DispatchRetryTexts is the fixed honest text of every retry policy word.
var DispatchRetryTexts = map[string]string{
	DispatchRetryRetry:       "Повтор разрешён: класс provider_idempotent, гарантия записана, попытки в пределах 3 — та же Operation несёт тот же внешний idempotency key.",
	DispatchRetryParkUnknown: "Возможен предыдущий вызов — слепой повтор запрещён: операция припаркована в unknown, сверка контуром 3-7.",
	DispatchRetryParkBound:   "Повтор не предпринимается: предел попыток (3) — bounded, без слепого цикла.",
	DispatchRetryDecided:     "Повтор задания: операция уже решена — новой отправки нет, нового intent нет (связь с существующим attempt).",
}

// DispatchRetryText speaks the retry policy word; a foreign word fails
// closed.
func DispatchRetryText(word string) string {
	if text, ok := DispatchRetryTexts[word]; ok {
		return text
	}
	return fmt.Sprintf("Слово политики повтора «%s» не входит в словарь (%s).", word, strings.Join(DispatchRetryWords, ", "))
}

// OperationNextStepTexts is the fixed honest NEXT STEP text of every
// operation life state (AC-13/FR-28: the card shows WHERE the action
// stands, WHY it stopped and WHICH step is allowed next — the word of the
// state alone never carries the step). authorized — the queue is the
// fact, the drain is the operator command; submitted — the write-ahead
// resting state heals by the verdict or the expired lease; accepted —
// the business result is awaited by the future reconciliation contour;
// committed — terminal, no step; failed — the new run needs a NEW
// Confirmation of 3-5; unknown — the blind repeat is forbidden, the
// reconciliation contour speaks next.
var OperationNextStepTexts = map[string]string{
	OperationAuthorized: "Следующий шаг: оператор запускает команду drain — очередь честно копится pending до явного запуска (демона нет).",
	OperationSubmitted:  "Следующий шаг: дождаться записи исхода попытки либо восстановления по истёкшей lease — повтор решает политика по записанной гарантии.",
	OperationAccepted:   "Следующий шаг: ожидание подтверждения бизнес-результата контуром сверки (3-7) — итога ещё нет.",
	OperationCommitted:  "Следующего шага нет: терминальный успех подтверждён структурным evidence.",
	OperationFailed:     "Следующий шаг: новый запуск требует новой Confirmation (контур 3-5) — авто-повтора нет.",
	OperationUnknown:    "Следующий шаг: сверка исхода командой reconcile (контур 3-7, запускает оператор — демона нет): система-владелец отвечает вердиктом, слепой повтор запрещён до доказанного no_effect.",
}

// OperationNextStepText speaks the next-step text of the state word; a
// foreign word fails closed (the honest refusal names the dictionary).
func OperationNextStepText(word string) string {
	if text, ok := OperationNextStepTexts[word]; ok {
		return text
	}
	return fmt.Sprintf("Слово состояния Operation «%s» не входит в словарь (%s).", word, strings.Join(OperationStateWords, ", "))
}

// DispatchAttemptRestingText speaks the honest word of an attempt WITHOUT
// a recorded outcome (the write-ahead resting state of a crash between
// the send and the verdict — the attempt view renders it by name, never a
// masked silence).
const DispatchAttemptRestingText = "Исход попытки не записан: write-ahead resting-состояние — ответ не попал в историю; истёкшая lease покажет восстановление."
