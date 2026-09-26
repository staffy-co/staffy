package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

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

func main() {
	server, err := httptransport.New(identityFixture{}, resolverFixture{}, idsFixture{}, clockFixture{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	if err := http.ListenAndServe("127.0.0.1:18081", server.Handler()); err != nil {
		panic(err)
	}
}
