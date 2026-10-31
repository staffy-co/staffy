package main

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/staffy-co/staffy/internal/adapters/connectorrefresolver"
	controlAPI "github.com/staffy-co/staffy/internal/control/api"
	controlApp "github.com/staffy-co/staffy/internal/control/app"
	controlDomain "github.com/staffy-co/staffy/internal/control/domain"
	controlPort "github.com/staffy-co/staffy/internal/control/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// The Story 3-2 fixture: the scoped control surface over the REAL app
// service with the REAL composition-root connector adapter and the LIVE
// fixture membership semantics; the persistence is the fixture-world
// memory repository that mirrors the store contract honestly (the
// optimistic concurrency, the monotonic generations, the append-only
// history, the tenant isolation). The points NEVER live in the product
// database: the production control state is the tenant PG rows the
// fixture server honestly does not carry — the default contour is the
// honestly empty world, the fixture-only /fixture/controls toggle of
// the z30 journey seeds the fixture world through the REAL Set path
// (a guard-refused seed panics the server before the first request —
// the admission-at-construction discipline of the actions fixture).

// controlPair is the material identity of one control point of the
// fixture world (the tenant-scoped key of the memory repository).
type controlPair struct {
	tenant, kind, ref string
}

// controlsMemoryRepository is the in-memory Repository of the fixture
// world: the honest mirror of the store contract — Set and Resume keep
// the optimistic generation discipline (the conflict answers the typed
// ConflictError with the CURRENT material), the lift keeps the row (a
// removed point is history, not absence), the reads stay
// tenant-isolated and deterministic, the returned records are fresh
// copies (the caller's mutation never reaches the stored state).
type controlsMemoryRepository struct {
	mu     sync.Mutex
	points map[controlPair]controlDomain.ScopedControl
}

func newControlsMemoryRepository() *controlsMemoryRepository {
	return &controlsMemoryRepository{points: map[controlPair]controlDomain.ScopedControl{}}
}

// reset clears the world (the fixture toggle: the honest empty contour).
func (r *controlsMemoryRepository) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.points = map[controlPair]controlDomain.ScopedControl{}
}

// copyControl answers a fresh copy with fresh window pointers.
func copyControl(c controlDomain.ScopedControl) controlDomain.ScopedControl {
	copied := c
	if c.ScheduledStartAt != nil {
		value := *c.ScheduledStartAt
		copied.ScheduledStartAt = &value
	}
	if c.ExpiresAt != nil {
		value := *c.ExpiresAt
		copied.ExpiresAt = &value
	}
	return copied
}

func (r *controlsMemoryRepository) Set(_ context.Context, p controlPort.SetParams) (controlDomain.ScopedControl, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := controlPair{p.Scope.TenantID().String(), p.Kind, p.Ref}
	now := p.Now.UTC().Truncate(time.Microsecond)
	current, exists := r.points[key]
	if !exists {
		if p.ExpectedGeneration != 0 {
			// Nothing to confirm over: the honest 404 (no audit of a
			// point that never existed — the FK honesty of the store).
			return controlDomain.ScopedControl{}, controlDomain.ErrNotFound
		}
		created := controlDomain.ScopedControl{
			ScopeKind: p.Kind, ScopeRef: p.Ref, Mode: p.Mode, Reason: p.Reason,
			ActorID: p.Scope.PrincipalID().String(), PrevMode: "", Generation: 1,
			ScheduledStartAt: p.ScheduledStartAt, ExpiresAt: p.ExpiresAt,
			State: controlDomain.ControlStateActive, UpdatedAt: now,
		}
		if created.ScheduledStartAt != nil {
			value := *p.ScheduledStartAt
			created.ScheduledStartAt = &value
		}
		if created.ExpiresAt != nil {
			value := *p.ExpiresAt
			created.ExpiresAt = &value
		}
		r.points[key] = copyControl(created)
		return copyControl(created), nil
	}
	if current.Generation != p.ExpectedGeneration {
		return controlDomain.ScopedControl{}, controlDomain.ConflictError{
			ScopeKind: p.Kind, ScopeRef: p.Ref, Expected: p.ExpectedGeneration, Current: copyControl(current),
		}
	}
	updated := controlDomain.ScopedControl{
		ScopeKind: p.Kind, ScopeRef: p.Ref, Mode: p.Mode, Reason: p.Reason,
		ActorID: p.Scope.PrincipalID().String(), PrevMode: current.Mode, Generation: current.Generation + 1,
		ScheduledStartAt: p.ScheduledStartAt, ExpiresAt: p.ExpiresAt,
		State: controlDomain.ControlStateActive, UpdatedAt: now,
	}
	if updated.ScheduledStartAt != nil {
		value := *p.ScheduledStartAt
		updated.ScheduledStartAt = &value
	}
	if updated.ExpiresAt != nil {
		value := *p.ExpiresAt
		updated.ExpiresAt = &value
	}
	r.points[key] = copyControl(updated)
	return copyControl(updated), nil
}

func (r *controlsMemoryRepository) Resume(_ context.Context, p controlPort.ResumeParams) (controlDomain.ScopedControl, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := controlPair{p.Scope.TenantID().String(), p.Kind, p.Ref}
	current, exists := r.points[key]
	if !exists {
		return controlDomain.ScopedControl{}, controlDomain.ErrNotFound
	}
	if current.Generation != p.ExpectedGeneration {
		return controlDomain.ScopedControl{}, controlDomain.ConflictError{
			ScopeKind: p.Kind, ScopeRef: p.Ref, Expected: p.ExpectedGeneration, Current: copyControl(current),
		}
	}
	if current.State == controlDomain.ControlStateRemoved {
		// The honest no-op refusal of the store: repeating a removal is
		// not a state change and does not pretend to be one.
		return controlDomain.ScopedControl{}, controlDomain.RuleError{Rule: "state", Reason: "точка контроля уже снята: повторное снятие не меняет состояние"}
	}
	updated := current
	updated.Reason = p.Reason
	updated.ActorID = p.Scope.PrincipalID().String()
	updated.PrevMode = current.Mode
	updated.Generation = current.Generation + 1
	updated.State = controlDomain.ControlStateRemoved
	updated.UpdatedAt = p.Now.UTC().Truncate(time.Microsecond)
	r.points[key] = copyControl(updated)
	return copyControl(updated), nil
}

func (r *controlsMemoryRepository) List(_ context.Context, scope tenancy.Context) ([]controlDomain.ScopedControl, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := []controlDomain.ScopedControl{}
	for key, point := range r.points {
		if key.tenant != scope.TenantID().String() {
			continue
		}
		list = append(list, copyControl(point))
	}
	sort.Slice(list, func(i, j int) bool {
		if !list[i].UpdatedAt.Equal(list[j].UpdatedAt) {
			return list[i].UpdatedAt.After(list[j].UpdatedAt)
		}
		if list[i].ScopeKind != list[j].ScopeKind {
			return list[i].ScopeKind < list[j].ScopeKind
		}
		return list[i].ScopeRef < list[j].ScopeRef
	})
	return list, nil
}

func (r *controlsMemoryRepository) LoadKeys(_ context.Context, scope tenancy.Context, keys []controlDomain.ScopeKey) ([]controlDomain.ScopedControl, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	wanted := make(map[controlPair]bool, len(keys))
	for _, key := range keys {
		wanted[controlPair{scope.TenantID().String(), key.Kind, key.Ref}] = true
	}
	list := []controlDomain.ScopedControl{}
	for key, point := range r.points {
		if !wanted[key] {
			continue
		}
		list = append(list, copyControl(point))
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].ScopeKind != list[j].ScopeKind {
			return list[i].ScopeKind < list[j].ScopeKind
		}
		return list[i].ScopeRef < list[j].ScopeRef
	})
	return list, nil
}

// controlsFixture is the fixture switch of the control world: the REAL
// app service over the memory repository, the REAL live connector
// adapter (the published catalog of 1-8 — no PG dependency), the LIVE
// fixture membership semantics (actionsMembership: the owner holds
// every scope, the second operator holds review+audit only — the
// honest 403 member of the no-scope row) and the shared journey clock
// (the /fixture/clock shift walks the read-time window legs). The
// /fixture/controls toggle seeds the fixture world through the REAL
// Set path (guard admission at the world construction — a dishonest
// seed panics the server) and resets it to the honestly empty contour.
type controlsFixture struct {
	mu      sync.Mutex
	on      bool
	repo    *controlsMemoryRepository
	service *controlApp.Service
}

// newControlsFixture constructs the fixture: the REAL app service over
// the fixture memory world. The default contour is the honest empty
// world (the production control state lives in the tenant PG rows the
// fixture server does not carry).
func newControlsFixture() *controlsFixture {
	repo := newControlsMemoryRepository()
	return &controlsFixture{
		repo:    repo,
		service: controlApp.New(repo, actionsMembership{}, connectorrefresolver.New(), journeyClock, idsFixture{}),
	}
}

// setLive switches the fixture world (the /fixture/controls toggle):
// enabled seeds the two fixture control points through the REAL Set
// path (the guard admission at the world construction — a refused seed
// panics the server, the admission-at-construction discipline);
// disabled resets the world to the honestly empty contour.
func (f *controlsFixture) setLive(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.on = enabled
	f.repo.reset()
	if !enabled {
		return
	}
	owner := identityDomain.Principal{ID: principalID, CanCreateTenant: true, StepUpVerified: true}
	seeds := []controlAPI.SetRequest{
		{
			Kind: "channel", Ref: "phone", Mode: controlDomain.ControlModeIntakeOnly,
			Reason: "Инцидент качества телефонии: приём обращений без новых обещаний",
		},
		{
			Kind: "connector", Ref: "staffy-synthetic-telephony", Mode: controlDomain.ControlModeHumanOnly,
			Reason: "Регламентный обход поставщика телефонии: до конца работ — только человек",
		},
	}
	for _, seed := range seeds {
		if _, err := f.service.Set(context.Background(), owner, tenantID, seed); err != nil {
			panic("the fixture control world must be guard-admitted at construction: " + err.Error())
		}
	}
}

// The narrow-interface proof: the fixture serves the REAL app surface
// the transport type-asserts.
var _ controlAPI.ControlService = (*controlsFixture)(nil)

func (f *controlsFixture) Set(ctx context.Context, actor identityDomain.Principal, tenant id.UUID, request controlAPI.SetRequest) (controlDomain.ScopedControl, error) {
	return f.service.Set(ctx, actor, tenant, request)
}

func (f *controlsFixture) Resume(ctx context.Context, actor identityDomain.Principal, tenant id.UUID, request controlAPI.ResumeRequest) (controlDomain.ScopedControl, error) {
	return f.service.Resume(ctx, actor, tenant, request)
}

func (f *controlsFixture) List(ctx context.Context, actor identityDomain.Principal, tenant id.UUID) ([]controlDomain.ScopedControl, error) {
	return f.service.List(ctx, actor, tenant)
}

func (f *controlsFixture) EffectiveFor(ctx context.Context, actor identityDomain.Principal, tenant id.UUID, keys []controlDomain.ScopeKey) (controlDomain.EffectiveControl, error) {
	return f.service.EffectiveFor(ctx, actor, tenant, keys)
}
