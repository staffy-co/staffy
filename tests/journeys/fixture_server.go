package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/packs"
	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	httptransport "github.com/artemiosu/staffy/internal/transport/http"
)

var (
	principalID = id.MustParse("01994f64-6c00-7000-8000-000000000001")
	tenantID    = id.MustParse("01994f64-6c00-7000-8000-000000000002")
)

type identityFixture struct{}

func (identityFixture) CreateTenant(context.Context, api.CreateTenantCommand) (api.CreateTenantResult, error) {
	return api.CreateTenantResult{Tenant: domain.Tenant{ID: tenantID, Name: "Магазин Олега"}, Role: "Owner"}, nil
}
func (identityFixture) Workspace(context.Context, domain.Principal, id.UUID) (domain.Workspace, error) {
	return domain.Workspace{Tenant: domain.Tenant{ID: tenantID, Name: "Магазин Олега"}, Role: "owner"}, nil
}
func (identityFixture) ListMembers(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error) {
	return []domain.Membership{{TenantID: tenantID, PrincipalID: &principalID, Roles: []domain.Role{domain.Owner}, Scopes: domain.AllScopes(), Status: "active", Version: 1}}, nil
}
func (identityFixture) InviteMember(context.Context, api.MembershipCommand) (domain.Membership, string, error) {
	return domain.Membership{}, "", domain.ErrStepUpRequired
}
func (identityFixture) AcceptInvitation(context.Context, domain.Principal, string, id.UUID) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrNotFound
}
func (identityFixture) ChangeMember(context.Context, api.MembershipCommand) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrStepUpRequired
}
func (identityFixture) RevokeMember(context.Context, api.MembershipCommand) error {
	return domain.ErrStepUpRequired
}
func (identityFixture) Authorize(context.Context, domain.Principal, id.UUID, domain.Scope) error {
	return nil
}

type resolverFixture struct{}

func (resolverFixture) Resolve(context.Context) (domain.Principal, error) {
	return domain.Principal{ID: principalID, CanCreateTenant: true}, nil
}

type clockFixture struct{}

func (clockFixture) Now() time.Time { return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC) }

type idsFixture struct{}

func (idsFixture) New(time.Time) (id.UUID, error) {
	return id.MustParse("01994f64-6c00-7000-8000-000000000003"), nil
}

type employeeFixture struct {
	mu    sync.Mutex
	draft *employeeDomain.Draft
}

var fixtureEmployeeID = id.MustParse("01994f64-6c00-7000-8000-000000000004")

func (f *employeeFixture) Create(_ context.Context, _ employeeAPI.CreateCommand) (employeeAPI.CreateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil {
		f.draft = &employeeDomain.Draft{TenantID: tenantID, EmployeeID: fixtureEmployeeID, OwnerID: principalID, Status: "draft", Readiness: "incomplete", Version: 1, CurrentStep: "brief", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	}
	return employeeAPI.CreateResult{Draft: *f.draft}, nil
}
func (f *employeeFixture) List(_ context.Context, _ domain.Principal, _ id.UUID) ([]employeeDomain.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil {
		return []employeeDomain.Draft{}, nil
	}
	return []employeeDomain.Draft{*f.draft}, nil
}
func (f *employeeFixture) Read(_ context.Context, _ domain.Principal, _ id.UUID, employeeID id.UUID) (employeeDomain.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil || employeeID != fixtureEmployeeID {
		return employeeDomain.Draft{}, employeeDomain.ErrNotFound
	}
	return *f.draft, nil
}
func (f *employeeFixture) Save(_ context.Context, c employeeAPI.SaveCommand) (employeeDomain.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil || c.EmployeeID != fixtureEmployeeID {
		return employeeDomain.Draft{}, employeeDomain.ErrNotFound
	}
	if c.ExpectedVersion != f.draft.Version {
		return employeeDomain.Draft{}, employeeDomain.ErrConflict
	}
	if !c.Brief.Valid() {
		return employeeDomain.Draft{}, employeeDomain.ErrInvalid
	}
	f.draft.Version++
	f.draft.Brief = c.Brief
	f.draft.CurrentStep = "launch_plan"
	f.draft.UpdatedAt = time.Now().UTC()
	return *f.draft, nil
}
func (f *employeeFixture) Catalog(context.Context, domain.Principal, id.UUID) ([]employeeDomain.PackManifest, error) {
	return packs.List(), nil
}
func (f *employeeFixture) Preview(_ context.Context, c employeeAPI.PinCommand) (employeeDomain.PackPreview, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := packs.Get(c.PackID, c.PackVersion)
	if !ok || f.draft == nil || f.draft.Pack == nil {
		return employeeDomain.PackPreview{}, employeeDomain.ErrInvalid
	}
	impact, allowed := packs.Impact(*f.draft.Pack, m)
	if !allowed {
		return employeeDomain.PackPreview{}, employeeDomain.ErrInvalid
	}
	return employeeDomain.PackPreview{Current: *f.draft.Pack, Target: m, Impact: impact}, nil
}
func (f *employeeFixture) Pin(_ context.Context, c employeeAPI.PinCommand) (employeeDomain.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.draft == nil {
		return employeeDomain.Draft{}, employeeDomain.ErrNotFound
	}
	if c.ExpectedVersion != f.draft.Version {
		return employeeDomain.Draft{}, employeeDomain.ErrConflict
	}
	m, ok := packs.Get(c.PackID, c.PackVersion)
	if !ok || len(employeeDomain.ValidateVariants(m, c.Variants)) > 0 {
		return employeeDomain.Draft{}, employeeDomain.ErrInvalid
	}
	if f.draft.Pack != nil && (f.draft.Pack.ID != c.PackID || f.draft.Pack.Version != c.PackVersion) {
		impact, ok := packs.Impact(*f.draft.Pack, m)
		if !ok || !c.Confirmed {
			return employeeDomain.Draft{}, employeeDomain.ErrInvalid
		}
		f.draft.ReviewSlots = impact.ReviewSlots
	}
	f.draft.Pack = &employeeDomain.PackPin{ID: c.PackID, Version: c.PackVersion, Variants: c.Variants}
	f.draft.Version++
	return *f.draft, nil
}

func main() {
	server, err := httptransport.New(identityFixture{}, resolverFixture{}, idsFixture{}, clockFixture{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	server.BindEmployee(&employeeFixture{})
	if err := http.ListenAndServe("127.0.0.1:18081", server.Handler()); err != nil {
		panic(err)
	}
}
