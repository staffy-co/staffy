package domain

import (
	"crypto/hmac"
	"crypto/rand"
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
// The commitment surface (Story 2.9, first slice): the dictionaries, the
// MaterialTerms v1, the versioned commitment, the exact-version confirmation
// ---------------------------------------------------------------------------
//
// The dictionaries of the Domain Pack v0 are fixed: the version statuses
// (draft, offered), the mandatory material terms (scope, service_items,
// price, payee, warranty_role, next_step, consequences), the action words
// v1 (prepare_order, schedule_appointment, issue_document — words only,
// no execution exists as a class: the Action Gateway is Epic 3), the
// confirmation method v1 (explicit_text) and the decline outcomes
// (declined, change_requested). No word appears outside its dictionary —
// the CHECK backstops of the tables and the guards carry the same words
// (the ruleTableV1 precedent of 2.6/2.7/2.8).
//
// The commitment is versioned append-only: a changed offer is a NEW row
// with version_no+1; the old versions never mutate. The confirmation binds
// to the EXACT shown version: the presenting sales-reply carries the pair
// (commitment_id, commitment_version), and the confirm command demands the
// verbatim span excerpts of the client words of the accepted text turn
// (the span honesty precedent of 2.8 — an invented consent is impossible).
//
// The material snapshot never leaves the row: the outbox events carry the
// identifiers and the dictionary words only (the «кандидат не покидает
// строку» precedent of 2.6/2.7/2.8). The content binding is the keyed
// HMAC-SHA256 token over the canonical material bytes — the per-tenant
// commitment key v1 (32 bytes, INSERT-only, rotation honestly deferred —
// the identifier keys precedent of 2.3).

// ---------------------------------------------------------------------------
// Dictionaries
// ---------------------------------------------------------------------------

// The commitment version statuses: draft is the honest non-presentable
// state (a draft is not a confirmable obligation — AC-1); offered is the
// presentable state of the exact material conditions.
const (
	CommitmentStatusDraft   = "draft"
	CommitmentStatusOffered = "offered"
)

// CommitmentStatuses is the full dictionary.
var CommitmentStatuses = []string{CommitmentStatusDraft, CommitmentStatusOffered}

// The mandatory material terms (AC-2): every offered version carries the
// full typed material — the scope, the service items, the price (amount or
// range), the payee, the warranty role, the next step and the consequences.
// An unknown MANDATORY term blocks the readiness (the honest hole — the
// offer exists but is not confirmable until the term is known).
const (
	CommitmentTermScope        = "scope"
	CommitmentTermServiceItems = "service_items"
	CommitmentTermPrice        = "price"
	CommitmentTermPayee        = "payee"
	CommitmentTermWarrantyRole = "warranty_role"
	CommitmentTermNextStep     = "next_step"
	CommitmentTermConsequences = "consequences"
)

// CommitmentMandatoryTerms is the full dictionary (the fixed order of the
// canonical serialization).
var CommitmentMandatoryTerms = []string{
	CommitmentTermScope, CommitmentTermServiceItems, CommitmentTermPrice,
	CommitmentTermPayee, CommitmentTermWarrantyRole, CommitmentTermNextStep,
	CommitmentTermConsequences,
}

// The action words v1 (AC-2/FR-27 boundary): the fixed dictionary of the
// actions the confirmation names — the WORDS only, the execution belongs
// to the Action Gateway of Epic 3 (no mutation adapter exists here).
const (
	CommitmentActionPrepareOrder        = "prepare_order"
	CommitmentActionScheduleAppointment = "schedule_appointment"
	CommitmentActionIssueDocument       = "issue_document"
)

// CommitmentActions is the full dictionary.
var CommitmentActions = []string{
	CommitmentActionPrepareOrder, CommitmentActionScheduleAppointment, CommitmentActionIssueDocument,
}

// The confirmation method v1: the explicit text of the client words (the
// voice readback belongs to the later slices — a voice turn refuses
// honestly with the modality dictionary).
const CommitmentMethodExplicitText = "explicit_text"

// CommitmentMethods is the full dictionary.
var CommitmentMethods = []string{CommitmentMethodExplicitText}

// The decline outcomes: declined is the honest refusal, change_requested
// is the demand of a new version (the operator creates version+1; nothing
// is mutated — AC-11).
const (
	CommitmentDeclineOutcomeDeclined        = "declined"
	CommitmentDeclineOutcomeChangeRequested = "change_requested"
)

// CommitmentDeclineOutcomes is the full dictionary.
var CommitmentDeclineOutcomes = []string{CommitmentDeclineOutcomeDeclined, CommitmentDeclineOutcomeChangeRequested}

// The confirmation validity words of the read-time derivation: expired
// (the validity time passed), superseded (a newer version exists),
// owner_stale (the case owner epoch moved), release_stale (the release
// lineage moved). revoked and consumed are the RESERVED words without a
// producer in this slice (the cancelling command is a later slice, the
// consumption is the Action Gateway of Epic 3 — the reserved
// mandatory_missing precedent of 2.6).
const (
	ConfirmationValidityValid        = "valid"
	ConfirmationValidityExpired      = "expired"
	ConfirmationValiditySuperseded   = "superseded"
	ConfirmationValidityOwnerStale   = "owner_stale"
	ConfirmationValidityReleaseStale = "release_stale"
	ConfirmationValidityRevoked      = "revoked"
	ConfirmationValidityConsumed     = "consumed"
)

// ConfirmationValidityWords is the full dictionary (the reserved words
// included — the honest reservation, never a hidden producer).
var ConfirmationValidityWords = []string{
	ConfirmationValidityValid, ConfirmationValidityExpired, ConfirmationValiditySuperseded,
	ConfirmationValidityOwnerStale, ConfirmationValidityReleaseStale,
	ConfirmationValidityRevoked, ConfirmationValidityConsumed,
}

// The commitment limits (the fixed dictionary of the material discipline):
// the action set carries 1..4 dictionary words, the consumption limit is
// 1..10, the canonical conditions bytes are at most 4096, the span
// discipline of the evidence is 1..8 excerpts of at most 200 runes each
// (the objection span limits of 2.8).
const (
	MinCommitmentActions        = 1
	MaxCommitmentActions        = 4
	MinCommitmentConsumption    = 1
	MaxCommitmentConsumption    = 10
	MaxCommitmentConditionsByte = 4096
	MaxCommitmentServiceItems   = 8
	MaxCommitmentTermRunes      = 200
)

// ---------------------------------------------------------------------------
// The price term (amount or range, cents)
// ---------------------------------------------------------------------------

// The price kinds: amount is the exact cents, range is the honest interval
// (0 < min < max). A price of zero does not exist — the free cases are not
// commitments of this dictionary.
const (
	CommitmentPriceKindAmount = "amount"
	CommitmentPriceKindRange  = "range"
)

// CommitmentPrice is the typed price term: the exact amount in cents or
// the min/max range in cents (exclusively — the canonical JSON carries the
// kind and the set fields only, the deterministic serialization of the
// fixed struct order).
type CommitmentPrice struct {
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amount_cents,omitempty"`
	MinCents    int64  `json:"min_cents,omitempty"`
	MaxCents    int64  `json:"max_cents,omitempty"`
}

// GuardCommitmentPrice validates the price term: the kind is the
// dictionary word; the amount is positive and carries no range; the range
// is positive with min < max and carries no amount.
func GuardCommitmentPrice(price CommitmentPrice) error {
	switch price.Kind {
	case CommitmentPriceKindAmount:
		if price.AmountCents <= 0 {
			return fmt.Errorf("цена-сумма обязана быть положительной (в копейках), получено %d", price.AmountCents)
		}
		if price.MinCents != 0 || price.MaxCents != 0 {
			return fmt.Errorf("цена-сумма не несёт диапазон: min/max обязаны быть пустыми")
		}
	case CommitmentPriceKindRange:
		if price.MinCents <= 0 {
			return fmt.Errorf("нижняя граница диапазона обязана быть положительной (в копейках), получено %d", price.MinCents)
		}
		if price.MinCents >= price.MaxCents {
			return fmt.Errorf("диапазон цены обязан быть честным интервалом: min < max, получено [%d..%d]", price.MinCents, price.MaxCents)
		}
		if price.AmountCents != 0 {
			return fmt.Errorf("цена-диапазон не несёт сумму: amount обязан быть пустым")
		}
	default:
		return fmt.Errorf("вид цены «%s» не входит в словарь (%s, %s)", price.Kind, CommitmentPriceKindAmount, CommitmentPriceKindRange)
	}
	return nil
}

// PriceKnown reports whether the price term carries a known value (the
// material_unknowns price never reaches this term — the honest hole of the
// unknown mandatory term).
func (p CommitmentPrice) PriceKnown() bool {
	return p.Kind == CommitmentPriceKindAmount || p.Kind == CommitmentPriceKindRange
}

// PricePositive reports whether the price requires the authority gate: the
// amount > 0 or the range min > 0 (the rule table v1 — a commitment with
// money always demands the mandate).
func (p CommitmentPrice) PricePositive() bool {
	return p.Kind == CommitmentPriceKindAmount && p.AmountCents > 0 ||
		p.Kind == CommitmentPriceKindRange && p.MinCents > 0
}

// PriceCentsOf resolves the comparable price value: the amount or the
// range minimum (the observation-backed comparison reads the single
// honest number of the term — the byte equality of the canonical value
// stays the gate's job).
func (p CommitmentPrice) PriceCentsOf() int64 {
	switch p.Kind {
	case CommitmentPriceKindAmount:
		return p.AmountCents
	case CommitmentPriceKindRange:
		return p.MinCents
	}
	return 0
}

// PriceValueJSON renders the canonical JSON bytes of the price term — the
// byte-equality material of the freshness gate (the observation value of
// 2.7 carries the canonical JSON of the field too).
func PriceValueJSON(price CommitmentPrice) ([]byte, error) {
	return json.Marshal(price)
}

// ---------------------------------------------------------------------------
// The MaterialTerms v1
// ---------------------------------------------------------------------------

// MaterialTerms is the typed material of one commitment version: the
// mandatory terms (AC-2), the action set and the limits. The struct field
// order IS the canonical key order of the deterministic serialization —
// json.Marshal renders the stable bytes (no spaces, the fixed order).
type MaterialTerms struct {
	Scope        string          `json:"scope"`
	ServiceItems []string        `json:"service_items"`
	Price        CommitmentPrice `json:"price"`
	Payee        string          `json:"payee"`
	WarrantyRole string          `json:"warranty_role"`
	NextStep     string          `json:"next_step"`
	Consequences string          `json:"consequences"`
}

// GuardMaterialTerms validates the material terms of one version: every
// string term is a non-empty bounded string, the service items are 1..8
// bounded strings without duplicates, the price passes its guard.
func GuardMaterialTerms(terms MaterialTerms) error {
	for term, value := range map[string]string{
		CommitmentTermScope:        terms.Scope,
		CommitmentTermPayee:        terms.Payee,
		CommitmentTermWarrantyRole: terms.WarrantyRole,
		CommitmentTermNextStep:     terms.NextStep,
		CommitmentTermConsequences: terms.Consequences,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("обязательный термин %s не бывает пустым", term)
		}
		if utf8.RuneCountInString(value) > MaxCommitmentTermRunes {
			return fmt.Errorf("обязательный термин %s длиннее %d символов", term, MaxCommitmentTermRunes)
		}
	}
	if len(terms.ServiceItems) < 1 {
		return fmt.Errorf("перечень услуг/позиций обязан нести хотя бы одну позицию (1..%d)", MaxCommitmentServiceItems)
	}
	if len(terms.ServiceItems) > MaxCommitmentServiceItems {
		return fmt.Errorf("перечень услуг/позиций несёт не больше %d позиций, получено %d", MaxCommitmentServiceItems, len(terms.ServiceItems))
	}
	seen := make(map[string]bool, len(terms.ServiceItems))
	for _, item := range terms.ServiceItems {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("позиция перечня услуг не бывает пустой")
		}
		if utf8.RuneCountInString(item) > MaxCommitmentTermRunes {
			return fmt.Errorf("позиция перечня услуг длиннее %d символов", MaxCommitmentTermRunes)
		}
		if seen[item] {
			return fmt.Errorf("позиции перечня услуг не повторяются: дубликат «%s»", item)
		}
		seen[item] = true
	}
	return GuardCommitmentPrice(terms.Price)
}

// GuardMaterialUnknowns validates the unknown words: only the mandatory
// term words, no duplicates (an unknown NON-mandatory term is a dictionary
// error — the honest hole exists for the material terms only).
func GuardMaterialUnknowns(unknowns []string) error {
	seen := make(map[string]bool, len(unknowns))
	for _, unknown := range unknowns {
		if !inDictionary(CommitmentMandatoryTerms, unknown) {
			return fmt.Errorf("неизвестный термин «%s» не входит в словарь обязательных терминов (%s)", unknown, joinQuoted(CommitmentMandatoryTerms))
		}
		if seen[unknown] {
			return fmt.Errorf("неизвестные термины не повторяются: дубликат «%s»", unknown)
		}
		seen[unknown] = true
	}
	return nil
}

// GuardCommitmentActionSet validates the action set: 1..4 dictionary
// words without duplicates (the words are the execution-naming only — no
// execution exists as a class).
func GuardCommitmentActionSet(actions []string) error {
	if len(actions) < MinCommitmentActions {
		return fmt.Errorf("набор действий обязан нести хотя бы одно слово (1..%d)", MaxCommitmentActions)
	}
	if len(actions) > MaxCommitmentActions {
		return fmt.Errorf("набор действий несёт не больше %d слов, получено %d", MaxCommitmentActions, len(actions))
	}
	seen := make(map[string]bool, len(actions))
	for _, action := range actions {
		if !inDictionary(CommitmentActions, action) {
			return fmt.Errorf("слово действия «%s» не входит в словарь (%s)", action, joinQuoted(CommitmentActions))
		}
		if seen[action] {
			return fmt.Errorf("слова действий не повторяются: дубликат «%s»", action)
		}
		seen[action] = true
	}
	return nil
}

// GuardCommitmentConsumptionLimit validates the consumption limit word:
// 1..10 (the consumption itself is the Action Gateway of Epic 3 — the
// limit is the honest snapshot of the moment).
func GuardCommitmentConsumptionLimit(limit int) error {
	if limit < MinCommitmentConsumption || limit > MaxCommitmentConsumption {
		return fmt.Errorf("лимит исполнений обязан быть в границах %d..%d, получено %d", MinCommitmentConsumption, MaxCommitmentConsumption, limit)
	}
	return nil
}

// MaterialTermsJSON renders the canonical JSON bytes of the material
// terms (the deterministic serialization: the fixed struct key order, no
// spaces — the content binding and the byte equality read these bytes).
func MaterialTermsJSON(terms MaterialTerms) ([]byte, error) {
	encoded, err := json.Marshal(terms)
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaxCommitmentConditionsByte {
		return nil, fmt.Errorf("канонические условия длиннее %d байт, получено %d", MaxCommitmentConditionsByte, len(encoded))
	}
	return encoded, nil
}

// MaterialUnknownTerm reports whether one mandatory term is marked
// unknown in the version material.
func MaterialUnknownTerm(unknowns []string, term string) bool {
	for _, unknown := range unknowns {
		if unknown == term {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// The rule table v1 (the observation-backed terms and the authority
// trigger)
// ---------------------------------------------------------------------------

// CommitmentObservationBackedTerms is the fixed rule table v1 of the
// observation-backed terms: price is owned by the catalog reading (the
// matrix 2.7: price → catalog/price_quote, freshness 300 seconds). An
// offered version with a KNOWN observation-backed term demands the
// current ok+fresh observation with the byte-equal value; the confirm
// gate re-checks the pinned observation is STILL the current one (the
// fail-closed discipline — a newer reading demands a new version).
var CommitmentObservationBackedTerms = []string{CommitmentTermPrice}

// ObservationBackedTerm reports whether one mandatory term demands the
// observation material of the owning system.
func ObservationBackedTerm(term string) bool {
	return inDictionary(CommitmentObservationBackedTerms, term)
}

// ---------------------------------------------------------------------------
// The commitment version (the append-only row)
// ---------------------------------------------------------------------------

// CommitmentVersion is one append-only row of the versioned commitment:
// the stable commitment id with the monotonic version number (a new
// version is a NEW row — the old rows never mutate, AC-8), the full
// material conditions, the action set, the limits, the relation pins (the
// work item, the response owner with the epoch, the release lineage word
// — the 2.4 precedent, the current policy outcome — 2.6, the pinned
// observation of the observation-backed terms) and the content token.
type CommitmentVersion struct {
	TenantID                 id.UUID
	CaseID                   id.UUID
	WorkItemID               id.UUID
	CommitmentID             id.UUID
	VersionNo                int64
	Status                   string
	Conditions               []byte
	ActionSet                []string
	ValidUntil               time.Time
	ConsumptionLimit         int
	MaterialUnknowns         []string
	OwnerKind                string
	OwnerRef                 id.UUID
	OwnerEpoch               int64
	LineageWord              string
	PolicyOutcomeNo          int64
	PinnedPriceObservationNo int64
	ContentToken             string
	ActorID                  id.UUID
	CreatedAt                time.Time
}

// CurrentCommitmentVersion resolves the CURRENT version of one
// commitment: the maximal version number (nil when no version exists —
// the honest pre-commitment state of the panel).
func CurrentCommitmentVersion(versions []CommitmentVersion) *CommitmentVersion {
	var current *CommitmentVersion
	for i := range versions {
		if current == nil || versions[i].VersionNo > current.VersionNo {
			current = &versions[i]
		}
	}
	return current
}

// ---------------------------------------------------------------------------
// The readiness derivation (read-time, never stored)
// ---------------------------------------------------------------------------

// The readiness words: draft is the non-presentable state;
// offered_unknown_blocked is the honest hole (an unknown mandatory term
// blocks the confirmation — AC-2); offered_confirmable is the readiness
// of the exact material; superseded is the honest age (a newer version
// exists — the old one stays in the history, never mutates).
const (
	CommitmentReadinessDraft          = "draft"
	CommitmentReadinessUnknownBlocked = "offered_unknown_blocked"
	CommitmentReadinessConfirmable    = "offered_confirmable"
	CommitmentReadinessSuperseded     = "superseded"
)

// CommitmentReadinessWords is the full dictionary.
var CommitmentReadinessWords = []string{
	CommitmentReadinessDraft, CommitmentReadinessUnknownBlocked,
	CommitmentReadinessConfirmable, CommitmentReadinessSuperseded,
}

// DeriveCommitmentReadiness derives the read-time readiness of one
// commitment version: draft never confirms (AC-1); a superseded version
// (the maximal version number moved past it) never confirms; an offered
// version with an unknown MANDATORY term is the honest hole — not
// confirmable until the term is known (AC-2); the rest is confirmable.
// The derivation reads the version material only — nothing is stored.
func DeriveCommitmentReadiness(version CommitmentVersion, maxVersionNo int64) string {
	switch {
	case version.Status == CommitmentStatusDraft:
		return CommitmentReadinessDraft
	case version.VersionNo < maxVersionNo:
		return CommitmentReadinessSuperseded
	case len(version.MaterialUnknowns) > 0:
		return CommitmentReadinessUnknownBlocked
	default:
		return CommitmentReadinessConfirmable
	}
}

// ---------------------------------------------------------------------------
// The confirmation validity derivation (read-time, never stored)
// ---------------------------------------------------------------------------

// ConfirmationRecord is one append-only row of the exact-version
// confirmation: the acting operator, the confirmed subject (kind+ref),
// the method word, the channel, the expiry snapshot (the valid_until of
// the confirmed version), the consumption state, the evidence reference,
// the material snapshot, the action set snapshot and the content binding
// token. The validity is NEVER a stored status — it is the derivation of
// the current material (a replay of an old event never resurrects it).
type ConfirmationRecord struct {
	TenantID         id.UUID
	CaseID           id.UUID
	WorkItemID       id.UUID
	CommitmentID     id.UUID
	VersionNo        int64
	ConfirmationID   id.UUID
	SubjectKind      string
	SubjectRef       string
	Method           string
	Channel          string
	ExpiresAt        time.Time
	ConsumedCount    int
	ConsumptionLimit int
	TurnRef          id.UUID
	ActorID          id.UUID
	ConfirmedAt      time.Time
}

// ConfirmationValidityInput is the material of the read-time derivation:
// the current moment, the confirmed expiry snapshot, the current maximal
// version of the commitment, the current owner epoch of the case, the
// current release lineage word and the pins the confirmation carries.
type ConfirmationValidityInput struct {
	Now               time.Time
	ExpiresAt         time.Time
	MaxVersionNo      int64
	ConfirmedVersion  int64
	OwnerEpochPinned  int64
	OwnerEpochCurrent int64
	LineagePinned     string
	LineageCurrent    string
}

// DeriveConfirmationValidity derives the validity word of one
// confirmation (read-time only): expired when the validity time passed;
// superseded when a newer version of the commitment exists; owner_stale
// when the case owner epoch moved past the pin; release_stale when the
// release lineage moved past the pin; valid otherwise. revoked and
// consumed are the RESERVED words — no producer exists in this slice
// (the derivation never returns them; the dictionary carries them for
// the honest reservation).
func DeriveConfirmationValidity(input ConfirmationValidityInput) string {
	switch {
	case input.Now.After(input.ExpiresAt):
		return ConfirmationValidityExpired
	case input.MaxVersionNo > input.ConfirmedVersion:
		return ConfirmationValiditySuperseded
	case input.OwnerEpochCurrent != input.OwnerEpochPinned:
		return ConfirmationValidityOwnerStale
	case input.LineageCurrent != input.LineagePinned:
		return ConfirmationValidityReleaseStale
	default:
		return ConfirmationValidityValid
	}
}

// ---------------------------------------------------------------------------
// The content binding token (the keyed versioned HMAC)
// ---------------------------------------------------------------------------

// CommitmentToken computes the keyed content binding of one commitment
// version: HMAC-SHA256(key, "commitment:v1:" + commitment_id + ":" +
// version_no + ":" + material bytes) in hex (char(64)). The key is the
// per-tenant commitment key v1 (32 bytes, INSERT-only — the identifier
// keys precedent of 2.3); the domain separation prefix, the id and the
// version bind the token to the EXACT version — the same material under
// another version never matches. The token never exposes the material
// (the raw hashes of the material values never appear as a class).
func CommitmentToken(key []byte, commitmentID id.UUID, versionNo int64, material []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("commitment:v1:"))
	mac.Write([]byte(commitmentID.String()))
	mac.Write([]byte(":"))
	mac.Write([]byte(fmt.Sprintf("%d", versionNo)))
	mac.Write([]byte(":"))
	mac.Write(material)
	return hex.EncodeToString(mac.Sum(nil))
}

// NewCommitmentKey generates the per-tenant commitment key material (32
// random bytes — the same size discipline as the identifier keys).
func NewCommitmentKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate commitment key entropy: %w", err)
	}
	return key, nil
}

// ---------------------------------------------------------------------------
// The command guards
// ---------------------------------------------------------------------------

// CreateCommitmentCommand is the material of the create_commitment
// command (the operator review): the typed material, the action set, the
// limits, the status word and the unknown terms of the honest holes.
type CreateCommitmentCommand struct {
	Status           string
	Terms            MaterialTerms
	ActionSet        []string
	ValidUntil       time.Time
	ConsumptionLimit int
	MaterialUnknowns []string
}

// GuardCreateCommitment validates the material of the version creation:
// the status word, the material terms, the action set, the consumption
// limit, the unknown words and the future validity. The observation gate
// (the offered status with the known price term) is the app-side
// freshness check of the owning-system reading — the domain guard
// carries the dictionary and the limit honesty.
func GuardCreateCommitment(command CreateCommitmentCommand, now time.Time) error {
	if !inDictionary(CommitmentStatuses, command.Status) {
		return fmt.Errorf("статус версии «%s» не входит в словарь (%s)", command.Status, joinQuoted(CommitmentStatuses))
	}
	if err := GuardMaterialTerms(command.Terms); err != nil {
		return err
	}
	if err := GuardCommitmentActionSet(command.ActionSet); err != nil {
		return err
	}
	if err := GuardCommitmentConsumptionLimit(command.ConsumptionLimit); err != nil {
		return err
	}
	if err := GuardMaterialUnknowns(command.MaterialUnknowns); err != nil {
		return err
	}
	if !command.ValidUntil.After(now) {
		return fmt.Errorf("срок действия версии обязан быть в будущем")
	}
	return nil
}

// TerminalWorkItemCommitmentText is the honest refusal of the commitment
// commands of a terminal work item (the write-command variant of the 2.2
// boundary — the merged item lives on in its target).
const TerminalWorkItemCommitmentText = "Терминальный элемент работы не принимает обязательств: resolved, split и merged — финальные состояния; для split/merged создавайте обязательство на целевом элементе работы."

// ConfirmCommitmentMaterial is the evidence material of the
// confirm_commitment command: the verbatim span excerpts of the client
// words (the byte-exact substrings of the accepted text turn — the span
// discipline of 2.8), the confirmed subject and the turn source.
type ConfirmCommitmentMaterial struct {
	SubjectKind string
	SubjectRef  string
	TurnPayload []byte
	Spans       []string
}

// GuardConfirmEvidence validates the evidence of the confirmation: the
// subject words are non-empty, the spans are 1..8 bounded strings
// without duplicates and every span is the byte-exact substring of the
// turn payload (an invented consent is impossible — AC-6).
func GuardConfirmEvidence(material ConfirmCommitmentMaterial) error {
	if strings.TrimSpace(material.SubjectKind) == "" || strings.TrimSpace(material.SubjectRef) == "" {
		return fmt.Errorf("субъект подтверждения обязан нести вид и ссылку (kind+ref)")
	}
	if len(material.Spans) < 1 {
		return fmt.Errorf("подтверждение обязано нести дословные выдержки слов согласия клиента (1..%d)", MaxObjectionSpans)
	}
	if len(material.Spans) > MaxObjectionSpans {
		return fmt.Errorf("подтверждение несёт не больше %d выдержек, получено %d", MaxObjectionSpans, len(material.Spans))
	}
	seen := make(map[string]bool, len(material.Spans))
	for _, span := range material.Spans {
		if span == "" {
			return fmt.Errorf("выдержка не бывает пустой: дословные слова согласия обязательны")
		}
		if utf8.RuneCountInString(span) > MaxObjectionSpanRunes {
			return fmt.Errorf("выдержка длиннее %d символов", MaxObjectionSpanRunes)
		}
		if seen[span] {
			return fmt.Errorf("выдержки не повторяются: дубликат «%s»", span)
		}
		seen[span] = true
	}
	for _, span := range material.Spans {
		if !SpanInPayload(span, material.TurnPayload) {
			return fmt.Errorf("выдержка «%s» не входит в сообщение-источник дословно: выдуманное согласие невозможно", span)
		}
	}
	return nil
}

// GuardConfirmReadiness validates the readiness gates of the confirm
// command against the derived readiness word and the validity time: the
// draft never confirms (AC-1), the superseded version never confirms
// (AC-8 — a new version is the honest path), the unknown-blocked version
// never confirms (AC-2), the passed validity never confirms (the expiry
// demands a new version with a new valid_until).
func GuardConfirmReadiness(readiness string, validUntil time.Time, now time.Time) error {
	switch readiness {
	case CommitmentReadinessDraft:
		return fmt.Errorf("черновик не считается подтверждаемым обязательством: предъявите offered-версию")
	case CommitmentReadinessSuperseded:
		return fmt.Errorf("версия устарела: действует новая версия обязательства — подтверждается только текущая")
	case CommitmentReadinessUnknownBlocked:
		return fmt.Errorf("неизвестный обязательный термин блокирует готовность: заполните материальные условия новой версией")
	}
	if !validUntil.After(now) {
		return fmt.Errorf("срок действия версии истёк: требуется новая версия с новым сроком")
	}
	return nil
}

// GuardDeclineCommitment validates the decline material: the outcome
// word and the evidence spans (the same span discipline — the honest
// refusal carries the verbatim words of the client).
func GuardDeclineCommitment(outcome string, material ConfirmCommitmentMaterial) error {
	if !inDictionary(CommitmentDeclineOutcomes, outcome) {
		return fmt.Errorf("исход отказа «%s» не входит в словарь (%s)", outcome, joinQuoted(CommitmentDeclineOutcomes))
	}
	return GuardConfirmEvidence(material)
}

// ---------------------------------------------------------------------------
// The presentation link (the sales-reply binding)
// ---------------------------------------------------------------------------

// PresentedCommitment is the binding pair of the presenting client
// reply: both values or neither (the nullable pair discipline of the
// table CHECK — the sales reply carries the commercial material of the
// EXACT shown version, AC-6).
type PresentedCommitment struct {
	CommitmentID id.UUID
	VersionNo    int64
}

// GuardPresentedCommitment validates the pair shape: both values set or
// both empty (the app layer decides whether the reply class permits the
// binding at all — the sales class only).
func GuardPresentedCommitment(link PresentedCommitment) error {
	if link.CommitmentID == id.UUID("") && link.VersionNo == 0 {
		return nil
	}
	if link.CommitmentID == id.UUID("") || link.VersionNo == 0 {
		return fmt.Errorf("связь обязательства — пара: идентификатор и версия задаются вместе")
	}
	return nil
}

// ---------------------------------------------------------------------------
// The outbox payloads (the safe canonical JSON — the material never
// leaves the row)
// ---------------------------------------------------------------------------

// Outbox event types of the commitment surface (the 2.1 bounded
// vocabulary).
const (
	OutboxEventCommitmentVersionCreated = "case.commitment_version_created"
	OutboxEventCommitmentConfirmed      = "case.commitment_confirmed"
	OutboxEventCommitmentDeclined       = "case.commitment_declined"
	OutboxEventCommitmentV1             = 1
)

// CommitmentVersionCreatedPayload is the safe canonical JSON of the
// version event: the identifiers, the version number, the status word
// and the dictionary words — the MATERIAL CONDITIONS never enter the
// envelope (the «кандидат не покидает строку» precedent).
type CommitmentVersionCreatedPayload struct {
	TenantID     id.UUID   `json:"tenant_id"`
	CaseID       id.UUID   `json:"case_id"`
	WorkItemID   id.UUID   `json:"work_item_id"`
	CommitmentID id.UUID   `json:"commitment_id"`
	VersionNo    int64     `json:"version_no"`
	Status       string    `json:"status"`
	ActorID      id.UUID   `json:"actor_id"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// CommitmentConfirmedPayload is the safe canonical JSON of the
// confirmation event: the identifiers and the snapshot numbers only.
type CommitmentConfirmedPayload struct {
	TenantID       id.UUID   `json:"tenant_id"`
	CaseID         id.UUID   `json:"case_id"`
	WorkItemID     id.UUID   `json:"work_item_id"`
	CommitmentID   id.UUID   `json:"commitment_id"`
	VersionNo      int64     `json:"version_no"`
	ConfirmationID id.UUID   `json:"confirmation_id"`
	SubjectKind    string    `json:"subject_kind"`
	SubjectRef     string    `json:"subject_ref"`
	ActorID        id.UUID   `json:"actor_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// CommitmentDeclinedPayload is the safe canonical JSON of the decline
// event: the identifiers, the version and the outcome word — the span
// excerpts never enter the envelope.
type CommitmentDeclinedPayload struct {
	TenantID     id.UUID   `json:"tenant_id"`
	CaseID       id.UUID   `json:"case_id"`
	WorkItemID   id.UUID   `json:"work_item_id"`
	CommitmentID id.UUID   `json:"commitment_id"`
	VersionNo    int64     `json:"version_no"`
	Outcome      string    `json:"outcome"`
	ActorID      id.UUID   `json:"actor_id"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// DedupKeyCommitmentVersionCreated builds the dedup key of one version event.
func DedupKeyCommitmentVersionCreated(commitmentID id.UUID, versionNo int64) string {
	return fmt.Sprintf("case.commitment_version_created:%s:%d", commitmentID, versionNo)
}

// DedupKeyCommitmentConfirmed builds the dedup key of one confirmation event.
func DedupKeyCommitmentConfirmed(confirmationID id.UUID) string {
	return "case.commitment_confirmed:" + confirmationID.String()
}

// DedupKeyCommitmentDeclined builds the dedup key of one decline event.
func DedupKeyCommitmentDeclined(declineID id.UUID) string {
	return "case.commitment_declined:" + declineID.String()
}

// CommitmentVersionCreatedPayloadJSON renders the safe canonical JSON.
func CommitmentVersionCreatedPayloadJSON(payload CommitmentVersionCreatedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// CommitmentConfirmedPayloadJSON renders the safe canonical JSON.
func CommitmentConfirmedPayloadJSON(payload CommitmentConfirmedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// CommitmentDeclinedPayloadJSON renders the safe canonical JSON.
func CommitmentDeclinedPayloadJSON(payload CommitmentDeclinedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
