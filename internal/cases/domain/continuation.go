package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The continuation surface (Story 2.10, first slice): the purpose-bound
// link dictionaries, the issuance guards, the read-time state derivation
// and the opaque capability-token codec
// ---------------------------------------------------------------------------
//
// The protected continuation is a purpose-bound link the operator weaves
// into the outgoing sales reply: the client opens the protected surface
// and corrects the migrated fact without a session (the token IS the
// credential — the short-lived one-use signed playback permit precedent
// of the architecture spine: a request-carried credential). The link
// carries the pins (case, work item, participant, owner epoch), the
// purpose set, the open question and the explanation of the transition;
// the material lifetime bounds are fixed: the ttl of 5..120 minutes and
// the consumption limit of 1..10 accepted fact corrections (the default
// 1 is the «exactly one submission per request» of FR-16).
//
// The token is OPAQUE: base64url(tenant_id 16 bytes || random 32 bytes).
// It carries NO case, work item or participant material — the tenant
// part is the scope candidate the hash verifies BEFORE any case
// material is read (the AD-10 honesty: the scope comes from the stored
// row, never from an unverified payload; a wrong tenant simply finds no
// hash row under the RLS scope — the generic refusal without disclosure).
// The random 256 bits make guessing impractical; the database stores
// ONLY the SHA-256 of the random part — the full token never enters the
// storage, the outbox, the logs or the exceptions.
//
// The link state is NEVER stored: DeriveContinuationLinkState reads the
// current material (the expiry, the revocation row, the accepted
// corrections count, the current owner epoch) and returns the honest
// word in the spec order — expired, revoked, consumed, owner_stale,
// valid. unknown is the lookup-layer word of the dictionary: the
// derivation never produces it (a token that finds no row has no
// material to derive from). The revoked/consumed/owner_stale words have
// honest producers of this slice; a dead link never resurrects — the
// old URL keeps failing no matter how often it is re-requested.

// ---------------------------------------------------------------------------
// Dictionaries
// ---------------------------------------------------------------------------

// The link purposes v1: the fact correction is the only allowed
// operation of the protected surface of this slice (the media purposes
// of the later slices arrive with the media edge).
const ContinuationPurposeFactCorrection = "fact_correction"

// ContinuationPurposes is the full dictionary.
var ContinuationPurposes = []string{ContinuationPurposeFactCorrection}

// The link use kinds: an accepted fact correction consumes the link
// limit; the vehicle confirmation of Story 4.2 consumes it too (a data
// submission — the client confirms the EXACT versions of the
// identification attributes); the human request and the return to the
// conversation are events, not data submissions — they never consume
// anything.
const (
	ContinuationUseFactCorrectionSubmitted = "fact_correction_submitted"
	ContinuationUseHumanRequested          = "human_requested"
	ContinuationUseReturnedToConversation  = "returned_to_conversation"
	// ContinuationUseVehicleConfirmationSubmitted is the +1 word of Story
	// 4.2: the client confirmation of the vehicle identification
	// attributes through the protected continuation (the SQL mirror of
	// the dictionary extends with migrations/cases/00018 — extending it
	// is a new spec version; this spec is that version).
	ContinuationUseVehicleConfirmationSubmitted = "vehicle_confirmation_submitted"
)

// ContinuationUseKinds is the full dictionary.
var ContinuationUseKinds = []string{
	ContinuationUseFactCorrectionSubmitted,
	ContinuationUseHumanRequested,
	ContinuationUseReturnedToConversation,
	ContinuationUseVehicleConfirmationSubmitted,
}

// ContinuationUseConsumesLimit answers whether one use kind consumes
// the link limit: the data submissions do (the fact correction and the
// vehicle confirmation of 4.2), the free events never do (the human
// request and the return). Every counting site (the store lookup, the
// app panel, the surface) reads THIS word — the limit accounting
// lives in one dictionary place.
func ContinuationUseConsumesLimit(useKind string) bool {
	return useKind == ContinuationUseFactCorrectionSubmitted || useKind == ContinuationUseVehicleConfirmationSubmitted
}

// FactChannelProtectedContinuation is the +1 word of the FactChannels
// dictionary of 2.2: the provenance channel of the client-submitted
// fact corrections (the FR-82 modality/source trace of the protected
// continuation, the text modality of this slice).
const FactChannelProtectedContinuation = "protected_continuation"

// ---------------------------------------------------------------------------
// The fixed material bounds (the domain guards carry them, the table
// CHECKs mirror them)
// ---------------------------------------------------------------------------

// The link lifetime bounds in minutes: short-lived by design (the
// protected continuation is a transition surface, not a standing
// account).
const (
	ContinuationTTLMinMinutes = 5
	ContinuationTTLMaxMinutes = 120
)

// The consumption bounds: how many accepted fact corrections one link
// may carry. The default is 1 — the FR-16 «exactly one submission per
// request»; the operator may raise it to 10 for the multi-question
// scenarios of the later slices.
const (
	ContinuationLimitMin = 1
	ContinuationLimitMax = 10
)

// DefaultContinuationConsumptionLimit is the honest default of the
// issuance command: one accepted submission, one link.
const DefaultContinuationConsumptionLimit = 1

// The open question and the transition explanation bounds (runes).
const (
	MaxContinuationQuestionRunes    = 500
	MaxContinuationExplanationRunes = 1000
)

// ---------------------------------------------------------------------------
// The link state words (the read-time derivation dictionary)
// ---------------------------------------------------------------------------

// The link state dictionary: expired (the ttl passed), revoked (the
// revocation row exists), consumed (the accepted corrections reached
// the limit), owner_stale (the case owner epoch moved past the pin),
// valid. unknown is the lookup-layer word: no row found for the token —
// the derivation never returns it (nothing to derive from).
const (
	ContinuationStateValid      = "valid"
	ContinuationStateExpired    = "expired"
	ContinuationStateRevoked    = "revoked"
	ContinuationStateConsumed   = "consumed"
	ContinuationStateOwnerStale = "owner_stale"
	ContinuationStateUnknown    = "unknown"
)

// ContinuationStateWords is the full dictionary.
var ContinuationStateWords = []string{
	ContinuationStateValid, ContinuationStateExpired, ContinuationStateRevoked,
	ContinuationStateConsumed, ContinuationStateOwnerStale, ContinuationStateUnknown,
}

// ---------------------------------------------------------------------------
// The issuance guards
// ---------------------------------------------------------------------------

// GuardContinuationPurposes validates the purpose set of the link: a
// non-empty duplicate-free subset of the dictionary (a purpose word
// outside the dictionary is the honest refusal — the dictionary is
// never a tenant configuration).
func GuardContinuationPurposes(purposes []string) error {
	if len(purposes) == 0 {
		return fmt.Errorf("набор целей ссылки не может быть пустым (словарь: %s)", strings.Join(ContinuationPurposes, ", "))
	}
	seen := make(map[string]bool, len(purposes))
	for _, purpose := range purposes {
		if !inDictionary(ContinuationPurposes, purpose) {
			return fmt.Errorf("цель ссылки «%s» не входит в словарь (%s)", purpose, strings.Join(ContinuationPurposes, ", "))
		}
		if seen[purpose] {
			return fmt.Errorf("набор целей ссылки не может содержать повторы: «%s»", purpose)
		}
		seen[purpose] = true
	}
	return nil
}

// GuardContinuationTTL validates the link lifetime in minutes: the
// bounds of the dictionary (5..120) keep the surface short-lived.
func GuardContinuationTTL(ttlMinutes int) error {
	if ttlMinutes < ContinuationTTLMinMinutes || ttlMinutes > ContinuationTTLMaxMinutes {
		return fmt.Errorf("срок жизни ссылки обязан быть %d..%d минут, получено %d",
			ContinuationTTLMinMinutes, ContinuationTTLMaxMinutes, ttlMinutes)
	}
	return nil
}

// GuardContinuationConsumptionLimit validates the accepted-corrections
// bound of the link (the same 1..10 discipline as the commitment
// consumption limit of 2.9).
func GuardContinuationConsumptionLimit(limit int) error {
	if limit < ContinuationLimitMin || limit > ContinuationLimitMax {
		return fmt.Errorf("лимит отправок ссылки обязан быть %d..%d, получено %d",
			ContinuationLimitMin, ContinuationLimitMax, limit)
	}
	return nil
}

// GuardContinuationTexts validates the two operator texts of the link:
// the open question (1..500 runes) the client answers on the surface,
// and the transition explanation (1..1000 runes) — «the transition
// explains the goal» of AC-1.
func GuardContinuationTexts(question, explanation string) error {
	questionRunes := utf8.RuneCountInString(question)
	if questionRunes < 1 || questionRunes > MaxContinuationQuestionRunes {
		return fmt.Errorf("открытый вопрос ссылки обязан быть 1..%d рун, получено %d",
			MaxContinuationQuestionRunes, questionRunes)
	}
	explanationRunes := utf8.RuneCountInString(explanation)
	if explanationRunes < 1 || explanationRunes > MaxContinuationExplanationRunes {
		return fmt.Errorf("объяснение перехода обязано быть 1..%d рун, получено %d",
			MaxContinuationExplanationRunes, explanationRunes)
	}
	return nil
}

// ---------------------------------------------------------------------------
// The link state derivation (read-time, never stored)
// ---------------------------------------------------------------------------

// ContinuationLinkStateInput is the material of the read-time
// derivation: the current moment, the link expiry, the revocation
// existence, the accepted fact corrections count against the link
// limit, and the pinned/current owner epoch pair of the case.
type ContinuationLinkStateInput struct {
	Now                 time.Time
	ExpiresAt           time.Time
	Revoked             bool
	AcceptedCorrections int
	ConsumptionLimit    int
	OwnerEpochPinned    int64
	OwnerEpochCurrent   int64
}

// DeriveContinuationLinkState derives the honest state word of one link
// in the spec order: expired when the ttl passed; revoked when the
// revocation row exists; consumed when the accepted corrections reached
// the limit; owner_stale when the case owner epoch moved past the pin;
// valid otherwise. The states are never stored — a dead link never
// resurrects, and a replayed old request changes nothing (the
// DeriveConfirmationValidity precedent of 2.9). The unknown word of the
// dictionary is the lookup-layer refusal of a token that finds no row:
// this derivation never returns it.
func DeriveContinuationLinkState(input ContinuationLinkStateInput) string {
	switch {
	case input.Now.After(input.ExpiresAt):
		return ContinuationStateExpired
	case input.Revoked:
		return ContinuationStateRevoked
	case input.AcceptedCorrections >= input.ConsumptionLimit:
		return ContinuationStateConsumed
	case input.OwnerEpochCurrent != input.OwnerEpochPinned:
		return ContinuationStateOwnerStale
	default:
		return ContinuationStateValid
	}
}

// ---------------------------------------------------------------------------
// The opaque capability-token codec (v1)
// ---------------------------------------------------------------------------

// The token layout: the 16 canonical tenant bytes followed by the 32
// random bytes, base64url without padding (64 characters). The token
// carries NO case, work item or participant material — the tenant part
// is the scope candidate the stored hash verifies under the RLS scope
// before any material is read (the AD-10 honesty); the random part is
// the unguessable secret whose SHA-256 is the only stored trace.
const (
	continuationTokenTenantBytes = 16
	continuationTokenRandomBytes = 32
	continuationTokenTotalBytes  = continuationTokenTenantBytes + continuationTokenRandomBytes
)

// RandomContinuationSecret generates the random half of the token (32
// crypto-random bytes — the NewV7/commitment-key entropy discipline).
func RandomContinuationSecret() ([]byte, error) {
	random := make([]byte, continuationTokenRandomBytes)
	if _, err := rand.Read(random); err != nil {
		return nil, fmt.Errorf("generate continuation token entropy: %w", err)
	}
	return random, nil
}

// ContinuationTokenHash computes the stored half: the SHA-256 of the
// random part (32 bytes). The full token NEVER enters the storage —
// the hash cannot be turned back into the URL (the preimage stays with
// the client who received the link).
func ContinuationTokenHash(random []byte) ([]byte, error) {
	if len(random) != continuationTokenRandomBytes {
		return nil, fmt.Errorf("хеш токена ссылки принимает ровно %d случайных байт, получено %d",
			continuationTokenRandomBytes, len(random))
	}
	sum := sha256.Sum256(random)
	return sum[:], nil
}

// EncodeContinuationToken renders the opaque URL token of the link:
// base64url(tenant 16 bytes || random 32 bytes), no padding. The tenant
// identifier must be a canonical UUID (the scope candidate); the random
// half must carry the full 32 bytes.
func EncodeContinuationToken(tenantID id.UUID, random []byte) (string, error) {
	if len(random) != continuationTokenRandomBytes {
		return "", fmt.Errorf("токен ссылки принимает ровно %d случайных байт, получено %d",
			continuationTokenRandomBytes, len(random))
	}
	tenant, err := continuationTenantBytes(tenantID)
	if err != nil {
		return "", err
	}
	raw := make([]byte, 0, continuationTokenTotalBytes)
	raw = append(raw, tenant...)
	raw = append(raw, random...)
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// ParseContinuationToken decodes the URL token into the scope candidate
// (the tenant) and the random half. The strictness is fail-closed: a
// token of the wrong length, a non-base64url alphabet or a tenant part
// that is not a valid UUIDv7 is the honest generic refusal — no case
// material is ever touched. The parse carries NOTHING else: the case,
// the work item and the participant live in the stored row the hash
// lookup finds (the opaque invariant).
func ParseContinuationToken(token string) (id.UUID, []byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", nil, fmt.Errorf("токен ссылки не декодируется: %w", err)
	}
	if len(raw) != continuationTokenTotalBytes {
		return "", nil, fmt.Errorf("токен ссылки обязан нести %d байт, получено %d",
			continuationTokenTotalBytes, len(raw))
	}
	parsed, err := id.Parse(continuationTenantOf(raw[:continuationTokenTenantBytes]))
	if err != nil {
		return "", nil, fmt.Errorf("токен ссылки: часть тенанта не является валидным UUID: %w", err)
	}
	random := make([]byte, continuationTokenRandomBytes)
	copy(random, raw[continuationTokenTenantBytes:])
	return parsed, random, nil
}

// continuationTenantBytes renders the canonical 16 tenant bytes of the
// scope candidate.
func continuationTenantBytes(tenantID id.UUID) ([]byte, error) {
	value := tenantID.String()
	if !tenantID.Valid() || len(value) != 36 {
		return nil, fmt.Errorf("токен ссылки: идентификатор тенанта обязан быть каноническим UUID, получено %q", value)
	}
	compact := value[0:8] + value[9:13] + value[14:18] + value[19:23] + value[24:36]
	raw, err := hex.DecodeString(compact)
	if err != nil || len(raw) != continuationTokenTenantBytes {
		return nil, fmt.Errorf("токен ссылки: идентификатор тенанта не декодируется: %q", value)
	}
	return raw, nil
}

// continuationTenantOf formats the canonical UUID string of the tenant
// bytes (the hex with the 8-4-4-4-12 dashes of the platform format).
func continuationTenantOf(raw []byte) string {
	compact := hex.EncodeToString(raw)
	return compact[0:8] + "-" + compact[8:12] + "-" + compact[12:16] + "-" + compact[16:20] + "-" + compact[20:32]
}

// ---------------------------------------------------------------------------
// The row shapes (the store reads/writes; the states are derived, never
// stored)
// ---------------------------------------------------------------------------

// ContinuationLink is one append-only link row: the relation pins (the
// case, the work item, the participant reference, the response owner
// epoch of the moment), the purpose set, the two operator texts, the
// lifetime bounds and the token hash — the ONLY stored trace of the
// capability (the preimage is not recoverable).
type ContinuationLink struct {
	TenantID         id.UUID
	CaseID           id.UUID
	WorkItemID       id.UUID
	LinkID           id.UUID
	ParticipantRef   string
	OwnerEpoch       int64
	Purposes         []string
	Question         string
	Explanation      string
	TTLMinutes       int
	IssuedAt         time.Time
	ExpiresAt        time.Time
	ConsumptionLimit int
	TokenHash        []byte
	ActorID          id.UUID
	CreatedAt        time.Time
}

// ContinuationLinkUse is one append-only client-action row: an accepted
// fact correction (the material pair — the corrected fact with its new
// version number — consumes the link limit), a human request or a
// return to the conversation (events, no material, no consumption).
type ContinuationLinkUse struct {
	TenantID      id.UUID
	CaseID        id.UUID
	WorkItemID    id.UUID
	LinkID        id.UUID
	UseID         id.UUID
	UseKind       string
	FactID        id.UUID
	FactVersionNo int64
	SubmittedAt   time.Time
}

// ContinuationRevocation is one append-only revocation row: the honest
// operator «прекратить ожидание» — the row mutates nothing, the
// read-time derivation reads it and blocks the surface without
// disclosure.
type ContinuationRevocation struct {
	TenantID     id.UUID
	CaseID       id.UUID
	WorkItemID   id.UUID
	LinkID       id.UUID
	RevocationID id.UUID
	Reason       string
	ActorID      id.UUID
	CreatedAt    time.Time
}

// ContinuationLinkLookup is the public-surface lookup answer: the link
// row the verified hash found, its uses, the revocation existence, the
// CURRENT owner epoch of the case and the derived state word (the
// read-time truth of the moment — the derivation input, never a stored
// status). The token itself is never part of the answer: the hash is
// the lookup key, the row is the scope.
type ContinuationLinkLookup struct {
	Link              ContinuationLink
	Uses              []ContinuationLinkUse
	Revoked           bool
	OwnerEpochCurrent int64
	State             string
}

// ---------------------------------------------------------------------------
// The honest refusal texts of the surface
// ---------------------------------------------------------------------------

// TerminalWorkItemContinuationText is the honest refusal of the link
// issuance on a closed work item (the write-command boundary of 2.2 —
// the continuation never extends a finished element).
const TerminalWorkItemContinuationText = "Терминальный элемент работы не принимает ссылок продолжения: resolved и merged — финальные состояния; для merged создавайте ссылку на целевом элементе работы."

// ContinuationAlreadyRevokedText is the honest refusal of the second
// revocation of the same link (the UNIQUE backstop of the table — the
// expectation ended once, the append-only history says so).
const ContinuationAlreadyRevokedText = "Ссылка продолжения уже отозвана: ожидание прекращено; при новом заходе выпустите новую ссылку."

// ContinuationNotUsableText is the honest refusal of the client action
// on a non-valid link (the store-side reason of the generic public
// refusal — the page never distinguishes the causes to the client; the
// exact state word stays the operator panel material).
const ContinuationNotUsableText = "Ссылка продолжения недоступна для действий: срок истёк, ссылка отозвана, лимит отправок исчерпан или владелец обращения изменился — клиенту показывается единая страница отказа без различения причин."

// ContinuationFactNotCorrectableText is the honest refusal of the
// correction against a fact that is not a CONFIRMED fact of the work
// item of the link (the surface offers the confirmed facts only —
// another work item, another participant or an unverified draft is not
// correctable here).
const ContinuationFactNotCorrectableText = "Исправлять можно только подтверждённый факт элемента работы этой ссылки: выберите факт из списка подтверждённых на странице продолжения."

// ---------------------------------------------------------------------------
// The outbox payloads (the safe canonical JSON — the token, the texts
// and the fact values never leave the row)
// ---------------------------------------------------------------------------

// Outbox event types of the continuation surface (the 2.1 bounded
// vocabulary; the revocation has NO event — the read-time derivation
// of the panel reads the row, the state is never an envelope).
const (
	OutboxEventContinuationLinkIssued     = "case.continuation_link_issued"
	OutboxEventContinuationSubmitted      = "case.continuation_submitted"
	OutboxEventContinuationHumanRequested = "case.continuation_human_requested"
	OutboxEventContinuationReturned       = "case.continuation_returned"
	OutboxEventContinuationV1             = 1
)

// ContinuationLinkIssuedPayload is the safe canonical JSON of the issue
// event: the identifiers, the purpose words and the expiry — the TOKEN
// and the question/explanation texts never enter the envelope (the
// token exists only in the command answer and on the operator side).
type ContinuationLinkIssuedPayload struct {
	TenantID   id.UUID   `json:"tenant_id"`
	CaseID     id.UUID   `json:"case_id"`
	WorkItemID id.UUID   `json:"work_item_id"`
	LinkID     id.UUID   `json:"link_id"`
	Purposes   []string  `json:"purposes"`
	ExpiresAt  time.Time `json:"expires_at"`
	ActorID    id.UUID   `json:"actor_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ContinuationSubmittedPayload is the safe canonical JSON of the
// accepted fact correction: the identifiers, the use kind and the new
// version number — the corrected VALUE never enters the envelope (the
// «кандидат не покидает строку» precedent of 2.6/2.7/2.8).
type ContinuationSubmittedPayload struct {
	TenantID      id.UUID   `json:"tenant_id"`
	CaseID        id.UUID   `json:"case_id"`
	WorkItemID    id.UUID   `json:"work_item_id"`
	LinkID        id.UUID   `json:"link_id"`
	UseID         id.UUID   `json:"use_id"`
	FactID        id.UUID   `json:"fact_id"`
	FactVersionNo int64     `json:"fact_version_no"`
	ActorID       id.UUID   `json:"actor_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// ContinuationEventPayload is the safe canonical JSON of the human
// request and the return events: the identifiers and the use kind — no
// material, no texts.
type ContinuationEventPayload struct {
	TenantID   id.UUID   `json:"tenant_id"`
	CaseID     id.UUID   `json:"case_id"`
	WorkItemID id.UUID   `json:"work_item_id"`
	LinkID     id.UUID   `json:"link_id"`
	UseID      id.UUID   `json:"use_id"`
	UseKind    string    `json:"use_kind"`
	ActorID    id.UUID   `json:"actor_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// DedupKeyContinuationLinkIssued builds the dedup key of one issue event.
func DedupKeyContinuationLinkIssued(linkID id.UUID) string {
	return "case.continuation_link_issued:" + linkID.String()
}

// DedupKeyContinuationSubmitted builds the dedup key of one correction event.
func DedupKeyContinuationSubmitted(useID id.UUID) string {
	return "case.continuation_submitted:" + useID.String()
}

// DedupKeyContinuationEvent builds the dedup key of one human/return event.
func DedupKeyContinuationEvent(useID id.UUID) string {
	return "case.continuation_event:" + useID.String()
}

// ContinuationLinkIssuedPayloadJSON renders the safe canonical JSON.
func ContinuationLinkIssuedPayloadJSON(payload ContinuationLinkIssuedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// ContinuationSubmittedPayloadJSON renders the safe canonical JSON.
func ContinuationSubmittedPayloadJSON(payload ContinuationSubmittedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// ContinuationEventPayloadJSON renders the safe canonical JSON.
func ContinuationEventPayloadJSON(payload ContinuationEventPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// ---------------------------------------------------------------------------
// The correction provenance (the FR-82 source trace of the text
// modality)
// ---------------------------------------------------------------------------

// ContinuationCorrectionSourceType is the writer class of the
// correction fact version: the machine surface record. The correction
// is NOT an operator command (no command key) and NOT a channel event
// (the public page is not an ingress delivery) — the client material
// flows through the protected continuation page, the server records it
// verbatim as an UNVERIFIED new version. The channel word
// protected_continuation carries the true origin story (the
// pseudo-channel of the client-submitted corrections, the +1 word of
// the FactChannels dictionary); the source ref is the link id (the
// capability that authorized the write). The reserved system word of
// the 2.2 source-type dictionary finds its first producer here — the
// write path arrived with its own story, exactly as the 2.2 comment
// promised (the domain record_fact guard still refuses it for the
// operator path: the correction has its own gate, never GuardRecordFact).
const ContinuationCorrectionSourceType = "system"
