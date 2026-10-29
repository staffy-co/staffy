package externaltools

// The dictionary service behind the membership ladder (Story 7-4
// step-02): the read gate runs FIRST — a non-member of the tenant is the
// honest ErrNotFound (no disclosure that the tenant has an external tool
// dictionary at all), a member without the integrations|configuration
// scopes is the ErrForbidden carrying the scope words BEFORE any
// dictionary material leaves the service (the 3-1/7-2 ladder precedent).
// The dictionary itself is the pure derivation of the live facts — the
// read never mutates anything and there is no second record of states.

import (
	"context"
	"errors"
	"fmt"

	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// Errors of the dictionary surface (the transport maps them onto the
// problem contract: the plain 404 and the 403 with the scope words).
var (
	ErrNotFound  = errors.New("externaltools: not found")
	ErrForbidden = errors.New("externaltools: forbidden")
)

// dictionaryScopeText is the honest scope refusal: the words of the two
// scopes that DO open the surface, spoken BEFORE any dictionary material.
const dictionaryScopeText = "Чтение словаря внешних инструментов требует полномочия integrations или configuration: словарь read-tools внешних MCP-серверов — материал настройки интеграций и конфигурации, оператору и аудиту он не открывается до вычисления доступности."

// Service is the read-only dictionary surface over the live facts.
type Service struct {
	identity identityAPI.MembershipService
	facts    InventoryFactsSource
}

// New wires the membership ladder and the inventory facts source.
func New(identity identityAPI.MembershipService, facts InventoryFactsSource) *Service {
	return &Service{identity: identity, facts: facts}
}

// authorize runs the membership read gate FIRST (the tools.Service
// precedent of 7-2 verbatim: integrations, then configuration as the
// second honest way in).
func (s *Service) authorize(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) error {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return ErrNotFound
	}
	integrationsErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Integrations)
	if integrationsErr == nil {
		return nil
	}
	if errors.Is(integrationsErr, identityDomain.ErrForbidden) {
		configurationErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Configuration)
		if configurationErr == nil {
			return nil
		}
		if errors.Is(configurationErr, identityDomain.ErrForbidden) {
			return fmt.Errorf("%w: %s", ErrForbidden, dictionaryScopeText)
		}
		if errors.Is(configurationErr, identityDomain.ErrNotFound) {
			return ErrNotFound
		}
		return configurationErr
	}
	if errors.Is(integrationsErr, identityDomain.ErrNotFound) {
		return ErrNotFound
	}
	return integrationsErr
}

// Dictionary answers the derived sections of the tenant (the read gate
// FIRST; the derivation is the pure function of the live facts — the
// honest empty array when no connection of the class exists).
func (s *Service) Dictionary(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]Section, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return nil, ErrNotFound
	}
	inventories, err := s.facts.InventoryFacts(ctx, scope)
	if err != nil {
		return nil, err
	}
	return Derive(inventories), nil
}
