package api

import (
	"context"
	"time"

	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

type CreateCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	Name           string
	IdempotencyKey string
	CorrelationID  id.UUID
}
type SaveCommand struct {
	Actor                               domain.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	ExpectedVersion                     int64
	Brief                               employeeDomain.Brief
}
type CreateResult struct {
	Draft    employeeDomain.Draft
	Replayed bool
}
type Service interface {
	Create(context.Context, CreateCommand) (CreateResult, error)
	List(context.Context, domain.Principal, id.UUID) ([]employeeDomain.Draft, error)
	Read(context.Context, domain.Principal, id.UUID, id.UUID) (employeeDomain.Draft, error)
	Save(context.Context, SaveCommand) (employeeDomain.Draft, error)
}
type Clock interface{ Now() time.Time }
type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}
