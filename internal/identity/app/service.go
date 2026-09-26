package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"
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

func (s *Service) membershipRepo() (port.MembershipRepository, error) {
	repo, ok := s.repository.(port.MembershipRepository)
	if !ok {
		return nil, domain.ErrForbidden
	}
	return repo, nil
}

func membershipScope(actor domain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	return tenancy.New(actor.ID, tenantID)
}

func (s *Service) ListMembers(ctx context.Context, actor domain.Principal, tenantID id.UUID) ([]domain.Membership, error) {
	scope, err := membershipScope(actor, tenantID)
	if err != nil {
		return nil, err
	}
	repo, err := s.membershipRepo()
	if err != nil {
		return nil, err
	}
	return repo.ListMembers(ctx, scope)
}

func (s *Service) Authorize(ctx context.Context, actor domain.Principal, tenantID id.UUID, required domain.Scope) error {
	scope, err := membershipScope(actor, tenantID)
	if err != nil {
		return err
	}
	repo, err := s.membershipRepo()
	if err != nil {
		return err
	}
	return repo.Authorize(ctx, scope, required)
}

func validMutation(c api.MembershipCommand) bool {
	return c.Actor.ID.Valid() && c.TenantID.Valid() && c.CorrelationID.Valid() && domain.ValidateGrant(c.Roles, c.Scopes)
}

func sensitive(roles []domain.Role) bool {
	for _, role := range roles {
		if role == domain.Owner || role == domain.IntegrationAdmin {
			return true
		}
	}
	return false
}

func (s *Service) InviteMember(ctx context.Context, c api.MembershipCommand) (domain.Membership, string, error) {
	if !validMutation(c) || !strings.Contains(c.Email, "@") || len(c.Email) > 254 {
		return domain.Membership{}, "", domain.ErrInvalidMembership
	}
	if sensitive(c.Roles) && !c.Actor.StepUpVerified {
		return domain.Membership{}, "", domain.ErrStepUpRequired
	}
	c.Reason = domain.ReasonInvitationCreated
	repo, err := s.membershipRepo()
	if err != nil {
		return domain.Membership{}, "", err
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return domain.Membership{}, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	digest := sha256.Sum256([]byte(token))
	now := s.clock.Now().UTC()
	m, err := repo.InviteMember(ctx, port.MembershipMutation{Actor: c.Actor, TenantID: c.TenantID, Email: strings.ToLower(strings.TrimSpace(c.Email)), Roles: c.Roles, Scopes: c.Scopes, Reason: c.Reason, CorrelationID: c.CorrelationID, TokenDigest: hex.EncodeToString(digest[:]), Now: now, ExpiresAt: now.Add(7 * 24 * time.Hour)})
	if err != nil {
		return domain.Membership{}, "", err
	}
	return m, token, nil
}

func (s *Service) AcceptInvitation(ctx context.Context, actor domain.Principal, token string, correlationID id.UUID) (domain.Membership, error) {
	if !actor.ID.Valid() || actor.VerifiedEmail == "" || !correlationID.Valid() || len(token) < 40 {
		return domain.Membership{}, domain.ErrNotFound
	}
	repo, err := s.membershipRepo()
	if err != nil {
		return domain.Membership{}, err
	}
	digest := sha256.Sum256([]byte(token))
	return repo.AcceptInvitation(ctx, actor, hex.EncodeToString(digest[:]), correlationID, s.clock.Now().UTC())
}

func (s *Service) ChangeMember(ctx context.Context, c api.MembershipCommand) (domain.Membership, error) {
	if !validMutation(c) || !c.TargetID.Valid() || c.ExpectedVersion < 1 || !domain.ValidChangeReason(c.Reason) {
		return domain.Membership{}, domain.ErrInvalidMembership
	}
	if !c.Actor.StepUpVerified {
		return domain.Membership{}, domain.ErrStepUpRequired
	}
	repo, err := s.membershipRepo()
	if err != nil {
		return domain.Membership{}, err
	}
	return repo.ChangeMember(ctx, port.MembershipMutation{Actor: c.Actor, TenantID: c.TenantID, TargetID: c.TargetID, Roles: c.Roles, Scopes: c.Scopes, ExpectedVersion: c.ExpectedVersion, Reason: c.Reason, CorrelationID: c.CorrelationID, Now: s.clock.Now().UTC()})
}

func (s *Service) RevokeMember(ctx context.Context, c api.MembershipCommand) error {
	if !c.Actor.ID.Valid() || !c.TargetID.Valid() || !c.TenantID.Valid() || !c.CorrelationID.Valid() || c.ExpectedVersion < 1 || !domain.ValidRevokeReason(c.Reason) {
		return domain.ErrInvalidMembership
	}
	if !c.Actor.StepUpVerified {
		return domain.ErrStepUpRequired
	}
	repo, err := s.membershipRepo()
	if err != nil {
		return err
	}
	return repo.RevokeMember(ctx, port.MembershipMutation{Actor: c.Actor, TenantID: c.TenantID, TargetID: c.TargetID, ExpectedVersion: c.ExpectedVersion, Reason: c.Reason, CorrelationID: c.CorrelationID, Now: s.clock.Now().UTC()})
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
