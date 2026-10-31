package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/control/api"
	"github.com/staffy-co/staffy/internal/control/domain"
	"github.com/staffy-co/staffy/internal/control/port"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// The step-02 unit matrix of the ControlService: the gate ladder BY NAME
// (each refusal before any store call — the fake repository counts every
// call), the dictionary guard before the transaction, the live connector
// resolution (fail-closed, nil-resolver fail-closed, loud infra), the
// optimistic conflict propagation with the CURRENT material, the read
// surfaces without the step-up gate, and the EffectiveFor computation
// over the repository rows.

var frozen = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

func mustID(t *testing.T, at time.Time) id.UUID {
	t.Helper()
	generated, err := id.NewV7(at)
	if err != nil {
		t.Fatal(err)
	}
	return generated
}

// fakeIdentity answers the Authorize gate from a scenario map; every
// other MembershipService method is irrelevant to the control module and
// stays an honest no-op (the control module never calls them).
type fakeIdentity struct {
	authorizeByScope map[identityDomain.Scope]error
	calls            int
}

func (f *fakeIdentity) Authorize(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, scope identityDomain.Scope) error {
	f.calls++
	if err, ok := f.authorizeByScope[scope]; ok {
		return err
	}
	return nil
}

func (f *fakeIdentity) ListMembers(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]identityDomain.Membership, error) {
	return nil, nil
}

func (f *fakeIdentity) InviteMember(ctx context.Context, command identityAPI.MembershipCommand) (identityDomain.Membership, string, error) {
	return identityDomain.Membership{}, "", nil
}

func (f *fakeIdentity) AcceptInvitation(ctx context.Context, actor identityDomain.Principal, token string, tenantID id.UUID) (identityDomain.Membership, error) {
	return identityDomain.Membership{}, nil
}

func (f *fakeIdentity) ChangeMember(ctx context.Context, command identityAPI.MembershipCommand) (identityDomain.Membership, error) {
	return identityDomain.Membership{}, nil
}

func (f *fakeIdentity) RevokeMember(ctx context.Context, command identityAPI.MembershipCommand) error {
	return nil
}

// fakeRepo records EVERY call the service makes — the refusal tests pin
// setCalls/resumeCalls/loadKeysCalls at zero (the honest "before the
// transaction" proof).
type fakeRepo struct {
	setCalls     []port.SetParams
	resumeCalls  []port.ResumeParams
	listCalls    int
	loadKeys     [][]domain.ScopeKey
	setResult    domain.ScopedControl
	setErr       error
	resumeResult domain.ScopedControl
	resumeErr    error
	listResult   []domain.ScopedControl
	listErr      error
	loadResult   []domain.ScopedControl
	loadErr      error
}

func (f *fakeRepo) Set(ctx context.Context, p port.SetParams) (domain.ScopedControl, error) {
	f.setCalls = append(f.setCalls, p)
	return f.setResult, f.setErr
}

func (f *fakeRepo) Resume(ctx context.Context, p port.ResumeParams) (domain.ScopedControl, error) {
	f.resumeCalls = append(f.resumeCalls, p)
	return f.resumeResult, f.resumeErr
}

func (f *fakeRepo) List(ctx context.Context, scope tenancy.Context) ([]domain.ScopedControl, error) {
	f.listCalls++
	return f.listResult, f.listErr
}

func (f *fakeRepo) LoadKeys(ctx context.Context, scope tenancy.Context, keys []domain.ScopeKey) ([]domain.ScopedControl, error) {
	f.loadKeys = append(f.loadKeys, keys)
	return f.loadResult, f.loadErr
}

// fakeResolver answers the LIVE connector registry from a map; err
// simulates the broken registry (the loud refusal).
type fakeResolver struct {
	known map[string]bool
	err   error
	calls int
}

func (f *fakeResolver) Resolve(ctx context.Context, tenantID id.UUID, connectorID string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return f.known[connectorID], nil
}

type frozenClock struct{ at time.Time }

func (c frozenClock) Now() time.Time { return c.at }

// realIDs mints platform v7 identifiers (the real generator — the ids of
// the service must be valid UUIDs the store can write).
type realIDs struct{}

func (realIDs) New(at time.Time) (id.UUID, error) { return id.NewV7(at) }

type fixture struct {
	actor    identityDomain.Principal
	tenantID id.UUID
	repo     *fakeRepo
	service  *Service
	resolver *fakeResolver
}

// newFixture wires the happy-path service: a member with the operations
// scope, a fresh step-up, a resolving registry and a working repository.
func newFixture(t *testing.T, authorizeByScope map[identityDomain.Scope]error, resolver *fakeResolver) *fixture {
	t.Helper()
	actor := identityDomain.Principal{ID: mustID(t, frozen), StepUpVerified: true}
	tenantID := mustID(t, frozen.Add(time.Second))
	repo := &fakeRepo{}
	if resolver == nil {
		resolver = &fakeResolver{known: map[string]bool{"montagio": true}}
	}
	service := New(repo, &fakeIdentity{authorizeByScope: authorizeByScope}, resolver, frozenClock{frozen}, realIDs{})
	return &fixture{actor: actor, tenantID: tenantID, repo: repo, service: service, resolver: resolver}
}

// assertNoStoreCall is the shared "refusal before the transaction" proof:
// the repository was never reached.
func assertNoStoreCall(t *testing.T, repo *fakeRepo) {
	t.Helper()
	if len(repo.setCalls) != 0 {
		t.Fatalf("repository Set was called %d times, want 0 (refusal before the transaction)", len(repo.setCalls))
	}
	if len(repo.resumeCalls) != 0 {
		t.Fatalf("repository Resume was called %d times, want 0 (refusal before the transaction)", len(repo.resumeCalls))
	}
	if repo.listCalls != 0 {
		t.Fatalf("repository List was called %d times, want 0", repo.listCalls)
	}
	if len(repo.loadKeys) != 0 {
		t.Fatalf("repository LoadKeys was called %d times, want 0", len(repo.loadKeys))
	}
}

func validSetRequest() api.SetRequest {
	return api.SetRequest{
		Kind: "connector", Ref: "montagio", Mode: "human_only",
		Reason: "рискованный маршрут остановлен до разбора", ExpectedGeneration: 0,
	}
}

// --- The Set gate ladder, by name, in the honest order -----------------

func TestSetInvalidActorOrTenantGetsNotFoundBeforeIdentity(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.actor.ID = id.UUID("")
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, validSetRequest())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("invalid actor: got %v, want domain.ErrNotFound", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetNonMemberOfTenantGetsModuleNotFoundBeforeMaterial(t *testing.T) {
	fx := newFixture(t, map[identityDomain.Scope]error{identityDomain.Operations: identityDomain.ErrNotFound}, nil)
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, validSetRequest())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("non-member: got %v, want domain.ErrNotFound", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetMemberWithoutOperationsScopeGetsForbiddenWordsBeforeMaterial(t *testing.T) {
	fx := newFixture(t, map[identityDomain.Scope]error{identityDomain.Operations: identityDomain.ErrForbidden}, nil)
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, validSetRequest())
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member without operations: got %v, want domain.ErrForbidden", err)
	}
	if !strings.Contains(err.Error(), "operations") {
		t.Fatalf("the 403 must speak the operations scope words, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), controlsScopeText) {
		t.Fatalf("the 403 text must be the single source controlsScopeText, got %q", err.Error())
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetWithoutStepUpGetsStepUpRequired(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.actor.StepUpVerified = false
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, validSetRequest())
	if !errors.Is(err, identityDomain.ErrStepUpRequired) {
		t.Fatalf("no step-up: got %v, want identityDomain.ErrStepUpRequired", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetStepUpComesBeforeTheDictionaryGuard(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.actor.StepUpVerified = false
	request := validSetRequest()
	request.Mode = "bogus"
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	if !errors.Is(err, identityDomain.ErrStepUpRequired) {
		t.Fatalf("the ladder order: a step-up-less operator with a bogus mode must see the step-up refusal first, got %v", err)
	}
	assertNoStoreCall(t, fx.repo)
}

// --- The dictionary guard: refusals BEFORE the transaction -------------

func TestSetUnknownModeRefusedBeforeTransaction(t *testing.T) {
	fx := newFixture(t, nil, nil)
	request := validSetRequest()
	request.Mode = "bogus"
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "mode" {
		t.Fatalf("unknown mode: got %v, want RuleError{mode}", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetUnknownScopeKindRefusedBeforeTransaction(t *testing.T) {
	fx := newFixture(t, nil, nil)
	request := validSetRequest()
	request.Kind = "region"
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_kind" {
		t.Fatalf("unknown kind: got %v, want RuleError{scope_kind}", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetEmptyReasonRefusedBeforeTransaction(t *testing.T) {
	fx := newFixture(t, nil, nil)
	request := validSetRequest()
	request.Reason = "   "
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "reason" {
		t.Fatalf("empty reason: got %v, want RuleError{reason}", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetInvertedWindowRefusedBeforeTransaction(t *testing.T) {
	fx := newFixture(t, nil, nil)
	request := validSetRequest()
	start := frozen.Add(time.Hour)
	expires := frozen.Add(-time.Hour)
	request.ScheduledStartAt, request.ExpiresAt = &start, &expires
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "window" {
		t.Fatalf("inverted window: got %v, want RuleError{window}", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetTenantKindWithForeignTenantRefRefusedBeforeTransaction(t *testing.T) {
	fx := newFixture(t, nil, nil)
	request := validSetRequest()
	request.Kind = "tenant"
	request.Ref = mustID(t, frozen.Add(2*time.Second)).String()
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_ref" {
		t.Fatalf("foreign tenant ref: got %v, want RuleError{scope_ref}", err)
	}
	if !strings.Contains(err.Error(), "собственный") {
		t.Fatalf("the refusal must speak the own-tenant material equality, got %q", err.Error())
	}
	assertNoStoreCall(t, fx.repo)
}

// --- The live connector resolution (the connector kind only) -----------

func TestSetConnectorRefUnknownToLiveRegistryRefusedFailClosed(t *testing.T) {
	fx := newFixture(t, nil, &fakeResolver{known: map[string]bool{"montagio": true}})
	request := validSetRequest()
	request.Ref = "not-in-registry"
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_ref" {
		t.Fatalf("unknown connector: got %v, want RuleError{scope_ref}", err)
	}
	if !strings.Contains(err.Error(), "not-in-registry") {
		t.Fatalf("the refusal must speak the honest cause with the ref, got %q", err.Error())
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetConnectorKindWithNilResolverFailsClosed(t *testing.T) {
	fx := newFixture(t, nil, nil)
	service := New(fx.repo, &fakeIdentity{}, nil, frozenClock{frozen}, realIDs{})
	request := validSetRequest()
	_, err := service.Set(context.Background(), fx.actor, fx.tenantID, request)
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_ref" {
		t.Fatalf("nil resolver: got %v, want RuleError{scope_ref} (fail-closed)", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetResolverInfrastructureFailurePropagatesLoud(t *testing.T) {
	infraErr := errors.New("registry is unreachable")
	fx := newFixture(t, nil, &fakeResolver{err: infraErr})
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, validSetRequest())
	if !errors.Is(err, infraErr) {
		t.Fatalf("infra failure: got %v, want the loud propagation (never a silent not-found)", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestSetNonConnectorKindSkipsTheLiveResolver(t *testing.T) {
	fx := newFixture(t, nil, &fakeResolver{known: map[string]bool{}})
	request := validSetRequest()
	request.Kind, request.Ref = "channel", "phone"
	fx.repo.setResult = domain.ScopedControl{ScopeKind: "channel", ScopeRef: "phone", Mode: "paused", State: "active", Generation: 1}
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	if err != nil {
		t.Fatal(err)
	}
	if fx.resolver.calls != 0 {
		t.Fatalf("the live resolver must be consulted for the connector kind only, called %d times", fx.resolver.calls)
	}
}

// --- The successful Set: the material that reaches the repository ------

func TestSetSuccessfulPassesGuardedMaterialToRepository(t *testing.T) {
	fx := newFixture(t, nil, nil)
	request := validSetRequest()
	request.ExpectedGeneration = 3
	start := frozen.Add(time.Minute)
	request.ScheduledStartAt = &start
	fx.repo.setResult = domain.ScopedControl{ScopeKind: "connector", ScopeRef: "montagio", Mode: "human_only", Generation: 4, State: "active"}
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(fx.repo.setCalls) != 1 {
		t.Fatalf("repository Set called %d times, want 1", len(fx.repo.setCalls))
	}
	params := fx.repo.setCalls[0]
	if params.Kind != "connector" || params.Ref != "montagio" || params.Mode != "human_only" || params.Reason != request.Reason {
		t.Fatalf("the guarded material must reach the repository verbatim, got %+v", params)
	}
	if params.ExpectedGeneration != 3 {
		t.Fatalf("expected generation = %d, want 3", params.ExpectedGeneration)
	}
	if params.ScheduledStartAt == nil || !params.ScheduledStartAt.Equal(start) {
		t.Fatalf("the window start must travel to the repository, got %v", params.ScheduledStartAt)
	}
	if !params.Now.Equal(frozen) {
		t.Fatalf("the repository Now must be the clock's frozen time, got %v", params.Now)
	}
	if params.Scope.TenantID() != fx.tenantID || params.Scope.PrincipalID() != fx.actor.ID {
		t.Fatalf("the RLS scope must carry the actor and the tenant, got %+v", params.Scope)
	}
	if !params.AuditID.Valid() || !params.EventID.Valid() || !params.CorrelationID.Valid() {
		t.Fatalf("the audit, event and correlation ids must be minted valid, got %q %q %q", params.AuditID, params.EventID, params.CorrelationID)
	}
	if params.AuditID == params.EventID {
		t.Fatalf("the audit and event ids must be distinct (two independent rows)")
	}
}

func TestSetRequestCorrelationWinsOverFreshMint(t *testing.T) {
	fx := newFixture(t, nil, nil)
	request := validSetRequest()
	request.CorrelationID = mustID(t, frozen.Add(3*time.Second))
	fx.repo.setResult = domain.ScopedControl{Generation: 1}
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	if err != nil {
		t.Fatal(err)
	}
	if got := fx.repo.setCalls[0].CorrelationID; got != request.CorrelationID {
		t.Fatalf("the request's correlation id must win, got %q want %q", got, request.CorrelationID)
	}
}

func TestSetGeneratesCorrelationWhenRequestBroughtNone(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.repo.setResult = domain.ScopedControl{Generation: 1}
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, validSetRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got := fx.repo.setCalls[0].CorrelationID; !got.Valid() {
		t.Fatalf("a missing request correlation must be minted fresh, got %q", got)
	}
}

func TestSetGenerationConflictPropagatesWithCurrentMaterial(t *testing.T) {
	fx := newFixture(t, nil, nil)
	current := domain.ScopedControl{ScopeKind: "connector", ScopeRef: "montagio", Mode: "paused", Generation: 7, State: "active", Reason: "уже остановлено другим оператором"}
	fx.repo.setErr = domain.ConflictError{ScopeKind: "connector", ScopeRef: "montagio", Expected: 0, Current: current}
	request := validSetRequest()
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, request)
	var conflict domain.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("the conflict must propagate typed, got %v", err)
	}
	if conflict.Current.Generation != 7 || conflict.Current.Mode != "paused" {
		t.Fatalf("the CURRENT material must travel back to the second operator, got %+v", conflict.Current)
	}
	if !errors.Is(err, domain.ErrGenerationConflict) {
		t.Fatalf("the conflict must unwrap to the sentinel, got %v", err)
	}
}

func TestSetRepositoryNotFoundPropagates(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.repo.setErr = domain.ErrNotFound
	_, err := fx.service.Set(context.Background(), fx.actor, fx.tenantID, validSetRequest())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("repository ErrNotFound must propagate, got %v", err)
	}
}

// --- The Resume ladder and material ------------------------------------

func validResumeRequest() api.ResumeRequest {
	return api.ResumeRequest{
		Kind: "connector", Ref: "montagio",
		Reason:             "инцидент разобран, работа возобновлена",
		ExpectedGeneration: 2,
	}
}

func TestResumeGateLadderMirrorsSet(t *testing.T) {
	// The non-member: 404 before material.
	fx := newFixture(t, map[identityDomain.Scope]error{identityDomain.Operations: identityDomain.ErrNotFound}, nil)
	if _, err := fx.service.Resume(context.Background(), fx.actor, fx.tenantID, validResumeRequest()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("non-member resume: got %v, want ErrNotFound", err)
	}
	assertNoStoreCall(t, fx.repo)

	// The member without the operations scope: the words 403.
	fx = newFixture(t, map[identityDomain.Scope]error{identityDomain.Operations: identityDomain.ErrForbidden}, nil)
	if _, err := fx.service.Resume(context.Background(), fx.actor, fx.tenantID, validResumeRequest()); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("scope-less resume: got %v, want ErrForbidden", err)
	}
	assertNoStoreCall(t, fx.repo)

	// The step-up-less operator: the step-up refusal.
	fx = newFixture(t, nil, nil)
	fx.actor.StepUpVerified = false
	if _, err := fx.service.Resume(context.Background(), fx.actor, fx.tenantID, validResumeRequest()); !errors.Is(err, identityDomain.ErrStepUpRequired) {
		t.Fatalf("step-up-less resume: got %v, want ErrStepUpRequired", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestResumeDictionaryRefusalsBeforeTransaction(t *testing.T) {
	fx := newFixture(t, nil, nil)
	request := validResumeRequest()
	request.Kind = "region"
	if _, err := fx.service.Resume(context.Background(), fx.actor, fx.tenantID, request); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("unknown kind: got %v, want the ErrInvalid family", err)
	}
	request = validResumeRequest()
	request.Reason = ""
	if _, err := fx.service.Resume(context.Background(), fx.actor, fx.tenantID, request); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("empty reason: got %v, want the ErrInvalid family", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestResumeForeignConnectorRefRefusedFailClosed(t *testing.T) {
	fx := newFixture(t, nil, &fakeResolver{known: map[string]bool{"montagio": true}})
	request := validResumeRequest()
	request.Ref = "not-in-registry"
	_, err := fx.service.Resume(context.Background(), fx.actor, fx.tenantID, request)
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_ref" {
		t.Fatalf("unknown connector resume: got %v, want RuleError{scope_ref}", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestResumeSuccessfulPassesMaterialToRepository(t *testing.T) {
	fx := newFixture(t, nil, nil)
	request := validResumeRequest()
	fx.repo.resumeResult = domain.ScopedControl{ScopeKind: "connector", ScopeRef: "montagio", Mode: "paused", State: "removed", Generation: 3}
	_, err := fx.service.Resume(context.Background(), fx.actor, fx.tenantID, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(fx.repo.resumeCalls) != 1 {
		t.Fatalf("repository Resume called %d times, want 1", len(fx.repo.resumeCalls))
	}
	params := fx.repo.resumeCalls[0]
	if params.Kind != "connector" || params.Ref != "montagio" || params.Reason != request.Reason || params.ExpectedGeneration != 2 {
		t.Fatalf("the resume material must reach the repository verbatim, got %+v", params)
	}
	if !params.AuditID.Valid() || !params.EventID.Valid() || !params.CorrelationID.Valid() {
		t.Fatalf("the ids must be minted valid for the resume too")
	}
}

func TestResumeConflictPropagatesWithMaterial(t *testing.T) {
	fx := newFixture(t, nil, nil)
	current := domain.ScopedControl{ScopeKind: "connector", ScopeRef: "montagio", Mode: "accept", Generation: 9, State: "active"}
	fx.repo.resumeErr = domain.ConflictError{Expected: 2, Current: current}
	_, err := fx.service.Resume(context.Background(), fx.actor, fx.tenantID, validResumeRequest())
	var conflict domain.ConflictError
	if !errors.As(err, &conflict) || conflict.Current.Generation != 9 {
		t.Fatalf("the resume conflict must carry the current material, got %v", err)
	}
}

// --- The read surfaces: the gate ladder WITHOUT the step-up gate -------

func TestListWorksWithoutStepUp(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.actor.StepUpVerified = false
	fx.repo.listResult = []domain.ScopedControl{{ScopeKind: "channel", ScopeRef: "phone", Mode: "paused", State: "active", Generation: 1}}
	list, err := fx.service.List(context.Background(), fx.actor, fx.tenantID)
	if err != nil {
		t.Fatalf("reading never requires the step-up: %v", err)
	}
	if len(list) != 1 || list[0].Mode != "paused" {
		t.Fatalf("the list must pass through verbatim, got %+v", list)
	}
	if fx.repo.listCalls != 1 {
		t.Fatalf("repository List called %d times, want 1", fx.repo.listCalls)
	}
}

func TestListNonMemberGetsNotFoundBeforeMaterial(t *testing.T) {
	fx := newFixture(t, map[identityDomain.Scope]error{identityDomain.Operations: identityDomain.ErrNotFound}, nil)
	if _, err := fx.service.List(context.Background(), fx.actor, fx.tenantID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("non-member list: got %v, want ErrNotFound", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestListMemberWithoutOperationsScopeGetsForbiddenWordsBeforeMaterial(t *testing.T) {
	fx := newFixture(t, map[identityDomain.Scope]error{identityDomain.Operations: identityDomain.ErrForbidden}, nil)
	_, err := fx.service.List(context.Background(), fx.actor, fx.tenantID)
	if !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), controlsScopeText) {
		t.Fatalf("scope-less list: got %v, want the words 403", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestEffectiveForReadGateMirrorsListWithoutStepUp(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.actor.StepUpVerified = false
	keys := []domain.ScopeKey{{Kind: "connector", Ref: "montagio"}}
	effective, err := fx.service.EffectiveFor(context.Background(), fx.actor, fx.tenantID, keys)
	if err != nil {
		t.Fatalf("the effective read never requires the step-up: %v", err)
	}
	if effective.Mode != domain.ControlModeAccept {
		t.Fatalf("absent controls mean accept, got %q", effective.Mode)
	}

	// The gates still guard the read: the non-member and the scope-less
	// member never reach the computation.
	gated := newFixture(t, map[identityDomain.Scope]error{identityDomain.Operations: identityDomain.ErrNotFound}, nil)
	if _, err := gated.service.EffectiveFor(context.Background(), gated.actor, gated.tenantID, keys); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("non-member effective: got %v, want ErrNotFound", err)
	}
	gated = newFixture(t, map[identityDomain.Scope]error{identityDomain.Operations: identityDomain.ErrForbidden}, nil)
	if _, err := gated.service.EffectiveFor(context.Background(), gated.actor, gated.tenantID, keys); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("scope-less effective: got %v, want ErrForbidden", err)
	}
	assertNoStoreCall(t, gated.repo)
}

func TestEffectiveForUnknownScopeKindOfKeyRefusedBeforeQuery(t *testing.T) {
	fx := newFixture(t, nil, nil)
	_, err := fx.service.EffectiveFor(context.Background(), fx.actor, fx.tenantID, []domain.ScopeKey{{Kind: "region", Ref: "eu"}})
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_kind" {
		t.Fatalf("unknown key kind: got %v, want RuleError{scope_kind}", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestEffectiveForTenantKeyOfForeignTenantRefused(t *testing.T) {
	fx := newFixture(t, nil, nil)
	foreign := mustID(t, frozen.Add(4*time.Second))
	_, err := fx.service.EffectiveFor(context.Background(), fx.actor, fx.tenantID, []domain.ScopeKey{{Kind: "tenant", Ref: foreign.String()}})
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_ref" {
		t.Fatalf("foreign tenant key: got %v, want RuleError{scope_ref}", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestEffectiveForTooManyKeysRefusedBeforeQuery(t *testing.T) {
	fx := newFixture(t, nil, nil)
	keys := make([]domain.ScopeKey, domain.MaxScopeKeys+1)
	for i := range keys {
		keys[i] = domain.ScopeKey{Kind: "channel", Ref: "phone"}
	}
	_, err := fx.service.EffectiveFor(context.Background(), fx.actor, fx.tenantID, keys)
	var rule domain.RuleError
	if !errors.As(err, &rule) || rule.Rule != "scope_keys" {
		t.Fatalf("too many keys: got %v, want RuleError{scope_keys}", err)
	}
	assertNoStoreCall(t, fx.repo)
}

func TestEffectiveForDeduplicatesEqualKeys(t *testing.T) {
	fx := newFixture(t, nil, nil)
	keys := []domain.ScopeKey{
		{Kind: "connector", Ref: "montagio"},
		{Kind: "connector", Ref: "montagio"},
		{Kind: "channel", Ref: "phone"},
	}
	_, err := fx.service.EffectiveFor(context.Background(), fx.actor, fx.tenantID, keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(fx.repo.loadKeys) != 1 || len(fx.repo.loadKeys[0]) != 2 {
		t.Fatalf("the duplicate keys must collapse before the query, got %+v", fx.repo.loadKeys)
	}
}

func TestEffectiveForComputesTheMostRestrictiveIntersectionOverRepositoryRows(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.repo.loadResult = []domain.ScopedControl{
		{ScopeKind: "tenant", ScopeRef: fx.tenantID.String(), Mode: domain.ControlModeIntakeOnly, Reason: "инцидент 1", State: "active", Generation: 2},
		{ScopeKind: "connector", ScopeRef: "montagio", Mode: domain.ControlModeHumanOnly, Reason: "инцидент 2", State: "active", Generation: 1},
		{ScopeKind: "channel", ScopeRef: "phone", Mode: domain.ControlModeAccept, Reason: "старое разрешение", State: "removed", Generation: 1},
	}
	keys := []domain.ScopeKey{
		{Kind: "tenant", Ref: fx.tenantID.String()},
		{Kind: "connector", Ref: "montagio"},
		{Kind: "channel", Ref: "phone"},
	}
	effective, err := fx.service.EffectiveFor(context.Background(), fx.actor, fx.tenantID, keys)
	if err != nil {
		t.Fatal(err)
	}
	// human_only (rank 2) beats intake_only (rank 1); the removed control
	// does not act; accept never cancels a blocking.
	if effective.Mode != domain.ControlModeHumanOnly {
		t.Fatalf("the intersection must be the rank maximum, got %q", effective.Mode)
	}
	if effective.Reason != "инцидент 2" {
		t.Fatalf("the winning reason must be the most restrictive control's, got %q", effective.Reason)
	}
	if len(effective.Influencing) != 2 {
		t.Fatalf("the provenance must keep every INFLUENCING control (the removed one is not), got %d", len(effective.Influencing))
	}
	wantIntersection := []string{domain.ControlModeIntakeOnly, domain.ControlModeHumanOnly}
	if len(effective.Intersection) != len(wantIntersection) || effective.Intersection[0] != wantIntersection[0] || effective.Intersection[1] != wantIntersection[1] {
		t.Fatalf("the intersection words must follow the ladder order, got %v", effective.Intersection)
	}
}

func TestEffectiveForCorruptedModeReadsPausedFailClosed(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.repo.loadResult = []domain.ScopedControl{
		{ScopeKind: "connector", ScopeRef: "montagio", Mode: "garbage", Reason: "повреждено", State: "active", Generation: 5},
		{ScopeKind: "channel", ScopeRef: "phone", Mode: domain.ControlModeAccept, Reason: "ок", State: "active", Generation: 1},
	}
	effective, err := fx.service.EffectiveFor(context.Background(), fx.actor, fx.tenantID, []domain.ScopeKey{{Kind: "connector", Ref: "montagio"}, {Kind: "channel", Ref: "phone"}})
	if err != nil {
		t.Fatal(err)
	}
	if effective.Mode != domain.ControlModePaused {
		t.Fatalf("a corrupted value must read as paused (fail-closed), got %q", effective.Mode)
	}
	if !effective.Corrupted {
		t.Fatalf("the corruption marker must stay honest")
	}
}

func TestEffectiveForScheduledFutureControlDoesNotActYet(t *testing.T) {
	fx := newFixture(t, nil, nil)
	start := frozen.Add(time.Hour)
	fx.repo.loadResult = []domain.ScopedControl{
		{ScopeKind: "connector", ScopeRef: "montagio", Mode: domain.ControlModePaused, Reason: "запланировано", State: "active", Generation: 1, ScheduledStartAt: &start},
	}
	effective, err := fx.service.EffectiveFor(context.Background(), fx.actor, fx.tenantID, []domain.ScopeKey{{Kind: "connector", Ref: "montagio"}})
	if err != nil {
		t.Fatal(err)
	}
	if effective.Mode != domain.ControlModeAccept || len(effective.Influencing) != 0 {
		t.Fatalf("a scheduled-future control must not act yet: mode %q, influencing %d", effective.Mode, len(effective.Influencing))
	}
}

func TestEffectiveForRepositoryFailurePropagates(t *testing.T) {
	fx := newFixture(t, nil, nil)
	fx.repo.loadErr = errors.New("pool is closed")
	_, err := fx.service.EffectiveFor(context.Background(), fx.actor, fx.tenantID, []domain.ScopeKey{{Kind: "connector", Ref: "montagio"}})
	if !errors.Is(err, fx.repo.loadErr) {
		t.Fatalf("the repository failure must propagate loud, got %v", err)
	}
}
