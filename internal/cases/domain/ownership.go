package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The single response owner and the writing inbox (Story 2.4, first slice)
// ---------------------------------------------------------------------------
//
// The ownership of a case is an append-only term history: one row per
// ownership period with the material uniqueness (tenant, case, epoch). The
// CURRENT owner, the pending transfer and the reply state are read-time
// derivations — nothing is ever updated. A handoff ask is one transfer
// request with the deadline taken from the human coverage of the acting
// route decision (never a command parameter); its resolution is one
// append-only row; its expiry is a time derivation that voids the request
// for every later command. The client reply is the only client-bound
// outgoing path: the row is the full snapshot of the send (the owner, the
// epoch, the surface, the route and the release lineage), the content
// never leaves it, and the execute-time revalidation of the drain writes
// the append-only outcome — dispatched or stale_needs_review.

// ---------------------------------------------------------------------------
// Dictionaries
// ---------------------------------------------------------------------------

// Owner kinds: a human participant of the tenant or the Staffy employee of
// the case. A queue, a department or an unconfirmed candidate is NEVER an
// owner — the owner_ref must resolve into a member or the case employee.
const (
	OwnerKindHuman  = "human"
	OwnerKindStaffy = "staffy"
)

// OwnerKinds is the full dictionary.
var OwnerKinds = []string{OwnerKindHuman, OwnerKindStaffy}

// Writable surfaces: the proven minimal Staffy operator surface (the 2-1
// precedent, OQ-17) and the external human Inbox — the latter only with the
// acting account-scoped write/ownership capability evidence (the product
// name is never a proof). The inbox value reuses the 00001 dictionary
// constant of the creation snapshot.
const (
	WritableSurfaceInboxV2       = WritableSurfaceInbox
	WritableSurfaceExternalInbox = "external_inbox"
)

// WritableSurfaces is the full dictionary.
var WritableSurfaces = []string{WritableSurfaceInboxV2, WritableSurfaceExternalInbox}

// Term origins: the ingest initial term, the take of a free case, the
// acknowledged transfer and the supervision reassignment.
const (
	OwnershipOriginIngestInitial       = "ingest_initial"
	OwnershipOriginTake                = "take"
	OwnershipOriginTransferAcknowledge = "transfer_acknowledge"
	OwnershipOriginReassign            = "reassign"
)

// OwnershipOrigins is the full dictionary.
var OwnershipOrigins = []string{OwnershipOriginIngestInitial, OwnershipOriginTake, OwnershipOriginTransferAcknowledge, OwnershipOriginReassign}

// Transfer resolution kinds: acknowledged (the target takes the ownership),
// declined (the target refuses) and cancelled (the author withdraws).
const (
	TransferResolutionAcknowledged = "acknowledged"
	TransferResolutionDeclined     = "declined"
	TransferResolutionCancelled    = "cancelled"
)

// TransferResolutions is the full dictionary.
var TransferResolutions = []string{TransferResolutionAcknowledged, TransferResolutionDeclined, TransferResolutionCancelled}

// Client reply outcomes of the execute-time revalidation: the synthetic
// dispatch record (the real channel delivery is the deferred outgoing
// story) and the stale verdict (the payload is never relayed to the new
// owner automatically).
const (
	ReplyOutcomeDispatched       = "dispatched"
	ReplyOutcomeStaleNeedsReview = "stale_needs_review"
)

// ReplyOutcomes is the full dictionary.
var ReplyOutcomes = []string{ReplyOutcomeDispatched, ReplyOutcomeStaleNeedsReview}

// The ownership health derivations.
const (
	OwnershipHealthStable           = "stable"
	OwnershipHealthTransferPending  = "transfer_pending"
	OwnershipHealthTransferTimedOut = "transfer_timed_out"
	OwnershipHealthReassignment     = "reassignment_required"
)

// The material limits of the ownership dictionary.
const (
	MaxOwnershipReasonRunes = 200
	MaxReplyContentRunes    = 2000
)

// The drain batch bound of the reply sweep (the DrainOutbox precedent).
const ClientReplyDrainLimit = 500

// ---------------------------------------------------------------------------
// The ownership material
// ---------------------------------------------------------------------------

// OwnershipTerm is one append-only ownership period of a case: the epoch
// (the monotonic CAS counter), the owner (kind + ref), the writable
// surface of the period, the origin, the previous owner snapshot, the
// actor, the honest reason, the request it answered (the acknowledged
// transfer) and both times.
type OwnershipTerm struct {
	TenantID          id.UUID
	CaseID            id.UUID
	TermID            id.UUID
	Epoch             int64
	OwnerKind         string
	OwnerRef          id.UUID
	WritableSurface   string
	Origin            string
	PreviousOwnerKind string
	PreviousOwnerRef  id.UUID
	ActorID           id.UUID
	Reason            string
	TransferRequestID id.UUID
	RequestedAt       time.Time
	AcknowledgedAt    time.Time
	CreatedAt         time.Time
}

// TransferRequest is one assigned handoff ask: the from/to owners, the
// surface the target would own, the honest reason, the author, the deadline
// from the human coverage and the request number (monotonic per case).
type TransferRequest struct {
	TenantID        id.UUID
	CaseID          id.UUID
	RequestID       id.UUID
	RequestNo       int64
	FromOwnerKind   string
	FromOwnerRef    id.UUID
	ToOwnerKind     string
	ToOwnerRef      id.UUID
	WritableSurface string
	Reason          string
	RequestedBy     id.UUID
	DeadlineAt      time.Time
	CreatedAt       time.Time
}

// TransferResolution is one append-only decision of a request: the kind,
// the resolver, the resulting epoch (the acknowledge only) and the reason.
type TransferResolution struct {
	TenantID       id.UUID
	ResolutionID   id.UUID
	RequestID      id.UUID
	Kind           string
	ActorID        id.UUID
	ResultingEpoch int64
	Reason         string
	CreatedAt      time.Time
}

// ClientReply is the single client-bound outgoing record of the slice: the
// governing case, the conversation, the owner/epoch/surface snapshot, the
// route snapshot of the conversation, the release lineage and the content.
// The state is a derivation — the absence of an outcome is `prepared`.
type ClientReply struct {
	TenantID        id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	ReplyID         id.UUID
	OwnerKind       string
	OwnerRef        id.UUID
	Epoch           int64
	WritableSurface string
	ConnectionID    id.UUID
	Provider        string
	Channel         string
	Direction       string
	EmployeeID      id.UUID
	DecisionNo      int64
	ProfileVersion  int64
	Content         string
	ActorID         id.UUID
	// The Story 2.5 material: the producer's reply class mark and the
	// Work Item anchor of the mode gate (the honest NULL/absent word of
	// the pre-migration rows).
	ReplyClass string
	WorkItemID id.UUID
	// The Story 2.6 material: the Policy Outcome the sales reply passed
	// through (the zero id of the pre-migration and non-sales rows).
	PolicyOutcomeID id.UUID
	CreatedAt       time.Time
}

// ClientReplyOutcome is the append-only execute-time revalidation verdict
// of one reply: dispatched (the synthetic contour record) or
// stale_needs_review (the ownership moved — the payload stays put).
type ClientReplyOutcome struct {
	TenantID  id.UUID
	OutcomeID id.UUID
	ReplyID   id.UUID
	Kind      string
	CreatedAt time.Time
}

// ---------------------------------------------------------------------------
// Read-time derivations
// ---------------------------------------------------------------------------

// OwnershipState is the derived ownership state of one case: the current
// term (the maximal epoch), the pending transfer (the latest request
// without a resolution, unexpired), the owner availability word, the
// backup owner and the ack deadline of the human coverage, and the health
// derivation. Everything is a read — nothing is stored.
type OwnershipState struct {
	Current            *OwnershipTerm
	Pending            *TransferRequest
	PendingTimedOut    bool
	OwnerActive        bool
	Health             string
	BackupOwnerID      id.UUID
	AckDeadlineMinutes int64
}

// CurrentOwnership resolves the term of the maximal epoch (nil when the
// case has no terms — the honest pre-migration state).
func CurrentOwnership(terms []OwnershipTerm) *OwnershipTerm {
	var current *OwnershipTerm
	for i := range terms {
		if current == nil || terms[i].Epoch > current.Epoch {
			current = &terms[i]
		}
	}
	return current
}

// resolutionOf resolves the resolution of one request (nil when the
// request is still open).
func resolutionOf(resolutions []TransferResolution, requestID id.UUID) *TransferResolution {
	for i := range resolutions {
		if resolutions[i].RequestID == requestID {
			return &resolutions[i]
		}
	}
	return nil
}

// PendingTransfer resolves the latest unresolved request the ownership
// has not moved past: the assigned handoff ask of the case. The ask is
// void when it was created STRICTLY BEFORE the current term — an
// acknowledged transfer, a take or a reassign already changed the
// ownership, so the ask targeted a gone state and must never keep the
// case takeable or misreport the health word (the R1 zombie lesson). An
// ask at the term's own timestamp was made against that term (the store
// commands are strictly later than the ask by the deadline math; the
// deterministic clocks of the fixtures share the instant).
func PendingTransfer(terms []OwnershipTerm, requests []TransferRequest, resolutions []TransferResolution, now time.Time) *TransferRequest {
	watermark := time.Time{}
	if current := CurrentOwnership(terms); current != nil {
		watermark = current.CreatedAt
	}
	var pending *TransferRequest
	for i := range requests {
		if resolutionOf(resolutions, requests[i].RequestID) != nil {
			continue
		}
		if pending == nil || requests[i].CreatedAt.After(pending.CreatedAt) {
			pending = &requests[i]
		}
	}
	if pending == nil || pending.CreatedAt.Before(watermark) {
		return nil
	}
	return pending
}

// ActiveTransferRequest resolves the LIVE assigned ask: the latest
// unresolved request still inside its deadline. An expired unresolved
// request is voided by the derivation — it never blocks the new asks (the
// honest «повторный запрос — новая строка» word of the matrix); a request
// the ownership already moved past is void for every word.
func ActiveTransferRequest(terms []OwnershipTerm, requests []TransferRequest, resolutions []TransferResolution, now time.Time) *TransferRequest {
	pending := PendingTransfer(terms, requests, resolutions, now)
	if pending == nil || !now.Before(pending.DeadlineAt) {
		return nil
	}
	return pending
}

// DeriveOwnershipState derives the full ownership state of one case: the
// current term, the pending transfer with its timeout word, the health
// derivation and the coverage words (the backup owner and the ack deadline
// come from the RouteFacts re-read of the caller — the coverage of the
// acting decision).
func DeriveOwnershipState(terms []OwnershipTerm, requests []TransferRequest, resolutions []TransferResolution, ownerActive bool, now time.Time) OwnershipState {
	state := OwnershipState{OwnerActive: ownerActive, Health: OwnershipHealthStable}
	state.Current = CurrentOwnership(terms)
	if pending := PendingTransfer(terms, requests, resolutions, now); pending != nil {
		state.Pending = pending
		state.PendingTimedOut = !now.Before(pending.DeadlineAt)
	}
	switch {
	case state.Current == nil:
		state.Health = OwnershipHealthStable // the honest pre-migration word: no terms, no derived state
	case !ownerActive && state.Current.OwnerKind == OwnerKindHuman:
		state.Health = OwnershipHealthReassignment
	case state.Pending != nil && state.PendingTimedOut:
		state.Health = OwnershipHealthTransferTimedOut
	case state.Pending != nil:
		state.Health = OwnershipHealthTransferPending
	}
	return state
}

// Takeable reports whether the take is allowed in this state: the deadline
// of the assigned request expired without a decision, or the owner is
// unavailable. A healthy ownership is never takeable — the honest refusal
// names the transfer path (AC-9). The take itself voids the expired ask
// with the cancel resolution of the same transaction, and every earlier
// unresolved ask is void by the term watermark — after a successful take
// the ownership is healthy and never takeable again.
func (s OwnershipState) Takeable() bool {
	if s.Current == nil {
		return false
	}
	if s.Health == OwnershipHealthReassignment {
		return true
	}
	return s.Pending != nil && s.PendingTimedOut
}

// ReplyBlocked reports whether the client reply gate refuses the CURRENT
// owner by the health word: the unavailable owner blocks the outgoing
// writes (the reassignment path stays open); a pending or timed-out
// request does NOT block — the owner remains the owner until the
// acknowledge (AC-5).
func (s OwnershipState) ReplyBlocked() bool {
	return s.Health == OwnershipHealthReassignment
}

// ---------------------------------------------------------------------------
// The surface evidence gate (AC-8)
// ---------------------------------------------------------------------------

// SurfaceCapability is the domain word of one capability report of the
// connection: the direction, the operation, the state and the expiry. The
// product name is deliberately absent — it is never a proof.
type SurfaceCapability struct {
	Direction string // inbound | outbound
	Operation string // query | command | event
	State     string // unverified | verifying | verified | unsupported | failed | unknown | stale
	ExpiresAt time.Time
	HasExpiry bool
}

// ExternalSurfaceEvidenced resolves the writable-surface evidence gate:
// the external Inbox needs an acting account-scoped write/ownership
// capability — the direction outbound, the operation command or event, the
// state verified, not expired. A name (Jivo or another product) is never a
// proof; without the evidence the honest refusal stands and the minimal
// Staffy surface is used instead.
func ExternalSurfaceEvidenced(capabilities []SurfaceCapability, now time.Time) bool {
	for _, capability := range capabilities {
		if capability.Direction != "outbound" {
			continue
		}
		if capability.Operation != "command" && capability.Operation != "event" {
			continue
		}
		if capability.State != "verified" {
			continue
		}
		if capability.HasExpiry && !now.Before(capability.ExpiresAt) {
			continue
		}
		return true
	}
	return false
}

// SurfaceNotEvidencedText is the honest evidence-gate refusal.
const SurfaceNotEvidencedText = "Внешний Inbox не подтверждён действующим доказательством записи (account-scoped capability): используется минимальная Staffy surface, либо маршрут не допускается."

// ---------------------------------------------------------------------------
// Command guards
// ---------------------------------------------------------------------------

// GuardOwnershipReason validates the bounded honest reason of the
// ownership commands.
func GuardOwnershipReason(reason string) error {
	if utf8.RuneCountInString(reason) > MaxOwnershipReasonRunes {
		return fmt.Errorf("причина владения длиннее %d символов", MaxOwnershipReasonRunes)
	}
	return nil
}

// GuardReplyContent validates the bounded reply content: the operator's
// own words, never empty, never a bare code echo.
func GuardReplyContent(content string) error {
	text := strings.TrimSpace(content)
	if text == "" {
		return fmt.Errorf("текст ответа клиенту обязателен")
	}
	if utf8.RuneCountInString(content) > MaxReplyContentRunes {
		return fmt.Errorf("текст ответа длиннее %d символов", MaxReplyContentRunes)
	}
	return nil
}

// GuardSurface validates the writable surface dictionary of a command.
func GuardSurface(surface string) error {
	if !inDictionary(WritableSurfaces, surface) {
		return fmt.Errorf("записывающая поверхность %s не входит в словарь (%s)", surface, joinQuoted(WritableSurfaces))
	}
	return nil
}

// GuardExpectedEpoch validates the CAS expectation: every transition
// command carries the epoch it believes current (>= 1).
func GuardExpectedEpoch(expected int64) error {
	if expected < 1 {
		return fmt.Errorf("expected_epoch обязателен (>= 1): команда владения сверяет текущую версию")
	}
	return nil
}

// GuardRequestTransfer validates the command material of the transfer
// request: the target owner (a member or the Staffy employee of the case —
// the app resolves it against the member facts and the case employee), the
// surface dictionary and the bounded reason. The deadline NEVER comes from
// the command — the app reads the human coverage word.
func GuardRequestTransfer(toOwnerKind string, surface, reason string) error {
	if !inDictionary(OwnerKinds, toOwnerKind) {
		return fmt.Errorf("вид владельца-цели %s не входит в словарь (%s)", toOwnerKind, joinQuoted(OwnerKinds))
	}
	if err := GuardSurface(surface); err != nil {
		return err
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("причина передачи обязательна")
	}
	return GuardOwnershipReason(reason)
}

// GuardResolveTransfer validates the resolution material: the dictionary
// and the bounded reason.
func GuardResolveTransfer(kind, reason string) error {
	if !inDictionary(TransferResolutions, kind) {
		return fmt.Errorf("решение передачи %s не входит в словарь (%s)", kind, joinQuoted(TransferResolutions))
	}
	return GuardOwnershipReason(reason)
}

// GuardSendClientReply validates the reply command material: the CAS
// expectation and the bounded content. The owner check is the state gate
// below — the actor identity is the server-side truth.
func GuardSendClientReply(expected int64, content string) error {
	if err := GuardExpectedEpoch(expected); err != nil {
		return err
	}
	return GuardReplyContent(content)
}

// OwnershipGateText is the honest refusal of the reply gate: the current
// owner (kind + name), the epoch and the permitted path. The values come
// from the caller (the state derivation + the member name read) — the
// shape is fixed so the UI shows it directly.
type OwnershipGateRefusal struct {
	OwnerKind     string
	OwnerName     string
	CurrentEpoch  int64
	PermittedPath string
}

// ReplyGateError is the machine-readable refusal of the client reply
// gate: the current owner (kind + name), the current epoch and the
// permitted path — the problem contract carries them as fields, the UI
// shows them directly, without a second logic. The reason text is safe
// (never the internal identifiers beyond the owner word).
type ReplyGateError struct {
	OwnerKind    string
	OwnerName    string
	CurrentEpoch int64
	Reason       string
}

func (e ReplyGateError) Error() string { return e.Reason }

// ReplyGatePermittedPath is the fixed permitted path word of the gate.
const ReplyGatePermittedPath = "Запросите передачу (request_transfer)."

// Text renders the refusal the operator sees.
func (r OwnershipGateRefusal) Text() string {
	kind := "участник"
	if r.OwnerKind == OwnerKindStaffy {
		kind = "цифровой сотрудник"
	}
	return fmt.Sprintf("Ответ клиенту доступен только текущему владельцу: %s %s (epoch %d). %s", kind, r.OwnerName, r.CurrentEpoch, r.PermittedPath)
}

// StaffyOwnerText is the honest refusal when the current owner is Staffy:
// the digital employee does not send in this slice (its autonomous
// behaviour is Story 2-5).
const StaffyOwnerText = "Текущий владелец — цифровой сотрудник: клиентские ответы в этом срезе не отправляются (автономное поведение — Story 2-5)."

// NotOwnerText is the refusal when the actor is not the current owner.
const NotOwnerText = "Вы не текущий владелец обращения: запросите передачу (команда request_transfer)."

// GoverningCaseText is the refusal when the target conversation is
// governed by another case (the unified dialog of an active binding or
// its own case after the unlink).
const GoverningCaseText = "Разговор управляется другим обращением: ответ доступен только владельцу объединённого диалога."

// StaleEpochText is the refusal when the expected epoch moved.
const StaleEpochText = "Версия владения изменилась: перечитайте текущего владельца и повторите команду."

// ReassignmentText is the refusal when the owner is unavailable.
const ReassignmentText = "Владелец недоступен: новые исходящие блокированы до безопасного переназначения (take резервным владельцем или reassign надзора)."

// HealthyTakeText is the refusal of the take over a healthy ownership.
const HealthyTakeText = "Владение здорово: забрать нельзя — запросите передачу (команда request_transfer)."

// PendingTransferText is the refusal of the second simultaneous request.
const PendingTransferText = "Запрос передачи уже назначен: дождитесь решения или дедлайна; повторный запрос не создаётся."

// RequestExpiredText is the refusal of the late acknowledge.
const RequestExpiredText = "Дедлайн подтверждения истёк: запрос погашен — передача не состоялась."

// TakeNotAllowedText is the generic take refusal of a non-takeable state.
const TakeNotAllowedText = "Забрать владение можно только когда дедлайн подтверждения истёк без решения или владелец недоступен."

// OwnerNotTargetText is the refusal when someone else than the target
// tries to acknowledge.
const OwnerNotTargetText = "Подтвердить передачу может только владелец-цель запроса."

// ResolveNotPermittedText is the refusal of the decline/cancel roles.
const ResolveNotPermittedText = "Отклонить или отменить запрос может цель или автор запроса."

// ReassignScopeText is the refusal when a non-configuration member tries
// the supervision path.
const ReassignScopeText = "Прямое переназначение — путь надзора: требуется полномочие configuration."

// TransferRequestScopeText is the refusal when neither the current owner
// nor a configuration member requests the transfer.
const TransferRequestScopeText = "Запросить передачу может текущий владелец или обладатель полномочия configuration."

// StaffyReplyText is the refusal when the Staffy principal is the actor of
// the reply command.
const StaffyReplyText = StaffyOwnerText

// CoverageUnavailableText is the fail-closed refusal when the human
// coverage of the acting decision cannot be read.
const CoverageUnavailableText = "Human coverage действующего решения недоступна: дедлайн подтверждения не выдумывается — команда честно отклоняется."

// ---------------------------------------------------------------------------
// The client confirmation of the ownership events (the outbox payloads)
// ---------------------------------------------------------------------------

// OwnershipChangedPayload is the safe canonical JSON of the ownership
// change event: the term snapshot — the identifiers, the epoch, the owner
// kind, the surface and the origin. No content, no reason text.
type OwnershipChangedPayload struct {
	TenantID        id.UUID   `json:"tenant_id"`
	CaseID          id.UUID   `json:"case_id"`
	TermID          id.UUID   `json:"term_id"`
	Epoch           int64     `json:"epoch"`
	OwnerKind       string    `json:"owner_kind"`
	OwnerRef        id.UUID   `json:"owner_ref"`
	WritableSurface string    `json:"writable_surface"`
	Origin          string    `json:"origin"`
	ActorID         id.UUID   `json:"actor_id"`
	OccurredAt      time.Time `json:"occurred_at"`
}

// ClientReplyPreparedPayload is the safe canonical JSON of the prepared
// reply event: the identifiers, the owner/epoch/surface and the route
// snapshot. The CONTENT never enters the envelope.
type ClientReplyPreparedPayload struct {
	TenantID        id.UUID   `json:"tenant_id"`
	CaseID          id.UUID   `json:"case_id"`
	ConversationID  id.UUID   `json:"conversation_id"`
	ReplyID         id.UUID   `json:"reply_id"`
	OwnerKind       string    `json:"owner_kind"`
	OwnerRef        id.UUID   `json:"owner_ref"`
	Epoch           int64     `json:"epoch"`
	WritableSurface string    `json:"writable_surface"`
	ConnectionID    id.UUID   `json:"connection_id"`
	Provider        string    `json:"provider"`
	Channel         string    `json:"channel"`
	OccurredAt      time.Time `json:"occurred_at"`
}

// ClientReplyDispatchedPayload is the safe canonical JSON of the dispatched
// reply outcome event: the identifiers and the kind. The CONTENT never
// enters the envelope.
type ClientReplyDispatchedPayload struct {
	TenantID   id.UUID   `json:"tenant_id"`
	CaseID     id.UUID   `json:"case_id"`
	ReplyID    id.UUID   `json:"reply_id"`
	Kind       string    `json:"kind"`
	OccurredAt time.Time `json:"occurred_at"`
}

// The outbox event types of the ownership surface (each v1).
const (
	OutboxEventOwnershipChanged        = "case.ownership_changed"
	OutboxEventOwnershipChangedV1      = 1
	OutboxEventClientReplyPrepared     = "case.client_reply_prepared"
	OutboxEventClientReplyPreparedV1   = 1
	OutboxEventClientReplyDispatched   = "case.client_reply_dispatched"
	OutboxEventClientReplyDispatchedV1 = 1
)

// The dedup keys of the ownership outbox records.
func DedupKeyOwnershipChanged(termID id.UUID) string {
	return "case.ownership_changed:" + termID.String()
}

func DedupKeyClientReplyPrepared(replyID id.UUID) string {
	return "case.client_reply_prepared:" + replyID.String()
}

func DedupKeyClientReplyDispatched(replyID id.UUID) string {
	return "case.client_reply_dispatched:" + replyID.String()
}

// OwnershipChangedPayloadJSON renders the safe canonical JSON.
func OwnershipChangedPayloadJSON(payload OwnershipChangedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// ClientReplyPreparedPayloadJSON renders the safe canonical JSON.
func ClientReplyPreparedPayloadJSON(payload ClientReplyPreparedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// ClientReplyDispatchedPayloadJSON renders the safe canonical JSON.
func ClientReplyDispatchedPayloadJSON(payload ClientReplyDispatchedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// OwnershipAttentionSummary is the fixed exception text of the drain
// observation (no PII): the expired pending request or the unavailable
// owner.
const OwnershipAttentionSummary = "Владение требует внимания: дедлайн подтверждения истёк без решения или владелец недоступен — безопасный путь: take резервным владельцем или reassign надзора."

// The operational exception kind of the slice (the dictionary extension
// of the migration).
const ExceptionOwnershipAttention = "ownership_attention"
