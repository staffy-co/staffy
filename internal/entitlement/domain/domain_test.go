package domain

import (
        "testing"
        "time"
)

func TestDeriveEntitlementStateFold(t *testing.T) {
        base := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
        events := func(pairs ...[2]string) []StateEvent {
                out := []StateEvent{}
                for i, pair := range pairs {
                        out = append(out, StateEvent{
                                EventID: string(rune('a'+i)), Event: pair[0],
                                CreatedAt: base.Add(time.Duration(i) * time.Minute),
                        })
                }
                return out
        }
        cases := []struct {
                name   string
                events [][2]string
                want   string
        }{
                {"no events", nil, StateDraft},
                {"activate", [][2]string{{"activate", ""}}, StateActive},
                {"activate pause", [][2]string{{"activate", ""}, {"pause", ""}}, StatePaused},
                {"activate pause resume", [][2]string{{"activate", ""}, {"pause", ""}, {"resume", ""}}, StateActive},
                {"activate end", [][2]string{{"activate", ""}, {"end", ""}}, StateEnded},
                {"end is terminal", [][2]string{{"activate", ""}, {"end", ""}, {"resume", ""}}, StateEnded},
                {"pause without activate is ignored", [][2]string{{"pause", ""}}, StateDraft},
        }
        for _, testCase := range cases {
                t.Run(testCase.name, func(t *testing.T) {
                        if got := DeriveEntitlementState(events(testCase.events...)); got != testCase.want {
                                t.Fatalf("state = %q, want %q", got, testCase.want)
                        }
                })
        }
}

func TestValidateTransition(t *testing.T) {
        valid := [][2]string{
                {StateDraft, EventActivate}, {StateActive, EventPause},
                {StatePaused, EventResume}, {StateActive, EventEnd}, {StatePaused, EventEnd},
        }
        for _, pair := range valid {
                if err := ValidateTransition(pair[0], pair[1]); err != nil {
                        t.Fatalf("transition %s→%s must be valid: %v", pair[0], pair[1], err)
                }
        }
        invalid := [][2]string{
                {StateActive, EventActivate}, {StateDraft, EventPause}, {StateActive, EventResume},
                {StateDraft, EventEnd}, {StateEnded, EventResume},
        }
        for _, pair := range invalid {
                if err := ValidateTransition(pair[0], pair[1]); err == nil {
                        t.Fatalf("transition %s→%s must be rejected", pair[0], pair[1])
                }
        }
}

func TestDeriveAdmissionContinuesNeverSilentlyCut(t *testing.T) {
        limit := 1000.0
        budget := &BudgetVersion{Amount: 800, Currency: "RUB", HardLimit: &limit, LimitPolicy: PolicyBlockNew}
        blocked := DeriveAdmission(StateActive, map[string]float64{"RUB": 1000}, budget)
        if blocked.Status != AdmissionBlocked {
                t.Fatalf("status = %q, want blocked", blocked.Status)
        }
        if len(blocked.Continues) != 4 || blocked.Owner != AdmissionOwner {
                t.Fatalf("continues/owner must be part of the answer: %+v", blocked)
        }
        paused := DeriveAdmission(StatePaused, nil, nil)
        if paused.Status != AdmissionBlocked || len(paused.Continues) != 4 {
                t.Fatalf("paused admission = %+v", paused)
        }
        draft := DeriveAdmission(StateDraft, nil, nil)
        if draft.Status != AdmissionBlocked {
                t.Fatalf("draft admission = %+v", draft)
        }
        degraded := DeriveAdmission(StateActive, map[string]float64{"RUB": 1000}, &BudgetVersion{Amount: 800, Currency: "RUB", HardLimit: &limit, LimitPolicy: PolicyDegraded})
        if degraded.Status != AdmissionDegraded {
                t.Fatalf("degraded admission = %+v", degraded)
        }
        ok := DeriveAdmission(StateActive, map[string]float64{"RUB": 100}, budget)
        if ok.Status != AdmissionOK {
                t.Fatalf("ok admission = %+v", ok)
        }
        // Estimates never reach the hard limit decision: 5000 of estimate
        // money in a foreign currency keeps the admission open.
        estimates := DeriveAdmission(StateActive, map[string]float64{"EUR": 5000}, budget)
        if estimates.Status != AdmissionOK {
                t.Fatalf("estimate money must not trip the limit: %+v", estimates)
        }
}

func TestComputeMeterUnknownIsNotZero(t *testing.T) {
        now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
        meter := ComputeMeter(nil, nil, nil, now)
        if meter.DataStatus != DataUnknown {
                t.Fatalf("empty meter status = %q, want unknown", meter.DataStatus)
        }
        for _, row := range meter.Categories {
                if len(row.MoneyConfirmed) != 0 {
                        t.Fatalf("unknown category must not carry sums: %+v", row)
                }
                if row.Status != DataUnknown || row.StatusNote == "" {
                        t.Fatalf("unknown category needs an honest note: %+v", row)
                }
        }
        if len(meter.MissingCategories) != len(AllCategories()) {
                t.Fatalf("missing categories = %v", meter.MissingCategories)
        }
        if meter.Budget != nil {
                t.Fatal("no budget → no budget projection")
        }
}

func fullPeriod() (time.Time, time.Time) {
        return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 31, 23, 59, 59, 0, time.UTC)
}

func TestComputeMeterSeparatesEstimateFromConfirmed(t *testing.T) {
        start, end := fullPeriod()
        now := end
        budget := &BudgetVersion{Amount: 1000, Currency: "RUB", WarningPercent: 80, PeriodStart: start, PeriodEnd: end}
        events := []UsageEvent{
                {Category: CategoryCarrier, Quantity: 100, Amount: float64Ptr(500), Currency: "RUB", PeriodStart: start, PeriodEnd: end, IngestedAt: now},
                {Category: CategoryCarrier, Quantity: 50, Amount: float64Ptr(400), Currency: "RUB", Estimate: true, PeriodStart: start, PeriodEnd: end, IngestedAt: now},
        }
        sources := map[string]UsageSource{
                "report": {Name: "report", Categories: []UsageCategory{CategoryCarrier}, FreshnessMinutes: 1440},
        }
        meter := ComputeMeter(events, sources, budget, now)
        if meter.ConfirmedTotal["RUB"] != 500 || meter.EstimateTotal["RUB"] != 400 {
                t.Fatalf("confirmed = %v, estimate = %v: they must not mix", meter.ConfirmedTotal, meter.EstimateTotal)
        }
        if meter.Budget == nil || meter.Budget.Warned {
                t.Fatal("the 50% confirmed ratio must not trip the 80% warning threshold")
        }
}

func TestComputeMeterPartialCoverageAndStaleFreshness(t *testing.T) {
        start, end := fullPeriod()
        now := end.Add(48 * time.Hour)
        budget := &BudgetVersion{Amount: 1000, Currency: "RUB", WarningPercent: 80, PeriodStart: start, PeriodEnd: end}
        sources := map[string]UsageSource{
                "report": {Name: "report", Categories: []UsageCategory{CategoryCarrier}, FreshnessMinutes: 60},
        }
        partial := []UsageEvent{
                {Category: CategoryCarrier, Quantity: 10, PeriodStart: start.Add(48 * time.Hour), PeriodEnd: end.Add(-48 * time.Hour), IngestedAt: now.Add(-2 * time.Hour)},
        }
        meter := ComputeMeter(partial, sources, budget, now)
        row, ok := categoryMeter(meter, CategoryCarrier)
        if !ok || row.Status != DataStale {
                t.Fatalf("stale freshness must win: %+v", row)
        }
        fresh := []UsageEvent{
                {Category: CategoryCarrier, Quantity: 10, PeriodStart: start, PeriodEnd: end, IngestedAt: now},
        }
        meter = ComputeMeter(fresh, sources, budget, now)
        row, _ = categoryMeter(meter, CategoryCarrier)
        if row.Status != DataComplete {
                t.Fatalf("full coverage in freshness = %+v", row)
        }
}

func categoryMeter(meter Meter, category UsageCategory) (CategoryMeter, bool) {
        for _, row := range meter.Categories {
                if row.Category == category {
                        return row, true
                }
        }
        return CategoryMeter{}, false
}

func TestComputeMeterCurrencyMismatchStaysOpenQuestion(t *testing.T) {
        start, end := fullPeriod()
        now := end
        budget := &BudgetVersion{Amount: 1000, Currency: "RUB", WarningPercent: 80, PeriodStart: start, PeriodEnd: end}
        events := []UsageEvent{
                {Category: CategoryCarrier, Quantity: 10, Amount: float64Ptr(900), Currency: "EUR", PeriodStart: start, PeriodEnd: end, IngestedAt: now},
        }
        sources := map[string]UsageSource{
                "report": {Name: "report", Categories: []UsageCategory{CategoryCarrier}, FreshnessMinutes: 1440},
        }
        meter := ComputeMeter(events, sources, budget, now)
        if meter.Budget == nil || meter.Budget.RatioNote == "" {
                t.Fatalf("foreign currency sums must be named as an open question: %+v", meter.Budget)
        }
        if meter.ConfirmedTotal["EUR"] != 900 {
                t.Fatalf("the EUR sum stays itself: %v", meter.ConfirmedTotal)
        }
}

func TestComputeMeterBreakdownCarriesNoContent(t *testing.T) {
        start, end := fullPeriod()
        now := end
        events := []UsageEvent{
                {Category: CategoryCarrier, Quantity: 10, EmployeeID: "consultant-01", Channel: "jivo", ReleaseVersion: "rel-1", PeriodStart: start, PeriodEnd: end, IngestedAt: now},
        }
        meter := ComputeMeter(events, nil, nil, now)
        if len(meter.Breakdowns) != 3 {
                t.Fatalf("breakdowns = %+v, want employee/channel/release rows", meter.Breakdowns)
        }
        for _, row := range meter.Breakdowns {
                if row.Value == "" {
                        t.Fatalf("breakdown rows carry identifiers only: %+v", row)
                }
        }
}

func TestCurrentSourcesLatestRegistrationWins(t *testing.T) {
        base := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
        all := []UsageSource{
                {RegistrationID: "a", Name: "report", Categories: []UsageCategory{CategoryCarrier}, FreshnessMinutes: 60, CreatedAt: base},
                {RegistrationID: "b", Name: "report", Categories: []UsageCategory{CategoryCarrier}, FreshnessMinutes: 1440, Retired: true, CreatedAt: base.Add(time.Minute)},
                {RegistrationID: "c", Name: "other", Categories: []UsageCategory{CategoryTools}, FreshnessMinutes: 30, CreatedAt: base.Add(2 * time.Minute)},
        }
        current := CurrentSources(all)
        if _, active := current["report"]; active {
                t.Fatal("the retired latest registration must remove the source")
        }
        if _, ok := current["other"]; !ok {
                t.Fatal("the active source stays")
        }
}

func TestTermsVersionOpenQuestionsAndActivationGate(t *testing.T) {
        empty := TermsVersion{}
        if len(empty.OpenQuestions()) < 10 {
                t.Fatalf("the empty version must list every open question: %v", empty.OpenQuestions())
        }
        if err := empty.ReadyForActivation(); err == nil {
                t.Fatal("activation without currency and period must be rejected")
        }
        start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
        ready := TermsVersion{Currency: "RUB", PeriodStart: &start}
        if err := ready.ReadyForActivation(); err != nil {
                t.Fatalf("currency + period start are activation-ready: %v", err)
        }
}

func float64Ptr(value float64) *float64 { return &value }
