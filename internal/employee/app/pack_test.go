package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/employee/api"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/port"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

var packTenant = id.MustParse("01994f64-6c00-7000-8000-000000000011")
var packOtherTenant = id.MustParse("01994f64-6c00-7000-8000-000000000012")
var packEmployee = id.MustParse("01994f64-6c00-7000-8000-000000000013")
var packActor = id.MustParse("01994f64-6c00-7000-8000-000000000014")
var packCorrelation = id.MustParse("01994f64-6c00-7000-8000-000000000015")

type packRepository struct {
	draft  domain.Draft
	writes int
}

func (r *packRepository) Create(context.Context, port.CreateParams) (domain.Draft, bool, error) {
	return domain.Draft{}, false, errors.New("unused")
}
func (r *packRepository) List(context.Context, tenancy.Context) ([]domain.Draft, error) {
	return nil, errors.New("unused")
}
func (r *packRepository) Read(_ context.Context, scope tenancy.Context, employee id.UUID) (domain.Draft, error) {
	if scope.TenantID() != r.draft.TenantID || employee != r.draft.EmployeeID {
		return domain.Draft{}, domain.ErrNotFound
	}
	return r.draft, nil
}
func (r *packRepository) Save(context.Context, port.SaveParams) (domain.Draft, error) {
	return domain.Draft{}, errors.New("unused")
}
func (r *packRepository) Pin(_ context.Context, p port.PinParams) (domain.Draft, error) {
	if p.Scope.TenantID() != r.draft.TenantID || p.EmployeeID != r.draft.EmployeeID {
		return domain.Draft{}, domain.ErrNotFound
	}
	if p.ExpectedVersion != r.draft.Version {
		return domain.Draft{}, domain.ErrConflict
	}
	r.writes++
	r.draft.Pack = &p.Pin
	r.draft.ReviewSlots = p.ReviewSlots
	r.draft.Version++
	return r.draft, nil
}

type packIdentity struct{ allowed bool }

func (i packIdentity) ListMembers(context.Context, identityDomain.Principal, id.UUID) ([]identityDomain.Membership, error) {
	return nil, nil
}
func (i packIdentity) InviteMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, string, error) {
	return identityDomain.Membership{}, "", nil
}
func (i packIdentity) AcceptInvitation(context.Context, identityDomain.Principal, string, id.UUID) (identityDomain.Membership, error) {
	return identityDomain.Membership{}, nil
}
func (i packIdentity) ChangeMember(context.Context, identityAPI.MembershipCommand) (identityDomain.Membership, error) {
	return identityDomain.Membership{}, nil
}
func (i packIdentity) RevokeMember(context.Context, identityAPI.MembershipCommand) error { return nil }
func (i packIdentity) Authorize(_ context.Context, _ identityDomain.Principal, _ id.UUID, scope identityDomain.Scope) error {
	if !i.allowed || scope != identityDomain.Configuration {
		return identityDomain.ErrForbidden
	}
	return nil
}

type packClock struct{}

func (packClock) Now() time.Time { return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC) }

type packIDs struct{}

func (packIDs) New(time.Time) (id.UUID, error) { return packCorrelation, nil }

func TestPinPreviewAndProtection(t *testing.T) {
	repo := &packRepository{draft: domain.Draft{TenantID: packTenant, EmployeeID: packEmployee, Version: 1, Status: "draft", Readiness: "incomplete"}}
	svc := New(repo, packIdentity{allowed: true}, packClock{}, packIDs{})
	cmd := api.PinCommand{Actor: identityDomain.Principal{ID: packActor}, TenantID: packTenant, EmployeeID: packEmployee, CorrelationID: packCorrelation, ExpectedVersion: 1, PackID: "auto-parts", PackVersion: "1.0.0", Variants: map[string]any{"parts_interest": "both"}}
	ctx := context.Background()
	d, err := svc.Pin(ctx, cmd)
	if err != nil || d.Pack == nil || d.Pack.Version != "1.0.0" || d.Readiness != "incomplete" {
		t.Fatalf("initial pin: %+v %v", d, err)
	}
	cmd.ExpectedVersion = 2
	cmd.Variants = map[string]any{"parts_interest": "bad"}
	if _, err = svc.Pin(ctx, cmd); !errors.Is(err, domain.ErrInvalid) || repo.writes != 1 {
		t.Fatalf("invalid write: %v %d", err, repo.writes)
	}
	var variantErr domain.VariantError
	if !errors.As(err, &variantErr) || variantErr.Fields["parts_interest"] == "" {
		t.Fatalf("missing field error: %v", err)
	}
	cmd.Variants = map[string]any{"parts_interest": "both", "matching_depth": float64(2)}
	cmd.PackVersion = "2.0.0"
	preview, err := svc.Preview(ctx, cmd)
	if err != nil || preview.Current.Version != "1.0.0" || preview.Impact.Compatibility != "incompatible" || repo.writes != 1 {
		t.Fatalf("preview mutated pin: %+v %v", preview, err)
	}
	if _, err = svc.Pin(ctx, cmd); !errors.Is(err, domain.ErrInvalid) || repo.writes != 1 {
		t.Fatalf("unconfirmed upgrade: %v", err)
	}
	cmd.Confirmed = true
	d, err = svc.Pin(ctx, cmd)
	if err != nil || d.Pack.Version != "2.0.0" || len(d.ReviewSlots) == 0 || repo.writes != 2 {
		t.Fatalf("upgrade: %+v %v", d, err)
	}
	if _, err = svc.Pin(ctx, cmd); !errors.Is(err, domain.ErrConflict) || repo.writes != 2 {
		t.Fatalf("stale write: %v", err)
	}
	repo.draft.ReviewSlots = append(repo.draft.ReviewSlots, "voice")
	cmd.ExpectedVersion = 3
	cmd.PackID = "montagio-services"
	cmd.PackVersion = "1.0.0"
	cmd.Variants = map[string]any{"engineering_plumbing": true, "heating": false}
	cmd.Confirmed = false
	cross, err := svc.Preview(ctx, cmd)
	if err != nil || cross.Impact.Compatibility != "incompatible" || cross.Current.ID != "auto-parts" || repo.writes != 2 {
		t.Fatalf("cross-pack preview: %+v %v", cross, err)
	}
	if _, err = svc.Pin(ctx, cmd); !errors.Is(err, domain.ErrInvalid) || repo.writes != 2 {
		t.Fatalf("unconfirmed cross-pack: %v", err)
	}
	cmd.Confirmed = true
	d, err = svc.Pin(ctx, cmd)
	if err != nil || d.Pack.ID != "montagio-services" || len(d.ReviewSlots) != 4 || d.ReviewSlots[3] != "voice" || repo.writes != 3 {
		t.Fatalf("cross-pack pin: %+v %v", d, err)
	}
	cmd.TenantID = packOtherTenant
	cmd.ExpectedVersion = 4
	if _, err = svc.Pin(ctx, cmd); !errors.Is(err, domain.ErrNotFound) || repo.writes != 3 {
		t.Fatalf("foreign tenant: %v", err)
	}
	operator := New(repo, packIdentity{allowed: false}, packClock{}, packIDs{})
	cmd.TenantID = packTenant
	if _, err = operator.Pin(ctx, cmd); !errors.Is(err, domain.ErrNotFound) || repo.writes != 3 {
		t.Fatalf("operator: %v", err)
	}
}

func TestReverseVersionCanPreviewAndPin(t *testing.T) {
	repo := &packRepository{draft: domain.Draft{TenantID: packTenant, EmployeeID: packEmployee, Version: 3, Status: "draft", Readiness: "incomplete", Pack: &domain.PackPin{ID: "auto-parts", Version: "2.0.0", Variants: map[string]any{"parts_interest": "both", "matching_depth": float64(2)}}}}
	svc := New(repo, packIdentity{allowed: true}, packClock{}, packIDs{})
	cmd := api.PinCommand{Actor: identityDomain.Principal{ID: packActor}, TenantID: packTenant, EmployeeID: packEmployee, CorrelationID: packCorrelation, ExpectedVersion: 3, PackID: "auto-parts", PackVersion: "1.0.0", Variants: map[string]any{"parts_interest": "original"}, Confirmed: true}
	preview, err := svc.Preview(context.Background(), cmd)
	if err != nil || preview.Impact.Compatibility != "incompatible" || repo.writes != 0 {
		t.Fatalf("reverse preview: %+v %v", preview, err)
	}
	d, err := svc.Pin(context.Background(), cmd)
	if err != nil || d.Pack.Version != "1.0.0" || d.Version != 4 || len(d.ReviewSlots) == 0 {
		t.Fatalf("reverse pin: %+v %v", d, err)
	}
}
