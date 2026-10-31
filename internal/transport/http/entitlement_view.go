package httptransport

import (
	"fmt"
	"strings"
	"time"

	entitlementAPI "github.com/staffy-co/staffy/internal/entitlement/api"
	entitlementDomain "github.com/staffy-co/staffy/internal/entitlement/domain"
)

// usageCategoryOptions lists the usage dictionary for the form selects.
func usageCategoryOptions() []map[string]any {
	options := make([]map[string]any, 0, len(entitlementDomain.CategorySpecs()))
	for _, spec := range entitlementDomain.CategorySpecs() {
		options = append(options, map[string]any{"ID": string(spec.ID), "Title": spec.Title, "Unit": spec.Unit, "Money": spec.Money})
	}
	return options
}

func entitlementStateText(state string) string {
	switch state {
	case entitlementDomain.StateActive:
		return "активен"
	case entitlementDomain.StatePaused:
		return "приостановлен"
	case entitlementDomain.StateEnded:
		return "завершён"
	default:
		return "черновик (не активирован)"
	}
}

func dataStatusText(status string) string {
	switch status {
	case entitlementDomain.DataComplete:
		return "полные"
	case entitlementDomain.DataPartial:
		return "частичные"
	case entitlementDomain.DataStale:
		return "устаревшие"
	default:
		return "неизвестны"
	}
}

func budgetQualityText(quality string) string {
	switch quality {
	case "over_budget":
		return "бюджет превышен"
	case "near_threshold":
		return "близко к порогу предупреждения"
	case "within_budget":
		return "в рамках бюджета"
	default:
		return "бюджет не задан"
	}
}

func admissionStatusText(status string) string {
	switch status {
	case entitlementDomain.AdmissionBlocked:
		return "новые обращения заблокированы"
	case entitlementDomain.AdmissionDegraded:
		return "работа в деградированном режиме"
	default:
		return "допуск открыт"
	}
}

// entitlementViewData is the whole entitlement page payload.
type entitlementViewData struct {
	Level             string
	State             string
	StateText         string
	Composition       []compositionRow
	OpenQuestions     []string
	Terms             *termsView
	History           []termsHistoryRow
	StateEvents       []stateEventView
	Budget            *budgetView
	BudgetQuality     string
	BudgetQualityText string
	Admission         admissionView
	Sources           []sourceView
	IsFull            bool
}

type compositionRow struct {
	Title string
	Unit  string
	Kind  string
}

type termsView struct {
	Version            int64
	Currency           string
	TaxRegime          string
	PeriodText         string
	IncludedEmployees  string
	IncludedChannels   string
	IncludedConnectors string
	Limits             []limitRow
	LaunchPrice        string
	SubscriptionPrice  string
	ChangeNote         string
	PublishedAt        string
}

type limitRow struct {
	Category string
	Title    string
	Cap      string
	Unit     string
}

type termsHistoryRow struct {
	Version     int64
	ChangeNote  string
	PublishedAt string
}

type stateEventView struct {
	Event        string
	EventText    string
	TermsVersion int64
	Basis        string
	LimitsCheck  string
	ActorID      string
	CreatedAt    string
}

type budgetView struct {
	Version        int64
	Amount         string
	WarningPercent int
	HardLimit      string
	LimitPolicy    string
	PeriodText     string
	Note           string
	AnomalyText    []string
	PublishedAt    string
}

type admissionView struct {
	Status       string
	StatusText   string
	Reason       string
	Policy       string
	Continues    []string
	Owner        string
	HasContinues bool
}

type sourceView struct {
	Name           string
	CategoriesText string
	FreshnessText  string
	Retired        bool
	Note           string
	CreatedAt      string
}

func money(value float64, currency string) string {
	if currency == "" {
		return "— открытый вопрос —"
	}
	return fmt.Sprintf("%.2f %s", value, currency)
}

func optionalMoney(value *float64, currency string) string {
	if value == nil {
		return "— открытый вопрос (не ноль) —"
	}
	return money(*value, currency)
}

func formatDay(t time.Time) string { return t.Format("02.01.2006") }

func formatTimestamp(t time.Time) string { return t.Format("02.01.2006 15:04 UTC") }

// prepareEntitlementView projects the derived commercial view into template
// values with readable Russian statuses.
func prepareEntitlementView(view entitlementAPI.EntitlementView) entitlementViewData {
	data := entitlementViewData{
		Level:             view.Level,
		State:             view.State,
		StateText:         entitlementStateText(view.State),
		Composition:       make([]compositionRow, 0, len(view.Composition)+2),
		OpenQuestions:     view.OpenQuestions,
		BudgetQuality:     view.BudgetQuality,
		BudgetQualityText: budgetQualityText(view.BudgetQuality),
		IsFull:            view.Level == entitlementAPI.LevelFull,
	}
	data.Composition = append(data.Composition, compositionRow{Title: "Запуск (разовая часть)", Unit: "услуга", Kind: "launch"})
	data.Composition = append(data.Composition, compositionRow{Title: "Подписка ИИ-сотрудника", Unit: "период", Kind: "subscription"})
	for _, spec := range view.Composition {
		data.Composition = append(data.Composition, compositionRow{Title: spec.Title, Unit: spec.Unit, Kind: "usage"})
	}
	if view.Terms != nil {
		terms := &termsView{
			Version:            view.Terms.Version,
			Currency:           view.Terms.Currency,
			TaxRegime:          view.Terms.TaxRegime,
			IncludedEmployees:  joinOrUnknown(view.Terms.IncludedEmployees),
			IncludedChannels:   joinOrUnknown(view.Terms.IncludedChannels),
			IncludedConnectors: joinOrUnknown(view.Terms.IncludedConnectors),
			LaunchPrice:        optionalMoney(view.Terms.LaunchPrice, view.Terms.Currency),
			SubscriptionPrice:  optionalMoney(view.Terms.SubscriptionPrice, view.Terms.Currency),
			ChangeNote:         view.Terms.ChangeNote,
			PublishedAt:        formatTimestamp(view.Terms.CreatedAt),
		}
		if terms.Currency == "" {
			terms.Currency = "— открытый вопрос —"
		}
		if terms.TaxRegime == "" {
			terms.TaxRegime = "— явная неопределённость —"
		}
		if view.Terms.PeriodStart != nil && view.Terms.PeriodEnd != nil {
			terms.PeriodText = formatDay(*view.Terms.PeriodStart) + " — " + formatDay(*view.Terms.PeriodEnd)
		} else {
			terms.PeriodText = "— открытый вопрос —"
		}
		titles := entitlementDomain.CategoryTitles()
		for _, limit := range view.Terms.Limits {
			terms.Limits = append(terms.Limits, limitRow{Category: string(limit.Category), Title: titles[limit.Category], Cap: fmt.Sprintf("%.2f", limit.Cap), Unit: limit.Unit})
		}
		data.Terms = terms
	}
	for _, version := range view.History {
		note := version.ChangeNote
		if note == "" {
			note = "—"
		}
		data.History = append(data.History, termsHistoryRow{Version: version.Version, ChangeNote: note, PublishedAt: formatTimestamp(version.CreatedAt)})
	}
	for _, event := range view.StateEvents {
		data.StateEvents = append(data.StateEvents, stateEventView{
			Event: event.Event, EventText: stateEventText(event.Event),
			TermsVersion: event.TermsVersion, Basis: event.Basis, LimitsCheck: event.LimitsCheck,
			ActorID: event.ActorID, CreatedAt: formatTimestamp(event.CreatedAt),
		})
	}
	if view.Budget != nil {
		data.Budget = prepareBudgetView(view.Budget)
	}
	data.Admission = admissionView{
		Status:       view.Admission.Status,
		StatusText:   admissionStatusText(view.Admission.Status),
		Reason:       view.Admission.Reason,
		Policy:       view.Admission.Policy,
		Continues:    view.Admission.Continues,
		Owner:        view.Admission.Owner,
		HasContinues: len(view.Admission.Continues) > 0,
	}
	for _, source := range view.Sources {
		data.Sources = append(data.Sources, sourceView{
			Name:           source.Name,
			CategoriesText: strings.Join(categoryStrings(source.Categories), ", "),
			FreshnessText:  fmt.Sprintf("%d мин", source.FreshnessMinutes),
			Retired:        source.Retired,
			Note:           source.Note,
			CreatedAt:      formatTimestamp(source.CreatedAt),
		})
	}
	return data
}

func stateEventText(event string) string {
	switch event {
	case entitlementDomain.EventActivate:
		return "активация"
	case entitlementDomain.EventPause:
		return "приостановка"
	case entitlementDomain.EventResume:
		return "возобновление"
	case entitlementDomain.EventEnd:
		return "завершение"
	}
	return event
}

func categoryStrings(categories []entitlementDomain.UsageCategory) []string {
	out := make([]string, 0, len(categories))
	for _, category := range categories {
		out = append(out, string(category))
	}
	return out
}

func joinOrUnknown(values []string) string {
	if len(values) == 0 {
		return "— не включено (не определено) —"
	}
	return strings.Join(values, ", ")
}

func prepareBudgetView(budget *entitlementDomain.BudgetVersion) *budgetView {
	view := &budgetView{
		Version:        budget.Version,
		Amount:         money(budget.Amount, budget.Currency),
		WarningPercent: budget.WarningPercent,
		LimitPolicy:    budget.LimitPolicy,
		PeriodText:     formatDay(budget.PeriodStart) + " — " + formatDay(budget.PeriodEnd),
		Note:           budget.Note,
		PublishedAt:    formatTimestamp(budget.CreatedAt),
	}
	if budget.HardLimit != nil {
		view.HardLimit = money(*budget.HardLimit, budget.Currency)
	} else {
		view.HardLimit = "не задан"
	}
	for _, threshold := range budget.AnomalyThresholds {
		view.AnomalyText = append(view.AnomalyText, fmt.Sprintf("%s: норма %.2f %s", threshold.Category, threshold.MaxQuantity, threshold.Unit))
	}
	return view
}

// ---------------------------------------------------------------------------
// Budget Meter page
// ---------------------------------------------------------------------------

type meterViewData struct {
	Level             string
	DataStatus        string
	DataStatusText    string
	PeriodText        string
	LastIngestedText  string
	AggRuleVersion    string
	Categories        []categoryMeterRow
	MissingCategories string
	ConfirmedText     string
	EstimateText      string
	Budget            *budgetRelationView
	BudgetQualityText string
	Breakdowns        []breakdownRowView
	IsFull            bool
}

type categoryMeterRow struct {
	Title            string
	Category         string
	Unit             string
	EventCount       int64
	Quantity         string
	ConfirmedEntries []string
	EstimateEntries  []string
	Status           string
	StatusText       string
	StatusNote       string
	Sources          string
	LastIngestedText string
	StatusClass      string
}

type budgetRelationView struct {
	Amount    string
	Ratio     string
	RatioNote string
	Warned    bool
	Over      bool
}

type breakdownRowView struct {
	Dimension string
	Value     string
	Quantity  string
	Money     string
}

type exceptionRowView struct {
	ExceptionID       string
	Cause             string
	CauseText         string
	Message           string
	DataStatusText    string
	AffectedEmployee  string
	AffectedRoute     string
	RecommendedAction string
	CreatedAt         string
	AcknowledgedText  string
	Acked             bool
}

func causeText(cause string) string {
	switch cause {
	case entitlementDomain.CauseBudgetThreshold:
		return "приближение к порогу бюджета"
	case entitlementDomain.CauseHardLimit:
		return "достижение hard limit"
	case entitlementDomain.CauseDataGap:
		return "разрыв данных usage"
	case entitlementDomain.CauseAnomalyTools:
		return "аномальные вызовы инструментов"
	case entitlementDomain.CauseAnomalyRetryStorm:
		return "retry storm"
	case entitlementDomain.CauseAnomalyManual:
		return "непропорциональные ручные минуты"
	case entitlementDomain.CauseManualEntry:
		return "ручное наблюдение"
	}
	return cause
}

func moneyEntries(entries map[string]float64) []string {
	if len(entries) == 0 {
		return nil
	}
	currencies := make([]string, 0, len(entries))
	for currency := range entries {
		currencies = append(currencies, currency)
	}
	sortStrings(currencies)
	out := make([]string, 0, len(currencies))
	for _, currency := range currencies {
		out = append(out, money(entries[currency], currency))
	}
	return out
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func statusClass(status string) string {
	switch status {
	case entitlementDomain.DataComplete:
		return "meter-complete"
	case entitlementDomain.DataPartial:
		return "meter-partial"
	case entitlementDomain.DataStale:
		return "meter-stale"
	default:
		return "meter-unknown"
	}
}

// prepareMeterView projects the derived meter into template values: sums as
// text with currencies and units, statuses as words, never color-only.
func prepareMeterView(view entitlementAPI.MeterView) meterViewData {
	data := meterViewData{
		Level:             view.Level,
		DataStatus:        view.DataStatus,
		DataStatusText:    dataStatusText(view.DataStatus),
		AggRuleVersion:    view.AggRuleVersion,
		Categories:        make([]categoryMeterRow, 0, len(view.Categories)),
		BudgetQualityText: budgetQualityText(view.BudgetQuality),
		IsFull:            view.Level == entitlementAPI.LevelFull,
	}
	if view.PeriodStart != nil && view.PeriodEnd != nil {
		data.PeriodText = formatDay(*view.PeriodStart) + " — " + formatDay(*view.PeriodEnd)
	} else {
		data.PeriodText = "— период измерения не задан (бюджет не опубликован) —"
	}
	if view.LastIngestedAt != nil {
		data.LastIngestedText = formatTimestamp(*view.LastIngestedAt)
	} else {
		data.LastIngestedText = "события ещё не поступали"
	}
	for _, row := range view.Categories {
		data.Categories = append(data.Categories, categoryMeterRow{
			Title:            row.Title,
			Category:         string(row.Category),
			Unit:             row.Unit,
			EventCount:       row.EventCount,
			Quantity:         fmt.Sprintf("%.2f %s", row.Quantity, row.Unit),
			ConfirmedEntries: moneyEntries(row.MoneyConfirmed),
			EstimateEntries:  moneyEntries(row.MoneyEstimate),
			Status:           row.Status,
			StatusText:       dataStatusText(row.Status),
			StatusNote:       row.StatusNote,
			Sources:          strings.Join(row.Sources, ", "),
			LastIngestedText: lastIngested(row.LastIngestedAt),
			StatusClass:      statusClass(row.Status),
		})
	}
	if len(view.MissingCategories) > 0 {
		data.MissingCategories = strings.Join(view.MissingCategories, ", ")
	}
	if len(view.ConfirmedTotal) > 0 {
		data.ConfirmedText = strings.Join(moneyEntries(view.ConfirmedTotal), "; ")
	} else {
		data.ConfirmedText = "нет подтверждённых начислений"
	}
	if len(view.EstimateTotal) > 0 {
		data.EstimateText = strings.Join(moneyEntries(view.EstimateTotal), "; ")
	} else {
		data.EstimateText = "нет оценок"
	}
	if view.Budget != nil {
		relation := &budgetRelationView{
			Amount:    money(view.Budget.Amount, view.Budget.Currency),
			RatioNote: view.Budget.RatioNote,
			Warned:    view.Budget.Warned,
			Over:      view.Budget.Over,
		}
		if view.Budget.Ratio != nil {
			relation.Ratio = fmt.Sprintf("%.0f%% бюджета", *view.Budget.Ratio*100)
		} else {
			relation.Ratio = "— открытый вопрос —"
		}
		data.Budget = relation
	}
	for _, row := range view.Breakdowns {
		dimension := map[string]string{"employee": "ИИ-сотрудник", "channel": "канал", "release": "релиз"}[row.Dimension]
		data.Breakdowns = append(data.Breakdowns, breakdownRowView{
			Dimension: dimension,
			Value:     row.Value,
			Quantity:  fmt.Sprintf("%.2f", row.Quantity),
			Money:     fmt.Sprintf("%.2f", row.Money),
		})
	}
	return data
}

func lastIngested(at *time.Time) string {
	if at == nil {
		return "—"
	}
	return formatTimestamp(*at)
}

// prepareExceptionViews projects the exceptions into readable rows.
func prepareExceptionViews(view entitlementAPI.ExceptionsView) []exceptionRowView {
	out := make([]exceptionRowView, 0, len(view.Exceptions))
	for _, exception := range view.Exceptions {
		row := exceptionRowView{
			ExceptionID:       exception.ExceptionID,
			Cause:             exception.Cause,
			CauseText:         causeText(exception.Cause),
			Message:           exception.Message,
			DataStatusText:    dataStatusText(exception.DataStatus),
			AffectedEmployee:  exception.AffectedEmployee,
			AffectedRoute:     exception.AffectedRoute,
			RecommendedAction: exception.RecommendedAction,
			CreatedAt:         formatTimestamp(exception.CreatedAt),
		}
		if exception.Acknowledged != nil {
			row.Acked = true
			row.AcknowledgedText = "подтверждено " + formatTimestamp(exception.Acknowledged.CreatedAt) + " (заметка: " + exception.Acknowledged.Note + ")"
		} else {
			row.AcknowledgedText = "не подтверждено"
		}
		out = append(out, row)
	}
	return out
}
