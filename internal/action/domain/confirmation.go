package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// Module action — story 3.5 slice 1 (step-01): the Confirmation domain core
// (FR-28/FR-29, AC-1..AC-3, AC-8, AC-10). This file owns the confirmation
// dictionaries (five read-time state words, six append-only event kinds,
// the idempotency-key charset of the authorize surface), the append-only
// Confirmation version shape with the LINKED MATERIAL COPIES (the preview
// digest, the material snapshot rows, the finite effect list — the consent
// carries WHAT was consented to), the server-truth valid_until derivation
// with the bounded clamp, the read-time consumption derivations (the count
// and the allowance), the state derivation in the FIXED rejection order,
// the fail-closed guards of the three commands and the honest texts of the
// rejection ladder. The words are frozen in code; runtime or tenant-config
// extension is NEVER allowed.
//
// The derivation discipline is the 2-12/3-3/3-4 precedent: a state is
// NEVER a stored column — DeriveConfirmationState reads the append-only
// event rows plus the server clock at EVERY read; the consumption is the
// count of the consumed events appended ONLY inside the authorization
// transaction.

// ---------------------------------------------------------------------------
// The confirmation state dictionary (5 words, derived — never stored)
// ---------------------------------------------------------------------------

// Confirmation state words: the read-time derivation vocabulary of one
// confirmation version, derived in the FIXED order (the order is
// normative — the unit matrix pins it): superseded (a newer version
// exists), revoked (the human revocation event), expired (the server
// clock passed valid_until), exhausted (the consumption count reached the
// allowance), valid (the last word — nothing matched). AC-1: the changed,
// expired, revoked or exhausted confirmation is rejected BY NAME at the
// authorization, never with a masked silence.
const (
	ConfirmationValid      = "valid"
	ConfirmationSuperseded = "superseded"
	ConfirmationRevoked    = "revoked"
	ConfirmationExpired    = "expired"
	ConfirmationExhausted  = "exhausted"
)

// ConfirmationStateWords is the full dictionary (5 words).
var ConfirmationStateWords = []string{
	ConfirmationValid, ConfirmationSuperseded, ConfirmationRevoked,
	ConfirmationExpired, ConfirmationExhausted,
}

// ---------------------------------------------------------------------------
// The confirmation event dictionary (6 words, append-only)
// ---------------------------------------------------------------------------

// Confirmation event kind words: the append-only history vocabulary.
// confirmed appends one version; superseded marks the replaced version
// (the stream stays 1:1 to the preview flow — a re-confirmation after a
// preview edit is a NEW version row, the old row NEVER mutates); revoked
// carries the human revocation (the step-up act of the app layer); consumed
// is appended ONLY inside the ReserveOperation transaction — one event per
// successful authorization, the read-time count is the audit;
// authorization_conflict and authorization_blocked are the visible audit
// words of a rejected authorization (the conflict exception is visible to
// the authorized owner — AC-8).
const (
	ConfirmationEventConfirmed             = "confirmed"
	ConfirmationEventSuperseded            = "superseded"
	ConfirmationEventRevoked               = "revoked"
	ConfirmationEventConsumed              = "consumed"
	ConfirmationEventAuthorizationConflict = "authorization_conflict"
	ConfirmationEventAuthorizationBlocked  = "authorization_blocked"
)

// ConfirmationEventKindWords is the full dictionary (6 words).
var ConfirmationEventKindWords = []string{
	ConfirmationEventConfirmed, ConfirmationEventSuperseded,
	ConfirmationEventRevoked, ConfirmationEventConsumed,
	ConfirmationEventAuthorizationConflict, ConfirmationEventAuthorizationBlocked,
}

// ---------------------------------------------------------------------------
// The idempotency-key charset dictionary of the authorize surface
// ---------------------------------------------------------------------------

// The idempotency-key bounds (FR-29): the key is the CLIENT's request
// identity word — the double-click, the webhook repeat and the replay job
// carry the SAME key again; the key is deterministically NOT derived from
// the material (otherwise the conflict of AC-8 would be impossible). The
// gate refuses a foreign charset or an out-of-bounds length with an honest
// reason BEFORE everything (AC-2).
const (
	IdempotencyKeyMinRunes = 8
	IdempotencyKeyMaxRunes = 128
)

// idempotencyKeyAllowed reports whether one byte belongs to the closed
// client charset: ASCII letters, digits, dot, underscore, hyphen.
func idempotencyKeyAllowed(b byte) bool {
	switch {
	case 'a' <= b && b <= 'z', 'A' <= b && b <= 'Z', '0' <= b && b <= '9':
		return true
	case b == '.', b == '_', b == '-':
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// The owner-kind mirror (the response-owner triple of 2-4)
// ---------------------------------------------------------------------------

// OwnerKindWords mirrors the response-owner kinds of the ownership
// dictionary of 2-4 (human | staffy). Mirror only: the action domain never
// imports the cases domain; the integration contract test pins this mirror
// against the living source. The owner triple (kind, ref, epoch) is the
// snapshot pin of the consent — the consent binds the owner the case had
// at the confirmation moment.
const (
	OwnerHuman  = "human"
	OwnerStaffy = "staffy"
)

// OwnerKindWords is the full mirror dictionary (2 words).
var OwnerKindWords = []string{OwnerHuman, OwnerStaffy}

// ---------------------------------------------------------------------------
// The Confirmation version: append-only shape with the linked copies
// ---------------------------------------------------------------------------

// Confirmation is one append-only version of the human consent artifact
// (AC-1): the stable confirmation identity, the monotonic version number
// (the stream is 1:1 to the preview version flow — one confirmation
// version pins EXACTLY ONE preview version; a re-confirmation after a
// preview edit is version+1 with the new preview pin, the old row NEVER
// mutates — the 2-9/3-4 append-only version pattern), the typed pins, the
// owner triple snapshot and the release snapshot (the 2-4 lineage
// precedent), and the LINKED MATERIAL COPIES: the preview digest (re-checked
// byte-exact against the live preview row at every authorization — the
// double digest protection), the material snapshot rows and the finite
// effect list. valid_until is the server truth (DeriveValidUntil); the
// state is NEVER stored here — it is derived at every read. The
// confirmation exists only for the required definitions (GuardConfirm);
// a not_required query definition honestly never enters this contour.
type Confirmation struct {
	TenantID          id.UUID               `json:"tenant_id"`
	ConfirmationID    id.UUID               `json:"confirmation_id"`
	VersionNo         int64                 `json:"version_no"`
	DefinitionID      string                `json:"definition_id"`
	DefinitionVersion string                `json:"definition_version"`
	CaseID            id.UUID               `json:"case_id"`
	WorkItemID        id.UUID               `json:"work_item_id"`
	Target            ProposalTarget        `json:"target"`
	OwnerKind         string                `json:"owner_kind"`
	OwnerRef          id.UUID               `json:"owner_ref"`
	OwnerEpoch        int64                 `json:"owner_epoch"`
	ReleaseDecisionNo int64                 `json:"release_decision_no"`
	ProfileVersion    int64                 `json:"profile_version"`
	PreviewID         id.UUID               `json:"preview_id"`
	PreviewVersionNo  int64                 `json:"preview_version_no"`
	PreviewDigest     string                `json:"preview_digest"`
	Snapshot          []MaterialSnapshotRow `json:"snapshot"`
	Effects           []PreviewEffect       `json:"effects"`
	ValidUntil        time.Time             `json:"valid_until"`
	ConfirmedBy       id.UUID               `json:"confirmed_by"`
	CreatedAt         time.Time             `json:"created_at"`
}

// ConfirmationEvent is one append-only row of the confirmation history:
// the six-word kind, the safe reason word (the dictionary reason of a
// blocked authorization — never a material value), the safe details
// (identifiers and words only — the store CHECK keeps the 500-rune bound)
// and the actor. The history is never rewritten; the consumed events are
// appended ONLY inside the ReserveOperation transaction (the losing race
// never appends — the count is the material audit).
type ConfirmationEvent struct {
	Kind    string    `json:"kind"`
	Reason  string    `json:"reason"`
	Details string    `json:"details"`
	At      time.Time `json:"at"`
	ActorID id.UUID   `json:"actor_id"`
}

// ---------------------------------------------------------------------------
// The valid_until derivation: server truth with the bounded clamp
// ---------------------------------------------------------------------------

// The valid_until window bounds: the requested lifetime clamps into
// [60, 3600] seconds — the server truth decides, never the client whim
// (the promised_at precedent of 2-16).
const (
	ConfirmationLifetimeMinSeconds = 60
	ConfirmationLifetimeMaxSeconds = 3600
)

// DeriveValidUntil derives the server-truth expiry of one confirmation
// version: the requested lifetime clamps into the bounded window, the
// server clock decides the moment. The stored valid_until is the truth of
// the expiry derivation; the clock is re-read at EVERY authorization —
// expired is a live word, never a stored state.
func DeriveValidUntil(now time.Time, requestedLifetimeSeconds int) time.Time {
	lifetime := requestedLifetimeSeconds
	if lifetime < ConfirmationLifetimeMinSeconds {
		lifetime = ConfirmationLifetimeMinSeconds
	}
	if lifetime > ConfirmationLifetimeMaxSeconds {
		lifetime = ConfirmationLifetimeMaxSeconds
	}
	return now.Add(time.Duration(lifetime) * time.Second)
}

// ---------------------------------------------------------------------------
// The consumption derivations: the count and the allowance (read-time)
// ---------------------------------------------------------------------------

// DeriveConsumedCount derives the consumption count of one confirmation
// version at read time: exactly the count of the consumed events. The
// count is NEVER a stored column; a consumed event is appended ONLY
// inside the authorization transaction, so the count is the material
// audit of the successful authorizations.
func DeriveConsumedCount(events []ConfirmationEvent) int {
	count := 0
	for _, e := range events {
		if e.Kind == ConfirmationEventConsumed {
			count++
		}
	}
	return count
}

// ConsumptionAllowance is the derivation of the consumption rule word:
// Bounded=false is the unlimited rule (exhausted is impossible by
// construction); Limit=1 is the single_use rule. The bounded rule carries
// NO numeric bound carrier in the 3-1 definition shape — its allowance is
// underivable and the guard fails closed (the word stays reserved for a
// future slice).
type ConsumptionAllowance struct {
	Bounded bool
	Limit   int
}

// DeriveConsumptionAllowance derives the allowance from the definition's
// consumption rule word: single_use → exactly 1; unlimited → no bound
// (exhausted impossible); bounded (and any foreign word) → RuleError
// fail-closed — the numeric bound carrier does not exist in the 3-1
// definitions, the honest word is spoken, never a silent guess (AC-1: the
// honest boundary).
func DeriveConsumptionAllowance(rule string) (ConsumptionAllowance, error) {
	switch rule {
	case ConsumptionSingleUse:
		return ConsumptionAllowance{Bounded: true, Limit: 1}, nil
	case ConsumptionUnlimited:
		return ConsumptionAllowance{Bounded: false}, nil
	default:
		return ConsumptionAllowance{}, RuleError{
			Rule:   "DeriveConsumptionAllowance",
			Reason: ConfirmationBoundedAllowanceText,
		}
	}
}

// ---------------------------------------------------------------------------
// The state derivation: the FIXED rejection order at every read
// ---------------------------------------------------------------------------

// DeriveConfirmationState derives the read-time state word of one
// confirmation version — the FIXED order is normative (the unit matrix
// pins it): superseded beats revoked beats expired beats exhausted, and
// valid is the last word. The inputs are read at EVERY authorization: the
// append-only events, the server clock against valid_until and the
// allowance derivation of the pinned consumption rule. The rejected
// confirmation is NEVER consumed (the app ladder consumes nothing on a
// rejection — the ladder speaks the word and stops); the revoked past
// consumption stays the audit fact — the revocation kills the FUTURE
// authorizations only. Fail-closed: an underivable allowance (bounded)
// refuses the derivation itself.
func DeriveConfirmationState(rule string, validUntil time.Time, events []ConfirmationEvent, now time.Time) (string, error) {
	for _, e := range events {
		if e.Kind == ConfirmationEventSuperseded {
			return ConfirmationSuperseded, nil
		}
	}
	for _, e := range events {
		if e.Kind == ConfirmationEventRevoked {
			return ConfirmationRevoked, nil
		}
	}
	if now.After(validUntil) {
		return ConfirmationExpired, nil
	}
	allowance, err := DeriveConsumptionAllowance(rule)
	if err != nil {
		return "", err
	}
	if allowance.Bounded && DeriveConsumedCount(events) >= allowance.Limit {
		return ConfirmationExhausted, nil
	}
	return ConfirmationValid, nil
}

// ---------------------------------------------------------------------------
// The fail-closed guards of the three commands (the dictionary gates)
// ---------------------------------------------------------------------------

// GuardConfirm is the fail-closed definition gate of the confirm command:
// the definition must REQUIRE the confirmation (a not_required query
// definition honestly never enters this contour — its answers live in the
// observations of 2-7), and the consumption rule must yield an allowance
// (bounded fails closed — a confirmation that could never be consumed is
// not created). The membership, the rights, the step-up and the
// freshness/readiness of the pinned preview version are the app gates IN
// ORDER — this guard owns the definition words only.
func GuardConfirm(def ActionDefinition) error {
	if def.Conditions.Confirmation != ConfirmationRequired {
		return RuleError{Rule: "GuardConfirm", Reason: ConfirmationNotRequiredText}
	}
	if _, err := DeriveConsumptionAllowance(def.Conditions.ConsumptionRule); err != nil {
		return RuleError{Rule: "GuardConfirm", Reason: ConfirmationBoundedAllowanceText}
	}
	return nil
}

// GuardRevoke is the fail-closed definition gate of the revocation
// command: only a required confirmation can be revoked (a not_required
// definition has no confirmation to revoke — the same honest boundary
// word). The revocation is the human act with the step-up of the app
// layer (the Approve precedent of 3-3); the event kind itself is guarded
// by GuardConfirmationEventKind. The revocation kills the FUTURE
// authorizations only — the past consumption stays the audit fact.
func GuardRevoke(def ActionDefinition) error {
	if def.Conditions.Confirmation != ConfirmationRequired {
		return RuleError{Rule: "GuardRevoke", Reason: ConfirmationNotRequiredText}
	}
	return nil
}

// GuardAuthorize is the fail-closed idempotency-key gate of the authorize
// surface: the key is the CLIENT's request identity word (FR-29 — the
// double-click, the webhook repeat and the replay job carry the SAME key),
// deterministically NOT derived from the material. The closed charset is
// ASCII letters, digits, dot, underscore, hyphen; the length bounds are
// 8..128 runes. A foreign byte or an out-of-bounds length refuses with the
// honest reason BEFORE everything (AC-2) — the gate runs before the
// confirmation pin resolution, before the replay check, before the
// ladder; nothing is read, nothing is consumed.
func GuardAuthorize(idempotencyKey string) error {
	n := utf8.RuneCountInString(idempotencyKey)
	if n < IdempotencyKeyMinRunes || n > IdempotencyKeyMaxRunes {
		return RuleError{Rule: "GuardAuthorize", Reason: fmt.Sprintf(
			"Idempotency key обязан быть от %d до %d рун — машинное слово повтора (double-click, webhook и replay несут тот же ключ).",
			IdempotencyKeyMinRunes, IdempotencyKeyMaxRunes)}
	}
	for i := 0; i < len(idempotencyKey); i++ {
		if !idempotencyKeyAllowed(idempotencyKey[i]) {
			return RuleError{Rule: "GuardAuthorize", Reason: fmt.Sprintf(
				"Символ idempotency key вне закрытого charset [A-Za-z0-9._-] (байт %d) — машинное слово повтора ограничено словарём.", i)}
		}
	}
	return nil
}

// GuardConfirmationEventKind refuses a foreign event kind word — the
// append-only history vocabulary is closed (6 words); the store CHECK is
// the loyal backstop (the GuardDryRunResult precedent of 3-4).
func GuardConfirmationEventKind(kind string) error {
	if WordIn(ConfirmationEventKindWords, kind) {
		return nil
	}
	return RuleError{Rule: "GuardConfirmationEventKind", Reason: fmt.Sprintf(
		"Событие confirmation «%s» вне словаря (%s).", kind, strings.Join(ConfirmationEventKindWords, ", "))}
}

// ---------------------------------------------------------------------------
// The honest texts: the rejection ladder and the fixed boundaries
// ---------------------------------------------------------------------------

// ConfirmationStateTexts is the fixed honest text of every state word —
// the rejection ladder speaks the exact word by name (AC-1/AC-2: every
// rejection is the exact reason word, never a masked silence).
var ConfirmationStateTexts = map[string]string{
	ConfirmationValid:      "Confirmation действительна: согласие живо, окно valid_until активно, потребление не исчерпано.",
	ConfirmationSuperseded: "Confirmation заменена новой версией (после редактирования preview) — эта версия к авторизации не пригодна.",
	ConfirmationRevoked:    "Confirmation отозвана человеком — будущие авторизации невозможны; прошлое потребление остаётся фактом аудита.",
	ConfirmationExpired:    "Срок Confirmation истёк (valid_until прошла серверными часами) — требуется новая версия Preview и Confirmation.",
	ConfirmationExhausted:  "Confirmation исчерпана — разрешённые потребления израсходованы; требуется новое подтверждение.",
}

// ConfirmationStateText speaks the state word honestly; a foreign word
// fails closed with the dictionary itself.
func ConfirmationStateText(word string) string {
	if text, ok := ConfirmationStateTexts[word]; ok {
		return text
	}
	return fmt.Sprintf("Слово состояния Confirmation «%s» не входит в словарь (%s).",
		word, strings.Join(ConfirmationStateWords, ", "))
}

// The fixed honest texts of the contour boundaries — the exact phrases are
// the slice's spoken truth; the card, the API refusal and the page share
// them (an app-private copy would drift).
const (
	// ConfirmationNotRequiredText is the honest refusal of a not_required
	// definition at the confirm gate (AC-1: the honest boundary in words).
	ConfirmationNotRequiredText = "Запросы идут через наблюдения 2-7 — контур подтверждения существует для мутирующих действий."
	// ConfirmationBoundedAllowanceText is the fail-closed refusal of the
	// bounded consumption rule: the numeric bound carrier is absent in the
	// 3-1 definitions, the word is reserved for a future slice.
	ConfirmationBoundedAllowanceText = "Числовая граница потребления не объявлена определением — слово bounded зарезервировано будущим срезом."
	// ConfirmationPreviewChangedText is the honest refusal of the digest
	// mismatch at the authorization: the live preview row changed against
	// the linked copy (the double digest protection, AC-1).
	ConfirmationPreviewChangedText = "Живая строка preview изменилась против копии Confirmation — подтверждение изменившегося материала невозможно, требуется новая версия Preview и Confirmation."
	// MaterialConditionChangedText is the honest refusal of the last
	// read-only material check (AC-3/AG-4): the owning-system fact moved —
	// the revalidation asks for the new version pair.
	MaterialConditionChangedText = "Материальное условие изменилось — требуется новая версия Preview и Confirmation."
	// FreshReadsUnavailableText is the fail-closed refusal of the nil
	// OwningSystemReader (production, AG-4): the authorization honestly
	// cannot happen without the fresh reads.
	FreshReadsUnavailableText = "Свежие данные владеющей системы недоступны — авторизация fail-closed (ридер не подключён)."
	// IdempotencyConflictText is the visible exception of the same
	// idempotency key with a different material (AC-8): the conflict is
	// honest, the original operation stays unchanged.
	IdempotencyConflictText = "Тот же idempotency key поступил с другим материалом — конфликт; исходная Operation неизменна, исключение видно уполномоченному владельцу."
)
