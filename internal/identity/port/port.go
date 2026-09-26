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

type MembershipMutation struct {
	Actor           domain.Principal
	TenantID        id.UUID
	TargetID        id.UUID
	Email           string
	Roles           []domain.Role
	Scopes          []domain.Scope
	ExpectedVersion int64
	Reason          domain.ReasonCode
	CorrelationID   id.UUID
	TokenDigest     string
	Now             time.Time
	ExpiresAt       time.Time
}

type MembershipRepository interface {
	ListMembers(context.Context, tenancy.Context) ([]domain.Membership, error)
	InviteMember(context.Context, MembershipMutation) (domain.Membership, error)
	AcceptInvitation(context.Context, domain.Principal, string, id.UUID, time.Time) (domain.Membership, error)
	ChangeMember(context.Context, MembershipMutation) (domain.Membership, error)
	RevokeMember(context.Context, MembershipMutation) error
	Authorize(context.Context, tenancy.Context, domain.Scope) error
}
