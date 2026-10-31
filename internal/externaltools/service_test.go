package externaltools

// The service ladder tests (Story 7-4 step-02): the membership read gate
// runs FIRST — the honest ErrNotFound of a non-member (no disclosure that
// the tenant has an external tool dictionary), the ErrForbidden with the
// scope words of a member without integrations|configuration, the two
// honest ways in (either scope opens the surface) — and the tenant
// isolation of the facts read (the scope of the facts carries the tenant
// of the session, the material of other tenants never enters).

import (
	"context"
	"errors"
	"strings"
	"testing"

	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

const (
	ladderTenant  = "01994f64-6c00-7000-8000-0000000000a1"
	ladderActor   = "01994f64-6c00-7000-8000-0000000000a2"
	foreignTenant = "01994f64-6c00-7000-8000-0000000000a3"
)

// ladderMembership scripts the authorize matrix: the owner holds both
// scopes, the restricted member holds neither, the integrations and
// configuration members hold exactly one honest way in, every other actor
// or tenant is the honest not-found. The mutation surfaces stay the
// fixture stubs (the dictionary is read-only).
type ladderMembership struct{}

func (ladderMembership) ListMembers(context.Context, identityDomain.Principal, id.UUID) ([]identityDomain.Membership, error) {
	return nil, nil
}

func (ladderMembership) InviteMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, string, error) {
	return identityDomain.Membership{}, "", identityDomain.ErrStepUpRequired
}

func (ladderMembership) AcceptInvitation(context.Context, identityDomain.Principal, string, id.UUID) (identityDomain.Membership, error) {
	return identityDomain.Membership{}, identityDomain.ErrNotFound
}

func (ladderMembership) ChangeMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, error) {
	return identityDomain.Membership{}, identityDomain.ErrStepUpRequired
}

func (ladderMembership) RevokeMember(context.Context, identityAPI.MembershipCommand) error {
	return identityDomain.ErrStepUpRequired
}

func (ladderMembership) Authorize(_ context.Context, actor identityDomain.Principal, tenant id.UUID, scope identityDomain.Scope) error {
	if !actor.ID.Valid() || tenant != id.MustParse(ladderTenant) {
		return identityDomain.ErrNotFound
	}
	switch actor.ID {
	case id.MustParse("01994f64-6c00-7000-8000-0000000000a2"):
		return nil
	case id.MustParse("01994f64-6c00-7000-8000-0000000000a4"):
		if scope == identityDomain.Integrations {
			return nil
		}
		return identityDomain.ErrForbidden
	case id.MustParse("01994f64-6c00-7000-8000-0000000000a5"):
		if scope == identityDomain.Configuration {
			return nil
		}
		return identityDomain.ErrForbidden
	case id.MustParse("01994f64-6c00-7000-8000-0000000000a6"):
		return identityDomain.ErrForbidden
	default:
		return identityDomain.ErrNotFound
	}
}

// scopeRecordingFacts records the scope of every read (the isolation leg).
type scopeRecordingFacts struct {
	scopes []tenancy.Context
}

func (f *scopeRecordingFacts) InventoryFacts(_ context.Context, scope tenancy.Context) ([]ConnectionInventory, error) {
	f.scopes = append(f.scopes, scope)
	return []ConnectionInventory{}, nil
}

func TestDictionaryLadderRunsBeforeAnyMaterial(t *testing.T) {
	facts := &scopeRecordingFacts{}
	service := New(ladderMembership{}, facts)
	ctx := context.Background()
	tenant := id.MustParse(ladderTenant)
	integrationsMember := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-0000000000a4")}
	restrictedMember := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-0000000000a6")}

	// The member without the scopes: the 403 words BEFORE the material.
	_, err := service.Dictionary(ctx, restrictedMember, tenant)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("ожидался отказ скоупов: %v", err)
	}
	if !strings.Contains(err.Error(), "integrations или configuration") {
		t.Fatalf("слова скоупов: %v", err)
	}
	if len(facts.scopes) != 0 {
		t.Fatalf("материал не читается до лестницы: %d", len(facts.scopes))
	}

	// The non-member tenant: the honest 404 without any disclosure.
	_, err = service.Dictionary(ctx, integrationsMember, id.MustParse(foreignTenant))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ожидался честный 404: %v", err)
	}

	// The two honest ways in: either scope opens the surface.
	for _, actor := range []identityDomain.Principal{
		integrationsMember,
		{ID: id.MustParse("01994f64-6c00-7000-8000-0000000000a5")},
		{ID: id.MustParse(ladderActor)},
	} {
		if _, err = service.Dictionary(ctx, actor, tenant); err != nil {
			t.Fatalf("полномочие обязано открывать словарь: %v", err)
		}
	}
	if len(facts.scopes) != 3 {
		t.Fatalf("читающий скоуп обязан доходить до фактов: %d", len(facts.scopes))
	}
	// The tenant isolation: every facts read carries the tenant of the session.
	for _, scope := range facts.scopes {
		if scope.TenantID() != tenant {
			t.Fatalf("изолированный скоуп: %v", scope.TenantID())
		}
	}
}
