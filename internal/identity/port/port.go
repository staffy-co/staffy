package port

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

type CreateTenantParams struct {
	Principal      domain.Principal
	TenantID       id.UUID
	Name           string
	IdempotencyKey string
	RequestHash    string
	CorrelationID  id.UUID
	EventID        id.UUID
	OccurredAt     time.Time
}

type Repository interface {
	ReplayTenant(context.Context, domain.Principal, string, string) (domain.Tenant, bool, error)
	CreateTenant(context.Context, CreateTenantParams) (domain.Tenant, bool, error)
	Workspace(context.Context, tenancy.Context) (domain.Workspace, error)
}

type PrincipalResolver interface {
	Resolve(context.Context) (domain.Principal, error)
}
