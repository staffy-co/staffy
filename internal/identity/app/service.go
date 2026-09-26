package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/identity/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

type Service struct {
	repository port.Repository
	clock      api.Clock
	ids        api.IDGenerator
}

func New(repository port.Repository, clock api.Clock, ids api.IDGenerator) *Service {
	return &Service{repository: repository, clock: clock, ids: ids}
}

func (s *Service) CreateTenant(ctx context.Context, command api.CreateTenantCommand) (api.CreateTenantResult, error) {
	if !command.Principal.ID.Valid() || !command.Principal.CanCreateTenant {
		return api.CreateTenantResult{}, domain.ErrForbidden
	}
	name := strings.TrimSpace(command.Name)
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 160 || !hasLetterOrNumber(name) {
		return api.CreateTenantResult{}, domain.ErrInvalidName
	}
	key := strings.TrimSpace(command.IdempotencyKey)
	if utf8.RuneCountInString(key) < 8 || utf8.RuneCountInString(key) > 200 {
		return api.CreateTenantResult{}, domain.ErrInvalidIdempotency
	}
	if !command.CorrelationID.Valid() {
		return api.CreateTenantResult{}, domain.ErrForbidden
	}
	digest := sha256.Sum256([]byte("create_tenant\x00" + name))
	requestHash := hex.EncodeToString(digest[:])
	if tenant, found, err := s.repository.ReplayTenant(ctx, command.Principal, key, requestHash); err != nil {
		return api.CreateTenantResult{}, err
	} else if found {
		return api.CreateTenantResult{Tenant: tenant, Role: "Owner", Replayed: true}, nil
	}

	now := s.clock.Now().UTC()
	tenantID, err := s.ids.New(now)
	if err != nil {
		return api.CreateTenantResult{}, err
	}
	eventID, err := s.ids.New(now)
	if err != nil {
		return api.CreateTenantResult{}, err
	}
	tenant, replayed, err := s.repository.CreateTenant(ctx, port.CreateTenantParams{
		Principal:      command.Principal,
		TenantID:       tenantID,
		Name:           name,
		IdempotencyKey: key,
		RequestHash:    requestHash,
		CorrelationID:  command.CorrelationID,
		EventID:        eventID,
		OccurredAt:     now,
	})
	if err != nil {
		return api.CreateTenantResult{}, err
	}
	return api.CreateTenantResult{Tenant: tenant, Role: "Owner", Replayed: replayed}, nil
}

func hasLetterOrNumber(value string) bool {
	for _, current := range value {
		if unicode.IsLetter(current) || unicode.IsNumber(current) {
			return true
		}
	}
	return false
}

func (s *Service) Workspace(ctx context.Context, principal domain.Principal, tenantID id.UUID) (domain.Workspace, error) {
	if !principal.ID.Valid() || !tenantID.Valid() {
		return domain.Workspace{}, domain.ErrNotFound
	}
	scope, err := tenancy.New(principal.ID, tenantID)
	if err != nil {
		return domain.Workspace{}, err
	}
	return s.repository.Workspace(ctx, scope)
}
