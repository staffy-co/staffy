package main

import (
	"context"
	"sync"
	"time"

	entitlementAPI "github.com/artemiosu/staffy/internal/entitlement/api"
	entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// entitlementFixture is the in-memory commercial perimeter for the journey
// server: the full lifecycle (terms → activation → budget → ingest →
// exception → ack) works honestly so the browser journeys exercise the real
// page flow.
type entitlementFixture struct {
	mu          sync.Mutex
	terms       []entitlementDomain.TermsVersion
	stateEvents []entitlementDomain.StateEvent
	budgets     []entitlementDomain.BudgetVersion
	sources     []entitlementDomain.UsageSource
	events      []entitlementDomain.UsageEvent
	exceptions  []entitlementDomain.UsageException
	acks        []entitlementDomain.ExceptionAck
}

var entitlementFixtureNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func (f *entitlementFixture) EntitlementView(_ context.Context, _ domain.Principal, _ id.UUID) (entitlementAPI.EntitlementView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	view := entitlementAPI.EntitlementView{
		Level:       entitlementAPI.LevelFull,
		State:       entitlementDomain.DeriveEntitlementState(f.stateEvents),
		Composition: entitlementDomain.CategorySpecs(),
		StateEvents: f.stateEvents,
		Sources:     f.sources,
	}
	if len(f.terms) > 0 {
		current := f.terms[len(f.terms)-1]
		view.Terms = &current
		view.OpenQuestions = current.OpenQuestions()
		view.History = f.terms
	} else {
		view.OpenQuestions = []string{
			"коммерческое предложение не составлено: опишите версию условий",
			"цена запуска не определена (не ноль, а открытый вопрос)",
			"цена подписки ИИ-сотрудника не определена (не ноль, а открытый вопрос)",
		}
	}
	meter := entitlementDomain.ComputeMeter(f.events, entitlementDomain.CurrentSources(f.sources), currentBudget(f.budgets), entitlementFixtureNow)
	view.BudgetQuality = fixtureBudgetQuality(meter)
	if len(f.budgets) > 0 {
		current := f.budgets[len(f.budgets)-1]
		view.Budget = &current
	}
	view.Admission = entitlementDomain.DeriveAdmission(view.State, meter.ConfirmedTotal, currentBudget(f.budgets))
	return view, nil
}

func currentBudget(budgets []entitlementDomain.BudgetVersion) *entitlementDomain.BudgetVersion {
	if len(budgets) == 0 {
		return nil
	}
	return &budgets[len(budgets)-1]
}

func fixtureBudgetQuality(meter entitlementDomain.Meter) string {
	if meter.Budget == nil {
		return "budget_not_set"
	}
	switch {
	case meter.Budget.Over:
		return "over_budget"
	case meter.Budget.Warned:
		return "near_threshold"
	default:
		return "within_budget"
	}
}

func (f *entitlementFixture) MeterView(_ context.Context, _ domain.Principal, _ id.UUID) (entitlementAPI.MeterView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	meter := entitlementDomain.ComputeMeter(f.events, entitlementDomain.CurrentSources(f.sources), currentBudget(f.budgets), entitlementFixtureNow)
	view := entitlementAPI.MeterView{
		Level:             entitlementAPI.LevelFull,
		DataStatus:        meter.DataStatus,
		PeriodStart:       meter.PeriodStart,
		PeriodEnd:         meter.PeriodEnd,
		LastIngestedAt:    meter.LastIngestedAt,
		AggRuleVersion:    meter.AggRuleVersion,
		Categories:        meter.Categories,
		MissingCategories: meter.MissingCategories,
		ConfirmedTotal:    meter.ConfirmedTotal,
		EstimateTotal:     meter.EstimateTotal,
		Budget:            meter.Budget,
		Breakdowns:        meter.Breakdowns,
		BudgetQuality:     fixtureBudgetQuality(meter),
	}
	return view, nil
}

func (f *entitlementFixture) AdmissionView(_ context.Context, _ domain.Principal, _ id.UUID) (entitlementDomain.AdmissionState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	meter := entitlementDomain.ComputeMeter(f.events, entitlementDomain.CurrentSources(f.sources), currentBudget(f.budgets), entitlementFixtureNow)
	return entitlementDomain.DeriveAdmission(entitlementDomain.DeriveEntitlementState(f.stateEvents), meter.ConfirmedTotal, currentBudget(f.budgets)), nil
}

func (f *entitlementFixture) ExceptionsView(_ context.Context, _ domain.Principal, _ id.UUID) (entitlementAPI.ExceptionsView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	exceptions := append([]entitlementDomain.UsageException(nil), f.exceptions...)
	for i := range exceptions {
		for _, ack := range f.acks {
			if ack.ExceptionID == exceptions[i].ExceptionID {
				latest := exceptions[i].Acknowledged
				if latest == nil || ack.CreatedAt.After(latest.CreatedAt) {
					exceptions[i].Acknowledged = &ack
				}
			}
		}
	}
	return entitlementAPI.ExceptionsView{Level: entitlementAPI.LevelFull, Exceptions: exceptions}, nil
}

func (f *entitlementFixture) ComposeTerms(_ context.Context, c entitlementAPI.TermsCommand) (entitlementDomain.TermsVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	terms := entitlementDomain.TermsVersion{
		Version:  int64(len(f.terms) + 1),
		Currency: c.Currency, TaxRegime: c.TaxRegime,
		PeriodStart: c.PeriodStart, PeriodEnd: c.PeriodEnd,
		IncludedEmployees: c.IncludedEmployees, IncludedChannels: c.IncludedChannels, IncludedConnectors: c.IncludedConnectors,
		Limits: c.Limits, LaunchPrice: c.LaunchPrice, SubscriptionPrice: c.SubscriptionPrice,
		ChangeNote: c.ChangeNote,
		ActorID:    "01994f64-6c00-7000-8000-000000000001", CreatedAt: entitlementFixtureNow,
	}
	f.terms = append(f.terms, terms)
	return terms, nil
}

func (f *entitlementFixture) RunStateTransition(_ context.Context, c entitlementAPI.StateCommand) (entitlementDomain.StateEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state := entitlementDomain.DeriveEntitlementState(f.stateEvents)
	if err := entitlementDomain.ValidateTransition(state, c.Action); err != nil {
		return entitlementDomain.StateEvent{}, err
	}
	if len(f.terms) == 0 {
		return entitlementDomain.StateEvent{}, entitlementDomain.RuleError{Rule: "terms", Reason: "Составьте версию условий тарифа до изменения коммерческого состояния."}
	}
	event := entitlementDomain.StateEvent{
		EventID: "01994f64-6c00-7000-8000-000000000003", Event: c.Action,
		TermsVersion: f.terms[len(f.terms)-1].Version, Basis: c.Basis,
		ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: entitlementFixtureNow,
	}
	if c.Action == entitlementDomain.EventActivate {
		if !c.Confirm {
			return entitlementDomain.StateEvent{}, entitlementDomain.RuleError{Rule: "confirm", Reason: "Подтвердите активацию коммерческих условий."}
		}
		if err := f.terms[len(f.terms)-1].ReadyForActivation(); err != nil {
			return entitlementDomain.StateEvent{}, err
		}
	}
	if c.Action == entitlementDomain.EventResume {
		meter := entitlementDomain.ComputeMeter(f.events, entitlementDomain.CurrentSources(f.sources), currentBudget(f.budgets), entitlementFixtureNow)
		event.LimitsCheck = entitlementDomain.LimitsCheckResult(meter.ConfirmedTotal, currentBudget(f.budgets), entitlementFixtureNow)
	}
	f.stateEvents = append(f.stateEvents, event)
	return event, nil
}

func (f *entitlementFixture) SetBudget(_ context.Context, c entitlementAPI.BudgetCommand) (entitlementDomain.BudgetVersion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	budget := entitlementDomain.BudgetVersion{
		Version: int64(len(f.budgets) + 1), Amount: c.Amount, Currency: c.Currency,
		PeriodStart: c.PeriodStart, PeriodEnd: c.PeriodEnd, WarningPercent: c.WarningPercent,
		HardLimit: c.HardLimit, LimitPolicy: c.LimitPolicy, AnomalyThresholds: c.AnomalyThresholds,
		Note: c.Note, ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: entitlementFixtureNow,
	}
	if !entitlementDomain.LimitPolicies[budget.LimitPolicy] {
		return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "limit_policy", Reason: "Политика hard limit не входит в словарь (block_new_admissions / degraded / notify_only)."}
	}
	if c.HardLimit != nil && *c.HardLimit < c.Amount {
		return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "hard_limit", Reason: "Hard limit не может быть меньше суммы бюджета."}
	}
	f.budgets = append(f.budgets, budget)
	return budget, nil
}

func (f *entitlementFixture) RegisterSource(_ context.Context, c entitlementAPI.SourceCommand) (entitlementDomain.UsageSource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	source := entitlementDomain.UsageSource{
		RegistrationID: "01994f64-6c00-7000-8000-000000000006", Name: c.Name,
		Categories: c.Categories, FreshnessMinutes: c.FreshnessMinutes, Retired: c.Retire,
		Note: c.Note, ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: entitlementFixtureNow,
	}
	f.sources = append(f.sources, source)
	return source, nil
}

func (f *entitlementFixture) IngestUsage(_ context.Context, c entitlementAPI.UsageEventCommand) (entitlementDomain.UsageEvent, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	spec, err := entitlementDomain.LookupCategory(c.Category)
	if err != nil {
		return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: "category", Reason: "Категория расхода не найдена в словаре."}
	}
	if spec.Money && c.Amount == nil {
		return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: "amount", Reason: "Категория " + string(spec.ID) + " учитывает плату: укажите сумму и валюту."}
	}
	for _, existing := range f.events {
		if existing.EventKey == c.EventKey {
			if existing.PayloadHash == "" || existing.PayloadHash == fixturePayloadHash(c) {
				return existing, true, nil
			}
			return entitlementDomain.UsageEvent{}, false, entitlementDomain.ErrEventKeyConflict
		}
	}
	event := entitlementDomain.UsageEvent{
		EventID: "01994f64-6c00-7000-8000-000000000007", EventKey: c.EventKey,
		PayloadHash: fixturePayloadHash(c), Source: c.Source, Category: c.Category,
		Quantity: c.Quantity, Unit: spec.Unit, Amount: c.Amount, Currency: c.Currency,
		Estimate: c.Estimate, PeriodStart: c.PeriodStart, PeriodEnd: c.PeriodEnd,
		OccurredAt: entitlementFixtureNow, IngestedAt: entitlementFixtureNow,
		EmployeeID: c.EmployeeID, Channel: c.Channel, ReleaseVersion: c.ReleaseVersion,
		Provenance: c.Provenance, AggRuleVersion: entitlementDomain.AggregationRuleVersion,
		ActorID: "01994f64-6c00-7000-8000-000000000001",
	}
	f.events = append(f.events, event)
	f.evaluateExceptionsLocked()
	return event, false, nil
}

func (f *entitlementFixture) evaluateExceptionsLocked() {
	meter := entitlementDomain.ComputeMeter(f.events, entitlementDomain.CurrentSources(f.sources), currentBudget(f.budgets), entitlementFixtureNow)
	if meter.DataStatus != entitlementDomain.DataComplete {
		f.appendExceptionLocked(entitlementDomain.UsageException{
			ExceptionID: "01994f64-6c00-7000-8000-000000000008", Cause: entitlementDomain.CauseDataGap,
			Message:    "Отсутствуют данные категорий: расход не известен и не считается нулём.",
			DataStatus: meter.DataStatus, RecommendedAction: "Проверьте доставку событий от источников.",
			DedupeKey: "gap:fixture", AggRuleVersion: entitlementDomain.AggregationRuleVersion,
			ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: entitlementFixtureNow,
		})
	}
	budget := currentBudget(f.budgets)
	if budget == nil {
		return
	}
	if meter.Budget != nil && meter.Budget.Warned {
		f.appendExceptionLocked(entitlementDomain.UsageException{
			ExceptionID: "01994f64-6c00-7000-8000-000000000009", Cause: entitlementDomain.CauseBudgetThreshold,
			Message:    "Расход приближается к порогу предупреждения; при неполных данных точная сумма не утверждается.",
			DataStatus: meter.DataStatus, RecommendedAction: "Проверьте прогноз расхода до конца периода.",
			DedupeKey: "warn:fixture", AggRuleVersion: entitlementDomain.AggregationRuleVersion,
			ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: entitlementFixtureNow,
		})
	}
}

func (f *entitlementFixture) appendExceptionLocked(exception entitlementDomain.UsageException) {
	for _, existing := range f.exceptions {
		if existing.DedupeKey == exception.DedupeKey {
			return
		}
	}
	f.exceptions = append(f.exceptions, exception)
}

func fixturePayloadHash(c entitlementAPI.UsageEventCommand) string {
	return "fixture-" + c.EventKey + "-" + string(c.Category)
}

func (f *entitlementFixture) RaiseException(_ context.Context, c entitlementAPI.ManualExceptionCommand) (entitlementDomain.UsageException, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !entitlementDomain.ExceptionCauses[c.Cause] {
		return entitlementDomain.UsageException{}, entitlementDomain.RuleError{Rule: "cause", Reason: "Причина исключения не входит в словарь."}
	}
	exception := entitlementDomain.UsageException{
		ExceptionID: "01994f64-6c00-7000-8000-000000000010", Cause: c.Cause,
		Message: c.Message, DataStatus: entitlementDomain.DataUnknown,
		AffectedEmployee: c.AffectedEmployee, AffectedRoute: c.AffectedRoute,
		RecommendedAction: c.RecommendedAction, DedupeKey: "manual:" + c.Message,
		AggRuleVersion: entitlementDomain.AggregationRuleVersion,
		ActorID:        "01994f64-6c00-7000-8000-000000000001", CreatedAt: entitlementFixtureNow,
	}
	f.exceptions = append(f.exceptions, exception)
	return exception, nil
}

func (f *entitlementFixture) AcknowledgeException(_ context.Context, c entitlementAPI.AckCommand) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, exception := range f.exceptions {
		if exception.ExceptionID == c.ExceptionID.String() {
			f.acks = append(f.acks, entitlementDomain.ExceptionAck{
				ExceptionID: c.ExceptionID.String(), AckID: "01994f64-6c00-7000-8000-000000000011",
				ActorID: "01994f64-6c00-7000-8000-000000000001", Note: c.Note, CreatedAt: entitlementFixtureNow,
			})
			return nil
		}
	}
	return entitlementDomain.ErrNotFound
}
