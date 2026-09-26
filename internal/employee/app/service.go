package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/packs"
	"github.com/artemiosu/staffy/internal/employee/port"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

type Service struct {
	repository port.Repository
	identity   identityAPI.MembershipService
	clock      api.Clock
	ids        api.IDGenerator
}

func New(repository port.Repository, identity identityAPI.MembershipService, clock api.Clock, ids api.IDGenerator) *Service {
	return &Service{repository, identity, clock, ids}
}

func (s *Service) scope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Configuration); err != nil {
		return tenancy.Context{}, domain.ErrNotFound
	}
	return tenancy.New(actor.ID, tenantID)
}

func (s *Service) Create(ctx context.Context, c api.CreateCommand) (api.CreateResult, error) {
	scope, err := s.scope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return api.CreateResult{}, err
	}
	key := strings.TrimSpace(c.IdempotencyKey)
	if len(key) < 8 || len(key) > 200 || !c.CorrelationID.Valid() {
		return api.CreateResult{}, domain.ErrInvalid
	}
	name := strings.TrimSpace(c.Name)
	if len([]rune(name)) > 120 {
		return api.CreateResult{}, domain.ErrInvalid
	}
	now := s.clock.Now().UTC()
	employeeID, err := s.ids.New(now)
	if err != nil {
		return api.CreateResult{}, err
	}
	eventID, err := s.ids.New(now)
	if err != nil {
		return api.CreateResult{}, err
	}
	hash := sha256.Sum256([]byte("employee.create.v1\x00" + c.TenantID.String() + "\x00" + name))
	draft, replayed, err := s.repository.Create(ctx, port.CreateParams{Scope: scope, EmployeeID: employeeID, CorrelationID: c.CorrelationID, EventID: eventID, Key: key, Hash: hex.EncodeToString(hash[:]), Name: name, Now: now})
	return api.CreateResult{Draft: draft, Replayed: replayed}, err
}

func (s *Service) List(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]domain.Draft, error) {
	scope, err := s.scope(ctx, actor, tenantID)
	if err != nil {
		return nil, err
	}
	return s.repository.List(ctx, scope)
}
func (s *Service) Read(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID) (domain.Draft, error) {
	scope, err := s.scope(ctx, actor, tenantID)
	if err != nil {
		return domain.Draft{}, err
	}
	if !employeeID.Valid() {
		return domain.Draft{}, domain.ErrNotFound
	}
	return s.repository.Read(ctx, scope, employeeID)
}
func (s *Service) Save(ctx context.Context, c api.SaveCommand) (domain.Draft, error) {
	scope, err := s.scope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.Draft{}, err
	}
	if !c.EmployeeID.Valid() || !c.CorrelationID.Valid() || c.ExpectedVersion < 1 || !c.Brief.Valid() {
		return domain.Draft{}, domain.ErrInvalid
	}
	now := s.clock.Now().UTC()
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.Draft{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.Draft{}, err
	}
	return s.repository.Save(ctx, port.SaveParams{Scope: scope, EmployeeID: c.EmployeeID, CorrelationID: c.CorrelationID, EventID: eventID, AuditID: auditID, ExpectedVersion: c.ExpectedVersion, Brief: c.Brief, Now: now})
}

func (s *Service) Catalog(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]domain.PackManifest, error) {
	if _, err := s.scope(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	return packs.List(), nil
}

func (s *Service) Preview(ctx context.Context, c api.PinCommand) (domain.PackPreview, error) {
	if _, err := s.scope(ctx, c.Actor, c.TenantID); err != nil {
		return domain.PackPreview{}, err
	}
	d, err := s.repository.Read(ctx, mustScope(c.Actor.ID, c.TenantID), c.EmployeeID)
	if err != nil {
		return domain.PackPreview{}, err
	}
	if d.Version != c.ExpectedVersion {
		return domain.PackPreview{}, domain.ErrConflict
	}
	if d.Pack == nil {
		return domain.PackPreview{}, domain.ErrInvalid
	}
	m, ok := packs.Get(c.PackID, c.PackVersion)
	if !ok || m.Stage != "setup" {
		return domain.PackPreview{}, domain.ErrInvalid
	}
	impact, ok := packs.Impact(*d.Pack, m)
	if !ok {
		return domain.PackPreview{}, domain.ErrInvalid
	}
	return domain.PackPreview{Current: *d.Pack, Target: m, Impact: impact}, nil
}

func mustScope(actor, tenant id.UUID) tenancy.Context {
	scope, _ := tenancy.New(actor, tenant)
	return scope
}

func (s *Service) Pin(ctx context.Context, c api.PinCommand) (domain.Draft, error) {
	scope, err := s.scope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.Draft{}, err
	}
	if !c.EmployeeID.Valid() || !c.CorrelationID.Valid() || c.ExpectedVersion < 1 {
		return domain.Draft{}, domain.ErrInvalid
	}
	m, ok := packs.Get(c.PackID, c.PackVersion)
	if !ok || m.Stage != "setup" {
		return domain.Draft{}, domain.ErrInvalid
	}
	if fields := domain.ValidateVariants(m, c.Variants); len(fields) > 0 {
		return domain.Draft{}, domain.VariantError{Fields: fields}
	}
	d, err := s.repository.Read(ctx, scope, c.EmployeeID)
	if err != nil {
		return domain.Draft{}, err
	}
	if d.Version != c.ExpectedVersion {
		return domain.Draft{}, domain.ErrConflict
	}
	slots := append([]string(nil), d.ReviewSlots...)
	if d.Pack != nil && (d.Pack.ID != m.ID || d.Pack.Version != m.Version) {
		impact, ok := packs.Impact(*d.Pack, m)
		if !ok || !c.Confirmed {
			return domain.Draft{}, domain.ErrInvalid
		}
		seen := make(map[string]bool, len(slots))
		for _, slot := range slots {
			seen[slot] = true
		}
		for _, slot := range impact.ReviewSlots {
			if !seen[slot] {
				slots = append(slots, slot)
				seen[slot] = true
			}
		}
	}
	now := s.clock.Now().UTC()
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.Draft{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.Draft{}, err
	}
	return s.repository.Pin(ctx, port.PinParams{Scope: scope, EmployeeID: c.EmployeeID, CorrelationID: c.CorrelationID, EventID: eventID, AuditID: auditID, ExpectedVersion: c.ExpectedVersion, Pin: domain.PackPin{ID: m.ID, Version: m.Version, Variants: c.Variants}, ReviewSlots: slots, Now: now})
}
