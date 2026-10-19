package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The external commitment envelope (Story 3.3, first slice): the
// dictionaries of the epic, the append-only versioned envelope, the 2.9
// exact-version pin, the content binding token, the read-time readiness,
// the seven delivery states derived from the append-only dispatch events,
// the idempotency key of the version pin and the deterministic outbound
// payload
// ---------------------------------------------------------------------------
//
// The external commitment NEVER mutates: a changed text is a NEW version row
// (the append-only precedent of 00009/2.9), the old versions never change and
// the queued payload therefore never drifts — the determinism comes from the
// immutability, not from a snapshot. The persist-before-emit core lives in
// the store transaction (step-02): the enqueue writes the outbox record in
// ONE PostgreSQL transaction whose signature carries no adapter — the
// network call is physically impossible before the commit, and a database
// failure leaves the external system untouched (the probe of the matrix).
//
// The delivery state is a READ-TIME derivation over the append-only
// dispatch_events (the 2.12 precedent — never a stored status): without the
// outbox record the truth is persisted; the outbox record without any event
// is queued; the LAST event of the monotonic history decides otherwise. The
// word cancelled_stale returns the record to persisted — the stale fence
// killed the queue BEFORE the words left (AC-6: «слова не уходят»), the
// reason stays in the append-only history as the material of the page.
//
// The material never leaves the row (AC-10): the outbox record carries the
// identifiers, the dictionary words, the idempotency key and the content
// digest only; the full content travels exclusively inside the outbound
// payload the drain builds from the immutable version at dispatch time.

// ---------------------------------------------------------------------------
// The dictionaries (the words of the epic)
// ---------------------------------------------------------------------------

// The purpose words of the external commitment (the epic vocabulary): a
// promise, an offer (the pinned 2.9 exact version), an agreed callback and
// a deadline update. No other purpose exists — the dictionaries are fixed
// by the epic and the CHECK of the migration backstops them.
const (
	ExternalPurposePromise        = "promise"
	ExternalPurposeOffer          = "offer"
	ExternalPurposeCallback       = "callback"
	ExternalPurposeDeadlineUpdate = "deadline_update"
)

// ExternalPurposeWords is the full dictionary (the canonical order).
var ExternalPurposeWords = []string{
	ExternalPurposePromise, ExternalPurposeOffer,
	ExternalPurposeCallback, ExternalPurposeDeadlineUpdate,
}

// The recipient kinds of the external commitment: the client of the case or
// the operator (the human on the other side of the wire). The executor
// recipient honestly NEVER exists in this slice — the executor registries
// belong to Epic 5 and no adapter would deliver to them.
const (
	ExternalRecipientClient   = "client"
	ExternalRecipientOperator = "operator"
)

// ExternalRecipientKindWords is the full dictionary.
var ExternalRecipientKindWords = []string{ExternalRecipientClient, ExternalRecipientOperator}

// The acknowledgement requirement words: the envelope either demands the
// acknowledgement of the recipient or honestly does not. The required-ack
// envelope renders delivered WITHOUT acknowledged as «доставлено, принятия
// НЕТ» — the acceptance is a distinct word, never implied (AC-7/AC-11).
const (
	ExternalAckRequired    = "required"
	ExternalAckNotRequired = "not_required"
)

// ExternalAckRequirementWords is the full dictionary.
var ExternalAckRequirementWords = []string{ExternalAckRequired, ExternalAckNotRequired}

// The seven delivery states of AC-7 — the words the client and operator
// surfaces share verbatim. submitted is NEVER a delivery, delivered is
// NEVER an acceptance: the words differ materially and so do the fixed
// render texts. unknown shows the reconciliation path — the blind retry is
// structurally impossible.
const (
	ExternalDeliveryPersisted    = "persisted"
	ExternalDeliveryQueued       = "queued"
	ExternalDeliverySubmitted    = "submitted"
	ExternalDeliveryDelivered    = "delivered"
	ExternalDeliveryAcknowledged = "acknowledged"
	ExternalDeliveryFailed       = "failed"
	ExternalDeliveryUnknown      = "unknown"
)

// ExternalDeliveryStateWords is the full dictionary (seven words, AC-7).
var ExternalDeliveryStateWords = []string{
	ExternalDeliveryPersisted, ExternalDeliveryQueued, ExternalDeliverySubmitted,
	ExternalDeliveryDelivered, ExternalDeliveryAcknowledged,
	ExternalDeliveryFailed, ExternalDeliveryUnknown,
}

// The dispatch event kinds — the append-only vocabulary of the
// dispatch_events table (seven kinds). The queued word is NOT an event kind:
// the pending outbox record itself is the queued evidence, exactly the
// dispatched-outcome precedent of 2.12. attempt_submitted carries the
// external correlation id (AC-8), attempt_unknown records the timeout — the
// possible-send ambiguity; the receipt kinds are the operator relay
// observations (2.12 ConfirmerOperatorRelay — the human passes the machine
// observation, the machine never invents it).
const (
	ExternalDispatchAttemptSubmitted    = "attempt_submitted"
	ExternalDispatchAttemptFailed       = "attempt_failed"
	ExternalDispatchAttemptUnknown      = "attempt_unknown"
	ExternalDispatchReceiptDelivered    = "receipt_delivered"
	ExternalDispatchReceiptAcknowledged = "receipt_acknowledged"
	ExternalDispatchReceiptFailed       = "receipt_failed"
	ExternalDispatchCancelledStale      = "cancelled_stale"
)

// ExternalDispatchKindWords is the full dictionary (seven kinds).
var ExternalDispatchKindWords = []string{
	ExternalDispatchAttemptSubmitted, ExternalDispatchAttemptFailed,
	ExternalDispatchAttemptUnknown, ExternalDispatchReceiptDelivered,
	ExternalDispatchReceiptAcknowledged, ExternalDispatchReceiptFailed,
	ExternalDispatchCancelledStale,
}

// The operator relay observation words (the input vocabulary of the
// dispatch-receipts surface): the operator relays what the channel world
// honestly observed — delivered, acknowledged or failed.
const (
	RelayObservationDelivered    = "delivered"
	RelayObservationAcknowledged = "acknowledged"
	RelayObservationFailed       = "failed"
)

// RelayObservationWords is the full dictionary.
var RelayObservationWords = []string{
	RelayObservationDelivered, RelayObservationAcknowledged, RelayObservationFailed,
}

// The stale fence reasons of the drain (the cause vocabulary of
// cancelled_stale): a newer version exists, the validity expired, the owner
// epoch moved under the queue, the scoped control of the route refused.
// The control reason is the live 3.2 enforcement point — the drain consumes
// EffectiveFor through the composition adapter (the DAG never lets the case
// import the control module).
const (
	ExternalStaleNewerVersion = "newer_version"
	ExternalStaleExpired      = "expired"
	ExternalStaleOwnerEpoch   = "owner_epoch_moved"
	ExternalStaleControlMode  = "control_mode"
)

// ExternalStaleReasonWords is the full dictionary.
var ExternalStaleReasonWords = []string{
	ExternalStaleNewerVersion, ExternalStaleExpired,
	ExternalStaleOwnerEpoch, ExternalStaleControlMode,
}

// The dispatch outcome words of the adapter contract (four): the channel
// accepted the request, the channel reported the delivery, the provider
// refused BEFORE the send (no external effect — the retry is safe within
// the bound) and the timeout (a possible send — the honest unknown).
const (
	DispatchOutcomeAccepted  = "accepted"
	DispatchOutcomeDelivered = "delivered"
	DispatchOutcomeFailed    = "failed"
	DispatchOutcomeTimeout   = "timeout"
)

// DispatchOutcomeWords is the full dictionary.
var DispatchOutcomeWords = []string{
	DispatchOutcomeAccepted, DispatchOutcomeDelivered,
	DispatchOutcomeFailed, DispatchOutcomeTimeout,
}

// The readiness words of the read-time derivation (never stored): the
// envelope is ready for the enqueue, or honestly refused — a newer version
// superseded it, its validity expired, the owner epoch moved or the 2.9 pin
// of the offer is not confirmable.
const (
	ExternalReadinessReady      = "ready"
	ExternalReadinessSuperseded = "superseded"
	ExternalReadinessExpired    = "expired"
	ExternalReadinessOwnerStale = "owner_stale"
	ExternalReadinessPinBlocked = "pin_blocked"
)

// ExternalReadinessWords is the full dictionary.
var ExternalReadinessWords = []string{
	ExternalReadinessReady, ExternalReadinessSuperseded,
	ExternalReadinessExpired, ExternalReadinessOwnerStale,
	ExternalReadinessPinBlocked,
}

// The material limits of the envelope.
const (
	// MaxExternalCommitmentContentRunes is the bound of the content material
	// (the reply-content precedent of 2.4).
	MaxExternalCommitmentContentRunes = 2000
	// MaxExpectedNextStepRunes is the bound of the expected next step.
	MaxExpectedNextStepRunes = 200
	// MaxDispatchAttempts is the bounded retry limit — the attempts of one
	// version never exceed the bound; the exhaustion parks the record with
	// the honest text instead of a blind loop (AC-5/AC-8).
	MaxDispatchAttempts = 3
	// MaxExternalDispatchDetailsRunes is the safe-details bound of the
	// dispatch events (correlation, outcome, cause — never the content).
	MaxExternalDispatchDetailsRunes = 500
)

// ---------------------------------------------------------------------------
// The fixed render texts (the render layer material — they never enter the
// database, the outbox or the events)
// ---------------------------------------------------------------------------

// ExternalPurposeTexts is the fixed render text of every purpose word.
var ExternalPurposeTexts = map[string]string{
	ExternalPurposePromise:        "Обещание",
	ExternalPurposeOffer:          "Предложение (пин точной версии обязательства)",
	ExternalPurposeCallback:       "Согласованный обратный звонок (callback)",
	ExternalPurposeDeadlineUpdate: "Обновление срока",
}

// ExternalRecipientKindTexts is the fixed render text of every recipient word.
var ExternalRecipientKindTexts = map[string]string{
	ExternalRecipientClient:   "Клиент",
	ExternalRecipientOperator: "Оператор",
}

// ExternalAckRequirementTexts is the fixed render text of the ack words.
var ExternalAckRequirementTexts = map[string]string{
	ExternalAckRequired:    "Требуется подтверждение получения (acknowledgement).",
	ExternalAckNotRequired: "Подтверждение получения не требуется.",
}

// ExternalDeliveryStateTexts is the fixed honest text of every delivery
// state word: submitted is materially distinct from delivered and
// acknowledged (AC-7/AC-11 — «нельзя спутать»), the unknown text names the
// reconciliation path instead of promising a retry.
var ExternalDeliveryStateTexts = map[string]string{
	ExternalDeliveryPersisted:    "Сохранено; отправки не было — в очереди не состоит.",
	ExternalDeliveryQueued:       "В очереди на отправку; слова ещё не покинули модуль.",
	ExternalDeliverySubmitted:    "Отправлено в канал; доставки и принятия НЕТ.",
	ExternalDeliveryDelivered:    "Доставка подтверждена квитанцией канала; принятия получателем НЕТ.",
	ExternalDeliveryAcknowledged: "Принято получателем — квитанция подтверждает принятие.",
	ExternalDeliveryFailed:       "Отправка не состоялась: отказ провайдера (до отправки либо наблюдение отказа).",
	ExternalDeliveryUnknown:      "Итог неопределён — возможная отправка; без слепого повтора, передаётся в reconciliation (3-7).",
}

// ExternalDeliveryStateText resolves the render text of one state word (the
// empty string of a foreign word — the render layer only).
func ExternalDeliveryStateText(state string) string {
	return ExternalDeliveryStateTexts[state]
}

// ExternalStaleReasonTexts is the fixed render text of every stale reason —
// the honest «слова не ушли» material of the cancelled queue.
var ExternalStaleReasonTexts = map[string]string{
	ExternalStaleNewerVersion: "Существует новее версия — слова не ушли.",
	ExternalStaleExpired:      "Срок действия истёк — слова не ушли.",
	ExternalStaleOwnerEpoch:   "Владелец сменился — слова не ушли.",
	ExternalStaleControlMode:  "Контроль маршрута запретил отправку — слова не ушли.",
}

// ExternalStaleReasonText resolves the render text of one stale reason.
func ExternalStaleReasonText(reason string) string {
	return ExternalStaleReasonTexts[reason]
}

// ExternalReadinessRefusalTexts is the fixed refusal text of every non-ready
// readiness word — the enqueue refuses honestly BEFORE the transaction
// (the fake-repo counter of the unit matrix proves the order).
var ExternalReadinessRefusalTexts = map[string]string{
	ExternalReadinessSuperseded: "Существует новее версия конверта — отправка этой версии невозможна.",
	ExternalReadinessExpired:    "Срок действия версии истёк — отправка невозможна.",
	ExternalReadinessOwnerStale: "Owner epoch сместился — требуется утверждение текущим владельцем-человеком.",
	ExternalReadinessPinBlocked: "Пин обязательства не является offered_confirmable — предложение не может быть отправлено этой версией.",
}

// ---------------------------------------------------------------------------
// The 2.9 exact-version pin (the nullable triple)
// ---------------------------------------------------------------------------

// ExternalCommitmentPin is the exact-version pin of the 2.9 commitment the
// offer refers to: the stable commitment id, the exact version number and
// the content binding token of THAT version. The pin is MANDATORY for the
// offer purpose (the discipline of 2.9 — the offer leaves only pinned to
// the exact shown version) and optional for the other purposes; the pin is
// an all-or-nothing triple — a partial pin is a refusal, never a silent
// default. The live verification of the pin against the 2.9 row (the token
// re-computation and the readiness of the pinned version) belongs to the
// app gate; the shape guard lives here.
type ExternalCommitmentPin struct {
	CommitmentID id.UUID
	VersionNo    int64
	ContentToken string
}

// Present reports whether the pin triple is carried at all.
func (p ExternalCommitmentPin) Present() bool {
	return p.CommitmentID != "" || p.VersionNo != 0 || p.ContentToken != ""
}

// isHex64 reports whether the token is exactly 64 lowercase hex characters
// (the shape of every HMAC-SHA256 hex print of the commitment tokens).
func isHex64(value string) bool {
	if len(value) != 64 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// GuardExternalCommitmentPin validates the shape of the pin triple: a
// valid commitment id, a positive exact version and the 64-hex content
// token. The LIVE match of the token against the pinned 2.9 row is the app
// gate — the shape alone never proves the pin.
func GuardExternalCommitmentPin(pin ExternalCommitmentPin) error {
	if !pin.CommitmentID.Valid() {
		return fmt.Errorf("пин обязательства требует действительный commitment_id")
	}
	if pin.VersionNo < 1 {
		return fmt.Errorf("пин обязательства требует точный номер версии (>= 1)")
	}
	if !isHex64(pin.ContentToken) {
		return fmt.Errorf("пин обязательства требует content token версии (64 hex)")
	}
	return nil
}

// ---------------------------------------------------------------------------
// The envelope version (the append-only row)
// ---------------------------------------------------------------------------

// ExternalCommitment is one append-only version row of the external
// commitment envelope: the stable external id with the monotonic version
// number (a new text is a NEW row — the old rows never mutate), the
// dictionary words of the epic (purpose, recipient kind, the channel of
// the delivery route), the content material, the validity, the expected
// next step and the acknowledgement requirement. The row carries the
// relation pins: the work item, the response-owner snapshot with the epoch
// (the 2.4 ClientReply precedent), the decision lineage of the policy
// context (the decision number and the profile version of 2.6), the policy
// outcome number (0 — honestly none), the optional 2.9 exact-version pin
// and the content binding token of THIS version.
type ExternalCommitment struct {
	TenantID         id.UUID
	CaseID           id.UUID
	WorkItemID       id.UUID
	ExtCommitmentID  id.UUID
	VersionNo        int64
	Purpose          string
	RecipientKind    string
	Channel          string
	ContentText      string
	ValidUntil       time.Time
	ExpectedNextStep string
	AckRequirement   string
	Pin              ExternalCommitmentPin
	OwnerKind        string
	OwnerRef         id.UUID
	OwnerEpoch       int64
	DecisionNo       int64
	ProfileVersion   int64
	PolicyOutcomeNo  int64
	ContentToken     string
	ActorID          id.UUID
	CreatedAt        time.Time
}

// CurrentExternalCommitmentVersion resolves the CURRENT version of one
// envelope: the maximal version number (nil when no version exists).
func CurrentExternalCommitmentVersion(versions []ExternalCommitment) *ExternalCommitment {
	var current *ExternalCommitment
	for i := range versions {
		if current == nil || versions[i].VersionNo > current.VersionNo {
			current = &versions[i]
		}
	}
	return current
}

// ---------------------------------------------------------------------------
// The canonical bytes and the content binding token
// ---------------------------------------------------------------------------

// canonicalFieldSep separates the canonical fields (the ASCII unit
// separator — it never occurs inside the material words).
const canonicalFieldSep = "\x1f"

// ExternalCommitmentCanonicalBytes builds the canonical bytes of the
// envelope version — the deterministic byte material of the content token.
// Every material field participates in a fixed order; the provenance
// columns (the actor and the creation time) honestly stay out: the token
// binds the TERMS of the version, and the version number inside the token
// input already separates the rows materially.
func ExternalCommitmentCanonicalBytes(commitment ExternalCommitment) []byte {
	fields := []string{
		commitment.TenantID.String(),
		commitment.CaseID.String(),
		commitment.WorkItemID.String(),
		commitment.ExtCommitmentID.String(),
		fmt.Sprintf("%d", commitment.VersionNo),
		commitment.Purpose,
		commitment.RecipientKind,
		commitment.Channel,
		commitment.ContentText,
		commitment.ValidUntil.UTC().Format(time.RFC3339Nano),
		commitment.ExpectedNextStep,
		commitment.AckRequirement,
		commitment.Pin.CommitmentID.String(),
		fmt.Sprintf("%d", commitment.Pin.VersionNo),
		commitment.Pin.ContentToken,
		commitment.OwnerKind,
		commitment.OwnerRef.String(),
		fmt.Sprintf("%d", commitment.OwnerEpoch),
		fmt.Sprintf("%d", commitment.DecisionNo),
		fmt.Sprintf("%d", commitment.ProfileVersion),
		fmt.Sprintf("%d", commitment.PolicyOutcomeNo),
	}
	return []byte(strings.Join(fields, canonicalFieldSep))
}

// ExternalCommitmentToken computes the keyed content binding of one
// envelope version: HMAC-SHA256(key, "external_commitment:v1:" + id +
// ":" + version_no + ":" + canonical bytes) in hex. The key is the
// per-tenant commitment key v1 (commitments_hmac_keys — the 2.9 reuse);
// the domain separation prefix differs from the 2.9 token space
// ("commitment:v1:") so the two token families never collide. The drain
// recomputes the token before the dispatch — a mutated row fails the
// re-check and the words never leave (AD-5 keyed/versioned token).
func ExternalCommitmentToken(key []byte, extID id.UUID, versionNo int64, canonical []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("external_commitment:v1:"))
	mac.Write([]byte(extID.String()))
	mac.Write([]byte(":"))
	mac.Write([]byte(fmt.Sprintf("%d", versionNo)))
	mac.Write([]byte(":"))
	mac.Write(canonical)
	return hex.EncodeToString(mac.Sum(nil))
}

// ExternalCommitmentContentToken recomputes the content token of the
// carried row — the drain re-check basis (the stored ContentToken must
// equal this recomputation, otherwise the row is not the row that was
// approved and the words never leave).
func ExternalCommitmentContentToken(key []byte, commitment ExternalCommitment) string {
	return ExternalCommitmentToken(key, commitment.ExtCommitmentID, commitment.VersionNo, ExternalCommitmentCanonicalBytes(commitment))
}

// ExternalContentDigest computes the plain SHA-256 digest of the content
// material (the digest class of the accepted-event payload precedent) —
// the outbox perimeter carries the digest, never the content (AC-10).
func ExternalContentDigest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// The create command guard (fail-closed, the dictionary honesty)
// ---------------------------------------------------------------------------

// CreateExternalCommitmentCommand is the material of the version creation:
// the identifiers, the dictionary words, the content, the window, the pin
// and the relation snapshots. The token is COMPUTED after the guard — the
// guard never validates what it can compute.
type CreateExternalCommitmentCommand struct {
	TenantID         id.UUID
	CaseID           id.UUID
	WorkItemID       id.UUID
	ExtCommitmentID  id.UUID
	VersionNo        int64
	Purpose          string
	RecipientKind    string
	Channel          string
	ContentText      string
	ValidUntil       time.Time
	ExpectedNextStep string
	AckRequirement   string
	Pin              ExternalCommitmentPin
	OwnerKind        string
	OwnerRef         id.UUID
	OwnerEpoch       int64
	DecisionNo       int64
	ProfileVersion   int64
	PolicyOutcomeNo  int64
}

// GuardCreateExternalCommitment validates the material of the version
// creation: the identifiers, the dictionary words (the channel comes from
// the nine-word cases mirror), the content bounds, the future validity,
// the next-step bound, the owner snapshot, the honest lineage numbers and
// the 2.9 pin (mandatory for the offer, the all-or-nothing triple
// otherwise). The freshness of the pinned 2.9 version and the live token
// match are the app gate — this guard is the dictionary honesty only.
func GuardCreateExternalCommitment(command CreateExternalCommitmentCommand, now time.Time) error {
	if !command.TenantID.Valid() {
		return fmt.Errorf("конверт требует действительный tenant")
	}
	if !command.CaseID.Valid() {
		return fmt.Errorf("конверт требует действительный case")
	}
	if !command.WorkItemID.Valid() {
		return fmt.Errorf("конверт требует действительный work item")
	}
	if !command.ExtCommitmentID.Valid() {
		return fmt.Errorf("конверт требует действительный стабильный ID")
	}
	if command.VersionNo < 1 {
		return fmt.Errorf("номер версии конверта начинается с 1")
	}
	if !inDictionary(ExternalPurposeWords, command.Purpose) {
		return fmt.Errorf("purpose «%s» не входит в словарь (%s)", command.Purpose, joinQuoted(ExternalPurposeWords))
	}
	if !inDictionary(ExternalRecipientKindWords, command.RecipientKind) {
		return fmt.Errorf("получатель «%s» не входит в словарь (%s)", command.RecipientKind, joinQuoted(ExternalRecipientKindWords))
	}
	if !inDictionary(ChannelOrder, command.Channel) {
		return fmt.Errorf("канал «%s» не входит в словарь (%s)", command.Channel, joinQuoted(ChannelOrder))
	}
	if strings.TrimSpace(command.ContentText) == "" {
		return fmt.Errorf("содержание конверта не бывает пустым")
	}
	if utf8.RuneCountInString(command.ContentText) > MaxExternalCommitmentContentRunes {
		return fmt.Errorf("содержание конверта длиннее %d символов", MaxExternalCommitmentContentRunes)
	}
	if !command.ValidUntil.After(now) {
		return fmt.Errorf("срок действия конверта обязан быть в будущем")
	}
	if strings.TrimSpace(command.ExpectedNextStep) == "" {
		return fmt.Errorf("ожидаемый следующий шаг не бывает пустым")
	}
	if utf8.RuneCountInString(command.ExpectedNextStep) > MaxExpectedNextStepRunes {
		return fmt.Errorf("ожидаемый следующий шаг длиннее %d символов", MaxExpectedNextStepRunes)
	}
	if !inDictionary(ExternalAckRequirementWords, command.AckRequirement) {
		return fmt.Errorf("требование acknowledgement «%s» не входит в словарь (%s)", command.AckRequirement, joinQuoted(ExternalAckRequirementWords))
	}
	if !inDictionary(OwnerKinds, command.OwnerKind) {
		return fmt.Errorf("вид владельца «%s» не входит в словарь (%s)", command.OwnerKind, joinQuoted(OwnerKinds))
	}
	if !command.OwnerRef.Valid() {
		return fmt.Errorf("снимок владельца требует действительную ссылку")
	}
	if command.OwnerEpoch < 1 {
		return fmt.Errorf("снимок владельца требует живой epoch (>= 1)")
	}
	if command.DecisionNo < 0 || command.ProfileVersion < 0 || command.PolicyOutcomeNo < 0 {
		return fmt.Errorf("lineage-числа конверта не бывают отрицательными (0 — честно нет)")
	}
	if command.Purpose == ExternalPurposeOffer && !command.Pin.Present() {
		return fmt.Errorf("purpose=offer требует пин точной версии обязательства (commitment_id, version, content token)")
	}
	if command.Pin.Present() {
		if err := GuardExternalCommitmentPin(command.Pin); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// The readiness derivation (read-time, never stored)
// ---------------------------------------------------------------------------

// ExternalReadinessInput is the live material of the read-time readiness:
// the current moment, the maximal version number of the envelope, the
// current owner epoch of the case and the derived readiness of the pinned
// 2.9 version (the DeriveCommitmentReadiness output — only the offer
// consumes it).
type ExternalReadinessInput struct {
	Now               time.Time
	MaxVersionNo      int64
	OwnerEpochCurrent int64
	PinReadiness      string
}

// DeriveExternalReadiness derives the read-time enqueue readiness of one
// envelope version — the stale fences IN ORDER (the drain re-checks the
// same order at the execute time): a superseded version never enqueues; an
// expired window never enqueues; a moved owner epoch never enqueues; an
// offer whose pinned 2.9 version is not confirmable never enqueues (the
// pin readiness must be the exact offered_confirmable word of the 2.9
// derivation). Everything else is ready. The derivation reads the
// material only — nothing is stored.
func DeriveExternalReadiness(version ExternalCommitment, input ExternalReadinessInput) string {
	switch {
	case version.VersionNo < input.MaxVersionNo:
		return ExternalReadinessSuperseded
	case !version.ValidUntil.After(input.Now):
		return ExternalReadinessExpired
	case input.OwnerEpochCurrent != version.OwnerEpoch:
		return ExternalReadinessOwnerStale
	case version.Purpose == ExternalPurposeOffer && input.PinReadiness != CommitmentReadinessConfirmable:
		return ExternalReadinessPinBlocked
	default:
		return ExternalReadinessReady
	}
}

// GuardEnqueueExternalCommitment refuses the enqueue of any non-ready
// readiness word BEFORE the transaction — the honest text names the fence,
// and a foreign word fails closed too (the ready path is the only open
// path).
func GuardEnqueueExternalCommitment(readiness string) error {
	if readiness == ExternalReadinessReady {
		return nil
	}
	if text, ok := ExternalReadinessRefusalTexts[readiness]; ok {
		return fmt.Errorf("%s", text)
	}
	return fmt.Errorf("готовность конверта не входит в словарь (%s) — отправка невозможна", joinQuoted(ExternalReadinessWords))
}

// ExternalApprovalRequiredText is the fixed refusal text of the approval
// gate (AC-9): the enqueue demands the approval of the EXACT version by
// the CURRENT human owner — an earlier approval of another version
// structurally never authorises the changed text.
func ExternalApprovalRequiredText(versionNo int64) string {
	return fmt.Sprintf("Требуется утверждение текущим человеком-владельцем (response owner), версия %d — ранее утверждённая версия не разрешает изменённый текст.", versionNo)
}

// ---------------------------------------------------------------------------
// The dispatch events (the append-only history) and the seven-state
// derivation (read-time, never stored)
// ---------------------------------------------------------------------------

// ExternalDispatchEvent is one append-only row of the dispatch history of
// one envelope version: the monotonic event number, the seven-word kind,
// the stale reason (the cancelled_stale material only), the attempt
// number (the attempt kinds only, 1..MaxDispatchAttempts), the external
// correlation id (the submitted/unknown evidence of AC-8), the safe
// details (correlation, outcome, cause — never the content) and the
// actor. The history is never rewritten; the LAST event decides the state.
type ExternalDispatchEvent struct {
	TenantID              id.UUID
	CaseID                id.UUID
	ExtCommitmentID       id.UUID
	VersionNo             int64
	EventID               id.UUID
	EventNo               int64
	Kind                  string
	Reason                string
	AttemptNo             int64
	ExternalCorrelationID string
	Details               string
	ActorID               id.UUID
	CreatedAt             time.Time
}

// ExternalDeliveryState is the derived delivery state of one envelope
// version: the seven-word state and — when at least one event exists —
// the material of the LAST event (the timeline anchor of the page).
type ExternalDeliveryState struct {
	State                     string
	LastEventNo               int64
	LastKind                  string
	LastReason                string
	LastAttemptNo             int64
	LastExternalCorrelationID string
	LastEventAt               time.Time
}

// DeriveExternalDeliveryState derives the read-time delivery state of one
// envelope version (the 2.12 derivation precedent — never a stored
// status): without the outbox record the truth is persisted (nothing has
// left); the outbox record without any event is queued (the words await
// the drain); otherwise the LAST event of the monotonic history decides —
// submitted, failed, unknown, delivered, acknowledged, and the cancelled
// stale fence returns the record to persisted honestly: the words never
// left, the reason stays in the history as the material. A foreign kind
// (impossible through the guard and the table CHECK) fails safely to
// unknown — the acknowledged word requires the exact receipt event, never
// a default.
func DeriveExternalDeliveryState(enqueued bool, events []ExternalDispatchEvent) ExternalDeliveryState {
	if !enqueued {
		return ExternalDeliveryState{State: ExternalDeliveryPersisted}
	}
	if len(events) == 0 {
		return ExternalDeliveryState{State: ExternalDeliveryQueued}
	}
	var last *ExternalDispatchEvent
	for i := range events {
		if last == nil || events[i].EventNo >= last.EventNo {
			last = &events[i]
		}
	}
	state := ExternalDeliveryState{
		State:                     ExternalDeliveryUnknown,
		LastEventNo:               last.EventNo,
		LastKind:                  last.Kind,
		LastReason:                last.Reason,
		LastAttemptNo:             last.AttemptNo,
		LastExternalCorrelationID: last.ExternalCorrelationID,
		LastEventAt:               last.CreatedAt,
	}
	switch last.Kind {
	case ExternalDispatchAttemptSubmitted:
		state.State = ExternalDeliverySubmitted
	case ExternalDispatchAttemptFailed:
		state.State = ExternalDeliveryFailed
	case ExternalDispatchAttemptUnknown:
		state.State = ExternalDeliveryUnknown
	case ExternalDispatchReceiptDelivered:
		state.State = ExternalDeliveryDelivered
	case ExternalDispatchReceiptAcknowledged:
		state.State = ExternalDeliveryAcknowledged
	case ExternalDispatchReceiptFailed:
		state.State = ExternalDeliveryFailed
	case ExternalDispatchCancelledStale:
		state.State = ExternalDeliveryPersisted
	}
	return state
}

// ---------------------------------------------------------------------------
// The relay and outcome guards (the honest machine boundaries)
// ---------------------------------------------------------------------------

// RelayObservationForeignText is the fixed refusal of a foreign relay
// observation word: the operator passes the channel observation only, an
// invented outcome never passes (the 2.12 precedent).
const RelayObservationForeignText = "Слово наблюдения оператора не входит в словарь (%s): передайте наблюдение канала — доставлено, принято или отказ."

// GuardRelayObservation validates the observation word of the operator
// relay (the dispatch-receipts surface).
func GuardRelayObservation(word string) error {
	if !inDictionary(RelayObservationWords, word) {
		return fmt.Errorf(RelayObservationForeignText, joinQuoted(RelayObservationWords))
	}
	return nil
}

// RelayObservationDispatchKind maps the relay observation word to the
// receipt event kind (the empty string of a foreign word — guard first).
func RelayObservationDispatchKind(word string) string {
	switch word {
	case RelayObservationDelivered:
		return ExternalDispatchReceiptDelivered
	case RelayObservationAcknowledged:
		return ExternalDispatchReceiptAcknowledged
	case RelayObservationFailed:
		return ExternalDispatchReceiptFailed
	}
	return ""
}

// DispatchOutcomeForeignText is the fixed refusal of a foreign adapter
// outcome word.
const DispatchOutcomeForeignText = "Исход диспетчера «%s» не входит в словарь (%s)."

// GuardDispatchOutcome validates the outcome word of the adapter result.
func GuardDispatchOutcome(outcome string) error {
	if !inDictionary(DispatchOutcomeWords, outcome) {
		return fmt.Errorf(DispatchOutcomeForeignText, outcome, joinQuoted(DispatchOutcomeWords))
	}
	return nil
}

// ExternalStaleReasonForeignText is the fixed refusal of a foreign stale
// reason word.
const ExternalStaleReasonForeignText = "Причина гашения «%s» не входит в словарь (%s)."

// GuardExternalStaleReason validates the reason word of the stale fence.
func GuardExternalStaleReason(reason string) error {
	if !inDictionary(ExternalStaleReasonWords, reason) {
		return fmt.Errorf(ExternalStaleReasonForeignText, reason, joinQuoted(ExternalStaleReasonWords))
	}
	return nil
}

// ---------------------------------------------------------------------------
// The outbox perimeter (the safe payload — the material never leaves the
// row, AC-10)
// ---------------------------------------------------------------------------

// The outbox event type of the enqueue (the 2.1 bounded vocabulary).
const (
	OutboxEventExternalCommitmentQueued   = "external_commitment.queued"
	OutboxEventExternalCommitmentQueuedV1 = 1
)

// ExternalCommitmentQueuedPayload is the safe canonical JSON of the queued
// outbox record: the identifiers, the version pin, the dictionary words,
// the idempotency key and the content digest — the content text NEVER
// enters the envelope (the candidate precedent of 2.6; the full material
// travels only inside the outbound payload the drain builds from the
// immutable version).
type ExternalCommitmentQueuedPayload struct {
	TenantID        id.UUID `json:"tenant_id"`
	CaseID          id.UUID `json:"case_id"`
	ExtCommitmentID id.UUID `json:"ext_commitment_id"`
	VersionNo       int64   `json:"version_no"`
	Purpose         string  `json:"purpose"`
	RecipientKind   string  `json:"recipient_kind"`
	Channel         string  `json:"channel"`
	IdempotencyKey  string  `json:"idempotency_key"`
	ContentDigest   string  `json:"content_digest"`
}

// DedupKeyExternalCommitmentQueued builds the dedup key of the queued
// record: the version pin itself — the UNIQUE constraint is the second
// dedup line, the repeated enqueue of the SAME version never creates a
// second intent (the first line is the event check of the drain).
func DedupKeyExternalCommitmentQueued(extID id.UUID, versionNo int64) string {
	return "external_commitment.queued:" + extID.String() + ":" + fmt.Sprintf("%d", versionNo)
}

// ExternalCommitmentQueuedPayloadJSON renders the safe canonical JSON.
func ExternalCommitmentQueuedPayloadJSON(payload ExternalCommitmentQueuedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// ---------------------------------------------------------------------------
// The idempotency key (the HMAC derivative of the version pin)
// ---------------------------------------------------------------------------

// ExternalCommitmentIdempotencyKey derives the idempotency key of one
// envelope version: HMAC-SHA256(keyed by the tenant bytes,
// "external_commitment:idem:v1:" + ext_id + ":" + version_no) in hex.
// The key is the pure derivative of the version pin — the repeated
// delivery of the SAME version carries the SAME key (the provider
// deduplicates), and a changed version NEVER reuses the old key (the new
// version number changes the message). The key deliberately does NOT
// include the attempt number — otherwise the safe retry would stop being
// safe. The output is opaque: the identifiers never leak to the provider.
func ExternalCommitmentIdempotencyKey(tenantID id.UUID, extID id.UUID, versionNo int64) string {
	mac := hmac.New(sha256.New, []byte(tenantID.String()))
	mac.Write([]byte("external_commitment:idem:v1:"))
	mac.Write([]byte(extID.String()))
	mac.Write([]byte(":"))
	mac.Write([]byte(fmt.Sprintf("%d", versionNo)))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// The outbound payload (deterministic from the immutable version row)
// ---------------------------------------------------------------------------

// OutboundExternalCommitment is the delivery material the drain hands to
// the dispatcher: built DETERMINISTICALLY from the immutable version row
// only — never from a snapshot, so a later draft change (a new version
// row) never alters the queued payload of the pinned version (AC-4). The
// content text travels here — this is the material being delivered; the
// outbox record and the events carry the safe envelope only.
type OutboundExternalCommitment struct {
	ExtCommitmentID  id.UUID   `json:"ext_commitment_id"`
	VersionNo        int64     `json:"version_no"`
	Purpose          string    `json:"purpose"`
	RecipientKind    string    `json:"recipient_kind"`
	Channel          string    `json:"channel"`
	ContentText      string    `json:"content_text"`
	ValidUntil       time.Time `json:"valid_until"`
	ExpectedNextStep string    `json:"expected_next_step"`
	AckRequirement   string    `json:"ack_requirement"`
	IdempotencyKey   string    `json:"idempotency_key"`
}

// BuildOutboundExternalPayload builds the outbound payload from the
// immutable version row and the idempotency key derivative — a pure
// function: the same row always yields the same payload byte-to-byte.
func BuildOutboundExternalPayload(commitment ExternalCommitment, idempotencyKey string) OutboundExternalCommitment {
	return OutboundExternalCommitment{
		ExtCommitmentID:  commitment.ExtCommitmentID,
		VersionNo:        commitment.VersionNo,
		Purpose:          commitment.Purpose,
		RecipientKind:    commitment.RecipientKind,
		Channel:          commitment.Channel,
		ContentText:      commitment.ContentText,
		ValidUntil:       commitment.ValidUntil.UTC(),
		ExpectedNextStep: commitment.ExpectedNextStep,
		AckRequirement:   commitment.AckRequirement,
		IdempotencyKey:   idempotencyKey,
	}
}
