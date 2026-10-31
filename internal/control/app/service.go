package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/staffy-co/staffy/internal/control/api"
	"github.com/staffy-co/staffy/internal/control/domain"
	"github.com/staffy-co/staffy/internal/control/port"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// controlsScopeText is the honest scope refusal of the control surfaces:
// the words of the ONE scope that opens them, spoken BEFORE any control
// material (AC-1). Scoped controls are the operational emergency surface
// (FR-32): a member holding any other scope reads nothing — neither the
// list, nor the effective computation, nor the mutation confirmation.
const controlsScopeText = "Поверхность аварийного ограничения работы требует полномочия operations: точечная остановка и возобновление маршрутов — операторский материал, участнику без operations он не открывается до вычисления итогового режима."

// The compile-time proof of the narrow interface registration: the
// transport type-asserts api.ControlService (the HandoffService
// precedent).
var _ api.ControlService = (*Service)(nil)

// Service is the application core of the control module (step-02):
// the gate ladder, the atomic Set/Resume, the reads. It owns NO store of
// its own — every row, audit line and outbox event lands through the
// repository port inside the repository's single transaction; the
// connector registry is the consumer-owned port (the composition root
// adapts the live 1-8 catalog in step-03's wiring). A nil connector
// resolver FAILS CLOSED: the connector kind is refused with an honest
// cause until the live registry is wired (the nil-resolver discipline of
// the action module 3-1).
type Service struct {
	identity   identityAPI.MembershipService
	connectors port.ConnectorRefResolver
	repository port.Repository
	clock      api.Clock
	ids        api.IDGenerator
}

// New wires the service. clock/ids follow the employee Service precedent
// (the module api Clock/IDGenerator contracts): the clock answers Now()
// for every mutation; the generator mints the audit, outbox and
// correlation identifiers.
func New(repository port.Repository, identity identityAPI.MembershipService, connectors port.ConnectorRefResolver, clock api.Clock, ids api.IDGenerator) *Service {
	return &Service{identity: identity, connectors: connectors, repository: repository, clock: clock, ids: ids}
}

// authorize is the READ gate ladder, in the honest order:
//
//  1. membership FIRST through the identity port — a non-member of the
//     tenant (or an invalid actor/tenant pair) gets the module's own 404
//     identity BEFORE any control material (the surface is not
//     disclosed);
//  2. the operations scope 403 IN WORDS before any material: a member
//     without operations reads nothing; the refusal names the one scope
//     that would open the surface (Operator/Owner — FR-32 personnel).
//
// Step-up is NOT a read gate: the operator pages and the effective
// computation are read surfaces; the confirmation discipline guards the
// MUTATIONS only (Set/Resume).
func (s *Service) authorize(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) error {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return domain.ErrNotFound
	}
	err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Operations)
	if err == nil {
		return nil
	}
	if errors.Is(err, identityDomain.ErrForbidden) {
		// The member exists without the operations scope: the honest 403
		// with the words of the surface, BEFORE any material.
		return fmt.Errorf("%w: %s", domain.ErrForbidden, controlsScopeText)
	}
	if errors.Is(err, identityDomain.ErrNotFound) {
		// A non-member of the tenant: the module's own 404 (the transport
		// maps the control domain error, never the identity one).
		return domain.ErrNotFound
	}
	return err
}

// authorizeMutation runs the READ ladder first, then the step-up gate:
// the dangerous operational surface (AC-4, the ChangeMember identity
// precedent). An operator without a fresh confirmation answers the
// step-up requirement — the transport maps it to the 403 with the
// step-up words.
func (s *Service) authorizeMutation(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) error {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return err
	}
	if !actor.StepUpVerified {
		return identityDomain.ErrStepUpRequired
	}
	return nil
}

// resolveConnectorRef is the LIVE half of the connector-kind honesty: the
// lexical shape was already guarded; here the ref must resolve in the
// tenant's LIVE registry (1-8). A non-resolving ref is refused with an
// honest cause — fail-closed; an infrastructure failure is returned
// loud, never silently answered as "not found" (a broken registry
// refuses honestly). A nil resolver fails closed the same way: the
// honest boundary until the composition root wires the live adapter.
func (s *Service) resolveConnectorRef(ctx context.Context, tenantID id.UUID, ref string) error {
	if s.connectors == nil {
		return domain.RuleError{Rule: "scope_ref", Reason: "реестр коннекторов не сконфигурирован: скоуп connector отказан честно (fail-closed)"}
	}
	resolved, err := s.connectors.Resolve(ctx, tenantID, ref)
	if err != nil {
		return err
	}
	if !resolved {
		return domain.RuleError{Rule: "scope_ref", Reason: "коннектор не найден в живом реестре тенента: " + ref}
	}
	return nil
}

// mintPair mints the audit and outbox identifiers of one mutation. The
// correlation id is handled separately: the request's own correlation
// (the platform correlation context via the transport) wins; a missing
// one is minted fresh — the mutation never loses its correlation trail.
func (s *Service) mintPair(now time.Time) (auditID, eventID id.UUID, err error) {
	auditID, err = s.ids.New(now)
	if err != nil {
		return id.UUID(""), id.UUID(""), fmt.Errorf("generate audit id: %w", err)
	}
	eventID, err = s.ids.New(now)
	if err != nil {
		return id.UUID(""), id.UUID(""), fmt.Errorf("generate event id: %w", err)
	}
	return auditID, eventID, nil
}

// correlationOf answers the correlation id of one mutation: the request's
// own when present, a fresh mint otherwise.
func (s *Service) correlationOf(requested id.UUID, now time.Time) (id.UUID, error) {
	if requested.Valid() {
		return requested, nil
	}
	correlationID, err := s.ids.New(now)
	if err != nil {
		return id.UUID(""), fmt.Errorf("generate correlation id: %w", err)
	}
	return correlationID, nil
}

// Set installs or re-installs a control point. The gate ladder, in the
// honest order (the spec's mandated order):
//
//  1. membership (404 before material) — authorizeMutation;
//  2. the operations scope (403 in words before material);
//  3. step-up (ErrStepUpRequired);
//  4. the dictionary guard (kind, ref, mode, reason, window — RuleError
//     BEFORE the transaction: a rejected mutation writes no audit row);
//  5. the LIVE connector resolution (the connector kind only);
//  6. the repository transaction: the row, the audit row, the outbox
//     event — atomically; a generation mismatch answers the typed
//     conflict with the CURRENT material (AC-11).
func (s *Service) Set(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, request api.SetRequest) (domain.ScopedControl, error) {
	if err := s.authorizeMutation(ctx, actor, tenantID); err != nil {
		return domain.ScopedControl{}, err
	}
	if err := domain.GuardValidateSet(request.Kind, request.Ref, tenantID.String(), request.Mode, request.Reason, request.ScheduledStartAt, request.ExpiresAt); err != nil {
		return domain.ScopedControl{}, err
	}
	if request.Kind == domain.ScopeKindConnector {
		if err := s.resolveConnectorRef(ctx, tenantID, request.Ref); err != nil {
			return domain.ScopedControl{}, err
		}
	}
	now := s.clock.Now()
	auditID, eventID, err := s.mintPair(now)
	if err != nil {
		return domain.ScopedControl{}, err
	}
	correlationID, err := s.correlationOf(request.CorrelationID, now)
	if err != nil {
		return domain.ScopedControl{}, err
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return domain.ScopedControl{}, err
	}
	return s.repository.Set(ctx, port.SetParams{
		Scope:              scope,
		Kind:               request.Kind,
		Ref:                request.Ref,
		Mode:               request.Mode,
		Reason:             request.Reason,
		ExpectedGeneration: request.ExpectedGeneration,
		ScheduledStartAt:   request.ScheduledStartAt,
		ExpiresAt:          request.ExpiresAt,
		AuditID:            auditID,
		EventID:            eventID,
		CorrelationID:      correlationID,
		Now:                now,
	})
}

// Resume lifts a control point (state removed, the generation grows, the
// history stays). The same gate ladder as Set; the dictionary guard of a
// lift is the scope pair plus the reason (no mode, no window).
func (s *Service) Resume(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, request api.ResumeRequest) (domain.ScopedControl, error) {
	if err := s.authorizeMutation(ctx, actor, tenantID); err != nil {
		return domain.ScopedControl{}, err
	}
	if err := domain.ValidScopeRef(request.Kind, request.Ref, tenantID.String()); err != nil {
		return domain.ScopedControl{}, err
	}
	if err := domain.ValidReason(request.Reason); err != nil {
		return domain.ScopedControl{}, err
	}
	if request.Kind == domain.ScopeKindConnector {
		if err := s.resolveConnectorRef(ctx, tenantID, request.Ref); err != nil {
			return domain.ScopedControl{}, err
		}
	}
	now := s.clock.Now()
	auditID, eventID, err := s.mintPair(now)
	if err != nil {
		return domain.ScopedControl{}, err
	}
	correlationID, err := s.correlationOf(request.CorrelationID, now)
	if err != nil {
		return domain.ScopedControl{}, err
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return domain.ScopedControl{}, err
	}
	return s.repository.Resume(ctx, port.ResumeParams{
		Scope:              scope,
		Kind:               request.Kind,
		Ref:                request.Ref,
		Reason:             request.Reason,
		ExpectedGeneration: request.ExpectedGeneration,
		AuditID:            auditID,
		EventID:            eventID,
		CorrelationID:      correlationID,
		Now:                now,
	})
}

// List returns every control point of the tenant — both states: the
// removed points stay readable (the history surface, the generation
// series continues). The read gate ladder (no step-up: reading never
// changes the operational truth).
func (s *Service) List(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]domain.ScopedControl, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return nil, err
	}
	return s.repository.List(ctx, scope)
}

// EffectiveFor computes the most-restrictive intersection of the controls
// matching the route scope keys (AC-3): the read gate ladder, the
// dictionary discipline of every key (an out-of-dictionary kind or a
// mis-shaped ref is a refusal, not a silent empty match), the honest
// upper bound of the key count, then ONE repository read and the domain
// computation. Absent controls mean accept — the route is not
// restricted; the enforcement points of the future consume this surface
// together with the generations (the typed read API of the slice).
func (s *Service) EffectiveFor(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, keys []domain.ScopeKey) (domain.EffectiveControl, error) {
	if err := s.authorize(ctx, actor, tenantID); err != nil {
		return domain.EffectiveControl{}, err
	}
	if len(keys) > domain.MaxScopeKeys {
		return domain.EffectiveControl{}, domain.RuleError{Rule: "scope_keys", Reason: "слишком много скоуп-ключей маршрута: максимум 16"}
	}
	deduplicated := make([]domain.ScopeKey, 0, len(keys))
	seen := make(map[domain.ScopeKey]bool)
	for _, key := range keys {
		if err := domain.ValidScopeRef(key.Kind, key.Ref, tenantID.String()); err != nil {
			return domain.EffectiveControl{}, err
		}
		if !seen[key] {
			seen[key] = true
			deduplicated = append(deduplicated, key)
		}
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return domain.EffectiveControl{}, err
	}
	controls, err := s.repository.LoadKeys(ctx, scope, deduplicated)
	if err != nil {
		return domain.EffectiveControl{}, err
	}
	return domain.ComputeEffective(s.clock.Now(), controls), nil
}
