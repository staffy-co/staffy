package port

import (
	"context"

	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// RouteFactsRepository is the persistence the RouteFacts facade reads:
// the tenant-wide admission history (the facade derives the acting
// decision per employee itself), the sealed manifest content behind an
// exact manifest link and one approved profile version by its number.
// The reads are caller-authority: the consumer verified its scope.
type RouteFactsRepository interface {
	ListTenantAdmissions(ctx context.Context, scope tenancy.Context) ([]domain.ReleaseAdmission, error)
	ReadManifestContent(ctx context.Context, scope tenancy.Context, employeeID id.UUID, link domain.ManifestLink) (domain.ManifestContent, error)
	ReadApprovedProfileVersion(ctx context.Context, scope tenancy.Context, employeeID id.UUID, version int64) (domain.ProfileVersion, error)
}
