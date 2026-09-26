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

type MembershipService interface {
	ListMembers(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error)
	InviteMember(context.Context, MembershipCommand) (domain.Membership, string, error)
	AcceptInvitation(context.Context, domain.Principal, string, id.UUID) (domain.Membership, error)
	ChangeMember(context.Context, MembershipCommand) (domain.Membership, error)
	RevokeMember(context.Context, MembershipCommand) error
	Authorize(context.Context, domain.Principal, id.UUID, domain.Scope) error
}

type MembershipCommand struct {
	Actor           domain.Principal
	TenantID        id.UUID
	TargetID        id.UUID
	Email           string
	Roles           []domain.Role
	Scopes          []domain.Scope
	ExpectedVersion int64
	Reason          domain.ReasonCode
	CorrelationID   id.UUID
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}
