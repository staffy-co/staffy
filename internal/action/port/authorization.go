package port

import (
	"context"
	"errors"
	"time"

	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.5: the consumer-owned ports of the confirmation→authorization
// contour (step-02: the migration, the store, the app and the ONE
// transaction)
// ---------------------------------------------------------------------------
//
// Two repositories share the SAME Postgres store of the module: the
// confirmation repository (the append-only consent ladder) and the
// operation repository (the immutable operations with the ONE
// transactional method). The store owns NO adapters (the 3-3
// persist-before-emit discipline): ReserveOperation takes ZERO ports — its
// signature is a pure persistence boundary, a network call is impossible
// BY TYPE (the Enqueue precedent of 3-3); every block of the ordered
// gateway happens BEFORE the transaction and consumes NOTHING.

// The store-contract errors of the contour (the consumer-owned vocabulary
// of the repository boundary; the transport maps them in step-03).
var (
	// ErrStaleConfirmationVersion is the honest raced-ladder refusal: the
	// expected version/event number moved under the advisory lock — the
	// caller re-derives, never merges (the 3-4 stale discipline).
	ErrStaleConfirmationVersion = errors.New("confirmation version ladder moved")
	// ErrConfirmationExhausted is the honest refusal raced INSIDE the
	// ReserveOperation transaction: the re-selected consumed count under
	// the advisory lock reached the allowance — the second authorization
	// of one single_use confirmation refuses honestly WITHOUT consuming
	// (the whole transaction rolls back).
	ErrConfirmationExhausted = errors.New("confirmation consumption exhausted under the lock")
	// ErrIdempotencyTwin is the INTERNAL raced-twin signal of the UNIQUE
	// (tenant, idempotency_key): the gateway re-reads the winner by the
	// key, compares the material digest and answers the honest replay
	// return — or the 409 conflict when the material differs. The
	// transport never sees this error raw.
	ErrIdempotencyTwin = errors.New("operation idempotency twin exists")
	// ErrReservationConflict is the INTERNAL cross-case loser signal of
	// the unique (tenant, owning_system, business_intent_key) reservation:
	// the gateway reads the existing tenant-visible Operation by the
	// intent and answers the honest reference — the loser's consumption
	// never happened (the whole transaction rolls back). The transport
	// never sees this error raw.
	ErrReservationConflict = errors.New("business intent reservation exists")
	// ErrIdempotencyConflict is the honest 409 of the replay comparison
	// (AC-8): the SAME client key arrived with a DIFFERENT material — the
	// original Operation stays unchanged, the conflict exception is
	// visible to the authorized owner.
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with another material")
	// ErrMaterialConditionChanged is the honest 409 of the last read-only
	// material check (AC-3/AG-4): an owning-system fact moved between the
	// preview and the authorization — the revalidation asks for the new
	// version pair.
	ErrMaterialConditionChanged = errors.New("material condition changed")
	// ErrFreshReadsUnavailable is the fail-closed 403 of the nil
	// OwningSystemReader (production, AG-4): the authorization honestly
	// cannot happen without the fresh reads.
	ErrFreshReadsUnavailable = errors.New("fresh reads unavailable")
	// ErrPreviewDigestChanged is the honest 409 of the double digest
	// protection (AC-1): the LIVE preview row changed against the linked
	// copy of the confirmation — the changed material is never
	// authorized.
	ErrPreviewDigestChanged = errors.New("live preview digest changed against the confirmation copy")
	// ErrPreviewBlockedMaterial is the honest 409 of the readiness
	// re-check at the authorization: the blocked material is never
	// authorized.
	ErrPreviewBlockedMaterial = errors.New("preview material is blocked")
	// ErrOperationIntegrity is the loud alarm of a structural violation
	// that cannot happen through the guards (the operation PK raced by a
	// minted identity) — never masked.
	ErrOperationIntegrity = errors.New("operation integrity violation")
)

// ConfirmationRejectedError carries the exact state word of the
// rejection-ladder refusal (AC-1: superseded / expired / revoked /
// exhausted — every rejection speaks the word BY NAME, never a masked
// silence; the consumption never happens on a rejection).
type ConfirmationRejectedError struct {
	State string
}

func (e ConfirmationRejectedError) Error() string {
	return "confirmation rejected: " + e.State
}

// AuthorizationStaleError carries the exact freshness reason word of the
// live re-check at the authorization (the 3-4 dictionary — the material
// condition moved between the preview and the authorization; the
// consumption never happens on a rejection).
type AuthorizationStaleError struct {
	Reason string
}

func (e AuthorizationStaleError) Error() string {
	return "authorization freshness: " + e.Reason
}

// ---------------------------------------------------------------------------
// The row shapes of the two repositories
// ---------------------------------------------------------------------------

// ConfirmationEventRow is one append-only row of the consent history: the
// version scope (the events of one confirmation VERSION — consumed is
// counted per version), the six-word kind, the safe reason/details and
// the actor (the 3-4 PreviewEventRow precedent).
type ConfirmationEventRow struct {
	ConfirmationID id.UUID
	VersionNo      int64
	CaseID         id.UUID
	EventID        id.UUID
	EventNo        int64
	Kind           string
	Reason         string
	Details        string
	ActorID        id.UUID
	CreatedAt      time.Time
}

// AppendConfirmationParams is the ONE transaction of a consent append: the
// fully-derived confirmation version (the app resolved the thread and
// derived version+1 with the linked material copies), the confirmed event
// id of the NEW version, the superseded event id of the OLD version (the
// zero uuid when this is the first version — no supersede event lands),
// and the moment. The store re-derives the version ladder under the
// advisory lock — the raced twin answers ErrStaleConfirmationVersion, the
// UNIQUE (tenant, preview_id, preview_version_no) is the structural
// bijection backstop.
type AppendConfirmationParams struct {
	Scope             tenancy.Context
	Confirmation      domain.Confirmation
	PrevVersionNo     int64
	ConfirmedEventID  id.UUID
	SupersededEventID id.UUID
	Now               time.Time
}

// AppendConfirmationEventParams is the append of ONE history event of one
// exact version (revoked of the human revocation; authorization_conflict
// of the visible digest mismatch; authorization_blocked of the visible
// ladder rejection) — the event_no re-derives under the advisory lock.
type AppendConfirmationEventParams struct {
	Scope          tenancy.Context
	ConfirmationID id.UUID
	VersionNo      int64
	CaseID         id.UUID
	EventID        id.UUID
	Kind           string
	Reason         string
	Details        string
	ActorID        id.UUID
	Now            time.Time
}

// OperationQueuedPayload is the SAFE canonical outbox payload of the
// operation.queued record: ids, pins, words and the TWO key identities —
// the material NEVER enters (no typed input values, no snapshot rows, no
// phone or template words; the 3-6 worker rebuilds the material payload
// from the immutable rows — the BuildOutboundPayload precedent of 3-3).
type OperationQueuedPayload struct {
	EventType             string    `json:"event_type"`
	TenantID              id.UUID   `json:"tenant_id"`
	OperationID           id.UUID   `json:"operation_id"`
	CaseID                id.UUID   `json:"case_id"`
	WorkItemID            id.UUID   `json:"work_item_id"`
	DefinitionID          string    `json:"definition_id"`
	DefinitionVersion     string    `json:"definition_version"`
	OwningSystem          string    `json:"owning_system"`
	PreviewID             id.UUID   `json:"preview_id"`
	PreviewVersionNo      int64     `json:"preview_version_no"`
	ConfirmationID        id.UUID   `json:"confirmation_id"`
	ConfirmationVersionNo int64     `json:"confirmation_version_no"`
	TargetKind            string    `json:"target_kind"`
	TargetRef             id.UUID   `json:"target_ref"`
	OwnerKind             string    `json:"owner_kind"`
	OwnerRef              id.UUID   `json:"owner_ref"`
	Partition             string    `json:"partition"`
	BusinessIntentKey     string    `json:"business_intent_key"`
	IdempotencyKey        string    `json:"idempotency_key"`
	MaterialDigest        string    `json:"material_digest"`
	QueuedAt              time.Time `json:"queued_at"`
}

// ReserveOperationParams is the ONE transactional command of the contour:
// the fully-derived immutable Operation row (the app derived the two key
// identities and the material digest over the pinned request), the
// definition's consumption rule word (a DATA pin — the allowance
// re-derives under the lock), the consumed event id, the ledger entry and
// the outbox record id with the SAFE payload. The signature carries ZERO
// adapters/ports — persist-before-emit is STRUCTURAL (the Enqueue
// precedent of 3-3); ANY insert failure rolls back the WHOLE chain:
// consumed, reservation, operation, ledger and outbox never exist
// partially («Confirmation остаётся непотреблённой, Connector не
// вызывается» — AC-6 материальна).
type ReserveOperationParams struct {
	Scope           tenancy.Context
	Operation       domain.Operation
	ConsumptionRule string
	ActorID         id.UUID
	ConsumedEventID id.UUID
	LedgerEntry     domain.OperationLedgerEntry
	OutboxEventID   id.UUID
	OutboxPayload   OperationQueuedPayload
	Now             time.Time
}

// ---------------------------------------------------------------------------
// The confirmation repository contract
// ---------------------------------------------------------------------------

// ConfirmationRepository is the persistence contract of the consent
// ladder: the version reads (the thread resolve of the confirm command,
// the exact pin resolve of the authorization), the event reads of the
// state derivation, and the TWO appends (the version append with the
// atomic supersede; the single event append). The store owns NO adapters
// — every transaction is a pure persistence boundary.
type ConfirmationRepository interface {
	// ReadPreviewConfirmations reads every confirmation version of one
	// preview thread ordered by the monotonic version (the thread resolve
	// of the confirm command: the newest version is the thread head).
	ReadPreviewConfirmations(ctx context.Context, scope tenancy.Context, previewID id.UUID) ([]domain.Confirmation, error)
	// ReadCaseConfirmations reads every confirmation version of one case
	// ordered by the confirmation identity and the version (the view
	// material).
	ReadCaseConfirmations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]domain.Confirmation, error)
	// ReadConfirmationVersion reads one exact confirmation version (the
	// pin resolve of the authorization); domain.ErrNotFound answers a
	// dead pin BEFORE any material.
	ReadConfirmationVersion(ctx context.Context, scope tenancy.Context, confirmationID id.UUID, versionNo int64) (domain.Confirmation, error)
	// ReadConfirmationVersionEvents reads the events of one exact version
	// ordered by the monotonic number (the state derivation input — the
	// consumption is counted per version).
	ReadConfirmationVersionEvents(ctx context.Context, scope tenancy.Context, confirmationID id.UUID, versionNo int64) ([]domain.ConfirmationEvent, error)
	// ReadCaseConfirmationEvents reads every confirmation event of one
	// case ordered by time (the view material with the visible conflict
	// exceptions).
	ReadCaseConfirmationEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]ConfirmationEventRow, error)
	// AppendConfirmationVersion appends one consent version (and the
	// confirmed event of the new version, and the superseded event of the
	// old version) under the confirmation advisory lock — ONE transaction,
	// atomic supersede.
	AppendConfirmationVersion(ctx context.Context, p AppendConfirmationParams) error
	// AppendConfirmationEvent appends ONE history event of one exact
	// version under the confirmation advisory lock.
	AppendConfirmationEvent(ctx context.Context, p AppendConfirmationEventParams) error
}

// ---------------------------------------------------------------------------
// The operation repository contract
// ---------------------------------------------------------------------------

// OperationRepository is the persistence contract of the authorized
// operations: the replay lookup by the client key, the cross-case
// reference lookup by the server intent key, the RLS reads of the view,
// and THE ONE transactional method. ReserveOperation takes ZERO ports —
// the persist-before-emit discipline is the signature itself.
type OperationRepository interface {
	// ReadPinnedPreviewVersion reads one exact LIVE preview version row
	// (the linked material of the confirm/authorization gates — the
	// digest re-check compares this row byte-exact); domain.ErrNotFound
	// answers a dead pin.
	ReadPinnedPreviewVersion(ctx context.Context, scope tenancy.Context, previewID id.UUID, versionNo int64) (domain.ActionPreview, error)
	// FindOperationByIdempotencyKey reads the operation of one client key
	// (the FIRST check of the gateway — the replay comparison; found=false
	// answers a first attempt).
	FindOperationByIdempotencyKey(ctx context.Context, scope tenancy.Context, idempotencyKey string) (domain.Operation, bool, error)
	// FindOperationByIntent reads the operation reserved under one
	// business intent (the loser reference of the cross-case race;
	// found=false answers an unreserved intent).
	FindOperationByIntent(ctx context.Context, scope tenancy.Context, owningSystem, businessIntentKey string) (domain.Operation, bool, error)
	// ReadCaseOperations reads every operation of one case ordered by
	// time (the view material).
	ReadCaseOperations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]domain.Operation, error)
	// ReadOperationLedger reads the append-only ledger of one operation
	// ordered by the monotonic entry number (the state derivation input).
	ReadOperationLedger(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]domain.OperationLedgerEntry, error)
	// ReadOutboxQueued reports the queue FACT of one operation (the
	// presence of the operation.queued record — the spoken word derives
	// from it, nothing is stored).
	ReadOutboxQueued(ctx context.Context, scope tenancy.Context, operationID id.UUID) (bool, error)
	// ReserveOperation is the ONE transaction of the authorization:
	// the consumed re-selection under the confirmation advisory lock, the
	// allowance check, the INSERT of the consumed event, the reservation,
	// the operation, the ledger authorized entry and the outbox
	// operation.queued record — ONE commit, ANY failure rolls back
	// everything. The raced UNIQUE violations map to the honest internal
	// signals (ErrIdempotencyTwin / ErrReservationConflict); the raced
	// consumption maps to ErrConfirmationExhausted.
	ReserveOperation(ctx context.Context, p ReserveOperationParams) error
}
