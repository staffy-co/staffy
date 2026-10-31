package api

import (
	"context"

	"github.com/staffy-co/staffy/internal/action/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ActionCatalogService is the read-only catalog surface of the action
// module. The transport type-asserts this narrow interface (the
// HandoffService/DecisionContextService precedent): the app Service
// implements it, and the render DTO of the transport step maps the domain
// definitions onto the view — never the other way around.
type ActionCatalogService interface {
	// List returns the newest version of every action definition of the
	// served dictionary. The read gate runs FIRST: a non-member of the
	// tenant is the honest ErrNotFound (404), a member without the
	// integrations|configuration scopes is the honest ErrForbidden (403)
	// BEFORE any catalog material leaves the service.
	List(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]domain.ActionDefinition, error)

	// Get pins one definition by id and exact version (AC-8: older
	// versions stay immutable and resolvable — the material for future
	// Manifest/Operation/Receipt references). The same read gate runs
	// FIRST; an unknown pin is the honest ErrNotFound.
	Get(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, definitionID, version string) (domain.ActionDefinition, error)
}
