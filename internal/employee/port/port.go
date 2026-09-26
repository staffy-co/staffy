package port

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

type CreateParams struct {
	Scope                              tenancy.Context
	EmployeeID, CorrelationID, EventID id.UUID
	Key, Hash, Name                    string
	Now                                time.Time
}
type SaveParams struct {
	Scope                                       tenancy.Context
	EmployeeID, CorrelationID, EventID, AuditID id.UUID
	ExpectedVersion                             int64
	Brief                                       domain.Brief
	Now                                         time.Time
}
type PinParams struct {
	Scope                                       tenancy.Context
	EmployeeID, CorrelationID, EventID, AuditID id.UUID
	ExpectedVersion                             int64
	Pin                                         domain.PackPin
	ReviewSlots                                 []string
	Now                                         time.Time
}
type Repository interface {
	Create(context.Context, CreateParams) (domain.Draft, bool, error)
	List(context.Context, tenancy.Context) ([]domain.Draft, error)
	Read(context.Context, tenancy.Context, id.UUID) (domain.Draft, error)
	Save(context.Context, SaveParams) (domain.Draft, error)
	Pin(context.Context, PinParams) (domain.Draft, error)
}
