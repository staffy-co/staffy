package main

import (
	"context"

	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/tools"
)

// toolsMembership is the LIVE membership semantics of the fixture world
// for the tool catalog read gate (the actionsMembership precedent of the
// Story 3.1 fixture): the owner holds every scope, the second operator
// holds the review+audit scopes only (the honest ErrForbidden member of
// the 403 words row), and every other actor or tenant is the honest
// ErrNotFound (the store semantics the service leans on — membership
// FIRST, the scope words BEFORE any material). The mutation surfaces
// stay the fixture stubs: the tool catalog is read-only and the journey
// never mutates the membership.
type toolsMembership struct{}

func (toolsMembership) ListMembers(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error) {
	return nil, nil
}

func (toolsMembership) InviteMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, string, error) {
	return domain.Membership{}, "", domain.ErrStepUpRequired
}

func (toolsMembership) AcceptInvitation(context.Context, domain.Principal, string, id.UUID) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrNotFound
}

func (toolsMembership) ChangeMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrStepUpRequired
}

func (toolsMembership) RevokeMember(context.Context, identityAPI.MembershipCommand) error {
	return domain.ErrStepUpRequired
}

func (toolsMembership) Authorize(_ context.Context, actor domain.Principal, tenant id.UUID, scope domain.Scope) error {
	if !actor.ID.Valid() || tenant != tenantID {
		return domain.ErrNotFound
	}
	switch actor.ID {
	case principalID:
		// The owner holds every scope (AllScopes).
		return nil
	case secondOperatorID:
		if scope == domain.Review || scope == domain.Audit {
			return nil
		}
		// The review member: the honest member refusal of the gate.
		return domain.ErrForbidden
	case operationsOperatorID:
		// The operations-only member: the member exists, the scope does
		// not — the honest 403 of the tool catalog read.
		return domain.ErrForbidden
	default:
		return domain.ErrNotFound
	}
}

// newToolsCatalogFixture wires the REAL tool catalog service (the Story
// 7.2 read gate over the embedded registry — the LIVE three contracts of
// the auto-parts binding) over the fixture membership ladder. The
// registry itself is the production embed: no fixture dictionary exists
// (unlike the action catalog), the live artifacts ARE the journey world.
func newToolsCatalogFixture() *tools.Service {
	return tools.NewCatalogService(toolsMembership{})
}
