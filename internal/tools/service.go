package tools

import (
	"context"
	"errors"
	"fmt"

	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Errors of the tool catalog surface. Authorization failures of members
// surface as ErrForbidden (an honest refusal with the scope words);
// unknown tenants/non-members as ErrNotFound (no disclosure that the
// tenant has a tool catalog at all).
var (
	ErrNotFound  = errors.New("tools: not found")
	ErrForbidden = errors.New("tools: forbidden")
)

// toolsCatalogScopeText is the honest scope refusal of the catalog read:
// the words of the two scopes that DO open the surface, spoken BEFORE any
// catalog material leaves the service (the Story 3.1 precedent).
const toolsCatalogScopeText = "Чтение реестра инструментов требует полномочия integrations или configuration: словарь типизированных контрактов — материал настройки интеграций и конфигурации, оператору и аудиту он не открывается до вычисления доступности."

// Service is the read-only catalog surface over the embedded tool
// registry: the list of the newest version of every contract and the pin
// read of one exact version (the material for the future manifest and
// planner references — Planner 8.3 / MCP 7.3). The registry is global and
// embedded; the surface sits behind the tenant membership ladder exactly
// like the action dictionary of Story 3.1: the read gate runs FIRST.
type Service struct {
	identity identityAPI.MembershipService
}

// NewCatalogService wires the narrow identity membership port (the read
// gate); the registry itself is the package's embedded state.
func NewCatalogService(identity identityAPI.MembershipService) *Service {
	return &Service{identity: identity}
}

// authorize runs the membership read gate FIRST: a non-member of the
// tenant is the honest ErrNotFound, a member without the
// integrations|configuration scopes is the ErrForbidden carrying the
// scope words BEFORE any catalog material (the Story 3.1 ladder).
func (s *Service) authorize(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) error {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return ErrNotFound
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
			return fmt.Errorf("%w: %s", ErrForbidden, toolsCatalogScopeText)
		}
		if errors.Is(configurationErr, identityDomain.ErrNotFound) {
			// The membership vanished between the two calls: the honest 404.
			return ErrNotFound
		}
		return configurationErr
	}
	if errors.Is(integrationsErr, identityDomain.ErrNotFound) {
		// A non-member of the tenant: the module's own 404 identity.
		return ErrNotFound
	}
	return integrationsErr
}

// List returns the newest version of every published contract (the read
// gate FIRST; the caller receives deep copies — the registry stays
// immutable behind the catalog).
func (s *Service) List(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]ToolContract, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	published := List()
	newest := map[string]ToolContract{}
	order := make([]string, 0, len(published))
	for _, contract := range published {
		current, seen := newest[contract.ID]
		if !seen {
			newest[contract.ID] = contract
			order = append(order, contract.ID)
			continue
		}
		// The newest version of one id wins by the honest semver
		// comparison — never the index position (the registry grows by
		// appends; the insertion point is not a version statement).
		if semverGreater(contract.Version, current.Version) {
			newest[contract.ID] = contract
		}
	}
	out := make([]ToolContract, 0, len(order))
	for _, contractID := range order {
		out = append(out, newest[contractID])
	}
	return out, nil
}

// Get pins one contract by id and exact version (the material for the
// future manifest/planner references). The read gate runs FIRST; an
// unknown pin is the honest ErrNotFound — the pinned past stays
// resolvable, the invented future does not exist.
func (s *Service) Get(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, toolID, version string) (ToolContract, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return ToolContract{}, err
	}
	contract, ok := Get(toolID, version)
	if !ok {
		return ToolContract{}, ErrNotFound
	}
	return contract, nil
}

// semverGreater reports whether version a is strictly newer than b in the
// semantic core order (both are already validated semver-core words of
// the registry).
func semverGreater(a, b string) bool {
	amajor, aminor, apatch, errA := parseSemver(a)
	bmajor, bminor, bpatch, errB := parseSemver(b)
	if errA != nil || errB != nil {
		return false
	}
	if amajor != bmajor {
		return amajor > bmajor
	}
	if aminor != bminor {
		return aminor > bminor
	}
	return apatch > bpatch
}
