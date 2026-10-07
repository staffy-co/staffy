//go:build integration

package integration_test

import (
        "context"
        "errors"
        "fmt"
        "testing"
        "time"

        "github.com/artemiosu/staffy/internal/identity/api"
        identityApp "github.com/artemiosu/staffy/internal/identity/app"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        identityStore "github.com/artemiosu/staffy/internal/identity/store"
        "github.com/artemiosu/staffy/internal/platform/id"
        entitlementAPI "github.com/artemiosu/staffy/internal/entitlement/api"
        entitlementApp "github.com/artemiosu/staffy/internal/entitlement/app"
        entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
        entitlementStore "github.com/artemiosu/staffy/internal/entitlement/store"
)

// entitlementWorld assembles the entitlement service over the real store,
// plus a tenant and fixtures.
type entitlementWorld struct {
        identity    *identityApp.Service
        entitlement *entitlementApp.Service
        principal   identityDomain.Principal
        tenant      id.UUID
}

var entitlementClock = fixedClock{time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)}

func newEntitlementWorld(t *testing.T, seed int) *entitlementWorld {
        t.Helper()
        ids := newPrivacyIDs(seed + 500)
        identity := identityApp.New(identityStore.NewPostgres(runtimePool), entitlementClock, ids)
        principal := identityDomain.Principal{ID: id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", seed*100000+500)), CanCreateTenant: true}
        result, err := identity.CreateTenant(context.Background(), api.CreateTenantCommand{
                Principal: principal, Name: "Магазин Олега", IdempotencyKey: fmt.Sprintf("entitlement-tenant-%03d", seed),
                CorrelationID: id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", seed*100000+501)),
        })
        if err != nil {
                t.Fatal(err)
        }
        return &entitlementWorld{
                identity: identity, principal: principal, tenant: result.Tenant.ID,
                entitlement: entitlementApp.New(entitlementStore.NewPostgres(runtimePool), identity, entitlementClock, ids),
        }
}

func termsCommand(world *entitlementWorld, currency string) entitlementAPI.TermsCommand {
        start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
        end := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
        launch := 49000.0
        subscription := 15000.0
        return entitlementAPI.TermsCommand{
                Actor: world.principal, TenantID: world.tenant,
                Currency: currency, TaxRegime: "УСН",
                PeriodStart: &start, PeriodEnd: &end,
                IncludedEmployees:  []string{"consultant-01"},
                IncludedChannels:   []string{"jivo"},
                IncludedConnectors: []string{"abcp"},
                Limits:             []entitlementDomain.CategoryLimit{{Category: entitlementDomain.CategoryCarrier, Cap: 1000}},
                LaunchPrice:        &launch, SubscriptionPrice: &subscription,
                ChangeNote:         "Коммерческое предложение №1",
                CorrelationID:      id.MustParse("01994f64-6c00-7000-8000-000000000601"),
        }
}

func budgetCommand(world *entitlementWorld, amount, hardLimit float64) entitlementAPI.BudgetCommand {
        limit := hardLimit
        command := entitlementAPI.BudgetCommand{
                Actor: world.principal, TenantID: world.tenant,
                Amount: amount, Currency: "RUB",
                PeriodStart:    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
                PeriodEnd:      time.Date(2026, 10, 31, 23, 59, 59, 0, time.UTC),
                WarningPercent: 80,
                LimitPolicy:    entitlementDomain.PolicyBlockNew,
                Note:           "Пилотный бюджет октября",
                CorrelationID:  id.MustParse("01994f64-6c00-7000-8000-000000000602"),
        }
        if limit > 0 {
                command.HardLimit = &limit
                command.AnomalyThresholds = []entitlementDomain.AnomalyThreshold{
                        {Category: entitlementDomain.CategoryManualMinutes, MaxQuantity: 600},
                }
        }
        return command
}

func usageCommand(world *entitlementWorld, key, category string, quantity, amount float64, estimate bool) entitlementAPI.UsageEventCommand {
        value := amount
        var amountPtr *float64
        if amount >= 0 {
                amountPtr = &value
        }
        return entitlementAPI.UsageEventCommand{
                Actor: world.principal, TenantID: world.tenant,
                EventKey: key, Source: "carrier-report",
                Category: entitlementDomain.UsageCategory(category), Quantity: quantity,
                Amount: amountPtr, Currency: "RUB", Estimate: estimate,
                PeriodStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
                PeriodEnd:   time.Date(2026, 10, 31, 23, 59, 59, 0, time.UTC),
                EmployeeID:  "consultant-01", Channel: "jivo", ReleaseVersion: "rel-1",
                Provenance:   "Отчёт оператора за октябрь",
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000603"),
        }
}

func registerSource(t *testing.T, world *entitlementWorld, name string, categories ...entitlementDomain.UsageCategory) {
        t.Helper()
        if _, err := world.entitlement.RegisterSource(context.Background(), entitlementAPI.SourceCommand{
                Actor: world.principal, TenantID: world.tenant,
                Name: name, Categories: categories, FreshnessMinutes: 1440,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000604"),
        }); err != nil {
                t.Fatal(err)
        }
}

// TestEntitlementDraftViewIsHonest walks the draft state: the composition
// skeleton with open questions instead of zero prices, and the fail-closed
// activation guards.
func TestEntitlementDraftViewIsHonest(t *testing.T) {
        world := newEntitlementWorld(t, 81)
        view, err := world.entitlement.EntitlementView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.State != entitlementDomain.StateDraft {
                t.Fatalf("state = %q, want draft", view.State)
        }
        if view.Level != entitlementAPI.LevelFull {
                t.Fatalf("level = %q, want full (Owner)", view.Level)
        }
        if len(view.OpenQuestions) == 0 {
                t.Fatal("draft view must carry open questions, not zeros")
        }
        joined := fmt.Sprint(view.OpenQuestions)
        for _, want := range []string{"цена запуска", "не ноль", "цена подписки"} {
                if !contains(joined, want) {
                        t.Fatalf("open questions must mention %q: %s", want, joined)
                }
        }
        if view.Admission.Status != entitlementDomain.AdmissionBlocked {
                t.Fatalf("draft admission = %q, want blocked_new_admissions", view.Admission.Status)
        }
        if len(view.Admission.Continues) != len(entitlementDomain.ContinuesList) {
                t.Fatal("admission must list what continues working")
        }

        // Activation without terms is honestly rejected.
        if _, err := world.entitlement.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: world.principal, TenantID: world.tenant, Action: entitlementDomain.EventActivate,
                Basis: "Договор №1", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000605"),
        }); err == nil {
                t.Fatal("activation without a terms version must be rejected")
        }

        // Terms without currency cannot activate (fail-closed).
        if _, err := world.entitlement.ComposeTerms(context.Background(), termsCommand(world, "")); err != nil {
                t.Fatal(err)
        }
        if _, err := world.entitlement.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: world.principal, TenantID: world.tenant, Action: entitlementDomain.EventActivate,
                Basis: "Договор №1", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000605"),
        }); err == nil {
                t.Fatal("activation without currency must be rejected with an honest cause")
        }
}

func contains(haystack, needle string) bool {
        return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
        for i := 0; i+len(needle) <= len(haystack); i++ {
                if haystack[i:i+len(needle)] == needle {
                        return i
                }
        }
        return -1
}

// TestEntitlementLifecycle walks the full honest lifecycle: versioned terms,
// confirmed activation, a second version without retroactive changes, pause,
// resume with a recorded limits check, and the terminal end.
func TestEntitlementLifecycle(t *testing.T) {
        world := newEntitlementWorld(t, 82)
        first, err := world.entitlement.ComposeTerms(context.Background(), termsCommand(world, "RUB"))
        if err != nil {
                t.Fatal(err)
        }
        // Activation without the explicit confirmation is rejected.
        if _, err := world.entitlement.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: world.principal, TenantID: world.tenant, Action: entitlementDomain.EventActivate,
                Basis: "Договор №77", Confirm: false,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000606"),
        }); err == nil {
                t.Fatal("activation without confirmation must be rejected")
        }
        activated, err := world.entitlement.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: world.principal, TenantID: world.tenant, Action: entitlementDomain.EventActivate,
                Basis: "Договор №77 от 01.10.2026, подтвердил Олег", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000607"),
        })
        if err != nil {
                t.Fatal(err)
        }
        if activated.TermsVersion != first.Version || activated.ActorID != world.principal.ID.String() {
                t.Fatalf("activation event = %+v", activated)
        }

        // A material change creates a new version; the old one stays untouched.
        second, err := world.entitlement.ComposeTerms(context.Background(), termsCommand(world, "RUB"))
        if err != nil {
                t.Fatal(err)
        }
        if second.Version != first.Version+1 {
                t.Fatalf("second version = %d, want %d", second.Version, first.Version+1)
        }
        view, err := world.entitlement.EntitlementView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.State != entitlementDomain.StateActive || view.Terms.Version != second.Version {
                t.Fatalf("active view: state=%s terms v%d", view.State, view.Terms.Version)
        }

        // Pause blocks admissions honestly; resume records the limits check.
        if _, err := world.entitlement.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: world.principal, TenantID: world.tenant, Action: entitlementDomain.EventPause,
                Basis: "Каникулы пилота", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000608"),
        }); err != nil {
                t.Fatal(err)
        }
        admission, err := world.entitlement.AdmissionView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if admission.Status != entitlementDomain.AdmissionBlocked || admission.Policy != "entitlement_state" {
                t.Fatalf("paused admission = %+v", admission)
        }
        resumed, err := world.entitlement.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: world.principal, TenantID: world.tenant, Action: entitlementDomain.EventResume,
                Basis: "Пилот продолжается", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000609"),
        })
        if err != nil {
                t.Fatal(err)
        }
        if resumed.LimitsCheck != "no_budget" {
                t.Fatalf("resume limits check = %q, want no_budget (no budget published)", resumed.LimitsCheck)
        }

        // End is terminal: a later resume is honestly rejected.
        if _, err := world.entitlement.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: world.principal, TenantID: world.tenant, Action: entitlementDomain.EventEnd,
                Basis: "Пилот завершён", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000610"),
        }); err != nil {
                t.Fatal(err)
        }
        if _, err := world.entitlement.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: world.principal, TenantID: world.tenant, Action: entitlementDomain.EventResume,
                Basis: "Возврат", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000611"),
        }); err == nil {
                t.Fatal("resume after end must be rejected: end is terminal")
        }
        view, err = world.entitlement.EntitlementView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.State != entitlementDomain.StateEnded {
                t.Fatalf("state = %q, want ended", view.State)
        }
}

// TestUsageIngestIdempotency covers the honest idempotency: one delivery
// counted once, a replay returning the stored result, and the same key with
// a different payload as a conflict for review.
func TestUsageIngestIdempotency(t *testing.T) {
        world := newEntitlementWorld(t, 83)
        registerSource(t, world, "carrier-report", entitlementDomain.CategoryCarrier)

        first, replayed, err := world.entitlement.IngestUsage(context.Background(), usageCommand(world, "carrier-2026-10-a", "carrier", 120, 600, false))
        if err != nil {
                t.Fatal(err)
        }
        if replayed {
                t.Fatal("first delivery must not be a replay")
        }
        again, replayed, err := world.entitlement.IngestUsage(context.Background(), usageCommand(world, "carrier-2026-10-a", "carrier", 120, 600, false))
        if err != nil {
                t.Fatal(err)
        }
        if !replayed || again.EventID != first.EventID {
                t.Fatalf("replay = %v (event %s vs %s)", replayed, again.EventID, first.EventID)
        }

        // The same key with a different material payload is a conflict.
        _, _, err = world.entitlement.IngestUsage(context.Background(), usageCommand(world, "carrier-2026-10-a", "carrier", 121, 600, false))
        if !errors.Is(err, entitlementDomain.ErrEventKeyConflict) {
                t.Fatalf("conflict = %v, want ErrEventKeyConflict", err)
        }

        // The meter counts the event exactly once.
        view, err := world.entitlement.MeterView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        for _, row := range view.Categories {
                if row.Category == entitlementDomain.CategoryCarrier {
                        if row.EventCount != 1 || row.Quantity != 120 {
                                t.Fatalf("carrier counted %d events, quantity %.2f", row.EventCount, row.Quantity)
                        }
                }
        }
        // The ingestion needs the operations or commercial scope: it is not
        // an anonymous write.
}

// TestMeterHonesty walks the honest data statuses: no source — unknown,
// a partial period — partial, an exceeded freshness — stale, and the strict
// separation of estimates from confirmed accruals.
func TestMeterHonesty(t *testing.T) {
        world := newEntitlementWorld(t, 84)
        if _, err := world.entitlement.SetBudget(context.Background(), budgetCommand(world, 100000, 0)); err != nil {
                t.Fatal(err)
        }
        registerSource(t, world, "carrier-report", entitlementDomain.CategoryCarrier, entitlementDomain.CategoryManualMinutes)

        // Unknown: no events at all for a registered source.
        view, err := world.entitlement.MeterView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.DataStatus != entitlementDomain.DataUnknown {
                t.Fatalf("empty meter data status = %q, want unknown", view.DataStatus)
        }
        if len(view.MissingCategories) == 0 {
                t.Fatal("categories without a registered source must be listed as missing")
        }
        // Unknown spend never becomes a zero: the money maps stay empty.
        for _, row := range view.Categories {
                if row.Category == entitlementDomain.CategoryCarrier {
                        if len(row.MoneyConfirmed) != 0 || row.Status != entitlementDomain.DataUnknown {
                                t.Fatalf("unknown category must not carry sums: %+v", row)
                        }
                }
        }

        // Partial: events cover only part of the period. The overall status
        // stays honest (categories without sources stay unknown); the carrier
        // row itself reports the partial coverage.
        if _, _, err := world.entitlement.IngestUsage(context.Background(), partialPeriodCommand(world, "carrier-partial-1")); err != nil {
                t.Fatal(err)
        }
        view, err = world.entitlement.MeterView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.DataStatus != entitlementDomain.DataUnknown {
                t.Fatalf("overall status = %q: categories without a registered source keep the meter honestly unknown", view.DataStatus)
        }
        carrierRow, ok := categoryRow(view, entitlementDomain.CategoryCarrier)
        if !ok || carrierRow.Status != entitlementDomain.DataPartial {
                t.Fatalf("carrier row = %+v, want partial coverage", carrierRow)
        }

        // Estimates are separated from the confirmed accrual.
        if _, _, err := world.entitlement.IngestUsage(context.Background(), usageCommand(world, "carrier-estimate-1", "carrier", 60, 300, true)); err != nil {
                t.Fatal(err)
        }
        view, err = world.entitlement.MeterView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.EstimateTotal["RUB"] != 300 || view.ConfirmedTotal["RUB"] != 150 {
                t.Fatalf("estimate = %v, confirmed = %v: the partial-period event is confirmed (150), the estimate stays separate (300)", view.EstimateTotal, view.ConfirmedTotal)
        }
        if view.Budget != nil && view.Budget.Warned {
                t.Fatal("estimates must not trip the warning threshold")
        }
}

func categoryRow(view entitlementAPI.MeterView, category entitlementDomain.UsageCategory) (entitlementDomain.CategoryMeter, bool) {
        for _, row := range view.Categories {
                if row.Category == category {
                        return row, true
                }
        }
        return entitlementDomain.CategoryMeter{}, false
}

func partialPeriodCommand(world *entitlementWorld, key string) entitlementAPI.UsageEventCommand {
        command := usageCommand(world, key, "carrier", 30, 150, false)
        command.PeriodStart = time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
        command.PeriodEnd = time.Date(2026, 10, 20, 23, 59, 59, 0, time.UTC)
        return command
}

// TestBudgetThresholdsAndHardLimit covers the warning threshold exception,
// the hard limit exception with the declared policy, the estimate exclusion
// and the deduplicated raising.
func TestBudgetThresholdsAndHardLimit(t *testing.T) {
        world := newEntitlementWorld(t, 85)
        registerSource(t, world, "all-report", entitlementDomain.AllCategories()...)
        if _, err := world.entitlement.SetBudget(context.Background(), budgetCommand(world, 1000, 1200)); err != nil {
                t.Fatal(err)
        }
        // The entitlement is activated so the admission state answers by the
        // budget policy, not by the draft state.
        if _, err := world.entitlement.ComposeTerms(context.Background(), termsCommand(world, "RUB")); err != nil {
                t.Fatal(err)
        }
        if _, err := world.entitlement.RunStateTransition(context.Background(), entitlementAPI.StateCommand{
                Actor: world.principal, TenantID: world.tenant, Action: entitlementDomain.EventActivate,
                Basis: "Договор №85", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000617"),
        }); err != nil {
                t.Fatal(err)
        }
        // Full-period coverage of every registered category keeps the data
        // complete, so the threshold message may carry the exact sum.
        for i, category := range entitlementDomain.AllCategories() {
                amount := 25.0
                if category == entitlementDomain.CategoryCarrier {
                        continue
                }
                command := usageCommand(world, fmt.Sprintf("filler-%d", i), string(category), 1, amount, false)
                if _, _, err := world.entitlement.IngestUsage(context.Background(), command); err != nil {
                        t.Fatal(err)
                }
        }
        for i := 0; i < 2; i++ {
                if _, _, err := world.entitlement.IngestUsage(context.Background(), usageCommand(world, fmt.Sprintf("carrier-fill-%d", i), "carrier", 60, 400, false)); err != nil {
                        t.Fatal(err)
                }
        }
        view, err := world.entitlement.MeterView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.ConfirmedTotal["RUB"] != 950 {
                t.Fatalf("confirmed total = %v, want 950 RUB (800 carrier + 150 fillers)", view.ConfirmedTotal)
        }
        // The warning threshold (80% of 1000) is crossed: one deduplicated exception.
        exceptions, err := world.entitlement.ExceptionsView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        thresholdCount := countCause(exceptions, entitlementDomain.CauseBudgetThreshold)
        if thresholdCount != 1 {
                t.Fatalf("budget_threshold exceptions = %d, want exactly 1 (deduplicated)", thresholdCount)
        }
        for _, exception := range exceptions.Exceptions {
                if exception.Cause == entitlementDomain.CauseBudgetThreshold && exception.DataStatus != entitlementDomain.DataComplete {
                        t.Fatalf("threshold exception data status = %q", exception.DataStatus)
                }
                if exception.Cause == entitlementDomain.CauseBudgetThreshold && !contains(exception.Message, "950.00") {
                        t.Fatalf("complete data must carry the exact sum: %q", exception.Message)
                }
        }

        // Reaching the hard limit applies the declared policy and raises the
        // hard_limit_reached exception; the admission state blocks new.
        if _, _, err := world.entitlement.IngestUsage(context.Background(), usageCommand(world, "carrier-over-1", "carrier", 30, 450, false)); err != nil {
                t.Fatal(err)
        }
        admission, err := world.entitlement.AdmissionView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if admission.Status != entitlementDomain.AdmissionBlocked || admission.Policy != entitlementDomain.PolicyBlockNew {
                t.Fatalf("admission = %+v", admission)
        }
        if len(admission.Continues) != len(entitlementDomain.ContinuesList) || admission.Owner != entitlementDomain.AdmissionOwner {
                t.Fatalf("admission must name what continues and the owner: %+v", admission)
        }
        exceptions, err = world.entitlement.ExceptionsView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if countCause(exceptions, entitlementDomain.CauseHardLimit) != 1 {
                t.Fatalf("hard_limit exceptions = %d, want 1", countCause(exceptions, entitlementDomain.CauseHardLimit))
        }
}

func countCause(view entitlementAPI.ExceptionsView, cause string) int {
        count := 0
        for _, exception := range view.Exceptions {
                if exception.Cause == cause {
                        count++
                }
        }
        return count
}

// TestExceptionAcknowledgeAndManualCauses covers the manual exception with
// the retry storm cause (the dictionary is complete even though the runtime
// trigger is an Epic 2 boundary) and the derived acknowledgement.
func TestExceptionAcknowledgeAndManualCauses(t *testing.T) {
        world := newEntitlementWorld(t, 86)
        exception, err := world.entitlement.RaiseException(context.Background(), entitlementAPI.ManualExceptionCommand{
                Actor: world.principal, TenantID: world.tenant,
                Cause:             entitlementDomain.CauseAnomalyRetryStorm,
                Message:           "Наблюдается шторм повторов обращений на маршруте jivo.",
                AffectedEmployee:  "consultant-01", AffectedRoute: "jivo",
                RecommendedAction: "Проверить маршрут доставки и лимиты повтора.",
                CorrelationID:     id.MustParse("01994f64-6c00-7000-8000-000000000612"),
        })
        if err != nil {
                t.Fatal(err)
        }
        view, err := world.entitlement.ExceptionsView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if len(view.Exceptions) != 1 || view.Exceptions[0].Acknowledged != nil {
                t.Fatalf("fresh exception must be unacknowledged: %+v", view.Exceptions)
        }
        if err := world.entitlement.AcknowledgeException(context.Background(), entitlementAPI.AckCommand{
                Actor: world.principal, TenantID: world.tenant,
                ExceptionID: id.MustParse(exception.ExceptionID), Note: "Принято к сведению",
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000613"),
        }); err != nil {
                t.Fatal(err)
        }
        view, err = world.entitlement.ExceptionsView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.Exceptions[0].Acknowledged == nil || view.Exceptions[0].Acknowledged.Note != "Принято к сведению" {
                t.Fatalf("acknowledgement must be derived: %+v", view.Exceptions[0].Acknowledged)
        }
        // An unknown exception id is an honest 404.
        if err := world.entitlement.AcknowledgeException(context.Background(), entitlementAPI.AckCommand{
                Actor: world.principal, TenantID: world.tenant,
                ExceptionID: id.MustParse("01994f64-6c00-7000-8000-000000000614"),
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000615"),
        }); !errors.Is(err, entitlementDomain.ErrNotFound) {
                t.Fatalf("unknown exception ack = %v, want ErrNotFound", err)
        }
}

// TestEntitlementIsolationAndLevels covers the tenant isolation (RLS) and
// the detail levels: a member without commercial/audit sees only the
// summary; the management commands are blocked server-side.
func TestEntitlementIsolationAndLevels(t *testing.T) {
        world := newEntitlementWorld(t, 87)
        other := newEntitlementWorld(t, 88)
        if _, err := world.entitlement.ComposeTerms(context.Background(), termsCommand(world, "RUB")); err != nil {
                t.Fatal(err)
        }
        // Tenant isolation: another principal cannot see the first tenant.
        if _, err := other.entitlement.EntitlementView(context.Background(), other.principal, world.tenant); !errors.Is(err, entitlementDomain.ErrNotFound) {
                t.Fatalf("cross-tenant view = %v, want ErrNotFound", err)
        }
        crossCommand := usageCommand(world, "carrier-cross-1", "carrier", 10, 50, false)
        crossCommand.Actor = other.principal
        if _, _, err := other.entitlement.IngestUsage(context.Background(), crossCommand); !errors.Is(err, entitlementDomain.ErrNotFound) {
                t.Fatalf("cross-tenant ingest = %v, want ErrNotFound", err)
        }

        // A member with the operations scope only: the summary level, no sums,
        // but the usage ingestion works.
        member := seedMember(t, world, 1, []identityDomain.Role{identityDomain.Operator}, []identityDomain.Scope{identityDomain.Operations})
        view, err := world.entitlement.MeterView(context.Background(), member, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if view.Level != entitlementAPI.LevelSummary {
                t.Fatalf("operator meter level = %q, want summary", view.Level)
        }
        if view.Categories != nil || view.ConfirmedTotal != nil || view.Budget != nil {
                t.Fatalf("summary level must carry no sums or terms: %+v", view)
        }
        // R1: the summary EntitlementView carries no tariff either — no terms,
        // no history, no lifecycle events with commercial grounds, no sources,
        // no open questions of the tariff (the tenant HAS a terms version).
        summary, err := world.entitlement.EntitlementView(context.Background(), member, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if summary.Level != entitlementAPI.LevelSummary {
                t.Fatalf("operator entitlement level = %q, want summary", summary.Level)
        }
        if summary.Terms != nil || summary.History != nil || summary.StateEvents != nil || summary.Sources != nil || summary.OpenQuestions != nil {
                t.Fatalf("summary entitlement view must carry no tariff, history, state events or sources: %+v", summary)
        }
        if summary.State != entitlementDomain.StateDraft || summary.BudgetQuality == "" {
                t.Fatalf("summary view must still carry the state and the budget quality: %+v", summary)
        }
        if _, _, err := world.entitlement.IngestUsage(context.Background(), usageMemberCommand(world, member, "carrier-op-1")); err != nil {
                t.Fatalf("operations member must ingest usage: %v", err)
        }
        // Management is blocked server-side for the operations member.
        if _, err := world.entitlement.ComposeTerms(context.Background(), memberTermsCommand(world, member)); err == nil {
                t.Fatal("terms composition without commercial scope must be rejected")
        }
        if _, err := world.entitlement.SetBudget(context.Background(), memberBudgetCommand(world, member)); err == nil {
                t.Fatal("budget publication without commercial scope must be rejected")
        }

        // An audit-scope member sees the full detail but cannot manage.
        auditor := seedMember(t, world, 2, []identityDomain.Role{identityDomain.ExpertQA}, []identityDomain.Scope{identityDomain.Audit})
        auditView, err := world.entitlement.MeterView(context.Background(), auditor, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if auditView.Level != entitlementAPI.LevelFull {
                t.Fatalf("audit meter level = %q, want full", auditView.Level)
        }
        if _, err := world.entitlement.SetBudget(context.Background(), memberBudgetCommand(world, auditor)); err == nil {
                t.Fatal("audit scope must not manage the budget")
        }
}

// seedMember inserts a membership with the given roles and scopes directly
// through the admin pool (the fixture path used by the privacy tests).
func seedMember(t *testing.T, world *entitlementWorld, suffix int, roles []identityDomain.Role, scopes []identityDomain.Scope) identityDomain.Principal {
        t.Helper()
        member := identityDomain.Principal{ID: id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", 800000+suffix)), CanCreateTenant: true}
        ctx := context.Background()
        if _, err := adminPool.Exec(ctx, `insert into identity.principals (principal_id, created_at) values ($1, $2)`,
                member.ID, entitlementClock.Now()); err != nil {
                t.Fatal(err)
        }
        roleTexts := make([]string, 0, len(roles))
        for _, role := range roles {
                roleTexts = append(roleTexts, string(role))
        }
        scopeTexts := make([]string, 0, len(scopes))
        for _, scope := range scopes {
                scopeTexts = append(scopeTexts, string(scope))
        }
        if _, err := adminPool.Exec(ctx, `insert into identity.memberships (tenant_id, principal_id, role, status, created_at, roles, scopes, version, updated_at, email) values ($1, $2, 'owner', 'active', $3, $4, $5, 1, $3, $6)`,
                world.tenant, member.ID, entitlementClock.Now(), roleTexts, scopeTexts, fmt.Sprintf("member%d@example.test", suffix)); err != nil {
                t.Fatal(err)
        }
        return member
}

func usageMemberCommand(world *entitlementWorld, member identityDomain.Principal, key string) entitlementAPI.UsageEventCommand {
        command := usageCommand(world, key, "carrier", 15, 75, false)
        command.Actor = member
        return command
}

func memberTermsCommand(world *entitlementWorld, member identityDomain.Principal) entitlementAPI.TermsCommand {
        command := termsCommand(world, "RUB")
        command.Actor = member
        return command
}

func memberBudgetCommand(world *entitlementWorld, member identityDomain.Principal) entitlementAPI.BudgetCommand {
        command := budgetCommand(world, 50000, 0)
        command.Actor = member
        return command
}

// TestBudgetPublishedAfterUsageRaisesExceptions covers the R1 regression:
// publishing a budget over already ingested spend evaluates the exceptions
// immediately — the threshold crossing does not wait for the next ingest,
// and the re-published budget version re-raises under its own dedupe key.
func TestBudgetPublishedAfterUsageRaisesExceptions(t *testing.T) {
        world := newEntitlementWorld(t, 89)
        registerSource(t, world, "all-report", entitlementDomain.AllCategories()...)
        // Full-period coverage of every category keeps the data complete, so
        // the threshold message may carry the exact sum.
        for i, category := range entitlementDomain.AllCategories() {
                amount := 50.0
                if category == entitlementDomain.CategoryCarrier {
                        continue
                }
                if _, _, err := world.entitlement.IngestUsage(context.Background(), usageCommand(world, fmt.Sprintf("pre-budget-%d", i), string(category), 1, amount, false)); err != nil {
                        t.Fatal(err)
                }
        }
        if _, _, err := world.entitlement.IngestUsage(context.Background(), usageCommand(world, "pre-budget-carrier", "carrier", 120, 500, false)); err != nil {
                t.Fatal(err)
        }
        // No budget exists yet: no threshold exception can be raised.
        exceptions, err := world.entitlement.ExceptionsView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if countCause(exceptions, entitlementDomain.CauseBudgetThreshold) != 0 {
                t.Fatalf("no budget published: threshold exceptions = %d, want 0", countCause(exceptions, entitlementDomain.CauseBudgetThreshold))
        }
        // Publishing the budget with 800 confirmed against a threshold of
        // 80% of 1000 must raise the exception right at SetBudget time.
        if _, err := world.entitlement.SetBudget(context.Background(), budgetCommand(world, 1000, 0)); err != nil {
                t.Fatal(err)
        }
        exceptions, err = world.entitlement.ExceptionsView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if countCause(exceptions, entitlementDomain.CauseBudgetThreshold) != 1 {
                t.Fatalf("threshold exceptions after SetBudget = %d, want exactly 1", countCause(exceptions, entitlementDomain.CauseBudgetThreshold))
        }
        for _, exception := range exceptions.Exceptions {
                if exception.Cause == entitlementDomain.CauseBudgetThreshold {
                        if exception.DataStatus != entitlementDomain.DataComplete || !contains(exception.Message, "800.00") {
                                t.Fatalf("complete data must carry the exact sum: %+v", exception)
                        }
                }
        }
        // A second budget version of the same period re-raises the threshold
        // under its own version-stamped dedupe key (no silent suppression).
        if _, err := world.entitlement.SetBudget(context.Background(), budgetCommand(world, 1000, 0)); err != nil {
                t.Fatal(err)
        }
        exceptions, err = world.entitlement.ExceptionsView(context.Background(), world.principal, world.tenant)
        if err != nil {
                t.Fatal(err)
        }
        if countCause(exceptions, entitlementDomain.CauseBudgetThreshold) != 2 {
                t.Fatalf("threshold exceptions after the second budget version = %d, want 2 (one per version)", countCause(exceptions, entitlementDomain.CauseBudgetThreshold))
        }
}
