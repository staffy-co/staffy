package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/identity/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

var (
	principalID = id.MustParse("01994f64-6c00-7000-8000-000000000001")
	tenantID    = id.MustParse("01994f64-6c00-7000-8000-000000000002")
	correlation = id.MustParse("01994f64-6c00-7000-8000-000000000003")
)

type fixedClock struct{ value time.Time }

func (c fixedClock) Now() time.Time { return c.value }

type fixedIDs struct{}

func (fixedIDs) New(time.Time) (id.UUID, error) { return tenantID, nil }

type fakeRepository struct {
	params port.CreateTenantParams
	err    error
	replay domain.Tenant
	found  bool
}

func (r *fakeRepository) ReplayTenant(context.Context, domain.Principal, string, string) (domain.Tenant, bool, error) {
	return r.replay, r.found, r.err
}

func (r *fakeRepository) CreateTenant(_ context.Context, params port.CreateTenantParams) (domain.Tenant, bool, error) {
	r.params = params
	return domain.Tenant{ID: params.TenantID, Name: params.Name}, false, r.err
}
func (r *fakeRepository) Workspace(context.Context, tenancy.Context) (domain.Workspace, error) {
	return domain.Workspace{}, r.err
}

func TestCreateTenantNormalizesAndAuthorizes(t *testing.T) {
	repository := &fakeRepository{}
	service := New(repository, fixedClock{time.Unix(1, 0)}, fixedIDs{})
	result, err := service.CreateTenant(context.Background(), api.CreateTenantCommand{
		Principal: domain.Principal{ID: principalID, CanCreateTenant: true},
		Name:      "  Магазин Олега  ", IdempotencyKey: "request-0001", CorrelationID: correlation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Tenant.ID != tenantID || repository.params.Name != "Магазин Олега" || repository.params.RequestHash == "" {
		t.Fatalf("unexpected result: %#v, params: %#v", result, repository.params)
	}
}

func TestCreateTenantRejectsMissingPermission(t *testing.T) {
	service := New(&fakeRepository{}, fixedClock{}, fixedIDs{})
	_, err := service.CreateTenant(context.Background(), api.CreateTenantCommand{
		Principal: domain.Principal{ID: principalID}, Name: "Staffy", IdempotencyKey: "request-0001", CorrelationID: correlation,
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("error = %v, want forbidden", err)
	}
}

func TestCreateTenantRejectsVisuallyBlankNameAndCountsUnicodeKeyCharacters(t *testing.T) {
	repository := &fakeRepository{}
	service := New(repository, fixedClock{time.Unix(1, 0)}, fixedIDs{})
	_, err := service.CreateTenant(context.Background(), api.CreateTenantCommand{
		Principal: domain.Principal{ID: principalID, CanCreateTenant: true},
		Name:      "\u200b\u200b", IdempotencyKey: "request-0001", CorrelationID: correlation,
	})
	if !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("error = %v, want invalid name", err)
	}
	_, err = service.CreateTenant(context.Background(), api.CreateTenantCommand{
		Principal: domain.Principal{ID: principalID, CanCreateTenant: true},
		Name:      "Staffy", IdempotencyKey: "ключ-абв", CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("unicode key rejected: %v", err)
	}
}

type failingIDs struct{}

func (failingIDs) New(time.Time) (id.UUID, error) { return "", errors.New("entropy unavailable") }

func TestCreateTenantReplayDoesNotNeedFreshIDs(t *testing.T) {
	want := domain.Tenant{ID: tenantID, Name: "Staffy"}
	service := New(&fakeRepository{replay: want, found: true}, fixedClock{}, failingIDs{})
	result, err := service.CreateTenant(context.Background(), api.CreateTenantCommand{
		Principal: domain.Principal{ID: principalID, CanCreateTenant: true},
		Name:      "Staffy", IdempotencyKey: "request-0001", CorrelationID: correlation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.Tenant != want {
		t.Fatalf("result = %#v", result)
	}
}
