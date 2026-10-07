package domain

import (
        "sort"
        "time"
)

// CategoryMeter is the derived projection of one usage category over the
// meter period: quantities, per-currency confirmed and estimated sums, the
// honest data status with its reason, the registered sources and the last
// ingest moment.
type CategoryMeter struct {
        Category       UsageCategory     `json:"category"`
        Title          string            `json:"title"`
        Unit           string            `json:"unit"`
        EventCount     int64             `json:"event_count"`
        Quantity       float64           `json:"quantity"`
        MoneyConfirmed map[string]float64 `json:"money_confirmed"`
        MoneyEstimate  map[string]float64 `json:"money_estimate"`
        Status         string            `json:"status"`
        StatusNote     string            `json:"status_note"`
        Sources        []string          `json:"sources"`
        LastIngestedAt *time.Time        `json:"last_ingested_at,omitempty"`
}

// BreakdownRow links the aggregate to an employee, channel or release
// version without any conversation content: dimension, label, quantity and
// the confirmed money of the budget currency.
type BreakdownRow struct {
        Dimension string  `json:"dimension"`
        Value     string  `json:"value"`
        Quantity  float64 `json:"quantity"`
        Money     float64 `json:"money"`
}

// BudgetView is the budget projection of the meter: the ratio of the
// confirmed spend of the budget currency to the budget amount, with honest
// notes instead of silent currency conversion.
type BudgetView struct {
        Version   int64    `json:"version"`
        Amount    float64  `json:"amount"`
        Currency  string   `json:"currency"`
        Ratio     *float64 `json:"ratio,omitempty"`
        RatioNote string   `json:"ratio_note"`
        Warned    bool     `json:"warned"`
        Over      bool     `json:"over"`
}

// Meter is the whole derived Budget Meter: honest per-category statuses,
// per-currency totals (confirmed and estimated separately), the budget
// relation, the overall data status and the breakdowns. Aggregates are
// computed on read — there is no materialized aggregate table and no second
// writer.
type Meter struct {
        PeriodStart       *time.Time            `json:"period_start,omitempty"`
        PeriodEnd         *time.Time            `json:"period_end,omitempty"`
        AggRuleVersion    string                `json:"agg_rule_version"`
        Categories        []CategoryMeter       `json:"categories"`
        MissingCategories []string              `json:"missing_categories"`
        ConfirmedTotal    map[string]float64    `json:"confirmed_total"`
        EstimateTotal     map[string]float64    `json:"estimate_total"`
        Budget            *BudgetView           `json:"budget,omitempty"`
        DataStatus        string                `json:"data_status"`
        LastIngestedAt    *time.Time            `json:"last_ingested_at,omitempty"`
        Breakdowns        []BreakdownRow        `json:"breakdowns"`
}

func addMoney(target map[string]float64, currency string, amount float64) {
        if currency == "" {
                return
        }
        target[currency] += amount
}

func round2(value float64) float64 {
        return float64(int64(value*100+0.5)) / 100
}

func dataStatusRank(status string) int {
        switch status {
        case DataUnknown:
                return 3
        case DataStale:
                return 2
        case DataPartial:
                return 1
        default:
                return 0
        }
}

// ComputeMeter derives the whole meter from the usage events, the active
// source registrations and the current budget. Unknown spend stays unknown:
// it is never reported as a zero sum. The estimate sums never mix with the
// confirmed accruals, and the budget ratio is computed only in the budget
// currency — a mismatch stays an honest open question.
func ComputeMeter(events []UsageEvent, sources map[string]UsageSource, budget *BudgetVersion, now time.Time) Meter {
        meter := Meter{
                AggRuleVersion: AggregationRuleVersion,
                ConfirmedTotal: map[string]float64{},
                EstimateTotal:  map[string]float64{},
                DataStatus:     DataComplete,
                Categories:     make([]CategoryMeter, 0, len(categoryRegistry)),
        }
        if budget != nil {
                periodStart, periodEnd := budget.PeriodStart, budget.PeriodEnd
                meter.PeriodStart, meter.PeriodEnd = &periodStart, &periodEnd
        }
        inPeriod := func(event UsageEvent) bool {
                if budget == nil {
                        return true
                }
                return !event.PeriodStart.Before(budget.PeriodStart) && !event.PeriodEnd.After(budget.PeriodEnd)
        }
        // Source coverage index: category → active source names.
        coveringSources := map[UsageCategory][]string{}
        sourceFreshness := map[string]time.Duration{}
        names := make([]string, 0, len(sources))
        for name, source := range sources {
                names = append(names, name)
                sourceFreshness[name] = time.Duration(source.FreshnessMinutes) * time.Minute
        }
        sort.Strings(names)
        for _, name := range names {
                for _, category := range sources[name].Categories {
                        coveringSources[category] = append(coveringSources[category], name)
                }
        }
        var lastIngested *time.Time
        for _, spec := range categoryRegistry {
                row := CategoryMeter{
                        Category: spec.ID, Title: spec.Title, Unit: spec.Unit,
                        MoneyConfirmed: map[string]float64{},
                        MoneyEstimate:  map[string]float64{},
                        Sources:        coveringSources[spec.ID],
                }
                var coveredFrom, coveredTo *time.Time
                var rowLastIngested *time.Time
                for _, event := range events {
                        if event.Category != spec.ID || !inPeriod(event) {
                                continue
                        }
                        row.EventCount++
                        row.Quantity += event.Quantity
                        if event.Amount != nil {
                                if event.Estimate {
                                        addMoney(row.MoneyEstimate, event.Currency, *event.Amount)
                                } else {
                                        addMoney(row.MoneyConfirmed, event.Currency, *event.Amount)
                                }
                        }
                        if coveredFrom == nil || event.PeriodStart.Before(*coveredFrom) {
                                moment := event.PeriodStart
                                coveredFrom = &moment
                        }
                        if coveredTo == nil || event.PeriodEnd.After(*coveredTo) {
                                moment := event.PeriodEnd
                                coveredTo = &moment
                        }
                        if rowLastIngested == nil || event.IngestedAt.After(*rowLastIngested) {
                                moment := event.IngestedAt
                                rowLastIngested = &moment
                        }
                }
                if rowLastIngested != nil {
                        row.LastIngestedAt = rowLastIngested
                        if lastIngested == nil || rowLastIngested.After(*lastIngested) {
                                lastIngested = rowLastIngested
                        }
                }
                row.Status, row.StatusNote = deriveCategoryStatus(spec, coveringSources[spec.ID], sourceFreshness, budget, coveredFrom, coveredTo, rowLastIngested, row.EventCount, now)
                if dataStatusRank(row.Status) > dataStatusRank(meter.DataStatus) {
                        meter.DataStatus = row.Status
                }
                for currency, sum := range row.MoneyConfirmed {
                        meter.ConfirmedTotal[currency] += round2(sum)
                }
                for currency, sum := range row.MoneyEstimate {
                        meter.EstimateTotal[currency] += round2(sum)
                }
                meter.Categories = append(meter.Categories, row)
                if len(coveringSources[spec.ID]) == 0 {
                        meter.MissingCategories = append(meter.MissingCategories, string(spec.ID))
                }
        }
        meter.LastIngestedAt = lastIngested
        if budget != nil {
                meter.Budget = projectBudget(meter.ConfirmedTotal, budget)
        }
        meter.Breakdowns = breakdowns(events, budget, inPeriod)
        return meter
}

// deriveCategoryStatus decides the honest data status of one category:
// no registered source or no events → unknown; the last ingest older than
// the source freshness → stale; the event coverage not spanning the meter
// period → partial; otherwise complete. Without a meter period (no budget
// published) completeness is honestly unknown.
func deriveCategoryStatus(spec CategorySpec, sourceNames []string, freshness map[string]time.Duration, budget *BudgetVersion, coveredFrom, coveredTo, lastIngested *time.Time, eventCount int64, now time.Time) (string, string) {
        if len(sourceNames) == 0 {
                return DataUnknown, "Нет зарегистрированного источника: расход не известен и не считается нулём."
        }
        if eventCount == 0 {
                return DataUnknown, "Зарегистрированные источники (" + joinNames(sourceNames) + ") не доставили событий: расход не известен и не считается нулём."
        }
        if lastIngested != nil {
                for _, name := range sourceNames {
                        if window, ok := freshness[name]; ok {
                                if lastIngested.Add(window).Before(now) {
                                        return DataStale, "Последнее обновление старше срока свежести источника " + name + ": данные могли устареть."
                                }
                        }
                }
        }
        if budget == nil {
                return DataUnknown, "Период измерения не задан (бюджет не опубликован): полноту покрытия определить невозможно."
        }
        if coveredFrom != nil && coveredTo != nil {
                if coveredFrom.After(budget.PeriodStart) || coveredTo.Before(budget.PeriodEnd) {
                        return DataPartial, "События покрывают только часть периода: " + coveredFrom.Format("02.01.2006") + " — " + coveredTo.Format("02.01.2006") + " из " + budget.PeriodStart.Format("02.01.2006") + " — " + budget.PeriodEnd.Format("02.01.2006") + "."
                }
        }
        return DataComplete, "Период покрыт событиями источников " + joinNames(sourceNames) + "."
}

func joinNames(names []string) string {
        out := ""
        for i, name := range names {
                if i > 0 {
                        out += ", "
                }
                out += name
        }
        return out
}

// projectBudget computes the budget relation: the ratio of the confirmed
// spend of the budget currency to the budget amount. Sums of other
// currencies are named as an open question — conversion does not exist.
func projectBudget(confirmed map[string]float64, budget *BudgetVersion) *BudgetView {
        view := &BudgetView{Version: budget.Version, Amount: budget.Amount, Currency: budget.Currency}
        ratio := confirmed[budget.Currency] / budget.Amount
        view.Ratio = &ratio
        others := make([]string, 0, len(confirmed))
        for currency := range confirmed {
                if currency != budget.Currency && confirmed[currency] > 0 {
                        others = append(others, currency)
                }
        }
        sort.Strings(others)
        if len(others) > 0 {
                view.RatioNote = "Есть подтверждённые суммы в других валютах (" + joinNames(others) + "): конвертация не выполняется, отношение к бюджету учитывает только " + budget.Currency + "."
        }
        view.Warned = ratio*100 >= float64(budget.WarningPercent)
        view.Over = ratio >= 1
        if HardLimitReached(confirmed, budget) {
                view.Over = true
        }
        return view
}

// breakdowns groups the in-period events by employee, channel and release
// version: identifiers and counters only, never conversation content.
func breakdowns(events []UsageEvent, budget *BudgetVersion, inPeriod func(UsageEvent) bool) []BreakdownRow {
        type key struct {
                dimension string
                value     string
        }
        sums := map[key]*BreakdownRow{}
        order := []key{}
        for _, event := range events {
                if !inPeriod(event) {
                        continue
                }
                dimensions := [][2]string{
                        {"employee", event.EmployeeID},
                        {"channel", event.Channel},
                        {"release", event.ReleaseVersion},
                }
                for _, pair := range dimensions {
                        if pair[1] == "" {
                                continue
                        }
                        k := key{dimension: pair[0], value: pair[1]}
                        row, seen := sums[k]
                        if !seen {
                                row = &BreakdownRow{Dimension: pair[0], Value: pair[1]}
                                sums[k] = row
                                order = append(order, k)
                        }
                        row.Quantity += event.Quantity
                        if event.Amount != nil && !event.Estimate && budget != nil && event.Currency == budget.Currency {
                                row.Money += *event.Amount
                        }
                }
        }
        sort.Slice(order, func(i, j int) bool {
                if order[i].dimension != order[j].dimension {
                        return order[i].dimension < order[j].dimension
                }
                return order[i].value < order[j].value
        })
        rows := make([]BreakdownRow, 0, len(order))
        for _, k := range order {
                row := sums[k]
                row.Quantity = round2(row.Quantity)
                row.Money = round2(row.Money)
                rows = append(rows, *row)
        }
        return rows
}
