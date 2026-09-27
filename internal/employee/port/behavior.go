package port

import (
	"context"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"time"
)

type BehaviorSaveParams struct {
	Scope                                       tenancy.Context
	EmployeeID, CorrelationID, EventID, AuditID id.UUID
	ExpectedVersion                             int64
	Config                                      domain.Behavior
	Summary                                     string
	Now                                         time.Time
}
type BehaviorRepository interface {
	ReadBehavior(context.Context, tenancy.Context, id.UUID) ([]domain.BehaviorVersion, error)
	SaveBehavior(context.Context, BehaviorSaveParams) (domain.BehaviorVersion, error)
}
