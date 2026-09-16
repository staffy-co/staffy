package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

type CreateTenantCommand struct {
	Principal      domain.Principal
	Name           string
	IdempotencyKey string
	CorrelationID  id.UUID
}

type CreateTenantResult struct {
	Tenant   domain.Tenant
	Role     string
	Replayed bool
}

type Service interface {
	CreateTenant(context.Context, CreateTenantCommand) (CreateTenantResult, error)
	Workspace(context.Context, domain.Principal, id.UUID) (domain.Workspace, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}
