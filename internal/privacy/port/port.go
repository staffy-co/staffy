package port

import (
        "context"
        "time"

        privacyDomain "github.com/artemiosu/staffy/internal/privacy/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

// MapEntryParams carries one append-only data map version.
type MapEntryParams struct {
        Scope    tenancy.Context
        Class    privacyDomain.ArtifactClass
        Entry    privacyDomain.DataMapEntry
        Now      time.Time
}

// PolicyParams carries one append-only retention policy version.
type PolicyParams struct {
        Scope   tenancy.Context
        Class   privacyDomain.ArtifactClass
        Policy  privacyDomain.RetentionPolicy
        Now     time.Time
}

// HoldEventParams carries one append-only legal hold event.
type HoldEventParams struct {
        Scope  tenancy.Context
        Event  privacyDomain.LegalHoldEvent
        Now    time.Time
}

// DeletionRecordParams carries the atomic deletion request bundle: the
// request identity, its per-class outcomes and the lifecycle receipt.
type DeletionRecordParams struct {
        Scope    tenancy.Context
        Request  DeletionRequestRecord
        Outcomes []privacyDomain.DeletionOutcome
        Receipt  privacyDomain.Receipt
}

// DeletionRequestRecord is the stored request identity.
type DeletionRequestRecord struct {
        RequestID     id.UUID
        ActorID       id.UUID
        IdempotencyKey string
        RequestHash   string
        Scope         []privacyDomain.ArtifactClass
        RequestedAt   time.Time
}

// Repository is the persistence port of the privacy module. All writes are
// appends; current states are derived at read time.
type Repository interface {
        // SaveMapEntry appends the next version of one class entry. The version
        // number is computed under a per-class advisory lock.
        SaveMapEntry(context.Context, MapEntryParams) (privacyDomain.DataMapEntry, error)
        // ListMapEntries returns every stored version of every class.
        ListMapEntries(context.Context, tenancy.Context) ([]privacyDomain.DataMapEntry, error)
        // SavePolicy appends the next policy version of one class.
        SavePolicy(context.Context, PolicyParams) (privacyDomain.RetentionPolicy, error)
        // ListPolicies returns every stored policy version of every class.
        ListPolicies(context.Context, tenancy.Context) ([]privacyDomain.RetentionPolicy, error)
        // SaveHoldEvent appends one hold activation or release event.
        SaveHoldEvent(context.Context, HoldEventParams) error
        // ListHoldEvents returns every stored hold event.
        ListHoldEvents(context.Context, tenancy.Context) ([]privacyDomain.LegalHoldEvent, error)
        // LookupDeletion answers the idempotency question without side effects:
        // a stored key with the same request hash replays the request bundle
        // (outcomes + latest receipt); a different hash is an honest conflict.
        LookupDeletion(ctx context.Context, scope tenancy.Context, actorID id.UUID, key, hash string) (DeletionRequestRecord, []privacyDomain.DeletionOutcome, privacyDomain.Receipt, bool, error)
        // SaveDeletion atomically persists the request, its outcomes and the
        // receipt. A duplicate idempotency key under the advisory lock replays
        // the stored bundle; a different request hash is an honest conflict.
        SaveDeletion(context.Context, DeletionRecordParams) (DeletionRequestRecord, []privacyDomain.DeletionOutcome, privacyDomain.Receipt, bool, error)
        // ListReceipts returns the newest receipts of every request.
        ListReceipts(context.Context, tenancy.Context) ([]privacyDomain.Receipt, error)
        // ReadReceipt returns the request bundle of one request id.
        ReadReceipt(ctx context.Context, scope tenancy.Context, requestID id.UUID) (DeletionRequestRecord, []privacyDomain.DeletionOutcome, privacyDomain.Receipt, error)
        // ClassReferenceTime computes the term anchor of one class: the last
        // change moment of the class data, when rows exist.
        ClassReferenceTime(ctx context.Context, scope tenancy.Context, class privacyDomain.ArtifactClass) (*time.Time, error)
}

// ExecutorResult is the honest result of one owning-module deletion step.
// A definite refusal returns an outcome with its reason; an ambiguous state
// returns OutcomeUnknown with a reconciliation owner and deadline — never a
// blind retry hint and never a panic.
type ExecutorResult struct {
        Outcome                string // deleted / scheduled / unsupported / unknown
        AffectedCount          int64
        DueAt                  *time.Time
        Note                   string
        Block                  *privacyDomain.ManagedBlock
        ReconciliationOwner    string
        ReconciliationDeadline time.Time
}

// ClassDeletionExecutor is implemented by owning modules (employee, identity)
// to delete their own local copies. The privacy orchestrator never writes to
// tables it does not own.
type ClassDeletionExecutor interface {
        ExecuteClassDeletion(ctx context.Context, scope tenancy.Context, class privacyDomain.ArtifactClass, now time.Time) (ExecutorResult, error)
}

// ClassAnchorQuery is implemented by owning modules to report the last-change
// moment of their class data; the privacy store falls back to the policy
// publication time when a module reports nothing.
type ClassAnchorQuery interface {
        ClassAnchor(ctx context.Context, scope tenancy.Context, class privacyDomain.ArtifactClass) (*time.Time, error)
}
