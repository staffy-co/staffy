package app

import (
        "context"
        "errors"
        "testing"
        "time"

        identityAPI "github.com/artemiosu/staffy/internal/identity/api"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
        entitlementAPI "github.com/artemiosu/staffy/internal/entitlement/api"
        entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
        "github.com/artemiosu/staffy/internal/entitlement/port"
)

var testNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

var (
        testPrincipal = identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000000001"), CanCreateTenant: true}
        testTenant    = id.MustParse("01994f64-6c00-7000-8000-000000000002")
        testCorr      = id.MustParse("01994f64-6c00-7000-8000-000000000003")
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type sequenceIDs struct{ counter int }

func (g *sequenceIDs) New(time.Time) (id.UUID, error) {
        g.counter++
        return id.MustParse("01994f64-6c00-7000-8000-" + idSuffix(g.counter)), nil
}

func idSuffix(counter int) string {
        digits := "000000000000"
        number := []byte(digits)
        value := counter
        for i := len(number) - 1; i >= 0 && value > 0; i-- {
                number[i] = byte('0' + value%10)
                value /= 10
        }
        return string(number)
}

// stubIdentity authorizes every action and reports the workspace; the level
// distinctions are covered by the integration tests against the real store.
type stubIdentity struct{ identityAPI.MembershipService }

func (stubIdentity) Authorize(context.Context, identityDomain.Principal, id.UUID, identityDomain.Scope) error {
        return nil
}
func (stubIdentity) Workspace(context.Context, identityDomain.Principal, id.UUID) (identityDomain.Workspace, error) {
        return identityDomain.Workspace{Tenant: identityDomain.Tenant{ID: testTenant}, Role: "owner"}, nil
}

// memoryRepo keeps the append-only facts in memory.
type memoryRepo struct {
        terms       []entitlementDomain.TermsVersion
        stateEvents []entitlementDomain.StateEvent
        budgets     []entitlementDomain.BudgetVersion
        sources     []entitlementDomain.UsageSource
        events      []entitlementDomain.UsageEvent
        exceptions  []entitlementDomain.UsageException
        acks        []entitlementDomain.ExceptionAck
}

func (r *memoryRepo) SaveTerms(_ context.Context, p port.TermsParams) (entitlementDomain.TermsVersion, error) {
        p.Terms.Version = int64(len(r.terms) + 1)
        p.Terms.TenantID = p.Scope.TenantID().String()
        p.Terms.ActorID = p.Scope.PrincipalID().String()
        p.Terms.CreatedAt = p.Now
        r.terms = append(r.terms, p.Terms)
        return p.Terms, nil
}

func (r *memoryRepo) ListTerms(context.Context, tenancy.Context) ([]entitlementDomain.TermsVersion, error) {
        return r.terms, nil
}

func (r *memoryRepo) SaveStateEvent(_ context.Context, p port.StateEventParams) error {
        p.Event.TenantID = p.Scope.TenantID().String()
        p.Event.ActorID = p.Scope.PrincipalID().String()
        p.Event.CreatedAt = p.Now
        r.stateEvents = append(r.stateEvents, p.Event)
        return nil
}

func (r *memoryRepo) ListStateEvents(context.Context, tenancy.Context) ([]entitlementDomain.StateEvent, error) {
        return r.stateEvents, nil
}

func (r *memoryRepo) IngestUsage(_ context.Context, p port.IngestParams) (entitlementDomain.UsageEvent, bool, error) {
        for _, existing := range r.events {
                if existing.EventKey == p.Event.EventKey {
                        if existing.PayloadHash != p.Event.PayloadHash {
                                return entitlementDomain.UsageEvent{}, false, entitlementDomain.ErrEventKeyConflict
                        }
                        return existing, true, nil
                }
        }
        p.Event.TenantID = p.Scope.TenantID().String()
        p.Event.ActorID = p.Scope.PrincipalID().String()
        r.events = append(r.events, p.Event)
        return p.Event, false, nil
}

func (r *memoryRepo) ListUsageEvents(context.Context, tenancy.Context) ([]entitlementDomain.UsageEvent, error) {
        return r.events, nil
}

func (r *memoryRepo) SaveSource(_ context.Context, p port.SourceParams) (entitlementDomain.UsageSource, error) {
        p.Source.TenantID = p.Scope.TenantID().String()
        p.Source.ActorID = p.Scope.PrincipalID().String()
        p.Source.CreatedAt = p.Now
        r.sources = append(r.sources, p.Source)
        return p.Source, nil
}

func (r *memoryRepo) ListSources(context.Context, tenancy.Context) ([]entitlementDomain.UsageSource, error) {
        return r.sources, nil
}

func (r *memoryRepo) SaveBudget(_ context.Context, p port.BudgetParams) (entitlementDomain.BudgetVersion, error) {
        p.Budget.Version = int64(len(r.budgets) + 1)
        p.Budget.TenantID = p.Scope.TenantID().String()
        p.Budget.ActorID = p.Scope.PrincipalID().String()
        p.Budget.CreatedAt = p.Now
        r.budgets = append(r.budgets, p.Budget)
        return p.Budget, nil
}

func (r *memoryRepo) ListBudgets(context.Context, tenancy.Context) ([]entitlementDomain.BudgetVersion, error) {
        return r.budgets, nil
}

func (r *memoryRepo) SaveException(_ context.Context, p port.ExceptionParams) (entitlementDomain.UsageException, bool, error) {
        for _, existing := range r.exceptions {
                if existing.DedupeKey == p.Exception.DedupeKey {
                        return existing, false, nil
                }
        }
        p.Exception.TenantID = p.Scope.TenantID().String()
        p.Exception.ActorID = p.Scope.PrincipalID().String()
        r.exceptions = append(r.exceptions, p.Exception)
        return p.Exception, true, nil
}

func (r *memoryRepo) ListExceptions(context.Context, tenancy.Context) ([]entitlementDomain.UsageException, error) {
        return r.exceptions, nil
}

func (r *memoryRepo) SaveAck(_ context.Context, p port.AckParams) error {
        p.Ack.TenantID = p.Scope.TenantID().String()
        p.Ack.ActorID = p.Scope.PrincipalID().String()
        p.Ack.CreatedAt = p.Now
        r.acks = append(r.acks, p.Ack)
        return nil
}

func (r *memoryRepo) ExceptionExists(_ context.Context, _ tenancy.Context, dedupeKey string) (bool, error) {
        for _, existing := range r.exceptions {
                if existing.DedupeKey == dedupeKey {
                        return true, nil
                }
        }
        return false, nil
}

func newTestService() (*Service, *memoryRepo) {
        repo := &memoryRepo{}
        return New(repo, stubIdentity{}, fixedClock{testNow}, &sequenceIDs{}), repo
}

func TestIngestUsageMoneyCategoryRequiresAmount(t *testing.T) {
        service, _ := newTestService()
        _, _, err := service.IngestUsage(context.Background(), entitlementAPI.UsageEventCommand{
                Actor: testPrincipal, TenantID: testTenant,
                EventKey: "connector-charge-1", Source: "provider-report",
                Category: entitlementDomain.CategoryConnectorCharges, Quantity: 10,
                PeriodStart: testNow, PeriodEnd: testNow.Add(time.Hour),
                CorrelationID: testCorr,
        })
        var rule entitlementDomain.RuleError
        if !errors.As(err, &rule) || rule.Rule != "amount" {
                t.Fatalf("money category without amount = %v, want amount rule error", err)
        }
}

func TestIngestUsageHonestConflict(t *testing.T) {
        service, _ := newTestService()
        command := entitlementAPI.UsageEventCommand{
                Actor: testPrincipal, TenantID: testTenant,
                EventKey: "carrier-0001", Source: "report", Category: entitlementDomain.CategoryCarrier,
                Quantity: 100, PeriodStart: testNow, PeriodEnd: testNow.Add(time.Hour),
                CorrelationID: testCorr,
        }
        if _, replayed, err := service.IngestUsage(context.Background(), command); err != nil || replayed {
                t.Fatalf("first ingest = %v (replayed %v)", err, replayed)
        }
        command.Quantity = 101
        if _, _, err := service.IngestUsage(context.Background(), command); !errors.Is(err, entitlementDomain.ErrEventKeyConflict) {
                t.Fatalf("material change under the same key = %v, want ErrEventKeyConflict", err)
        }
}

func TestIngestUsageReplayDoesNotRaiseDuplicateExceptions(t *testing.T) {
        service, repo := newTestService()
        command := entitlementAPI.UsageEventCommand{
                Actor: testPrincipal, TenantID: testTenant,
                EventKey: "carrier-0002", Source: "report", Category: entitlementDomain.CategoryCarrier,
                Quantity: 100, PeriodStart: testNow, PeriodEnd: testNow.Add(time.Hour),
                CorrelationID: testCorr,
        }
        if _, _, err := service.IngestUsage(context.Background(), command); err != nil {
                t.Fatal(err)
        }
        count := len(repo.exceptions)
        if _, replayed, err := service.IngestUsage(context.Background(), command); err != nil || !replayed {
                t.Fatalf("replay = %v (replayed %v)", err, replayed)
        }
        if len(repo.exceptions) != count {
                t.Fatalf("replay must not raise new exceptions: %d → %d", count, len(repo.exceptions))
        }
}

func TestActivationRequiresConfirmationAndReadyTerms(t *testing.T) {
        service, _ := newTestService()
        start := testNow
        end := testNow.Add(30 * 24 * time.Hour)
        launch, subscription := 49000.0, 15000.0
        if _, err := service.ComposeTerms(context.Background(), entitlementAPI.TermsCommand{
                Actor: testPrincipal, TenantID: testTenant, Currency: "RUB",
                PeriodStart: &start, PeriodEnd: &end,
                LaunchPrice: &launch, SubscriptionPrice: &subscription,
                CorrelationID: testCorr,
        }); err != nil {
                t.Fatal(err)
        }
        if _, err := service.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: testPrincipal, TenantID: testTenant, Action: entitlementDomain.EventActivate,
                Basis: "Договор №1", Confirm: false, CorrelationID: testCorr,
        }); err == nil {
                t.Fatal("activation without confirmation must be rejected")
        }
        if _, err := service.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: testPrincipal, TenantID: testTenant, Action: entitlementDomain.EventActivate,
                Basis: "Договор №1", Confirm: true, CorrelationID: testCorr,
        }); err != nil {
                t.Fatalf("confirmed activation of ready terms = %v", err)
        }
}

func TestActivationWithoutCurrencyIsFailClosed(t *testing.T) {
        service, _ := newTestService()
        start := testNow
        if _, err := service.ComposeTerms(context.Background(), entitlementAPI.TermsCommand{
                Actor: testPrincipal, TenantID: testTenant,
                PeriodStart: &start, CorrelationID: testCorr,
        }); err != nil {
                t.Fatal(err)
        }
        if _, err := service.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: testPrincipal, TenantID: testTenant, Action: entitlementDomain.EventActivate,
                Basis: "Договор №1", Confirm: true, CorrelationID: testCorr,
        }); err == nil {
                t.Fatal("terms without currency must not activate (fail-closed)")
        }
}

func TestBudgetValidationDictionary(t *testing.T) {
        service, _ := newTestService()
        if _, err := service.SetBudget(context.Background(), entitlementAPI.BudgetCommand{
                Actor: testPrincipal, TenantID: testTenant, Amount: 1000, Currency: "RUB",
                PeriodStart: testNow, PeriodEnd: testNow.Add(24 * time.Hour),
                WarningPercent: 80, LimitPolicy: "unknown_policy",
                CorrelationID: testCorr,
        }); err == nil {
                t.Fatal("unknown limit policy must be rejected with an honest cause")
        }
        limit := 500.0
        if _, err := service.SetBudget(context.Background(), entitlementAPI.BudgetCommand{
                Actor: testPrincipal, TenantID: testTenant, Amount: 1000, Currency: "RUB",
                PeriodStart: testNow, PeriodEnd: testNow.Add(24 * time.Hour),
                WarningPercent: 80, LimitPolicy: entitlementDomain.PolicyBlockNew, HardLimit: &limit,
                CorrelationID: testCorr,
        }); err == nil {
                t.Fatal("hard limit below the budget amount must be rejected")
        }
}

func TestThresholdExceptionWithIncompleteDataHidesTheExactSum(t *testing.T) {
        service, _ := newTestService()
        limit := 1200.0
        if _, err := service.SetBudget(context.Background(), entitlementAPI.BudgetCommand{
                Actor: testPrincipal, TenantID: testTenant, Amount: 1000, Currency: "RUB",
                PeriodStart: testNow, PeriodEnd: testNow.Add(24 * time.Hour),
                WarningPercent: 80, LimitPolicy: entitlementDomain.PolicyBlockNew, HardLimit: &limit,
                CorrelationID: testCorr,
        }); err != nil {
                t.Fatal(err)
        }
        amount := 850.0
        // No registered source: the data stays unknown, so the threshold
        // message must not claim the exact sum.
        if _, _, err := service.IngestUsage(context.Background(), entitlementAPI.UsageEventCommand{
                Actor: testPrincipal, TenantID: testTenant,
                EventKey: "carrier-no-source", Source: "report", Category: entitlementDomain.CategoryCarrier,
                Quantity: 100, Amount: &amount, Currency: "RUB",
                PeriodStart: testNow, PeriodEnd: testNow.Add(time.Hour),
                CorrelationID: testCorr,
        }); err != nil {
                t.Fatal(err)
        }
        view, err := service.ExceptionsView(context.Background(), testPrincipal, testTenant)
        if err != nil {
                t.Fatal(err)
        }
        found := false
        for _, exception := range view.Exceptions {
                if exception.Cause == entitlementDomain.CauseBudgetThreshold {
                        found = true
                        if exception.DataStatus == entitlementDomain.DataComplete {
                                t.Fatalf("no registered source keeps the status honest: %+v", exception)
                        }
                        if containsSubstring(exception.Message, "850.00") {
                                t.Fatalf("incomplete data must not claim the exact sum: %q", exception.Message)
                        }
                }
        }
        if !found {
                t.Fatal("the threshold exception must be raised")
        }
}

func containsSubstring(haystack, needle string) bool {
        for i := 0; i+len(needle) <= len(haystack); i++ {
                if haystack[i:i+len(needle)] == needle {
                        return true
                }
        }
        return false
}
