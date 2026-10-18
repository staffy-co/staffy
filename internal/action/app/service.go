package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/artemiosu/staffy/internal/action/api"
	actionCatalog "github.com/artemiosu/staffy/internal/action/catalog"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// actionCatalogScopeText is the honest scope refusal of the catalog read:
// the words of the two scopes that DO open the surface, spoken BEFORE any
// catalog material (AC-9). The action dictionary is configuration and
// integration material — an operator or audit member reads the working and
// audit contours, never the setup material (the decision-context 2-14
// precedent of the words-first 403).
const actionCatalogScopeText = "Чтение каталога действий требует полномочия integrations или configuration: словарь действий — материал настройки интеграций и конфигурации, оператору и аудиту он не открывается до вычисления доступности."

// The compile-time proof of the narrow interface registration: the
// transport type-asserts api.ActionCatalogService (the HandoffService
// precedent).
var _ api.ActionCatalogService = (*Service)(nil)

// Service is the read-only CatalogService of the action module (step-02:
// ports, resolvers, the app layer). It owns NO store — the definition
// dictionary is compile-time; the service gates the read, resolves nothing
// on the read path (the admission pass is a separate, test-facing surface)
// and serves the injected dictionary with the 1-8 pin semantics.
type Service struct {
	identity identityAPI.MembershipService
	packs    domain.PackRefResolver
	caps     domain.CapabilityRefResolver
	defs     []domain.ActionDefinition
}

// New wires the service. The resolvers are the composition-root adapters
// over the live pack and connector catalogs (the ports). defs is the
// definition dictionary this instance serves: the PRODUCTION composition
// root passes the binary catalog (honestly empty today, AG-4); the TEST
// composition root registers fixture definitions (the vertical of the
// slice — fixtures never live in the product binary). The slice is treated
// as immutable from here on; every read returns a defensive copy.
func New(identity identityAPI.MembershipService, packs port.PackRefResolver, caps port.CapabilityRefResolver, defs []domain.ActionDefinition) *Service {
	return &Service{identity: identity, packs: packs, caps: caps, defs: defs}
}

// authorize is the read gate, in the honest order:
//
//  1. membership FIRST through the identity port — a non-member of the
//     tenant gets the 404 (Authorize answers ErrNotFound for a missing
//     active membership; no disclosure that the tenant has an action
//     catalog at all);
//  2. the scope words 403 BEFORE any catalog material — a member holding
//     NEITHER integrations NOR configuration reads nothing: the honest
//     refusal names the two scopes that would open the surface.
//
// The member-with-integrations path is one Authorize call; only the
// integrations-less member pays the second (configuration) call.
func (s *Service) authorize(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) error {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return domain.ErrNotFound
	}
	integrationsErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Integrations)
	if integrationsErr == nil {
		return nil
	}
	if errors.Is(integrationsErr, identityDomain.ErrForbidden) {
		// The member exists: configuration is the second honest way in.
		configurationErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Configuration)
		if configurationErr == nil {
			return nil
		}
		if errors.Is(configurationErr, identityDomain.ErrForbidden) {
			return fmt.Errorf("%w: %s", domain.ErrForbidden, actionCatalogScopeText)
		}
		if errors.Is(configurationErr, identityDomain.ErrNotFound) {
			// The membership vanished between the two calls: the honest 404.
			return domain.ErrNotFound
		}
		return configurationErr
	}
	if errors.Is(integrationsErr, identityDomain.ErrNotFound) {
		// A non-member of the tenant: the module's own 404 identity (the
		// transport maps the action domain error, never the identity one).
		return domain.ErrNotFound
	}
	return integrationsErr
}

// List serves the newest version of every definition of the injected
// dictionary (the read gate FIRST; a defensive copy — the caller may
// mutate the result, never the dictionary).
func (s *Service) List(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]domain.ActionDefinition, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return actionCatalog.ListFrom(s.defs), nil
}

// Get pins one definition by id and exact version (AC-8). The read gate
// runs FIRST; an unknown or unparseable pin is the honest ErrNotFound —
// the pinned past stays resolvable, the invented future does not exist.
func (s *Service) Get(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, definitionID, version string) (domain.ActionDefinition, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return domain.ActionDefinition{}, err
	}
	def, ok := actionCatalog.GetFrom(s.defs, definitionID, version)
	if !ok {
		return domain.ActionDefinition{}, domain.ErrNotFound
	}
	return def, nil
}

// ValidateBinaryCatalog walks the PRODUCTION binary dictionary through the
// full admission pass: the guard integrity (CatalogIntegrity — every entry
// individually guarded, pin pairs unique, versions parseable) plus the
// reference resolution and the delivery-word cross-check of every entry
// (ValidateDefinitionRefs) through the live resolvers. The honest empty
// catalog passes vacuously (AG-4); the integration test keeps this pass
// live so the first real entry can only appear through the full admission
// truth. Nothing here touches the injected fixture dictionary — the
// product binary is the subject.
func (s *Service) ValidateBinaryCatalog() error {
	defs := actionCatalog.All()
	if err := domain.CatalogIntegrity(defs); err != nil {
		return err
	}
	for _, d := range defs {
		if err := domain.ValidateDefinitionRefs(d, s.packs, s.caps); err != nil {
			return err
		}
	}
	return nil
}
