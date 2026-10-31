package api

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
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

// TenantNameService is the principal-free tenant name read of the public
// surfaces (the Story 2.10 continuation page header — the page honestly
// shows whose conversation it is; the token IS the credential, no session
// exists). The transport type-asserts this narrow interface (the
// BindEmployee precedent): the composite Service stays untouched and the
// anonymous scope never reads the membership material.
type TenantNameService interface {
	TenantName(context.Context, id.UUID) (string, error)
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
