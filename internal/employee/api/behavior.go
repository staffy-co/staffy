package api

import (
	"context"
	"github.com/staffy-co/staffy/internal/employee/domain"
	identity "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

type BehaviorCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	ExpectedVersion                     int64
	Config                              domain.Behavior
	ChangeSummary                       string
	Scenario                            string
	PackID, PackVersion                 string
}
type BehaviorService interface {
	ReadBehavior(context.Context, identity.Principal, id.UUID, id.UUID) ([]domain.BehaviorVersion, error)
	SaveBehavior(context.Context, BehaviorCommand) (domain.BehaviorVersion, error)
	PreviewBehavior(context.Context, BehaviorCommand) (domain.BehaviorPreview, error)
}
