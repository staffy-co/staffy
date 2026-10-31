package port

import (
	"context"
	"time"

	entitlementDomain "github.com/staffy-co/staffy/internal/entitlement/domain"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// TermsParams carries one append-only terms version.
type TermsParams struct {
	Scope tenancy.Context
	Terms entitlementDomain.TermsVersion
	Now   time.Time
}

// StateEventParams carries one append-only entitlement lifecycle event.
type StateEventParams struct {
	Scope tenancy.Context
	Event entitlementDomain.StateEvent
	Now   time.Time
}

// SourceParams carries one append-only source registration.
type SourceParams struct {
	Scope  tenancy.Context
	Source entitlementDomain.UsageSource
	Now    time.Time
}

// BudgetParams carries one append-only budget version.
type BudgetParams struct {
	Scope  tenancy.Context
	Budget entitlementDomain.BudgetVersion
	Now    time.Time
}

// IngestParams carries one usage event to append. The store resolves the
// idempotency question atomically under a per-key advisory lock: the same
// key with the same payload hash replays the stored event, a different
// hash is an honest conflict.
type IngestParams struct {
	Scope tenancy.Context
	Event entitlementDomain.UsageEvent
	Now   time.Time
}

// ExceptionParams carries one operational exception. The UNIQUE dedupe key
// makes the save idempotent: a repeated key is a silent no-op (created=false).
type ExceptionParams struct {
	Scope     tenancy.Context
	Exception entitlementDomain.UsageException
	Now       time.Time
}

// AckParams carries one append-only exception acknowledgement.
type AckParams struct {
	Scope tenancy.Context
	Ack   entitlementDomain.ExceptionAck
	Now   time.Time
}

// Repository is the persistence port of the entitlement module. All writes
// are appends; current states and aggregates are derived at read time.
type Repository interface {
	// SaveTerms appends the next terms version under an advisory lock.
	SaveTerms(context.Context, TermsParams) (entitlementDomain.TermsVersion, error)
	// ListTerms returns every stored terms version.
	ListTerms(context.Context, tenancy.Context) ([]entitlementDomain.TermsVersion, error)
	// SaveStateEvent appends one lifecycle event.
	SaveStateEvent(context.Context, StateEventParams) error
	// ListStateEvents returns every stored lifecycle event.
	ListStateEvents(context.Context, tenancy.Context) ([]entitlementDomain.StateEvent, error)
	// IngestUsage appends one usage event or replays/conflicts by key.
	IngestUsage(context.Context, IngestParams) (entitlementDomain.UsageEvent, bool, error)
	// ListUsageEvents returns every stored usage event of the tenant.
	ListUsageEvents(context.Context, tenancy.Context) ([]entitlementDomain.UsageEvent, error)
	// SaveSource appends one source registration.
	SaveSource(context.Context, SourceParams) (entitlementDomain.UsageSource, error)
	// ListSources returns every stored source registration.
	ListSources(context.Context, tenancy.Context) ([]entitlementDomain.UsageSource, error)
	// SaveBudget appends the next budget version under an advisory lock.
	SaveBudget(context.Context, BudgetParams) (entitlementDomain.BudgetVersion, error)
	// ListBudgets returns every stored budget version.
	ListBudgets(context.Context, tenancy.Context) ([]entitlementDomain.BudgetVersion, error)
	// SaveException appends one exception; a duplicate dedupe key is a
	// silent no-op returning created=false.
	SaveException(context.Context, ExceptionParams) (entitlementDomain.UsageException, bool, error)
	// ListExceptions returns every stored exception with derived acks.
	ListExceptions(context.Context, tenancy.Context) ([]entitlementDomain.UsageException, error)
	// SaveAck appends one exception acknowledgement.
	SaveAck(context.Context, AckParams) error
	// ExceptionExists answers whether a dedupe key is already stored.
	ExceptionExists(ctx context.Context, scope tenancy.Context, dedupeKey string) (bool, error)
}
